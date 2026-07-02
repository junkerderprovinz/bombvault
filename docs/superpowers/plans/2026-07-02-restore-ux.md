# Restore UX (progress + cancel + busy-feedback) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** While a restore runs, show its live percentage in the restore UI, let the user cancel it safely (cancelled ≠ failed), and never let a blocked backup look like it silently did nothing.

**Architecture:** Restore is already async and already emits `phase:"restore"` progress over the existing SSE stream, so live progress is a frontend surfacing job. Cancel is a new backend cancel-func registry keyed by progress key + a `POST /api/restore/cancel` endpoint, with `executeRestore*` mapping `context.Canceled` to a distinct `cancelled` run status (no failure notification). Busy-feedback adds a per-domain activity tracker at the `lockDomain` chokepoint so backup starters return a clear 409 instead of launching-then-blocking, plus a broader frontend "something is running" signal.

**Tech Stack:** Go (`internal/api`, `internal/store`), React/Vite/TS (`web/src`), restic 0.17.3, in-process progress pub/sub over SSE.

## Global Constraints

- Branch `feat/v4` only; never touch `main`. Do NOT push (the orchestrator gates + pushes).
- No AI attribution in commits. No em dashes in user-facing English is not required (repo English exempt), but match existing copy style.
- Go gates (every backend task): `go build ./... && go vet ./...`; `gofmt -l internal/ cmd/` empty; `go test ./... -count=1` green; `golangci-lint run ./internal/...` 0 issues. Async fire-and-detach tests MUST wait for terminal state before returning (see [[bombvault-async-test-cleanup-race]]) — never reintroduce the `t.TempDir` cleanup flake.
- Frontend gates (every frontend task): `cd web && npx tsc --noEmit` (0 errors) && `npm run build` (green); then `git checkout -- web/dist/index.html`.
- New i18n keys go into `web/src/lib/i18n.ts` en+de ONLY. The 24 locale files under `web/src/lib/locales/` are updated in one batch at close-out (Task 8) — implementers must NOT edit them.
- Runs status column is free-text TEXT (`internal/store/runs.go`); the vocabulary today is `running`/`success`/`failed`. Adding `cancelled` needs NO migration.

---

### Task 1: Per-domain activity tracker + busy-check in backup starters (C-b)

Kills the "Back up selected appears to do nothing" stall when a scheduler/maintenance op holds only `lockDomain`.

**Files:**
- Modify: `internal/api/service.go` — add `domainActivity` map + `lockDomainFor`; make backup starters refuse a busy domain.
- Modify: `internal/api/handlers.go` — the bulk/single backup handlers return 409 on the new busy error.
- Test: `internal/api/service_test.go`.

**Interfaces:**
- Consumes: `repoMu map[string]*sync.Mutex` (service.go:144), `lockDomain(domain string) func()` (service.go:213), `batchActive atomic.Bool` (service.go:157).
- Produces: `func (s *Service) domainBusy(domain string) (string, bool)` (returns the activity label + whether busy); `lockDomainFor(domain, reason string) func()`; a sentinel `errDomainBusy` is NOT reused — starters return a formatted error the handler maps to 409.

- [ ] **Step 1: Write the failing test**

```go
// internal/api/service_test.go
func TestStartBackupAllRefusesBusyDomain(t *testing.T) {
	svc := newTestService(t) // existing helper used by sibling tests
	// Simulate a scheduler/maintenance op holding the containers domain.
	unlock := svc.lockDomainFor("containers", "prune")
	defer unlock()

	started, err := svc.StartBackupAll(context.Background(), []string{"plex"})
	if err == nil || started {
		t.Fatalf("expected StartBackupAll to refuse a busy domain, got started=%v err=%v", started, err)
	}
	if got := err.Error(); !strings.Contains(got, "prune") || !strings.Contains(got, "containers") {
		t.Fatalf("busy error should name the op and domain, got %q", got)
	}
	// batchActive must be released so a later attempt can run.
	if svc.batchActive.Load() {
		t.Fatal("batchActive must be cleared after a refused start")
	}
}
```
(If `newTestService`/`StartBackupAll` signatures differ, read the existing `TestStartBackupAll*` tests and mirror their setup exactly.)

