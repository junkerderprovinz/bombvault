# Module Map: `internal/backup`

**Analysis Date:** 2026-09-09

Package `backup` is the per-domain backup/restore orchestration core of BombVault. It coordinates five backup domains — containers (Docker), VMs (libvirt/qemu), the Unraid flash drive, user-defined file sets, and BombVault's own `/config` — around the restic storage engine. It is the **security-critical core**: it performs every destructive operation (container stop/remove, VM destroy/undefine, appdata overwrite) and therefore owns the guard chain (confirmation, snapshot-id validation, path-safety validation, wrong-target re-inspect, pre-flight verification).

## Technology

**Language / runtime:**
- Go 1.25 (`go.mod`: `go 1.25.0`, module `github.com/junkerderprovinz/bombvault`).

**Dependencies imported by this package — stdlib only, plus two internal packages:**
- Stdlib: `context`, `errors`, `fmt`, `log`, `regexp`, `strings`, `time` (`internal/backup/orchestrator.go`); plus `io`, `os`, `path`, `path/filepath` (`internal/backup/vm_orchestrator.go`).
- `internal/compose` — `compose.StartOrder` and `compose.DepGraph`, reused for the topological (depends_on) ordering of the restart-after-backup phase (`internal/backup/orchestrator.go:553-554`).
- `internal/model` — data shapes only (`model.Inspect`, `model.Health`, `model.Allocation`, `model.Config`, `model.HostConfig`); the rich container profile travels through the DI seam so security-relevant fields reach the real adapter on recreate.
- **Zero third-party imports.** This is a hard, documented rule: the package doc comment (`internal/backup/orchestrator.go:1-16`) states the package "imports ONLY the interfaces defined here … and never the concrete dockercli/restic packages, so it is fully unit-testable with fakes." The same isolation is applied to `internal/virshcli` and `internal/sshconn` (`internal/backup/vm_orchestrator.go:54-56`, `849-860`).

**Subprocess/exec patterns:**
- **None directly.** This package never calls `os/exec`. All external command execution (docker CLI, virsh, restic, zfs-over-SSH) happens behind interfaces; the concrete adapters live in other packages (`internal/restic`, `internal/dockercli`, `internal/virshcli`, `internal/sshconn` — not explored here).
- The only process-like constructs are **streams**: the zvol path bridges a remote `zfs send` stdout into `restic backup --stdin` via `io.ReadCloser` + a mandatory `wait()` reap function (`ZFSHost.StreamSend`, `internal/backup/vm_orchestrator.go:872`), and bridges `restic dump` into `zfs receive` via an `io.Pipe` + goroutine + done-channel (`RestoreZvolDisk`, `internal/backup/vm_orchestrator.go:1163-1177`). Streams are never buffered in memory (a zvol can be many GB).
- The one direct filesystem write is the VM domain-XML temp file: `os.MkdirAll(xmlDir, 0o700)` + `os.WriteFile(xmlPath, …, 0o600)` (with `//nolint:gosec // G306` annotation) before `virsh define` (`internal/backup/vm_orchestrator.go:747-754`).
- Repo-wide gotcha honored here: a cancelled exec surfaces as `*ExitError` from adapters; this package maps user cancellation via `errors.Is(err, context.Canceled)` → run status `"cancelled"` (`restoreOutcome`, `internal/backup/orchestrator.go:683-688`).

## Integrations

All integrations are **semantic interfaces defined inside this package**; adapters are wired at the service layer (`internal/api/service.go`). The orchestrators never touch docker.sock, the restic binary, virsh, or SSH directly.

