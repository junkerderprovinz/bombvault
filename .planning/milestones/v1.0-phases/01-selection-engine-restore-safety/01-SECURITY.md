---
phase: 01
slug: selection-engine-restore-safety
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-09-10
---

# Phase 01 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

> Register merged from the five `<threat_model>` blocks (01-01→01-05-PLAN.md, all authored at plan time).
> Verification depth: L1 grep-level classification (ASVS L1 short-circuit — threats_open 0, register authored
> at plan time), corroborated by the full green test suite, the 10/10 re-verification (01-VERIFICATION.md),
> and the 19-file code review (01-REVIEW.md, 0 Critical / 1 Warning fixed at 04363500).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| PATCH body → selection store | User-influenced path strings cross into persisted state and later into restic argv | `backupPaths` entries (prefixed exclusions, bare includes); untrusted |
| Stored selection → readers/backup | Persisted strings are re-parsed by six readers; a mis-classified entry inverts backup behavior | Flat string list; must stay host-mount contained |
| Stored selection → restic argv (excludes) | Stored exclusion branches cross into `--exclude` patterns consumed by the restic binary (surface opened by gap closure 01-05) | Derived absolute container-form patterns; typed builders, no shell |
| Query string → filesystem listing | `?path=` is untrusted input that must never escape the host mount root, directly or via symlink | Path string; kernel + lexical containment |
| Listing response → SPA | Entry names and error text cross to the client; must not leak host layout or raw paths | Names, status kind, scrubbed error text |
| Stored selection → restore selectors | Persisted paths become restic restore selectors; re-contained before use | Selector strings after `--` in argv |
| Snapshot metadata → mapping input | Snapshot Paths come from repo metadata; mapping decides what is destructively restored | Repo-owned path list |
| Restore outcome → run record | Skip notes cross into persisted run rows shown in the SPA | Bounded, scrubbed note text |
| Backup argv → snapshot metadata | Derived patterns are recorded in the snapshot's restic `Excludes` metadata (user-owned exclusions-editor surface, accepted) | Absolute container-form paths |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-01-01 | Tampering | service.SetBackupPaths | high | mitigate | Per-element containment re-check via toContainerPath on the bare path of both classes (service.go:1189); TrimSpace; bare "!" with empty path rejected; prefix parsed before translation | closed |
| T-01-02 | Tampering | Backup → restic argv | high | mitigate | Selection compiles to positionals only (after `--`); at plan-01-01 time no exclude was derived from the selection — the surface later opened by 01-05 is governed by T-01-05-01..04 (positionals remain maximal roots, excludes strictly derived from stored exclusion entries; contract tests 3/3 under real restic) | closed |
| T-01-03 | Tampering (user intent) | storedDataIsGone / #181 guard | medium | mitigate | Explicit-none classification returns false so a deselected container is never refused as not reachable; three-state table tests (selection_readers_internal_test.go:88-132) | closed |
| T-01-04 | Information Disclosure | SetBackupPaths error text | low | accept | Error echoes the caller's own submitted path (`%q`) — same-subject disclosure, house pattern, flows through failEnvelope scrubbing | closed |
| T-01-05 | Information Disclosure | handleBrowse direct-address symlink | high | mitigate | os.Root kernel-enforced containment layered behind paths.Resolve lexical reject (handlers.go:4250-4256, os.OpenRoot); TestBrowseSymlinkEscapeRejected PASS (CI, GOOS-guarded fixture) | closed |
| T-01-06 | Information Disclosure | handleBrowse traversal | high | mitigate | Browsable universe stays under HostMountRoot only; byte-identical traversal/absolute rejection; no new route (same authGate/csrfGate chain) | closed |
| T-01-07 | Denial of Service | handleBrowse huge directories | medium | mitigate | maxBrowseEntries = 500 cap + truncated flag (handlers.go:2984); single ReadDir per request; no recursion, no emptiness probes | closed |
| T-01-08 | Information Disclosure | browse error text / status field | medium | mitigate | Generic "could not read directory" string verbatim; status carries the error KIND, never the path; TestBrowseStatusRestricted PASS | closed |
| T-01-09 | Tampering | prepareRestoreForTarget mapping input | high | mitigate | paths.Within re-validation of the STORED list before mapping (service.go:5497) PLUS the mapped list before the destructive phase (service.go:5531, review fix 04363500); orchestrator SEC guard unchanged; selectors stay after `--` | closed |
| T-01-10 | Tampering (data loss) | empty-intersection path | high | mitigate | Clean abort in the synchronous prepare phase with an explicit error BEFORE executeRestore's Stop/Remove; TestRestoreEmptyIntersection asserts the fake Docker never stops/removes (restore_selection_test.go:254) | closed |
| T-01-11 | Information Disclosure | skip notes in run record + logs | medium | mitigate | paths→`[path]` scrub FIRST (package-duplicated regexes), bounded note length (truncateErr); status/reason text carries no raw host paths; scrubError on every failEnvelope (handlers.go:61,71) | closed |
| T-01-12 | Denial of Service | mapping over huge stored lists | low | accept | Stored lists are user-created selections bounded by the 1 MiB PATCH body; mapping is O(stored × snapshotPaths) with small constants | closed |
| T-01-13 | Tampering (user intent) | PATCH empty selection | high | mitigate | Tree-source empty list over a non-empty prior selection refused with code "empty-selection" (handlers.go:1153); prior state preserved on refusal; legacy path byte-compat tested | closed |
| T-01-14 | Tampering | selectionSource spoofing/unknown values | low | mitigate | Only the literal "tree" carries meaning; every other value ignored (handlers.go:1114); pointer field keeps omission legal | closed |
| T-01-15 | Information Disclosure | mounts excluded field | low | mitigate | Exclusions rendered via the existing toHostPath translation (inverse of toContainerPath, service.go:1132-1189); no raw container-internal layout introduced | closed |
| T-01-05-01 | Tampering (glob injection) | excludedBranches → BackupDeps.Excludes → BackupArgs | low | accept | Stored entries are path-validated at save time (mount-root contained, cleaned) but may hold restic glob metacharacters — the same semantics user-written exclude patterns already have on this authenticated single-admin surface; blast radius bounded by positional roots; documented in the helper's doc comment (selection.go:141) | closed |
| T-01-05-02 | Repudiation (advertisement mismatch — THE gap) | SetBackupPaths advertisement vs backup engine | high | mitigate | Stored exclusion branch enforced content-wise on the backup argv (selection.go:185 excludedBranches, service.go:4243 single wiring site); tracer test pins positionals + excludes together; real-restic contract test proves the pattern form on the 0.17 floor (3/3 PASS) | closed |
| T-01-05-03 | Tampering (restore contamination) | prepareRestoreForTarget / mapRestorePaths / restore argv builders | high | mitigate | Derivation referenced only by the backup construction site and its tests (file-level enumeration gate); restore mapping input remains tg.AppdataPaths, pinned excludes-free by the revised tracer test; internal/backup diff-empty (scope guard, 01-VERIFICATION.md) | closed |
| T-01-05-04 | Information disclosure (paths in snapshot metadata) | snapshot Excludes metadata | low | accept | Derived absolute container-form paths persist in snapshot metadata readable by anyone with repo access — same exposure class as positionals in snapshot Paths and user exclude patterns, both already absolute in metadata | closed |
| T-01-SC (plans 01-01→01-04) | Tampering | package installs | low | accept | No package installs across the four plans — Go stdlib + in-repo code only (os.Root is Go 1.24+ stdlib); RESEARCH package-legitimacy audit: nothing to audit | closed |
| T-01-05-SC | Tampering | package installs (gap closure) | high→n/a | accept | No package installs in the gap closure (zero dependencies; Go stdlib + existing CI-pinned restic 0.17.3), so the supply-chain gate has nothing to gate | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01-04 | T-01-04 | Same-subject disclosure: error echoes the caller's own submitted path, already scrubbed in envelopes | Plan-time register (gsd-planner), reviewed at audit | 2026-09-10 |
| AR-01-12 | T-01-12 | Mapping cost bounded by user-created list size (1 MiB PATCH cap) — no amplification vector | Plan-time register (gsd-planner), reviewed at audit | 2026-09-10 |
| AR-01-05-01 | T-01-05-01 | Glob semantics of stored exclusion entries equal those of user-written exclude patterns on the same authenticated single-admin surface; positional roots bound the blast radius | Plan-time register (gsd-planner), reviewed at audit | 2026-09-10 |
| AR-01-05-04 | T-01-05-04 | Snapshot metadata exposure unchanged in class: restic already records absolute positionals and user patterns there | Plan-time register (gsd-planner), reviewed at audit | 2026-09-10 |
| AR-01-SC | T-01-SC, T-01-05-SC | Zero package installs across all five plans — supply-chain gate has no surface | Plan-time register (gsd-planner), reviewed at audit | 2026-09-10 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-10 | 21 | 21 | 0 | gsd-secure-phase orchestrator (L1 grep-depth, ASVS L1 short-circuit; corroborated by full green suite, 10/10 re-verification, 19-file code review with WR-01 fixed at 04363500) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-10
