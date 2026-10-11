import type { EncryptionDetection, EncryptionVerdict, RepoEncryption } from "../../lib/api";
import type { TranslationKey } from "../../lib/i18n";
import { ToggleRow } from "../../pages/settings/shared";
import { InfoBubble } from "../InfoBubble";

const VERDICT_TONE: Record<EncryptionVerdict, string> = {
  encrypted: "text-statusOk",
  plain: "text-statusOk",
  absent: "text-carbon-textMuted",
  unconfigured: "text-carbon-textMuted",
  unknown: "text-statusWarn",
  conflict: "text-statusFail",
};

const VERDICT_MESSAGE: Record<EncryptionVerdict, TranslationKey> = {
  encrypted: "recovery.encEncrypted",
  plain: "recovery.encPlain",
  absent: "recovery.encAbsent",
  unconfigured: "recovery.encUnconfigured",
  unknown: "recovery.encUnknown",
  conflict: "recovery.encConflict",
};

/** Verdicts that leave the mode open, so the user decides or overrides. */
const NEEDS_CONTROL: ReadonlySet<EncryptionVerdict> = new Set<EncryptionVerdict>(["absent", "unconfigured", "unknown", "conflict"]);

/** Verdicts that only help with the repositories named: which ones disagree,
 *  or which one failed and why. */
const SHOWS_REPOS: ReadonlySet<EncryptionVerdict> = new Set<EncryptionVerdict>(["unknown", "conflict"]);

const STATE_KEY: Record<RepoEncryption["state"], TranslationKey> = {
  encrypted: "recovery.encStateEncrypted",
  plain: "recovery.encStatePlain",
  absent: "recovery.encStateAbsent",
  unreachable: "recovery.encStateUnreachable",
};

const DOMAIN_KEY: Record<string, TranslationKey> = {
  containers: "nav.containers",
  vms: "nav.vms",
  flash: "nav.flash",
  files: "nav.files",
  config: "nav.config",
};

/**
 * EncryptionStatus says how the attached repositories are encrypted. That is
 * a fact about a repository, fixed when it is created, so the server probes
 * them and the setting follows what it finds. The switch shows only where
 * there is nothing to detect, or as an override when the probe could not
 * tell or the repositories disagree.
 */
export function EncryptionStatus({
  t,
  detection,
  detecting,
  encryptionEnabled,
  onOverride,
}: {
  t: (k: TranslationKey) => string;
  detection: EncryptionDetection | null;
  detecting: boolean;
  encryptionEnabled: boolean;
  onOverride: (v: boolean) => void;
}) {
  // On the first run a control would flash that the probe is about to make
  // unnecessary.
  if (detecting && !detection) {
    return (
      <div className="flex items-center gap-2">
        <span
          className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
          style={{ borderColor: "var(--accent)", borderTopColor: "transparent" }}
        />
        <span className="text-sm text-carbon-textMuted">{t("recovery.encChecking")}</span>
      </div>
    );
  }

  // A probe that failed on the way says nothing and counts as unknown.
  const verdict: EncryptionVerdict = detection?.ok ? (detection.verdict ?? "unknown") : "unknown";
  const repos = detection?.repos ?? [];

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-start gap-1.5">
        <p className={`text-sm leading-relaxed ${VERDICT_TONE[verdict]}`}>{t(VERDICT_MESSAGE[verdict])}</p>
        <InfoBubble tip={t("recovery.encDetectHint")} />
      </div>

      {SHOWS_REPOS.has(verdict) && repos.length > 0 && (
        <ul className="flex flex-col gap-1">
          {repos.map((r, i) => (
            <li key={`${r.domain}-${r.source}-${r.name ?? ""}-${i}`} className="text-xs leading-relaxed text-carbon-textMuted">
              <span className="text-carbon-textSub">
                {t(DOMAIN_KEY[r.domain] ?? "nav.containers")}
                {" · "}
                {t(r.source === "offsite" ? "recovery.encSourceOffsite" : "recovery.encSourceLocal")}
                {r.name ? ` (${r.name})` : ""}
              </span>
              {": "}
              {t(STATE_KEY[r.state])}
              {r.error && (
                <span dir="ltr" className="break-all font-mono">
                  : {r.error}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}

      {NEEDS_CONTROL.has(verdict) && (
        <ToggleRow
          label={t("settings.encryptionLabel")}
          hint={encryptionEnabled ? t("settings.encryptionOn") : t("settings.encryptionOff")}
          checked={encryptionEnabled}
          onChange={onOverride}
        />
      )}
    </div>
  );
}
