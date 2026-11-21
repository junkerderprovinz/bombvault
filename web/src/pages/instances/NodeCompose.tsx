import { useEffect, useState } from "react";
import { nodeCompose, type NodeCompose, type NodeView } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { composeFile } from "../../lib/nodeCompose";
import { secretsSentence } from "../../lib/secretsRefusal";
import { useOwnLogin } from "../../lib/useOwnLogin";
import { Button } from "../../components/Button";
import { CopyBlock } from "../../components/CopyBlock";
import { InstanceWindow } from "./InstanceWindow";

/** ComposeView is the node's compose file, ready to copy, and how to start it. */
export function ComposeView({ values, lead }: { values: NodeCompose; lead?: string }) {
  const { t } = useT();
  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-carbon-textSub">{lead ?? t("nodes.composeIntro")}</p>
      <div data-testid="node-compose">
        <CopyBlock text={composeFile(values)} />
      </div>
      <p className="text-xs text-statusWarn">{t("nodes.composeKeep")}</p>
    </div>
  );
}

/** ReplaceWindow hands out a node's compose file again, to start the node on
 *  another server or rebuild it. Only a person signed in with a password gets it. */
export function ReplaceWindow({ node, onClose }: { node: NodeView; onClose: () => void }) {
  const { t } = useT();
  const ownLogin = useOwnLogin();
  const [values, setValues] = useState<NodeCompose | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    nodeCompose(node.id)
      .then((res) => {
        if (!alive) return;
        if (res.compose) {
          setValues(res.compose);
          return;
        }
        if (ownLogin.handled(res, onClose)) return;
        setError(secretsSentence(t, res.code) ?? res.error ?? t("settings.error"));
      })
      .catch((err: unknown) => alive && setError(err instanceof Error ? err.message : t("settings.error")));
    return () => {
      alive = false;
    };
    // The values are asked for once per window; the login offer is no reason to ask again.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [node.id]);

  return (
    <InstanceWindow
      title={t("nodes.replaceTitle")}
      facts={node.name}
      onClose={onClose}
      footer={<Button label={t("common.done")} labelKey="common.done" tone="accent" onClick={onClose} />}
    >
      {values && <ComposeView values={values} lead={t("nodes.replaceIntro").replace("{name}", () => node.name)} />}
      {!values && !error && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {error && (
        <p role="alert" className="text-sm text-statusFail wrap-break-word">
          {error}
        </p>
      )}
      {ownLogin.dialog}
    </InstanceWindow>
  );
}
