import { useEffect, useState } from "react";
import { IconTabStorage } from "../../../components/navGlyphs";
import { PathModeSwitch } from "../../../components/PathModeSwitch";
import { useT } from "../../../lib/i18n";
import { listStorageLocations } from "../../../lib/api";
import { PlacementDefaultsCard } from "../PlacementDefaultsCard";
import { ReposCard } from "../ReposCard";
import { PageLinkCard, SidebarCard } from "../SidebarTile";
import { CloudCards } from "../sections/CloudCards";
import { OffsiteCards } from "../sections/OffsiteCards";
import { RetentionCards } from "../sections/RetentionCards";
import { Card, hueCounter, type SaveState } from "../shared";
import { useSettings } from "../settingsStore";

export function StoragePage() {
  const { t } = useT();
  const { settings, setSettings, hostMountRoot, save, debouncedSave } = useSettings();

  const [, setPathSaveState] = useState<SaveState>("idle");
  const [, setPathSaveError] = useState<string | null>(null);
  const [counts, setCounts] = useState<{ all: number; offsite: number } | null>(null);

  useEffect(() => {
    let alive = true;
    listStorageLocations()
      .then((res) => {
        const locations = res.locations ?? [];
        if (alive && res.ok) setCounts({ all: locations.length, offsite: locations.filter((l) => l.kind === "offsite").length });
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  const nextHue = hueCounter();

  return (
    <>
      <SidebarCard pin="storage" hueIndex={nextHue()} />
      <PageLinkCard
        to="/storage"
        label={t("storage.title")}
        sub={
          counts
            ? t("tile.storage.sub").replace("{n}", String(counts.all)).replace("{m}", String(counts.offsite))
            : t("dashboard.checking")
        }
        glyph={<IconTabStorage />}
        hueIndex={nextHue()}
      />

      {/* Above the domain paths: these are the places an individual
          container, VM or folder set can be pointed at instead of the domain
          path below, so the more specific answer is read first. */}
      <ReposCard hueIndex={nextHue()} />
      <PlacementDefaultsCard hueIndex={nextHue()} />

      <Card title={t("settings.paths")} hint={t("settings.pathsHint").replace("{root}", hostMountRoot)} hueIndex={nextHue()}>
        {/* Each field saves itself, debounced per field name like the
            schedules page's cadence fields. The six PathModeSwitch rows are
            one group with their own 0-based hueIndex, separate from this
            card's heading. */}
        <PathModeSwitch
          label={t("settings.containersPath")}
          domain="containers"
          value={settings.containersPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, containersPath: v } : prev);
            debouncedSave("containersPath", () =>
              void save({ containersPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={0}
        />
        <PathModeSwitch
          label={t("settings.vmsPath")}
          domain="vms"
          value={settings.vmsPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, vmsPath: v } : prev);
            debouncedSave("vmsPath", () =>
              void save({ vmsPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={1}
        />
        <PathModeSwitch
          label={t("settings.flashPath")}
          domain="flash"
          value={settings.flashPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, flashPath: v } : prev);
            debouncedSave("flashPath", () =>
              void save({ flashPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={2}
        />
        <PathModeSwitch
          label={t("settings.configPath")}
          domain="config"
          value={settings.configPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, configPath: v } : prev);
            debouncedSave("configPath", () =>
              void save({ configPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={3}
        />
        <PathModeSwitch
          label={t("settings.filesPath")}
          domain="files"
          value={settings.filesPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, filesPath: v } : prev);
            debouncedSave("filesPath", () =>
              void save({ filesPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={4}
        />
        <PathModeSwitch
          label={t("settings.zfsPath")}
          domain="zfs"
          value={settings.zfsPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setSettings((prev) => prev ? { ...prev, zfsPath: v } : prev);
            debouncedSave("zfsPath", () =>
              void save({ zfsPath: v }, setPathSaveState, setPathSaveError)
            );
          }}
          settings={settings}
          setSettings={setSettings}
          save={save}
          hueIndex={5}
        />
      </Card>

      <RetentionCards nextHue={nextHue} />
      <OffsiteCards nextHue={nextHue} />
      <CloudCards nextHue={nextHue} />
    </>
  );
}
