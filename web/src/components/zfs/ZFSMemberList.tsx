import { Link } from "react-router-dom";

import { anomalySeverityTone, worstSeverity } from "../../lib/anomalies";
import type { AnomalySeriesInfo, ZFSMemberView } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { tLtr } from "../../lib/ltrFragments";
import { zfsCodeSentence, zfsFixKey, zfsMemberKey } from "../../lib/zfsCodes";
import { Badge } from "../Badge";
import { InfoBubble } from "../InfoBubble";
import { Toggle } from "../Toggle";

type T = ReturnType<typeof useT>["t"];

// A dataset with canmount=off and almost nothing in it is the usual parent of
// a set of child datasets, not data nobody can read.
const STRUCTURE_LIMIT = 1024 * 1024;

function isStructure(m: ZFSMemberView): boolean {
  return m.outcome === "canmount-off" && m.usedByDataset < STRUCTURE_LIMIT;
}

/** Whether a member's state is one the reader can act on. Volumes and empty
 *  structure datasets stay in the list, but no setting on this page would ever
 *  back them up, so they are not counted as skipped. */
export function zfsMemberActionable(m: ZFSMemberView): boolean {
  if (zfsMemberKey(m.outcome)) return false;
  return m.outcome !== "zvol" && !isStructure(m);
}

function depthOf(relPath: string): number {
  return relPath === "" ? 0 : relPath.split("/").length;
}

/** ZFSMemberList shows one item's tree: every dataset below the root with what
 *  became of it, and, in the item's settings, a switch that leaves one out. */
export function ZFSMemberList({
  members,
  root,
  t,
  excluded,
  onToggle,
  busy,
  series,
  targetId,
}: {
  members: ZFSMemberView[];
  /** The item's root dataset, the name the first line carries. */
  root: string;
  t: T;
  /** Datasets the user has switched off, so the tree can show the pending
   *  state before the next check rewrites the outcomes. */
  excluded?: Set<string>;
  /** Given, every child gets a switch. A volume never does: it cannot be a
   *  member at all. */
  onToggle?: (dataset: string, include: boolean) => void;
  busy?: boolean;
  /** What anomaly detection knows about each dataset, by name. */
  series?: ReadonlyMap<string, AnomalySeriesInfo>;
  /** The item the findings belong to, for the link to them. */
  targetId?: string;
}) {
  return (
    <ul className="flex flex-col gap-1">
      {members.map((m) => {
        const off = excluded?.has(m.dataset) ?? false;
        const outcome = off ? "excluded" : m.outcome;
        const memberKey = zfsMemberKey(outcome);
        const fixKey = memberKey ? null : zfsFixKey(outcome);
        const structure = isStructure(m);
        const label = memberKey
          ? t(memberKey)
          : structure
            ? t("zfs.member.structure")
            : zfsCodeSentence(t, outcome, { hostMountpoint: m.hostMountpoint });
        const tone = memberKey || structure || outcome === "zvol" ? "text-carbon-textMuted" : "text-statusWarn";
        return (
          <li
            key={m.dataset}
            className="flex items-center gap-2 text-xs"
            style={{ paddingInlineStart: `${depthOf(m.relPath) * 12}px` }}
          >
            <span dir="ltr" className="font-mono text-carbon-textSub text-start truncate">
              {m.relPath === "" ? root : m.relPath}
            </span>
            {m.isNew && (
              <Badge tone="active" size="small">
                {t("zfs.member.new")}
              </Badge>
            )}
            <span className={tone}>{label}</span>
            {fixKey && <InfoBubble tip={tLtr(t, fixKey)} />}
            <DatasetAnomalies dataset={m.dataset} info={series?.get(m.dataset)} targetId={targetId} t={t} />
            {onToggle && m.relPath !== "" && m.outcome !== "zvol" && (
              <Toggle
                hideLabel
                label={m.dataset}
                checked={!off}
                disabled={busy}
                onChange={(next) => onToggle(m.dataset, next)}
                className="ms-auto"
              />
            )}
          </li>
        );
      })}
    </ul>
  );
}

/** A dataset's open findings, and whether its old backups are kept because of
 *  one. Each dataset of a tree is watched on its own, so the tree is where a
 *  child that lost its data shows. */
function DatasetAnomalies({
  dataset,
  info,
  targetId,
  t,
}: {
  dataset: string;
  info?: AnomalySeriesInfo;
  targetId?: string;
  t: T;
}) {
  if (!info) return null;
  const open = info.open.critical + info.open.warning + info.open.info;
  const worst = worstSeverity(info.open);
  return (
    <>
      {open > 0 && worst && targetId && (
        <Link
          to={`/anomalies?scope=item:${encodeURIComponent(targetId)}#findings`}
          aria-label={t("anomaly.itemBadgeAria").replace("{name}", dataset).replace("{n}", open.toLocaleString())}
        >
          <Badge tone={anomalySeverityTone(worst)} size="small" shape="pill">
            {open}
          </Badge>
        </Link>
      )}
      {info.retentionHeld && <span className="text-statusWarn">{t("anomaly.retentionPaused")}</span>}
    </>
  );
}
