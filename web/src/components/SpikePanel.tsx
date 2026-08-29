import { useState } from "react";
import { runSpike } from "../lib/api";
import type { SpikeCheck } from "../lib/api";
import type { useT } from "../lib/i18n";
import { Badge } from "./Badge";
import { useToast } from "../lib/toast";
import { Button } from "./Button";

type T = ReturnType<typeof useT>["t"];

// A best-effort (optional) check that failed is informational, not a hard
// failure — distinct WARN-tone "INFO" label, matching Dashboard's own
// chipFor/statusTone mapping for the exact same case. Previously hard-coded,
// untranslated English "OK"/"FAIL"/"INFO" text (GlimStone form-engine Task
// 5) — spike.ok/spike.fail already existed as translated keys (reused
// as-is); spike.info is new, propagated to all 26 locales alongside them.
function StatusChip({ ok, bestEffort, t }: { ok: boolean; bestEffort?: boolean; t: T }) {
  if (bestEffort && !ok) return <Badge tone="warn">{t("spike.info")}</Badge>;
  if (ok) return <Badge tone="ok">{t("spike.ok")}</Badge>;
  return <Badge tone="fail">{t("spike.fail")}</Badge>;
}

interface SpikePanelProps {
  t: T;
  /** GlimStone follow-up round (jdp, live review, five-escalations-deep
   *  standing rule — "IMMER alles in die Farb- und Formengine
   *  integrieren"): the "Check Now" button had no tie to the enclosing
   *  Card's own hue. Threaded straight through from Settings.tsx's own
   *  single `nextHue()` call for this Card — see that call site's own IIFE
   *  comment for why it shares ONE position with the Card's heading notch
   *  rather than a second independent `nextHue()` call. */
  hueIndex?: number;
}

export function SpikePanel({ t, hueIndex }: SpikePanelProps) {
  const { push } = useToast();
  const [checks, setChecks] = useState<SpikeCheck[] | null>(null);
  const [allOk, setAllOk] = useState<boolean | null>(null);
  const [loading, setLoading] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): a failed
  // action shows its message via a TOAST, never as permanent page text, and
  // the button that triggered it replays `.glim-shake` once — the same fix
  // IntegrityCard's own run()/runTamperFor() already established elsewhere in
  // Settings.tsx (commit 108cc93). This replaces the old permanent inline
  // `error` paragraph below the button with `shake` (a bumped nonce, same
  // shape as IntegrityCard's own, so a repeated identical failure still
  // replays the animation — see ToggleRow's own shakeNonce doc comment).
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
        {/* Button-size sweep (jdp, live review: "Die vielen Buttons sind
            unterschiedlich groß"): was `px-4 py-2` — measured live at 36px,
            taller than every sibling primary button on this tab (Generate/
            Install/Confirm, all `px-4 py-1.5` = 32px, the dominant control
            height this whole Settings page already standardizes on). Now
            matches that convention exactly, plus `.glim-hue` per this Card's
            own hueOn/hueStyle above. */}
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
          {/* Table header */}
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
