---
phase: 1
slug: selection-engine-restore-safety
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-09-09
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `01-RESEARCH.md` § Validation Architecture (authoritative source for this contract).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `go test` (no testify), `httptest` router harness (`newTestRouter`) |
| **Config file** | none — stdlib convention (`*_test.go` beside code); `just check` runs the Go chain |
| **Quick run command** | `go test ./internal/api/ -run 'TestSelect\|TestBrowse\|TestRestore\|TestBackupPaths' -count=1` |
| **Full suite command** | `go test ./...` (restic-dependent tests skip locally; run fully on CI) |
| **Estimated runtime** | ~30 seconds (quick, fakes only) / several minutes (full) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/api/ -run 'TestSelect\|TestBrowse\|TestRestore\|TestBackupPaths' -count=1` (target < 30 s — fakes only)
- **After every plan wave:** Run `go test ./...` + `go vet ./...` + `gofmt -l .` (must print nothing) + `golangci-lint run ./...`
- **Before `/gsd-verify-work`:** Full suite must be green (CI: restic 0.17.3 installed)
- **Max feedback latency:** 30 seconds (quick path)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| (populated at validate-phase, after PLAN task IDs exist) | | | SELECT-01 | T1 symlink escape / T4 path disclosure | containment re-checked at save (`toContainerPath`) | unit (table) | `go test ./internal/api/ -run TestNormalizeSelection -count=1` | ❌ W0 `selection_test.go` | ⬜ pending |
| (as above) | | | BROWSE-03 | T1 symlink escape | `os.Root` kernel-enforced containment; `paths.Resolve` first reject | contract | `go test ./internal/api/ -run TestBrowse -count=1` | ✅ traversal tests exist + ❌ symlink fixture | ⬜ pending |
| (as above) | | | INTEG-04 (backend) | T7 auto-detect inversion | PATCH guard `{ok:false, code:"empty-selection"}`; legacy `[]` unchanged | handler/contract | `go test ./internal/api/ -run TestEmptySelectionGuard -count=1` | ❌ W0 | ⬜ pending |
| (as above) | | | RESTORE-01 | T5 crafted PATCH path | longest-prefix vs snapshot `Paths`; empty-intersection abort BEFORE teardown | integration | `go test ./internal/api/ -run TestRestoreSelectionChange -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/api/selection_test.go` — normalization/pruning/classification tables (SELECT-01/04, both root configs)
- [ ] Browse contract additions (`internal/api/handlers_test.go` or new `browse_contract_test.go`): cap+truncated, status trio, hidden delta, escaping-symlink fixture (GOOS-guarded) — BROWSE-01..04
- [ ] `internal/api` restore-hardening tests (new file, e.g. `restore_selection_test.go`): the five RESTORE-01 cases, seeded `fakeResticEngine.snaps`
- [ ] Empty-selection guard test: PATCH boundary, coded envelope + legacy-compat cases (INTEG-04 backend)
- [ ] `internal/restic` positional contract test file (real binary, skip pattern — green on CI only)
- No framework/config installs needed — stdlib infra complete

---

## Manual-Only Verifications

*All phase behaviors have automated verification.* Note: the restic 0.17 positional contract tests (L12) skip on the restic-less Windows dev box and prove only on CI (restic 0.17.3 installed by lint.yml) — automated, environment-constrained.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
