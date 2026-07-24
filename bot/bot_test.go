package bot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"sts2bot/formatter"
	"sts2bot/handler"
)

type fakeSender struct {
	params []*telegram.SendRichMessageParams
	err    error
}

func (f *fakeSender) SendRichMessage(_ context.Context, params *telegram.SendRichMessageParams) (*models.Message, error) {
	f.params = append(f.params, params)
	return &models.Message{}, f.err
}

type fakeHandler struct {
	response handler.Response
	request  handler.Request
}

func (f *fakeHandler) Handle(_ context.Context, request handler.Request) handler.Response {
	f.request = request
	return f.response
}

func commandUpdate(text string, commandLength int) *models.Update {
	return &models.Update{Message: &models.Message{
		ID:   42,
		From: &models.User{ID: 7, Username: "tester"},
		Chat: models.Chat{ID: 99},
		Text: text,
		Entities: []models.MessageEntity{{
			Type:   models.MessageEntityTypeBotCommand,
			Offset: 0,
			Length: commandLength,
		}},
	}}
}

func TestAdapterSendsRichMessage(t *testing.T) {
	richHTML := formatter.RichHTML(`<h1>🃏 计划妥当</h1><code>WELL_LAID_PLANS</code><a href="https://example.test">来源</a>`)
	commandHandler := &fakeHandler{response: handler.Response{RichHTML: richHTML}}
	sender := &fakeSender{}
	adapter := New(commandHandler, slog.New(slog.NewTextHandler(io.Discard, nil)))
	adapter.handleUpdate(context.Background(), sender, commandUpdate("/card 计划妥当", 5))

	if len(sender.params) != 1 {
		t.Fatalf("发送次数 = %d", len(sender.params))
	}
	params := sender.params[0]
	if params.ChatID != int64(99) || params.RichMessage.HTML != string(richHTML) {
		t.Fatalf("SendRichMessageParams = %+v", params)
	}
	if params.RichMessage.Markdown != "" {
		t.Fatalf("RichMessage 不应同时设置 Markdown: %+v", params.RichMessage)
	}
	if params.ReplyParameters == nil || params.ReplyParameters.MessageID != 42 {
		t.Fatalf("ReplyParameters = %+v", params.ReplyParameters)
	}
	if commandHandler.request.Command != "card" || commandHandler.request.Args != "计划妥当" || commandHandler.request.UserID != 7 {
		t.Fatalf("handler request = %+v", commandHandler.request)
	}
}

func TestAdapterParsesCommandWithBotUsername(t *testing.T) {
	commandHandler := &fakeHandler{response: handler.Response{RichHTML: "ok"}}
	sender := &fakeSender{}
	New(commandHandler, nil).handleUpdate(context.Background(), sender, commandUpdate("/relic@sts2bot 奥利哈钢", 14))
	if len(sender.params) != 1 || commandHandler.request.Command != "relic" || commandHandler.request.Args != "奥利哈钢" {
		t.Fatalf("params=%d request=%+v", len(sender.params), commandHandler.request)
	}
}

func TestAdapterHelpAndStart(t *testing.T) {
	tests := []struct {
		text   string
		length int
		want   string
	}{
		{text: "/help", length: 5, want: "help"},
		{text: "/HELP", length: 5, want: "help"},
		{text: "/help@sts2bot", length: 13, want: "help"},
		{text: "/start", length: 6, want: "start"},
		{text: "/START", length: 6, want: "start"},
		{text: "/start@sts2bot", length: 14, want: "start"},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			commandHandler := &fakeHandler{response: handler.Response{RichHTML: "help"}}
			sender := &fakeSender{}
			New(commandHandler, nil).handleUpdate(context.Background(), sender, commandUpdate(tt.text, tt.length))
			if len(sender.params) != 1 || commandHandler.request.Command != tt.want || commandHandler.request.Args != "" {
				t.Fatalf("sent=%d request=%+v", len(sender.params), commandHandler.request)
			}
		})
	}
}

func TestAdapterEnemyAndPotion(t *testing.T) {
	tests := []struct {
		text   string
		length int
		want   string
		args   string
	}{
		{"/enemy 飞蝇菌子", 6, "enemy", "飞蝇菌子"},
		{"/enemy@sts2bot 飞蝇菌子", 14, "enemy", "飞蝇菌子"},
		{"/potion 爆炸安瓿", 7, "potion", "爆炸安瓿"},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			commandHandler := &fakeHandler{response: handler.Response{RichHTML: "ok"}}
			sender := &fakeSender{}
			New(commandHandler, nil).handleUpdate(context.Background(), sender, commandUpdate(tt.text, tt.length))
			if len(sender.params) != 1 || commandHandler.request.Command != tt.want || commandHandler.request.Args != tt.args {
				t.Fatalf("sent=%d request=%+v", len(sender.params), commandHandler.request)
			}
		})
	}
}

func TestAdapterIgnoresUnsupportedUpdates(t *testing.T) {
	tests := []*models.Update{
		nil,
		{},
		{Message: &models.Message{Text: "普通消息", Chat: models.Chat{ID: 1}}},
		commandUpdate("/unknown test", 8),
	}
	for _, update := range tests {
		sender := &fakeSender{}
		New(&fakeHandler{}, nil).handleUpdate(context.Background(), sender, update)
		if len(sender.params) != 0 {
			t.Fatalf("update %+v 被意外发送", update)
		}
	}
}

func TestAdapterHandlesSendError(t *testing.T) {
	sender := &fakeSender{err: errors.New("发送失败")}
	commandHandler := &fakeHandler{response: handler.Response{RichHTML: "ok"}}
	New(commandHandler, slog.New(slog.NewTextHandler(io.Discard, nil))).handleUpdate(context.Background(), sender, commandUpdate("/card 打击", 5))
	if len(sender.params) != 1 {
		t.Fatalf("发送次数 = %d", len(sender.params))
	}
}
