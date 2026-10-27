// @vitest-environment jsdom
// LanguagePicker, the interface language on the Look page: it opens and
// closes, lists every offered language and switches at once.
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { settle } from "../components/selectField.testsupport";
import { LanguagePicker } from "./settings/LanguageCard";
import { I18nProvider, useT } from "../lib/i18n";

const STORAGE_KEY = "bv-lang";

function Harness() {
  const { t } = useT();
  return <LanguagePicker t={t} />;
}

async function renderCard() {
  render(
    <I18nProvider>
      <Harness />
    </I18nProvider>
  );
  await settle();
}

function trigger(): HTMLElement {
  return screen.getByRole("combobox", { name: "Language" });
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

describe("LanguagePicker", () => {
  it("shows the current language on its trigger", async () => {
    localStorage.setItem(STORAGE_KEY, "en");
    await renderCard();
    expect(trigger().querySelector('[aria-hidden="false"]')?.textContent).toBe("English");
  });

  it("stands at a field's height, like the buttons beside fields", async () => {
    localStorage.setItem(STORAGE_KEY, "en");
    await renderCard();
    expect(trigger().className).toContain("h-[var(--btn-h)]");
  });

  it("the trigger is closed by default and opens the listbox on click", async () => {
    localStorage.setItem(STORAGE_KEY, "en");
    await renderCard();
    expect(screen.queryByRole("listbox")).toBeNull();
    await settle(() => fireEvent.click(trigger()));
    expect(screen.getByRole("listbox")).toBeTruthy();
  });

  it("lists every offered language as an option, each with its own flag", async () => {
    localStorage.setItem(STORAGE_KEY, "en");
    await renderCard();
    await settle(() => fireEvent.click(trigger()));
    const listbox = screen.getByRole("listbox");
    // LANGUAGES in lib/i18n.ts; update the count when a locale is added.
    const options = within(listbox).getAllByRole("option");
    expect(options.length).toBe(42);
    expect(options.every((o) => o.querySelector(".fi"))).toBe(true);
    expect(within(listbox).getByRole("option", { name: /Deutsch/ })).toBeTruthy();
  });

  it("picking a different language calls through to setLanguage and updates the trigger", async () => {
    localStorage.setItem(STORAGE_KEY, "en");
    await renderCard();
    await settle(() => fireEvent.click(trigger()));
    await settle(() => fireEvent.click(screen.getByRole("option", { name: /Deutsch/ })));

    // findBy, because setLanguage loads the locale chunk before it switches,
    // so the page never flashes English on the way.
    expect(await screen.findByRole("combobox", { name: "Sprache" })).toBeTruthy();
    expect(localStorage.getItem(STORAGE_KEY)).toBe("de");
    expect(document.documentElement.getAttribute("lang")).toBe("de");
  });

  it("picking a language closes the listbox", async () => {
    localStorage.setItem(STORAGE_KEY, "en");
    await renderCard();
    await settle(() => fireEvent.click(trigger()));
    await settle(() => fireEvent.click(screen.getByRole("option", { name: /Deutsch/ })));
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("closes on Escape without changing the language", async () => {
    localStorage.setItem(STORAGE_KEY, "en");
    await renderCard();
    await settle(() => fireEvent.click(trigger()));
    expect(screen.getByRole("listbox")).toBeTruthy();
    await settle(() => fireEvent.keyDown(screen.getByRole("listbox"), { key: "Escape" }));
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(localStorage.getItem(STORAGE_KEY)).toBe("en");
  });

  it("closes on an outside click without changing the language", async () => {
    localStorage.setItem(STORAGE_KEY, "en");
    await renderCard();
    await settle(() => fireEvent.click(trigger()));
    expect(screen.getByRole("listbox")).toBeTruthy();
    // Zag starts listening for the outside press a task after the list
    // opens and answers it on the next frame, so the press repeats until it
    // lands.
    await waitFor(() => {
      fireEvent.pointerDown(document.body);
      expect(screen.queryByRole("listbox")).toBeNull();
    });
    expect(localStorage.getItem(STORAGE_KEY)).toBe("en");
  });
});
