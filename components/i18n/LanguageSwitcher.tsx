"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { LANGUAGES } from "@/lib/i18n/locales";
import { writeLanguageCookie } from "@/lib/i18n/detect";
import { Flag } from "./Flag";
import styles from "./LanguageSwitcher.module.css";

// Flag-button language picker matching featherdrop's UX: the button shows the
// current language's flag; clicking it opens a scrollable dropdown listing every
// language with its flag + native label. The current language is shown bold.
// Selection switches i18next live and persists the choice in a cookie.
export function LanguageSwitcher() {
  const { i18n, t } = useTranslation();
  const current = LANGUAGES.find((l) => l.code === i18n.language) ?? LANGUAGES[0];
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  const choose = useCallback(
    (code: string) => {
      void i18n.changeLanguage(code);
      writeLanguageCookie(code);
      setOpen(false);
    },
    [i18n],
  );

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  // Close on Escape
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [open]);

  return (
    <div ref={containerRef} className={styles.wrapper}>
      <button
        className={styles.trigger}
        onClick={() => setOpen((v) => !v)}
        aria-label={t("language.label")}
        aria-haspopup="listbox"
        aria-expanded={open}
        title={t("language.label")}
      >
        <Flag code={current.flag} size={20} />
      </button>

      {open && (
        <div className={styles.dropdown} role="listbox" aria-label={t("language.label")}>
          {LANGUAGES.map((lang) => (
            <button
              key={lang.code}
              role="option"
              aria-selected={lang.code === current.code}
              className={`${styles.item} ${lang.code === current.code ? styles.itemActive : ""}`}
              onClick={() => choose(lang.code)}
            >
              <Flag code={lang.flag} size={20} />
              <span>{lang.label}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
