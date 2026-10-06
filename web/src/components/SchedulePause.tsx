import { useState } from "react";
import type { OkEnvelope } from "../lib/api";
import { useT } from "../lib/i18n";
import { useToast } from "../lib/toast";
import { Badge } from "./Badge";
import { Button } from "./Button";
import { InfoBubble } from "./InfoBubble";

// The card-level face of an item's "Include in schedule" switch. Both write
// the same stored flag, and the page keeps that flag in its list, so the
// switch, this button and the badge always show one value.

interface PauseButtonProps {
  /** Whether the item is out of the schedule. */
  paused: boolean;
  /** Stores the flag; takes the new "include in schedule" value. */
  save: (include: boolean) => Promise<OkEnvelope>;
  /** Called with the stored value once the save succeeded. */
  onSaved?: (include: boolean) => void;
}

export function PauseButton({ paused, save, onSaved }: PauseButtonProps) {
  const { t } = useT();
  const { push } = useToast();
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);

  async function handleClick() {
    const include = paused;
    setBusy(true);
    try {
      const res = await save(include);
      if (res.ok) {
        onSaved?.(include);
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

  // The glyph set has a play mark but no pause mark, so both states stay in
  // words: a button that turned from a word into a symbol on click would read
  // as two different controls.
  return (
    <Button
      key={shake}
      label={paused ? t("schedule.resume") : t("schedule.pause")}
      labelKey={paused ? "schedule.resume" : "schedule.pause"}
      glyph={undefined}
      tone="neutral"
      onClick={() => void handleClick()}
      disabled={busy}
      busy={busy}
      className={shake ? "glim-shake" : ""}
    />
  );
}

/** PausedBadge marks an item that scheduled runs skip. `explained={false}`
 *  drops the (i) where the badge sits inside another control, such as the
 *  phone list row, which is a button itself. */
export function PausedBadge({ explained = true }: { explained?: boolean }) {
  const { t } = useT();
  // The hint names both actions by their own labels, so it follows a rename.
  const tip = t("schedule.pausedHint")
    .replace("{everything}", t("settings.everythingTitle"))
    .replace("{backupNow}", t("containers.backupNow"));
  return (
    <Badge tone="neutral">
      {t("schedule.paused")}
      {explained && <InfoBubble tip={tip} onAccent />}
    </Badge>
  );
}
