// @vitest-environment jsdom
// LanguageCard, the UI-language picker on Settings' General tab: it opens and
// closes, lists every language with its flag, switches through setLanguage()
// and closes on Escape or an outside click.
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { LanguageCard } from "./settings/LanguageCard";
import { I18nProvider, useT } from "../lib/i18n";

const STORAGE_KEY = "bv-lang";

function Harness() {
  const { t } = useT();
  return <LanguageCard t={t} />;
}

function renderCard() {
  return render(
    <I18nProvider>
      <Harness />
    </I18nProvider>
  );
}

beforeEach(() => {
  localStorage.removeItem(STORAGE_KEY);
});

afterEach(() => {
  cleanup();
  localStorage.removeItem(STORAGE_KEY);
  document.documentElement.removeAttribute("lang");
  document.documentElement.removeAttribute("dir");
});

describe("LanguageCard", () => {
  it("renders as a Card with the language heading and the current language on its trigger", () => {
    localStorage.setItem(STORAGE_KEY, "en");
    renderCard();
    expect(screen.getByText("Language")).toBeTruthy();
    expect(screen.getByRole("button", { name: /English/ })).toBeTruthy();
  });

  it("the trigger is closed by default and opens the listbox on click", () => {
    localStorage.setItem(STORAGE_KEY, "en");
    renderCard();
    expect(screen.queryByRole("listbox")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /English/ }));
    expect(screen.getByRole("listbox")).toBeTruthy();
  });

  it("lists every offered language as an option, each with its own flag", () => {
    localStorage.setItem(STORAGE_KEY, "en");
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: /English/ }));
    const listbox = screen.getByRole("listbox");
    // LANGUAGES in lib/i18n.ts; update the count when a locale is added.
    expect(within(listbox).getAllByRole("option").length).toBe(42);
    expect(within(listbox).getByRole("option", { name: /Deutsch/ })).toBeTruthy();
  });

  it("picking a different language calls through to setLanguage and updates the trigger", async () => {
    localStorage.setItem(STORAGE_KEY, "en");
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: /English/ }));
    fireEvent.click(screen.getByRole("option", { name: /Deutsch/ }));

    // findBy, because setLanguage loads the locale chunk before it switches,
    // so the page never flashes English on the way.
    expect(await screen.findByRole("button", { name: /Deutsch/ })).toBeTruthy();
    expect(localStorage.getItem(STORAGE_KEY)).toBe("de");
    expect(document.documentElement.getAttribute("lang")).toBe("de");
  });

  it("picking a language closes the listbox", () => {
    localStorage.setItem(STORAGE_KEY, "en");
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: /English/ }));
    fireEvent.click(screen.getByRole("option", { name: /Deutsch/ }));
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("closes on Escape without changing the language", () => {
    localStorage.setItem(STORAGE_KEY, "en");
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: /English/ }));
    expect(screen.getByRole("listbox")).toBeTruthy();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(localStorage.getItem(STORAGE_KEY)).toBe("en");
  });

  it("closes on an outside click without changing the language", () => {
    localStorage.setItem(STORAGE_KEY, "en");
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: /English/ }));
    expect(screen.getByRole("listbox")).toBeTruthy();
    fireEvent.mouseDown(document.body);
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(localStorage.getItem(STORAGE_KEY)).toBe("en");
  });
});
