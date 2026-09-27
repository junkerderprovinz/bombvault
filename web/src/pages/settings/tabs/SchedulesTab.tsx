import { useState } from "react";
import {
  ApiError,
  backupEverythingNow,
  patchFileSet,
  patchZFSDataset,
  setScheduleCadence,
  setVMScheduleCadence,
} from "../../../lib/api";
import { NumberField } from "../../../components/NumberField";
import { InfoBubble } from "../../../components/InfoBubble";
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
import { Card, ToggleRow } from "../shared";
import type { SettingsTabProps } from "./types";

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
  /** When on, each included container gets a per-item schedule override. */
  perItem: boolean;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const schedule = settings.containersSchedule;
  // BombVault's own container can never be backed up, so it stays out even
  // if a stale flag lingers on its row.
  const included = containers.filter((c) => c.installed && c.includeInSchedule && !c.self);

  return (
    <Card title={t("jobs.containersSection")} hint={t("containers.scheduleHint")} hueIndex={hueIndex}>
      <ScheduleRow schedule={schedule} />

      {/* The card's own hue goes to the builder so its time picker matches
          the heading. */}
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
        <div className="flex flex-col gap-1">
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
  vms: VM[];
  /** When on, each included VM gets a per-item schedule override. */
  perItem: boolean;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const schedule = syncSchedules ? settings.containersSchedule : settings.vmsSchedule;
  const included = vms.filter((v) => v.includeInSchedule);

  return (
    <Card title={t("jobs.vmsSection")} hint={t("jobs.vmIncludeHint")} hueIndex={hueIndex}>
      {/* While the Containers schedule owns this domain the editor stays
          visible but dimmed, because the cadence it shows still runs, and
          the row says who owns it. */}
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

      {perItem && (
        included.length === 0 ? (
          <p className="text-sm text-carbon-textMuted">{t("jobs.noVMsIncluded")}</p>
        ) : (
          <div className="flex flex-col gap-1">
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
                  // PATCH /api/vms/{name} resolves the libvirt name, not the
                  // friendly name TrueNAS displays.
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
    // Flash has no member list, so the hint says what a flash backup covers.
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

// FilesSection's per-set toggles PATCH each file set directly, the same
// {enabled} flag the Files page edits.
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
  /** While on, folders follow the Containers cadence, like VMs and flash. */
  syncSchedules: boolean;
  fileSets: FileSetView[];
  /** The per-set cadence editor is hidden while off, so nobody can set a
   *  cadence the scheduler would ignore. */
  perItem: boolean;
  onChange: (schedule: string) => void;
  /** A toggle PATCHed a set, reload the list so the rows reflect the server. */
  onSetsChanged: () => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const { push } = useToast();
  const schedule = syncSchedules ? settings.containersSchedule : settings.filesSchedule;
  const [busy, setBusy] = useState<Record<string, boolean>>({});

  // The PATCH carries only the cadence, so it cannot disturb the set's name,
  // path or excludes; the list reloads because the server may have reloaded
  // the scheduler.
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
        <div className="flex flex-col gap-1">
          {fileSets.map((s) => (
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
              {/* The set's name says which row this is, not what the switch
                  does, so the switch carries a visible caption of its own. */}
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
            {/* Rendered even without per-item schedules, because "not backed
                up automatically" is reachable with that toggle off. */}
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
// directly rather than going through a save bar.
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
        <div className="flex flex-col gap-1">
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
  // A failed run toasts and shakes the button; `key={shake}` remounts it so
  // the animation restarts on a repeat failure.
  const [shake, setShake] = useState(0);

  // The overlap is only real when this cadence and at least one domain
  // cadence are on, configSchedule included, since the pass ends with the
  // self-backup.
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
    if (busy) return; // guard the in-flight window (badge also disables)
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
        {/* Status amber stays outside the accent and rainbow engine: it is a
            status colour, not control chrome. */}
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
        {/* A stored hook is never echoed back, so the field arrives blank
            with ...Set true. A blank field means "keep" on save, which makes
            the Remove badge the only way to delete one. */}
        <label className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("hooks.pre")}</span>
          <div className="flex items-center gap-2">
            <input
              value={settings.everythingPreHook}
              onChange={(e) => update({ everythingPreHook: e.target.value })}
              spellCheck={false}
              placeholder={settings.everythingPreHookSet ? t("cloud.secretSet") : "echo starting"}
              className="flex-1 rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-2 py-1 glim-field-focus"
            />
            {settings.everythingPreHookSet && (
              <Badge
                tone="active"
                size="small"
                hueIndex={hueIndex}
                onClick={() => update({ everythingPreHook: "", everythingPreHookClear: true })}
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
              className="flex-1 rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-2 py-1 glim-field-focus"
            />
            {settings.everythingPostHookSet && (
              <Badge
                tone="active"
                size="small"
                hueIndex={hueIndex}
                onClick={() => update({ everythingPostHook: "", everythingPostHookClear: true })}
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

export function SchedulesTab({
  t,
  settings,
  containers,
  vms,
  fileSets,
  syncSchedules,
  schedFieldBusy,
  schedFieldShake,
  syncToggleBusy,
  syncToggleShake,
  configScheduleToggleBusy,
  configScheduleToggleShake,
  loadFileSets,
  zfsItems,
  loadZFSItems,
  fieldPulse,
  scheduleField,
  autoSaveScheduleField,
  scheduleUpdate,
  handleSyncSchedulesToggle,
  toggleConfigSchedule,
}: SettingsTabProps) {
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    <>
      {/* Every field on this tab saves itself, so there is no Save bar. */}
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
        {/* Applies the Containers cadence to VMs, flash and folders. */}
        <ToggleRow
          label={t("jobs.syncSchedules")}
          hint={t("jobs.syncSchedulesHint")}
          checked={syncSchedules}
          onChange={(v) => void handleSyncSchedulesToggle(v)}
          disabled={syncToggleBusy}
          shakeNonce={syncToggleShake || undefined}
          // The sync save patches vms, flash and files schedules together,
          // so any one of the three pulses on success.
          pulseNonce={fieldPulse.vmsSchedule}
          hueIndex={1}
        />
      </Card>
      {/* Backup Everything comes before the per-domain cadences it is made
          of, so somebody looking for one schedule for the whole server finds
          it. scheduleUpdate debounces each key through save(), because this
          tab has no Save button to persist a local-only patch. */}
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

      {/* One hue slot feeds both the heading and the builder's time picker,
          which a second inline nextHue() call could not. The toggle has no
          hueIndex because a lone switch has no list to walk. */}
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

      {/* Anacron-style catch-up: the backend runs a missed domain job about
          two minutes after boot (internal/schedule CatchUpMissed). */}
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

      {/* Containers stopped for a backup restart in compose depends_on
          order, each healthy before its dependents start; the wait also
          covers the post-backup update recreate. */}
      <Card title={t("settings.restartHealthTitle")} hueIndex={nextHue()}>
        <ToggleRow
          label={t("settings.restartHealthWait")}
          hint={t("settings.restartHealthWaitHint")}
          checked={settings.restartHealthWait}
          onChange={(v) => void autoSaveScheduleField("restartHealthWait", v)}
          disabled={schedFieldBusy.restartHealthWait}
          shakeNonce={schedFieldShake.restartHealthWait}
          pulseNonce={fieldPulse.restartHealthWait}
        />
        {settings.restartHealthWait && (
          <label className="flex flex-col gap-1 sm:w-1/2">
            <span className="flex items-center gap-1 text-xs text-carbon-textSub">
              {t("settings.restartHealthTimeoutLabel")}
              <InfoBubble tip={t("settings.restartHealthTimeoutHint")} />
            </span>
            <NumberField
              min={5}
              max={3600}
              value={settings.restartHealthTimeoutSec}
              onChange={(e) => {
                const raw = (e.target as unknown as { value: string }).value;
                // Keeps a transient value below 5 out of state; the server
                // clamps to 5..3600 as well.
                const n = Math.max(5, parseInt(raw, 10) || 0);
                scheduleField("restartHealthTimeoutSec", n);
              }}
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
            />
          </label>
        )}
      </Card>
    </>
  );
}
