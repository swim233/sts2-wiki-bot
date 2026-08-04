package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"sts2bot/domain"
)

func TestOfflineLookupSmoke(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	_ = server
	lookup := NewLookup(NewSnapshot(
		[]domain.Card{{Name: "粒子墙", ID: "PARTICLE_WALL", StarCost: "2"}},
		[]domain.Relic{{Name: "燃烧之血", ID: "BURNING_BLOOD"}},
		[]domain.Enemy{{Name: "飞蝇菌子", ID: "FLYCONID"}},
		[]domain.Potion{{Name: "爆炸安瓿", ID: "EXPLOSIVE_AMPOULE"}},
	))
	card, cardErr := lookup.LookupCard(context.Background(), "粒子墙")
	relic, relicErr := lookup.LookupRelic(context.Background(), "燃烧之血")
	enemy, enemyErr := lookup.LookupEnemy(context.Background(), "飞蝇菌子")
	potion, potionErr := lookup.LookupPotion(context.Background(), "爆炸安瓿")
	if cardErr != nil || relicErr != nil || enemyErr != nil || potionErr != nil {
		t.Fatalf("errors=%v/%v/%v/%v", cardErr, relicErr, enemyErr, potionErr)
	}
	if card.StarCost != "2" || relic.ID != "BURNING_BLOOD" || enemy.ID != "FLYCONID" || potion.ID != "EXPLOSIVE_AMPOULE" {
		t.Fatalf("values=%+v/%+v/%+v/%+v", card, relic, enemy, potion)
	}
	if requests != 0 {
		t.Fatalf("offline lookup made %d requests", requests)
	}
}
