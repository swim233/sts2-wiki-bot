package bot

import (
	"context"
	"log/slog"
	"strings"
	"unicode/utf16"

	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"sts2bot/handler"
)

const MaxConcurrentHandlers = 8

type commandHandler interface {
	Handle(ctx context.Context, req handler.Request) handler.Response
}

type messageSender interface {
	SendRichMessage(ctx context.Context, params *telegram.SendRichMessageParams) (*models.Message, error)
}

// Adapter 将 Telegram update 转换为项目命令并发送 Rich Message 回复。
type Adapter struct {
	handler commandHandler
	logger  *slog.Logger
}

func New(commandHandler commandHandler, logger *slog.Logger) *Adapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Adapter{handler: commandHandler, logger: logger}
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

	request := handler.Request{Command: command, Args: args}
	if message.From != nil {
		request.UserID = message.From.ID
		request.Username = message.From.Username
	}
	response := a.handler.Handle(ctx, request)
	if response.RichHTML == "" {
		return
	}

	params := &telegram.SendRichMessageParams{
		ChatID: message.Chat.ID,
		RichMessage: models.InputRichMessage{
			HTML: string(response.RichHTML),
		},
		ReplyParameters: &models.ReplyParameters{MessageID: message.ID},
	}
	if _, err := sender.SendRichMessage(ctx, params); err != nil {
		a.logger.Error("回复用户失败", "event", "reply_error", "user_id", request.UserID, "command", request.Command, "error", err)
		return
	}
	a.logger.Info("回复用户", "event", "reply_sent", "user_id", request.UserID, "command", request.Command)
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
	case "card", "relic", "enemy", "potion", "help", "start":
	default:
		return "", "", false
	}

	args := strings.TrimSpace(strings.TrimPrefix(message.Text, commandToken))
	return command, args, true
}

func sliceUTF16(value string, offset, length int) (string, bool) {
	encoded := utf16.Encode([]rune(value))
	if offset < 0 || length < 0 || offset+length > len(encoded) {
		return "", false
	}
	return string(utf16.Decode(encoded[offset : offset+length])), true
}
