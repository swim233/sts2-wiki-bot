package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sts2bot/domain"
)

func validFile() File {
	return File{SchemaVersion: SchemaVersion,
		Cards:   []domain.Card{{Name: "打击", ID: "STRIKE", Character: "铁甲战士", Rarity: "初始", Cost: "1", Description: "造成6点伤害。", UpgradedCost: "1", UpgradedDescription: "造成9点伤害。", SourceURL: "https://example.test/打击"}},
		Relics:  []domain.Relic{{Name: "燃烧之血", ID: "BURNING_BLOOD", Pool: "铁甲战士", Rarity: "初始", Description: "回复生命。", SourceURL: "https://example.test/燃烧之血"}},
		Enemies: []domain.Enemy{{Name: "飞蝇菌子", ID: "FLYCONID", Health: "47", Type: "普通", FirstSeen: "密林", Moves: []domain.EnemyMove{{Name: "猛砸", Effect: "伤害"}}, SourceURL: "https://example.test/飞蝇菌子"}},
		Potions: []domain.Potion{{Name: "爆炸安瓿", ID: "EXPLOSIVE_AMPOULE", Pool: "通用", Rarity: "普通", Description: "造成伤害。", SourceURL: "https://example.test/爆炸安瓿"}},
	}
}

func TestStorePublishLoadAndManualEdit(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Prepare(); err != nil {
		t.Fatal(err)
	}
	file := validFile()
	file.Cards[0].Description = "第一行\n第二行"
	if err := store.Publish(file); err != nil {
		t.Fatal(err)
	}
	loaded, initialized, err := store.LoadCurrent()
	if err != nil || !initialized {
		t.Fatalf("LoadCurrent() initialized=%v err=%v", initialized, err)
	}
	if loaded.Cards[0].Description != file.Cards[0].Description {
		t.Fatalf("Description=%q", loaded.Cards[0].Description)
	}
	if info, err := os.Stat(store.Path()); err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("mode=%v err=%v", info.Mode().Perm(), err)
	}
}

func TestStoreStrictValidation(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Prepare(); err != nil {
		t.Fatal(err)
	}
	file := validFile()
	file.Cards = append(file.Cards, file.Cards[0])
	file.Cards[1].Name = "  打击  "
	if err := store.Validate(file); err == nil || !strings.Contains(err.Error(), "名称重复") {
		t.Fatalf("error=%v", err)
	}
	if err := os.WriteFile(store.Path(), []byte("schema_version=1\nunknown=true\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.LoadCurrent(); err == nil || !strings.Contains(err.Error(), "未知字段") {
		t.Fatalf("error=%v", err)
	}
}

func TestPrepareCleansTemporaryFiles(t *testing.T) {
	directory := t.TempDir()
	temporary := filepath.Join(directory, ".wiki-stale.tmp")
	if err := os.WriteFile(temporary, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := NewStore(directory).Prepare(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(temporary); !os.IsNotExist(err) {
		t.Fatalf("temporary still exists: %v", err)
	}
}

func TestCheckpointRoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Prepare(); err != nil {
		t.Fatal(err)
	}
	file := validFile()
	if err := store.SaveCheckpoint(file); err != nil {
		t.Fatal(err)
	}
	loaded, exists, err := store.LoadCheckpoint()
	if err != nil || !exists || len(loaded.Cards) != 1 {
		t.Fatalf("exists=%v loaded=%+v err=%v", exists, loaded, err)
	}
	if err := store.ClearCheckpoint(); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := store.LoadCheckpoint(); err != nil || exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}
}

func TestValidateCardWithoutUpgrade(t *testing.T) {
	file := validFile()
	file.Cards[0].Character = "状态"
	file.Cards[0].Rarity = "状态"
	file.Cards[0].Cost = "无"
	file.Cards[0].UpgradedCost = ""
	file.Cards[0].UpgradedDescription = ""
	if err := NewStore(t.TempDir()).Validate(file); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCardRejectsPartialUpgrade(t *testing.T) {
	file := validFile()
	file.Cards[0].UpgradedCost = ""
	if err := NewStore(t.TempDir()).Validate(file); err == nil {
		t.Fatal("expected partial upgrade validation error")
	}
}

func TestValidateIncompleteEnemyWithoutBehavior(t *testing.T) {
	file := validFile()
	file.Enemies[0].FirstSeen = ""
	file.Enemies[0].Moves = nil
	if err := NewStore(t.TempDir()).Validate(file); err != nil {
		t.Fatal(err)
	}
}

func TestValidateEnemyAllowsMissingBehaviorMetadata(t *testing.T) {
	file := validFile()
	file.Enemies[0].Moves = nil
	if err := NewStore(t.TempDir()).Validate(file); err != nil {
		t.Fatal(err)
	}
}
