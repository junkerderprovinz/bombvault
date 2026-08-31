import { useEffect, useState } from "react";
import { setInclude } from "../lib/api";
import { useT } from "../lib/i18n";
import { ToggleRow } from "../pages/settings/shared";
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
  // der Toggle/Button kurz zittern. Systemweit!!"): a bumped nonce, passed
  // straight through as ToggleRow's own `shakeNonce` (see that prop's doc for
  // why a nonce, not a boolean, and why it remounts the Toggle to replay the
  // animation).
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

  // Renders through the SAME shared ToggleRow every other row-shaped toggle
  // in the app already uses (UpdateAfterBackupRow right below this one in
  // ContainerRow, Config.tsx, Recovery.tsx) — jdp, live-review: "Die Toggles
  // 'Im Zeitplan einschließen' und 'Nach erfolgreichem Backup updaten' bitte
  // gleich anordnen und der Text gleich formatieren. Der Text soll immer
  // ganz links stehen und der Toggle ganz rechts sein." This component used
  // to render a bare `hideLabel` Toggle and leave its caller (ContainerRow)
  // to hand-roll a `<label className="flex items-center gap-2">` around it —
  // toggle FIRST, text SECOND, `text-xs text-carbon-textSub` — the mirror
  // image of ToggleRow's own text-first/switch-last, `text-sm
  // text-carbon-text` shape, so the two toggles stacked in the same actions
  // column read as two different components even though they are visually
  // meant to be one small, consistent group. Fixed at the actual shared
  // mechanism, not by hand-matching classes at the call site: this component
  // now renders ToggleRow directly, the identical component
  // UpdateAfterBackupRow already wraps a few lines below it in
  // Containers.tsx, so the two rows are now structurally the SAME markup,
  // not two independently-maintained copies that can drift apart again.
  // ContainerRow's own call site lost its wrapping `<label>`/`<span>` — see
  // that component's own comment.
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
