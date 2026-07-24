package handler

import (
	"context"
	"strings"

	"sts2bot/formatter"
)

func (h *Handler) handlePotion(ctx context.Context, args string) Response {
	name := strings.TrimSpace(args)
	if name == "" {
		return Response{RichHTML: formatter.Text("用法：/potion <药水名称>")}
	}
	potion, err := h.lookup.LookupPotion(ctx, name)
	if err != nil {
		h.logger.Error("药水查询失败", "event", "lookup_error", "type", "potion", "name", name, "error", err)
		return errorResponse(name, err)
	}
	return Response{RichHTML: formatter.Potion(potion)}
}
