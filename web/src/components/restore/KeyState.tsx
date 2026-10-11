// Says which key this server writes its backups with when it is not its own:
// one taken over in an emergency, or one that APP_KEY stopped opening.
import { useState } from "react";
import { undoEmergency, type EmergencyState } from "../../lib/api";
import { ATTENTION_CHANGED_EVENT } from "../../lib/attention";
import { refusalText } from "../../lib/emergency";
import { useT, type TranslationKey } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useOwnLogin } from "../../lib/useOwnLogin";
import { Button } from "../Button";
import { Note } from "./EmergencyParts";

const ADOPTED_FROM: Record<NonNullable<EmergencyState["source"]>, TranslationKey> = {
  kit: "restorePage.keyAdopted.kit",
  typed: "restorePage.keyAdopted.typed",
  group: "restorePage.keyAdopted.group",
};

export function KeyState({ state, onChanged }: { state: EmergencyState | null; onChanged: (state: EmergencyState) => void }) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [undoing, setUndoing] = useState(false);
  const ownLogin = useOwnLogin();

  if (!state || state.backupKey === "own") return null;
  if (state.backupKey === "locked") return <Note tone="fail">{t("emergency.code.backup-key-locked")}</Note>;

  async function undo() {
    if (!(await confirm(t("emergency.undoAsk"), { confirmLabel: t("emergency.undo"), confirmLabelKey: "snapshots.restore" }))) return;
    setUndoing(true);
    try {
      const res = await undoEmergency();
      if (res.emergency) onChanged(res.emergency);
      if (res.ok) {
        push(t("emergency.undone"), "success");
        window.dispatchEvent(new Event(ATTENTION_CHANGED_EVENT));
      } else if (!ownLogin.handled(res)) push(refusalText(t, res.code, res.error), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setUndoing(false);
    }
  }

  const on = state.adoptedAt ? new Date(state.adoptedAt * 1000).toLocaleDateString(lang, { dateStyle: "medium" }) : "";
  return (
    <div data-testid="key-adopted" className="flex flex-col gap-2 rounded-card bg-carbon-surface2 px-3 py-2.5 text-sm text-carbon-text">
      <p className="leading-relaxed">
        {t(ADOPTED_FROM[state.source ?? "kit"]).replace("{date}", () => on)} {t("restorePage.keyAdoptedKit")}
      </p>
      {state.undoable ? (
        <div className="flex flex-wrap items-center gap-3">
          <span className="min-w-0 flex-1 basis-48 text-xs text-carbon-textMuted">{t("emergency.undoable")}</span>
          <Button label={t("emergency.undo")} labelKey={null} onClick={() => void undo()} disabled={undoing} busy={undoing} />
        </div>
      ) : (
        <p className="text-xs text-carbon-textMuted">{t("emergency.code.adopt-in-use")}</p>
      )}
      {confirmDialog}
      {ownLogin.dialog}
    </div>
  );
}
