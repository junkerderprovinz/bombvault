// What can be done with one entry as a whole: the two actions the page is
// there for, in its corner, and the rare ones beside its name.

import { useEffect, useRef, useState, type ReactNode } from "react";
import {
  deleteBackups,
  deleteBackupsVM,
  deleteFileSet,
  deleteFileSetBackups,
  deleteZFSDataset,
  forgetContainer,
  forgetVM,
  type OkEnvelope,
  type Run,
} from "../../lib/api";
import { ENTRY_KIND } from "../../lib/backupEntry";
import { inSchedule, isGone, itemProgressKey } from "../../lib/backupList";
import { useBackupWatch } from "../../lib/backupWatch";
import type { Entry } from "../../lib/entry";
import { humanBytes } from "../../lib/forecast";
import { useT, type TranslationKey } from "../../lib/i18n";
import { anyActive, busyPhraseKey, useProgress } from "../../lib/progress";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useIsDesktop } from "../../lib/useMediaQuery";
import { containerTakeover, vmTakeover } from "../../lib/useTakeOver";
import { AnomalyWindow } from "../anomalies/AnomalyWindow";
import { STOP_ACK_KEY } from "../BackupButton";
import { Button } from "../Button";
import { ExportButton } from "../containers/ExportButton";
import { LinkEntryPicker } from "../LinkEntryPicker";
import { RunDetailSheet } from "../mobile/RunDetailSheet";
import { IconBackupNow, IconDownload, IconRestore, IconTrash, IconZFS } from "../navGlyphs";
import { OrphanRemoveButton } from "../OrphanRemoveButton";
import { PageAction } from "../PageActions";
import { PauseButton } from "../SchedulePause";
import { VMExportButton } from "../vms/VMExportButton";
import { ZFSConnectionCard } from "../zfs/ZFSConnectionCard";
import { DeleteSafetyToggle } from "../zfs/ZFSDatasetRow";

function stopAcknowledged(): boolean {
  try {
    return localStorage.getItem(STOP_ACK_KEY) === "1";
  } catch {
    return false;
  }
}

/**
 * BackupAction starts a backup of the entry and follows it to its end. The
 * server runs the backup by itself and answers at once, so the outcome is
 * read from the progress stream and the recorded run. On a phone the run
 * opens in a sheet as soon as it is known.
 */
function BackupAction({ entry, onDone }: { entry: Entry; onDone: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const isDesktop = useIsDesktop();
  const { item, detail } = entry;
  const spec = ENTRY_KIND[item.kind];
  const progressMap = useProgress();
  const pkey = itemProgressKey(item);
  const progress = progressMap[pkey];
  const running = anyActive(progressMap);
  const target = spec.keyedBy === "name" ? item.key : item.name;

  const [sheetRun, setSheetRun] = useState<Run | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);
  // A sheet closed by hand stays closed for that run. The next run opens it again.
  const dismissed = useRef(false);
  const lastRun = useRef("");

  const { state, fire, isPending } = useBackupWatch({
    progressKey: pkey,
    start: () => spec.backup(item.key),
    matchRun: (run) => run.domain === item.kind && (spec.keyedBy === "kind" || run.target === target),
    onDone,
    onRun: (run) => {
      if (isDesktop) return;
      if (lastRun.current !== run.id) {
        lastRun.current = run.id;
        dismissed.current = false;
      }
      setSheetRun(run);
      if (!dismissed.current) setSheetOpen(true);
    },
  });
  const [shake, setShake] = useState(0);
  const seenPhase = useRef(state.phase);

  useEffect(() => {
    if (state.phase === seenPhase.current) return;
    seenPhase.current = state.phase;
    if (state.phase === "success") {
      const id = state.snapshotId ? ` · ${state.snapshotId.slice(0, 8)}` : "";
      // A container without data folders has only its definition saved.
      push(item.kind === "container" && !state.snapshotId ? t("backup.configOnly") : `${t("common.done")}${id}`, "success");
    } else if (state.phase === "error") {
      push(state.message, "fail");
      setShake((n) => n + 1);
    } else if (state.phase === "skipped") {
      push(t("containers.notInstalledTitle"), "warn");
    }
  }, [state, push, t, item.kind]);

  async function start() {
    // A container is stopped for the whole run. The warning is about how
    // backups work, so one answer per browser is enough.
    if (item.kind === "container" && !stopAcknowledged()) {
      if (!(await confirm(t("containers.stopWarning"), { confirmKey: "containers.backupNow" }))) return;
      try {
        localStorage.setItem(STOP_ACK_KEY, "1");
      } catch {
        // Without storage the warning comes again next time.
      }
    }
    await fire();
  }

  const noFolder = detail.kind === "files" && detail.set.path === "";
  const blockedByOther = running.active && !isPending;
  let title: string | undefined;
  if (noFolder) title = t("files.noPathHint");
  else if (isPending) {
    title =
      progress?.stage === "dbdump"
        ? t("dbdump.progress").replace("{bytes}", humanBytes(progress.bytes ?? 0))
        : t("common.backingUp");
  } else if (blockedByOther) title = t(busyPhraseKey(running.phase, running.stage));

  return (
    <>
      {confirmDialog}
      <PageAction
        key={shake}
        primary
        label={t("containers.backupNow")}
        glyph={<IconBackupNow />}
        onClick={() => void start()}
        disabled={isPending || blockedByOther || noFolder}
        busy={isPending}
        title={title}
        shake={shake > 0}
      />
      {sheetRun && (
        <RunDetailSheet
          run={sheetRun}
          open={sheetOpen}
          onClose={() => {
            dismissed.current = true;
            setSheetOpen(false);
          }}
        />
      )}
    </>
  );
}

