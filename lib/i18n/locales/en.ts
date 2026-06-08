// English is the source of truth: its key set defines TranslationKey, and every
// other locale must provide exactly these keys (enforced at compile time via the
// Translation type and at runtime by test/locales.test.ts).
//
// Count-neutral phrasing is used throughout — no fragile i18next plural forms.
export const en = {
  "language.label": "Language",
  "theme.toggle": "Toggle theme",

  "onboarding.title": "Welcome to BombVault",
  "onboarding.subtitle": "Set the admin password to finish setup.",
  "onboarding.passwordPlaceholder": "Admin password (min 8 chars)",
  "onboarding.submit": "Create admin",

  "login.title": "BombVault — Sign in",
  "login.error": "Invalid password.",
  "login.passwordPlaceholder": "Admin password",
  "login.submit": "Sign in",

  "dashboard.title": "BombVault — Dashboard",
  "dashboard.body": "P0 foundation is running.",
  "dashboard.spikeLink": "Run the host-integration spike",
  "dashboard.signOut": "Sign out",

  "spike.title": "Host Integration Spike",
  "spike.overall": "Overall:",
  "spike.allOk": "ALL OK",
  "spike.degraded": "DEGRADED",
  "spike.colCheck": "Check",
  "spike.colStatus": "Status",
  "spike.colDetail": "Detail",
  "spike.ok": "OK",
  "spike.fail": "FAIL",
  "spike.probeFailed": "probe failed (see server logs)",
} as const;

export type TranslationKey = keyof typeof en;
export type Translation = Record<TranslationKey, string>;
