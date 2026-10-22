import { createContext, useContext, useEffect, useRef, useState } from "react";
import {
  getAuth,
  getSettings,
  importSettingsApply,
  listContainers,
  listOffsiteTargets,
  listVMs,
  putSettings,
  type OffsiteTarget,
} from "../../lib/api";
import { offsiteTargetsChanged, subscribeOffsiteTargets } from "../../lib/useOffsiteTargets";
import { destinationsChanged } from "../../lib/useDestinations";
import { reposChanged } from "../../lib/useNamedRepos";
import { storageLocationsChanged } from "../../lib/useStorageLocations";
import { placementChanged } from "../../lib/placementEvents";
import { useConfirm } from "../../lib/useConfirm";
import { pushSaveWarnings } from "../../lib/placementCodes";
import type { Settings, Container, VM } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import type { SaveState } from "./shared";

/** settleHookClears turns a sent hook removal into the state it left behind.
 *  The clear flag holds for one write; kept, it would wipe the hook again on
 *  every later save, including the one that sets a new command. */
function settleHookClears(s: Settings): Settings {
  const { everythingPreHookClear, everythingPostHookClear, ...rest } = s;
  return {
    ...rest,
    everythingPreHookSet: everythingPreHookClear ? false : s.everythingPreHookSet,
    everythingPostHookSet: everythingPostHookClear ? false : s.everythingPostHookSet,
  };
}

/** adoptChangesFromElsewhere takes over what the server holds differently from
 *  the state this page last confirmed, in every field without an unsaved edit
 *  on this page. */
function adoptChangesFromElsewhere(live: Settings, known: Settings, fresh: Settings): Settings {
  const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);
  const next: Record<string, unknown> = { ...live };
  for (const key of Object.keys(fresh) as (keyof Settings)[]) {
    if (!same(fresh[key], known[key]) && same(live[key], known[key])) next[key] = fresh[key];
  }
  return next as unknown as Settings;
}

