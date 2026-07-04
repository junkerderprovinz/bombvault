# Healthchecks start ping + lifecycle decoupling — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use `- [ ]`.

**Goal:** Healthchecks gets a `/start` ping at backup start + success/`/fail` on done, decoupled from the
message `On` policy (#27). **Spec:** `docs/superpowers/specs/2026-07-04-healthchecks-start-ping-design.md`.

## Global Constraints
- Branch `feat/healthchecks-start-ping` (off `main` == v4.3.0). Sequential implementers.
- Go gates: `go build ./... && go vet ./...`, `gofmt -l` empty, `go test ./... -count=1`,
  `golangci-lint run ./internal/...` 0. Frontend: `tsc --noEmit` + `npm run build`.
- No AI attribution (controller commits). i18n en+de then all 24 locales.

---

### Task 1: notify.go — phase-based Healthchecks + decoupled Send + SendStart
**Files:** `internal/notify/notify.go`, `internal/notify/notify_test.go` (+ the redact test file if present).

- [ ] Change `pingHealthchecks(ctx, client, base string, ok bool)` → `pingHealthchecks(ctx, client, base, phase string)`, `phase` ∈ `"start"|"success"|"fail"`:
```go
// pingHealthchecks pings the check for a lifecycle phase: "start" (<base>/start),
// "success" (<base>) or "fail" (<base>/fail).
func pingHealthchecks(ctx context.Context, client *http.Client, base, phase string) error {
	u := strings.TrimRight(base, "/")
	switch phase {
	case "start":
		u += "/start"
	case "fail":
		u += "/fail"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return do(client, req)
}
```
- [ ] Restructure `Send` so Healthchecks fires on both outcomes regardless of the failure/always policy (but not for "never"), and the message channels keep honouring `On`:
```go
func Send(ctx context.Context, c Config, ev Event) {
	if c.On == "never" || c.On == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	client := &http.Client{Timeout: sendTimeout}

	// Healthchecks is a monitor, not a human message: it must get the success ping
	// to stay green, so it fires on both outcomes whenever configured — the On
	// policy governs only the message channels below.
	if c.HealthchecksURL != "" {
		phase := "success"
		if !ev.OK {
			phase = "fail"
		}
		if err := pingHealthchecks(ctx, client, c.HealthchecksURL, phase); err != nil {
			log.Printf("notify: healthchecks: %v", redactErr(err))
		}
	}

	if !c.shouldSend(ev.OK) {
		return
	}
	// webhook / matrix / smtp — KEEP the existing three blocks exactly as they are.
	...
}
```
  (Move the existing webhook/matrix/smtp blocks below the shouldSend gate; delete the old Healthchecks block that was among them. The old top-of-func `if !c.shouldSend(ev.OK) { return }` is replaced by the never-guard at top + the shouldSend gate before the message channels.)
- [ ] Add `SendStart`:
```go
// SendStart pings the Healthchecks check's /start endpoint at the beginning of a
// backup, so the check can measure duration and detect a hung/never-finished run.
// Healthchecks-only (message channels have no "start" concept); best-effort.
func SendStart(ctx context.Context, c Config) {
	if c.On == "never" || c.On == "" || c.HealthchecksURL == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	client := &http.Client{Timeout: sendTimeout}
	if err := pingHealthchecks(ctx, client, c.HealthchecksURL, "start"); err != nil {
		log.Printf("notify: healthchecks start: %v", redactErr(err))
	}
}
```
- [ ] Update `SendTest`'s `pingHealthchecks(..., true)` call to the new phase form: `pingHealthchecks(ctx, client, c.HealthchecksURL, "success")`.
- [ ] Tests (`httptest.Server` recording the request path):
  - `Send` `On=failure` + `OK=true` + HealthchecksURL → server sees `GET /` (base, success), and NO webhook fired (point WebhookURL at a second recorder and assert it's untouched).
  - `Send` `OK=false` → server sees `/fail`.
  - `SendStart` → server sees `/start`; `On=never` or empty URL → server sees nothing.
  - `pingHealthchecks` phase→path (start/success/fail) via the recorder.
- [ ] Gates + commit: `feat(notify): Healthchecks lifecycle — start ping + success independent of On policy (#27)`.

---

### Task 2: service.go — notifyBackupStart wired into the 4 backup fns
**Files:** `internal/api/service.go`, `internal/api/notify_internal_test.go`.

- [ ] Add `notifyBackupStart` near `notifyBackup` (service.go:5886):
```go
// notifyBackupStart pings the Healthchecks /start endpoint at the beginning of a
// backup (best-effort; never affects the backup). The message channels have no
// "start" concept, so this is Healthchecks-only.
func (s *Service) notifyBackupStart(ctx context.Context) {
	c, err := s.NotifyConfig()
	if err != nil {
		return
	}
	notify.SendStart(ctx, c)
}
```
- [ ] Call `s.notifyBackupStart(ctx)` at the START of each backup, after settings are loaded / before the restic work: in `Backup` (per-container — find where the per-container backup begins), `BackupVM`, `BackupFlash` (service.go:4028-area, after `GetSettings`), and `BackupConfig` (after staging or right after GetSettings). Grep each fn; place the call symmetric to where `notifyBackup(...)` is called at the end. Use the detached ctx already in scope (or `ctx` before it's timeout-wrapped — a start ping is quick).
- [ ] Test: `notifyBackupStart` with an `httptest` Healthchecks URL configured via `SetNotifyConfig(notify.Config{On:"failure", HealthchecksURL: srv.URL})` pings `/start`; with `On:"never"` pings nothing. (Mirror the existing notify_internal_test patterns.)
- [ ] Gates + commit: `feat(notify): ping Healthchecks /start at the beginning of each backup (#27)`.

---

### Task 3: Frontend note + i18n
**Files:** `web/src/pages/Settings.tsx`, `web/src/lib/i18n.ts` (en+de), then `web/src/lib/locales/*.ts` (24).

- [ ] In the notifications section of Settings.tsx, under the Healthchecks URL field, add a small helptext using a new key `notify.healthchecksLifecycle`. Read the section to match the existing help/hint styling.
- [ ] i18n.ts en: `notify.healthchecksLifecycle` = "Healthchecks is pinged for the whole backup lifecycle — start, success and failure — whenever a URL is set, independent of the 'notify on' setting above, so the check stays green on success even with failure-only notifications."
      de: "Healthchecks wird über den ganzen Backup-Lebenszyklus gepingt — Start, Erfolg und Fehler — sobald eine URL gesetzt ist, unabhängig von der 'Benachrichtigen bei'-Einstellung oben, damit die Prüfung auch bei nur-Fehler-Benachrichtigungen bei Erfolg grün bleibt."
- [ ] Grep to confirm no collision. Gate `tsc --noEmit` + `npm run build` (don't commit web/dist). Commit: `feat(notify): Settings note that Healthchecks pings the full lifecycle (#27)`.
- [ ] Add `notify.healthchecksLifecycle` to all 24 locale files, translated (disjoint-partition translators, no nested delegation). Verify each has it once; `tsc`; `npm run build` (commit web/dist locales). Commit: `i18n(notify): healthchecks-lifecycle key in all 24 locale files (#27)`.

## Self-review
- Spec coverage: start ping (T1 SendStart + T2 wiring), On-decoupling (T1 Send restructure), phase mapping (T1), UI note (T3). Covered.
- Types: `pingHealthchecks(...,phase string)` (T1) used by Send/SendStart/SendTest; `SendStart` (T1) used by `notifyBackupStart` (T2). Consistent.

## Handoff
Subagent-driven, sequential. After T3: `/code-review` on the branch diff, then release as **v4.4.0** (additive
+ one intentional behavior change: Healthchecks now pings success under On=failure — call it out in the notes).
