import { useState } from "react";
import { flashDownloadURL } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { useConfirm } from "../../lib/useConfirm";
import { checkRestoreOnce, restoreBlockReason } from "../../lib/useRestoreCheck";
import { Button } from "../Button";
import { FlashPluginList } from "../FlashPluginList";
import { RestoreCheckPanel } from "../restore/RestoreCheckPanel";
import { IconDownload } from "../Sidebar";
import type { TimelinePick } from "../timeline/Timeline";

type T = ReturnType<typeof useT>["t"];

// How long the download button spins after a click. The server sends nothing
// until it has recompressed the whole snapshot, and the browser has no event to
// wait for, so this is a guess.
const DOWNLOAD_PREPARING_MS = 20_000;

export function FlashActions({ pick, t }: { pick: TimelinePick; t: T }) {
  const [preparing, setPreparing] = useState(false);
  const [showPlugins, setShowPlugins] = useState(false);
  const { confirm, confirmDialog } = useConfirm();

  // A native <a download> leaves progress to the browser's download manager,
  // which outlives this row. The check before it speaks up only when the
  // download cannot work.
  async function handleDownload() {
    setPreparing(true);
    const check = await checkRestoreOnce({ kind: "flash", snapshotId: pick.snapshotId, source: pick.source });
    const blocked = restoreBlockReason(check, t);
    if (blocked !== undefined) {
      setPreparing(false);
      await confirm(blocked, { extra: <RestoreCheckPanel check={check} t={t} />, confirmBlocked: blocked });
      return;
    }
    setTimeout(() => setPreparing(false), DOWNLOAD_PREPARING_MS);
    const a = document.createElement("a");
    a.href = flashDownloadURL(pick.snapshotId, pick.source);
    a.download = `flash-${pick.snapshotId.slice(0, 8)}.zip`;
    document.body.appendChild(a);
    a.click();
    a.remove();
  }

  return (
    <>
      <Button
        label={t("flash.download")}
        labelKey="flash.download"
        glyph={<IconDownload />}
        tone={pick.lead ? "accent" : "neutral"}
        onClick={() => void handleDownload()}
        disabled={preparing}
        busy={preparing}
        className="shrink-0"
      />
      {/* order-last puts the plugins under the row's other actions. */}
      <div className="order-last basis-full flex flex-col gap-1">
        <Button
          label={t("flash.plugins")}
          labelKey="flash.plugins"
          tone="neutral"
          onClick={() => setShowPlugins((v) => !v)}
          ariaExpanded={showPlugins}
          glyph={
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none" className={`transition-transform ${showPlugins ? "rotate-90" : "rtl:rotate-180"}`}>
              <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
            </svg>
          }
          className="self-start"
        />
        {showPlugins && <FlashPluginList snapshotId={pick.snapshotId} source={pick.source} t={t} />}
      </div>
      {confirmDialog}
    </>
  );
}
