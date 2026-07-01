import { useEffect, useState } from "react";
import { useT } from "../lib/i18n";

// ---------------------------------------------------------------------------
// Schedule cadence builder (shared by the Plans tab and the Settings drills card)
// ---------------------------------------------------------------------------

export type CadenceMode = "off" | "daily" | "weekly" | "everyN";

export const WEEKDAYS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"] as const;

export interface CadenceState {
  mode: CadenceMode;
  time: string; // "HH:MM"
  weekdays: string[]; // subset of WEEKDAYS, for weekly
  intervalDays: number; // for everyN
}

export const DEFAULT_CADENCE: CadenceState = {
  mode: "off",
  time: "02:00",
  weekdays: ["Mon"],
  intervalDays: 3,
};

/** Build the grammar string from builder state. */
export function buildCadenceString(s: CadenceState): string {
  switch (s.mode) {
    case "off":
      return "off";
    case "daily":
      return `daily ${s.time}`;
    case "weekly": {
      const days = WEEKDAYS.filter((d) => s.weekdays.includes(d));
      const daysStr = days.length > 0 ? days.join(",") : "Mon";
      return `weekly ${daysStr} ${s.time}`;
    }
    case "everyN":
      return `everyN ${Math.max(1, s.intervalDays)} ${s.time}`;
  }
}

// prettyTime turns "HH:MM" into "H:MM" (drops a leading zero on the hour), e.g.
// "04:00" -> "4:00".
function prettyTime(hhmm: string): string {
  const m = /^(\d{1,2}):(\d{2})$/.exec(hhmm);
  return m ? `${parseInt(m[1], 10)}:${m[2]}` : hhmm;
}

// WEEKDAY_OFFSET maps the stored English abbreviation to a day in the first week
// of 2023 (2023-01-01 is a Sunday, so +1 = Mon … +7 = Sun).
const WEEKDAY_OFFSET: Record<string, number> = { Mon: 1, Tue: 2, Wed: 3, Thu: 4, Fri: 5, Sat: 6, Sun: 7 };

// localizedWeekday renders a stored English 3-letter weekday in the given
// language's short form via Intl (e.g. "Mon" -> "Mo." in de), falling back to the
// stored abbreviation.
function localizedWeekday(abbr: string, lang: string): string {
  const off = WEEKDAY_OFFSET[abbr];
  if (!off) return abbr;
  try {
    return new Intl.DateTimeFormat(lang, { weekday: "short" }).format(new Date(Date.UTC(2023, 0, off)));
  } catch {
    return abbr;
  }
}

type CadenceT = ReturnType<typeof useT>["t"];

/**
 * formatCadence renders a stored cadence string as human-readable, localized text
 * (e.g. "everyN 3 04:00" -> "jeden 3. Tag um 4:00 Uhr"). Returns "" for off/empty,
 * so callers can decide how to show a disabled schedule.
 */
export function formatCadence(raw: string, t: CadenceT, lang: string): string {
  const s = parseCadenceString(raw);
  const time = prettyTime(s.time);
  switch (s.mode) {
    case "off":
      return "";
    case "daily":
      return t("cadence.fmtDaily").replace("{time}", time);
    case "weekly": {
      const days = (s.weekdays.length ? s.weekdays : ["Mon"]).map((d) => localizedWeekday(d, lang)).join(", ");
      return t("cadence.fmtWeekly").replace("{days}", days).replace("{time}", time);
    }
    case "everyN":
      // "every 1 day" reads oddly — an interval of 1 is just daily.
      if (s.intervalDays <= 1) return t("cadence.fmtDaily").replace("{time}", time);
      return t("cadence.fmtEveryN").replace("{n}", String(s.intervalDays)).replace("{time}", time);
  }
}

