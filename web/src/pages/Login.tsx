import { useEffect, useState } from "react";
import {
  finishPasskeyWithCode,
  finishPasswordReset,
  getAuth,
  login,
  loginWithPasskey,
  passkeyStatus,
  passkeysAvailableInBrowser,
  startPasswordReset,
  type PasswordResetRemoved,
} from "../lib/api";
import { useT } from "../lib/i18n";
import type { SignInText } from "../lib/signIn";
import { Button } from "../components/Button";
import { SignInCard } from "../components/SignInCard";

interface LoginPageProps {
  /** Called after a successful login so the parent can re-check auth state. */
  onLogin: () => void;
}

// LoginPage is the full-screen login shown while auth is on and nobody is
// signed in. The password goes first; when a second factor is armed the
// server answers needCode and the code field appears, with the password kept.
// The field waits for that answer even when GET /api/auth already reports a
// factor, so nobody spends a 30-second code window on a mistyped password.
export function LoginPage({ onLogin }: LoginPageProps) {
  const { t } = useT();
  // Set once a passkey signed in and the server asks for the code on top.
  const [passkeyHandle, setPasskeyHandle] = useState<string | null>(null);
  const [minLen, setMinLen] = useState(12);
  // What a reset removed, held until the reader has seen it.
  const [removed, setRemoved] = useState<PasswordResetRemoved | null>(null);

  // The passkey button needs WebAuthn in the browser, an address that can carry
  // a passkey (a bare IP cannot, see internal/api/passkeys.go) and a key
  // registered for it. A prompt that cannot succeed is worse than no button.
  const [passkeyOffer, setPasskeyOffer] = useState(false);
  useEffect(() => {
    if (!passkeysAvailableInBrowser()) return;
    let live = true;
    void passkeyStatus()
      .then((s) => {
        if (live) setPasskeyOffer(s.ok && s.supported === true && (s.here ?? 0) > 0);
      })
      .catch(() => undefined);
    return () => {
      live = false;
    };
  }, []);

  const text: SignInText = {
    password: t("auth.passwordLabel"),
    signIn: t("auth.signIn"),
    signingIn: t("auth.signingIn"),
    passkey: t("auth.signInWithPasskey"),
    forgot: t("auth.forgotPassword"),
    code: t("auth.codeLabel"),
    codeHint: t("auth.codeHint"),
    codeInvalid: t("auth.codeInvalid"),
    back: t("common.back"),
    resetCode: t("auth.resetCodeLabel"),
    resetCodeTip: t("auth.resetCodeTip"),
    newPassword: t("auth.newPassword"),
    confirmPassword: t("auth.confirmPassword"),
    setPassword: t("auth.setPassword"),
    saving: t("auth.saving"),
    mismatch: t("auth.passwordMismatch"),
    tooShort: t("auth.passwordMinHint", minLen),
    resetRejected: t("auth.resetCodeRejected"),
    failed: t("auth.loginError"),
  };

  return (
    <SignInCard
      logo="/logo.svg"
      appName={t("auth.loginTitle")}
      text={text}
      idPrefix="bv"
      passkey={passkeyOffer}
      reset
      minPassword={minLen}
      step={removed ? <ResetDone removed={removed} onDone={onLogin} /> : undefined}
      onSignIn={async ({ password, code, passkey }) => {
        try {
          if (passkey) {
            const res = await finishPasskeyWithCode(passkeyHandle!, code!);
            if (res.ok) {
              onLogin();
              return { ok: true };
            }
            const again = res.needCode && res.ceremonyId ? res.ceremonyId : null;
            setPasskeyHandle(again);
            return { needCode: !!again, error: res.error ?? t("auth.codeInvalid") };
          }
          const res = await login(password, code);
          if (res.ok) {
            onLogin();
            return { ok: true };
          }
          if (res.needCode) return { needCode: true, error: res.error ?? t("auth.codeInvalid") };
          return { error: res.error ?? t("auth.invalidPassword") };
        } catch {
          return { error: t("auth.loginError") };
        }
      }}
      onPasskey={async () => {
        try {
          const res = await loginWithPasskey();
          if (res.ok) {
            onLogin();
            return { ok: true };
          }
          if (res.needCode && res.ceremonyId) {
            setPasskeyHandle(res.ceremonyId);
            return { needCode: true };
          }
          return {
            error: res.code === "passkeyNeedsVerification" ? t("auth.passkeyNeedsVerification") : (res.error ?? t("auth.passkeySignInFailed")),
          };
        } catch (err) {
          // A cancelled prompt lands here too, and its own message says more than
          // a generic one.
          return { error: err instanceof Error ? err.message : t("auth.passkeySignInFailed") };
        }
      }}
      // The reset form shows only after the server has written a code to its
      // log, so the form never asks for a code that does not exist.
      onAskCode={async () => {
        try {
          const res = await startPasswordReset();
          if (!res.ok) return { error: res.error ?? t("auth.loginError") };
          void getAuth()
            .then((a) => {
              if (a.minPasswordLen) setMinLen(a.minPasswordLen);
            })
            .catch(() => undefined);
          return { ok: true };
        } catch {
          return { error: t("auth.loginError") };
        }
      }}
      onSetPassword={async ({ code, password }) => {
        try {
          const res = await finishPasswordReset(code, password);
          if (res.ok) {
            // The answer already carries the session.
            if (res.removed) setRemoved(res.removed);
            else onLogin();
            return { ok: true };
          }
          if (res.code === "resetCodeRejected") return { rejected: true };
          return { error: res.error ?? t("auth.saveError") };
        } catch {
          return { error: t("auth.saveError") };
        }
      }}
    />
  );
}

/** ResetDone lists what the reset removed before the app opens. The owner has
 *  to know that the passkeys are gone, or the next sign-in looks for a key that
 *  no longer exists. */
function ResetDone({ removed, onDone }: { removed: PasswordResetRemoved; onDone: () => void }) {
  const { t } = useT();
  const keysGone = removed.passkeys + removed.apiTokens + removed.mcpKeys > 0;
  return (
    <div className="glim-signin-form" role="status">
      <h2 className="text-sm font-semibold text-carbon-text">{t("auth.resetDoneTitle")}</h2>
      <div className="flex flex-col gap-1.5">
        <p className="text-xs text-carbon-textSub font-medium">{t("auth.resetDoneIntro")}</p>
        <ul className="list-disc ps-5 text-sm text-carbon-text flex flex-col gap-1">
          <li>{t("auth.resetDoneSessions")}</li>
          {removed.phonePairingOff && <li>{t("auth.resetDonePhones")}</li>}
          {removed.twoFactor && <li>{t("auth.twoFactorOffNow")}</li>}
          {removed.passkeys > 0 && <li>{t("auth.resetDonePasskeys", removed.passkeys)}</li>}
          {removed.apiTokens > 0 && <li>{t("auth.resetDoneApiTokens", removed.apiTokens)}</li>}
          {removed.mcpKeys > 0 && <li>{t("auth.resetDoneMcpKeys", removed.mcpKeys)}</li>}
        </ul>
      </div>
      {keysGone && <p className="text-xs text-carbon-textSub font-medium">{t("auth.resetDoneHint")}</p>}
      <Button label={t("common.done")} labelKey="common.done" tone="accent" onClick={onDone} />
    </div>
  );
}
