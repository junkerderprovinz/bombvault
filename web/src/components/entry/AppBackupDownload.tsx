import { useState } from "react";
import { appExportDownloadURL, checkAppExportDownload, type TimelinePlace, type TimelineRow } from "../../lib/api";
import { actionMark } from "../../lib/entry";
import { useT } from "../../lib/i18n";
import { secretsSentence } from "../../lib/secretsRefusal";
import { newestId } from "../../lib/timeline";
import { useToast } from "../../lib/toast";
import { useOwnLogin } from "../../lib/useOwnLogin";
import { Button } from "../Button";
import { IconDownload } from "../navGlyphs";

// The browser owns the download once the link is followed, so the button only
// shows for a while that something started.
const DOWNLOAD_PREPARING_MS = 20_000;

/**
 * AppBackupDownload hands the own backup of the app in container `name` to the
 * browser, from the place a restore would read it at. The server is asked
 * first, so a refusal reads as a message rather than a failed download.
 */
export function AppBackupDownload({ name, row, places }: { name: string; row: TimelineRow; places: TimelinePlace[] }) {
  const { t } = useT();
  const { push } = useToast();
  const ownLogin = useOwnLogin();
  const [preparing, setPreparing] = useState(false);
  const mark = actionMark(row, places);
  const id = newestId(mark);

  async function download() {
    setPreparing(true);
    try {
      const res = await checkAppExportDownload(name, id, mark.place);
      if (!res.ok) {
        setPreparing(false);
        if (!ownLogin.handled(res)) {
          push(secretsSentence(t, res.code) ?? `${t("entry.list.downloadRefused")}: ${res.error ?? ""}`.trim(), "fail");
        }
        return;
      }
    } catch (err) {
      setPreparing(false);
      push(err instanceof Error ? err.message : t("entry.list.downloadRefused"), "fail");
      return;
    }
    setTimeout(() => setPreparing(false), DOWNLOAD_PREPARING_MS);
    const a = document.createElement("a");
    a.href = appExportDownloadURL(name, id, mark.place);
    a.download = "";
    document.body.appendChild(a);
    a.click();
    a.remove();
  }

  return (
    <>
      <Button
        label={t("entry.list.download")}
        labelKey="entry.list.download"
        glyph={<IconDownload />}
        onClick={() => void download()}
        disabled={preparing}
        busy={preparing}
      />
      {ownLogin.dialog}
    </>
  );
}
