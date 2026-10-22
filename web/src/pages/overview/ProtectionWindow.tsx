import { Link } from "react-router-dom";

import { AnomalyWindow } from "../../components/anomalies/AnomalyWindow";
import { KIND_LABEL, KIND_SCHEDULE_LABEL, entryName } from "../../components/backups/kinds";
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { useScheduleSentence } from "../../components/EffectiveScheduleLine";
import { IconForward, IconPlay } from "../../components/glyphs";
import type { BackupItem } from "../../lib/api";
import { entryTarget, protection, waitingLabel, type Protection } from "../../lib/backupList";
import type { TranslationKey, useT } from "../../lib/i18n";
import { formatTs, relativeTime } from "../../lib/reltime";
import { TONE_DOT } from "./ProtectionCard";
import { VERDICT, canDrill, drilledText, type CheckTone, type Section } from "./protectionChecks";
import type { TrialRestore } from "./useTrialRestore";

type T = ReturnType<typeof useT>["t"];

const VERDICT_BADGE: Record<CheckTone, "ok" | "warn" | "fail" | "active" | "neutral"> = {
  ok: "ok",
  warn: "warn",
  fail: "fail",
  run: "active",
  neutral: "neutral",
};

const ENTRY_STATE: Record<Exclude<Protection, "pending">, [tone: "ok" | "warn", label: TranslationKey]> = {
  ok: ["ok", "ransomware.protGreen"],
  none: ["warn", "backups.status.notProtected"],
};

interface Line {
  key: string;
  tone: CheckTone;
  say: string;
  at: number;
  /** Where the gap is closed, for a line that names one. */
  fix?: string;
}

function CheckLine({ line, t }: { line: Line; t: T }) {
  const body = (
    <>
      <span aria-hidden="true" className={`h-2 w-2 flex-none rounded-full ${TONE_DOT[line.tone]}`} />
      <span className="min-w-0 flex-1 text-sm text-carbon-text">{line.say}</span>
      {line.at > 0 && (
        <span className="flex-none text-xs tabular-nums text-carbon-textMuted" title={formatTs(line.at)}>
          {relativeTime(t, line.at)}
        </span>
      )}
    </>
  );
  const row = "flex items-center gap-3 border-t border-carbon-border px-1.5 py-2.5 first:border-t-0";
  if (!line.fix) return <li className={row}>{body}</li>;
  return (
    <li className="border-t border-carbon-border first:border-t-0">
      <Link to={line.fix} replace className="flex items-center gap-3 rounded-control px-1.5 py-2.5 hover:bg-carbon-hover">
        {body}
        <span className="inline-flex flex-none text-carbon-textMuted">
          <IconForward />
        </span>
      </Link>
    </li>
  );
}

function EntryRow({ item, t }: { item: BackupItem; t: T }) {
  const sentence = useScheduleSentence();
  const state = protection(item);
  const [tone, label] = state === "pending" ? (["neutral", waitingLabel(item)] as const) : ENTRY_STATE[state];
  const why = sentence(item.effectiveSchedule, KIND_SCHEDULE_LABEL[item.kind]);
  return (
    <li className="border-t border-carbon-border first:border-t-0">
      <Link to={entryTarget(item)} replace className="flex items-center gap-3 rounded-control px-1.5 py-2.5 hover:bg-carbon-hover">
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="truncate text-sm font-semibold text-carbon-text">{entryName(item, t)}</span>
          {why && <span className="text-xs text-carbon-textMuted">{why.text}</span>}
        </span>
        <Badge tone={tone}>{t(label)}</Badge>
      </Link>
    </li>
  );
}

/**
 * ProtectionWindow shows what stands behind a section's four dots: each check
 * with its age, what the last verifications found, the trial restore with its
 * button, and the section's entries.
 */
