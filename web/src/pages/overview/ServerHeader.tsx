import type { ReactNode } from "react";

import { Button } from "../../components/Button";
import { IconBack } from "../../components/glyphs";
import { goHome } from "../../lib/appServers";
import type { useT } from "../../lib/i18n";

type T = ReturnType<typeof useT>["t"];

/**
 * ServerHeader names the server on screen at the top of the overview inside
 * the Android app, behind an arrow that takes the app back to its own
 * overview, where all of its servers are listed.
 */
export function ServerHeader({ t, name, logo }: { t: T; name: string; logo: ReactNode }) {
  return (
    <div className="flex min-w-0 items-center gap-2 md:hidden">
      <span data-new="overview-instances" className="flex shrink-0">
        <Button
          label={t("overview.backToApp")}
          labelKey={null}
          glyph={<IconBack />}
          tone="neutral"
          variant="icon"
          glyphOnly
          onClick={goHome}
          className="glim-btn-key flex-none rounded-control"
        />
      </span>
      {logo}
      <span className="min-w-0 truncate text-lg font-semibold tracking-tight text-carbon-text">{name}</span>
    </div>
  );
}
