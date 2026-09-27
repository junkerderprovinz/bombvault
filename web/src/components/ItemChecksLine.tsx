// ItemChecksLine is the part of an item card that says whether the item's
// backup was restored and read back successfully, and for a container whether
// a restored copy of it starts, each with the button that runs the check now.

import { useState } from "react";

import { Badge } from "./Badge";
import { Button } from "./Button";
import { IconPlay } from "./glyphs";
import { IconCheckCircle } from "./Sidebar";
import { probeItem, runStartTest, type ItemChecks, type StartTest, type StartTestBlocked } from "../lib/api";
import { useT, type TranslationKey } from "../lib/i18n";
import { relativeTime } from "../lib/reltime";
import { useToast } from "../lib/toast";

type T = ReturnType<typeof useT>["t"];

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

  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-2 flex-wrap">
        {probe ? (
          <Badge tone={probe.ok ? "ok" : "fail"} size="small" title={t("checks.probeFiles", probe.files)}>
            {probe.ok
              ? `✓ ${t("checks.restoreOk").replace("{time}", relativeTime(t, probe.at))}`
              : `✗ ${t("checks.restoreFailed")}`}
          </Badge>
        ) : (
          <span className="text-xs text-carbon-textMuted">{t("checks.restoreNever")}</span>
        )}
        <span className="ms-auto">
          <Button
            key={shake}
            label={t("checks.probeNow")}
            labelKey="checks.probeNow"
            glyph={<IconCheckCircle />}
            tone="neutral"
            onClick={() => void run()}
            disabled={busy}
            busy={busy}
            title={busy ? t("checks.probeRunning") : undefined}
            hint={t("checks.probeHint")}
            className={shake ? "glim-shake" : ""}
          />
        </span>
      </div>
      {probe && !probe.ok && probe.detail && (
        <span className="text-xs text-statusFail wrap-break-word">{probe.detail}</span>
      )}
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

  if (blocked) {
    return (
      <span className="text-xs text-carbon-textMuted">
        {t("checks.blocked").replace("{reason}", t(BLOCKED_KEYS[blocked]))}
      </span>
    );
  }

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
  const title = test
    ? [mk ? t(mk) : "", t("checks.startDuration", Math.max(1, Math.round(test.durationMs / 1000)))].filter(Boolean).join(" · ")
    : undefined;

  return (
    <>
      <div className="flex items-center gap-2 flex-wrap">
        {test ? (
          <Badge tone={test.ok ? "ok" : "fail"} size="small" title={title}>
            {test.ok
              ? `✓ ${t("checks.startOk").replace("{time}", relativeTime(t, test.at))}`
              : `✗ ${t("checks.startFailed")}`}
          </Badge>
        ) : (
          <span className="text-xs text-carbon-textMuted">{t("checks.startNever")}</span>
        )}
        <span className="ms-auto">
          <Button
            key={shake}
            label={t("checks.startNow")}
            labelKey="checks.startNow"
            glyph={<IconPlay />}
            tone="neutral"
            onClick={() => void run()}
            disabled={busy}
            busy={busy}
            title={busy ? t("checks.startRunning") : undefined}
            hint={t("checks.startHint")}
            className={shake ? "glim-shake" : ""}
          />
        </span>
      </div>
      {test && !test.ok && test.detail && (
        <span className="text-xs text-statusFail wrap-break-word">{test.detail}</span>
      )}
    </>
  );
}
