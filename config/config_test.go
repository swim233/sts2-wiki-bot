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
owner_id = 42
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	wantData := filepath.Join(filepath.Dir(path), "data")
	if cfg.Log.Level != "info" || cfg.Data.Directory != wantData || cfg.Wiki.TLSProfile != "safari-16.0" || cfg.Wiki.RequestIntervalMS != 1000 {
		t.Fatalf("默认值错误: %+v", cfg)
	}
	if cfg.RequestInterval() != time.Second {
		t.Fatalf("RequestInterval() = %v", cfg.RequestInterval())
	}
}

func TestLoadValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"占位 token", `[telegram]
token = "YOUR_BOT_TOKEN_HERE"
owner_id = 1`, "telegram.token"},
		{"非法 owner", `[telegram]
token = "x"
owner_id = 0`, "owner_id"},
		{"非法日志级别", `[telegram]
token = "x"
owner_id = 1
[log]
level = "trace"`, "log.level"},
		{"非正请求间隔", `[telegram]
token = "x"
owner_id = 1
[wiki]
request_interval_ms = 0`, "request_interval_ms"},
		{"未知字段", `[telegram]
token = "x"
owner_id = 1
[data]
directry = "data"`, "未知字段"},
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
