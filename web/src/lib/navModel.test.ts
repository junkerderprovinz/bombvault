// The navigation registry's data contract: one ordered list, gates that flip
// flags without removing entries, the sidebar, bar and More sheet as filters of
// it, and no colour anywhere in it.
import { describe, expect, it } from "vitest";
import type { Settings } from "./api";
import { ENTRY_LIST_PATH } from "./backupEntry";
import { en } from "./i18n";
import {
  barDestinations,
  destinations,
  moreDestinations,
  onDestination,
  sidebarDestinations,
  type NavDestination,
} from "./navModel";
import type { NavPins } from "./navPins";

// The registry reads one settings field. The pins decide the other two gates.
const DETECTION_OFF = { anomalyEnabled: false } as Settings;
const DETECTION_ON = { anomalyEnabled: true } as Settings;
const UNPINNED: NavPins = { storage: false, instances: false };
const PINNED: NavPins = { storage: true, instances: true };

const ORDER = ["/dashboard", "/anomalies", "/backups", "/restore", "/storage", "/instances", "/settings"];
const BAR = ["/dashboard", "/backups", "/restore", "/settings"];

const routes = (list: NavDestination[]) => list.map((d) => d.to);

describe("destinations, the one ordered registry", () => {
  it("holds every entry in sidebar order, all of them shown with every gate open", () => {
    const all = destinations(DETECTION_ON, PINNED);
    expect(routes(all)).toEqual(ORDER);
    expect(all.every((d) => d.sidebar && d.phone)).toBe(true);
  });

  it("sends Backups to the list of entries", () => {
    const backups = destinations(null, PINNED).find((d) => d.labelKey === "backups.title");
    expect(backups?.to).toBe(ENTRY_LIST_PATH);
  });

  it("names every route once", () => {
    const all = routes(destinations(DETECTION_ON, PINNED));
    expect(new Set(all).size).toBe(all.length);
  });

  it("hands out the full list with every gate closed, with only the flags changed", () => {
    const all = destinations(DETECTION_OFF, UNPINNED);
    expect(routes(all)).toEqual(ORDER);
    expect(routes(all.filter((d) => d.sidebar))).toEqual(["/dashboard", "/backups", "/restore", "/settings"]);
  });

  it("treats settings that have not loaded as detection off", () => {
    expect(destinations(null, UNPINNED)).toEqual(destinations(DETECTION_OFF, UNPINNED));
  });

  it("labels every entry with a key the English table defines", () => {
    for (const d of destinations(DETECTION_ON, PINNED)) {
      expect(en[d.labelKey], `${d.to} carries the label key "${d.labelKey}"`).toBeDefined();
    }
  });

  it("reads Overview, Anomalies, Backups, Restore, Storage locations, Instances, Settings in English", () => {
    const labels = destinations(DETECTION_ON, PINNED).map((d) => en[d.labelKey]);
    expect(labels).toEqual(["Overview", "Anomalies", "Backups", "Restore", "Storage locations", "Instances", "Settings"]);
  });

  it("puts only Settings at the foot of the sidebar, as the last entry", () => {
    const all = destinations(DETECTION_ON, PINNED);
    expect(routes(all.filter((d) => d.footer))).toEqual(["/settings"]);
    expect(all[all.length - 1].footer).toBe(true);
  });

  it("gives Overview the count of what needs the person and Anomalies the open findings", () => {
    const counted = destinations(DETECTION_ON, PINNED)
      .filter((d) => d.count)
      .map((d) => [d.to, d.count]);
    expect(counted).toEqual([
      ["/dashboard", "attention"],
      ["/anomalies", "anomalies"],
    ]);
  });
});

