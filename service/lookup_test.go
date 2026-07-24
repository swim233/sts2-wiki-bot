package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"sts2bot/cache"
	"sts2bot/domain"
)

type fakeSource struct {
	mu          sync.Mutex
	cardCalls   int
	relicCalls  int
	enemyCalls  int
	potionCalls int
	err         error
}

func (f *fakeSource) GetCard(_ context.Context, name string) (domain.Card, error) {
	f.mu.Lock()
	f.cardCalls++
	f.mu.Unlock()
	if f.err != nil {
		return domain.Card{}, f.err
	}
	return domain.Card{Name: name}, nil
}

func (f *fakeSource) GetEnemy(_ context.Context, name string) (domain.Enemy, error) {
	f.mu.Lock()
	f.enemyCalls++
	f.mu.Unlock()
	if f.err != nil {
		return domain.Enemy{}, f.err
	}
	return domain.Enemy{Name: name}, nil
}

func (f *fakeSource) GetPotion(_ context.Context, name string) (domain.Potion, error) {
	f.mu.Lock()
	f.potionCalls++
	f.mu.Unlock()
	if f.err != nil {
		return domain.Potion{}, f.err
	}
	return domain.Potion{Name: name}, nil
}

func (f *fakeSource) GetRelic(_ context.Context, name string) (domain.Relic, error) {
	f.mu.Lock()
	f.relicCalls++
	f.mu.Unlock()
	if f.err != nil {
		return domain.Relic{}, f.err
	}
	return domain.Relic{Name: name}, nil
}

func newLookupForTest(source *fakeSource) *Lookup {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewLookup(source,
		cache.New[domain.Card](time.Hour), cache.New[domain.Relic](time.Hour),
		cache.New[domain.Enemy](time.Hour), cache.New[domain.Potion](time.Hour), logger)
}

func TestLookupCachesResult(t *testing.T) {
	source := &fakeSource{}
	lookup := newLookupForTest(source)
	for range 2 {
		card, err := lookup.LookupCard(context.Background(), "  打击  ")
		if err != nil || card.Name != "打击" {
			t.Fatalf("LookupCard() = %+v, %v", card, err)
		}
	}
	if source.cardCalls != 1 {
		t.Fatalf("source calls = %d", source.cardCalls)
	}
}

func TestLookupCachesEnemy(t *testing.T) {
	source := &fakeSource{}
	lookup := newLookupForTest(source)
	for range 2 {
		enemy, err := lookup.LookupEnemy(context.Background(), "  飞蝇菌子 ")
		if err != nil || enemy.Name != "飞蝇菌子" {
			t.Fatalf("LookupEnemy() = %+v, %v", enemy, err)
		}
	}
	if source.enemyCalls != 1 {
		t.Fatalf("enemy source calls = %d", source.enemyCalls)
	}
}

func TestLookupDoesNotCacheErrors(t *testing.T) {
	source := &fakeSource{err: errors.New("失败")}
	lookup := newLookupForTest(source)
	for range 2 {
		_, _ = lookup.LookupRelic(context.Background(), "遗物")
	}
	if source.relicCalls != 2 {
		t.Fatalf("source calls = %d", source.relicCalls)
	}
}
