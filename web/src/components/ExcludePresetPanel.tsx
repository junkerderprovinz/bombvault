import { useState } from "react";
import type { ExcludePreset } from "../lib/api";
import type { TranslationKey, useT } from "../lib/i18n";
import { Button } from "./Button";
import { InfoBubble } from "./InfoBubble";
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

// ExcludePresetPanel offers the folders a recognised app fills again by
// itself. Nothing is stored until the user presses apply, and every line can
// be switched off first.
export function ExcludePresetPanel({
  preset,
  currentLines,
  saving,
  onApply,
  t,
}: {
  preset: ExcludePreset;
  currentLines: string[];
  saving: boolean;
  onApply: (lines: string[]) => void;
  t: T;
}) {
  const [open, setOpen] = useState(false);
  const entries = preset.entries.filter((e) => KIND_LABEL[e.kind] && !currentLines.includes(e.line));
  const [picked, setPicked] = useState<Record<string, boolean>>(() =>
    Object.fromEntries(preset.entries.map((e) => [e.line, !e.optional]))
  );
  if (entries.length === 0) return null;
  const chosen = entries.filter((e) => picked[e.line]).map((e) => e.line);
  const app = APP_NAME[preset.app] ?? preset.app;
  const panelId = `exclude-preset-${preset.app}`;

  return (
    <div className="flex flex-col gap-2">
      <Button
        label={t("excludes.presetLoad").replace("{app}", app)}
        labelKey="excludes.presetLoad"
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
        <div id={panelId} className="flex flex-col gap-1.5 rounded-control bg-carbon-surface2 p-2">
          {entries.map((e) => (
            <div key={e.line} className="flex items-center gap-2">
              <Toggle
                checked={picked[e.line] ?? false}
                onChange={(next) => setPicked((p) => ({ ...p, [e.line]: next }))}
                label={t(KIND_LABEL[e.kind])}
              />
              <InfoBubble tip={t(KIND_HINT[e.kind])} />
              <span dir="ltr" className="min-w-0 flex-1 truncate text-start font-mono text-xs text-carbon-textSub" title={e.line}>
                {e.line}
              </span>
            </div>
          ))}
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
