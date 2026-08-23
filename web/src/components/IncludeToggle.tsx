import { useEffect, useState } from "react";
import { setInclude } from "../lib/api";
import { useT } from "../lib/i18n";
import { Toggle } from "./Toggle";
import { useToast } from "../lib/toast";

interface IncludeToggleProps {
  name: string;
  initial: boolean;
}

export function IncludeToggle({ name, initial }: IncludeToggleProps) {
  const { t } = useT();
  const { push } = useToast();
  const [enabled, setEnabled] = useState(initial);
  const [busy, setBusy] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, fifth escalation:
  // "Warum muss ich dich immer wieder extra dran erinnern? Kannst du das
  // jetzt nicht einfach selbst immer machen?" — "Wenn etwas fehlschlägt soll
  // der Toggle/Button kurz zittern. Systemweit!!"): a bumped nonce, keyed
  // onto the Toggle exactly like VMs.tsx's own VMIncludeToggle — the same
  // shared control, same failure path, that already carries this — see
  // Settings.tsx's ToggleRow for the fuller "why a nonce, not a boolean"
  // reasoning. This component had drifted from its own twin by never
  // gaining it.
  const [shake, setShake] = useState(0);

  // Re-seed when the parent passes a fresh value (e.g. after "Include all in
  // schedule" reloads the list). Rows are keyed by name and do not remount, so
  // without this the toggle would keep showing its stale pre-bulk state.
  useEffect(() => setEnabled(initial), [initial]);

  async function handleChange(next: boolean) {
    setBusy(true);
    try {
      const res = await setInclude(name, next);
      if (res.ok) {
        setEnabled(next);
      } else {
        // Server returned a graceful failure — revert and toast the message.
        push(res.error ?? t("schedule.updateFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      // Network error — revert and toast a brief message.
      push(err instanceof Error ? err.message : t("schedule.updateFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <Toggle
        key={shake}
        hideLabel
        label={t("containers.includeInSchedule")}
        checked={enabled}
        onChange={(next) => void handleChange(next)}
        disabled={busy}
        className={shake ? "glim-shake" : undefined}
      />
    </div>
  );
}
