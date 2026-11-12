import { useEffect, useRef, useState, useSyncExternalStore, type FormEvent, type ReactNode } from "react";
import {
  createSignInFlow,
  focusFirstField,
  labelInside,
  resetBlocked,
  showRefusal,
  signInBlocked,
  signInStage,
  type SignInButton,
  type SignInHandlers,
  type SignInText,
} from "../lib/signIn";
import { useReveal } from "../lib/useReveal";
import { Button } from "./Button";
import { InfoBubble } from "./InfoBubble";
import { RevealInput } from "./RevealInput";

// The way into a locked instance, after GlimStone's SignInCard ("The sign-in
// card" in its design language), drawn with BombVault's own controls.

const FIELD = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-2 glim-field-focus";

export interface SignInCardProps extends SignInHandlers {
  /** The app's logo, as an address. It stands 7.5rem tall above the name, as wide as its shape asks. */
  logo: string;
  /** The page's heading. */
  appName: string;
  text: SignInText;
  /** Offers Sign in with a passkey. Pass it only where a passkey can answer on this address. */
  passkey?: boolean;
  /** Offers Forgot password? and the reset form. */
  reset?: boolean;
  /** The shortest new password the reset form takes, as the server says. */
  minPassword?: number;
  /** A step the page runs itself, in place of the card's forms. */
  step?: ReactNode;
  /** The start of the fields' ids: `-password`, `-code`, `-reset-code`, `-new` and `-confirm` follow it. */
  idPrefix: string;
}

// A field on the sign-in card. Its label stands inside the field until the
// first key, and above it when the field brings a placeholder or a hint.
export function SignInField({ label, htmlFor, hint, hidden, children }: { label: string; htmlFor: string; hint?: string; hidden?: boolean; children: ReactNode }) {
  const [inside, setInside] = useState(false);
  const was = useRef<string | undefined>(undefined);
  useEffect(() => {
    setInside(labelInside(document.getElementById(htmlFor) as HTMLInputElement | null, label, hint, was.current));
    was.current = label;
  }, [htmlFor, label, hint]);
  return (
    <div className="glim-signin-field" hidden={hidden}>
      <label htmlFor={htmlFor} className={inside ? "glim-sr-only" : "glim-signin-label"}>
        {label}
        {hint && <InfoBubble tip={hint} />}
      </label>
      {children}
    </div>
  );
}

