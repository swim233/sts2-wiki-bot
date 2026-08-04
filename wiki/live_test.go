package wiki

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestLiveWikiTLSProfile(t *testing.T) {
	if os.Getenv("STS2BOT_LIVE_WIKI") != "1" {
		t.Skip("设置 STS2BOT_LIVE_WIKI=1 后运行一次授权 Wiki smoke test")
	}
	profileName := os.Getenv("STS2BOT_LIVE_TLS_PROFILE")
	if profileName == "" {
		profileName = string(TLSProfileSafari160)
	}
	profile, err := ParseTLSProfile(profileName)
	if err != nil {
		t.Fatal(err)
	}
	httpClient, err := NewHTTPClient(DefaultBaseURL, profile, 15*time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(DefaultBaseURL, httpClient, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	for _, kind := range []EntityKind{EntityCard, EntityRelic, EntityEnemy, EntityPotion} {
		items, err := client.List(ctx, kind)
		if err != nil {
			t.Fatalf("profile %q 枚举 %q 失败: %v", profile, kind, err)
		}
		if len(items) == 0 {
			t.Fatalf("profile %q 枚举 %q 返回空列表", profile, kind)
		}
		t.Logf("profile %q 成功枚举 %q：%d 项", profile, kind, len(items))
	}
	card, err := client.GetCard(ctx, "打击")
	if err != nil {
		t.Fatalf("profile %q 请求失败: %v", profile, err)
	}
	if card.Name == "" || card.ID == "" || card.Description == "" {
		t.Fatalf("profile %q 返回字段不完整: %+v", profile, card)
	}
	t.Logf("profile %q 成功获取卡牌 %q（%s）", profile, card.Name, card.ID)
	for _, name := range []string{"伤口", "债务", "多尼斯异鸟蛋", "灯火钥匙(卡牌)"} {
		card, err := client.GetCard(ctx, name)
		if err != nil {
			t.Fatalf("profile %q 请求特殊卡牌 %q 失败: %v", profile, name, err)
		}
		if card.Name == "" || card.ID == "" || card.Character == "" || card.Rarity == "" || card.Cost == "" || card.Description == "" {
			t.Fatalf("profile %q 特殊卡牌 %q 字段不完整: %+v", profile, name, card)
		}
		if card.UpgradedCost != "" || card.UpgradedDescription != "" {
			t.Fatalf("profile %q 特殊卡牌 %q 不应有升级字段: %+v", profile, name, card)
		}
	}
}
