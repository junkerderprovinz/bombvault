// @vitest-environment jsdom
// A key's log on the MCP card links runs here. The log loads only the newest
// runs, so a linked run has to be asked for by id, and a run the history no
// longer holds has to say so instead of showing an empty log.
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18nProvider, en } from "../lib/i18n";
import type { Run } from "../lib/api";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

const listRuns = vi.fn();
vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listRuns: (...a: unknown[]) => listRuns(...a),
    getScheduleNext: () => Promise.resolve([]),
  };
});

const { ActivityLog } = await import("./ActivityLog");

function run(over: Partial<Run>): Run {
  return {
    id: "r1",
    targetId: "t1",
    kind: "backup",
    status: "success",
    startedAt: 1_700_000_000,
    finishedAt: 1_700_000_060,
    snapshotId: "",
    bytes: 0,
    error: "",
    acknowledged: false,
    target: "plex",
    domain: "container",
    ...over,
  } as Run;
}

async function renderLog(runFilter: string | null) {
  await act(async () => {
    render(
      <I18nProvider>
        <ActivityLog runFilter={runFilter} />
      </I18nProvider>
    );
  });
}

beforeEach(() => {
  listRuns.mockReset();
});

afterEach(cleanup);

it("asks the server for the linked run and shows its line", async () => {
  listRuns.mockImplementation((id?: string) =>
    Promise.resolve({
      ok: true,
      runs: [run({ id: "new1", target: "sonarr" }), ...(id === "old1" ? [run({ id: "old1", target: "plex" })] : [])],
    })
  );
  await renderLog("old1");

  expect(listRuns).toHaveBeenCalledWith("old1");
  expect(await screen.findByText(/plex/)).toBeTruthy();
  expect(screen.queryByText(/sonarr/)).toBeNull();
  expect(screen.queryByText(en["activityLog.runGone"])).toBeNull();
});

it("says so when the linked run is no longer in the history", async () => {
  listRuns.mockResolvedValue({ ok: true, runs: [run({ id: "new1", target: "sonarr" })] });
  await renderLog("gone1");

  expect(await screen.findByText(en["activityLog.runGone"])).toBeTruthy();
});

it("says nothing about a missing run before the history has loaded", async () => {
  listRuns.mockReturnValue(new Promise(() => {}));
  await renderLog("gone1");

  expect(screen.queryByText(en["activityLog.runGone"])).toBeNull();
});
