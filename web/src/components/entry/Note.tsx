import type { ReactNode } from "react";
import { InfoBubble } from "../InfoBubble";

export type NoteTone = "neutral" | "warn" | "fail";

const TONE: Record<NoteTone, string> = {
  neutral: "bg-carbon-surface2 text-carbon-textSub",
  warn: "bg-statusWarnBgSoft text-statusWarn",
  fail: "bg-statusFailBgSoft text-statusFail",
};

/** Note is a line an entry has to say above its cards, with the action that
 *  settles it at its end. */
export function Note({
  tone = "neutral",
  hint,
  action,
  children,
}: {
  tone?: NoteTone;
  /** What to do about it, in an (i) bubble beside the text. */
  hint?: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className={`flex flex-wrap items-center gap-x-3 gap-y-2 rounded-control px-3 py-2 text-sm ${TONE[tone]}`}>
      <span className="flex min-w-0 flex-1 items-center gap-1.5 wrap-break-word">
        <span className="min-w-0">{children}</span>
        {hint && <InfoBubble tip={hint} />}
      </span>
      {action}
    </div>
  );
}
