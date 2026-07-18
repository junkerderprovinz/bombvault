import { useCallback, useEffect, useRef, useState } from "react";
import { useT } from "../lib/i18n";
import { StepCard, type StepState } from "../components/recovery/StepCard";
import { FolderBrowser } from "../components/FolderBrowser";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { CloudCard, RcloneCard, ToggleRow } from "./Settings";
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
  recoveryKitUrl,
  foreignOpen,
  foreignClose,
  foreignRestore,
  type Settings,
  type Container,
  type VM,
  type FileSetView,
  type ForeignInventory,
  type ForeignItem,
} from "../lib/api";

// classifyReadable's probe: discover() + discoverVMs() OPEN the encrypted repo
// (they read the mirrored, restic-encrypted definitions), so they are the
// cleanest "can BombVault read your backups?" check with no backend change:
//   - a wrong APP_KEY  -> the mapped "APP_KEY differs" error in {ok:false,error}
//   - a missing/empty repo -> {ok:true, discovered:0}
//   - a readable repo   -> {ok:true, discovered:>0}
// See the report notes for why the snapshot-list probe can't be used pre-discover
// (it needs a container name we don't have on a fresh install).
type DiscoverResult = Awaited<ReturnType<typeof discover>>;
type SaveState = "idle" | "saving" | "saved" | "error";

function isKeyMismatch(err: string | undefined): boolean {
  return !!err && /APP_KEY/i.test(err);
}

// Shared mono text-input styling (off-site URLs, foreign location/key fields).
const offsiteInput =
  "rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono focus:outline-hidden focus:ring-1 focus:ring-accent";

