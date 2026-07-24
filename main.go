package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	telegram "github.com/go-telegram/bot"

	"sts2bot/bot"
	"sts2bot/cache"
	"sts2bot/config"
	"sts2bot/domain"
	"sts2bot/handler"
	"sts2bot/logging"
	"sts2bot/service"
	"sts2bot/wiki"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "config.toml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rootLogger := newLogger(cfg.Log.Level)
	slog.SetDefault(rootLogger)
	mainLogger := rootLogger.With("module", "main")
	wikiLogger := rootLogger.With("module", "wiki")
	serviceLogger := rootLogger.With("module", "service")
	handlerLogger := rootLogger.With("module", "handler")
	botLogger := rootLogger.With("module", "bot")
	telegramLogger := rootLogger.With("module", "telegram")

	tlsProfile, err := wiki.ParseTLSProfile(cfg.Wiki.TLSProfile)
	if err != nil {
		wikiLogger.Error("Wiki TLS profile 配置错误", "event", "wiki_tls_profile_error", "error", err)
		os.Exit(1)
	}
	wikiHTTPClient, err := wiki.NewHTTPClient(wiki.DefaultBaseURL, tlsProfile, 15*time.Second)
	if err != nil {
		wikiLogger.Error("初始化 Wiki HTTP 客户端失败", "event", "wiki_http_init_error", "error", err)
		os.Exit(1)
	}
	wikiClient, err := wiki.NewClient(wiki.DefaultBaseURL, wikiHTTPClient, wikiLogger)
	if err != nil {
		wikiLogger.Error("初始化 Wiki 客户端失败", "event", "wiki_init_error", "error", err)
		os.Exit(1)
	}
	lookup := service.NewLookup(
		wikiClient,
		cache.New[domain.Card](cfg.TTL()),
		cache.New[domain.Relic](cfg.TTL()),
		cache.New[domain.Enemy](cfg.TTL()),
		cache.New[domain.Potion](cfg.TTL()),
		serviceLogger,
	)
	commandHandler := handler.New(lookup, handlerLogger)
	adapter := bot.New(commandHandler, botLogger)
	telegramBot, err := telegram.New(
		cfg.Telegram.Token,
		telegram.WithAllowedUpdates(telegram.AllowedUpdates{"message"}),
		telegram.WithWorkers(bot.MaxConcurrentHandlers),
		telegram.WithNotAsyncHandlers(),
		telegram.WithErrorsHandler(func(err error) {
			telegramLogger.Error("Telegram 内部错误", "event", "telegram_internal_error", "error", err)
		}),
		telegram.WithDefaultHandler(adapter.HandleUpdate),
	)
	if err != nil {
		telegramLogger.Error("初始化 Telegram Bot 失败", "event", "bot_init_error", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	mainLogger.Info("Bot 启动成功", "event", "bot_start", "version", version, "token_prefix", maskedTokenPrefix(cfg.Telegram.Token), "cache_ttl_hours", cfg.Cache.TTLHours, "wiki_tls_profile", tlsProfile)
	telegramBot.Start(ctx)
	mainLogger.Info("Bot 停止", "event", "bot_stop", "reason", ctx.Err())
}

func newLogger(level string) *slog.Logger {
	levels := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	return slog.New(logging.NewHandler(os.Stdout, logging.Options{Level: levels[level], Color: logging.ColorAuto}))
}

func maskedTokenPrefix(token string) string {
	prefix := token
	if index := strings.IndexByte(token, ':'); index >= 0 {
		prefix = token[:index]
	}
	if len(prefix) > 6 {
		prefix = prefix[:6]
	}
	return prefix + "…"
}
