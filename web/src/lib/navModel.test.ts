// ---------------------------------------------------------------------------
// NAV MODEL — guard tests for the ONE ordered navigation registry.
//
// Node environment, no DOM: this renders nothing; it asserts the registry's
// pure data contract the way app/routedPages.test.ts reads source text —
// because the registry's whole job is to be the ONE list every chrome surface
// (desktop Sidebar, the mobile bar + More sheet) derives from, so the
// contract itself is what must hold: full-list order, gates that flip only
// their own entry's `enabled`, bar/More as structural filter derivations, and
// never a hue field (hue assignment stays Sidebar's render-time nextHue()
// counter — see navModel.ts's header comment for why).
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import type { Settings } from "./api";
import { en } from "./i18n";
import { barDestinations, destinations, moreDestinations } from "./navModel";

// Only the domain gates destinations() reads; every other Settings field
// is unused by the registry. `as Settings` matches this repo's own established
// partial-fixture convention (Sidebar.tabColor.dom.test.tsx's `allOn` stub).
// Upstream's resync collapsed the old /receiver + /fleet rows into ONE
// /instances row gated by ANY of its three settings — receiver, fleet and
// pull remain three gates, they now flip the same entry's `enabled`.
const ALL_OFF = {
  vmsEnabled: false,
  flashEnabled: false,
  filesEnabled: false,
  configEnabled: false,
  receiverEnabled: false,
  fleetEnabled: false,
  pullEnabled: false,
} as Settings;

const ALL_ON = {
  ...ALL_OFF,
  vmsEnabled: true,
  flashEnabled: true,
  filesEnabled: true,
  configEnabled: true,
  receiverEnabled: true,
  fleetEnabled: true,
  pullEnabled: true,
} as Settings;

// The documented desktop Sidebar order — dashboard, recovery, containers, the
// gated tabs (instances last of them, gated by ANY of receiver/fleet/pull),
// settings.
const SIDEBAR_ORDER = [
  "/dashboard",
  "/recovery",
  "/containers",
  "/vms",
  "/flash",
  "/files",
  "/config",
  "/instances",
  "/settings",
];

// The four bottom-bar destinations.
const BAR_ROUTES = ["/dashboard", "/containers", "/files", "/settings"];

describe("destinations — the ONE ordered registry", () => {
  it("finds the full registry at all (guards against this test silently matching nothing)", () => {
    expect(destinations(ALL_ON)).toHaveLength(SIDEBAR_ORDER.length);
  });

  it("returns the FULL nine-entry list in desktop Sidebar order, all enabled with every gate on", () => {
    const dests = destinations(ALL_ON);
    expect(dests.map((d) => d.to)).toEqual(SIDEBAR_ORDER);
    expect(dests.every((d) => d.enabled)).toBe(true);
  });

  it("routes are unique (a duplicate route would render two rows for one destination)", () => {
    const routes = destinations(ALL_ON).map((d) => d.to);
    expect(new Set(routes).size).toBe(routes.length);
  });

  it("gates never pre-filter: with every domain off the full list still comes back, only `enabled` shrunken", () => {
    // Sidebar's nextHue() render counter consumes the FULL list and skips
    // disabled entries itself, so the registry must always hand out all nine.
    const dests = destinations(ALL_OFF);
    expect(dests.map((d) => d.to)).toEqual(SIDEBAR_ORDER);
    expect(dests.filter((d) => d.enabled).map((d) => d.to)).toEqual([
      "/dashboard",
      "/recovery",
      "/containers",
      "/settings",
    ]);
  });

  it("null settings (Sidebar's pre-boot state) behaves exactly like every gate off", () => {
    expect(destinations(null).map((d) => d.to)).toEqual(SIDEBAR_ORDER);
    expect(destinations(null).filter((d) => d.enabled).map((d) => d.to)).toEqual([
      "/dashboard",
      "/recovery",
      "/containers",
      "/settings",
    ]);
  });

  it("every labelKey is a real key in the en table (labels are reused verbatim, never re-typed)", () => {
    // No namespace assertion any more: upstream's resync named the merged
    // receiver/fleet/pull row with `instances.title` (its own page's key,
    // outside the nav.* namespace). The contract that matters — the label is
    // an EXISTING table key, never a re-typed string — is the lookup below,
    // and the TranslationKey union already makes a bogus key a compile error.
    for (const d of destinations(ALL_ON)) {
      expect(
        en[d.labelKey],
        `${d.to} carries labelKey "${d.labelKey}" which the en table does not define`
      ).toBeDefined();
    }
  });
});

