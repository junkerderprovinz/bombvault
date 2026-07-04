import { useCallback, useEffect, useRef, useState } from "react";
import { useT } from "../lib/i18n";
import { StepCard, type StepState } from "../components/recovery/StepCard";
import { FolderBrowser } from "../components/FolderBrowser";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { CloudCard, RcloneCard, ToggleRow } from "./Settings";
import { ProgressBar } from "../components/ProgressBar";
import { RestoreCancelButton } from "../components/RestoreCancelButton";
import { useBackupWatch, fireAndWaitRun } from "../lib/backupWatch";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import {
  discover,
  discoverVMs,
  discoverAll,
  getSettings,
  putSettings,
  listContainers,
  listVMs,
  restore,
  restoreVM,
  restoreConfig,
  waitForAppBack,
  getVMSSH,
  recoveryKitUrl,
  type Settings,
  type Container,
  type VM,
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

// RestoreRow — a single discovered target (container or VM) with its latest
// snapshot and a per-item Restore button. It mirrors the Containers/VMs
// RestorePanel EXACTLY: useBackupWatch drives the async fire-and-watch, the v4
// SSE ProgressBar shows live progress, and RestoreCancelButton cancels — so a
// recovery restore behaves identically to one launched from those tabs. The
// restore is IN PLACE and LEFT STOPPED (leaveStopped=true): the recovery flow
// restores everything first, then you start them from the Containers/VMs tabs.
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
  const progressKey = `${domain}:${name}`;
  const cancelledRef = useRef(false);
  const { state, fire, isPending } = useBackupWatch({
    progressKey,
    kind: "restore",
    // Same single-restore call shape as the Containers/VMs RestorePanel, with
    // the leave-stopped flag set (restore(name,"latest",true,undefined,true) /
    // restoreVM(name,"latest",true,undefined,true)).
    start: () =>
      domain === "container"
        ? restore(name, "latest", true, undefined, true)
        : restoreVM(name, "latest", true, undefined, true),
    matchRun: (r) => r.domain === domain && r.target === name,
    cancelledRef,
  });
  const prog = useProgress()[progressKey];
  const blockedByOther = otherActive && !isPending;

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
        <button
          onClick={() => void fire()}
          disabled={isPending || blockedByOther || state.phase === "success"}
          className="inline-flex items-center gap-1.5 rounded-lg bg-accent px-2.5 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed shrink-0"
        >
          {isPending ? (
            <>
              <span
                className="h-2.5 w-2.5 rounded-full border-2 border-t-transparent animate-spin inline-block"
                style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
              />
              {t("common.restoring")}
            </>
          ) : (
            t("snapshots.restore")
          )}
        </button>
      </div>
      {isPending && (
        <div className="flex flex-col gap-1">
          {prog?.phase === "restore" && prog.active && (
            <ProgressBar
              percent={prog.percent}
              active
              inline
              label={prog.percent > 0 ? t("restore.progress").replace("{pct}", String(Math.round(prog.percent))) : undefined}
            />
          )}
          <RestoreCancelButton cancelKey={progressKey} inPlace name={name} t={t} cancelledRef={cancelledRef} />
        </div>
      )}
      {state.phase === "success" && <p className="text-xs text-[#6fdc8c]">{t("common.done")}</p>}
      {state.phase === "cancelled" && (
        <p className="text-xs text-carbon-textSub break-words">{t("restore.cancelled")}</p>
      )}
      {state.phase === "error" && <p className="text-xs text-[#ff8389] break-words">{state.message}</p>}
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
  // Step 1's "Re-check" and Step 2's "Connect & preview".
  const checkReadable = useCallback(async () => {
    setChecking(true);
    setLastError(null);
    try {
      const [c, v] = await Promise.all([discover(), discoverVMs()]);
      const results: DiscoverResult[] = [c, v];
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
      const total = (c.discovered ?? 0) + (v.discovered ?? 0);
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
      containersOffsite: settings.containersOffsite,
      vmsOffsite: settings.vmsOffsite,
      flashOffsite: settings.flashOffsite,
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
  const [discovered, setDiscovered] = useState<{ containers: number; vms: number } | null>(null);
  const [discoverError, setDiscoverError] = useState<string | null>(null);
  // Reconstructed target lists — populated by Discover, read by the review step.
  const [containers, setContainers] = useState<Container[]>([]);
  const [vms, setVMs] = useState<VM[]>([]);

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
      const [cs, vs] = await Promise.all([listContainers(), listVMs()]);
      setContainers(cs.containers ?? []);
      setVMs(vs.vms ?? []);
      setDiscovered(counts);
    } catch (err) {
      setDiscoverError(err instanceof Error ? err.message : String(err));
      setDiscovered(null);
    } finally {
      setDiscovering(false);
    }
  }, [t]);

  const discoverStepState: StepState = discovered
    ? discovered.containers + discovered.vms > 0
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

  const anyDiscovered = containers.length > 0 || vms.length > 0;
  const restoreStepState: StepState = restoreAllResult
    ? restoreAllResult.fail > 0
      ? "warn"
      : "ok"
    : "idle";
  // Rows are blocked while ANY op runs OR while the bulk loop is mid-flight
  // (between two items the SSE store can briefly show nothing active).
  const rowOtherActive = running.active || restoreAllBusy;

  const offsiteInput =
    "rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono focus:outline-none focus:ring-1 focus:ring-accent";

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

                {/* Restarting — optimistic; waitForAppBack() reloads on return. */}
                {configPhase === "restarting" && (
                  <p className="text-sm text-[#78a9ff]">{t("recovery.configRestarting")}</p>
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
                  <div className="rounded-lg bg-[#2a1c1c] border border-[#4a2a2a] px-3 py-2.5 text-xs text-[#ff8389] leading-relaxed break-words">
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

            {/* Off-site repo URLs (rest / S3 / B2 / sftp / rclone). */}
            <span className="text-xs font-medium text-carbon-textSub pt-1">{t("settings.offsiteTitle")}</span>
            {([
              ["containersOffsite", "nav.containers"],
              ["vmsOffsite", "nav.vms"],
              ["flashOffsite", "nav.flash"],
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

          {discovered && discovered.containers + discovered.vms > 0 && (
            <span className="text-sm text-[#6fdc8c]">
              {t("recovery.foundCounts")
                .replace("{c}", String(discovered.containers))
                .replace("{v}", String(discovered.vms))}
            </span>
          )}
        </div>

        {/* 0/0 — nothing found: point back to Step 1/2. */}
        {discovered && discovered.containers + discovered.vms === 0 && (
          <p className="text-sm text-[#f1c21b]">{t("recovery.foundNone")}</p>
        )}
        {discoverError && (
          <div className="rounded-lg bg-[#2a1c1c] border border-[#4a2a2a] px-3 py-2.5 text-xs text-[#ff8389] leading-relaxed break-words">
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
            {/* Restore all — every container then VM, sequential + left stopped. */}
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
    </div>
  );
}
