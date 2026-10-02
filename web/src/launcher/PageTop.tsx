import type { ReactNode } from "react";
import { Badge } from "../components/Badge";
import type { useT } from "../lib/i18n";

type T = ReturnType<typeof useT>["t"];

/** The back glyph of the app's pages, a filled triangle. */
export function BackGlyph() {
  return (
    <svg aria-hidden width="12" height="12" viewBox="0 0 12 12" fill="none">
      <path fill="currentColor" d="M9 1.3 2.5 6 9 10.7Z" />
    </svg>
  );
}

export function PageTop({ t, title, onBack, children }: { t: T; title: string; onBack: () => void; children?: ReactNode }) {
  return (
    <header className="flex items-center gap-3">
      <Badge as="button" shape="square" size="icon" tone="neutral" tip={t("common.back")} onClick={onBack}>
        <BackGlyph />
      </Badge>
      <h1 className="min-w-0 truncate text-xl font-semibold text-carbon-text">{title}</h1>
      {children}
    </header>
  );
}
