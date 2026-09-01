// ---------------------------------------------------------------------------
// nextBackup — which schedule the dashboard's "next backup" cell should name.
//
// This is deliberately NOT a countdown. The backend does expose real next-fire
// timestamps (GET /api/schedule/next), but they are keyed by scheduler entry
// rather than by domain, and the cell's job is the simpler one of naming the
// cadence that fires soonest. Keeping it pure (and out of Dashboard.tsx) is
// what lets it be tested in the plain node environment.
// ---------------------------------------------------------------------------
import { parseCadenceString } from "../components/CadenceBuilder";
import type { CadenceState } from "../components/CadenceBuilder";
import { cronPeriodSeconds } from "./cron";
import type { DomainStatus } from "./api";

// cadencePeriodDays approximates how often a parsed cadence fires, in days, so
// the soonest (most frequent) enabled schedule can be picked WITHOUT a live
// next-run timestamp. Smaller = fires sooner; "off" yields Infinity so it never
// wins.
export function cadencePeriodDays(s: CadenceState): number {
  switch (s.mode) {
    case "daily":
      return 1;
    case "everyN":
      return Math.max(1, s.intervalDays);
    case "weekly":
      return 7 / Math.max(1, s.weekdays.length);
    case "cron": {
      // Raw cron cadence (#107): approximate the fire interval from the gap
      // between its first two fires (mirrors the backend's PeriodSeconds).
      // 0 = not computable → Infinity so it never falsely wins "soonest".
      const secs = cronPeriodSeconds(s.cron);
      return secs > 0 ? secs / 86400 : Infinity;
    }
    default:
      return Infinity;
  }
}

// minutesOfDay turns "HH:MM" into minutes since midnight — a stable tiebreak
// between two equally-frequent schedules (the earlier clock time wins).
export function minutesOfDay(hhmm: string): number {
  const m = /^(\d{1,2}):(\d{2})$/.exec(hhmm);
  return m ? parseInt(m[1], 10) * 60 + parseInt(m[2], 10) : 24 * 60;
}

/** The cadence the "next backup" cell names, and where it comes from. */
export type NextBackup = {
  /** The raw stored cadence string, for formatCadence to render. */
  cadence: string;
  /** True when this cadence belongs to the whole-server "Backup Everything"
   *  pass rather than to a domain's own schedule, so the cell can say so
   *  instead of implying the domain is scheduled by itself. */
  viaEverything: boolean;
};

// Candidate carries its sort keys rather than recomputing them per comparison.
// cadencePeriodDays is not free for a cron cadence: it walks cronPeriodSeconds'
// stepping loop twice, and a never-firing expression steps a five-year horizon.
type Candidate = NextBackup & { periodDays: number; minutes: number };

function candidate(cadence: string, viaEverything: boolean, state: CadenceState): Candidate {
  return {
    cadence,
    viaEverything,
    periodDays: cadencePeriodDays(state),
    minutes: minutesOfDay(state.time),
  };
}

/**
 * nextBackupCadence picks the soonest (most frequent) cadence that will actually
 * back something up, and reports whether it is a domain's own schedule or the
 * whole-server "Backup Everything" pass's. The pass competes as a candidate in
 * its own right, which is why it arrives as its own argument rather than through
 * the domains (see DomainStatusEntry.CoveredBy in internal/api/service.go for
 * why that field cannot carry it).
 *
 * Both halves of #186 come from having read the domains alone: the reported half
 * (no domain has a cadence, so the cell said "not scheduled") and its mirror
 * (every domain is weekly, so the cell named Sunday while the real next backup
 * was that night).
 *
 * Returns null when nothing is scheduled at all, so the caller can show its own
 * "not scheduled" label rather than an empty string.
 */
export function nextBackupCadence(
  domains: DomainStatus[],
  everythingSchedule?: string
): NextBackup | null {
  const candidates: Candidate[] = [];

  for (const d of domains) {
    if (!d.enabled) continue;
    const state = parseCadenceString(d.schedule);
    if (state.mode !== "off") candidates.push(candidate(d.schedule, false, state));
  }

  // A pass over zero enabled domains backs nothing up — internal/api/
  // everything.go logs exactly that and writes no snapshot — so it only counts
  // as a next backup while at least one domain is switched on.
  const passCadence = everythingSchedule ?? "";
  const pass = parseCadenceString(passCadence);
  if (pass.mode !== "off" && domains.some((d) => d.enabled)) {
    candidates.push(candidate(passCadence, true, pass));
  }

  if (candidates.length === 0) return null;
  // Stable sort, so an exact tie between a domain's own cadence and the pass's
  // keeps the domain's — naming the schedule the user set beats naming the pass
  // when both fire at the same moment.
  candidates.sort((a, b) => a.periodDays - b.periodDays || a.minutes - b.minutes);
  return { cadence: candidates[0].cadence, viaEverything: candidates[0].viaEverything };
}
