package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf16"

	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"sts2bot/formatter"
	"sts2bot/handler"
	"sts2bot/service"
)

const MaxConcurrentHandlers = 8

type commandHandler interface {
	Handle(ctx context.Context, req handler.Request) handler.Response
}

type dataUpdater interface {
	TryAcquire() bool
	Release()
	Run(context.Context, func(service.UpdateProgress)) (service.UpdateSummary, error)
}

type messageSender interface {
	SendRichMessage(ctx context.Context, params *telegram.SendRichMessageParams) (*models.Message, error)
	EditMessageText(ctx context.Context, params *telegram.EditMessageTextParams) (*models.Message, error)
}

// Adapter 将 Telegram update 转换为项目命令并发送 Rich Message 回复。
type Adapter struct {
	handler commandHandler
	updater dataUpdater
	ownerID int64
	logger  *slog.Logger
}

func New(commandHandler commandHandler, updater dataUpdater, ownerID int64, logger *slog.Logger) *Adapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Adapter{handler: commandHandler, updater: updater, ownerID: ownerID, logger: logger}
}

func (a *Adapter) HandleUpdate(ctx context.Context, sender *telegram.Bot, update *models.Update) {
	a.handleUpdate(ctx, sender, update)
}

func (a *Adapter) handleUpdate(ctx context.Context, sender messageSender, update *models.Update) {
	if sender == nil || update == nil || update.Message == nil {
		return
	}
	message := update.Message
	command, args, ok := parseCommand(message)
	if !ok {
		return
	}
	if command == "update" {
		a.handleUpdateCommand(ctx, sender, message)
		return
	}
	request := handler.Request{Command: command, Args: args}
	if message.From != nil {
		request.UserID = message.From.ID
		request.Username = message.From.Username
	}
	response := a.handler.Handle(ctx, request)
	if response.RichHTML == "" {
		return
	}
	if _, err := sender.SendRichMessage(ctx, richReply(message, response.RichHTML)); err != nil {
		a.logger.Error("回复用户失败", "event", "reply_error", "user_id", request.UserID, "command", request.Command, "error", err)
		return
	}
	a.logger.Info("回复用户", "event", "reply_sent", "user_id", request.UserID, "command", request.Command)
}

func richReply(message *models.Message, html formatter.RichHTML) *telegram.SendRichMessageParams {
	return &telegram.SendRichMessageParams{ChatID: message.Chat.ID, RichMessage: models.InputRichMessage{HTML: string(html)}, ReplyParameters: &models.ReplyParameters{MessageID: message.ID}}
}

func (a *Adapter) handleUpdateCommand(ctx context.Context, sender messageSender, message *models.Message) {
	if message.From == nil || message.From.ID != a.ownerID {
		_, _ = sender.SendRichMessage(ctx, richReply(message, formatter.Text("⛔ 仅 Bot Owner 可执行 /update。")))
		return
	}
	if a.updater == nil || !a.updater.TryAcquire() {
		_, _ = sender.SendRichMessage(ctx, richReply(message, formatter.Text("⏳ 数据同步正在进行中。")))
		return
	}
	started, err := sender.SendRichMessage(ctx, richReply(message, renderProgress(service.UpdateProgress{Stage: service.StageEnumerating})))
	if err != nil {
		a.updater.Release()
		a.logger.Error("发送同步进度失败", "event", "update_progress_send_error", "error", err)
		return
	}
	defer a.updater.Release()
	lastHTML := string(renderProgress(service.UpdateProgress{Stage: service.StageEnumerating}))
	lastEdited := time.Time{}
	lastStage := service.UpdateStage("")
	lastFailures := 0
	report := func(progress service.UpdateProgress) {
		failureCount := len(progress.Failures)
		terminal := progress.Stage == service.StageCompleted || progress.Stage == service.StageFailed
		immediate := progress.Stage != lastStage || lastFailures == 0 && failureCount > 0 || terminal
		if !immediate && time.Since(lastEdited) < time.Second {
			return
		}
		html := renderProgress(progress)
		htmlText := string(html)
		if htmlText == lastHTML {
			lastStage, lastFailures = progress.Stage, failureCount
			return
		}
		_, editErr := sender.EditMessageText(ctx, &telegram.EditMessageTextParams{ChatID: message.Chat.ID, MessageID: started.ID, RichMessage: &models.InputRichMessage{HTML: htmlText}})
		notModified := editErr != nil && strings.Contains(strings.ToLower(editErr.Error()), "message is not modified")
		if editErr != nil && !notModified {
			a.logger.Error("更新同步进度失败", "event", "update_progress_edit_error", "error", editErr)
		}
		if editErr == nil || notModified {
			lastHTML = htmlText
		}
		lastEdited, lastStage, lastFailures = time.Now(), progress.Stage, failureCount
	}
	_, runErr := a.updater.Run(ctx, report)
	if runErr != nil {
		a.logger.Error("数据同步失败", "event", "update_error", "error", runErr)
	}
}

