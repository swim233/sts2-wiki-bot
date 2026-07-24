package formatter

func Help() RichHTML {
	return `<h1>使用帮助</h1>
<p>查询《杀戮尖塔 2》中文维基资料。</p>

<h2>可用命令</h2>
<ul>
<li><code>/card &lt;卡牌名称&gt;</code>：查询卡牌</li>
<li><code>/relic &lt;遗物名称&gt;</code>：查询遗物</li>
<li><code>/enemy &lt;敌人名称&gt;</code>：查询敌人</li>
<li><code>/potion &lt;药水名称&gt;</code>：查询药水</li>
<li><code>/help</code>：显示本帮助</li>
</ul>

<h2>示例</h2>
<ul>
<li><code>/card 打击</code></li>
<li><code>/relic 燃烧之血</code></li>
<li><code>/enemy 飞蝇菌子</code></li>
<li><code>/potion 爆炸安瓿</code></li>
</ul>

<p>查询名称请使用中文维基中的中文名。</p>`
}
