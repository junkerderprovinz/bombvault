// ItemChecksLine is the row on an item card that says whether the item's
// backup was restored and read back successfully, with the button that runs
// that check now. Children join the status row.

import { useState, type ReactNode } from "react";

import { Badge } from "./Badge";
import { Button } from "./Button";
import { IconCheckCircle } from "./Sidebar";
import { probeItem, type ItemChecks } from "../lib/api";
import { useT } from "../lib/i18n";
import { relativeTime } from "../lib/reltime";
import { useToast } from "../lib/toast";

export function ItemChecksLine({
  checks,
  hasBackup,
  onChanged,
  children,
}: {
  checks?: ItemChecks;
  /** Without a backup there is nothing to read back, so the row stays away. */
  hasBackup: boolean;
  onChanged?: () => void;
  children?: ReactNode;
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
          <Badge
            tone={probe.ok ? "ok" : "fail"}
            size="small"
            title={t("checks.probeFiles", probe.files)}
          >
            {probe.ok
              ? `✓ ${t("checks.restoreOk").replace("{time}", relativeTime(t, probe.at))}`
              : `✗ ${t("checks.restoreFailed")}`}
          </Badge>
        ) : (
          <span className="text-xs text-carbon-textMuted">{t("checks.restoreNever")}</span>
        )}
        {children}
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
    </div>
  );
}
