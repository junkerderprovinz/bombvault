import { createContext, useContext, useEffect, useRef, useState } from "react";
import {
  getAuth,
  getSettings,
  importSettingsApply,
  listContainers,
  listFileSets,
  listOffsiteTargets,
  listVMs,
  listZFSDatasets,
  putSettings,
  type OffsiteTarget,
} from "../../lib/api";
import { subscribeOffsiteTargets, type OffsiteDomain } from "../../lib/useOffsiteTargets";
import { useNamedRepos } from "../../lib/useNamedRepos";
import { useConfirm } from "../../lib/useConfirm";
import { pushSaveWarnings } from "../../lib/placementCodes";
import { directAsk, primaryDirects, retentionLowered } from "../../lib/directRepo";
import type { Settings, Container, VM, FileSetView, ZFSDatasetView } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import type { SaveState } from "./shared";

/** The off-site rules a domain ages by: its own, or the shared ones. */
function offsiteRetentionOf(s: Settings, domain: string) {
  const own = s.ownOffsiteRetention[domain as OffsiteDomain];
  return {
    retentionKeepLast: own ? own.keepLast : s.offsiteRetentionKeepLast,
    retentionKeepDaily: own ? own.keepDaily : s.offsiteRetentionKeepDaily,
    retentionKeepWeekly: own ? own.keepWeekly : s.offsiteRetentionKeepWeekly,
    retentionKeepMonthly: own ? own.keepMonthly : s.offsiteRetentionKeepMonthly,
    retentionKeepYearly: own ? own.keepYearly : s.offsiteRetentionKeepYearly,
  };
}

