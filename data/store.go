package data

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"sts2bot/domain"
)

const (
	SchemaVersion  = 1
	FileName       = "wiki.toml"
	CheckpointName = "wiki.update.toml"
)

type File struct {
	SchemaVersion int             `toml:"schema_version"`
	Cards         []domain.Card   `toml:"cards"`
	Relics        []domain.Relic  `toml:"relics"`
	Enemies       []domain.Enemy  `toml:"enemies"`
	Potions       []domain.Potion `toml:"potions"`
}

type Store struct {
	directory      string
	path           string
	checkpointPath string
}

func NewStore(directory string) *Store {
	return &Store{
		directory:      directory,
		path:           filepath.Join(directory, FileName),
		checkpointPath: filepath.Join(directory, CheckpointName),
	}
}

func (s *Store) Path() string { return s.path }

func (s *Store) Prepare() error {
	if err := os.MkdirAll(s.directory, 0o750); err != nil {
		return fmt.Errorf("创建数据目录 %q: %w", s.directory, err)
	}
	info, err := os.Stat(s.directory)
	if err != nil {
		return fmt.Errorf("访问数据目录 %q: %w", s.directory, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("数据路径 %q 不是目录", s.directory)
	}
	matches, err := filepath.Glob(filepath.Join(s.directory, ".wiki-*.tmp"))
	if err != nil {
		return fmt.Errorf("扫描临时数据文件: %w", err)
	}
	for _, match := range matches {
		if err := os.Remove(match); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("清理临时数据文件 %q: %w", match, err)
		}
	}
	return nil
}

func (s *Store) LoadCurrent() (File, bool, error) {
	file, err := loadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return File{}, false, nil
	}
	if err != nil {
		return File{}, false, err
	}
	return file, true, nil
}

// LoadCheckpoint 加载上次中断同步已成功抓取的记录。
func (s *Store) LoadCheckpoint() (File, bool, error) {
	file, err := loadFile(s.checkpointPath)
	if errors.Is(err, os.ErrNotExist) {
		return File{}, false, nil
	}
	if err != nil {
		return File{}, false, err
	}
	return file, true, nil
}

// SaveCheckpoint 原子保存当前同步进度，不替换线上查询快照。
func (s *Store) SaveCheckpoint(file File) error {
	return s.write(file, s.checkpointPath)
}

func (s *Store) ClearCheckpoint() error {
	if err := os.Remove(s.checkpointPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("删除同步检查点 %q: %w", s.checkpointPath, err)
	}
	return nil
}

func (s *Store) Validate(file File) error {
	return validateFile(s.path, file)
}

func (s *Store) Publish(file File) error {
	return s.write(file, s.path)
}

func (s *Store) write(file File, targetPath string) error {
	if err := validateFile(targetPath, file); err != nil {
		return err
	}
	sortFile(&file)
	temporary, err := os.CreateTemp(s.directory, ".wiki-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时数据文件: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		if !committed {
			_ = temporary.Close()
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o640); err != nil {
		return fmt.Errorf("设置临时数据文件权限: %w", err)
	}
	if err := toml.NewEncoder(temporary).Encode(file); err != nil {
		return fmt.Errorf("编码本地数据: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("同步临时数据文件: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("关闭临时数据文件: %w", err)
	}
	if _, err := loadFile(temporaryPath); err != nil {
		return fmt.Errorf("复验临时数据文件: %w", err)
	}
	if err := os.Rename(temporaryPath, targetPath); err != nil {
		return fmt.Errorf("发布本地数据: %w", err)
	}
	committed = true
	return nil
}

func loadFile(path string) (File, error) {
	var file File
	metadata, err := toml.DecodeFile(path, &file)
	if err != nil {
		return File{}, fmt.Errorf("读取本地数据 %q: %w", path, err)
	}
	if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for index, key := range undecoded {
			keys[index] = key.String()
		}
		return File{}, fmt.Errorf("本地数据 %q 包含未知字段: %s", path, strings.Join(keys, ", "))
	}
	if err := validateFile(path, file); err != nil {
		return File{}, err
	}
	return file, nil
}

func validateFile(path string, file File) error {
	if file.SchemaVersion != SchemaVersion {
		return fmt.Errorf("本地数据 %q schema_version 必须为 %d", path, SchemaVersion)
	}
	if err := validateCards(path, file.Cards); err != nil {
		return err
	}
	if err := validateRelics(path, file.Relics); err != nil {
		return err
	}
	if err := validateEnemies(path, file.Enemies); err != nil {
		return err
	}
	return validatePotions(path, file.Potions)
}

