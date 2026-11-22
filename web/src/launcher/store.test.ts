import { expect, it } from "vitest";
import { updatesFrom } from "./store";

it("names the store that installed the app", () => {
  expect(updatesFrom("com.android.vending")).toEqual({ kind: "store", store: "Google Play" });
  expect(updatesFrom("org.fdroid.fdroid")).toEqual({ kind: "store", store: "F-Droid" });
  expect(updatesFrom("com.looker.droidify")).toEqual({ kind: "store", store: "F-Droid" });
});

it("sends an APK installed by hand to the release page", () => {
  expect(updatesFrom("")).toEqual({ kind: "direct" });
  expect(updatesFrom("com.google.android.packageinstaller")).toEqual({ kind: "direct" });
});

it("names no store for an app too old to say which one installed it", () => {
  expect(updatesFrom(undefined)).toEqual({ kind: "store" });
});
