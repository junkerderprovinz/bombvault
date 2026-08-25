import { useEffect, useState } from "react";
import { useT } from "../lib/i18n";
import { isValidCronExpression, nextCronFires } from "../lib/cron";
import { Selector } from "./Selector";
import { TimePicker } from "./TimePicker";

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
  hueIndex,
}: {
  label: string;
  value: string;
  disabled?: boolean;
  onChange: (v: string) => void;
  /** Rainbow position for the TimePicker rendered inside (Task 3, jdp
   *  live-review: "Der Zeitpicker ist nicht im Regenbogenmodus" — the
   *  TimePicker always accepted an optional `hueIndex` but this, its one
   *  real call site, never actually passed one through, so the popover's
   *  selected-hour/-minute highlight always fell back to the flat accent
   *  even in rainbow mode). Every caller of THIS component passes the SAME
   *  `hueIndex` its own enclosing `<Card hueIndex={...}>` already receives
   *  (see each call site in Settings.tsx), so the TimePicker inside reads
   *  as part of the same coloured group as the Card around it — the exact
   *  "a card and its own action button read as one coloured group" pattern
   *  SaveBar's own `hueIndex` doc already established for this file. */
  hueIndex?: number;
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
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-2.5 py-1.5 bv-field-focus-well disabled:opacity-50";

  return (
    // A <fieldset> — not opacity — carries the disabled state: it natively
    // disables every nested button/input (they match the :disabled CSS
    // pseudo-class without each one needing its own `disabled` prop wired
    // through), so no pointer-events-none hack is needed either. Rule 15
    // rules out opacity on a CONTAINER (it composites the whole subtree);
    // each interactive element below dims itself individually instead via
    // its own `disabled:opacity-50` — the same per-control pattern the
    // switch/button controls elsewhere in this app already use.
    //
    // `group` + `group-disabled:opacity-50` extends that same per-element
    // dimming to the plain text nodes here (the <legend>, sub-labels, and
    // preview/error text) — they are not "listed" form elements, so
    // fieldset[disabled] alone leaves them at full brightness next to the
    // now-dimmed controls. The selector is a plain CSS descendant match, so
    // it also reaches CronEditor's own text below without threading a
    // `disabled` prop through that child.
    <fieldset disabled={disabled} className="group flex min-w-0 flex-col gap-3 border-0 m-0 p-0">
      {/* The label doubles as the fieldset's accessible name via <legend> —
          a bare leading <span> left the group unnamed in the a11y tree.
            Live-review round 5 (jdp, correcting a misread of an earlier
          request: "Den Text in die Cardtitelbadges wieder einfügen, den habe
          ich nicht gemeint. Den 'Titeltext' aus der Zeitplancard [entfernen].
          Siehe Screenshot. Das ist es, das Wort 'Container'."): every one of
          this component's callers wraps it in its own Card, and that Card's
          `title` names the exact same domain one level up (e.g. "Container")
          — this <legend> then repeated that same word again, in plain grey
          text, directly above the schedule pills. jdp's screenshot showed
          THIS plain-text repeat, not the Card's own heading badge (a
          previous round misread the complaint and removed the Card's
          `title` instead, which is reverted separately — see each
          ContainersSection/VMsSection/FlashSection/FilesSection/
          RestoreChecksSection Card in Settings.tsx).
            `sr-only` (Tailwind's built-in visually-hidden utility — clips to
          a 1px box, not `display:none`/`visibility:hidden`, so screen
          readers still read it) rather than deleting the element outright:
          a fieldset's accessible name comes from its <legend>, and nothing
          else in this component names the group (no aria-label on the
          fieldset itself) — removing this entirely would leave the fieldset
          unnamed in the a11y tree again, the exact regression this
          <legend> was originally added to fix (see this file's own commit
          history / the paragraph above). Visually hiding it keeps the
          accessible name intact for screen-reader users while sighted users
          see the word exactly once (the Card's own heading badge above),
          not twice.
            The `mb-3` spacing compensation this <legend> used to provide
          (a <legend> renders in its own out-of-flow box, never as a flex
          item, so the fieldset's own `gap-3` never reached it) is now moot:
          an `sr-only` element is clipped to a 1x1px box removed from normal
          flow-affecting layout, so it no longer displaces the row below it
          either way — verified live, no gap regression at any of this
          component's call sites. */}
      <legend className="sr-only">
        {label}
      </legend>

      {/* Mode pills — the shared Selector component (GlimStone form-engine
          Phase 2, Task 3). Disabling still comes from the ancestor
          <fieldset disabled> above, not a prop here: Selector renders real
          <button> elements, which a native fieldset already disables
          regardless of the wrapping <div> between them.
          `variant="well"` with NO `equalWidth` — the SMALL scale of the app's
          one grooved horizontal selector (round 7 escalation, jdp: "Du hast
          keinen richtigen horizontalen Selektor gemacht!" — the plain-chip-
          plus-`raised` treatment this used to carry, even with its idle fill
          bumped a shade deeper, still read as loose separate buttons next to
          the page's big pickers, not as one real Selector control). Every
          call site of THIS component wraps it in its own `rounded-card
          bg-carbon-surface2 p-4` well (see each Settings.tsx caller, and
          ItemScheduleOverride.tsx's own `p-3` variant of it) — which is
          exactly why round 7's first cut read as nothing at all here: it
          painted the groove `bg-carbon-surface2`, the literal same token as
          that wrapper, so the enclosure was invisible on every schedule
          card. The groove is `bg-carbon-surface3` now — the same depth
          `inputCls` below already puts this component's own time/number/cron
          fields at, inside this same well — so the mode row reads as a real
          nested control whether it sits in a surface2 well like this one or
          directly on a Card like NotifyCard's "on" row. The wrapper stays:
          it is what anchors the time picker, weekday row, interval field,
          cron editor and preview text as one group, and `inputCls`'s own
          `bv-field-focus-well` focus treatment is specified against a
          surface2 well — the variant, not the caller, is what moved.
            Round 8 (jdp: "Die kleinen Selektoren sollen so aussehen wie die
          grossen! Die nicht ausgewaehlten Optionen sollen kein Badge sein")
          then removed the separate "track" variant this used to name: idle
          segments are transparent against the groove here now, exactly as in
          the Theme/Shape/Motion pickers, and the only real difference left
          between the two scales is the pinned width/height — which is
          `equalWidth`, which this call site simply does not pass. See
          Selector.tsx's own file header item 6 for the full writeup. */}
      <Selector
        items={(["off", "daily", "weekly", "everyN", "cron"] as CadenceMode[]).map((m) => ({
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

      {/* Time picker — shown for all non-off modes except cron (the expression
          carries its own times). Formerly a native `<input type="time">`;
          replaced by the shared TimePicker component (GlimStone form-engine,
          new standard component — jdp, live-review: "einen schönen Stunden-
          und Minuten-Picker... damit man es nicht manuell eintippen muss").
          Same "HH:MM" string wired straight into `update({ time })` as
          before — only the input UI changed, CadenceState's own data model
          didn't. Disabling still comes from the ancestor `<fieldset
          disabled>` alone (a real `<button>` trigger, same as every other
          disabled-aware control in this fieldset), no separate `disabled`
          prop needed here. */}
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

      {/* Weekly: weekday multi-select — select="many" (toggling a day never
          replaces the others, "at least one" is still enforced by
          toggleWeekday itself, unchanged). `variant="well"`, not `raised`,
          for the same round-7 reason as the mode pills above — kept on the
          same variant so the two Selector rows inside one CadenceBuilder
          instance read as one family, not one grooved control sitting
          directly above a leftover plain-raised-chip one. Unpinned like the
          mode row (no `equalWidth`); `size="sm"` is the only thing that
          makes this the tighter of the two. */}
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

      {/* Every N days: number input */}
      {state.mode === "everyN" && (
        <div className="flex items-center gap-3">
          <label className="text-xs text-carbon-textMuted w-16 group-disabled:opacity-50">{t("cadence.every")}</label>
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
          <span className="text-xs text-carbon-textMuted group-disabled:opacity-50">{t("cadence.daysUnit")}</span>
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

      {/* The plain one-line preview that used to sit here is GONE (jdp,
          live-review, with a screenshot of a schedule card showing the green
          badge "Täglich um 02:00" above the card and the line "täglich um
          2:00 Uhr" inside it: "Bei den ganzen Zeitplänen den Text in der
          Auswahlcard entfernen. Das wird ja über der Card schon als grüner
          Badge angezeigt. Ist redundant.").
            It said the same thing as the resolved-schedule badge above the
          card, only in the sentence-cased prose grammar (`formatCadence`)
          instead of the badge's short one (`cadenceLabel`) — two renderings
          of one value, ~40px apart.
            Deleting it here is only correct because EVERY call site now has
          that badge: the four domain Cards and Selbst-Backup already did;
          Restore-Prüfungen, Wochenbericht and Wiederherstellungs-Prüfplan had
          nothing above them and got one this round; ItemScheduleOverride's own
          plain-text summary became the same badge. See ScheduleBadge.tsx
          (ScheduleRow) — the row is one shared component now precisely so a
          future cadence editor can't be added without it and quietly lose the
          only place its resolved schedule was shown.
            CRON KEEPS ITS OWN PREVIEW, deliberately: CronEditor's "next
          fires" list below shows upcoming fire TIMES ("24 Jul 2026, 18:00,
          …"), which no badge anywhere renders — genuinely more information,
          not a second copy of the same string. That is why this removal was
          scoped to the non-cron branch's paragraph only, and why `lang` is
          still threaded down into CronEditor. */}
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
  // The preview must never be WRONG: nextCronFires only evaluates the grammar
  // subset it fully understands, and when it cannot produce at least two fire
  // times we degrade to a plain "valid expression" note instead of guessing.
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
        <p className="text-xs text-statusFail group-disabled:opacity-50">{t("cadence.cronInvalid")}</p>
      ) : fires && fires.length >= 2 ? (
        <p className="text-xs text-carbon-textSub group-disabled:opacity-50">
          {t("cadence.cronNext")
            .replace("{first}", formatFireTime(fires[0], lang))
            .replace("{rest}", fires.slice(1).map((d) => formatFireTime(d, lang)).join(", "))}
        </p>
      ) : (
        <p className="text-xs text-carbon-textSub group-disabled:opacity-50">{t("cadence.cronValid")}</p>
      )}

      {/* Quick help — clickable examples that fill the input. */}
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
