# Per-domain Healthchecks URLs — implementation plan

> REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use `- [ ]`.

**Goal:** each backup domain can point at its own Healthchecks check, falling back to the global URL (#27
follow-up). **Spec:** `docs/superpowers/specs/2026-07-04-per-domain-healthchecks-design.md`.

## Global Constraints
- Branch `feat/per-domain-healthchecks` (off `main` == v4.4.1). Sequential implementers.
- Go gates: `go build ./... && go vet ./...`, `gofmt -l` empty, `go test ./... -count=1`,
  `golangci-lint run ./internal/...` 0. Frontend: `tsc --noEmit` + `npm run build`.
- No AI attribution (controller commits). i18n en+de then all 24 locales.

---

### Task 1: notify.go — per-domain URL resolver + threaded domain
**Files:** `internal/notify/notify.go`, `internal/notify/notify_test.go`.

- [ ] Add field to `Config`: `HealthchecksByDomain map[string]string` with `json:"healthchecksByDomain"`
  (place near `HealthchecksURL`).
- [ ] Add resolver:
```go
// healthchecksURLFor returns the per-domain Healthchecks URL when one is set for
// domain, otherwise the global HealthchecksURL. A per-domain URL replaces (does
// not add to) the global for that domain.
func (c Config) healthchecksURLFor(domain string) string {
	if u := c.HealthchecksByDomain[domain]; u != "" {
		return u
	}
	return c.HealthchecksURL
}
```
- [ ] `Send` — change signature to `Send(ctx context.Context, c Config, domain string, ev Event)`; replace the
  `c.HealthchecksURL` used in the Healthchecks ping block with `hcURL := c.healthchecksURLFor(domain)` and
  guard `if hcURL != ""`. Message channels unchanged.
- [ ] `SendStart` — change to `SendStart(ctx context.Context, c Config, domain string)`; resolve
  `hcURL := c.healthchecksURLFor(domain)`; guard `if (c.On != "always" && c.On != "failure") || hcURL == "" { return }`; ping `hcURL`.
- [ ] `SendTest` — ping every distinct configured Healthchecks URL (global + each non-empty
  `HealthchecksByDomain` value), de-duplicated, returning the first error. Build the set:
```go
seen := map[string]bool{}
var urls []string
for _, u := range append([]string{c.HealthchecksURL}, mapValues(c.HealthchecksByDomain)...) {
	if u != "" && !seen[u] {
		seen[u] = true
		urls = append(urls, u)
	}
}
// ping each with pingHealthchecks(ctx, client, u, "success"); return first error
```
  (add a tiny `mapValues` helper or inline the loop over the map).
- [ ] `Configured()` — return true also when `len(nonEmpty(c.HealthchecksByDomain)) > 0` (any per-domain URL
  set), in addition to the existing checks.
- [ ] Tests (httptest recorders):
  - `healthchecksURLFor`: domain with an entry → that URL; blank/absent → global; unknown domain → global.
  - `Send(..., "flash", okEvent)` with `HealthchecksByDomain{"flash": srvA}` + global `srvB` → srvA pinged, srvB not.
  - `Send(..., "config", ...)` with no config entry → global srvB pinged.
  - `SendStart(..., "flash")` pings the flash URL; a domain with no per-domain URL pings the global.
  - `SendTest` pings global + each distinct per-domain URL once (count the hits across recorders).
  - `Configured()` true with only a per-domain URL set.
- [ ] Gates + commit: `feat(notify): per-domain Healthchecks URLs with global fallback (#27)`.

---

### Task 2: service.go — thread the domain into start + send
**Files:** `internal/api/service.go`, `internal/api/notify_internal_test.go`.

- [ ] `notifyBackupStart` — change to `func (s *Service) notifyBackupStart(ctx context.Context, domain string)`;
  call `notify.SendStart(ctx, c, domain)`.
- [ ] Update the 4 start call sites to pass the domain matching their `notifyBackup` domain:
  `s.notifyBackupStart(ctx, "container")` (~:1980), `"VM"` (~:3930), `"flash"` (~:4274), `"config"` (~:4414).
- [ ] `notifyBackup` — change its `notify.Send(ctx, c, notify.Event{...})` to
  `notify.Send(ctx, c, domain, notify.Event{...})` (domain is already a param of notifyBackup).
- [ ] The 4 non-backup `notify.Send(ctx, c, notify.Event{...})` callers (grep `notify.Send(` — offsite
  over-budget, replication-failed, drill-failure, protection-lost): pass each one's domain. Read each; most
  have a `domain` in scope (e.g. `notifyReplicationFailed(ctx, domain, ...)`). Where none exists, pass `""`
  (resolver falls back to global). Note which got `""`.
- [ ] Test: extend notify_internal_test — `notifyBackupStart(ctx, "flash")` with
  `HealthchecksByDomain{"flash": srv.URL}` pings that URL; a domain without an entry pings the global.
- [ ] Gates + commit: `feat(notify): route start/done pings to the domain's Healthchecks URL (#27)`.

---

### Task 3: Frontend + i18n
**Files:** `web/src/pages/Settings.tsx`, `web/src/lib/api.ts`, `web/src/lib/i18n.ts` (en+de), then `locales/*.ts` (24).

- [ ] `api.ts`: add `healthchecksByDomain?: Record<string, string>` to the `NotifyConfig` type (near
  `healthchecksUrl`).
- [ ] `Settings.tsx`: under the global Healthchecks URL field (grep `healthchecksUrl`), add a
  "Per-domain checks (advanced)" group: four URL `<input>`s labelled Containers / VMs / Flash / Config, bound
  to `cfg.healthchecksByDomain?.[key]` for keys `container`/`VM`/`flash`/`config`; onChange updates the map
  immutably (`{...cfg.healthchecksByDomain, [key]: value}`, deleting the key when blanked is optional). A hint
  says a blank field falls back to the global URL. Reuse the existing field/hint styling + the existing notify
  save path (do not invent persistence).
- [ ] i18n en+de: `notify.hcPerDomain` (group label), `notify.hcPerDomainHint` (fallback note). The domain
  names can reuse existing keys (`nav.containers`/`nav.vms`/`nav.flash`/`nav.config` or `dashboard.domain*`) —
  grep and reuse; only add new keys if none fit. Grep to avoid collisions.
- [ ] Gate `tsc --noEmit` + `npm run build` (don't commit web/dist). Commit: `feat(notify): per-domain Healthchecks fields in Settings (#27)`.
- [ ] Add the new `notify.hcPerDomain*` keys to all 24 locale files (disjoint-partition translators, no nested
  delegation). Verify each has them once; tsc; build (commit web/dist locales). Commit:
  `i18n(notify): per-domain Healthchecks keys in all 24 locale files (#27)`.

## Self-review
- Spec coverage: resolver + map (T1), domain threading (T2), UI + fallback (T3). Covered.
- Types: `healthchecksURLFor`/`HealthchecksByDomain` (T1) used by Send/SendStart (T1) + service (T2);
  `healthchecksByDomain` DTO (T3) round-trips through the existing NotifyConf blob.

## Handoff
Subagent-driven, sequential. After T3: `/code-review` on the diff, then release **v4.5.0** (additive) + answer
+ close #27.
