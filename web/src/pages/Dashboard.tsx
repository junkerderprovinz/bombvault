// The overview: one sentence on how the backups are doing, then the cards.
// The first card lists what needs the person, the others can be arranged.
import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";

import { ErrorDetailPanel } from "../components/ErrorDetailPanel";
import { RunDetailSheet } from "../components/mobile/RunDetailSheet";
import { IconCheck, IconPencil } from "../components/navGlyphs";
import { PageAction, PageActions } from "../components/PageActions";
import { PageTitle } from "../components/PageTitle";
import { Button } from "../components/Button";
import { InfoBubble } from "../components/InfoBubble";
import { IconRefresh } from "../components/glyphs";
import { getCoverage, getScheduleNext, getStatus, listRuns } from "../lib/api";
import type { CoverageReport, DomainStatus, Run, ScheduleNext } from "../lib/api";
import { inApp } from "../lib/appServers";
import { useAttention } from "../lib/attention";
import type { EntryKind } from "../lib/backupEntry";
import { cardSpan, useDashboardLayout } from "../lib/dashboardLayout";
import { foundOnServer, foundSentence } from "../lib/firstSteps";
import { useT } from "../lib/i18n";
import { useInstanceName } from "../lib/instanceName";
import { usePlatform } from "../lib/usePlatform";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { anyActive, useProgress } from "../lib/progress";
import { runRestorePath } from "../lib/restoreRequest";
import { tallyEntries } from "../lib/protectionTally";
import { failedRunsNeedingAttention } from "../lib/runFailures";
import { useAnomalySummary } from "../lib/useAnomalies";
import { useLogoHold } from "../lib/useLogoHold";
import { useIsDesktop } from "../lib/useMediaQuery";
import { AnomaliesBlock } from "./overview/AnomaliesCard";
import { OVERVIEW_CARDS, OVERVIEW_MIGRATION, PHONE_ORDER } from "./overview/cards";
import { CoverageCard } from "./overview/CoverageCard";
import { EverythingAction } from "./overview/EverythingAction";
import { FirstStepsCard } from "./overview/FirstStepsCard";
import { HealthHeatmapCard } from "./overview/HealthHeatmapCard";
import { HistoryCard, type HistoryView } from "./overview/HistoryCard";
import { NeedsCard } from "./overview/NeedsCard";
import { NextRunCard, nextBackupFireAt } from "./overview/NextRunCard";
import { HiddenTray, OverviewGrid, type GridCard } from "./overview/OverviewGrid";
import { ProtectionCard } from "./overview/ProtectionCard";
import { sectionsOf } from "./overview/protectionChecks";
import { ProtectionWindow } from "./overview/ProtectionWindow";
import { ServerHeader } from "./overview/ServerHeader";
import { SpikeCard } from "./overview/SpikeCard";
import { StatCardsRow } from "./overview/StatCardsRow";
import { headlineOf, liveRunOf, passCounted, passOf, passRuns } from "./overview/headState";
import { PassWindow } from "./overview/PassWindow";
import { StatusHead } from "./overview/StatusHead";
import { StorageCard } from "./overview/StorageCard";
import { SummaryTier } from "./overview/SummaryTier";
import { TargetsCard } from "./overview/TargetsCard";
import { useTrialRestore } from "./overview/useTrialRestore";

// The activity log reads the schedule at this rate, and two readers of one
// endpoint at different rates can show two answers.
const SCHEDULE_POLL_MS = 30000;

const CARD = Object.fromEntries(OVERVIEW_CARDS.map((card) => [card.id, card]));

const HOLDABLE = "touch-none select-none [-webkit-touch-callout:none]";

function liveRunIn(runs: Run[]): Run | undefined {
  const running = runs.filter((r) => r.status === "running");
  return running.find((r) => r.domain === "everything") ?? running[0];
}

