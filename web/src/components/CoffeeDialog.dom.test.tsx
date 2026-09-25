// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { CoffeeDialog } from "./CoffeeDialog";
import { I18nProvider, en } from "../lib/i18n";

afterEach(cleanup);

function open(onClose: () => void = () => {}) {
  return render(
    <I18nProvider>
      <CoffeeDialog onClose={onClose} />
    </I18nProvider>
  );
}

it("frames the maker's BMAC widget, with payments allowed", () => {
  open();
  const frame = screen.getByTitle("Buy Me a Coffee");
  // The widget page is the one BMAC page that may be framed; the profile page
  // answers SAMEORIGIN and would stay empty.
  expect(frame.getAttribute("src")).toBe(
    "https://buymeacoffee.com/widget/page/junkerderprovinz?description=&color=%23FFDD00"
  );
  expect(frame.getAttribute("allow")).toBe("payment");
});

it("says the payment runs through BMAC, under the brand name as title", () => {
  open();
  const dialog = screen.getByRole("dialog", { name: "Buy Me a Coffee" });
  expect(dialog.textContent).toContain(en["about.coffeeIntro"]);
});

it("closes on Escape and on the button", () => {
  const onClose = vi.fn();
  open(onClose);
  fireEvent.keyDown(document, { key: "Escape" });
  expect(onClose).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole("button", { name: new RegExp(en["common.close"], "i") }));
  expect(onClose).toHaveBeenCalledTimes(2);
});
