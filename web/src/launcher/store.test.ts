import { expect, it } from "vitest";
import { storeOf } from "./store";

it("names the store that installed the app", () => {
  expect(storeOf("com.android.vending")).toBe("Google Play");
  expect(storeOf("org.fdroid.fdroid")).toBe("F-Droid");
  expect(storeOf("com.looker.droidify")).toBe("F-Droid");
});

it("points an APK installed by hand to the releases on GitHub", () => {
  expect(storeOf("")).toBe("GitHub Releases");
  expect(storeOf("com.google.android.packageinstaller")).toBe("GitHub Releases");
});
