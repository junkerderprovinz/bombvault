import { useEffect, useId, useRef, useState } from "react";
import { checkDomain, listSnapshotFiles, listSnapshotFilesFileSet } from "../../lib/api";
import type { FileEntry, Run } from "../../lib/api";
import { useT } from "../../lib/i18n";
import type { TranslationKey } from "../../lib/i18n";
import { humanBytes } from "../../lib/forecast";
import { formatClockTime, formatDuration, formatTs } from "../../lib/reltime";
import { isOwnReason, runReason } from "../../lib/runReason";
import { runKindLabel, runTargetText, statusLabel, statusTone } from "../../lib/runDisplay";
import { buildLogLines, formatLogDate } from "../../lib/activityLog";
import type { LogLine, ResolveName } from "../../lib/activityLog";
import { useProgress } from "../../lib/progress";
import { useVisibilityGate } from "../../lib/useVisibilityGate";
import { loadErrorMessage } from "../../lib/errors";
import { Badge } from "../Badge";
import { CheckDraw } from "../CheckDraw";
import { colorFor, glyphFor, glyphLabelKey } from "../ActivityLog";
import { ProgressBar } from "../ProgressBar";
import { SnapshotFileTree } from "../SnapshotFileTree";
import { BottomSheet } from "./BottomSheet";

// ---------------------------------------------------------------------------
// RunDetailSheet — the full-screen run-detail sheet.
//
// Hosted COMPONENT-LOCALLY by whichever surface owns the run row (the
// container/file-set surfaces, the Dashboard): `<RunDetailSheet run={...} open
// onClose={...} />` with the consumer's own open state. No route is added and
// router.tsx is frozen — the sheet opens over whatever surface invoked it,
// which is the hosting contract.
//
// RECORDED DEVIATION vs the screen spec's verbatim wording (the frozen-API
// data adaptations contract — binding): the spec's
// "new / changed / unchanged" stat triad and its per-file exclusion-reason log
// lines have NO source on the frozen Run record (web/src/lib/api.ts —
// id, targetId, kind, status, startedAt, finishedAt, snapshotId, bytes,
// error, acknowledged, target, domain; progress events carry only
// key/phase/percent/active/startedAt/snapshotIndex/snapshotTotal). There are
// no per-file counts and no exclusion lines anywhere on the wire, and this
// milestone may not change that. The contracted substitutes, and ONLY these,
// are rendered:
//   - Data volume  = humanBytes(run.bytes)          (lib/forecast formatter)
//   - Duration     = formatDuration(finishedAt − startedAt) (lib/reltime)
//   - Snapshot     = run.snapshotId.slice(0, 8), mono, full id as title
//   - activity log = the EXISTING buildLogLines output (same lib the desktop
//     ActivityLog uses — reuse, no duplication, mono timestamp pattern verbatim)
// The per-file triad and the unticked / CACHEDIR.TAG attributions live where
// the truth lives today (the selection surfaces' own copy) and are a recorded
// v2 data candidate (they need a runs schema extension, i.e. an API-bearing
// milestone). Numbers are never fabricated here — repo honesty culture.
//
// Every presentation decision reuses an existing piece:
//   - title/status: lib/runDisplay's shared helpers (extracted from Dashboard
//     into the lib — one implementation, two surfaces)
//   - shell: BottomSheet fullHeight + footer, three close paths
//     and the focus mechanism inherited
//   - log: buildLogLines + ActivityLog's mono pattern (font-mono text-xs +
//     formatLogDate/formatClockTime stamps, colorFor/glyphFor vocabulary) —
//     never a second log style
//   - browse: SnapshotFileTree behind a tonal row (the parent-fetch precedent
//     in Recovery.tsx / Files.tsx; RestorePanel.tsx is the local
//     container twin)
//   - verify: checkDomain() with the Settings integrity tab's own labels
//   - restore entry: the desktop restore surface's own label
//     ("snapshots.restore"), secondary/tonal, never accent — restore is
//     deliberate (design-bible product rule). The guided restore flow itself
//     arrives with the Recovery mobile-flow PR; this entry stays reveal-only
//     by the recorded research default — it reveals the snapshot file
//     tree, the surface the desktop restore flow itself starts from, and
//     deliberately adds no deep link into the wizard.
//
// Domain honesty — what each run kind gets:
//   - browse/restore entry: container + files domains only (the two domains
//     with a snapshot file-listing endpoint: listSnapshotFiles /
//     listSnapshotFilesFileSet). vm/flash/config have no file-level browse API
//     (block storage / singleton paths), so the rows are absent rather than
//     dead — honest degradation, not a button that cannot work.
//   - verify: container/vm/flash/files (checkDomain's domain union). The
//     "config" and "everything" domains have no check endpoint, so the row is
//     absent there.
//   - live section: running/checking runs whose domain has a progress key,
//     gated on page visibility (the visibility gate): a hidden page unmounts the section,
//     which IS the SSE unsubscribe (progress.ts's ref-count closes the shared
//     EventSource); on return the remount reconnects into the backend's
//     snapshot replay, and completion arrives through the consumer's refetched
//     run record — never extrapolated. The "everything" parent run streams no
//     key of its own (its children do), so it shows terminal content only.
// ---------------------------------------------------------------------------
export interface RunDetailSheetProps {
  /** The run to render. The consumer refetches it (listRuns) on visibility
   *  return — the sheet is a pure view over this record and never
   *  extrapolates completion from clocks. */
  run: Run;
  /** Whether the sheet is open. The consumer owns the state (component-local
   *  hosting contract, above). */
  open: boolean;
  /** Called by every close path (BottomSheet's three paths call through). */
  onClose: () => void;
}

