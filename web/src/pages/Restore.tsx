// The Restore page. Most restores bring one entry back to one moment, so the
// page opens on the entries and the backups of the one that is picked. The
// rare ways, the emergency and the other sources, wait at the bottom.
import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type MouseEvent, type ReactNode } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { BarButton, BulkBar } from "../components/backups/BulkBar";
import { KIND_LABEL, KindGlyph, entryName, kindHue } from "../components/backups/kinds";
import { KindTile, TileRow } from "../components/backups/KindTiles";
import { useRowSelection } from "../components/backups/useRowSelection";
import { Button } from "../components/Button";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconArchive, IconForward, IconSearch, IconSelectAll } from "../components/glyphs";
import { InfoBubble } from "../components/InfoBubble";
import { InstanceWindow } from "./instances/InstanceWindow";
import { IconCheck, IconDownload, IconFolder, IconRecovery, IconRestore } from "../components/navGlyphs";
import { PageAction, PageActions } from "../components/PageActions";
import { PageTitle } from "../components/PageTitle";
import { ProgressBar } from "../components/ProgressBar";
import { AppdataWindow } from "../components/restore/AppdataWindow";
import { BackupRail } from "../components/restore/BackupRail";
import { EmergencyScene } from "../components/restore/EmergencyScene";
import { EmergencyWindow } from "../components/restore/EmergencyWindow";
import { KeyState } from "../components/restore/KeyState";
import { NotStartedNotice } from "../components/restore/NotStartedNotice";
import { OldFolderWindow } from "../components/restore/OldFolderWindow";
import { PlaceChecks } from "../components/restore/PlaceChecks";
import { RestoreManyWindow } from "../components/restore/RestoreManyWindow";
import { RestoreWindow, type RestoreAsk } from "../components/restore/RestoreWindow";
import { RestoreCancelButton } from "../components/RestoreCancelButton";
import { Rows } from "../components/storage/rows";
import { getEmergency, getSettings, listItems, type BackupItem, type EmergencyState } from "../lib/api";
import { ATTENTION_CHANGED_EVENT } from "../lib/attention";
import { hueVars } from "../lib/appearance";
import { ENTRY_KIND, ENTRY_KINDS, type EntryKind } from "../lib/backupEntry";
import { entriesOf, entryTarget, itemId, itemProgressKey, listedItems } from "../lib/backupList";
import { useT, type TranslationKey } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { anyActive, useProgress, type ProgressState } from "../lib/progress";
import { formatRecent } from "../lib/reltime";
import { joinsMany, leftOut, type LeftOut } from "../lib/restoreMany";
import { findingsOf, parseEmergencyStep, parseEmergencyWay, recommend, recommendationWhy } from "../lib/restorePlan";
import { useLongPress } from "../lib/useLongPress";
import { useOpenAnomalies } from "../lib/useAnomalies";
import { leaveLayers } from "../lib/useBackCloses";
import { useEntry } from "../lib/useEntry";
import { useTimeline } from "../lib/useTimeline";
import { Card, hueCounter } from "./settings/shared";

// The tiles stand in the order of the Backups page, with the self-backup last.
const TILE_ORDER: EntryKind[] = ["container", "vm", "flash", "files", "zfs", "config"];

function hasBackups(item: BackupItem): boolean {
  return item.lastBackup > 0;
}

const LEFT_OUT_TIP: Record<LeftOut, TranslationKey> = {
  zfs: "restoreMany.leftOut.zfs",
  config: "restoreMany.leftOut.config",
};

/**
 * EntryPick is one entry of the list. A click opens its backups below the
 * list; Ctrl+click, Shift+click or a long press pick it for a restore together
 * with others, as rows are picked on the Backups page. While entries are being
 * picked a plain click or Space picks it, and a picked entry shows a check in
 * place of its glyph.
 */
