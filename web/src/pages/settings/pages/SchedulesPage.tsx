import { useCallback, useEffect, useState, type CSSProperties } from "react";
import { listItems, type BackupItem, type Settings } from "../../../lib/api";
import { type EntryKind } from "../../../lib/backupEntry";
import { hueVars } from "../../../lib/appearance";
import { useT } from "../../../lib/i18n";
import { formatList } from "../../../lib/placement";
import { KIND_LABEL, KindGlyph } from "../../../components/backups/kinds";
import { Button } from "../../../components/Button";
import { EXACT_CADENCE_MODES } from "../../../components/CadenceBuilder";
import { BackupOrderPanel } from "../../../components/containers/BackupOrderPanel";
import { Exceptions } from "../../../components/Exceptions";
import { IconSchedules, IconTrash } from "../../../components/navGlyphs";
import { NumberField } from "../../../components/NumberField";
import { PlanButton } from "../../../components/PlanButton";
import { scheduleStatus } from "../../../components/ScheduleBadge";
import { Rows, SettingRow } from "../../../components/storage/rows";
import { Toggle } from "../../../components/Toggle";
import { VMBackupOrderPanel } from "../../../components/vms/VMBackupOrderPanel";
import { IdleCard } from "../IdleCard";
import { ScheduleMembersWindow } from "../ScheduleMembersWindow";
import { Card, SubCard, ToggleRow, hueCounter } from "../shared";
import { useSettings } from "../settingsStore";
import { WeekCard } from "../WeekCard";

// autoSaveField reports through a toast, so its two callbacks have nothing to do here.
const ignoreSaveState = () => undefined;
const ignoreSaveError = () => undefined;

type ScheduleKey =
  | "containersSchedule"
  | "vmsSchedule"
  | "flashSchedule"
  | "filesSchedule"
  | "zfsSchedule"
  | "configSchedule";
type OffsiteKey =
  | "containersOffsiteSchedule"
  | "vmsOffsiteSchedule"
  | "flashOffsiteSchedule"
  | "filesOffsiteSchedule"
  | "zfsOffsiteSchedule"
  | "configOffsiteSchedule";

interface Section {
  kind: EntryKind;
  enabled: keyof Settings;
  schedule: ScheduleKey;
  offsite: OffsiteKey;
}

// The order the full backup runs the sections in.
const SECTIONS: Section[] = [
  { kind: "container", enabled: "containersEnabled", schedule: "containersSchedule", offsite: "containersOffsiteSchedule" },
  { kind: "vm", enabled: "vmsEnabled", schedule: "vmsSchedule", offsite: "vmsOffsiteSchedule" },
  { kind: "flash", enabled: "flashEnabled", schedule: "flashSchedule", offsite: "flashOffsiteSchedule" },
  { kind: "files", enabled: "filesEnabled", schedule: "filesSchedule", offsite: "filesOffsiteSchedule" },
  { kind: "zfs", enabled: "zfsEnabled", schedule: "zfsSchedule", offsite: "zfsOffsiteSchedule" },
  { kind: "config", enabled: "configEnabled", schedule: "configSchedule", offsite: "configOffsiteSchedule" },
];

// What a new exception starts from.
const NEW_SECTION_PLAN = "weekly Sun 04:00";
const NEW_COPY_PLAN = "daily 05:00";

// The kinds whose entries each have a switch for the schedule.
const MEMBER_KINDS: EntryKind[] = ["container", "vm", "files", "zfs"];

/** A schedule reaches an entry that exists on the host and is not BombVault itself. */
function schedulable(item: BackupItem): boolean {
  return !item.self && item.installed !== false;
}

