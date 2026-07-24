package formatter

import (
	"fmt"
	"strings"

	"sts2bot/domain"
)

func Relic(relic domain.Relic) RichHTML {
	var builder strings.Builder
	fmt.Fprintf(&builder, `<h1>🏺 %s</h1>
<code>%s</code>

<h2>属性</h2>
<ul>
<li>所属遗物池：%s</li>
<li>稀有度：%s</li>
</ul>

<h2>📖 描述</h2>
<p>%s</p>`, Text(relic.Name), Text(relic.ID), Text(relic.Pool), Text(relic.Rarity), multilineText(relic.Description))
	if relic.FlavorText != "" {
		fmt.Fprintf(&builder, `

<h2>💬 引言</h2>
<p>%s</p>`, multilineText(relic.FlavorText))
	}
	fmt.Fprintf(&builder, `

<footer>——来源：<a href="%s">杀戮尖塔2中文维基</a></footer>`, attribute(relic.SourceURL))
	return RichHTML(builder.String())
}
