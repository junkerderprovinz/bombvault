import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";

import {
  backupZFSDataset,
  deleteBackupsZFSDataset,
  deleteZFSDataset,
  deleteZFSSafetySnapshot,
  listContainers,
  listZFSSafetySnapshots,
  patchZFSDataset,
  previewZFSExcludes,
  probeZFSDataset,
  sweepZFSDataset,
  zfsRunMembers,
} from "../../lib/api";
import type {
  AnomalyItem,
  AnomalySeriesInfo,
  Container,
  Run,
  ZFSDatasetPatch,
  ZFSDatasetView,
  ZFSExcludePreviewRow,
  ZFSHostDataset,
  ZFSRunDetail,
  ZFSSafetySnapshot,
} from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { useAdvanced } from "../../lib/advanced";
import { useBackupWatch } from "../../lib/backupWatch";
import { humanBytes } from "../../lib/forecast";
import { useT } from "../../lib/i18n";
import { tLtr } from "../../lib/ltrFragments";
import { anyActive, busyPhraseKey, useProgress } from "../../lib/progress";
import { relativeTime, formatTs } from "../../lib/reltime";
import type { RestoreRequest } from "../../lib/restoreRequest";
import { useConfirm } from "../../lib/useConfirm";
import { useDebouncedSave } from "../../lib/useDebouncedSave";
import { useToast } from "../../lib/toast";
import { RunReasonText } from "../../lib/runReason";
import { ZFS_CODE_VARS, zfsCodeSentence, zfsFixKey, zfsMemberKey, zfsRunReasonCode } from "../../lib/zfsCodes";
import { Badge } from "../Badge";
import { BackupCancelButton } from "../BackupCancelButton";
import { Button } from "../Button";
import { EffectiveScheduleLine } from "../EffectiveScheduleLine";
import { IconDisclosure } from "../IconDisclosure";
import { InfoBubble } from "../InfoBubble";
import { ItemAnomalyBadge } from "../ItemAnomalyBadge";
import { ItemAnomalySettings } from "../ItemAnomalySettings";
import { ProgressBar } from "../ProgressBar";
import { RecentRunsList } from "../RecentRunsList";
import { RepoPicker } from "../RepoPicker";
import { SelectField } from "../SelectField";
import { IconBackupNow, IconPencil, IconTrash } from "../Sidebar";
import { ToggleRow } from "../../pages/settings/shared";
import { ZFSMemberList, zfsMemberActionable } from "./ZFSMemberList";
import { ZFSRestorePanel } from "./ZFSRestorePanel";

type T = ReturnType<typeof useT>["t"];

const DAY = 24 * 60 * 60;

// Codes that mean the last look at this item found nothing usable, as opposed
// to one dataset of the tree the reader could mount or unlock.
const RED_CODES = new Set([
  "not-found",
  "not-filesystem",
  "nothing-readable",
  "snapshot-failed",
  "snapshot-not-visible",
  "snapshot-loop",
  "pre-snapshot-failed",
  "consistency-stop-failed",
]);

function progressKeyOf(item: ZFSDatasetView): string {
  return `zfs:${item.dataset}`;
}

function ZFSBackupButton({ item, t, onDone, running }: {
  item: ZFSDatasetView;
  t: T;
  onDone: () => void;
  running: { active: boolean; phase?: string };
}) {
  const { fire, isPending } = useBackupWatch({
    progressKey: progressKeyOf(item),
    start: () => backupZFSDataset(item.id),
    matchRun: (r) => r.domain === "zfs" && r.target === item.dataset,
    onDone,
  });
  const blockedByOther = running.active && !isPending;
  return (
    <Button
      label={t("containers.backupNow")}
      labelKey="containers.backupNow"
      glyph={<IconBackupNow />}
      tone="accent"
      onClick={() => void fire()}
      disabled={isPending || blockedByOther}
      busy={isPending}
      title={isPending ? t("common.backingUp") : blockedByOther ? t(busyPhraseKey(running.phase)) : undefined}
    />
  );
}

