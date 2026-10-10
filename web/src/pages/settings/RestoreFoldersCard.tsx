import { useRef, useState } from "react";
import { ActionMenu } from "../../components/backups/ActionMenu";
import { Button } from "../../components/Button";
import { FolderBrowser } from "../../components/FolderBrowser";
import { IconAdd, IconTrash } from "../../components/navGlyphs";
import { DomainGlyph } from "../../components/storage/rows";
import type { OffsiteDomain, Settings } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { FLOW_DOMAINS } from "../../lib/storageFlow";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { Card, ToggleRow, type SaveState } from "./shared";
import { useSettings } from "./settingsStore";

const SWITCH: Record<OffsiteDomain, keyof Settings> = {
  containers: "containersEnabled",
  vms: "vmsEnabled",
  flash: "flashEnabled",
  files: "filesEnabled",
  zfs: "zfsEnabled",
  config: "configEnabled",
};

type Overrides = NonNullable<Settings["restoreFolderOverrides"]>;

/**
 * RestoreFoldersCard sets where a restore into a folder lands: one folder for
 * everything, a subfolder of it per section, and for single sections a folder
 * of their own. The server drops an empty folder, so a section stays listed
 * as an exception while its field is being filled.
 */
export function RestoreFoldersCard({ hueIndex }: { hueIndex: number }) {
  const { t } = useT();
  const { settings, setSettings, hostMountRoot, save, debouncedSave, cancelDebounce } = useSettings();
  const [, setSaveState] = useState<SaveState>("idle");
  const [, setSaveError] = useState<string | null>(null);
  const [opened, setOpened] = useState<OffsiteDomain[]>([]);
  const [picking, setPicking] = useState(false);
  const addButton = useRef<HTMLSpanElement>(null);

  const overrides: Overrides = settings.restoreFolderOverrides ?? {};
  const sections = FLOW_DOMAINS.filter((domain) => settings[SWITCH[domain]]);
  const own = FLOW_DOMAINS.filter((domain) => domain in overrides || opened.includes(domain));
  const free = sections.filter((domain) => !own.includes(domain));
  const base = settings.restoreFolder.trim();
  const perSection = settings.restoreFolderPerSection ?? false;

  function write(next: Overrides, now: boolean) {
    setSettings((prev) => (prev ? { ...prev, restoreFolderOverrides: next } : prev));
    const send = () => void save({ restoreFolderOverrides: next }, setSaveState, setSaveError);
    if (now) {
      cancelDebounce("restoreFolderOverrides");
      send();
    } else debouncedSave("restoreFolderOverrides", send);
  }

  function add(domain: OffsiteDomain) {
    setOpened((list) => [...list, domain]);
    const folder = settings.restoreFolders?.[domain];
    if (folder) write({ ...overrides, [domain]: folder }, true);
  }

  function drop(domain: OffsiteDomain) {
    setOpened((list) => list.filter((d) => d !== domain));
    if (!(domain in overrides)) return;
    const next = { ...overrides };
    delete next[domain];
    write(next, true);
  }

  const where = `${hostMountRoot}/${base}${perSection ? `/‹${t("tile.restore.slot")}›` : ""}`;
  const [before, after] = t(own.length > 0 ? "tile.restore.others" : "tile.restore.all").split("{where}");

  return (
    <Card title={t("tile.restore.folders")} hint={t("tile.restore.foldersHint")} hueIndex={hueIndex}>
      <FolderBrowser
        label={t("settings.restoreFolder")}
        value={settings.restoreFolder}
        hostMountRoot={hostMountRoot}
        hint={t("settings.restoreFolderHint")}
        onChange={(v) => {
          setSettings((prev) => (prev ? { ...prev, restoreFolder: v } : prev));
          debouncedSave("restoreFolder", () => void save({ restoreFolder: v }, setSaveState, setSaveError));
        }}
      />
      <ToggleRow
        label={t("tile.restore.perSection")}
        hint={t("tile.restore.perSectionHint")}
        checked={perSection}
        onChange={(v) => {
          setSettings((prev) => (prev ? { ...prev, restoreFolderPerSection: v } : prev));
          void save({ restoreFolderPerSection: v }, setSaveState, setSaveError);
        }}
      />
      {own.map((domain, i) => (
        <div key={domain} className="flex flex-col gap-1.5">
          <div className="flex items-center justify-between gap-2">
            <span className="flex items-center gap-2 text-sm text-carbon-text">
              <DomainGlyph domain={domain} />
              {t(DOMAIN_LABEL[domain])}
            </span>
            <Button
              label={t("tile.restore.removeException")}
              labelKey="tile.restore.removeException"
              glyph={<IconTrash />}
              glyphOnly
              tone="neutral"
              onClick={() => drop(domain)}
            />
          </div>
          <FolderBrowser
            label={t(DOMAIN_LABEL[domain])}
            renderLabel={false}
            value={overrides[domain] ?? ""}
            hostMountRoot={hostMountRoot}
            hueIndex={i}
            onChange={(v) => write({ ...overrides, [domain]: v }, false)}
          />
        </div>
      ))}
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <p className="min-w-0 text-xs text-carbon-textMuted">
          {base !== "" && (
            <>
              {before}
              <code dir="ltr" className="break-all font-mono">
                {where}
              </code>
              {after}
            </>
          )}
        </p>
        {free.length > 0 && (
          <span ref={addButton} className="flex">
            <Button
              label={t("tile.restore.exception")}
              labelKey="tile.restore.exception"
              glyph={<IconAdd />}
              tone="neutral"
              onClick={() => setPicking((was) => !was)}
            />
          </span>
        )}
      </div>
      {picking && addButton.current && (
        <ActionMenu
          label={t("tile.restore.exception")}
          actions={free.map((domain) => ({
            id: domain,
            label: t(DOMAIN_LABEL[domain]),
            glyph: <DomainGlyph domain={domain} />,
            onSelect: () => add(domain),
          }))}
          place={{ anchor: addButton.current }}
          onClose={() => setPicking(false)}
        />
      )}
    </Card>
  );
}
