package formatter

import (
	"strings"
	"testing"

	"sts2bot/domain"
)

func TestHelp(t *testing.T) {
	got := string(Help())
	for _, want := range []string{
		"<h1>使用帮助</h1>",
		"<code>/card &lt;卡牌名称&gt;</code>",
		"<code>/relic &lt;遗物名称&gt;</code>",
		"<code>/enemy &lt;敌人名称&gt;</code>",
		"<code>/potion &lt;药水名称&gt;</code>",
		"<code>/help</code>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("帮助输出缺少 %q：%s", want, got)
		}
	}
	if strings.Contains(got, "<卡牌名称>") {
		t.Fatalf("帮助占位符未编码：%s", got)
	}
}

func TestTextEscapesRichHTML(t *testing.T) {
	if got, want := Text(`<tag attr="x">Tom & Jerry's</tag>`), RichHTML(`&lt;tag attr=&#34;x&#34;&gt;Tom &amp; Jerry&#39;s&lt;/tag&gt;`); got != want {
		t.Fatalf("Text() = %q，期望 %q", got, want)
	}
}

func TestMultilineTextPreservesLineBreaks(t *testing.T) {
	if got, want := multilineText("第一行<&\n第二行"), RichHTML("第一行&lt;&amp;<br>第二行"); got != want {
		t.Fatalf("multilineText() = %q，期望 %q", got, want)
	}
}

func TestCard(t *testing.T) {
	got := string(Card(domain.Card{Name: "打击", ID: "STRIKE", Color: "铁甲战士", Rarity: "初始", Cost: "1", Description: "造成6点伤害。", UpgradedCost: "1", UpgradedDescription: "造成9点伤害。", SourceURL: "https://example.test/wiki/打击"}))
	for _, want := range []string{"<h1>🃏 打击</h1>", "<code>STRIKE</code>", "<li>颜色：铁甲战士</li>", "<p>造成6点伤害。</p>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("输出缺少 %q：%s", want, got)
		}
	}
}

func TestCardEscapesDynamicRichHTML(t *testing.T) {
	got := string(Card(domain.Card{
		Name: `计划</h1><script>妥当`, ID: `WELL</code>&PLANS`, Color: `静默&猎手`, Rarity: `罕见>`, Cost: `1" onclick="bad`,
		Description: "保留<1>张牌。\n第二行", UpgradedCost: "1", UpgradedDescription: "保留2张牌。",
		SourceURL: `https://example.test/wiki?a=1&b="bad"`,
	}))
	for _, want := range []string{
		`<h1>🃏 计划&lt;/h1&gt;&lt;script&gt;妥当</h1>`,
		`<code>WELL&lt;/code&gt;&amp;PLANS</code>`,
		`静默&amp;猎手`,
		`罕见&gt;`,
		`1&#34; onclick=&#34;bad`,
		`保留&lt;1&gt;张牌。<br>第二行`,
		`href="https://example.test/wiki?a=1&amp;b=&#34;bad&#34;"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("输出缺少 %q：%s", want, got)
		}
	}
	if strings.Contains(got, "<script>") {
		t.Fatalf("动态内容逃逸到 HTML：%s", got)
	}
}

func TestHealthText(t *testing.T) {
	tests := []struct {
		name   string
		health string
		want   RichHTML
	}{
		{name: "单值", health: "121", want: "121"},
		{name: "高难度值", health: "121\n126", want: "121(126)"},
		{name: "编码动态值", health: "121<&\n126>", want: "121&lt;&amp;(126&gt;)"},
		{name: "异常多行保留换行", health: "1\n2\n3", want: "1<br>2<br>3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := healthText(tt.health); got != tt.want {
				t.Fatalf("healthText(%q) = %q，期望 %q", tt.health, got, tt.want)
			}
		})
	}
}

func TestEnemyAndPotion(t *testing.T) {
	enemy := string(Enemy(domain.Enemy{Name: "飞蝇菌子", ID: "FLYCONID", Health: "47~49\n51~53", Type: "普通", InitialAbility: "nil", FirstSeen: "密林", Notes: "无", BehaviorRule: "等权重随机", Moves: []domain.EnemyMove{{Name: "猛砸", EnglishName: "Smash", Effect: "造成11点伤害。", AdvancedEffect: "造成12点伤害。"}}, SourceURL: "https://example.test"}))
	for _, want := range []string{"<h1>👾 飞蝇菌子</h1>", "<code>FLYCONID</code>", "<li>生命值：47~49(51~53)</li>", "<h3>猛砸</h3>", "<i>Smash</i>", "9+"} {
		if !strings.Contains(enemy, want) {
			t.Fatalf("敌人输出缺少 %q：%s", want, enemy)
		}
	}
	potion := string(Potion(domain.Potion{Name: "爆炸安瓿", ID: "EXPLOSIVE_AMPOULE", Pool: "通用", Rarity: "普通", Description: "对所有敌人造成10点伤害。", SourceURL: "https://example.test"}))
	for _, want := range []string{"<h1>🧪 爆炸安瓿</h1>", "<code>EXPLOSIVE_AMPOULE</code>", "所属药水池：通用", "造成10点伤害"} {
		if !strings.Contains(potion, want) {
			t.Fatalf("药水输出缺少 %q：%s", want, potion)
		}
	}
}

func TestRelicOmitsEmptyFlavor(t *testing.T) {
	got := string(Relic(domain.Relic{Name: "锚", ID: "ANCHOR", Pool: "通用", Rarity: "普通", Description: "获得10点格挡。", SourceURL: "https://example.test"}))
	if strings.Contains(got, "引言") {
		t.Fatalf("空引言仍被输出：%s", got)
	}
}
