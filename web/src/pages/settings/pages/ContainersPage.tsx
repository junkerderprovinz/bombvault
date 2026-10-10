import { useState } from "react";
import { NumberField } from "../../../components/NumberField";
import { InfoBubble } from "../../../components/InfoBubble";
import { Button } from "../../../components/Button";
import { RevealInput } from "../../../components/RevealInput";
import type { RegistryAuthEntry } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { randomId } from "../../../lib/uuid";
import { IconAdd, IconTrash } from "../../../components/Sidebar";
import { Card, ToggleRow, hueCounter, type SaveState } from "../shared";
import { useSettings } from "../settingsStore";

// keepRegistryAuths is what the server should store for the registries card:
// untouched blank rows dropped, and a freshly typed token marked stored so the
// field shows the kept placeholder once the save lands. `auths` and `rowIds`
// are index-aligned. It answers for the payload only: applied to the visible
// list, it would delete a row the user has just added whenever a save from
// another field lands.
export function keepRegistryAuths(
  auths: RegistryAuthEntry[],
  rowIds: string[]
): { auths: RegistryAuthEntry[]; rowIds: string[] } {
  const kept = auths
    .map((a, idx) => ({ a, idx }))
    .filter(
      ({ a }) =>
        a.host.trim() !== "" ||
        a.username.trim() !== "" ||
        a.token.trim() !== ""
    );
  return {
    auths: kept.map(({ a }) => ({
      ...a,
      tokenSet: a.tokenSet || a.token.trim() !== "",
    })),
    rowIds: kept.map(({ idx }) => rowIds[idx]),
  };
}

// markRegistryTokensStored is what the screen shows once a registry save lands:
// every row, including a blank one just added, with a freshly typed token
// marked stored. It is keepRegistryAuths without the filter, kept as a separate
// function because what gets persisted and what stays under the cursor are
// different questions. A blank row leaves the screen when the user removes it
// or reloads, since nothing persisted it.
export function markRegistryTokensStored(
  auths: RegistryAuthEntry[]
): RegistryAuthEntry[] {
  return auths.map((a) => ({
    ...a,
    tokenSet: a.tokenSet || a.token.trim() !== "",
  }));
}

