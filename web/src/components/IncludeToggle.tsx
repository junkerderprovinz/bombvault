import { useEffect, useState } from "react";
import { setInclude, type OkEnvelope } from "../lib/api";
import { useT } from "../lib/i18n";
import { ToggleRow } from "../pages/settings/shared";
import { useToast } from "../lib/toast";

interface IncludeToggleProps {
  name: string;
  initial: boolean;
  /** The request that stores the switch. Containers by default; the VMs page
   *  passes setVMInclude, so both pages render this one control. */
  save?: (name: string, include: boolean) => Promise<OkEnvelope>;
}

export function IncludeToggle({ name, initial, save = setInclude }: IncludeToggleProps) {
  const { t } = useT();
  const { push } = useToast();
  const [enabled, setEnabled] = useState(initial);
  const [busy, setBusy] = useState(false);
  // Bumped on failure so ToggleRow shakes the switch (see its `shakeNonce`).
  const [shake, setShake] = useState(0);

  // Rows are keyed by name and do not remount, so a fresh value from the parent
  // (after "Include all in schedule", say) has to be copied in.
  useEffect(() => setEnabled(initial), [initial]);

  async function handleChange(next: boolean) {
    setBusy(true);
    try {
      const res = await save(name, next);
      if (res.ok) {
        setEnabled(next);
      } else {
        push(res.error ?? t("schedule.updateFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("schedule.updateFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  return (
    <ToggleRow
      label={t("containers.includeInSchedule")}
      checked={enabled}
      onChange={(next) => void handleChange(next)}
      disabled={busy}
      shakeNonce={shake}
    />
  );
}
