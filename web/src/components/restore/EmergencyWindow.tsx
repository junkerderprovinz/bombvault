// The emergency window brings a lost or rebuilt server back in four steps:
// access to the backups, BombVault's own settings, the choice of what comes
// back, and the restore. Each step works on its own, because a rebuilt
// server often needs its settings before any backup is readable.
import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import {
  detectEncryption,
  discover,
  discoverAll,
  discoverFiles,
  discoverVMs,
  discoverZFS,
  downloadRecoveryKit,
  fileSetSnapshots,
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
  waitForAppBack,
  type Container,
  type EncryptionDetection,
  type FileSetView,
  type Settings,
  type VM,
  type ZFSDatasetView,
} from "../../lib/api";
import { fireAndWaitRun } from "../../lib/backupWatch";
import { useT, type TranslationKey } from "../../lib/i18n";
import { placementChanged } from "../../lib/placementEvents";
import { anyActive, busyPhraseKey, useProgress } from "../../lib/progress";
import { EMERGENCY_STEPS, emergencyPath, restoreFolderFor, type EmergencyStep } from "../../lib/restorePlan";
import { isRuntimeRefusal } from "../../lib/runReason";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { checkRestoreOnce, restoreBlockReason } from "../../lib/useRestoreCheck";
import type { Verdict } from "../../lib/useTestVerdict";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { CloudCard } from "../../pages/settings/CloudCard";
import { RcloneCard } from "../../pages/settings/RcloneCard";
import { ToggleRow } from "../../pages/settings/shared";
import { KindGlyph } from "../backups/kinds";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { FolderBrowser } from "../FolderBrowser";
import { IconBack, IconForward } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconCheck, IconDownload, IconRestore } from "../navGlyphs";
import { DiscoverFindings } from "../placement/DiscoverFindings";
import { ProgressBar } from "../ProgressBar";
import { SourceToggle, type RepoSource } from "../SourceToggle";
import { Rows } from "../storage/rows";
import { TestButton, VerdictLine } from "../TestButton";
import { Toggle } from "../Toggle";
import { DumpOnlyRow } from "./DumpOnlyRow";
import { EmergencyScene } from "./EmergencyScene";
import { EncryptionStatus } from "./EncryptionStatus";
import { RuntimeRetry } from "./RuntimeRetry";
import { StepBar } from "./StepBar";

const STEP_LABEL: Record<EmergencyStep, TranslationKey> = {
  access: "emergency.access",
  settings: "nav.settings",
  selection: "emergency.selection",
  restore: "snapshots.restore",
};

const PATHS = [
  ["containersPath", "settings.containersPath"],
  ["vmsPath", "settings.vmsPath"],
  ["flashPath", "settings.flashPath"],
  ["filesPath", "settings.filesPath"],
  ["zfsPath", "settings.zfsPath"],
] as const;

const OFFSITE = [
  ["containersOffsite", "nav.containers"],
  ["vmsOffsite", "nav.vms"],
  ["flashOffsite", "nav.flash"],
  ["filesOffsite", "nav.files"],
  ["zfsOffsite", "nav.zfs"],
] as const;

const FIELD = "rounded-control bg-carbon-surface3 px-3 py-2 text-start font-mono text-sm text-carbon-text glim-field-focus";

type Readable = "idle" | "ok" | "warn" | "bad";
type DiscoverResult = Awaited<ReturnType<typeof discover>>;
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

function Section({ title, hint, children }: { title: string; hint?: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3 rounded-card bg-carbon-surface2 p-4">
      <h3 className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-widest text-carbon-textSub">
        {title}
        {hint && <InfoBubble tip={hint} />}
      </h3>
      {children}
    </section>
  );
}

