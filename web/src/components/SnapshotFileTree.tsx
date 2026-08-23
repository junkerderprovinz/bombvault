// ---------------------------------------------------------------------------
// SnapshotFileTree — the collapsible file-tree selection UI shared by every
// file-level restore: the container "Individual files" flow (RestorePanel) and
// the file-set selective restore (Files, #65). It renders a filter box plus,
// when unfiltered, a collapsible folder tree and, while filtering, a flat matched
// list — each node a checkbox that toggles it in the multi-select set. Ticking a
// folder selects its whole subtree. Purely presentational: the parent owns the
// files, filter and selection state.
// ---------------------------------------------------------------------------

import { useMemo, useState } from "react";
import type { FileEntry } from "../lib/api";
import type { useT } from "../lib/i18n";

type T = ReturnType<typeof useT>["t"];

// Cap the rendered file list — a snapshot can hold thousands of nodes; rendering
// them all would jank the UI. Users narrow with the filter box.
export const FILE_DISPLAY_CAP = 500;

export interface TreeNode {
  name: string;
  path: string; // full absolute path (matches FileEntry.path)
  type: "dir" | "file";
  children: Map<string, TreeNode>;
}

// buildTree turns restic's flat file list into a nested folder tree keyed by
// path segment, so the browser can be expanded/collapsed like a file manager.
export function buildTree(files: FileEntry[]): TreeNode {
  const root: TreeNode = { name: "", path: "", type: "dir", children: new Map() };
  for (const f of files) {
    const segs = f.path.split("/").filter(Boolean);
    let node = root;
    let acc = "";
    segs.forEach((seg, i) => {
      acc += "/" + seg;
      let child = node.children.get(seg);
      if (!child) {
        const isLast = i === segs.length - 1;
        const type: "dir" | "file" = isLast && f.type === "file" ? "file" : "dir";
        child = { name: seg, path: acc, type, children: new Map() };
        node.children.set(seg, child);
      }
      node = child;
    });
  }
  return root;
}

// sortNodes orders tree children: directories first, then alphabetically.
export function sortNodes(a: TreeNode, b: TreeNode): number {
  if (a.type !== b.type) return a.type === "dir" ? -1 : 1;
  return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
}

// FileRow is one entry in the flat (filtered) list: a checkbox that toggles the
// file/dir in the multi-select restore set.
function FileRow({
  file,
  selected,
  onToggle,
}: {
  file: FileEntry;
  selected: boolean;
  onToggle: () => void;
}) {
  return (
    <label className="flex items-center gap-2 py-1 text-xs border-b border-carbon-border last:border-0 cursor-pointer">
      <input
        type="checkbox"
        checked={selected}
        onChange={onToggle}
        className="shrink-0"
        style={{ accentColor: "var(--accent)" }}
      />
      <span dir="ltr" className="font-mono text-carbon-textSub flex-1 min-w-0 truncate text-start" title={file.path}>
        {file.type === "dir" ? "📁 " : ""}
        {file.path}
      </span>
    </label>
  );
}

