package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const defaultTTLHours = 72

// Config 是应用程序的完整配置。
type Config struct {
	Telegram TelegramConfig `toml:"telegram"`
	Log      LogConfig      `toml:"log"`
	Cache    CacheConfig    `toml:"cache"`
	Wiki     WikiConfig     `toml:"wiki"`
}

type TelegramConfig struct {
	Token string `toml:"token"`
}

type LogConfig struct {
	Level string `toml:"level"`
}

type CacheConfig struct {
	TTLHours int `toml:"ttl_hours"`
}

type WikiConfig struct {
	TLSProfile string `toml:"tls_profile"`
}

// Load 从 TOML 文件加载配置并验证所有字段。
func Load(path string) (Config, error) {
	cfg := Config{
		Log:   LogConfig{Level: "info"},
		Cache: CacheConfig{TTLHours: defaultTTLHours},
		Wiki:  WikiConfig{TLSProfile: "safari-16.0"},
	}

	metadata, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("读取配置文件 %q: %w", path, err)
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
	cfg.Wiki.TLSProfile = strings.ToLower(strings.TrimSpace(cfg.Wiki.TLSProfile))
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

	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("不支持的 log.level %q，可选值为 debug、info、warn、error", c.Log.Level)
	}

	if c.Cache.TTLHours <= 0 {
		return fmt.Errorf("cache.ttl_hours 必须大于 0")
	}
	if c.Wiki.TLSProfile == "" {
		return fmt.Errorf("wiki.tls_profile 未配置")
	}
	return nil
}

// TTL 返回配置的缓存有效时间。
func (c Config) TTL() time.Duration {
	return time.Duration(c.Cache.TTLHours) * time.Hour
}
