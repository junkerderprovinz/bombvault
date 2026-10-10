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

// `open` is controlled by the caller (ContainerRow's shared five-chip
// Selector strip) — see HooksEditor's own comment for the full "why" this and
// its three siblings dropped their own internal useState.
//
// The mounts/custom list IS the selection tree now. The
// editor holds the (includes, exclusions) mirror in HOST path space — the
// Phase 1 flat encoding's two classes — and every checkbox toggle runs the
// pure applyToggle reducer over it, then live-saves the whole flat list with
// selectionSource "tree" (D-03). All per-node display state is derived by
// SelectionTree from those two sets; this component never tracks checkedness
// per row.
// Exported for the SelectionTree dom harness (same precedent as
// ExcludesEditor below): the tree's integration tests render this editor
// against the mocked api client instead of a whole ContainerRow.

/** The (includes, exclusions) host-path mirror pair, shared by the paths-class
 *  descriptor below. */
interface MirrorSets {
  includes: ReadonlySet<string>;
  exclusions: ReadonlySet<string>;
}

/** What one queued container-PATCH save was initiated for (plan 02; generalized
 *  in the plan). The `cls` discriminates the two mutation classes the
 *  editor can owe the server:
 *
 *  - "paths": a backupPaths save. `pre`/`sent` carry the initiating mutation's
 *    effect so a FAILURE can revert by set-difference inverse (revertFrom
 *    below) instead of a captured snapshot — a snapshot would also undo newer
 *    toggles stacked behind the failed one. `structural` marks
 *    custom add/remove, which keep their historical toast-only failure path.
 *    `source` carries the SELECTION SOURCE: every tree mutation sets the
 *    literal "tree", keeping the Phase 1 empty-selection guard live; the
 *    reset descriptor (reset: true) carries NO source — its drain sends
 *    exactly {backupPaths: []}, which the strictly tree-source-gated guard
 *    passes by design, making the confirmed reset the ONE sanctioned exit to
 *    auto-detection. Stacked cases resolve through latest-descriptor-wins,
 *    with one deliberate exception (WR-01): a PENDING reset desc is sticky,
 *    so a toggle arriving behind it cannot displace it in scheduleSave.
 *
 *  - "caches": a per-root CACHEDIR.TAG flip (D-06, RESTIC-01). The drain
 *    always sends the FULL live map (the server replaces it wholesale), so
 *    the descriptor only needs the flipped key's pre/next for the failure
 *    revert — restore pre, and only when the live map still equals the
 *    attempted value (a newer flip on the same key survives). */
type SaveDesc =
  | {
      cls: "paths";
      node: string;
      pre: MirrorSets;
      sent: MirrorSets;
      structural: boolean;
      source?: "tree";
      reset?: true;
      /** Host path of a custom root this save REMOVES. Its CACHEDIR.TAG entry
       *  is dropped only once the removal has actually landed - see the ok
       *  branch in attemptSave. */
      removedRoot?: string;
    }
  | {
      cls: "caches";
      node: string;
      caches: { path: string; pre: boolean; next: boolean };
    };

/** Busy/shake map key for the reset control (D-05). Not a host path, so it
 *  can never collide with a tree row's key in the same maps. */
