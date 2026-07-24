package domain

// Enemy 表示从 Wiki 解析出的敌人信息。
type Enemy struct {
	Name           string
	ID             string
	Introduction   string
	Health         string
	Type           string
	InitialAbility string
	FirstSeen      string
	Notes          string
	BehaviorRule   string
	Moves          []EnemyMove
	SourceURL      string
}

// EnemyMove 表示敌人的一项行为及普通、进阶难度效果。
type EnemyMove struct {
	Name           string
	EnglishName    string
	Effect         string
	AdvancedEffect string
	Notes          string
}
