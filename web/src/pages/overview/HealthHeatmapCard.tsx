import { useEffect, useState } from "react";
import { getHistory } from "../../lib/api";
import type { DayStat, HistoryDay } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { Selector } from "../../components/Selector";
import { Card } from "./Card";

type HeatDomain = "containers" | "vms" | "flash" | "config" | "files" | "zfs";

// cellColor maps a day's outcome (for the selected domain) to a fill color:
// any failure → red; all-ok → green shades that deepen with more successful
// runs; no runs → neutral carbon surface. Colors come from the theme vars in
// index.css (#105) so the scale stays legible in light mode too.
function cellColor(stat: DayStat | undefined): string {
  if (!stat || (stat.ok === 0 && stat.failed === 0)) return "var(--carbon-surface2, #262626)";
  if (stat.failed > 0) return "var(--status-fail-solid, #ff8389)";
  // All ok: a deeper green for more runs that day.
  if (stat.ok >= 3) return "var(--heat-ok-3, #42be65)";
  if (stat.ok === 2) return "var(--heat-ok-2, #6fdc8c)";
  return "var(--heat-ok-1, #a7f0ba)";
}

// mondayIndex returns 0..6 for Mon..Sun (JS getDay() is 0=Sun..6=Sat).
function mondayIndex(d: Date): number {
  return (d.getDay() + 6) % 7;
}

export function HealthHeatmapCard({
  t,
  selectedDay,
  onSelectDay,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  /** The Activity Log's active day filter (ISO YYYY-MM-DD, local). The
   *  matching cell renders an accent outline; clicking it again clears. */
  selectedDay: string | null;
  /** Fired with a cell's local ISO day. The Dashboard toggles the Activity
   *  Log day filter and scrolls the log into view. Zero-run days fire too:
   *  the log then shows nothing for that day. */
  onSelectDay: (isoDay: string) => void;
  hueIndex?: number;
}) {
  const [days, setDays] = useState<HistoryDay[]>([]);
  const [loading, setLoading] = useState(true);
  const [domain, setDomain] = useState<HeatDomain>("containers");

  useEffect(() => {
    let active = true;
    getHistory(90)
      .then((res) => {
        if (!active) return;
        if (res.ok) setDays(res.days ?? []);
      })
      .catch(() => {/* non-fatal */})
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  const byDate = new Map(days.map((d) => [d.date, d]));
  const statFor = (d: HistoryDay | undefined): DayStat | undefined =>
    d ? d[domain] : undefined;

  // Build columns of 7 days (Mon..Sun). Lead the first column with empty cells so
  // each row lines up with its weekday. Parse the YYYY-MM-DD as a local date.
  const cells: Array<{ key: string; date?: string; stat?: DayStat }> = [];
  if (days.length > 0) {
    const first = new Date(days[0].date + "T00:00:00");
    const last = new Date(days[days.length - 1].date + "T00:00:00");
    const lead = mondayIndex(first);
    for (let i = 0; i < lead; i++) {
      cells.push({ key: `lead-${i}` });
    }
    // Walk calendar days (setDate), not fixed 24h steps: a millisecond walk
    // lands on the same local date twice on the 25h DST fall-back day, which
    // would emit a duplicate cell and shift the whole grid.
    const cur = new Date(first);
    while (cur <= last) {
      const iso = cur.toLocaleDateString("en-CA"); // YYYY-MM-DD, local
      const hd = byDate.get(iso);
      cells.push({ key: iso, date: iso, stat: statFor(hd) });
      cur.setDate(cur.getDate() + 1);
    }
  }

  // Chunk the flat day list into week columns of 7 (Mon..Sun rows).
  const weeks: Array<typeof cells> = [];
  for (let i = 0; i < cells.length; i += 7) {
    weeks.push(cells.slice(i, i + 7));
  }

  const domainLabel = (d: HeatDomain): string => {
    switch (d) {
      case "containers":
        return t("dashboard.domainContainers");
      case "vms":
        return t("dashboard.domainVMs");
      case "flash":
        return t("dashboard.domainFlash");
      case "config":
        return t("dashboard.domainConfig");
      case "files":
        return t("dashboard.domainFiles");
      case "zfs":
        return t("dashboard.domainZFS");
    }
  };

  return (
    <Card title={t("dashboard.healthTitle")} hueIndex={hueIndex}>
      {/* Hued like every other selector. Two rainbow positions match the
          grid's fail and ok colours in the dark theme, which is harmless:
          every cell says its counts in text as well. */}
      <Selector
        items={(["containers", "vms", "flash", "config", "files", "zfs"] as HeatDomain[]).map((d) => ({
          id: d,
          label: domainLabel(d),
        }))}
        label={t("dashboard.healthTitle")}
        select="one"
        active={domain}
        onChange={(id) => setDomain(id as HeatDomain)}
        size="lg"
        equalWidth
      />
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {!loading && days.length > 0 && (
        <div className="flex flex-col gap-2 glim-content-fade">
          <div className="flex gap-1 overflow-x-auto">
            {weeks.map((week, wi) => (
              <div key={wi} className="flex flex-col gap-1">
                {week.map((cell) => {
                  if (!cell.date) {
                    return <div key={cell.key} className="w-[11px] h-[11px]" />;
                  }
                  const date = cell.date;
                  const stat = cell.stat ?? { ok: 0, failed: 0 };
                  const active = selectedDay === date;
                  // A real <button>, so Enter and Space work for free. Tailwind's
                  // preflight strips the browser button chrome, so only the
                  // cell's own size and fill classes remain.
                  // The tooltip stays the plain "<date>: N ok, N failed" data
                  // line (it doubles as the accessible name).
                  //
                  // bv-convention-exception: control-reads-engine-tokens --
                  // an 11px heat-map cell, not a control with chrome. The
                  // shape engine's control radius is 10px in `round`, which
                  // on an 11px box is a disc, and the grid stops reading as
                  // a grid. `rounded-xs` (2px) is also exactly what the four
                  // legend swatches below already use, so the cells and
                  // their legend stay one shape instead of drifting apart.
                  return (
                    <button
                      key={cell.key}
                      type="button"
                      onClick={() => onSelectDay(date)}
                      aria-pressed={active}
                      className={`w-[11px] h-[11px] rounded-xs cursor-pointer focus:outline-solid focus:outline-2 focus:outline-(--focus-ring) ${
                        active ? "outline-solid outline-2 outline-accent" : ""
                      }`}
                      style={{ backgroundColor: cellColor(cell.stat) }}
                      title={`${date}: ${stat.ok} ok, ${stat.failed} failed`}
                    />
                  );
                })}
              </div>
            ))}
          </div>
          {/* Legend */}
          <div className="flex items-center gap-1.5 text-xs text-carbon-textMuted">
            <span>{t("dashboard.heatLess")}</span>
            <span className="w-[11px] h-[11px] rounded-xs" style={{ backgroundColor: "var(--carbon-surface2, #262626)" }} />
            <span className="w-[11px] h-[11px] rounded-xs" style={{ backgroundColor: "var(--heat-ok-1, #a7f0ba)" }} />
            <span className="w-[11px] h-[11px] rounded-xs" style={{ backgroundColor: "var(--heat-ok-2, #6fdc8c)" }} />
            <span className="w-[11px] h-[11px] rounded-xs" style={{ backgroundColor: "var(--heat-ok-3, #42be65)" }} />
            <span>{t("dashboard.heatMore")}</span>
          </div>
        </div>
      )}
    </Card>
  );
}
