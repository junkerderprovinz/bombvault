import { useCallback, useEffect, useState } from "react";
import { Button } from "../components/Button";
import { InfoBubble } from "../components/InfoBubble";
import { Toggle } from "../components/Toggle";
import { answerOAuthConsent, getAuth, getOAuthConsent, type OAuthConsentInfo } from "../lib/api";
import { useT, type TranslationKey } from "../lib/i18n";
import { OTHER_CLIENT, clientById } from "../lib/mcpClients";
import { ClientMark } from "./settings/McpClientMark";
import { LoginPage } from "./Login";

/** The refusals of the consent endpoints, each with the sentence it gets here. */
const REFUSAL: Record<string, TranslationKey> = {
  "oauth-off": "oauth.errorOff",
  "oauth-unknown-client": "oauth.errorUnknownClient",
  "oauth-bad-redirect": "oauth.errorBadRedirect",
  "oauth-invalid-request": "oauth.errorInvalid",
  "oauth-consent-expired": "oauth.errorExpired",
  "oauth-busy": "oauth.errorBusy",
  "mcp-grant-limit": "oauth.limitReached",
};

function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return "";
  }
}

type Gate = "loading" | "pass" | "blocked";

/** How long Allow stays disabled after the page shows or comes back to the
 *  front. A page opened under the pointer, or raised by another site, would
 *  otherwise take the second click of a double click meant for something else. */
const ALLOW_PAUSE_MS = 1000;

/**
 * OAuthConsent is the page a client sends the operator to when it signs in
 * through OAuth. It needs a real session, so an operator who is not signed in
 * sees the login form first, second factor included. The page shows who asks,
 * where the answer goes and what the client may do, and nothing leaves it until
 * the operator presses Allow or Deny. A request the server refuses stays here
 * as a message: its return address is whatever the client registered, which
 * can be anybody's.
 */
