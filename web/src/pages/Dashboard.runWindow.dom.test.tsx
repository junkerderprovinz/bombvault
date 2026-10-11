// @vitest-environment jsdom
// The run window of the overview and the two things that write to it: a row
// of the history, which opens one run, and the watch of a pass started with
// "Back up everything now", which opens the pass's run and keeps it fresh.
// They must not fight over the window:
//
//   1. While the watch is alive every poll reports its run again. A window
//      the person has since pointed at another run stays on that run.
//   2. A run opened from the history follows the page's polled list to its
//      end without being opened again.
//   3. The window and the watch both survive the change between the phone
//      and the desktop layout, as a phone turned to landscape makes it.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import { en } from "../lib/i18n";
import { Dashboard } from "./Dashboard";
import {
  FakeEventSource,
  installMatchMedia,
  resetServer,
  run,
  server,
  setDesktop,
} from "./overview/overview.testsupport";
import { inApp } from "./overview/overviewApp.testsupport";

vi.mock("../lib/api", async (importOriginal) => {
  const { stagedApi } = await import("./overview/overview.testsupport");
  return stagedApi(await importOriginal<typeof import("../lib/api")>());
});

const plex = run({ id: "run-plex", target: "plex" });
const pass = run({
  id: "run-everything",
  target: "all-domains",
  targetId: "everything",
  domain: "everything",
  status: "running",
  finishedAt: null,
  snapshotId: "",
  bytes: 0,
});
const START = en["backups.backUpEverything"];
const RUNNING = en["run.statusRunning"];

beforeEach(() => {
  vi.useFakeTimers();
  resetServer();
  installMatchMedia();
  vi.stubGlobal("EventSource", FakeEventSource);
  window.localStorage.clear();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

async function renderPage() {
  render(inApp(<Dashboard />));
  await act(async () => {});
}

const tick = (ms: number) =>
  act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });

function historyRow(target: string): HTMLElement {
  const card = document.querySelector<HTMLElement>("[data-grid-block='history']")!;
  return within(card).getByRole("button", { name: target });
}

const started = () => server.calls.filter((c) => c.name === "backupEverythingNow").length;

/** Starts a pass and lets the watch find its run. */
async function startPass() {
  fireEvent.click(screen.getByRole("button", { name: START }));
  await tick(0);
  server.runs = [pass, plex];
  // The watch polls for the first time 600ms after the start was accepted.
  await tick(700);
}

describe("the run window of the overview", () => {
  it("starts a pass at once on a desktop and opens its run", async () => {
    server.runs = [plex];
    await renderPage();

    await startPass();

    expect(started()).toBe(1);
    // The window is named after its run.
    const window = screen.getByRole("dialog", { name: /full backup/i });
    expect(within(window).getByText(RUNNING)).toBeTruthy();
    // The main action leads to the run while it is under way.
    expect(screen.queryByRole("button", { name: START })).toBeNull();
    expect(screen.getByRole("button", { name: en["overview.head.backingUp"] })).toBeTruthy();
  });

  it("stays on the run the person opened while the watch keeps reporting the pass", async () => {
    server.runs = [plex];
    await renderPage();
    await startPass();

    fireEvent.click(historyRow("plex"));
    expect(screen.getByRole("dialog", { name: /plex/i })).toBeTruthy();

    await tick(2100);
    await tick(2100);

    expect(screen.getByRole("dialog", { name: /plex/i })).toBeTruthy();
    expect(screen.queryByRole("dialog", { name: /full backup/i })).toBeNull();
  });

  it("stays closed once the person closed it, until a row opens it again", async () => {
    server.runs = [plex];
    await renderPage();
    await startPass();

    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
    await tick(2100);
    expect(screen.queryByRole("dialog")).toBeNull();

    fireEvent.click(historyRow("plex"));
    expect(screen.getByRole("dialog")).toBeTruthy();
  });

  it("follows a run opened from the history to its end", async () => {
    const jellyfin = run({ id: "run-jellyfin", target: "jellyfin", status: "running", finishedAt: null });
    server.runs = [jellyfin];
    await renderPage();

    fireEvent.click(historyRow("jellyfin"));
    expect(within(screen.getByRole("dialog")).getByText(RUNNING)).toBeTruthy();

    server.runs = [{ ...jellyfin, status: "success", finishedAt: jellyfin.startedAt + 30 }];
    await tick(10_500);

    const window = within(screen.getByRole("dialog"));
    expect(window.getByText(en["spike.ok"])).toBeTruthy();
    expect(window.queryByText(RUNNING)).toBeNull();
  });

  it("shows the end of a pass at the watch's tick, before the slower list has it", async () => {
    server.runs = [plex];
    await renderPage();
    await startPass();
    // The page's own poll picks the running record up, so the list the window
    // also reads from holds the stale one.
    await tick(9_500);
    expect(within(screen.getByRole("dialog")).getByText(RUNNING)).toBeTruthy();

    server.runs = [{ ...pass, status: "success", finishedAt: pass.startedAt + 45 }, plex];
    await tick(2_000);

    const window = within(screen.getByRole("dialog"));
    expect(window.getByText(en["spike.ok"])).toBeTruthy();
    expect(window.queryByText(RUNNING)).toBeNull();
  });

  it("opens the run under way from the status head", async () => {
    server.runs = [run({ id: "run-live", target: "immich", status: "running", finishedAt: null }), plex];
    await renderPage();
    act(() => FakeEventSource.last!.send({ key: "container:immich", phase: "backup", percent: 40, active: true }));

    fireEvent.click(screen.getByRole("button", { name: en["overview.head.viewRun"] }));

    expect(screen.getByRole("dialog", { name: /immich/ })).toBeTruthy();
  });
});

