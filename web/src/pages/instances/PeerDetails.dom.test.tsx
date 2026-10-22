// @vitest-environment jsdom
// The Details window of one instance: what it does for this server, with a
// switch per role, what this server does for it, with its requests to answer,
// its protection per section, whether it is polled, and the way to remove it.
// The key keeper is a role like the others on both sides: a row with a switch
// for the deposit this server gave the instance, and among the requests the
// one to hand a deposit out and the deposit this server keeps for it.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import { receiveRequest, replica } from "../../components/zfs/replica/replica.testsupport";
import type {
  Deposits,
  DomainStatus,
  MemberRoles,
  PullSourceInput,
  ReceivedRepoInput,
  RoleDecision,
  RoleRequest,
  ZFSReceiveRequest,
} from "../../lib/api";
import { I18nProvider, en } from "../../lib/i18n";
import { destination } from "../../lib/placement.testsupport";
import { ToastProvider } from "../../lib/toast";
import {
  depositRelease,
  deposits,
  fleetPeer,
  groupMember,
  keptDeposit,
  memberRoles,
  meshOffer,
  NOW_S,
  pullSource,
  receivedRepo,
  roleRequest,
  roleSide,
  sentDeposit,
  zfsRole,
} from "./instances.testsupport";
import { buildInstances, type InstancesData } from "./instancesModel";
import type { InstancesState, Modules } from "./useInstances";

const domain = (name: string, protection: string): DomainStatus =>
  ({ domain: name, enabled: true, protection, lastSuccess: NOW_S - 2 * 3600 }) as DomainStatus;

const PEER = fleetPeer({
  lastPollDomains: [domain("containers", "green"), domain("vms", "red"), domain("config", "green")],
});
const MEMBER = groupMember();

let requests: ZFSReceiveRequest[] = [];
let serverRuns = false;
let depositList: Deposits = deposits();
const keeperCalls: string[] = [];
let decideAnswer: (id: string, decision: RoleDecision) => { ok: boolean; code?: string; error?: string } = () => ({ ok: true });
const decided: [string, RoleDecision][] = [];
const asked: [string, string, unknown][] = [];
const withdrawn: [string, string][] = [];
const serversSetUp: unknown[] = [];
const checks: [string, string][] = [];
const polledSet: [string, boolean][] = [];
const removed: string[] = [];
const createdRepos: ReceivedRepoInput[] = [];
const createdSources: PullSourceInput[] = [];
const offered: [string, string, string][] = [];

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getGroup: () => Promise.resolve({ ok: true, active: true, name: "cellar", members: [MEMBER], away: [], settling: false }),
    memberRepos: () => Promise.resolve({ ok: true, repos: [] }),
    listCloudCredSets: () => Promise.resolve({ ok: true, sets: [] }),
    listZFSReceiveRequests: () => Promise.resolve(requests),
    listZFSLocalPools: () => Promise.resolve([{ name: "tank", sizeBytes: 1, freeBytes: 1 }]),
    listZFSReplicaServers: () => Promise.resolve([]),
    decideRole: (id: string, decision: RoleDecision) => {
      decided.push([id, decision]);
      return Promise.resolve(decideAnswer(id, decision));
    },
    askRole: (memberId: string, role: string, ask: unknown) => {
      asked.push([memberId, role, ask]);
      return Promise.resolve({ ok: true, request: roleRequest() });
    },
    withdrawRole: (memberId: string, role: string) => {
      withdrawn.push([memberId, role]);
      return Promise.resolve({ ok: true, told: true });
    },
    askKeeper: (memberId: string) => {
      keeperCalls.push(`ask:${memberId}`);
      return Promise.resolve({ ok: true, keeper: { state: "held", at: "" } });
    },
    withdrawKeeper: (memberId: string) => {
      keeperCalls.push(`withdraw:${memberId}`);
      return Promise.resolve({ ok: true, told: true });
    },
    decideRelease: (id: string, decision: string) => {
      keeperCalls.push(`${decision}:${id}`);
      return Promise.resolve({ ok: true, state: decision === "allow" ? "allowed" : "declined" });
    },
    dropKeptDeposit: (ownerId: string) => {
      keeperCalls.push(`drop:${ownerId}`);
      return Promise.resolve({ ok: true });
    },
    getReceiverServer: () =>
      Promise.resolve({
        ok: true,
        server: serverRuns ? { logins: [], check: "protected", present: true, checkedAt: 0, url: "", user: "outside", hostPath: "/mnt/user/restic" } : null,
        defaultPort: 8000,
        hostMountRoot: "/host/user",
      }),
    setUpReceiverServer: (input: unknown) => {
      serversSetUp.push(input);
      serverRuns = true;
      return Promise.resolve({
        ok: true,
        server: { logins: [], check: "protected", present: true, checkedAt: 0, url: "", user: "outside", hostPath: "/mnt/user/restic" },
        password: "kE7v-Qm2d",
        template: "none",
      });
    },
    browseDirs: () => Promise.resolve({ ok: true, path: "", parent: "", dirs: [] }),
    checkFleetPeer: (id: string, d: string) => {
      checks.push([id, d]);
      return Promise.resolve({ ok: true });
    },
    updateFleetPeer: (id: string, input: { enabled?: boolean }) => {
      polledSet.push([id, input.enabled ?? false]);
      return Promise.resolve({ ok: true });
    },
    deleteFleetPeer: (id: string) => {
      removed.push(id);
      return Promise.resolve({ ok: true });
    },
    createReceivedRepo: (input: ReceivedRepoInput) => {
      createdRepos.push(input);
      return Promise.resolve({ ok: true });
    },
    createPullSource: (input: PullSourceInput) => {
      createdSources.push(input);
      return Promise.resolve({ ok: true });
    },
    proposeMeshOffer: (peerId: string, d: string, baseUrl: string) => {
      offered.push([peerId, d, baseUrl]);
      return Promise.resolve({ ok: true, snippet: { dockerRun: "docker run rest", compose: "services:", repo: "rest:x" } });
    },
  };
});

