import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, it } from "vitest";

import { CRYPTO_COINS } from "./donate";

/**
 * The donation page in the docs says the same thing as the app.
 *
 * The addresses now live in two places: this module, which the app's own
 * window reads, and `docs/donate.md`, which the README's Crypto button opens
 * because a README cannot open a window and jdp wants no addresses in it
 * (2026-09-11). Two copies of a payment address is exactly the kind of pair
 * that drifts, and the drift is expensive in a way nothing else here is: a
 * stale address does not fail, it accepts the money and keeps it, and the
 * person it happens to is a stranger who tried to give something away.
 *
 * So the page is checked against the module rather than trusted. Both
 * directions, because either is a real failure:
 *
 *   - an address the app offers that the page does not list is a donor sent
 *     somewhere the app no longer uses,
 *   - a long string on the page that the app does not know is an address
 *     nobody is watching, whether it is a typo or a leftover.
 *
 * It does NOT check the prose, the grouping or the table's shape. A test that
 * pinned the wording would fail on every edit and be deleted within a month;
 * what has to hold is the strings the money follows.
 */

const page = readFileSync(
  join(dirname(fileURLToPath(import.meta.url)), "../../../docs/donate.md"),
  "utf8"
);

/** Every address the app can hand out, once each. */
const offered = [...new Set(CRYPTO_COINS.flatMap((c) => c.networks.map((n) => n.address)))];

it("lists every address the app offers", () => {
  expect(offered.length).toBeGreaterThan(0);
  for (const address of offered) {
    expect(page, `${address} is offered in the app but missing from docs/donate.md`)
      .toContain(address);
  }
});

it("carries no address the app does not know", () => {
  // Only the backticked cells are read, so a URL or a word in the prose can
  // never be mistaken for an address. 25 characters is comfortably below the
  // shortest of these (an XRP account) and far above anything else in code
  // ticks on that page.
  const onPage = [...page.matchAll(/`([A-Za-z0-9]{25,})`/g)].map((m) => m[1]!);
  expect(onPage.length).toBeGreaterThan(0);
  for (const candidate of onPage) {
    expect(offered, `${candidate} is on docs/donate.md but the app never offers it`)
      .toContain(candidate);
  }
});

it("names every chain the app can send on", () => {
  // The page is grouped BY CHAIN, which is the rule the window follows too:
  // what decides whether the money arrives is the network, not the coin. A
  // chain the app offers and the page omits sends somebody looking for a row
  // that is not there.
  const chains = [...new Set(CRYPTO_COINS.flatMap((c) => c.networks.map((n) => n.name)))];
  for (const chain of chains) {
    expect(page, `chain "${chain}" is offered in the app but not named on docs/donate.md`)
      .toContain(chain);
  }
});
