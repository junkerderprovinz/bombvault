import { useEffect, useId, useRef, useState, type CSSProperties } from "react";
import { hueVars } from "../../lib/appearance";
import {
  checkDomain,
  listSnapshotFiles,
  listSnapshotFilesFileSet,
  restoreContainerFiles,
  restoreFileSetFiles,
} from "../../lib/api";
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
import type { ProgressMap } from "../../lib/progress";
import { useVisibilityGate } from "../../lib/useVisibilityGate";
import { loadErrorMessage } from "../../lib/errors";
import { useConfirm } from "../../lib/useConfirm";
import { Badge } from "../Badge";
import { CheckDraw } from "../CheckDraw";
import { colorFor, glyphFor, glyphLabelKey } from "../ActivityLog";
import { ProgressBar } from "../ProgressBar";
import { SnapshotFileTree } from "../SnapshotFileTree";
import { BottomSheet } from "./BottomSheet";

// ---------------------------------------------------------------------------
// RunDetailSheet; the full-screen run-detail sheet.
//
// Hosted component-locally by whichever surface owns the run row (the
// container/file-set surfaces, the Dashboard): `<RunDetailSheet run={...} open
// onClose={...} />` with the consumer's own open state. No route is added:
// the sheet opens over whatever surface invoked it, so a run detail never
// costs the user their place.
//
// The Run record carries no per-file counts and no exclusion lines (see
// web/src/lib/api.ts: id, targetId, kind, status, startedAt, finishedAt,
// snapshotId, bytes, error, acknowledged, target, domain; progress events
// carry only key/phase/percent/active/startedAt/snapshotIndex/snapshotTotal),
// so exactly the substitutes below are rendered, and nothing beyond them:
//   - Data volume  = humanBytes(run.bytes)          (lib/forecast formatter)
//   - Duration     = formatDuration(finishedAt − startedAt) (lib/reltime)
//   - Snapshot     = run.snapshotId.slice(0, 8), mono, full id as title
//   - activity log = the existing buildLogLines output (same lib the desktop
//     ActivityLog uses; reuse, no duplication, mono timestamp pattern verbatim)
// The per-file counts and the unticked / CACHEDIR.TAG attributions live where
// the truth lives today (the selection surfaces' own copy); surfacing them
// here would need a runs schema extension first. Numbers are never fabricated
// here; repo honesty culture.
//
// Every presentation decision reuses an existing piece:
//   - title/status: lib/runDisplay's shared helpers (extracted from Dashboard
//     into the lib; one implementation, two surfaces)
//   - shell: BottomSheet fullHeight + footer, three close paths
//     and the focus mechanism inherited
//   - log: buildLogLines + ActivityLog's mono pattern (font-mono text-xs +
//     formatLogDate/formatClockTime stamps, colorFor/glyphFor vocabulary);
//     never a second log style
//   - browse: SnapshotFileTree behind a tonal row (the parent-fetch precedent
//     in Recovery.tsx / Files.tsx; RestorePanel.tsx is the local
//     container twin)
//   - verify: checkDomain() with the Settings integrity tab's own labels
//   - restore entry: the desktop restore surface's own label
//     ("snapshots.restore"), secondary and tonal, never accent, because a
//     restore is a decision rather than the sheet's primary action. The
//     guided restore flow is not wired here; the entry restores the browse
//     tree's selection in place (the desktop surfaces' files.restoreConfirm
//     gate, restoreContainerFiles / restoreFileSetFiles) and with nothing
//     selected opens the tree, since the selection is the path's input. No deep
//     link into the wizard.
//
// Domain honesty; what each run kind gets:
//   - browse/restore entry: container + files domains only (the two domains
//     with a snapshot file-listing endpoint: listSnapshotFiles /
//     listSnapshotFilesFileSet). vm/flash/config have no file-level browse API
//     (block storage / singleton paths), so the rows are absent rather than
//     dead; honest degradation, not a button that cannot work.
//   - verify: container/vm/flash/files (checkDomain's domain union). The
//     "config" and "everything" domains have no check endpoint, so the row is
//     absent there.
//   - live section: running/checking runs whose domain has a progress key,
//     gated on page visibility (the visibility gate): a hidden page unmounts the section,
//     which is the SSE unsubscribe (progress.ts's ref-count closes the shared
//     EventSource); on return the remount reconnects into the backend's
//     snapshot replay, and completion arrives through the consumer's refetched
//     run record; never extrapolated. The "everything" parent run streams no
//     key of its own, so it shows its children's lines: while it runs, every
//     live key belongs to it.
// ---------------------------------------------------------------------------
export interface RunDetailSheetProps {
  /** The run to render. The consumer refetches it (listRuns) on visibility
   *  return; the sheet is a pure view over this record and never
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
 *  children are the ones that publish; store.EverythingTargetID). Shapes
 *  mirror the wire examples in progress.ts ("container:plex", "vm:win11") and
 *  the consumers (BackupButton.tsx, VMs.tsx, Files.tsx, Config.tsx
 *  "config", flash as the bare "flash").
 *
 *  The suffix is run.target; the name; never run.targetId. The backend
 *  records vm/files runs under the row's 32-hex id (StartRun(tg.ID) /
 *  StartRun(set.ID)) but publishes progress under the human name: "vm:"+name
 *  (the backup vkey and the restore rkey, internal/api/service.go) and
 *  "files:"+set.Name (the files backup key; both restore rkeys use the set
 *  name too). Keying by targetId looked up entries the backend never
 *  publishes, so the live section stayed dark for both domains; the
 *  container domain only ever worked by coincidence: a container's id is its
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
 *  domain has no verify endpoint ("config", "everything", ""; none are in
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

// A run is in flight when it carries the shared "active" status bucket; the
// same running/checking definition statusTone gives the Badges, so the sheet's
// notion of "in flight" can never drift from the status chip two inches above.
function isInFlight(run: Run): boolean {
  return statusTone(run.status) === "active";
}

// Resolves a translation key (+ optional {placeholder} params); the only
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

// The ActivityLog.tsx mono line rendering, copied class-for-class;
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
          {/* Minute precision, always: the sheet is phone-only (it unmounts
              at the desktop switch) and narrower than the dashboard card, so
              the seconds are precisely the characters that sink the message
              span under one-word width and break words mid-word. The
              dashboard card keeps its seconds on the desktop face; same
              call, split by the face that can afford them. */}
          <span className="text-carbon-textMuted shrink-0 tabular-nums">
            {formatLogDate(l.atMs)} {formatClockTime(l.atMs / 1000, false)}
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

