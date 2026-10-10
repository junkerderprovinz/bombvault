import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { listRuns, getSettings, getStatus, getCoverage, downloadRecoveryKit, ackRecoveryKit, getScheduleNext, backupEverythingNow, ApiError } from "../lib/api";
import type { Run, Settings, DomainStatus, CoverageReport, ScheduleNext } from "../lib/api";
import { PageTitle } from "../components/PageTitle";
import { useT } from "../lib/i18n";
// The responsive page rhythm; gap-6 below the 48rem breakpoint, the
// PAGE_SHELL gap-10 at and above (identical on desktop by construction).
// Dashboard joins Containers/Files as a stated exception in eslint.config.js.
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { useIsDesktop } from "../lib/useMediaQuery";
import { useAdvanced } from "../lib/advanced";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { RunDetailSheet } from "../components/mobile/RunDetailSheet";
import { StickyActionBar } from "../components/mobile/StickyActionBar";
import { useBackupWatch } from "../lib/backupWatch";
import { useConfirm } from "../lib/useConfirm";
import { useToast } from "../lib/toast";
import { isFreshInstall } from "../lib/freshInstall";
import { useDashboardLayout, CustomizableBlock } from "../lib/dashboardLayout";
import { useReorder } from "../lib/dragLift";
import { ActivityLog } from "../components/ActivityLog";
import { useLoudAnomalies } from "../lib/useAnomalies";
import { Badge } from "../components/Badge";
import { IconPencil, IconBackupNow } from "../components/Sidebar";
import { IconTipButton } from "../components/IconTipButton";
import { Button } from "../components/Button";
import { CoverageCard } from "./overview/CoverageCard";
import { AnomaliesBlock } from "./overview/AnomaliesCard";
import { ProtectionCard } from "./overview/ProtectionCard";
import { RansomwareCard } from "./overview/RansomwareCard";
import { StatCardsRow } from "./overview/StatCardsRow";
import { RunsCard } from "./overview/RunsCard";
import { LastBackupsCard } from "./overview/LastBackupsCard";
import { HealthHeatmapCard } from "./overview/HealthHeatmapCard";
import { NextRunCard } from "./overview/NextRunCard";
import { SummaryTier } from "./overview/SummaryTier";
import { StorageCard } from "./overview/StorageCard";
import { SpikeCard } from "./overview/SpikeCard";

// Same cadence as ActivityLog's own runs polling (web/src/components/ActivityLog.tsx)
// so the summary tier's "Last result" cell and the Activity Log never disagree
// about which domain is currently running.
const SUMMARY_RUNS_POLL_MS = 10000;
// Same cadence ActivityLog.tsx polls /api/schedule/next at, deliberately: two
// widgets reading one endpoint at different rates can show two answers.
const SUMMARY_SCHEDULE_POLL_MS = 30000;

// ---------------------------------------------------------------------------
// Recovery-kit nag — shown only when encryption is ON and the kit has not been
// acknowledged. Prompts the user to download + safely store the encryption
// recovery kit so disaster recovery works even without a running BombVault.
// ---------------------------------------------------------------------------

