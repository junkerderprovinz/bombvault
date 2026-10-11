// The Restore page. Most restores bring one entry back to one moment, so the
// page opens on the entries and the backups of the one that is picked. The
// rare ways, the emergency and the other sources, wait at the bottom.
import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { useNavigate } from "react-router-dom";

import { KIND_LABEL, KindGlyph, entryName, kindHue } from "../components/backups/kinds";
import { KindTile, TileRow } from "../components/backups/KindTiles";
import { Button } from "../components/Button";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconArchive, IconSearch } from "../components/glyphs";
import { InstanceWindow } from "./instances/InstanceWindow";
import { IconRestore } from "../components/navGlyphs";
import { PageTitle } from "../components/PageTitle";
import { ProgressBar } from "../components/ProgressBar";
import { BackupRail } from "../components/restore/BackupRail";
import { PlaceChecks } from "../components/restore/PlaceChecks";
import { RestoreWindow, type RestoreAsk } from "../components/restore/RestoreWindow";
import { RestoreCancelButton } from "../components/RestoreCancelButton";
import { displayTags } from "../components/RestorePanel";
import { Rows } from "../components/storage/rows";
import { listItems, type BackupItem, type TimelineRow } from "../lib/api";
import { hueVars } from "../lib/appearance";
import { ENTRY_KIND, ENTRY_KINDS, type EntryKind } from "../lib/backupEntry";
import { entriesOf, entryTarget, itemId, itemProgressKey, listedItems } from "../lib/backupList";
import { useT } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { anyActive, useProgress, type ProgressState } from "../lib/progress";
import { formatRecent } from "../lib/reltime";
import { findingsOf, recommend, recommendationWhy } from "../lib/restorePlan";
import { useOpenAnomalies } from "../lib/useAnomalies";
import { useEntry } from "../lib/useEntry";
import { useTimeline } from "../lib/useTimeline";
import { Card, hueCounter } from "./settings/shared";

// The tiles stand in the order of the Backups page, with the self-backup last.
const TILE_ORDER: EntryKind[] = ["container", "vm", "flash", "files", "zfs", "config"];

function hasBackups(item: BackupItem): boolean {
  return item.lastBackup > 0;
}

/** EntryRestore reads what the window needs to know about the entry, which
 *  the list of all entries does not carry. */
function EntryRestore({
  item,
  name,
  timeline,
  ask,
  onClose,
}: {
  item: BackupItem;
  name: string;
  timeline: Parameters<typeof RestoreWindow>[0]["timeline"];
  ask: RestoreAsk;
  onClose: () => void;
}) {
  const { t } = useT();
  const { entry, status } = useEntry(item.kind, item.key);
  if (entry) return <RestoreWindow entry={entry} name={name} timeline={timeline} ask={ask} onClose={onClose} />;
  if (status === "loading") return null;
  return (
    <InstanceWindow
      title={t("entry.restore.title").replace("{name}", () => name)}
      onClose={onClose}
      footer={<Button label={t("common.close")} labelKey="common.close" tone="accent" onClick={onClose} />}
    >
      <p className="text-sm text-statusWarn">{t(status === "missing" ? "entry.missing" : "backups.loadFailed")}</p>
    </InstanceWindow>
  );
}

/** The backups of the picked entry, and the window a restore of one opens. */
function PickedEntry({ item, hueIndex }: { item: BackupItem; hueIndex: number }) {
  const { t, lang } = useT();
  const navigate = useNavigate();
  const { list: openFindings } = useOpenAnomalies();
  const name = entryName(item, t);
  const domain = ENTRY_KIND[item.kind].timelineDomain;
  const read = useTimeline(domain, item.key, true);
  const timeline = useMemo(() => ({ ...read, domain }), [read, domain]);
  const { rows, loading, error } = read;
  const findings = useMemo(() => findingsOf(openFindings, item.kind, item.key), [openFindings, item.kind, item.key]);
  const recommendation = useMemo(() => recommend(rows, findings), [rows, findings]);
  const [pick, setPick] = useState<string | null>(null);
  const [opened, setOpened] = useState<RestoreAsk | null>(null);
  const card = useRef<HTMLDivElement>(null);

  const picked = rows.findIndex((row) => row.key === pick);
  const chosen = picked >= 0 ? picked : recommendation.index;

  // The card of another entry may stand below the fold.
  useEffect(() => {
    card.current?.scrollIntoView?.({ block: "nearest", behavior: "smooth" });
  }, []);

  const tagsOf = (row: TimelineRow) =>
    item.kind === "container"
      ? displayTags([...new Set(row.places.flatMap((m) => m.tags))], item.key, item.formerNames ?? [])
      : [];

  return (
    <div ref={card}>
      <Card title={name} hueIndex={hueIndex}>
        {loading && <p className="text-sm text-carbon-textMuted">{t("common.loadingBackups")}</p>}
        {error !== null && <p className="text-sm text-statusFail">{error || t("common.loadBackupsFailed")}</p>}
        {!loading && error === null && rows.length === 0 && (
          <p className="text-sm text-carbon-textMuted">{t("restoreWindow.noBackup")}</p>
        )}
        {rows.length > 0 && (
          <BackupRail
            timeline={timeline}
            recommendation={recommendation}
            chosen={chosen}
            why={recommendationWhy(recommendation, t, lang)}
            tagsOf={tagsOf}
            onChoose={(i) => setPick(rows[i].key)}
            onRestore={(i) => setOpened({ snapshot: rows[i].key })}
          />
        )}
        <PlaceChecks timeline={timeline} />
        <div className="flex justify-end">
          <Button
            label={t("restorePage.openEntry")}
            labelKey={null}
            glyph={<IconArchive />}
            tone="subtle"
            onClick={() => navigate(entryTarget(item))}
          />
        </div>
      </Card>
      {opened && (
        <EntryRestore
          key={opened.snapshot}
          item={item}
          name={name}
          timeline={timeline}
          ask={opened}
          onClose={() => setOpened(null)}
        />
      )}
    </div>
  );
}

