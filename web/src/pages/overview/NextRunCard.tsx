import type { CSSProperties } from "react";
import type { DomainStatus, ScheduleNext } from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import type { useT } from "../../lib/i18n";
import { formatDuration, formatTs } from "../../lib/reltime";
import { MobileSectionLabel } from "../../components/mobile/MobileSectionLabel";
import { IconBackupNow } from "../../components/Sidebar";
import { SummaryCell } from "./SummaryCell";

/**
 * When the next backup actually fires, taken from the scheduler rather than
 * derived here (issue #187).
 *
 * Ranking cadence strings on an approximate period cannot be made right, only
 * less wrong: it reads "weekly Sun 04:00" as seven days out whatever today
 * is, and it cannot walk an `everyN` entry through its due gate, so an
 * `everyN 7` pass that last ran three days ago is four days out to the
 * scheduler and seven to the tile. Two issues came out of that (#177, #186).
 * GET /api/schedule/next answers the question directly, and the activity log
 * and the Unraid widget already read it.
 *
 * The result names a moment rather than a schedule: "Täglich um 05:00"
 * describes a rule, and what a dashboard is asked is when the next one runs.
 * Filtered to job "backup", since the list also carries the offsite, drill,
 * tamper, digest and watchdog fires.
 *
 * The scheduler registers the "Backup Everything" pass even when all five
 * domains are switched off, because its entry has no off field of its own. A
 * pass over zero enabled domains backs nothing up (internal/api/everything.go
 * logs exactly that and writes no snapshot), so without the domain condition
 * the result would name a moment at which nothing gets backed up.
 */
function nextBackupFireAt(
  scheduleNext: ScheduleNext[],
  domains: DomainStatus[]
): { at: ScheduleNext | null; ms: number } {
  const anyDomainOn = domains.some((d) => d.enabled);
  const at =
    scheduleNext.find((n) => n.job === "backup" && (n.domain !== "everything" || anyDomainOn)) ??
    null;
  return { at, ms: at ? new Date(at.next).getTime() : NaN };
}

/** Reader-facing label for a ScheduleNext domain; the same vocabulary the
 *  protection rows and the activity log use, so the mobile Next-run card
 *  cannot invent a second name for a domain the rest of the app already
 *  names. */
function scheduleDomainLabel(t: ReturnType<typeof useT>["t"], domain: string): string {
  switch (domain) {
    case "containers":
      return t("dashboard.domainContainers");
    case "vms":
      return t("dashboard.domainVMs");
    case "flash":
      return t("dashboard.domainFlash");
    case "files":
      return t("dashboard.domainFiles");
    case "config":
      return t("dashboard.domainConfig");
    case "everything":
      return t("activityLog.domainEverything");
    default:
      return domain;
  }
}

export function NextRunCard({
  t,
  scheduleNext,
  domains,
  loading,
  dense,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  scheduleNext: ScheduleNext[];
  domains: DomainStatus[];
  loading: boolean;
  /** True = the phone glance block; false = the summary tier's middle cell. */
  dense: boolean;
  hueIndex?: number;
}) {
  // The same derivation both faces render; one source of truth
  // (nextBackupFireAt above), two surfaces that cannot disagree. Accent here
  // is sanctioned: soft tint + accent-derived text on the phone face's one
  // icon and the countdown chip (accentSoft backdrop + accentText chip),
  // never a solid accent fill.
  const { at, ms } = nextBackupFireAt(scheduleNext, domains);
  const countdown = Number.isFinite(ms)
    ? t("dashboard.summaryNextIn").replace(
        "{countdown}",
        formatDuration(Math.max(0, Math.round((ms - Date.now()) / 1000)))
      )
    : "";

  if (dense) {
    return (
      <section className="relative glim-notch-card glim-hue" style={hueVars(hueIndex ?? 0) as CSSProperties}>
        <MobileSectionLabel t={t} labelKey="dashboard.summaryNextBackup" />
        <div className="flex items-center gap-2 rounded-card bg-carbon-surface p-4 pt-5">
          {loading ? (
            <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
          ) : at ? (
            <>
              <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-card bg-accentSoft text-accentText">
                <IconBackupNow />
              </span>
              <span className="min-w-0 flex-1">
                {/* Schedule name (14px ≈ text-sm / 600) + when · what meta (12px).
                    "what" is the run kind the scheduler entry names; the card
                    labels a backup fire, so run.kindBackup is the honest kind. */}
                <span className="block truncate text-sm font-semibold text-carbon-text">
                  {scheduleDomainLabel(t, at.domain)}
                </span>
                <span className="mt-0.5 block truncate text-xs text-carbon-textMuted">
                  {t("run.kindBackup")} · {formatTs(Math.round(ms / 1000))}
                </span>
              </span>
              {countdown && (
                <span className="shrink-0 rounded-pill bg-accentSoft px-2 py-1 text-xs font-semibold text-accentText">
                  {countdown}
                </span>
              )}
            </>
          ) : (
            <p className="text-sm text-carbon-textMuted">{t("dashboard.rpoOff")}</p>
          )}
        </div>
      </section>
    );
  }

  return (
    <SummaryCell label={t("dashboard.summaryNextBackup")} hueIndex={hueIndex}>
      {loading ? (
        <span className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</span>
      ) : (
        <span className="text-sm text-carbon-text truncate min-w-0">
          {countdown || t("dashboard.rpoOff")}
        </span>
      )}
    </SummaryCell>
  );
}
