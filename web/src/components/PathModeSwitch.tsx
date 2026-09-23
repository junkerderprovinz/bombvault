import { useState } from "react";
import type { Settings, PrimaryRemoteDomain } from "../lib/api";
import { FolderBrowser } from "./FolderBrowser";
import { OffsiteWizard } from "./OffsiteWizard";
import { Selector } from "./Selector";
import { IconCloud, IconLocal } from "./Sidebar";
import { Badge } from "./Badge";
import { useT } from "../lib/i18n";

// Local/Remote switch for a domain's backup path. A path already accepts a
// restic remote URL, and restic backs up to it directly; this only frames the
// value FolderBrowser edits. "Local" shows the folder browser. "Remote" shows a
// URL field and, below it, OffsiteWizard in primary mode for bandwidth limits,
// append-only protection and the growth-budget alarm.
//
// The caller's per-domain label ("Containers path") is the visible caption of
// the row and the Selector's accessible name. Each segment's tip explains what
// its glyph means.

// Mirrors restic's remoteRepoRe (internal/restic/restic.go): a leading scheme
// naming one of restic's remote backends or rclone. isRemotePath.test.ts keeps
// the two in step.
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
  hueIndex,
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
  /** This row's position among the Storage tab's path rows, for the primary
   *  remote button and the OffsiteWizard it opens. */
  hueIndex?: number;
}) {
  const { t } = useT();
  // Derived from the value only on mount, so switching to Remote with an empty
  // or local value does not snap back before a URL has been typed.
  const [remoteMode, setRemoteMode] = useState(() => isRemotePath(value));
  const [dialogOpen, setDialogOpen] = useState(false);

  function switchToLocal() {
    setRemoteMode(false);
    setDialogOpen(false);
    if (isRemotePath(value)) onChange(""); // a remote URL is meaningless as a local subpath
  }

  return (
    <div className="flex flex-col gap-1.5">
      {/* The label and the switch are one decision about one path, so they
          share a row; FolderBrowser's own label is turned off below. */}
      <div className="flex items-center justify-between gap-2">
        <label className="text-xs text-carbon-textSub">{label}</label>
        <Selector
          items={[
            {
              id: "local",
              label: t("settings.pathMode.local"),
              icon: <IconLocal />,
              tip: t("settings.pathMode.localTip"),
            },
            {
              id: "remote",
              label: t("settings.pathMode.remote"),
              icon: <IconCloud />,
              tip: t("settings.pathMode.remoteTip"),
            },
          ]}
          label={label}
          size="sm"
          select="one"
          equalWidth
          active={remoteMode ? "remote" : "local"}
          onChange={(id) => (id === "remote" ? setRemoteMode(true) : switchToLocal())}
        />
      </div>

      {remoteMode ? (
        <div className="flex flex-col gap-1.5">
          {/* Pinned LTR like FolderBrowser's path field, or a leading `/` or
              `:` lands at the wrong edge in ar and he. */}
          <input
            value={value}
            spellCheck={false}
            onChange={(e) => onChange(e.target.value)}
            placeholder="s3:bucket/path or rest:http://host:8000/repo"
            dir="ltr"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus text-start"
          />
          {/* tone="active" lets hueIndex paint. While the value is not a
              remote URL there is no button, only a sentence saying why. */}
          {isRemotePath(value) ? (
            <Badge
              as="button"
              tone="active"
              size="medium"
              hueIndex={hueIndex}
              onClick={() => setDialogOpen((o) => !o)}
              className="self-start"
            >
              {dialogOpen ? t("offsite.wizard.close") : t("settings.primaryRemote.title")}
            </Badge>
          ) : (
            <p className="text-xs text-carbon-textMuted">{t("settings.primaryRemote.hint")}</p>
          )}
          {dialogOpen && isRemotePath(value) && (
            <OffsiteWizard domain={domain} settings={settings} setSettings={setSettings} save={save} t={t} primary hueIndex={hueIndex} />
          )}
        </div>
      ) : (
        <FolderBrowser
          label={label}
          renderLabel={false}
          value={value}
          hostMountRoot={hostMountRoot}
          onChange={onChange}
          placeholder={placeholder}
        />
      )}
    </div>
  );
}
