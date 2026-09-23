import type { ResolvedTheme } from "../../lib/theme";
import { Card } from "../settings/shared";
import { HUE_OFFSET, Selector } from "../../components/Selector";
import { getResolvedTheme, getTheme, onSystemThemeChange, setTheme } from "../../lib/theme";
import { useEffect, useState } from "react";
import { useT } from "../../lib/i18n";

// ThemeCard picks the light or dark theme. There is no segment for following
// the system; lib/theme.ts has no UI path back to "system" yet.
export function ThemeCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  // The stored default is "system", so the picker shows the resolved theme,
  // the one actually painted.
  const [theme, setThemeState] = useState(getResolvedTheme);

  function selectTheme(next: ResolvedTheme) {
    setTheme(next);
    setThemeState(next);
  }

  // lib/theme.ts repaints the page when the OS flips under a "system"
  // preference; this keeps the selected segment in step with it.
  useEffect(() => {
    return onSystemThemeChange(() => {
      if (getTheme() === "system") setThemeState(getResolvedTheme());
    });
  }, []);

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

  return (
    <Card title={t("settings.theme")} hueIndex={hueIndex}>
      <Selector
        items={[
          { id: "light", label: t("theme.light"), icon: sunIcon },
          { id: "dark", label: t("theme.dark"), icon: moonIcon },
        ]}
        label={t("settings.theme")}
        // This card sits in the same tab as the shape, motion and label
        // selectors, so it takes its own start out of the shared table.
        hueOffset={HUE_OFFSET.theme}
        select="one"
        active={theme}
        onChange={(id) => selectTheme(id as ResolvedTheme)}
        size="lg"
        variant="well"
        // Pins every segment to the widest one, floored at MIN_PINNED_WIDTH,
        // like the other large pickers on this tab.
        equalWidth
      />
    </Card>
  );
}
