// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import {
  FORMER_NAMES,
  FORMER_PAGES,
  LEGACY_HASH,
  SETTINGS_PAGES,
  isSettingsPage,
  orderPages,
  redirectFor,
} from "./settingsPages";
import { PAGE_COMPONENTS } from "./pages";

afterEach(() => localStorage.clear());

describe("the settings tiles", () => {
  it("stand in the order of the fourteen tiles", () => {
    expect(SETTINGS_PAGES.map((p) => p.id)).toEqual([
      "general",
      "look",
      "schedules",
      "storage",
      "restore",
      "notifications",
      "integrity",
      "security",
      "instances",
      "pairing",
      "integrations",
      "apps",
      "system",
      "info",
    ]);
  });

  it("each have a component, and no component is left without a tile", () => {
    expect(Object.keys(PAGE_COMPONENTS).sort()).toEqual(SETTINGS_PAGES.map((p) => p.id).sort());
  });

  it("put a tile the saved order does not know after the ones it does", () => {
    const order = orderPages(["system", "general"]).map((p) => p.id);
    expect(order.slice(0, 3)).toEqual(["system", "general", "look"]);
    expect(order).toHaveLength(SETTINGS_PAGES.length);
  });
});

describe("an address from an earlier version", () => {
  it.each([
    ["retention", "storage#retention"],
    ["offsite", "storage#offsite"],
    ["cloud", "storage#cloud"],
    ["containers", "schedules#order"],
  ])("leads /settings/%s to where its cards went", (id, to) => {
    expect(isSettingsPage(id)).toBe(false);
    expect(redirectFor(id, "")).toBe(to);
  });

  it("leaves a page that kept its id alone", () => {
    for (const id of ["general", "look", "schedules", "storage", "notifications", "integrity", "security", "pairing", "integrations", "apps", "system"]) {
      expect(redirectFor(id, ""), id).toBeNull();
    }
  });

  it("opens General for an id nobody knows", () => {
    expect(redirectFor("nonsense", "")).toBe("general");
  });

  it("leads the hashes of the tabbed page to the card they named", () => {
    expect(redirectFor(undefined, "#offsite")).toBe("storage#offsite");
    expect(redirectFor(undefined, "#anomalies")).toBe("integrity#anomalies");
    expect(redirectFor(undefined, "#schedules")).toBe("schedules");
  });

  it("opens the page seen last, and General when that page is gone", () => {
    localStorage.setItem("bombvault.settingsPage", "notifications");
    expect(redirectFor(undefined, "")).toBe("notifications");
    localStorage.setItem("bombvault.settingsPage", "retention");
    expect(redirectFor(undefined, "")).toBe("general");
  });

  it("only ever leads to a page that exists", () => {
    const targets = [
      ...Object.values(FORMER_PAGES).map((f) => f.to),
      ...Object.values(LEGACY_HASH),
      ...FORMER_NAMES.map((f) => f.page),
    ];
    for (const to of targets) expect(isSettingsPage(to.split("#")[0]), to).toBe(true);
  });
});