| Interface | Backs | Methods (subset) | Real adapter (out of scope) |
|---|---|---|---|
| `Docker` (`orchestrator.go:72-95`) | containers | Stop/Start/WaitRunning/Health/Remove/Pull/CreateAndStart/InspectName/Allocations/Exec | dockercli |
| `Restic` (`orchestrator.go:98-117`) | all file backups | Backup / RestorePaths / RestoreSubtreeTo / VerifySnapshot | `internal/restic` engine |
| `Templates` (`orchestrator.go:123-126`) | Unraid XML templates | Read(dir,name)(xml,found,err) / Write(dir,name,xml) | filesystem impl in service layer |
| `Runs` (`orchestrator.go:129-132`) | run history | Start(targetID,kind) / Finish(runID,status,snapshotID,bytes,err) | SQLite (`internal/store`) |
| `VM` (`vm_orchestrator.go:60-81`) | libvirt domains | State/IsActive/DumpXML/Shutdown/Destroy/Start/Define/Undefine/Autostart/SnapshotCreateDiskOnly/BlockCommitActivePivot/GuestAgentPing | `internal/virshcli` |
| `ZFSHost` (`vm_orchestrator.go:860-877`) | TrueNAS zvol ops over SSH | SnapshotCreate/SnapshotDestroy/StreamSend(→io.ReadCloser+wait)/StreamReceive | `internal/sshconn.Conn` + virshcli argv builders |
| `ZvolRestic` (`vm_orchestrator.go:887-895`) | zvol byte-stream backup | BackupStdin / DumpTo | `internal/restic` (`BackupStdin`/`DumpRaw`) |
| `FlashRestic` / `FilesRestic` / `ConfigRestic` | flash / fileset / config | Backup-only, identical shape (`flash_orchestrator.go:13`, `files_orchestrator.go:14`, `config_orchestrator.go:11`) | same restic engine |

**Who talks to what, per orchestrator:**
- `BackupContainer` / `RestoreContainer` — Docker + restic + Unraid templates (`orchestrator.go`). Restore pre-flights IP/port conflicts via `Docker.Allocations`; pre/post hooks run via `Docker.Exec` (`sh -c`).
- `BackupVMGraceful` / `BackupVMLive` / `RestoreVM` — VM (virsh) + restic; zvol disks additionally ZFSHost + ZvolRestic (`vm_orchestrator.go`). NVRAM write-back on restore is a caller-supplied `PreDefine` hook (the SSH write lives in the service layer, not here).
- `BackupFlash` — restic only, over the mounted flash dir (e.g. `/host/boot`), hard-coded `--exclude .git` (`flash_orchestrator.go:39`). Flash restore is a zip download in the service layer, never in-place.
- `BackupFileSetDir` — restic only, over one resolved host folder; per-set excludes pass through verbatim (`files_orchestrator.go:44`).
- `BackupConfig` — restic only, over the **staged** `/config` snapshot dir (VACUUM-INTO DB + rclone.conf + ssh/), never the live config (`config_orchestrator.go:16-27`).

