SHELL := /bin/sh

VERSION ?= dev
NAME ?= sts2bot
ARCH ?= $(shell $(GO) env GOARCH)
OS ?= $(shell $(GO) env GOOS)
CONFIG ?= config.toml
GO ?= go
GOFLAGS ?=
TLS_PROFILE ?= safari-16.0

BUILD_FLAGS := -trimpath -ldflags "-X main.version=$(VERSION)"

.PHONY: all build run fmt fmt-check tidy verify test test-race vet check live-wiki clean help

all: build

## build: 使用 VERSION、NAME、ARCH、OS 构建可执行文件
build:
	GOOS=$(OS) GOARCH=$(ARCH) $(GO) build $(GOFLAGS) $(BUILD_FLAGS) -o $(NAME)-$(OS)-$(ARCH)-$(VERSION) .

## run: 使用指定配置运行 Bot
run:
	$(GO) run $(GOFLAGS) . -config $(CONFIG)

## fmt: 格式化所有 Go 源码
fmt:
	$(GO) fmt ./...

## fmt-check: 检查是否存在未格式化的 Go 源码
fmt-check:
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		printf '%s\n' "以下文件需要运行 gofmt：" "$$files"; \
		exit 1; \
	fi

## tidy: 整理并同步 Go 模块依赖
tidy:
	$(GO) mod tidy

## verify: 验证下载的模块内容
verify:
	$(GO) mod verify

## test: 以随机顺序运行全部单元测试
test:
	$(GO) test $(GOFLAGS) -shuffle=on -count=1 ./...

## test-race: 启用竞态检测运行全部测试
test-race:
	$(GO) test $(GOFLAGS) -race -count=1 ./...

## vet: 运行 Go 静态检查
vet:
	$(GO) vet $(GOFLAGS) ./...

## check: 运行提交前的完整检查
check: fmt-check verify test test-race vet build

## live-wiki: 对真实 Wiki 执行一次授权低频 smoke test
live-wiki:
	STS2BOT_LIVE_WIKI=1 STS2BOT_LIVE_TLS_PROFILE=$(TLS_PROFILE) \
		$(GO) test $(GOFLAGS) ./wiki -run '^TestLiveWikiTLSProfile$$' -count=1 -v

## clean: 删除 NAME 指定的构建产物和 Go 测试缓存
clean:
	rm -f $(NAME)
	$(GO) clean -testcache

## help: 显示可用目标
help:
	@grep '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
