// The page's language decides the words of a date, the reader's region its
// order and clock. Intl writes a bare "en" the American way, 11/07 05:05 PM,
// so an English page read in Portugal borrows the region of the browser's
// formatting locale and writes 07/11 17:05.

import { numberLocale } from "./numberFormat";

let browserDefault: string | undefined;
const chosen = new Map<string, string>();

/**
 * dateLocale is the locale every date and time on the page is written in: the
 * page's language with the region and clock of the browser's formatting
 * locale, where Intl has data for that pairing, else the language alone.
 */
export function dateLocale(lang = numberLocale(), browser?: string): string {
  browser ??= browserDefault ??= new Intl.DateTimeFormat().resolvedOptions().locale;
  const key = `${lang} ${browser}`;
  let tag = chosen.get(key);
  if (tag === undefined) {
    tag = withRegionOf(lang, browser);
    chosen.set(key, tag);
  }
  return tag;
}

function withRegionOf(lang: string, browser: string): string {
  try {
    const page = new Intl.Locale(lang);
    const { region, hourCycle } = new Intl.Locale(browser);
    if (page.region || !region) return lang;
    const paired = new Intl.Locale(page.baseName, { region });
    // Without data for the pairing Intl falls back to the bare language.
    if (new Intl.Locale(new Intl.DateTimeFormat(paired).resolvedOptions().locale).baseName !== paired.baseName) return lang;
    return (hourCycle ? new Intl.Locale(paired, { hourCycle }) : paired).toString();
  } catch {
    return lang;
  }
}