// TreeRow renders one node of the collapsible file tree. Directories expand /
// collapse; a checkbox on every node (file or folder) toggles it in the
// multi-select restore set. Ticking a folder restores its whole subtree.
function TreeRow({
  node,
  depth,
  selected,
  onToggle,
}: {
  node: TreeNode;
  depth: number;
  selected: Set<string>;
  onToggle: (p: string) => void;
}) {
  const [expanded, setExpanded] = useState(depth === 0); // top level open by default
  const isDir = node.type === "dir";
  const kids = isDir ? Array.from(node.children.values()).sort(sortNodes) : [];

  return (
    <div>
      <div
        className="flex items-center gap-1 py-0.5 text-xs rounded-control hover:bg-carbon-hover"
        // `paddingInlineStart`, not `paddingLeft` — a physical inline style
        // (a plain `style` prop, not a Tailwind class) is exactly what the
        // class-based `pl-`/`pr-`/etc. grep sweep couldn't see (RTL sweep,
        // form-engine Phase 2 Task 6 follow-up fix). This row carries no
        // `dir` override of its own, so the logical property correctly
        // resolves against the surrounding page's real direction: the
        // indent piles up on the reading-start edge (left in LTR, right in
        // RTL) either way, instead of always visually hugging the left
        // regardless of direction — which made every depth in RTL look the
        // same distance from the RTL reading edge, hiding the hierarchy.
        style={{ paddingInlineStart: depth * 14 }}
      >
        {isDir ? (
          <button
            onClick={() => setExpanded((e) => !e)}
            className="w-4 shrink-0 text-carbon-textMuted hover:text-carbon-text"
            aria-label={expanded ? "collapse" : "expand"}
          >
            <svg width="10" height="10" viewBox="0 0 12 12" fill="none" className={`transition-transform ${expanded ? "rotate-90" : "rtl:rotate-180"}`}>
              <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
            </svg>
          </button>
        ) : (
          <span className="w-4 shrink-0" />
        )}
        <input
          type="checkbox"
          checked={selected.has(node.path)}
          onChange={() => onToggle(node.path)}
          className="shrink-0"
          style={{ accentColor: "var(--accent)" }}
          aria-label={node.path}
        />
        <span className="shrink-0">{isDir ? "📁" : "📄"}</span>
        <span dir="ltr" className="font-mono text-carbon-textSub flex-1 truncate text-start" title={node.path}>
          {node.name}
        </span>
      </div>
      {isDir && expanded && kids.map((c) => (
        <TreeRow key={c.path} node={c} depth={depth + 1} selected={selected} onToggle={onToggle} />
      ))}
    </div>
  );
}

// SnapshotFileTree is the filter box + flat/tree selection list. Controlled: the
// parent owns `files`, `filter` and `selected` and the toggle. Loading/error
// states render inline (the parent drives them off its own fetch).
export function SnapshotFileTree({
  files,
  loading,
  error,
  filter,
  onFilterChange,
  selected,
  onToggle,
  t,
}: {
  files: FileEntry[];
  loading: boolean;
  error: string | null;
  filter: string;
  onFilterChange: (v: string) => void;
  selected: Set<string>;
  onToggle: (p: string) => void;
  t: T;
}) {
  const q = filter.trim().toLowerCase();
  const matched = q ? files.filter((f) => f.path.toLowerCase().includes(q)) : files;
  const shown = matched.slice(0, FILE_DISPLAY_CAP);
  const tree = useMemo(() => buildTree(files), [files]);
  const topLevel = Array.from(tree.children.values()).sort(sortNodes);

  return (
    <>
      <input
        type="text"
        value={filter}
        onChange={(e) => onFilterChange(e.target.value)}
        placeholder={t("files.filterPlaceholder")}
        spellCheck={false}
        className="rounded-control bg-carbon-surface2 text-carbon-text text-xs px-2 py-1 bv-field-focus"
      />
      {loading && <p className="text-xs text-carbon-textMuted">…</p>}
      {error && <p className="text-xs text-statusFail">{error}</p>}
      {!loading && !error && (q ? matched.length === 0 : topLevel.length === 0) && (
        <p className="text-xs text-carbon-textMuted">{t("files.none")}</p>
      )}
      {/* Filtering → flat matched list (easier to scan); otherwise → folder tree. */}
      {!loading && q && shown.length > 0 && (
        <div className="max-h-64 overflow-y-auto">
          {shown.map((f) => (
            <FileRow key={f.path} file={f} selected={selected.has(f.path)} onToggle={() => onToggle(f.path)} />
          ))}
        </div>
      )}
      {!loading && q && matched.length > shown.length && (
        <p className="text-xs text-carbon-textMuted">{t("files.more")}</p>
      )}
      {!loading && !q && topLevel.length > 0 && (
        <div className="max-h-64 overflow-y-auto">
          {topLevel.map((n) => (
            <TreeRow key={n.path} node={n} depth={0} selected={selected} onToggle={onToggle} />
          ))}
        </div>
      )}
    </>
  );
}
