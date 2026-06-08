"use client";

import { createContext, useContext, useCallback, useEffect, useState } from "react";
import { type Theme, DEFAULT_THEME, THEMES, THEME_COOKIE, writeThemeCookie } from "@/lib/theme";

interface ThemeContextValue {
  theme: Theme;
  toggle: () => void;
}

const ThemeContext = createContext<ThemeContextValue>({
  theme: DEFAULT_THEME,
  toggle: () => undefined,
});

export function useTheme(): ThemeContextValue {
  return useContext(ThemeContext);
}

// Read the current data-theme attribute from <html>. On first visit (no cookie),
// the server set DEFAULT_THEME ("dark"). The prefers-color-scheme check and the
// cookie write happen inside initTheme, which is called once as a lazy useState
// initializer — before the first paint, no extra render needed.
function initTheme(serverTheme: Theme): Theme {
  if (typeof document === "undefined") return serverTheme;

  const hasCookie = document.cookie
    .split(";")
    .some((c) => c.trim().startsWith(`${THEME_COOKIE}=`));

  if (!hasCookie) {
    // First visit: respect OS preference, write cookie so SSR picks it up next time.
    const sys: Theme = window.matchMedia("(prefers-color-scheme: light)").matches
      ? "light"
      : "dark";
    document.documentElement.setAttribute("data-theme", sys);
    writeThemeCookie(sys);
    return sys;
  }

  // Cookie exists — trust what the server already rendered on <html>.
  return serverTheme;
}

export function ThemeProvider({
  initialTheme,
  children,
}: {
  initialTheme: Theme;
  children: React.ReactNode;
}) {
  // Lazy initializer: runs once synchronously before the first render.
  // Reads prefers-color-scheme on first visit and updates <html> immediately —
  // no extra render, no flash.
  const [theme, setTheme] = useState<Theme>(() => initTheme(initialTheme));

  // Add `theme-ready` class after first mount so the CSS transition for
  // background/color only fires on intentional theme switches, not on the
  // initial paint (avoids an animated flash from default → OS preference).
  useEffect(() => {
    document.documentElement.classList.add("theme-ready");
  }, []);

  const toggle = useCallback(() => {
    setTheme((prev) => {
      const next: Theme = prev === "dark" ? "light" : "dark";
      document.documentElement.setAttribute("data-theme", next);
      writeThemeCookie(next);
      return next;
    });
  }, []);

  return (
    <ThemeContext.Provider value={{ theme, toggle }}>
      {children}
    </ThemeContext.Provider>
  );
}