const { PeerDetails } = await import("./PeerDetails");

const ALL: Modules = { fleet: true, receiver: true, pull: true, zfs: true };
const reloads: string[] = [];
let closed = 0;

function data(over: Partial<InstancesData> = {}): InstancesData {
  return {
    members: [MEMBER],
    away: [],
    settling: false,
    peers: [PEER],
    received: [],
    pulls: [],
    offers: [],
    roles: [],
    destinations: [],
    groupReceivers: [],
    replicas: [],
    ...over,
  };
}

async function open(over: Partial<InstancesData> = {}, modules: Modules = ALL) {
  const all = data(over);
  const reload = (name: string) => () => {
    reloads.push(name);
    return Promise.resolve();
  };
  const state: InstancesState = {
    loading: false,
    errors: [],
    self: null,
    selfRoles: { receiver: [], fetcher: [], zfs: [], keeper: [] },
    receiverServer: serverRuns,
    deposits: depositList,
    data: all,
    reloadGroup: reload("group"),
    reloadPeers: reload("peers"),
    reloadOffers: reload("offers"),
    reloadReceived: reload("received"),
    reloadPulls: reload("pulls"),
    reloadRoles: reload("roles"),
    reloadDeposits: reload("deposits"),
    reloadReplicas: reload("replicas"),
    reloadServer: reload("server"),
  };
  const instance = buildInstances(all).instances[0];
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <PeerDetails
            instance={instance}
            peers={[instance]}
            modules={modules}
            sections={["containers", "flash"]}
            state={state}
            onClose={() => closed++}
          />
        </ToastProvider>
      </I18nProvider>,
    );
  });
  return screen.getByRole("dialog", { name: instance.name });
}

function section(name: string): HTMLElement {
  return screen.getByRole("region", { name });
}

const does = () => section("What attic does for this server");
const serves = () => section("What this server does for attic");
const row = (role: string) => does().querySelector(`[data-role="${role}"]`) as HTMLElement;
const by = (over: Parameters<typeof roleSide>[0]): MemberRoles[] => [memberRoles({ byMember: roleSide(over) })];
const forPeer = (over: Parameters<typeof roleSide>[0], rest: Partial<MemberRoles> = {}): MemberRoles[] => [
  memberRoles({ forMember: roleSide(over), ...rest }),
];
const allowed = (over: Partial<RoleRequest> = {}) =>
  roleRequest({ state: "allowed", decidedBy: "person", decidedAt: new Date((NOW_S - 3600) * 1000).toISOString(), ...over });

async function click(el: Element) {
  await act(async () => {
    fireEvent.click(el);
  });
}

