// @vitest-environment jsdom
// The card of the other servers, which the overview shows inside the Android
// app: the app names them, says how each answers and opens the one tapped.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import { en } from "../lib/i18n";
import { Dashboard } from "./Dashboard";
import { FakeEventSource, domain, installMatchMedia, item, resetServer, server, setDesktop } from "./overview/overview.testsupport";
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
const rows = () => screen.getAllByTestId("instance-row");

describe("the card of the other servers", () => {
  it("is not there in a browser", async () => {
    await renderPage();

    expect(shownCards()).not.toContain("instances");
    expect(screen.queryByTestId("instance-row")).toBeNull();
  });

  it("is not there while the app has not answered, as an app too old for the question never does", async () => {
    const app = fakeApp();
    await renderPage();

    expect(app.sent).toEqual([{ op: "servers" }]);
    expect(shownCards()).not.toContain("instances");
  });

  it("is not there when the app knows no other server", async () => {
    const app = fakeApp();
    await renderPage();
    await app.answer({ op: "servers", servers: [] });

    expect(shownCards()).not.toContain("instances");
  });

  it("stands right under what needs the person, with each server the app names and how it answers", async () => {
    server.domains = [domain()];
    server.items = [
      item(),
      item({ key: "gitea", included: false, lastBackup: 0, lastRunStatus: "", effectiveSchedule: { kind: "none", spec: "", reason: "excluded" } }),
    ];
    const app = fakeApp();
    await renderPage();
    await app.answer({ op: "servers", servers: [B, C] });

    expect(shownCards().slice(0, 3)).toEqual(["needs", "instances", "nextRun"]);
    expect(rows().map((row) => within(row).getByText(/BV-RC/).textContent)).toEqual(["BV-RC-B", "BV-RC-C"]);
    expect(within(rows()[0]).queryByText(en["instances.connected"])).toBeNull();

    await app.answer({ op: "reach", id: "b", status: 200 });
    await app.answer({ op: "reach", id: "c", status: 0 });

    expect(within(rows()[0]).getByText(en["instances.connected"])).toBeTruthy();
    expect(within(rows()[1]).getByText(en["instances.notConnected"])).toBeTruthy();
  });

  it("offers only the servers the app names, which leave out the one on screen", async () => {
    const app = fakeApp();
    await renderPage();
    await app.answer({ op: "servers", servers: [C] });

    expect(rows()).toHaveLength(1);
    expect(screen.queryByText("BV-RC-B")).toBeNull();
  });

  it("has the app open the server that is tapped, by its id", async () => {
    const app = fakeApp();
    await renderPage();
    await app.answer({ op: "servers", servers: [B, C] });

    fireEvent.click(rows()[1]);

    expect(app.sent.at(-1)).toEqual({ op: "open", id: "c" });
  });

  it("keeps a server's answer when the app names the servers again", async () => {
    const app = fakeApp();
    await renderPage();
    await app.answer({ op: "servers", servers: [B] });
    await app.answer({ op: "reach", id: "b", status: 401 });
    await app.answer({ op: "servers", servers: [B, C] });

    expect(within(rows()[0]).getByText(en["launcher.signIn"])).toBeTruthy();
  });
});