export function Dashboard() {
  const { t, lang } = useT();
  const logoHold = useLogoHold();
  const navigate = useNavigate();
  const isDesktop = useIsDesktop();
  const progress = useProgress();
  const { needs, fresh, items: backupItems, runs, runsReady, runsFailed, refreshRuns, locations } = useAttention();
  const anomalySummary = useAnomalySummary().summary;
  const platform = usePlatform();
  const trial = useTrialRestore(t("verify.failed"));
  const app = inApp();
  const instanceName = useInstanceName();

  // The run window is opened by a row of the history, by the status head
  // while a run is under way, and by the watch of a pass started here. Once
  // the person closes it, later watch polls refresh its run but do not open
  // it again; a row or a new pass does.
  const [sheetRun, setSheetRun] = useState<Run | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);
  const sheetDismissed = useRef(false);
  // Watch polls keep reporting the same run, so only another id is a new pass.
  const lastCorrelatedRun = useRef<string | null>(null);
  // True from the press on "Back up everything now" until the watch finds the
  // pass it started. Only then may the watch take the window from another run.
  const fireArmed = useRef(false);
  const armFire = useCallback(() => {
    fireArmed.current = true;
  }, []);
  // The run the watch last spoke about. While it is the one in the window,
  // the watch's record is fresher than the polled list's.
  const [watchOwnedRun, setWatchOwnedRun] = useState<string | null>(null);
  const openRun = useCallback((run: Run) => {
    sheetDismissed.current = false;
    setWatchOwnedRun(null);
    setSheetRun(run);
    setSheetOpen(true);
  }, []);
  const handleWatchRun = useCallback((run: Run) => {
    const isNewCorrelation = lastCorrelatedRun.current !== run.id;
    // Read before it is spent: the updater below runs after this returns.
    const armed = fireArmed.current;
    lastCorrelatedRun.current = run.id;
    setWatchOwnedRun(run.id);
    if (isNewCorrelation) {
      sheetDismissed.current = false;
      fireArmed.current = false;
    }
    setSheetRun((prev) => (prev == null || prev.id === run.id || (isNewCorrelation && armed) ? run : prev));
    if (!sheetDismissed.current) setSheetOpen(true);
  }, []);
  const watchStopped = useCallback(() => {
    setWatchOwnedRun(null);
  }, []);

  const [statusDomains, setStatusDomains] = useState<DomainStatus[]>([]);
  const [statusLoading, setStatusLoading] = useState(true);
  // A refused or failed read is not an answer: an empty list must not read as
  // "nothing is switched on" or "no off-site copy".
  const [statusFailed, setStatusFailed] = useState(false);
  const [coverage, setCoverage] = useState<CoverageReport | null>(null);
  const [coverageLoading, setCoverageLoading] = useState(true);
  useEffect(() => {
    let active = true;
    const load = () => {
      getStatus()
        .then((res) => {
          if (!active) return;
          if (res.ok) setStatusDomains(res.domains ?? []);
          setStatusFailed(!res.ok);
        })
        .catch(() => {
          if (active) setStatusFailed(true);
        })
        .finally(() => {
          if (active) setStatusLoading(false);
        });
      getCoverage()
        .then((res) => {
          if (active && res.ok) setCoverage(res.coverage);
        })
        .catch(() => undefined)
        .finally(() => {
          if (active) setCoverageLoading(false);
        });
    };
    load();
    // A trial restore started here and a change in the settings both announce
    // themselves with this event.
    window.addEventListener("bv:settings-changed", load);
    return () => {
      active = false;
      window.removeEventListener("bv:settings-changed", load);
    };
  }, []);

  // The stream reports the end of a run before the next poll would. Reading
  // the runs then also brings the entries up to date.
  const running = anyActive(progress).active;
  const wasRunning = useRef(running);
  useEffect(() => {
    if (wasRunning.current && !running) refreshRuns();
    wasRunning.current = running;
  }, [running, refreshRuns]);

  // The stream counts an entry of a pass as soon as it is through. Its mark
  // waits for the run that says how it ended.
  const streamed = passOf(progress);
  const passDone = streamed?.done;
  useEffect(() => {
    if (passDone !== undefined) refreshRuns();
  }, [passDone, refreshRuns]);
  const finishedInPass = passRuns(runs);
  const pass = streamed && passCounted(streamed, finishedInPass.length);

  // The window shows the freshest record of its run. A finished record wins
  // whichever feed carries it. Before that the watch's record wins while the
  // watch speaks about this run, and the polled list's otherwise.
  const listRun = sheetRun ? runs.find((r) => r.id === sheetRun.id) : undefined;
  const displayedRun = !sheetRun
    ? null
    : sheetRun.finishedAt != null
      ? sheetRun
      : listRun?.finishedAt != null
        ? listRun
        : watchOwnedRun === sheetRun.id
          ? sheetRun
          : (listRun ?? sheetRun);

  const openLiveRun = useCallback(() => {
    const known = liveRunIn(runs);
    if (known) {
      openRun(known);
      return;
    }
    // A run that started after the last poll is not in the list yet.
    listRuns()
      .then((res) => {
        const found = res.ok ? liveRunIn(res.runs ?? []) : undefined;
        if (found) openRun(found);
      })
      .catch(() => undefined);
  }, [runs, openRun]);

  const [scheduleNext, setScheduleNext] = useState<ScheduleNext[]>([]);
  useEffect(() => {
    let active = true;
    const load = () => {
      getScheduleNext()
        .then((next) => {
          if (active) setScheduleNext(next);
        })
        .catch(() => undefined);
    };
    load();
    const id = setInterval(load, SCHEDULE_POLL_MS);
    return () => {
      active = false;
      clearInterval(id);
    };
  }, []);

  // A day picked in the calendar and a run linked as ?run=<id> both narrow
  // the log, so either one turns the history card to its log.
  const [historyView, setHistoryView] = useState<HistoryView>("runs");
  const [logDay, setLogDay] = useState<string | null>(null);
  const [logRun, setLogRun] = useState<string | null>(() => new URLSearchParams(window.location.search).get("run") || null);
  const historyRef = useRef<HTMLDivElement>(null);
  const clearLogRun = () => {
    setLogRun(null);
    const url = new URL(window.location.href);
    url.searchParams.delete("run");
    window.history.replaceState(window.history.state, "", url);
  };
  const selectLogDay = (isoDay: string) => {
    const next = logDay === isoDay ? null : isoDay;
    setLogDay(next);
    if (next === null) return;
    const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    historyRef.current?.scrollIntoView?.({ behavior: reducedMotion ? "auto" : "smooth", block: "start" });
  };
  const narrowed = logDay !== null || logRun !== null;
  const showHistoryView = (view: HistoryView) => {
    setHistoryView(view);
    if (view === "runs") {
      setLogDay(null);
      if (logRun !== null) clearLogRun();
    }
  };
  // The link that opened the page brings its run into view once.
  useEffect(() => {
    if (logRun) historyRef.current?.scrollIntoView?.({ block: "start" });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const [editing, setEditing] = useState(false);
  const [errorsOpen, setErrorsOpen] = useState(false);
  const [openSection, setOpenSection] = useState<EntryKind | null>(null);

  const { items } = backupItems;
  const tally = tallyEntries(items);
  const sections = sectionsOf(statusDomains, items, progress, trial.running);
  const liveRun = liveRunOf(progress);
  const nextFire = nextBackupFireAt(scheduleNext, statusDomains).ms;
  const ready = !statusLoading && backupItems.loaded;
  const statusMissing = statusFailed && statusDomains.length === 0;

  const { layout, reorder, setVisibleOrder, toggleHidden, stepWidth, resize, autoHeight, reset } = useDashboardLayout(
    OVERVIEW_CARDS,
    OVERVIEW_MIGRATION
  );
  const arranging = editing && isDesktop;

  const blocks: Record<string, { label: string; render: (hue: number) => ReactNode }> = {
    needs: {
      label: t("overview.needs.title"),
      render: (hue) => (
        <NeedsCard
          needs={needs}
          hueIndex={hue}
          onErrors={() => setErrorsOpen(true)}
          onItemsChanged={backupItems.reload}
        />
      ),
    },
    protection: {
      label: t("dashboard.protectionTitle"),
      render: (hue) => (
        <ProtectionCard
          t={t}
          lang={lang}
          sections={sections}
          loading={!ready}
          failed={statusMissing}
          hueIndex={hue}
          found={foundSentence(foundOnServer(backupItems.reported, platform), t, lang)}
          onOpen={(section) => setOpenSection(section.kind)}
        />
      ),
    },
    targets: { label: t("storage.title"), render: (hue) => <TargetsCard t={t} list={locations} hueIndex={hue} /> },
    history: {
      label: t("overview.history.title"),
      render: (hue) => (
        <div ref={historyRef} className="flex min-h-0 flex-1 scroll-mt-4 flex-col">
          <HistoryCard
            t={t}
            lang={lang}
            hueIndex={hue}
            runs={runs}
            loading={!runsReady}
            failed={runsFailed}
            view={narrowed ? "log" : historyView}
            onView={showHistoryView}
            logDay={logDay}
            onClearLogDay={() => setLogDay(null)}
            logRun={logRun}
            onClearLogRun={clearLogRun}
            onOpenRun={openRun}
          />
        </div>
      ),
    },
    anomalies: { label: t("anomaly.title"), render: (hue) => <AnomaliesBlock t={t} hueIndex={hue} /> },
    coverage: {
      label: t("coverage.title"),
      render: (hue) => (
        <CoverageCard
          t={t}
          coverage={coverage}
          tally={tally}
          loading={coverageLoading || (!backupItems.loaded && !backupItems.failed)}
          failed={(backupItems.failed && !backupItems.loaded) || (!coverageLoading && coverage === null)}
          hueIndex={hue}
        />
      ),
    },
    heatmap: {
      label: t("overview.calendar.title"),
      render: (hue) => <HealthHeatmapCard t={t} selectedDay={logDay} onSelectDay={selectLogDay} hueIndex={hue} />,
    },
    storage: {
      label: t("dashboard.storageTitle"),
      render: (hue) => (
        <StorageCard
          t={t}
          hueIndex={hue}
          dense={false}
          domains={statusDomains}
          statusLoading={statusLoading}
          statusFailed={statusFailed}
        />
      ),
    },
    summary: {
      label: t("dashboard.blockSummary"),
      render: (hue) => (
        <SummaryTier
          t={t}
          domains={statusDomains}
          scheduleNext={scheduleNext}
          loading={statusLoading}
          newestRun={runs[0] ?? null}
          hueIndex={hue}
        />
      ),
    },
    stats: {
      label: t("dashboard.blockStats"),
      render: (hue) => (
        <StatCardsRow
          t={t}
          hueIndex={hue}
          containers={tallyEntries(items, "container")}
          vms={tallyEntries(items, "vm")}
          all={tally}
          errors={failedRunsNeedingAttention(runs).length}
          onErrors={() => setErrorsOpen(true)}
        />
      ),
    },
    spike: { label: t("spike.title"), render: (hue) => <SpikeCard t={t} hueIndex={hue} /> },
  };

  // The history shows where the layout hides it while the log is narrowed,
  // or a link to a run would land on a page without its log. The card of
  // what needs the person has nothing to show while nothing does, and stands
  // empty only while the page is arranged.
  const shows = (id: string) =>
    (!layout.hidden.includes(id) || (id === "history" && narrowed)) && (id !== "needs" || needs.length > 0 || arranging);
  const visible = (isDesktop ? layout.order : PHONE_ORDER).filter(shows);
  const hiddenCards = layout.order.filter((id) => layout.hidden.includes(id)).map((id) => ({ id, label: blocks[id].label }));

  // The heading of each card takes the next palette position in the order
  // the cards stand on the page. The first steps stand above them all.
  let hues = fresh ? 1 : 0;
  const cards: GridCard[] = visible.map((id) => ({
    id,
    label: blocks[id].label,
    span: cardSpan(layout, CARD[id]),
    height: layout.heights[id],
    node: blocks[id].render(hues++),
  }));
  if (!isDesktop) {
    // A phone gets the next run and the repositories at a glance, right
    // under what needs the person.
    const glance: GridCard[] = [
      {
        id: "nextRun",
        label: t("dashboard.summaryNextBackup"),
        span: 6,
        node: <NextRunCard dense t={t} hueIndex={hues++} scheduleNext={scheduleNext} domains={statusDomains} loading={statusLoading} />,
      },
      {
        id: "storageGlance",
        label: t("dashboard.storageTitle"),
        span: 6,
        node: (
          <StorageCard
            dense
            t={t}
            hueIndex={hues++}
            domains={statusDomains}
            statusLoading={statusLoading}
            statusFailed={statusFailed}
          />
        ),
      },
    ];
    cards.splice(cards[0]?.id === "needs" ? 1 : 0, 0, ...glance);
  }

  const section = sections.find((s) => s.kind === openSection);

  const logoMark = (
    <span className={`glim-logo-mark flex h-6 w-6 ${logoHold.markClass}`}>
      <img ref={logoHold.markRef} src="/logo.svg" alt="" draggable={false} className="glim-logo-img block h-6 w-6 object-contain" />
    </span>
  );

  return (
    <div className={PAGE_SHELL_RESPONSIVE}>
      <div className="flex flex-col gap-6">
        {/* "BombVault" is the product's name and is not translated. A held
            finger plays the rail logo's burst. The page is already the
            overview, so a tap has nowhere to go, and the press may neither
            scroll, select the name nor open the image's long-press menu.
            Inside the app the name is the server's own, since the app holds
            several, and the arrow before it leads back to the app's list of
            them. */}
        {app ? (
          <ServerHeader
            t={t}
            name={instanceName}
            logo={
              <span {...logoHold.handlers} className={`flex shrink-0 ${HOLDABLE}`}>
                {logoMark}
              </span>
            }
          />
        ) : (
          <div {...logoHold.handlers} className={`flex w-fit items-center gap-2 md:hidden ${HOLDABLE}`}>
            {logoMark}
            <span className="text-lg font-semibold tracking-tight text-carbon-text">BombVault</span>
          </div>
        )}
        <PageTitle>{t("nav.overview")}</PageTitle>
        <StatusHead
          t={t}
          lang={lang}
          ready={ready}
          failed={statusMissing || (backupItems.failed && !backupItems.loaded)}
          headline={headlineOf({ needs: needs.length, tally, sections, notes: anomalySummary?.enabled ? anomalySummary.open.info : 0 })}
          needs={needs.length}
          tally={tally}
          sections={sections}
          nextRun={Number.isFinite(nextFire) ? Math.round(nextFire / 1000) : null}
          run={liveRun}
          pass={pass}
          passRuns={finishedInPass}
          onOpenRun={openLiveRun}
        />

        {arranging && (
          <div className="flex flex-wrap items-center justify-end gap-3">
            <InfoBubble tip={t("overview.customizeHint")} />
            <Button label={t("dashboard.resetLayout")} labelKey="dashboard.resetLayout" glyph={<IconRefresh />} onClick={reset} />
          </div>
        )}
      </div>

      {fresh && <FirstStepsCard tally={tally} t={t} lang={lang} hueIndex={0} />}

      <OverviewGrid
        cards={cards}
        editing={arranging}
        isDesktop={isDesktop}
        t={t}
        onReorder={setVisibleOrder}
        onMove={reorder}
        onHide={toggleHidden}
        onStepWidth={stepWidth}
        onResize={resize}
        onAutoHeight={autoHeight}
      />

      {arranging && <HiddenTray cards={hiddenCards} t={t} onShow={toggleHidden} />}

      {displayedRun && sheetOpen && displayedRun.domain === "everything" && pass ? (
        <PassWindow
          t={t}
          pass={pass}
          run={liveRun}
          done={finishedInPass}
          onClose={() => {
            sheetDismissed.current = true;
            setSheetOpen(false);
          }}
        />
      ) : displayedRun && (
        <RunDetailSheet
          windowed
          run={displayedRun}
          open={sheetOpen}
          onClose={() => {
            sheetDismissed.current = true;
            setSheetOpen(false);
          }}
          onRestore={() => {
            const path = runRestorePath(displayedRun);
            if (path) navigate(path, { replace: true });
          }}
        />
      )}
      {errorsOpen && <ErrorDetailPanel onClose={() => setErrorsOpen(false)} />}
      {section && <ProtectionWindow section={section} trial={trial} t={t} onClose={() => setOpenSection(null)} />}

      <PageActions>
        {isDesktop && (
          <span data-new="overview-customize" className="flex">
            <PageAction
              primary={editing}
              label={t(editing ? "dashboard.customizeDone" : "dashboard.customize")}
              glyph={editing ? <IconCheck /> : <IconPencil />}
              onClick={() => setEditing((on) => !on)}
            />
          </span>
        )}
        <EverythingAction
          t={t}
          running={liveRun !== null || pass !== null}
          hidden={arranging}
          onWatchRun={handleWatchRun}
          onArmFire={armFire}
          onWatchStopped={watchStopped}
          onOpenRun={openLiveRun}
        />
      </PageActions>
    </div>
  );
}
