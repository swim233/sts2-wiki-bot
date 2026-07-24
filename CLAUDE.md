# CLAUDE.md — Slay the Spire 2 Wiki Telegram Bot

## 项目概述

使用 Go 语言开发一个 Telegram Bot，从杀戮尖塔2中文维基（https://sts2.huijiwiki.com）抓取数据，支持卡牌、遗物、敌人和药水查询，并提供 `/help` 帮助。

---

## 技术栈

- **语言**: Go
- **配置**: `config.toml`
- **数据来源**: https://sts2.huijiwiki.com（HTML爬取/解析）
- **缓存**: 本地缓存，TTL 可配置，默认 3 天

---

## 目录结构（建议）

```
.
├── main.go
├── config.toml
├── config/
│   └── config.go# 配置加载
├── bot/
│   └── bot.go             # Telegram Bot 主逻辑、指令注册与分发
├── handler/
│   ├── card.go            # /card 指令处理
│   ├── relic.go           # /relic 指令处理
│   ├── enemy.go           # /enemy 指令处理
│   └── potion.go          # /potion 指令处理
├── wiki/
│   ├── client.go          # HTTP 请求封装
│   ├── card.go            # 卡牌页面爬取与解析
│   └── relic.go           # 遗物页面爬取与解析
├── cache/
│   └── cache.go           # 缓存管理（TTL 支持）
├── logging/
│   └── handler.go         # 彩色终端 slog Handler
└── formatter/
    ├── card.go            # 卡牌信息 Rich HTML 格式化
    ├── relic.go           # 遗物信息 Rich HTML 格式化
    ├── enemy.go           # 敌人信息 Rich HTML 格式化
    ├── potion.go          # 药水信息 Rich HTML 格式化
    └── help.go            # /help Rich HTML 内容
```

---

## 配置文件 `config.toml`

```toml
[telegram]
token = "YOUR_BOT_TOKEN_HERE"

[log]
level = "info"   # 可选: debug, info, warn, error

[cache]
ttl_hours = 72# 缓存有效时间，单位：小时，默认 72（3天）
```

---

## 功能指令规范

| 指令 | 用途 |
| --- | --- |
| `/card <卡牌名称>` | 查询卡牌 |
| `/relic <遗物名称>` | 查询遗物 |
| `/enemy <敌人名称>` | 查询敌人 |
| `/potion <药水名称>` | 查询药水 |
| `/help` | 显示全部指令及示例 |

### `/help`

**描述**：显示全部可用指令、用途和示例。该指令不需要参数，不请求 Wiki，也不使用缓存；通过 Rich Message 回复原消息。

### `/card <卡牌名称>`

**描述**: 查询指定卡牌信息。

**Wiki URL规则**: `https://sts2.huijiwiki.com/wiki/<URL编码后的卡牌名称>`

**示例**: `/card 打击` → 访问 `https://sts2.huijiwiki.com/wiki/%E6%89%93%E5%87%BB`

**需要解析的字段**:

| 字段       | 说明                                |
| ---------- | ----------------------------------- |
| 卡牌名称   | 中文名                              |
| 英文 ID    | 如 `STRIKE_IRONCLAD`                |
| 颜色       | 所属角色，如 `铁甲战士`             |
| 稀有度     | 如 `初始`、`普通`、`非普通`、`稀有` |
| 耗能       | 费用                                |
| 描述       | 卡牌效果文本                        |
| 升级后耗能 | 升级版费用                          |
| 升级后描述 | 升级版效果文本                      |

**Rich Message 输出格式**:

```html
<h1>🃏 打击</h1>
<code>STRIKE_IRONCLAD</code>

<h2>属性</h2>
<ul>
<li>颜色：铁甲战士</li>
<li>稀有度：初始</li>
<li>耗能：1</li>
</ul>

<h2>📖 描述</h2>
<p>造成6点伤害。</p>

<h2>⬆️ 升级后</h2>
<ul><li>耗能：1</li></ul>
<p>造成9点伤害。</p>

<footer>——来源：<a href="https://sts2.huijiwiki.com">杀戮尖塔2中文维基</a></footer>
```

---

### `/relic <遗物名称>`

**描述**: 查询指定遗物信息。

**Wiki URL 规则**: `https://sts2.huijiwiki.com/wiki/<URL编码后的遗物名称>`

**示例**: `/relic 燃烧之血` → 访问 `https://sts2.huijiwiki.com/wiki/%E7%87%83%E7%83%A7%E4%B9%8B%E8%A1%80`

**需要解析的字段**:

