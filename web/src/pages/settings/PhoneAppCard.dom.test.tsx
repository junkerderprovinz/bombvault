// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en, useT } from "../../lib/i18n";

let version: string | undefined = "v9.8.0";
vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  getHealth: () => Promise.resolve({ ok: true, version }),
}));

let desktop = true;
vi.mock("../../lib/useMediaQuery", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/useMediaQuery")>()),
  useIsDesktop: () => desktop,
}));

const { PhoneAppCard, apkRelease } = await import("./AppsCards");

function Harness() {
  const { t } = useT();
  return <PhoneAppCard t={t} />;
}

async function renderCard() {
  await act(async () => {
    render(
      <I18nProvider>
        <Harness />
      </I18nProvider>
    );
  });
}

const REPO = "https://github.com/junkerderprovinz/bombvault";

describe("apkRelease", () => {
  it.each([
    ["v9.8.0", "9.8.0"],
    ["9.7.0", "9.7.0"],
    ["v10.0.1", "10.0.1"],
    ["v9.6.4", null],
    ["v8.11.0", null],
    ["v9.8.0+main.59b73a6", null],
    ["dev", null],
    [undefined, null],
  ])("names the release whose APK %s runs with", (running, want) => {
    expect(apkRelease(running)).toBe(want);
  });
});

describe("PhoneAppCard", () => {
  beforeEach(() => {
    window.localStorage.setItem("bv-lang", "en");
    version = "v9.8.0";
    desktop = true;
  });
  afterEach(cleanup);

  it("gives the running release's APK and names that release in the corner", async () => {
    await renderCard();
    expect(screen.getByRole("link", { name: "Android APK" }).getAttribute("href")).toBe(
      `${REPO}/releases/download/v9.8.0/bombvault-android.apk`
    );
    expect(screen.getByRole("link", { name: "v9.8.0" }).getAttribute("href")).toBe(`${REPO}/releases/tag/v9.8.0`);
  });

  it("gives the latest APK and no number to a server that is no release", async () => {
    version = "v9.8.0+main.59b73a6";
    await renderCard();
    expect(screen.getByRole("link", { name: "Android APK" }).getAttribute("href")).toBe(
      `${REPO}/releases/latest/download/bombvault-android.apk`
    );
    expect(screen.queryByRole("link", { name: /^v\d/ })).toBeNull();
  });

  it("shows both stores as coming, before the APK", async () => {
    await renderCard();
    const soon = en["apps.soon"];
    const names = Array.from(document.querySelectorAll(".glim-readme-btn")).map((b) => b.getAttribute("aria-label"));
    expect(names).toEqual([`Google Play ${soon}`, `F-Droid ${soon}`, "Android APK", `${en["apps.phone.qrCode"]} APK`]);
    expect(screen.queryByRole("link", { name: /Google Play|F-Droid/ })).toBeNull();
  });

  it("opens the APK's QR code from its segment and closes it on Escape", async () => {
    await renderCard();
    fireEvent.click(screen.getByRole("button", { name: `${en["apps.phone.qrCode"]} APK` }));
    expect(screen.getByRole("dialog", { name: en["apps.phone.qrCode"] }).querySelector("svg")).not.toBeNull();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("leaves the QR segment out on a phone", async () => {
    desktop = false;
    await renderCard();
    expect(screen.queryByRole("button", { name: `${en["apps.phone.qrCode"]} APK` })).toBeNull();
    expect(screen.getByRole("link", { name: "Android APK" })).toBeTruthy();
  });
});
