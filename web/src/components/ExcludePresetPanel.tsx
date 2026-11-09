import { useState } from "react";
import type { ExcludePreset } from "../lib/api";
import type { TranslationKey, useT } from "../lib/i18n";
import { Button } from "./Button";
import { Rows, SettingRow } from "./storage/rows";
import { Toggle } from "./Toggle";

type T = ReturnType<typeof useT>["t"];

const KIND_LABEL: Record<string, TranslationKey> = {
  cache: "excludes.presetKind.cache",
  logs: "excludes.presetKind.logs",
  crashReports: "excludes.presetKind.crashReports",
  previews: "excludes.presetKind.previews",
  trickplay: "excludes.presetKind.trickplay",
  metadata: "excludes.presetKind.metadata",
  covers: "excludes.presetKind.covers",
  encodedVideo: "excludes.presetKind.encodedVideo",
  models: "excludes.presetKind.models",
  macMetadata: "excludes.presetKind.macMetadata",
  windowsThumbnails: "excludes.presetKind.windowsThumbnails",
  windowsFolderSettings: "excludes.presetKind.windowsFolderSettings",
  synologyIndex: "excludes.presetKind.synologyIndex",
  recycleBin: "excludes.presetKind.recycleBin",
  trash: "excludes.presetKind.trash",
  tempFiles: "excludes.presetKind.tempFiles",
};

const KIND_HINT: Record<string, TranslationKey> = {
  cache: "excludes.presetHint.cache",
  logs: "excludes.presetHint.logs",
  crashReports: "excludes.presetHint.crashReports",
  previews: "excludes.presetHint.previews",
  trickplay: "excludes.presetHint.trickplay",
  metadata: "excludes.presetHint.metadata",
  covers: "excludes.presetHint.covers",
  encodedVideo: "excludes.presetHint.encodedVideo",
  models: "excludes.presetHint.models",
  macMetadata: "excludes.presetHint.macMetadata",
  windowsThumbnails: "excludes.presetHint.windowsThumbnails",
  windowsFolderSettings: "excludes.presetHint.windowsFolderSettings",
  synologyIndex: "excludes.presetHint.synologyIndex",
  recycleBin: "excludes.presetHint.recycleBin",
  trash: "excludes.presetHint.trash",
  tempFiles: "excludes.presetHint.tempFiles",
};

const APP_NAME: Record<string, string> = {
  plex: "Plex",
  jellyfin: "Jellyfin",
  emby: "Emby",
  sonarr: "Sonarr",
  radarr: "Radarr",
  lidarr: "Lidarr",
  readarr: "Readarr",
  prowlarr: "Prowlarr",
  immich: "Immich",
  nextcloud: "Nextcloud",
  photoprism: "PhotoPrism",
  tautulli: "Tautulli",
};

/** The lines of a preset whose kind the interface has words for. */
export function knownEntries(preset: ExcludePreset): ExcludePreset["entries"] {
  return preset.entries.filter((e) => KIND_LABEL[e.kind]);
}

/** What a preset starts with: every line ticked that is not optional. */
export function presetPicks(preset: ExcludePreset): Record<string, boolean> {
  return Object.fromEntries(preset.entries.map((e) => [e.line, !e.optional]));
}

// PresetRows is one switch per offered line, with what the line leaves out.
export function PresetRows({
  entries,
  picked,
  onPick,
  disabled,
  t,
}: {
  entries: ExcludePreset["entries"];
  picked: Record<string, boolean>;
  onPick: (line: string, on: boolean) => void;
  disabled?: boolean;
  t: T;
}) {
  return (
    <Rows>
      {entries.map((e) => (
        <SettingRow
          key={e.line}
          label={t(KIND_LABEL[e.kind])}
          hint={t(KIND_HINT[e.kind])}
          note={
            <bdi dir="ltr" className="text-start font-mono wrap-anywhere">
              {e.line}
            </bdi>
          }
        >
          <Toggle hideLabel checked={picked[e.line] ?? false} onChange={(next) => onPick(e.line, next)} label={t(KIND_LABEL[e.kind])} disabled={disabled} />
        </SettingRow>
      ))}
    </Rows>
  );
}

// ExcludePresetPanel offers the folders a recognised app fills again by
// itself. Nothing is stored until the user presses apply, and every line can
// be switched off first.
export function ExcludePresetPanel({
  preset,
  currentLines,
  saving,
  onApply,
  loadKey,
  t,
}: {
  preset: ExcludePreset;
  /** The words of the button, for a preset that is no app's. */
  loadKey?: TranslationKey;
  currentLines: string[];
  saving: boolean;
  onApply: (lines: string[]) => void;
  t: T;
}) {
  const [open, setOpen] = useState(false);
  const entries = knownEntries(preset).filter((e) => !currentLines.includes(e.line));
  const [picked, setPicked] = useState<Record<string, boolean>>(() => presetPicks(preset));
  if (entries.length === 0) return null;
  const chosen = entries.filter((e) => picked[e.line]).map((e) => e.line);
  const app = APP_NAME[preset.app] ?? preset.app;
  const panelId = `exclude-preset-${preset.app}`;

  return (
    <div className="flex flex-col gap-2">
      <Button
        label={loadKey ? t(loadKey) : t("excludes.presetLoad").replace("{app}", app)}
        labelKey={loadKey ?? "excludes.presetLoad"}
        tone="neutral"
        onClick={() => setOpen((o) => !o)}
        ariaExpanded={open}
        ariaControls={panelId}
        glyph={
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none" className={`transition-transform ${open ? "rotate-90" : "rtl:rotate-180"}`}>
            <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
          </svg>
        }
      />
      {open && (
        <div id={panelId} className="flex flex-col gap-3">
          <PresetRows entries={entries} picked={picked} onPick={(line, on) => setPicked((p) => ({ ...p, [line]: on }))} t={t} />
          <div className="flex justify-end">
            <Button
              label={t("excludes.presetApply")}
              labelKey="excludes.presetApply"
              tone="accent"
              disabled={saving || chosen.length === 0}
              onClick={() => onApply(chosen)}
            />
          </div>
        </div>
      )}
    </div>
  );
}
