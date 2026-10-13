// The one window that restores an entry of any kind. It asks in the order a
// restore is decided: which backup and from which storage location, what of
// it, and where to. The check and what will happen stand above the button,
// and a target that replaces the original asks for its name first.
import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  flashDownloadURL,
  listDbDumps,
  openedAsPairedPhone,
  listSnapshotFiles,
  listSnapshotFilesFileSet,
  listSnapshotFilesZFS,
  zfsHostDatasets,
  zfsRestorePoints,
  type BackupItem,
  type DBDumpView,
  type FileEntry,
  type TimelineMark,
  type TimelineRow,
  type ZFSHostDataset,
  type ZFSRestorePoint,
} from "../../lib/api";
import { ENTRY_KIND } from "../../lib/backupEntry";
import { isGone, itemProgressKey } from "../../lib/backupList";
import { useBackupWatch } from "../../lib/backupWatch";
import { copyText } from "../../lib/clipboard";
import { ENGINE_NAMES, pairsWith } from "../../lib/dbdump";
import { actionMark, comparable, rowHolds, rowTime, type Entry } from "../../lib/entry";
import { loadErrorMessage } from "../../lib/errors";
import { useT, type TranslationKey } from "../../lib/i18n";
import { isolateLtr, tLtr } from "../../lib/ltrFragments";
import { placementChanged } from "../../lib/placementEvents";
import { anyActive, busyPhraseKey, useProgress } from "../../lib/progress";
import { formatRecent } from "../../lib/reltime";
import {
  backsUpFirst,
  defaultWhere,
  emergencyPath,
  findingsOf,
  recommend,
  recommendationWhy,
  requestedRow,
  restoreFolderFor,
  snapshotIds,
  whatsFor,
  wheresFor,
  type RestoreWhat,
  type RestoreWhere,
} from "../../lib/restorePlan";
import { secretsSentence } from "../../lib/secretsRefusal";
import { SNAPSHOT_MISSING, autoMark, newestId, sourceOfPlace } from "../../lib/timeline";
import { useToast } from "../../lib/toast";
import { useOpenAnomalies } from "../../lib/useAnomalies";
import { useConfirm } from "../../lib/useConfirm";
import { restoreBlockReason, useRestoreCheck } from "../../lib/useRestoreCheck";
import { zfsCodeSentence, zfsFixKey, zfsMemberKey } from "../../lib/zfsCodes";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { ToggleRow } from "../../pages/settings/shared";
import { KIND_LABEL } from "../backups/kinds";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { CheckDraw } from "../CheckDraw";
import { usePlaceLabel, type TimelineState } from "../entry/BackupPlaces";
import { CompareWindow } from "../entry/CompareWindow";
import { FlashPluginList } from "../FlashPluginList";
import { FolderBrowser } from "../FolderBrowser";
import { IconCompare } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconCheck, IconCopy, IconDownload, IconRecovery, IconRestore, IconTrash } from "../navGlyphs";
import { RadioMark } from "../RadioMark";
import { Selector } from "../Selector";
import { SelectField } from "../SelectField";
import { SnapshotFileTree } from "../SnapshotFileTree";
import { Rows, SettingRow } from "../storage/rows";
import { TimelineDeleteDialog } from "../timeline/TimelineDeleteDialog";
import { ZFSPropertyList } from "../zfs/ZFSPropertyList";
import { PlaceChecks } from "./PlaceChecks";
import { DatabaseDumpList } from "./DatabaseDumpList";
import { MissingRestorePoint } from "./MissingRestorePoint";
import { RestoreCheckPanel } from "./RestoreCheckPanel";
import { checkRequest, startRestore, type RestoreChoice } from "./restoreJob";
import { RestoreProgress } from "./RestoreProgress";
import { TargetScene } from "./TargetScene";
import { WhatHappens } from "./WhatHappens";

/** What a link or a click asks the window to open on. */
export interface RestoreAsk {
  /** The backup: its row or one of its snapshots. Without one the
   *  recommended backup is taken. */
  snapshot?: string;
  /** When that backup was taken, in Unix seconds. */
  at?: number;
  /** The storage location a click on its mark asked for. */
  place?: string;
  /** The dataset of a ZFS entry the request is about. */
  dataset?: string;
}

/** ZFS: the member states that make writing into the live dataset impossible. */
const NOT_WRITABLE = new Set(["gone", "not-mounted", "not-visible"]);

const WHOLE_TREE = "";

// The server sends nothing until it has packed the whole backup, and the
// browser has no event to wait for, so the button spins for a guessed while.
const DOWNLOAD_PREPARING_MS = 20_000;

const WHAT_LABEL: Record<RestoreWhat, TranslationKey> = {
  all: "restoreWindow.all",
  files: "restore.mode.files",
  dumps: "dbdump.listTitle",
  plugins: "flash.plugins",
};

/** DeleteBackup offers to delete the backup the window stands on: at the
 *  storage location it is taken from and, where others hold it too, at all. */
