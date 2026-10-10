// @vitest-environment jsdom
// The Instances page is a grid of cards: this server first, then every
// paired instance and every ZFS server reached over SSH. Each module adds
// what it knows and nothing is asked of a module that is off. The page asks
// again by itself who is connected, and fetches a connected member's
// scorecard once it has gone stale.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { I18nProvider, countText, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type {
  FleetPeer,
  GroupMember,
  MeshOffer,
  PullSourceView,
  ReceivedRepoStatus,
  ReceiverLogin,
  ZFSReceiveRequest,
  ZFSReplicaServer,
} from "../lib/api";
import { fleetPeer, groupMember, meshOffer, NOW_S, pullSource, receivedRepo } from "./instances/instances.testsupport";
import { receiveRequest, replica, server } from "../components/zfs/replica/replica.testsupport";

const peer = (over: Partial<FleetPeer>) => fleetPeer({ name: "DXP480T", lastPollVersion: "v8.0.0+main.e3db401", ...over });
const M1 = groupMember({ name: "DXP480T" });

type Flags = { receiverEnabled: boolean; fleetEnabled: boolean; pullEnabled: boolean; zfsEnabled: boolean };

let flags: Flags;
let peers: FleetPeer[];
let members: GroupMember[];
let peersOk: boolean;
let requests: ZFSReceiveRequest[];
let servers: ZFSReplicaServer[];
let received: ReceivedRepoStatus[];
let pulls: PullSourceView[];
let logins: ReceiverLogin[];
let offers: MeshOffer[];
const calls: string[] = [];
const polled: string[] = [];
const removed: string[] = [];

function called(name: string): number {
  return calls.filter((c) => c === name).length;
}

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  const answer = <T,>(name: string, value: T) => {
    calls.push(name);
    return Promise.resolve(value);
  };
  return {
    ...actual,
    getSettings: () => Promise.resolve({ ok: true, settings: flags }),
    getGroup: () =>
      answer("getGroup", { ok: true, active: members.length > 0, name: "cellar", selfAddress: "https://192.168.1.20:3443", members }),
    getHealth: () => Promise.resolve({ ok: true, version: "v9.2.1" }),
    getStatus: () => Promise.resolve({ ok: true, domains: [] }),
    listFleetPeers: () => answer("listFleetPeers", peersOk ? { ok: true, peers } : { ok: false, error: "the list is locked" }),
    pollFleetPeer: (id: string) => {
      polled.push(id);
      return Promise.resolve({ ok: true });
    },
    deleteFleetPeer: (id: string) => {
      removed.push(id);
      peers = peers.filter((p) => p.id !== id);
      return Promise.resolve({ ok: true });
    },
    listMeshOffers: () => answer("listMeshOffers", { ok: true, offers }),
    listReceivedRepos: () => answer("listReceivedRepos", { ok: true, repos: received }),
    listPullSources: () => answer("listPullSources", { ok: true, sources: pulls }),
    getReceiverServer: () =>
      answer("getReceiverServer", {
        ok: true,
        server: logins.length > 0 ? { logins, check: "protected", present: true, checkedAt: 0, url: "", user: "", hostPath: "" } : null,
      }),
    listDestinations: () => Promise.resolve({ ok: true, destinations: [] }),
    listGroupReceivers: () => Promise.resolve({ ok: true, receivers: [] }),
    listZFSDatasets: () => answer("listZFSDatasets", { ok: true, datasets: [{ id: "z1", dataset: "tank/appdata" }] }),
    getZFSReplica: () => Promise.resolve(replica({ target: { kind: "peer", id: "m1" }, peerState: "allowed" })),
    listZFSReceiveRequests: () => Promise.resolve(requests),
    listZFSLocalPools: () => Promise.resolve([]),
    listZFSReplicaServers: () => Promise.resolve(servers),
  };
});

