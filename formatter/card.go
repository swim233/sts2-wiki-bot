package formatter

import (
	"fmt"
	"strings"

	"sts2bot/domain"
)

func Card(card domain.Card) RichHTML {
	return RichHTML(fmt.Sprintf(`<h1>🃏 %s</h1>
<code>%s</code>

<h2>属性</h2>
<ul>
<li>角色：%s</li>
<li>稀有度：%s</li>
<li>耗能：%s</li>%s
</ul>

<h2>📖 描述</h2>
<p>%s</p>

<h2>⬆️ 升级后</h2>
<ul>
<li>耗能：%s</li>%s
</ul>
<p>%s</p>

<footer>——来源：<a href="%s">杀戮尖塔2中文维基</a></footer>`,
		Text(card.Name), Text(card.ID), Text(card.Character), Text(card.Rarity), Text(card.Cost),
		starCostItem(card.StarCost), multilineText(card.Description), Text(card.UpgradedCost),
		starCostItem(card.UpgradedStarCost), multilineText(card.UpgradedDescription), attribute(card.SourceURL)))
}

func starCostItem(cost string) RichHTML {
	if strings.TrimSpace(cost) == "" {
		return ""
	}
	return RichHTML(fmt.Sprintf("\n<li>辉星：%s</li>", Text(cost)))
}
