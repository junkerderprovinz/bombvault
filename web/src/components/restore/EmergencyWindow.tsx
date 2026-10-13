// The emergency window brings a lost or rebuilt server back in four steps:
// the key of its backups, BombVault's own settings, the choice of what comes
// back, and the restore. Each step can be opened on its own, because the
// settings come back with a restart that ends the first one.
import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import {
  discoverAll,
  downloadRecoveryKit,
  fileSetSnapshots,
  cancelEmergency,
  finishEmergency,
  getEmergency,
  getSettings,
  getVMSSH,
  listContainers,
  listFileSets,
  listVMs,
  listZFSDatasets,
  putSettings,
  restore,
  restoreConfig,
  restoreFileSet,
  restoreVM,
  undoEmergency,
  waitForAppBack,
  type Container,
  type EmergencyState,
  type FileSetView,
  type Settings,
  type VM,
  type ZFSDatasetView,
} from "../../lib/api";
import { fireAndWaitRun } from "../../lib/backupWatch";
import { lossText, refusalText, sessionKeyInUse, settingsOfPlaces, takenOver } from "../../lib/emergency";
import { useT, type TranslationKey } from "../../lib/i18n";
import { placementChanged } from "../../lib/placementEvents";
import { anyActive, busyPhraseKey, useProgress } from "../../lib/progress";
import { EMERGENCY_STEPS, emergencyPath, restoreFolderFor, type EmergencyStep, type EmergencyWay } from "../../lib/restorePlan";
import { isRuntimeRefusal } from "../../lib/runReason";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useOwnLogin } from "../../lib/useOwnLogin";
import { checkRestoreOnce, restoreBlockReason } from "../../lib/useRestoreCheck";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { ToggleRow } from "../../pages/settings/shared";
import { KindGlyph } from "../backups/kinds";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { FolderBrowser } from "../FolderBrowser";
import { IconBack, IconForward } from "../glyphs";
import { IconCheck, IconDownload, IconRestore } from "../navGlyphs";
import { DiscoverFindings } from "../placement/DiscoverFindings";
import { ProgressBar } from "../ProgressBar";
import { RadioMark } from "../RadioMark";
import { SourceToggle, type RepoSource } from "../SourceToggle";
import { Rows, SettingRow } from "../storage/rows";
import { Toggle } from "../Toggle";
import { DumpOnlyRow } from "./DumpOnlyRow";
import { EmergencyAccess } from "./EmergencyAccess";
import { EMERGENCY_FIELD, Note, SECTION_OFFSITE, SECTION_PATHS, Section } from "./EmergencyParts";
import { RuntimeRetry } from "./RuntimeRetry";
import { StepBar } from "./StepBar";

const STEP_LABEL: Record<EmergencyStep, TranslationKey> = {
  access: "emergency.access",
  settings: "nav.settings",
  selection: "emergency.selection",
  restore: "snapshots.restore",
};

type Found = Awaited<ReturnType<typeof discoverAll>>;
type ConfigPhase = "idle" | "saving" | "restarting" | "manual" | "reload" | "error";

interface Picked {
  kind: "container" | "vm" | "files";
  /** What the restore route takes. */
  id: string;
  name: string;
  /** What the recorded run names the entry by. */
  runTarget: string;
}

type Outcome = "waiting" | "running" | "done" | "failed";

function isKeyMismatch(err: string | undefined): boolean {
  return !!err && /APP_KEY/i.test(err);
}

function Choice({ on, title, sub, onPick }: { on: boolean; title: string; sub: string; onPick: () => void }) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={on}
      onClick={onPick}
      className={`flex items-center gap-3 rounded-control bg-carbon-surface2 p-3 text-start ring-2 ring-inset transition-colors hover:bg-carbon-surface3 ${
        on ? "ring-accent" : "ring-transparent"
      }`}
    >
      <RadioMark on={on} />
      <span className="flex min-w-0 flex-col">
        <span className="text-sm font-semibold text-carbon-text wrap-anywhere">{title}</span>
        <span className="text-xs text-carbon-textMuted wrap-anywhere">{sub}</span>
      </span>
    </button>
  );
}