// Same cadence ActivityLog.tsx's live tail re-renders at (its LIVE_TICK_MS):
// the off-site live line's elapsed duration and this sheet's live log tick at
// the same visible rate.
const LIVE_TICK_MS = 1000;

/** The shared-SSE progress key for a run's domain, or null when the domain
 *  streams no key of its own ("everything" is the parent run; its per-domain
 *  children are the ones that publish — store.EverythingTargetID). Shapes
 *  mirror the wire examples in progress.ts ("container:plex", "vm:win11") and
 *  the consumers (BackupButton.tsx, VMs.tsx, Files.tsx, Config.tsx
 *  "config", flash as the bare "flash").
 *
 *  The suffix is run.target — the NAME — never run.targetId. The backend
 *  RECORDS vm/files runs under the row's 32-hex id (StartRun(tg.ID) /
 *  StartRun(set.ID)) but PUBLISHES progress under the human name: "vm:"+name
 *  (the backup vkey and the restore rkey, internal/api/service.go) and
 *  "files:"+set.Name (the files backup key; both restore rkeys use the set
 *  name too). Keying by targetId looked up entries the backend never
 *  publishes, so the live section stayed dark for both domains — the
 *  container domain only ever worked by coincidence: a container's id IS its
 *  name. */
function progressKeyFor(run: Run): string | null {
  switch (run.domain) {
    case "container":
    case "vm":
    case "files":
      // target = the name the SSE key publishes under (see above); targetId is
      // the 32-hex row id for vm/files and matches no published key.
      return `${run.domain}:${run.target}`;
    case "flash":
      return "flash";
    case "config":
      return "config";
    default:
      return null;
  }
}

/** The checkDomain() domain union a run's domain maps to, or null when the
 *  domain has no verify endpoint ("config", "everything", "" — none are in
 *  checkDomain's "containers" | "vms" | "flash" | "files" union). */
function verifyDomainFor(domain: Run["domain"]): "containers" | "vms" | "flash" | "files" | null {
  switch (domain) {
    case "container":
      return "containers";
    case "vm":
      return "vms";
    case "flash":
      return "flash";
    case "files":
      return "files";
    default:
      return null;
  }
}

// A run is in flight when it carries the shared "active" status bucket — the
// same running/checking definition statusTone gives the Badges, so the sheet's
// notion of "in flight" can never drift from the status chip two inches above.
function isInFlight(run: Run): boolean {
  return statusTone(run.status) === "active";
}

// Resolves a translation key (+ optional {placeholder} params) — the only
// i18n dependency buildLogLines takes (ActivityLog.tsx's exact closure, kept
// so the merge/dedupe/order logic stays pure and identical between surfaces).
function makeResolver(t: ReturnType<typeof useT>["t"]): ResolveName {
  return (key, params) => {
    let s = t(key as TranslationKey);
    if (params) {
      for (const [name, value] of Object.entries(params)) s = s.split(`{${name}}`).join(value);
    }
    return s;
  };
}

