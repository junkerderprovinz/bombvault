import { useState } from "react";
import type { Settings, PrimaryRemoteDomain } from "../lib/api";
import { FolderBrowser } from "./FolderBrowser";
import { OffsiteWizard } from "./OffsiteWizard";
import { Selector } from "./Selector";
import { useT } from "../lib/i18n";

// ---------------------------------------------------------------------------
// PathModeSwitch — inline Local/Remote mode switcher for a domain's backup
// path field (issue #152).
//
// A backup path already accepts a raw restic remote URL (s3:/rest:/sftp:/
// b2:/rclone:...) and restic backs up to it directly — nothing about that
// changes here. This widget is UI framing over the SAME path value
// FolderBrowser already edits: "Local" shows the familiar folder browser;
// "Remote" swaps it for a plain URL field and, below it, reuses
// OffsiteWizard's connection-test/credentials dialog (primary=true) — the
// SAME dialog off-site destinations already use — to configure bandwidth
// limits, append-only protection and a growth-budget alarm for it, in place
// rather than duplicated.
//
// The Local/Remote pair itself renders on the shared Selector component
// (GlimStone form-engine Phase 2, Task 3 follow-up — this file landed via an
// unrelated PR that merged around the same time as Task 3 and never got
// swept into it). Was two hand-rolled `<button>`s sharing one
// `bg-carbon-background` pill behind them, styled independently of
// Selector's twelve other call sites and — because a plain `<button>`'s
// focus/hover/roving-tabindex behaviour isn't free the way Selector's is —
// the one segmented control in the app with no arrow-key navigation. Same
// shape as SourceToggle.tsx's Local/Off-site toggle (a bare two-item
// `select="one"` strip, no leading caption, `plain`/`hue` left at their
// defaults), so this follows that call site rather than the captioned
// SortControl/ChipFilter ones: the shared wrapper pill is dropped exactly
// as SourceToggle's own migration dropped its shared `bg-carbon-surface2`
// pill (Selector.tsx's own header comment, point 2) — Selector's default
// `plain={false}` chip look already carries that visual weight per segment,
// nothing else needs to. `label` doubles as the strip's accessible name
// (Selector's `label` prop is required and never rendered as visible text
// here, same as SourceToggle's own `t("source.label")`) — reusing the
// caller's own per-domain string ("Containers path", "VMs path", ...)
// rather than adding one new generic i18n key shared by all five call
// sites, since that string is already translated everywhere and gives each
// instance a more specific accessible name than a single shared "Path mode"
// ever could. No `disabled` prop existed on this component before the
// migration and none was added — the original two buttons were never
// wired to any busy/disabled state, so there is nothing to carry over.
// ---------------------------------------------------------------------------

// Mirrors restic's remoteRepoRe (internal/restic/restic.go): a leading
// "scheme:" that names one of restic's native remote backends or rclone.
// Exported for isRemotePath.test.ts — keeping this in lockstep with the
// backend regex is exactly the kind of drift a unit test catches cheaply.
const REMOTE_RE = /^(rclone|sftp|rest|s3|b2|azure|gs|swift):/;

export function isRemotePath(v: string): boolean {
  return REMOTE_RE.test(v.trim());
}

export function PathModeSwitch({
  label,
  domain,
  value,
  hostMountRoot,
  onChange,
  placeholder,
  settings,
  setSettings,
  save,
}: {
  label: string;
  domain: PrimaryRemoteDomain;
  value: string;
  hostMountRoot: string;
  onChange: (v: string) => void;
  placeholder?: string;
  settings: Settings;
  setSettings: React.Dispatch<React.SetStateAction<Settings | null>>;
  save: (
    patch: Partial<Settings>,
    setState: (s: "idle" | "saving" | "saved" | "error") => void,
    setError: (e: string | null) => void
  ) => Promise<boolean>;
}) {
  const { t } = useT();
  // The mode starts derived from the CURRENT value (a remote-shaped path
  // opens in Remote mode on load), but is then an independent UI choice: a
  // user can switch to "Remote" with an empty/local value to start typing a
  // URL, without the widget snapping back to Local because the value doesn't
  // look remote YET.
  const [remoteMode, setRemoteMode] = useState(() => isRemotePath(value));
  const [dialogOpen, setDialogOpen] = useState(false);

  function switchToLocal() {
    setRemoteMode(false);
    setDialogOpen(false);
    if (isRemotePath(value)) onChange(""); // a remote URL is meaningless as a local subpath
  }

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center justify-end gap-2 -mb-1">
        <Selector
          items={[
            { id: "local", label: t("settings.pathMode.local") },
            { id: "remote", label: t("settings.pathMode.remote") },
          ]}
          label={label}
          select="one"
          active={remoteMode ? "remote" : "local"}
          onChange={(id) => (id === "remote" ? setRemoteMode(true) : switchToLocal())}
        />
      </div>

      {remoteMode ? (
        <div className="flex flex-col gap-1.5">
          <label className="text-xs text-carbon-textSub">{label}</label>
          {/* Task 6 (RTL sweep): this input occupies the SAME slot as
              FolderBrowser's path field below and holds the same kind of
              technical value (a restic remote URL), so it carries the same
              `dir="ltr"`/`text-start` pin — without it, switching Local →
              Remote in ar/he would flip the field's direction and strand a
              leading `/` or `:` at the wrong edge. */}
          <input
            value={value}
            spellCheck={false}
            onChange={(e) => onChange(e.target.value)}
            placeholder="s3:bucket/path or rest:http://host:8000/repo"
            dir="ltr"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus text-start"
          />
          <button
            type="button"
            onClick={() => setDialogOpen((o) => !o)}
            disabled={!isRemotePath(value)}
            className="self-start rounded-control bg-carbon-surface2 px-2.5 py-1 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
          >
            {dialogOpen ? t("offsite.wizard.close") : t("settings.primaryRemote.title")}
          </button>
          {!isRemotePath(value) && (
            <p className="text-xs text-carbon-textMuted">{t("settings.primaryRemote.hint")}</p>
          )}
          {dialogOpen && isRemotePath(value) && (
            <OffsiteWizard domain={domain} settings={settings} setSettings={setSettings} save={save} t={t} primary />
          )}
        </div>
      ) : (
        <FolderBrowser label={label} value={value} hostMountRoot={hostMountRoot} onChange={onChange} placeholder={placeholder} />
      )}
    </div>
  );
}
