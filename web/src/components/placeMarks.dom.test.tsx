// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";
import { PlaceMark } from "./placeMarks";

afterEach(cleanup);

const here = dirname(fileURLToPath(import.meta.url));
const marksCss = readFileSync(join(here, "../placeMarks.css"), "utf8");
const indexCss = readFileSync(join(here, "../index.css"), "utf8");

/** The provider ids of internal/places/catalog.go, in tile order. */
const CATALOG = [
  "b2", "s3", "r2", "wasabi", "hetzner-os", "storj", "idrive", "scaleway", "ovh", "digitalocean",
  "ionos", "contabo", "exoscale", "vultr", "gcs", "azure", "storagebox",
  "minio", "seaweedfs", "garage", "ceph", "juicefs", "rustfs", "versitygw", "s3-other",
  "nextcloud", "owncloud", "opencloud", "rest-server", "sftp", "bombvault", "rclone",
  "synology", "qnap", "truenas", "unraid-other", "share", "unraid-folder",
];

function draw(provider: string, size?: number, onFill?: boolean): HTMLElement {
  return render(<PlaceMark provider={provider} size={size} onFill={onFill} />).container;
}

/** The --place-* values one rule of placeMarks.css sets. */
function tokens(selector: string): Record<string, string> {
  const start = marksCss.indexOf(`${selector} {`);
  if (start < 0) throw new Error(`no rule for ${selector}`);
  const body = marksCss.slice(start, marksCss.indexOf("}", start));
  return Object.fromEntries([...body.matchAll(/(--place-[\w-]+):\s*(#[0-9a-f]{6})/g)].map((m) => [m[1]!, m[2]!]));
}

function luminance(hex: string): number {
  const channel = (i: number) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * channel(1) + 0.7152 * channel(3) + 0.0722 * channel(5);
}

function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi! + 0.05) / (lo! + 0.05);
}

/** Every token the brand marks draw in. */
function usedTokens(): string[] {
  const used = new Set<string>();
  for (const id of CATALOG) {
    for (const m of draw(id).innerHTML.matchAll(/var\((--place-[\w-]+)\)/g)) used.add(m[1]!);
    cleanup();
  }
  return [...used];
}

describe("PlaceMark", () => {
  it("draws something for every provider of the catalog", () => {
    for (const id of CATALOG) {
      const box = draw(id);
      expect(box.querySelector("svg, img"), id).toBeTruthy();
      cleanup();
    }
  });

  it("takes each brand from the provider's own entry, never from a pattern", () => {
    const brand = (id: string) => draw(id).querySelector("svg[data-mark]")?.getAttribute("data-mark");
    expect(brand("b2")).toBe("b2");
    expect(brand("s3")).toBe("aws");
    expect(brand("storagebox")).toBe("hetzner");
    expect(brand("unraid-folder")).toBe("unraid");
    expect(brand("versitygw")).toBe("versity");
    // Names that merely contain a brand, or name a member every object has,
    // get the plain glyph.
    for (const id of ["nextcloud-hub", "b2-eu", "dropbox", "constructor", "toString"]) {
      const box = draw(id);
      expect(box.querySelector("[data-mark]"), id).toBeNull();
      expect(box.querySelector('[data-glyph="server"]'), id).toBeTruthy();
    }
  });

  it("stands in our own glyphs where a provider has no mark", () => {
    expect(draw("rest-server").querySelector('[data-glyph="server"]')).toBeTruthy();
    expect(draw("sftp").querySelector('[data-glyph="terminal"]')).toBeTruthy();
    expect(draw("share").querySelector('[data-glyph="share"]')).toBeTruthy();
    // Filled like the rest of the glyph set.
    for (const glyph of document.querySelectorAll("[data-glyph] svg")) {
      expect(glyph.getAttribute("fill")).toBe("currentColor");
      expect(glyph.querySelector("[stroke]")).toBeNull();
    }
  });

  it("draws at the size it is given", () => {
    const svg = draw("b2", 48).querySelector("svg")!;
    expect([svg.getAttribute("width"), svg.getAttribute("height")]).toEqual(["48", "48"]);
    const glyph = draw("sftp", 32).querySelector<HTMLElement>("[data-glyph]")!;
    expect([glyph.style.width, glyph.style.height]).toEqual(["32px", "32px"]);
  });

  it("gives every drawing its own gradient and clip ids", () => {
    const { container } = render(
      <>
        <PlaceMark provider="seaweedfs" />
        <PlaceMark provider="seaweedfs" />
      </>
    );
    const ids = [...container.querySelectorAll("[id]")].map((n) => n.id);
    expect(ids.length).toBeGreaterThan(0);
    expect(new Set(ids).size).toBe(ids.length);
    for (const ref of container.innerHTML.matchAll(/url\(#([^)]+)\)/g)) expect(ids).toContain(ref[1]);
  });

  it("takes the ink of an accent fill when asked", () => {
    expect(draw("b2", 16, true).querySelector("svg")!.getAttribute("class")).toContain("glim-mark-on-fill");
    expect(marksCss).toMatch(/\.glim-mark-on-fill \*,[\s\S]*?\{\s*fill: currentColor;/);
    expect(marksCss).toMatch(/\.glim-coin-tile\.glim-active \.glim-place-mark \*/);
  });

  it("shows our logo drawn for a light ground on a hovered tile", () => {
    const logos = draw("bombvault").querySelectorAll(".glim-place-logo > img");
    expect([...logos].map((img) => img.getAttribute("src"))).toEqual(["/logo.svg", "/logo-light.svg"]);
    expect(marksCss).toMatch(
      /\[data-theme="dark"\] \.glim-coin-tile:not\(\.glim-active\):hover \.glim-place-logo > img:first-child \{\s*display: block;/
    );
    expect(marksCss).toMatch(
      /\[data-theme="dark"\] \.glim-coin-tile:not\(\.glim-active\):hover \.glim-place-logo > img:last-child \{\s*display: none;/
    );
  });
});

describe("the mark colours", () => {
  const dark = tokens(':root,\n[data-theme="dark"]');
  const light = tokens('[data-theme="light"]');
  const hover = tokens(".glim-coin-tile:not(.glim-active):hover");
  const used = usedTokens();

  it("define every token a mark draws in, for both grounds and the hover", () => {
    expect(used.length).toBeGreaterThan(30);
    for (const name of used) {
      expect(dark[name], name).toBeDefined();
      expect(light[name], name).toBeDefined();
      expect(hover[name], name).toBeDefined();
    }
  });

  it("read at 2:1 on the resting tile in both themes", () => {
    for (const name of used) {
      expect(contrast(dark[name]!, "#393939"), name).toBeGreaterThanOrEqual(2);
      expect(contrast(light[name]!, "#e8e8e8"), name).toBeGreaterThanOrEqual(2);
    }
  });

  it("keep 2.5:1 on the grey tile hover", () => {
    const grey = /--carbon-tile-hover:\s*(#[0-9a-f]{6})/.exec(indexCss)![1]!;
    expect(grey).toBe("#a8a8a8");
    for (const name of used) {
      expect(contrast(hover[name]!, grey), name).toBeGreaterThanOrEqual(2.5);
    }
  });
});
