# MCP 服务器

BombVault 内置了一个 Model Context Protocol（MCP）服务器。MCP 是 Claude Code、Claude Desktop 等 AI 助手用来访问外部工具的协议。助手可以通过它读取你的备份状况；如果你允许，它还可以启动一次备份，或取消它自己启动的备份。在你创建密钥之前，服务器处于关闭状态：没有有效密钥时，连接地址 `/mcp` 对所有请求都返回 `404`。

## 助手能做什么，不能做什么 {#tools}

| 工具 | 作用 | 类型 |
|---|---|---|
| `get_health` | 版本、实例名称、是否有备份正在运行，以及这个密钥被允许做什么 | 读取 |
| `get_status` | 各个域的保护状态：最近一次成功备份、预期间隔、校验和异地检查、接下来的计划运行 | 读取 |
| `get_coverage` | BombVault 保护了什么、没有保护什么，并给出各自的原因 | 读取 |
| `list_items` | 每个受保护的容器、虚拟机和文件夹集、闪存盘以及应用配置，附带计划、备份时会停止什么、最近一次备份及其耗时；数据库容器还会列出最近一次转储；ZFS 数据集也会列出，并附上次检查的结果 | 读取 |
| `list_runs` | 运行历史，最新的在前，可按域、项目、状态、类型和时间筛选 | 读取 |
| `list_restore_points` | 一个项目在其主仓库中的还原点；对于容器，还包括它的数据库转储；ZFS 数据集每次备份对应一个还原点，其中包含它下面每个数据集的快照 | 读取 |
| `get_activity` | 此刻正在运行的内容，带阶段和百分比 | 读取 |
| `get_storage_stats` | 某个域主仓库的大小历史及每周增长 | 读取 |
| `list_anomalies` | BombVault 在备份中注意到的异常，可按状态、严重程度和域筛选，并附有未处理项的摘要 | 读取 |
| `get_anomaly` | 其中一条，附确认时留下的备注 | 读取 |
| `start_backup` | 立即备份一个项目 | 启动 |
| `start_domain_backup` | 备份一个域中所有受保护的项目 | 启动 |
| `start_backup_everything` | 运行一轮 Backup Everything | 启动 |
| `cancel_backup` | 取消由这个密钥启动、正在运行的备份 | 取消 |

以下操作只能在网页界面中进行：任何形式的还原（包括下载、保存或导入数据库转储）、删除备份、prune、unlock、检查和演练、异地复制、设置、凭据和 MCP 密钥，以及取消由计划、网页界面或其他密钥启动的备份。确认异常或将其标记为预期同样如此，这些在 **异常** 页面中完成。原因是：工具的回答中包含来自你服务器的名称和错误信息，其中任何一条都可能夹带专门用来操纵助手的文字。受这类文字蒙骗的助手，最坏也只能在下面的限制范围内启动一次备份，或取消它自己启动的备份。

如果某个项目的主仓库在别处（S3、REST、SFTP、rclone），`list_restore_points` 会去连接它，调用可能需要一些时间。异地副本无法通过 MCP 列出。

## 启动的备份会做什么 {#starting-backups}

助手启动的备份与网页界面启动的备份完全相同。正在运行的容器会在其备份完成前停止，设置为随它一起停止的容器也会停止。采用 "graceful" 方式的虚拟机会被关机，然后重新启动。ZFS 数据集在拍摄快照期间会停止为其设置的容器。文件夹集、闪存盘和配置会继续运行。之后 BombVault 会执行保留策略，并可能复制到异地仓库。`list_items` 会告诉助手某个项目会停止什么、最近一次备份用了多久，而工具说明要求它在启动任何操作之前先告诉你。

由于备份会停止服务并把旧的还原点挤出去，通过 MCP 的启动是有限制的：

- 每个密钥每小时最多启动 12 次备份。
- 同一项目、同一个域或 Backup Everything 的两次 MCP 启动之间间隔 15 分钟。
- 同一项目在 24 小时内最多 4 次 MCP 启动。
- **保留保护。** 当某个域只保留固定数量的还原点（只有"保留最近 N 个"，没有按日、按周或按月的规则，无论本地还是异地目标）时，每次新备份都会把最旧的那个挤出去。这时，如果某个项目最近 N-1 次成功备份全部是通过 MCP 启动的，BombVault 会拒绝对它的 MCP 启动。因此，保留下来的集合中始终至少有一个由计划或由你创建的还原点。设置为"保留最近 1 个"时，助手完全无法备份该项目。下一次计划备份会重新腾出空间。

