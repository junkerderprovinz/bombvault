// ItemChecksLine is the part of an item card that says whether the item's
// backup was restored and read back successfully, and for a container whether
// a restored copy of it starts, each with the button that runs the check now.

import { useState, type ReactNode } from "react";

import { Button } from "./Button";
import { IconPlay } from "./glyphs";
import { InfoBubble } from "./InfoBubble";
import { probeItem, runStartTest, type ItemChecks, type StartTest, type StartTestBlocked } from "../lib/api";
import { useT, type TranslationKey } from "../lib/i18n";
import { relativeTime } from "../lib/reltime";
import { useToast } from "../lib/toast";

type T = ReturnType<typeof useT>["t"];

/** One check: its name and what it found on the start, its button on the end. */
function CheckRow({
  label,
  tip,
  state,
  failed,
  detail,
  action,
}: {
  label: string;
  tip: string;
  state: string;
  failed: boolean;
  /** Why the last run failed, behind an (i) after the state. */
  detail?: string;
  action: ReactNode;
}) {
  return (
    <div data-search-row={label} className="flex flex-wrap items-center gap-x-4 gap-y-2">
      <div className="flex min-w-0 flex-1 basis-56 flex-col gap-0.5">
        <span className="flex items-center gap-1.5 text-sm text-carbon-text">
          {label}
          <InfoBubble tip={tip} />
        </span>
        <span className={`flex items-center gap-1.5 text-xs ${failed ? "text-statusFail" : "text-carbon-textMuted"}`}>
          {state}
          {detail && <InfoBubble tip={detail} />}
        </span>
      </div>
      {action}
    </div>
  );
}

export function ItemChecksLine({
  checks,
  hasBackup,
  onChanged,
  startTest = false,
}: {
  checks?: ItemChecks;
  /** Without a backup there is nothing to read back, so the rows stay away. */
  hasBackup: boolean;
  onChanged?: () => void;
  /** Adds the start test row, which only containers have. */
  startTest?: boolean;
}) {
  const { t } = useT();
  const { push } = useToast();
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);

  if (!hasBackup || !checks) return null;
  const probe = checks.probe;
  const targetId = checks.targetId;

  async function run() {
    setBusy(true);
    try {
      const r = await probeItem(targetId);
      if (!r.ok) {
        push(r.error ?? t("checks.restoreFailed"), "fail");
        setShake((n) => n + 1);
      } else if (r.probe && !r.probe.ok) {
        push(r.probe.detail || t("checks.restoreFailed"), "fail");
        setShake((n) => n + 1);
      }
      onChanged?.();
    } catch (err) {
      push(err instanceof Error ? err.message : t("checks.restoreFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  const state = !probe
    ? t("checks.restoreNever")
    : probe.ok
      ? `${t("checks.restoreOk").replace("{time}", relativeTime(t, probe.at))} · ${t("checks.probeFiles", probe.files)}`
      : t("checks.restoreFailed");

  return (
    <div className="flex flex-col divide-y divide-carbon-border/60 [&>*]:py-3 [&>*:first-child]:pt-0 [&>*:last-child]:pb-0">
      <CheckRow
        label={t("overview.check.drill")}
        tip={t("checks.probeHint")}
        state={state}
        failed={probe?.ok === false}
        detail={probe && !probe.ok ? probe.detail : undefined}
        action={
          <Button
            key={shake}
            label={t("checks.probeNow")}
            labelKey="checks.probeNow"
            glyph={<IconPlay />}
            tone="neutral"
            onClick={() => void run()}
            disabled={busy}
            busy={busy}
            title={busy ? t("checks.probeRunning") : undefined}
            className={shake ? "glim-shake" : ""}
          />
        }
      />
      {startTest && <StartTestRow checks={checks} onChanged={onChanged} t={t} />}
    </div>
  );
}

const BLOCKED_KEYS: Record<StartTestBlocked, TranslationKey> = {
  "host-network": "checks.blocked.hostNetwork",
  privileged: "checks.blocked.privileged",
  devices: "checks.blocked.devices",
  "host-namespace": "checks.blocked.hostNamespace",
  "depends-on": "checks.blocked.dependsOn",
  "no-definition": "checks.blocked.noDefinition",
};

/** The key that names how a start test judged its copy. */
function methodKey(method: StartTest["method"]): TranslationKey | null {
  if (method === "health") return "checks.startMethod.health";
  if (method === "tcp") return "checks.startMethod.tcp";
  if (method === "running") return "checks.startMethod.running";
  return null;
}

function StartTestRow({ checks, onChanged, t }: { checks: ItemChecks; onChanged?: () => void; t: T }) {
  const { push } = useToast();
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const test = checks.startTest;
  const blocked = checks.startTestBlocked;

  async function run() {
    setBusy(true);
    try {
      const r = await runStartTest(checks.targetId);
      if (!r.ok) {
        push(r.error ?? t("checks.startFailed"), "fail");
        setShake((n) => n + 1);
      } else if (r.startTest && !r.startTest.ok) {
        push(r.startTest.detail || t("checks.startFailed"), "fail");
        setShake((n) => n + 1);
      }
      onChanged?.();
    } catch (err) {
      push(err instanceof Error ? err.message : t("checks.startFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  const mk = test ? methodKey(test.method) : null;
  let state: string;
  if (blocked) state = t("checks.blocked").replace("{reason}", t(BLOCKED_KEYS[blocked]));
  else if (!test) state = t("checks.startNever");
  else if (!test.ok) state = t("checks.startFailed");
  else {
    state = [
      t("checks.startOk").replace("{time}", relativeTime(t, test.at)),
      mk ? t(mk) : "",
      t("checks.startDuration", Math.max(1, Math.round(test.durationMs / 1000))),
    ]
      .filter(Boolean)
      .join(" · ");
  }

  return (
    <CheckRow
      label={t("settings.startTest")}
      tip={t("checks.startHint")}
      state={state}
      failed={!blocked && test?.ok === false}
      detail={!blocked && test && !test.ok ? test.detail : undefined}
      action={
        <Button
          key={shake}
          label={t("checks.startNow")}
          labelKey="checks.startNow"
          glyph={<IconPlay />}
          tone="neutral"
          onClick={() => void run()}
          disabled={busy || !!blocked}
          busy={busy}
          title={busy ? t("checks.startRunning") : undefined}
          className={shake ? "glim-shake" : ""}
        />
      }
    />
  );
}