// useSettingsStore loads the settings and owns every write to them.
// SettingsPage calls it once and hands the result to its pages.
export function useSettingsStore() {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const namedRepos = useNamedRepos();
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
  const fieldDirects = primaryDirects(allTargets, namedRepos);

  const [settings, setSettings] = useState<Settings | null>(null);
  // savedBaseline is the server's last-confirmed state. Every save merges its
  // own fields onto this baseline rather than the live `settings`, so saving
  // one field never commits another card's unsaved edits.
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
  const [loadError, setLoadError] = useState<string | null>(null);

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

  const [, setRetSaveState] = useState<SaveState>("idle");
  const [, setRetSaveError] = useState<string | null>(null);

  // Schedules page. The container list feeds the Containers section's member
  // list; syncSchedules applies the Containers cadence to VMs, Flash, Folders
  // and ZFS.
  const [containers, setContainers] = useState<Container[]>([]);
  // VMs feed the VMs schedule section's per-item override list (#121).
  const [vms, setVMs] = useState<VM[]>([]);
  // File sets feed the Files schedule section's member list (live enabled toggles).
  const [fileSets, setFileSets] = useState<FileSetView[]>([]);
  // ZFS items do the same for the ZFS schedule section.
  const [zfsItems, setZFSItems] = useState<ZFSDatasetView[]>([]);
  const [syncSchedules, setSyncSchedules] = useState(false);
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
  // The "sync" toggle itself isn't a Settings field (syncSchedules above is
  // local UI state derived from whether the domain schedules already match),
  // so it cannot go through autoSaveScheduleField; handleSyncSchedulesToggle
  // below uses this busy/shake pair.
  const [syncToggleBusy, setSyncToggleBusy] = useState(false);
  const [syncToggleShake, setSyncToggleShake] = useState(0);
  // The self-backup card's on/off toggle. configSchedule is a cadence string,
  // not a boolean, so like the sync toggle it gets its own busy/shake pair.
  const [configScheduleToggleBusy, setConfigScheduleToggleBusy] = useState(false);
  const [configScheduleToggleShake, setConfigScheduleToggleShake] = useState(0);
  // Remembers the cadence in force before the self-backup schedule was switched
  // off, so switching it back on restores that instead of the shipped daily
  // 02:00 default: off writes the literal "off" over the stored string.
  //
  // Like FlashZipExportCard's rememberedKeep, it lasts for this page's
  // lifetime. The server stores one cadence string per domain, so a reload
  // cannot bring the old value back, and covering that would take a second
  // persisted field, a second source of truth for the same fact.
  const [rememberedConfigSchedule, setRememberedConfigSchedule] = useState("daily 02:00");

  // installSettings adopts a settings object from the server as both the live
  // state and the confirmed baseline, and re-derives the page state computed
  // from it. The mount load and the reload after a settings import both use
  // it, since an import replaces the whole configuration.
  function installSettings(s: Settings) {
    setSettings(s);
    savedBaseline.current = s;
    // The sync toggle reads as on only when Containers, VMs, Flash, Folders
    // and ZFS already share one cadence that is not off. A domain that differs
    // keeps its own value until the next edit, so the toggle must not claim
    // otherwise.
    setSyncSchedules(
      s.vmsSchedule === s.containersSchedule &&
        s.flashSchedule === s.containersSchedule &&
        s.filesSchedule === s.containersSchedule &&
        s.zfsSchedule === s.containersSchedule &&
        s.containersSchedule !== "off" &&
        s.containersSchedule !== ""
    );
  }

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) {
          installSettings(res.settings);
          if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
          if (res.platform) setPlatformKind(res.platform);
        } else {
          setLoadError("Failed to load settings");
        }
      })
      .catch(() => setLoadError("Failed to load settings"));

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
        // Non-fatal: the Containers schedule section shows an empty member list.
      });

    // The VM list for the per-item overrides (#121).
    listVMs()
      .then((r) => {
        if (r.ok) setVMs(r.vms ?? []);
      })
      .catch(() => {
        // Non-fatal: the VMs schedule section shows an empty per-item list.
      });

    loadFileSets();
    loadZFSItems();
  }, []);

  // loadFileSets runs on mount and after a Files section toggle PATCHes a set,
  // so the member rows track the server state.
  function loadFileSets() {
    listFileSets()
      .then((r) => {
        if (r.ok) setFileSets(r.fileSets ?? []);
      })
      .catch(() => {
        // Non-fatal: the Files schedule section shows an empty member list.
      });
  }

  // loadZFSItems does for the ZFS section what loadFileSets does for Folders.
  function loadZFSItems() {
    listZFSDatasets()
      .then((r) => {
        if (r.ok) setZFSItems(r.datasets ?? []);
      })
      .catch(() => {
        // Non-fatal: the ZFS schedule section shows an empty member list.
      });
  }

  // While sync is on, mirror the Containers cadence onto VMs, Flash, Folders
  // and ZFS in live state, not only in the saved patch, so turning sync off
  // does not snap those editors back to stale values. The equality guard stops
  // the effect from looping. debouncedSave (keyed "schedSync", separate from
  // the containersSchedule key) coalesces rapid edits into one PATCH of the
  // mirrored fields.
  useEffect(() => {
    if (!syncSchedules || !settings) return;
    const merged = settings.containersSchedule;
    if (
      settings.vmsSchedule === merged &&
      settings.flashSchedule === merged &&
      settings.filesSchedule === merged &&
      settings.zfsSchedule === merged
    ) {
      return;
    }
    setSettings((prev) =>
      prev
        ? { ...prev, vmsSchedule: merged, flashSchedule: merged, filesSchedule: merged, zfsSchedule: merged }
        : prev
    );
    debouncedSave("schedSync", () => {
      void save(
        { vmsSchedule: merged, flashSchedule: merged, filesSchedule: merged, zfsSchedule: merged },
        setSchedSaveState,
        setSchedSaveError
      );
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [syncSchedules, settings?.containersSchedule]);

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
    const base = savedBaseline.current ?? settings;
    if (!base) return false;
    setSaveState("saving");
    setSaveError(null);
    // Persist only this card's fields, merged onto the server baseline rather
    // than the live `settings`, which may hold unsaved edits from other cards.
    const updated: Settings = { ...base, ...patch };
    try {
      const res = await putSettings(updated);
      if (res.ok) {
        // Advance the baseline; reflect just the saved fields in the live state so
        // other cards' in-progress edits are left untouched.
        savedBaseline.current = updated;
        setSettings((prev) =>
          prev ? { ...prev, ...patch, ...(echo ? echo(prev) : null) } : updated
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

  // A domain's off-site rules, its own or the shared ones, are copied onto its
  // field target, so a save that lowers them reaches that target's direct
  // repository at once. Declined, the fields go back to what was saved.
  async function saveOffsiteRetention(patch: Partial<Settings>): Promise<boolean> {
    const before = savedBaseline.current;
    if (before) {
      const after = { ...before, ...patch };
      const reached = fieldDirects.filter((u) =>
        retentionLowered(offsiteRetentionOf(before, u.target.domain), offsiteRetentionOf(after, u.target.domain))
      );
      if (reached.length > 0 && !(await confirm(directAsk(t, lang, "offsite.directRetentionAsk", reached)))) {
        const restore = Object.fromEntries(Object.keys(patch).map((k) => [k, before[k as keyof Settings]]));
        setSettings((prev) => (prev ? { ...prev, ...restore } : prev));
        return false;
      }
    }
    return save(patch, setRetSaveState, setRetSaveError);
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
  // happen. The import itself has already been applied.
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
        const res = await importSettingsApply(fileText);
        if (!res.ok) return res;
        const fresh = await getSettings();
        if (fresh.ok) {
          installSettings(fresh.settings);
          if (fresh.hostMountRoot) setHostMountRoot(fresh.hostMountRoot);
          if (fresh.platform) setPlatformKind(fresh.platform);
        } else {
          setLoadError("Settings were imported, but reloading them failed. Reload the page.");
        }
        // Domains may have been switched on or off by the import: the sidebar and
        // layout listen for this and refetch, as they do after a save.
        window.dispatchEvent(new Event("bv:settings-changed"));
        return res;
      } finally {
        importing.current = false;
      }
    });
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

  // handleSyncSchedulesToggle saves the sync toggle. syncSchedules is local
  // state derived on load, not a stored field. Turning it on writes the
  // Containers cadence to VMs, Flash, Folders and ZFS at once, with the usual
  // revert and shake on failure. Turning it off writes nothing, since every
  // field already holds its last-saved value.
  async function handleSyncSchedulesToggle(next: boolean) {
    setSyncSchedules(next);
    if (!next || !settings) return;
    const merged = settings.containersSchedule;
    const prevVms = settings.vmsSchedule;
    const prevFlash = settings.flashSchedule;
    const prevFiles = settings.filesSchedule;
    const prevZFS = settings.zfsSchedule;
    setSettings((s) =>
      s
        ? { ...s, vmsSchedule: merged, flashSchedule: merged, filesSchedule: merged, zfsSchedule: merged }
        : s
    );
    setSyncToggleBusy(true);
    const ok = await save(
      { vmsSchedule: merged, flashSchedule: merged, filesSchedule: merged, zfsSchedule: merged },
      setSchedSaveState,
      setSchedSaveError
    );
    setSyncToggleBusy(false);
    if (!ok) {
      setSyncSchedules(false);
      setSettings((s) =>
        s
          ? { ...s, vmsSchedule: prevVms, flashSchedule: prevFlash, filesSchedule: prevFiles, zfsSchedule: prevZFS }
          : s
      );
      setSyncToggleShake((n) => n + 1);
    }
  }

  // toggleConfigSchedule is the self-backup card's on/off toggle. Off writes
  // the literal "off" cadence; on restores the cadence in force before the
  // last off (rememberedConfigSchedule), or "daily 02:00" when there is none.
  // There is no separate configScheduleEnabled field: parseCadenceString and
  // buildCadenceString already round-trip "off", so a boolean would be a second
  // source of truth. `configEnabled` in the Domains card is a different thing,
  // whether the self-backup domain exists at all.
  async function toggleConfigSchedule(next: boolean) {
    const prev = settings?.configSchedule ?? "off";
    // Switching off is the only moment the cadence is lost, and `prev` is the
    // value being overwritten, wherever it came from.
    if (!next && prev && prev !== "off") setRememberedConfigSchedule(prev);
    const value = next ? rememberedConfigSchedule : "off";
    setSettings((s) => (s ? { ...s, configSchedule: value } : s));
    setConfigScheduleToggleBusy(true);
    const ok = await save({ configSchedule: value }, setSchedSaveState, setSchedSaveError);
    setConfigScheduleToggleBusy(false);
    if (!ok) {
      setSettings((s) => (s ? { ...s, configSchedule: prev } : s));
      setConfigScheduleToggleShake((n) => n + 1);
    }
  }

  return {
    settings,
    setSettings,
    savedBaseline,
    loadError,
    hostMountRoot,
    platformKind,
    confirmDialog,
    allTargets,
    fieldDirects,
    authEnabled,
    setAuthEnabled,
    totpEnabled,
    setTotpEnabled,
    recoveryLeft,
    setRecoveryLeft,
    minPasswordLen,
    containers,
    vms,
    fileSets,
    zfsItems,
    loadFileSets,
    loadZFSItems,
    save,
    saveOffsiteRetention,
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
    syncSchedules,
    handleSyncSchedulesToggle,
    syncToggleBusy,
    syncToggleShake,
    toggleConfigSchedule,
    configScheduleToggleBusy,
    configScheduleToggleShake,
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
