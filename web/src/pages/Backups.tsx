// The Backups page: everything that is backed up in one list, whatever its
// kind. The tiles choose what the list shows, the bar above it filters, orders
// and searches, and a row says in one status how its entry is doing. Rows can
// be picked like files, and what is picked is acted on together.
import { useCallback, useEffect, useMemo, useRef, useState, type MouseEvent } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { ActionMenu, type MenuAction, type MenuPlace } from "../components/backups/ActionMenu";
import { AddWindow } from "../components/backups/AddWindow";
import { BackupRow, LIST_COLUMNS, LIST_SPAN, SelfRow } from "../components/backups/BackupRow";
import { BarButton, BulkBar } from "../components/backups/BulkBar";
import { KIND_LABEL, KIND_SCHEDULE_LABEL, entryName, tileLabel } from "../components/backups/kinds";
import { KindTiles } from "../components/backups/KindTiles";
import { ListBar } from "../components/backups/ListBar";
import { BatchNote, DumpIntro, ZFSDownNote } from "../components/backups/ListNotes";
import { StacksCard, StackWindow } from "../components/backups/StackWindow";
import { useBulkActions } from "../components/backups/useBulkActions";
import { useRowSelection } from "../components/backups/useRowSelection";
import { ZFSConnectionWindow } from "../components/backups/ZFSConnectionWindow";
import { stopAcknowledged } from "../components/BackupButton";
import { Button } from "../components/Button";
import { useScheduleSentence } from "../components/EffectiveScheduleLine";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconArchive, IconClearSelection, IconEye, IconPause, IconSearch, IconSelectAll } from "../components/glyphs";
import { InfoBubble } from "../components/InfoBubble";
import { IconAdd, IconBackupNow, IconClose, IconMore, IconRestore, IconSchedules, IconZFS } from "../components/navGlyphs";
import { PageAction, PageActions } from "../components/PageActions";
import { PageTitle } from "../components/PageTitle";
import { zfsReaches } from "../components/zfs/ZFSConnectionCard";
import { Card } from "./settings/shared";
import {
  ApiError,
  backupEverythingNow,
  discoverAll,
  getSettings,
  listItems,
  zfsConnection,
  type BackupItem,
  type ListItemsResponse,
  type Settings,
  type ZFSConnectionResult,
} from "../lib/api";
import { ENTRY_KIND, type EntryKind } from "../lib/backupEntry";
import {
  NOT_INSTALLED,
  TILE_KINDS,
  addStart,
  canBackUp,
  entriesOf,
  entryTarget,
  filterEntries,
  inSchedule,
  isGone,
  itemId,
  itemProgressKey,
  itemReplicaKey,
  listedItems,
  loadListView,
  parseTile,
  runStrip,
  runsNow,
  saveListView,
  showsSelf,
  sortItems,
  stacksOf,
  tileEntries,
  tilePath,
  type AddedKind,
  type ListView,
  type Tile,
} from "../lib/backupList";
import { introDatabases } from "../lib/dbdump";
import { useT } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { anyActive, busyPhraseKey, useProgress } from "../lib/progress";
import { useToast } from "../lib/toast";
import { useAnomalyItems, useAnomalySummary, useOpenAnomalies } from "../lib/useAnomalies";
import { useConfirm } from "../lib/useConfirm";
import { useLoadMore } from "../lib/useLoadMore";
import { useIsDesktop } from "../lib/useMediaQuery";

const PHONE_ROWS = 20;

// Storage keys keep the bv- prefix, since renaming one resets what every
// browser has stored under it.
const DBDUMP_INTRO_KEY = "bv-dbdump-intro-seen";

function introSeen(): boolean {
  try {
    return localStorage.getItem(DBDUMP_INTRO_KEY) === "1";
  } catch {
    // Without storage the note shows again, which is the harmless way to be wrong.
    return false;
  }
}

function kindsEnabled(settings: Settings | null): Partial<Record<EntryKind, boolean>> {
  if (!settings) return {};
  return {
    container: settings.containersEnabled,
    vm: settings.vmsEnabled,
    flash: settings.flashEnabled,
    files: settings.filesEnabled,
    zfs: settings.zfsEnabled,
    config: settings.configEnabled,
  };
}

