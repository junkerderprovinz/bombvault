// Retention rules as the interface offers them, and what such a rule keeps of
// a list of backups. The plan follows restic's forget policy, which is what the
// server runs after a backup, so a preview drawn from it matches the pass.

/** How many backups stay: the newest ones, then one per day, week, month and
 *  year. Zero switches a count off. */
export type KeepCounts = [last: number, daily: number, weekly: number, monthly: number, yearly: number];

export type KeepPreset = "short" | "balanced" | "long";

export const KEEP_PRESETS: Readonly<Record<KeepPreset, KeepCounts>> = {
  short: [0, 7, 4, 3, 0],
  balanced: [0, 7, 4, 6, 1],
  long: [0, 14, 8, 12, 3],
};

/** presetOf names the preset with exactly these counts, or "custom". */
export function presetOf(counts: KeepCounts): KeepPreset | "custom" {
  const presets = Object.keys(KEEP_PRESETS) as KeepPreset[];
  return presets.find((p) => KEEP_PRESETS[p].every((n, i) => n === counts[i])) ?? "custom";
}

/** The counts of a rule by name, in the order KeepCounts holds them. */
export const KEEP_BUCKETS = ["last", "daily", "weekly", "monthly", "yearly"] as const;

export type KeepBucket = (typeof KEEP_BUCKETS)[number];

export interface KeepInput {
  /** RFC 3339, as restic recorded it. The calendar day is the one in the
   *  stamp's own offset, which is how restic sorts a backup into its day, week,
   *  month and year. */
  time: string;
  /** Kept whatever the counts say, as imported backups are. It still takes its
   *  place in the buckets. */
  held?: boolean;
}

export interface KeepMark<T> {
  backup: T;
  kept: boolean;
  /** Every count that keeps the backup. Empty for a removed backup, and for one
   *  that stays only because it is held or because no count is set. */
  buckets: KeepBucket[];
}

export interface KeepPlan<T> {
  /** No count is set, so nothing is removed. */
  keepsAll: boolean;
  /** Newest first. */
  marks: KeepMark<T>[];
}

function isoWeek(year: number, month: number, day: number): number {
  // The week belongs to the year its Thursday falls in.
  const thursday = new Date(Date.UTC(year, month - 1, day));
  thursday.setUTCDate(thursday.getUTCDate() + 4 - (thursday.getUTCDay() || 7));
  const isoYear = thursday.getUTCFullYear();
  const dayOfYear = (thursday.getTime() - Date.UTC(isoYear, 0, 1)) / 86_400_000;
  return isoYear * 100 + Math.floor(dayOfYear / 7) + 1;
}

function bucketKeys(time: string, nr: number): number[] {
  const [year, month, day] = time.slice(0, 10).split("-").map(Number);
  return [nr, year * 10_000 + month * 100 + day, isoWeek(year, month, day), year * 100 + month, year];
}

/**
 * keepPlan marks which of the backups a rule keeps. Each count keeps the newest
 * backup of that many days, weeks, months or years, counting only those that
 * hold a backup, and a backup stays when any count keeps it. A count that is
 * not used up also keeps the oldest backup.
 */
export function keepPlan<T extends KeepInput>(counts: KeepCounts, backups: readonly T[]): KeepPlan<T> {
  const sorted = [...backups].sort((a, b) => Date.parse(b.time) - Date.parse(a.time));
  if (counts.every((n) => n <= 0)) {
    return { keepsAll: true, marks: sorted.map((backup) => ({ backup, kept: true, buckets: [] })) };
  }
  const left = [...counts];
  const lastKey: number[] = [];
  const marks = sorted.map((backup, nr) => {
    const keys = bucketKeys(backup.time, nr);
    const oldest = nr === sorted.length - 1;
    const buckets: KeepBucket[] = [];
    KEEP_BUCKETS.forEach((bucket, i) => {
      if (left[i] <= 0 || (keys[i] === lastKey[i] && !oldest)) return;
      left[i] -= 1;
      lastKey[i] = keys[i];
      buckets.push(bucket);
    });
    return { backup, kept: buckets.length > 0 || backup.held === true, buckets };
  });
  return { keepsAll: false, marks };
}

/** lostByChange lists the backups the rule `from` keeps and the rule `to`
 *  removes, newest first. */
export function lostByChange<T extends KeepInput>(from: KeepCounts, to: KeepCounts, backups: readonly T[]): T[] {
  const before = keepPlan(from, backups).marks;
  const after = keepPlan(to, backups).marks;
  return before.filter((mark, i) => mark.kept && !after[i].kept).map((mark) => mark.backup);
}

/** keepsFewer reports whether changing the rule removes a backup that would
 *  have stayed. */
export function keepsFewer(from: KeepCounts, to: KeepCounts, backups: readonly KeepInput[]): boolean {
  return lostByChange(from, to, backups).length > 0;
}

// Forty backups of a repository that has run for six years, as their age in
// days: two weeks of dailies, weeklies after that, then monthlies and yearlies.
const SAMPLE_AGES = [
  ...Array.from({ length: 14 }, (_, i) => i),
  ...Array.from({ length: 8 }, (_, i) => 14 + 7 * i),
  ...Array.from({ length: 12 }, (_, i) => 90 + 30 * i),
  ...Array.from({ length: 6 }, (_, i) => 540 + 360 * i),
];

/** sampleBackups is a long history ending at `now`, newest first, for showing
 *  what a rule keeps before any backup exists. */
export function sampleBackups(now: Date): KeepInput[] {
  return SAMPLE_AGES.map((age) => ({ time: new Date(now.getTime() - age * 86_400_000).toISOString() }));
}
