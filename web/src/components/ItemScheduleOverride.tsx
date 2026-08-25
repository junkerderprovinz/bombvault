import { useEffect, useRef, useState } from "react";
import { useT } from "../lib/i18n";
import { CadenceBuilder, formatCadence } from "./CadenceBuilder";
import { Badge } from "./Badge";
import { ScheduleBadge } from "./ScheduleBadge";
import { useToast } from "../lib/toast";

// ---------------------------------------------------------------------------
// Per-item schedule override (#121)
//
// A single container's or VM's optional schedule override. It reuses the exact
// same CadenceBuilder the domain schedules use, so the grammar and editing feel
// are identical. An empty override ("" / "off") means the item follows its domain
// schedule; a concrete cadence overrides it. This control is only rendered while
// the perItemSchedules setting is on — when it is off the item lists are unchanged.
//
// The editor is collapsed by default and shows a one-line summary (the override
// cadence, or "uses the domain schedule"), so a long member list stays compact.
//
// Full-page Speichern-Button sweep (jdp, live review, emphatic: "Die Speicher-
// Buttons sollen in allen Tabs weg. Überall soll es automatisch speichern."):
// this used to hold its own manual "Übernehmen"-style Save button, found on a
// re-sweep after the rest of the Schedules tab's cadence editors (the DOMAIN-
// level Containers/VMs/Flash/Ordner Cards) had already been converted to
// auto-save via `scheduleField` in Settings.tsx — the exact same CadenceBuilder
// control, just wrapped in this collapse/expand disclosure for a potentially
// long member list. There's no genuine reason for the PER-ITEM copy to still
// batch into a click: the collapse/expand toggle is a compactness affordance,
// not a "discard my edit" one — closing the panel never resets `value` back to
// `initial`, so no cancel semantic exists here to protect (unlike
// CloudCredSetsCard's/OffsiteTargetsSection's own draft editors elsewhere,
// which keep their manual Save for exactly that reason). Debounces on the SAME
// 800ms timing as every other free-text/cadence field on this page.
// ---------------------------------------------------------------------------

const DEBOUNCE_MS = 800;

export function ItemScheduleOverride({
  name,
  initial,
  onSave,
}: {
  /** The container/VM name (only used for the accessible label). */
  name: string;
  /** The stored override cadence ("" = follows the domain schedule). */
  initial: string;
  /** Persist the override; an empty string clears it. */
  onSave: (cadence: string) => Promise<{ ok: boolean; error?: string }>;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const [open, setOpen] = useState(false);
  const [value, setValue] = useState(initial ?? "");
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // A non-empty, non-"off" cadence is an active override; anything else means the
  // item follows its domain schedule.
  const active = value.trim() !== "" && value.trim() !== "off";
  const summary = active ? formatCadence(value, t, lang) : t("schedule.overrideUsesDefault");

  async function persist(v: string) {
    // Normalize "off" to an empty override: the backend treats "off" as a valid
    // cadence, but for a per-item override the intent of "off" is "no override".
    const toStore = v.trim() === "off" ? "" : v.trim();
    try {
      const res = await onSave(toStore);
      if (res.ok) {
        push(t("schedule.overrideSaved"), "success");
      } else {
        push(res.error ?? t("schedule.updateFailed"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("schedule.updateFailed"), "fail");
    }
  }

  // handleChange — optimistic local update + debounce, same shape as every
  // other cadence field on this page (Settings.tsx's own scheduleField).
  function handleChange(v: string) {
    setValue(v);
    if (debounceTimer.current) clearTimeout(debounceTimer.current);
    debounceTimer.current = setTimeout(() => void persist(v), DEBOUNCE_MS);
  }

  // A pending debounce must not fire after this row unmounts (e.g. the
  // container list re-fetches while the user is still typing).
  useEffect(() => {
    return () => {
      if (debounceTimer.current) clearTimeout(debounceTimer.current);
    };
  }, []);

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("schedule.overrideTitle")}:</span>
        {/* The resolved override is a real ScheduleBadge now (jdp, live-review
            on the schedule cards: the cadence preview inside a CadenceBuilder
            is redundant with the badge above it — see CadenceBuilder.tsx and
            ScheduleBadge.tsx). This row already showed the resolved schedule,
            but as plain text with an italic/muted variant of its own, so it
            was the one cadence editor in the app whose summary did NOT look
            like every other cadence editor's summary. Same
            active-green/off-neutral pair the domain Cards' rows use.
              The LABEL stays `formatCadence` (CadenceBuilder's prose grammar),
            NOT the badge grammar `cadenceLabel` the ScheduleRow sites use: the
            inactive case here is not "Kein Zeitplan" but a specific sentence,
            "folgt dem Domain-Zeitplan" (schedule.overrideUsesDefault) — a
            per-item override that is absent means it INHERITS, which is a
            different statement from "nothing is scheduled", and swapping in
            the generic label would have destroyed exactly that distinction. */}
        <ScheduleBadge status={active ? "active" : "off"} label={summary} />
        {/* Task 5 (rule 13): was a plain underlined text button. */}
        <Badge as="button" onClick={() => setOpen((o) => !o)} tone="neutral" size="small">
          {open ? t("common.close") : t("schedule.overrideEdit")}
        </Badge>
      </div>

      {open && (
        <div className="rounded-card bg-carbon-surface2 p-3 flex flex-col gap-3">
          <CadenceBuilder
            label={`${t("schedule.overrideTitle")}: ${name}`}
            value={value}
            onChange={handleChange}
          />
          <p className="text-xs text-carbon-textMuted">{t("schedule.overrideHint")}</p>
        </div>
      )}
    </div>
  );
}
