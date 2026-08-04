package domain

// Enemy 表示从 Wiki 解析出的敌人信息。
type Enemy struct {
	Name           string      `toml:"name"`
	ID             string      `toml:"id"`
	Introduction   string      `toml:"introduction"`
	Health         string      `toml:"health"`
	Type           string      `toml:"type"`
	InitialAbility string      `toml:"initial_ability"`
	FirstSeen      string      `toml:"first_seen"`
	Notes          string      `toml:"notes"`
	BehaviorRule   string      `toml:"behavior_rule"`
	Moves          []EnemyMove `toml:"moves"`
	SourceURL      string      `toml:"source_url"`
}

// EnemyMove 表示敌人的一项行为及普通、进阶难度效果。
type EnemyMove struct {
	Name           string `toml:"name"`
	EnglishName    string `toml:"english_name"`
	Effect         string `toml:"effect"`
	AdvancedEffect string `toml:"advanced_effect"`
	Notes          string `toml:"notes"`
}
