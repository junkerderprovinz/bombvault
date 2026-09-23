import { useT } from "../lib/i18n";
import { Badge, type BadgeTone } from "./Badge";
import { InfoBubble } from "./InfoBubble";

// The schedule badge lives in its own file rather than in Settings.tsx because
// ItemScheduleOverride.tsx needs it too, and Settings.tsx imports that file.

export type ScheduleStatus = "active" | "paused" | "off";

export function scheduleStatus(schedule: string): ScheduleStatus {
  if (!schedule || schedule === "off") return "off";
  return "active";
}

type ScheduleT = ReturnType<typeof useT>["t"];

/**
 * cadenceLabel renders a stored cadence in the badge's short form ("Täglich um
 * 02:00"). CadenceBuilder's formatCadence is the prose form for running text.
 */
export function cadenceLabel(raw: string, t: ScheduleT): string {
  const s = (raw ?? "").trim();
  if (!s || s === "off") return t("jobs.notScheduled");

  const dailyM = /^daily\s+(\d{1,2}:\d{2})$/.exec(s);
  if (dailyM) return t("jobs.cadenceDaily").replace("{time}", dailyM[1]);

  const weeklyM = /^weekly\s+([\w,]+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (weeklyM) return t("jobs.cadenceWeekly").replace("{days}", weeklyM[1]).replace("{time}", weeklyM[2]);

  const everyNM = /^everyN\s+(\d+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (everyNM) return t("jobs.cadenceEveryN", Number(everyNM[1])).replace("{time}", everyNM[2]);

  return s;
}

// Status tones, which Badge keeps out of the hue engine, so no hueIndex.
const SCHEDULE_BADGE_TONE: Record<ScheduleStatus, BadgeTone> = {
  active: "ok",
  paused: "warn",
  off: "neutral",
};

export function ScheduleBadge({
  status,
  label,
}: {
  status: ScheduleStatus;
  label: string;
}) {
  return <Badge tone={SCHEDULE_BADGE_TONE[status]}>{label}</Badge>;
}

/**
 * ScheduleRow is the "Zeitplan: [badge]" line above a CadenceBuilder. Every
 * cadence editor has one, since CadenceBuilder shows no summary of its own.
 */
export function ScheduleRow({
  schedule,
  enabled,
  hint,
}: {
  /** The stored cadence string ("" / "off" = not scheduled). */
  schedule: string;
  /** The switch of the feature this cadence belongs to (drillsEnabled,
   *  digestEnabled). A cadence whose feature is off never runs, so the badge
   *  reads as not scheduled. Omit where the cadence's own "off" mode is the
   *  switch. */
  enabled?: boolean;
  /** Names who owns the schedule when the dimmed editor below cannot change
   *  it, such as a synced schedule. Omit where the editor is the owner. */
  hint?: string;
}) {
  const { t } = useT();
  const status = enabled === false ? "off" : scheduleStatus(schedule);
  return (
    <div className="flex items-center gap-3 flex-wrap">
      <span className="text-xs text-carbon-textMuted">{t("settings.schedule")}:</span>
      <ScheduleBadge
        status={status}
        label={status === "off" ? t("jobs.notScheduled") : cadenceLabel(schedule, t)}
      />
      {hint && <InfoBubble tip={hint} />}
    </div>
  );
}
