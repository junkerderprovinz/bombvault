import { useEffect, useState } from "react";
import { useT } from "../lib/i18n";
import { isValidCronExpression, nextCronFires } from "../lib/cron";

// ---------------------------------------------------------------------------
// Schedule cadence builder (shared by the Plans tab and the Settings drills card)
// ---------------------------------------------------------------------------

export type CadenceMode = "off" | "daily" | "weekly" | "everyN" | "cron";

export const WEEKDAYS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"] as const;

export interface CadenceState {
  mode: CadenceMode;
  time: string; // "HH:MM"
  weekdays: string[]; // subset of WEEKDAYS, for weekly
  intervalDays: number; // for everyN
  cron: string; // raw 5-field cron expression, for cron (#107)
}

export const DEFAULT_CADENCE: CadenceState = {
  mode: "off",
  time: "02:00",
  weekdays: ["Mon"],
  intervalDays: 3,
  cron: "",
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
    case "cron":
      // The raw expression IS the cadence string — the backend's ParseCadence
      // accepts any 5-field cron verbatim. Callers only emit this when the
      // expression validates (see CadenceBuilder's update()).
      return s.cron.trim();
  }
}

// prettyTime turns "HH:MM" into "H:MM" (drops a leading zero on the hour), e.g.
// "04:00" -> "4:00".
function prettyTime(hhmm: string): string {
  const m = /^(\d{1,2}):(\d{2})$/.exec(hhmm);
  return m ? `${parseInt(m[1], 10)}:${m[2]}` : hhmm;
}

// WEEKDAY_OFFSET maps the stored English abbreviation to a day-of-month in the
// first week of January 2024. 2024-01-01 is a MONDAY, so day 1 = Mon … day 7 =
// Sun. (The previous reference, 2024's predecessor, started on a Sunday, which
// shifted every label back by one — a Sunday schedule read as "Sat".)
const WEEKDAY_OFFSET: Record<string, number> = { Mon: 1, Tue: 2, Wed: 3, Thu: 4, Fri: 5, Sat: 6, Sun: 7 };

// localizedWeekday renders a stored English 3-letter weekday in the given
// language's short form via Intl (e.g. "Mon" -> "Mo." in de), falling back to the
// stored abbreviation.
function localizedWeekday(abbr: string, lang: string): string {
  const off = WEEKDAY_OFFSET[abbr];
  if (!off) return abbr;
  try {
    return new Intl.DateTimeFormat(lang, { weekday: "short" }).format(new Date(Date.UTC(2024, 0, off)));
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
    case "cron":
      // A raw expression has no natural prose form — show it verbatim with a
      // "cron:" prefix so schedule summaries stay recognizable.
      return t("cadence.fmtCron").replace("{expr}", s.cron);
  }
}