describe("the run window on a phone", () => {
  beforeEach(() => setDesktop(false));

  it("asks before it starts a pass, and a no leaves the server alone", async () => {
    server.runs = [plex];
    await renderPage();

    fireEvent.click(screen.getByRole("button", { name: START }));
    await tick(0);
    const question = screen.getByText(en["home.newBackupConfirm"]).closest("[role='dialog'],[role='alertdialog']") as HTMLElement;
    fireEvent.click(within(question).getByRole("button", { name: en["common.cancel"] }));
    await tick(0);

    expect(started()).toBe(0);
    expect(screen.getByRole("button", { name: START })).toBeTruthy();
  });

  it("starts the pass on a yes and opens its run as a sheet", async () => {
    server.runs = [plex];
    await renderPage();

    fireEvent.click(screen.getByRole("button", { name: START }));
    await tick(0);
    const question = screen.getByText(en["home.newBackupConfirm"]).closest("[role='dialog'],[role='alertdialog']") as HTMLElement;
    fireEvent.click(within(question).getByRole("button", { name: en["settings.everythingTitle"] }));
    await tick(0);
    server.runs = [pass, plex];
    await tick(700);

    expect(started()).toBe(1);
    expect(within(screen.getByRole("dialog")).getByText(RUNNING)).toBeTruthy();
  });

  it("keeps the window and the watch when the phone is turned to landscape and back", async () => {
    server.runs = [plex];
    await renderPage();
    fireEvent.click(screen.getByRole("button", { name: START }));
    await tick(0);
    const question = screen.getByText(en["home.newBackupConfirm"]).closest("[role='dialog'],[role='alertdialog']") as HTMLElement;
    fireEvent.click(within(question).getByRole("button", { name: en["settings.everythingTitle"] }));
    await tick(0);
    server.runs = [pass, plex];
    await tick(700);

    act(() => setDesktop(true));
    expect(within(screen.getByRole("dialog")).getByText(RUNNING)).toBeTruthy();
    act(() => setDesktop(false));
    expect(within(screen.getByRole("dialog")).getByText(RUNNING)).toBeTruthy();

    // The watch is still the one that brings the end.
    server.runs = [{ ...pass, status: "success", finishedAt: pass.startedAt + 45 }, plex];
    await tick(2_000);
    expect(within(screen.getByRole("dialog")).queryByText(RUNNING)).toBeNull();
    expect(screen.getByRole("button", { name: START })).toBeTruthy();
  });

  it("puts the sheet's action rows on the colour engine", async () => {
    server.runs = [plex];
    await renderPage();

    fireEvent.click(historyRow("plex"));

    const sheet = within(screen.getByRole("dialog"));
    for (const key of ["recovery.foreignStepBrowse", "integrity.verify", "snapshots.restore"] as const) {
      const row = sheet.getByRole("button", { name: en[key] });
      expect(row.className, key).toContain("glim-hue");
      expect(row.style.getPropertyValue("--item-hue"), key).not.toBe("");
    }
  });
});
