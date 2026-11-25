import { useEffect, useMemo, useState } from "react";
import { signInCardHandlers, signInServiceLabel, takeSignInCode, type SignInOffer, type SignInRemoved } from "@glimstone/core";
import { IDENTITY_PROVIDERS } from "@glimstone/core/identityProviders";
import { SignInCard, type SignInCardService } from "@glimstone/react/SignInCard";
import { Button } from "../components/Button";
import { withinBase } from "../lib/base";
import { useT } from "../lib/i18n";
import { signIn } from "../lib/signInKit";
import { arrivedWords, cardText, errorWords, resetNote } from "../lib/signInWords";

interface LoginPageProps {
  /** Called once somebody is signed in, so the parent reads the sign-in again. */
  onLogin: () => void;
}

const CLOSED: SignInOffer = { open: false, owner: true, minPassword: 12, signedIn: false, passkeys: false, services: [] };

// LoginPage is the way in while somebody has to sign in: GlimStone's sign-in card on the kit's sign-in. The sign-in
// service that is on stands under Sign in, and passkeys come through the browser's offer in the password field.
export function LoginPage({ onLogin }: LoginPageProps) {
  const { t, lang } = useT();
  const [offer, setOffer] = useState<SignInOffer | null>(null);
  // What a reset took away, held until the reader has seen it.
  const [removed, setRemoved] = useState<SignInRemoved | null>(null);
  const [refused] = useState(() => {
    const code = takeSignInCode();
    return code ? (({ ...errorWords(t), ...arrivedWords(t) }) as Record<string, string | undefined>)[code] ?? t("signin.failed") : undefined;
  });

  useEffect(() => {
    let live = true;
    void signIn.options().then((r) => {
      if (live) setOffer(r.ok ? r.value : CLOSED);
    });
    return () => {
      live = false;
    };
  }, []);

  const text = useMemo(() => cardText(t, lang), [t, lang]);
  const handlers = useMemo(
    () =>
      signInCardHandlers(signIn, {
        words: { errors: errorWords(t), failed: t("signin.failed") },
        onSignedIn: onLogin,
        onReset: setRemoved,
      }),
    [t, onLogin],
  );

  const first = offer?.services[0];
  const service = useMemo<SignInCardService | undefined>(() => {
    if (!first) return undefined;
    const { name, mark } = signInServiceLabel(first, IDENTITY_PROVIDERS, { services: { unnamed: t("signin.tile.services.unnamed") } });
    return mark ? { name, mark } : { name };
  }, [first, t]);

  if (!offer) return null;
  return (
    <SignInCard
      logo={withinBase("/logo.svg")}
      appName={t("auth.loginTitle")}
      text={text}
      idPrefix="bv"
      reset
      minPassword={offer.minPassword}
      error={refused}
      service={service}
      onService={first ? () => location.assign(signIn.serviceAddress(first.id, location.pathname)) : undefined}
      step={removed ? <ResetDone note={resetNote(t, lang, removed)} onDone={onLogin} /> : undefined}
      onSignIn={handlers.onSignIn}
      onPasskeyOffer={offer.passkeys ? handlers.onPasskeyOffer : undefined}
      onAskCode={handlers.onAskCode}
      onSetPassword={handlers.onSetPassword}
    />
  );
}

/** ResetDone says what the reset took away before the app opens. The owner has to know that the passkeys and keys
 *  are gone, or the next sign-in looks for a key that no longer exists. */
function ResetDone({ note, onDone }: { note: string; onDone: () => void }) {
  const { t } = useT();
  return (
    <div className="glim-signin-form" role="status">
      <h2 className="text-sm font-semibold text-carbon-text">{t("auth.resetDoneTitle")}</h2>
      <p className="text-xs text-carbon-textSub font-medium">{note}</p>
      <Button label={t("common.done")} labelKey="common.done" tone="accent" onClick={onDone} />
    </div>
  );
}
