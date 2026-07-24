# Slay the Spire 2 Wiki Telegram Bot

一个使用 Go 编写的 Telegram Bot，从[杀戮尖塔 2 中文维基](https://sts2.huijiwiki.com)查询并返回卡牌、遗物、敌人和药水信息。

## 功能

- `/card <卡牌名称>`：查询卡牌
- `/relic <遗物名称>`：查询遗物
- `/enemy <敌人名称>`：查询敌人
- `/potion <药水名称>`：查询药水
- `/help`：显示帮助
- 本地内存 TTL 缓存
- Telegram Rich Message HTML 回复
- 优雅退出与结构化日志

## 环境要求

- Go 1.26.5 或更高版本
- Telegram Bot Token

## 配置

复制示例配置：

```bash
cp config.example.toml config.toml
```

然后编辑 `config.toml`：

```toml
[telegram]
token = "YOUR_BOT_TOKEN_HERE"

[log]
level = "info"

[cache]
ttl_hours = 72

[wiki]
tls_profile = "safari-16.0"
```

`config.toml` 包含本地凭据，已被 Git 忽略，请勿提交。

## 运行

```bash
go mod download
go run .
```

也可以使用 Makefile：

```bash
make run
```

## 测试

```bash
go test ./...
```

运行完整提交前检查：

```bash
make check
```

## 构建

```bash
make build VERSION=0.1.0
```

构建产物默认不会被 Git 跟踪。