// The ActivityLog.tsx mono line rendering, copied CLASS-FOR-CLASS —
// a single scrollable monospace list (the sheet's body scrolls; the extra
// max-h-96 scroll container of the dashboard widget would nest two scrollers
// inside the full-height sheet, so only the inner row pattern is reused).
function LogList({ lines }: { lines: LogLine[] }) {
  const { t } = useT();
  if (lines.length === 0) return null;
  return (
    <div className="rounded-card bg-black/20 font-mono text-xs leading-relaxed px-4 py-2 flex flex-col gap-0.5">
      {lines.map((l) => (
        <div key={l.id} className="flex items-start gap-2">
          <span className="text-carbon-textMuted shrink-0 tabular-nums">
            {formatLogDate(l.atMs)} {formatClockTime(l.atMs / 1000, true)}
          </span>
          <span className={`shrink-0 w-4 text-center ${colorFor(l.status)}`} aria-label={t(glyphLabelKey(l.status))}>
            {glyphFor(l.status)}
          </span>
          <span className={`flex-1 min-w-0 wrap-break-word ${colorFor(l.status)}`}>{l.text}</span>
        </div>
      ))}
    </div>
  );
}

// The in-flight half of the log section: the ONLY place this sheet subscribes
// to the shared progress singleton. Mounting is the subscription — the parent
// renders this conditionally on useVisibilityGate() (`{visible && ...}`,
// below), so hiding the page unmounts it and the frozen singleton's ref-count
// drops the shared EventSource (whose reconnect replays the server snapshot —
// progress.ts's closeSource contract).
function LiveRunSection({ run, progressKey }: { run: Run; progressKey: string | null }) {
  const { t } = useT();
  const progressMap = useProgress();
  // buildLogLines reads `now` for the live line's elapsed-duration text; tick
  // at the ActivityLog live cadence while mounted.
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), LIVE_TICK_MS);
    return () => clearInterval(id);
  }, []);

  const resolveName = makeResolver(t);
  // Computed per render, deliberately unmemoized: the input is ONE run (the
  // builder is linear in runs), and the live tick re-renders this section
  // every second anyway. An idle "next up" line is dashboard-log furniture;
  // inside a specific run's detail it would read as run content — the
  // `!l.idle` filter is also what keeps the empty-progress edge (no SSE entry
  // yet) from rendering the activity-log's idle-empty line in the sheet.
  const lines = buildLogLines([run], progressMap, [], resolveName, now, now).filter((l) => !l.idle);

  const prog = progressKey ? progressMap[progressKey] : undefined;
  return (
    <>
      {/* Inline (in-flow) variant, deliberately: the default ProgressBar pins
          to a positioned card's bottom edge, which inside the sheet would be
          the fixed panel itself — over the footer. Indeterminate until the
          first SSE frame carries a percent; renders nothing while the entry
          is inactive (the headline Badge and the log tail carry "running"
          until then). */}
      <ProgressBar percent={prog?.percent ?? 0} active={prog?.active ?? false} inline />
      <LogList lines={lines} />
    </>
  );
}

// A finished (or failed/cancelled) run renders its history line from the same
// builder — an empty progress map and no schedule input, so buildLogLines
// reduces to exactly this run's completed line and nothing else.
function HistoryLogSection({ run }: { run: Run }) {
  const { t } = useT();
  const resolveName = makeResolver(t);
  // Same unmemoized reasoning as LiveRunSection: one run in, linear builder,
  // and Date.now() is only the ordering anchor for a finished line.
  const lines = buildLogLines([run], {}, [], resolveName, Date.now()).filter((l) => !l.idle);
  return <LogList lines={lines} />;
}