func require(path, kind string, index int, name, field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("本地数据 %q %s[%d] %q 缺少字段 %s", path, kind, index, name, field)
	}
	return nil
}

func unique(path, kind string, index int, name, id string, names, ids map[string]struct{}) error {
	normalized := domain.NormalizeName(name)
	if normalized == "" {
		return fmt.Errorf("本地数据 %q %s[%d] 名称为空", path, kind, index)
	}
	if _, exists := names[normalized]; exists {
		return fmt.Errorf("本地数据 %q %s[%d] %q 名称重复", path, kind, index, name)
	}
	names[normalized] = struct{}{}
	trimmedID := strings.TrimSpace(id)
	if _, exists := ids[trimmedID]; exists {
		return fmt.Errorf("本地数据 %q %s[%d] %q ID %q 重复", path, kind, index, name, trimmedID)
	}
	ids[trimmedID] = struct{}{}
	return nil
}

func validateCards(path string, items []domain.Card) error {
	names, ids := map[string]struct{}{}, map[string]struct{}{}
	for index, item := range items {
		for field, value := range map[string]string{"name": item.Name, "id": item.ID, "character": item.Character, "rarity": item.Rarity, "cost": item.Cost, "description": item.Description, "source_url": item.SourceURL} {
			if err := require(path, "cards", index, item.Name, field, value); err != nil {
				return err
			}
		}
		if (strings.TrimSpace(item.UpgradedCost) == "") != (strings.TrimSpace(item.UpgradedDescription) == "") {
			return fmt.Errorf("本地数据 %q cards[%d] %q 的 upgraded_cost 与 upgraded_description 必须同时填写或同时留空", path, index, item.Name)
		}
		if err := unique(path, "cards", index, item.Name, item.ID, names, ids); err != nil {
			return err
		}
	}
	return nil
}

func validateRelics(path string, items []domain.Relic) error {
	names, ids := map[string]struct{}{}, map[string]struct{}{}
	for index, item := range items {
		for field, value := range map[string]string{"name": item.Name, "id": item.ID, "pool": item.Pool, "rarity": item.Rarity, "description": item.Description, "source_url": item.SourceURL} {
			if err := require(path, "relics", index, item.Name, field, value); err != nil {
				return err
			}
		}
		if err := unique(path, "relics", index, item.Name, item.ID, names, ids); err != nil {
			return err
		}
	}
	return nil
}

func validateEnemies(path string, items []domain.Enemy) error {
	names, ids := map[string]struct{}{}, map[string]struct{}{}
	for index, item := range items {
		for field, value := range map[string]string{"name": item.Name, "id": item.ID, "health": item.Health, "type": item.Type, "source_url": item.SourceURL} {
			if err := require(path, "enemies", index, item.Name, field, value); err != nil {
				return err
			}
		}
		if err := unique(path, "enemies", index, item.Name, item.ID, names, ids); err != nil {
			return err
		}
	}
	return nil
}

func validatePotions(path string, items []domain.Potion) error {
	names, ids := map[string]struct{}{}, map[string]struct{}{}
	for index, item := range items {
		for field, value := range map[string]string{"name": item.Name, "id": item.ID, "pool": item.Pool, "rarity": item.Rarity, "description": item.Description, "source_url": item.SourceURL} {
			if err := require(path, "potions", index, item.Name, field, value); err != nil {
				return err
			}
		}
		if err := unique(path, "potions", index, item.Name, item.ID, names, ids); err != nil {
			return err
		}
	}
	return nil
}

func sortFile(file *File) {
	sort.Slice(file.Cards, func(i, j int) bool {
		return less(file.Cards[i].Name, file.Cards[i].ID, file.Cards[j].Name, file.Cards[j].ID)
	})
	sort.Slice(file.Relics, func(i, j int) bool {
		return less(file.Relics[i].Name, file.Relics[i].ID, file.Relics[j].Name, file.Relics[j].ID)
	})
	sort.Slice(file.Enemies, func(i, j int) bool {
		return less(file.Enemies[i].Name, file.Enemies[i].ID, file.Enemies[j].Name, file.Enemies[j].ID)
	})
	sort.Slice(file.Potions, func(i, j int) bool {
		return less(file.Potions[i].Name, file.Potions[i].ID, file.Potions[j].Name, file.Potions[j].ID)
	})
}

func less(leftName, leftID, rightName, rightID string) bool {
	left, right := domain.NormalizeName(leftName), domain.NormalizeName(rightName)
	if left == right {
		return leftID < rightID
	}
	return left < right
}
