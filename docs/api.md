# API and integrations

BombVault has a small HTTP API for scripts, dashboards and home automation. It reads the same things the dashboard shows and can start a backup. Everything else, such as restores, deleting backups and settings, stays in the web interface.

## Tokens {#tokens}

Every request needs an API token, even when no login password is set. Create one under **Settings, System, API tokens**:

1. Type a name that says where the token is used, for example "Home Assistant" or "Uptime Kuma".
2. Switch **Allow starting backups** on if the token should be able to start backups. Without it the token can only read.
3. Click **Create token**. The token is shown once. BombVault keeps only a fingerprint of it, so copy it now.

Send the token in a header, either `Authorization: Bearer <token>` or `X-API-Key: <token>`. A token starts with `bvapi_`. It only opens the API: an MCP key does not work here, and a token does not work for MCP.

Each token has a tile with its name, whether it may start backups, the last four characters, when it was last used and from where, and its calls today. On the tile you can rename it, change what it may do, replace it or revoke it. **Log** shows the backups it started and its recent calls. Restoring BombVault's configuration from a backup revokes every token, because the backup may hold tokens you revoked later.

Without a login password, anyone who can open the web interface can also create a token. If you open BombVault under a public-looking name and no password is set, tokens cannot be created from that address, the same rule as for [MCP keys](mcp.md#switch-on).

## Endpoints {#endpoints}

| Route | What it returns or does | Token |
|---|---|---|
| `GET /api/v1/health` | Version, instance name, whether a backup is running, and what this token may do | read |
| `GET /api/v1/status` | Protection status per domain: last successful backup, expected interval, checks, next scheduled runs | read |
| `GET /api/v1/activity` | What is running right now, with phase and percentage | read |
| `GET /api/v1/items` | Every protected item with its schedule, what a backup stops and its last backup; `?domain=` for one domain | read |
| `GET /api/v1/runs` | Run history, newest first; filters `limit`, `domain`, `item`, `status`, `kind`, `since` | read |
| `GET /api/v1/anomalies` | Anomalies with a summary of what is open; filters `state`, `severity`, `domain`, `limit` | read |
| `GET /api/v1/anomalies/{id}` | One anomaly | read |
| `GET /api/v1/storage/{domain}` | Size history, growth per week, and free space of each repository of a domain | read |
| `POST /api/v1/backups` | Backs up one item (`{"domain":"containers","item":"plex"}`) or a whole domain (`{"domain":"vms"}`) | start |
| `POST /api/v1/backups/everything` | Runs the Backup Everything pass | start |
| `POST /api/v1/runs/{id}/cancel` | Cancels a running backup that this token started | start |

The domains are `containers`, `vms`, `files`, `zfs`, `flash` and `config`. Times are unix seconds. The answers are the same as those of the [MCP tools](mcp.md#tools) with the same names, so both stay in step.

A started backup is the same backup the web interface starts: a running container is stopped until its backup is done. The request returns at once, and `/api/v1/activity` and `/api/v1/runs` show how it goes.

## Examples {#examples}

```sh
# How are the backups doing?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Back up one container now.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

With BombVault's own self-signed certificate, add `--cacert bombvault-cert.pem` (the file **Download certificate** on the MCP card gives you) or `-k` on a network you trust.

## Errors and limits {#errors}

An error comes back as `{"error": {"code": "...", "message": "..."}}` with a matching status:

| Status | Codes | Meaning |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | An argument is missing or wrong |
| 401 | `no_token`, `invalid_token` | No token, or not an active one |
| 403 | `not_permitted` | The token may only read, or the run was not started by it |
| 404 | `not_found` | No such item, run or anomaly |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Something else is running, the domain is off, or there is nothing to do |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | A limit holds the request back; `Retry-After` says when to try again |

Starts follow the same limits as [starts through MCP](mcp.md#starting-backups): 12 per hour per token, 15 minutes between two starts of the same item, at most 4 starts of one item in 24 hours, and the retention guard. The last three count starts through MCP, the API and Home Assistant together. A token can make 120 requests a minute. Five failed attempts from one address lock it out for a minute.

## OpenAPI {#openapi}

BombVault serves a description of these routes at `/api/v1/openapi.json` (OpenAPI 3.1). It needs no token. Load it into Swagger UI, Postman or a code generator.

## Home Assistant {#home-assistant}

BombVault can appear in Home Assistant as a device, through MQTT discovery. Home Assistant needs its MQTT integration and a broker, for example the Mosquitto add-on. No custom component is involved.

1. In BombVault, open **Settings, System, Home Assistant**.
2. Enter the broker's address and port, and its user name and password if it asks for them. Switch **Use TLS** on if the broker takes TLS, usually on port 8883; its certificate has to be valid for the address you entered.
3. Switch **Connect to Home Assistant** on and click **Save**. The card shows when the connection is up.

The device is called BombVault, or BombVault with the instance name in brackets, and has these entities:

| Entity | What it shows |
|---|---|
| Status | `ok`, `warning`, `failed` or `off`, the worst of the domains that are switched on |
| Running job | What runs now, or `idle` |
| Open anomalies | How many anomalies are open |
| Next scheduled backup | When the next scheduled backup starts |
| *Domain* last backup | When the domain's last successful backup ran |
| *Domain* last result | How its last backup ended |
| *Domain* repository free space | Free space where its primary repository lives, if BombVault can read it |
| Back up *domain* | A button that backs up the whole domain |

Every switched-on domain gets its own entities, and a domain you switch off loses them. The buttons follow the same limits as [starts through the API](#errors). Anyone who can publish to the broker can press them, so give the broker a password, or switch **Buttons start backups** off.

BombVault reads its state every 15 seconds and publishes it when something changed, as JSON under `<prefix>/<node>/state`. The prefix is `bombvault` unless you change it, and the node is a short id BombVault picks once. The discovery messages go to Home Assistant's default prefix `homeassistant`. Both are retained. A last will marks the device unavailable if BombVault stops without saying so. Switching the link off removes the device and its entities from Home Assistant.
