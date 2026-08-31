// Every locale table at once, for the tests that have to compare them ([344]).
//
// The application loads locales one chunk at a time and holds only `en` and
// `de` resident. The parity, orphan and quality guards need all 42 in one
// place, which is the opposite requirement — so they get their own eager glob
// here rather than the app keeping 4.6 MB resident on their behalf.
//
// NOTHING IN THE APPLICATION MAY IMPORT THIS FILE. Vite builds from what the
// entry reaches, so a single app-side import would pull all 42 back into the
// bundle and undo the change without failing anything: the tests would still
// pass, the app would still work, and the download would quietly be five
// megabytes again. If a runtime need for all tables ever appears, it wants
// `loadLocale` in a loop, not this.
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
