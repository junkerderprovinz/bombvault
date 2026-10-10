// One item's monitoring in a window, reached from a learning ring or from a
// finding of that item, which then offers the way back.

import { useNavigate } from "react-router-dom";

import { Button } from "../Button";
import { IconBack, IconEye } from "../glyphs";
import type { AnomalyGlobals } from "../ItemAnomalySettings";
import { anomalyDomainsLabel, anomalyItemPath, type TranslateAnomaly } from "../../lib/anomalies";
import type { AnomalyItem, AnomalyView } from "../../lib/api";
import { AnomalyWindow } from "./AnomalyWindow";
import { ItemMonitoring } from "./ItemMonitoring";

export function MonitoringWindow({
  item,
  t,
  globals,
  enabled,
  finding,
  onBack,
  onClose,
}: {
  item: AnomalyItem;
  t: TranslateAnomaly;
  globals: AnomalyGlobals;
  enabled: boolean;
  /** The finding the curve follows: the one this window was opened from, or
   *  the item's most pressing one. */
  finding?: AnomalyView;
  /** Returns to the finding this window was opened from. */
  onBack?: () => void;
  onClose: () => void;
}) {
  const navigate = useNavigate();
  const itemPath = anomalyItemPath(item);

  return (
    <AnomalyWindow
      title={t("anomaly.card.monitoring")}
      onClose={onClose}
      actions={
        <>
          {onBack && <Button label={t("common.back")} labelKey="common.back" glyph={<IconBack />} onClick={onBack} />}
          {itemPath && (
            <Button
              label={t("anomaly.openItem")}
              labelKey="anomaly.openItem"
              glyph={<IconEye />}
              onClick={() => {
                onClose();
                navigate(itemPath);
              }}
            />
          )}
        </>
      }
    >
      <p className="text-sm text-carbon-textMuted wrap-anywhere">{item.name || anomalyDomainsLabel(item.domain, t)}</p>
      <ItemMonitoring t={t} item={item} globals={globals} enabled={enabled} finding={finding} />
    </AnomalyWindow>
  );
}
