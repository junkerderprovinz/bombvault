// RestoreChecksSection, lifted out of Settings.tsx ([337]).
//
// A MOVE, not a rewrite: the component is byte-identical to what stood
// in Settings.tsx, and it was already module-level and prop-driven, so
// nothing crosses a new seam. See that file's own note for why the cut
// stops here rather than continuing into SettingsPage itself.
import type { Settings } from "../../lib/api";
import { CadenceBuilder } from "../../components/CadenceBuilder";
import { Card, ToggleRow } from "../settings/shared";
import { NumberField } from "../../components/NumberField";
import { ScheduleRow } from "../../components/ScheduleBadge";
import { useT } from "../../lib/i18n";

// Domain section — Restore checks (scheduled restore-verification drills).
// The drill schedule sits beside the backup schedules; always visible.
export function RestoreChecksSection({
  settings,
  update,
  busy,
  shake,
  pulse,
  t,
  hueIndex,
}: {
  settings: Settings;
  update: (patch: Partial<Settings>) => void;
  /** Task 5 auto-save (no Speichern button on this tab anymore): busy/shake
   *  feedback for this Card's two plain booleans, keyed the same way
   *  domainToggleBusy/domainToggleShake and mergedFieldBusy/mergedFieldShake
   *  already are elsewhere — see SettingsPage's own autoSaveScheduleField. */
  busy?: Partial<Record<"drillsEnabled" | "offsiteDrillsEnabled", boolean>>;
  shake?: Partial<Record<"drillsEnabled" | "offsiteDrillsEnabled", number>>;
  /** Confirmation-pulse (GlimStone motion-engine animation 2) — same shape
   *  as `shake` above, opposite outcome. SettingsPage passes its own shared
   *  `fieldPulse` map straight through (see that state's own declaration
   *  comment next to save()) — this narrower prop type is still satisfied
   *  because `fieldPulse` is keyed by the full `keyof Settings`, a superset
   *  of the two keys this Card actually reads. */
  pulse?: Partial<Record<"drillsEnabled" | "offsiteDrillsEnabled", number>>;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  return (
    <Card title={t("verify.auto")} hint={t("verify.hint")} hueIndex={hueIndex}>
      {/* Task 4 (jdp, live-review: "Bei erstem Toggle bitte 'Automatische
          Restore-Prüfungen' hinschreiben") — `hideLabel` removed: an earlier
          round hid this row's own caption on the reasoning that the Card's
          own title above already says the same thing (the same
          single-purpose-Card pattern the master Regenbogen-Modus toggle used
          too), but jdp reversed that exact pattern there as well and wants
          the label visible directly on the toggle here too. */}
      <ToggleRow
        label={t("verify.auto")}
        checked={settings.drillsEnabled}
        onChange={(v) => update({ drillsEnabled: v })}
        disabled={busy?.drillsEnabled}
        shakeNonce={shake?.drillsEnabled}
        pulseNonce={pulse?.drillsEnabled}
      />
      {/* Sub-toggle: only meaningful while scheduled drills are on. ToggleRow
          itself dims its switch AND its caption/description together — no
          wrapping container opacity needed here. */}
      <ToggleRow
        label={t("settings.offsiteDrills")}
        hint={t("settings.offsiteDrillsHelp")}
        checked={settings.offsiteDrillsEnabled}
        disabled={!settings.drillsEnabled || busy?.offsiteDrillsEnabled}
        onChange={(v) => update({ offsiteDrillsEnabled: v })}
        shakeNonce={shake?.offsiteDrillsEnabled}
        pulseNonce={pulse?.offsiteDrillsEnabled}
      />
      {/* Resolved-schedule badge — NEW this round. This Card was one of the
          three cadence editors with nothing above it, so CadenceBuilder's own
          inline preview paragraph was the only place its resolved schedule
          was shown; deleting that paragraph (see CadenceBuilder.tsx) without
          adding this row would have lost information rather than removed a
          duplicate. `enabled` is wired to `drillsEnabled` because THIS card's
          on/off lives in a separate toggle rather than in the cadence string's
          own "off" mode — see ScheduleRow's own `enabled` doc. */}
      <ScheduleRow schedule={settings.drillsSchedule} enabled={settings.drillsEnabled} />
      {/* `hueIndex` passed straight through to the TimePicker inside (Task 3
          fix) — the SAME position as this Card's own heading notch above. */}
      <div className="rounded-card bg-carbon-surface2 p-4">
        {/* No `modes` restriction (#166): the drill pass stamps
            schedule_job_runs when it runs and the scheduler gates on that, so
            "every N days" is genuinely enforced here and the API accepts it. */}
        <CadenceBuilder
          label={t("settings.schedule")}
          value={settings.drillsSchedule}
          disabled={!settings.drillsEnabled}
          onChange={(v) => update({ drillsSchedule: v })}
          hueIndex={hueIndex}
        />
      </div>
      <label className="flex flex-col gap-1 max-w-40">
        <span className="text-xs text-carbon-textSub">{t("verify.subsetPct")}</span>
        <NumberField
          min={1}
          max={100}
          value={settings.drillsSubsetPct}
          onChange={(e) => {
            const n = parseInt(e.target.value, 10);
            const clamped = isNaN(n) ? 1 : Math.min(100, Math.max(1, n));
            update({ drillsSubsetPct: clamped });
          }}
          className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus"
        />
      </label>
    </Card>
  );
}

// Domain section — "Backup Everything": a 6th, independent pseudo-domain
// cadence that runs containers → VMs → flash → folders → self-backup in
// sequence, bracketed by a global pre/post shell hook (the post-hook is the
// dead-man's-switch ping point — see docs/superpowers/specs/
// 2026-08-20-backup-everything-design.md). It does NOT gate or replace the
// five domain schedules above.
//
// Convention pass (this branch): this card arrived from main written against
// the app as it looked BEFORE the settled UI conventions existed here, so
// every one of them had to be applied after the merge. What changed, and why
// — each is a rule someone will otherwise re-break:
//
//   * `hueIndex` — the Card had NONE, so its heading badge sat outside the
//     rainbow entirely (flat accent in all three rainbow modes) and, because
//     Card only emits `.glim-hue` when it HAS a hueIndex, so did every
//     control inside it. It is the LAST Card on the Schedules tab, so it
//     takes that tab's next `nextHue()` slot and no sibling Card's rainbow
//     position moves. The same index is threaded into CadenceBuilder, exactly
//     as the four domain sections above already do, so the TimePicker
//     popover's selected hour/minute reads as part of this card's own
//     coloured group instead of falling back to the flat accent.
//
//   * Both explanations moved into bubbles (rule 8, "explanations live in a
//     bubble, not on the page"). `everythingHint` was a permanent grey <p>
//     under the title and is now the Card's own `hint` — the same title-badge
//     InfoBubble ~20 other Cards on this page already use. `everythingHooksHint`
//     was a second permanent <p> above the hook fields and is now an
//     InfoBubble on a real `hooks.title` group label, which also gives those
//     two fields the visible heading they never had.
//
//   * The overlap warning is now CONDITIONAL instead of permanent. Its own
//     text is "if BOTH are on, each domain runs twice" — which it asserted
//     even with this cadence off, when nothing runs twice and there is
//     nothing to warn about. Gated on the overlap being real it stops being a
//     permanent explanation (which rule 8 would move into a bubble) and
//     becomes the live conditional warning rule 8 explicitly keeps VISIBLE —
//     the same shape as the tamper-schedule-inactive warning further down,
//     whose markup it already mirrors. This is not the "smart live conflict
//     detector" the design spec rejected: it is two `scheduleStatus()` reads
//     over settings this component already holds, not a cadence-intersection
//     calculation.
//
//   * The manual trigger is a flush-right square icon Badge, not a text
//     button — the same conversion Flash's and Config's own backup-now
//     buttons already received (63f53d5, f2bf15b): same IconBackupNow glyph,
//     same `shape="square" size="icon" tone="active"` recipe, same
//     `flex justify-end` wrapper, same tip priority (in-flight → label), and
//     the same terminal-state migration — the started/409/error line that sat
//     inline beside the button becomes a toast, because a 32px badge has no
//     room for it. A failure also shakes the badge, per the system-wide "a
//     failed action toasts AND shakes its button" rule.
//
//   * The hook inputs keep Containers.tsx's HooksEditor field style but spell
//     their class list out at the call site instead of hiding it in an
//     `inputCls` local. Not cosmetic: `control-reads-engine-tokens` reads
//     `className` LITERALS and deliberately skips a bare identifier
//     (lint-rules/README.md, "Known limits"), so behind that variable these
//     two controls were invisible to the guard. They happened to be
//     compliant, but nothing was checking — the exact condition the guards
//     exist to end.
//
// `ScheduleRow` above the well is also new here: every other cadence editor
// in the app renders one (ScheduleBadge.tsx's own doc says so), and this was
// the only one that did not.
//
// Exported for Settings.everythingCard.dom.test.tsx only — the same reason
// ThemeCard/AccentCard/LanguageCard are (see their own tests): the manual
// trigger fires a real, cross-domain backup orchestration, so its wiring is
// pinned against a MOCKED api client rather than by clicking it on a live
// box. Not routed and not used anywhere else.
