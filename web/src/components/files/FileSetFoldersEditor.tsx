import { useId, useRef, useState } from "react";
import { patchFileSet } from "../../lib/api";
import type { AnomalyItem, BrowseResponse, FileSetView } from "../../lib/api";
import { applyToggle, browseRelToHost, splitFlatSet, toFlatList } from "../../lib/selectionTree";
import { SelectionTree } from "../SelectionTree";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { ItemAnomalySettings } from "../ItemAnomalySettings";

type T = ReturnType<typeof useT>["t"];

/** What a queued file-set PATCH was sent for. `pre` and `sent` hold the
 *  toggle's effect, so a failure undoes exactly that toggle (revertFrom)
 *  rather than restoring a snapshot, which would also undo newer toggles
 *  queued behind it. `node` is the row a failure points at: it shakes, and
 *  an empty-selection refusal puts its warn line under it. */
interface FileSetSaveDesc {
  node: string;
  pre: { includes: ReadonlySet<string>; exclusions: ReadonlySet<string> };
  sent: { includes: ReadonlySet<string>; exclusions: ReadonlySet<string> };
}

/** fileSetEditorKey is the remount key for FileSetFoldersEditor: set id, path
 *  and whether a selection is stored. The editor seeds its state from the
 *  mount-time props and never re-syncs, so after a path edit, which clears the
 *  selection on the server, it has to remount; otherwise its next PATCH would
 *  be refused against the new root or bring the cleared entries back. A changed
 *  selection keeps the key, so a list refetch leaves the expansion, the browse
 *  cache and a PATCH in flight alone. */
export function fileSetEditorKey(set: FileSetView): string {
  return `${set.id}:${set.path}:${set.selectedPaths ? "set" : "null"}`;
}

