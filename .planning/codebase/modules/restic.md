# Module Map: `internal/restic` — restic argv builders + execution engine

**Analysis Date:** 2026-09-09

Package doc (`internal/restic/restic.go:1-4`): "Package restic provides argv builders and execution helpers for the restic backup CLI. All operations run restic as an external process; no cgo or native bindings are used." This module is the ONLY place in BombVault that touches the restic binary. One file dominates: `internal/restic/restic.go` (2118 lines) holds every builder, the engine, JSON parsing, error scrubbing, and progress plumbing.

## Technology

**How the restic binary is located and invoked** (`internal/restic/restic.go`):

- The engine is a small value struct `Restic` (`restic.go:242-257`) with three fields: `Bin` (path or name of the restic binary, default `"restic"` via `bin()` at `restic.go:260-265`), `RcloneConfig` (path to the rclone config file), `CacheDir` (persistent restic cache location).
- There is NO version detection, vendoring, or embedding. The binary is expected on `PATH`; the only contract asserted in comments/tests is **restic >= 0.17** (`--insecure-no-password` requires it, `restic.go:269-270`; apt's restic is too old per `CLAUDE.md` — CI installs a current restic before running tests).
- Sole construction site: `cmd/bombvault/main.go:242` — `engine := &restic.Restic{Bin: "restic", RcloneConfig: filepath.Join(cfg.DataDir, "rclone.conf"), CacheDir: resticCacheDir}`. All other consumers (internal/api, internal/backup) go through an interface defined in internal/backup and fake engines in tests.

**Exec patterns** (`restic.go:853-1231`):

- Every invocation is `exec.CommandContext(ctx, r.bin(), args...)` with a `//nolint:gosec // G204` justification comment ("argv is constructed by typed builders in this package; no user input reaches here"). There are five execution helpers, all taking the already-built `cmd` plus `args` (args are needed only for the subcommand name in error/log messages):
  - `runBuffered` (`restic.go:965-976`) — capture all stdout into a buffer (snapshots/ls/check/forget/diff/stats default path).
  - `runWithStdin` (`restic.go:985-997`) — same, but stdin wired to an `io.Reader` (the `zfs send` zvol path).
  - `runToWriter` (`restic.go:1001-1009`) — stream stdout straight into an `io.Writer` (zip/raw dump downloads), stderr buffered for error scrubbing.
  - `scanLines` (`restic.go:1018-1057`) — pipe stdout through `bufio.Scanner`, invoke `onLine` per complete line, still accumulate full output (16 MiB max line size, `sc.Buffer(make([]byte, 0, 64*1024), 16<<20)`) so a trailing summary line survives an over-long status line. Scanner errors are logged and swallowed.
  - `streamLines` (`restic.go:1075-1106`) — `scanLines` without the accumulating buffer; scanner errors are FATAL here (a truncated listing must never pass as complete, issue #175).
- Dispatch wrapper `Restic.run` (`restic.go:924-944`): if `progress.SinkFrom(ctx)` returns a sink, it appends `RESTIC_PROGRESS_FPS=3` to the env and uses the streaming path (restic only emits periodic progress when stdout is a TTY or that env var is set; BombVault's stdout is a pipe); otherwise it runs buffered.
- Every child gets `configureProcGroup(cmd)` (`internal/restic/proc_unix.go:42-57` for POSIX, `proc_windows.go:17-19` for Windows): `Setpgid: true` puts restic in its own process group so ctx cancel SIGTERMs the whole **group** (`syscall.Kill(-pid, SIGTERM)`) — this reaps the `rclone` grandchild restic spawns for cloud backends. `cmd.WaitDelay = 10s` (`resticWaitDelay`) bounds the post-cancel wait before Go force-kills. SIGTERM (not SIGKILL) is deliberate: restic treats it as clean-abort and never writes a half-written snapshot (root-caused against a real corrupted repo 2026-08-12, see `proc_unix.go:26-41`).
- Cancel semantics: `ctxCancelErr` (`restic.go:957-962`) re-wraps any failure while `ctx.Err() != nil` as `fmt.Errorf("restic %s cancelled: %w", subcommand(args), ctx.Err())`, so `errors.Is(err, context.Canceled)` / `context.DeadlineExceeded` hold (exec's kill alone surfaces as a generic `*ExitError`).
- CPU cap: package-level `maxProcs atomic.Int64` (`restic.go:878-891`), set via `SetMaxProcs(n)` (clamps negatives to 0), exported to every child as `GOMAXPROCS` in `authEnv` (`restic.go:913-915`). Deliberately package-level, not a struct field, because all `Restic` methods use value receivers.

## Integrations

**External restic process** — the only integration; everything else is restic's own backend surface reached *through* argv + env:

**Repo location types** (recognized by `remoteRepoRe`, `restic.go:28`):

- **Local filesystem paths** — no scheme (`/mnt/user/bombvault/containers`). Default repo type.
- **rclone** (`rclone:Remote:bucket/path`) — the primary off-site mechanism (B2/S3/Drive via rclone). Authenticated by exporting `RCLONE_CONFIG=<path>` (`authEnv`, `restic.go:903-907`) when the config file exists.
- **restic native remote backends**: `s3:`, `sftp:`, `rest:`, `b2:`, `azure:`, `gs:`, `swift:`. Backend credentials travel in `Mode.Env` as `KEY=VALUE` pairs (`AWS_*` for S3, `RESTIC_REST_*` for rest-server) — never argv.
- `IsRemoteRepo(loc)` (`restic.go:33`) classifies; `LooksLikeUnprefixedRemote(loc)` (`restic.go:45-47`) catches the common operator mistake of typing `BackBlaze:bucket` without the required `rclone:` prefix (a scheme-like prefix that is not a recognized backend would otherwise be silently treated as a local path).
- **S3 storage class**: `Mode.StorageClass` is emitted as global `-o s3.storage-class=<class>` ONLY for a native `s3:` repo AND only for the `AllowedStorageClasses` whitelist (`restic.go:111-117`: STANDARD, STANDARD_IA, ONEZONE_IA, INTELLIGENT_TIERING, GLACIER_IR). GLACIER / DEEP_ARCHIVE are deliberately excluded — they need an asynchronous thaw, which would break restore (see `storageClassFlags`, `restic.go:140-148`; validated by `StorageClassAllowed`, `restic.go:131`).

**Credential transport (strict rule: env, never argv):**

- Repo password → `RESTIC_PASSWORD=<Mode.Password>` when `Mode.Encrypted`; otherwise `RESTIC_INSECURE_NO_PASSWORD=true` (`authEnv`, `restic.go:893-918`; builders emit the `--insecure-no-password` flag for unencrypted repos).
- `restic copy` additionally needs the SOURCE repo's password → `RESTIC_FROM_PASSWORD` appended in `Restic.Copy` (`restic.go:1812-1833`), not in `authEnv`.
- Foreign read-only sessions (`Mode.NoAmbientCreds`, `restic.go:70-80`) withhold `RCLONE_CONFIG` so this instance lends none of its own stored remotes to a caller-supplied location (issue #185).

**rclone** — no direct API use; restic spawns it as a grandchild for `rclone:` repos, which is exactly why the process-group kill exists.

**Progress surface** — `internal/progress`: `progress.Sink` (`func(percent float64)`) and `progress.CopySink` (`func(CopyProgress)`) are attached to `ctx` via `WithSink`/`WithCopySink` and read with `SinkFrom`/`CopySinkFrom` (`internal/progress/progress.go:19-77`). `run`/`Copy` check for them and switch to streaming execution.

**Boundary note:** the engine's Go-side consumers implement/depend on the `backup.Restic` interface (internal/backup) — fake engines in `internal/api/*_test.go` mimic `RepoOpens`, `Snapshots`, `Copy`, `ForgetPolicy`, `BackupStdin`, etc. New engine methods require touching that interface.

## Architecture

**Overall pattern:** pure-function **argv builders** (exported `*Args` functions, no I/O) + a thin **engine** (`Restic` value type whose methods run the built argv). Builders and engine are strictly separated so argv shapes are unit-testable without executing anything.

**Key characteristics:**

- Builders are deterministic, side-effect-free, and fully table-tested (`restic_args_test.go` pins exact argv slices).
- One `Mode` struct threads through every builder and method — it is the security/behavior context (encryption, lock policy, ambient credentials, storage class, bandwidth limits).
- Positional/user-influenced values (snapshot IDs, paths) are always placed **after `--`** (restic's end-of-options marker) as an arg-injection guard. This is enforced per-builder and pinned in tests.
- Every argv follows the same prefix order: `-r <repo>` → `-o s3.storage-class=…` (global option) → `--retry-lock 5m` → `--limit-upload/--limit-download` (global flags) → subcommand → subcommand-specific flags → `--` → positionals. Global flags MUST precede the subcommand (restic requirement).

**Engine composition** (`restic.go:1662-2043`): one method per restic subcommand, each just `r.run(ctx, XArgs(...), m)` + JSON parse:

| Method | Wraps | Notes |
|---|---|---|
| `Init` | `InitArgs` | creates repo, `--insecure-no-password` when unencrypted |
| `RepoOpens` / `RepoOpensErr` | `CatConfigArgs` | cheapest open+decrypt probe; `--no-lock` always |
| `Backup` | `BackupArgs` | parses summary; exit 3 → success-with-warning |
| `BackupStdin` | `BackupStdinArgs` + `runWithStdin` | `zfs send` stream in; mirrored by `DumpRaw` |
| `DumpZip` / `DumpRaw` | `DumpZipArgs` / `DumpRawArgs` + `runToWriter` | stream out to HTTP writer; cancel re-wrap |
| `Copy` | `CopyArgs` + `runBuffered`/`runStreamingCopy` | needs `RESTIC_FROM_PASSWORD`; text-scrape progress |
| `RestorePath` / `RestoreSubtreeTo` / `RestoreSubtreeInclude` / `RestoreInclude` | respective builders | NoLock flag for foreign sessions |
| `Snapshots` / `Stats` / `StatsRestoreSize` / `Diff` / `Ls` / `LsPath` | builders | buffered, `--no-lock`, single JSON parse |
| `LsStream` | `LsArgs` + `streamLines` | O(1) memory listing — do NOT collapse into `Ls` (see Concerns) |
| `TagAdd` / `Forget` / `ForgetPolicy` | builders | no-op on empty tags/IDs / inert policy |
| `Check` / `CheckData` | builders | `--read-data-subset=N%` drill, clamped 1..100 |
| `Unlock` / `Prune` / `CacheCleanup` | builders | `CacheCleanup` takes a zero `Mode{}` (opens no repo) |

**Data flow (backup with progress):**

1. Caller (internal/api via the backup domain) builds `Mode` and calls `Restic.Backup(ctx, repo, paths, tags, m, excludes...)` (`restic.go:1688-1703`).
2. `BackupArgs` composes argv; `run` builds env via `authEnv`, sees the `progress.Sink` in ctx, sets `RESTIC_PROGRESS_FPS=3`, and takes `runStreaming` → `scanLines`.
3. Each `{"message_type":"status","percent_done":…}` line is parsed by `statusPercent` (`restic.go:1364-1379`, clamped 0..100) and pushed to the sink; full stdout is still accumulated.
4. On exit: `backupExit3Err` (`restic.go:1280-1294`) maps restic exit code 3 on a `backup` subcommand to `ErrBackupSourceUnreadable` (snapshot WAS created; success-with-warning); otherwise `runError` (`restic.go:1299-1316`) logs full stderr server-side and returns a scrubbed, concise reason.
5. `ParseBackupSummary` (`restic.go:2103-2118`) scans for the `"summary"` JSON line and returns `Summary{SnapshotID, FilesNew, FilesChanged, BytesAdded}`.

**Error-sentinel pattern** (success-with-warning classification):

- `ErrRestoreMetadataOnly` (`restic.go:1242`) — restore whose stderr shows ONLY per-file ownership/permission errors on an Unraid `/mnt/user` FUSE target (`isMetadataOnlyRestoreFailure`, `restic.go:1328-1350`; deliberately conservative — any other error line means genuine failure). Detected via `metadataOnlyRestoreErr.Is`.
- `ErrBackupSourceUnreadable` (`restic.go:1267`) — backup exit 3.
- `ErrRestoreMetadataOnly`/`ErrBackupSourceUnreadable` are detected with `errors.Is` by callers; message text is preserved unchanged for callers that only display it.

**State management:** effectively stateless. The only mutable state is the package-level `maxProcs` atomic. Repo locks are restic's own concern — BombVault adds `--retry-lock 5m` (`resticRetryLock`, `restic.go:311`) on lock-taking ops (backup/forget/check/prune/copy) so cross-process locks are waited out, and `--no-lock` on strictly read-only ops (snapshots/stats/diff/diff/cat config, and restore/check under `Mode.NoLock`) so listings never collide with an exclusive prune lock (#57, #94/#96, #138).

## Structure

**File layout** (`internal/restic/`, 16 files, ~4223 lines total):

| File | Lines | Purpose |
|---|---|---|
| `internal/restic/restic.go` | 2118 | everything non-test: types, builders, engine, JSON parsing, error scrubbing, progress |
| `internal/restic/proc_unix.go` | 57 | `!windows`: `configureProcGroup` (Setpgid + SIGTERM-to-group + WaitDelay) |
| `internal/restic/proc_windows.go` | 19 | `windows`: WaitDelay only; default kill stays |
| `internal/restic/restic_args_test.go` | 778 | exact-argv tests for every builder |
| `internal/restic/restic_internal_test.go` | 318 | parsers, lastReason, metadata-only classifier, runError tagging |
| `internal/restic/restic_roundtrip_test.go` | 156 | real-binary init→backup→dump roundtrips (skips without restic) |
| `internal/restic/restic_copy_progress_internal_test.go` | 172 | copy progress pipeline via self-exec fake |
| `internal/restic/restic_stream_lines_internal_test.go` | 106 | streamLines fatal-on-truncation contract (#175) |
| `internal/restic/restic_cancel_test.go` | 85 | cancel/deadline re-wrap via self-exec sleeper |
| `internal/restic/credential_scrub_internal_test.go` | 95 | URL-credential + path scrubbing pins |
| `internal/restic/proc_test.go` | 109 | `!windows`: group kill + SIGTERM-not-SIGKILL proof |
| `internal/restic/restic_subcommand_test.go` | 46 | `subcommand()` extraction + lastReason basics |
| `internal/restic/copy_progress_fps_internal_test.go` | 74 | `RESTIC_PROGRESS_FPS=3` wiring pin (#159) |
| `internal/restic/maxprocs_internal_test.go` | 53 | GOMAXPROCS reaches child env |
| `internal/restic/backup_warn_test.go` | 37 | exit-3 sentinel classification |

**`restic.go` internal sections** (marked by banner comments): types & repo classification (`restic.go:25-265`) → argv builders (`:267-851`) → execution helpers (`:853-1231`) → error sentinels & scrubbing (`:1233-1657`) → high-level operations (`:1659-2043`) → JSON parsing (`:2045-2118`).

**Key functions quick index:**

- Builders: `InitArgs` `:319`, `CatConfigArgs` `:347`, `BackupArgs` `:361`, `DumpZipArgs` `:392`, `BackupStdinArgs` `:418`, `DumpRawArgs` `:445`, `CopyArgs` `:462`, `RestoreSubtreeToArgs` `:489`, `RestorePathArgs` `:509`, `LsArgs` `:530`, `LsPathArgs` `:546`, `RestoreIncludeArgs` `:560`, `RestoreSubtreeIncludeArgs` `:584`, `CheckArgs` `:602`, `CheckDataArgs` `:624`, `SnapshotsArgs` `:649`, `StatsArgs` `:669`, `StatsRestoreSizeArgs` `:687`, `DiffArgs` `:706`, `TagAddArgs` `:720`, `ForgetArgs` `:735`, `ForgetPolicyArgs` `:782`, `UnlockArgs` `:817`, `PruneArgs` `:831`, `CacheCleanupArgs` `:849`
- Flag helpers: `repoFlag` `:280`, `storageClassFlags` `:140`, `limitFlags` `:296`, `retryLockFlags` `:316`
- Execution: `run` `:924`, `authEnv` `:893`, `runBuffered` `:965`, `runWithStdin` `:985`, `runToWriter` `:1001`, `scanLines` `:1018`, `streamLines` `:1075`, `runStreaming` `:1146`, `runStreamingCopy` `:1216`, `ctxCancelErr` `:957`
- Errors/scrubbing: `runError` `:1299`, `backupExit3Err` `:1280`, `lastReason` `:1456`, `scrubSecrets` `:1443`, `itemErrorCause(s)` `:1543`/`:1577`, `subcommand` `:1641`
- Parsing: `statusPercent` `:1364`, `copyStatusPercent` `:1190`, `parseFileEntries` `:1957`, `parseDiffStatistics` `:2078`, `ParseBackupSummary` `:2103`

**Where a new restic subcommand wrapper goes** (prescriptive):

1. Add `XyzArgs(repo string, ..., m Mode) []string` in the argv-builders section of `internal/restic/restic.go`, following the canonical prefix order (`repoFlag` → `storageClassFlags` → `retryLockFlags`/`--no-lock` decision → subcommand → `insecureFlag` when `!m.Encrypted` → flags → `--` → positionals). Never interpolate user-influenced values before `--`.
2. Decide lock policy: lock-taking → `retryLockFlags()`; strictly read-only → hard-code `--no-lock` (see `SnapshotsArgs`/`CatConfigArgs` doc comments for the reasoning to copy).
3. Add the engine method `(r Restic) Xyz(ctx, ..., m)` in the high-level-operations section using `r.run` (or an execution helper if streaming).
4. If the subcommand is a new value-taking GLOBAL flag placement, add it to `subcommandValueFlags` (`restic.go:1630-1636`) or `subcommand()` misparses error messages; extend `TestSubcommandSkipsGlobalFlagValues`.
5. Pin the exact argv in `internal/restic/restic_args_test.go` (encrypted + unencrypted + NoLock variants, mirroring existing tests).

## Conventions

**Arg building:**

- Plain `[]string` appends, one concern per helper (`repoFlag`, `retryLockFlags`, `limitFlags`, `storageClassFlags`); composable in a fixed order. No flag struct/marshal library.
- `--` before every positional that derives from user/config input (snapshot IDs, paths, tags are flag values but IDs/paths are positional). Snapshot selectors use restic's `<id>:<path>` form (`DumpZipArgs`, `RestoreSubtreeToArgs`).
- Fixed `--host bombvault` on backups (`backupHost`, `restic.go:277`) so container recreation doesn't fragment restic's host grouping.
- Retention is **identity-stable**: `ForgetPolicyArgs` with a tag emits `--tag <tag> --group-by ""` (ungrouped). NEVER reintroduce `--group-by paths` for the tagged path (issue #91; `--group-by paths` survives only in the legacy tag=="" repo-wide pass). VM live snapshots carry an extra `live` tag — never `--group-by tags` globally.

**Password/env handling:**

- Credentials only ever enter via env: `RESTIC_PASSWORD`, `RESTIC_FROM_PASSWORD`, `RESTIC_INSECURE_NO_PASSWORD=true`, backend creds in `Mode.Env`, `RCLONE_CONFIG`, `RESTIC_CACHE_DIR`, `GOMAXPROCS` — all in one place, `authEnv` (`restic.go:893-918`). m.Env is appended LAST so a caller can override deliberately.
- Full stderr is logged server-side (`log.Printf` in `runError`) but only a scrubbed, ≤300-char reason (`maxReasonLen`, `restic.go:1517`) is returned to callers/UI.

**JSON output parsing:**

- NDJSON line-scanning pattern everywhere: `bytes.Split(out, []byte("\n"))` → `json.Unmarshal` per line → skip failures → select by `message_type` (`"summary"` → `ParseBackupSummary`, `"statistics"` → `parseDiffStatistics`, `"status"` → `statusPercent`, `"error"` → `restoreJSONError`). Single-object outputs (`snapshots`, `stats`) parse in one `json.Unmarshal`.
- restic copy has NO JSON mode; its plain-text progress is text-scraped by deliberately loose regexes anchored to stable prefixes only (`copyPercentRe`, `copyStartedRe`, `restic.go:1169-1179`) so future upstream wording changes degrade to "no percentage" rather than crashing.

**Error scrubbing:** `scrubSecrets` (`restic.go:1443-1446`) runs TWO passes in fixed order — path-like tokens first (`reasonPathRe` → `[path]`), then `user:pass@` userinfo (`credentialRe` → `[redacted]@`). The order is load-bearing: credentials-first would let the path pass eat the hostname an operator needs. Defense-in-depth: twins of this scrub exist in `internal/api/handlers.go` and `internal/virshcli/virshcli.go`. `lastReason` prefers informative stderr lines over restic's "open an issue" boilerplate and appends deduped, bounded per-item causes (max 3, each ≤100 chars) to count-only "There were N errors" summaries.

**Doc comments:** the house style is extensive — every non-obvious builder/exec helper carries the issue number, the live verification against restic 0.17.3, and the reason a "simpler" alternative was rejected. Preserve these when editing; they are the memory that prevents regressions (e.g. the `LsStream` "do not simplify" comment).

**Testing conventions are covered in `## Testing` below.**

## Testing

**Framework:** standard `go test`; no external assertion library — `reflect.DeepEqual` + `t.Fatalf`/`t.Errorf` throughout. Package naming splits two styles: internal tests (`package restic`, access unexported helpers) for argv/parse/scrub/proc internals, and one external test (`package restic_test` in `restic_roundtrip_test.go`) exercising only the exported API.

**Run commands:**

```sh
go test ./internal/restic/...   # this package alone
go test ./...                    # whole repo; needs restic >= 0.17 on PATH
```

**Test categories and patterns:**

1. **Exact-argv pins** (`restic_args_test.go`, 778 lines): every builder tested as a literal expected slice, `got := XArgs(...); want := []string{...}; reflect.DeepEqual`. Always cover: encrypted vs unencrypted (the `--insecure-no-password` flag), `NoLock` flag threading, flag ordering (global flags before the subcommand), `--` placement, clamping (`CheckDataArgs` percent 0→1, 250→100), Limits variants (both/upload-only/zero). Tests cite issue numbers in comments (#31, #61, #62, #91, #94/#96, #138, #159).
2. **Pure-parser tables** (`restic_internal_test.go`): `cases := []struct{name, line string; want...}` loops with `t.Run(c.name, ...)` — for `statusPercent`, `copyStatusPercent`, `copyStartedRe`, `lastReason` variants, `isMetadataOnlyRestoreFailure`, `runError` sentinel tagging.
3. **Self-exec child-process fakes** (the signature pattern of this package): the test binary re-execs ITSELF as the "restic" binary. A helper test (`TestResticSleeper`, `TestFakeResticCopyOutput`, `TestFakeResticOverlongLine`) checks an env var (`BOMBVAULT_RESTIC_SLEEPER`, `BOMBVAULT_RESTIC_COPY_FAKE`, `BOMBVAULT_RESTIC_STREAM_FAKE`) and, when set, blocks or prints a canned transcript; the parent test points `Restic{Bin: os.Args[0]}` at it with `-test.run=^TestFake...$` and `t.Setenv`/`cmd.Env`. This exercises the REAL stdout-pipe/bufio.Scanner/exec-kill paths cross-platform. Used by `restic_cancel_test.go`, `restic_stream_lines_internal_test.go`, `restic_copy_progress_internal_test.go`.
4. **POSIX-only process tests** (`proc_test.go`, `backup_warn_test.go`, `copy_progress_fps_internal_test.go`): `runtime.GOOS == "windows" → t.Skip` (or `//go:build !windows`); use `sh`/`sleep` fixtures with marker files to PROVE SIGTERM (not SIGKILL) reached a trap handler, and a shebang script fake-restic to capture the child's env.
5. **Real-binary roundtrips** (`restic_roundtrip_test.go`, `package restic_test`): `exec.LookPath("restic")` → `t.Skip("no restic")` when absent; `t.TempDir()` repo; init → backup → DumpZip zip inspection / BackupStdin → DumpRaw byte-identity. Runs in CI (restic installed by the workflow), skips for most local dev.
6. **Env-var wiring pins** (`copy_progress_fps_internal_test.go`, `maxprocs_internal_test.go`): assert the exact child env (`RESTIC_PROGRESS_FPS=3` only when a sink is present; `GOMAXPROCS` only when capped; negatives clamp to 0). `t.Cleanup(func(){ SetMaxProcs(0) })` to avoid cross-test leakage.

**Coverage gaps** (assessed from the file list, not a coverage run):

- `runError`'s `lastReason` boilerplate/informative selection is well tested, but `isPermCause`'s breadth (e.g. "read-only file system") has no dedicated table.
- `LooksLikeUnprefixedRemote`/`IsRemoteRepo` are table-tested; `authEnv`'s `RcloneConfig` stat-missing branch and `NoAmbientCreds` withholding have NO test (only the GOMAXPROCS slice is covered in `maxprocs_internal_test.go`).
- `Backup`/`BackupStdin`'s exit-3-with-parseable-summary success path is covered only indirectly (`backupExit3Err` unit + real-binary happy path); no fake produces an exit-3 child with a valid summary line.
- `parseDiffStatistics` has no dedicated table test in this package (exercised only via real restic in integration).
- Windows CI: `proc_test.go`, `backup_warn_test.go`, `copy_progress_fps_internal_test.go`, and both cancel/stream self-exec tests that need POSIX shells skip on Windows — fine for a Linux-only deployment, but the Windows dev sandbox runs a reduced suite.

## Concerns

**Monolith file:** `internal/restic/restic.go` at 2118 lines / 95KB carries six distinct concerns (builders, exec, parsing, error classification, scrubbing, progress). Impact: high cognitive load and merge-conflict surface; the banner comments (`---- argv builders ----` etc.) are the only navigation. Fix approach: mechanical split into `args.go` / `exec.go` / `errors.go` / `json.go` / `progress.go` within the package — pure code motion, tests already pin behavior, no API change needed. Low urgency, do it opportunistically.

**Fragile: `subcommand()` coupling.** `subcommand` (`restic.go:1641`) identifies the subcommand for EVERY error/log message by skipping known value-taking global flags (`subcommandValueFlags`, `restic.go:1630`). Adding a new global flag to any builder without registering it here makes every failure of that subcommand log/record the flag's VALUE as the subcommand name. Mitigation already in place: `TestSubcommandSkipsGlobalFlagValues` builds argv from the real builders so it breaks when a new flag is added — keep that test discipline for every new global flag.

**Security: credential exposure windows.** Current defenses are strong and layered (env-only credentials; `scrubSecrets` on every surfaced reason; path scrub before credential scrub; per-item causes scrubbed too). Two documented residual risks (`restic.go:1402-1423`): (1) benign `word:word@word` shapes are false-positive-redacted (cosmetic only); (2) a REAL false negative — a URL password containing an unencoded `/` is partially consumed by the path pass first, leaving the front half of the secret in the clear in surfaced reasons. Both live in `credentialRe`/`scrubSecrets`. The accepted tradeoff is documented in-code; if revisited, tighten only with a fix that provably cannot introduce a false negative — the doc comment explains why a safe tighter regex was not found.

**Fragile: text-scrape of `restic copy` progress.** `copyPercentRe`/`copyStartedRe` depend on restic 0.17.x's plain-text output format (no JSON mode exists). Deliberately loose anchors and defensive fallbacks (`snap == 0 → 1`) limit the blast radius to a missing percentage for one run, verified by `restic_copy_progress_internal_test.go`'s no-boundary case. A restic upgrade that changes the header shape will silently degrade off-site progress display — check this module's copy-progress tests after any restic version bump.

**Fragile: `Ls` vs `LsStream` memory.** `Restic.Ls` buffers restic's ENTIRE `ls --json` stdout; measured on a 672k-node snapshot: 1355 MiB retained / 1610 MiB churn vs 1–3 MiB for `LsStream` (`restic.go:1108-1120` doc comment). The type system will happily let someone "simplify" `LsStream` onto `Ls`. Any new listing feature MUST use `LsStream`/`streamLines`; consider deprecating buffered `Ls` for large snapshots.

**Performance: scanner ceiling.** The 16 MiB `bufio.Scanner` line cap (`scanLines`, `streamLines`) prevents a giant-path abort but also bounds the worst case per line; beyond it, `scanLines` swallows (logged) and `streamLines` fails the run by design (#175). Not a bug — but a listing of a snapshot with a path-like blob line >16 MiB will fail; that is the intended all-or-nothing tradeoff.

**Platform asymmetry: process-group kill.** `proc_windows.go` cannot replicate Setpgid/SIGTERM; on Windows a cancelled restic leaves the rclone grandchild and lock refresh to restic's own handling plus the 10s WaitDelay. BombVault never deploys on Windows (container target), so this is dev-sandbox-only, but tests exercising group-kill semantics all skip on Windows — do not assume the full cancel suite ran locally on Windows.

**No version gate on the restic binary.** The engine assumes restic >= 0.17 (`--insecure-no-password`, `--retry-lock`, `--read-data-subset`) but never checks `restic version`. An old binary produces flag errors at runtime surfaced only as scrubbed failures. Low risk (Docker image pins the binary) but a startup-time version assert in `cmd/bombvault` would make misconfiguration obvious.

---

*Module analysis: 2026-09-09*
