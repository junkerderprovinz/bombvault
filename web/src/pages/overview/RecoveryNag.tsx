import { useEffect, useState } from "react";
import { ackRecoveryKit, downloadRecoveryKit, getSettings } from "../../lib/api";
import type { Settings } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { Button } from "../../components/Button";

// RecoveryNag asks the user to download and store the encryption recovery kit,
// so a restore works without a running BombVault. It shows only while
// encryption is on and the kit has not been acknowledged.
export function RecoveryNag({ t, suppressed }: { t: ReturnType<typeof useT>["t"]; suppressed?: boolean }) {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [dismissing, setDismissing] = useState(false);
  // Backend refusal text from the fetch-based kit download (null = no error).
  const [kitError, setKitError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    getSettings()
      .then((res) => {
        if (active && res.ok) setSettings(res.settings);
      })
      .catch(() => {/* non-fatal */});
    return () => {
      active = false;
    };
  }, []);

  if (suppressed) return null;
  if (!settings || !settings.encryptionEnabled || settings.recoveryKitAck) {
    return null;
  }

  const dismiss = () => {
    setDismissing(true);
    void ackRecoveryKit()
      .then((res) => {
        if (res.ok) setSettings({ ...settings, recoveryKitAck: true });
      })
      .catch(() => {/* non-fatal */})
      .finally(() => setDismissing(false));
  };

  return (
    <div className="rounded-card bg-statusWarnBg px-4 py-3 flex flex-col gap-2">
      {/* A plain heading, not a heading badge: the panel is already a filled
          status wash, and a badge fill on it measures between 1.00:1 and
          1.39:1, so it would read as text with extra padding and lose the
          text-statusWarn colour that carries the alert (8.62:1 against the
          panel). Badge.tsx's file header has the full reasoning. */}
      <h2 className="text-sm font-semibold text-statusWarn">
        {t("recovery.nagTitle")}
      </h2>
      <p className="text-xs text-statusWarn leading-relaxed">
        {t("recovery.nagBody")}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        {/* Downloaded through fetch, as in Settings: the backend fails closed
            for this export while auth is off, and a raw <a download> would
            save the 403 refusal body as the .md file. This way the message
            shows instead. */}
        <button
          type="button"
          onClick={() => void downloadRecoveryKit().then(setKitError)}
          className="rounded-pill bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors"
        >
          {t("recovery.download")}
        </button>
        {kitError && (
          <span className="text-xs text-statusFail wrap-break-word">✗ {kitError}</span>
        )}
        <Button
          label={t("recovery.stored")}
          labelKey="recovery.stored"
          tone="neutral"
          onClick={dismiss}
          disabled={dismissing}
        />
      </div>
    </div>
  );
}
