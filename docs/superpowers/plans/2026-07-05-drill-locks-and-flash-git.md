# Drill scheduling/lock fixes (#30) + flash .git exclude (#31) — plan

> REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use `- [ ]`.

Two GitHub bugs. **#30** (manilx): the SCHEDULED restore drill fails / records nothing while a MANUAL one
succeeds, and the dashboard doesn't reflect it. Two independent defects (A: drill locking; B: dashboard read
keys). **#31** (BaukeZwart): the flash backup includes a `.git` folder Unraid's doesn't.

## Global Constraints
- Branch `fix/drill-locks-and-flash-git` (off `main`, which is 2 UI-tweak commits past v4.5.0). Sequential
  implementers.
- Go gates: `go build ./... && go vet ./...`, `gofmt -l internal/ cmd/` empty, `go test ./... -count=1`,
  `golangci-lint run ./internal/...` 0. Frontend: `tsc --noEmit` + `npm run build`.
- No AI attribution (controller commits).

---

### Task 1 (#30-A1 + A2): drill locking — scheduled blocks, manual doesn't; self-heal stale locks
**Files:** `internal/api/service.go` (`RunRestoreDrill`, `runSubsetDrill`, `runDRDrill`),
`internal/api/handlers.go` (`handleRunDrill`), `cmd/bombvault/main.go` (scheduler drillFn wiring),
`internal/api/service_test.go`.

Root cause (recon): `runSubsetDrill`/`runDRDrill` take the shared per-domain mutex NON-blocking
(`tryLockDomainFor(domain,"verify")`) and return `errDomainBusy` **recording nothing** on contention. The
scheduled off-site replication/backup hold the same mutex blocking, so a nightly co-fire makes the scheduled
drill silently vanish → dashboard "never". Also, unlike `CheckDomain` (v4.4.1 #29 fix at ~:4767), the drill
does NOT `unlockStale` before its `CheckData`/`RestoreInclude` restic step.

- [ ] **A1:** add a `wait bool` param to `RunRestoreDrill(ctx, domain, source, kind string, wait bool)` and
  thread it into `runSubsetDrill`/`runDRDrill`. In each drill fn, replace the lock acquisition:
  ```go
  var unlock func()
  if wait {
      unlock = s.lockDomainFor(domain, "verify") // scheduled: wait for the domain, always record a result
  } else {
      u, ok := s.tryLockDomainFor(domain, "verify") // manual: immediate busy feedback
      if !ok {
          return store.RestoreDrill{}, errDomainBusy
      }
      unlock = u
  }
  defer unlock()
  ```
  (Match the real `lockDomainFor`/`tryLockDomainFor` signatures — read them; `lockDomainFor` returns an
  unlock func, `tryLockDomainFor` returns `(func(), bool)`.)
- [ ] **A1 wiring:** `cmd/bombvault/main.go` scheduler drillFn (~:165-166) passes `wait=true`;
  `handlers.go` `handleRunDrill` (~:1189) passes `wait=false`. Grep every `RunRestoreDrill(` caller and update.
- [ ] **A2:** in `runSubsetDrill`, add `s.unlockStale(ctx, repo, mode)` immediately before
  `s.engine.CheckData(...)` (~:4850). In `runDRDrill`, add `s.unlockStale(ctx, repo, mode)` before
  `s.engine.RestoreInclude(...)` (~:5115), using the DR off-site `repo` + `mode` already resolved in the fn.
  Mirror `CheckDomain`'s `s.unlockStale(ctx, repo, mode)` (service.go:4767).
- [ ] Tests: `runSubsetDrill`/`runDRDrill` with `wait=false` on a held domain lock → `errDomainBusy`, no row
  written; with `wait=true` → blocks until the lock frees then records (a goroutine holds the lock briefly, the
  drill records after release). Assert `unlockStale` (Unlock) is called before `CheckData`/`RestoreInclude`
  (fake engine call-order, like the #29 tag test). Reuse the existing drill test harness (grep for existing
  `RunRestoreDrill`/drill tests).
- [ ] Gates + commit: `fix(drill): scheduled drill waits for the domain + self-heals stale locks (#30)`.

---

### Task 2 (#30-B): scorecard honors ok; refetch status after a manual drill
**Files:** `internal/api/service.go` (drill scorecard state ~:887-921), `web/src/pages/Settings.tsx` (manual
drill action), maybe `web/src/pages/Dashboard.tsx`.

Root cause (recon): the scorecard "restore drill" row derives `drillState` from **currency only, ignoring
`ok`** (`service.go:887-889`, `913-921` via `rpoStatus(lastDRDrillAt,…)`), so a recorded FAILED DR drill reads
green while Pill #2 (`lastDrDrillOK`) reads red — a contradiction. And after a manual drill the Settings card
only updates its own local state (`Settings.tsx:955`), never refetching `/api/status`, so the dashboard's
pills stay stale.

- [ ] **Honor ok:** read the scorecard `drillState` derivation (`service.go` ~:887-921). Make it reflect the
  latest DR drill's `ok`: when a DR drill exists and `lastDrDrillOK == false`, the row state is red/bad (not
  green-by-currency). Keep "never" when `lastDRDrillAt == 0`. (Read `rpoStatus` + `protectionChecks`; add an
  `ok`-aware branch so a failed drill can't read green.)
- [ ] **Refetch after manual drill:** in `Settings.tsx`, after a manual drill completes, trigger a refetch of
  the shared status (`/api/status`) so the dashboard pills update (mirror how other actions refresh; if there's
  a shared status hook/query, invalidate it — grep for the status fetch). At minimum refetch the drill/status
  the dashboard reads.
- [ ] (If cheap) clarify the two drills in the UI copy so a user knows a local **subset** verify updates
  "Verified restorable" while the **off-site DR** drill updates "proven restorable from off-site" / the
  scorecard row — so running a subset check doesn't look like it "did nothing" to the DR row. (i18n if a new
  string is added; en+de + note for the 24-locale pass. Skip if it balloons scope — the honor-ok + refetch are
  the required fixes.)
