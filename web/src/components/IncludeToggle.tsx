import { useEffect, useState } from "react";
import { setInclude } from "../lib/api";

interface IncludeToggleProps {
  name: string;
  initial: boolean;
}

export function IncludeToggle({ name, initial }: IncludeToggleProps) {
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
        setError(res.error ?? "Failed to update schedule");
      }
    } catch (err) {
      // Network error — revert and show a brief message.
      setError(err instanceof Error ? err.message : "Failed to update schedule");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <button
        role="switch"
        aria-label="Include in schedule"
        aria-checked={enabled}
        disabled={busy}
        onClick={() => void handleChange(!enabled)}
        title="Include in schedule"
        className={`relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#78a9ff] disabled:opacity-50 ${
          enabled ? "bg-accent" : "bg-carbon-surface3"
        }`}
      >
        <span
          className={`inline-block h-3.5 w-3.5 rounded-full bg-carbon-background transition-transform ${
            enabled ? "translate-x-[18px]" : "translate-x-[3px]"
          }`}
        />
      </button>
      {error && (
        <span className="text-xs text-[#ff8389] max-w-[12rem] text-right leading-tight">
          {error}
        </span>
      )}
    </div>
  );
}
