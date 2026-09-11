---
phase: 03-selection-trust-controls
plan: 01
subsystem: backup-engine
tags: [restic, sqlite, migration, http-api, argv-builders]

requires:
  - phase: 01-selection-encoding
    provides: SetBackupPaths boundary validation and toContainerPath containment discipline reused for excludeCaches keys
  - phase: 02-tree-editor
    provides: FoldersEditor save surface the 03-03 toggle will ride on
provides:
  - targets.exclude_caches column (migration v100) with owned setter Repo.SetExcludeCaches
  - restic.Mode.ExcludeCaches field + BackupArgs emission of the constant --exclude-caches flag
  - PATCH /api/containers/{name} excludeCaches field (nil=untouched, {}=clear) with atomic whole-save validation
  - GET /api/containers/{name}/mounts excludeCaches key (always an object)
  - Service.SetExcludeCaches boundary validation (toContainerPath containment + 64-entry cap)
  - anyRootExcludeCaches union compiled into mode at backup time from the fresh target re-read
affects: [03-02 frontend wiring, 03-03 CACHEDIR toggle UI, RESTIC-01 verification]

actuals:
  tokens: 10000   # chars/4 over the realized internal/ diff (39,930 chars)
  tasks: 2
  commits: 4

tech-stack:
  added: []
  patterns:
    - "Per-root UI state persisted as a root-to-bool JSON map compiled at backup time into an item-level boolean union (only the union reaches argv)"
    - "Non-pointer map body field: absent decodes to nil = untouched, explicit {} = clear — maps natively distinguish absent from zero"

key-files:
  created: []
  modified:
    - internal/store/migrate.go
    - internal/store/targets.go
    - internal/store/targets_test.go
    - internal/restic/restic.go
    - internal/restic/restic_args_test.go
    - internal/api/handlers.go
    - internal/api/service.go
    - internal/api/service_test.go
    - internal/api/handlers_test.go

key-decisions:
  - "mode.ExcludeCaches is set after the UpsertTarget re-read rather than literally inside primaryModeFor: tg does not exist at the mode-construction line, the upsert re-read IS the fresh target read (ON CONFLICT never touches exclude_caches), and a separate read would be redundant — behavior identical, one store read saved"
  - "Non-pointer map[string]bool body field (not *map): absent key decodes to nil which is the untouched signal; an explicit empty object clears every root toggle"
  - "64-entry cap on the map as defense-in-depth on top of the 1 MiB body cap and the map[string]bool decode (T-03-02)"

patterns-established:
  - "JSON map column lifecycle mirroring excludes: v-numbered ALTER + owned setter + ON CONFLICT omission + scanTarget unmarshal (five positional SQL sites move together)"
  - "Item-level union helper (anyRootExcludeCaches) computed at the single Backup mode site, documented as the literal A1 reading"

requirements-completed: [RESTIC-01]

coverage:
  - id: D1
    description: "Migration v100 target_exclude_caches plus Target.ExcludeCaches persistence: round-trip, Upsert preserves, pre-v100 '{}' scans empty, deterministic idempotent re-set, nil clears"
    requirement: RESTIC-01
    verification:
      - kind: unit
        ref: internal/store/targets_test.go#TestSetExcludeCachesRoundTripAndUpsertPreserves
        status: pass
    human_judgment: false
  - id: D2
    description: "BackupArgs emits the constant --exclude-caches between the tag loop and the exclude loop when Mode.ExcludeCaches is true; zero value byte-identical to the pre-phase baseline"
    requirement: RESTIC-01
    verification:
      - kind: unit
        ref: internal/restic/restic_args_test.go#TestBackupArgsExcludeCaches
        status: pass
    human_judgment: false
  - id: D3
    description: "PATCH excludeCaches boundary: valid host-path keys persist, an out-of-mount key rejects the whole save atomically with a scrubbed error, non-boolean values fail at decode, absent key leaves the column untouched; mounts response serves the identical map, {} when never set"
    requirement: RESTIC-01
    verification:
      - kind: integration
        ref: internal/api/handlers_test.go#TestPatchContainerExcludeCaches
        status: pass
      - kind: integration
        ref: internal/api/handlers_test.go#TestContainerMountsExcludeCaches
        status: pass
    human_judgment: false
  - id: D4
    description: "Backup compiles the stored per-root toggles into mode.ExcludeCaches from a fresh target read: A1 edge (toggled root deselected, other roots backed up, flag still fires), all-false and cleared maps leave it off"
    requirement: RESTIC-01
    verification:
      - kind: integration
        ref: internal/api/service_test.go#TestBackupExcludeCachesUnion
        status: pass
    human_judgment: false

duration: 13min
completed: 2026-09-10
status: complete
---

# Phase 3 Plan 1: CACHEDIR.TAG Backend Slice Summary

**Per-root exclude-caches toggle persisted as a targets JSON map (migration v100) and compiled at backup time into restic's constant --exclude-caches flag through Mode threading, with atomic PATCH validation and nil-safe mounts serving**

## Performance

