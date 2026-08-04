package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	localdata "sts2bot/data"
	"sts2bot/domain"
	"sts2bot/wiki"
)

type UpdateStage string

const (
	StageEnumerating UpdateStage = "enumerating"
	StageFetching    UpdateStage = "fetching"
	StageValidating  UpdateStage = "validating"
	StagePublishing  UpdateStage = "publishing"
	StageCompleted   UpdateStage = "completed"
	StageFailed      UpdateStage = "failed"
)

var ErrUpdateInProgress = errors.New("数据同步正在进行中")

type KindProgress struct{ Total, Completed, Succeeded, Failed int }
type UpdateCounts struct{ Cards, Relics, Enemies, Potions KindProgress }
type UpdateFailure struct {
	Kind         wiki.EntityKind
	Name, Reason string
}
type UpdateProgress struct {
	Stage    UpdateStage
	Kind     wiki.EntityKind
	Name     string
	Counts   UpdateCounts
	Failures []UpdateFailure
}
type UpdateSummary struct {
	Counts    UpdateCounts
	Failures  []UpdateFailure
	Published bool
}

type updateSource interface {
	List(context.Context, wiki.EntityKind) ([]wiki.PageRef, error)
	GetCard(context.Context, string) (domain.Card, error)
	GetRelic(context.Context, string) (domain.Relic, error)
	GetEnemy(context.Context, string) (domain.Enemy, error)
	GetPotion(context.Context, string) (domain.Potion, error)
}

type updateStore interface {
	Validate(localdata.File) error
	Publish(localdata.File) error
	LoadCheckpoint() (localdata.File, bool, error)
	SaveCheckpoint(localdata.File) error
	ClearCheckpoint() error
}

type Updater struct {
	source  updateSource
	store   updateStore
	lookup  *Lookup
	logger  *slog.Logger
	mu      sync.Mutex
	running bool
}

func NewUpdater(source updateSource, store updateStore, lookup *Lookup, logger ...*slog.Logger) *Updater {
	updateLogger := slog.Default()
	if len(logger) > 0 && logger[0] != nil {
		updateLogger = logger[0]
	}
	return &Updater{source: source, store: store, lookup: lookup, logger: updateLogger}
}

func (u *Updater) TryAcquire() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.running {
		return false
	}
	u.running = true
	return true
}

func (u *Updater) Release() {
	u.mu.Lock()
	u.running = false
	u.mu.Unlock()
}