// ---------------------------------------------------------------------------
// Tonal touch row (>=44px) — the sheet's action-row shape, shared by the
// footer's verify/browse rows and the body's restore entry. Hand-rolled rather
// than Button because the two browse-section controls are DISCLOSURES and need
// aria-expanded/aria-controls, which the Button engine does not pass through;
// the classes are the Button "subtle"/"neutral" tone tokens (bg-carbon-surface2
// / surface3, rounded-control) so the rows read as the same engine language.
// Never accent: restore and browse are deliberate, secondary actions (the
// design bible reserves accent for the page's one primary action).
// ---------------------------------------------------------------------------
function SheetActionRow({
  label,
  onClick,
  disabled,
  expanded,
  controlsId,
  tone = "subtle",
}: {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  /** Disclosure state — set only for the rows that expand the browse section. */
  expanded?: boolean;
  controlsId?: string;
  tone?: "subtle" | "neutral";
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-expanded={expanded}
      aria-controls={controlsId}
      className={`flex min-h-[2.75rem] w-full items-center justify-center gap-2 rounded-control text-sm text-carbon-text hover:bg-carbon-hover motion-safe:active:scale-[.97] disabled:opacity-60 ${
        tone === "neutral" ? "bg-carbon-surface3" : "bg-carbon-surface2"
      }`}
    >
      {label}
    </button>
  );
}