export function ProtectionWindow({
  section,
  trial,
  t,
  onClose,
}: {
  section: Section;
  trial: TrialRestore;
  t: T;
  onClose: () => void;
}) {
  const { status, checks, tally } = section;
  const drilling = trial.running === status.domain;
  const refused = trial.refused[status.domain];
  const drillable = status.offsiteConfigured && canDrill(status.domain);
  const drillFailed = status.lastDrDrillAt > 0 && !status.lastDrDrillOK;
  const drilled = status.lastDrDrillAt > 0 ? drilledText(t, status.lastDrDrillItem, status.lastDrDrillSnapshot) : "";

  const lines: Line[] = [
    ...checks.map((c) => ({
      key: c.key,
      tone: c.tone,
      say: t(c.say),
      at: c.at,
      fix: c.key === "offsite" && c.tone === "fail" ? "/settings/offsite" : undefined,
    })),
    {
      key: "encryption",
      tone: status.encryptionOn ? "ok" : "fail",
      say: t(status.encryptionOn ? "overview.check.encryptionOn" : "overview.check.encryptionOff"),
      at: 0,
    },
    {
      key: "prune",
      tone: status.pruneStrategySet ? "ok" : "warn",
      say: t(status.pruneStrategySet ? "overview.check.pruneSet" : "overview.check.pruneMissing"),
      at: 0,
    },
  ];

  return (
    <AnomalyWindow
      title={t(KIND_LABEL[section.kind])}
      onClose={onClose}
      actions={
        drillable && (
          <Button
            label={t(status.lastDrDrillAt > 0 ? "overview.drill.again" : "overview.drill.start")}
            labelKey={status.lastDrDrillAt > 0 ? "overview.drill.again" : "overview.drill.start"}
            glyph={<IconPlay />}
            onClick={() => trial.start(status.domain)}
            disabled={drilling}
            busy={drilling}
            title={drilling ? t("overview.check.drillRunning") : undefined}
            className="glim-btn-wrap pointer-coarse:[--btn-h:2.75rem]"
          />
        )
      }
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-sm text-carbon-textSub">{t("dashboard.protectionTitle")}</span>
        <Badge tone={VERDICT_BADGE[section.tone]}>{t(VERDICT[section.tone])}</Badge>
      </div>

      <ul className="flex flex-col">
        {lines.map((line) => (
          <CheckLine key={line.key} line={line} t={t} />
        ))}
      </ul>

      {(status.lastVerified > 0 || status.lastOffsiteSubsetAt > 0 || status.lastStartTest || drilled) && (
        <div className="flex flex-wrap gap-1.5">
          {status.lastVerified > 0 && (
            <Badge tone={status.lastVerifiedOK ? "ok" : "fail"} wrap title={status.verifiedDetail || undefined}>
              {t("verify.shield")} · {relativeTime(t, status.lastVerified)}
            </Badge>
          )}
          {status.lastOffsiteSubsetAt > 0 && (
            <Badge tone={status.lastOffsiteSubsetOK ? "ok" : "fail"} wrap>
              {t("drill.offsiteVerified")} · {relativeTime(t, status.lastOffsiteSubsetAt)}
            </Badge>
          )}
          {drilled && (
            <Badge tone={status.lastDrDrillOK ? "ok" : "fail"} wrap>
              {t("verify.trialRestore")} · {drilled}
            </Badge>
          )}
          {status.lastStartTest && (
            <Badge tone={status.lastStartTest.ok ? "ok" : "fail"} wrap title={status.lastStartTest.detail || undefined}>
              {t(status.lastStartTest.ok ? "dashboard.startTestOk" : "dashboard.startTestFailed").replace(
                "{name}",
                status.lastStartTest.container
              )}{" "}
              · {relativeTime(t, status.lastStartTest.at)}
            </Badge>
          )}
        </div>
      )}

      {drillable && ((drillFailed && status.drillDetail && !drilling) || refused) && (
        <div className="flex flex-col items-start gap-2">
          {drillFailed && status.drillDetail && !drilling && (
            <p className="text-xs text-statusFail wrap-break-word">
              {t("drill.failReasonPrefix")} <bdi dir="ltr">{status.drillDetail}</bdi>
            </p>
          )}
          {refused && <p className="text-xs text-statusFail wrap-break-word">{refused}</p>}
        </div>
      )}

      {tally.entries.length > 0 && (
        <div className="flex flex-col gap-1">
          <h3 className="text-xs font-medium uppercase tracking-wider text-carbon-textMuted">{t("overview.protection.entries")}</h3>
          <ul className="flex flex-col">
            {tally.entries.map((item) => (
              <EntryRow key={`${item.kind}:${item.key}`} item={item} t={t} />
            ))}
          </ul>
        </div>
      )}
    </AnomalyWindow>
  );
}
