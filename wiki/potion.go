package wiki

import (
	"fmt"
	"io"

	"sts2bot/domain"
)

func parsePotion(r io.Reader, requestedName, sourceURL string) (domain.Potion, error) {
	page, err := parsePage(r)
	if err != nil {
		return domain.Potion{}, err
	}
	potion := domain.Potion{
		Name:        firstNonEmpty(page.name, requestedName),
		ID:          page.fields["id"],
		Pool:        page.fields["potion_pool"],
		Rarity:      page.fields["rarity"],
		Description: page.fields["description"],
		SourceURL:   sourceURL,
	}
	missing := missingFields(map[string]string{
		"药水名称": potion.Name, "英文 ID": potion.ID, "所属药水池": potion.Pool,
		"稀有度": potion.Rarity, "描述": potion.Description,
	})
	if len(missing) > 0 {
		return domain.Potion{}, &Error{Kind: KindParse, Operation: "解析药水", URL: sourceURL, Missing: missing, Err: fmt.Errorf("缺少字段: %v", missing)}
	}
	return potion, nil
}
