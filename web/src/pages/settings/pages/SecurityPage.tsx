import { useState } from "react";
import { getAuth, setAuthPassword, explainedError } from "../../../lib/api";
import { PasskeyCard } from "../PasskeyCard";
import { TwoFactorCard } from "../TwoFactorCard";
import { EncryptionCard } from "../EncryptionCard";
import { PhonePairingCard } from "../PhonePairingCard";
import { Button } from "../../../components/Button";
import { RevealInput } from "../../../components/RevealInput";
import { Rows, SettingRow } from "../../../components/storage/rows";
import { useReveal } from "../../../lib/useReveal";
import { useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { useConfirm } from "../../../lib/useConfirm";
import { Card, LOGIN_PASSWORD_FIELD, hueCounter, type SaveState } from "../shared";
import { useSettings } from "../settingsStore";

export function SecurityPage() {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const {
    authEnabled,
    setAuthEnabled,
    totpEnabled,
    setTotpEnabled,
    recoveryLeft,
    setRecoveryLeft,
    minPasswordLen,
  } = useSettings();

  const [pwCurrent, setPwCurrent] = useState("");
  const [pwNew, setPwNew] = useState("");
  const [pwConfirm, setPwConfirm] = useState("");
  const [pwCode, setPwCode] = useState("");
  const [pwSaveState, setPwSaveState] = useState<SaveState>("idle");
  const [pwSaveMsg, setPwSaveMsg] = useState<string | null>(null);
  const [pwSaveShake, setPwSaveShake] = useState(0);
  const revealPwCurrent = useReveal();
  const revealPwNew = useReveal();
  const revealPwConfirm = useReveal();

  // The password keeps a manual Save button. Two fields must agree before a
  // write is safe, so there is no keystroke to auto-save on, and
  // setAuthPassword takes effect at once for the whole instance (a blank
  // password switches login off). A mismatch stays inline next to the fields
  // rather than in a toast that could vanish while the user is still typing.
  async function handleSetPassword() {
    if (pwNew !== pwConfirm) {
      setPwSaveMsg(t("auth.passwordMismatch"));
      setPwSaveState("error");
      return;
    }
    // The server refuses a short password too, and its answer is what the
    // user would eventually see. Checking here as well saves a round trip and
    // puts the message next to the field instead of in a toast. An empty
    // password is not "too short": it means "switch authentication off".
    if (pwNew !== "" && [...pwNew].length < minPasswordLen) {
      setPwSaveMsg(t("auth.passwordMinHint", minPasswordLen));
      setPwSaveState("error");
      setPwSaveShake((n) => n + 1);
      return;
    }
    // A session left open is not enough to change or remove the login.
    if (authEnabled && pwCurrent === "") {
      setPwSaveMsg(t("auth.currentPasswordWrong"));
      setPwSaveState("error");
      setPwSaveShake((n) => n + 1);
      return;
    }
    if (authEnabled && totpEnabled && pwCode.trim() === "") {
      setPwSaveMsg(t("auth.codeHint"));
      setPwSaveState("error");
      setPwSaveShake((n) => n + 1);
      return;
    }
    if (pwNew === "" && authEnabled && !(await confirm(t("auth.turnOffAsk"), { confirmKey: "auth.loginDisable" }))) return;
    setPwSaveState("saving");
    setPwSaveMsg(null);
    try {
      const res = await setAuthPassword(pwNew, pwCurrent, pwCode.trim());
      if (res.ok) {
        setAuthEnabled(res.enabled ?? false);
        // The same response carries the session, so the card can go straight to
        // its signed-in state; otherwise enabling the second factor would answer
        // 401 until a reload, because the new login had issued no session yet.
        setPwSaveState("idle");
        push(pwNew === "" ? t("auth.passwordCleared") : t("auth.passwordSaved"), "success");
        setPwCurrent("");
        setPwNew("");
        setPwConfirm("");
        setPwCode("");
        if (pwNew === "") setTotpEnabled(false);
      } else {
        setPwSaveState("idle");
        const said =
          res.code === "currentPasswordWrong" ? t("auth.currentPasswordWrong") : res.needCode ? t("auth.codeInvalid") : res.error;
        push(said ?? t("auth.saveError"), "fail");
        setPwSaveShake((n) => n + 1);
      }
    } catch (err) {
      setPwSaveState("idle");
      push(explainedError(err) ?? t("auth.saveError"), "fail");
      setPwSaveShake((n) => n + 1);
    }
  }

  const nextHue = hueCounter();

  return (
    <>
      {/* One hueIdx for the heading and the button inside. */}
      {(() => {
        const hueIdx = nextHue();
        return (
      <Card
        title={t("auth.security")}
        hint={t("auth.passwordHint")}
        hueIndex={hueIdx}
        foot={
          <span data-new="card-buttons" className="flex">
            <Button
              key={pwSaveShake || 0}
              label={t("settings.save")}
              labelKey="settings.save"
              tone="accent"
              onClick={() => void handleSetPassword()}
              disabled={pwSaveState === "saving"}
              busy={pwSaveState === "saving"}
              title={pwSaveState === "saving" ? t("auth.saving") : undefined}
              className={pwSaveShake ? "glim-shake" : ""}
              hueIndex={hueIdx}
            />
          </span>
        }
      >
        <div className="flex items-center gap-2">
          <span
            className={`inline-block h-2 w-2 rounded-full ${authEnabled ? "bg-statusOkSolid" : "bg-carbon-textMuted"}`}
          />
          <span className="text-sm text-carbon-text">
            {authEnabled ? t("auth.authOn") : t("auth.authOff")}
          </span>
        </div>

        <div className="flex flex-col gap-3">
          <Rows>
          {authEnabled && (
            <SettingRow label={t("auth.currentPassword")} htmlFor="bv-password-current">
              <RevealInput
                {...revealPwCurrent}
                id="bv-password-current"
                value={pwCurrent}
                onChange={(e) => setPwCurrent(e.target.value)}
                autoComplete="current-password"
                placeholder="••••••••"
                wrapperClassName="w-72 max-w-full"
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
              />
            </SettingRow>
          )}
          <SettingRow label={authEnabled ? t("auth.changePassword") : t("auth.setPassword")} htmlFor={LOGIN_PASSWORD_FIELD}>
            <RevealInput
              {...revealPwNew}
              id={LOGIN_PASSWORD_FIELD}
              value={pwNew}
              onChange={(e) => setPwNew(e.target.value)}
              autoComplete="new-password"
              placeholder="••••••••"
              wrapperClassName="w-72 max-w-full"
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
            />
          </SettingRow>
          <SettingRow label={t("auth.confirmPassword")} htmlFor="bv-password-confirm">
            <RevealInput
              {...revealPwConfirm}
              id="bv-password-confirm"
              value={pwConfirm}
              onChange={(e) => setPwConfirm(e.target.value)}
              autoComplete="new-password"
              placeholder="••••••••"
              wrapperClassName="w-72 max-w-full"
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
            />
          </SettingRow>
          {authEnabled && totpEnabled && (
            <SettingRow label={t("auth.codeLabel")} htmlFor="bv-password-code">
              <input
                id="bv-password-code"
                value={pwCode}
                onChange={(e) => setPwCode(e.target.value)}
                autoComplete="one-time-code"
                placeholder="000000"
                className="w-40 rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm tracking-[0.35em] text-carbon-text glim-field-focus"
              />
            </SettingRow>
          )}
          </Rows>
          {/* The rule, stated before it is broken rather than after. */}
          <span className="text-xs text-carbon-textSub">
            {t("auth.passwordMinHint", minPasswordLen)}
          </span>

          {/* Only the pre-flight validation errors render here; the save
              outcome is a toast. */}
          {pwSaveState === "error" && pwSaveMsg && <span className="text-sm text-statusFail">{pwSaveMsg}</span>}
        </div>

        {/* No sign-out buttons here: a settings card configures and the shell
            operates, and sign-out lives in the sidebar. Changing the password
            rotates the session epoch, which ends every other session (see
            handleSetPassword in internal/api/handlers.go). */}
      </Card>
        );
      })()}

      {/* The second factor has its own card: enrolment takes three steps with a */}
      {/* QR code and recovery codes, and it is a separate decision from having a */}
      {/* password at all. */}
      <TwoFactorCard
        passwordSet={authEnabled}
        enabled={totpEnabled}
        recoveryLeft={recoveryLeft}
        onChanged={() => {
          void getAuth()
            .then((res) => {
              setTotpEnabled(res.totp ?? false);
              setRecoveryLeft(res.recoveryCodesLeft);
            })
            .catch(() => undefined);
        }}
        hueIndex={nextHue()}
      />

      {/* A passkey replaces typing the password rather than strengthening it, */}
      {/* and it is not always available, so the card first explains when it is not. */}
      <PasskeyCard passwordSet={authEnabled} hueIndex={nextHue()} />
      <PhonePairingCard hueIndex={nextHue()} />
      <EncryptionCard hueIndex={nextHue()} />
      {confirmDialog}
    </>
  );
}
