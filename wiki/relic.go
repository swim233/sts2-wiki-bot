package wiki

import (
	"fmt"
	"io"

	"sts2bot/domain"
)

func parseRelic(r io.Reader, requestedName, sourceURL string) (domain.Relic, error) {
	page, err := parsePage(r)
	if err != nil {
		return domain.Relic{}, err
	}
	relic := domain.Relic{
		Name:        firstNonEmpty(page.name, requestedName),
		ID:          page.fields["id"],
		Pool:        firstNonEmpty(page.fields["pool"], page.fields["owner"]),
		Rarity:      page.fields["rarity"],
		Description: page.fields["description"],
		FlavorText:  page.fields["flavor"],
		SourceURL:   sourceURL,
	}
	missing := missingFields(map[string]string{
		"遗物名称": relic.Name, "英文 ID": relic.ID, "所属遗物池": relic.Pool,
		"稀有度": relic.Rarity, "描述": relic.Description,
	})
	if len(missing) > 0 {
		return domain.Relic{}, &Error{Kind: KindParse, Operation: "解析遗物", URL: sourceURL, Missing: missing, Err: fmt.Errorf("缺少字段: %v", missing)}
	}
	return relic, nil
}
