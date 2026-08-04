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
	"sts2bot/config"
	localdata "sts2bot/data"
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
	handlerLogger := rootLogger.With("module", "handler")
	botLogger := rootLogger.With("module", "bot")
	serviceLogger := rootLogger.With("module", "service")
	telegramLogger := rootLogger.With("module", "telegram")

	tlsProfile, err := wiki.ParseTLSProfile(cfg.Wiki.TLSProfile)
	if err != nil {
		wikiLogger.Error("Wiki TLS profile 配置错误", "event", "wiki_tls_profile_error", "error", err)
		os.Exit(1)
	}
	wikiHTTPClient, err := wiki.NewHTTPClient(wiki.DefaultBaseURL, tlsProfile, 15*time.Second, cfg.RequestInterval())
	if err != nil {
		wikiLogger.Error("初始化 Wiki HTTP 客户端失败", "event", "wiki_http_init_error", "error", err)
		os.Exit(1)
	}
	wikiClient, err := wiki.NewClient(wiki.DefaultBaseURL, wikiHTTPClient, wikiLogger)
	if err != nil {
		wikiLogger.Error("初始化 Wiki 客户端失败", "event", "wiki_init_error", "error", err)
		os.Exit(1)
	}
	dataStore := localdata.NewStore(cfg.Data.Directory)
	if err := dataStore.Prepare(); err != nil {
		mainLogger.Error("准备本地数据目录失败", "event", "data_prepare_error", "error", err)
		os.Exit(1)
	}
	dataFile, initialized, err := dataStore.LoadCurrent()
	if err != nil {
		mainLogger.Error("加载本地数据失败", "event", "data_load_error", "error", err)
		os.Exit(1)
	}
	var snapshot *service.Snapshot
	if initialized {
		snapshot = service.NewSnapshot(dataFile.Cards, dataFile.Relics, dataFile.Enemies, dataFile.Potions)
	}
	lookup := service.NewLookup(snapshot)
	updater := service.NewUpdater(wikiClient, dataStore, lookup, serviceLogger)
	commandHandler := handler.New(lookup, handlerLogger)
	adapter := bot.New(commandHandler, updater, cfg.Telegram.OwnerID, botLogger)
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
	mainLogger.Info("Bot 启动成功", "event", "bot_start", "version", version, "token_prefix", maskedTokenPrefix(cfg.Telegram.Token), "owner_id", cfg.Telegram.OwnerID, "data_directory", cfg.Data.Directory, "wiki_request_interval_ms", cfg.Wiki.RequestIntervalMS, "wiki_tls_profile", tlsProfile)
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
