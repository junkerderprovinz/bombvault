// Every locale table at once, for the tests that compare them. The app loads
// locales on demand and keeps only en and de resident, so the eager glob lives
// here. Application code must not import this file: Vite bundles whatever the
// entry reaches, and one import would put all 42 tables back into the download
// without any test failing. A runtime need for every table should call
// loadLocale in a loop.
import { en, de, type Translations } from "./i18n";

const eager = import.meta.glob<{ default: Partial<Translations> }>("./locales/*.ts", {
  eager: true,
});

/** code -> table, for all 42. */
export const allLocales: Record<string, Partial<Translations>> = { en, de };

for (const [path, mod] of Object.entries(eager)) {
  const code = path.replace("./locales/", "").replace(/\.ts$/, "");
  allLocales[code] = mod.default;
}
