import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { listContainers, deleteBackups, forgetContainer, backupAll, restore, discover, getContainerMounts, setInclude, setIncludeAll, ApiError, type ContainerMountsResponse } from "../lib/api";
import type { AnomalyItem, Container, ItemChecks, PlacementView, Run } from "../lib/api";
import { useIsDesktop } from "../lib/useMediaQuery";
import { PageTitle } from "../components/PageTitle";
import { RunDetailSheet } from "../components/mobile/RunDetailSheet";
import { MobileListCard } from "../components/mobile/MobileListCard";
import { MobileDetailShell } from "../components/mobile/MobileDetailShell";
import { ListToolbar } from "../components/mobile/ListToolbar";
import { useLoadMore } from "../lib/useLoadMore";
import { PlacementRow } from "../components/placement/PlacementRow";
import { subscribePlacement } from "../lib/placementEvents";
import { subscribeRepos } from "../lib/useNamedRepos";
import { FilterPopover } from "../components/FilterPopover";
import { ChipFilter, loadStoredFilterKey } from "../components/ChipFilter";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { BULK_HUE } from "../lib/bulkHue";
import { useT, stateLabel } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { Advanced, useAdvanced } from "../lib/advanced";
import { BackupButton } from "../components/BackupButton";
import { fireAndWaitRun } from "../lib/backupWatch";
import { RestorePanel } from "../components/RestorePanel";
import { ItemAnomalyBadge } from "../components/ItemAnomalyBadge";
import { ItemChecksLine } from "../components/ItemChecksLine";
import { useItemChecks } from "../lib/useItemChecks";
import { ContainerChangeNotice } from "../components/ContainerChangeNotice";
import { IdleWaitLine, IdleWaitRow } from "../components/IdleWaitRow";
import { useAnomalyItems, useAnomalySummary } from "../lib/useAnomalies";
import { useRestoreRequest, type RestoreRequest } from "../lib/restoreRequest";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconContainers } from "../components/Sidebar";
import { IncludeToggle } from "../components/IncludeToggle";
import { PauseButton, PausedBadge } from "../components/SchedulePause";
import { NotInstalledHeading } from "../components/NotInstalledHeading";
import { OrphanRemoveButton } from "../components/OrphanRemoveButton";
import { RenameTakeoverRow } from "../components/RenameTakeoverRow";
import { LinkEntryPicker } from "../components/LinkEntryPicker";
import { FormerNames } from "../components/FormerNames";
import { containerTakeover } from "../lib/useTakeOver";
import { Badge, type BadgeTone } from "../components/Badge";
import { Button } from "../components/Button";
import { DatabaseDumpRow } from "../components/DatabaseDumpRow";
import { introDatabases, updateWarnKey } from "../lib/dbdump";
import { BackupCancelButton } from "../components/BackupCancelButton";
import { ProgressBar } from "../components/ProgressBar";
import { tLtr } from "../lib/ltrFragments";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useConfirm } from "../lib/useConfirm";
import { hueVars } from "../lib/appearance";
import { Selector, type SelectorItem } from "../components/Selector";
import { useToast } from "../lib/toast";

import { RuntimeRetry } from "../components/restore/RuntimeRetry";
import { isRuntimeRefusal } from "../lib/runReason";
import { HooksEditor } from "../components/containers/HooksEditor";
import { StopContainersEditor } from "../components/containers/StopContainersEditor";
import { UpdateAfterBackupRow } from "../components/containers/UpdateAfterBackupRow";
import { ExportButton } from "../components/containers/ExportButton";
import { BackupOrderPanel } from "../components/containers/BackupOrderPanel";
import { FoldersEditor } from "../components/containers/FoldersEditor";
import { ExcludesEditor } from "../components/containers/ExcludesEditor";
import { StackCard, type StackGroup } from "../components/containers/StackCard";
type T = ReturnType<typeof useT>["t"];

// Helpers

function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

// State chip — stateTone maps a raw container state to the shared Badge's
// tone; stateLabel (lib/i18n) still does the actual state->text translation.

function stateTone(state: string): BadgeTone {
  const lower = state.toLowerCase();
  if (lower === "running") return "ok";
  if (lower === "exited" || lower === "stopped") return "fail";
  return "neutral";
}

// Sort control

type SortKey = "name" | "status" | "ip";

const SORT_STORAGE_KEY = "bv-containers-sort";

function loadSortKey(): SortKey {
  const v = localStorage.getItem(SORT_STORAGE_KEY);
  if (v === "name" || v === "status" || v === "ip") return v;
  return "name";
}

/** Parse an IP like "192.168.1.5" into a numeric tuple for numeric-aware sort. */
function ipToTuple(ip: string): number[] {
  if (!ip) return [Infinity];
  const parts = ip.split(".").map(Number);
  if (parts.length !== 4 || parts.some(isNaN)) return [Infinity];
  return parts;
}

function compareIPs(a: string, b: string): number {
  if (!a && !b) return 0;
  if (!a) return 1;
  if (!b) return -1;
  const ta = ipToTuple(a);
  const tb = ipToTuple(b);
  for (let i = 0; i < 4; i++) {
    const d = (ta[i] ?? 0) - (tb[i] ?? 0);
    if (d !== 0) return d;
  }
  return 0;
}

function sortContainers(containers: Container[], key: SortKey): Container[] {
  const copy = [...containers];
  switch (key) {
    case "name":
      return copy.sort((a, b) =>
        a.name.localeCompare(b.name, undefined, { sensitivity: "base" })
      );
    case "status": {
      const rank = (c: Container) => (c.state.toLowerCase() === "running" ? 0 : 1);
      return copy.sort((a, b) => {
        const r = rank(a) - rank(b);
        if (r !== 0) return r;
        return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
      });
    }
    case "ip":
      return copy.sort((a, b) => {
        const cmp = compareIPs(a.ip, b.ip);
        if (cmp !== 0) return cmp;
        return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
      });
  }
}

const SORT_KEYS = {
  name: "sort.nameAsc",
  status: "sort.status",
  ip: "sort.ip",
} as const;

// SortControl/FilterControl/ChipFilter below are thin, page-specific adapters
// onto the shared Selector component:
// each maps this page's own domain data (a sort key, a filter key, a
// generic option list) onto Selector's generic items/active/onChange shape.
// The actual button rendering, keyboard nav (roving tabindex, arrow keys/
// Home/End, RTL) and rainbow hueing all now live in Selector itself, not
// copy-pasted here — that duplicated rendering (with zero keyboard support)
// was exactly what had drifted apart between this file and VMs.tsx's own
// near-identical copies. ChipFilter, the most generic of the three
// (parameterised over its option set), has since moved to
// components/ChipFilter.tsx to serve Containers and VMs from one copy.
//
// All three render the small horizontal selector, the well without
// `equalWidth`: one grooved control with one choice made, rather than three
// competing buttons. They hug their segments as toolbar strips, and stay on the
// small scale because they sit inside FilterPopover's 256-416px panel, where a
// 200px per-segment floor would put every option on a row of its own.
function SortControl({
  value,
  onChange,
  t,
}: {
  value: SortKey;
  onChange: (k: SortKey) => void;
  t: T;
}) {
  return (
    <div className="flex items-center gap-2 flex-wrap">
      <span className="text-xs text-carbon-textMuted">{t("sort.label")}</span>
      <Selector
        items={(["name", "status", "ip"] as SortKey[]).map((k) => ({ id: k, label: t(SORT_KEYS[k]) }))}
        label={t("sort.label")}
        inline
        select="one"
        active={value}
        onChange={(id) => onChange(id as SortKey)}
      />
    </div>
  );
}

// Installed / not-installed filter

type FilterKey = "all" | "installed" | "notInstalled";

const FILTER_STORAGE_KEY = "bv-containers-filter";

function loadFilterKey(): FilterKey {
  const v = localStorage.getItem(FILTER_STORAGE_KEY);
  if (v === "all" || v === "installed" || v === "notInstalled") return v;
  return "all";
}

function FilterControl({
  value,
  onChange,
  t,
}: {
  value: FilterKey;
  onChange: (k: FilterKey) => void;
  t: T;
}) {
  const labels: Record<FilterKey, string> = {
    all: t("containers.filterAll"),
    installed: t("containers.filterInstalled"),
    notInstalled: t("containers.notInstalled"),
  };
  return (
    <div className="flex items-center gap-2 flex-wrap">
      <span className="text-xs text-carbon-textMuted">{t("containers.filter")}</span>
      <Selector
        items={(["all", "installed", "notInstalled"] as FilterKey[]).map((k) => ({ id: k, label: labels[k] }))}
        label={t("containers.filter")}
        inline
        select="one"
        active={value}
        onChange={(id) => onChange(id as FilterKey)}
      />
    </div>
  );
}

// Schedule / backup chip filters (#41)
// Generic sibling of FilterControl: same chip look + localStorage pattern, but
// parameterised over its option set so the schedule and backup dimensions can
// each instantiate it without duplicating the markup.

type ScheduleFilterKey = "all" | "scheduled" | "notScheduled";
type BackupFilterKey = "all" | "backedUp" | "neverBackedUp";

const SCHEDULE_FILTER_STORAGE_KEY = "bv-containers-schedule-filter";
const BACKUP_FILTER_STORAGE_KEY = "bv-containers-backup-filter";
// Per browser, not per user: the note introduces the dump switch once, and
// has done its job as soon as someone has read it.
const DBDUMP_INTRO_KEY = "bv-dbdump-intro-seen";

function loadScheduleFilterKey(): ScheduleFilterKey {
  return loadStoredFilterKey(
    SCHEDULE_FILTER_STORAGE_KEY,
    ["all", "scheduled", "notScheduled"] as const,
    "all",
  );
}

function loadBackupFilterKey(): BackupFilterKey {
  return loadStoredFilterKey(
    BACKUP_FILTER_STORAGE_KEY,
    ["all", "backedUp", "neverBackedUp"] as const,
    "all",
  );
}

/** The page's four filter controls: the same block the desktop filter
 *  popover and the phone ListToolbar render, so the two toolbars cannot
 *  disagree about which dimensions the list filters by. Pure
 *  props-to-controls: the filter state and its handlers stay the page's. */
