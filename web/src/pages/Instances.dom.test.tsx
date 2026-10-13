// @vitest-environment jsdom
// The Instances page is a picture of the group above a grid of cards: this
// server first, then every paired instance and every ZFS server reached over
// SSH. Both stand on the page itself, with no card around them. The tiles on
// a card switch what the instance does for this server, read from the roles
// the server keeps. The key keeper is the fourth of them:
// its tile gives the instance this server's key deposit and shows the
// recovery code the one time there is one. Each module adds what it knows and
// nothing is asked of a module that is off. The page asks again by itself who
// is connected, and fetches a connected member's scorecard once it has gone
// stale.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { I18nProvider, countText, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type {
  DepositRefusal,
  Deposits,
  FleetPeer,
  GroupMember,
  MemberRoles,
  MeshOffer,
  PullSourceView,
  ReceivedRepoStatus,
  RoleRequest,
  SelfRoles,
  ZFSReceiveRequest,
  ZFSReplica,
  ZFSReplicaPatch,
  ZFSReplicaServer,
} from "../lib/api";
import {
  deposits,
  fleetPeer,
  groupMember,
  memberRoles,
  meshOffer,
  NOW_S,
  pullSource,
  receivedRepo,
  roleRequest,
  roleSide,
  sentDeposit,
  zfsRole,
} from "./instances/instances.testsupport";
import { receiveRequest, replica, server } from "../components/zfs/replica/replica.testsupport";

const peer = (over: Partial<FleetPeer>) => fleetPeer({ name: "DXP480T", lastPollVersion: "v8.0.0+main.e3db401", ...over });
const M1 = groupMember({ name: "DXP480T" });

type Flags = { receiverEnabled: boolean; fleetEnabled: boolean; pullEnabled: boolean; zfsEnabled: boolean };

let flags: Flags;
let peers: FleetPeer[];
let members: GroupMember[];
let peersOk: boolean;
let roles: MemberRoles[];
let rolesOk: boolean;
let selfRoles: SelfRoles;
let serverRuns: boolean;
let requests: ZFSReceiveRequest[];
let servers: ZFSReplicaServer[];
let received: ReceivedRepoStatus[];
let pulls: PullSourceView[];
let offers: MeshOffer[];
let itemReplica: ZFSReplica;
let askAnswer: Partial<RoleRequest>;
let depositList: Deposits;
let keeperRefusal: DepositRefusal | "" | null;
let keeperTold: boolean;
const CODE = "K7QF2-M9XRT-4HPLD-W3NVC";
const SINCE = new Date(2026, 2, 9, 12).toISOString();
const keeperAsked: string[] = [];
const keeperWithdrawn: string[] = [];
const calls: string[] = [];
const polled: string[] = [];
const removed: string[] = [];
const forgotten: string[] = [];
const asked: [string, string, unknown][] = [];
const withdrawn: [string, string][] = [];
const patched: [string, ZFSReplicaPatch][] = [];

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
    getSettings: () =>
      Promise.resolve({
        ok: true,
        settings: { ...flags, containersEnabled: true, vmsEnabled: false, filesEnabled: false, flashEnabled: true, configEnabled: false },
      }),
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
    forgetMemberKey: (memberId: string) => {
      forgotten.push(memberId);
      return Promise.resolve({ ok: true });
    },
    listRoles: () => answer("listRoles", rolesOk ? { ok: true, members: roles, self: selfRoles } : { ok: false, error: "the roles are locked" }),
    askRole: (memberId: string, role: "receiver" | "fetcher", ask: unknown) => {
      asked.push([memberId, role, ask]);
      const request = roleRequest({ sections: (ask as { sections: RoleRequest["sections"] }).sections, ...askAnswer });
      roles = roles.map((r) => (r.memberId === memberId ? { ...r, byMember: { ...r.byMember, [role]: request } } : r));
      return Promise.resolve({ ok: true, request });
    },
    withdrawRole: (memberId: string, role: "receiver" | "fetcher") => {
      withdrawn.push([memberId, role]);
      roles = roles.map((r) => (r.memberId === memberId ? { ...r, byMember: { ...r.byMember, [role]: null } } : r));
      return Promise.resolve({ ok: true, told: true });
    },
    listDeposits: () => answer("listDeposits", { ok: true, deposits: depositList }),
    askKeeper: (memberId: string) => {
      keeperAsked.push(memberId);
      if (keeperRefusal !== null) {
        return Promise.resolve({ ok: false, ...(keeperRefusal && { code: keeperRefusal }), error: "the server's own words" });
      }
      const first = depositList.codeSince === "";
      const keeper = { state: "held" as const, at: new Date().toISOString() };
      depositList = {
        ...depositList,
        codeSince: first ? SINCE : depositList.codeSince,
        sent: [...depositList.sent.filter((d) => d.memberId !== memberId), sentDeposit({ memberId, name: "DXP480T" })],
      };
      roles = roles.map((r) => (r.memberId === memberId ? { ...r, byMember: { ...r.byMember, keeper } } : r));
      return Promise.resolve({ ok: true, keeper, ...(first ? { recoveryCode: CODE } : {}) });
    },
    withdrawKeeper: (memberId: string) => {
      keeperWithdrawn.push(memberId);
      depositList = { ...depositList, sent: depositList.sent.filter((d) => d.memberId !== memberId) };
      roles = roles.map((r) => (r.memberId === memberId ? { ...r, byMember: { ...r.byMember, keeper: null } } : r));
      return Promise.resolve({ ok: true, told: keeperTold, codeGone: depositList.sent.length === 0 });
    },
    listMeshOffers: () => answer("listMeshOffers", { ok: true, offers }),
    listReceivedRepos: () => answer("listReceivedRepos", { ok: true, repos: received }),
    listPullSources: () => answer("listPullSources", { ok: true, sources: pulls }),
    getReceiverServer: () =>
      answer("getReceiverServer", {
        ok: true,
        server: serverRuns ? { logins: [], check: "protected", present: true, checkedAt: 0, url: "", user: "", hostPath: "" } : null,
      }),
    listDestinations: () => Promise.resolve({ ok: true, destinations: [] }),
    listGroupReceivers: () => Promise.resolve({ ok: true, receivers: [] }),
    listZFSDatasets: () => answer("listZFSDatasets", { ok: true, datasets: [{ id: "z1", dataset: "tank/appdata" }] }),
    getZFSReplica: () => Promise.resolve(itemReplica),
    patchZFSReplica: (id: string, patch: ZFSReplicaPatch) => {
      patched.push([id, patch]);
      itemReplica = { ...itemReplica, ...patch };
      return Promise.resolve({ ok: true });
    },
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

