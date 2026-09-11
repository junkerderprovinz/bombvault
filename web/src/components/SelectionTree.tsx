import { Fragment, useCallback, useEffect, useId, useRef, useState, type ReactNode } from "react";
import { useT } from "../lib/i18n";
import { browse, type BrowseDirEntry, type BrowseResponse, type CustomPath, type MountInfo } from "../lib/api";
import {
  browseRelToHost,
  classifyNode,
  hostToBrowseRel,
  isAtOrUnder,
  loadExpanded,
  rootExclusions,
  rootIncludeCount,
  saveExpanded,
} from "../lib/selectionTree";
import { Button } from "./Button";
import { InfoBubble } from "./InfoBubble";
import { Toggle } from "./Toggle";

// ---------------------------------------------------------------------------
// SelectionTree — the container panel's backup-folder tree (Phase 2, D-02:
// the mounts/custom list BECOMES the tree; no modal, no second presentation
// of the same selection on the same screen).
//
// One role="tree" per FoldersEditor. Its level-1 treeitems are the mount rows
// followed by the custom-path rows; expanded children wrap in role="group".
// Notice rows (the D-04 blocked warn line, loading/empty/error/truncated)
// each ride inside a role="presentation" wrapper: APG allows tree/group
// children to be treeitem, group, or presentation-wrapped only, and
// presentation removes the wrapper from that structural contract while its
// text content stays exposed to assistive tech (review WR-01).
// Every node's checked/mixed/excluded state comes from classifyNode over the
// (includes, exclusions) props — pure list arithmetic in HOST path space, so
// it is correct for collapsed and never-loaded subtrees (TREE-03/04) and
// never depends on which children happen to be fetched.
//
// Laziness (TREE-01): children fetch on first expand only, through the
// browseCache Map the editor owns (panel lifetime — survives section close,
// dies with the page). Rejections and refused reads (ok:false) are removed
// from the cache so a retry genuinely refetches instead of replaying the
// failure. Browse is called with the browse-relative path (hostSourceRoot
// prefix swapped); the hidden-visibility opt-in is deliberately NOT sent
// (BROWSE-04 consistency with FolderBrowser).
//
// Selection is expressed ONLY via aria-checked on the treeitem
// ("true"/"mixed"/"false"), never aria-selected (APG: never mix). The mixed
// state is the DOM indeterminate property set in a callback ref — React has
// no prop for it; this is the app's first tri-state checkbox (Pattern 5).
// aria-expanded sits on every expandable treeitem (every directory is a
// potential parent — Phase 1 rejected emptiness probes; an expanded empty dir
// honestly reports true with zero children); it is omitted on the two honest
// leaf kinds (an unreachable mount, a custom path outside the served root),
// because APG end nodes must not announce themselves as parents (Pitfall 7).
//
// Keyboard (TREE-05, plan 03): the full APG TreeView checkbox-variant map on
// one onKeyDown on the tree element, with roving tabindex — exactly one
// treeitem (the focused, or last-focused, node; first root initially) carries
// tabIndex 0. Right expands with focus STAYING on the parent and descends to
// the first child only on a second press (a no-op while the loading row is
// the only child, since notice rows are not focusable); Left collapses /
// walks to the parent / does nothing on a closed root; Down/Up/Home/End move
// focus between treeitems without ever expanding; Enter is the expansion
// default action; Space is the ONLY selection toggler and routes through the
// same onToggle as checkbox clicks — one toggle semantics, so the editor's
// D-04 guard and serialized save queue cannot be bypassed by key (T-02-10).
//
// Expansion is comfort state only (D-05): persisted per container in
// localStorage, capped; selection NEVER goes there.
// ---------------------------------------------------------------------------

/** Per-node listing state. ok:false is its own state — never folded into
 *  "empty", never a silent collapse (TREE-06). */
type Listing =
  | { status: "loading" }
  | { status: "ok"; dirs: BrowseDirEntry[]; truncated: boolean }
  | { status: "error"; message: string };

