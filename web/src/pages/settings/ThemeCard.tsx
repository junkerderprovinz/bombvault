import type { Theme } from "../../lib/theme";
import { Selector } from "../../components/Selector";
import { IconMoon, IconSun, IconThemeSystem } from "../../components/glyphs";
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

  return (
    <Selector
      items={[
        { id: "dark", label: t("theme.dark"), icon: <IconMoon /> },
        { id: "light", label: t("theme.light"), icon: <IconSun /> },
        { id: "system", label: t("theme.system"), tip: t("theme.systemHint"), icon: <IconThemeSystem /> },
      ]}
      label={t("settings.theme")}
      select="one"
      active={theme}
      onChange={(id) => selectTheme(id as Theme)}
      inline
    />
  );
}