const RESET_ROW_KEY = "__resetSelection__";

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
  /** Unix seconds of the container's last successful backup, null when none
   *  exists (Container.lastBackup verbatim). The D-02 narrowing gate: a
   *  narrowing selection only warns when there is at least one prior
   *  snapshot whose scope the narrowing changes — with no backup ever run,
   *  nothing has been captured under the wider selection to communicate
   *  about. Optional only so the dom harnesses can omit it; the production
   *  caller (ContainerRow) always passes container.lastBackup. */
  lastBackup?: number | null;
  /** Passed through to SelectionTree's viewportClassName: the stacked
   *  detail renders the tree at natural height so the page owns
   *  scrolling instead of an inner clamp-height scrollbox. Optional; every
   *  existing mount (desktop ContainerRow) omits it and keeps the clamp. */
  treeViewportClassName?: string;
}) {
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(false);
  const [mounts, setMounts] = useState<MountInfo[]>([]);
  // The selection mirror, host form: bare includes and "!"-class exclusions.
  // Includes = selected mount sources ∪ custom paths; exclusions = the stored
  // branches the mounts response has served since Phase 1 (dormant ones
  // included — they ARE D-01's remembered partial).
  const [includes, setIncludes] = useState<Set<string>>(new Set());
  const [exclusions, setExclusions] = useState<Set<string>>(new Set());
  const [custom, setCustom] = useState<CustomPath[]>([]);
  // The folder picker works in paths relative to the host mount (like File Sets);
  // browseValue stages one pick before it is translated to a host path and added.
  const [browseValue, setBrowseValue] = useState("");
  const [hostMountRoot, setHostMountRoot] = useState("/host/user");
  const [hostSourceRoot, setHostSourceRoot] = useState("/mnt");
  const { push } = useToast();
  // Live-save conversion (jdp, live review — see HooksEditor's own header
  // comment for the full "why" across all four editors): a tree checkbox is
  // a discrete pick, the SAME shape SettingsPage's toggleDomainEnabled
  // already established for "flip one thing, persist immediately, revert +
  // `.glim-shake` on failure" — rowBusy/rowShake below are that same per-key
  // busy/shake map, now keyed by the toggled node's HOST path. Adding/removing
  // a CUSTOM path is a structural list edit instead (closer to Settings.tsx's
  // registryAuths row add/remove), so it saves immediately too but withOUT
  // revert/shake — see addCustom/removeCustomPath's own comments below.
  const [rowBusy, setRowBusy] = useState<Record<string, boolean>>({});
  const [rowShake, setRowShake] = useState<Record<string, number>>({});
  // A React-state MIRROR of queueRef.current.inFlight, so
  // the Save bar (rendered by the parent, outside this component) can re-render
  // when a drain starts and settles. The queue itself stays ref-driven: this
  // flag is publish-only and never read by the save logic.
  const [, setQueueBusy] = useState(false);
  // The path whose last toggle was refused client-side for emptying the
  // selection; SelectionTree renders the inline warn line under that row.
  const [blockedPath, setBlockedPath] = useState<string | null>(null);
  // The tree's interaction mode derives from pointer capability, never from
  // viewport width: a landscape phone (>=48rem, the width-only chrome switch
  // keeps the desktop Sidebar there) still has a coarse primary pointer and
  // gets the touch tree (tap = check),
  // while a hybrid touchpad laptop stays on the pointer tree. jsdom answers
  // no coarse-pointer query, so the hook's false default keeps every existing
  // editor harness on the pointer mode it was written against.
  const coarsePointer = useIsCoarsePointer();
  // Editor-lifetime listings cache: survives
  // section close because this component stays mounted above its null
  // return, dies with the page — exactly the panel-lifetime scope the tree
  // is allowed to remember listings for. Plain Map, no state library.
  const browseCache = useRef(new Map<string, Promise<BrowseResponse>>());
  // The one-deep serialized PATCH queue: every
  // backupPaths save funnels through ONE
  // attempt at a time. While an attempt is in flight a further toggle just
  // updates the mirror and marks the queue dirty; when the attempt resolves,
  // a single drain sends the LATEST full flat list. A slow or failing save
  // can therefore never clobber a newer toggle, and a burst collapses to one
  // draining request.
  //
  // The queue also serializes the post-reset reload:
  // the `reload` flag below is set by a successful reset drain and consumed
  // by the finally chain only when no drain is owed, so the refetch GET
  // always starts with the queue idle and can never race (and locally
  // clobber, via its apply) a mutation stacked during the reset PATCH's
  // flight.
  //
  // The queue reads the mirror through a REF, not the state closure: two
  // rapid toggles inside one React batch must each see their predecessor's
  // effect, and batched setIncludes calls are not visible to the second
  // call's closure.
  const mirrorRef = useRef<{ inc: Set<string>; exc: Set<string> }>({ inc: new Set(), exc: new Set() });
  const queueRef = useRef<{ inFlight: boolean; dirty: boolean; reload: boolean }>({ inFlight: false, dirty: false, reload: false });
  // Descs of the mutations that most recently marked each CLASS dirty — the
  // failure-revert recipes (and reset flag) for the drain attempt they cause.
  // Per-class because a drain can owe paths AND caches at once while each
  // class's latest desc is the only revert recipe it needs. Empty while idle.
  const pendingDescsRef = useRef<{ paths?: Extract<SaveDesc, { cls: "paths" }>; caches?: Extract<SaveDesc, { cls: "caches" }> }>({});
  // Classes the next drain owes the server. While an attempt is in flight a
  // mutation of class X marks X owed; the drain composes ONE body carrying
  // exactly these classes (T-03-07: never two concurrent PATCHes, never a
  // class the drain does not owe).
  const owedRef = useRef<Set<"paths" | "caches">>(new Set());
  // Rows whose toggles are folded into the next attempt's body. An attempt
  // always carries every not-yet-acknowledged toggle (it sends the live
  // mirror), so each row's busy flag clears exactly when the attempt that
  // acknowledges its effect settles.
  const pendingRowsRef = useRef<Set<string>>(new Set());
  // D-06 (RESTIC-01): the per-root CACHEDIR.TAG map in HOST path form —
  // server truth at load, optimistically flipped per save, reverted on
  // failure. Ref-and-state mirror pair for the same reason as mirrorRef: the
  // queue must read the LIVE map through a ref, the tree renders the state.
  const [excludeCaches, setExcludeCaches] = useState<Record<string, boolean>>({});
  const cachesRef = useRef<Record<string, boolean>>({});
  // The narrowing-note
  // baseline: the include count of the last state the SERVER acknowledged
  // (the served selection at load, then every ok save's attempted count).
  // Event-driven, not derived: the note fires when an acknowledged attempt
  // carried FEWER includes than this, and the comparison is against the
  // last-SAVED count — never a pre-mutation count — so a burst collapsed by
  // the queue nets correctly (stacked toggles must not
  // each fire their own comparison against the same stale baseline).
  const lastSavedCountRef = useRef(0);
  // The note itself. Transient editor-session state, deliberately NOT
  // persisted: no server-side include-count history exists to restore it
  // from, and reopening the section re-derives truth from the served state.
  const [narrowed, setNarrowed] = useState(false);
  // The reset confirm names both consequences in its message, so the dialog
  // carries the weight and the trigger stays neutral.
  const { confirm, confirmDialog } = useConfirm();

  // Closing the section clears the note: the editor session it belongs to is
  // over, and a reopened section starts from the served state again.
  useEffect(() => {
    if (!open) setNarrowed(false);
  }, [open]);

  // The single mirror-write helper: every mutation (load, toggle, custom
  // add/remove) lands here so the ref the queue reads and the state the tree
  // renders can never drift apart.
  function applyMirror(inc: Set<string>, exc: Set<string>): void {
    mirrorRef.current = { inc, exc };
    setIncludes(inc);
    setExclusions(exc);
  }

  // Same contract for the CACHEDIR map (D-06): one write helper so the queue's
  // ref and the tree's state can never drift apart.
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
          // The includes derive from mount rows AND custom paths together —
          // one Set, because the wire list carries both classes flat.
          const inc = new Set([
            ...ms.filter((m) => m.selected && m.reachable).map((m) => m.source),
            ...(r.custom ?? []).map((c) => c.path),
          ]);
          const exc = new Set(r.excluded ?? []);
          applyMirror(inc, exc);
          // The served selection IS the last-saved state — the D-02 baseline
          // starts here, and the post-reset refetch re-runs this whole block
          // (setLoaded(false) below) so a reset re-baselines too.
          lastSavedCountRef.current = inc.size;
          // D-06: the served CACHEDIR map is the stored truth (the server
          // always sends an object; the ?? {} is fixture tolerance only).
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

  // Queue entry point — called AFTER the mirror has been updated. If an
  // attempt is in flight, the mutation's effect rides the next drain: mark
  // dirty and remember its class's desc (latest desc per class wins, matching
  // the latest-state drain — EXCEPT a pending reset, which is sticky, see
  // WR-01 below). Otherwise the mutation becomes the in-flight attempt
  // itself.
  function scheduleSave(desc: SaveDesc): void {
    if (queueRef.current.inFlight) {
      queueRef.current.dirty = true;
      owedRef.current.add(desc.cls);
      // Discriminant-narrowed writes: a dynamic [desc.cls] index would lose
      // the cls-to-shape correlation TypeScript needs here.
      if (desc.cls === "paths") {
        // WR-01: a confirmed reset descriptor is STICKY. Overwriting it with
        // a later toggle's desc would make the drain send the live pre-reset
        // list under "tree" — the confirmed destructive reset would silently
        // never reach the server while the toggle's drain still toasts the
        // ordinary "Saved". A toggle stacked behind a PENDING reset is
        // therefore superseded: it still marks the class owed (the drain must
        // fire) and still flips the mirror optimistically, but the reset body
        // is what goes out, and the ok refetch re-derives the whole editor
        // from the served auto-detected state — a toggle's effect is defined
        // relative to that post-reset state, so its optimistic flip simply
        // reverts when the served state lands (re-click after it does). A
        // second reset desc may still replace it (reset replaces reset).
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

  // One save attempt. The body is composed at attempt start from the LIVE
  // mirrors — never from desc.sent, which is already stale when later
  // mutations stacked behind it — and carries ONLY the classes this attempt
  // owes. Every container PATCH the editor sends comes through here as a
  // single fetchJSON call, so no two saves are ever concurrent — not even a
  // CACHEDIR flip riding behind a selection save (T-03-07).
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
      // A reset drain (WR-04): the confirmed reset clears BOTH remembered
      // per-root classes — the selection AND the CACHEDIR map. Sending
      // excludeCaches: {} with the empty backupPaths is what keeps a stored
      // toggle keyed by a root that disappears in the reset (a standalone
      // custom row, gone once the selection becomes auto-detected) from
      // surviving invisibly: anyRootExcludeCaches reads the stored map
      // regardless of whether the root still renders, so an orphaned true
      // would keep --exclude-caches firing with no switch left to turn it
      // off. The empty-selection guard only gates on the literal "tree"
      // source, which the reset never sends, so the added field keeps the
      // sanctioned no-source reset shape.
      const isReset = owed.has("paths") && pathsDesc?.reset === true;
      if (owed.has("paths")) {
        // D-05 (INTEG-04): a reset drain sends EXACTLY {backupPaths: []} with
        // NO selectionSource — the Phase 1 empty-selection guard is strictly
        // gated on the literal "tree", so this is the one sanctioned shape
        // that passes it back into auto-detection. Every other drain carries
        // the live flat list under the "tree" source. Stacking semantics
        // around a reset split by window (WR-01): while the reset is PENDING
        // its desc is sticky in scheduleSave, so the drain sends this reset
        // body and the stacked toggle is superseded by the ok refetch; once
        // the reset's own drain has STARTED (its desc consumed here) a later
        // toggle stacks normally and its drain re-sends the live NON-empty
        // list — latest-intent-wins over the just-landed auto-detection,
        // which the guard never bites on and the post-reset reload then re-serves.
        if (isReset) {
          body.backupPaths = [];
          body.excludeCaches = {};
        } else {
          body.backupPaths = toFlatList(live.inc, live.exc);
          if (pathsDesc?.source) body.selectionSource = pathsDesc.source;
        }
      }
      if (owed.has("caches") && !isReset) {
        // D-06: the whole live map, one class — the server replaces it
        // wholesale, so a flip and a later drain of the same class can never
        // lose each other's entries. Suppressed on a reset drain (see isReset
        // above): the reset's {} stands and a flip stacked behind it is
        // superseded — its optimistic state is re-derived by the ok reload,
        // the same reset-wins supersession a stacked paths toggle gets. On a
        // FAILED combined drain the flip's revert recipe still runs and lands
        // on server truth (the server kept the pre-reset map, whose entry for
        // the flipped key is exactly desc.caches.pre).
        body.excludeCaches = { ...cachesRef.current };
      }
      const r = await setContainerTargets(name, body);
      if (r.ok) {
        // Success feedback is per-class: a paths save announces itself with
        // the Saved toast; a caches-only save is quiet (the switch's own
        // state change IS the feedback, the live-save house shape).
        if (owed.has("paths")) {
          push(t("folders.saved"), "success");
          // The removal landed, so its now-orphaned CACHEDIR.TAG entry can go.
          // It drains as its own caches-class save, which is quiet by design.
          const gone = pathsDesc?.removedRoot;
          if (gone !== undefined && cachesRef.current[gone] !== undefined) {
            const wasOn = cachesRef.current[gone] ?? false;
            const nextCaches = { ...cachesRef.current };
            delete nextCaches[gone];
            applyCaches(nextCaches);
            scheduleSave({ cls: "caches", node: gone, caches: { path: gone, pre: wasOn, next: false } });
          }
          if (pathsDesc?.reset) {
            // Non-optimistic success: nothing was ever emptied locally, so
            // the served auto-detected state (mounts re-selected, remembered
            // exclusions, custom rows and cache toggles gone) must REPLACE
            // everything — the refetch re-runs the load block above,
            // re-seeding the mirror, the custom list, the caches map and the
            // lastSavedCount baseline together. The refetch does not
            // start here. Flag it and let the finally chain below issue it
            // only once every stacked drain has settled — a GET fired at this
            // spot would race the drain the finally starts for a mutation
            // stacked during THIS reset PATCH's flight, and a response
            // reflecting pre-drain server state would then clobber that
            // mutation's optimistic apply locally, silently losing the
            // user's click.
            queueRef.current.reload = true;
          } else {
            // D-02 narrowing gate (SELECT-03 second half): compare THIS
            // attempt's acknowledged include count against the last-SAVED
            // count, gated on container.lastBackup — a narrowing selection
            // only communicates when at least one prior snapshot exists whose
            // scope the narrowing changes. Attempted-at-drain-start, not the
            // mirror at settle time: mutations stacked behind this save are
            // the NEXT attempt's comparison, never this one's.
            const attempted = live.inc.size;
            if (attempted < lastSavedCountRef.current && lastBackup !== null) setNarrowed(true);
            lastSavedCountRef.current = attempted;
          }
        }
      } else {
        // Server error text VERBATIM, coded envelope or not: the D-04
        // backstop (code "empty-selection") is unreachable while the client
        // block below exists and still lands here as defense-in-depth —
        // toast + revert. One toast per attempt: a single failed fetch
        // failed both classes it carried.
        push(r.error ?? t("settings.error"), "fail");
        if (owed.has("paths")) {
          if (pathsDesc?.reset) {
            // Failed reset: non-optimistic means nothing was mutated
            // locally, so there is no mirror to revert — the failure is the
            // toast plus the reset control's own shake. The selection stays
            // exactly as it was, remembered exclusions included.
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
      // `chained` must capture the drain decision BEFORE attemptSave() runs:
      // its first synchronous step consumes owedRef, so checking owedRef
      // after the call would always read empty and fire the reload below
      // while the chained drain is still in flight.
      let chained = false;
      if (queueRef.current.dirty) {
        queueRef.current.dirty = false;
        if (owedRef.current.size > 0) {
          chained = true;
          void attemptSave();
        }
      }
      // The post-reset reload rides the queue tail: issued only when
      // this finally did NOT chain a drain and nothing else is owed, so the
      // GET starts after every stacked drain has settled and its response
      // can only reflect final server state. When a drain WAS chained the
      // flag stays set and that drain's own finally re-reaches this check;
      // the reload keeps deferring while the user keeps stacking mutations,
      // which is the correct order (mutations settle, then state reloads).
      if (!chained && queueRef.current.reload && owedRef.current.size === 0) {
        queueRef.current.reload = false;
        setLoaded(false);
      }
    }
  }

  // Failure revert: un-apply the failed mutation's DELTA onto the live
  // mirror — delete what it added, re-add what it removed. That is
  // applyToggle's inverse computed as set differences, so a toggle that
  // happened AFTER the failed one (but before its save resolved) survives
  // untouched; restoring a captured pre-mutation snapshot here is the exact
  // bug, because the snapshot also undoes that newer toggle.
  function revertFrom(desc: Extract<SaveDesc, { cls: "paths" }>): void {
    const live = mirrorRef.current;
    const inc = new Set(live.inc);
    const exc = new Set(live.exc);
    for (const p of desc.sent.includes) if (!desc.pre.includes.has(p)) inc.delete(p);
    for (const p of desc.pre.includes) if (!desc.sent.includes.has(p)) inc.add(p);
    for (const p of desc.sent.exclusions) if (!desc.pre.exclusions.has(p)) exc.delete(p);
    for (const p of desc.pre.exclusions) if (!desc.sent.exclusions.has(p)) exc.add(p);
    // The D-04 floor again, because a revert can walk through it sideways. A
    // toggle stacked behind this save was checked against a mirror that still
    // carried this attempt's optimistic include; once that include is taken
    // back, the newer toggle's own removal can leave ZERO includes. The
    // chained drain then PATCHes that exclusions-only list under the "tree"
    // source, the server stores it (a non-empty list passes the empty-selection
    // guard), and every later backup succeeds capturing nothing while the first
    // one overwrites the last record of where the data was. The user sees a
    // fail toast followed by a green Saved.
    //
    // Fall back to this attempt's pre-state: the failed save never reached the
    // server, so that IS server truth, and the newer toggle is refused exactly
    // as the floor would have refused it had it been evaluated against the
    // truth instead of against an optimistic mirror. Same warn line as a
    // blocked click, so the refusal is not silent.
    if (inc.size === 0) {
      for (const p of desc.pre.includes) inc.add(p);
      setBlockedPath(desc.node);
    }
    applyMirror(inc, exc);
    setRowShake((s) => ({ ...s, [desc.node]: (s[desc.node] ?? 0) + 1 }));
  }

  // Caches-class failure revert (D-06): restore the flipped key to its stored
  // value — but ONLY when the live map still carries the attempted value. A
  // newer flip of the SAME key that stacked behind the failed save survives
  // untouched, the same newer-intent-survives discipline revertFrom applies to
  // paths (one class over).
  function revertCachesFrom(desc: Extract<SaveDesc, { cls: "caches" }>): void {
    if (cachesRef.current[desc.caches.path] !== desc.caches.next) return;
    const next = { ...cachesRef.current, [desc.caches.path]: desc.caches.pre };
    applyCaches(next);
    setRowShake((s) => ({ ...s, [desc.node]: (s[desc.node] ?? 0) + 1 }));
  }

  // One tree checkbox toggle — optimistic reducer apply over the LIVE mirror
  // (the ref, not the state closure; see the queue refs above), then the
  // queue. The D-04 pre-Phase-3 guard runs BEFORE anything else: a toggle
  // that would leave ZERO includes for the whole item never PATCHes — the
  // next includes set IS the whole item, because all mounts plus custom
  // paths live in this one mirror (D-02 single tree). The warn line routes
  // to Include in schedule, the honest way to back up nothing; full-deselect
  // semantics are Phase 3 (INTEG-04).
  function onToggle(hostPath: string): void {
    const pre = { includes: mirrorRef.current.inc, exclusions: mirrorRef.current.exc };
    const next = applyToggle(hostPath, pre.includes, pre.exclusions);
    // Defense-in-depth (review CR-01): a reducer no-op must never become a
    // save. If both sets round-trip identical, the toggle changed nothing —
    // return before the mirror apply, busy flag, and queue, so no request
    // leaves and no "Saved" toast claims one did. Equal sizes plus one-way
    // membership is set equality (subset of same cardinality).
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

  // Structural list add — queued save, no revert/shake on failure (same
  // shape as Settings.tsx's registryAuths row add/remove: the path stays in
  // the list either way, a failed save just gets picked up by the next edit
  // or a reload — see debouncedSave's own "no revert" comment for the
  // identical reasoning applied to a structural edit instead of a text
  // edit). It still goes through the queue so it can never run concurrently
  // with a toggle's save.
  function addCustom() {
    const raw = browseValue.trim();
    if (!raw) return;
    // The folder picker yields a path relative to the host mount; translate
    // it to the host path SetBackupPaths expects — through the SAME exported
    // translator the tree uses for its children, so a manually typed variant
    // ("appdata/plex/", "appdata//plex", "a/../appdata/plex") is path.Clean-ed
    // to the canonical form before it enters `custom` and `includes`. Raw
    // entry made the row fail partitionCustomPaths' cleaned membership test
    // and vanish from the tree while still counting toward the D-04 floor —
    // selected, invisible, and unremovable until a reload re-served it
    // cleaned (review WR-03); the duplicate guard missed spelling variants
    // too. An already-absolute path still passes through untranslated
    // (cleaned only), the established manual-fallback precedent.
    const p = browseRelToHost(raw, hostSourceRoot);
    // Already covered by an ancestor include: adding it changes nothing. The
    // server prunes a redundant descendant include (PruneMaximal), so the row
    // would count toward the mount's path total, survive until the next reload,
    // and then quietly disappear. Treated exactly like the literal duplicate
    // below - the staged pick stays in the input, which is the feedback.
    if (classifyNode(p, mirrorRef.current.inc, mirrorRef.current.exc) === "checked") return;
    if (custom.some((c) => c.path === p) || includes.has(p)) {
      // Duplicate: leave the staged pick IN the input (review WR-04). This
      // guard used to run AFTER setBrowseValue(""), so adding an
      // already-present path silently wiped the user's entry — no row
      // change, no toast, nothing to retry from. A "duplicate path" toast
      // was the review's alternative; it needs a brand-new folders.* key
      // across all 42 locales, parity churn not worth it for a guard this
      // rare — the kept text IS the feedback that the path is already in
      // the list.
      return;
    }
    setBrowseValue("");
    const nextCustom = [...custom, { path: p, exists: true }];
    const pre = { includes: mirrorRef.current.inc, exclusions: mirrorRef.current.exc };
    const nextIncludes = new Set(pre.includes);
    const nextExclusions = new Set(pre.exclusions);
    // A folder inside an already-excluded branch has to clear that exclusion
    // first, the same rule applyToggle applies when the tree's own checkbox is
    // clicked on an excluded node. Adding it while the exclusion stands saved a
    // no-op: the server pruned the redundant include, kept the exclusion, and
    // the backup argv still carried --exclude for the branch, which swallows
    // the folder - while the row counted toward the mount's path total and the
    // UI toasted Saved. The flat encoding has no way to say "exclude this
    // branch except this one folder" (an --exclude swallows everything below
    // it), so the only honest reading of "add this folder" is the one the tree
    // already uses: drop the exclusions that COVER it, deeper ones stay.
    for (const e of pre.exclusions) {
      if (isAtOrUnder(p, e)) nextExclusions.delete(e);
    }
    // Add the include unless an ancestor already covers the path, in which case
    // it is the redundant entry the server prunes.
    //
    // "unchecked" alone was too narrow, and that was a regression: classifyNode
    // also answers "mixed" when an include sits STRICTLY BELOW p. Adding a
    // folder that happens to be the parent of an existing include is the
    // opposite of redundant - nothing covers p from above, so PruneMaximal
    // would keep p and swallow the child. Skipping it sent the list out
    // unchanged, cleared the input, added a row and toasted Saved, while the
    // folder was never backed up.
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

  // Structural list remove — same queued-save-no-revert shape as addCustom
  // above.
  function removeCustomPath(path: string) {
    const nextCustom = custom.filter((x) => x.path !== path);
    const pre = { includes: mirrorRef.current.inc, exclusions: mirrorRef.current.exc };
    const nextIncludes = new Set(pre.includes);
    nextIncludes.delete(path);
    // The D-04 floor applies to the Remove chip too. It used to live only in
    // onToggle, so the identical end state - zero includes, one dormant
    // exclusion left over - was refused through the checkbox and waved through
    // here: the save goes out as an exclusions-only list, which the server
    // stores because it is not EMPTY, and from then on backups report success
    // while capturing nothing and the first run overwrites AppdataPaths, the
    // last record of where the data was. Blocking it keeps the one honest route
    // to backing nothing up the one the warn line names: leave the container
    // out of the schedule.
    if (nextIncludes.size === 0) {
      setBlockedPath(path);
      setRowShake((s) => ({ ...s, [path]: (s[path] ?? 0) + 1 }));
      return;
    }
    setBlockedPath(null);
    setCustom(nextCustom);
    applyMirror(nextIncludes, pre.exclusions);
    // The removed root's CACHEDIR.TAG entry goes with it, but only AFTER the
    // removal has actually landed. Nothing prunes that map - not the server's
    // setter, not SetBackupPaths - and the switch renders only for a root that
    // still has a row, so an orphan stayed ON with no control left to turn it
    // off: every backup kept running with --exclude-caches, skipping every
    // tagged directory under the roots that DID remain, while every switch on
    // screen read off.
    //
    // Tying it to the save's SUCCESS is the point. Scheduling it here as its own
    // class sent it as a second PATCH - the first scheduleSave starts its drain
    // synchronously, so a second class can never join it - and that second PATCH
    // landed even when the removal before it had failed. A structural save is
    // deliberately never reverted, so nothing undid it: the row came back on the
    // next reload with its switch silently flipped off.
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

  // D-05 (INTEG-04): Reset selection — the ONE sanctioned exit back to
  // auto-detection. Confirmed first (fail-tone dialog, both consequences in
  // the message: auto-detection returns AND remembered exclusions are gone,
  // plus the WR-04 caches clearing the body performs), then serialized
  // through the SAME one-deep queue as every toggle — a reset can never race
  // an in-flight toggle save, and a toggle stacked behind a reset simply
  // becomes the next drain with latest-intent-wins. Non-optimistic in BOTH
  // directions: the mirror is never emptied locally, so ok refetches (the
  // served state replaces everything) and failure leaves the editor exactly
  // as it was.
  async function onResetSelection(): Promise<void> {
    if (!(await confirm(t("folders.resetConfirm")))) return;
    setRowBusy((b) => ({ ...b, [RESET_ROW_KEY]: true }));
    pendingRowsRef.current.add(RESET_ROW_KEY);
    scheduleSave({
      node: RESET_ROW_KEY,
      // pre/sent describe a mirror delta, but a reset never applies one
      // locally — the fields exist because SaveDesc's toggle path demands
      // the shape; the reset branch in attemptSave never reads them.
      pre: { includes: mirrorRef.current.inc, exclusions: mirrorRef.current.exc },
      sent: { includes: new Set<string>(), exclusions: new Set<string>() },
      structural: false,
      reset: true,
      cls: "paths",
    });
  }

  // D-06 (RESTIC-01): flip one root's CACHEDIR.TAG entry. Same live-save
  // shape as a checkbox toggle — optimistic flip, one queued save, revert +
  // shake on failure, busy map keyed by the root's HOST path (so the switch
  // and the row's checkbox disable together while the root has an
  // unacknowledged mutation of either class). The scope is item-wide (the
  // flag compiles into the backup argv for the whole container); the
  // InfoBubble beside the switch says so.
  function onToggleCaches(hostPath: string, next: boolean): void {
    const pre = cachesRef.current[hostPath] === true;
    if (pre === next) return; // defense-in-depth: a no-op flip never saves
    applyCaches({ ...cachesRef.current, [hostPath]: next });
    setRowBusy((b) => ({ ...b, [hostPath]: true }));
    pendingRowsRef.current.add(hostPath);
    scheduleSave({ cls: "caches", node: hostPath, caches: { path: hostPath, pre, next } });
  }

  // Sub-include absorption (INTEG-01, D-02, RESEARCH Q1): the server files
  // every include that is not EXACTLY a mount root under custom[], so a
  // sub-include under a reachable mount arrives here as a custom row. Those
  // entries stay in the includes mirror (their mount then classifies mixed
  // via the whitelist start-state and the sub-include renders checked inside
  // the tree once browsed) but are filtered from the RENDERED custom rows —
  // one presentation of every path, never a duplicate level-1 row beside the
  // mount that already contains it. Only reachable mounts absorb: an
  // unreachable mount cannot be browsed, so its sub-includes keep standalone
  // rows. addCustom's duplicate guard above still checks the RAW custom list,
  // so an absorbed path cannot be re-added as a row either.
  const { standalone: standaloneCustom } = partitionCustomPaths(
    custom.map((c) => c.path),
    mounts.filter((m) => m.reachable).map((m) => m.source),
  );
  const standaloneSet = new Set(standaloneCustom);
  // A VANISHED sub-include keeps its row even when its mount would absorb it.
  // Absorption assumes the tree can show the path instead, and the tree builds
  // its children from browse listings - a folder that is no longer on disk is in
  // no listing, so it had no row, no child, and no warning anywhere, while the
  // mount row still counted it in its "{n} paths" line. Before the tree it
  // showed the issue-#115 "no data folder detected" warning. At run time the
  // path is dropped from the positionals and the backup is recorded a success,
  // and the empty-backup guard only speaks up once EVERY include has gone, so
  // this row is the only place the partial case can surface.
  const customRows = custom.filter((c) => standaloneSet.has(c.path) || !c.exists);

  if (!open) return null;

  return (
    <div className="mt-2 rounded-card bg-carbon-background p-3 flex flex-col gap-2">
      <p className="text-xs text-carbon-textMuted">{t("folders.hint")}</p>
      {/* A compose member's project folder is NOT in this list, and its absence
          would otherwise read as "nothing to back up". It is backed up once for
          the whole stack instead of once per service (issue #189), so it needs
          saying exactly where someone would look for it and not find it. */}
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

      {/* D-02: the mount rows and custom rows ARE the tree's level-1 items —
          rendered by SelectionTree with lazy children under each, per-node
          state derived from the (includes, exclusions) mirror. The row's
          shake nonce stays the "key = nonce" technique this app's
          shake-capable controls already use (ToggleRow's own
          `shakeNonce`-keyed Toggle is the precedent); the custom row's
          remove control keeps t("offsite.targets.remove"), an existing key
          already translated in all 42 locales. The Add control below stays
          OUTSIDE the tree: it is an input, not a selection row. */}
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
      {/* D-02 (SELECT-03 second half): the narrowing note — event-driven,
          gated on a prior backup, transient for the editor session. Placed
          directly under the tree so it reads as a consequence of the
          selection change above it, before the Add row. role="status" makes
          it a polite live region; text-statusWarn on a non-interactive <p>
          (the no-status-color-on-controls rule governs interactive
          elements). */}
      {narrowed && (
        <p role="status" className="text-xs text-statusWarn">
          {t("folders.narrowedNote")}
        </p>
      )}
      {/* Under 48rem this panel's column (~223px at a 390px viewport) is
          narrower than the browser field's minimum width plus the add
          control, so the horizontal row let the field's right edge run under
          the button (proven via
          elementFromPoint). Below that breakpoint the row therefore wraps:
          the field takes the full line and the add action drops onto its
          own, left-aligned; from 48rem up the horizontal items-end row is
          untouched. */}
      <div className="flex items-end gap-2 pt-1 max-md:flex-wrap">
        <div className="flex-1 min-w-0 max-md:min-w-full">
          <FolderBrowser
            label={t("folders.addCustom")}
            value={browseValue}
            hostMountRoot={hostMountRoot}
            onChange={setBrowseValue}
          />
        </div>
        {/* Square icon badge (icon-badge round, standing rule: every icon
            badge gets real hue integration + a hover tooltip carrying its
            old label). Colour-engine integration is the same already-
            verified mechanism the prior text-button version of this control
            used: no `hueIndex` needed, this
            panel already lives inside ContainerRow's own `.glim-hue`
            element, so Badge's `tone="active"` (icon-only → solid
            `bg-accent`/`text-accentContrast`, see Badge.tsx's own
            `isIconOnly && tone==="active"` branch) resolves to the row's
            own rainbow position via the ordinary CSS custom-property
            cascade, verified live via getComputedStyle against the real
            deployed container.
              `size="icon"` — the app's one square-icon-badge size (32px). The
            old `size="compact"` stage was 32px too, so this badge's rendered
            box is unchanged; only the token name moved, because `compact`
            existed solely to hold one arm of the role-based 28/32/36px split
            that jdp rejected (see Badge.tsx's "ONE SIZE FOR SQUARE ICON
            BADGES" block). The 32px value is still exactly right here for the
            reason it always was — this badge shares an `items-end` row with a
            FolderBrowser field that measures 32px live (`text-sm px-3 py-1.5`),
            it is simply no longer a number this call site owns. That shared
            row is the desktop presentation: under 48rem the row wraps (see
            the block directly above this control), so the badge's
            neighbourhood becomes the vertical one instead of the horizontal.
            `tip`
            carries the exact text this button showed before becoming
            icon-only. */}
        <Button
          label={t("folders.add")}
          labelKey="folders.add"
          glyph={<IconAdd />}
          tone="accent"
          onClick={addCustom}
        />
      </div>
      {/* D-05 (INTEG-04): Reset selection — the ONE sanctioned exit back to
          auto-detection. Neutral tone on purpose: the fail-weighted confirm
          dialog carries the destructive signal (
          dialog carries the weight, not the trigger). The keyed wrapper +
          glim-shake is the same nonce-remount technique the tree rows use,
          keyed by the reset control's own shake entry. */}
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
