// Server-side translation helper for use in Server Components and Route Handlers.
// Returns a `t(key)` function bound to the resolved language — no hooks, no
// client code, fully compatible with the Next.js App Router server boundary.
//
// Language resolution: an explicit cookie choice wins; when no cookie is set
// the UI defaults to "en" — Accept-Language is intentionally NOT used so the
// default experience is always English regardless of the browser's locale.
import { cookies } from "next/headers";
import { resources, DEFAULT_LANGUAGE, SUPPORTED, type TranslationKey } from "./locales";
import { resolveLanguage, COOKIE as LANG_COOKIE } from "./detect";

export async function getTranslator() {
  const cookieStore = await cookies();
  const langCookie = cookieStore.get(LANG_COOKIE)?.value;

  // Only honour an explicit cookie. No Accept-Language fallback — default is en.
  const candidates = langCookie ? [langCookie] : [];
  const lang = resolveLanguage(candidates, SUPPORTED, DEFAULT_LANGUAGE);
  const dict = resources[lang] ?? resources[DEFAULT_LANGUAGE];

  return {
    lang,
    t: (key: TranslationKey): string => dict[key] ?? resources[DEFAULT_LANGUAGE][key] ?? key,
  };
}
