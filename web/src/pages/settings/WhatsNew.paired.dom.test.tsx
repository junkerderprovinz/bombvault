// @vitest-environment jsdom
// A paired phone's session cannot open every page, so What's new leaves out
// the changes on those pages: no line in the window and no place in the count.
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  openedAsPairedPhone: () => true,
}));

const { I18nProvider } = await import("../../lib/i18n");
const { CHANGES, RELEASES } = await import("../../lib/whatsNew");
const { beginWhatsNew, openWhatsNew, useWhatsNew } = await import("../../lib/whatsNewStore");
const { WhatsNewHost } = await import("./WhatsNew");

const CLOSED = ["instance-address", "node-copy-onward"];

function Count() {
  return <output data-testid="count">{useWhatsNew().unseen.length}</output>;
}

beforeEach(() => {
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  beginWhatsNew("v9.9.1");
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response(JSON.stringify({ ok: true, versions: RELEASES.map((r) => `v${r.version}`), body: "" }), { status: 200 })),
  );
});

afterEach(() => {
  act(() => openWhatsNew(false));
  cleanup();
  vi.unstubAllGlobals();
});

describe("What's new on a paired phone", () => {
  it("neither lists nor counts the changes on pages the session cannot open", async () => {
    await act(async () => {
      render(
        <I18nProvider>
          <MemoryRouter>
            <Count />
            <WhatsNewHost version={`v${RELEASES[0].version}`} />
          </MemoryRouter>
        </I18nProvider>,
      );
      openWhatsNew(true);
    });
    expect(screen.getByRole("dialog")).toBeTruthy();
    for (const id of CLOSED) expect(document.querySelector(`[data-change="${id}"]`)).toBeNull();
    expect(document.querySelectorAll("[data-change]")).toHaveLength(CHANGES.length - CLOSED.length);
    expect(screen.getByTestId("count").textContent).toBe(String(CHANGES.length - CLOSED.length));
  });
});
