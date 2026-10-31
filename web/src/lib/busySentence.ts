import type { OffsiteDomain } from "./api";
import type { TranslationKey } from "./i18n";
import { DOMAIN_LABEL } from "./sectionLabels";

/** What holds a section, as a refusal names it. */
export type BusyKind = "backup" | "copy" | "restore" | "prune" | "check" | "other";

const BUSY_KEYS: Record<BusyKind, TranslationKey> = {
  backup: "busy.backup",
  copy: "busy.copy",
  restore: "busy.restore",
  prune: "busy.prune",
  check: "busy.check",
  other: "busy.other",
};

/** busySentence names the section a refusal was about and what holds it, or
 *  undefined when the refusal does not say. */
export function busySentence(
  t: (key: TranslationKey) => string,
  busy: { domain?: string; busyWith?: string }
): string | undefined {
  const domain = busy.domain as OffsiteDomain | undefined;
  const key = BUSY_KEYS[busy.busyWith as BusyKind];
  if (!domain || !(domain in DOMAIN_LABEL) || !key) return undefined;
  const section = t(DOMAIN_LABEL[domain]);
  return t(key).replace("{section}", () => section);
}