function EntryPick({
  item,
  open,
  chosen,
  selecting,
  onClick,
  onLongPress,
}: {
  item: BackupItem;
  /** Its backups stand below the list. */
  open: boolean;
  chosen: boolean;
  selecting: boolean;
  onClick: (e: MouseEvent) => void;
  onLongPress: () => void;
}) {
  const { t, lang } = useT();
  const press = useLongPress(onLongPress);
  const joins = joinsMany(item);
  const out = leftOut(item.kind);
  const name = entryName(item, t);
  const lit = open || chosen;
  return (
    <li
      style={hueVars(kindHue(item.kind)) as CSSProperties}
      className="glim-hue relative min-w-0 pointer-coarse:select-none pointer-coarse:[-webkit-touch-callout:none]"
    >
      <button
        type="button"
        role={selecting ? "checkbox" : undefined}
        aria-checked={selecting ? chosen : undefined}
        aria-expanded={selecting ? undefined : open}
        disabled={!hasBackups(item) || (selecting && !joins)}
        onClick={(e) => {
          if (!press.took()) onClick(e);
        }}
        // Shift+click would otherwise mark the text between the two rows.
        onMouseDown={(e) => {
          if (e.shiftKey) e.preventDefault();
        }}
        {...(joins ? press.handlers : {})}
        className={`flex w-full items-center gap-3 rounded-control p-2 text-start transition-colors disabled:opacity-55 ${
          lit ? "bg-accentSoft" : "enabled:hover:bg-carbon-surface2"
        }${selecting && out ? " pe-9" : ""}`}
      >
        <span
          className={`flex h-9 w-9 flex-none items-center justify-center rounded-control [&_svg]:h-[18px] [&_svg]:w-[18px] ${
            lit ? "bg-accent text-accentContrast" : "bg-carbon-surface2 text-accentText"
          }`}
        >
          {chosen ? <IconCheck /> : <KindGlyph kind={item.kind} />}
        </span>
        <span className="flex min-w-0 flex-col">
          <span className="truncate text-sm font-semibold text-carbon-text">{name}</span>
          <span className={`truncate text-xs ${lit ? "text-carbon-textSub" : "text-carbon-textMuted"}`}>
            {hasBackups(item)
              ? t("restorePage.newest").replace("{when}", () => formatRecent(item.lastBackup, lang))
              : t("restorePage.none")}
          </span>
        </span>
      </button>
      {selecting && out && hasBackups(item) && (
        <span className="absolute end-2 top-1/2 flex -translate-y-1/2">
          <InfoBubble tip={t(LEFT_OUT_TIP[out])} />
        </span>
      )}
    </li>
  );
}

/** EntryRestore reads what the window needs to know about the entry, which
 *  the list of all entries does not carry. The window opens while it reads. */
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
  if (entry || status === "loading") {
    return <RestoreWindow item={item} entry={entry} name={name} timeline={timeline} ask={ask} onClose={onClose} />;
  }
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
function PickedEntry({
  item,
  reveal,
  hueIndex,
}: {
  item: BackupItem;
  /** The entry was picked by hand, so its card comes into view. */
  reveal: boolean;
  hueIndex: number;
}) {
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

  // jsdom has no layout to scroll.
  useEffect(() => {
    if (reveal) card.current?.scrollIntoView?.({ block: "nearest", behavior: "smooth" });
  }, [reveal]);

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

/** One of the rare ways to restore, as a row that opens its window. */
function Way({
  glyph,
  name,
  sub,
  warn = false,
  onOpen,
}: {
  glyph: ReactNode;
  name: string;
  sub: string;
  /** The emergency stands out from the ways that are only rare. */
  warn?: boolean;
  onOpen: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onOpen}
      className="group flex w-full items-center gap-3.5 rounded-control px-1.5 py-3 text-start transition-colors hover:bg-carbon-surface2"
    >
      <span
        className={`flex h-9 w-9 flex-none items-center justify-center rounded-control [&_svg]:h-[18px] [&_svg]:w-[18px] ${
          warn ? "bg-statusWarnBg text-statusWarn" : "bg-carbon-surface2 text-accentText group-hover:bg-carbon-surface3"
        }`}
      >
        {glyph}
      </span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="text-sm font-semibold text-carbon-text wrap-anywhere">{name}</span>
        <span className="text-xs text-carbon-textMuted wrap-anywhere">{sub}</span>
      </span>
      <span className="flex-none text-carbon-textMuted rtl:-scale-x-100 [&_svg]:h-3.5 [&_svg]:w-3.5">
        <IconForward />
      </span>
    </button>
  );
}

