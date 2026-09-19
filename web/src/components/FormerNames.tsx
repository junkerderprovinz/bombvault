import { useState } from "react";
import type { useT } from "../lib/i18n";
import { useTakeOver, type TakeoverEntry } from "../lib/useTakeOver";
import { Button } from "./Button";
import { InfoBubble } from "./InfoBubble";

type T = ReturnType<typeof useT>["t"];

function UnlinkButton({
  old,
  unlink,
  disabled,
  t,
}: {
  old: string;
  unlink: (old: string, onRefused: () => void) => Promise<void>;
  disabled: boolean;
  t: T;
}) {
  const [shake, setShake] = useState(0);
  return (
    <Button
      key={shake}
      label={t("takeover.unlinkName").replace("{old}", old)}
      labelKey="takeover.unlink"
      tone="neutral"
      onClick={() => void unlink(old, () => setShake((n) => n + 1))}
      disabled={disabled}
      className={shake ? "glim-shake" : ""}
    />
  );
}

/** The names an entry had before a takeover, and any of them a live machine carries again. */
export function FormerNames({
  aliases,
  conflicts,
  entry,
  onDone,
  t,
}: {
  aliases: string[];
  conflicts: string[];
  entry: TakeoverEntry;
  onDone: () => void;
  t: T;
}) {
  const { unlink, busy, confirmDialog } = useTakeOver(entry, onDone, t);
  const conflictSet = new Set(conflicts);
  const unlinkable = aliases.filter((old) => !conflictSet.has(old));

  return (
    <>
      {conflicts.map((old) => (
        <div key={old} className="flex items-center gap-2 flex-wrap rounded-card bg-carbon-background px-3 py-2">
          <span className="flex min-w-0 items-center gap-1 text-xs text-carbon-textSub">
            {t("takeover.conflict").replace("{old}", old)}
            <InfoBubble tip={t("takeover.conflictHint")} />
          </span>
        </div>
      ))}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="flex items-center gap-1 text-xs text-carbon-textSub">
          {t("takeover.renamed")}
          <InfoBubble
            tip={`${t("takeover.formerly").replace("{names}", aliases.join(", "))} ${t("takeover.formerlyHint")}`}
          />
        </span>
        <div className="ms-auto flex items-center gap-1.5 flex-wrap">
          {unlinkable.map((old) => (
            <UnlinkButton key={old} old={old} unlink={unlink} disabled={busy} t={t} />
          ))}
        </div>
      </div>
      {confirmDialog}
    </>
  );
}