export function SignInCard({
  logo,
  appName,
  text,
  passkey = false,
  reset = false,
  minPassword = 12,
  step,
  idPrefix,
  ...handlers
}: SignInCardProps) {
  const latest = useRef({ ...handlers, text, minPassword });
  latest.current = { ...handlers, text, minPassword };
  const [flow] = useState(() => createSignInFlow(() => latest.current));
  const s = useSyncExternalStore(flow.subscribe, flow.state, flow.state);
  const id = (name: string) => `${idPrefix}-${name}`;
  const revealPassword = useReveal();
  const revealNew = useReveal();
  const revealConfirm = useReveal();
  const card = useRef<HTMLDivElement>(null);
  const buttons = useRef<Partial<Record<SignInButton, HTMLButtonElement | null>>>({});
  const hold = (button: SignInButton) => (el: HTMLButtonElement | null) => {
    buttons.current[button] = el;
  };

  const stage = step ? "step" : signInStage(s);
  useEffect(() => focusFirstField(card.current), [stage]);

  const refusal = s.refusal;
  useEffect(
    () => (refusal ? showRefusal(card.current, refusal, buttons.current[refusal.button] ?? null) : undefined),
    // Each refusal is answered once, when its count changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [refusal?.count],
  );

  const submit = (e: FormEvent) => {
    e.preventDefault();
    void flow.submit();
  };
  const error = s.error && (
    <p className="glim-signin-error" role="alert">
      {s.error}
    </p>
  );
  const back = <Button label={text.back} labelKey="common.back" tone="subtle" disabled={!!s.busy} onClick={flow.back} />;

  return (
    <div className="glim-signin">
      <div ref={card} className="glim-card glim-signin-card">
        <div className="glim-signin-head">
          <span className="glim-signin-logo" aria-hidden="true">
            <img src={logo} alt="" aria-hidden="true" draggable={false} />
          </span>
          <h1 className="glim-signin-name">{appName}</h1>
        </div>

        {step ??
          (s.view === "reset" ? (
            <form className="glim-signin-form" noValidate onSubmit={submit}>
              <SignInField label={text.resetCode} htmlFor={id("reset-code")} hint={text.resetCodeTip}>
                <input
                  id={id("reset-code")}
                  data-signin-field="resetCode"
                  className={`glim-signin-log-code w-full ${FIELD}`}
                  value={s.resetCode}
                  onChange={(e) => flow.set("resetCode", e.target.value)}
                  autoComplete="one-time-code"
                  autoCapitalize="characters"
                  spellCheck={false}
                  placeholder="XXXX-XXXX-XXXX"
                />
              </SignInField>
              <SignInField label={text.newPassword} htmlFor={id("new")}>
                <RevealInput
                  {...revealNew}
                  id={id("new")}
                  data-signin-field="newPassword"
                  value={s.newPassword}
                  onChange={(e) => flow.set("newPassword", e.target.value)}
                  autoComplete="new-password"
                  wrapperClassName="w-full"
                  className={FIELD}
                />
              </SignInField>
              <SignInField label={text.confirmPassword} htmlFor={id("confirm")}>
                <RevealInput
                  {...revealConfirm}
                  id={id("confirm")}
                  data-signin-field="confirm"
                  value={s.confirm}
                  onChange={(e) => flow.set("confirm", e.target.value)}
                  autoComplete="new-password"
                  wrapperClassName="w-full"
                  className={FIELD}
                />
              </SignInField>
              {error}
              {/* Stacked, since the longer translations of the two labels do not fit side by side. */}
              <Button
                ref={hold("set-password")}
                label={text.setPassword}
                labelKey="auth.setPassword"
                tone="accent"
                type="submit"
                disabled={resetBlocked(s)}
                busy={s.busy === "setPassword"}
                title={s.busy === "setPassword" ? text.saving : undefined}
              />
              {back}
            </form>
          ) : (
            <form className="glim-signin-form" noValidate onSubmit={submit}>
              {/* Kept while a passkey asks for the code, so the password survives a step back. */}
              <SignInField label={text.password} htmlFor={id("password")} hidden={s.viaPasskey}>
                <RevealInput
                  {...revealPassword}
                  id={id("password")}
                  data-signin-field="password"
                  value={s.password}
                  onChange={(e) => flow.set("password", e.target.value)}
                  autoComplete="current-password"
                  wrapperClassName="w-full"
                  className={FIELD}
                />
              </SignInField>
              {s.needCode && (
                <div className="glim-signin-field">
                  <label htmlFor={id("code")} className="glim-signin-label">
                    {text.code}
                  </label>
                  <input
                    id={id("code")}
                    data-signin-field="code"
                    className={`glim-signin-code w-full ${FIELD}`}
                    value={s.code}
                    onChange={(e) => flow.set("code", e.target.value)}
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    spellCheck={false}
                    placeholder="000000"
                  />
                  <p className="glim-signin-hint">{text.codeHint}</p>
                </div>
              )}
              {error}
              <Button
                ref={hold("submit")}
                label={text.signIn}
                labelKey="auth.signIn"
                tone="accent"
                type="submit"
                disabled={signInBlocked(s)}
                busy={s.busy === "signIn"}
                title={s.busy === "signIn" ? text.signingIn : undefined}
              />
              {/* Left out rather than disabled where it cannot work: a disabled
                  control on a sign-in page reads as being locked out. */}
              {!s.needCode && passkey && (
                <Button
                  ref={hold("passkey")}
                  label={text.passkey}
                  labelKey="auth.signInWithPasskey"
                  disabled={!!s.busy}
                  busy={s.busy === "passkey"}
                  onClick={() => void flow.passkey()}
                />
              )}
              {s.needCode
                ? back
                : reset && (
                    <Button
                      ref={hold("forgot")}
                      label={text.forgot}
                      labelKey="auth.forgotPassword"
                      tone="subtle"
                      disabled={!!s.busy}
                      busy={s.busy === "askCode"}
                      onClick={() => void flow.forgot()}
                    />
                  )}
            </form>
          ))}
      </div>
    </div>
  );
}
