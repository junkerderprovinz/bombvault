// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Sidebar <-> navModel registry consistency.
//
// Sidebar's header comment says the rail derives from the one ordered nav
// registry (lib/navModel.ts), but the rail still evaluates its own hand-written
// NavItem JSX (with a nextHue() counter and inline settings gates, which is the
// part the registry cannot own; see navModel.ts's header). Two
// lists describing one navigation is exactly the drift the registry exists to
// kill, so this file pins the two together: whatever Sidebar renders must equal
// destinations(settings) filtered to enabled; same routes, same order, same
// gates; across gate combinations, and any divergence on either side fails
// here instead of silently becoming a rail the registry no longer describes.
//
// Routes are read off the rendered anchors' href (NavLink renders `to` as
// href), so the comparison is structural and never re-types the labels. The
// footer's sign-out and view toggle are buttons, not links, so every anchor in
// the render is a nav destination and DOM order is the rail's order.
//
// The comparison is a triple: route +
// accessible name + glyph markup. The rail still renders its own hand-written
// literals (`label={t("nav.vms")} icon={<IconVM />}`), so href parity alone
// passed forever with a rail whose label or glyph had drifted from the
// registry's labelKey/icon for the same route; each lane of the triple is
// read from a different field, so swapping labelKey/icon between two
// registry entries, or retyping the rail's literals, goes red here.
// ---------------------------------------------------------------------------
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { I18nProvider, en } from "../lib/i18n";
import { AdvancedProvider } from "../lib/advanced";
import type { Settings } from "../lib/api";
import { destinations } from "../lib/navModel";

// The gate fields destinations() reads; the navModel.test.ts idiom (`as
// Settings` partials) keeps every unused Settings field out of the fixtures.
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

// A mixed middle state: consecutive gated tabs split on/off around the list,
// the shape most likely to expose an ordering or gate drift.
const MIXED = {
  ...ALL_OFF,
  vmsEnabled: true,
  filesEnabled: true,
  pullEnabled: true,
} as Settings;

/** One nav row's full identity, the triple the rail must match the registry
 *  on: where it goes, what it is called, and what is drawn on it. */
interface NavEntry {
  href: string;
  name: string;
  glyph: string;
}

/** Renders the Sidebar alone (no chrome siblings, so every anchor is a nav
 *  destination) and returns the destinations it rendered, in DOM order, each
 *  as its identity triple. The glyph is the anchor's serialized <svg>: the
 *  navGlyphs are deterministic function components (static paths, no ids, no
 *  randomness), so byte-identical markup is exactly component identity, and
 *  whitespace is collapsed to keep the comparison off formatting. */
function renderedNavEntries(settings: Settings | null): NavEntry[] {
  const { container } = render(
    <MemoryRouter initialEntries={["/"]}>
      <I18nProvider>
        <AdvancedProvider>
          <Sidebar settings={settings} authEnabled={false} />
        </AdvancedProvider>
      </I18nProvider>
    </MemoryRouter>,
  );
  const anchors = Array.from(container.querySelectorAll("a[href]"));
  return anchors.map((a) => ({
    href: a.getAttribute("href") ?? "",
    name: (a.getAttribute("aria-label") ?? a.textContent ?? "").trim(),
    glyph: (a.querySelector("svg")?.outerHTML ?? "").replace(/\s+/g, " "),
  }));
}

/** The registry side of the triple, derived the way every consumer must:
 *  the en-table label for labelKey (what t() resolves to at render) and the
 *  registry icon rendered to static markup. */
function registryNavEntries(settings: Settings | null): NavEntry[] {
  return destinations(settings)
    .filter((d) => d.enabled)
    .map((d) => ({
      href: d.to,
      name: en[d.labelKey],
      glyph: renderToStaticMarkup(createElement(d.icon)).replace(/\s+/g, " "),
    }));
}

beforeEach(() => {
  localStorage.removeItem("bv-lang");
  localStorage.removeItem("bombvault.advanced");
});

afterEach(cleanup);

describe("Sidebar renders exactly the navModel registry", () => {
  // Fixture-completeness guard: the
  // hand-written ALL_ON has the same guard navModel.test.ts carries. Without
  // it a NEW gate field read by destinations() but forgotten in the fixture
  // leaves that entry disabled on both sides of the parity comparison, so every
  // assertion stays green while the destination silently leaves the
  // comparison. This turns that silence red, naming the mechanism.
  it("the ALL_ON fixture really turns every gate on; no destination gated off", () => {
    const stillOff = destinations(ALL_ON)
      .filter((d) => !d.enabled)
      .map((d) => d.to);
    expect(
      stillOff,
      "ALL_ON leaves destinations gated off; a gate field was added to " +
        "navModel.ts but not to this file's fixture, so those destinations " +
        "silently left the parity comparison. Add the field to ALL_ON."
    ).toEqual([]);
  });

  it.each([
    ["every gate off", ALL_OFF],
    ["every gate on", ALL_ON],
    ["a mixed middle state", MIXED],
    ["null settings (pre-boot)", null],
  ])("%s: the rail's rows equal destinations(settings) filtered to enabled, in registry order, on route + name + glyph", (_name, settings) => {
    expect(renderedNavEntries(settings)).toEqual(registryNavEntries(settings));
  });
});