- [ ] **Step 2: Run it — expect FAIL** (`lockDomainFor` undefined / start proceeds). `go test ./internal/api/ -run TestStartBackupAllRefusesBusyDomain`.

- [ ] **Step 3: Implement the tracker**

Add to the `Service` struct (near `repoMu`, service.go ~144):
```go
	// domainActivity names the operation currently holding each domain's repoMu
	// ("backup"|"restore"|"prune"|"verify"|"replicate"|"scheduled"), so starters
	// can return a clear busy error instead of launching a goroutine that then
	// blocks silently on the mutex. Guarded by activityMu.
	activityMu     sync.Mutex
	domainActivity map[string]string
```
Initialise `domainActivity: map[string]string{}` in the constructor next to `repoMu`.

Add (near `lockDomain`, service.go ~213):
```go
// lockDomainFor is lockDomain plus an activity label recorded for the hold, so
// domainBusy can report what is running. The returned closure clears the label
// and unlocks.
func (s *Service) lockDomainFor(domain, reason string) func() {
	mu := s.repoMu[domain]
	mu.Lock()
	s.activityMu.Lock()
	s.domainActivity[domain] = reason
	s.activityMu.Unlock()
	return func() {
		s.activityMu.Lock()
		delete(s.domainActivity, domain)
		s.activityMu.Unlock()
		mu.Unlock()
	}
}

// domainBusy reports the activity label of a domain that is currently held, if any.
func (s *Service) domainBusy(domain string) (string, bool) {
	s.activityMu.Lock()
	defer s.activityMu.Unlock()
	r, ok := s.domainActivity[domain]
	return r, ok
}
```
Make `lockDomain` delegate so EVERY hold is tracked:
```go
func (s *Service) lockDomain(domain string) func() { return s.lockDomainFor(domain, "backup") }
```
Then set precise labels at the known hold sites by switching them to `lockDomainFor`: `executeRestore`/`executeRestoreVM`/flash restore → `"restore"`; `PruneDomain` → `"prune"`; `CheckDomain` → `"verify"`; `copyToOffsite`/replication → `"replicate"`; the scheduled backup path (`svc.Backup` direct) label stays `"backup"` via the default. (Grep the `lockDomain(` call sites and relabel the non-backup ones.)

In `StartBackupAll` (service.go ~1773) and `StartBackup`/VM/flash starters, AFTER the `batchActive` CAS succeeds but BEFORE launching the goroutine, add the busy-domain guard:
```go
	if op, busy := s.domainBusy("containers"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on containers", op)
	}
```
(Use the right domain per starter; the VM/flash starters guard their own domain. There is an inherent tiny race — a scheduler can grab the lock right after this check — that is acceptable UX; it shrinks the silent stall to a rare window.)

- [ ] **Step 4: Run it — expect PASS.**

- [ ] **Step 5: Map the busy error to 409 in the handler.** In `handleBackupAll` (handlers.go ~419) and `handleBackup`, when the starter returns a non-nil error that is the busy error, respond `http.StatusConflict` with `{ok:false,error:err.Error()}` (mirror the existing 409 at handlers.go:422). Keep the existing "a batch backup is already running" 409 for the `started==false && err==nil` single-flight case.

- [ ] **Step 6: Gates + commit** — `feat: per-domain activity tracker so a blocked backup returns a clear busy error`.

---

### Task 2: Cancel registry + WithCancel in restore starters + cancel endpoint (B backend, part 1)

**Files:**
- Modify: `internal/api/service.go` — `runCancels` registry; wrap each `Start*Restore` detached context with `context.WithCancel`; `CancelRun`.
- Modify: `internal/api/stacks.go` — same for `StartRestoreStack`.
- Modify: `internal/api/handlers.go` + `internal/api/api.go` — `POST /api/restore/cancel`.
- Test: `internal/api/service_test.go`.