func (u *Updater) Run(ctx context.Context, report func(UpdateProgress)) (UpdateSummary, error) {
	var summary UpdateSummary
	u.logger.Info("开始同步 Wiki 数据", "event", "wiki_update_started")
	emit := func(stage UpdateStage, kind wiki.EntityKind, name string) {
		if report == nil {
			return
		}
		failures := append([]UpdateFailure(nil), summary.Failures...)
		report(UpdateProgress{Stage: stage, Kind: kind, Name: name, Counts: summary.Counts, Failures: failures})
	}
	kinds := []wiki.EntityKind{wiki.EntityCard, wiki.EntityRelic, wiki.EntityEnemy, wiki.EntityPotion}
	lists := make(map[wiki.EntityKind][]wiki.PageRef, len(kinds))
	emit(StageEnumerating, "", "")
	listFailed := false
	for _, kind := range kinds {
		u.logger.Info("开始枚举 Wiki 数据", "event", "wiki_update_list_started", "kind", kind)
		items, err := u.source.List(ctx, kind)
		if err != nil {
			listFailed = true
			summary.Failures = append(summary.Failures, UpdateFailure{Kind: kind, Reason: safeUpdateReason(err)})
			u.logger.Error("枚举 Wiki 数据失败", "event", "wiki_update_list_failed", "kind", kind, "error", err)
			emit(StageEnumerating, kind, "")
			if ctx.Err() != nil {
				break
			}
			continue
		}
		lists[kind] = items
		u.logger.Info("枚举 Wiki 数据完成", "event", "wiki_update_list_completed", "kind", kind, "total", len(items))
		progress := summary.kind(kind)
		progress.Total = len(items)
		summary.setKind(kind, progress)
		emit(StageEnumerating, kind, "")
	}
	if listFailed || ctx.Err() != nil {
		emit(StageFailed, "", "")
		if ctx.Err() != nil {
			return summary, ctx.Err()
		}
		return summary, fmt.Errorf("枚举 Wiki 数据失败")
	}

	file := localdata.File{SchemaVersion: localdata.SchemaVersion}
	if checkpoint, exists, err := u.store.LoadCheckpoint(); err != nil {
		summary.Failures = append(summary.Failures, UpdateFailure{Reason: "storage"})
		emit(StageFailed, "", "")
		return summary, err
	} else if exists {
		file = checkpoint
		u.logger.Info("加载同步检查点", "event", "wiki_update_checkpoint_loaded", "cards", len(file.Cards), "relics", len(file.Relics), "enemies", len(file.Enemies), "potions", len(file.Potions))
	}
	file = keepListed(file, lists)
	lists = removeDuplicatePageIdentities(file, lists)
	for _, kind := range kinds {
		progress := summary.kind(kind)
		progress.Succeeded = countKind(file, kind)
		progress.Completed = progress.Succeeded
		summary.setKind(kind, progress)
	}
	emit(StageFetching, "", "")
	for _, kind := range kinds {
		completed := existingNames(file, kind)
		for _, ref := range lists[kind] {
			if _, exists := completed[NormalizeName(ref.Name)]; exists {
				u.logger.Debug("跳过检查点已有数据", "event", "wiki_update_item_skipped", "kind", kind, "name", ref.Name)
				continue
			}
			if err := ctx.Err(); err != nil {
				summary.Failures = append(summary.Failures, UpdateFailure{Kind: kind, Name: ref.Name, Reason: "canceled"})
				emit(StageFailed, kind, ref.Name)
				return summary, err
			}
			u.logger.Info("开始拉取 Wiki 数据", "event", "wiki_update_item_started", "kind", kind, "name", ref.Name, "completed", summary.kind(kind).Completed, "total", summary.kind(kind).Total)
			err := u.fetchOne(ctx, kind, ref.Name, &file)
			if err == nil {
				file = deduplicateByID(file, kind, ref.Name)
			}
			progress := summary.kind(kind)
			progress.Completed++
			if err != nil {
				progress.Failed++
				summary.Failures = append(summary.Failures, UpdateFailure{Kind: kind, Name: ref.Name, Reason: safeUpdateReason(err)})
				u.logger.Warn("拉取 Wiki 数据失败", "event", "wiki_update_item_failed", "kind", kind, "name", ref.Name, "reason", safeUpdateReason(err), "error", err)
			} else if err = u.store.SaveCheckpoint(file); err != nil {
				progress.Failed++
				summary.Failures = append(summary.Failures, UpdateFailure{Kind: kind, Name: ref.Name, Reason: "storage"})
				u.logger.Error("保存同步检查点失败", "event", "wiki_update_checkpoint_failed", "kind", kind, "name", ref.Name, "error", err)
			} else {
				progress.Succeeded++
				completed[NormalizeName(ref.Name)] = struct{}{}
				u.logger.Info("拉取并保存 Wiki 数据完成", "event", "wiki_update_item_completed", "kind", kind, "name", ref.Name, "completed", progress.Completed, "succeeded", progress.Succeeded, "total", progress.Total)
			}
			summary.setKind(kind, progress)
			emit(StageFetching, kind, ref.Name)
			if err != nil && summary.Failures[len(summary.Failures)-1].Reason == "storage" {
				emit(StageFailed, kind, ref.Name)
				return summary, err
			}
		}
	}
	if len(summary.Failures) > 0 {
		emit(StageFailed, "", "")
		return summary, fmt.Errorf("Wiki 详情抓取存在失败")
	}
	emit(StageValidating, "", "")
	u.logger.Info("开始校验 Wiki 数据", "event", "wiki_update_validation_started")
	if err := u.store.Validate(file); err != nil {
		summary.Failures = append(summary.Failures, UpdateFailure{Reason: "validation"})
		emit(StageFailed, "", "")
		u.logger.Error("校验 Wiki 数据失败", "event", "wiki_update_validation_failed", "error", err)
		return summary, err
	}
	emit(StagePublishing, "", "")
	u.logger.Info("Wiki 数据校验完成", "event", "wiki_update_validation_completed")
	u.logger.Info("开始发布 Wiki 数据", "event", "wiki_update_publish_started")
	if err := u.store.Publish(file); err != nil {
		summary.Failures = append(summary.Failures, UpdateFailure{Reason: "storage"})
		emit(StageFailed, "", "")
		u.logger.Error("发布 Wiki 数据失败", "event", "wiki_update_publish_failed", "error", err)
		return summary, err
	}
	u.lookup.Publish(NewSnapshot(file.Cards, file.Relics, file.Enemies, file.Potions))
	summary.Published = true
	_ = u.store.ClearCheckpoint()
	u.logger.Info("Wiki 数据同步完成", "event", "wiki_update_completed", "cards", len(file.Cards), "relics", len(file.Relics), "enemies", len(file.Enemies), "potions", len(file.Potions))
	emit(StageCompleted, "", "")
	return summary, nil
}

