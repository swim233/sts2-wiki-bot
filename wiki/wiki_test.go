package wiki

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()
	file, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func TestParseCard(t *testing.T) {
	tests := []struct {
		fixture  string
		wantDesc string
	}{
		{"card_standard.html", "造成6点伤害。"},
		{"card_variant.html", "造成 6 点伤害。\n第二行"},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			card, err := parseCard(openFixture(t, tt.fixture), "打击", "https://example.test/wiki/card")
			if err != nil {
				t.Fatalf("parseCard() error = %v", err)
			}
			if card.ID != "STRIKE_IRONCLAD" || card.UpgradedCost != "1" || card.Description != tt.wantDesc {
				t.Fatalf("parseCard() = %+v", card)
			}
		})
	}
}

func TestParseEnemy(t *testing.T) {
	enemy, err := parseEnemy(openFixture(t, "enemy_standard.html"), "飞蝇菌子", "https://example.test/wiki/enemy")
	if err != nil {
		t.Fatal(err)
	}
	if enemy.ID != "FLYCONID" || enemy.Health != "47~49\n51~53" || enemy.FirstSeen != "密林" || len(enemy.Moves) != 3 {
		t.Fatalf("parseEnemy() = %+v", enemy)
	}
	if enemy.Moves[0].EnglishName != "Frail Spores" || enemy.Moves[1].Effect != "造成11点伤害。" || enemy.BehaviorRule == "" {
		t.Fatalf("moves = %+v rule=%q", enemy.Moves, enemy.BehaviorRule)
	}
}

func TestParsePotion(t *testing.T) {
	potion, err := parsePotion(openFixture(t, "potion_standard.html"), "爆炸安瓿", "https://example.test/wiki/potion")
	if err != nil {
		t.Fatal(err)
	}
	if potion.ID != "EXPLOSIVE_AMPOULE" || potion.Pool != "通用" || potion.Description != "对所有敌人造成10点伤害。" {
		t.Fatalf("parsePotion() = %+v", potion)
	}
}

func TestParseRelic(t *testing.T) {
	relic, err := parseRelic(openFixture(t, "relic_standard.html"), "燃烧之血", "https://example.test")
	if err != nil {
		t.Fatal(err)
	}
	if relic.ID != "BURNING_BLOOD" || relic.FlavorText == "" {
		t.Fatalf("parseRelic() = %+v", relic)
	}

	withoutFlavor, err := parseRelic(openFixture(t, "relic_without_flavor.html"), "锚", "https://example.test")
	if err != nil {
		t.Fatal(err)
	}
	if withoutFlavor.FlavorText != "" {
		t.Fatalf("FlavorText = %q", withoutFlavor.FlavorText)
	}
}

func TestParseMissingFields(t *testing.T) {
	_, err := parseCard(openFixture(t, "missing_fields.html"), "不完整", "https://example.test")
	if !IsKind(err, KindParse) {
		t.Fatalf("error = %v", err)
	}
	var wikiErr *Error
	if !errors.As(err, &wikiErr) || len(wikiErr.Missing) == 0 {
		t.Fatalf("missing = %#v", wikiErr)
	}
}

func TestClientStatusAndHeaders(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.EscapedPath()
		if r.Header.Get("User-Agent") == "" || r.Header.Get("Accept-Language") == "" {
			t.Error("缺少必要请求头")
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.GetCard(context.Background(), "打击 测试")
	if !IsKind(err, KindNotFound) {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(requestedPath, "%E6%89%93%E5%87%BB%20%E6%B5%8B%E8%AF%95") {
		t.Fatalf("path = %q", requestedPath)
	}
}

func TestClientDetectsChallenge(t *testing.T) {
	body, err := os.ReadFile("testdata/cloudflare_challenge.html")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, server.Client(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err = client.GetRelic(context.Background(), "遗物")
	if !IsKind(err, KindBlocked) {
		t.Fatalf("error = %v", err)
	}
}
