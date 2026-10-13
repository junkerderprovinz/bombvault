import { useEffect, useState } from "react";

import { useT } from "../../lib/i18n";
import { clearNotStarted, readNotStarted, runLockHeld, type NotStarted } from "../../lib/restoreMany";
import { Button } from "../Button";
import { IconClose } from "../navGlyphs";

/** NotStartedNotice names the entries a restore of several never started
 *  because its window was left while it ran. A restore that still runs in
 *  another tab is not named. */
export function NotStartedNotice() {
  const { t } = useT();
  const [kept] = useState(readNotStarted);
  const [shown, setShown] = useState<NotStarted | null>(null);

  useEffect(() => {
    if (!kept) return;
    let current = true;
    void runLockHeld().then((held) => {
      if (current && !held) setShown(kept);
    });
    return () => {
      current = false;
    };
  }, [kept]);

  if (!shown) return null;
  return (
    <div className="flex items-start gap-3 rounded-card bg-statusWarnBg px-4 py-3 text-sm text-statusWarn">
      <p role="status" className="min-w-0 flex-1 wrap-break-word">{t("restoreMany.leftBehind").replace("{names}", shown.names.join(", "))}</p>
      <Button
        label={t("common.close")}
        labelKey="common.close"
        glyph={<IconClose />}
        variant="icon"
        tone="subtle"
        onClick={() => {
          clearNotStarted();
          setShown(null);
        }}
      />
    </div>
  );
}
