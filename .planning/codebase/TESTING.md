# Testing Patterns

**Analysis Date:** 2026-09-09

## Test Framework

**Runner:**
- Go: stdlib `go test` (Go 1.26 toolchain). No testify/ginkgo/testify anywhere — plain `if` + `t.Fatalf`/`t.Errorf`, `reflect.DeepEqual` for slices/argv.
- Web: vitest ^4.1.10, config `web/vitest.config.ts`.

**Assertion Library:**
- Go: none (stdlib). Web: @testing-library/react ^16.3.2 + jsdom ^30.0.1 for DOM tests.

**Run Commands:**
```sh
go build ./...            # compile first
go vet ./...
gofmt -l .                # must print nothing
golangci-lint run ./...
go test ./...             # REQUIRES restic >= 0.17 on PATH (apt's too old)
go test ./internal/restic/...   # single package
go test ./... -cover ./internal/api/   # inspect coverage (no target enforced)
just check                # the whole Go chain in one go (see justfile)
# Frontend (only when web/ changed):
cd web && npm ci && npm run build   # tsc --noEmit && vite build
npm test                  # vitest run
npm run lint              # eslint src (includes the 8 bombvault/* rules)
```

## Test File Organization

**Location:** co-located with sources everywhere (Go and web).

**Naming:**
- Go: `*_test.go` = external package (`api_test`, `backup_test`, `restic_test`) — the default for new tests; `*_internal_test.go` = internal package for unexported units (`credential_scrub_internal_test.go`, `csrf_internal_test.go`, `migrate_upgrade_internal_test.go`).
- Web: `*.test.ts` = pure node; `*.dom.test.tsx` = jsdom via a `// @vitest-environment jsdom` docblock as the FIRST line of the file (default env is node). Test files are excluded from the tsc program (`web/tsconfig.json`) — esbuild transpiles them.

## Test Structure

**Suite Organization (Go, the universal shape):**
```go
func TestRestoreAbortsWhenNotConfirmed(t *testing.T) {
    d := sampleDeps()                 // t.Helper() driver builds full deps struct
    d.Runs = &fakeRuns{}
    d.Restic = &fakeRestic{}
    err := RestoreContainer(t.Context(), d)
    if !errors.Is(err, ErrNotConfirmed) {
        t.Fatalf("want ErrNotConfirmed, got %v", err)
    }
}
```

**Patterns:**
- Subtests via `t.Run(name, ...)` with table structs (`internal/restic/restic_internal_test.go` parsers).
- `t.Helper()` driver functions building full deps (`runHealthRestartBackup`, `sampleVMBackupDeps` in `internal/backup`).
- `t.Context()` in newer tests (prefer it over `context.Background()`); `t.TempDir()` for DataDir/repos; `t.Cleanup` for timing knobs and `SetMaxProcs(0)`.
- Happy path + one test per failure/guard branch (e.g. `TestRestoreRejectsBadSnapshotID`, `TestRestoreRejectsUnsafePath`).
- **Byte-identity regression pins**: every additive deps field ships a test proving the zero value changes nothing (`TestRunTagEmptyIsByteIdentical*` in `internal/backup`).

## Mocking

**Framework:** none — hand-written FAKES over mocks, universal across packages.

**Patterns:**
```go
// internal/backup/orchestrator_test.go — every fake appends "verb:arg" strings
// to a log slice; tests assert call ORDER by index (idxOf) and presence.
type fakeRestic struct{ log []string; err error }
func (f *fakeRestic) Backup(ctx context.Context, repo string, paths, tags []string, excludes ...string) (Summary, error) {
    f.log = append(f.log, "backup")
    return Summary{}, f.err
}
```
- Fixtures: `internal/api/testutil_test.go` — `newMemStore(t)` (in-memory SQLite + Migrate + Cleanup), `fakeServiceDocker` (records `calls` labels for ordering assertions), `fakeVirsh`; `fakeResticEngine` (standard `ResticEngine` fake) in `internal/api/service_test.go`; `fakeRuns`/`fakeVM`/`fakeZFSHost` in `internal/backup`.
- Store: `db := store.OpenMem(t)` → `store.Migrate(db)` → `store.New(db)` (`internal/store/helpers_test.go:10`); file DBs via `t.TempDir()` when file semantics matter.
- Handler tests go through the REAL router: `newTestRouter(t, d, eng)` (`internal/api/handlers_test.go:23-63`) wires Handler + Service over fakes and returns the real mux, so `authGate`/`csrfGate` are exercised. Assert with `httptest.NewRecorder` + JSON decode of the `{ok, ...}` envelope.
- **Self-exec child-process fakes** (the signature `internal/restic` pattern): the test binary re-execs ITSELF as "restic" — a helper test checks an env var (`BOMBVAULT_RESTIC_SLEEPER` etc.) and, when set, blocks or prints a canned transcript; the parent points `Restic{Bin: os.Args[0]}` at it with `-test.run=^TestFake...$`. Exercises the REAL stdout-pipe/exec-kill paths (`restic_cancel_test.go`, `restic_stream_lines_internal_test.go`).
- Real-binary roundtrips: `restic_roundtrip_test.go` runs init→backup→dump against `t.TempDir()` repos and `t.Skip`s when `exec.LookPath("restic")` fails (runs in CI, skips locally without restic).

