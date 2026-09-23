import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { Link } from "react-router-dom";
import { listVMs, backupVMNow, restoreVM, listVMSnapshots, setVMInclude, setVMIncludeAll, setVMMethod, deleteSnapshot, deleteBackupsVM, forgetVM, discoverVMs, exportVM, getVmBackupOrder, setVmBackupOrder, setVMRepo, getSettings } from "../lib/api";
import type { VM, Snapshot, VmOrder, Run } from "../lib/api";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { FilterPopover } from "../components/FilterPopover";
import { ChipFilter, loadStoredFilterKey } from "../components/ChipFilter";
import { IconTipButton } from "../components/IconTipButton";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { BULK_HUE } from "../lib/bulkHue";
import { useT, stateLabel } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { useDragReorder } from "../lib/useDragReorder";
import { Advanced, useAdvanced } from "../lib/advanced";
import { BackupCancelButton } from "../components/BackupCancelButton";
import { ProgressBar } from "../components/ProgressBar";
import { RestoreAction } from "../components/restore/RestoreAction";
import { RecentRunsList } from "../components/RecentRunsList";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconVM, IconRestore, IconTrash, IconBackupNow, IconDownload, IconPower, IconLive } from "../components/Sidebar";
import { InfoBubble } from "../components/InfoBubble";
import { NotInstalledHeading } from "../components/NotInstalledHeading";
import { OrphanRemoveButton } from "../components/OrphanRemoveButton";
import { RenameTakeoverRow } from "../components/RenameTakeoverRow";
import { LinkEntryPicker } from "../components/LinkEntryPicker";
import { FormerNames } from "../components/FormerNames";
import { vmTakeover } from "../lib/useTakeOver";
import { Badge, type BadgeTone } from "../components/Badge";
import { Button } from "../components/Button";
import { groupStage } from "../lib/controls";
// The Containers page's schedule switch, saving through setVMInclude here.
import { IncludeToggle } from "../components/IncludeToggle";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch, fireAndWaitRun } from "../lib/backupWatch";
import { useConfirm } from "../lib/useConfirm";
import { hueVars } from "../lib/appearance";
import { Selector } from "../components/Selector";
import { useToast } from "../lib/toast";
import { RepoPicker } from "../components/RepoPicker";
// The phone face's building blocks: the breakpoint hook, the ONE pagination
// primitive, the shared mobile list chrome and the card block's surfaces.
import { useIsDesktop } from "../lib/useMediaQuery";
import { useLoadMore } from "../lib/useLoadMore";
import { ListToolbar } from "../components/mobile/ListToolbar";
import { MobileListCard } from "../components/mobile/MobileListCard";
import { MobileDetailShell } from "../components/mobile/MobileDetailShell";
import { RunDetailSheet } from "../components/mobile/RunDetailSheet";
import { MobileSectionLabel } from "../components/mobile/MobileSectionLabel";

type T = ReturnType<typeof useT>["t"];

function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

// stateTone maps a raw VM state to a Badge tone, as in Containers.tsx;
// stateLabel translates the state itself.
function stateTone(state: string): BadgeTone {
  const lower = state.toLowerCase();
  if (lower === "running") return "ok";
  if (lower === "shut off" || lower === "shutoff" || lower === "stopped") return "fail";
  return "neutral";
}

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

// SortControl and ChipFilter adapt the shared Selector to this page, like the
// matching pair in Containers.tsx; the filter menus of both pages have to stay
// the same control.
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
        variant="well"
        select="one"
        active={value}
        onChange={(id) => onChange(id as SortKey)}
      />
    </div>
  );
}

// The schedule and backup filters. VMs have no installed filter: the split
// into live VMs and orphans already covers it.
type ScheduleFilterKey = "all" | "scheduled" | "notScheduled";
type BackupFilterKey = "all" | "backedUp" | "neverBackedUp";

