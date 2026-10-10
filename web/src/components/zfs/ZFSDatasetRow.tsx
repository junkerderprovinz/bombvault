import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";

import { backupZFSDataset, deleteZFSDataset, patchZFSDataset, sweepZFSDataset, zfsRunMembers } from "../../lib/api";
import type { AnomalyItem, ItemChecks, Run, ZFSDatasetView, ZFSHostDataset, ZFSRunDetail } from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { useBackupWatch } from "../../lib/backupWatch";
import { humanBytes } from "../../lib/forecast";
import { useT } from "../../lib/i18n";
import { tLtr } from "../../lib/ltrFragments";
import { anyActive, busyPhraseKey, useProgress } from "../../lib/progress";
import { relativeTime, formatTs } from "../../lib/reltime";
import type { RestoreRequest } from "../../lib/restoreRequest";
import { useConfirm } from "../../lib/useConfirm";
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
import { ItemChecksLine } from "../ItemChecksLine";
import { ProgressBar } from "../ProgressBar";
import { RecentRunsList } from "../RecentRunsList";
import { IconBackupNow, IconPencil, IconTrash } from "../Sidebar";
import { ToggleRow } from "../../pages/settings/shared";
import { ZFSMemberList, zfsMemberActionable } from "./ZFSMemberList";
import { ReplicaPlaceRow } from "./replica/ReplicaPlaceRow";
import { ReplicaPlanLine } from "./replica/ReplicaPlanLine";
import { ZFSSitesLine } from "./ZFSSitesLine";
import { ZFSRestorePanel } from "./ZFSRestorePanel";
import { ZFSItemSettings } from "./ZFSItemSettings";
import { ZFSSafetySection } from "./ZFSSafetySection";
import { failText } from "./failText";

type T = ReturnType<typeof useT>["t"];

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
        const fixKey = memberKey ? null : zfsFixKey(m.outcome);
        return (
          <p key={m.dataset} className="flex items-center gap-2 text-caption text-carbon-textSub max-md:flex-wrap">
            <span dir="ltr" className="font-mono text-start max-md:wrap-anywhere">{m.dataset}</span>
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
            {fixKey && <InfoBubble tip={tLtr(t, fixKey)} />}
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
  checks,
  onChecksChanged,
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
  checks?: ItemChecks;
  onChecksChanged?: () => void;
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
      push(t("zfs.deleteKeptSafety", res.safetyRemaining), "warn");
    }
    if (res.leftoversRemaining) {
      push(t("zfs.sweepRemaining", res.leftoversRemaining), "warn");
    }
    onRefresh();
  }

  async function handleSweep() {
    setSweeping(true);
    try {
      const res = await sweepZFSDataset(item.id);
      const remaining = res.remaining ?? 0;
      if (res.ok && remaining === 0) push(t("zfs.sweepDone"), "success");
      else push(t("zfs.sweepRemaining", remaining), "warn");
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
            <span dir="ltr" className="font-semibold text-carbon-text text-sm truncate text-start max-md:whitespace-normal max-md:wrap-anywhere">
              {item.dataset}
            </span>
            <ItemAnomalyBadge item={anomaly} enabled={anomalyEnabled} t={t} />
            {item.excludes.length > 0 && (
              <Badge tone="neutral" wrap>
                {t("zfs.excludesCount", item.excludes.length)}
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
          <p dir="ltr" className="mt-1 text-xs font-mono text-carbon-textMuted truncate text-start max-md:whitespace-normal max-md:wrap-anywhere">
            {item.hostMountpoint}
          </p>
          {checkCode !== "" && item.lastCheckAt > 0 && (
            <p className="mt-1 text-caption text-carbon-textMuted">
              {t("zfs.checkedAt").replace("{when}", relativeTime(t, item.lastCheckAt))}
            </p>
          )}
        </div>

        <div className="ms-auto flex items-start gap-1.5 shrink-0 flex-wrap max-md:w-full max-md:justify-end">
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
      <ReplicaPlanLine itemId={item.id} />
      <ZFSSitesLine item={item} />
      <ReplicaPlaceRow itemId={item.id} name={item.dataset} />

      {item.stopContainers.length > 0 && (
        <p className="text-xs text-carbon-textMuted">
          {t("zfs.consistencyChips").replace("{names}", item.stopContainers.join(", "))}
        </p>
      )}

      <p className="text-xs text-carbon-textMuted">
        {`${t("containers.lastBackup")}: ${item.lastBackup ? formatTs(item.lastBackup) : t("containers.never")}`}
      </p>

      <ItemChecksLine checks={checks} hasBackup={item.lastBackup > 0} onChanged={onChecksChanged} />

      {item.restartPending.length > 0 && (
        <p className="flex items-center gap-1.5 text-xs text-statusFail">
          {t("zfs.restartPending").replace("{names}", item.restartPending.join(", "))}
          <InfoBubble tip={t("zfs.restartPendingHint")} />
        </p>
      )}

      {item.leftoverCount > 0 && (
        <p className="flex flex-wrap items-center gap-1.5 text-xs text-statusWarn">
          {t("zfs.leftovers", item.leftoverCount)}
          <InfoBubble tip={t("zfs.leftoversHint")} />
          <Button
            label={t("zfs.removeLeftovers")}
            labelKey="zfs.removeLeftovers"
            tone="subtle"
            onClick={() => void handleSweep()}
            disabled={sweeping}
            busy={sweeping}
            className="glim-btn-wrap"
          />
        </p>
      )}

      {skipped.length > 0 && (
        <p className="flex items-center gap-1.5 text-xs text-statusWarn">
          {t("zfs.skippedCount", skipped.length)}
          <InfoBubble tip={skipped.map((m) => m.dataset).join(", ")} />
        </p>
      )}

      <div className="flex flex-col gap-1">
        <button
          type="button"
          aria-expanded={membersOpen}
          onClick={() => setMembersOpen((open) => !open)}
          className="flex items-center gap-1.5 self-start text-xs text-carbon-textSub hover:text-carbon-text pointer-coarse:min-h-11"
        >
          <IconDisclosure open={membersOpen} />
          {t("zfs.membersSummary", item.members.length)}
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
