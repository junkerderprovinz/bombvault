import { Fragment, useRef, useState, type ReactNode } from "react";

import type { ZFSHostDataset } from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import type { useT } from "../../lib/i18n";
import { zfsCodeSentence } from "../../lib/zfsCodes";
import { Badge } from "../Badge";
import { Toggle } from "../Toggle";

type T = ReturnType<typeof useT>["t"];

// The host listing arrives flat and in tree order; the dialog draws the pools
// from the names. The tree follows SelectionTree's contract (role="tree",
// treeitems with a roving tabindex, arrow-key navigation) so the two selection
// trees of the app behave the same, but it selects with a Toggle rather than a
// checkbox: a root becomes an item, and a child of a chosen root is included
// or left out.
//
// Datasets with mountpoint=legacy are Docker's image layers, often thousands
// of them. They are counted on their parent instead of drawn, because no one
// picks one of them and the list would be unusable.

interface TreeNode {
  entry: ZFSHostDataset;
  children: TreeNode[];
  /** Docker layer datasets directly below this one. */
  legacy: number;
  depth: number;
}

interface Row {
  node: TreeNode;
  parent: string | null;
  setSize: number;
  posInSet: number;
  expandable: boolean;
  expanded: boolean;
}

function parentOf(dataset: string): string {
  const at = dataset.lastIndexOf("/");
  return at < 0 ? "" : dataset.slice(0, at);
}

function baseOf(dataset: string): string {
  const at = dataset.lastIndexOf("/");
  return at < 0 ? dataset : dataset.slice(at + 1);
}

function poolOf(dataset: string): string {
  const at = dataset.indexOf("/");
  return at < 0 ? dataset : dataset.slice(0, at);
}

/** The path below the pool, "" for a pool root. */
function shareOf(dataset: string): string {
  const at = dataset.indexOf("/");
  return at < 0 ? "" : dataset.slice(at + 1);
}

export function isBelow(dataset: string, root: string): boolean {
  return dataset.startsWith(root + "/");
}

function isLegacyLayer(entry: ZFSHostDataset): boolean {
  return entry.type === "filesystem" && entry.hostMountpoint === "legacy";
}

function buildNodes(entries: ZFSHostDataset[]): TreeNode[] {
  const byName = new Map<string, TreeNode>();
  const roots: TreeNode[] = [];
  for (const entry of entries) {
    const parent = byName.get(parentOf(entry.dataset));
    if (isLegacyLayer(entry)) {
      if (parent) parent.legacy++;
      continue;
    }
    const node: TreeNode = { entry, children: [], legacy: 0, depth: parent ? parent.depth + 1 : 0 };
    byName.set(entry.dataset, node);
    if (parent) parent.children.push(node);
    else roots.push(node);
  }
  return roots;
}

/** Keeps the nodes whose name matches and the ancestors that lead to them. */
function pruneNodes(nodes: TreeNode[], needle: string): TreeNode[] {
  const out: TreeNode[] = [];
  for (const node of nodes) {
    const children = pruneNodes(node.children, needle);
    const hit = node.entry.dataset.toLowerCase().includes(needle);
    if (children.length > 0 || hit) out.push({ ...node, children });
  }
  return out;
}

/** Datasets that carry the same path on more than one pool, mapped to the
 *  other pools they live on. Unraid's mover splits one user share this way. */
function shareSplits(entries: ZFSHostDataset[]): Map<string, string[]> {
  const byShare = new Map<string, ZFSHostDataset[]>();
  for (const entry of entries) {
    const share = shareOf(entry.dataset);
    if (entry.type !== "filesystem" || share === "" || isLegacyLayer(entry)) continue;
    byShare.set(share, [...(byShare.get(share) ?? []), entry]);
  }
  const out = new Map<string, string[]>();
  for (const group of byShare.values()) {
    if (group.length < 2) continue;
    for (const entry of group) {
      out.set(
        entry.dataset,
        group.filter((o) => o !== entry).map((o) => poolOf(o.dataset)),
      );
    }
  }
  return out;
}

