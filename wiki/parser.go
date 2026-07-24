package wiki

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

var whitespace = regexp.MustCompile(`[\p{Z}\s]+`)

type parsedPage struct {
	name       string
	fields     map[string]string
	cardImages []string
}

var fieldAliases = map[string][]string{
	"id":                   {"英文ID", "ID", "内部ID", "卡牌ID", "遗物ID"},
	"color":                {"颜色", "角色"},
	"owner":                {"所属角色"},
	"rarity":               {"稀有度", "罕见度"},
	"cost":                 {"耗能", "费用", "能量"},
	"star_cost":            {"辉星", "辉星耗能", "辉星费用", "辉星消耗", "消耗辉星"},
	"description":          {"描述", "效果"},
	"upgraded_cost":        {"升级后耗能", "升级耗能", "升级费用", "升级后费用"},
	"upgraded_star_cost":   {"升级后辉星", "升级后辉星耗能", "升级后辉星费用", "升级后辉星消耗"},
	"upgraded_description": {"升级后描述", "升级描述", "升级效果", "升级后效果"},
	"pool":                 {"所属遗物池", "遗物池"},
	"flavor":               {"引言", "Flavortext", "风味文本"},
	"potion_pool":          {"所属药水池", "药水池"},
	"health":               {"生命值"},
	"enemy_type":           {"类型"},
	"initial_ability":      {"初始能力"},
	"first_seen":           {"初次登场"},
	"notes":                {"备注"},
}

func parsePage(r io.Reader) (parsedPage, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return parsedPage{}, &Error{Kind: KindParse, Operation: "解析 HTML", Err: err}
	}
	return parseDocument(doc)
}

func parseDocument(doc *goquery.Document) (parsedPage, error) {
	page := parsedPage{fields: make(map[string]string)}
	page.name = firstNonEmpty(
		firstLine(cleanText(doc.Find("#firstHeading").First())),
		firstLine(cleanText(doc.Find(".page-header__title").First())),
		firstLine(cleanText(doc.Find("h1").First())),
	)
	if page.name == "" {
		title := cleanText(doc.Find("title").First())
		page.name = strings.TrimSpace(strings.Split(title, "-")[0])
	}

	var best *goquery.Selection
	bestScore := -1
	doc.Find("table").Each(func(_ int, table *goquery.Selection) {
		score := 0
		table.Find("tr").Each(func(_ int, row *goquery.Selection) {
			cells := directCells(row)
			if len(cells) >= 2 && canonicalField(cleanText(cells[0])) != "" {
				score++
			}
		})
		if table.HasClass("infobox") {
			score += 2
		}
		if score > bestScore {
			best, bestScore = table, score
		}
	})
	if best == nil || bestScore < 2 {
		return parsedPage{}, &Error{Kind: KindParse, Operation: "定位信息框", Err: fmt.Errorf("未找到可识别的信息框")}
	}

	best.Find("tr").Each(func(_ int, row *goquery.Selection) {
		cells := directCells(row)
		if len(cells) < 2 {
			return
		}
		label := canonicalField(cleanText(cells[0]))
		if label == "" {
			return
		}
		value := cleanInfoboxValue(label, cells[1])
		if value != "" {
			page.fields[label] = value
		}
		if len(cells) >= 3 {
			upgraded := cleanInfoboxValue(label, cells[2])
			switch label {
			case "cost":
				page.fields["upgraded_cost"] = upgraded
			case "star_cost":
				page.fields["upgraded_star_cost"] = upgraded
			case "description":
				page.fields["upgraded_description"] = upgraded
			}
		}
	})
	if page.fields["id"] == "" {
		page.fields["id"] = infoboxID(best, page.name)
	}
	best.Find("img").Each(func(_ int, image *goquery.Selection) {
		alt := strings.TrimSpace(image.AttrOr("alt", ""))
		src := strings.TrimSpace(image.AttrOr("src", ""))
		if src == "" || !looksLikeCardImage(alt, src) {
			return
		}
		if absolute, ok := imageSource(image); ok {
			src = absolute
		}
		page.cardImages = append(page.cardImages, src)
	})
	return page, nil
}