const SCHEDULE_FILTER_STORAGE_KEY = "bv-vms-schedule-filter";
const BACKUP_FILTER_STORAGE_KEY = "bv-vms-backup-filter";

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
  const { push } = useToast();

  // Rows are keyed by libvirt name and do not remount, so re-seed when a list
  // reload hands down a new value.
  useEffect(() => setMethod(initial || "graceful"), [initial]);

  async function handleChange(next: string) {
    // Reverted on failure, so a rejected switch to "live" does not leave the
    // UI promising no downtime while the next backup shuts the VM down.
    const prev = method;
    setMethod(next);
    setBusy(true);
    try {
      const res = await setVMMethod(name, next);
      if (!res.ok) {
        setMethod(prev);
        push(res.error ?? t("vm.method.saveFailed"), "fail");
      }
    } catch (err) {
      setMethod(prev);
      push(err instanceof Error ? err.message : t("vm.method.saveFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }

  // Both options stay visible with the active one filled, rather than one
  // badge that cycles: this decides whether the VM is shut down for its
  // backup, so the alternative should be in view. The tip names the method.
  return (
    <Selector
      items={[
        {
          id: "graceful",
          label: t("vm.method.graceful"),
          icon: <IconPower />,
          tip: t("vm.method.graceful"),
        },
        {
          id: "live",
          label: t("vm.method.live"),
          icon: <IconLive />,
          tip: t("vm.method.live"),
        },
      ]}
      label={t("vm.method")}
      size="sm"
      select="one"
      equalWidth
      disabled={busy}
      active={method}
      onChange={(id) => void handleChange(id)}
    />
  );
}

// VMExportButton keeps its result inline as well as toasting a failure, like
// Containers.tsx's ExportButton: the destination path is something to copy
// down, not a passing notice.
function VMExportButton({ name, t }: { name: string; t: T }) {
  const [state, setState] = useState<"idle" | "pending" | "done" | "error">("idle");
  const [msg, setMsg] = useState<string | null>(null);
  const { push } = useToast();
  const [shake, setShake] = useState(0);
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
        const message = r.error ?? t("settings.error");
        setMsg(message);
        push(message, "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      setState("error");
      const message = err instanceof Error ? err.message : t("settings.error");
      setMsg(message);
      push(message, "fail");
      setShake((n) => n + 1);
    }
  }
  return (
    <div className="flex flex-col items-end gap-1">
      {/* No hueIndex: the VMRow card already carries this VM's hue. The
          column aligns to the end so the text hangs beneath the button at
          the card edge. */}
      <Button
        key={shake}
        label={t("export.button")}
        labelKey="export.button"
        glyph={<IconDownload />}
        tone="accent"
        // Shares its width stage with the backup button beside it.
        stage={groupStage([t("containers.backupNow"), t("export.button")])}
        onClick={() => void run()}
        disabled={state === "pending"}
        busy={state === "pending"}
        className={shake ? "glim-shake" : ""}
      />
      {state === "done" && (
        <span className="text-xs text-statusOk break-all text-end max-w-[18rem]">{t("export.exportedTo")} {msg}</span>
      )}
      {state === "error" && <span className="text-xs text-statusFail break-all text-end max-w-[18rem]">{msg}</span>}
    </div>
  );
}

// VMBackupButton is the VM counterpart of components/BackupButton.tsx. Results
// arrive as toasts (a failure also shakes the button), and the reason it is
// blocked by another run goes in the title. The toasts are raised here rather
// than in useBackupWatch, whose state also carries RestoreAction's sticky
// restore outcome.
function VMBackupButton({
  name,
  t,
  onBackedUp,
  running,
  onRunCorrelated,
}: {
  name: string;
  t: T;
  onBackedUp?: () => void;
  /** Whether any operation is running (anyActive). It blocks this backup
   *  while another one runs, but never because of its own (isPending). */
  running?: { active: boolean; phase?: string };
  /** Optional correlated-run deep-link. useBackupWatch's `onRun` fires on
   *  EVERY poll with the baseline-id-correlated run; the mobile card host
   *  feeds it the block's component-local RunDetailSheet (with the dismissed
   *  latch — the same mechanism Containers.tsx's mobile detail hosts).
   *  Desktop callers omit it and stay byte-identical, exactly as
   *  BackupButton.tsx's own passthrough pins. */
  onRunCorrelated?: (run: Run) => void;
}) {
  // The server backs the VM up detached and answers at once, so the outcome
  // comes from watching the "vm:<name>" progress and the recorded run.
  const { state, fire, isPending } = useBackupWatch({
    progressKey: `vm:${name}`,
    start: () => backupVMNow(name),
    matchRun: (r) => r.domain === "vm" && r.target === name,
    onDone: onBackedUp,
    onRun: onRunCorrelated,
  });
  const blockedByOther = !!running?.active && !isPending;
  const { push } = useToast();
  const [shake, setShake] = useState(0);
  // The last phase already reported, so each terminal transition toasts once.
  // state.phase starts at "idle", so nothing fires on mount.
  const seenPhase = useRef(state.phase);

  useEffect(() => {
    if (state.phase === seenPhase.current) return;
    seenPhase.current = state.phase;
    if (state.phase === "success") {
      push(
        state.snapshotId ? `${t("common.done")} · ${state.snapshotId.slice(0, 8)}` : t("common.done"),
        "success"
      );
    } else if (state.phase === "error") {
      push(state.message, "fail");
      setShake((n) => n + 1);
    }
  }, [state, push, t]);

  // The label stays stable; exceptional states go in the title.
  const stateTip = isPending
    ? t("common.backingUp")
    : blockedByOther
      ? t(busyPhraseKey(running?.phase))
      : undefined;

  return (
    <Button
      key={shake}
      label={t("containers.backupNow")}
      labelKey="containers.backupNow"
      glyph={<IconBackupNow />}
      tone="accent"
      onClick={() => void fire()}
      disabled={isPending || blockedByOther}
      busy={isPending}
      title={stateTip}
      className={shake ? "glim-shake" : ""}
    />
  );
}

function VMSnapshotRow({
  snap,
  vmName,
  vmDisplayName,
  source,
  onDeleted,
  t,
}: {
  snap: Snapshot;
  /** Raw libvirt name, used for the progress key and the restore action. */
  vmName: string;
  /** Display name for the cancel-confirm text; falls back to vmName. */
  vmDisplayName?: string;
  source: RepoSource;
  onDeleted: () => void;
  t: T;
}) {
  const progressMap = useProgress();
  // RestoreAction blocks a new restore while any other operation runs; this
  // VM's own restore is handled there through isPending.
  const running = anyActive(progressMap);
  // Delete is blocked only by this VM's own backup or restore: deleting one
  // VM's snapshot has to stay possible while another VM is backing up.
  const busy = progressMap[`vm:${vmName}`]?.active ?? false;
  const [deleting, setDeleting] = useState(false);
  const { push } = useToast();
  const [shake, setShake] = useState(0);
  // Collapsed by default so the list stays compact.
  const [showRestore, setShowRestore] = useState(false);
  const { confirm, confirmDialog } = useConfirm();

  async function handleDelete() {
    if (!(await confirm(t("snapshots.deleteConfirm"), { confirmKey: "snapshots.delete" }))) return;
    setDeleting(true);
    try {
      const res = await deleteSnapshot("vms", snap.id, source);
      if (res.ok) onDeleted();
      else {
        push(res.error ?? t("common.deleteFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.deleteFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setDeleting(false);
    }
  }

  return (
    // py-1.5 keeps the row at 44px with 32px buttons, like the snapshot rows
    // of the other pages.
    <div className="flex flex-col gap-1 py-1.5 border-b border-carbon-border last:border-0">
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
        {/* No hueIndex: the VMRow card carries this VM's hue. Delete gets no
            colour of its own; the glyph and the confirm dialog carry its
            meaning. */}
        <Button
          label={t("restore.open")}
          labelKey="restore.open"
          glyph={<IconRestore />}
          tone="accent"
          onClick={() => setShowRestore((p) => !p)}
          className={"shrink-0"}
        />
        <Button
          key={shake}
          label={t("snapshots.delete")}
          labelKey="snapshots.delete"
          glyph={<IconTrash />}
          tone="accent"
          onClick={() => void handleDelete()}
          disabled={deleting || busy}
          className={`shrink-0${shake ? " glim-shake" : ""}`}
        />
      </div>
      {/* Indented past the id column; ps-24 is logical, so it follows the
          reading direction. */}
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
      {confirmDialog}
    </div>
  );
}

function VMRestorePanel({
  name,
  displayName,
  t,
  open,
}: {
  /** Raw libvirt name. Every call in this panel uses it, never displayName. */
  name: string;
  /** Display name shown in the restore cancel-confirm text; falls back to
   *  name. */
  displayName?: string;
  t: T;
  /** Owned by VMRow's `openSections`, as components/RestorePanel.tsx takes
   *  `open` from ContainerRow, so both cards share one disclosure. */
  open: boolean;
}) {
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(false);
  // A failed list load replaces the list, so it stays inline rather than in a
  // toast.
  const [error, setError] = useState<string | null>(null);

  const [reloadTick, setReloadTick] = useState(0);
  const [deletingAll, setDeletingAll] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [shakeDeleteAll, setShakeDeleteAll] = useState(0);

  useEffect(() => {
    if (!open) return;
    // An answer that arrives after the next switch is dropped, and a failure
    // empties the list, whose rows belong to the source just left.
    let current = true;
    const fail = (message: string) => {
      if (!current) return;
      setSnapshots([]);
      setError(message);
    };
    setLoading(true);
    setError(null);
    listVMSnapshots(name, source)
      .then((res) => {
        if (!res.ok) return fail(res.error ?? t("common.loadBackupsFailed"));
        if (current) setSnapshots(res.snapshots ?? []);
      })
      .catch(() => fail(t("common.loadBackupsFailed")))
      .finally(() => {
        if (current) setLoading(false);
      });
    return () => {
      current = false;
    };
  }, [open, name, source, reloadTick]); // eslint-disable-line react-hooks/exhaustive-deps -- t() is only read to build a failure message; re-fetching on a language switch would be a wasted round-trip

  // A failure is toasted rather than set as `error`: the reload in .finally()
  // clears `error` again straight away.
  async function handleDeleteAll() {
    // TODO: name the stake in the confirmation ("N snapshots, X GB").
    if (!(await confirm(t("snapshots.deleteAllConfirm"), { confirmKey: "snapshots.deleteAll" }))) return;
    setDeletingAll(true);
    deleteBackupsVM(name, source)
      .then((res) => {
        if (!res.ok) {
          push(res.error ?? t("common.deleteBackupsFailed"), "fail");
          setShakeDeleteAll((n) => n + 1);
        }
      })
      .catch(() => {
        push(t("common.deleteBackupsFailed"), "fail");
        setShakeDeleteAll((n) => n + 1);
      })
      .finally(() => {
        setDeletingAll(false);
        setReloadTick((n) => n + 1);
      });
  }

  // The trigger lives in VMRow, so a closed panel renders nothing. The hooks
  // above still run, so the list refetches on the render that opens it.
  if (!open) return null;

  return (
    <>
        <div className="rounded-card bg-carbon-background px-3 py-1">
          {/* The source hint sits on the label, so it shows only when the
              toggle does. */}
          <div className="flex items-center gap-2 py-2 border-b border-carbon-border">
              {/* Source (Local / Off-site) toggle is advanced; basic mode uses local. */}
              <Advanced>
                <span className="flex items-center gap-1 text-xs text-carbon-textMuted">
                  {t("source.label")}
                  <InfoBubble tip={t("source.hint")} />
                </span>
                <SourceToggle source={source} onChange={setSource} disabled={loading} domain="vms" />
              </Advanced>
              {snapshots.length > 0 && (
                // Neutral, with no red of its own, like the same control in
                // Files.tsx; the label and the confirm dialog carry the
                // meaning.
                <Button
                  key={shakeDeleteAll}
                  label={t("snapshots.deleteAll")}
                  labelKey="snapshots.deleteAll"
                  tone="neutral"
                  onClick={() => void handleDeleteAll()}
                  disabled={deletingAll || loading}
                  busy={deletingAll}
                  title={deletingAll ? t("snapshots.deletingAll") : undefined}
                  className={`ms-auto${shakeDeleteAll ? " glim-shake" : ""}`}
                />
              )}
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
      {confirmDialog}
    </>
  );
}

// VMRow is exported for VMs.test.tsx, which checks that its actions pass
// VM.libvirtName to the backend rather than the display name VM.name.
export function VMRow({
  vm,
  t,
  onRefresh,
  selected,
  onToggleSelect,
  linkCandidates = [],
  index,
}: {
  vm: VM;
  t: T;
  onRefresh: () => void;
  selected?: boolean;
  onToggleSelect?: () => void;
  /** The libvirt names of the not-installed entries this card can take over by hand. */
  linkCandidates?: string[];
  /** Position in the rendered list, which sets the palette position. */
  index: number;
}) {
  const installed = vm.state !== "not-installed";
  const progressMap = useProgress();
  // The server keys progress by the raw libvirt name ("vm:"+name in
  // internal/api/service.go), not by the display name.
  const progress = progressMap[`vm:${vm.libvirtName}`];
  // Whether anything is running in any domain; the button handles its own
  // backup through isPending.
  const running = anyActive(progressMap);

  // The same shape as ContainerRow's `openSections`, although VMs have only one
  // section, so the two cards keep one disclosure control.
  const [openSections, setOpenSections] = useState<Set<string>>(() => new Set());
  const { push } = useToast();
  // Reverted on a failed save, so the picker never shows a destination the
  // server did not accept.
  const [repoChoice, setRepoChoice] = useState(vm.repo ?? "");
  useEffect(() => {
    setRepoChoice(vm.repo ?? "");
  }, [vm.repo]);
  function toggleSection(id: string) {
    setOpenSections((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  const lastBackupText = `${t("containers.lastBackup")}: ${vm.lastBackup ? formatTs(vm.lastBackup) : t("containers.never")}`;

  const aliases = vm.aliases ?? [];
  const takeoverEntry = { name: vm.libvirtName, displayName: vm.name, api: vmTakeover };

  return (
    <div
      style={{ ...hueVars(index), "--row-i": String(index) } as CSSProperties}
      // glim-active while this VM's own backup or restore runs, so reactive
      // mode shows the hue without hover, as on ContainerRow.
      className={`relative overflow-hidden bg-carbon-surface rounded-card p-4 flex flex-col gap-3 glim-hue glim-stagger-row ${
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
            aria-label={t("common.selectItem").replace("{name}", vm.name)}
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

        {/* The method shares the line with the action buttons. gap-4 between
            the groups and gap-1.5 within the pair keep "Live" from sitting as
            close to the backup button as to its own sibling, where a mis-click
            would start a backup.
              A VM that is no longer installed has nothing to back up, so the
            corner holds its removal button instead, as on the container card.
            It deletes the local backups; the Backups panel keeps its own
            source-aware delete for the off-site copy. */}
        {installed ? (
          <div className="ms-auto flex items-center gap-4 shrink-0">
            {/* Never behind Advanced: it decides whether the VM is shut down. */}
            <div className="flex items-center gap-2">
              <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                {t("vm.method")}
                <InfoBubble tip={t("vm.method.hint")} />
              </span>
              <VMMethodSelect name={vm.libvirtName} initial={vm.method} t={t} />
            </div>
            <div className="flex items-center gap-1.5">
              <VMBackupButton name={vm.libvirtName} t={t} onBackedUp={onRefresh} running={running} />
              {/* Plain export is an advanced-only extra. */}
              <Advanced><VMExportButton name={vm.libvirtName} t={t} /></Advanced>
            </div>
          </div>
        ) : (
          <div className="ms-auto shrink-0">
            <OrphanRemoveButton
              hasBackups={vm.lastBackup != null}
              deleteConfirm={t("vms.deleteBackupsConfirm")}
              removeConfirm={t("vms.removeEntryConfirm")}
              deleteBackups={() => deleteBackupsVM(vm.libvirtName, "local")}
              removeEntry={() => forgetVM(vm.libvirtName)}
              onDone={onRefresh}
              t={t}
            />
          </div>
        )}
      </div>

      {installed && vm.renameFrom && (
        <RenameTakeoverRow
          key={vm.renameFrom}
          from={vm.renameFrom}
          reason={vm.renameReason ?? ""}
          entry={takeoverEntry}
          onDone={onRefresh}
          t={t}
        />
      )}
      {aliases.length > 0 && (
        <FormerNames
          aliases={aliases}
          conflicts={vm.aliasConflicts ?? []}
          entry={takeoverEntry}
          onDone={onRefresh}
          t={t}
        />
      )}

      {/* The include toggle shows on a removed VM too: it stays scheduled, and
          every run tries it again and logs a skip until this switch goes off. */}
      <div className="flex items-start gap-3">
        {/* The start of this row is free, so the link picker takes it, as on
            the container card. */}
        {installed && vm.lastBackup == null && aliases.length === 0 && linkCandidates.length > 0 && (
          <LinkEntryPicker candidates={linkCandidates} entry={takeoverEntry} onDone={onRefresh} t={t} />
        )}
        {/* IncludeToggle renders its own label. */}
        <div className="ms-auto">
          <IncludeToggle
            name={vm.libvirtName}
            initial={vm.includeInSchedule}
            save={setVMInclude}
          />
        </div>
      </div>

      {/* ContainerRow's disclosure block with a single section. */}
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2 flex-wrap">
          {/* buttonHeight, as on the container and folder cards. */}
          <Selector
            items={[{ id: "backups", label: t("snapshots.title") }]}
            label={t("containers.sectionsLabel")}
            select="many"
            active={openSections}
            buttonHeight
            onChange={toggleSection}
          />
          <span className="ms-auto shrink-0 text-xs text-carbon-textMuted whitespace-nowrap">
            {lastBackupText}
          </span>
        </div>

        {/* Where this VM's backups go. Locked once the VM has backups: they
            stay in the repository they were written to. */}
        <Advanced>
          <RepoPicker
            value={repoChoice}
            onChange={(next) => {
              const before = repoChoice;
              setRepoChoice(next);
              void setVMRepo(vm.libvirtName, next).then((r) => {
                if (r.ok) {
                  push(t("folders.saved"), "success");
                  return;
                }
                push(r.error ?? t("settings.error"), "fail");
                setRepoChoice(before);
              });
            }}
            locked={vm.lastBackup != null}
          />
        </Advanced>

        <VMRestorePanel
          name={vm.libvirtName}
          displayName={vm.name}
          t={t}
          open={openSections.has("backups")}
        />
      </div>

      {/* Not during a restore, which has its own cancel in the Backups panel,
          and only while the run is active, so a finished run leaves no button
          behind. */}
      {progress && progress.active && progress.phase !== "restore" && (
        <div className="flex justify-end">
          <BackupCancelButton cancelKey={`vm:${vm.libvirtName}`} name={vm.name} t={t} />
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
  const { push } = useToast();
  // One shake counter per button, so only the one that was clicked shakes.
  const [shakeInclude, setShakeInclude] = useState(0);
  const [shakeExclude, setShakeExclude] = useState(0);

  async function run(include: boolean) {
    setBusy(true);
    try {
      const res = await setVMIncludeAll(include);
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
        className={`inline-flex items-center rounded-control bg-accent px-3 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
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
        className={`inline-flex items-center rounded-control px-3 py-1 text-xs font-medium text-carbon-textSub hover:text-carbon-text transition-colors disabled:opacity-50${
          shakeExclude ? " glim-shake" : ""
        }`}
      />
    </div>
  );
}

function VMSelectAll({ t, checked, onChange }: { t: T; checked: boolean; onChange: () => void }) {
  return (
    <label className="flex items-center gap-2 text-xs text-carbon-textSub cursor-pointer">
      <input
        type="checkbox"
        checked={checked}
        onChange={onChange}
        className="h-4 w-4 cursor-pointer"
        style={{ accentColor: "var(--accent)" }}
      />
      {t("containers.selectAll")}
    </label>
  );
}

// VMBulkBar acts on the ticked VMs and is the same bar at both widths.
function VMBulkBar({
  t,
  count,
  busy,
  running,
  onBackup,
  onRestore,
  onClear,
}: {
  t: T;
  count: number;
  busy: boolean;
  running: { active: boolean; phase?: string };
  onBackup: () => void;
  onRestore: () => void;
  onClear: () => void;
}) {
  return (
    <div className="flex items-center gap-3 flex-wrap rounded-card bg-carbon-surface2 px-3 py-2">
      <span className="text-xs text-carbon-textSub">
        {count} {t("containers.selectedCount")}
      </span>
      <Button
        label={t("vms.backupSelected")}
        labelKey="vms.backupSelected"
        hueIndex={BULK_HUE.backup}
        tone="accent"
        onClick={onBackup}
        disabled={busy || running.active}
      />
      {/* Bulk restore is advanced-only; bulk backup stays basic. */}
      <Advanced>
        <Button
          label={t("vms.restoreSelected")}
          labelKey="vms.restoreSelected"
          hueIndex={BULK_HUE.restore}
          tone="accent"
          onClick={onRestore}
          disabled={busy || running.active}
        />
      </Advanced>
      <Button
        label={t("containers.clearSelection")}
        labelKey="containers.clearSelection"
        tone="neutral"
        onClick={onClear}
        disabled={busy}
      />
      {busy && <span className="text-xs text-carbon-textMuted">{t("containers.working")}</span>}
      {!busy && running.active && (
        <span className="text-xs text-carbon-textMuted">{t(busyPhraseKey(running.phase))}</span>
      )}
    </div>
  );
}

const VM_BACKUP_ORDER_COLLAPSED_KEY = "bombvault.vmBackupOrderCollapsed";

// VMBackupOrderPanel sets the order in which the scheduled run backs up the
// included VMs, like the container BackupOrderPanel. It loads the saved order
// once, then reconciles as VMs come and go without discarding a reorder in
// progress. Save puts the whole displayed sequence; Reset puts an empty list,
// which falls back to name order.
//
// `names` holds raw libvirt names, which the backend stores and matches on.
// `displayByLibvirtName` maps them back to the readable name for display and
// for the name-order tiebreak.
function VMBackupOrderPanel({
  vms,
  t,
  hueIndex,
}: {
  vms: VM[];
  t: T;
  /** Palette position for the panel heading. The caller computes it with
   *  nextHue() at the call site; a child calling nextHue() itself would run
   *  after all its siblings and get the wrong slot. */
  hueIndex?: number;
}) {
  const [savedOrder, setSavedOrder] = useState<VmOrder[] | null>(null);
  const [names, setNames] = useState<string[]>([]);
  const [saveState, setSaveState] = useState<"idle" | "saving">("idle");
  const { push } = useToast();
  // persist() cannot tell Save from Reset, so each has its own counter.
  const [shakeSave, setShakeSave] = useState(0);
  const [shakeReset, setShakeReset] = useState(0);
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
        /* private mode or quota: the collapse just won't persist */
      }
      return next;
    });
  }

  async function persist(order: string[], via: "save" | "reset") {
    setSaveState("saving");
    const bumpShake = via === "save" ? setShakeSave : setShakeReset;
    try {
      const res = await setVmBackupOrder(order);
      if (res.ok) {
        setSavedOrder(order.map((vm, i) => ({ vm, order: i + 1 })));
        push(t("backupOrder.saved"), "success");
      } else {
        push(res.error ?? t("backupOrder.saveError"), "fail");
        bumpShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("backupOrder.saveError"), "fail");
      bumpShake((n) => n + 1);
    } finally {
      setSaveState("idle");
    }
  }

  function clearOrder() {
    const sorted = [...names].sort((a, b) =>
      (displayByLibvirtName.get(a) ?? a).localeCompare(displayByLibvirtName.get(b) ?? b, undefined, {
        sensitivity: "base",
      })
    );
    setNames(sorted);
    void persist([], "reset");
  }

  if (savedOrder === null) return null;

  return (
    // glim-notch-card makes the card the notch's positioned ancestor and lets
    // hovering anywhere on it reveal the hue in reactive mode. It does not set
    // --accent, so glim-hue is needed too, or the Save button keeps the flat
    // theme accent.
    <div
      className={`relative glim-notch-card bg-carbon-surface rounded-card p-4 flex flex-col gap-3${
        hueIndex !== undefined ? " glim-hue" : ""
      }`}
      style={hueIndex !== undefined ? (hueVars(hueIndex) as CSSProperties) : undefined}
    >
      {/* The Badge is the h2's only child: size="heading" positions it
          absolutely, so a sibling would land in its vacated slot. The count
          goes inside the badge instead. Visible even while collapsed. */}
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={hueIndex}>
          {t("vmBackupOrder.title")}
          {names.length > 0 && (
            <span className="ms-1.5 font-normal normal-case tracking-normal tabular-nums opacity-80">
              ({names.length})
            </span>
          )}
        </Badge>
      </h2>
      {/* The title lives in the h2, so aria-label names this button; while
          collapsed it shows only the hidden chevron. w-full keeps the whole
          row clickable. */}
      <button
        type="button"
        onClick={toggleCollapsed}
        aria-expanded={!collapsed}
        aria-label={t("vmBackupOrder.title")}
        className="flex w-full items-start gap-2 text-start"
      >
        <svg
          width="14"
          height="14"
          viewBox="0 0 12 12"
          fill="none"
          aria-hidden="true"
          // Closed points to the reading start (flipped in RTL); open points
          // down in both directions.
          className={`mt-0.5 shrink-0 text-carbon-textSub transition-transform ${collapsed ? "rtl:rotate-180" : "rotate-90"}`}
        >
          <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
        </svg>
        {!collapsed && (
          <span className="min-w-0 flex-1 text-xs text-carbon-textMuted">{t("vmBackupOrder.hint")}</span>
        )}
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
                  <IconTipButton
                    tip={t("backupOrder.moveUp")}
                    onClick={() => move(i, -1)}
                    disabled={i === 0 || saveState === "saving"}
                    className="shrink-0 inline-flex items-center rounded-control p-1 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-30"
                  >
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                      <path fill="currentColor" d="M1.3 8.7 6 3.3 10.7 8.7Z" />
                    </svg>
                  </IconTipButton>
                  <IconTipButton
                    tip={t("backupOrder.moveDown")}
                    onClick={() => move(i, 1)}
                    disabled={i === names.length - 1 || saveState === "saving"}
                    className="shrink-0 inline-flex items-center rounded-control p-1 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-30"
                  >
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                      <path fill="currentColor" d="M1.3 3.3 6 8.7 10.7 3.3Z" />
                    </svg>
                  </IconTipButton>
                </li>
              ))}
            </ol>
            <div className="flex items-center gap-3 flex-wrap">
              <Button
                key={shakeReset}
                label={t("backupOrder.reset")}
                labelKey="backupOrder.reset"
                tone="subtle"
                onClick={clearOrder}
                disabled={saveState === "saving"}
                className={`inline-flex items-center rounded-control px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:text-carbon-text transition-colors disabled:opacity-50${
                  shakeReset ? " glim-shake" : ""
                }`}
              />
              <Button
                key={shakeSave}
                label={t("backupOrder.save")}
                labelKey="backupOrder.save"
                tone="accent"
                onClick={() => void persist(names, "save")}
                disabled={saveState === "saving"}
                busy={saveState === "saving"}
                className={shakeSave ? "glim-shake" : ""}
              />
            </div>
          </>
        ))}
    </div>
  );
}