**What to Mock:** docker daemon, virsh, restic engine, SSH/ZFS host, templates FS, runs store — everything behind the `internal/backup` ports.

**What NOT to Mock:** the router/middleware stack in handler tests (use the real one); SQLite (use real in-memory); the restic ARGV shapes (pin them exactly in `internal/restic/restic_args_test.go` rather than mocking builders).

**Async discipline (repo rule):** backups/restores run in detached goroutines; tests MUST wait for them (`newTestRouterSvc` exists for exactly this; see `panic_recovery_test.go`, `run_group_test.go`). Linux CI flakes otherwise.

## Fixtures and Factories

**Test Data:**
- Table-driven structs for parsers (`cases := []struct{name, line string; want ...}`).
- Scripted sequences via per-name queues consumed one call at a time (`fakeDocker.healthSeq`).
- Structured-field flow-through: `fakeDocker.createdInspect` captures the full `model.Inspect` so tests verify security-relevant fields survive the seam.

**Location:** fakes live beside the tests they serve (`orchestrator_test.go`, `vm_zvol_test.go`, `service_test.go`); shared store helper in `internal/store/helpers_test.go`; gitleaks allowlist covers `internal/*_test.go` and `web/src/*.test.tsx?` so planted fake credentials in tests are fine — realistic fake AWS/GitHub credentials OUTSIDE the allowlist still trip the pre-push hook.

## Convention-Guard Tests (special class)

- `internal/store/settings_writers_test.go:50` — source-scan failing on any production `.UpdateSettings(` call.
- `internal/releasenotes/releasenotes_test.go:47` — embed-sync: every `.github/release-notes/*.md` must have an identical embedded copy.
- `internal/releasenotes/catalog_version_test.go:32` — TrueNAS catalog must track the newest release note.
- `internal/store/migrate_upgrade_internal_test.go` — reconstructs every reachable historical DB state FOR REAL (running actual migration bodies under historical numbers) and asserts convergence to the fresh-install schema. Extend this when touching migrations.
- `web/src/app/routedPages.test.ts` — every routed page must satisfy the page-shell rule; `web/src/lib/uiConventions.test.ts` tests the lint rules themselves; `i18n.parity/quality/orphans` tests guard the 41 locale tables.

## Coverage

**Requirements:** None enforced (Go or web). Inspect with `go test ./... -cover ./internal/api/`.

## Test Types

**Unit Tests:** the overwhelming majority — pure functions, builders, parsers, orchestrators over fakes.
**Integration Tests:** handler tests through the real router + in-memory SQLite; real-restic roundtrips in CI.
**E2E Tests:** None. No Playwright/Cypress; the web repo comments describe live-manual verification only.

## Common Patterns

**Async Testing:**
```go
// wait for the detached goroutine before asserting (CI flakes otherwise)
```
Use the `newTestRouterSvc` blocking pattern; see `run_group_test.go`.

**Error Testing:** assert sentinel classification with `errors.Is`, and that guard failures record NO run (nothing destructive happened).

**Env-var wiring pins:** assert the exact child env (`RESTIC_PROGRESS_FPS=3` only when a sink present; `GOMAXPROCS` only when capped) — `copy_progress_fps_internal_test.go`, `maxprocs_internal_test.go`.

**POSIX-only tests:** `runtime.GOOS == "windows" → t.Skip` (or `//go:build !windows`); marker files prove SIGTERM reached a trap handler (`internal/restic/proc_test.go`). These skip on Windows — the full cancel suite does NOT run locally on Windows.

**CI gates (`.github/workflows/lint.yml` + `build.yml`):**
- `test` job: install restic 0.17.3 from upstream (SHA256-checked; keep in sync with the Dockerfile's `RESTIC_SHA256_AMD64`) → `go test ./...`.
- `web` job: `npm ci && npm run lint && npm test`.
- `smoke` job: amd64 image must serve `/api/health` and run tini as PID 1; Trivy scan is non-blocking.

---

*Testing analysis: 2026-09-09*
