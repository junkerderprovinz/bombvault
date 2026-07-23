import { useState, useCallback } from "react";
import { browse } from "../lib/api";
import { useT } from "../lib/i18n";

// ---------------------------------------------------------------------------
// Folder browser (shared)
// ---------------------------------------------------------------------------

export interface FolderBrowserProps {
  label: string;
  value: string;
  hostMountRoot: string;
  onChange: (v: string) => void;
}

export function FolderBrowser({ label, value, hostMountRoot, onChange }: FolderBrowserProps) {
  const { t } = useT();
  // browsePath tracks the *current directory being listed* (not the selected value).
  // We initialise it to the current value so opening the browser starts in the right folder.
  const [open, setOpen] = useState(false);
  const [browsePath, setBrowsePath] = useState(value);
  const [dirs, setDirs] = useState<{ name: string; path: string }[]>([]);
  const [browseError, setBrowseError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [manualFallback, setManualFallback] = useState(false);

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
          placeholder="user/bombvault/container"
          className="flex-1 rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-hidden focus:border-[#78a9ff]"
        />
        <button
          onClick={handleOpen}
          title={t("folder.browseTitle")}
          className="shrink-0 rounded-lg bg-carbon-surface2 border border-carbon-border px-3 py-1.5 text-xs text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors"
        >
          {t("folder.browse")}
        </button>
      </div>

      {/* Absolute path preview */}
      {resolved && (
        <p className="text-xs text-carbon-textMuted font-mono break-all">→ {resolved}</p>
      )}
      {!resolved && trimmed && (
        <p className="text-xs text-[#ff8389]">
          {t("folder.pathHint")}
        </p>
      )}

      {/* Browser panel */}
      {open && (
        <div className="mt-1 rounded-lg bg-carbon-surface2 border border-carbon-border p-3 flex flex-col gap-2">
          {/* Header: current path + close */}
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-mono text-carbon-textSub min-w-0 truncate">
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
            <p className="text-xs text-[#ff8389]">{browseError}</p>
          )}

          {/* Loading spinner */}
          {loading && (
            <div className="flex items-center gap-2 text-xs text-carbon-textMuted">
              <span className="h-3 w-3 rounded-full border-2 border-[#78a9ff] border-t-transparent animate-spin" />
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
                  className="text-left text-xs font-mono text-carbon-textSub px-2 py-1 rounded-sm hover:bg-carbon-hover hover:text-carbon-text transition-colors"
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
                  className="text-left text-xs font-mono text-carbon-textSub px-2 py-1 rounded-sm hover:bg-carbon-hover hover:text-carbon-text transition-colors"
                >
                  {d.name}/
                </button>
              ))}
            </div>
          )}

          {/* Action buttons */}
          {!manualFallback && (
            <div className="flex items-center gap-2 pt-1 border-t border-carbon-border">
              <button
                onClick={handleSelect}
                className="text-xs rounded-lg bg-carbon-surface3 px-3 py-1 text-carbon-text hover:bg-carbon-hover transition-colors"
              >
                {t("folder.use")}
              </button>
              <span className="text-xs text-carbon-textMuted font-mono min-w-0 truncate">
                {browsePath || "(root)"}
              </span>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