export function ZFSDatasetTree({
  entries,
  filter,
  selected,
  excluded,
  onSelectRoot,
  onIncludeChild,
  maxNameLength,
  t,
}: {
  entries: ZFSHostDataset[];
  /** Name filter; while it is set the whole matching tree is open. */
  filter: string;
  /** Datasets the user is adding as items of their own. */
  selected: ReadonlySet<string>;
  /** Children of a chosen root that are to stay out of it. */
  excluded: ReadonlySet<string>;
  onSelectRoot: (dataset: string, on: boolean) => void;
  onIncludeChild: (dataset: string, include: boolean) => void;
  maxNameLength: number;
  t: T;
}) {
  const [openState, setOpenState] = useState<Record<string, boolean>>({});
  const [focusName, setFocusName] = useState<string | null>(null);
  const treeRef = useRef<HTMLDivElement>(null);
  const rowRefs = useRef(new Map<string, HTMLDivElement>());

  const needle = filter.trim().toLowerCase();
  const all = buildNodes(entries);
  const roots = needle === "" ? all : pruneNodes(all, needle);
  const splits = shareSplits(entries);
  const itemNames = new Map(entries.filter((e) => e.managedId !== "").map((e) => [e.managedId, e.dataset]));

  /** The chosen root this dataset sits under, itself included. */
  function rootFor(dataset: string): string | null {
    for (const root of selected) {
      if (dataset === root || isBelow(dataset, root)) return root;
    }
    return null;
  }

  function isOpen(node: TreeNode): boolean {
    if (needle !== "") return true;
    return openState[node.entry.dataset] ?? (node.depth === 0 || rootFor(node.entry.dataset) !== null);
  }

  function toggleOpen(node: TreeNode): void {
    const wasOpen = isOpen(node);
    setOpenState((prev) => ({ ...prev, [node.entry.dataset]: !wasOpen }));
  }

  const rows: Row[] = [];
  (function walk(nodes: TreeNode[], parent: string | null) {
    nodes.forEach((node, i) => {
      const expandable = node.children.length > 0;
      const expanded = expandable && isOpen(node);
      rows.push({ node, parent, setSize: nodes.length, posInSet: i + 1, expandable, expanded });
      if (expanded) walk(node.children, node.entry.dataset);
    });
  })(roots, null);

  const names = new Set(rows.map((r) => r.node.entry.dataset));
  const tabTarget = focusName && names.has(focusName) ? focusName : (rows[0]?.node.entry.dataset ?? null);

  function focusRow(dataset: string): void {
    setFocusName(dataset);
    const row = rowRefs.current.get(dataset);
    row?.scrollIntoView({ block: "nearest" });
    row?.focus();
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLDivElement>): void {
    const el = e.target as HTMLElement;
    if (el !== treeRef.current && el.getAttribute("role") !== "treeitem") return;
    const idx = rows.findIndex((r) => r.node.entry.dataset === tabTarget);
    if (idx < 0) return;
    const row = rows[idx];
    switch (e.key) {
      case "ArrowRight":
        e.preventDefault();
        if (!row.expandable) return;
        if (!row.expanded) toggleOpen(row.node);
        else focusRow(row.node.children[0].entry.dataset);
        return;
      case "ArrowLeft":
        e.preventDefault();
        if (row.expanded) toggleOpen(row.node);
        else if (row.parent) focusRow(row.parent);
        return;
      case "ArrowDown":
        e.preventDefault();
        if (idx + 1 < rows.length) focusRow(rows[idx + 1].node.entry.dataset);
        return;
      case "ArrowUp":
        e.preventDefault();
        if (idx > 0) focusRow(rows[idx - 1].node.entry.dataset);
        return;
      case "Home":
        e.preventDefault();
        focusRow(rows[0].node.entry.dataset);
        return;
      case "End":
        e.preventDefault();
        focusRow(rows[rows.length - 1].node.entry.dataset);
        return;
      case "Enter":
        e.preventDefault();
        if (row.expandable) toggleOpen(row.node);
        return;
    }
  }

  /** The switch or the sentence that stands in for it, at the row's end. */
  function control(entry: ZFSHostDataset): ReactNode {
    const dataset = entry.dataset;
    if (entry.type === "volume") {
      return (
        <>
          <span className="text-caption text-carbon-textMuted">{zfsCodeSentence(t, "zvol")}</span>
          <span className="text-caption text-carbon-textMuted">
            {entry.vmVolume ? t("zfs.add.vmVolume") : t("zfs.add.unusedZvol")}
          </span>
        </>
      );
    }
    if (entry.managedId !== "") {
      return <span className="text-caption text-carbon-textMuted">{t("zfs.add.isItem")}</span>;
    }
    const root = rootFor(dataset);
    if (root !== null && root !== dataset) {
      const off = excluded.has(dataset) || [...excluded].some((x) => isBelow(dataset, x));
      return (
        <>
          {entry.vmDisk && <span className="text-caption text-carbon-textMuted">{t("zfs.add.vmDisk")}</span>}
          {entry.system && <span className="text-caption text-carbon-textMuted">{t("zfs.add.system")}</span>}
          {entry.memberCode !== "" && (
            <span className="text-caption text-statusWarn">
              {t("zfs.add.willSkip").replace(
                "{reason}",
                zfsCodeSentence(t, entry.memberCode, { hostMountpoint: entry.hostMountpoint }),
              )}
            </span>
          )}
          <Toggle
            hideLabel
            label={dataset}
            checked={!off}
            onChange={(next) => onIncludeChild(dataset, next)}
          />
        </>
      );
    }
    if (entry.coveredBy !== "") {
      return (
        <span className="text-caption text-carbon-textMuted">
          {t("zfs.add.inItem").replace("{dataset}", itemNames.get(entry.coveredBy) ?? entry.coveredBy)}
        </span>
      );
    }
    if (entry.blockers.length > 0) {
      const blocker = entry.blockers[0];
      const names =
        blocker === "overlaps-item"
          ? entries.filter((o) => o.managedId !== "" && isBelow(o.dataset, dataset)).map((o) => o.dataset)
          : [dataset];
      return (
        <span className="text-caption text-carbon-textMuted">
          {zfsCodeSentence(t, blocker, { names, max: maxNameLength })}
        </span>
      );
    }
    // A root above another chosen root would overlap it; switching that one
    // off is the way to pick this one instead.
    if ([...selected].some((s) => isBelow(s, dataset))) return null;
    return (
      <Toggle
        hideLabel
        label={`${t("zfs.add.asItem")} ${dataset}`}
        checked={selected.has(dataset)}
        onChange={(next) => onSelectRoot(dataset, next)}
      />
    );
  }

  function renderNode(node: TreeNode, setSize: number, posInSet: number): ReactNode {
    const entry = node.entry;
    const expandable = node.children.length > 0;
    const expanded = expandable && isOpen(node);
    const indent = { paddingInlineStart: node.depth * 16 };
    const others = splits.get(entry.dataset);
    return (
      <Fragment key={entry.dataset}>
        <div
          role="treeitem"
          aria-expanded={expandable ? expanded : undefined}
          aria-level={node.depth + 1}
          aria-setsize={setSize}
          aria-posinset={posInSet}
          tabIndex={tabTarget === entry.dataset ? 0 : -1}
          ref={(el) => {
            if (el) rowRefs.current.set(entry.dataset, el);
            else rowRefs.current.delete(entry.dataset);
          }}
          className="flex min-h-8 items-center gap-2 text-xs text-carbon-textSub"
          style={indent}
          onFocus={() => setFocusName(entry.dataset)}
          onClick={expandable ? () => toggleOpen(node) : undefined}
        >
          <span className="w-4 shrink-0 flex items-center justify-center" aria-hidden="true">
            {expandable && (
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
          <span className="flex min-w-0 flex-col">
            <span dir="ltr" className="font-mono truncate text-start text-carbon-text" title={entry.dataset}>
              {node.depth === 0 ? entry.dataset : baseOf(entry.dataset)}
            </span>
            <span dir="ltr" className="text-caption text-carbon-textMuted truncate text-start">
              {entry.hostMountpoint}
            </span>
          </span>
          <span className="text-caption text-carbon-textMuted shrink-0" title={humanBytes(entry.used)}>
            {t("zfs.add.size").replace("{size}", humanBytes(entry.referenced))}
          </span>
          {entry.encrypted && (
            <Badge tone="neutral" size="small">
              {t("zfs.add.encrypted")}
            </Badge>
          )}
          <span className="ms-auto flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
            {control(entry)}
          </span>
        </div>
        {others && (
          <div role="presentation" style={indent}>
            <p className="text-caption text-carbon-textMuted">
              {t("zfs.add.shareSplit")
                .replace("{dataset}", shareOf(entry.dataset))
                .replace("{names}", others.join(", "))}
            </p>
          </div>
        )}
        {node.legacy > 0 && (
          <div role="presentation" style={{ paddingInlineStart: (node.depth + 1) * 16 }}>
            <p className="text-caption text-carbon-textMuted">
              {t("zfs.add.hiddenLegacy").replace("{n}", String(node.legacy))}
            </p>
          </div>
        )}
        {expanded && (
          <div role="group" className="flex flex-col">
            {node.children.map((child, i) => renderNode(child, node.children.length, i + 1))}
          </div>
        )}
      </Fragment>
    );
  }

  return (
    <div
      ref={treeRef}
      role="tree"
      aria-label={t("nav.zfs")}
      onKeyDown={handleKeyDown}
      className="h-[clamp(12rem,45vh,28rem)] overflow-y-auto"
    >
      {roots.length === 0 && (
        <div role="presentation">
          <p className="text-xs text-carbon-textMuted">{t("zfs.add.none")}</p>
        </div>
      )}
      {roots.map((node, i) => renderNode(node, roots.length, i + 1))}
    </div>
  );
}
