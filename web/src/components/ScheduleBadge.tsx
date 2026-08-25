import { useT } from "../lib/i18n";
import { Badge, type BadgeTone } from "./Badge";

// ---------------------------------------------------------------------------
// Resolved-schedule badge (the green "Täglich um 02:00" chip above a cadence
// editor) — extracted from Settings.tsx, unchanged in appearance.
//
// WHY THIS FILE EXISTS (jdp, live-review, with a screenshot of a schedule card
// showing the green badge "Täglich um 02:00" above the card AND the line
// "täglich um 2:00 Uhr" inside it: "Bei den ganzen Zeitplänen den Text in der
// Auswahlcard entfernen. Das wird ja über der Card schon als grüner Badge
// angezeigt. Ist redundant."):
//
// CadenceBuilder used to render its OWN one-line preview paragraph of the
// cadence it was editing. That paragraph is gone now (see CadenceBuilder.tsx's
// own comment where it was), which is only correct as long as EVERY cadence
// editor in the app has a resolved-schedule badge above it instead — otherwise
// removing the paragraph would delete information rather than a duplicate.
// Five of the nine call sites already had one (the four domain Cards +
// Selbst-Backup, all hand-rolling the same row markup and the same
// off/cadence ternary); three had nothing at all (Restore-Prüfungen,
// Wochenbericht, Wiederherstellungs-Prüfplan) and ItemScheduleOverride had a
// plain-text summary instead of a badge.
//
// Rather than hand-roll the row an 8th and 9th time, the row itself is a
// component now (`ScheduleRow`) and lives here with the status/label helpers
// it needs, so "does this cadence editor show its resolved schedule?" has ONE
// answer for the whole app instead of nine independently-drifting ones. A
// shared file rather than an export from Settings.tsx specifically because
// ItemScheduleOverride.tsx also needs the badge and Settings.tsx already
// imports THAT — exporting from Settings.tsx would be an import cycle.
//
// STATUS COLOURS STAY OUT OF THE HUE ENGINE. jdp confirmed the green badge's
// own colour is correct as-is; ok/warn/neutral are rule 4 state hues that
// Badge deliberately exempts from `hueIndex` (see Badge.tsx's own `hueIndex`
// doc: "ok/fail/warn/neutral are load-bearing status signals hueIndex must
// never overwrite"). Nothing here passes a hueIndex, and nothing here should.
// ---------------------------------------------------------------------------

export type ScheduleStatus = "active" | "paused" | "off";

export function scheduleStatus(schedule: string): ScheduleStatus {
  if (!schedule || schedule === "off") return "off";
  return "active";
}

type ScheduleT = ReturnType<typeof useT>["t"];

/**
 * cadenceLabel renders a stored cadence string in the badge's own short,
 * capitalized grammar ("Täglich um 02:00") — deliberately NOT CadenceBuilder's
 * `formatCadence` ("täglich um 2:00 Uhr"), which is the sentence-cased prose
 * form used inside running text. Moved here verbatim from Settings.tsx along
 * with the badge it has always fed.
 */
export function cadenceLabel(raw: string, t: ScheduleT): string {
  const s = (raw ?? "").trim();
  if (!s || s === "off") return t("jobs.notScheduled");

  const dailyM = /^daily\s+(\d{1,2}:\d{2})$/.exec(s);
  if (dailyM) return t("jobs.cadenceDaily").replace("{time}", dailyM[1]);

  const weeklyM = /^weekly\s+([\w,]+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (weeklyM) return t("jobs.cadenceWeekly").replace("{days}", weeklyM[1]).replace("{time}", weeklyM[2]);

  const everyNM = /^everyN\s+(\d+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (everyNM) return t("jobs.cadenceEveryN").replace("{n}", everyNM[1]).replace("{time}", everyNM[2]);

  return s;
}

// ScheduleBadge → Badge tone mapping (GlimStone form-engine Task 5 follow-up):
// this was its own hand-rolled `px-2 py-0.5 rounded-control text-xs
// font-medium` + tone-lookup pair, byte-for-byte the same shape the shared
// Badge component now owns — a 6th duplicate the migration's audit found
// alongside the five named in the plan. active/paused/off map onto Badge's
// ok/warn/neutral tones (the only three a schedule status ever needs).
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
 * ScheduleRow is the full "Zeitplan: [badge]" line that sits directly above a
 * CadenceBuilder well. Every cadence editor in the app renders one — that is
 * what makes CadenceBuilder's own removed preview paragraph redundant rather
 * than missed.
 */
export function ScheduleRow({
  schedule,
  enabled,
}: {
  /** The stored cadence string ("" / "off" = not scheduled). */
  schedule: string;
  /** Optional master switch for the whole feature this cadence belongs to
   *  (Restore-Prüfungen's `drillsEnabled`, Wochenbericht's `digestEnabled`).
   *  A stored cadence whose feature toggle is OFF genuinely does not run, so
   *  the badge reads "Kein Zeitplan" rather than showing a green time the
   *  scheduler will never honour — the same thing the CadenceBuilder below it
   *  already says by rendering `disabled`. Omit where the cadence's own "off"
   *  mode IS the on/off control (the four domain Cards, Selbst-Backup). NOTE
   *  this deliberately does NOT reach for the `"paused"`/amber status: that
   *  would be a new warning-coloured state nobody asked for, and rule 5 keeps
   *  status colours out of any redesign. */
  enabled?: boolean;
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
    </div>
  );
}
