// i18next instance for the client. Initialized once with the full resource set.
// The initial language is decided on the server (cookie -> Accept-Language ->
// fallback) and passed in, so the server and the first client render agree —
// no hydration mismatch, and the page renders translated without JS.
import i18next from "i18next";
import { initReactI18next } from "react-i18next";
import { DEFAULT_LANGUAGE, resources } from "./locales/index";

// i18next wants resources shaped as { <lng>: { <ns>: { <key>: value } } }.
const i18nResources = Object.fromEntries(
  Object.entries(resources).map(([code, dict]) => [code, { translation: dict }]),
);

let initialized = false;

export function initI18n(lng: string = DEFAULT_LANGUAGE): typeof i18next {
  if (!initialized) {
    void i18next.use(initReactI18next).init({
      resources: i18nResources,
      lng,
      fallbackLng: DEFAULT_LANGUAGE,
      interpolation: { escapeValue: false }, // React already escapes
      returnNull: false,
    });
    initialized = true;
  } else if (i18next.language !== lng) {
    void i18next.changeLanguage(lng);
  }
  return i18next;
}
