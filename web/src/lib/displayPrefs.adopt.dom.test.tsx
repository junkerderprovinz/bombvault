// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// The half of #191 that survived the first fix.
//
// The look lives on the server, and a browser whose data was cleared picks it
// up a moment after boot. Six of the eight axes are attributes on the document
// element and can simply be applied again. The other two — the language and the
// advanced view — are React state, read ONCE when their provider mounts, and
// the old code bridged that with a `location.reload()`.
//
// A reload can be suppressed. A session-scoped guard existed to stop a reload
// loop, and it did that by refusing every reload after the first in the same
// tab, which is exactly what a restored tab (or a tab left open while its site
// data was cleared) presents. The values landed in localStorage, the page did
// not reload, and it sat there in English on a white background with the
// correct settings already stored: "all gone!", reported a second time.
//
// These pin the replacement: both providers follow the adopted values in place,
// with no reload anywhere. Each fails without its own listener.
// ---------------------------------------------------------------------------
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
    // Mounted from an empty browser: the simple view, like a cleared Firefox.
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
    // adopt() must not persist: the value came FROM the server, and saving it
    // again would be a write nobody asked for on every boot of every browser.
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
