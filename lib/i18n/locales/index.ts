// Central locale registry. To add a language:
//   1. create ./<code>.ts exporting a `Translation` (TS enforces the full key set)
//   2. import it and add one entry to LANGUAGES + resources below
// The runtime parity test (test/locales.test.ts) guarantees no key is missing or
// empty in any registered language.
import { en, type TranslationKey, type Translation } from "./en";
import { de } from "./de";

export type { TranslationKey, Translation };

export interface Language {
  code: string;
  label: string; // endonym — the language's own name
  rtl?: boolean; // true for RTL languages (Arabic, Hebrew, …)
}

// Order here is the order shown in the language menu.
// RTL locales (ar, he) are listed here so adding them later requires only:
// 1. create the .ts locale file, 2. add { code, label, rtl: true } here,
// 3. add the import + entry in resources below.
export const LANGUAGES: Language[] = [
  { code: "en", label: "English" },
  { code: "de", label: "Deutsch" },
  // Future locales slot in here — each needs a locale file + resources entry.
];

export const DEFAULT_LANGUAGE = "en";

// Supported language codes — the single source for detection logic.
export const SUPPORTED = LANGUAGES.map((l) => l.code);

/** Whether a language code is right-to-left (Arabic, Hebrew, …). */
export const isRtl = (code: string): boolean =>
  LANGUAGES.find((l) => l.code === code)?.rtl ?? false;

export const resources: Record<string, Translation> = {
  en,
  de,
};
