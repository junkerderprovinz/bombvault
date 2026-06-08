"use client";

import { useEffect } from "react";
import { I18nextProvider } from "react-i18next";
import { initI18n } from "@/lib/i18n/config";
import { isRtl } from "@/lib/i18n/locales";

// Wraps the app in the i18next context. The initial language is resolved on the
// server and passed in, so i18next starts in that language and the first client
// render matches the server HTML (no hydration mismatch, content renders without
// JS). On later switches it keeps <html lang> and <html dir> in sync.
export function I18nProvider({
  initialLanguage,
  children,
}: {
  initialLanguage: string;
  children: React.ReactNode;
}) {
  const i18n = initI18n(initialLanguage);

  useEffect(() => {
    const apply = (code: string) => {
      document.documentElement.lang = code;
      document.documentElement.dir = isRtl(code) ? "rtl" : "ltr";
    };
    i18n.on("languageChanged", apply);
    return () => {
      i18n.off("languageChanged", apply);
    };
  }, [i18n]);

  return <I18nextProvider i18n={i18n}>{children}</I18nextProvider>;
}
