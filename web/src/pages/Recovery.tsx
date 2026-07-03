import { useCallback, useState } from "react";
import { useT } from "../lib/i18n";
import { StepCard, type StepState } from "../components/recovery/StepCard";
import { discover, discoverVMs } from "../lib/api";

// classifyReadable's probe: discover() + discoverVMs() OPEN the encrypted repo
// (they read the mirrored, restic-encrypted definitions), so they are the
// cleanest "can BombVault read your backups?" check with no backend change:
//   - a wrong APP_KEY  -> the mapped "APP_KEY differs" error in {ok:false,error}
//   - a missing/empty repo -> {ok:true, discovered:0}
//   - a readable repo   -> {ok:true, discovered:>0}
// See the report notes for why the snapshot-list probe can't be used pre-discover
// (it needs a container name we don't have on a fresh install).
type DiscoverResult = Awaited<ReturnType<typeof discover>>;

function isKeyMismatch(err: string | undefined): boolean {
  return !!err && /APP_KEY/i.test(err);
}

export default function Recovery() {
  const { t } = useT();

  // Step 1 — repo-readable / APP_KEY state, shared with later steps.
  const [readableState, setReadableState] = useState<StepState>("idle");
  const [lastError, setLastError] = useState<string | null>(null);
  const [checking, setChecking] = useState(false);

  // checkReadable runs the discover probe and classifies the outcome. Shared by
  // Step 1's "Re-check" and (Task 3) Step 2's "Connect & preview".
  const checkReadable = useCallback(async () => {
    setChecking(true);
    setLastError(null);
    try {
      const [c, v] = await Promise.all([discover(), discoverVMs()]);
      const results: DiscoverResult[] = [c, v];
      const keyErr = results.find((r) => !r.ok && isKeyMismatch(r.error));
      if (keyErr) {
        setReadableState("bad");
        setLastError(keyErr.error ?? null);
        return;
      }
      const otherErr = results.find((r) => !r.ok);
      if (otherErr) {
        setReadableState("warn");
        setLastError(otherErr.error ?? null);
        return;
      }
      const total = (c.discovered ?? 0) + (v.discovered ?? 0);
      // >0 = repo readable with content; 0 = reachable but empty / not attached yet.
      setReadableState(total > 0 ? "ok" : "warn");
    } catch (err) {
      // Network/HTTP failure (unreachable, auth, 5xx) — not a key mismatch.
      setReadableState("warn");
      setLastError(err instanceof Error ? err.message : String(err));
    } finally {
      setChecking(false);
    }
  }, []);

  return (
    <div className="flex flex-col gap-5 p-1">
      <div>
        <h1 className="text-lg font-semibold text-carbon-text">{t("recovery.pageTitle")}</h1>
        <p className="text-sm text-carbon-textMuted mt-1 max-w-2xl">{t("recovery.intro")}</p>
      </div>

      {/* Step 1 — Can BombVault read your backups? (repo-readable / APP_KEY) */}
      <StepCard n={1} title={t("recovery.step1")} state={readableState}>
        <p className="max-w-2xl">{t("recovery.appKeyExplain")}</p>

        <div className="flex items-center gap-3 pt-1">
          <button
            onClick={() => void checkReadable()}
            disabled={checking}
            className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {checking && (
              <span
                className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
                style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
              />
            )}
            {checking ? t("dashboard.checking") : t("recovery.recheck")}
          </button>

          {readableState === "ok" && (
            <span className="text-sm text-[#6fdc8c]">{t("recovery.readable")}</span>
          )}
          {readableState === "warn" && (
            <span className="text-sm text-[#f1c21b]">{t("recovery.notReachable")}</span>
          )}
        </div>

        {/* Exact remedy when the key doesn't match the repo. */}
        {readableState === "bad" && (
          <div className="rounded-lg bg-[#2a1c1c] border border-[#4a2a2a] px-3 py-2.5 text-xs text-[#ff8389] leading-relaxed">
            {t("recovery.appKeyRemedy")}
          </div>
        )}

        {/* The raw (scrubbed) backend message for a warn/other error, as a hint. */}
        {readableState === "warn" && lastError && (
          <p className="text-xs text-carbon-textMuted font-mono break-all">{lastError}</p>
        )}
      </StepCard>
    </div>
  );
}
