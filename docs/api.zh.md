# API 与集成

BombVault 提供一个小型 HTTP API，供脚本、仪表盘和家庭自动化使用。它读取概览里显示的内容，也能启动备份。其余的操作，例如恢复、删除备份和更改设置，仍然只在网页界面里进行。

## 令牌 {#tokens}

即使没有设置登录密码，每个请求也都需要 API 令牌。在**设置、集成、API 令牌**下创建：

1. 输入一个能说明令牌用在哪里的名称，例如"Home Assistant"或"Uptime Kuma"。
2. 如果令牌需要能启动备份，就开启**允许启动备份**。不开启的话，令牌只能读取。
3. 点击**创建令牌**。令牌只显示一次。BombVault 只保存它的指纹，所以现在就复制下来。

令牌放在请求头里发送，用 `Authorization: Bearer <token>` 或 `X-API-Key: <token>` 都可以。令牌以 `bvapi_` 开头。它只能打开 API：MCP 密钥在这里不能用，令牌也不能用于 MCP。

每个令牌都有一个磁贴，显示它的名称、是否可以启动备份、最后四个字符、上次使用的时间和来源，以及今天的调用次数。在磁贴上可以重命名、更改它的权限、更换或撤销它。**日志**显示它启动的备份和最近的调用。从备份恢复 BombVault 的配置会撤销所有令牌，因为备份里可能有你后来撤销的令牌。

