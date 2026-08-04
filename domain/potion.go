package domain

// Potion 表示从 Wiki 解析出的药水信息。
type Potion struct {
	Name        string `toml:"name"`
	ID          string `toml:"id"`
	Pool        string `toml:"pool"`
	Rarity      string `toml:"rarity"`
	Description string `toml:"description"`
	SourceURL   string `toml:"source_url"`
}
