package service

import (
	"context"
	"errors"
	"sync/atomic"

	"sts2bot/domain"
)

var (
	ErrUninitialized = errors.New("本地数据未初始化")
	ErrNotFound      = errors.New("本地数据未找到")
)

// Snapshot 是一次完整、不可变的本地 Wiki 数据快照。
type Snapshot struct {
	Cards   map[string]domain.Card
	Relics  map[string]domain.Relic
	Enemies map[string]domain.Enemy
	Potions map[string]domain.Potion
}

func NewSnapshot(cards []domain.Card, relics []domain.Relic, enemies []domain.Enemy, potions []domain.Potion) *Snapshot {
	snapshot := &Snapshot{
		Cards:   make(map[string]domain.Card, len(cards)),
		Relics:  make(map[string]domain.Relic, len(relics)),
		Enemies: make(map[string]domain.Enemy, len(enemies)),
		Potions: make(map[string]domain.Potion, len(potions)),
	}
	for _, item := range cards {
		snapshot.Cards[NormalizeName(item.Name)] = item
	}
	for _, item := range relics {
		snapshot.Relics[NormalizeName(item.Name)] = item
	}
	for _, item := range enemies {
		snapshot.Enemies[NormalizeName(item.Name)] = item
	}
	for _, item := range potions {
		snapshot.Potions[NormalizeName(item.Name)] = item
	}
	return snapshot
}

// Lookup 只查询当前内存快照，不访问文件或网络。
type Lookup struct {
	snapshot atomic.Pointer[Snapshot]
}

func NewLookup(snapshot *Snapshot) *Lookup {
	lookup := &Lookup{}
	lookup.Publish(snapshot)
	return lookup
}

func NormalizeName(name string) string {
	return domain.NormalizeName(name)
}

func (s *Lookup) Publish(snapshot *Snapshot) {
	s.snapshot.Store(snapshot)
}

func (s *Lookup) Current() *Snapshot {
	return s.snapshot.Load()
}

func (s *Lookup) LookupCard(_ context.Context, name string) (domain.Card, error) {
	snapshot := s.snapshot.Load()
	if snapshot == nil {
		return domain.Card{}, ErrUninitialized
	}
	item, ok := snapshot.Cards[NormalizeName(name)]
	if !ok {
		return domain.Card{}, ErrNotFound
	}
	return item, nil
}

func (s *Lookup) LookupRelic(_ context.Context, name string) (domain.Relic, error) {
	snapshot := s.snapshot.Load()
	if snapshot == nil {
		return domain.Relic{}, ErrUninitialized
	}
	item, ok := snapshot.Relics[NormalizeName(name)]
	if !ok {
		return domain.Relic{}, ErrNotFound
	}
	return item, nil
}

func (s *Lookup) LookupEnemy(_ context.Context, name string) (domain.Enemy, error) {
	snapshot := s.snapshot.Load()
	if snapshot == nil {
		return domain.Enemy{}, ErrUninitialized
	}
	item, ok := snapshot.Enemies[NormalizeName(name)]
	if !ok {
		return domain.Enemy{}, ErrNotFound
	}
	return item, nil
}

func (s *Lookup) LookupPotion(_ context.Context, name string) (domain.Potion, error) {
	snapshot := s.snapshot.Load()
	if snapshot == nil {
		return domain.Potion{}, ErrUninitialized
	}
	item, ok := snapshot.Potions[NormalizeName(name)]
	if !ok {
		return domain.Potion{}, ErrNotFound
	}
	return item, nil
}
