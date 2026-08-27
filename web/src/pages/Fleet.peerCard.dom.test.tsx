// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Issue #179 (manilx): "Have to open it every time. Collapsed not a lot of info
// is shown."
//
// The scorecard IS the peer card's content — collapsed, a card shows little more
// than a name and a URL — so the open state is remembered per browser instead of
// resetting to closed on every visit.
//
// The screenshots on that issue also carry a second, unreported defect: the
// version read "vv8.0.0+main.e3db401". The value already carries its own "v".
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listFleetPeers: () =>
      Promise.resolve({
        ok: true,
        peers: [
          {
            id: "p1",
            name: "DXP480T",
            url: "http://192.168.2.53:3003",
            enabled: true,
            lastPollAt: 1_700_000_000,
            lastPollOk: true,
            lastPollError: "",
            lastPollVersion: "v8.0.0+main.e3db401",
            lastPollDomains: [],
          },
        ],
      }),
    listMeshOffers: () => Promise.resolve({ ok: true, offers: [] }),
    getSettings: () =>
      Promise.resolve({ ok: true, settings: { fleetEnabled: true }, hostMountRoot: "/host/user", platform: "unraid" }),
  };
});

const { Fleet } = await import("./Fleet");

async function renderFleet() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Fleet />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

function detailsButton() {
  return screen.getByRole("button", { name: en["fleet.details"] });
}

function scorecardVisible() {
  return screen.queryByText(en["fleet.scorecardTitle"]) !== null;
}

beforeEach(() => {
  localStorage.clear();
});

afterEach(cleanup);

describe("fleet peer card", () => {
  it("starts collapsed when nothing has been remembered", async () => {
    await renderFleet();
    expect(scorecardVisible()).toBe(false);
  });

  it("remembers that details were opened", async () => {
    await renderFleet();
    await act(async () => {
      fireEvent.click(detailsButton());
    });
    expect(scorecardVisible()).toBe(true);
    expect(localStorage.getItem("bombvault.fleetDetailsOpen")).toBe("1");

    // The whole point of the issue: come back and it is still open.
    cleanup();
    await renderFleet();
    expect(scorecardVisible()).toBe(true);
  });

  it("remembers that they were closed again", async () => {
    localStorage.setItem("bombvault.fleetDetailsOpen", "1");
    await renderFleet();
    expect(scorecardVisible()).toBe(true);

    await act(async () => {
      fireEvent.click(detailsButton());
    });
    expect(scorecardVisible()).toBe(false);
    expect(localStorage.getItem("bombvault.fleetDetailsOpen")).toBe("0");
  });

  it("prints the peer version once, not with a doubled v", async () => {
    await renderFleet();
    expect(screen.getByText("v8.0.0+main.e3db401")).toBeTruthy();
    expect(screen.queryByText(/vv8\.0\.0/)).toBeNull();
  });
});
