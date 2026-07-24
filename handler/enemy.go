package handler

import (
	"context"
	"strings"

	"sts2bot/formatter"
)

func (h *Handler) handleEnemy(ctx context.Context, args string) Response {
	name := strings.TrimSpace(args)
	if name == "" {
		return Response{RichHTML: formatter.Text("用法：/enemy <敌人名称>")}
	}
	enemy, err := h.lookup.LookupEnemy(ctx, name)
	if err != nil {
		h.logger.Error("敌人查询失败", "event", "lookup_error", "type", "enemy", "name", name, "error", err)
		return errorResponse(name, err)
	}
	return Response{RichHTML: formatter.Enemy(enemy)}
}
