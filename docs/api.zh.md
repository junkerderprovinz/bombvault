# API 与集成

BombVault 提供一个小型 HTTP API，供脚本、仪表板和家庭自动化使用。它读取仪表板显示的内容，也能启动备份。其余操作，比如恢复、删除备份和设置，仍留在网页界面中。

## 令牌 {#tokens}

即使没有设置登录密码，每个请求也都需要 API 令牌。在 **设置、系统、API 令牌** 中创建：

1. 输入一个能说明令牌用途的名称，例如“Home Assistant”或“Uptime Kuma”。
2. 如果令牌需要启动备份，就打开 **允许启动备份**。不打开时它只能读取。
3. 点击 **创建令牌**。令牌只显示一次。BombVault 只保存它的指纹，所以请现在复制。

在请求头中发送令牌，`Authorization: Bearer <token>` 或 `X-API-Key: <token>` 均可。令牌以 `bvapi_` 开头。它只能打开 API：MCP 密钥在这里无效，令牌也不能用于 MCP。

每个令牌都有一个卡片，显示名称、能否启动备份、最后四个字符、最近一次使用的时间和来源，以及今天的调用次数。在卡片上可以重命名、修改权限、更换或吊销它。**日志** 显示它启动的备份和最近的调用。从备份恢复 BombVault 的配置会吊销所有令牌，因为备份中可能包含你后来吊销的令牌。