export function RunDetailSheet({ run, open, onClose }: RunDetailSheetProps) {
  const { t } = useT();
  const browseId = useId();
  const inFlight = isInFlight(run);
  const pKey = progressKeyFor(run);
  const browseSupported = run.domain === "container" || run.domain === "files";
  const verifyDomain = verifyDomainFor(run.domain);
  // The visibility pause gate (kept with the other hooks, above the !open early
  // return — hooks rules). Gating the LIVE SECTION below, nothing else: the
  // sheet's terminal content is a pure view over the `run` record and needs
  // no live connection.
  const visible = useVisibilityGate();

  // --- browse section (the SnapshotFileTree mount + its fetch state) --------
  const [browseOpen, setBrowseOpen] = useState(false);
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [filesLoading, setFilesLoading] = useState(false);
  const [filesError, setFilesError] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());

  // Lazy parent-fetch on expand — the RestorePanel.tsx shape (list →
  // {ok, files} → tree props; #129 server reason over the generic message).
  useEffect(() => {
    if (!open || !browseOpen) return;
    setFilesLoading(true);
    setFilesError(null);
    const req =
      run.domain === "container"
        ? listSnapshotFiles(run.targetId, run.snapshotId)
        : listSnapshotFilesFileSet(run.targetId, run.snapshotId);
    req
      .then((res) => {
        if (res.ok) setFiles(res.files ?? []);
        else setFilesError(loadErrorMessage(res, t("files.loadFailed")));
      })
      .catch(() => setFilesError(t("files.loadFailed")))
      .finally(() => setFilesLoading(false));
    // Keyed on the run's identity primitives, not the object: a parent
    // refetch that re-creates the Run object must not re-fetch the tree.
  }, [open, browseOpen, run.domain, run.targetId, run.snapshotId, t]);

  function togglePath(p: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(p)) next.delete(p);
      else next.add(p);
      return next;
    });
  }

  // --- verify row (checkDomain + inline result, IntegrityCard's vocabulary) -
  const [verifyState, setVerifyState] = useState<"idle" | "busy" | "ok" | "fail">("idle");
  const [verifyError, setVerifyError] = useState<string | null>(null);

  async function runVerify() {
    if (!verifyDomain || verifyState === "busy") return;
    setVerifyState("busy");
    setVerifyError(null);
    try {
      const res = await checkDomain(verifyDomain);
      if (res.ok) {
        setVerifyState("ok");
      } else {
        setVerifyState("fail");
        setVerifyError(res.error ?? t("verify.failed"));
      }
    } catch (err) {
      setVerifyState("fail");
      setVerifyError(err instanceof Error ? err.message : t("verify.failed"));
    }
  }

  // --- fresh-success CheckDraw gate -----------------------------------------
  // CheckDraw's own contract (CheckDraw.tsx): it may only appear at the exact
  // moment state transitions busy → FRESH success, never at initial mount —
  // a sheet opened on an old successful run must not animate a checkmark as
  // if the run just completed. One ref-carrying effect watches for that
  // transition — but only counts it WHILE THE SHEET IS OPEN: every host keeps
  // the sheet mounted when closed and keeps feeding it refreshed records from
  // its own poll, so a running → success flip landing behind a dismissed
  // sheet was never witnessed by the user and must not arm the animation for
  // the reopen. prevStatusRef keeps updating unconditionally either way, so
  // the unwitnessed transition is consumed while closed and can never re-fire
  // on reopen. openRef is the house ref-mirror (useBackupWatch's pattern):
  // the effect keys on run.status alone and reads `open` through the ref.
  const [freshOk, setFreshOk] = useState(false);
  const prevStatusRef = useRef(run.status);
  const openRef = useRef(open);
  openRef.current = open;
  useEffect(() => {
    const was = prevStatusRef.current;
    prevStatusRef.current = run.status;
    if (run.status === "success" && (was === "running" || was === "checking") && openRef.current)
      setFreshOk(true);
    else if (run.status !== "success") setFreshOk(false);
  }, [run.status]);

  // Everything the sheet holds is per-open-session state; reset on close so a
  // consumer reusing the same mount for a different run starts clean — the
  // check gate included: a witnessed animation belongs to the open session
  // that saw the transition, never to the next one.
  useEffect(() => {
    if (open) return;
    setBrowseOpen(false);
    setFiles([]);
    setFilesLoading(false);
    setFilesError(null);
    setFilter("");
    setSelected(new Set());
    setVerifyState("idle");
    setVerifyError(null);
    setFreshOk(false);
  }, [open]);

  if (!open) return null;

  // Stat triad — the Frozen-API substitutes ONLY (see the header deviation
  // note). Duration falls back to the same "—" degraded meta mark formatTs
  // uses for a missing timestamp rather than a blank tile, muted so the
  // absence reads as intentional.
  const durationSecs = run.finishedAt != null ? run.finishedAt - run.startedAt : null;
  const durationText = durationSecs == null ? "—" : formatDuration(durationSecs) || "—";
  const durationMissing = durationText === "—";

  const footerContent =
    browseSupported || verifyDomain ? (
      <div className="flex flex-col gap-2 px-4 py-4">
        <div className={browseSupported && verifyDomain ? "grid grid-cols-2 gap-2" : "flex"}>
          {browseSupported && (
            <SheetActionRow
              label={t("recovery.foreignStepBrowse")}
              onClick={() => setBrowseOpen((o) => !o)}
              expanded={browseOpen}
              controlsId={browseId}
            />
          )}
          {verifyDomain && (
            <SheetActionRow
              label={verifyState === "busy" ? t("integrity.checking") : t("integrity.verify")}
              onClick={() => void runVerify()}
              disabled={verifyState === "busy"}
              tone="neutral"
            />
          )}
        </div>
        {/* The verify row's own result, adjacent to its control — the same
            vocabulary IntegrityCard renders after a check (CheckDraw +
            integrity.ok / the scrubbed server reason over verify.failed). */}
        {verifyState === "ok" && (
          <p className="flex items-center gap-2 text-xs text-statusOk">
            <CheckDraw />
            {t("integrity.ok")}
          </p>
        )}
        {verifyState === "fail" && verifyError != null && (
          <p className="rounded-card bg-statusFailBgSoft px-4 py-2 text-xs text-statusFail leading-relaxed wrap-break-word">
            {verifyError}
          </p>
        )}
      </div>
    ) : undefined;

  return (
    <BottomSheet
      open={open}
      onClose={onClose}
      fullHeight
      // Title composes from EXISTING keys only (runKindLabel + runTargetText —
      // no new title key). The interpunct separator keeps user text free of
      // dashes (the em-dash ban is lint-enforced and a hyphen reads as a
      // hyphenation, not a separator).
      title={`${runKindLabel(t, run.kind)} · ${runTargetText(t, run)}`}
      footer={footerContent}
    >
      <div className="flex flex-col gap-4 px-4 py-4">
        {/* Status headline: the shared status chip + completion time. The
            drawn check appears only on a fresh running→success transition
            observed while open (see the gate above). */}
        <div className="flex items-center justify-between gap-2 flex-wrap">
          <Badge tone={statusTone(run.status)}>{statusLabel(run.status, t)}</Badge>
          <span className="flex items-center gap-2 text-xs text-carbon-textMuted tabular-nums">
            {run.status === "success" && freshOk && <CheckDraw />}
            {formatTs(run.finishedAt)}
          </span>
        </div>

        {/* Failed path — the backend's already-scrubbed reason verbatim
            (runReason translates only BombVault's OWN sentences; untranslated
            restic text stays dir="ltr" on any page language, the
            lib/runReason direction contract). */}
        {run.error !== "" && (
          <p
            className="rounded-card bg-statusFailBgSoft px-4 py-2 text-xs text-statusFail leading-relaxed wrap-break-word"
            dir={isOwnReason(run.error) ? undefined : "ltr"}
          >
            {runReason(run.error, t)}
          </p>
        )}

        {/* Stat triad — the UI-review fix carried into this file: stat VALUES
            take the heading role at weight 600 (text-heading font-semibold,
            the Fab label's typography) with tabular numerals so values don't
            shimmer while a rerender moves digits; the snapshot tile is mono
            with the full id as its title (BackupButton toast precedent).
            Weight law: 400/600 only — the house weight discipline. */}
        <div className="grid grid-cols-3 gap-2">
          <div className="flex min-w-0 flex-col gap-1 rounded-card bg-carbon-background px-2 py-2">
            <span className="text-xs text-carbon-textMuted">{t("run.statVolume")}</span>
            <span className="truncate text-heading font-semibold tabular-nums">{humanBytes(run.bytes)}</span>
          </div>
          <div className="flex min-w-0 flex-col gap-1 rounded-card bg-carbon-background px-2 py-2">
            <span className="text-xs text-carbon-textMuted">{t("dashboard.duration")}</span>
            <span
              className={`truncate text-heading font-semibold tabular-nums ${durationMissing ? "text-carbon-textMuted" : ""}`}
            >
              {durationText}
            </span>
          </div>
          <div className="flex min-w-0 flex-col gap-1 rounded-card bg-carbon-background px-2 py-2">
            <span className="text-xs text-carbon-textMuted">{t("run.statSnapshot")}</span>
            <span className="truncate font-mono text-heading font-semibold tabular-nums" title={run.snapshotId}>
              {run.snapshotId.slice(0, 8)}
            </span>
          </div>
        </div>

        {/* Live section — in-flight runs get the progress bar + live log tail,
            conditionally on the visibility gate: hiding the
            page unmounts this subtree, which IS the unsubscribe — the frozen
            singleton's ref-count drops the shared EventSource and drops its
            cached state; showing the page remounts + resubscribes into the
            backend's snapshot replay. While hidden the slot renders nothing
            (an in-flight run has no history line to fall back to, and nobody
            is watching). A run that finished while hidden arrives as a
            refetched `run` record from the consumer — the terminal flip is
            server state, never extrapolated here. Terminal runs get the same
            builder's history line. Both render through LogList, so the sheet
            can never grow a second log style. */}
        {inFlight ? (
          visible ? (
            <LiveRunSection run={run} progressKey={pKey} />
          ) : null
        ) : (
          <HistoryLogSection run={run} />
        )}

        {/* Browse section — the SnapshotFileTree mount (the Recovery.tsx /
            Files.tsx precedent: purely presentational tree, the parent
            owns the fetch). Revealed by either the footer browse row or the
            restore entry below. */}
        {browseSupported && browseOpen && (
          <div id={browseId}>
            <SnapshotFileTree
              files={files}
              loading={filesLoading}
              error={filesError}
              filter={filter}
              onFilterChange={setFilter}
              selected={selected}
              onToggle={togglePath}
              t={t}
            />
          </div>
        )}

        {/* Restore entry — in the scroll body, ABOVE the footer rows
            (secondary, away from the thumb's default path; the design bible's
            "restore is deliberate" rule). It reveals the same snapshot file
            surface the desktop restore flow starts from; the guided restore
            flow itself arrives with the Recovery mobile-flow PR and this
            entry deliberately stays reveal-only — no deep link (the recorded
            research default). */}
        {browseSupported && (
          <SheetActionRow
            label={t("snapshots.restore")}
            onClick={() => setBrowseOpen((o) => !o)}
            expanded={browseOpen}
            controlsId={browseId}
            tone="neutral"
          />
        )}
      </div>
    </BottomSheet>
  );
}