// RestoreRow — a single discovered target (container or VM) with its latest
// snapshot and a per-item Restore button. The restore mechanics are the shared
// <RestoreAction> (the same control the Containers/VMs tabs use), so a recovery
// restore behaves identically to one launched from those tabs. The restore is
// IN PLACE and LEFT STOPPED (forceLeaveStopped): the recovery flow restores
// everything first, then you start them from the Containers/VMs tabs.
function RestoreRow({
  domain,
  name,
  lastBackup,
  t,
  otherActive,
}: {
  domain: "container" | "vm";
  name: string;
  lastBackup: number | null;
  t: ReturnType<typeof useT>["t"];
  otherActive: boolean;
}) {
  // Latest-backup label — DISPLAY ONLY, read straight from the target list's own
  // lastBackup field (unix seconds). No per-row snapshot fetch: a discovered list
  // of N containers + M VMs would otherwise spawn N+M concurrent restic processes
  // just for this label. The restore itself resolves "latest" on the server.
  const snapLabel = lastBackup ? new Date(lastBackup * 1000).toLocaleString() : "";

  return (
    <div className="flex flex-col gap-1 py-2 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        <span className="text-carbon-text font-medium flex-1 truncate">{name}</span>
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
}: {
  set: FileSetView;
  hostMountRoot: string;
  t: ReturnType<typeof useT>["t"];
  otherActive: boolean;
}) {
  const [target, setTarget] = useState("");
  const [state, setState] = useState<"idle" | "busy" | "ok" | "fail">("idle");
  const [error, setError] = useState<string | null>(null);

  const snapLabel = set.lastBackup ? new Date(set.lastBackup * 1000).toLocaleString() : "";

  async function handleRestore() {
    if (target.trim() === "" || state === "busy") return;
    setState("busy");
    setError(null);
    try {
      // Resolve the newest snapshot of this set now (tag-filtered server-side).
      const snaps = await fileSetSnapshots(set.id);
      const list = snaps.ok ? snaps.snapshots ?? [] : [];
      if (list.length === 0) {
        setState("fail");
        setError(snaps.error ?? t("snapshots.none"));
        return;
      }
      const latest = list.reduce((a, b) => (new Date(a.time) > new Date(b.time) ? a : b));
      const res = await fireAndWaitRun({
        kind: "restore",
        matchRun: (r) => r.domain === "files" && r.target === set.name,
        start: () => restoreFileSet(set.id, latest.id, true, target.trim()),
      });
      if (res.ok) {
        setState("ok");
      } else {
        setState("fail");
        setError(res.error ?? null);
      }
    } catch (err) {
      setState("fail");
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  return (
    <div className="flex flex-col gap-2 py-2 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        <span className="text-carbon-text font-medium flex-1 truncate">{set.name}</span>
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
          onClick={() => void handleRestore()}
          disabled={state === "busy" || otherActive || target.trim() === ""}
          className="inline-flex items-center gap-1.5 rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {state === "busy" && (
            <span
              className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
          )}
          {state === "busy" ? t("common.restoring") : t("snapshots.restore")}
        </button>
        {state === "ok" && <span className="text-xs text-[#6fdc8c]">✓ {t("common.done")}</span>}
        {state === "fail" && error && (
          <span className="text-xs text-[#ff8389] wrap-break-word">✗ {error}</span>
        )}
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

// One restorable foreign item: snapshot picker (default latest), a target
// folder for file sets (required — a foreign file set has no trusted local
// path), and a Restore button driven by fireAndWaitRun on the recorded run —
// runs land with domain "container" | "vm" | "files" exactly like local ones.
function ForeignItemRow({
  domain,
  item,
  session,
  hostMountRoot,
  existsLocally,
  t,
  blocked,
  onBusyChange,
  onSessionGone,
}: {
  domain: "containers" | "vms" | "files";
  item: ForeignItem;
  session: string;
  hostMountRoot: string;
  /** A same-named local container/VM exists — restore would overwrite it. */
  existsLocally: boolean;
  t: ReturnType<typeof useT>["t"];
  blocked: boolean;
  onBusyChange: (busy: boolean) => void;
  onSessionGone: () => void;
}) {
  const [snapshot, setSnapshot] = useState("latest");
  const [target, setTarget] = useState("");
  const [state, setState] = useState<"idle" | "busy" | "ok" | "fail">("idle");
  const [error, setError] = useState<string | null>(null);

  // The recorded run's domain strings (see handleRuns): singular for
  // containers/VMs, "files" for file sets.
  const runDomain = domain === "containers" ? "container" : domain === "vms" ? "vm" : "files";
  // Newest-first for the picker; restic lists snapshots oldest-first.
  const snaps = [...item.snapshots].reverse();

  async function handleRestore() {
    if (state === "busy" || blocked) return;
    if (domain === "files" && target.trim() === "") return;
    // Same-named local item: explicit overwrite confirm BEFORE anything fires.
    if (existsLocally && !window.confirm(t("recovery.foreignExistsConfirm").replace("{name}", item.name))) {
      return;
    }
    setState("busy");
    setError(null);
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
            target: domain === "files" ? target.trim() : undefined,
          }),
      });
      if (res.ok) {
        setState("ok");
      } else {
        setState("fail");
        setError(res.error ?? null);
        if (isForeignSessionGone(res.error)) onSessionGone();
      }
    } catch (err) {
      setState("fail");
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      onBusyChange(false);
    }
  }

  return (
    <div className="flex flex-col gap-2 py-2 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm flex-wrap">
        <span className="text-carbon-text font-medium flex-1 truncate">{item.name}</span>
        <select
          value={snapshot}
          onChange={(e) => setSnapshot(e.target.value)}
          disabled={state === "busy"}
          className="rounded-lg border border-carbon-border bg-carbon-surface2 px-2 py-1.5 text-xs text-carbon-text focus:outline-hidden focus:ring-1 focus:ring-accent"
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
        <FolderBrowser
          label={t("recovery.foreignTargetFolder")}
          value={target}
          hostMountRoot={hostMountRoot}
          onChange={setTarget}
        />
      )}
      <div className="flex items-center gap-3 flex-wrap">
        <button
          onClick={() => void handleRestore()}
          disabled={state === "busy" || blocked || (domain === "files" && target.trim() === "")}
          className="inline-flex items-center gap-1.5 rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {state === "busy" && (
            <span
              className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
          )}
          {state === "busy" ? t("common.restoring") : t("recovery.foreignRestore")}
        </button>
        {state === "ok" && <span className="text-xs text-[#6fdc8c]">✓ {t("common.done")}</span>}
        {state === "fail" && error && (
          <span className="text-xs text-[#ff8389] wrap-break-word">✗ {error}</span>
        )}
      </div>
    </div>
  );
}

