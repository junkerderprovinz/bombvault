// What the tile of a page with its own place in the app is made of: the switch
// that puts the page in the sidebar, and the row that leads to it.
import type { ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Button } from "../../components/Button";
import { IconForward } from "../../components/glyphs";
import { useT, type TranslationKey } from "../../lib/i18n";
import { setNavPin, useNavPins, type NavPin } from "../../lib/navPins";
import { Card, ToggleRow } from "./shared";
import { useSettings } from "./settingsStore";

export function SidebarCard({ pin, hueIndex }: { pin: NavPin; hueIndex: number }) {
  const { t } = useT();
  const { settings } = useSettings();
  const pins = useNavPins(settings);
  return (
    <Card title={t("settings.labels.sidebar")} hueIndex={hueIndex}>
      <ToggleRow
        label={t("tile.sidebar.show")}
        hint={t("tile.sidebar.showHint")}
        checked={pins[pin]}
        onChange={(on) => setNavPin(pin, on)}
      />
    </Card>
  );
}

/** PageLinkCard names a page, says in a line what stands there and opens it.
 *  The search finds it under the page's name. */
export function PageLinkCard({
  to,
  label,
  sub,
  glyph,
  hueIndex,
}: {
  to: string;
  label: string;
  sub: string;
  glyph: ReactNode;
  hueIndex: number;
}) {
  const { t } = useT();
  const navigate = useNavigate();
  return (
    <Card hueIndex={hueIndex}>
      <div data-search-card={label} className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <span className="flex h-(--btn-h) w-(--btn-h) shrink-0 items-center justify-center rounded-control bg-carbon-surface2 text-accentText">
          {glyph}
        </span>
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="text-sm font-semibold text-carbon-text">{label}</span>
          <span className="text-xs text-carbon-textMuted wrap-break-word">{sub}</span>
        </div>
        <Button
          label={t("instances.open")}
          labelKey="instances.open"
          glyph={<IconForward />}
          tone="accent"
          onClick={() => navigate(to)}
          className="ms-auto"
        />
      </div>
    </Card>
  );
}

/**
 * PageSentence is a sentence that names a settings page as a link. The
 * translation marks the place with {page}, so each language puts the name
 * where its grammar wants it.
 */
export function PageSentence({ text, page, to }: { text: TranslationKey; page: TranslationKey; to: string }) {
  const { t } = useT();
  const [before, after = ""] = t(text).split("{page}");
  return (
    <p className="text-xs text-carbon-textMuted">
      {before}
      <Link to={to} className="text-accentText underline-offset-2 hover:underline">
        {t(page)}
      </Link>
      {after}
    </p>
  );
}
