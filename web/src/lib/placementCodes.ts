import type { OkEnvelope, RefusalTarget, SaveWarning, TargetUse } from "./api";
import type { TranslationKey, useT } from "./i18n";
import type { ToastSeverity } from "./toastEngine";

type T = ReturnType<typeof useT>["t"];

/** A refusal from the placement routes, with the fields some codes carry. */
export type PlacementRefusal = OkEnvelope & { defaultDomains?: string[]; use?: TargetUse; target?: RefusalTarget };

const CODE_KEYS: Record<string, TranslationKey> = {
  "placement-unreadable": "placementCode.unreadable",
  "invalid-placement": "placementCode.invalid",
  "copies-not-allowed": "placementCode.copiesNotAllowed",
  "unknown-target": "placementCode.unknownTarget",
  "stack-rule": "placementCode.stackRule",
  "copy-rule-taken": "placementCode.copyRuleTaken",
  "domain-busy": "placementCode.domainBusy",
  "has-backups": "placementCode.hasBackups",
  stale: "placementCode.stale",
  "repo-invalid": "placementCode.repoInvalid",
  "default-repo-missing": "placementCode.defaultRepoMissing",
  "nested-location": "placementCode.nestedLocation",
  "foreign-domain": "placementCode.foreignDomain",
  "mirrored-field": "placementCode.mirroredField",
  "companion-taken": "placementCode.companionTaken",
  "exclusion-unsaved": "placementCode.exclusionUnsaved",
  "append-only": "placementCode.appendOnly",
  "removal-grown": "placementCode.removalGrown",
  "name-mismatch": "placementCode.nameMismatch",
  "home-unreadable": "placementCode.homeUnreadable",
  "snapshot-missing": "placementCode.snapshotMissing",
};

const WARNING_KEYS: Record<SaveWarning["code"], TranslationKey> = {
  "direct-retention-lowered": "saveWarning.directRetentionLowered",
  "direct-append-only-off": "saveWarning.directAppendOnlyOff",
  "direct-creds-kept": "saveWarning.directCredsKept",
};

const DOMAIN_KEYS: Record<string, TranslationKey> = {
  containers: "nav.containers",
  vms: "nav.vms",
  files: "nav.files",
};

function domainNames(t: T, lang: string, domains: string[]): string {
  const names = domains.map((d) => (DOMAIN_KEYS[d] ? t(DOMAIN_KEYS[d]) : d));
  return new Intl.ListFormat(lang, { type: "conjunction" }).format(names);
}

/** placementErrorText is the sentence a refusal shows: the translated text for
 *  its code, the server's own error for a code this table does not know, and
 *  the fallback when there is neither. */
export function placementErrorText(t: T, lang: string, res: PlacementRefusal, fallback: TranslationKey): string {
  if (res.code === "repo-in-use") {
    const domains = res.defaultDomains ?? [];
    return domains.length > 0
      ? t("placementCode.repoInUseDefault").replace("{domains}", domainNames(t, lang, domains))
      : t("repos.deleteBlocked");
  }
  if (res.code === "target-in-use" && res.use) {
    return res.use.items > 0
      ? t("placementCode.targetInUseItems").replace("{n}", String(res.use.items))
      : t("placementCode.targetInUseDefault").replace("{domains}", domainNames(t, lang, res.use.defaultDomains));
  }
  if (res.code === "direct-repo" && res.target) {
    return t("placementCode.directRepo").replace("{target}", res.target.name);
  }
  const key = res.code ? CODE_KEYS[res.code] : undefined;
  if (key) return t(key);
  return res.error ?? t(fallback);
}

/** saveWarningText is the message for one warning a save answered with. */
export function saveWarningText(t: T, w: SaveWarning): string {
  return t(WARNING_KEYS[w.code]).replace("{target}", w.targetName).replace("{n}", String(w.items));
}

/** pushSaveWarnings shows every warning a save answered with. */
export function pushSaveWarnings(
  push: (message: string, severity?: ToastSeverity) => void,
  t: T,
  warnings: SaveWarning[] | undefined
): void {
  for (const w of warnings ?? []) push(saveWarningText(t, w), "warn");
}
