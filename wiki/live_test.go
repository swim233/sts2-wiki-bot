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
	httpClient, err := NewHTTPClient(DefaultBaseURL, profile, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(DefaultBaseURL, httpClient, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	card, err := client.GetCard(ctx, "打击")
	if err != nil {
		t.Fatalf("profile %q 请求失败: %v", profile, err)
	}
	if card.Name == "" || card.ID == "" || card.Description == "" {
		t.Fatalf("profile %q 返回字段不完整: %+v", profile, card)
	}
	t.Logf("profile %q 成功获取卡牌 %q（%s）", profile, card.Name, card.ID)
}