/**
 * EntryCorner is the content of the page's action corner. An entry that is
 * gone has nothing to back up and offers its newest backup instead.
 */
export function EntryCorner({
  entry,
  onChanged,
  onRestoreNewest,
}: {
  entry: Entry;
  onChanged: () => void;
  /** Opens the newest backup, absent while there is none to open. */
  onRestoreNewest?: () => void;
}) {
  const { t } = useT();
  const { item } = entry;
  const setIncluded = ENTRY_KIND[item.kind].setIncluded;

  if (isGone(item)) {
    if (!onRestoreNewest) return null;
    return <PageAction primary label={t("snapshots.restore")} glyph={<IconRestore />} onClick={onRestoreNewest} />;
  }
  return (
    <>
      {setIncluded && (
        <PauseButton
          corner
          paused={!inSchedule(item)}
          save={(include) => setIncluded(item.key, include)}
          onSaved={onChanged}
        />
      )}
      <BackupAction entry={entry} onDone={onChanged} />
    </>
  );
}

/** useRemoval asks, runs one removing call and reports a refusal as a toast. */
function useRemoval() {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [busy, setBusy] = useState(false);

  async function remove(
    question: string,
    confirmKey: TranslationKey,
    call: () => Promise<OkEnvelope>,
    done: () => void,
    extra?: ReactNode
  ) {
    if (!(await confirm(question, { confirmKey, extra }))) return;
    setBusy(true);
    try {
      const res = await call();
      if (res.ok) done();
      else push(res.error ?? t("common.removeFailed"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.removeFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }

  return { remove, busy, confirmDialog };
}

/**
 * EntryHeadActions are the actions an entry needs seldom: linking it to the
 * entry it was before a rename, exporting it, and taking it or its backups
 * away. Which of them exist depends on the kind.
 */
export function EntryHeadActions({
  entry,
  onChanged,
  onRemoved,
  onDownloadNewest,
}: {
  entry: Entry;
  onChanged: () => void;
  /** The entry is off the list, so the page has nothing left to show. */
  onRemoved: () => void;
  /** Flash: opens the newest backup, which is where its ZIP is fetched. */
  onDownloadNewest?: () => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const { remove, busy, confirmDialog } = useRemoval();
  const [connection, setConnection] = useState(false);
  const safetyToo = useRef(false);
  const { item, detail } = entry;
  const hasBackups = item.lastBackup > 0;

  let actions: ReactNode = null;
  if (detail.kind === "container") {
    const c = detail.container;
    const takeover = { name: c.name, displayName: c.name, api: containerTakeover };
    if (!c.installed) {
      actions = (
        <OrphanRemoveButton
          hasBackups={c.lastBackup != null}
          deleteConfirm={t("containers.deleteBackupsConfirm")}
          removeConfirm={t("containers.removeEntryConfirm")}
          deleteBackups={() => deleteBackups(c.name)}
          removeEntry={() => forgetContainer(c.name)}
          onDone={onRemoved}
          t={t}
        />
      );
    } else if (!c.self) {
      actions = (
        <>
          {c.lastBackup == null && (c.aliases ?? []).length === 0 && detail.linkCandidates.length > 0 && (
            <LinkEntryPicker candidates={detail.linkCandidates} entry={takeover} onDone={onChanged} t={t} />
          )}
          <ExportButton name={c.name} t={t} tone="neutral" />
        </>
      );
    }
  } else if (detail.kind === "vm") {
    const vm = detail.vm;
    const takeover = { name: vm.libvirtName, displayName: vm.name, api: vmTakeover };
    if (vm.state === "not-installed") {
      actions = (
        <OrphanRemoveButton
          hasBackups={vm.lastBackup != null}
          deleteConfirm={t("vms.deleteBackupsConfirm")}
          removeConfirm={t("vms.removeEntryConfirm")}
          deleteBackups={() => deleteBackupsVM(vm.libvirtName, "local")}
          removeEntry={() => forgetVM(vm.libvirtName)}
          onDone={onRemoved}
          t={t}
        />
      );
    } else {
      actions = (
        <>
          {vm.lastBackup == null && (vm.aliases ?? []).length === 0 && detail.linkCandidates.length > 0 && (
            <LinkEntryPicker candidates={detail.linkCandidates} entry={takeover} onDone={onChanged} t={t} />
          )}
          <VMExportButton name={vm.libvirtName} t={t} tone="neutral" />
          {hasBackups && (
            <Button
              label={t("entry.deleteLocal")}
              labelKey="entry.deleteLocal"
              glyph={<IconTrash />}
              disabled={busy}
              onClick={() =>
                void remove(
                  t("snapshots.deleteAllConfirm"),
                  "snapshots.deleteAll",
                  () => deleteBackupsVM(vm.libvirtName, "local"),
                  onChanged
                )
              }
            />
          )}
        </>
      );
    }
  } else if (detail.kind === "files") {
    const set = detail.set;
    actions = (
      <>
        {hasBackups && (
          <Button
            label={t("snapshots.deleteAll")}
            labelKey="snapshots.deleteAll"
            glyph={<IconTrash />}
            disabled={busy}
            onClick={() =>
              void remove(
                t("files.deleteBackupsConfirm"),
                "snapshots.deleteAll",
                () => deleteFileSetBackups(set.id),
                onRemoved
              )
            }
          />
        )}
        <Button
          label={t("zfs.removeMissing")}
          labelKey="zfs.removeMissing"
          glyph={<IconTrash />}
          disabled={busy}
          onClick={() =>
            void remove(t("files.deleteSetConfirm"), "zfs.removeMissing", () => deleteFileSet(set.id), onRemoved)
          }
        />
      </>
    );
  } else if (detail.kind === "zfs") {
    const dataset = detail.dataset;
    const removeDataset = async () => {
      const res = await deleteZFSDataset(dataset.id, safetyToo.current);
      if (res.ok && res.safetyRemaining) push(t("zfs.deleteKeptSafety", res.safetyRemaining), "warn");
      if (res.ok && res.leftoversRemaining) push(t("zfs.sweepRemaining", res.leftoversRemaining), "warn");
      return res;
    };
    actions = (
      <>
        <Button
          label={t("zfs.connection.title")}
          labelKey="zfs.connection.title"
          glyph={<IconZFS />}
          onClick={() => setConnection(true)}
        />
        <Button
          label={t("zfs.removeMissing")}
          labelKey="zfs.removeMissing"
          glyph={<IconTrash />}
          disabled={busy}
          onClick={() => {
            safetyToo.current = false;
            void remove(
              t("zfs.deleteRowConfirm"),
              "zfs.removeMissing",
              removeDataset,
              onRemoved,
              dataset.safetyCount > 0 ? (
                <DeleteSafetyToggle label={t("zfs.deleteSafetyToo")} sink={safetyToo} />
              ) : undefined
            );
          }}
        />
      </>
    );
  } else if (detail.kind === "flash" && onDownloadNewest) {
    actions = (
      <Button
        label={t("entry.flashDownloadNewest")}
        labelKey="entry.flashDownloadNewest"
        glyph={<IconDownload />}
        onClick={onDownloadNewest}
      />
    );
  }

  if (!actions) return null;
  return (
    <>
      {actions}
      {confirmDialog}
      {connection && (
        <AnomalyWindow title={t("zfs.connection.title")} onClose={() => setConnection(false)}>
          <ZFSConnectionCard />
        </AnomalyWindow>
      )}
    </>
  );
}
