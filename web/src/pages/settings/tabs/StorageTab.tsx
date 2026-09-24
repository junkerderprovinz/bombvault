import { downloadRecoveryKit } from "../../../lib/api";
import { FolderBrowser } from "../../../components/FolderBrowser";
import { ReposCard } from "../ReposCard";
import { PlacementDefaultsCard } from "../PlacementDefaultsCard";
import { NumberField } from "../../../components/NumberField";
import { PathModeSwitch } from "../../../components/PathModeSwitch";
import { InfoBubble } from "../../../components/InfoBubble";
import { Button } from "../../../components/Button";
import { RevealInput } from "../../../components/RevealInput";
import type { Settings, RegistryAuthEntry } from "../../../lib/api";
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
  setRetSaveState,
  setRetSaveError,
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
      {/* ------------------------------------------------------------------ */}
      {/* STORAGE: Named repositories (#204)                                 */}
      {/* ------------------------------------------------------------------ */}
      {/* Above the domain paths on purpose: these are the places an INDIVIDUAL
          container, VM or folder set can be pointed at instead of the domain
          path below, so the more specific answer is read first. */}
      <ReposCard hueIndex={nextHue()} />
      <PlacementDefaultsCard hueIndex={nextHue()} />

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE: Backup paths                                              */}
      {/* ------------------------------------------------------------------ */}
      <Card title={t("settings.paths")} hint={t("settings.pathsHint").replace("{root}", hostMountRoot)} hueIndex={nextHue()}>
        {/* Full-page Speichern-Button sweep (jdp, live review, emphatic:
            "Die Speicher-Buttons sollen in allen Tabs weg. Überall soll es
            automatisch speichern."): all six fields below used to batch into
            one bottom SaveBar. Each now debounce-auto-saves itself instead,
            the exact same `debouncedSave`-keyed-by-field-name shape the
            Schedules tab's own `scheduleField` already established for
            continuously-typed values (a path is typed/browsed the same way a
            cron string is), just called directly here since these six PATCH
            single independent fields rather than a whole cadence group.
              `hueIndex={0..4}` below (GlimStone standing colour-engine rule,
            closing the gap OffsiteWizard's own hueIndex doc comment already
            named): these five PathModeSwitch rows are one related GROUP (own
            local 0-based index per group, same rule as the Domains Card's
            seven ToggleRows), separate from this Card's own heading
            `nextHue()` call above. */}
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
        <FolderBrowser
          label={t("settings.restoreFolder")}
          value={settings.restoreFolder}
          hostMountRoot={hostMountRoot}
          hint={t("settings.restoreFolderHint")}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, restoreFolder: v } : prev);
            debouncedSave("restoreFolder", () =>
              void save({ restoreFolder: v }, setPathSaveState, setPathSaveError)
            );
          }}
        />
      </Card>

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE: Local snapshot retention (#51, moved here from Off-site,    */}
      {/* so it sits with the local backup paths it prunes).                   */}
      {/* ------------------------------------------------------------------ */}
      <Card
        title={t("settings.retentionTitle")}
        // Live-review round 3, point 4 sweep: this Card's own intro used to
        // sit as a permanent visible <p> below the title instead of going
        // through the Card `hint` mechanism every OTHER Card-level intro in
        // this file already uses, a plain miss, not a documented exception
        // (compare settings.offsiteHint further down, which stayed visible
        // on purpose with its own comment explaining why). retentionHint
        // (what this Card does) and retentionCombineInfo (the OR-combination
        // rule, a "why wasn't this pruned" answer someone re-checks, same
        // category as notify.healthchecksLifecycle's carve-out) both fold
        // into the one title-level bubble rather than leaving the second as
        // an orphaned bare icon once the wrapping <p> it lived in is gone.
        hint={`${t("settings.retentionHint")} ${t("settings.retentionCombineInfo")}`}
        hueIndex={nextHue()}
      >
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {([
            ["retentionKeepLast", "settings.retentionLast", "settings.retentionLastInfo"],
            ["retentionKeepDaily", "settings.retentionDaily", "settings.retentionDailyInfo"],
            ["retentionKeepWeekly", "settings.retentionWeekly", "settings.retentionWeeklyInfo"],
            ["retentionKeepMonthly", "settings.retentionMonthly", "settings.retentionMonthlyInfo"],
          ] as const).map(([key, label, info]) => (
            <label key={key} className="flex flex-col gap-1">
              <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                {t(label)}
                <InfoBubble tip={t(info)} />
              </span>
              <NumberField
                min={0}
                value={settings[key]}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setSettings((prev) => (prev ? { ...prev, [key]: n } : prev));
                  // Full-page Speichern-Button sweep: this whole grid used to
                  // batch into one bottom SaveBar, each cell now debounce-
                  // auto-saves itself, keyed by its own field name so typing
                  // in one cell never resets another cell's pending timer.
                  debouncedSave(key, () => void save({ [key]: n } as Partial<Settings>, setRetSaveState, setRetSaveError));
                }}
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
              />
            </label>
          ))}
        </div>
      </Card>

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE: Image cleanup and Unraid's own update-status                */}
      {/* reconciliation (GlimStone follow-up round, merge A): both feed the   */}
      {/* SAME post-backup container-update pipeline (#56, #116). Every field  */}
      {/* here auto-saves instead of batching into a Speichern button,         */}
      {/* mirrors the Domains card's own auto-save mechanism (#142): both      */}
      {/* toggles use the exact optimistic-flip + persist + revert-on-failure  */}
      {/* shape toggleDomainEnabled established (see autoSaveField below).     */}
      {/*   Private container registries (#106) USED to be a third             */}
      {/* sub-section merged into this same card. SPLIT BACK OUT into its own  */}
      {/* standalone Card below (jdp, live-review: "Registries: wir machen     */}
      {/* eine eigene Card daraus"), a registry credential is consulted BY     */}
      {/* the update-pull, but isn't itself image cleanup or Unraid's own      */}
      {/* status reconciliation, so the merge was really "three things on the  */}
      {/* same Storage tab," not three parts of one coherent decision; this    */}
      {/* undoes exactly that, not a mechanical revert of merge A as a whole.  */}
      {/* This card's own title/hint (settings.imageMaintenanceTitle/-Hint,    */}
      {/* same keys, retitled values) dropped every registries mention         */}
      {/* accordingly.                                                        */}
      {/* ------------------------------------------------------------------ */}
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

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE: private container registries (#106), its own standalone     */}
      {/* Card again, see the Image Cleanup card's own comment above for why   */}
      {/* it split out. `hint` now carries what used to be a separate          */}
      {/* <h3>+InfoBubble pair right inside the merged card                    */}
      {/* (settings.registriesTitle/-Hint, unchanged keys/values, just         */}
      {/* promoted to the Card's own title/hint slot), the exact same          */}
      {/* content, through the ONE heading+bubble mechanism every other Card   */}
      {/* on this page already uses instead of a second, bespoke one. No       */}
      {/* `border-t` divider carried over either, that only ever separated     */}
      {/* this sub-section from its two former siblings; a standalone Card     */}
      {/* already has its own surface/edge doing that job, same as every       */}
      {/* other single-purpose Card in this file.                              */}
      {/* ------------------------------------------------------------------ */}
      <Card title={t("settings.registriesTitle")} hint={t("settings.registriesHint")} hueIndex={nextHue()}>
        <div className="flex flex-col gap-3">
          {settings.registryAuths.length === 0 && (
            <p className="text-sm text-carbon-textMuted">
              {t("settings.registriesEmpty")}
            </p>
          )}
          {settings.registryAuths.map((entry, i) => {
            // Fallback only guards a transient/impossible index mismatch (see
            // registryRowIds' declaration), every mutation site below keeps
            // the two arrays in lockstep, so this should never actually miss.
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
              {/* Square icon-only remove button with a trash-can glyph (jdp,
                  live-review: "Wenn man eine Registry hinzufügt, soll der
                  Entfernen-Button quadratisch sein mit Mülleimer-Icon"), was
                  a bare text `<button>` ("Entfernen"/"Remove"). IconTipButton
                  (components/IconTipButton.tsx) for the same real
                  `.glim-bubble` hover tooltip every other icon-only control on
                  this page already gets, not a native `title=`;
                  `settings.registryRemove`'s existing value moves from
                  visible button text to this tooltip's own content unchanged,
                  same "text moves onto the tip, key stays" move the
                  Registry-add button below already made.
                    COLOUR-ENGINE ROUND (jdp's standing rule, five escalations
                  deep): this badge and the Registry-add one below were still
                  flat `bg-carbon-surface3` grey with no tie to this Card's own
                  hue at all, the same gap that got the delete badge's grey
                  special-casing removed a round earlier. Both are real
                  `Badge`s now (`as="button" tone="active" shape="square"
                  size="icon"`), which for an icon-only badge resolves to the
                  full solid `bg-accent`/`text-accentContrast` fill, NOT the
                  pale wash jdp rejected as "halb abgedunkelt" (see Badge.tsx's
                  own `toneClasses` ROUND 2 comment).
                    No `hueIndex` prop, and none needed: this Card is
                  `<Card ... hueIndex={nextHue()}>` with no local variable to
                  hand down, but Card's own wrapper carries `.glim-hue`, and
                  `[data-rainbow] .glim-hue` (index.css) redefines
                  `--color-accent` for its whole subtree, so `bg-accent` here
                  already computes to THIS Card's rainbow position by ordinary
                  custom-property inheritance. Same mechanism Containers.tsx's
                  own folder-add badge documents; wrapping this Card in an IIFE
                  purely to capture `nextHue()` would add a second source of
                  truth for a colour that already resolves correctly. Verified
                  live with getComputedStyle against the Card's own
                  `--item-hue`.
                    `size="icon"` is the app's ONE square-icon-badge size and
                  is the same 32px this call site already had, so the footprint
                  is unchanged; `shrink-0` survives as `className` because it
                  is layout, not appearance. Not a fresh guess either: this
                  row's own three text fields
                  are `text-sm px-3 py-1.5`, the SAME classes already
                  measured live to render at 32px for those other controls
                  (see Selector.tsx's own `iconOnly` doc for that
                  measurement's full writeup), so 32px is this row's real
                  control height too, confirmed, not assumed from a token
                  used elsewhere. IconTrash (components/Sidebar.tsx) drawn
                  fresh for this, no trash glyph existed in this codebase
                  yet, filled/`currentColor`-only, no `stroke`, matching
                  every other icon in that file's icon-only-badge set. */}
              <Button
                label={t("settings.registryRemove")}
                labelKey="settings.registryRemove"
                glyph={<IconTrash />}
                tone="accent"
                onClick={() => {
                  // Removing a row is a discrete action, not a text edit, it
                  // saves IMMEDIATELY (no debounce), and cancels any pending
                  // debounced save from an edit elsewhere in this section so
                  // a stale pre-removal snapshot can't land after this one.
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
          {/* Icon-only + right-aligned (GlimStone follow-up round, live-review:
              "Registry hinzufügen button soll bündig nach rechts... einen
              Glyph statt Text bekommen, mit Hover-Infobubble"), `flex
              justify-end` is this file's own established idiom for a single
              trailing action in an otherwise block-level row (see e.g.
              Dashboard.tsx's/VMs.tsx's identical `<div className="flex
              justify-end">` wrapper for a lone action). The row's own text
              label moves onto the button's `IconTipButton` tip instead of
              disappearing, an icon-only trigger has no other way to say
              what it does. Same 32px square-icon-badge footprint as
              FolderBrowser's own "Durchsuchen" badge (the one real field/
              control height already established on this page), expressed as
              Badge's `size="icon"` stage now rather than a hand-written
              `h-8 w-8`. Converted from flat `bg-carbon-surface3` grey to a
              hue-carrying Badge in the same colour-engine round as the
              Registry-remove badge above: see that call site's own comment
              for the full reasoning, including why neither needs an explicit
              `hueIndex`. */}
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
                // A brand-new row always starts with its OWN fresh id, never
                // reusing one, so it can't inherit a stale "revealed" flag left
                // behind by a since-removed row that used to sit at this index.
                // Not saved yet, a blank row has nothing worth persisting
                // until a field in it is actually filled in (debouncedSave
                // above then fires, and its own "kept" filter would drop it
                // again anyway if it's abandoned blank).
                setRegistryRowIds((prev) => [...prev, randomId()]);
              }}
              className={"shrink-0"}
            />
          </div>
        </div>
      </Card>

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE: restic cache size limit. The persistent cache under         */}
      {/* /config (RESTIC_CACHE_DIR) survives restarts and would otherwise     */}
      {/* grow unbounded; LRU per-repo caches are evicted after scheduled runs.*/}
      {/* ------------------------------------------------------------------ */}
      {/* `advanced &&` inline (not the <Advanced> wrapper component): the
          wrapper takes children as an ALREADY-BUILT prop, so this Card's own
          hueIndex={nextHue()} would fire every render regardless of whether
          Advanced end up showing it, caught live (Playwright against the
          real container) as a hue slot silently "spent" on a Card that never
          painted, shifting every later Storage-tab heading by one position
          while Advanced was off. Plain `&&` short-circuits properly, exactly
          like every other conditional Card on this page. */}
      {advanced && (
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
              // Full-page Speichern-Button sweep: was its own bottom SaveBar.
              debouncedSave("resticCacheMaxMB", () =>
                void save({ resticCacheMaxMB: n }, setCacheSaveState, setCacheSaveError)
              );
            }}
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
          />
        </label>
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE: How much CPU a backup may take ([558], issue #189)         */}
      {/* ------------------------------------------------------------------ */}
      {/* Sits beside the cache card because both answer the same question:
          how much of this machine BombVault is allowed to use. The cap is
          handed to each restic child as GOMAXPROCS; restic is a Go program and
          without it takes every core there is. Reported from a 12-thread box
          that sat at 99% on every core and 100 °C for a whole backup, with the
          reporter's own summary: "Makes backups slow."

          `advanced &&` inline rather than the <Advanced> wrapper, and the
          reason is in the cache card's comment above: the wrapper builds its
          children before deciding to render them, so a hueIndex={nextHue()}
          inside it spends a hue slot on a card that never paints. */}
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

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE: Plain-export encryption (age) and the restic repositories'   */}
      {/* own encryption, merged into one card (GlimStone follow-up round,      */}
      {/* merge B). Every field auto-saves instead of batching into a           */}
      {/* Speichern button (#142's own mechanism), the two toggles use          */}
      {/* autoSaveField (optimistic + revert-on-failure); the recipients field  */}
      {/* debounces instead, same reasoning as the registries fields in the     */}
      {/* Image Cleanup card above.                                            */}
      {/*   Flash-ZIP-Export (#28) used to be a third sub-section in THIS same  */}
      {/* card. Moved out in TWO steps, live-review (jdp): first "trenn bitte   */}
      {/* flash zip export und den rest wieder in zwei separate cards", then,   */}
      {/* superseding that, "soll die flash zip export toggle nicht einfach     */}
      {/* in den flash tab? macht doch mehr sinn." It now lives on the Flash    */}
      {/* page itself (pages/Flash.tsx's own FlashZipExportCard, exported from  */}
      {/* this file the same way AccentCard/ThemeCard/RcloneCard/CloudCard      */}
      {/* already are for cross-page reuse, see that component's own header     */}
      {/* comment for the full move and why it's self-contained rather than     */}
      {/* threaded through SettingsPage's own save()/autoSaveField()). This     */}
      {/* card's own title/hint dropped every flash-zip-export mention          */}
      {/* accordingly, it now only covers what's actually left: plain-export    */}
      {/* encryption and repository encryption, both real "encrypt SOMETHING"   */}
      {/* settings, so `settings.exportsEncryptionTitle`/`Hint` keep their OLD   */}
      {/* key names (an internal identifier, not user-facing) with NEW values.  */}
      {/* ------------------------------------------------------------------ */}
      <Card title={t("settings.exportsEncryptionTitle")} hint={t("settings.exportsEncryptionHint")} hueIndex={nextHue()}>
        {/* Plain-export encryption (age) -------------------------------------- */}
        {/* No `border-t` divider against Repository encryption below it (jdp,
            live review: "die Linien dazwischen weg"), the Card's own `gap-4`
            between direct children already separates the two sub-sections,
            same spacing-only convention as the Colors Card's own accent/
            rainbow halves and its own rainbow ToggleRow trio
            (settings.rainbow/-Reactive/-Rotate) elsewhere in this file, none
            of which ever had a rule line between their parts either. (A
            THIRD sub-section, Flash-ZIP-Export, used to sit above this one,
            see this Card's own header comment for where it moved.) */}
        <div className="flex flex-col gap-3">
          {/* No more standalone <h3> sub-heading (jdp, live-review: "Export
              und Verschlüsselung: Texte normal formatieren, es sind keine
              Überschriften mehr"), this sub-section is now JUST a single
              ToggleRow with an optional conditional block beneath it, not a
              heading introducing its own block of content, so it shouldn't
              LOOK like one either. `hideLabel` is gone below: the row's own
              native `label` (ToggleRow's plain `text-sm text-carbon-text`
              span, the same normal weight every other row's own caption in
              this app already uses, not the bold/uppercase/tracking-widest
              heading treatment the removed `<h3>` had) is now this
              sub-section's only visible caption. `export.encrypt.title` (the
              old heading's own text, "Encrypt plain exports"/"Plain-Exporte
              verschlüsseln") is retired, the ToggleRow's own
              `export.encrypt.enable` label already names the same action
              ("Encrypt exports with age"/"Exporte mit age verschlüsseln")
              and is the one text a screen reader announces for this switch
              either way, so keeping both would be two competing captions for
              one control. The old heading's own three-sentence InfoBubble
              tip (what age is, what enabling it does) moves onto the
              ToggleRow's own `hint` unchanged, the same content, now
              anchored to the control it actually describes instead of a
              heading standing in front of it. */}
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
              {/* Live-review round 3 sweep: export.encrypt.recipientsHint below
                  (the "one per line, age1.../SSH key format" caption on the
                  textarea) is left as permanent text on purpose, the same
                  "genuine toss-up" carve-out settings.offsiteHint documents
                  further up this file, it names the exact accepted KEY
                  SYNTAX for a multi-line field someone fills in by pasting one
                  key per line, which reads as reference to consult while
                  composing the list rather than a one-time "what does this
                  toggle do" explainer (that half is already covered by
                  export.encrypt.enableHint, now folded into the sub-heading's
                  own InfoBubble above). Flagged, not force-converted. */}
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

        {/* Repository encryption ---------------------------------------------- */}
        {/* jdp, live review: "keine Überschrift und nochmal darunter der
            Text. Nur die Überschrift als Text, alles andere in die
            Infobubble." This sub-heading was the one holdout in this card
            still pairing a bare <h3> with a permanent paragraph underneath
            it (settings.encryptionWarning, now settings.encryptionHint),
            its two siblings above already fold that same kind of one-time
            "here's what this does" text into the heading's own InfoBubble
            (flash.zipExport.hint, export.encrypt.hint+ageInfo). Renamed
            .../Warning -> .../Hint on the move: it's no longer a
            statusWarnBg banner, so it no longer earns the "Warning" name,
            see the still-conditional flash.zipExport.plaintextWarn a few
            lines up for the genuine, actively-risky warning case (only
            rendered while the risk applies) this text never was: it's an
            unconditional, one-time explainer of how the toggle behaves, the
            exact content InfoBubble exists for. No `border-t` here either,
            same reasoning as the Plain-export block above.
              FOLLOW-UP (jdp, live-review, fresh screenshot proved a prior
            round's claim wrong): that earlier pass only bubbled the STATIC
            explainer above, it left the master ToggleRow's own DYNAMIC
            status label ("Aktiviert (Passwort aus APP_KEY)" /
            "Deaktiviert (kein Passwort)") sitting directly under this same
            heading in plain view, which is exactly the line the fresh
            screenshot still showed. `hideLabel` below hides it now, same as
            its two siblings above; the state it used to carry moves into the
            bubble's own tip, computed per render off the live
            `settings.encryptionEnabled` value (the same values
            settings.encryptionOn/Off already translate in every locale, just
            read here instead of handed to the ToggleRow as visible text),
            rather than a static string, so the bubble still answers "is this
            actually on right now" concretely instead of only explaining the
            feature in the abstract. The switch's own filled/unfilled track
            still shows on/off at a glance without hovering anything. */}
        <div className="flex flex-col gap-3">
          {/* No more standalone <h3> sub-heading here either, same fix, same
              reasoning, as the Plain-export block above (jdp, live-review:
              "Export und Verschlüsselung: Texte normal formatieren, es sind
              keine Überschriften mehr"). `hideLabel` is gone: the ToggleRow's
              own DYNAMIC on/off label ("Enabled (password derived from
              APP_KEY)"/"Disabled (no password)") is now this sub-section's
              only visible caption, at ToggleRow's normal `text-sm
              text-carbon-text` weight, not the retired heading's bold/
              uppercase/tracking-widest treatment. `settings.encryption` (the
              old heading's own generic "Encryption"/"Verschlüsselung" text)
              is retired, the row's own live on/off label already says more
              than that static word did. The bubble's own tip drops the
              on/off-state PREFIX it used to carry (`settings.encryptionOn`/
              `Off` concatenated in front of `settings.encryptionHint`): that
              existed only because the label sitting above it was hidden and
              had nowhere else to show the current state, now that the
              state IS the visible label, repeating it inside the bubble too
              would just be the same sentence twice. */}
          <ToggleRow
            label={
              settings.encryptionEnabled
                ? t("settings.encryptionOn")
                : t("settings.encryptionOff")
            }
            // The label says "password derived from APP_KEY" and used to stop
            // there, which reads as an explanation and lands as a riddle: a
            // reporter on the support forum went looking for that password to
            // run restic by hand, could not find it, and only worked out that
            // it sits in the recovery kit after a detour through a container
            // shell. The kit is named here now, at the one place that raises
            // the question.
            hint={`${t("settings.encryptionHint")} ${t("settings.encryptionPasswordWhere")}`}
            checked={settings.encryptionEnabled}
            onChange={(v) => void autoSaveField("encryptionEnabled", v, setEncSaveState, setEncSaveError)}
            disabled={mergedFieldBusy.encryptionEnabled}
            shakeNonce={mergedFieldShake.encryptionEnabled}
            pulseNonce={fieldPulse.encryptionEnabled}
          />
          {settings.encryptionEnabled && (
            <div className="flex flex-col gap-2">
              {/* recovery.why is bubbled, not kept as permanent text, even though
                  it explains a real data-loss risk: the RECURRING "you still
                  haven't saved this" job is already owned by Dashboard.tsx's own
                  separate, more prominent recovery.nagTitle/nagBody banner
                  (dismissed only by recovery.stored), this paragraph is purely
                  the one-time "here's why, if you're curious" context for the
                  button below it, not the app's only safeguard against
                  forgetting. */}
              {/* No more heading-styled <h4> here either (jdp, live-review:
                  "Wiederherstellungs-Kit bitte auch normal formatieren"), same
                  fix, same reasoning, as the Plain-export/Encryption blocks
                  above: this sub-section is just a caption plus a single
                  icon-only download button beneath it, not a heading
                  introducing its own block of content, so it shouldn't LOOK
                  like a section heading either. Swapped the semantic `<h4>`
                  for a plain `<span>` carrying ToggleRow's own exact label
                  classes (`flex items-center gap-1.5 text-sm text-carbon-text`,
                  see ToggleRow's own label span above) instead of the retired
                  bold/uppercase/tracking-widest heading treatment, the ONE
                  normal-caption style this page already uses everywhere else,
                  reused verbatim rather than inventing a second one for this
                  call site. The InfoBubble stays put unchanged; there's no
                  ToggleRow here to fold it onto (this row is a caption + a
                  bare download button, not a toggle). */}
              <span className="flex items-center gap-1.5 text-sm text-carbon-text">
                {t("recovery.title")}
                <InfoBubble tip={t("recovery.why")} />
              </span>
              {/* Icon-only + right-aligned (GlimStone follow-up round,
                  live-review: "...ebenso der Recovery Kit herunterladen
                  button. Beide sollen einen Glyph statt Text bekommen, mit
                  Hover-Infobubble"), the visible "Recovery-Kit
                  herunterladen" label moves onto the IconTipButton's own
                  tip, the only remaining visible text in this sub-section is
                  its heading, same "only heading + a bare bubbled/tooltipped
                  control" shape the encryption toggle right above it now
                  has. `self-end` (not a `flex justify-end` wrapper, this
                  button is already a direct child of the section's own
                  `flex flex-col` above) flips this from the row's start edge
                  to its end edge, RTL-safe, same as every other logical
                  start/end pairing on this page. `size="icon"` is the app's
                  ONE square-icon-badge size (32px), the same footprint
                  FolderBrowser's own Browse badge and the Registry-add badge
                  above use, expressed as Badge's own size stage rather than a
                  hand-written `h-8 w-8`. Converted from flat
                  `bg-carbon-surface3` grey to a hue-carrying Badge in the same
                  colour-engine round as those two: see the Registry-remove
                  badge's own comment for the full reasoning, including why the
                  enclosing Card's `.glim-hue` makes an explicit `hueIndex`
                  unnecessary here. */}
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
                // Backend-provided error text shown verbatim BY DESIGN (e.g. the
                // fail-closed "set a login password" refusal when auth is off),
                // the API answers English and is not translated client-side.
                <span className="text-xs text-statusFail wrap-break-word">✗ {kitError}</span>
              )}
            </div>
          )}
        </div>
      </Card>
    </>
  );
}
