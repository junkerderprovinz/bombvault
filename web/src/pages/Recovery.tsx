import { useCallback, useEffect, useRef, useState } from "react";
import type { CSSProperties, ReactNode } from "react";
import { useT, type TranslationKey } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { SelectField } from "../components/SelectField";
import { hueVars } from "../lib/appearance";
import { RevealInput } from "../components/RevealInput";
import { useReveal } from "../lib/useReveal";
import { withLtrIsolates, FOREIGN_APPDATA_DEST_HINT_LTR_FRAGMENTS } from "../lib/ltrFragments";
import { StepCard, type StepState } from "../components/recovery/StepCard";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { IconRestore } from "../components/Sidebar";
import { InfoBubble } from "../components/InfoBubble";
import { FolderBrowser } from "../components/FolderBrowser";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { CloudCard } from "./settings/CloudCard";
import { RcloneCard } from "./settings/RcloneCard";
import { ToggleRow } from "./settings/shared";
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
  detectEncryption,
  type EncryptionDetection,
  type EncryptionVerdict,
  type RepoEncryption,
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
import { DiscoverFindings } from "../components/placement/DiscoverFindings";
import { placementChanged } from "../lib/placementEvents";
import { Toggle } from "../components/Toggle";

type DiscoverResult = Awaited<ReturnType<typeof discover>>;

function isKeyMismatch(err: string | undefined): boolean {
  return !!err && /APP_KEY/i.test(err);
}

// Shared mono text-input styling (off-site URLs, foreign location/key fields).
const offsiteInput =
  "rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono glim-field-focus";

// RestoreRow restores one discovered container or VM in place through the
// shared RestoreAction and leaves it stopped: recovery restores everything
// first, and the user starts things from the Containers and VMs tabs.
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
  /** Identifier sent to the backend. For VMs this is VM.libvirtName, because
   *  VM.name is display-only on TrueNAS. */
  name: string;
  /** Shown in the row and the confirm text; defaults to name. */
  displayName?: string;
  lastBackup: number | null;
  t: ReturnType<typeof useT>["t"];
  otherActive: boolean;
  /** Rainbow position; the row's glim-hue gives the restore button its accent. */
  hueIndex: number;
}) {
  // Taken from the list's own lastBackup instead of a per-row snapshot fetch,
  // which would start one restic process per item. The restore resolves
  // "latest" on the server.
  const snapLabel = lastBackup ? new Date(lastBackup * 1000).toLocaleString() : "";

  return (
    <div
      className="flex flex-col gap-1 py-2 border-b border-carbon-border last:border-0 glim-hue"
      style={hueVars(hueIndex) as CSSProperties}
    >
      {/* A confirm checkbox does not fit a one-line row, so confirmMessage
          guards the restore with a modal instead. `leading` puts the name and
          time on the badge's own line, so ms-auto pushes the badge to the far
          edge; `label` becomes its tooltip and accessible name. */}
      <RestoreAction
        domain={domain}
        name={name}
        displayName={displayName}
        snapshotId="latest"
        otherActive={{ active: otherActive }}
        successMessage={t("common.done")}
        requireConfirm={false}
        confirmMessage={t("recovery.restoreRowConfirm").replace("{name}", displayName ?? name)}
        showLeaveStopped={false}
        forceLeaveStopped
        showBusyHint={false}
        showStartedHint={false}
        label={t("snapshots.restore")}
        iconBadge
        leading={
          <>
            <span className="text-sm text-carbon-text font-medium flex-1 min-w-0 truncate">
              {displayName ?? name}
            </span>
            <span className="text-carbon-textMuted text-xs shrink-0">
              {snapLabel || t("containers.never")}
            </span>
          </>
        }
        t={t}
      />
    </div>
  );
}

// FileSetRecoveryRow restores a discovered file set into a folder the user
// picks. Sets rebuilt from `fileset:` snapshot tags carry no source path, so an
// in-place restore is impossible. The files endpoint needs a concrete snapshot
// id, and it is resolved on click so that N rows do not start N restic
// processes.
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
  /** Rainbow position, as in RestoreRow. */
  hueIndex: number;
}) {
  const [target, setTarget] = useState("");
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  // Bumped per failure and used as the button key, so the shake replays.
  const [shake, setShake] = useState(0);

  const snapLabel = set.lastBackup ? new Date(set.lastBackup * 1000).toLocaleString() : "";

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
        t,
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
      style={hueVars(hueIndex) as CSSProperties}
    >
      {/* Built inline rather than with RestoreAction because this row drives
          fireAndWaitRun itself. The badge stays disabled until a folder is
          picked in the browser below it. */}
      <div className="flex items-center gap-3 text-sm">
        <span className="text-carbon-text font-medium flex-1 min-w-0 truncate">{set.name}</span>
        <span className="text-carbon-textMuted text-xs shrink-0">
          {snapLabel || t("containers.never")}
        </span>
        <Button
          key={shake}
          label={t("snapshots.restore")}
          labelKey="snapshots.restore"
          glyph={<IconRestore />}
          tone="accent"
          onClick={() => void handleRestore()}
          disabled={busy || otherActive || target.trim() === ""}
          busy={busy}
          className={`ms-auto shrink-0${shake ? " glim-shake" : ""}`}
        />
      </div>
      <FolderBrowser
        label={t("restore.targetPath")}
        value={target}
        hostMountRoot={hostMountRoot}
        onChange={setTarget}
      />
    </div>
  );
}

// Restoring from another BombVault instance's repository, opened read-only with
// that instance's APP_KEY. Unlike the attach steps nothing persists: the
// session lives in server memory for 30 minutes, this card never calls
// putSettings, and foreignClose runs on disconnect and unmount so the foreign
// key does not linger for the full TTL.

/** Reports whether a foreign-restore error means the session lapsed, which a
 *  reconnect fixes. */
function isForeignSessionGone(err: string | undefined): boolean {
  return !!err && /session/i.test(err) && /(expired|unknown)/i.test(err);
}

