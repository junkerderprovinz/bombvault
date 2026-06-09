import { useState } from "react";
import { runSpike } from "../lib/api";
import type { SpikeCheck } from "../lib/api";
import type { useT } from "../lib/i18n";

type T = ReturnType<typeof useT>["t"];

function StatusChip({ ok, bestEffort }: { ok: boolean; bestEffort?: boolean }) {
  if (bestEffort && !ok) {
    return (
      <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-[#2a2a1c] text-[#f1c21b] border border-[#4a4a2a]">
        INFO
      </span>
    );
  }
  if (ok) {
    return (
      <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]">
        OK
      </span>
    );
  }
  return (
    <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-[#3a1c1c] text-[#ff8389] border border-[#5a2a2a]">
      FAIL
    </span>
  );
}

interface SpikePanelProps {
  t: T;
}

export function SpikePanel({ t }: SpikePanelProps) {
  const [checks, setChecks] = useState<SpikeCheck[] | null>(null);
  const [allOk, setAllOk] = useState<boolean | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleCheck() {
    setLoading(true);
    setError(null);
    try {
      const res = await runSpike();
      setChecks(res.checks ?? []);
      setAllOk(res.allOk);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Check failed";
      setError(msg);
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
      {/* Explanation */}
      <p className="text-sm text-carbon-textSub leading-relaxed">
        The host-integration spike verifies that BombVault can reach the tools
        and paths it needs to perform backups and restores: Docker socket access,
        restic binary presence, path writability under the mount root, and
        optional tools (qemu-img, rclone, libvirt) for future domain support.
        Required checks must pass; optional (best-effort) checks are informational
        only and will not block backups.
      </p>

      <div className="flex items-center gap-3">
        <button
          onClick={() => void handleCheck()}
          disabled={loading}
          className="inline-flex items-center gap-2 rounded-lg bg-carbon-surface3 px-4 py-2 text-sm font-medium text-carbon-text hover:bg-carbon-hover transition-colors disabled:opacity-50"
        >
          {loading ? (
            <>
              <span className="h-3.5 w-3.5 rounded-full border-2 border-[#78a9ff] border-t-transparent animate-spin" />
              {t("dashboard.checking")}
            </>
          ) : (
            t("spike.checkNow")
          )}
        </button>

        {allOk !== null && !loading && (
          <span
            className={`text-sm font-medium ${
              !hasRequiredFails ? "text-[#6fdc8c]" : "text-[#ff8389]"
            }`}
          >
            {!hasRequiredFails ? t("spike.allOk") : t("spike.degraded")}
          </span>
        )}
      </div>

      {error && (
        <p className="text-xs text-[#ff8389]">{error}</p>
      )}

      {checks && checks.length > 0 && (
        <div className="rounded-lg border border-carbon-border overflow-hidden">
          {/* Table header */}
          <div className="grid grid-cols-[8rem_5rem_1fr_5rem] gap-x-3 bg-carbon-surface2 px-3 py-2 text-xs font-semibold text-carbon-textMuted uppercase tracking-wider">
            <span>{t("spike.colCheck")}</span>
            <span>{t("spike.colStatus")}</span>
            <span>{t("spike.colDetail")}</span>
            <span className="text-right">{t("spike.bestEffort")}</span>
          </div>
          {checks.map((c) => (
            <div
              key={c.Name}
              className="grid grid-cols-[8rem_5rem_1fr_5rem] gap-x-3 items-center px-3 py-2.5 border-t border-carbon-border text-sm"
            >
              <span className="font-mono text-carbon-text text-xs">{c.Name}</span>
              <StatusChip ok={c.OK} bestEffort={c.BestEffort} />
              <span className="text-carbon-textMuted text-xs break-words">
                {c.Detail || "—"}
              </span>
              <span className="text-right text-xs text-carbon-textMuted">
                {c.BestEffort ? "optional" : "required"}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