function HookRow({
  label,
  hint,
  value,
  isSet,
  placeholder,
  onChange,
  onClear,
}: {
  label: string;
  hint: string;
  value: string;
  isSet: boolean;
  placeholder: string;
  onChange: (v: string) => void;
  onClear: () => void;
}) {
  const { t } = useT();
  return (
    <SettingRow label={label} hint={hint}>
      {/* A stored command is never sent back, so the field starts blank and
          says it holds one. Blank keeps it; the bin removes it. */}
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        spellCheck={false}
        aria-label={label}
        dir="ltr"
        placeholder={isSet ? t("cloud.secretSet") : placeholder}
        className="w-80 max-w-full rounded-control bg-carbon-surface3 px-3 py-1.5 text-start font-mono text-xs text-carbon-text glim-field-focus"
      />
      {isSet && (
        <Button label={t("offsite.targets.remove")} labelKey="offsite.targets.remove" glyph={<IconTrash />} variant="icon" glyphOnly onClick={onClear} />
      )}
    </SettingRow>
  );
}

export function SchedulesPage() {
  const { t, lang } = useT();
  const {
    settings,
    containers,
    vms,
    fieldPulse,
    scheduleField,
    scheduleUpdate,
    autoSaveScheduleField,
    schedFieldBusy,
    schedFieldShake,
    autoSaveField,
    mergedFieldBusy,
    mergedFieldShake,
  } = useSettings();
  const [items, setItems] = useState<BackupItem[]>([]);
  const [members, setMembers] = useState<EntryKind | null>(null);

  const loadItems = useCallback(() => {
    listItems()
      .then((res) => {
        if (res.ok) setItems(res.items ?? []);
      })
      .catch(() => {
        // The entry counts stay empty; the plans above do not depend on them.
      });
  }, []);
  useEffect(loadItems, [loadItems]);

  const nextHue = hueCounter();
  const sections = SECTIONS.filter((s) => settings[s.enabled]);
  const label = (kind: EntryKind) => t(KIND_LABEL[kind]);
  const asException = (s: Section) => ({ id: s.kind, label: label(s.kind), glyph: <KindGlyph kind={s.kind} /> });
  const section = (kind: string) => SECTIONS.find((s) => s.kind === kind)!;

  const fullOn = scheduleStatus(settings.everythingSchedule) !== "off";
  const ownPlans = sections.filter((s) => scheduleStatus(settings[s.schedule]) !== "off");
  const inFull = sections.filter((s) => !ownPlans.includes(s));
  const ownCopies = sections.filter((s) => settings[s.offsite].trim() !== "");
  const fullHue = nextHue();

  return (
    <>
      <WeekCard settings={settings} hueIndex={nextHue()} />

      <Card title={t("settings.everythingTitle")} hint={t("schedules.fullCardHint")} hueIndex={fullHue}>
        <div>
          <SettingRow
            label={t("schedules.whenRuns")}
            hint={t("schedules.whenRunsHint")}
            note={
              !fullOn
                ? t("schedules.fullOff")
                : inFull.length > 0
                  ? t("schedules.includes").replace("{sections}", formatList(lang, inFull.map((s) => label(s.kind))))
                  : t("schedules.fullAllOwn")
            }
          >
            <PlanButton
              label={t("settings.everythingTitle")}
              value={settings.everythingSchedule}
              onChange={(v) => scheduleField("everythingSchedule", v)}
              hueIndex={fullHue}
            />
          </SettingRow>
          <div className="border-t border-carbon-border/60 pt-3">
            <ToggleRow
              label={t("settings.catchUpMissed")}
              hint={t("settings.catchUpMissedHint")}
              checked={settings.catchUpMissed}
              onChange={(v) => void autoSaveScheduleField("catchUpMissed", v)}
              disabled={schedFieldBusy.catchUpMissed}
              shakeNonce={schedFieldShake.catchUpMissed}
              pulseNonce={fieldPulse.catchUpMissed}
            />
          </div>
        </div>
        <SubCard title={t("schedules.hooks")}>
          <Rows>
            <HookRow
              label={t("schedules.hookPre")}
              hint={t("schedules.hookPreHint")}
              value={settings.everythingPreHook}
              isSet={settings.everythingPreHookSet}
              placeholder="/boot/config/scripts/before.sh"
              onChange={(v) => scheduleUpdate({ everythingPreHook: v })}
              onClear={() => scheduleUpdate({ everythingPreHook: "", everythingPreHookClear: true })}
            />
            <HookRow
              label={t("schedules.hookPost")}
              hint={t("schedules.hookPostHint")}
              value={settings.everythingPostHook}
              isSet={settings.everythingPostHookSet}
              placeholder="/boot/config/scripts/after.sh"
              onChange={(v) => scheduleUpdate({ everythingPostHook: v })}
              onClear={() => scheduleUpdate({ everythingPostHook: "", everythingPostHookClear: true })}
            />
          </Rows>
        </SubCard>
      </Card>

      {sections.length > 0 &&
        (() => {
          const hue = nextHue();
          return (
            <Card title={t("schedules.plans")} hint={t("schedules.plansCardHint")} hueIndex={hue}>
              <Exceptions
                label={t("schedules.plans")}
                taken={ownPlans.map(asException)}
                free={inFull.map(asException)}
                rule={t(
                  !fullOn
                    ? "schedules.plansRuleFullOff"
                    : ownPlans.length > 0
                      ? "schedules.plansRuleOthers"
                      : "schedules.plansRuleAll"
                )}
                control={(kind) => (
                  <PlanButton
                    label={label(kind as EntryKind)}
                    value={settings[section(kind).schedule]}
                    onChange={(v) => scheduleField(section(kind).schedule, v)}
                    hueIndex={hue}
                  />
                )}
                onAdd={(kind) => scheduleField(section(kind).schedule, NEW_SECTION_PLAN)}
                onRemove={(kind) => scheduleField(section(kind).schedule, "off")}
              />
              <SubCard title={t("schedules.offsite")}>
                <Exceptions
                  label={t("schedules.offsite")}
                  taken={ownCopies.map(asException)}
                  free={sections.filter((s) => !ownCopies.includes(s)).map(asException)}
                  rule={t(ownCopies.length > 0 ? "schedules.offsiteRuleOthers" : "schedules.offsiteRuleAll")}
                  control={(kind) => (
                    <PlanButton
                      label={`${t("schedules.offsite")}: ${label(kind as EntryKind)}`}
                      value={settings[section(kind).offsite]}
                      onChange={(v) => scheduleField(section(kind).offsite, v)}
                      modes={EXACT_CADENCE_MODES}
                      hueIndex={hue}
                    />
                  )}
                  onAdd={(kind) => scheduleField(section(kind).offsite, NEW_COPY_PLAN)}
                  onRemove={(kind) => scheduleField(section(kind).offsite, "")}
                />
              </SubCard>
            </Card>
          );
        })()}

      <Card title={t("schedules.entries")} hint={t("schedules.entriesHint")} hueIndex={nextHue()}>
        <Rows>
          <SettingRow label={t("schedules.perItem")} hint={t("schedules.perItemHint")}>
            <Toggle
              hideLabel
              label={t("schedules.perItem")}
              checked={settings.perItemSchedules}
              onChange={(v) => void autoSaveScheduleField("perItemSchedules", v)}
              disabled={schedFieldBusy.perItemSchedules}
              className={schedFieldShake.perItemSchedules ? "glim-shake" : ""}
            />
          </SettingRow>
          {sections
            .filter((s) => MEMBER_KINDS.includes(s.kind))
            .map((s, i) => {
              const list = items.filter((item) => item.kind === s.kind && schedulable(item));
              return (
                <div key={s.kind} data-kind={s.kind} className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 py-3 last:pb-0">
                  <span className="flex min-w-0 flex-col gap-0.5">
                    <span className="flex items-center gap-2 text-sm text-carbon-text">
                      <span className="glim-hue inline-flex text-accentText" style={hueVars(i) as CSSProperties}>
                        <KindGlyph kind={s.kind} />
                      </span>
                      {label(s.kind)}
                    </span>
                    <span className="text-xs text-carbon-textMuted">
                      {t("schedules.scheduledCount")
                        .replace("{n}", String(list.filter((item) => item.included).length))
                        .replace("{m}", String(list.length))}
                    </span>
                  </span>
                  <Button
                    label={t("schedules.entriesButton")}
                    labelKey="schedules.entriesButton"
                    glyph={<IconSchedules />}
                    onClick={() => setMembers(s.kind)}
                  />
                </div>
              );
            })}
        </Rows>
      </Card>

      {(settings.containersEnabled || settings.vmsEnabled) && (
        <div id="order" className="scroll-mt-6">
          <Card title={t("schedules.order")} hint={t("schedules.orderHint")} hueIndex={nextHue()}>
            {settings.containersEnabled && <BackupOrderPanel containers={containers} t={t} heading={t("nav.containers")} />}
            {settings.vmsEnabled && <VMBackupOrderPanel vms={vms} t={t} heading={t("nav.vms")} />}
            {settings.containersEnabled && (
              <section className="flex flex-col gap-3">
                <h3 className="text-sm font-medium text-carbon-text">{t("settings.restartHealthTitle")}</h3>
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
                  <div className="ps-4">
                    <SettingRow label={t("settings.restartHealthTimeoutLabel")} hint={t("settings.restartHealthTimeoutHint")}>
                      <NumberField
                        min={5}
                        max={3600}
                        value={settings.restartHealthTimeoutSec}
                        aria-label={t("settings.restartHealthTimeoutLabel")}
                        // The server takes 5 to 3600, so a smaller number never reaches the state.
                        onChange={(e) =>
                          scheduleField("restartHealthTimeoutSec", Math.max(5, parseInt(e.target.value, 10) || 0))
                        }
                        className="w-28 pointer-coarse:w-36 rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus"
                      />
                    </SettingRow>
                  </div>
                )}
              </section>
            )}
          </Card>
        </div>
      )}

      {settings.containersEnabled && (
        <Card title={t("settings.containerSnapshotsTitle")} hueIndex={nextHue()} newId="schedule-snapshot">
          <ToggleRow
            label={t("settings.containerSnapshots")}
            hint={t("settings.containerSnapshotsHint")}
            checked={settings.containerSnapshots}
            onChange={(v) => void autoSaveField("containerSnapshots", v, ignoreSaveState, ignoreSaveError)}
            disabled={mergedFieldBusy.containerSnapshots}
            shakeNonce={mergedFieldShake.containerSnapshots}
            pulseNonce={fieldPulse.containerSnapshots}
          />
        </Card>
      )}

      {settings.containersEnabled && (
        <Card title={t("schedules.updates")} hueIndex={nextHue()}>
          <ToggleRow
            label={t("settings.updateEveryContainer")}
            hint={t("settings.updateEveryContainerHint")}
            checked={settings.updateContainersAfterBackup}
            onChange={(v) => void autoSaveField("updateContainersAfterBackup", v, ignoreSaveState, ignoreSaveError)}
            disabled={mergedFieldBusy.updateContainersAfterBackup}
            shakeNonce={mergedFieldShake.updateContainersAfterBackup}
            pulseNonce={fieldPulse.updateContainersAfterBackup}
          />
          <ToggleRow
            label={t("settings.pruneImageAfterUpdate")}
            hint={t("settings.pruneImageAfterUpdateHint")}
            checked={settings.pruneImageAfterUpdate}
            onChange={(v) => void autoSaveField("pruneImageAfterUpdate", v, ignoreSaveState, ignoreSaveError)}
            disabled={mergedFieldBusy.pruneImageAfterUpdate}
            shakeNonce={mergedFieldShake.pruneImageAfterUpdate}
            pulseNonce={fieldPulse.pruneImageAfterUpdate}
          />
          <ToggleRow
            label={t("settings.reconcileUnraidStatus")}
            hint={t("settings.reconcileUnraidStatusHint")}
            checked={settings.reconcileUnraidUpdateStatus}
            onChange={(v) => void autoSaveField("reconcileUnraidUpdateStatus", v, ignoreSaveState, ignoreSaveError)}
            disabled={mergedFieldBusy.reconcileUnraidUpdateStatus}
            shakeNonce={mergedFieldShake.reconcileUnraidUpdateStatus}
            pulseNonce={fieldPulse.reconcileUnraidUpdateStatus}
          />
        </Card>
      )}

      {settings.containersEnabled && <IdleCard t={t} hueIndex={nextHue()} />}

      {members && (
        <ScheduleMembersWindow
          kind={members}
          items={items.filter((item) => item.kind === members && schedulable(item))}
          onChanged={loadItems}
          onClose={() => setMembers(null)}
        />
      )}
    </>
  );
}