function tiles(of: HTMLElement): Record<string, string> {
  return Object.fromEntries([...of.querySelectorAll("[data-role]")].map((el) => [el.getAttribute("data-role"), el.textContent]));
}

function tile(key: string, role: string): HTMLButtonElement {
  return card(key).querySelector(`button[data-role="${role}"]`) as HTMLButtonElement;
}

function question(): HTMLElement {
  return screen.getByRole("dialog", { name: en["confirmDialog.title"] });
}

async function click(el: Element) {
  await act(async () => {
    fireEvent.click(el);
  });
}

const allowed = (over: Partial<RoleRequest> = {}) => roleRequest({ state: "allowed", decidedBy: "person", ...over });

beforeEach(() => {
  localStorage.clear();
  window.location.hash = "";
  flags = { receiverEnabled: true, fleetEnabled: true, pullEnabled: true, zfsEnabled: false };
  peers = [peer({})];
  members = [M1];
  peersOk = true;
  roles = [memberRoles({ name: "DXP480T" })];
  rolesOk = true;
  selfRoles = { receiver: [], fetcher: [], zfs: [], keeper: [] };
  serverRuns = false;
  requests = [];
  servers = [];
  received = [];
  pulls = [];
  offers = [];
  itemReplica = replica({ target: { kind: "none", id: "" } });
  askAnswer = {};
  depositList = deposits();
  keeperRefusal = null;
  keeperTold = true;
  keeperAsked.length = 0;
  keeperWithdrawn.length = 0;
  calls.length = 0;
  polled.length = 0;
  removed.length = 0;
  forgotten.length = 0;
  asked.length = 0;
  withdrawn.length = 0;
  patched.length = 0;
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

  it("stands on the page with the picture, with no card around the cards", async () => {
    await renderPage();
    const grid = card("self").parentElement as HTMLElement;
    expect(grid.closest(".rounded-card")).toBeNull();
    expect(grid.parentElement).toBe(screen.getByRole("heading", { level: 1 }).parentElement);
    expect(screen.getByTestId("net-picture").closest(".rounded-card")?.parentElement).toBe(grid);
    for (const key of ["self", "m1"]) expect(card(key).classList.contains("bg-carbon-surface")).toBe(true);
  });

  it("gives every card the next colour of the page, and the rows without an instance the one after the last card", async () => {
    flags.zfsEnabled = true;
    servers = [server()];
    members = [M1, groupMember({ id: "m3", name: "Pixel 8", kind: "android", address: "", direct: false, relay: true })];
    peers = [peer({}), peer({ id: "p3", memberId: "m3", name: "Pixel 8", kind: "android", direct: false, relay: true, lastPollDomains: [] })];
    received = [receivedRepo({ id: "old", name: "Office off-site", memberId: "", needsPairing: true })];
    await renderPage();
    const hue = (el: HTMLElement) => el.style.getPropertyValue("--item-hue");
    expect(cards().map(hue)).toEqual(["var(--rb-0)", "var(--rb-1)", "var(--rb-2)", "var(--rb-3)"]);
    const unplaced = screen.getByText(en["instances.unplaced.title"]).closest(".glim-notch-card") as HTMLElement;
    expect(hue(unplaced)).toBe("var(--rb-4)");
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
    expect(card("old").querySelector("[data-role]")).toBeNull();
  });

  it("prints the peer version once, not with a doubled v", async () => {
    await renderPage();
    expect(card("m1").textContent).toContain("v8.0.0+main.e3db401");
    expect(card("m1").textContent).not.toMatch(/vv8\.0\.0/);
  });

  it("removes a member only on the second click", async () => {
    await renderPage();
    await click(within(card("m1")).getByRole("button", { name: en["fleet.remove"] }));
    expect(removed).toEqual([]);
    await click(within(card("m1")).getByRole("button", { name: en["fleet.confirmRemove"] }));
    expect(removed).toEqual(["p1"]);
  });

  it("opens pairing in Settings from the page's own action, the one filled button", async () => {
    await renderPage();
    const pair = within(screen.getByTestId("page-actions")).getByRole("button", { name: en["pairing.title"] });
    expect(pair.className).toContain("bg-accent");
    await click(pair);
    expect(screen.getByTestId("where").textContent).toBe("/settings/pairing");
  });

  it("opens this server's own dashboard from its card", async () => {
    await renderPage();
    await click(within(card("self")).getByRole("button", { name: en["instances.open"] }));
    expect(screen.getByTestId("where").textContent).toBe("/dashboard");
  });

  it("says so when the roles could not be read, and still shows the cards", async () => {
    rolesOk = false;
    await renderPage();
    expect(screen.getByText("the roles are locked")).toBeTruthy();
    expect(tiles(card("m1")).receiver).toBe(en["receiver.title"] + en["zfs.replica.servers.off"]);
  });
});

