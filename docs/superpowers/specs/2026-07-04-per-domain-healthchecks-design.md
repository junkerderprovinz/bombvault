# Per-domain Healthchecks URLs — design

**Source:** GitHub issue #27 follow-up (ptmorris1): *"can we have 1 healthchecks per domain? I want to see
runtime stats per domain but currently healthchecks pings for any domain that runs. so every domain pings the
same check and no way to see what ran when and how long."*

**Branch:** `feat/per-domain-healthchecks` (off `main` == v4.4.1). **Stack:** Go backend + embedded SPA.

**Goal:** let the user point each backup domain (containers / VMs / flash / config) at its **own** Healthchecks
check, so each domain has its own runtime, history and start/success/fail lifecycle — while keeping the single
global URL as a fallback for domains left blank (backwards-compatible).

## Design

`notify.Config` already has one `HealthchecksURL` (the global). Add an optional **per-domain override map** and
resolve the effective URL per domain:

- **`HealthchecksByDomain map[string]string`** (JSON `healthchecksByDomain`) — keyed by the domain strings the
  notify layer already uses: `"container"`, `"VM"`, `"flash"`, `"config"` (the exact values passed to
  `notifyBackup`/`notifyBackupStart`). An empty/absent entry means "use the global URL".
- **`func (c Config) healthchecksURLFor(domain string) string`** — returns `c.HealthchecksByDomain[domain]`
  when non-empty, else `c.HealthchecksURL`. This is the single resolver every ping goes through.

**Semantics:** a per-domain URL, when set, **replaces** the global for that domain (pings only the domain's
check — that's the point, separate history per domain). Domains left blank fall back to the global. So a user
can set a global check for everything, or a distinct check per domain, or a mix.

### notify.go changes
- `pingHealthchecks` unchanged (phase-based, from v4.4.0).
- `Send(ctx, c, domain, ev)` — add a `domain` param; ping `c.healthchecksURLFor(domain)` instead of
  `c.HealthchecksURL`. The message-channel (webhook/matrix/smtp) behaviour and the `On`-policy decoupling from
  v4.4.0 are unchanged.
- `SendStart(ctx, c, domain)` — add `domain`; resolve via `healthchecksURLFor(domain)`; the guard now checks
  the RESOLVED url is non-empty (not the global specifically) in addition to `On != never`.
- `SendTest(ctx, c)` — ping EVERY configured Healthchecks URL (the global plus each distinct non-empty
  per-domain value), so the Test button validates whatever the user has set, not just the global. De-dup so a
  URL shared across domains is pinged once.
- `Configured()` — also true when any per-domain URL is set (so notifications count as configured even with no
  global URL).

### service.go changes
- `notifyBackupStart(ctx, domain string)` — add the `domain` param; pass `notify.SendStart(ctx, c, domain)`.
  Update the 4 call sites to pass their domain: `"container"` (`:1980`), `"VM"` (`:3930`), `"flash"`
  (`:4274`), `"config"` (`:4414`).
- `notifyBackup` already has `domain`; change its `notify.Send(ctx, c, ev)` call to
  `notify.Send(ctx, c, domain, ev)`.
- The 4 non-backup `notify.Send` callers (offsite-over-budget, replication-failed, drill-failure,
  protection-lost) — pass their domain (each has one) so a per-domain Healthchecks URL also catches those
  failure events; where a caller has no meaningful domain, pass `""` (resolver falls back to the global).

### Frontend (`web/src/pages/Settings.tsx`)
Under the existing global Healthchecks URL field, add an optional **"Per-domain checks (advanced)"** group:
four URL inputs labelled Containers / VMs / Flash / Config, each bound to
`notifyConfig.healthchecksByDomain[<key>]` (keys `container`/`VM`/`flash`/`config`), with a hint that a blank
field falls back to the global URL. The `NotifyConfig` TS type gains
`healthchecksByDomain?: Record<string, string>`. Persist via the existing notify-config save path (no new
endpoint). i18n keys for the group label + per-domain hint (the domain names reuse existing nav/domain keys
where possible), en+de + all 24 locales.

## Testing
- `healthchecksURLFor`: per-domain set → returns it; blank → returns global; unknown domain → global.
- `Send`/`SendStart` ping the per-domain URL for a domain that has one, and the global for one that doesn't
  (httptest recorders per URL).
- `SendTest` pings the global + each distinct per-domain URL exactly once.
- `Configured()` true when only a per-domain URL is set.
- Frontend gate: tsc + build.

## Out of scope (YAGNI)
- Auto-provisioning Healthchecks checks via its management API / ping-key slugs (the user sets explicit URLs).
- Per-container / per-VM (individual item) checks — per-DOMAIN is what was asked.

## File map
- **Modify (backend):** `internal/notify/notify.go` (+ test), `internal/api/service.go`
  (notifyBackupStart signature + the notify.Send/SendStart call sites).
- **Modify (frontend):** `web/src/pages/Settings.tsx`, `web/src/lib/api.ts` (NotifyConfig type),
  `web/src/lib/i18n.ts` + `web/src/lib/locales/*.ts` (24).
- **Create:** this spec + the plan.