export function ContainersPage() {
  const { t } = useT();
  const {
    settings,
    setSettings,
    save,
    debouncedSave,
    cancelDebounce,
    fieldPulse,
    autoSaveField,
    mergedFieldBusy,
    mergedFieldShake,
    scheduleField,
    autoSaveScheduleField,
    schedFieldBusy,
    schedFieldShake,
  } = useSettings();

  // Reveal state for the registry token rows. A hook cannot be called inside
  // the rows' .map(), so this is one record at the top level, keyed by a stable
  // row id rather than the array index: after a removal, an index key would
  // show the row that slides into the slot already revealed, exposing a token
  // nobody asked to see.
  const [registryTokenVisible, setRegistryTokenVisible] = useState<Record<string, boolean>>({});

  // registryRowIds pairs by index with settings.registryAuths and gives each
  // row a client-only stable id for registryTokenVisible and the React key.
  // Every change to the array's length or order updates both. It is not a
  // field on the rows because the settings PUT decoder rejects unknown fields
  // (DisallowUnknownFields in internal/api/handlers.go), which would break
  // every settings save.
  //
  // The server sends no row ids, so one is minted per row. randomId() rather
  // than crypto.randomUUID(), which needs a secure context and would throw on
  // a plain-HTTP origin (see lib/uuid.ts).
  const [registryRowIds, setRegistryRowIds] = useState<string[]>(() =>
    settings.registryAuths.map(() => randomId())
  );

  // Image cleanup, Unraid update-status reconciliation and registries (#56,
  // #116, #106).
  const [, setPruneSaveState] = useState<SaveState>("idle");
  const [, setPruneSaveError] = useState<string | null>(null);
  const [, setReconcileSaveState] = useState<SaveState>("idle");
  const [, setReconcileSaveError] = useState<string | null>(null);
  const [, setRegistrySaveState] = useState<SaveState>("idle");
  const [, setRegistrySaveError] = useState<string | null>(null);

  // saveRegistries is the registries' save, shared by the debounced field edit
  // and the immediate row removal. It takes the (auths, rowIds) pair its callers
  // have just computed rather than reading state, which would be one render
  // stale.
  //
  // The PUT carries the trimmed list (keepRegistryAuths) while the screen keeps
  // every row (markRegistryTokensStored), so a blank row just added survives a
  // save triggered by typing in another row. The screen's half goes to save()
  // as `echo` and is applied to the live list when the response lands: a list
  // frozen at send time would delete rows added and revert characters typed
  // during the round trip, and leave registryRowIds out of step with the rows
  // on screen.
  function saveRegistries(nextAuths: RegistryAuthEntry[], nextRowIds: string[]) {
    const { auths } = keepRegistryAuths(nextAuths, nextRowIds);
    void save(
      { registryAuths: auths },
      setRegistrySaveState,
      setRegistrySaveError,
      (live) => ({ registryAuths: markRegistryTokensStored(live.registryAuths) })
    );
  }

  const nextHue = hueCounter();

  return (
    <>
      {/* Image cleanup and Unraid's update status both feed the post-backup */}
      {/* update (#56, #116), so they share a card. */}
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

      {/* Registry logins (#106). The update pull reads them, but they are not */}
      {/* image cleanup, so they have a card of their own. */}
      <Card title={t("settings.registriesTitle")} hint={t("settings.registriesHint")} hueIndex={nextHue()}>
        <div className="flex flex-col gap-3">
          {settings.registryAuths.length === 0 && (
            <p className="text-sm text-carbon-textMuted">
              {t("settings.registriesEmpty")}
            </p>
          )}
          {settings.registryAuths.map((entry, i) => {
            // The fallback only guards an index mismatch that every mutation
            // site below prevents by keeping the two arrays in step.
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
              {/* No hueIndex needed: the card's wrapper carries `.glim-hue`,
                  which redefines --color-accent for its subtree in rainbow
                  mode, so the accent here is already the card's colour. */}
              <Button
                label={t("settings.registryRemove")}
                labelKey="settings.registryRemove"
                glyph={<IconTrash />}
                tone="accent"
                onClick={() => {
                  // Removing a row is a discrete action, so it saves at once
                  // and cancels any pending debounced save from this section,
                  // so a stale pre-removal snapshot cannot land after it.
                  const nextAuths = settings.registryAuths.filter((_, j) => j !== i);
                  const nextRowIds = registryRowIds.filter((_, j) => j !== i);
                  setSettings((prev) => (prev ? { ...prev, registryAuths: nextAuths } : prev));
                  setRegistryRowIds(nextRowIds);
                  // Drop this row's reveal-state entry too, so neither an id
                  // nor a stray "revealed" flag survives to be picked up by
                  // whatever row slides into this index next.
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
                // A new row gets a fresh id, so it cannot inherit a stale
                // "revealed" flag from a removed row. It is not saved until a
                // field in it is filled in; keepRegistryAuths drops it blank.
                setRegistryRowIds((prev) => [...prev, randomId()]);
              }}
              className={"shrink-0"}
            />
          </div>
        </div>
      </Card>

      {/* Health-gated ordered restart (#119): containers stopped for a backup
          restart in compose depends_on order, each healthy before its
          dependents start. The wait also covers the post-backup update
          recreate (internal/backup WhileDependentsStopped). */}
      <Card title={t("settings.restartHealthTitle")} hueIndex={nextHue()}>
        <ToggleRow
          label={t("settings.restartHealthWait")}
          hint={t("settings.restartHealthWaitHint")}
          checked={settings.restartHealthWait}
          onChange={(v) => void autoSaveScheduleField("restartHealthWait", v)}
          disabled={schedFieldBusy.restartHealthWait}
          shakeNonce={schedFieldShake.restartHealthWait}
          pulseNonce={fieldPulse.restartHealthWait}
        />
        {settings.restartHealthWait && (
          <label className="flex flex-col gap-1 sm:w-1/2">
            <span className="flex items-center gap-1 text-xs text-carbon-textSub">
              {t("settings.restartHealthTimeoutLabel")}
              <InfoBubble tip={t("settings.restartHealthTimeoutHint")} />
            </span>
            <NumberField
              min={5}
              max={3600}
              value={settings.restartHealthTimeoutSec}
              onChange={(e) => {
                const raw = (e.target as unknown as { value: string }).value;
                // Clamp to the field minimum (5): never let a transient sub-5
                // value sit in component state. The server clamps to 5..3600.
                const n = Math.max(5, parseInt(raw, 10) || 0);
                scheduleField("restartHealthTimeoutSec", n);
              }}
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
            />
          </label>
        )}
      </Card>
    </>
  );
}
