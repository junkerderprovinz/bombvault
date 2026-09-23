import { useEffect, useState } from "react";
import {
  login,
  loginWithPasskey,
  passkeyStatus,
  passkeysAvailableInBrowser,
} from "../lib/api";
import { useT } from "../lib/i18n";
import { RevealInput } from "../components/RevealInput";
import { Button } from "../components/Button";
import { useReveal } from "../lib/useReveal";

interface LoginPageProps {
  /** Called after a successful login so the parent can re-check auth state. */
  onLogin: () => void;
}

// LoginPage is the full-screen login form shown while auth is on and nobody is
// signed in. The password goes first; when a second factor is armed the
// server answers needCode and the code field appears, with the password kept.
// The field waits for that answer even when GET /api/auth already reports a
// factor, so nobody spends a 30-second code window on a mistyped password.
export function LoginPage({ onLogin }: LoginPageProps) {
  const { t } = useT();
  const reveal = useReveal();
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [needCode, setNeedCode] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      const res = await login(password, needCode ? code : undefined);
      if (res.ok) {
        onLogin();
        return;
      }
      if (res.needCode) {
        setNeedCode(true);
        setCode("");
        // The first needCode answer only reveals the field; a message waits
        // until a code has been tried and rejected.
        setError(code === "" ? null : (res.error ?? t("auth.codeInvalid")));
        return;
      }
      setError(res.error ?? t("auth.invalidPassword"));
    } catch {
      setError(t("auth.loginError"));
    } finally {
      setBusy(false);
    }
  }

  // The passkey button needs WebAuthn in the browser, an address that can carry
  // a passkey (a bare IP cannot, see internal/api/passkeys.go) and a key
  // registered for it. A prompt that cannot succeed is worse than no button.
  const [passkeyOffer, setPasskeyOffer] = useState(false);
  const [passkeyBusy, setPasskeyBusy] = useState(false);
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

  async function signInWithPasskey() {
    setPasskeyBusy(true);
    setError(null);
    try {
      const res = await loginWithPasskey();
      if (res.ok) {
        onLogin();
        return;
      }
      setError(res.error ?? t("auth.passkeySignInFailed"));
    } catch (err) {
      // A cancelled prompt lands here too, and its own message says more than
      // a generic one.
      setError(err instanceof Error ? err.message : t("auth.passkeySignInFailed"));
    } finally {
      setPasskeyBusy(false);
    }
  }

  const submitDisabled = busy || password === "" || (needCode && code.trim() === "");

  return (
    <div className="flex items-center justify-center min-h-screen bg-carbon-background">
      <div className="w-full max-w-sm rounded-card bg-carbon-surface p-8 flex flex-col gap-6 shadow-lg">
        {/* This screen has no rail, so the mark tells which instance is being
            unlocked. The theme marks switch like the rail's and stay hidden
            from assistive technology, since the heading names the product. */}
        <div className="flex flex-col items-center gap-3">
          <span className="flex h-16 w-16 items-center justify-center">
            <img
              src="/logo.svg"
              alt=""
              aria-hidden="true"
              draggable={false}
              className="h-16 w-16 object-contain block dark:hidden"
            />
            <img
              src="/logo-light.svg"
              alt=""
              aria-hidden="true"
              draggable={false}
              className="h-16 w-16 object-contain hidden dark:block"
            />
          </span>
          <h1 className="text-2xl font-semibold text-carbon-text text-center">
            {t("auth.loginTitle")}
          </h1>
        </div>

        <form onSubmit={(e) => void handleSubmit(e)} className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <label
              htmlFor="bv-password"
              className="text-xs text-carbon-textSub font-medium"
            >
              {t("auth.passwordLabel")}
            </label>
            <RevealInput
              {...reveal}
              id="bv-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoFocus
              autoComplete="current-password"
              wrapperClassName="w-full"
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-2 glim-field-focus"
            />
          </div>

          {/* Second factor, once the password has been accepted. */}
          {needCode && (
            <div className="flex flex-col gap-1.5">
              <label
                htmlFor="bv-code"
                className="text-xs text-carbon-textSub font-medium"
              >
                {t("auth.codeLabel")}
              </label>
              <input
                id="bv-code"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                autoFocus
                inputMode="numeric"
                autoComplete="one-time-code"
                placeholder="000000"
                className="w-full rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-2 tracking-[0.35em] glim-field-focus"
              />
              <p className="text-xs text-carbon-textSub">{t("auth.codeHint")}</p>
            </div>
          )}

          {error && (
            <p className="text-xs text-statusFail" role="alert">
              {error}
            </p>
          )}

          <Button
            label={t("auth.signIn")}
            labelKey="auth.signIn"
            tone="accent"
            type="submit"
            disabled={submitDisabled}
            busy={busy}
            title={busy ? t("auth.signingIn") : undefined}
          />

          {/* Below the password, which always works. Hidden rather than
              disabled where it cannot work: a disabled control on a login
              screen reads as "you are locked out". */}
          {passkeyOffer && !needCode && (
            <Button
              label={t("auth.signInWithPasskey")}
              labelKey="auth.signInWithPasskey"
              tone="neutral"
              type="button"
              onClick={() => void signInWithPasskey()}
              disabled={busy || passkeyBusy}
              busy={passkeyBusy}
            />
          )}
        </form>
      </div>
    </div>
  );
}
