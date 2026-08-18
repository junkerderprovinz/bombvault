import { useState } from "react";
import { login } from "../lib/api";
import { useT } from "../lib/i18n";
import { RevealInput } from "../components/RevealInput";
import { useReveal } from "../lib/useReveal";

interface LoginPageProps {
  /** Called after a successful login so the parent can re-check auth state. */
  onLogin: () => void;
}

// ---------------------------------------------------------------------------
// LoginPage — full-screen centered login form, shown when auth is ON + not authed.
// ---------------------------------------------------------------------------

export function LoginPage({ onLogin }: LoginPageProps) {
  const { t } = useT();
  const reveal = useReveal();
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      const res = await login(password);
      if (res.ok) {
        onLogin();
      } else {
        setError(res.error ?? t("auth.invalidPassword"));
      }
    } catch {
      setError(t("auth.loginError"));
    } finally {
      setBusy(false);
    }
  }

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
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-2 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
            />
          </div>

          {/* Error message */}
          {error && (
            <p className="text-xs text-statusFail" role="alert">
              {error}
            </p>
          )}

          {/* Submit button */}
          <button
            type="submit"
            disabled={busy || password === ""}
            className="inline-flex items-center justify-center gap-2 rounded-control bg-accent px-4 py-2 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {busy ? (
              <>
                <span
                  className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
                  style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
                />
                {t("auth.signingIn")}
              </>
            ) : (
              t("auth.signIn")
            )}
          </button>
        </form>
      </div>
    </div>
  );
}
