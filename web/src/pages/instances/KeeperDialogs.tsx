// Making a key deposit, and the one moment its recovery code is shown.
import { useState } from "react";

import { Button } from "../../components/Button";
import { IconKey } from "../../components/glyphs";
import { IconCheck, IconCopy } from "../../components/navGlyphs";
import { askKeeper } from "../../lib/api";
import { copyText } from "../../lib/clipboard";
import { depositSentence } from "../../lib/depositCodes";
import { useT, type TranslationKey } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useOwnLogin } from "../../lib/useOwnLogin";
import { ToggleRow } from "../settings/shared";
import { RoleDialog } from "./RoleDialog";
import { ROLES } from "./RoleTile";
import { tooOldToKeep, type Instance } from "./instancesModel";

/** RecoveryCodeDialog shows a recovery code the server just made. The server
 *  keeps no copy and cannot show it again, so the window stays until the
 *  person says the code is written down. */
export function RecoveryCodeDialog({ code, onDone }: { code: string; onDone: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const [written, setWritten] = useState(false);
  const [copied, setCopied] = useState(false);

  async function copy() {
    if (await copyText(code)) setCopied(true);
    else push(t("vm.ssh.copyFailed"), "fail");
  }

  return (
    <RoleDialog
      title={t("keeper.code.title")}
      hint={t("keeper.code.tip")}
      lead={t("keeper.code.lead")}
      locked
      onClose={onDone}
      footer={<Button label={t("common.done")} labelKey="common.done" tone="accent" onClick={onDone} disabled={!written} />}
    >
      <ol dir="ltr" aria-label={t("keeper.code.title")} data-testid="recovery-code" className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {code.split("-").map((group, i) => (
          <li
            key={i}
            className="rounded-control bg-carbon-surface2 px-2 py-2.5 text-center font-mono text-status font-semibold tracking-[.18em] text-carbon-text"
          >
            {group}
          </li>
        ))}
      </ol>
      <div className="flex justify-end">
        <Button
          label={t(copied ? "common.copied" : "common.copy")}
          labelKey={copied ? "common.copied" : "common.copy"}
          glyph={copied ? <IconCheck /> : <IconCopy />}
          tone="neutral"
          onClick={() => void copy()}
        />
      </div>
      <p className="text-sm text-carbon-textSub">{t("keeper.code.lost")}</p>
      <ToggleRow label={t("keeper.code.written")} checked={written} onChange={setWritten} />
    </RoleDialog>
  );
}

/** KeeperDialog gives an instance this server's key deposit. It says what the
 *  deposit is and what the person will need on the day it is used, and after
 *  the first deposit it shows the recovery code. An instance too old to keep
 *  one gets the reason and no deposit. */
export function KeeperDialog({
  instance,
  hasCode,
  onClose,
  onDone,
}: {
  instance: Instance;
  /** This server has a recovery code already, so this deposit makes none. */
  hasCode: boolean;
  onClose: () => void;
  onDone: () => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [code, setCode] = useState<string | null>(null);
  const ownLogin = useOwnLogin();
  const named = (key: TranslationKey) => t(key).replaceAll("{peer}", () => instance.name);
  const title = named("instances.role.asTitle").replace("{role}", t(ROLES.keeper.label));
  const tooOld = tooOldToKeep(instance);

  async function deposit() {
    setError(null);
    setBusy(true);
    try {
      const res = await askKeeper(instance.memberId);
      if (!res.ok) {
        if (!ownLogin.handled(res)) setError(depositSentence(t, res));
        return;
      }
      if (res.recoveryCode) {
        setCode(res.recoveryCode);
        return;
      }
      push(named("keeper.made"), "success");
      onDone();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("instances.role.saveError"));
    } finally {
      setBusy(false);
    }
  }

  if (code) return <RecoveryCodeDialog code={code} onDone={onDone} />;

  if (tooOld) {
    return (
      <RoleDialog
        title={title}
        hint={t(ROLES.keeper.tip)}
        lead={named("keeper.tooOld")}
        onClose={onClose}
        footer={<Button label={t("common.close")} labelKey="common.close" tone="accent" onClick={onClose} />}
      />
    );
  }

  return (
    <RoleDialog
      title={title}
      hint={t(ROLES.keeper.tip)}
      lead={named("keeper.lead")}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onClose} disabled={busy} />
          <Button
            label={t("keeper.make")}
            labelKey="keeper.make"
            glyph={<IconKey />}
            tone="accent"
            onClick={() => void deposit()}
            disabled={busy}
            busy={busy}
          />
        </>
      }
    >
      <div className="flex flex-col gap-2">
        <span className="text-subline font-semibold text-carbon-textSub">{t("keeper.need.title")}</span>
        <ol data-testid="keeper-needs" className="flex list-decimal flex-col gap-1 ps-5 text-sm text-carbon-text">
          <li>{t("keeper.need.words")}</li>
          <li>{t(hasCode ? "keeper.need.codeHave" : "keeper.need.code")}</li>
          <li>{named("keeper.need.access")}</li>
        </ol>
      </div>
      <p className="text-sm text-carbon-textSub">{named("keeper.without")}</p>
      {error && <p className="text-xs text-statusFail wrap-break-word">{error}</p>}
      {ownLogin.dialog}
    </RoleDialog>
  );
}
