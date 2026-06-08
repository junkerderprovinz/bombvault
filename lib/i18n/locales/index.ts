// Central locale registry. To add a language:
//   1. create ./<code>.ts exporting a `Translation` (TS enforces the full key set)
//   2. import it and add one entry to LANGUAGES + resources below
// The runtime parity test (test/locales.test.ts) guarantees no key is missing or
// empty in any registered language.
import { en, type TranslationKey, type Translation } from "./en";
import { de } from "./de";
import { fr } from "./fr";
import { es } from "./es";
import { it } from "./it";
import { pt } from "./pt";
import { nl } from "./nl";
import { pl } from "./pl";
import { ru } from "./ru";
import { uk } from "./uk";
import { cs } from "./cs";
import { sv } from "./sv";
import { da } from "./da";
import { fi } from "./fi";
import { no } from "./no";
import { tr } from "./tr";
import { el } from "./el";
import { hu } from "./hu";
import { ro } from "./ro";
import { ja } from "./ja";
import { ko } from "./ko";
import { zh } from "./zh";
import { ar } from "./ar";
import { he } from "./he";
import { th } from "./th";
import { vi } from "./vi";

export type { TranslationKey, Translation };

export interface Language {
  code: string;
  label: string; // endonym — the language's own name
  rtl?: boolean; // true for RTL languages (Arabic, Hebrew, …)
}

// Order here is the order shown in the language menu.
export const LANGUAGES: Language[] = [
  { code: "en", label: "English" },
  { code: "de", label: "Deutsch" },
  { code: "fr", label: "Français" },
  { code: "es", label: "Español" },
  { code: "it", label: "Italiano" },
  { code: "pt", label: "Português" },
  { code: "nl", label: "Nederlands" },
  { code: "pl", label: "Polski" },
  { code: "ru", label: "Русский" },
  { code: "uk", label: "Українська" },
  { code: "cs", label: "Čeština" },
  { code: "sv", label: "Svenska" },
  { code: "da", label: "Dansk" },
  { code: "fi", label: "Suomi" },
  { code: "no", label: "Norsk" },
  { code: "tr", label: "Türkçe" },
  { code: "el", label: "Ελληνικά" },
  { code: "hu", label: "Magyar" },
  { code: "ro", label: "Română" },
  { code: "ja", label: "日本語" },
  { code: "ko", label: "한국어" },
  { code: "zh", label: "中文" },
  { code: "ar", label: "العربية", rtl: true },
  { code: "he", label: "עברית", rtl: true },
  { code: "th", label: "ไทย" },
  { code: "vi", label: "Tiếng Việt" },
];

export const DEFAULT_LANGUAGE = "en";

// Supported language codes — the single source for detection logic.
export const SUPPORTED = LANGUAGES.map((l) => l.code);

/** Whether a language code is right-to-left (Arabic, Hebrew, …). */
export const isRtl = (code: string): boolean =>
  LANGUAGES.find((l) => l.code === code)?.rtl ?? false;

export const resources: Record<string, Translation> = {
  en, de, fr, es, it, pt, nl, pl, ru, uk, cs, sv, da, fi, no,
  tr, el, hu, ro, ja, ko, zh, ar, he, th, vi,
};