启动一个域或 Backup Everything 时，被某项限制挡下的项目会被跳过，并在回答中列出名称。这些限制都不影响网页界面和计划。每小时的额度保存在内存中，所以重启 BombVault 会将其清零。

## 开启 {#switch-on}

1. 打开 **设置、系统、MCP 服务器**，点击 **新密钥**。
2. 给密钥起一个能说明用途的名字，例如"笔记本上的 Claude Code"。每个客户端一个密钥，这样你可以撤销其中一个而不影响其他。
3. 保持 **允许启动备份** 为开启，或者对只需读取的密钥关闭它。之后可以在该密钥所在行修改，修改从助手的下一次请求起生效，无需重新连接。
4. 点击 **创建密钥**。密钥只显示一次。BombVault 只保存它的指纹，无法再次显示，所以请立即复制，或使用下方的某个片段，此时片段中包含真实密钥。

没有登录密码时，网页界面本身对你网络中的所有人开放，能打开它的人也能创建密钥。卡片上会提示这一点。如果你用一个看起来像公网的名字打开 BombVault（例如反向代理后面的 `bombvault.example.com`），并且没有设置登录密码，那么从该地址无法创建或更换密钥，这样互联网上的任何网页都无法让你的浏览器去创建密钥。请设置登录密码，或者通过 IP 地址或 `tower`、`tower.local` 这样的本地名称打开 BombVault。

## 连接客户端 {#clients}

卡片会针对你打开它时使用的地址显示现成的片段：选择客户端并复制片段即可。下面说明这些片段的作用，并给出卡片没有显示的写法。

### Claude Code {#claude-code}

在终端中运行一次卡片里的命令。使用你的电脑信任的证书时，它是这样的：

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

在 Claude Code 中用 `/mcp` 检查连接。`--scope user` 会把密钥保存在你的用户配置中，而不是项目文件里。

这条命令包含密钥，你的 shell 可能会把它留在历史记录里。为避免这种情况，可以在项目文件夹中放一个 `.mcp.json`，并把密钥放在环境变量中。Claude Code 读取文件时会替换 `${BOMBVAULT_MCP_KEY}`：

```json
{
  "mcpServers": {
    "bombvault": {
      "type": "http",
      "url": "https://bombvault.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${BOMBVAULT_MCP_KEY}"
      }
    }
  }
}
```

在 Claude Code 启动的环境中设置 `BOMBVAULT_MCP_KEY`，例如写在 shell 的配置文件里，用文本编辑器写，而不是在命令行中输入。绝不要提交写有密钥的 `.mcp.json`。

