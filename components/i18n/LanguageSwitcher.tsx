"use client";

import { useTranslation } from "react-i18next";
import { LANGUAGES } from "@/lib/i18n/locales";
import { writeLanguageCookie } from "@/lib/i18n/detect";
import styles from "./LanguageSwitcher.module.css";

// A simple select-based language picker. Choosing a language switches i18next
// live and persists the choice in a cookie so the server can pick it up on the
// next navigation.
export function LanguageSwitcher() {
  const { i18n, t } = useTranslation();
  const current = i18n.language;

  const choose = (code: string) => {
    void i18n.changeLanguage(code);
    writeLanguageCookie(code);
  };

  return (
    <label className={styles.wrapper} aria-label={t("language.label")}>
      <span className={styles.label}>{t("language.label")}</span>
      <select
        className={styles.select}
        value={current}
        onChange={(e) => choose(e.target.value)}
      >
        {LANGUAGES.map((lang) => (
          <option key={lang.code} value={lang.code}>
            {lang.label}
          </option>
        ))}
      </select>
    </label>
  );
}