/** The extra switch in the delete confirm. It owns its state and reports
 *  through `sink`, because the dialog keeps the node it was handed. */
function DeleteSafetyToggle({ label, sink }: { label: string; sink: { current: boolean } }) {
  const [on, setOn] = useState(false);
  return (
    <ToggleRow
      label={label}
      checked={on}
      onChange={(next) => {
        setOn(next);
        sink.current = next;
      }}
    />
  );
}

/** The message of a request the server turned away, for example while a ZFS
 *  run holds the domain. */
function failText(t: T, err: unknown): string {
  return err instanceof Error ? err.message : t("settings.error");
}

/** Why a run failed or stopped, in the reader's language where the code allows
 *  it. A detail the command output below repeats is left out. */
function zfsRunReason(t: T, raw: string, hookDetail: string): ReactNode {
  const parsed = zfsRunReasonCode(raw);
  if (!parsed || ZFS_CODE_VARS[parsed.code]) return <RunReasonText reason={raw} t={t} />;
  const sentence = zfsCodeSentence(t, parsed.code);
  if (!parsed.detail || parsed.detail === hookDetail) return sentence;
  return (
    <>
      {sentence} <bdi dir="ltr">{parsed.detail}</bdi>
    </>
  );
}

function ZFSRunDetailView({ run, item, t }: { run: Run; item: ZFSDatasetView; t: T }) {
  const [detail, setDetail] = useState<ZFSRunDetail | null>(null);
  const runId = run.id;

  useEffect(() => {
    let alive = true;
    zfsRunMembers(runId)
      .then((res) => {
        if (alive && res.ok) setDetail(res);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, [runId]);

  if (!detail) return null;
  const window = detail.windowSeconds ?? -1;
  const reason = run.status === "failed" || run.status === "cancelled" ? run.error : "";
  return (
    <div className="ps-4 flex flex-col gap-1">
      {reason && (
        <p className={`text-caption ${run.status === "failed" ? "text-statusFail" : "text-carbon-textMuted"}`}>
          {zfsRunReason(t, reason, detail.hookDetail ?? "")}
        </p>
      )}
      {window >= 0 && (
        <p className="text-caption text-carbon-textMuted">
          {t("zfs.window").replace("{seconds}", String(window))}
        </p>
      )}
      {(detail.members ?? []).map((m) => {
        const memberKey = zfsMemberKey(m.outcome);
        return (
          <p key={m.dataset} className="flex items-center gap-2 text-caption text-carbon-textSub">
            <span dir="ltr" className="font-mono text-start">{m.dataset}</span>
            {m.isNew && (
              <Badge tone="active" size="small">
                {t("zfs.member.new")}
              </Badge>
            )}
            <span className="text-carbon-textMuted">
              {memberKey
                ? t(memberKey)
                : zfsCodeSentence(t, m.outcome, {
                    hostMountpoint: item.members.find((known) => known.dataset === m.dataset)?.hostMountpoint,
                  })}
            </span>
            {m.bytesAdded > 0 && <span className="text-carbon-textMuted">{humanBytes(m.bytesAdded)}</span>}
          </p>
        );
      })}
      {detail.hookDetail && (
        <pre
          dir="ltr"
          className="overflow-x-auto rounded-control bg-carbon-surface2 p-2 text-caption text-carbon-textSub whitespace-pre-wrap text-start"
        >
          {detail.hookDetail}
        </pre>
      )}
    </div>
  );
}

function ZFSSafetySection({ item, t, onRefresh }: { item: ZFSDatasetView; t: T; onRefresh: () => void }) {
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [open, setOpen] = useState(false);
  const [snapshots, setSnapshots] = useState<ZFSSafetySnapshot[]>([]);

  const title = t("zfs.safety.title").replace("{n}", String(item.safetyCount));

  function load() {
    listZFSSafetySnapshots(item.id)
      .then((res) => setSnapshots(res.snapshots ?? []))
      .catch(() => undefined);
  }

  function handleOpen() {
    const next = !open;
    setOpen(next);
    if (next) load();
  }

  async function handleDelete(snap: ZFSSafetySnapshot) {
    const question = t("zfs.safety.deleteConfirm")
      .replace("{name}", snap.name)
      .replace("{dataset}", snap.dataset);
    if (!(await confirm(question, { confirmKey: "common.delete" }))) return;
    try {
      const res = await deleteZFSSafetySnapshot(item.id, snap.dataset, snap.name);
      if (res.ok) {
        load();
        onRefresh();
      } else {
        push(res.error ?? t("common.deleteFailed"), "fail");
      }
    } catch (err) {
      push(failText(t, err), "fail");
    }
  }

  const old = item.safetyOldestAt > 0 && Date.now() / 1000 - item.safetyOldestAt > 30 * DAY;

  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-1.5">
        <button
          type="button"
          aria-expanded={open}
          onClick={handleOpen}
          className="flex items-center gap-1.5 text-xs text-carbon-textSub hover:text-carbon-text"
        >
          <IconDisclosure open={open} />
          {title}
        </button>
        <InfoBubble tip={t("zfs.safety.hint")} />
      </div>
      {old && <p className="text-xs text-statusWarn">{t("zfs.safety.old")}</p>}
      {open && (
        <ul aria-label={title} className="flex flex-col gap-1">
          {snapshots.map((snap) => (
            <li key={`${snap.dataset}@${snap.name}`} className="flex items-center gap-2 text-xs">
              <span dir="ltr" className="font-mono text-carbon-textSub text-start truncate">
                {t("zfs.safety.row")
                  .replace("{dataset}", `${snap.dataset}@${snap.name}`)
                  .replace("{age}", relativeTime(t, snap.createdAt))
                  .replace("{size}", humanBytes(snap.usedBytes))}
              </span>
              <Button
                label={t("common.delete")}
                labelKey="common.delete"
                glyph={<IconTrash />}
                tone="accent"
                variant="icon"
                onClick={() => void handleDelete(snap)}
                className="ms-auto"
              />
            </li>
          ))}
        </ul>
      )}
      {confirmDialog}
    </div>
  );
}

function ZFSExcludesEditor({ item, t, onSaved }: { item: ZFSDatasetView; t: T; onSaved: () => void }) {
  const { push } = useToast();
  const [text, setText] = useState(item.excludes.join("\n"));
  const [rows, setRows] = useState<ZFSExcludePreviewRow[]>([]);
  const [refusal, setRefusal] = useState("");
  const { debouncedSave } = useDebouncedSave();

  const lines = text.split("\n").map((l) => l.trim()).filter(Boolean);
  const linesKey = lines.join("\n");

  useEffect(() => {
    if (linesKey === "") {
      setRows([]);
      return;
    }
    let alive = true;
    const id = setTimeout(() => {
      previewZFSExcludes(item.id, linesKey.split("\n"))
        .then((res) => {
          if (alive) setRows(res.rows ?? []);
        })
        .catch(() => undefined);
    }, 400);
    return () => {
      alive = false;
      clearTimeout(id);
    };
  }, [item.id, linesKey]);

  function handleChange(next: string) {
    setText(next);
    const list = next.split("\n").map((l) => l.trim()).filter(Boolean);
    debouncedSave(() => {
      void patchZFSDataset(item.id, { excludes: list }).then((res) => {
        if (res.ok) {
          setRefusal("");
          onSaved();
        } else if (res.code) {
          setRefusal(zfsCodeSentence(t, res.code, { names: list }));
        } else {
          push(res.error ?? t("excludes.error"), "fail");
        }
      });
    });
  }

  return (
    <div className="flex flex-col gap-1">
      <span className="flex items-center gap-1.5 text-sm text-carbon-text">
        {t("zfs.excludes")}
        <InfoBubble tip={tLtr(t, "zfs.excludesHint")} />
      </span>
      <textarea
        dir="ltr"
        rows={4}
        value={text}
        onChange={(e) => handleChange(e.target.value)}
        aria-label={t("zfs.excludes")}
        className="rounded-control bg-carbon-surface2 p-2 font-mono text-xs text-carbon-text text-start"
      />
      {refusal && <p className="text-xs text-statusFail">{refusal}</p>}
      {rows.map((row) => (
        <p key={row.pattern} className="text-caption text-carbon-textMuted">
          <span dir="ltr" className="font-mono text-start">{row.pattern}</span>
          {": "}
          {t("zfs.excludesCount").replace("{n}", String(row.matches))}
          {row.sample.length > 0 && ` (${row.sample.join(", ")})`}
        </p>
      ))}
    </div>
  );
}

function ZFSItemSettings({
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
          <p key={dataset} className="flex items-center gap-2 text-xs text-carbon-textMuted">
            <span dir="ltr" className="font-mono text-start">{dataset}</span>
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
        labelKey="zfs.repo"
        hintKey="zfs.repoHint"
        defaultLabelKey="zfs.repoPlaceholder"
        lockedKey="zfs.repoLocked"
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

/** ZFSDatasetRow is one item: a root dataset and the tree below it, with what
 *  the last check and the last run made of it. */
export function ZFSDatasetRow({
  item,
  t,
  onRefresh,
  index,
  host,
  hostMountRoot,
  restoreFolder,
  anomaly,
  anomalyEnabled = false,
  restoreRequest,
}: {
  item: ZFSDatasetView;
  t: T;
  onRefresh: () => void;
  /** Rainbow position by list index, as on the Folders cards. */
  index: number;
  /** The cached host listing by dataset name, for the restore panel. */
  host: ReadonlyMap<string, ZFSHostDataset>;
  hostMountRoot: string;
  restoreFolder: string;
  /** What anomaly detection knows about this item and each of its datasets. */
  anomaly?: AnomalyItem;
  anomalyEnabled?: boolean;
  /** A finding's link to a backup of this item to restore. */
  restoreRequest?: RestoreRequest;
}) {
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const progressMap = useProgress();
  const progress = progressMap[progressKeyOf(item)];
  const running = anyActive(progressMap);
  const [editing, setEditing] = useState(false);
  const [membersOpen, setMembersOpen] = useState(false);
  const [enabled, setEnabled] = useState(item.enabled);
  const [enabledBusy, setEnabledBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const [sweeping, setSweeping] = useState(false);
  const deleteSafetyToo = useRef(false);
  const cardRef = useRef<HTMLDivElement>(null);
  const series = new Map((anomaly?.datasets ?? []).map((s) => [s.part, s]));

  useEffect(() => setEnabled(item.enabled), [item.enabled]);
  useEffect(() => {
    // jsdom has no scrollIntoView.
    if (restoreRequest) cardRef.current?.scrollIntoView?.({ block: "start" });
  }, [restoreRequest]);

  const runFailed = item.lastRunStatus === "failed";
  const checkCode = item.lastCheckCode;
  const skipped = item.members.filter(zfsMemberActionable);
  const fixKey = zfsFixKey(checkCode);

  async function handleEnabled(next: boolean) {
    setEnabled(next);
    setEnabledBusy(true);
    try {
      const res = await patchZFSDataset(item.id, { enabled: next });
      if (res.ok) {
        onRefresh();
      } else {
        setEnabled(!next);
        push(res.error ?? t("schedule.updateFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      setEnabled(!next);
      push(failText(t, err), "fail");
      setShake((n) => n + 1);
    } finally {
      setEnabledBusy(false);
    }
  }

  async function handleRemove() {
    deleteSafetyToo.current = false;
    const answered = await confirm(t("zfs.deleteRowConfirm"), {
      confirmKey: "common.delete",
      extra:
        item.safetyCount > 0 ? (
          <DeleteSafetyToggle label={t("zfs.deleteSafetyToo")} sink={deleteSafetyToo} />
        ) : undefined,
    });
    if (!answered) return;
    let res: Awaited<ReturnType<typeof deleteZFSDataset>>;
    try {
      res = await deleteZFSDataset(item.id, deleteSafetyToo.current);
    } catch (err) {
      push(failText(t, err), "fail");
      setShake((n) => n + 1);
      return;
    }
    if (!res.ok) {
      push(res.error ?? t("common.removeFailed"), "fail");
      setShake((n) => n + 1);
      return;
    }
    if (res.safetyRemaining) {
      push(t("zfs.deleteKeptSafety").replace("{n}", String(res.safetyRemaining)), "warn");
    }
    if (res.leftoversRemaining) {
      push(t("zfs.sweepRemaining").replace("{n}", String(res.leftoversRemaining)), "warn");
    }
    onRefresh();
  }

  async function handleSweep() {
    setSweeping(true);
    try {
      const res = await sweepZFSDataset(item.id);
      const remaining = res.remaining ?? 0;
      if (res.ok && remaining === 0) push(t("zfs.sweepDone"), "success");
      else push(t("zfs.sweepRemaining").replace("{n}", String(remaining)), "warn");
      onRefresh();
    } catch (err) {
      push(failText(t, err), "fail");
    } finally {
      setSweeping(false);
    }
  }

  return (
    <div
      ref={cardRef}
      style={{ ...hueVars(index), "--row-i": String(index) } as CSSProperties}
      className={`relative overflow-hidden bg-carbon-surface rounded-card p-4 flex flex-col gap-3 glim-hue glim-stagger-row ${
        progress?.active ? "glim-active" : ""
      }`}
    >
      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span dir="ltr" className="font-semibold text-carbon-text text-sm truncate text-start">
              {item.dataset}
            </span>
            <ItemAnomalyBadge item={anomaly} enabled={anomalyEnabled} t={t} />
            {item.excludes.length > 0 && (
              <Badge tone="neutral" wrap>
                {t("zfs.excludesCount").replace("{n}", String(item.excludes.length))}
              </Badge>
            )}
            {runFailed && (
              <Badge tone="fail" wrap>
                {t("run.statusFailed")}
              </Badge>
            )}
            {item.lastRunStatus === "cancelled" && (
              <Badge tone="neutral" wrap>
                {t("run.statusCancelled")}
              </Badge>
            )}
            {!runFailed && checkCode === "" && (
              <Badge tone="neutral" wrap>
                {t("zfs.notChecked")}
              </Badge>
            )}
            {checkCode !== "" && checkCode !== "ok" && (
              <Badge tone={RED_CODES.has(checkCode) ? "fail" : "warn"} wrap>
                {zfsCodeSentence(t, checkCode, {
                  hostMountpoint: item.hostMountpoint,
                  leftoverCount: item.leftoverCount,
                })}
                {fixKey && <InfoBubble tip={tLtr(t, fixKey)} />}
              </Badge>
            )}
            {checkCode === "not-found" && (
              <Button
                label={t("zfs.removeMissing")}
                labelKey="zfs.removeMissing"
                tone="subtle"
                onClick={() => void handleRemove()}
              />
            )}
          </div>
          <p dir="ltr" className="mt-1 text-xs font-mono text-carbon-textMuted truncate text-start">
            {item.hostMountpoint}
          </p>
          {checkCode !== "" && item.lastCheckAt > 0 && (
            <p className="mt-1 text-caption text-carbon-textMuted">
              {t("zfs.checkedAt").replace("{when}", relativeTime(t, item.lastCheckAt))}
            </p>
          )}
        </div>

        <div className="ms-auto flex items-start gap-1.5 shrink-0">
          <ZFSBackupButton item={item} t={t} onDone={onRefresh} running={running} />
          <Button
            label={t("common.edit")}
            labelKey="common.edit"
            glyph={<IconPencil />}
            tone="accent"
            onClick={() => setEditing((open) => !open)}
          />
          <Button
            key={shake}
            label={t("common.delete")}
            labelKey="common.delete"
            glyph={<IconTrash />}
            tone="accent"
            onClick={() => void handleRemove()}
            className={shake ? "glim-shake" : ""}
          />
        </div>
      </div>

      <div className="flex items-start">
        <div className="ms-auto">
          <ToggleRow
            label={t("zfs.enabled")}
            checked={enabled}
            onChange={(next) => void handleEnabled(next)}
            disabled={enabledBusy}
          />
        </div>
      </div>

      <EffectiveScheduleLine effective={item.effectiveSchedule} domainLabelKey="jobs.zfsSection" />

      <p className="text-xs text-carbon-textMuted">
        {t("zfs.repoEffective").replace("{repo}", item.repoEffective)}
      </p>

      {item.stopContainers.length > 0 && (
        <p className="text-xs text-carbon-textMuted">
          {t("zfs.consistencyChips").replace("{names}", item.stopContainers.join(", "))}
        </p>
      )}

      <p className="text-xs text-carbon-textMuted">
        {`${t("containers.lastBackup")}: ${item.lastBackup ? formatTs(item.lastBackup) : t("containers.never")}`}
      </p>

      {item.restartPending.length > 0 && (
        <p className="flex items-center gap-1.5 text-xs text-statusFail">
          {t("zfs.restartPending").replace("{names}", item.restartPending.join(", "))}
          <InfoBubble tip={t("zfs.restartPendingHint")} />
        </p>
      )}

      {item.leftoverCount > 0 && (
        <p className="flex items-center gap-1.5 text-xs text-statusWarn">
          {t("zfs.leftovers").replace("{n}", String(item.leftoverCount))}
          <InfoBubble tip={t("zfs.leftoversHint")} />
          <Button
            label={t("zfs.removeLeftovers")}
            labelKey="zfs.removeLeftovers"
            tone="subtle"
            onClick={() => void handleSweep()}
            disabled={sweeping}
            busy={sweeping}
          />
        </p>
      )}

      {skipped.length > 0 && (
        <p className="flex items-center gap-1.5 text-xs text-statusWarn">
          {t("zfs.skippedCount").replace("{n}", String(skipped.length))}
          <InfoBubble tip={skipped.map((m) => m.dataset).join(", ")} />
        </p>
      )}

      <div className="flex flex-col gap-1">
        <button
          type="button"
          aria-expanded={membersOpen}
          onClick={() => setMembersOpen((open) => !open)}
          className="flex items-center gap-1.5 self-start text-xs text-carbon-textSub hover:text-carbon-text"
        >
          <IconDisclosure open={membersOpen} />
          {t("zfs.membersSummary").replace("{n}", String(item.members.length))}
        </button>
        {membersOpen && (
          <ZFSMemberList members={item.members} root={item.dataset} t={t} series={series} targetId={item.id} />
        )}
      </div>

      {item.safetyCount > 0 && <ZFSSafetySection item={item} t={t} onRefresh={onRefresh} />}

      {editing && (
        <ZFSItemSettings
          item={item}
          t={t}
          onChanged={onRefresh}
          anomaly={anomaly}
          anomalyEnabled={anomalyEnabled}
          series={series}
        />
      )}

      <ZFSRestorePanel
        item={item}
        host={host}
        hostMountRoot={hostMountRoot}
        restoreFolder={restoreFolder}
        preselect={restoreRequest}
      />

      <RecentRunsList
        name={item.dataset}
        domain="zfs"
        t={t}
        renderDetail={(run) => <ZFSRunDetailView run={run} item={item} t={t} />}
        refreshKey={`${item.lastBackup}:${item.lastRunStatus}:${progress?.active ? "running" : ""}`}
      />

      {progress && (
        <ProgressBar
          percent={progress.percent}
          active={progress.active}
          label={progress.phase === "restore" ? t("common.restoring") : t("common.backingUp")}
        />
      )}
      {progress?.active && progress.phase !== "restore" && (
        <div className="flex justify-end">
          <BackupCancelButton cancelKey={progressKeyOf(item)} name={item.dataset} t={t} />
        </div>
      )}
      {confirmDialog}
    </div>
  );
}
