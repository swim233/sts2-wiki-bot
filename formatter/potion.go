package formatter

import (
	"fmt"

	"sts2bot/domain"
)

func Potion(potion domain.Potion) RichHTML {
	return RichHTML(fmt.Sprintf(`<h1>🧪 %s</h1>
<code>%s</code>

<h2>属性</h2>
<ul>
<li>所属药水池：%s</li>
<li>稀有度：%s</li>
</ul>

<h2>📖 描述</h2>
<p>%s</p>

<footer>——来源：<a href="%s">杀戮尖塔2中文维基</a></footer>`,
		Text(potion.Name), Text(potion.ID), Text(potion.Pool), Text(potion.Rarity),
		multilineText(potion.Description), attribute(potion.SourceURL)))
}