function ContainerFilterControls({
  t,
  filterKey,
  onFilterChange,
  scheduleFilter,
  onScheduleFilterChange,
  backupFilter,
  onBackupFilterChange,
  sortKey,
  onSortChange,
}: {
  t: T;
  filterKey: FilterKey;
  onFilterChange: (k: FilterKey) => void;
  scheduleFilter: ScheduleFilterKey;
  onScheduleFilterChange: (k: ScheduleFilterKey) => void;
  backupFilter: BackupFilterKey;
  onBackupFilterChange: (k: BackupFilterKey) => void;
  sortKey: SortKey;
  onSortChange: (k: SortKey) => void;
}) {
  return (
    <>
      <FilterControl value={filterKey} onChange={onFilterChange} t={t} />
      <ChipFilter<ScheduleFilterKey>
        label={t("filter.schedule")}
        value={scheduleFilter}
        onChange={onScheduleFilterChange}
        options={[
          { key: "all", label: t("filter.all") },
          { key: "scheduled", label: t("filter.scheduled") },
          { key: "notScheduled", label: t("filter.notScheduled") },
        ]}
      />
      <ChipFilter<BackupFilterKey>
        label={t("filter.backup")}
        value={backupFilter}
        onChange={onBackupFilterChange}
        options={[
          { key: "all", label: t("filter.all") },
          { key: "backedUp", label: t("filter.backedUp") },
          { key: "neverBackedUp", label: t("filter.neverBackedUp") },
        ]}
      />
      <SortControl value={sortKey} onChange={onSortChange} t={t} />
    </>
  );
}

// The mobile card list + locally stacked detail.
//
// Both components render only below the breakpoint (the page JSX-gates them
// on `!isDesktop`, and every desktop-only block above them is gated on
// `isDesktop` in return), so exactly one face of the page is ever mounted at
// a given width: no CSS-hidden second copy doing fetch work in the
// background. The stacked detail lives in this page (component-local
// openContainer state), it is not a BottomSheet and not a route.
//
// The per-card selection summary reuses the existing mounts endpoint: the
// container list payload carries no mount data, and no new endpoint is
// authorised. One lazy GET per card, cached at module scope so re-renders and
// list refetches never re-fire it; a cache entry is dropped when its detail
// closes so the card refetches a post-edit count. A failed fetch renders NO
// count at all rather than a wrong one.

/** Page-lifetime cache for the card summary/detail-meta fetches. Plain Map of
 *  promises: a failed fetch resolves to null and is not retried for the
 *  page's lifetime (a card without a count line beats a spinner forever). */
const cardMountsCache = new Map<string, Promise<ContainerMountsResponse | null>>();

function mountsMeta(name: string): Promise<ContainerMountsResponse | null> {
  let p = cardMountsCache.get(name);
  if (!p) {
    p = getContainerMounts(name)
      .then((r) => (r.ok ? r : null))
      .catch(() => null);
    cardMountsCache.set(name, p);
  }
  return p;
}

/** The ticked-include count a fresh FoldersEditor would seed from this
 *  response: selected AND reachable mount sources plus custom paths, the
 *  exact load-block rule (Containers.tsx FoldersEditor). Same derivation
 *  family as folders.handedToRestic, so the card's count line and the Save
 *  bar's can never disagree about what "n" means. */
function tickedCountFrom(r: ContainerMountsResponse): number {
  return new Set([
    ...(r.mounts ?? []).filter((m) => m.selected && m.reachable).map((m) => m.source),
    ...(r.custom ?? []).map((c) => c.path),
  ]).size;
}

/** The disclosure chips of one container card: the same block both faces of
 *  the page render: the desktop row and the phone detail share the Selector,
 *  the section set, the advanced+installed gating and the "has data" dots,
 *  so the two faces cannot drift apart. The panes the chips open stay at the
 *  call site (the desktop stacks them in the card; the phone detail keeps
 *  its FoldersEditor permanently open and only chips the rest).
 *  `showFoldersChip={false}` is the phone detail: its FoldersEditor is the
 *  detail's own body, always open, so a chip would open what is already
 *  open. */
function ContainerSectionChips({
  container,
  t,
  openSections,
  onToggle,
  trailing,
  showFoldersChip = true,
}: {
  container: Container;
  t: T;
  /** Controlled open set: one section at a time, the desktop row's rule. */
  openSections: ReadonlySet<string>;
  onToggle: (id: string) => void;
  /** Trailing inline summary; the desktop row renders the last-backup line. */
  trailing?: ReactNode;
  showFoldersChip?: boolean;
}) {
  const { advanced } = useAdvanced();
  const installed = container.installed;
  // "Has data configured" dots: the same three facts the three editors
  // render on the desktop card's chips.
  const stopHasData = (container.stopContainers ?? []).length > 0;
  const excludesHasData = (container.excludes ?? []).length > 0;
  const hooksHasData = !!(container.preHook || container.postHook);
  const configuredDot = (
    <span aria-hidden className="h-1.5 w-1.5 rounded-full bg-statusOk shrink-0" />
  );
  // Advanced+installed-only sections mirror the exact gate their panes sit
  // behind, so a chip never exists for a pane that could not render. Backups
  // (RestorePanel) is always offered: it works on a not-installed entry too.
  const sectionItems: SelectorItem[] = [];
  if (advanced && installed) {
    if (showFoldersChip) sectionItems.push({ id: "folders", label: t("folders.title") });
    sectionItems.push(
      { id: "stop", label: t("stophook.title"), icon: stopHasData ? configuredDot : undefined },
      { id: "excludes", label: t("excludes.title"), icon: excludesHasData ? configuredDot : undefined },
      { id: "hooks", label: t("hooks.title"), icon: hooksHasData ? configuredDot : undefined }
    );
  }
  sectionItems.push({ id: "backups", label: t("snapshots.title") });
  return (
    <div className="flex items-center gap-2 flex-wrap">
      <Selector
        items={sectionItems}
        label={t("containers.sectionsLabel")}
        variant="chip"
        inline
        select="many"
        active={openSections}
        buttonHeight
        onChange={onToggle}
      />
      {trailing && (
        <span className="ms-auto shrink-0 text-xs text-carbon-textMuted whitespace-nowrap">{trailing}</span>
      )}
    </div>
  );
}

function MobileContainerCard({
  container,
  t,
  index,
  nonce,
  onOpen,
}: {
  container: Container;
  t: T;
  /** Rainbow position: continues the desktop list's index space so the two
   *  presentations of one list never hand the same hue to two containers. */
  index: number;
  /** Bumped by the page when a detail closes: cards of the edited container
   *  re-read the (refreshed) cache and drop the pre-edit count. */
  nonce: number;
  onOpen: () => void;
}) {
  // null = not fetched yet / fetch failed: both render no count line (a
  // missing number is honest, a wrong one is not).
  const [ticked, setTicked] = useState<number | null>(null);
  useEffect(() => {
    let alive = true;
    void mountsMeta(container.name).then((r) => {
      if (!alive || !r) return;
      setTicked(tickedCountFrom(r));
    });
    return () => {
      alive = false;
    };
  }, [container.name, nonce]);
  return (
    <MobileListCard
      title={container.name}
      meta={
        // folders.previewPaths ("{n} paths") is the sanctioned existing key
        // for this line: the ticked include count is the mount+custom folder
        // count the editor derives.
        ticked === null ? undefined : t("folders.previewPaths", ticked)
      }
      badge={
        container.installed ? (
          <>
            {!container.self && !container.includeInSchedule && <PausedBadge explained={false} />}
            <Badge tone={stateTone(container.state)}>{stateLabel(t, container.state)}</Badge>
          </>
        ) : (
          <Badge tone="neutral">{t("containers.notInstalled")}</Badge>
        )
      }
      hueIndex={index}
      onOpen={onOpen}
    />
  );
}

