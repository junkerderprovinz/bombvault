"use client";

import { ThemeToggle } from "./theme/ThemeToggle";
import { LanguageSwitcher } from "./i18n/LanguageSwitcher";
import styles from "./ControlsBar.module.css";

// Thin top bar showing the theme toggle and language switcher.
// Rendered on every page via app/layout.tsx.
export function ControlsBar() {
  return (
    <div className={styles.bar}>
      <span className={styles.brand}>BombVault</span>
      <div className={styles.controls}>
        <LanguageSwitcher />
        <ThemeToggle />
      </div>
    </div>
  );
}
