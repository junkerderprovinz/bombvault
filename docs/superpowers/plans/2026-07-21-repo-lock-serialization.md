# Repo Lock Contention Fix — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Stop restic "repository is already locked by PID N" failures (#94/#96 retention, #92/#97 verify) by preventing concurrent live restic operations on one repo, waiting out transient locks, reaping hung restic processes, and removing the unsafe automatic force-unlock.

**Architecture:** Four independent, additive changes. Root cause (reproduced with restic 0.17.3): the failing lock is held by a LIVE restic process, not a dead orphan, so the current `restic unlock --remove-all` self-heal cannot fix it (and is unsafe — it yanks a running op's lock). Fix by (A1) making read-only ops lock-free and serializing the one write bypass, (A2) letting genuine lock-takers wait out a transient lock, (B) actually killing hung restic children, (C) deleting the force-unlock heals.

**Tech Stack:** Go, restic 0.17.3 CLI (argv builders in `internal/restic/restic.go`, unit-tested in `internal/restic/restic_args_test.go`), service layer `internal/api/service.go`.

## Global Constraints

- 3-digit SemVer. NEVER tag/release without explicit approval.
- Before every push run: `go build ./...`, `go vet ./...`, `gofmt -l .` (must be empty), `golangci-lint run ./...`, `go test ./...` (needs restic ≥0.17 on PATH). `just check` runs the Go chain.
- restic arg builders are unit-tested in `internal/restic/restic_args_test.go` — every argv change updates the matching test.
- Do NOT reintroduce `--group-by paths` (issue #91). VM live snapshots carry a `live` tag; never `--group-by tags` globally.
- Public repo; no real user data / IPs. Repo prose is English.
- `--no-lock` and `--retry-lock <dur>` are restic GLOBAL flags: they go BEFORE the subcommand (right after `-r <repo>`), same placement as the existing `limitFlags`.

---

### Task 1: `--no-lock` on read-only stats + diff builders (A1a)

Read-only ops must not take a repo lock (they can otherwise block an exclusive `forget --prune`). `SnapshotsArgs` already does this; extend the same treatment to `stats` and `diff`, which `CollectStats`/`DiffSnapshots` run from background goroutines and which the reproduction showed holding the shared lock that fails retention.

**Files:**
- Modify: `internal/restic/restic.go` — `StatsArgs`, `StatsRestoreSizeArgs`, `DiffArgs`
- Test: `internal/restic/restic_args_test.go`

**Interfaces:** No signature changes. Each builder appends `--no-lock` (before `--json`), mirroring `SnapshotsArgs`.

- [ ] **Step 1: Update the arg tests** to expect `--no-lock` in the stats, stats-restore-size, and diff argv (add the flag to the expected slices; keep order consistent with the builder).
- [ ] **Step 2: Run the tests, verify they FAIL** (`go test ./internal/restic/ -run Args`).
- [ ] **Step 3: Add `--no-lock`** to `StatsArgs` (after `--json --mode <mode>` — order just must match the test), `StatsRestoreSizeArgs`, and `DiffArgs`. Update each doc comment to state the op is read-only and takes `--no-lock` so it can never block a concurrent backup/forget (cite the same rationale as `SnapshotsArgs`).
- [ ] **Step 4: Run the tests, verify they PASS.**
- [ ] **Step 5: `go build ./... && gofmt -l . && go vet ./...`; commit** (`fix(restic): stats/diff run --no-lock so reads never block retention (#94/#96)`).

### Task 2: `--retry-lock` on the lock-taking builders (A2)

A transient lock held by another process (a second BombVault instance, a lingering old container after an update, or the brief window between two same-repo ops) currently fails immediately — restic 0.17.3 defaults `--retry-lock` to 0 (verified in the reproduction). Make genuine lock-takers WAIT a bounded time instead of failing.

**Files:**
- Modify: `internal/restic/restic.go` — add a `retryLockFlags()` helper + a package const; apply in `BackupArgs`, `ForgetArgs`, `ForgetPolicyArgs`, `CheckArgs`, `CheckDataArgs`, `CopyArgs`, `PruneArgs`
- Test: `internal/restic/restic_args_test.go`

**Interfaces:**
- Add `const resticRetryLock = "5m"` and `func retryLockFlags() []string { return []string{"--retry-lock", resticRetryLock} }`.
- Placement: immediately after `repoFlag(repo)` (before the subcommand), like `limitFlags`. For `CopyArgs` it goes with the other global flags before `copy`.
- Do NOT add it to `SnapshotsArgs`/`StatsArgs`/`StatsRestoreSizeArgs`/`DiffArgs` (now `--no-lock`), nor `InitArgs`/`CatConfigArgs`/`LsArgs`/restore/dump.

- [ ] **Step 1: Update the arg tests** for backup, forget, forget-policy, check, check-data, copy, prune to expect `--retry-lock 5m` right after `-r <repo>` (before the subcommand; for copy, before `copy` alongside the limit flags).
- [ ] **Step 2: Run tests, verify FAIL.**
- [ ] **Step 3: Add the const + `retryLockFlags()`; insert it** in the seven builders at the stated position. Document that it lets a lock-taking op wait out a transient/cross-process lock instead of failing (the in-process domain mutex already prevents same-process overlap).
- [ ] **Step 4: Run tests, verify PASS.**
- [ ] **Step 5: `go build/vet/gofmt`; commit** (`fix(restic): --retry-lock 5m so lock-takers wait out a transient lock (#94/#96)`).

### Task 3: Serialize `TagSnapshot` under the domain lock (A1b)

`TagSnapshot` runs `restic tag` (an exclusive-lock WRITE) but is the one write path that does NOT hold the per-domain `repoMu`, so it can collide with a live backup/prune on the same repo. Wrap it like the other maintenance ops.

**Files:**
- Modify: `internal/api/service.go` — `TagSnapshot` (currently ~4182-4227)
- Test: `internal/api/*_test.go` (add/extend a service test if a fake engine harness exists)

**Interfaces:** `TagSnapshot(ctx, name, source, snapID string, addTags []string) error` unchanged. It operates on the `containers` domain.

- [ ] **Step 1:** In `TagSnapshot`, after resolving `repo` and before `unlockStale`/`TagAdd`, acquire the domain lock non-blockingly: `unlock, ok := s.tryLockDomainFor("containers", "tag"); if !ok { return errDomainBusy }; defer unlock()`. Update the comment (drop the "sole writer, lock always stale" claim; state it now serializes against backup/prune via the domain lock).
- [ ] **Step 2:** If a service test harness with a fake `ResticEngine` exists, add a test asserting `TagSnapshot` returns `errDomainBusy` while the `containers` domain lock is held (acquire it via `lockDomain("containers")` in the test). If no such harness exists, note that in the commit and rely on the build + existing tag tests.
- [ ] **Step 3: `go build/vet/gofmt`; `go test ./internal/api/`; commit** (`fix(api): serialize TagSnapshot under the domain lock (#96)`).

### Task 4: Remove the unsafe automatic force-unlock heals (C)

`unlock --remove-all` force-removes even a LIVE lock — the reproduction shows it deletes the file while the holder stays alive and re-locks, and it is unsafe (removes a running op's protection). With Tasks 1-3 preventing same-repo collisions and Task 2 waiting out transient ones, the automatic force-unlock is both ineffective and dangerous. Remove it from the two heals; keep the plain `unlockStale` pre-op (which correctly clears a GENUINE stale orphan: dead PID on this host) and keep the manual Unlock endpoint (operator-initiated force is their choice).

**Files:**
- Modify: `internal/api/service.go` — `forgetWithLockHeal` (~793-803), `CheckDomain` verify heal (~6592-6601)

**Interfaces:** `forgetWithLockHeal(ctx, repo, p, mode, tag, prune) error` unchanged; it just stops force-unlocking.

- [ ] **Step 1: `forgetWithLockHeal`** — replace the body with: `s.unlockStale(ctx, repo, mode)` (clear a genuine stale orphan) then `return s.engine.ForgetPolicy(ctx, repo, p, mode, tag, prune)`. Delete the `isLockErr` → `Unlock(true)` → retry block. Rewrite the doc comment: reads are `--no-lock`, writes are serialized + `--retry-lock`, hung restic is reaped (Task 5); a lingering lock is a genuine orphan cleared by `unlockStale`, never force-removed while a process may be live.
- [ ] **Step 2: `CheckDomain`** — delete lines that do `if isLockErr(err) { ... Unlock(true) ... retry }` (~6594-6600). Keep the `unlockStale` + single `Check`. Update the comment to match Step 1's reasoning (the domain lock is still held for the whole verify).
- [ ] **Step 3:** If `isLockErr` becomes unused after this, leave it (it may be used by `listSnapshots` heal) — verify with `go vet`/golangci-lint (unused-function lint). Remove it only if the linter flags it.
- [ ] **Step 4: `go build/vet/gofmt/golangci-lint`; `go test ./...`; commit** (`fix(api): drop unsafe automatic force-unlock; it cannot clear a live lock (#92/#94)`).

### Task 5: Reap hung restic child processes (B)

A restic wedged on a dead mount / stalled rclone keeps refreshing its lock; `exec.CommandContext` only SIGKILLs the direct child (not the rclone grandchild) and `cmd.Wait` can block indefinitely, so the lock lives until a container restart (#97, "manual unlock did not help"). Put each restic in its own process group, kill the whole group on cancel, and bound `cmd.Wait` with `WaitDelay`.

**Files:**
- Modify: `internal/restic/restic.go` — apply to the `exec.Cmd` in `run()`, `DumpZip`, `Copy`
- Create: `internal/restic/proc_unix.go` (`//go:build !windows`) + `internal/restic/proc_windows.go` (`//go:build windows`)
- Test: `internal/restic/proc_test.go` (best-effort)

**Interfaces:**
- `func configureProcGroup(cmd *exec.Cmd)` — unix: sets `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` and `cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }` (kills the group incl. rclone); windows: no-op. Both set `cmd.WaitDelay = 10 * time.Second`.
- Call `configureProcGroup(cmd)` right after each `exec.CommandContext(...)` and before setting `cmd.Env`.

- [ ] **Step 1:** Create `proc_unix.go` and `proc_windows.go` with `configureProcGroup`. Unix sets Setpgid + the group-kill Cancel + `WaitDelay`; Windows sets only `WaitDelay` (leave default cancel). Document the honest limit in a comment: uninterruptible I/O on a truly dead mount cannot be reaped even by SIGKILL; this handles the common hangs and the rclone grandchild, not a wedged NFS mount (which needs separate mount-health detection).
- [ ] **Step 2:** Wire `configureProcGroup(cmd)` into `run()`, `DumpZip`, `Copy`.
- [ ] **Step 3:** Add a best-effort `proc_test.go` (unix build tag) that starts a long child via the same mechanism, cancels ctx, and asserts it returns within a few seconds. If reliable cross-platform testing is impractical, keep the test unix-only and note it.
- [ ] **Step 4: `go build ./...` (confirm BOTH `GOOS=windows` and `GOOS=linux` compile — `GOOS=windows go build ./internal/restic/` and `GOOS=linux go build ./internal/restic/`); `gofmt/vet`; `go test ./internal/restic/`; commit** (`fix(restic): process-group kill + WaitDelay so hung restic is reaped (#92/#97)`).

### Task 6: Doc fix — graceful sshd reload in the go-file snippet

The VM-backup SSH doc tells users to run `/etc/rc.d/rc.sshd restart` at boot, which hard-kills the SSH listener and prints a boot warning. Use a graceful SIGHUP reload instead.

**Files:**
- Modify: `docs/vm-backup-ssh-setup.md` (the `/boot/config/go` snippet, ~line 131)

- [ ] **Step 1:** Replace `/etc/rc.d/rc.sshd restart` with `pidof sshd >/dev/null && killall -HUP sshd` (reload config without dropping the listener; only if sshd is already up — otherwise it reads the edited config on start). Add a one-line note that this avoids the "killing listener / Restarting SSH server daemon" boot warning.
- [ ] **Step 2: Commit** (`docs: graceful sshd reload in go-file snippet (no boot warning)`).

## Self-Review

- Spec coverage: A1 = Tasks 1 (stats/diff no-lock) + 3 (tag serialize); A2 = Task 2 (retry-lock); B = Task 5 (pgid kill); C = Task 4 (force-unlock removed); doc = Task 6. All four parts + the doc covered.
- The mutex stays DOMAIN-keyed (not re-keyed to repo-path): for the default config domain==repo, and A2's `--retry-lock` covers the rare cross-domain-same-repo / cross-process case, avoiding a risky lock refactor.
- No `--group-by paths` reintroduced; no arg builder loses its `--` injection guard; every argv change has a matching test update.
- After the build, re-run the local restic 0.17.3 reproduction with `stats --no-lock` running concurrently with `forget --prune` to confirm the collision is gone (verification, not a code task).
