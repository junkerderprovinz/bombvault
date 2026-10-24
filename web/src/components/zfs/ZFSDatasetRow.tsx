import { useEffect, useState, type ReactNode } from "react";

import { zfsRunMembers } from "../../lib/api";
import type { Run, ZFSDatasetView, ZFSRunDetail } from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { useT } from "../../lib/i18n";
import { tLtr } from "../../lib/ltrFragments";
import { RunReasonText } from "../../lib/runReason";
import { ZFS_CODE_VARS, zfsCodeSentence, zfsFixKey, zfsMemberKey, zfsRollbackTargets, zfsRunReasonCode } from "../../lib/zfsCodes";
import { Badge } from "../Badge";
import { InfoBubble } from "../InfoBubble";
import { RollbackLine } from "./replica/RollbackLine";

type T = ReturnType<typeof useT>["t"];

// Codes that mean the last look at this item found nothing usable, as opposed
// to one dataset of the tree the reader could mount or unlock.
export const RED_CODES = new Set([
  "not-found",
  "not-filesystem",
  "nothing-readable",
  "snapshot-failed",
  "snapshot-not-visible",
  "snapshot-loop",
  "pre-snapshot-failed",
  "consistency-stop-failed",
]);

/** Why a run failed or stopped, in the reader's language where the code allows
 *  it. A detail the command output below repeats is left out. */
function zfsRunReason(t: T, raw: string, hookDetail: string): ReactNode {
  const parsed = zfsRunReasonCode(raw);
  if (!parsed || ZFS_CODE_VARS[parsed.code]) return <RunReasonText reason={raw} t={t} />;
  const sentence = zfsCodeSentence(t, parsed.code);
  const rollback = zfsRollbackTargets(raw);
  if (rollback.length > 0) {
    return (
      <>
        {sentence}
        {rollback.map((snapshot) => (
          <RollbackLine key={snapshot} snapshot={snapshot} />
        ))}
      </>
    );
  }
  if (!parsed.detail || parsed.detail === hookDetail) return sentence;
  return (
    <>
      {sentence} <bdi dir="ltr">{parsed.detail}</bdi>
    </>
  );
}

export function ZFSRunDetailView({ run, item, t }: { run: Run; item: ZFSDatasetView; t: T }) {
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
