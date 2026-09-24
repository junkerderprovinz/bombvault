import { useEffect, useState } from "react";

import { LabelledSelect } from "./SelectField";
import {
  ANOMALY_NOTIFY_LABEL,
  ANOMALY_SENSITIVITY_LABEL,
  anomalyErrorText,
  type TranslateAnomaly,
} from "../lib/anomalies";
import { getSettings, setItemAnomalyPrefs, type AnomalyItem } from "../lib/api";
import { useToast } from "../lib/toast";

/** The global preset and notification minimum an item follows by default. */
export type AnomalyGlobals = { sensitivity: string; notifyMin: string };

type Props = {
  item?: AnomalyItem;
  enabled: boolean;
  /** Passed by a caller that already holds the settings; read here otherwise. */
  globals?: AnomalyGlobals;
  t: TranslateAnomaly;
};

/**
 * ItemAnomalySettings is how closely one item is watched: the sensitivity and
 * the notification minimum it uses instead of the global ones. It is absent
 * for an item the engine does not know yet and while detection is off, since
 * an editor for a switched-off feature changes nothing anyone can see.
 */
export function ItemAnomalySettings({ item, enabled, globals, t }: Props) {
  if (!item || !enabled) return null;
  if (globals) return <PrefsFields item={item} globals={globals} t={t} />;
  return <PrefsWithGlobals item={item} t={t} />;
}

function PrefsWithGlobals({ item, t }: { item: AnomalyItem; t: TranslateAnomaly }) {
  const [globals, setGlobals] = useState<AnomalyGlobals>({ sensitivity: "balanced", notifyMin: "critical" });
  useEffect(() => {
    let active = true;
    getSettings()
      .then((res) => {
        if (active && res.ok && res.settings) {
          setGlobals({ sensitivity: res.settings.anomalySensitivity, notifyMin: res.settings.anomalyNotifyMin });
        }
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);
  return <PrefsFields item={item} globals={globals} t={t} />;
}

function PrefsFields({ item, globals, t }: { item: AnomalyItem; globals: AnomalyGlobals; t: TranslateAnomaly }) {
  const { push } = useToast();
  const [sensitivity, setSensitivity] = useState(item.sensitivity);
  const [notifyMin, setNotifyMin] = useState(item.notifyMin);

  // Both fields save on their own, and a refusal puts the stored value back,
  // so the control never shows a setting the server does not hold.
  async function save(patch: { sensitivity?: string; notifyMin?: string }, revert: () => void) {
    try {
      const res = await setItemAnomalyPrefs(item.targetId, patch);
      if (res.ok) return;
      push(anomalyErrorText(res.code, t), "fail");
    } catch {
      push(anomalyErrorText(undefined, t), "fail");
    }
    revert();
  }

  const sensitivityOptions = [
    {
      value: "",
      label: t("anomaly.sensitivity.follow").replace(
        "{preset}",
        t(ANOMALY_SENSITIVITY_LABEL[globals.sensitivity] ?? "anomaly.sensitivity.balanced")
      ),
    },
    ...Object.entries(ANOMALY_SENSITIVITY_LABEL).map(([value, key]) => ({ value, label: t(key) })),
  ];
  const notifyOptions = [
    {
      value: "",
      label: t("anomaly.items.notifyFollow").replace(
        "{value}",
        t(ANOMALY_NOTIFY_LABEL[globals.notifyMin] ?? "anomaly.settings.notify.critical")
      ),
    },
    ...Object.entries(ANOMALY_NOTIFY_LABEL).map(([value, key]) => ({ value, label: t(key) })),
  ];

  return (
    <div className="flex flex-wrap items-end gap-3">
      <LabelledSelect
        label={t("anomaly.items.sensitivity")}
        value={sensitivity}
        onChange={(next: string) => {
          const before = sensitivity;
          setSensitivity(next);
          void save({ sensitivity: next }, () => setSensitivity(before));
        }}
        options={sensitivityOptions}
      />
      <LabelledSelect
        label={t("anomaly.items.notifyMin")}
        value={notifyMin}
        onChange={(next: string) => {
          const before = notifyMin;
          setNotifyMin(next);
          void save({ notifyMin: next }, () => setNotifyMin(before));
        }}
        options={notifyOptions}
      />
    </div>
  );
}
