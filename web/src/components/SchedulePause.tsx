import { useState } from "react";
import type { OkEnvelope } from "../lib/api";
import { useT } from "../lib/i18n";
import { useToast } from "../lib/toast";
import { Badge } from "./Badge";
import { Button } from "./Button";
import { IconPause } from "./glyphs";
import { InfoBubble } from "./InfoBubble";
import { IconSchedules } from "./navGlyphs";

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
  /** Sizes the button for the action corner of a page. */
  corner?: boolean;
}

export function PauseButton({ paused, save, onSaved, corner = false }: PauseButtonProps) {
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

  return (
    <Button
      key={shake}
      label={paused ? t("schedule.resume") : t("schedule.pause")}
      labelKey={paused ? "schedule.resume" : "schedule.pause"}
      glyph={paused ? <IconSchedules /> : <IconPause />}
      tone={corner ? "subtle" : "neutral"}
      onClick={() => void handleClick()}
      disabled={busy}
      busy={busy}
      className={`${corner ? "glim-btn-key" : ""}${shake ? " glim-shake" : ""}`}
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
