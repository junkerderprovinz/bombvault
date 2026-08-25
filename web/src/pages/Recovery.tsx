import { useCallback, useEffect, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { useT } from "../lib/i18n";
import { hueVars, rainbowAt } from "../lib/appearance";
import { RevealInput } from "../components/RevealInput";
import { useReveal } from "../lib/useReveal";
import { withLtrIsolates, FOREIGN_APPDATA_DEST_HINT_LTR_FRAGMENTS } from "../lib/ltrFragments";
import { StepCard, type StepState } from "../components/recovery/StepCard";
import { Badge } from "../components/Badge";
import { InfoBubble } from "../components/InfoBubble";
import { FolderBrowser } from "../components/FolderBrowser";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { CloudCard, RcloneCard, ToggleRow } from "./Settings";
import { Selector } from "../components/Selector";
import { RestoreAction } from "../components/restore/RestoreAction";
import { fireAndWaitRun } from "../lib/backupWatch";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import {
  discover,
  discoverVMs,
  discoverFiles,
  discoverAll,
  getSettings,
  putSettings,
  getCloud,
  getRclone,
  listContainers,
  listVMs,
  listFileSets,
  fileSetSnapshots,
  restore,
  restoreVM,
  restoreFileSet,
  restoreConfig,
  waitForAppBack,
  getVMSSH,
  downloadRecoveryKit,
  foreignOpen,
  foreignClose,
  foreignRestore,
  listForeignFiles,
  foreignContainerWarnings,
  type ForeignBindWarning,
  type Settings,
  type Container,
  type VM,
  type FileSetView,
  type FileEntry,
  type ForeignInventory,
  type ForeignItem,
} from "../lib/api";
import { SnapshotFileTree } from "../components/SnapshotFileTree";
import { useConfirm } from "../lib/useConfirm";
import { useToast } from "../lib/toast";

// classifyReadable's probe: discover() + discoverVMs() OPEN the encrypted repo
// (they read the mirrored, restic-encrypted definitions), so they are the
// cleanest "can BombVault read your backups?" check with no backend change:
//   - a wrong APP_KEY  -> the mapped "APP_KEY differs" error in {ok:false,error}
//   - a missing/empty repo -> {ok:true, discovered:0}
//   - a readable repo   -> {ok:true, discovered:>0}
// See the report notes for why the snapshot-list probe can't be used pre-discover
// (it needs a container name we don't have on a fresh install).
type DiscoverResult = Awaited<ReturnType<typeof discover>>;

function isKeyMismatch(err: string | undefined): boolean {
  return !!err && /APP_KEY/i.test(err);
}

// Shared mono text-input styling (off-site URLs, foreign location/key fields).
const offsiteInput =
  "rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono bv-field-focus";

// RestoreRow — a single discovered target (container or VM) with its latest
// snapshot and a per-item Restore button. The restore mechanics are the shared
// <RestoreAction> (the same control the Containers/VMs tabs use), so a recovery
// restore behaves identically to one launched from those tabs. The restore is
// IN PLACE and LEFT STOPPED (forceLeaveStopped): the recovery flow restores
// everything first, then you start them from the Containers/VMs tabs.
function RestoreRow({
  domain,
  name,
  displayName,
  lastBackup,
  t,
  otherActive,
  hueIndex,
}: {
  domain: "container" | "vm";
  /** Raw identifier — the ONLY value the restore action below may send to the
   *  backend. For VMs this MUST be VM.libvirtName, never VM.name (which is
   *  display-only on TrueNAS). Containers have no such split. */
  name: string;
  /** Display name shown in the row + the cancel-confirm text; falls back to
   *  name. */
  displayName?: string;
  lastBackup: number | null;
  t: ReturnType<typeof useT>["t"];
  otherActive: boolean;
  /** Rainbow position for this row — same `.glim-hue`-on-the-row-wrapper
   *  mechanism as ContainerRow/VMRow (see StepCard.tsx's own comment for the
   *  cascade reasoning): the shared RestoreAction's plain bg-accent button
   *  needs no changes of its own, it just inherits --accent/--focus-ring
   *  from this row once the wrapper below carries the class. Assigned from
   *  Recovery()'s page-flat `nextHue()` counter at the call site, one call
   *  per row, in render order. */
  hueIndex: number;
}) {
  // Latest-backup label — DISPLAY ONLY, read straight from the target list's own
  // lastBackup field (unix seconds). No per-row snapshot fetch: a discovered list
  // of N containers + M VMs would otherwise spawn N+M concurrent restic processes
  // just for this label. The restore itself resolves "latest" on the server.
  const snapLabel = lastBackup ? new Date(lastBackup * 1000).toLocaleString() : "";

  return (
    <div
      className="flex flex-col gap-1 py-2 border-b border-carbon-border last:border-0 glim-hue"
      style={hueVars(rainbowAt(hueIndex)) as CSSProperties}
    >
      <div className="flex items-center gap-3 text-sm">
        <span className="text-carbon-text font-medium flex-1 truncate">{displayName ?? name}</span>
        <span className="text-carbon-textMuted text-xs shrink-0">
          {snapLabel || t("containers.never")}
        </span>
      </div>
      {/* In-place restore, LEFT STOPPED (forceLeaveStopped): the recovery flow
          restores everything first, then you start them from the Containers/VMs
          tabs. source omitted => the backend-default repo. */}
      <RestoreAction
        domain={domain}
        name={name}
        displayName={displayName}
        snapshotId="latest"
        otherActive={{ active: otherActive }}
        successMessage={t("common.done")}
        requireConfirm={false}
        showLeaveStopped={false}
        forceLeaveStopped
        showBusyHint={false}
        showStartedHint={false}
        label={t("snapshots.restore")}
        t={t}
      />
    </div>
  );
}

// FileSetRecoveryRow — a discovered file set with a target-folder picker and a
// per-item Restore button. File sets rebuilt from `fileset:` snapshot tags carry
// NO source path (tags alone don't store it), so an in-place restore is
// impossible here — the restore always extracts into a folder the user picks
// (non-destructive, FolderBrowser convention). The newest snapshot is resolved
// AT CLICK TIME (the files restore endpoint takes a concrete hex id, no
// "latest" alias) so rendering N rows never spawns N restic processes.
function FileSetRecoveryRow({
  set,
  hostMountRoot,
  t,
  otherActive,
  hueIndex,
}: {
  set: FileSetView;
  hostMountRoot: string;
  t: ReturnType<typeof useT>["t"];
  otherActive: boolean;
  /** Same `.glim-hue`-on-the-row-wrapper mechanism as RestoreRow above (see
   *  its own comment) — this row's inline bg-accent Restore button inherits
   *  --accent/--focus-ring from the wrapper with no button-level change. */
  hueIndex: number;
}) {
  const [target, setTarget] = useState("");
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a failed
  // action toasts AND shakes its button — a bumped nonce keyed onto the Restore
  // button replays `.glim-shake` once per failure, same mechanism as
  // VMExportButton/ExportButton's shakeNonce in the Containers/VMs tabs.
  const [shake, setShake] = useState(0);

  const snapLabel = set.lastBackup ? new Date(set.lastBackup * 1000).toLocaleString() : "";

  // GlimStone follow-up pass (v8.0.0): the "done"/"fail" result below is a
  // genuinely one-shot completion notice — unlike VMBackupButton/BackupButton's
  // shared useBackupWatch hook (deliberately left elsewhere in this pass), this
  // row drives fireAndWaitRun directly with its own local state, the same
  // shape as Settings.tsx's already-migrated ReplicateNowButton/
  // TestConnectionButton, so it gets the same treatment.
  async function handleRestore() {
    if (target.trim() === "" || busy) return;
    setBusy(true);
    try {
      // Resolve the newest snapshot of this set now (tag-filtered server-side).
      const snaps = await fileSetSnapshots(set.id);
      const list = snaps.ok ? snaps.snapshots ?? [] : [];
      if (list.length === 0) {
        push(snaps.error ?? t("snapshots.none"), "fail");
        setShake((n) => n + 1);
        return;
      }
      const latest = list.reduce((a, b) => (new Date(a.time) > new Date(b.time) ? a : b));
      const res = await fireAndWaitRun({
        kind: "restore",
        matchRun: (r) => r.domain === "files" && r.target === set.name,
        start: () => restoreFileSet(set.id, latest.id, true, target.trim()),
      });
      if (res.ok) {
        push(t("common.done"), "success");
      } else {
        push(res.error ?? t("settings.error"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : String(err), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      className="flex flex-col gap-2 py-2 border-b border-carbon-border last:border-0 glim-hue"
      style={hueVars(rainbowAt(hueIndex)) as CSSProperties}
    >
      <div className="flex items-center gap-3 text-sm">
        <span className="text-carbon-text font-medium flex-1 min-w-0 truncate">{set.name}</span>
        <span className="text-carbon-textMuted text-xs shrink-0">
          {snapLabel || t("containers.never")}
        </span>
      </div>
      <FolderBrowser
        label={t("restore.targetPath")}
        value={target}
        hostMountRoot={hostMountRoot}
        onChange={setTarget}
      />
      <div className="flex items-center gap-3 flex-wrap">
        <button
          key={shake}
          onClick={() => void handleRestore()}
          disabled={busy || otherActive || target.trim() === ""}
          className={`inline-flex items-center gap-1.5 rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed${
            shake ? " glim-shake" : ""
          }`}
        >
          {busy && (
            <span
              className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
          )}
          {busy ? t("common.restoring") : t("snapshots.restore")}
        </button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Foreign-repo restore (#61) — "Restore from another BombVault repo".
//
// A clearly separated section: connect READ-ONLY to a DIFFERENT BombVault
// instance's repository (its own APP_KEY), browse the inventory, restore
// single items. Two hard rules distinguish it from the attach steps above:
//   1. NOTHING persists. The session lives server-side in memory (30-min TTL);
//      this card must NEVER call putSettings (the neighbouring connectPreview
//      deliberately does — that is the anti-pattern here).
//   2. foreignClose runs on unmount/leave and on disconnect, so the foreign
//      key does not linger server-side for the full TTL.
// ---------------------------------------------------------------------------

/** True when a foreign-restore error means the 30-min session lapsed — the
 *  remedy is always the same: reconnect (the card offers exactly that). */
function isForeignSessionGone(err: string | undefined): boolean {
  return !!err && /session/i.test(err) && /(expired|unknown)/i.test(err);
}

// One restorable foreign item: snapshot picker (default latest), a destination
// folder, and a Restore button driven by fireAndWaitRun on the recorded run —
// runs land with domain "container" | "vm" | "files" exactly like local ones.
//
// The destination folder is REQUIRED for two domains, for different reasons:
//   - files: a foreign file set has no trusted local source path, so it always
//     extracts into a folder the user picks.
//   - vms (#122): a cross-instance VM must NEVER reuse the source server's disk
//     paths (that wrote multi-GB images onto the destination host's RAM rootfs
//     and bricked it). The user chooses where the disks land; they are written
//     to <destination>/<vm-name>/ and the backend rewrites the libvirt XML to
//     match. Defaults to the local VM domains path; a foreign VM is restored
//     LEFT STOPPED so the operator can check it before starting it.
function ForeignItemRow({
  domain,
  item,
  session,
  hostMountRoot,
  existsLocally,
  collisionKnown,
  t,
  blocked,
  onBusyChange,
  onSessionGone,
  hueIndex,
}: {
  domain: "containers" | "vms" | "files";
  item: ForeignItem;
  session: string;
  hostMountRoot: string;
  /** Show the overwrite confirm before restoring (a real or unverifiable collision). */
  existsLocally: boolean;
  /** True only when a same-named local item is KNOWN to exist; false when the local
   *  inventory could not be read, so the confirm should say "could not verify". */
  collisionKnown: boolean;
  t: ReturnType<typeof useT>["t"];
  blocked: boolean;
  onBusyChange: (busy: boolean) => void;
  onSessionGone: () => void;
  /** Same `.glim-hue`-on-the-row-wrapper mechanism as RestoreRow/
   *  FileSetRecoveryRow above — this row's own inline bg-accent Restore
   *  button inherits --accent/--focus-ring from the wrapper. Assigned from
   *  ForeignRestoreCard's own `nextHue()` (the SAME counter passed down from
   *  Recovery(), continuing that one page-flat sequence). */
  hueIndex: number;
}) {
  const [snapshot, setSnapshot] = useState("latest");
  // VMs default the destination to the local VM domains path (subpath under the
  // host mount); this exact subpath resolves to the same folder the backend
  // would fall back to, so leaving it untouched matches the safe default. File
  // sets start blank (the user must pick a folder).
  const [target, setTarget] = useState(domain === "vms" ? "user/domains" : "");
  const needsTarget = domain === "files" || domain === "vms";
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a failed
  // action toasts AND shakes its button — a bumped nonce keyed onto the
  // Restore button replays `.glim-shake` once per failure, same mechanism as
  // VMExportButton/ExportButton's shakeNonce in the Containers/VMs tabs.
  const [shake, setShake] = useState(0);

  // Files domain only: restore the WHOLE set (default) or PICK a subfolder/file
  // subset of it (#123 — pull one stack out of a whole-appdata set). The subset
  // selection + its file tree are lazy: nothing is listed until the user switches
  // to "pick a subfolder".
  const [filesMode, setFilesMode] = useState<"whole" | "subset">("whole");
  const [foreignFiles, setForeignFiles] = useState<FileEntry[]>([]);
  const [filesLoading, setFilesLoading] = useState(false);
  const [filesError, setFilesError] = useState<string | null>(null);
  const [filesFilter, setFilesFilter] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const subsetActive = domain === "files" && filesMode === "subset";

  // Containers domain only (#125): a cross-instance container restore remaps appdata
  // onto a destination on THIS host. `overwrite` confirms writing into a non-empty
  // destination that may belong to a different container; `warnings` lists the
  // container's NON-appdata binds whose source pool this host lacks (appdata is
  // remapped automatically — these the operator fixes in the template).
  const [overwrite, setOverwrite] = useState(false);
  const [warnings, setWarnings] = useState<ForeignBindWarning[]>([]);

  // onSessionGone is an inline arrow at the call site, so its identity changes on
  // every parent re-render — and the parent re-renders on every /api/progress SSE
  // tick. Hold it in a ref so the file-listing effect below can call the latest
  // handler WITHOUT listing it as a dependency (else each SSE tick would wipe the
  // ticked selection and re-fetch the tree mid-pick).
  const onSessionGoneRef = useRef(onSessionGone);
  onSessionGoneRef.current = onSessionGone;

  // The recorded run's domain strings (see handleRuns): singular for
  // containers/VMs, "files" for file sets.
  const runDomain = domain === "containers" ? "container" : domain === "vms" ? "vm" : "files";
  // Newest-first for the picker; restic lists snapshots oldest-first.
  const snaps = [...item.snapshots].reverse();

  // Containers: fetch the cross-pool bind warnings once (best-effort; the restore
  // still guards the destination regardless). Read-only, session-scoped.
  useEffect(() => {
    if (domain !== "containers") return;
    let cancelled = false;
    foreignContainerWarnings(session, item.name)
      .then((res) => {
        if (cancelled) return;
        if (res.ok) setWarnings(res.warnings ?? []);
        else if (isForeignSessionGone(res.error)) onSessionGoneRef.current();
      })
      .catch(() => {
        /* non-fatal: the appdata remap + destination guard still protect the restore */
      });
    return () => {
      cancelled = true;
    };
  }, [domain, session, item.name]);

  // Lazily list the chosen snapshot's file tree for the subset picker; re-list
  // when the snapshot changes and clear any prior selection (it belonged to the
  // previous snapshot). Read-only session-scoped call (listForeignFiles).
  useEffect(() => {
    if (!subsetActive) return;
    let cancelled = false;
    setFilesLoading(true);
    setFilesError(null);
    setSelected(new Set());
    listForeignFiles(session, item.name, snapshot)
      .then((res) => {
        if (cancelled) return;
        if (res.ok) setForeignFiles(res.files ?? []);
        else setForeignFiles([]);
        if (!res.ok) {
          setFilesError(res.error ?? t("files.loadFailed"));
          if (isForeignSessionGone(res.error)) onSessionGoneRef.current();
        }
      })
      .catch((err) => {
        if (!cancelled) setFilesError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        if (!cancelled) setFilesLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [subsetActive, session, item.name, snapshot, t]);

  function toggleSelected(p: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(p)) next.delete(p);
      else next.add(p);
      return next;
    });
  }

  // GlimStone follow-up pass (v8.0.0): same reasoning as FileSetRecoveryRow
  // above — this row drives fireAndWaitRun directly with its OWN local state
  // (not the shared, deliberately-sticky useBackupWatch hook), so the "ok"/
  // "fail" result is a genuinely one-shot completion notice, now a toast.
  async function handleRestore() {
    if (busy || blocked) return;
    if (needsTarget && target.trim() === "") return;
    // A subset restore needs at least one ticked path (the whole-set restore
    // sends none).
    if (subsetActive && selected.size === 0) return;
    // Overwrite confirm BEFORE anything fires. A KNOWN same-named local item warns
    // that it will be overwritten; an unreadable local inventory instead says it
    // could not verify (it is not claiming the item exists).
    if (existsLocally) {
      const key = collisionKnown ? "recovery.foreignExistsConfirm" : "recovery.foreignUnverifiedConfirm";
      if (!(await confirm(t(key).replace("{name}", item.name)))) return;
    }
    setBusy(true);
    onBusyChange(true);
    try {
      const res = await fireAndWaitRun({
        kind: "restore",
        matchRun: (r) => r.domain === runDomain && r.target === item.name,
        start: () =>
          foreignRestore({
            session,
            domain,
            item: item.name,
            snapshot,
            confirm: true,
            // Send whatever destination the field holds; empty lets the backend use
            // its default (files require one, vms default to user/domains, containers
            // default to the restore folder / user/appdata).
            target: target.trim() || undefined,
            // Only the subset mode selects paths; the whole-set restore omits them.
            paths: subsetActive ? [...selected] : undefined,
            // Containers only: confirm overwriting a non-empty destination (#125).
            overwrite: domain === "containers" ? overwrite : undefined,
          }),
      });
      if (res.ok) {
        push(t("common.done"), "success");
      } else {
        push(res.error ?? t("settings.error"), "fail");
        setShake((n) => n + 1);
        if (isForeignSessionGone(res.error)) onSessionGone();
      }
    } catch (err) {
      push(err instanceof Error ? err.message : String(err), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
      onBusyChange(false);
    }
  }

  return (
    <div
      className="flex flex-col gap-2 py-2 border-b border-carbon-border last:border-0 glim-hue"
      style={hueVars(rainbowAt(hueIndex)) as CSSProperties}
    >
      <div className="flex items-center gap-3 text-sm flex-wrap">
        <span className="text-carbon-text font-medium flex-1 truncate">{item.name}</span>
        <select
          value={snapshot}
          onChange={(e) => setSnapshot(e.target.value)}
          disabled={busy}
          className="rounded-control bg-carbon-surface2 px-2 py-1.5 text-xs text-carbon-text bv-field-focus"
        >
          <option value="latest">{t("recovery.foreignLatest")}</option>
          {snaps.map((s) => (
            <option key={s.id} value={s.id}>
              {new Date(s.time).toLocaleString()} — {s.id.slice(0, 8)}
            </option>
          ))}
        </select>
      </div>
      {domain === "files" && (
        <div className="flex flex-col gap-2">
          {/* Whole set vs. a subfolder/file subset of it (#123). */}
          <div className="flex items-center gap-4 text-xs">
            <label className="inline-flex items-center gap-1.5 cursor-pointer">
              <input
                type="radio"
                name={`filesmode-${item.name}`}
                checked={filesMode === "whole"}
                onChange={() => setFilesMode("whole")}
                disabled={busy}
                className="accent-accent"
              />
              <span className="text-carbon-text">{t("recovery.foreignWholeSet")}</span>
            </label>
            {/* jdp live-review ("Info-Texte in i Infobubbles"): the
                foreignSubfolderHint <p> that appeared under this pair once
                "pick a subfolder" was selected explained what the subset mode
                DOES — permanent prose about this exact control, so it belongs
                on this control's own label. On the label rather than the mode
                block below because it is then readable BEFORE choosing the
                mode, which is when the explanation is actually useful. */}
            <label className="inline-flex items-center gap-1.5 cursor-pointer">
              <input
                type="radio"
                name={`filesmode-${item.name}`}
                checked={filesMode === "subset"}
                onChange={() => setFilesMode("subset")}
                disabled={busy}
                className="accent-accent"
              />
              <span className="text-carbon-text">{t("recovery.foreignPickSubfolder")}</span>
              <InfoBubble tip={t("recovery.foreignSubfolderHint")} />
            </label>
          </div>
          {subsetActive && (
            <>
              <SnapshotFileTree
                files={foreignFiles}
                loading={filesLoading}
                error={filesError}
                filter={filesFilter}
                onFilterChange={setFilesFilter}
                selected={selected}
                onToggle={toggleSelected}
                t={t}
              />
            </>
          )}
          <FolderBrowser
            label={t("recovery.foreignTargetFolder")}
            value={target}
            hostMountRoot={hostMountRoot}
            onChange={setTarget}
          />
        </div>
      )}
      {domain === "vms" && (
        <div className="flex flex-col gap-1.5">
          {/* jdp live-review ("Info-Texte in i Infobubbles"): the destination
              hint under this picker moves onto the picker's own label bubble,
              same as the connect step's location field above. */}
          <FolderBrowser
            label={t("recovery.foreignVMDest")}
            value={target}
            hostMountRoot={hostMountRoot}
            onChange={setTarget}
            placeholder="user/domains"
            hint={t("recovery.foreignVMDestHint")}
          />
        </div>
      )}
      {domain === "containers" && (
        <div className="flex flex-col gap-1.5">
          {/* Same move as the VM destination above, with one extra step: this
              hint names a literal `/mnt/zfs` pool path INSIDE the translated
              sentence, which needs bidi isolation or its leading `/` migrates
              to the wrong end of the path under RTL (see ltrFragments.tsx).
              An InfoBubble tip is a plain string that is ALSO the trigger's
              aria-label, so the `<span dir="ltr">` form withLtrFragments emits
              has nowhere to live here — `withLtrIsolates` applies the identical
              isolation with the U+2066/U+2069 characters instead, off the SAME
              fragment list, so the locale-parity guard still covers it. */}
          <FolderBrowser
            label={t("recovery.foreignAppdataDest")}
            value={target}
            hostMountRoot={hostMountRoot}
            onChange={setTarget}
            placeholder="user/appdata"
            hint={withLtrIsolates(
              t("recovery.foreignAppdataDestHint"),
              FOREIGN_APPDATA_DEST_HINT_LTR_FRAGMENTS
            )}
          />
          <label className="inline-flex items-center gap-1.5 text-xs cursor-pointer">
            <input
              type="checkbox"
              checked={overwrite}
              onChange={(e) => setOverwrite(e.target.checked)}
              disabled={busy}
              className="accent-accent"
            />
            <span className="text-carbon-text">{t("recovery.foreignOverwrite")}</span>
          </label>
          {warnings.length > 0 && (
            <div className="rounded-card bg-carbon-surface2 px-3 py-2 text-xs text-carbon-textMuted max-w-2xl">
              <p className="text-statusWarn">{t("recovery.foreignBindWarning")}</p>
              <ul className="mt-1 flex flex-col gap-0.5">
                {/* The key joins two free-form strings with a separator neither can
                    contain, so "a" + "b|c" and "a|b" + "c" can't collide. It MUST stay
                    the "\u0000" ESCAPE and never be re-typed as a literal NUL byte: a
                    raw 0x00 anywhere in this file makes ripgrep/grep/git classify the
                    WHOLE file as binary and return zero content lines for it, so every
                    repo-wide sweep silently skips Recovery.tsx. That already happened
                    once — the GlimStone form-engine confirm-dialog migration had to
                    hand-find this file's two "grep-invisible" call sites after the
                    sweep missed them. */}
                {warnings.map((wn) => (
                  <li key={wn.host + "\u0000" + wn.container} className="font-mono wrap-break-word text-start" dir="ltr">
                    {wn.host} → {wn.container}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
      <div className="flex items-center gap-3 flex-wrap">
        <button
          key={shake}
          onClick={() => void handleRestore()}
          disabled={
            busy ||
            blocked ||
            (needsTarget && target.trim() === "") ||
            (subsetActive && selected.size === 0)
          }
          className={`inline-flex items-center gap-1.5 rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed${
            shake ? " glim-shake" : ""
          }`}
        >
          {busy && (
            <span
              className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
          )}
          {busy ? t("common.restoring") : t("recovery.foreignRestore")}
        </button>
      </div>
      {confirmDialog}
    </div>
  );
}

// The whole foreign section: heading + two StepCards (connect, browse &
// restore). All session state is COMPONENT state — never Settings.
function ForeignRestoreCard({
  hostMountRoot,
  t,
  otherActive,
  nextHue,
}: {
  hostMountRoot: string;
  t: ReturnType<typeof useT>["t"];
  otherActive: boolean;
  /** The PARENT Recovery()'s own `nextHue()` counter, passed down as the
   *  function itself (not a single computed value): this card renders THREE
   *  of its own heading notches (the section h2 below + its two StepCards),
   *  so each needs its own call to keep continuing the same page-flat
   *  sequence in JSX order, exactly as if these three headings were inline
   *  in Recovery()'s own return. */
  nextHue: () => number;
}) {
  // Connect input. The backend only opens a LOCALLY MOUNTED repository, so the
  // location is always a folder under the host mount (e.g. a mounted share
  // holding the other server's backups) — no remote-URL / off-site option here.
  const [localPath, setLocalPath] = useState("");
  const [key, setKey] = useState("");
  const revealKey = useReveal();

  const [phase, setPhase] = useState<"idle" | "connecting" | "connected" | "error">("idle");
  const [connectError, setConnectError] = useState<string | null>(null);
  const [session, setSession] = useState<string | null>(null);
  const [inventory, setInventory] = useState<ForeignInventory | null>(null);
  // The 30-min server-side TTL lapsed mid-browse (a restore reported it):
  // surface it and offer a one-click reconnect with the kept inputs.
  const [sessionGone, setSessionGone] = useState(false);
  // Local container/VM names ("container:x" / "vm:y"), fetched at connect time
  // so each row knows whether a restore would overwrite something local.
  const [localNames, setLocalNames] = useState<Set<string>>(new Set());
  // Was the local container/VM inventory successfully read at connect time? When
  // FALSE (the fetch failed), the collision state is UNKNOWN — every foreign
  // container/VM then still prompts the overwrite confirm rather than silently
  // skipping it (fail safe: confirm when unknown, never overwrite silently).
  const [localKnown, setLocalKnown] = useState(true);
  const [busyRows, setBusyRows] = useState(0);
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a failed
  // action toasts AND shakes its button, layered ON TOP of this card's own
  // pre-existing sticky inline connectError box (kept — the scrubbed backend
  // message is worth reading, not just a toast ping). A bumped nonce keyed onto
  // the Connect button replays `.glim-shake` once per failure, same mechanism
  // as VMExportButton/ExportButton's shakeNonce.
  const [shake, setShake] = useState(0);

  // Ref-mirror of the session id so the unmount cleanup closes the CURRENT
  // session (an effect capturing `session` directly would close stale ids on
  // every change instead).
  const sessionRef = useRef<string | null>(null);
  sessionRef.current = session;
  useEffect(
    () => () => {
      // Leave/unmount: drop the session server-side (harmless if expired).
      // NOTE: nothing is persisted here — this card never calls putSettings.
      if (sessionRef.current) {
        foreignClose(sessionRef.current).catch(() => undefined);
      }
    },
    []
  );

  const location = localPath.trim();
  const canConnect = location !== "" && key.trim() !== "" && phase !== "connecting";

  const connect = useCallback(async () => {
    if (location === "" || key.trim() === "") return;
    setPhase("connecting");
    setConnectError(null);
    setSessionGone(false);
    // Replacing an open session: close the old one first (no dangling TTLs).
    if (sessionRef.current) {
      foreignClose(sessionRef.current).catch(() => undefined);
      setSession(null);
      setInventory(null);
    }
    try {
      const res = await foreignOpen(location, key.trim());
      if (!res.ok || !res.session) {
        const message = res.error ?? t("settings.error");
        setConnectError(message);
        setPhase("error");
        push(message, "fail");
        setShake((n) => n + 1);
        return;
      }
      // Read the LOCAL inventory BEFORE enabling the restore rows: which foreign
      // names already exist locally decides whether a restore shows the overwrite
      // confirm. Awaiting it here (rather than after phase "connected") means the
      // rows never render enabled with a stale/empty collision set. If the fetch
      // FAILS the collision state is UNKNOWN (localKnown=false) — every foreign
      // container/VM then still prompts the confirm (fail safe).
      const names = new Set<string>();
      let known = true;
      try {
        const [cs, vs] = await Promise.all([listContainers(), listVMs()]);
        // These endpoints answer HTTP 200 {ok:false} when docker/libvirt is
        // briefly unavailable (fetchJSON does not throw on that), so the ok flag
        // — not just a thrown error — decides whether the collision set is
        // trustworthy. An untrusted set forces the overwrite confirm (fail safe).
        if (!cs.ok || !vs.ok) {
          known = false;
        } else {
          for (const c of cs.containers ?? []) names.add(`container:${c.name}`);
          // ForeignItem.Name (below, item.name) is always the raw libvirt name
          // — it comes from parsing the foreign repo's restic tags
          // ("vm:"+rawName at backup time), never a friendly display name. The
          // local side of this collision check must match on the same raw
          // identifier (VM.libvirtName), not the display VM.name, or a
          // TrueNAS VM's real collision would go undetected.
          for (const v of vs.vms ?? []) names.add(`vm:${v.libvirtName}`);
        }
      } catch {
        known = false;
      }
      setLocalNames(names);
      setLocalKnown(known);
      setSession(res.session);
      setInventory(res.inventory ?? { containers: [], vms: [], fileSets: [] });
      setPhase("connected");
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setConnectError(message);
      setPhase("error");
      push(message, "fail");
      setShake((n) => n + 1);
    }
  }, [location, key, t, push]);

  const disconnect = useCallback(() => {
    if (sessionRef.current) {
      foreignClose(sessionRef.current).catch(() => undefined);
    }
    setSession(null);
    setInventory(null);
    setPhase("idle");
    setSessionGone(false);
  }, []);

  const onBusyChange = useCallback((busy: boolean) => {
    setBusyRows((n) => (busy ? n + 1 : Math.max(0, n - 1)));
  }, []);
  const rowBlocked = otherActive || busyRows > 0;

  const connectState: StepState =
    phase === "connected" ? "ok" : phase === "error" ? "bad" : "idle";
  const total = inventory
    ? inventory.containers.length + inventory.vms.length + inventory.fileSets.length
    : 0;
  const browseState: StepState = !session ? "idle" : sessionGone ? "warn" : total > 0 ? "ok" : "warn";

  const groups: { domain: "containers" | "vms" | "files"; label: string; items: ForeignItem[] }[] =
    inventory
      ? [
          { domain: "containers" as const, label: t("nav.containers"), items: inventory.containers },
          { domain: "vms" as const, label: t("nav.vms"), items: inventory.vms },
          { domain: "files" as const, label: t("nav.files"), items: inventory.fileSets },
        ].filter((g) => g.items.length > 0)
      : [];

  return (
    // gap-10 + pt-10, matching the page rhythm the parent wrapper now uses
    // (jdp: "Bitte machen wie sonst überall") — this section's two StepCards
    // are cards on the same page and can't sit at half the gap the six above
    // them use. `mt-2` is gone with it: the parent's own gap-10 already sets
    // the distance to the divider, and pt-10 sets the same 40px below it, so
    // the rule sits centred in one consistent break instead of 40px above /
    // 28px below.
    <div className="flex flex-col gap-10 border-t border-carbon-border pt-10">
      <div>
        {/* Task 5 (rule 11): page-level group heading, same Badge-in-<h2>
            treatment as Containers.tsx's StacksPanel `stack.title` heading.
            GlimStone follow-up pass ("half-overlap card notch"): `relative`
            added directly on this <h2> — no padding wraps it, so the h2
            itself is the right anchor; see Badge.tsx's badgeClassName
            comment.
            jdp live-review ("Info-Texte in i Infobubbles"): foreignIntro —
            this section's whole pitch, a permanent paragraph under the
            heading — is now the badge's own `onAccent` (i), the same fix
            Flash.tsx's and Config.tsx's card headings already carry. */}
        <h2 className="relative flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={nextHue()}>
            {t("recovery.foreignTitle")}
            <InfoBubble tip={t("recovery.foreignIntro")} onAccent />
          </Badge>
        </h2>
      </div>

      {/* Foreign step 1 — connect (read-only; nothing is saved). */}
      <StepCard n={1} title={t("recovery.foreignStepConnect")} state={connectState} hueIndex={nextHue()}>
        {/* Local mounted path only — the backend never opens a remote/off-site
            repo here, so the other server's backup share must be mounted on this
            host and pointed at below. */}
        {/* jdp live-review ("Info-Texte in i Infobubbles"): the standing hint
            under this picker is now the picker's OWN label bubble —
            FolderBrowser has carried a `hint` prop for exactly this since the
            same convention landed on Settings/Config, so this is a move, not
            new machinery. */}
        <FolderBrowser
          label={t("recovery.foreignLocation")}
          value={localPath}
          hostMountRoot={hostMountRoot}
          onChange={setLocalPath}
          hint={t("recovery.foreignLocationHint")}
        />

        <div className="flex flex-col gap-1">
          {/* Same fix one level down: the key field's own permanent hint <p>
              becomes the (i) on its label, matching how every labelled field
              in Settings.tsx/Config.tsx already carries its explanation. */}
          <label className="flex items-center gap-1 text-xs text-carbon-textSub">
            {t("recovery.foreignKey")}
            <InfoBubble tip={t("recovery.foreignKeyHint")} />
          </label>
          <RevealInput
            {...revealKey}
            value={key}
            spellCheck={false}
            autoComplete="off"
            onChange={(e) => setKey(e.target.value)}
            wrapperClassName="w-full"
            className={offsiteInput}
          />
        </div>

        <div className="flex items-center gap-3 pt-1 flex-wrap">
          <button
            key={shake}
            onClick={() => void connect()}
            disabled={!canConnect}
            className={`inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
              shake ? " glim-shake" : ""
            }`}
          >
            {phase === "connecting" && (
              <span
                className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
                style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
              />
            )}
            {phase === "connecting" ? t("recovery.foreignConnecting") : t("recovery.foreignConnect")}
          </button>
          {phase === "connected" && (
            <>
              <span className="text-sm text-statusOk">{t("recovery.foreignConnected")}</span>
              <button
                type="button"
                onClick={disconnect}
                className="text-xs text-carbon-textSub hover:text-carbon-text transition-colors"
              >
                {t("recovery.foreignClose")}
              </button>
            </>
          )}
        </div>
        {phase === "error" && connectError && (
          <div className="rounded-card bg-statusFailBgSoft px-3 py-2.5 text-xs text-statusFail leading-relaxed wrap-break-word">
            {connectError}
          </div>
        )}
      </StepCard>

      {/* Foreign step 2 — browse the inventory & restore single items. */}
      <StepCard n={2} title={t("recovery.foreignStepBrowse")} state={browseState} hueIndex={nextHue()}>
        {!session || !inventory ? (
          <p className="text-sm text-carbon-textMuted">{t("recovery.foreignNotConnected")}</p>
        ) : (
          <>
            {/* Session lapsed mid-browse (30-min TTL) — offer the reconnect. */}
            {sessionGone && (
              <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed flex items-center gap-3 flex-wrap">
                <span className="flex-1">{t("recovery.foreignExpired")}</span>
                <button
                  type="button"
                  onClick={() => void connect()}
                  className="rounded-control bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-xs text-carbon-text transition-colors"
                >
                  {t("recovery.foreignReconnect")}
                </button>
              </div>
            )}
            {total === 0 ? (
              <p className="text-sm text-statusWarn">{t("recovery.foreignEmpty")}</p>
            ) : (
              groups.map((g) => (
                <div key={g.domain} className="flex flex-col">
                  <span className="text-xs font-medium text-carbon-textSub pt-1 pb-1">{g.label}</span>
                  {g.items.map((item) => (
                    <ForeignItemRow
                      key={`${g.domain}:${item.name}`}
                      domain={g.domain}
                      item={item}
                      session={session}
                      hostMountRoot={hostMountRoot}
                      existsLocally={
                        // File sets restore into a chosen folder — they never
                        // overwrite a same-named local item, so no confirm. For
                        // containers/VMs, an UNKNOWN local inventory (fetch
                        // failed) counts as a possible collision → confirm.
                        g.domain !== "files" &&
                        (!localKnown ||
                          localNames.has(
                            (g.domain === "containers" ? "container:" : "vm:") + item.name
                          ))
                      }
                      collisionKnown={
                        // A real, verified collision (vs an unreadable inventory) —
                        // decides whether the confirm says "exists" or "could not verify".
                        g.domain !== "files" &&
                        localKnown &&
                        localNames.has(
                          (g.domain === "containers" ? "container:" : "vm:") + item.name
                        )
                      }
                      t={t}
                      blocked={rowBlocked}
                      onBusyChange={onBusyChange}
                      onSessionGone={() => setSessionGone(true)}
                      hueIndex={nextHue()}
                    />
                  ))}
                </div>
              ))
            )}
          </>
        )}
      </StepCard>
    </div>
  );
}

// ---------------------------------------------------------------------------
// CloudCredsDisclosure — step 3's optional cloud/rclone credential cards,
// behind one expander.
//
// WHY A DISCLOSURE (jdp: "Der Abschnitt von Cloud-Zugangsdaten (S3 / restic
// REST) und Off-site (rclone): brauchen wir die immer oder sind die optional?
// Können wir die in einen Ein-/Aufklapp-Button verstecken wenn sie optional
// sind?"). They are optional, confirmed against the backend rather than
// assumed: CloudCard's fields become nothing but env vars for the restic
// child process (internal/api/service.go's `cloudEnv`, which emits only the
// non-empty ones — AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY/AWS_DEFAULT_REGION
// /RESTIC_REST_USERNAME/RESTIC_REST_PASSWORD), and restic reads none of them
// for a repo that is a plain filesystem path. RcloneCard's config is written
// to DataDir/rclone.conf and only ever consulted for a repo carrying the
// `rclone:` prefix. So a user whose backups live on a local path under the
// host mount, or on a share already mounted on Unraid, needs neither card —
// which is exactly what rclone.hint already tells them in words ("SMB/NFS
// need no rclone: mount the share on Unraid and set a Backup Path to it").
// Two large credential forms permanently open in the middle of the step is a
// lot of screen for something most installs never fill in.
//
// THE TRIGGER is a `Selector` in `select="many"` mode — the mechanism this app
// already uses for disclosure sections (Containers.tsx's per-container
// Ordner/Stoppen/Ausschlussmuster/Hooks/Backups chip row, whose `openSections`
// is a Set for the same "these open independently, this is not a tablist"
// reason). One chip here rather than five, but the same component, the same
// `aria-pressed` state and the same "chip on = its pane below is open"
// reading, instead of a second, bespoke expander idiom.
//   `hue={false}`, deliberately: Selector otherwise gives each item its OWN
// rainbow position by list index, which for a lone chip means position 0 — a
// red chip sitting inside step 3's yellow card. Turning its own hueing off
// does NOT take it out of the colour engine: the chip still paints
// `bg-accent`/`text-accentContrast` when active, and --accent under it comes
// from the StepCard's own `.glim-hue`, so it carries THIS STEP's hue exactly
// like every other button in the step body (Connect & preview, Discover, …)
// and follows rainbow/reactive mode with them. That is the "genuine singleton
// keeps its container's accent" case design-language carves out, not an
// exemption from the engine.
//
// OPEN BY DEFAULT WHEN CREDENTIALS ALREADY EXIST: someone who needs these is
// not made to hunt for them. `configured` probes the same two endpoints the
// cards themselves read (getCloud/getRclone) — the GETs return set-flags, not
// secrets, so this can tell "something is stored" without ever handling one.
// While the probe is in flight `open` stays null and the pane renders closed;
// it can only open itself once, on the probe's answer, and never fights a
// user who has clicked in the meantime (`setOpen((o) => (o === null ? … : o))`).
// ---------------------------------------------------------------------------
function CloudCredsDisclosure({
  t,
  cloudHue,
  rcloneHue,
}: {
  t: ReturnType<typeof useT>["t"];
  /** The two rainbow positions the cards used to take inline. Passed in (and
   *  therefore evaluated by the caller's own `nextHue()` at exactly the point
   *  in the JSX where these cards used to sit) so that COLLAPSING this section
   *  does not renumber the rest of the page: `nextHue()` is a running counter
   *  consumed in JSX evaluation order, so calling it inside a `{open && …}`
   *  branch would shift every heading after step 3 by two positions the moment
   *  the section closed. Props evaluate unconditionally; the cards they colour
   *  do not. */
  cloudHue: number;
  rcloneHue: number;
}) {
  const [open, setOpen] = useState<boolean | null>(null);

  useEffect(() => {
    let alive = true;
    void Promise.all([
      getCloud().catch(() => null),
      getRclone().catch(() => null),
    ]).then(([cloud, rclone]) => {
      if (!alive) return;
      const configured =
        !!cloud?.ok &&
          (!!cloud.s3KeyId || !!cloud.s3Region || !!cloud.restUser || !!cloud.s3SecretSet || !!cloud.restPasswordSet)
        ? true
        : !!rclone?.ok && (rclone.remotes?.length ?? 0) > 0;
      setOpen((o) => (o === null ? configured : o));
    });
    return () => {
      alive = false;
    };
  }, []);

  const isOpen = open === true;
  return (
    // pt-3 on top of the step body's own gap-2, so the chip clears the
    // encryption toggle above it — see this call site's own comment in step 3.
    <div className="pt-3 flex flex-col gap-8">
      <Selector
        items={[{ id: "creds", label: t("recovery.cloudCreds"), tip: t("recovery.cloudCredsHint") }]}
        label={t("recovery.cloudCreds")}
        select="many"
        hue={false}
        active={isOpen ? OPEN_CREDS : NO_CREDS}
        onChange={() => setOpen(!isOpen)}
      />
      {isOpen && (
        <>
          {/* `nested` — these are the Settings page's own Cards rendered inside
              a card, so they drop their (identical-to-the-parent) surface and
              their horizontal padding and line up on the step's own content
              edge. See Card's `nested` doc in Settings.tsx for the measured
              20px indent this removes. */}
          <CloudCard t={t} hueIndex={cloudHue} nested />
          <RcloneCard t={t} hueIndex={rcloneHue} nested />
        </>
      )}
    </div>
  );
}
// Two frozen Sets rather than a `new Set([...])` per render: Selector takes a
// ReadonlySet and there are only ever two possible values here.
const OPEN_CREDS: ReadonlySet<string> = new Set(["creds"]);
const NO_CREDS: ReadonlySet<string> = new Set<string>();

export default function Recovery() {
  const { t } = useT();
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();

  // Step 1 — repo-readable / APP_KEY state, shared with later steps.
  const [readableState, setReadableState] = useState<StepState>("idle");
  const [lastError, setLastError] = useState<string | null>(null);
  const [checking, setChecking] = useState(false);

  // Step 2 — attach settings. Own copy of the settings object; persisted through
  // the SAME putSettings/setCloud/setRclone the Settings page uses (CloudCard and
  // RcloneCard self-persist; paths/off-site/encryption go through the mirrored
  // merge-onto-baseline save below — no new endpoint, no duplicate storage).
  const [settings, setSettings] = useState<Settings | null>(null);
  const [savedSettings, setSavedSettings] = useState<Settings | null>(null);
  const [hostMountRoot, setHostMountRoot] = useState<string>("/host/user");
  const [attachState, setAttachState] = useState<"idle" | "saving">("idle");
  const [previewed, setPreviewed] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a failed
  // action toasts AND shakes its button — a bumped nonce keyed onto the
  // "Connect & preview" button replays `.glim-shake` once per failure, same
  // mechanism as VMExportButton/ExportButton's shakeNonce.
  const [connectPreviewShake, setConnectPreviewShake] = useState(0);

  // Config-restore step (runs BEFORE attach/discover): restore BombVault's OWN
  // settings first so the attach + discover steps come pre-filled. Optional and
  // skippable — a user without a settings backup just attaches manually below.
  // The location (local path / off-site URL) is stored on `settings` and saved
  // right before the restore so the backend resolves the right repo.
  const [configSource, setConfigSource] = useState<RepoSource>("local");
  type ConfigPhase = "idle" | "saving" | "restarting" | "manual" | "reload" | "error";
  const [configPhase, setConfigPhase] = useState<ConfigPhase>("idle");
  const [configError, setConfigError] = useState<string | null>(null);
  const [configSkipped, setConfigSkipped] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a failed
  // action toasts AND shakes its button. This CTA can restart the container, so
  // it gets the exact same treatment as every other primary action in this
  // file — a bumped nonce keyed onto the "Restore my settings" button replays
  // `.glim-shake` once per failure, same mechanism as
  // VMExportButton/ExportButton's shakeNonce.
  const [configShake, setConfigShake] = useState(0);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) {
          setSettings(res.settings);
          setSavedSettings(res.settings);
          if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
        }
      })
      .catch(() => undefined);
  }, []);

  // checkReadable runs the discover probe and classifies the outcome. Shared by
  // Step 1's "Re-check" and Step 2's "Connect & preview". It uses the READ-ONLY
  // probe (probe=true) so merely checking readability never rebuilds the target
  // list — only Step 3's explicit "Discover" does (#44). The count + error
  // classification are identical to a real discover.
  //
  // Returns the classification (not just void) so connectPreview below can
  // react to the FRESH result synchronously — reading the `readableState`
  // React state var right after `await checkReadable()` would risk a stale
  // closure value from before this render's state settled.
  const checkReadable = useCallback(async (): Promise<StepState> => {
    setChecking(true);
    setLastError(null);
    try {
      const [c, v, f] = await Promise.all([discover(true), discoverVMs(true), discoverFiles(true)]);
      const results: DiscoverResult[] = [c, v, f];
      const keyErr = results.find((r) => !r.ok && isKeyMismatch(r.error));
      if (keyErr) {
        setReadableState("bad");
        setLastError(keyErr.error ?? null);
        return "bad";
      }
      const otherErr = results.find((r) => !r.ok);
      if (otherErr) {
        setReadableState("warn");
        setLastError(otherErr.error ?? null);
        return "warn";
      }
      const total = (c.discovered ?? 0) + (v.discovered ?? 0) + (f.discovered ?? 0);
      // >0 = repo readable with content; 0 = reachable but empty / not attached yet.
      const next: StepState = total > 0 ? "ok" : "warn";
      setReadableState(next);
      return next;
    } catch (err) {
      // Network/HTTP failure (unreachable, auth, 5xx) — not a key mismatch.
      setReadableState("warn");
      setLastError(err instanceof Error ? err.message : String(err));
      return "warn";
    } finally {
      setChecking(false);
    }
  }, []);

  // connectPreview saves the paths/off-site/encryption fields (mirroring the
  // Settings save() merge onto the server baseline), then re-runs checkReadable
  // so Step 1's pill reflects the freshly-attached location.
  //
  // GlimStone follow-up pass (v8.0.0): the "saved"/"error" 3000ms inline flash
  // is now a toast, same shape as Settings.tsx's shared save() helper — with
  // one twist: the success flash only ever showed when the FOLLOW-UP
  // readability check also came back "ok" (attaching a bad repo shouldn't look
  // like a completed success), so the toast keeps that same condition, driven
  // by checkReadable's own return value rather than the readableState React
  // var (which would still read stale here, mid-function, before this
  // render's state settles).
  const connectPreview = useCallback(async () => {
    const base = savedSettings ?? settings;
    if (!base || !settings) return;
    setAttachState("saving");
    const patch: Partial<Settings> = {
      containersPath: settings.containersPath,
      vmsPath: settings.vmsPath,
      flashPath: settings.flashPath,
      filesPath: settings.filesPath,
      containersOffsite: settings.containersOffsite,
      vmsOffsite: settings.vmsOffsite,
      flashOffsite: settings.flashOffsite,
      filesOffsite: settings.filesOffsite,
      encryptionEnabled: settings.encryptionEnabled,
    };
    const updated: Settings = { ...base, ...patch };
    try {
      const res = await putSettings(updated);
      if (res.ok) {
        setSavedSettings(updated);
        setSettings((prev) => (prev ? { ...prev, ...patch } : updated));
        // Keep the sidebar/Settings in sync (same event the Settings page fires).
        window.dispatchEvent(new Event("bv:settings-changed"));
        setPreviewed(true);
        // Attaching a (possibly different) repo invalidates any previously
        // discovered targets — clear them so Step 4 can never offer to restore
        // the OLD repo's data; the user must re-Discover against the new repo.
        setContainers([]);
        setVMs([]);
        setFileSets([]);
        setDiscovered(null);
        setRestoreAllResult(null);
        const state = await checkReadable();
        if (state === "ok") push(t("recovery.readable"), "success");
      } else {
        push(res.error ?? t("settings.error"), "fail");
        setConnectPreviewShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      setConnectPreviewShake((n) => n + 1);
    } finally {
      setAttachState("idle");
    }
  }, [savedSettings, settings, checkReadable, push, t]);

  // restoreOwnConfig stages a restore of BombVault's OWN settings and drives the
  // self-restart that applies it. It first persists the chosen config-repo
  // location (merged onto the server baseline, like connectPreview), then calls
  // restoreConfig("latest", source). On autoRestart it polls the health endpoint
  // until BombVault returns and reloads so the restored settings load; without an
  // auto-restart it shows the manual container-restart instruction.
  const restoreOwnConfig = useCallback(async () => {
    const base = savedSettings ?? settings;
    if (!base || !settings) return;
    setConfigPhase("saving");
    setConfigError(null);
    const patch: Partial<Settings> =
      configSource === "offsite"
        ? { configOffsite: settings.configOffsite }
        : { configPath: settings.configPath };
    const updated: Settings = { ...base, ...patch };
    try {
      const saveRes = await putSettings(updated);
      if (!saveRes.ok) {
        const message = saveRes.error ?? t("settings.error");
        setConfigError(message);
        setConfigPhase("error");
        push(message, "fail");
        setConfigShake((n) => n + 1);
        return;
      }
      setSavedSettings(updated);
      setSettings((prev) => (prev ? { ...prev, ...patch } : updated));
      const res = await restoreConfig("latest", configSource === "offsite" ? "offsite" : undefined);
      if (!res.ok) {
        // e.g. an APP_KEY / encryption mismatch — show the mapped remedy.
        const message = isKeyMismatch(res.error) ? t("recovery.appKeyRemedy") : res.error ?? t("settings.error");
        setConfigError(message);
        setConfigPhase("error");
        push(message, "fail");
        setConfigShake((n) => n + 1);
        return;
      }
      if (!res.staged) {
        // Contract drift guard: ok:true but the snapshot was NOT staged — don't drive
        // the restart/reload flow (nothing would be applied). Surface it as an error.
        const message = res.error ?? t("settings.error");
        setConfigError(message);
        setConfigPhase("error");
        push(message, "fail");
        setConfigShake((n) => n + 1);
        return;
      }
      if (res.autoRestart) {
        // BombVault is restarting itself to apply the staged restore. Poll the
        // health endpoint until it answers again, then reload so the restored
        // paths / off-site / creds populate this page (and the steps below).
        setConfigPhase("restarting");
        const back = await waitForAppBack();
        if (back) {
          window.location.reload();
        } else {
          // Poll window elapsed — the restore is already applied on boot, so let
          // the user reload manually once BombVault is back.
          setConfigPhase("reload");
        }
      } else {
        // Docker socket unreachable: the restore is staged + persisted, but the
        // user must restart the container themselves to apply it.
        setConfigPhase("manual");
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t("settings.error");
      setConfigError(message);
      setConfigPhase("error");
      push(message, "fail");
      setConfigShake((n) => n + 1);
    }
  }, [savedSettings, settings, configSource, t, push]);

  const configStepState: StepState =
    configPhase === "error"
      ? "bad"
      : configPhase === "manual" || configPhase === "reload"
        ? "warn"
        : "idle";

  // Step 3 — discover everything. Runs discoverAll(), then re-fetches the target
  // lists (kept for the later review/restore step).
  //
  // GlimStone follow-up pass (v8.0.0) audit note: `discovered`/`discoverError`
  // below are DELIBERATELY left as inline status, not migrated to a toast —
  // unlike Containers.tsx/VMs.tsx's own discoverMsg (which WAS migrated),
  // this counts+error feed `discoverStepState` (this StepCard's own ok/warn
  // pill) AND are read by Step 5 below to decide what's about to be restored.
  // It's reference content the wizard's later steps depend on, not a one-shot
  // ping — the same "what did the last check say" reasoning as
  // IntegrityCard's persisted results.
  const [discovering, setDiscovering] = useState(false);
  const [discovered, setDiscovered] = useState<{ containers: number; vms: number; files: number } | null>(null);
  const [discoverError, setDiscoverError] = useState<string | null>(null);
  // Reconstructed target lists — populated by Discover, read by the review step.
  const [containers, setContainers] = useState<Container[]>([]);
  const [vms, setVMs] = useState<VM[]>([]);
  const [fileSets, setFileSets] = useState<FileSetView[]>([]);

  const runDiscover = useCallback(async () => {
    setDiscovering(true);
    setDiscoverError(null);
    try {
      const counts = await discoverAll();
      // A discover that returned {ok:false} (e.g. a wrong APP_KEY) surfaces its
      // real message here — show it instead of the misleading "found none" state.
      if (counts.error) {
        setDiscoverError(
          isKeyMismatch(counts.error) ? t("recovery.appKeyRemedy") : counts.error
        );
        setDiscovered(null);
        return;
      }
      // Re-fetch the reconstructed target lists and store them for the restore step.
      const [cs, vs, fs] = await Promise.all([listContainers(), listVMs(), listFileSets()]);
      setContainers(cs.containers ?? []);
      setVMs(vs.vms ?? []);
      setFileSets(fs.ok ? fs.fileSets ?? [] : []);
      setDiscovered(counts);
    } catch (err) {
      setDiscoverError(err instanceof Error ? err.message : String(err));
      setDiscovered(null);
    } finally {
      setDiscovering(false);
    }
  }, [t]);

  const discoverStepState: StepState = discovered
    ? discovered.containers + discovered.vms + discovered.files > 0
      ? "ok"
      : "warn"
    : "idle";

  // Step 4 — review & restore all. anyActive() over the shared progress store is
  // the v4 "something is in flight" signal: it gates "Restore all" (and each
  // row) so a bulk run can't collide with a live per-item op, and vice-versa.
  const progressMap = useProgress();
  const running = anyActive(progressMap);
  const [restoreAllBusy, setRestoreAllBusy] = useState(false);
  // GlimStone follow-up pass (v8.0.0) audit note: DELIBERATELY left as inline
  // status, not migrated to a toast — unlike Containers.tsx/VMs.tsx's own
  // bulk-action result (which WAS migrated, see runBulk there), this count
  // ALSO drives `restoreStepState` below (this StepCard's own ok/warn pill),
  // and a disaster-recovery restore's ok/fail counts are exactly what a user
  // needs to keep reading and act on (which rows below need a retry), not a
  // 4s ping to glance at and lose. Same reasoning as IntegrityCard's results.
  const [restoreAllResult, setRestoreAllResult] = useState<{ ok: number; fail: number } | null>(null);

  // Recovery-kit download refusal (e.g. the 403 "set a login password" fail-closed
  // answer when auth is off) — surfaced next to the Step 6 download button.
  const [kitError, setKitError] = useState<string | null>(null);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a failed
  // action toasts AND shakes its button, layered ON TOP of this button's own
  // pre-existing sticky inline error above (kept — same "reference-value error
  // kept inline" pattern as VMExportButton/ExportButton, both of which layer
  // the same fail toast + button shake on top of their own sticky inline msg).
  const [kitShake, setKitShake] = useState(0);

  // Is the libvirt SSH link set up? VM restore needs it. VMSSHInfo() errors
  // (ok:false) precisely when SSH is not wired, so this is the settings check.
  // Advisory only (a note, never a hard block).
  const [vmSshConfigured, setVmSshConfigured] = useState<boolean | null>(null);
  useEffect(() => {
    getVMSSH()
      .then((r) => setVmSshConfigured(r.ok && !!r.host))
      .catch(() => setVmSshConfigured(false));
  }, []);

  // Restore every discovered container THEN every VM, SEQUENTIALLY and LEFT
  // STOPPED — exactly the Containers.tsx restoreSelected pattern: fireAndWaitRun
  // fires one restore and waits for its NEW recorded run to reach a terminal
  // state before the next, so the shared single-flight guard never rejects the
  // follow-ups as "already running". Accumulate an ok/fail count.
  const restoreAll = useCallback(async () => {
    if (restoreAllBusy) return;
    if (containers.length === 0 && vms.length === 0) return;
    if (!(await confirm(t("containers.restoreSelectedConfirm")))) return;
    setRestoreAllBusy(true);
    setRestoreAllResult(null);
    let ok = 0;
    let fail = 0;
    // try/finally so a throw mid-loop can never strand the busy flag (which would
    // leave "Restore all" and every row permanently disabled).
    try {
      for (const c of containers) {
        const res = await fireAndWaitRun({
          kind: "restore",
          matchRun: (r) => r.domain === "container" && r.target === c.name,
          start: () => restore(c.name, "latest", true, undefined, true),
        });
        if (res.ok) ok++;
        else fail++;
      }
      for (const v of vms) {
        // libvirtName, not name: on TrueNAS `name` is the display-only
        // friendly name, and both the recorded run's target and virsh itself
        // only ever know the VM by its raw libvirt name.
        const res = await fireAndWaitRun({
          kind: "restore",
          matchRun: (r) => r.domain === "vm" && r.target === v.libvirtName,
          start: () => restoreVM(v.libvirtName, "latest", true, undefined, true),
        });
        if (res.ok) ok++;
        else fail++;
      }
      setRestoreAllResult({ ok, fail });
    } finally {
      setRestoreAllBusy(false);
    }
  }, [restoreAllBusy, containers, vms, t, confirm]);

  const anyDiscovered = containers.length > 0 || vms.length > 0 || fileSets.length > 0;
  const restoreStepState: StepState = restoreAllResult
    ? restoreAllResult.fail > 0
      ? "warn"
      : "ok"
    : "idle";
  // Rows are blocked while ANY op runs OR while the bulk loop is mid-flight
  // (between two items the SSE store can briefly show nothing active).
  const rowOtherActive = running.active || restoreAllBusy;

  // hueSeq/nextHue — same mechanism as Settings.tsx's own counter (see that
  // file's comment for the full history and jdp's standing rule, "Es soll
  // immer alles in die Farb- und Formengine integriert werden!! IMMER!!").
  // Recovery has no tabs, so this is one flat, page-wide sequence: every
  // StepCard/CloudCard/RcloneCard heading notch below takes
  // `hueIndex={nextHue()}` in exactly the order the JSX evaluates each call,
  // so a branch that isn't currently rendering (e.g. Step 3's own
  // settings-not-loaded-yet fallback) never leaves a gap in the visible
  // rainbow sequence. ForeignRestoreCard gets the counter FUNCTION itself
  // (not one computed value) so its own three headings continue this same
  // sequence rather than restarting at 0.
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    // gap-10, not the gap-5 this page shipped with, and no `p-1`: jdp, live
    // review — "Die Abstände zwischen den Cards sind zu klein. Die oberste
    // Card ist auch zu weit oben. Bitte machen wie sonst überall."
    //   40px (gap-10) is this app's settled page-level rhythm, already the
    // value on Settings.tsx, Config.tsx, Dashboard.tsx, Receiver.tsx and
    // Fleet.tsx — measured on Config.tsx before changing anything here, whose
    // own wrapper comment states it as "the same corrected 40px DOM gap (29px
    // visible, after the same 11px badge overlap every other gap-10 page also
    // has)". Recovery was the last page still on the pre-correction 20px, so
    // this is that one flat bump, governing BOTH complaints at once: the same
    // wrapper gap sets heading→first-card and card→card.
    //   The `p-1` went with it. It dated from this page's original scaffold
    // (eddbb2e), no other page has one, and <main> already provides the
    // page's own p-6 inset — it was 4px of extra ground on one page only,
    // which is exactly the "wie sonst überall" this round is about.
    <div className="flex flex-col gap-10">
      <div>
        {/* The page's <h1> + subtitle pair, kept as-is: every page in this app
            renders a plain `<p>` subtitle under its own heading (Config's
            config.subtitle, Fleet's, Receiver's, Dashboard's), so this one is
            the page's own standing description, not a per-control
            explanation the "Infotexte in i Infobubbles" round is about. Folding
            it into a bubble would make Recovery the one page whose heading
            reads differently from all the others. */}
        <h1 className="text-lg font-semibold text-carbon-text">{t("recovery.pageTitle")}</h1>
        <p className="text-sm text-carbon-textMuted mt-1 max-w-2xl">{t("recovery.intro")}</p>
      </div>

      {/* Step 1 — Can BombVault read your backups? (repo-readable / APP_KEY)
          jdp live-review ("Info-Texte in i Infobubbles"): the permanent
          appKeyExplain <p> is now the heading badge's own (i), same treatment
          Flash.tsx/Config.tsx/Settings.tsx's Cards already give theirs. */}
      <StepCard n={1} title={t("recovery.step1")} hint={t("recovery.appKeyExplain")} state={readableState} hueIndex={nextHue()}>
        <div className="flex items-center gap-3">
          <button
            onClick={() => void checkReadable()}
            disabled={checking}
            className="inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {checking && (
              <span
                className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
                style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
              />
            )}
            {checking ? t("dashboard.checking") : t("recovery.recheck")}
          </button>

          {readableState === "ok" && (
            <span className="text-sm text-statusOk">{t("recovery.readable")}</span>
          )}
          {readableState === "warn" && (
            <span className="text-sm text-statusWarn">{t("recovery.notReachable")}</span>
          )}
        </div>

        {/* Exact remedy when the key doesn't match the repo. */}
        {readableState === "bad" && (
          <div className="rounded-card bg-statusFailBgSoft px-3 py-2.5 text-xs text-statusFail leading-relaxed">
            {t("recovery.appKeyRemedy")}
          </div>
        )}

        {/* The raw (scrubbed) backend message for a warn/other error, as a hint. */}
        {readableState === "warn" && lastError && (
          <p dir="ltr" className="text-xs text-carbon-textMuted font-mono break-all text-start">{lastError}</p>
        )}
      </StepCard>

      {/* Step 2 — Restore BombVault's OWN settings first (optional, pre-attach).
          On a rebuilt box this pre-fills the attach + discover steps below; it
          ends with a self-restart, so it lives here rather than on the Config
          page. Skippable — a user without a settings backup attaches manually. */}
      {/* jdp live-review ("Info-Texte in i Infobubbles"): configHint and
          configAppKeyReminder were two stacked permanent <p>s — one bubble on
          the heading now carries both. They are one explanation split across
          two sentences (what this step does, and the precondition it needs),
          not two separate topics, so a second bubble on the same heading would
          just be two (i) glyphs a reader has to hover in turn.
          `recovery.configSkipped` below is NOT folded in: it is what the card
          says once the step has been skipped — a state readout, and the card's
          only content in that state. */}
      <StepCard
        n={2}
        title={t("recovery.stepConfig")}
        hint={`${t("recovery.configHint")} ${t("recovery.configAppKeyReminder")}`}
        state={configStepState}
        hueIndex={nextHue()}
      >
        {configSkipped ? (
          <p className="text-sm text-carbon-textMuted">{t("recovery.configSkipped")}</p>
        ) : (
          <>
            {settings ? (
              <>
                {/* Where the config backup lives: a local path or an off-site URL. */}
                <div className="flex items-center gap-2 flex-wrap pt-1">
                  <span className="text-xs text-carbon-textMuted">{t("recovery.configSourceLabel")}</span>
                  <SourceToggle
                    source={configSource}
                    onChange={setConfigSource}
                    disabled={configPhase === "saving" || configPhase === "restarting"}
                  />
                </div>

                {configSource === "local" ? (
                  <FolderBrowser
                    label={t("recovery.configLocalPath")}
                    value={settings.configPath}
                    hostMountRoot={hostMountRoot}
                    onChange={(v) => setSettings((prev) => (prev ? { ...prev, configPath: v } : prev))}
                  />
                ) : (
                  <div className="flex flex-col gap-1">
                    <label className="text-xs text-carbon-textSub">{t("recovery.configOffsiteUrl")}</label>
                    <input
                      value={settings.configOffsite}
                      spellCheck={false}
                      onChange={(e) =>
                        setSettings((prev) => (prev ? { ...prev, configOffsite: e.target.value } : prev))
                      }
                      placeholder="rest:http://host:8000/repo"
                      dir="ltr"
                      className={`${offsiteInput} text-start`}
                    />
                  </div>
                )}

                <div className="flex flex-wrap items-center gap-3 pt-1">
                  <button
                    key={configShake}
                    onClick={() => void restoreOwnConfig()}
                    disabled={configPhase === "saving" || configPhase === "restarting"}
                    className={`inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
                      configShake ? " glim-shake" : ""
                    }`}
                  >
                    {(configPhase === "saving" || configPhase === "restarting") && (
                      <span
                        className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
                        style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
                      />
                    )}
                    {configPhase === "saving" ? t("recovery.configRestoring") : t("recovery.configRestore")}
                  </button>
                  <button
                    type="button"
                    onClick={() => setConfigSkipped(true)}
                    disabled={configPhase === "saving" || configPhase === "restarting"}
                    className="text-xs text-carbon-textSub hover:text-carbon-text transition-colors disabled:opacity-50"
                  >
                    {t("recovery.configSkip")}
                  </button>
                </div>

                {/* Restarting — optimistic; waitForAppBack() reloads on return. The
                    manual reload is offered right away too: if BombVault comes back
                    faster than the poll's down-detection window, the user isn't stuck
                    watching the spinner and can reload the moment the app is up. */}
                {configPhase === "restarting" && (
                  <div className="flex flex-col gap-1">
                    {/* Task 7: was text-statusInfo (the old fifth hue) — genuine
                        activity (the app really is restarting right now), a
                        single occurrence on this page, so plain accent-derived
                        text is safe (no competing solid-accent elements at
                        once). text-accentText, not the flat text-accent: a
                        spec-compliance review measured the flat accent gold
                        at 1.61:1 in light theme here (7.79:1 as
                        text-statusInfo #0043ce before this task) — badly under the
                        4.5:1 text minimum. See index.css's --accent-text
                        comment for the fix and the measured numbers. */}
                    <p className="text-sm text-accentText">{t("recovery.configRestarting")}</p>
                    {/* Task 5 (rule 13): same shape as ItemScheduleOverride's
                        converted button — a plain underlined text link. */}
                    <Badge as="button" onClick={() => window.location.reload()} tone="neutral" size="small" className="self-start">
                      {t("recovery.configReload")}
                    </Badge>
                  </div>
                )}
                {/* Manual restart needed (Docker socket unreachable). */}
                {configPhase === "manual" && (
                  <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
                    {t("recovery.configManualRestart")}
                  </div>
                )}
                {/* Auto-restart poll timed out — offer a manual reload. */}
                {configPhase === "reload" && (
                  <div className="flex flex-wrap items-center gap-3">
                    <span className="text-xs text-statusWarn">{t("recovery.configReloadWhenBack")}</span>
                    <button
                      type="button"
                      onClick={() => window.location.reload()}
                      className="rounded-control bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors"
                    >
                      {t("recovery.configReload")}
                    </button>
                  </div>
                )}
                {configPhase === "error" && configError && (
                  <div className="rounded-card bg-statusFailBgSoft px-3 py-2.5 text-xs text-statusFail leading-relaxed wrap-break-word">
                    {configError}
                  </div>
                )}
              </>
            ) : (
              <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
            )}
          </>
        )}
      </StepCard>

      {/* Step 3 — Attach your backups (consolidated; cloud creds un-gated here) */}
      {/* jdp live-review ("Info-Texte in i Infobubbles"): attachHint (a
          permanent <p> at the top of the card) and credsSaveHint (another one
          buried between the credential cards and the Connect button) are both
          "how this step works" prose with no live value in them, so both fold
          into the heading's own (i) — same single-bubble reasoning as Step 2
          above. */}
      <StepCard
        n={3}
        title={t("recovery.step2")}
        hint={`${t("recovery.attachHint")} ${t("recovery.credsSaveHint")}`}
        state={previewed ? readableState : "idle"}
        hueIndex={nextHue()}
      >
        {settings ? (
          <>
            {/* Local backup paths (relative to the host mount). */}
            <FolderBrowser
              label={t("settings.containersPath")}
              value={settings.containersPath}
              hostMountRoot={hostMountRoot}
              onChange={(v) => setSettings((prev) => (prev ? { ...prev, containersPath: v } : prev))}
            />
            <FolderBrowser
              label={t("settings.vmsPath")}
              value={settings.vmsPath}
              hostMountRoot={hostMountRoot}
              onChange={(v) => setSettings((prev) => (prev ? { ...prev, vmsPath: v } : prev))}
            />
            <FolderBrowser
              label={t("settings.flashPath")}
              value={settings.flashPath}
              hostMountRoot={hostMountRoot}
              onChange={(v) => setSettings((prev) => (prev ? { ...prev, flashPath: v } : prev))}
            />
            <FolderBrowser
              label={t("settings.filesPath")}
              value={settings.filesPath}
              hostMountRoot={hostMountRoot}
              onChange={(v) => setSettings((prev) => (prev ? { ...prev, filesPath: v } : prev))}
            />

            {/* Off-site repo URLs (rest / S3 / B2 / sftp / rclone). */}
            <span className="text-xs font-medium text-carbon-textSub pt-1">{t("settings.offsiteTitle")}</span>
            {([
              ["containersOffsite", "nav.containers"],
              ["vmsOffsite", "nav.vms"],
              ["flashOffsite", "nav.flash"],
              ["filesOffsite", "nav.files"],
            ] as const).map(([key, label]) => (
              <div key={key} className="flex flex-col gap-1">
                <label className="text-xs text-carbon-textSub">{t(label)}</label>
                <input
                  value={settings[key]}
                  spellCheck={false}
                  onChange={(e) => setSettings((prev) => (prev ? { ...prev, [key]: e.target.value } : prev))}
                  placeholder="rest:http://host:8000/repo"
                  dir="ltr"
                  className={`${offsiteInput} text-start`}
                />
              </div>
            ))}

            {/* Encryption on/off (reuses the Settings ToggleRow).
                jdp, live review: "Card 3: Passworttoggle soll 'Passwort'
                heissen und 'Passwort aus APP_KEY' soll in eine i Infobubble."
                The row used to render settings.encryptionOn/Off as its LABEL,
                so the caption itself changed text with the switch ("Aktiviert
                (Passwort aus APP_KEY)" / "Deaktiviert (kein Passwort)") —
                the one ToggleRow in the app whose label was a status readout
                rather than a name. It is now a plain static caption like every
                other row's, with the switch carrying on/off on its own.
                  NOTHING IS LOST by that, which is the reason the state string
                moved into the bubble instead of being dropped: those two
                strings say more than "on"/"off" (WHERE the password comes
                from when on, that there is none when off), and that extra
                sentence is exactly bubble content. Read per render off the
                live `settings.encryptionEnabled`, so the (i) still answers "is
                it on right now, and what does that mean" concretely — the same
                shape Settings.tsx's own copy of this row already uses for the
                same value. */}
            <div className="pt-1">
              <ToggleRow
                label={t("settings.encryptionLabel")}
                hint={
                  settings.encryptionEnabled
                    ? t("settings.encryptionOn")
                    : t("settings.encryptionOff")
                }
                checked={settings.encryptionEnabled}
                onChange={(v) => setSettings((prev) => (prev ? { ...prev, encryptionEnabled: v } : prev))}
              />
            </div>

            {/* Cloud + rclone credential cards — the exact Settings components,
                self-persisting via setCloud/setRclone (no duplicate persistence)
                — now behind one expander, because they are optional for most
                installs. See CloudCredsDisclosure above for the whole "why a
                disclosure / why this trigger / why open-when-configured"
                writeup, and for why the two `nextHue()` calls stay HERE, at
                this exact point in the JSX, instead of moving inside the
                collapsed branch (a conditional nextHue() would renumber every
                heading below step 3 whenever the section is closed).
                  SPACING (jdp: "Der darunter folgende Badge ist zu nah am
                Passworttoggle-Text"): measured live before this change, the
                CloudCard heading notch's top edge sat at y=1189 while the
                toggle label's bottom sat at y=1190 — a NEGATIVE 1px gap, the
                badge literally overlapping the text. The DOM gap looked fine
                (8px, the step body's own gap-2) and that is exactly the trap:
                a notch badge is centred ON its card's top edge, so it eats
                half its own height (11px) out of whatever gap precedes it.
                8 - 11 = -3. The disclosure wrapper adds `pt-3` on top of the
                body gap, and its own `gap-8` sits between the chip and the
                first card's edge, so both badge gaps land ~20px clear — see
                that component. */}
            <CloudCredsDisclosure t={t} cloudHue={nextHue()} rcloneHue={nextHue()} />

            {/* Connect & preview — save paths/off-site/encryption, then re-check.
                (The "credentials save via each card's own Save button" note that
                used to sit here is now part of this step's heading bubble — see
                the StepCard's own `hint` above.) */}
            <div className="flex items-center gap-3 pt-1">
              <button
                key={connectPreviewShake}
                onClick={() => void connectPreview()}
                disabled={attachState === "saving"}
                className={`inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
                  connectPreviewShake ? " glim-shake" : ""
                }`}
              >
                {attachState === "saving" && (
                  <span
                    className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
                    style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
                  />
                )}
                {t("recovery.connectPreview")}
              </button>
            </div>
          </>
        ) : (
          <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
        )}
      </StepCard>

      {/* Step 4 — Discover everything (rebuild targets from the backup defs) */}
      <StepCard n={4} title={t("recovery.step3")} state={discoverStepState} hueIndex={nextHue()}>
        <div className="flex items-center gap-3">
          <button
            onClick={() => void runDiscover()}
            disabled={discovering}
            className="inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {discovering && (
              <span
                className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
                style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
              />
            )}
            {discovering ? t("containers.discovering") : t("recovery.discover")}
          </button>

          {discovered && discovered.containers + discovered.vms + discovered.files > 0 && (
            <span className="text-sm text-statusOk">
              {t("recovery.foundCounts")
                .replace("{c}", String(discovered.containers))
                .replace("{v}", String(discovered.vms))}
              {discovered.files > 0 && (
                <> {t("recovery.filesFound").replace("{f}", String(discovered.files))}</>
              )}
            </span>
          )}
        </div>

        {/* 0/0/0 — nothing found: point back to Step 1/2. */}
        {discovered && discovered.containers + discovered.vms + discovered.files === 0 && (
          <p className="text-sm text-statusWarn">{t("recovery.foundNone")}</p>
        )}
        {discoverError && (
          <div className="rounded-card bg-statusFailBgSoft px-3 py-2.5 text-xs text-statusFail leading-relaxed wrap-break-word">
            {discoverError}
          </div>
        )}
      </StepCard>

      {/* Step 5 — Review & restore everything (in place, left stopped) */}
      <StepCard n={5} title={t("recovery.step4")} state={restoreStepState} hueIndex={nextHue()}>
        {!anyDiscovered ? (
          <p className="text-sm text-carbon-textMuted">{t("recovery.noneDiscovered")}</p>
        ) : (
          <>
            {/* Restore all — every container then VM, sequential + left stopped.
                Shown ONLY when there are containers/VMs to bulk-restore: file
                sets carry no original path, so they're restored per-row (below)
                into a chosen folder and restoreAll() deliberately skips them. */}
            {(containers.length > 0 || vms.length > 0) && (
            /* jdp live-review: "Card 5: Der Wiederherstellen-Button der ganzen
               Container soll ganz nach rechts." The button used to LEAD this
               row, with the busy phrase and the ok/fail result trailing it.
               Both readouts now come first and the button is pushed to the
               row's far edge with `ms-auto` — this app's established
               flush-right idiom for a control that shares its row with a
               leading sibling (Containers.tsx's BackupButton/ExportButton
               row; Flash.tsx's own comment spells out the same pair of
               options and why `justify-end` is the one to use only when there
               is nothing to push away from). `ms-auto`, not `ml-auto`: under
               dir="rtl" the row's far edge is its left one, and the button
               has to follow it.
                 It still lands flush right when NEITHER readout is present —
               a lone flex child with `margin-inline-start: auto` absorbs all
               the free space on its start side. Verified live in both states. */
            <div className="flex flex-wrap items-center gap-3">
              {running.active && !restoreAllBusy && (
                <span className="text-xs text-carbon-textMuted">{t(busyPhraseKey(running.phase))}</span>
              )}
              {restoreAllResult && (
                <span
                  className={`text-sm ${restoreAllResult.fail > 0 ? "text-statusWarn" : "text-statusOk"}`}
                >
                  {t("recovery.restoreAllResult")
                    .replace("{ok}", String(restoreAllResult.ok))
                    .replace("{fail}", String(restoreAllResult.fail))}
                </span>
              )}
              <button
                onClick={() => void restoreAll()}
                disabled={restoreAllBusy || running.active}
                className="ms-auto inline-flex items-center gap-2 rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {restoreAllBusy && (
                  <span
                    className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
                    style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
                  />
                )}
                {t("recovery.restoreAll")}
              </button>
            </div>
            )}

            {/* VM restore needs the libvirt SSH link — advisory note, not a block. */}
            {vms.length > 0 && vmSshConfigured === false && (
              <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
                {t("recovery.vmSshNote")}
              </div>
            )}

            {/* Containers first, then VMs. */}
            {containers.length > 0 && (
              <div className="flex flex-col">
                <span className="text-xs font-medium text-carbon-textSub pt-1 pb-1">
                  {t("nav.containers")}
                </span>
                {containers.map((c) => (
                  <RestoreRow
                    key={`container:${c.name}`}
                    domain="container"
                    name={c.name}
                    lastBackup={c.lastBackup}
                    t={t}
                    otherActive={rowOtherActive}
                    hueIndex={nextHue()}
                  />
                ))}
              </div>
            )}
            {vms.length > 0 && (
              <div className="flex flex-col">
                <span className="text-xs font-medium text-carbon-textSub pt-2 pb-1">
                  {t("nav.vms")}
                </span>
                {vms.map((v) => (
                  <RestoreRow
                    key={`vm:${v.libvirtName}`}
                    domain="vm"
                    name={v.libvirtName}
                    displayName={v.name}
                    lastBackup={v.lastBackup}
                    t={t}
                    otherActive={rowOtherActive}
                    hueIndex={nextHue()}
                  />
                ))}
              </div>
            )}
            {/* File sets — restore into a chosen folder ("Restore all" covers
                containers + VMs only; a rediscovered set has no original path,
                so each row needs its own target folder). */}
            {fileSets.length > 0 && (
              <div className="flex flex-col">
                {/* jdp live-review ("Info-Texte in i Infobubbles"): the
                    filesRestoreHint <p> under this group label explained why
                    each set needs its own target folder — permanent prose about
                    a group of controls, so it moves onto the group's own label
                    as the plain (neutral) InfoBubble, not the `onAccent` one:
                    this is a bare eyebrow label on the card surface, not a
                    solid-accent heading badge. */}
                <span className="inline-flex items-center gap-1 self-start text-xs font-medium text-carbon-textSub pt-2 pb-1">
                  {t("nav.files")}
                  <InfoBubble tip={t("recovery.filesRestoreHint")} />
                </span>
                {fileSets.map((s) => (
                  <FileSetRecoveryRow
                    key={`files:${s.id}`}
                    set={s}
                    hostMountRoot={hostMountRoot}
                    t={t}
                    otherActive={rowOtherActive}
                    hueIndex={nextHue()}
                  />
                ))}
              </div>
            )}
          </>
        )}
      </StepCard>

      {/* Step 6 — Your recovery kit (safety net for next time)
          jdp live-review ("Info-Texte in i Infobubbles"): kitHint was the
          card's whole body apart from the download button — now the heading's
          own (i). The `kitError` span below stays: it is the backend's own
          refusal text, shown only when a download is actually refused. */}
      <StepCard n={6} title={t("recovery.step5")} hint={t("recovery.kitHint")} state="idle" hueIndex={nextHue()}>
        <button
          type="button"
          key={kitShake}
          onClick={() => {
            setKitError(null);
            void downloadRecoveryKit().then((err) => {
              setKitError(err);
              if (err) {
                push(err, "fail");
                setKitShake((n) => n + 1);
              }
            });
          }}
          className={`self-start rounded-control bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors${
            kitShake ? " glim-shake" : ""
          }`}
        >
          {t("recovery.kitDownload")}
        </button>
        {kitError && (
          // Backend-provided error text shown verbatim BY DESIGN (e.g. the
          // fail-closed "set a login password" refusal when auth is off) —
          // the API answers English and is not translated client-side.
          <span className="text-xs text-statusFail wrap-break-word">✗ {kitError}</span>
        )}
      </StepCard>

      {/* Restore from ANOTHER BombVault repo (#61) — visually separate from the
          attach steps above; read-only session, nothing persisted. */}
      <ForeignRestoreCard hostMountRoot={hostMountRoot} t={t} otherActive={rowOtherActive} nextHue={nextHue} />
      {confirmDialog}
    </div>
  );
}
