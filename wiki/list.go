package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type EntityKind string

const (
	EntityCard   EntityKind = "card"
	EntityRelic  EntityKind = "relic"
	EntityEnemy  EntityKind = "enemy"
	EntityPotion EntityKind = "potion"
)

var entityTemplates = map[EntityKind]string{
	EntityCard:   "模板:卡牌信息框",
	EntityRelic:  "模板:遗物信息框",
	EntityEnemy:  "模板:怪物信息框",
	EntityPotion: "模板:药水信息框",
}

type PageRef struct {
	PageID int64
	Name   string
	URL    string
}

type embeddedInResponse struct {
	Continue struct {
		EIContinue string `json:"eicontinue"`
	} `json:"continue"`
	Query struct {
		EmbeddedIn []struct {
			PageID int64  `json:"pageid"`
			NS     int    `json:"ns"`
			Title  string `json:"title"`
		} `json:"embeddedin"`
	} `json:"query"`
	Warnings map[string]json.RawMessage `json:"warnings"`
	Error    *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
}

func (c *Client) List(ctx context.Context, kind EntityKind) ([]PageRef, error) {
	template, ok := entityTemplates[kind]
	if !ok {
		return nil, fmt.Errorf("不支持的数据类型 %q", kind)
	}
	seenIDs := make(map[int64]PageRef)
	seenTitles := make(map[string]int64)
	continuation := ""
	for {
		query := url.Values{
			"action":        {"query"},
			"list":          {"embeddedin"},
			"eititle":       {template},
			"einamespace":   {"0"},
			"eilimit":       {"max"},
			"format":        {"json"},
			"formatversion": {"2"},
		}
		if continuation != "" {
			query.Set("eicontinue", continuation)
		}
		body, sourceURL, err := c.fetchURL(ctx, c.baseURL+"/api.php?"+query.Encode(), "application/json")
		if err != nil {
			return nil, err
		}
		var response embeddedInResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, &Error{Kind: KindParse, Operation: "解析页面列表", URL: sourceURL, Err: err}
		}
		if response.Error != nil {
			return nil, &Error{Kind: KindUpstream, Operation: "获取页面列表", URL: sourceURL, Err: fmt.Errorf("API %s", response.Error.Code)}
		}
		if len(response.Warnings) > 0 {
			return nil, &Error{Kind: KindUpstream, Operation: "获取页面列表", URL: sourceURL, Err: fmt.Errorf("API 返回 warning")}
		}
		for _, item := range response.Query.EmbeddedIn {
			name := strings.TrimSpace(item.Title)
			if item.NS != 0 || item.PageID <= 0 || name == "" {
				return nil, &Error{Kind: KindParse, Operation: "解析页面列表", URL: sourceURL, Err: fmt.Errorf("页面列表包含非法条目")}
			}
			if existingID, exists := seenTitles[name]; exists && existingID != item.PageID {
				return nil, &Error{Kind: KindParse, Operation: "解析页面列表", URL: sourceURL, Err: fmt.Errorf("页面标题重复")}
			}
			seenTitles[name] = item.PageID
			seenIDs[item.PageID] = PageRef{PageID: item.PageID, Name: name, URL: c.baseURL + "/wiki/" + url.PathEscape(name)}
		}
		continuation = response.Continue.EIContinue
		if continuation == "" {
			break
		}
	}
	if len(seenIDs) == 0 {
		return nil, &Error{Kind: KindParse, Operation: "获取页面列表", URL: c.baseURL, Err: fmt.Errorf("页面列表为空")}
	}
	items := make([]PageRef, 0, len(seenIDs))
	for _, item := range seenIDs {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}