export interface SelectionTreeProps {
  /** Server-discovered mount rows; the first level-1 treeitems. */
  mounts: MountInfo[];
  /** Custom backup paths (host form); trailing level-1 treeitems. */
  customPaths: CustomPath[];
  /** Includes in HOST path space — the server-truth mirror. */
  includes: ReadonlySet<string>;
  /** Exclusions in HOST path space ("!" stripped) — dormant entries included. */
  exclusions: ReadonlySet<string>;
  /** Host source root (e.g. "/mnt"); the browse translation prefix. */
  hostSourceRoot: string;
  /** Container name — scopes the bv-tree-expanded-{name} key (D-05). */
  containerName: string;
  /** Editor-lifetime listings cache: host path -> browse promise. */
  browseCache: Map<string, Promise<BrowseResponse>>;
  /** Checkbox toggle; carries the node's HOST path. */
  onToggle: (hostPath: string) => void;
  /** Remove-button handler for custom rows; carries the HOST path. */
  onRemoveCustom: (hostPath: string) => void;
  /** Per-root CACHEDIR.TAG toggles in HOST path form (D-06, RESTIC-01) —
   *  the stored map the switches render from, exactly as the mounts response
   *  served it plus the editor's optimistic flips.
   *  Optional since Phase 4 (INTEG-02): the Files page reuses this tree over
   *  a single set root, where RESTIC-01 deliberately does not apply (D-07
   *  deferral). When this or onToggleCaches is absent the CACHEDIR sub-row is
   *  skipped entirely — an empty map plus a no-op handler would render a dead
   *  switch, a silent lie (UI-SPEC reuse contract item 5). Container callers
   *  pass both, unchanged. */
  excludeCaches?: Readonly<Record<string, boolean>>;
  /** CACHEDIR switch flip; carries the root's HOST path and the new value.
   *  Rides the SAME serialized save queue as onToggle (T-03-07). Optional
   *  together with excludeCaches — see that prop's note. */
  onToggleCaches?: (hostPath: string, next: boolean) => void;
  /** Paths with a save in flight; their checkboxes disable. */
  busyPaths?: ReadonlySet<string>;
  /** Shake nonces per path; a bumped key replays .glim-shake on that row. */
  shakeCounts?: Readonly<Record<string, number>>;
  /** Path whose last toggle was blocked (D-04); shows the inline warn line. */
  blockedPath?: string | null;
  /** Copy for the blocked warn line. Optional since Phase 4 (INTEG-02): the
   *  Files page reuses this tree, and its refusal copy orients to that
   *  domain's own exit ("Remove set", D-06) — reusing the folders wording
   *  ("Use Reset") would name an action the card does not have (UI-SPEC
   *  copy table). Absent keeps the folders key, byte-identical for the
   *  container callers. */
  blockedMessage?: string;
}

/** What one treeitem row needs; children specs derive from listings. */
interface RowSpec {
  path: string;
  /** 0 for roots; children of a root are 1, and so on. */
  depth: number;
  setSize: number;
  posInSet: number;
  expandable: boolean;
  unreachable: boolean;
  removable: boolean;
  label: ReactNode;
}

/** One VISIBLE treeitem in visual order, with its parent treeitem's host path
 *  (null at level 1). The keyboard handler navigates this flat model; it is
 *  built by the exact same walk that renders (spec.expandable && expanded),
 *  so focus can never disagree with what is on screen. */
interface FlatNode {
  spec: RowSpec;
  parent: string | null;
}

