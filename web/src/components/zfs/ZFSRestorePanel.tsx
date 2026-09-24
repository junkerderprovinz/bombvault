import { useEffect, useRef, useState } from "react";

import { listSnapshotFilesZFS, restoreZFS, zfsRestorePoints } from "../../lib/api";
import type { FileEntry, ZFSDatasetView, ZFSHostDataset, ZFSRestoreAck, ZFSRestorePoint } from "../../lib/api";
import { Advanced, useAdvanced } from "../../lib/advanced";
import { useBackupWatch } from "../../lib/backupWatch";
import { copyText } from "../../lib/clipboard";
import { useT } from "../../lib/i18n";
import { tLtr } from "../../lib/ltrFragments";
import { anyActive, busyPhraseKey, useProgress } from "../../lib/progress";
import { useConfirm } from "../../lib/useConfirm";
import { useToast } from "../../lib/toast";
import { zfsCodeSentence, zfsMemberKey } from "../../lib/zfsCodes";
import { Button } from "../Button";
import { FolderBrowser } from "../FolderBrowser";
import { IconDisclosure } from "../IconDisclosure";
import { InfoBubble } from "../InfoBubble";
import { RestoreProgress } from "../restore/RestoreProgress";
import { Selector, type SelectorItem } from "../Selector";
import { SelectField } from "../SelectField";
import { SnapshotFileTree } from "../SnapshotFileTree";
import { IconCopy, IconRestore } from "../Sidebar";
import { SourceToggle, type RepoSource } from "../SourceToggle";
import { ToggleRow } from "../../pages/settings/shared";

// The panel restores one dataset of an item's tree from one run instant. It
// follows the Folders restore controls rather than the container panel: the
// destination is a Selector, the outcome stays inline, and the progress comes
// from the shared RestoreProgress over the item's own progress key.
//
// Writing into the live dataset is the one destructive path, so the two guards
// the server applies anyway are shown before the click: a dataset the host no
// longer has mounted, and a mapping BombVault may only read.

/** The stand-in dataset value for restoring every member of one instant. */
const WHOLE_TREE = "";

/** Member states that make writing into the live dataset impossible. */
const NOT_WRITABLE = new Set(["gone", "not-mounted", "not-visible"]);

type Mode = "inPlace" | "folder" | "select";

