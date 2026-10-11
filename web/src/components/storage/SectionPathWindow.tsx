import { useEffect, useRef, useState } from "react";
import { getSettings, putSettings, type OffsiteDomain, type OkEnvelope, type SaveWarning, type Settings } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { pushSaveWarnings } from "../../lib/placementCodes";
import { useToast } from "../../lib/toast";
import { useDebouncedSave } from "../../lib/useDebouncedSave";
import { storageLocationsChanged } from "../../lib/useStorageLocations";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { IconCheck } from "../navGlyphs";
import { PathModeSwitch } from "../PathModeSwitch";

type PathKey = "containersPath" | "vmsPath" | "flashPath" | "filesPath" | "zfsPath" | "configPath";

const PATH: Record<OffsiteDomain, { key: PathKey; label: TranslationKey }> = {
  containers: { key: "containersPath", label: "settings.containersPath" },
  vms: { key: "vmsPath", label: "settings.vmsPath" },
  flash: { key: "flashPath", label: "settings.flashPath" },
  files: { key: "filesPath", label: "settings.filesPath" },
  zfs: { key: "zfsPath", label: "settings.zfsPath" },
  config: { key: "configPath", label: "settings.configPath" },
};

type Answer = OkEnvelope & { warnings?: SaveWarning[] };

/**
 * SectionPathWindow sets where one section's own backups are written: a
 * folder on this host or the address of a remote repository, with what a
 * remote one needs to sign in. A typed path saves itself after a pause, and
 * what is still waiting is saved when the window closes.
 */
export function SectionPathWindow({ domain, onClose }: { domain: OffsiteDomain; onClose: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const { key, label } = PATH[domain];
  const [settings, setSettings] = useState<Settings | null>(null);
  const [root, setRoot] = useState("");
  // The settings as the server holds them. Every save sends them whole, so a
  // path that is still being typed never rides along with another field.
  const stored = useRef<Settings | null>(null);
  const waiting = useRef<(() => Promise<boolean>) | null>(null);
  const { debouncedSave, cancel } = useDebouncedSave();

  useEffect(() => {
    let alive = true;
    void getSettings()
      .then((res) => {
        if (!alive || !res.ok) return;
        stored.current = res.settings;
        setSettings(res.settings);
        setRoot(res.hostMountRoot);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  async function save(patch: Partial<Settings>): Promise<boolean> {
    if (!stored.current) return false;
    const next = { ...stored.current, ...patch };
    const res: Answer = await putSettings(next).catch((e: unknown) => ({ ok: false, error: e instanceof Error ? e.message : undefined }));
    if (!res.ok) {
      push(res.error ?? t("settings.error"), "fail");
      return false;
    }
    stored.current = next;
    push(t("settings.saved"), "success");
    pushSaveWarnings(push, t, res.warnings);
    return true;
  }

  function type(path: string) {
    setSettings((prev) => (prev ? { ...prev, [key]: path } : prev));
    waiting.current = () => {
      waiting.current = null;
      return save({ [key]: path });
    };
    debouncedSave(() => void waiting.current?.());
  }

  function close() {
    cancel();
    void Promise.resolve(waiting.current?.()).then(storageLocationsChanged);
    onClose();
  }

  return (
    <InstanceWindow
      title={t(label)}
      hint={t("settings.pathsHint").replace("{root}", () => root)}
      onClose={close}
      footer={<Button label={t("common.done")} labelKey="common.done" glyph={<IconCheck />} tone="accent" onClick={close} />}
    >
      {settings ? (
        <PathModeSwitch
          label={t(label)}
          domain={domain}
          value={settings[key]}
          hostMountRoot={root}
          onChange={type}
          settings={settings}
          setSettings={setSettings}
          save={save}
        />
      ) : (
        <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
      )}
    </InstanceWindow>
  );
}
