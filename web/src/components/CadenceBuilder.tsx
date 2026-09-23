import { useEffect, useState } from "react";
import { useT } from "../lib/i18n";
import { isValidCronExpression, nextCronFires } from "../lib/cron";
import { Selector } from "./Selector";
import { NumberField } from "./NumberField";
import { TimePicker } from "./TimePicker";
import { tLtr } from "../lib/ltrFragments";

export type CadenceMode = "off" | "daily" | "weekly" | "everyN" | "cron";

/** Every mode the grammar knows, in the order the pills are rendered. */
export const ALL_CADENCE_MODES: CadenceMode[] = ["off", "daily", "weekly", "everyN", "cron"];

/**
 * EXACT_CADENCE_MODES leaves out everyN, for schedules that have no last-run
 * record to count an interval from. The backend refuses everyN for them
 * (SetScheduleCadence and SetVMScheduleCadence in internal/api/service.go),
 * since it would fire daily.
 */
export const EXACT_CADENCE_MODES: CadenceMode[] = ["off", "daily", "weekly", "cron"];

export const WEEKDAYS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"] as const;

export interface CadenceState {
  mode: CadenceMode;
  time: string; // "HH:MM"
  weekdays: string[]; // subset of WEEKDAYS, for weekly
  intervalDays: number; // for everyN
  cron: string; // raw 5-field cron expression, for cron
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
      // The backend's ParseCadence accepts any 5-field cron verbatim.
      // CadenceBuilder's update() only emits this when the expression validates.
      return s.cron.trim();
  }
}

// prettyTime drops the leading zero of the hour: "04:00" -> "4:00".
function prettyTime(hhmm: string): string {
  const m = /^(\d{1,2}):(\d{2})$/.exec(hhmm);
  return m ? `${parseInt(m[1], 10)}:${m[2]}` : hhmm;
}

// WEEKDAY_OFFSET maps the stored English abbreviation to a day of January 2024,
// which starts on a Monday.
const WEEKDAY_OFFSET: Record<string, number> = { Mon: 1, Tue: 2, Wed: 3, Thu: 4, Fri: 5, Sat: 6, Sun: 7 };

