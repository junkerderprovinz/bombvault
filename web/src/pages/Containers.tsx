import { useEffect, useRef, useState, type CSSProperties } from "react";
import { listContainers, deleteBackups, backupAll, restore, restoreStack, discover, setContainerHooks, getContainerMounts, setBackupPaths, setStopContainers, setContainerExcludes, previewContainerExcludes, suggestContainerExcludes, exportContainer, setIncludeAll, setUpdateAfterBackup, getBackupOrder, setBackupOrder, ApiError } from "../lib/api";
import type { Container, ExcludeSuggestion, MountInfo, CustomPath, ContainerOrder } from "../lib/api";
import { FolderBrowser } from "../components/FolderBrowser";
import { humanBytes } from "../lib/forecast";
import { FilterPopover } from "../components/FilterPopover";
import { IconTipButton } from "../components/IconTipButton";
import { DropdownListbox } from "../components/DropdownListbox";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { useT, stateLabel, type TranslationKey } from "../lib/i18n";
import { InfoBubble } from "../components/InfoBubble";
import { PAGE_SHELL } from "../lib/pageShell";
import { Advanced, useAdvanced } from "../lib/advanced";
import { BackupButton } from "../components/BackupButton";
import { fireAndWaitRun } from "../lib/backupWatch";
import { RestorePanel } from "../components/RestorePanel";
import { RestoreCancelButton } from "../components/RestoreCancelButton";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconContainers, IconDownload, IconAdd } from "../components/Sidebar";
import { IncludeToggle } from "../components/IncludeToggle";
import { Badge, type BadgeTone } from "../components/Badge";
import { Button } from "../components/Button";
import { groupStage } from "../lib/controls";
import { ToggleRow } from "./settings/shared";
import { ProgressBar } from "../components/ProgressBar";
import { tLtr, withLtrFragments, withLtrPlaceholder, EXCLUDES_HINT_LTR_FRAGMENTS } from "../lib/ltrFragments";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { relativeTime } from "../lib/reltime";
import { useDragReorder } from "../lib/useDragReorder";
import { useConfirm } from "../lib/useConfirm";
import { hueVars, rainbowAt } from "../lib/appearance";
import { useRainbow } from "../lib/useRainbow";
import { Selector, type SelectorItem } from "../components/Selector";
import { useToast } from "../lib/toast";
import { IconSearch } from "../components/glyphs";

import { Toggle } from "../components/Toggle";
type T = ReturnType<typeof useT>["t"];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

// formatSnapshotWhen renders a snapshot's RFC3339 timestamp for the exclusion
// assistant's source line. The date is load-bearing there: it is what turns
// "sizes as of the last backup" from a misleading number into an honest one.
// An unparseable value falls back to the raw string rather than "Invalid Date".
function formatSnapshotWhen(rfc3339: string): string {
  const d = new Date(rfc3339);
  return Number.isNaN(d.getTime()) ? rfc3339 : d.toLocaleString();
}

// SNAPSHOT_STALE_MS — past this age the assistant says out loud that its list
// describes the past. The date alone is not enough: a user reading "sizes come
// from the backup of <date>" still has to notice the date is not today, and the
// case that matters (a cache that exploded since the last backup, a junk folder
// created yesterday) is invisible in a snapshot list — no row, no warning. One
// day is the threshold because the schedule most installs run is nightly, so
// anything older than that means the backup the panel is quoting is not even
// the one the user thinks they made last night.
const SNAPSHOT_STALE_MS = 24 * 60 * 60 * 1000;

// snapshotIsStale reports whether a snapshot timestamp is old enough to warrant
// the "check the folders as they are now" nudge. An unparseable value is never
// stale — it would be a warning about a date nobody can read.
function snapshotIsStale(rfc3339: string): boolean {
  const d = new Date(rfc3339);
  if (Number.isNaN(d.getTime())) return false;
  return Date.now() - d.getTime() > SNAPSHOT_STALE_MS;
}

// liveSourceKey picks the sentence that says WHY the sizes came from a folder
// scan. One string used to serve all three cases and claimed "this container has
// no backup yet" on every one of them, including the folder scan offered after
// an index read failed — a container that demonstrably HAS a backup, which is
// the only reason its index was read at all.
function liveSourceKey(reason: "no-snapshot" | "requested" | "not-in-snapshot"): TranslationKey {
  if (reason === "requested") return "excludes.assistSourceLiveRequested";
  if (reason === "not-in-snapshot") return "excludes.assistSourceLiveNotInSnapshot";
  return "excludes.assistSourceLive";
}

// ---------------------------------------------------------------------------
// State chip — stateTone maps a raw container state to the shared Badge's
// tone; stateLabel (lib/i18n) still does the actual state->text translation.
// ---------------------------------------------------------------------------

function stateTone(state: string): BadgeTone {
  const lower = state.toLowerCase();
  if (lower === "running") return "ok";
  if (lower === "exited" || lower === "stopped") return "fail";
  return "neutral";
}

// ---------------------------------------------------------------------------
// Sort control
// ---------------------------------------------------------------------------

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
// onto the shared Selector component (GlimStone form-engine Phase 2, Task 3):
// each maps this page's own domain data (a sort key, a filter key, a
// generic option list) onto Selector's generic items/active/onChange shape.
// The actual button rendering, keyboard nav (roving tabindex, arrow keys/
// Home/End, RTL) and rainbow hueing all now live in Selector itself, not
// copy-pasted here — that duplicated rendering (with zero keyboard support)
// was exactly what had drifted apart between this file and VMs.tsx's own
// near-identical copies.
//
// All three render the SMALL horizontal selector (`variant="well"`, no
// `equalWidth`) — jdp, live review: "Im Filtermenü: die Optionen bitte in
// kleine horizontale Selektoren einpflegen". They used to be Selector's
// default `chip` variant, where every idle option carries its own filled
// `bg-carbon-surface2` pill, so a three-option filter read as three competing
// buttons rather than one control with one choice made. The "well" track puts
// them in a single grooved strip with transparent idle segments, which is the
// same small selector Settings' notify/integrity rows and CadenceBuilder
// already use — and the small one, not the pinned `equalWidth size="lg"` one,
// because these sit inside FilterPopover's 256-416px panel where a 200px
// per-segment floor would immediately wrap every option onto its own line.
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
        variant="well"
        select="one"
        active={value}
        onChange={(id) => onChange(id as SortKey)}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Installed / not-installed filter
