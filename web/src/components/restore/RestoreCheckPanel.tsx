// RestoreCheckPanel shows the pre-flight checklist of a restore and its plan:
// what the restore adds, replaces and leaves, how the definition it recreates
// differs from the one in use, and which other containers share its folders.
// The caller owns the check (useRestoreCheck) and gates its Start button on it.

import { useId, useState } from "react";
import type { CheckLine, DefinitionChange, PlanChange, RestorePlan } from "../../lib/api";
import type { TranslationKey, useT } from "../../lib/i18n";
import { humanBytes } from "../../lib/forecast";
import { CHECK_LINE_LABEL, type RestoreCheck } from "../../lib/useRestoreCheck";
import { Badge, type BadgeTone } from "../Badge";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import { IconDisclosure } from "../IconDisclosure";
import { IconSync } from "../Sidebar";

type T = ReturnType<typeof useT>["t"];

const STATUS_TONE: Record<CheckLine["status"], BadgeTone> = {
  ok: "ok",
  fail: "fail",
  skip: "neutral",
};
const STATUS_LABEL: Record<CheckLine["status"], TranslationKey> = {
  ok: "restoreCheck.status.ok",
  fail: "restoreCheck.status.fail",
  skip: "restoreCheck.status.skip",
};

const REASON: Record<string, TranslationKey> = {
  unencrypted: "restoreCheck.reason.unencrypted",
  "no-repository": "restoreCheck.reason.noRepository",
  "no-snapshot": "restoreCheck.reason.noSnapshot",
  download: "restoreCheck.reason.download",
  "nothing-written": "restoreCheck.reason.nothing",
  unmeasured: "restoreCheck.reason.unmeasured",
};

const CHANGE_LABEL: Record<PlanChange, TranslationKey> = {
  added: "restoreCheck.change.added",
  changed: "restoreCheck.change.changed",
  removed: "restoreCheck.change.removed",
  extra: "restoreCheck.change.extra",
};

const DEF_CHANGE_LABEL: Record<PlanChange, TranslationKey> = {
  added: "restoreCheck.def.added",
  changed: "restoreCheck.def.changed",
  removed: "restoreCheck.def.removed",
  extra: "restoreCheck.def.removed",
};

const FIELD_LABEL: Record<DefinitionChange["field"], TranslationKey> = {
  image: "restoreCheck.field.image",
  port: "restoreCheck.field.port",
  env: "restoreCheck.field.env",
  volume: "restoreCheck.field.volume",
  memory: "restoreCheck.field.memory",
  vcpus: "restoreCheck.field.vcpus",
  disk: "restoreCheck.field.disk",
  network: "restoreCheck.field.network",
};

function lineText(c: CheckLine, t: T): string {
  if (c.id === "space" && (c.status === "ok" || c.reason === "short") && c.free !== undefined) {
    return t("restoreCheck.space")
      .replace("{need}", humanBytes(c.need ?? 0))
      .replace("{free}", humanBytes(c.free ?? 0));
  }
  if (c.detail)
    return c.id === "snapshot" && c.status === "ok" ? t("restoreCheck.snapshotId").replace("{id}", c.detail) : c.detail;
  return c.reason && REASON[c.reason] ? t(REASON[c.reason]) : "";
}

/** check.recheck is left out where the result is a finished answer, as in a
 *  confirm dialog. */
export function RestoreCheckPanel({
  check,
  t,
}: {
  check: Pick<RestoreCheck, "state"> & { recheck?: () => void };
  t: T;
}) {
  const { state } = check;
  if (state.phase === "idle") return null;
  const lines = state.phase === "done" ? (state.result.checks ?? []) : [];
  const plan = state.phase === "done" ? state.result.plan : null;
  const members = state.phase === "done" ? (state.result.members ?? []) : [];

  return (
    <section
      className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3"
      aria-live="polite"
      aria-busy={state.phase === "running"}
    >
      <div className="flex flex-wrap items-center gap-2">
        <h4 className="text-sm font-semibold text-carbon-text">{t("restoreCheck.title")}</h4>
        <InfoBubble tip={t("restoreCheck.titleHint")} />
        {state.phase === "running" && (
          <span className="text-caption text-carbon-textMuted">{t("restoreCheck.running")}</span>
        )}
        {check.recheck && (
          <Button
            label={t("restoreCheck.recheck")}
            labelKey="restoreCheck.recheck"
            glyph={<IconSync />}
            variant="icon"
            tone="subtle"
            onClick={check.recheck}
            disabled={state.phase === "running"}
            className="ms-auto"
          />
        )}
      </div>
      {state.phase === "error" && (
        <p className="text-sm text-statusFail">{t("restoreCheck.failed").replace("{error}", state.error)}</p>
      )}
      {lines.length > 0 && <CheckLines lines={lines} t={t} />}
      {plan && <PlanView plan={plan} t={t} />}
      {members.map((m) => (
        <div key={m.name} className="flex flex-col gap-2 border-t border-carbon-border pt-2">
          <h5 className="font-mono text-sm font-semibold text-carbon-text" dir="ltr">
            {m.name}
          </h5>
          <CheckLines lines={m.checks} t={t} />
          {m.plan && <PlanView plan={m.plan} t={t} />}
        </div>
      ))}
    </section>
  );
}

