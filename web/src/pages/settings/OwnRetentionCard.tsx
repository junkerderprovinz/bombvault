import { useState } from "react";

import { InfoBubble } from "../../components/InfoBubble";
import { NumberField } from "../../components/NumberField";
import type { OffsiteDomain, RetentionKeep, Settings } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { Card, ToggleRow } from "./shared";

// The order and names of the off-site page, so a source reads the same on both.
const SOURCES = [
  { domain: "containers", labelKey: "nav.containers" },
  { domain: "vms", labelKey: "nav.vms" },
  { domain: "flash", labelKey: "nav.flash" },
  { domain: "files", labelKey: "nav.files" },
  { domain: "zfs", labelKey: "nav.zfs" },
  { domain: "config", labelKey: "nav.config" },
] as const satisfies readonly { domain: OffsiteDomain; labelKey: string }[];

const RULES = [
  ["keepLast", "settings.retentionLast", "settings.retentionLastInfo"],
  ["keepDaily", "settings.retentionDaily", "settings.retentionDailyInfo"],
  ["keepWeekly", "settings.retentionWeekly", "settings.retentionWeeklyInfo"],
  ["keepMonthly", "settings.retentionMonthly", "settings.retentionMonthlyInfo"],
  ["keepYearly", "settings.retentionYearly", "settings.retentionYearlyInfo"],
] as const;

type Own = Settings["ownRetention"];

/** The shared policy, which a source starts from when it gets its own. */
function sharedKeep(s: Settings): RetentionKeep {
  return {
    keepLast: s.retentionKeepLast,
    keepDaily: s.retentionKeepDaily,
    keepWeekly: s.retentionKeepWeekly,
    keepMonthly: s.retentionKeepMonthly,
    keepYearly: s.retentionKeepYearly,
  };
}

/**
 * Lets each source age by a keep-policy of its own instead of the shared one.
 * The whole map is saved on every change, so the debounce runs under one key
 * and a toggle drops the edit it would otherwise race.
 */
export function OwnRetentionCard({
  settings,
  setSettings,
  save,
  debouncedSave,
  cancelDebounce,
  t,
  hueIndex,
}: {
  settings: Settings;
  setSettings: React.Dispatch<React.SetStateAction<Settings | null>>;
  save: (patch: Partial<Settings>, setState: () => void, setError: () => void) => Promise<boolean>;
  debouncedSave: (key: string, run: () => void) => void;
  cancelDebounce: (key: string) => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const [busy, setBusy] = useState<Partial<Record<OffsiteDomain, boolean>>>({});
  const [shake, setShake] = useState<Partial<Record<OffsiteDomain, number>>>({});
  const [pulse, setPulse] = useState<Partial<Record<OffsiteDomain, number>>>({});
  const own = settings.ownRetention;

  const persist = (next: Own) => save({ ownRetention: next }, () => undefined, () => undefined);

  async function toggle(domain: OffsiteDomain, on: boolean) {
    cancelDebounce("ownRetention");
    const before = own;
    const next: Own = { ...own };
    if (on) next[domain] = sharedKeep(settings);
    else delete next[domain];
    setSettings((prev) => (prev ? { ...prev, ownRetention: next } : prev));
    setBusy((b) => ({ ...b, [domain]: true }));
    const ok = await persist(next);
    setBusy((b) => ({ ...b, [domain]: false }));
    if (ok) {
      setPulse((p) => ({ ...p, [domain]: (p[domain] ?? 0) + 1 }));
      return;
    }
    setSettings((prev) => (prev ? { ...prev, ownRetention: before } : prev));
    setShake((s) => ({ ...s, [domain]: (s[domain] ?? 0) + 1 }));
  }

  function edit(domain: OffsiteDomain, keep: RetentionKeep) {
    const next: Own = { ...own, [domain]: keep };
    setSettings((prev) => (prev ? { ...prev, ownRetention: next } : prev));
    debouncedSave("ownRetention", () => void persist(next));
  }

  return (
    <Card title={t("settings.ownRetentionTitle")} hint={t("settings.ownRetentionHint")} hueIndex={hueIndex}>
      {SOURCES.map(({ domain, labelKey }, i) => {
        const keep = own[domain];
        return (
          <div key={domain} className="flex flex-col gap-3">
            <ToggleRow
              label={t("settings.ownRetentionFor").replace("{source}", t(labelKey))}
              hint={t("settings.ownRetentionToggleHint")}
              checked={keep !== undefined}
              onChange={(v) => void toggle(domain, v)}
              disabled={busy[domain]}
              shakeNonce={shake[domain]}
              pulseNonce={pulse[domain]}
              hueIndex={i}
            />
            {keep && (
              <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
                {RULES.map(([key, label, info]) => (
                  <label key={key} className="flex flex-col gap-1">
                    <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                      {t(label)}
                      <InfoBubble tip={t(info)} />
                    </span>
                    <NumberField
                      min={0}
                      value={keep[key]}
                      onChange={(e) => edit(domain, { ...keep, [key]: Math.max(0, parseInt(e.target.value, 10) || 0) })}
                      className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
                    />
                  </label>
                ))}
              </div>
            )}
          </div>
        );
      })}
    </Card>
  );
}
