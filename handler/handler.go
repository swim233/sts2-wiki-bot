package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"sts2bot/domain"
	"sts2bot/formatter"
	"sts2bot/service"
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
	case "help", "start":
		return Response{RichHTML: formatter.Help()}
	default:
		return Response{}
	}
}

func errorResponse(name string, err error) Response {
	text := ""
	switch {
	case errors.Is(err, service.ErrUninitialized):
		text = "❌ 本地数据尚未同步，请联系管理员执行 /update。"
	case errors.Is(err, service.ErrNotFound):
		text = fmt.Sprintf("❌ 未找到「%s」的相关信息，请确认名称是否正确。", name)
	case errors.Is(err, context.DeadlineExceeded):
		text = "❌ 本地数据查询超时，请稍后重试。"
	case errors.Is(err, context.Canceled):
		text = "❌ 本地数据查询已取消，请稍后重试。"
	default:
		text = "❌ 本地数据不可用，请联系管理员。"
	}
	return Response{RichHTML: formatter.Text(text)}
}