- **Duration:** 13 min
- **Started:** 2026-09-10T19:01:48Z
- **Completed:** 2026-09-10T19:14:31Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- Migration v100 `target_exclude_caches` appended after v99 (numbering-hazard rules preserved) with the owned setter `Repo.SetExcludeCaches`; UpsertTarget inserts but never resets the column (ON CONFLICT omission)
- `restic.Mode.ExcludeCaches` (Limits precedent — no `backup.Restic` widening, `internal/backup` untouched) emits the bare constant `--exclude-caches` in `BackupArgs` exactly between the tag loop and the exclude loop; zero value is argv-byte-identical
- HTTP boundary: PATCH `excludeCaches` field (nil = untouched, `{}` = clear) with `Service.SetExcludeCaches` validating every host-path key through the `toContainerPath` containment discipline (whole-save atomic reject, scrubbed error) plus a 64-entry cap; mounts response always serves an object under `excludeCaches`
- Backup-time union: `anyRootExcludeCaches(tg.ExcludeCaches)` sets `mode.ExcludeCaches` from the Upsert re-read — recomputed fresh on every backup, literal A1 reading (independent of selection inclusion), pinned by a wiring test through the Mode-recording fake engine

## Task Commits

Each task was committed atomically (TDD: RED then GREEN per task):

1. **Task 1: CACHEDIR.TAG path end-to-end at the persistence and engine layers** (tracer)
   - `3e33a956` (test): failing argv + store round-trip tests
   - `8ece98ff` (feat): migration v100, store field/setter/sites, Mode field + flag emission
2. **Task 2: HTTP boundary and service union**
   - `73f6cb89` (test): failing router/wiring tests + fake Mode recording
   - `a35a7cfd` (feat): PATCH field, mounts key, service setter, union threading

**Plan metadata:** (see final docs commit below)

## Files Created/Modified
- `internal/store/migrate.go` — migration v100 `target_exclude_caches` (ALTER TABLE targets ADD COLUMN exclude_caches TEXT NOT NULL DEFAULT '{}')
- `internal/store/targets.go` — `Target.ExcludeCaches` field, five positional SQL sites (INSERT-only upsert, three SELECTs, scanTarget), owned setter `SetExcludeCaches` (~525)
- `internal/store/targets_test.go` — `TestSetExcludeCachesRoundTripAndUpsertPreserves` (includes pre-v100 row + raw-JSON determinism pins)
- `internal/restic/restic.go` — `Mode.ExcludeCaches` why-commented field; `BackupArgs` flag emission between tags and excludes
- `internal/restic/restic_args_test.go` — `TestBackupArgsExcludeCaches` (presence pin + two zero-value absence pins)
- `internal/api/handlers.go` — PATCH body field + branch (~1133), mounts response key (~1349)
- `internal/api/service.go` — `Service.SetExcludeCaches` (~10136), `ContainerMounts` 5th return, union at `mode.ExcludeCaches = anyRootExcludeCaches(...)` (~4187), helper (~10159)
- `internal/api/service_test.go` — `TestBackupExcludeCachesUnion`; `fakeResticEngine.Backup` records `lastMode`; four `ContainerMounts` call sites updated for the new return
- `internal/api/handlers_test.go` — `TestPatchContainerExcludeCaches`, `TestContainerMountsExcludeCaches`, shared split-root harness

## Decisions Made
- `mode.ExcludeCaches` is assigned after the `UpsertTarget` re-read rather than literally inside `primaryModeFor` (~4116): `tg` does not exist at the mode-construction line, the upsert's authoritative re-read IS the fresh target read (ON CONFLICT never touches `exclude_caches`), and a separate earlier read would be redundant. Behavior identical; the must-have "recomputed on every backup from a fresh target read" is honored exactly.
- Non-pointer `map[string]bool` body field: an absent key decodes to nil, which is the untouched signal; an explicit `{}` clears every root toggle (maps natively distinguish absent from zero, unlike the scalar pointer siblings).
- 64-entry cap on the map (`maxExcludeCachesEntries`) as defense-in-depth on top of the 1 MiB body cap and the `map[string]bool` decode (threat T-03-02).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. (gofmt realigned the new migration entry and body-struct field on first write — fixed in-place before each GREEN commit, no behavioral impact.)

## Verification

- `go test ./...` green (full local suite; restic-binary contract tests skip on this Windows box per the documented reduced suite — CI runs them against restic 0.17.3)
- `gofmt -l .` prints nothing; `go vet ./...` and `go build ./...` clean; golangci-lint runs in CI (not on this box's PATH, Phase 2 precedent)
- Prohibition checks: `internal/store/settings_writers_test.go` green (no MutateSettings routing); exactly one appended migration entry after v99; flag pinned between verb and `--`; argv test asserts the bare constant flag (no user string reaches argv)
- Tracer feedback gate re-run end-to-end after Task 1 (auto mode): verify chain passed before expansion

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- RESTIC-01 backend complete: persistence (D-07), PATCH/mounts wire fields, and the item-level union flag (D-06) all landed and pinned; plans 03-02 (frontend wiring) and 03-03 (CACHEDIR toggle UI) can consume the `excludeCaches` mounts key and PATCH field as-is
- SELECT-03, INTEG-03, INTEG-04 untouched by this plan (frontend plans)

## Self-Check: PASSED

All 9 modified key files exist on disk; all 4 task commits (3e33a956, 8ece98ff, 73f6cb89, a35a7cfd) present in git log.

---
*Phase: 03-selection-trust-controls*
*Completed: 2026-09-10*
