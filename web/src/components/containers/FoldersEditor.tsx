import { useEffect, useRef, useState } from "react";
import { getContainerMounts, setContainerTargets, type ContainerTargetsBody } from "../../lib/api";
import type { AnomalyItem, MountInfo, CustomPath, BrowseResponse } from "../../lib/api";
import { applyToggle, browseRelToHost, classifyNode, isAtOrUnder, partitionCustomPaths, toFlatList } from "../../lib/selectionTree";
import { useIsCoarsePointer } from "../../lib/useMediaQuery";
import { SelectionTree } from "../SelectionTree";
import { FolderBrowser } from "../FolderBrowser";
import { useT } from "../../lib/i18n";
import { ItemAnomalySettings } from "../ItemAnomalySettings";
import { IconAdd } from "../Sidebar";
import { Button } from "../Button";
import { useConfirm } from "../../lib/useConfirm";
import { useToast } from "../../lib/toast";

type T = ReturnType<typeof useT>["t"];

/** The selection as two sets of host paths. */
interface MirrorSets {
  includes: ReadonlySet<string>;
  exclusions: ReadonlySet<string>;
}

/** What a queued save was started for. The class says which of the two things
 *  the editor can owe the server:
 *
 *  - "paths": a backupPaths save. `pre` and `sent` carry the mutation's effect
 *    so a failure can undo just that delta (see revertFrom); a snapshot would
 *    also undo newer toggles stacked behind the failed one. `structural` marks
 *    a custom add or remove, which is not reverted on failure. `source` is
 *    "tree" for every tree mutation, which keeps the server's empty-selection
 *    guard active. A reset carries no source and sends an empty list, the one
 *    shape that guard lets through, back to auto-detection.
 *
 *  - "caches": a flip of one root's CACHEDIR.TAG switch. The save always sends
 *    the full map, so the descriptor only needs the flipped key's values for
 *    the failure revert. */
type SaveDesc =
  | {
      cls: "paths";
      node: string;
      pre: MirrorSets;
      sent: MirrorSets;
      structural: boolean;
      source?: "tree";
      reset?: true;
      /** Host path of a custom root this save removes. Its CACHEDIR.TAG entry
       *  is dropped once the removal has landed. */
      removedRoot?: string;
    }
  | {
      cls: "caches";
      node: string;
      caches: { path: string; pre: boolean; next: boolean };
    };

/** Busy and shake map key for the reset control. Not a host path, so it
 *  cannot collide with a tree row's key. */
const RESET_ROW_KEY = "__resetSelection__";

