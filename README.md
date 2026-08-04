# Slay the Spire 2 Wiki Telegram Bot

一个使用 Go 编写的 Telegram Bot。卡牌、遗物、敌人和药水查询只读取本地 TOML；Bot Owner 通过 `/update` 低频同步[杀戮尖塔 2 中文维基](https://sts2.huijiwiki.com)。

## 功能

- `/card <卡牌名称>`：查询卡牌
- `/relic <遗物名称>`：查询遗物
- `/enemy <敌人名称>`：查询敌人
- `/potion <药水名称>`：查询药水
- `/help`：显示帮助
- Owner-only `/update`：批量同步 Wiki，并在同一 Telegram 消息中更新进度
- 人类可读、可手工编辑的 `data/wiki.toml`
- 可配置 Wiki 请求间隔
- Telegram Rich Message HTML 回复与结构化日志

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
owner_id = 123456789

[log]
level = "info"

[data]
directory = "data"

[wiki]
tls_profile = "safari-16.0"
request_interval_ms = 1000
```

`owner_id` 是可执行 `/update` 的 Telegram 用户 ID。相对数据目录以配置文件所在目录为基准。首次启动没有 `wiki.toml` 时普通查询提示尚未同步；Owner 执行 `/update` 后生成完整本地文件。可停机后手工编辑该 TOML 并重启加载；下次 `/update` 会覆盖手工修改。

同步期间每成功抓取一条记录都会原子写入 `data/wiki.update.toml` 检查点；取消或失败后再次执行 `/update` 会跳过检查点中仍在站点列表里的已完成记录。只有四类数据全部完成并通过完整校验后，检查点才发布为 `wiki.toml`。辉星耗能不再从卡图推断；Wiki 没有文本值时请在 `wiki.toml` 中手工填写，非储君卡牌保持为空。

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