**Interfaces:**
- Consumes: the existing detach blocks (`context.WithoutCancel` + `context.WithTimeout(bctx, restoreTimeout)`) in `StartRestore` (service.go:2334), `StartRestoreFiles` (2580), `StartRestoreToPath` (2776), `StartRestoreVM` (3782), `StartRestoreStack` (stacks.go:261); the progress key each uses in `progBegin` (e.g. `"container:"+name`, `"vm:"+name`).
- Produces: `func (s *Service) registerCancel(key string, cancel context.CancelFunc)`, `func (s *Service) unregisterCancel(key string)`, `func (s *Service) CancelRun(key string) bool`.

- [ ] **Step 1: Write the failing test**

```go
func TestCancelRunLifecycle(t *testing.T) {
	svc := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	svc.registerCancel("container:plex", cancel)
	if !svc.CancelRun("container:plex") {
		t.Fatal("CancelRun should report true for a registered key")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("CancelRun must cancel the registered context")
	}
	svc.unregisterCancel("container:plex")
	if svc.CancelRun("container:plex") {
		t.Fatal("CancelRun should report false for an unknown/finished key (idempotent no-op)")
	}
}
```

- [ ] **Step 2: Run it — expect FAIL.**

- [ ] **Step 3: Implement the registry.** Add to `Service`:
```go
	cancelMu   sync.Mutex
	runCancels map[string]context.CancelFunc
```
Init `runCancels: map[string]context.CancelFunc{}`. Add:
```go
func (s *Service) registerCancel(key string, cancel context.CancelFunc) {
	s.cancelMu.Lock(); s.runCancels[key] = cancel; s.cancelMu.Unlock()
}
func (s *Service) unregisterCancel(key string) {
	s.cancelMu.Lock(); delete(s.runCancels, key); s.cancelMu.Unlock()
}
// CancelRun cancels a running restore by its progress key; false if none is registered.
func (s *Service) CancelRun(key string) bool {
	s.cancelMu.Lock(); cancel, ok := s.runCancels[key]; s.cancelMu.Unlock()
	if ok { cancel() }
	return ok
}
```

- [ ] **Step 4: Run it — expect PASS.**