const { Instances } = await import("./Instances");
const { FLEET_REFRESH_MS, SCORECARD_STALE_S } = await import("./instances/instancesModel");

function Where() {
  const loc = useLocation();
  return <div data-testid="where">{loc.pathname + loc.hash}</div>;
}

async function renderPage() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <MemoryRouter initialEntries={["/instances"]}>
            <Routes>
              <Route path="/instances" element={<Instances />} />
              <Route path="*" element={<Where />} />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </I18nProvider>,
    );
  });
  // The cards follow the settings, the lists and the replicas of each ZFS item.
  await act(async () => undefined);
}

function cards(): HTMLElement[] {
  return Array.from(document.querySelectorAll<HTMLElement>("[data-instance]"));
}

function card(key: string): HTMLElement {
  const found = document.querySelector<HTMLElement>(`[data-instance="${key}"]`);
  if (!found) throw new Error(`no card for ${key}`);
  return found;
}

function roles(of: HTMLElement): Record<string, string> {
  return Object.fromEntries([...of.querySelectorAll("[data-role]")].map((el) => [el.getAttribute("data-role"), el.textContent]));
}

beforeEach(() => {
  localStorage.clear();
  window.location.hash = "";
  flags = { receiverEnabled: true, fleetEnabled: true, pullEnabled: true, zfsEnabled: false };
  peers = [peer({})];
  members = [M1];
  peersOk = true;
  requests = [];
  servers = [];
  received = [];
  pulls = [];
  logins = [];
  offers = [];
  calls.length = 0;
  polled.length = 0;
  removed.length = 0;
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("the instances grid", () => {
  it("shows this instance first, marked as such, then every member", async () => {
    peers = [peer({}), peer({ id: "p2", memberId: "m2", name: "attic" })];
    await renderPage();
    const all = cards();
    expect(all.map((c) => c.getAttribute("data-instance"))).toEqual(["self", "m1", "m2"]);
    expect(all[0].textContent).toContain("cellar");
    expect(all[0].textContent).toContain(en["instances.thisInstance"]);
    expect(all[0].textContent).toContain("v9.2.1");
    expect(all[1].textContent).not.toContain(en["instances.thisInstance"]);
  });

  it("shows where each instance is reached, without the scheme", async () => {
    peers = [peer({}), peer({ id: "p2", memberId: "gone", name: "attic" })];
    await renderPage();
    expect(card("self").textContent).toContain("192.168.1.20:3443");
    expect(card("self").textContent).not.toContain("https://");
    expect(card("m1").textContent).toContain("192.168.1.21:3443");
    expect(card("gone").textContent).not.toContain("192.168.1.");
  });

  it("says for each member whether it is connected, without naming the route", async () => {
    members = [];
    peers = [
      peer({ id: "p1", memberId: "direct", direct: true, relay: false }),
      peer({ id: "p2", memberId: "relayed", name: "attic", direct: false, relay: true }),
      peer({ id: "p3", memberId: "gone", name: "office", direct: false, relay: false }),
    ];
    await renderPage();
    expect(card("direct").textContent).toContain(en["instances.connected"]);
    expect(card("relayed").textContent).toContain(en["instances.connected"]);
    expect(card("gone").textContent).toContain(en["instances.notConnected"]);
    for (const key of ["direct", "relayed", "gone"]) {
      expect(card(key).textContent).not.toContain(en["pairing.viaRelay"]);
      expect(card(key).textContent).not.toContain(en["pairing.direct"]);
    }
  });

  it("says monitoring is off in place of the connection, and asks a row from before pairing to pair again", async () => {
    peers = [peer({ enabled: false }), peer({ id: "old", memberId: "", name: "office", needsPairing: true })];
    await renderPage();
    expect(card("m1").textContent).toContain(en["fleet.monitoringOff"]);
    expect(card("m1").textContent).not.toContain(en["instances.connected"]);
    expect(card("old").textContent).toContain(en["pairing.pairAgain"]);
    expect(within(card("old")).queryByRole("button", { name: en["fleet.details"] })).toBeNull();
    expect(within(card("old")).getByRole("button", { name: en["fleet.remove"] })).toBeTruthy();
  });

  it("prints the peer version once, not with a doubled v", async () => {
    await renderPage();
    expect(card("m1").textContent).toContain("v8.0.0+main.e3db401");
    expect(card("m1").textContent).not.toMatch(/vv8\.0\.0/);
  });

  it("counts the requests that wait on a card, and the count opens its details", async () => {
    requests = [receiveRequest({ peer: "m1" }), receiveRequest({ id: "rq2", peer: "m1", item: "tank/media" })];
    await renderPage();
    const badge = within(card("m1")).getByRole("button", { name: countText(en["instances.requests"], "en", 2) });
    await act(async () => {
      fireEvent.click(badge);
    });
    expect(screen.getByRole("dialog", { name: "DXP480T" })).toBeTruthy();
  });

  it("shows no request count while nothing waits", async () => {
    requests = [receiveRequest({ peer: "m1", state: "allowed" })];
    await renderPage();
    expect(card("m1").textContent).not.toContain("request");
  });

  it("says on this server's card whom it serves in each role", async () => {
    received = [receivedRepo()];
    requests = [receiveRequest({ peer: "m1", state: "allowed" })];
    await renderPage();
    expect(roles(card("self"))).toEqual({
      receiver: en["receiver.title"] + "for DXP480T",
      fetcher: en["instances.role.fetcher"] + en["instances.forNoOne"],
      zfs: en["zfs.replica.servers.title"] + "for DXP480T",
    });
  });

  it("shows a member's roles as far as they are known, and none as a switch", async () => {
    flags.zfsEnabled = true;
    await renderPage();
    expect(roles(card("m1"))).toEqual({
      receiver: en["receiver.title"] + en["zfs.replica.servers.off"],
      zfs: en["zfs.replica.servers.title"] + en["instances.role.on"],
    });
    expect(within(card("m1")).queryByRole("switch")).toBeNull();
    expect(card("m1").querySelector("button[data-role]")).toBeNull();
  });

  it("removes a member only on the second click", async () => {
    await renderPage();
    await act(async () => {
      fireEvent.click(within(card("m1")).getByRole("button", { name: en["fleet.remove"] }));
    });
    expect(removed).toEqual([]);
    await act(async () => {
      fireEvent.click(within(card("m1")).getByRole("button", { name: en["fleet.confirmRemove"] }));
    });
    expect(removed).toEqual(["p1"]);
  });

  it("opens pairing in Settings from the button under the cards", async () => {
    await renderPage();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["instances.pair"] }));
    });
    expect(screen.getByTestId("where").textContent).toBe("/settings/pairing");
  });

  it("opens this server's own dashboard from its card", async () => {
    await renderPage();
    await act(async () => {
      fireEvent.click(within(card("self")).getByRole("button", { name: en["instances.open"] }));
    });
    expect(screen.getByTestId("where").textContent).toBe("/dashboard");
  });
});

