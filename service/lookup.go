package service

import (
	"context"
	"log/slog"
	"strings"

	"golang.org/x/sync/singleflight"

	"sts2bot/domain"
)

type Store[T any] interface {
	Get(key string) (T, bool)
	Set(key string, value T)
}

type CardSource interface {
	GetCard(ctx context.Context, name string) (domain.Card, error)
}

type RelicSource interface {
	GetRelic(ctx context.Context, name string) (domain.Relic, error)
}

type EnemySource interface {
	GetEnemy(ctx context.Context, name string) (domain.Enemy, error)
}

type PotionSource interface {
	GetPotion(ctx context.Context, name string) (domain.Potion, error)
}

// Lookup 负责缓存、并发请求合并和 Wiki 数据源调用。
type Lookup struct {
	source interface {
		CardSource
		RelicSource
		EnemySource
		PotionSource
	}
	cards       Store[domain.Card]
	relics      Store[domain.Relic]
	enemies     Store[domain.Enemy]
	potions     Store[domain.Potion]
	logger      *slog.Logger
	cardGroup   singleflight.Group
	relicGroup  singleflight.Group
	enemyGroup  singleflight.Group
	potionGroup singleflight.Group
}

func NewLookup(source interface {
	CardSource
	RelicSource
	EnemySource
	PotionSource
}, cards Store[domain.Card], relics Store[domain.Relic], enemies Store[domain.Enemy], potions Store[domain.Potion], logger *slog.Logger) *Lookup {
	if logger == nil {
		logger = slog.Default()
	}
	return &Lookup{source: source, cards: cards, relics: relics, enemies: enemies, potions: potions, logger: logger}
}

func NormalizeName(name string) string {
	return strings.Join(strings.Fields(name), " ")
}

func (s *Lookup) LookupCard(ctx context.Context, name string) (domain.Card, error) {
	name = NormalizeName(name)
	key := "card:" + name
	if card, ok := s.cards.Get(key); ok {
		s.logger.Debug("缓存命中", "event", "cache_hit", "cache_key", key)
		return card, nil
	}

	result, err, _ := s.cardGroup.Do(key, func() (any, error) {
		if card, ok := s.cards.Get(key); ok {
			return card, nil
		}
		card, err := s.source.GetCard(ctx, name)
		if err != nil {
			return domain.Card{}, err
		}
		s.cards.Set(key, card)
		return card, nil
	})
	if err != nil {
		return domain.Card{}, err
	}
	return result.(domain.Card), nil
}

func (s *Lookup) LookupEnemy(ctx context.Context, name string) (domain.Enemy, error) {
	name = NormalizeName(name)
	key := "enemy:" + name
	if enemy, ok := s.enemies.Get(key); ok {
		s.logger.Debug("缓存命中", "event", "cache_hit", "cache_key", key)
		return enemy, nil
	}
	result, err, _ := s.enemyGroup.Do(key, func() (any, error) {
		if enemy, ok := s.enemies.Get(key); ok {
			return enemy, nil
		}
		enemy, err := s.source.GetEnemy(ctx, name)
		if err != nil {
			return domain.Enemy{}, err
		}
		s.enemies.Set(key, enemy)
		return enemy, nil
	})
	if err != nil {
		return domain.Enemy{}, err
	}
	return result.(domain.Enemy), nil
}

func (s *Lookup) LookupPotion(ctx context.Context, name string) (domain.Potion, error) {
	name = NormalizeName(name)
	key := "potion:" + name
	if potion, ok := s.potions.Get(key); ok {
		s.logger.Debug("缓存命中", "event", "cache_hit", "cache_key", key)
		return potion, nil
	}
	result, err, _ := s.potionGroup.Do(key, func() (any, error) {
		if potion, ok := s.potions.Get(key); ok {
			return potion, nil
		}
		potion, err := s.source.GetPotion(ctx, name)
		if err != nil {
			return domain.Potion{}, err
		}
		s.potions.Set(key, potion)
		return potion, nil
	})
	if err != nil {
		return domain.Potion{}, err
	}
	return result.(domain.Potion), nil
}

func (s *Lookup) LookupRelic(ctx context.Context, name string) (domain.Relic, error) {
	name = NormalizeName(name)
	key := "relic:" + name
	if relic, ok := s.relics.Get(key); ok {
		s.logger.Debug("缓存命中", "event", "cache_hit", "cache_key", key)
		return relic, nil
	}

	result, err, _ := s.relicGroup.Do(key, func() (any, error) {
		if relic, ok := s.relics.Get(key); ok {
			return relic, nil
		}
		relic, err := s.source.GetRelic(ctx, name)
		if err != nil {
			return domain.Relic{}, err
		}
		s.relics.Set(key, relic)
		return relic, nil
	})
	if err != nil {
		return domain.Relic{}, err
	}
	return result.(domain.Relic), nil
}
