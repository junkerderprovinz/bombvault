"use client";

import { useTranslation } from "react-i18next";
import { useTheme } from "./ThemeProvider";
import styles from "./ThemeToggle.module.css";

export function ThemeToggle() {
  const { theme, toggle } = useTheme();
  const { t } = useTranslation();

  return (
    <button
      className={styles.button}
      onClick={toggle}
      aria-label={t("theme.toggle")}
      title={t("theme.toggle")}
    >
      {theme === "dark" ? "☀" : "☾"}
    </button>
  );
}