export function ZFSRestorePanel({
  item,
  host,
  hostMountRoot,
  restoreFolder,
}: {
  item: ZFSDatasetView;
  /** The cached host listing by dataset name, for the read-only check. Empty
   *  when the host could not be reached; the server checks again either way. */
  host: ReadonlyMap<string, ZFSHostDataset>;
  hostMountRoot: string;
  restoreFolder: string;
}) {
  const { t } = useT();
  const { push } = useToast();
  const { advanced } = useAdvanced();
  const { confirm, confirmDialog } = useConfirm();
  const [open, setOpen] = useState(false);
  const [source, setSource] = useState<RepoSource>("local");
  const [points, setPoints] = useState<ZFSRestorePoint[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [stamp, setStamp] = useState("");
  const [dataset, setDataset] = useState(item.dataset);
  const [mode, setMode] = useState<Mode>("inPlace");
  const [targetPath, setTargetPath] = useState(restoreFolder);
  const [safety, setSafety] = useState(true);
  const [safetyOffConfirmed, setSafetyOffConfirmed] = useState(false);
  const [stopContainers, setStopContainers] = useState(true);
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [filesLoading, setFilesLoading] = useState(false);
  const [fileFilter, setFileFilter] = useState("");
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [refusal, setRefusal] = useState("");
  const [ack, setAck] = useState<ZFSRestoreAck | null>(null);

  useEffect(() => {
    if (!open) return;
    let current = true;
    setLoading(true);
    setError(null);
    zfsRestorePoints(item.id, source)
      .then((res) => {
        if (!current) return;
        if (!res.ok) {
          setPoints([]);
          setError(res.error ?? t("common.loadBackupsFailed"));
          return;
        }
        const list = res.points ?? [];
        setPoints(list);
        const first = list[0];
        if (!first) return;
        setStamp(first.stamp);
        const root = first.members.find((m) => m.dataset === item.dataset && m.snapshotId !== "");
        setDataset((root ?? first.members.find((m) => m.snapshotId !== ""))?.dataset ?? item.dataset);
      })
      .catch(() => {
        if (current) setError(t("common.loadBackupsFailed"));
      })
      .finally(() => {
        if (current) setLoading(false);
      });
    return () => {
      current = false;
    };
    // t() is read only to build a failure message, so a language switch is no
    // reason to fetch the points again.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, item.id, item.dataset, source]);

  const point = points.find((p) => p.stamp === stamp);
  const wholeTree = dataset === WHOLE_TREE;
  const member = point?.members.find((m) => m.dataset === dataset);
  const live = item.members.find((m) => m.dataset === dataset);
  const entry = host.get(dataset);

  const blocked = ((): "zfs.restore.missingDataset" | "zfs.code.read-only-mount" | null => {
    if (wholeTree) return null;
    if (item.lastCheckCode === "not-found") return "zfs.restore.missingDataset";
    if (live === undefined || NOT_WRITABLE.has(live.outcome)) return "zfs.restore.missingDataset";
    if (entry && entry.visible && !entry.writable) return "zfs.code.read-only-mount";
    return null;
  })();
  // A destination the server would refuse falls back to the folder, which
  // works for every dataset, gone ones included. A whole tree has no single
  // live dataset to write into.
  const folderOnly = wholeTree || blocked !== null;
  const active: Mode = folderOnly && mode !== "folder" ? "folder" : mode;
  const inPlace = active !== "folder";
  const mountpoint = live?.hostMountpoint ?? item.hostMountpoint;

  const progressKey = `zfs:${item.dataset}`;
  const cancelledRef = useRef(false);
  const { state, fire, reset, isPending } = useBackupWatch({
    progressKey,
    kind: "restore",
    matchRun: (r) => r.domain === "zfs" && r.target === item.dataset,
    cancelledRef,
    start: async () => {
      setRefusal("");
      setAck(null);
      const res = await restoreZFS(
        item.id,
        {
          stamp,
          dataset: wholeTree ? "" : dataset,
          wholeTree,
          paths: active === "select" ? [...picked] : [],
          targetPath: active === "folder" ? targetPath.trim() : "",
          confirm: true,
          safetySnapshot: inPlace && safety,
          safetyOffConfirm: inPlace && safetyOffConfirmed,
          stopContainers: inPlace && item.stopContainers.length > 0 && stopContainers,
        },
        source,
      );
      if (res.ok) setAck(res);
      else if (res.code) setRefusal(zfsCodeSentence(t, res.code, { hostMountpoint: mountpoint }));
      return res;
    },
  });
  const progressMap = useProgress();
  const prog = progressMap[progressKey];
  const otherActive = anyActive(progressMap);
  const blockedByOther = otherActive.active && !isPending;

  useEffect(() => {
    reset();
    setAck(null);
    setRefusal("");
  }, [dataset, stamp, active, targetPath, reset]);

  useEffect(() => {
    if (active !== "select" || !member || member.snapshotId === "") return;
    let current = true;
    // Paths picked in another dataset's snapshot mean nothing in this one.
    setPicked(new Set());
    setFilesLoading(true);
    listSnapshotFilesZFS(item.id, member.snapshotId, source)
      .then((res) => {
        if (current) setFiles(res.ok ? (res.files ?? []) : []);
      })
      .catch(() => {
        if (current) setFiles([]);
      })
      .finally(() => {
        if (current) setFilesLoading(false);
      });
    return () => {
      current = false;
    };
  }, [active, item.id, member, source]);

  async function handleSafety(next: boolean) {
    if (!next && !(await confirm(t("zfs.restore.safetyOffConfirm")))) return;
    setSafety(next);
    setSafetyOffConfirmed(!next);
  }

  async function handleRestore() {
    if (active === "folder" && targetPath.trim() === "") return;
    if (inPlace) {
      const question = t("zfs.restore.confirm").replace("{path}", mountpoint);
      if (!(await confirm(question, { confirmKey: "snapshots.restore" }))) return;
    }
    void fire();
  }

  async function copySnapshot(name: string) {
    if (await copyText(name)) push(t("common.copied"), "success");
    else push(t("vm.ssh.copyFailed"), "fail");
  }

  function outcomeLabel(outcome: string): string {
    const key = zfsMemberKey(outcome);
    return key ? t(key) : zfsCodeSentence(t, outcome);
  }

  const pointOptions = points.map((p) => ({
    value: p.stamp,
    label: `${new Date(p.time * 1000).toLocaleString()} · ${t("zfs.membersSummary").replace("{n}", String(p.members.length))}`,
  }));

  const datasetOptions = [
    ...(advanced ? [{ value: WHOLE_TREE, label: t("zfs.restore.wholeTree") }] : []),
    ...(point?.members ?? []).map((m) => ({
      value: m.dataset,
      label: `${m.dataset} (${outcomeLabel(m.outcome)})`,
      disabled: m.snapshotId === "",
    })),
  ];

  const modeItems: SelectorItem[] = [
    {
      id: "inPlace",
      label: t("zfs.restore.inPlace"),
      disabled: folderOnly,
      title: blocked !== null ? t(blocked) : undefined,
    },
    { id: "folder", label: t("zfs.restore.toFolder") },
    ...(advanced
      ? [{ id: "select", label: t("zfs.restore.selectFiles"), disabled: folderOnly }]
      : []),
  ];

  const title = t("zfs.restore.title").replace("{dataset}", item.dataset);

  return (
    <div className="mt-1">
      <Button
        label={t("snapshots.title")}
        labelKey="snapshots.title"
        tone="neutral"
        onClick={() => setOpen((prev) => !prev)}
        glyph={<IconDisclosure open={open} />}
      />

      {open && (
        <div
          role="group"
          aria-label={title}
          className="mt-2 flex flex-col gap-3 rounded-card bg-carbon-background p-3"
        >
          <Advanced>
            <span className="flex items-center gap-2">
              <span className="flex items-center gap-1 text-xs text-carbon-textMuted">
                {t("source.label")}
                <InfoBubble tip={t("source.hint")} />
              </span>
              <SourceToggle source={source} onChange={setSource} disabled={loading} domain="zfs" />
            </span>
          </Advanced>

          {loading && <p className="text-xs text-carbon-textMuted">{t("common.loadingBackups")}</p>}
          {error && <p className="text-xs text-statusFail">{error}</p>}
          {!loading && !error && points.length === 0 && (
            <p className="text-xs text-carbon-textMuted">{t("snapshots.none")}</p>
          )}

          {points.length > 0 && (
            <>
              <div className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textSub">{t("zfs.restore.point")}</span>
                <SelectField
                  value={stamp}
                  label={t("zfs.restore.point")}
                  onChange={setStamp}
                  options={pointOptions}
                  className="w-full max-w-md rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs text-carbon-text"
                />
              </div>

              <div className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textSub">{t("zfs.restore.dataset")}</span>
                <SelectField
                  value={dataset}
                  label={t("zfs.restore.dataset")}
                  onChange={setDataset}
                  options={datasetOptions}
                  className="w-full max-w-md rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs text-carbon-text"
                />
              </div>

              <div className="flex items-center gap-2 flex-wrap">
                <Selector
                  items={modeItems}
                  label={t("snapshots.restore")}
                  select="one"
                  active={active}
                  buttonHeight
                  onChange={(id) => setMode(id as Mode)}
                  disabled={isPending}
                />
                {inPlace && <InfoBubble tip={t("zfs.restore.inPlaceHint").replace("{path}", mountpoint)} />}
                <Button
                  label={t("snapshots.restore")}
                  labelKey="snapshots.restore"
                  glyph={<IconRestore />}
                  tone="accent"
                  onClick={() => void handleRestore()}
                  disabled={isPending || blockedByOther || (active === "select" && picked.size === 0)}
                  busy={isPending}
                  title={isPending ? t("common.restoring") : undefined}
                  className="shrink-0"
                />
                {blockedByOther && (
                  <span className="text-caption text-carbon-textMuted">
                    {t(busyPhraseKey(otherActive.phase))}
                  </span>
                )}
              </div>

              {blocked !== null && <p className="text-xs text-statusWarn">{tLtr(t, blocked)}</p>}

              {active === "folder" && (
                <FolderBrowser
                  label={t("restore.targetPath")}
                  value={targetPath}
                  hostMountRoot={hostMountRoot}
                  onChange={setTargetPath}
                />
              )}

              {active === "select" && (
                <SnapshotFileTree
                  files={files}
                  loading={filesLoading}
                  error={null}
                  filter={fileFilter}
                  onFilterChange={setFileFilter}
                  selected={picked}
                  onToggle={(path) => {
                    const next = new Set(picked);
                    if (next.has(path)) next.delete(path);
                    else next.add(path);
                    setPicked(next);
                  }}
                  t={t}
                />
              )}

              {inPlace && (
                <div className="flex flex-col gap-1">
                  <span className="flex items-center gap-1.5">
                    <ToggleRow
                      label={t("zfs.restore.safetySnapshot")}
                      checked={safety}
                      onChange={(next) => void handleSafety(next)}
                      disabled={isPending}
                    />
                    <InfoBubble tip={t("zfs.restore.safetySnapshotHint")} />
                  </span>
                  {item.stopContainers.length > 0 && (
                    <span className="flex items-center gap-1.5">
                      <ToggleRow
                        label={t("zfs.restore.stopContainers").replace(
                          "{names}",
                          item.stopContainers.join(", "),
                        )}
                        checked={stopContainers}
                        onChange={setStopContainers}
                        disabled={isPending}
                      />
                      <InfoBubble tip={t("zfs.restore.stopContainersHint")} />
                    </span>
                  )}
                </div>
              )}

              {refusal !== "" && <p className="text-sm text-statusFail">{refusal}</p>}

              {ack && (
                <div className="flex flex-col gap-1">
                  <p className="text-xs text-carbon-textSub">
                    {/* In place the server answers with the path inside the
                        container; the reader knows the dataset by where the
                        host mounts it. */}
                    {t("zfs.restore.started").replace("{target}", inPlace ? mountpoint : (ack.target ?? ""))}
                  </p>
                  {ack.safetySnapshot && (
                    <p className="flex items-center gap-2 text-xs text-carbon-textSub">
                      <span dir="ltr" className="font-mono text-start">
                        {t("zfs.restore.safetyTaken").replace("{snapshot}", ack.safetySnapshot)}
                      </span>
                      <Button
                        label={t("common.copy")}
                        labelKey="common.copy"
                        glyph={<IconCopy />}
                        tone="accent"
                        onClick={() => void copySnapshot(ack.safetySnapshot ?? "")}
                      />
                    </p>
                  )}
                </div>
              )}

              <RestoreProgress
                state={state}
                isPending={isPending}
                prog={prog}
                cancelKey={progressKey}
                inPlace={inPlace}
                name={item.dataset}
                cancelledRef={cancelledRef}
                showStartedHint={false}
                successMessage={t("files.restoreComplete")}
                t={t}
              />
            </>
          )}
        </div>
      )}
      {confirmDialog}
    </div>
  );
}
