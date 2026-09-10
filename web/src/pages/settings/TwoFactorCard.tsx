// TwoFactorCard — the second login factor, v8.6.0.
//
// The card walks one line: OFF, or a three-step enrolment (scan, prove, write
// the recovery codes down), or ON. Enrolment is a sequence, so the card renders
// the step it is on rather than every control at once with most of them
// disabled.
//
// Two rules the interface has to carry, because they are not obvious and the
// consequence of missing either is being locked out of your own backups:
//   - the recovery codes are shown exactly once, so the card says so before
//     it shows them and asks for an acknowledgement before it puts them away;
//   - the factor is not on until a code from the app has been accepted, so the
//     status line never claims it is armed while the enrolment is half done.

import { useState } from "react";
import { Button } from "../../components/Button";
import { QRCode } from "../../components/QRCode";
import { IconCopy } from "../../components/navGlyphs";
import { confirmTOTP, disableTOTP, setupTOTP } from "../../lib/api";
import { copyText } from "../../lib/clipboard";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { Card } from "./shared";

type Step =
  | { kind: "idle" }
  | { kind: "scan"; secret: string; uri: string }
  | { kind: "codes"; codes: string[] };

export function TwoFactorCard({
  /** Whether a login password exists at all. A second factor without a first
   *  one protects nothing, and the server refuses to set one up. */
  passwordSet,
  enabled,
  recoveryLeft,
  onChanged,
  hueIndex,
}: {
  passwordSet: boolean;
  enabled: boolean;
  recoveryLeft?: number;
  /** Called after the factor is armed or removed so the page can re-read auth state. */
  onChanged: () => void;
  hueIndex?: number;
}) {
  const { t } = useT();
  const { push } = useToast();
  const [step, setStep] = useState<Step>({ kind: "idle" });
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [disarming, setDisarming] = useState(false);

  async function begin() {
    setBusy(true);
    try {
      const res = await setupTOTP();
      if (res.ok && res.secret && res.uri) {
        setStep({ kind: "scan", secret: res.secret, uri: res.uri });
        setCode("");
      } else {
        push(res.error ?? t("auth.saveError"), "fail");
      }
    } catch {
      push(t("auth.saveError"), "fail");
    } finally {
      setBusy(false);
    }
  }

  async function confirm() {
    setBusy(true);
    try {
      const res = await confirmTOTP(code);
      if (res.ok && res.recoveryCodes) {
        setStep({ kind: "codes", codes: res.recoveryCodes });
        setCode("");
        push(t("auth.twoFactorOnNow"), "success");
        onChanged();
      } else {
        push(res.error ?? t("auth.codeInvalid"), "fail");
      }
    } catch {
      push(t("auth.saveError"), "fail");
    } finally {
      setBusy(false);
    }
  }

  async function disable() {
    setBusy(true);
    try {
      const res = await disableTOTP(code);
      if (res.ok) {
        setStep({ kind: "idle" });
        setCode("");
        setDisarming(false);
        push(t("auth.twoFactorOffNow"), "success");
        onChanged();
      } else {
        push(res.error ?? t("auth.codeInvalid"), "fail");
      }
    } catch {
      push(t("auth.saveError"), "fail");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card title={t("auth.twoFactor")} hint={t("auth.twoFactorHint")} hueIndex={hueIndex}>
      {/* Status line. It reads the server's answer, never the local step, so a
          half-finished enrolment cannot make it claim the factor is armed. */}
      <div className="flex items-center gap-2">
        <span
          className={`inline-block h-2 w-2 rounded-full ${enabled ? "bg-statusOkSolid" : "bg-carbon-textMuted"}`}
        />
        <span className="text-sm text-carbon-text">
          {enabled ? t("auth.twoFactorOn") : t("auth.twoFactorOff")}
        </span>
      </div>

      {!passwordSet && (
        <p className="text-sm text-carbon-textSub">{t("auth.twoFactorNeedsPassword")}</p>
      )}

      {/* OFF: one button starts the enrolment. */}
      {passwordSet && !enabled && step.kind === "idle" && (
        // self-start, or the Card's flex column stretches it across the full
        // width: a lone button in a column has nothing beside it to size
        // against. Every other button in this card sits in a flex row and was
        // therefore already the width of its own words.
        <Button
          label={t("auth.twoFactorEnable")}
          labelKey="auth.twoFactorEnable"
          tone="accent"
          onClick={() => void begin()}
          disabled={busy}
          busy={busy}
          hueIndex={hueIndex}
          className="self-start"
        />
      )}

      {/* Step 1 and 2: scan, then prove. Both on screen at once, because the
          code has to be typed while the app is still open on the same page. */}
      {step.kind === "scan" && (
        <div className="flex flex-col gap-4">
          <p className="text-sm text-carbon-text">{t("auth.scanQR")}</p>
          <QRCode value={step.uri} size={196} className="rounded-control self-start" />
          <div className="flex flex-col gap-1.5">
            <span className="text-xs text-carbon-textSub">{t("auth.secretManual")}</span>
            <div className="flex items-center gap-2">
              <code className="rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm tracking-widest text-carbon-text break-all">
                {step.secret}
              </code>
              <Button
                label={t("common.copy")}
                labelKey="common.copy"
                tone="neutral"
                glyph={<IconCopy />}
                onClick={() => void copyText(step.secret)}
                hueIndex={hueIndex}
              />
            </div>
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="bv-totp-confirm" className="text-xs text-carbon-textSub">
              {t("auth.confirmCode")}
            </label>
            <input
              id="bv-totp-confirm"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              inputMode="numeric"
              autoComplete="one-time-code"
              placeholder="000000"
              className="w-40 rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm tracking-[0.35em] text-carbon-text glim-field-focus"
            />
          </div>
          <div className="flex items-center gap-3">
            <Button
              label={t("auth.twoFactorConfirm")}
              labelKey="auth.twoFactorConfirm"
              tone="accent"
              onClick={() => void confirm()}
              disabled={busy || code.trim() === ""}
              busy={busy}
              hueIndex={hueIndex}
            />
            <Button
              label={t("common.cancel")}
              labelKey="common.cancel"
              tone="neutral"
              onClick={() => {
                setStep({ kind: "idle" });
                setCode("");
              }}
              hueIndex={hueIndex}
            />
          </div>
        </div>
      )}

      {/* Step 3: the codes, once. */}
      {step.kind === "codes" && (
        <div className="flex flex-col gap-3">
          <p className="text-sm font-medium text-carbon-text">{t("auth.recoveryTitle")}</p>
          <p className="text-sm text-carbon-textSub">{t("auth.recoveryHint")}</p>
          <ul className="grid grid-cols-2 gap-x-6 gap-y-1 rounded-control bg-carbon-surface2 p-4 font-mono text-sm text-carbon-text">
            {step.codes.map((c) => (
              <li key={c}>{c}</li>
            ))}
          </ul>
          <div className="flex items-center gap-3">
            <Button
              label={t("common.copy")}
              labelKey="common.copy"
              tone="neutral"
              glyph={<IconCopy />}
              onClick={() => void copyText(step.codes.join("\n"))}
              hueIndex={hueIndex}
            />
            <Button
              label={t("auth.recoverySaved")}
              labelKey="auth.recoverySaved"
              tone="accent"
              onClick={() => setStep({ kind: "idle" })}
              hueIndex={hueIndex}
            />
          </div>
        </div>
      )}

      {/* ON: how many codes are left, and the way out. Turning it off needs a
          live code, so a session somebody walked away from cannot remove it. */}
      {enabled && step.kind === "idle" && (
        <div className="flex flex-col gap-3">
          {recoveryLeft !== undefined && (
            <p className="text-sm text-carbon-textSub">
              {t("auth.recoveryLeft").replace("{n}", String(recoveryLeft))}
            </p>
          )}
          {!disarming ? (
            <Button
              label={t("auth.twoFactorDisable")}
              labelKey="auth.twoFactorDisable"
              tone="neutral"
              onClick={() => setDisarming(true)}
              hueIndex={hueIndex}
            />
          ) : (
            <div className="flex flex-col gap-2">
              <label htmlFor="bv-totp-disable" className="text-xs text-carbon-textSub">
                {t("auth.disableCodePrompt")}
              </label>
              <input
                id="bv-totp-disable"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                inputMode="numeric"
                autoComplete="one-time-code"
                placeholder="000000"
                className="w-40 rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm tracking-[0.35em] text-carbon-text glim-field-focus"
              />
              <div className="flex items-center gap-3">
                <Button
                  label={t("auth.twoFactorDisable")}
                  labelKey="auth.twoFactorDisable"
                  tone="accent"
                  onClick={() => void disable()}
                  disabled={busy || code.trim() === ""}
                  busy={busy}
                  hueIndex={hueIndex}
                />
                <Button
                  label={t("common.cancel")}
                  labelKey="common.cancel"
                  tone="neutral"
                  onClick={() => {
                    setDisarming(false);
                    setCode("");
                  }}
                  hueIndex={hueIndex}
                />
              </div>
            </div>
          )}
        </div>
      )}
    </Card>
  );
}