export function OAuthConsent() {
  const { t } = useT();
  const [gate, setGate] = useState<Gate>("loading");
  const [info, setInfo] = useState<OAuthConsentInfo | null>(null);
  const [problem, setProblem] = useState<TranslationKey | null>(null);
  const [allowStart, setAllowStart] = useState(false);
  const [busy, setBusy] = useState<"allow" | "deny" | null>(null);
  const [leaving, setLeaving] = useState<string | null>(null);
  const [armed, setArmed] = useState(false);

  const checkAuth = useCallback(() => {
    getAuth()
      .then((res) => setGate(res.enabled && !res.authed ? "blocked" : "pass"))
      .catch(() => setGate("pass"));
  }, []);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  const leave = useCallback((url: string) => {
    setLeaving(hostOf(url));
    window.location.assign(url);
  }, []);

  useEffect(() => {
    if (gate !== "pass") return;
    let live = true;
    getOAuthConsent(window.location.search)
      .then((res) => {
        if (!live) return;
        if (res.ok) {
          setInfo(res);
          return;
        }
        setProblem(REFUSAL[res.code ?? ""] ?? "oauth.errorFailed");
      })
      .catch(() => {
        if (live) setProblem("oauth.errorFailed");
      });
    return () => {
      live = false;
    };
  }, [gate]);

  useEffect(() => {
    if (info === null) return;
    let timer = window.setTimeout(() => setArmed(true), ALLOW_PAUSE_MS);
    const pause = () => {
      setArmed(false);
      window.clearTimeout(timer);
      timer = window.setTimeout(() => setArmed(true), ALLOW_PAUSE_MS);
    };
    window.addEventListener("focus", pause);
    document.addEventListener("visibilitychange", pause);
    return () => {
      window.clearTimeout(timer);
      window.removeEventListener("focus", pause);
      document.removeEventListener("visibilitychange", pause);
    };
  }, [info]);

  async function answer(allow: boolean) {
    if (!info?.ticket) return;
    setBusy(allow ? "allow" : "deny");
    try {
      const res = await answerOAuthConsent(info.ticket, allow, allowStart);
      if (res.ok && res.redirect) {
        leave(res.redirect);
        return;
      }
      setInfo(null);
      setProblem(REFUSAL[res.code ?? ""] ?? "oauth.errorFailed");
    } catch {
      setInfo(null);
      setProblem("oauth.errorFailed");
    } finally {
      setBusy(null);
    }
  }

  if (gate === "loading") return null;
  if (gate === "blocked") return <LoginPage onLogin={checkAuth} />;

  const client = info?.client;
  const known = clientById(client?.known);
  const name = known?.name ?? (client?.name || t("oauth.unnamedClient"));

  return (
    <div className="flex min-h-screen items-center justify-center bg-carbon-background p-4">
      <main className="flex w-full max-w-md flex-col gap-6 rounded-card bg-carbon-surface p-8 shadow-lg [--mark-ground:var(--carbon-surface)]">
        <div className="flex flex-col items-center gap-3 text-center">
          <span className="flex h-16 w-16 items-center justify-center">
            <img
              src="/logo.svg"
              alt=""
              aria-hidden="true"
              draggable={false}
              className="block h-16 w-16 object-contain dark:hidden"
            />
            <img
              src="/logo-light.svg"
              alt=""
              aria-hidden="true"
              draggable={false}
              className="hidden h-16 w-16 object-contain dark:block"
            />
          </span>
          <h1 className="text-2xl font-semibold text-carbon-text">{t("oauth.heading")}</h1>
        </div>

        {leaving !== null && (
          <p role="status" className="text-center text-sm text-carbon-textSub">
            {t("oauth.returning").replace("{host}", leaving)}
          </p>
        )}

        {problem !== null && (
          <p role="alert" className="rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-sm leading-relaxed text-carbon-text">
            {t(problem)}
          </p>
        )}

        {info !== null && client && leaving === null && (
          <>
            <div className="flex items-center gap-3">
              <span className="glim-client-mark glim-client-mark-lg" aria-hidden="true">
                <ClientMark client={known ?? OTHER_CLIENT} />
              </span>
              {/* Nothing here is cut short: a lookalike host differs in the part an ellipsis would hide. */}
              <span className="flex min-w-0 flex-col">
                <span className="break-words text-sm font-semibold text-carbon-text">{name}</span>
                <span dir="ltr" className="break-all text-start text-xs text-carbon-textSub">
                  {client.redirectHost}
                </span>
              </span>
            </div>

            <div className="flex flex-col gap-3 text-sm leading-relaxed text-carbon-text">
              <p>{t("oauth.asks").replace("{name}", name)}</p>
              <p className="text-carbon-textSub">{t("oauth.reads")}</p>
              <p className="text-carbon-textSub">
                {known ? t("oauth.startedHere").replace("{name}", known.name) : t("oauth.unverified")}
              </p>
              {client.loopback && (
                <p className="rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-carbon-text">{t("oauth.loopback")}</p>
              )}
              {info.limitReached && (
                <p className="rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-carbon-text">{t("oauth.limitReached")}</p>
              )}
            </div>

            <span className="flex items-center gap-1.5">
              <Toggle label={t("mcp.allowStart")} checked={allowStart} onChange={setAllowStart} disabled={busy !== null} />
              <InfoBubble tip={t("oauth.allowStartHint")} />
            </span>

            <div className="flex flex-col gap-3">
              <p className="text-xs text-carbon-textMuted">
                {t("oauth.returnsTo").replace("{host}", client.redirectHost)}
              </p>
              <div className="flex flex-wrap items-center justify-end gap-3">
                <Button
                  label={t("oauth.decline")}
                  labelKey="oauth.decline"
                  tone="neutral"
                  onClick={() => void answer(false)}
                  disabled={busy !== null}
                  busy={busy === "deny"}
                />
                <Button
                  label={t("oauth.accept")}
                  labelKey="oauth.accept"
                  tone="accent"
                  onClick={() => void answer(true)}
                  disabled={busy !== null || info.limitReached === true || !armed}
                  busy={busy === "allow"}
                />
              </div>
            </div>
          </>
        )}
      </main>
    </div>
  );
}