| 字段       | 说明                                                |
| ---------- | --------------------------------------------------- |
| 遗物名称   | 中文名                                              |
| 英文 ID    | 如 `BURNING_BLOOD`                                  |
| 所属遗物池 | 所属角色，如 `铁甲战士`                             |
| 稀有度     | 如 `初始`、`普通`、`非普通`、`稀有`、`Boss`、`商店` |
| 描述       | 遗物效果文本                                        |
| 引言       | Flavor text（可选，若存在则展示）                   |

**Rich Message 输出格式**:

```html
<h1>🏺 燃烧之血</h1>
<code>BURNING_BLOOD</code>

<h2>属性</h2>
<ul>
<li>所属遗物池：铁甲战士</li>
<li>稀有度：初始</li>
</ul>

<h2>📖 描述</h2>
<p>在战斗结束时，回复6点生命。</p>

<h2>💬 引言</h2>
<p>这个遗物的细节将在未来揭晓……</p>

<footer>——来源：<a href="https://sts2.huijiwiki.com">杀戮尖塔2中文维基</a></footer>
```

---

## 缓存规范

-缓存 key 格式：`card:<名称>` 或 `relic:<名称>`

- 缓存命中时直接返回，不重新请求 Wiki
- 缓存存储：内存（`sync.Map` 或类似方案），程序重启后失效
- TTL 从 `config.toml` 读取，精度为小时

---

## 错误处理规范

| 场景                     | 回复内容                                                   |
| ------------------------ | ---------------------------------------------------------- |
| 未提供查询名称           | 提示用法，如 `用法：/card <卡牌名称>`                      |
| Wiki 页面不存在（404）   | `❌ 未找到「xxx」的相关信息，请确认名称是否正确。`         |
| 网络请求失败             | `❌ 请求失败，请稍后重试。（错误：<error message>）`       |
| 页面解析失败（字段缺失） | `❌ 数据解析失败，Wiki 页面格式可能已更新，请联系管理员。` |

所有错误响应均以 **回复用户消息** 的形式发送（reply to message）。

---

## 日志规范

使用基于 `log/slog` 的单行可读结构化日志，格式如下：

```text
[wiki] 2026/07/24 - 18:15:42  INFO   wiki/client.go:114  Wiki 请求完成 event="wiki_response" status_code="200" duration_ms="83"
```

- 模块前缀使用 `main`、`wiki`、`service`、`handler`、`bot`、`telegram`；未注入模块时回退为 `sts2bot`。
- DEBUG、INFO、WARN、ERROR 分别使用青、绿、黄、红色；仅在终端自动着色，管道、文件重定向、`NO_COLOR` 或 `TERM=dumb` 环境不输出 ANSI。
- 每条日志包含本地时间、级别、实际调用源文件和行号，属性继续使用稳定的 `key=value` 结构。
- 应用日志写入 stdout；配置尚未加载时的启动错误写入 stderr。配置仍只使用 `log.level`。
- 默认不启用 Telegram SDK 请求/响应 debug，避免用户消息和请求正文进入日志。

| 事件              | 级别  | 内容                             |
| ----------------- | ----- | -------------------------------- |
| Bot 启动          | INFO  | 启动成功，token 前缀             |
| 收到指令          | INFO  | user_id, username, command, args |
| 缓存命中          | DEBUG | cache_key                        |
| 发起Wiki 请求     | DEBUG | url                              |
| Wiki 请求完成     | DEBUG | url, status_code, duration       |
| 解析成功          | DEBUG | type(card/relic), name           |
| 回复用户          | INFO  | user_id, command                 |
| 错误（网络/解析） | ERROR | error message, url               |
| Bot 停止          | INFO  | 收到信号，退出                   |

---

## 实现注意事项

1. **HTML 解析**: 使用 `golang.org/x/net/html` 或 `github.com/PuerkitoBio/goquery` 解析 Wiki infobox 表格。
2. **URL 编码**: 查询名称需用 `url.QueryEscape` 或 `url.PathEscape` 转义后拼接到 URL。
3. **Telegram 回复**: 使用 `sendRichMessage` 和 `InputRichMessage.HTML` 发送原生 Rich Message，并通过 `reply_parameters.message_id` 关联原始消息。Rich Message 端点不使用 `parse_mode`、`entities` 或 `link_preview_options`。
4. **Rich HTML**: 普通文本、code 内容和链接属性必须在插入模板时按 HTML 上下文编码，且不得对已经格式化的 Rich HTML 二次编码。`InputRichMessage` 只设置一种主体格式，本项目统一设置 `html`，不同时设置 `markdown`。
5. **Graceful Shutdown**: 监听 `SIGINT`/`SIGTERM` 信号优雅退出。
6. **并发安全**: 缓存读写需加锁。

---

## 开发与运行

```bash
# 安装依赖
go mod tidy

# 运行
go run main.go

# 编译
go build -o sts2bot .
```

确保 `config.toml` 与可执行文件在同一目录下。