/** The restores that run, each with its progress and the way to stop it. */
function RunningCard({
  running,
  nameOf,
  hueIndex,
}: {
  running: [string, ProgressState][];
  nameOf: (key: string) => string;
  hueIndex: number;
}) {
  const { t } = useT();
  return (
    <Card title={t("restorePage.running")} hueIndex={hueIndex}>
      <Rows>
        {running.map(([key, state]) => (
          <div key={key} data-testid="running-restore" className="flex flex-wrap items-center gap-x-3 gap-y-2 py-3 first:pt-0 last:pb-0">
            <span className="flex h-9 w-9 flex-none items-center justify-center rounded-control bg-carbon-surface2 text-accentText [&_svg]:h-[18px] [&_svg]:w-[18px]">
              <IconRestore />
            </span>
            <span className="flex min-w-40 flex-1 flex-col gap-1.5">
              <span className="text-sm font-semibold text-carbon-text wrap-anywhere">
                {t("restorePage.restoring").replace("{name}", () => nameOf(key))}
              </span>
              <ProgressBar percent={state.percent} active inline />
            </span>
            <span className="text-sm tabular-nums text-carbon-textSub">{Math.round(state.percent)} %</span>
            <RestoreCancelButton cancelKey={key} inPlace name={nameOf(key)} t={t} />
          </div>
        ))}
      </Rows>
    </Card>
  );
}

