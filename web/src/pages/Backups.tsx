// The Backups page: everything that is backed up in one list, whatever its
// kind. The tiles choose what the list shows, the bar above it filters, orders
// and searches, and a row says in one status how its entry is doing.
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { BackupRow, SelfRow } from "../components/backups/BackupRow";
import { KIND_LABEL, KIND_SCHEDULE_LABEL, entryName, tileLabel } from "../components/backups/kinds";
import { KindTiles } from "../components/backups/KindTiles";
import { ListBar } from "../components/backups/ListBar";
import { Button } from "../components/Button";
import { useScheduleSentence } from "../components/EffectiveScheduleLine";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconArchive, IconSearch } from "../components/glyphs";
import { InfoBubble } from "../components/InfoBubble";
import { IconAdd, IconBackupNow } from "../components/navGlyphs";
import { PageAction, PageActions } from "../components/PageActions";
import { PageTitle } from "../components/PageTitle";
import { Card } from "./settings/shared";
import {
  ApiError,
  backupAll,
  backupEverythingNow,
  backupFilesAll,
  backupFlashNow,
  backupZFSAll,
  discoverAll,
  getSettings,
  listItems,
  type BackupItem,
  type ListItemsResponse,
  type OkEnvelope,
  type Settings,
} from "../lib/api";
import type { EntryKind } from "../lib/backupEntry";
import {
  NOT_INSTALLED,
  TILE_KINDS,
  addTarget,
  entriesOf,
  filterEntries,
  inSchedule,
  isGone,
  itemProgressKey,
  listedItems,
  loadListView,
  parseTile,
  runStrip,
  runsNow,
  saveListView,
  showsSelf,
  sortItems,
  tileEntries,
  tilePath,
  type ListView,
  type Tile,
  type TileKind,
} from "../lib/backupList";
import { useT } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { anyActive, busyPhraseKey, useProgress } from "../lib/progress";
import { useToast } from "../lib/toast";
import { useAnomalyItems, useAnomalySummary, useOpenAnomalies } from "../lib/useAnomalies";
import { useConfirm } from "../lib/useConfirm";
import { useLoadMore } from "../lib/useLoadMore";
import { useIsDesktop } from "../lib/useMediaQuery";

const PHONE_ROWS = 20;

// What "Back up all now" starts for a kind. The server runs each of these by
// itself, so the page may be closed. It has no such batch for VMs.
const BATCH: Partial<Record<TileKind, (keys: string[]) => Promise<OkEnvelope>>> = {
  container: backupAll,
  files: backupFilesAll,
  zfs: backupZFSAll,
  flash: () => backupFlashNow(),
};

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
  const [view, setView] = useState(loadListView);
  const [query, setQuery] = useState("");
  const [discovering, setDiscovering] = useState(false);
  const [starting, setStarting] = useState(false);

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

  // A reload replaces the array, which would send a phone back to its first
  // rows. The key keeps the reader's place until the view itself changes.
  const windowKey = JSON.stringify([isDesktop, tile, view, query]);
  const { visible, showMore, hasMore } = useLoadMore(shown, isDesktop ? Infinity : PHONE_ROWS, windowKey);
  const installed = visible.filter((item) => !isGone(item));
  const notInstalled = visible.filter(isGone);
  const selfShown = showsSelf(self, tile, view, query) && installed.length === shown.filter((item) => !isGone(item)).length;

  function changeView(next: ListView) {
    setView(next);
    saveListView(next);
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
      await load();
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.discoverFailed"), "fail");
    } finally {
      setDiscovering(false);
    }
  }

  async function start(run: () => Promise<OkEnvelope>, started: string, alreadyRunning: string) {
    setStarting(true);
    try {
      const res = await run();
      if (res.ok) push(started, "success");
      else push(res.error ?? t("containers.backupStartFailed"), "fail");
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) push(alreadyRunning, "warn");
      else push(err instanceof Error ? err.message : t("containers.backupStartFailed"), "fail");
    } finally {
      setStarting(false);
    }
  }

  async function backUpEverything() {
    // On a phone the corner is one tap from the thumb.
    if (!isDesktop && !(await confirm(t("home.newBackupConfirm"), { confirmKey: "settings.everythingTitle" }))) return;
    await start(backupEverythingNow, t("settings.everythingStarted"), t("settings.everythingAlreadyRunning"));
  }

  async function backUpKind(kind: TileKind, batch: (keys: string[]) => Promise<OkEnvelope>) {
    const keys = tileEntries(entries, kind)
      .filter((item) => !isGone(item) && !item.kindDisabled && inSchedule(item))
      .map((item) => item.key);
    if (keys.length === 0) {
      push(t("backups.nothingScheduled"), "warn");
      return;
    }
    await start(() => batch(keys), t("containers.batchStarted"), t("containers.batchAlreadyRunning"));
  }

  const kindTile = tile === "all" || tile === NOT_INSTALLED ? null : tile;
  const batch = kindTile && BATCH[kindTile];
  const addPath = addTarget(tile, enabled);
  const busyTitle = running.active ? t(busyPhraseKey(running.phase, running.stage)) : undefined;

  // An open finding that is more than a note colours the run it was raised on.
  const flagged = (runId: string) => (byRunId.get(runId) ?? []).some((a) => a.severity !== "info");
  const row = (item: BackupItem) => (
    <BackupRow
      key={`${item.kind}:${item.key}`}
      item={item}
      name={entryName(item, t)}
      tones={runStrip(item, flagged)}
      progress={progress[itemProgressKey(item)]}
      anomaly={anomalies.find(item.kind, item.key)}
      anomalyEnabled={anomalyEnabled}
      scheduleSentence={(it) => sentence(it.effectiveSchedule, KIND_SCHEDULE_LABEL[it.kind])}
      t={t}
      lang={lang}
    />
  );

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
                <div className="@container glim-content-fade">
                  <ul>
                    {installed.map(row)}
                    {selfShown && <SelfRow item={self} t={t} />}
                  </ul>
                  {notInstalled.length > 0 && (installed.length > 0 || selfShown) && (
                    <h3 className="flex items-center gap-1.5 border-t border-carbon-border px-1.5 pb-1.5 pt-4 text-xs font-medium uppercase tracking-wider text-carbon-textMuted">
                      {t("containers.notInstalledTitle")}
                      <InfoBubble tip={t("backups.notInstalledHint")} />
                    </h3>
                  )}
                  <ul>{notInstalled.map(row)}</ul>
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
            </div>
          )}
        </>
      )}

      {list && !failed && (
        <PageActions>
          <PageAction
            label={t("containers.discover")}
            glyph={<IconSearch />}
            onClick={() => void discover()}
            disabled={discovering}
            busy={discovering}
            title={discovering ? t("containers.discovering") : t("containers.discoverHint")}
          />
          {tile === "all" ? (
            <PageAction
              label={t("backups.backUpEverything")}
              glyph={<IconBackupNow />}
              onClick={() => void backUpEverything()}
              disabled={starting || running.active}
              title={busyTitle}
            />
          ) : (
            kindTile &&
            batch && (
              <PageAction
                label={t("files.backupAll")}
                glyph={<IconBackupNow />}
                onClick={() => void backUpKind(kindTile, batch)}
                disabled={starting || running.active}
                title={busyTitle}
              />
            )
          )}
          {addPath && (
            <PageAction
              primary
              label={t(tile === "files" ? "backups.addFolder" : tile === "zfs" ? "backups.addDataset" : "folders.add")}
              glyph={<IconAdd />}
              onClick={() => navigate(addPath)}
            />
          )}
        </PageActions>
      )}
    </div>
  );
}
