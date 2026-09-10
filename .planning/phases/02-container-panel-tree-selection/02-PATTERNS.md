# Phase 2: Container Panel Tree Selection - Pattern Map

**Mapped:** 2026-09-10
**Files analyzed:** 8 (5 new + 3 modified, plus web/dist rebuild)
**Analogs found:** 8 / 8

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/src/lib/selectionTree.ts` (+ `.test.ts`) | utility (pure logic) | transform | `web/src/lib/controls.ts` / pure fns in `web/src/lib/*`; reducer semantics per RESEARCH Pattern 1 | role-match (no existing selection-logic module — semantics pinned by `internal/api/selection.go`) |
| `web/src/components/SelectionTree.tsx` | component | event-driven (lazy fetch-on-expand + live-save) | `web/src/components/FolderBrowser.tsx` (browse fetch/states) + `web/src/components/SnapshotFileTree.tsx` (tree rows/chevron) | exact (composite) |
| `web/src/components/SelectionTree.dom.test.tsx` | test | — | `web/src/pages/Containers.excludesAssistant.dom.test.tsx` (vi.mock api harness) | exact |
| `web/src/components/SelectionTree.keyboard.dom.test.tsx` | test | — | `web/src/components/DropdownListbox.keyboard.dom.test.tsx` | exact |
| `web/src/pages/Containers.tree.dom.test.tsx` (name per planner) | test | — | `Containers.excludesAssistant.dom.test.tsx` (FoldersEditor import-after-mock harness) | exact |
| `web/src/pages/Containers.tsx` (modified: FoldersEditor) | component/page | request-response (live-save PATCH) | itself — `FoldersEditor` at lines 696-929 is the base being evolved | exact |
| `web/src/lib/api.ts` (modified) | api client types | request-response | itself — existing `BrowseResponse`/`setBackupPaths` blocks | exact |
| `web/src/lib/i18n.ts` + 40 locale files (modified) | config (i18n) | — | existing `folders.*`/`folder.*` keys in `i18n.ts` (en block ~line 728, de block ~line 2435); locale files in `web/src/lib/locales/` | exact |

No Go files this phase. `web/dist` rebuild is a build step, not a pattern.

## Pattern Assignments

### `web/src/lib/selectionTree.ts` (utility, transform)

**Analog:** none in `web/src/lib` carries selection semantics — the SEMANTIC source of truth is `internal/api/selection.go` (Go, Phase 1). The TS module must mirror its prefix/pruning rules segment-aligned (RESEARCH Pattern 1 sketch is the pinned classifier shape: excluded/checked/mixed/unchecked from `I` and `E` sets only). For file shape/style, follow the codebase's pure-lib convention: named exports, narrative header comment, table tests in a sibling `.test.ts` (node env — NO jsdom pragma; e.g. `web/src/lib/i18n.parity.test.ts`, `web/src/lib/uiConventions.test.ts`).
**Path translation precedent** — `web/src/pages/Containers.tsx:799-802` (verbatim):
```typescript
// The folder picker yields a path relative to the host mount; translate it to
// the host path SetBackupPaths expects. An already-absolute path (manual
// fallback) is used as-is.
const p = raw.startsWith("/") ? raw : `${hostSourceRoot}/${raw}`;
```
**localStorage pattern** — `web/src/lib/displayPrefs.ts` shows the house `bv-*` discipline: try/catch around every `localStorage.getItem/setItem` (private-window storage throws — see its `collect()` at lines 73-85), narrative comment explaining WHY the key exists and what it must never become (selection NEVER goes here — D-05). Key name per UI-SPEC: `bv-tree-expanded-{containerName}`, JSON array, cap 64, evict oldest.

---

### `web/src/components/SelectionTree.tsx` (component, event-driven lazy fetch + live-save)

**Analogs:** `FolderBrowser.tsx` (browse contract consumption + per-state rendering) and `SnapshotFileTree.tsx` (tree row layout + chevron).

**Browse fetch pattern** — `web/src/components/FolderBrowser.tsx:92-111` (verbatim — the tree's per-node fetch copies this shape; note `ok:false` is a distinct state, never `dirs ?? []`):
```typescript
const doFetch = useCallback((path: string) => {
  setLoading(true);
  setBrowseError(null);
  browse(path)
    .then((res) => {
      if (!res.ok) {
        setBrowseError(res.error ?? t("folder.couldNotRead"));
        setManualFallback(true);
        return;
      }
      setDirs(res.dirs ?? []);
      setBrowsePath(path);
    })
    .catch((err: unknown) => {
      const msg = err instanceof Error ? err.message : t("folder.browseFailed");
      setBrowseError(msg);
      setManualFallback(true);
    })
    .finally(() => setLoading(false));
}, [t]);
```

**Spinner + loading row** — `FolderBrowser.tsx:222-227` (verbatim; UI-SPEC pins this exact spinner for per-node loading):
```typescript
{loading && (
  <div className="flex items-center gap-2 text-xs text-carbon-textMuted">
    <span className="h-3 w-3 rounded-full border-2 border-accentText border-t-transparent animate-spin" />
    {t("folder.loading")}
  </div>
)}
```

**Empty-dir muted row** — `FolderBrowser.tsx:295-297`: `<p className="text-xs text-carbon-textMuted px-2">{t("folder.none")}</p>` — reuse this key/class for the expanded-empty child row; truncated notice (D-06) is the same shape with `folders.truncatedList`.

**Scroll region** — `FolderBrowser.tsx:275`: `h-[clamp(12rem,55vh,32rem)] overflow-y-auto` — the UI-SPEC pins this exact clamp around the whole tree.

**Tree row + chevron** — `web/src/components/SnapshotFileTree.tsx:103-148` (TreeRow). Copy the chevron SVG exactly (lines 120-128, 10×10 triangle, `transition-transform ${expanded ? "rotate-90" : "rtl:rotate-180"}`) and the logical-property indent (line 117 `style={{ paddingInlineStart: depth * 14 }}` — but use **16** per UI-SPEC, not 14), the checkbox with `style={{ accentColor: "var(--accent)" }}` (line 132-139), and `dir="ltr" font-mono ... truncate` + `title={node.path}` on the name span (line 141). DO NOT copy its `aria-label={expanded ? "collapse" : "expand"}` (line 123) — hard-coded untranslated, a lint violation; route through `t()`. Do not copy its emoji glyphs (lines 140) or its `hover:bg-carbon-hover` row treatment without checking the UI-SPEC accent reservation (hover is "no special treatment" per UI-SPEC pointer model).

**Mixed state (indeterminate)** — no precedent exists (first tri-state checkbox in the app, RESEARCH Pattern 5). Use native `<input type="checkbox">` with the `indeterminate` DOM property set in a callback ref/effect; checkbox semantics live on the treeitem's `aria-checked`, never duplicated on both.

**Keyboard map + ARIA** — APG TreeView (RESEARCH Pattern 4 / UI-SPEC contract); no in-repo tree keyboard precedent — implement per spec table.

---

### `web/src/pages/Containers.tsx` — FoldersEditor evolution (component, request-response live-save)

**Analog:** itself; the current implementation at lines 696-929 is the base.

**Live-save toggle with revert + shake** — `Containers.tsx:771-789` (verbatim; the tree toggle extends this with `selectionSource:"tree"`, re-derived revert instead of captured snapshot, and a serialized PATCH queue per RESEARCH Pitfall 5):
```typescript
async function toggle(source: string) {
  const wasChecked = checked.has(source);
  const next = new Set(checked);
  if (wasChecked) next.delete(source);
  else next.add(source);
  setChecked(next);
  setCheckRowBusy((b) => ({ ...b, [source]: true }));
  const ok = await persistPaths([...next, ...custom.map((c) => c.path)]);
  setCheckRowBusy((b) => ({ ...b, [source]: false }));
  if (!ok) {
    setChecked((prev) => { /* revert */ });
    setCheckRowShake((s) => ({ ...s, [source]: (s[source] ?? 0) + 1 }));
  }
}
```

**Shake-replay key technique** — `Containers.tsx:843-844` (verbatim — row key includes the shake nonce so `.glim-shake` remounts/replays; reuse for tree rows keyed by host path):
```typescript
key={`${m.source}-${checkRowShake[m.source] ?? 0}`}
className={`... ${checkRowShake[m.source] ? " glim-shake" : ""}`}
```

**Mount row markup** — `Containers.tsx:836-859`: the `<label>` row, `accent-(--accent)` checkbox, `font-mono break-all` `dest ← source` span, `statusOk`/`statusFail` text lines — these rows become level-1 treeitems (D-02) keeping this exact styling/text-tone vocabulary (UI-SPEC color table maps excluded → `text-carbon-textMuted`, unchecked → `text-carbon-textSub`).

**Panel state survives close** — `Containers.tsx:818` (`if (!open) return null;`) — hold (I, E) mirror, listings cache, and translation roots in FoldersEditor-level state ABOVE the null return so they survive close/reopen (RESEARCH Pitfall 3); expansion restored from localStorage on tree mount.

**Lazy load-on-first-open effect** — `Containers.tsx:721-744` (`if (!open || loaded) return;` + `getContainerMounts` then/finally) — extend to also read `r.excluded` into `E` and derive `I`.

---

### `web/src/lib/api.ts` (modified — Wave 0)

**Analog:** itself. Current shapes to extend verbatim (lines 475-484 and 858-876, quoted in full in RESEARCH Pitfall 2): add `status?: "ok" | "restricted" | "missing" | "error"` and `truncated?: boolean` to `BrowseResponse`; `excluded?: string[]` to `ContainerMountsResponse`; an optional `selectionSource` param on `setBackupPaths`; optionally `code?: string` on the envelope. Keep the per-field doc comments (house rule: TS wire types mirror Go JSON with a doc comment per field). Backend truth: `internal/api/handlers.go:4328-4335` (browse response fields), `handlers.go:1337-1343` (mounts response), `handlers.go:1112-1123` (PATCH body incl. `selectionSource`).

---

### `web/src/lib/i18n.ts` + 40 locale files (modified)

**Analog:** existing `folders.*` block — `i18n.ts` line 728 (en: `"folders.add": "Add"`, ...) and line 2435 (de block). Exactly 4 new keys: `folders.treeLabel`, `folders.truncatedList`, `folders.retry`, `folders.emptySelectionBlocked` (copy + German drafts pinned in UI-SPEC Copywriting/i18n Budget). Add to en + de inline AND all 40 files in `web/src/lib/locales/` — `i18n.parity.test.ts` fails on any divergence (precedent commit fb6400ce: 2 keys touched all 40 files). No em dashes in user text.

---

### Test files

**vi.mock api harness** — `web/src/pages/Containers.excludesAssistant.dom.test.tsx:1, 25-63` (verbatim shape): `// @vitest-environment jsdom` first line; `vi.mock("../lib/api", async (importOriginal) => ({...actual, browse: ..., setBackupPaths: ...}))`; component imported AFTER vi.mock via `const { FoldersEditor } = await import("./Containers")`; Harness wrapped in `<I18nProvider><ToastProvider>`. Use for `SelectionTree.dom.test.tsx` and the FoldersEditor integration test.

