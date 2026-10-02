import type { ReactNode } from "react";
import { Badge } from "../components/Badge";
import { IconBack } from "../components/glyphs";
import type { useT } from "../lib/i18n";

type T = ReturnType<typeof useT>["t"];

export function PageTop({ t, title, onBack, children }: { t: T; title: string; onBack: () => void; children?: ReactNode }) {
  return (
    <header className="flex items-center gap-3">
      <Badge as="button" shape="square" size="icon" tone="neutral" tip={t("common.back")} onClick={onBack}>
        <IconBack />
      </Badge>
      <h1 className="min-w-0 truncate text-xl font-semibold text-carbon-text">{title}</h1>
      {children}
    </header>
  );
}
