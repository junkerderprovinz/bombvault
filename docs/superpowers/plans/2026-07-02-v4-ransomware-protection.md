# v4 Ransomware Protection (Immutable Off-site) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Off-site backups that a fully compromised Unraid box cannot delete — guided append-only rest-server setup, an active tamper test that *proves* protection, real DR drills from off-site, and a dashboard scorecard that never shows green on configuration claims alone.

**Architecture:** BombVault orchestrates and verifies; the far side (rest-server `--append-only`) enforces. New per-domain immutable flags change BombVault's own behaviour (skip off-site prune, refuse off-site deletes), a raw-HTTP tamper test probes the REST endpoint's delete path, replication history + growth land in new tables, and `RunRestoreDrill` gains a `kind="dr"` sandbox-restore mode. Spec: `docs/superpowers/specs/2026-07-02-v4-ransomware-protection-design.md`.

**Tech Stack:** Go (net/http, golang.org/x/crypto/bcrypt), SQLite (modernc, forward-only migrations), restic CLI, React/Vite/TS, i18n × 26 locales.

## Global Constraints

- Branch: `feat/v4` only; never push `main`. CI builds `ghcr.io/junkerderprovinz/bombvault:v4-preview` from this branch.
- Gates for EVERY task: `go build ./... && go vet ./...`, `gofmt -l internal/` empty, `go test ./... -count=1`, `golangci-lint run ./internal/...` = 0 issues; for frontend tasks additionally `cd web && npx tsc --noEmit && npm run build`, then `git checkout -- web/dist/index.html`.
- Every new i18n key: `en` + `de` inline in `web/src/lib/i18n.ts` in the same task; the 24 files under `web/src/lib/locales/` are filled by a dedicated translation pass at the end of each UI task (the orchestrator dispatches it — implementers must NOT edit the 24 locale files).
- No AI attribution in commits. Commit per task with the message given in the task.
- SQLite: one column per `ALTER TABLE` migration (house pattern in `internal/store/migrate.go`); migrations are forward-only, appended after the last entry (`settings_restore_folder`).
- Errors shown to users are scrubbed (no host paths); restic invocations follow existing arg-builder + `--` guard patterns in `internal/restic`.
- Generic placeholder IPs only (192.168.x.x) in snippets/docs.

---

### Task 1: Store foundation — migrations + Settings threading

**Files:**
- Modify: `internal/store/migrate.go` (append migrations after `settings_restore_folder`)
- Modify: `internal/store/settings.go` (struct + GetSettings SELECT + UpdateSettings UPDATE)
- Modify: `internal/store/drills.go` (kind column threading)
- Create: `internal/store/offsite.go` (tamper_tests + offsite_runs accessors)
- Test: `internal/store/settings_test.go`, `internal/store/offsite_test.go`

**Interfaces (Produces):**
```go
// settings additions (store.Settings)
ContainersOffsiteImmutable bool
VMsOffsiteImmutable        bool
FlashOffsiteImmutable      bool
OffsiteGrowthBudgetGB      int
TamperTestSchedule         string // cadence grammar, default "weekly Sun 04:30"
DRDrillTarget              string // '' = auto

// internal/store/offsite.go
func (r *Repo) RecordTamperTest(domain string, protected bool, detail string) error
func (r *Repo) LatestTamperTest(domain string) (TamperTest, bool, error)   // TamperTest{Domain string; At int64; Protected bool; Detail string}
func (r *Repo) RecordOffsiteRun(domain string, startedAt int64) (int64, error) // returns rowid
func (r *Repo) FinishOffsiteRun(id int64, ok bool, errText string) error
func (r *Repo) LatestOffsiteRun(domain string) (OffsiteRun, bool, error)   // OffsiteRun{Domain string; StartedAt, FinishedAt int64; OK bool; Error string}

// drills: RestoreDrill gains Kind string ("subset"|"dr"); RecordRestoreDrill + LatestRestoreDrill gain a kind-aware variant:
func (r *Repo) LatestRestoreDrillKind(domain, source, kind string) (RestoreDrill, bool, error)
```

