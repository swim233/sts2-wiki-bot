package logging

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHandlerFormat(t *testing.T) {
	var output bytes.Buffer
	handler := NewHandler(&output, Options{Level: slog.LevelDebug, Color: ColorNever})
	record := slog.NewRecord(time.Date(2026, 7, 24, 18, 15, 42, 0, time.Local), slog.LevelInfo, "Wiki 请求完成", 0)
	record.AddAttrs(slog.String("module", "wiki"), slog.String("event", "wiki_response"), slog.Int("status_code", 200))
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	want := `[wiki] 2026/07/24 - 18:15:42  INFO   unknown:0  Wiki 请求完成 event="wiki_response" status_code="200"` + "\n"
	if got := output.String(); got != want {
		t.Fatalf("日志 = %q，期望 %q", got, want)
	}
}

func TestHandlerColors(t *testing.T) {
	tests := []struct {
		level slog.Level
		color string
	}{
		{slog.LevelDebug, ansiCyan},
		{slog.LevelInfo, ansiGreen},
		{slog.LevelWarn, ansiYellow},
		{slog.LevelError, ansiRed},
	}
	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			var output bytes.Buffer
			handler := NewHandler(&output, Options{Level: slog.LevelDebug, Color: ColorAlways})
			record := slog.NewRecord(time.Date(2026, 7, 24, 18, 15, 42, 0, time.Local), tt.level, "message", 0)
			if err := handler.Handle(context.Background(), record); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), tt.color+strings.ToUpper(tt.level.String())+ansiReset) {
				t.Fatalf("颜色输出错误：%q", output.String())
			}
		})
	}
}

func TestColorModes(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "xterm")
	if NewHandler(io.Discard, Options{Color: ColorAuto}).colorEnabled {
		t.Fatal("NO_COLOR 下不应启用颜色")
	}
	if !NewHandler(io.Discard, Options{Color: ColorAlways}).colorEnabled {
		t.Fatal("ColorAlways 应启用颜色")
	}
	if NewHandler(io.Discard, Options{Color: ColorNever}).colorEnabled {
		t.Fatal("ColorNever 不应启用颜色")
	}

	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	if NewHandler(io.Discard, Options{Color: ColorAuto}).colorEnabled {
		t.Fatal("TERM=dumb 下不应启用颜色")
	}
}

func TestHandlerSourcePointsToCaller(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(NewHandler(&output, Options{Color: ColorNever}))
	logger.Info("来源测试", "module", "logging")
	got := output.String()
	if !regexp.MustCompile(`logging/handler_test\.go:\d+`).MatchString(got) {
		t.Fatalf("source 未指向测试调用点：%s", got)
	}
	if strings.Contains(got, "logging/handler.go:") {
		t.Fatalf("source 错误指向 Handler：%s", got)
	}
}

func TestShortenPath(t *testing.T) {
	tests := map[string]string{
		"/home/developer/code/example/wiki/client.go": "wiki/client.go",
		"example/service/lookup.go":                   "service/lookup.go",
		"/home/developer/code/sts2bot/main.go":        "main.go",
		"main.go":                                     "main.go",
		"":                                            "unknown",
	}
	for input, want := range tests {
		if got := shortenPath(input); got != want {
			t.Errorf("shortenPath(%q) = %q，期望 %q", input, got, want)
		}
	}
}

func TestHandlerLevelFiltering(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(NewHandler(&output, Options{Level: slog.LevelWarn, Color: ColorNever}))
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")
	got := output.String()
	if strings.Contains(got, "debug") || strings.Contains(got, "info") {
		t.Fatalf("低级别日志未过滤：%s", got)
	}
	if !strings.Contains(got, "WARN") || !strings.Contains(got, "ERROR") {
		t.Fatalf("高级别日志缺失：%s", got)
	}
}

func TestHandlerAttrsAndGroups(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(NewHandler(&output, Options{Color: ColorNever})).
		With("module", "wiki", "bound", "one").
		WithGroup("request").
		With("method", "GET")
	logger.Info("请求\n完成", slog.Group("response", "status", 200), "module", "nested")
	got := output.String()
	for _, want := range []string{
		"[wiki]", `bound="one"`, `request.method="GET"`, `request.response.status="200"`, `request.module="nested"`, `"请求\n完成"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("日志缺少 %q：%s", want, got)
		}
	}
	if strings.Contains(got, `module="wiki"`) {
		t.Fatalf("顶层 module 被重复输出：%s", got)
	}
}

type unsafeBuffer struct {
	mu         sync.Mutex
	buffer     bytes.Buffer
	concurrent bool
	writing    bool
	writes     int
}

func (w *unsafeBuffer) Write(data []byte) (int, error) {
	w.mu.Lock()
	if w.writing {
		w.concurrent = true
	}
	w.writing = true
	w.mu.Unlock()
	time.Sleep(time.Microsecond)
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writes++
	n, err := w.buffer.Write(data)
	w.writing = false
	return n, err
}

func TestHandlerConcurrentWrites(t *testing.T) {
	writer := &unsafeBuffer{}
	root := slog.New(NewHandler(writer, Options{Color: ColorNever}))
	const workers = 8
	const messages = 50
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		logger := root.With("module", "worker")
		wait.Add(1)
		go func() {
			defer wait.Done()
			for i := 0; i < messages; i++ {
				logger.Info("message", "index", i)
			}
		}()
	}
	wait.Wait()
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.concurrent {
		t.Fatal("writer 被并发调用")
	}
	if writer.writes != workers*messages {
		t.Fatalf("Write 次数 = %d", writer.writes)
	}
	lines := strings.Split(strings.TrimSpace(writer.buffer.String()), "\n")
	if len(lines) != workers*messages {
		t.Fatalf("日志行数 = %d", len(lines))
	}
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

type shortWriter struct{}

func (shortWriter) Write(data []byte) (int, error) { return len(data) - 1, nil }

func TestHandlerWriterErrors(t *testing.T) {
	sentinel := errors.New("write failed")
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "message", 0)
	if err := NewHandler(errorWriter{err: sentinel}, Options{}).Handle(context.Background(), record); !errors.Is(err, sentinel) {
		t.Fatalf("错误 = %v", err)
	}
	if err := NewHandler(shortWriter{}, Options{}).Handle(context.Background(), record); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("短写错误 = %v", err)
	}
}
