package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	localdata "sts2bot/data"
	"sts2bot/domain"
	"sts2bot/wiki"
)

type fakeUpdateSource struct{ failName string }

func (f *fakeUpdateSource) List(_ context.Context, kind wiki.EntityKind) ([]wiki.PageRef, error) {
	return []wiki.PageRef{{PageID: 1, Name: string(kind), URL: "https://example.test"}}, nil
}
func (f *fakeUpdateSource) GetCard(_ context.Context, name string) (domain.Card, error) {
	if name == f.failName {
		return domain.Card{}, errors.New("fail")
	}
	return domain.Card{Name: name, ID: "CARD", Character: "角色", Rarity: "普通", Cost: "1", Description: "描述", UpgradedCost: "1", UpgradedDescription: "升级", SourceURL: "https://example.test"}, nil
}
func (f *fakeUpdateSource) GetRelic(_ context.Context, name string) (domain.Relic, error) {
	return domain.Relic{Name: name, ID: "RELIC", Pool: "通用", Rarity: "普通", Description: "描述", SourceURL: "https://example.test"}, nil
}
func (f *fakeUpdateSource) GetEnemy(_ context.Context, name string) (domain.Enemy, error) {
	return domain.Enemy{Name: name, ID: "ENEMY", Health: "1", Type: "普通", FirstSeen: "密林", Moves: []domain.EnemyMove{{Name: "行动"}}, SourceURL: "https://example.test"}, nil
}
func (f *fakeUpdateSource) GetPotion(_ context.Context, name string) (domain.Potion, error) {
	return domain.Potion{Name: name, ID: "POTION", Pool: "通用", Rarity: "普通", Description: "描述", SourceURL: "https://example.test"}, nil
}

type fakeUpdateStore struct {
	published     bool
	checkpoint    localdata.File
	hasCheckpoint bool
	saves         int
}

func (f *fakeUpdateStore) Validate(localdata.File) error { return nil }
func (f *fakeUpdateStore) Publish(localdata.File) error  { f.published = true; return nil }
func (f *fakeUpdateStore) LoadCheckpoint() (localdata.File, bool, error) {
	return f.checkpoint, f.hasCheckpoint, nil
}
func (f *fakeUpdateStore) SaveCheckpoint(file localdata.File) error {
	f.checkpoint = file
	f.hasCheckpoint = true
	f.saves++
	return nil
}
func (f *fakeUpdateStore) ClearCheckpoint() error { f.hasCheckpoint = false; return nil }

func TestUpdaterPublishesCompleteSnapshot(t *testing.T) {
	store := &fakeUpdateStore{}
	lookup := NewLookup(nil)
	updater := NewUpdater(&fakeUpdateSource{}, store, lookup)
	if !updater.TryAcquire() || updater.TryAcquire() {
		t.Fatal("admission failed")
	}
	defer updater.Release()
	var stages []UpdateStage
	summary, err := updater.Run(context.Background(), func(p UpdateProgress) { stages = append(stages, p.Stage) })
	if err != nil || !summary.Published || !store.published || lookup.Current() == nil || store.saves != 4 {
		t.Fatalf("summary=%+v published=%v saves=%d err=%v", summary, store.published, store.saves, err)
	}
	if summary.Counts.Cards.Succeeded != 1 || stages[len(stages)-1] != StageCompleted {
		t.Fatalf("counts=%+v stages=%v", summary.Counts, stages)
	}
}

func TestUpdaterFailurePreservesSnapshot(t *testing.T) {
	old := NewSnapshot([]domain.Card{{Name: "old"}}, nil, nil, nil)
	lookup := NewLookup(old)
	store := &fakeUpdateStore{}
	updater := NewUpdater(&fakeUpdateSource{failName: "card"}, store, lookup)
	_, err := updater.Run(context.Background(), nil)
	if err == nil || store.published || lookup.Current() != old {
		t.Fatalf("published=%v current=%p old=%p err=%v", store.published, lookup.Current(), old, err)
	}
}

func TestUpdaterResumesCheckpoint(t *testing.T) {
	store := &fakeUpdateStore{
		hasCheckpoint: true,
		checkpoint:    localdata.File{SchemaVersion: localdata.SchemaVersion, Cards: []domain.Card{{Name: "card", ID: "CARD", Character: "角色", Rarity: "普通", Cost: "1", Description: "描述", UpgradedCost: "1", UpgradedDescription: "升级", SourceURL: "https://example.test"}}},
	}
	lookup := NewLookup(nil)
	summary, err := NewUpdater(&fakeUpdateSource{}, store, lookup).Run(context.Background(), nil)
	if err != nil || !summary.Published || store.saves != 3 || summary.Counts.Cards.Succeeded != 1 {
		t.Fatalf("summary=%+v saves=%d err=%v", summary, store.saves, err)
	}
}

