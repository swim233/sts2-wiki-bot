package formatter

import (
	"html"
	"strings"
)

// RichHTML 是已经按 Telegram Rich HTML 规则格式化的消息内容。
type RichHTML string

// Text 将普通文本编码为可安全嵌入 Rich HTML 的内容。
func Text(value string) RichHTML {
	return RichHTML(html.EscapeString(value))
}

func multilineText(value string) RichHTML {
	return RichHTML(strings.ReplaceAll(string(Text(value)), "\n", "<br>"))
}

func attribute(value string) string {
	return html.EscapeString(value)
}
