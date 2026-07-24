package domain

// Potion 表示从 Wiki 解析出的药水信息。
type Potion struct {
	Name        string
	ID          string
	Pool        string
	Rarity      string
	Description string
	SourceURL   string
}
