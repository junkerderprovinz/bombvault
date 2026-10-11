import { Fragment, type CSSProperties, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { itemOpenCounts } from "../../lib/anomalies";
import type { AnomalyItem } from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { ENTRY_KIND } from "../../lib/backupEntry";
import { inSchedule, isGone, itemProgressKey, protection, runsNow } from "../../lib/backupList";
import { entryReason, type Entry } from "../../lib/entry";
import { stateLabel, useT } from "../../lib/i18n";
import type { ProgressState } from "../../lib/progress";
import { formatRecent } from "../../lib/reltime";
import { BackupCancelButton } from "../BackupCancelButton";
import { runningLabel } from "../backups/BackupRow";
import { KIND_SCHEDULE_LABEL, KindGlyph, kindHue } from "../backups/kinds";
import { Badge } from "../Badge";
import { ContainerChangeNotice } from "../ContainerChangeNotice";
import { useScheduleSentence } from "../EffectiveScheduleLine";
import { InfoBubble } from "../InfoBubble";
import { ItemAnomalyBadge } from "../ItemAnomalyBadge";
import { IconAnomalies } from "../navGlyphs";
import { ProgressBar } from "../ProgressBar";
import { PausedBadge } from "../SchedulePause";

function useReasonText(entry: Entry): string {
  const { t, lang } = useT();
  const sentence = useScheduleSentence();
  const { item } = entry;
  const reason = entryReason(item);
  switch (reason.is) {
    case "gone":
      return t("entry.reason.gone");
    case "declined":
      return t("entry.reason.declined");
    case "lastFailed":
      return t("entry.reason.lastFailed").replace("{when}", formatRecent(reason.lastGood, lang));
    case "lastBackup":
      return t("entry.reason.lastBackup").replace("{when}", formatRecent(reason.at, lang));
    case "pausedNever":
      return t("entry.reason.pausedNever");
    case "nextRun":
      return t("backups.fact.nextRun");
    case "notScheduled":
      return sentence(item.effectiveSchedule, KIND_SCHEDULE_LABEL[item.kind])?.text ?? t("backups.status.notProtected");
  }
}

function ltr(text: string, mono = false): ReactNode {
  return (
    <span dir="ltr" className={`text-start wrap-anywhere ${mono ? "font-mono" : ""}`}>
      {text}
    </span>
  );
}

/** What stands after the reason: whether the container or VM is up, and
 *  where the entry lives. */
function facts(entry: Entry, t: ReturnType<typeof useT>["t"]): ReactNode[] {
  const { detail, hostMountRoot } = entry;
  const gone = isGone(entry.item);
  switch (detail.kind) {
    case "container": {
      const c = detail.container;
      const out: ReactNode[] = [];
      if (!gone) out.push(stateLabel(t, c.state));
      if (!gone && c.ip) out.push(ltr(c.ip, true));
      if (c.image) out.push(ltr(c.image));
      return out;
    }
    case "vm":
      return gone ? [] : [stateLabel(t, detail.vm.state)];
    case "files":
      return detail.set.path === "" ? [] : [ltr(`${hostMountRoot}/${detail.set.path}`, true)];
    default:
      return [];
  }
}

function ProtectionBadge({ entry }: { entry: Entry }) {
  const { t } = useT();
  if (isGone(entry.item)) return <Badge tone="neutral">{t("containers.notInstalled")}</Badge>;
  switch (protection(entry.item)) {
    case "ok":
      return <Badge tone="ok">{t("ransomware.protGreen")}</Badge>;
    case "pending":
      return <Badge tone="active">{t("backups.status.waiting")}</Badge>;
    case "none":
      return <Badge tone="warn">{t("backups.status.notProtected")}</Badge>;
  }
}

function AnomalyState({ anomaly, enabled }: { anomaly?: AnomalyItem; enabled: boolean }) {
  const { t } = useT();
  const open = anomaly ? itemOpenCounts(anomaly) : null;
  const loud = open ? open.critical + open.warning : 0;
  if (!anomaly || loud === 0) return <ItemAnomalyBadge item={anomaly} enabled={enabled} t={t} />;
  return (
    <Link to={`/anomalies?scope=item:${encodeURIComponent(anomaly.targetId)}`}>
      <Badge tone="warn">
        <span className="flex [&_svg]:h-3 [&_svg]:w-3">
          <IconAnomalies />
        </span>
        {loud === 1 ? t("anomaly.runBadge") : t("backups.status.anomalies", loud)}
      </Badge>
    </Link>
  );
}

/**
 * EntryHead is the top of an entry's page: the glyph of its kind, its name,
 * one line that says how it is doing, its state and its rare actions. While a
 * backup or restore of the entry runs, the state gives way to the run.
 */
export function EntryHead({
  entry,
  name,
  progress,
  anomaly,
  anomalyEnabled,
  actions,
}: {
  entry: Entry;
  name: string;
  progress: ProgressState | undefined;
  anomaly?: AnomalyItem;
  anomalyEnabled: boolean;
  actions: ReactNode;
}) {
  const { t } = useT();
  const { item, detail } = entry;
  const reason = useReasonText(entry);
  const running = runsNow(progress);
  const gone = isGone(item);
  const paused = !gone && !inSchedule(item) && ENTRY_KIND[item.kind].setIncluded !== undefined;
  const aliases = (detail.kind === "container" ? detail.container.aliases : detail.kind === "vm" ? detail.vm.aliases : []) ?? [];
  const line = [reason, ...facts(entry, t)];

  return (
    <div
      style={hueVars(kindHue(item.kind)) as CSSProperties}
      className={`glim-hue flex flex-wrap items-center gap-x-4 gap-y-3 ${running ? "glim-active" : ""}`}
    >
      <span data-new="entry-page" className="glim-row-glyph flex h-10 w-10 flex-none items-center justify-center rounded-control bg-carbon-surface2 [&_svg]:h-5 [&_svg]:w-5">
        <KindGlyph kind={item.kind} />
      </span>
      <div className="flex min-w-0 flex-1 basis-64 flex-col gap-0.5">
        <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
          <span
            dir={item.kind === "zfs" ? "ltr" : undefined}
            className="min-w-0 text-status font-semibold text-carbon-text text-start wrap-anywhere"
          >
            {name}
          </span>
          {aliases.length > 0 && (
            <span className="inline-flex items-center gap-1">
              <Badge tone="neutral" size="small" shape="pill">
                {t("takeover.renamed")}
              </Badge>
              <InfoBubble
                tip={`${t("takeover.formerly").replace("{names}", aliases.join(", "))} ${t("takeover.formerlyHint")}`}
              />
            </span>
          )}
          {detail.kind === "container" && (
            <ContainerChangeNotice changes={detail.container.changedSinceBackup} t={t} />
          )}
        </span>
        <span className="text-sm text-carbon-textMuted">
          {line.map((part, i) => (
            <Fragment key={i}>
              {i > 0 && " · "}
              {part}
            </Fragment>
          ))}
        </span>
      </div>
      <div className="flex min-w-0 max-w-full flex-wrap items-center justify-end gap-2 max-md:justify-start">
        {running ? (
          <span className="flex items-center gap-2">
            <span className="flex w-44 min-w-0 flex-col gap-1">
              <span className="flex items-center gap-2 text-dense tabular-nums text-carbon-textSub">
                <span
                  className="inline-block h-2.5 w-2.5 shrink-0 animate-spin rounded-full border-2"
                  style={{ borderColor: "var(--accent)", borderTopColor: "transparent" }}
                />
                <span className="min-w-0 leading-tight wrap-break-word">{runningLabel(progress, t)}</span>
              </span>
              <ProgressBar percent={progress.percent} active inline />
            </span>
            {progress.active && progress.phase !== "restore" && (
              <BackupCancelButton cancelKey={itemProgressKey(item)} name={name} t={t} />
            )}
          </span>
        ) : (
          <>
            {paused && <PausedBadge />}
            <AnomalyState anomaly={anomaly} enabled={anomalyEnabled} />
            <ProtectionBadge entry={entry} />
          </>
        )}
        {actions}
      </div>
    </div>
  );
}
