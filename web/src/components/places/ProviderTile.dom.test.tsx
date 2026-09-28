// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { ProviderGrid } from "./ProviderTile";
import { placeTile } from "../placeMarks";
import { I18nProvider, en } from "../../lib/i18n";
import { setLabelMode } from "../../lib/controls";
import type { CatalogProvider } from "../../lib/places";

afterEach(() => {
  cleanup();
  setLabelMode("buttons", "textGlyph");
  vi.restoreAllMocks();
});

const p = (id: string, group: CatalogProvider["group"]): CatalogProvider => ({ id, group, kind: "s3", fields: [] });

/** Out of group order: nothing promises the server sorts by group. */
const PROVIDERS = [
  p("b2", "cloud"),
  p("minio", "self"),
  p("s3", "cloud"),
  p("hetzner-os", "cloud"),
  p("synology", "here"),
  p("wasabi", "cloud"),
  p("versitygw", "self"),
];

function grid(onPick = vi.fn(), selected: string | null = null) {
  render(
    <I18nProvider>
      <ProviderGrid providers={PROVIDERS} selected={selected} onPick={onPick} />
    </I18nProvider>
  );
  return onPick;
}

const tiles = () => within(screen.getByRole("listbox", { name: en["places.pick"] })).getAllByRole("option");

/** jsdom lays nothing out: rows of three, each group starting a new row. */
function layOut() {
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
    const row = this.parentElement;
    const group = this.closest('[role="group"]');
    const i = Array.from(row?.children ?? []).indexOf(this);
    const g = Array.from(group?.parentElement?.children ?? []).indexOf(group as Element);
    return { top: g * 400 + Math.floor(i / 3) * 120, left: (i % 3) * 120, width: 112, height: 112 } as DOMRect;
  });
}

