# CLAUDE.md — Slay the Spire 2 Wiki Telegram Bot

## 项目概述

使用 Go 语言开发一个 Telegram Bot，从杀戮尖塔2中文维基（<https://sts2.huijiwiki.com>）抓取数据，支持卡牌、遗物、敌人和药水查询，并提供 `/help` 帮助。

---

## 技术栈

- **语言**: Go
- **配置**: `config.toml`
- **数据来源**: `<data.directory>/wiki.toml`；仅 owner `/update` 从 <https://sts2.huijiwiki.com> 批量同步
- **HTTPS 抓取**: `github.com/refraction-networking/utls` 模拟浏览器 ClientHello，`golang.org/x/net/http2` 承载 HTTP/2
- **本地存储**: schema-versioned TOML，启动时严格加载到不可变内存快照

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
├── data/
│   └── store.go           # 本地 TOML 严格加载与原子发布
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
owner_id = 123456789

[log]
level = "info"   # 可选: debug, info, warn, error

[data]
directory = "data" # 相对于配置文件目录

[wiki]
tls_profile = "safari-16.0" # 可选：standard、chrome-133、firefox-120、safari-16.0
request_interval_ms = 1000 # 所有 Wiki API/详情请求的最小间隔
```

---

## 功能指令规范

| 指令                 | 用途               |
| -------------------- | ------------------ |
| `/card <卡牌名称>`   | 查询卡牌           |
| `/relic <遗物名称>`  | 查询遗物           |
| `/enemy <敌人名称>`  | 查询敌人           |
| `/potion <药水名称>` | 查询药水           |
| `/help`              | 显示全部指令及示例 |
| `/update`            | Owner 批量同步数据 |

### `/help`

**描述**：显示全部普通用户指令、用途和示例。该指令不需要参数，不请求 Wiki；通过 Rich Message 回复原消息。管理命令 `/update` 不在公开帮助中展示。

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
<ul>
  <li>耗能：1</li>
</ul>
<p>造成9点伤害。</p>

<footer>
  ——来源：<a href="https://sts2.huijiwiki.com">杀戮尖塔2中文维基</a>
</footer>
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

<footer>
  ——来源：<a href="https://sts2.huijiwiki.com">杀戮尖塔2中文维基</a>
</footer>
```

---

## 使用 uTLS 抓取 Wiki 内容

### 使用入口

- 统一通过 `wiki.NewHTTPClient(wiki.DefaultBaseURL, profile, timeout, requestInterval)` 创建可复用的 `*http.Client`，再将其传给 `wiki.NewClient`；请求间隔覆盖列表 API 与详情页面。
- `main.go` 从 `wiki.tls_profile` 和 `wiki.request_interval_ms` 读取配置。Wiki client 只注入 updater，普通查询不得访问网络。
- 支持以下 profile：

| 配置值        | 请求方式                                                 |
| ------------- | -------------------------------------------------------- |
| `standard`    | 克隆 Go 的 `http.DefaultTransport`，不使用 uTLS          |
| `chrome-133`  | uTLS `HelloChrome_133` + HTTP/2                          |
| `firefox-120` | uTLS `HelloFirefox_120` + HTTP/2                         |
| `safari-16.0` | 基于 uTLS `HelloSafari_16_0` 的兼容 ClientHello + HTTP/2 |

初始化方式：

```go
profile, err := wiki.ParseTLSProfile(cfg.Wiki.TLSProfile)
if err != nil {
    return err
}

httpClient, err := wiki.NewHTTPClient(
    wiki.DefaultBaseURL,
    profile,
    15*time.Second,
    cfg.RequestInterval(),
)
if err != nil {
    return err
}

client, err := wiki.NewClient(wiki.DefaultBaseURL, httpClient, logger)
```

### 请求链路

