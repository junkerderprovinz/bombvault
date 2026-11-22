// Which store an installed copy of the app gets its new versions from, read
// from the package that installed it.

/** The F-Droid clients that install from F-Droid's repository. */
const FDROID = new Set([
  "org.fdroid.fdroid",
  "org.fdroid.basic",
  "com.looker.droidify",
  "com.machiav3lli.fdroid",
  "eu.bubu1.fdroidclassic",
  "nya.kitsunyan.foxydroid",
]);

/**
 * The store's name for the Version card. An APK installed by hand, or by a
 * tool that fetches it from the releases page, is updated from GitHub.
 */
export function storeOf(installer: string): string {
  if (installer === "com.android.vending") return "Google Play";
  if (FDROID.has(installer)) return "F-Droid";
  return "GitHub Releases";
}