func imageSource(image *goquery.Selection) (string, bool) {
	if srcset := strings.TrimSpace(image.AttrOr("srcset", "")); srcset != "" {
		candidates := strings.Split(srcset, ",")
		last := strings.Fields(strings.TrimSpace(candidates[len(candidates)-1]))
		if len(last) > 0 {
			return last[0], true
		}
	}
	return "", false
}

func looksLikeCardImage(alt, src string) bool {
	value := strings.ToLower(alt + " " + src)
	if strings.Contains(value, "star_icon") || strings.Contains(value, "star icon") {
		return false
	}
	return strings.Contains(value, ".png") && !strings.Contains(value, "icon")
}

func infoboxID(table *goquery.Selection, pageName string) string {
	firstRow := table.Find("tr").First()
	cells := directCells(firstRow)
	for _, cell := range cells {
		text := cleanText(cell)
		if text == "" || text == pageName {
			continue
		}
		if remainder, found := strings.CutPrefix(text, pageName); found {
			text = strings.TrimSpace(remainder)
		}
		if text != "" {
			return text
		}
	}
	return ""
}

func cleanInfoboxValue(label string, cell *goquery.Selection) string {
	clone := cell.Clone()
	if len(clone.Nodes) > 0 {
		clone = clone.Eq(0)
	}
	if label == "health" {
		clone.Find(".ascension_icon").Remove()
	}
	if label != "description" && label != "upgraded_description" {
		return cleanText(clone)
	}

	var builder strings.Builder
	for _, node := range clone.Nodes {
		appendDescriptionText(&builder, node)
	}
	return cleanString(builder.String())
}

func appendDescriptionText(builder *strings.Builder, node *html.Node) {
	if node.Type == html.ElementNode {
		tag := strings.ToLower(node.Data)
		if tag == "script" || tag == "style" || tag == "noscript" || tag == "sup" {
			return
		}
		if tag == "br" {
			builder.WriteByte('\n')
			return
		}
		if tag == "img" {
			alt := normalizeLabel(nodeAttribute(node, "alt"))
			title := normalizeLabel(nodeAttribute(node, "title"))
			src := strings.ToLower(nodeAttribute(node, "src"))
			if alt == "staricon.png" || title == "辉星" || strings.Contains(src, "/star_icon.png") {
				builder.WriteString("辉星")
			}
			return
		}
	}
	if node.Type == html.TextNode {
		builder.WriteString(node.Data)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		appendDescriptionText(builder, child)
	}
}

func nodeAttribute(node *html.Node, key string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val
		}
	}
	return ""
}

func directCells(row *goquery.Selection) []*goquery.Selection {
	var cells []*goquery.Selection
	row.ChildrenFiltered("th, td").Each(func(_ int, cell *goquery.Selection) {
		cells = append(cells, cell)
	})
	return cells
}

func canonicalField(label string) string {
	normalized := normalizeLabel(label)
	for field, aliases := range fieldAliases {
		for _, alias := range aliases {
			if normalized == normalizeLabel(alias) {
				return field
			}
		}
	}
	return ""
}

func normalizeLabel(value string) string {
	value = strings.Trim(value, " \t\r\n:：")
	value = whitespace.ReplaceAllString(value, "")
	return strings.ToLower(value)
}

func cleanText(selection *goquery.Selection) string {
	if selection == nil || selection.Length() == 0 {
		return ""
	}
	var builder strings.Builder
	for _, node := range selection.Nodes {
		appendNodeText(&builder, node)
	}
	return cleanString(builder.String())
}

func cleanString(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r", ""), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(whitespace.ReplaceAllString(line, " "))
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

func appendNodeText(builder *strings.Builder, node *html.Node) {
	if node.Type == html.ElementNode {
		tag := strings.ToLower(node.Data)
		if tag == "script" || tag == "style" || tag == "noscript" || tag == "sup" {
			return
		}
		if tag == "br" {
			builder.WriteByte('\n')
			return
		}
	}
	if node.Type == html.TextNode {
		builder.WriteString(node.Data)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		appendNodeText(builder, child)
	}
}

func firstLine(value string) string {
	line, _, _ := strings.Cut(value, "\n")
	return strings.TrimSpace(line)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