- [ ] **Step 5: Wire WithCancel into every restore starter.** In each `Start*Restore` goroutine, change the detach from
```go
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.batchActive.Store(false)
		rctx, cancel := context.WithTimeout(bctx, restoreTimeout)
		defer cancel()
		...
```
to (register the cancel under the SAME key the starter passes to `progBegin`):
```go
	bctx := context.WithoutCancel(ctx)
	key := "container:" + name // the exact progBegin key for this starter
	go func() {
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(key, cancel)
		defer s.unregisterCancel(key)
		... // pass rctx into executeRestore*
```
Do this for `StartRestore` (key `"container:"+name`), `StartRestoreFiles` (its progBegin key), `StartRestoreToPath` (its key), `StartRestoreVM` (`"vm:"+name`), `StartRestoreStack` (its per-run keys — register/unregister around each member, or a single stack key; keep it simple: register the stack's aggregate key). Read each starter to use its exact key string.

- [ ] **Step 6: Add the endpoint.** In `handlers.go`:
```go
func (h *API) handleRestoreCancel(w http.ResponseWriter, r *http.Request) {
	var body struct{ Key string `json:"key"` }
	if err := decodeJSON(r, &body); err != nil { // use the existing decode helper/pattern
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid request body"}); return
	}
	cancelled := h.svc.CancelRun(body.Key)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "cancelled": cancelled})
}
```
Register in `api.go` inside the authGate mux: `mux.HandleFunc("POST /api/restore/cancel", h.handleRestoreCancel)`.

- [ ] **Step 7: Gates + commit** — `feat: cancel registry + POST /api/restore/cancel for in-flight restores`.

---

### Task 3: cancelled ≠ failed in executeRestore* + runs status + no failure notification (B backend, part 2)

**Files:**
- Modify: `internal/api/service.go` — `executeRestore`, `executeRestoreVM`, flash restore, `StartRestoreFiles`/`ToPath` run-finish, and the stack loop: map `context.Canceled` → status `"cancelled"`, skip the failure notifier.
- Modify: `internal/api/stacks.go` — stack restore cancelled handling.
- Test: `internal/api/service_test.go`.

**Interfaces:**
- Consumes: `runsAdapter{s.store}.Finish(runID, status, snapshotID, bytes, errMsg)` (service.go ~2626); `s.store.FinishRun(id, status, ...)` (service.go ~3989); `progEnd(key, phase, ok)`; the failure notifier used on a failed restore (grep the restore-failure notify call).
- Produces: restore runs recorded with status `"cancelled"` on `context.Canceled`.

- [ ] **Step 1: Write the failing test** (fake engine returns context.Canceled)

```go
func TestRestoreCancelledRecordsCancelledNotError(t *testing.T) {
	svc, fake, notifier := newTestServiceWithFakes(t) // mirror the existing restore tests' harness
	fake.restoreErr = context.Canceled                 // fake engine RestorePath returns Canceled
	// Run the synchronous restore core directly (no goroutine) so the test is deterministic.
	err := svc.executeRestore(context.Background(), "plex", testContainerPlan(t, "plex"), false)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: %v", err)
	}
	run := lastRunFor(t, svc, "plex", "restore")
	if run.Status != "cancelled" {
		t.Fatalf("expected run status \"cancelled\", got %q", run.Status)
	}
	if notifier.failureCalls != 0 {
		t.Fatalf("a cancelled restore must not fire a failure notification, got %d", notifier.failureCalls)
	}
}
```
(Use whatever fake/notifier harness the existing restore tests use; if none isolates the notifier, assert only the run status + that `progEnd` emitted a terminal event.)

- [ ] **Step 2: Run it — expect FAIL** (status is `failed`, notifier fired).

- [ ] **Step 3: Implement.** In each restore finish path, branch on cancellation. Example for `executeRestore` (service.go ~2298) / the `runsAdapter.Finish` sites (~2626):
```go
	switch {
	case rerr == nil:
		_ = runsAdapter{s.store}.Finish(runID, "success", snapshotID, 0, "")
	case errors.Is(rerr, context.Canceled):
		_ = runsAdapter{s.store}.Finish(runID, "cancelled", "", 0, "cancelled by user")
		// no failure notification for an intentional cancel
	default:
		_ = runsAdapter{s.store}.Finish(runID, "failed", "", 0, truncateRunErr(rerr))
		s.notifyRestoreFailure(...) // the existing failure-notify call, kept ONLY on the default branch
	}
	s.progEnd(key, "restore", rerr == nil)
```
Apply the same three-way branch to `executeRestoreVM`, the flash restore finish (service.go ~3989), `StartRestoreFiles`/`ToPath`, and the stack loop (stacks.go): a cancelled member aborts the loop and the stack run records `cancelled`. Ensure `progEnd` still fires (Active:false) so the bar clears on cancel.

- [ ] **Step 4: Run it — expect PASS.** Also run the async restore tests a couple times to confirm no flake.

- [ ] **Step 5: Gates + commit** — `feat: record cancelled restores distinctly (no failure alert)`.

---

### Task 4: In-panel restore progress bar (A, frontend)

**Files:**
- Modify: `web/src/components/ProgressBar.tsx` — phase-aware caption support (optional prop).
- Modify: `web/src/components/RestorePanel.tsx` — render the restore `ProgressBar` in the `isPending` blocks.
- Modify: `web/src/pages/VMs.tsx` — restore section progress bar.
- Modify: `web/src/lib/i18n.ts` — `restore.progress` en+de.
- Test: tsc + build (no unit harness for these components).

**Interfaces:**
- Consumes: `useProgress()` (web/src/lib/progress.ts:136) → `Record<key,{phase,percent,active}>`; keys `"container:"+name` / `"vm:"+name`; existing `ProgressBar` (`percent`,`active`).
- Produces: a visible "Restoring… NN%" bar inside each restore panel.

- [ ] **Step 1:** Add i18n key `restore.progress` = "Restoring… {pct}%" (en) / "Wiederherstellen… {pct} %" (de) in `i18n.ts`.

- [ ] **Step 2:** In `RestorePanel.tsx`, in each `isPending` block (file-level ~195, `RecreateButton` ~373, `RestoreToFolder` ~436, whole-snapshot ~725), read `const prog = useProgress()["container:"+name]` and render below the existing "restore.started" text:
```tsx
{prog?.phase === "restore" && (
  <ProgressBar percent={prog.percent} active={prog.active} />
)}
{prog?.phase === "restore" && prog.percent > 0 && (
  <span className="text-xs text-carbon-textMuted">{t("restore.progress").replace("{pct}", String(Math.round(prog.percent)))}</span>
)}
```
Mirror in `VMs.tsx` restore section (~356) with key `"vm:"+name`. Follow how the container card already renders `ProgressBar` (`Containers.tsx:707`) for class/style consistency.

- [ ] **Step 3:** Give `ProgressBar` an optional `label?: string` (or read `phase`) so the card bar can caption "Restoring"/"Backing up"; wire the phase label on the container/VM cards (`Containers.tsx:707`, `VMs.tsx:675`). Keep it minimal — a small caption, no layout churn.

- [ ] **Step 4: Gate** — `cd web && npx tsc --noEmit && npm run build`, then `git checkout -- web/dist/index.html`.

- [ ] **Step 5: Commit** — `feat: show live restore progress inside the restore panel`.

---

### Task 5: Cancel button + type-differentiated confirm + cancelled watch state (B, frontend)

**Files:**
- Modify: `web/src/lib/api.ts` — `cancelRestore(key)`.
- Modify: `web/src/lib/backupWatch.ts` — `cancelled` terminal state.
- Modify: `web/src/components/RestorePanel.tsx`, `web/src/pages/VMs.tsx` — Cancel button + confirm.
- Modify: `web/src/lib/i18n.ts` — cancel keys en+de.
- Test: tsc + build.

**Interfaces:**
- Consumes: `POST /api/restore/cancel {key}` → `{ok, cancelled}`; `BackupWatchState` union (backupWatch.ts:31); the per-panel restore type (safe vs in-place) known statically by each component.
- Produces: `cancelRestore(key: string): Promise<{ok:boolean; cancelled:boolean}>`; `BackupWatchState` gains `| { phase: "cancelled" }`.

- [ ] **Step 1:** `api.ts`: `export function cancelRestore(key: string) { return fetchJSON<{ok:boolean;cancelled:boolean}>("/api/restore/cancel", {method:"POST", body: JSON.stringify({key})}); }` (match the existing fetch helper).

- [ ] **Step 2:** i18n keys en+de: `restore.cancel` ("Cancel restore"), `restore.cancelConfirmSafe` ("Cancel the restore? The partial output folder is left as-is."), `restore.cancelConfirmInPlace` ("{name} is mid-restore. Cancelling leaves it partially restored — you'll need to restore again to get a working {name}. Cancel anyway?"), `restore.cancelling` ("Cancelling…"), `restore.cancelled` ("Restore cancelled").

- [ ] **Step 3:** `backupWatch.ts`: add `| { phase: "cancelled" }` to `BackupWatchState` (line 31); when the polled run row's status is `"cancelled"`, `finish({phase:"cancelled"})`. In `finish` (line 110), treat `cancelled` like a neutral terminal (sticky, no red styling).

- [ ] **Step 4:** In each restore panel `isPending` block, render a Cancel button next to the progress bar:
```tsx
<button type="button" className="<secondary-btn classes>" onClick={async () => {
  const msg = IN_PLACE
    ? t("restore.cancelConfirmInPlace").replace(/\{name\}/g, name)
    : t("restore.cancelConfirmSafe");
  if (!window.confirm(msg)) return;
  await cancelRestore(KEY); // "container:"+name or "vm:"+name
}}>{t("restore.cancel")}</button>
```
Set `IN_PLACE=true` for `RecreateButton`, whole-snapshot in-place, VM restore, stack; `IN_PLACE=false` for `RestoreToFolder` and file-level-to-folder. Render a neutral "Restore cancelled" banner when the watch state is `cancelled`.

- [ ] **Step 5: Gate + commit** — `feat: cancel button for in-flight restores with type-aware confirmation`.

---

### Task 6: Frontend running-awareness + busy hints (C-a)

**Files:**
- Modify: `web/src/pages/Containers.tsx` — replace the `"batch:containers"`-only `batchActive` derivation (line ~936) with an any-active signal; disable "Back up selected"/restore-start when busy; show a hint.
- Modify: `web/src/pages/VMs.tsx`, `web/src/pages/Flash.tsx` — same signal.
- Modify: `web/src/lib/i18n.ts` — busy-hint keys en+de.
- Test: tsc + build.

**Interfaces:**
- Consumes: `useProgress()` → any entry with `active===true` and `phase in {backup,restore,replicate}` means something is running.
- Produces: a shared `anyRunning(progress)` helper or inline derivation; disabled buttons + a hint when a restore/backup is active.

- [ ] **Step 1:** Add a small helper in `web/src/lib/progress.ts`:
```ts
export function anyActive(map: Record<string, {phase:string; active:boolean}>): {active:boolean; phase?:string} {
  for (const k of Object.keys(map)) {
    const e = map[k];
    if (e.active && (e.phase === "backup" || e.phase === "restore" || e.phase === "replicate")) return {active:true, phase:e.phase};
  }
  return {active:false};
}
```

- [ ] **Step 2:** In `Containers.tsx`, replace the `batchActive` flag derivation (~936) so the "Back up selected" button is `disabled` whenever `anyActive(progress).active` (in addition to `bulkBusy`), and show a hint using the active phase: `restore` → `t("common.restoreRunning")`, `backup` → `t("common.backupRunning")`. Mirror on `VMs.tsx`, `Flash.tsx`, and the restore-start buttons.

- [ ] **Step 3:** i18n keys en+de: `common.restoreRunning` ("A restore is running…"), `common.backupRunning` ("A backup is running…").

- [ ] **Step 4: Gate + commit** — `feat: disable start buttons and show a hint while any backup/restore runs`.

---

### Task 7: Close-out — i18n ×24 + full re-gate + docs

**Files:**
- Modify: `web/src/lib/locales/*.ts` (24 files) — translate the new `restore.*` / `common.*` keys.
- Modify: `README.md` — one line under Restore about live progress + cancel (if warranted).
- Test: full gate suite.

- [ ] **Step 1:** Dispatch the 24-locale translation for every new key added in Tasks 4/5/6 (single batch, same process as the v4 i18n pass); do NOT translate any value that is a placeholder token.
- [ ] **Step 2:** Full re-gate: Go (build/vet/gofmt/test/golangci) + `cd web && npx tsc --noEmit && npm run build` + `git checkout -- web/dist/index.html`.
- [ ] **Step 3:** README: add "live progress + cancel for restores" to the Restore feature list if it reads naturally.
- [ ] **Step 4: Commit** — `i18n+docs: restore-UX strings in all locales + feature note`.

---

## Self-review notes

Spec coverage: A in-panel progress (Task 4) + phase label (Task 4 step 3); B cancel registry+endpoint (Task 2), cancelled≠failed (Task 3), FE cancel button + confirm + watch state (Task 5); C-a FE awareness (Task 6), C-b activity tracker + busy 409 (Task 1). Data model: `cancelled` run status (Task 3, no migration). Testing: busy-refusal, cancel lifecycle, cancelled-not-failed. Out of scope honored: no per-file/bytes/ETA, no backup cancel, no restoreTimeout change. Type consistency: progress key strings (`"container:"+name`/`"vm:"+name`) used identically in registerCancel (Task 2) and the FE cancel button (Task 5); run status `"cancelled"` used in Task 3 backend and Task 5 watch state.
