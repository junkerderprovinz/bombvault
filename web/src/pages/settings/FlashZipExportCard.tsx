import type { Settings } from "../../lib/api";
import { Card, ToggleRow } from "../settings/shared";
import { FolderBrowser } from "../../components/FolderBrowser";
import { InfoBubble } from "../../components/InfoBubble";
import { NumberField } from "../../components/NumberField";
import { getSettings, putSettings } from "../../lib/api";
import { useEffect, useRef, useState } from "react";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";

// FlashZipExportCard sits on the Flash page, outside SettingsPage, so it loads
// its own settings and saves the way Config.tsx's ConfigSettingsCard does:
// re-fetch the latest settings, merge only this card's fields, put them back.
// Every field saves itself; the toggles flip optimistically and revert on
// failure, the path and count are debounced.
export function FlashZipExportCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { push } = useToast();
  const [hostMountRoot, setHostMountRoot] = useState<string>("/host/user");
  const [enabled, setEnabled] = useState(false);
  const [path, setPath] = useState("");
  const [keep, setKeep] = useState(0);
  // Turning history off zeroes the stored count; this brings the user's count
  // back when it is turned on again.
  const [rememberedKeep, setRememberedKeep] = useState(7);
  const [busyEnabled, setBusyEnabled] = useState(false);
  const [busyKeep, setBusyKeep] = useState(false);
  const [shakeEnabled, setShakeEnabled] = useState(0);
  const [shakeKeep, setShakeKeep] = useState(0);
  const [pulseEnabled, setPulseEnabled] = useState(0);
  const [pulseKeep, setPulseKeep] = useState(0);
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  const DEBOUNCE_MS = 800;

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (!res.ok) return;
        setEnabled(res.settings.flashZipExportEnabled);
        setPath(res.settings.flashZipExportPath);
        setKeep(res.settings.flashZipExportKeep);
        if (res.settings.flashZipExportKeep > 0) setRememberedKeep(res.settings.flashZipExportKeep);
        setHostMountRoot(res.hostMountRoot);
      })
      .catch(() => undefined);
  }, []);

  // persist merges the patch onto freshly fetched settings rather than a
  // mount-time snapshot, which could write back a field another page has
  // changed since. It reports success so a toggle can revert its flip.
  async function persist(patch: Partial<Settings>): Promise<boolean> {
    try {
      const latest = await getSettings();
      if (!latest.ok) {
        push(latest.error ?? t("settings.error"), "fail");
        return false;
      }
      const merged: Settings = { ...latest.settings, ...patch };
      const res = await putSettings(merged);
      if (res.ok) {
        push(t("settings.saved"), "success");
        return true;
      }
      push(res.error ?? t("settings.error"), "fail");
      return false;
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      return false;
    }
  }

  async function toggleEnabled(next: boolean) {
    const prev = enabled;
    setEnabled(next);
    setBusyEnabled(true);
    const ok = await persist({ flashZipExportEnabled: next });
    setBusyEnabled(false);
    if (!ok) {
      setEnabled(prev);
      setShakeEnabled((n) => n + 1);
    } else {
      setPulseEnabled((n) => n + 1);
    }
  }

  async function toggleKeepHistory(next: boolean) {
    const prev = keep;
    const nextKeep = next ? rememberedKeep : 0;
    // Otherwise a count typed just before the flip would be saved after it,
    // writing the old count back over keep=0.
    cancelKeepDebounce();
    setKeep(nextKeep);
    setBusyKeep(true);
    const ok = await persist({ flashZipExportKeep: nextKeep });
    setBusyKeep(false);
    if (!ok) {
      setKeep(prev);
      setShakeKeep((n) => n + 1);
    } else {
      setPulseKeep((n) => n + 1);
    }
  }

  function debounced(key: string, run: () => void) {
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing);
    debounceTimers.current[key] = setTimeout(run, DEBOUNCE_MS);
  }

  function cancelKeepDebounce() {
    const existing = debounceTimers.current.flashZipExportKeep;
    if (existing) {
      clearTimeout(existing);
      delete debounceTimers.current.flashZipExportKeep;
    }
  }

  return (
    <Card
      title={t("flash.zipExport.title")}
      hint={`${t("flash.zipExport.hint")} ${t("flash.zipExport.enableHint")}`}
      hueIndex={hueIndex}
    >
      <ToggleRow
        label={t("flash.zipExport.enable")}
        checked={enabled}
        onChange={(v) => void toggleEnabled(v)}
        disabled={busyEnabled}
        shakeNonce={shakeEnabled}
        pulseNonce={pulseEnabled}
      />
      {enabled && (
        <>
          <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
            {t("flash.zipExport.plaintextWarn")}
          </div>
          <FolderBrowser
            label={t("flash.zipExport.path")}
            value={path}
            hostMountRoot={hostMountRoot}
            hint={t("flash.zipExport.pathHint")}
            onChange={(v) => {
              setPath(v);
              debounced("flashZipExportPath", () => void persist({ flashZipExportPath: v }));
            }}
          />
          {!path.trim() && (
            <p className="text-xs text-statusFail -mt-1">{t("flash.zipExport.pathRequired")}</p>
          )}
          <ToggleRow
            label={t("flash.zipExport.keepHistory")}
            hint={t("flash.zipExport.keepHistoryHint")}
            // History is "on" whenever we keep more than a single overwritten zip.
            checked={keep > 0}
            onChange={(v) => void toggleKeepHistory(v)}
            disabled={busyKeep}
            shakeNonce={shakeKeep}
            pulseNonce={pulseKeep}
          />
          {keep > 0 ? (
            <label className="flex flex-col gap-1 max-w-40">
              <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                {t("flash.zipExport.keepN")}
                <InfoBubble tip={t("flash.zipExport.keepNHint")} />
              </span>
              <NumberField
                min={1}
                value={keep}
                onChange={(e) => {
                  const n = Math.max(1, parseInt(e.target.value, 10) || 1);
                  setRememberedKeep(n);
                  setKeep(n);
                  debounced("flashZipExportKeep", () => void persist({ flashZipExportKeep: n }));
                }}
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
              />
            </label>
          ) : (
            <p className="text-xs text-carbon-textMuted">{t("flash.zipExport.latestNote")}</p>
          )}
        </>
      )}
    </Card>
  );
}
