import { useEffect, useState } from "react";
import { finishPasswordReset, getAuth, startPasswordReset } from "../lib/api";
import { useT } from "../lib/i18n";
import { useReveal } from "../lib/useReveal";
import { Button } from "./Button";
import { InfoBubble } from "./InfoBubble";
import { RevealInput } from "./RevealInput";

const FIELD =
  "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-2 glim-field-focus";
const LABEL = "text-xs text-carbon-textSub font-medium";

// PasswordResetForm takes the code BombVault wrote to its container log and a
// new password. The caller asks for the code before showing it, so the form
// never appears while the log is still empty.
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
        onDone();
        return;
      }
      if (res.code === "resetCodeExpired") {
        // The message promises a new code, so it has to be in the log first.
        await startPasswordReset();
        setCode("");
        setError(t("auth.resetCodeExpired"));
        return;
      }
      if (res.code === "resetCodeInvalid") {
        setError(t("auth.resetCodeInvalid"));
        return;
      }
      setError(res.error ?? t("auth.saveError"));
    } catch {
      setError(t("auth.saveError"));
    } finally {
      setBusy(false);
    }
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
          placeholder="XXXX-XXXX"
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
