package domain

// Relic 表示从 Wiki 解析出的遗物信息。
type Relic struct {
	Name        string `toml:"name"`
	ID          string `toml:"id"`
	Pool        string `toml:"pool"`
	Rarity      string `toml:"rarity"`
	Description string `toml:"description"`
	FlavorText  string `toml:"flavor_text"`
	SourceURL   string `toml:"source_url"`
}
