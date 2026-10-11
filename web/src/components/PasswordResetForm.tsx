import { useEffect, useState } from "react";
import { finishPasswordReset, getAuth, startPasswordReset, type PasswordResetRemoved } from "../lib/api";
import { useT } from "../lib/i18n";
import { useReveal } from "../lib/useReveal";
import { Button } from "./Button";
import { InfoBubble } from "./InfoBubble";
import { RevealInput } from "./RevealInput";

const FIELD =
  "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-2 glim-field-focus";
const LABEL = "text-xs text-carbon-textSub font-medium";

// PasswordResetForm takes the code BombVault wrote to its container log and a
// new password, then lists what the reset removed. The caller asks for the code
// before showing it, so the form never appears while the log is still empty.
export function PasswordResetForm({
  onDone,
  onBack,
}: {
  /** Called once the password is set. The answer already carries the session. */
  onDone: () => void;
  onBack: () => void;
}) {
  const { t } = useT();
  const revealNew = useReveal();
  const revealConfirm = useReveal();
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [minLen, setMinLen] = useState(12);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [removed, setRemoved] = useState<PasswordResetRemoved | null>(null);

  useEffect(() => {
    let live = true;
    void getAuth()
      .then((a) => {
        if (live && a.minPasswordLen) setMinLen(a.minPasswordLen);
      })
      .catch(() => undefined);
    return () => {
      live = false;
    };
  }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (busy) return;
    if (password !== confirm) {
      setError(t("auth.passwordMismatch"));
      return;
    }
    if ([...password].length < minLen) {
      setError(t("auth.passwordMinHint", minLen));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const res = await finishPasswordReset(code, password);
      if (res.ok) {
        if (res.removed) setRemoved(res.removed);
        else onDone();
        return;
      }
      if (res.code === "resetCodeRejected") {
        // The answer does not say whether the code expired, and the message
        // promises a new one for that case. Asking again writes one only when
        // none is live.
        await startPasswordReset();
        setError(t("auth.resetCodeRejected"));
        return;
      }
      setError(res.error ?? t("auth.saveError"));
    } catch {
      setError(t("auth.saveError"));
    } finally {
      setBusy(false);
    }
  }

  if (removed) {
    const keysGone = removed.passkeys + removed.apiTokens + removed.mcpKeys > 0;
    // The owner has to know that the passkeys are gone, or the next sign-in
    // looks for a key that no longer exists.
    return (
      <div className="flex flex-col gap-4" role="status">
        <p className="text-sm font-semibold text-carbon-text">{t("auth.resetDoneTitle")}</p>
        <div className="flex flex-col gap-1.5">
          <p className={LABEL}>{t("auth.resetDoneIntro")}</p>
          <ul className="list-disc pl-5 text-sm text-carbon-text flex flex-col gap-1">
            <li>{t("auth.resetDoneSessions")}</li>
            {removed.twoFactor && <li>{t("auth.resetDoneTwoFactor")}</li>}
            {removed.passkeys > 0 && <li>{t("auth.resetDonePasskeys", removed.passkeys)}</li>}
            {removed.apiTokens > 0 && <li>{t("auth.resetDoneApiTokens", removed.apiTokens)}</li>}
            {removed.mcpKeys > 0 && <li>{t("auth.resetDoneMcpKeys", removed.mcpKeys)}</li>}
          </ul>
        </div>
        {keysGone && <p className={LABEL}>{t("auth.resetDoneHint")}</p>}
        <Button label={t("common.done")} labelKey="common.done" tone="accent" type="button" onClick={onDone} />
      </div>
    );
  }

  return (
    <form onSubmit={(e) => void handleSubmit(e)} className="flex flex-col gap-4">
      <div className="flex flex-col gap-1.5">
        <label htmlFor="bv-reset-code" className={`${LABEL} flex items-center gap-1.5`}>
          {t("auth.resetCodeLabel")}
          <InfoBubble tip={t("auth.resetCodeTip")} />
        </label>
        <input
          id="bv-reset-code"
          value={code}
          onChange={(e) => setCode(e.target.value)}
          autoFocus
          autoComplete="one-time-code"
          autoCapitalize="characters"
          spellCheck={false}
          placeholder="XXXX-XXXX-XXXX"
          className={`w-full uppercase tracking-[0.2em] placeholder:tracking-[0.2em] ${FIELD}`}
        />
      </div>

      <div className="flex flex-col gap-1.5">
        <label htmlFor="bv-reset-password" className={LABEL}>
          {t("auth.newPassword")}
        </label>
        <RevealInput
          {...revealNew}
          id="bv-reset-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="new-password"
          wrapperClassName="w-full"
          className={FIELD}
        />
      </div>

      <div className="flex flex-col gap-1.5">
        <label htmlFor="bv-reset-confirm" className={LABEL}>
          {t("auth.confirmPassword")}
        </label>
        <RevealInput
          {...revealConfirm}
          id="bv-reset-confirm"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          autoComplete="new-password"
          wrapperClassName="w-full"
          className={FIELD}
        />
      </div>

      {error && (
        <p className="text-xs text-statusFail" role="alert">
          {error}
        </p>
      )}

      {/* Stacked like the sign-in form. Side by side, the longer translations
          of the two labels do not fit the card. */}
      <Button
        label={t("auth.setPassword")}
        labelKey="auth.setPassword"
        tone="accent"
        type="submit"
        disabled={busy || code.trim() === "" || password === ""}
        busy={busy}
        title={busy ? t("auth.saving") : undefined}
      />
      <Button
        label={t("common.back")}
        labelKey="common.back"
        tone="subtle"
        type="button"
        onClick={onBack}
        disabled={busy}
      />
    </form>
  );
}
