import { useT } from "../lib/i18n";
import { excludesEverything } from "../lib/excludeLines";

// ExcludesEverythingNote warns under an exclude field without a preview of its
// own about each line the backup ignores because it would exclude everything.
export function ExcludesEverythingNote({ text }: { text: string }) {
  const { t } = useT();
  const lines = text.split("\n").filter(excludesEverything);
  if (lines.length === 0) return null;
  return (
    <div className="flex flex-col gap-1">
      {lines.map((line, i) => (
        <p key={i} className="text-xs wrap-break-word leading-snug flex items-baseline gap-1.5">
          <span dir="ltr" className="font-mono text-carbon-textSub text-start">{line.trim()}</span>
          <span className="text-statusFail">⚠ {t("excludes.excludesEverything")}</span>
        </p>
      ))}
    </div>
  );
}
