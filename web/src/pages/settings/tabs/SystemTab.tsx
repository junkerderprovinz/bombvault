import { getAuth } from "../../../lib/api";
import { PasskeyCard } from "../PasskeyCard";
import { TwoFactorCard } from "../TwoFactorCard";
import { Button } from "../../../components/Button";
import { RevealInput } from "../../../components/RevealInput";
import { tLtr } from "../../../lib/ltrFragments";
import { SpikePanel } from "../../../components/SpikePanel";
import { Card, LOGIN_PASSWORD_FIELD, ToggleRow } from "../shared";
import { VMSSHCard } from "../VMSSHCard";
import { FleetSettingsCard } from "../FleetSettingsCard";
import { SettingsPortabilityCard } from "../SettingsPortabilityCard";
import { DashboardWidgetCard } from "../DashboardWidgetCard";
import { McpServerCard } from "../McpServerCard";
import { mcpShipped } from "../../../lib/mcpSwitch";
import type { SettingsTabProps } from "./types";

export function SystemTab({
  t,
  advanced,
  settings,
  setSettings,
  savedBaseline,
  authEnabled,
  totpEnabled,
  setTotpEnabled,
  recoveryLeft,
  setRecoveryLeft,
  minPasswordLen,
  pwNew,
  setPwNew,
  pwConfirm,
  setPwConfirm,
  pwSaveState,
  pwSaveMsg,
  pwSaveShake,
  revealPwNew,
  revealPwConfirm,
  revealMetricsToken,
  setMetricsSaveState,
  setMetricsSaveError,
  fieldPulse,
  save,
  fieldBusy,
  fieldShake,
  autoSaveToggle,
  debouncedSave,
  applyImportedSettings,
  handleSetPassword,
}: SettingsTabProps) {
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    <>
      {/* Conditional cards use a plain `&&` rather than the <Advanced>
          wrapper, whose children are built before it decides, so a hidden
          card would still spend a hue slot and shift every later heading. */}
      {advanced && (
      <Card title={t("settings.metrics")} hueIndex={nextHue()}>
        {/* The metrics syntax sits in the toggle's bubble: /metrics and the
            Bearer token are used right at this toggle, so the bubble hides
            nothing a reader needs on another screen. */}
        <ToggleRow
          label={tLtr(t, "settings.metricsEnable")}
          hint={tLtr(t, "settings.metricsHint")}
          checked={settings.metricsEnabled}
          onChange={(v) => void autoSaveToggle("metricsEnabled", v, setMetricsSaveState, setMetricsSaveError)}
          disabled={fieldBusy.metricsEnabled}
          shakeNonce={fieldShake.metricsEnabled}
          pulseNonce={fieldPulse.metricsEnabled}
        />
        {/* Write-only secret: the GET never echoes it and a blank save keeps
            the stored token, so a stored one shows the "saved" placeholder. */}
        <label className="flex flex-col gap-1.5">
          <span className="text-xs text-carbon-textSub">{t("settings.metricsToken")}</span>
          <RevealInput
            {...revealMetricsToken}
            value={settings.metricsToken}
            spellCheck={false}
            autoComplete="off"
            onChange={(e) => {
              const v = e.target.value;
              setSettings((prev) => prev ? { ...prev, metricsToken: v } : prev);
              // A non-blank token marks itself set; a blank save keeps
              // whatever was stored.
              debouncedSave("metricsToken", () =>
                void save(
                  { metricsToken: v, metricsTokenSet: v.trim() !== "" || settings.metricsTokenSet },
                  setMetricsSaveState,
                  setMetricsSaveError
                )
              );
            }}
            placeholder={settings.metricsTokenSet && settings.metricsToken === "" ? t("cloud.secretSet") : ""}
            wrapperClassName="w-full"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus"
          />
        </label>
      </Card>
      )}

      {/* Not behind Advanced: the widget is an end-user feature, unlike the
          metrics card. */}
      <DashboardWidgetCard
        t={t}
        tokenSet={settings.widgetTokenSet}
        onTokenSet={(set) => {
          // The token has its own endpoints, so the saved baseline is updated
          // too; a later save merges onto it and must not carry a stale flag.
          setSettings((prev) => (prev ? { ...prev, widgetTokenSet: set } : prev));
          if (savedBaseline.current) {
            savedBaseline.current = { ...savedBaseline.current, widgetTokenSet: set };
          }
        }}
        hueIndex={nextHue()}
      />
      <FleetSettingsCard
        t={t}
        settings={settings}
        setSettings={setSettings}
        save={save}
        tokenSet={settings.fleetTokenSet}
        onTokenSet={(set) => {
          setSettings((prev) => (prev ? { ...prev, fleetTokenSet: set } : prev));
          if (savedBaseline.current) {
            savedBaseline.current = { ...savedBaseline.current, fleetTokenSet: set };
          }
        }}
        hueIndex={nextHue()}
      />
      {mcpShipped && <McpServerCard hueIndex={nextHue()} passwordSet={authEnabled} />}

      {/* Shown whenever VMs or ZFS are enabled, so the SSH setup their backups
          need is never hidden behind Advanced. */}
      {(advanced || settings.vmsEnabled || settings.zfsEnabled) && <VMSSHCard t={t} hueIndex={nextHue()} />}

      {advanced && (() => {
        // One slot for the heading and the panel's button, so both share a
        // colour.
        const hueIdx = nextHue();
        return (
          <Card title={t("spike.title")} hueIndex={hueIdx}>
            <SpikePanel t={t} hueIndex={hueIdx} />
          </Card>
        );
      })()}

      {(() => {
        const hueIdx = nextHue();
        return (
      <Card title={t("auth.security")} hint={t("auth.passwordHint")} hueIndex={hueIdx}>
        <div className="flex items-center gap-2">
          <span
            className={`inline-block h-2 w-2 rounded-full ${authEnabled ? "bg-statusOkSolid" : "bg-carbon-textMuted"}`}
          />
          <span className="text-sm text-carbon-text">
            {authEnabled ? t("auth.authOn") : t("auth.authOff")}
          </span>
        </div>

        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-carbon-textSub">
              {authEnabled ? t("auth.changePassword") : t("auth.setPassword")}
            </label>
            <RevealInput
              {...revealPwNew}
              id={LOGIN_PASSWORD_FIELD}
              value={pwNew}
              onChange={(e) => setPwNew(e.target.value)}
              autoComplete="new-password"
              placeholder="••••••••"
              wrapperClassName="w-full"
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-carbon-textSub">
              {t("auth.confirmPassword")}
            </label>
            <RevealInput
              {...revealPwConfirm}
              value={pwConfirm}
              onChange={(e) => setPwConfirm(e.target.value)}
              autoComplete="new-password"
              placeholder="••••••••"
              wrapperClassName="w-full"
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
            />
            {/* The rule, stated before it is broken rather than after. */}
            <span className="text-xs text-carbon-textSub">
              {t("auth.passwordMinHint", minPasswordLen)}
            </span>
          </div>

          <div className="flex items-center gap-3 pt-1">
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
            {/* Only the mismatch check shows here; the result of the save is a
                toast (see handleSetPassword). */}
            {pwSaveState === "error" && pwSaveMsg && (
              <span className="text-sm text-statusFail">{pwSaveMsg}</span>
            )}
          </div>
        </div>

        {/* No sign-out here: a settings card configures, the shell operates.
            A password change ends every other session anyway, because
            handleSetPassword rotates the session epoch. */}
      </Card>
        );
      })()}

      {/* Enrolment is a three-step sequence with a QR code and recovery
          codes, a separate decision from whether there is a password. */}
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

      {/* A card of its own: the second factor strengthens the password, a
          passkey replaces typing it. */}
      <PasskeyCard passwordSet={authEnabled} hueIndex={nextHue()} />

      <SettingsPortabilityCard t={t} hueIndex={nextHue()} applyImport={applyImportedSettings} />
    </>
  );
}
