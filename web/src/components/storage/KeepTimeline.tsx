import type { CSSProperties } from "react";
import { useT } from "../../lib/i18n";
import { KEEP_BUCKETS, keepPlan, type KeepBucket, type KeepCounts, type KeepInput } from "../../lib/keepPlan";

const UNIT = { daily: "day", weekly: "week", monthly: "month", yearly: "year" } as const;

const DAY_MS = 86_400_000;

/** How long ago a backup was made, in the largest unit that fits. */
function ago(lang: string, time: string, now: number): string {
  const days = (now - Date.parse(time)) / DAY_MS;
  const format = new Intl.RelativeTimeFormat(lang, { numeric: "always" });
  if (days >= 365) return format.format(-Math.round(days / 365.25), "year");
  if (days >= 30) return format.format(-Math.round(days / 30.44), "month");
  return format.format(-Math.round(days), "day");
}

/**
 * KeepTimeline draws what a retention rule keeps of a list of backups: one
 * dot per backup, the oldest first, and under them one band per count that
 * spans the backups it keeps. A removed backup sinks and fades.
 */
export function KeepTimeline({ counts, backups }: { counts: KeepCounts; backups: readonly KeepInput[] }) {
  const { t, lang } = useT();
  const plan = keepPlan(counts, backups);
  const n = plan.marks.length;
  const kept = plan.marks.filter((mark) => mark.kept).length;
  // marks are newest first and the line runs from the oldest.
  const column = (i: number) => n - 1 - i;

  function label(bucket: KeepBucket, count: number): string {
    if (bucket === "last") return t("storage.keep.latest", count);
    return new Intl.NumberFormat(lang, { style: "unit", unit: UNIT[bucket], unitDisplay: "long" }).format(count);
  }

  function bar(bucket: KeepBucket): CSSProperties | null {
    const hits = plan.marks.flatMap((mark, i) => (mark.buckets.includes(bucket) ? [i] : []));
    if (hits.length === 0) return null;
    const oldest = hits[hits.length - 1];
    return {
      insetInlineStart: `${(column(oldest) / n) * 100}%`,
      width: `${((oldest - hits[0] + 1) / n) * 100}%`,
    };
  }

  const bands = KEEP_BUCKETS.map((bucket, i) => ({ bucket, count: counts[i] })).filter(
    ({ bucket, count }) => bucket !== "last" || count > 0
  );

  return (
    <div className="glim-pic glim-keep text-sm text-carbon-text" style={{ "--keep-n": n } as CSSProperties}>
      <span className="glim-keep-gap" />
      <span className="glim-keep-dots" aria-hidden="true">
        {[...plan.marks].reverse().map((mark, k) => {
          const band = mark.kept ? `glim-keep-band-${mark.buckets[0] ?? "all"}` : "glim-keep-gone";
          return <i key={k} className={`glim-keep-dot ${band}`} />;
        })}
      </span>

      {plan.keepsAll ? (
        <>
          <span className="glim-keep-label glim-keep-band-all">{t("storage.keep.all")}</span>
          <span className="glim-keep-track glim-keep-band-all">
            <i className="glim-keep-bar" style={{ insetInlineStart: 0, width: "100%" }} />
          </span>
        </>
      ) : (
        bands.map(({ bucket, count }) => {
          const span = bar(bucket);
          return (
            <span key={bucket} className="contents">
              <span className={`glim-keep-label glim-keep-band-${bucket}`}>{label(bucket, count)}</span>
              <span className={`glim-keep-track glim-keep-band-${bucket}`}>
                {span && <i className="glim-keep-bar" style={span} />}
              </span>
            </span>
          );
        })
      )}

      <span className="glim-keep-gap" />
      <span className="flex justify-between text-xs text-carbon-textMuted">
        <span>{n > 0 ? ago(lang, plan.marks[n - 1].backup.time, Date.now()) : ""}</span>
        <span>{new Intl.RelativeTimeFormat(lang, { numeric: "auto" }).format(0, "day")}</span>
      </span>

      <p className="col-span-full mt-1.5 flex items-baseline gap-1.5 text-sm text-carbon-textSub">
        <span>{t("storage.keep.keeps")}</span>
        <b className="min-w-[2ch] text-base font-semibold tabular-nums text-carbon-text">
          {kept}
        </b>
        <span className="mx-1 text-carbon-textMuted" aria-hidden="true">
          ·
        </span>
        <span>{t("storage.keep.removed")}</span>
        <b className="min-w-[2ch] text-base font-semibold tabular-nums text-carbon-text">
          {n - kept}
        </b>
      </p>
    </div>
  );
}
