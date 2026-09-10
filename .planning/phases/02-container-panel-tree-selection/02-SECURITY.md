---
phase: 02
slug: container-panel-tree-selection
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-09-10
---

# Phase 02 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

> Register merged from the three `<threat_model>` blocks (02-01→02-03-PLAN.md, all authored at plan time).
> Verification depth: L1 grep-level classification (ASVS L1 short-circuit — threats_open 0, register authored
> at plan time), corroborated by the full green vitest suite (unit + dom + keyboard + integration + i18n
> across 42 locales), the code-review fixes WR-01..WR-04 (42af7305, 00e76da3, 25389f2b, fab1b61c),
> the 10/10 verification (02-VERIFICATION.md), and the 5/5 UAT on the live Unraid instance (02-UAT.md).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| SPA → /api/browse + PATCH /api/containers/{name} | Existing authed/CSRF-protected routes; the tree builds PATCH bodies and browse queries from strings the server itself served | `backupPaths` flat list (prefixed exclusions, bare includes) + `selectionSource`; derived from served data |
| Server listings → rendered DOM | Directory names are external data rendered into the panel | Entry names, status kinds, scrubbed error text |
| Server error/status payloads → rendered rows | Browse error strings and the coded envelope cross into the DOM | Error text (already server-scrubbed), coded refusal kinds |
| Client guard → PATCH boundary | The D-04 block is the only guard on the exclusions-only save path (the server guard fires only on fully-empty lists) | Toggle intents; must never produce a zero-include PATCH |
| localStorage expansion key → focus/expansion model | Parsed JSON drives which nodes render expanded (never what is selected or fetched beyond cached paths) | Expansion hints; comfort state only |
| Keyboard events → selection toggles | Space is an alternative input path into the same PATCH boundary as clicks | Key events routed through the shared onToggle pipeline |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-02-01 | Tampering | PATCH body construction (SelectionTree onToggle → setBackupPaths) | high | mitigate | Every toggled entry derives from server-served paths only (mounts response, browse dirs); no free-text path entry in the tree; host-form translation is prefix arithmetic over the two served roots. Verified: Containers.tsx:851 sends `toFlatList(live.inc, live.exc)` with `selectionSource: "tree"` from the live mirror; Phase 1 containment re-validation (toContainerPath + os.Root, T-01-01) is the inherited backstop | closed |
| T-02-02 | Tampering (user intent) | optimistic mirror vs server truth (FoldersEditor persist) | medium | mitigate | Single selection truth: every toggle round-trips the full flat list (D-03, Containers.tsx:703, 851); failure reverts via `revertFrom(desc)` (Containers.tsx:860-864) and shakes (rowShake, Containers.tsx:646-681); nothing derived from localStorage ever reaches backupPaths | closed |
| T-02-03 | Information Disclosure | directory-name rendering (SelectionTree rows) | low | mitigate | React text-node auto-escaping; zero `dangerouslySetInnerHTML` in web/src (grep clean); names render like every existing path; browse error text is server-scrubbed and shown verbatim | closed |
| T-02-04 | Tampering | localStorage expansion key (loadExpanded/saveExpanded) | low | mitigate | Guarded JSON parse (try/catch, isArray, string-only filter) and capped writes (`slice(-MAX_EXPANDED)`, 64) — selectionTree.ts:315-340; cap 64 pinned through the component path (Containers.tree.dom.test.tsx:263-305); parsed values are expansion hints only, never paths to fetch or PATCH (selectionTree.ts:27-29) | closed |
| T-02-SC | Tampering | package installs (plans 02-01, 02-03) | low | accept | Zero npm installs this phase — `git diff package.json package-lock.json` empty across all Phase 2 commits; npm ci reinstalls the committed lockfile only | closed |
| T-02-05 | Tampering (user intent) | D-04 guard bypass (exclusions-only explicit-none) | high | mitigate | The guard runs BEFORE the mirror apply, busy flag, and queue: a toggle that would leave ZERO includes for the whole item never PATCHes (Containers.tsx:901-912, early return before any request); the server coded refusal is handled defensively (toast verbatim + revert); dom tests pin that no request is sent | closed |
| T-02-06 | Tampering (user intent) | revert race clobbering newer toggles | medium | mitigate | One-deep queue `queueRef {inFlight, dirty}` (Containers.tsx:774); a mutation during flight marks dirty and rides the next drain, which sends only the LATEST full flat list (Containers.tsx:846-873); revert re-derives from the live mirror via the desc recipe (Containers.tsx:711-712, 775-776); pinned by the two-toggle failing-first-save dom test | closed |
| T-02-07 | Information Disclosure | error/status text rendering (no-access rows) | low | accept | Server text is already scrubbed (paths then credentials, Phase 1) and shown verbatim per house rule; the client adds no path formatting of its own | closed |
| T-02-08 | DoS (client) | rapid-toggle PATCH storms | low | mitigate | The dirty-flag queue collapses bursts to a single draining request per in-flight save (Containers.tsx:761-767, T-02-08 cited at line 767); the debounce was observed live in UAT test 3 (one PATCH after a burst of toggles) | closed |
| T-02-09 | Tampering | localStorage-parsed expansion entries | low | mitigate | Same guarded parse as T-02-04 (selectionTree.ts:315-327); entries only match rendered node host paths; unknown entries are inert — never fetched, never PATCHed, never promoted into the selection mirror | closed |
| T-02-10 | Tampering (user intent) | keyboard Space path bypassing the D-04 block | medium | mitigate | Space routes through the identical onToggle pipeline as checkbox clicks — one toggle semantics, one guard, one queue (SelectionTree.tsx:58, 362-368; checkbox onChange at 446); the keyboard test asserts Space twice returns the prior flat list and the integration test pins the no-call block | closed |
| T-02-11 | Information Disclosure | absorbed sub-include rendering (paths from mounts response) | low | accept | Paths are server-served host paths already shown in the panel today; rendering moves them between two views of the same response, no new data source | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-02-07 | T-02-07 | Server error text is already scrubbed (paths then credentials, Phase 1 scrubError order) and shown verbatim per the house rule; the client adds no path formatting of its own | Plan-time register (gsd-planner), reviewed at audit | 2026-09-10 |
| AR-02-11 | T-02-11 | Absorbed sub-include paths come from the same mounts response already rendered in the panel; moving them between two views of one response introduces no new data source | Plan-time register (gsd-planner), reviewed at audit | 2026-09-10 |
| AR-02-SC | T-02-SC (plans 02-01, 02-03) | Zero package installs across all three plans — package.json/package-lock.json diff empty over the phase commits; supply-chain gate has no surface | Plan-time register (gsd-planner), reviewed at audit | 2026-09-10 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-10 | 12 | 12 | 0 | gsd-secure-phase orchestrator (L1 grep-depth, ASVS L1 short-circuit; corroborated by the full green vitest suite, code-review fixes WR-01..WR-04 at 42af7305/00e76da3/25389f2b/fab1b61c, 10/10 verification, and 5/5 UAT on the live Unraid instance) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-10
