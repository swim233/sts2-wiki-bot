package domain

// Card 表示从 Wiki 解析出的卡牌信息。
type Card struct {
	Name                string
	ID                  string
	Character           string
	Rarity              string
	Cost                string
	StarCost            string
	Description         string
	UpgradedCost        string
	UpgradedStarCost    string
	UpgradedDescription string
	ImageURLs           []string
	SourceURL           string
}
