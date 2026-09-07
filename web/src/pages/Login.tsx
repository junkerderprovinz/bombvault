import { useState } from "react";
import { login } from "../lib/api";
import { useT } from "../lib/i18n";
import { RevealInput } from "../components/RevealInput";
import { Button } from "../components/Button";
import { useReveal } from "../lib/useReveal";

interface LoginPageProps {
  /** Called after a successful login so the parent can re-check auth state. */
  onLogin: () => void;
}

// ---------------------------------------------------------------------------
// LoginPage — full-screen centered login form, shown when auth is ON + not authed.
//
// Since v8.6.0 it has a second step. The password field is submitted on its own
// first; if a second factor is armed the server answers needCode and the code
// field appears. The password is kept in state across that, so accepting the
// code does not mean typing the password again.
//
// The code field is NOT shown up front, even when GET /api/auth says a factor is
// armed. Asking for a code before the password has been accepted invites people
// to burn a 30-second window on a password they then mistype, and the round trip
// that reveals the field costs nothing.
// ---------------------------------------------------------------------------

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
        // The first answer of needCode is not a failure, it is the form
        // discovering it has a second field. Only say something once the code
        // has actually been tried and rejected.
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

  const submitDisabled = busy || password === "" || (needCode && code.trim() === "");

  return (
    <div className="flex items-center justify-center min-h-screen bg-carbon-background">
      <div className="w-full max-w-sm rounded-card bg-carbon-surface p-8 flex flex-col gap-6 shadow-lg">
        {/* Title */}
        <h1 className="text-2xl font-semibold text-carbon-text text-center">
          {t("auth.loginTitle")}
        </h1>

        <form onSubmit={(e) => void handleSubmit(e)} className="flex flex-col gap-4">
          {/* Password field */}
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

          {/* Error message */}
          {error && (
            <p className="text-xs text-statusFail" role="alert">
              {error}
            </p>
          )}

          {/* Submit button */}
          <Button
            label={t("auth.signIn")}
            labelKey="auth.signIn"
            tone="accent"
            type="submit"
            disabled={submitDisabled}
            busy={busy}
            title={busy ? t("auth.signingIn") : undefined}
          />
        </form>
      </div>
    </div>
  );
}
