package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"sts2bot/domain"
	"sts2bot/formatter"
	"sts2bot/wiki"
)

type LookupService interface {
	LookupCard(ctx context.Context, name string) (domain.Card, error)
	LookupRelic(ctx context.Context, name string) (domain.Relic, error)
	LookupEnemy(ctx context.Context, name string) (domain.Enemy, error)
	LookupPotion(ctx context.Context, name string) (domain.Potion, error)
}

type Request struct {
	Command  string
	Args     string
	UserID   int64
	Username string
}

type Response struct {
	RichHTML formatter.RichHTML
}

// Handler 将 Telegram 指令转换为查询和格式化后的回复。
type Handler struct {
	lookup LookupService
	logger *slog.Logger
}

func New(lookup LookupService, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{lookup: lookup, logger: logger}
}

func (h *Handler) Handle(ctx context.Context, req Request) Response {
	h.logger.Info("收到指令", "event", "command_received", "user_id", req.UserID, "username", req.Username, "command", req.Command, "args", req.Args)
	switch req.Command {
	case "card":
		return h.handleCard(ctx, req.Args)
	case "relic":
		return h.handleRelic(ctx, req.Args)
	case "enemy":
		return h.handleEnemy(ctx, req.Args)
	case "potion":
		return h.handlePotion(ctx, req.Args)
	case "help":
		return Response{RichHTML: formatter.Help()}
	default:
		return Response{}
	}
}

func errorResponse(name string, err error) Response {
	text := ""
	switch {
	case wiki.IsKind(err, wiki.KindNotFound):
		text = fmt.Sprintf("❌ 未找到「%s」的相关信息，请确认名称是否正确。", name)
	case wiki.IsKind(err, wiki.KindParse):
		text = "❌ 数据解析失败，Wiki 页面格式可能已更新，请联系管理员。"
	case wiki.IsKind(err, wiki.KindBlocked):
		text = "❌ 请求失败，请稍后重试。（错误：Wiki 暂时拒绝访问）"
	case errors.Is(err, context.DeadlineExceeded):
		text = "❌ 请求失败，请稍后重试。（错误：请求超时）"
	default:
		text = fmt.Sprintf("❌ 请求失败，请稍后重试。（错误：%s）", safeError(err))
	}
	return Response{RichHTML: formatter.Text(text)}
}

func safeError(err error) string {
	var wikiErr *wiki.Error
	if errors.As(err, &wikiErr) {
		switch wikiErr.Kind {
		case wiki.KindRateLimited:
			return "Wiki 请求过于频繁"
		case wiki.KindUpstream, wiki.KindHTTPStatus:
			return "Wiki 服务异常"
		case wiki.KindNetwork:
			return "网络请求失败"
		case wiki.KindBodyTooLarge:
			return "Wiki 响应过大"
		}
	}
	return "未知错误"
}
