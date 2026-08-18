import { useEffect, useState } from "react";
import { setInclude } from "../lib/api";
import { useT } from "../lib/i18n";
import { Toggle } from "./Toggle";

interface IncludeToggleProps {
  name: string;
  initial: boolean;
}

export function IncludeToggle({ name, initial }: IncludeToggleProps) {
  const { t } = useT();
  const [enabled, setEnabled] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Re-seed when the parent passes a fresh value (e.g. after "Include all in
  // schedule" reloads the list). Rows are keyed by name and do not remount, so
  // without this the toggle would keep showing its stale pre-bulk state.
  useEffect(() => setEnabled(initial), [initial]);

  async function handleChange(next: boolean) {
    setBusy(true);
    setError(null);
    try {
      const res = await setInclude(name, next);
      if (res.ok) {
        setEnabled(next);
      } else {
        // Server returned a graceful failure — revert and show the message.
        setError(res.error ?? t("schedule.updateFailed"));
      }
    } catch (err) {
      // Network error — revert and show a brief message.
      setError(err instanceof Error ? err.message : t("schedule.updateFailed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <Toggle
        hideLabel
        label={t("containers.includeInSchedule")}
        checked={enabled}
        onChange={(next) => void handleChange(next)}
        disabled={busy}
      />
      {error && (
        <span className="text-xs text-statusFail max-w-48 text-right leading-tight">
          {error}
        </span>
      )}
    </div>
  );
}
