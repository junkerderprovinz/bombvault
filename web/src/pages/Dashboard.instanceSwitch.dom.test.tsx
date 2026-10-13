// @vitest-environment jsdom
// The name at the top of the overview inside the Android app: the server's
// own, and once the app names other servers, the way to switch to one.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import { en } from "../lib/i18n";
import { Dashboard } from "./Dashboard";
import { FakeEventSource, installMatchMedia, resetServer, server, setDesktop } from "./overview/overview.testsupport";
import { inApp } from "./overview/overviewApp.testsupport";

vi.mock("../lib/api", async (importOriginal) => {
  const { stagedApi } = await import("./overview/overview.testsupport");
  return stagedApi(await importOriginal<typeof import("../lib/api")>());
});

/** The app's message port as the page sees it, with what the page sent. */
function fakeApp() {
  const listeners = new Set<(e: { data: string }) => void>();
  const sent: unknown[] = [];
  const port = {
    postMessage: (data: string) => sent.push(JSON.parse(data)),
    addEventListener: (_type: string, fn: (e: { data: string }) => void) => listeners.add(fn),
    removeEventListener: (_type: string, fn: (e: { data: string }) => void) => listeners.delete(fn),
  };
  vi.stubGlobal("bombvaultApp", port);
  return {
    sent,
    answer: (message: unknown) =>
      act(async () => {
        for (const fn of listeners) fn({ data: JSON.stringify(message) });
      }),
  };
}

const B = { id: "b", name: "BV-RC-B" };
const C = { id: "c", name: "BV-RC-C" };

beforeEach(() => {
  resetServer();
  server.settings = { instanceName: "BV-RC" };
  installMatchMedia();
  setDesktop(false);
  vi.stubGlobal("EventSource", FakeEventSource);
  window.localStorage.clear();
  window.history.replaceState(null, "", "/dashboard");
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

async function renderPage() {
  render(inApp(<Dashboard />));
  await act(async () => {});
}

const shownCards = () => [...document.querySelectorAll("[data-grid-block]")].map((el) => el.getAttribute("data-grid-block"));
const switcher = () => screen.queryByRole("button", { name: `BV-RC, ${en["instances.title"]}` });

describe("the top of the overview in a browser", () => {
  it("names the product and offers no switch", async () => {
    await renderPage();

    expect(screen.getByText("BombVault", { selector: "span" })).toBeTruthy();
    expect(screen.queryByText("BV-RC")).toBeNull();
    expect(screen.queryByTestId("instance-chevron")).toBeNull();
  });
});

describe("the top of the overview in the app", () => {
  it("names the server before the app has answered, without a switch yet", async () => {
    const app = fakeApp();
    await renderPage();

    expect(app.sent).toEqual([{ op: "servers" }]);
    expect(screen.getByText("BV-RC")).toBeTruthy();
    expect(switcher()).toBeNull();
  });

  it("falls back to the product's name for a server without a name", async () => {
    server.settings = { instanceName: "  " };
    fakeApp();
    await renderPage();

    expect(screen.getByText("BombVault", { selector: "span" })).toBeTruthy();
  });

  it("offers no switch when the app knows no other server", async () => {
    const app = fakeApp();
    await renderPage();
    await app.answer({ op: "servers", servers: [] });

    expect(screen.getByText("BV-RC")).toBeTruthy();
    expect(screen.queryByTestId("instance-chevron")).toBeNull();
  });

  it("turns the name into a switch with a chevron once the app names other servers", async () => {
    const app = fakeApp();
    await renderPage();
    await app.answer({ op: "servers", servers: [B, C] });

    const name = switcher()!;
    expect(within(name).getByText("BV-RC")).toBeTruthy();
    expect(within(name).getByTestId("instance-chevron")).toBeTruthy();
    expect(name.getAttribute("data-new")).toBe("overview-instances");
  });

  it("lists this server first and not as a choice, then the others with how each answers", async () => {
    const app = fakeApp();
    await renderPage();
    await app.answer({ op: "servers", servers: [B, C] });
    await app.answer({ op: "reach", id: "b", status: 200 });
    await app.answer({ op: "reach", id: "c", status: 0 });

    fireEvent.click(switcher()!);
    const sheet = screen.getByRole("dialog");

    const current = within(sheet).getByTestId("instance-current");
    expect(current.getAttribute("aria-current")).toBe("true");
    expect(within(current).getByText("BV-RC")).toBeTruthy();
    expect(within(current).getByText(en["instances.thisInstance"])).toBeTruthy();
    expect(within(current).queryByRole("button")).toBeNull();

    const rows = within(sheet).getAllByTestId("instance-row");
    expect(rows.map((row) => within(row).getByText(/BV-RC/).textContent)).toEqual(["BV-RC-B", "BV-RC-C"]);
    expect(within(rows[0]).getByText(en["instances.connected"])).toBeTruthy();
    expect(within(rows[1]).getByText(en["instances.notConnected"])).toBeTruthy();
  });

  it("has the app open the server that is tapped, by its id", async () => {
    const app = fakeApp();
    await renderPage();
    await app.answer({ op: "servers", servers: [B, C] });

    fireEvent.click(switcher()!);
    fireEvent.click(within(screen.getByRole("dialog")).getAllByTestId("instance-row")[1]);

    expect(app.sent.at(-1)).toEqual({ op: "open", id: "c" });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("keeps a server's answer when the app names the servers again", async () => {
    const app = fakeApp();
    await renderPage();
    await app.answer({ op: "servers", servers: [B] });
    await app.answer({ op: "reach", id: "b", status: 401 });
    await app.answer({ op: "servers", servers: [B, C] });

    fireEvent.click(switcher()!);

    expect(within(screen.getAllByTestId("instance-row")[0]).getByText(en["launcher.signIn"])).toBeTruthy();
  });

  it("leaves the servers off the overview's cards", async () => {
    const app = fakeApp();
    await renderPage();
    await app.answer({ op: "servers", servers: [B, C] });

    expect(shownCards()).not.toContain("instances");
    expect(shownCards()[0]).toBe("nextRun");
  });
});