export function Restore() {
  const { t, lang } = useT();
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const emergency = parseEmergencyStep(params.get("emergency"));
  const progress = useProgress();
  const { list: openFindings } = useOpenAnomalies();

  const [items, setItems] = useState<BackupItem[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [kind, setKind] = useState<EntryKind | null>(null);
  const [query, setQuery] = useState("");
  const [pick, setPick] = useState<string | null>(null);
  const [source, setSource] = useState<"oldFolder" | "appdata" | null>(null);
  const [hostMountRoot, setHostMountRoot] = useState("/host/user");
  const [keyState, setKeyState] = useState<EmergencyState | null>(null);
  const [many, setMany] = useState<BackupItem[] | null>(null);

  // The emergency window can take a key over or give it back, so the key is
  // read again each time it closes.
  const emergencyOpen = emergency !== null;
  useEffect(() => {
    if (emergencyOpen) return;
    getEmergency()
      .then((res) => {
        if (res.ok && res.emergency) setKeyState(res.emergency);
      })
      .catch(() => undefined);
  }, [emergencyOpen]);

  // The window's URL takes the place of the page's entry and the window holds
  // one above it, so closing steps back over that one and clears the URL in
  // place: the page is left with the one entry it had.
  const openEmergency = () => setParams({ emergency: "access" }, { replace: true });
  function closeEmergency(then?: () => void) {
    leaveLayers(() => {
      setParams({}, { replace: true });
      then?.();
    });
    window.dispatchEvent(new Event(ATTENTION_CHANGED_EVENT));
  }

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok && res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
      })
      .catch(() => undefined);
  }, []);

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
  const joinable = shown.filter(joinsMany).map(itemId);
  const joinableKey = joinable.join(" ");
  const selection = useRowSelection(useMemo(() => (joinableKey === "" ? [] : joinableKey.split(" ")), [joinableKey]));
  const selectStart = useRef<HTMLSpanElement>(null);
  const chosenItems = shown.filter((item) => selection.picked.has(itemId(item)));

  function clickEntry(e: MouseEvent, item: BackupItem) {
    const id = itemId(item);
    const adds = e.ctrlKey || e.metaKey || e.shiftKey;
    if (joinsMany(item) && (selection.selecting || adds)) {
      // The entry opened by hand counts as picked, as the item under the
      // cursor does in a file manager, so a Shift+click reaches from it.
      if (!selection.selecting && pick !== null && picked && joinsMany(picked) && itemId(picked) !== id) selection.set(itemId(picked), true);
      selection.click(e, id);
      return;
    }
    setPick(id);
  }

  const running = Object.entries(progress).filter(([, state]) => state.active && !state.finished && state.phase === "restore");
  const runningName = (key: string) => {
    const item = (items ?? []).find((i) => itemProgressKey(i) === key);
    return item ? entryName(item, t) : key.slice(key.indexOf(":") + 1);
  };

  const nextHue = hueCounter();

  return (
    <div className={PAGE_SHELL_RESPONSIVE}>
      <PageTitle>{t("nav.restore")}</PageTitle>

      {emergency && (
        <EmergencyWindow
          start={emergency}
          way={parseEmergencyWay(params.get("way"))}
          onClose={() => closeEmergency()}
          onPairing={() => navigate("/settings/pairing?for=emergency", { replace: true })}
          onOldFolder={() => closeEmergency(() => setSource("oldFolder"))}
        />
      )}
      {source === "oldFolder" && <OldFolderWindow hostMountRoot={hostMountRoot} onClose={() => setSource(null)} />}
      {source === "appdata" && <AppdataWindow hostMountRoot={hostMountRoot} onClose={() => setSource(null)} />}
      {many && <RestoreManyWindow items={many} onClose={() => setMany(null)} />}
      <NotStartedNotice />

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
              <TileRow label={t("restorePage.tiles")} newId="restore-tiles" chips={false}>
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
                        chip={false}
                        onClick={() => setKind(kind === k ? null : k)}
                      />
                    );
                  })}
              </TileRow>

              {selection.selecting && (
                <BulkBar
                  count={selection.picked.size}
                  allPicked={selection.allPicked}
                  onAll={selection.setAll}
                  onDone={selection.clear}
                  returnFocus={() => selectStart.current?.querySelector("button")?.focus()}
                  t={t}
                >
                  <BarButton label={t("snapshots.restore")} glyph={<IconRestore />} onClick={() => setMany(chosenItems)} wide />
                </BulkBar>
              )}

              <div className="flex flex-col gap-3 rounded-card bg-carbon-surface p-4">
                <div className="flex items-center gap-2">
                  <label className="flex h-11 min-w-0 flex-1 items-center gap-2.5 rounded-control bg-carbon-surface2 px-3.5 glim-field-focus-within">
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
                  {!selection.selecting && joinable.length > 1 && (
                    <span ref={selectStart} data-new="restore-many" className="flex">
                      <Button
                        label={t("backups.select")}
                        labelKey={null}
                        glyph={<IconSelectAll />}
                        tone="subtle"
                        hint={t("restoreMany.selectHint")}
                        onClick={() => selection.begin()}
                        className="glim-btn-key"
                      />
                    </span>
                  )}
                </div>
                <ul
                  data-testid="restore-entries"
                  className={`grid grid-cols-[repeat(auto-fill,minmax(230px,1fr))] gap-x-2 gap-y-0.5 max-md:max-h-[312px] max-md:grid-cols-1 max-md:overflow-y-auto ${
                    selection.selecting ? "select-none" : ""
                  }`}
                >
                  {shown.map((item) => {
                    const id = itemId(item);
                    return (
                      <EntryPick
                        key={id}
                        item={item}
                        open={!selection.selecting && picked !== undefined && id === itemId(picked)}
                        chosen={selection.picked.has(id)}
                        selecting={selection.selecting}
                        onClick={(e) => clickEntry(e, item)}
                        onLongPress={() => selection.begin(id)}
                      />
                    );
                  })}
                </ul>
                {shown.length === 0 && <p className="text-sm text-carbon-textMuted">{t("restorePage.noMatch")}</p>}
              </div>

              {picked && !selection.selecting && (
                <PickedEntry key={itemId(picked)} item={picked} reveal={pick !== null} hueIndex={nextHue()} />
              )}
            </>
          )}
        </>
      )}

      <div className="grid gap-6 md:grid-cols-2 md:gap-10">
        <Card title={t("restorePage.emergency")} hueIndex={nextHue()} newId="restore-emergency">
          <EmergencyScene words />
          <KeyState state={keyState} onChanged={setKeyState} />
          <Way
            glyph={<IconRecovery />}
            name={t("restorePage.emergencyWay")}
            sub={t("restorePage.emergencyWayKit")}
            warn
            onOpen={openEmergency}
          />
        </Card>
        <Card title={t("restorePage.otherSources")} hueIndex={nextHue()}>
          <Rows>
            <Way
              glyph={<IconFolder />}
              name={t("restorePage.oldFolder")}
              sub={t("restorePage.oldFolderHint")}
              onOpen={() => setSource("oldFolder")}
            />
            <Way
              glyph={<IconDownload />}
              name={t("recovery.abTitle")}
              sub={t("restorePage.appdataHint")}
              onOpen={() => setSource("appdata")}
            />
          </Rows>
        </Card>
      </div>

      <PageActions>
        <PageAction
          primary
          label={t("entry.config.startRecovery")}
          glyph={<IconRecovery />}
          onClick={openEmergency}
        />
      </PageActions>
    </div>
  );
}
