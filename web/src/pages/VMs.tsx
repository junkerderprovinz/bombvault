import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { listVMs, backupVMNow, restoreVM, listVMSnapshots, setVMInclude, setVMIncludeAll, setVMMethod, deleteSnapshot, deleteBackupsVM, forgetVM, discoverVMs, exportVM, getVmBackupOrder, setVmBackupOrder } from "../lib/api";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { FilterPopover } from "../components/FilterPopover";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import type { VM, Snapshot, VmOrder } from "../lib/api";
import { useT, stateLabel } from "../lib/i18n";
import { useDragReorder } from "../lib/useDragReorder";
import { Advanced } from "../lib/advanced";
import { ProgressBar } from "../components/ProgressBar";
import { RestoreAction } from "../components/restore/RestoreAction";
import { RecentRunsList } from "../components/RecentRunsList";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconVM } from "../components/Sidebar";
import { Badge, type BadgeTone } from "../components/Badge";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch, fireAndWaitRun } from "../lib/backupWatch";
import { useConfirm } from "../lib/useConfirm";
import { hueVars, rainbowAt } from "../lib/appearance";
import { useRainbow } from "../lib/useRainbow";
import { Selector } from "../components/Selector";

type T = ReturnType<typeof useT>["t"];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

// ---------------------------------------------------------------------------
// State chip (mirrors Containers.tsx) — stateTone maps a raw VM state to the
// shared Badge's tone; stateLabel (lib/i18n) still does the actual
// state->text translation.
// ---------------------------------------------------------------------------

function stateTone(state: string): BadgeTone {
  const lower = state.toLowerCase();
  if (lower === "running") return "ok";
  if (lower === "shut off" || lower === "shutoff" || lower === "stopped") return "fail";
  return "neutral";
}

// ---------------------------------------------------------------------------
// Sort control
// ---------------------------------------------------------------------------

type SortKey = "name" | "status";

const SORT_STORAGE_KEY = "bv-vms-sort";

function loadSortKey(): SortKey {
  const v = localStorage.getItem(SORT_STORAGE_KEY);
  if (v === "name" || v === "status") return v;
  return "name";
}

function sortVMs(vms: VM[], key: SortKey): VM[] {
  const copy = [...vms];
  switch (key) {
    case "name":
      return copy.sort((a, b) =>
        a.name.localeCompare(b.name, undefined, { sensitivity: "base" })
      );
    case "status": {
      const rank = (v: VM) => (v.state.toLowerCase() === "running" ? 0 : 1);
      return copy.sort((a, b) => {
        const r = rank(a) - rank(b);
        if (r !== 0) return r;
        return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
      });
    }
  }
}

const SORT_KEYS = {
  name: "sort.nameAsc",
  status: "sort.status",
} as const;