- [ ] Gates (Go + `tsc`/`build`) + commit: `fix(drill): scorecard honors drill outcome + refetch status after a manual drill (#30)`.

---

### Task 3 (#31): exclude .git from the flash backup
**Files:** `internal/restic/restic.go` (`BackupArgs`, `Backup`), `internal/api/service.go` (`ResticEngine`
interface, `resticAdapter.Backup`), `internal/backup/flash_orchestrator.go` (`FlashRestic` + `BackupFlash`
call), `internal/backup/flash_orchestrator_test.go`.

Root cause (recon): the flash backup is a wholesale `/boot` backup with NO excludes; the user's `/boot/.git`
(user/plugin-created) flows into the snapshot + every zip. Unraid's flash backup excludes it. (Note: a stray
`.git` does NOT break the USB creator — it's parity + smaller backups.) The `Backup`/`BackupArgs` chain has no
exclude support.

- [ ] Make `excludes` **variadic** so container/VM/config callers (sharing the same adapter) compile unchanged:
  - `restic.go:185` `BackupArgs(repo string, paths, tags []string, m Mode, excludes ...string)` — after the
    `--tag` loop, before `--`: `for _, ex := range excludes { args = append(args, "--exclude", ex) }`.
  - `restic.go:783` `Restic.Backup(ctx, repo string, paths, tags []string, m Mode, excludes ...string)` →
    forward `excludes...` to `BackupArgs`.
  - `service.go:66` `ResticEngine.Backup(ctx, repo string, paths, tags []string, mode restic.Mode, excludes ...string)`.
  - `service.go:3468` `resticAdapter.Backup(ctx, repo string, paths, tags []string, excludes ...string)` →
    `a.engine.Backup(ctx, repo, paths, tags, a.mode, excludes...)`.
  - `flash_orchestrator.go:14` `FlashRestic.Backup(..., excludes ...string)`; `:36` pass `".git"` at the flash
    call: `d.Restic.Backup(ctx, d.Repo, []string{d.SourceDir}, []string{"flash"}, ".git")`. Container/VM/config
    orchestrators pass nothing → unchanged.
  (restic matches a bare `.git` by basename at any depth; that's the intended drop of `/boot/.git`.)
- [ ] Update `flash_orchestrator_test.go:18` `fakeFlashRestic.Backup` to the new signature + assert `.git` is
  passed as an exclude. Confirm container/VM/config orchestrator tests still compile (variadic → no change).
- [ ] Gates + commit: `fix(flash): exclude .git from the flash backup to match Unraid (#31)`.

---

## Self-review
- #30-A1 (block scheduled / non-block manual) → Task 1; #30-A2 (unlockStale) → Task 1; #30-B (honor ok +
  refetch) → Task 2; #31 (.git exclude) → Task 3. Covered.
- Types: `RunRestoreDrill(..., wait bool)` (T1) used by main.go + handlers.go; `Backup(..., excludes ...string)`
  (T3) variadic so existing callers unaffected.

## Handoff
Subagent-driven, sequential. After T3: `/code-review` on the diff, then release **v4.5.1** (bug-fix patch,
also carries the two main UI tweaks since v4.5.0) + answer/close #30 + #31.