**Keyboard test harness** — `web/src/components/DropdownListbox.keyboard.dom.test.tsx` (full file, 126 lines): `beforeEach` stubs `Element.prototype.scrollIntoView` (lines 50-54, mandatory — jsdom lacks it); `document.activeElement` assertions; `await act(async () => { fireEvent.keyDown(...) })` for every key event. File-per-concern naming: `SelectionTree.keyboard.dom.test.tsx`.

**Pure unit test** — `selectionTree.test.ts` runs in node env: NO jsdom pragma (contrast `i18n.parity.test.ts`).

## Shared Patterns

### Live-save (optimistic + revert + shake)
**Source:** `Containers.tsx:751-789` (`persistPaths`/`toggle`) — every tree toggle funnels through one persist fn; extend with serialized one-deep PATCH queue (Pitfall 5) and `selectionSource:"tree"`.
**Apply to:** FoldersEditor integration + SelectionTree toggle handler.

### Error display (scrubbed server text verbatim, distinct from empty)
**Source:** `FolderBrowser.tsx:97-98` (`res.error ?? t("folder.couldNotRead")`) + `FolderBrowser.tsx:204` inline error paragraph.
**Apply to:** per-node no-access row (TREE-06); never toast a listing failure (see FolderBrowser's own comment at lines 80-85 on why list-load errors stay inline).

### i18n discipline
**Source:** `t()` from `useT()` everywhere; `T` type alias `type T = ReturnType<typeof useT>["t"]` (used in FoldersEditor signature, Containers.tsx:696).
**Apply to:** every user-visible string in SelectionTree — including aria-labels (SnapshotFileTree's untranslated chevron label is the recorded anti-precedent).

### Component file shape
**Source:** `FolderBrowser.tsx:1-58` — narrative header comment, named export, props interface with per-prop doc comments, relative imports.
**Apply to:** `SelectionTree.tsx` (lives in `components/`, not `pages/` — Phase 4 reuse).

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `selectionTree.ts` classifier/reducer core | utility | transform | No selection-semantics module exists SPA-side; semantics MUST mirror `internal/api/selection.go` (segment-aligned prefix tests; RESEARCH Pattern 1 sketch is the pinned shape) |
| Tri-state `indeterminate` checkbox | component | — | First in the app (grep confirms zero `indeterminate` usage in web/src) — use DOM property in callback ref per RESEARCH Pattern 5 |
| APG TreeView keyboard/roving tabindex | component | event-driven | No tree keyboard precedent; follow APG + DropdownListbox test harness |

## Metadata

**Analog search scope:** `web/src/components/`, `web/src/pages/`, `web/src/lib/`, `internal/api/` (contract truth only)
**Key analogs read:** Containers.tsx (FoldersEditor 600-929), FolderBrowser.tsx (full), SnapshotFileTree.tsx (80-215), DropdownListbox.keyboard.dom.test.tsx (full), Containers.excludesAssistant.dom.test.tsx (1-80), displayPrefs.ts (full), api.ts (460-509, 840-876), i18n.ts (key blocks)
**Pattern extraction date:** 2026-09-10
