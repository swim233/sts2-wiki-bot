package service

import (
	"context"
	"errors"
	"testing"

	"sts2bot/domain"
)

func TestLookupReadsSnapshot(t *testing.T) {
	lookup := NewLookup(NewSnapshot(
		[]domain.Card{{Name: "计划 妥当", ID: "WELL_LAID_PLANS"}},
		nil,
		[]domain.Enemy{{Name: "飞蝇菌子", ID: "FLYCONID"}},
		nil,
	))
	card, err := lookup.LookupCard(context.Background(), "  计划   妥当 ")
	if err != nil || card.ID != "WELL_LAID_PLANS" {
		t.Fatalf("LookupCard() = %+v, %v", card, err)
	}
	enemy, err := lookup.LookupEnemy(context.Background(), "飞蝇菌子")
	if err != nil || enemy.ID != "FLYCONID" {
		t.Fatalf("LookupEnemy() = %+v, %v", enemy, err)
	}
}

func TestLookupErrors(t *testing.T) {
	lookup := NewLookup(nil)
	if _, err := lookup.LookupCard(context.Background(), "打击"); !errors.Is(err, ErrUninitialized) {
		t.Fatalf("error = %v", err)
	}
	lookup.Publish(NewSnapshot(nil, nil, nil, nil))
	if _, err := lookup.LookupRelic(context.Background(), "不存在"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}