function DeleteBackup({
  timeline,
  itemKey,
  row,
  mark,
  onDeleted,
}: {
  timeline: TimelineState;
  itemKey: string;
  row: TimelineRow;
  mark: TimelineMark;
  onDeleted: () => void;
}) {
  const { t } = useT();
  const [deleting, setDeleting] = useState<string[] | null>(null);
  const place = timeline.places.find((p) => p.place === mark.place);
  if (!place) return null;
  return (
    <span className="flex flex-wrap gap-2">
      <Button
        label={t("common.delete")}
        labelKey="common.delete"
        glyph={<IconTrash />}
        tone="subtle"
        disabled={place.appendOnly}
        hint={place.appendOnly ? t("placementCode.appendOnly") : undefined}
        onClick={() => setDeleting([mark.place])}
      />
      {row.places.length > 1 && (
        <Button
          label={t("timeline.deleteRow")}
          labelKey="timeline.deleteRow"
          glyph={<IconTrash />}
          tone="subtle"
          onClick={() => setDeleting([])}
        />
      )}
      {deleting && (
        <TimelineDeleteDialog
          key={deleting.join(",")}
          domain={timeline.domain}
          itemKey={itemKey}
          row={row}
          places={deleting}
          onDone={() => {
            const everywhere = deleting.length === 0;
            setDeleting(null);
            onDeleted();
            timeline.reload();
            // The cards count the copies at targets, so they read them again.
            if (everywhere || deleting.some((p) => p !== "local")) placementChanged();
          }}
          onClose={() => setDeleting(null)}
        />
      )}
    </span>
  );
}

/** SelfRestoreNote stands in for the window on the self-backup: the running
 *  instance does not overwrite its own settings. */
function SelfRestoreNote({ deletes, onClose }: { deletes: React.ReactNode; onClose: () => void }) {
  const { t } = useT();
  const navigate = useNavigate();
  return (
    <InstanceWindow
      title={t("restoreWindow.self.title")}
      facts={t("restoreWindow.self.text")}
      onClose={onClose}
      footer={
        <>
          {deletes}
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
          <Button
            label={t("entry.config.startRecovery")}
            labelKey={null}
            glyph={<IconRecovery />}
            tone="accent"
            onClick={() => {
              onClose();
              navigate(emergencyPath("settings"), { replace: true });
            }}
          />
        </>
      }
    >
      {null}
    </InstanceWindow>
  );
}

const ARROW_STEP: Record<string, number> = { ArrowDown: 1, ArrowRight: 1, ArrowUp: -1, ArrowLeft: -1 };

/** WhereOption is one target of a restore, with the scene that plays it. Its
 *  radio group is one tab stop, and the arrow keys pick the next target, as
 *  in any radio group. */
export function WhereOption({
  where,
  on,
  title,
  sub,
  kept,
  disabled,
  onPick,
}: {
  where: RestoreWhere;
  on: boolean;
  title: string;
  sub: string;
  kept: boolean;
  disabled: boolean;
  onPick: () => void;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={on}
      tabIndex={on ? 0 : -1}
      disabled={disabled}
      onClick={onPick}
      onKeyDown={(e) => {
        const step = ARROW_STEP[e.key];
        const group = e.currentTarget.closest('[role="radiogroup"]');
        if (!step || !group) return;
        e.preventDefault();
        // Left and right follow the reading direction.
        const flip = (e.key === "ArrowLeft" || e.key === "ArrowRight") && getComputedStyle(group).direction === "rtl";
        const radios = Array.from(group.querySelectorAll<HTMLButtonElement>('[role="radio"]:not(:disabled)'));
        const next = radios[(radios.indexOf(e.currentTarget) + (flip ? -step : step) + radios.length) % radios.length];
        next.focus();
        next.click();
      }}
      className={`flex items-center gap-3 rounded-control bg-carbon-surface2 p-3 text-start ring-2 ring-inset transition-colors hover:bg-carbon-surface3 ${
        on ? "ring-accent" : "ring-transparent"
      }`}
    >
      <RadioMark on={on} />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="text-sm font-semibold text-carbon-text wrap-anywhere">{title}</span>
        <span className="text-xs text-carbon-textMuted wrap-anywhere">{sub}</span>
      </span>
      <TargetScene where={where} playing={on} kept={kept} />
    </button>
  );
}

/**
 * RestoreWindow restores one entry from one of its backups. It reads nothing
 * about the entry itself: the caller hands it the entry and its timeline.
 * It opens on the item alone, since the rest of the entry comes from the
 * slow list of its kind, and fills in what depends on it once it is there.
 */
export function RestoreWindow({
  item,
  entry,
  name,
  timeline,
  ask = {},
  deletable = false,
  onClose,
}: {
  item: BackupItem;
  /** null while the caller still reads it. */
  entry: Entry | null;
  /** The entry's name as the page shows it. */
  name: string;
  timeline: TimelineState;
  ask?: RestoreAsk;
  /** Also offers to delete the backup, as the list of an entry's backups does. */
  deletable?: boolean;
  onClose: () => void;
}) {
  if (item.kind === "config") {
    const { snapshot } = ask;
    const row = deletable && snapshot ? timeline.rows.find((r) => rowHolds(r, snapshot)) : undefined;
    const mark = row && (row.places.find((m) => m.place === ask.place) ?? autoMark(row, timeline.places) ?? row.places[0]);
    return (
      <SelfRestoreNote
        deletes={
          row && mark && <DeleteBackup timeline={timeline} itemKey={item.key} row={row} mark={mark} onDeleted={onClose} />
        }
        onClose={onClose}
      />
    );
  }
  return (
    <RestoreForm item={item} entry={entry} name={name} timeline={timeline} ask={ask} deletable={deletable} onClose={onClose} />
  );
}

