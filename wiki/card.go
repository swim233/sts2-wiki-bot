package wiki

import (
	"fmt"
	"io"
	"strings"

	"sts2bot/domain"
)

func parseCard(r io.Reader, requestedName, sourceURL string) (domain.Card, error) {
	page, err := parsePage(r)
	if err != nil {
		return domain.Card{}, err
	}
	card := domain.Card{
		Name:                canonicalEntityName(page.name, requestedName),
		ID:                  page.fields["id"],
		Character:           firstNonEmpty(page.fields["color"], page.fields["owner"]),
		Rarity:              page.fields["rarity"],
		Cost:                page.fields["cost"],
		StarCost:            page.fields["star_cost"],
		Description:         page.fields["description"],
		UpgradedCost:        page.fields["upgraded_cost"],
		UpgradedStarCost:    page.fields["upgraded_star_cost"],
		UpgradedDescription: page.fields["upgraded_description"],
		ImageURLs:           page.cardImages,
		SourceURL:           sourceURL,
	}
	required := map[string]string{
		"卡牌名称": card.Name, "英文 ID": card.ID, "角色": card.Character,
		"稀有度": card.Rarity, "耗能": card.Cost, "描述": card.Description,
	}
	if card.UpgradedCost != "" || card.UpgradedDescription != "" {
		required["升级后耗能"] = card.UpgradedCost
		required["升级后描述"] = card.UpgradedDescription
	}
	missing := missingFields(required)
	if len(missing) > 0 {
		return domain.Card{}, &Error{Kind: KindParse, Operation: "解析卡牌", URL: sourceURL, Missing: missing, Err: fmt.Errorf("缺少字段: %v", missing)}
	}
	return card, nil
}
func canonicalEntityName(parsedName, requestedName string) string {
	parsedName = strings.TrimSpace(parsedName)
	requestedName = strings.TrimSpace(requestedName)
	if parsedName == "" {
		return requestedName
	}
	if requestedName != parsedName && strings.HasPrefix(requestedName, parsedName+"(") {
		return requestedName
	}
	return parsedName
}

func missingFields(fields map[string]string) []string {
	missing := make([]string, 0)
	for name, value := range fields {
		if value == "" {
			missing = append(missing, name)
		}
	}
	return missing
}