- [ ] **Step 1: Write failing store tests** — extend the settings roundtrip test with the six new fields (set → update → get, assert values + defaults `false/false/false/0/"weekly Sun 04:30"/""`); new `offsite_test.go` covering RecordTamperTest/LatestTamperTest (latest wins), RecordOffsiteRun/FinishOffsiteRun/LatestOffsiteRun, and a drill recorded with kind "dr" retrievable via LatestRestoreDrillKind while plain LatestRestoreDrill still returns the newest of any kind.
- [ ] **Step 2: Run tests, verify failure** (`go test ./internal/store/ -run 'Settings|Offsite|Drill' -count=1`).
- [ ] **Step 3: Implement** — migrations (exact SQL from the spec: three `*_offsite_immutable` INTEGER cols, `offsite_growth_budget_gb`, `tamper_test_schedule` TEXT DEFAULT 'weekly Sun 04:30', `dr_drill_target` TEXT DEFAULT '', `CREATE TABLE tamper_tests(...)`, `CREATE TABLE offsite_runs(...)`, `ALTER TABLE restore_drills ADD COLUMN kind TEXT NOT NULL DEFAULT 'subset'`), settings threading in all three spots, the offsite.go accessors, drills kind threading.
- [ ] **Step 4: Gates green, migration idempotence** — run the migration test against a fresh DB and confirm `go test ./internal/store/...` passes.
- [ ] **Step 5: Commit** — `feat(v4): store foundation for immutable off-site (flags, tamper/replication history, drill kinds)`

### Task 2: Immutable behaviour — prune-skip + off-site delete refusal

**Files:**
- Modify: `internal/api/service.go` (`copyToOffsite` at ~752 — the `ForgetPolicy` block at ~779; `DeleteSnapshot`; `Prune`; add helper)
- Modify: `internal/api/handlers.go` (`handlePutSettings` warning)
- Test: `internal/api/service_test.go`

**Interfaces (Produces):**
```go
func offsiteImmutableFor(domain string, s store.Settings) bool // pure helper, switch on domain
// PUT /api/settings response gains optional "warnings": []string
```

- [ ] **Step 1: Failing tests** — fake-engine test: with `ContainersOffsiteImmutable=true` a replication runs `Copy` but records NO `ForgetPolicy` call against the off-site repo; `DeleteSnapshot(domain, id, source="offsite")` and `Prune(domain, "offsite")` return an error containing "append-only"; `Unlock` with source offsite still works.
- [ ] **Step 2: Verify failure.**
- [ ] **Step 3: Implement** — `offsiteImmutableFor`; in `copyToOffsite` wrap the ForgetPolicy block: `if offsiteImmutableFor(domain, settings) { log.Printf("api: offsite %s: retention is enforced far-side (append-only)", domain) } else { ...existing... }`; refusal in DeleteSnapshot/Prune for offsite+immutable with error `"repo is append-only; prune far-side or use a maintenance window"`; `handlePutSettings` appends a warning string when immutable && off-site retention set (response envelope `{ok:true, warnings:[...]}`, backward compatible).
- [ ] **Step 4: Gates green.**
- [ ] **Step 5: Commit** — `feat(v4): immutable off-site flags change prune/delete behaviour`

### Task 3: Connection test endpoint

**Files:**
- Modify: `internal/api/service.go` (new `TestOffsite`), `internal/api/handlers.go` + `internal/api/api.go` (route `POST /api/offsite/{domain}/test`)
- Modify: `web/src/lib/api.ts` (`testOffsite(domain)`), `web/src/pages/Settings.tsx` ("Test connection" button per domain in the off-site card), `web/src/lib/i18n.ts` (en+de keys `offsite.test`, `offsite.testOk`, `offsite.testUninitialized`, `offsite.testFailed`)
- Test: `internal/api/service_test.go`

**Interfaces (Produces):**
```go
func (s *Service) TestOffsite(ctx context.Context, domain string) (reachable, initialized bool, err error)
// POST /api/offsite/{domain}/test -> {ok, reachable, initialized, error?}
```

- [ ] **Step 1: Failing test** — no off-site configured → error "no off-site repo configured"; configured + fake engine RepoOpens ok → reachable+initialized true.
- [ ] **Step 2: Verify failure. Step 3: Implement** (resolve via `offsiteRepoFor`+`resolveRepo`, probe with the same `cat config` path `EnsureRepo` uses; model the handler on `handleVMSSHTest`). **Step 4: Gates (incl. web).** 
- [ ] **Step 5: Commit** — `feat(v4): off-site connection test endpoint + button`

### Task 4: rest-server deployment snippet generator

**Files:**
- Create: `internal/api/deploy.go` (snippet builder), route `GET /api/offsite/{domain}/deploy-snippet` in handlers/api.go
- Test: `internal/api/deploy_test.go`