describe("an empty group", () => {
  beforeEach(() => {
    peers = [];
    members = [];
  });

  it("keeps this server's card and names the way to pair", async () => {
    await renderPage();
    expect(cards().map((c) => c.getAttribute("data-instance"))).toEqual(["self", "none"]);
    expect(card("none").textContent).toContain(en["instances.emptyTitle"]);
    expect(card("none").textContent).toContain(en["instances.emptyLead"]);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["instances.pair"] }));
    });
    expect(screen.getByTestId("where").textContent).toBe("/settings/pairing");
  });

  it("offers pairing once, not on the card and under it", async () => {
    await renderPage();
    expect(screen.getAllByRole("button", { name: en["instances.pair"] })).toHaveLength(1);
  });
});

describe("rows that belong to no instance", () => {
  it("stay reachable in a card of their own", async () => {
    received = [receivedRepo({ id: "old", name: "Office off-site", memberId: "", needsPairing: true })];
    pulls = [pullSource({ id: "old", name: "Office containers", memberId: "", needsPairing: true })];
    offers = [meshOffer({ from: "" })];
    await renderPage();
    const card = screen.getByText(en["instances.unplaced.title"]).closest(".glim-notch-card") as HTMLElement;
    expect(within(card).getByLabelText(en["instances.unplaced.hint"])).toBeTruthy();
    expect(within(card).getByText("Office off-site")).toBeTruthy();
    expect(within(card).getByText("Office containers")).toBeTruthy();
    expect(within(card).getByText(en["fleet.mesh.unknownPeer"])).toBeTruthy();
    expect(within(card).getAllByText(en["pairing.pairAgain"])).toHaveLength(2);
  });

  it("lets a row from before pairing choose its instance", async () => {
    received = [receivedRepo({ id: "old", name: "Office off-site", memberId: "", needsPairing: true })];
    await renderPage();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["receiver.edit"] }));
    });
    const dialog = screen.getByRole("dialog", { name: en["receiver.editTitle"] });
    expect(within(dialog).getByText(en["receiver.member"])).toBeTruthy();
  });

  it("is no card while every row has its instance", async () => {
    received = [receivedRepo()];
    offers = [meshOffer({ from: "DXP480T" })];
    await renderPage();
    expect(screen.queryByText(en["instances.unplaced.title"])).toBeNull();
    expect(card("m1").textContent).toContain(countText(en["instances.requests"], "en", 1));
  });
});

