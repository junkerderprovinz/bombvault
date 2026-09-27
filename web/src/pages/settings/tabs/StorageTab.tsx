import { downloadRecoveryKit } from "../../../lib/api";
import { FolderBrowser } from "../../../components/FolderBrowser";
import { DomainsCard } from "../../../components/places/DomainsCard";
import { PlacesCard } from "../../../components/places/PlacesCard";
import { NumberField } from "../../../components/NumberField";
import { InfoBubble } from "../../../components/InfoBubble";
import { Button } from "../../../components/Button";
import { RevealInput } from "../../../components/RevealInput";
import type { RegistryAuthEntry } from "../../../lib/api";
import { tLtr } from "../../../lib/ltrFragments";
import { randomId } from "../../../lib/uuid";
import { IconAdd, IconDownload, IconTrash } from "../../../components/Sidebar";
import { Card, ToggleRow } from "../shared";
import type { SettingsTabProps } from "./types";

export function StorageTab({
  t,
  advanced,
  settings,
  setSettings,
  hostMountRoot,
  registryTokenVisible,
  setRegistryTokenVisible,
  registryRowIds,
  setRegistryRowIds,
  setEncSaveState,
  setEncSaveError,
  kitError,
  setKitError,
  setPathSaveState,
  setPathSaveError,
  setExportEncSaveState,
  setExportEncSaveError,
  setPruneSaveState,
  setPruneSaveError,
  setReconcileSaveState,
  setReconcileSaveError,
  setCacheSaveState,
  setCacheSaveError,
  setCoresSaveState,
  setCoresSaveError,
  fieldPulse,
  save,
  mergedFieldBusy,
  mergedFieldShake,
  autoSaveField,
  debouncedSave,
  cancelDebounce,
  saveRegistries,
}: SettingsTabProps) {
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    <>
      <PlacesCard hueIndex={nextHue()} hostMountRoot={hostMountRoot} />
      <DomainsCard hueIndex={nextHue()} />

      <Card title={t("settings.restoreFolder")} hint={t("settings.restoreFolderHint")} hueIndex={nextHue()}>
        <FolderBrowser
          label={t("settings.restoreFolder")}
          value={settings.restoreFolder}
          hostMountRoot={hostMountRoot}
          renderLabel={false}
          onChange={(v) => {
            setSettings((prev) => (prev ? { ...prev, restoreFolder: v } : prev));
            debouncedSave("restoreFolder", () => void save({ restoreFolder: v }, setPathSaveState, setPathSaveError));
          }}
        />
      </Card>

      <Card title={t("settings.imageMaintenanceTitle")} hint={t("settings.imageMaintenanceHint")} hueIndex={nextHue()}>
        <ToggleRow
          label={t("settings.pruneImageAfterUpdate")}
          hint={t("settings.pruneImageAfterUpdateHint")}
          checked={settings.pruneImageAfterUpdate}
          onChange={(v) => void autoSaveField("pruneImageAfterUpdate", v, setPruneSaveState, setPruneSaveError)}
          disabled={mergedFieldBusy.pruneImageAfterUpdate}
          shakeNonce={mergedFieldShake.pruneImageAfterUpdate}
          pulseNonce={fieldPulse.pruneImageAfterUpdate}
        />
        <ToggleRow
          label={t("settings.reconcileUnraidStatus")}
          hint={t("settings.reconcileUnraidStatusHint")}
          checked={settings.reconcileUnraidUpdateStatus}
          onChange={(v) => void autoSaveField("reconcileUnraidUpdateStatus", v, setReconcileSaveState, setReconcileSaveError)}
          disabled={mergedFieldBusy.reconcileUnraidUpdateStatus}
          shakeNonce={mergedFieldShake.reconcileUnraidUpdateStatus}
          pulseNonce={fieldPulse.reconcileUnraidUpdateStatus}
        />
      </Card>

      <Card title={t("settings.registriesTitle")} hint={t("settings.registriesHint")} hueIndex={nextHue()}>
        <div className="flex flex-col gap-3">
          {settings.registryAuths.length === 0 && (
            <p className="text-sm text-carbon-textMuted">
              {t("settings.registriesEmpty")}
            </p>
          )}
          {settings.registryAuths.map((entry, i) => {
            // Every handler below updates both arrays together; the fallback
            // only covers a render that lands between the two state updates.
            const rowId = registryRowIds[i] ?? `registry-row-fallback-${i}`;
            return (
            <div
              key={rowId}
              className="grid grid-cols-1 sm:grid-cols-[1fr_1fr_1fr_auto] gap-2 items-end"
            >
              <label className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textSub">
                  {t("settings.registryHost")}
                </span>
                <input
                  type="text"
                  value={entry.host}
                  placeholder="ghcr.io"
                  onChange={(e) => {
                    const host = e.target.value;
                    const nextAuths = settings.registryAuths.map((a, j) =>
                      j === i ? { ...a, host } : a
                    );
                    setSettings((prev) => (prev ? { ...prev, registryAuths: nextAuths } : prev));
                    debouncedSave("registryAuths", () => saveRegistries(nextAuths, registryRowIds));
                  }}
                  className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
                />
              </label>
              <label className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textSub">
                  {t("settings.registryUser")}
                </span>
                <input
                  type="text"
                  value={entry.username}
                  autoComplete="off"
                  onChange={(e) => {
                    const username = e.target.value;
                    const nextAuths = settings.registryAuths.map((a, j) =>
                      j === i ? { ...a, username } : a
                    );
                    setSettings((prev) => (prev ? { ...prev, registryAuths: nextAuths } : prev));
                    debouncedSave("registryAuths", () => saveRegistries(nextAuths, registryRowIds));
                  }}
                  className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
                />
              </label>
              <label className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textSub">
                  {t("settings.registryToken")}
                </span>
                <RevealInput
                  visible={!!registryTokenVisible[rowId]}
                  onToggleVisible={() =>
                    setRegistryTokenVisible((p) => ({ ...p, [rowId]: !p[rowId] }))
                  }
                  showLabel={t("common.showValue")}
                  hideLabel={t("common.hideValue")}
                  value={entry.token}
                  autoComplete="new-password"
                  placeholder={
                    entry.tokenSet && entry.token === ""
                      ? t("cloud.secretSet")
                      : ""
                  }
                  onChange={(e) => {
                    const token = e.target.value;
                    const nextAuths = settings.registryAuths.map((a, j) =>
                      j === i ? { ...a, token } : a
                    );
                    setSettings((prev) => (prev ? { ...prev, registryAuths: nextAuths } : prev));
                    debouncedSave("registryAuths", () => saveRegistries(nextAuths, registryRowIds));
                  }}
                  wrapperClassName="w-full"
                  className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
                />
              </label>
              {/* The accent fill takes this card's hue through .glim-hue, so no hueIndex is passed. */}
              <Button
                label={t("settings.registryRemove")}
                labelKey="settings.registryRemove"
                glyph={<IconTrash />}
                tone="accent"
                onClick={() => {
                  // Removing a row saves at once and drops any pending debounced
                  // save, so an older snapshot cannot land after it.
                  const nextAuths = settings.registryAuths.filter((_, j) => j !== i);
                  const nextRowIds = registryRowIds.filter((_, j) => j !== i);
                  setSettings((prev) => (prev ? { ...prev, registryAuths: nextAuths } : prev));
                  setRegistryRowIds(nextRowIds);
                  // The row that slides into this index must not inherit its reveal flag.
                  setRegistryTokenVisible((prev) => {
                    if (!(rowId in prev)) return prev;
                    const next = { ...prev };
                    delete next[rowId];
                    return next;
                  });
                  cancelDebounce("registryAuths");
                  saveRegistries(nextAuths, nextRowIds);
                }}
                className={"shrink-0"}
              />
            </div>
            );
          })}
          <div className="flex justify-end">
            <Button
              label={t("settings.registryAdd")}
              labelKey="settings.registryAdd"
              glyph={<IconAdd />}
              tone="accent"
              onClick={() => {
                setSettings((prev) => {
                  if (!prev) return prev;
                  const blank: RegistryAuthEntry = {
                    host: "",
                    username: "",
                    token: "",
                    tokenSet: false,
                  };
                  return { ...prev, registryAuths: [...prev.registryAuths, blank] };
                });
                // A fresh id keeps a stale reveal flag off the new row. The row is
                // saved once one of its fields is filled in.
                setRegistryRowIds((prev) => [...prev, randomId()]);
              }}
              className={"shrink-0"}
            />
          </div>
        </div>
      </Card>

      {/* The restic cache under /config survives restarts and would grow without
          bound. `advanced &&` instead of <Advanced>: the wrapper builds its
          children before deciding to show them, so nextHue() would spend a hue
          slot on a card that never paints. */}
      {advanced && (
      <Card title={t("settings.cacheTitle")} hint={tLtr(t, "settings.cacheHint")} hueIndex={nextHue()}>
        <label className="flex flex-col gap-1 sm:w-1/2">
          <span className="text-xs text-carbon-textSub">{t("settings.cacheLimitLabel")}</span>
          <NumberField
            min={0}
            value={settings.resticCacheMaxMB}
            onChange={(e) => {
              // Read through a structural cast; the DOM lib typing of e.target is broken here.
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
      )}

      {/* Beside the cache card: both limit how much of the machine BombVault
          uses. The cap reaches each restic child as GOMAXPROCS; without it
          restic takes every core (#189). `advanced &&` for the reason the cache
          card gives. */}
      {advanced && (
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
      )}

      {/* Plain-export encryption and the repositories' own encryption share one
          card. Each field saves on its own: the toggles optimistically with a
          revert on failure, the recipients field debounced like the registry
          fields above. */}
      <Card title={t("settings.exportsEncryptionTitle")} hint={t("settings.exportsEncryptionHint")} hueIndex={nextHue()}>
        {/* Plain-export encryption (age) */}
        <div className="flex flex-col gap-3">
          <ToggleRow
            label={t("export.encrypt.enable")}
            hint={`${t("export.encrypt.hint")} ${t("export.encrypt.ageInfo")} ${t("export.encrypt.enableHint")}`}
            checked={settings.exportEncryptEnabled}
            onChange={(v) => void autoSaveField("exportEncryptEnabled", v, setExportEncSaveState, setExportEncSaveError)}
            disabled={mergedFieldBusy.exportEncryptEnabled}
            shakeNonce={mergedFieldShake.exportEncryptEnabled}
            pulseNonce={fieldPulse.exportEncryptEnabled}
          />
          {settings.exportEncryptEnabled && (
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("export.encrypt.recipients")}</span>
              {/* The recipients hint stays visible because it names the key
                  syntax someone checks while pasting one key per line. */}
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

        {/* Repository encryption. The live on/off label is the caption and the
            explainer sits in its bubble, as in the export block above. */}
        <div className="flex flex-col gap-3">
          <ToggleRow
            label={
              settings.encryptionEnabled
                ? t("settings.encryptionOn")
                : t("settings.encryptionOff")
            }
            // "Password derived from APP_KEY" alone sends people looking for a
            // password to run restic by hand; the recovery kit is where it is.
            hint={`${t("settings.encryptionHint")} ${t("settings.encryptionPasswordWhere")}`}
            checked={settings.encryptionEnabled}
            onChange={(v) => void autoSaveField("encryptionEnabled", v, setEncSaveState, setEncSaveError)}
            disabled={mergedFieldBusy.encryptionEnabled}
            shakeNonce={mergedFieldShake.encryptionEnabled}
            pulseNonce={fieldPulse.encryptionEnabled}
          />
          {settings.encryptionEnabled && (
            <div className="flex flex-col gap-2">
              {/* recovery.why sits in a bubble: the dashboard's recovery banner
                  already asks until the kit is stored. */}
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
                // The server's refusal is shown as sent, in English.
                <span className="text-xs text-statusFail wrap-break-word">✗ {kitError}</span>
              )}
            </div>
          )}
        </div>
      </Card>
    </>
  );
}