1. `/update` 使用 MediaWiki `embeddedin` API 枚举四类详情页，再由 `wiki.Client.fetch` 使用 `url.PathEscape(name)` 请求 `/wiki/<页面名>`。
2. `intervalTransport` 在共享 client 范围限制所有请求开始间隔；`fetchURL` 统一处理请求头、状态码、2 MiB 上限和 challenge。
3. 浏览器 profile 由 `profileRoundTripper` 设置与 ClientHello 匹配的请求头，并附加 `From: sts2bot authorized crawler`。
4. uTLS transport 完成经系统根证书验证的握手；浏览器 profile 要求 ALPN `h2` 并复用连接。

### Safari 兼容 profile

- `safari-16.0` 不是直接使用默认 `HelloSafari_16_0`：`safariClientHelloSpec` 保留 Safari 的证书压缩扩展，并将 TLS 版本限制为 TLS 1.2。
- 这是针对 Wiki 当前 Cloudflare 边缘兼容性的项目级处理；如需改变版本或扩展，必须先用 live test 验证，再同步更新 `wiki/transport_test.go`。
- 不应把该兼容策略扩散为通用 TLS 降级；Chrome 和 Firefox profile 仍允许 TLS 1.2–1.3。

### 安全与运行约束

- uTLS 仅用于控制 ClientHello 指纹，不代表关闭 TLS 安全校验。必须保留 `ServerName`、系统根证书验证和完整证书链校验；禁止设置 `InsecureSkipVerify`、自定义“全部接受”的校验回调或绕过证书错误。
- uTLS profile 当前只支持 HTTPS 直连。若目标命中 `HTTP_PROXY`、`HTTPS_PROXY` 或 `ALL_PROXY`，初始化会拒绝启动；应为 Wiki 主机配置 `NO_PROXY=sts2.huijiwiki.com`，不要静默绕过代理策略。
- 保持上下文和超时有效：TCP dial 使用 `DialContext`，TLS 握手使用 `HandshakeContext`，客户端必须设置正数 timeout。
- HTTP/2 是浏览器 profile 的硬性要求；ALPN 未得到 `h2` 时应返回错误，不得悄悄退回 HTTP/1.1 而形成与声明 profile 不一致的网络指纹。
- 只将该客户端用于本项目获准抓取的 Wiki；遵守站点条款、robots 规则和合理请求频率。uTLS 不用于绕过验证码、访问控制或限流；检测到 challenge、403 或 429 时按现有错误分类返回，不进行自动规避或高频重试。
- 不记录响应正文、Cookie、授权信息或 Telegram token。日志仅记录 URL、状态码、耗时、profile 和经过归类的错误。

### 修改与验证

- 新增或修改 profile 时，同时更新 `ParseTLSProfile`、`config` 默认值/校验、请求头映射及测试。
- 至少运行：

```bash
go test ./wiki ./config
go test -race ./wiki ./config
go vet ./...
```

- `wiki/transport_test.go` 应持续覆盖：profile 解析、自定义 ClientHello、HTTP/2 连接复用、证书拒绝、ALPN 拒绝、代理拒绝及 context 取消。
- 需要连接真实 Wiki 的验证应放在显式启用的 live test 中，避免普通单元测试依赖外网或触发站点限流。

---

## 本地数据规范

- 普通查询只读启动时加载的 `<data.directory>/wiki.toml` 内存快照，不在缺失时回退 Wiki。
- `/update` 仅允许 `telegram.owner_id` 对应用户执行；全量成功并严格校验后才通过同目录临时文件 + rename 原子替换。
- TOML 顶层 `schema_version = 1`，包含 cards、relics、enemies、potions 四个数组；未知字段、必填字段缺失、重复规范名或重复 ID 会拒绝启动/发布。
- 同步每成功一条记录原子保存到 `wiki.update.toml`；取消/失败后下次 `/update` 从检查点继续。检查点可部分完成，只有完整数据通过严格校验后才替换 `wiki.toml`。
- 不请求或解析卡图来推断辉星耗能；Wiki 无文本值时由维护者在 `wiki.toml` 手工填写，非储君卡牌保持为空。
- 人工编辑后需要重启；下一次 `/update` 会覆盖人工修改。

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
