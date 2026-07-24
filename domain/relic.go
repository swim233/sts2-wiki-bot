package domain

// Relic 表示从 Wiki 解析出的遗物信息。
type Relic struct {
	Name        string
	ID          string
	Pool        string
	Rarity      string
	Description string
	FlavorText  string
	SourceURL   string
}
