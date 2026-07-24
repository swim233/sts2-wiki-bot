package handler

import (
	"context"
	"strings"

	"sts2bot/formatter"
)

func (h *Handler) handleRelic(ctx context.Context, args string) Response {
	name := strings.TrimSpace(args)
	if name == "" {
		return Response{RichHTML: formatter.Text("用法：/relic <遗物名称>")}
	}
	relic, err := h.lookup.LookupRelic(ctx, name)
	if err != nil {
		h.logger.Error("遗物查询失败", "event", "lookup_error", "type", "relic", "name", name, "error", err)
		return errorResponse(name, err)
	}
	return Response{RichHTML: formatter.Relic(relic)}
}
