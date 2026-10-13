// @vitest-environment jsdom
// The top of the overview inside the Android app: the server's own name, and
// an arrow back to the app's list of servers.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";

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
  const sent: unknown[] = [];
  vi.stubGlobal("bombvaultApp", { postMessage: (data: string) => sent.push(JSON.parse(data)) });
  return sent;
}

beforeEach(() => {
  resetServer();
  server.settings = { instanceName: "BV-RC" };
  installMatchMedia();
  setDesktop(false);
  vi.stubGlobal("EventSource", FakeEventSource);
  window.localStorage.clear();
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

const backToApp = () => screen.queryByRole("button", { name: en["overview.backToApp"] });

describe("the top of the overview in a browser", () => {
  it("names the product and offers no way back to an app", async () => {
    await renderPage();

    expect(screen.getByText("BombVault", { selector: "span" })).toBeTruthy();
    expect(screen.queryByText("BV-RC")).toBeNull();
    expect(backToApp()).toBeNull();
  });
});

describe("the top of the overview in the app", () => {
  it("names the server as a plain label behind an arrow back to the app", async () => {
    fakeApp();
    await renderPage();

    const name = screen.getByText("BV-RC");
    expect(name.closest("button")).toBeNull();
    expect(backToApp()).toBeTruthy();
  });

  it("falls back to the product's name for a server without a name", async () => {
    server.settings = { instanceName: "  " };
    fakeApp();
    await renderPage();

    expect(screen.getByText("BombVault", { selector: "span" })).toBeTruthy();
  });

  it("asks the app for its own overview when the arrow is tapped", async () => {
    const sent = fakeApp();
    await renderPage();
    expect(sent).toEqual([]);

    fireEvent.click(backToApp()!);

    expect(sent).toEqual([{ op: "home" }]);
  });

  it("marks the arrow as what is new on the overview", async () => {
    fakeApp();
    await renderPage();

    expect(backToApp()!.closest("[data-new]")?.getAttribute("data-new")).toBe("overview-instances");
  });

  it("opens no list of servers", async () => {
    fakeApp();
    await renderPage();

    fireEvent.click(backToApp()!);

    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