func keepListed(file localdata.File, lists map[wiki.EntityKind][]wiki.PageRef) localdata.File {
	allowed := func(kind wiki.EntityKind) map[string]struct{} {
		result := make(map[string]struct{}, len(lists[kind]))
		for _, ref := range lists[kind] {
			result[NormalizeName(ref.Name)] = struct{}{}
		}
		return result
	}
	cards := allowed(wiki.EntityCard)
	file.Cards = filterSlice(file.Cards, cards, func(item domain.Card) string { return item.Name })
	relics := allowed(wiki.EntityRelic)
	file.Relics = filterSlice(file.Relics, relics, func(item domain.Relic) string { return item.Name })
	enemies := allowed(wiki.EntityEnemy)
	file.Enemies = filterSlice(file.Enemies, enemies, func(item domain.Enemy) string { return item.Name })
	potions := allowed(wiki.EntityPotion)
	file.Potions = filterSlice(file.Potions, potions, func(item domain.Potion) string { return item.Name })
	return file
}

func removeDuplicatePageIdentities(file localdata.File, lists map[wiki.EntityKind][]wiki.PageRef) map[wiki.EntityKind][]wiki.PageRef {
	result := make(map[wiki.EntityKind][]wiki.PageRef, len(lists))
	for kind, refs := range lists {
		idsByTitle := existingIdentityByTitle(file, kind)
		knownIDs := make(map[string]string, len(idsByTitle))
		for title, id := range idsByTitle {
			knownIDs[id] = title
		}
		filtered := make([]wiki.PageRef, 0, len(refs))
		for _, ref := range refs {
			title := NormalizeName(ref.Name)
			if id := idsByTitle[title]; id != "" {
				knownIDs[id] = title
				filtered = append(filtered, ref)
				continue
			}
			parsedTitle := NormalizeName(strings.TrimSuffix(ref.Name, "(卡牌)"))
			if id := idsByTitle[parsedTitle]; id != "" && knownIDs[id] != title {
				continue
			}
			filtered = append(filtered, ref)
		}
		result[kind] = filtered
	}
	return result
}

func existingIdentityByTitle(file localdata.File, kind wiki.EntityKind) map[string]string {
	result := make(map[string]string)
	switch kind {
	case wiki.EntityCard:
		for _, item := range file.Cards {
			result[NormalizeName(item.Name)] = strings.TrimSpace(item.ID)
		}
	case wiki.EntityRelic:
		for _, item := range file.Relics {
			result[NormalizeName(item.Name)] = strings.TrimSpace(item.ID)
		}
	case wiki.EntityEnemy:
		for _, item := range file.Enemies {
			result[NormalizeName(item.Name)] = strings.TrimSpace(item.ID)
		}
	case wiki.EntityPotion:
		for _, item := range file.Potions {
			result[NormalizeName(item.Name)] = strings.TrimSpace(item.ID)
		}
	}
	return result
}

func deduplicateByID(file localdata.File, kind wiki.EntityKind, preferredName string) localdata.File {
	preferredName = NormalizeName(preferredName)
	switch kind {
	case wiki.EntityCard:
		file.Cards = keepUniqueID(file.Cards, preferredName, func(item domain.Card) (string, string) { return item.Name, item.ID })
	case wiki.EntityRelic:
		file.Relics = keepUniqueID(file.Relics, preferredName, func(item domain.Relic) (string, string) { return item.Name, item.ID })
	case wiki.EntityEnemy:
		file.Enemies = keepUniqueID(file.Enemies, preferredName, func(item domain.Enemy) (string, string) { return item.Name, item.ID })
	case wiki.EntityPotion:
		file.Potions = keepUniqueID(file.Potions, preferredName, func(item domain.Potion) (string, string) { return item.Name, item.ID })
	}
	return file
}

func keepUniqueID[T any](items []T, preferredName string, identity func(T) (string, string)) []T {
	preferredIDs := make(map[string]struct{})
	for _, item := range items {
		name, id := identity(item)
		if NormalizeName(name) == preferredName {
			preferredIDs[strings.TrimSpace(id)] = struct{}{}
		}
	}
	result := items[:0]
	for _, item := range items {
		name, id := identity(item)
		if _, duplicate := preferredIDs[strings.TrimSpace(id)]; duplicate && NormalizeName(name) != preferredName {
			continue
		}
		result = append(result, item)
	}
	return result
}

