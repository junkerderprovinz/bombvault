// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// The crypto donation window ([3524], coin-first since [3554]).
//
// Everything here guards ONE failure: the donor sends money to a string that is
// not the one they picked. Nobody would ever report it. There is no error
// state, no retry and no support ticket, because the person it happens to is a
// stranger who tried to give something away, watched it disappear, and never
// writes.
//
// So the address is checked three times over, at every place it is offered:
//   1. as TEXT, character for character against the list, with nothing
//      shortened, grouped or prettified,
//   2. as the QR CODE, by rebuilding the code from the address on its own and
//      demanding the same path — a picture drawn from a different string is the
//      one wrong address a reader cannot proofread,
//   3. as what the COPY button actually puts on the clipboard.
// Every coin AND every one of its chains is walked, not just the pair that
// happens to be open.
//
// lib/donate.test.ts checks that each address is well formed and that each
// chain points at the wallet it should. This file checks that the right one of
// them reaches the donor unchanged.
// ---------------------------------------------------------------------------
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, expect, it, vi } from "vitest";
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

/** The two pickers, told apart by their accessible names rather than by
 *  position: a test that reaches for "the second listbox" starts passing for
 *  the wrong reason the day the layout moves. */
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
  // Each tile carries a drawing as well as the word. A bare ticker beside
  // seven drawn ones reads as a broken image, and lib/donate.test.ts already
  // holds every coin to HAVING a mark — this is the other half, that the tile
  // actually renders it.
  for (const [i, coin] of CRYPTO_COINS.entries()) {
    expect(tiles[i]!.querySelector("svg"), `${coin.id} tile has no mark`).toBeTruthy();
  }
});

it("shows each coin's own chains, and each chain's own address three ways", () => {
  open();
  for (const [i, coin] of CRYPTO_COINS.entries()) {
    fireEvent.click(coinTiles().getAllByRole("option")[i]!);

    // The chain row is shown for every coin, including the ones with a single
    // chain: it is the line that says WHICH network the address belongs to,
    // and it may not come and go depending on the tile.
    const chips = chainChips().getAllByRole("option");
    expect(chips.map((c) => c.textContent), coin.id).toEqual(coin.networks.map((n) => n.name));

    for (const [j, network] of coin.networks.entries()) {
      fireEvent.click(chainChips().getAllByRole("option")[j]!);
      const where = `${coin.id}/${network.id}`;

      // 1. As text, whole. An address is read back by eye before somebody
      //    sends to it, so it may not be shortened or regrouped anywhere.
      expect(screen.getByText(network.address, { exact: true }), where).toBeTruthy();

      // 2. As the code, rebuilt here from the address by itself: equal paths
      //    mean the window put the same string in, and a QR is the one form of
      //    an address a human cannot check.
      expect(qrPath(), where).toBe(qrPathFor(network.address));

      // 3. As what the button copies.
      fireEvent.click(screen.getByRole("button", { name: new RegExp(en["common.copy"], "i") }));
      expect(copied.at(-1), where).toBe(network.address);
    }
  }
});

it("lands on a chain of the coin just picked, never one left over", () => {
  open();
  // USDT and ETH share Ethereum, so a window that kept the previous chain
  // could look right on one tile and be wrong on the next. The rule is simpler
  // than a shared-chain check: picking a coin always selects that coin's own
  // first chain.
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
    // Exchanges train people to look for a destination tag, so the one chain
    // that does not want one has to say so where the address is. The others
    // must not, or the line stops being information.
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

// ---------------------------------------------------------------------------
// The tiles' colour, which is a second thing that fails silently.
//
// The block above guards the address. This block guards the only other way
// this window can be wrong without anybody reporting it: a mark nobody can
// see. The same stranger who would not write about a lost transfer will not
// write about an empty tile either - they will pick the wrong coin, or give
// up.
//
// Two of the three below were real, and neither showed up in a class list.
// They only appeared once the window was rendered and looked at: Bitcoin's
// #F7931A sitting on the default accent #FCC419 is 1.45:1, an orange mark on a
// yellow tile, and the white XRP mark on the dark theme's white hover was
// white on white. Both are one line of CSS away from coming back.
// ---------------------------------------------------------------------------

/** index.css, read as text - these are rules no jsdom test can compute. */
const indexCss = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "../index.css"), "utf8");

it("gives every coin mark its own brand colour", () => {
  open();
  // jdp, 2026-09-10: "Die Logos der Kryptowährungen sollen die original-logos
  // sein mit deren original Farben". Reversing this file's earlier rule, where
  // every mark took `currentColor` and the grid was eight grey drawings.
  for (const [i, coin] of CRYPTO_COINS.entries()) {
    const path = coinTiles().getAllByRole("option")[i]!.querySelector("svg > path");
    expect(path?.parentElement?.getAttribute("fill"), coin.id).not.toBe("currentColor");
  }
});

it("drops the brand colour on the tile that is FILLED with the accent", () => {
  // The accent is the user's to choose and rainbow mode hands every tile a
  // different one, so no fixed brand colour can be checked against it. The
  // selected tile therefore paints its mark in the ink that fill was paired
  // with - the rule .glim-hue-icon already states for every other glyph in
  // the app. Measured before it existed: 1.45:1, against the 3:1 a graphic
  // needs.
  expect(indexCss).toMatch(/\.glim-coin-tile\.glim-active\s+svg\s*\{[^}]*fill:\s*currentColor/);
});

it("moves XRP's colour off the dark theme's white hover", () => {
  // XRP is the one mark with no colour of its own: it is a shape, drawn in
  // --coin-xrp, which the dark theme sets to white. The hover this grid
  // borrowed from the sibling turns the tile white underneath it.
  expect(indexCss).toMatch(
    /\[data-theme="dark"\]\s+\.glim-coin-tile:not\(\.glim-active\):hover\s*\{[^}]*--coin-xrp/
  );
});

it("keeps the tiles square", () => {
  // jdp, 2026-09-10: "Die kacheln der Kryptowährungen sollen quadratisch
  // sein". `aspect-square` rather than a height, so the square survives the
  // labelling engine swapping the tile's contents between mark, ticker and
  // both - jsdom lays nothing out, so the class is what there is to hold on
  // to here.
  open();
  for (const option of coinTiles().getAllByRole("option")) {
    expect(option.className).toContain("aspect-square");
  }
});
