import { useEffect, useRef, useState } from "react";
import { listContainers, deleteBackups, backupAll, restore, restoreStack, discover, setContainerHooks, getContainerMounts, setBackupPaths, setStopContainers, setContainerExcludes, previewContainerExcludes, exportContainer, setIncludeAll, ApiError } from "../lib/api";
import type { Container, MountInfo } from "../lib/api";
import { FilterPopover } from "../components/FilterPopover";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { useT, stateLabel } from "../lib/i18n";
import { Advanced } from "../lib/advanced";
import { BackupButton } from "../components/BackupButton";
import { fireAndWaitRun } from "../lib/backupWatch";
import { RestorePanel } from "../components/RestorePanel";
import { RestoreCancelButton } from "../components/RestoreCancelButton";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { IncludeToggle } from "../components/IncludeToggle";
import { ProgressBar } from "../components/ProgressBar";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";

type T = ReturnType<typeof useT>["t"];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

// ---------------------------------------------------------------------------
// State chip
// ---------------------------------------------------------------------------

function StateChip({ state }: { state: string }) {
  const { t } = useT();
  const lower = state.toLowerCase();
  const cls =
    lower === "running"
      ? "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]"
      : lower === "exited" || lower === "stopped"
      ? "bg-[#3a1c1c] text-[#ff8389] border border-[#5a2a2a]"
      : "bg-carbon-surface2 text-carbon-textSub border border-carbon-border";
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${cls}`}>
      {stateLabel(t, state)}
    </span>
  );
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
      {(["name", "status", "ip"] as SortKey[]).map((k) => (
        <button
          key={k}
          onClick={() => onChange(k)}
          className={`rounded-lg px-3 py-1 text-xs font-medium transition-colors ${
            value === k
              ? "bg-accent text-accentContrast"
              : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
          }`}
        >
          {t(SORT_KEYS[k])}
        </button>
      ))}
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
      {(["all", "installed", "notInstalled"] as FilterKey[]).map((k) => (
        <button
          key={k}
          onClick={() => onChange(k)}
          className={`rounded-lg px-3 py-1 text-xs font-medium transition-colors ${
            value === k
              ? "bg-accent text-accentContrast"
              : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
          }`}
        >
          {labels[k]}
        </button>
      ))}
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
      {options.map((o) => (
        <button
          key={o.key}
          onClick={() => onChange(o.key)}
          className={`rounded-lg px-3 py-1 text-xs font-medium transition-colors ${
            value === o.key
              ? "bg-accent text-accentContrast"
              : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
          }`}
        >
          {o.label}
        </button>
      ))}
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
  const [error, setError] = useState<string | null>(null);

  async function handleDelete() {
    if (!window.confirm(t("containers.deleteBackupsConfirm"))) return;
    setPending(true);
    setError(null);
    try {
      const res = await deleteBackups(name);
      if (res.ok) onDeleted();
      else setError(res.error ?? "Delete failed");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Delete failed");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="flex flex-col gap-1">
      <button
        onClick={() => void handleDelete()}
        disabled={pending}
        className="inline-flex items-center gap-2 rounded-lg bg-[#3a1c1c] px-3 py-1.5 text-xs font-medium text-[#ff8389] hover:bg-[#4a2424] transition-colors disabled:opacity-50"
      >
        {pending ? t("dashboard.checking") : t("containers.deleteBackups")}
      </button>
      {error && <p className="text-xs text-[#ff8389]">{error}</p>}
    </div>
  );
}

// ExportButton writes a plain, tool-free tar+xml copy of the container (the same
// folders restic backs up, plus the Unraid template) into a browsable folder next
// to the repo — restic stays the engine; this is an extra, unencrypted export.
function ExportButton({ name, t }: { name: string; t: T }) {
  const [state, setState] = useState<"idle" | "pending" | "done" | "error">("idle");
  const [msg, setMsg] = useState<string | null>(null);

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
        setMsg(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setState("error");
      setMsg(err instanceof Error ? err.message : t("settings.error"));
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <button
        onClick={() => void run()}
        disabled={state === "pending"}
        className="inline-flex items-center gap-1.5 rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors disabled:opacity-50"
      >
        {state === "pending" ? "…" : t("export.button")}
      </button>
      {state === "done" && msg && (
        <span className="text-xs text-[#6fdc8c] break-all text-right">{t("export.exportedTo")} {msg}</span>
      )}
      {state === "error" && msg && (
        <span className="text-xs text-[#ff8389] break-words text-right">{msg}</span>
      )}
    </div>
  );
}

// HooksEditor edits the per-container pre/post-backup commands (collapsible).
function HooksEditor({
  name,
  initialPre,
  initialPost,
  t,
}: {
  name: string;
  initialPre: string;
  initialPost: string;
  t: T;
}) {
  const [open, setOpen] = useState(false);
  const [pre, setPre] = useState(initialPre);
  const [post, setPost] = useState(initialPost);
  const [state, setState] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [msg, setMsg] = useState<string | null>(null);

  async function save() {
    setState("saving");
    setMsg(null);
    try {
      const r = await setContainerHooks(name, pre, post);
      if (r.ok) {
        setState("saved");
        setTimeout(() => setState("idle"), 2500);
      } else {
        setState("error");
        setMsg(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setState("error");
      setMsg(err instanceof Error ? err.message : t("settings.error"));
    }
  }

  const inputCls =
    "rounded bg-carbon-surface2 border border-carbon-border text-carbon-text text-xs font-mono px-2 py-1 focus:outline-none focus:border-[#78a9ff]";

  return (
    <div className="mt-1">
      <button
        onClick={() => setOpen((p) => !p)}
        className="flex items-center gap-1.5 text-xs text-carbon-textSub hover:text-carbon-text transition-colors"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" className={`transition-transform ${open ? "rotate-90" : ""}`}>
          <path d="M4 2l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        {t("hooks.title")}
        {(initialPre || initialPost) && <span className="text-[#6fdc8c]">●</span>}
      </button>
      {open && (
        <div className="mt-2 rounded-lg border border-carbon-border bg-carbon-background p-3 flex flex-col gap-2">
          <p className="text-xs text-carbon-textMuted">{t("hooks.hint")}</p>
          <label className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("hooks.pre")}</span>
            <input value={pre} onChange={(e) => setPre(e.target.value)} spellCheck={false}
              placeholder="mysqldump -uroot -p$PW db > /config/dump.sql" className={inputCls} />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("hooks.post")}</span>
            <input value={post} onChange={(e) => setPost(e.target.value)} spellCheck={false}
              placeholder="curl -fsS https://hooks.example/done" className={inputCls} />
          </label>
          <div className="flex items-center gap-3 pt-0.5">
            <button onClick={() => void save()} disabled={state === "saving"}
              className="rounded-lg bg-accent px-3 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50">
              {state === "saving" ? "…" : t("settings.save")}
            </button>
            {state === "saved" && <span className="text-xs text-[#6fdc8c]">{t("settings.saved")}</span>}
            {state === "error" && msg && <span className="text-xs text-[#ff8389] break-words">{msg}</span>}
          </div>
        </div>
      )}
    </div>
  );
}

// FoldersEditor lets the user choose which of a container's mapped folders get
// backed up (appdata is the default), plus add custom paths under the host
// mount. Collapsible; loads the mount list lazily on first open.
function FoldersEditor({ name, t }: { name: string; t: T }) {
  const [open, setOpen] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(false);
  const [mounts, setMounts] = useState<MountInfo[]>([]);
  const [checked, setChecked] = useState<Set<string>>(new Set());
  const [custom, setCustom] = useState<string[]>([]);
  const [customInput, setCustomInput] = useState("");
  const [state, setState] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [msg, setMsg] = useState<string | null>(null);

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
        } else {
          setState("error");
          setMsg(r.error ?? t("settings.error"));
        }
      })
      .catch((err) => {
        setState("error");
        setMsg(err instanceof Error ? err.message : t("settings.error"));
      })
      .finally(() => {
        setLoading(false);
        setLoaded(true);
      });
  }, [open, loaded, name, t]);

  function toggle(source: string) {
    setChecked((prev) => {
      const next = new Set(prev);
      if (next.has(source)) next.delete(source);
      else next.add(source);
      return next;
    });
  }
  function addCustom() {
    const p = customInput.trim();
    if (p && !custom.includes(p)) setCustom((c) => [...c, p]);
    setCustomInput("");
  }
  async function save() {
    setState("saving");
    setMsg(null);
    const paths = [...checked, ...custom];
    try {
      const r = await setBackupPaths(name, paths);
      if (r.ok) {
        setState("saved");
        setTimeout(() => setState("idle"), 2500);
      } else {
        setState("error");
        setMsg(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setState("error");
      setMsg(err instanceof Error ? err.message : t("settings.error"));
    }
  }

  const inputCls =
    "rounded bg-carbon-surface2 border border-carbon-border text-carbon-text text-xs font-mono px-2 py-1 focus:outline-none focus:border-[#78a9ff]";

  return (
    <div className="mt-1">
      <button
        onClick={() => setOpen((p) => !p)}
        className="flex items-center gap-1.5 text-xs text-carbon-textSub hover:text-carbon-text transition-colors"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" className={`transition-transform ${open ? "rotate-90" : ""}`}>
          <path d="M4 2l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        {t("folders.title")}
      </button>
      {open && (
        <div className="mt-2 rounded-lg border border-carbon-border bg-carbon-background p-3 flex flex-col gap-2">
          <p className="text-xs text-carbon-textMuted">{t("folders.hint")}</p>
          {loading && <p className="text-xs text-carbon-textMuted">{t("common.loadingBackups")}</p>}
          {!loading && mounts.length === 0 && custom.length === 0 && (
            <p className="text-xs text-carbon-textMuted">{t("folders.empty")}</p>
          )}
          {mounts.map((m) => (
            <label
              key={m.source}
              className={`flex items-start gap-2 text-xs ${m.reachable ? "text-carbon-text" : "text-carbon-textMuted"}`}
            >
              <input
                type="checkbox"
                disabled={!m.reachable}
                checked={m.reachable && checked.has(m.source)}
                onChange={() => toggle(m.source)}
                className="mt-0.5 accent-[var(--accent)]"
              />
              <span className="flex flex-col">
                <span className="font-mono break-all">{m.dest} ← {m.source}</span>
                {m.isAppdata && <span className="text-[#6fdc8c]">{t("folders.appdataDefault")}</span>}
                {!m.reachable && <span className="text-[#ff8389]">{t("folders.notReachable")}</span>}
              </span>
            </label>
          ))}
          {custom.map((p) => (
            <div key={p} className="flex items-center gap-2 text-xs text-carbon-text">
              <input type="checkbox" checked readOnly className="accent-[var(--accent)]" />
              <span className="font-mono break-all flex-1">{p}</span>
              <button
                onClick={() => setCustom((c) => c.filter((x) => x !== p))}
                className="text-carbon-textMuted hover:text-[#ff8389] px-1"
                aria-label="remove"
              >
                ×
              </button>
            </div>
          ))}
          <div className="flex items-center gap-2 pt-1">
            <input
              value={customInput}
              onChange={(e) => setCustomInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  addCustom();
                }
              }}
              spellCheck={false}
              placeholder={t("folders.customPlaceholder")}
              className={`${inputCls} flex-1`}
            />
            <button
              onClick={addCustom}
              className="rounded-lg bg-carbon-surface2 border border-carbon-border px-3 py-1 text-xs text-carbon-text hover:bg-carbon-hover transition-colors"
            >
              {t("folders.add")}
            </button>
          </div>
          <div className="flex items-center gap-3 pt-0.5">
            <button
              onClick={() => void save()}
              disabled={state === "saving" || loading}
              className="rounded-lg bg-accent px-3 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {state === "saving" ? "…" : t("folders.save")}
            </button>
            {state === "saved" && <span className="text-xs text-[#6fdc8c]">{t("folders.saved")}</span>}
            {state === "error" && msg && <span className="text-xs text-[#ff8389] break-words">{msg}</span>}
          </div>
        </div>
      )}
    </div>
  );
}

// StopContainersEditor edits the list of OTHER containers to stop during this
// container's backup (e.g. a database), one name per line. Collapsible.
function StopContainersEditor({ name, initial, t }: { name: string; initial: string[]; t: T }) {
  const [open, setOpen] = useState(false);
  const [text, setText] = useState(initial.join("\n"));
  const [state, setState] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [msg, setMsg] = useState<string | null>(null);

  async function save() {
    setState("saving");
    setMsg(null);
    const list = text
      .split("\n")
      .map((s) => s.trim())
      .filter(Boolean);
    try {
      const r = await setStopContainers(name, list);
      if (r.ok) {
        setState("saved");
        setTimeout(() => setState("idle"), 2500);
      } else {
        setState("error");
        setMsg(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setState("error");
      setMsg(err instanceof Error ? err.message : t("settings.error"));
    }
  }

  const inputCls =
    "rounded bg-carbon-surface2 border border-carbon-border text-carbon-text text-xs font-mono px-2 py-1 focus:outline-none focus:border-[#78a9ff]";

  return (
    <div className="mt-1">
      <button
        onClick={() => setOpen((p) => !p)}
        className="flex items-center gap-1.5 text-xs text-carbon-textSub hover:text-carbon-text transition-colors"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" className={`transition-transform ${open ? "rotate-90" : ""}`}>
          <path d="M4 2l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        {t("stophook.title")}
        {initial.length > 0 && <span className="text-[#6fdc8c]">●</span>}
      </button>
      {open && (
        <div className="mt-2 rounded-lg border border-carbon-border bg-carbon-background p-3 flex flex-col gap-2">
          <p className="text-xs text-carbon-textMuted">{t("stophook.hint")}</p>
          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            spellCheck={false}
            rows={3}
            placeholder={"mariadb\nredis"}
            className={inputCls}
          />
          <div className="flex items-center gap-3 pt-0.5">
            <button
              onClick={() => void save()}
              disabled={state === "saving"}
              className="rounded-lg bg-accent px-3 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {state === "saving" ? "…" : t("settings.save")}
            </button>
            {state === "saved" && <span className="text-xs text-[#6fdc8c]">{t("settings.saved")}</span>}
            {state === "error" && msg && <span className="text-xs text-[#ff8389] break-words">{msg}</span>}
          </div>
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

function ExcludesEditor({ name, initial, t }: { name: string; initial: string[]; t: T }) {
  const [open, setOpen] = useState(false);
  const [text, setText] = useState(initial.join("\n"));
  const [state, setState] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [msg, setMsg] = useState<string | null>(null);
  const [preview, setPreview] = useState<ExcludePreviewRow[]>([]);

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

  async function save() {
    setState("saving");
    setMsg(null);
    const list = text
      .split("\n")
      .map((s) => s.trim())
      .filter(Boolean);
    try {
      const r = await setContainerExcludes(name, list);
      if (r.ok) {
        setState("saved");
        setTimeout(() => setState("idle"), 2500);
      } else {
        setState("error");
        setMsg(r.error ?? t("excludes.error"));
      }
    } catch (err) {
      setState("error");
      setMsg(err instanceof Error ? err.message : t("excludes.error"));
    }
  }

  const inputCls =
    "rounded bg-carbon-surface2 border border-carbon-border text-carbon-text text-xs font-mono px-2 py-1 focus:outline-none focus:border-[#78a9ff]";

  return (
    <div className="mt-1">
      <button
        onClick={() => setOpen((p) => !p)}
        className="flex items-center gap-1.5 text-xs text-carbon-textSub hover:text-carbon-text transition-colors"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" className={`transition-transform ${open ? "rotate-90" : ""}`}>
          <path d="M4 2l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        {t("excludes.title")}
        {initial.length > 0 && <span className="text-[#6fdc8c]">●</span>}
      </button>
      {open && (
        <div className="mt-2 rounded-lg border border-carbon-border bg-carbon-background p-3 flex flex-col gap-2">
          <p className="text-xs text-carbon-textMuted">{t("excludes.hint")}</p>
          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            spellCheck={false}
            rows={3}
            placeholder={t("excludes.placeholder")}
            className={inputCls}
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
                    className="text-xs break-words leading-snug flex items-baseline gap-1.5"
                    title={row.status === "translated" ? row.resolved : undefined}
                  >
                    <span className="font-mono text-carbon-textSub">{row.raw}</span>
                    <span className={good ? "text-[#6fdc8c]" : "text-[#ff8389]"}>
                      {good ? "✓" : "⚠"} {msg}
                    </span>
                  </div>
                );
              })}
            </div>
          )}
          <div className="flex items-center gap-3 pt-0.5">
            <button
              onClick={() => void save()}
              disabled={state === "saving"}
              className="rounded-lg bg-accent px-3 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {state === "saving" ? "…" : t("excludes.save")}
            </button>
            {state === "saved" && <span className="text-xs text-[#6fdc8c]">{t("excludes.saved")}</span>}
            {state === "error" && msg && <span className="text-xs text-[#ff8389] break-words">{msg}</span>}
          </div>
        </div>
      )}
    </div>
  );
}

function ContainerRow({
  container,
  t,
  onDeleted,
  selected,
  onToggleSelect,
}: {
  container: Container;
  t: T;
  onDeleted: () => void;
  selected?: boolean;
  onToggleSelect?: () => void;
}) {
  const installed = container.installed;
  const progressMap = useProgress();
  const progress = progressMap[`container:${container.name}`];
  // "Something is running" across any domain — used to busy-guard this row's
  // own backup button (its OWN in-flight backup is handled by isPending inside).
  const running = anyActive(progressMap);
  return (
    <div className="relative overflow-hidden bg-carbon-surface rounded-card border border-carbon-border p-4 flex flex-col gap-3">
      {/* Top row */}
      <div className="flex items-start gap-3 flex-wrap">
        {/* Multi-select checkbox (installed containers only) */}
        {onToggleSelect && (
          <input
            type="checkbox"
            checked={!!selected}
            onChange={onToggleSelect}
            aria-label={`Select ${container.name}`}
            className="mt-1 h-4 w-4 shrink-0 cursor-pointer"
            style={{ accentColor: "var(--accent)" }}
          />
        )}
        {/* Name + image */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm truncate">
              {container.name}
            </span>
            {installed ? (
              <StateChip state={container.state} />
            ) : (
              <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-carbon-surface2 text-carbon-textSub border border-carbon-border">
                {t("containers.notInstalled")}
              </span>
            )}
            {container.ip && (
              <span className="text-xs text-carbon-textMuted font-mono">{container.ip}</span>
            )}
          </div>
          {container.image && (
            <p className="text-xs text-carbon-textMuted mt-0.5 truncate">{container.image}</p>
          )}
        </div>

        {/* Last backup */}
        <div className="text-right shrink-0">
          <p className="text-xs text-carbon-textMuted">{t("containers.lastBackup")}</p>
          <p className="text-xs text-carbon-textSub">
            {container.lastBackup ? formatTs(container.lastBackup) : t("containers.never")}
          </p>
        </div>
      </div>

      {/* Actions row — include toggle on the left, backup button on the right. */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        {installed ? (
          <>
            {/* Include toggle */}
            <label className="flex items-center gap-2 cursor-pointer">
              <IncludeToggle name={container.name} initial={container.includeInSchedule} />
              <span className="text-xs text-carbon-textSub">
                {t("containers.includeInSchedule")}
              </span>
            </label>

            {/* Backup + plain export (right) — backup refreshes the list so "last backup" updates.
                BombVault's own container has no backup action: backing it up would stop itself. */}
            <div className="ml-auto flex flex-col items-end gap-2">
              {container.self ? (
                <span className="text-xs text-carbon-textMuted max-w-[18rem] text-right">
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
          </>
        ) : (
          /* Not installed: can't back up; offer delete-all-backups instead. */
          <DeleteBackupsButton name={container.name} t={t} onDeleted={onDeleted} />
        )}
      </div>

      {/* Backup-folder selection + stop-other-containers + pre/post hooks
          (installed only). These expert editors are advanced-only. */}
      <Advanced when={installed}>
        <FoldersEditor name={container.name} t={t} />
        <StopContainersEditor name={container.name} initial={container.stopContainers ?? []} t={t} />
        <ExcludesEditor name={container.name} initial={container.excludes ?? []} t={t} />
        <HooksEditor
          name={container.name}
          initialPre={container.preHook}
          initialPost={container.postHook}
          t={t}
        />
      </Advanced>

      {/* Backups / Restore disclosure (works even when not installed) */}
      <RestorePanel name={container.name} t={t} installed={installed} />

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
  const [error, setError] = useState<string | null>(null);

  async function run(include: boolean) {
    setBusy(true);
    setError(null);
    try {
      const res = await setIncludeAll(include);
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
        className="inline-flex items-center rounded-lg bg-accent px-3 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
      >
        {t("schedule.includeAll")}
      </button>
      <button
        onClick={() => void run(false)}
        disabled={busy}
        className="inline-flex items-center rounded-lg bg-carbon-surface2 px-3 py-1 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50"
      >
        {t("schedule.excludeAll")}
      </button>
      {error && <span className="text-xs text-[#ff8389]">{error}</span>}
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
function StackCard({ group, onRestored, t }: { group: StackGroup; onRestored: () => void; t: T }) {
  const [open, setOpen] = useState(false);
  const [source, setSource] = useState<RepoSource>("local");
  const [startInOrder, setStartInOrder] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
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
    if (!window.confirm(t("stack.restoreConfirm"))) return;
    setBusy(true);
    setError(null);
    setStarted(false);
    setFinished(false);
    sawActive.current = false;
    try {
      const res = await restoreStack(group.project, startInOrder, true, source);
      if (res.ok) {
        setStarted(true);
        onRestored(); // refresh the main list so run-state/orphan rows update
      } else {
        setError(res.error ?? t("settings.error"));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("settings.error"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="bg-carbon-surface rounded-card border border-carbon-border p-4 flex flex-col gap-2">
      <div className="flex items-start justify-between gap-3 flex-wrap">
        <div className="min-w-0">
          <span className="font-semibold text-carbon-text text-sm">{group.project}</span>
          <span className="ml-2 text-xs text-carbon-textMuted">
            {t("stack.members").replace("{n}", String(group.members.length))}
          </span>
          <p className="mt-0.5 text-[11px] text-carbon-textMuted truncate">
            {group.members.map((m) => m.name).join(", ")}
          </p>
        </div>
        {/* Disclosure toggle (icon only) so the sole "Restore stack" label is the
            action button inside the panel. */}
        <button
          onClick={() => setOpen((p) => !p)}
          aria-expanded={open}
          aria-label={t("stack.restore")}
          title={t("stack.restore")}
          className="shrink-0 inline-flex items-center rounded-lg border border-carbon-border p-1.5 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors"
        >
          <svg width="14" height="14" viewBox="0 0 12 12" fill="none" className={`transition-transform ${open ? "rotate-90" : ""}`}>
            <path d="M4 2l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </button>
      </div>

      {open && (
        <div className="mt-1 rounded-lg border border-carbon-border bg-carbon-background p-3 flex flex-col gap-2">
          <p className="text-xs text-carbon-textMuted">{t("stack.restoreHint")}</p>
          <div className="flex items-center gap-2">
            <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
            <SourceToggle source={source} onChange={setSource} disabled={busy} />
          </div>
          <label className="flex items-center gap-2 text-xs text-carbon-textSub cursor-pointer">
            <input
              type="checkbox"
              checked={startInOrder}
              onChange={(e) => setStartInOrder(e.target.checked)}
              className="h-4 w-4 cursor-pointer"
              style={{ accentColor: "var(--accent)" }}
            />
            {t("stack.startInOrder")}
          </label>
          <div className="flex items-center gap-3 pt-0.5">
            <button
              onClick={() => void run()}
              disabled={busy}
              className="rounded-lg bg-accent px-3 py-1 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {busy ? t("stack.restoring") : t("stack.restore")}
            </button>
            {error && <span className="text-xs text-[#ff8389] break-words">{error}</span>}
          </div>

          {/* Async ack: the server runs the stack restore detached and the ack
              carries no member results — per-member outcomes are in the run
              history. The cancel button stays up for the whole restoring window
              (no per-member flicker); once every member goes inactive the panel
              flips to a neutral "finished" note (see the terminal effect above). */}
          {started && !busy && (
            <div className="flex flex-col gap-1">
              <p className="text-xs text-carbon-textSub">{t("restore.started")}</p>
              <p className="text-[11px] text-carbon-textMuted">{t("restore.bgHint")}</p>
              {/* Whole-stack in-place restore — hard warning, keyed to the stack. */}
              <RestoreCancelButton cancelKey={`stack:${group.project}`} inPlace name={group.project} t={t} />
            </div>
          )}
          {finished && !busy && (
            <p className="text-xs text-carbon-textSub">{t("stack.restoreFinished")}</p>
          )}
        </div>
      )}
    </div>
  );
}

// StacksPanel renders one card per detected compose stack, above the container
// list. It renders nothing when no multi-member stack is present.
function StacksPanel({ containers, onRestored, t }: { containers: Container[]; onRestored: () => void; t: T }) {
  const stacks = groupStacks(containers);
  if (stacks.length === 0) return null;
  return (
    <div className="flex flex-col gap-3">
      <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
        {t("stack.title")}
      </h2>
      {stacks.map((g) => (
        <StackCard key={g.project} group={g} onRestored={onRestored} t={t} />
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Containers page
// ---------------------------------------------------------------------------

export function Containers() {
  const { t } = useT();
  const [containers, setContainers] = useState<Container[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [sortKey, setSortKey] = useState<SortKey>(loadSortKey);
  const [filterKey, setFilterKey] = useState<FilterKey>(loadFilterKey);
  const [search, setSearch] = useState("");
  const [scheduleFilter, setScheduleFilter] = useState<ScheduleFilterKey>(loadScheduleFilterKey);
  const [backupFilter, setBackupFilter] = useState<BackupFilterKey>(loadBackupFilterKey);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [bulkBusy, setBulkBusy] = useState(false);
  const [bulkMsg, setBulkMsg] = useState<string | null>(null);
  const [discovering, setDiscovering] = useState(false);
  const [discoverMsg, setDiscoverMsg] = useState<string | null>(null);
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
        if (res.ok) setContainers(res.containers ?? []);
        else setError("Failed to load containers");
      })
      .catch(() => setError("Failed to load containers"));
  }

  useEffect(() => {
    void loadContainers().finally(() => setLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

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
  useEffect(() => {
    setSelected((prev) => {
      if (prev.size === 0) return prev;
      const visible = new Set(selectable.map((c) => c.name));
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
    setBulkMsg(t("containers.bulkResult").replace("{ok}", String(ok)).replace("{fail}", String(fail)));
    setSelected(new Set());
    void loadContainers();
  }

  // Back up the selected containers SERVER-SIDE: one request kicks off a batch
  // that runs on the server, so it survives this browser going away (closing the
  // tab, or stopping the container the UI runs in). Progress comes over SSE.
  async function backupSelected() {
    if (bulkBusy) return; // guard the in-flight window (button also disables)
    setBulkBusy(true);
    setBulkMsg(null);
    const names = [...selected];
    try {
      const res = await backupAll(names);
      if (!res.ok) {
        setBulkMsg(res.error ?? "Failed to start backup");
        return;
      }
      setSelected(new Set());
      setBulkMsg(t("containers.batchStarted"));
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setBulkMsg(t("containers.batchAlreadyRunning"));
      } else {
        setBulkMsg(e instanceof Error ? e.message : "Failed to start backup");
      }
    } finally {
      setBulkBusy(false);
    }
  }

  // Restores are ASYNC and share the server's single-flight guard, so the bulk
  // loop must fire one restore and WAIT for its recorded run before the next
  // (firing them in a tight loop would make every call after the first hit
  // "already running"). fireAndWaitRun handles the fire/retry/wait cycle.
  function restoreSelected() {
    if (!window.confirm(t("containers.restoreSelectedConfirm"))) return;
    void runBulk((name) =>
      fireAndWaitRun({
        kind: "restore",
        matchRun: (r) => r.domain === "container" && r.target === name,
        start: () => restore(name, "latest", true),
      })
    );
  }

  async function handleDiscover() {
    setDiscovering(true);
    setDiscoverMsg(null);
    try {
      const res = await discover();
      if (res.ok) {
        setDiscoverMsg(`+${res.discovered ?? 0}`);
        await loadContainers();
      } else {
        setDiscoverMsg(res.error ?? "Discover failed");
      }
    } catch (err) {
      setDiscoverMsg(err instanceof Error ? err.message : "Discover failed");
    } finally {
      setDiscovering(false);
    }
  }

  return (
    <div className="flex flex-col gap-6 max-w-5xl">
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
          {discoverMsg && (
            <span className="text-xs text-carbon-textSub">{discoverMsg}</span>
          )}
          <button
            onClick={() => void handleDiscover()}
            disabled={discovering}
            title={t("containers.discoverHint")}
            className="inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {discovering ? t("containers.discovering") : t("containers.discover")}
          </button>
        </div>
      </div>

      {/* Server-side batch-backup banner — visible while a "back up all" run is in
          flight, even if it was started from another tab/session. */}
      {batchActive && (
        <div className="flex items-center gap-3 rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-2">
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
        <p className="text-sm text-[#ff8389]">{error}</p>
      )}
      {!loading && !error && containers.length === 0 && (
        <div className="bg-carbon-surface rounded-card border border-carbon-border p-6 text-center">
          <p className="text-sm text-carbon-textMuted">
            {t("containers.emptyDocker")}
          </p>
        </div>
      )}
      {/* Controls: search + filter (installed / schedule / backup) + sort. */}
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
              className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-3 py-1.5 focus:outline-none focus:border-[#78a9ff]"
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
          </FilterPopover>
          <SortControl value={sortKey} onChange={handleSortChange} t={t} />
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
            <div className="ml-auto">
              <ScheduleIncludeAllControl t={t} onChanged={() => void loadContainers()} />
            </div>
          )}
        </div>
      )}

      {/* Bulk action bar — appears when one or more containers are selected. */}
      {!loading && selected.size > 0 && (
        <div className="flex items-center gap-3 flex-wrap rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-2">
          <span className="text-xs text-carbon-textSub">
            {selected.size} {t("containers.selectedCount")}
          </span>
          <button
            onClick={() => void backupSelected()}
            disabled={bulkBusy || batchActive || running.active}
            className="inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {t("containers.backupSelected")}
          </button>
          {/* Bulk restore is advanced-only; bulk backup stays basic. */}
          <Advanced>
            <button
              onClick={restoreSelected}
              disabled={bulkBusy || running.active}
              className="inline-flex items-center rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {t("containers.restoreSelected")}
            </button>
          </Advanced>
          <button
            onClick={() => setSelected(new Set())}
            disabled={bulkBusy}
            className="text-xs text-carbon-textMuted hover:text-carbon-text transition-colors disabled:opacity-50"
          >
            {t("containers.clearSelection")}
          </button>
          {running.active && (
            <span className="text-xs text-carbon-textMuted">
              {t(busyPhraseKey(running.phase))}
            </span>
          )}
        </div>
      )}

      {/* Bulk status — kept OUTSIDE the action bar so it stays visible after a
          server-side backup clears the selection (the bar unmounts then). */}
      {(bulkBusy || bulkMsg) && (
        <p className="text-xs text-carbon-textSub">
          {bulkBusy ? t("containers.working") : bulkMsg}
        </p>
      )}

      {/* Stacks panel — one card per detected compose stack, above the list. */}
      {!loading && !error && <StacksPanel containers={containers} onRestored={() => void loadContainers()} t={t} />}

      {!loading && filterKey !== "notInstalled" && live.length > 0 && (
        <div className="flex flex-col gap-3">
          {live.map((c) => (
            <ContainerRow
              key={c.name}
              container={c}
              t={t}
              onDeleted={() => void loadContainers()}
              selected={selected.has(c.name)}
              onToggleSelect={c.self ? undefined : () => toggleSelect(c.name)}
            />
          ))}
        </div>
      )}

      {/* Not-installed containers that still have backups. */}
      {!loading && filterKey !== "installed" && orphans.length > 0 && (
        <div className="flex flex-col gap-3">
          <div>
            <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
              {t("containers.notInstalledTitle")}
            </h2>
            <p className="mt-1 text-xs text-carbon-textMuted">
              {t("containers.notInstalledHint")}
            </p>
          </div>
          {orphans.map((c) => (
            <ContainerRow key={c.name} container={c} t={t} onDeleted={() => void loadContainers()} />
          ))}
        </div>
      )}

      {/* No container matches the active search / schedule / backup / installed filters. */}
      {!loading && !error && noMatch && (
        <p className="text-sm text-carbon-textMuted">{t("filter.noMatch")}</p>
      )}
    </div>
  );
}
