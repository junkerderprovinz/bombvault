import { useState } from "react";
import {
  ApiError,
  backupEverythingNow,
  patchFileSet,
  patchZFSDataset,
  setScheduleCadence,
  setVMScheduleCadence,
} from "../../../lib/api";
import { InfoBubble } from "../../../components/InfoBubble";
import { IdleCard } from "../IdleCard";
// Every cadence picker in settings edits a schedule that can count an
// interval (#166): the six domains and Backup Everything from their last
// successful backup, drills, tamper test and digest from schedule_job_runs.
// The off-site cadences are raw text inputs; rejectEveryNSchedules refuses
// everyN for them on the server.
import { CadenceBuilder } from "../../../components/CadenceBuilder";
import { EffectiveScheduleLine } from "../../../components/EffectiveScheduleLine";
import { ItemScheduleOverride } from "../../../components/ItemScheduleOverride";
import { Toggle } from "../../../components/Toggle";
import { Badge } from "../../../components/Badge";
import { Button } from "../../../components/Button";
import { ScheduleRow, scheduleStatus } from "../../../components/ScheduleBadge";
import type { Settings, Container, VM, FileSetView, ZFSDatasetView } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { tLtr } from "../../../lib/ltrFragments";
import { IconBackupNow } from "../../../components/Sidebar";
import { Card, ToggleRow, hueCounter } from "../shared";
import { useSettings } from "../settingsStore";