describe("requests from another instance", () => {
  it("shows how many wait as a badge that is no control, and the details answer them", async () => {
    roles = [memberRoles({ name: "DXP480T", asks: 2, forMember: roleSide({ receiver: roleRequest(), fetcher: roleRequest({ id: "rr2" }) }) })];
    await renderPage();
    const badge = within(card("m1")).getByText(countText(en["instances.requests"], "en", 2));
    expect(badge.tagName).toBe("SPAN");
    expect(badge.className).toContain("statusWarn");
    expect(within(card("m1")).queryByRole("button", { name: /request/ })).toBeNull();
    await click(within(card("m1")).getByRole("button", { name: en["fleet.details"] }));
    const win = screen.getByRole("dialog", { name: "DXP480T" });
    expect(within(win).getAllByRole("button", { name: en["zfs.receive.allow"] })).toHaveLength(2);
  });

  it("shows no request count while nothing waits", async () => {
    roles = [memberRoles({ name: "DXP480T", forMember: roleSide({ receiver: allowed() }) })];
    await renderPage();
    expect(card("m1").textContent).not.toContain("request");
  });

  it("counts a storage offer with the requests", async () => {
    offers = [meshOffer({ from: "DXP480T" })];
    await renderPage();
    expect(card("m1").textContent).toContain(countText(en["instances.requests"], "en", 1));
  });
});

