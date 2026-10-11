// What can be done with one entry as a whole: the two actions the page is
// there for, in its corner, and the rare ones beside its name.

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import {
  deleteBackups,
  deleteBackupsVM,
  forgetContainer,
  forgetVM,
  type OkEnvelope,
  type Run,
} from "../../lib/api";
import { useEntriesReload } from "../../lib/attention";
import { ENTRY_KIND } from "../../lib/backupEntry";
import { inSchedule, isGone, itemProgressKey } from "../../lib/backupList";
import { useBackupWatch } from "../../lib/backupWatch";
import type { Entry } from "../../lib/entry";
import { humanBytes } from "../../lib/forecast";
import { useT, type TranslationKey } from "../../lib/i18n";
import { anyActive, busyPhraseKey, useProgress } from "../../lib/progress";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useDeclineBackup } from "../../lib/useDeclineBackup";
import { useIsDesktop } from "../../lib/useMediaQuery";
import { containerTakeover, vmTakeover, type TakeoverEntry } from "../../lib/useTakeOver";
import { STOP_ACK_KEY } from "../BackupButton";
import type { MenuAction } from "../backups/ActionMenu";
import { ZFSConnectionWindow } from "../backups/ZFSConnectionWindow";
import { Button } from "../Button";
import { IconClearSelection, IconLink, IconSelectAll } from "../glyphs";
import { RunDetailSheet } from "../mobile/RunDetailSheet";
import { IconBackupNow, IconDownload, IconRestore, IconTrash, IconZFS } from "../navGlyphs";
import { PageAction } from "../PageActions";
import { PauseButton } from "../SchedulePause";
import type { TimelineState } from "./BackupPlaces";
import { ExportWindow, LiveExportWindow } from "./ExportWindow";
import { LinkWindow } from "./LinkWindow";
import { RemoveWindow, type Removable } from "./RemoveWindow";

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

type MenuWindow = "link" | "export" | "exportNewest" | "connection" | "remove";

/**
 * EntryHeadActions are the buttons beside an entry's name: linking it to the
 * entry it was before a rename, exporting it, saying that it is not to be
 * backed up, and taking it or its backups away. Which of them exist depends on
 * the kind, and the removals are the ones the server has for that kind.
 */