function Note({ tone, children }: { tone: "ok" | "warn" | "fail" | "neutral"; children: ReactNode }) {
  const look = {
    ok: "bg-statusOkBg text-statusOk",
    warn: "bg-statusWarnBg text-statusWarn",
    fail: "bg-statusFailBgSoft text-statusFail",
    neutral: "bg-carbon-surface2 text-carbon-text",
  }[tone];
  return <div className={`rounded-card px-3 py-2.5 text-sm leading-relaxed wrap-break-word ${look}`}>{children}</div>;
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
      <span
        className={`flex h-[18px] w-[18px] flex-none items-center justify-center rounded-pill border-2 ${
          on ? "border-accent" : "border-carbon-textMuted"
        }`}
      >
        {on && <span className="h-2 w-2 rounded-pill bg-accent" />}
      </span>
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

export function EmergencyWindow({ start, onClose }: { start: EmergencyStep; onClose: () => void }) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [step, setStep] = useState<EmergencyStep>(start);

  const [settings, setSettings] = useState<Settings | null>(null);
  const [hostMountRoot, setHostMountRoot] = useState("/host/user");

  const [readable, setReadable] = useState<Readable>("idle");
  // The folders the check read, so an empty answer says where it looked.
  const [readSources, setReadSources] = useState<string[]>([]);
  const [readError, setReadError] = useState<string | null>(null);
  // Repositories the check left out because they are switched off. That is
  // said without turning the verdict into a warning.
  const [readNote, setReadNote] = useState<string | null>(null);
  const [checking, setChecking] = useState(false);
  const [readShake, setReadShake] = useState(0);
  const [attaching, setAttaching] = useState(false);

  const [encDetection, setEncDetection] = useState<EncryptionDetection | null>(null);
  const [encDetecting, setEncDetecting] = useState(true);

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

  // The detection writes what it found into the settings held here.
  // Otherwise the next save would put the stale switch back over it.
  const detect = useCallback(async () => {
    setEncDetecting(true);
    try {
      const res = await detectEncryption();
      setEncDetection(res);
      if (res.ok && typeof res.encryptionEnabled === "boolean") {
        const detected = res.encryptionEnabled;
        setSettings((prev) => (prev ? { ...prev, encryptionEnabled: detected } : prev));
      }
    } catch (err) {
      setEncDetection({ ok: false, error: err instanceof Error ? err.message : String(err) });
    } finally {
      setEncDetecting(false);
    }
  }, []);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) {
          setSettings(res.settings);
          if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
        }
      })
      .catch(() => undefined)
      // After the settings, so a late answer cannot undo what the detection wrote.
      .finally(() => void detect());
    // VM restore needs the libvirt SSH link. A missing one is a note, never a lock.
    getVMSSH()
      .then((r) => setVmSsh(r.ok && !!r.host))
      .catch(() => setVmSsh(false));
  }, [detect]);

  // The check is the read-only probe, so it never rebuilds the list of
  // entries. It opens the encrypted repositories, which is how a wrong
  // APP_KEY shows.
  const checkReadable = useCallback(async (): Promise<Readable> => {
    setChecking(true);
    setReadError(null);
    try {
      const results: DiscoverResult[] = await Promise.all([discover(true), discoverVMs(true), discoverFiles(true), discoverZFS(true)]);
      const keyErr = results.find((r) => !r.ok && isKeyMismatch(r.error));
      if (keyErr) {
        setReadable("bad");
        setReadError(keyErr.error ?? null);
        return "bad";
      }
      const otherErr = results.find((r) => !r.ok);
      if (otherErr) {
        setReadable("warn");
        setReadError(otherErr.error ?? null);
        return "warn";
      }
      setReadSources(results.map((r) => r.repo).filter((r): r is string => !!r));
      // A repository that could not be opened keeps the verdict off green,
      // one switched off on purpose is only named.
      const skipped = [...new Set(results.flatMap((r) => r.skipped ?? []))];
      const needsAction = results.some((r) => r.skippedNeedsAction === true);
      const skippedLine = skipped.length > 0 ? t("common.discoverSkipped").replace("{list}", skipped.join(", ")) : null;
      setReadNote(needsAction ? null : skippedLine);
      if (skippedLine && needsAction) {
        setReadError(skippedLine);
        setReadable("warn");
        return "warn";
      }
      // Nothing found means reachable but empty, or not attached yet.
      const next: Readable = results.reduce((n, r) => n + (r.discovered ?? 0), 0) > 0 ? "ok" : "warn";
      setReadable(next);
      return next;
    } catch (err) {
      setReadable("warn");
      setReadError(err instanceof Error ? err.message : String(err));
      return "warn";
    } finally {
      setChecking(false);
    }
  }, [t]);

  async function recheck() {
    if ((await checkReadable()) !== "ok") setReadShake((n) => n + 1);
  }

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

  async function attach() {
    if (!settings) return;
    setAttaching(true);
    try {
      const patch: Partial<Settings> = { encryptionEnabled: settings.encryptionEnabled };
      for (const [key] of [...PATHS, ...OFFSITE]) patch[key] = settings[key];
      const refused = await saveOnto(patch);
      if (refused !== null) {
        push(refused, "fail");
        return;
      }
      window.dispatchEvent(new Event("bv:settings-changed"));
      // Another repository holds other entries, so the selection searches again.
      setFound(null);
      searched.current = false;
      // The detection probes the saved locations, and the check after it
      // says whether attaching them worked.
      await detect();
      if ((await checkReadable()) === "ok") push(t("recovery.readable"), "success");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setAttaching(false);
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
        return fail(failed?.id === "key" ? t("recovery.appKeyRemedy") : [blocked, detail].filter(Boolean).join(" "));
      }
      const res = await restoreConfig("latest", offsite ? "offsite" : undefined);
      if (!res.ok) return fail(isKeyMismatch(res.error) ? t("recovery.appKeyRemedy") : (res.error ?? t("settings.error")));
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
      if (counts.error) setSearchError(isKeyMismatch(counts.error) ? t("recovery.appKeyRemedy") : counts.error);
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

  const readVerdict: Verdict | null =
    checking || readable === "idle" ? null : { ok: readable === "ok", reason: readable === "warn" ? t("recovery.notReachable") : undefined };
  const configBusy = configPhase === "saving" || configPhase === "restarting";
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

  const cancel = <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />;
  const back = (to: EmergencyStep) => (
    <Button label={t("common.back")} labelKey="common.back" glyph={<IconBack />} tone="subtle" onClick={() => setStep(to)} />
  );
  const next = (to: EmergencyStep) => (
    <Button
      label={t("zfs.replica.add.next")}
      labelKey="zfs.replica.add.next"
      glyph={<IconForward />}
      tone="accent"
      onClick={() => setStep(to)}
    />
  );

  let footer: ReactNode;
  if (step === "access") {
    footer = (
      <>
        {cancel}
        {next("settings")}
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
            tone="accent"
            onClick={() => void restoreOwnSettings()}
            disabled={!settings || configBusy}
            busy={configBusy}
          />
        ) : (
          next("selection")
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
        onClick={onClose}
      />
    );
  }

  return (
    <InstanceWindow title={t("emergency.title")} onClose={onClose} footer={footer}>
      <StepBar
        steps={EMERGENCY_STEPS.map((s) => t(STEP_LABEL[s]))}
        current={at}
        reach={restoring ? 0 : queue.length > 0 ? 4 : 3}
        label={t("emergency.title")}
        onGo={(n) => setStep(EMERGENCY_STEPS[n - 1])}
      />

      {step === "access" && (
        <>
          <EmergencyScene />
          <Section title={t("recovery.step1")} hint={t("recovery.appKeyExplain")}>
            <VerdictLine verdict={readVerdict} />
            {readable === "ok" && <Note tone="ok">{t("recovery.readable")}</Note>}
            {readSources.length > 0 && readable !== "idle" && (
              <p className="text-xs leading-relaxed text-carbon-textMuted wrap-break-word">
                {t("recovery.readFrom")} <span className="font-mono">{readSources.join(", ")}</span>
              </p>
            )}
            {readable === "bad" && <Note tone="fail">{t("recovery.appKeyRemedy")}</Note>}
            {readable === "warn" && readError && (
              <p dir="ltr" className="break-all font-mono text-start text-xs text-carbon-textMuted">
                {readError}
              </p>
            )}
            {readNote && (
              <p dir="ltr" className="text-start text-xs text-carbon-textMuted wrap-break-word">
                {readNote}
              </p>
            )}
            <div>
              <TestButton
                label={t("recovery.recheck")}
                labelKey="recovery.recheck"
                words="check"
                test={{ verdict: readVerdict, running: checking, shake: readShake, shaking: readShake > 0 && readable !== "ok" }}
                onClick={() => void recheck()}
              />
            </div>
          </Section>

          <Section title={t("recovery.step2")} hint={t("emergency.attachHint")}>
            {settings ? (
              <>
                <EncryptionStatus
                  t={t}
                  detection={encDetection}
                  detecting={encDetecting}
                  encryptionEnabled={settings.encryptionEnabled}
                  onOverride={(v) => setSettings((prev) => (prev ? { ...prev, encryptionEnabled: v } : prev))}
                />
                {PATHS.map(([key, label], i) => (
                  <FolderBrowser
                    key={key}
                    label={t(label)}
                    value={settings[key]}
                    hostMountRoot={hostMountRoot}
                    hueIndex={i}
                    onChange={(v) => setSettings((prev) => (prev ? { ...prev, [key]: v } : prev))}
                  />
                ))}
                <h4 className="pt-1 text-xs font-semibold text-carbon-textSub">{t("emergency.offsite")}</h4>
                {OFFSITE.map(([key, label]) => (
                  <label key={key} className="flex flex-col gap-1 text-xs text-carbon-textSub">
                    {t(label)}
                    <input
                      value={settings[key]}
                      spellCheck={false}
                      onChange={(e) => setSettings((prev) => (prev ? { ...prev, [key]: e.target.value } : prev))}
                      placeholder="rest:http://host:8000/repo"
                      dir="ltr"
                      className={FIELD}
                    />
                  </label>
                ))}
                <h4 className="flex items-center gap-1.5 pt-1 text-xs font-semibold text-carbon-textSub">
                  {t("recovery.cloudCreds")}
                  <InfoBubble tip={`${t("recovery.cloudCredsHint")} ${t("recovery.credsSaveHint")}`} />
                </h4>
                <CloudCard t={t} hueIndex={5} nested />
                <RcloneCard t={t} hueIndex={6} nested />
                <div className="flex justify-end">
                  <Button
                    label={t("emergency.connect")}
                    labelKey={null}
                    glyph={<IconCheck />}
                    onClick={() => void attach()}
                    disabled={attaching}
                    busy={attaching}
                  />
                </div>
              </>
            ) : (
              <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
            )}
          </Section>
        </>
      )}

      {step === "settings" && (
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
                sub={t("emergency.settingsSkipSub")}
                onPick={() => setUseBackup(false)}
              />
            </div>
          )}
          {useBackup && settings && configPhase !== "restarting" && (
            <>
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-sm text-carbon-text">{t("recovery.configSourceLabel")}</span>
                <SourceToggle source={configSource} onChange={setConfigSource} disabled={configBusy} />
              </div>
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
                    className={FIELD.replace("bg-carbon-surface3", "bg-carbon-surface2")}
                  />
                </label>
              )}
              <p className="text-xs text-carbon-textMuted">{t("emergency.settingsRestartHint")}</p>
            </>
          )}
          {configPhase === "manual" && <Note tone="warn">{t("recovery.configManualRestart")}</Note>}
          {(configPhase === "reload" || configPhase === "restarting") && (
            <div className="flex flex-wrap items-center gap-3">
              {configPhase === "reload" && <span className="text-xs text-statusWarn">{t("recovery.configReloadWhenBack")}</span>}
              <Button label={t("recovery.configReload")} labelKey="recovery.configReload" onClick={() => window.location.reload()} />
            </div>
          )}
          {configPhase === "error" && configError && <Note tone="fail">{configError}</Note>}
        </>
      )}

      {step === "selection" && (
        <>
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

      {step === "restore" && (
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
    </InstanceWindow>
  );
}
