package domain

// Card 表示从 Wiki 解析出的卡牌信息。
type Card struct {
	Name                string
	ID                  string
	Color               string
	Rarity              string
	Cost                string
	Description         string
	UpgradedCost        string
	UpgradedDescription string
	SourceURL           string
}