// A gate that moved a second entry would shift that entry's rank, and with it
// the colour of every row behind it.
describe("each gate flips exactly its own entry", () => {
  const closed = destinations(DETECTION_OFF, UNPINNED);

  function changed(after: NavDestination[]): string[] {
    expect(routes(after)).toEqual(routes(closed));
    return after
      .filter((d, i) => d.sidebar !== closed[i].sidebar || d.phone !== closed[i].phone)
      .map((d) => d.to);
  }

  it("detection shows Anomalies in the sidebar and on a phone", () => {
    const after = destinations(DETECTION_ON, UNPINNED);
    expect(changed(after)).toEqual(["/anomalies"]);
    const anomalies = after.find((d) => d.to === "/anomalies");
    expect(anomalies).toMatchObject({ sidebar: true, phone: true });
  });

  it("pinning the storage locations shows them in the sidebar", () => {
    const after = destinations(DETECTION_OFF, { ...UNPINNED, storage: true });
    expect(changed(after)).toEqual(["/storage"]);
    expect(after.find((d) => d.to === "/storage")).toMatchObject({ sidebar: true, phone: true });
  });

  it("pinning Instances shows them in the sidebar and on a phone", () => {
    const after = destinations(DETECTION_OFF, { ...UNPINNED, instances: true });
    expect(changed(after)).toEqual(["/instances"]);
    expect(after.find((d) => d.to === "/instances")).toMatchObject({ sidebar: true, phone: true });
  });
});

describe("sidebarDestinations, the rail's rows", () => {
  it("is the registry filtered to the entries the sidebar shows", () => {
    expect(routes(sidebarDestinations(DETECTION_ON, PINNED))).toEqual(ORDER);
    expect(routes(sidebarDestinations(DETECTION_ON, UNPINNED))).toEqual([
      "/dashboard",
      "/anomalies",
      "/backups",
      "/restore",
      "/settings",
    ]);
    expect(routes(sidebarDestinations(DETECTION_OFF, { storage: true, instances: false }))).toEqual([
      "/dashboard",
      "/backups",
      "/restore",
      "/storage",
      "/settings",
    ]);
  });
});

describe("barDestinations, the bottom bar's slots", () => {
  it("is Overview, Backups, Restore and Settings whatever the gates say", () => {
    expect(routes(barDestinations(DETECTION_ON, PINNED))).toEqual(BAR);
    expect(routes(barDestinations(DETECTION_OFF, UNPINNED))).toEqual(BAR);
    expect(routes(barDestinations(null, UNPINNED))).toEqual(BAR);
  });
});

describe("moreDestinations, the More sheet's rows", () => {
  it("holds every phone entry without a slot, in sidebar order", () => {
    const more = routes(moreDestinations(DETECTION_ON, PINNED));
    expect(more).toEqual(["/anomalies", "/storage", "/instances"]);
    for (const slot of BAR) expect(more, `${slot} has a slot in the bar`).not.toContain(slot);
  });

  it("keeps the storage locations with every gate closed, so the sheet never opens empty", () => {
    expect(routes(moreDestinations(DETECTION_OFF, UNPINNED))).toEqual(["/storage"]);
  });

  it("keeps its order when a gate opens, being a filter of the one list", () => {
    const before = routes(moreDestinations(DETECTION_OFF, UNPINNED));
    const after = routes(moreDestinations(DETECTION_OFF, PINNED));
    expect(after.filter((r) => before.includes(r))).toEqual(before);
    expect(after).toEqual(["/storage", "/instances"]);
  });

  it("splits the phone entries with the bar and loses none", () => {
    const phone = routes(destinations(DETECTION_ON, PINNED).filter((d) => d.phone));
    const shown = [...routes(barDestinations(DETECTION_ON, PINNED)), ...routes(moreDestinations(DETECTION_ON, PINNED))];
    expect(shown.sort()).toEqual(phone.sort());
  });
});

describe("registry shape", () => {
  it("carries no hue or colour field, because a colour follows the rank a surface renders", () => {
    for (const d of destinations(DETECTION_ON, PINNED)) {
      const hueish = Object.keys(d).filter((k) => /hue|colou?r/i.test(k));
      expect(hueish, `${d.to} carries colour data: ${hueish.join(", ")}`).toEqual([]);
    }
  });

  it("gives every entry a glyph component", () => {
    for (const d of destinations(DETECTION_ON, PINNED)) {
      expect(typeof d.icon, `${d.to} has no glyph`).toBe("function");
    }
  });
});

describe("onDestination", () => {
  const storage = destinations(null, PINNED).find((d) => d.to === "/storage")!;

  it("holds for the destination and the pages below it", () => {
    expect(onDestination("/storage", storage)).toBe(true);
    expect(onDestination("/storage/local", storage)).toBe(true);
  });

  it("does not hold for a route that merely starts with the same letters", () => {
    expect(onDestination("/storages", storage)).toBe(false);
    expect(onDestination("/settings/storage", storage)).toBe(false);
  });
});
