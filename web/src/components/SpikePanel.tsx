import { useState } from "react";
import { runSpike } from "../lib/api";
import type { SpikeCheck } from "../lib/api";
import type { useT } from "../lib/i18n";
import { Badge } from "./Badge";
import { useTestVerdict } from "../lib/useTestVerdict";
import { TestButton, VerdictLine } from "./TestButton";

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
  const [checks, setChecks] = useState<SpikeCheck[] | null>(null);
  const test = useTestVerdict(null, t("common.checkFailed"));

  // A failed best-effort check is informational, so only a required one fails
  // the run. The table below says which.
  function handleCheck() {
    void test.run(async () => {
      try {
        const res = await runSpike();
        const found = res.checks ?? [];
        setChecks(found);
        return { ok: !found.some((c) => !c.OK && !c.BestEffort) };
      } catch (e) {
        setChecks(null);
        throw e;
      }
    });
  }

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

      <VerdictLine verdict={test.verdict} />
      <div className="flex items-center gap-3">
        <TestButton
          label={t("spike.checkNow")}
          labelKey="spike.checkNow"
          words="check"
          tone="accent"
          test={test}
          onClick={handleCheck}
          hueIndex={hueIndex}
        />
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