**Tag scheme written into restic (identity-stable retention — repo rule: never reintroduce `--group-by paths`, issue #91):**

| Domain | Tags | Where |
|---|---|---|
| container | `container:<ref>`, `p1` | `orchestrator.go:378` |
| VM graceful | `vm:<name>`, `p2` (+ `vmrun:<runID>` when RunTag set) | `vm_orchestrator.go:461` |
| VM live | `vm:<name>`, `p2`, **`live`** (+ RunTag) | `vm_orchestrator.go:555` |
| VM zvol disk | `vm:<name>:zvol:<dev>` (or `vm:<name>` when Dev empty), `p2` (+ RunTag) | `vm_orchestrator.go:1040-1044` |
| flash | `flash` | `flash_orchestrator.go:39` |
| file set | `fileset:<name>` — the ONLY link between snapshot and set (no defs dir) | `files_orchestrator.go:44` |
| config | `config` | `config_orchestrator.go:33` |

The `live` tag on live VM snapshots is why global `--group-by tags` is forbidden (repo CLAUDE.md rule). `withRunTag` (`vm_orchestrator.go:44-51`) appends the correlation tag as a fresh slice, never mutating the identity tags.

## Architecture

**Overall:** ports-and-adapters (hexagonal) around pure orchestrator functions. Every orchestrator is an exported function taking `(ctx context.Context, d XDeps)` where `XDeps` is a plain struct bundling the interface values plus config. No receivers, no global state (the only package-level mutable vars, `healthPollInterval`/`healthNoCheckGrace`, exist solely as test knobs — `orchestrator.go:314-317`).

**Key characteristics:**
- **Always-undo guarantee via deferred unwinding.** Backup ALWAYS restarts what it stopped, even on error: `BackupContainer` uses two deliberately-ordered defers inside a closure (target-restart defer registered second/runs first; dependents-restart defer registered first/runs last — "never leave a dep stopped", `orchestrator.go:394-464`); `runVMGraceful` defers `VM.Start` when it was running (`vm_orchestrator.go:432-439`); `runVMLive` always commits EVERY overlay back even if restic failed (`vm_orchestrator.go:561-569`); `BackupZvolDisk` defers `zfs destroy` of the consistency snapshot on every path (`vm_orchestrator.go:959-964`).
- **Guard chain before any destruction** (restores): `Confirmed` flag → strict snapshot-id hex (`snapshotIDRe`, 8–64 lowercase hex, `orchestrator.go:320`) → absolute + traversal-free path validation (`strings.HasPrefix(p, "/")` + no `..`) → wrong-target live re-inspect (containers) → `VerifySnapshot` pre-flight → IP/port conflict pre-flight (containers) → pull image before stop/remove → only then destructive steps. Guards return **sentinel errors** (`ErrNotConfirmed`, `ErrInvalidSnapshotID`, `ErrRestoreConflict`, `ErrVMNotInstalled`, `ErrContainerNotInstalled`) so the API layer can `errors.Is` without string matching (`orchestrator.go:37-60`).
- **Run recording brackets every operation**: `Runs.Start` before, `Runs.Finish(success|failed|cancelled)` after; error messages are scrubbed + capped at 500 chars via `truncateErr` (`orchestrator.go:931-946`). A guard failure returns WITHOUT recording a run (nothing destructive happened yet).
- **Best-effort vs fatal is explicit per step**: PreHook failure aborts the backup (never snapshot an inconsistent DB); PostHook failure is logged only; dependency stop failures are logged and that dep drops out of the restart set; template-read errors log but don't fail a valid snapshot; template-write errors DO fail it.
- **Stop-window choreography** (`BackupDeps`, `orchestrator.go:156-230`): `WasRunning` gates stop/start + hooks (#33: an already-stopped container is left exactly as it was, and dependencies carry their own `WasRunning`); `StopContainers` are restarted in compose `depends_on` order (always) with optional health-gating (`HealthWait`/`HealthTimeout`, #119); `WhileDependentsStopped` lets the update-after-backup recreate run inside the stop window, with a target re-wait afterwards so netns dependents (`network_mode: container:<target>`) never start against a torn-down target.
- **VM live backup has NO graceful fallback** — a VM chosen for live backup is never silently shut down. The only self-heal is one crash-consistent retry (no `--quiesce`) when a quiesced snapshot fails with a guest-agent freeze error (`isFreezeErr` string match, `vm_orchestrator.go:776-786`). Leftover-overlay recovery (snapshot name `bombvault-tmp`, exported as `LiveSnapshotName`) is the service layer's job, run before the next live backup.
- **Zvol path (v8.0.0 TrueNAS expansion)**: `zfs snapshot` → `zfs send` streamed over SSH into `restic backup --stdin` (synthetic path `/vm-disks/<dataset>@<snapName>`, `ZvolStdinPath`, `vm_orchestrator.go:905-907`) → deferred `zfs destroy`. Restore always lands in a FRESH dataset (`<base>-bombvault-restore-<unix-nano>`, structurally never the live source — `TestRestoreZvolDiskNeverTargetsSourceDataset`); rename/promote is a documented MANUAL operator step. A mixed file+zvol VM backup necessarily produces multiple restic snapshots (restic `--stdin` can't combine with a path-list backup); the per-disk snapshot ids are correlated at restore time via the `vmrun:<runID>` tag by the service layer.
- **Remapped (cross-instance/cross-pool) restores** share one type: `RestoreDir = VMRestoreDir` (`orchestrator.go:237`); non-empty `RestoreDirs` switches both container and VM restores from restore-in-place to per-subtree `RestoreSubtreeTo`.
- **Credential/path scrubbing with a deliberate local copy**: `scrubRunErr` + `truncateErr` (`orchestrator.go:869-946`) duplicate the regexes from `internal/restic/restic.go` and `internal/api/handlers.go` ON PURPOSE — importing the concrete restic package just for two regexes would break the DI seam. `ErrRestoreConflict` messages bypass scrubbing (`restoreConflictBypass`) so "8080/tcp" isn't mangled into "8080[path]".

**Primary backup flow (container, the richest example):**
1. `Runs.Start(targetID, "backup")` (`orchestrator.go:357`)
2. PreHook via `Docker.Exec` while still up; failure aborts (`orchestrator.go:366-372`)
3. Closure with two ordered defers: stop target (if WasRunning) → stop running deps (best-effort) → `Restic.Backup(paths, ["container:<ref>","p1"], excludes...)` if any paths → read flash template + write snapshot-scoped copy `my-<snapshotID>-<Name>.xml` (`orchestrator.go:386-500`)
4. Deferred unwind: target `Start` + `WaitRunning` (60s) → optional `WhileDependentsStopped` + re-wait → `restartStoppedDeps` (topological order, optional health-gated `waitHealthy`) (`orchestrator.go:399-439`, `543-584`)
5. PostHook best-effort; `Runs.Finish("success", snapshotID, bytes)`; re-throw on any failure with `Runs.Finish("failed", …, truncateErr(err))`

**State management:** none held. All state is either in the deps structs (captured up front: WasRunning, WasAutostart, Inspect) or delegated to the adapters. Run history and run-state live in the store behind `Runs`.

## Structure

```text
internal/backup/
├── orchestrator.go                  # container domain + shared DI interfaces, sentinels, guards, scrubbing (946 lines)
├── vm_orchestrator.go               # VM domain: graceful/live/zvol backup+restore (1226 lines)
├── flash_orchestrator.go            # flash domain: thin record-around-restic (48 lines)
├── files_orchestrator.go            # fileset domain: thin record-around-restic (53 lines)
├── config_orchestrator.go           # config domain: thin record-around-restic (42 lines)
├── export_test.go                   # test-only hook: SetHealthTimingForTest (shrinks poll/grace)
├── orchestrator_test.go             # container tests + shared fakes fakeDocker/fakeRestic/fakeTemplates/fakeRuns
├── vm_orchestrator_test.go          # VM tests + fakeVM (+ RunTag byte-identity regression pins)
├── vm_zvol_test.go                  # zvol unit tests + fakeZFSHost/fakeZvolRestic
├── vm_zvol_wiring_test.go           # zvol-in-VM wiring + file-only byte-identity regression pins
├── restart_health_test.go           # health-gated ordered restart (#119)
├── update_window_test.go            # WhileDependentsStopped hook timing
├── credential_scrub_internal_test.go# truncateErr scrubbing (internal pkg)
└── vm_freeze_internal_test.go       # isFreezeErr (internal pkg)
```

**Key files:**
- `internal/backup/orchestrator.go` — the seam. Read the package doc (lines 1-16) and the interface block (lines 62-132) first; everything else follows from them. Also holds `BackupContainer`/`RestoreContainer`, `restartStoppedDeps`/`waitHealthy`, the sentinel errors, `ValidSnapshotID`, `truncateErr`/`scrubRunErr`.
- `internal/backup/vm_orchestrator.go` — `BackupVMGraceful`, `BackupVMLive`, `RestoreVM`, `BackupZvolDisk`, `RestoreZvolDisk`, the `ZFSHost`/`ZvolRestic`/`VM` interfaces, `LiveSnapshotName`, `ZvolStdinPath`. The long section comment (lines 789-847) records the hardware-verification status of the zvol commands.

**Where to add a NEW backup domain** (follow `flash`/`files`/`config` as the template — they are the canonical minimal shape):
1. Create `internal/backup/<domain>_orchestrator.go` with:
   - a minimal `XRestic interface { Backup(ctx, repo, paths, tags []string, excludes ...string) (Summary, error) }` (Backup-only when the domain has no lifecycle to manage);
   - an `XBackupDeps struct { SourceDir string; Repo string; TargetID string; Restic XRestic; Runs Runs }` (add domain-specific fields with doc comments explaining why);
   - a thin exported `BackupX(ctx context.Context, d XBackupDeps) (Summary, error)` that does `Runs.Start` → `Restic.Backup` with the domain's identity tag → `Runs.Finish`, wrapping errors with the `"domain backup: "` prefix and routing every `Finish` error message through `truncateErr`.
2. If the domain needs restore, add it to the service layer (like flash/files) or a full `RestoreX` orchestrator with the full guard chain (like container/VM).
3. Tests: `internal/backup/<domain>_orchestrator_test.go` in package `backup_test`, reusing the shared `fakeRuns` from `orchestrator_test.go`; write your own thin `fakeXRestic`; assert the exact tag string, the run status on success AND failure, and that the run is attributed to the stable target id.
4. NEVER bypass the DI seam by importing `internal/restic`, `internal/dockercli`, or `internal/virshcli` here.

## Conventions

**Naming:**
- Orchestrators: exported, verb-first, `Backup<Domain>` / `Restore<Domain>` / `Backup<Domain><Variant>` (`BackupVMGraceful`, `BackupVMLive`); deps structs `<X>Deps` / `Backup<X>Deps` / `Restore<X>Deps`; per-domain restic interfaces `<X>Restic`.
- Unexported helpers are lowerCamelCase verbs (`restartStoppedDeps`, `waitHealthy`, `waitShutOff`, `runVMGraceful`, `backupBlockDisksAndLog`); `logPrefix` parameters keep helper log/error lines carrying the CALLER's method name.
- Tags are always `<kind>:<stable-identity>` (`container:plex`, `vm:win10`, `fileset:docs`, `vm:win10:zvol:vdb`).

**Error handling:**
- Wrap with `fmt.Errorf("domain: step: %w", err)` — the domain prefix ("`backup: `", "`restore: `", "`vm backup: `", "`vm live backup: `", "`zvol backup: `", "`zvol restore: `", "`config backup: `", "`flash backup: `", "`files backup: `") lets an operator tell which path failed from the log line alone.
- Sentinel errors for guard outcomes (`errors.Is` branching by the API layer); `%w`-wrapped human detail elsewhere (e.g. `ErrRestoreConflict` wraps the conflict list).
- Best-effort steps log via `log.Printf` and continue; fatal steps return. Errors written to `runs.error` ALWAYS go through `truncateErr` (scrub paths + credentials, then cap at 500 chars).
- `Runs.Finish` errors are ignored with `_ =` on the failure path (the original error wins) but returned on the success path.
- Cancellation: poll loops `select` on `ctx.Done()`; `restoreOutcome` maps `context.Canceled` → `"cancelled"` (distinct from `"failed"`, fires no failure alert). Adapters' exec failures arrive as `*ExitError`; cancellation is remapped via `ctx.Err()`.

**ctx discipline:** `ctx` is always the first parameter; every interface method takes it; time-bounded waits use `select { case <-ctx.Done(): … case <-time.After(interval): }`. Default timeouts are consts (`defaultStopTimeout` 30s, `runningWaitTimeout` 60s, `defaultHealthTimeout` 120s, shutdown poll 5s × 18 = 90s); values that tests must shrink are vars with a `SetHealthTimingForTest` hook instead.

**Slice discipline:** never mutate a caller's tags/paths slice — `withRunTag` copies before append; `append([]string(nil), d.DiskPaths...)` builds fresh path lists. This is asserted by the "byte-identical when RunTag empty" regression tests.

**Comments:** extensive doc comments explaining WHY, citing issue numbers (#31, #33, #57, #91, #119, bostafari), SEC section references, and cross-file contracts. When a field is intentionally not yet wired by a caller, the doc comment says so explicitly (see `VMBackupDeps.TPMPath`). Long "UPDATE (Task N)" blocks record design evolution inside the comment rather than deleting history. Match this density for anything touching the restore guards.

## Testing

**Runner:** stdlib `go test` only — no testify/ginkgo. Run via `go test ./...` from repo root (needs restic ≥ 0.17 on PATH per repo CLAUDE.md, though this package's tests never invoke the real binary).

**Two test-package modes:**
- External `package backup_test` (majority) — imports `internal/backup`; this is the default for new tests.
- Internal `package backup` — only for unexported units: `truncateErr` (`credential_scrub_internal_test.go`), `isFreezeErr` (`vm_freeze_internal_test.go`).

**Test doubles (fakes over mocks — no mock framework):** `fakeDocker`, `fakeRestic`, `fakeTemplates`, `fakeRuns` (`orchestrator_test.go:18-205`); `fakeVM` (`vm_orchestrator_test.go:14-109`); `fakeZFSHost`, `fakeZvolRestic` (`vm_zvol_test.go:26-106`); `fakeFlashRestic`, `fakeFilesRestic`. The universal pattern: each fake appends a `"verb:arg1:arg2"` string to a `log []string` slice and returns scripted errors from exported fields; tests assert on call ORDER by index (`idxOf`) and presence (`contains`/`vmContains`), never on exact full-log equality across fakes. Scripted sequences use per-name queues consumed one call at a time (`fakeDocker.healthSeq`).

**Patterns to copy:**
- Happy path + one test per failure/guard branch (`TestRestoreAbortsWhenNotConfirmed`, `TestRestoreRejectsBadSnapshotID`, `TestRestoreRejectsUnsafePath`, …).
- `t.Helper()` driver functions (`runHealthRestartBackup`, `runBackupWithHook`, `sampleVMBackupDeps`) that build full deps structs.
- `t.Context()` in newer tests (`restart_health_test.go`), `context.Background()` in older ones — prefer `t.Context()`.
- `t.TempDir()` for `DataDir`; `t.Cleanup` with `SetHealthTimingForTest` for timing knobs (`export_test.go`).
- **Byte-identity regression pins** for additive features: `TestRunTagEmptyIsByteIdentical*`, `TestBackupVMGracefulFileOnlyDiskUnchanged`, `TestRestoreVMFileOnlyDiskUnchanged` — every additive deps field ships with a test proving the zero value changes nothing.
- Structured-field flow-through assertions: `fakeDocker.createdInspect` captures the full `model.Inspect` so tests verify security-relevant fields survive the seam.

**Coverage (48 test functions total):** container backup/restore guards + hooks + dependency restart (~30), VM graceful/live/restore (~26 incl. TPM paths), zvol backup/restore + wiring (~19), health gating (6), update window (4), scrubbing (6), freeze detection (1). Flash/files/config: 2 tests each (success + failure).

**Gaps:**
- `parentDirs` (`vm_orchestrator.go:22-34`) has no direct unit test (only exercised via same-instance `RestoreVM` tests) — edge cases like a file directly under `/` rely on the defensive skip being right.
- `Runs.Start` failure path is tested for container/VM restore guards but not for the flash/files/config thin orchestrators.
- The zvol orchestration SEQUENCING is fake-tested only; the individual host commands are hardware-verified (TrueNAS SCALE 25.10.0, 2026-08-27) but no end-to-end hardware test exists for the ordering/cleanup paths — documented in `vm_orchestrator.go:789-806`.
- No benchmarks, no fuzzing, no race-detector-specific tests (the `io.Pipe` goroutine in `RestoreZvolDisk` is the only concurrency; `TestRestoreZvolDiskReceiveNeverBlocksForeverOnEarlyFailure` covers the deadlock edge).

## Concerns

**Tech debt — file size and comment weight:**
- Issue: `vm_orchestrator.go` (1226 lines) and `orchestrator.go` (946 lines) mix shared interfaces, five domains' logic, and multi-hundred-word historical doc comments. The zvol section alone carries ~60 lines of dated UPDATE commentary.
- Files: `internal/backup/vm_orchestrator.go`, `internal/backup/orchestrator.go`
- Impact: high cognitive load per change; the important invariants (defer ordering, guard order) are buried among historical notes.
- Fix approach: extract the zvol orchestrators (`BackupZvolDisk`/`RestoreZvolDisk` + interfaces) into `zvol_orchestrator.go`; collapse "UPDATE (Task N)" chains into a single current-state statement per contract. Preserve, do not shorten, the safety-property comments.

**Tech debt — deliberate cross-package duplication:**
- Issue: the path/credential scrub regexes exist in three copies (`internal/backup/orchestrator.go:869-872`, `internal/restic/restic.go`, `internal/api/handlers.go`); `zvolBackupSnapshotPrefix` (`bombvault-`) and the fresh-restore-dataset naming mirror identical contracts in `internal/virshcli`.
- Impact: a regex or naming fix must be applied N times or behavior diverges silently. The duplication is DOCUMENTED and justified (DI-seam isolation), so treat as accepted debt, not a bug.
- Fix approach: only consolidate if a shared leaf package with no adapter imports can host the regexes/naming without breaking the seam.

**Fragile area — defer ordering in `BackupContainer`:**
- Files: `internal/backup/orchestrator.go:386-440`
- Why fragile: correctness depends on the two defers' registration order inside the closure (dependents-restart registered FIRST so it runs LAST). A refactor that hoists either defer or merges them can leave a dependency stopped forever or start netns peers against a torn-down target.
- Safe modification: change behavior INSIDE the closure body; if the defers must move, re-read the block comment at lines 394-417 and keep the "never leave a dep stopped" defer last-to-run. `restart_health_test.go` + `update_window_test.go` pin the order — run them.

**Fragile area — `isFreezeErr` string matching:**
- Files: `internal/backup/vm_orchestrator.go:776-786`
- Why fragile: matches substrings ("freeze", "quiesce", "guest agent") in virsh error text; a libvirt/qemu wording change silently disables the crash-consistent retry and live backups of guests with broken fsfreeze hooks start failing outright (the exact Home-Assistant-style case it was built for).
- Safe modification: keep the substring list broad; if virshcli can classify errors structurally later, switch to that. `vm_freeze_internal_test.go` pins current matches.

**Documented wiring gap — NVRAM/TPM paths not populated by the real caller:**
- Files: `internal/backup/vm_orchestrator.go:131-154` (⚠ comment), `290-290`
- Issue: `VMBackupDeps.NVRAMPath`/`TPMPath` are exercised only by this package's own unit tests; per the in-code note (verified against the real code), `internal/api/service.go`'s `BackupVM` does not set `NVRAMPath`, and the live NVRAM capture actually rides a separate SSH mechanism in the service layer. The zvol `BlockDisks` side, by contrast, IS wired end to end (Task 2/3 notes).
- Impact: a reader may assume NVRAM/TPM are captured by the scheduled VM backup path when they are not, at this layer.
- Fix approach: service-layer work (out of this module); until then keep the ⚠ comments accurate when touching these fields.

**Multi-snapshot coupling for mixed file+zvol VMs:**
- Files: `internal/backup/vm_orchestrator.go:166-211`, `1008-1064`
- Issue: one mixed VM backup yields 1 + N restic snapshots (restic `--stdin` cannot combine with a path-list backup). This package logs per-disk snapshot ids but cannot persist them (`Runs.Finish` takes one id); correlation depends entirely on the `vmrun:<runID>` tag being present, which the service layer sets only for zvol-bearing VMs. Backups from before that feature have no `vmrun:` tag — restore then fails loudly per-disk (empty `SnapshotID` reaches restic) rather than silently skipping.
- Fix approach: keep the fail-loud fallback; do not add a silent skip. If run-recording ever gains multi-snapshot support, persist ids there.

**Security posture (strong, with two known-text tradeoffs):**
- In place: confirmation gate, strict hex snapshot-id regex (arg-injection guard), absolute + no-`..` path validation on every restore list, wrong-target live re-inspect before stop/remove, `VerifySnapshot` pre-flight before any teardown, IP/port conflict pre-flight, restore into fresh zvol datasets only, credential+path scrubbing of every persisted error, `0600` domain-XML perms, `--` end-of-options guard at the restic adapter.
- Known tradeoffs (documented at `orchestrator.go:855-868`): the path regex over-matches any slash token (hence the `ErrRestoreConflict` bypass) and under-matches unencoded `/` in passwords; the credential regex relies on a `@`-anchored shape. Both are accepted repo-wide, not local bugs.
- No secrets handling in this package: repo URLs/credentials stay inside the adapters; this package only sees scrubbed error strings.

**Performance:**
- No bottlenecks in this package: all heavy I/O is streamed through adapters; the only in-process buffering risks are handled (`io.Pipe` + `CloseWithError` unblocking in `RestoreZvolDisk`). The health-wait loop polls every 2s bounded by 120s/container — fine for tens of dependencies. `restoreStoppedDeps` restarts sequentially, which is correct (ordering) rather than slow.

---

*Module map analysis: 2026-09-09*
