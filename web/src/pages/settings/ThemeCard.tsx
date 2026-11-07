import type { Theme } from "../../lib/theme";
import { Selector } from "../../components/Selector";
import { IconMoon, IconSun, IconThemeSystem } from "../../components/glyphs";
import { darkShowing, getOled, getTheme, oledRefusal, setOled, setTheme, watchDarkShowing, watchTheme } from "../../lib/theme";
import { useState, useSyncExternalStore } from "react";
import { useT } from "../../lib/i18n";
import { ToggleRow } from "./shared";

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

/** OledRow puts the dark theme on true black. The choice belongs to this
 *  screen, so it never goes to the server. It rests while the light theme
 *  shows without letting go, and refuses while Light is chosen. */
export function OledRow({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [on, setOn] = useState(getOled);
  const dark = useSyncExternalStore(watchDarkShowing, darkShowing);
  const theme = useSyncExternalStore(watchTheme, getTheme);
  return (
    <ToggleRow
      label={t("settings.oled")}
      hint={t("settings.oledHint")}
      checked={on}
      onChange={(next) => {
        setOled(next);
        setOn(next);
      }}
      resting={!dark}
      refused={oledRefusal(theme, {
        sentence: t("settings.oledLight"),
        dark: t("theme.dark"),
        system: t("theme.system"),
        light: t("theme.light"),
      })}
      newId="look-oled"
    />
  );
}