function PickRow({
  kind,
  name,
  on,
  onToggle,
  children,
}: {
  kind: Picked["kind"];
  name: string;
  on: boolean;
  onToggle: (on: boolean) => void;
  children?: ReactNode;
}) {
  return (
    <div data-testid="emergency-pick" className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0">
      <div className="flex items-center gap-3">
        <span className="flex h-9 w-9 flex-none items-center justify-center rounded-control bg-carbon-surface3 text-accentText [&_svg]:h-[18px] [&_svg]:w-[18px]">
          <KindGlyph kind={kind} />
        </span>
        <span className="min-w-0 flex-1 truncate text-sm font-semibold text-carbon-text">{name}</span>
        <Toggle checked={on} onChange={onToggle} label={name} hideLabel />
      </div>
      {children}
    </div>
  );
}

export function EmergencyWindow({
  start,
  way,
  onClose,
  onOldFolder,
  onPairing,
}: {
  start: EmergencyStep;
  /** The way into the first step a link names. */
  way?: EmergencyWay | null;
  onClose: () => void;
  /** Opens the window that reads another install's backups read-only. */
  onOldFolder?: () => void;
  /** Leads to the page where this install joins a group. */
  onPairing?: () => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [step, setStep] = useState<EmergencyStep>(start);

  const [settings, setSettings] = useState<Settings | null>(null);
  const [hostMountRoot, setHostMountRoot] = useState("/host/user");

  const [emergency, setEmergency] = useState<EmergencyState | null>(null);
  const session = emergency?.session;
  const taken = takenOver(emergency);
  // The key was shown and is neither taken over nor the one in use, so no
  // later step can read a backup yet.
  const undecided = session !== undefined && !sessionKeyInUse(emergency);
  const [leaving, setLeaving] = useState(false);
  const ownLogin = useOwnLogin();
  // Where the first step puts its own buttons, ahead of the window's Next.
  const [accessFoot, setAccessFoot] = useState<HTMLElement | null>(null);

  const [useBackup, setUseBackup] = useState(true);
  const [configSource, setConfigSource] = useState<RepoSource>("local");
  const [configPhase, setConfigPhase] = useState<ConfigPhase>("idle");
  const [configError, setConfigError] = useState<string | null>(null);

  const [found, setFound] = useState<Found | null>(null);
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);
  const [containers, setContainers] = useState<Container[]>([]);
  const [vms, setVMs] = useState<VM[]>([]);
  const [fileSets, setFileSets] = useState<FileSetView[]>([]);
  const [zfsItems, setZFSItems] = useState<ZFSDatasetView[]>([]);
  const [on, setOn] = useState<ReadonlySet<string>>(new Set());
  const [folders, setFolders] = useState<Record<string, string>>({});
  const [keepStopped, setKeepStopped] = useState(true);
  const [vmSsh, setVmSsh] = useState<boolean | null>(null);

  const [queue, setQueue] = useState<Picked[]>([]);
  const [outcomes, setOutcomes] = useState<Record<string, Outcome>>({});
  const [finished, setFinished] = useState(false);
  const [runtimeRefused, setRuntimeRefused] = useState<string[]>([]);
  const [kitError, setKitError] = useState<string | null>(null);

  const running = anyActive(useProgress());
  const restoring = queue.length > 0 && !finished;
  const searched = useRef(false);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) {
          setSettings(res.settings);
          if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
        }
      })
      .catch(() => undefined);
    getEmergency()
      .then((res) => {
        if (res.ok && res.emergency) setEmergency(res.emergency);
      })
      .catch(() => undefined);
    // VM restore needs the libvirt SSH link. A missing one is a note, never a lock.
    getVMSSH()
      .then((r) => setVmSsh(r.ok && !!r.host))
      .catch(() => setVmSsh(false));
  }, []);

  // A save sends the whole settings, so it starts from a fresh read. One
  // taken when the window opened would roll back what changed since.
  async function saveOnto(patch: Partial<Settings>): Promise<string | null> {
    const latest = await getSettings();
    if (!latest.ok) return latest.error ?? t("config.loadSettingsFailed");
    const res = await putSettings({ ...latest.settings, ...patch });
    if (!res.ok) return res.error ?? t("settings.error");
    setSettings((prev) => (prev ? { ...prev, ...patch } : prev));
    return null;
  }

  // What a recovery kit says about each section goes into the fields the
  // next steps start from, also when the kit came out of a deposit. A typed
  // place is in them already.
  const kitPlaces = session && session.source !== "typed" ? session.places : undefined;
  useEffect(() => {
    if (!kitPlaces) return;
    const named = settingsOfPlaces(kitPlaces);
    setSettings((prev) => (prev ? { ...prev, ...named } : prev));
    if (!named.configPath && named.configOffsite) setConfigSource("offsite");
  }, [kitPlaces]);

  // Without a settings backup the places of the first step are all this
  // server learns about the old one, so they are saved before the search.
  async function attachPlaces(): Promise<boolean> {
    if (!settings || !session) return true;
    const patch: Partial<Settings> = {};
    for (const [name] of [...SECTION_PATHS, ...SECTION_OFFSITE]) patch[name] = settings[name];
    if (session.places.some((place) => place.state === "readable")) patch.encryptionEnabled = session.encrypted;
    const refused = await saveOnto(patch).catch((err: unknown) => (err instanceof Error ? err.message : t("settings.error")));
    if (refused !== null) {
      push(refused, "fail");
      return false;
    }
    window.dispatchEvent(new Event("bv:settings-changed"));
    setFound(null);
    searched.current = false;
    return true;
  }

  // Closing ends the emergency. A key that was only shown is cancelled and
  // leaves the server's memory, and one that was taken over is kept unless
  // the person undoes it, which works until the first backup under it.
  function leave() {
    if (taken && emergency?.undoable) {
      setLeaving(true);
      return;
    }
    if (session) void (taken ? finishEmergency(session.id) : cancelEmergency(session.id)).catch(() => undefined);
    onClose();
  }

  // After the restart a settings restore ends in there is no session, and
  // finishing only clears what the restore could not bring over.
  async function finish() {
    await finishEmergency(session?.id ?? "").catch(() => undefined);
    onClose();
  }

  async function undoAndClose() {
    try {
      const res = await undoEmergency();
      if (res.emergency) setEmergency(res.emergency);
      if (!res.ok) {
        if (!ownLogin.handled(res)) push(refusalText(t, res.code, res.error), "fail");
        setLeaving(false);
        // A backup ran since the window read whether the key can be given back.
        const now = await getEmergency();
        if (now.ok && now.emergency) setEmergency(now.emergency);
        return;
      }
      push(t("emergency.undone"), "success");
      onClose();
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  async function restoreOwnSettings() {
    if (!settings) return;
    const fail = (message: string) => {
      setConfigError(message);
      setConfigPhase("error");
      push(message, "fail");
    };
    setConfigPhase("saving");
    setConfigError(null);
    const offsite = configSource === "offsite";
    try {
      // The location is saved first, so the server restores from the right repository.
      const refused = await saveOnto(offsite ? { configOffsite: settings.configOffsite } : { configPath: settings.configPath });
      if (refused !== null) return fail(refused);
      const check = await checkRestoreOnce({ kind: "config", snapshotId: "latest", source: offsite ? "offsite" : undefined });
      const blocked = restoreBlockReason(check, t);
      if (blocked !== undefined) {
        const failed = check.state.phase === "done" ? check.state.result.checks?.find((c) => c.status === "fail") : undefined;
        const detail = check.state.phase === "error" ? check.state.error : failed?.detail;
        return fail(failed?.id === "key" ? t("emergency.keyRemedy") : [blocked, detail].filter(Boolean).join(" "));
      }
      // With an emergency the settings are the old install's, which the
      // server takes over under this server's key and login.
      const res = await restoreConfig("latest", offsite ? "offsite" : undefined, session?.id);
      if (!res.ok) return fail(res.code ? refusalText(t, res.code, res.error) : isKeyMismatch(res.error) ? t("emergency.keyRemedy") : (res.error ?? t("settings.error")));
      if (res.lost) setEmergency((prev) => (prev ? { ...prev, lost: res.lost } : prev));
      // Without a staged snapshot the restart would apply nothing.
      if (!res.staged) return fail(res.error ?? t("settings.error"));
      if (!res.autoRestart) {
        setConfigPhase("manual");
        return;
      }
      setConfigPhase("restarting");
      // The restored settings fill the next step, so the page comes back on it.
      if (await waitForAppBack()) window.location.assign(emergencyPath("selection"));
      else setConfigPhase("reload");
    } catch (err) {
      fail(err instanceof Error ? err.message : t("settings.error"));
    }
  }

  const search = useCallback(async () => {
    setSearching(true);
    setSearchError(null);
    try {
      const counts = await discoverAll();
      // A search that failed in one place keeps what it found in the others.
      if (counts.error) setSearchError(isKeyMismatch(counts.error) ? t("emergency.keyRemedy") : counts.error);
      const [cs, vs, fs, zs] = await Promise.all([listContainers(), listVMs(), listFileSets(), listZFSDatasets()]);
      const sets = fs.ok ? (fs.fileSets ?? []) : [];
      setContainers(cs.containers ?? []);
      setVMs(vs.vms ?? []);
      setFileSets(sets);
      setZFSItems(zs.ok ? (zs.datasets ?? []) : []);
      // What this server still has would be replaced, so only what it lacks
      // starts switched on.
      setOn(
        new Set([
          ...(cs.containers ?? []).filter((c) => !c.installed && !c.dumpOnly && !c.self).map((c) => `container:${c.name}`),
          ...(vs.vms ?? []).filter((v) => v.state === "not-installed").map((v) => `vm:${v.libvirtName}`),
          ...sets.filter((s) => s.path === "").map((s) => `files:${s.id}`),
        ])
      );
      setFound(counts);
      if (counts.paused.length > 0 || counts.leftOpen.length > 0) placementChanged();
    } catch (err) {
      setSearchError(err instanceof Error ? err.message : String(err));
    } finally {
      setSearching(false);
    }
  }, [t]);

  useEffect(() => {
    if (step !== "selection" || searched.current) return;
    searched.current = true;
    void search();
  }, [step, search]);

  const restorable = containers.filter((c) => !c.dumpOnly && !c.self && c.lastBackup);
  const dumpOnly = containers.filter((c) => c.dumpOnly);
  const backedUpVMs = vms.filter((v) => v.lastBackup);
  const folderOf = (set: FileSetView) =>
    folders[set.id] ?? (settings ? `${restoreFolderFor(settings, "files")}/${set.name}` : "");

  const picked: Picked[] = [
    ...restorable
      .filter((c) => on.has(`container:${c.name}`))
      .map((c) => ({ kind: "container" as const, id: c.name, name: c.name, runTarget: c.name })),
    // The host and the recorded run know a VM by its libvirt name.
    ...backedUpVMs
      .filter((v) => on.has(`vm:${v.libvirtName}`))
      .map((v) => ({ kind: "vm" as const, id: v.libvirtName, name: v.name, runTarget: v.libvirtName })),
    ...fileSets
      .filter((s) => on.has(`files:${s.id}`))
      .map((s) => ({ kind: "files" as const, id: s.id, name: s.name, runTarget: s.name })),
  ];
  const keyOf = (p: Picked) => `${p.kind}:${p.id}`;
  const missingFolder = fileSets.some((s) => on.has(`files:${s.id}`) && folderOf(s).trim() === "");

  function toggle(key: string, next: boolean) {
    setOn((prev) => {
      const set = new Set(prev);
      if (next) set.add(key);
      else set.delete(key);
      return set;
    });
  }

  async function restoreOne(p: Picked): Promise<{ ok: boolean; error?: string }> {
    const matchRun = (r: { domain: string; target: string }) => r.domain === p.kind && r.target === p.runTarget;
    if (p.kind === "container") {
      return fireAndWaitRun({ kind: "restore", matchRun, start: () => restore(p.id, "latest", true, undefined, keepStopped), t });
    }
    if (p.kind === "vm") {
      return fireAndWaitRun({ kind: "restore", matchRun, start: () => restoreVM(p.id, "latest", true, undefined, keepStopped), t });
    }
    // The files route takes no "latest", so the newest snapshot of the set is
    // looked up when its turn comes.
    const snaps = await fileSetSnapshots(p.id);
    const list = snaps.ok ? (snaps.snapshots ?? []) : [];
    if (list.length === 0) return { ok: false, error: snaps.error ?? t("snapshots.none") };
    const latest = list.reduce((a, b) => (new Date(a.time) > new Date(b.time) ? a : b));
    const target = folderOf(fileSets.find((s) => s.id === p.id) as FileSetView).trim();
    const check = await checkRestoreOnce({ kind: "fileSet", name: p.id, snapshotId: latest.id, targetPath: target });
    const blocked = restoreBlockReason(check, t);
    if (blocked !== undefined) return { ok: false, error: blocked };
    return fireAndWaitRun({ kind: "restore", matchRun, start: () => restoreFileSet(p.id, latest.id, true, target), t });
  }

  // One at a time: each restore waits for the one before it, so the server
  // never turns the next away as already running.
  async function restoreAll() {
    const replaces =
      restorable.some((c) => c.installed && on.has(`container:${c.name}`)) ||
      backedUpVMs.some((v) => v.state !== "not-installed" && on.has(`vm:${v.libvirtName}`));
    if (replaces) {
      const question = `${t("backups.restore.confirm")} ${t("backups.restore.alsoVMs")}`;
      if (!(await confirm(question, { confirmKey: "snapshots.restore" }))) return;
    }
    const list = picked;
    setQueue(list);
    setOutcomes(Object.fromEntries(list.map((p) => [keyOf(p), "waiting" as Outcome])));
    setFinished(false);
    setRuntimeRefused([]);
    setStep("restore");
    const refused: string[] = [];
    for (const p of list) {
      setOutcomes((prev) => ({ ...prev, [keyOf(p)]: "running" }));
      const res = await restoreOne(p).catch((err: unknown) => ({ ok: false, error: err instanceof Error ? err.message : String(err) }));
      setOutcomes((prev) => ({ ...prev, [keyOf(p)]: res.ok ? "done" : "failed" }));
      if (p.kind === "container" && isRuntimeRefusal(res.error)) refused.push(p.id);
    }
    setRuntimeRefused(refused);
    setFinished(true);
  }

  function downloadKit() {
    setKitError(null);
    void downloadRecoveryKit().then((err) => {
      setKitError(err);
      if (err) push(err, "fail");
    });
  }

  const configBusy = configPhase === "saving" || configPhase === "restarting";
  const reloadable = configPhase === "reload" || configPhase === "restarting";
  const at = EMERGENCY_STEPS.indexOf(step) + 1;
  const total = found ? found.containers + found.vms + found.files + found.zfs : 0;
  const offered = restorable.length + backedUpVMs.length + fileSets.length + zfsItems.length + dumpOnly.length;
  const count = (kind: Picked["kind"]) => picked.filter((p) => p.kind === kind).length;
  const words = [
    count("container") > 0 && t("emergency.count.containers", count("container")),
    count("vm") > 0 && t("emergency.count.vms", count("vm")),
    count("files") > 0 && t("emergency.count.sets", count("files")),
  ].filter((w): w is string => !!w);
  const doneCount = queue.filter((p) => outcomes[keyOf(p)] === "done" || outcomes[keyOf(p)] === "failed").length;
  const failedCount = queue.filter((p) => outcomes[keyOf(p)] === "failed").length;
  const lost = lossText(t, lang, emergency?.lost);

  const cancel = <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={leave} />;
  const back = (to: EmergencyStep) => (
    <Button label={t("common.back")} labelKey="common.back" glyph={<IconBack />} tone="subtle" onClick={() => setStep(to)} />
  );
  const next = (to: EmergencyStep, go: () => void = () => setStep(to)) => (
    <Button
      label={t("zfs.replica.add.next")}
      labelKey="zfs.replica.add.next"
      glyph={<IconForward />}
      tone="accent"
      onClick={go}
    />
  );

  let footer: ReactNode;
  if (step === "access") {
    footer = (
      <>
        {cancel}
        <span ref={setAccessFoot} className="contents" />
        {!undecided && next("settings")}
      </>
    );
  } else if (step === "settings") {
    footer = (
      <>
        {back("access")}
        {useBackup ? (
          <Button
            label={t("emergency.settingsGo")}
            labelKey={null}
            glyph={<IconRestore />}
            tone={reloadable ? "neutral" : "accent"}
            onClick={() => void restoreOwnSettings()}
            disabled={!settings || configBusy}
            busy={configBusy}
          />
        ) : (
          next("selection", () => void attachPlaces().then((ok) => ok && setStep("selection")))
        )}
        {reloadable && (
          <Button
            label={t("recovery.configReload")}
            labelKey="recovery.configReload"
            tone="accent"
            onClick={() => window.location.reload()}
          />
        )}
      </>
    );
  } else if (step === "selection") {
    footer = (
      <>
        {back("settings")}
        <Button
          label={t("snapshots.restore")}
          labelKey="snapshots.restore"
          glyph={<IconRestore />}
          tone="accent"
          onClick={() => void restoreAll()}
          disabled={picked.length === 0 || missingFolder || running.active}
          hint={running.active ? t(busyPhraseKey(running.phase, running.stage)) : undefined}
        />
      </>
    );
  } else {
    footer = (
      <Button
        label={t(finished ? "common.done" : "common.close")}
        labelKey={finished ? "common.done" : "common.close"}
        glyph={finished ? <IconCheck /> : undefined}
        tone="accent"
        onClick={() => void (finished ? finish() : onClose())}
      />
    );
  }
  if (leaving) {
    footer = (
      <>
        <Button label={t("common.back")} labelKey="common.back" glyph={<IconBack />} tone="subtle" onClick={() => setLeaving(false)} />
        <Button label={t("emergency.undo")} labelKey="emergency.undo" onClick={() => void undoAndClose()} />
        <Button label={t("emergency.keep")} labelKey={null} glyph={<IconCheck />} tone="accent" onClick={() => void finish()} />
      </>
    );
  }

  return (
    <InstanceWindow title={t("emergency.title")} onClose={leave} footer={footer}>
      <StepBar
        steps={EMERGENCY_STEPS.map((s) => t(STEP_LABEL[s]))}
        current={at}
        reach={restoring || leaving ? 0 : undecided ? 1 : queue.length > 0 ? 4 : 3}
        label={t("emergency.title")}
        onGo={(n) => setStep(EMERGENCY_STEPS[n - 1])}
      />
      {leaving && <Note tone="warn">{t("emergency.leaveAsk")}</Note>}

      {step === "access" && !leaving && (
        <EmergencyAccess
          emergency={emergency}
          onEmergency={setEmergency}
          settings={settings}
          setSettings={setSettings}
          hostMountRoot={hostMountRoot}
          footer={accessFoot}
          start={way}
          onPairing={onPairing}
          onOldFolder={
            onOldFolder &&
            (() => {
              if (session) void cancelEmergency(session.id).catch(() => undefined);
              onOldFolder();
            })
          }
        />
      )}

      {step === "settings" && !leaving && (
        <>
          <p className="text-sm text-carbon-text">{t("emergency.settingsIntro")}</p>
          {configPhase === "restarting" ? (
            <Note tone="neutral">{t("emergency.restarting")}</Note>
          ) : (
            <div role="radiogroup" aria-label={t("recovery.stepConfig")} className="flex flex-col gap-2">
              <Choice
                on={useBackup}
                title={t("emergency.settingsRestore")}
                sub={t("emergency.settingsRestoreSub")}
                onPick={() => setUseBackup(true)}
              />
              <Choice
                on={!useBackup}
                title={t("recovery.configSkip")}
                sub={t("emergency.settingsSkipPlaces")}
                onPick={() => setUseBackup(false)}
              />
            </div>
          )}
          {useBackup && settings && configPhase !== "restarting" && (
            <>
              <SettingRow label={t("recovery.configSourceLabel")}>
                <SourceToggle source={configSource} onChange={setConfigSource} disabled={configBusy} />
              </SettingRow>
              {configSource === "local" ? (
                <FolderBrowser
                  label={t("recovery.configLocalPath")}
                  value={settings.configPath}
                  hostMountRoot={hostMountRoot}
                  onChange={(v) => setSettings((prev) => (prev ? { ...prev, configPath: v } : prev))}
                />
              ) : (
                <label className="flex flex-col gap-1 text-xs text-carbon-textSub">
                  {t("recovery.configOffsiteUrl")}
                  <input
                    value={settings.configOffsite}
                    spellCheck={false}
                    onChange={(e) => setSettings((prev) => (prev ? { ...prev, configOffsite: e.target.value } : prev))}
                    placeholder="rest:http://host:8000/repo"
                    dir="ltr"
                    className={EMERGENCY_FIELD.replace("bg-carbon-surface3", "bg-carbon-surface2")}
                  />
                </label>
              )}
              <p className="text-xs text-carbon-textMuted">
                {taken && <>{t("emergency.loginStays")} </>}
                {t("emergency.settingsRestartHint")}
              </p>
            </>
          )}
          {configPhase === "manual" && <Note tone="warn">{t("recovery.configManualRestart")}</Note>}
          {configPhase === "reload" && <p className="text-xs text-statusWarn">{t("recovery.configReloadWhenBack")}</p>}
          {configPhase === "error" && configError && <Note tone="fail">{configError}</Note>}
        </>
      )}

      {step === "selection" && !leaving && (
        <>
          {lost && <Note tone="warn">{lost}</Note>}
          {searching && <p className="text-sm text-carbon-textMuted">{t("emergency.searching")}</p>}
          {found && total > 0 && (
            <Note tone="ok">
              {t("recovery.foundCounts").replace("{c}", String(found.containers)).replace("{v}", String(found.vms))}
              {found.files > 0 && <> {t("recovery.filesFound").replace("{f}", String(found.files))}</>}
              {found.zfs > 0 && <> {t("recovery.zfsFound", found.zfs)}</>}
            </Note>
          )}
          {found && !searching && offered === 0 && <Note tone="warn">{t("recovery.foundNone")}</Note>}
          {found && found.skipped.length > 0 && (
            <Note tone="warn">{t("common.discoverSkipped").replace("{list}", found.skipped.join(", "))}</Note>
          )}
          {found && <DiscoverFindings paused={found.paused} leftOpen={found.leftOpen} directRepos={found.directRepos} />}
          {searchError && <Note tone="fail">{searchError}</Note>}

          {!searching && restorable.length + backedUpVMs.length + fileSets.length > 0 && (
            <p className="text-xs text-carbon-textMuted">
              {t("emergency.selectionHint")} {t("emergency.selectionKept")}
            </p>
          )}
          {restorable.length > 0 && (
            <Section title={t("nav.containers")}>
              <Rows>
                {restorable.map((c) => (
                  <PickRow
                    key={c.name}
                    kind="container"
                    name={c.name}
                    on={on.has(`container:${c.name}`)}
                    onToggle={(v) => toggle(`container:${c.name}`, v)}
                  />
                ))}
              </Rows>
            </Section>
          )}
          {backedUpVMs.length > 0 && (
            <Section title={t("nav.vms")}>
              <Rows>
                {backedUpVMs.map((v) => (
                  <PickRow
                    key={v.libvirtName}
                    kind="vm"
                    name={v.name}
                    on={on.has(`vm:${v.libvirtName}`)}
                    onToggle={(next) => toggle(`vm:${v.libvirtName}`, next)}
                  />
                ))}
              </Rows>
              {vmSsh === false && <Note tone="warn">{t("recovery.vmSshNote")}</Note>}
            </Section>
          )}
          {fileSets.length > 0 && (
            <Section title={t("nav.files")} hint={t("recovery.filesRestoreHint")}>
              <Rows>
                {fileSets.map((s) => (
                  <PickRow key={s.id} kind="files" name={s.name} on={on.has(`files:${s.id}`)} onToggle={(v) => toggle(`files:${s.id}`, v)}>
                    {on.has(`files:${s.id}`) && (
                      <FolderBrowser
                        label={t("restore.targetPath")}
                        value={folderOf(s)}
                        hostMountRoot={hostMountRoot}
                        onChange={(v) => setFolders((prev) => ({ ...prev, [s.id]: v }))}
                      />
                    )}
                  </PickRow>
                ))}
              </Rows>
            </Section>
          )}
          {zfsItems.length > 0 && (
            <Note tone="neutral">
              {t("recovery.zfsFound", zfsItems.length)} {t("recovery.zfsRestoreHint")}
            </Note>
          )}
          {dumpOnly.length > 0 && (
            <Section title={t("recovery.dumpOnlyTitle")} hint={t("recovery.dumpOnlyHint")}>
              <Rows>
                {dumpOnly.map((c) => (
                  <DumpOnlyRow key={c.name} name={c.name} t={t} otherActive={running.active} />
                ))}
              </Rows>
            </Section>
          )}
          {restorable.length + backedUpVMs.length > 0 && (
            <ToggleRow
              label={t("restoreWindow.leaveStopped")}
              hint={t("emergency.leaveStoppedHint")}
              checked={keepStopped}
              onChange={setKeepStopped}
            />
          )}
          {!searching && found && (
            <section className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3 text-sm text-carbon-text">
              <h4 className="font-semibold">{t("restoreWindow.happens")}</h4>
              <p className="text-carbon-textSub">
                {words.length === 0
                  ? t("emergency.nothing")
                  : `${t("emergency.brings").replace("{list}", () => new Intl.ListFormat(lang, { type: "conjunction" }).format(words))} ${t(
                      keepStopped ? "emergency.staysStopped" : "emergency.startsAgain"
                    )}`}
              </p>
            </section>
          )}
        </>
      )}

      {step === "restore" && !leaving && (
        <>
          <ProgressBar
            percent={queue.length > 0 ? (doneCount / queue.length) * 100 : 100}
            active={!finished}
            inline
            label={t(finished ? "common.done" : "common.restoring")}
          />
          <ul>
            {queue.map((p) => {
              const outcome = outcomes[keyOf(p)];
              return (
                <li
                  key={keyOf(p)}
                  data-testid="emergency-run"
                  className="flex min-h-11 items-center gap-3 border-t border-carbon-border py-2 first:border-t-0"
                >
                  <span className="flex h-9 w-9 flex-none items-center justify-center rounded-control bg-carbon-surface2 text-accentText [&_svg]:h-[18px] [&_svg]:w-[18px]">
                    <KindGlyph kind={p.kind} />
                  </span>
                  <span className="min-w-0 flex-1 text-sm text-carbon-text wrap-anywhere">{p.name}</span>
                  {outcome === "done" ? (
                    <span className="text-sm text-statusOk">{t("common.done")}</span>
                  ) : outcome === "failed" ? (
                    <Badge tone="fail">{t("run.statusFailed")}</Badge>
                  ) : outcome === "running" ? (
                    <Badge tone="active">{t("run.statusRunning")}</Badge>
                  ) : (
                    <span className="text-sm text-carbon-textMuted">{t("zfs.replica.state.waiting")}</span>
                  )}
                </li>
              );
            })}
          </ul>
          {finished && (
            <>
              <Note tone={failedCount > 0 ? "warn" : "ok"}>
                {t("emergency.result")
                  .replace("{ok}", String(queue.length - failedCount))
                  .replace("{fail}", String(failedCount))}
              </Note>
              {runtimeRefused.length > 0 && (
                <RuntimeRetry key={runtimeRefused.join(",")} names={runtimeRefused} leaveStopped={keepStopped} t={t} />
              )}
              <Section title={t("recovery.step5")} hint={t("emergency.kitHint")}>
                <div className="flex flex-wrap items-center gap-3">
                  <Button
                    label={t("recovery.kitDownload")}
                    labelKey="recovery.kitDownload"
                    glyph={<IconDownload />}
                    onClick={downloadKit}
                  />
                  {kitError && <span className="text-xs text-statusFail wrap-break-word">{kitError}</span>}
                </div>
              </Section>
            </>
          )}
        </>
      )}
      {confirmDialog}
      {ownLogin.dialog}
    </InstanceWindow>
  );
}