// FoldersEditor chooses which of a container's folders are backed up. The
// mount rows and custom paths are the top level of one selection tree. The
// editor holds the selection as two sets of host paths (includes and
// exclusions), runs every checkbox toggle through applyToggle and saves the
// whole flat list at once. SelectionTree derives each node's state from the
// two sets. The caller controls `open`.
export function FoldersEditor({
  name,
  stack,
  open,
  t,
  lastBackup = null,
  treeViewportClassName,
  anomaly,
  anomalyEnabled = false,
}: {
  name: string;
  stack: string;
  open: boolean;
  t: T;
  /** What anomaly detection knows about this container, for its own
   *  sensitivity and notification setting. */
  anomaly?: AnomalyItem;
  anomalyEnabled?: boolean;
  /** Unix seconds of the container's last successful backup, null when there
   *  is none. A narrowed selection only warns when a snapshot exists whose
   *  scope the narrowing changes. */
  lastBackup?: number | null;
  /** Passed to SelectionTree as viewportClassName. The stacked detail renders
   *  the tree at natural height so the page scrolls, not an inner box. */
  treeViewportClassName?: string;
}) {
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(false);
  const [mounts, setMounts] = useState<MountInfo[]>([]);
  // The selection in host form. Includes are the selected mount sources and
  // the custom paths. Exclusions are the stored branches the mounts response
  // serves, dormant ones included: they are how a partial selection is
  // remembered.
  const [includes, setIncludes] = useState<Set<string>>(new Set());
  const [exclusions, setExclusions] = useState<Set<string>>(new Set());
  const [custom, setCustom] = useState<CustomPath[]>([]);
  // The folder picker works in paths relative to the host mount (like File Sets);
  // browseValue stages one pick before it is translated to a host path and added.
  const [browseValue, setBrowseValue] = useState("");
  const [hostMountRoot, setHostMountRoot] = useState("/host/user");
  const [hostSourceRoot, setHostSourceRoot] = useState("/mnt");
  const { push } = useToast();
  // Busy and shake state per row, keyed by the node's host path.
  const [rowBusy, setRowBusy] = useState<Record<string, boolean>>({});
  const [rowShake, setRowShake] = useState<Record<string, number>>({});
  // Set alongside queueRef.current.inFlight only to trigger a render when a
  // save starts and settles. The save logic never reads it.
  const [, setQueueBusy] = useState(false);
  // The path whose last toggle was refused client-side for emptying the
  // selection; SelectionTree renders the inline warn line under that row.
  const [blockedPath, setBlockedPath] = useState<string | null>(null);
  // The tree's interaction mode follows pointer capability, not viewport
  // width: a landscape phone is wide enough for the desktop layout but still
  // has a coarse pointer and gets the touch tree, while a touchpad laptop
  // stays on the pointer tree.
  const coarsePointer = useIsCoarsePointer();
  // Listings cache for the editor's lifetime. It survives closing the section,
  // because the component stays mounted above its null return.
  const browseCache = useRef(new Map<string, Promise<BrowseResponse>>());
  // Saves go through a one-deep queue, one attempt at a time. A toggle during
  // an attempt only updates the mirror and marks the queue dirty; when the
  // attempt resolves, one drain sends the latest full list. A slow or failing
  // save therefore cannot overwrite a newer toggle, and a burst collapses to
  // one request. `reload` is set by a successful reset and consumed only when
  // no drain is owed, so the refetch cannot race a mutation stacked during
  // the reset.
  //
  // The queue reads the mirror through a ref, not the state closure: two
  // toggles inside one React batch must each see the other's effect, and a
  // batched setState is not visible to the second call's closure.
  const mirrorRef = useRef<{ inc: Set<string>; exc: Set<string> }>({ inc: new Set(), exc: new Set() });
  const queueRef = useRef<{ inFlight: boolean; dirty: boolean; reload: boolean }>({ inFlight: false, dirty: false, reload: false });
  // The latest descriptor per class that marked it dirty: the revert recipe
  // and reset flag for the drain it causes. Per class, because a drain can
  // owe paths and caches at once.
  const pendingDescsRef = useRef<{ paths?: Extract<SaveDesc, { cls: "paths" }>; caches?: Extract<SaveDesc, { cls: "caches" }> }>({});
  // Classes the next drain owes the server. It sends one body carrying
  // exactly these.
  const owedRef = useRef<Set<"paths" | "caches">>(new Set());
  // Rows whose toggles ride the next attempt. An attempt sends the live
  // mirror, so it acknowledges all of them and their busy flags clear when it
  // settles.
  const pendingRowsRef = useRef<Set<string>>(new Set());
  // The per-root CACHEDIR.TAG map in host path form, flipped optimistically
  // and reverted on failure. A ref and state pair for the same reason as
  // mirrorRef.
  const [excludeCaches, setExcludeCaches] = useState<Record<string, boolean>>({});
  const cachesRef = useRef<Record<string, boolean>>({});
  // Include count of the last state the server acknowledged: the served
  // selection at load, then each successful save. The narrowing note compares
  // an acknowledged attempt against this and not against a pre-mutation
  // count, so a burst the queue collapsed is compared once.
  const lastSavedCountRef = useRef(0);
  // Not persisted: the server keeps no include-count history to restore it
  // from.
  const [narrowed, setNarrowed] = useState(false);
  // The reset confirm names both consequences in its message, so the dialog
  // carries the weight and the trigger stays neutral.
  const { confirm, confirmDialog } = useConfirm();

  // Closing the section clears the note: the editor session it belongs to is
  // over, and a reopened section starts from the served state again.
  useEffect(() => {
    if (!open) setNarrowed(false);
  }, [open]);

  // Every mirror write goes through here, so the ref the queue reads and the
  // state the tree renders cannot drift apart.
  function applyMirror(inc: Set<string>, exc: Set<string>): void {
    mirrorRef.current = { inc, exc };
    setIncludes(inc);
    setExclusions(exc);
  }

  // The same for the CACHEDIR map.
  function applyCaches(next: Record<string, boolean>): void {
    cachesRef.current = next;
    setExcludeCaches(next);
  }

  useEffect(() => {
    if (!open || loaded) return;
    setLoading(true);
    getContainerMounts(name)
      .then((r) => {
        if (r.ok) {
          const ms = r.mounts ?? [];
          setMounts(ms);
          // One set for mount rows and custom paths, because the wire list
          // carries both flat.
          const inc = new Set([
            ...ms.filter((m) => m.selected && m.reachable).map((m) => m.source),
            ...(r.custom ?? []).map((c) => c.path),
          ]);
          const exc = new Set(r.excluded ?? []);
          applyMirror(inc, exc);
          // The served selection is the last saved state. The refetch after a
          // reset runs this block again and so renews the baseline too.
          lastSavedCountRef.current = inc.size;
          applyCaches(r.excludeCaches ?? {});
          setCustom(r.custom ?? []);
          if (r.hostMountRoot) setHostMountRoot(r.hostMountRoot);
          if (r.hostSourceRoot) setHostSourceRoot(r.hostSourceRoot);
        } else {
          push(r.error ?? t("settings.error"), "fail");
        }
      })
      .catch((err) => {
        push(err instanceof Error ? err.message : t("settings.error"), "fail");
      })
      .finally(() => {
        setLoading(false);
        setLoaded(true);
      });
  }, [open, loaded, name, t, push]);

  // Called after the mirror has been updated. During an attempt the mutation
  // rides the next drain; otherwise it becomes the attempt.
  function scheduleSave(desc: SaveDesc): void {
    if (queueRef.current.inFlight) {
      queueRef.current.dirty = true;
      owedRef.current.add(desc.cls);
      // Narrowed by hand: a dynamic [desc.cls] index would lose the link
      // between class and shape that TypeScript needs.
      if (desc.cls === "paths") {
        // The latest descriptor wins, except over a pending reset. Replacing
        // a confirmed reset with a later toggle's descriptor would send the
        // pre-reset list, and the reset would never reach the server. The
        // toggle still marks the class owed and flips the mirror; the refetch
        // after the reset then replaces that flip with the served state.
        if (!pendingDescsRef.current.paths?.reset) pendingDescsRef.current.paths = desc;
      } else {
        pendingDescsRef.current.caches = desc;
      }
      return;
    }
    owedRef.current = new Set([desc.cls]);
    pendingDescsRef.current = desc.cls === "paths" ? { paths: desc } : { caches: desc };
    void attemptSave();
  }

  // One save attempt. The body is built from the live mirrors at the start,
  // not from desc.sent, which is stale once later mutations have stacked
  // behind it, and carries only the classes this attempt owes. Every PATCH
  // the editor sends comes through here, so no two saves run at once.
  async function attemptSave(): Promise<void> {
    queueRef.current.inFlight = true;
    setQueueBusy(true);
    const rows = [...pendingRowsRef.current];
    pendingRowsRef.current.clear();
    const owed = new Set(owedRef.current);
    owedRef.current.clear();
    const descs = pendingDescsRef.current;
    pendingDescsRef.current = {};
    const pathsDesc = descs.paths;
    const cachesDesc = descs.caches;
    try {
      const live = mirrorRef.current;
      const body: ContainerTargetsBody = {};
      // A reset clears both remembered per-root classes, the selection and the
      // CACHEDIR map. A stored switch for a root that disappears with the
      // reset (a standalone custom row) would otherwise keep --exclude-caches
      // on with no control left to turn it off.
      const isReset = owed.has("paths") && pathsDesc?.reset === true;
      if (owed.has("paths")) {
        // A reset sends an empty list without a selection source, the one
        // shape the server's empty-selection guard lets through. Every other
        // drain sends the live list under the "tree" source.
        if (isReset) {
          body.backupPaths = [];
          body.excludeCaches = {};
        } else {
          body.backupPaths = toFlatList(live.inc, live.exc);
          if (pathsDesc?.source) body.selectionSource = pathsDesc.source;
        }
      }
      if (owed.has("caches") && !isReset) {
        // The whole live map, which the server replaces as one. Skipped on a
        // reset, whose empty map stands: a flip stacked behind it is replaced
        // by the reload.
        body.excludeCaches = { ...cachesRef.current };
      }
      const r = await setContainerTargets(name, body);
      if (r.ok) {
        // A paths save toasts. A caches-only save is quiet, because the switch
        // changing state is the feedback.
        if (owed.has("paths")) {
          push(t("folders.saved"), "success");
          // The removal landed, so its orphaned CACHEDIR.TAG entry can go, as
          // a quiet caches save of its own.
          const gone = pathsDesc?.removedRoot;
          if (gone !== undefined && cachesRef.current[gone] !== undefined) {
            const wasOn = cachesRef.current[gone] ?? false;
            const nextCaches = { ...cachesRef.current };
            delete nextCaches[gone];
            applyCaches(nextCaches);
            scheduleSave({ cls: "caches", node: gone, caches: { path: gone, pre: wasOn, next: false } });
          }
          if (pathsDesc?.reset) {
            // Nothing was emptied locally, so the served auto-detected state
            // has to replace everything. The refetch is only flagged here and
            // issued by the finally below once every stacked drain has
            // settled: a GET fired at this point could answer with the state
            // before such a drain and overwrite the user's click.
            queueRef.current.reload = true;
          } else {
            // Compares the count this attempt sent, not the mirror at settle
            // time: mutations stacked behind this save belong to the next
            // attempt's comparison. Without a prior backup there is no
            // snapshot whose scope the narrowing changes.
            const attempted = live.inc.size;
            if (attempted < lastSavedCountRef.current && lastBackup !== null) setNarrowed(true);
            lastSavedCountRef.current = attempted;
          }
        }
      } else {
        // One toast per attempt: a single failed request failed both classes
        // it carried.
        push(r.error ?? t("settings.error"), "fail");
        if (owed.has("paths")) {
          if (pathsDesc?.reset) {
            // A reset mutates nothing locally, so there is no mirror to
            // revert and only the reset control shakes.
            setRowShake((s) => ({ ...s, [RESET_ROW_KEY]: (s[RESET_ROW_KEY] ?? 0) + 1 }));
          } else if (pathsDesc && !pathsDesc.structural) revertFrom(pathsDesc);
        }
        if (owed.has("caches") && cachesDesc) revertCachesFrom(cachesDesc);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      if (owed.has("paths")) {
        if (pathsDesc?.reset) {
          setRowShake((s) => ({ ...s, [RESET_ROW_KEY]: (s[RESET_ROW_KEY] ?? 0) + 1 }));
        } else if (pathsDesc && !pathsDesc.structural) revertFrom(pathsDesc);
      }
      if (owed.has("caches") && cachesDesc) revertCachesFrom(cachesDesc);
    } finally {
      queueRef.current.inFlight = false;
      setQueueBusy(false);
      setRowBusy((b) => {
        const n = { ...b };
        for (const p of rows) n[p] = false;
        return n;
      });
      // `chained` is decided before attemptSave() runs. The call's first
      // synchronous step consumes owedRef, so reading owedRef afterwards
      // would always find it empty and fire the reload below while the
      // chained drain is still in flight.
      let chained = false;
      if (queueRef.current.dirty) {
        queueRef.current.dirty = false;
        if (owedRef.current.size > 0) {
          chained = true;
          void attemptSave();
        }
      }
      // The reload after a reset waits for the queue to go idle, so its
      // response reflects the final server state. If a drain was chained the
      // flag stays set and that drain's finally comes back to this check.
      if (!chained && queueRef.current.reload && owedRef.current.size === 0) {
        queueRef.current.reload = false;
        setLoaded(false);
      }
    }
  }

  // Undoes the failed mutation's delta on the live mirror: delete what it
  // added, add back what it removed. A toggle made after the failed one
  // survives, which restoring a snapshot would not allow.
  function revertFrom(desc: Extract<SaveDesc, { cls: "paths" }>): void {
    const live = mirrorRef.current;
    const inc = new Set(live.inc);
    const exc = new Set(live.exc);
    for (const p of desc.sent.includes) if (!desc.pre.includes.has(p)) inc.delete(p);
    for (const p of desc.pre.includes) if (!desc.sent.includes.has(p)) inc.add(p);
    for (const p of desc.sent.exclusions) if (!desc.pre.exclusions.has(p)) exc.delete(p);
    for (const p of desc.pre.exclusions) if (!desc.sent.exclusions.has(p)) exc.add(p);
    // The revert can leave zero includes: a toggle stacked behind this save
    // was checked against a mirror that still had this attempt's optimistic
    // include. The chained drain would then store an exclusions-only list,
    // which the server accepts because it is not empty, and every later
    // backup would succeed while capturing nothing. So fall back to this
    // attempt's pre-state, which is what the server has, and show the same
    // warn line as a blocked click.
    if (inc.size === 0) {
      for (const p of desc.pre.includes) inc.add(p);
      setBlockedPath(desc.node);
    }
    applyMirror(inc, exc);
    setRowShake((s) => ({ ...s, [desc.node]: (s[desc.node] ?? 0) + 1 }));
  }

  // Restores the flipped key only while the live map still carries the
  // attempted value, so a newer flip of the same key survives.
  function revertCachesFrom(desc: Extract<SaveDesc, { cls: "caches" }>): void {
    if (cachesRef.current[desc.caches.path] !== desc.caches.next) return;
    const next = { ...cachesRef.current, [desc.caches.path]: desc.caches.pre };
    applyCaches(next);
    setRowShake((s) => ({ ...s, [desc.node]: (s[desc.node] ?? 0) + 1 }));
  }

  // A toggle that would leave no includes for the whole item is refused before
  // it reaches the mirror or the server. The warn line points to "Include in
  // schedule", the way to back up nothing.
  function onToggle(hostPath: string): void {
    const pre = { includes: mirrorRef.current.inc, exclusions: mirrorRef.current.exc };
    const next = applyToggle(hostPath, pre.includes, pre.exclusions);
    // A reducer no-op must not become a save, or a "Saved" toast would claim
    // a request that never left. Equal sizes plus one-way membership is set
    // equality.
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
      cls: "paths",
      node: hostPath,
      pre,
      sent: { includes: next.includes, exclusions: next.exclusions },
      structural: false,
      source: "tree",
    });
  }

  // A structural edit: saved through the queue so it cannot run alongside a
  // toggle's save, but not reverted on failure. The path stays in the list and
  // the next edit or a reload picks the failed save up.
  function addCustom() {
    const raw = browseValue.trim();
    if (!raw) return;
    // The picker yields a path relative to the host mount. It goes through
    // the translator the tree uses for its children, so a typed variant
    // ("appdata/plex/", "appdata//plex") is cleaned to the canonical form. A
    // raw entry would fail partitionCustomPaths' membership test and vanish
    // from the tree while still counting as an include. An absolute path is
    // cleaned but not translated.
    const p = browseRelToHost(raw, hostSourceRoot);
    // Already covered by an ancestor include. The server prunes a redundant
    // descendant, so the row would disappear on the next reload. The staged
    // pick stays in the input, which is the feedback.
    if (classifyNode(p, mirrorRef.current.inc, mirrorRef.current.exc) === "checked") return;
    if (custom.some((c) => c.path === p) || includes.has(p)) {
      // A duplicate also leaves the staged pick in the input as the feedback.
      return;
    }
    setBrowseValue("");
    const nextCustom = [...custom, { path: p, exists: true }];
    const pre = { includes: mirrorRef.current.inc, exclusions: mirrorRef.current.exc };
    const nextIncludes = new Set(pre.includes);
    const nextExclusions = new Set(pre.exclusions);
    // A folder inside an excluded branch clears the exclusions that cover it,
    // as applyToggle does for a click on an excluded node. The flat encoding
    // cannot say "exclude this branch except this folder", so with the
    // exclusion left standing the folder would be saved but never backed up.
    // Deeper exclusions stay.
    for (const e of pre.exclusions) {
      if (isAtOrUnder(p, e)) nextExclusions.delete(e);
    }
    // Add the include unless an ancestor already covers the path. "mixed"
    // counts as not covered: it means an include sits below p, and the server
    // would keep p and drop that child.
    if (classifyNode(p, nextIncludes, nextExclusions) !== "checked") nextIncludes.add(p);
    setCustom(nextCustom);
    applyMirror(nextIncludes, nextExclusions);
    scheduleSave({
      cls: "paths",
      node: p,
      pre,
      sent: { includes: nextIncludes, exclusions: nextExclusions },
      structural: true,
      source: "tree",
    });
  }

  // A structural edit like addCustom.
  function removeCustomPath(path: string) {
    const nextCustom = custom.filter((x) => x.path !== path);
    const pre = { includes: mirrorRef.current.inc, exclusions: mirrorRef.current.exc };
    const nextIncludes = new Set(pre.includes);
    nextIncludes.delete(path);
    // Removing the last include is refused like a toggle that would do it. An
    // exclusions-only list is not empty, so the server would store it and
    // backups would report success while capturing nothing.
    if (nextIncludes.size === 0) {
      setBlockedPath(path);
      setRowShake((s) => ({ ...s, [path]: (s[path] ?? 0) + 1 }));
      return;
    }
    setBlockedPath(null);
    setCustom(nextCustom);
    applyMirror(nextIncludes, pre.exclusions);
    // The removed root's CACHEDIR.TAG entry has to go as well: nothing on the
    // server prunes that map, and the switch only renders for a root that has
    // a row, so an orphan would keep --exclude-caches on with no control to
    // turn it off. It is dropped once this save has succeeded (see
    // attemptSave). As a save of its own it would land even when the removal
    // failed, and a structural save is never reverted.
    scheduleSave({
      cls: "paths",
      node: path,
      pre,
      sent: { includes: nextIncludes, exclusions: pre.exclusions },
      structural: true,
      source: "tree",
      removedRoot: cachesRef.current[path] !== undefined ? path : undefined,
    });
  }

  // Reset returns the item to auto-detection after a confirm that names the
  // consequences. It runs through the same queue as the toggles and is not
  // optimistic: the mirror is never emptied locally, so success refetches and
  // failure leaves the editor as it was.
  async function onResetSelection(): Promise<void> {
    if (!(await confirm(t("folders.resetConfirm")))) return;
    setRowBusy((b) => ({ ...b, [RESET_ROW_KEY]: true }));
    pendingRowsRef.current.add(RESET_ROW_KEY);
    scheduleSave({
      node: RESET_ROW_KEY,
      // A reset applies no mirror delta. The fields are here because the
      // type demands them; attemptSave's reset branch never reads them.
      pre: { includes: mirrorRef.current.inc, exclusions: mirrorRef.current.exc },
      sent: { includes: new Set<string>(), exclusions: new Set<string>() },
      structural: false,
      reset: true,
      cls: "paths",
    });
  }

  // Flips one root's CACHEDIR.TAG switch. The busy map is keyed by the root's
  // host path, so the switch and the row's checkbox disable together. The flag
  // applies to the whole container's backup, which the InfoBubble beside the
  // switch says.
  function onToggleCaches(hostPath: string, next: boolean): void {
    const pre = cachesRef.current[hostPath] === true;
    if (pre === next) return; // a flip to the current value is not a save
    applyCaches({ ...cachesRef.current, [hostPath]: next });
    setRowBusy((b) => ({ ...b, [hostPath]: true }));
    pendingRowsRef.current.add(hostPath);
    scheduleSave({ cls: "caches", node: hostPath, caches: { path: hostPath, pre, next } });
  }

  // The server files every include that is not exactly a mount root under
  // custom[], so a sub-include of a mount arrives as a custom row. Those stay
  // in the includes mirror but are left out of the rendered custom rows: the
  // tree shows the path inside its mount. Only reachable mounts absorb, since
  // an unreachable one cannot be browsed. addCustom's duplicate guard checks
  // the raw custom list, so an absorbed path cannot be added again as a row.
  const { standalone: standaloneCustom } = partitionCustomPaths(
    custom.map((c) => c.path),
    mounts.filter((m) => m.reachable).map((m) => m.source),
  );
  const standaloneSet = new Set(standaloneCustom);
  // A sub-include that is gone from disk keeps its row even when its mount
  // would absorb it. The tree builds its children from browse listings, so a
  // missing folder would have no row and no warning anywhere. The backup drops
  // the path and still records a success, which makes this row the only place
  // the partial case shows.
  const customRows = custom.filter((c) => standaloneSet.has(c.path) || !c.exists);

  if (!open) return null;

  return (
    <div className="mt-2 rounded-card bg-carbon-background p-3 flex flex-col gap-2">
      <p className="text-xs text-carbon-textMuted">{t("folders.hint")}</p>
      {/* A compose member's project folder is missing from this list, which
          would read as "nothing to back up". It is backed up once for the
          whole stack (issue #189), and this says so where someone would look
          for it. */}
      {stack !== "" && (
        <p className="text-xs text-carbon-textSub">
          {t("folders.stackNote").replace("{stack}", stack)}
        </p>
      )}
      {loading && <p className="text-xs text-carbon-textMuted">{t("common.loadingBackups")}</p>}
      {!loading && mounts.length === 0 && custom.length === 0 && (
        <p className="text-xs text-carbon-textMuted">{t("folders.empty")}</p>
      )}
      {!loading && <ItemAnomalySettings item={anomaly} enabled={anomalyEnabled} t={t} />}

      {!loading && (mounts.length > 0 || custom.length > 0) && (
        <SelectionTree
          mounts={mounts}
          customPaths={customRows}
          includes={includes}
          exclusions={exclusions}
          hostSourceRoot={hostSourceRoot}
          containerName={name}
          browseCache={browseCache.current}
          onToggle={onToggle}
          onRemoveCustom={removeCustomPath}
          excludeCaches={excludeCaches}
          onToggleCaches={onToggleCaches}
          busyPaths={new Set(Object.keys(rowBusy).filter((k) => rowBusy[k]))}
          shakeCounts={rowShake}
          blockedPath={blockedPath}
          interactionMode={coarsePointer ? "touch" : "pointer"}
          viewportClassName={treeViewportClassName}
        />
      )}
      {/* Directly under the tree, so it reads as a consequence of the
          selection change above it. */}
      {narrowed && (
        <p role="status" className="text-xs text-statusWarn">
          {t("folders.narrowedNote")}
        </p>
      )}
      {/* Under 48rem the panel's column is narrower than the browser field
          plus the add control, so the row wraps and the field takes the full
          line. The add control stays outside the tree because it is an input,
          not a selection row. */}
      <div className="flex items-end gap-2 pt-1 max-md:flex-wrap">
        <div className="flex-1 min-w-0 max-md:min-w-full">
          <FolderBrowser
            label={t("folders.addCustom")}
            value={browseValue}
            hostMountRoot={hostMountRoot}
            onChange={setBrowseValue}
          />
        </div>
        <Button
          label={t("folders.add")}
          labelKey="folders.add"
          glyph={<IconAdd />}
          tone="accent"
          onClick={addCustom}
        />
      </div>
      {/* Neutral tone: the confirm dialog carries the warning. The key
          remounts the wrapper so the shake replays. */}
      <div className="pt-1" key={rowShake[RESET_ROW_KEY] ?? 0}>
        <Button
          label={t("folders.resetSelection")}
          labelKey="folders.resetSelection"
          disabled={rowBusy[RESET_ROW_KEY] === true}
          className={rowShake[RESET_ROW_KEY] ? "glim-shake" : ""}
          onClick={() => void onResetSelection()}
        />
      </div>
      {confirmDialog}
    </div>
  );
}
