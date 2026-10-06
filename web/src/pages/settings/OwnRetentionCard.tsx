import { useState } from "react";
import { Link } from "react-router-dom";

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

export type RetentionScope = "local" | "offsite";

// What each section reads and writes, and the words it uses.
const SCOPES = {
  local: {
    shared: {
      keepLast: "retentionKeepLast",
      keepDaily: "retentionKeepDaily",
      keepWeekly: "retentionKeepWeekly",
      keepMonthly: "retentionKeepMonthly",
      keepYearly: "retentionKeepYearly",
    },
    own: "ownRetention",
    title: "settings.retentionLocalTitle",
    hints: ["settings.retentionHint", "settings.retentionCombineInfo"],
    perSourceHint: "settings.ownRetentionHint",
    toggle: "settings.ownRetentionFor",
    toggleHint: "settings.ownRetentionToggleHint",
  },
  offsite: {
    shared: {
      keepLast: "offsiteRetentionKeepLast",
      keepDaily: "offsiteRetentionKeepDaily",
      keepWeekly: "offsiteRetentionKeepWeekly",
      keepMonthly: "offsiteRetentionKeepMonthly",
      keepYearly: "offsiteRetentionKeepYearly",
    },
    own: "ownOffsiteRetention",
    title: "settings.retentionOffsiteTitle",
    hints: ["settings.retentionOffsiteHint", "settings.retentionCombineInfo", "settings.retentionImmutableNotPruned"],
    perSourceHint: "settings.ownOffsiteRetentionHint",
    toggle: "settings.ownOffsiteRetentionFor",
    toggleHint: "settings.ownOffsiteRetentionToggleHint",
  },
} as const;

type Own = Settings["ownRetention"];
type T = ReturnType<typeof useT>["t"];

/** The shared policy of a section, which a source starts from when it gets
 *  its own. */
function sharedKeep(s: Settings, scope: RetentionScope): RetentionKeep {
  const keys = SCOPES[scope].shared;
  return {
    keepLast: s[keys.keepLast],
    keepDaily: s[keys.keepDaily],
    keepWeekly: s[keys.keepWeekly],
    keepMonthly: s[keys.keepMonthly],
    keepYearly: s[keys.keepYearly],
  };
}

/** The five keep rules as number fields, each with its (i). */
function KeepFields({ keep, onChange, t }: { keep: RetentionKeep; onChange: (k: RetentionKeep) => void; t: T }) {
  return (
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
            onChange={(e) => onChange({ ...keep, [key]: Math.max(0, parseInt(e.target.value, 10) || 0) })}
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
          />
        </label>
      ))}
    </div>
  );
}

/**
 * The keep rules of the local or the off-site copies: the shared ones, and
 * below them a switch per source that gives it rules of its own. A source's
 * map is saved whole on every change, so its debounce runs under one key and
 * a switch drops the edit it would otherwise race. children stand under the
 * switches.
 */
export function RetentionRulesCard({
  scope,
  settings,
  setSettings,
  save,
  debouncedSave,
  cancelDebounce,
  t,
  hueIndex,
  children,
}: {
  scope: RetentionScope;
  settings: Settings;
  setSettings: React.Dispatch<React.SetStateAction<Settings | null>>;
  save: (patch: Partial<Settings>) => Promise<boolean>;
  debouncedSave: (key: string, run: () => void) => void;
  cancelDebounce: (key: string) => void;
  t: T;
  hueIndex?: number;
  children?: React.ReactNode;
}) {
  const [busy, setBusy] = useState<Partial<Record<OffsiteDomain, boolean>>>({});
  const [shake, setShake] = useState<Partial<Record<OffsiteDomain, number>>>({});
  const [pulse, setPulse] = useState<Partial<Record<OffsiteDomain, number>>>({});
  const text = SCOPES[scope];
  const ownKey = text.own;
  const own = settings[ownKey];

  function editShared(keep: RetentionKeep) {
    const patch: Partial<Settings> = {};
    for (const [rule, field] of Object.entries(text.shared) as [keyof RetentionKeep, keyof Settings][]) {
      if (keep[rule] === settings[field]) continue;
      Object.assign(patch, { [field]: keep[rule] });
      // Keyed by field name, so typing in one cell never resets another
      // cell's pending save.
      debouncedSave(field, () => void save({ [field]: keep[rule] }));
    }
    setSettings((prev) => (prev ? { ...prev, ...patch } : prev));
  }

  async function toggle(domain: OffsiteDomain, on: boolean) {
    cancelDebounce(ownKey);
    const before = own;
    const next: Own = { ...own };
    if (on) next[domain] = sharedKeep(settings, scope);
    else delete next[domain];
    setSettings((prev) => (prev ? { ...prev, [ownKey]: next } : prev));
    setBusy((b) => ({ ...b, [domain]: true }));
    const ok = await save({ [ownKey]: next });
    setBusy((b) => ({ ...b, [domain]: false }));
    if (ok) {
      setPulse((p) => ({ ...p, [domain]: (p[domain] ?? 0) + 1 }));
      return;
    }
    setSettings((prev) => (prev ? { ...prev, [ownKey]: before } : prev));
    setShake((s) => ({ ...s, [domain]: (s[domain] ?? 0) + 1 }));
  }

  function editOwn(domain: OffsiteDomain, keep: RetentionKeep) {
    const next: Own = { ...own, [domain]: keep };
    setSettings((prev) => (prev ? { ...prev, [ownKey]: next } : prev));
    debouncedSave(ownKey, () => void save({ [ownKey]: next }));
  }

  return (
    <Card title={t(text.title)} hint={text.hints.map((k) => t(k)).join(" ")} hueIndex={hueIndex}>
      <span className="text-sm text-carbon-text">{t("retentionPreview.sharedPolicy")}</span>
      <KeepFields keep={sharedKeep(settings, scope)} onChange={editShared} t={t} />

      <span className="mt-2 flex items-center gap-1 text-sm text-carbon-text">
        {t("settings.ownRetentionTitle")}
        <InfoBubble tip={t(text.perSourceHint)} />
      </span>
      {SOURCES.map(({ domain, labelKey }, i) => {
        const keep = own[domain];
        return (
          <div key={domain} className="flex flex-col gap-3">
            <ToggleRow
              label={t(labelKey)}
              accessibleName={t(text.toggle).replace("{source}", t(labelKey))}
              hint={t(text.toggleHint)}
              checked={keep !== undefined}
              onChange={(v) => void toggle(domain, v)}
              disabled={busy[domain]}
              shakeNonce={shake[domain]}
              pulseNonce={pulse[domain]}
              hueIndex={i}
            />
            {keep && <KeepFields keep={keep} onChange={(k) => editOwn(domain, k)} t={t} />}
          </div>
        );
      })}

      {children}
      {scope === "offsite" && <p className="text-sm text-carbon-textSub">{t("settings.retentionOffsiteScope")}</p>}
      {scope === "offsite" && (
        <Link
          to="/settings/offsite"
          className="w-fit text-sm text-accentText hover:underline pointer-coarse:inline-flex pointer-coarse:min-h-(--btn-h) pointer-coarse:items-center"
        >
          {t("settings.retentionExtraTargets")}
        </Link>
      )}
    </Card>
  );
}
