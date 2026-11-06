// KeepNow applies a location's keep rule right away, which otherwise starts
// with the next backup. It asks the server what the rule would remove and
// removes it only once the person says yes.
import { useState } from "react";
import { applyLocationKeep, previewLocationKeep, type LocationKeepPreview, type StorageLocation } from "../../lib/api";
import { heldTagLabels } from "../../lib/anomalies";
import { useT, type TranslationKey } from "../../lib/i18n";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useOwnLogin } from "../../lib/useOwnLogin";
import { storageLocationsChanged } from "../../lib/useStorageLocations";
import { Button } from "../Button";
import { SettingRow } from "./rows";

/** What each section loses, and the entries an unusual backup holds there. */
function removals(preview: LocationKeepPreview) {
  return preview.sections.map((section) => ({
    domain: section.domain,
    count: section.preview.repos.reduce((n, repo) => n + repo.items.reduce((m, item) => m + (item.remove?.length ?? 0), 0), 0),
    held: section.preview.repos.flatMap((repo) => repo.items.filter((item) => item.paused).map((item) => item.tag)),
  }));
}

export function KeepNow({ location }: { location: StorageLocation }) {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const ownLogin = useOwnLogin();
  const [busy, setBusy] = useState(false);
  const say = (key: TranslationKey) => t(key).replace("{name}", () => location.name);

  function refused(answer: { code?: string; error?: string }) {
    if (!ownLogin.handled(answer)) push(answer.error ?? t("storage.keep.pruneNowFailed"), "fail");
  }

  async function ask(): Promise<boolean> {
    const asked = await previewLocationKeep(location.id);
    if (!asked.ok || !asked.preview) {
      refused(asked);
      return false;
    }
    if (asked.preview.empty) {
      push(say("storage.keep.pruneNowEmpty"), "success");
      return false;
    }
    const parts = removals(asked.preview).filter((part) => part.count > 0 || part.held.length > 0);
    const total = parts.reduce((n, part) => n + part.count, 0);
    if (total === 0) {
      push(say("storage.keep.pruneNowNothing"), "success");
      return false;
    }
    const held = heldTagLabels(parts.flatMap((part) => part.held), t);
    const extra = (
      <ul className="flex flex-col gap-0.5 text-sm text-carbon-textSub">
        {parts.map((part) => (
          <li key={part.domain}>
            {t("storage.keep.pruneNowSection")
              .replace("{section}", () => t(DOMAIN_LABEL[part.domain]))
              .replace("{n}", String(part.count))}
          </li>
        ))}
        {held.length > 0 && <li className="text-statusWarn">{t("storage.keep.pruneNowHeld").replace("{names}", () => held.join(", "))}</li>}
      </ul>
    );
    return confirm(say("storage.keep.pruneNowAsk").replace("{n}", String(total)), {
      title: t("storage.keep.pruneNowLabel"),
      confirmKey: "storage.keep.pruneNow",
      extra,
    });
  }

  async function run() {
    setBusy(true);
    try {
      if (!(await ask())) return;
      const done = await applyLocationKeep(location.id);
      if (!done.ok) {
        refused(done);
        return;
      }
      if (done.paused?.length) push(t("anomaly.prunePaused").replace("{names}", () => heldTagLabels(done.paused ?? [], t).join(", ")), "warn");
      else push(say(done.empty ? "storage.keep.pruneNowEmpty" : "storage.keep.pruneNowDone"), "success");
      storageLocationsChanged();
    } catch (err) {
      push(err instanceof Error ? err.message : t("storage.keep.pruneNowFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <SettingRow label={t("storage.keep.pruneNowLabel")} hint={t("storage.keep.pruneNowHint")}>
        <Button label={t("storage.keep.pruneNow")} labelKey="storage.keep.pruneNow" busy={busy} onClick={() => void run()} />
      </SettingRow>
      {confirmDialog}
      {ownLogin.dialog}
    </>
  );
}
