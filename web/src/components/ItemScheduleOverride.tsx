import { useEffect, useRef, useState } from "react";
import { useT } from "../lib/i18n";
import { CadenceBuilder, EXACT_CADENCE_MODES, formatCadence } from "./CadenceBuilder";
import { Badge } from "./Badge";
import { ScheduleBadge } from "./ScheduleBadge";
import { useToast } from "../lib/toast";

// ItemScheduleOverride is a container's or VM's optional schedule override. An
// empty override ("" or "off") follows the domain schedule. It starts collapsed
// to one line so a long member list stays compact, and saves as you type, like
// the other cadence fields.

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
  // The value a pending debounce would write, so unmount can flush it rather
  // than drop it. Cleared when the timer fires normally.
  const pendingValue = useRef<string | null>(null);

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

  function handleChange(v: string) {
    setValue(v);
    if (debounceTimer.current) clearTimeout(debounceTimer.current);
    pendingValue.current = v;
    debounceTimer.current = setTimeout(() => {
      pendingValue.current = null;
      void persist(v);
    }, DEBOUNCE_MS);
  }

  // Flush a pending write on unmount: the debounce is the only thing that saves
  // a per-item cadence, so switching tab within 800ms of an edit would lose it.
  // persist() does not touch this component's state, so it is safe here.
  useEffect(() => {
    return () => {
      if (debounceTimer.current) {
        clearTimeout(debounceTimer.current);
        if (pendingValue.current !== null) void persist(pendingValue.current);
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- unmount-only; persist is stable for this row's lifetime
  }, []);

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("schedule.overrideTitle")}:</span>
        {/* The label is formatCadence's prose rather than the badge grammar
            of cadenceLabel: an absent override means the item inherits its
            domain schedule, which is not the same as "nothing scheduled". */}
        <ScheduleBadge status={active ? "active" : "off"} label={summary} />
        <Badge as="button" onClick={() => setOpen((o) => !o)} tone="neutral" size="small">
          {open ? t("common.close") : t("schedule.overrideEdit")}
        </Badge>
      </div>

      {open && (
        <div className="rounded-card bg-carbon-surface2 p-3 flex flex-col gap-3">
          <CadenceBuilder
            label={`${t("schedule.overrideTitle")}: ${name}`}
            value={value}
            // A per-item override has no last-run gate of its own, so the
            // backend refuses everyN here (SetScheduleCadence and
            // SetVMScheduleCadence in internal/api/service.go).
            modes={EXACT_CADENCE_MODES}
            onChange={handleChange}
          />
          <p className="text-xs text-carbon-textMuted">{t("schedule.overrideHint")}</p>
        </div>
      )}
    </div>
  );
}
