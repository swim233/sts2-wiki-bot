package handler

import (
	"context"
	"strings"

	"sts2bot/formatter"
)

func (h *Handler) handleCard(ctx context.Context, args string) Response {
	name := strings.TrimSpace(args)
	if name == "" {
		return Response{RichHTML: formatter.Text("用法：/card <卡牌名称>")}
	}
	card, err := h.lookup.LookupCard(ctx, name)
	if err != nil {
		h.logger.Error("卡牌查询失败", "event", "lookup_error", "type", "card", "name", name, "error", err)
		return errorResponse(name, err)
	}
	return Response{RichHTML: formatter.Card(card)}
}
