// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// The crypto donation window (#3479).
//
// Everything here guards ONE failure: the donor sends money to a string that is
// not the string they picked. Nobody would ever report it. There is no error
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
// Every chain in the list is walked, not just the one that happens to open.
//
// lib/donate.test.ts checks that each address is well formed. This file checks
// that the well-formed address reaches the donor unchanged.
// ---------------------------------------------------------------------------
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { CryptoDonateDialog } from "./CryptoDonateDialog";
import { QRCode } from "./QRCode";
import { I18nProvider, en } from "../lib/i18n";
import { CRYPTO_CHAINS } from "../lib/donate";

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

/** The QR as it is drawn right now, as one string. */
function qrPath(): string {
  const svg = document.querySelector('[role="dialog"] svg');
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

it("shows every chain, and each one's own address as text, code and copy", () => {
  open();
  const options = screen.getAllByRole("option");
  expect(options.map((o) => o.textContent)).toEqual(
    CRYPTO_CHAINS.map((c) => `${c.name}${c.coins}`)
  );

  for (const [i, chain] of CRYPTO_CHAINS.entries()) {
    fireEvent.click(options[i]!);

    // 1. As text, whole. An address is read back by eye before somebody sends
    //    to it, so it may not be shortened or regrouped anywhere.
    expect(screen.getByText(chain.address, { exact: true })).toBeTruthy();

    // 2. As the code. Built here from the address by itself: equal paths mean
    //    the window put the same string in, and a QR is the one form of the
    //    address a human cannot check.
    expect(qrPath(), chain.id).toBe(qrPathFor(chain.address));

    // 3. As what the button copies.
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["common.copy"], "i") }));
    expect(copied.at(-1), chain.id).toBe(chain.address);
  }
});

it("marks the picked chain, and only that one", () => {
  open();
  const options = screen.getAllByRole("option");
  fireEvent.click(options[2]!);
  expect(options.map((o) => o.getAttribute("aria-selected"))).toEqual(
    CRYPTO_CHAINS.map((_, i) => String(i === 2))
  );
});

it("says on XRP that no tag is needed, and says it nowhere else", () => {
  open();
  const options = screen.getAllByRole("option");
  for (const [i, chain] of CRYPTO_CHAINS.entries()) {
    fireEvent.click(options[i]!);
    // Exchanges train people to look for a destination tag, so the one chain
    // that does not want one has to say so where the address is. The others
    // must not, or the line stops being information.
    const shown = Boolean(screen.queryByText(en["about.cryptoNoTag"], { exact: false }));
    expect(shown, chain.id).toBe(chain.id === "xrp");
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
