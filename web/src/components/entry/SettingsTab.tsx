import { useState } from "react";
import {
  patchFileSet,
  patchZFSDataset,
  setScheduleCadence,
  setVMScheduleCadence,
  type AnomalyItem,
  type Container,
  type FileSetView,
  type OkEnvelope,
  type PlacementDomain,
  type PlacementView,
  type Settings,
  type VM,
  type ZFSDatasetView,
} from "../../lib/api";
import { ENTRY_KIND } from "../../lib/backupEntry";
import { isGone } from "../../lib/backupList";
import { dumpRuns, updateWarnKey } from "../../lib/dbdump";
import type { Entry } from "../../lib/entry";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { tLtr } from "../../lib/ltrFragments";
import { lockHint, viewHomeLabel } from "../../lib/placement";
import { useHostLabel } from "../../lib/useHostLabel";
import { usePlacementOptions } from "../../lib/usePlacementOptions";
import { containerTakeover, vmTakeover, type TakeoverEntry } from "../../lib/useTakeOver";
import { FlashZipExportCard } from "../../pages/settings/FlashZipExportCard";
import { Card, ToggleRow } from "../../pages/settings/shared";
import { KIND_SCHEDULE_LABEL } from "../backups/kinds";
import { Button } from "../Button";
import type { CadenceMode } from "../CadenceBuilder";
import { ConfigSettingsCard } from "../config/ConfigSettingsCard";
import { ExcludesEditor } from "../containers/ExcludesEditor";
import { FoldersEditor } from "../containers/FoldersEditor";
import { HooksEditor } from "../containers/HooksEditor";
import { StopContainersEditor } from "../containers/StopContainersEditor";
import { UpdateAfterBackupRow } from "../containers/UpdateAfterBackupRow";
import { DatabaseDumpRow } from "../DatabaseDumpRow";
import { EffectiveScheduleLine } from "../EffectiveScheduleLine";
import { FileSetDialog } from "../files/FileSetDialog";
import { FileSetFoldersEditor, fileSetEditorKey } from "../files/FileSetFoldersEditor";
import { FormerNames } from "../FormerNames";
import { IdleWaitLine, IdleWaitRow } from "../IdleWaitRow";
import { IncludeToggle } from "../IncludeToggle";
import { InfoBubble } from "../InfoBubble";
import { IconPencil } from "../navGlyphs";
import { PlacementFlow } from "../placement/PlacementFlow";
import { PlacementRow } from "../placement/PlacementRow";
import { PlanButton } from "../PlanButton";
import { SettingRow } from "../storage/rows";
import { VMBlockToggle } from "../VMBlockToggle";
import { VMMethodSelect } from "../vms/VMMethodSelect";
import { ReplicaCard } from "../zfs/replica/ReplicaCard";
import { ReplicaPlanLine } from "../zfs/replica/ReplicaPlanLine";
import { ZFSItemSettings } from "../zfs/ZFSItemSettings";
import { ZFSSafetySection } from "../zfs/ZFSSafetySection";
import { ZFSSitesLine } from "../zfs/ZFSSitesLine";
import { DumpScene } from "./DumpScene";

// What stores an entry's own schedule, per kind. Flash and the self-backup
// run on their kind's schedule and have none.
const SAVE_CADENCE: Partial<Record<Entry["item"]["kind"], (key: string, cadence: string) => Promise<OkEnvelope>>> = {
  container: setScheduleCadence,
  vm: setVMScheduleCadence,
  files: (id, scheduleCadence) => patchFileSet(id, { scheduleCadence }),
  zfs: (id, scheduleCadence) => patchZFSDataset(id, { scheduleCadence }),
};

// What an entry's own plan starts from. The server refuses Every N days for
// one, since nothing records when it last ran, and Off is the switch above it.
const NEW_OWN_PLAN = "daily 03:00";
const OWN_PLAN_MODES: CadenceMode[] = ["daily", "weekly", "cron"];

// The switch rows of a container hug the end of the old card's corner. In a
// card of their own they span it, label at the start and switch at the end.
const SPAN_ROWS = "flex flex-col gap-4 [&>div]:items-stretch";

function ownCadence(entry: Entry): string {
  const { detail } = entry;
  switch (detail.kind) {
    case "container":
      return detail.container.scheduleCadence ?? "";
    case "vm":
      return detail.vm.scheduleCadence ?? "";
    case "files":
      return detail.set.scheduleCadence ?? "";
    case "zfs":
      return detail.dataset.scheduleCadence;
    default:
      return "";
  }
}