没有登录密码时，能打开网页界面的人也可以创建令牌。如果你用一个看起来像公网的名称打开 BombVault，又没有设置密码，就无法从该地址创建令牌，规则和 [MCP 密钥](mcp.md#switch-on)相同。

## 端点 {#endpoints}

| 路由 | 返回或执行的内容 | 令牌 |
|---|---|---|
| `GET /api/v1/health` | 版本、实例名称、是否有备份在运行，以及这个令牌可以做什么 | 读取 |
| `GET /api/v1/status` | 每个领域的保护状态：上次成功备份、预期间隔、检查项、接下来的计划运行 | 读取 |
| `GET /api/v1/activity` | 现在正在运行的内容，带阶段和百分比 | 读取 |
| `GET /api/v1/items` | 每个受保护的条目，带它的计划、备份会停止什么以及上次备份。用 `?domain=` 只看一个领域 | 读取 |
| `GET /api/v1/runs` | 运行历史，最新的在前。筛选条件 `limit`、`domain`、`item`、`status`、`kind`、`since` | 读取 |
| `GET /api/v1/anomalies` | 异常，附带未处理内容的摘要。筛选条件 `state`、`severity`、`domain`、`limit` | 读取 |
| `GET /api/v1/anomalies/{id}` | 一个异常 | 读取 |
| `GET /api/v1/storage/{domain}` | 一个领域每个仓库的大小历史、每周增长和可用空间 | 读取 |
| `POST /api/v1/backups` | 备份一个条目（`{"domain":"containers","item":"plex"}`）或整个领域（`{"domain":"vms"}`） | 启动 |
| `POST /api/v1/backups/everything` | 运行全部备份这一轮 | 启动 |
| `POST /api/v1/runs/{id}/cancel` | 取消这个令牌启动的、正在运行的备份 | 启动 |

领域有 `containers`、`vms`、`files`、`zfs`、`flash` 和 `config`。时间是 Unix 秒。返回的内容和同名的 [MCP 工具](mcp.md#tools)相同，所以两者保持一致。

启动的备份和网页界面启动的备份是同一种：运行中的容器会被停止，直到它的备份完成。请求立即返回，`/api/v1/activity` 和 `/api/v1/runs` 会显示进展。

## 示例 {#examples}

```sh
# How are the backups doing?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Back up one container now.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

使用 BombVault 自己的自签名证书时，请加上 `--cacert bombvault-cert.pem`（MCP 卡片上的**下载证书**会给你这个文件），在你信任的网络上也可以用 `-k`。

## 错误与限制 {#errors}

错误以 `{"error": {"code": "...", "message": "..."}}` 的形式返回，并带有对应的状态码：

| 状态 | 代码 | 含义 |
|---|---|---|
| 400 | `invalid_argument`、`ambiguous` | 缺少参数或参数有误 |
| 401 | `no_token`、`invalid_token` | 没有令牌，或令牌不是有效的 |
| 403 | `not_permitted`、`forbidden_origin` | 令牌只能读取、这次运行不是由它启动的，或请求来自另一个来源的页面 |
| 404 | `not_found` | 没有这样的条目、运行或异常 |
| 409 | `busy`、`domain_off`、`nothing_to_back_up`、`not_running` | 其他东西正在运行、这个领域已关闭，或没有可做的事 |
| 429 | `throttled`、`rate_limited`、`cooldown`、`retention_guard` | 有限制挡住了请求。`Retry-After` 说明什么时候再试 |

启动遵循和[通过 MCP 启动](mcp.md#starting-backups)相同的限制：每个令牌每小时 12 次，同一个条目的两次启动之间隔 15 分钟，同一个条目 24 小时内最多启动 4 次，以及保留保护。后三项把通过 MCP、API 和 Home Assistant 的启动合在一起计算。一个令牌每分钟可以发出 120 个请求。同一个地址连续失败五次，会被锁定一分钟。

## OpenAPI {#openapi}

BombVault 在 `/api/v1/openapi.json` 提供这些路由的描述（OpenAPI 3.1）。不需要令牌。可以把它加载到 Swagger UI、Postman 或代码生成器里。

## Home Assistant {#home-assistant}

BombVault 可以通过 MQTT 发现，作为一台设备出现在 Home Assistant 里。Home Assistant 需要它的 MQTT 集成和一个代理，例如 Mosquitto 加载项。不涉及任何自定义组件。

1. 在 BombVault 里打开**设置、集成**，在**服务**卡片里开启 **Home Assistant**。代理的字段会出现在它下面。
2. 输入代理的地址和端口，如果它要求，再输入用户名和密码。如果代理支持 TLS，就开启**使用 TLS**，通常是 8883 端口。它的证书必须对你输入的地址有效。如果你更改了地址、端口或用户名，请重新输入密码：BombVault 不会把保存的密码传给另一个代理或用户。
3. 点击**保存**。连接建立后，这一行会显示出来。

设备叫 BombVault，或者在括号里带上实例名称，有这些实体：

| 实体 | 显示什么 |
|---|---|
| 状态 | `ok`、`warning`、`failed` 或 `off`，取开启的领域里最差的那个 |
| 正在运行的作业 | 现在在运行什么，或 `idle` |
| 未处理的异常 | 有多少异常未处理 |
| 下一次计划备份 | 下一次计划备份何时开始 |
| *领域*上次备份 | 该领域上次成功备份的时间 |
| *领域*上次结果 | 它上次备份如何结束 |
| *领域*仓库可用空间 | 它的主仓库所在位置的可用空间，如果 BombVault 能读取的话 |
| 备份*领域* | 一个按钮，备份整个领域 |

每个开启的领域都有自己的实体，关闭一个领域就会失去它们。开启**按钮启动备份**之后才会出现这些按钮。新安装时它是关闭的。它们遵循和[通过 API 启动](#errors)相同的限制。除此之外，BombVault 每个领域一次只处理一次按下，每分钟不超过六次，并忽略代理当作保留消息保存的按下。任何能向代理发布消息的人都可以按下它们，所以请给代理设置密码。

BombVault 每 15 秒读取一次自己的状态，有变化时就以 JSON 发布到 `<prefix>/<node>/state` 下。前缀是 `bombvault`，除非你更改，节点是 BombVault 选定一次的短 ID。发现消息发往 Home Assistant 的默认前缀 `homeassistant`。两者都是保留消息。如果 BombVault 没打招呼就停止，遗嘱消息会把设备标记为不可用。关闭这个连接会从 Home Assistant 中移除设备及其实体。

### 示例：一张卡片和一个提醒 {#home-assistant-example}

Home Assistant 根据名称生成实体 ID，所以 BombVault 设备的 ID 形如 `sensor.bombvault_status`。如果括号里有实例名称，ID 里也会带上，所以请先在 **Developer Tools, States** 下查一下。仪表盘的一张卡片：

```yaml
type: entities
title: Backups
entities:
  - sensor.bombvault_status
  - sensor.bombvault_running_job
  - sensor.bombvault_open_anomalies
  - sensor.bombvault_next_scheduled_backup
```

一个在状态变为 `failed` 时通知你手机的自动化：

```yaml
alias: BombVault backup failed
trigger:
  - platform: state
    entity_id: sensor.bombvault_status
    to: failed
action:
  - service: notify.mobile_app_my_phone
    data:
      message: A BombVault backup failed. Open Needs you on the overview.
```

## Prometheus {#metrics}

**设置、集成**下**服务**卡片里的**暴露 /metrics**会在网页界面端口的 `/metrics` 上，为 Prometheus、Grafana 或 Uptime Kuma 提供备份状态、大小和时间戳。默认是关闭的。**抓取令牌（可选）**会让抓取程序发送一个 bearer 令牌。标签里没有密钥，也没有路径。

## 仪表盘小组件 {#dashboard-widget}

**设置、集成**下的**仪表盘小组件**卡片会生成一个只读令牌，并给你一个地址，用于任何显示 iframe 的仪表盘，例如 Homepage、Organizr 或 Heimdall。这个地址会在 `/widget` 打开一个小页面，只带实时活动日志。令牌只授予这份日志，别无其他，点击**停用**会立即撤销它。嵌入的页面只有英文。

## 在网络上找到 BombVault {#mdns}

BombVault 通过 mDNS 在本地网络上宣告它的网页界面，也就是 Bonjour 和 Avahi 背后的协议。浏览器随后可以通过 `https://bombvault.local:3443` 访问它，使用 `HTTP_ONLY` 时是 `http://bombvault.local:3000`，服务浏览器会把它列为带有子类型 `_bombvault` 的网页服务。它的 TXT 记录带有版本和路径。开关是**设置、集成**下**服务**卡片里的**在网络中查找**，默认开启。如果另一台设备已经在用这个名字，BombVault 会取 `bombvault-2.local`，依此类推，卡片会显示它得到的地址。当 BombVault 停止或你关闭宣告时，它会通知网络，所以浏览器会立即去掉这一条。

宣告能否到达你的网络，取决于容器是怎样连接的：

- **bridge**，Unraid 模板里的默认值：宣告留在 Docker 的网络内部，局域网上没有人看得到。像以前一样用主机的地址打开 BombVault。
- **br0** 或其他 macvlan 或 ipvlan 网络：容器在局域网上有自己的地址，宣告可以到达。
- **host**：宣告从主机的接口发出，和 Unraid 自己的并排。它会跳过 Docker 和 libvirt 的网桥。

只宣告 IPv4 地址。
