// ZFS backs up a dataset together with the datasets below it, from one
// recursive snapshot taken on the host over SSH.

import { useCallback, useEffect, useState, type CSSProperties } from "react";

import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { InfoBubble } from "../components/InfoBubble";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { IconZFS } from "../components/navGlyphs";
import { ZFSConnectionCard } from "../components/zfs/ZFSConnectionCard";
import { ZFSDatasetRow } from "../components/zfs/ZFSDatasetRow";
import { backupZFSAll, discoverZFS, listZFSDatasets, zfsHostDatasets } from "../lib/api";
import type { ZFSDatasetView } from "../lib/api";
import { hueVars } from "../lib/appearance";
import { BULK_HUE } from "../lib/bulkHue";
import { useT } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { anyActive, busyPhraseKey, useProgress } from "../lib/progress";
import { useToast } from "../lib/toast";

export function ZFS() {
  const { t } = useT();
  const { push } = useToast();
  const running = anyActive(useProgress());
  const [items, setItems] = useState<ZFSDatasetView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notInItem, setNotInItem] = useState(0);
  const [unusedZvols, setUnusedZvols] = useState(0);
  const [discovering, setDiscovering] = useState(false);
  const [shakeDiscover, setShakeDiscover] = useState(0);
  const [backupAllBusy, setBackupAllBusy] = useState(false);
  const [shakeBackupAll, setShakeBackupAll] = useState(0);

  const loadItems = useCallback(() => {
    return listZFSDatasets()
      .then((res) => {
        if (res.ok) {
          setItems((res.datasets ?? []).slice().sort((a, b) => a.dataset.localeCompare(b.dataset)));
          setError(null);
        } else setError(res.error ?? t("zfs.loadFailed"));
      })
      .catch(() => setError(t("zfs.loadFailed")));
  }, [t]);

  useEffect(() => {
    void loadItems().finally(() => setLoading(false));
    // The host listing is the only source for what is not in an item yet. It
    // reaches the server over SSH, so a failure leaves the counts at zero and
    // the page renders without them.
    zfsHostDatasets()
      .then((res) => {
        if (!res.ok || !res.available) return;
        setNotInItem(res.notInItem);
        setUnusedZvols(res.unusedZvols);
      })
      .catch(() => undefined);
  }, [loadItems]);

  async function handleDiscover() {
    setDiscovering(true);
    try {
      const res = await discoverZFS();
      if (res.skipped?.length) {
        push(t("common.discoverSkipped").replace("{list}", res.skipped.join(", ")), "warn");
      }
      if (res.ok) {
        push(t("zfs.discovered").replace("{n}", String(res.discovered ?? 0)), "success");
      } else {
        push(res.error ?? t("common.discoverFailed"), "fail");
        setShakeDiscover((n) => n + 1);
      }
      await loadItems();
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.discoverFailed"), "fail");
      setShakeDiscover((n) => n + 1);
    } finally {
      setDiscovering(false);
    }
  }

  const backupableIds = items.filter((i) => i.enabled).map((i) => i.id);

  async function handleBackupAll() {
    setBackupAllBusy(true);
    try {
      const res = await backupZFSAll(backupableIds);
      if (res.ok) {
        push(t("containers.batchStarted"), "success");
      } else {
        push(res.error ?? t("settings.error"), "fail");
        setShakeBackupAll((n) => n + 1);
      }
    } finally {
      setBackupAllBusy(false);
    }
  }

  const showEmptyState = !loading && error === null && items.length === 0;

  return (
    <div className={PAGE_SHELL}>
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">{t("zfs.title")}</h1>
          <p className="mt-1 text-sm text-carbon-textSub">{t("zfs.subtitle")}</p>
          <div className="mt-2"><OffsiteIndicator domain="zfs" /></div>
        </div>
        <div className="flex items-center gap-2 shrink-0 flex-wrap">
          <Button
            key={shakeDiscover}
            label={t("containers.discover")}
            labelKey="containers.discover"
            tone="neutral"
            onClick={() => void handleDiscover()}
            disabled={discovering}
            busy={discovering}
            title={t("zfs.discoverHint")}
            className={shakeDiscover ? "glim-shake" : ""}
          />
          {!showEmptyState && (
            <Button
              key={shakeBackupAll}
              label={t("zfs.backupAll")}
              labelKey="zfs.backupAll"
              hueIndex={BULK_HUE.backup}
              tone="neutral"
              onClick={() => void handleBackupAll()}
              disabled={backupAllBusy || running.active || backupableIds.length === 0}
              className={shakeBackupAll ? "glim-shake" : ""}
            />
          )}
          {!backupAllBusy && running.active && (
            <span className="text-xs text-carbon-textMuted">{t(busyPhraseKey(running.phase))}</span>
          )}
        </div>
      </div>

      <ZFSConnectionCard />

      {loading && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {error !== null && <p className="text-sm text-statusFail">{error}</p>}

      {showEmptyState && (
        <div
          className="relative glim-notch-card glim-hue bg-carbon-surface rounded-card p-6 text-center flex flex-col items-center gap-3"
          style={hueVars(0) as CSSProperties}
        >
          <h2 className="flex items-center">
            <Badge tone="heading" size="heading" wrap hueIndex={0} insetStart={6}>
              {t("zfs.emptyTitle")}
              <InfoBubble tip={t("zfs.empty")} onAccent />
            </Badge>
          </h2>
          <EmptyStateIcon icon={IconZFS} />
        </div>
      )}

      {items.length > 0 && (
        <div className="flex flex-col gap-3 glim-content-fade">
          {items.map((item, i) => (
            <ZFSDatasetRow key={item.id} item={item} t={t} onRefresh={() => void loadItems()} index={i} />
          ))}
        </div>
      )}

      {notInItem > 0 && (
        <p className="text-xs text-carbon-textMuted">
          {t("zfs.notInItem").replace("{n}", String(notInItem))}
        </p>
      )}
      {unusedZvols > 0 && (
        <p className="flex items-center gap-1.5 text-xs text-carbon-textMuted">
          {t("zfs.unusedZvols").replace("{n}", String(unusedZvols))}
          <InfoBubble tip={t("zfs.unusedZvolsHint")} />
        </p>
      )}
    </div>
  );
}
