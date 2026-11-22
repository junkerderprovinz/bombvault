import type { UpdateState } from "@glimstone/core";

// Where an installed copy of the app gets its new versions from, read from the
// package that installed it.

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
 * The Version card's state for the installing package. An APK installed by
 * hand, or by a tool that fetches it from the releases page, finds its next
 * version there, and an older app that names no installer gets the store
 * sentence without a name.
 */
export function updatesFrom(installer: string | undefined): UpdateState {
  if (installer === undefined) return { kind: "store" };
  if (installer === "com.android.vending") return { kind: "store", store: "Google Play" };
  if (FDROID.has(installer)) return { kind: "store", store: "F-Droid" };
  return { kind: "direct" };
}