使用 BombVault 自己的证书时（见 [TLS 与证书](#tls)），卡片里的命令改为运行 `mcp-remote`，并让 Node.js 使用下载的证书：

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

单引号阻止 shell 展开这个变量；展开由 `mcp-remote` 自己完成。同样的写法也适用于 `.mcp.json`：使用下面 Claude Desktop 的条目，并从它的 `env` 中去掉 `BOMBVAULT_MCP_KEY`，密钥就会来自你的环境。

### Claude Desktop {#claude-desktop}

Claude Desktop 通过 `mcp-remote` 连接 BombVault，这需要该电脑上装有 Node.js。在 Claude Desktop 中通过 **Settings, Developer, Edit Config** 打开配置文件。它在 Windows 上位于 `%APPDATA%\Claude\claude_desktop_config.json`，在 macOS 上位于 `~/Library/Application Support/Claude/claude_desktop_config.json`。把卡片中的条目加到 `"mcpServers"` 里，放在已有服务器旁边，然后重启 Claude Desktop：

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://192.168.1.10:3443/mcp", "--header", "X-API-Key:${BOMBVAULT_MCP_KEY}"],
      "env": {
        "BOMBVAULT_MCP_KEY": "<your key>",
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` 只为 BombVault 自己的证书而设。如果证书已被你的电脑信任，请去掉它。
- 只有纯 `http://` 地址才会加上 `--allow-http`。
- 请求头写作 `X-API-Key:${BOMBVAULT_MCP_KEY}`，冒号后不加空格，密钥放在 `env` 中。在某些系统上，`mcp-remote` 会在第一个空格处拆开 `--header` 的值，写在空格之后的密钥就会丢失。

### Claude 设置中的自定义连接器 {#custom-connectors}

在 Claude 自身的设置中添加的连接器（claude.ai 上以及 Claude Desktop 的连接器列表中）目前还不受支持。这些连接器是从 Anthropic 的云端访问的，因此需要一个公网 HTTPS 地址，并且通过 OAuth 登录。它们无法发送固定密钥，而 BombVault 只提供固定密钥，没有 OAuth 登录。为此把 BombVault 暴露到互联网上也无济于事。请使用 Claude Code，或按上文通过 `mcp-remote` 使用 Claude Desktop。

### 其他客户端 {#other-clients}

任何支持 Streamable HTTP 的客户端都可以：

- URL：网页界面的地址加上 `/mcp`，例如 `https://192.168.1.10:3443/mcp`。
- 密钥放在 `Authorization: Bearer <key>` 或 `X-API-Key: <key>` 中。如果两者都发送，必须是同一个密钥。
- 使用 `POST`，并带上 `Content-Type: application/json` 和 `Accept: application/json, text/event-stream`。
- 每个请求一条 JSON-RPC 消息；批量请求（batch）会被拒绝。
- 协议版本：2026-07-28、2025-11-25、2025-06-18 和 2025-03-26。

## TLS 与证书 {#tls}

BombVault 使用自己签发的证书提供 HTTPS，起初这张证书只包含 `localhost`、`127.0.0.1` 和 `::1`。Claude Code 和 `mcp-remote` 在局域网地址上会拒绝它。解决办法按适合大多数 Unraid 安装的顺序如下：

1. **在 MCP 卡片中添加地址。** 如果通过 HTTPS 在证书未包含的地址上打开卡片，卡片会指出这一点，并提供 **把这个地址加进证书**。BombVault 随后会重新签发包含该地址的证书（浏览器会像第一次那样再警告一次）。然后点击 **下载证书**；片段会把 `NODE_EXTRA_CA_CERTS` 设为下载的文件，客户端就会信任这张证书。
2. **带受信任证书的反向代理**（Nginx Proxy Manager、SWAG、Caddy、Traefik）。客户端看到的是代理的证书，无需其他操作，卡片也不会就 BombVault 自己的证书发出警告。
3. **Tailscale。** 在容器前使用 `tailscale serve`，或使用 Unraid 的 Tailscale 集成，可以得到一个带受信任证书的 `ts.net` 名称。
4. **`HTTP_ONLY=true`**，只能在终止 TLS 的代理之后或你完全信任的网络中使用。它会把整个网页界面切换为纯 HTTP，需要修改容器设置，并且以未加密的方式发送密钥。

绝不要设置 `NODE_TLS_REJECT_UNAUTHORIZED=0`。这会让该 Node.js 进程对所有通信对象都跳过证书校验。

反向代理必须转发 `Authorization`（或 `X-API-Key`）请求头（除非另有配置，代理都会这样做），并且不能缓冲或改写 `/mcp`。下面是一个同时校验 BombVault 证书的 Nginx 或 Nginx Proxy Manager 的 location 块：

```nginx
location /mcp {
    proxy_pass https://192.168.1.10:3443;
    proxy_ssl_verify on;
    proxy_ssl_trusted_certificate /data/bombvault-cert.pem;
    proxy_ssl_name localhost;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_set_header Host $host;
}
```

在代理之后，每个请求都带着代理的地址。于是一个配置错误的客户端发送 5 次错误密钥，就会让该代理后面的所有 MCP 客户端被锁定一分钟。在 `TRUSTED_PROXY` 中填写代理（见 [配置](configuration.md)），即可按客户端分别计数。

## 安全模型 {#security}

- 没有有效密钥时，`/mcp` 返回 `404`。
- 没有任何地址被豁免。来自 `localhost`、Unraid 主机、反向代理或 `tailscale serve` 的请求都和其他请求一样需要密钥，即使网页界面没有登录密码也是如此。
- 密钥只以指纹形式保存，只显示一次，可以重命名、更换和撤销。最多 10 个有效密钥，每个都有自己的 **可以启动备份** 开关。
- 每次创建、更换、修改权限和撤销，只要通知没有关闭，都会通过你的通知渠道发送通知，并附上请求来源地址。
- 每个地址每分钟 5 次错误密钥后返回 `429`。每个密钥每分钟 120 个请求、每小时启动 12 次备份，另外还有上文所述的等待时间和保留保护。
- 来自其他来源（origin）的浏览器页面的请求会被拒绝。
- 在没有设置登录密码时，无法从看起来像公网的主机名创建密钥。
- 助手启动的每一次备份，以及由此引发的 prune 和异地运行，都会在活动日志、错误面板和备份通知中标注"通过 MCP"以及密钥名称。
- 每次工具调用都会连同密钥的 ID 和最后四个字符（从不包含名称）写入容器日志，并在 `/metrics` 中计数（`bombvault_mcp_requests_total`、`bombvault_mcp_tool_calls_total`、`bombvault_mcp_active_keys`）。
- 还原配置备份会撤销所有密钥，因为还原的数据库中可能含有你在它保存之后才撤销的密钥。之后请创建新的密钥。
- 当 `APP_KEY` 改变时（重新安装，或还原到另一个容器），密钥会失效。卡片会检测到并标记该密钥，**更换密钥** 会重新给它一个有效的密钥值。
- 像对待密码一样对待密钥。Claude Code 和 Claude Desktop 会在配置中以明文保存它。在你不太信任的电脑上，最好使用只读密钥。

## 哪些信息会离开本机 {#privacy}

助手读取的所有内容都会发送给它背后的 AI 提供商：项目名称、计划、含错误信息的运行历史、还原点的 ID 和时间、数据库引擎名称和转储大小、当前活动、存储数据、覆盖范围和状态。在任何内容离开之前，BombVault 会去除主机路径、仓库位置、主机名、凭据、钩子命令和密钥。

## 故障排除 {#troubleshooting}

| 你看到的 | 含义 |
|---|---|
| `404` | 没有有效密钥，或者路径错误，例如 `/api/mcp`。连接地址是 `/mcp`。 |
| `401` | 密钥缺失、输错、已撤销或已更换。可能是代理丢掉了 `Authorization` 请求头（试试 `X-API-Key`）。如果卡片把密钥标记为已失效，说明 `APP_KEY` 已改变：请更换密钥。 |
| `403` | 请求来自其他来源的浏览器页面。请使用桌面或命令行客户端。 |
| GET 时返回 `405` | 正常。连接地址只接受 `POST`。 |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | 客户端太旧，不支持 Streamable HTTP。请更新它。 |
| `400` "batch requests are not accepted" | 客户端在发送 JSON-RPC 批量请求。每个请求只发一条消息。 |
| `429` | 该地址的错误密钥过多，或一个密钥每分钟的请求超过 120 个。等一分钟，并检查助手是否陷入了循环。 |
| 含有 "certificate"、"self-signed" 或 "unable to verify" 的错误 | 客户端不信任 BombVault 的证书。见 [TLS 与证书](#tls)。 |
| `busy` | 另一个备份或维护任务正在占用该域。等它结束后再试。 |
| `cooldown` | 这个项目、这个域或 Backup Everything 在 15 分钟内已通过 MCP 启动过。 |
| `retention_guard` | 再做一次 MCP 备份，"保留最近 N 个"的窗口里就只剩来自 MCP 的还原点了。下一次计划备份会腾出空间，或者在网页界面中启动它。 |
| `rate_limited` | 该密钥本小时的 12 次启动已用完。 |
| 启动时出现 `not_permitted` | 该密钥为只读。在卡片中打开 **可以启动备份**；无需重新连接。如果出现在取消时，表示该运行不是由这个密钥启动的。 |
| `domain_off` | 这种备份类型在设置中已关闭。 |
| `not_found` | BombVault 没有保护这个项目。请先在网页界面中添加；MCP 从不创建配置。 |

不要在容器上设置环境变量 `MCPGODEBUG`。它会改变 MCP 库的行为，而错误的值会让 BombVault 在启动时、甚至还没写下一行日志时就停止。
