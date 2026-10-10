import { useState } from "react";
import { downloadRecoveryKit } from "../../../lib/api";
import { FolderBrowser } from "../../../components/FolderBrowser";
import { ReposCard } from "../ReposCard";
import { PlacementDefaultsCard } from "../PlacementDefaultsCard";
import { NumberField } from "../../../components/NumberField";
import { PathModeSwitch } from "../../../components/PathModeSwitch";
import { InfoBubble } from "../../../components/InfoBubble";
import { Button } from "../../../components/Button";
import { useT } from "../../../lib/i18n";
import { tLtr } from "../../../lib/ltrFragments";
import { IconDownload } from "../../../components/Sidebar";
import { Card, ToggleRow, hueCounter, type SaveState } from "../shared";
import { useSettings } from "../settingsStore";

export function StoragePage() {
  const { t } = useT();
  const {
    settings,
    setSettings,
    hostMountRoot,
    save,
    debouncedSave,
    fieldPulse,
    autoSaveField,
    mergedFieldBusy,
    mergedFieldShake,
  } = useSettings();

  // Save state per card. Only the setters are used, as the callbacks that
  // autoSaveField and debouncedSave take.
  const [, setEncSaveState] = useState<SaveState>("idle");
  const [, setEncSaveError] = useState<string | null>(null);
  // Recovery-kit download refusal (such as the fail-closed 403 "set a login
  // password" answer when auth is off), shown next to the download button.
  const [kitError, setKitError] = useState<string | null>(null);

  const [, setPathSaveState] = useState<SaveState>("idle");
  const [, setPathSaveError] = useState<string | null>(null);
  const [, setExportEncSaveState] = useState<SaveState>("idle");
  const [, setExportEncSaveError] = useState<string | null>(null);

  const [, setCacheSaveState] = useState<SaveState>("idle");
  const [, setCacheSaveError] = useState<string | null>(null);
  const [, setCoresSaveState] = useState<SaveState>("idle");
  const [, setCoresSaveError] = useState<string | null>(null);

  const nextHue = hueCounter();

  return (
    <>
      {/* Above the domain paths: these are the places an individual
          container, VM or folder set can be pointed at instead of the domain
          path below, so the more specific answer is read first. */}
      <ReposCard hueIndex={nextHue()} />
      <PlacementDefaultsCard hueIndex={nextHue()} />

      <Card title={t("settings.paths")} hint={t("settings.pathsHint").replace("{root}", hostMountRoot)} hueIndex={nextHue()}>
        {/* Each field saves itself, debounced per field name like the
            schedules page's cadence fields. The six PathModeSwitch rows are
            one group with their own 0-based hueIndex, separate from this
            card's heading. */}
        <PathModeSwitch
          label={t("settings.containersPath")}
          domain="containers"
          value={settings.containersPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, containersPath: v } : prev);
            debouncedSave("containersPath", () =>
              void save({ containersPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={0}
        />
        <PathModeSwitch
          label={t("settings.vmsPath")}
          domain="vms"
          value={settings.vmsPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, vmsPath: v } : prev);
            debouncedSave("vmsPath", () =>
              void save({ vmsPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={1}
        />
        <PathModeSwitch
          label={t("settings.flashPath")}
          domain="flash"
          value={settings.flashPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, flashPath: v } : prev);
            debouncedSave("flashPath", () =>
              void save({ flashPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={2}
        />
        <PathModeSwitch
          label={t("settings.configPath")}
          domain="config"
          value={settings.configPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, configPath: v } : prev);
            debouncedSave("configPath", () =>
              void save({ configPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={3}
        />
        <PathModeSwitch
          label={t("settings.filesPath")}
          domain="files"
          value={settings.filesPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, filesPath: v } : prev);
            debouncedSave("filesPath", () =>
              void save({ filesPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={4}
        />
        <PathModeSwitch
          label={t("settings.zfsPath")}
          domain="zfs"
          value={settings.zfsPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, zfsPath: v } : prev);
            debouncedSave("zfsPath", () =>
              void save({ zfsPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={5}
        />
        <FolderBrowser
          label={t("settings.restoreFolder")}
          value={settings.restoreFolder}
          hostMountRoot={hostMountRoot}
          hint={t("settings.restoreFolderHint")}
          hueIndex={6}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, restoreFolder: v } : prev);
            debouncedSave("restoreFolder", () =>
              void save({ restoreFolder: v }, setPathSaveState, setPathSaveError)
            );
          }}
        />
      </Card>

      {/* The restic cache under /config survives restarts and would grow without */}
      {/* bound; per-repository caches are evicted after scheduled runs. */}
      <Card title={t("settings.cacheTitle")} hint={tLtr(t, "settings.cacheHint")} hueIndex={nextHue()}>
        <label className="flex flex-col gap-1 sm:w-1/2">
          <span className="text-xs text-carbon-textSub">{t("settings.cacheLimitLabel")}</span>
          <NumberField
            min={0}
            value={settings.resticCacheMaxMB}
            onChange={(e) => {
              // Structural cast (cf. downloadRecoveryKit in api.ts): runtime-identical to
              // e.target.value, but immune to the broken DOM lib resolution.
              const raw = (e.target as unknown as { value: string }).value;
              const n = Math.max(0, parseInt(raw, 10) || 0);
              setSettings((prev) => (prev ? { ...prev, resticCacheMaxMB: n } : prev));
              debouncedSave("resticCacheMaxMB", () =>
                void save({ resticCacheMaxMB: n }, setCacheSaveState, setCacheSaveError)
              );
            }}
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
          />
        </label>
      </Card>

      {/* How much CPU a backup may take (#189), beside the cache card because
          both say how much of this machine BombVault may use. The cap reaches
          each restic child as GOMAXPROCS; without it restic takes every core. */}
      <Card title={t("settings.coresTitle")} hint={t("settings.coresHint")} hueIndex={nextHue()}>
        <label className="flex flex-col gap-1 sm:w-1/2">
          <span className="text-xs text-carbon-textSub">{t("settings.coresLabel")}</span>
          <NumberField
            min={0}
            value={settings.backupCores}
            onChange={(e) => {
              const n = Math.max(0, parseInt(e.target.value, 10) || 0);
              setSettings((prev) => (prev ? { ...prev, backupCores: n } : prev));
              debouncedSave("backupCores", () =>
                void save({ backupCores: n }, setCoresSaveState, setCoresSaveError)
              );
            }}
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
          />
        </label>
      </Card>

      {/* Plain-export encryption (age) and the repositories' own encryption. The */}
      {/* switches save at once; the recipients field is debounced. */}
      <Card title={t("settings.exportsEncryptionTitle")} hint={t("settings.exportsEncryptionHint")} hueIndex={nextHue()}>
        <div className="flex flex-col gap-3">
          <ToggleRow
            label={t("export.encrypt.enable")}
            hint={`${t("export.encrypt.hint")} ${t("export.encrypt.ageInfo")} ${t("export.encrypt.enableHint")} ${t("export.encrypt.kitSealed")}`}
            checked={settings.exportEncryptEnabled}
            onChange={(v) => void autoSaveField("exportEncryptEnabled", v, setExportEncSaveState, setExportEncSaveError)}
            disabled={mergedFieldBusy.exportEncryptEnabled}
            shakeNonce={mergedFieldShake.exportEncryptEnabled}
            pulseNonce={fieldPulse.exportEncryptEnabled}
          />
          {settings.exportEncryptEnabled && (
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("export.encrypt.recipients")}</span>
              {/* The recipients hint stays visible text: it names the accepted
                  key syntax, a reference to consult while pasting keys rather
                  than a one-time explainer. */}
              <textarea
                value={settings.exportAgeRecipients}
                spellCheck={false}
                rows={3}
                onChange={(e) => {
                  const v = e.target.value;
                  setSettings((prev) => prev ? { ...prev, exportAgeRecipients: v } : prev);
                  debouncedSave("exportAgeRecipients", () =>
                    void save({ exportAgeRecipients: v }, setExportEncSaveState, setExportEncSaveError)
                  );
                }}
                placeholder={t("export.encrypt.recipientsPlaceholder")}
                dir="ltr"
                className="rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono glim-field-focus text-start"
              />
              <span className="text-xs text-carbon-textMuted">{t("export.encrypt.recipientsHint")}</span>
              {!settings.exportAgeRecipients.trim() && (
                <span className="text-xs text-statusFail">{t("export.encrypt.recipientsRequired")}</span>
              )}
            </label>
          )}
        </div>

        <div className="flex flex-col gap-3">
          {/* The live on/off state is the visible label, so the bubble only
              explains the feature. */}
          <ToggleRow
            label={
              settings.encryptionEnabled
                ? t("settings.encryptionOn")
                : t("settings.encryptionOff")
            }
            // "Password derived from APP_KEY" raises the question of where that
            // password is, so the hint names the recovery kit.
            hint={`${t("settings.encryptionHint")} ${t("settings.encryptionPasswordWhere")}`}
            checked={settings.encryptionEnabled}
            onChange={(v) => void autoSaveField("encryptionEnabled", v, setEncSaveState, setEncSaveError)}
            disabled={mergedFieldBusy.encryptionEnabled}
            shakeNonce={mergedFieldShake.encryptionEnabled}
            pulseNonce={fieldPulse.encryptionEnabled}
          />
          {settings.encryptionEnabled && (
            <div className="flex flex-col gap-2">
              {/* recovery.why can sit in a bubble despite the data-loss risk it
                  explains: the Dashboard's recovery banner does the reminding,
                  and this is only context for the button. */}
              <span className="flex items-center gap-1.5 text-sm text-carbon-text">
                {t("recovery.title")}
                <InfoBubble tip={t("recovery.why")} />
              </span>
              <Button
                label={t("recovery.download")}
                labelKey="recovery.download"
                glyph={<IconDownload />}
                tone="accent"
                onClick={() => {
                  setKitError(null);
                  void downloadRecoveryKit().then(setKitError);
                }}
                className={"self-end shrink-0"}
              />
              {kitError && (
                // Backend error text shown verbatim (such as the fail-closed
                // "set a login password" refusal when auth is off); the API
                // answers in English and is not translated client-side.
                <span className="text-xs text-statusFail wrap-break-word">✗ {kitError}</span>
              )}
            </div>
          )}
        </div>
      </Card>
    </>
  );
}