func filterSlice[T any](items []T, allowed map[string]struct{}, name func(T) string) []T {
	result := items[:0]
	for _, item := range items {
		if _, ok := allowed[NormalizeName(name(item))]; ok {
			result = append(result, item)
		}
	}
	return result
}

func existingNames(file localdata.File, kind wiki.EntityKind) map[string]struct{} {
	result := make(map[string]struct{})
	switch kind {
	case wiki.EntityCard:
		for _, item := range file.Cards {
			result[NormalizeName(item.Name)] = struct{}{}
		}
	case wiki.EntityRelic:
		for _, item := range file.Relics {
			result[NormalizeName(item.Name)] = struct{}{}
		}
	case wiki.EntityEnemy:
		for _, item := range file.Enemies {
			result[NormalizeName(item.Name)] = struct{}{}
		}
	case wiki.EntityPotion:
		for _, item := range file.Potions {
			result[NormalizeName(item.Name)] = struct{}{}
		}
	}
	return result
}

func countKind(file localdata.File, kind wiki.EntityKind) int {
	switch kind {
	case wiki.EntityCard:
		return len(file.Cards)
	case wiki.EntityRelic:
		return len(file.Relics)
	case wiki.EntityEnemy:
		return len(file.Enemies)
	case wiki.EntityPotion:
		return len(file.Potions)
	default:
		return 0
	}
}

func (u *Updater) fetchOne(ctx context.Context, kind wiki.EntityKind, name string, file *localdata.File) error {
	switch kind {
	case wiki.EntityCard:
		item, err := u.source.GetCard(ctx, name)
		if err == nil {
			if current := u.lookup.Current(); current != nil {
				if existing, ok := current.Cards[NormalizeName(item.Name)]; ok {
					if item.StarCost == "" {
						item.StarCost = existing.StarCost
					}
					if item.UpgradedStarCost == "" {
						item.UpgradedStarCost = existing.UpgradedStarCost
					}
				}
			}
			file.Cards = append(file.Cards, item)
		}
		return err
	case wiki.EntityRelic:
		item, err := u.source.GetRelic(ctx, name)
		if err == nil {
			file.Relics = append(file.Relics, item)
		}
		return err
	case wiki.EntityEnemy:
		item, err := u.source.GetEnemy(ctx, name)
		if err == nil {
			file.Enemies = append(file.Enemies, item)
		}
		return err
	case wiki.EntityPotion:
		item, err := u.source.GetPotion(ctx, name)
		if err == nil {
			file.Potions = append(file.Potions, item)
		}
		return err
	default:
		return fmt.Errorf("不支持的数据类型 %q", kind)
	}
}

func safeUpdateReason(err error) string {
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if wiki.IsKind(err, wiki.KindNotFound) {
		return "not found"
	}
	if wiki.IsKind(err, wiki.KindBlocked) {
		return "blocked"
	}
	if wiki.IsKind(err, wiki.KindRateLimited) {
		return "rate limited"
	}
	if wiki.IsKind(err, wiki.KindUpstream) || wiki.IsKind(err, wiki.KindHTTPStatus) {
		return "upstream"
	}
	if wiki.IsKind(err, wiki.KindNetwork) {
		return "network"
	}
	if wiki.IsKind(err, wiki.KindBodyTooLarge) {
		return "response too large"
	}
	if wiki.IsKind(err, wiki.KindParse) {
		return "parse"
	}
	return "storage"
}

func (s UpdateSummary) kind(kind wiki.EntityKind) KindProgress { return s.Counts.kind(kind) }
func (s *UpdateSummary) setKind(kind wiki.EntityKind, value KindProgress) {
	s.Counts.setKind(kind, value)
}
func (c UpdateCounts) kind(kind wiki.EntityKind) KindProgress {
	switch kind {
	case wiki.EntityCard:
		return c.Cards
	case wiki.EntityRelic:
		return c.Relics
	case wiki.EntityEnemy:
		return c.Enemies
	case wiki.EntityPotion:
		return c.Potions
	}
	return KindProgress{}
}
func (c *UpdateCounts) setKind(kind wiki.EntityKind, value KindProgress) {
	switch kind {
	case wiki.EntityCard:
		c.Cards = value
	case wiki.EntityRelic:
		c.Relics = value
	case wiki.EntityEnemy:
		c.Enemies = value
	case wiki.EntityPotion:
		c.Potions = value
	}
}
