import { useState, type ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";

import { HeldMark } from "../../components/anomalies/FindingLine";
import type { MenuAction } from "../../components/backups/ActionMenu";
import { MoreMenu } from "../../components/backups/MoreMenu";
import { KIND_SCHEDULE_LABEL, entryName } from "../../components/backups/kinds";
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { useScheduleSentence } from "../../components/EffectiveScheduleLine";
import { IconClearSelection, IconEye, IconRefresh } from "../../components/glyphs";
import { IconBackupNow, IconCheck, IconDownload, IconRecovery } from "../../components/navGlyphs";
import { anomalyEntryName, anomalySentence } from "../../lib/anomalies";
import {
  ApiError,
  ackFailedCopies,
  ackRecoveryKit,
  ackRuns,
  downloadRecoveryKit,
  replicateOffsite,
  type AnomalyView,
  type BackupItem,
  type Run,
} from "../../lib/api";
import { ATTENTION_CHANGED_EVENT } from "../../lib/attention";
import { ENTRY_KIND } from "../../lib/backupEntry";
import { entryTarget } from "../../lib/backupList";
import { useT } from "../../lib/i18n";
import type { Need } from "../../lib/needs";
import { formatList } from "../../lib/placement";
import { formatRecent } from "../../lib/reltime";
import { emergencyPath } from "../../lib/restorePlan";
import { runTargetText } from "../../lib/runDisplay";
import { runKindLabel } from "../../lib/runKind";
import { isOwnReason, RunReasonText } from "../../lib/runReason";
import { DOMAIN_LABEL, locationRoute } from "../../lib/storageLocations";
import { useToast } from "../../lib/toast";
import { useIsDesktop } from "../../lib/useMediaQuery";
import { useDeclineBackup } from "../../lib/useDeclineBackup";
import { Card } from "./Card";

type T = ReturnType<typeof useT>["t"];

// A failed dump is repeated by the backup it belongs to. A failed restore is
// not repeated from here: it would write over the entry a second time.
const REPEATABLE = new Set(["backup", "dbdump"]);

function Row({
  tone,
  title,
  children,
  actions,
  testid,
  newId,
}: {
  tone: "fail" | "warn";
  title: ReactNode;
  children: ReactNode;
  actions: ReactNode;
  testid: string;
  /** The change the row shows after an update, for the dot that marks it. */
  newId?: string;
}) {
  return (
    <li
      data-testid={testid}
      data-new={newId}
      className="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-carbon-border py-3 first:border-t-0"
    >
      <span
        aria-hidden="true"
        className={`mt-1.5 h-2 w-2 flex-none self-start rounded-full ${tone === "fail" ? "bg-statusFailSolid" : "bg-statusWarnSolid"}`}
      />
      <div className="flex min-w-0 flex-1 basis-56 flex-col gap-0.5">
        <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-sm font-semibold text-carbon-text">{title}</div>
        {children}
      </div>
      <div className="flex flex-wrap items-center gap-2 max-md:w-full max-md:ps-5 max-md:[&>*]:flex-1">{actions}</div>
    </li>
  );
}

// On a phone a row keeps to one line of detail; the rest is a tap away on its page.
const SUB = "text-sm text-carbon-textSub wrap-break-word max-md:truncate";
// The height a finger needs, for the actions of a row on a touch screen.
const TOUCH = "pointer-coarse:[--btn-h:2.75rem]";
// Two long labels or three actions share a phone's row only cut short, so
// each keeps the width of its label and the row breaks instead.
const WRAPPING = `${TOUCH} max-md:basis-auto!`;
const NAME_LINK = "min-w-0 wrap-anywhere hover:underline";

function FailedRunRow({
  run,
  item,
  t,
  onErrors,
}: {
  run: Run;
  item?: BackupItem;
  t: T;
  onErrors: () => void;
}) {
  const { push } = useToast();
  const [starting, setStarting] = useState(false);
  const name = runTargetText(t, run);
  const repeatable = !!item && REPEATABLE.has(run.kind);

  function resolve() {
    ackRuns({ ids: [run.id] })
      .then((res) => {
        if (res.ok) window.dispatchEvent(new Event(ATTENTION_CHANGED_EVENT));
        else push(t("common.saveFailed"), "fail");
      })
      .catch((err) => push(err instanceof Error ? err.message : t("common.saveFailed"), "fail"));
  }

  const viewErrors: MenuAction = { id: "errors", label: t("overview.needs.viewErrors"), glyph: <IconEye />, onSelect: onErrors };
  const resolved: MenuAction = { id: "resolve", label: t("errorPanel.resolve"), glyph: <IconCheck />, onSelect: resolve };

  async function retry(entry: BackupItem) {
    setStarting(true);
    try {
      const res = await ENTRY_KIND[entry.kind].backup(entry.key);
      if (res.ok) push(t("containers.backupStarted"), "success");
      else push(res.error ?? t("containers.backupStartFailed"), "fail");
    } catch (err) {
      if (err instanceof ApiError && (err.code === "no-place" || err.code === "no-location")) push(err.message, "warn");
      else if (err instanceof ApiError && err.status === 409) push(t("containers.batchAlreadyRunning"), "warn");
      else push(err instanceof Error ? err.message : t("containers.backupStartFailed"), "fail");
    } finally {
      setStarting(false);
    }
  }

  return (
    <Row
      testid="need-failed-run"
      tone="fail"
      title={
        <>
          {item ? (
            <Link to={entryTarget(item)} className={NAME_LINK}>
              {name}
            </Link>
          ) : (
            <span className="min-w-0 wrap-anywhere">{name}</span>
          )}
          {run.kind !== "backup" && <span className="font-normal text-carbon-textMuted">{runKindLabel(t, run.kind)}</span>}
        </>
      }
      actions={
        <>
          {repeatable ? (
            <Button
              className={WRAPPING}
              label={t("breakdown.retry")}
              labelKey="breakdown.retry"
              glyph={<IconRefresh />}
              onClick={() => void retry(item)}
              disabled={starting}
              busy={starting}
            />
          ) : (
            <Button
              className={WRAPPING}
              label={t("overview.needs.viewErrors")}
              labelKey="overview.needs.viewErrors"
              glyph={<IconEye />}
              onClick={onErrors}
            />
          )}
          <MoreMenu className={WRAPPING} label={name} actions={repeatable ? [resolved, viewErrors] : [resolved]} />
        </>
      }
    >
      <p dir={isOwnReason(run.error) ? undefined : "ltr"} className={`${SUB} text-start`}>
        {run.error ? <RunReasonText reason={run.error} t={t} /> : t("run.statusFailed")}
      </p>
    </Row>
  );
}

function UnprotectedRow({ item, t, onChanged }: { item: BackupItem; t: T; onChanged: () => void }) {
  const navigate = useNavigate();
  const { push } = useToast();
  const sentence = useScheduleSentence();
  const [saving, setSaving] = useState(false);
  const include = item.included ? undefined : ENTRY_KIND[item.kind].setIncluded;
  const why = sentence(item.effectiveSchedule, KIND_SCHEDULE_LABEL[item.kind]);
  const { decline, busy: declining } = useDeclineBackup(onChanged);

  async function start(setIncluded: NonNullable<typeof include>) {
    setSaving(true);
    try {
      const res = await setIncluded(item.key, true);
      if (!res.ok) push(res.error ?? t("common.saveFailed"), "fail");
      onChanged();
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.saveFailed"), "fail");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Row
      testid="need-unprotected"
      newId="overview-decline"
      tone="warn"
      title={
        <>
          <Link to={entryTarget(item)} className={NAME_LINK}>
            {entryName(item, t)}
          </Link>
          {item.lastBackup === 0 && <Badge tone="neutral">{t("overview.needs.new")}</Badge>}
          <Badge tone="warn">{t("backups.status.notProtected")}</Badge>
        </>
      }
      actions={
        <>
          {include ? (
            <Button
              className={WRAPPING}
              label={t("overview.needs.include")}
              labelKey="overview.needs.include"
              glyph={<IconBackupNow />}
              onClick={() => void start(include)}
              disabled={saving || declining}
              busy={saving}
            />
          ) : (
            <Button
              className={WRAPPING}
              label={t("zfs.replica.view")}
              labelKey="zfs.replica.view"
              glyph={<IconEye />}
              onClick={() => navigate(entryTarget(item))}
            />
          )}
          <MoreMenu
            className={WRAPPING}
            label={entryName(item, t)}
            actions={[
              {
                id: "decline",
                label: t("backups.decline"),
                glyph: <IconClearSelection />,
                onSelect: () => {
                  if (!saving && !declining) void decline(item, entryName(item, t));
                },
              },
            ]}
          />
        </>
      }
    >
      {why && <p className={SUB}>{why.text}</p>}
    </Row>
  );
}

function FailedCopyRow({ need, t, lang }: { need: Need & { is: "failedCopy" }; t: T; lang: string }) {
  const navigate = useNavigate();
  const { push } = useToast();
  const [busy, setBusy] = useState<"retry" | "resolve" | null>(null);
  const { location, sections, since } = need;
  const route = locationRoute(location.id);
  const domains = [...new Set(sections.map((section) => section.domain))];

  // The server copies a section to every location it has, which takes in the
  // one that failed.
  async function retry() {
    setBusy("retry");
    try {
      const answers = await Promise.all(domains.map((domain) => replicateOffsite(domain)));
      const refused = answers.find((res) => !res.ok);
      if (refused) push(refused.error ?? t("offsite.replicateFailed"), "fail");
      else push(t("offsite.replicateStarted"), "success");
    } catch (err) {
      push(err instanceof Error ? err.message : t("offsite.replicateFailed"), "fail");
    } finally {
      setBusy(null);
    }
  }

  async function resolve() {
    setBusy("resolve");
    try {
      const res = await ackFailedCopies(sections.flatMap((section) => section.targetId ?? []));
      if (res.ok) window.dispatchEvent(new Event(ATTENTION_CHANGED_EVENT));
      else push(t("common.saveFailed"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.saveFailed"), "fail");
    } finally {
      setBusy(null);
    }
  }

  return (
    <Row
      testid="need-failed-copy"
      tone="fail"
      title={
        <Link to={route} className={NAME_LINK}>
          {t("overview.needs.copyFailed")}
        </Link>
      }
      actions={
        <>
          <Button
            className={WRAPPING}
            label={t("zfs.replica.view")}
            labelKey="zfs.replica.view"
            glyph={<IconEye />}
            onClick={() => navigate(route)}
            busy={busy !== null}
          />
          <MoreMenu
            className={WRAPPING}
            label={t("overview.needs.copyFailed")}
            actions={[
              { id: "retry", label: t("breakdown.retry"), glyph: <IconRefresh />, onSelect: () => void retry() },
              { id: "resolve", label: t("errorPanel.resolve"), glyph: <IconCheck />, onSelect: () => void resolve() },
            ]}
          />
        </>
      }
    >
      <p className={SUB}>
        {[
          location.name,
          formatList(
            lang,
            domains.map((domain) => t(DOMAIN_LABEL[domain]))
          ),
          t("overview.needs.copySince").replace("{when}", formatRecent(since, lang)),
        ].join(" · ")}
      </p>
    </Row>
  );
}

function AnomalyRow({ finding, t, lang }: { finding: AnomalyView; t: T; lang: string }) {
  const navigate = useNavigate();
  const target = `/anomalies?scope=item:${encodeURIComponent(finding.targetId)}`;
  return (
    <Row
      testid="need-anomaly"
      tone={finding.severity === "critical" ? "fail" : "warn"}
      title={
        <>
          <Link to={target} className={NAME_LINK}>
            {t("overview.needs.anomalyAt").replace("{name}", anomalyEntryName(finding, t))}
          </Link>
          {finding.retentionHeld && <HeldMark tip={t("anomaly.retentionPaused")} />}
        </>
      }
      actions={
        <Button
          className={TOUCH}
          label={t("overview.needs.check")}
          labelKey="overview.needs.check"
          glyph={<IconEye />}
          onClick={() => navigate(target)}
        />
      }
    >
      <p className={SUB}>{anomalySentence(finding, t, lang)}</p>
    </Row>
  );
}

function KeyLockedRow({ t }: { t: T }) {
  const navigate = useNavigate();
  return (
    <Row
      testid="need-key-locked"
      tone="fail"
      title={t("overview.needs.keyLocked")}
      actions={
        <Button
          className={TOUCH}
          label={t("entry.config.startRecovery")}
          labelKey={null}
          glyph={<IconRecovery />}
          onClick={() => navigate(emergencyPath("access"))}
        />
      }
    >
      <p className={SUB}>{t("emergency.code.backup-key-locked")}</p>
    </Row>
  );
}

function RecoveryKitRow({ t }: { t: T }) {
  const { push } = useToast();
  const desktop = useIsDesktop();
  const [storing, setStoring] = useState(false);
  // What the server answered instead of the kit, such as its refusal while
  // no login password is set.
  const [refused, setRefused] = useState<string | null>(null);

  function stored() {
    setStoring(true);
    ackRecoveryKit()
      .then((res) => {
        // The row stands for a setting the layout holds, and leaves with it.
        if (res.ok) window.dispatchEvent(new Event("bv:settings-changed"));
        else push(res.error ?? t("common.saveFailed"), "fail");
      })
      .catch((err) => push(err instanceof Error ? err.message : t("common.saveFailed"), "fail"))
      .finally(() => setStoring(false));
  }

  return (
    <Row
      testid="need-recovery-kit"
      tone="warn"
      title={t("recovery.nagTitle")}
      actions={
        <>
          <Button
            className={TOUCH}
            label={t("dbdump.download")}
            labelKey="dbdump.download"
            glyph={<IconDownload />}
            onClick={() => void downloadRecoveryKit().then(setRefused)}
          />
          {desktop ? (
            <Button
              className={TOUCH}
              label={t("recovery.stored")}
              labelKey="recovery.stored"
              glyph={<IconCheck />}
              onClick={stored}
              disabled={storing}
              busy={storing}
            />
          ) : (
            <MoreMenu
              className={WRAPPING}
              label={t("recovery.nagTitle")}
              actions={[{ id: "stored", label: t("recovery.stored"), glyph: <IconCheck />, onSelect: stored }]}
            />
          )}
        </>
      }
    >
      <p className={SUB}>{t("recovery.nagBody")}</p>
      {refused && <p className="text-xs text-statusFail wrap-break-word">{refused}</p>}
    </Row>
  );
}

/**
 * NeedsCard lists what waits for the person, each with the action that
 * settles it: a backup key that does not open, a failed run, an entry
 * nothing backs up, a failed off-site copy, an anomaly, the recovery kit.
 * An entry nothing backs up can also be left that way, which takes it off
 * the list.
 */
export function NeedsCard({
  needs,
  hueIndex,
  onErrors,
  onItemsChanged,
}: {
  needs: Need[];
  hueIndex?: number;
  /** Opens the window in which failures are read and marked as resolved. */
  onErrors: () => void;
  /** An entry was taken into the schedule or set aside. */
  onItemsChanged: () => void;
}) {
  const { t, lang } = useT();
  return (
    <Card title={t("overview.needs.title")} hueIndex={hueIndex}>
      {needs.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("overview.needs.none")}</p>
      ) : (
        <ul className="flex flex-col">
          {needs.map((need) => {
            switch (need.is) {
              case "keyLocked":
                return <KeyLockedRow key="key" t={t} />;
              case "failedRun":
                return <FailedRunRow key={`run:${need.run.id}`} run={need.run} item={need.item} t={t} onErrors={onErrors} />;
              case "unprotected":
                return (
                  <UnprotectedRow
                    key={`entry:${need.item.kind}:${need.item.key}`}
                    item={need.item}
                    t={t}
                    onChanged={onItemsChanged}
                  />
                );
              case "failedCopy":
                return <FailedCopyRow key={`copy:${need.location.id}`} need={need} t={t} lang={lang} />;
              case "anomaly":
                return <AnomalyRow key={`finding:${need.finding.id}`} finding={need.finding} t={t} lang={lang} />;
              case "recoveryKit":
                return <RecoveryKitRow key="kit" t={t} />;
            }
          })}
        </ul>
      )}
    </Card>
  );
}