**Interfaces (Produces):**
```go
type DeploySnippet struct { User, Password, Htpasswd, DockerRun, Compose string } // Password shown once, never persisted
func buildDeploySnippet(domain string) (DeploySnippet, error) // random 24-char user-safe password, bcrypt via golang.org/x/crypto/bcrypt (cost 12), htpasswd line "bombvault-<domain>:<hash>"
// GET handler returns it as JSON; nothing stored server-side.
```
Snippets (exact content, placeholders literal): docker run: `docker run -d --name rest-server -p 8000:8000 -v /path/on/storage-box/restic:/data -e OPTIONS="--append-only --private-repos --htpasswd-file /data/.htpasswd" restic/rest-server:0.14.0` plus an `echo '<htpasswd-line>' >> /path/on/storage-box/restic/.htpasswd` pre-step and a final hint line `# repo URL for BombVault: rest:http://192.168.x.x:8000/bombvault-<domain>/<domain>`; compose equivalent with the same values.

- [ ] **Step 1: Failing test** — snippet contains `--append-only`, `--private-repos`, the htpasswd line verifies against the returned password with bcrypt, no real IPs. **Step 2-4: implement, gates.** 
- [ ] **Step 5: Commit** — `feat(v4): rest-server deployment snippet generator`

### Task 5: Tamper test (stage 1) — probe, history, notification, schedule

**Files:**
- Create: `internal/api/tamper.go`
- Modify: `internal/api/handlers.go` + `api.go` (route `POST /api/offsite/{domain}/tamper-test`), `cmd/bombvault/main.go` + `internal/schedule` wiring (tamper job on `TamperTestSchedule`, running for every immutable-flagged domain)
- Test: `internal/api/tamper_test.go` (httptest servers simulating both behaviours)

**Interfaces (Produces):**
```go
type TamperVerdict struct { Testable bool; Protected bool; Detail string }
func (s *Service) RunTamperTest(ctx context.Context, domain string) (TamperVerdict, error)
```
Logic: repo must be a `rest:` URL (else `Testable=false`, Detail "only REST repos are verifiable"); issue authenticated `DELETE <base>/data/<64-hex-random>` and `DELETE <base>/snapshots/<8-hex-random>` with `net/http` + Basic auth from `CloudCreds.RESTUser/RESTPassword`; per response: 403/405 → protected; 404 → NOT protected; 2xx → NOT protected + Detail "server accepted a delete"; both probes must be protected for `Protected=true`. Persist via `RecordTamperTest`; when the previous LatestTamperTest was protected and the new one is not → `notify.Send` (reuse the drill-failure notification path) with subject "Off-site protection LOST". Conservative: any transport error → `Testable=true, Protected=false, Detail=scrubbed error` is WRONG — instead return err (test inconclusive ≠ unprotected); only real HTTP verdicts decide.

- [ ] **Step 1: Failing tests** — httptest server returning 403 → protected; returning 404 → unprotected + recorded + notification fired on flip; 200 → unprotected; non-rest repo → Testable=false; network error → error, nothing recorded.
- [ ] **Step 2-4: implement, wire schedule (cadence parse via existing grammar; job iterates domains with immutable flag), gates.**
- [ ] **Step 5: Commit** — `feat(v4): active tamper test with history, protection-loss alerts and schedule`

### Task 6: Off-site setup wizard UI

**Files:**
- Create: `web/src/components/OffsiteWizard.tsx`
- Modify: `web/src/pages/Settings.tsx` (per-domain "Set up…" button in the off-site card, immutable toggle, retention-strategy chooser), `web/src/lib/api.ts` (deploySnippet, tamperTest calls + types), `web/src/lib/i18n.ts` (en+de)

Flow (from the spec): backend choice (rest-server recommended / rclone / S3) → rest-server: snippet display with copy buttons + one-time password warning → URL+credentials + Test connection → Immutable toggle triggers immediate tamper test, verdict shown verbatim ("✓ delete refused — append-only active" / "✗ server ACCEPTED the delete — NOT protected"); toggling immutable on an `rclone:`-backed repo shows the known-bug warning; S3 shows the "configured, not verified — I confirm versioning+deny-delete manually" checkbox (stored client-side into the notes? NO — store as part of the immutable flag only when checked; scorecard shows "manually confirmed" state from a new settings bool? Keep simple: S3 + immutable toggle allowed, tamper endpoint returns Testable=false and the scorecard shows "unverified"). Retention chooser: three radio strategies (far-side cron → snippet with `--keep-within 14d` included; maintenance window → explanation; grow+budget → budget input wired to `offsiteGrowthBudgetGB`).

