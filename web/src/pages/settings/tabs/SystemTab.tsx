import { getAuth } from "../../../lib/api";
import { PasskeyCard } from "../PasskeyCard";
import { TwoFactorCard } from "../TwoFactorCard";
import { Button } from "../../../components/Button";
import { RevealInput } from "../../../components/RevealInput";
import { tLtr } from "../../../lib/ltrFragments";
import { SpikePanel } from "../../../components/SpikePanel";
import { Card, ToggleRow } from "../shared";
import { VMSSHCard } from "../VMSSHCard";
import { FleetSettingsCard } from "../FleetSettingsCard";
import { SettingsPortabilityCard } from "../SettingsPortabilityCard";
import { DashboardWidgetCard } from "../DashboardWidgetCard";
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
      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM: Monitoring (Prometheus)                                    */}
      {/* ------------------------------------------------------------------ */}
      {/* `advanced &&` inline, not the <Advanced> wrapper, same reason as
          the Storage tab's cacheTitle Card above. */}
      {advanced && (
      <Card title={t("settings.metrics")} hueIndex={nextHue()}>
        {/* GlimStone follow-up round (jdp, live review: "Prometheus-Metriken
            unter /metrics ... in eine InfoBubble", design-language.md rule 8,
            "explanations live in a bubble, not on the page"): this used to be
            a permanent `<p>` under the Card title, reasoned at the time as an
            "exact syntax to copy correctly" carve-out (the same one RcloneCard's/
            CloudCard's own hints still use). jdp's live review overruled that
            specifically for this text, unlike rclone.pathHint's own
            "rclone:<remote>:<bucket>/path" syntax (which someone fills into a
            DIFFERENT tab's Backup Path field from memory, so it needs to stay
            findable without already hovering an icon here), this hint is
            self-contained: /metrics and the Bearer-token syntax are both used
            right here, on the same toggle, so a hover bubble is not hiding
            anything a reader would need on a different screen. Moved onto the
            ToggleRow's own `hint` prop below (the same "(i) beside the label"
            mechanism as every other bubbled explanation in this file), no
            `description` here for the same "the Card's own hint already
            covers it" reasoning this row's OLD comment gave, just now living
            on the toggle's `hint` instead of a Card-level paragraph. */}
        <ToggleRow
          label={tLtr(t, "settings.metricsEnable")}
          hint={tLtr(t, "settings.metricsHint")}
          checked={settings.metricsEnabled}
          onChange={(v) => void autoSaveToggle("metricsEnabled", v, setMetricsSaveState, setMetricsSaveError)}
          disabled={fieldBusy.metricsEnabled}
          shakeNonce={fieldShake.metricsEnabled}
          pulseNonce={fieldPulse.metricsEnabled}
        />
        {/* Write-only secret (the GET never echoes it): blank-on-save keeps the
            stored token, so a stored one shows as the same "saved, leave blank
            to keep" placeholder the cloud-credential secrets use. */}
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
              // Full-page Speichern-Button sweep: was this Card's own bottom
              // SaveBar. Keeps the SAME "is-set flag honest locally" patch
              // shape the old onSave sent, a non-blank token being saved
              // marks itself set; a blank save keeps whatever was stored.
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

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM: Dashboard widget (embeddable activity log). Not behind       */}
      {/* Advanced: it is an end-user feature, unlike the ops-y metrics card.  */}
      {/* ------------------------------------------------------------------ */}
      <DashboardWidgetCard
        t={t}
        tokenSet={settings.widgetTokenSet}
        onTokenSet={(set) => {
          // Keep BOTH the live state and the saved baseline in sync: the token
          // is managed by its own endpoints, so a later save (which merges onto
          // the baseline) must not carry a stale widgetTokenSet.
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

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM: VM Backup over SSH                                         */}
      {/* Advanced, OR shown whenever VMs are enabled so the SSH setup you    */}
      {/* need to make VM backups work is never hidden behind Advanced.       */}
      {/* ------------------------------------------------------------------ */}
      {(advanced || settings.vmsEnabled) && <VMSSHCard t={t} hueIndex={nextHue()} />}

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM: Spike (host-integration check; KEEP, it is LIVE).           */}
      {/* ------------------------------------------------------------------ */}
      {/* `advanced &&` inline, not the <Advanced> wrapper component: the
          wrapper takes `children` as an ALREADY-BUILT prop, so a
          hueIndex={nextHue()} inside it would fire every render regardless
          of whether Advanced ends up showing it, caught live (Playwright
          against the real deployed container: this exact site, plus three
          more of the same shape, cacheTitle/offsiteLimits/metrics above,
          were each silently "spending" a hue slot on a Card that never
          painted, shifting every later heading on that tab by one position
          while Advanced was off). Plain `&&` short-circuits correctly,
          exactly like every other conditional Card on this page, this was
          the one call site that still used the wrapper component instead. */}
      {advanced && (() => {
        // Button-size/colour-engine sweep (jdp, live review: "Die vielen
        // Buttons sind unterschiedlich groß und nicht alle im
        // Regenbogenmodus"): the Check Now button inside SpikePanel had no
        // tie to this Card's own hueIndex at all. `hueIdx` captured once in
        // this IIFE and threaded into BOTH the Card's own heading notch and
        // SpikePanel's new `hueIndex` prop, the same "one Card, two
        // hue-aware children share ONE position" shape the schedulesChecks
        // Card's own IIFE below already uses for its Card+CadenceBuilder
        // pair, not a second independent `nextHue()` call.
        const hueIdx = nextHue();
        return (
          <Card title={t("spike.title")} hueIndex={hueIdx}>
            <SpikePanel t={t} hueIndex={hueIdx} />
          </Card>
        );
      })()}

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM: Security                                                   */}
      {/* ------------------------------------------------------------------ */}
      {/* Button-size/colour-engine sweep (jdp, live review: "Die vielen
          Buttons sind unterschiedlich groß und nicht alle im
          Regenbogenmodus"): the buttons below had no tie to this Card's own
          hue at all. (It used to name three - Save, Logout and
          Logout-everywhere; the two sign-out buttons are gone, see the note
          where they stood.) IIFE captures `hueIdx`
          once and reuses it for both the Card's own heading notch and every
          button inside it, the same "one Card, several hue-aware children
          share ONE position" shape the schedulesChecks/Spike Cards above
          already use, not several independent `nextHue()` calls. */}
      {(() => {
        const hueIdx = nextHue();
        return (
      <Card title={t("auth.security")} hint={t("auth.passwordHint")} hueIndex={hueIdx}>
        {/* Status badge */}
        <div className="flex items-center gap-2">
          <span
            className={`inline-block h-2 w-2 rounded-full ${authEnabled ? "bg-statusOkSolid" : "bg-carbon-textMuted"}`}
          />
          <span className="text-sm text-carbon-text">
            {authEnabled ? t("auth.authOn") : t("auth.authOff")}
          </span>
        </div>

        {/* Set / Change password form */}
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-carbon-textSub">
              {authEnabled ? t("auth.changePassword") : t("auth.setPassword")}
            </label>
            <RevealInput
              {...revealPwNew}
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
            {/* The rule, stated before it is broken rather than after. There
                was no minimum at all before v8.6.0, and "1234" was accepted. */}
            <span className="text-xs text-carbon-textSub">
              {t("auth.passwordMinHint", minPasswordLen)}
            </span>
          </div>

          {/* Save / status row */}
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
            {/* Only the pre-flight mismatch validation error renders here now
                (GlimStone form-engine Task 9), the post-save success/failure
                notice is a toast instead; see handleSetPassword's own comment. */}
            {pwSaveState === "error" && pwSaveMsg && (
              <span className="text-sm text-statusFail">{pwSaveMsg}</span>
            )}
          </div>
        </div>

        {/* No sign-out here, and no "sign out everywhere" either (GlimStone
            2.1.0, rule 22: a settings card CONFIGURES, the shell OPERATES).
            Both used to sit along this card's bottom edge, which put the two
            controls that throw a half-filled password form away directly under
            the field somebody was typing in - and the plain one duplicated the
            sidebar's own sign-out, where everybody looks for it anyway.
              Removing a button must not remove what it could do, so the
            "everywhere" half moved into the action that already implies it:
            handleSetPassword rotates the session epoch now, which ends every
            other session exactly when somebody changes a password because they
            fear it leaked. See its comment in internal/api/handlers.go. */}
      </Card>
        );
      })()}

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM: the second login factor (v8.6.0). Its own Card rather than  */}
      {/* another section inside Security: enrolment is a three-step sequence */}
      {/* with a QR code and a one-time list of recovery codes, which is more  */}
      {/* than the password form's register, and it reads as a separate        */}
      {/* decision from "is there a password at all".                          */}
      {/* ------------------------------------------------------------------ */}
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

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM: passkeys. Its own Card beside the second factor because it   */}
      {/* is a different decision: the factor makes the password stronger,     */}
      {/* a passkey replaces typing it. And unlike the factor it is not always */}
      {/* available, so the card's first job is explaining when it is not.     */}
      {/* ------------------------------------------------------------------ */}
      <PasskeyCard passwordSet={authEnabled} hueIndex={nextHue()} />

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM: Export / import settings                                    */}
      {/* Portable config file: move this instance's settings + off-site      */}
      {/* destinations (and, opt-in, credentials) to another install. Backups, */}
      {/* snapshots and history are never touched.                            */}
      {/* ------------------------------------------------------------------ */}
      <SettingsPortabilityCard t={t} hueIndex={nextHue()} applyImport={applyImportedSettings} />
    </>
  );
}
