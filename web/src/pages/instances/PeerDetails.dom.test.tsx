// @vitest-environment jsdom
// The Details window of one instance: what it does for this server, what
// this server does for it, its protection per section, whether it is polled,
// and the way to remove it.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import { receiveRequest, replica } from "../../components/zfs/replica/replica.testsupport";
import type { DomainStatus, PullSourceInput, ReceivedRepoInput, ZFSReceiveRequest } from "../../lib/api";
import { I18nProvider, en } from "../../lib/i18n";
import { destination } from "../../lib/placement.testsupport";
import { ToastProvider } from "../../lib/toast";
import { fleetPeer, groupMember, meshOffer, NOW_S, pullSource, receivedRepo } from "./instances.testsupport";
import { buildInstances, type InstancesData } from "./instancesModel";
import type { InstancesState, Modules } from "./useInstances";

const domain = (name: string, protection: string): DomainStatus =>
  ({ domain: name, enabled: true, protection, lastSuccess: NOW_S - 2 * 3600 }) as DomainStatus;

const PEER = fleetPeer({
  lastPollDomains: [domain("containers", "green"), domain("vms", "red"), domain("config", "green")],
});
const MEMBER = groupMember();

let requests: ZFSReceiveRequest[] = [];
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
    getGroup: () => Promise.resolve({ ok: true, active: true, name: "cellar", members: [MEMBER] }),
    memberRepos: () => Promise.resolve({ ok: true, repos: [] }),
    listCloudCredSets: () => Promise.resolve({ ok: true, sets: [] }),
    listZFSReceiveRequests: () => Promise.resolve(requests),
    listZFSLocalPools: () => Promise.resolve([{ name: "tank", sizeBytes: 1, freeBytes: 1 }]),
    listZFSReplicaServers: () => Promise.resolve([]),
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
    peers: [PEER],
    received: [],
    pulls: [],
    offers: [],
    requests,
    logins: [],
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
    data: all,
    reloadPeers: reload("peers"),
    reloadOffers: reload("offers"),
    reloadReceived: reload("received"),
    reloadPulls: reload("pulls"),
    reloadLogins: reload("logins"),
  };
  const instance = buildInstances(all).instances[0];
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <PeerDetails instance={instance} modules={modules} state={state} onClose={() => closed++} />
        </ToastProvider>
      </I18nProvider>,
    );
  });
  return screen.getByRole("dialog", { name: instance.name });
}

function section(name: string): HTMLElement {
  return screen.getByRole("region", { name });
}

beforeEach(() => {
  requests = [];
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
  it("lists the roles that are known with their state, and explains how they differ", async () => {
    await open();
    const card = section("What attic does for this server");
    expect(within(card).getByLabelText(en["instances.role.diff"])).toBeTruthy();
    const rows = [...card.querySelectorAll("[data-role]")];
    expect(rows.map((r) => r.getAttribute("data-role"))).toEqual(["receiver", "zfs"]);
    expect(rows[0].textContent).toContain(en["zfs.replica.servers.off"]);
    expect(within(card).getByLabelText(en["instances.role.receiverTip"])).toBeTruthy();
    expect(within(card).getByText(en["instances.role.noReplica"])).toBeTruthy();
    expect(within(card).queryByRole("switch")).toBeNull();
  });

  it("names the storage location its receiving server became", async () => {
    await open({
      groupReceivers: [{ memberId: "m1", name: "attic", url: "http://192.168.1.21:8000", needsAddress: false }],
      destinations: [destination({ name: "Attic off-site", repo: "rest:http://192.168.1.21:8000/cellar-login" })],
    });
    const row = section("What attic does for this server").querySelector('[data-role="receiver"]')!;
    expect(row.textContent).toContain(en["instances.role.on"]);
    expect(row.textContent).toContain("Storage location Attic off-site");
  });

  it("lists the ZFS items that replicate to it, each with where it stands", async () => {
    await open({
      replicas: [
        { itemId: "z1", dataset: "tank/appdata", replica: replica({ target: { kind: "peer", id: "m1" }, peerState: "allowed" }) },
        { itemId: "z2", dataset: "tank/media", replica: replica({ target: { kind: "peer", id: "m1" }, peerState: "asked", state: "waiting" }) },
      ],
    });
    const row = section("What attic does for this server").querySelector('[data-role="zfs"]')!;
    expect(row.textContent).toContain(en["instances.role.waiting"]);
    expect(row.textContent).toContain("tank/appdata");
    expect(row.textContent).toContain("Replicated 2 hours ago");
    expect(row.textContent).toContain("tank/media");
  });

  it("shows the storage it offered, to accept or decline", async () => {
    await open({
      offers: [meshOffer()],
    });
    const card = section("What attic does for this server");
    expect(within(card).getByText("rest:http://192.168.1.21:8000/vms")).toBeTruthy();
    expect(within(card).getByRole("button", { name: en["fleet.mesh.accept"] })).toBeTruthy();
    expect(within(card).getByRole("button", { name: en["fleet.mesh.decline"] })).toBeTruthy();
  });
});

describe("what this server does for the instance", () => {
  it("shows its ZFS requests and no other instance's", async () => {
    requests = [receiveRequest({ peer: "m1", peerName: "attic", sourceServer: "attic" }), receiveRequest({ id: "rq2", peer: "m2", peerName: "barn", item: "tank/barn" })];
    await open();
    const card = section("What this server does for attic");
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
  });

  it("has neither while the Instances module is off", async () => {
    await open({ peers: [] }, { fleet: false, receiver: true, pull: true, zfs: false });
    expect(screen.queryByRole("region", { name: en["instances.protection"] })).toBeNull();
    expect(screen.queryByRole("region", { name: en["instances.monitoring"] })).toBeNull();
    expect(screen.queryByRole("button", { name: en["fleet.remove"] })).toBeNull();
  });
});