// A stored "off" pauses the entry and is no plan of its own.
function hasOwnPlan(cadence: string): boolean {
  return cadence !== "" && cadence !== "off";
}

function placementOf(entry: Entry): PlacementView | null {
  const { detail } = entry;
  switch (detail.kind) {
    case "container":
      return detail.container.placement;
    case "vm":
      return detail.vm.placement;
    case "files":
      return detail.set.placement;
    default:
      return null;
  }
}

/**
 * ScheduleCard holds what every kind has: whether the entry runs with the
 * schedule, when, and where its backups go. Both follow the default of the
 * kind until the entry is given a value of its own.
 */
function ScheduleCard({
  entry,
  name,
  onChanged,
  onPlacement,
  hueIndex,
}: {
  entry: Entry;
  name: string;
  onChanged: () => void;
  onPlacement: (next: PlacementView) => void;
  hueIndex: number;
}) {
  const { t } = useT();
  const { push } = useToast();
  const { item, detail, settings } = entry;
  const spec = ENTRY_KIND[item.kind];
  const [own, setOwn] = useState(() => ownCadence(entry));
  const [busy, setBusy] = useState(false);
  const setIncluded = spec.setIncluded;
  const saveCadence = SAVE_CADENCE[item.kind];
  const placement = placementOf(entry);
  const pausedHint = t("schedule.pausedHint")
    .replace("{everything}", t("settings.everythingName"))
    .replace("{backupNow}", t("containers.backupNow"));

  async function saveOwn(cadence: string) {
    if (!saveCadence) return;
    const before = own;
    setOwn(cadence);
    setBusy(true);
    try {
      const res = await saveCadence(item.key, cadence);
      if (res.ok) onChanged();
      else {
        setOwn(before);
        push(res.error ?? t("schedule.updateFailed"), "fail");
      }
    } catch (err) {
      setOwn(before);
      push(err instanceof Error ? err.message : t("schedule.updateFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card title={t("entry.settings.schedule")} hueIndex={hueIndex} newId="entry-defaults">
      {setIncluded && (
        <IncludeToggle
          name={item.key}
          initial={item.included}
          save={setIncluded}
          onSaved={onChanged}
          label={t("entry.settings.inSchedule")}
          hint={isGone(item) ? t("entry.settings.inScheduleGone") : pausedHint}
        />
      )}
      {saveCadence && settings.perItemSchedules && !isGone(item) && (
        <>
          <ToggleRow
            label={t("entry.settings.ownPlan")}
            hint={t("entry.settings.ownPlanHint")}
            checked={hasOwnPlan(own)}
            disabled={busy}
            onChange={(on) => void saveOwn(on ? NEW_OWN_PLAN : "")}
          />
          {hasOwnPlan(own) && (
            <SettingRow label={t("entry.settings.plan")}>
              <PlanButton
                label={`${t("entry.settings.plan")}: ${name}`}
                value={own}
                modes={OWN_PLAN_MODES}
                onChange={(cadence) => void saveOwn(cadence)}
                hueIndex={hueIndex}
              />
            </SettingRow>
          )}
        </>
      )}
      <EffectiveScheduleLine effective={item.effectiveSchedule} domainLabelKey={KIND_SCHEDULE_LABEL[item.kind]} />
      {placement && spec.placementDomain && (
        <PlacementRow
          item={{ domain: spec.placementDomain, key: item.key }}
          name={name}
          view={placement}
          onView={onPlacement}
        />
      )}
      {(item.kind === "flash" || item.kind === "config") && <PlacementFlow domain={item.kind} />}
      {detail.kind === "zfs" && (
        <>
          <ReplicaPlanLine itemId={detail.dataset.id} />
          <ZFSSitesLine item={detail.dataset} />
        </>
      )}
    </Card>
  );
}

/** RepositoryCard names the repository the entry's backups are written to,
 *  which is fixed once the first backup exists. */
function RepositoryCard({
  domain,
  view,
  hueIndex,
}: {
  domain: PlacementDomain;
  view: PlacementView;
  hueIndex: number;
}) {
  const { t } = useT();
  const host = useHostLabel();
  const { options } = usePlacementOptions(domain);
  if (!options) return null;
  const home = viewHomeLabel(t, host, view, options);
  return (
    <Card title={t("entry.settings.repository")} hueIndex={hueIndex}>
      <SettingRow label={t("placement.storedOn")} hint={t("entry.settings.storedOnHint")}>
        <span className="text-sm text-carbon-textSub wrap-anywhere">{home}</span>
        <InfoBubble tip={view.locked ? lockHint(t, "home-fixed", home) : t("entry.settings.fromFirstBackup")} />
      </SettingRow>
    </Card>
  );
}

function ContainerCards({
  container,
  installed,
  onChanged,
  nextHue,
}: {
  container: Container;
  installed: Container[];
  onChanged: () => void;
  nextHue: () => number;
}) {
  const { t } = useT();
  const aliases = container.aliases ?? [];
  const takeover: TakeoverEntry = { name: container.name, displayName: container.name, api: containerTakeover };
  return (
    <>
      {container.dbTier !== "" && (
        <Card title={t("entry.settings.database")} hint={t("entry.settings.databaseHint")} hueIndex={nextHue()}>
          <div className="grid items-start gap-x-8 gap-y-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,380px)]">
            <div className={SPAN_ROWS}>
              <DatabaseDumpRow container={container} t={t} onSaved={onChanged} />
            </div>
            <DumpScene on={dumpRuns(container)} stops={container.dbDataCoverage === "stopped"} />
          </div>
        </Card>
      )}
      <Card title={t("entry.settings.container")} hueIndex={nextHue()}>
        <div className={SPAN_ROWS}>
          <UpdateAfterBackupRow
            name={container.name}
            initial={container.updateAfterBackup ?? false}
            lastUpdateCheck={container.lastUpdateCheck}
            lastUpdateResult={container.lastUpdateResult}
            databaseWarn={updateWarnKey(container)}
            t={t}
          />
          <IdleWaitRow name={container.name} initial={container.idleWaitHours ?? 0} onSaved={onChanged} />
        </div>
        <IdleWaitLine name={container.name} />
      </Card>
      <Card title={t("folders.title")} hint={t("folders.hint")} hueIndex={nextHue()}>
        <FoldersEditor name={container.name} stack={container.stack} open t={t} lastBackup={container.lastBackup} />
      </Card>
      <Card title={t("excludes.title")} hint={tLtr(t, "excludes.hint")} hueIndex={nextHue()}>
        <ExcludesEditor name={container.name} initial={container.excludes ?? []} open t={t} />
      </Card>
      <Card title={t("stophook.title")} hint={t("stophook.hint")} hueIndex={nextHue()}>
        <StopContainersEditor
          name={container.name}
          initial={container.stopContainers ?? []}
          installedContainers={installed}
          open
          t={t}
        />
      </Card>
      <Card title={t("entry.settings.hooks")} hint={t("hooks.hint")} hueIndex={nextHue()}>
        <HooksEditor name={container.name} initialPre={container.preHook} initialPost={container.postHook} open t={t} />
      </Card>
      {aliases.length > 0 && (
        <Card title={t("entry.settings.formerNames")} hueIndex={nextHue()}>
          <FormerNames
            aliases={aliases}
            conflicts={container.aliasConflicts ?? []}
            entry={takeover}
            onDone={onChanged}
            t={t}
          />
        </Card>
      )}
    </>
  );
}

function VMCards({ vm, onChanged, nextHue }: { vm: VM; onChanged: () => void; nextHue: () => number }) {
  const { t } = useT();
  const aliases = vm.aliases ?? [];
  return (
    <>
      <Card title={t("entry.settings.vm")} hueIndex={nextHue()}>
        <SettingRow label={t("vm.method")} hint={t("vm.method.hint")}>
          <div className="w-80 max-w-full">
            <VMMethodSelect name={vm.libvirtName} initial={vm.method} t={t} />
          </div>
        </SettingRow>
        <VMBlockToggle vm={vm} />
      </Card>
      {aliases.length > 0 && (
        <Card title={t("entry.settings.formerNames")} hueIndex={nextHue()}>
          <FormerNames
            aliases={aliases}
            conflicts={vm.aliasConflicts ?? []}
            entry={{ name: vm.libvirtName, displayName: vm.name, api: vmTakeover }}
            onDone={onChanged}
            t={t}
          />
        </Card>
      )}
    </>
  );
}

function FilesCard({
  set,
  hostMountRoot,
  onChanged,
  hueIndex,
}: {
  set: FileSetView;
  hostMountRoot: string;
  onChanged: () => void;
  hueIndex: number;
}) {
  const { t } = useT();
  const [editing, setEditing] = useState(false);
  return (
    <Card title={t("files.path")} hueIndex={hueIndex}>
      <SettingRow label={set.name} note={set.path === "" ? t("files.noPath") : undefined}>
        {set.path !== "" && (
          <span dir="ltr" className="font-mono text-xs text-carbon-textSub text-start wrap-anywhere">
            {hostMountRoot}/{set.path}
          </span>
        )}
        <Button label={t("common.edit")} labelKey="common.edit" glyph={<IconPencil />} onClick={() => setEditing(true)} />
      </SettingRow>
      {/* Keyed, because the editor seeds from its first props and has to
          start over after the folder changed. */}
      {set.path !== "" && (
        <FileSetFoldersEditor key={fileSetEditorKey(set)} set={set} hostMountRoot={hostMountRoot} t={t} />
      )}
      {editing && (
        <FileSetDialog
          initial={set}
          presetSeed={null}
          hostMountRoot={hostMountRoot}
          t={t}
          onClose={() => setEditing(false)}
          onSaved={() => {
            setEditing(false);
            onChanged();
          }}
        />
      )}
    </Card>
  );
}

function ZFSCards({
  dataset,
  anomaly,
  onChanged,
  nextHue,
}: {
  dataset: ZFSDatasetView;
  anomaly?: AnomalyItem;
  onChanged: () => void;
  nextHue: () => number;
}) {
  const { t } = useT();
  // What detection has learned about each dataset stands beside it in the list.
  const series = new Map((anomaly?.datasets ?? []).map((s) => [s.part, s]));
  return (
    <>
      <Card title={t("entry.settings.zfs")} hueIndex={nextHue()}>
        <ZFSItemSettings
          item={dataset}
          t={t}
          onChanged={onChanged}
          anomalyEnabled={false}
          series={series}
          embedded
        />
        {dataset.safetyCount > 0 && <ZFSSafetySection item={dataset} t={t} onRefresh={onChanged} />}
      </Card>
      <ReplicaCard itemId={dataset.id} name={dataset.dataset} />
    </>
  );
}

function ConfigCard({ settings: stored, hueIndex }: { settings: Settings; hueIndex: number }) {
  const { t } = useT();
  // The card writes the switch itself and shows it from what it is handed.
  const [settings, setSettings] = useState(stored);
  return <ConfigSettingsCard t={t} settings={settings} setSettings={(update) => setSettings(update)} hueIndex={hueIndex} />;
}

/**
 * SettingsTab holds what can be set about one entry: the schedule and the
 * storage locations every kind has, then the cards of its own kind. An entry
 * that is gone keeps only the first, which is what stops it being skipped at
 * every run.
 */
export function SettingsTab({
  entry,
  name,
  anomaly,
  onChanged,
  onPlacement,
  nextHue,
}: {
  entry: Entry;
  name: string;
  /** What anomaly detection knows about the entry. */
  anomaly?: AnomalyItem;
  onChanged: () => void;
  onPlacement: (next: PlacementView) => void;
  nextHue: () => number;
}) {
  const { t } = useT();
  const { item, detail, settings, hostMountRoot } = entry;
  const placement = placementOf(entry);
  const domain = ENTRY_KIND[item.kind].placementDomain;
  const gone = isGone(item);

  return (
    <>
      <ScheduleCard entry={entry} name={name} onChanged={onChanged} onPlacement={onPlacement} hueIndex={nextHue()} />
      {placement && domain && <RepositoryCard domain={domain} view={placement} hueIndex={nextHue()} />}
      {detail.kind === "container" && !gone && (
        <ContainerCards
          container={detail.container}
          installed={detail.installed}
          onChanged={onChanged}
          nextHue={nextHue}
        />
      )}
      {detail.kind === "vm" && !gone && <VMCards vm={detail.vm} onChanged={onChanged} nextHue={nextHue} />}
      {detail.kind === "files" && (
        <FilesCard set={detail.set} hostMountRoot={hostMountRoot} onChanged={onChanged} hueIndex={nextHue()} />
      )}
      {detail.kind === "zfs" && <ZFSCards dataset={detail.dataset} anomaly={anomaly} onChanged={onChanged} nextHue={nextHue} />}
      {detail.kind === "flash" && <FlashZipExportCard t={t} hueIndex={nextHue()} />}
      {detail.kind === "config" && <ConfigCard settings={settings} hueIndex={nextHue()} />}
    </>
  );
}