function RecoveryNag({ t, suppressed }: { t: ReturnType<typeof useT>["t"]; suppressed?: boolean }) {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [dismissing, setDismissing] = useState(false);
  // Backend refusal text from the fetch-based kit download (null = no error).
  const [kitError, setKitError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    getSettings()
      .then((res) => {
        if (active && res.ok) setSettings(res.settings);
      })
      .catch(() => {/* non-fatal */});
    return () => {
      active = false;
    };
  }, []);

  if (suppressed) return null;
  if (!settings || !settings.encryptionEnabled || settings.recoveryKitAck) {
    return null;
  }

  const dismiss = () => {
    setDismissing(true);
    void ackRecoveryKit()
      .then((res) => {
        if (res.ok) setSettings({ ...settings, recoveryKitAck: true });
      })
      .catch(() => {/* non-fatal */})
      .finally(() => setDismissing(false));
  };

  return (
    <div className="rounded-card bg-statusWarnBg px-4 py-3 flex flex-col gap-2">
      {/* Task 5 (rule 11): deliberately NOT a heading badge, and the one
          outermost <h2> on this page that isn't — see Badge.tsx's file header
          for the shared reasoning. Short version: this panel's own surface is
          already a filled status wash (bg-statusWarnBg), so a "filled" badge
          on top of it has nothing to fill against. Measured on the live page,
          badge-fill vs. this panel: accent-soft 1.06:1 light / 1.39:1 dark,
          warn-strong 1.00:1 light (the two warn-bg tokens share one value in
          light mode) / 1.11:1 dark. Either way the fill reads as invisible,
          so the badge would look like plain text wearing extra padding while
          also throwing away the text-statusWarn colour that currently carries
          the alert's meaning (8.62:1 against the panel). Rule 11's filled
          badge presumes a neutral card surface underneath; this alert isn't
          one. Revisit only if a genuine "badge on a status surface" token
          pair ever exists. */}
      <h2 className="text-sm font-semibold text-statusWarn">
        {t("recovery.nagTitle")}
      </h2>
      <p className="text-xs text-statusWarn leading-relaxed">
        {t("recovery.nagBody")}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        {/* Fetch-based download (mirrors Settings): a raw <a download> would save
            the 403 refusal body as the .md file when auth is off — the backend
            fails closed for this export, so surface its message instead (#A1). */}
        <button
          type="button"
          onClick={() => void downloadRecoveryKit().then(setKitError)}
          className="rounded-pill bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors"
        >
          {t("recovery.download")}
        </button>
        {kitError && (
          <span className="text-xs text-statusFail wrap-break-word">✗ {kitError}</span>
        )}
        <Button
          label={t("recovery.stored")}
          labelKey="recovery.stored"
          tone="neutral"
          onClick={dismiss}
          disabled={dismissing}
        />
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Fresh-install nudge — on a brand-new or rebuilt install (no domain has ever
// backed up successfully) point the user at the guided Recovery tab to recover
// their existing backups. Dismissible; the dismissal persists in localStorage.
// The fresh signal is derived purely from the shared /api/status domains the
// dashboard already fetched — no extra round-trip, and nothing is fetched or
// computed once dismissed.
// ---------------------------------------------------------------------------

const RECOVERY_NUDGE_DISMISSED = "bombvault.recoveryNudgeDismissed";

function FreshInstallNudge({
  t,
  domains,
  loading,
  dismissed,
  onDismiss,
}: {
  t: ReturnType<typeof useT>["t"];
  domains: DomainStatus[];
  loading: boolean;
  dismissed: boolean;
  onDismiss: () => void;
}) {
  // Gate: do nothing (and read nothing) once dismissed or while status is still
  // loading. Only then is the fresh predicate evaluated against shared data.
  if (dismissed || loading) return null;
  if (!isFreshInstall(domains)) return null;

  return (
    <div className="bg-carbon-surface rounded-card p-5 flex items-center gap-4">
      <div className="flex-1 flex flex-col gap-1.5">
        <p className="text-sm text-carbon-text">{t("recovery.freshNudge")}</p>
        {/* Task 5 (rule 13): was a plain underline-on-hover text link, styled
            with the raw accent colour and no fill at all. This card's own one
            call-to-action functions as a primary action (rule 3 allows
            exactly one solid-accent primary action per page/card), so it
            takes the SAME filled rounded-pill/bg-accent/text-accentContrast
            treatment every other primary button in this app already uses
            (e.g. Config.tsx's Save button) — matching an established idiom
            rather than routing through Badge's tone system, which has no
            "primary CTA" tone of its own and isn't the right place to invent
            one for a single call site. Under 48rem the CTA also takes the
            app's link-as-control height (the same value the Config/Flash
            destinations-gate links carry), staying a real touch target on a
            phone; desktop keeps the engine's 32px control height. Still not
            a Button: that renders a plain <button>, which cannot navigate. */}
        <Link
          to="/recovery"
          className="self-start inline-flex items-center gap-1 rounded-pill bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity max-md:min-h-[2.75rem]"
        >
          {t("recovery.freshNudgeCta")} <span className="inline-block rtl:-scale-x-100">→</span>
        </Link>
      </div>
      {/* A chip normally rides inside a host pill whose row carries the touch
          floor. This one is the card's only close control, loose in a flex
          row, so below 48rem an ::after owned by the button widens its 18px
          engine box by 14px on each side (46px in total). A padded wrapper
          would only be dead zone. */}
      <Button
        label={t("common.close")}
        labelKey="common.close"
        variant="chip"
        onClick={onDismiss}
        className="shrink-0 max-md:relative max-md:after:absolute max-md:after:-inset-3.5 max-md:after:content-['']"
      />
    </div>
  );
}

// Mobile Home blocks; the glanceable phone surface.
//
// Below the 48rem breakpoint the desktop customizable block grid is replaced
// by these blocks in a fixed order: identity (the page header above), next
// run (NextRunCard dense), recent runs (RunsCard dense), repo health
// (StorageCard dense), the anomalies card, the three safety cards
// (RansomwareCard, advanced view only as on the desktop, then ProtectionCard
// and CoverageCard, which stays under the protection card its hint points
// to), the self-contained ActivityLog card, and the thumb-zone trigger that
// joins the page column's last child (the StickyActionBar below). While a
// critical or warning finding is open the anomalies card moves to the top,
// because that is what a phone is opened for. The anomalies and safety cards
// fit a phone column as they are and render their desktop face; the other
// blocks are the desktop block of the same name at its second
// density; one component per card, two faces (`dense`), so a
// derivation or a fix lands on both faces in one place instead of drifting
// between a desktop card and a private per-density phone copy (the previous
// trio of phone-only cards drifted on exactly this seam). The phone density
// contract: cards p-4, gap-4 between blocks, gap-2 inside a card, filled
// section Badges (MobileSectionLabel) as headings.
// Every consumer reads state the page has already fetched (runs,
// scheduleNext, statusDomains, coverage) or the same endpoint a desktop card
// already reads; zero new endpoints.
//
// Phone hues are static literals (0 to 3 down the column, shifted by one when
// the anomalies card leads, 4 to 6 on the safety cards, 7 on the activity-log
// card, and per-row positions inside a block's list), unlike the
// desktop grid's running `nextHue()` counter: the phone order is fixed (not
// user-customizable), so a static position is honest, and the phone faces
// never render inside the desktop counter's pass; the desktop counter must
// not observe a phone-only draw.
//
// Mount discipline: the blocks are JSX-gated on `!isDesktop` (jsdom's
// matchMedia answers desktop, so these surfaces render only in real mobile
// browsers/e2e), and the desktop grid is JSX-gated on `isDesktop` in return;
// a CSS-hidden grid would stay mounted on the phone and its cards
// (LastBackupsCard, the heatmap, StorageCard) would keep fetching behind the
// user's back, doubling every phone load's round-trips. With both faces
// JSX-gated exactly one surface is ever alive, and at the 48rem boundary the
// two switches agree.

// Phone thumb-zone trigger; the surface's one solid-accent control, and the
// owner of the everything pass's fire-and-watch cycle. Mounted at every
// width: unmounting it at 48rem killed the live watch the moment a phone
// rotated to a landscape at or above the breakpoint (iPhone 15 is 852px,
// Pixel 8 is 892px), and rotating back mounted a fresh hook in the idle
// phase: the bar read ready while the pass was still running on the
// server, with no sheet, no progress and no toast. The desktop cost is
// bounded by the child itself: the bar is not rendered above the
// breakpoint, the confirm dialog exists only while a confirm is pending,
// and what remains mounted is one idle
// progress subscription in a leaf component, not a page re-render per SSE
// frame. Phone behavior is unchanged from when the watch lived on the page
// component: same watch args, confirm-first press, deep-link into the run
// sheet via the page's latch, and terminal toasts (at either width, since a
// pass fired on a phone also reports its outcome after the rotation).
function PhoneEverythingTrigger({
  t,
  onWatchRun,
  onArmFire,
  onWatchStopped,
}: {
  t: ReturnType<typeof useT>["t"];
  /** Called on every watch poll with the correlated run; the page's
   *  sheet-latch logic decides replace/keep/open (see handleWatchRun). */
  onWatchRun: (run: Run) => void;
  /** Called the moment the user confirms a fire; arms the page's deep-link
   *  latch so the correlation of this pass may steal the sheet. */
  onArmFire: () => void;
  /** Called when the fire-and-watch chain ends (the pending phase clearing,
   *  by terminal outcome or timeout). The page releases the watch's display
   *  ownership at that moment (watchOwnedRun): only a live chain's records
   *  are fresher than the polled list. */
  onWatchStopped: () => void;
}) {
  // Thumb-zone trigger watch (BackupButton's semantics verbatim). The
  // everything pass is async on the server, so the press only starts it
  // ({ok:true,started:true} is never read for the outcome) and the watch
  // resolves from the recorded run; baseline ids seeded from listRuns before
  // firing (never a client clock), then polled until the pass's run turns
  // terminal. `progressKey: ""` is the honest key: the everything parent run
  // publishes no SSE entry of its own (its per-domain children do; see
  // RunDetailSheet's progressKeyFor), so the watch resolves via the run-poll
  // belt exactly like a target whose key never appears.
  const startEverything = useCallback(async () => {
    // The everything pass is single-flight server-side: a second press while
    // one is already running surfaces as HTTP 409. The raw HTTP status text
    // must never reach a toast; map it to the shared translated "already
    // running" copy (Settings.tsx's runNow mapping) and return it as the
    // start failure the hook already displays.
    try {
      return await backupEverythingNow();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        return { ok: false, error: t("settings.everythingAlreadyRunning") };
      }
      throw err;
    }
  }, [t]);
  const { state, fire, isPending } = useBackupWatch({
    progressKey: "",
    start: startEverything,
    matchRun: (r) => r.domain === "everything",
    onRun: onWatchRun,
  });

  // Terminal outcomes toast per BackupButton's contract ("failed action toasts
  // And shakes its button"); success mirrors its snapshot-id form, falling back
  // to plain Done when the parent run carries no snapshot (the everything pass
  // aggregates its domains, so the parent snapshot is often empty; the
  // container-specific configOnly fallback does not apply here). cancelled and
  // skipped stay silent like BackupButton's cancelled arm: the deep-linked
  // sheet is already showing the run's own record.
  const { push } = useToast();
  const [shake, setShake] = useState(0);
  const seenPhase = useRef(state.phase);
  useEffect(() => {
    if (state.phase === seenPhase.current) return;
    const was = seenPhase.current;
    seenPhase.current = state.phase;
    // Leaving "pending" is the chain's end (terminal outcome, timeout, or a
    // reset). The page hears it before the toast arms: ownership and
    // outcome travel together.
    if (was === "pending" && state.phase !== "pending") onWatchStopped();
    if (state.phase === "success") {
      push(
        state.snapshotId ? `${t("common.done")} · ${state.snapshotId.slice(0, 8)}` : t("common.done"),
        "success"
      );
    } else if (state.phase === "error") {
      push(state.message, "fail");
      setShake((n) => n + 1);
    }
  }, [state, push, t, onWatchStopped]);

  // The question stands between the press and the POST; useConfirm answers it
  // in a sheet below the breakpoint and in the card above it. confirmKey makes
  // the commit button name the outcome with the trigger's own words.
  const isDesktop = useIsDesktop();
  const { confirm, confirmDialog } = useConfirm();
  const confirmThenFire = useCallback(async () => {
    const ok = await confirm(t("home.newBackupConfirm"), {
      confirmKey: "settings.everythingTitle",
    });
    if (!ok) return;
    onArmFire();
    await fire();
  }, [confirm, fire, t, onArmFire]);

  // StickyActionBar as the last direct child of the page column; sticky
  // resolves against main#bv-main, so nothing may wrap it. Never a fab, never
  // position:fixed. While the pass runs the button shows its busy spinner and
  // is disabled; a failed start also shakes (the glim-shake key remount,
  // Containers' Save-bar pattern).
  //
  // The bar is not rendered above the breakpoint, rather than hidden by CSS:
  // the desktop tree carries no phone chrome at all, which is the contract
  // Layout.tsx states for the Sidebar and the bar alike. The component itself
  // stays mounted at every width, because that is what keeps the watch alive
  // across a rotation; a pending confirmation also survives the flip, since
  // useConfirm swaps the sheet for the card without dropping the promise.
  return (
    <>
      {!isDesktop && (
        <StickyActionBar>
          <Button
            key={shake}
            label={t("settings.everythingTitle")}
            labelKey="settings.everythingTitle"
            glyph={<IconBackupNow />}
            tone="accent"
            keepLabel
            disabled={isPending}
            busy={isPending}
            onClick={() => void confirmThenFire()}
            className={`w-full min-h-[2.75rem] justify-center${shake ? " glim-shake" : ""}`}
          />
        </StickyActionBar>
      )}
      {confirmDialog}
    </>
  );
}

// ---------------------------------------------------------------------------
// Dashboard page
// ---------------------------------------------------------------------------

export function Dashboard() {
  const { t } = useT();
  const { advanced } = useAdvanced();
  // The phone surface switch. Below the 48rem breakpoint the page renders the
  // glanceable Home blocks instead of the desktop customizable grid;
  // each face JSX-gated (see the Mobile Home blocks banner above for why a
  // CSS-hidden grid would double the phone's fetches). jsdom's matchMedia
  // answers desktop, so the mobile blocks stay e2e-only and every existing
  // dom test sees the desktop page.
  const isDesktop = useIsDesktop();
  const anomaliesLead = useLoudAnomalies().count > 0;
  const lead = anomaliesLead ? 1 : 0;

  // Component-local run-sheet host (the Containers.tsx contract): the
  // recent-run rows open the shared RunDetailSheet for their own record here;
  // no route (router.tsx frozen). The dismissal latch exists for the
  // thumb-zone backup watch (the phone trigger below): once the user closes a
  // sheet, later onRun polls refresh sheetRun but never re-open it; an
  // explicit row tap or a new fire always re-arms.
  const [sheetRun, setSheetRun] = useState<Run | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);
  const sheetDismissed = useRef(false);
  // The run id the watch last correlated: polls refresh the same run, so only
  // a different id is a new fire; the latch's re-arm signal. Without it, one
  // dismissal would silence every later "Backup Everything" press (the latch would
  // never re-arm outside openRun), and the user would wait out the whole run
  // for nothing but the terminal toast.
  const lastCorrelatedRun = useRef<string | null>(null);
  // True from the user's confirm press until the watch correlates the pass it
  // started. Within that window a correlation may steal the sheet from
  // whatever run it currently shows (the deep-link contract); once spent, a
  // poll tick re-reporting the same correlated run must not.
  const fireArmed = useRef(false);
  const armFire = useCallback(() => {
    fireArmed.current = true;
  }, []);
  const openRun = (run: Run) => {
    sheetDismissed.current = false;
    // A fresh open also releases the watch's display ownership
    // (watchOwnedRun, declared with displayedRun below): the run the user
    // just opened is the polled list's to refresh until a live watch tick
    // re-claims it, never the frozen record of a watch that has since
    // stopped speaking about it.
    setWatchOwnedRun(null);
    setSheetRun(run);
    setSheetOpen(true);
  };
  // The watch's onRun, handed to the phone trigger. Replace the sheet's run
  // only when (a) it shows nothing yet, (b) it shows this run already (the
  // poll refresh that carries a running run to its terminal state), or (c)
  // this correlation is the user's just-fired pass. Any other tick; the live
  // everything watch re-reporting its run on every poll while the user reads
  // a different run's sheet; must not yank the sheet back (the "sheet jumps
  // back to the everything pass every two seconds" bug).
  const handleWatchRun = useCallback((run: Run) => {
    const isNewCorrelation = lastCorrelatedRun.current !== run.id;
    // Read the arm before spending it: the state updater below runs at commit
    // time, after this function has returned.
    const armed = fireArmed.current;
    lastCorrelatedRun.current = run.id;
    // This tick's record is the freshest statement about this run; the sheet's
    // resolution lets it outrank the slower page-list copy (see displayedRun).
    setWatchOwnedRun(run.id);
    if (isNewCorrelation) {
      sheetDismissed.current = false; // new fire re-arms the deep-link
      fireArmed.current = false; // the arm is spent on its correlation
    }
    setSheetRun((prev) =>
      prev == null || prev.id === run.id || (isNewCorrelation && armed) ? run : prev
    );
    if (!sheetDismissed.current) setSheetOpen(true);
  }, []);

  // Rotation past 48rem: the bottom sheet is a phone surface. Crossing to
  // desktop clears the open state (and the latch) so the sheet cannot float
  // over the desktop grid; the sheet host below carries the matching
  // !isDesktop gate as the belt.
  useEffect(() => {
    if (isDesktop) {
      setSheetRun(null);
      setSheetOpen(false);
      sheetDismissed.current = false;
    }
  }, [isDesktop]);

  // Single /api/status fetch shared by the Protection + Ransomware cards (no
  // duplicate round-trip — both cards read the same extended domain status).
  const [statusDomains, setStatusDomains] = useState<DomainStatus[]>([]);
  const [statusLoading, setStatusLoading] = useState(true);
  // The last /api/status load refused or failed; cleared by the next
  // success. The phone off-site line reads it to distinguish "checked, and
  // genuinely no copy" from "never got an answer".
  const [statusFailed, setStatusFailed] = useState(false);
  const [coverage, setCoverage] = useState<CoverageReport | null>(null);
  const [coverageLoading, setCoverageLoading] = useState(true);

  // Newest run for the summary tier's "Last result" cell. listRuns returns
  // newest-first, so runs[0] is the latest. Polled (not fetched once) so the
  // cell doesn't freeze on whatever domain happened to be running at page
  // load — mirrors ActivityLog's own listRuns polling (same cadence) so both
  // widgets stay in sync (#158: card stuck on a finished run while the
  // Activity Log had already moved on to the next domain).
  const [runs, setRuns] = useState<Run[]>([]);
  const [runsReady, setRunsReady] = useState(false);
  const [runsFailed, setRunsFailed] = useState(false);
  const refreshRuns = useCallback(() => {
    listRuns()
      .then((res) => {
        if (res.ok) {
          setRuns(res.runs ?? []);
          setRunsFailed(false);
        } else {
          setRunsFailed(true);
        }
      })
      .catch(() => setRunsFailed(true))
      .finally(() => setRunsReady(true));
  }, []);
  useEffect(() => {
    refreshRuns();
    const id = setInterval(refreshRuns, SUMMARY_RUNS_POLL_MS);
    return () => clearInterval(id);
  }, [refreshRuns]);

  // The sheet renders the freshest copy of its run. Two feeds write the two
  // candidates, at different cadences: the page's polled listRuns state
  // (10s) and the everything watch's onRun records (2s, delivered through
  // handleWatchRun into sheetRun). Which one wins is decided by who last
  // spoke about the id: once the watch has correlated the run the sheet
  // shows, sheetRun is the freshest statement (it updates on every watch
  // tick), and letting the slower list copy override it ghosted "Running"
  // for up to 10s past the Done toast. A run the user opened from Recent
  // runs and the watch does not track keeps the old resolution: the list
  // copy refreshes the tap-time record on the page's own cadence. Resolving
  // by id at render, not storing a frozen copy, is still the shape of it.
  // (Placed after the runs state it reads.)
  // State rather than a ref: releasing ownership has to reach the screen. As
  // a ref the release changed nothing until some other update happened to
  // re-render the page, so the sheet kept painting the frozen watch record.
  // React bails out when the value does not change, so the per-tick claim
  // below costs nothing.
  const [watchOwnedRun, setWatchOwnedRun] = useState<string | null>(null);
  const listRun = sheetRun ? runs.find((r) => r.id === sheetRun.id) : undefined;
  // A finished run has nothing left to say, so whichever feed carries the
  // terminal record wins, whether or not the watch is still alive. That is
  // what keeps the Done toast and the sheet on the same tick, and it is why
  // the watch ending is safe to act on: only a chain that died without a
  // terminal record hands the sheet back to the slower list.
  const displayedRun = !sheetRun
    ? null
    : sheetRun.finishedAt != null
      ? sheetRun
      : listRun?.finishedAt != null
        ? listRun
        : watchOwnedRun === sheetRun.id
          ? sheetRun
          : listRun ?? sheetRun;
  // The watch chain's end, reported by the trigger (the pending phase
  // clearing): ownership is released so displayedRun falls back to the
  // polled list again. A chain that died without delivering a terminal
  // record, the watch's own timeout for instance, kept the sheet
  // frozen on its last watch record ("Running") while the 10s list already
  // showed the truth: the row behind the sheet flipping to Success under a
  // sheet that never follows. openRun
  // clears it the same way. Idempotent by construction: a later tick simply
  // re-claims ownership.
  const watchStopped = useCallback(() => {
    setWatchOwnedRun(null);
  }, []);

  // The scheduler's own "what fires next" list, for the summary tier's Next
  // backup cell ([545], issue #187). Polled on the same 30s cadence
  // ActivityLog uses for the identical endpoint — the two read the same source
  // and must not be able to disagree with each other on screen, which was the
  // whole complaint.
  const [scheduleNext, setScheduleNext] = useState<ScheduleNext[]>([]);
  useEffect(() => {
    let active = true;
    const load = () => {
      getScheduleNext()
        .then((next) => {
          if (active) setScheduleNext(next);
        })
        .catch(() => {/* non-fatal; the cell falls back to "not scheduled" */});
    };
    load();
    const id = setInterval(load, SUMMARY_SCHEDULE_POLL_MS);
    return () => {
      active = false;
      clearInterval(id);
    };
  }, []);

  // Page-level banners are capped at one: the Fresh-install nudge wins over the
  // Recovery-kit nag. Fresh dismissal persists in localStorage (shared key), and
  // while Fresh is showing the Recovery nag is suppressed.
  const [freshDismissed, setFreshDismissed] = useState(() => {
    try {
      return localStorage.getItem(RECOVERY_NUDGE_DISMISSED) === "1";
    } catch {
      return false;
    }
  });
  const dismissFresh = () => {
    try {
      localStorage.setItem(RECOVERY_NUDGE_DISMISSED, "1");
    } catch {
      /* storage unavailable — dismiss for this session only */
    }
    setFreshDismissed(true);
  };
  const freshShown = !statusLoading && !freshDismissed && isFreshInstall(statusDomains);

  useEffect(() => {
    let active = true;
    const load = () => {
      getStatus()
        .then((res) => {
          if (active && res.ok) {
            setStatusDomains(res.domains ?? []);
            setStatusFailed(false);
          } else if (active) {
            // A refused read is a failed read: the phone off-site line has to
            // say "could not load" instead of claiming "no off-site copy"
            // from an empty list.
            setStatusFailed(true);
          }
        })
        .catch(() => {
          if (active) setStatusFailed(true);
        })
        .finally(() => {
          if (active) setStatusLoading(false);
        });
      // Same trigger as the status above: adding a container to a schedule, or
      // switching a whole domain off, changes what is covered, and both of
      // those dispatch bv:settings-changed.
      getCoverage()
        .then((res) => {
          if (active && res.ok) setCoverage(res.coverage);
        })
        .catch(() => {/* non-fatal */})
        .finally(() => {
          if (active) setCoverageLoading(false);
        });
    };
    load();
    // Live-refresh when protection-relevant state changes elsewhere (e.g. a manual
    // restore drill on the Settings page, which dispatches this event) so the
    // scorecard pills reflect the new outcome without a page reload.
    window.addEventListener("bv:settings-changed", load);
    return () => {
      active = false;
      window.removeEventListener("bv:settings-changed", load);
    };
  }, []);

  // Heatmap → Activity Log drilldown: clicking a heatmap cell narrows the log
  // to that LOCAL calendar day (ISO YYYY-MM-DD — the same en-CA local mapping
  // the heatmap cells are keyed by) and scrolls the log card into view.
  // Clicking the active day again — or the chip's × inside ActivityLog —
  // clears it. State lives here because the two cards are independent,
  // individually hideable dashboard blocks with no other shared parent.
  const [logDayFilter, setLogDayFilter] = useState<string | null>(null);
  const activityLogBlockRef = useRef<HTMLDivElement>(null);
  const selectLogDay = (isoDay: string) => {
    const next = logDayFilter === isoDay ? null : isoDay;
    setLogDayFilter(next);
    if (next === null) return; // toggled off; stay put, nothing to show
    const el = activityLogBlockRef.current;
    if (!el) return; // Activity Log block hidden via customize; filter still set
    const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    el.scrollIntoView({ behavior: reducedMotion ? "auto" : "smooth", block: "start" });
  };

  // A key's log on the MCP card links a run here as ?run=<id>. The log narrows
  // to it and comes into view once, for the link that opened the page; clearing
  // it drops the parameter, so a reload shows the whole log again.
  const [logRunFilter, setLogRunFilter] = useState<string | null>(
    () => new URLSearchParams(window.location.search).get("run") || null
  );
  useEffect(() => {
    if (logRunFilter) activityLogBlockRef.current?.scrollIntoView?.({ block: "start" });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  const clearLogRun = () => {
    setLogRunFilter(null);
    const url = new URL(window.location.href);
    url.searchParams.delete("run");
    window.history.replaceState(window.history.state, "", url);
  };

  // Customizable dashboard (#46) — everything below the heading + banners is a
  // reorderable / hideable block, persisted per-browser via useDashboardLayout.
  const [editing, setEditing] = useState(false);

  // Ordered block list. Each block has a stable id, a label, a `render`
  // callback that produces the node (props preserved exactly from the
  // original render) and an advancedOnly flag. advancedOnly blocks are
  // dropped from BOTH the render and the customize list when not in Advanced
  // view — their order/hidden state still persists.
  //
  // `render` — GlimStone follow-up pass, jdp's live review of this page:
  // "Cardtitelbadges sind falsch platziert. Alle sind nicht im
  // Regenbogenmodus." Was `node: React.ReactNode`, a pre-built element
  // constructed eagerly, in this array's own FIXED definition order — which
  // cannot give a Card heading a correct rainbow position, because the
  // block a user actually SEES at position N depends on their own persisted
  // drag-reorder + hide/show state (`visibleBlocks` below), not this
  // array's literal order. Deferred to a function so each Card's hueIndex
  // can be assigned from the shared `nextHue()` counter (declared right
  // before `visibleBlocks.map()` in the JSX below) at the point each block
  // is ACTUALLY rendered, in the user's own current visible order — the
  // exact same "own running counter, consumed in rendered order" contract
  // Settings.tsx's `nextHue()` already uses, just passed through explicitly
  // here instead of called inline, since the render order here isn't a
  // static JSX literal the way Settings.tsx's tab bodies are.
  //   Most blocks consume exactly one hue slot (one Card, one `nextHue()`
  // call); "summary" consumes three (its own three SummaryCells, via the
  // `nextHue` callback threaded into SummaryTier); "stats" consumes none
  // (StatCardsRow's tiles have no heading badge of their own — nothing to
  // hue).
  const blocks: {
    id: string;
    label: string;
    advancedOnly?: boolean;
    render: (nextHue: () => number) => React.ReactNode;
  }[] = [
    {
      id: "summary",
      label: t("dashboard.blockSummary"),
      // Three DIRECT nextHue() calls, eagerly resolved to plain numbers here
      // rather than a `nextHue` function handed to SummaryTier to call from
      // its own component body — see SummaryTier's own `healthHueIndex` doc
      // for the live ordering bug that shape caused (React doesn't invoke a
      // child component's body until after this whole `blocks` array/render
      // pass has already returned, so calls made from inside SummaryTier ran
      // AFTER every sibling block below had already consumed its own slot).
      render: (nextHue) => (
        <SummaryTier
          t={t}
          domains={statusDomains}
          scheduleNext={scheduleNext}
          loading={statusLoading}
          newestRun={runs[0] ?? null}
          healthHueIndex={nextHue()}
          nextBackupHueIndex={nextHue()}
          lastResultHueIndex={nextHue()}
        />
      ),
    },
    {
      id: "activityLog",
      label: t("activityLog.title"),
      // The wrapper div carries the scroll anchor for the heatmap drilldown
      // (scroll-mt keeps the card heading clear of the viewport's top edge).
      render: (nextHue) => (
        <div ref={activityLogBlockRef} className="scroll-mt-4">
          <ActivityLog
            dayFilter={logDayFilter}
            onClearDayFilter={() => setLogDayFilter(null)}
            runFilter={logRunFilter}
            onClearRunFilter={clearLogRun}
            hueIndex={nextHue()}
          />
        </div>
      ),
    },
    {
      id: "stats",
      label: t("dashboard.blockStats"),
      render: () => <StatCardsRow t={t} advanced={advanced} />,
    },
    {
      id: "protection",
      label: t("dashboard.protectionTitle"),
      render: (nextHue) => (
        <ProtectionCard t={t} domains={statusDomains} loading={statusLoading} hueIndex={nextHue()} />
      ),
    },
    {
      // Directly after protection, because the two answer halves of one
      // question: that card says whether what IS scheduled ran on time, this
      // one says what is not scheduled at all.
      id: "coverage",
      label: t("coverage.title"),
      render: (nextHue) => (
        <CoverageCard t={t} coverage={coverage} loading={coverageLoading} hueIndex={nextHue()} />
      ),
    },
    {
      // Beside coverage, because the two answer the same question from
      // opposite ends: what nothing backs up, and what the backups say went
      // wrong.
      id: "anomalies",
      label: t("anomaly.title"),
      render: (nextHue) => <AnomaliesBlock t={t} hueIndex={nextHue()} />,
    },
    {
      id: "ransomware",
      label: t("ransomware.title"),
      advancedOnly: true,
      render: (nextHue) => (
        <RansomwareCard t={t} domains={statusDomains} loading={statusLoading} hueIndex={nextHue()} />
      ),
    },
    // Last Backups and Run History are separate blocks (#50 follow-up) so each
    // can be hidden, reordered and read at full width independently.
    {
      id: "lastBackups",
      label: t("dashboard.lastBackups"),
      render: (nextHue) => <LastBackupsCard t={t} hueIndex={nextHue()} />,
    },
    {
      id: "runHistory",
      label: t("run.historyTitle"),
      // dense={false} = the desktop history-card face; the data comes from the
      // page's polled listRuns (the phone face reads the same list; one
      // fetch, one source, no disagreement).
      render: (nextHue) => (
        <RunsCard
          t={t}
          hueIndex={nextHue()}
          dense={false}
          runs={runs}
          loading={!runsReady}
          failed={runsFailed}
          refreshRuns={refreshRuns}
        />
      ),
    },
    {
      id: "heatmap",
      label: t("dashboard.healthTitle"),
      render: (nextHue) => (
        <HealthHeatmapCard t={t} selectedDay={logDayFilter} onSelectDay={selectLogDay} hueIndex={nextHue()} />
      ),
    },
    {
      id: "storage",
      label: t("dashboard.storageTitle"),
      render: (nextHue) => (
        <StorageCard t={t} hueIndex={nextHue()} dense={false} domains={statusDomains} statusLoading={statusLoading} statusFailed={statusFailed} />
      ),
    },
    {
      id: "spike",
      label: t("spike.title"),
      advancedOnly: true,
      render: (nextHue) => <SpikeCard t={t} hueIndex={nextHue()} />,
    },
  ];

  const defaultOrder = blocks.map((b) => b.id);
  const { order, hidden, reorder, setVisibleOrder, toggleHidden, toggleWidth, getWidth, reset } =
    useDashboardLayout(defaultOrder);

  // Persisted order → concrete blocks. Unknown/stale ids are guarded out, and
  // advancedOnly blocks are dropped while not in Advanced view.
  const byId = new Map(blocks.map((b) => [b.id, b]));
  const orderedAvailable = order
    .map((id) => byId.get(id))
    .filter(
      (b): b is (typeof blocks)[number] => !!b && (advanced || !b.advancedOnly)
    );
  // A link to one run shows the activity log even where the layout hides it,
  // or the link would land on a page without the log it points to.
  const shown = (id: string) => !hidden.has(id) || (id === "activityLog" && logRunFilter !== null);
  const visibleBlocks = orderedAvailable.filter((b) => shown(b.id));
  const hiddenBlocks = orderedAvailable.filter((b) => !shown(b.id));

  // A card is carried by the grip in its control bar and lands in its gap;
  // the move buttons are the keyboard's way to do the same.
  const grid = useRef<HTMLDivElement>(null);
  const drag = useReorder({
    ids: visibleBlocks.map((b) => b.id),
    container: grid,
    attr: "data-grid-block",
    axis: "x",
    arm: "move",
    enabled: editing,
    onReorder: setVisibleOrder,
  });
  const byVisibleId = new Map(visibleBlocks.map((b) => [b.id, b]));
  const carriedBlocks = drag.order
    .map((id) => byVisibleId.get(id))
    .filter((b): b is (typeof visibleBlocks)[number] => !!b);

  return (
    // GlimStone follow-up pass (live-review round, jdp emphatic: "Die
    // Abstände der Cards passen nicht. Bitte systemweit anpassen!"): this
    // page's own block-to-block rhythm was gap-6 (24px) — measured live via
    // getBoundingClientRect, every adjacent pair of stacked cards sat exactly
    // 24px apart, so it wasn't a mix of ad-hoc values WITHIN this page. The
    // actual drift is against the rest of the app: Settings.tsx's own
    // tab-panels wrapper already settled on gap-10 (40px) as "the same 40px
    // rhythm every Card-to-Card gap already uses" (see that file's own
    // comment, live-review round) and even bumped its heading-to-first-card
    // gap to match it for the identical reason this pass now applies here —
    // a smaller gap right before the first card read as visually mismatched
    // next to the wider rhythm below it. Splitting this outer wrapper into
    // its own gap-6 header/banner group plus an outer gap-10 mirrors that
    // exact two-level structure (Settings' `hueSeq` comment calls out the
    // same shared-counter pattern this page already reuses, for the same
    // reason: matching an established convention beats reinventing one).
    //   PAGE_SHELL (jdp live-review, "Können wir die nicht überall gleich
    // breit machen?"): this page's `gap-10 max-w-6xl` is now that shared
    // constant, unchanged in value — it is the page the app-wide 1152px was
    // chosen FROM, because it owns the only content dense enough to have a
    // measurable opinion about width (a md:grid-cols-2 block grid, 7-column
    // container-query run rows, and the Advanced 7-across stat tier, whose
    // longest German label needs exactly 136px of a 136px cell at 1024px —
    // zero slack). Swapping the literal for the constant is what stops the
    // other nine pages drifting away from it again. See lib/pageShell.ts.
    //   The nested gap-6 group below stays: heading + banner are a tight pair
    // that deliberately sits closer than the 40px Card rhythm, the same
    // two-level shape Settings.tsx uses for its heading + tab strip.
    // The responsive rhythm (gap-6 below 48rem, the PAGE_SHELL
    // gap-10 at and above; identical on desktop by construction). See
    // lib/pageShell.ts's PAGE_SHELL_RESPONSIVE and the eslint exceptions data.
    <div className={PAGE_SHELL_RESPONSIVE}>
      <div className="flex flex-col gap-6">
      {/* Page heading — fixed (contextual, not customizable). The pencil in the
          top-right corner toggles the customize/edit mode.
            That pencil is `h-8 w-8` + centring, not the `p-2` it used to size
          itself with. It is a square icon-only badge by every other measure
          (the same rounded-pill tile, the same bg-carbon-surface2/hover
          recipe as Settings' Registry add/remove and FolderBrowser's browse
          badge), but it derived its own footprint from padding around an 18px
          glyph and landed on 34px — measured live — where every other square
          icon badge in the app is 32px. Two pixels, but exactly the drift the
          one-size rule exists to stop: a call site sizing itself from its own
          contents instead of taking the shared number. See Badge.tsx's "ONE
          SIZE FOR SQUARE ICON BADGES" block. */}
      <div className="flex items-start justify-between gap-4">
        <div>
          {/* Identity header, mobile half: the app wordmark (the same
              theme-switching mark pair the desktop sidebar renders) leads the
              phone page; md+ never sees this row. "BombVault" is the brand
              proper noun, not a translation unit; the same standing choice as
              the logo marks' own alt text in Sidebar.tsx. */}
          <div className="mb-2 flex items-center gap-2 md:hidden">
            <img
              src="/logo.svg"
              alt=""
              draggable={false}
              className="h-6 w-6 object-contain block dark:hidden"
            />
            <img
              src="/logo-light.svg"
              alt=""
              draggable={false}
              className="h-6 w-6 object-contain hidden dark:block"
            />
            <span className="text-lg font-semibold tracking-tight text-carbon-text">
              BombVault
            </span>
          </div>
          <PageTitle>{t("dashboard.title")}</PageTitle>
          <div className="flex flex-col gap-1">
            <OffsiteIndicator domain="containers" withLabel />
            <OffsiteIndicator domain="vms" withLabel />
            <OffsiteIndicator domain="flash" withLabel />
            <OffsiteIndicator domain="files" withLabel />
            <OffsiteIndicator domain="zfs" withLabel />
          </div>
        </div>
        {/* Real `.glim-bubble` tooltip, not the OS's native `title=` balloon
            (whole-app sweep). This was the LAST icon-only control in the app
            still naming itself with a bare `title=`/`aria-label` pair —
            measured live on the deployed container: 32px, rounded-control,
            and `title` present, while every other icon-only trigger (every
            Badge `tip`, FolderBrowser's browse badge, Settings' registry and
            copy badges, PathModeSwitch's and SourceToggle's segments) already
            rendered the shared bubble. IconTipButton.tsx's own header is
            explicit that a stray native `title=` on an icon-only trigger is
            precisely the anti-pattern that file exists to replace, and
            design-language's tooltip section calls the bubble unconditional
            for a control with no visible text.
              It stays a hand-rolled `h-8 w-8` button rather than becoming a
            Badge, for the reason already recorded below: its background
            legitimately flips between two states, and Badge's icon-only
            tone="active" is unconditionally accent-filled. IconTipButton
            takes the className verbatim, so both states survive
            byte-identical and only the tooltip mechanism changes. `aria-pressed` is threaded
            through IconTipButton's new optional prop so the toggle state is
            not lost in the swap (this is a toggle, not a one-shot action).
              32px and `rounded-pill` keep it matched to every other square
            icon control app-wide and tracking the shape engine. */}
        <IconTipButton
          onClick={() => setEditing((v) => !v)}
          tip={editing ? t("dashboard.customizeDone") : t("dashboard.customize")}
          ariaPressed={editing}
          /* The pencil toggles the customizable desktop grid's edit
             mode; on phones that grid is replaced by the fixed Home block
             order, so the control has nothing to edit and stays desktop-only.
             md+ renders it exactly as before. */
          className={`max-md:hidden shrink-0 inline-flex h-8 w-8 items-center justify-center rounded-pill motion-safe:transition-colors ${
            editing
              ? "bg-accent text-accentContrast"
              : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-surface3 hover:text-carbon-text"
          }`}
        >
          {/* FILLED pencil (design-language.md "Icon glyphs", rule 218 —
              already a closed silhouette under its old stroke, so it flips
              directly: same path data, `fill="currentColor"`). The old
              facet-highlight line is dropped rather than faked as a
              surface-colour cutout — this button's own background flips
              between `bg-carbon-surface2` and `bg-accent` (the `editing`
              state above), so a cutout hard-coded to one of those two colours
              would show a visible mismatched patch in the other; the plain
              pencil silhouette alone already reads clearly as "edit" without
              it.
                This used to be an inline `<svg>` right here, and it was the
              app's ONLY pencil. Files.tsx's own "Ordner-Set bearbeiten" badge
              needed the same glyph (jdp's icon-badge round for that tab), and
              the standing instruction there was to reuse this one rather than
              draw a second — so the path moved verbatim into Sidebar.tsx's
              shared icon set as IconPencil and this call site now renders that
              component. Same silhouette; the only difference is that IconPencil
              crops its viewBox to the ink (see its own comment for the measured
              numbers and why), so this button's glyph goes from 11.34 × 10.58
              px of ink at 18px to 12.20 × 11.39 px at the app's standard 16px
              icon-badge glyph size — sub-pixel-per-axis in practice, and it
              brings this one in line with every other 16px glyph in a 32px
              tile. The button itself is untouched: it stays a hand-rolled
              `h-8 w-8` rather than a Badge, because its background legitimately
              flips between two states (`bg-accent` while editing,
              `bg-carbon-surface2` at rest) and Badge's icon-only tone="active"
              is unconditionally accent-filled. */}
          <IconPencil />
        </IconTipButton>
      </div>

      {/* Fresh/rebuilt install nudge to the guided Recovery tab — fixed
          (contextual). Reuses the shared /api/status fetch below. */}
      <FreshInstallNudge
        t={t}
        domains={statusDomains}
        loading={statusLoading}
        dismissed={freshDismissed}
        onDismiss={dismissFresh}
      />

      {/* Recovery-kit nag — fixed (contextual): only while encryption is on and
          the recovery kit is unstored. */}
      <RecoveryNag t={t} suppressed={freshShown} />

      {/* Customize controls — the pencil in the heading toggles edit mode; while
          editing, the Reset button + hint appear here. */}
      {editing && (
        <div className="flex flex-col gap-2 max-md:hidden">
          <Button
            label={t("dashboard.resetLayout")}
          labelKey="dashboard.resetLayout"
            tone="neutral"
            onClick={reset}
            className="self-start"
          />
          <p className="text-xs text-carbon-textMuted">{t("dashboard.customizeHint")}</p>
        </div>
      )}
      </div>

      {/* The phone Home blocks; the glanceable surfaces in the contracted
          order (identity header is the page header above). JSX-gated on
          !isDesktop; see the Mobile Home blocks banner above for the mount
          discipline, the density contract and why the hue positions are
          static literals. The thumb-zone trigger joins as this column's last
          child (the StickyActionBar below). */}
      {!isDesktop && (
        <div className="flex flex-col gap-4">
          {anomaliesLead && <AnomaliesBlock t={t} hueIndex={0} />}
          <NextRunCard dense t={t} hueIndex={lead} scheduleNext={scheduleNext} domains={statusDomains} loading={statusLoading} />
          <RunsCard
            dense
            t={t}
            hueIndex={lead + 1}
            runs={runs}
            loading={!runsReady}
            failed={runsFailed}
            refreshRuns={refreshRuns}
            onOpenRun={openRun}
          />
          <StorageCard dense t={t} hueIndex={lead + 2} domains={statusDomains} statusLoading={statusLoading} statusFailed={statusFailed} />
          {!anomaliesLead && <AnomaliesBlock t={t} hueIndex={3} />}
          {advanced && <RansomwareCard t={t} domains={statusDomains} loading={statusLoading} hueIndex={4} />}
          <ProtectionCard t={t} domains={statusDomains} loading={statusLoading} hueIndex={5} />
          <CoverageCard t={t} coverage={coverage} loading={coverageLoading} hueIndex={6} />
          {/* The activity log reaches mobile here; the component is
              self-contained (own card chrome + heading, per its header
              comment), so the mount is one line. dayFilter stays shared: a
              heatmap tap narrows both presentations because they render the
              same state. */}
          <ActivityLog dayFilter={logDayFilter} onClearDayFilter={() => setLogDayFilter(null)} hueIndex={7} />
        </div>
      )}

      {/* Ordered, visible blocks in a responsive grid: full-width cards span
          both columns, half-width cards flow two-per-row (request B). Below
          grid is JSX-gated on isDesktop (see the max-md note by the gate
          below); the phone reads the Home blocks above instead. The
          width. The col-span lives on this wrapper div (not on
          CustomizableBlock's own root) so it applies in both edit mode (where
          CustomizableBlock renders its control-bar div) and view mode (where
          it renders only `<>{children}</>`). In edit mode the wrapper is also
          what lifts when a card is carried by its grip; the grid derives each
          cell's span from order and width. */}
      {/* hueSeq/nextHue — SAME page-wide running-counter pattern as
          Settings.tsx's own `nextHue()` (see that file's own `hueSeq`
          comment), just declared here instead of at the top of the return:
          it must be freshly reset to 0 on every render (a stale count would
          drift every heading's colour after any state change) AND consumed
          in the ACTUAL rendered order of `visibleBlocks` below — the user's
          own current drag-reorder/hide-show layout, not this file's fixed
          `blocks` definition order (GlimStone follow-up pass, jdp: "Alle
          sind nicht im Regenbogenmodus" — every heading on this page was
          still the flat, un-hued Task-5 default; see the `blocks` array's
          `blocks` array's
          own `render` comment above for why a plain per-block literal index
          can't do this and a shared counter can). Each block's own `render`
          callback calls this DIRECTLY, once per real heading badge it owns,
          right here in this synchronous pass (most blocks once; "summary"
          three times, back-to-back, for its own three SummaryCells; "stats"
          zero times — StatCardsRow has no heading to hue) — never by handing
          the `nextHue` function itself to a child component to call later
          from its own body: React doesn't actually invoke a child function
          component until after THIS WHOLE render pass has returned, so a
          call made from inside a child runs strictly after every direct
          call made here, landing on the wrong (already-past-the-end)
          indices — caught live the first time this shipped (see
          SummaryTier's own `healthHueIndex` doc for the exact live numbers)
          and fixed by resolving all three of its indices to plain numbers
          right here, the same way every other block already does. */}
      {/* The isDesktop JSX gate, not a max-md:hidden class: below the
          breakpoint the desktop customizable grid must not merely be invisible
          but unmounted; a CSS-hidden grid stays mounted, and its cards keep
          fetching behind the user's back (the storage card re-reads its four
          stats windows; the runs list and heatmap keep polling) on top of the
          phone blocks' own reads, doubling every phone load's round-trips for
          data the user cannot see. Gating both faces on the same isDesktop
          (the phone column above uses its complement) means exactly one face
          is ever alive, whichever way the breakpoint is crossed. At md+ the
          gate is transparent, so the desktop grid is unchanged. */}
      {isDesktop && (
      <div
        ref={grid}
        className={`relative grid grid-cols-1 gap-10 md:grid-cols-2 ${drag.held !== null ? "glim-drag-armed" : ""}`}
      >
        {(() => {
          let hueSeq = 0;
          const nextHue = () => hueSeq++;
          return carriedBlocks.map((b, i) => (
            <div
              key={b.id}
              data-grid-block={b.id}
              className={`${getWidth(b.id) === "half" ? "md:col-span-1" : "md:col-span-2"} ${drag.look(b.id)}`}
            >
              <CustomizableBlock
                id={b.id}
                label={b.label}
                index={i}
                total={visibleBlocks.length}
                isFirst={i === 0}
                isLast={i === visibleBlocks.length - 1}
                editing={editing}
                onGripPointerDown={(e) => drag.press(e, b.id)}
                /* Move relative to the VISIBLE neighbour (skips hidden / advanced-gated
                   blocks in the stored order) so a single press always reorders. */
                onMoveUp={() => {
                  if (i > 0) reorder(b.id, visibleBlocks[i - 1].id);
                }}
                onMoveDown={() => {
                  if (i < visibleBlocks.length - 1)
                    reorder(b.id, visibleBlocks[i + 1].id);
                }}
                onHide={() => toggleHidden(b.id)}
                width={getWidth(b.id)}
                onToggleWidth={() => toggleWidth(b.id)}
                t={t}
              >
                {b.render(nextHue)}
              </CustomizableBlock>
            </div>
          ));
        })()}
      </div>
      )}

      {/* Hidden-cards tray — only while editing and something is hidden. */}
      {/* Desktop-only like the grid it serves: editing is only reachable
          through the pencil, which the isDesktop gate unmounts on the phone.
          max-md:hidden stays as the belt over the resize window where editing
          is on and the viewport drops below the breakpoint before React
          commits the unmount. */}
      {editing && hiddenBlocks.length > 0 && (
        <div className="relative flex max-md:hidden flex-col gap-3 rounded-card border border-dashed border-carbon-border p-4">
          <h2 className="flex items-center">
            <Badge tone="heading" size="heading" wrap>{t("dashboard.hiddenCards")}</Badge>
          </h2>
          <div className="flex flex-wrap gap-2">
            {hiddenBlocks.map((b) => (
              <div
                key={b.id}
                className="flex items-center gap-2 rounded-pill bg-carbon-surface2 px-2.5 py-1.5"
              >
                <span className="max-w-48 truncate text-xs text-carbon-textSub">
                  {b.label}
                </span>
                <Button
                  label={`${t("dashboard.showCard")} ${b.label}`}
                  labelKey="dashboard.showCard"
                  tone="neutral"
                  onClick={() => toggleHidden(b.id)}
                />
              </div>
            ))}
          </div>
        </div>
      )}

      {/* The run detail sheet, hosted component-locally (the
          Containers.tsx contract): opened by a recent-run row tap and by the
          backup watch's onRun correlation (the thumb-zone trigger deep-links
          the live run into the same sheet). A closed sheet stays closed
          against later watch polls (the latch); an explicit row tap re-arms
          it. Gated on !isDesktop like the rest of the phone surface: the
          bottom sheet has no desktop form, and the rotation-clear effect
          above empties the state anyway (the gate is the belt, the effect
          the braces). */}
      {!isDesktop && displayedRun && (
        <RunDetailSheet
          run={displayedRun}
          open={sheetOpen}
          onClose={() => {
            sheetDismissed.current = true;
            setSheetOpen(false);
          }}
        />
      )}

      {/* Thumb-zone trigger; the watch, the confirm and the toasts live in the
          child. Mounted at every width, because a rotation that unmounts it
          strands a running pass: the watch dies with it and a fresh mount
          comes up idle. Above the breakpoint the child renders no bar, so the
          desktop tree carries no phone chrome. */}
      <PhoneEverythingTrigger
        t={t}
        onWatchRun={handleWatchRun}
        onArmFire={armFire}
        onWatchStopped={watchStopped}
      />
    </div>
  );
}
