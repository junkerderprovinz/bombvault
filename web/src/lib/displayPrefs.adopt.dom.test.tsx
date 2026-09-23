// @vitest-environment jsdom
// The language and the advanced view are React state read once when their
// provider mounts, so both providers have to follow values adopted from the
// server in place, without a reload.
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { ADOPTED_EVENT } from "./displayPrefs";
import { AdvancedProvider, useAdvanced } from "./advanced";
import { I18nProvider, useT } from "./i18n";

function AdvancedProbe() {
  const { advanced } = useAdvanced();
  return <span data-testid="mode">{advanced ? "advanced" : "simple"}</span>;
}

function LangProbe() {
  const { lang } = useT();
  return <span data-testid="lang">{lang}</span>;
}

beforeEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute("lang");
});
afterEach(cleanup);

describe("adopting the server's look in a page that already booted", () => {
  it("switches the advanced view without a reload", () => {
    // An empty browser mounts the simple view.
    render(
      <AdvancedProvider>
        <AdvancedProbe />
      </AdvancedProvider>
    );
    expect(screen.getByTestId("mode").textContent).toBe("simple");

    // What sync() does a moment later: writes the server's values, then says so.
    localStorage.setItem("bombvault.advanced", "1");
    act(() => {
      window.dispatchEvent(new Event(ADOPTED_EVENT));
    });

    expect(
      screen.getByTestId("mode").textContent,
      "the advanced view was read once at mount and has to be read again"
    ).toBe("advanced");
  });

  it("switches the language without a reload", async () => {
    render(
      <I18nProvider>
        <LangProbe />
      </I18nProvider>
    );
    expect(screen.getByTestId("lang").textContent).toBe("en");

    localStorage.setItem("bv-lang", "de");
    await act(async () => {
      window.dispatchEvent(new Event(ADOPTED_EVENT));
    });

    expect(screen.getByTestId("lang").textContent).toBe("de");
    expect(
      document.documentElement.getAttribute("lang"),
      "the document element carries the language for CSS and screen readers"
    ).toBe("de");
  });

  it("does not echo an adopted language back to the server", async () => {
    // The value came from the server, so saving it would be a pointless write
    // on every boot.
    const calls: string[] = [];
    const realFetch = globalThis.fetch;
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      calls.push(`${init?.method ?? "GET"} ${String(input)}`);
      return Promise.resolve({ ok: true, json: async () => ({}) } as Response);
    }) as typeof fetch;

    try {
      render(
        <I18nProvider>
          <LangProbe />
        </I18nProvider>
      );
      localStorage.setItem("bv-lang", "fr");
      await act(async () => {
        window.dispatchEvent(new Event(ADOPTED_EVENT));
      });
      expect(calls.filter((c) => c.includes("display-prefs"))).toEqual([]);
    } finally {
      globalThis.fetch = realFetch;
    }
  });
});