export function Backups() {
  const { t, lang } = useT();
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const isDesktop = useIsDesktop();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const progress = useProgress();
  const anomalies = useAnomalyItems();
  const anomalyEnabled = useAnomalySummary().summary?.enabled ?? false;
  const { byRunId } = useOpenAnomalies();
  const sentence = useScheduleSentence();

  const [list, setList] = useState<ListItemsResponse | null>(null);
  const [failed, setFailed] = useState(false);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [host, setHost] = useState({ mountRoot: "", sourceRoot: "" });
  const [view, setView] = useState(loadListView);
  const [query, setQuery] = useState("");
  const [discovering, setDiscovering] = useState(false);
  const [starting, setStarting] = useState(false);
  const [introDismissed, setIntroDismissed] = useState(introSeen);
  const [rowMenu, setRowMenu] = useState<{ place: MenuPlace; id: string } | null>(null);
  const [moreOpen, setMoreOpen] = useState(false);
  const moreButton = useRef<HTMLSpanElement>(null);
  const [adding, setAdding] = useState<AddedKind | "choose" | null>(null);
  const [stackOpen, setStackOpen] = useState<string | null>(null);
  const [zfsOpen, setZfsOpen] = useState(false);
  const [zfsDown, setZfsDown] = useState<ZFSConnectionResult | null>(null);

  // Only the newest read may land, or a slow answer would bring back a list
  // that a later one has already replaced.
  const read = useRef(0);
  const load = useCallback(() => {
    const n = ++read.current;
    return listItems()
      .then((res) => {
        if (n !== read.current) return;
        if (res.ok) setList(res);
        setFailed(!res.ok);
      })
      .catch(() => {
        if (n === read.current) setFailed(true);
      });
  }, []);

  useEffect(() => {
    void load();
    getSettings()
      .then((res) => {
        if (res.ok && res.settings) setSettings(res.settings);
        if (res.hostMountRoot) setHost({ mountRoot: res.hostMountRoot, sourceRoot: res.hostSourceRoot ?? res.hostMountRoot });
      })
      .catch(() => undefined);
  }, [load]);

  // The list changes when a run ends and not in between, so it is read again
  // then and never on a timer: listing the VMs costs the server a round trip
  // to libvirt.
  const running = anyActive(progress);
  const wasBusy = useRef(false);
  useEffect(() => {
    if (wasBusy.current && !running.active) void load();
    wasBusy.current = running.active;
  }, [running.active, load]);

  // A replica run reports apart from the backups and ends the same way, so
  // the list is read again when the last one does.
  const replicating = Object.entries(progress).some(([key, state]) => key.startsWith("zfs-replica:") && state.active);
  const wasReplicating = useRef(false);
  useEffect(() => {
    if (wasReplicating.current && !replicating) void load();
    wasReplicating.current = replicating;
  }, [replicating, load]);

  const items = useMemo(() => listedItems(list?.items ?? []), [list]);
  const entries = useMemo(() => entriesOf(items), [items]);
  const self = items.find((item) => item.self);
  const enabled = kindsEnabled(settings);

  const gone = entries.filter(isGone).length;
  const tiles: { tile: Tile; count: number }[] = [
    { tile: "all", count: entries.length },
    ...TILE_KINDS.filter((kind) => enabled[kind] || entries.some((item) => item.kind === kind)).map((kind) => ({
      tile: kind,
      count: tileEntries(entries, kind).length,
    })),
    ...(gone > 0 ? [{ tile: NOT_INSTALLED, count: gone } as const] : []),
  ];
  const asked = parseTile(params.get("kind"));
  const tile = tiles.some((x) => x.tile === asked) ? asked : "all";

  const shown = useMemo(() => {
    const nameOf = (item: BackupItem) => entryName(item, t);
    return sortItems(filterEntries(entries, tile, view, query, nameOf), view.sort, {
      nameOf,
      isRunning: (item) => runsNow(progress[itemProgressKey(item)]),
      lang,
    });
  }, [entries, tile, view, query, progress, t, lang]);

  const selection = useRowSelection(useMemo(() => shown.map(itemId), [shown]));
  const pickedItems = shown.filter((item) => selection.picked.has(itemId(item)));
  const actions = useBulkActions({ progress, reload: load, confirm, push, t });
  const nameOf = (item: BackupItem) => entryName(item, t);

  // A reload replaces the array, which would send a phone back to its first
  // rows. The key keeps the reader's place until the view itself changes.
  const windowKey = JSON.stringify([isDesktop, tile, view, query]);
  const { visible, showMore, hasMore } = useLoadMore(shown, isDesktop ? Infinity : PHONE_ROWS, windowKey);
  const installed = visible.filter((item) => !isGone(item));
  const notInstalled = visible.filter(isGone);
  const selfShown = showsSelf(self, tile, view, query) && installed.length === shown.filter((item) => !isGone(item)).length;
  const showIp = view.sort.field === "ip";

  const onContainers = tile === "all" || tile === "container";
  const stacks = useMemo(() => stacksOf(items, lang), [items, lang]);
  const databases = onContainers
    ? introDatabases(entries.filter((item) => item.kind === "container" && !isGone(item)))
    : [];

  // A connection that is down fails every ZFS entry, so the list of them
  // says so. It is asked again once its window closes.
  const onZFS = tile === "zfs" && !!enabled.zfs;
  useEffect(() => {
    if (!onZFS || zfsOpen) return;
    let alive = true;
    zfsConnection()
      .then((res) => {
        if (alive) setZfsDown(zfsReaches(res) ? null : res);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, [onZFS, zfsOpen]);

  function changeView(next: ListView) {
    setView(next);
    saveListView(next);
  }

  function dismissIntro() {
    setIntroDismissed(true);
    try {
      localStorage.setItem(DBDUMP_INTRO_KEY, "1");
    } catch {
      // The note comes back next time.
    }
  }

  async function discover() {
    setDiscovering(true);
    try {
      const found = await discoverAll();
      if (found.skipped.length > 0) {
        push(t("common.discoverSkipped").replace("{list}", found.skipped.join(", ")), "warn");
      }
      const count = found.containers + found.vms + found.files + found.zfs;
      if (found.error) push(found.error, "fail");
      else push(count > 0 ? t("backups.discover.found", count) : t("backups.discover.none"), "success");
      // The count does not say that a rebuilt ZFS item stays off until it was checked.
      if (found.zfs > 0) push(t("zfs.discovered", found.zfs), "success");
      await load();
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.discoverFailed"), "fail");
    } finally {
      setDiscovering(false);
    }
  }

  async function backUpEverything() {
    // On a phone the corner is one tap from the thumb.
    if (!isDesktop && !(await confirm(t("home.newBackupConfirm"), { confirmKey: "settings.everythingTitle" }))) return;
    setStarting(true);
    try {
      const res = await backupEverythingNow();
      if (res.ok) push(t("settings.everythingStarted"), "success");
      else push(res.error ?? t("containers.backupStartFailed"), "fail");
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) push(t("settings.everythingAlreadyRunning"), "warn");
      else push(err instanceof Error ? err.message : t("containers.backupStartFailed"), "fail");
    } finally {
      setStarting(false);
    }
  }

  // A tile backs up what it shows and the schedule would run.
  async function backUpTile() {
    const scheduled = tileEntries(entries, tile).filter((item) => canBackUp(item) && inSchedule(item));
    if (scheduled.length === 0) {
      push(t("backups.nothingScheduled"), "warn");
      return;
    }
    await actions.backUp(scheduled);
  }

  async function backUpOne(item: BackupItem) {
    if (item.kind === "container" && !(await stopAcknowledged(confirm, t))) return;
    await actions.backUpOne(item);
  }

  function openRowMenu(e: MouseEvent, item: BackupItem) {
    setRowMenu({ place: { x: e.clientX, y: e.clientY }, id: itemId(item) });
  }

  // A right click acts on the row, or on the whole selection when the row is
  // part of it.
  function rowMenuActions(item: BackupItem): { heading?: string; actions: MenuAction[] } {
    const id = itemId(item);
    if (selection.picked.has(id) && selection.picked.size > 1) {
      const inPlan = pickedItems.every((it) => it.included);
      return {
        heading: t("zfs.add.selected", pickedItems.length),
        actions: [
          { id: "backup", label: t("containers.backupSelected"), glyph: <IconBackupNow />, onSelect: () => void actions.backUp(pickedItems) },
          inPlan
            ? { id: "pause", label: t("schedule.pause"), glyph: <IconPause />, onSelect: () => void actions.setIncluded(pickedItems, false, nameOf) }
            : { id: "resume", label: t("schedule.resume"), glyph: <IconSchedules />, onSelect: () => void actions.setIncluded(pickedItems, true, nameOf) },
          { id: "clear", label: t("backups.select.clear"), glyph: <IconClearSelection />, onSelect: selection.clear },
        ],
      };
    }
    const live = progress[itemProgressKey(item)];
    const list: MenuAction[] = [];
    if (runsNow(live) && live.phase === "backup") {
      list.push({ id: "cancel", label: t("backup.cancel"), glyph: <IconClose />, onSelect: () => void actions.cancelBackup(item, nameOf(item)) });
    } else if (canBackUp(item) && !runsNow(live)) {
      list.push({ id: "backup", label: t("containers.backupNow"), glyph: <IconBackupNow />, onSelect: () => void backUpOne(item) });
    }
    if (ENTRY_KIND[item.kind].setIncluded) {
      list.push(
        item.included
          ? { id: "pause", label: t("schedule.pause"), glyph: <IconPause />, onSelect: () => void actions.setIncluded([item], false, nameOf) }
          : { id: "resume", label: t("schedule.resume"), glyph: <IconSchedules />, onSelect: () => void actions.setIncluded([item], true, nameOf) }
      );
    }
    list.push(
      { id: "open", label: t("instances.open"), glyph: <IconEye />, onSelect: () => navigate(entryTarget(item)) },
      { id: "select", label: t("backups.select"), glyph: <IconSelectAll />, onSelect: () => selection.set(id, true) }
    );
    return { actions: list };
  }

  const moreActions: MenuAction[] = [
    { id: "select", label: t("backups.select"), glyph: <IconSelectAll />, onSelect: () => selection.begin() },
    { id: "discover", label: t("containers.discover"), glyph: <IconSearch />, onSelect: () => void discover() },
    ...(stacks.length > 0
      ? [{ id: "stack", label: t("backups.stack.restore"), glyph: <IconRestore />, onSelect: () => setStackOpen(stacks[0].project) }]
      : []),
    ...(enabled.zfs
      ? [{ id: "zfs", label: t("backups.zfsConnection"), glyph: <IconZFS />, onSelect: () => setZfsOpen(true) }]
      : []),
  ];

  const addWith = addStart(tile, enabled);
  const busyTitle = running.active ? t(busyPhraseKey(running.phase, running.stage)) : undefined;
  const menuItem = rowMenu && shown.find((item) => itemId(item) === rowMenu.id);
  const menu = menuItem && rowMenuActions(menuItem);

  // An open finding that is more than a note colours the run it was raised on.
  const flagged = (runId: string) => (byRunId.get(runId) ?? []).some((a) => a.severity !== "info");
  const row = (item: BackupItem) => {
    const id = itemId(item);
    return (
      <BackupRow
        key={id}
        item={item}
        name={nameOf(item)}
        tones={runStrip(item, flagged)}
        progress={progress[itemProgressKey(item)]}
        replicaProgress={progress[itemReplicaKey(item)]}
        anomaly={anomalies.find(item.kind, item.key)}
        anomalyEnabled={anomalyEnabled}
        scheduleSentence={(it) => sentence(it.effectiveSchedule, KIND_SCHEDULE_LABEL[it.kind])}
        picked={selection.picked.has(id)}
        showIp={showIp}
        onClick={(e) => selection.click(e, id)}
        onPick={(on) => selection.set(id, on)}
        onMenu={(e) => openRowMenu(e, item)}
        onLongPress={() => selection.begin(id)}
        t={t}
        lang={lang}
      />
    );
  };

  return (
    <div className={PAGE_SHELL_RESPONSIVE}>
      <PageTitle>{t("backups.title")}</PageTitle>
      {confirmDialog}

      {failed ? (
        <div className="flex flex-col items-start gap-2">
          <p className="text-sm text-statusWarn">{t("backups.loadFailed")}</p>
          <Button label={t("anomaly.retry")} labelKey="anomaly.retry" onClick={() => void load()} />
        </div>
      ) : !list ? (
        <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
      ) : (
        <>
          <KindTiles tiles={tiles} chosen={tile} onChoose={(next) => navigate(tilePath(next), { replace: true })} t={t} />

          {list.unlisted.length > 0 && (
            <p className="rounded-card bg-carbon-surface p-4 text-sm text-statusWarn">
              {t("backups.unlisted").replace(
                "{kinds}",
                new Intl.ListFormat(lang, { type: "conjunction" }).format(list.unlisted.map((kind) => t(KIND_LABEL[kind])))
              )}
            </p>
          )}

          {!introDismissed && databases.length > 0 && <DumpIntro databases={databases} onDismiss={dismissIntro} t={t} />}
          {actions.batch && <BatchNote {...actions.batch} t={t} />}
          {selection.selecting && (
            <BulkBar count={selection.picked.size} allPicked={selection.allPicked} onAll={selection.setAll} onDone={selection.clear} t={t}>
              <BarButton
                label={t("containers.backupSelected")}
                glyph={<IconBackupNow />}
                onClick={() => void actions.backUp(pickedItems)}
                disabled={actions.busy || running.active}
                title={busyTitle}
              />
              <BarButton
                label={t("schedule.resume")}
                glyph={<IconSchedules />}
                onClick={() => void actions.setIncluded(pickedItems, true, nameOf)}
                disabled={actions.busy}
              />
              <BarButton
                label={t("schedule.pause")}
                glyph={<IconPause />}
                onClick={() => void actions.setIncluded(pickedItems, false, nameOf)}
                disabled={actions.busy}
              />
            </BulkBar>
          )}
          {onZFS && zfsDown && <ZFSDownNote result={zfsDown} onView={() => setZfsOpen(true)} t={t} />}

          {entries.length === 0 && !self ? (
            <div className="flex flex-col items-center gap-3 rounded-card bg-carbon-surface p-6 text-center">
              <EmptyStateIcon icon={IconArchive} />
              <p className="text-sm text-carbon-text">{t("backups.empty")}</p>
              <p className="max-w-prose text-sm text-carbon-textMuted">{t("backups.emptyHow")}</p>
            </div>
          ) : (
            <div className="flex flex-col gap-4 md:gap-6">
              <ListBar view={view} onView={changeView} query={query} onQuery={setQuery} t={t} />
              <Card title={tile === "all" ? t("backups.allEntries") : tileLabel(tile, t)} hueIndex={0}>
                <div className={`@container glim-content-fade${selection.selecting ? " select-none" : ""}`}>
                  <div className={LIST_COLUMNS}>
                    <ul className={LIST_SPAN}>
                      {installed.map(row)}
                      {selfShown && <SelfRow item={self} showIp={showIp} t={t} />}
                    </ul>
                    {notInstalled.length > 0 && (installed.length > 0 || selfShown) && (
                      <h3 className="col-span-full flex items-center gap-1.5 border-t border-carbon-border px-1.5 pb-1.5 pt-4 text-xs font-medium uppercase tracking-wider text-carbon-textMuted">
                        {t("containers.notInstalledTitle")}
                        <InfoBubble tip={t("backups.notInstalledHint")} />
                      </h3>
                    )}
                    <ul className={LIST_SPAN}>{notInstalled.map(row)}</ul>
                  </div>
                  {shown.length === 0 && !selfShown && (
                    <p className="px-1.5 py-3 text-sm text-carbon-textMuted">{t("filter.noMatch")}</p>
                  )}
                  {hasMore && (
                    <div className="pt-2">
                      <Button
                        label={t("common.loadMore")}
                        labelKey="common.loadMore"
                        tone="subtle"
                        onClick={showMore}
                        className="min-h-11 w-full justify-center"
                      />
                    </div>
                  )}
                </div>
              </Card>
              {tile === "container" && stacks.length > 0 && <StacksCard stacks={stacks} onRestore={setStackOpen} t={t} />}
            </div>
          )}
        </>
      )}

      {menuItem && menu && rowMenu && (
        <ActionMenu
          label={nameOf(menuItem)}
          heading={menu.heading}
          actions={menu.actions}
          place={rowMenu.place}
          onClose={() => setRowMenu(null)}
        />
      )}
      {moreOpen && moreButton.current && (
        <ActionMenu label={t("nav.more")} actions={moreActions} place={{ anchor: moreButton.current }} onClose={() => setMoreOpen(false)} />
      )}
      {adding && (
        <AddWindow
          start={adding}
          enabled={enabled}
          hostMountRoot={host.mountRoot}
          hostRoot={host.sourceRoot}
          onClose={() => setAdding(null)}
          onAdded={() => void load()}
        />
      )}
      {stackOpen !== null && stacks.length > 0 && (
        <StackWindow stacks={stacks} initial={stackOpen} onClose={() => setStackOpen(null)} onStarted={() => void load()} />
      )}
      {zfsOpen && <ZFSConnectionWindow onClose={() => setZfsOpen(false)} />}

      {list && !failed && (
        <PageActions>
          <span ref={moreButton} data-new="backups-actions" className="flex">
            <PageAction
              label={t("nav.more")}
              glyph={<IconMore />}
              onClick={() => setMoreOpen((was) => !was)}
              busy={discovering}
              title={discovering ? t("containers.discovering") : undefined}
            />
          </span>
          {tile === "all" ? (
            <PageAction
              label={t("backups.backUpEverything")}
              glyph={<IconBackupNow />}
              onClick={() => void backUpEverything()}
              disabled={starting || running.active}
              title={busyTitle}
            />
          ) : (
            tile !== NOT_INSTALLED && (
              <PageAction
                label={t("files.backupAll")}
                glyph={<IconBackupNow />}
                onClick={() => void backUpTile()}
                disabled={actions.busy || running.active}
                title={busyTitle}
              />
            )
          )}
          {addWith && (
            <PageAction
              primary
              label={t(tile === "files" ? "backups.addFolder" : tile === "zfs" ? "backups.addDataset" : "folders.add")}
              glyph={<IconAdd />}
              onClick={() => setAdding(addWith)}
            />
          )}
        </PageActions>
      )}
    </div>
  );
}