describe("this server's own card", () => {
  it("says for whom this server is Receiver, Fetcher, ZFS server and Key keeper, as the server reports it", async () => {
    selfRoles = {
      receiver: [
        { memberId: "m1", name: "DXP480T" },
        { memberId: "m2", name: "barn" },
      ],
      fetcher: [],
      zfs: [{ memberId: "m9", name: "" }],
      keeper: [{ memberId: "m2", name: "barn" }],
    };
    await renderPage();
    expect(tiles(card("self"))).toEqual({
      receiver: en["receiver.title"] + "for DXP480T, barn",
      fetcher: en["instances.role.fetcher"] + en["instances.forNoOne"],
      zfs: en["zfs.replica.servers.title"] + "for m9",
      keeper: en["instances.role.keeper"] + "for barn",
    });
    expect(card("self").querySelector("button[data-role]")).toBeNull();
  });

  it("does not take a received repository or a pull source for a role", async () => {
    received = [receivedRepo()];
    pulls = [pullSource()];
    await renderPage();
    expect(tiles(card("self")).receiver).toBe(en["receiver.title"] + en["instances.forNoOne"]);
    expect(tiles(card("self")).fetcher).toBe(en["instances.role.fetcher"] + en["instances.forNoOne"]);
  });

  it("opens the key deposits only while there is one on either side", async () => {
    await renderPage();
    expect(within(card("self")).queryByRole("button", { name: en["keeper.window.title"] })).toBeNull();
    cleanup();
    depositList = deposits({ codeSince: SINCE, sent: [sentDeposit({ name: "DXP480T" })] });
    await renderPage();
    await click(within(card("self")).getByRole("button", { name: en["keeper.window.title"] }));
    const opened = screen.getByRole("dialog", { name: en["keeper.window.title"] });
    expect(within(opened).getByTestId("code-since").textContent).toBe(
      en["keeper.own.codeSince"].replace("{date}", new Date(SINCE).toLocaleDateString()),
    );
    await click(within(opened).getByRole("button", { name: en["common.done"] }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("opens the receiving server only while this server runs one", async () => {
    await renderPage();
    expect(within(card("self")).queryByRole("button", { name: en["receiver.server.title"] })).toBeNull();
    cleanup();
    serverRuns = true;
    await renderPage();
    const before = called("getReceiverServer");
    await click(within(card("self")).getByRole("button", { name: en["receiver.server.title"] }));
    const opened = screen.getByRole("dialog", { name: en["receiver.server.title"] });
    await click(within(opened).getByRole("button", { name: en["common.done"] }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(called("getReceiverServer")).toBe(before + 2);
  });
});

describe("the role tiles of a paired instance", () => {
  it("show Receiver, Fetcher and ZFS server as buttons with what each stands at", async () => {
    flags.zfsEnabled = true;
    roles = [
      memberRoles({
        name: "DXP480T",
        byMember: roleSide({ receiver: allowed(), fetcher: roleRequest(), zfs: zfsRole("off") }),
        // What this server does for the member is no business of these tiles.
        forMember: roleSide({ zfs: zfsRole("allowed") }),
      }),
    ];
    await renderPage();
    expect(tiles(card("m1"))).toEqual({
      receiver: en["receiver.title"] + en["instances.role.on"],
      fetcher: en["instances.role.fetcher"] + en["instances.role.waiting"],
      zfs: en["zfs.replica.servers.title"] + en["zfs.replica.servers.off"],
      keeper: en["instances.role.keeper"] + en["zfs.replica.servers.off"],
    });
    expect([tile("m1", "receiver"), tile("m1", "fetcher"), tile("m1", "zfs")].map((b) => b.getAttribute("data-state"))).toEqual([
      "on",
      "waiting",
      "off",
    ]);
    expect(tile("m1", "fetcher").getAttribute("aria-pressed")).toBe("true");
    expect(tile("m1", "fetcher").className).toContain("outline-dashed");
    expect(tile("m1", "receiver").className).not.toContain("outline-dashed");
  });

  it("treat a declined or revoked request as off", async () => {
    roles = [
      memberRoles({
        name: "DXP480T",
        byMember: roleSide({ receiver: roleRequest({ state: "refused" }), fetcher: roleRequest({ state: "revoked" }) }),
      }),
    ];
    await renderPage();
    expect(tile("m1", "receiver").getAttribute("data-state")).toBe("off");
    expect(tile("m1", "fetcher").getAttribute("data-state")).toBe("off");
  });

  it("leave out the ZFS server while this server has no ZFS section and no replica there", async () => {
    await renderPage();
    expect(Object.keys(tiles(card("m1")))).toEqual(["receiver", "fetcher", "keeper"]);
  });

  it("ask for a role that is off: the sections this server backs up, without ZFS data at first", async () => {
    flags.zfsEnabled = true;
    await renderPage();
    await click(tile("m1", "receiver"));
    const dialog = screen.getByRole("dialog", { name: "DXP480T as Receiver" });
    expect(dialog.textContent).toContain("DXP480T gets a request.");
    const switches = within(dialog).getAllByRole("switch");
    expect(switches.map((s) => [s.getAttribute("aria-label"), s.getAttribute("aria-checked")])).toEqual([
      [en["jobs.containersSection"], "true"],
      [en["jobs.flashSection"], "true"],
      [en["jobs.zfsSection"], "false"],
    ]);
    await click(switches[1]);
    await click(within(dialog).getByRole("button", { name: en["instances.role.send"] }));
    expect(asked).toEqual([["m1", "receiver", { sections: ["containers"], store: "rest" }]]);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByText("DXP480T has to allow the request first.")).toBeTruthy();
    expect(tile("m1", "receiver").getAttribute("data-state")).toBe("waiting");
    expect(tile("m1", "receiver").textContent).toContain(en["instances.role.waiting"]);
  });

  it("ask a Receiver where it keeps the copies: its receiving server unless a share is picked", async () => {
    await renderPage();
    await click(tile("m1", "receiver"));
    const dialog = screen.getByRole("dialog", { name: "DXP480T as Receiver" });
    const where = within(dialog).getByRole("radiogroup", { name: "Where on DXP480T" });
    const choices = within(where).getAllByRole("radio");
    expect(choices.map((c) => [c.textContent, c.getAttribute("aria-checked")])).toEqual([
      ["DXP480T's receiving server", "true"],
      [en["rcloneRemote.share"], "false"],
    ]);
    expect(within(dialog).getByText(en["instances.role.storeRestNote"])).toBeTruthy();
    await click(choices[1]);
    expect(within(dialog).queryByText(en["instances.role.storeRestNote"])).toBeNull();
    await click(within(dialog).getByRole("button", { name: en["instances.role.send"] }));
    expect(asked[0][2]).toMatchObject({ store: "share" });
  });

  it("ask a Fetcher for its sections only, since it keeps nothing for this server", async () => {
    await renderPage();
    await click(tile("m1", "fetcher"));
    const dialog = screen.getByRole("dialog", { name: "DXP480T as Fetcher" });
    expect(within(dialog).queryByRole("radiogroup")).toBeNull();
    await click(within(dialog).getByRole("button", { name: en["instances.role.send"] }));
    expect(asked[0][2]).not.toHaveProperty("store");
  });

  it("send nothing without a section", async () => {
    await renderPage();
    await click(tile("m1", "fetcher"));
    const dialog = screen.getByRole("dialog", { name: "DXP480T as Fetcher" });
    expect(dialog.textContent).toContain(en["instances.role.sectionsFetch"]);
    for (const s of within(dialog).getAllByRole("switch")) {
      if (s.getAttribute("aria-checked") === "true") await click(s);
    }
    await click(within(dialog).getByRole("button", { name: en["instances.role.send"] }));
    expect(asked).toEqual([]);
    expect(within(dialog).getByText(en["instances.role.needSection"])).toBeTruthy();
  });

  it("change nothing when the dialog is cancelled", async () => {
    await renderPage();
    await click(tile("m1", "receiver"));
    await click(within(screen.getByRole("dialog")).getByRole("button", { name: en["common.cancel"] }));
    expect(asked).toEqual([]);
    expect(tile("m1", "receiver").getAttribute("data-state")).toBe("off");
  });

  it("withdraw a role that is on after saying what stops", async () => {
    roles = [memberRoles({ name: "DXP480T", byMember: roleSide({ receiver: allowed() }) })];
    await renderPage();
    await click(tile("m1", "receiver"));
    expect(question().textContent).toContain("Stop using DXP480T as Receiver?");
    expect(question().textContent).toContain("This server stops sending off-site copies to DXP480T. What is there already stays there.");
    expect(withdrawn).toEqual([]);
    await click(within(question()).getByRole("button", { name: en["instances.role.switchOff"] }));
    expect(withdrawn).toEqual([["m1", "receiver"]]);
    expect(tile("m1", "receiver").getAttribute("data-state")).toBe("off");
  });

  it("take back a request that still waits the same way", async () => {
    roles = [memberRoles({ name: "DXP480T", byMember: roleSide({ fetcher: roleRequest() }) })];
    await renderPage();
    await click(tile("m1", "fetcher"));
    expect(question().textContent).toContain("DXP480T then fetches nothing more from this server.");
    await click(within(question()).getByRole("button", { name: en["instances.role.switchOff"] }));
    expect(withdrawn).toEqual([["m1", "fetcher"]]);
  });

  it("keep a role that is on when the question is cancelled", async () => {
    roles = [memberRoles({ name: "DXP480T", byMember: roleSide({ receiver: allowed() }) })];
    await renderPage();
    await click(tile("m1", "receiver"));
    await click(within(question()).getByRole("button", { name: en["common.cancel"] }));
    expect(withdrawn).toEqual([]);
    expect(tile("m1", "receiver").getAttribute("data-state")).toBe("on");
  });
});

describe("the key keeper tile", () => {
  const keeps = (state: "held" | "stale" | "changed" | "declined", over: Partial<MemberRoles> = {}) =>
    memberRoles({ name: "DXP480T", byMember: roleSide({ keeper: { state, at: new Date((NOW_S - 3600) * 1000).toISOString() } }), ...over });
  const dialog = () => screen.getByRole("dialog", { name: "DXP480T as Key keeper" });

  it("is filled while the instance holds the deposit, and says when the deposit is behind or was deleted there", async () => {
    roles = [keeps("held"), keeps("stale", { memberId: "m2", name: "barn" }), keeps("declined", { memberId: "m3", name: "shed" })];
    await renderPage();
    expect(["m1", "m2", "m3"].map((key) => [tile(key, "keeper").getAttribute("data-state"), tile(key, "keeper").textContent])).toEqual([
      ["on", en["instances.role.keeper"] + en["instances.role.on"]],
      ["waiting", en["instances.role.keeper"] + en["keeper.state.stale"]],
      ["off", en["instances.role.keeper"] + en["keeper.state.declined"]],
    ]);
    expect(tile("m2", "keeper").className).toContain("outline-dashed");
  });

  it("says when the instance holds something this server did not send", async () => {
    roles = [keeps("changed")];
    await renderPage();
    expect(tile("m1", "keeper").getAttribute("data-state")).toBe("waiting");
    expect(tile("m1", "keeper").textContent).toBe(en["instances.role.keeper"] + en["keeper.state.changed"]);
  });

  it("tells an instance that could not be asked apart from one that is too old", async () => {
    roles = [
      memberRoles({ name: "DXP480T", canKeep: false, keep: "too-old" }),
      memberRoles({ memberId: "m2", name: "barn", canKeep: false, reachable: false, keep: "unreachable" }),
    ];
    await renderPage();
    expect(tile("m1", "keeper").textContent).toBe(en["instances.role.keeper"] + en["keeper.tile.tooOld"]);
    expect(tile("m2", "keeper").textContent).toBe(en["instances.role.keeper"] + en["keeper.tile.unreachable"]);
    expect(tile("m2", "keeper").getAttribute("data-state")).toBe("off");
  });

  it("asks for a sign-in at this instance when the phone app's session may not make a deposit", async () => {
    keeperRefusal = "own-login-needed";
    await renderPage();
    await click(tile("m1", "keeper"));
    await click(within(dialog()).getByRole("button", { name: en["keeper.make"] }));
    const ask = question();
    expect(ask.textContent).toContain(en["secrets.ownLoginNeeded"]);
    expect(within(ask).getByRole("button", { name: en["secrets.signInHere"] })).toBeTruthy();
    expect(within(dialog()).queryByText("the server's own words")).toBeNull();
  });

  it("says what is missing when no login password is set", async () => {
    keeperRefusal = "password-not-set";
    await renderPage();
    await click(tile("m1", "keeper"));
    await click(within(dialog()).getByRole("button", { name: en["keeper.make"] }));
    expect(within(dialog()).getByText(en["secrets.passwordNotSet"])).toBeTruthy();
  });

  it("says in two sentences what a key keeper is when the pointer rests on it", async () => {
    await renderPage();
    fireEvent.mouseEnter(tile("m1", "keeper"));
    expect(screen.getByRole("tooltip").textContent).toBe(en["instances.role.keeperTip"]);
    expect(en["instances.role.keeperTip"].match(/\.( |$)/g)).toHaveLength(2);
  });

  it("says what the deposit is and what the day it is used takes, before anything is sent", async () => {
    await renderPage();
    await click(tile("m1", "keeper"));
    expect(dialog().textContent).toContain("DXP480T gets a sealed copy of this server's recovery kit and cannot open it.");
    expect([...within(dialog()).getByTestId("keeper-needs").children].map((li) => li.textContent)).toEqual([
      en["keeper.need.words"],
      en["keeper.need.code"],
      "Access to DXP480T's interface, where someone has to press Allow",
    ]);
    expect(dialog().textContent).toContain("Without the code nobody can open the copy, DXP480T included.");
    await click(within(dialog()).getByRole("button", { name: en["common.cancel"] }));
    expect(keeperAsked).toEqual([]);
    expect(tile("m1", "keeper").getAttribute("data-state")).toBe("off");
  });

  it("shows the recovery code of a first deposit once, and does not close before it is written down", async () => {
    await renderPage();
    await click(tile("m1", "keeper"));
    await click(within(dialog()).getByRole("button", { name: en["keeper.make"] }));
    expect(keeperAsked).toEqual(["m1"]);
    const shown = screen.getByRole("dialog", { name: en["keeper.code.title"] });
    expect([...within(shown).getByTestId("recovery-code").children].map((li) => li.textContent)).toEqual(CODE.split("-"));
    expect(shown.textContent).toContain(en["keeper.code.lead"]);
    expect(shown.textContent).toContain(en["keeper.code.lost"]);
    const done = within(shown).getByRole("button", { name: en["common.done"] }) as HTMLButtonElement;
    expect(done.disabled).toBe(true);
    fireEvent.keyDown(document, { key: "Escape" });
    fireEvent.mouseDown(shown.closest(".glim-modal-backdrop")!);
    expect(screen.getByRole("dialog", { name: en["keeper.code.title"] })).toBeTruthy();
    await click(within(shown).getByRole("switch", { name: en["keeper.code.written"] }));
    expect(done.disabled).toBe(false);
    await click(done);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.body.textContent).not.toContain("K7QF2");
    expect(tile("m1", "keeper").getAttribute("data-state")).toBe("on");
  });

  it("makes a later deposit for the code this server has, and shows none", async () => {
    depositList = deposits({ codeSince: SINCE, sent: [sentDeposit({ memberId: "m2", name: "barn" })] });
    await renderPage();
    await click(tile("m1", "keeper"));
    expect(within(dialog()).getByTestId("keeper-needs").children[1].textContent).toBe(en["keeper.need.codeHave"]);
    await click(within(dialog()).getByRole("button", { name: en["keeper.make"] }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByText("DXP480T keeps this server's key deposit now.")).toBeTruthy();
    expect(tile("m1", "keeper").getAttribute("data-state")).toBe("on");
  });

  it("tells an instance that is too old to update first, and sends nothing", async () => {
    roles = [memberRoles({ name: "DXP480T", canKeep: false, keep: "too-old" })];
    await renderPage();
    expect(tile("m1", "keeper").getAttribute("data-state")).toBe("off");
    await click(tile("m1", "keeper"));
    expect(dialog().textContent).toContain("DXP480T runs a version that cannot keep a key deposit. Update it first.");
    expect(within(dialog()).queryByRole("button", { name: en["keeper.make"] })).toBeNull();
    await click(within(dialog()).getByRole("button", { name: en["common.close"] }));
    expect(keeperAsked).toEqual([]);
  });

  it("says why a deposit was refused, and stays open", async () => {
    keeperRefusal = "member-silent";
    await renderPage();
    await click(tile("m1", "keeper"));
    await click(within(dialog()).getByRole("button", { name: en["keeper.make"] }));
    expect(within(dialog()).getByText(en["deposit.code.member-silent"])).toBeTruthy();
    expect(tile("m1", "keeper").getAttribute("data-state")).toBe("off");
  });

  it("says in the server's words why a deposit may not be made, when the refusal has no code", async () => {
    keeperRefusal = "";
    await renderPage();
    await click(tile("m1", "keeper"));
    await click(within(dialog()).getByRole("button", { name: en["keeper.make"] }));
    expect(within(dialog()).getByText("the server's own words")).toBeTruthy();
  });

  it("shows a deposit in a state it does not know as one that is there, in the server's word", async () => {
    roles = [memberRoles({ name: "DXP480T", byMember: roleSide({ keeper: { state: "someday" as "held", at: "" } }) })];
    await renderPage();
    expect(tile("m1", "keeper").getAttribute("data-state")).toBe("waiting");
    expect(tile("m1", "keeper").textContent).toBe(en["instances.role.keeper"] + "someday");
  });

  it("takes the deposit back after saying that the last keeper takes the recovery code with it", async () => {
    roles = [keeps("held")];
    depositList = deposits({ codeSince: SINCE, sent: [sentDeposit({ name: "DXP480T" })] });
    await renderPage();
    await click(tile("m1", "keeper"));
    expect(question().textContent).toContain("Stop using DXP480T as Key keeper?");
    expect(question().textContent).toContain("This server asks DXP480T to delete the deposit.");
    expect(question().textContent).toContain("DXP480T is the only keeper.");
    expect(keeperWithdrawn).toEqual([]);
    await click(within(question()).getByRole("button", { name: en["instances.role.switchOff"] }));
    expect(keeperWithdrawn).toEqual(["m1"]);
    expect(tile("m1", "keeper").getAttribute("data-state")).toBe("off");
    expect(screen.getByText(en["keeper.codeGone"])).toBeTruthy();
  });

  it("does not speak of the code while another keeper still holds a deposit", async () => {
    roles = [keeps("held")];
    depositList = deposits({ codeSince: SINCE, sent: [sentDeposit({ name: "DXP480T" }), sentDeposit({ memberId: "m2", name: "barn" })] });
    await renderPage();
    await click(tile("m1", "keeper"));
    expect(question().textContent).toContain("This server asks DXP480T to delete the deposit.");
    expect(question().textContent).not.toContain("only keeper");
    await click(within(question()).getByRole("button", { name: en["instances.role.switchOff"] }));
    expect(keeperWithdrawn).toEqual(["m1"]);
    expect(screen.queryByText(en["keeper.codeGone"])).toBeNull();
  });

  it("says where to delete the deposit by hand when the keeper did not hear of it", async () => {
    roles = [keeps("held")];
    keeperTold = false;
    await renderPage();
    await click(tile("m1", "keeper"));
    await click(within(question()).getByRole("button", { name: en["instances.role.switchOff"] }));
    expect(screen.getByText("DXP480T could not be reached and still holds the deposit. Delete it there, under Instances, Key deposit.")).toBeTruthy();
  });

  it("is not part of the net picture, whose arrows are where the backups go", async () => {
    roles = [keeps("held")];
    await renderPage();
    expect(screen.getByTestId("net-picture").querySelectorAll(".glim-net-wide [data-lane]")).toHaveLength(0);
  });
});

describe("an instance from before role requests", () => {
  beforeEach(() => {
    roles = [memberRoles({ name: "DXP480T", answers: false })];
    askAnswer = { state: "allowed", decidedBy: "automatic" };
  });

  it("is told apart in the dialog: the role is active at once, and the button says so", async () => {
    await renderPage();
    await click(tile("m1", "receiver"));
    const dialog = screen.getByRole("dialog", { name: "DXP480T as Receiver" });
    expect(dialog.textContent).toContain("DXP480T runs an older BombVault that cannot answer a request.");
    expect(dialog.textContent).not.toContain("gets a request");
    expect(within(dialog).queryByRole("button", { name: en["instances.role.send"] })).toBeNull();
    await click(within(dialog).getByRole("button", { name: en["instances.role.switchOn"] }));
    expect(asked).toHaveLength(1);
    expect(screen.getByText("DXP480T is your Receiver now.")).toBeTruthy();
    expect(tile("m1", "receiver").getAttribute("data-state")).toBe("on");
  });

  it("cannot take ZFS replicas, which the dialog explains instead of asking", async () => {
    flags.zfsEnabled = true;
    await renderPage();
    await click(tile("m1", "zfs"));
    const dialog = screen.getByRole("dialog", { name: "DXP480T as ZFS server" });
    expect(dialog.textContent).toContain("Update it to BombVault 10");
    expect(within(dialog).queryByRole("switch")).toBeNull();
    expect(within(dialog).queryByRole("button", { name: en["instances.role.send"] })).toBeNull();
    await click(within(dialog).getByRole("button", { name: en["common.close"] }));
    expect(patched).toEqual([]);
  });
});

describe("the ZFS server role", () => {
  beforeEach(() => {
    flags.zfsEnabled = true;
  });

  it("points the picked ZFS entries at the instance, each asking on its own", async () => {
    await renderPage();
    await click(tile("m1", "zfs"));
    const dialog = screen.getByRole("dialog", { name: "DXP480T as ZFS server" });
    expect(dialog.textContent).toContain("Each ZFS entry asks DXP480T on its own.");
    expect(within(dialog).getByRole("switch", { name: "tank/appdata" }).getAttribute("aria-checked")).toBe("true");
    await click(within(dialog).getByRole("button", { name: en["instances.role.send"] }));
    expect(patched).toEqual([["z1", { target: { kind: "peer", id: "m1" } }]]);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("says where an entry replicates so far", async () => {
    servers = [server()];
    itemReplica = replica();
    await renderPage();
    await click(tile("m1", "zfs"));
    expect(screen.getByRole("dialog").textContent).toContain("Replicates to Backup-NAS so far");
  });

  it("points the entries away again when the role is switched off", async () => {
    roles = [memberRoles({ name: "DXP480T", byMember: roleSide({ zfs: zfsRole("allowed") }) })];
    itemReplica = replica({ target: { kind: "peer", id: "m1" }, peerState: "allowed" });
    await renderPage();
    await click(tile("m1", "zfs"));
    expect(question().textContent).toContain("No ZFS entry replicates to DXP480T any more.");
    await click(within(question()).getByRole("button", { name: en["instances.role.switchOff"] }));
    expect(patched).toEqual([["z1", { target: { kind: "none", id: "" } }]]);
  });
});

describe("a paired phone", () => {
  beforeEach(() => {
    members = [M1, groupMember({ id: "m3", name: "Pixel 8", kind: "android", address: "", direct: false, relay: true })];
    peers = [peer({}), peer({ id: "p3", memberId: "m3", name: "Pixel 8", kind: "android", direct: false, relay: true, lastPollDomains: [] })];
  });

  it("is a card of its own with the logo, and no roles, figures or details", async () => {
    await renderPage();
    const phone = card("m3");
    expect(phone.textContent).toContain("Pixel 8");
    expect(phone.textContent).toContain(en["fleet.kindAndroid"]);
    expect(phone.textContent).toContain(en["instances.connected"]);
    expect(phone.querySelector("[data-role]")).toBeNull();
    expect(phone.querySelector("img")?.getAttribute("src")).toBe("/logo.svg");
    expect(within(phone).queryByRole("button", { name: en["fleet.details"] })).toBeNull();
    expect(within(phone).getByRole("button", { name: en["fleet.remove"] })).toBeTruthy();
  });

  it("forgets the key pinned for it on the second click, so the phone pins its own on its next call", async () => {
    await renderPage();
    const phone = card("m3");
    await click(within(phone).getByRole("button", { name: en["instances.forgetKey"] }));
    expect(forgotten).toEqual([]);
    await click(within(phone).getByRole("button", { name: en["instances.forgetKeyConfirm"] }));
    expect(forgotten).toEqual(["m3"]);
    expect(within(card("m1")).queryByRole("button", { name: en["instances.forgetKey"] })).toBeNull();
  });

  it("stands in the net picture as a phone, with no arrows", async () => {
    await renderPage();
    const node = screen.getByTestId("net-picture").querySelector('.glim-net-wide [data-net="m3"]')!;
    expect(node.textContent).toContain(en["fleet.kindAndroid"]);
    expect(node.querySelector("image")).toBeNull();
    expect(node.querySelectorAll("[data-lane]")).toHaveLength(0);
  });
});

describe("the net picture above the grid", () => {
  it("stands before the cards and shows this server with every instance and ZFS server", async () => {
    flags.zfsEnabled = true;
    servers = [server()];
    await renderPage();
    const picture = screen.getByTestId("net-picture");
    expect(picture.compareDocumentPosition(card("self")) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const names = within(picture.querySelector(".glim-net-wide") as HTMLElement)
      .getAllByRole("button")
      .map((b) => b.getAttribute("aria-label"));
    expect(names).toEqual(["DXP480T", "Backup-NAS", "cellar"]);
  });

  it("draws the arrows of the roles the cards show, and a new one when a role is asked for", async () => {
    roles = [memberRoles({ name: "DXP480T", byMember: roleSide({ receiver: allowed() }), forMember: roleSide({ fetcher: roleRequest() }) })];
    await renderPage();
    const lanes = () =>
      [...screen.getByTestId("net-picture").querySelectorAll(".glim-net-wide [data-lane]")].map((l) => l.getAttribute("data-lane"));
    expect(lanes()).toEqual(["out:receiver:on", "in:fetcher:waiting"]);
    await click(tile("m1", "fetcher"));
    await click(within(screen.getByRole("dialog")).getByRole("button", { name: en["instances.role.send"] }));
    expect(lanes()).toEqual(["out:receiver:on", "out:fetcher:waiting", "in:fetcher:waiting"]);
  });

  it("lights up the card of the instance the pointer is on in the picture", async () => {
    await renderPage();
    const node = screen.getByTestId("net-picture").querySelector('.glim-net-wide [data-net="m1"] [role="button"]')!;
    fireEvent.pointerOver(node);
    expect(card("m1").className).toContain("glim-net-entry-hot");
    expect(card("self").className).not.toContain("glim-net-entry-hot");
  });

  it("leaves out a row from before pairing, which is in no group", async () => {
    peers = [peer({}), peer({ id: "old", memberId: "", name: "office", needsPairing: true })];
    await renderPage();
    expect(screen.getByTestId("net-picture").querySelector('[data-net="old"]')).toBeNull();
  });
});

describe("an empty group", () => {
  beforeEach(() => {
    peers = [];
    members = [];
    roles = [];
  });

  it("keeps this server's card and names the way to pair", async () => {
    await renderPage();
    expect(cards().map((c) => c.getAttribute("data-instance"))).toEqual(["self", "none"]);
    expect(card("none").textContent).toContain(en["instances.emptyTitle"]);
    expect(card("none").textContent).toContain(en["instances.emptyLead"]);
    expect(screen.queryByTestId("net-picture")).toBeNull();
    expect(within(card("none")).queryByRole("button")).toBeNull();
  });

  it("offers pairing once, as the page's action and not on the card", async () => {
    await renderPage();
    expect(screen.getAllByRole("button", { name: en["pairing.title"] })).toHaveLength(1);
    await click(within(screen.getByTestId("page-actions")).getByRole("button", { name: en["pairing.title"] }));
    expect(screen.getByTestId("where").textContent).toBe("/settings/pairing");
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
    await click(screen.getByRole("button", { name: en["receiver.edit"] }));
    const dialog = screen.getByRole("dialog", { name: en["receiver.editTitle"] });
    expect(within(dialog).getByText(en["receiver.member"])).toBeTruthy();
  });

  it("is no card while every row has its instance", async () => {
    received = [receivedRepo()];
    await renderPage();
    expect(screen.queryByText(en["instances.unplaced.title"])).toBeNull();
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
    await click(within(nas).getByRole("button", { name: en["instances.open"] }));
    expect(screen.getByTestId("where").textContent).toBe("/instances/zfs/nas");
  });

  it("stand before the phones, which come last", async () => {
    members = [groupMember({ id: "m3", name: "Pixel 8", kind: "android", address: "", direct: false, relay: true }), M1];
    peers = [peer({ id: "p3", memberId: "m3", name: "Pixel 8", kind: "android", direct: false, relay: true, lastPollDomains: [] }), peer({})];
    await renderPage();
    expect(cards().map((c) => c.getAttribute("data-instance"))).toEqual(["self", "m1", "zfs:nas", "m3"]);
  });

  it("say Connected when the server answered the last time it was asked", async () => {
    await renderPage();
    expect(within(card("zfs:nas")).getByText(en["instances.connected"])).toBeTruthy();
  });

  it("say Not connected when it did not answer", async () => {
    servers = [server({ reachable: false })];
    await renderPage();
    expect(within(card("zfs:nas")).getByText(en["instances.notConnected"])).toBeTruthy();
    expect(within(card("zfs:nas")).queryByText(en["instances.connected"])).toBeNull();
  });

  it("say Off for one that is switched off, whether it would answer or not", async () => {
    servers = [server({ enabled: false, reachable: false })];
    await renderPage();
    expect(within(card("zfs:nas")).getByText(en["zfs.replica.servers.off"])).toBeTruthy();
    expect(within(card("zfs:nas")).queryByText(en["instances.notConnected"])).toBeNull();
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
    roles = [memberRoles({ name: "DXP480T" }), memberRoles({ memberId: "m2", name: "barn", reachable: false, forMember: roleSide({ receiver: allowed() }) })];
    await renderPage();
    expect(called("listFleetPeers")).toBe(0);
    expect(called("listMeshOffers")).toBe(0);
    expect(called("listPullSources")).toBe(0);
    expect(called("listReceivedRepos")).toBe(1);
    expect(called("listRoles")).toBe(1);
    expect(cards().map((c) => c.getAttribute("data-instance"))).toEqual(["self", "m1", "m2"]);
    expect(within(card("m1")).queryByRole("button", { name: en["fleet.remove"] })).toBeNull();
    expect(within(card("m1")).getByRole("button", { name: en["fleet.details"] })).toBeTruthy();
  });

  it("asks only for the pull sources when only pulling is on", async () => {
    flags = { receiverEnabled: false, fleetEnabled: false, pullEnabled: true, zfsEnabled: false };
    serverRuns = true;
    pulls = [pullSource()];
    await renderPage();
    expect(called("listFleetPeers")).toBe(0);
    expect(called("listReceivedRepos")).toBe(0);
    expect(called("getReceiverServer")).toBe(0);
    expect(called("listPullSources")).toBe(1);
    expect(within(card("self")).queryByRole("button", { name: en["receiver.server.title"] })).toBeNull();
  });

  it("is only the way into pairing while all three are off", async () => {
    flags = { receiverEnabled: false, fleetEnabled: false, pullEnabled: false, zfsEnabled: false };
    await renderPage();
    expect(cards().map((c) => c.getAttribute("data-instance"))).toEqual(["none"]);
    expect(screen.getByText(en["instances.title"])).toBeTruthy();
    expect(screen.queryByTestId("net-picture")).toBeNull();
  });

  it("says so when the members could not be read", async () => {
    peersOk = false;
    await renderPage();
    expect(screen.getByText("the list is locked")).toBeTruthy();
    expect(card("self")).toBeTruthy();
  });
});

describe("keeping the grid current", () => {
  it("asks again who is connected and where the roles stand while the page is open, with no button for it", async () => {
    vi.useFakeTimers();
    await renderPage();
    const before = [called("listFleetPeers"), called("listRoles")];
    await act(async () => {
      await vi.advanceTimersByTimeAsync(FLEET_REFRESH_MS);
    });
    expect(called("listFleetPeers")).toBeGreaterThan(before[0]);
    expect(called("listRoles")).toBeGreaterThan(before[1]);
    expect(called("listDeposits")).toBe(2);
    expect(screen.queryByRole("button", { name: /poll/i })).toBeNull();
  });

  it("shows an answer the other side gave in the meantime", async () => {
    vi.useFakeTimers();
    roles = [memberRoles({ name: "DXP480T", byMember: roleSide({ receiver: roleRequest() }) })];
    await renderPage();
    expect(tile("m1", "receiver").getAttribute("data-state")).toBe("waiting");
    roles = [memberRoles({ name: "DXP480T", byMember: roleSide({ receiver: allowed() }) })];
    await act(async () => {
      await vi.advanceTimersByTimeAsync(FLEET_REFRESH_MS);
    });
    expect(tile("m1", "receiver").getAttribute("data-state")).toBe("on");
  });

  it("reads the roles again when a ZFS request was answered", async () => {
    requests = [receiveRequest({ peer: "m1" })];
    await renderPage();
    const before = called("listRoles");
    await click(within(card("m1")).getByRole("button", { name: en["fleet.details"] }));
    requests = [receiveRequest({ peer: "m1", state: "refused" })];
    vi.spyOn(await import("../lib/api"), "decideZFSReceiveRequest").mockResolvedValue({ ok: true });
    await click(within(screen.getByRole("dialog", { name: "DXP480T" })).getByRole("button", { name: en["zfs.receive.decline"] }));
    await act(async () => undefined);
    expect(called("listRoles")).toBeGreaterThan(before);
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

  it("opens the receiving server for #receiver and drops the hash, whether one runs yet or not", async () => {
    window.location.hash = "#receiver";
    await renderPage();
    const win = screen.getByRole("dialog", { name: en["receiver.server.title"] });
    expect(within(win).getByRole("button", { name: en["receiver.server.setUp"] })).toBeTruthy();
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