- [ ] **Steps: build the wizard from existing primitives (Card, copy-block pattern from the VM-SSH card), wire calls, tsc+build gates, commit** — `feat(v4): guided off-site setup wizard with immutable verification`

### Task 7: Replication history + growth sampling + budget alarm

**Files:**
- Modify: `internal/api/service.go` (`copyToOffsite` records `RecordOffsiteRun`/`FinishOffsiteRun`; after success `s.CollectStatsAsync(domain, "offsite")`; budget check in the stats sampler comparing latest offsite repo_stats size vs `OffsiteGrowthBudgetGB`, breach → notify once per flip (persist last-breach state in memory is fine))
- Test: `internal/api/service_test.go`

- [ ] **Steps: failing tests (replication writes an offsite_run with ok/error; failed copy records error text; stats collect called), implement, gates, commit** — `feat(v4): persisted replication history + off-site growth budget alarm`

### Task 8: Scorecard backend — DomainStatusEntry + metrics

**Files:**
- Modify: `internal/api/service.go` (`DomainStatusEntry` at ~402 + the builder at ~445-500), `internal/api/metrics.go`
- Test: `internal/api/service_test.go`, `internal/api/metrics_test.go`

**Interfaces (Produces):** DomainStatusEntry gains `OffsiteConfigured bool`, `OffsiteImmutable bool`, `LastTamperAt int64`, `LastTamperOK bool`, `LastReplicationAt int64`, `LastReplicationOK bool`, `LastDRDrillAt int64`, `LastDRDrillOK bool`, `Protection string` ("red"|"amber"|"green") — JSON camelCase. Aggregate rule from the spec (red: no off-site OR tamper failed OR tamper older than 2× its schedule period; amber: replication/drill overdue by the rpoStatus doubling rule; else green). Gauges: `bombvault_offsite_immutable{domain}`, `bombvault_tamper_test_ok{domain}`, `bombvault_offsite_last_replication_timestamp_seconds{domain}`.

- [ ] **Steps: failing tests (status JSON carries the fields; metrics lines present + escaped), implement, gates, commit** — `feat(v4): ransomware-protection scorecard data + metrics`

### Task 9: Dashboard "Ransomware protection" card

**Files:**
- Modify: `web/src/pages/Dashboard.tsx` (new `RansomwareCard` beside ProtectionCard; growth graph in Advanced via existing repo-stats endpoint), `web/src/lib/api.ts` (DomainStatus type ext.), `web/src/lib/i18n.ts` (en+de)

Card: per domain a checklist (configured / append-only verified + age via `relativeTime` / replication current / off-site drill + age / encryption on / prune strategy set) with the aggregate chip; every red row is a `<Link>` into Settings. Uses only `GET /api/status` (extended) — no new round-trips.

- [ ] **Steps: build, tsc+build gates, commit** — `feat(v4): ransomware-protection dashboard card`

### Task 10: DR-drill v2 backend

**Files:**
- Modify: `internal/api/service.go` (`RunRestoreDrill` at ~3594 gains `kind`; new `runDRDrill` doing: domain lock (tryLock, busy error) → off-site repo → pick target (settings `DRDrillTarget` or most recently backed-up container via `LastSuccessfulBackup`-style query; flash = whole snapshot; VMs = refuse kind dr with clear error) → newest snapshot → restore via `RestoreInclude` into `<RestoreFolder>/bombvault-drill-<domain>-<unix>` resolved by `paths.Resolve` + write marker file `.bombvault-drill` → verify: `restic ls` count + `stats --mode restore-size` bytes vs on-disk walk → cleanup only when marker present → record `restore_drills(kind='dr', source)`; failure → `notifyDrillFailure`)
- Modify: `internal/restic` if `stats --mode restore-size` isn't wrapped yet (add `StatsRestoreSize(ctx, repo, snapshotID, mode) (files int, bytes int64, err error)` with arg test)
- Modify: handlers (`POST /api/verify/{domain}` accepts `?kind=dr|subset`, default subset), scheduler wiring (drill job: per configured domain × source × kind — extend the drills settings usage; scheduled off-site dr for containers+flash when drills enabled and off-site configured)
- Test: `internal/api/service_test.go` (fake engine: dr drill happy path creates+cleans sandbox, marker guard refuses cleanup of unmarked dirs, VM dr refused, failure notifies + records kind dr)

- [ ] **Steps: failing tests first, implement, gates, commit** — `feat(v4): real off-site DR drills (sandbox restore + verification)`