describe("ProviderGrid", () => {
  it("puts every provider in its group, under the group's section badge", () => {
    grid();
    const groups = screen.getAllByRole("group");
    expect(groups.map((g) => g.getAttribute("aria-labelledby") && document.getElementById(g.getAttribute("aria-labelledby")!)?.textContent)).toEqual([
      en["places.group.cloud"],
      en["places.group.self"],
      en["places.group.here"],
    ]);
    expect(tiles().map((o) => o.getAttribute("aria-label"))).toEqual([
      "Backblaze B2", "Amazon S3", "Hetzner Object Storage", "Wasabi", "MinIO", "Versity S3 Gateway", "Synology",
    ]);
  });

  it("gives every tile the same size whatever its name: fixed on the desktop, its cell's square on a phone", () => {
    grid();
    for (const tile of tiles()) {
      const classes = tile.className.split(/\s+/);
      expect(classes).toContain("h-28");
      expect(classes).toContain("w-28");
      expect(classes).not.toContain("aspect-square");
      expect(classes).toEqual(expect.arrayContaining(["max-md:aspect-square", "max-md:h-auto", "max-md:w-full"]));
    }
    const long = within(tiles()[2]!).getByText("Hetzner Object Storage");
    expect(long.className).toContain("line-clamp-2");
    expect(long.className).toContain("text-xs");
    expect(tiles()[2]!.querySelector("svg")?.getAttribute("width")).toBe("48");
  });

  it("lays each group's tiles out in fixed columns, centred in the window", () => {
    grid();
    for (const group of screen.getAllByRole("group")) {
      const row = within(group).getAllByRole("option")[0]!.parentElement!;
      expect(row.className).toMatch(/\bgrid\b/);
      expect(row.className).toContain("grid-cols-[repeat(auto-fill,7rem)]");
      expect(row.className).toMatch(/\bjustify-center\b/);
      expect(row.className).not.toMatch(/\bflex-wrap\b/);
    }
  });

  it("is the coin tile, lit in its brand's colour under the pointer", () => {
    grid();
    // The tiles in group order, as the first test reads them.
    const ids = ["b2", "s3", "hetzner-os", "wasabi", "minio", "versitygw", "synology"];
    for (const [i, tile] of tiles().entries()) {
      const lit = placeTile(ids[i]!)!;
      expect(tile.className).toContain("glim-coin-tile");
      expect(tile.className).toContain("glim-brand-tile");
      expect(tile.className).not.toMatch(/transition|hover:bg-/);
      expect(tile.style.getPropertyValue("--tile")).toBe(lit.color);
      expect(tile.style.getPropertyValue("--tile-ink")).toBe(lit.ink);
      expect(tile.className).not.toMatch(/outline-none|outline-0/);
    }
  });

  it("gives a provider without a brand mark the hover one step up the ramp", () => {
    render(
      <I18nProvider>
        <ProviderGrid providers={[p("s3-other", "self"), p("bombvault", "self")]} selected={null} onPick={vi.fn()} />
      </I18nProvider>
    );
    for (const tile of tiles()) {
      expect(tile.className).toContain("hover:bg-carbon-surface3");
      expect(tile.className).not.toContain("glim-brand-tile");
      expect(tile.style.getPropertyValue("--tile")).toBe("");
    }
  });

  it("keeps the accent on the tile it came back from", () => {
    grid(vi.fn(), "wasabi");
    const picked = tiles()[3]!;
    expect(picked.className).toContain("glim-active");
    expect(picked.className).not.toContain("glim-brand-tile");
  });

  it("walks the tiles with the arrow keys, Home and End, across the groups", () => {
    layOut();
    grid();
    expect(tiles().filter((o) => o.tabIndex === 0)).toEqual([tiles()[0]]);
    tiles()[0]!.focus();
    fireEvent.keyDown(tiles()[0]!, { key: "ArrowRight" });
    expect(document.activeElement).toBe(tiles()[1]);
    fireEvent.keyDown(tiles()[1]!, { key: "ArrowDown" });
    expect(document.activeElement).toBe(tiles()[3]);
    fireEvent.keyDown(tiles()[3]!, { key: "ArrowDown" });
    expect(document.activeElement).toBe(tiles()[4]);
    fireEvent.keyDown(tiles()[4]!, { key: "End" });
    expect(document.activeElement).toBe(tiles()[6]);
    fireEvent.keyDown(tiles()[6]!, { key: "Home" });
    expect(document.activeElement).toBe(tiles()[0]);
    expect(tiles().filter((o) => o.tabIndex === 0)).toEqual([tiles()[0]]);
  });

  it("leaves a key with Alt, Ctrl or Meta to the browser", () => {
    layOut();
    grid();
    tiles()[1]!.focus();
    for (const modifier of ["altKey", "ctrlKey", "metaKey"]) {
      for (const key of ["ArrowLeft", "Home"]) {
        expect(fireEvent.keyDown(tiles()[1]!, { key, [modifier]: true }), `${modifier} ${key}`).toBe(true);
        expect(document.activeElement).toBe(tiles()[1]);
      }
    }
  });

  it("picks a tile on a press only, and marks the one it came back from", () => {
    const onPick = grid();
    fireEvent.keyDown(tiles()[0]!, { key: "ArrowRight" });
    expect(onPick).not.toHaveBeenCalled();
    fireEvent.click(tiles()[3]!);
    expect(onPick).toHaveBeenCalledWith(PROVIDERS[5]);
    cleanup();
    grid(vi.fn(), "wasabi");
    expect(tiles().map((o) => o.getAttribute("aria-selected"))).toEqual(["false", "false", "false", "true", "false", "false", "false"]);
    expect(tiles()[3]!.tabIndex).toBe(0);
  });

  it("follows the label engine, and keeps both mark and name in reactive mode", () => {
    setLabelMode("buttons", "glyph");
    grid();
    expect(tiles()[0]!.textContent).toBe("");
    expect(tiles()[0]!.querySelector("svg")).toBeTruthy();
    cleanup();

    setLabelMode("buttons", "text");
    grid();
    expect(tiles()[0]!.querySelector("svg")).toBeNull();
    expect(tiles()[0]!.textContent).toBe("Backblaze B2");
    cleanup();

    setLabelMode("buttons", "reactive");
    grid();
    expect(tiles()[0]!.querySelector("svg")).toBeTruthy();
    expect(tiles()[0]!.textContent).toBe("Backblaze B2");
    expect(tiles()[0]!.querySelector(".glim-label-reactive")).toBeNull();
  });
});
