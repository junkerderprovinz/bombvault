# MCP server

BombVault has a built-in server for the Model Context Protocol (MCP), the protocol AI assistants such as Claude Code and Claude Desktop use to reach outside tools. Through it an assistant can read how your backups are doing and, if you allow it, start a backup or cancel one it started. It is off until you create a key or switch on [sign-in through OAuth](#oauth): until then the endpoint `/mcp` answers `404` to everything.

## What an assistant can and cannot do {#tools}

| Tool | What it does | Kind |
|---|---|---|
| `get_health` | Version, instance name, whether a backup is running, and what this key may do | read |
| `get_status` | Protection status per domain: last successful backup, expected interval, verification and off-site checks, next scheduled runs | read |
| `get_coverage` | What BombVault protects and what it does not, with the reason for each | read |
| `list_items` | Every protected container, VM, folder set, the flash drive and the app configuration, with its schedule, what a backup of it stops, its last backup and how long that took; database containers also carry their last dump; ZFS datasets are listed too, with the result of their last check; a container recreated with other settings since its last backup lists what changed | read |
| `list_runs` | Run history, newest first, filterable by domain, item, status, kind and time; a slow backup that one thing held back names it | read |
| `list_restore_points` | Restore points of one item from its primary repository, and for a container its database dumps; a ZFS dataset gets one restore point per backup, with a snapshot of every dataset below it | read |
| `get_activity` | What is running right now, with phase and percentage | read |
| `get_storage_stats` | Size history of one domain's primary repository and its growth per week | read |
| `get_size_breakdown` | Which folders and files take the space in the newest backup of one container, VM or folder set, and how much of each the latest backup added | read |
| `list_anomalies` | Anomalies BombVault noticed in the backups, filterable by state, severity and domain, with a summary of what is open | read |
| `get_anomaly` | One of those findings, with the note left when it was acknowledged | read |
| `start_backup` | Backs up one item now | start |
| `start_domain_backup` | Backs up every protected item of one domain | start |
| `start_backup_everything` | Runs the Backup Everything pass | start |
| `cancel_backup` | Cancels a running backup that this key started | cancel |

These stay in the web interface: restores of any kind (including downloading, saving or importing a database dump), deleting backups, prune, unlock, checks and drills, off-site replication, settings, credentials and MCP keys, and cancelling a backup that the schedule, the web interface or another key started. The same goes for acknowledging an anomaly or marking it as expected, which happens on the **Anomalies** page. The reason is that tool output contains names and error messages from your server, and any of them could carry text written to steer the assistant. An assistant that falls for such text can at worst start a backup within the limits below, or cancel one it started itself.

If the primary repository of an item is remote (S3, REST, SFTP, rclone), `list_restore_points` contacts it, so the call can take a while. Off-site copies cannot be listed through MCP. What the anomaly checks look at is described under [Features](features.md), and how a ZFS item keeps one snapshot per dataset under [ZFS datasets](zfs-datasets.md#contents).

## What a started backup does {#starting-backups}

An assistant's backup is the same backup the web interface starts. A running container is stopped until its backup finishes, together with the containers set to stop with it. A VM with the graceful method is shut down and started again. A ZFS dataset stops the containers set for it while its snapshot is taken. Folder sets, the flash drive and the configuration keep running. Afterwards BombVault applies the retention policy and may copy to the off-site repository. `list_items` tells the assistant what an item stops and how long its last backup took, and the tool descriptions ask it to tell you before it starts anything.

Because a backup stops things and rotates old restore points out, starts through MCP are limited:

- 12 started backups per hour per key.
- 15 minutes between two MCP starts of the same item, domain or Backup Everything.
- At most 4 MCP starts of the same item in 24 hours.
- **Retention guard.** When a domain keeps a fixed number of restore points (only "keep last N", no daily, weekly or monthly rule, locally or on an off-site destination), every new backup pushes the oldest one out. BombVault then refuses an MCP start of an item whose newest N-1 successful backups were all started through MCP. At least one restore point that the schedule or you made therefore always stays in the kept set. With "keep last 1" an assistant cannot back that item up at all. The next scheduled backup makes room again.

A domain or Backup Everything start leaves out the items a limit holds back and names them in its answer. The web interface and the schedule are not limited by any of this. The hourly budget lives in memory, so a restart of BombVault resets it.

## Switch it on {#switch-on}

1. Open **Settings, System, MCP server** and click the button of your client. A client that is not listed connects through **Other client**.
2. Under **Key**, keep **New key** and the name it suggests, which is the client's, or type one that says where the key is used, for example "Claude Code on the laptop". One key per client lets you revoke one without touching the others. **Existing key** gives the client a key you made before.
3. Switch **Allow starting backups** on for a key that should be able to start backups; without it the key can only read. You can change it later on the key's tile, and the change applies to the assistant's next request without a reconnect.
4. Click **Create key**. The key is shown once. BombVault keeps only a fingerprint of it and cannot show it again, so copy it now. If you close the dialog before the client has used the key, the card keeps showing it until you confirm that you have copied it.

Without a login password the web interface itself is open to everyone on your network, and anyone who can open it can also create a key. The card says so. If you open BombVault under a public-looking name (for example `bombvault.example.com` through a reverse proxy) and no login password is set, keys cannot be created or replaced from that address, so that no web page on the internet can make your browser create one. Set a login password, or open BombVault by its IP address or a local name such as `tower` or `tower.local`.

## Your keys and their log {#keys}

Each key has a tile of its own on the card. It shows the key's name, whether it may start backups or only read, the last four characters of the key, when it was created or last replaced, when a client last used it and how many calls it made today. On the tile you rename the key, change its permission, replace it or revoke it. A revoked key moves to the list of revoked keys, where you can delete it for good once no run in the history names it.

Next to its name, the tile shows the mark of the client the key was made for. A key made through **Other client**, or before the card listed clients, shows a key instead.

**Log** on a tile opens what that key did. The backups it started come first, each with its state and a link to that run in the activity log on the dashboard. Below them are its calls, newest first, with the tool and what became of the call. A refusal says why: the key may only read, the retention guard held the backup back, another backup was already running, the same item, domain or pass was started through MCP a few minutes earlier, or the key sent too many requests. A cancel links to the run it was about.

BombVault keeps each key's entries for up to 30 days: the newest 500 successful starts and cancels, and next to them the newest 200 other calls (reads, refusals and errors). An assistant that keeps polling a running backup, or keeps retrying a refused call, cannot push the start of that backup out of the log. For each call it stores the tool, the outcome and the run a cancel named. It never stores what the assistant sent, and never the key or its fingerprint. The diagnostics bundle only counts the entries, and a settings export leaves them out.

## Connect a client {#clients}

Each client has a button on the card, under **On this computer** or **In the cloud**. The button opens a dialog in three steps: the key; the configuration for that client, with the address you opened the card at, a button to copy it, where the configuration lives and, with BombVault's own certificate, what the client needs to trust it; and the wait for the client's first call. The dialog watches the key's last use and turns green when that call comes in.

The dialog keeps the key off every command line. Where the client can read it from an environment variable (`BOMBVAULT_MCP_KEY`), a masked prompt or a file of its own, the configuration only names it. Where the client has no such way, the key sits in its configuration file or its settings, and the dialog says so. Where a client's documentation does not say how it treats a certificate it does not know, the dialog writes that step as what to do if the client refuses BombVault's certificate.

| Client | Setup | Where the key comes from |
|---|---|---|
| AnythingLLM | configuration file | the configuration file |
| Antigravity | configuration file | environment variable |
| Claude Code | command | key file |
| Claude Desktop | configuration file | key file |
| Cline | configuration file | the configuration file |
| Codex CLI | configuration file | environment variable |
| Continue | configuration file | `~/.continue/.env` |
| Copilot CLI | configuration file | the configuration file |
| Cursor | configuration file | environment variable |
| Gemini CLI | configuration file | environment variable |
| GitHub Copilot (VS Code) | configuration file | masked prompt |
| Goose | configuration file | environment variable |
| Jan | form in the app | the app's settings |
| JetBrains (AI Assistant, Junie) | configuration file | the configuration file |
| Kimi Code | configuration file | the configuration file |
| LM Studio | configuration file | the configuration file |
| Mistral Vibe | configuration file | environment variable |
| Msty | form in the app | the app's settings |
| n8n | form in the app | n8n's credential store |
| Open WebUI | form in the app | the app's settings |
| opencode | configuration file | environment variable |
| Perplexity (Mac) | form in the app | key file |
| Qwen Code | configuration file | environment variable |
| Roo Code | configuration file | environment variable |
| Visual Studio | configuration file | the configuration file |
| Warp | configuration file | the configuration file |
| Windsurf | configuration file | environment variable |
| Zed | configuration file | the configuration file |
| Grok | form, in the cloud | the vendor's servers |
| Le Chat | form, in the cloud | the vendor's servers |
| ChatGPT | sign-in through OAuth, in the cloud | an access token, see [below](#oauth) |
| Claude (claude.ai) | sign-in through OAuth, in the cloud | an access token, see [below](#oauth) |

The sections below explain the Claude Code and Claude Desktop setups in more detail and list what any other client needs.

### Claude Code {#claude-code}

Claude Code reaches BombVault through `mcp-remote`, which needs Node.js on that computer. First save the key in a text file of its own, as a single line:

```text
X-API-Key: <your key>
```

Then run the command from the card once in a terminal, with the path of that file filled in. Behind a certificate your computer trusts, it looks like this:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

With BombVault's own certificate (see [TLS and certificates](#tls)) the command also points Node.js at the downloaded certificate:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Check the connection with `/mcp` inside Claude Code. `--scope user` makes BombVault available in all your projects. Claude Code keeps only the path of the key file, so the key shows up neither in the command and your shell history nor in the process list. Keep the file where only you can read it, and outside any folder you commit. `@latest` makes `npx` fetch a current `mcp-remote`; an older one installed globally would be used instead, and it does not know `--header-file`.

Do not write `${BOMBVAULT_MCP_KEY}` into the `mcp-remote` arguments for Claude Code. Claude Code fills such a reference in from its own environment before it starts `mcp-remote`, so the key ends up on the command line of that process, where other programs and users of the computer can read it.

Without Node.js, and only behind a certificate your computer trusts, Claude Code can connect by itself. Put a `.mcp.json` into the project folder:

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

Set `BOMBVAULT_MCP_KEY` where Claude Code starts, for example under `"env"` in `~/.claude/settings.json` or in your shell profile, edited in a text editor rather than typed at the prompt. Here the reference is safe, because Claude Code starts no second process that would carry it. BombVault's own certificate does not work this way: Claude Code's own connection refuses it even with `NODE_EXTRA_CA_CERTS` set. Never commit a `.mcp.json` with the key written into it.

### Claude Desktop {#claude-desktop}

Claude Desktop reaches BombVault through `mcp-remote`, which needs Node.js on that computer. Save the key in a text file of its own first, as a single line, as described for [Claude Code](#claude-code). Open the configuration file through **Settings, Developer, Edit Config** in Claude Desktop. It lives at `%APPDATA%\Claude\claude_desktop_config.json` on Windows and at `~/Library/Application Support/Claude/claude_desktop_config.json` on macOS. Add the entry from the card inside `"mcpServers"`, next to any servers that are already there, and restart Claude Desktop:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote@latest", "https://192.168.1.10:3443/mcp", "--header-file", "<path of the file with your key>"],
      "env": {
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` is only there for BombVault's own certificate. Leave it out behind a certificate your computer already trusts.
- `--allow-http` is added only for a plain `http://` address.
- On Windows, write the paths with forward slashes, for example `C:/Users/sam/bombvault-key.txt`, because a single backslash is not valid JSON. Keep the key file's path free of spaces: Claude Desktop on Windows hands a path with a space to `npx` in two pieces.
- The configuration names only the key file, so the key shows up neither in it nor in the process list. Keep the file where only you can read it.

### Clients in the cloud {#cloud-clients}

ChatGPT, Claude on claude.ai, Grok and Le Chat call BombVault from their vendors' servers, so BombVault has to be reachable from the internet with a publicly trusted certificate, for example behind a reverse proxy; Le Chat refuses self-signed ones. A login on the proxy can protect the web interface, but `/mcp` has to pass through to BombVault without it: these services cannot sign in to a proxy, and BombVault checks their key or token itself. Grok and Le Chat send a fixed key, and their buttons set them up like the others. ChatGPT, and Claude on claude.ai in most organisations, connect only through sign-in with OAuth, described next.

### Sign-in through OAuth {#oauth}

For a client that cannot take a key, BombVault is its own OAuth authorization server. The client registers itself, sends you to a BombVault page, and there you sign in with your login password (and the second factor, if you set one up) and allow it. The client then gets a token that works only for the MCP endpoint of this BombVault, and renews it by itself.

1. Set a login password under **Settings, System**. Without one BombVault offers no sign-in at all, because there would be nobody to ask for consent.
2. Make BombVault reachable from the internet over https with a certificate browsers trust, usually through a reverse proxy. The client calls `/mcp`, `/oauth/` and `/.well-known/` from its own servers, so a proxy with a login of its own has to let those three paths through to BombVault. The consent page at `/oauth/authorize` opens in your own browser and may stay behind the proxy login. Name the proxy in `TRUSTED_PROXY` as well (see [Configuration](configuration.md)). BombVault limits client registrations per address, and without it every client seems to come from the proxy.
3. On the MCP card, switch on **Sign-in through OAuth** and enter the **Public address**: the https address without a path, for example `https://backup.example.com`. Every token is tied to this address, so after a change each client has to sign in again.
4. Click the ChatGPT or Claude button. The dialog shows the **Connector URL**, which is the public address plus `/mcp`, and where it goes in that client. In ChatGPT, switch on Developer mode under **Settings, Apps & Connectors, Advanced settings**, choose **Create**, paste the connector URL as the MCP server URL and pick OAuth as authentication. On claude.ai, open **Settings, Connectors, Add custom connector**, paste the connector URL, leave the OAuth client ID and secret empty and choose **Connect**.
5. The client opens the consent page. It shows who asks, where your answer sends you back to, and the **Allow starting backups** switch, which starts off. Choose **Allow** or **Deny**.

Each client that signed in gets a tile beside the keys, with its mark, its log, **Revoke** and **Allow starting backups**, and the same limits as a key. Revoking it takes effect at once. When the same client signs in again, its new grant replaces the old one, and a grant nobody used for 30 days expires. Up to 10 clients can be signed in at a time, on top of the 10 keys.

The consent page accepts a request only from a registered client that names one of its registered return addresses exactly: https, or a loopback address on any port for a client on your own computer. Only the authorization code flow with PKCE (S256) is accepted, and your answer is bound to your session, so no other website can send it for you. Access tokens last an hour. A refresh token is replaced each time it is used, and one that shows up again after that revokes the grant, because someone else holds a copy. A client that repeats its last refresh within 30 seconds, because the answer never reached it, gets new tokens instead. BombVault does not fetch client metadata documents from the internet, so clients register through dynamic client registration.

### Other clients {#other-clients}

Any client that speaks Streamable HTTP works:

- URL: the address of the web interface plus `/mcp`, for example `https://192.168.1.10:3443/mcp`.
- The key in `Authorization: Bearer <key>` or in `X-API-Key: <key>`. If both are sent they must carry the same key.
- `POST` with `Content-Type: application/json` and `Accept: application/json, text/event-stream`.
- One JSON-RPC message per request; batches are refused.
- Protocol versions 2026-07-28, 2025-11-25, 2025-06-18 and 2025-03-26.

## TLS and certificates {#tls}

BombVault serves HTTPS with a certificate it made itself, and at first that certificate names only `localhost`, `127.0.0.1` and `::1`. Claude Code and `mcp-remote` refuse it on a LAN address. The ways around that, in the order that suits most Unraid installs:

1. **Add the address in the MCP card.** When the card is opened over HTTPS at an address the certificate does not name, it says so and offers **Add this address to the certificate**. BombVault issues its certificate again with that address added (your browser warns once more, as it did the first time). Then click **Download certificate**; the snippets set `NODE_EXTRA_CA_CERTS` to the downloaded file, so the client trusts exactly this certificate. That also means that every client set up with an earlier download stops connecting as soon as the certificate is issued again, on this computer and on every other one, until it gets the new file.
2. **A reverse proxy with a trusted certificate** (Nginx Proxy Manager, SWAG, Caddy, Traefik). The client then sees the proxy's certificate and needs nothing extra, and the card does not warn about BombVault's own.
3. **Tailscale.** `tailscale serve` in front of the container, or the Unraid Tailscale integration, gives you a `ts.net` name with a trusted certificate.
4. **`HTTP_ONLY=true`**, only behind a proxy that ends TLS or on a network you fully trust. It switches the whole web interface to plain HTTP, needs a change to the container settings, and sends the key unencrypted.

Never set `NODE_TLS_REJECT_UNAUTHORIZED=0`. It switches certificate checks off for everything that Node.js process talks to.

A reverse proxy has to pass the `Authorization` (or `X-API-Key`) header through, which proxies do unless told otherwise, and must not buffer or rewrite `/mcp`. A location block for Nginx or Nginx Proxy Manager that also checks BombVault's certificate:

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

Behind a proxy every request carries the proxy's address. Five wrong keys from one misconfigured client then lock out every MCP client behind that proxy for a minute. Name the proxy in `TRUSTED_PROXY` (see [Configuration](configuration.md)) to count per client.

## Security model {#security}

- Without an active key and with sign-in through OAuth off, `/mcp` answers `404`.
- Sign-in through OAuth is only offered while a login password is set. Tokens, codes and client secrets are stored as fingerprints only, and a token works only for the address it was issued for.
- A client can register at most 10 times an hour from one address, and BombVault keeps at most 100 registered clients nobody signed in with, each for a day. Wrong codes and refresh tokens count towards the same lockout as wrong keys.
- Grants behave like keys when a configuration backup is restored or `APP_KEY` changes: after a restore every client has to sign in again.
- No exempt addresses. Requests from `localhost`, the Unraid host, a reverse proxy or `tailscale serve` need a key like any other, also when the web interface has no login password.
- Keys are stored as fingerprints only, shown once, and can be renamed, replaced and revoked. Up to 10 active keys, each with its own **Allow starting backups** switch.
- Every create, replace, permission change and revoke sends a notification through your notification channels, with the address it came from, unless notifications are switched off.
- 5 wrong keys per minute per address, then `429`. 120 requests per minute and 12 started backups per hour per key, plus the cooldown and the retention guard above.
- Requests from a browser page on another origin are refused.
- Keys cannot be created from a public-looking host name while no login password is set.
- Every backup an assistant starts, and the prune and off-site runs it causes, is marked "via MCP" with the key's name in the Activity log, in the error panel and in the backup notification.
- Every tool call is written to the container log with the key's id and last four characters (never its name) and counted in `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Restoring a configuration backup revokes every key, because the restored database may hold keys you revoked after it was saved. Create new keys afterwards.
- A key stops working when `APP_KEY` changes (a reinstall, or a restore onto another container). The card detects that and marks the key, and **Replace key** gives it a working secret again.
- Treat a key like a password. A client that cannot read the key from an environment variable, a prompt or a key file keeps it in plain text in its configuration or settings, and its dialog says so. Prefer a read-only key on a computer you trust less.

## What leaves the box {#privacy}

Whatever an assistant reads goes to the AI provider behind it: item names, schedules, run history with error messages, restore point ids and times, database engine names and dump sizes, current activity, storage figures, coverage and status. BombVault removes host paths, repository locations, host names, credentials, hook commands and keys before anything leaves.

## Troubleshooting {#troubleshooting}

| What you see | What it means |
|---|---|
| `404` | No active key and sign-in through OAuth is off, or a wrong path such as `/api/mcp`. The endpoint is `/mcp`. |
| `401` | The key is missing, mistyped, revoked or replaced. A proxy may be dropping the `Authorization` header (try `X-API-Key`). If the card marks the key as no longer valid, `APP_KEY` has changed: replace the key. |
| `403` | The request came from a browser page on another origin. Use a desktop or command-line client. |
| `405` on GET | Normal. The endpoint only takes `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | The client is too old for Streamable HTTP. Update it. |
| `400` "batch requests are not accepted" | The client sends JSON-RPC batches. Send one message per request. |
| `429` | Too many wrong keys from this address, or more than 120 requests a minute with one key. Wait a minute, and check whether the assistant is stuck in a loop. |
| "certificate", "self-signed" or "unable to verify" errors | The client does not trust BombVault's certificate. See [TLS and certificates](#tls). |
| `busy` | Another backup or maintenance job holds that domain. Try again when it has finished. |
| `cooldown` | This item, domain or Backup Everything was started through MCP less than 15 minutes ago. |
| `retention_guard` | One more MCP backup would leave only MCP-made restore points in a "keep last N" window, or the item already got 4 backups through MCP in the last 24 hours, failed and cancelled ones included. In the first case the next scheduled backup makes room, in the second the item is free again 24 hours after the oldest of those backups. Either way you can start it in the web interface. |
| `rate_limited` | The key has used its 12 starts for this hour. |
| `not_permitted` on a start | The key is read-only. Switch **Allow starting backups** on in the card; no reconnect needed. On a cancel it means the run was not started by this key. |
| `domain_off` | That backup type is switched off in Settings. |
| `not_found` | The item is not protected by BombVault. Add it in the web interface first; MCP never creates configuration. |
| The client cannot find the authorization server | Sign-in through OAuth is off, no login password is set, or the proxy does not let `/.well-known/` through to BombVault. |
| The consent page says the return address is not registered | The client sent a return address it did not register. Remove the connector in the client and add it again. |
| A signed-in client gets `401` | Its grant was revoked, it expired after 30 days without use, or the public address changed. The client signs in again. |

Do not set the environment variable `MCPGODEBUG` on the container. It changes how the MCP library behaves, and a malformed value stops BombVault at start before it writes a single log line.