function RestoreForm({
  item,
  entry,
  name,
  timeline,
  ask,
  deletable,
  onClose,
}: {
  item: BackupItem;
  entry: Entry | null;
  name: string;
  timeline: TimelineState;
  ask: RestoreAsk;
  deletable: boolean;
  onClose: () => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const detail = entry?.detail;
  const { kind } = item;
  const { rows, places, loadPlace, reload } = timeline;
  const labelOf = usePlaceLabel(places);
  const { list: openFindings, flagged: flaggedIds } = useOpenAnomalies();
  const container = detail?.kind === "container" ? detail.container : null;
  const zfsItem = detail?.kind === "zfs" ? detail.dataset : null;
  const gone = isGone(item);
  const hasOriginal = detail?.kind === "files" ? detail.set.path !== "" : !gone;
  // Whether a folder set still has its original path, and so which targets
  // it has, comes with the entry. Until then the targets wait and none is
  // picked.
  const targetsReading = kind === "files" && entry === null;

  const findings = useMemo(() => findingsOf(openFindings, kind, item.key), [openFindings, kind, item.key]);
  const recommendation = useMemo(() => recommend(rows, findings), [rows, findings]);
  const asked = ask.snapshot ? requestedRow(rows, ask.snapshot, ask.at ?? 0) : undefined;

  const [rowKey, setRowKey] = useState<string | null>(null);
  const [placePick, setPlacePick] = useState(ask.place);
  const [whatPick, setWhat] = useState<RestoreWhat>("all");
  const [wherePick, setWherePick] = useState<RestoreWhere | null>(null);
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [filter, setFilter] = useState("");
  const [folderPick, setFolder] = useState<string | null>(null);
  const defaultFolder = entry ? restoreFolderFor(entry.settings, kind) : "";
  const folder = folderPick ?? defaultFolder;
  const [leaveStopped, setLeaveStopped] = useState(false);
  const [withoutRuntime, setWithoutRuntime] = useState(false);
  const [backupFirst, setBackupFirst] = useState(true);
  const [preparing, setPreparing] = useState(false);
  const [comparing, setComparing] = useState(false);

  const [datasetPick, setDatasetPick] = useState<string | null>(null);
  const [newDataset, setNewDataset] = useState("");
  const [applyProps, setApplyProps] = useState(false);
  const [safety, setSafety] = useState(true);
  const [stopContainers, setStopContainers] = useState(true);

  const row = rows.find((r) => r.key === rowKey) ?? asked?.row ?? rows[recommendation.index];
  const mark = row && (row.places.find((m) => m.place === placePick) ?? autoMark(row, places) ?? row.places[0]);
  const source = mark ? sourceOfPlace(mark.place) : "local";
  const snapshotId = mark ? newestId(mark) : "";

  // null until the first answer, so the window can tell a tree without
  // restore points from one whose points are still on the way.
  const [points, setPoints] = useState<ZFSRestorePoint[] | null>(null);
  const [host, setHost] = useState<ReadonlyMap<string, ZFSHostDataset>>(new Map());
  // A ZFS entry's key is the id of its item, so the points need not wait for the entry.
  const zfsId = kind === "zfs" ? item.key : undefined;
  useEffect(() => {
    if (!zfsId) return;
    let current = true;
    zfsRestorePoints(zfsId, source)
      .then((res) => {
        if (current) setPoints(res.ok ? (res.points ?? []) : []);
      })
      .catch(() => {
        if (current) setPoints([]);
      });
    return () => {
      current = false;
    };
  }, [zfsId, source]);
  // The listing goes over SSH. Without it the server still checks whether a
  // dataset can be written into.
  useEffect(() => {
    if (!zfsId) return;
    zfsHostDatasets()
      .then((res) => {
        if (res.ok && res.available) setHost(new Map(res.datasets.map((d) => [d.dataset, d])));
      })
      .catch(() => undefined);
  }, [zfsId]);

  // Until the dataset and its points are read, the window stands on the
  // targets a tree usually has and locks them.
  const zfsReading = kind === "zfs" && (zfsItem === null || points === null);
  const point = points?.find((p) => p.stamp === row?.key);
  const held = (point?.members ?? []).filter((m) => m.snapshotId !== "");
  const dataset =
    datasetPick ??
    (
      held.find((m) => m.dataset === ask.dataset) ??
      held.find((m) => m.dataset === zfsItem?.dataset) ??
      held[0]
    )?.dataset ??
    zfsItem?.dataset ??
    "";
  const wholeTree = zfsItem !== null && dataset === WHOLE_TREE;
  const member = point?.members.find((m) => m.dataset === dataset);
  const live = zfsItem?.members.find((m) => m.dataset === dataset);
  const onHost = host.get(dataset);
  const blocked = ((): "zfs.restore.missingDataset" | "zfs.code.read-only-mount" | null => {
    if (!zfsItem || wholeTree) return null;
    if (zfsItem.lastCheckCode === "not-found") return "zfs.restore.missingDataset";
    if (live === undefined || NOT_WRITABLE.has(live.outcome)) return "zfs.restore.missingDataset";
    if (onHost && onHost.visible && !onHost.writable) return "zfs.code.read-only-mount";
    return null;
  })();
  const storedProps = member?.properties ?? {};
  const hasProps = Object.keys(storedProps).length > 0;
  const mountpoint = live?.hostMountpoint ?? zfsItem?.hostMountpoint ?? "";
  // A finding about one dataset marks that dataset's backup, not the run of the tree.
  const flagged =
    zfsItem && !wholeTree
      ? member !== undefined && flaggedIds.has(member.snapshotId)
      : row !== undefined && snapshotIds(row).some((id) => flaggedIds.has(id));

  const facts = {
    hasOriginal,
    writable: kind === "zfs" && !wholeTree && (zfsReading || blocked === null),
    oneDataset: kind === "zfs" && !wholeTree && (zfsReading || (member?.snapshotId ?? "") !== ""),
  };
  // The item carries what the window asks of a database as well, so the
  // container is not waited for.
  const database = (container?.dbTier ?? item.dbTier ?? "") !== "";
  const coverage = container?.dbDataCoverage ?? item.dbDataCoverage;
  const whats = whatsFor(kind, { database }).filter(
    (w) => w !== "files" || wheresFor(kind, "files", facts).length > 0
  );
  // A dataset that cannot take files leaves only the whole restore, whatever was picked before.
  const what = whats.includes(whatPick) ? whatPick : "all";
  const wheres = wheresFor(kind, what, facts);
  const where =
    wherePick !== null && wheres.includes(wherePick) ? wherePick : targetsReading ? undefined : defaultWhere(wheres, hasOriginal);
  const replaces = where === "replace" && hasOriginal;
  const replaceBacksUp = backsUpFirst(kind, what, "replace", hasOriginal);
  const canBackUpFirst = where === "replace" && replaceBacksUp;
  const fileSnapshot = zfsItem ? (member?.snapshotId ?? "") : snapshotId;

  const [files, setFiles] = useState<FileEntry[]>([]);
  const [filesLoading, setFilesLoading] = useState(false);
  const [filesError, setFilesError] = useState<string | null>(null);
  useEffect(() => {
    if (what !== "files" || fileSnapshot === "") return;
    let current = true;
    // Paths ticked in another backup mean nothing in this one.
    setPicked(new Set());
    setFilesLoading(true);
    setFilesError(null);
    const listing =
      kind === "container"
        ? listSnapshotFiles(item.key, fileSnapshot, source)
        : kind === "files"
          ? listSnapshotFilesFileSet(item.key, fileSnapshot, source)
          : listSnapshotFilesZFS(item.key, fileSnapshot, source);
    listing
      .then((res) => {
        if (!current) return;
        setFiles(res.ok ? (res.files ?? []) : []);
        if (!res.ok) setFilesError(loadErrorMessage(res, t("files.loadFailed")));
      })
      .catch(() => {
        if (current) setFilesError(t("files.loadFailed"));
      })
      .finally(() => {
        if (current) setFilesLoading(false);
      });
    return () => {
      current = false;
    };
  }, [what, kind, item.key, fileSnapshot, source, t]);

  const [dumps, setDumps] = useState<DBDumpView[]>([]);
  const paired = row !== undefined && dumps.some((dump) => pairsWith(dump, { id: snapshotId, original: row.key }));
  useEffect(() => {
    if (!database) return;
    listDbDumps(item.key, source)
      .then((res) => setDumps(res.ok ? (res.dumps ?? []) : []))
      .catch(() => setDumps([]));
  }, [database, item.key, source]);

  const choice: RestoreChoice | null =
    row && mark && where && !zfsReading
      ? {
          kind,
          id: item.key,
          snapshotId,
          source,
          what,
          where,
          paths: [...picked],
          folder: folder.trim(),
          leaveStopped,
          withoutRuntime,
          backupFirst: canBackUpFirst && backupFirst,
          zfs: zfsItem
            ? {
                stamp: point?.stamp ?? "",
                dataset,
                newDataset: newDataset.trim(),
                applyProperties: hasProps && applyProps,
                safetySnapshot: safety,
                safetyOffConfirm: !safety,
                stopContainers: zfsItem.stopContainers.length > 0 && stopContainers,
              }
            : undefined,
        }
      : null;
  const check = useRestoreCheck(choice ? checkRequest(choice) : null);
  // While the tree is read, the check stands in its place wherever one will
  // run, so the window keeps its height.
  const checkComing =
    zfsReading &&
    what === "all" &&
    (where === "newDataset" ? newDataset.trim() !== "" : where !== "folder" || folder.trim() !== "");
  const plan = check.state.phase === "done" ? check.state.result.plan : null;

  const progressKey = itemProgressKey(item);
  const runTarget = ENTRY_KIND[kind].keyedBy === "name" ? item.key : item.name;
  const cancelledRef = useRef(false);
  const retryWithoutRuntime = useRef(false);
  const [ack, setAck] = useState<{ target: string; created?: string; safetySnapshot?: string } | null>(null);
  const [refusalFix, setRefusalFix] = useState<TranslationKey | null>(null);
  // What the restore that runs was asked for, kept apart from the form. The
  // ref is what the start reads, the state what the window draws.
  const fired = useRef<RestoreChoice | null>(null);
  const [ran, setRan] = useState<RestoreChoice | null>(null);
  const { state, fire, reset, isPending } = useBackupWatch({
    progressKey,
    kind: "restore",
    matchRun: (r) => r.domain === kind && r.target === runTarget,
    cancelledRef,
    onDone: reload,
    start: async () => {
      const run = fired.current;
      if (!run) return { ok: false };
      setAck(null);
      setRefusalFix(null);
      const res = await startRestore({ ...run, withoutRuntime: run.withoutRuntime || retryWithoutRuntime.current });
      if (res.code === SNAPSHOT_MISSING) void missing();
      const refusal = secretsSentence(t, res.code);
      if (res.ok) setAck({ target: res.target ?? "", created: res.created, safetySnapshot: res.safetySnapshot });
      else if (refusal) return { ...res, error: refusal };
      else if (kind === "zfs" && res.code) {
        setRefusalFix(zfsFixKey(res.code));
        // What the server said beyond the dataset the restore was about,
        // such as zfs's own message, follows the sentence of the code.
        const said = (res.error ?? "").startsWith(`${res.code}: `) ? (res.error ?? "").slice(res.code.length + 2) : "";
        const sentence = zfsCodeSentence(t, res.code, { hostMountpoint: mountpoint });
        const named = said === "" || said === run.zfs?.dataset || said === run.zfs?.newDataset;
        return { ...res, error: named ? sentence : `${sentence} ${isolateLtr(said)}` };
      }
      return res;
    },
  });
  const progress = useProgress();
  const prog = progress[progressKey];
  const running = anyActive(progress);
  const own = isPending || (prog?.active === true && prog.phase === "restore");
  const blockedByOther = running.active && !own;
  const shown = own || state.phase !== "idle";

  // The place answered that the backup is gone from it. It is read again, and
  // the next place that holds the backup is offered.
  async function missing() {
    if (!row || !mark) return;
    const lost = mark.place;
    const next = autoMark({ ...row, places: row.places.filter((m) => m.place !== lost) }, places);
    await loadPlace(lost);
    if (next === null) {
      onClose();
      return;
    }
    const yes = await confirm(
      t("timeline.snapshotMissing")
        .replace("{place}", () => labelOf(lost))
        .replace("{next}", () => labelOf(next.place))
    );
    if (yes) {
      reset();
      setPlacePick(next.place);
    } else onClose();
  }

  async function handleSafety(next: boolean) {
    if (!next && !(await confirm(t("zfs.restore.safetyOffConfirm")))) return;
    setSafety(next);
  }

  function download() {
    setPreparing(true);
    setTimeout(() => setPreparing(false), DOWNLOAD_PREPARING_MS);
    const a = document.createElement("a");
    a.href = flashDownloadURL(snapshotId, source);
    a.download = `flash-${snapshotId.slice(0, 8)}.zip`;
    document.body.appendChild(a);
    a.click();
    a.remove();
  }

  const copyName = plan?.beside?.name ?? "";
  const says = sayReplace();
  function sayReplace(): string {
    if (kind === "zfs") return t("zfs.restore.confirm").replace("{path}", mountpoint);
    if (what === "files") return t("files.restoreConfirm");
    if (kind === "container") return t("restoreWindow.says.replaceContainer");
    if (kind === "vm") return t("restoreWindow.says.replaceVM");
    return t("restoreWindow.says.replaceFiles");
  }

  async function begin() {
    if (!choice || !where) return;
    if (where === "zip") {
      download();
      return;
    }
    if (where === "newDataset") {
      const question = t("zfs.restore.newDatasetConfirm").replace("{name}", () => newDataset.trim());
      if (!(await confirm(question, { confirmKey: "snapshots.restore" }))) return;
    }
    if (replaces) {
      const typed = await confirm(says, {
        requireText: name,
        requirePrompt: t("restoreWindow.confirmType").replace("{name}", () => name),
        confirmLabel: t("restoreWindow.replaceNamed").replace("{name}", () => name),
        confirmLabelKey: "snapshots.restore",
      });
      if (!typed) return;
    }
    fired.current = choice;
    setRan(choice);
    retryWithoutRuntime.current = false;
    void fire();
  }

  function again() {
    retryWithoutRuntime.current = true;
    void fire();
  }

  async function copySnapshot(snapshot: string) {
    if (await copyText(snapshot)) push(t("common.copied"), "success");
    else push(t("vm.ssh.copyFailed"), "fail");
  }

  const when = (r: TimelineRow) => formatRecent(rowTime(r), lang);
  // An option starts a line, so it starts with a capital like the rail does.
  const optionWhen = (r: TimelineRow) => {
    const text = when(r);
    return text.charAt(0).toLocaleUpperCase(lang) + text.slice(1);
  };
  const recommended = rows[recommendation.index];
  const why = recommendationWhy(recommendation, t, lang);
  const title = t("entry.restore.title").replace("{name}", () => name);
  const engine = container?.dbEngine ? ENGINE_NAMES[container.dbEngine] : "";
  const subtitle = [t(KIND_LABEL[kind]), engine].filter(Boolean).join(" · ");

  const doneText = !ran
    ? ""
    : ran.where === "beside"
      ? t("restoreWindow.doneBeside").replace("{name}", () => ack?.target || copyName || name)
      : ran.where === "folder"
        ? t("restoreWindow.doneFolder").replace("{path}", () => ack?.target || ran.folder)
        : t("restoreWindow.doneReplace").replace("{name}", () => name);

  if (shown) {
    const finished = !own;
    return (
      <InstanceWindow
        title={title}
        facts={t(
          own
            ? "restore.started"
            : state.phase === "success"
              ? "common.done"
              : state.phase === "cancelled"
                ? "run.statusCancelled"
                : "restoreWindow.notDone"
        )}
        onClose={onClose}
        footer={
          <>
            {finished && state.phase !== "success" && (
              <Button label={t("common.back")} labelKey="common.back" tone="subtle" onClick={reset} />
            )}
            <Button
              label={t(finished ? "common.done" : "common.close")}
              labelKey={finished ? "common.done" : "common.close"}
              glyph={finished ? <IconCheck /> : undefined}
              tone="accent"
              onClick={onClose}
            />
          </>
        }
      >
        <div className="flex flex-col gap-2">
          {ack && kind === "zfs" && (
            <div className="flex flex-col gap-1 text-xs text-carbon-textSub">
              {ack.created && <p>{t("zfs.restore.created").replace("{name}", ack.created)}</p>}
              <p>{t("zfs.restore.started").replace("{target}", ran?.where === "replace" ? mountpoint : ack.target)}</p>
              {ack.safetySnapshot && (
                <p className="flex items-center gap-2">
                  <span dir="ltr" className="font-mono text-start">
                    {t("zfs.restore.safetyTaken").replace("{snapshot}", ack.safetySnapshot)}
                  </span>
                  <Button
                    label={t("common.copy")}
                    labelKey="common.copy"
                    glyph={<IconCopy />}
                    onClick={() => void copySnapshot(ack.safetySnapshot ?? "")}
                  />
                </p>
              )}
            </div>
          )}
          <RestoreProgress
            state={state}
            isPending={own}
            prog={prog}
            cancelKey={progressKey}
            inPlace={ran ? ran.where === "replace" : true}
            name={name}
            cancelledRef={cancelledRef}
            showStartedHint={false}
            successMessage={
              <span className="inline-flex items-center gap-1.5 text-sm" data-testid="restore-done">
                <CheckDraw />
                {doneText}
              </span>
            }
            onRestoreWithoutRuntime={kind === "container" && ran?.where !== "folder" ? again : undefined}
            t={t}
          />
          {own && <p className="text-xs text-carbon-textMuted">{t("restoreWindow.background")}</p>}
          {state.phase === "success" && kind === "container" && ran?.where === "beside" && (
            <p className="text-xs text-carbon-textMuted">{t("restoreWindow.copyListed")}</p>
          )}
          {state.phase === "error" && refusalFix && <p className="text-xs text-carbon-textSub">{t(refusalFix)}</p>}
        </div>
        {confirmDialog}
      </InstanceWindow>
    );
  }

  const listed = what === "dumps" || what === "plugins";
  // A paired phone's session is refused the download.
  const downloadRefused = where === "zip" && openedAsPairedPhone();
  const blockReason = blockedByOther
    ? t(busyPhraseKey(running.phase, running.stage))
    : downloadRefused
      ? t("restoreMany.flashOwnLogin")
    : what === "files" && picked.size === 0
      ? t("files.selectHint")
      : where === "folder" && folder.trim() === ""
        ? t("restoreWindow.needsFolder")
        : restoreBlockReason(check, t);
  const primary =
    where === "beside"
      ? t("restoreWindow.besideNamed")
      : where === "zip"
        ? t("restoreWindow.zip")
        : where === "folder"
          ? t("restoreWindow.toFolderNamed")
          : where === "newDataset"
            ? t("zfs.restore.newDataset")
            : replaces
              ? t("restoreWindow.replaceNamed")
              : t("entry.restore.title");

  const optionText = (w: RestoreWhere): [string, string] => {
    switch (w) {
      case "beside":
        return [
          t("restoreWindow.beside"),
          copyName ? t("restoreWindow.besideSub").replace("{name}", () => copyName) : t("restoreWindow.untouched"),
        ];
      case "replace":
        if (kind === "zfs") return [t("zfs.restore.inPlace"), t("restoreWindow.zfsInPlaceSub")];
        if (!hasOriginal) return [t("restore.mode.inPlace"), t("restoreCheck.def.missing")];
        return [
          t("restoreWindow.replace"),
          t(replaceBacksUp && backupFirst ? "restoreWindow.replaceSubBackup" : "restoreWindow.replaceSub"),
        ];
      case "folder":
        return [t("restore.mode.toFolder"), t("restoreWindow.folderSub")];
      case "newDataset":
        return [t("zfs.restore.newDataset"), t("restoreWindow.untouched")];
      default:
        return [t("restoreWindow.zip"), t("restoreWindow.zipSub")];
    }
  };

  const marks = row ? [...row.places].sort((a, b) => places.findIndex((p) => p.place === a.place) - places.findIndex((p) => p.place === b.place)) : [];
  const datasetOptions = [
    { value: WHOLE_TREE, label: t("zfs.restore.wholeTree") },
    ...(point?.members ?? []).map((m) => {
      const key = zfsMemberKey(m.outcome);
      const outcome = key ? t(key) : zfsCodeSentence(t, m.outcome, { hostMountpoint: host.get(m.dataset)?.hostMountpoint });
      return { value: m.dataset, label: `${m.dataset} (${outcome})`, disabled: m.snapshotId === "" };
    }),
  ];

  return (
    <InstanceWindow
      title={title}
      facts={subtitle}
      onClose={onClose}
      footer={
        <>
          {deletable && row && mark && (
            <DeleteBackup timeline={timeline} itemKey={item.key} row={row} mark={mark} onDeleted={onClose} />
          )}
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
          {!listed && (
            <Button
              label={primary.replace("{name}", () => name)}
              labelKey={where === "zip" ? "flash.download" : "snapshots.restore"}
              glyph={where === "zip" ? <IconDownload /> : <IconRestore />}
              tone="accent"
              onClick={() => void begin()}
              disabled={!choice || blockedByOther || downloadRefused || !check.ready || preparing}
              busy={preparing}
              hint={blockReason}
            />
          )}
        </>
      }
    >
      {timeline.loading && <p className="text-sm text-carbon-textMuted">{t("common.loadingBackups")}</p>}
      {timeline.error !== null && <p className="text-sm text-statusFail">{timeline.error || t("common.loadBackupsFailed")}</p>}
      {!timeline.loading && timeline.error === null && !row && (
        <>
          <p className="text-sm text-carbon-textMuted">{t("restoreWindow.noBackup")}</p>
          <PlaceChecks timeline={timeline} />
        </>
      )}

      {row && mark && (
        <>
          {ask.snapshot && asked?.missing && places.every((p) => p.state === "read") && (
            <MissingRestorePoint
              requested={ask.snapshot}
              requestedAt={ask.at ?? 0}
              points={rows.map((r) => ({ id: r.key, at: rowTime(r) }))}
              t={t}
            />
          )}
          <Rows>
            <SettingRow
              label={t("restoreWindow.backup")}
              hint={t("restoreWindow.backupHint")}
              note={row === recommended ? why : undefined}
            >
              <SelectField
                value={row.key}
                onChange={setRowKey}
                label={t("restoreWindow.backup")}
                options={rows.map((r) => ({
                  value: r.key,
                  label:
                    r === recommended ? t("restoreWindow.recommendedOption").replace("{when}", () => optionWhen(r)) : optionWhen(r),
                }))}
                className="max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus"
              />
              {comparable(rows, actionMark(row, places).place).length > 1 && (
                <Button
                  label={t("snapshot.compare")}
                  labelKey="snapshot.compare"
                  glyph={<IconCompare />}
                  tone="subtle"
                  onClick={() => setComparing(true)}
                />
              )}
            </SettingRow>
            {marks.length > 1 && (
              <SettingRow label={t("restoreWindow.source")} hint={t("restoreWindow.sourceHint")}>
                <Selector
                  items={marks.map((m) => ({
                    id: m.place,
                    label: m.incomplete ? `${labelOf(m.place)} · ${t("timeline.incomplete")}` : labelOf(m.place),
                    tip: m.incomplete ? t("timeline.incompleteHint") : undefined,
                  }))}
                  label={t("restoreWindow.source")}
                  select="one"
                  active={mark.place}
                  onChange={setPlacePick}
                  inline
                />
              </SettingRow>
            )}
            {kind === "zfs" && (
              <SettingRow label={t("zfs.restore.dataset")}>
                <SelectField
                  value={dataset}
                  onChange={setDatasetPick}
                  label={t("zfs.restore.dataset")}
                  options={zfsReading ? [{ value: dataset, label: t("folder.loading") }] : datasetOptions}
                  disabled={zfsReading}
                  className="max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus"
                />
              </SettingRow>
            )}
            {whats.length > 1 && (
              <SettingRow label={t("restoreWindow.what")}>
                <Selector
                  items={whats.map((w) => ({ id: w, label: t(WHAT_LABEL[w]) }))}
                  label={t("restoreWindow.what")}
                  select="one"
                  active={what}
                  onChange={(id) => setWhat(id as RestoreWhat)}
                  inline
                />
              </SettingRow>
            )}
          </Rows>

          {row !== recommended && recommended && (
            <div className="flex flex-wrap items-center gap-3 rounded-card bg-carbon-surface2 px-4 py-3 text-sm text-carbon-text">
              <span className="min-w-0 flex-1">
                {t("restoreWindow.recommendedIs").replace("{when}", () => when(recommended))} {why}
              </span>
              <Button
                label={t("restoreWindow.useThis")}
                labelKey={null}
                glyph={<IconCheck />}
                onClick={() => setRowKey(recommended.key)}
              />
            </div>
          )}
          {(flagged || paired) && (
            <div className="flex flex-wrap items-center gap-2">
              {flagged && (
                <Badge tone="warn" size="small">
                  {t("anomaly.snapshotFlagged")}
                </Badge>
              )}
              {paired && (
                <span className="flex items-center gap-1">
                  <Badge tone="neutral" size="small">
                    {t("dbdump.pairedBadge")}
                  </Badge>
                  <InfoBubble tip={t("dbdump.pairedTip")} />
                </span>
              )}
            </div>
          )}
          <PlaceChecks timeline={timeline} />

          {what === "files" && (
            <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3 text-xs">
              <SnapshotFileTree
                files={files}
                loading={filesLoading}
                error={filesError}
                filter={filter}
                onFilterChange={setFilter}
                selected={picked}
                onToggle={(path) => {
                  const next = new Set(picked);
                  if (next.has(path)) next.delete(path);
                  else next.add(path);
                  setPicked(next);
                }}
                t={t}
              />
            </div>
          )}
          {what === "dumps" && entry && detail?.kind === "container" && (
            <DatabaseDumpList
              containerName={detail.container.name}
              source={source}
              recognised
              canImport={detail.container.installed && detail.container.state === "running"}
              importStops={(detail.container.stopContainers ?? []).filter((dep) =>
                detail.installed.some((c) => c.name === dep && c.state === "running")
              )}
              hostMountRoot={entry.hostMountRoot}
              defaultFolder={defaultFolder}
              reloadTick={0}
              t={t}
            />
          )}
          {what === "plugins" && <FlashPluginList snapshotId={snapshotId} source={source} t={t} />}

          {!listed && (where || targetsReading) && (
            <>
              <div data-new="restore-where" className="flex flex-col gap-2">
                <h3 className="text-sm font-semibold text-carbon-textSub">{t("restoreWindow.where")}</h3>
                <div role="radiogroup" aria-label={t("restoreWindow.where")} className="flex flex-col gap-2">
                  {wheres.map((w) => {
                    const [optionTitle, sub] = optionText(w);
                    return (
                      <WhereOption
                        key={w}
                        where={w}
                        on={w === where}
                        title={optionTitle}
                        sub={sub}
                        kept={replaceBacksUp && backupFirst}
                        disabled={zfsReading || targetsReading}
                        onPick={() => {
                          if (w === "newDataset" && newDataset === "") setNewDataset(`${dataset}-restored`);
                          setWherePick(w);
                        }}
                      />
                    );
                  })}
                </div>
                {blocked !== null && <p className="text-xs text-statusWarn">{tLtr(t, blocked)}</p>}
              </div>

              {where === "folder" && entry && (
                <FolderBrowser label={t("restore.targetPath")} value={folder} hostMountRoot={entry.hostMountRoot} onChange={setFolder} />
              )}
              {where === "newDataset" && (
                <div className="flex flex-col gap-1">
                  <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                    {t("zfs.restore.newDatasetName")}
                    <InfoBubble tip={t("zfs.restore.newDatasetHint")} />
                  </span>
                  <input
                    type="text"
                    dir="ltr"
                    aria-label={t("zfs.restore.newDatasetName")}
                    value={newDataset}
                    onChange={(e) => setNewDataset(e.target.value)}
                    spellCheck={false}
                    className="w-full max-w-md rounded-control bg-carbon-surface2 px-3 py-1.5 text-start font-mono text-xs text-carbon-text glim-field-focus"
                  />
                </div>
              )}
              {where === "newDataset" && (hasProps || zfsReading) && (
                <ZFSPropertyList properties={storedProps} into="new" reading={zfsReading} t={t} />
              )}
              {kind === "zfs" && where === "replace" && (hasProps || zfsReading) && (
                <ZFSPropertyList properties={storedProps} into="existing" reading={zfsReading} t={t} />
              )}

              <div className="-my-3 flex flex-col [&>*]:border-t [&>*]:border-carbon-border/60 [&>*]:py-3">
                {canBackUpFirst && (
                  <ToggleRow
                    label={t("restoreWindow.backupFirst")}
                    hint={t("restoreWindow.backupFirstHint")}
                    checked={backupFirst}
                    onChange={setBackupFirst}
                  />
                )}
                {(kind === "container" || kind === "vm") && where === "replace" && (
                  <ToggleRow
                    label={t("restoreWindow.leaveStopped")}
                    hint={t(kind === "vm" ? "restoreWindow.leaveStoppedVM" : "restoreWindow.leaveStoppedContainer")}
                    checked={leaveStopped}
                    onChange={setLeaveStopped}
                  />
                )}
                {kind === "container" && what === "all" && where !== "folder" && (
                  <ToggleRow
                    label={t("restore.withoutRuntime")}
                    hint={t("restoreWindow.withoutRuntimeHint")}
                    checked={withoutRuntime}
                    onChange={setWithoutRuntime}
                  />
                )}
                {kind === "zfs" && where === "replace" && (
                  <>
                    {hasProps && (
                      <ToggleRow
                        label={t("zfs.restore.applyProperties")}
                        hint={t("zfs.restore.applyPropertiesHint")}
                        checked={applyProps}
                        onChange={setApplyProps}
                      />
                    )}
                    <ToggleRow
                      label={t("zfs.restore.safetySnapshot")}
                      hint={t("zfs.restore.safetySnapshotHint")}
                      checked={safety}
                      onChange={(next) => void handleSafety(next)}
                    />
                    {zfsItem && zfsItem.stopContainers.length > 0 && (
                      <ToggleRow
                        label={t("zfs.restore.stopContainers").replace("{names}", zfsItem.stopContainers.join(", "))}
                        hint={t("zfs.restore.stopContainersHint")}
                        checked={stopContainers}
                        onChange={setStopContainers}
                      />
                    )}
                  </>
                )}
              </div>

              {kind === "container" && what === "all" && where !== "folder" && (coverage === "live" || coverage === "none") && (
                <p className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs leading-relaxed text-statusWarn">
                  {t(coverage === "live" ? "restoreWindow.dbLive" : "restoreWindow.dbNone")}
                </p>
              )}

              <RestoreCheckPanel check={checkComing ? { ...check, state: { phase: "running" } } : check} t={t} />
              {where && (
                <WhatHappens
                  kind={kind}
                  what={what}
                  where={where}
                  stopped={leaveStopped}
                  backupFirst={canBackUpFirst && backupFirst}
                  plan={plan}
                />
              )}
            </>
          )}
        </>
      )}

      {comparing && row && entry && (
        <CompareWindow entry={entry} name={name} row={row} timeline={timeline} onClose={() => setComparing(false)} />
      )}
      {confirmDialog}
    </InstanceWindow>
  );
}
