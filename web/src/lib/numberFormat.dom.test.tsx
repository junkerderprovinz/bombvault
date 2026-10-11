// @vitest-environment jsdom
// A size is written in the language the reader picked, from the first render.
import { afterEach, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { humanBytes } from "./forecast";
import { I18nProvider } from "./i18n";

function Size() {
  return <span>{humanBytes(1.2 * 1024 ** 3)}</span>;
}

afterEach(() => {
  cleanup();
  localStorage.clear();
});

it("writes a size the German way on a German page", () => {
  localStorage.setItem("bv-lang", "de");
  render(
    <I18nProvider>
      <Size />
    </I18nProvider>
  );
  expect(screen.getByText("1,2 GB")).toBeTruthy();
  expect(document.documentElement.lang).toBe("de");
});

it("writes a size the English way on an English page", () => {
  localStorage.setItem("bv-lang", "en");
  render(
    <I18nProvider>
      <Size />
    </I18nProvider>
  );
  expect(screen.getByText("1.2 GB")).toBeTruthy();
});