beforeEach(() => {
  requests = [];
  serverRuns = false;
  depositList = deposits();
  keeperCalls.length = 0;
  decideAnswer = () => ({ ok: true });
  decided.length = 0;
  asked.length = 0;
  withdrawn.length = 0;
  serversSetUp.length = 0;
  checks.length = 0;
  polledSet.length = 0;
  removed.length = 0;
  createdRepos.length = 0;
  createdSources.length = 0;
  offered.length = 0;
  reloads.length = 0;
  closed = 0;
  localStorage.clear();
});

afterEach(cleanup);

describe("the head of the window", () => {
  it("names the instance with where it is reached, its version and its last poll", async () => {
    const win = await open();
    expect(win.textContent).toContain("192.168.1.21:3443 · v9.9.1 · Last polled 2 minutes ago");
  });

  it("closes on Done", async () => {
    await open();
    fireEvent.click(screen.getByRole("button", { name: en["common.done"] }));
    expect(closed).toBe(1);
  });
});

describe("what the instance does for this server", () => {
  it("lists the four roles, each with a switch, and explains how they differ", async () => {
    await open({ roles: by({ receiver: allowed(), fetcher: roleRequest() }) });
    expect(within(does()).getByLabelText(en["instances.role.diff"])).toBeTruthy();
    const rows = [...does().querySelectorAll("[data-role]")];
    expect(rows.map((r) => [r.getAttribute("data-role"), r.getAttribute("data-state")])).toEqual([
      ["receiver", "on"],
      ["fetcher", "waiting"],
      ["zfs", "off"],
      ["keeper", "off"],
    ]);
    const switches = within(does()).getAllByRole("switch");
    expect(switches.map((sw) => [sw.getAttribute("aria-label"), sw.getAttribute("aria-checked")])).toEqual([
      [en["receiver.title"], "true"],
      [en["instances.role.fetcher"], "true"],
      [en["zfs.replica.servers.title"], "false"],
      [en["instances.role.keeper"], "false"],
    ]);
    expect(within(does()).getByLabelText(en["instances.role.receiverTip"])).toBeTruthy();
    expect(within(does()).getByLabelText(en["instances.role.keeperTip"])).toBeTruthy();
  });

  it("says what a role covers, and for one that waits, whom it waits for", async () => {
    await open({ roles: by({ receiver: allowed(), fetcher: roleRequest({ sections: ["vms"] }) }) });
    expect(row("receiver").textContent).toContain("Containers, Flash");
    expect(row("fetcher").textContent).toContain("Waiting for attic to allow it · VMs");
    expect(row("zfs").textContent).toContain(en["zfs.replica.servers.off"]);
  });

  it("says when the instance declined a request or took a role back", async () => {
    await open({ roles: by({ receiver: roleRequest({ state: "refused" }), fetcher: roleRequest({ state: "revoked" }) }) });
    expect(row("receiver").textContent).toContain("attic declined the request");
    expect(row("receiver").getAttribute("data-state")).toBe("off");
    expect(row("fetcher").textContent).toContain("attic revoked access");
  });

  it("says why a role is active that nobody there allowed", async () => {
    await open({ roles: by({ receiver: allowed({ decidedBy: "automatic" }) }) });
    expect(row("receiver").textContent).toContain("active without a request, because attic runs an older BombVault");
  });

  it("names the storage location its receiving server became", async () => {
    await open({
      roles: by({ receiver: allowed() }),
      groupReceivers: [{ memberId: "m1", name: "attic", url: "http://192.168.1.21:8000", needsAddress: false }],
      destinations: [destination({ name: "Attic off-site", repo: "rest:http://192.168.1.21:8000/cellar-login" })],
    });
    expect(row("receiver").textContent).toContain("Storage location Attic off-site");
  });

  it("lists the ZFS items that replicate to it, each with where it stands", async () => {
    await open({
      roles: by({ zfs: zfsRole("asked", ["tank/appdata", "tank/media"]) }),
      replicas: [
        { itemId: "z1", dataset: "tank/appdata", replica: replica({ target: { kind: "peer", id: "m1" }, peerState: "allowed" }) },
        { itemId: "z2", dataset: "tank/media", replica: replica({ target: { kind: "peer", id: "m1" }, peerState: "asked", state: "waiting" }) },
      ],
    });
    expect(row("zfs").getAttribute("data-state")).toBe("waiting");
    expect(row("zfs").textContent).toContain("tank/appdata");
    expect(row("zfs").textContent).toContain("Replicated 2 hours ago");
    expect(row("zfs").textContent).toContain("tank/media");
  });

  it("leaves out the ZFS server while this server has no ZFS section", async () => {
    await open({}, { fleet: true, receiver: true, pull: true, zfs: false });
    expect([...does().querySelectorAll("[data-role]")].map((r) => r.getAttribute("data-role"))).toEqual(["receiver", "fetcher", "keeper"]);
  });

  it("says where this server's key deposit stands there, and what to do when it is behind or gone", async () => {
    const keeps = (state: "held" | "stale" | "declined") => by({ keeper: { state, at: new Date((NOW_S - 2 * 3600) * 1000).toISOString() } });
    await open({ roles: keeps("held") });
    expect(row("keeper").getAttribute("data-state")).toBe("on");
    expect(row("keeper").textContent).toContain("keeps this server's key deposit · sent 2 hours ago");
    expect(within(row("keeper")).queryByRole("button", { name: en["instances.role.change"] })).toBeNull();
    cleanup();
    await open({ roles: keeps("stale") });
    expect(row("keeper").getAttribute("data-state")).toBe("waiting");
    expect(row("keeper").textContent).toContain("sent 2 hours ago · attic still holds an older deposit.");
    cleanup();
    await open({ roles: keeps("declined") });
    expect(row("keeper").getAttribute("data-state")).toBe("off");
    expect(row("keeper").textContent).toContain("The deposit was deleted on attic. attic takes a new one only when you ask again.");
  });

  it("says that an instance too old to keep a deposit has to be updated first", async () => {
    await open({ roles: [memberRoles({ canKeep: false, keep: "too-old" })] });
    expect(row("keeper").textContent).toContain("attic runs a version that cannot keep a key deposit. Update it first.");
  });

  it("makes a deposit from the keeper's switch, and takes it back the same way", async () => {
    await open();
    await click(within(row("keeper")).getByRole("switch"));
    await click(within(screen.getByRole("dialog", { name: "attic as Key keeper" })).getByRole("button", { name: en["keeper.make"] }));
    expect(keeperCalls).toEqual(["ask:m1"]);
    expect(reloads).toContain("deposits");
    cleanup();
    depositList = deposits({ sent: [sentDeposit()] });
    await open({ roles: by({ keeper: { state: "held", at: "" } }) });
    await click(within(row("keeper")).getByRole("switch"));
    const question = screen.getByRole("dialog", { name: en["confirmDialog.title"] });
    await click(within(question).getByRole("button", { name: en["instances.role.switchOff"] }));
    expect(keeperCalls).toEqual(["ask:m1", "withdraw:m1"]);
  });

  it("asks for a role from its switch", async () => {
    await open();
    await click(within(row("fetcher")).getByRole("switch"));
    const dialog = screen.getByRole("dialog", { name: "attic as Fetcher" });
    await click(within(dialog).getByRole("button", { name: en["instances.role.send"] }));
    expect(asked).toEqual([["m1", "fetcher", { sections: ["containers"] }]]);
    expect(reloads).toContain("roles");
  });

  it("withdraws a role from its switch after the question", async () => {
    await open({ roles: by({ receiver: allowed() }) });
    await click(within(row("receiver")).getByRole("switch"));
    const question = screen.getByRole("dialog", { name: en["confirmDialog.title"] });
    await click(within(question).getByRole("button", { name: en["instances.role.switchOff"] }));
    expect(withdrawn).toEqual([["m1", "receiver"]]);
    expect(reloads).toContain("roles");
  });

  it("changes the sections of a role that is asked for or on, and offers no change for one that is off", async () => {
    await open({ roles: by({ receiver: allowed({ sections: ["flash"] }) }) });
    expect(within(row("fetcher")).queryByRole("button", { name: en["instances.role.change"] })).toBeNull();
    await click(within(row("receiver")).getByRole("button", { name: en["instances.role.change"] }));
    const dialog = screen.getByRole("dialog", { name: "attic as Receiver" });
    expect(within(dialog).getAllByRole("switch").map((sw) => sw.getAttribute("aria-checked"))).toEqual(["false", "true"]);
    await click(within(dialog).getAllByRole("switch")[0]);
    await click(within(dialog).getByRole("button", { name: en["settings.save"] }));
    expect(asked).toEqual([["m1", "receiver", { sections: ["flash", "containers"], store: "rest" }]]);
    expect(await screen.findByText(en["folders.saved"])).toBeTruthy();
  });

  it("keeps the place of a Receiver that is on", async () => {
    await open({ roles: by({ receiver: allowed({ store: "share" }) }) });
    await click(within(row("receiver")).getByRole("button", { name: en["instances.role.change"] }));
    const dialog = screen.getByRole("dialog", { name: "attic as Receiver" });
    const choices = within(dialog).getAllByRole("radio");
    expect(choices.map((c) => [c.getAttribute("aria-checked"), (c as HTMLButtonElement).disabled])).toEqual([
      ["false", true],
      ["true", true],
    ]);
    await click(within(dialog).getByRole("button", { name: en["settings.save"] }));
    expect(asked[0][2]).toMatchObject({ store: "share" });
  });

  it("lets a request that still waits name another place", async () => {
    await open({ roles: by({ receiver: roleRequest() }) });
    await click(within(row("receiver")).getByRole("button", { name: en["instances.role.change"] }));
    const dialog = screen.getByRole("dialog", { name: "attic as Receiver" });
    await click(within(dialog).getByRole("radio", { name: en["rcloneRemote.share"] }));
    await click(within(dialog).getByRole("button", { name: en["settings.save"] }));
    expect(asked[0][2]).toMatchObject({ store: "share" });
  });

  it("shows the storage it offered, to accept or decline", async () => {
    await open({
      offers: [meshOffer()],
    });
    expect(within(does()).getByText("rest:http://192.168.1.21:8000/vms")).toBeTruthy();
    expect(within(does()).getByRole("button", { name: en["fleet.mesh.accept"] })).toBeTruthy();
    expect(within(does()).getByRole("button", { name: en["fleet.mesh.decline"] })).toBeTruthy();
  });
});