// The whole foreign section: heading + two StepCards (connect, browse &
// restore). All session state is COMPONENT state — never Settings.
function ForeignRestoreCard({
  hostMountRoot,
  t,
  otherActive,
}: {
  hostMountRoot: string;
  t: ReturnType<typeof useT>["t"];
  otherActive: boolean;
}) {
  // Connect input. The backend only opens a LOCALLY MOUNTED repository, so the
  // location is always a folder under the host mount (e.g. a mounted share
  // holding the other server's backups) — no remote-URL / off-site option here.
  const [localPath, setLocalPath] = useState("");
  const [key, setKey] = useState("");

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
        setConnectError(res.error ?? t("settings.error"));
        setPhase("error");
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
          for (const v of vs.vms ?? []) names.add(`vm:${v.name}`);
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
      setConnectError(err instanceof Error ? err.message : String(err));
      setPhase("error");
    }
  }, [location, key, t]);

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
    <div className="flex flex-col gap-5 border-t border-carbon-border pt-5 mt-2">
      <div>
        <h2 className="text-lg font-semibold text-carbon-text">{t("recovery.foreignTitle")}</h2>
        <p className="text-sm text-carbon-textMuted mt-1 max-w-2xl">{t("recovery.foreignIntro")}</p>
      </div>

      {/* Foreign step 1 — connect (read-only; nothing is saved). */}
      <StepCard n={1} title={t("recovery.foreignStepConnect")} state={connectState}>
        {/* Local mounted path only — the backend never opens a remote/off-site
            repo here, so the other server's backup share must be mounted on this
            host and pointed at below. */}
        <FolderBrowser
          label={t("recovery.foreignLocation")}
          value={localPath}
          hostMountRoot={hostMountRoot}
          onChange={setLocalPath}
        />
        <p className="text-xs text-carbon-textMuted max-w-2xl">{t("recovery.foreignLocationHint")}</p>

        <div className="flex flex-col gap-1">
          <label className="text-xs text-carbon-textSub">{t("recovery.foreignKey")}</label>
          <input
            type="password"
            value={key}
            spellCheck={false}
            autoComplete="off"
            onChange={(e) => setKey(e.target.value)}
            className={offsiteInput}
          />
          <p className="text-xs text-carbon-textMuted max-w-2xl">{t("recovery.foreignKeyHint")}</p>
        </div>

        <div className="flex items-center gap-3 pt-1 flex-wrap">
          <button
            onClick={() => void connect()}
            disabled={!canConnect}
            className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
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
              <span className="text-sm text-[#6fdc8c]">{t("recovery.foreignConnected")}</span>
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
          <div className="rounded-lg bg-[#2a1c1c] border border-[#4a2a2a] px-3 py-2.5 text-xs text-[#ff8389] leading-relaxed wrap-break-word">
            {connectError}
          </div>
        )}
      </StepCard>

      {/* Foreign step 2 — browse the inventory & restore single items. */}
      <StepCard n={2} title={t("recovery.foreignStepBrowse")} state={browseState}>
        {!session || !inventory ? (
          <p className="text-sm text-carbon-textMuted">{t("recovery.foreignNotConnected")}</p>
        ) : (
          <>
            {/* Session lapsed mid-browse (30-min TTL) — offer the reconnect. */}
            {sessionGone && (
              <div className="rounded-lg bg-[#2a2a1c] border border-[#4a4a2a] px-3 py-2.5 text-xs text-[#f1c21b] leading-relaxed flex items-center gap-3 flex-wrap">
                <span className="flex-1">{t("recovery.foreignExpired")}</span>
                <button
                  type="button"
                  onClick={() => void connect()}
                  className="rounded-md bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-xs text-carbon-text transition-colors"
                >
                  {t("recovery.foreignReconnect")}
                </button>
              </div>
            )}
            {total === 0 ? (
              <p className="text-sm text-[#f1c21b]">{t("recovery.foreignEmpty")}</p>
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
                      t={t}
                      blocked={rowBlocked}
                      onBusyChange={onBusyChange}
                      onSessionGone={() => setSessionGone(true)}
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

export default function Recovery() {
  const { t } = useT();

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
  const [attachState, setAttachState] = useState<SaveState>("idle");
  const [attachError, setAttachError] = useState<string | null>(null);
  const [previewed, setPreviewed] = useState(false);

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
  const checkReadable = useCallback(async () => {
    setChecking(true);
    setLastError(null);
    try {
      const [c, v, f] = await Promise.all([discover(true), discoverVMs(true), discoverFiles(true)]);
      const results: DiscoverResult[] = [c, v, f];
      const keyErr = results.find((r) => !r.ok && isKeyMismatch(r.error));
      if (keyErr) {
        setReadableState("bad");
        setLastError(keyErr.error ?? null);
        return;
      }
      const otherErr = results.find((r) => !r.ok);
      if (otherErr) {
        setReadableState("warn");
        setLastError(otherErr.error ?? null);
        return;
      }
      const total = (c.discovered ?? 0) + (v.discovered ?? 0) + (f.discovered ?? 0);
      // >0 = repo readable with content; 0 = reachable but empty / not attached yet.
      setReadableState(total > 0 ? "ok" : "warn");
    } catch (err) {
      // Network/HTTP failure (unreachable, auth, 5xx) — not a key mismatch.
      setReadableState("warn");
      setLastError(err instanceof Error ? err.message : String(err));
    } finally {
      setChecking(false);
    }
  }, []);

  // connectPreview saves the paths/off-site/encryption fields (mirroring the
  // Settings save() merge onto the server baseline), then re-runs checkReadable
  // so Step 1's pill reflects the freshly-attached location.
  const connectPreview = useCallback(async () => {
    const base = savedSettings ?? settings;
    if (!base || !settings) return;
    setAttachState("saving");
    setAttachError(null);
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
        setAttachState("saved");
        // Keep the sidebar/Settings in sync (same event the Settings page fires).
        window.dispatchEvent(new Event("bv:settings-changed"));
        setTimeout(() => setAttachState("idle"), 3000);
        setPreviewed(true);
        // Attaching a (possibly different) repo invalidates any previously
        // discovered targets — clear them so Step 4 can never offer to restore
        // the OLD repo's data; the user must re-Discover against the new repo.
        setContainers([]);
        setVMs([]);
        setFileSets([]);
        setDiscovered(null);
        setRestoreAllResult(null);
        await checkReadable();
      } else {
        setAttachError(res.error ?? t("settings.error"));
        setAttachState("error");
      }
    } catch (err) {
      setAttachError(err instanceof Error ? err.message : t("settings.error"));
      setAttachState("error");
    }
  }, [savedSettings, settings, checkReadable, t]);

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
        setConfigError(saveRes.error ?? t("settings.error"));
        setConfigPhase("error");
        return;
      }
      setSavedSettings(updated);
      setSettings((prev) => (prev ? { ...prev, ...patch } : updated));
      const res = await restoreConfig("latest", configSource === "offsite" ? "offsite" : undefined);
      if (!res.ok) {
        // e.g. an APP_KEY / encryption mismatch — show the mapped remedy.
        setConfigError(isKeyMismatch(res.error) ? t("recovery.appKeyRemedy") : res.error ?? t("settings.error"));
        setConfigPhase("error");
        return;
      }
      if (!res.staged) {
        // Contract drift guard: ok:true but the snapshot was NOT staged — don't drive
        // the restart/reload flow (nothing would be applied). Surface it as an error.
        setConfigError(res.error ?? t("settings.error"));
        setConfigPhase("error");
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
      setConfigError(err instanceof Error ? err.message : t("settings.error"));
      setConfigPhase("error");
    }
  }, [savedSettings, settings, configSource, t]);

  const configStepState: StepState =
    configPhase === "error"
      ? "bad"
      : configPhase === "manual" || configPhase === "reload"
        ? "warn"
        : "idle";

  // Step 3 — discover everything. Runs discoverAll(), then re-fetches the target
  // lists (kept for the later review/restore step).
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
  const [restoreAllResult, setRestoreAllResult] = useState<{ ok: number; fail: number } | null>(null);

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
    if (!window.confirm(t("containers.restoreSelectedConfirm"))) return;
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
        const res = await fireAndWaitRun({
          kind: "restore",
          matchRun: (r) => r.domain === "vm" && r.target === v.name,
          start: () => restoreVM(v.name, "latest", true, undefined, true),
        });
        if (res.ok) ok++;
        else fail++;
      }
      setRestoreAllResult({ ok, fail });
    } finally {
      setRestoreAllBusy(false);
    }
  }, [restoreAllBusy, containers, vms, t]);

  const anyDiscovered = containers.length > 0 || vms.length > 0 || fileSets.length > 0;
  const restoreStepState: StepState = restoreAllResult
    ? restoreAllResult.fail > 0
      ? "warn"
      : "ok"
    : "idle";
  // Rows are blocked while ANY op runs OR while the bulk loop is mid-flight
  // (between two items the SSE store can briefly show nothing active).
  const rowOtherActive = running.active || restoreAllBusy;

  return (
    <div className="flex flex-col gap-5 p-1">
      <div>
        <h1 className="text-lg font-semibold text-carbon-text">{t("recovery.pageTitle")}</h1>
        <p className="text-sm text-carbon-textMuted mt-1 max-w-2xl">{t("recovery.intro")}</p>
      </div>

      {/* Step 1 — Can BombVault read your backups? (repo-readable / APP_KEY) */}
      <StepCard n={1} title={t("recovery.step1")} state={readableState}>
        <p className="max-w-2xl">{t("recovery.appKeyExplain")}</p>

        <div className="flex items-center gap-3 pt-1">
          <button
            onClick={() => void checkReadable()}
            disabled={checking}
            className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
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
            <span className="text-sm text-[#6fdc8c]">{t("recovery.readable")}</span>
          )}
          {readableState === "warn" && (
            <span className="text-sm text-[#f1c21b]">{t("recovery.notReachable")}</span>
          )}
        </div>

        {/* Exact remedy when the key doesn't match the repo. */}
        {readableState === "bad" && (
          <div className="rounded-lg bg-[#2a1c1c] border border-[#4a2a2a] px-3 py-2.5 text-xs text-[#ff8389] leading-relaxed">
            {t("recovery.appKeyRemedy")}
          </div>
        )}

        {/* The raw (scrubbed) backend message for a warn/other error, as a hint. */}
        {readableState === "warn" && lastError && (
          <p className="text-xs text-carbon-textMuted font-mono break-all">{lastError}</p>
        )}
      </StepCard>

      {/* Step 2 — Restore BombVault's OWN settings first (optional, pre-attach).
          On a rebuilt box this pre-fills the attach + discover steps below; it
          ends with a self-restart, so it lives here rather than on the Config
          page. Skippable — a user without a settings backup attaches manually. */}
      <StepCard n={2} title={t("recovery.stepConfig")} state={configStepState}>
        {configSkipped ? (
          <p className="text-sm text-carbon-textMuted">{t("recovery.configSkipped")}</p>
        ) : (
          <>
            <p className="max-w-2xl">{t("recovery.configHint")}</p>
            <p className="text-xs text-carbon-textMuted max-w-2xl">{t("recovery.configAppKeyReminder")}</p>

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
                      className={offsiteInput}
                    />
                  </div>
                )}

                <div className="flex flex-wrap items-center gap-3 pt-1">
                  <button
                    onClick={() => void restoreOwnConfig()}
                    disabled={configPhase === "saving" || configPhase === "restarting"}
                    className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
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
                    <p className="text-sm text-[#78a9ff]">{t("recovery.configRestarting")}</p>
                    <button
                      type="button"
                      onClick={() => window.location.reload()}
                      className="self-start text-xs text-carbon-textSub hover:text-carbon-text transition-colors underline"
                    >
                      {t("recovery.configReload")}
                    </button>
                  </div>
                )}
                {/* Manual restart needed (Docker socket unreachable). */}
                {configPhase === "manual" && (
                  <div className="rounded-lg bg-[#2a2a1c] border border-[#4a4a2a] px-3 py-2.5 text-xs text-[#f1c21b] leading-relaxed">
                    {t("recovery.configManualRestart")}
                  </div>
                )}
                {/* Auto-restart poll timed out — offer a manual reload. */}
                {configPhase === "reload" && (
                  <div className="flex flex-wrap items-center gap-3">
                    <span className="text-xs text-[#f1c21b]">{t("recovery.configReloadWhenBack")}</span>
                    <button
                      type="button"
                      onClick={() => window.location.reload()}
                      className="rounded-md bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors"
                    >
                      {t("recovery.configReload")}
                    </button>
                  </div>
                )}
                {configPhase === "error" && configError && (
                  <div className="rounded-lg bg-[#2a1c1c] border border-[#4a2a2a] px-3 py-2.5 text-xs text-[#ff8389] leading-relaxed wrap-break-word">
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
      <StepCard n={3} title={t("recovery.step2")} state={previewed ? readableState : "idle"}>
        <p className="max-w-2xl">{t("recovery.attachHint")}</p>

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
                  className={offsiteInput}
                />
              </div>
            ))}

            {/* Encryption on/off (reuses the Settings ToggleRow). */}
            <div className="pt-1">
              <ToggleRow
                label={
                  settings.encryptionEnabled
                    ? t("settings.encryptionOn")
                    : t("settings.encryptionOff")
                }
                checked={settings.encryptionEnabled}
                onChange={(v) => setSettings((prev) => (prev ? { ...prev, encryptionEnabled: v } : prev))}
              />
            </div>

            {/* Cloud + rclone credential cards — the exact Settings components,
                self-persisting via setCloud/setRclone (no duplicate persistence). */}
            <CloudCard t={t} />
            <RcloneCard t={t} />

            {/* Credentials save via each card's OWN Save button, not "Connect &
                preview" below — flag it so the user saves creds first. */}
            <p className="text-xs text-carbon-textMuted max-w-2xl">{t("recovery.credsSaveHint")}</p>

            {/* Connect & preview — save paths/off-site/encryption, then re-check. */}
            <div className="flex items-center gap-3 pt-1">
              <button
                onClick={() => void connectPreview()}
                disabled={attachState === "saving"}
                className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
              >
                {attachState === "saving" && (
                  <span
                    className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
                    style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
                  />
                )}
                {t("recovery.connectPreview")}
              </button>
              {attachState === "saved" && previewed && readableState === "ok" && (
                <span className="text-sm text-[#6fdc8c]">{t("recovery.readable")}</span>
              )}
              {attachState === "error" && attachError && (
                <span className="text-sm text-[#ff8389]">{attachError}</span>
              )}
            </div>
          </>
        ) : (
          <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
        )}
      </StepCard>

      {/* Step 4 — Discover everything (rebuild targets from the backup defs) */}
      <StepCard n={4} title={t("recovery.step3")} state={discoverStepState}>
        <div className="flex items-center gap-3">
          <button
            onClick={() => void runDiscover()}
            disabled={discovering}
            className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
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
            <span className="text-sm text-[#6fdc8c]">
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
          <p className="text-sm text-[#f1c21b]">{t("recovery.foundNone")}</p>
        )}
        {discoverError && (
          <div className="rounded-lg bg-[#2a1c1c] border border-[#4a2a2a] px-3 py-2.5 text-xs text-[#ff8389] leading-relaxed wrap-break-word">
            {discoverError}
          </div>
        )}
      </StepCard>

      {/* Step 5 — Review & restore everything (in place, left stopped) */}
      <StepCard n={5} title={t("recovery.step4")} state={restoreStepState}>
        {!anyDiscovered ? (
          <p className="text-sm text-carbon-textMuted">{t("recovery.noneDiscovered")}</p>
        ) : (
          <>
            {/* Restore all — every container then VM, sequential + left stopped.
                Shown ONLY when there are containers/VMs to bulk-restore: file
                sets carry no original path, so they're restored per-row (below)
                into a chosen folder and restoreAll() deliberately skips them. */}
            {(containers.length > 0 || vms.length > 0) && (
            <div className="flex flex-wrap items-center gap-3">
              <button
                onClick={() => void restoreAll()}
                disabled={restoreAllBusy || running.active}
                className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {restoreAllBusy && (
                  <span
                    className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
                    style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
                  />
                )}
                {t("recovery.restoreAll")}
              </button>
              {running.active && !restoreAllBusy && (
                <span className="text-xs text-carbon-textMuted">{t(busyPhraseKey(running.phase))}</span>
              )}
              {restoreAllResult && (
                <span
                  className={`text-sm ${restoreAllResult.fail > 0 ? "text-[#f1c21b]" : "text-[#6fdc8c]"}`}
                >
                  {t("recovery.restoreAllResult")
                    .replace("{ok}", String(restoreAllResult.ok))
                    .replace("{fail}", String(restoreAllResult.fail))}
                </span>
              )}
            </div>
            )}

            {/* VM restore needs the libvirt SSH link — advisory note, not a block. */}
            {vms.length > 0 && vmSshConfigured === false && (
              <div className="rounded-lg bg-[#2a2a1c] border border-[#4a4a2a] px-3 py-2.5 text-xs text-[#f1c21b] leading-relaxed">
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
                    key={`vm:${v.name}`}
                    domain="vm"
                    name={v.name}
                    lastBackup={v.lastBackup}
                    t={t}
                    otherActive={rowOtherActive}
                  />
                ))}
              </div>
            )}
            {/* File sets — restore into a chosen folder ("Restore all" covers
                containers + VMs only; a rediscovered set has no original path,
                so each row needs its own target folder). */}
            {fileSets.length > 0 && (
              <div className="flex flex-col">
                <span className="text-xs font-medium text-carbon-textSub pt-2 pb-1">
                  {t("nav.files")}
                </span>
                <p className="text-xs text-carbon-textMuted pb-1">
                  {t("recovery.filesRestoreHint")}
                </p>
                {fileSets.map((s) => (
                  <FileSetRecoveryRow
                    key={`files:${s.id}`}
                    set={s}
                    hostMountRoot={hostMountRoot}
                    t={t}
                    otherActive={rowOtherActive}
                  />
                ))}
              </div>
            )}
          </>
        )}
      </StepCard>

      {/* Step 6 — Your recovery kit (safety net for next time) */}
      <StepCard n={6} title={t("recovery.step5")} state="idle">
        <p className="max-w-2xl">{t("recovery.kitHint")}</p>
        <a
          href={recoveryKitUrl()}
          download="bombvault-recovery-kit.md"
          className="self-start rounded-md bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors"
        >
          {t("recovery.kitDownload")}
        </a>
      </StepCard>

      {/* Restore from ANOTHER BombVault repo (#61) — visually separate from the
          attach steps above; read-only session, nothing persisted. */}
      <ForeignRestoreCard hostMountRoot={hostMountRoot} t={t} otherActive={rowOtherActive} />
    </div>
  );
}