没有登录密码时，能打开网页界面的人也能创建令牌。如果用看起来像公网的名称打开 BombVault 且没有密码，就无法从该地址创建令牌，规则与 [MCP 密钥](mcp.md#switch-on) 相同。

## 端点 {#endpoints}

| 路由 | 返回内容或作用 | 令牌 |
|---|---|---|
| `GET /api/v1/health` | 版本、实例名称、是否有备份在运行，以及此令牌能做什么 | 读取 |
| `GET /api/v1/status` | 各领域的保护状态：最近一次成功备份、预期间隔、检查、下一次计划运行 | 读取 |
| `GET /api/v1/activity` | 当前正在运行的操作及其阶段和百分比 | 读取 |
| `GET /api/v1/items` | 每个受保护的项目及其计划、备份时会停止什么和最近一次备份；`?domain=` 只看一个领域 | 读取 |
| `GET /api/v1/runs` | 运行历史，最新在前；筛选 `limit`、`domain`、`item`、`status`、`kind`、`since` | 读取 |
| `GET /api/v1/anomalies` | 异常及未处理项的摘要；筛选 `state`、`severity`、`domain`、`limit` | 读取 |
| `GET /api/v1/anomalies/{id}` | 单个异常 | 读取 |
| `GET /api/v1/storage/{domain}` | 某领域每个仓库的大小历史、每周增长和剩余空间 | 读取 |
| `POST /api/v1/backups` | 备份一个项目（`{"domain":"containers","item":"plex"}`）或整个领域（`{"domain":"vms"}`） | 启动 |
| `POST /api/v1/backups/everything` | 运行 Backup Everything | 启动 |
| `POST /api/v1/runs/{id}/cancel` | 取消此令牌启动的正在运行的备份 | 启动 |

领域为 `containers`、`vms`、`files`、`zfs`、`flash` 和 `config`。时间是 Unix 秒。回答与同名的 [MCP 工具](mcp.md#tools) 相同，因此两者保持一致。

在这里启动的备份与网页界面启动的相同：正在运行的容器会停止，直到其备份完成。请求会立即返回，进度可在 `/api/v1/activity` 和 `/api/v1/runs` 中查看。

## 示例 {#examples}

```sh
# 备份情况如何？
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# 现在备份一个容器。
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

使用 BombVault 自带的自签名证书时，加上 `--cacert bombvault-cert.pem`（通过 MCP 卡片上的 **下载证书** 获取该文件），或在可信网络中加上 `-k`。

## 错误与限制 {#errors}

错误以 `{"error": {"code": "...", "message": "..."}}` 的形式返回，并带有相应的状态码：

| 状态 | 代码 | 含义 |
|---|---|---|
| 400 | `invalid_argument`、`ambiguous` | 参数缺失或错误 |
| 401 | `no_token`、`invalid_token` | 没有令牌，或令牌不是可用的 |
| 403 | `not_permitted` | 令牌只能读取，或该运行不是它启动的 |
| 404 | `not_found` | 没有这样的项目、运行或异常 |
| 409 | `busy`、`domain_off`、`nothing_to_back_up`、`not_running` | 已有其他操作在运行、领域已关闭，或无事可做 |
| 429 | `throttled`、`rate_limited`、`cooldown`、`retention_guard` | 某项限制暂缓了请求；`Retry-After` 说明何时重试 |

启动与 [通过 MCP 启动](mcp.md#starting-backups) 受同样的限制：每个令牌每小时 12 次，同一项目两次启动相隔 15 分钟，同一项目 24 小时内最多 4 次，以及保留保护。后三项把通过 MCP、API 和 Home Assistant 的启动合在一起计算。一个令牌每分钟最多 120 个请求。同一地址失败五次后会被锁定一分钟。

## OpenAPI {#openapi}

BombVault 在 `/api/v1/openapi.json` 提供这些路由的说明（OpenAPI 3.1）。不需要令牌。可以把它载入 Swagger UI、Postman 或代码生成器。

## Home Assistant {#home-assistant}

BombVault 可以通过 MQTT 发现，以设备的形式出现在 Home Assistant 中。Home Assistant 需要启用它的 MQTT 集成，并有一个代理，例如 Mosquitto 加载项。不需要任何专用组件。

1. 在 BombVault 中打开 **设置、系统、Home Assistant**。
2. 输入代理的地址和端口；如果代理要求，再输入用户名和密码。如果代理使用 TLS（通常在端口 8883），就打开 **使用 TLS**；它的证书必须对你输入的地址有效。
3. 打开 **连接到 Home Assistant** 并点击 **保存**。连接建立后，卡片会显示出来。

设备名为 BombVault，或在括号中带上实例名称的 BombVault，包含以下实体：

| 实体 | 显示内容 |
|---|---|
| Status | `ok`、`warning`、`failed` 或 `off`，即已启用领域中最差的状态 |
| Running job | 当前正在运行的操作，或 `idle` |
| Open anomalies | 未处理的异常数量 |
| Next scheduled backup | 下一次计划备份的开始时间 |
| *领域* last backup | 该领域最近一次成功备份的时间 |
| *领域* last result | 最近一次备份的结果 |
| *领域* repository free space | 其主仓库所在位置的剩余空间（如果 BombVault 能读取） |
| Back up *领域* | 备份整个领域的按钮 |

实体名称是英文的，因为 Home Assistant 会原样使用 BombVault 发送的名称。每个已启用的领域都有自己的实体，关闭的领域会失去它们。按钮与 [通过 API 启动](#errors) 受同样的限制。任何能向代理发布消息的人都能按下它们，所以请给代理设置密码，或关闭 **按钮启动备份**。

BombVault 每 15 秒读取一次自身状态，有变化时以 JSON 发布到 `<前缀>/<节点>/state`。前缀默认为 `bombvault`，除非你修改它；节点是 BombVault 选定一次的短 ID。发现消息发送到 Home Assistant 的默认前缀 `homeassistant`。两者都会保留（retained）。如果 BombVault 未通知就停止，遗嘱消息（last will）会把设备标为不可用。关闭这个连接后，BombVault 会从 Home Assistant 中移除该设备及其实体。

## 在网络中找到 BombVault {#mdns}

BombVault 通过 mDNS（Bonjour 和 Avahi 背后的协议）在局域网中广播自己的网页界面。浏览器随后可以用 `https://bombvault.local:3443` 打开它，使用 `HTTP_ONLY` 时为 `http://bombvault.local:3000`；服务浏览器会把它列为子类型 `_bombvault` 的网页服务。它的 TXT 记录包含版本和路径。开关位于 **设置、系统、在网络中查找**，默认打开。如果名称已被其他设备占用，BombVault 会改用 `bombvault-2.local` 等名称，卡片会显示它得到的地址。当 BombVault 停止或你关闭广播时，它会通知网络，浏览器会立即移除该条目。

广播能否到达你的网络，取决于容器的连接方式：

- **bridge**，Unraid 模板中的默认设置：广播留在 Docker 网络内部，局域网中没人能看到。请像以前一样通过主机地址打开 BombVault。
- **br0** 或其他 macvlan、ipvlan 网络：容器在局域网中有自己的地址，广播可以到达。
- **host**：广播从主机的网络接口发出，与 Unraid 自己的广播并列。

只广播 IPv4 地址。