describe("requests from the instance", () => {
  it("shows what it asks for, marked as a request, with Allow and Decline", async () => {
    await open({ roles: forPeer({ receiver: roleRequest(), fetcher: roleRequest({ id: "rr2", sections: ["vms"] }) }, { asks: 2 }) });
    const asks = [...serves().querySelectorAll("[data-request]")];
    expect(asks.map((a) => a.getAttribute("data-request"))).toEqual(["receiver", "fetcher"]);
    expect(asks[0].textContent).toContain(en["instances.request"]);
    expect(asks[0].textContent).toContain("attic wants to keep its off-site copies here: Containers, Flash");
    expect(asks[1].textContent).toContain("attic wants this server to fetch its backups: VMs");
    expect(within(asks[0] as HTMLElement).getByText(en["instances.request"]).tagName).toBe("SPAN");
  });

  it("says when a Receiver is asked to keep the copies on a share", async () => {
    await open({ roles: forPeer({ receiver: roleRequest({ store: "share" }) }, { asks: 1 }) });
    const ask = serves().querySelector('[data-request="receiver"]')!;
    expect(ask.textContent).toContain(`Containers, Flash · ${en["rcloneRemote.share"]}`);
  });

  it("allows a Fetcher with the sections it showed, then asks what to fetch", async () => {
    await open({ roles: forPeer({ fetcher: roleRequest({ sections: ["vms"] }) }) });
    await click(within(serves()).getByRole("button", { name: en["zfs.receive.allow"] }));
    expect(decided).toEqual([["rr1", { decision: "allow", sections: ["vms"] }]]);
    expect(reloads).toContain("roles");
    expect(screen.getByText("Allowed for attic.")).toBeTruthy();
    expect(await screen.findByRole("dialog", { name: en["pull.addSource"] })).toBeTruthy();
  });

  it("declines a request", async () => {
    await open({ roles: forPeer({ receiver: roleRequest() }) });
    await click(within(serves()).getByRole("button", { name: en["zfs.receive.decline"] }));
    expect(decided).toEqual([["rr1", { decision: "refuse" }]]);
    expect(screen.getByText("attic gets no access.")).toBeTruthy();
    expect(screen.queryByRole("dialog", { name: en["pull.addSource"] })).toBeNull();
  });

  it("leads to setting up a receiving server when a Receiver is allowed without one, then allows", async () => {
    decideAnswer = () => (serverRuns ? { ok: true } : { ok: false, code: "no-receiver", error: "set up a receiver first" });
    await open({ roles: forPeer({ receiver: roleRequest() }) });
    await click(within(serves()).getByRole("button", { name: en["zfs.receive.allow"] }));
    expect(screen.queryByText("set up a receiver first")).toBeNull();
    const form = await screen.findByRole("dialog", { name: en["receiver.server.setUp"] });
    expect(form.textContent).toContain(en["receiver.server.neededLead"]);
    fireEvent.change(within(form).getByPlaceholderText("user/restic"), { target: { value: "user/restic" } });
    await click(within(form).getByRole("button", { name: en["receiver.server.startContinue"] }));
    expect(serversSetUp).toHaveLength(1);
    // The password of the new server shows once, so its window stays until it is closed.
    const win = screen.getByRole("dialog", { name: en["receiver.server.title"] });
    expect(within(win).getByText("kE7v-Qm2d")).toBeTruthy();
    await click(within(win).getByRole("button", { name: en["common.done"] }));
    expect(decided).toEqual([
      ["rr1", { decision: "allow", sections: ["containers", "flash"] }],
      ["rr1", { decision: "allow", sections: ["containers", "flash"] }],
    ]);
    expect(reloads).toContain("server");
    expect(screen.getByText("Allowed for attic.")).toBeTruthy();
  });

  it("does not allow when the set-up is left without a server", async () => {
    decideAnswer = () => ({ ok: false, code: "no-receiver", error: "set up a receiver first" });
    await open({ roles: forPeer({ receiver: roleRequest() }) });
    await click(within(serves()).getByRole("button", { name: en["zfs.receive.allow"] }));
    const form = await screen.findByRole("dialog", { name: en["receiver.server.setUp"] });
    await click(within(form).getByRole("button", { name: en["common.cancel"] }));
    await click(within(screen.getByRole("dialog", { name: en["receiver.server.title"] })).getByRole("button", { name: en["common.done"] }));
    expect(decided).toHaveLength(1);
  });

  it("reads the request again when the instance has asked for other sections since", async () => {
    decideAnswer = () => ({ ok: false, code: "request-changed", error: "asked for other sections" });
    await open({ roles: forPeer({ receiver: roleRequest() }) });
    await click(within(serves()).getByRole("button", { name: en["zfs.receive.allow"] }));
    expect(screen.getByText("attic has since asked for other sections. Look at the request again.")).toBeTruthy();
    expect(reloads).toContain("roles");
  });

  it("keeps an allowed role with when it was allowed, and takes it back after a question", async () => {
    await open({ roles: forPeer({ receiver: allowed() }) });
    const grant = serves().querySelector('[data-request="receiver"]') as HTMLElement;
    expect(grant.textContent).toContain("Allowed 1 hour ago · Containers, Flash");
    expect(grant.textContent).not.toContain(en["instances.request"]);
    await click(within(grant).getByRole("button", { name: en["zfs.receive.revoke"] }));
    const question = screen.getByRole("dialog", { name: en["confirmDialog.title"] });
    expect(question.textContent).toContain("Revoke access for attic?");
    expect(question.textContent).toContain("attic then cannot copy anything here any more.");
    await click(within(question).getByRole("button", { name: en["zfs.receive.revokeConfirm"] }));
    expect(decided).toEqual([["rr1", { decision: "revoke" }]]);
    expect(screen.getByText(en["zfs.receive.revoked"])).toBeTruthy();
  });

  it("shows its request to hand out a key deposit with the others, to allow or to refuse", async () => {
    depositList = deposits({
      releases: [
        depositRelease({ requesterId: "m1", requesterName: "attic", ownerId: "m4", ownerName: "barn" }),
        depositRelease({ id: "rel2", requesterId: "m7", requesterName: "shed" }),
      ],
    });
    await open({ roles: forPeer({ receiver: roleRequest() }, { asks: 2 }) });
    const asks = [...serves().querySelectorAll("[data-request], [data-release]")];
    expect(asks.map((a) => a.getAttribute("data-request") ?? a.getAttribute("data-release"))).toEqual(["receiver", "rel1"]);
    const ask = asks[1] as HTMLElement;
    expect(ask.textContent).toContain(en["instances.request"]);
    expect(ask.textContent).toContain("attic asks for the key deposit of barn.");
    expect(ask.textContent).toContain(en["keeper.release.unknown"]);
    await click(within(ask).getByRole("button", { name: en["zfs.receive.allow"] }));
    expect(keeperCalls).toEqual(["allow:rel1"]);
    expect(reloads).toContain("deposits");
  });

  it("shows the deposit this server keeps for it with what was allowed, and deletes it after a question", async () => {
    depositList = deposits({ kept: [keptDeposit(), keptDeposit({ ownerId: "m4", ownerName: "barn" })] });
    await open({ roles: forPeer({ keeper: { state: "held", at: "" } }) });
    const rows = [...serves().querySelectorAll("[data-kept]")] as HTMLElement[];
    expect(rows.map((r) => r.getAttribute("data-kept"))).toEqual(["m1"]);
    expect(rows[0].textContent).toContain("Key deposit of attic");
    await click(within(rows[0]).getByRole("button", { name: en["common.delete"] }));
    const question = screen.getByRole("dialog", { name: en["confirmDialog.title"] });
    expect(question.textContent).toContain("Delete the key deposit of attic?");
    await click(within(question).getByRole("button", { name: en["common.delete"] }));
    expect(keeperCalls).toEqual(["drop:m1"]);
  });

  it("reads as one list with the ZFS requests: what waits first, then what was allowed", async () => {
    requests = [
      receiveRequest({ peer: "m1", peerName: "attic", sourceServer: "attic" }),
      receiveRequest({ id: "rq2", peer: "m1", peerName: "attic", item: "cache/media", state: "allowed", pool: "tank", root: "tank/replica" }),
    ];
    await open({ roles: forPeer({ receiver: roleRequest(), fetcher: allowed({ id: "rr2" }) }) });
    await screen.findByText("attic wants to replicate cache/appdata here");
    const text = serves().textContent ?? "";
    const order = [
      "attic wants to keep its off-site copies here",
      "attic wants to replicate cache/appdata here",
      "Allowed 1 hour ago",
      "cache/media from attic",
    ].map((part) => text.indexOf(part));
    expect(order.every((at) => at >= 0)).toBe(true);
    expect(order).toEqual([...order].sort((a, b) => a - b));
  });
});

