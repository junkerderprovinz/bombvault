import { useState, useCallback, useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import { browse, createFolder } from "../lib/api";
import { useT } from "../lib/i18n";
import { InfoBubble } from "./InfoBubble";
import { Button } from "./Button";
import { Badge } from "./Badge";
import { groupStage } from "../lib/controls";
import { IconCheckCircle, IconFolder } from "./Sidebar";
import { IconBack } from "./glyphs";
import { useToast } from "../lib/toast";
import { usePortalHue } from "../lib/portalHue";

export interface FolderBrowserProps {
  label: string;
  value: string;
  hostMountRoot: string;
  onChange: (v: string) => void;
  /** Example path shown while the field is empty. Defaults to "user/appdata",
   *  which exists on every Unraid box. */
  placeholder?: string;
  /** One-line explanation of the field, shown as an (i) beside the label. */
  hint?: string;
  /** False when the caller renders the label itself, as PathModeSwitch does on
   *  the row it shares with its Local/Remote selector. */
  renderLabel?: boolean;
  /** Render in place instead of as a dialog, for call sites that are already
   *  inside one. */
  inDialog?: boolean;
}

export function FolderBrowser({ label, value, hostMountRoot, onChange, placeholder, hint, renderLabel = true, inDialog = false }: FolderBrowserProps) {
  const { t } = useT();
  // "New folder" and "Use this folder" sit one above the other; one stage for
  // both gives the column a straight edge in every language.
  const folderActionStage = groupStage([t("folder.newFolder"), t("folder.use")]);
  const { push } = useToast();
  // Anchors usePortalHue, so the portalled dialog keeps the hue of the card it
  // was opened from instead of falling back to the global accent.
  const triggerRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  // The directory being listed, not the selected value. It starts at the value
  // so the browser opens in the right folder.
  const [browsePath, setBrowsePath] = useState(value);
  const [dirs, setDirs] = useState<{ name: string; path: string }[]>([]);
  // Shown inline rather than as a toast: a failed listing replaces the whole
  // panel with the manual fallback.
  const [browseError, setBrowseError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [manualFallback, setManualFallback] = useState(false);
  const [newName, setNewName] = useState("");
  const [creating, setCreating] = useState(false);

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

  // Bound only while open, so the many mounted browsers do not each keep a
  // document listener for a dialog nobody opened.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        handleClose();
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open]);

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
    createFolder(browsePath, name)
      .then((res) => {
        if (!res.ok) {
          push(res.error ?? t("folder.createFailed"), "fail");
          return;
        }
        setNewName("");
        // Navigate into the new folder so "use this folder" selects it.
        doFetch(res.path ?? browsePath);
      })
      .catch((err: unknown) => {
        push(err instanceof Error ? err.message : t("folder.createFailed"), "fail");
      })
      .finally(() => setCreating(false));
  }

  const trimmed = value.trim();
  const resolved =
    trimmed && !trimmed.startsWith("/") && !trimmed.includes("..")
      ? `${hostMountRoot}/${trimmed}`
      : "";

  // Rendered by both branches below, as a dialog or in place.
  const panel = (
    <>
      <div className="flex items-center justify-between gap-2">
        <span dir="ltr" className="text-xs font-mono text-carbon-textSub min-w-0 truncate text-start">
          {hostMountRoot}/{browsePath || ""}
        </span>
        <Button
          label={t("common.close")}
          labelKey="common.close"
          tone="neutral"
          onClick={handleClose}
          className="shrink-0"
        />
      </div>

      {browseError && (
        <p className="text-xs text-statusFail">{browseError}</p>
      )}

      {/* border-accentText rather than border-accent: the flat accent is under
          the 3:1 contrast a non-text indicator needs in the light theme. */}
      {loading && (
        <div className="flex items-center gap-2 text-xs text-carbon-textMuted">
          <span className="h-3 w-3 rounded-full border-2 border-accentText border-t-transparent animate-spin" />
          {t("folder.loading")}
        </div>
      )}

      {/* A fixed height, so the dialog keeps one size from directory to
          directory. Rows set min-h-8 because .glim-btn-xs sets no height, and
          !justify-start because .glim-btn centres with a higher specificity. */}
      {!loading && !manualFallback && (
        <div className="flex flex-col gap-0.5 h-[clamp(12rem,55vh,32rem)] overflow-y-auto">
          {browsePath !== "" && (
            <Button
              // The file-manager convention, not a phrase to translate.
              label={".."}
              labelKey={null}
              glyph={<IconBack />}
              tone="neutral"
              onClick={handleUp}
              keepLabel
              className="w-full !justify-start min-h-8"
            />
          )}
          {dirs.length === 0 && !browseError && (
            <p className="text-xs text-carbon-textMuted px-2">{t("folder.none")}</p>
          )}
          {dirs.map((d) => (
            <Button
              key={d.path}
              label={d.name}
              // a directory name is user data, not a translation key
              labelKey={null}
              glyph={<IconFolder />}
              tone="neutral"
              onClick={() => doFetch(d.path)}
              // Folder names stay visible in every label mode: a listing read
              // one hover at a time is not a listing.
              keepLabel
              className="w-full !justify-start min-h-8"
            />
          ))}
        </div>
      )}

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
            // The house field size, which matches the 32px button beside it.
            className="flex-1 min-w-0 rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus text-start"
          />
          <Button
            label={t("folder.newFolder")}
            labelKey="folder.newFolder"
            // The two actions take the accent. The directory rows and Close
            // stay neutral, so the list still reads as a list.
            tone="accent"
            onClick={handleCreate}
            disabled={creating || newName.trim() === ""}
            busy={creating}
            title={creating ? t("folder.creating") : undefined}
            stage={folderActionStage}
            className="shrink-0 justify-start"
          />
        </div>
      )}

      {!manualFallback && (
        <div className="flex items-center gap-2 pt-1">
          <span dir="ltr" className="text-xs text-carbon-textMuted font-mono min-w-0 flex-1 truncate text-start">
            {browsePath || "(root)"}
          </span>
          {/* justify-start keeps the words from floating in the middle of the
              wide pill. */}
          <Button
            label={t("folder.use")}
            labelKey="folder.use"
            glyph={<IconCheckCircle />}
            tone="accent"
            onClick={handleSelect}
            stage={folderActionStage}
            className="shrink-0 justify-start"
          />
        </div>
      )}
    </>
  );

  const hue = usePortalHue(open, triggerRef);

  return (
    <div className="flex flex-col gap-1.5" ref={triggerRef}>
      {renderLabel && (
        <label className="flex items-center gap-1 text-xs text-carbon-textSub">
          {label}
          {hint && <InfoBubble tip={hint} />}
        </label>
      )}

      {/* The path input refuses to shrink below its intrinsic width (~192px),
          so on a narrow column the row overflowed and the trigger landed on
          neighbouring controls, here and in every other call site of this
          shared component. Under 48rem the input takes the whole line and the
          trigger wraps below it; the single-line row stays the desktop
          layout. */}
      <div className="flex items-center gap-2 max-md:flex-wrap">
        <input
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          spellCheck={false}
          placeholder={placeholder ?? "user/appdata"}
          dir="ltr"
          className="flex-1 rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus text-start max-md:min-w-full"
        />
        <Button
          label={t("folder.browseTitle")}
          labelKey="folder.browseTitle"
          glyph={<IconFolder />}
          tone="accent"
          onClick={handleOpen}
          className={"shrink-0"}
        />
      </div>

      {resolved && (
        <p dir="ltr" className="text-xs text-carbon-textMuted font-mono break-all text-start">→ {resolved}</p>
      )}
      {!resolved && trimmed && (
        <p className="text-xs text-statusFail">
          {t("folder.pathHint")}
        </p>
      )}

      {/* A dialog, except when this is already inside one: a second window on
          top of the one being filled in gets in the way. */}
      {open && (inDialog ? (
        <div className="mt-1 rounded-card bg-carbon-background p-3 flex flex-col gap-2">
          {panel}
        </div>
      ) : createPortal(
        <div
          className={`glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4${
            hue.className ? ` ${hue.className}` : ""
          }`}
          style={hue.style}
          onClick={handleClose}
        >
          {/* The shell is relative so the heading notch can straddle its edge,
              and it does not scroll, so the notch is not clipped. */}
          <div className="relative w-full max-w-2xl" onClick={(e) => e.stopPropagation()}>
            {/* px-5 repeats the box's p-5: the notch takes its inset from the
                padding around it, and this heading sits outside the scrolling
                box, which would clip it. */}
            <h2 className="flex items-center px-5">
              <Badge tone="heading" size="heading" wrap>{label}</Badge>
            </h2>
            <div
              role="dialog"
              aria-modal="true"
              aria-label={label}
              className="w-full max-h-[90vh] overflow-y-auto rounded-card bg-carbon-surface p-5 shadow-2xl flex flex-col gap-2"
            >
              {panel}
            </div>
          </div>
        </div>,
        document.body,
      ))}
    </div>
  );
}
