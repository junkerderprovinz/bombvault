// @vitest-environment jsdom
// The totals row closes the inventory table. Space sets it apart, never a line,
// so it does not read as one more source.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { ReceivedRepoStatus, ReceiverSource } from "../lib/api";

const tower: ReceivedRepoStatus = {
  id: "r1",
  name: "tower off-site",
  repo: "rest:http://192.168.1.50:8000/tower-containers",
  deadManHours: 26,
  checkCadence: "",
  readDataPercent: 0,
  lastCheckAt: 0,
  lastCheckOk: null,
  lastCheckError: "",
  lastCheckReadData: false,
  enabled: true,
  createdAt: 0,
  sortOrder: 0,
  hasAppKey: true,
  lastReceived: "",
  snapshotCount: 3,
  reachable: true,
};

const source = (item: string): ReceiverSource => ({
  host: "tower",
  item,
  snapshotCount: 1,
  lastReceived: "",
  totalSize: 1024,
});

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  listReceivedRepos: async () => ({ ok: true, repos: [tower] }),
  receiverInventory: async () => ({
    ok: true,
    inventory: { sources: [source("plex"), source("nextcloud")], snapshotCount: 3, lastReceived: "", totalSize: 2048 },
  }),
}));

const { Receiver } = await import("./Receiver");

afterEach(cleanup);

describe("receiver inventory", () => {
  it("sets the totals row apart from the sources by space", async () => {
    await act(async () => {
      render(
        <I18nProvider>
          <ToastProvider>
            <Receiver />
          </ToastProvider>
        </I18nProvider>,
      );
    });
    await act(async () => fireEvent.click(screen.getByRole("button", { name: en["receiver.details"] })));

    const total = screen.getByText(en["receiver.total"]).closest("tr")!;
    expect(total.parentElement!.tagName).toBe("TFOOT");
    for (const cell of total.querySelectorAll("td")) expect(cell.className).toMatch(/\bpt-3\b/);
    for (const cell of screen.getByText("plex").closest("tr")!.querySelectorAll("td")) {
      expect(cell.className).not.toMatch(/\bpt-3\b/);
    }
  });
});
