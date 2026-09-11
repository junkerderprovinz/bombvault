---
phase: 3
slug: selection-trust-controls
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-09-10
---

# Phase 3 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Register authored at plan time (all three PLANs carry a `<threat_model>` block);
> verified post-execution at L1 grep-depth per the ASVS level 1 short-circuit
> (threats_open: 0, register_authored_at_plan_time: true, asvs_level: 1 — no
> auditor spawn required).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| client to API | untrusted JSON body on PATCH /api/containers/{name} (existing authed/CSRF endpoint) | excludeCaches map keys (host paths), backupPaths list |
| API to SQLite | trusted internal store calls | validated settings fields |
| service to restic child | argv constructed only by typed builders; no user string reaches argv through this feature | boolean union → constant `--exclude-caches` flag |
| API responses to SPA render | server-served host paths rendered as text in the tree | mounts, exclusions, excludeCaches map, lastBackup |
| user actions to PATCH bodies | reset and toggle mutations cross into authenticated writes on the existing endpoint | composed single PATCH body per queue drain |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-03-01 | Tampering | PATCH excludeCaches map keys | high | mitigate | every key passes the `toContainerPath` containment validation (`internal/api/service.go:10143`, path.Clean resolves `..`, `!ok` rejects); the whole save rejects atomically before any write; keys never reach argv, only the boolean union emits the constant flag | closed |
| T-03-02 | DoS | oversized excludeCaches map | medium | mitigate | existing 1 MiB MaxBytesReader + typed `map[string]bool` decode (non-bool fails at unmarshal) + `maxExcludeCachesEntries` 64-entry cap (`service.go` SetExcludeCaches) | closed |
| T-03-03 | Tampering (intent integrity) | silent backup-scope change | high | mitigate | flag effect disclosed in the UI tooltip (`folders.cachedirScope` InfoBubble, `SelectionTree.tsx:540`); argv presence/absence pinned (`internal/restic/restic.go:398`, `restic_args_test.go:396`) + item-level union service test | closed |
| T-03-04 | Spoofing | CSRF on the new PATCH field | medium | accept | the field rides the existing authed and CSRF-protected endpoint; no new route is added | closed |
| T-03-05 | Tampering (XSS) | rendered relative exclusion paths | medium | mitigate | React text-node auto-escaping; plain-text render with the house mono ltr pattern + `title` attribute (`SelectionTree.tsx:253,272,299`); zero `dangerouslySetInnerHTML` in the file | closed |
| T-03-06 | Tampering (intent integrity) | preview drifting from argv truth | high | mitigate | `rootIncludeCount` agreement tests against `toFlatList` (`selectionTree.ts:184`); the count never derives from screen state or existence flags | closed |
| T-03-07 | Tampering | concurrent PATCH lost-update from one editor | medium | mitigate | the generalized one-deep queue serializes every container PATCH from FoldersEditor into one fetch per drain (`Containers.tsx:836-1011`); dom test pins no overlapping fetches via maxConcurrent (`Containers.tree.dom.test.tsx:37-38`) | closed |
| T-03-08 | Tampering (intent integrity) | silent deselect into auto-detection | high | mitigate | reset is explicit and confirmed with both consequences named (fail-tone confirm `Containers.tsx:1282`, dom test :584); guard message teaches it; reset body pin keeps the tree guard live for toggles; narrowing announced (`Containers.tsx:1386`) | closed |
| T-03-09 | Spoofing | CSRF on PATCH fields | medium | accept | existing CSRF middleware on the unchanged endpoint; no new route | closed |
| T-03-10 | Information disclosure | PATCH failure text | low | accept | server errors already scrubbed (`scrubError`) before the envelope; the SPA shows them verbatim per house rule | closed |
| T-03-SC | Tampering | package installs | low | accept | zero npm and Go installs this phase (03-RESEARCH.md Package Legitimacy Audit: none; recorded in all three plans) | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-03-01 | T-03-04 | new PATCH field rides the existing authed/CSRF-protected endpoint; no new route | plan 03-01 (threat model) | 2026-09-10 |
| AR-03-02 | T-03-09 | existing CSRF middleware on the unchanged endpoint | plan 03-03 (threat model) | 2026-09-10 |
| AR-03-03 | T-03-10 | server errors already scrubbed before the envelope; verbatim display is the house rule | plan 03-03 (threat model) | 2026-09-10 |
| AR-03-04 | T-03-SC | zero package installs this phase; no legitimacy checkpoint applies | plans 03-01/02/03 (threat models) | 2026-09-10 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-10 | 11 | 11 | 0 | gsd orchestrator (L1 grep-depth short-circuit, ASVS 1 — no auditor spawned) |

Verification evidence gathered post-review-fix (HEAD f13963af): containment and
cap confirmed in `SetExcludeCaches`; argv flag + tests confirmed in
`internal/restic`; queue serialization + no-overlap dom pin confirmed in
`Containers.tsx` / `Containers.tree.dom.test.tsx` (including the WR-01..WR-04
fix commits 6aa9f24f, a5be53f2, daee9390, 4976fa07 which hardened T-03-07 and
T-03-08 mitigations: sticky reset descriptor, queue-tail refetch, orphaned
excludeCaches keys cleared on reset).

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-10
