---
phase: 02-container-panel-tree-selection
reviewed: 2026-09-10T15:21:11Z
depth: standard
files_reviewed: 49
files_reviewed_list:
  - web/src/components/SelectionTree.dom.test.tsx
  - web/src/components/SelectionTree.keyboard.dom.test.tsx
  - web/src/components/SelectionTree.tsx
  - web/src/lib/api.ts
  - web/src/lib/i18n.ts
  - web/src/lib/locales/ar.ts
  - web/src/lib/locales/bg.ts
  - web/src/lib/locales/ca.ts
  - web/src/lib/locales/cs.ts
  - web/src/lib/locales/da.ts
  - web/src/lib/locales/el.ts
  - web/src/lib/locales/es.ts
  - web/src/lib/locales/et.ts
  - web/src/lib/locales/eu.ts
  - web/src/lib/locales/fa.ts
  - web/src/lib/locales/fi.ts
  - web/src/lib/locales/fr.ts
  - web/src/lib/locales/gl.ts
  - web/src/lib/locales/he.ts
  - web/src/lib/locales/hi.ts
  - web/src/lib/locales/hr.ts
  - web/src/lib/locales/hu.ts
  - web/src/lib/locales/id.ts
  - web/src/lib/locales/is.ts
  - web/src/lib/locales/it.ts
  - web/src/lib/locales/ja.ts
  - web/src/lib/locales/ko.ts
  - web/src/lib/locales/lt.ts
  - web/src/lib/locales/lv.ts
  - web/src/lib/locales/ms.ts
  - web/src/lib/locales/nl.ts
  - web/src/lib/locales/no.ts
  - web/src/lib/locales/pl.ts
  - web/src/lib/locales/pt.ts
  - web/src/lib/locales/ro.ts
  - web/src/lib/locales/ru.ts
  - web/src/lib/locales/sk.ts
  - web/src/lib/locales/sl.ts
  - web/src/lib/locales/sr.ts
  - web/src/lib/locales/sv.ts
  - web/src/lib/locales/th.ts
  - web/src/lib/locales/tr.ts
  - web/src/lib/locales/uk.ts
  - web/src/lib/locales/vi.ts
  - web/src/lib/locales/zh.ts
  - web/src/lib/selectionTree.test.ts
  - web/src/lib/selectionTree.ts
  - web/src/pages/Containers.tree.dom.test.tsx
  - web/src/pages/Containers.tsx
findings:
  critical: 1
  warning: 4
  info: 1
  total: 6
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-09-10T15:21:11Z
**Depth:** standard
**Files Reviewed:** 49 (40 locale files reviewed as a group; 9 load-bearing files read in full or contract-verified)
**Status:** issues_found

## Narrative Findings (AI reviewer)

### Summary

Reviewed the Phase 02 container-panel tree-selection implementation: the pure reducer (`selectionTree.ts`), the tree component (`SelectionTree.tsx`), the editor mirror/save queue in `Containers.tsx` (FoldersEditor), the four test files, the i18n tables, the API client contract, and the 40 locale files as a group (grep parity confirmed every locale carries all four new `folders.*` keys: `treeLabel`, `truncatedList`, `retry`, `emptySelectionBlocked`; no em dashes in the new en strings; en/de spot-read).

Overall this is careful, well-documented code. Cross-module verification performed against the Go side: the browse path translation (`hostToBrowseRel`/`browseRelToHost` against `hostSourceRoot`) is CORRECT — `internal/api/service.go:1132-1148` (`toContainerPath`) and `service.go:3511-3515` prove the whole `HostSourceRoot` (`/mnt`) is bind-mounted at `HostMountRoot` (`/host/user`, so host `/mnt/user/x` = container `/host/user/user/x`), which means HostMountRoot-relative browse paths equal HostSourceRoot-relative paths. A suspected path-space mismatch was investigated and refuted.

However, one confirmed behavioral bug ships in the core reducer (CR-01: a reachable toggle that is a silent no-op yet still fires a PATCH and a "Saved" toast), plus two accessibility contract violations in the tree and two defects in the custom-path add flow.