function MobileContainerDetail({
  container,
  t,
  nonce,
  onBack,
  onDeleted,
  onPlacement,
  installedContainers,
  linkCandidates,
  anomaly,
  anomalyEnabled,
  checks,
  onChecksChanged,
  restoreRequest,
  onIdleWaitSaved,
  onIncludeSaved,
}: {
  container: Container;
  t: T;
  nonce: number;
  onBack: () => void;
  /** A remove or delete-backups from this detail took the entry off the
   *  list: the parent closes the detail and refreshes. */
  onDeleted: () => void;
  /** Takes the card view a placement change answered with. */
  onPlacement: (next: PlacementView) => void;
  /** The full installed set, straight through to StopContainersEditor's
   *  picker: the same array the desktop row gets. */
  installedContainers: Container[];
  /** Not-installed entries this one can take over, as on the desktop row. */
  linkCandidates: string[];
  anomaly?: AnomalyItem;
  anomalyEnabled: boolean;
  checks?: ItemChecks;
  onChecksChanged?: () => void;
  /** A link from another page asking to restore this container. */
  restoreRequest?: RestoreRequest;
  onIdleWaitSaved?: (hours: number) => void;
  onIncludeSaved?: (include: boolean) => void;
}) {
  // The host mount root for the detail's mono meta line: served by the same
  // already-cached mounts response the cards use (React escaping
  // plus LTR isolation on the path; break-all wraps long host paths).
  const [hostMountRoot, setHostMountRoot] = useState<string | null>(null);
  useEffect(() => {
    let alive = true;
    void mountsMeta(container.name).then((r) => {
      if (alive) setHostMountRoot(r?.hostMountRoot ?? null);
    });
    return () => {
      alive = false;
    };
  }, [container.name, nonce]);
  // Backup trigger + run deep-link: the trigger IS the desktop row's
  // BackupButton component (zero new trigger path; the #197 stop-ack confirm
  // inherits the ConfirmSheet below the breakpoint through useConfirm's own
  // media switch). On correlation the RunDetailSheet opens over the detail
  // with the LIVE run (component-local hosting, no route). onRun fires on
  // every poll, so sheetRun always holds the freshest record (running →
  // terminal renders truthfully); a sheet the user closed is never re-opened
  // by later polls of the same watch: the terminal outcome still toasts
  // from BackupButton.
  const progressMap = useProgress();
  const progress = progressMap[`container:${container.name}`];
  const running = anyActive(progressMap);
  const [sheetRun, setSheetRun] = useState<Run | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);
  const sheetDismissed = useRef(false);
  // The run id the watch last correlated: polls refresh the same run, so only
  // a different id is a new fire: the latch's re-arm signal. Without it, one
  // dismissal would silence every later "Back up now" press, and the user
  // would wait out the whole run for nothing but the terminal toast.
  const lastCorrelatedRun = useRef<string | null>(null);
  // Same one-section-at-a-time chips rule as the desktop row, through the
  // same shared block.
  const [openSections, setOpenSections] = useState<Set<string>>(
    () => new Set(restoreRequest ? ["backups"] : [])
  );
  function toggleSection(id: string) {
    setOpenSections((prev) => (prev.has(id) ? new Set() : new Set([id])));
  }
  const installed = container.installed;
  const self = container.self;
  const paused = installed && !self && !container.includeInSchedule;
  const aliases = container.aliases ?? [];
  const takeoverEntry = { name: container.name, displayName: container.name, api: containerTakeover };
  return (
    <MobileDetailShell
      title={container.name}
      badges={
        <>
          <ItemAnomalyBadge item={anomaly} enabled={anomalyEnabled} t={t} />
          <ContainerChangeNotice changes={container.changedSinceBackup} t={t} />
          {installed ? (
            <Badge tone={stateTone(container.state)}>{stateLabel(t, container.state)}</Badge>
          ) : (
            <Badge tone="neutral">{t("containers.notInstalled")}</Badge>
          )}
          {paused && <PausedBadge />}
        </>
      }
      subtitle={
        hostMountRoot && (
          <p dir="ltr" className="text-xs text-carbon-textMuted font-mono break-all text-start">
            {hostMountRoot}
          </p>
        )
      }
      onBack={onBack}
    >
      {/* Every control the desktop card carries, in the detail's flow. The
          corner's removal button for a not-installed entry (a deleted entry
          used to open a detail whose only way out was Back) and the self
          note come first, then the takeover pair, then the trigger. */}
      {!installed && (
        <OrphanRemoveButton
          hasBackups={container.lastBackup != null}
          deleteConfirm={t("containers.deleteBackupsConfirm")}
          removeConfirm={t("containers.removeEntryConfirm")}
          deleteBackups={() => deleteBackups(container.name)}
          removeEntry={() => forgetContainer(container.name)}
          onDone={onDeleted}
          t={t}
        />
      )}
      {self && <p className="text-xs text-carbon-textMuted">{t("containers.selfNote")}</p>}
      {installed && !self && container.renameFrom && (
        <RenameTakeoverRow
          key={container.renameFrom}
          from={container.renameFrom}
          reason={container.renameReason ?? ""}
          entry={takeoverEntry}
          onDone={onDeleted}
          t={t}
        />
      )}
      {aliases.length > 0 && (
        <FormerNames
          aliases={aliases}
          conflicts={container.aliasConflicts ?? []}
          entry={takeoverEntry}
          onDone={onDeleted}
          t={t}
        />
      )}
      {installed && !self && container.lastBackup == null && aliases.length === 0 && linkCandidates.length > 0 && (
        <LinkEntryPicker candidates={linkCandidates} entry={takeoverEntry} onDone={onDeleted} t={t} />
      )}
      {installed && !self && (
        <div className="flex items-center gap-1.5 flex-wrap">
          <PauseButton
            paused={paused}
            save={(include) => setInclude(container.name, include)}
            onSaved={onIncludeSaved}
          />
          <BackupButton
            name={container.name}
            t={t}
            running={running}
            progress={progress}
            onRunCorrelated={(run) => {
              if (lastCorrelatedRun.current !== run.id) {
                lastCorrelatedRun.current = run.id;
                sheetDismissed.current = false; // new fire re-arms the deep-link
              }
              setSheetRun(run);
              if (!sheetDismissed.current) setSheetOpen(true);
            }}
          />
        </div>
      )}
      {installed && !self && (
        <Advanced>
          <ExportButton name={container.name} t={t} />
        </Advanced>
      )}
      {/* The schedule block: the include switch, then the update-after row.
          A not-installed entry keeps the switch too, same as the desktop
          card: it stays scheduled and every run records a skip for it, and
          this switch ends that without deleting its backups. */}
      {installed && (
        <div className="flex flex-col gap-2">
          <IncludeToggle name={container.name} initial={container.includeInSchedule} onSaved={onIncludeSaved} />
          {/* Outside Advanced: the dump is on by default and changes what a
              backup does. */}
          <DatabaseDumpRow container={container} t={t} />
          <Advanced when={installed}>
            <UpdateAfterBackupRow
              name={container.name}
              initial={container.updateAfterBackup ?? false}
              lastUpdateCheck={container.lastUpdateCheck}
              lastUpdateResult={container.lastUpdateResult}
              databaseWarn={updateWarnKey(container)}
              t={t}
            />
            <IdleWaitRow name={container.name} initial={container.idleWaitHours ?? 0} onSaved={onIdleWaitSaved} />
          </Advanced>
          <IdleWaitLine name={container.name} />
        </div>
      )}
      {!self && (
        <PlacementRow
          item={{ domain: "containers", key: container.name }}
          name={container.name}
          view={container.placement}
          onView={onPlacement}
        />
      )}
      {/* The same editor the desktop row expands: one tree, one queue, zero
          forks. Keyed by container identity: switching targets can
          never inherit the previous container's mirror, browse cache or save
          queue. Advanced+installed gating mirrors the desktop row's folders
          chip exactly. */}
      <Advanced when={installed}>
        <FoldersEditor
          key={container.name}
          name={container.name}
          stack={container.stack}
          open
          t={t}
          lastBackup={container.lastBackup}
          anomaly={anomaly}
          anomalyEnabled={anomalyEnabled}
          treeViewportClassName="h-auto"
        />
      </Advanced>
      {!self && (
        <ItemChecksLine checks={checks} hasBackup={container.lastBackup != null} onChanged={onChecksChanged} startTest />
      )}
      {/* The remaining sections through the SAME chips block the desktop row
          renders (folders is the detail's own body, already open), then
          stop-a-running-backup and the live progress, both gated exactly as
          on the desktop card. */}
      <div className="flex flex-col gap-2">
        <ContainerSectionChips
          container={container}
          t={t}
          openSections={openSections}
          onToggle={toggleSection}
          showFoldersChip={false}
        />
        <Advanced when={installed}>
          <StopContainersEditor
            name={container.name}
            initial={container.stopContainers ?? []}
            installedContainers={installedContainers}
            open={openSections.has("stop")}
            t={t}
          />
          <ExcludesEditor
            name={container.name}
            initial={container.excludes ?? []}
            open={openSections.has("excludes")}
            t={t}
          />
          <HooksEditor
            name={container.name}
            initialPre={container.preHook}
            initialPost={container.postHook}
            open={openSections.has("hooks")}
            t={t}
          />
        </Advanced>
        <RestorePanel
          name={container.name}
          preselect={restoreRequest && !restoreRequest.dump ? restoreRequest.snapshot : ""}
          preselectDump={restoreRequest?.dump ? restoreRequest.snapshot : ""}
          preselectAt={restoreRequest?.at}
          aliases={aliases}
          t={t}
          installed={installed}
          open={openSections.has("backups")}
        />
      </div>
      {/* Not on a restore, which has its own control and warning about a
          half-restored target inside the Backups panel, and only while the
          run is active, so a finished run's last frame does not leave a
          button that can only answer "nothing to cancel". */}
      {progress && progress.active && progress.phase !== "restore" && (
        <div className="flex justify-end">
          <BackupCancelButton cancelKey={`container:${container.name}`} name={container.name} t={t} />
        </div>
      )}
      {progress && (
        <ProgressBar
          percent={progress.percent}
          active={progress.active}
          label={progress.phase === "restore" ? t("common.restoring") : t("common.backingUp")}
        />
      )}
      {/* The run sheet, hosted component-locally: opens on the
          useBackupWatch baseline-id correlation, closes through BottomSheet's
          three paths, and never re-opens itself after dismissal. */}
      {sheetRun && (
        <RunDetailSheet
          run={sheetRun}
          open={sheetOpen}
          onClose={() => {
            sheetDismissed.current = true;
            setSheetOpen(false);
          }}
        />
      )}
    </MobileDetailShell>
  );
}

