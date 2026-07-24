package formatter

import (
	"fmt"

	"sts2bot/domain"
)

func Card(card domain.Card) RichHTML {
	return RichHTML(fmt.Sprintf(`<h1>🃏 %s</h1>
<code>%s</code>

<h2>属性</h2>
<ul>
<li>颜色：%s</li>
<li>稀有度：%s</li>
<li>耗能：%s</li>
</ul>

<h2>📖 描述</h2>
<p>%s</p>

<h2>⬆️ 升级后</h2>
<ul>
<li>耗能：%s</li>
</ul>
<p>%s</p>

<footer>——来源：<a href="%s">杀戮尖塔2中文维基</a></footer>`,
		Text(card.Name), Text(card.ID), Text(card.Color), Text(card.Rarity), Text(card.Cost),
		multilineText(card.Description), Text(card.UpgradedCost), multilineText(card.UpgradedDescription),
		attribute(card.SourceURL)))
}
