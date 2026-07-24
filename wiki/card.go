package wiki

import (
	"fmt"
	"io"

	"sts2bot/domain"
)

func parseCard(r io.Reader, requestedName, sourceURL string) (domain.Card, error) {
	page, err := parsePage(r)
	if err != nil {
		return domain.Card{}, err
	}
	card := domain.Card{
		Name:                firstNonEmpty(page.name, requestedName),
		ID:                  page.fields["id"],
		Color:               firstNonEmpty(page.fields["color"], page.fields["owner"]),
		Rarity:              page.fields["rarity"],
		Cost:                page.fields["cost"],
		Description:         page.fields["description"],
		UpgradedCost:        page.fields["upgraded_cost"],
		UpgradedDescription: page.fields["upgraded_description"],
		SourceURL:           sourceURL,
	}
	missing := missingFields(map[string]string{
		"卡牌名称": card.Name, "英文 ID": card.ID, "颜色": card.Color,
		"稀有度": card.Rarity, "耗能": card.Cost, "描述": card.Description,
		"升级后耗能": card.UpgradedCost, "升级后描述": card.UpgradedDescription,
	})
	if len(missing) > 0 {
		return domain.Card{}, &Error{Kind: KindParse, Operation: "解析卡牌", URL: sourceURL, Missing: missing, Err: fmt.Errorf("缺少字段: %v", missing)}
	}
	return card, nil
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
