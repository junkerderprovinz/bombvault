import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { listVMs, backupVMNow, restoreVM, listVMSnapshots, setVMInclude, setVMIncludeAll, setVMMethod, deleteSnapshot, deleteBackupsVM, forgetVM, discoverVMs, exportVM, getVmBackupOrder, setVmBackupOrder, setVMRepo } from "../lib/api";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { FilterPopover } from "../components/FilterPopover";
import { IconTipButton } from "../components/IconTipButton";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import type { VM, Snapshot, VmOrder } from "../lib/api";
import { BULK_HUE } from "../lib/bulkHue";
import { useT, stateLabel } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
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
        variant="well"
        select="one"
        active={value}
        onChange={(id) => onChange(id as K)}
      />
    </div>
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
}: {
  name: string;
  t: T;
  onBackedUp?: () => void;
  /** Whether any operation is running (anyActive). It blocks this backup
   *  while another one runs, but never because of its own (isPending). */
  running?: { active: boolean; phase?: string };
}) {
  // The server backs the VM up detached and answers at once, so the outcome
  // comes from watching the "vm:<name>" progress and the recorded run.
  const { state, fire, isPending } = useBackupWatch({
    progressKey: `vm:${name}`,
    start: () => backupVMNow(name),
    matchRun: (r) => r.domain === "vm" && r.target === name,
    onDone: onBackedUp,
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
        {installed && vm.lastBackup == null && linkCandidates.length > 0 && (
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
  // Read directly rather than relying on <Advanced>: the order panel's
  // hueIndex={nextHue()} is evaluated when the element is built, even if
  // <Advanced> then renders nothing, so nextHue() may only run when the panel
  // will render.
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
  const filtered = vms.filter((v) => {
    if (query && !v.name.toLowerCase().includes(query)) return false;
    if (scheduleFilter === "scheduled" && !v.includeInSchedule) return false;
    if (scheduleFilter === "notScheduled" && v.includeInSchedule) return false;
    if (backupFilter === "backedUp" && v.lastBackup == null) return false;
    if (backupFilter === "neverBackedUp" && v.lastBackup != null) return false;
    return true;
  });

  // The filters persist in localStorage, so a restored value could shrink the
  // list behind the collapsed Filters button; the trigger's dot shows it. Sort
  // never hides rows, so it does not count.
  const filtersActive =
    query !== "" || scheduleFilter !== "all" || backupFilter !== "all";

  const sorted = sortVMs(filtered, sortKey);
  const live = sorted.filter((v) => v.state !== "not-installed");
  const orphans = sorted.filter((v) => v.state === "not-installed");
  // Unfiltered, so a search cannot hide the entry to link.
  const notInstalledNames = vms.filter((v) => v.state === "not-installed").map((v) => v.libvirtName);

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
    <div className={PAGE_SHELL}>
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
          {/* No Add action: the list is what libvirt reports, and Discover
              above is the action for an empty result. */}
          <EmptyStateIcon icon={IconVM} />
          <p className="text-sm text-carbon-textMuted">{t("vms.empty")}</p>
        </div>
      )}

      {/* Gated on `advanced` as well: the hueIndex expression is evaluated
          even when <Advanced> renders nothing, and would push the
          not-installed heading one position late. */}
      {!loading && !error && (
        <Advanced>
          <VMBackupOrderPanel vms={vms} t={t} hueIndex={advanced ? nextHue() : undefined} />
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
          <Button
            label={t("vms.backupSelected")}
            labelKey="vms.backupSelected"
            hueIndex={BULK_HUE.backup}
            tone="accent"
            onClick={backupSelected}
            disabled={bulkBusy || running.active}
          />
          {/* Bulk restore is advanced-only; bulk backup stays basic. */}
          <Advanced>
            <Button
              label={t("vms.restoreSelected")}
              labelKey="vms.restoreSelected"
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
          {bulkBusy && (
            <span className="text-xs text-carbon-textMuted">{t("containers.working")}</span>
          )}
          {!bulkBusy && running.active && (
            <span className="text-xs text-carbon-textMuted">
              {t(busyPhraseKey(running.phase))}
            </span>
          )}
        </div>
      )}

      {/* Live VMs */}
      {!loading && live.length > 0 && (
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

      {/* VMs gone from the host that still have backups. */}
      {!loading && orphans.length > 0 && (
        <div className="flex flex-col gap-3 glim-content-fade">
          <NotInstalledHeading tip={t("vms.notInstalledHint")} hueIndex={nextHue()} t={t} />
          {/* Continues the live list's colour sequence, so the first orphan
              does not repeat the first live row's colour. */}
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
