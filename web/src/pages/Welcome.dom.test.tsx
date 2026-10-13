// @vitest-environment jsdom
// The welcome page: what it tells a new install, which of the three steps it
// marks, where its two buttons lead, and which installs it passes on to the
// overview.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { Route, Routes, useLocation } from "react-router-dom";

import { en } from "../lib/i18n";
import { FakeEventSource, installMatchMedia, item, location, resetServer, server } from "./overview/overview.testsupport";
import { inApp } from "./overview/overviewApp.testsupport";
import { Welcome } from "./Welcome";

vi.mock("../lib/api", async (importOriginal) => {
  const { stagedApi } = await import("./overview/overview.testsupport");
  return stagedApi(await importOriginal<typeof import("../lib/api")>());
});

const host = vi.hoisted(() => ({ platform: "unraid" }));
vi.mock("../lib/usePlatform", () => ({ usePlatform: () => host.platform }));

/** A container that was found, is in no schedule and has never been backed up. */
const found = (key: string, over: Parameters<typeof item>[0] = {}) =>
  item({
    key,
    name: key,
    included: false,
    lastBackup: 0,
    lastRunStatus: "",
    effectiveSchedule: { kind: "none", spec: "", reason: "schedule-off" },
    ...over,
  });

function Where() {
  return <p data-testid="where">{useLocation().pathname}</p>;
}

async function open() {
  render(
    inApp(
      <>
        <Routes>
          <Route path="/welcome" element={<Welcome />} />
          <Route path="*" element={null} />
        </Routes>
        <Where />
      </>,
      "/welcome"
    )
  );
  await act(async () => {});
}

const where = () => screen.getByTestId("where").textContent;
const step = (name: string) => document.querySelector<HTMLElement>(`[data-step='${name}']`)!;
const states = () => ["sections", "location", "backup"].map((name) => step(name).getAttribute("data-state"));

beforeEach(() => {
  resetServer();
  installMatchMedia();
  vi.stubGlobal("EventSource", FakeEventSource);
  window.localStorage.clear();
  host.platform = "unraid";
  server.items = [found("plex"), found("gitea"), found("bombvault", { self: true }), found("win11", { kind: "vm", kindDisabled: true })];
  server.locations = [location()];
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("the welcome page of a new install", () => {
  it("greets, says what the app does and what it has found", async () => {
    await open();

    expect(where()).toBe("/welcome");
    expect(screen.getByRole("heading", { level: 1, name: en["welcome.title"] })).toBeTruthy();
    expect(screen.getByText(en["welcome.leadUnraid"])).toBeTruthy();
    expect(screen.getByTestId("welcome-found").textContent).toBe("BombVault found 2 containers, 1 VM, and the flash drive.");
  });

  it("shows its greeting as a paragraph and keeps the one heading for screen readers", async () => {
    await open();

    const shown = screen.getByText(en["welcome.title"], { selector: "p" });
    expect(shown.getAttribute("aria-hidden")).toBe("true");
    expect(document.querySelectorAll("h1")).toHaveLength(1);
  });

  it("names no product and no flash drive on another host", async () => {
    host.platform = "generic";
    await open();

    expect(screen.getByText(en["welcome.lead"])).toBeTruthy();
    expect(screen.getByTestId("welcome-found").textContent).toBe("BombVault found 2 containers and 1 VM.");
  });

  it("says nothing about what was found where the server reports nothing", async () => {
    host.platform = "generic";
    server.items = [];
    await open();

    expect(where()).toBe("/welcome");
    expect(screen.queryByTestId("welcome-found")).toBeNull();
  });
});

describe("the three steps on the welcome page", () => {
  it("start with the sections as the next one", async () => {
    await open();

    expect(states()).toEqual(["next", "open", "open"]);
    expect(step("sections").getAttribute("aria-current")).toBe("step");
    expect(within(step("sections")).getByText(en["firstSteps.sectionsSub"])).toBeTruthy();
    expect(within(step("location")).getByText(en["firstSteps.locationSub"])).toBeTruthy();
  });

  it("tick off the sections once they were chosen on the overview", async () => {
    window.localStorage.setItem("bombvault.firstSteps.sectionsChosen", "1");
    await open();

    expect(states()).toEqual(["done", "next", "open"]);
    expect(step("sections").textContent).toContain(en["common.done"]);
  });

  it("tick off the storage location once one was added to the folder the install came with", async () => {
    window.localStorage.setItem("bombvault.firstSteps.sectionsChosen", "1");
    server.locations = [location(), location({ id: "destination:1", object: "destination", name: "Storage Box" })];
    await open();

    expect(states()).toEqual(["done", "done", "next"]);
  });

  it("promise backups that go on by themselves only where a schedule reaches an entry", async () => {
    await open();
    expect(within(step("backup")).queryByText(en["firstSteps.backupSub"])).toBeNull();

    cleanup();
    server.items = [found("plex", { included: true, effectiveSchedule: { kind: "domain", spec: "daily 02:00" } })];
    await open();
    expect(within(step("backup")).getByText(en["firstSteps.backupSub"])).toBeTruthy();
  });
});

describe("the two buttons of the welcome page", () => {
  it("stand at the taller height, the way forward last", async () => {
    await open();

    const restore = screen.getByRole("button", { name: en["welcome.restore"] });
    const start = screen.getByRole("button", { name: en["welcome.start"] });
    expect(restore.className).toContain("glim-btn-key");
    expect(start.className).toContain("glim-btn-key");
    expect(start.className).toContain("bg-accent");
    expect(restore.className).not.toContain("bg-accent");
    expect(restore.compareDocumentPosition(start) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("lead to the Restore page and to the first steps on the overview", async () => {
    await open();
    fireEvent.click(screen.getByRole("button", { name: en["welcome.restore"] }));
    expect(where()).toBe("/restore");

    cleanup();
    await open();
    fireEvent.click(screen.getByRole("button", { name: en["welcome.start"] }));
    expect(where()).toBe("/dashboard");
  });
});

describe("an install the welcome page is not for", () => {
  it("goes on to the overview when an entry has a backup", async () => {
    server.items = [found("gitea"), item()];
    await open();

    expect(where()).toBe("/dashboard");
    expect(screen.queryByText(en["welcome.leadUnraid"])).toBeNull();
  });

  it("goes on to the overview when the entries cannot be read", async () => {
    server.items = "refused";
    await open();

    expect(where()).toBe("/dashboard");
  });

  it("shows nothing and stays while the entries are still being read", async () => {
    server.items = "pending";
    await open();

    expect(where()).toBe("/welcome");
    expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
  });
});
