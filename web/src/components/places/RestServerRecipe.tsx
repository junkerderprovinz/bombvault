import { useState } from "react";
import { Button } from "../Button";
import { CopyBlock } from "../CopyBlock";
import { InfoBubble } from "../InfoBubble";
import type { DeploySnippetData } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { restServerRecipe } from "../../lib/places";
import { useToast } from "../../lib/toast";

/** RestServerRecipe shows, on request, how to run an append-only rest-server
 *  with one user for this BombVault. The password is made for this request
 *  and shown once; onLogin hands the user and the password to the form. */
export function RestServerRecipe({ onLogin }: { onLogin: (user: string, password: string) => void }) {
  const { t } = useT();
  const { push } = useToast();
  const [snippet, setSnippet] = useState<DeploySnippetData | null>(null);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);

  async function make() {
    setBusy(true);
    try {
      const res = await restServerRecipe();
      if (res.ok && res.snippet) {
        setSnippet(res.snippet);
        onLogin(res.snippet.user, res.snippet.password);
        return;
      }
      push(res.error ?? t("common.actionFailed"), "fail");
      setShake((n) => n + 1);
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.actionFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
      <span className="flex items-center gap-1 text-xs text-carbon-textSub">
        {t("places.recipe.title")}
        <InfoBubble tip={t("places.recipe.hint")} />
      </span>
      {snippet && (
        <>
          <span className="text-xs text-carbon-textSub">{t("places.recipe.password")}</span>
          <CopyBlock text={snippet.password} t={t} />
          <span className="text-xs text-carbon-textSub">{t("fleet.mesh.dockerRun")}</span>
          <CopyBlock text={snippet.dockerRun} t={t} />
          <span className="text-xs text-carbon-textSub">{t("fleet.mesh.compose")}</span>
          <CopyBlock text={snippet.compose} t={t} />
          <span className="text-xs text-carbon-textSub">{t("places.recipe.unraid")}</span>
          <CopyBlock text={snippet.unraid} t={t} />
        </>
      )}
      <Button
        key={shake}
        label={t(snippet ? "places.recipe.newPassword" : "places.recipe.show")}
        labelKey={snippet ? "places.recipe.newPassword" : "places.recipe.show"}
        tone="neutral"
        busy={busy}
        disabled={busy}
        onClick={() => void make()}
        className={`self-start${shake ? " glim-shake" : ""}`}
      />
    </div>
  );
}
