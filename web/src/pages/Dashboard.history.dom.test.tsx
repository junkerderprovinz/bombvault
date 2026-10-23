// @vitest-environment jsdom
// The history card: every run as a list that opens a run, and the same runs
// as the log. The list may only say "no runs yet" once the read has answered
// and the answer is none. A read still under way and a read that failed each
// say so, because an empty list must not pass for an empty history.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import { en } from "../lib/i18n";
import { Dashboard } from "./Dashboard";
import { FakeEventSource, installMatchMedia, resetServer, run, server } from "./overview/overview.testsupport";
import { inApp } from "./overview/overviewApp.testsupport";

vi.mock("../lib/api", async (importOriginal) => {
  const { stagedApi } = await import("./overview/overview.testsupport");
  return stagedApi(await importOriginal<typeof import("../lib/api")>());
});

beforeEach(() => {
  resetServer();
  installMatchMedia();
  vi.stubGlobal("EventSource", FakeEventSource);
  window.localStorage.clear();
  window.history.replaceState(null, "", "/dashboard");
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

async function renderPage() {
  render(inApp(<Dashboard />));
  await act(async () => {});
}

function historyCard(): HTMLElement {
  return document.querySelector<HTMLElement>("[data-grid-block='history']")!;
}

describe("the runs of the history card", () => {
  it("reads checking while the runs are on their way", async () => {
    server.runs = "pending";
    await renderPage();

    const card = within(historyCard());
    expect(card.getByText(en["dashboard.checking"])).toBeTruthy();
    expect(card.queryByText(en["dashboard.noRuns"])).toBeNull();
  });

  it("reports a failed read instead of an empty history", async () => {
    server.runs = "refused";
    await renderPage();

    const card = within(historyCard());
    expect(card.getByText(en["dashboard.loadRunsFailed"])).toBeTruthy();
    expect(card.queryByText(en["dashboard.noRuns"])).toBeNull();
  });

  it("says no runs yet once the read answered with none", async () => {
    await renderPage();

    expect(within(historyCard()).getByText(en["dashboard.noRuns"])).toBeTruthy();
  });

  it("lists the runs with what each one was and how it ended", async () => {
    server.runs = [
      run({ id: "r2", target: "immich", status: "failed", error: "restic exited 1" }),
      run({ id: "r1", kind: "dbdump" }),
    ];
    await renderPage();

    const card = within(historyCard());
    expect(card.queryByText(en["dashboard.noRuns"])).toBeNull();
    const rows = card.getAllByRole("listitem");
    expect(rows).toHaveLength(2);
    expect(within(rows[0]).getByRole("button", { name: "immich" })).toBeTruthy();
    expect(within(rows[0]).getByText(en["run.statusFailed"])).toBeTruthy();
    expect(within(rows[1]).getByText(en["run.statusSuccess"])).toBeTruthy();
    expect(within(rows[0]).getByText(/restic exited 1/).closest("span[class]")!.className).toContain("text-statusFail");
    expect(within(rows[1]).getByText(en["run.kindDbDump"])).toBeTruthy();
  });

  it("names a failed replica by the sentence of its code", async () => {
    const error = "zfs replica: tank/docs: zfs: zfs-not-found: bash: line 1: [path]: No such file or directory [zfs-not-found]";
    server.runs = [run({ id: "r1", kind: "replica", target: "tank/docs", status: "failed", error })];
    await renderPage();

    const row = within(within(historyCard()).getByRole("listitem"));
    expect(row.getByText(en["zfs.code.zfs-not-found"]).className).toContain("text-statusFail");
    expect(row.queryByText(/No such file or directory/)).toBeNull();
  });

  it("opens a run in its window", async () => {
    server.runs = [run({ id: "r1", target: "immich" })];
    await renderPage();

    fireEvent.click(within(historyCard()).getByRole("button", { name: "immich" }));

    const window = screen.getByRole("dialog");
    expect(within(window).getByText(en["run.statSnapshot"])).toBeTruthy();
    expect(within(window).getAllByText(/immich/).length).toBeGreaterThan(0);
  });

  it("narrows the list to one day", async () => {
    const yesterday = Math.floor(Date.now() / 1000) - 86400;
    server.runs = [
      run({ id: "r2", target: "immich" }),
      run({ id: "r1", target: "plex", startedAt: yesterday, finishedAt: yesterday + 30 }),
    ];
    await renderPage();
    const card = within(historyCard());
    expect(card.getAllByRole("listitem")).toHaveLength(2);

    fireEvent.click(card.getByRole("combobox", { name: en["run.filterDay"] }));
    fireEvent.click(screen.getByRole("option", { name: new Date(yesterday * 1000).toLocaleDateString("en") }));

    const rows = card.getAllByRole("listitem");
    expect(rows).toHaveLength(1);
    expect(within(rows[0]).getByRole("button", { name: "plex" })).toBeTruthy();
  });
});

describe("the log of the history card", () => {
  it("stands behind the selector, in place of the list", async () => {
    server.runs = [run({ id: "r1", target: "immich" })];
    await renderPage();
    const card = within(historyCard());
    expect(card.queryByTestId("activity-log")).toBeNull();

    fireEvent.click(card.getByRole("tab", { name: en["mcp.log"] }));
    await act(async () => {});

    expect(card.getByTestId("activity-log")).toBeTruthy();
    expect(card.queryByRole("listitem")).toBeNull();
    // The day filter belongs to the list and leaves with it.
    expect(card.queryByRole("combobox", { name: en["run.filterDay"] })).toBeNull();
  });

  it("opens for a link to one run and goes back to the list with the link cleared", async () => {
    server.runs = [run({ id: "r1", target: "immich" })];
    window.history.replaceState(null, "", "/dashboard?run=r1");
    await renderPage();
    const card = within(historyCard());

    expect(card.getByTestId("activity-log")).toBeTruthy();
    expect(card.getByText(en["activityLog.runFilterChip"])).toBeTruthy();

    fireEvent.click(card.getByRole("tab", { name: en["overview.history.runs"] }));
    await act(async () => {});

    expect(card.queryByTestId("activity-log")).toBeNull();
    expect(window.location.search).toBe("");
  });

  it("shows for a link to one run even where the layout hides the history", async () => {
    window.localStorage.setItem(
      "bombvault.dashboardLayout",
      JSON.stringify({ v: 2, order: ["needs", "protection", "targets", "history"], hidden: ["history"], widths: {}, heights: {} })
    );
    window.history.replaceState(null, "", "/dashboard?run=r1");
    await renderPage();

    expect(within(historyCard()).getByTestId("activity-log")).toBeTruthy();
  });
});