export function VMs() {
  const { t } = useT();
  // ONE responsive layout (the Dashboard.tsx/Containers.tsx rewrite pattern):
  // every block below is JSX-gated on `isDesktop` or `!isDesktop`, so exactly
  // one face of the page ever mounts — there is no CSS-hidden twin. The
  // dual-block shape this page's mobile port arrived in (hidden-by-utility
  // wrappers over a second mobile gate) double-rendered the list and kept a
  // hidden DOM copy in sync for nothing; killing it is the point of the
  // rewrite. jsdom's matchMedia stub answers "desktop", so unit tests pin the
  // desktop face and the phone face is exercised by the Playwright harness
  // (web/e2e/destination-vms.spec.ts).
  const isDesktop = useIsDesktop();
  // Advanced-mode flag read directly (not just via the <Advanced> wrapper
  // below): VMBackupOrderPanel's own hueIndex must only be resolved via
  // `nextHue()` when the panel will ACTUALLY render — see this function's
  // own `nextHue()` comment below for why a JSX child's props (including a
  // `hueIndex={nextHue()}` expression) evaluate eagerly as part of building
  // the <Advanced> element, regardless of whether <Advanced> itself goes on
  // to render null.
  const { advanced } = useAdvanced();
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();
  // Any backup, restore or replication in flight disables the bulk start
  // buttons and shows a hint.
  const running = anyActive(useProgress());
  const [vms, setVMs] = useState<VM[]>([]);
  const [loading, setLoading] = useState(true);
  // A failed page load replaces the list, so it stays inline rather than in a
  // toast.
  const [error, setError] = useState<string | null>(null);
  const [sortKey, setSortKey] = useState<SortKey>(loadSortKey);
  const [search, setSearch] = useState("");
  const [scheduleFilter, setScheduleFilter] = useState<ScheduleFilterKey>(loadScheduleFilterKey);
  const [backupFilter, setBackupFilter] = useState<BackupFilterKey>(loadBackupFilterKey);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [bulkBusy, setBulkBusy] = useState(false);
  const [discovering, setDiscovering] = useState(false);
  const [shakeDiscover, setShakeDiscover] = useState(0);

  async function handleDiscover() {
    setDiscovering(true);
    try {
      const res = await discoverVMs();
      // Both paths reload the list and name what was left out. Named
      // repositories are searched before the domain's own, so a failed pass can
      // still have written real rows, and this page does not poll.
      if (res.skipped?.length) {
        // Without this, "+0" looks the same whether every repository was read
        // or one was switched off, unresolvable or on a share that did not
        // mount.
        push(t("common.discoverSkipped").replace("{list}", res.skipped.join(", ")), "warn");
      }
      if (res.ok) {
        push(`+${res.discovered ?? 0}`, "success");
      } else {
        push(res.error ?? t("common.discoverFailed"), "fail");
        setShakeDiscover((n) => n + 1);
      }
      await loadVMs();
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.discoverFailed"), "fail");
      setShakeDiscover((n) => n + 1);
    } finally {
      setDiscovering(false);
    }
  }

  function loadVMs() {
    return listVMs()
      .then((res) => {
        if (res.ok) {
          setVMs(res.vms ?? []);
          // A transient failure (a daemon restart, a proxy 502) must not leave
          // its banner above a reloaded list, hiding the empty state.
          setError(null);
        } else setError(t("vms.loadFailed"));
      })
      .catch(() => setError(t("vms.loadFailed")));
  }

  useEffect(() => {
    void loadVMs().finally(() => setLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps -- t() is only read to build a failure message; re-fetching on a language switch would be a wasted round-trip

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

  // Search and the schedule and backup filters form one predicate, applied
  // before sorting and the split into live VMs and orphans. VMs have no image,
  // so the search matches the name only.
  const query = search.trim().toLowerCase();
  const filtered = useMemo(() => vms.filter((v) => {
    if (query && !v.name.toLowerCase().includes(query)) return false;
    if (scheduleFilter === "scheduled" && !v.includeInSchedule) return false;
    if (scheduleFilter === "notScheduled" && v.includeInSchedule) return false;
    if (backupFilter === "backedUp" && v.lastBackup == null) return false;
    if (backupFilter === "neverBackedUp" && v.lastBackup != null) return false;
    return true;
  }), [vms, query, scheduleFilter, backupFilter]);

  // The filters persist in localStorage, so a restored value could shrink the
  // list behind the collapsed Filters button; the trigger's dot shows it. Sort
  // never hides rows, so it does not count.
  const filtersActive =
    query !== "" || scheduleFilter !== "all" || backupFilter !== "all";

  const sorted = useMemo(() => sortVMs(filtered, sortKey), [filtered, sortKey]);
  const live = sorted.filter((v) => v.state !== "not-installed");
  const orphans = sorted.filter((v) => v.state === "not-installed");
  // Unfiltered, so a search cannot hide the entry to link. An entry under
  // another entry's former name is left out, as on the container page.
  const formerNames = new Set(vms.flatMap((v) => v.aliases ?? []));
  const notInstalledNames = vms
    .filter((v) => v.state === "not-installed" && !formerNames.has(v.libvirtName))
    .map((v) => v.libvirtName);

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

  // Drop selected VMs that a search or filter hides, so the bulk count stays
  // right and a bulk restore never overwrites a VM the user cannot see.
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
    // The containers key, since the sentence is domain-neutral.
    push(
      t("containers.bulkResult").replace("{ok}", String(ok)).replace("{fail}", String(fail)),
      fail > 0 ? "warn" : "success"
    );
    setSelected(new Set());
    void loadVMs();
  }

  // Backups and restores are asynchronous and share the server's single-flight
  // guard, so a tight loop would get "already running" after the first call.
  // fireAndWaitRun starts one run (retrying briefly while the previous guard
  // releases) and waits for that run, matched by run id rather than by the
  // client clock, to finish.
  function backupSelected() {
    void runBulk((name) =>
      fireAndWaitRun({
        kind: "backup",
        matchRun: (r) => r.domain === "vm" && r.target === name,
        start: () => backupVMNow(name),
        t,
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
        t,
      })
    );
  }

  // Hands out heading palette positions in the order the JSX evaluates
  // nextHue(), reset on every render. Each call sits at its call site and
  // only runs for a heading that will render.
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    // Same shell as Containers; eslint.config.js carries the
    // PAGE_SHELL_RESPONSIVE exception.
    <div className={PAGE_SHELL_RESPONSIVE}>
      {/* Page heading + Discover (disaster-recovery) action, at both
          widths, the same header the Containers page renders. */}
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
        {/* Discover is the way back after a lost database, so it stays
            whether or not VM backups are switched on, as on Containers. */}
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
            title={t("vms.discoverHint")}
            className={shakeDiscover ? "glim-shake" : ""}
          />
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
              BombVault-managed list to add to. The page's own Discover action
              (disaster-recovery re-scan, the header button above) is already the
              relevant action for an empty result, so a second button here
              would be redundant. */}
          <EmptyStateIcon icon={IconVM} />
          <p className="text-sm text-carbon-textMuted">{t("vms.empty")}</p>
        </div>
      )}

      {/* VM backup-order panel (#119, VMs) — advanced, DESKTOP-ONLY: a
          drag-reorder editor has no phone face (mobile edits each VM's
          schedule on the card's own sheet instead), and the phone never
          mounts it — exactly one face per width.
          `advanced ? nextHue() : undefined`, not a bare `nextHue()` inside
          <Advanced>: a JSX child's own props (this `hueIndex` expression
          included) evaluate eagerly as part of building the <Advanced>
          element itself, before <Advanced> ever runs its own `advanced &&
          when` check — so an unconditional `nextHue()` here would burn a
          slot every render regardless of whether the panel actually paints,
          landing the not-installed section's own notch below one index late
          whenever Advanced mode is off. Gating on the same `advanced` flag
          read directly above keeps the counter honest: only increment for a
          notch that will actually render, exactly like Dashboard.tsx's own
          advancedOnly blocks pre-filtering before ever calling nextHue(). */}
      {isDesktop && !loading && !error && (
        <Advanced>
          <VMBackupOrderPanel vms={vms} t={t} hueIndex={advanced ? nextHue() : undefined} />
        </Advanced>
      )}

      {/* Controls: Filters popover (search + schedule/backup filters + sort) + select-all.
          Desktop face — below the breakpoint the mobile block's ListToolbar
          carries the SAME state (search/chips/sort) on the shared primitives.
          One predicate, two presentations; the `isDesktop` gate mounts exactly
          one of them per width. */}
      {isDesktop && !loading && vms.length > 0 && (
        <div className="flex items-center gap-x-6 gap-y-2 flex-wrap">
          <FilterPopover label={t("filter.button")} active={filtersActive}>
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t("vms.searchPlaceholder")}
              spellCheck={false}
              autoComplete="off"
              className="w-full rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
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
            <VMSelectAll t={t} checked={allLiveSelected} onChange={toggleSelectAll} />
          )}
          {live.length > 0 && (
            <div className="ms-auto">
              <ScheduleIncludeAllControl t={t} onChanged={() => void loadVMs()} />
            </div>
          )}
        </div>
      )}

      {/* The phone block renders the same bar under its summary line. */}
      {isDesktop && !loading && selected.size > 0 && (
        <VMBulkBar
          t={t}
          count={selected.size}
          busy={bulkBusy}
          running={running}
          onBackup={backupSelected}
          onRestore={() => void restoreSelected()}
          onClear={() => setSelected(new Set())}
        />
      )}

      {/* Live VMs — the desktop list, JSX-gated: at >=48rem this is the list;
          below it the phone gets the card list in the mobile block instead and
          these full row cards (with their inline editors' weight) never mount
          at all — the point of the gate, versus a CSS-hidden second copy. */}
      {isDesktop && !loading && live.length > 0 && (
        <div className="flex flex-col gap-3 glim-content-fade">
          {live.map((v, i) => (
            <VMRow
              key={v.libvirtName}
              vm={v}
              t={t}
              onRefresh={() => void loadVMs()}
              selected={selected.has(v.libvirtName)}
              onToggleSelect={() => toggleSelect(v.libvirtName)}
              linkCandidates={notInstalledNames}
              index={i}
            />
          ))}
        </div>
      )}

      {/* Orphan VMs — no longer defined on the host but still have backups.
          Desktop face — the phone's card list renders its own not-installed
          section inline (see the mobile block below). */}
      {isDesktop && !loading && orphans.length > 0 && (
        <div className="flex flex-col gap-3 glim-content-fade">
          <NotInstalledHeading tip={t("vms.notInstalledHint")} hueIndex={nextHue()} t={t} />
          {/* Continues the live list's colour sequence, so the first orphan
              does not repeat the first live row's colour. */}
          {orphans.map((v, i) => (
            <VMRow key={v.libvirtName} vm={v} t={t} onRefresh={() => void loadVMs()} index={live.length + i} />
          ))}
        </div>
      )}

      {/* The phone face is mounted only below 48rem rather than hidden, so
          the settings read it owns never reaches the desktop. */}
      {!isDesktop && (
        <MobileVMsBlock
          sorted={sorted}
          liveCount={live.length}
          loading={loading}
          error={error !== null}
          onRetry={() => void loadVMs()}
          search={search}
          onSearch={setSearch}
          scheduleFilter={scheduleFilter}
          onScheduleFilterChange={handleScheduleFilterChange}
          backupFilter={backupFilter}
          onBackupFilterChange={handleBackupFilterChange}
          sortKey={sortKey}
          onSortChange={handleSortChange}
          running={running}
          onRefresh={() => void loadVMs()}
          linkCandidates={notInstalledNames}
          selected={selected}
          onToggleSelect={toggleSelect}
          allLiveSelected={allLiveSelected}
          onToggleSelectAll={toggleSelectAll}
          onBackupSelected={backupSelected}
          onRestoreSelected={() => void restoreSelected()}
          onClearSelection={() => setSelected(new Set())}
          bulkBusy={bulkBusy}
          onIncludeAllChanged={() => void loadVMs()}
          vms={vms}
        />
      )}

      {/* No VM matches the active search / schedule / backup filters. Shared:
          rendered at every width, the same sentence either face's own filter
          chrome sits above. */}
      {!loading && !error && noMatch && (
        <p className="text-sm text-carbon-textMuted">{t("filter.noMatch")}</p>
      )}
      {confirmDialog}
    </div>
  );
}

