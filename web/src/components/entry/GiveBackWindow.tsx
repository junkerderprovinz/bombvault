import { useState } from "react";
import { importAppExport, type TimelineRow } from "../../lib/api";
import { actionMark, rowTime } from "../../lib/entry";
import { useT, type TranslationKey } from "../../lib/i18n";
import { formatRecent } from "../../lib/reltime";
import { secretsSentence } from "../../lib/secretsRefusal";
import { newestId } from "../../lib/timeline";
import { useToast } from "../../lib/toast";
import { useOwnLogin } from "../../lib/useOwnLogin";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { IconRestore } from "../navGlyphs";
import type { TimelineState } from "./BackupPlaces";

// The reasons the server names, each with its own sentence. Any other reason
// is quoted in the general one.
const REFUSALS: Record<string, TranslationKey> = {
  notRunning: "appImport.notRunning",
  noApp: "appImport.noApp",
  otherApp: "appImport.otherApp",
  relay: "appImport.relay",
  permissionOff: "appImport.permissionOff",
  busy: "appImport.busy",
  versionTooNew: "appImport.versionTooNew",
  wrongApp: "appImport.wrongApp",
  keyMissing: "appImport.keyMissing",
  broken: "appImport.broken",
};

/**
 * GiveBackWindow asks before it gives the app in container `name` its own
 * backup back, since the app gives up what it holds for it, and says why when
 * the app does not take it.
 */
export function GiveBackWindow({
  name,
  row,
  timeline,
  onClose,
}: {
  name: string;
  row: TimelineRow;
  timeline: TimelineState;
  onClose: () => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const ownLogin = useOwnLogin();
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<string | null>(null);
  const app = row.appExport?.name ?? "";
  const mark = actionMark(row, timeline.places);
  const fill = (text: string, reason = "") =>
    text.replaceAll("{app}", () => app).replaceAll("{name}", () => name).replace("{reason}", () => reason);

  async function giveBack() {
    setBusy(true);
    setRefusal(null);
    try {
      const res = await importAppExport(name, newestId(mark), mark.place);
      if (res.ok) {
        push(fill(t("appImport.done")), "success");
        onClose();
        return;
      }
      if (ownLogin.handled(res)) return;
      const key = res.code ? REFUSALS[res.code] : undefined;
      setRefusal(key ? fill(t(key)) : (secretsSentence(t, res.code) ?? fill(t("appImport.failed"), res.error ?? "")));
    } catch (err) {
      setRefusal(fill(t("appImport.failed"), err instanceof Error ? err.message : ""));
    } finally {
      setBusy(false);
    }
  }

  return (
    <InstanceWindow
      title={fill(t("appImport.title"))}
      facts={name}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
          <Button
            label={t("appImport.confirm")}
            labelKey="appImport.confirm"
            glyph={<IconRestore />}
            tone="accent"
            onClick={() => void giveBack()}
            disabled={busy}
            busy={busy}
          />
        </>
      }
    >
      <p className="text-sm text-carbon-text">
        {fill(t("appImport.body")).replace("{when}", () => formatRecent(rowTime(row), lang))}
      </p>
      {refusal && (
        <p role="alert" className="text-sm text-statusFail wrap-break-word">
          {refusal}
        </p>
      )}
      {ownLogin.dialog}
    </InstanceWindow>
  );
}