func TestUpdaterPreservesManualStarCosts(t *testing.T) {
	lookup := NewLookup(NewSnapshot([]domain.Card{{Name: "card", StarCost: "2", UpgradedStarCost: "3"}}, nil, nil, nil))
	store := &fakeUpdateStore{}
	_, err := NewUpdater(&fakeUpdateSource{}, store, lookup).Run(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	card := lookup.Current().Cards["card"]
	if card.StarCost != "2" || card.UpgradedStarCost != "3" {
		t.Fatalf("card=%+v", card)
	}
}

func TestRemoveDuplicateCardPageIdentities(t *testing.T) {
	file := localdata.File{Cards: []domain.Card{{Name: "播种", ID: "SOW"}}}
	lists := map[wiki.EntityKind][]wiki.PageRef{
		wiki.EntityCard: {
			{PageID: 1, Name: "播种"},
			{PageID: 2, Name: "播种(卡牌)"},
			{PageID: 3, Name: "伤口"},
		},
	}
	filtered := removeDuplicatePageIdentities(file, lists)[wiki.EntityCard]
	if len(filtered) != 2 || filtered[0].Name != "播种" || filtered[1].Name != "伤口" {
		t.Fatalf("filtered=%+v", filtered)
	}
}

func TestDeduplicateByIDPrefersFetchedTitle(t *testing.T) {
	file := localdata.File{Cards: []domain.Card{
		{Name: "播种", ID: "SOW"},
		{Name: "播种(卡牌)", ID: "SOW"},
	}}
	file = deduplicateByID(file, wiki.EntityCard, "播种(卡牌)")
	if len(file.Cards) != 1 || file.Cards[0].Name != "播种(卡牌)" {
		t.Fatalf("cards=%+v", file.Cards)
	}
}

type duplicateTitleSource struct{ fakeUpdateSource }

func (duplicateTitleSource) List(_ context.Context, kind wiki.EntityKind) ([]wiki.PageRef, error) {
	if kind == wiki.EntityCard {
		return []wiki.PageRef{{PageID: 1, Name: "播种"}, {PageID: 2, Name: "播种(卡牌)"}, {PageID: 3, Name: "伤口"}}, nil
	}
	return []wiki.PageRef{{PageID: 1, Name: string(kind)}}, nil
}

func (duplicateTitleSource) GetCard(_ context.Context, name string) (domain.Card, error) {
	if name == "伤口" {
		return domain.Card{Name: name, ID: "WOUND", Character: "状态", Rarity: "状态", Cost: "无", Description: "不能被打出。", SourceURL: "https://example.test/伤口"}, nil
	}
	return domain.Card{Name: name, ID: "SOW", Character: "角色", Rarity: "普通", Cost: "1", Description: "描述", UpgradedCost: "1", UpgradedDescription: "升级", SourceURL: "https://example.test/播种"}, nil
}

func TestUpdaterResumesSpecialCardsAndDuplicateTitles(t *testing.T) {
	store := &fakeUpdateStore{hasCheckpoint: true, checkpoint: localdata.File{SchemaVersion: localdata.SchemaVersion, Cards: []domain.Card{{Name: "播种", ID: "SOW", Character: "角色", Rarity: "普通", Cost: "1", Description: "描述", UpgradedCost: "1", UpgradedDescription: "升级", SourceURL: "https://example.test/播种"}}}}
	summary, err := NewUpdater(&duplicateTitleSource{}, store, NewLookup(nil)).Run(context.Background(), nil)
	if err != nil || !summary.Published || len(store.checkpoint.Cards) != 2 {
		t.Fatalf("summary=%+v cards=%+v err=%v", summary, store.checkpoint.Cards, err)
	}
	if store.checkpoint.Cards[1].Name != "伤口" || store.checkpoint.Cards[1].UpgradedCost != "" {
		t.Fatalf("special card=%+v", store.checkpoint.Cards[1])
	}
}

func TestUpdaterLogsFetchProgress(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	store := &fakeUpdateStore{}
	_, err := NewUpdater(&fakeUpdateSource{}, store, NewLookup(nil), logger).Run(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	logs := output.String()
	for _, event := range []string{
		"wiki_update_started",
		"wiki_update_list_started",
		"wiki_update_list_completed",
		"wiki_update_item_started",
		"wiki_update_item_completed",
		"wiki_update_validation_started",
		"wiki_update_validation_completed",
		"wiki_update_publish_started",
		"wiki_update_completed",
	} {
		if !strings.Contains(logs, "event="+event) {
			t.Fatalf("日志缺少 %s：%s", event, logs)
		}
	}
	if !strings.Contains(logs, "kind=card") || !strings.Contains(logs, "name=card") || !strings.Contains(logs, "completed=1") || !strings.Contains(logs, "total=1") {
		t.Fatalf("拉取进度字段不完整：%s", logs)
	}
}