export function ContainerRow({
  container,
  installedContainers,
  t,
  onDeleted,
  onPlacement,
  selected,
  onToggleSelect,
  linkCandidates = [],
  index,
  anomaly,
  anomalyEnabled = false,
  restoreRequest,
  checks,
  onChecksChanged,
  onIdleWaitSaved,
  onIncludeSaved,
}: {
  container: Container;
  /** Every installed container on this BombVault instance — threaded down
   *  to StopContainersEditor's own multi-select picker (icon-badge round,
   *  jdp: "eine Dropdownliste aller installierten Container"), reusing this
   *  page's own already-fetched `containers` list rather than a second
   *  `listContainers()` call inside the editor. See Containers()'s own call
   *  sites below for why this is the FULL unfiltered installed set, not the
   *  page's search/filter-narrowed `live`. */
  installedContainers: Container[];
  t: T;
  onDeleted: () => void;
  /** Takes the card view a placement change answered with. */
  onPlacement: (next: PlacementView) => void;
  selected?: boolean;
  onToggleSelect?: () => void;
  /** The not-installed entries this card can take over by hand. */
  linkCandidates?: string[];
  /** Position in the rendered list, which picks the card's rainbow hue. It is
   *  the list index rather than a hash of `container.name`; see the callers
   *  below. */
  index: number;
  anomaly?: AnomalyItem;
  anomalyEnabled?: boolean;
  /** A finding's restore link for this container: the card opens its backups
   *  and comes into view. */
  restoreRequest?: RestoreRequest;
  checks?: ItemChecks;
  onChecksChanged?: () => void;
  onIdleWaitSaved?: (hours: number) => void;
  /** Takes the include-in-schedule value the switch or the Pause button stored. */
  onIncludeSaved?: (include: boolean) => void;
}) {
  const installed = container.installed;
  const paused = installed && !container.self && !container.includeInSchedule;
  const progressMap = useProgress();
  const progress = progressMap[`container:${container.name}`];
  // "Something is running" across any domain — used to busy-guard this row's
  // own backup button (its OWN in-flight backup is handled by isPending inside).
  const running = anyActive(progressMap);

  // ONE section at a time (jdp: "Von den tabs in der container-card ... soll
  // immer nur einer angezeigt werden"). The state stays a `Set` rather than
  // becoming `string | undefined`, because the editors below each take an
  // `open` boolean off `.has(...)` and the shared chips stay `select="many"`,
  // which is what keeps a section closable by clicking its own chip again.
  const [openSections, setOpenSections] = useState<Set<string>>(
    () => new Set(restoreRequest ? ["backups"] : [])
  );
  function toggleSection(id: string) {
    setOpenSections((prev) => (prev.has(id) ? new Set() : new Set([id])));
  }
  const cardRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    // jsdom has no scrollIntoView.
    if (restoreRequest) cardRef.current?.scrollIntoView?.({ block: "start" });
  }, [restoreRequest]);

  const lastBackupText = `${t("containers.lastBackup")}: ${container.lastBackup ? formatTs(container.lastBackup) : t("containers.never")}`;

  const aliases = container.aliases ?? [];
  const takeoverEntry = { name: container.name, displayName: container.name, api: containerTakeover };

  return (
    <div
      ref={cardRef}
      id={`container-${container.name}`}
      style={{ ...hueVars(index), "--row-i": String(index) } as CSSProperties}
      // glim-hue owns the position; glim-tint washes the WHOLE card with it
      // (trap #2, design-language.md's "Rainbow" section) — without the wash
      // this card shows almost no colour at rest, since nothing else on it
      // reads --accent except the checkbox and the (usually hidden) backup
      // button. glim-active while a backup/restore is actively running on
      // THIS row: reactive mode then shows the hue without needing hover,
      // same as knightloader's TaskRow keying off task.status === 'running'.
      // glim-stagger-row (GlimStone motion-engine animation 3) reuses this
      // SAME `index` (via --row-i) the colour engine already threads through
      // every call site — see that class's own keyframe comment in index.css.
      className={`relative overflow-hidden bg-carbon-surface rounded-card p-4 flex flex-col gap-3 glim-hue glim-stagger-row ${
        progress?.active ? "glim-active" : ""
      }`}
    >
      {/* Top row */}
      <div className="flex items-start gap-3 flex-wrap">
        {/* Multi-select checkbox (installed containers only) */}
        {onToggleSelect && (
          <input
            type="checkbox"
            checked={!!selected}
            onChange={onToggleSelect}
            aria-label={t("common.selectItem").replace("{name}", container.name)}
            className="mt-1 h-4 w-4 shrink-0 cursor-pointer"
            style={{ accentColor: "var(--accent)" }}
          />
        )}
        {/* Name + image */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm min-w-0 truncate">
              {container.name}
            </span>
            <ItemAnomalyBadge item={anomaly} enabled={anomalyEnabled} t={t} />
            <ContainerChangeNotice changes={container.changedSinceBackup} t={t} />
            {installed ? (
              <Badge tone={stateTone(container.state)}>{stateLabel(t, container.state)}</Badge>
            ) : (
              <Badge tone="neutral">{t("containers.notInstalled")}</Badge>
            )}
            {paused && <PausedBadge />}
            {container.ip && (
              <span dir="ltr" className="text-xs text-carbon-textMuted font-mono text-start">{container.ip}</span>
            )}
          </div>
          {container.image && (
            <p dir="ltr" className="text-xs text-carbon-textMuted mt-0.5 truncate text-start">{container.image}</p>
          )}
        </div>

        {/* Action badges (jdp: "Jetzt sichern und Export
            sollen quadratische Badges mit Glyph sein, die sollen rechts oben
            in der Ecke sein wo jetzt Letztes Backup steht") — the row's
            top-right corner, the exact spot "Letztes Backup" used to occupy.
            That text moved OUT of this slot into the disclosure row below
            instead (`lastBackupText`, computed above and rendered next to
            the shared section-trigger row — see that row's own comment) —
            showing the same fact in both places would just duplicate it.
            BombVault's own container has no
            backup action at all (backing it up would stop itself), so this
            slot is empty for it — same self-gating the pre-existing
            `selfNote` text already had at its old position in the Actions
            row below.
              A not-installed container has nothing to back up, so the corner
            holds its removal button instead (#232). It sat at the start of the
            Actions row before, the one action on the card that was not on the
            right, and it never matched the VM card's; OrphanRemoveButton is
            the one both cards use now. */}
        <div className="ms-auto flex items-start gap-1.5 shrink-0">
          {!installed ? (
            <OrphanRemoveButton
              hasBackups={container.lastBackup != null}
              deleteConfirm={t("containers.deleteBackupsConfirm")}
              removeConfirm={t("containers.removeEntryConfirm")}
              deleteBackups={() => deleteBackups(container.name)}
              removeEntry={() => forgetContainer(container.name)}
              onDone={onDeleted}
              t={t}
            />
          ) : container.self ? (
            <span className="text-xs text-carbon-textMuted max-w-[18rem] text-end">
              {t("containers.selfNote")}
            </span>
          ) : (
            <>
              <PauseButton
                paused={paused}
                save={(include) => setInclude(container.name, include)}
                onSaved={onIncludeSaved}
              />
              <BackupButton name={container.name} t={t} onBackedUp={onDeleted} running={running} progress={progress} />
              {/* Plain tar+xml export is an advanced-only extra. */}
              <Advanced><ExportButton name={container.name} t={t} /></Advanced>
            </>
          )}
        </div>
      </div>

      {installed && !container.self && container.renameFrom && (
        <RenameTakeoverRow
          key={container.renameFrom}
          from={container.renameFrom}
          reason={container.renameReason ?? ""}
          entry={takeoverEntry}
          onDone={onDeleted}
          t={t}
        />
      )}
      {aliases.length > 0 && (
        <FormerNames
          aliases={aliases}
          conflicts={container.aliasConflicts ?? []}
          entry={takeoverEntry}
          onDone={onDeleted}
          t={t}
        />
      )}

      {/* Actions row: the include toggle with update-after-backup under it,
          both flush right. The include toggle shows on a not-installed card
          too: such an entry stays scheduled and every run records a skip for
          it, and this switch ends that without deleting its backups.
          No wrapping `<label>`: IncludeToggle renders the full ToggleRow
          itself, the same shape as UpdateAfterBackupRow. */}
      <div className="flex items-start gap-3">
        {/* The start of this row is free, so the link picker takes it instead
            of a line of its own on every card without backups. A card with
            former names is already linked, even before its first run. */}
        {installed && !container.self && container.lastBackup == null && aliases.length === 0 && linkCandidates.length > 0 && (
          <LinkEntryPicker candidates={linkCandidates} entry={takeoverEntry} onDone={onDeleted} t={t} />
        )}
        <div className="ms-auto flex flex-col items-end gap-2">
          <IncludeToggle
            name={container.name}
            initial={container.includeInSchedule}
            onSaved={onIncludeSaved}
          />
          {/* The dump row shows in both views: it is on by default and changes
              what a backup does, so it must not hide behind advanced. */}
          <DatabaseDumpRow container={container} t={t} />
          <Advanced when={installed}>
            <UpdateAfterBackupRow
              name={container.name}
              initial={container.updateAfterBackup ?? false}
              lastUpdateCheck={container.lastUpdateCheck}
              lastUpdateResult={container.lastUpdateResult}
              databaseWarn={updateWarnKey(container)}
              t={t}
            />
            <IdleWaitRow name={container.name} initial={container.idleWaitHours ?? 0} onSaved={onIdleWaitSaved} />
          </Advanced>
          <IdleWaitLine name={container.name} />
        </div>
      </div>

      {!container.self && (
        <>
          <PlacementRow
            item={{ domain: "containers", key: container.name }}
            name={container.name}
            view={container.placement}
            onView={onPlacement}
          />
          <ItemChecksLine checks={checks} hasBackup={container.lastBackup != null} onChanged={onChecksChanged} startTest />
        </>
      )}

      {/* Disclosure sections through the shared chips block (the phone
          detail renders the same one): `lastBackupText` trails the chips as
          always-visible summary data, wrapping onto its own line at narrow
          widths via the same `flex-wrap` the chips themselves need. */}
      <div className="flex flex-col gap-2">
        <ContainerSectionChips
          container={container}
          t={t}
          openSections={openSections}
          onToggle={toggleSection}
          trailing={lastBackupText}
        />

        {/* Content panes, in a fixed, predictable order regardless of which
            chip was clicked last — each editor now takes `open` as a PROP
            (from the shared `openSections` above) instead of owning its own
            internal useState; see each editor's own comment. Still gated by
            the same `<Advanced when={installed}>` the trigger row's own
            `sectionItems` construction mirrors, so these stay entirely
            unmounted (no wasted fetches/effects) whenever their chip
            couldn't have been clicked in the first place. */}
        <Advanced when={installed}>
          <FoldersEditor
            name={container.name}
            stack={container.stack}
            open={openSections.has("folders")}
            t={t}
            lastBackup={container.lastBackup}
            anomaly={anomaly}
            anomalyEnabled={anomalyEnabled}
          />
          <StopContainersEditor
            name={container.name}
            initial={container.stopContainers ?? []}
            installedContainers={installedContainers}
            open={openSections.has("stop")}
            t={t}
          />
          <ExcludesEditor
            name={container.name}
            initial={container.excludes ?? []}
            open={openSections.has("excludes")}
            t={t}
          />
          <HooksEditor
            name={container.name}
            initialPre={container.preHook}
            initialPost={container.postHook}
            open={openSections.has("hooks")}
            t={t}
          />
        </Advanced>
        <RestorePanel
          name={container.name}
          preselect={restoreRequest && !restoreRequest.dump ? restoreRequest.snapshot : ""}
          preselectDump={restoreRequest?.dump ? restoreRequest.snapshot : ""}
          preselectAt={restoreRequest?.at}
          aliases={aliases}
          t={t}
          installed={installed}
          open={openSections.has("backups")}
          isDatabase={container.dbTier !== ""}
          dbCoverage={container.dbDataCoverage}
          containerRunning={container.state === "running"}
          importStops={(container.stopContainers ?? []).filter((dep) =>
            installedContainers.some((c) => c.name === dep && c.state === "running")
          )}
        />
      </div>

      {/* Stop a running backup, gated as on the Folders page: not on a
          restore, which has its own control and warning about a half-restored
          target inside the Backups panel, and only while the run is active, so
          a finished run's last frame does not leave a button that can only
          answer "nothing to cancel". */}
      {progress && progress.active && progress.phase !== "restore" && (
        <div className="flex justify-end">
          <BackupCancelButton cancelKey={`container:${container.name}`} name={container.name} t={t} />
        </div>
      )}

      {/* Live backup/restore progress, pinned to the card's bottom edge */}
      {progress && (
        <ProgressBar
          percent={progress.percent}
          active={progress.active}
          label={progress.phase === "restore" ? t("common.restoring") : t("common.backingUp")}
        />
      )}
    </div>
  );
}