// localizedWeekday renders a stored English 3-letter weekday in the given
// language's short form via Intl (e.g. "Mon" -> "Mo." in de), falling back to the
// stored abbreviation. The reference date is midnight UTC, so it is formatted in
// UTC; in the viewer's zone it would fall on the previous day west of UTC.
function localizedWeekday(abbr: string, lang: string): string {
  const off = WEEKDAY_OFFSET[abbr];
  if (!off) return abbr;
  try {
    return new Intl.DateTimeFormat(lang, { weekday: "short", timeZone: "UTC" }).format(
      new Date(Date.UTC(2024, 0, off))
    );
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
      // An interval of 1 is just daily.
      if (s.intervalDays <= 1) return t("cadence.fmtDaily").replace("{time}", time);
      return t("cadence.fmtEveryN", s.intervalDays).replace("{time}", time);
    case "cron":
      // A raw expression has no prose form, so it is shown verbatim.
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

  // Anything else is kept verbatim as a raw cron cadence, so re-emitting it
  // never destroys a stored schedule. A string the validator rejects is kept
  // too; the cron editor shows it with an inline error.
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

/** Edits a stored cadence string with mode pills and the fields each mode needs. */
export function CadenceBuilder({
  label,
  value,
  disabled,
  modes,
  onChange,
  hueIndex,
}: {
  label: string;
  value: string;
  disabled?: boolean;
  /** Modes this call site offers. Defaults to ALL_CADENCE_MODES. */
  modes?: CadenceMode[];
  onChange: (v: string) => void;
  /** Rainbow position for the TimePicker inside. Callers pass the `hueIndex`
   *  of the Card around them, so the picker takes the card's colour. */
  hueIndex?: number;
}) {
  const { t, lang } = useT();
  const [state, setState] = useState<CadenceState>(() => parseCadenceString(value));

  const allowed = modes ?? ALL_CADENCE_MODES;
  // A stored mode outside `allowed`, such as an everyN value from an import,
  // keeps its pill so it is displayed rather than rewritten. The pill goes once
  // the user picks another mode.
  const offered = ALL_CADENCE_MODES.filter((m) => allowed.includes(m) || m === state.mode);

  // The stored value can change from outside, for example once settings load.
  useEffect(() => {
    setState(parseCadenceString(value));
  }, [value]);

  // update() builds the next state from this render's `state` rather than in a
  // setState updater, because it calls the parent's onChange and React runs
  // updaters during render. Every caller is a discrete user event, so `state`
  // is current.
  function update(patch: Partial<CadenceState>) {
    let next = { ...state, ...patch };
    // Entering cron mode with no expression yet: prefill the equivalent of
    // the schedule the user was on, so the field starts valid and editable.
    if (patch.mode === "cron" && next.cron.trim() === "") {
      next = { ...next, cron: cronFromState(state) };
    }
    setState(next);
    // Never emit a broken cadence string: while the cron text is invalid the
    // parent keeps the last good value and the editor shows an inline error.
    if (next.mode !== "cron" || isValidCronExpression(next.cron)) {
      onChange(buildCadenceString(next));
    }
  }

  function toggleWeekday(day: string) {
    const current = state.weekdays;
    const next = current.includes(day)
      ? current.filter((d) => d !== day)
      : [...current, day];
    if (next.length === 0) return;
    update({ weekdays: next });
  }

  const inputCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-2.5 py-1.5 glim-field-focus-well disabled:opacity-50";

  return (
    // The fieldset disables every nested control natively. Opacity on the
    // container would composite the whole subtree, so each control dims itself
    // with `disabled:opacity-50`, and plain text, which fieldset[disabled] does
    // not reach, uses `group-disabled:opacity-50`.
    <fieldset disabled={disabled} className="group flex min-w-0 flex-col gap-3 border-0 m-0 p-0">
      {/* The legend is the fieldset's accessible name. It is visually hidden
          because every caller's Card title already shows the same word. */}
      <legend className="sr-only">
        {label}
      </legend>

      {/* `variant="well"` without `equalWidth` is the small scale of the
          grooved selector; see Selector.tsx. */}
      <Selector
        items={offered.map((m) => ({
          id: m,
          label:
            m === "off"
              ? t("cadence.off")
              : m === "daily"
                ? t("cadence.daily")
                : m === "weekly"
                  ? t("cadence.weekly")
                  : m === "everyN"
                    ? t("cadence.everyN")
                    : t("cadence.cron"),
        }))}
        label={label}
        select="one"
        active={state.mode}
        onChange={(id) => update({ mode: id as CadenceMode })}
        variant="well"
      />

      {/* Explains a missing Every N days, which a user may know from another
          card. Keyed off `allowed` rather than `offered`, so it also shows
          while a stored everyN value is displayed. */}
      {!allowed.includes("everyN") && (
        <p className="text-xs text-carbon-textMuted group-disabled:opacity-50">{t("cadence.everyNUnavailable")}</p>
      )}

      {/* A cron expression carries its own times. */}
      {state.mode !== "off" && state.mode !== "cron" && (
        <div className="flex items-center gap-3">
          <label className="text-xs text-carbon-textMuted w-16 group-disabled:opacity-50">{t("cadence.time")}</label>
          <TimePicker
            value={state.time}
            onChange={(time) => update({ time })}
            label={t("cadence.time")}
            hueIndex={hueIndex}
          />
        </div>
      )}

      {/* Same variant as the mode pills, so the two rows read as one family. */}
      {state.mode === "weekly" && (
        <div className="flex items-center gap-2 flex-wrap">
          <label className="text-xs text-carbon-textMuted w-16 group-disabled:opacity-50">{t("cadence.days")}</label>
          <Selector
            items={WEEKDAYS.map((d) => ({ id: d, label: d }))}
            label={t("cadence.days")}
            select="many"
            active={new Set(state.weekdays)}
            onChange={toggleWeekday}
            size="sm"
            variant="well"
          />
        </div>
      )}

      {state.mode === "everyN" && (
        <div className="flex items-center gap-3">
          <label className="text-xs text-carbon-textMuted w-16 group-disabled:opacity-50">{t("cadence.every")}</label>
          <NumberField
            min={1}
            value={state.intervalDays}
            onChange={(e) => {
              const n = parseInt(e.target.value, 10);
              if (!isNaN(n) && n >= 1) update({ intervalDays: n });
            }}
            className={`${inputCls} w-20`}
          />
          <span className="text-xs text-carbon-textMuted group-disabled:opacity-50">{t("cadence.daysUnit")}</span>
        </div>
      )}

      {/* The backend decides validity; this only pre-checks the grammar it
          accepts. */}
      {state.mode === "cron" && (
        <CronEditor
          value={state.cron}
          inputCls={inputCls}
          onChange={(expr) => update({ cron: expr })}
          t={t}
          lang={lang}
        />
      )}

      {/* No prose preview: every caller shows the cadence in a ScheduleRow
          badge above the card. Only cron lists its next fire times. */}
    </fieldset>
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
  // nextCronFires only evaluates the grammar subset it fully understands. With
  // fewer than two fire times the editor says "valid expression" rather than
  // guessing.
  const fires = valid ? nextCronFires(trimmed, 3) : null;

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-3">
        <label className="text-xs text-carbon-textMuted w-16 group-disabled:opacity-50">{t("cadence.cronExpr")}</label>
        <input
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={t("cadence.cronPlaceholder")}
          spellCheck={false}
          autoCapitalize="off"
          autoCorrect="off"
          dir="ltr"
          className={`${inputCls} font-mono w-56 max-w-full text-start`}
        />
      </div>

      {!valid ? (
        <p className="text-xs text-statusFail group-disabled:opacity-50">{tLtr(t, "cadence.cronInvalid")}</p>
      ) : fires && fires.length >= 2 ? (
        <p className="text-xs text-carbon-textSub group-disabled:opacity-50">
          {t("cadence.cronNext")
            .replace("{first}", formatFireTime(fires[0], lang))
            .replace("{rest}", fires.slice(1).map((d) => formatFireTime(d, lang)).join(", "))}
        </p>
      ) : (
        <p className="text-xs text-carbon-textSub group-disabled:opacity-50">{t("cadence.cronValid")}</p>
      )}

      <div className="flex flex-col gap-1">
        <span className="text-xs text-carbon-textMuted group-disabled:opacity-50">{t("cadence.cronExamples")}</span>
        {CRON_EXAMPLES.map((ex) => (
          <button
            key={ex.expr}
            onClick={() => onChange(ex.expr)}
            className="self-start rounded-control px-1.5 py-0.5 text-xs text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50"
          >
            <code dir="ltr" className="font-mono text-carbon-text text-start">{ex.expr}</code>
            <span className="ms-2">{t(ex.key)}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