// ForeignItemRow restores one foreign item from a chosen snapshot; its runs are
// recorded under the same domains as local ones. Files and VMs need a
// destination folder: a foreign file set has no trusted local source path, and
// a foreign VM must not reuse the source server's disk paths, which can point
// at the destination host's RAM rootfs (#122). VM disks land in
// <destination>/<vm-name>/ with the libvirt XML rewritten to match, and the VM
// stays stopped so it can be checked before its first start.
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
  /** A same-named local item is known to exist; false when the local inventory
   *  could not be read, so the confirm says "could not verify". */
  collisionKnown: boolean;
  t: ReturnType<typeof useT>["t"];
  blocked: boolean;
  onBusyChange: (busy: boolean) => void;
  onSessionGone: () => void;
  /** Rainbow position, continuing the page's sequence. */
  hueIndex: number;
}) {
  const [snapshot, setSnapshot] = useState("latest");
  // VMs start at the local VM domains path, the folder the backend falls back
  // to anyway; file sets start blank.
  const [target, setTarget] = useState(domain === "vms" ? "user/domains" : "");
  const needsTarget = domain === "files" || domain === "vms";
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [shake, setShake] = useState(0);

  // Files only: restore the whole set or a picked part of it, such as one stack
  // out of a whole-appdata set. The tree is listed once the user switches to
  // picking.
  const [filesMode, setFilesMode] = useState<"whole" | "subset">("whole");
  const [foreignFiles, setForeignFiles] = useState<FileEntry[]>([]);
  const [filesLoading, setFilesLoading] = useState(false);
  const [filesError, setFilesError] = useState<string | null>(null);
  const [filesFilter, setFilesFilter] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const subsetActive = domain === "files" && filesMode === "subset";

  // Containers only: appdata is remapped onto this host. `overwrite` confirms
  // writing into a non-empty destination that may belong to another container;
  // `warnings` lists the other binds whose source pool this host lacks, which
  // the operator fixes in the template.
  const [overwrite, setOverwrite] = useState(false);
  const [warnings, setWarnings] = useState<ForeignBindWarning[]>([]);

  // onSessionGone is a new arrow on every parent render, and the parent renders
  // on every progress tick. As an effect dependency it would wipe the selection
  // and refetch the tree mid-pick, so the effects read it through a ref.
  const onSessionGoneRef = useRef(onSessionGone);
  onSessionGoneRef.current = onSessionGone;

  const runDomain = domain === "containers" ? "container" : domain === "vms" ? "vm" : "files";
  // Newest first for the picker; restic lists oldest first.
  const snaps = [...item.snapshots].reverse();

  // Best effort: the restore guards the destination either way.
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
        /* The appdata remap and the destination guard still protect the restore. */
      });
    return () => {
      cancelled = true;
    };
  }, [domain, session, item.name]);

  // A snapshot change clears the selection, which belonged to the old tree.
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

  async function handleRestore() {
    if (busy || blocked) return;
    if (needsTarget && target.trim() === "") return;
    if (subsetActive && selected.size === 0) return;
    // An unreadable local inventory gets a "could not verify" confirm rather
    // than claiming the item exists.
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
            // Empty leaves the default to the backend: user/domains for VMs,
            // the restore folder or user/appdata for containers.
            target: target.trim() || undefined,
            paths: subsetActive ? [...selected] : undefined,
            overwrite: domain === "containers" ? overwrite : undefined,
          }),
        t,
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
      style={hueVars(hueIndex) as CSSProperties}
    >
      <div className="flex items-center gap-3 text-sm flex-wrap">
        <span className="text-carbon-text font-medium flex-1 min-w-0 truncate">{item.name}</span>
        <SelectField
          value={snapshot}
          onChange={setSnapshot}
          label={t("recovery.foreignLatest")}
          disabled={busy}
          options={[
            { value: "latest", label: t("recovery.foreignLatest") },
            ...snaps.map((s) => ({
              value: s.id,
              label: `${new Date(s.time).toLocaleString()}, ${s.id.slice(0, 8)}`,
            })),
          ]}
          className="rounded-control bg-carbon-surface2 px-2 py-1.5 text-xs text-carbon-text glim-field-focus"
        />
        <Button
          key={shake}
          label={t("recovery.foreignRestore")}
          labelKey="recovery.foreignRestore"
          glyph={<IconRestore />}
          tone="accent"
          onClick={() => void handleRestore()}
          disabled={busy ||
            blocked ||
            (needsTarget && target.trim() === "") ||
            (subsetActive && selected.size === 0)}
          busy={busy}
          className={`ms-auto shrink-0${shake ? " glim-shake" : ""}`}
        />
      </div>
      {domain === "files" && (
        <div className="flex flex-col gap-2">
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
            {/* The hint sits on the label so it can be read before choosing. */}
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
          {/* The hint holds a literal pool path that needs bidi isolation under
              RTL. A tip is a plain string that doubles as the aria-label, so it
              gets isolate characters from withLtrIsolates instead of the
              <span dir="ltr"> that withLtrFragments emits. */}
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
          <Toggle
            checked={overwrite}
            onChange={setOverwrite}
            disabled={busy}
            label={t("recovery.foreignOverwrite")}
          />
          {warnings.length > 0 && (
            <div className="rounded-card bg-carbon-surface2 px-3 py-2 text-xs text-carbon-textMuted max-w-2xl">
              <p className="text-statusWarn">{t("recovery.foreignBindWarning")}</p>
              <ul className="mt-1 flex flex-col gap-0.5">
                {/* A separator neither string can contain keeps the keys apart.
                    Keep it as the \u0000 escape: a literal NUL byte makes grep
                    and git treat the whole file as binary. */}
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
      {confirmDialog}
    </div>
  );
}

// ForeignRestoreCard is the foreign section: a heading and two steps, connect
// and restore. The session lives in component state, never in Settings.
function ForeignRestoreCard({
  hostMountRoot,
  t,
  otherActive,
  nextHue,
}: {
  hostMountRoot: string;
  t: ReturnType<typeof useT>["t"];
  otherActive: boolean;
  /** The page's hue counter. The card renders three heading notches, and
   *  each takes the next hue in order. */
  nextHue: () => number;
}) {
  const [localPath, setLocalPath] = useState("");
  const [key, setKey] = useState("");
  // The foreign repository's own backend credentials, held for one session and
  // never stored. This instance's cloud credentials are not offered as a
  // default: lending them to a user-supplied URL would be a confused deputy.
  const [foreignS3KeyId, setForeignS3KeyId] = useState("");
  const [foreignS3Secret, setForeignS3Secret] = useState("");
  const [foreignS3Region, setForeignS3Region] = useState("");
  const [foreignRestUser, setForeignRestUser] = useState("");
  const [foreignRestPassword, setForeignRestPassword] = useState("");
  const revealKey = useReveal();
  const revealForeignS3Secret = useReveal();
  const revealForeignRestPassword = useReveal();

  const [phase, setPhase] = useState<"idle" | "connecting" | "connected" | "error">("idle");
  // Shown inline as well as in the toast, because the backend's message is
  // worth reading in full.
  const [connectError, setConnectError] = useState<string | null>(null);
  const [session, setSession] = useState<string | null>(null);
  const [inventory, setInventory] = useState<ForeignInventory | null>(null);
  // Set when a restore reports the session expired, to offer a reconnect with
  // the kept inputs.
  const [sessionGone, setSessionGone] = useState(false);
  // "container:x" and "vm:y", read at connect time so each row knows whether a
  // restore would overwrite something local.
  const [localNames, setLocalNames] = useState<Set<string>>(new Set());
  // False when the local inventory could not be read; every container and VM
  // then asks before overwriting.
  const [localKnown, setLocalKnown] = useState(true);
  const [busyRows, setBusyRows] = useState(0);
  const { push } = useToast();
  const [shake, setShake] = useState(0);

  // The unmount cleanup reads the current session through the ref; an effect
  // on `session` would close each old id on every change.
  const sessionRef = useRef<string | null>(null);
  sessionRef.current = session;
  useEffect(
    () => () => {
      if (sessionRef.current) {
        foreignClose(sessionRef.current).catch(() => undefined);
      }
    },
    []
  );

  const location = localPath.trim();
  // Anything restic would treat as a backend rather than a path: a known scheme,
  // or the unprefixed rclone "name:bucket" that is a common typo. Mirrors
  // restic.IsRemoteRepo and LooksLikeUnprefixedRemote; it only decides which
  // fields to show, the server checks again.
  const isRemoteLocation = /^(rest|s3|sftp|rclone|b2|gs|azure|swift):/i.test(location) ||
    /^[A-Za-z0-9_-]+:[^/\\]/.test(location);
  // Which credential a remote repo needs depends on the backend, so any one
  // unlocks Connect and the server reports what is missing.
  const hasForeignCreds =
    foreignS3KeyId.trim() !== "" ||
    foreignS3Secret.trim() !== "" ||
    foreignRestUser.trim() !== "" ||
    foreignRestPassword.trim() !== "";
  const canConnect =
    location !== "" &&
    key.trim() !== "" &&
    phase !== "connecting" &&
    (!isRemoteLocation || hasForeignCreds);

  const connect = useCallback(async () => {
    if (location === "" || key.trim() === "") return;
    setPhase("connecting");
    setConnectError(null);
    setSessionGone(false);
    if (sessionRef.current) {
      foreignClose(sessionRef.current).catch(() => undefined);
      setSession(null);
      setInventory(null);
    }
    try {
      // A mounted path needs no credentials, so none go on the wire for it.
      const res = await foreignOpen(
        location,
        key.trim(),
        isRemoteLocation
          ? {
              s3KeyId: foreignS3KeyId.trim(),
              s3Secret: foreignS3Secret,
              s3Region: foreignS3Region.trim(),
              restUser: foreignRestUser.trim(),
              restPassword: foreignRestPassword,
              s3StorageClass: "",
            }
          : undefined,
      );
      if (!res.ok || !res.session) {
        const message = res.error ?? t("settings.error");
        setConnectError(message);
        setPhase("error");
        push(message, "fail");
        setShake((n) => n + 1);
        return;
      }
      // The local inventory is read before the rows are enabled, so they never
      // render with a stale collision set.
      const names = new Set<string>();
      let known = true;
      try {
        const [cs, vs] = await Promise.all([listContainers(), listVMs()]);
        // Both answer 200 with ok:false while docker or libvirt is briefly
        // away, and fetchJSON does not throw on that.
        if (!cs.ok || !vs.ok) {
          known = false;
        } else {
          for (const c of cs.containers ?? []) names.add(`container:${c.name}`);
          // Foreign item names come from restic tags and are raw libvirt names,
          // so the match has to be on libvirtName; the display name would miss
          // a TrueNAS VM.
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
  }, [
    location,
    key,
    isRemoteLocation,
    foreignS3KeyId,
    foreignS3Secret,
    foreignS3Region,
    foreignRestUser,
    foreignRestPassword,
    t,
    push,
  ]);

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
    // pt-10 matches the parent's gap-10, so the divider sits centred in the
    // break.
    <div className="flex flex-col gap-10 border-t border-carbon-border pt-10">
      <div>
        {/* No padding wraps this h2, so it anchors the badge itself. */}
        <h2 className="relative flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={nextHue()}>
            {t("recovery.foreignTitle")}
            <InfoBubble tip={t("recovery.foreignIntro")} onAccent />
          </Badge>
        </h2>
      </div>

      <StepCard n={1} title={t("recovery.foreignStepConnect")} state={connectState} hueIndex={nextHue()}>
        <FolderBrowser
          label={t("recovery.foreignLocation")}
          value={localPath}
          hostMountRoot={hostMountRoot}
          onChange={setLocalPath}
          hint={t("recovery.foreignLocationHint")}
        />

        <div className="flex flex-col gap-1">
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

        {isRemoteLocation && (
          <div className="flex flex-col gap-3 rounded-control border border-carbon-border/60 p-3">
            <p className="text-xs text-carbon-textSub">{t("recovery.foreignCredsIntro")}</p>

            <div className="grid gap-3 sm:grid-cols-2">
              <div className="flex flex-col gap-1">
                <label className="flex items-center gap-1 text-xs text-carbon-textSub">
                  {t("recovery.foreignS3KeyId")}
                </label>
                <input
                  value={foreignS3KeyId}
                  spellCheck={false}
                  autoComplete="off"
                  onChange={(e) => setForeignS3KeyId(e.target.value)}
                  className={offsiteInput}
                />
              </div>
              <div className="flex flex-col gap-1">
                <label className="flex items-center gap-1 text-xs text-carbon-textSub">
                  {t("recovery.foreignS3Region")}
                </label>
                <input
                  value={foreignS3Region}
                  spellCheck={false}
                  autoComplete="off"
                  onChange={(e) => setForeignS3Region(e.target.value)}
                  className={offsiteInput}
                />
              </div>
            </div>

            <div className="flex flex-col gap-1">
              <label className="flex items-center gap-1 text-xs text-carbon-textSub">
                {t("recovery.foreignS3Secret")}
              </label>
              <RevealInput
                {...revealForeignS3Secret}
                value={foreignS3Secret}
                spellCheck={false}
                autoComplete="off"
                onChange={(e) => setForeignS3Secret(e.target.value)}
                wrapperClassName="w-full"
                className={offsiteInput}
              />
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <div className="flex flex-col gap-1">
                <label className="flex items-center gap-1 text-xs text-carbon-textSub">
                  {t("recovery.foreignRestUser")}
                  <InfoBubble tip={t("recovery.foreignRestHint")} />
                </label>
                <input
                  value={foreignRestUser}
                  spellCheck={false}
                  autoComplete="off"
                  onChange={(e) => setForeignRestUser(e.target.value)}
                  className={offsiteInput}
                />
              </div>
              <div className="flex flex-col gap-1">
                <label className="flex items-center gap-1 text-xs text-carbon-textSub">
                  {t("recovery.foreignRestPassword")}
                </label>
                <RevealInput
                  {...revealForeignRestPassword}
                  value={foreignRestPassword}
                  spellCheck={false}
                  autoComplete="off"
                  onChange={(e) => setForeignRestPassword(e.target.value)}
                  wrapperClassName="w-full"
                  className={offsiteInput}
                />
              </div>
            </div>
          </div>
        )}

        <div className="flex items-center gap-3 pt-1 flex-wrap">
          {phase === "connected" && (
            <>
              <span className="text-sm text-statusOk">{t("recovery.foreignConnected")}</span>
              <Button
                label={t("recovery.foreignClose")}
                labelKey="recovery.foreignClose"
                tone="neutral"
                onClick={disconnect}
              />
            </>
          )}
          <Button
            key={shake}
            label={t("recovery.foreignConnect")}
            labelKey="recovery.foreignConnect"
            tone="accent"
            onClick={() => void connect()}
            disabled={!canConnect}
            busy={phase === "connecting"}
            title={phase === "connecting" ? t("recovery.foreignConnecting") : undefined}
            className={shake ? "glim-shake" : ""}
          />
        </div>
        {phase === "error" && connectError && (
          <div className="rounded-card bg-statusFailBgSoft px-3 py-2.5 text-xs text-statusFail leading-relaxed wrap-break-word">
            {connectError}
          </div>
        )}
      </StepCard>

      <StepCard n={2} title={t("recovery.foreignStepBrowse")} state={browseState} hueIndex={nextHue()}>
        {!session || !inventory ? (
          <p className="text-sm text-carbon-textMuted">{t("recovery.foreignNotConnected")}</p>
        ) : (
          <>
            {sessionGone && (
              <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed flex items-center gap-3 flex-wrap">
                <span className="flex-1">{t("recovery.foreignExpired")}</span>
                <Button
                  label={t("recovery.foreignReconnect")}
                  labelKey="recovery.foreignReconnect"
                  tone="neutral"
                  onClick={() => void connect()}
                />
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
                        // File sets restore into a chosen folder and never
                        // overwrite. An unreadable local inventory counts as a
                        // possible collision.
                        g.domain !== "files" &&
                        (!localKnown ||
                          localNames.has(
                            (g.domain === "containers" ? "container:" : "vm:") + item.name
                          ))
                      }
                      collisionKnown={
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

// StepDisclosure is the expander for step 3's two optional sections, the
// off-site repo URLs and the cloud and rclone credentials. A repo on a local
// path or on a share mounted on Unraid needs neither: the credentials only
// become env vars for restic, and the rclone config is read only for an
// `rclone:` repo. The trigger is a one-item Selector in "many" mode, the same
// disclosure mechanism as the per-container section chips, and both start
// closed on every load.
function StepDisclosure({
  label,
  tip,
  gap = "gap-8",
  children,
}: {
  label: string;
  tip: string;
  /** Gap between the chip and what it reveals. The default clears a Card's
   *  heading notch, which sits centred on the card's top edge; plain fields
   *  pass the step body's gap-2. */
  gap?: string;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  return (
    // pt-3 on top of the step body's gap-2, so the chip clears what sits above.
    <div className={`pt-3 flex flex-col ${gap}`}>
      <Selector
        items={[{ id: DISCLOSURE_ID, label, tip }]}
        label={label}
        select="many"
        size="lg"
        /* A rainbow hue encodes a position in a list, and a lone chip has
           none, so it takes the StepCard's accent instead. */
        hue={false}
        active={open ? OPEN_SECTION : NO_SECTION}
        onChange={() => setOpen((o) => !o)}
      />
      {open && children}
    </div>
  );
}

// Each chip is alone in its Selector, so one id and two shared sets serve
// every StepDisclosure.
const DISCLOSURE_ID = "sec";
const OPEN_SECTION: ReadonlySet<string> = new Set([DISCLOSURE_ID]);
const NO_SECTION: ReadonlySet<string> = new Set<string>();

// CloudCredsDisclosure is step 3's credential cards inside a StepDisclosure,
// taking its hues as props so the caller's nextHue() calls stay unconditional.
function CloudCredsDisclosure({
  t,
  cloudHue,
  rcloneHue,
}: {
  t: ReturnType<typeof useT>["t"];
  /** nextHue() is a running counter; called inside the collapsible branch it
   *  would renumber every later heading whenever the section closes. */
  cloudHue: number;
  rcloneHue: number;
}) {
  return (
    <StepDisclosure label={t("recovery.cloudCreds")} tip={t("recovery.cloudCredsHint")}>
      {/* `nested` drops the card surface and horizontal padding, so the
          cards line up with the step's content edge. */}
      <CloudCard t={t} hueIndex={cloudHue} nested />
      <RcloneCard t={t} hueIndex={rcloneHue} nested />
    </StepDisclosure>
  );
}

// EncryptionStatus shows step 3's encryption state. Encryption is a fact about
// a repository, fixed when it is created (see ModeFor and
// encryption_detect.go), so the backend probes the configured repos and the
// setting follows what it finds. A detected state gets a plain status line,
// since even a disabled switch reads as something the user should have set.
// The switch appears only when there is nothing to detect yet, which leaves
// the choice to the user, or as an override next to the per-repo detail when
// the probe could not tell or the repos disagree.

const ENC_VERDICT_TONE: Record<EncryptionVerdict, string> = {
  encrypted: "text-statusOk",
  plain: "text-statusOk",
  absent: "text-carbon-textMuted",
  unconfigured: "text-carbon-textMuted",
  unknown: "text-statusWarn",
  conflict: "text-statusFail",
};

const ENC_VERDICT_MESSAGE: Record<EncryptionVerdict, TranslationKey> = {
  encrypted: "recovery.encEncrypted",
  plain: "recovery.encPlain",
  absent: "recovery.encAbsent",
  unconfigured: "recovery.encUnconfigured",
  unknown: "recovery.encUnknown",
  conflict: "recovery.encConflict",
};

/** Verdicts that leave the mode open, so the user decides or overrides. */
const ENC_NEEDS_CONTROL: ReadonlySet<EncryptionVerdict> = new Set<EncryptionVerdict>([
  "absent",
  "unconfigured",
  "unknown",
  "conflict",
]);

/** Verdicts that only help with the repos named: which ones disagree, or
 *  which one failed and why. */
const ENC_SHOWS_REPOS: ReadonlySet<EncryptionVerdict> = new Set<EncryptionVerdict>([
  "unknown",
  "conflict",
]);

const ENC_STATE_KEY: Record<RepoEncryption["state"], TranslationKey> = {
  encrypted: "recovery.encStateEncrypted",
  plain: "recovery.encStatePlain",
  absent: "recovery.encStateAbsent",
  unreachable: "recovery.encStateUnreachable",
};

const ENC_DOMAIN_KEY: Record<string, TranslationKey> = {
  containers: "nav.containers",
  vms: "nav.vms",
  flash: "nav.flash",
  files: "nav.files",
  config: "nav.config",
};

function EncryptionStatus({
  t,
  detection,
  detecting,
  encryptionEnabled,
  onOverride,
}: {
  t: (k: TranslationKey) => string;
  detection: EncryptionDetection | null;
  detecting: boolean;
  encryptionEnabled: boolean;
  onOverride: (v: boolean) => void;
}) {
  // First run: say what is happening rather than flashing a control the probe
  // is about to make unnecessary.
  if (detecting && !detection) {
    return (
      <div className="flex items-center gap-2 pb-1">
        <span
          className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
          style={{ borderColor: "var(--accent)", borderTopColor: "transparent" }}
        />
        <span className="text-sm text-carbon-textMuted">{t("recovery.encChecking")}</span>
      </div>
    );
  }

  // A failed probe (network or HTTP, not a verdict) counts as "unknown".
  const verdict: EncryptionVerdict = detection?.ok ? detection.verdict ?? "unknown" : "unknown";
  const repos = detection?.repos ?? [];

  return (
    <div className="flex flex-col gap-2 pb-1">
      <div className="flex items-start gap-1.5">
        <p className={`text-sm leading-relaxed ${ENC_VERDICT_TONE[verdict]}`}>
          {t(ENC_VERDICT_MESSAGE[verdict])}
        </p>
        <InfoBubble tip={t("recovery.encDetectHint")} />
      </div>

      {ENC_SHOWS_REPOS.has(verdict) && repos.length > 0 && (
        <ul className="flex flex-col gap-1">
          {repos.map((r, i) => (
            <li
              key={`${r.domain}-${r.source}-${r.name ?? ""}-${i}`}
              className="text-xs text-carbon-textMuted leading-relaxed"
            >
              <span className="text-carbon-textSub">
                {t(ENC_DOMAIN_KEY[r.domain] ?? "nav.containers")}
                {" · "}
                {t(r.source === "offsite" ? "recovery.encSourceOffsite" : "recovery.encSourceLocal")}
                {r.name ? ` (${r.name})` : ""}
              </span>
              {": "}
              {t(ENC_STATE_KEY[r.state])}
              {r.error && (
                <span dir="ltr" className="font-mono break-all">: {r.error}</span>
              )}
            </li>
          ))}
        </ul>
      )}

      {ENC_NEEDS_CONTROL.has(verdict) && (
        <ToggleRow
          label={t("settings.encryptionLabel")}
          hint={encryptionEnabled ? t("settings.encryptionOn") : t("settings.encryptionOff")}
          checked={encryptionEnabled}
          onChange={onOverride}
        />
      )}
    </div>
  );
}

export default function Recovery() {
  const { t } = useT();
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();

  // Step 1's readability state, shared with the later steps.
  const [readableState, setReadableState] = useState<StepState>("idle");
  // The folders the check read, shown so an empty answer says where it looked.
  const [readSources, setReadSources] = useState<string[]>([]);
  const [lastError, setLastError] = useState<string | null>(null);
  // Repositories the probe skipped because they are switched off. Kept apart
  // from lastError, which only renders in the warn state: this is said at every
  // pill colour without turning the pill amber.
  const [readNote, setReadNote] = useState<string | null>(null);
  const [checking, setChecking] = useState(false);

  // Step 2 keeps its own copy of the settings and saves through the same calls
  // as the Settings page; CloudCard and RcloneCard save themselves.
  const [settings, setSettings] = useState<Settings | null>(null);
  const [hostMountRoot, setHostMountRoot] = useState<string>("/host/user");
  const [attachState, setAttachState] = useState<"idle" | "saving">("idle");
  const [previewed, setPreviewed] = useState(false);
  const [connectPreviewShake, setConnectPreviewShake] = useState(0);

  // The probe runs on mount, so detecting starts true.
  const [encDetection, setEncDetection] = useState<EncryptionDetection | null>(null);
  const [encDetecting, setEncDetecting] = useState(true);

  // runEncryptionDetect probes the configured repos and writes the result into
  // the local settings copy; otherwise the next connectPreview would PUT the
  // stale value back over the detected one. It returns the detection so a
  // caller need not read encDetection through a stale closure.
  const runEncryptionDetect = useCallback(async (): Promise<EncryptionDetection | null> => {
    setEncDetecting(true);
    try {
      const res = await detectEncryption();
      setEncDetection(res);
      if (res.ok && typeof res.encryptionEnabled === "boolean") {
        const detected = res.encryptionEnabled;
        setSettings((prev) => (prev ? { ...prev, encryptionEnabled: detected } : prev));
      }
      return res;
    } catch (err) {
      // A transport failure says nothing about encryption and renders as
      // "unknown".
      const failed: EncryptionDetection = {
        ok: false,
        error: err instanceof Error ? err.message : String(err),
      };
      setEncDetection(failed);
      return failed;
    } finally {
      setEncDetecting(false);
    }
  }, []);

  // The config step restores BombVault's own settings first, so attach and
  // discover come pre-filled. It is optional; without a settings backup the
  // user attaches by hand. The location is saved right before the restore so
  // the backend resolves the right repo.
  const [configSource, setConfigSource] = useState<RepoSource>("local");
  type ConfigPhase = "idle" | "saving" | "restarting" | "manual" | "reload" | "error";
  const [configPhase, setConfigPhase] = useState<ConfigPhase>("idle");
  const [configError, setConfigError] = useState<string | null>(null);
  const [configSkipped, setConfigSkipped] = useState(false);
  const [configShake, setConfigShake] = useState(0);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) {
          setSettings(res.settings);
          if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
        }
      })
      .catch(() => undefined)
      // After the settings rather than in parallel, so a late getSettings
      // response cannot overwrite the detection's write-back.
      .finally(() => void runEncryptionDetect());
  }, [runEncryptionDetect]);

  // checkReadable runs the read-only discover probe, so a check never rebuilds
  // the target list; only step 3's Discover does. The probe opens the encrypted
  // repo, so a wrong APP_KEY shows up as the mapped key error. It returns the
  // state so connectPreview need not read readableState through a stale
  // closure.
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
      // The wizard reads each domain's primary path, which after a disaster is
      // often an empty local folder. With the path named, an empty answer reads
      // as "it looked in the wrong place" rather than "my backups are gone".
      setReadSources([c.repo, v.repo, f.repo].filter((r): r is string => !!r));
      // A repository that could not be opened keeps the pill off green even when
      // the others had content. One switched off on purpose is only named: it is
      // no fault, and holding the pill amber would also swallow the success
      // toast gated on "ok". skippedNeedsAction is the server's split between
      // the two, see repoSkip.Note.
      const skipped = [...new Set(results.flatMap((r) => r.skipped ?? []))];
      const needsAction = results.some((r) => r.skippedNeedsAction === true);
      const skippedLine = skipped.length > 0 ? t("common.discoverSkipped").replace("{list}", skipped.join(", ")) : null;
      setReadNote(needsAction ? null : skippedLine);
      if (skippedLine && needsAction) {
        setLastError(skippedLine);
        setReadableState("warn");
        return "warn";
      }
      // Zero means reachable but empty, or not attached yet.
      const next: StepState = total > 0 ? "ok" : "warn";
      setReadableState(next);
      return next;
    } catch (err) {
      // Network or HTTP failure, not a key mismatch.
      setReadableState("warn");
      setLastError(err instanceof Error ? err.message : String(err));
      return "warn";
    } finally {
      setChecking(false);
    }
    // The skip sentence is translated here, and t changes identity on a
    // language switch without a remount.
  }, [t]);

  // connectPreview saves the attach fields and re-runs checkReadable for step
  // 1's pill. The success toast waits for that check to come back ok, since
  // attaching an unreadable repo is no success.
  const connectPreview = useCallback(async () => {
    if (!settings) return;
    setAttachState("saving");
    // The PUT sends a full object, so it merges onto freshly fetched settings;
    // a snapshot from mount would roll back whatever another tab or an import
    // changed meanwhile. The fetch answers ok:false instead of throwing, and a
    // failure aborts rather than falling back to stale data.
    const latest = await getSettings();
    if (!latest.ok) {
      setAttachState("idle");
      push(latest.error ?? t("config.loadSettingsFailed"), "fail");
      return;
    }
    const base = latest.settings;
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
        setSettings((prev) => (prev ? { ...prev, ...patch } : updated));
        window.dispatchEvent(new Event("bv:settings-changed"));
        setPreviewed(true);
        // A new repo invalidates the discovered targets, so the restore step
        // cannot offer the old repo's data.
        setContainers([]);
        setVMs([]);
        setFileSets([]);
        setDiscovered(null);
        setRestoreAllResult(null);
        // Detection probes the configured locations, so it runs once the new
        // paths are saved. The patch still carries encryptionEnabled because
        // for a new, empty location it is the user's choice; where the mode is
        // detectable, this run overwrites it and writes the result back.
        await runEncryptionDetect();
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
  }, [settings, checkReadable, runEncryptionDetect, push, t]);

  // restoreOwnConfig saves the chosen config-repo location, stages a restore of
  // BombVault's own settings and follows the restart that applies it: with
  // autoRestart it waits for the app and reloads, otherwise it shows the manual
  // restart instruction.
  const restoreOwnConfig = useCallback(async () => {
    if (!settings) return;
    setConfigPhase("saving");
    setConfigError(null);
    // The same fresh baseline as in connectPreview.
    const latest = await getSettings();
    if (!latest.ok) {
      const message = latest.error ?? t("config.loadSettingsFailed");
      setConfigError(message);
      setConfigPhase("error");
      push(message, "fail");
      setConfigShake((n) => n + 1);
      return;
    }
    const base = latest.settings;
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
      setSettings((prev) => (prev ? { ...prev, ...patch } : updated));
      const res = await restoreConfig("latest", configSource === "offsite" ? "offsite" : undefined);
      if (!res.ok) {
        const message = isKeyMismatch(res.error) ? t("recovery.appKeyRemedy") : res.error ?? t("settings.error");
        setConfigError(message);
        setConfigPhase("error");
        push(message, "fail");
        setConfigShake((n) => n + 1);
        return;
      }
      if (!res.staged) {
        // Without a staged snapshot the restart would apply nothing.
        const message = res.error ?? t("settings.error");
        setConfigError(message);
        setConfigPhase("error");
        push(message, "fail");
        setConfigShake((n) => n + 1);
        return;
      }
      if (res.autoRestart) {
        // Reload once BombVault answers again, so the restored settings fill
        // this page.
        setConfigPhase("restarting");
        const back = await waitForAppBack();
        if (back) {
          window.location.reload();
        } else {
          // The restore applies on boot anyway; the user reloads once
          // BombVault is back.
          setConfigPhase("reload");
        }
      } else {
        // Docker socket unreachable: the restore is staged, and the user
        // restarts the container to apply it.
        setConfigPhase("manual");
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t("settings.error");
      setConfigError(message);
      setConfigPhase("error");
      push(message, "fail");
      setConfigShake((n) => n + 1);
    }
  }, [settings, configSource, t, push]);

  const configStepState: StepState =
    configPhase === "error"
      ? "bad"
      : configPhase === "manual" || configPhase === "reload"
        ? "warn"
        : "idle";

  // Step 3 runs discoverAll() and refetches the target lists for the restore
  // step. The result stays inline instead of becoming a toast, because the
  // step's pill and step 5 both read it.
  const [discovering, setDiscovering] = useState(false);
  const [discovered, setDiscovered] = useState<Awaited<ReturnType<typeof discoverAll>> | null>(null);
  const [discoverError, setDiscoverError] = useState<string | null>(null);
  // Filled by Discover, read by the restore step.
  const [containers, setContainers] = useState<Container[]>([]);
  const [vms, setVMs] = useState<VM[]>([]);
  const [fileSets, setFileSets] = useState<FileSetView[]>([]);

  const runDiscover = useCallback(async () => {
    setDiscovering(true);
    setDiscoverError(null);
    try {
      const counts = await discoverAll();
      // A failed discover shows its real message instead of "found none". The
      // counts and skip list are kept either way: named repositories are
      // searched before the domain's own, so when that one fails, whatever was
      // found is real and already written back.
      if (counts.error) {
        setDiscoverError(
          isKeyMismatch(counts.error) ? t("recovery.appKeyRemedy") : counts.error
        );
      }
      const [cs, vs, fs] = await Promise.all([listContainers(), listVMs(), listFileSets()]);
      setContainers(cs.containers ?? []);
      setVMs(vs.vms ?? []);
      setFileSets(fs.ok ? fs.fileSets ?? [] : []);
      setDiscovered(counts);
      if (counts.paused.length > 0 || counts.leftOpen.length > 0) placementChanged();
    } catch (err) {
      setDiscoverError(err instanceof Error ? err.message : String(err));
      setDiscovered(null);
    } finally {
      setDiscovering(false);
    }
  }, [t]);

  // The pill answers for the whole pass: a partial discover keeps what it found
  // and carries the error too, so going by the count alone would show ok above
  // a red error, the rule checkReadable follows as well.
  const discoverStepState: StepState = discovered
    ? discoverError || discovered.skippedNeedsAction
      ? "warn"
      : discovered.containers + discovered.vms + discovered.files > 0
        ? "ok"
        : "warn"
    : "idle";

  // The restore step. anyActive() gates "Restore all" and each row, so a bulk
  // run cannot collide with a single restore.
  const progressMap = useProgress();
  const running = anyActive(progressMap);
  const [restoreAllBusy, setRestoreAllBusy] = useState(false);
  // Inline rather than a toast: it drives the step's pill, and the counts say
  // whether rows need a retry.
  const [restoreAllResult, setRestoreAllResult] = useState<{ ok: number; fail: number } | null>(null);

  // A refused kit download, such as the 403 while no login password is set.
  const [kitError, setKitError] = useState<string | null>(null);
  const [kitShake, setKitShake] = useState(0);

  // VM restore needs the libvirt SSH link, and getVMSSH fails exactly when it
  // is not set up. Only a note, never a block.
  const [vmSshConfigured, setVmSshConfigured] = useState<boolean | null>(null);
  useEffect(() => {
    getVMSSH()
      .then((r) => setVmSshConfigured(r.ok && !!r.host))
      .catch(() => setVmSshConfigured(false));
  }, []);

  // Restores every container, then every VM, one at a time and left stopped.
  // fireAndWaitRun waits for each run to finish, so the single-flight guard
  // never rejects the next one as already running.
  const restoreAll = useCallback(async () => {
    if (restoreAllBusy) return;
    if (containers.length === 0 && vms.length === 0) return;
    if (!(await confirm(t("containers.restoreSelectedConfirm")))) return;
    setRestoreAllBusy(true);
    setRestoreAllResult(null);
    let ok = 0;
    let fail = 0;
    try {
      for (const c of containers) {
        const res = await fireAndWaitRun({
          kind: "restore",
          matchRun: (r) => r.domain === "container" && r.target === c.name,
          start: () => restore(c.name, "latest", true, undefined, true),
          t,
        });
        if (res.ok) ok++;
        else fail++;
      }
      for (const v of vms) {
        // On TrueNAS `name` is display-only; virsh and the recorded run know
        // the VM by its libvirt name.
        const res = await fireAndWaitRun({
          kind: "restore",
          matchRun: (r) => r.domain === "vm" && r.target === v.libvirtName,
          start: () => restoreVM(v.libvirtName, "latest", true, undefined, true),
          t,
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
  // The bulk loop counts too: between two items the progress store can
  // briefly show nothing active.
  const rowOtherActive = running.active || restoreAllBusy;

  // One page-wide hue sequence, handed out in JSX evaluation order, so a
  // branch that is not rendered leaves no gap. ForeignRestoreCard gets the
  // function itself so its headings continue the sequence.
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    <div className={PAGE_SHELL}>
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">{t("nav.recovery")}</h1>
        <p className="mt-1 text-sm text-carbon-textSub max-w-2xl">{t("recovery.intro")}</p>
      </div>

      <StepCard n={1} title={t("recovery.step1")} hint={t("recovery.appKeyExplain")} state={readableState} hueIndex={nextHue()}>
        <div className="flex items-center gap-3">
          <Button
            label={t("recovery.recheck")}
            labelKey="recovery.recheck"
            tone="accent"
            onClick={() => void checkReadable()}
            disabled={checking}
            busy={checking}
            title={checking ? t("dashboard.checking") : undefined}
          />

          {readableState === "ok" && (
            <span className="text-sm text-statusOk">{t("recovery.readable")}</span>
          )}
          {readableState === "warn" && (
            <span className="text-sm text-statusWarn">{t("recovery.notReachable")}</span>
          )}
        </div>

        {/* Shown after every check, not only on failure: a good result confirms
            the place. Raw paths, because a stuck user compares them with what
            they typed. */}
        {readSources.length > 0 && readableState !== "idle" && (
          <p className="text-xs text-carbon-textMuted leading-relaxed wrap-break-word">
            {t("recovery.readFrom")}{" "}
            <span className="font-mono">{readSources.join(", ")}</span>
          </p>
        )}

        {readableState === "bad" && (
          <div className="rounded-card bg-statusFailBgSoft px-3 py-2.5 text-xs text-statusFail leading-relaxed">
            {t("recovery.appKeyRemedy")}
          </div>
        )}

        {readableState === "warn" && lastError && (
          <p dir="ltr" className="text-xs text-carbon-textMuted font-mono break-all text-start">{lastError}</p>
        )}

        {readNote && (
          <p dir="ltr" className="text-xs text-carbon-textMuted break-words text-start">{readNote}</p>
        )}
      </StepCard>

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
                  <Button
                    label={t("recovery.configSkip")}
                    labelKey="recovery.configSkip"
                    tone="neutral"
                    onClick={() => setConfigSkipped(true)}
                    disabled={configPhase === "saving" || configPhase === "restarting"}
                  />
                  <Button
                    key={configShake}
                    label={t("recovery.configRestore")}
                    labelKey="recovery.configRestore"
                    tone="accent"
                    onClick={() => void restoreOwnConfig()}
                    disabled={configPhase === "saving" || configPhase === "restarting"}
                    busy={(configPhase === "saving" || configPhase === "restarting")}
                    title={(configPhase === "saving" || configPhase === "restarting") ? t("recovery.configRestoring") : undefined}
                    className={configShake ? "glim-shake" : ""}
                  />
                </div>

                {/* The reload shows right away, in case BombVault is back before
                    the poll notices it went down. */}
                {configPhase === "restarting" && (
                  <div className="flex flex-col gap-1">
                    {/* text-accentText: the flat accent misses the 4.5:1 text
                        contrast in the light theme, see index.css. */}
                    <p className="text-sm text-accentText">{t("recovery.configRestarting")}</p>
                    <Badge as="button" onClick={() => window.location.reload()} tone="neutral" size="small" className="self-start">
                      {t("recovery.configReload")}
                    </Badge>
                  </div>
                )}
                {configPhase === "manual" && (
                  <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
                    {t("recovery.configManualRestart")}
                  </div>
                )}
                {configPhase === "reload" && (
                  <div className="flex flex-wrap items-center gap-3">
                    <span className="text-xs text-statusWarn">{t("recovery.configReloadWhenBack")}</span>
                    <Button
                      label={t("recovery.configReload")}
                      labelKey="recovery.configReload"
                      tone="neutral"
                      onClick={() => window.location.reload()}
                    />
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

      <StepCard
        n={3}
        title={t("recovery.step2")}
        hint={`${t("recovery.attachHint")} ${t("recovery.credsSaveHint")}`}
        state={previewed ? readableState : "idle"}
        hueIndex={nextHue()}
      >
        {settings ? (
          <>
            {/* First in the card because it is the step's outcome: it turns
                into the detected verdict once Connect & preview saves the
                paths below. */}
            <EncryptionStatus
              t={t}
              detection={encDetection}
              detecting={encDetecting}
              encryptionEnabled={settings.encryptionEnabled}
              onOverride={(v) =>
                setSettings((prev) => (prev ? { ...prev, encryptionEnabled: v } : prev))
              }
            />

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

            {/* gap-2 because plain fields have no notch to clear. */}
            <StepDisclosure
              label={t("settings.offsiteTitle")}
              tip={t("settings.offsiteHint")}
              gap="gap-2"
            >
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
            </StepDisclosure>

            {/* The Settings page's own cards, which save themselves. */}
            <CloudCredsDisclosure t={t} cloudHue={nextHue()} rcloneHue={nextHue()} />

            <div className="flex items-center gap-3 pt-1">
              <Button
                key={connectPreviewShake}
                label={t("recovery.connectPreview")}
                labelKey="recovery.connectPreview"
                tone="accent"
                onClick={() => void connectPreview()}
                disabled={attachState === "saving"}
                busy={attachState === "saving"}
                className={connectPreviewShake ? "glim-shake" : ""}
              />
            </div>
          </>
        ) : (
          <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
        )}
      </StepCard>

      <StepCard n={4} title={t("recovery.step3")} state={discoverStepState} hueIndex={nextHue()}>
        <div className="flex items-center gap-3">
          <Button
            label={t("recovery.discover")}
            labelKey="recovery.discover"
            tone="accent"
            onClick={() => void runDiscover()}
            disabled={discovering}
            busy={discovering}
            title={discovering ? t("containers.discovering") : undefined}
          />

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

        {discovered && discovered.containers + discovered.vms + discovered.files === 0 && (
          <p className="text-sm text-statusWarn">{t("recovery.foundNone")}</p>
        )}
        {/* Without the skipped repositories named, an unmounted share would
            read like an empty archive. */}
        {discovered && discovered.skipped.length > 0 && (
          <p className="text-sm text-statusWarn">
            {t("common.discoverSkipped").replace("{list}", discovered.skipped.join(", "))}
          </p>
        )}
        {discovered && (
          <DiscoverFindings paused={discovered.paused} leftOpen={discovered.leftOpen} directRepos={discovered.directRepos} />
        )}
        {discoverError && (
          <div className="rounded-card bg-statusFailBgSoft px-3 py-2.5 text-xs text-statusFail leading-relaxed wrap-break-word">
            {discoverError}
          </div>
        )}
      </StepCard>

      <StepCard n={5} title={t("recovery.step4")} state={restoreStepState} hueIndex={nextHue()}>
        {!anyDiscovered ? (
          <p className="text-sm text-carbon-textMuted">{t("recovery.noneDiscovered")}</p>
        ) : (
          <>
            {/* File sets carry no original path, so restoreAll() skips them and
                they restore per row into a chosen folder. */}
            {(containers.length > 0 || vms.length > 0) && (
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
                <Button
                  label={t("recovery.restoreAll")}
                  labelKey="recovery.restoreAll"
                  tone="accent"
                  onClick={() => void restoreAll()}
                  disabled={restoreAllBusy || running.active}
                  busy={restoreAllBusy}
                  className="ms-auto"
                />
              </div>
            )}

            {vms.length > 0 && vmSshConfigured === false && (
              <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
                {t("recovery.vmSshNote")}
              </div>
            )}

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
            {fileSets.length > 0 && (
              <div className="flex flex-col">
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

      <StepCard n={6} title={t("recovery.step5")} hint={t("recovery.kitHint")} state="idle" hueIndex={nextHue()}>
        <Button
          key={kitShake}
          label={t("recovery.kitDownload")}
          labelKey="recovery.kitDownload"
          tone="neutral"
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
          // Only layout here: tone and .glim-btn carry the rest, and a second
          // background utility would win by stylesheet order over the tone's.
          className={`self-start${kitShake ? " glim-shake" : ""}`}
        />
        {kitError && (
          // Shown verbatim; the backend answers in English.
          <span className="text-xs text-statusFail wrap-break-word">✗ {kitError}</span>
        )}
      </StepCard>

      <ForeignRestoreCard hostMountRoot={hostMountRoot} t={t} otherActive={rowOtherActive} nextHue={nextHue} />
      {confirmDialog}
    </div>
  );
}
