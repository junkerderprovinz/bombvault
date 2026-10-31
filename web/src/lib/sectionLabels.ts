import type { OffsiteDomain } from "./api";
import type { TranslationKey } from "./i18n";

export const DOMAIN_LABEL: Record<OffsiteDomain, TranslationKey> = {
  containers: "nav.containers",
  vms: "nav.vms",
  flash: "nav.flash",
  files: "nav.files",
  zfs: "nav.zfs",
  config: "nav.config",
};