describe("what this server does for the instance", () => {
  it("shows its ZFS requests and no other instance's", async () => {
    requests = [receiveRequest({ peer: "m1", peerName: "attic", sourceServer: "attic" }), receiveRequest({ id: "rq2", peer: "m2", peerName: "barn", item: "tank/barn" })];
    await open();
    const card = serves();
    expect(await within(card).findByText("attic wants to replicate cache/appdata here")).toBeTruthy();
    expect(within(card).queryByText(/barn wants/)).toBeNull();
  });

  it("shows the repositories it receives from the instance and the sources it pulls from it", async () => {
    await open({
      received: [receivedRepo({ snapshotCount: 3 })],
      pulls: [pullSource({ lastPullOk: true, lastPullAt: NOW_S - 60, snapshotsPulled: 2 })],
    });
    const card = section("What this server does for attic");
    expect(within(card).getByText("Attic off-site")).toBeTruthy();
    expect(within(card).getByText("Attic containers")).toBeTruthy();
    expect(within(card).getByRole("button", { name: en["pull.pullNow"] })).toBeTruthy();
  });

  it("adds a received repository for this instance without asking which one", async () => {
    await open();
    fireEvent.click(screen.getByRole("button", { name: en["receiver.addRepo"] }));
    const dialog = await screen.findByRole("dialog", { name: en["receiver.addTitle"] });
    fireEvent.change(within(dialog).getByPlaceholderText("tower off-site"), { target: { value: "Attic copies" } });
    fireEvent.change(within(dialog).getByPlaceholderText(/^rest:http/), { target: { value: "/mnt/user/restic/attic" } });
    await act(async () => {
      fireEvent.click(within(dialog).getByRole("button", { name: en["settings.save"] }));
    });
    expect(createdRepos).toHaveLength(1);
    expect(createdRepos[0]).toMatchObject({ name: "Attic copies", repo: "/mnt/user/restic/attic", memberId: "m1" });
    expect(reloads).toContain("received");
  });

  it("adds a pull source for this instance without asking which one", async () => {
    await open();
    fireEvent.click(screen.getByRole("button", { name: en["pull.addSource"] }));
    const dialog = await screen.findByRole("dialog", { name: en["pull.addSource"] });
    fireEvent.change(within(dialog).getByPlaceholderText("tower next door"), { target: { value: "Attic containers" } });
    fireEvent.change(within(dialog).getByPlaceholderText(/^rest:http/), { target: { value: "rest:http://192.168.1.21:8000/attic" } });
    await act(async () => {
      fireEvent.click(within(dialog).getByRole("button", { name: en["settings.save"] }));
    });
    expect(createdSources).toHaveLength(1);
    expect(createdSources[0]).toMatchObject({ name: "Attic containers", memberId: "m1" });
    expect(reloads).toContain("pulls");
  });

  it("offers this server's storage to the instance at the address typed in", async () => {
    await open();
    fireEvent.click(screen.getByRole("button", { name: en["fleet.mesh.proposeButton"] }));
    const dialog = await screen.findByRole("dialog", { name: en["fleet.mesh.proposeTitle"] });
    fireEvent.change(within(dialog).getByRole("textbox"), { target: { value: "http://192.168.1.20:8000" } });
    await act(async () => {
      fireEvent.click(within(dialog).getByRole("button", { name: en["fleet.mesh.send"] }));
    });
    expect(offered).toEqual([["p1", "containers", "http://192.168.1.20:8000"]]);
    expect(within(dialog).getByText("docker run rest")).toBeTruthy();
  });

  it("offers only what the switched-on modules can do", async () => {
    await open({}, { fleet: false, receiver: true, pull: false, zfs: false });
    expect(screen.getByRole("button", { name: en["receiver.addRepo"] })).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["pull.addSource"] })).toBeNull();
    expect(screen.queryByRole("button", { name: en["fleet.mesh.proposeButton"] })).toBeNull();
  });
});

