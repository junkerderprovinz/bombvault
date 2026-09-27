import type { ContainerChange } from "../lib/api";
import type { TranslationKey, useT } from "../lib/i18n";
import { Badge } from "./Badge";
import { InfoBubble } from "./InfoBubble";

type T = ReturnType<typeof useT>["t"];

// The bubble names this many changes and counts the rest.
const LISTED = 6;

// The server words a change from the restore's side: added is only in the
// backup, so seen from now it is gone.
const KEYS: Record<Exclude<ContainerChange["field"], "image">, Partial<Record<ContainerChange["change"], TranslationKey>>> = {
  port: { added: "changeNotice.portGone", removed: "changeNotice.portNew" },
  env: { added: "changeNotice.envGone", removed: "changeNotice.envNew", changed: "changeNotice.envChanged" },
  volume: { added: "changeNotice.volumeGone", removed: "changeNotice.volumeNew" },
};

function changeText(c: ContainerChange, t: T): string {
  if (c.field === "image") {
    return c.change === "updated"
      ? t("changeNotice.imageUpdated").replace("{now}", c.now ?? "")
      : t("changeNotice.image").replace("{now}", c.now ?? "").replace("{backup}", c.backup ?? "");
  }
  const key = KEYS[c.field]?.[c.change];
  return key ? t(key).replace("{name}", c.name || c.backup || c.now || "") : "";
}

/**
 * ContainerChangeNotice is the mark beside a container's name when it was
 * recreated with other settings since its last good backup. It only informs:
 * nothing is blocked, and it goes away with the next backup.
 */
export function ContainerChangeNotice({ changes, t }: { changes?: ContainerChange[]; t: T }) {
  const lines = (changes ?? []).map((c) => changeText(c, t)).filter(Boolean);
  if (lines.length === 0) return null;
  const listed = lines.slice(0, LISTED);
  if (lines.length > LISTED) listed.push(t("changeNotice.more", lines.length - LISTED));
  return (
    <span className="inline-flex items-center gap-1">
      <Badge tone="warn" size="small" shape="pill" className="whitespace-nowrap">
        {t("changeNotice.badge")}
      </Badge>
      <InfoBubble tip={t("changeNotice.hint").replace("{changes}", listed.join("; "))} />
    </span>
  );
}
