import { useEffect, useState } from "react";

import { Selector } from "./Selector";
import { Rows, SettingRow } from "./storage/rows";
import {
  ANOMALY_NOTIFY_LABEL,
  ANOMALY_SENSITIVITY_LABEL,
  anomalyErrorText,
  type TranslateAnomaly,
} from "../lib/anomalies";
import { getSettings, setItemAnomalyPrefs, type AnomalyItem } from "../lib/api";
import { useToast } from "../lib/toast";

// The segment that stands for "no value of its own", which the server stores as "".
const FOLLOW = "follow";

/** The global preset and notification minimum an item follows by default. */
export type AnomalyGlobals = { sensitivity: string; notifyMin: string };

type Props = {
  item?: AnomalyItem;
  enabled: boolean;
  /** Passed by a caller that already holds the settings; read here otherwise. */
  globals?: AnomalyGlobals;
  /** The caller lists the two settings among rows of its own. */
  bare?: boolean;
  t: TranslateAnomaly;
};

/**
 * ItemAnomalySettings is how closely one item is watched: the sensitivity and
 * the notification minimum it uses instead of the global ones. It is absent
 * for an item the engine does not know yet and while detection is off, since
 * an editor for a switched-off feature changes nothing anyone can see.
 */
export function ItemAnomalySettings({ item, enabled, globals, bare = false, t }: Props) {
  if (!item || !enabled) return null;
  const fields = globals ? <PrefsFields item={item} globals={globals} t={t} /> : <PrefsWithGlobals item={item} t={t} />;
  return bare ? fields : <Rows>{fields}</Rows>;
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

  const follow = t("anomaly.items.followGlobal");
  const sensitivityItems = [
    {
      id: FOLLOW,
      label: follow,
      tip: t("anomaly.sensitivity.follow").replace(
        "{preset}",
        t(ANOMALY_SENSITIVITY_LABEL[globals.sensitivity] ?? "anomaly.sensitivity.balanced")
      ),
    },
    ...Object.entries(ANOMALY_SENSITIVITY_LABEL).map(([id, key]) => ({ id, label: t(key) })),
  ];
  const notifyItems = [
    {
      id: FOLLOW,
      label: follow,
      tip: t("anomaly.items.notifyFollow").replace(
        "{value}",
        t(ANOMALY_NOTIFY_LABEL[globals.notifyMin] ?? "anomaly.settings.notify.critical")
      ),
    },
    ...Object.entries(ANOMALY_NOTIFY_LABEL).map(([id, key]) => ({ id, label: t(key) })),
  ];
  const stored = (id: string) => (id === FOLLOW ? "" : id);

  return (
    <>
      <SettingRow label={t("anomaly.items.sensitivity")} hint={t("anomaly.items.sensitivityHint")}>
        <Selector
          items={sensitivityItems}
          label={t("anomaly.items.sensitivity")}
          select="one"
          activation="manual"
          inline
          pairsOnPhone
          active={sensitivity || FOLLOW}
          onChange={(id) => {
            const before = sensitivity;
            setSensitivity(stored(id));
            void save({ sensitivity: stored(id) }, () => setSensitivity(before));
          }}
        />
      </SettingRow>
      <SettingRow label={t("anomaly.items.notifyMin")}>
        <Selector
          items={notifyItems}
          label={t("anomaly.items.notifyMin")}
          select="one"
          activation="manual"
          inline
          pairsOnPhone
          active={notifyMin || FOLLOW}
          onChange={(id) => {
            const before = notifyMin;
            setNotifyMin(stored(id));
            void save({ notifyMin: stored(id) }, () => setNotifyMin(before));
          }}
        />
      </SettingRow>
    </>
  );
}