describe("protection per section", () => {
  it("shows each section's protection and asks the instance to check one now", async () => {
    await open();
    const card = section(en["instances.protection"]);
    const rows = [...card.querySelectorAll("[data-domain]")];
    expect(rows.map((r) => r.getAttribute("data-domain"))).toEqual(["containers", "vms", "config"]);
    expect(rows[0].textContent).toContain(en["fleet.protection.green"]);
    expect(rows[1].textContent).toContain(en["fleet.protection.red"]);
    expect(rows[0].textContent).toContain("last backup 2 hours ago");
    await act(async () => {
      fireEvent.click(within(rows[1] as HTMLElement).getByRole("button", { name: en["fleet.checkNow"] }));
    });
    expect(checks).toEqual([["p1", "vms"]]);
    expect(await screen.findByText(en["fleet.checkStarted"])).toBeTruthy();
  });

  it("offers no check for the self-backup, which has no repository check of its own", async () => {
    await open();
    const config = section(en["instances.protection"]).querySelector('[data-domain="config"]') as HTMLElement;
    expect(within(config).queryByRole("button")).toBeNull();
  });

  it("says when the instance has reported nothing yet", async () => {
    await open({ peers: [{ ...PEER, lastPollDomains: [] }] });
    expect(within(section(en["instances.protection"])).getByText(en["fleet.noScorecard"])).toBeTruthy();
  });

  it("shows why the last poll failed", async () => {
    await open({ peers: [{ ...PEER, lastPollOk: false, lastPollError: "connection refused" }] });
    expect(within(section(en["instances.protection"])).getByText("connection refused")).toBeTruthy();
  });
});