/** Parse a stored cadence string back into builder state. */
export function parseCadenceString(raw: string): CadenceState {
  const s = (raw ?? "").trim();
  if (!s || s === "off") return { ...DEFAULT_CADENCE, mode: "off" };

  const dailyM = /^daily\s+(\d{1,2}:\d{2})$/.exec(s);
  if (dailyM) return { mode: "daily", time: dailyM[1], weekdays: ["Mon"], intervalDays: 3 };

  const weeklyM = /^weekly\s+([\w,]+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (weeklyM) {
    const days = weeklyM[1]
      .split(",")
      .map((d) => d.trim())
      .map((d) => d.charAt(0).toUpperCase() + d.slice(1).toLowerCase());
    return { mode: "weekly", time: weeklyM[2], weekdays: days, intervalDays: 3 };
  }

  const everyNM = /^everyN\s+(\d+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (everyNM) {
    return { mode: "everyN", time: everyNM[2], weekdays: ["Mon"], intervalDays: parseInt(everyNM[1], 10) };
  }

  // Unrecognised (e.g. raw cron from old data) — fall back to off
  return { ...DEFAULT_CADENCE, mode: "off" };
}

export function CadenceBuilder({
  label,
  value,
  disabled,
  onChange,
}: {
  label: string;
  value: string;
  disabled?: boolean;
  onChange: (v: string) => void;
}) {
  const { t, lang } = useT();
  const [state, setState] = useState<CadenceState>(() => parseCadenceString(value));

  // Re-parse when the stored value changes externally (e.g. after load or sync checkbox)
  useEffect(() => {
    setState(parseCadenceString(value));
  }, [value]);

  function update(patch: Partial<CadenceState>) {
    setState((prev) => {
      const next = { ...prev, ...patch };
      onChange(buildCadenceString(next));
      return next;
    });
  }

  function toggleWeekday(day: string) {
    const current = state.weekdays;
    const next = current.includes(day)
      ? current.filter((d) => d !== day)
      : [...current, day];
    // Always keep at least one weekday selected
    if (next.length === 0) return;
    update({ weekdays: next });
  }

  const inputCls =
    "rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-2.5 py-1.5 focus:outline-none focus:border-[#78a9ff] disabled:opacity-50";

  return (
    <div className={`flex flex-col gap-3 ${disabled ? "opacity-50 pointer-events-none" : ""}`}>
      <span className="text-xs text-carbon-textSub font-medium">{label}</span>

      {/* Mode pills */}
      <div className="flex flex-wrap gap-2">
        {(["off", "daily", "weekly", "everyN"] as CadenceMode[]).map((m) => (
          <button
            key={m}
            onClick={() => update({ mode: m })}
            className={`rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
              state.mode === m
                ? "bg-carbon-surface3 text-carbon-text"
                : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
            }`}
          >
            {m === "off" ? t("cadence.off") : m === "daily" ? t("cadence.daily") : m === "weekly" ? t("cadence.weekly") : t("cadence.everyN")}
          </button>
        ))}
      </div>

      {/* Time picker — shown for all non-off modes */}
      {state.mode !== "off" && (
        <div className="flex items-center gap-3">
          <label className="text-xs text-carbon-textMuted w-16">{t("cadence.time")}</label>
          <input
            type="time"
            value={state.time}
            onChange={(e) => update({ time: e.target.value })}
            className={inputCls}
          />
        </div>
      )}

      {/* Weekly: weekday checkboxes */}
      {state.mode === "weekly" && (
        <div className="flex items-center gap-2 flex-wrap">
          <label className="text-xs text-carbon-textMuted w-16">{t("cadence.days")}</label>
          <div className="flex flex-wrap gap-1.5">
            {WEEKDAYS.map((d) => (
              <button
                key={d}
                onClick={() => toggleWeekday(d)}
                className={`rounded px-2 py-0.5 text-xs font-medium transition-colors ${
                  state.weekdays.includes(d)
                    ? "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]"
                    : "bg-carbon-surface2 text-carbon-textSub border border-carbon-border hover:bg-carbon-hover"
                }`}
              >
                {d}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Every N days: number input */}
      {state.mode === "everyN" && (
        <div className="flex items-center gap-3">
          <label className="text-xs text-carbon-textMuted w-16">{t("cadence.every")}</label>
          <input
            type="number"
            min={1}
            value={state.intervalDays}
            onChange={(e) => {
              const n = parseInt(e.target.value, 10);
              if (!isNaN(n) && n >= 1) update({ intervalDays: n });
            }}
            className={`${inputCls} w-20`}
          />
          <span className="text-xs text-carbon-textMuted">{t("cadence.daysUnit")}</span>
        </div>
      )}

      {/* Preview — human-readable, localized (e.g. "jeden 3. Tag um 4:00 Uhr"). */}
      {state.mode !== "off" && (
        <p className="text-xs text-carbon-textSub">
          {formatCadence(buildCadenceString(state), t, lang)}
        </p>
      )}
    </div>
  );
}
