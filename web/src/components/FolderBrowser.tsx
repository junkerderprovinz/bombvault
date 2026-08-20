import { useState, useCallback } from "react";
import { browse, createFolder } from "../lib/api";
import { useT } from "../lib/i18n";

// ---------------------------------------------------------------------------
// Folder browser (shared)
// ---------------------------------------------------------------------------

export interface FolderBrowserProps {
  label: string;
  value: string;
  hostMountRoot: string;
  onChange: (v: string) => void;
  // Example path shown when the field is empty. Defaults to the conventional
  // appdata share ("user/appdata" -> /mnt/user/appdata, which exists on every
  // Unraid box). Pass a field-specific example (e.g. "user/domains" for VM
  // disks) so the greyed hint matches what a blank field actually defaults to.
  // The old hardcoded "user/bombvault/container" example named a path that does
  // not exist and read as a real chosen default (#125).
  placeholder?: string;
}

export function FolderBrowser({ label, value, hostMountRoot, onChange, placeholder }: FolderBrowserProps) {
  const { t } = useT();
  // browsePath tracks the *current directory being listed* (not the selected value).
  // We initialise it to the current value so opening the browser starts in the right folder.
  const [open, setOpen] = useState(false);
  const [browsePath, setBrowsePath] = useState(value);
  const [dirs, setDirs] = useState<{ name: string; path: string }[]>([]);
  const [browseError, setBrowseError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [manualFallback, setManualFallback] = useState(false);
  // New-folder creation inside the current browsePath.
  const [newName, setNewName] = useState("");
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const doFetch = useCallback((path: string) => {
    setLoading(true);
    setBrowseError(null);
    browse(path)
      .then((res) => {
        if (!res.ok) {
          setBrowseError(res.error ?? t("folder.couldNotRead"));
          setManualFallback(true);
          return;
        }
        setDirs(res.dirs ?? []);
        setBrowsePath(path);
      })
      .catch((err: unknown) => {
        const msg = err instanceof Error ? err.message : t("folder.browseFailed");
        setBrowseError(msg);
        setManualFallback(true);
      })
      .finally(() => setLoading(false));
  }, [t]);

  function handleOpen() {
    setManualFallback(false);
    setOpen(true);
    doFetch(value);
  }

  function handleClose() {
    setOpen(false);
    setBrowseError(null);
  }

  function handleUp() {
    const parts = browsePath.split("/").filter(Boolean);
    parts.pop();
    doFetch(parts.join("/"));
  }

  function handleSelect() {
    onChange(browsePath);
    setOpen(false);
  }

  function handleCreate() {
    const name = newName.trim();
    if (!name || creating) return;
    setCreating(true);
    setCreateError(null);
    createFolder(browsePath, name)
      .then((res) => {
        if (!res.ok) {
          setCreateError(res.error ?? t("folder.createFailed"));
          return;
        }
        setNewName("");
        // Navigate into the freshly created folder so "use this folder" selects it.
        doFetch(res.path ?? browsePath);
      })
      .catch((err: unknown) => {
        setCreateError(err instanceof Error ? err.message : t("folder.createFailed"));
      })
      .finally(() => setCreating(false));
  }

  const trimmed = value.trim();
  const resolved =
    trimmed && !trimmed.startsWith("/") && !trimmed.includes("..")
      ? `${hostMountRoot}/${trimmed}`
      : "";

  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-xs text-carbon-textSub">{label}</label>

      {/* Current value + browser trigger */}
      <div className="flex items-center gap-2">
        <input
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          spellCheck={false}
          placeholder={placeholder ?? "user/appdata"}
          dir="ltr"
          className="flex-1 rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus text-start"
        />
        <button
          onClick={handleOpen}
          title={t("folder.browseTitle")}
          className="shrink-0 rounded-control bg-carbon-surface3 px-3 py-1.5 text-xs text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors"
        >
          {t("folder.browse")}
        </button>
      </div>

      {/* Absolute path preview */}
      {resolved && (
        <p dir="ltr" className="text-xs text-carbon-textMuted font-mono break-all text-start">→ {resolved}</p>
      )}
      {!resolved && trimmed && (
        <p className="text-xs text-statusFail">
          {t("folder.pathHint")}
        </p>
      )}

      {/* Browser panel */}
      {open && (
        <div className="mt-1 rounded-card bg-carbon-background p-3 flex flex-col gap-2">
          {/* Header: current path + close */}
          <div className="flex items-center justify-between gap-2">
            <span dir="ltr" className="text-xs font-mono text-carbon-textSub min-w-0 truncate text-start">
              {hostMountRoot}/{browsePath || ""}
            </span>
            <button
              onClick={handleClose}
              className="text-xs text-carbon-textMuted hover:text-carbon-text shrink-0"
            >
              ✕
            </button>
          </div>

          {/* Error state with manual fallback */}
          {browseError && (
            <p className="text-xs text-statusFail">{browseError}</p>
          )}

          {/* Loading spinner — Task 7: was border-statusInfoSolid (the old
              fifth hue). Genuine activity (the directory listing IS being
              fetched right now), matching OffsiteIndicator.tsx's own plain
              border-color: var(--accent) spinner for the identical
              not-inside-a-button case. border-accentText, not the flat
              border-accent: a spec-compliance review measured the flat
              accent gold at 1.61:1 in light theme here — badly under SC
              1.4.11's 3:1 non-text-indicator minimum. This wasn't a fresh
              regression from this task (OffsiteIndicator.tsx's own spinner,
              the precedent this matched, had the identical unverified gap
              already) — fixed here anyway since it's the same root cause;
              see index.css's --accent-text comment for the fix. That comment
              also carries the full inventory of the four still-unmigrated
              flat --accent sites, OffsiteIndicator's spinner among them, so
              the two spinners have now diverged on purpose. */}
          {loading && (
            <div className="flex items-center gap-2 text-xs text-carbon-textMuted">
              <span className="h-3 w-3 rounded-full border-2 border-accentText border-t-transparent animate-spin" />
              {t("folder.loading")}
            </div>
          )}

          {/* Directory listing */}
          {!loading && !manualFallback && (
            <div className="flex flex-col gap-0.5 max-h-48 overflow-y-auto">
              {/* ".." go up — only when not at root */}
              {browsePath !== "" && (
                <button
                  onClick={handleUp}
                  dir="ltr"
                  className="text-start text-xs font-mono text-carbon-textSub px-2 py-1 rounded-control hover:bg-carbon-hover hover:text-carbon-text transition-colors"
                >
                  ..
                </button>
              )}
              {dirs.length === 0 && !browseError && (
                <p className="text-xs text-carbon-textMuted px-2">{t("folder.none")}</p>
              )}
              {dirs.map((d) => (
                <button
                  key={d.path}
                  onClick={() => doFetch(d.path)}
                  dir="ltr"
                  className="text-start text-xs font-mono text-carbon-textSub px-2 py-1 rounded-control hover:bg-carbon-hover hover:text-carbon-text transition-colors"
                >
                  {d.name}/
                </button>
              ))}
            </div>
          )}

          {/* Create a new folder inside the current directory */}
          {!manualFallback && (
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    handleCreate();
                  }
                }}
                spellCheck={false}
                placeholder={t("folder.newFolderPlaceholder")}
                dir="ltr"
                className="flex-1 min-w-0 rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-2.5 py-1 bv-field-focus text-start"
              />
              <button
                onClick={handleCreate}
                disabled={creating || newName.trim() === ""}
                className="shrink-0 rounded-control bg-carbon-surface3 px-3 py-1 text-xs text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
              >
                {creating ? t("folder.creating") : t("folder.newFolder")}
              </button>
            </div>
          )}
          {createError && <p className="text-xs text-statusFail">{createError}</p>}

          {/* Action buttons */}
          {!manualFallback && (
            <div className="flex items-center gap-2 pt-1 border-t border-carbon-border">
              <button
                onClick={handleSelect}
                className="text-xs rounded-control bg-carbon-surface3 px-3 py-1 text-carbon-text hover:bg-carbon-hover transition-colors"
              >
                {t("folder.use")}
              </button>
              <span dir="ltr" className="text-xs text-carbon-textMuted font-mono min-w-0 truncate text-start">
                {browsePath || "(root)"}
              </span>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