// ScheduleIncludeAllControl is the one-click header control: "Include all in
// schedule" / "Exclude all" for every installed container, refreshing the list
// so each row's include toggle reflects the new state.
function ScheduleIncludeAllControl({
  t,
  onChanged,
}: {
  t: T;
  onChanged: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // whichever of the two buttons was actually clicked — a separate nonce per
  // direction, mirroring VMs.tsx's identical ScheduleIncludeAllControl.
  const [shakeInclude, setShakeInclude] = useState(0);
  const [shakeExclude, setShakeExclude] = useState(0);

  async function run(include: boolean) {
    setBusy(true);
    try {
      const res = await setIncludeAll(include);
      if (res.ok) onChanged();
      else {
        push(res.error ?? t("settings.error"), "fail");
        (include ? setShakeInclude : setShakeExclude)((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      (include ? setShakeInclude : setShakeExclude)((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex items-center gap-2 flex-wrap">
      <Button
        key={shakeInclude}
        label={t("schedule.includeAll")}
        labelKey="schedule.includeAll"
        hueIndex={BULK_HUE.include}
        tone="accent"
        onClick={() => void run(true)}
        disabled={busy}
        className={`inline-flex items-center rounded-pill bg-accent px-3 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
          shakeInclude ? " glim-shake" : ""
        }`}
      />
      <Button
        key={shakeExclude}
        label={t("schedule.excludeAll")}
        labelKey="schedule.excludeAll"
        tone="subtle"
        onClick={() => void run(false)}
        disabled={busy}
        className={`inline-flex items-center rounded-pill px-3 py-1 text-xs font-medium text-carbon-textSub hover:text-carbon-text transition-colors disabled:opacity-50${
          shakeExclude ? " glim-shake" : ""
        }`}
      />
    </div>
  );
}

// groupStacks buckets BACKED-UP containers by their non-empty compose project and
// keeps only groups with 2+ members (a lone container isn't a "stack" worth its
// own card). A member is included when it is backed up — orphans (deleted, so
// not installed) always are; an installed one needs a recorded backup. This
// mirrors what the backend RestoreStack enumerates (stored definitions), so the
// count doesn't mislead AND a fully-wiped stack (the disaster-recovery case) still
// shows a card. Groups + members are sorted by name for a stable render.
function groupStacks(containers: Container[]): StackGroup[] {
  const byProject = new Map<string, Container[]>();
  for (const c of containers) {
    if (!c.stack) continue;
    if (c.installed && c.lastBackup == null) continue; // installed but never backed up
    const arr = byProject.get(c.stack) ?? [];
    arr.push(c);
    byProject.set(c.stack, arr);
  }
  const groups: StackGroup[] = [];
  for (const [project, members] of byProject) {
    if (members.length < 2) continue;
    members.sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: "base" }));
    groups.push({ project, members });
  }
  groups.sort((a, b) => a.project.localeCompare(b.project, undefined, { sensitivity: "base" }));
  return groups;
}

// StacksPanel renders one card per detected compose stack, above the container
// list. It renders nothing when no multi-member stack is present.
function StacksPanel({
  containers,
  onRestored,
  t,
  hueIndex,
}: {
  containers: Container[];
  onRestored: () => void;
  t: T;
  /** Rainbow position of the panel heading. The caller passes it only when
   *  a stack exists, because the panel renders nothing without one and an
   *  unrendered heading must not use up a position. */
  hueIndex?: number;
}) {
  const stacks = groupStacks(containers);
  if (stacks.length === 0) return null;
  return (
    <div className="flex flex-col gap-3">
      {/* GlimStone follow-up pass ("half-overlap card notch"): `relative`
          added directly on this <h2> — no padding wraps it, so the h2 itself
          is the right anchor for the heading Badge's new
          `position: absolute` straddle; see Badge.tsx's badgeClassName
          comment. */}
      <h2 className="relative flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={hueIndex}>
          {t("stack.title")}
        </Badge>
      </h2>
      {stacks.map((g, i) => (
        <StackCard key={g.project} group={g} onRestored={onRestored} t={t} index={i} />
      ))}
    </div>
  );
}

// Containers page

export function Containers() {
  const { t } = useT();
  const anomalies = useAnomalyItems();
  const anomalyEnabled = useAnomalySummary().summary?.enabled ?? false;
  const itemChecks = useItemChecks();
  const restoreRequest = useRestoreRequest();
  // Advanced-mode flag read directly (not just via the <Advanced> wrapper
  // below): BackupOrderPanel's own hueIndex must only be resolved via
  // `nextHue()` when the panel will ACTUALLY render — a JSX child's props
  // (including a `hueIndex={nextHue()}` expression) evaluate eagerly as
  // part of building the <Advanced> element, regardless of whether
  // <Advanced> itself goes on to render null. See VMs.tsx's identical
  // `advanced`/VMBackupOrderPanel comment for the full reasoning.
  const { advanced } = useAdvanced();
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();
  const [containers, setContainers] = useState<Container[]>([]);
  // The idle wait is saved by its row; the list keeps the hours, so a row drawn
  // again after the advanced view was off shows them.
  function idleWaitSaved(name: string, hours: number) {
    setContainers((list) => list.map((c) => (c.name === name ? { ...c, idleWaitHours: hours } : c)));
  }
  // The switch and the Pause button store the same flag, and both read it back
  // from this list, as do the scheduled count and the schedule filter.
  function includeSaved(name: string, include: boolean) {
    setContainers((list) => list.map((c) => (c.name === name ? { ...c, includeInSchedule: include } : c)));
  }
  const [loading, setLoading] = useState(true);
  // Page-level load failure — NOT migrated to a toast (GlimStone follow-up pass,
  // v8.0.0 audit note): this blocks the whole list from rendering, so it is a
  // structural "the page failed" condition the user needs to keep seeing (and
  // act on, e.g. reload), not a one-shot confirmation of a button click. Matches
  // the page-level `error` in VMs.tsx, left the same way.
  const [error, setError] = useState<string | null>(null);
  const [sortKey, setSortKey] = useState<SortKey>(loadSortKey);
  const [filterKey, setFilterKey] = useState<FilterKey>(loadFilterKey);
  const [search, setSearch] = useState("");
  const [scheduleFilter, setScheduleFilter] = useState<ScheduleFilterKey>(loadScheduleFilterKey);
  const [backupFilter, setBackupFilter] = useState<BackupFilterKey>(loadBackupFilterKey);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [bulkBusy, setBulkBusy] = useState(false);
  // The containers the last bulk restore could not recreate for a GPU or
  // runtime this host lacks, offered again without them.
  const [runtimeRefused, setRuntimeRefused] = useState<string[]>([]);
  const [discovering, setDiscovering] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Discover / "Backup selected" buttons alongside their existing toasts.
  const [shakeDiscover, setShakeDiscover] = useState(0);
  const [shakeBackupSelected, setShakeBackupSelected] = useState(0);
  const [introDismissed, setIntroDismissed] = useState(() => {
    try {
      return localStorage.getItem(DBDUMP_INTRO_KEY) === "1";
    } catch {
      return false; // without storage the note shows again, which is the harmless way to be wrong
    }
  });
  // Overall server-side batch-backup progress (independent of this browser).
  const progress = useProgress();
  const batch = progress["batch:containers"];
  const batchActive = !!batch?.active;
  // Broader "something is running" signal: any backup/restore/replication in
  // flight (not just this page's batch) disables the start buttons + shows a
  // hint, instead of relying on the 409 round-trip.
  const running = anyActive(progress);

  // The locally stacked detail's target. Component-local state on purpose:
  // the detail is not a route and not a BottomSheet; it stacks in the page
  // column while the list itself is taken off the tree, so the phone shows
  // one surface at a time and Back restores the list plus its scroll offset.
  const isDesktop = useIsDesktop();
  // Only the name is held: the detail reads its container out of the list on
  // every render, so a refetch reaches it the way it reaches a row. Holding
  // the object would freeze it at the moment of the tap, and the list is
  // replaced wholesale on every poll.
  // A restore link from another page opens its container's detail, as the
  // desktop opens that row's Backups.
  const [openName, setOpenName] = useState<string | null>(() =>
    !isDesktop && restoreRequest.item !== "" ? restoreRequest.item : null
  );
  // Scroll handoff: captured from main#bv-main when a card opens the detail,
  // restored when Back closes it (after the commit that unhides the list, so
  // the full list height exists to scroll back into).
  const listScrollRef = useRef(0);
  const restoreScrollRef = useRef(false);
  // Bumped when a detail closes: cards of the edited container re-read the
  // (cache-invalidated) mounts response so their count line reflects the edit.
  const [cardNonce, setCardNonce] = useState(0);

  // A name missing from the list was deleted or renamed away under the open
  // detail, and the detail closes with it.
  const openContainer = openName === null ? null : containers.find((c) => c.name === openName) ?? null;

  function openCard(c: Container) {
    listScrollRef.current = document.getElementById("bv-main")?.scrollTop ?? 0;
    setOpenName(c.name);
    // After the detail commits, the page reads from the top (the back row is
    // the first thing on screen).
    requestAnimationFrame(() => {
      document.getElementById("bv-main")?.scrollTo(0, 0);
    });
  }

  function closeDetail() {
    if (openName === null) return;
    // The editor session is over: drop this container's cached mounts
    // response so the card list refetches the just-saved selection, then let
    // the post-commit effect below put the scroll position back.
    cardMountsCache.delete(openName);
    restoreScrollRef.current = true;
    setCardNonce((n) => n + 1);
    setOpenName(null);
  }

  useEffect(() => {
    if (openContainer !== null || !restoreScrollRef.current) return;
    restoreScrollRef.current = false;
    document.getElementById("bv-main")?.scrollTo(0, listScrollRef.current);
  }, [openContainer]);

  // While the stacked detail is open on a phone, the list chrome (toolbar,
  // bulk bar, feature panels) hides with the list: the detail replaces the
  // list experience instead of stacking under its controls. Desktop is
  // untouched: isDesktop is always true there, so this is permanently false.
  const listChromeHidden = !isDesktop && openContainer !== null;

  // Only the newest read may land; an older answer arriving late would undo the
  // placement a card wrote while it was in flight.
  const read = useRef(0);

  function loadContainers() {
    const n = ++read.current;
    return listContainers()
      .then((res) => {
        if (n !== read.current) return;
        if (res.ok) {
          setContainers(res.containers ?? []);
          // Clear on success, which nothing in this file did. A red banner set by
          // one transient failure (the daemon restarting, a proxy 502) stayed
          // above the correctly reloaded list for as long as the page was open,
          // and it also suppressed the empty state and the "no matches" hint, so
          // the page looked broken until the user navigated away. Files.tsx
          // clears it explicitly and says why.
          setError(null);
        } else setError(t("containers.loadFailed"));
      })
      .catch(() => {
        if (n === read.current) setError(t("containers.loadFailed"));
      });
  }

  useEffect(() => {
    void loadContainers().finally(() => setLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps -- t() is only read to build a failure message; re-fetching on a language switch would be a wasted round-trip

  // loadContainers is stable for this page's lifetime; as a dependency it would
  // subscribe again on every render.
  useEffect(() => {
    const offs = [subscribeRepos(() => void loadContainers()), subscribePlacement(() => void loadContainers())];
    return () => offs.forEach((off) => off());
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  function placeContainer(name: string, next: PlacementView) {
    setContainers((prev) => prev.map((c) => (c.name === name ? { ...c, placement: next } : c)));
  }

  // Reload when the last operation finishes.
  // The fetch above ran once, on mount, and nothing refreshed it afterwards. So
  // restoring a container that was no longer installed worked, the daemon had it
  // running again, and this page went on saying "Not installed" until the user
  // navigated away or reloaded. Measured on a demo instance: GET /api/containers
  // reported state=running and installed=true while the card had been claiming
  // the opposite for over a minute. That reads as "the restore did nothing",
  // which is the one conclusion it must not invite.
  //
  // Keyed on the falling edge of `running` rather than a timer: the list only
  // changes as a result of an operation, so polling it would spend requests to
  // learn nothing, and reloading on the RISING edge would fetch the state we
  // already have.
  // anyActive returns {active, phase}; only the boolean matters here.
  const busy = running.active;
  const wasBusy = useRef(false);
  useEffect(() => {
    if (wasBusy.current && !busy) void loadContainers();
    wasBusy.current = busy;
  }, [busy]); // eslint-disable-line react-hooks/exhaustive-deps -- loadContainers is stable for this page's lifetime; adding it would re-run the effect on every render

  function handleSortChange(k: SortKey) {
    setSortKey(k);
    localStorage.setItem(SORT_STORAGE_KEY, k);
  }

  function handleFilterChange(k: FilterKey) {
    setFilterKey(k);
    localStorage.setItem(FILTER_STORAGE_KEY, k);
  }

  function handleScheduleFilterChange(k: ScheduleFilterKey) {
    setScheduleFilter(k);
    localStorage.setItem(SCHEDULE_FILTER_STORAGE_KEY, k);
  }

  function handleBackupFilterChange(k: BackupFilterKey) {
    setBackupFilter(k);
    localStorage.setItem(BACKUP_FILTER_STORAGE_KEY, k);
  }

  // Compose search (#40) + schedule/backup chips (#41) into one predicate applied
  // BEFORE sort + live/orphans split, so they combine with the installed toggle.
  //
  // useMemo'd (the shape VMs.tsx's toolbar derivation uses): the mobile block's
  // useLoadMore
  // resets its window when the items array IDENTITY changes, so every array in
  // the chain must be stable across renders that don't change the filter result
  // (lib/useLoadMore.ts's consumer contract). The desktop derivation below reads
  // the same memos: one predicate, two presentations; the two lists can never
  // disagree.
  const query = search.trim().toLowerCase();
  const filtered = useMemo(() => containers.filter((c) => {
    if (query && !(c.name.toLowerCase().includes(query) || c.image.toLowerCase().includes(query)))
      return false;
    if (scheduleFilter === "scheduled" && !c.includeInSchedule) return false;
    if (scheduleFilter === "notScheduled" && c.includeInSchedule) return false;
    if (backupFilter === "backedUp" && c.lastBackup == null) return false;
    if (backupFilter === "neverBackedUp" && c.lastBackup != null) return false;
    return true;
  }), [containers, query, scheduleFilter, backupFilter]);

  // Any contained filter off its default narrows the list. The chips persist to
  // localStorage, so a restored non-"all" value would silently shrink the list
  // behind the collapsed "Filters" button — surface it via the trigger's dot.
  const filtersActive =
    query !== "" ||
    filterKey !== "all" ||
    scheduleFilter !== "all" ||
    backupFilter !== "all";

  const sorted = useMemo(() => sortContainers(filtered, sortKey), [filtered, sortKey]);
  const live = useMemo(() => sorted.filter((c) => c.installed), [sorted]);
  const orphans = useMemo(() => sorted.filter((c) => !c.installed), [sorted]);

  // Unfiltered by the search and filters above: StopContainersEditor's picker in
  // each ContainerRow needs every installed container, not just those in view.
  const installedContainers = containers.filter((c) => c.installed);
  // Unfiltered for the same reason: a search must not hide the entry to link.
  // An entry under another entry's former name is left out, because the server
  // refuses to move it: that name's older backups belong to the other entry.
  const formerNames = new Set(containers.flatMap((c) => c.aliases ?? []));
  const notInstalledNames = containers.filter((c) => !c.installed && !formerNames.has(c.name)).map((c) => c.name);

  // Grouped from the unfiltered list, as StacksPanel does internally, so the
  // heading's `nextHue()` call can be skipped when StacksPanel renders nothing.
  // An ungated call would shift every later heading's rainbow position by one.
  const stackGroups = groupStacks(containers);

  // Sections the installed toggle actually renders below; when none show but the
  // box has containers, the filters excluded everything → show the no-match hint.
  const liveVisible = filterKey !== "notInstalled" && live.length > 0;
  const orphansVisible = filterKey !== "installed" && orphans.length > 0;
  const noMatch = containers.length > 0 && !liveVisible && !orphansVisible;

  // The mobile card list paginates the rendered card array:
  // the filtered+sorted rows in server order, with the installed toggle's
  // section gate already applied. That gate is why this is its own array rather
  // than `sorted`: windowing all of `sorted` under filterKey="installed" would
  // let the window fill with not-installed rows that never render, and hasMore
  // would then offer a "Load more" that shows nothing new, a dishonest button.
  // Slicing what renders keeps hasMore honest by construction. Identity (not
  // deep equality) is useLoadMore's reset signal, so the memo keeps the window
  // stable across unrelated renders and resets it exactly when a filter
  // (search, schedule/backup chips, or the installed toggle) changes the list.
  const mobileCards = useMemo(
    () => [
      ...(filterKey !== "notInstalled" ? live : []),
      ...(filterKey !== "installed" ? orphans : []),
    ],
    [live, orphans, filterKey]
  );
  // The array is rebuilt on every poll, which without a key would send the
  // window back to the first twenty rows every few seconds. The key carries
  // the filter state alone, so a refetch keeps the reader's place and a
  // changed filter still rewinds.
  const cardWindowKey = [query, filterKey, scheduleFilter, backupFilter, sortKey].join("\u0000");
  const { visible: visibleCards, showMore, hasMore } = useLoadMore(mobileCards, 20, cardWindowKey);
  const visibleLive = useMemo(() => visibleCards.filter((c) => c.installed), [visibleCards]);
  const visibleOrphans = useMemo(() => visibleCards.filter((c) => !c.installed), [visibleCards]);

  function toggleSelect(name: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  }

  // BombVault's own container can't be backed up (it would stop itself), so it is
  // never selectable and "select all" skips it.
  const selectable = live.filter((c) => !c.self);
  const allLiveSelected = selectable.length > 0 && selectable.every((c) => selected.has(c.name));
  function toggleSelectAll() {
    setSelected(allLiveSelected ? new Set() : new Set(selectable.map((c) => c.name)));
  }

  // Keep the selection in sync with what's actually visible+selectable: when a
  // search or filter hides a previously-selected container, drop it. This keeps
  // the bulk-bar count honest and, crucially, stops a bulk action — including the
  // DESTRUCTIVE "Restore selected" — from ever touching a row the user can no
  // longer see. Deps are exactly the inputs that change `selectable`'s membership.
  //
  // `selectable` derives from `live`, which honours search, scheduleFilter and
  // backupFilter but NOT filterKey: the installed/not-installed choice is
  // applied at render time only. So the effect had filterKey in its deps and
  // recomputed the identical set, dropping nothing. Switching to "not
  // installed" hid every installed row and hid the select-all box with them,
  // but the bulk action bar below is gated on `selected.size > 0` alone and
  // stayed. Selecting three containers, switching the filter, then pressing
  // "Restore selected" started in-place restores on rows that were not on
  // screen. filterKey now takes part in the visible set, which is what the
  // paragraph above always claimed.
  useEffect(() => {
    setSelected((prev) => {
      if (prev.size === 0) return prev;
      const visible = new Set(filterKey === "notInstalled" ? [] : selectable.map((c) => c.name));
      let changed = false;
      const next = new Set<string>();
      for (const n of prev) {
        if (visible.has(n)) next.add(n);
        else changed = true;
      }
      return changed ? next : prev;
    });
  }, [search, scheduleFilter, backupFilter, filterKey, containers]); // eslint-disable-line react-hooks/exhaustive-deps

  // Run an action over every selected container, then refresh + clear.
  // GlimStone follow-up pass (v8.0.0): the "{ok} ok, {fail} failed" summary was
  // a persistent inline note (no auto-dismiss); now a one-shot toast, same as
  // every other migrated bulk-action result. Severity follows the result: a
  // clean run is routine (success), any failure needs to actually be noticed
  // (warn) rather than blend into a quiet-mode-suppressed success.
  async function runBulk(action: (name: string) => Promise<{ ok: boolean }>) {
    setBulkBusy(true);
    let ok = 0;
    let fail = 0;
    for (const name of selected) {
      try {
        const res = await action(name);
        if (res.ok) ok++;
        else fail++;
      } catch {
        fail++;
      }
    }
    setBulkBusy(false);
    push(
      t("containers.bulkResult").replace("{ok}", String(ok)).replace("{fail}", String(fail)),
      fail > 0 ? "warn" : "success"
    );
    setSelected(new Set());
    void loadContainers();
  }

  // Back up the selected containers SERVER-SIDE: one request kicks off a batch
  // that runs on the server, so it survives this browser going away (closing the
  // tab, or stopping the container the UI runs in). Progress comes over SSE.
  async function backupSelected() {
    if (bulkBusy) return; // guard the in-flight window (button also disables)
    setBulkBusy(true);
    const names = [...selected];
    try {
      const res = await backupAll(names);
      if (!res.ok) {
        push(res.error ?? t("containers.backupStartFailed"), "fail");
        setShakeBackupSelected((n) => n + 1);
        return;
      }
      setSelected(new Set());
      push(t("containers.batchStarted"), "success");
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        push(t("containers.batchAlreadyRunning"), "warn");
      } else {
        push(e instanceof Error ? e.message : t("containers.backupStartFailed"), "fail");
        setShakeBackupSelected((n) => n + 1);
      }
    } finally {
      setBulkBusy(false);
    }
  }

  // Restores are ASYNC and share the server's single-flight guard, so the bulk
  // loop must fire one restore and WAIT for its recorded run before the next
  // (firing them in a tight loop would make every call after the first hit
  // "already running"). fireAndWaitRun handles the fire/retry/wait cycle.
  async function restoreSelected() {
    if (!(await confirm(t("containers.restoreSelectedConfirm")))) return;
    setRuntimeRefused([]);
    const refused: string[] = [];
    await runBulk(async (name) => {
      const res = await fireAndWaitRun({
        kind: "restore",
        matchRun: (r) => r.domain === "container" && r.target === name,
        start: () => restore(name, "latest", true),
        t,
      });
      if (isRuntimeRefusal(res.error)) refused.push(name);
      return res;
    });
    setRuntimeRefused(refused);
  }

  // GlimStone follow-up pass (v8.0.0): the "+N" / error note never auto-cleared
  // (it stuck around next to the Discover button until the next click); it's a
  // one-shot completion notice like every other migrated action here, so it's
  // now a toast.
  async function handleDiscover() {
    setDiscovering(true);
    try {
      const res = await discover();
      // Both paths reload the list and name what was left out. A failed pass no
      // longer means nothing happened: the named repositories are searched
      // before the domain's own, so when the domain's own is what failed, the
      // rows already found are real and already written - and this page does no
      // polling, so without the reload the operator reads a true red error over
      // an unchanged, empty list.
      if (res.skipped?.length) {
        // A pass that could not open every repository says so. "+0" and "+3" look
        // identical whether everything was read or a named repository was switched
        // off, unresolvable or on a share that did not mount, and the second case
        // is the one somebody has to act on.
        push(t("common.discoverSkipped").replace("{list}", res.skipped.join(", ")), "warn");
      }
      if (res.ok) {
        push(`+${res.discovered ?? 0}`, "success");
      } else {
        push(res.error ?? t("common.discoverFailed"), "fail");
        setShakeDiscover((n) => n + 1);
      }
      await loadContainers();
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.discoverFailed"), "fail");
      setShakeDiscover((n) => n + 1);
    } finally {
      setDiscovering(false);
    }
  }

  // hueSeq/nextHue (GlimStone follow-up pass — see Settings.tsx's own
  // identical hueSeq/nextHue comment for the full reasoning): a plain,
  // freshly-reset-every-render counter assigning 0,1,2,... to this page's
  // heading notches in the exact order the JSX below actually evaluates each
  // `hueIndex={nextHue()}` call, which for a `cond ? nextHue() : undefined`
  // or `cond && (<Badge hueIndex={nextHue()} />)` short-circuit is also
  // exactly the order those notches are, or would be, painted. Three heading
  // notches exist on this page today, in render order: BackupOrderPanel's
  // own (advanced-only, gated on `advanced` directly rather than trusting
  // <Advanced> below), StacksPanel's own (gated on `stackGroups.length > 0`,
  // since that panel returns null internally with no compose stacks present),
  // and the not-installed section's (gated on `orphans.length > 0`, naturally
  // short-circuited by the `&&` chain around it). Every call is made DIRECTLY
  // at its JSX call site as a plain number, never handed down as a function
  // for a child to call from its own body later — see SummaryTier's own
  // regression, fixed earlier this session in Dashboard.tsx, for exactly why
  // that shape breaks the ordering.
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  const recognisedDatabases = introDatabases(containers);
  const [introBefore, introAfter] = t("dbdump.introNotice", recognisedDatabases.length).split("{name}");

  function dismissIntro() {
    setIntroDismissed(true);
    try {
      localStorage.setItem(DBDUMP_INTRO_KEY, "1");
    } catch {
      /* the note comes back next time */
    }
  }

  return (
    // PAGE_SHELL (jdp live-review, "Können wir die nicht überall gleich breit
    // machen?"): was `gap-6 max-w-5xl` — 1024px wide on a 24px Card rhythm,
    // i.e. BOTH values off the app-wide standard. The 40px rhythm was settled
    // several rounds ago and rolled out to Config/Receiver/Fleet/Recovery, but
    // never reached this page, because each of those rounds only touched the
    // one page jdp had named that day. See lib/pageShell.ts for the table.
    //   Flat, not nested: this page's filter/sort toolbar is a sibling of the
    // heading rather than part of it (it renders conditionally, below the
    // loading/error/empty branches), so it takes the same 40px as everything
    // else — verified live at 1152px, where it reads as its own band between
    // heading and list rather than looking orphaned.
    //   Responsive rhythm: PAGE_SHELL_RESPONSIVE, gap-6 below the 48rem
    // breakpoint (the stacked detail + Save bar live on a phone), gap-10 at
    // and above, byte-identical to this page's settled desktop rhythm by
    // construction. Exception declared in eslint.config.js.
    <div className={PAGE_SHELL_RESPONSIVE}>
      {/* Page heading + Discover (disaster-recovery) action */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <PageTitle>{t("containers.title")}</PageTitle>
          <OffsiteIndicator domain="containers" />
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <Button
            key={shakeDiscover}
            label={t("containers.discover")}
            labelKey="containers.discover"
            hueIndex={BULK_HUE.discover}
            tone="accent"
            onClick={() => void handleDiscover()}
            disabled={discovering}
            busy={discovering}
            title={discovering ? t("containers.discovering") : tLtr(t, "containers.discoverHint")}
            className={shakeDiscover ? "glim-shake" : ""}
          />
        </div>
      </div>

      {/* The databases BombVault recognised on this box, said once. It names a
          count and points at the first card, so the switch is one click away. */}
      {!introDismissed && recognisedDatabases.length > 0 && (
        <div className="flex items-start gap-3 rounded-card bg-carbon-surface p-4 flex-wrap">
          <p className="min-w-0 flex-1 text-sm text-carbon-textSub max-md:basis-full">
            {introBefore}
            <a className="text-accentText underline hover:no-underline" href={`#container-${recognisedDatabases[0].name}`}>
              <bdi>{recognisedDatabases[0].name}</bdi>
            </a>
            {introAfter}
          </p>
          <Button
            label={t("dbdump.introDismiss")}
            labelKey="dbdump.introDismiss"
            onClick={dismissIntro}
            className="max-md:ms-auto"
          />
        </div>
      )}

      {/* Server-side batch-backup banner — visible while a "back up all" run is in
          flight, even if it was started from another tab/session. */}
      {batchActive && (
        <div className="flex items-center gap-3 rounded-card bg-carbon-surface2 px-3 py-2">
          <span
            className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
            style={{ borderColor: "var(--accent)", borderTopColor: "transparent" }}
          />
          <span className="text-xs text-carbon-textSub">
            {t("containers.batchRunning")} ({Math.round(batch?.percent ?? 0)}%)
          </span>
        </div>
      )}

      {/* The locally stacked container detail, mobile only. Renders in the
          page column above where the list sits: the back row is the first
          thing on screen after openCard scrolls to top. The list itself is
          off the tree while the detail is open (its scroll position saved and
          restored on Back).
          Desktop never evaluates this branch (`!isDesktop`). */}
      {!isDesktop && openContainer !== null && (
        <MobileContainerDetail
          container={openContainer}
          t={t}
          nonce={cardNonce}
          onBack={closeDetail}
          onDeleted={() => {
            closeDetail();
            void loadContainers();
          }}
          onPlacement={(next) => placeContainer(openContainer.name, next)}
          installedContainers={installedContainers}
          linkCandidates={notInstalledNames}
          anomaly={anomalies.find("container", openContainer.name)}
          anomalyEnabled={anomalyEnabled}
          checks={itemChecks.find("container", openContainer.name)}
          onChecksChanged={itemChecks.reload}
          restoreRequest={restoreRequest.item === openContainer.name ? restoreRequest : undefined}
          onIdleWaitSaved={(hours) => idleWaitSaved(openContainer.name, hours)}
          onIncludeSaved={(include) => includeSaved(openContainer.name, include)}
        />
      )}

      {/* Container list */}
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {error && (
        <p className="text-sm text-statusFail">{error}</p>
      )}
      {!loading && !error && containers.length === 0 && (
        <div className="bg-carbon-surface rounded-card p-6 text-center flex flex-col items-center gap-3">
          {/* No "Add" action here (unlike Receiver/Fleet/Files): this list is a
              live enumeration of what Docker actually reports, not a
              BombVault-managed list to add to. The page's own Discover button
              above (disaster-recovery re-scan) is already the relevant action
              for an empty result, so a second button here would be redundant. */}
          <EmptyStateIcon icon={IconContainers} />
          <p className="text-sm text-carbon-textMuted">
            {t("containers.emptyDocker")}
          </p>
        </div>
      )}
      {/* Backup-order panel (#119) — advanced: arrange the scheduled/batch backup
          sequence. It and the stacks panel below are standalone feature CARDS,
          so they sit above the toolbar; the toolbar, the bulk action bar and
          the list are one group and stay together (jdp, live review: "im VM-Tab
          ist die Backup-Reihenfolge über dem Filter und im Container-Tab unter
          dem Filter. Bitte überall gleich machen." — resolved in favour of the
          VMs page's arrangement). Before this, the filter sat at the very top
          and these two cards wedged themselves between it and the list it
          filters, so on a host with compose stacks the user scrolled past two
          unrelated cards to get from "Filter" to the filtered rows.
          `advanced ? nextHue() : undefined`, not a bare `nextHue()` inside
          <Advanced>: a JSX child's own props (this `hueIndex` expression
          included) evaluate eagerly as part of building the <Advanced>
          element itself, before <Advanced> ever runs its own `advanced &&
          when` check — so an unconditional `nextHue()` here would burn a
          slot every render regardless of whether the panel actually paints,
          landing every later heading's notch one index late whenever
          Advanced mode is off. Gating on the same `advanced` flag read
          directly above keeps the counter honest.
          MOVING THIS BLOCK IS SAFE FOR THE HUE COUNTER only because the
          toolbar it jumped over contains no `nextHue()` call of its own, and
          because it moved together with the stacks panel — the two kept their
          relative order, and both still precede the not-installed section's
          own notch. Re-check that if a notch is ever added to the toolbar. */}
      {!loading && !error && !listChromeHidden && (
        <Advanced>
          <BackupOrderPanel containers={containers} t={t} hueIndex={advanced ? nextHue() : undefined} />
        </Advanced>
      )}

      {/* Stacks panel — one card per detected compose stack, above the toolbar
          with the backup-order card (see its comment above).
          `stackGroups.length > 0 ? nextHue() : undefined`: StacksPanel
          returns null internally (its own groupStacks() call, computed
          again from the identical `containers` array) when there are no
          multi-member stacks — the common case on most setups — so an
          ungated `nextHue()` here would burn a slot on every render where
          the panel paints nothing, landing the not-installed heading below
          one index late. Gating on the parent's own precomputed
          `stackGroups` (see its own comment above) keeps the counter
          honest, same reasoning as BackupOrderPanel's `advanced` gate. */}
      {!loading && !error && !listChromeHidden && (
        <StacksPanel
          containers={containers}
          onRestored={() => void loadContainers()}
          t={t}
          hueIndex={stackGroups.length > 0 ? nextHue() : undefined}
        />
      )}

      {/* Controls: search + filter (installed / schedule / backup) + sort.
          Directly above the list it filters — see the backup-order card's own
          comment above for why the two feature cards moved above this row.
          Desktop face: below the breakpoint the mobile card list's ListToolbar
          carries the same state (search/filterKey/schedule/backup/sort) on the
          shared primitives, the shape VMs.tsx's toolbar uses. One state, two
          presentations; the `isDesktop` gate mounts exactly one of them per
          width. */}
      {isDesktop && !loading && !listChromeHidden && containers.length > 0 && (
        <div className="flex items-center gap-x-6 gap-y-2 flex-wrap">
          <FilterPopover label={t("filter.button")} active={filtersActive}>
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t("containers.searchPlaceholder")}
              spellCheck={false}
              autoComplete="off"
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
            />
            <ContainerFilterControls
              t={t}
              filterKey={filterKey}
              onFilterChange={handleFilterChange}
              scheduleFilter={scheduleFilter}
              onScheduleFilterChange={handleScheduleFilterChange}
              backupFilter={backupFilter}
              onBackupFilterChange={handleBackupFilterChange}
              sortKey={sortKey}
              onSortChange={handleSortChange}
            />
          </FilterPopover>
          {filterKey !== "notInstalled" && selectable.length > 0 && (
            <label className="flex items-center gap-2 text-xs text-carbon-textSub cursor-pointer">
              <input
                type="checkbox"
                checked={allLiveSelected}
                onChange={toggleSelectAll}
                className="h-4 w-4 cursor-pointer"
                style={{ accentColor: "var(--accent)" }}
              />
              {t("containers.selectAll")}
            </label>
          )}
          {live.length > 0 && (
            <div className="ms-auto">
              <ScheduleIncludeAllControl t={t} onChanged={() => void loadContainers()} />
            </div>
          )}
        </div>
      )}

      {/* Bulk action bar — appears when one or more containers are selected. */}
      {!loading && !listChromeHidden && selected.size > 0 && (
        <div className="flex items-center gap-3 flex-wrap rounded-card bg-carbon-surface2 px-3 py-2">
          <span className="text-xs text-carbon-textSub">
            {selected.size} {t("containers.selectedCount")}
          </span>
          <Button
            key={shakeBackupSelected}
        label={t("containers.backupSelected")}
            labelKey="containers.backupSelected"
            hueIndex={BULK_HUE.backup}
            tone="accent"
            onClick={() => void backupSelected()}
            disabled={bulkBusy || batchActive || running.active}
            className={`inline-flex items-center rounded-pill bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
              shakeBackupSelected ? " glim-shake" : ""
            }`}
          />
          {/* Bulk restore is advanced-only; bulk backup stays basic. */}
          <Advanced>
            <Button
              label={t("containers.restoreSelected")}
              labelKey="containers.restoreSelected"
              hueIndex={BULK_HUE.restore}
              tone="accent"
              onClick={() => void restoreSelected()}
              disabled={bulkBusy || running.active}
            />
          </Advanced>
          <Button
            label={t("containers.clearSelection")}
            labelKey="containers.clearSelection"
            tone="neutral"
            onClick={() => setSelected(new Set())}
            disabled={bulkBusy}
          />
          {running.active && (
            <span className="text-xs text-carbon-textMuted">
              {t(busyPhraseKey(running.phase))}
            </span>
          )}
        </div>
      )}

      {/* Bulk busy indicator — kept OUTSIDE the action bar so it stays visible
          after a server-side backup clears the selection (the bar unmounts
          then). The completion result itself is a toast now (see runBulk /
          backupSelected); this is only the LIVE "still working" state. */}
      {bulkBusy && (
        <p className="text-xs text-carbon-textSub">{t("containers.working")}</p>
      )}
      {runtimeRefused.length > 0 && (
        <RuntimeRetry key={runtimeRefused.join(",")} names={runtimeRefused} onDone={() => void loadContainers()} t={t} />
      )}

      {/* The desktop list: full row cards with their inline editors. JSX-gated
          on `isDesktop`: at >=48rem this is the list; below it the phone gets
          the card list further down instead, and these rows (with their
          editors' weight) never mount at all, the point of the gate, versus
          a CSS-hidden second copy. */}
      {isDesktop && !loading && filterKey !== "notInstalled" && live.length > 0 && (
        <div className="flex flex-col gap-3 glim-content-fade">
          {live.map((c, i) => (
            <ContainerRow
              key={c.name}
              container={c}
              installedContainers={installedContainers}
              t={t}
              onDeleted={() => void loadContainers()}
              onPlacement={(next) => placeContainer(c.name, next)}
              selected={selected.has(c.name)}
              onToggleSelect={c.self ? undefined : () => toggleSelect(c.name)}
              linkCandidates={notInstalledNames}
              index={i}
              anomaly={anomalies.find("container", c.name)}
              anomalyEnabled={anomalyEnabled}
              restoreRequest={restoreRequest.item === c.name ? restoreRequest : undefined}
              checks={itemChecks.find("container", c.name)}
              onChecksChanged={itemChecks.reload}
              onIdleWaitSaved={(hours) => idleWaitSaved(c.name, hours)}
              onIncludeSaved={(include) => includeSaved(c.name, include)}
            />
          ))}
        </div>
      )}

      {/* The mobile card list: summary line, toolbar, one card per installed
          container, then the not-installed section. Phone-only (`!isDesktop`
          keeps it out of the desktop DOM entirely); off the tree while the
          stacked detail is open (listChromeHidden), with the saved scroll
          position restored on Back.
          The ListToolbar binds the page's own search/filter/sort state: the
          exact state the desktop FilterPopover above reads, so there is one
          predicate with two presentations and no parallel mobile filter state
          to drift. Cards paginate through the ONE useLoadMore primitive over
          `mobileCards` (the section-gated rendered array, see its own
          comment); the not-installed section's cards append under the same
          visible window. */}
      {!isDesktop && !loading && !error && !listChromeHidden && (live.length > 0 || orphans.length > 0 || noMatch) && (
        <div className="flex flex-col gap-3 glim-content-fade">
          {live.length > 0 && (
            <p className="text-xs text-carbon-textMuted">
              {/* Derived from the same list payload the desktop protection
                  summary reads, no new endpoint, no new key. */}
              {`${live.length} ${t("nav.containers")}${
                live.some((c) => c.includeInSchedule)
                  ? ` · ${live.filter((c) => c.includeInSchedule).length} ${t("filter.scheduled")}`
                  : ""
              }`}
            </p>
          )}
          {/* Rendered whenever the list has settled (mirrors the desktop
              controls row's own `!loading` condition), including when the
              active filters currently match nothing, so a cleared search stays
              clearable (an unreachable toolbar would strand the empty state). */}
          <ListToolbar search={search} onSearch={setSearch} placeholder="containers.searchPlaceholder">
            <ContainerFilterControls
              t={t}
              filterKey={filterKey}
              onFilterChange={handleFilterChange}
              scheduleFilter={scheduleFilter}
              onScheduleFilterChange={handleScheduleFilterChange}
              backupFilter={backupFilter}
              onBackupFilterChange={handleBackupFilterChange}
              sortKey={sortKey}
              onSortChange={handleSortChange}
            />
          </ListToolbar>
          {filterKey !== "notInstalled" &&
            visibleLive.map((c, i) => (
              <MobileContainerCard key={c.name} container={c} t={t} index={i} nonce={cardNonce} onOpen={() => openCard(c)} />
            ))}
          {filterKey !== "installed" && visibleOrphans.length > 0 && (
            <div className="flex flex-col gap-3 pt-2">
              {/* The SAME heading block the desktop face renders (tip in the
                  badge's (i), not the two sentences re-printed in the open):
                  continues the page hue sequence after both panels, the same
                  render-order discipline the desktop heading follows. */}
              <NotInstalledHeading
                tip={`${t("containers.notInstalledHint")} ${t("containers.notInstalledSkipped")}`}
                hueIndex={nextHue()}
                t={t}
              />
              {visibleOrphans.map((c, i) => (
                <MobileContainerCard key={c.name} container={c} t={t} index={live.length + i} nonce={cardNonce} onOpen={() => openCard(c)} />
              ))}
            </div>
          )}
          {/* The one load-more affordance, gated on hasMore: no
              rows beyond the window, no button (hasMore is the only signal
              this may gate on); never auto-loads (no observer, no scroll
              listener, lib/useLoadMore.ts's construction-level ban). */}
          {hasMore && (
            <Button
              label={t("common.loadMore")}
              labelKey="common.loadMore"
              tone="subtle"
              onClick={showMore}
              className="w-full min-h-[2.75rem] justify-center"
              glyph={
                <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true" className="rotate-180 rtl:rotate-0">
                  <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
                </svg>
              }
            />
          )}
          {/* Zero-match honesty: an explicit empty state, never a blank
              column below the toolbar. The existing filter.noMatch copy (the
              same sentence the desktop face renders as a bare line) as a
              card, the mobile empty-state language. Exactly one presentation
              renders at any width. */}
          {noMatch && (
            <div className="rounded-card bg-carbon-surface p-4">
              <p className="text-sm text-carbon-textMuted">{t("filter.noMatch")}</p>
            </div>
          )}
        </div>
      )}

      {/* Not-installed containers that still have backups. Desktop face:
          the phone's card list renders its own not-installed section inline
          (see the mobile block above). */}
      {isDesktop && !loading && filterKey !== "installed" && orphans.length > 0 && (
        <div className="flex flex-col gap-3 glim-content-fade">
          {/* `hueIndex={nextHue()}` (GlimStone follow-up pass, proactive sweep
              of this same file per the standing colour-engine rule): threaded
              through the same page-wide `nextHue()` counter, in render order
              after BackupOrderPanel's and StacksPanel's own calls, so none of
              the three heading notches ever collide on the same rainbow
              position.
                The skip sentence connects the skipped runs in the log to
              this list: measured on jdp's box, three definitions here (OpenRGB,
              QDirStat, MinIO) had produced twelve skipped runs in the visible
              window. Since #232 it names the switch on each card that ends
              them, and it rides in the heading's (i) with the hint. */}
          <NotInstalledHeading
            tip={`${t("containers.notInstalledHint")} ${t("containers.notInstalledSkipped")}`}
            hueIndex={nextHue()}
            t={t}
          />
          {/* Continues the live list's index sequence (live.length + i)
              instead of restarting at 0. Both sections render on the same
              page at once, so a second sequence starting at 0 would hand the
              first orphan the first live row's colour, the second orphan the
              second live row's, and so on down the overlap — a colour
              repeating inside what a reader takes for one list. Offsetting
              makes the two sections one continuous sequence instead. Past the
              eighth row the 8-colour palette still cycles, here as in any
              long list (rainbowColorAt in lib/appearance.ts is
              i % palette.length); that is intended, because a repeat then
              lands a full palette apart rather than adjacent. */}
          {orphans.map((c, i) => (
            <ContainerRow
              key={c.name}
              container={c}
              installedContainers={installedContainers}
              t={t}
              onDeleted={() => void loadContainers()}
              onPlacement={(next) => placeContainer(c.name, next)}
              index={live.length + i}
              anomaly={anomalies.find("container", c.name)}
              anomalyEnabled={anomalyEnabled}
              restoreRequest={restoreRequest.item === c.name ? restoreRequest : undefined}
              checks={itemChecks.find("container", c.name)}
              onChecksChanged={itemChecks.reload}
              onIncludeSaved={(include) => includeSaved(c.name, include)}
            />
          ))}
        </div>
      )}

      {/* No container matches the active search / schedule / backup / installed
          filters. Desktop face: the mobile block's no-match card carries this
          same copy below the breakpoint, so exactly one renders per width. */}
      {isDesktop && !loading && !error && !listChromeHidden && noMatch && (
        <p className="text-sm text-carbon-textMuted">{t("filter.noMatch")}</p>
      )}
      {confirmDialog}
    </div>
  );
}
