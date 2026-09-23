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

// SelectionTree is the backup-folder tree of a container's folder editor: one
// role="tree" whose top-level items are the mounts followed by the custom
// paths.
//
// A node's checked, mixed or excluded state comes from classifyNode over the
// includes and exclusions in host path space, so it is right for subtrees that
// are collapsed or were never loaded. Selection is exposed only as
// aria-checked, never aria-selected.
//
// Children are fetched on first expand through the editor's browseCache.
// Failed and refused reads are evicted so a retry really refetches. The browse
// includes hidden directories: the tree is the backup selection, and a
// dot-directory without a row could not be ticked or unticked (Home Assistant
// keeps its config and device registry in .storage).
//
// There is no emptiness probe, so every directory is expandable and an empty
// one reports aria-expanded="true" with no children. Leaves (an unreachable
// mount, a custom path outside the served root) omit aria-expanded.
//
// Notice rows (the blocked warning, loading, empty, error, truncated) sit in
// role="presentation" wrappers, because a tree or group may only contain
// treeitems, groups and presentational wrappers.
//
// Keyboard follows the APG tree view with a roving tabindex. Space toggles the
// checkbox through the same onToggle as a click, so the editor's
// empty-selection guard and save queue apply to it too.
//
// Expansion is saved per container in localStorage. Selection never is.

/** Per-node listing state. A refused read is an error, not an empty listing. */
type Listing =
  | { status: "loading" }
  | { status: "ok"; dirs: BrowseDirEntry[]; truncated: boolean }
  | { status: "error"; message: string };

export interface SelectionTreeProps {
  /** Mount rows from the server, listed first. */
  mounts: MountInfo[];
  /** Custom backup paths in host form, listed after the mounts. */
  customPaths: CustomPath[];
  /** Includes in host path space, mirroring the server. */
  includes: ReadonlySet<string>;
  /** Exclusions in host path space without the "!", dormant ones included. */
  exclusions: ReadonlySet<string>;
  /** Host source root such as "/mnt"; browse paths are relative to it. */
  hostSourceRoot: string;
  /** Scopes the bv-tree-expanded-{name} localStorage key. */
  containerName: string;
  /** Listings cache owned by the editor: host path to browse promise. */
  browseCache: Map<string, Promise<BrowseResponse>>;
  /** Checkbox toggle; carries the node's host path. */
  onToggle: (hostPath: string) => void;
  /** Remove-button handler for custom rows; carries the host path. */
  onRemoveCustom: (hostPath: string) => void;
  /** Per-root CACHEDIR.TAG switches in host path form. The Files page passes
   *  neither this nor onToggleCaches, which hides the switch row rather than
   *  rendering a switch that does nothing. */
  excludeCaches?: Readonly<Record<string, boolean>>;
  /** CACHEDIR switch handler, on the same save queue as onToggle. */
  onToggleCaches?: (hostPath: string, next: boolean) => void;
  /** Paths with a save in flight; their checkboxes disable. */
  busyPaths?: ReadonlySet<string>;
  /** Shake nonces per path; a bumped key replays .glim-shake on that row. */
  shakeCounts?: Readonly<Record<string, number>>;
  /** Path whose last toggle was blocked; shows the inline warning. */
  blockedPath?: string | null;
  /** Text of the blocked warning. The Files page passes its own, because the
   *  default names a reset action its card does not have. */
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

/** A visible treeitem and its parent's host path (null at the top level). */
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
  // Newest last, which is the order saveExpanded trims by.
  const [expandedOrder, setExpandedOrder] = useState<string[]>(() => loadExpanded(containerName));
  const [listings, setListings] = useState<Record<string, Listing>>({});
  const [focusPath, setFocusPath] = useState<string | null>(null);
  // Roots whose exclusion list is open. Not persisted: it is a review view,
  // so it reopens collapsed.
  const [exclOpen, setExclOpen] = useState<ReadonlySet<string>>(new Set());
  const exclIdPrefix = useId();
  // Rendered treeitem rows by host path, for moving focus. The callback ref
  // drops a row's entry when it unmounts.
  const treeRef = useRef<HTMLDivElement>(null);
  const rowRefs = useRef(new Map<string, HTMLDivElement>());