export function Restore() {
  const { t, lang } = useT();
  const progress = useProgress();
  const { list: openFindings } = useOpenAnomalies();

  const [items, setItems] = useState<BackupItem[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [kind, setKind] = useState<EntryKind | null>(null);
  const [query, setQuery] = useState("");
  const [pick, setPick] = useState<string | null>(null);

  const load = useCallback(() => {
    listItems()
      .then((res) => {
        if (res.ok) setItems(res.items);
        setFailed(!res.ok);
      })
      .catch(() => setFailed(true));
  }, []);
  useEffect(load, [load]);

  // A restore can bring an entry back or add a copy, so the list is read
  // again when one ends.
  const busy = anyActive(progress).active;
  const wasBusy = useRef(false);
  useEffect(() => {
    if (wasBusy.current && !busy) load();
    wasBusy.current = busy;
  }, [busy, load]);

  const entries = useMemo(() => {
    const list = entriesOf(listedItems(items ?? []));
    const nameOf = (item: BackupItem) => entryName(item, t);
    return [...list].sort(
      (a, b) => TILE_ORDER.indexOf(a.kind) - TILE_ORDER.indexOf(b.kind) || nameOf(a).localeCompare(nameOf(b), lang)
    );
  }, [items, t, lang]);
  const restorable = entries.filter(hasBackups);

  // The page opens on the entry that most likely needs a restore: one whose
  // recommended backup is not its newest.
  const picked =
    restorable.find((item) => itemId(item) === pick) ??
    restorable.find((item) => findingsOf(openFindings, item.kind, item.key).some((a) => a.lastGood !== undefined)) ??
    restorable[0];

  const needle = query.trim().toLowerCase();
  const shown = entries.filter(
    (item) =>
      (kind === null || item.kind === kind) &&
      `${entryName(item, t)} ${t(KIND_LABEL[item.kind])}`.toLowerCase().includes(needle)
  );

  const running = Object.entries(progress).filter(([, state]) => state.active && !state.finished && state.phase === "restore");
  const runningName = (key: string) => {
    const item = (items ?? []).find((i) => itemProgressKey(i) === key);
    return item ? entryName(item, t) : key.slice(key.indexOf(":") + 1);
  };

  const nextHue = hueCounter();

  return (
    <div className={PAGE_SHELL_RESPONSIVE}>
      <PageTitle>{t("nav.restore")}</PageTitle>

      {failed ? (
        <div className="flex flex-col items-start gap-2">
          <p className="text-sm text-statusWarn">{t("backups.loadFailed")}</p>
          <Button label={t("anomaly.retry")} labelKey="anomaly.retry" onClick={load} />
        </div>
      ) : !items ? (
        <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
      ) : (
        <>
          {running.length > 0 && <RunningCard running={running} nameOf={runningName} hueIndex={nextHue()} />}

          {entries.length === 0 ? (
            <div className="flex flex-col items-center gap-3 rounded-card bg-carbon-surface p-6 text-center">
              <EmptyStateIcon icon={IconArchive} />
              <p className="text-sm text-carbon-text">{t("backups.empty")}</p>
            </div>
          ) : (
            <>
              <TileRow label={t("restorePage.tiles")}>
                {ENTRY_KINDS.filter((k) => entries.some((item) => item.kind === k))
                  .sort((a, b) => TILE_ORDER.indexOf(a) - TILE_ORDER.indexOf(b))
                  .map((k) => {
                    const count = restorable.filter((item) => item.kind === k).length;
                    return (
                      <KindTile
                        key={k}
                        on={kind === k}
                        hue={kindHue(k)}
                        glyph={<KindGlyph kind={k} />}
                        label={t(KIND_LABEL[k])}
                        sub={t("restorePage.tileCount", count)}
                        count={count}
                        onClick={() => setKind(kind === k ? null : k)}
                      />
                    );
                  })}
              </TileRow>

              <div className="flex flex-col gap-3 rounded-card bg-carbon-surface p-4">
                <label className="flex h-11 items-center gap-2.5 rounded-control bg-carbon-surface2 px-3.5 glim-field-focus-within">
                  <span className="shrink-0 text-carbon-textMuted [&_svg]:h-[18px] [&_svg]:w-[18px]">
                    <IconSearch />
                  </span>
                  <input
                    type="search"
                    aria-label={t("restorePage.search")}
                    placeholder={t("restorePage.search")}
                    value={query}
                    spellCheck={false}
                    autoComplete="off"
                    onChange={(e) => setQuery(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Escape" && query !== "") {
                        e.stopPropagation();
                        setQuery("");
                      }
                    }}
                    className="min-w-0 flex-1 bg-transparent text-base text-carbon-text outline-none placeholder:text-carbon-textMuted"
                  />
                </label>
                <ul
                  data-testid="restore-entries"
                  className="grid grid-cols-[repeat(auto-fill,minmax(230px,1fr))] gap-x-2 gap-y-0.5 max-md:max-h-[312px] max-md:grid-cols-1 max-md:overflow-y-auto"
                >
                  {shown.map((item) => {
                    const on = picked !== undefined && itemId(item) === itemId(picked);
                    return (
                      <li key={itemId(item)} className="min-w-0">
                        <button
                          type="button"
                          aria-pressed={on}
                          disabled={!hasBackups(item)}
                          onClick={() => setPick(itemId(item))}
                          style={hueVars(kindHue(item.kind)) as CSSProperties}
                          className={`glim-hue flex w-full items-center gap-3 rounded-control p-2 text-start transition-colors disabled:opacity-55 ${
                            on ? "bg-accentSoft" : "enabled:hover:bg-carbon-surface2"
                          }`}
                        >
                          <span
                            className={`flex h-9 w-9 flex-none items-center justify-center rounded-control [&_svg]:h-[18px] [&_svg]:w-[18px] ${
                              on ? "bg-accent text-accentContrast" : "bg-carbon-surface2 text-accentText"
                            }`}
                          >
                            <KindGlyph kind={item.kind} />
                          </span>
                          <span className="flex min-w-0 flex-col">
                            <span className="truncate text-sm font-semibold text-carbon-text">{entryName(item, t)}</span>
                            <span className="truncate text-xs text-carbon-textMuted">
                              {hasBackups(item)
                                ? t("restorePage.newest").replace("{when}", () => formatRecent(item.lastBackup, lang))
                                : t("restorePage.none")}
                            </span>
                          </span>
                        </button>
                      </li>
                    );
                  })}
                </ul>
                {shown.length === 0 && <p className="text-sm text-carbon-textMuted">{t("restorePage.noMatch")}</p>}
              </div>

              {picked && <PickedEntry key={itemId(picked)} item={picked} hueIndex={nextHue()} />}
            </>
          )}
        </>
      )}
    </div>
  );
}
