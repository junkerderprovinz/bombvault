// @vitest-environment jsdom
// A phone card has no room for the inventory's four columns, so there each
// source is a short block that keeps its date and size in view.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import { DESKTOP_QUERY } from "../lib/useMediaQuery";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listReceivedRepos: () =>
      Promise.resolve({
        ok: true,
        repos: [
          {
            id: "r1",
            name: "tower off-site",
            repo: "rest:http://192.168.1.9:8000/tower",
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
            memberId: "member-1",
            needsPairing: false,
            lastReceived: "",
            snapshotCount: 12,
            reachable: true,
          },
        ],
      }),
    receiverInventory: () =>
      Promise.resolve({
        ok: true,
        inventory: {
          sources: [
            { host: "tower", item: "nextcloud", snapshotCount: 12, lastReceived: "", totalSize: 2 * 1024 ** 3 },
          ],
          snapshotCount: 12,
          lastReceived: "",
          totalSize: 3 * 1024 ** 3,
        },
      }),
  };
});

const { Receiver } = await import("./Receiver");

// useIsDesktop keeps the first MediaQueryList it gets, so the stub answers
// from a variable instead of being swapped per test.
let desktop = true;
window.matchMedia = ((query: string) => ({
  get matches() {
    return query === DESKTOP_QUERY ? desktop : false;
  },
  media: query,
  onchange: null,
  addListener: () => {},
  removeListener: () => {},
  addEventListener: () => {},
  removeEventListener: () => {},
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia;

async function openInventory() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Receiver embedded />
        </ToastProvider>
      </I18nProvider>,
    );
  });
  await act(async () => {
    fireEvent.click(await screen.findByRole("button", { name: en["receiver.details"] }));
  });
  await screen.findByText(en["receiver.total"]);
}

afterEach(cleanup);

describe("the received inventory", () => {
  it("is a table on the desktop", async () => {
    desktop = true;
    await openInventory();
    expect(screen.getByRole("table")).toBeTruthy();
  });

  it("lists each source with its size and date on a phone", async () => {
    desktop = false;
    await openInventory();
    expect(screen.queryByRole("table")).toBeNull();
    const items = screen.getAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(items[0].textContent).toContain("nextcloud");
    expect(items[0].textContent).toContain("2.0 GB");
    expect(items[0].textContent).toContain(en["receiver.colLastReceived"]);
    expect(items[1].textContent).toContain(en["receiver.total"]);
    expect(items[1].textContent).toContain("3.0 GB");
  });
});
