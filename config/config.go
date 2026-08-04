package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Config 是应用程序的完整配置。
type Config struct {
	Telegram TelegramConfig `toml:"telegram"`
	Log      LogConfig      `toml:"log"`
	Data     DataConfig     `toml:"data"`
	Wiki     WikiConfig     `toml:"wiki"`
}

type TelegramConfig struct {
	Token   string `toml:"token"`
	OwnerID int64  `toml:"owner_id"`
}

type LogConfig struct {
	Level string `toml:"level"`
}

type DataConfig struct {
	Directory string `toml:"directory"`
}

type WikiConfig struct {
	TLSProfile        string `toml:"tls_profile"`
	RequestIntervalMS int    `toml:"request_interval_ms"`
}

// Load 从 TOML 文件加载配置并验证所有字段。
func Load(path string) (Config, error) {
	configPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return Config{}, fmt.Errorf("解析配置文件路径 %q: %w", path, err)
	}
	cfg := Config{
		Log:  LogConfig{Level: "info"},
		Data: DataConfig{Directory: "data"},
		Wiki: WikiConfig{TLSProfile: "safari-16.0", RequestIntervalMS: 1000},
	}

	metadata, err := toml.DecodeFile(configPath, &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("读取配置文件 %q: %w", configPath, err)
	}
	if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, 0, len(undecoded))
		for _, key := range undecoded {
			keys = append(keys, key.String())
		}
		return Config{}, fmt.Errorf("配置文件包含未知字段: %s", strings.Join(keys, ", "))
	}

	cfg.Telegram.Token = strings.TrimSpace(cfg.Telegram.Token)
	cfg.Log.Level = strings.ToLower(strings.TrimSpace(cfg.Log.Level))
	cfg.Data.Directory = strings.TrimSpace(cfg.Data.Directory)
	cfg.Wiki.TLSProfile = strings.ToLower(strings.TrimSpace(cfg.Wiki.TLSProfile))
	if cfg.Data.Directory != "" && !filepath.IsAbs(cfg.Data.Directory) {
		cfg.Data.Directory = filepath.Join(filepath.Dir(configPath), cfg.Data.Directory)
	}
	cfg.Data.Directory = filepath.Clean(cfg.Data.Directory)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate 验证配置是否可用于启动应用。
func (c Config) Validate() error {
	if c.Telegram.Token == "" || c.Telegram.Token == "YOUR_BOT_TOKEN_HERE" {
		return fmt.Errorf("telegram.token 未配置")
	}
	if c.Telegram.OwnerID <= 0 {
		return fmt.Errorf("telegram.owner_id 必须大于 0")
	}

	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("不支持的 log.level %q，可选值为 debug、info、warn、error", c.Log.Level)
	}

	if strings.TrimSpace(c.Data.Directory) == "" || c.Data.Directory == "." {
		return fmt.Errorf("data.directory 未配置")
	}
	if c.Wiki.TLSProfile == "" {
		return fmt.Errorf("wiki.tls_profile 未配置")
	}
	if c.Wiki.RequestIntervalMS <= 0 {
		return fmt.Errorf("wiki.request_interval_ms 必须大于 0")
	}
	return nil
}

// RequestInterval 返回相邻 Wiki 请求的最小间隔。
func (c Config) RequestInterval() time.Duration {
	return time.Duration(c.Wiki.RequestIntervalMS) * time.Millisecond
}