Verified sound (no findings): queue serialization with set-difference failure revert (Pitfall 5 handled correctly); D-04 empty-selection guard incl. whole-item counting; keyboard map and its target guard (Space on inner controls cannot bypass onToggle); lazy fetch with rejection/ok:false cache eviction; expansion persistence cap and storage guards; segment-aligned prefix arithmetic throughout; orphan-exclusion dormancy; all i18n keys used by the new UI exist in en/de; the four test suites are reliable (no flaky patterns, deferred promises used correctly, mocks scoped per test).

### Critical Issues

#### CR-01: `applyToggle` "mixed" branch is a silent no-op for the carve-out flavor — dead checkbox plus spurious PATCH and "Saved" toast

**File:** `web/src/lib/selectionTree.ts:262-268` (defect), `web/src/pages/Containers.tsx:907-925` (consequence)
**Issue:** The "mixed" case only deletes includes STRICTLY BELOW the node. But `classifyNode` returns "mixed" in two flavors: (a) an include strictly below (whitelist start-state) — handled; (b) an ANCESTOR include applying with an exclusion strictly below (carve-out) — NOT handled. In flavor (b) there is no include at or under the node, so the loop deletes nothing and the returned sets are identical to the input.

Repro (fully reachable in the UI): mount `/mnt/user/appdata/plex` selected whole (`I={"/mnt/user/appdata/plex"}`); expand → `library` → expand → uncheck `library/tmp` (`E={"/mnt/user/appdata/plex/library/tmp"}`). Node `/mnt/user/appdata/plex/library` now classifies "mixed". Click its checkbox: `includes.has(node)` is false, the mixed branch drops zero entries, and `Containers.tsx onToggle` proceeds because `next.includes.size !== 0` — it re-applies an unchanged mirror, disables the row's checkbox while a PATCH of the IDENTICAL flat list goes out, and toasts "Saved" on success. The user clicked a checked-looking (indeterminate) box and absolutely nothing changed, with a success toast claiming something did.

No test covers this flavor: `selectionTree.test.ts:133-137` ("mixed node loses own and strictly-below includes") and the D-01 cycle test at `:165-182` both toggle nodes that carry an OWN include, which routes through the first `includes.has(node)` branch, never the mixed case. The dispatch-table doc at `selectionTree.ts:222-224` states assumption "A2" (mixed without own include implies includes below) — that assumption is false for the carve-out flavor.

**Fix:** Handle the flavor in the mixed branch (carve-out deselect, consistent with the "checked via ancestor" branch and the rendered checked box; dropping the strictly-below exclusions for select-all is an equally valid product choice — the bug is the no-op, not the direction):

```typescript
case "mixed": {
  let dropped = false;
  for (const i of includes) {
    if (isStrictlyUnder(i, node)) {
      next.includes.delete(i);
      dropped = true;
    }
  }
  if (!dropped) {
    // Carve-out flavor (ancestor include applies, exclusion strictly below,
    // nothing of ours below to deselect): the click is a deselect of this
    // branch, same as "checked via ancestor only".
    next.exclusions.add(node);
  }
  return next;
}
```

And add a defense-in-depth no-op guard in `Containers.tsx onToggle` so a reducer no-op can never produce a save/toast again:

```typescript
const unchanged =
  next.includes.size === pre.includes.size && [...next.includes].every((p) => pre.includes.has(p)) &&
  next.exclusions.size === pre.exclusions.size && [...next.exclusions].every((p) => pre.exclusions.has(p));
if (unchanged) return;
```

Add a table case pinning the flavor: `before {includes:["/a"], exclusions:["/a/b/c"]}, node "/a/b"` → expect a state change.

### Warnings

#### WR-01: Non-treeitem children inside `role="tree"` / `role="group"` violate the APG tree structure the file claims to implement