// useSettingsStore loads the settings and owns every write to them.
// SettingsPage calls it once and hands the result to its pages.
export function useSettingsStore() {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [allTargets, setAllTargets] = useState<OffsiteTarget[]>([]);
  useEffect(() => {
    const load = () => {
      listOffsiteTargets()
        .then((r) => {
          if (r.ok) setAllTargets(r.targets ?? []);
        })
        .catch(() => undefined);
    };
    load();
    return subscribeOffsiteTargets(load);
  }, []);

  const [settings, setSettings] = useState<Settings | null>(null);
  // savedBaseline is the server's state as this page last confirmed it. A save
  // merges its own fields onto a fresh read rather than the live `settings`,
  // so saving one field never commits another card's unsaved edits, and
  // compares the read with this baseline to tell a write made elsewhere from
  // an edit this page has not saved yet.
  //
  // It is a ref, not state. The PUT carries the full settings object, so a save
  // has to read the newest confirmed baseline when the request is built. A state
  // value is frozen into the render that called save(), and two saves inside
  // one round trip would each overwrite the other's field with a stale value.
  // queueSettingsWrite below makes sure the ref is read after the previous
  // write has landed and moved it.
  const savedBaseline = useRef<Settings | null>(null);
  // settingsWrites serializes every settings write on this page (saves and the
  // settings import), so a full-object PUT is never built from a baseline that
  // another in-flight PUT is about to replace. It is a promise chain rather than
  // a busy flag because no write may be dropped: a debounced edit that arrives
  // mid-flight still has to land afterwards.
  const settingsWrites = useRef<Promise<unknown>>(Promise.resolve());

  function queueSettingsWrite<T>(run: () => Promise<T>): Promise<T> {
    const next = settingsWrites.current.then(run);
    // The chain itself must never reject, or every later write would be
    // skipped: a failed save is reported by its own caller, not here.
    settingsWrites.current = next.then(
      () => undefined,
      () => undefined
    );
    return next;
  }
  const [hostMountRoot, setHostMountRoot] = useState<string>("/host/user");
  // The detected or overridden platform.Kind ("unraid" | "generic" | "truenas")
  // from the "platform" field next to the settings in GET /api/settings. It
  // defaults to "unraid" like the Go side's platformFn(), so NotifyCard's
  // mismatch banner does not flash on before this loads.
  const [platformKind, setPlatformKind] = useState<string>("unraid");
  // Whether PLATFORM chose platformKind, from "platformSet" next to it.
  const [platformSet, setPlatformSet] = useState(false);
  // Which read failed: the first one, or the one after an import.
  const [loadError, setLoadError] = useState<"load" | "import" | null>(null);

  // Auth state for the Security card.
  const [authEnabled, setAuthEnabled] = useState(false);
  // The second factor's state, and the minimum the server enforces. The
  // minimum is read rather than hard-coded so the field and the server can
  // never disagree about the number they both quote to the user.
  const [totpEnabled, setTotpEnabled] = useState(false);
  const [recoveryLeft, setRecoveryLeft] = useState<number | undefined>(undefined);
  const [minPasswordLen, setMinPasswordLen] = useState(12);

  // Save state per card. Only the setters are used, as the callbacks that
  // autoSaveField and debouncedSave take.
  const [, setDomSaveState] = useState<SaveState>("idle");
  const [, setDomSaveError] = useState<string | null>(null);
  // Per-row busy flag and shake nonce for the domain toggles, keyed by Settings
  // field name. The nonce is bumped on a rejected save so the shake plays again
  // on a second consecutive failure of the same row.
  type DomainToggleKey =
    | "containersEnabled"
    | "vmsEnabled"
    | "flashEnabled"
    | "filesEnabled"
    | "zfsEnabled"
    | "configEnabled"
    | "receiverEnabled"
    | "pullEnabled"
    | "fleetEnabled"
    | "dbDumpsEnabled";
  const [domainToggleBusy, setDomainToggleBusy] = useState<Partial<Record<DomainToggleKey, boolean>>>({});
  const [domainToggleShake, setDomainToggleShake] = useState<Partial<Record<DomainToggleKey, number>>>({});

  // The containers and VMs the order lists of the schedules page arrange.
  const [containers, setContainers] = useState<Container[]>([]);
  const [vms, setVMs] = useState<VM[]>([]);
  const [, setSchedSaveState] = useState<SaveState>("idle");
  const [, setSchedSaveError] = useState<string | null>(null);
  // The plain boolean fields of the schedules page, saved by
  // autoSaveScheduleField below with their own busy and shake maps.
  type ScheduleBoolKey =
    | "perItemSchedules"
    | "catchUpMissed"
    | "drillsEnabled"
    | "offsiteDrillsEnabled"
    | "startTestEnabled"
    | "restartHealthWait";
  const [schedFieldBusy, setSchedFieldBusy] = useState<Partial<Record<ScheduleBoolKey, boolean>>>({});
  const [schedFieldShake, setSchedFieldShake] = useState<Partial<Record<ScheduleBoolKey, number>>>({});
  // installSettings adopts a settings object from the server as both the live
  // state and the confirmed baseline. The mount load and the reload after a
  // settings import both use it, since an import replaces the whole
  // configuration.
  function installSettings(s: Settings) {
    setSettings(s);
    savedBaseline.current = s;
  }

  function loadSettings() {
    setLoadError(null);
    getSettings()
      .then((res) => {
        if (res.ok) {
          installSettings(res.settings);
          if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
          if (res.platform) setPlatformKind(res.platform);
          setPlatformSet(res.platformSet === true);
        } else {
          setLoadError("load");
        }
      })
      .catch(() => setLoadError("load"));
  }

  useEffect(() => {
    loadSettings();

    getAuth()
      .then((res) => {
        setAuthEnabled(res.enabled);
        setTotpEnabled(res.totp ?? false);
        setRecoveryLeft(res.recoveryCodesLeft);
        if (res.minPasswordLen) setMinPasswordLen(res.minPasswordLen);
      })
      .catch(() => {
        // Non-fatal: Security card shows auth as off.
      });

    listContainers()
      .then((r) => {
        if (r.ok) setContainers(r.containers ?? []);
      })
      .catch(() => {
        // Non-fatal: the order list stays empty.
      });

    listVMs()
      .then((r) => {
        if (r.ok) setVMs(r.vms ?? []);
      })
      .catch(() => {
        // Non-fatal: the order list stays empty.
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // fieldPulse holds a confirmation-pulse nonce per Settings field, bumped for
  // every key of a successful patch: a ToggleRow passes `fieldPulse.someKey`
  // as `pulseNonce`, the success counterpart of the shake nonces. Keys nobody
  // reads cost nothing, so a new toggle gets the pulse without new plumbing.
  const [fieldPulse, setFieldPulse] = useState<Partial<Record<keyof Settings, number>>>({});

  // save persists one card's fields and returns true only when the server
  // confirmed the write. Callers that gate a follow-up on a confirmed save
  // (such as the off-site immutable toggle, which must not run a tamper test on
  // a failed save) await the boolean; others ignore it with `void`. Both
  // outcomes are reported as toasts, and setSaveError is always cleared.
  //
  // Every save goes through queueSettingsWrite, so the object is built from a
  // baseline no other in-flight write is about to change. With auto-save, two
  // saves inside one round trip are the normal case (editing the Containers
  // cadence while sync is on arms two debounces one render apart), and without
  // the queue each would send the other's field at its old value, so the
  // server would keep only one of two edits the screen shows.
  //
  // `echo` is for the case where what the server stores and what the screen
  // shows differ: it is handed the live settings when the response lands and
  // returns what the screen keeps instead of the patch's value. It is a
  // function rather than a second object because the user keeps typing during
  // the round trip, so anything computed at send time would be stale. Only
  // saveRegistries uses it.
  async function save(
    patch: Partial<Settings>,
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void,
    echo?: (live: Settings) => Partial<Settings>
  ): Promise<boolean> {
    return queueSettingsWrite(() => sendSettingsPatch(patch, setSaveState, setSaveError, echo));
  }

  async function sendSettingsPatch(
    patch: Partial<Settings>,
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void,
    echo?: (live: Settings) => Partial<Settings>
  ): Promise<boolean> {
    // Read at send time, not at call time: the previous write in the queue has
    // already advanced this ref by the time we get here.
    const known = savedBaseline.current ?? settings;
    if (!known) return false;
    setSaveState("saving");
    setSaveError(null);
    try {
      // The PUT replaces the whole row, so the patch goes onto what the server
      // holds now: another tab, another device or a storage location page may
      // have written since this page loaded.
      const fresh = await getSettings();
      if (!fresh.ok) {
        setSaveState("idle");
        push(t("settings.error"), "fail");
        return false;
      }
      const updated: Settings = { ...fresh.settings, ...patch };
      const res = await putSettings(updated);
      if (res.ok) {
        // Advance the baseline; reflect the saved fields and what was written
        // elsewhere in the live state, leaving other cards' in-progress edits
        // untouched.
        savedBaseline.current = settleHookClears(updated);
        setSettings((prev) =>
          settleHookClears(
            prev
              ? { ...adoptChangesFromElsewhere(prev, known, fresh.settings), ...patch, ...(echo ? echo(prev) : null) }
              : updated
          )
        );
        setSaveState("idle");
        setFieldPulse((p) => {
          const next = { ...p };
          for (const key of Object.keys(patch) as (keyof Settings)[]) {
            next[key] = (p[key] ?? 0) + 1;
          }
          return next;
        });
        // Layout and Sidebar refetch, so a domain that was just enabled or
        // disabled appears or vanishes without a reload.
        window.dispatchEvent(new Event("bv:settings-changed"));
        push(t("settings.saved"), "success");
        pushSaveWarnings(push, t, res.warnings);
        return true;
      }
      setSaveState("idle");
      push(res.error ?? t("settings.error"), "fail");
      return false;
    } catch (err) {
      setSaveState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      return false;
    }
  }

  // toggleDomainEnabled saves a domain row the moment it is clicked (#142): an
  // optimistic flip, then the shared save(). A rejected save (such as enabling
  // VMs without a working SSH connection to the libvirt host, which
  // handlePutSettings checks on the off-to-on transition) rolls the flip back
  // and bumps the row's shake nonce, whatever the domain or the reason.
  async function toggleDomainEnabled(key: DomainToggleKey, next: boolean) {
    const prev = settings?.[key];
    setSettings((s) => (s ? { ...s, [key]: next } : s));
    setDomainToggleBusy((b) => ({ ...b, [key]: true }));
    const ok = await save({ [key]: next } as Partial<Settings>, setDomSaveState, setDomSaveError);
    setDomainToggleBusy((b) => ({ ...b, [key]: false }));
    if (!ok) {
      // Roll back to the pre-click state; save() already pushed the reason.
      setSettings((s) => (s ? { ...s, [key]: prev ?? !next } : s));
      setDomainToggleShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
    }
  }

  // Switching every dump off at once can leave a database with no consistent
  // copy at all, and the containers it happens to are named before it does.
  // Only a dump that runs today can be lost, and "unknown" coverage cannot
  // carry the claim that the dump is the only consistent copy. A label naming
  // the engine wins over the switch on the card.
  async function toggleDbDumps(next: boolean) {
    if (!next) {
      const atRisk = containers
        .filter(
          (c) =>
            c.dbTier !== "" &&
            (!c.dbDumpOff || c.dbTier === "label") &&
            !c.dbDumpLabelOff &&
            (c.dbTier !== "lookalike" || c.dbDumpEngine !== "") &&
            (c.dbDataCoverage === "live" || c.dbDataCoverage === "none")
        )
        .map((c) => c.name);
      const question = atRisk.length
        ? t("settings.dbDumpsOffConfirm", atRisk.length).replace("{names}", atRisk.join(", "))
        : t("settings.dbDumpsOffConfirmPlain");
      if (!(await confirm(question, { confirmKey: "common.confirm" }))) return;
    }
    await toggleDomainEnabled("dbDumpsEnabled", next);
  }

  // autoSaveField is toggleDomainEnabled's optimistic flip, save and revert for
  // the toggles of the encryption and image-cleanup cards, with busy and shake
  // keyed by field name.
  type MergedAutoSaveKey =
    | "pruneImageAfterUpdate"
    | "reconcileUnraidUpdateStatus"
    | "updateContainersAfterBackup"
    | "containerSnapshots"
    | "exportEncryptEnabled"
    | "encryptionEnabled";
  const [mergedFieldBusy, setMergedFieldBusy] = useState<Partial<Record<MergedAutoSaveKey, boolean>>>({});
  const [mergedFieldShake, setMergedFieldShake] = useState<Partial<Record<MergedAutoSaveKey, number>>>({});

  async function autoSaveField<K extends MergedAutoSaveKey>(
    key: K,
    next: Settings[K],
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void
  ): Promise<boolean> {
    const prev = settings?.[key];
    setSettings((s) => (s ? { ...s, [key]: next } : s));
    setMergedFieldBusy((b) => ({ ...b, [key]: true }));
    const ok = await save({ [key]: next } as Partial<Settings>, setSaveState, setSaveError);
    setMergedFieldBusy((b) => ({ ...b, [key]: false }));
    if (!ok) {
      // Roll back to the pre-click state; save() already pushed the reason.
      // Text and number fields use debouncedSave instead, which has no revert.
      setSettings((s) => (s ? { ...s, [key]: prev as Settings[K] } : s));
      setMergedFieldShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
    }
    return ok;
  }

  // autoSaveToggle is the same flip, save, revert and shake for any boolean
  // Settings key, with one busy/shake map keyed by field name. The narrower
  // unions above name a group of related toggles on one card; this one is for
  // standalone toggles that belong to no such group.
  const [fieldBusy, setFieldBusy] = useState<Partial<Record<keyof Settings, boolean>>>({});
  const [fieldShake, setFieldShake] = useState<Partial<Record<keyof Settings, number>>>({});

  async function autoSaveToggle<K extends keyof Settings>(
    key: K,
    next: Settings[K],
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void
  ): Promise<boolean> {
    const prev = settings?.[key];
    setSettings((s) => (s ? { ...s, [key]: next } : s));
    setFieldBusy((b) => ({ ...b, [key]: true }));
    const ok = await save({ [key]: next } as Partial<Settings>, setSaveState, setSaveError);
    setFieldBusy((b) => ({ ...b, [key]: false }));
    if (!ok) {
      setSettings((s) => (s ? { ...s, [key]: prev as Settings[K] } : s));
      setFieldShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
    }
    return ok;
  }

  // debouncedSave runs `run` DEBOUNCE_MS after the last edit to the same `key`,
  // for text and number fields that should not be persisted on every
  // keystroke. There is no revert on failure: save() toasts it, and reverting a
  // field the user may still be typing into would fight them. The key is chosen
  // by the caller, so one debounce can cover fields saved together (every
  // registry row shares "registryAuths").
  //
  // Each entry keeps the pending write next to its timer, so a debounce can be
  // completed early (flushDebounces) instead of only cancelled.
  type PendingWrite = { timer: ReturnType<typeof setTimeout>; run: () => void };
  const debounceTimers = useRef<Record<string, PendingWrite>>({});
  const DEBOUNCE_MS = 800;
  // importing is true for the whole import window, from the click until the
  // reloaded configuration is installed, not only while the import is at the
  // head of the write queue. See applyImportedSettings for what it protects.
  const importing = useRef(false);

  function debouncedSave(key: string, run: () => void) {
    // An import is replacing the configuration this edit was typed against, so
    // arming it would only queue a write that lands on top of the imported one.
    // Dropped rather than deferred, for the same reason cancelAllDebounces
    // drops the edits that were already armed when the import started.
    if (importing.current) return;
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing.timer);
    debounceTimers.current[key] = {
      run,
      timer: setTimeout(() => {
        delete debounceTimers.current[key];
        run();
      }, DEBOUNCE_MS),
    };
  }

  function cancelDebounce(key: string) {
    const existing = debounceTimers.current[key];
    if (existing) {
      clearTimeout(existing.timer);
      delete debounceTimers.current[key];
    }
  }

  // flushDebounces sends every pending edit now instead of waiting out its
  // delay. The page needs both this and cancelling: an import replaces the
  // configuration a pending edit was typed against, so that edit is dropped
  // (cancelAllDebounces); leaving the page invalidates nothing, so those edits
  // are sent.
  //
  // Entries are removed from the map before their write runs, so a flush never
  // double-sends and a write that queues another edit is not re-collected. The
  // map object is mutated in place, never replaced, because the unmount effect
  // captures it.
  function flushDebounces() {
    for (const key of Object.keys(debounceTimers.current)) {
      const pending = debounceTimers.current[key];
      delete debounceTimers.current[key];
      clearTimeout(pending.timer);
      pending.run();
    }
  }

  // cancelAllDebounces drops every pending edit that has not been sent yet. Its
  // only caller is the settings import: a debounce armed seconds earlier would
  // land on top of the imported configuration with a value typed against the
  // old one. The `importing` guard above covers the rest of that window.
  function cancelAllDebounces() {
    for (const key of Object.keys(debounceTimers.current)) cancelDebounce(key);
  }

  // applyImportedSettings is the Import button's write. It joins the same queue
  // every save uses, so a save already in flight cannot land on the fresh
  // configuration, and then reloads the page state from the server. Keeping the
  // pre-import baseline would let the next click on any toggle PUT the whole
  // pre-import object back and undo the import.
  //
  // If the reload fails, the page shows the load error rather than work from a
  // baseline it knows is stale; that unmounts every card, so no stale save can
  // happen. The import itself may have been applied by then, in whole or in
  // part.
  //
  // Pending debounces are dropped here at the click, not inside the queued
  // body: a save in flight can hold the import back for a round trip, and a
  // debounce armed just before the click would elapse meanwhile and queue its
  // write behind the import, landing a value typed against the replaced
  // configuration. `importing` keeps the map empty for the rest of the window.
  async function applyImportedSettings(fileText: string) {
    importing.current = true;
    cancelAllDebounces();
    return queueSettingsWrite(async () => {
      try {
        return await importSettingsApply(fileText);
      } finally {
        // Reloaded whatever the answer: an import that fails partway, or whose
        // answer is lost, has still written part of the file.
        await reloadAfterImport();
        importing.current = false;
      }
    });
  }

  async function reloadAfterImport() {
    const fresh = await getSettings().catch(() => null);
    if (fresh?.ok) {
      installSettings(fresh.settings);
      if (fresh.hostMountRoot) setHostMountRoot(fresh.hostMountRoot);
      if (fresh.platform) setPlatformKind(fresh.platform);
      setPlatformSet(fresh.platformSet === true);
    } else {
      setLoadError("import");
    }
    // Domains may have been switched on or off, and the off-site targets,
    // destinations, named repositories and placement replaced: whatever shows
    // them reads again, as after a save of their own.
    window.dispatchEvent(new Event("bv:settings-changed"));
    offsiteTargetsChanged();
    destinationsChanged();
    reposChanged();
    storageLocationsChanged();
    placementChanged();
  }

  // Leaving the page commits pending edits rather than discarding them. The
  // debounce is the only thing that writes a text field, so clearing it would
  // silently lose an edit made within 800ms of navigating away. scheduleField
  // and debouncedSave capture their values explicitly, and a setState after
  // unmount is a no-op, so flushing is safe. The card-level debounce maps
  // (FlashZipExportCard, FleetSettingsCard, CloudCard, NotifyCard) also
  // complete their pending writes on unmount.
  //
  // The cleanup closes over the flushDebounces it had at mount. That reads
  // debounceTimers.current, whose identity never changes, so it finds the live
  // entries.
  const flushOnUnmount = flushDebounces;
  useEffect(() => {
    return () => {
      flushOnUnmount();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // scheduleField saves a cadence or cron field: an optimistic update plus a
  // debouncedSave keyed by field name, so rapid changes to one field (typing a
  // cron expression, stepping through time values) coalesce into one PATCH.
  function scheduleField<K extends keyof Settings>(key: K, value: Settings[K]) {
    setSettings((prev) => (prev ? { ...prev, [key]: value } : prev));
    debouncedSave(String(key), () => {
      void save({ [key]: value } as Partial<Settings>, setSchedSaveState, setSchedSaveError);
    });
  }

  // autoSaveScheduleField saves a plain boolean of the schedules page. A click
  // is not continuous typing, so it saves at once and reverts and shakes on
  // failure, like autoSaveField.
  async function autoSaveScheduleField<K extends ScheduleBoolKey>(key: K, next: Settings[K]): Promise<boolean> {
    const prev = settings?.[key];
    setSettings((s) => (s ? { ...s, [key]: next } : s));
    setSchedFieldBusy((b) => ({ ...b, [key]: true }));
    const ok = await save({ [key]: next } as Partial<Settings>, setSchedSaveState, setSchedSaveError);
    setSchedFieldBusy((b) => ({ ...b, [key]: false }));
    if (!ok) {
      setSettings((s) => (s ? { ...s, [key]: prev as Settings[K] } : s));
      setSchedFieldShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
    }
    return ok;
  }

  // scheduleUpdate is RestoreChecksSection's `update` prop: its toggles save at
  // once through autoSaveScheduleField, its cadence and number fields debounce
  // through scheduleField.
  function scheduleUpdate(patch: Partial<Settings>) {
    for (const [key, value] of Object.entries(patch) as [keyof Settings, Settings[keyof Settings]][]) {
      if (key === "drillsEnabled" || key === "offsiteDrillsEnabled" || key === "startTestEnabled") {
        void autoSaveScheduleField(key, value as boolean);
      } else {
        scheduleField(key, value);
      }
    }
  }

  return {
    settings,
    setSettings,
    savedBaseline,
    loadError,
    loadSettings,
    hostMountRoot,
    platformKind,
    platformSet,
    confirmDialog,
    allTargets,
    authEnabled,
    setAuthEnabled,
    totpEnabled,
    setTotpEnabled,
    recoveryLeft,
    setRecoveryLeft,
    minPasswordLen,
    containers,
    vms,
    save,
    debouncedSave,
    cancelDebounce,
    applyImportedSettings,
    fieldPulse,
    toggleDomainEnabled,
    toggleDbDumps,
    domainToggleBusy,
    domainToggleShake,
    autoSaveField,
    mergedFieldBusy,
    mergedFieldShake,
    autoSaveToggle,
    fieldBusy,
    fieldShake,
    scheduleField,
    autoSaveScheduleField,
    scheduleUpdate,
    schedFieldBusy,
    schedFieldShake,
  };
}

/** What a settings page sees: the store once the settings have loaded. */
export type SettingsStore = Omit<ReturnType<typeof useSettingsStore>, "settings"> & { settings: Settings };

export const SettingsStoreContext = createContext<SettingsStore | null>(null);

export function useSettings(): SettingsStore {
  const store = useContext(SettingsStoreContext);
  if (!store) throw new Error("useSettings outside SettingsPage");
  return store;
}
