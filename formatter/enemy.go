package formatter

import (
	"fmt"
	"strings"

	"sts2bot/domain"
)

func healthText(health string) RichHTML {
	values := strings.Split(health, "\n")
	if len(values) == 2 {
		return RichHTML(fmt.Sprintf("%s(%s)", Text(strings.TrimSpace(values[0])), Text(strings.TrimSpace(values[1]))))
	}
	return multilineText(health)
}

func Enemy(enemy domain.Enemy) RichHTML {
	var builder strings.Builder
	fmt.Fprintf(&builder, `<h1>👾 %s</h1>
<code>%s</code>`, Text(enemy.Name), Text(enemy.ID))
	if enemy.Introduction != "" {
		fmt.Fprintf(&builder, "\n\n<p>%s</p>", multilineText(enemy.Introduction))
	}
	fmt.Fprintf(&builder, `

<h2>属性</h2>
<ul>
<li>生命值：%s</li>
<li>类型：%s</li>
<li>初始能力：%s</li>
<li>初次登场：%s</li>
<li>备注：%s</li>
</ul>`, healthText(enemy.Health), Text(enemy.Type), multilineText(enemy.InitialAbility), Text(enemy.FirstSeen), multilineText(enemy.Notes))

	builder.WriteString("\n\n<h2>⚔️ 行为</h2>")
	if enemy.BehaviorRule != "" {
		fmt.Fprintf(&builder, "\n<p>%s</p>", multilineText(enemy.BehaviorRule))
	}
	for _, move := range enemy.Moves {
		fmt.Fprintf(&builder, "\n\n<h3>%s</h3>", Text(move.Name))
		if move.EnglishName != "" {
			fmt.Fprintf(&builder, "\n<p><i>%s</i></p>", Text(move.EnglishName))
		}
		fmt.Fprintf(&builder, "\n<ul>\n<li>普通：%s</li>\n<li>9+：%s</li>\n</ul>", multilineText(move.Effect), multilineText(move.AdvancedEffect))
	}
	fmt.Fprintf(&builder, "\n\n<footer>——来源：<a href=\"%s\">杀戮尖塔2中文维基</a></footer>", attribute(enemy.SourceURL))
	return RichHTML(builder.String())
}