export function SelectionTree({
  mounts,
  customPaths,
  includes,
  exclusions,
  hostSourceRoot,
  containerName,
  browseCache,
  onToggle,
  onRemoveCustom,
  excludeCaches,
  onToggleCaches,
  busyPaths,
  shakeCounts,
  blockedPath,
  blockedMessage,
}: SelectionTreeProps) {
  const { t } = useT();
  // D-05: expansion restored once on mount; the save effect below keeps the
  // key in step. Write order is recency (newest last) — selectionTree caps it.
  const [expandedOrder, setExpandedOrder] = useState<string[]>(() => loadExpanded(containerName));
  const [listings, setListings] = useState<Record<string, Listing>>({});
  const [focusPath, setFocusPath] = useState<string | null>(null);
  // Exclusions-disclosure expansion (D-03): synchronous component state over
  // the already-loaded mirror — no async, no loading state by construction.
  // Deliberately NOT persisted (unlike tree expansion): this is an audit
  // view, not navigation comfort, so every section reopens collapsed.
  const [exclOpen, setExclOpen] = useState<ReadonlySet<string>>(new Set());
  const exclIdPrefix = useId();
  // Roving-tabindex plumbing (TREE-05): the tree element (key target) and the
  // currently rendered treeitem rows, keyed by host path. Callback refs keep
  // the map exact through every expand/collapse — React nulls a row's entry
  // the moment it unmounts.
  const treeRef = useRef<HTMLDivElement>(null);
  const rowRefs = useRef(new Map<string, HTMLDivElement>());

  const fetchListing = useCallback(
    (hostPath: string) => {
      let promise = browseCache.get(hostPath);
      if (!promise) {
        promise = browse(hostToBrowseRel(hostPath, hostSourceRoot));
        browseCache.set(hostPath, promise);
        // A rejected fetch must not be memoized: the next expand retries.
        promise.catch(() => browseCache.delete(hostPath));
      }
      setListings((prev) => ({ ...prev, [hostPath]: { status: "loading" } }));
      promise.then(
        (res) => {
          if (!res.ok) {
            // A refused read is not a listing: drop it from the cache so
            // "Try again" refetches rather than replaying the refusal.
            browseCache.delete(hostPath);
            setListings((prev) => ({
              ...prev,
              [hostPath]: { status: "error", message: res.error ?? t("folder.couldNotRead") },
            }));
            return;
          }
          setListings((prev) => ({
            ...prev,
            [hostPath]: { status: "ok", dirs: res.dirs ?? [], truncated: res.truncated === true },
          }));
        },
        () => {
          browseCache.delete(hostPath);
          setListings((prev) => ({
            ...prev,
            [hostPath]: { status: "error", message: t("folder.couldNotRead") },
          }));
        },
      );
    },
    [browseCache, hostSourceRoot, t],
  );

  // Every expanded path must have its listing ensured — on expand (below) and
  // again on mount when D-05 restores expansion. Idempotent: paths already
  // tracked (any state) are left alone, so a retry is never auto-fired.
  useEffect(() => {
    for (const p of expandedOrder) {
      if (!listings[p]) fetchListing(p);
    }
  }, [expandedOrder, listings, fetchListing]);

  useEffect(() => {
    saveExpanded(containerName, expandedOrder);
  }, [containerName, expandedOrder]);

  const toggleExpansion = useCallback((hostPath: string) => {
    setExpandedOrder((order) =>
      order.includes(hostPath) ? order.filter((p) => p !== hostPath) : [...order, hostPath],
    );
  }, []);

  const toggleExclusions = useCallback((root: string) => {
    setExclOpen((prev) => {
      const next = new Set(prev);
      if (next.has(root)) next.delete(root);
      else next.add(root);
      return next;
    });
  }, []);

  const rootCount = mounts.length + customPaths.length;
  const firstRoot = mounts[0]?.source ?? customPaths[0]?.path ?? null;

  function childSpecs(parent: string, depth: number): RowSpec[] {
    const listing = listings[parent];
    if (!listing || listing.status !== "ok") return [];
    return listing.dirs.map((d, i) => {
      const host = browseRelToHost(d.path, hostSourceRoot);
      return {
        path: host,
        depth,
        setSize: listing.dirs.length,
        posInSet: i + 1,
        // Every directory is a potential parent (Phase 1 rejected emptiness
        // probes): always expandable, honestly reporting empty when opened.
        expandable: true,
        unreachable: false,
        removable: false,
        label: (
          <span dir="ltr" className="font-mono truncate text-start" title={host}>
            {d.name}
          </span>
        ),
      };
    });
  }

  const rootSpecs: RowSpec[] = [
    ...mounts.map((m, i) => ({
      path: m.source,
      depth: 0,
      setSize: rootCount,
      posInSet: i + 1,
      expandable: m.reachable,
      unreachable: !m.reachable,
      removable: false,
      label: (
        <span className="flex flex-col min-w-0">
          <span dir="ltr" className="font-mono break-all text-start">
            {/* Phase 4 (INTEG-02, UI-SPEC item 2): a dest-less synthetic mount
                row (the Files page's single set root) labels itself with the
                bare host path — the dest ← source arrow only describes a
                container mount. Container rows always carry a non-empty dest,
                so this branch never fires there and the container panel is
                byte-identical. */}
            {m.dest === "" ? m.source : `${m.dest} ← ${m.source}`}
          </span>
          {m.isAppdata && <span className="text-statusOk">{t("folders.appdataDefault")}</span>}
          {!m.reachable && <span className="text-statusFail">{t("folders.notReachable")}</span>}
          {/* D-01 preview: muted, derived from the includes set alone (never
              checked nodes, never existence) so it matches the bare entries
              the next save serializes. Renders on EVERY root incl. "0 paths"
              (INTEG-04: zero is information, not an empty state). */}
          <span className="text-xs text-carbon-textMuted">
            {t("folders.previewPaths").replace("{n}", String(rootIncludeCount(m.source, includes)))}
          </span>
        </span>
      ),
    })),
    ...customPaths.map((cp, i) => ({
      path: cp.path,
      depth: 0,
      setSize: rootCount,
      posInSet: mounts.length + i + 1,
      // A custom path outside the served root cannot be browsed (server
      // containment) — it renders as a plain row.
      expandable: isAtOrUnder(cp.path, hostSourceRoot) && cp.path !== hostSourceRoot,
      unreachable: false,
      removable: true,
      label: (
        <span className="flex flex-col flex-1 min-w-0">
          <span dir="ltr" className="font-mono break-all text-start">
            {cp.path}
          </span>
          {!cp.exists && <span className="text-statusFail">{t("folders.customMissing")}</span>}
          {/* Same D-01 preview contract as mount rows: standalone custom
              roots announce their own at-or-under include count. */}
          <span className="text-xs text-carbon-textMuted">
            {t("folders.previewPaths").replace("{n}", String(rootIncludeCount(cp.path, includes)))}
          </span>
        </span>
      ),
    })),
  ];

  // The flat visible model the keyboard navigates: same walk that renders, so
  // the order a user sees and the order arrows move through cannot diverge.
  const expandedSet = new Set(expandedOrder);
  function walk(specs: RowSpec[], parent: string | null, out: FlatNode[]): void {
    for (const spec of specs) {
      out.push({ spec, parent });
      if (spec.expandable && expandedSet.has(spec.path)) {
        walk(childSpecs(spec.path, spec.depth + 1), spec.path, out);
      }
    }
  }
  const flatNodes: FlatNode[] = [];
  walk(rootSpecs, null, flatNodes);
  const flatPaths = new Set(flatNodes.map((n) => n.spec.path));

  // Roving tabindex home: the focused node, or the first root before any
  // focus. A focusPath that is no longer VISIBLE (its ancestor collapsed
  // underneath it) falls back to the first root so exactly one tabbable
  // treeitem always exists in the DOM.
  const tabTarget = focusPath && flatPaths.has(focusPath) ? focusPath : firstRoot;

  /** Move the roving tabindex AND the real DOM focus to a treeitem. */
  function focusNode(hostPath: string): void {
    setFocusPath(hostPath);
    const row = rowRefs.current.get(hostPath);
    row?.scrollIntoView({ block: "nearest" });
    row?.focus();
  }

  // The APG TreeView key map (TREE-05). It acts on the roving-focus node, and
  // only when the event came from the tree itself or a treeitem row: focus
  // sitting on an inner control (a checkbox, the retry Button, a remove chip)
  // keeps that control's own key semantics — Space on a focused checkbox is
  // the input's click, which already routes through the same onToggle.
  function handleTreeKeyDown(e: React.KeyboardEvent<HTMLDivElement>): void {
    const el = e.target as HTMLElement;
    if (el !== treeRef.current && el.getAttribute("role") !== "treeitem") return;
    const idx = flatNodes.findIndex((n) => n.spec.path === tabTarget);
    if (idx < 0) return;
    const { spec, parent } = flatNodes[idx];
    const expanded = expandedSet.has(spec.path);
    switch (e.key) {
      case "ArrowRight":
        e.preventDefault();
        if (!spec.expandable) return; // honest leaf: nothing to descend into
        if (!expanded) {
          // APG: expand with focus STAYING on the parent (children may still
          // be loading); the browse fires through the toggleExpansion path.
          toggleExpansion(spec.path);
          return;
        }
        // Expanded: descend to the first child — but only once a child
        // treeitem EXISTS. While the loading notice row is the only thing
        // under the node there is nothing focusable, so this is a no-op.
        {
          const child = flatNodes.find((n) => n.parent === spec.path);
          if (child) focusNode(child.spec.path);
        }
        return;
      case "ArrowLeft":
        e.preventDefault();
        if (spec.expandable && expanded) {
          toggleExpansion(spec.path); // collapse, focus stays
          return;
        }
        // Closed child: walk up. A closed level-1 root has no parent: no-op.
        if (parent) focusNode(parent);
        return;
      case "ArrowDown":
        e.preventDefault();
        if (idx + 1 < flatNodes.length) focusNode(flatNodes[idx + 1].spec.path);
        return;
      case "ArrowUp":
        e.preventDefault();
        if (idx > 0) focusNode(flatNodes[idx - 1].spec.path);
        return;
      case "Home":
        e.preventDefault();
        if (flatNodes.length > 0) focusNode(flatNodes[0].spec.path);
        return;
      case "End":
        e.preventDefault();
        if (flatNodes.length > 0) focusNode(flatNodes[flatNodes.length - 1].spec.path);
        return;
      case "Enter":
        e.preventDefault();
        // Default action is expansion; a leaf has no default action.
        if (spec.expandable) toggleExpansion(spec.path);
        return;
      case " ":
        e.preventDefault();
        // The ONLY selection toggler, routed through the identical onToggle
        // pipeline as checkbox clicks (T-02-10) — and it respects the same
        // disabled rule the rendered checkbox has (unreachable row, save in
        // flight for that node).
        if (!spec.unreachable && !busyPaths?.has(spec.path)) onToggle(spec.path);
        return;
    }
  }

  function renderSpec(spec: RowSpec): ReactNode {
    const state = classifyNode(spec.path, includes, exclusions);
    const expanded = expandedSet.has(spec.path);
    const listing = listings[spec.path];
    const kids = expanded ? childSpecs(spec.path, spec.depth + 1) : [];
    const shaken = !!shakeCounts?.[spec.path];
    // D-03 review list: only ROOT rows carry an exclusions section (children
    // never do). The id pairs the button's aria-controls with the ul.
    const rootExcl = spec.depth === 0 ? rootExclusions(spec.path, exclusions) : [];
    // WR-03: the id derivation must be collision-free per root. The previous
    // non-alphanumeric stripping collided for paths that differ only in
    // stripped characters — "/mnt/user/app-data" and "/mnt/user/app_data"
    // both sanitized to "mnt-user-app-data", and any two CJK-named segments
    // collapsed the same way — producing duplicate DOM ids and an
    // aria-controls that resolved every button to the FIRST list. The
    // useId() prefix keeps trees apart; encodeURIComponent is injective on
    // the path, so distinct roots within one tree can never share an id.
    const exclListId = `${exclIdPrefix}excl-${encodeURIComponent(spec.path)}`;
    // UI-SPEC tone table: checked/mixed carry the row tone, unchecked is
    // dimmed, excluded (and unreachable) muted. Status hues live on the text
    // lines inside the label, never on a control.
    const tone =
      spec.unreachable || state === "excluded"
        ? "text-carbon-textMuted"
        : state === "unchecked"
          ? "text-carbon-textSub"
          : "text-carbon-text";
    const ariaChecked = state === "checked" ? "true" : state === "mixed" ? "mixed" : "false";
    // The mixed state rides the DOM indeterminate property — React has no
    // prop for it, so a fresh callback ref runs on every render pass.
    const boxRef = (el: HTMLInputElement | null) => {
      if (el) el.indeterminate = state === "mixed";
    };
    const indent = spec.depth > 0 ? { paddingInlineStart: spec.depth * 16 } : undefined;
    return (
      <Fragment key={`${spec.path}-${shakeCounts?.[spec.path] ?? 0}`}>
        <div
          role="treeitem"
          aria-checked={ariaChecked}
          aria-expanded={spec.expandable ? expanded : undefined}
          aria-level={spec.depth + 1}
          aria-setsize={spec.setSize}
          aria-posinset={spec.posInSet}
          tabIndex={tabTarget === spec.path ? 0 : -1}
          ref={(el) => {
            if (el) rowRefs.current.set(spec.path, el);
            else rowRefs.current.delete(spec.path);
          }}
          className={`flex items-start gap-2 text-xs min-w-0 ${spec.depth === 0 ? "min-h-8" : "min-h-7"} ${tone}${shaken ? " glim-shake" : ""}`}
          style={indent}
          onFocus={() => setFocusPath(spec.path)}
          onClick={spec.expandable ? () => toggleExpansion(spec.path) : undefined}
        >
          <span className="mt-0.5 w-4 shrink-0 flex items-center justify-center" aria-hidden="true">
            {spec.expandable && (
              // Presentational chevron (SnapshotFileTree's triangle): the row
              // body itself is the expansion control, so the glyph carries no
              // separate label to translate or double-fire.
              <svg
                width="10"
                height="10"
                viewBox="0 0 12 12"
                fill="none"
                className={`transition-transform ${expanded ? "rotate-90" : "rtl:rotate-180"}`}
              >
                <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
              </svg>
            )}
          </span>
          {/* aria-hidden on the input (review WR-02): the treeitem's
              aria-checked is the ONE selection announcement. For a mixed node
              the real input would otherwise say "checkbox checked" (the DOM
              indeterminate property has no ARIA reflection), contradicting
              "mixed" one row-element later. The input is tabIndex -1 and
              keyboard selection routes through Space on the treeitem
              (T-02-10), so hiding it from the a11y tree costs nothing; mouse
              users keep clicking it. */}
          <input
            ref={boxRef}
            type="checkbox"
            aria-hidden="true"
            tabIndex={-1}
            checked={state === "checked" || state === "mixed"}
            disabled={spec.unreachable || !!busyPaths?.has(spec.path)}
            onClick={(e) => e.stopPropagation()}
            onChange={() => onToggle(spec.path)}
            className="mt-0.5 shrink-0 accent-(--accent)"
          />
          {spec.label}
          {spec.removable && (
            <span onClick={(e) => e.stopPropagation()}>
              <Button
                label={t("offsite.targets.remove")}
                labelKey="offsite.targets.remove"
                variant="chip"
                onClick={() => onRemoveCustom(spec.path)}
              />
            </span>
          )}
        </div>
        {blockedPath === spec.path && (
          // Presentation wrapper (see the header note): the tree root's
          // children must stay treeitem/group-shaped while the warn text
          // remains announced.
          <div role="presentation">
            <p className="text-xs text-statusWarn" style={indent}>
              {blockedMessage ?? t("folders.emptySelectionBlocked")}
            </p>
          </div>
        )}
        {spec.depth === 0 && rootExcl.length > 0 && (
          // Reviewable exclusions (D-03/D-04): a per-root disclosure listing
          // the remembered exclusions as relative paths. Presentation-wrapped
          // like every notice row (the tree's structural contract), rendered
          // for ACTIVE and DORMANT roots identically — the root checkbox
          // above distinguishes them, and a deselected root's memory is
          // never hidden. Nothing here mutates state: selection flows only
          // through the tree's one toggle pipeline (T-02-10).
          <div role="presentation">
            <button
              type="button"
              aria-expanded={exclOpen.has(spec.path)}
              aria-controls={exclListId}
              onClick={() => toggleExclusions(spec.path)}
              // A normal tabbable control outside the roving set (retry-
              // Button precedent); the keydown target guard keeps its own
              // Space/Enter semantics.
              className="flex w-full items-center gap-2 py-1 text-start text-xs text-carbon-textMuted"
              style={{ paddingInlineStart: 16 }}
            >
              <span className="w-4 shrink-0 flex items-center justify-center" aria-hidden="true">
                <svg
                  width="10"
                  height="10"
                  viewBox="0 0 12 12"
                  fill="none"
                  className={`transition-transform ${exclOpen.has(spec.path) ? "rotate-90" : "rtl:rotate-180"}`}
                >
                  <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
                </svg>
              </span>
              {t("folders.exclusions").replace("{n}", String(rootExcl.length))}
            </button>
            {exclOpen.has(spec.path) && (
              // Non-interactive audit rows: relative paths, mono ltr
              // break-all with a title (house long-path pattern), muted,
              // lexically sorted, no controls, not focusable.
              <ul id={exclListId}>
                {rootExcl.map((rel) => (
                  <li
                    key={rel}
                    dir="ltr"
                    title={rel}
                    className="py-1 font-mono break-all text-start text-carbon-textMuted"
                    style={{ paddingInlineStart: 16 }}
                  >
                    {rel}
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
        {expanded && spec.expandable && (
          <div role="group" className="flex flex-col">
            {kids.map(renderSpec)}
            {/* Each notice row below rides in a role="presentation" wrapper
                (header note, review WR-01): group children stay
                treeitem/group-shaped, the notice text stays announced. */}
            {listing && listing.status === "loading" && (
              <div role="presentation">
                <div
                  className="flex items-center gap-2 text-xs text-carbon-textMuted py-1"
                  style={{ paddingInlineStart: (spec.depth + 1) * 16 }}
                >
                  <span className="h-3 w-3 rounded-full border-2 border-accentText border-t-transparent animate-spin" />
                  {t("folder.loading")}
                </div>
              </div>
            )}
            {listing && listing.status === "ok" && listing.dirs.length === 0 && (
              <div role="presentation">
                <p
                  className="text-xs text-carbon-textMuted"
                  style={{ paddingInlineStart: (spec.depth + 1) * 16 }}
                >
                  {t("folder.none")}
                </p>
              </div>
            )}
            {listing && listing.status === "error" && (
              <div role="presentation">
                <div
                  className="flex items-center gap-2 text-xs text-carbon-textMuted py-1"
                  style={{ paddingInlineStart: (spec.depth + 1) * 16 }}
                >
                  <span className="min-w-0 break-all">{listing.message}</span>
                  <Button
                    label={t("folders.retry")}
                    labelKey="folders.retry"
                    onClick={() => fetchListing(spec.path)}
                  />
                </div>
              </div>
            )}
            {listing && listing.status === "ok" && listing.truncated && (
              <div role="presentation">
                <p
                  className="text-xs text-carbon-textMuted"
                  style={{ paddingInlineStart: (spec.depth + 1) * 16 }}
                >
                  {t("folders.truncatedList")}
                </p>
              </div>
            )}
          </div>
        )}
        {spec.depth === 0 && excludeCaches !== undefined && onToggleCaches !== undefined && (
          // CACHEDIR.TAG sub-row (D-06, RESTIC-01): one switch per ROOT —
          // mounts and standalone customs alike, regardless of expand state —
          // in the same presentation-wrapped shape as the blocked warn line
          // (the tree root's children stay treeitem/group-shaped). The
          // switch carries hideLabel because the CALLER draws the visible
          // label right beside it: Toggle's internal caption span is text-sm
          // and would break the tree's 12px register (the sanctioned
          // caller-drawn-label case from Toggle's own contract). The
          // InfoBubble discloses the item-wide scope — the flag applies to
          // the whole backup, not only this folder. glim-shake rides the row
          // wrapper, replayed by the keyed nonce remount above; unreachable
          // mounts cannot back up, so their switch disables. indent 16 / py-1
          // match the exclusions disclosure's sub-row rhythm.
          //
          // The row renders AFTER the expanded children group, not between
          // the root and its first child (UAT finding 2026-09-10): indent 16
          // is exactly the depth-1 child indent, so a switch wedged above an
          // expanded subtree read as that subtree's first subfolder. Below
          // the group the root and its subfolders stay one contiguous visual
          // block and the control reads as the root's affordance; a collapsed
          // root still carries it directly beneath (no group renders). DOM
          // order follows the same rule, so Tab reaches the switch after the
          // subtree — one position, no roving-tabindex interaction.
          <div role="presentation">
            <div
              className={`flex items-center gap-2 py-1${shakeCounts?.[spec.path] ? " glim-shake" : ""}`}
              style={{ paddingInlineStart: 16 }}
            >
              <Toggle
                checked={excludeCaches[spec.path] === true}
                onChange={(next) => onToggleCaches(spec.path, next)}
                label={t("folders.cachedirToggle")}
                hideLabel
                disabled={spec.unreachable || !!busyPaths?.has(spec.path)}
              />
              <span className="text-xs text-carbon-textSub">{t("folders.cachedirToggle")}</span>
              <InfoBubble tip={t("folders.cachedirScope")} />
            </div>
          </div>
        )}
      </Fragment>
    );
  }

  return (
    <div
      ref={treeRef}
      role="tree"
      aria-label={t("folders.treeLabel")}
      onKeyDown={handleTreeKeyDown}
      className="h-[clamp(12rem,55vh,32rem)] overflow-y-auto"
    >
      {rootSpecs.map(renderSpec)}
    </div>
  );
}