**File:** `web/src/components/SelectionTree.tsx:447-451` (blockedPath `<p>` as a direct child of `role="tree"`), `453-493` (loading/empty/error/truncated rows as plain `div`/`p` children of `role="group"`)
**Issue:** APG TreeView requires child elements of `tree`/`group` to be `treeitem` (or `group`); any wrapper must carry `role="presentation"`. The blocked warn line is a `<p>` directly under the tree root, and the four notice rows sit unroled inside each `role="group"`. Screen readers in tree mode may misannounce or skip these rows; the component's own header (lines 34-54) leans on APG compliance for the keyboard map, so the structure should match the same spec.
**Fix:** Wrap each notice row (and the blockedPath `<p>`) in a `role="presentation"` container — the presentation role removes the wrapper from the structural contract while its text content stays exposed to assistive tech, so the notices remain announced without breaking the tree's child contract.

#### WR-02: Mixed nodes announce contradictory state — inner checkbox says "checked", treeitem says "mixed"

**File:** `web/src/components/SelectionTree.tsx:425-434` (`checked={state === "checked" || state === "mixed"}`), `386-388` (indeterminate via callback ref)
**Issue:** For a mixed node the treeitem carries `aria-checked="mixed"` but the nested real `<input type="checkbox">` renders `checked` (true) — and the DOM `indeterminate` property is not reflected in any ARIA state. Screen-reader users traversing into the row hear "checkbox checked" from the input contradicting "mixed" from the treeitem. The input is already `tabIndex={-1}` (keyboard selection routes through Space on the treeitem, T-02-10), so hiding it from the a11y tree loses nothing.
**Fix:** Add `aria-hidden="true"` to the checkbox input (keep `tabIndex={-1}`); mouse users keep clicking it, and `aria-checked` on the treeitem remains the single selection announcement.

#### WR-03: `addCustom` composes an uncleaned host path — the new partition filter then renders the row invisible and unreachable to remove

**File:** `web/src/pages/Containers.tsx:940-942` (composition), `985-990` (`customRows` filter)
**Issue:** `const p = raw.startsWith("/") ? raw : `${hostSourceRoot}/${raw}`` never passes `cleanPath`. A manually typed variant (`"appdata/plex/"`, `"appdata//plex"`, `"a/../appdata/plex"`) enters `custom` and `includes` uncleaned. The wire list is safe (`toFlatList` cleans), but in-memory behavior is not: the NEW `partitionCustomPaths` returns CLEANED paths while `customRows = custom.filter((c) => standaloneSet.has(c.path))` compares them against the RAW `c.path` — an uncleaned custom path fails the membership test and is dropped from the tree entirely: an invisible selected include (it still counts toward the D-04 floor and is saved), with no remove chip until a reload re-serves it cleaned. The duplicate guard (`custom.some(...) || includes.has(p)`, extended in this phase) also misses spelling variants of an already-added path.
**Fix:** Compose through the existing exported translator, which cleans both directions:
```typescript
const p = browseRelToHost(raw, hostSourceRoot); // prefixes rel paths, cleans, absolute pass-through
```

#### WR-04: `addCustom` clears the staged pick before the duplicate early-return — silent no-op that destroys input

**File:** `web/src/pages/Containers.tsx:941-942`
**Issue:** `setBrowseValue("")` executes BEFORE `if (custom.some((c) => c.path === p) || includes.has(p)) return;`. Adding an already-present path silently wipes the FolderBrowser input and does nothing else — no toast, no shake, no row change. The user loses their staged pick with zero feedback. (The `setBrowseValue` placement is pre-existing, but this phase extended exactly this guard and left the ordering defect beside it.)
**Fix:** Move `setBrowseValue("")` after the guard passes, or keep the input and surface a toast for the duplicate case:
```typescript
if (custom.some((c) => c.path === p) || includes.has(p)) {
  push(t("folders.duplicatePath"), "fail"); // or a neutral existing key
  return;
}
setBrowseValue("");
```

### Info

#### IN-01: `aria-setsize` reports the capped count under truncation

**File:** `web/src/components/SelectionTree.tsx:212-213`
**Issue:** `setSize: listing.dirs.length` announces "of 500" when the listing was truncated at the server cap, though more entries exist. The truncated notice row does convey the cap, so this is minor announcement inaccuracy, not a correctness bug.
**Fix:** Either leave as-is (the rendered set IS the announced set) or omit `aria-setsize`/`aria-posinset` on children of a truncated listing; document the choice next to the D-06 notice rendering.

---

_Reviewed: 2026-09-10T15:21:11Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