describe("monitoring and removal", () => {
  it("switches polling off and reads the members again", async () => {
    await open();
    const toggle = within(section(en["instances.monitoring"])).getByRole("switch", { name: en["fleet.enabledLabel"] });
    expect(within(section(en["instances.monitoring"])).getByLabelText(en["instances.pollHint"])).toBeTruthy();
    await act(async () => {
      fireEvent.click(toggle);
    });
    expect(polledSet).toEqual([["p1", false]]);
    expect(reloads).toContain("peers");
  });

  it("removes the instance only on the second click, then closes", async () => {
    await open();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["fleet.remove"] }));
    });
    expect(removed).toEqual([]);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["fleet.confirmRemove"] }));
    });
    expect(removed).toEqual(["p1"]);
    expect(closed).toBe(1);
    expect(reloads).toContain("peers");
    expect(reloads).toContain("group");
  });

  it("has neither while the Instances module is off", async () => {
    await open({ peers: [] }, { fleet: false, receiver: true, pull: true, zfs: false });
    expect(screen.queryByRole("region", { name: en["instances.protection"] })).toBeNull();
    expect(screen.queryByRole("region", { name: en["instances.monitoring"] })).toBeNull();
    expect(screen.queryByRole("button", { name: en["fleet.remove"] })).toBeNull();
  });
});
