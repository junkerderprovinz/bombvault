import { useEffect, useState } from "react";
import { deleteBackupsZFSDataset, listContainers, patchZFSDataset, probeZFSDataset } from "../../lib/api";
import type { AnomalyItem, AnomalySeriesInfo, Container, ZFSDatasetPatch, ZFSDatasetView } from "../../lib/api";
import { useAdvanced } from "../../lib/advanced";
import { useT } from "../../lib/i18n";
import { useConfirm } from "../../lib/useConfirm";
import { useDebouncedSave } from "../../lib/useDebouncedSave";
import { useToast } from "../../lib/toast";
import { zfsCodeSentence } from "../../lib/zfsCodes";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import { ItemAnomalySettings } from "../ItemAnomalySettings";
import { RepoPicker } from "../RepoPicker";
import { SelectField } from "../SelectField";
import { ZFSMemberList } from "./ZFSMemberList";
import { ReplicaCard } from "./replica/ReplicaCard";
import { failText } from "./failText";
import { ZFSExcludesEditor } from "./ZFSExcludesEditor";

type T = ReturnType<typeof useT>["t"];

export function ZFSItemSettings({
  item,
  t,
  onChanged,
  anomaly,
  anomalyEnabled,
  series,
}: {
  item: ZFSDatasetView;
  t: T;
  onChanged: () => void;
  anomaly?: AnomalyItem;
  anomalyEnabled: boolean;
  series: ReadonlyMap<string, AnomalySeriesInfo>;
}) {
  const { advanced } = useAdvanced();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [excluded, setExcluded] = useState(new Set(item.excludedChildren));
  const [busy, setBusy] = useState(false);
  const [containers, setContainers] = useState<Container[]>([]);
  const [stopped, setStopped] = useState(item.stopContainers);
  const [probing, setProbing] = useState(false);
  const [pre, setPre] = useState(item.preSnapshot);
  const [post, setPost] = useState(item.postSnapshot);
  const { debouncedSave } = useDebouncedSave();

  useEffect(() => {
    listContainers()
      .then((res) => setContainers(res.ok ? (res.containers ?? []) : []))
      .catch(() => undefined);
  }, []);

  async function save(patch: ZFSDatasetPatch): Promise<boolean> {
    setBusy(true);
    try {
      const res = await patchZFSDataset(item.id, patch);
      if (res.ok) {
        onChanged();
        return true;
      }
      push(res.code ? zfsCodeSentence(t, res.code) : (res.error ?? t("settings.error")), "fail");
      return false;
    } catch (err) {
      push(failText(t, err), "fail");
      return false;
    } finally {
      setBusy(false);
    }
  }

  async function toggleChild(dataset: string, include: boolean) {
    const next = new Set(excluded);
    if (include) next.delete(dataset);
    else next.add(dataset);
    setExcluded(next);
    if (!(await save({ excludedChildren: [...next] }))) setExcluded(excluded);
  }

  async function dropStoredExclusion(dataset: string) {
    const next = new Set(excluded);
    next.delete(dataset);
    setExcluded(next);
    await save({ excludedChildren: [...next] });
  }

  async function setStopList(list: string[]) {
    setStopped(list);
    if (!(await save({ stopContainers: list }))) setStopped(stopped);
  }

  function saveHooks(nextPre: string, nextPost: string) {
    debouncedSave(() => {
      void save({ hookContainer: item.hookContainer, preSnapshot: nextPre, postSnapshot: nextPost });
    });
  }

  async function handleProbe() {
    setProbing(true);
    try {
      const res = await probeZFSDataset(item.id);
      if (!res.ok) push(res.error ?? t("settings.error"), "fail");
      onChanged();
    } catch (err) {
      push(failText(t, err), "fail");
    } finally {
      setProbing(false);
    }
  }

  async function handleDeleteBackups() {
    if (!(await confirm(t("zfs.deleteBackupsConfirm"), { confirmKey: "snapshots.deleteAll" }))) return;
    try {
      const res = await deleteBackupsZFSDataset(item.id);
      if (res.ok) onChanged();
      else push(res.error ?? t("common.deleteBackupsFailed"), "fail");
    } catch (err) {
      push(failText(t, err), "fail");
    }
  }

  const known = new Set(item.members.map((m) => m.dataset));
  const gone = [...excluded].filter((d) => !known.has(d));
  const candidates = containers.filter((c) => !c.self && !stopped.includes(c.name));
  const overlap = stopped.filter((n) => containers.some((c) => c.name === n && c.includeInSchedule));

  return (
    <div className="mt-1 flex flex-col gap-4 rounded-card bg-carbon-background p-3">
      <div className="flex flex-col gap-1">
        <span className="flex items-center gap-1.5 text-sm text-carbon-text">
          {t("zfs.children")}
          <InfoBubble tip={t("zfs.childrenHint")} />
        </span>
        <div role="group" aria-label={t("zfs.children")}>
          <ZFSMemberList
            members={item.members}
            root={item.dataset}
            t={t}
            excluded={excluded}
            busy={busy}
            onToggle={(dataset, include) => void toggleChild(dataset, include)}
            series={series}
            targetId={item.id}
          />
        </div>
        {gone.map((dataset) => (
          <p key={dataset} className="flex items-center gap-2 text-xs text-carbon-textMuted max-md:flex-wrap">
            <span dir="ltr" className="font-mono text-start max-md:wrap-anywhere">{dataset}</span>
            {t("zfs.excludedGone")}
            <Button
              label={t("zfs.removeMissing")}
              labelKey="zfs.removeMissing"
              tone="subtle"
              onClick={() => void dropStoredExclusion(dataset)}
            />
          </p>
        ))}
      </div>

      <RepoPicker
        value={item.repo}
        onChange={(next) => void save({ repo: next })}
        locked={item.lastBackup > 0}
        disabled={busy}
      />

      <div className="flex flex-col gap-1">
        <span className="flex items-center gap-1.5 text-sm text-carbon-text">
          {t("zfs.stopContainers")}
          <InfoBubble tip={t("zfs.stopContainersHint")} />
        </span>
        {stopped.length === 0 && (
          <p className="text-xs text-carbon-textMuted">{t("zfs.stopContainersNone")}</p>
        )}
        <div className="flex items-center gap-1.5 flex-wrap">
          {stopped.map((name) => (
            <Badge key={name} tone="neutral" wrap>
              {name}
              <Button
                label={t("offsite.targets.remove")}
                labelKey="offsite.targets.remove"
                tone="subtle"
                variant="chip"
                onClick={() => void setStopList(stopped.filter((n) => n !== name))}
              />
            </Badge>
          ))}
        </div>
        <SelectField
          value=""
          label={t("zfs.stopContainersPlaceholder")}
          onChange={(name) => void setStopList([...stopped, name])}
          disabled={busy || candidates.length === 0}
          className="w-64 max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs text-carbon-text"
          options={[
            { value: "", label: t("zfs.stopContainersPlaceholder") },
            ...candidates.map((c) => ({ value: c.name, label: c.name })),
          ]}
        />
        {overlap.length > 0 && (
          <p className="text-xs text-statusWarn">
            {t("zfs.overlapContainers").replace("{names}", overlap.join(", "))}
          </p>
        )}
      </div>

      <ItemAnomalySettings item={anomaly} enabled={anomalyEnabled} t={t} />

      <ReplicaCard itemId={item.id} name={item.dataset} />

      {advanced && (
        <>
          <ZFSExcludesEditor item={item} t={t} onSaved={onChanged} />

          <div className="flex flex-col gap-1">
            <span className="flex items-center gap-1.5 text-sm text-carbon-text">
              {t("zfs.hookContainer")}
              <InfoBubble tip={t("zfs.hooksHint")} />
            </span>
            <SelectField
              value={item.hookContainer}
              label={t("zfs.hookContainer")}
              onChange={(name) => void save({ hookContainer: name, preSnapshot: pre, postSnapshot: post })}
              disabled={busy}
              className="w-64 max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs text-carbon-text"
              options={[
                { value: "", label: t("zfs.hookContainerNone") },
                ...containers.filter((c) => !c.self).map((c) => ({ value: c.name, label: c.name })),
              ]}
            />
            <label className="flex flex-col gap-1 text-sm text-carbon-text">
              {t("zfs.preSnapshot")}
              <input
                dir="ltr"
                value={pre}
                onChange={(e) => {
                  setPre(e.target.value);
                  saveHooks(e.target.value, post);
                }}
                disabled={item.hookContainer === ""}
                className="rounded-control bg-carbon-surface2 px-3 py-1.5 font-mono text-xs text-carbon-text text-start"
              />
            </label>
            <label className="flex flex-col gap-1 text-sm text-carbon-text">
              {t("zfs.postSnapshot")}
              <input
                dir="ltr"
                value={post}
                onChange={(e) => {
                  setPost(e.target.value);
                  saveHooks(pre, e.target.value);
                }}
                disabled={item.hookContainer === ""}
                className="rounded-control bg-carbon-surface2 px-3 py-1.5 font-mono text-xs text-carbon-text text-start"
              />
            </label>
          </div>

          <div className="flex items-center gap-2 flex-wrap">
            <Button
              label={t("zfs.probe")}
              labelKey="zfs.probe"
              tone="neutral"
              onClick={() => void handleProbe()}
              disabled={probing}
              busy={probing}
              title={probing ? t("zfs.probing") : undefined}
            />
            <Button
              label={t("snapshots.deleteAll")}
              labelKey="snapshots.deleteAll"
              tone="subtle"
              onClick={() => void handleDeleteBackups()}
              className="ms-auto"
            />
          </div>
        </>
      )}
      {confirmDialog}
    </div>
  );
}