// MobileVMsBlock is the VMs page below 48rem. It is its own component so the
// one read only the phone needs (whether VM backups are switched on) stays
// off the desktop. The filter state is the page's own, so the phone toolbar
// and the desktop popover drive the same predicate.
//
// It hosts the one RunDetailSheet for its cards. onRun fires on every poll,
// so the sheet always holds the freshest record, and a sheet the user
// dismissed stays closed through later polls of the same run.
function MobileVMsBlock({
  sorted,
  liveCount,
  loading,
  error,
  onRetry,
  search,
  onSearch,
  scheduleFilter,
  onScheduleFilterChange,
  backupFilter,
  onBackupFilterChange,
  sortKey,
  onSortChange,
  running,
  onRefresh,
  linkCandidates,
  selected,
  onToggleSelect,
  allLiveSelected,
  onToggleSelectAll,
  onBackupSelected,
  onRestoreSelected,
  onClearSelection,
  bulkBusy,
  onIncludeAllChanged,
  vms,
}: {
  /** The page's memoized filtered+sorted list; useLoadMore's identity
   *  contract needs a stable array identity across unrelated renders. */
  sorted: VM[];
  /** Live (non not-installed) count for the summary line. */
  liveCount: number;
  /** The list is still loading (gates the toolbar, mirroring the desktop
   *  controls row's own `!loading` condition). */
  loading: boolean;
  /** Page-level load failure (the shared error paragraph carries the text;
   *  this block adds the mobile retry row). */
  error: boolean;
  onRetry: () => void;
  search: string;
  onSearch: (next: string) => void;
  scheduleFilter: ScheduleFilterKey;
  onScheduleFilterChange: (k: ScheduleFilterKey) => void;
  backupFilter: BackupFilterKey;
  onBackupFilterChange: (k: BackupFilterKey) => void;
  sortKey: SortKey;
  onSortChange: (k: SortKey) => void;
  running: { active: boolean; phase?: string };
  onRefresh: () => void;
  /** The not-installed names the detail's link picker offers. */
  linkCandidates: string[];
  /** Bulk-selection state over libvirt names, shared with the desktop face. */
  selected: ReadonlySet<string>;
  onToggleSelect: (name: string) => void;
  /** True when every live VM is ticked (the select-all checkbox). */
  allLiveSelected: boolean;
  onToggleSelectAll: () => void;
  onBackupSelected: () => void;
  onRestoreSelected: () => void;
  onClearSelection: () => void;
  bulkBusy: boolean;
  /** The include-all switch's refresh: the page reloads its list. */
  onIncludeAllChanged: () => void;
  /** The full list payload, for the advanced backup order panel. */
  vms: VM[];
}) {
  const { t } = useT();

  // Whether VM backups are switched on, read here rather than by the page so
  // the desktop, which never mounts this block, makes no extra request. An
  // unknown or failed read counts as on: the real list and its own error
  // surfaces are honest, a claim that the feature is off might not be.
  const [gate, setGate] = useState<"unknown" | "on" | "off">("unknown");
  useEffect(() => {
    let alive = true;
    getSettings()
      .then((r) => {
        if (alive) setGate(r.ok && r.settings.vmsEnabled === false ? "off" : "on");
      })
      .catch(() => {
        if (alive) setGate("on");
      });
    return () => {
      alive = false;
    };
  }, []);

  // The open detail holds the VM's name, not the object taken at tap time, and
  // reads the VM out of the unfiltered list: a reload brings fresh data, and a
  // change made in the detail can take the VM out of the current filter.
  const [openVmName, setOpenVmName] = useState<string | null>(null);
  const openVm = openVmName === null ? null : vms.find((v) => v.libvirtName === openVmName) ?? null;
  const listScrollRef = useRef(0);
  const restoreScrollRef = useRef(false);

  function openCard(vm: VM) {
    listScrollRef.current = document.getElementById("bv-main")?.scrollTop ?? 0;
    setOpenVmName(vm.libvirtName);
    // After the detail commits, the page reads from the top (the back row is
    // the first thing on screen).
    requestAnimationFrame(() => {
      document.getElementById("bv-main")?.scrollTo(0, 0);
    });
  }

  function closeDetail() {
    if (openVmName === null) return;
    restoreScrollRef.current = true;
    setOpenVmName(null);
  }

  useEffect(() => {
    if (openVm !== null || !restoreScrollRef.current) return;
    restoreScrollRef.current = false;
    document.getElementById("bv-main")?.scrollTo(0, listScrollRef.current);
  }, [openVm]);

  // Every save, finished backup and takeover reloads the list and hands this
  // a new array. Keyed on the filter state alone, the window keeps the
  // reader's place (and the card being edited mounted) through a reload, and
  // a changed filter still rewinds.
  const windowKey = [search.trim().toLowerCase(), scheduleFilter, backupFilter, sortKey].join("\u0000");
  const { visible, showMore, hasMore } = useLoadMore(sorted, 20, windowKey);
  const mobileLive = visible.filter((v) => v.state !== "not-installed");
  const mobileOrphans = visible.filter((v) => v.state === "not-installed");
  // A removed VM only logs a skip when its run comes, so it does not count as
  // scheduled, as on the Containers page.
  const scheduledCount = sorted.filter((v) => v.state !== "not-installed" && v.includeInSchedule).length;
  // While the detail is open the list is off the tree (its scroll position is
  // saved and restored on Back), so the rows the window kept are the detail's
  // to render through.
  const listChromeHidden = openVm !== null;

  return (
    <div className="flex flex-col gap-4 glim-content-fade">
      {/* The VM detail, in the frame the Containers detail renders, above
          where the list sits; the list is off the tree while it is open. */}
      {openVm !== null && (
        <MobileVMDetail
          vm={openVm}
          t={t}
          running={running}
          onRefresh={onRefresh}
          onBack={closeDetail}
          linkCandidates={linkCandidates}
        />
      )}

      {/* Page-level load failure: the shared error paragraph above carries the
          message; this >=44px tonal row is the mobile recovery affordance.
          folders.retry ("Try again") is the sanctioned existing label; no
          new key needed. */}
      {error && !listChromeHidden && (
        <button
          type="button"
          onClick={onRetry}
          className="min-h-[2.75rem] w-full rounded-control bg-carbon-surface2 px-4 text-sm text-carbon-text"
        >
          {t("folders.retry")}
        </button>
      )}

      {gate === "off" ? (
        // The destinations gate, mobile face: the block says plainly that VM
        // backups are off and links to the settings row that turns them on.
        // A gated surface shows nothing else: no toolbar, no cards.
        <div className="relative glim-notch-card">
          <MobileSectionLabel t={t} labelKey="settings.vmsEnabled" />
          <div className="flex flex-col gap-2 rounded-card bg-carbon-surface p-4 pt-5">
            <p className="text-sm text-carbon-textSub">{t("settings.vmsEnabledHint")}</p>
            <Link
              to="/settings"
              className="flex min-h-[2.75rem] items-center rounded-control bg-carbon-surface2 px-4 text-sm text-carbon-text"
            >
              {t("nav.settings")}
            </Link>
          </div>
        </div>
      ) : (
        <>
          {/* Summary counts line, the Containers.tsx mobile list precedent:
              derived from the same list payload the desktop reads, no new
              endpoint, no new key. */}
          {liveCount > 0 && !listChromeHidden && (
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-xs text-carbon-textMuted">
                {`${liveCount} ${t("nav.vms")}${
                  scheduledCount > 0 ? ` · ${scheduledCount} ${t("filter.scheduled")}` : ""
                }`}
              </p>
              <div className="flex flex-wrap items-center gap-3">
                <VMSelectAll t={t} checked={allLiveSelected} onChange={onToggleSelectAll} />
                <ScheduleIncludeAllControl t={t} onChanged={onIncludeAllChanged} />
              </div>
            </div>
          )}

          {!loading && !error && !listChromeHidden && (
            <Advanced>
              <VMBackupOrderPanel vms={vms} t={t} hueIndex={0} />
            </Advanced>
          )}

          {!loading && !listChromeHidden && selected.size > 0 && (
            <VMBulkBar
              t={t}
              count={selected.size}
              busy={bulkBusy}
              running={running}
              onBackup={onBackupSelected}
              onRestore={onRestoreSelected}
              onClear={onClearSelection}
            />
          )}

          {/* The ONE toolbar: lifted page state, shared chip filters. Rendered
              once the first load has settled (mirrors the desktop controls
              row), including when filters currently match nothing; a cleared
              search must remain clearable. */}
          {!loading && !error && !listChromeHidden && (
            <ListToolbar search={search} onSearch={onSearch} placeholder="vms.searchPlaceholder">
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
            </ListToolbar>
          )}

          {!listChromeHidden && (
            <>
              {mobileLive.map((v, i) => (
                <MobileVMCard
                  key={v.libvirtName}
                  vm={v}
                  t={t}
                  index={i}
                  selected={selected.has(v.libvirtName)}
                  onToggleSelect={() => onToggleSelect(v.libvirtName)}
                  onOpen={() => openCard(v)}
                />
              ))}

              {mobileOrphans.length > 0 && (
                <div className="flex flex-col gap-4 pt-2">
                  {/* The shared heading the Containers phone list renders: the
                      hint rides the badge's (i), not an open paragraph. */}
                  <NotInstalledHeading tip={t("vms.notInstalledHint")} hueIndex={0} t={t} />
                  {/* No selection here, as on the desktop: a bulk backup of a
                      removed VM parks a run for hours. */}
                  {mobileOrphans.map((v, i) => (
                    <MobileVMCard
                      key={v.libvirtName}
                      vm={v}
                      t={t}
                      index={liveCount + i}
                      onOpen={() => openCard(v)}
                    />
                  ))}
                </div>
              )}

              {/* The one load-more affordance, gated on hasMore: no rows beyond
                  the window, no button (hasMore is the only signal this may gate
                  on). */}
              {hasMore && (
                <button
                  type="button"
                  onClick={showMore}
                  className="min-h-[2.75rem] w-full rounded-control bg-carbon-surface2 px-4 text-sm text-carbon-text"
                >
                  {t("common.loadMore")}
                </button>
              )}
            </>
          )}
        </>
      )}
    </div>
  );
}