// The in-flight half of the log section: the only place this sheet subscribes
// to the shared progress singleton. Mounting is the subscription; the parent
// renders this conditionally on useVisibilityGate() (`{visible && ...}`,
// below), so hiding the page unmounts it and the singleton's ref-count
// drops the shared EventSource (whose reconnect replays the server snapshot;
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
  // The map is sliced to THIS run's single key before the builder sees it.
  // buildLiveLines emits one line per active key in the map it is handed, so
  // handing over the whole shared map made the sheet of one run list every
  // other run's live lines under its title: during a Backup Everything pass,
  // the "Backup · plex" sheet carried the VM, flash and folder lines too
  // under its title. The runs array was already scoped to
  // the one record, so history lines were never affected; only the map was.
  // The everything parent streams no key of its own (progressKeyFor returns
  // null) and owns every key that is live while it runs, since those are its
  // children. It therefore keeps the whole map: slicing it to nothing would
  // leave the one sheet the thumb-zone trigger deep-links into with no live
  // content at all for the length of the pass.
  const entry = progressKey != null ? progressMap[progressKey] : undefined;
  const ownMap: ProgressMap =
    progressKey == null ? progressMap : entry ? { [progressKey]: entry } : {};
  // Computed per render rather than memoized: the input is one run (the
  // builder is linear in runs), and the live tick re-renders this section
  // every second anyway. An idle "next up" line is dashboard-log furniture;
  // inside a specific run's detail it would read as run content; the
  // `!l.idle` filter is also what keeps the empty-progress edge (no SSE entry
  // yet) from rendering the activity-log's idle-empty line in the sheet.
  const lines = buildLogLines([run], ownMap, [], resolveName, now, now).filter((l) => !l.idle);

  const prog = entry;
  return (
    <>
      {/* The inline variant: the default ProgressBar pins
          to a positioned card's bottom edge, which inside the sheet would be
          the fixed panel itself; over the footer. Indeterminate until the
          first SSE frame carries a percent; renders nothing while the entry
          is inactive (the headline Badge and the log tail carry "running"
          until then). */}
      <ProgressBar percent={prog?.percent ?? 0} active={prog?.active ?? false} inline />
      <LogList lines={lines} />
    </>
  );
}

