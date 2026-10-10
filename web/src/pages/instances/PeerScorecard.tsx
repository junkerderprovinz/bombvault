import type { ReactNode } from "react";

import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconConfig, IconContainers, IconFiles, IconFlash, IconVM, IconZFS } from "../../components/navGlyphs";
import type { DomainStatus } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { relativeTime } from "../../lib/reltime";

export const DOMAIN_LABEL_KEYS: Record<string, TranslationKey> = {
  containers: "settings.containersEnabled",
  vms: "settings.vmsEnabled",
  flash: "settings.flashEnabled",
  files: "settings.filesEnabled",
  zfs: "settings.zfsEnabled",
  config: "settings.configEnabled",
};

export function domainLabelKey(domain: string): TranslationKey {
  return DOMAIN_LABEL_KEYS[domain] ?? "settings.containersEnabled";
}

const DOMAIN_GLYPH: Record<string, ReactNode> = {
  containers: <IconContainers />,
  vms: <IconVM />,
  flash: <IconFlash />,
  files: <IconFiles />,
  zfs: <IconZFS />,
  config: <IconConfig />,
};

// Same mapping as Dashboard's protectionChip, which is not exported.
function protectionTone(level: string): "ok" | "fail" | "warn" | "neutral" {
  switch (level) {
    case "green":
      return "ok";
    case "amber":
      return "warn";
    case "red":
      return "fail";
    default:
      return "neutral";
  }
}

function protectionLabelKey(level: string): TranslationKey {
  switch (level) {
    case "green":
      return "fleet.protection.green";
    case "amber":
      return "fleet.protection.amber";
    case "red":
      return "fleet.protection.red";
    default:
      return "fleet.protection.none";
  }
}

/** PeerScorecard is an instance's protection, one row per section it backs
 *  up. With `onCheck` each row can ask the instance to check that section's
 *  repository now. */
export function PeerScorecard({
  domains,
  onCheck,
}: {
  domains: DomainStatus[];
  onCheck?: (domain: string) => void;
}) {
  const { t } = useT();
  const shown = domains.filter((d) => d.enabled && d.protection !== "");
  if (shown.length === 0) {
    return <p className="text-xs text-carbon-textMuted">{t("fleet.noScorecard")}</p>;
  }
  return (
    <ul className="flex flex-col gap-3">
      {shown.map((d) => (
        <li key={d.domain} data-domain={d.domain} className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <span className="grid h-9 w-9 shrink-0 place-items-center rounded-control bg-carbon-surface3 text-accentText">
            {DOMAIN_GLYPH[d.domain]}
          </span>
          <span className="flex min-w-0 flex-1 flex-col">
            <span className="text-sm font-semibold text-carbon-text">{t(domainLabelKey(d.domain))}</span>
            {d.lastSuccess > 0 && (
              <span className="text-xs text-carbon-textMuted">
                {t("fleet.lastBackup").replace("{time}", relativeTime(t, d.lastSuccess))}
              </span>
            )}
          </span>
          <span className="ms-auto flex flex-wrap items-center justify-end gap-2">
            <Badge tone={protectionTone(d.protection)}>{t(protectionLabelKey(d.protection))}</Badge>
            {onCheck && d.domain !== "config" && (
              <Button label={t("fleet.checkNow")} labelKey="fleet.checkNow" tone="neutral" onClick={() => onCheck(d.domain)} />
            )}
          </span>
        </li>
      ))}
    </ul>
  );
}
