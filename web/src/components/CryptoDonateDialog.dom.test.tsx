// @vitest-environment jsdom
// A donor who sends to the wrong address loses the money and never reports it,
// so every coin and every chain is checked three ways: the address as text,
// unshortened; the QR code, rebuilt from the address alone; and what the copy
// button puts on the clipboard. lib/donate.test.ts checks the addresses
// themselves; this file checks that the right one reaches the donor.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, expect, it, vi } from "vitest";
import { setLabelMode } from "../lib/controls";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { CryptoDonateDialog } from "./CryptoDonateDialog";
import { QRCode } from "./QRCode";
import { I18nProvider, en } from "../lib/i18n";
import { CRYPTO_COINS } from "../lib/donate";

const copied: string[] = [];
vi.mock("../lib/clipboard", () => ({
  copyText: (text: string) => {
    copied.push(text);
    return Promise.resolve(true);
  },
}));

afterEach(() => {
  copied.length = 0;
  cleanup();
});

function open(onClose: () => void = () => {}) {
  return render(
    <I18nProvider>
      <CryptoDonateDialog onClose={onClose} />
    </I18nProvider>
  );
}

/** The two pickers, found by accessible name rather than position. */
const coinTiles = () => within(screen.getByRole("listbox", { name: en["about.cryptoTitle"] }));
const chainChips = () => within(screen.getByRole("listbox", { name: en["about.cryptoNetworks"] }));

/** The QR as it is drawn right now, as one string. */
function qrPath(): string {
  const svg = document.querySelector('[role="dialog"] svg[shape-rendering="crispEdges"]');
  return svg?.querySelector("path")?.getAttribute("d") ?? "";
}

/** The same code built from one string alone, for comparison. Nothing is
 *  mocked, so an equal path means the window encoded exactly this. */
function qrPathFor(value: string): string {
  const { container, unmount } = render(<QRCode value={value} />);
  const d = container.querySelector("path")?.getAttribute("d") ?? "";
  unmount();
  return d;
}

it("offers every coin as a tile, with its mark and its ticker", () => {
  open();
  const tiles = coinTiles().getAllByRole("option");
  expect(tiles.map((t) => t.textContent)).toEqual(CRYPTO_COINS.map((c) => c.symbol));
  for (const [i, coin] of CRYPTO_COINS.entries()) {
    expect(tiles[i]!.querySelector("svg"), `${coin.id} tile has no mark`).toBeTruthy();
  }
});

it("shows each coin's own chains, and each chain's own address three ways", () => {
  open();
  for (const [i, coin] of CRYPTO_COINS.entries()) {
    fireEvent.click(coinTiles().getAllByRole("option")[i]!);

    // The chain row shows even for a single chain, since it names the network.
    const chips = chainChips().getAllByRole("option");
    expect(chips.map((c) => c.textContent), coin.id).toEqual(coin.networks.map((n) => n.name));

    for (const [j, network] of coin.networks.entries()) {
      fireEvent.click(chainChips().getAllByRole("option")[j]!);
      const where = `${coin.id}/${network.id}`;

      expect(screen.getByText(network.address, { exact: true }), where).toBeTruthy();

      // Equal paths mean the window encoded the same string.
      expect(qrPath(), where).toBe(qrPathFor(network.address));

      fireEvent.click(screen.getByRole("button", { name: new RegExp(en["common.copy"], "i") }));
      expect(copied.at(-1), where).toBe(network.address);
    }
  }
});

it("selects the first chain of the coin just picked", () => {
  open();
  for (const [i, coin] of CRYPTO_COINS.entries()) {
    fireEvent.click(coinTiles().getAllByRole("option")[i]!);
    const chips = chainChips().getAllByRole("option");
    expect(chips.map((c) => c.getAttribute("aria-selected")), coin.id).toEqual(
      coin.networks.map((_, j) => String(j === 0))
    );
    expect(screen.getByText(coin.networks[0]!.address, { exact: true })).toBeTruthy();
  }
});

it("marks the picked coin, and only that one", () => {
  open();
  fireEvent.click(coinTiles().getAllByRole("option")[2]!);
  expect(coinTiles().getAllByRole("option").map((o) => o.getAttribute("aria-selected"))).toEqual(
    CRYPTO_COINS.map((_, i) => String(i === 2))
  );
});

it("says on XRP that no tag is needed, and says it nowhere else", () => {
  open();
  for (const [i, coin] of CRYPTO_COINS.entries()) {
    fireEvent.click(coinTiles().getAllByRole("option")[i]!);
    const shown = Boolean(screen.queryByText(en["about.cryptoNoTag"], { exact: false }));
    expect(shown, coin.id).toBe(coin.id === "xrp");
  }
});

it("closes on Escape and on the button", () => {
  const onClose = vi.fn();
  open(onClose);
  fireEvent.keyDown(document, { key: "Escape" });
  expect(onClose).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole("button", { name: new RegExp(en["common.close"], "i") }));
  expect(onClose).toHaveBeenCalledTimes(2);
});

// The coin marks must stay visible on their tiles. Bitcoin's #F7931A on the
// default accent #FCC419 is 1.45:1, and the white XRP mark on the dark theme's
// white hover disappears; both are guarded by a rule in index.css.

/** index.css as text, since jsdom does not compute these rules. */
const indexCss = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "../index.css"), "utf8");

it("gives every coin mark its own brand colour", () => {
  open();
  for (const [i, coin] of CRYPTO_COINS.entries()) {
    const path = coinTiles().getAllByRole("option")[i]!.querySelector("svg > path");
    expect(path?.parentElement?.getAttribute("fill"), coin.id).not.toBe("currentColor");
  }
});

it("drops the brand colour on the tile filled with the accent", () => {
  // The accent is user-chosen and differs per tile in rainbow mode, so no
  // brand colour is safe on it; the mark takes the fill's paired ink instead.
  expect(indexCss).toMatch(/\.glim-coin-tile\.glim-active\s+svg\s*\{[^}]*fill:\s*currentColor/);
});

it("moves XRP's colour off the dark theme's white hover", () => {
  // The XRP mark is drawn in --coin-xrp, which the dark theme sets to white,
  // the same colour as the dark theme's hover.
  expect(indexCss).toMatch(
    /\[data-theme="dark"\]\s+\.glim-coin-tile:not\(\.glim-active\):hover\s*\{[^}]*--coin-xrp/
  );
});

it("keeps the tiles square", () => {
  // jsdom does no layout, so the class is what can be checked.
  open();
  for (const option of coinTiles().getAllByRole("option")) {
    expect(option.className).toContain("aspect-square");
  }
});

it("puts the tickers into the reactive label mode, and only there", () => {
  // The reveal is CSS that jsdom does not apply, so this checks the class.
  // Counted inside the grid, because the Copy and Close buttons carry the same
  // class in reactive mode.
  const grid = () => screen.getByRole("listbox", { name: en["about.cryptoTitle"] });

  setLabelMode("buttons", "reactive");
  const { unmount } = open();
  expect(grid().querySelectorAll(".glim-label-reactive").length).toBe(CRYPTO_COINS.length);
  expect(coinTiles().getAllByRole("option")[0]!.className).toContain("glim-reactive");
  unmount();

  setLabelMode("buttons", "textGlyph");
  open();
  expect(grid().querySelectorAll(".glim-label-reactive").length).toBe(0);
  expect(coinTiles().getAllByRole("option")[0]!.className).not.toContain("glim-reactive");
});
