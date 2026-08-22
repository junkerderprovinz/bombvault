import { useState } from "react";
import { useT } from "../lib/i18n";
import { CadenceBuilder, EXACT_CADENCE_MODES, formatCadence } from "./CadenceBuilder";
import { Badge } from "./Badge";
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
// A single save persists the current value via the supplied PATCH function.
// ---------------------------------------------------------------------------

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
  const [busy, setBusy] = useState(false);

  // A non-empty, non-"off" cadence is an active override; anything else means the
  // item follows its domain schedule.
  const active = value.trim() !== "" && value.trim() !== "off";
  const summary = active ? formatCadence(value, t, lang) : t("schedule.overrideUsesDefault");

  async function handleSave() {
    setBusy(true);
    try {
      // Normalize "off" to an empty override: the backend treats "off" as a valid
      // cadence, but for a per-item override the intent of "off" is "no override".
      const toStore = value.trim() === "off" ? "" : value.trim();
      const res = await onSave(toStore);
      if (res.ok) {
        push(t("schedule.overrideSaved"), "success");
      } else {
        push(res.error ?? t("schedule.updateFailed"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("schedule.updateFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("schedule.overrideTitle")}:</span>
        <span className={`text-xs ${active ? "text-carbon-textSub" : "text-carbon-textMuted italic"}`}>
          {summary}
        </span>
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
            modes={EXACT_CADENCE_MODES}
            onChange={setValue}
          />
          <p className="text-xs text-carbon-textMuted">{t("schedule.overrideHint")}</p>
          <div className="flex items-center gap-3">
            <button
              onClick={() => void handleSave()}
              disabled={busy}
              className="rounded-control bg-accent text-accentContrast px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-50"
            >
              {busy ? t("common.saving") : t("schedule.overrideSave")}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