// ---------------------------------------------------------------------------

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
        variant="well"
        select="one"
        active={value}
        onChange={(id) => onChange(id as FilterKey)}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Schedule / backup chip filters (#41)
// ---------------------------------------------------------------------------
// Generic sibling of FilterControl: same chip look + localStorage pattern, but
// parameterised over its option set so the schedule and backup dimensions can
// each instantiate it without duplicating the markup.

type ScheduleFilterKey = "all" | "scheduled" | "notScheduled";
type BackupFilterKey = "all" | "backedUp" | "neverBackedUp";

const SCHEDULE_FILTER_STORAGE_KEY = "bv-containers-schedule-filter";
const BACKUP_FILTER_STORAGE_KEY = "bv-containers-backup-filter";

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

// ---------------------------------------------------------------------------
// Container row
// ---------------------------------------------------------------------------

// DeleteBackupsButton permanently forgets all backups of a (usually
// no-longer-installed) container and refreshes the list on success.
function DeleteBackupsButton({
  name,
  t,
  onDeleted,
}: {
  name: string;
  t: T;
  onDeleted: () => void;
}) {
  const [pending, setPending] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed action toasts AND shakes its button.
  const [shake, setShake] = useState(0);

  async function handleDelete() {
    // TODO(#follow-up): once richer stake-detail copy ("N snapshots, X GB")
    // ships (deferred — needs new interpolated i18n keys across all 25
    // non-English locales, out of scope for the window.confirm() → dialog
    // mechanism swap, form-engine Task 7), it renders here as extra body
    // content passed to confirm(), same as the two other flagged sites in
    // VMs.tsx's deleteAllConfirm and Files.tsx's deleteBackupsConfirm.
    if (!(await confirm(t("containers.deleteBackupsConfirm")))) return;
    setPending(true);
    try {
      const res = await deleteBackups(name);
      if (res.ok) onDeleted();
      else {
        push(res.error ?? t("common.deleteFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.deleteFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="flex flex-col gap-1">
      {/* NO bespoke red (whole-app sweep). This was `bg-statusFailBg`/
          `text-statusFail`, the last of that treatment on this page. The
          standing rule is explicit — status colours stay OUT of the accent
          engine AND a destructive action gets no special red of its own
          either (jdp: "Keine Sonderfarbe fuer den Entfernen-Badge", and
          RestorePanel's own delete badge records the same reversal: "Der
          Löschen-Badge ist auch anders eingefärbt, soll nicht so sein").
          Plain neutral secondary chrome now, identical to every other
          secondary text button in the app.
            Nothing about the action becomes ambiguous: the label says
          "Alle Backups löschen" verbatim, and handleDelete still routes
          through the shared useConfirm dialog
          (t("containers.deleteBackupsConfirm")) before anything is deleted —
          that confirmation is untouched. `glim-shake` on failure survives:
          behaviour, not colour. Stays a TEXT button (not an icon badge) —
          it is a labelled action that also carries an in-flight label, not
          a row-action glyph pair. */}
      <Button
        key={shake}
        label={t("containers.deleteBackups")}
        labelKey="containers.deleteBackups"
        tone="neutral"
        onClick={() => void handleDelete()}
        disabled={pending}
        busy={pending}
        title={pending ? t("dashboard.checking") : undefined}
        className={shake ? "glim-shake" : ""}
      />
      {confirmDialog}
    </div>
  );
}

// ExportButton writes a plain, tool-free tar+xml copy of the container (the same
// folders restic backs up, plus the Unraid template) into a browsable folder next
// to the repo — restic stays the engine; this is an extra, unencrypted export.
//
// GlimStone follow-up pass (v8.0.0) audit note: the "done"/"error" result below
// is DELIBERATELY left as inline status, not migrated to a toast — it shows the
// actual destination PATH the export landed at (or, on failure, the raw error),
// neither of which auto-dismissed even before this pass. Like
// SettingsPortabilityCard's export/import banners, this is a reference value the
// user needs to actually copy down or read, not a one-shot "it worked" ping a 4s
// toast would cut off mid-read. STILL TRUE after the Task 2 icon-badge
// conversion below — only the TRIGGER became a square glyph badge (jdp:
// "Export soll ein quadratischer Badge mit Glyph sein... rechts oben in der
// Ecke"); this sticky result column renders in the exact same place right
// below it, unchanged, so the copyable path is still there to read once the
// export finishes.
function ExportButton({ name, t }: { name: string; t: T }) {
  const [state, setState] = useState<"idle" | "pending" | "done" | "error">("idle");
  const [msg, setMsg] = useState<string | null>(null);
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed action toasts AND shakes its button, layered ON TOP of this
  // button's own pre-existing sticky inline error (kept deliberately — see
  // this component's header comment: the error is a reference value the user
  // may need to read/copy, not a one-shot ping the toast alone would replace).
  const [shake, setShake] = useState(0);

  async function run() {
    setState("pending");
    setMsg(null);
    try {
      const r = await exportContainer(name);
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
      {/* `size="icon"` — the app's one square-icon-badge size (32px); see
          Badge.tsx's "ONE SIZE FOR SQUARE ICON BADGES" block.
            `tone="active"`, NOT the `tone="neutral"` this badge shipped with:
          neutral resolves to a flat `bg-carbon-surface2` grey that takes no
          colour-engine position at all, which left Export as the single grey
          tile in a Container card whose every other badge (Jetzt sichern,
          Lokal/Offsite, Wiederherstellen, Löschen) is hue-integrated — the
          same "anders eingefärbt" defect jdp reported one badge over, on
          RestorePanel's delete. The standing icon-badge rule is that a square
          icon badge gets colour-engine integration and a tooltip
          automatically; neutral here was an unexamined default, not a
          decision. `active` + icon-only resolves to the solid `bg-accent`/
          `text-accentContrast` pair (Badge's own `isIconOnly && tone==="active"`
          branch) and inherits this row's own rainbow position from the
          ambient `.glim-hue` cascade, exactly like BackupButton beside it —
          no `hueIndex` needed. */}
      <Button
        key={shake}
        label={t("export.button")}
        labelKey="export.button"
        glyph={<IconDownload />}
        tone="accent"
        // Same shared stage as BackupButton beside it — see that file.
        stage={groupStage([t("containers.backupNow"), t("export.button")])}
        onClick={() => void run()}
        disabled={state === "pending"}
        busy={state === "pending"}
        className={shake ? "glim-shake" : ""}
      />
      {state === "done" && msg && (
        <span className="text-xs text-statusOk break-all text-end max-w-[18rem]">
          {t("export.exportedTo")} <span dir="ltr" className="text-start">{msg}</span>
        </span>
      )}
      {state === "error" && msg && (
        <span dir="ltr" className="text-xs text-statusFail wrap-break-word text-start max-w-[18rem]">{msg}</span>
      )}
    </div>
  );
}

// useDebouncedSave — a small per-instance "call `run` DELAY_MS after the last
// invocation, cancel the pending one if called again first" hook: the exact
// same shape/delay as Settings.tsx's own page-level debouncedSave/DEBOUNCE_MS
// (which every remaining free-text field in this app — registry host/user/
// token, the age-recipients list, cron/schedule strings — already auto-saves
// through), just scoped to ONE component instance instead of a shared
// page-level timer map keyed by field name. HooksEditor/ExcludesEditor below
// are each their own instance (one per container row), so unlike
// SettingsPage — which can have several unrelated debounced fields live at
// once and needs a string key to keep their timers apart — each of these only
// ever has ONE outstanding timer of its own, so no key is needed here.
const AUTOSAVE_DEBOUNCE_MS = 800;

function useDebouncedSave(delayMs: number = AUTOSAVE_DEBOUNCE_MS) {
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // A pending debounce must not fire after this component has unmounted (the
  // user edits a field then closes/navigates away within the delay window) —
  // same "capture the ref, clear on unmount" guard Settings.tsx's own
  // debounce-cleanup effect uses.
  useEffect(() => {
    return () => {
      if (timer.current) clearTimeout(timer.current);
    };
  }, []);
  function debouncedSave(run: () => void) {
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(run, delayMs);
  }
  // cancel() is what lets an IMMEDIATE save win over a pending debounced one.
  // Without it the two paths raced and the timer won by construction, because
  // it fires later: clicking a suggestion chip within 800ms of a keystroke saved
  // the chip's list, then the timer wrote the PRE-CHIP list back over it — and
  // both paths toasted "Saved". A caller that saves right now must retire the
  // timer first; every such caller already computes the full next list, so
  // nothing typed is lost by dropping it.
  function cancel() {
    if (timer.current) {
      clearTimeout(timer.current);
      timer.current = null;
    }
  }
  return { debouncedSave, cancel };
}

// HooksEditor edits the per-container pre/post-backup commands (collapsible).
// `open` is now controlled by the caller (Containers.tsx's ContainerRow, via
// its own shared five-chip Selector strip — see that call site's own comment)
// rather than an internal useState: this component no longer renders its own
// trigger button, only the content pane, shown or hidden by the prop.
//
// Live-save conversion (jdp, live review: "Brauchen wir die Speichern-Buttons
// in den Aufklappcards überhaupt? Es soll doch immer live speichern."): the
// explicit Save button is GONE — both fields now debounce-auto-save via
// useDebouncedSave above, 800ms after the last keystroke, combined into ONE
// setContainerHooks(pre, post) call (the same "compute the next value
// locally, pass it straight into the debounced closure" shape Settings.tsx's
// own registryAuths row edits use for their own multi-field-into-one-PATCH
// save). No revert-on-failure and no `.glim-shake` here — matching
// Settings.tsx's OWN debouncedSave text-field convention exactly (see e.g.
// pathSaveState's "only the setters are needed" comment): a shell command is
// free text like a cron string or a registry token, already saved this exact
// way elsewhere in this app with zero exception, and reverting a field the
// user might still be actively typing into would be jarring rather than
// helpful — the toast alone reports a failure, and the value simply stays as
// typed for the next edit (or a reload) to pick up. Discrete boolean/
// selection saves (FoldersEditor's mount checkboxes, StopContainersEditor's
// picker rows) are the other half of this conversion and DO keep revert +
// shake, since those really are one-click toggles, not continuous typing —
// see FoldersEditor's own `toggle()` comment for that half's reasoning.
function HooksEditor({
  name,
  initialPre,
  initialPost,
  open,
  t,
}: {
  name: string;
  initialPre: string;
  initialPost: string;
  open: boolean;
  t: T;
}) {
  const [pre, setPre] = useState(initialPre);
  const [post, setPost] = useState(initialPost);
  const { push } = useToast();
  const { debouncedSave } = useDebouncedSave();

  async function saveHooks(nextPre: string, nextPost: string) {
    try {
      const r = await setContainerHooks(name, nextPre, nextPost);
      if (r.ok) push(t("settings.saved"), "success");
      else push(r.error ?? t("settings.error"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-2 py-1 bv-field-focus";

  if (!open) return null;

  return (
    <div className="mt-2 rounded-card bg-carbon-background p-3 flex flex-col gap-2">
      <p className="text-xs text-carbon-textMuted">{t("hooks.hint")}</p>
      <label className="flex flex-col gap-1">
        <span className="text-xs text-carbon-textSub">{t("hooks.pre")}</span>
        <input value={pre} onChange={(e) => {
          const nextPre = e.target.value;
          setPre(nextPre);
          debouncedSave(() => void saveHooks(nextPre, post));
        }} spellCheck={false}
          placeholder="mysqldump -uroot -p$PW db > /config/dump.sql" className={inputCls} />
      </label>
      <label className="flex flex-col gap-1">
        <span className="text-xs text-carbon-textSub">{t("hooks.post")}</span>
        <input value={post} onChange={(e) => {
          const nextPost = e.target.value;
          setPost(nextPost);
          debouncedSave(() => void saveHooks(pre, nextPost));
        }} spellCheck={false}
          placeholder="curl -fsS https://hooks.example/done" className={inputCls} />
      </label>
    </div>
  );
}

// FoldersEditor lets the user choose which of a container's mapped folders get
// backed up (appdata is the default), plus add custom paths under the host
// mount. Collapsible; loads the mount list lazily on first open.
// UpdateAfterBackupRow toggles the per-container "update after successful backup"
// opt-in (#52): after a backup, BombVault pulls the image and recreates the
// container only when a newer image is available (the fresh backup is the safety
// net). Off by default; advanced-only.
//
// Containers.tsx Task 5 (jdp, live-review): the explanation used to sit as a
// permanently-visible caption under the label — moved into a real "(i)"
// InfoBubble instead, via the SAME shared `ToggleRow` Settings.tsx/Config.tsx/
// Recovery.tsx already use for every other "label + hint bubble + flush-right
// switch" row in this app (its own `hint` prop wires an InfoBubble internally
// — see ToggleRow's own doc), rather than hand-rolling a second, bespoke
// bubble treatment here. This row also grew the post-backup update-check
// RESULT line (moved here from the top-right corner, where it used to sit
// under "Letztes Backup" — see ContainerRow's own comment on that move): it
// is live status data about THIS toggle's own last run, not static
// explanatory text, so it stays visible prose, not a second bubble.
function UpdateAfterBackupRow({
  name,
  initial,
  lastUpdateCheck,
  lastUpdateResult,
  t,
}: {
  name: string;
  initial: boolean;
  lastUpdateCheck: number;
  lastUpdateResult: string;
  t: T;
}) {
  const [enabled, setEnabled] = useState(initial);
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed toggle toasts AND shakes, same mechanism as ToggleRow's shakeNonce.
  const [shake, setShake] = useState(0);
  useEffect(() => setEnabled(initial), [initial]);

  async function handle(next: boolean) {
    setBusy(true);
    try {
      const res = await setUpdateAfterBackup(name, next);
      if (res.ok) setEnabled(next);
      else {
        push(res.error ?? t("containers.updateSettingFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("containers.updateSettingFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <ToggleRow
        label={t("update.afterBackup")}
        hint={t("update.afterBackupHint")}
        checked={enabled}
        onChange={(next) => void handle(next)}
        disabled={busy}
        shakeNonce={shake}
      />
      {/* Post-backup update-check signal (G4): only meaningful when the
          opt-in is on and a check has actually completed. An up-to-date
          check records no run, so this line is its only surface. */}
      {enabled && lastUpdateCheck > 0 && (
        <p className="text-xs text-carbon-textMuted text-end">
          {t("containers.updateCheckLabel")}: {relativeTime(t, lastUpdateCheck)}, {updateCheckResultText(t, lastUpdateResult)}
        </p>
      )}
    </div>
  );
}

// `open` is controlled by the caller (ContainerRow's shared five-chip
// Selector strip) — see HooksEditor's own comment for the full "why" this and
// its three siblings dropped their own internal useState.
function FoldersEditor({ name, stack, open, t }: { name: string; stack: string; open: boolean; t: T }) {
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(false);
  const [mounts, setMounts] = useState<MountInfo[]>([]);
  const [checked, setChecked] = useState<Set<string>>(new Set());
  const [custom, setCustom] = useState<CustomPath[]>([]);
  // The folder picker works in paths relative to the host mount (like File Sets);
  // browseValue stages one pick before it is translated to a host path and added.
  const [browseValue, setBrowseValue] = useState("");
  const [hostMountRoot, setHostMountRoot] = useState("/host/user");
  const [hostSourceRoot, setHostSourceRoot] = useState("/mnt");
  const { push } = useToast();
  // Live-save conversion (jdp, live review — see HooksEditor's own header
  // comment for the full "why" across all four editors): a mount checkbox is
  // a discrete boolean pick, the SAME shape SettingsPage's toggleDomainEnabled
  // already established for "flip one boolean, persist immediately, revert +
  // `.glim-shake` on failure" — checkRowBusy/checkRowShake below are that
  // same per-key busy/shake map, just keyed by mount `source` instead of a
  // domain name. Adding/removing a CUSTOM path is a structural list edit
  // instead (closer to Settings.tsx's registryAuths row add/remove), so it
  // saves immediately too but withOUT revert/shake — see addCustom/
  // removeCustomPath's own comments below.
  const [checkRowBusy, setCheckRowBusy] = useState<Record<string, boolean>>({});
  const [checkRowShake, setCheckRowShake] = useState<Record<string, number>>({});

  useEffect(() => {
    if (!open || loaded) return;
    setLoading(true);
    getContainerMounts(name)
      .then((r) => {
        if (r.ok) {
          const ms = r.mounts ?? [];
          setMounts(ms);
          setChecked(new Set(ms.filter((m) => m.selected && m.reachable).map((m) => m.source)));
          setCustom(r.custom ?? []);
          if (r.hostMountRoot) setHostMountRoot(r.hostMountRoot);
          if (r.hostSourceRoot) setHostSourceRoot(r.hostSourceRoot);
        } else {
          push(r.error ?? t("settings.error"), "fail");
        }
      })
      .catch((err) => {
        push(err instanceof Error ? err.message : t("settings.error"), "fail");
      })
      .finally(() => {
        setLoading(false);
        setLoaded(true);
      });
  }, [open, loaded, name, t, push]);

  // persistPaths is the single call every mutation below funnels through —
  // toggling a mount checkbox or adding/removing a custom path all resolve to
  // the same setBackupPaths(name, fullList) PATCH, just with different
  // revert/shake behaviour on failure (see toggle/addCustom/removeCustomPath's
  // own comments for which half of the live-save conversion each belongs to).
  async function persistPaths(paths: string[]): Promise<boolean> {
    try {
      const r = await setBackupPaths(name, paths);
      if (r.ok) {
        push(t("folders.saved"), "success");
        return true;
      }
      push(r.error ?? t("settings.error"), "fail");
      return false;
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      return false;
    }
  }

  // Discrete boolean toggle — optimistic flip, immediate save, revert +
  // `.glim-shake` (keyed by mount `source`) on failure. Same shape as
  // SettingsPage's toggleDomainEnabled; see this component's own top-level
  // comment for the full "why this half of the conversion reverts and the
  // other half doesn't" reasoning.
  async function toggle(source: string) {
    const wasChecked = checked.has(source);
    const next = new Set(checked);
    if (wasChecked) next.delete(source);
    else next.add(source);
    setChecked(next);
    setCheckRowBusy((b) => ({ ...b, [source]: true }));
    const ok = await persistPaths([...next, ...custom.map((c) => c.path)]);
    setCheckRowBusy((b) => ({ ...b, [source]: false }));
    if (!ok) {
      setChecked((prev) => {
        const reverted = new Set(prev);
        if (wasChecked) reverted.add(source);
        else reverted.delete(source);
        return reverted;
      });
      setCheckRowShake((s) => ({ ...s, [source]: (s[source] ?? 0) + 1 }));
    }
  }

  // Structural list add — immediate save, no revert/shake (same shape as
  // Settings.tsx's registryAuths row add/remove: the path stays in the list
  // either way, a failed save just gets picked up by the next edit or a
  // reload — see debouncedSave's own "no revert" comment for the identical
  // reasoning applied to a structural edit instead of a text edit).
  function addCustom() {
    const raw = browseValue.trim();
    if (!raw) return;
    // The folder picker yields a path relative to the host mount; translate it to
    // the host path SetBackupPaths expects. An already-absolute path (manual
    // fallback) is used as-is.
    const p = raw.startsWith("/") ? raw : `${hostSourceRoot}/${raw}`;
    setBrowseValue("");
    if (custom.some((c) => c.path === p)) return;
    const nextCustom = [...custom, { path: p, exists: true }];
    setCustom(nextCustom);
    void persistPaths([...checked, ...nextCustom.map((c) => c.path)]);
  }

  // Structural list remove — same immediate-save-no-revert shape as addCustom
  // above.
  function removeCustomPath(path: string) {
    const nextCustom = custom.filter((x) => x.path !== path);
    setCustom(nextCustom);
    void persistPaths([...checked, ...nextCustom.map((c) => c.path)]);
  }

  if (!open) return null;

  return (
    <div className="mt-2 rounded-card bg-carbon-background p-3 flex flex-col gap-2">
      <p className="text-xs text-carbon-textMuted">{t("folders.hint")}</p>
      {/* A compose member's project folder is NOT in this list, and its absence
          would otherwise read as "nothing to back up". It is backed up once for
          the whole stack instead of once per service (issue #189), so it needs
          saying exactly where someone would look for it and not find it. */}
      {stack !== "" && (
        <p className="text-xs text-carbon-textSub">
          {t("folders.stackNote").replace("{stack}", stack)}
        </p>
      )}
      {loading && <p className="text-xs text-carbon-textMuted">{t("common.loadingBackups")}</p>}
      {!loading && mounts.length === 0 && custom.length === 0 && (
        <p className="text-xs text-carbon-textMuted">{t("folders.empty")}</p>
      )}
      {mounts.map((m) => (
        <label
          // Keyed by source PLUS its own shake nonce (not just source) — a
          // genuinely new key remounts this one row so `.glim-shake` replays
          // on a rejected save, the same "key = nonce" technique this app's
          // shake-capable controls already use (ToggleRow's own
          // `shakeNonce`-keyed Toggle is the precedent).
          key={`${m.source}-${checkRowShake[m.source] ?? 0}`}
          className={`flex items-start gap-2 text-xs ${m.reachable ? "text-carbon-text" : "text-carbon-textMuted"}${checkRowShake[m.source] ? " glim-shake" : ""}`}
        >
          <input
            type="checkbox"
            disabled={!m.reachable || !!checkRowBusy[m.source]}
            checked={m.reachable && checked.has(m.source)}
            onChange={() => void toggle(m.source)}
            className="mt-0.5 accent-(--accent)"
          />
          <span className="flex flex-col">
            <span dir="ltr" className="font-mono break-all text-start">{m.dest} ← {m.source}</span>
            {m.isAppdata && <span className="text-statusOk">{t("folders.appdataDefault")}</span>}
            {!m.reachable && <span className="text-statusFail">{t("folders.notReachable")}</span>}
          </span>
        </label>
      ))}
      {custom.map((cp) => (
        <div key={cp.path} className="flex items-start gap-2 text-xs text-carbon-text">
          <input type="checkbox" checked readOnly className="mt-0.5 accent-(--accent)" />
          <span className="flex flex-col flex-1 min-w-0">
            <span dir="ltr" className="font-mono break-all text-start">{cp.path}</span>
            {!cp.exists && <span className="text-statusFail">{t("folders.customMissing")}</span>}
          </span>
          {/* NO bespoke red hover (whole-app sweep): this carried
              `hover:text-statusFail`, the same "a destructive control paints
              itself red" treatment removed everywhere else in this pass. It
              is a bare `×` glyph with no resting fill, so it is NOT a square
              icon badge and does not take the 32px badge treatment (see
              Badge.tsx's "ONE SIZE FOR SQUARE ICON BADGES" block, which
              carves out exactly this shape of affordance alongside the
              backup-order reorder arrows).
                `aria-label` was the hard-coded, untranslated English string
              "remove" — the only one left in web/src, and invisible to the
              i18n parity test because it never went through t(). Now
              t("offsite.targets.remove"), an existing key already translated
              in all 42 locales, so this adds no new key. */}
          <Button
            label={t("offsite.targets.remove")}
            labelKey="offsite.targets.remove"
            variant="chip"
            onClick={() => removeCustomPath(cp.path)}
          />
        </div>
      ))}
      <div className="flex items-end gap-2 pt-1">
        <div className="flex-1 min-w-0">
          <FolderBrowser
            label={t("folders.addCustom")}
            value={browseValue}
            hostMountRoot={hostMountRoot}
            onChange={setBrowseValue}
          />
        </div>
        {/* Square icon badge (icon-badge round, standing rule: every icon
            badge gets real hue integration + a hover tooltip carrying its
            old label). Colour-engine integration is the SAME already-
            verified mechanism the prior text-button version of this control
            used (Task 3, jdp live-review): no `hueIndex` needed — this
            panel already lives inside ContainerRow's own `.glim-hue`
            element, so Badge's `tone="active"` (icon-only → solid
            `bg-accent`/`text-accentContrast`, see Badge.tsx's own
            `isIconOnly && tone==="active"` branch) resolves to the row's
            own rainbow position via the ordinary CSS custom-property
            cascade, verified live via getComputedStyle against the real
            deployed container.
              `size="icon"` — the app's one square-icon-badge size (32px). The
            old `size="compact"` stage was 32px too, so this badge's rendered
            box is unchanged; only the token name moved, because `compact`
            existed solely to hold one arm of the role-based 28/32/36px split
            that jdp rejected (see Badge.tsx's "ONE SIZE FOR SQUARE ICON
            BADGES" block). The 32px value is still exactly right here for the
            reason it always was — this badge shares an `items-end` row with a
            FolderBrowser field that measures 32px live (`text-sm px-3 py-1.5`)
            — it is simply no longer a number this call site owns. `tip`
            carries the exact text this button showed before becoming
            icon-only. */}
        <Button
          label={t("folders.add")}
          labelKey="folders.add"
          glyph={<IconAdd />}
          tone="accent"
          onClick={addCustom}
        />
      </div>
    </div>
  );
}

// StopContainersEditor edits the list of OTHER containers to stop during this
// container's backup (e.g. a database). Collapsible; `open` is controlled by
// the caller — see HooksEditor's own comment.
//
// REWORKED (jdp, live review: "Können wir da nicht eine Dropdownliste aller
// installierten Container machen? Also dass es automatisch alle installierten
// Container auflistet, die man dann auswählen kann."): this used to be a
// free-text `<textarea>`, one hand-typed container name per line — no
// validation against what is actually installed, a typo just silently never
// matched anything at backup time. Replaced with a proper multi-select: a
// dropdown/listbox populated from `installedContainers`, the SAME container
// list ContainerRow's own caller (Containers()) already fetches to render
// every row on this page — no second API call. Custom listbox, not a native
// `<select multiple>` (illegible checkbox-free multi-select UI, no per-row
// icon/status room) — same "escape hatch" precedent as Settings.tsx's
// LanguageCard dropdown (role="listbox", outside-click/Escape-to-close), here
// extended to `aria-multiselectable="true"` with a real checkbox per row
// instead of LanguageCard's single-select radio-like rows.
function StopContainersEditor({
  name,
  initial,
  installedContainers,
  open,
  t,
}: {
  name: string;
  initial: string[];
  /** Every INSTALLED container on this BombVault instance, as already
   *  fetched once by Containers() for rendering the row list — threaded
   *  through ContainerRow rather than a second `listContainers()` call here. */
  installedContainers: Container[];
  open: boolean;
  t: T;
}) {
  const [selected, setSelected] = useState<Set<string>>(() => new Set(initial));
  const [pickerOpen, setPickerOpen] = useState(false);
  // The BUTTON itself, not its wrapper: DropdownListbox sizes the portalled
  // panel to whatever this ref measures, and the wrapper below is a flex
  // ITEM of this editor's `flex flex-col` box — `inline-block` gets
  // blockified and stretched to the card's full content width there, so
  // measuring the wrapper handed the panel a ~970px width instead of the
  // button's own 256px (caught by measuring it live, not by reading the
  // markup). It also keeps the outside-click exemption tight: only the
  // button is exempt, which is all that needs to be.
  const pickerRef = useRef<HTMLButtonElement>(null);
  const { push } = useToast();
  // Live-save conversion (jdp, live review — see HooksEditor's own header
  // comment for the full "why" across all four editors): each listbox row is
  // a discrete boolean pick (same shape as FoldersEditor's mount checkboxes),
  // so rowBusy/rowShake below are that identical per-key busy/shake map, keyed
  // by candidate container name instead of mount source.
  const [rowBusy, setRowBusy] = useState<Record<string, boolean>>({});
  const [rowShake, setRowShake] = useState<Record<string, number>>({});

  // Re-seed whenever the SAVED value changes underneath this editor (e.g. a
  // fresh `listContainers()` reload after Discover) — the same "derived from
  // props but independently editable until the next save" shape
  // UpdateAfterBackupRow's own `initial`-seeded toggle already uses.
  //
  // Keyed on the CONTENT, not the array's identity. The call site passes
  // `container.stopContainers ?? []`, and the API returns null for any
  // container that has no target row yet, so that fallback minted a BRAND NEW
  // array on every parent render — and this effect then reset the selection to
  // empty each time. What made it costly rather than merely annoying: the next
  // toggle saves the visible set, and the server replaces the stored list
  // wholesale, so a user who ticked three containers and then triggered any
  // parent re-render silently saved a list with only the fourth in it.
  const initialKey = JSON.stringify(initial);
  useEffect(() => {
    setSelected(new Set(JSON.parse(initialKey) as string[]));
  }, [initialKey]);

  // Candidates: every OTHER installed container — excludes this row's own
  // container (a container can't stop itself) and BombVault's own container
  // (the established "BombVault's own container never appears in
  // schedule-member lists" rule, Settings.tsx's ContainersSection).
  const candidates = installedContainers
    .filter((c) => c.name !== name && !c.self)
    .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: "base" }));

  // A previously-saved name that no longer matches an installed container
  // (uninstalled since, renamed, or a leftover from the old free-text field)
  // still needs to stay visible and removable — silently dropping it on the
  // next save would be data loss the user never asked for. Marked inline
  // with the existing `containers.notInstalled` badge text rather than a
  // second bespoke "stale" label.
  const candidateNames = new Set(candidates.map((c) => c.name));

  // Discrete boolean toggle — optimistic flip, immediate save, revert +
  // `.glim-shake` (keyed by container name) on failure. Same shape as
  // SettingsPage's toggleDomainEnabled/FoldersEditor's own mount-checkbox
  // `toggle` — see this component's own top-level comment.
  async function toggle(n: string) {
    const wasSelected = selected.has(n);
    const next = new Set(selected);
    if (wasSelected) next.delete(n);
    else next.add(n);
    setSelected(next);
    setRowBusy((b) => ({ ...b, [n]: true }));
    try {
      const r = await setStopContainers(name, [...next]);
      if (r.ok) {
        push(t("settings.saved"), "success");
      } else {
        push(r.error ?? t("settings.error"), "fail");
        setSelected((prev) => {
          const reverted = new Set(prev);
          if (wasSelected) reverted.add(n);
          else reverted.delete(n);
          return reverted;
        });
        setRowShake((s) => ({ ...s, [n]: (s[n] ?? 0) + 1 }));
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      setSelected((prev) => {
        const reverted = new Set(prev);
        if (wasSelected) reverted.add(n);
        else reverted.delete(n);
        return reverted;
      });
      setRowShake((s) => ({ ...s, [n]: (s[n] ?? 0) + 1 }));
    } finally {
      setRowBusy((b) => ({ ...b, [n]: false }));
    }
  }

  // Outside-click / Escape / scroll dismissal is deliberately NOT wired up
  // here any more: it moved into DropdownListbox along with the panel itself
  // (see that component's header for the clipping bug that forced the panel
  // out of this card and into a portal). A second copy left behind here would
  // have been actively wrong — the old handler asked "is the mousedown inside
  // `pickerRef`", which a portalled option button no longer is, so it would
  // have unmounted the list on mousedown and the option's own click would
  // never have landed.

  if (!open) return null;

  const sortedSelected = [...selected].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: "base" }));

  return (
    <div className="mt-2 rounded-card bg-carbon-background p-3 flex flex-col gap-2">
      <p className="text-xs text-carbon-textMuted">{t("stophook.hint")}</p>
      {/* Picker trigger deliberately stays plain `bg-carbon-surface2` (not
          rainbow-hued): every other VALUE picker in this app (this same
          file's own offsite-target `<select>`, the Language/Theme card
          dropdowns, FolderBrowser's text field) is plain neutral chrome —
          rainbow hue in this app marks a genuine ACTION control (FoldersEditor's
          own "Hinzufügen" icon badge is that pattern's live example), never a
          value-holding input/picker. This editor's own former Save badge is
          gone entirely (live-save conversion, see this component's own
          top-level comment) — every row below now persists itself. */}
      <div className="inline-block">
        <button
          ref={pickerRef}
          type="button"
          aria-haspopup="listbox"
          aria-expanded={pickerOpen}
          onClick={() => setPickerOpen((v) => !v)}
          className="flex items-center gap-2 w-64 max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs text-carbon-text hover:bg-carbon-hover transition-colors text-start"
        >
          <span className="min-w-0 flex-1 truncate">{t("stophook.title")}</span>
          <svg width="10" height="10" viewBox="0 0 12 12" fill="none" className={`shrink-0 transition-transform ${pickerOpen ? "rotate-90" : "rtl:rotate-180"}`}>
            <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
          </svg>
        </button>
        {/* Portalled, not `absolute` inside this card: ContainerRow's own
            wrapper is `relative overflow-hidden` (ProgressBar needs that
            clip), which hard-clipped this panel at the card's bottom edge no
            matter what z-index it carried — jdp, live review: "Sie soll über
            die Card hinausgehen und voll angezeigt werden." See
            DropdownListbox.tsx for the full root cause. */}
        <DropdownListbox
          open={pickerOpen}
          onClose={() => setPickerOpen(false)}
          triggerRef={pickerRef}
          label={t("stophook.title")}
          multiselectable
        >
          <>
            {candidates.length === 0 && sortedSelected.length === 0 && (
              <p className="px-3 py-2 text-xs text-carbon-textMuted">{t("stophook.noCandidates")}</p>
            )}
            {candidates.map((c) => {
              const checked = selected.has(c.name);
              return (
                <button
                  // Keyed by name PLUS its own shake nonce — see
                  // FoldersEditor's identical mount-row key comment.
                  key={`${c.name}-${rowShake[c.name] ?? 0}`}
                  type="button"
                  role="option"
                  aria-selected={checked}
                  onClick={() => void toggle(c.name)}
                  disabled={!!rowBusy[c.name]}
                  className={`flex items-center gap-2.5 w-full px-3 py-2 text-xs text-start transition-colors disabled:opacity-60 ${
                    checked ? "bg-carbon-surface3 text-carbon-text" : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
                  }${rowShake[c.name] ? " glim-shake" : ""}`}
                >
                  <input type="checkbox" checked={checked} readOnly tabIndex={-1} className="pointer-events-none" style={{ accentColor: "var(--accent)" }} />
                  <span className="min-w-0 flex-1 truncate">{c.name}</span>
                </button>
              );
            })}
            {/* Stale entries: a previously-saved name no longer among the
                installed candidates above — still listed (so it stays
                removable) but marked with the existing notInstalled label. */}
            {sortedSelected.filter((n) => !candidateNames.has(n)).map((n) => (
              <button
                key={`${n}-${rowShake[n] ?? 0}`}
                type="button"
                role="option"
                aria-selected
                onClick={() => void toggle(n)}
                disabled={!!rowBusy[n]}
                className={`flex items-center gap-2.5 w-full px-3 py-2 text-xs text-start text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-60${rowShake[n] ? " glim-shake" : ""}`}
              >
                <input type="checkbox" checked readOnly tabIndex={-1} className="pointer-events-none" style={{ accentColor: "var(--accent)" }} />
                <span dir="ltr" className="min-w-0 flex-1 truncate font-mono text-start">{n}</span>
                <span className="shrink-0 text-caption text-statusFail">{t("containers.notInstalled")}</span>
              </button>
            ))}
          </>
        </DropdownListbox>
      </div>
      {sortedSelected.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {sortedSelected.map((n) => (
            <span
              key={`${n}-${rowShake[n] ?? 0}`}
              className={`inline-flex items-center gap-1.5 rounded-control bg-carbon-surface2 px-2 py-0.5 text-xs text-carbon-textSub${rowShake[n] ? " glim-shake" : ""}`}
            >
              {n}
              <Button
                label={t("stophook.remove").replace("{name}", n)}
                labelKey="stophook.remove"
                variant="chip"
                onClick={() => void toggle(n)}
                disabled={!!rowBusy[n]}
              />
            </span>
          ))}
        </div>
      )}
    </div>
  );
}

// ExcludesEditor edits this container's restic exclude patterns, one per line,
// and shows a debounced live preview of how each line resolves against the
// container's live mounts: a container path is translated to the anchored host
// path restic stored (shown muted), a bare name passes through, and a line that
// would exclude nothing is warned. Clones StopContainersEditor + a preview pane.
type ExcludePreviewRow = { raw: string; resolved: string; status: string; matches: boolean };

// `open` is controlled by the caller — see HooksEditor's own comment.
export function ExcludesEditor({ name, initial, open, t }: { name: string; initial: string[]; open: boolean; t: T }) {
  const [text, setText] = useState(initial.join("\n"));
  const [state, setState] = useState<"idle" | "saving">("idle");
  const { push } = useToast();
  const [preview, setPreview] = useState<ExcludePreviewRow[]>([]);
  // Live-save conversion (jdp, live review — see HooksEditor's own header
  // comment for the full "why" across all four editors): the manual Save
  // button + its own `shakeSave` nonce are GONE — the textarea below now
  // debounce-auto-saves itself the same way HooksEditor's two inputs do (see
  // that component's own comment for why no shake/revert applies to free
  // text). `state`/`saveLines` stay exactly as they were: the assistant's
  // one-click suggestion chips (addExclude/removeExclude below) already
  // saved immediately, no button involved, before this conversion — that
  // half of this editor was ALREADY live-save and needed no change.
  const { debouncedSave, cancel: cancelPendingSave } = useDebouncedSave();

  // Debounced live preview: whenever the editor is open and the textarea holds at
  // least one non-blank line, resolve the candidate lines against the container's
  // mounts (~400ms after the last keystroke). Depends only on `text`/`open`, so
  // it re-previews on real edits — never in a loop (setPreview doesn't touch text).
  useEffect(() => {
    if (!open) return;
    const lines = text.split("\n").map((s) => s.trim()).filter(Boolean);
    if (lines.length === 0) {
      setPreview([]);
      return;
    }
    let cancelled = false;
    const id = setTimeout(() => {
      previewContainerExcludes(name, lines)
        .then((r) => {
          if (!cancelled) setPreview(r.ok ? r.preview : []);
        })
        .catch(() => {
          if (!cancelled) setPreview([]);
        });
    }, 400);
    return () => {
      cancelled = true;
      clearTimeout(id);
    };
  }, [text, name, open]);

  // The current exclude lines as the editor holds them (unsaved edits included) —
  // the single source both the save button and the assistant's one-click actions
  // work from, so they can never diverge.
  const currentLines = text
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean);

  // saveLines persists an explicit line list and mirrors it back into the
  // textarea, sharing the editor's save state machine. The debounced textarea
  // auto-save below passes the freshly-typed lines; the assistant passes the
  // list ± one line.
  async function saveLines(list: string[]) {
    setState("saving");
    try {
      const r = await setContainerExcludes(name, list);
      if (r.ok) {
        setText(list.join("\n"));
        push(t("excludes.saved"), "success");
      } else {
        push(r.error ?? t("excludes.error"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("excludes.error"), "fail");
    } finally {
      setState("idle");
    }
  }

  // --- Exclusion assistant: server-side scan for junk/large folders with
  // one-click exclude.
  const [assistOpen, setAssistOpen] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [suggestions, setSuggestions] = useState<ExcludeSuggestion[] | null>(null); // null = not scanned yet
  const [truncated, setTruncated] = useState(false);
  // Where the sizes came from, and the promise that goes with them (#175): the
  // snapshot source is exact but AS OF snapshotTime, which is why the timestamp
  // is rendered next to the list rather than treated as optional polish; the
  // live source is as of now but can stop early, inside stoppedAt.
  const [source, setSource] = useState<"snapshot" | "live">("live");
  // Why the live walk ran. Three different facts, three different sentences —
  // see liveSourceKey.
  const [liveReason, setLiveReason] = useState<"no-snapshot" | "requested" | "not-in-snapshot">(
    "no-snapshot"
  );
  const [snapshotTime, setSnapshotTime] = useState("");
  const [stoppedAt, setStoppedAt] = useState("");
  // Backup folders the walk never opened (its budget went on an earlier one) and
  // ones it could not read at all. Neither can be expressed by a per-row flag:
  // a folder that produced no rows has nothing to flag, and with several roots
  // an unmentioned one reads as a finished scan of all of them (#175).
  const [unexamined, setUnexamined] = useState<string[]>([]);
  const [unreadable, setUnreadable] = useState<string[]>([]);
  // The folders are configured but none is reachable (unmounted array/share) and
  // there was no backup to read instead. "Nothing left to exclude" would be the
  // loudest lie this panel can tell.
  const [pathsUnavailable, setPathsUnavailable] = useState(false);
  // The backup index could not be read. Not a failed scan: the panel stays up
  // and offers the folder scan as an explicit second request.
  const [indexFailed, setIndexFailed] = useState(false);
  // Distinguishes "scanned, found nothing" from "the scan itself failed" for the
  // "nothing found" hint below — the failure TEXT itself no longer lives here
  // (see scan()'s own comment), just the fact of it.
  const [scanFailed, setScanFailed] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Scan/Rescan button alongside its existing toast on a failed scan.
  const [shakeScan, setShakeScan] = useState(0);

  // GlimStone follow-up pass (v8.0.0): the scan-failed inline error is now a
  // toast — a Scan/Rescan click is a one-shot action like every other migrated
  // save/test button here. `truncated` (rendered below) stays inline: it is a
  // persistent fact ABOUT the current suggestion list ("this list was cut
  // short"), not a completion notice of the scan action itself.
  async function scan(live = false) {
    setScanning(true);
    setScanFailed(false);
    setIndexFailed(false);
    // Every "why is this list short" fact below describes ONE scan: the one that
    // produced the list currently on screen. A rescan that fails leaves no list,
    // so keeping the previous run's truncation banner up would describe a scan
    // that no longer exists ("the scan hit its time limit inside /config/Media"
    // sitting above an empty panel and a failure toast).
    setTruncated(false);
    setStoppedAt("");
    setUnexamined([]);
    setUnreadable([]);
    setPathsUnavailable(false);
    try {
      const r = await suggestContainerExcludes(name, live ? "live" : undefined);
      if (r.ok) {
        setSuggestions(r.suggestions);
        setTruncated(r.truncated);
        setSource(r.source ?? "live");
        // A scan the user asked for is known to be one HERE, whatever the server
        // says: the fallback must never be the "no backup yet" sentence on a
        // request that only exists because a backup was there to read.
        setLiveReason(r.liveReason ?? (live ? "requested" : "no-snapshot"));
        setSnapshotTime(r.snapshotTime ?? "");
        setStoppedAt(r.stoppedAt ?? "");
        setUnexamined(r.unexaminedRoots ?? []);
        setUnreadable(r.unreadableRoots ?? []);
        setPathsUnavailable(r.pathsUnavailable === true);
        setIndexFailed(r.indexFailed === true);
      } else {
        setSuggestions([]);
        setScanFailed(true);
        push(r.error ?? t("excludes.assistScanFailed"), "fail");
        setShakeScan((n) => n + 1);
      }
    } catch (err) {
      setSuggestions([]);
      setScanFailed(true);
      push(err instanceof Error ? err.message : t("excludes.assistScanFailed"), "fail");
      setShakeScan((n) => n + 1);
    }
    setScanning(false);
  }

  function toggleAssistant() {
    const opening = !assistOpen;
    setAssistOpen(opening);
    if (opening && suggestions === null) void scan();
  }

  // Both chip paths save IMMEDIATELY, so they retire any pending textarea
  // debounce first — see cancel()'s own comment. currentLines is derived from
  // the textarea's live value, so the list sent here already carries whatever
  // was typed in that same window; the dropped timer would only have written an
  // older copy of it.
  async function addExclude(line: string) {
    if (currentLines.includes(line)) return;
    cancelPendingSave();
    await saveLines([...currentLines, line]);
  }

  async function removeExclude(line: string) {
    cancelPendingSave();
    await saveLines(currentLines.filter((l) => l !== line));
  }

  // A suggestion whose line is already stored disappears from the list (it shows
  // up in the current-exclusions chips instead).
  const openSuggestions = (suggestions ?? []).filter((sg) => !currentLines.includes(sg.line));

  const inputCls =
    "rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-2 py-1 bv-field-focus";

  if (!open) return null;

  return (
    <div className="mt-2 rounded-card bg-carbon-background p-3 flex flex-col gap-2">
      <p className="text-xs text-carbon-textMuted">
        {withLtrFragments(t("excludes.hint"), EXCLUDES_HINT_LTR_FRAGMENTS)}
      </p>
      <textarea
        value={text}
        onChange={(e) => {
          const nextText = e.target.value;
          setText(nextText);
          // Debounced auto-save (800ms after the last keystroke) — see this
          // component's own top-level comment. Lines are parsed from
          // `nextText` right here, not re-read from `text` when the timer
          // fires, matching Settings.tsx's own "compute the next value
          // locally, pass it straight into the debounced closure" shape.
          const nextLines = nextText.split("\n").map((s) => s.trim()).filter(Boolean);
          debouncedSave(() => void saveLines(nextLines));
        }}
        spellCheck={false}
        rows={3}
        placeholder={tLtr(t, "excludes.placeholder")}
        dir="ltr"
        className={`${inputCls} text-start`}
      />
      {preview.length > 0 && (
        <div className="flex flex-col gap-1">
          {preview.map((row, i) => {
            // Show a plain, reassuring confirmation — NOT the raw internal
            // restic path (BombVault's rebased host-mount view, e.g.
            // /host/user/user/appdata/…), which looked like an invalid path
            // and confused users (#38). The exact pattern is still available
            // on hover (title) for the curious.
            const good = row.matches;
            const msg = good
              ? row.status === "basename"
                ? t("excludes.matchesAnywhere")
                : t("excludes.willExclude")
              : row.status === "passthrough"
                ? t("excludes.noMatch")
                : t("excludes.excludesNothing");
            return (
              <div
                key={i}
                className="text-xs wrap-break-word leading-snug flex items-baseline gap-1.5"
                title={row.status === "translated" ? row.resolved : undefined}
              >
                <span dir="ltr" className="font-mono text-carbon-textSub text-start">{row.raw}</span>
                <span className={good ? "text-statusOk" : "text-statusFail"}>
                  {good ? "✓" : "⚠"} {msg}
                </span>
              </div>
            );
          })}
        </div>
      )}

      {/* Exclusion assistant */}
      <div className="mt-1 flex flex-col gap-2">
        <Button
          label={t("excludes.assistTitle")}
          labelKey="excludes.assistTitle"
          tone="neutral"
          onClick={toggleAssistant}
          glyph={
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none" className={`transition-transform ${assistOpen ? "rotate-90" : "rtl:rotate-180"}`}>
              <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
            </svg>
          }
        />
        {assistOpen && (
          <div className="flex flex-col gap-2">
            <p className="text-xs text-carbon-textMuted">{t("excludes.assistHint")}</p>
            <div className="flex items-center gap-3">
              {/* Colour-engine integration (Task 3, same fix/reasoning as
                  FoldersEditor's "Hinzufügen" button above): was the one
                  plain grey `bg-carbon-surface2` button in this
                  assistant sub-panel, next to its own "Ausschließen"
                  suggestion-accept button below which was ALREADY
                  `bg-accent` — matches that sibling now, same
                  already-correct .glim-hue-cascade mechanism. */}
              <Button
                key={shakeScan}
                label={suggestions === null
                    ? t("excludes.assistScan")
                    : t("excludes.assistRescan")}
                labelKey={suggestions === null ? "excludes.assistScan" : "excludes.assistRescan"}
                glyph={<IconSearch />}
                tone="accent"
                onClick={() => void scan()}
                disabled={scanning}
                busy={scanning}
                title={scanning ? t("excludes.assistScanning") : undefined}
                className={shakeScan ? "glim-shake" : ""}
              />
              {/* The standing "what is on disk RIGHT NOW" question. A snapshot
                  cannot answer it: a junk folder created since the last backup
                  is not in the index, so it has no row and no warning at all,
                  and "what can I stop backing up" is exactly the question a
                  cache that exploded yesterday answers. Offered whenever the
                  list came from a backup, not only after an index failure.
                  NOT while indexFailed: that branch already offers the same
                  action under its own label, and two differently-worded
                  buttons for one thing read as two different things. */}
              {!scanning && suggestions !== null && !scanFailed && !indexFailed && source === "snapshot" && (
                <Button
                  label={t("excludes.assistScanCurrent")}
                  labelKey="excludes.assistScanCurrent"
                  tone="neutral"
                  onClick={() => void scan(true)}
                />
              )}
              {truncated && !scanning && stoppedAt && (
                // Stays inline and stays a separate line from the per-row size
                // flags below: a folder the walk never reached has no row at
                // all, so "the rest was not examined" is a claim no per-row flag
                // can make. It names WHERE the list ends (#175), and the folder
                // is pinned LTR so the leading `/` does not migrate to the far
                // end of the path in ar/he/fa.
                <span className="text-xs text-statusWarn">
                  {withLtrPlaceholder(t("excludes.assistTruncated"), "{path}", stoppedAt)}
                </span>
              )}
            </div>
            {/* Whole backup folders that produced no rows, for two different
                reasons. Both are claims no per-row flag can make, and with
                several roots their absence read as a finished scan of all of
                them. */}
            {!scanning && unexamined.length > 0 && (
              <p className="text-xs text-statusWarn">
                {withLtrPlaceholder(t("excludes.assistUnexamined"), "{paths}", unexamined.join(", "))}
              </p>
            )}
            {!scanning && unreadable.length > 0 && (
              <p className="text-xs text-statusWarn">
                {withLtrPlaceholder(t("excludes.assistUnreadable"), "{paths}", unreadable.join(", "))}
              </p>
            )}
            {!scanning && pathsUnavailable && (
              <p className="text-xs text-statusWarn">{t("excludes.assistPathsUnavailable")}</p>
            )}
            {!scanning && indexFailed && (
              <div className="flex items-center gap-3">
                <span className="text-xs text-statusWarn">{t("excludes.assistIndexFailed")}</span>
                <Button
                  label={t("excludes.assistScanLive")}
                  labelKey="excludes.assistScanLive"
                  tone="accent"
                  onClick={() => void scan(true)}
                />
              </div>
            )}
            {/* "Nothing left to exclude" is a POSITIVE finding and may only be
                said when the scan actually looked. pathsUnavailable means it
                could not look at all. */}
            {!scanning && suggestions !== null && !scanFailed && !indexFailed && !pathsUnavailable && openSuggestions.length === 0 && (
              <p className="text-xs text-carbon-textMuted">{t("excludes.assistNothingFound")}</p>
            )}
            {!scanning && openSuggestions.length > 0 && (
              <div className="flex flex-col gap-1">
                {openSuggestions.map((sg) => (
                  <div
                    key={sg.line}
                    title={sg.line}
                    className="flex items-center gap-2 rounded-control bg-carbon-surface2 px-2 py-1.5"
                  >
                    <span dir="ltr" className="min-w-0 flex-1 truncate font-mono text-xs text-carbon-text text-start">{sg.path}</span>
                    <span
                      // Task 7: "cache" was bg-statusInfoBg/text-statusInfo (the
                      // old fifth hue). This is a categorisation label — "this
                      // looks like a cache dir" — not activity and not a
                      // pass/fail/warn outcome, so it folds into --status-neutral-*
                      // (already documented in index.css as "skipped/neutral
                      // chip", the same broad "not a real state" bucket this
                      // chip belongs in, sitting next to its "large" sibling
                      // which keeps its own real warn meaning unchanged).
                      className={`inline-flex items-center rounded-control px-2 py-0.5 text-xs font-medium ${
                        sg.reason === "large" ? "bg-statusWarnBgStrong text-statusWarn" : "bg-statusNeutralBg text-statusNeutral"
                      }`}
                    >
                      {sg.reason === "large" ? t("excludes.assistReasonLarge") : t("excludes.assistReasonCache")}
                    </span>
                    {/* #175: a size the scan could not finish measuring is a
                        MINIMUM, and says so. Rendering it as a plain number is
                        what showed a 55 GB folder as "5.7 GB". */}
                    {sg.complete ? (
                      <span className="text-xs text-carbon-textSub whitespace-nowrap">{humanBytes(sg.sizeBytes)}</span>
                    ) : (
                      // Same tone as the exact figure on purpose. It used to be
                      // text-carbon-textMuted, which made the least legible text
                      // in the row the one carrying the caveat, in both colour
                      // modes. The explanation moved out of a native title= —
                      // invisible on touch — into the house InfoBubble.
                      <span className="inline-flex items-center gap-1 whitespace-nowrap">
                        <span className="text-xs text-carbon-textSub">
                          {t("excludes.assistSizeAtLeast").replace("{size}", humanBytes(sg.sizeBytes))}
                        </span>
                        <InfoBubble tip={t("excludes.assistSizeMinimumTip")} />
                      </span>
                    )}
                    <Button
                      label={t("excludes.assistExclude")}
                      labelKey="excludes.assistExclude"
                      tone="accent"
                      onClick={() => void addExclude(sg.line)}
                      disabled={state === "saving"}
                    />
                  </div>
                ))}
              </div>
            )}
            {/* Where the numbers come from. Required, not decoration: a
                snapshot size is exact but AS OF that backup, so a cache that
                has grown tenfold since reads at its old size — stating the date
                is the whole reason that is honest rather than misleading. */}
            {!scanning && suggestions !== null && !scanFailed && !indexFailed && (
              <p className="text-xs text-carbon-textMuted">
                {source === "snapshot"
                  ? t("excludes.assistSourceSnapshot").replace("{when}", formatSnapshotWhen(snapshotTime))
                  : t(liveSourceKey(liveReason))}
              </p>
            )}
            {/* The date above is necessary and not sufficient. A snapshot list
                cannot show a folder that did not exist when the backup ran, so
                once the backup is old enough to matter the panel says so rather
                than leaving the user to compare a timestamp. */}
            {!scanning && suggestions !== null && !scanFailed && !indexFailed && source === "snapshot" && snapshotIsStale(snapshotTime) && (
              <p className="text-xs text-statusWarn">{t("excludes.assistSnapshotStale")}</p>
            )}
            <p className="text-xs text-carbon-textSub">{t("excludes.assistCurrent")}</p>
            {currentLines.length === 0 ? (
              <p className="text-xs text-carbon-textMuted">{t("excludes.assistNoneYet")}</p>
            ) : (
              <div className="flex flex-wrap gap-1.5">
                {currentLines.map((line) => (
                  <span
                    key={line}
                    className="inline-flex items-center gap-1.5 rounded-control bg-carbon-surface2 px-2 py-0.5 text-xs font-mono text-carbon-textSub"
                  >
                    {line}
                    <Button
                      label={t("excludes.assistRemoveLine").replace("{line}", line)}
                      labelKey="excludes.assistRemoveLine"
                      variant="chip"
                      onClick={() => void removeExclude(line)}
                      disabled={state === "saving"}
                      title={t("excludes.assistRemove")}
                    />
                  </span>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// updateCheckResultText maps the stored update-check result literal to its
// translated display text; an unknown literal falls back to the raw string.
function updateCheckResultText(t: T, result: string): string {
  switch (result) {
    case "up-to-date":
      return t("containers.updateCheckUpToDate");
    case "updated":
      return t("containers.updateCheckUpdated");
    case "failed":
      return t("containers.updateCheckFailed");
    default:
      return result;
  }
}

function ContainerRow({
  container,
  installedContainers,
  t,
  onDeleted,
  selected,
  onToggleSelect,
  index,
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
  selected?: boolean;
  onToggleSelect?: () => void;
  /** Position in the rendered list — the rainbow palette position (GlimStone
   *  form-engine Phase 2, Task 2). A container list is exactly the case the
   *  mode exists for: a variable, user-configured set a person tracks several
   *  of at once. Assigned by LIST INDEX, never a hash of `container.name` —
   *  see the callers below. */
  index: number;
}) {
  const installed = container.installed;
  const progressMap = useProgress();
  const progress = progressMap[`container:${container.name}`];
  // "Something is running" across any domain — used to busy-guard this row's
  // own backup button (its OWN in-flight backup is handled by isPending inside).
  const running = anyActive(progressMap);
  const { advanced } = useAdvanced();

  // GlimStone follow-up round (jdp, live-review, screenshot of this exact
  // row's five stacked disclosure triggers: "Können wir hier Buttons machen
  // die alle in einer Zeile stehen?") — ONE state bag for all five sections
  // (Gesicherte Ordner/Andere Container stoppen/Ausschlussmuster/Backup-
  // Hooks/Backups), replacing the five separate internal `useState` booleans
  // each editor used to own. Confirmed against the PRE-existing code before
  // touching any of it: every one of the five toggled independently already
  // (five separate `useState(false)`s, none of them ever closing a sibling),
  // so this is a `Set`, not a single active id — the shared row below uses
  // Selector's `select="many"` mode for exactly that shape (the SAME
  // independent-toggle-chips pattern CadenceBuilder's own weekday multi-
  // select already established on this codebase, not a tablist/accordion).
  // ONE section at a time (jdp: "Von den tabs in der container-card ... soll
  // immer nur einer angezeigt werden. jetzt stapeln sie sich untereinander wenn
  // man den tab wechselt"). This reverses the original design, which made them
  // independently openable on purpose — see the `sectionItems` comment further
  // down, which still explains that reasoning. It reads fine on paper and
  // stacks four editors down the card in practice.
  //
  // The state stays a `Set` rather than becoming `string | undefined`, because
  // the five editors below each take an `open` boolean off `.has(...)` and the
  // Selector stays `select="many"` — which is what keeps a section CLOSABLE by
  // clicking its own chip again. A `select="one"` strip always has exactly one
  // thing selected and could never close the last one.
  const [openSections, setOpenSections] = useState<Set<string>>(() => new Set());
  function toggleSection(id: string) {
    setOpenSections((prev) => (prev.has(id) ? new Set() : new Set([id])));
  }

  // "Has data configured" indicator — the same three facts
  // StopContainersEditor/ExcludesEditor/HooksEditor used to check internally
  // to decide whether to render their own green dot next to their own label.
  // Computed once here instead, since the dot now lives on the SHARED chip
  // (see `sectionItems` below), not on each editor's own now-removed trigger.
  const stopHasData = (container.stopContainers ?? []).length > 0;
  const excludesHasData = (container.excludes ?? []).length > 0;
  const hooksHasData = !!(container.preHook || container.postHook);
  const configuredDot = (
    <span aria-hidden className="h-1.5 w-1.5 rounded-full bg-statusOk shrink-0" />
  );

  // Folders/Stop/Excludes/Hooks are advanced+installed-only, mirroring the
  // exact `advanced && installed` gate `<Advanced when={installed}>` always
  // enforced for them (see the render below) — so their chips simply don't
  // exist otherwise, rather than existing disabled. Backups is unconditional.
  const sectionItems: SelectorItem[] = [];
  if (advanced && installed) {
    sectionItems.push(
      { id: "folders", label: t("folders.title") },
      { id: "stop", label: t("stophook.title"), icon: stopHasData ? configuredDot : undefined },
      { id: "excludes", label: t("excludes.title"), icon: excludesHasData ? configuredDot : undefined },
      { id: "hooks", label: t("hooks.title"), icon: hooksHasData ? configuredDot : undefined }
    );
  }
  sectionItems.push({ id: "backups", label: t("snapshots.title") });

  const lastBackupText = `${t("containers.lastBackup")}: ${container.lastBackup ? formatTs(container.lastBackup) : t("containers.never")}`;

  return (
    <div
      style={{ ...hueVars(rainbowAt(index)), "--row-i": String(index) } as CSSProperties}
      // glim-hue owns the position; glim-tint washes the WHOLE card with it
      // (trap #2, design-language.md's "Rainbow" section) — without the wash
      // this card shows almost no colour at rest, since nothing else on it
      // reads --accent except the checkbox and the (usually hidden) backup
      // button. glim-active while a backup/restore is actively running on
      // THIS row: reactive mode then shows the hue without needing hover,
      // same as knightloader's TaskRow keying off task.status === 'running'.
      // bv-stagger-row (GlimStone motion-engine animation 3) reuses this
      // SAME `index` (via --row-i) the colour engine already threads through
      // every call site — see that class's own keyframe comment in index.css.
      className={`relative overflow-hidden bg-carbon-surface rounded-card p-4 flex flex-col gap-3 glim-hue glim-tint bv-stagger-row ${
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
            {installed ? (
              <Badge tone={stateTone(container.state)}>{stateLabel(t, container.state)}</Badge>
            ) : (
              <Badge tone="neutral">{t("containers.notInstalled")}</Badge>
            )}
            {container.ip && (
              <span dir="ltr" className="text-xs text-carbon-textMuted font-mono text-start">{container.ip}</span>
            )}
          </div>
          {container.image && (
            <p dir="ltr" className="text-xs text-carbon-textMuted mt-0.5 truncate text-start">{container.image}</p>
          )}
        </div>

        {/* Action badges (Task 2, jdp live-review: "Jetzt sichern und Export
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
            row below. */}
        {installed && (
          <div className="ms-auto flex items-start gap-1.5 shrink-0">
            {container.self ? (
              <span className="text-xs text-carbon-textMuted max-w-[18rem] text-end">
                {t("containers.selfNote")}
              </span>
            ) : (
              <>
                <BackupButton name={container.name} t={t} onBackedUp={onDeleted} running={running} />
                {/* Plain tar+xml export is an advanced-only extra. */}
                <Advanced><ExportButton name={container.name} t={t} /></Advanced>
              </>
            )}
          </div>
        )}
      </div>

      {/* Actions row — schedule-include + update-after-backup, both flush
          right (Task 4/5, jdp live-review: "Im Zeitplan einschließen: Toggle
          ganz nach rechts. Darunter der Toggle für Update nach Backup"). Both
          toggles used to live at opposite ends of this row (include on the
          left) / scattered among the advanced editors below (update-after-
          backup) — "Jetzt sichern"/Export moving to the top-right icon-badge
          corner above (Task 2) freed this row up to become a single flush-
          right stack instead. DeleteBackupsButton (not-installed branch) is
          unrelated to either toggle and stays at the row's own start.
          IncludeToggle no longer needs a wrapping `<label>`/`<span>` here
          (Task 2 follow-up, jdp live-review — "gleich anordnen ... Text ...
          ganz links ... Toggle ganz rechts"): it now renders the full
          ToggleRow itself, the identical shape UpdateAfterBackupRow already
          renders right below it, so both toggles share one markup instead of
          two independently hand-matched ones — see IncludeToggle.tsx's own
          comment. */}
      <div className="flex items-start">
        {installed ? (
          <div className="ms-auto flex flex-col items-end gap-2">
            <IncludeToggle name={container.name} initial={container.includeInSchedule} />
            <Advanced>
              <UpdateAfterBackupRow
                name={container.name}
                initial={container.updateAfterBackup ?? false}
                lastUpdateCheck={container.lastUpdateCheck}
                lastUpdateResult={container.lastUpdateResult}
                t={t}
              />
            </Advanced>
          </div>
        ) : (
          /* Not installed: can't back up; offer delete-all-backups instead. */
          <DeleteBackupsButton name={container.name} t={t} onDeleted={onDeleted} />
        )}
      </div>

      {/* Disclosure-section trigger row (GlimStone follow-up round, jdp
          live-review, screenshot of the five stacked full-width triggers
          below: "Können wir hier Buttons machen die alle in einer Zeile
          stehen?") — Gesicherte Ordner/Andere Container stoppen/
          Ausschlussmuster/Backup-Hooks are advanced+installed-only (the same
          `advanced && installed` gate `<Advanced when={installed}>` always
          enforced); Backups is always offered (works even when not
          installed, same as before). One shared `Selector` in `select="many"`
          mode — the established "several independent toggle chips in one
          row" pattern (CadenceBuilder's own weekday multi-select is the
          precedent), NOT a tablist/accordion: `openSections` is a `Set`
          precisely because these stay independently openable, never
          mutually exclusive. Each chip's own rainbow position comes from
          Selector's own default per-item hueing (list index 0..4), the same
          mechanism every other Selector row on this page already uses — nesting
          it inside this already-hued ContainerRow doesn't fight that; it's the
          same "a row of several items gets its own hue sequence" rule
          CadenceBuilder's weekday pills already apply inside their own
          (also-hued) Settings card.
          The small green dot HooksEditor/StopContainersEditor/ExcludesEditor
          used to render next to their own label ("has data configured")
          survives as the chip's own leading `icon` slot — a plain
          `bg-statusOk` dot, not an `<svg>`, so Selector's `.glim-hue-icon`
          rule (which only ever touches an `<svg>` descendant — checked
          against index.css) never tints it; it stays the same fixed status
          colour regardless of the chip's own hue or open/closed state.
          `lastBackupText` (used to share the "Backups" trigger's own line,
          Task 3's "eine Zeile, nicht zwei") is rendered next to the row
          instead — always-visible summary data, not part of the expandable
          content, wrapping onto its own line at narrow widths via the same
          `flex-wrap` this row already needs for the chips themselves. */}
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2 flex-wrap">
          <Selector
            items={sectionItems}
            label={t("containers.sectionsLabel")}
            select="many"
            active={openSections}
            onChange={toggleSection}
          />
          <span className="ms-auto shrink-0 text-xs text-carbon-textMuted whitespace-nowrap">
            {lastBackupText}
          </span>
        </div>

        {/* Content panes, in a fixed, predictable order regardless of which
            chip was clicked last — each editor now takes `open` as a PROP
            (from the shared `openSections` above) instead of owning its own
            internal useState; see each editor's own comment. Still gated by
            the same `<Advanced when={installed}>` the trigger row's own
            `sectionItems` construction mirrors, so these stay entirely
            unmounted (no wasted fetches/effects) whenever their chip
            couldn't have been clicked in the first place. */}
        <Advanced when={installed}>
          <FoldersEditor name={container.name} stack={container.stack} open={openSections.has("folders")} t={t} />
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
        <RestorePanel name={container.name} t={t} installed={installed} open={openSections.has("backups")} />
      </div>

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

// ---------------------------------------------------------------------------
// Stacks panel (compose-project restore)
// ---------------------------------------------------------------------------

interface StackGroup {
  project: string;
  members: Container[];
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

// Grace after the last member goes inactive before a stack restore is treated as
// finished. Comfortably longer than the per-member progress linger (~800ms) plus
// the gap before the next member starts, so the cancel button doesn't flicker out
// between sequential members.
const STACK_DONE_GRACE_MS = 8000;

// StackCard is one compose stack: its name, members, and (in a collapsible panel)
// a "Restore stack" action that restores every member stopped, then optionally
// starts them in dependency order. The restore is ASYNC on the server (the POST
// only acks {started:true} and carries no member results), so on start the card
// shows a sticky "restore started" hint; per-member outcomes land in the run
// history. Synchronous validation errors (empty stack, busy, …) show inline.
function StackCard({
  group,
  onRestored,
  t,
  index,
}: {
  group: StackGroup;
  onRestored: () => void;
  t: T;
  /** Rainbow position for THIS card — GlimStone standing colour-engine rule
   *  (jdp, live review, emphatic, five escalations deep: "Es soll immer
   *  alles in die Farb- und Formengine integriert werden!! IMMER!!"). A gap
   *  that survived even the fifth escalation's own sweep of this file (see
   *  StacksPanel's own `hueIndex` doc comment above, which hued the panel's
   *  HEADING but left every card underneath it flat): StackCard is the exact
   *  same "row card in a list" shape as ContainerRow right above it in this
   *  file (and Files.tsx/Fleet.tsx/Receiver.tsx/VMs.tsx's own list-row
   *  cards) — glim-hue/glim-tint/bv-stagger-row + `hueVars(rainbowAt(index))`
   *  — yet was the one card shape in this file with NO colour-engine wiring
   *  at all. By LIST INDEX among the stacks rendered together (StacksPanel's
   *  own `stacks.map`), a separate local 0-based sequence from
   *  ContainerRow's own `live`/`orphans` index (a different list, own local
   *  index per group — the same rule ToggleRow's own `hueIndex` doc
   *  documents) and from the page-wide `nextHue()` counter the panel's own
   *  heading badge uses (a heading notch and its list's row cards are two
   *  independent sequences, same split as every other headed list on this
   *  page). */
  index: number;
}) {
  const [open, setOpen] = useState(false);
  const [source, setSource] = useState<RepoSource>("local");
  const [startInOrder, setStartInOrder] = useState(true);
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Restore button when the restore fails to even START (see run()'s own
  // comment for why a started-then-running restore stays a durable inline
  // status instead — the shake, like the toast, only covers the click itself).
  const [shake, setShake] = useState(0);
  const [started, setStarted] = useState(false);
  // Terminal state for the stack restore: since StackCard drives no fire-and-
  // watch of its own, we derive "finished" from the members' progress below.
  const [finished, setFinished] = useState(false);
  // The stack restore has no aggregate progress bar (it restores members one by
  // one under their own "container:<name>" keys). A member is "active" while it
  // is being restored; cancelling targets the synthetic "stack:<project>" key,
  // which aborts the member loop at the current member.
  const progress = useProgress();
  const anyMemberActive =
    started &&
    group.members.some((m) => {
      const p = progress[`container:${m.name}`];
      return !!p && p.active && p.phase === "restore";
    });
  // Once we have seen a member go active, keep the cancel button up for the WHOLE
  // restoring window (through the ~800ms linger + gap between sequential members)
  // and only flip to a neutral "finished" once NO member has been active for a
  // grace window longer than that gap — otherwise the cancel button flickered out
  // between members and the "runs in background" banner stayed sticky forever.
  const sawActive = useRef(false);
  const { confirm, confirmDialog } = useConfirm();
  useEffect(() => {
    if (!started) return;
    if (anyMemberActive) {
      sawActive.current = true;
      return; // renewed activity — the cleanup below cleared any pending terminal
    }
    if (!sawActive.current) return; // nothing has run yet: don't finish early
    const timer = setTimeout(() => {
      sawActive.current = false;
      setStarted(false); // reset so a later single-member restore can't resurrect
      setFinished(true); //   the stack cancel button
    }, STACK_DONE_GRACE_MS);
    return () => clearTimeout(timer);
  }, [started, anyMemberActive]);

  async function run() {
    if (!(await confirm(t("stack.restoreConfirm")))) return;
    setBusy(true);
    setStarted(false);
    setFinished(false);
    sawActive.current = false;
    try {
      const res = await restoreStack(group.project, startInOrder, true, source);
      if (res.ok) {
        setStarted(true);
        onRestored(); // refresh the main list so run-state/orphan rows update
      } else {
        // GlimStone follow-up pass (v8.0.0): a failure to even START the async
        // restore is a one-shot action-failed notice, now a toast. `started` /
        // `finished` above stay inline — once the restore DOES start, its
        // progress and eventual completion are a durable, ongoing status (a
        // background job with a Cancel button attached), not a one-shot ping;
        // see the render below for the same reasoning applied to `finished`.
        push(res.error ?? t("settings.error"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      style={{ ...hueVars(rainbowAt(index)), "--row-i": String(index) } as CSSProperties}
      // glim-hue owns the position; glim-tint washes the whole card with it,
      // bv-stagger-row reuses the same `index` for the entrance stagger — the
      // identical trio ContainerRow's own outer <div> carries above (see this
      // function's own `index` doc comment).
      className="relative overflow-hidden bg-carbon-surface rounded-card p-4 flex flex-col gap-2 glim-hue glim-tint bv-stagger-row"
    >
      <div className="flex items-start justify-between gap-3 flex-wrap">
        <div className="min-w-0">
          <span className="font-semibold text-carbon-text text-sm wrap-break-word">{group.project}</span>
          <span className="ms-2 text-xs text-carbon-textMuted">
            {t("stack.members").replace("{n}", String(group.members.length))}
          </span>
          <p className="mt-0.5 text-caption text-carbon-textMuted truncate">
            {group.members.map((m) => m.name).join(", ")}
          </p>
        </div>
        {/* Disclosure toggle (icon only) so the sole "Restore stack" label is the
            action button inside the panel.
              IconTipButton, not a plain <button> + `title`: it carried both
            an `aria-label` and a duplicate native `title` of the same string,
            the OS-balloon pairing IconTipButton.tsx exists to replace. Same
            tip, same handler, same chrome — and `ariaExpanded` (added to
            IconTipButton for exactly this call site) keeps the disclosure
            state this trigger has always exposed. */}
        <IconTipButton
          tip={t("stack.restore")}
          onClick={() => setOpen((p) => !p)}
          ariaExpanded={open}
          className="shrink-0 inline-flex items-center rounded-control p-1.5 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors"
        >
          <svg width="14" height="14" viewBox="0 0 12 12" fill="none" className={`transition-transform ${open ? "rotate-90" : "rtl:rotate-180"}`}>
            <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
          </svg>
        </IconTipButton>
      </div>

      {open && (
        <div className="mt-1 rounded-card bg-carbon-background p-3 flex flex-col gap-2">
          <p className="text-xs text-carbon-textMuted">{t("stack.restoreHint")}</p>
          <div className="flex items-center gap-2">
            <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
            <SourceToggle source={source} onChange={setSource} disabled={busy} domain="containers" />
          </div>
          <Toggle checked={startInOrder} onChange={setStartInOrder} label={t("stack.startInOrder")} />
          <div className="flex items-center gap-3 pt-0.5">
            <Button
              key={shake}
              label={t("stack.restore")}
              labelKey="stack.restore"
              tone="accent"
              onClick={() => void run()}
              disabled={busy}
              busy={busy}
              title={busy ? t("stack.restoring") : undefined}
              className={shake ? "glim-shake" : ""}
            />
          </div>

          {/* Async ack: the server runs the stack restore detached and the ack
              carries no member results — per-member outcomes are in the run
              history. The cancel button stays up for the whole restoring window
              (no per-member flicker); once every member goes inactive the panel
              flips to a neutral "finished" note (see the terminal effect above). */}
          {started && !busy && (
            <div className="flex flex-col gap-1">
              <p className="text-xs text-carbon-textSub">{t("restore.started")}</p>
              <p className="text-caption text-carbon-textMuted">{t("restore.bgHint")}</p>
              {/* Whole-stack in-place restore — hard warning, keyed to the stack. */}
              <RestoreCancelButton cancelKey={`stack:${group.project}`} inPlace name={group.project} t={t} />
            </div>
          )}
          {finished && !busy && (
            <p className="text-xs text-carbon-textSub">{t("stack.restoreFinished")}</p>
          )}
        </div>
      )}
      {confirmDialog}
    </div>
  );
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
  /** Rainbow position for THIS panel's own heading notch — GlimStone
   *  follow-up pass (jdp, live review, emphatic, fifth escalation of the
   *  standing colour-engine rule): this section heading was a
   *  tone="heading" Badge with no hueIndex at all, so in rainbow mode it
   *  stayed flat --accent instead of joining the same sequence the
   *  ContainerRow cards below it clearly carry (rainbowAt(index)). Resolved
   *  by the caller's own `nextHue()` counter, called DIRECTLY at the JSX
   *  call site, gated on `stackGroups.length > 0` there (never an
   *  unconditional call — see that call site's own comment for why: this
   *  component returns null internally whenever there is no multi-member
   *  stack, and hueIndex must never fire for a heading that won't actually
   *  render). Omit for a genuine singleton — same rule as every other
   *  hueIndex call site. */
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

// ---------------------------------------------------------------------------
// Backup-order panel (#119) — manual per-container backup sequence
// ---------------------------------------------------------------------------

// Per-browser: whether the backup-order card is collapsed (#124 — ptmorris1 has
// many containers). Same "bombvault.*" localStorage convention as the other UI prefs.
const BACKUP_ORDER_COLLAPSED_KEY = "bombvault.backupOrderCollapsed";

// BackupOrderPanel lets the user arrange the order scheduled + batch backups run
// in. The orderable set is the installed, schedule-included containers (never
// BombVault itself). It hydrates once from the persisted order (GET
// /api/containers/backup-order), then reconciles as containers come and go
// without discarding an in-progress reorder. Save PUTs the whole displayed
// sequence (authoritative: the list becomes the explicit order); Clear order
// PUTs an empty list, returning every container to the most-overdue-first
// tiebreak.
function BackupOrderPanel({
  containers,
  t,
  hueIndex,
}: {
  containers: Container[];
  t: T;
  /** Rainbow position for THIS panel's own heading notch — GlimStone
   *  follow-up pass (jdp, live review, emphatic, fifth escalation of the
   *  standing colour-engine rule: "Warum muss ich dich immer wieder extra
   *  dran erinnern? Kannst du das jetzt nicht einfach selbst immer
   *  machen?"): this panel's collapsible-header title was still a plain
   *  `<span>`, never routed through Badge's tone="heading"/hueIndex the way
   *  every other static Card heading in the app now is (Dashboard.tsx's
   *  Card(), Config.tsx's Card, Settings.tsx's Card/ToggleRow, VMs.tsx's own
   *  VMBackupOrderPanel — its exact twin, already fixed a commit ago).
   *  Resolved by the caller's own `nextHue()` counter, called DIRECTLY at
   *  the JSX call site (never handed down as a function for this component
   *  to call from its own body — that exact shape is what caused the
   *  SummaryTier regression earlier this session: React doesn't invoke a
   *  child component's body until after the parent's own render pass has
   *  already returned, so a `nextHue` prop called from inside a child lands
   *  strictly after every sibling's own direct call already consumed its
   *  slot). Omit for a genuine singleton — same rule as every other
   *  `hueIndex` call site. */
  hueIndex?: number;
}) {
  const [savedOrder, setSavedOrder] = useState<ContainerOrder[] | null>(null);
  const [names, setNames] = useState<string[]>([]);
  const [saveState, setSaveState] = useState<"idle" | "saving">("idle");
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // whichever button triggered the failed persist() — Save or Reset — kept as
  // two separate nonces, mirroring VMs.tsx's identical VMBackupOrderPanel.
  const [shakeSave, setShakeSave] = useState(0);
  const [shakeReset, setShakeReset] = useState(0);
  const hydrated = useRef(false);
  // #124: collapse the whole card (persisted per browser) and reorder rows by
  // native drag-and-drop (live reorder via the shared useDragReorder hook below).
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem(BACKUP_ORDER_COLLAPSED_KEY) === "1";
    } catch {
      return false;
    }
  });

  useEffect(() => {
    getBackupOrder()
      .then((res) => setSavedOrder(res.ok ? res.order ?? [] : []))
      .catch(() => setSavedOrder([]));
  }, []);

  useEffect(() => {
    if (savedOrder === null) return; // still loading the persisted order
    const orderable = containers
      .filter((c) => c.installed && c.includeInSchedule && !c.self)
      .map((c) => c.name);
    const set = new Set(orderable);
    const byName = (a: string, b: string) =>
      a.localeCompare(b, undefined, { sensitivity: "base" });
    if (!hydrated.current) {
      hydrated.current = true;
      const ranked = savedOrder
        .filter((o) => set.has(o.container))
        .sort((a, b) => a.order - b.order)
        .map((o) => o.container);
      const rest = orderable.filter((n) => !ranked.includes(n)).sort(byName);
      setNames([...ranked, ...rest]);
      return;
    }
    setNames((prev) => {
      const kept = prev.filter((n) => set.has(n));
      const added = orderable.filter((n) => !kept.includes(n)).sort(byName);
      const next = [...kept, ...added];
      return next.length === prev.length && next.every((n, i) => n === prev[i])
        ? prev
        : next;
    });
  }, [containers, savedOrder]);

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

  // Drag-and-drop reorder: lift `from` out and drop it at `to` (arrows do a swap;
  // a drag can jump several rows at once, so this splices instead).
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

  // Live drag-to-reorder: the shared hook calls reorder() as a row is dragged over
  // another, so the rows shift immediately instead of only settling on drop.
  const { dragIndex, rowProps } = useDragReorder<HTMLLIElement>(reorder, saveState === "saving");

  function toggleCollapsed() {
    setCollapsed((v) => {
      const next = !v;
      try {
        localStorage.setItem(BACKUP_ORDER_COLLAPSED_KEY, next ? "1" : "0");
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
      const res = await setBackupOrder(order);
      if (res.ok) {
        setSavedOrder(order.map((container, i) => ({ container, order: i + 1 })));
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
      a.localeCompare(b, undefined, { sensitivity: "base" })
    );
    setNames(sorted);
    void persist([], "reset");
  }

  if (savedOrder === null) return null;

  return (
    // Rainbow-mode completeness sweep (jdp, live review, sixth escalation of
    // this same standing rule on this exact panel: "Es sind nicht alle
    // Buttons in den Regenbogen-Modus eingepflegt"): `.glim-hue` added below
    // — `glim-notch-card` alone only wires the reactive-mode hover reveal on
    // the heading Badge's own notch, it never redefines --accent/
    // --focus-ring, so the "Save" button further down stayed the flat theme
    // accent regardless of rainbow even after the title notch itself was
    // fixed. Same hueIndex prop the Badge already uses.
    //
    // relative + glim-notch-card: same "half-overlap card notch" pattern
    // every other real Card in this app uses (VMs.tsx's own
    // VMBackupOrderPanel is the closest twin — a single div carrying both
    // the visible surface AND the notch's positioned ancestor, no separate
    // outer wrapper needed since this box has no overflow-hidden to clip the
    // badge's own -11px poke above it). glim-notch-card is the hook
    // index.css's card-wide reactive-hover rule keys off, so hovering
    // anywhere on this panel (not just the tiny badge glyph) reveals its hue
    // in reactive rainbow mode.
    //   mt-4 (Task 1, jdp live-review: "Backup-Reihenfolge Card ist zu weit
    // oben, der Abstand nach oben ist zu klein"): the page's own outer
    // `flex flex-col gap-6` already puts a flat, uniform 24px between every
    // top-level section — measured live, byte-identical both above AND below
    // this panel (the controls row's own bottom edge to this div's own CSS
    // box top, and this div's bottom to the next card's top, both exactly
    // 24px). What actually reads as "too little" is this panel's OWN notch
    // badge poking `-translate-y-1/2` ABOVE that box — measured live at 11px
    // — which eats into the gap from the TOP side only (nothing pokes
    // downward below the panel, so its own bottom gap is unaffected): the
    // real visible whitespace between the controls row and the first
    // painted pixel of this card (the badge) measured only 13px, barely
    // half the page's other gaps. `mt-4` adds 16px on top of the existing
    // 24px flex gap (margin and `gap` are independent and stack, they don't
    // collapse into each other), landing the visible gap at ~29px —
    // deliberately a bit MORE than the page's plain 24px rhythm, not just
    // parity with it, matching jdp's own framing ("increase", not merely
    // "restore").
    <div
      className={`relative glim-notch-card bg-carbon-surface rounded-card p-4 mt-4 flex flex-col gap-3${
        hueIndex !== undefined ? " glim-hue" : ""
      }`}
      style={hueIndex !== undefined ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined}
    >
      {/* Title notch, always visible regardless of collapse state (matches
          the PRE-fix behaviour, where title+count stayed visible collapsed
          and only the hint hid) — moved OUT of the disclosure <button>
          below: every real tone="heading" call site in this app keeps the
          Badge as its <h2>'s SOLE child (Dashboard.tsx's Card()/SummaryCell,
          Config.tsx's Card, this file's own notInstalledTitle below,
          StacksPanel above) because size="heading" makes the badge
          `position: absolute` — a flex-row sibling next to it would render
          at the badge's own now-vacated in-flow slot instead of after it.
          The count folds INSIDE the badge's own children instead (Badge's
          span is `inline-flex gap-1`, built to hold more than one child),
          same visual "title (N)" pairing as before, just now inheriting the
          badge's own solid accent-fill/accentContrast ink. */}
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={hueIndex}>
          {t("backupOrder.title")}
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
        aria-label={t("backupOrder.title")}
        className="flex w-full items-start gap-2 text-start"
      >
        <svg
          width="14"
          height="14"
          viewBox="0 0 12 12"
          fill="none"
          aria-hidden="true"
          className={`mt-0.5 shrink-0 text-carbon-textSub transition-transform ${collapsed ? "rtl:rotate-180" : "rotate-90"}`}
        >
          <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
        </svg>
        {!collapsed && (
          <span className="min-w-0 flex-1 text-xs text-carbon-textMuted">{t("backupOrder.hint")}</span>
        )}
      </button>
      {!collapsed &&
        (names.length === 0 ? (
          <p className="text-xs text-carbon-textMuted">{t("backupOrder.empty")}</p>
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
                  {/* Drag grip — a mouse affordance; keyboard users reorder with the
                      arrow buttons below (so the grip is decorative / aria-hidden). */}
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
                  <span className="w-6 text-xs text-carbon-textMuted tabular-nums">
                    {i + 1}.
                  </span>
                  <span className="flex-1 min-w-0 truncate text-sm text-carbon-text">
                    {name}
                  </span>
                  {/* IconTipButton, not plain <button> + `title` (whole-app
                      sweep — VMs.tsx's identical reorder pair converted in
                      the same pass). Both carried an `aria-label` plus a
                      duplicate native `title`, i.e. the OS balloon
                      IconTipButton.tsx exists to replace. Same tips, same
                      handlers, same disabled chrome. */}
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
                key={shakeSave}
                label={t("backupOrder.save")}
                labelKey="backupOrder.save"
                tone="accent"
                onClick={() => void persist(names, "save")}
                disabled={saveState === "saving"}
                busy={saveState === "saving"}
                className={shakeSave ? "glim-shake" : ""}
              />
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
            </div>
          </>
        ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Containers page
// ---------------------------------------------------------------------------

export function Containers() {
  const { t } = useT();
  // One subscription for the whole list rather than one per row: the palette
  // changes for every row at once anyway, so this alone is what makes rainbow
  // on/off/reactive/rotate/palette edits repaint the list live, no reload.
  useRainbow();
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
  const [discovering, setDiscovering] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Discover / "Backup selected" buttons alongside their existing toasts.
  const [shakeDiscover, setShakeDiscover] = useState(0);
  const [shakeBackupSelected, setShakeBackupSelected] = useState(0);
  // Overall server-side batch-backup progress (independent of this browser).
  const progress = useProgress();
  const batch = progress["batch:containers"];
  const batchActive = !!batch?.active;
  // Broader "something is running" signal: any backup/restore/replication in
  // flight (not just this page's batch) disables the start buttons + shows a
  // hint, instead of relying on the 409 round-trip.
  const running = anyActive(progress);

  function loadContainers() {
    return listContainers()
      .then((res) => {
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
      .catch(() => setError(t("containers.loadFailed")));
  }

  useEffect(() => {
    void loadContainers().finally(() => setLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps -- t() is only read to build a failure message; re-fetching on a language switch would be a wasted round-trip

  // Reload when the last operation finishes.
  // ---------------------------------------------------------------------------
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
  const query = search.trim().toLowerCase();
  const filtered = containers.filter((c) => {
    if (query && !(c.name.toLowerCase().includes(query) || c.image.toLowerCase().includes(query)))
      return false;
    if (scheduleFilter === "scheduled" && !c.includeInSchedule) return false;
    if (scheduleFilter === "notScheduled" && c.includeInSchedule) return false;
    if (backupFilter === "backedUp" && c.lastBackup == null) return false;
    if (backupFilter === "neverBackedUp" && c.lastBackup != null) return false;
    return true;
  });

  // Any contained filter off its default narrows the list. The chips persist to
  // localStorage, so a restored non-"all" value would silently shrink the list
  // behind the collapsed "Filters" button — surface it via the trigger's dot.
  const filtersActive =
    query !== "" ||
    filterKey !== "all" ||
    scheduleFilter !== "all" ||
    backupFilter !== "all";

  const sorted = sortContainers(filtered, sortKey);
  const live = sorted.filter((c) => c.installed);
  const orphans = sorted.filter((c) => !c.installed);

  // The FULL installed-container set, unaffected by the search/schedule/
  // backup/installed filters above — StopContainersEditor's own multi-select
  // picker (each ContainerRow below) needs every real candidate to stop, not
  // just whatever this page's own view happens to have filtered down to.
  const installedContainers = containers.filter((c) => c.installed);

  // Precomputed here (against the UNFILTERED containers, matching
  // StacksPanel's own internal groupStacks() call below) so its own
  // emptiness can gate the heading's `nextHue()` call at the JSX render
  // site — StacksPanel returns null internally when there are no
  // multi-member stacks, and hueIndex must never fire for a heading that
  // won't actually render (see this file's own `nextHue()` comment near the
  // return statement for why an ungated call would shift every later
  // heading's rainbow position by one, the exact bug class VMs.tsx's
  // notInstalledTitle fix already caught once this session).
  const stackGroups = groupStacks(containers);

  // Sections the installed toggle actually renders below; when none show but the
  // box has containers, the filters excluded everything → show the no-match hint.
  const liveVisible = filterKey !== "notInstalled" && live.length > 0;
  const orphansVisible = filterKey !== "installed" && orphans.length > 0;
  const noMatch = containers.length > 0 && !liveVisible && !orphansVisible;

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
    void runBulk((name) =>
      fireAndWaitRun({
        kind: "restore",
        matchRun: (r) => r.domain === "container" && r.target === name,
        start: () => restore(name, "latest", true),
        t,
      })
    );
  }

  // GlimStone follow-up pass (v8.0.0): the "+N" / error note never auto-cleared
  // (it stuck around next to the Discover button until the next click); it's a
  // one-shot completion notice like every other migrated action here, so it's
  // now a toast.
  async function handleDiscover() {
    setDiscovering(true);
    try {
      const res = await discover();
      if (res.ok) {
        push(`+${res.discovered ?? 0}`, "success");
        await loadContainers();
      } else {
        push(res.error ?? t("common.discoverFailed"), "fail");
        setShakeDiscover((n) => n + 1);
      }
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
    <div className={PAGE_SHELL}>
      {/* Page heading + Discover (disaster-recovery) action */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">
            {t("containers.title")}
          </h1>
          <p className="mt-1 text-sm text-carbon-textSub">
            {t("containers.subtitle")}
          </p>
          <div className="mt-2"><OffsiteIndicator domain="containers" /></div>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <Button
            key={shakeDiscover}
            label={t("containers.discover")}
            labelKey="containers.discover"
            tone="accent"
            onClick={() => void handleDiscover()}
            disabled={discovering}
            busy={discovering}
            title={discovering ? t("containers.discovering") : tLtr(t, "containers.discoverHint")}
            className={shakeDiscover ? "glim-shake" : ""}
          />
        </div>
      </div>

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
      {!loading && !error && (
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
      {!loading && !error && (
        <StacksPanel
          containers={containers}
          onRestored={() => void loadContainers()}
          t={t}
          hueIndex={stackGroups.length > 0 ? nextHue() : undefined}
        />
      )}

      {/* Controls: search + filter (installed / schedule / backup) + sort.
          Directly above the list it filters — see the backup-order card's own
          comment above for why the two feature cards moved above this row. */}
      {!loading && containers.length > 0 && (
        <div className="flex items-center gap-x-6 gap-y-2 flex-wrap">
          <FilterPopover label={t("filter.button")} active={filtersActive}>
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t("containers.searchPlaceholder")}
              spellCheck={false}
              autoComplete="off"
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 bv-field-focus"
            />
            <FilterControl value={filterKey} onChange={handleFilterChange} t={t} />
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
            {/* Sort lives INSIDE the popover, like VMs.tsx has always had it
                (jdp, live review: "bitte überall gleich machen"). It was the
                one control this page kept outside as a bare sibling of the
                trigger, so the two pages' toolbars disagreed about what the
                "Filter" button contains — and sorting is one of the options
                that menu is for. */}
            <SortControl value={sortKey} onChange={handleSortChange} t={t} />
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
      {!loading && selected.size > 0 && (
        <div className="flex items-center gap-3 flex-wrap rounded-card bg-carbon-surface2 px-3 py-2">
          <span className="text-xs text-carbon-textSub">
            {selected.size} {t("containers.selectedCount")}
          </span>
          <Button
            key={shakeBackupSelected}
        label={t("containers.backupSelected")}
            labelKey="containers.backupSelected"
            tone="accent"
            onClick={() => void backupSelected()}
            disabled={bulkBusy || batchActive || running.active}
            className={`inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
              shakeBackupSelected ? " glim-shake" : ""
            }`}
          />
          {/* Bulk restore is advanced-only; bulk backup stays basic. */}
          <Advanced>
            <Button
              label={t("containers.restoreSelected")}
              labelKey="containers.restoreSelected"
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

      {!loading && filterKey !== "notInstalled" && live.length > 0 && (
        <div className="flex flex-col gap-3 bv-content-fade">
          {live.map((c, i) => (
            <ContainerRow
              key={c.name}
              container={c}
              installedContainers={installedContainers}
              t={t}
              onDeleted={() => void loadContainers()}
              selected={selected.has(c.name)}
              onToggleSelect={c.self ? undefined : () => toggleSelect(c.name)}
              index={i}
            />
          ))}
        </div>
      )}

      {/* Not-installed containers that still have backups. */}
      {!loading && filterKey !== "installed" && orphans.length > 0 && (
        <div className="flex flex-col gap-3 bv-content-fade">
          <div>
            {/* GlimStone follow-up pass ("half-overlap card notch"):
                `relative` directly on this <h2> — same bare-heading case as
                StacksPanel above.
                `hueIndex={nextHue()}` (GlimStone follow-up pass, proactive
                sweep of this same file per the standing colour-engine rule):
                this badge used to be tone="heading"/size="heading" with no
                hueIndex at all — a real gap once BackupOrderPanel's and
                StacksPanel's own notches above can render on the very same
                page, which silently assumed this was the page's only
                heading notch and stayed flat --accent while its siblings
                joined the rainbow (the exact "everything the same colour"
                pattern this rule exists to catch). Threaded through the
                same page-wide `nextHue()` counter, in render order after
                both panels' own calls, so none of the three ever collide on
                the same rainbow position. */}
            <h2 className="relative flex items-center">
              <Badge tone="heading" size="heading" wrap hueIndex={nextHue()}>
                {t("containers.notInstalledTitle")}
              </Badge>
            </h2>
            <p className="mt-1 text-xs text-carbon-textMuted">
              {t("containers.notInstalledHint")}
            </p>
            {/* The operational half the hint above never said ([378]).
                "They still have backups" explains the disk space. It does not
                explain the LOG, and the log is where these are actually
                noticed: measured on jdp's box, three definitions here
                (OpenRGB, QDirStat, MinIO) had produced twelve skipped runs in
                the visible window, one per definition per scheduled run,
                forever, and nothing connected those lines to this page. A skip
                is cheap and correct, so the fix is not to stop skipping - it is
                to say out loud that it will keep happening until somebody
                decides otherwise. */}
            <p className="mt-1 text-xs text-carbon-textMuted">
              {t("containers.notInstalledSkipped")}
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
          {orphans.map((c, i) => (
            <ContainerRow key={c.name} container={c} installedContainers={installedContainers} t={t} onDeleted={() => void loadContainers()} index={live.length + i} />
          ))}
        </div>
      )}

      {/* No container matches the active search / schedule / backup / installed filters. */}
      {!loading && !error && noMatch && (
        <p className="text-sm text-carbon-textMuted">{t("filter.noMatch")}</p>
      )}
      {confirmDialog}
    </div>
  );
}