// MobileVMCard is one VM as the phone row the Containers list renders too,
// with the backup method as its meta line. Every control lives in the
// detail the row opens.
function MobileVMCard({
  vm,
  t,
  index,
  onOpen,
  selected,
  onToggleSelect,
}: {
  vm: VM;
  t: T;
  index: number;
  onOpen: () => void;
  selected?: boolean;
  onToggleSelect?: () => void;
}) {
  const installed = vm.state !== "not-installed";
  return (
    <MobileListCard
      title={vm.name}
      meta={
        installed ? (vm.method === "live" ? t("vm.method.live") : t("vm.method.graceful")) : undefined
      }
      badge={
        installed ? (
          <Badge tone={stateTone(vm.state)}>{stateLabel(t, vm.state)}</Badge>
        ) : (
          <Badge tone="neutral">{t("containers.notInstalled")}</Badge>
        )
      }
      hueIndex={index}
      onOpen={onOpen}
      selected={selected}
      onToggleSelect={onToggleSelect}
    />
  );
}

// MobileVMDetail is the stacked VM detail: the shared detail shell with the
// desktop row's controls inside. It hosts the RunDetailSheet for its own
// deep-linked runs, the way the Containers detail does; onRun fires on
// every poll, so the sheet always holds the freshest record, and a sheet
// the user dismissed stays closed through later polls of the same run.
function MobileVMDetail({
  vm,
  t,
  running,
  onRefresh,
  onBack,
  linkCandidates,
}: {
  vm: VM;
  t: T;
  running: { active: boolean; phase?: string };
  onRefresh: () => void;
  onBack: () => void;
  /** The not-installed entries this detail can take over by hand. */
  linkCandidates: string[];
}) {
  const { push } = useToast();
  // Reverted on a failed save, so the picker never shows a destination the
  // server did not accept.
  const [repoChoice, setRepoChoice] = useState(vm.repo ?? "");
  useEffect(() => {
    setRepoChoice(vm.repo ?? "");
  }, [vm.repo]);
  const progressMap = useProgress();
  const progress = progressMap[`vm:${vm.libvirtName}`];
  const installed = vm.state !== "not-installed";
  // The same one-section disclosure the desktop row keeps (snapshots and
  // restore live behind it), through the same Set rule ContainerRow uses.
  const [openSections, setOpenSections] = useState<Set<string>>(() => new Set());
  function toggleSection(id: string) {
    setOpenSections((prev) => (prev.has(id) ? new Set() : new Set([id])));
  }
  const aliases = vm.aliases ?? [];
  const takeoverEntry = { name: vm.libvirtName, displayName: vm.name, api: vmTakeover };
  // The run-sheet latch, the Containers detail's, verbatim.
  const [sheetRun, setSheetRun] = useState<Run | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);
  const sheetDismissed = useRef(false);
  const lastCorrelatedRun = useRef<string | null>(null);

  return (
    <MobileDetailShell title={vm.name} onBack={onBack}>
      {/* The takeover pair, as the desktop row renders it. */}
      {installed && vm.renameFrom && (
        <RenameTakeoverRow
          key={vm.renameFrom}
          from={vm.renameFrom}
          reason={vm.renameReason ?? ""}
          entry={takeoverEntry}
          onDone={onRefresh}
          t={t}
        />
      )}
      {aliases.length > 0 && (
        <FormerNames
          aliases={aliases}
          conflicts={vm.aliasConflicts ?? []}
          entry={takeoverEntry}
          onDone={onRefresh}
          t={t}
        />
      )}

      {/* The desktop card's corner, verbatim: a not-installed entry gets the
          removal control (#232) instead of a backup trigger; backing up a
          VM that no longer exists only parks a run in flight for hours. */}
      {!installed && (
        <OrphanRemoveButton
          hasBackups={vm.lastBackup != null}
          deleteConfirm={t("vms.deleteBackupsConfirm")}
          removeConfirm={t("vms.removeEntryConfirm")}
          deleteBackups={() => deleteBackupsVM(vm.libvirtName, "local")}
          removeEntry={() => forgetVM(vm.libvirtName)}
          onDone={onRefresh}
          t={t}
        />
      )}

      {/* Last-run line, the desktop row's combined line, verbatim keys. */}
      <p className="text-xs text-carbon-textMuted">
        {`${t("containers.lastBackup")}: ${vm.lastBackup ? formatTs(vm.lastBackup) : t("containers.never")}`}
      </p>

      {/* The start of the actions row is free, so the link picker takes it,
          as on the desktop row: a card with former names is already linked,
          even before its first run. */}
      {installed && vm.lastBackup == null && aliases.length === 0 && linkCandidates.length > 0 && (
        <LinkEntryPicker candidates={linkCandidates} entry={takeoverEntry} onDone={onRefresh} t={t} />
      )}

      {installed && (
        <div className="flex items-center justify-between gap-4 flex-wrap">
          <div className="flex items-center gap-2">
            <span className="flex items-center gap-1 text-xs text-carbon-textSub">
              {t("vm.method")}
              <InfoBubble tip={t("vm.method.hint")} />
            </span>
            <VMMethodSelect name={vm.libvirtName} initial={vm.method} t={t} />
          </div>
          <div className="flex items-center gap-1.5">
            <VMBackupButton
              name={vm.libvirtName}
              t={t}
              running={running}
              onBackedUp={onRefresh}
              onRunCorrelated={(run) => {
                if (lastCorrelatedRun.current !== run.id) {
                  lastCorrelatedRun.current = run.id;
                  sheetDismissed.current = false; // new fire re-arms the deep-link
                }
                setSheetRun(run);
                if (!sheetDismissed.current) setSheetOpen(true);
              }}
            />
            <Advanced>
              <VMExportButton name={vm.libvirtName} t={t} />
            </Advanced>
          </div>
        </div>
      )}

      {/* The schedule switch, the desktop card's own: without it there is no
          way to schedule or unschedule a VM from a phone at all. It shows on
          a removed VM too: the entry stays scheduled and every run logs a
          skip until this switch goes off. */}
      <IncludeToggle name={vm.libvirtName} initial={vm.includeInSchedule} save={setVMInclude} />

      {/* ContainerRow's disclosure block with a single section. */}
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2 flex-wrap">
          <Selector
            items={[{ id: "backups", label: t("snapshots.title") }]}
            label={t("containers.sectionsLabel")}
            select="many"
            active={openSections}
            buttonHeight
            onChange={toggleSection}
          />
          <span className="ms-auto shrink-0 text-xs text-carbon-textMuted whitespace-nowrap">
            {`${t("containers.lastBackup")}: ${vm.lastBackup ? formatTs(vm.lastBackup) : t("containers.never")}`}
          </span>
        </div>
        {/* Where this VM's backups go. Locked once the VM has backups: they
            stay in the repository they were written to. */}
        <Advanced>
          <RepoPicker
            value={repoChoice}
            onChange={(next) => {
              const before = repoChoice;
              setRepoChoice(next);
              void setVMRepo(vm.libvirtName, next).then((r) => {
                if (r.ok) {
                  push(t("folders.saved"), "success");
                  return;
                }
                push(r.error ?? t("settings.error"), "fail");
                setRepoChoice(before);
              });
            }}
            locked={vm.lastBackup != null}
          />
        </Advanced>

        <VMRestorePanel
          name={vm.libvirtName}
          displayName={vm.name}
          t={t}
          open={openSections.has("backups")}
        />
      </div>

      {/* Not during a restore, which has its own cancel in the Backups panel,
          and only while the run is active, so a finished run leaves no button
          behind. */}
      {progress && progress.active && progress.phase !== "restore" && (
        <div className="flex justify-end">
          <BackupCancelButton cancelKey={`vm:${vm.libvirtName}`} name={vm.name} t={t} />
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

      {/* The run sheet, hosted component-locally: opens on the useBackupWatch
          baseline-id correlation, closes through BottomSheet's three paths,
          and never re-opens itself after dismissal. */}
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