function ContainersSection({
  settings,
  containers,
  onChange,
  perItem,
  t,
  hueIndex,
}: {
  settings: Settings;
  containers: Container[];
  onChange: (schedule: string) => void;
  /** #121: when on, each included container exposes a per-item schedule override. */
  perItem: boolean;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const schedule = settings.containersSchedule;
  // Exclude BombVault's own container: it can never be backed up, so it must
  // never appear as a schedule member even if a stale flag lingers on its row.
  const included = containers.filter((c) => c.installed && c.includeInSchedule && !c.self);

  return (
    <Card title={t("jobs.containersSection")} hint={t("containers.scheduleHint")} hueIndex={hueIndex}>
      <ScheduleRow schedule={schedule} />

      {/* The card's own hueIndex, so the time picker takes the card's colour
          in rainbow mode. */}
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.containersSection")}
          value={schedule}
          onChange={onChange}
          hueIndex={hueIndex}
        />
      </div>

      {included.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("jobs.noContainersIncluded")}</p>
      ) : (
        <div className="flex flex-col gap-1 divide-y divide-carbon-border">
          {included.map((c) => (
            <div key={c.name} className="flex flex-col gap-2 py-2 text-sm">
              <div className="flex items-center gap-3">
                <div
                  className={`w-2 h-2 rounded-full shrink-0 ${
                    c.state.toLowerCase() === "running"
                      ? "bg-statusOkSolid"
                      : "bg-carbon-surface3"
                  }`}
                />
                <span className="font-medium text-carbon-text flex-1 truncate">
                  {c.name}
                </span>
                {c.image && (
                  <span className="text-xs text-carbon-textMuted truncate hidden sm:block max-w-xs">
                    {c.image}
                  </span>
                )}
              </div>
              {perItem && (
                <ItemScheduleOverride
                  name={c.name}
                  initial={c.scheduleCadence ?? ""}
                  onSave={(cadence) => setScheduleCadence(c.name, cadence)}
                />
              )}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

function VMsSection({
  settings,
  syncSchedules,
  onChange,
  vms,
  perItem,
  t,
  hueIndex,
}: {
  settings: Settings;
  syncSchedules: boolean;
  onChange: (schedule: string) => void;
  /** Included VMs, for the per-item override list (#121). */
  vms: VM[];
  /** #121: when on, each included VM exposes a per-item schedule override. */
  perItem: boolean;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const schedule = syncSchedules ? settings.containersSchedule : settings.vmsSchedule;
  const included = vms.filter((v) => v.includeInSchedule);

  return (
    <Card title={t("jobs.vmsSection")} hint={t("jobs.vmIncludeHint")} hueIndex={hueIndex}>
      {/* The editor stays visible and dimmed while the Containers schedule
          owns this domain, because the cadence it shows still runs, so the row
          says who owns it. */}
      <ScheduleRow schedule={schedule} hint={syncSchedules ? t("jobs.syncSchedulesHint") : undefined} />
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.vmsSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
          hueIndex={hueIndex}
        />
      </div>

      {/* Per-item overrides (#121): an included-VM list with a per-VM cadence,
          shown only when the toggle is on so the section is otherwise unchanged. */}
      {perItem && (
        included.length === 0 ? (
          <p className="text-sm text-carbon-textMuted">{t("jobs.noVMsIncluded")}</p>
        ) : (
          <div className="flex flex-col gap-1 divide-y divide-carbon-border">
            {included.map((v) => (
              <div key={v.libvirtName} className="flex flex-col gap-2 py-2 text-sm">
                <div className="flex items-center gap-3">
                  <div
                    className={`w-2 h-2 rounded-full shrink-0 ${
                      v.state.toLowerCase() === "running" ? "bg-statusOkSolid" : "bg-carbon-surface3"
                    }`}
                  />
                  <span className="font-medium text-carbon-text flex-1 truncate">{v.name}</span>
                </div>
                <ItemScheduleOverride
                  name={v.name}
                  initial={v.scheduleCadence ?? ""}
                  // libvirtName, not name: PATCH /api/vms/{name} resolves the
                  // path segment against the raw name (see vmNameParam),
                  // never the TrueNAS display-only friendly name.
                  onSave={(cadence) => setVMScheduleCadence(v.libvirtName, cadence)}
                />
              </div>
            ))}
          </div>
        )
      )}
    </Card>
  );
}

function FlashSection({
  settings,
  syncSchedules,
  onChange,
  t,
  hueIndex,
}: {
  settings: Settings;
  syncSchedules: boolean;
  onChange: (schedule: string) => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const schedule = syncSchedules ? settings.containersSchedule : settings.flashSchedule;

  return (
    // jobs.flashScheduleHint says what a Flash backup covers: unlike
    // Containers, VMs, Folders and ZFS, Flash has no per-item list to show it.
    <Card title={t("jobs.flashSection")} hint={tLtr(t, "jobs.flashScheduleHint")} hueIndex={hueIndex}>
      <ScheduleRow schedule={schedule} hint={syncSchedules ? t("jobs.syncSchedulesHint") : undefined} />
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.flashSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
          hueIndex={hueIndex}
        />
      </div>
    </Card>
  );
}

// FilesSection mirrors VMsSection for the cadence and ContainersSection for the
// member list, except that each set's "include in schedule" toggle PATCHes the
// set directly (the same {enabled} flag the Files page edits).
function FilesSection({
  settings,
  syncSchedules,
  fileSets,
  perItem,
  onChange,
  onSetsChanged,
  t,
  hueIndex,
}: {
  settings: Settings;
  /** While synced, the section follows the Containers cadence and its own
   *  editor is disabled, as for VMs and Flash. */
  syncSchedules: boolean;
  fileSets: FileSetView[];
  /** Whether per-item schedules are switched on (#199): the row's cadence
   *  editor is hidden while off, as the container rows do it, so nobody can
   *  set a cadence that the scheduler would then ignore. */
  perItem: boolean;
  onChange: (schedule: string) => void;
  /** A toggle PATCHed a set; reload the list so the rows reflect the server. */
  onSetsChanged: () => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const { push } = useToast();
  const schedule = syncSchedules ? settings.containersSchedule : settings.filesSchedule;
  const [busy, setBusy] = useState<Record<string, boolean>>({});

  // Mirrors ContainersSection's setScheduleCadence: the PATCH carries only the
  // cadence, so an edit here cannot disturb the set's name, path or excludes,
  // and the list is reloaded because the server may have reloaded the scheduler.
  async function setFileSetCadence(id: string, cadence: string) {
    const res = await patchFileSet(id, { scheduleCadence: cadence });
    if (res.ok) onSetsChanged();
    return res;
  }

  async function toggle(set: FileSetView) {
    setBusy((b) => ({ ...b, [set.id]: true }));
    try {
      const res = await patchFileSet(set.id, { enabled: !set.enabled });
      if (res.ok) onSetsChanged();
      else push(res.error ?? t("settings.error"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setBusy((b) => ({ ...b, [set.id]: false }));
    }
  }

  return (
    <Card title={t("jobs.filesSection")} hint={t("jobs.filesIncludeHint")} hueIndex={hueIndex}>
      <ScheduleRow schedule={schedule} hint={syncSchedules ? t("jobs.syncSchedulesHint") : undefined} />
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.filesSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
          hueIndex={hueIndex}
        />
      </div>

      {fileSets.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("jobs.noFileSetsIncluded")}</p>
      ) : (
        <div className="flex flex-col gap-1 divide-y divide-carbon-border">
          {fileSets.map((s) => (
            // Same layout as ContainersSection: the identifying row stays one
            // flex line and the override (#199) sits under it while per-item
            // schedules are on.
            <div key={s.id} className="flex flex-col gap-2 py-2 text-sm">
            <div className="flex items-center gap-3">
              <div
                className={`w-2 h-2 rounded-full shrink-0 ${
                  s.enabled ? "bg-statusOkSolid" : "bg-carbon-surface3"
                }`}
              />
              <span className="font-medium text-carbon-text flex-1 min-w-0 truncate">{s.name}</span>
              {s.path && (
                <span dir="ltr" className="text-xs font-mono text-carbon-textMuted truncate hidden sm:block max-w-xs text-start">
                  {s.path}
                </span>
              )}
              {/* The set name identifies the row, not what the switch does,
                  so the switch gets a visible caption beside it. */}
              <label className="flex items-center gap-2 shrink-0 cursor-pointer">
                <span className="text-xs text-carbon-textSub">{t("files.enabled")}</span>
                <Toggle
                  hideLabel
                  label={`${t("files.enabled")}: ${s.name}`}
                  checked={s.enabled}
                  onChange={() => void toggle(s)}
                  disabled={!!busy[s.id]}
                />
              </label>
            </div>
            {/* Says what actually happens to this set (#199). Rendered even
                with per-item schedules off, because "Include in schedule" on
                its own can leave a set not backed up automatically while
                users believe Backup Everything still covers it. */}
            <EffectiveScheduleLine effective={s.effectiveSchedule} />
            {perItem && (
              <ItemScheduleOverride
                name={s.name}
                initial={s.scheduleCadence ?? ""}
                onSave={(cadence) => setFileSetCadence(s.id, cadence)}
              />
            )}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

// Domain section for ZFS, in the same shape as FilesSection: the cadence card
// plus one row per item whose "include in schedule" toggle PATCHes the item
// directly.
function ZFSSection({
  settings,
  syncSchedules,
  items,
  perItem,
  onChange,
  onItemsChanged,
  t,
  hueIndex,
}: {
  settings: Settings;
  syncSchedules: boolean;
  items: ZFSDatasetView[];
  perItem: boolean;
  onChange: (schedule: string) => void;
  onItemsChanged: () => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const { push } = useToast();
  const schedule = syncSchedules ? settings.containersSchedule : settings.zfsSchedule;
  const [busy, setBusy] = useState<Record<string, boolean>>({});

  async function setItemCadence(id: string, cadence: string) {
    const res = await patchZFSDataset(id, { scheduleCadence: cadence });
    if (res.ok) onItemsChanged();
    return res;
  }

  async function toggle(item: ZFSDatasetView) {
    setBusy((b) => ({ ...b, [item.id]: true }));
    try {
      const res = await patchZFSDataset(item.id, { enabled: !item.enabled });
      if (res.ok) onItemsChanged();
      else push(res.error ?? t("settings.error"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setBusy((b) => ({ ...b, [item.id]: false }));
    }
  }

  return (
    <Card title={t("jobs.zfsSection")} hint={t("jobs.zfsIncludeHint")} hueIndex={hueIndex}>
      <ScheduleRow schedule={schedule} hint={syncSchedules ? t("jobs.syncSchedulesHint") : undefined} />
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.zfsSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
          hueIndex={hueIndex}
        />
      </div>

      {items.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("jobs.noZfsDatasetsIncluded")}</p>
      ) : (
        <div className="flex flex-col gap-1 divide-y divide-carbon-border">
          {items.map((d) => (
            <div key={d.id} className="flex flex-col gap-2 py-2 text-sm">
              <div className="flex items-center gap-3">
                <div
                  className={`w-2 h-2 rounded-full shrink-0 ${
                    d.enabled ? "bg-statusOkSolid" : "bg-carbon-surface3"
                  }`}
                />
                <span dir="ltr" className="font-medium text-carbon-text flex-1 min-w-0 truncate text-start">
                  {d.dataset}
                </span>
                {d.hostMountpoint && (
                  <span dir="ltr" className="text-xs font-mono text-carbon-textMuted truncate hidden sm:block max-w-xs text-start">
                    {d.hostMountpoint}
                  </span>
                )}
                <label className="flex items-center gap-2 shrink-0 cursor-pointer">
                  <span className="text-xs text-carbon-textSub">{t("files.enabled")}</span>
                  <Toggle
                    hideLabel
                    label={`${t("files.enabled")}: ${d.dataset}`}
                    checked={d.enabled}
                    onChange={() => void toggle(d)}
                    disabled={!!busy[d.id]}
                  />
                </label>
              </div>
              <EffectiveScheduleLine effective={d.effectiveSchedule} domainLabelKey="jobs.zfsSection" />
              {perItem && (
                <ItemScheduleOverride
                  name={d.dataset}
                  initial={d.scheduleCadence}
                  onSave={(cadence) => setItemCadence(d.id, cadence)}
                />
              )}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

export function EverythingSection({
  settings,
  update,
  t,
  hueIndex,
}: {
  settings: Settings;
  update: (patch: Partial<Settings>) => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  // A failed action toasts and shakes its button. `key={shake}` remounts the
  // button so the CSS animation restarts on a repeat failure.
  const [shake, setShake] = useState(0);

  // The overlap this card warns about is only real when this cadence is on
  // and at least one domain cadence is too, the self-backup included, since
  // the pass ends with it.
  const everythingOn = scheduleStatus(settings.everythingSchedule) !== "off";
  const anyDomainOn = [
    settings.containersSchedule,
    settings.vmsSchedule,
    settings.flashSchedule,
    settings.filesSchedule,
    settings.zfsSchedule,
    settings.configSchedule,
  ].some((s) => scheduleStatus(s) !== "off");
  const overlapWarning = everythingOn && anyDomainOn;

  async function runNow() {
    if (busy) return; // guard the in-flight window (the button also disables)
    setBusy(true);
    try {
      const res = await backupEverythingNow();
      if (res.ok) {
        push(t("settings.everythingStarted"), "success");
      } else {
        push(res.error ?? t("settings.error"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        push(t("settings.everythingAlreadyRunning"), "fail");
      } else {
        push(err instanceof Error ? err.message : t("settings.error"), "fail");
      }
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card title={t("settings.everythingTitle")} hint={t("settings.everythingHint")} hueIndex={hueIndex}>
      <ScheduleRow schedule={settings.everythingSchedule} />
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("settings.everythingTitle")}
          value={settings.everythingSchedule}
          onChange={(v) => update({ everythingSchedule: v })}
          hueIndex={hueIndex}
        />
        {/* Status amber on a plain surface, outside the accent and rainbow
            engine: it is a status colour, not control chrome. */}
        {overlapWarning && (
          <div className="mt-3 rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
            {t("settings.everythingDuplicateWarning")}
          </div>
        )}
      </div>
      <div className="flex flex-col gap-2">
        <span className="flex items-center gap-1 text-xs text-carbon-textSub">
          {t("hooks.title")}
          <InfoBubble tip={t("settings.everythingHooksHint")} />
        </span>
        {/* A stored hook is never echoed back, so the field arrives blank with
            ...Set true and shows the "already set" placeholder. A blank field
            means "keep" on save, so the Remove badge is the only way to delete
            a hook. */}
        <label className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("hooks.pre")}</span>
          <div className="flex items-center gap-2">
            <input
              value={settings.everythingPreHook}
              onChange={(e) => update({ everythingPreHook: e.target.value })}
              spellCheck={false}
              placeholder={settings.everythingPreHookSet ? t("cloud.secretSet") : "echo starting"}
              className="flex-1 min-w-0 rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-2 py-1 glim-field-focus"
            />
            {settings.everythingPreHookSet && (
              <Badge
                as="button"
                tone="active"
                size="small"
                hueIndex={hueIndex}
                onClick={() => update({ everythingPreHook: "", everythingPreHookClear: true })}
                className="pointer-coarse:h-(--btn-h) pointer-coarse:px-3"
              >
                {t("offsite.targets.remove")}
              </Badge>
            )}
          </div>
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("hooks.post")}</span>
          <div className="flex items-center gap-2">
            <input
              value={settings.everythingPostHook}
              onChange={(e) => update({ everythingPostHook: e.target.value })}
              spellCheck={false}
              placeholder={
                settings.everythingPostHookSet ? t("cloud.secretSet") : "curl -fsS https://hc-ping.com/your-uuid"
              }
              className="flex-1 min-w-0 rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-2 py-1 glim-field-focus"
            />
            {settings.everythingPostHookSet && (
              <Badge
                as="button"
                tone="active"
                size="small"
                hueIndex={hueIndex}
                onClick={() => update({ everythingPostHook: "", everythingPostHookClear: true })}
                className="pointer-coarse:h-(--btn-h) pointer-coarse:px-3"
              >
                {t("offsite.targets.remove")}
              </Badge>
            )}
          </div>
        </label>
      </div>
      <div className="flex justify-end">
        <Button
          key={shake}
          label={t("settings.everythingRunNow")}
          labelKey="settings.everythingRunNow"
          glyph={<IconBackupNow />}
          tone="accent"
          onClick={() => void runNow()}
          disabled={busy}
          busy={busy}
          title={busy ? t("settings.everythingBusy") : undefined}
          className={shake ? "glim-shake" : ""}
        />
      </div>
    </Card>
  );
}

export function SchedulesPage() {
  const { t } = useT();
  const {
    settings,
    containers,
    vms,
    fileSets,
    zfsItems,
    loadFileSets,
    loadZFSItems,
    fieldPulse,
    scheduleField,
    autoSaveScheduleField,
    scheduleUpdate,
    schedFieldBusy,
    schedFieldShake,
    syncSchedules,
    handleSyncSchedulesToggle,
    syncToggleBusy,
    syncToggleShake,
    toggleConfigSchedule,
    configScheduleToggleBusy,
    configScheduleToggleShake,
  } = useSettings();

  const nextHue = hueCounter();

  return (
    <>
      {/* Every cadence lives here, and every field saves as it changes. */}
      {/* A two-member list, so each toggle takes a local hueIndex (0, 1)
          independent of the card's own position. */}
      <Card title={t("settings.schedulesOptions")} hueIndex={nextHue()}>
        <ToggleRow
          label={t("settings.perItemSchedules")}
          hint={t("settings.perItemSchedulesHint")}
          checked={settings.perItemSchedules}
          onChange={(v) => void autoSaveScheduleField("perItemSchedules", v)}
          disabled={schedFieldBusy.perItemSchedules}
          shakeNonce={schedFieldShake.perItemSchedules}
          pulseNonce={fieldPulse.perItemSchedules}
          hueIndex={0}
        />
        <ToggleRow
          label={t("jobs.syncSchedules")}
          hint={t("jobs.syncSchedulesHint")}
          checked={syncSchedules}
          onChange={(v) => void handleSyncSchedulesToggle(v)}
          disabled={syncToggleBusy}
          shakeNonce={syncToggleShake || undefined}
          // The sync save writes all mirrored cadences together, so any of
          // them carries the pulse.
          pulseNonce={fieldPulse.vmsSchedule}
          hueIndex={1}
        />
      </Card>
      {/* Backup Everything, an independent pass over all domains plus a
          manual trigger. It sits second rather than last, so someone who
          wants one schedule for the whole server finds it before the parts
          it is made of. scheduleUpdate saves each key through the shared
          save(), since nothing on this page has a Save button. */}
      <EverythingSection settings={settings} update={scheduleUpdate} t={t} hueIndex={nextHue()} />
      <ContainersSection
        settings={settings}
        containers={containers}
        onChange={(v) => scheduleField("containersSchedule", v)}
        perItem={settings.perItemSchedules}
        t={t}
        hueIndex={nextHue()}
      />
      <VMsSection
        settings={settings}
        syncSchedules={syncSchedules}
        onChange={(v) => scheduleField("vmsSchedule", v)}
        vms={vms}
        perItem={settings.perItemSchedules}
        t={t}
        hueIndex={nextHue()}
      />
      <FlashSection
        settings={settings}
        syncSchedules={syncSchedules}
        onChange={(v) => scheduleField("flashSchedule", v)}
        t={t}
        hueIndex={nextHue()}
      />
      <FilesSection
        settings={settings}
        syncSchedules={syncSchedules}
        fileSets={fileSets}
        perItem={settings.perItemSchedules}
        onChange={(v) => scheduleField("filesSchedule", v)}
        onSetsChanged={loadFileSets}
        t={t}
        hueIndex={nextHue()}
      />
      <ZFSSection
        settings={settings}
        syncSchedules={syncSchedules}
        items={zfsItems}
        perItem={settings.perItemSchedules}
        onChange={(v) => scheduleField("zfsSchedule", v)}
        onItemsChanged={loadZFSItems}
        t={t}
        hueIndex={nextHue()}
      />

      {/* Off-site replication schedules, one cadence per domain. These
          editors are the only owner of these fields. */}
      <Card title={t("settings.schedulesOffsite")} hueIndex={nextHue()}>
        {([
          ["containersOffsiteSchedule", "nav.containers"],
          ["vmsOffsiteSchedule", "nav.vms"],
          ["flashOffsiteSchedule", "nav.flash"],
          ["configOffsiteSchedule", "nav.config"],
          ["filesOffsiteSchedule", "nav.files"],
          ["zfsOffsiteSchedule", "nav.zfs"],
        ] as const).map(([key, label]) => (
          <div key={key} className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t(label)}</span>
            <input
              value={settings[key]}
              spellCheck={false}
              onChange={(e) => scheduleField(key, e.target.value)}
              placeholder={t("offsite.schedulePlaceholder")}
              dir="ltr"
              className="rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono glim-field-focus text-start"
            />
          </div>
        ))}
      </Card>

      {/* BombVault's own config backup, in the same shape as the domain
          sections. One hueIdx feeds both the heading and the time picker,
          which a nextHue() call per prop could not. The toggle shows the
          card title as its visible label and has no hueIndex, being a lone
          switch with no list to walk. */}
      {(() => {
        const hueIdx = nextHue();
        const schedule = settings.configSchedule;
        const status = scheduleStatus(schedule);
        return (
          <Card title={t("settings.schedulesSelfBackup")} hint={t("config.scheduleHint")} hueIndex={hueIdx}>
            <ToggleRow
              label={t("settings.schedulesSelfBackup")}
              checked={status !== "off"}
              onChange={(v) => void toggleConfigSchedule(v)}
              disabled={configScheduleToggleBusy}
              shakeNonce={configScheduleToggleShake}
              pulseNonce={fieldPulse.configSchedule}
            />
            <ScheduleRow schedule={schedule} />
            <div className="rounded-card bg-carbon-surface2 p-4">
              <CadenceBuilder
                label={t("settings.schedulesSelfBackup")}
                value={schedule}
                onChange={(v) => scheduleField("configSchedule", v)}
                hueIndex={hueIdx}
              />
            </div>
          </Card>
        );
      })()}

      {/* Missed schedules: anacron-style catch-up after start. Backend runs
          the missed domain job ~2 minutes after boot (see internal/schedule
          CatchUpMissed). */}
      <Card title={t("settings.missedSchedulesTitle")} hueIndex={nextHue()}>
        <ToggleRow
          label={t("settings.catchUpMissed")}
          hint={t("settings.catchUpMissedHint")}
          checked={settings.catchUpMissed}
          onChange={(v) => void autoSaveScheduleField("catchUpMissed", v)}
          disabled={schedFieldBusy.catchUpMissed}
          shakeNonce={schedFieldShake.catchUpMissed}
          pulseNonce={fieldPulse.catchUpMissed}
        />
      </Card>

      <IdleCard t={t} hueIndex={nextHue()} />
    </>
  );
}
