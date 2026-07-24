package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"sts2bot/domain"
	"sts2bot/wiki"
)

type fakeLookup struct {
	card   domain.Card
	relic  domain.Relic
	enemy  domain.Enemy
	potion domain.Potion
	err    error
}

func (f fakeLookup) LookupCard(context.Context, string) (domain.Card, error) { return f.card, f.err }
func (f fakeLookup) LookupRelic(context.Context, string) (domain.Relic, error) {
	return f.relic, f.err
}
func (f fakeLookup) LookupEnemy(context.Context, string) (domain.Enemy, error) {
	return f.enemy, f.err
}
func (f fakeLookup) LookupPotion(context.Context, string) (domain.Potion, error) {
	return f.potion, f.err
}

func testHandler(lookup LookupService) *Handler {
	return New(lookup, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestHandlerHelp(t *testing.T) {
	h := testHandler(fakeLookup{})
	help := h.Handle(context.Background(), Request{Command: "help", Args: "ignored"})
	got := string(help.RichHTML)
	for _, command := range []string{"/card", "/relic", "/enemy", "/potion", "/help"} {
		if !strings.Contains(got, command) {
			t.Fatalf("帮助响应缺少 %q：%s", command, got)
		}
	}

	start := h.Handle(context.Background(), Request{Command: "start", Args: "ignored"})
	if start.RichHTML != help.RichHTML {
		t.Fatalf("/start 响应与 /help 不同：start=%q help=%q", start.RichHTML, help.RichHTML)
	}
}

func TestHandlerUsage(t *testing.T) {
	resp := testHandler(fakeLookup{}).Handle(context.Background(), Request{Command: "card"})
	if !strings.Contains(string(resp.RichHTML), "/card") || !strings.Contains(string(resp.RichHTML), "&lt;卡牌名称&gt;") {
		t.Fatalf("response = %+v", resp)
	}
}

func TestHandlerNotFound(t *testing.T) {
	err := &wiki.Error{Kind: wiki.KindNotFound, Err: errors.New("404")}
	resp := testHandler(fakeLookup{err: err}).Handle(context.Background(), Request{Command: "relic", Args: "不存在"})
	if !strings.Contains(string(resp.RichHTML), "未找到") || !strings.Contains(string(resp.RichHTML), "不存在") {
		t.Fatalf("response = %+v", resp)
	}
}

func TestHandlerEscapesRichHTMLInQueryName(t *testing.T) {
	err := &wiki.Error{Kind: wiki.KindNotFound, Err: errors.New("404")}
	resp := testHandler(fakeLookup{err: err}).Handle(context.Background(), Request{Command: "card", Args: `坏</b><script>&名字`})
	got := string(resp.RichHTML)
	if !strings.Contains(got, `坏&lt;/b&gt;&lt;script&gt;&amp;名字`) || strings.Contains(got, "<script>") {
		t.Fatalf("response = %+v", resp)
	}
}

func TestHandlerEnemyAndPotion(t *testing.T) {
	enemyResp := testHandler(fakeLookup{enemy: domain.Enemy{Name: "飞蝇菌子", ID: "FLYCONID", Health: "47~49", Type: "普通", FirstSeen: "密林", Moves: []domain.EnemyMove{{Name: "猛砸", Effect: "伤害", AdvancedEffect: "更多伤害"}}, SourceURL: "https://example.test"}}).Handle(context.Background(), Request{Command: "enemy", Args: "飞蝇菌子"})
	if !strings.Contains(string(enemyResp.RichHTML), "FLYCONID") {
		t.Fatalf("enemy response = %+v", enemyResp)
	}
	potionResp := testHandler(fakeLookup{potion: domain.Potion{Name: "爆炸安瓿", ID: "EXPLOSIVE_AMPOULE", Pool: "通用", Rarity: "普通", Description: "伤害", SourceURL: "https://example.test"}}).Handle(context.Background(), Request{Command: "potion", Args: "爆炸安瓿"})
	if !strings.Contains(string(potionResp.RichHTML), "EXPLOSIVE_AMPOULE") {
		t.Fatalf("potion response = %+v", potionResp)
	}
}

func TestHandlerSuccess(t *testing.T) {
	lookup := fakeLookup{card: domain.Card{Name: "打击", ID: "STRIKE", Character: "铁甲战士", Rarity: "初始", Cost: "1", Description: "伤害", UpgradedCost: "1", UpgradedDescription: "更多伤害", SourceURL: "https://example.test"}}
	resp := testHandler(lookup).Handle(context.Background(), Request{Command: "card", Args: "打击"})
	if !strings.Contains(string(resp.RichHTML), "STRIKE") {
		t.Fatalf("response = %+v", resp)
	}
}
