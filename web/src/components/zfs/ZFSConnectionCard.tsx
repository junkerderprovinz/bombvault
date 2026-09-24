import { useCallback, useEffect, useState } from "react";

import { getVMSSH, zfsConnection } from "../../lib/api";
import type { ZFSConnectionResult } from "../../lib/api";
import { copyText } from "../../lib/clipboard";
import { useT } from "../../lib/i18n";
import { tLtr } from "../../lib/ltrFragments";
import { authorizeCommand } from "../../lib/sshAuthorize";
import { useToast } from "../../lib/toast";
import { zfsCodeSentence, zfsFixKey } from "../../lib/zfsCodes";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { IconDisclosure } from "../IconDisclosure";
import { InfoBubble } from "../InfoBubble";
import { IconCopy } from "../Sidebar";

// Codes the reader can only clear by authorizing BombVault's key on the
// server, so the card hands out the key and the command itself. The Settings
// SSH card is hidden for a basic-mode user who backs up no VMs.
const NEEDS_KEY = new Set(["ssh-auth", "host-placeholder", "ssh-unreachable"]);

/** ZFSConnectionCard reports whether zfs commands reach the server, and what
 *  to do when they do not. */
export function ZFSConnectionCard() {
  const { t } = useT();
  const { push } = useToast();
  const [result, setResult] = useState<ZFSConnectionResult | null>(null);
  const [testing, setTesting] = useState(true);
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [publicKey, setPublicKey] = useState("");

  const test = useCallback(() => {
    setTesting(true);
    zfsConnection()
      .then(setResult)
      .catch(() => setResult(null))
      .finally(() => setTesting(false));
  }, []);

  useEffect(test, [test]);

  const code = result?.code ?? "";
  const needsKey = NEEDS_KEY.has(code);

  useEffect(() => {
    if (!needsKey || publicKey) return;
    getVMSSH()
      .then((r) => {
        if (r.ok && r.publicKey) setPublicKey(r.publicKey);
      })
      .catch(() => undefined);
  }, [needsKey, publicKey]);

  async function copy(value: string) {
    if (await copyText(value)) push(t("common.copied"), "success");
    else push(t("vm.ssh.copyFailed"), "fail");
  }

  const connected = code === "ok" || code === "host-fallback" || code === "propagation-missing";
  const fixKey = zfsFixKey(code);
  const authorizeCmd = authorizeCommand(publicKey);

  return (
    <div className="relative glim-notch-card flex flex-col gap-3 bg-carbon-surface rounded-card p-5">
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap>
          {t("zfs.connection.title")}
          <InfoBubble tip={t("zfs.connection.hint")} onAccent />
        </Badge>
      </h2>

      {testing && <p className="text-sm text-carbon-textMuted">{t("zfs.connection.testing")}</p>}

      {!testing && !result && (
        <div className="flex items-center gap-2 flex-wrap">
          <p className="text-sm text-statusFail">{t("common.networkError")}</p>
          <Button
            label={t("zfs.connection.test")}
            labelKey="zfs.connection.test"
            tone="neutral"
            onClick={test}
            className="ms-auto"
          />
        </div>
      )}

      {!testing && result && (
        <div className="flex flex-col gap-2">
          {connected && (
            <p className="flex items-center gap-2 flex-wrap text-sm text-carbon-text">
              <span>{t("zfs.connection.target").replace("{target}", result.target)}</span>
              {result.version && (
                <span className="text-carbon-textMuted">
                  {t("zfs.connection.version").replace("{version}", result.version)}
                </span>
              )}
            </p>
          )}

          {code === "host-fallback" && (
            <p className="text-xs text-carbon-textMuted">{zfsCodeSentence(t, code)}</p>
          )}

          {code === "propagation-missing" && (
            <p className="flex items-center gap-1.5 text-sm text-statusWarn">
              {zfsCodeSentence(t, code)}
              {fixKey && <InfoBubble tip={tLtr(t, fixKey)} />}
            </p>
          )}

          {!connected && (
            <p className="flex items-center gap-1.5 text-sm text-statusFail">
              {zfsCodeSentence(t, code, { target: result.target, uriTarget: result.uriTarget })}
              {fixKey && <InfoBubble tip={tLtr(t, fixKey)} />}
            </p>
          )}

          {needsKey && publicKey && (
            <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
              <span className="text-xs text-carbon-textMuted">{tLtr(t, "vm.ssh.publicKey")}</span>
              <div className="flex items-start gap-2">
                <code className="flex-1 break-all rounded-control bg-carbon-surface p-2 text-xs text-carbon-text">
                  {publicKey}
                </code>
                <Button
                  label={t("common.copy")}
                  labelKey="common.copy"
                  glyph={<IconCopy />}
                  tone="accent"
                  onClick={() => void copy(publicKey)}
                  className="shrink-0"
                />
              </div>
              <span className="text-xs text-carbon-textSub">{t("zfs.connection.authorize")}</span>
              <div className="flex items-start gap-2">
                <pre className="flex-1 overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre">
                  {authorizeCmd}
                </pre>
                <Button
                  label={t("vm.ssh.copyCmd")}
                  labelKey="vm.ssh.copyCmd"
                  glyph={<IconCopy />}
                  tone="accent"
                  onClick={() => void copy(authorizeCmd)}
                  className="shrink-0"
                />
              </div>
            </div>
          )}

          {result.detail && (
            <div className="flex flex-col gap-1">
              <button
                type="button"
                onClick={() => setDetailsOpen((open) => !open)}
                aria-expanded={detailsOpen}
                className="flex items-center gap-1.5 self-start text-xs text-carbon-textMuted hover:text-carbon-text"
              >
                <IconDisclosure open={detailsOpen} />
                {t("zfs.connection.details")}
              </button>
              {detailsOpen && (
                <pre
                  dir="ltr"
                  className="overflow-x-auto rounded-control bg-carbon-surface2 p-2 text-caption text-carbon-textSub whitespace-pre-wrap text-start"
                >
                  {result.detail}
                </pre>
              )}
            </div>
          )}

          <div className="flex justify-end">
            <Button
              label={t("zfs.connection.test")}
              labelKey="zfs.connection.test"
              tone="neutral"
              onClick={test}
            />
          </div>
        </div>
      )}
    </div>
  );
}
