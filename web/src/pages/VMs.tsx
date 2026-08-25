import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { listVMs, backupVMNow, restoreVM, listVMSnapshots, setVMInclude, setVMIncludeAll, setVMMethod, deleteSnapshot, deleteBackupsVM, forgetVM, discoverVMs, exportVM, getVmBackupOrder, setVmBackupOrder } from "../lib/api";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { FilterPopover } from "../components/FilterPopover";
import { IconTipButton } from "../components/IconTipButton";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import type { VM, Snapshot, VmOrder } from "../lib/api";
import { useT, stateLabel } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { useDragReorder } from "../lib/useDragReorder";
import { Advanced, useAdvanced } from "../lib/advanced";
import { ProgressBar } from "../components/ProgressBar";
import { RestoreAction } from "../components/restore/RestoreAction";
import { RecentRunsList } from "../components/RecentRunsList";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconVM, IconRestore, IconTrash, IconBackupNow, IconDownload } from "../components/Sidebar";
import { InfoBubble } from "../components/InfoBubble";
import { Badge, type BadgeTone } from "../components/Badge";
// ToggleRow, not the bare Toggle: VMIncludeToggle renders the shared row
// (label + switch) rather than a naked switch its caller labels by hand — the
// same import components/IncludeToggle.tsx already uses for the Container
// tab's copy of that control.
import { ToggleRow } from "./Settings";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch, fireAndWaitRun } from "../lib/backupWatch";
import { useConfirm } from "../lib/useConfirm";
import { hueVars, rainbowAt } from "../lib/appearance";
import { useRainbow } from "../lib/useRainbow";
import { Selector } from "../components/Selector";
import { useToast } from "../lib/toast";

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
  const { push } = useToast();

  async function handleChange(next: string) {
    setBusy(true);
    try {
      const res = await setVMMethod(name, next);
      if (res.ok) {
        setMethod(next);
      } else {
        // Surface the failure instead of silently reverting — a swallowed error
        // here means the user thinks they switched to live (no downtime) when the
        // VM will actually be shut down at backup time.
        push(res.error ?? t("vm.method.saveFailed"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("vm.method.saveFailed"), "fail");
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
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): a bumped
  // nonce, keyed onto the Toggle exactly like ToggleRow's own shakeNonce
  // prop, replays `.glim-shake` once per failure — see Settings.tsx's
  // ToggleRow for the fuller "why a nonce, not a boolean" reasoning.
  const [shake, setShake] = useState(0);

  // Re-seed when the parent passes a fresh value (e.g. after "Include all in
  // schedule" reloads the list). Rows are keyed by name and do not remount, so
  // without this the toggle would keep showing its stale pre-bulk state.
  useEffect(() => setEnabled(initial), [initial]);

  async function handleChange(next: boolean) {
    setBusy(true);
    try {
      const res = await setVMInclude(name, next);
      if (res.ok) {
        setEnabled(next);
      } else {
        push(res.error ?? t("schedule.updateFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("schedule.updateFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  // Renders through the SAME shared ToggleRow as components/IncludeToggle.tsx
  // (the Container tab's copy of this exact control) and Files.tsx's
  // FileSetEnabledToggle — all three converted together in the whole-app sweep.
  // See IncludeToggle.tsx for jdp's original ask ("Der Text soll immer ganz
  // links stehen und der Toggle ganz rechts sein") and Files.tsx's copy for
  // why this one was still the mirror image of it: a bare `hideLabel` Toggle
  // with a hand-rolled switch-first/text-second `<label>` at the call site.
  return (
    <ToggleRow
      label={t("containers.includeInSchedule")}
      checked={enabled}
      onChange={(next) => void handleChange(next)}
      disabled={busy}
      shakeNonce={shake}
    />
  );
}

// ---------------------------------------------------------------------------
// VM-aware BackupButton variant
// ---------------------------------------------------------------------------

// GlimStone follow-up pass (v8.0.0) audit note: deliberately NOT migrated to a
// toast — same reasoning as Containers.tsx's ExportButton, its exact twin.
// The "done"/"error" result below shows the actual export destination path (or
// the raw error), neither of which auto-dismissed before this pass; it's a
// reference value to copy down, not a one-shot ping.
function VMExportButton({ name, t }: { name: string; t: T }) {
  const [state, setState] = useState<"idle" | "pending" | "done" | "error">("idle");
  const [msg, setMsg] = useState<string | null>(null);
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed action toasts AND shakes its button, layered ON TOP of this
  // button's own pre-existing sticky inline error (kept deliberately — see
  // this component's header comment: the error text sits right next to the
  // success path's copyable destination, both a "reference value", not a
  // one-shot ping the toast alone would replace).
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
      {/* WHOLE-TREE SWEEP FINDING — square icon badge, mirroring
          Containers.tsx's ExportButton verbatim (same role, same glyph, same
          tone). This was the LAST plain-text Export button in the app: its
          Containers twin was converted with `tone="active"` specifically
          because the `bg-carbon-surface2` grey it used to carry (identical to
          this one's) takes NO colour-engine position at all, leaving it the
          single flat grey control in a card whose every other badge follows
          the accent/rainbow engine. Leaving this copy would have reproduced
          that exact "anders eingefärbt" report on the VM tab, one tab over.
            size="icon" = 32px, the app's ONE square-icon-badge stage — not
          re-measured against this button's own former text footprint.
          IconDownload reused verbatim. `tip` carries the label the glyph
          replaced. No hueIndex: VMRow's card already carries `.glim-hue` with
          this VM's list position.
            The sticky inline done/error text below is KEPT deliberately,
          exactly as Containers' ExportButton keeps its own: it holds a
          copyable destination path (a reference value the user reads off the
          screen), not a one-shot ping a toast would replace. The column flips
          items-start → items-end so the tile lines up with the card edge and
          the text hangs beneath it. */}
      <Badge
        key={shake}
        as="button"
        shape="square"
        size="icon"
        tone="active"
        tip={t("export.button")}
        onClick={() => void run()}
        disabled={state === "pending"}
        className={shake ? "glim-shake" : undefined}
      >
        {state === "pending" ? (
          <span
            className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
            style={{ borderColor: "currentColor", borderTopColor: "transparent" }}
          />
        ) : (
          <IconDownload />
        )}
      </Badge>
      {state === "done" && (
        <span className="text-xs text-statusOk break-all text-end max-w-[18rem]">{t("export.exportedTo")} {msg}</span>
      )}
      {state === "error" && <span className="text-xs text-statusFail break-all text-end max-w-[18rem]">{msg}</span>}
    </div>
  );
}

// WHOLE-TREE SWEEP FINDING — square icon badge, mirroring components/
// BackupButton.tsx (the Containers twin) verbatim: same role, same
// IconBackupNow glyph, same `shape="square" size="icon" tone="active"`
// recipe, same `tip` priority order (pending → blocked-by-other → label),
// same terminal-states-become-toasts trade.
//
// This was the LAST plain-text "Jetzt sichern" button in the app. Containers'
// was converted first, then Flash's, then Files' FileSetBackupButton (also as
// a sweep finding, not a named ask), then Config's in this same pass — which
// left this one alone rendering as text. It sits in the top-right corner of
// the VMRow card, the same card whose snapshot rows this pass just converted
// to 32px badges, so leaving it would have put text buttons and icon badges
// side by side in one card: exactly the "a user sees one card" contract
// Badge.tsx's "ONE SIZE FOR SQUARE ICON BADGES" block spells out, and exactly
// the report ("Jetzt sichern ist auf dem VM-Tab noch ein Text-Button") this
// round exists to pre-empt rather than collect for a fifth time.
//
// The inline states are gone with the text button that had room for them:
// success/error now toast (an error also shakes the badge), and the
// blocked-by-other hint moves into the `tip`, which is where the other four
// already put it. The nested `<Advanced><VMExportButton/></Advanced>` moved
// OUT to the call site, so the two badges sit side by side in the corner the
// way Containers.tsx's own BackupButton/ExportButton pair does, rather than
// one badge being stacked underneath the other inside its sibling's column.
//
// This supersedes the v8.0.0 audit note that used to sit here, which deferred
// the toast migration on the grounds that useBackupWatch's state shape also
// backs RESTORE outcomes (sticky by design, RestoreAction.tsx). That reasoning
// still correctly blocks changing the HOOK — untouched here — but rendering
// state.phase as a toast is a per-component decision, which
// components/BackupButton.tsx, Flash.tsx and Config.tsx have each now proved
// with zero hook changes. The old note read, for reference: splitting
// that shared, cross-file state machine's rendering by kind (backup vs.
// restore) is a hook-level architecture change, not the local flash-swap this
// pass does everywhere else, so it's left as its own deliberate follow-up.
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
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed action toasts AND shakes its button.
  const [shake, setShake] = useState(0);
  // Tracks the last phase already reported, so this effect toasts exactly
  // once per NEW terminal transition — same guard as components/
  // BackupButton.tsx (state.phase can only ever start at "idle", so this
  // never fires on mount, only on a real fire()-driven change).
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

  const tip = isPending
    ? t("common.backingUp")
    : blockedByOther
      ? t(busyPhraseKey(running?.phase))
      : t("containers.backupNow");

  return (
    <Badge
      key={shake}
      as="button"
      shape="square"
      tone="active"
      size="icon"
      tip={tip}
      onClick={() => void fire()}
      disabled={isPending || blockedByOther}
      className={shake ? "glim-shake" : undefined}
    >
      {isPending ? (
        <span
          className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
          style={{ borderColor: "currentColor", borderTopColor: "transparent" }}
        />
      ) : (
        <IconBackupNow />
      )}
    </Badge>
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
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed delete toasts AND shakes the delete button (bumped nonce → fresh
  // key → `.glim-shake` replays, same mechanism as ToggleRow's shakeNonce).
  const [shake, setShake] = useState(0);
  // Collapsed by default so the list stays compact (mirrors Containers'
  // SnapshotRow) — the restore controls (confirm + leave-stopped + progress
  // banner) only render once the user opts in.
  const [showRestore, setShowRestore] = useState(false);
  const { confirm, confirmDialog } = useConfirm();

  async function handleDelete() {
    if (!(await confirm(t("snapshots.deleteConfirm")))) return;
    setDeleting(true);
    try {
      const res = await deleteSnapshot("vms", snap.id, source);
      if (res.ok) onDeleted();
      else {
        push(res.error ?? "Delete failed", "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : "Delete failed", "fail");
      setShake((n) => n + 1);
    } finally {
      setDeleting(false);
    }
  }

  return (
    // py-1.5, not the py-2.5 this row used to carry — the identical trade
    // components/RestorePanel.tsx's SnapshotRow, pages/Config.tsx's
    // ConfigSnapshotRow, pages/Files.tsx's FileSetSnapshotRow and
    // pages/Flash.tsx's FlashSnapshotRow each already made: the controls grew
    // from ~24px text buttons to the app's one 32px square icon badge, and
    // trimming 4px of padding per side keeps the row at exactly the 44px it
    // measured before.
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
        {/* WHOLE-TREE SWEEP FINDING — this row was NOT in the brief.
            This round was scoped as "Flash's FlashSnapshotRow is the FOURTH
            and last copy of the row-action pattern already converted in
            RestorePanel, Config and Files". Grepping the tree for the
            pattern's own signature (`hover:bg-statusFailBg
            hover:text-statusFail` on a row-action text button) instead of
            trusting that count turned up a FIFTH: this one. Its own comment
            below says it "mirrors Containers' SnapshotRow" — it is a direct
            copy of the very row that was converted first, so it carried the
            identical defect the whole time and would have been reported as
            "the same bug, again" on the next VM-tab review.
              Both controls take the same recipe as all four siblings:
            shape="square" size="icon" (32px, Badge.tsx's ONE square-icon-badge
            stage), tone="active", NO hueIndex — VMRow's own card carries
            `.glim-hue` with this VM's list position, so the custom-property
            cascade paints both badges in that row's rainbow position. Glyphs
            are IconRestore and IconTrash, reused verbatim from RestorePanel's
            already-converted pair, and each badge carries a `tip` with the
            label its glyph replaced.
              The delete badge gets NO special colour: not the
            `hover:bg-statusFailBg hover:text-statusFail` red it carried, and
            not a grey neutral (neutral is exempt from the rainbow and would
            leave it flat beside a hued sibling). Meaning is carried by
            IconTrash, its tip, and the untouched confirm dialog. Its "…"
            in-flight label shows as `disabled` instead, exactly like the
            other four.
              The restore toggle's "highlighted while open"
            `bg-carbon-surface3` swap is dropped rather than layered onto
            Badge's own tone fill — equal-specificity Tailwind utilities
            resolve by stylesheet order, not className order, so that was
            never a safe override. The panel appearing below is the visible
            feedback, the same call RestorePanel's converted toggle made. */}
        <Badge
          as="button"
          shape="square"
          size="icon"
          tone="active"
          tip={t("restore.open")}
          onClick={() => setShowRestore((p) => !p)}
          className="shrink-0"
        >
          <IconRestore />
        </Badge>
        <Badge
          key={shake}
          as="button"
          shape="square"
          size="icon"
          tone="active"
          tip={t("snapshots.delete")}
          onClick={() => void handleDelete()}
          disabled={deleting || busy}
          className={`shrink-0${shake ? " glim-shake" : ""}`}
        >
          <IconTrash />
        </Badge>
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
  // Section-load error (list failed to load) — NOT migrated to a toast
  // (GlimStone follow-up pass, v8.0.0 audit note): it replaces the whole
  // snapshot-list content area, the same "the section failed to load"
  // structural condition as the page-level `error` in VMs()/Containers(), not
  // a one-shot button-click confirmation.
  const [error, setError] = useState<string | null>(null);

  const [reloadTick, setReloadTick] = useState(0);
  const [deletingAll, setDeletingAll] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the "Delete all" control on a failed delete, alongside the toast below.
  const [shakeDeleteAll, setShakeDeleteAll] = useState(0);

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

  // GlimStone follow-up pass (v8.0.0): "Delete all" is a one-shot action
  // failure — was ALSO routed through the section-load `error` above, but the
  // .finally() below unconditionally bumps reloadTick, which re-fires the
  // effect and clears `error` again almost immediately (setError(null) at the
  // top of that effect) — so this failure was already near-invisible before
  // this pass, a latent dead branch this migration also resolves (same shape
  // as 0d4d195's CloudCredSetsCard fix). A toast survives that reload.
  async function handleDeleteAll() {
    // TODO(#follow-up): richer stake-detail copy ("N snapshots, X GB") belongs
    // here once it ships (deferred — new interpolated i18n keys across all 25
    // non-English locales, out of scope for this window.confirm() → dialog
    // mechanism swap). Same flagged follow-up as Containers.tsx's
    // deleteBackupsConfirm and Files.tsx's deleteBackupsConfirm.
    if (!(await confirm(t("snapshots.deleteAllConfirm")))) return;
    setDeletingAll(true);
    deleteBackupsVM(name, source)
      .then((res) => {
        if (!res.ok) {
          push(res.error ?? "Failed to delete backups", "fail");
          setShakeDeleteAll((n) => n + 1);
        }
      })
      .catch(() => {
        push("Failed to delete backups", "fail");
        setShakeDeleteAll((n) => n + 1);
      })
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
          <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
        </svg>
        {t("snapshots.title")}
      </button>

      {open && (
        <div className="mt-2 rounded-card bg-carbon-background px-3 py-1">
          {/* `source.hint` moved from a permanent `text-caption` <p> under
              this row onto the "Quelle" label as an InfoBubble — rule 8's
              "read once, costs vertical space forever" case, the same
              conversion Flash.tsx got in 63f53d5 and the other three copies
              (components/RestorePanel.tsx, pages/Config.tsx, pages/Files.tsx)
              get in this same pass.
                Moving it also fixes a latent mismatch this row had: the
              label + SourceToggle are wrapped in <Advanced>, but the <p>
              explaining what choosing a source DOES sat outside it, so basic
              mode rendered a hint about a control it wasn't showing. As part
              of the label it now appears exactly when the toggle does.
                The old outer `flex flex-col gap-1` wrapper is gone with the
              <p> (one child left); its `py-2 border-b` moves onto this row,
              so the row's own box is unchanged. */}
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
                // Task 5 (rule 13): was a plain underline-on-hover text
                // button; already correctly fault-red per "the destructive
                // control is always the fault colour" (Destructive actions).
                <Badge
                  key={shakeDeleteAll}
                  as="button"
                  onClick={() => void handleDeleteAll()}
                  disabled={deletingAll || loading}
                  tone="fail"
                  size="small"
                  className={`ms-auto${shakeDeleteAll ? " glim-shake" : ""}`}
                >
                  {deletingAll ? t("snapshots.deletingAll") : t("snapshots.deleteAll")}
                </Badge>
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
      style={{ ...hueVars(rainbowAt(index)), "--row-i": String(index) } as CSSProperties}
      // glim-tint washes the card (trap #2 — without it this card shows
      // almost no colour at rest); glim-active while THIS row's own
      // backup/restore is actively running, so reactive mode shows the hue
      // without needing hover — mirrors ContainerRow's identical treatment.
      // bv-stagger-row (GlimStone motion-engine animation 3) — see
      // ContainerRow's identical comment.
      className={`relative overflow-hidden bg-carbon-surface rounded-card p-4 flex flex-col gap-3 glim-hue glim-tint bv-stagger-row ${
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
            {/* No wrapping `<label>`/`<span>` anymore: VMIncludeToggle now
                renders the full ToggleRow itself (label included, text-first),
                the identical shape Containers.tsx's IncludeToggle call site
                already uses — see that component's own comment. */}
            <VMIncludeToggle name={vm.libvirtName} initial={vm.includeInSchedule} />
            {/* Backup method (graceful / live) — always visible; it decides VM downtime. */}
            <label className="flex items-center gap-2">
              <span className="text-xs text-carbon-textSub">{t("vm.method")}</span>
              <VMMethodSelect name={vm.libvirtName} initial={vm.method} t={t} />
            </label>
          </div>
          {/* The corner action pair, laid out exactly like Containers.tsx's
              own BackupButton/ExportButton corner: two 32px badges side by
              side (`items-start gap-1.5 shrink-0`), not one stacked inside
              the other's column. VMExportButton used to be rendered from
              INSIDE VMBackupButton — fine while both were text buttons in a
              vertical stack, wrong once they became square tiles. */}
          <div className="ms-auto flex items-start gap-1.5 shrink-0">
            <VMBackupButton name={vm.libvirtName} t={t} onBackedUp={onRefresh} running={running} />
            {/* Plain export is an advanced-only extra. */}
            <Advanced><VMExportButton name={vm.libvirtName} t={t} /></Advanced>
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
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed action toasts AND shakes its button.
  const [shake, setShake] = useState(0);

  async function handleForget() {
    if (!(await confirm(t("vms.removeEntryConfirm")))) return;
    setPending(true);
    try {
      const res = await forgetVM(name);
      if (res.ok) onForgotten();
      else {
        push(res.error ?? "Remove failed", "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : "Remove failed", "fail");
      setShake((n) => n + 1);
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      {/* NO bespoke red (whole-app sweep) — the exact twin of Containers.tsx's
          DeleteBackupsButton, converted in the same pass and for the same
          reason; see that call site for the full writeup. The label names the
          action, handleForget still routes through the shared useConfirm
          dialog (t("vms.removeEntryConfirm")), and `glim-shake` survives as
          behaviour rather than colour. */}
      <button
        key={shake}
        onClick={() => void handleForget()}
        disabled={pending}
        className={`inline-flex items-center gap-2 rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors disabled:opacity-50${
          shake ? " glim-shake" : ""
        }`}
      >
        {pending ? t("dashboard.checking") : t("vms.removeEntry")}
      </button>
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
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // whichever of the two buttons was actually clicked — a separate nonce per
  // direction rather than one shared one, since only run()'s own `include`
  // argument tells us which.
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
      <button
        key={shakeInclude}
        onClick={() => void run(true)}
        disabled={busy}
        className={`inline-flex items-center rounded-control bg-accent px-3 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
          shakeInclude ? " glim-shake" : ""
        }`}
      >
        {t("schedule.includeAll")}
      </button>
      <button
        key={shakeExclude}
        onClick={() => void run(false)}
        disabled={busy}
        className={`inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50${
          shakeExclude ? " glim-shake" : ""
        }`}
      >
        {t("schedule.excludeAll")}
      </button>
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
function VMBackupOrderPanel({
  vms,
  t,
  hueIndex,
}: {
  vms: VM[];
  t: T;
  /** Rainbow position for THIS panel's own heading notch — GlimStone
   *  follow-up pass (jdp, live review, emphatic, system-wide standing rule
   *  after a fifth escalation: "Warum muss ich dich immer wieder extra dran
   *  erinnern?"): this panel's collapsible-header title was still a plain
   *  `<span>`, never routed through Badge's tone="heading"/hueIndex the way
   *  every other static Card heading in the app now is (Dashboard.tsx's
   *  Card(), Config.tsx's Card, Settings.tsx's Card/ToggleRow) — grepping
   *  this whole file found zero `hueIndex` usages before this fix. Resolved
   *  by the caller's own `nextHue()` counter, called DIRECTLY at the JSX
   *  call site (never handed down as a function for this component to call
   *  from its own body — that exact shape is what caused the SummaryTier
   *  regression a commit ago: React doesn't invoke a child component's body
   *  until after the parent's own render pass has already returned, so a
   *  `nextHue` prop called from inside a child lands strictly after every
   *  sibling's own direct call already consumed its slot). Omit for a
   *  genuine singleton — same rule as every other `hueIndex` call site. */
  hueIndex?: number;
}) {
  const [savedOrder, setSavedOrder] = useState<VmOrder[] | null>(null);
  const [names, setNames] = useState<string[]>([]);
  const [saveState, setSaveState] = useState<"idle" | "saving">("idle");
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // whichever button triggered the failed persist() — Save or Reset — kept as
  // two separate nonces since persist() alone can't tell them apart (both can
  // call it with an empty order in principle).
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
        /* private mode / quota — collapse just won't persist */
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
    // Rainbow-mode completeness sweep (jdp, live review: "Es sind nicht alle
    // Buttons in den Regenbogen-Modus eingepflegt"): `.glim-hue` added below,
    // same fix as Containers.tsx's own identical BackupOrderPanel twin —
    // `glim-notch-card` alone only wires the reactive-mode hover reveal on
    // the heading Badge's own notch, it never redefines --accent/
    // --focus-ring, so the "Save" button further down stayed the flat theme
    // accent regardless of rainbow even after the title notch itself was
    // fixed. Same hueIndex prop the Badge already uses.
    //
    // relative + glim-notch-card: same "half-overlap card notch" pattern
    // every other real Card in this app uses (Config.tsx's Card() is the
    // closest twin — a single div carrying both the visible surface AND the
    // notch's positioned ancestor, no separate outer wrapper needed since
    // this box has no overflow-hidden to clip the badge's own -11px poke
    // above it). glim-notch-card is the hook index.css's card-wide
    // reactive-hover rule keys off, so hovering anywhere on this panel (not
    // just the tiny badge glyph) reveals its hue in reactive rainbow mode.
    <div
      className={`relative glim-notch-card bg-carbon-surface rounded-card p-4 flex flex-col gap-3${
        hueIndex !== undefined ? " glim-hue" : ""
      }`}
      style={hueIndex !== undefined ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined}
    >
      {/* Title notch, always visible regardless of collapse state (matches
          the PRE-fix behaviour, where title+count stayed visible collapsed
          and only the hint hid) — moved OUT of the disclosure <button> below:
          every real tone="heading" call site in this app keeps the Badge as
          its <h2>'s SOLE child (Dashboard.tsx's Card()/SummaryCell,
          Config.tsx's Card, this file's own notInstalledTitle below) because
          size="heading" makes the badge `position: absolute` — a flex-row
          sibling next to it would render at the badge's own now-vacated
          in-flow slot instead of after it. The count folds INSIDE the
          badge's own children instead (Badge's span is `inline-flex gap-1`,
          built to hold more than one child), same visual "title (N)"
          pairing as before, just now inheriting the badge's own solid
          accent-fill/accentContrast ink. */}
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
      {/* Disclosure toggle, now chevron(+hint)-only: the title text that used
          to double as this button's accessible name moved into the h2 notch
          above, so `aria-label` keeps this control genuinely named rather
          than falling back to nothing once its only other content
          (`aria-hidden` chevron, hint text hidden while collapsed) has none
          to offer. `w-full` (unchanged) keeps the full row clickable even
          though the visible content is now just the chevron while collapsed. */}
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
          // Disclosure chevron — same RTL rotation scheme as the identical
          // icon above (form-engine Phase 2 Task 6): closed points start
          // (right in LTR, `rtl:rotate-180` flips it left), open always
          // rotates to straight-down in both directions.
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
                  {/* IconTipButton, not plain <button> + `title` — byte-for-
                      byte the same conversion Containers.tsx's identical
                      reorder pair got in this pass. See that call site. */}
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
              <button
                key={shakeSave}
                onClick={() => void persist(names, "save")}
                disabled={saveState === "saving"}
                className={`inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
                  shakeSave ? " glim-shake" : ""
                }`}
              >
                {t("backupOrder.save")}
              </button>
              <button
                key={shakeReset}
                onClick={clearOrder}
                disabled={saveState === "saving"}
                className={`inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50${
                  shakeReset ? " glim-shake" : ""
                }`}
              >
                {t("backupOrder.reset")}
              </button>
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
  // Broader "something is running" signal: any backup/restore/replication in
  // flight disables the bulk start buttons + shows a hint.
  const running = anyActive(useProgress());
  const [vms, setVMs] = useState<VM[]>([]);
  const [loading, setLoading] = useState(true);
  // Page-level load failure — NOT migrated to a toast (GlimStone follow-up
  // pass, v8.0.0 audit note): matches Containers.tsx's identical page-level
  // `error` — a structural "the page failed" condition, not a one-shot
  // confirmation of a button click.
  const [error, setError] = useState<string | null>(null);
  const [sortKey, setSortKey] = useState<SortKey>(loadSortKey);
  const [search, setSearch] = useState("");
  const [scheduleFilter, setScheduleFilter] = useState<ScheduleFilterKey>(loadScheduleFilterKey);
  const [backupFilter, setBackupFilter] = useState<BackupFilterKey>(loadBackupFilterKey);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [bulkBusy, setBulkBusy] = useState(false);
  const [discovering, setDiscovering] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Discover button on a failed discover, alongside its existing toast.
  const [shakeDiscover, setShakeDiscover] = useState(0);

  // GlimStone follow-up pass (v8.0.0): the "+N" / error note never
  // auto-cleared (stuck next to the Discover button until the next click) —
  // now a toast, mirroring Containers.tsx's identical handleDiscover.
  async function handleDiscover() {
    setDiscovering(true);
    try {
      const res = await discoverVMs();
      if (res.ok) {
        push(`+${res.discovered ?? 0}`, "success");
        await loadVMs();
      } else {
        push(res.error ?? "Discover failed", "fail");
        setShakeDiscover((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : "Discover failed", "fail");
      setShakeDiscover((n) => n + 1);
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

  // GlimStone follow-up pass (v8.0.0): the "N ok, N failed" summary was a
  // persistent inline note (no auto-dismiss); now a one-shot toast, mirroring
  // Containers.tsx's identical runBulk. Severity follows the result.
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
    push(`${ok} ok, ${fail} failed`, fail > 0 ? "warn" : "success");
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

  // hueSeq/nextHue (GlimStone follow-up pass — see Settings.tsx's own
  // identical hueSeq/nextHue comment for the full reasoning): a plain,
  // freshly-reset-every-render counter assigning 0,1,2,... to this page's
  // heading notches in the exact order the JSX below actually evaluates each
  // `hueIndex={nextHue()}` call, which for a `cond && (<Badge hueIndex=
  // {nextHue()} />)` short-circuit is also exactly the order those notches
  // are, or would be, painted. Two heading notches exist on this page today:
  // VMBackupOrderPanel's own (advanced-only, gated on `advanced` directly
  // rather than trusting <Advanced> below — see this component's own
  // `advanced` doc above) and the not-installed section's (gated on
  // `orphans.length > 0`, naturally short-circuited by the `&&` chain around
  // it). Both calls are made DIRECTLY at their JSX call site as a plain
  // number, never handed down as a function for a child to call from its own
  // body later — see SummaryTier's own regression, fixed a commit ago in
  // Dashboard.tsx, for exactly why that shape breaks the ordering.
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    // PAGE_SHELL (jdp live-review, "Können wir die nicht überall gleich breit
    // machen?"): was `gap-6 max-w-5xl`, the same off-standard pair Containers
    // carried — this page is Containers' structural twin and drifted with it.
    // jdp did not name this page (it has no sidebar entry on a host without
    // VMs, so he could not have), which is exactly why it gets swept here in
    // the same pass rather than surfacing as the same complaint a round later.
    // See lib/pageShell.ts for the measurement table behind 1152px/40px.
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
          <button
            key={shakeDiscover}
            onClick={() => void handleDiscover()}
            disabled={discovering}
            title={t("vms.discoverHint")}
            className={`inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
              shakeDiscover ? " glim-shake" : ""
            }`}
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
          run sequence. Above the list, like the container backup-order card.
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
        </div>
      )}

      {/* Live VMs */}
      {!loading && live.length > 0 && (
        <div className="flex flex-col gap-3 bv-content-fade">
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
        <div className="flex flex-col gap-3 bv-content-fade">
          <div>
            {/* GlimStone follow-up pass ("half-overlap card notch"):
                `relative` directly on this <h2> — no padding wraps it, so
                the h2 itself is the right anchor; see Badge.tsx's
                badgeClassName comment and Containers.tsx's identical
                notInstalled section.
                `hueIndex={nextHue()}` (GlimStone follow-up pass, proactive
                sweep of this same file): this badge used to be the ONLY
                tone="heading" notch anywhere in VMs.tsx, so it always read
                as a "genuine singleton" and correctly kept the flat,
                un-rainbowed default — but VMBackupOrderPanel's own notch
                above can render on the very same page now, which makes this
                one no longer a singleton whenever both are visible at once
                (Advanced mode on + at least one orphaned VM). Threaded
                through the same page-wide `nextHue()` counter, in render
                order after VMBackupOrderPanel's own call, so the two never
                collide on the same rainbow position. */}
            <h2 className="relative flex items-center">
              <Badge tone="heading" size="heading" wrap hueIndex={nextHue()}>
                {t("containers.notInstalledTitle")}
              </Badge>
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