/** Parse a stored cadence string back into builder state. */
export function parseCadenceString(raw: string): CadenceState {
  const s = (raw ?? "").trim();
  if (!s || s === "off") return { ...DEFAULT_CADENCE, mode: "off" };

  const dailyM = /^daily\s+(\d{1,2}:\d{2})$/.exec(s);
  if (dailyM) return { ...DEFAULT_CADENCE, mode: "daily", time: dailyM[1] };

  const weeklyM = /^weekly\s+([\w,]+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (weeklyM) {
    const days = weeklyM[1]
      .split(",")
      .map((d) => d.trim())
      .map((d) => d.charAt(0).toUpperCase() + d.slice(1).toLowerCase());
    return { ...DEFAULT_CADENCE, mode: "weekly", time: weeklyM[2], weekdays: days };
  }

  const everyNM = /^everyN\s+(\d+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (everyNM) {
    return { ...DEFAULT_CADENCE, mode: "everyN", time: everyNM[2], intervalDays: parseInt(everyNM[1], 10) };
  }

  // Anything else is a raw cron cadence (the backend accepts any 5-field cron
  // verbatim, #107). Preserve the string EXACTLY — mapping it to "off" here
  // would silently destroy a stored schedule the moment the builder re-emits.
  // Even a string our validator dislikes is kept: the cron editor then shows
  // it with an inline error instead of eating it.
  return { ...DEFAULT_CADENCE, mode: "cron", cron: s };
}

// CRON_DOW maps the builder's stored weekday abbreviations to cron numbers
// (Sun=0 … Sat=6) for the switch-to-cron prefill.
const CRON_DOW: Record<string, number> = { Sun: 0, Mon: 1, Tue: 2, Wed: 3, Thu: 4, Fri: 5, Sat: 6 };

// cronFromState derives an equivalent cron expression from the current builder
// state, so switching to the Cron pill starts from the schedule the user
// already had (daily 02:00 -> "0 2 * * *") instead of an empty invalid field.
function cronFromState(s: CadenceState): string {
  const m = /^(\d{1,2}):(\d{2})$/.exec(s.time);
  const hour = m ? parseInt(m[1], 10) : 2;
  const minute = m ? parseInt(m[2], 10) : 0;
  if (s.mode === "weekly") {
    const days = WEEKDAYS.filter((d) => s.weekdays.includes(d))
      .map((d) => CRON_DOW[d])
      .sort((a, b) => a - b);
    if (days.length > 0) return `${minute} ${hour} * * ${days.join(",")}`;
  }
  return `${minute} ${hour} * * *`;
}

// CRON_EXAMPLES are the clickable quick-help rows under the cron input. The
// expressions are universal cron syntax (never translated); the descriptions
// come from i18n.
const CRON_EXAMPLES = [
  { expr: "0 */6 * * *", key: "cadence.cronExEvery6h" },
  { expr: "30 2 * * 1-5", key: "cadence.cronExWeekdays" },
  { expr: "0 3 1 * *", key: "cadence.cronExMonthly" },
] as const;

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
      let next = { ...prev, ...patch };
      // Entering cron mode with no expression yet: prefill the equivalent of
      // the schedule the user was on, so the field starts valid and editable.
      if (patch.mode === "cron" && next.cron.trim() === "") {
        next = { ...next, cron: cronFromState(prev) };
      }
      // Never emit a broken cadence string: while the cron text is invalid the
      // parent keeps the last good value and the editor shows an inline error.
      if (next.mode !== "cron" || isValidCronExpression(next.cron)) {
        onChange(buildCadenceString(next));
      }
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
    "rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-2.5 py-1.5 focus:outline-hidden focus:border-statusInfoSolid disabled:opacity-50";

  return (
    <div className={`flex flex-col gap-3 ${disabled ? "opacity-50 pointer-events-none" : ""}`}>
      <span className="text-xs text-carbon-textSub font-medium">{label}</span>

      {/* Mode pills */}
      <div className="flex flex-wrap gap-2">
        {(["off", "daily", "weekly", "everyN", "cron"] as CadenceMode[]).map((m) => (
          <button
            key={m}
            onClick={() => update({ mode: m })}
            className={`rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
              state.mode === m
                ? "bg-carbon-surface3 text-carbon-text"
                : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
            }`}
          >
            {m === "off"
              ? t("cadence.off")
              : m === "daily"
                ? t("cadence.daily")
                : m === "weekly"
                  ? t("cadence.weekly")
                  : m === "everyN"
                    ? t("cadence.everyN")
                    : t("cadence.cron")}
          </button>
        ))}
      </div>

      {/* Time picker — shown for all non-off modes except cron (the expression carries its own times) */}
      {state.mode !== "off" && state.mode !== "cron" && (
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
                    ? "bg-statusOkBg text-statusOk border border-statusOkBorder"
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

      {/* Cron: raw 5-field expression with validation, next-fire preview and
          clickable examples (#107). The backend stays the validity authority —
          this only pre-checks the grammar it is known to accept. */}
      {state.mode === "cron" && (
        <CronEditor
          value={state.cron}
          inputCls={inputCls}
          onChange={(expr) => update({ cron: expr })}
          t={t}
          lang={lang}
        />
      )}

      {/* Preview — human-readable, localized (e.g. "jeden 3. Tag um 4:00 Uhr").
          Cron renders its own richer preview inside CronEditor. */}
      {state.mode !== "off" && state.mode !== "cron" && (
        <p className="text-xs text-carbon-textSub">
          {formatCadence(buildCadenceString(state), t, lang)}
        </p>
      )}
    </div>
  );
}

// formatFireTime renders one upcoming fire as a short localized local
// datetime (e.g. "24 Jul 2026, 18:00"), falling back to the default
// locale rendering if Intl rejects the language tag.
function formatFireTime(d: Date, lang: string): string {
  try {
    return new Intl.DateTimeFormat(lang, { dateStyle: "medium", timeStyle: "short" }).format(d);
  } catch {
    return d.toLocaleString();
  }
}

// CronEditor is the cron-mode body: expression input, inline validity error,
// a "next fires" preview computed client-side, and clickable example rows.
function CronEditor({
  value,
  inputCls,
  onChange,
  t,
  lang,
}: {
  value: string;
  inputCls: string;
  onChange: (expr: string) => void;
  t: CadenceT;
  lang: string;
}) {
  const trimmed = value.trim();
  const valid = trimmed !== "" && isValidCronExpression(trimmed);
  // The preview must never be WRONG: nextCronFires only evaluates the grammar
  // subset it fully understands, and when it cannot produce at least two fire
  // times we degrade to a plain "valid expression" note instead of guessing.
  const fires = valid ? nextCronFires(trimmed, 3) : null;

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-3">
        <label className="text-xs text-carbon-textMuted w-16">{t("cadence.cronExpr")}</label>
        <input
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={t("cadence.cronPlaceholder")}
          spellCheck={false}
          autoCapitalize="off"
          autoCorrect="off"
          className={`${inputCls} font-mono w-56 max-w-full`}
        />
      </div>

      {!valid ? (
        <p className="text-xs text-statusFail">{t("cadence.cronInvalid")}</p>
      ) : fires && fires.length >= 2 ? (
        <p className="text-xs text-carbon-textSub">
          {t("cadence.cronNext")
            .replace("{first}", formatFireTime(fires[0], lang))
            .replace("{rest}", fires.slice(1).map((d) => formatFireTime(d, lang)).join(", "))}
        </p>
      ) : (
        <p className="text-xs text-carbon-textSub">{t("cadence.cronValid")}</p>
      )}

      {/* Quick help — clickable examples that fill the input. */}
      <div className="flex flex-col gap-1">
        <span className="text-xs text-carbon-textMuted">{t("cadence.cronExamples")}</span>
        {CRON_EXAMPLES.map((ex) => (
          <button
            key={ex.expr}
            onClick={() => onChange(ex.expr)}
            className="self-start rounded px-1.5 py-0.5 text-xs text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors"
          >
            <code className="font-mono text-carbon-text">{ex.expr}</code>
            <span className="ml-2">{t(ex.key)}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
