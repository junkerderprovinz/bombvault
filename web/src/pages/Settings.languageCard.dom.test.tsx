// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// LanguageCard (GlimStone follow-up pass, live-review point 9) — the app's
// UI-language switcher, moved out of Sidebar.tsx's footer into its own Card
// in Settings' General tab. This covers the picker's OWN behaviour in its
// new home: it opens/closes, lists every offered language with its flag,
// switching actually calls setLanguage() (which persists to localStorage and
// re-renders the whole tree per lib/i18n.ts), and Escape/outside-click still
// close it — the exact same interaction contract the sidebar version had,
// just relocated. Sidebar.language.dom.test.tsx (sibling file) is the other
// half: proving the OLD location no longer renders it.
//
// jsdom opted in explicitly (real DOM/click/keyboard behaviour needed) — see
// Selector.dom.test.tsx's own header comment for this repo's naming
// convention for the jsdom-opted-in exception.
// ---------------------------------------------------------------------------
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
    // 42 offered locales (lib/i18n.ts's LANGUAGES) — sanity count, kept in
    // sync with LANGUAGES.length whenever a locale is added.
    expect(within(listbox).getAllByRole("option").length).toBe(42);
    expect(within(listbox).getByRole("option", { name: /Deutsch/ })).toBeTruthy();
  });

  it("picking a different language calls through to setLanguage and updates the trigger", async () => {
    localStorage.setItem(STORAGE_KEY, "en");
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: /English/ }));
    fireEvent.click(screen.getByRole("option", { name: /Deutsch/ }));

    // The whole I18nProvider tree re-renders on setLanguage, so the trigger
    // itself now reads the new language, and the choice persisted.
    //
    // `findBy`, not `getBy`, since locales became lazy chunks ([344]):
    // setLanguage now fetches the table BEFORE it moves the state, so that
    // switching shows the new language rather than a beat of English on the
    // way to it. That is one microtask, and the assertion has to allow for
    // it. Waiting here is not papering over a race - the alternative,
    // setting the state first, is the visible flash this ordering avoids.
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
