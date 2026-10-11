// The list of what is new names elements, links, rows and tabs by words the
// pages carry in their markup. This reads the sources to hold both sides
// together: a renamed mark leaves a dot that never shows, and nothing fails.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { ENTRY_TABS } from "./entry";
import { en } from "./i18n";
import { destinations } from "./navModel";
import { CHANGES, WHATS_NEW_RELEASE } from "./whatsNew";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sources(path);
    return /\.tsx?$/.test(name) && !/\.test\.tsx?$/.test(name) ? [path] : [];
  });
}

const CODE = sources(SRC).map((path) => readFileSync(path, "utf8"));
const found = (pattern: RegExp) => new Set(CODE.flatMap((source) => [...source.matchAll(pattern)].map((m) => m[1])));

// A card takes its mark as a prop, every other element as the attribute.
const MARKED = found(/(?:data-new|newId)="([^"$]+)"/g);
const WAYS = found(/data-new-way="([^"$]+)"/g);
// A link on the way is written with the path it leads to.
const LINKED = found(/\bto[=:]\s*\{?[`"](\/[^`"$]*)[`"]/g);
const SETTINGS_PAGES = found(/\{ id: "(\w+)", label: "[\w.]+", icon:/g);

describe("what is new", () => {
  it("names a release", () => {
    expect(WHATS_NEW_RELEASE).toMatch(/^\d+\.\d+\.\d+$/);
  });

  it("marks every change on an element a page really carries", () => {
    const missing = CHANGES.map((change) => change.id).filter((id) => !MARKED.has(id));
    expect(missing).toEqual([]);
  });

  it("leaves no mark in a page that no change stands behind", () => {
    const known = new Set(CHANGES.map((change) => change.id));
    expect([...MARKED].filter((id) => !known.has(id))).toEqual([]);
  });

  it("gives every change an id of its own", () => {
    const ids = CHANGES.map((change) => change.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it("leads to every change over links, rows and tabs that exist", () => {
    const rail = new Set(destinations(null, { storage: true, instances: true }).map((d) => d.to));
    const reachable = (path: string) =>
      rail.has(path) || LINKED.has(path) || (path.startsWith("/settings/") && SETTINGS_PAGES.has(path.slice("/settings/".length)));
    const broken: string[] = [];
    for (const change of CHANGES) {
      expect(change.way.length, change.id).toBeGreaterThan(0);
      for (const step of change.way) {
        if (typeof step !== "string") {
          if (!WAYS.has(step.within) || !(ENTRY_TABS as readonly string[]).includes(step.tab)) broken.push(`${change.id}: tab ${step.tab} in ${step.within}`);
        } else if (step.startsWith("/")) {
          if (!reachable(step)) broken.push(`${change.id}: ${step}`);
        } else if (!WAYS.has(step)) {
          broken.push(`${change.id}: ${step}`);
        }
      }
    }
    expect(broken).toEqual([]);
  });

  it("starts every way in the navigation", () => {
    const rail = new Set(destinations(null, { storage: true, instances: true }).map((d) => d.to));
    expect(CHANGES.filter((change) => !rail.has(change.way[0] as string)).map((change) => change.id)).toEqual([]);
  });

  it("sends the list to an address that names no single entry", () => {
    expect(CHANGES.filter((change) => (change.go ?? change.page).includes(":")).map((change) => change.id)).toEqual([]);
  });

  it("says each change in a sentence and names its page", () => {
    for (const change of CHANGES) {
      expect(en[change.text], change.id).toMatch(/\.$/);
      expect(change.where.length, change.id).toBeGreaterThan(0);
    }
  });
});
