import { useEffect, useRef, useState } from "react";
import type { TranslationKey, useT } from "../lib/i18n";
import { backupCountText, countBackups, useTakeOver, type TakeoverEntry } from "../lib/useTakeOver";
import { Button } from "./Button";
import { InfoBubble } from "./InfoBubble";

type T = ReturnType<typeof useT>["t"];

const REASONS: Record<string, TranslationKey> = {
  "docker-id": "takeover.reason.dockerId",
  "appdata-bind": "takeover.reason.appdataBind",
  "compose-service": "takeover.reason.composeService",
  "template-lineage": "takeover.reason.templateLineage",
  "libvirt-uuid": "takeover.reason.libvirtUuid",
};

// Pairs of [old name, new name] the user said are not a rename.
const DISMISSED_KEY = "bombvault.renameSuggestionsDismissed";

function isDismissed(from: string, name: string): boolean {
  try {
    const pairs: [string, string][] = JSON.parse(localStorage.getItem(DISMISSED_KEY) ?? "[]");
    return pairs.some(([o, n]) => o === from && n === name);
  } catch {
    return false;
  }
}

function rememberDismissed(from: string, name: string) {
  try {
    const pairs: [string, string][] = JSON.parse(localStorage.getItem(DISMISSED_KEY) ?? "[]");
    localStorage.setItem(DISMISSED_KEY, JSON.stringify([...pairs, [from, name]]));
  } catch {
    // Without storage the row stays hidden until the page is loaded again.
  }
}

const FOCUSABLE_SELECTOR =
  'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

/** The first focusable control on the card after row, so dismissing it hands
 *  focus onward instead of losing it to <body> once the row unmounts. */
function nextCardFocus(row: HTMLElement): HTMLElement | null {
  const card = row.parentElement;
  if (!card) return null;
  const rest = Array.from(card.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter((el) => !row.contains(el));
  const after = rest.find((el) => (row.compareDocumentPosition(el) & Node.DOCUMENT_POSITION_FOLLOWING) !== 0);
  return after ?? rest[rest.length - 1] ?? null;
}

export function RenameTakeoverRow({
  from,
  reason,
  entry,
  onDone,
  t,
}: {
  /** The not-installed entry this one looks like, by the name its routes take. */
  from: string;
  reason: string;
  entry: TakeoverEntry;
  onDone: () => void;
  t: T;
}) {
  // Read once, which holds because the callers key this row by `from`.
  const [dismissed, setDismissed] = useState(() => isDismissed(from, entry.name));
  const [backups, setBackups] = useState<number | null>(null);
  const [shake, setShake] = useState(0);
  const { takeOver, busy, confirmDialog } = useTakeOver(entry, onDone, t);
  const rowRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (dismissed) return;
    let alive = true;
    void countBackups(entry.api, from).then((n) => {
      if (alive) setBackups(n);
    });
    return () => {
      alive = false;
    };
  }, [entry.api, from, dismissed]);

  if (dismissed) return null;

  const facts = [REASONS[reason] && t(REASONS[reason]), backups !== null && backupCountText(t, backups)]
    .filter(Boolean)
    .join(", ");

  return (
    <div ref={rowRef} className="flex items-center gap-2 flex-wrap rounded-card bg-carbon-background px-3 py-2">
      <span className="flex min-w-0 items-center gap-1 text-xs text-carbon-textSub">
        {t("takeover.looksLike").replace("{old}", from)}
        <InfoBubble tip={t("takeover.suggestHint")} />
      </span>
      {facts && <span className="text-xs text-carbon-textMuted">{facts}</span>}
      <div className="ms-auto flex items-center gap-1.5">
        <Button
          label={t("takeover.decline")}
          labelKey="takeover.decline"
          tone="neutral"
          onClick={() => {
            const next = rowRef.current && nextCardFocus(rowRef.current);
            rememberDismissed(from, entry.name);
            setDismissed(true);
            next?.focus();
          }}
        />
        <Button
          key={shake}
          label={t("takeover.accept")}
          labelKey="takeover.accept"
          tone="accent"
          onClick={() => void takeOver(from, backups, () => setShake((n) => n + 1))}
          disabled={busy}
          busy={busy}
          className={shake ? "glim-shake" : ""}
        />
      </div>
      {confirmDialog}
    </div>
  );
}