### Task 11: DR-drill UI + drill-target picker + badge

**Files:**
- Modify: `web/src/pages/Settings.tsx` (Verify card: kind toggle "Integrity check | Real restore (off-site)", drill-target dropdown fed by `listContainers`, saved via settings `drDrillTarget`), `web/src/pages/Dashboard.tsx` (badge strings: "proven restorable from off-site" uses LastDRDrill fields), `web/src/lib/api.ts`, `web/src/lib/i18n.ts` (en+de)

- [ ] **Steps: build, gates, commit** — `feat(v4): DR-drill controls + off-site restorability badge`

### Task 12: Wave close-out — i18n parity, adversarial review, docs

- [ ] Orchestrator dispatches the 24-locale translation pass for ALL keys added in Tasks 3/6/9/11 (single batch), then `npm run build` gate + commit `i18n: v4 ransomware-protection keys in all 24 locale files`.
- [ ] Adversarial review workflow over `git diff main...feat/v4` (concurrency, tamper-verdict semantics, drill cleanup safety, scorecard truthfulness); fix confirmed findings; gates; commit.
- [ ] README: add the ransomware-protection feature bullet(s) under Features (marked as v4-preview until release). Commit `docs: ransomware protection feature docs`.
- [ ] Box-gate checklist appended to the plan file for Bottich (wizard against real rest-server, tamper verdict both modes, DR drill green from off-site, protection-loss notification end-to-end) — executed by the owner with the `v4-preview` image.

## Self-review notes

Spec coverage: flags/behaviour (T1-2), connection test (T3), snippet (T4), tamper stage 1 + schedule + alerts (T5), wizard + rclone warning + S3 unverified (T6), replication history + growth/budget (T7), scorecard + gauges (T8-9), DR drill + scheduling + target + badge (T10-11), i18n/review/docs (T12). Wave-5 items (canary, SigV4, retention snippet automation beyond the wizard text) intentionally deferred per spec.

## Box-gate checklist (Bottich, `v4-preview` image)

Run by the owner on the real box before merging feat/v4 to main and tagging the release. Pull the
`v4-preview` image (feat/v4 build); production stays on `:latest` (v3.x) throughout.

- [ ] **Deploy a real append-only far side** — use the wizard's generated `restic/rest-server --append-only`
  snippet on the box, then point a domain's off-site repo at it (`rest:http://…`).
- [ ] **Wizard end-to-end** — backend choice → copy the deploy snippet → connection test shows
  reachable/uninitialized correctly → save URL + REST credentials → immutable toggle ON runs the tamper
  test immediately and shows `✓ delete refused — append-only active`.
- [ ] **Tamper verdict, both modes** — against the append-only server → protected (green). Point at a
  server with `--append-only` OFF → `✗ server ACCEPTED the delete — NOT protected` (red). Stop the server →
  inconclusive, and confirm the stored verdict does NOT flip.
- [ ] **Replication currency uses last SUCCESS** — replicate a domain; scorecard "replication current" is
  green. Break the far side and let a scheduled replication FAIL → a failure notification fires AND
  "replication current" goes stale/amber (must NOT stay green on a failing replication).
- [ ] **DR drill from off-site** — Settings → Verify → "Real restore (off-site)" for a container → restores
  into a sandbox, verifies, cleans up (no leftover `bombvault-drill-*` under the restore folder), Dashboard
  shows "proven restorable from off-site". Confirm VM DR is refused with a clear message.
- [ ] **Protection-loss notification** — from a verified-green immutable off-site, flip the far side OFF
  append-only and run the tamper test → exactly one "protection LOST" notification, scorecard goes red.
- [ ] **Growth budget** — set a small off-site growth budget on the immutable repo, replicate past it →
  the budget alarm fires once.
- [ ] **Default-mode reachability** — Advanced OFF: off-site / retention / DR cards + the ransomware
  Dashboard card are visible; red rows deep-link to the off-site settings; the destructive Prune button is
  hidden (Advanced-only).
- [ ] **rclone path guard** — entering an off-site path without a scheme (e.g. `BackBlaze:bucket`) is
  rejected with guidance to use `rclone:BackBlaze:bucket`; `rclone:<remote>:<bucket>` saves and replicates.
- [ ] **i18n** — switch UI to German → new ransomware/off-site/drill strings are translated; the three
  tamper-verdict strings stay English verbatim (by design).
- [ ] **Regression** — normal local backup + restore still work; the `:latest` (v3.x) production container
  is unaffected.
