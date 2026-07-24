package wiki

import (
	"fmt"
	"io"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"sts2bot/domain"
)

func parseEnemy(r io.Reader, requestedName, sourceURL string) (domain.Enemy, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return domain.Enemy{}, &Error{Kind: KindParse, Operation: "解析敌人 HTML", URL: sourceURL, Err: err}
	}
	page, err := parseDocument(doc)
	if err != nil {
		return domain.Enemy{}, err
	}
	enemy := domain.Enemy{
		Name:           firstNonEmpty(page.name, requestedName),
		ID:             page.fields["id"],
		Introduction:   enemyIntroduction(doc),
		Health:         page.fields["health"],
		Type:           page.fields["enemy_type"],
		InitialAbility: page.fields["initial_ability"],
		FirstSeen:      page.fields["first_seen"],
		Notes:          page.fields["notes"],
		SourceURL:      sourceURL,
	}
	enemy.Moves, enemy.BehaviorRule = enemyMoves(doc)
	missing := missingFields(map[string]string{
		"敌人名称": enemy.Name, "英文 ID": enemy.ID, "生命值": enemy.Health,
		"类型": enemy.Type, "初次登场": enemy.FirstSeen,
	})
	if len(enemy.Moves) == 0 {
		missing = append(missing, "行为")
	}
	if len(missing) > 0 {
		return domain.Enemy{}, &Error{Kind: KindParse, Operation: "解析敌人", URL: sourceURL, Missing: missing, Err: fmt.Errorf("缺少字段: %v", missing)}
	}
	return enemy, nil
}

func enemyIntroduction(doc *goquery.Document) string {
	infobox := doc.Find("table.infobox").First()
	for sibling := infobox.Next(); sibling.Length() > 0; sibling = sibling.Next() {
		if goquery.NodeName(sibling) == "h2" {
			break
		}
		if goquery.NodeName(sibling) == "p" {
			if text := cleanText(sibling); text != "" {
				return text
			}
		}
	}
	return ""
}

func enemyMoves(doc *goquery.Document) ([]domain.EnemyMove, string) {
	var table *goquery.Selection
	doc.Find("h2").EachWithBreak(func(_ int, heading *goquery.Selection) bool {
		if firstLine(cleanText(heading)) != "行为" {
			return true
		}
		table = heading.NextAllFiltered("table.wikitable").First()
		return false
	})
	if table == nil || table.Length() == 0 {
		return nil, ""
	}

	var moves []domain.EnemyMove
	sharedNotes := ""
	table.Find("tr").Each(func(index int, row *goquery.Selection) {
		if index == 0 {
			return
		}
		cells := directCells(row)
		if len(cells) < 3 {
			return
		}
		name, english := splitMoveName(cleanText(cells[0]))
		move := domain.EnemyMove{
			Name:           name,
			EnglishName:    english,
			Effect:         cleanEffect(cells[1]),
			AdvancedEffect: cleanEffect(cells[2]),
		}
		if len(cells) >= 4 {
			sharedNotes = cleanText(cells[3])
		}
		move.Notes = sharedNotes
		moves = append(moves, move)
	})
	return moves, sharedNotes
}

func splitMoveName(value string) (string, string) {
	lines := strings.Split(value, "\n")
	name := strings.TrimSpace(lines[0])
	if len(lines) < 2 {
		return name, ""
	}
	english := strings.Trim(strings.TrimSpace(strings.Join(lines[1:], " ")), "()（）")
	return name, english
}

func cleanEffect(cell *goquery.Selection) string {
	clone := cell.Clone()
	clone.Find(".huiji-tt-preload, .tooltip, .tooltip-content, .popup, .mw-parser-output-hover, [role=tooltip]").Remove()
	return cleanText(clone)
}