// FileSetFoldersEditor puts the SelectionTree over the set's one root, as a
// disclosure on the set card. It stays out of FileSetDialog, whose full-set
// PATCHes would race the live save queue, and leaves out the container extras
// (CACHEDIR switch, custom rows, Reset). Exported for the page's dom tests.
export function FileSetFoldersEditor({
  set,
  hostMountRoot,
  t,
  anomaly,
  anomalyEnabled = false,
}: {
  set: FileSetView;
  hostMountRoot: string;
  t: T;
  /** What anomaly detection knows about this set, for its own sensitivity
   *  and notification setting under the tree. */
  anomaly?: AnomalyItem;
  anomalyEnabled?: boolean;
}) {
  const regionId = useId();
  const [open, setOpen] = useState(false);
  const noPath = set.path === "";
  // The set's resolved host path, cleaned on both segments (browseRelToHost is
  // the exact inverse of the browse prefix swap the tree performs below).
  const root = noPath ? "" : browseRelToHost(set.path, hostMountRoot);
  // Includes and exclusions in host paths. A set without a stored selection is
  // seeded with an include of its root, so the root renders checked and the
  // preview reads "1 path", which is what its backup covers; nothing is
  // written until the first toggle. The seed is read once per mount, and
  // fileSetEditorKey remounts the editor when it has to change.
  const seed = splitFlatSet(noPath ? [] : (set.selectedPaths ?? [root]));
  const [includes, setIncludes] = useState<Set<string>>(seed.includes);
  const [exclusions, setExclusions] = useState<Set<string>>(seed.exclusions);
  // The save queue reads the selection through this ref; applyMirror keeps it
  // in step with the state the tree renders.
  const mirrorRef = useRef<{ inc: Set<string>; exc: Set<string> }>({ inc: seed.includes, exc: seed.exclusions });
  // Per-row busy and shake state keyed by host path, as in FoldersEditor.
  // blockedPath is the row whose last toggle was refused, for the warn line.
  const [rowBusy, setRowBusy] = useState<Record<string, boolean>>({});
  const [rowShake, setRowShake] = useState<Record<string, number>>({});
  const [blockedPath, setBlockedPath] = useState<string | null>(null);
  // Listings cached for the editor's lifetime, so closing the disclosure
  // keeps them.
  const browseCache = useRef(new Map<string, Promise<BrowseResponse>>());
  const { push } = useToast();
  // A one-deep PATCH queue. While an attempt is in flight, further toggles
  // only update the selection and mark the queue dirty; when it settles, one
  // follow-up sends the latest full list. A slow or failing save can never
  // overwrite a newer toggle.
  const queueRef = useRef<{ inFlight: boolean; dirty: boolean }>({ inFlight: false, dirty: false });
  const pendingDescRef = useRef<FileSetSaveDesc | null>(null);
  // Rows whose toggles are folded into the next attempt's body (an attempt
  // always carries the live mirror); each row's busy flag clears exactly when
  // the attempt that acknowledges its effect settles.
  const pendingRowsRef = useRef<Set<string>>(new Set());

  function applyMirror(inc: Set<string>, exc: Set<string>): void {
    mirrorRef.current = { inc, exc };
    setIncludes(inc);
    setExclusions(exc);
  }

  // Called after the selection has been updated, as in FoldersEditor.
  function scheduleSave(desc: FileSetSaveDesc): void {
    if (queueRef.current.inFlight) {
      queueRef.current.dirty = true;
      pendingDescRef.current = desc; // latest desc wins, matching the drain
      return;
    }
    pendingDescRef.current = desc;
    void attemptSave();
  }

  // The body is built from the current selection when the attempt starts, not
  // from desc.sent, which is stale once later toggles queued behind it. Every
  // selection PATCH goes through here, so no two run at once.
  async function attemptSave(): Promise<void> {
    queueRef.current.inFlight = true;
    const rows = [...pendingRowsRef.current];
    pendingRowsRef.current.clear();
    const desc = pendingDescRef.current;
    pendingDescRef.current = null;
    try {
      const live = mirrorRef.current;
      const r = await patchFileSet(set.id, { selectedPaths: toFlatList(live.inc, live.exc) });
      if (r.ok) {
        push(t("folders.saved"), "success");
      } else {
        // A coded empty-selection refusal also gets the client check's warn
        // line. The client check normally prevents it, but a concurrent
        // writer or a stale anchor can still cause one.
        push(r.error ?? t("settings.error"), "fail");
        if (desc) {
          if (r.code === "empty-selection") setBlockedPath(desc.node);
          revertFrom(desc);
        }
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      if (desc) revertFrom(desc);
    } finally {
      queueRef.current.inFlight = false;
      setRowBusy((b) => {
        const n = { ...b };
        for (const p of rows) n[p] = false;
        return n;
      });
      if (queueRef.current.dirty) {
        queueRef.current.dirty = false;
        if (pendingDescRef.current) void attemptSave();
      }
    }
  }

  // Undoes only the failed toggle on the current selection: removes what it
  // added and adds back what it removed, so a toggle made after it survives.
  function revertFrom(desc: FileSetSaveDesc): void {
    const live = mirrorRef.current;
    const inc = new Set(live.inc);
    const exc = new Set(live.exc);
    for (const p of desc.sent.includes) if (!desc.pre.includes.has(p)) inc.delete(p);
    for (const p of desc.pre.includes) if (!desc.sent.includes.has(p)) inc.add(p);
    for (const p of desc.sent.exclusions) if (!desc.pre.exclusions.has(p)) exc.delete(p);
    for (const p of desc.pre.exclusions) if (!desc.sent.exclusions.has(p)) exc.add(p);
    // A toggle queued behind this save was checked against a selection that
    // still had this attempt's include, so undoing it can leave none. The
    // server never stored that, so fall back to the includes it still has.
    if (inc.size === 0) {
      for (const p of desc.pre.includes) inc.add(p);
    }
    applyMirror(inc, exc);
    setRowShake((s) => ({ ...s, [desc.node]: (s[desc.node] ?? 0) + 1 }));
  }

  // Applies the toggle optimistically to the current selection (the ref, not
  // the state closure) and queues the save. A toggle that would leave the set
  // without includes never reaches the server; its warn line points to
  // deleting the set, since file sets have no Reset.
  function onToggle(hostPath: string): void {
    const pre = { includes: mirrorRef.current.inc, exclusions: mirrorRef.current.exc };
    const next = applyToggle(hostPath, pre.includes, pre.exclusions);
    const unchanged =
      next.includes.size === pre.includes.size &&
      [...next.includes].every((p) => pre.includes.has(p)) &&
      next.exclusions.size === pre.exclusions.size &&
      [...next.exclusions].every((p) => pre.exclusions.has(p));
    if (unchanged) return;
    if (next.includes.size === 0) {
      setBlockedPath(hostPath);
      setRowShake((s) => ({ ...s, [hostPath]: (s[hostPath] ?? 0) + 1 }));
      return;
    }
    setBlockedPath(null);
    applyMirror(next.includes, next.exclusions);
    setRowBusy((b) => ({ ...b, [hostPath]: true }));
    pendingRowsRef.current.add(hostPath);
    scheduleSave({
      node: hostPath,
      pre,
      sent: { includes: next.includes, exclusions: next.exclusions },
    });
  }

  // A set without a path has no tree; the card's noPathHint line stands in.
  // FileSetRow checks this too. The check comes after the hooks.
  if (noPath) return null;

  return (
    <div className="border-t border-carbon-border pt-3">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={regionId}
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-2 py-1 text-start text-sm text-carbon-textSub"
      >
        <span className="w-4 shrink-0 flex items-center justify-center" aria-hidden="true">
          <svg
            width="10"
            height="10"
            viewBox="0 0 12 12"
            fill="none"
            className={`transition-transform ${open ? "rotate-90" : "rtl:rotate-180"}`}
          >
            <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
          </svg>
        </span>
        {t("files.foldersToggle")}
      </button>
      {open && (
        <div id={regionId} role="region" aria-label={t("folders.title")} className="mt-2 flex flex-col gap-2">
          <p className="text-xs text-carbon-textMuted">{t("files.foldersHint")}</p>
          {/* One root: the resolved host path with an empty dest, so the row
              is labelled with the bare path. Its count comes from
              rootIncludeCount, the paths the next backup hands restic. */}
          <SelectionTree
            mounts={[{ source: root, dest: "", selected: true, isAppdata: false, reachable: true }]}
            customPaths={[]}
            includes={includes}
            exclusions={exclusions}
            hostSourceRoot={hostMountRoot}
            containerName={`fileset-${set.id}`}
            browseCache={browseCache.current}
            onToggle={onToggle}
            // customPaths is always [] so the remove chip can never render;
            // the prop stays required on SelectionTreeProps (container shape).
            onRemoveCustom={() => {}}
            busyPaths={new Set(Object.keys(rowBusy).filter((k) => rowBusy[k]))}
            shakeCounts={rowShake}
            blockedPath={blockedPath}
            // Points to deleting the set rather than to a Reset.
            blockedMessage={t("files.emptySelectionBlocked")}
          />
          <ItemAnomalySettings item={anomaly} enabled={anomalyEnabled} t={t} />
        </div>
      )}
    </div>
  );
}
