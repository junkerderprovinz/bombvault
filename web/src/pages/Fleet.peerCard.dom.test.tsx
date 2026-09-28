// @vitest-environment jsdom
// The Fleet tab shows every instance of the group as a card, this one first,
// each with whether it is connected. It asks again by itself instead of
// offering a button, and fetches a connected member's scorecard once it has
// gone stale. A card's details open state is remembered per browser.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { FleetPeer } from "../lib/api";

const NOW_S = Math.floor(Date.now() / 1000);

function peer(over: Partial<FleetPeer>): FleetPeer {
  return {
    id: "p1",
    memberId: "m1",
    name: "DXP480T",
    url: "",
    enabled: true,
    needsPairing: false,
    direct: true,
    relay: false,
    lastPollAt: NOW_S - 60,
    lastPollOk: true,
    lastPollError: "",
    lastPollInstanceName: "",
    lastPollVersion: "v8.0.0+main.e3db401",
    lastPollDomains: [],
    createdAt: 0,
    sortOrder: 0,
    ...over,
  };
}

let peers: FleetPeer[] = [];
const listCalls = vi.fn();
const polled: string[] = [];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listFleetPeers: () => {
      listCalls();
      return Promise.resolve({ ok: true, peers });
    },
    pollFleetPeer: (id: string) => {
      polled.push(id);
      return Promise.resolve({ ok: true });
    },
    getGroup: () => Promise.resolve({ ok: true, name: "cellar" }),
    getHealth: () => Promise.resolve({ ok: true, version: "v9.2.1" }),
    getStatus: () => Promise.resolve({ ok: true, domains: [] }),
    listMeshOffers: () => Promise.resolve({ ok: true, offers: [] }),
  };
});

const { Fleet, FLEET_REFRESH_MS, SCORECARD_STALE_S, scorecardDue } = await import("./Fleet");

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

function cards(): HTMLElement[] {
  return Array.from(document.querySelectorAll<HTMLElement>(".glim-stagger-row"));
}

function detailsButton() {
  return screen.getByRole("button", { name: en["fleet.details"] });
}

function detailsOpen() {
  return screen.queryByRole("switch", { name: en["fleet.enabledLabel"] }) !== null;
}

beforeEach(() => {
  localStorage.clear();
  peers = [peer({})];
  listCalls.mockClear();
  polled.length = 0;
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("fleet cards", () => {
  it("shows this instance first, marked as such, then every member", async () => {
    peers = [peer({}), peer({ id: "p2", memberId: "m2", name: "attic" })];
    await renderFleet();
    const all = cards();
    expect(all).toHaveLength(3);
    expect(all[0].textContent).toContain("cellar");
    expect(all[0].textContent).toContain(en["instances.thisInstance"]);
    expect(all[0].textContent).toContain("v9.2.1");
    expect(all[1].textContent).not.toContain(en["instances.thisInstance"]);
  });

  it("says for each member whether it is connected, without naming the route", async () => {
    peers = [
      peer({ id: "direct", direct: true, relay: false }),
      peer({ id: "relayed", name: "attic", direct: false, relay: true }),
      peer({ id: "gone", name: "office", direct: false, relay: false }),
    ];
    await renderFleet();
    const [, direct, relayed, gone] = cards();
    expect(direct.textContent).toContain(en["instances.connected"]);
    expect(relayed.textContent).toContain(en["instances.connected"]);
    expect(gone.textContent).toContain(en["instances.notConnected"]);
    for (const card of [direct, relayed, gone]) {
      expect(card.textContent).not.toContain(en["pairing.viaRelay"]);
      expect(card.textContent).not.toContain(en["pairing.direct"]);
    }
  });

  it("asks again who is connected while the page is open, with no button for it", async () => {
    vi.useFakeTimers();
    await renderFleet();
    const before = listCalls.mock.calls.length;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(FLEET_REFRESH_MS);
    });
    expect(listCalls.mock.calls.length).toBeGreaterThan(before);
    expect(screen.queryByRole("button", { name: /poll/i })).toBeNull();
  });

  it("fetches the scorecard of a connected member once it is stale", async () => {
    const stale = NOW_S - SCORECARD_STALE_S - 1;
    peers = [
      peer({ id: "stale", lastPollAt: stale }),
      peer({ id: "fresh" }),
      peer({ id: "away", direct: false, relay: false, lastPollAt: stale }),
      peer({ id: "off", enabled: false, lastPollAt: stale }),
    ];
    await renderFleet();
    expect(polled).toEqual(["stale"]);
    expect(scorecardDue(peer({ lastPollAt: 0 }), NOW_S)).toBe(true);
    expect(scorecardDue(peer({ needsPairing: true, lastPollAt: 0 }), NOW_S)).toBe(false);
  });

  it("prints the peer version once, not with a doubled v", async () => {
    await renderFleet();
    expect(screen.getByText("v8.0.0+main.e3db401")).toBeTruthy();
    expect(screen.queryByText(/vv8\.0\.0/)).toBeNull();
  });
});

describe("fleet card details", () => {
  it("start collapsed when nothing has been remembered", async () => {
    await renderFleet();
    expect(detailsOpen()).toBe(false);
  });

  it("remember that they were opened", async () => {
    await renderFleet();
    await act(async () => {
      fireEvent.click(detailsButton());
    });
    expect(detailsOpen()).toBe(true);
    expect(localStorage.getItem("bombvault.fleetDetailsOpen")).toBe("1");

    cleanup();
    await renderFleet();
    expect(detailsOpen()).toBe(true);
  });

  it("remember that they were closed again", async () => {
    localStorage.setItem("bombvault.fleetDetailsOpen", "1");
    await renderFleet();
    expect(detailsOpen()).toBe(true);

    await act(async () => {
      fireEvent.click(detailsButton());
    });
    expect(detailsOpen()).toBe(false);
    expect(localStorage.getItem("bombvault.fleetDetailsOpen")).toBe("0");
  });
});
