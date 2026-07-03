import { useCallback, useEffect, useState } from "react";
import { useT } from "../lib/i18n";
import { StepCard, type StepState } from "../components/recovery/StepCard";
import { FolderBrowser } from "../components/FolderBrowser";
import { CloudCard, RcloneCard, ToggleRow } from "./Settings";
import { discover, discoverVMs, getSettings, putSettings, type Settings } from "../lib/api";

// classifyReadable's probe: discover() + discoverVMs() OPEN the encrypted repo
// (they read the mirrored, restic-encrypted definitions), so they are the
// cleanest "can BombVault read your backups?" check with no backend change:
//   - a wrong APP_KEY  -> the mapped "APP_KEY differs" error in {ok:false,error}
//   - a missing/empty repo -> {ok:true, discovered:0}
//   - a readable repo   -> {ok:true, discovered:>0}
// See the report notes for why the snapshot-list probe can't be used pre-discover
// (it needs a container name we don't have on a fresh install).
type DiscoverResult = Awaited<ReturnType<typeof discover>>;
type SaveState = "idle" | "saving" | "saved" | "error";

function isKeyMismatch(err: string | undefined): boolean {
  return !!err && /APP_KEY/i.test(err);
}

export default function Recovery() {
  const { t } = useT();

  // Step 1 — repo-readable / APP_KEY state, shared with later steps.
  const [readableState, setReadableState] = useState<StepState>("idle");
  const [lastError, setLastError] = useState<string | null>(null);
  const [checking, setChecking] = useState(false);

  // Step 2 — attach settings. Own copy of the settings object; persisted through
  // the SAME putSettings/setCloud/setRclone the Settings page uses (CloudCard and
  // RcloneCard self-persist; paths/off-site/encryption go through the mirrored
  // merge-onto-baseline save below — no new endpoint, no duplicate storage).
  const [settings, setSettings] = useState<Settings | null>(null);
  const [savedSettings, setSavedSettings] = useState<Settings | null>(null);
  const [hostMountRoot, setHostMountRoot] = useState<string>("/host/user");
  const [attachState, setAttachState] = useState<SaveState>("idle");
  const [attachError, setAttachError] = useState<string | null>(null);
  const [previewed, setPreviewed] = useState(false);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) {
          setSettings(res.settings);
          setSavedSettings(res.settings);
          if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
        }
      })
      .catch(() => undefined);
  }, []);

  // checkReadable runs the discover probe and classifies the outcome. Shared by
  // Step 1's "Re-check" and Step 2's "Connect & preview".
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

  // connectPreview saves the paths/off-site/encryption fields (mirroring the
  // Settings save() merge onto the server baseline), then re-runs checkReadable
  // so Step 1's pill reflects the freshly-attached location.
  const connectPreview = useCallback(async () => {
    const base = savedSettings ?? settings;
    if (!base || !settings) return;
    setAttachState("saving");
    setAttachError(null);
    const patch: Partial<Settings> = {
      containersPath: settings.containersPath,
      vmsPath: settings.vmsPath,
      flashPath: settings.flashPath,
      containersOffsite: settings.containersOffsite,
      vmsOffsite: settings.vmsOffsite,
      flashOffsite: settings.flashOffsite,
      encryptionEnabled: settings.encryptionEnabled,
    };
    const updated: Settings = { ...base, ...patch };
    try {
      const res = await putSettings(updated);
      if (res.ok) {
        setSavedSettings(updated);
        setSettings((prev) => (prev ? { ...prev, ...patch } : updated));
        setAttachState("saved");
        // Keep the sidebar/Settings in sync (same event the Settings page fires).
        window.dispatchEvent(new Event("bv:settings-changed"));
        setTimeout(() => setAttachState("idle"), 3000);
        setPreviewed(true);
        await checkReadable();
      } else {
        setAttachError(res.error ?? t("settings.error"));
        setAttachState("error");
      }
    } catch (err) {
      setAttachError(err instanceof Error ? err.message : t("settings.error"));
      setAttachState("error");
    }
  }, [savedSettings, settings, checkReadable, t]);

  const offsiteInput =
    "rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono focus:outline-none focus:ring-1 focus:ring-accent";

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

      {/* Step 2 — Attach your backups (consolidated; cloud creds un-gated here) */}
      <StepCard n={2} title={t("recovery.step2")} state={previewed ? readableState : "idle"}>
        <p className="max-w-2xl">{t("recovery.attachHint")}</p>

        {settings ? (
          <>
            {/* Local backup paths (relative to the host mount). */}
            <FolderBrowser
              label={t("settings.containersPath")}
              value={settings.containersPath}
              hostMountRoot={hostMountRoot}
              onChange={(v) => setSettings((prev) => (prev ? { ...prev, containersPath: v } : prev))}
            />
            <FolderBrowser
              label={t("settings.vmsPath")}
              value={settings.vmsPath}
              hostMountRoot={hostMountRoot}
              onChange={(v) => setSettings((prev) => (prev ? { ...prev, vmsPath: v } : prev))}
            />
            <FolderBrowser
              label={t("settings.flashPath")}
              value={settings.flashPath}
              hostMountRoot={hostMountRoot}
              onChange={(v) => setSettings((prev) => (prev ? { ...prev, flashPath: v } : prev))}
            />

            {/* Off-site repo URLs (rest / S3 / B2 / sftp / rclone). */}
            <span className="text-xs font-medium text-carbon-textSub pt-1">{t("settings.offsiteTitle")}</span>
            {([
              ["containersOffsite", "nav.containers"],
              ["vmsOffsite", "nav.vms"],
              ["flashOffsite", "nav.flash"],
            ] as const).map(([key, label]) => (
              <div key={key} className="flex flex-col gap-1">
                <label className="text-xs text-carbon-textSub">{t(label)}</label>
                <input
                  value={settings[key]}
                  spellCheck={false}
                  onChange={(e) => setSettings((prev) => (prev ? { ...prev, [key]: e.target.value } : prev))}
                  placeholder="rest:http://host:8000/repo"
                  className={offsiteInput}
                />
              </div>
            ))}

            {/* Encryption on/off (reuses the Settings ToggleRow). */}
            <div className="pt-1">
              <ToggleRow
                label={
                  settings.encryptionEnabled
                    ? t("settings.encryptionOn")
                    : t("settings.encryptionOff")
                }
                checked={settings.encryptionEnabled}
                onChange={(v) => setSettings((prev) => (prev ? { ...prev, encryptionEnabled: v } : prev))}
              />
            </div>

            {/* Cloud + rclone credential cards — the exact Settings components,
                self-persisting via setCloud/setRclone (no duplicate persistence). */}
            <CloudCard t={t} />
            <RcloneCard t={t} />

            {/* Connect & preview — save paths/off-site/encryption, then re-check. */}
            <div className="flex items-center gap-3 pt-1">
              <button
                onClick={() => void connectPreview()}
                disabled={attachState === "saving"}
                className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
              >
                {attachState === "saving" && (
                  <span
                    className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
                    style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
                  />
                )}
                {t("recovery.connectPreview")}
              </button>
              {attachState === "saved" && previewed && readableState === "ok" && (
                <span className="text-sm text-[#6fdc8c]">{t("recovery.readable")}</span>
              )}
              {attachState === "error" && attachError && (
                <span className="text-sm text-[#ff8389]">{attachError}</span>
              )}
            </div>
          </>
        ) : (
          <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
        )}
      </StepCard>
    </div>
  );
}
