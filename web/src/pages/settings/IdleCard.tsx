import { useEffect, useRef, useState } from "react";
import { InfoBubble } from "../../components/InfoBubble";
import { NumberField } from "../../components/NumberField";
import { getIdle, setIdle } from "../../lib/api";
import type { IdleSettings } from "../../lib/api";
import type { TranslationKey, useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { Card } from "./shared";

const DEBOUNCE_MS = 800;

const FIELDS: { key: keyof IdleSettings; label: TranslationKey; hint?: TranslationKey; max: number }[] = [
  { key: "cpuPct", label: "idle.cpu", hint: "idle.cpuHint", max: 3200 },
  { key: "netMbit", label: "idle.net", max: 10000 },
  { key: "quietMin", label: "idle.quiet", hint: "idle.quietHint", max: 60 },
];

// IdleCard says when an app counts as idle for a container whose scheduled
// backup waits for that.
export function IdleCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { push } = useToast();
  const [cfg, setCfg] = useState<IdleSettings | null>(null);
  const cfgRef = useRef<IdleSettings | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    getIdle()
      .then((res) => {
        if (!res.ok || !res.settings) return;
        cfgRef.current = res.settings;
        setCfg(res.settings);
      })
      .catch(() => undefined);
  }, []);

  async function save() {
    if (!cfgRef.current) return;
    try {
      const res = await setIdle(cfgRef.current);
      push(res.ok ? t("settings.saved") : (res.error ?? t("settings.error")), res.ok ? "success" : "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  // A value typed just before leaving the tab is saved, not dropped.
  useEffect(
    () => () => {
      if (timer.current && cfgRef.current) {
        clearTimeout(timer.current);
        void setIdle(cfgRef.current).catch(() => undefined);
      }
    },
    []
  );

  function change(key: keyof IdleSettings, value: number) {
    if (!cfgRef.current) return;
    const next = { ...cfgRef.current, [key]: value };
    cfgRef.current = next;
    setCfg(next);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      timer.current = null;
      void save();
    }, DEBOUNCE_MS);
  }

  if (!cfg) return null;

  return (
    <Card title={t("idle.title")} hint={t("idle.hint")} hueIndex={hueIndex}>
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        {FIELDS.map(({ key, label, hint, max }) => (
          <label key={key} className="flex flex-col gap-1">
            <span className="flex items-center gap-1 text-xs text-carbon-textSub">
              {t(label)}
              {hint && <InfoBubble tip={t(hint)} />}
            </span>
            <NumberField
              min={1}
              max={max}
              value={cfg[key]}
              onChange={(e) => change(key, Math.min(max, Math.max(1, parseInt(e.target.value, 10) || 1)))}
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
            />
          </label>
        ))}
      </div>
    </Card>
  );
}
