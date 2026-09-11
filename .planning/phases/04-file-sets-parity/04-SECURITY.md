---
phase: 4
slug: file-sets-parity
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-09-11
---

# Phase 4 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| store → compile → restic | stored `selected_paths` entries become restic positionals; entries are boundary-validated at write time, and the compile re-anchors defensively because the anchor (`set.Path`) is user-mutable | user-influenced path strings → argv positionals |
| client → API | untrusted `selectedPaths` entries and path edits cross into `handlePatchFileSet` on the existing authed + CSRF-protected endpoint | JSON body (1 MiB-capped, DisallowUnknownFields) |
| API → SQLite | validated canonical list only; the owned setter is the sole write path | normalized flat selection list |
| API → restore | snapshot selection crosses into destructive in-place restore; the D-08 guard runs before any work | snapshot Paths |
| SPA → API | the editor PATCHes through the plan-02 boundary; server validation is the trust anchor, the client refusal (D-06) is UX only | selection PATCH bodies |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-04-01 | Tampering | stale `selected_paths` after a set Path edit (silent backup-scope broadening) | high | mitigate | compile-time re-anchor `fileSetPositionals` against THIS run's freshly resolved root (`internal/api/service.go:8769-8770`); pinned by the stale-root re-anchor subtest (`internal/api/service_test.go:427-468`); PATCH-time clear is layer 1 (T-04-05) | closed |
| T-04-02 | Tampering | entries reaching argv as flags instead of positionals | high | mitigate | entries flow only through `FilesRestic.Backup` typed builder as `SourcePaths` positionals after `--` (`internal/backup/files_orchestrator.go:23-64`); no new argv construction | closed |
| T-04-03 | DoS | pathologically large stored selection inflating argv | low | mitigate | compile is a linear pure pass; write-side 64-entry cap (T-04-06) bounds it at the source | closed |
| T-04-04 | Tampering / Elevation | path traversal via selectedPaths entries ("..", absolute escapes, sibling-prefix confusion) | high | mitigate | `paths.Resolve` on the effective root (`internal/api/service.go:8967`) + segment-aligned `isStrictDescendant` per entry (`service.go:8982` — `/data/doc` never passes for `/data/docs`); refusal "not under the set's source folder"; atomic whole-save rejection; verified traversal-proof on both write and compile sides by the phase code review | closed |
| T-04-05 | Tampering | silent backup-scope change after a set Path edit orphans stored entries | high | mitigate | PATCH-time clear via the atomic single-statement `Repo.UpdateFileSetClearingSelection` (`internal/store/filesets.go:118`, post-review WR-01 fix `0529320c` — path change and selection clear are now one write), layered on plan-01's compile-time re-anchor (layer 2) | closed |
| T-04-06 | DoS | oversized selectedPaths payload / entry count | medium | mitigate | `maxFileSetSelectedPaths = 64` (`internal/api/service.go:8931`, enforced `:8960-8961`) + existing 1 MiB `MaxBytesReader` + `DisallowUnknownFields` (`internal/api/handlers.go:386`) | closed |
| T-04-07 | Information disclosure | raw paths leaking in refusal or abort error text | medium | mitigate | all failures route through the scrubbed envelope (`scrubError` — paths → `[path]` first, then credentials; `internal/api/handlers.go:61,71`) | closed |
| T-04-08 | Tampering (intent integrity) | empty selection silently stored, producing a no-op backup | high | mitigate | D-06 refusal client+server: server `code:"empty-selection"` (`internal/api/service.go:8810`, `internal/api/handlers.go:66`), client `files.emptySelectionBlocked` warn line (`web/src/pages/Files.tsx:1361`); never stored, never backed up | closed |
| T-04-09 | Spoofing | CSRF on the new PATCH field | medium | accept | the field rides the existing authed + CSRF-protected `handlePatchFileSet` endpoint; no new route | closed |
| T-04-10 | Tampering (intent integrity) | preview count drifting from the compiled argv | medium | mitigate | counts derive only from `rootIncludeCount` over the same (I, E) sets the compile consumes (`web/src/components/SelectionTree.tsx:303,327`); pinned against toFlatList membership | closed |
| T-04-11 | DoS | concurrent PATCHes from rapid toggles corrupting the stored selection | medium | mitigate | one-deep serialized PATCH queue over a ref mirror (`web/src/pages/Files.tsx:1173-1175`); `maxConcurrentPatches === 1` pinned across a burst; revert re-derived from the live mirror | closed |
| T-04-12 | Information disclosure | browse error text rendering raw paths | low | accept | the server already scrubs (paths → `[path]` first); the client renders the server's message verbatim per house rule, adding no new surface | closed |
| T-04-13 | Repudiation | selection state hidden in localStorage contradicting server truth | low | mitigate | zero `localStorage` references in `Files.tsx`; selection never persists client-side — only expansion comfort state keyed per set (`bv-tree-expanded-{name}`, `web/src/components/SelectionTree.tsx:87`); pinned by test | closed |
| T-04-14 | Tampering (intent integrity) | the exclusions audit list becoming a second mutation path bypassing the tree's normalization | high | mitigate | the `rootExclusions` list renders zero interactive controls (`web/src/pages/Files.tsx:1084`); mutations flow only through the one toggle pipeline; pinned by the no-controls dom tests | closed |
| T-04-15 | Repudiation | a path edit silently discarding a selection the user does not know about | medium | mitigate | `files.pathChangeHint` disclosed in the dialog BEFORE the edit (`web/src/pages/Files.tsx:1005`, rendered unconditionally); the clear rule itself is server-enforced | closed |
| T-04-SC | Tampering | package installs | low | accept | zero npm and Go installs this phase (04-RESEARCH.md Package Legitimacy Audit: none); no legitimacy checkpoint applies | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-04-01 | T-04-09 | The `selectedPaths` field rides the already authed + CSRF-protected PATCH endpoint; no new route or auth surface introduced | plan-time disposition (04-02-PLAN.md), verified 2026-09-11 | 2026-09-11 |
| AR-04-02 | T-04-12 | Browse error text renders the server's already-scrubbed message verbatim (house rule); client adds no new disclosure surface | plan-time disposition (04-03-PLAN.md), verified 2026-09-11 | 2026-09-11 |
| AR-04-03 | T-04-SC | Zero package installs this phase (04-RESEARCH.md Package Legitimacy Audit: none) | plan-time disposition (all plans), verified 2026-09-11 | 2026-09-11 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-11 | 16 | 16 | 0 | orchestrator (L1 grep + first-hand verification evidence: 04-VERIFICATION.md 12/12 must-haves incl. real-restic-0.17.3 Linux compile proof; 04-REVIEW.md backend core verified clean; post-review fixes d8fd491e + 0529320c) |

Register authored at plan time (all four PLAN.md files carry parseable `<threat_model>` blocks); verification performed at ASVS L1 grep depth with short-circuit per workflow (threats_open: 0, register complete, asvs_level 1). SUMMARY.md threat flags: none (no new network endpoints, auth paths, or trust-boundary surfaces in plans 03/04).

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-11
