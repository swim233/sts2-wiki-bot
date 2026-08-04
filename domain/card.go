package domain

import "strings"

// NormalizeName trims and collapses whitespace for local lookup keys.
func NormalizeName(name string) string {
	return strings.Join(strings.Fields(name), " ")
}

// Card 表示从 Wiki 解析出的卡牌信息。
type Card struct {
	Name                string   `toml:"name"`
	ID                  string   `toml:"id"`
	Character           string   `toml:"character"`
	Rarity              string   `toml:"rarity"`
	Cost                string   `toml:"cost"`
	StarCost            string   `toml:"star_cost"`
	Description         string   `toml:"description"`
	UpgradedCost        string   `toml:"upgraded_cost"`
	UpgradedStarCost    string   `toml:"upgraded_star_cost"`
	UpgradedDescription string   `toml:"upgraded_description"`
	ImageURLs           []string `toml:"image_urls"`
	SourceURL           string   `toml:"source_url"`
}
