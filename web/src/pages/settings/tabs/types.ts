import type { Dispatch, RefObject, SetStateAction } from "react";
import type {
  Container,
  FileSetView,
  ImportSettingsResponse,
  OffsiteTarget,
  RegistryAuthEntry,
  Settings,
  VM,
} from "../../../lib/api";
import type { RainbowState } from "../../../lib/appearance";
import type { ControlAxis, LabelMode } from "../../../lib/controls";
import type { DirectUse } from "../../../lib/directRepo";
import type { useT } from "../../../lib/i18n";
import type { MotionIntensity } from "../../../lib/motion";
import type { Shape } from "../../../lib/shape";
import type { OffsiteDomain } from "../../../lib/useOffsiteTargets";
import type { useReveal } from "../../../lib/useReveal";
import type { SaveState } from "../shared";

export type DomainToggleKey =
  | "containersEnabled"
  | "vmsEnabled"
  | "flashEnabled"
  | "filesEnabled"
  | "configEnabled"
  | "receiverEnabled"
  | "pullEnabled"
  | "fleetEnabled";

export type ScheduleBoolKey =
  | "perItemSchedules"
  | "catchUpMissed"
  | "drillsEnabled"
  | "offsiteDrillsEnabled"
  | "restartHealthWait";

export type MergedAutoSaveKey =
  | "pruneImageAfterUpdate"
  | "reconcileUnraidUpdateStatus"
  | "exportEncryptEnabled"
  | "encryptionEnabled";

export type OffsiteRetentionKey =
  | "offsiteRetentionKeepLast"
  | "offsiteRetentionKeepDaily"
  | "offsiteRetentionKeepWeekly"
  | "offsiteRetentionKeepMonthly";

type Setter<T> = Dispatch<SetStateAction<T>>;
type SetSaveState = (s: SaveState) => void;
type SetSaveError = (e: string | null) => void;
type Reveal = ReturnType<typeof useReveal>;

type SaveSettings = (
  patch: Partial<Settings>,
  setSaveState: SetSaveState,
  setSaveError: SetSaveError,
  echo?: (live: Settings) => Partial<Settings>
) => Promise<boolean>;

/** What SettingsPage hands every tab: its state, the setters the cards write,
 *  and the save paths. The page owns all of it, so every tab shares one
 *  settings object and one write queue. */