// ADJACENCY probe: toggling any single gate changes only that entry's
// `enabled` flag — no other entry's position, adjacency, or identity moves.
// This is what lets Sidebar's hue counter stay byte-identical across a gate
// flip for every tab the flip does not touch.
describe("destinations — each gate flips exactly its own entry", () => {
  // All three instances gates flip the SAME /instances row (the row appears
  // as soon as ANY of receiver/fleet/pull is on) — so each gate's flip must
  // light up /instances and touch nothing else.
  const GATES = [
    ["vmsEnabled", "/vms"],
    ["flashEnabled", "/flash"],
    ["filesEnabled", "/files"],
    ["configEnabled", "/config"],
    ["receiverEnabled", "/instances"],
    ["fleetEnabled", "/instances"],
    ["pullEnabled", "/instances"],
  ] as const;

  it.each(GATES)("%s toggles only %s", (field, route) => {
    const before = destinations(ALL_OFF);
    const after = destinations({ ...ALL_OFF, [field]: true });
    // Identity and position of every entry unchanged...
    expect(after.map((d) => d.to)).toEqual(before.map((d) => d.to));
    for (let i = 0; i < after.length; i++) {
      if (after[i].to === route) {
        expect(after[i].enabled, `${route} must be enabled by ${field}`).toBe(true);
      } else {
        expect(
          after[i].enabled,
          `${after[i].to} must not change when ${field} flips`
        ).toBe(before[i].enabled);
      }
    }
  });

  it("bar and More derivations keep their relative order across a gate flip (filters of the ONE list)", () => {
    const beforeBar = barDestinations(ALL_OFF).map((d) => d.to);
    const afterBar = barDestinations({ ...ALL_OFF, fleetEnabled: true }).map((d) => d.to);
    expect(afterBar).toEqual(beforeBar); // instances is not a bar destination
    const beforeMore = moreDestinations(ALL_OFF).map((d) => d.to);
    const afterMore = moreDestinations({ ...ALL_OFF, fleetEnabled: true }).map((d) => d.to);
    // The entries that existed before keep their exact relative order; the
    // instances row joins at its registry position (after Recovery, before
    // nothing else).
    expect(afterMore.filter((r) => beforeMore.includes(r))).toEqual(beforeMore);
    expect(afterMore).toEqual(["/recovery", "/instances"]);
  });
});

describe("barDestinations — the bottom bar's slot list", () => {
  it("at full config: dashboard, containers, files, settings in registry order", () => {
    expect(barDestinations(ALL_ON).map((d) => d.to)).toEqual(BAR_ROUTES);
  });

  it("filesEnabled=false drops exactly the Files slot (bar slots are destinations(settings) filtered to enabled bar entries, matching desktop Sidebar gating, so a gated tab never appears in mobile chrome when the Sidebar hides it)", () => {
    const filesOn = { ...ALL_OFF, filesEnabled: true } as Settings;
    expect(barDestinations(filesOn).map((d) => d.to)).toEqual(["/dashboard", "/containers", "/files", "/settings"]);
    expect(barDestinations(ALL_OFF).map((d) => d.to)).toEqual(["/dashboard", "/containers", "/settings"]);
  });

  it("never surfaces a disabled entry, whatever else is on (a gated tab can never reach mobile chrome while the Sidebar hides it)", () => {
    for (const d of barDestinations(ALL_ON)) {
      expect(d.enabled).toBe(true);
    }
    for (const d of barDestinations(ALL_OFF)) {
      expect(d.enabled).toBe(true);
    }
  });
});

describe("moreDestinations — the More sheet's list", () => {
  it("Recovery-first sidebar order at full config, excluding every bar destination", () => {
    const more = moreDestinations(ALL_ON);
    expect(more.map((d) => d.to)).toEqual(["/recovery", "/vms", "/flash", "/config", "/instances"]);
    for (const bar of BAR_ROUTES) {
      expect(more.find((d) => d.to === bar), `${bar} must never appear in More`).toBeUndefined();
    }
  });

  it("EMPTY probe: with every gate off, Recovery is still there — no caller can ever receive an empty navigation", () => {
    expect(moreDestinations(ALL_OFF).map((d) => d.to)).toEqual(["/recovery"]);
  });
});

describe("registry shape", () => {
  it("no entry carries any hue or colour field — hue assignment is Sidebar's render-time counter, never data", () => {
    for (const d of destinations(ALL_ON)) {
      const hueish = Object.keys(d).filter((k) => /hue|colou?r/i.test(k));
      expect(hueish, `${d.to} carries colour data: ${hueish.join(", ")}`).toEqual([]);
    }
  });

  it("every entry carries a renderable icon component reference", () => {
    for (const d of destinations(ALL_ON)) {
      expect(typeof d.icon, `${d.to} has no icon component`).toBe("function");
    }
  });
});
