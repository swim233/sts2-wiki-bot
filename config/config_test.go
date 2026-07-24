package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad(t *testing.T) {
	path := writeConfig(t, `[telegram]
token = "123456:token"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Log.Level != "info" || cfg.Cache.TTLHours != 72 || cfg.Wiki.TLSProfile != "safari-16.0" {
		t.Fatalf("默认值错误: %+v", cfg)
	}
	if cfg.TTL() != 72*time.Hour {
		t.Fatalf("TTL() = %v", cfg.TTL())
	}
}

func TestLoadValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"占位 token", `[telegram]
token = "YOUR_BOT_TOKEN_HERE"`, "telegram.token"},
		{"非法日志级别", `[telegram]
token = "x"
[log]
level = "trace"`, "log.level"},
		{"非正数 TTL", `[telegram]
token = "x"
[cache]
ttl_hours = 0`, "ttl_hours"},
		{"未知字段", `[telegram]
token = "x"
[cache]
ttl_hour = 5`, "未知字段"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(writeConfig(t, tt.content))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load() error = %v，期望包含 %q", err, tt.want)
			}
		})
	}
}