describe("ZFS servers without BombVault", () => {
  beforeEach(() => {
    flags.zfsEnabled = true;
    servers = [server({ usedBy: ["z1"] })];
  });

  it("stand in the grid after the instances and open their own page", async () => {
    await renderPage();
    expect(cards().map((c) => c.getAttribute("data-instance"))).toEqual(["self", "m1", "zfs:nas"]);
    const nas = card("zfs:nas");
    expect(nas.textContent).toContain("Backup-NAS");
    expect(nas.textContent).toContain(en["instances.overSsh"]);
    expect(nas.textContent).toContain("root@192.168.1.30");
    expect(nas.textContent).toContain("backup/bombvault-replica");
    await act(async () => {
      fireEvent.click(within(nas).getByRole("button", { name: en["instances.open"] }));
    });
    expect(screen.getByTestId("where").textContent).toBe("/instances/zfs/nas");
  });

  it("say when one is switched off", async () => {
    servers = [server({ enabled: false })];
    await renderPage();
    expect(within(card("zfs:nas")).getByText(en["zfs.replica.servers.off"])).toBeTruthy();
  });

  it("are not shown while the ZFS section is off", async () => {
    flags.zfsEnabled = false;
    await renderPage();
    expect(document.querySelector('[data-instance="zfs:nas"]')).toBeNull();
    expect(called("listZFSDatasets")).toBe(0);
  });
});

