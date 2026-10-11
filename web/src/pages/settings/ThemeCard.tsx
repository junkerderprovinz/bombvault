import type { Theme } from "../../lib/theme";
import { Selector } from "../../components/Selector";
import { getTheme, setTheme } from "../../lib/theme";
import { useState } from "react";
import { useT } from "../../lib/i18n";

/** ThemePicker chooses the dark or the light theme, or leaves it to the
 *  device, which is also the way back from a choice once made. */
export function ThemePicker({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [theme, setThemeState] = useState(getTheme);

  function selectTheme(next: Theme) {
    setTheme(next);
    setThemeState(next);
  }

  // Both glyphs paint in currentColor like every other glyph, so a selected
  // segment flips them to the contrast ink. The sun's rays are thin filled
  // rects rather than strokes (design-language.md, "Icon glyphs").
  const sunIcon = (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor" className="shrink-0" aria-hidden="true">
      <circle cx="10" cy="10" r="4.25" />
      <rect x="9.15" y="2" width="1.7" height="2" rx="0.85" />
      <rect x="9.15" y="16" width="1.7" height="2" rx="0.85" />
      <rect x="2" y="9.15" width="2" height="1.7" rx="0.85" />
      <rect x="16" y="9.15" width="2" height="1.7" rx="0.85" />
      <rect x="4.435" y="4.785" width="2.4" height="1.7" rx="0.85" transform="rotate(45 5.635 5.635)" />
      <rect x="13.165" y="13.515" width="2.4" height="1.7" rx="0.85" transform="rotate(45 14.365 14.365)" />
      <rect x="4.435" y="13.515" width="2.4" height="1.7" rx="0.85" transform="rotate(-45 5.635 14.365)" />
      <rect x="13.165" y="4.785" width="2.4" height="1.7" rx="0.85" transform="rotate(-45 14.365 5.635)" />
    </svg>
  );
  const moonIcon = (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" className="shrink-0" aria-hidden="true">
      <path
        d="M17.5 12.5A7.5 7.5 0 017.5 2.5a7.5 7.5 0 100 15 7.5 7.5 0 0010-5z"
        fill="currentColor"
      />
    </svg>
  );

  const systemIcon = (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor" className="shrink-0" aria-hidden="true">
      <circle cx="10" cy="10" r="7.25" fill="none" stroke="currentColor" strokeWidth="1.5" />
      <path d="M10 3.5a6.5 6.5 0 0 1 0 13Z" />
    </svg>
  );

  return (
    <Selector
      items={[
        { id: "dark", label: t("theme.dark"), icon: moonIcon },
        { id: "light", label: t("theme.light"), icon: sunIcon },
        { id: "system", label: t("theme.system"), tip: t("theme.systemHint"), icon: systemIcon },
      ]}
      label={t("settings.theme")}
      select="one"
      active={theme}
      onChange={(id) => selectTheme(id as Theme)}
      inline
    />
  );
}
