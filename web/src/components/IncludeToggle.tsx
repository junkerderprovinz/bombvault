import { useState } from "react";
import { setInclude } from "../lib/api";

interface IncludeToggleProps {
  name: string;
  initial: boolean;
}

export function IncludeToggle({ name, initial }: IncludeToggleProps) {
  const [enabled, setEnabled] = useState(initial);
  const [busy, setBusy] = useState(false);

  async function handleChange(next: boolean) {
    setBusy(true);
    try {
      await setInclude(name, next);
      setEnabled(next);
    } catch {
      // revert on network error — the value stays unchanged
    } finally {
      setBusy(false);
    }
  }

  return (
    <button
      role="switch"
      aria-checked={enabled}
      disabled={busy}
      onClick={() => void handleChange(!enabled)}
      title="Include in schedule"
      className={`relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#78a9ff] disabled:opacity-50 ${
        enabled ? "bg-[#6fdc8c]" : "bg-carbon-surface3"
      }`}
    >
      <span
        className={`inline-block h-3.5 w-3.5 rounded-full bg-[#161616] transition-transform ${
          enabled ? "translate-x-[18px]" : "translate-x-[3px]"
        }`}
      />
    </button>
  );
}
