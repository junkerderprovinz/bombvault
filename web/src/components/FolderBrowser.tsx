import { useState, useCallback } from "react";
import { browse, createFolder } from "../lib/api";
import { useT } from "../lib/i18n";
import { InfoBubble } from "./InfoBubble";
import { Badge } from "./Badge";
import { IconFolder } from "./Sidebar";
import { useToast } from "../lib/toast";

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
  /** Optional one-line explanation of what THIS field is for, rendered as a
   *  neutral (i) beside the label (design-language.md rule 8) instead of a
   *  separate permanent grey <p> under the whole field — same additive,
   *  every-other-call-site-unchanged shape as Card's own `hint` prop
   *  (GlimStone form-engine Phase 2 Task 4). Optional: omitted call sites
   *  (Recovery.tsx, Files.tsx, Containers.tsx, RestorePanel.tsx,
   *  PathModeSwitch's own internal Local-mode use) render byte-for-byte the
   *  same as before. */
  hint?: string;
  /** Suppresses this component's own label row (text + optional hint bubble)
   *  — GlimStone follow-up round, Paths & Storage tab rework point 5:
   *  PathModeSwitch now renders the SAME label on a shared row alongside its
   *  own Local/Remote Selector, so FolderBrowser must not also render a
   *  second copy of it directly underneath. Default true: every other call
   *  site (Recovery.tsx, Files.tsx, Containers.tsx, RestorePanel.tsx,
   *  Settings.tsx's own direct calls) keeps rendering its own label exactly
   *  as before. */
  renderLabel?: boolean;
}

export function FolderBrowser({ label, value, hostMountRoot, onChange, placeholder, hint, renderLabel = true }: FolderBrowserProps) {
  const { t } = useT();
  const { push } = useToast();
  // browsePath tracks the *current directory being listed* (not the selected value).
  // We initialise it to the current value so opening the browser starts in the right folder.
  const [open, setOpen] = useState(false);
  const [browsePath, setBrowsePath] = useState(value);
  const [dirs, setDirs] = useState<{ name: string; path: string }[]>([]);
  // Directory-listing failure — NOT migrated to a toast (GlimStone follow-up
  // pass, v8.0.0 audit note): it replaces the whole browser panel content
  // (falls back to manualFallback below), the same structural "the section
  // failed to load" condition every other page-level list-load error gets
  // left inline for, not a one-shot action.
  const [browseError, setBrowseError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [manualFallback, setManualFallback] = useState(false);
  // New-folder creation inside the current browsePath.
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
        // Navigate into the freshly created folder so "use this folder" selects it.
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

  return (
    <div className="flex flex-col gap-1.5">
      {renderLabel && (
        <label className="flex items-center gap-1 text-xs text-carbon-textSub">
          {label}
          {hint && <InfoBubble tip={hint} />}
        </label>
      )}

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
        {/* Icon-only (GlimStone follow-up round, point 1 — "Durchsuchen"
            becomes a glyph, no text label).
            CORRECTED (jdp, live-review screenshot — the Local/Remote badges
            above this row read as wider/pill-shaped while this button read as
            genuinely square, and neither matched this very input's own
            height): the original `p-[2px]` around a 16×16 glyph rendered a
            true 20×20 square, which WAS internally consistent with
            PathModeSwitch's Selector segments at the time this comment was
            written, but neither control was ever actually checked against
            THIS field's own real rendered height — verified live via
            getComputedStyle: this input (text-sm px-3 py-1.5) renders at
            exactly 32px tall, not 20px. Fixed `h-8 w-8` (Tailwind's own
            plain default spacing scale, step 8 = 2rem = 32px, not a new
            bracket/arbitrary value) makes this button a true square at
            exactly that height — the same fixed size Selector.tsx now gives
            every `iconOnly` segment for the identical reason (see that
            file's own comment on the `item.iconOnly` branch). `rounded-control`
            (not a fixed pill radius — the two colour-reset Badges elsewhere
            in Settings.tsx are shape="square" now too, but still their OWN
            "icon" size stage measured against a different neighbour, see
            those call sites) — this button sits flush
            beside the path input's own shape-engine-reactive `rounded-control`
            corner, and a permanently-circular neighbour would visibly break
            from it under the square/soft shape settings.
              CORRECTED AGAIN (jdp, live-review: "beim Ordnersymbol ist die
            Hover-Infobubble nicht im GlimStone") — this button's icon-only
            conversion above never actually got a hover tooltip of its OWN
            kind: it carried a plain native `title=`/`aria-label=`, the
            browser's own OS balloon, while PathModeSwitch's own Local/Remote
            icon pair right next to it already renders the app's real
            `.glim-bubble` chrome (via Selector's `tip`) — two icon-only
            triggers sitting in the same view, two visibly different tooltip
            systems. `IconTipButton` (new shared component, extracted
            specifically so a third bespoke copy of InfoBubble.tsx's/
            SelectorTab's identical tooltip-state logic didn't need to exist)
            is the same engine wired to a plain `<button>` instead of a
            Selector segment. `folder.browseTitle` (already translated in
            every locale — it was this button's native `title` a moment ago)
            becomes the tip text and the button's `aria-label` unchanged. */}
        {/* COLOUR-ENGINE ROUND (jdp's standing rule, five escalations deep,
            "IMMER alles in die Farb- und Formengine integrieren"): this button
            was still a flat `bg-carbon-surface3` grey — one of six square icon
            badges left outside the engine after the delete badge's own grey
            special-casing was removed for exactly this reason. It is a real
            `Badge` now (`as="button" tone="active" shape="square"
            size="icon"`), the identical construction Settings.tsx's
            ReplicateNowButton/TestConnectionButton already use: `tone="active"`
            is the one non-heading tone the hue engine is allowed to drive, and
            for an icon-only badge Badge resolves it to a full solid
            `bg-accent`/`text-accentContrast` fill rather than the pale wash jdp
            rejected as "halb abgedunkelt" (see Badge.tsx's own `toneClasses`
            ROUND 2 comment). The glyph itself stays neutral `currentColor` —
            "die icons sollen nicht eingefaerbt werden, nur die badges also der
            hintergrund".
              NO `hueIndex` prop, deliberately, and no new prop threaded
            through this component's 19 call sites: `[data-rainbow] .glim-hue`
            (index.css) redefines `--color-accent` for its whole SUBTREE, and
            every one of those call sites already sits inside an ancestor that
            carries it — Settings.tsx's Card(), recovery/StepCard.tsx,
            Recovery.tsx's Restore/Cloud/Rclone rows, Containers.tsx's
            ContainerRow, Files.tsx's FileSetRow. So `bg-accent` here resolves
            to the enclosing card's OWN rainbow position by ordinary custom-
            property inheritance, with no per-call-site wiring to forget. That
            is the same already-established mechanism Containers.tsx's own
            folder-add badge documents ("no `hueIndex` needed — this panel
            already lives inside ContainerRow's own `.glim-hue` element").
            Verified live with getComputedStyle, not assumed.
              `size="icon"` is the app's ONE square-icon-badge size (32px) and
            is the exact `h-8 w-8` this call site already had, so the footprint
            is unchanged; `shrink-0` survives as `className` because it is
            layout, not appearance. `tip` still carries `folder.browseTitle`
            and still routes through IconTipButton — Badge renders its own
            `tip` branch through that very component. */}
        <Badge
          as="button"
          tone="active"
          shape="square"
          size="icon"
          className="shrink-0"
          onClick={handleOpen}
          tip={t("folder.browseTitle")}
        >
          <IconFolder />
        </Badge>
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