export type SettingsTabProps = {
  t: ReturnType<typeof useT>["t"];
  advanced: boolean;
  quiet: boolean;
  setQuiet: (next: boolean) => void;
  allTargets: OffsiteTarget[];
  fieldDirects: DirectUse[];
  settings: Settings;
  setSettings: Setter<Settings | null>;
  savedBaseline: RefObject<Settings | null>;
  hostMountRoot: string;
  platformKind: string;
  authEnabled: boolean;
  totpEnabled: boolean;
  setTotpEnabled: Setter<boolean>;
  recoveryLeft: number | undefined;
  setRecoveryLeft: Setter<number | undefined>;
  minPasswordLen: number;
  pwNew: string;
  setPwNew: Setter<string>;
  pwConfirm: string;
  setPwConfirm: Setter<string>;
  pwSaveState: SaveState;
  pwSaveMsg: string | null;
  pwSaveShake: number;
  revealPwNew: Reveal;
  revealPwConfirm: Reveal;
  revealMetricsToken: Reveal;
  registryTokenVisible: Record<string, boolean>;
  setRegistryTokenVisible: Setter<Record<string, boolean>>;
  registryRowIds: string[];
  setRegistryRowIds: Setter<string[]>;
  shape: Shape;
  setShapeLocal: Setter<Shape>;
  motion: MotionIntensity;
  setMotionLocal: Setter<MotionIntensity>;
  stormFound: boolean;
  setStormFound: Setter<boolean>;
  stormClicks: RefObject<{ taps: number }>;
  discoFound: boolean;
  disco: boolean;
  setDiscoLocal: Setter<boolean>;
  labelModes: Record<ControlAxis, LabelMode>;
  setLabelModes: Setter<Record<ControlAxis, LabelMode>>;
  rainbow: RainbowState;
  updateRainbow: (patch: Partial<RainbowState>) => void;
  rainbowToggled: (on: boolean) => void;
  setEncSaveState: SetSaveState;
  setEncSaveError: SetSaveError;
  kitError: string | null;
  setKitError: Setter<string | null>;
  setPathSaveState: SetSaveState;
  setPathSaveError: SetSaveError;
  setExportEncSaveState: SetSaveState;
  setExportEncSaveError: SetSaveError;
  setOffsiteSaveState: SetSaveState;
  setOffsiteSaveError: SetSaveError;
  offsiteWizard: OffsiteDomain | null;
  setOffsiteWizard: Setter<OffsiteDomain | null>;
  domainToggleBusy: Partial<Record<DomainToggleKey, boolean>>;
  domainToggleShake: Partial<Record<DomainToggleKey, number>>;
  setRetSaveState: SetSaveState;
  setRetSaveError: SetSaveError;
  setPruneSaveState: SetSaveState;
  setPruneSaveError: SetSaveError;
  setReconcileSaveState: SetSaveState;
  setReconcileSaveError: SetSaveError;
  setCacheSaveState: SetSaveState;
  setCacheSaveError: SetSaveError;
  setCoresSaveState: SetSaveState;
  setCoresSaveError: SetSaveError;
  setLimSaveState: SetSaveState;
  setLimSaveError: SetSaveError;
  setMetricsSaveState: SetSaveState;
  setMetricsSaveError: SetSaveError;
  setDigestSaveState: SetSaveState;
  setDigestSaveError: SetSaveError;
  setWatchdogSaveState: SetSaveState;
  setWatchdogSaveError: SetSaveError;
  containers: Container[];
  vms: VM[];
  fileSets: FileSetView[];
  syncSchedules: boolean;
  schedFieldBusy: Partial<Record<ScheduleBoolKey, boolean>>;
  schedFieldShake: Partial<Record<ScheduleBoolKey, number>>;
  syncToggleBusy: boolean;
  syncToggleShake: number;
  configScheduleToggleBusy: boolean;
  configScheduleToggleShake: number;
  loadFileSets: () => void;
  fieldPulse: Partial<Record<keyof Settings, number>>;
  save: SaveSettings;
  saveOffsiteRetention: (key: OffsiteRetentionKey, n: number) => Promise<void>;
  toggleDomainEnabled: (key: DomainToggleKey, next: boolean) => Promise<void>;
  mergedFieldBusy: Partial<Record<MergedAutoSaveKey, boolean>>;
  mergedFieldShake: Partial<Record<MergedAutoSaveKey, number>>;
  autoSaveField: <K extends MergedAutoSaveKey>(
    key: K,
    next: Settings[K],
    setSaveState: SetSaveState,
    setSaveError: SetSaveError
  ) => Promise<boolean>;
  fieldBusy: Partial<Record<keyof Settings, boolean>>;
  fieldShake: Partial<Record<keyof Settings, number>>;
  autoSaveToggle: <K extends keyof Settings>(
    key: K,
    next: Settings[K],
    setSaveState: SetSaveState,
    setSaveError: SetSaveError
  ) => Promise<boolean>;
  debouncedSave: (key: string, run: () => void) => void;
  cancelDebounce: (key: string) => void;
  applyImportedSettings: (fileText: string) => Promise<ImportSettingsResponse>;
  saveRegistries: (nextAuths: RegistryAuthEntry[], nextRowIds: string[]) => void;
  scheduleField: <K extends keyof Settings>(key: K, value: Settings[K]) => void;
  autoSaveScheduleField: <K extends ScheduleBoolKey>(key: K, next: Settings[K]) => Promise<boolean>;
  scheduleUpdate: (patch: Partial<Settings>) => void;
  handleSyncSchedulesToggle: (next: boolean) => Promise<void>;
  toggleConfigSchedule: (next: boolean) => Promise<void>;
  handleSetPassword: () => Promise<void>;
};
