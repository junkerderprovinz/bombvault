import { useState } from "react";
import type { Settings, PrimaryRemoteDomain } from "../lib/api";
import { FolderBrowser } from "./FolderBrowser";
import { OffsiteWizard } from "./OffsiteWizard";
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

  const pillCls = (active: boolean) =>
    `rounded-control px-2.5 py-1 text-xs transition-colors ${
      active ? "bg-accent text-accentContrast" : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-hover"
    }`;

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center justify-end gap-2 -mb-1">
        <span className="inline-flex items-center gap-1 rounded-control bg-carbon-background p-0.5">
          <button type="button" onClick={switchToLocal} className={pillCls(!remoteMode)}>
            {t("settings.pathMode.local")}
          </button>
          <button type="button" onClick={() => setRemoteMode(true)} className={pillCls(remoteMode)}>
            {t("settings.pathMode.remote")}
          </button>
        </span>
      </div>

      {remoteMode ? (
        <div className="flex flex-col gap-1.5">
          <label className="text-xs text-carbon-textSub">{label}</label>
          <input
            value={value}
            spellCheck={false}
            onChange={(e) => onChange(e.target.value)}
            placeholder="s3:bucket/path or rest:http://host:8000/repo"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus"
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