  const fetchListing = useCallback(
    (hostPath: string) => {
      let promise = browseCache.get(hostPath);
      if (!promise) {
        promise = browse(hostToBrowseRel(hostPath, hostSourceRoot), true);
        browseCache.set(hostPath, promise);
        // A rejected fetch must not be memoized: the next expand retries.
        promise.catch(() => browseCache.delete(hostPath));
      }
      setListings((prev) => ({ ...prev, [hostPath]: { status: "loading" } }));
      promise.then(
        (res) => {
          if (!res.ok) {
            // Evict a refused read so a retry refetches.
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

  // Fetches a listing for every expanded path, including those restored from
  // localStorage. Paths already tracked in any state are left alone, so a
  // failed read is not retried automatically.
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
            {/* The Files page passes its set root as a mount without a dest. */}
            {m.dest === "" ? m.source : `${m.dest} ← ${m.source}`}
          </span>
          {m.isAppdata && <span className="text-statusOk">{t("folders.appdataDefault")}</span>}
          {!m.reachable && <span className="text-statusFail">{t("folders.notReachable")}</span>}
          {/* Counted from the includes alone, so it matches what the next
              save sends. Shown on every root, zero included. */}
          <span className="text-xs text-carbon-textMuted">
            {t("folders.previewPaths", rootIncludeCount(m.source, includes))}
          </span>
        </span>
      ),
    })),
    ...customPaths.map((cp, i) => ({
      path: cp.path,
      depth: 0,
      setSize: rootCount,
      posInSet: mounts.length + i + 1,
      // The server only browses inside the served root.
      expandable: isAtOrUnder(cp.path, hostSourceRoot) && cp.path !== hostSourceRoot,
      unreachable: false,
      removable: true,
      label: (
        <span className="flex flex-col flex-1 min-w-0">
          <span dir="ltr" className="font-mono break-all text-start">
            {cp.path}
          </span>
          {!cp.exists && <span className="text-statusFail">{t("folders.customMissing")}</span>}
          <span className="text-xs text-carbon-textMuted">
            {t("folders.previewPaths", rootIncludeCount(cp.path, includes))}
          </span>
        </span>
      ),
    })),
  ];

  // The visible rows in order for the keyboard handler, built by the same walk
  // as the render so the two cannot disagree.
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

  // The one tabbable treeitem: the focused node, or the first root when
  // nothing was focused yet or the focused node was collapsed away.
  const tabTarget = focusPath && flatPaths.has(focusPath) ? focusPath : firstRoot;

  /** Moves the roving tabindex and DOM focus to a treeitem. */
  function focusNode(hostPath: string): void {
    setFocusPath(hostPath);
    const row = rowRefs.current.get(hostPath);
    row?.scrollIntoView({ block: "nearest" });
    row?.focus();
  }

  // The APG tree view key map. Keys pressed on an inner control (the retry
  // button, a remove chip, the exclusions toggle) keep that control's own
  // behaviour.
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
        if (!spec.expandable) return;
        if (!expanded) {
          // Focus stays on the parent while the children load.
          toggleExpansion(spec.path);
          return;
        }
        // Move to the first child. While the children are still loading
        // there is none, and nothing happens.
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
        // Move to the parent; a top-level row has none.
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
        // Space is the only key that changes the selection. It goes through
        // onToggle like a click and is disabled when the checkbox is.
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
    // Only top-level rows list their exclusions.
    const rootExcl = spec.depth === 0 ? rootExclusions(spec.path, exclusions) : [];
    // encodeURIComponent is injective, so roots that differ only in
    // punctuation still get distinct ids; useId keeps separate trees apart.
    const exclListId = `${exclIdPrefix}excl-${encodeURIComponent(spec.path)}`;
    const tone =
      spec.unreachable || state === "excluded"
        ? "text-carbon-textMuted"
        : state === "unchecked"
          ? "text-carbon-textSub"
          : "text-carbon-text";
    const ariaChecked = state === "checked" ? "true" : state === "mixed" ? "mixed" : "false";
    // React has no prop for indeterminate. A new callback ref on every render
    // keeps it in sync.
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
              // Decorative: clicking the row itself expands it.
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
          {/* Hidden from assistive tech: the treeitem's aria-checked carries
              the state, and for a mixed node the input would announce
              "checked". Keyboard users toggle with Space on the row. */}
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
          <div role="presentation">
            <p className="text-xs text-statusWarn" style={indent}>
              {blockedMessage ?? t("folders.emptySelectionBlocked")}
            </p>
          </div>
        )}
        {spec.depth === 0 && rootExcl.length > 0 && (
          // The root's remembered exclusions as relative paths, read-only,
          // shown whether or not the root is selected.
          <div role="presentation">
            <button
              type="button"
              aria-expanded={exclOpen.has(spec.path)}
              aria-controls={exclListId}
              onClick={() => toggleExclusions(spec.path)}
              // Tabbable on its own, outside the roving tabindex.
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
              {t("folders.exclusions", rootExcl.length)}
            </button>
            {exclOpen.has(spec.path) && (
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
          // One CACHEDIR.TAG switch per root. The label is drawn here because
          // Toggle's own caption is text-sm, too large for the tree, and the
          // InfoBubble says the flag applies to the whole backup. The row
          // comes after the expanded children: at this indent, placed above
          // them it reads as the first subfolder.
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