export function EntryHeadActions({
  entry,
  name,
  timeline,
  onChanged,
  onRemoved,
  onOpenNewest,
}: {
  entry: Entry;
  name: string;
  timeline: TimelineState;
  onChanged: () => void;
  /** The entry is off the list, so the page has nothing left to show. */
  onRemoved: () => void;
  /** Opens the newest backup, which is where the flash drive's ZIP is fetched. */
  onOpenNewest?: () => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [shown, setShown] = useState<MenuWindow | null>(null);
  const [busy, setBusy] = useState(false);
  const { item, detail } = entry;
  const hasBackups = item.lastBackup > 0;
  const newest = timeline.rows[0];
  const close = () => setShown(null);
  const reloadEntries = useEntriesReload();
  const decided = useCallback(() => {
    reloadEntries();
    onChanged();
  }, [reloadEntries, onChanged]);
  const { decline, accept, busy: deciding } = useDeclineBackup(decided);

  async function removeAfter(question: string, confirmKey: TranslationKey, call: () => Promise<OkEnvelope>, done: () => void) {
    if (!(await confirm(question, { confirmKey }))) return;
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

  const actions: MenuAction[] = [];
  const add = (id: string, label: string, glyph: ReactNode, onSelect: () => void) =>
    actions.push({ id, label, glyph, onSelect });
  const exportNewest = () => {
    if (newest) add("export", t("entry.menu.exportNewest"), <IconDownload />, () => setShown("exportNewest"));
  };
  let takeover: TakeoverEntry | null = null;
  let candidates: string[] = [];

  if (detail.kind === "container" && !detail.container.self) {
    const c = detail.container;
    if (c.installed) {
      takeover = { name: c.name, displayName: c.name, api: containerTakeover };
      if (c.lastBackup == null && (c.aliases ?? []).length === 0) candidates = detail.linkCandidates;
      if (candidates.length > 0) add("link", t("entry.menu.link"), <IconLink />, () => setShown("link"));
      add("export", t("entry.menu.exportContainer"), <IconDownload />, () => setShown("export"));
    } else {
      exportNewest();
      // The server keeps the entry of a container with backups, so they go with it.
      if (c.lastBackup != null) {
        add("remove", t("containers.deleteBackups"), <IconTrash />, () =>
          void removeAfter(t("containers.deleteBackupsConfirm"), "containers.deleteBackups", () => deleteBackups(c.name), onRemoved)
        );
      } else {
        add("remove", t("vms.removeEntry"), <IconTrash />, () =>
          void removeAfter(t("containers.removeEntryConfirm"), "vms.removeEntry", () => forgetContainer(c.name), onRemoved)
        );
      }
    }
  } else if (detail.kind === "vm") {
    const vm = detail.vm;
    if (vm.state !== "not-installed") {
      takeover = { name: vm.libvirtName, displayName: vm.name, api: vmTakeover };
      if (vm.lastBackup == null && (vm.aliases ?? []).length === 0) candidates = detail.linkCandidates;
      if (candidates.length > 0) add("link", t("entry.menu.link"), <IconLink />, () => setShown("link"));
      add("export", t("entry.menu.exportVM"), <IconDownload />, () => setShown("export"));
      if (hasBackups) {
        add("remove", t("entry.deleteLocal"), <IconTrash />, () =>
          void removeAfter(t("snapshots.deleteAllConfirm"), "snapshots.deleteAll", () => deleteBackupsVM(vm.libvirtName, "local"), onChanged)
        );
      }
    } else {
      exportNewest();
      if (vm.lastBackup != null) {
        add("remove", t("containers.deleteBackups"), <IconTrash />, () =>
          void removeAfter(t("vms.deleteBackupsConfirm"), "containers.deleteBackups", () => deleteBackupsVM(vm.libvirtName, "local"), onRemoved)
        );
      } else {
        add("remove", t("vms.removeEntry"), <IconTrash />, () =>
          void removeAfter(t("vms.removeEntryConfirm"), "vms.removeEntry", () => forgetVM(vm.libvirtName), onRemoved)
        );
      }
    }
  } else if (detail.kind === "files" || detail.kind === "zfs") {
    exportNewest();
    if (detail.kind === "zfs") add("connection", t("zfs.connection.title"), <IconZFS />, () => setShown("connection"));
    add("remove", t("zfs.removeMissing"), <IconTrash />, () => setShown("remove"));
  } else if (detail.kind === "flash" || detail.kind === "config") {
    exportNewest();
    if (detail.kind === "flash" && onOpenNewest) {
      add("download", t("entry.flashDownloadNewest"), <IconDownload />, onOpenNewest);
    }
  }

  // The server takes the mark only for an entry no schedule reaches. It
  // stands before the removals, which end the menu.
  const self = detail.kind === "container" && detail.container.self;
  if (!self && !isGone(item) && (item.backupDeclined || item.effectiveSchedule.kind === "none")) {
    const removals = actions.findIndex((action) => action.id === "remove");
    actions.splice(
      removals < 0 ? actions.length : removals,
      0,
      item.backupDeclined
        ? { id: "accept", label: t("backups.accept"), glyph: <IconSelectAll />, onSelect: () => void accept(item) }
        : { id: "decline", label: t("backups.decline"), glyph: <IconClearSelection />, onSelect: () => void decline(item, name) }
    );
  }

  if (actions.length === 0) return null;
  const removable: Removable | null =
    detail.kind === "files"
      ? { kind: "files", set: detail.set }
      : detail.kind === "zfs"
        ? { kind: "zfs", dataset: detail.dataset }
        : null;

  return (
    <>
      <span data-new="entry-menu" className="flex flex-wrap items-center justify-end gap-2 max-md:justify-start">
        {actions.map((action) => (
          <Button
            key={action.id}
            label={action.label}
            labelKey={null}
            glyph={action.glyph}
            disabled={busy || deciding}
            onClick={action.onSelect}
            className="glim-btn-wrap"
          />
        ))}
      </span>
      {confirmDialog}
      {shown === "link" && takeover && (
        <LinkWindow
          candidates={candidates}
          entry={takeover}
          onDone={() => {
            close();
            onChanged();
          }}
          onClose={close}
        />
      )}
      {shown === "export" && <LiveExportWindow entry={entry} name={name} onClose={close} />}
      {shown === "exportNewest" && newest && (
        <ExportWindow entry={entry} name={name} row={newest} timeline={timeline} onClose={close} />
      )}
      {shown === "connection" && <ZFSConnectionWindow onClose={close} />}
      {shown === "remove" && removable && (
        <RemoveWindow target={removable} name={name} hasBackups={hasBackups} onRemoved={onRemoved} onClose={close} />
      )}
    </>
  );
}
