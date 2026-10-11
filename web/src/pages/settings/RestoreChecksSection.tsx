import type { Container, Settings, VM } from "../../lib/api";
import { Card, ToggleRow } from "../settings/shared";
import { Note } from "../../components/entry/Note";
import { NumberField } from "../../components/NumberField";
import { PlanButton } from "../../components/PlanButton";
import { SelectField } from "../../components/SelectField";
import { SettingRow } from "../../components/storage/rows";
import { useT } from "../../lib/i18n";
import { PackGrid, SandboxPicture } from "./pictures";

const TARGET_FIELD = "rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus";

type DrillKey = "drillsEnabled" | "offsiteDrillsEnabled" | "startTestEnabled";

/**
 * RestoreChecksSection is the card of the checks that run on a plan: the trial
 * restore, with what it reads and where it starts things, and the test of the
 * delete protection off site.
 */
export function RestoreChecksSection({
  settings,
  update,
  tamperActive,
  containers,
  vms,
  busy,
  shake,
  pulse,
  t,
  hueIndex,
}: {
  settings: Settings;
  update: (patch: Partial<Settings>) => void;
  /** Whether an off-site location refuses deletes, without which the test of
   *  the delete protection never runs. */
  tamperActive: boolean;
  /** What a trial restore can start, to choose its test target from. */
  containers: Container[];
  vms: VM[];
  /** Save feedback for the switches, which save themselves. */
  busy?: Partial<Record<DrillKey, boolean>>;
  shake?: Partial<Record<DrillKey, number>>;
  pulse?: Partial<Record<DrillKey, number>>;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  // The trial restore has a switch and a cadence of its own; its plan shows
  // both, with Off for the switch.
  function setDrills(plan: string) {
    if (plan === "off") update({ drillsEnabled: false });
    else update(settings.drillsEnabled ? { drillsSchedule: plan } : { drillsEnabled: true, drillsSchedule: plan });
  }

  return (
    <Card title={t("verify.autoChecks")} hueIndex={hueIndex}>
      <div className="flex flex-col divide-y divide-carbon-border/60">
        <SettingRow label={t("verify.trialRestore")} hint={t("verify.trialRestoreHint")}>
          <PlanButton
            label={t("verify.trialRestore")}
            value={settings.drillsEnabled ? settings.drillsSchedule : "off"}
            onChange={setDrills}
            disabled={busy?.drillsEnabled}
            hueIndex={hueIndex}
          />
        </SettingRow>
        {settings.drillsEnabled && (
          <div className="flex flex-col gap-4 py-3 ps-4">
            <ToggleRow
              label={t("settings.offsiteDrills")}
              hint={t("settings.offsiteDrillsHelp")}
              checked={settings.offsiteDrillsEnabled}
              disabled={busy?.offsiteDrillsEnabled}
              onChange={(v) => update({ offsiteDrillsEnabled: v })}
              shakeNonce={shake?.offsiteDrillsEnabled}
              pulseNonce={pulse?.offsiteDrillsEnabled}
            />
            <ToggleRow
              label={t("settings.startTest")}
              hint={t("settings.startTestHelp")}
              checked={settings.startTestEnabled}
              disabled={busy?.startTestEnabled}
              onChange={(v) => update({ startTestEnabled: v })}
              shakeNonce={shake?.startTestEnabled}
              pulseNonce={pulse?.startTestEnabled}
            />
            <SettingRow label={t("verify.subsetPct")} hint={t("verify.subsetPctHint")}>
              <NumberField
                min={1}
                max={100}
                value={settings.drillsSubsetPct}
                aria-label={t("verify.subsetPct")}
                onChange={(e) => {
                  const n = parseInt(e.target.value, 10);
                  const clamped = isNaN(n) ? 1 : Math.min(100, Math.max(1, n));
                  update({ drillsSubsetPct: clamped });
                }}
                className="w-28 rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus"
              />
            </SettingRow>
            <div className="flex flex-wrap gap-x-6 gap-y-4">
              <PackGrid percent={settings.drillsSubsetPct} />
              <SandboxPicture startTest={settings.startTestEnabled} />
            </div>
            {settings.containersEnabled && (
              <SettingRow label={t("verify.testTarget")} hint={t("verify.testTargetHint")}>
                <SelectField
                  value={settings.drDrillTarget}
                  onChange={(v) => update({ drDrillTarget: v })}
                  label={t("verify.testTarget")}
                  options={[
                    { value: "", label: t("verify.newestBackup") },
                    ...containers.map((c) => ({ value: c.name, label: c.name })),
                  ]}
                  className={TARGET_FIELD}
                />
              </SettingRow>
            )}
            {settings.vmsEnabled && (
              <SettingRow label={t("verify.testTargetVM")}>
                <SelectField
                  value={settings.drDrillTargetVm}
                  onChange={(v) => update({ drDrillTargetVm: v })}
                  label={t("verify.testTargetVM")}
                  options={[
                    { value: "", label: t("verify.newestBackup") },
                    // The value is the raw libvirt name: the server matches it
                    // against the vm: backup tag, never the friendly name a
                    // TrueNAS VM shows here.
                    ...vms.map((vm) => ({ value: vm.libvirtName, label: vm.name })),
                  ]}
                  className={TARGET_FIELD}
                />
              </SettingRow>
            )}
          </div>
        )}
        <SettingRow label={t("verify.tamper")} hint={t("verify.tamperHint")}>
          <PlanButton
            label={t("verify.tamper")}
            value={settings.tamperTestSchedule}
            onChange={(v) => update({ tamperTestSchedule: v })}
            hueIndex={hueIndex}
          />
        </SettingRow>
      </div>
      {!tamperActive && settings.tamperTestSchedule !== "off" && <Note tone="warn">{t("settings.tamperScheduleInactive")}</Note>}
    </Card>
  );
}
