import type { Settings } from "../../lib/api";
import { CadenceBuilder } from "../../components/CadenceBuilder";
import { Card, ToggleRow } from "../settings/shared";
import { NumberField } from "../../components/NumberField";
import { ScheduleRow } from "../../components/ScheduleBadge";
import { useT } from "../../lib/i18n";

// RestoreChecksSection is the card for scheduled restore-verification drills.
export function RestoreChecksSection({
  settings,
  update,
  busy,
  shake,
  pulse,
  t,
  hueIndex,
}: {
  settings: Settings;
  update: (patch: Partial<Settings>) => void;
  /** Save feedback for the two toggles, which save themselves. */
  busy?: Partial<Record<"drillsEnabled" | "offsiteDrillsEnabled", boolean>>;
  shake?: Partial<Record<"drillsEnabled" | "offsiteDrillsEnabled", number>>;
  /** The success counterpart of shake. SettingsPage passes its whole
   *  fieldPulse map, which is keyed by every Settings key. */
  pulse?: Partial<Record<"drillsEnabled" | "offsiteDrillsEnabled", number>>;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  return (
    <Card title={t("verify.auto")} hint={t("verify.hint")} hueIndex={hueIndex}>
      <ToggleRow
        label={t("verify.auto")}
        checked={settings.drillsEnabled}
        onChange={(v) => update({ drillsEnabled: v })}
        disabled={busy?.drillsEnabled}
        shakeNonce={shake?.drillsEnabled}
        pulseNonce={pulse?.drillsEnabled}
      />
      {/* A sub-switch is absent while its parent is off rather than dimmed:
          a switch greyed because something else is off offers a decision
          nobody can make. It is still disabled while its own save runs. */}
      {settings.drillsEnabled && (
        <ToggleRow
          label={t("settings.offsiteDrills")}
          hint={t("settings.offsiteDrillsHelp")}
          checked={settings.offsiteDrillsEnabled}
          disabled={busy?.offsiteDrillsEnabled}
          onChange={(v) => update({ offsiteDrillsEnabled: v })}
          shakeNonce={shake?.offsiteDrillsEnabled}
          pulseNonce={pulse?.offsiteDrillsEnabled}
        />
      )}
      {/* `enabled` follows drillsEnabled because this card's on/off is a
          separate toggle, not the cadence's own "off" mode. */}
      <ScheduleRow schedule={settings.drillsSchedule} enabled={settings.drillsEnabled} />
      {/* The editor hides with the switch too. The ScheduleRow above still
          shows what would run. */}
      {settings.drillsEnabled && (
        <div className="rounded-card bg-carbon-surface2 p-4">
          {/* No `modes` restriction (#166): the drill pass stamps
              schedule_job_runs when it runs and the scheduler gates on that, so
              "every N days" is enforced here and the API accepts it. */}
          <CadenceBuilder
            label={t("settings.schedule")}
            value={settings.drillsSchedule}
            onChange={(v) => update({ drillsSchedule: v })}
            hueIndex={hueIndex}
          />
        </div>
      )}
      <label className="flex flex-col gap-1 max-w-40">
        <span className="text-xs text-carbon-textSub">{t("verify.subsetPct")}</span>
        <NumberField
          min={1}
          max={100}
          value={settings.drillsSubsetPct}
          onChange={(e) => {
            const n = parseInt(e.target.value, 10);
            const clamped = isNaN(n) ? 1 : Math.min(100, Math.max(1, n));
            update({ drillsSubsetPct: clamped });
          }}
          className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
        />
      </label>
    </Card>
  );
}
