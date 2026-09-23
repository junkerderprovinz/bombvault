import { useState } from "react";
import { runSpike } from "../lib/api";
import type { SpikeCheck } from "../lib/api";
import type { useT } from "../lib/i18n";
import { Badge } from "./Badge";
import { useToast } from "../lib/toast";
import { Button } from "./Button";

type T = ReturnType<typeof useT>["t"];

// A failed best-effort check is informational, so it gets the warn tone, as on
// the Dashboard.
function StatusChip({ ok, bestEffort, t }: { ok: boolean; bestEffort?: boolean; t: T }) {
  if (bestEffort && !ok) return <Badge tone="warn">{t("spike.info")}</Badge>;
  if (ok) return <Badge tone="ok">{t("spike.ok")}</Badge>;
  return <Badge tone="fail">{t("spike.fail")}</Badge>;
}

interface SpikePanelProps {
  t: T;
  /** Hue of the enclosing card, so the button matches its heading. */
  hueIndex?: number;
}

export function SpikePanel({ t, hueIndex }: SpikePanelProps) {
  const { push } = useToast();
  const [checks, setChecks] = useState<SpikeCheck[] | null>(null);
  const [allOk, setAllOk] = useState<boolean | null>(null);
  const [loading, setLoading] = useState(false);
  // A failure shows a toast and shakes the button. The counter is bumped on
  // every failure so a repeated one replays the animation.
  const [shake, setShake] = useState(0);

  async function handleCheck() {
    setLoading(true);
    try {
      const res = await runSpike();
      setChecks(res.checks ?? []);
      setAllOk(res.allOk);
    } catch (err) {
      const msg = err instanceof Error ? err.message : t("common.checkFailed");
      push(msg, "fail");
      setShake((n) => n + 1);
      setChecks(null);
      setAllOk(false);
    } finally {
      setLoading(false);
    }
  }

  const hasRequiredFails = checks
    ? checks.some((c) => !c.OK && !c.BestEffort)
    : false;

  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-carbon-textSub leading-relaxed">
        The host-integration spike verifies that BombVault can reach the tools
        and paths it needs to perform backups and restores: Docker socket access,
        restic binary presence, path writability under the mount root, and
        optional tools (qemu-img, rclone, libvirt) for future domain support.
        Required checks must pass; optional (best-effort) checks are informational
        only and will not block backups.
      </p>

      <div className="flex items-center gap-3">
        <Button
          label={t("spike.checkNow")}
          labelKey="spike.checkNow"
          tone="accent"
          onClick={() => void handleCheck()}
          disabled={loading}
          busy={loading}
          title={loading ? t("dashboard.checking") : undefined}
          className={shake ? "glim-shake" : ""}
          hueIndex={hueIndex}
        />

        {allOk !== null && !loading && (
          <span
            className={`text-sm font-medium ${
              !hasRequiredFails ? "text-statusOk" : "text-statusFail"
            }`}
          >
            {!hasRequiredFails ? t("spike.allOk") : t("spike.degraded")}
          </span>
        )}
      </div>

      {checks && checks.length > 0 && (
        <div className="rounded-card overflow-hidden">
          <div className="grid grid-cols-[8rem_5rem_1fr_5rem] gap-x-3 bg-carbon-surface2 px-3 py-2 text-xs font-semibold text-carbon-textMuted uppercase tracking-wider">
            <span>{t("spike.colCheck")}</span>
            <span>{t("spike.colStatus")}</span>
            <span>{t("spike.colDetail")}</span>
            <span className="text-end">{t("spike.bestEffort")}</span>
          </div>
          {checks.map((c) => (
            <div
              key={c.Name}
              className="grid grid-cols-[8rem_5rem_1fr_5rem] gap-x-3 items-center px-3 py-2.5 border-t border-carbon-border text-sm"
            >
              <span className="font-mono text-carbon-text text-xs">{c.Name}</span>
              <StatusChip ok={c.OK} bestEffort={c.BestEffort} t={t} />
              <span className="text-carbon-textMuted text-xs wrap-break-word">
                {c.Detail || "—"}
              </span>
              <span className="text-end text-xs text-carbon-textMuted">
                {c.BestEffort ? "optional" : "required"}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
