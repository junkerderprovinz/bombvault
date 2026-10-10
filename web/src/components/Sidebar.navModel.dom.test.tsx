// @vitest-environment jsdom
// The rail against the navigation registry. The rail renders from the registry,
// and this pins what that has to mean on screen: every row it shows is a
// registry entry the sidebar flag is on for, in registry order, with that
// entry's route, its name and its glyph. Each of the three is read from a
// different field, so swapping two entries' labels or glyphs fails here.
//
// Routes are read off the anchors' href. Sign out is a button, so every anchor
// in the render is a destination and DOM order is the rail's order.
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { I18nProvider, en } from "../lib/i18n";
import type { Settings } from "../lib/api";
import { destinations, sidebarDestinations } from "../lib/navModel";
import { readNavPins, type NavPins } from "../lib/navPins";

const PINS_KEY = "bombvault.navPins";

const DETECTION_OFF = { anomalyEnabled: false } as Settings;
const DETECTION_ON = { anomalyEnabled: true } as Settings;
// Instances are pinned by default for an install that uses one of its modules.
const WITH_FLEET = { anomalyEnabled: false, fleetEnabled: true } as Settings;

interface NavEntry {
  href: string;
  name: string;
  glyph: string;
}

/** The destinations the rail rendered, in DOM order. The glyphs are static
 *  function components, so equal markup means the same component. */
function renderedNavEntries(settings: Settings | null): NavEntry[] {
  const { container } = render(
    <MemoryRouter initialEntries={["/"]}>
      <I18nProvider>
        <Sidebar settings={settings} authEnabled={false} />
      </I18nProvider>
    </MemoryRouter>,
  );
  return Array.from(container.querySelectorAll("a[href]")).map((a) => ({
    href: a.getAttribute("href") ?? "",
    name: (a.getAttribute("aria-label") ?? a.textContent ?? "").trim(),
    glyph: (a.querySelector("svg")?.outerHTML ?? "").replace(/\s+/g, " "),
  }));
}

function registryNavEntries(settings: Settings | null): NavEntry[] {
  return sidebarDestinations(settings, readNavPins(settings)).map((d) => ({
    href: d.to,
    name: en[d.labelKey],
    glyph: renderToStaticMarkup(createElement(d.icon)).replace(/\s+/g, " "),
  }));
}

function pin(pins: Partial<NavPins>) {
  localStorage.setItem(PINS_KEY, JSON.stringify(pins));
}

beforeEach(() => {
  localStorage.clear();
});

afterEach(cleanup);

describe("Sidebar renders exactly the navigation registry", () => {
  // A gate added to the registry and forgotten here would leave its entry
  // hidden on both sides of the comparison, and every assertion would pass
  // without it.
  it("has a fixture that opens every gate", () => {
    pin({ storage: true, instances: true });
    const hidden = destinations(DETECTION_ON, readNavPins(DETECTION_ON))
      .filter((d) => !d.sidebar)
      .map((d) => d.to);
    expect(hidden, "entries no fixture of this file shows in the sidebar").toEqual([]);
  });

  it.each<[string, Settings | null, Partial<NavPins> | null]>([
    ["every gate closed", DETECTION_OFF, { storage: false, instances: false }],
    ["every gate open", DETECTION_ON, { storage: true, instances: true }],
    ["detection on, Instances pinned, storage locations unpinned", DETECTION_ON, { storage: false, instances: true }],
    ["the default pins of an install with a fleet", WITH_FLEET, null],
    ["settings that have not loaded", null, null],
  ])("%s: the rows are the registry's sidebar entries, in order, on route, name and glyph", (_name, settings, pins) => {
    if (pins) pin(pins);
    const expected = registryNavEntries(settings);
    expect(expected.length).toBeGreaterThan(0);
    expect(renderedNavEntries(settings)).toEqual(expected);
  });

  it("shows no per-kind entry, whatever backup types are on", () => {
    const everyKind = {
      vmsEnabled: true,
      flashEnabled: true,
      filesEnabled: true,
      zfsEnabled: true,
      configEnabled: true,
    } as Settings;
    const hrefs = renderedNavEntries(everyKind).map((e) => e.href);
    for (const route of ["/containers", "/vms", "/flash", "/files", "/zfs", "/config"]) {
      expect(hrefs, `${route} left the sidebar`).not.toContain(route);
    }
  });
});