describe("the modules behind the page", () => {
  it("asks only the receiver's lists when only the receiver is on", async () => {
    flags = { receiverEnabled: true, fleetEnabled: false, pullEnabled: false, zfsEnabled: false };
    logins = [{ memberId: "m2", name: "barn", user: "barn-login", createdAt: 1 }];
    await renderPage();
    expect(called("listFleetPeers")).toBe(0);
    expect(called("listMeshOffers")).toBe(0);
    expect(called("listPullSources")).toBe(0);
    expect(called("listReceivedRepos")).toBe(1);
    expect(cards().map((c) => c.getAttribute("data-instance"))).toEqual(["self", "m1", "m2"]);
    expect(within(card("self")).getByRole("button", { name: en["receiver.server.title"] })).toBeTruthy();
    expect(roles(card("self")).receiver).toBe(en["receiver.title"] + "for barn");
    expect(within(card("m1")).queryByRole("button", { name: en["fleet.remove"] })).toBeNull();
    expect(within(card("m1")).getByRole("button", { name: en["fleet.details"] })).toBeTruthy();
  });

  it("asks only for the pull sources when only pulling is on", async () => {
    flags = { receiverEnabled: false, fleetEnabled: false, pullEnabled: true, zfsEnabled: false };
    pulls = [pullSource()];
    await renderPage();
    expect(called("listFleetPeers")).toBe(0);
    expect(called("listReceivedRepos")).toBe(0);
    expect(called("getReceiverServer")).toBe(0);
    expect(called("listPullSources")).toBe(1);
    expect(within(card("self")).queryByRole("button", { name: en["receiver.server.title"] })).toBeNull();
    expect(roles(card("self")).fetcher).toBe(en["instances.role.fetcher"] + "for DXP480T");
  });

  it("is only the way into pairing while all three are off", async () => {
    flags = { receiverEnabled: false, fleetEnabled: false, pullEnabled: false, zfsEnabled: false };
    await renderPage();
    expect(cards().map((c) => c.getAttribute("data-instance"))).toEqual(["none"]);
    expect(screen.getByText(en["instances.title"])).toBeTruthy();
  });

  it("says so when the members could not be read", async () => {
    peersOk = false;
    await renderPage();
    expect(screen.getByText("the list is locked")).toBeTruthy();
    expect(card("self")).toBeTruthy();
  });
});

describe("keeping the grid current", () => {
  it("asks again who is connected while the page is open, with no button for it", async () => {
    vi.useFakeTimers();
    await renderPage();
    const before = called("listFleetPeers");
    await act(async () => {
      await vi.advanceTimersByTimeAsync(FLEET_REFRESH_MS);
    });
    expect(called("listFleetPeers")).toBeGreaterThan(before);
    expect(screen.queryByRole("button", { name: /poll/i })).toBeNull();
  });

  it("fetches the scorecard of a connected member once it is stale", async () => {
    const stale = NOW_S - SCORECARD_STALE_S - 1;
    peers = [
      peer({ id: "stale", memberId: "a", lastPollAt: stale }),
      peer({ id: "fresh", memberId: "b" }),
      peer({ id: "away", memberId: "c", direct: false, relay: false, lastPollAt: stale }),
      peer({ id: "off", memberId: "d", enabled: false, lastPollAt: stale }),
    ];
    await renderPage();
    expect(polled).toEqual(["stale"]);
  });
});

describe("an address with a hash", () => {
  it("sends #pairing on to Settings", async () => {
    window.location.hash = "#pairing";
    await renderPage();
    expect(screen.getByTestId("where").textContent).toBe("/settings/pairing");
  });

  it("opens the receiving server for #receiver and drops the hash", async () => {
    window.location.hash = "#receiver";
    await renderPage();
    expect(screen.getByRole("dialog", { name: en["receiver.server.title"] })).toBeTruthy();
    expect(window.location.hash).toBe("");
  });

  it("opens nothing for #receiver while the receiver is off", async () => {
    window.location.hash = "#receiver";
    flags.receiverEnabled = false;
    await renderPage();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("shows the grid for #pull and drops the hash", async () => {
    window.location.hash = "#pull";
    await renderPage();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(card("m1")).toBeTruthy();
    expect(window.location.hash).toBe("");
  });
});

describe("the receiving server", () => {
  it("opens from this server's card and reads the logins again when it closes", async () => {
    await renderPage();
    const before = called("getReceiverServer");
    await act(async () => {
      fireEvent.click(within(card("self")).getByRole("button", { name: en["receiver.server.title"] }));
    });
    const opened = screen.getByRole("dialog", { name: en["receiver.server.title"] });
    await act(async () => {
      fireEvent.click(within(opened).getByRole("button", { name: en["common.done"] }));
    });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(called("getReceiverServer")).toBe(before + 2);
  });
});
