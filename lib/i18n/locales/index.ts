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
  flag: string; // ISO 3166-1 alpha-2 region code used to pick the flag SVG
  rtl?: boolean; // true for RTL languages (Arabic, Hebrew, …)
}

// Order here is the order shown in the language menu.
export const LANGUAGES: Language[] = [
  { code: "en", label: "English", flag: "gb" },
  { code: "de", label: "Deutsch", flag: "de" },
  { code: "fr", label: "Français", flag: "fr" },
  { code: "es", label: "Español", flag: "es" },
  { code: "it", label: "Italiano", flag: "it" },
  { code: "pt", label: "Português", flag: "pt" },
  { code: "nl", label: "Nederlands", flag: "nl" },
  { code: "pl", label: "Polski", flag: "pl" },
  { code: "ru", label: "Русский", flag: "ru" },
  { code: "uk", label: "Українська", flag: "ua" },
  { code: "cs", label: "Čeština", flag: "cz" },
  { code: "sv", label: "Svenska", flag: "se" },
  { code: "da", label: "Dansk", flag: "dk" },
  { code: "fi", label: "Suomi", flag: "fi" },
  { code: "no", label: "Norsk", flag: "no" },
  { code: "tr", label: "Türkçe", flag: "tr" },
  { code: "el", label: "Ελληνικά", flag: "gr" },
  { code: "hu", label: "Magyar", flag: "hu" },
  { code: "ro", label: "Română", flag: "ro" },
  { code: "ja", label: "日本語", flag: "jp" },
  { code: "ko", label: "한국어", flag: "kr" },
  { code: "zh", label: "中文", flag: "cn" },
  { code: "ar", label: "العربية", flag: "sa", rtl: true },
  { code: "he", label: "עברית", flag: "il", rtl: true },
  { code: "th", label: "ไทย", flag: "th" },
  { code: "vi", label: "Tiếng Việt", flag: "vn" },
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