func renderProgress(progress service.UpdateProgress) formatter.RichHTML {
	label := map[service.UpdateStage]string{service.StageEnumerating: "枚举", service.StageFetching: "拉取", service.StageValidating: "校验", service.StagePublishing: "发布", service.StageCompleted: "完成", service.StageFailed: "失败"}[progress.Stage]
	if label == "" {
		label = string(progress.Stage)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<h1>Wiki 数据同步：%s</h1>", formatter.Text(label))
	if progress.Kind != "" || progress.Name != "" {
		fmt.Fprintf(&b, "<p>当前：%s %s</p>", formatter.Text(string(progress.Kind)), formatter.Text(progress.Name))
	}
	for _, row := range []struct {
		name string
		p    service.KindProgress
	}{{"卡牌", progress.Counts.Cards}, {"遗物", progress.Counts.Relics}, {"敌人", progress.Counts.Enemies}, {"药水", progress.Counts.Potions}} {
		fmt.Fprintf(&b, "<p>%s：成功 %d / 已完成 %d / 总数 %d</p>", row.name, row.p.Succeeded, row.p.Completed, row.p.Total)
	}
	fmt.Fprintf(&b, "<p>失败：%d</p>", len(progress.Failures))
	limit := len(progress.Failures)
	if limit > 20 {
		limit = 20
	}
	if limit > 0 {
		b.WriteString("<ul>")
		for _, failure := range progress.Failures[:limit] {
			fmt.Fprintf(&b, "<li>%s / %s / %s</li>", formatter.Text(string(failure.Kind)), formatter.Text(failure.Name), formatter.Text(failure.Reason))
		}
		b.WriteString("</ul>")
	}
	if remaining := len(progress.Failures) - limit; remaining > 0 {
		fmt.Fprintf(&b, "<p>另有 %d 项失败。</p>", remaining)
	}
	return formatter.RichHTML(b.String())
}

func parseCommand(message *models.Message) (string, string, bool) {
	if message == nil || message.Text == "" {
		return "", "", false
	}
	var entity *models.MessageEntity
	for i := range message.Entities {
		candidate := &message.Entities[i]
		if candidate.Type == models.MessageEntityTypeBotCommand && candidate.Offset == 0 {
			entity = candidate
			break
		}
	}
	if entity == nil || entity.Length <= 1 {
		return "", "", false
	}
	commandToken, ok := sliceUTF16(message.Text, entity.Offset, entity.Length)
	if !ok || !strings.HasPrefix(commandToken, "/") {
		return "", "", false
	}
	command := strings.TrimPrefix(commandToken, "/")
	command, _, _ = strings.Cut(command, "@")
	command = strings.ToLower(command)
	switch command {
	case "card", "relic", "enemy", "potion", "help", "start", "update":
	default:
		return "", "", false
	}
	return command, strings.TrimSpace(strings.TrimPrefix(message.Text, commandToken)), true
}

func sliceUTF16(value string, offset, length int) (string, bool) {
	encoded := utf16.Encode([]rune(value))
	if offset < 0 || length < 0 || offset+length > len(encoded) {
		return "", false
	}
	return string(utf16.Decode(encoded[offset : offset+length])), true
}
