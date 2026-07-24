package wiki

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sts2bot/domain"
)

const (
	DefaultBaseURL = "https://sts2.huijiwiki.com"
	maxBodySize    = 2 << 20
)

// Client 请求并解析杀戮尖塔 2 中文 Wiki。
type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewClient(baseURL string, httpClient *http.Client, logger *slog.Logger) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("无效的 Wiki base URL %q", baseURL)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{baseURL: baseURL, httpClient: httpClient, logger: logger}, nil
}

func (c *Client) GetCard(ctx context.Context, name string) (domain.Card, error) {
	body, sourceURL, err := c.fetch(ctx, name)
	if err != nil {
		return domain.Card{}, err
	}
	card, err := parseCard(bytes.NewReader(body), name, sourceURL)
	if err != nil {
		return domain.Card{}, err
	}
	if card.StarCost == "" || card.UpgradedStarCost == "" {
		if err := c.populateCardStarCosts(ctx, &card); err != nil {
			c.logger.Warn("解析卡牌辉星消耗失败", "event", "card_star_cost_error", "name", card.Name, "error", err)
		}
	}
	c.logger.Debug("Wiki 解析成功", "event", "parse_success", "type", "card", "name", card.Name)
	return card, nil
}

func (c *Client) populateCardStarCosts(ctx context.Context, card *domain.Card) error {
	for index, imageURL := range card.ImageURLs {
		if index > 1 {
			break
		}
		if index == 0 && card.StarCost != "" || index == 1 && card.UpgradedStarCost != "" {
			continue
		}
		cost, err := c.cardStarCost(ctx, imageURL)
		if err != nil {
			return err
		}
		if index == 0 {
			card.StarCost = cost
		} else {
			card.UpgradedStarCost = cost
		}
	}
	return nil
}

func (c *Client) cardStarCost(ctx context.Context, imageURL string) (string, error) {
	parsed, err := url.Parse(imageURL)
	if err != nil {
		return "", fmt.Errorf("解析卡图 URL: %w", err)
	}
	if !parsed.IsAbs() {
		base, _ := url.Parse(c.baseURL)
		parsed = base.ResolveReference(parsed)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", fmt.Errorf("创建卡图请求: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求卡图: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("请求卡图返回 HTTP %d", resp.StatusCode)
	}
	cardImage, _, err := image.Decode(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return "", fmt.Errorf("解析卡图: %w", err)
	}
	return starCostFromCardImage(cardImage), nil
}

func starCostFromCardImage(cardImage image.Image) string {
	bounds := cardImage.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return ""
	}
	left, top := width/80, height*2/15
	right, bottom := width/16, height*11/60
	opaque := 0
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			_, _, _, alpha := cardImage.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			if alpha > 0x1000 {
				opaque++
			}
		}
	}
	area := (right - left) * (bottom - top)
	if area > 0 && opaque*100/area >= 25 {
		return "2"
	}
	return ""
}

func (c *Client) GetEnemy(ctx context.Context, name string) (domain.Enemy, error) {
	body, sourceURL, err := c.fetch(ctx, name)
	if err != nil {
		return domain.Enemy{}, err
	}
	enemy, err := parseEnemy(bytes.NewReader(body), name, sourceURL)
	if err != nil {
		return domain.Enemy{}, err
	}
	c.logger.Debug("Wiki 解析成功", "event", "parse_success", "type", "enemy", "name", enemy.Name)
	return enemy, nil
}

func (c *Client) GetPotion(ctx context.Context, name string) (domain.Potion, error) {
	body, sourceURL, err := c.fetch(ctx, name)
	if err != nil {
		return domain.Potion{}, err
	}
	potion, err := parsePotion(bytes.NewReader(body), name, sourceURL)
	if err != nil {
		return domain.Potion{}, err
	}
	c.logger.Debug("Wiki 解析成功", "event", "parse_success", "type", "potion", "name", potion.Name)
	return potion, nil
}

func (c *Client) GetRelic(ctx context.Context, name string) (domain.Relic, error) {
	body, sourceURL, err := c.fetch(ctx, name)
	if err != nil {
		return domain.Relic{}, err
	}
	relic, err := parseRelic(bytes.NewReader(body), name, sourceURL)
	if err != nil {
		return domain.Relic{}, err
	}
	c.logger.Debug("Wiki 解析成功", "event", "parse_success", "type", "relic", "name", relic.Name)
	return relic, nil
}

func (c *Client) fetch(ctx context.Context, name string) ([]byte, string, error) {
	pageURL := c.baseURL + "/wiki/" + url.PathEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, pageURL, &Error{Kind: KindNetwork, Operation: "创建请求", URL: pageURL, Err: err}
	}
	req.Header.Set("User-Agent", "sts2bot/1.0 (+Telegram wiki lookup bot)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	c.logger.Debug("发起 Wiki 请求", "event", "wiki_request", "url", pageURL)
	started := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, pageURL, &Error{Kind: KindNetwork, Operation: "请求", URL: pageURL, Err: err}
	}
	defer resp.Body.Close()
	finalURL := resp.Request.URL.String()
	c.logger.Debug("Wiki 请求完成", "event", "wiki_response", "url", finalURL, "status_code", resp.StatusCode, "duration_ms", time.Since(started).Milliseconds())

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		kind := KindHTTPStatus
		switch {
		case resp.StatusCode == http.StatusNotFound:
			kind = KindNotFound
		case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
			kind = KindBlocked
		case resp.StatusCode == http.StatusTooManyRequests:
			kind = KindRateLimited
		case resp.StatusCode >= 500:
			kind = KindUpstream
		}
		return nil, finalURL, &Error{Kind: kind, Operation: "请求", URL: finalURL, StatusCode: resp.StatusCode, Err: fmt.Errorf("HTTP %d", resp.StatusCode)}
	}

	limited := io.LimitReader(resp.Body, maxBodySize+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, finalURL, &Error{Kind: KindNetwork, Operation: "读取响应", URL: finalURL, Err: err}
	}
	if len(body) > maxBodySize {
		return nil, finalURL, &Error{Kind: KindBodyTooLarge, Operation: "读取响应", URL: finalURL, Err: fmt.Errorf("响应超过 %d 字节", maxBodySize)}
	}
	if isChallengePage(body) {
		return nil, finalURL, &Error{Kind: KindBlocked, Operation: "请求", URL: finalURL, Err: fmt.Errorf("Wiki 返回了访问验证页面")}
	}
	return body, finalURL, nil
}

func isChallengePage(body []byte) bool {
	text := strings.ToLower(string(body))
	markers := []string{"<title>just a moment...</title>", "cf-chl-", "challenge-platform", "cdn-cgi/challenge-platform"}
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