// A finished (or failed/cancelled) run renders its history line from the same
// builder; an empty progress map and no schedule input, so buildLogLines
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
// Tonal touch row (>=44px); the sheet's action-row shape, shared by the
// footer's verify/browse rows and the body's restore entry. Hand-rolled rather
// than Button because the browse disclosure row needs aria-expanded
// /aria-controls, which the Button engine does not pass through; the classes
// are the Button "subtle"/"neutral" tone tokens (bg-carbon-surface2 /
// surface3, rounded-control) so the rows read as the same engine language.
// Never accent: restore and browse are secondary actions, and the accent
// belongs to the one primary action on a surface. Each row
// still takes its position in the sheet's hue rotation (.glim-hue): the
// rotation's focus ring and any accent reading inside the row follow the
// position, while the row's own fill stays the tonal token above.
// ---------------------------------------------------------------------------
function SheetActionRow({
  label,
  onClick,
  disabled,
  expanded,
  controlsId,
  tone = "subtle",
  hueIndex,
}: {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  /** Disclosure state; set only for the rows that expand the browse section. */
  expanded?: boolean;
  controlsId?: string;
  tone?: "subtle" | "neutral";
  /** The row's position in the sheet's hue rotation. */
  hueIndex?: number;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-expanded={expanded}
      aria-controls={controlsId}
      className={`flex min-h-[2.75rem] w-full items-center justify-center gap-2 rounded-control text-sm text-carbon-text hover:bg-carbon-hover motion-safe:active:scale-[var(--motion-press-scale)] disabled:opacity-60 glim-hue ${
        tone === "neutral" ? "bg-carbon-surface3" : "bg-carbon-surface2"
      }`}
      style={hueVars(hueIndex ?? 0) as CSSProperties}
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
  // return; hooks rules). Gating the live section below, nothing else: the
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

  // Lazy parent-fetch on expand; the RestorePanel.tsx shape (list →
  // {ok, files} → tree props; #129 server reason over the generic message).
  // The cancelled flag is the late-response guard: hosts keep one mounted
  // sheet and swap its `run` prop (the Dashboard's run-sheet host), so the
  // effect re-runs per run identity and a slow listing for the previous run
  // must not overwrite the next run's tree when it finally lands.
  useEffect(() => {
    if (!open || !browseOpen) return;
    setFilesLoading(true);
    setFilesError(null);
    let cancelled = false;
    const req =
      run.domain === "container"
        ? listSnapshotFiles(run.targetId, run.snapshotId)
        : listSnapshotFilesFileSet(run.targetId, run.snapshotId);
    req
      .then((res) => {
        if (cancelled) return;
        if (res.ok) setFiles(res.files ?? []);
        else setFilesError(loadErrorMessage(res, t("files.loadFailed")));
      })
      .catch(() => {
        if (cancelled) return;
        setFilesError(t("files.loadFailed"));
      })
      .finally(() => {
        if (!cancelled) setFilesLoading(false);
      });
    return () => {
      cancelled = true;
    };
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
  // A check that resolves after the sheet closed must not write its outcome
  // onto whatever run the host shows next: "open a VM run, see the container
  // repo's red failure line". The refs are
  // runVerify's shape of the browse effect's `let cancelled` above: a ref,
  // because runVerify is a callback rather than an effect with a cleanup.
  // verifyCancelledRef is flipped by the close/run-swap effects below and
  // re-checked after every await, before any setState.
  //
  // The busy state is the check's own, kept apart from verifyState, which the
  // close resets: a check that outlives the sheet has to keep the row busy
  // when the row comes back, or the row advertises itself as ready while the
  // guard refuses the press. The server is doing the work either way.
  const verifyCancelledRef = useRef(false);
  const [verifyBusy, setVerifyBusy] = useState(false);

  async function runVerify() {
    if (!verifyDomain || verifyBusy) return;
    setVerifyBusy(true);
    verifyCancelledRef.current = false;
    setVerifyState("busy");
    setVerifyError(null);
    try {
      const res = await checkDomain(verifyDomain);
      if (verifyCancelledRef.current) return;
      if (res.ok) {
        setVerifyState("ok");
      } else {
        setVerifyState("fail");
        setVerifyError(res.error ?? t("verify.failed"));
      }
    } catch (err) {
      if (verifyCancelledRef.current) return;
      setVerifyState("fail");
      setVerifyError(err instanceof Error ? err.message : t("verify.failed"));
    } finally {
      setVerifyBusy(false);
    }
  }

  // --- restore row (the selection the tree collects, restored in place) -----
  // The sheet's own restore path: the SnapshotFileTree selection restored to
  // the original locations, behind the same confirm gate the desktop restore
  // surfaces answer (Files.tsx / RestorePanel.tsx, files.restoreConfirm).
  // Round 1 shipped the row as a second toggle of the browse disclosure, so
  // two adjacent buttons promised a restore while one disclosure answered
  // for both, and pressing "Restore" with the tree open CLOSED it
  // and pressing "Restore" with the tree open closed it. The row restores the selection,
  // and with nothing selected it only ever opens the tree: the selection is
  // this path's input; it never closes it. The same cancel/in-flight ref
  // pair as the verify row above, for the identical reason: an ack landing
  // after a close must not paint the next run's sheet.
  const [restoring, setRestoring] = useState(false);
  const [restoreResult, setRestoreResult] = useState<{ started: boolean; text: string } | null>(null);
  const restoreCancelledRef = useRef(false);
  const restoreInFlightRef = useRef(false);
  const { confirm, confirmDialog } = useConfirm();

  async function restoreSelection() {
    if (restoring || restoreInFlightRef.current) return;
    if (selected.size === 0) {
      setBrowseOpen(true);
      return;
    }
    if (!(await confirm(t("files.restoreConfirm")))) return;
    restoreInFlightRef.current = true;
    restoreCancelledRef.current = false;
    setRestoring(true);
    setRestoreResult(null);
    try {
      const paths = [...selected];
      const res =
        run.domain === "container"
          ? await restoreContainerFiles(run.target, run.snapshotId, paths, "", true)
          : await restoreFileSetFiles(run.targetId, run.snapshotId, paths, "", true);
      if (restoreCancelledRef.current) return;
      // The server runs the restore detached, so the ack says it started and
      // nothing more. Claiming success here painted a green line over a
      // restore that was still copying, and left it green when it failed. The
      // outcome arrives where outcomes live: this sheet's own live section
      // (the restore publishes under the same key) and the run list behind it.
      setRestoreResult(
        res.ok
          ? { started: true, text: t("restore.started") }
          : { started: false, text: loadErrorMessage(res, t("files.loadFailed")) }
      );
    } catch (err) {
      if (restoreCancelledRef.current) return;
      setRestoreResult({ started: false, text: err instanceof Error ? err.message : t("files.loadFailed") });
    } finally {
      restoreInFlightRef.current = false;
      // Unconditionally: the cancel ref already drops the result write, and
      // leaving the flag set disabled the row for the life of the mount.
      setRestoring(false);
    }
  }

  // --- fresh-success CheckDraw gate -----------------------------------------
  // CheckDraw's own contract (CheckDraw.tsx): it may only appear at the exact
  // moment state transitions busy → fresh success, never at initial mount;
  // a sheet opened on an old successful run must not animate a checkmark as
  // if the run just completed. One ref-carrying effect watches for that
  // transition; but only counts it while the sheet is open: every host keeps
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
  // consumer reusing the same mount for a different run starts clean; the
  // check gate included: a witnessed animation belongs to the open session
  // that saw the transition, never to the next one. The cancel refs flip
  // here too: in-flight verify/restore writes are dropped, not delivered
  // into the reset (freshly cleaned) state.
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
    setRestoreResult(null);
    setRestoring(false);
    verifyCancelledRef.current = true;
    restoreCancelledRef.current = true;
  }, [open]);

  // A run swap by the host (the single-mounted-sheet contract) clears the same
  // things a close does: an outcome must never surface under the next run's
  // title, and a check that was in flight at the swap must not leave the row
  // reading "Checking…" for a run it was never about. Keyed on the run's
  // identity, so a parent refetch that re-creates the Run object clears
  // nothing.
  useEffect(() => {
    verifyCancelledRef.current = true;
    restoreCancelledRef.current = true;
    setVerifyState("idle");
    setVerifyError(null);
    setVerifyBusy(false);
    setRestoreResult(null);
    setRestoring(false);
  }, [run.id]);

  if (!open) return null;

  // Stat tiles; the Run-record substitutes only (see the header note).
  // Duration falls back to the same muted absent-data mark formatTs uses
  // for a missing timestamp rather than a blank tile, so the absence reads
  // as intentional.
  const durationSecs = run.finishedAt != null ? run.finishedAt - run.startedAt : null;
  const durationText = durationSecs == null ? "—" : formatDuration(durationSecs) || "—";
  const durationMissing = durationText === "—";
  // Runs with no snapshot (the Backup Everything parent, prune, verify) have
  // no volume and no snapshot id to show. humanBytes(0) would claim a
  // measured "0 B" and an empty mono slice would render a blank tile; both
  // read as data, not as absence. The placeholder is the same muted mark
  // the duration tile uses. A real zero-byte backup that has a snapshot
  // keeps its honest "0 B": the snapshot exists, the number is true.
  const hasSnapshot = run.snapshotId !== "";
  const volumeText = hasSnapshot ? humanBytes(run.bytes) : "—";
  const volumeMissing = !hasSnapshot;

  // No horizontal padding on either slot wrapper (footer below, body in the
  // JSX): the BottomSheet slots already inset-clamp every side to
  // max(1rem, safe-area), so a wrapper px-4 doubled the edge to 32px, which is 16px
  // past every other sheet surface, worse in landscape where the clamp has
  // widened to the cutout. The inner
  // rounded cards keep their own px-4: they inset from the slot, not from
  // the screen.
  const footerContent =
    browseSupported || verifyDomain ? (
      <div className="flex flex-col gap-2 py-4">
        <div className={browseSupported && verifyDomain ? "grid grid-cols-2 gap-2" : "flex"}>
          {browseSupported && (
            <SheetActionRow
              label={t("recovery.foreignStepBrowse")}
              onClick={() => setBrowseOpen((o) => !o)}
              expanded={browseOpen}
              controlsId={browseId}
              hueIndex={0}
            />
          )}
          {verifyDomain && (
            <SheetActionRow
              label={verifyBusy ? t("integrity.checking") : t("integrity.verify")}
              onClick={() => void runVerify()}
              disabled={verifyBusy}
              tone="neutral"
              hueIndex={1}
            />
          )}
        </div>
        {/* The verify row's own result, adjacent to its control; the same
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
      // Title composes from existing keys only (runKindLabel + runTargetText;
      // no new title key). The interpunct separator keeps user text free of
      // dashes (the em-dash ban is lint-enforced and a hyphen reads as a
      // hyphenation, not a separator).
      title={`${runKindLabel(t, run.kind)} · ${runTargetText(t, run)}`}
      footer={footerContent}
    >
      <div className="flex flex-col gap-4 py-4">
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

        {/* Failed path; the backend's already-scrubbed reason verbatim
            (runReason translates only BombVault's own sentences; untranslated
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

        {/* Stat triad: stat values take the heading role at weight 600
            (text-heading font-semibold, the Fab label's typography) with
            tabular numerals so values don't shimmer while a rerender moves
            digits; the snapshot tile is mono with the full id as its title
            (BackupButton toast precedent). Weight law: 400/600 only; the
            house weight discipline. */}
        <div className="grid grid-cols-3 gap-2">
          <div className="flex min-w-0 flex-col gap-1 rounded-card bg-carbon-background px-2 py-2">
            <span className="text-xs text-carbon-textMuted">{t("run.statVolume")}</span>
            <span
              className={`truncate text-heading font-semibold tabular-nums ${volumeMissing ? "text-carbon-textMuted" : ""}`}
            >
              {volumeText}
            </span>
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
            <span
              className={`truncate font-mono text-heading font-semibold tabular-nums ${hasSnapshot ? "" : "text-carbon-textMuted"}`}
              title={hasSnapshot ? run.snapshotId : undefined}
            >
              {hasSnapshot ? run.snapshotId.slice(0, 8) : "—"}
            </span>
          </div>
        </div>

        {/* Live section; in-flight runs get the progress bar + live log tail,
            conditionally on the visibility gate: hiding the
            page unmounts this subtree, which is the unsubscribe; the
            singleton's ref-count drops the shared EventSource and drops its
            cached state; showing the page remounts + resubscribes into the
            backend's snapshot replay. While hidden the slot renders nothing
            (an in-flight run has no history line to fall back to, and nobody
            is watching). A run that finished while hidden arrives as a
            refetched `run` record from the consumer; the terminal flip is
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

        {/* Browse section; the SnapshotFileTree mount (the Recovery.tsx /
            Files.tsx precedent: purely presentational tree, the parent
            owns the fetch). Revealed by the footer browse disclosure, or
            opened by the restore entry below when it needs a selection. */}
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

        {/* Restore entry; in the scroll body, above the footer rows, so it
            sits away from the thumb's default path: a restore is a decision.
            The row restores the tree's selection in place behind the desktop
            confirm gate (restoreSelection above); it carries no
            aria-expanded/aria-controls: the footer's Browse & restore row
            is the section's one disclosure owner, and a second control
            answering "expanded" about a different action read as a
            duplicate disclosure to a screen reader. */}
        {browseSupported && (
          <SheetActionRow
            label={t("snapshots.restore")}
            onClick={() => void restoreSelection()}
            disabled={restoring}
            tone="neutral"
            hueIndex={2}
          />
        )}
        {/* The restore's own line, adjacent to its control; the same shape as
            the verify result below the footer rows. A started restore is
            muted, not green: the tick belongs to the recorded run, which the
            live section above and the run list behind the sheet both carry. */}
        {restoreResult != null && (
          <p
            className={`text-xs leading-relaxed wrap-break-word ${
              restoreResult.started ? "text-carbon-textSub" : "text-statusFail"
            }`}
          >
            {restoreResult.text}
          </p>
        )}
        {/* The restore confirm renders through its own portal; nested here
            so the sheet's JSX owns its lifecycle (the sheet-stack keyboard
            contract makes a confirm over a sheet safe). */}
        {confirmDialog}
      </div>
    </BottomSheet>
  );
}