// SortControl/ChipFilter below are thin, page-specific adapters onto the
// shared Selector component (GlimStone form-engine Phase 2, Task 3) — the
// actual button rendering, keyboard nav (roving tabindex, arrow keys/Home/
// End, RTL) and rainbow hueing all live in Selector now, mirroring
// Containers.tsx's own identical adapter pair.
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
        items={(["name", "status"] as SortKey[]).map((k) => ({ id: k, label: t(SORT_KEYS[k]) }))}
        label={t("sort.label")}
        select="one"
        active={value}
        onChange={(id) => onChange(id as SortKey)}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Schedule / backup chip filters (#41)
// ---------------------------------------------------------------------------
// Generic sibling of the sort chips: same chip look + localStorage pattern, but
// parameterised over its option set so the schedule and backup dimensions each
// instantiate it without duplicating the markup. Mirrors Containers.tsx's
// ChipFilter. VMs have NO installed/not-installed FilterControl — the state-
// based live/orphans split already covers that dimension.

type ScheduleFilterKey = "all" | "scheduled" | "notScheduled";
type BackupFilterKey = "all" | "backedUp" | "neverBackedUp";

const SCHEDULE_FILTER_STORAGE_KEY = "bv-vms-schedule-filter";
const BACKUP_FILTER_STORAGE_KEY = "bv-vms-backup-filter";

function loadScheduleFilterKey(): ScheduleFilterKey {
  const v = localStorage.getItem(SCHEDULE_FILTER_STORAGE_KEY);
  if (v === "all" || v === "scheduled" || v === "notScheduled") return v;
  return "all";
}

function loadBackupFilterKey(): BackupFilterKey {
  const v = localStorage.getItem(BACKUP_FILTER_STORAGE_KEY);
  if (v === "all" || v === "backedUp" || v === "neverBackedUp") return v;
  return "all";
}

function ChipFilter<K extends string>({
  label,
  options,
  value,
  onChange,
}: {
  label: string;
  options: { key: K; label: string }[];
  value: K;
  onChange: (k: K) => void;
}) {
  return (
    <div className="flex items-center gap-2 flex-wrap">
      <span className="text-xs text-carbon-textMuted">{label}</span>
      <Selector
        items={options.map((o) => ({ id: o.key, label: o.label }))}
        label={label}
        select="one"
        active={value}
        onChange={(id) => onChange(id as K)}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// VM-aware IncludeToggle variant
// ---------------------------------------------------------------------------

// VMMethodSelect picks the per-VM backup method (graceful shutdown vs live
// snapshot) via PATCH /api/vms/{name}.
function VMMethodSelect({
  name,
  initial,
  t,
}: {
  name: string;
  initial: string;
  t: ReturnType<typeof useT>["t"];
}) {
  const [method, setMethod] = useState(initial || "graceful");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleChange(next: string) {
    setBusy(true);
    setError(null);
    try {
      const res = await setVMMethod(name, next);
      if (res.ok) {
        setMethod(next);
      } else {
        // Surface the failure instead of silently reverting — a swallowed error
        // here means the user thinks they switched to live (no downtime) when the
        // VM will actually be shut down at backup time.
        setError(res.error ?? t("vm.method.saveFailed"));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("vm.method.saveFailed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <select
        value={method}
        disabled={busy}
        onChange={(e) => void handleChange(e.target.value)}
        title={t("vm.method.hint")}
        className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text bv-field-focus disabled:opacity-50"
      >
        <option value="graceful">{t("vm.method.graceful")}</option>
        <option value="live">{t("vm.method.live")}</option>
      </select>
      {error && (
        <span className="text-xs text-statusFail max-w-48 text-end leading-tight">
          {error}
        </span>
      )}
    </div>
  );
}

function VMIncludeToggle({
  name,
  initial,
}: {
  name: string;
  initial: boolean;
}) {
  const { t } = useT();
  const [enabled, setEnabled] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Re-seed when the parent passes a fresh value (e.g. after "Include all in
  // schedule" reloads the list). Rows are keyed by name and do not remount, so
  // without this the toggle would keep showing its stale pre-bulk state.
  useEffect(() => setEnabled(initial), [initial]);

  async function handleChange(next: boolean) {
    setBusy(true);
    setError(null);
    try {
      const res = await setVMInclude(name, next);
      if (res.ok) {
        setEnabled(next);
      } else {
        setError(res.error ?? t("schedule.updateFailed"));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("schedule.updateFailed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <button
        role="switch"
        aria-label={t("containers.includeInSchedule")}
        aria-checked={enabled}
        disabled={busy}
        onClick={() => void handleChange(!enabled)}
        title={t("containers.includeInSchedule")}
        className={`relative inline-flex h-5 w-9 shrink-0 items-center rounded-pill transition-colors focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--focus-ring) disabled:opacity-50 ${
          enabled ? "bg-accent" : "bg-carbon-surface3"
        }`}
      >
        <span
          className={`inline-block h-3.5 w-3.5 rounded-full bg-carbon-background transition-transform ${
            enabled ? "translate-x-[18px]" : "translate-x-[3px]"
          }`}
        />
      </button>
      {error && (
        <span className="text-xs text-statusFail max-w-48 text-end leading-tight">
          {error}
        </span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// VM-aware BackupButton variant
// ---------------------------------------------------------------------------

function VMExportButton({ name, t }: { name: string; t: T }) {
  const [state, setState] = useState<"idle" | "pending" | "done" | "error">("idle");
  const [msg, setMsg] = useState<string | null>(null);
  async function run() {
    setState("pending");
    setMsg(null);
    try {
      const r = await exportVM(name);
      if (r.ok) {
        setState("done");
        setMsg(r.path ?? null);
      } else {
        setState("error");
        setMsg(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setState("error");
      setMsg(err instanceof Error ? err.message : t("settings.error"));
    }
  }
  return (
    <div className="flex flex-col items-start gap-1">
      <button
        onClick={() => void run()}
        disabled={state === "pending"}
        className="inline-flex items-center gap-1.5 rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors disabled:opacity-50"
      >
        {state === "pending" ? "…" : t("export.button")}
      </button>
      {state === "done" && (
        <span className="text-xs text-statusOk break-all">{t("export.exportedTo")} {msg}</span>
      )}
      {state === "error" && <span className="text-xs text-statusFail break-all">{msg}</span>}
    </div>
  );
}

function VMBackupButton({
  name,
  t,
  onBackedUp,
  running,
}: {
  name: string;
  t: T;
  onBackedUp?: () => void;
  /** "Something is running" signal (anyActive): busy-guards this backup while
   *  another op runs, but never for its OWN in-flight backup (isPending). */
  running?: { active: boolean; phase?: string };
}) {
  // Fire-and-watch (see useBackupWatch): the server backs the VM up detached and
  // answers immediately, so we watch the "vm:<name>" progress + recorded run for
  // the outcome instead of awaiting the whole backup.
  const { state, fire, isPending } = useBackupWatch({
    progressKey: `vm:${name}`,
    start: () => backupVMNow(name),
    matchRun: (r) => r.domain === "vm" && r.target === name,
    onDone: onBackedUp,
  });
  const blockedByOther = !!running?.active && !isPending;

  return (
    <div className="flex flex-col gap-1 items-start">
      <button
        onClick={() => void fire()}
        disabled={isPending || blockedByOther}
        className="inline-flex items-center gap-1.5 rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {isPending ? (
          <>
            <span
              className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
            {t("common.backingUp")}
          </>
        ) : (
          t("containers.backupNow")
        )}
      </button>
      {/* A backup/restore/replication elsewhere blocks a new VM backup — say why. */}
      {blockedByOther && (
        <span className="text-xs text-carbon-textMuted">{t(busyPhraseKey(running?.phase))}</span>
      )}
      {/* Plain export is an advanced-only extra. */}
      <Advanced><VMExportButton name={name} t={t} /></Advanced>
      {state.phase === "success" && (
        <span className="text-xs text-statusOk">
          ✓ {t("common.done")}
          {state.phase === "success" && state.snapshotId && (
            <span dir="ltr" className="font-mono ms-1 text-start text-carbon-textMuted">
              {state.snapshotId.slice(0, 8)}
            </span>
          )}
        </span>
      )}
      {state.phase === "error" && (
        <span className="text-xs text-statusFail max-w-[18rem] wrap-break-word">
          {state.message}
        </span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// VM-aware RestorePanel variant
// ---------------------------------------------------------------------------

function VMSnapshotRow({
  snap,
  vmName,
  vmDisplayName,
  source,
  onDeleted,
  t,
}: {
  snap: Snapshot;
  /** Raw libvirt name — drives the progress key and the restore action. */
  vmName: string;
  /** Display name for the cancel-confirm text; falls back to vmName. */
  vmDisplayName?: string;
  source: RepoSource;
  onDeleted: () => void;
  t: T;
}) {
  const progressMap = useProgress();
  // Busy-guard handed to the shared RestoreAction: block a new restore while any
  // OTHER backup/restore/replication runs (this VM's own in-flight restore is
  // covered inside RestoreAction via isPending, never self-blocked).
  const running = anyActive(progressMap);
  // The delete button is guarded only against THIS VM's own in-flight
  // backup/restore, not any global activity — deleting VM A's snapshot must stay
  // available while VM B is backing up.
  const busy = progressMap[`vm:${vmName}`]?.active ?? false;
  const [deleting, setDeleting] = useState(false);
  const [deleteErr, setDeleteErr] = useState<string | null>(null);
  // Collapsed by default so the list stays compact (mirrors Containers'
  // SnapshotRow) — the restore controls (confirm + leave-stopped + progress
  // banner) only render once the user opts in.
  const [showRestore, setShowRestore] = useState(false);
  const { confirm, confirmDialog } = useConfirm();

  async function handleDelete() {
    if (!(await confirm(t("snapshots.deleteConfirm")))) return;
    setDeleting(true);
    setDeleteErr(null);
    try {
      const res = await deleteSnapshot("vms", snap.id, source);
      if (res.ok) onDeleted();
      else setDeleteErr(res.error ?? "Delete failed");
    } catch (err) {
      setDeleteErr(err instanceof Error ? err.message : "Delete failed");
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="flex flex-col gap-1 py-2.5 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        <span dir="ltr" className="font-mono text-start text-carbon-text text-xs w-20 shrink-0">
          {snap.id.slice(0, 8)}
        </span>
        <span className="text-carbon-textMuted text-xs flex-1">
          {new Date(snap.time).toLocaleString()}
        </span>
        {snap.tags && snap.tags.length > 0 && (
          <span className="text-carbon-textMuted text-xs hidden sm:block">
            {snap.tags.join(", ")}
          </span>
        )}
        {/* Compact restore toggle: opens the inline RestoreAction panel below
            (mirrors Containers' SnapshotRow) instead of always rendering it. */}
        <button
          onClick={() => setShowRestore((p) => !p)}
          className={`shrink-0 rounded-control px-2.5 py-1 text-xs transition-colors ${
            showRestore ? "bg-carbon-surface3 text-carbon-text" : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
          }`}
        >
          {t("restore.open")}
        </button>
        <button
          onClick={() => void handleDelete()}
          disabled={deleting || busy}
          title={t("snapshots.delete")}
          className="shrink-0 rounded-control px-2 py-1 text-xs text-carbon-textSub hover:bg-statusFailBg hover:text-statusFail transition-colors disabled:opacity-50"
        >
          {deleting ? "…" : t("snapshots.delete")}
        </button>
      </div>
      {/* Restore control (confirm + leave-stopped + progress banner), indented
          under the id column (ps-24, LOGICAL — the row's content column sits
          at the reading-direction start, not a fixed physical left) to match
          the row's content alignment. Only rendered once the user opts in via
          the toggle above. */}
      {showRestore && (
        <div className="ps-24">
          <RestoreAction
            domain="vm"
            name={vmName}
            displayName={vmDisplayName}
            snapshotId={snap.id}
            source={source}
            otherActive={running}
            successMessage={t("restore.completeVM")}
            t={t}
          />
        </div>
      )}
      {deleteErr && <p className="text-xs text-statusFail ps-24 wrap-break-word">{deleteErr}</p>}
      {confirmDialog}
    </div>
  );
}

function VMRestorePanel({
  name,
  displayName,
  t,
}: {
  /** Raw libvirt name — every call in this panel (snapshots, delete-all,
   *  recent runs, restore) MUST use this, never displayName. */
  name: string;
  /** Display name shown in the restore cancel-confirm text; falls back to
   *  name. */
  displayName?: string;
  t: T;
}) {
  const [open, setOpen] = useState(false);
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [reloadTick, setReloadTick] = useState(0);
  const [deletingAll, setDeletingAll] = useState(false);
  const { confirm, confirmDialog } = useConfirm();

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    setError(null);
    listVMSnapshots(name, source)
      .then((res) => {
        if (res.ok) setSnapshots(res.snapshots ?? []);
        else setError(res.error ?? "Failed to load backups");
      })
      .catch(() => setError("Failed to load backups"))
      .finally(() => setLoading(false));
  }, [open, name, source, reloadTick]);

  async function handleDeleteAll() {
    // TODO(#follow-up): richer stake-detail copy ("N snapshots, X GB") belongs
    // here once it ships (deferred — new interpolated i18n keys across all 25
    // non-English locales, out of scope for this window.confirm() → dialog
    // mechanism swap). Same flagged follow-up as Containers.tsx's
    // deleteBackupsConfirm and Files.tsx's deleteBackupsConfirm.
    if (!(await confirm(t("snapshots.deleteAllConfirm")))) return;
    setDeletingAll(true);
    setError(null);
    deleteBackupsVM(name, source)
      .then((res) => {
        if (!res.ok) setError(res.error ?? "Failed to delete backups");
      })
      .catch(() => setError("Failed to delete backups"))
      .finally(() => {
        setDeletingAll(false);
        setReloadTick((n) => n + 1);
      });
  }

  return (
    <div className="mt-1">
      <button
        onClick={() => setOpen((prev) => !prev)}
        className="flex items-center gap-1.5 text-xs text-carbon-textSub hover:text-carbon-text transition-colors"
      >
        <svg
          width="12"
          height="12"
          viewBox="0 0 12 12"
          fill="none"
          // Disclosure chevron (RTL sweep, form-engine Phase 2 Task 6):
          // closed points to the reading-direction start — right in LTR
          // (the base, unrotated path), left in RTL via `rtl:rotate-180`
          // (Tailwind v4's built-in `:dir(rtl)` variant, no config needed).
          // Open always rotates to straight-down (`rotate-90`) in BOTH
          // directions — "there is more below" has no reading direction, so
          // it never gets the rtl: treatment. This is plain rotation, not
          // scaleX mirroring: mirroring the closed glyph and then rotating
          // it 90° lands on "pointing up", not "pointing down" (transform
          // functions compose right-to-left), so a second rtl:-only rotate
          // angle is the correct fix, not a mirror.
          className={`transition-transform ${open ? "rotate-90" : "rtl:rotate-180"}`}
        >
          <path
            d="M4 2l4 4-4 4"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        {t("snapshots.title")}
      </button>

      {open && (
        <div className="mt-2 rounded-card bg-carbon-background px-3 py-1">
          <div className="flex flex-col gap-1 py-2 border-b border-carbon-border">
            <div className="flex items-center gap-2">
              {/* Source (Local / Off-site) toggle is advanced; basic mode uses local. */}
              <Advanced>
                <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
                <SourceToggle source={source} onChange={setSource} disabled={loading} domain="vms" />
              </Advanced>
              {snapshots.length > 0 && (
                // Task 5 (rule 13): was a plain underline-on-hover text
                // button; already correctly fault-red per "the destructive
                // control is always the fault colour" (Destructive actions).
                <Badge
                  as="button"
                  onClick={() => void handleDeleteAll()}
                  disabled={deletingAll || loading}
                  tone="fail"
                  size="small"
                  className="ms-auto"
                >
                  {deletingAll ? t("snapshots.deletingAll") : t("snapshots.deleteAll")}
                </Badge>
              )}
            </div>
            <p className="text-[11px] text-carbon-textMuted">{t("source.hint")}</p>
          </div>
          <RecentRunsList name={name} domain="vm" t={t} />
          {loading && (
            <p className="py-3 text-xs text-carbon-textMuted">{t("common.loadingBackups")}</p>
          )}
          {error && (
            <p className="py-3 text-xs text-statusFail">{error}</p>
          )}
          {!loading && !error && snapshots.length === 0 && (
            <p className="py-3 text-xs text-carbon-textMuted">{t("snapshots.none")}</p>
          )}
          {!loading &&
            snapshots.map((snap) => (
              <VMSnapshotRow
                key={snap.id}
                snap={snap}
                vmName={name}
                vmDisplayName={displayName}
                source={source}
                onDeleted={() => setReloadTick((n) => n + 1)}
                t={t}
              />
            ))}
        </div>
      )}
      {confirmDialog}
    </div>
  );
}

// ---------------------------------------------------------------------------
// VM row
// ---------------------------------------------------------------------------

// Exported (only) so VMs.test.tsx can render one row directly and assert its
// action buttons wire up VM.libvirtName — never the display-only VM.name — to
// the backend calls. See that test's header comment for the bug it pins.
export function VMRow({
  vm,
  t,
  onRefresh,
  selected,
  onToggleSelect,
  index,
}: {
  vm: VM;
  t: T;
  onRefresh: () => void;
  selected?: boolean;
  onToggleSelect?: () => void;
  /** Position in the rendered list — the rainbow palette position (GlimStone
   *  form-engine Phase 2, Task 2). Assigned by LIST INDEX, never a hash of
   *  `vm.libvirtName` — see the callers below. */
  index: number;
}) {
  const installed = vm.state !== "not-installed";
  const progressMap = useProgress();
  // Progress keys are published server-side off the raw libvirt name (see
  // "vm:"+name in internal/api/service.go) — never the display vm.name.
  const progress = progressMap[`vm:${vm.libvirtName}`];
  // "Something is running" across any domain — busy-guards this row's own VM
  // backup (its OWN in-flight backup is handled by isPending inside the button).
  const running = anyActive(progressMap);
  return (
    <div
      style={hueVars(rainbowAt(index)) as CSSProperties}
      // glim-tint washes the card (trap #2 — without it this card shows
      // almost no colour at rest); glim-active while THIS row's own
      // backup/restore is actively running, so reactive mode shows the hue
      // without needing hover — mirrors ContainerRow's identical treatment.
      className={`relative overflow-hidden bg-carbon-surface rounded-card p-4 flex flex-col gap-3 glim-hue glim-tint ${
        progress?.active ? "glim-active" : ""
      }`}
    >
      {/* Top row */}
      <div className="flex items-start gap-3 flex-wrap">
        {/* Multi-select checkbox (installed VMs only) */}
        {onToggleSelect && (
          <input
            type="checkbox"
            checked={!!selected}
            onChange={onToggleSelect}
            aria-label={`Select ${vm.name}`}
            className="mt-1 h-4 w-4 shrink-0 cursor-pointer"
            style={{ accentColor: "var(--accent)" }}
          />
        )}
        {/* Name + state */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm min-w-0 truncate">
              {vm.name}
            </span>
            {installed ? (
              <Badge tone={stateTone(vm.state)}>{stateLabel(t, vm.state)}</Badge>
            ) : (
              <Badge tone="neutral">{t("containers.notInstalled")}</Badge>
            )}
          </div>
        </div>

        {/* Last backup */}
        <div className="text-end shrink-0">
          <p className="text-xs text-carbon-textMuted">{t("containers.lastBackup")}</p>
          <p className="text-xs text-carbon-textSub">
            {vm.lastBackup ? formatTs(vm.lastBackup) : t("containers.never")}
          </p>
        </div>
      </div>

      {/* Actions row */}
      {installed && (
        <div className="flex items-start justify-between gap-4 flex-wrap">
          <div className="flex items-center gap-4 flex-wrap">
            <label className="flex items-center gap-2 cursor-pointer">
              <VMIncludeToggle name={vm.libvirtName} initial={vm.includeInSchedule} />
              <span className="text-xs text-carbon-textSub">
                {t("containers.includeInSchedule")}
              </span>
            </label>
            {/* Backup method (graceful / live) — always visible; it decides VM downtime. */}
            <label className="flex items-center gap-2">
              <span className="text-xs text-carbon-textSub">{t("vm.method")}</span>
              <VMMethodSelect name={vm.libvirtName} initial={vm.method} t={t} />
            </label>
          </div>
          <div className="ms-auto flex flex-col items-end">
            <VMBackupButton name={vm.libvirtName} t={t} onBackedUp={onRefresh} running={running} />
          </div>
        </div>
      )}

      {/* Not installed: offer to clear the stale entry (also stops the scheduler
          retrying a deleted VM). Deleting actual backups stays in the panel below. */}
      {!installed && (
        <div className="flex justify-end">
          <VMForgetButton name={vm.libvirtName} t={t} onForgotten={onRefresh} />
        </div>
      )}

      {/* Backups / Restore disclosure */}
      <VMRestorePanel name={vm.libvirtName} displayName={vm.name} t={t} />

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

// VMForgetButton clears a no-longer-installed VM's stale entry (its target row),
// for a deleted VM that has no backups left — answering "how do I remove this".
function VMForgetButton({
  name,
  t,
  onForgotten,
}: {
  name: string;
  t: T;
  onForgotten: () => void;
}) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { confirm, confirmDialog } = useConfirm();

  async function handleForget() {
    if (!(await confirm(t("vms.removeEntryConfirm")))) return;
    setPending(true);
    setError(null);
    try {
      const res = await forgetVM(name);
      if (res.ok) onForgotten();
      else setError(res.error ?? "Remove failed");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Remove failed");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <button
        onClick={() => void handleForget()}
        disabled={pending}
        className="inline-flex items-center gap-2 rounded-control bg-statusFailBg px-3 py-1.5 text-xs font-medium text-statusFail hover:bg-statusFailBgHover transition-colors disabled:opacity-50"
      >
        {pending ? t("dashboard.checking") : t("vms.removeEntry")}
      </button>
      {error && <p className="text-xs text-statusFail">{error}</p>}
      {confirmDialog}
    </div>
  );
}

// ScheduleIncludeAllControl is the one-click header control: "Include all in
// schedule" / "Exclude all" for every known VM, refreshing the list so each
// row's include toggle reflects the new state.
function ScheduleIncludeAllControl({
  t,
  onChanged,
}: {
  t: T;
  onChanged: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function run(include: boolean) {
    setBusy(true);
    setError(null);
    try {
      const res = await setVMIncludeAll(include);
      if (res.ok) onChanged();
      else setError(res.error ?? t("settings.error"));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("settings.error"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex items-center gap-2 flex-wrap">
      <button
        onClick={() => void run(true)}
        disabled={busy}
        className="inline-flex items-center rounded-control bg-accent px-3 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
      >
        {t("schedule.includeAll")}
      </button>
      <button
        onClick={() => void run(false)}
        disabled={busy}
        className="inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50"
      >
        {t("schedule.excludeAll")}
      </button>
      {error && <span className="text-xs text-statusFail">{error}</span>}
    </div>
  );
}

// ---------------------------------------------------------------------------
// VM backup-order panel (#119, VMs) — manual per-VM scheduled-run sequence
// ---------------------------------------------------------------------------

const VM_BACKUP_ORDER_COLLAPSED_KEY = "bombvault.vmBackupOrderCollapsed";

// VMBackupOrderPanel lets the user arrange the order the scheduled VM run backs
// VMs up in (mirrors the container BackupOrderPanel, sharing useDragReorder). The
// orderable set is the schedule-included VMs. It hydrates once from the persisted
// order (GET /api/vms/backup-order), then reconciles as VMs come and go without
// discarding an in-progress reorder. Save PUTs the whole displayed sequence
// (authoritative); Reset PUTs an empty list, returning every VM to the name-order
// tiebreak.
//
// `names` (despite the name, kept for minimal diff against the container
// sibling) holds each VM's raw libvirtName — the value the backend persists
// and matches on (store.VMTarget rows are keyed by the raw name) — never the
// display vm.name. `displayByLibvirtName` resolves a row's libvirtName back
// to its friendly name for rendering and for the tiebreak sort, so a TrueNAS
// user still sees and alphabetizes by the readable name.
function VMBackupOrderPanel({ vms, t }: { vms: VM[]; t: T }) {
  const [savedOrder, setSavedOrder] = useState<VmOrder[] | null>(null);
  const [names, setNames] = useState<string[]>([]);
  const [saveState, setSaveState] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [error, setError] = useState<string | null>(null);
  const hydrated = useRef(false);
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem(VM_BACKUP_ORDER_COLLAPSED_KEY) === "1";
    } catch {
      return false;
    }
  });
  const displayByLibvirtName = useMemo(
    () => new Map(vms.map((v) => [v.libvirtName, v.name])),
    [vms]
  );

  useEffect(() => {
    getVmBackupOrder()
      .then((res) => setSavedOrder(res.ok ? res.order ?? [] : []))
      .catch(() => setSavedOrder([]));
  }, []);

  useEffect(() => {
    if (savedOrder === null) return;
    const orderable = vms.filter((v) => v.includeInSchedule).map((v) => v.libvirtName);
    const set = new Set(orderable);
    const byDisplayName = (a: string, b: string) =>
      (displayByLibvirtName.get(a) ?? a).localeCompare(displayByLibvirtName.get(b) ?? b, undefined, {
        sensitivity: "base",
      });
    if (!hydrated.current) {
      hydrated.current = true;
      const ranked = savedOrder
        .filter((o) => set.has(o.vm))
        .sort((a, b) => a.order - b.order)
        .map((o) => o.vm);
      const rest = orderable.filter((n) => !ranked.includes(n)).sort(byDisplayName);
      setNames([...ranked, ...rest]);
      return;
    }
    setNames((prev) => {
      const kept = prev.filter((n) => set.has(n));
      const added = orderable.filter((n) => !kept.includes(n)).sort(byDisplayName);
      const next = [...kept, ...added];
      return next.length === prev.length && next.every((n, i) => n === prev[i])
        ? prev
        : next;
    });
  }, [vms, savedOrder, displayByLibvirtName]);

  function move(index: number, dir: -1 | 1) {
    setNames((prev) => {
      const to = index + dir;
      if (to < 0 || to >= prev.length) return prev;
      const next = [...prev];
      [next[index], next[to]] = [next[to], next[index]];
      return next;
    });
    setSaveState("idle");
  }

  function reorder(from: number, to: number) {
    setNames((prev) => {
      if (from === to || to < 0 || to >= prev.length) return prev;
      const next = [...prev];
      const [moved] = next.splice(from, 1);
      next.splice(to, 0, moved);
      return next;
    });
    setSaveState("idle");
  }

  const { dragIndex, rowProps } = useDragReorder<HTMLLIElement>(reorder, saveState === "saving");

  function toggleCollapsed() {
    setCollapsed((v) => {
      const next = !v;
      try {
        localStorage.setItem(VM_BACKUP_ORDER_COLLAPSED_KEY, next ? "1" : "0");
      } catch {
        /* private mode / quota — collapse just won't persist */
      }
      return next;
    });
  }

  async function persist(order: string[]) {
    setSaveState("saving");
    setError(null);
    try {
      const res = await setVmBackupOrder(order);
      if (res.ok) {
        setSavedOrder(order.map((vm, i) => ({ vm, order: i + 1 })));
        setSaveState("saved");
        setTimeout(() => setSaveState("idle"), 3000);
      } else {
        setError(res.error ?? t("backupOrder.saveError"));
        setSaveState("error");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("backupOrder.saveError"));
      setSaveState("error");
    }
  }

  function clearOrder() {
    const sorted = [...names].sort((a, b) =>
      (displayByLibvirtName.get(a) ?? a).localeCompare(displayByLibvirtName.get(b) ?? b, undefined, {
        sensitivity: "base",
      })
    );
    setNames(sorted);
    void persist([]);
  }

  if (savedOrder === null) return null;

  return (
    <div className="bg-carbon-surface rounded-card p-4 flex flex-col gap-3">
      <button
        type="button"
        onClick={toggleCollapsed}
        aria-expanded={!collapsed}
        className="flex w-full items-start gap-2 text-start"
      >
        <svg
          width="14"
          height="14"
          viewBox="0 0 12 12"
          fill="none"
          aria-hidden="true"
          // Disclosure chevron — same RTL rotation scheme as the identical
          // icon above (form-engine Phase 2 Task 6): closed points start
          // (right in LTR, `rtl:rotate-180` flips it left), open always
          // rotates to straight-down in both directions.
          className={`mt-0.5 shrink-0 text-carbon-textSub transition-transform ${collapsed ? "rtl:rotate-180" : "rotate-90"}`}
        >
          <path d="M4 2l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        <span className="min-w-0 flex-1">
          <span className="font-semibold text-carbon-text text-sm">
            {t("vmBackupOrder.title")}
            {names.length > 0 && (
              <span className="ms-1.5 text-xs font-normal text-carbon-textMuted tabular-nums">
                ({names.length})
              </span>
            )}
          </span>
          {!collapsed && (
            <span className="mt-0.5 block text-xs text-carbon-textMuted">{t("vmBackupOrder.hint")}</span>
          )}
        </span>
      </button>
      {!collapsed &&
        (names.length === 0 ? (
          <p className="text-xs text-carbon-textMuted">{t("vmBackupOrder.empty")}</p>
        ) : (
          <>
            <ol className="flex flex-col gap-1">
              {names.map((name, i) => (
                <li
                  key={name}
                  {...rowProps(i)}
                  className={`flex items-center gap-2 rounded-control bg-carbon-surface2 px-3 py-1.5 ${
                    dragIndex === i ? "opacity-40" : ""
                  }`}
                >
                  <span className="shrink-0 cursor-grab text-carbon-textSub active:cursor-grabbing" aria-hidden="true">
                    <svg width="10" height="14" viewBox="0 0 10 14" fill="currentColor">
                      <circle cx="3" cy="3" r="1" />
                      <circle cx="7" cy="3" r="1" />
                      <circle cx="3" cy="7" r="1" />
                      <circle cx="7" cy="7" r="1" />
                      <circle cx="3" cy="11" r="1" />
                      <circle cx="7" cy="11" r="1" />
                    </svg>
                  </span>
                  <span className="w-6 text-xs text-carbon-textMuted tabular-nums">{i + 1}.</span>
                  <span className="flex-1 min-w-0 truncate text-sm text-carbon-text">
                    {displayByLibvirtName.get(name) ?? name}
                  </span>
                  <button
                    onClick={() => move(i, -1)}
                    disabled={i === 0 || saveState === "saving"}
                    aria-label={t("backupOrder.moveUp")}
                    title={t("backupOrder.moveUp")}
                    className="shrink-0 inline-flex items-center rounded-control p-1 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-30"
                  >
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                      <path d="M2 8l4-4 4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  </button>
                  <button
                    onClick={() => move(i, 1)}
                    disabled={i === names.length - 1 || saveState === "saving"}
                    aria-label={t("backupOrder.moveDown")}
                    title={t("backupOrder.moveDown")}
                    className="shrink-0 inline-flex items-center rounded-control p-1 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-30"
                  >
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                      <path d="M2 4l4 4 4-4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  </button>
                </li>
              ))}
            </ol>
            <div className="flex items-center gap-3 flex-wrap">
              <button
                onClick={() => void persist(names)}
                disabled={saveState === "saving"}
                className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
              >
                {t("backupOrder.save")}
              </button>
              <button
                onClick={clearOrder}
                disabled={saveState === "saving"}
                className="inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50"
              >
                {t("backupOrder.reset")}
              </button>
              {saveState === "saved" && <span className="text-xs text-statusOk">{t("backupOrder.saved")}</span>}
              {saveState === "error" && error && <span className="text-xs text-statusFail">{error}</span>}
            </div>
          </>
        ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// VMs page
// ---------------------------------------------------------------------------

export function VMs() {
  const { t } = useT();
  // One subscription for the whole list rather than one per row — see
  // Containers.tsx's identical call for the same reasoning.
  useRainbow();
  const { confirm, confirmDialog } = useConfirm();
  // Broader "something is running" signal: any backup/restore/replication in
  // flight disables the bulk start buttons + shows a hint.
  const running = anyActive(useProgress());
  const [vms, setVMs] = useState<VM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [sortKey, setSortKey] = useState<SortKey>(loadSortKey);
  const [search, setSearch] = useState("");
  const [scheduleFilter, setScheduleFilter] = useState<ScheduleFilterKey>(loadScheduleFilterKey);
  const [backupFilter, setBackupFilter] = useState<BackupFilterKey>(loadBackupFilterKey);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [bulkBusy, setBulkBusy] = useState(false);
  const [bulkMsg, setBulkMsg] = useState<string | null>(null);
  const [discovering, setDiscovering] = useState(false);
  const [discoverMsg, setDiscoverMsg] = useState<string | null>(null);

  async function handleDiscover() {
    setDiscovering(true);
    setDiscoverMsg(null);
    try {
      const res = await discoverVMs();
      if (res.ok) {
        setDiscoverMsg(`+${res.discovered ?? 0}`);
        await loadVMs();
      } else {
        setDiscoverMsg(res.error ?? "Discover failed");
      }
    } catch (err) {
      setDiscoverMsg(err instanceof Error ? err.message : "Discover failed");
    } finally {
      setDiscovering(false);
    }
  }

  function loadVMs() {
    return listVMs()
      .then((res) => {
        if (res.ok) setVMs(res.vms ?? []);
        else setError("Failed to load VMs");
      })
      .catch(() => setError("Failed to load VMs"));
  }

  useEffect(() => {
    void loadVMs().finally(() => setLoading(false));
  }, []);

  function handleSortChange(k: SortKey) {
    setSortKey(k);
    localStorage.setItem(SORT_STORAGE_KEY, k);
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
  // BEFORE sort + the live/orphans split, so they combine. VMs have no image, so
  // the search matches the name only.
  const query = search.trim().toLowerCase();
  const filtered = vms.filter((v) => {
    if (query && !v.name.toLowerCase().includes(query)) return false;
    if (scheduleFilter === "scheduled" && !v.includeInSchedule) return false;
    if (scheduleFilter === "notScheduled" && v.includeInSchedule) return false;
    if (backupFilter === "backedUp" && v.lastBackup == null) return false;
    if (backupFilter === "neverBackedUp" && v.lastBackup != null) return false;
    return true;
  });

  // Any contained filter off its default narrows the list. The schedule/backup
  // chips persist to localStorage, so a restored non-"all" value would silently
  // shrink the list behind the collapsed "Filters" button — surface it via the
  // trigger's dot. Sort is not a filter (it never hides rows), so it is excluded.
  const filtersActive =
    query !== "" || scheduleFilter !== "all" || backupFilter !== "all";

  const sorted = sortVMs(filtered, sortKey);
  const live = sorted.filter((v) => v.state !== "not-installed");
  const orphans = sorted.filter((v) => v.state === "not-installed");

  // When the list has VMs but the filters excluded them all, show a no-match hint
  // (distinct from the "no VMs at all" empty state, which keys off vms.length).
  const noMatch = vms.length > 0 && live.length === 0 && orphans.length === 0;

  function toggleSelect(name: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  }

  // Selection is keyed by libvirtName (the raw identifier every bulk action
  // below sends to the backend), never the display name.
  const allLiveSelected = live.length > 0 && live.every((v) => selected.has(v.libvirtName));
  function toggleSelectAll() {
    setSelected(allLiveSelected ? new Set() : new Set(live.map((v) => v.libvirtName)));
  }

  // Keep the selection in sync with what's actually visible: when a search or
  // filter hides a previously-selected VM, drop it, so the bulk-bar count stays
  // honest and a bulk action — including the DESTRUCTIVE "Restore selected" —
  // can never overwrite a VM the user can no longer see.
  useEffect(() => {
    setSelected((prev) => {
      if (prev.size === 0) return prev;
      const visible = new Set(live.map((v) => v.libvirtName));
      let changed = false;
      const next = new Set<string>();
      for (const n of prev) {
        if (visible.has(n)) next.add(n);
        else changed = true;
      }
      return changed ? next : prev;
    });
  }, [search, scheduleFilter, backupFilter, vms]); // eslint-disable-line react-hooks/exhaustive-deps

  async function runBulk(action: (name: string) => Promise<{ ok: boolean }>) {
    setBulkBusy(true);
    setBulkMsg(null);
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
    setBulkMsg(`${ok} ok, ${fail} failed`);
    setSelected(new Set());
    void loadVMs();
  }

  // Single VM backups AND restores are ASYNC and share the server's
  // single-flight guard, so firing them in a tight loop would make every call
  // after the first hit "already running". Run the bulk serially via
  // fireAndWaitRun: it fires one run (retrying briefly while the previous VM's
  // guard is still releasing), then waits for the NEW recorded run to finish
  // before the next — correlated by run id, never by the client clock.
  function backupSelected() {
    void runBulk((name) =>
      fireAndWaitRun({
        kind: "backup",
        matchRun: (r) => r.domain === "vm" && r.target === name,
        start: () => backupVMNow(name),
      })
    );
  }

  async function restoreSelected() {
    if (!(await confirm(t("vms.restoreSelectedConfirm")))) return;
    void runBulk((name) =>
      fireAndWaitRun({
        kind: "restore",
        matchRun: (r) => r.domain === "vm" && r.target === name,
        start: () => restoreVM(name, "latest", true),
      })
    );
  }

  return (
    <div className="flex flex-col gap-6 max-w-5xl">
      {/* Page heading + Discover (disaster-recovery) action */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">
            {t("vms.title")}
          </h1>
          <p className="mt-1 text-sm text-carbon-textSub">
            {t("vms.subtitle")}
          </p>
          <div className="mt-2"><OffsiteIndicator domain="vms" /></div>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {discoverMsg && (
            <span className="text-xs text-carbon-textSub">{discoverMsg}</span>
          )}
          <button
            onClick={() => void handleDiscover()}
            disabled={discovering}
            title={t("vms.discoverHint")}
            className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {discovering ? t("containers.discovering") : t("containers.discover")}
          </button>
        </div>
      </div>

      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {error && (
        <p className="text-sm text-statusFail">{error}</p>
      )}
      {!loading && !error && vms.length === 0 && (
        <div className="bg-carbon-surface rounded-card p-6 text-center flex flex-col items-center gap-3">
          {/* No "Add" action here (unlike Receiver/Fleet/Files): this list is a
              live enumeration of what libvirt/KVM actually reports, not a
              BombVault-managed list to add to. The page's own Discover button
              above (disaster-recovery re-scan) is already the relevant action
              for an empty result, so a second button here would be redundant. */}
          <EmptyStateIcon icon={IconVM} />
          <p className="text-sm text-carbon-textMuted">{t("vms.empty")}</p>
        </div>
      )}

      {/* VM backup-order panel (#119, VMs) — advanced: arrange the scheduled VM
          run sequence. Above the list, like the container backup-order card. */}
      {!loading && !error && (
        <Advanced>
          <VMBackupOrderPanel vms={vms} t={t} />
        </Advanced>
      )}

      {/* Controls: Filters popover (search + schedule/backup filters + sort) + select-all. */}
      {!loading && vms.length > 0 && (
        <div className="flex items-center gap-x-6 gap-y-2 flex-wrap">
          <FilterPopover label={t("filter.button")} active={filtersActive}>
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t("vms.searchPlaceholder")}
              spellCheck={false}
              autoComplete="off"
              className="w-full rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 bv-field-focus"
            />
            <ChipFilter<ScheduleFilterKey>
              label={t("filter.schedule")}
              value={scheduleFilter}
              onChange={handleScheduleFilterChange}
              options={[
                { key: "all", label: t("filter.all") },
                { key: "scheduled", label: t("filter.scheduled") },
                { key: "notScheduled", label: t("filter.notScheduled") },
              ]}
            />
            <ChipFilter<BackupFilterKey>
              label={t("filter.backup")}
              value={backupFilter}
              onChange={handleBackupFilterChange}
              options={[
                { key: "all", label: t("filter.all") },
                { key: "backedUp", label: t("filter.backedUp") },
                { key: "neverBackedUp", label: t("filter.neverBackedUp") },
              ]}
            />
            <SortControl value={sortKey} onChange={handleSortChange} t={t} />
          </FilterPopover>
          {live.length > 0 && (
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
              <ScheduleIncludeAllControl t={t} onChanged={() => void loadVMs()} />
            </div>
          )}
        </div>
      )}

      {/* Bulk action bar */}
      {!loading && selected.size > 0 && (
        <div className="flex items-center gap-3 flex-wrap rounded-card bg-carbon-surface2 px-3 py-2">
          <span className="text-xs text-carbon-textSub">
            {selected.size} {t("containers.selectedCount")}
          </span>
          <button
            onClick={backupSelected}
            disabled={bulkBusy || running.active}
            className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {t("vms.backupSelected")}
          </button>
          {/* Bulk restore is advanced-only; bulk backup stays basic. */}
          <Advanced>
            <button
              onClick={() => void restoreSelected()}
              disabled={bulkBusy || running.active}
              className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {t("vms.restoreSelected")}
            </button>
          </Advanced>
          <button
            onClick={() => setSelected(new Set())}
            disabled={bulkBusy}
            className="text-xs text-carbon-textMuted hover:text-carbon-text transition-colors disabled:opacity-50"
          >
            {t("containers.clearSelection")}
          </button>
          {bulkBusy && (
            <span className="text-xs text-carbon-textMuted">{t("containers.working")}</span>
          )}
          {!bulkBusy && running.active && (
            <span className="text-xs text-carbon-textMuted">
              {t(busyPhraseKey(running.phase))}
            </span>
          )}
          {!bulkBusy && bulkMsg && (
            <span className="text-xs text-carbon-textSub">{bulkMsg}</span>
          )}
        </div>
      )}

      {/* Live VMs */}
      {!loading && live.length > 0 && (
        <div className="flex flex-col gap-3">
          {live.map((v, i) => (
            <VMRow
              key={v.libvirtName}
              vm={v}
              t={t}
              onRefresh={() => void loadVMs()}
              selected={selected.has(v.libvirtName)}
              onToggleSelect={() => toggleSelect(v.libvirtName)}
              index={i}
            />
          ))}
        </div>
      )}

      {/* Orphan VMs — no longer defined on the host but still have backups */}
      {!loading && orphans.length > 0 && (
        <div className="flex flex-col gap-3">
          <div>
            <h2 className="flex items-center">
              <Badge tone="heading" size="heading" wrap>{t("containers.notInstalledTitle")}</Badge>
            </h2>
            <p className="mt-1 text-xs text-carbon-textMuted">
              {t("vms.notInstalledHint")}
            </p>
          </div>
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
          {orphans.map((v, i) => (
            <VMRow key={v.libvirtName} vm={v} t={t} onRefresh={() => void loadVMs()} index={live.length + i} />
          ))}
        </div>
      )}

      {/* No VM matches the active search / schedule / backup filters. */}
      {!loading && !error && noMatch && (
        <p className="text-sm text-carbon-textMuted">{t("filter.noMatch")}</p>
      )}
      {confirmDialog}
    </div>
  );
}
