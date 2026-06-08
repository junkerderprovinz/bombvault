// Server-side translation helper for use in Server Components and Route Handlers.
// Returns a `t(key)` function bound to the resolved language — no hooks, no
// client code, fully compatible with the Next.js App Router server boundary.
//
// Language resolution: cookie > Accept-Language > DEFAULT_LANGUAGE ("en").
import { cookies, headers } from "next/headers";
import { resources, DEFAULT_LANGUAGE, SUPPORTED, type TranslationKey } from "./locales";
import { pickLanguage, COOKIE as LANG_COOKIE } from "./detect";

export async function getTranslator() {
  const cookieStore = await cookies();
  const headerStore = await headers();
  const langCookie = cookieStore.get(LANG_COOKIE)?.value;
  const acceptLanguage = headerStore.get("accept-language");
  const lang = pickLanguage(langCookie, acceptLanguage, SUPPORTED, DEFAULT_LANGUAGE);
  const dict = resources[lang] ?? resources[DEFAULT_LANGUAGE];

  return {
    lang,
    t: (key: TranslationKey): string => dict[key] ?? resources[DEFAULT_LANGUAGE][key] ?? key,
  };
}