function CheckLines({ lines, t }: { lines: CheckLine[]; t: T }) {
  return (
    <ul className="flex flex-col gap-1.5">
      {lines.map((c) => (
        <li key={c.id} className="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-sm">
          <Badge tone={STATUS_TONE[c.status]} size="small" shape="pill">
            {t(STATUS_LABEL[c.status])}
          </Badge>
          <span className="text-carbon-text">{t(CHECK_LINE_LABEL[c.id])}</span>
          {lineText(c, t) && (
            <span className="min-w-0 wrap-break-word text-caption text-carbon-textMuted">{lineText(c, t)}</span>
          )}
        </li>
      ))}
    </ul>
  );
}

function PlanView({ plan, t }: { plan: RestorePlan; t: T }) {
  const [open, setOpen] = useState(false);
  const listId = useId();
  const counts: [number, TranslationKey][] = [
    [plan.added, "restoreCheck.plan.added"],
    [plan.changed, "restoreCheck.plan.changed"],
    [plan.unchanged, "restoreCheck.plan.unchanged"],
  ];
  return (
    <div className="flex flex-col gap-2 border-t border-carbon-border pt-2">
      <div className="flex flex-wrap items-center gap-2">
        <h5 className="text-sm font-semibold text-carbon-text">{t("restoreCheck.plan.title")}</h5>
        <InfoBubble tip={t("restoreCheck.plan.titleHint")} />
      </div>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-carbon-text">
        {counts.map(([n, key]) => (
          <span key={key}>{t(key, n)}</span>
        ))}
        <span className="inline-flex items-center gap-1">
          {t("restoreCheck.plan.extra", plan.extra)}
          <InfoBubble tip={t("restoreCheck.plan.extraHint")} />
        </span>
      </div>
      {plan.partial && <p className="text-caption text-statusWarn">{t("restoreCheck.plan.partial")}</p>}
      {plan.error && (
        <p className="text-caption text-statusWarn">{t("restoreCheck.plan.error").replace("{error}", plan.error)}</p>
      )}
      {plan.files.length > 0 && (
        <div className="flex flex-col gap-1">
          <Button
            label={open ? t("restoreCheck.plan.hideFiles") : t("restoreCheck.plan.showFiles")}
            labelKey={open ? "restoreCheck.plan.hideFiles" : "restoreCheck.plan.showFiles"}
            glyph={<IconDisclosure open={open} />}
            tone="subtle"
            onClick={() => setOpen((o) => !o)}
            ariaExpanded={open}
            ariaControls={listId}
            hint={plan.listCapped ? t("restoreCheck.plan.capped") : undefined}
            className="self-start"
          />
          {open && (
            <ul id={listId} className="max-h-64 overflow-y-auto rounded-card bg-carbon-background p-2 text-caption">
              {plan.files.map((f) => (
                <li key={f.change + f.path} className="flex items-baseline gap-2">
                  <span className="min-w-20 shrink-0 text-carbon-textMuted">{t(CHANGE_LABEL[f.change])}</span>
                  <span className="min-w-0 break-all font-mono text-carbon-text" dir="ltr">
                    {f.path}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
      {plan.missing && <p className="text-sm text-carbon-text">{t("restoreCheck.def.missing")}</p>}
      {plan.definition.length > 0 && (
        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            <h5 className="text-sm font-semibold text-carbon-text">{t("restoreCheck.def.title")}</h5>
            <InfoBubble tip={t("restoreCheck.def.titleHint")} />
          </div>
          <ul className="flex flex-col gap-1 text-caption">
            {plan.definition.map((d, i) => (
              <li key={i} className="flex flex-wrap items-baseline gap-x-2">
                <span className="font-semibold text-carbon-text">{t(FIELD_LABEL[d.field])}</span>
                {d.name && (
                  <span className="font-mono text-carbon-text" dir="ltr">
                    {d.name}
                  </span>
                )}
                <span className="text-carbon-textMuted">{t(DEF_CHANGE_LABEL[d.change])}</span>
                {(d.backup || d.now) && (
                  <span className="min-w-0 break-all font-mono text-carbon-text" dir="ltr">
                    {d.backup && d.now
                      ? t("restoreCheck.def.values").replace("{backup}", d.backup).replace("{now}", d.now)
                      : d.backup || d.now}
                  </span>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
      {plan.shared.length > 0 && (
        <div className="flex flex-col gap-1 rounded-card bg-statusWarnBg p-2 text-sm text-carbon-text">
          <div className="flex items-center gap-2">
            <span className="font-semibold text-statusWarn">{t("restoreCheck.shared.title")}</span>
            <InfoBubble tip={t("restoreCheck.shared.hint")} />
          </div>
          {plan.shared.map((s) => (
            <p key={s.path} className="wrap-break-word">
              {t("restoreCheck.shared.line").replace("{path}", s.path).replace("{names}", s.containers.join(", "))}
            </p>
          ))}
        </div>
      )}
    </div>
  );
}
