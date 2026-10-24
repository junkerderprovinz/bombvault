// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { en, I18nProvider } from "../lib/i18n";
import { ExcludesEverythingNote } from "./ExcludesEverythingNote";

beforeEach(() => localStorage.setItem("bv-lang", "en"));

afterEach(() => {
  cleanup();
  localStorage.removeItem("bv-lang");
});

describe("folder exclude field", () => {
  it("warns about each line the backup ignores because it would exclude everything", () => {
    render(
      <I18nProvider>
        <ExcludesEverythingNote text={"*.tmp\n /\n**\ncache/"} />
      </I18nProvider>,
    );
    const warnings = screen.getAllByText(en["excludes.excludesEverything"], { exact: false });
    expect(warnings).toHaveLength(2);
    expect(screen.getByText("/")).toBeTruthy();
    expect(screen.getByText("**")).toBeTruthy();
    expect(screen.queryByText("*.tmp")).toBeNull();
  });

  it("stays away when no line would exclude everything", () => {
    const { container } = render(
      <I18nProvider>
        <ExcludesEverythingNote text={"*.tmp\ncache/"} />
      </I18nProvider>,
    );
    expect(container.textContent).toBe("");
  });
});
