// One record per instance out of the lists the modules keep and the roles the
// server reports, and the rows that belong to no card.
import { describe, expect, it } from "vitest";

import { replica as replicaOf } from "../../components/zfs/replica/replica.testsupport";
import { destination } from "../../lib/placement.testsupport";
import {
  fleetPeer as peer,
  groupMember as member,
  memberRoles as roles,
  meshOffer as offer,
  NOW_S,
  pullSource as pull,
  receivedRepo as received,
  roleRequest as request,
  roleSide as side,
  zfsRole,
} from "./instances.testsupport";
import {
  buildInstances,
  protectionFigures,
  roleState,
  roleStates,
  scorecardDue,
  SCORECARD_STALE_S,
  tooOldToKeep,
  unreachableToKeep,
  type InstancesData,
} from "./instancesModel";

function data(over: Partial<InstancesData> = {}): InstancesData {
  return {
    members: [member()],
    away: [],
    settling: false,
    peers: [peer()],
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

describe("the instances of the group", () => {
  it("is one record per member, with the fleet row and where the member is reached", () => {
    const { instances } = buildInstances(data());
    expect(instances).toHaveLength(1);
    expect(instances[0]).toMatchObject({ key: "m1", name: "attic", address: "https://192.168.1.21:3443", connected: true });
    expect(instances[0].peer?.id).toBe("p1");
  });

  it("names an instance the way it last named itself", () => {
    const { instances } = buildInstances(data({ peers: [peer({ lastPollInstanceName: "Attic NAS" })] }));
    expect(instances[0].name).toBe("Attic NAS");
  });

  it("keeps a member that is away as an inactive record with when it was last seen", () => {
    const { instances } = buildInstances(
      data({ members: [], peers: [], away: [{ id: "m1", name: "attic", address: "https://192.168.1.21:3443", lastSeen: 1700 }] }),
    );
    expect(instances).toHaveLength(1);
    expect(instances[0]).toMatchObject({ key: "m1", name: "attic", connected: false, presence: "inactive", lastSeen: 1700 });
    expect(instances[0].address).toBe("https://192.168.1.21:3443");
  });

  it("makes one record of the fleet row and the group's word that the member is away", () => {
    const { instances } = buildInstances(
      data({ members: [], peers: [peer({ direct: false, relay: false })], away: [{ id: "m1", name: "attic", address: "", lastSeen: 1700 }] }),
    );
    expect(instances.map((i) => [i.key, i.presence, i.lastSeen])).toEqual([["m1", "inactive", 1700]]);
  });

  it("calls a member that is in reach connected, and one away while the group comes back checking", () => {
    expect(buildInstances(data()).instances[0].presence).toBe("connected");
    const settling = buildInstances(data({ members: [], peers: [], settling: true, away: [{ id: "m1", name: "attic", address: "", lastSeen: 1 }] }));
    expect(settling.instances[0].presence).toBe("checking");
  });

  it("knows a phone that is away for a phone", () => {
    const { instances } = buildInstances(data({ members: [], peers: [], away: [{ id: "m3", name: "Pixel 8", kind: "android", address: "", lastSeen: 1 }] }));
    expect(instances[0]).toMatchObject({ app: true, presence: "inactive" });
  });

  it("still shows a member while the Instances module keeps no row for it", () => {
    const { instances } = buildInstances(data({ peers: [] }));
    expect(instances).toHaveLength(1);
    expect(instances[0].peer).toBeUndefined();
    expect(instances[0].connected).toBe(true);
  });

  it("keeps a row from before pairing as a card that has to be paired again", () => {
    const { instances } = buildInstances(
      data({ members: [], peers: [peer({ id: "old", memberId: "", needsPairing: true, url: "https://10.0.0.9:3443" })] }),
    );
    expect(instances[0]).toMatchObject({ key: "old", needsPairing: true, address: "https://10.0.0.9:3443" });
    expect(instances[0].roles).toBeUndefined();
  });

  it("makes a card for a member only a role names, by the name the request carries", () => {
    const { instances } = buildInstances(
      data({
        members: [],
        peers: [],
        roles: [roles({ memberId: "m7", name: "barn", reachable: false }), roles({ memberId: "m8", name: "" })],
      }),
    );
    expect(instances.map((i) => [i.key, i.name, i.connected])).toEqual([
      ["m7", "barn", false],
      ["m8", "m8", false],
    ]);
  });

  it("tells a member on this network from one behind the relay", () => {
    const { instances } = buildInstances(
      data({
        members: [member(), member({ id: "m2", direct: false, relay: true })],
        peers: [peer(), peer({ id: "p2", memberId: "m2", direct: false, relay: true })],
      }),
    );
    expect(instances.map((i) => [i.connected, i.direct])).toEqual([
      [true, true],
      [true, false],
    ]);
  });
});

describe("what belongs to an instance", () => {
  it("hands each received repository and pull source to its member", () => {
    const { instances, unplaced } = buildInstances(data({ received: [received()], pulls: [pull()] }));
    expect(instances[0].received.map((r) => r.id)).toEqual(["r1"]);
    expect(instances[0].pulls.map((s) => s.id)).toEqual(["s1"]);
    expect(unplaced).toEqual({ received: [], pulls: [], offers: [] });
  });

  it("leaves a row from before pairing, or of a member it cannot name, to no card", () => {
    const { instances, unplaced } = buildInstances(
      data({
        received: [received({ id: "legacy", memberId: "", needsPairing: true }), received({ id: "gone", memberId: "m9" })],
        pulls: [pull({ id: "legacy", memberId: "", needsPairing: true })],
      }),
    );
    expect(instances[0].received).toEqual([]);
    expect(unplaced.received.map((r) => r.id)).toEqual(["legacy", "gone"]);
    expect(unplaced.pulls.map((s) => s.id)).toEqual(["legacy"]);
  });

  it("counts what waits for an answer: the requests the server counted and the open storage offers", () => {
    const { instances } = buildInstances(
      data({
        roles: [roles({ asks: 2 })],
        offers: [offer(), offer({ id: "o2", status: "accepted" })],
      }),
    );
    expect(instances[0].asks).toBe(3);
    expect(instances[0].offers.map((o) => o.id)).toEqual(["o1"]);
  });

  it("gives an offer to no card when its sender's name fits none, or more than one", () => {
    const twins = data({
      members: [member(), member({ id: "m2" })],
      peers: [peer(), peer({ id: "p2", memberId: "m2" })],
      offers: [offer(), offer({ id: "o2", from: "" }), offer({ id: "o3", from: "cellar" })],
    });
    const { instances, unplaced } = buildInstances(twins);
    expect(instances.flatMap((i) => i.offers)).toEqual([]);
    expect(unplaced.offers.map((o) => o.id)).toEqual(["o1", "o2", "o3"]);
  });
});

describe("the roles an instance plays for this server", () => {
  it("reads each role from what the server reports: asked waits, allowed is on, anything else is off", () => {
    expect(roleState(side({ receiver: request() }), "receiver")).toBe("waiting");
    expect(roleState(side({ receiver: request({ state: "allowed" }) }), "receiver")).toBe("on");
    expect(roleState(side({ fetcher: request({ state: "refused" }) }), "fetcher")).toBe("off");
    expect(roleState(side({ fetcher: request({ state: "revoked" }) }), "fetcher")).toBe("off");
    expect(roleState(side(), "fetcher")).toBe("off");
    expect(roleState(side({ zfs: zfsRole("asked") }), "zfs")).toBe("waiting");
    expect(roleState(side({ zfs: zfsRole("allowed") }), "zfs")).toBe("on");
    expect(roleState(side({ zfs: zfsRole("off") }), "zfs")).toBe("off");
    expect(roleState(undefined, "zfs")).toBe("off");
  });

  it("reads a key deposit the same way: held is on, one that is behind waits, one deleted there is off", () => {
    const keeper = (state: string) => side({ keeper: { state: state as "held", at: "" } });
    expect(roleState(keeper("held"), "keeper")).toBe("on");
    expect(roleState(keeper("stale"), "keeper")).toBe("waiting");
    expect(roleState(keeper("declined"), "keeper")).toBe("off");
    expect(roleState(side(), "keeper")).toBe("off");
    // A newer server may know states this build does not. Such a deposit is
    // there and not known to be current.
    expect(roleState(keeper("someday"), "keeper")).toBe("waiting");
  });

  it("calls an instance too old to keep a deposit only when the instance answered and said so", () => {
    const of = (over: Parameters<typeof roles>[0]) => buildInstances(data({ roles: [roles(over)] })).instances[0];
    expect(tooOldToKeep(of({ canKeep: false, keep: "too-old" }))).toBe(true);
    expect(tooOldToKeep(of({ canKeep: true, keep: "yes" }))).toBe(false);
    expect(tooOldToKeep(of({ canKeep: false, reachable: false, keep: "unreachable" }))).toBe(false);
    expect(tooOldToKeep(buildInstances(data()).instances[0])).toBe(false);
  });

  it("tells an instance that could not be asked apart from one that is too old", () => {
    const of = (over: Parameters<typeof roles>[0]) => buildInstances(data({ roles: [roles(over)] })).instances[0];
    expect(unreachableToKeep(of({ canKeep: false, reachable: false, keep: "unreachable" }))).toBe(true);
    expect(unreachableToKeep(of({ canKeep: false, keep: "too-old" }))).toBe(false);
    expect(unreachableToKeep(of({ keep: "yes" }))).toBe(false);
  });

  it("takes them from what the member does for this server, not the other way round", () => {
    const { instances } = buildInstances(
      data({
        roles: [
          roles({
            byMember: side({ receiver: request({ state: "allowed" }), zfs: zfsRole("asked") }),
            forMember: side({ fetcher: request({ state: "allowed" }) }),
          }),
        ],
      }),
    );
    expect(roleStates(instances[0])).toEqual({ receiver: "on", fetcher: "off", zfs: "waiting", keeper: "off" });
  });

  it("shows every role as off for an instance the server reports no roles for", () => {
    expect(roleStates(buildInstances(data()).instances[0])).toEqual({ receiver: "off", fetcher: "off", zfs: "off", keeper: "off" });
  });

  it("names the destination an active Receiver became, by its receiving server's address", () => {
    const offered = [{ memberId: "m1", name: "attic", url: "http://192.168.1.21:8000", needsAddress: false }];
    const login = destination({ id: "d1", name: "attic", repo: "rest:http://192.168.1.21:8000/tower-login/" });
    const elsewhere = destination({ id: "d2", repo: "rest:http://192.168.1.210:8000/tower-login" });
    const on = [roles({ byMember: side({ receiver: request({ state: "allowed" }) }) })];
    const found = (over: Partial<InstancesData>) => buildInstances(data({ groupReceivers: offered, ...over })).instances[0].destination?.id;
    expect(found({ roles: on, destinations: [elsewhere, login] })).toBe("d1");
    expect(found({ roles: on, destinations: [elsewhere] })).toBeUndefined();
    expect(found({ roles: on, destinations: null })).toBeUndefined();
    expect(found({ roles: [roles({ byMember: side({ receiver: request() }) })], destinations: [login] })).toBeUndefined();
  });

  it("knows when an active Receiver has no destination yet", () => {
    const offered = [{ memberId: "m1", name: "attic", url: "http://192.168.1.21:8000", needsAddress: false }];
    const login = destination({ id: "d1", repo: "rest:http://192.168.1.21:8000/tower-login" });
    const on = (store: "rest" | "share" = "rest") => [roles({ byMember: side({ receiver: request({ state: "allowed", store }) }) })];
    const none = (over: Partial<InstancesData>) => buildInstances(data({ roles: on(), ...over })).instances[0].noDestination ?? false;
    expect(none({ groupReceivers: offered, destinations: [] })).toBe(true);
    expect(none({ groupReceivers: offered, destinations: [login] })).toBe(false);
    expect(none({ groupReceivers: offered, destinations: null })).toBe(false);
    expect(none({ groupReceivers: offered, destinations: [], roles: on("share") })).toBe(false);
  });

  it("says a Receiver just allowed has no destination while it is away or gives no address", () => {
    const on = [roles({ byMember: side({ receiver: request({ state: "allowed" }) }) })];
    const none = (over: Partial<InstancesData>) => buildInstances(data({ roles: on, ...over })).instances[0].noDestination ?? false;
    const relayOnly = [{ memberId: "m1", name: "attic", needsAddress: true }];
    const needsHost = [{ memberId: "m1", name: "attic", needsHost: true }];
    const onItsHost = destination({ id: "d1", repo: "rest:http://192.168.1.21:8000/tower-login" });
    const elsewhere = destination({ id: "d2", repo: "rest:http://192.168.1.210:8000/tower-login" });
    expect(none({ groupReceivers: [], destinations: [] })).toBe(true);
    expect(none({ groupReceivers: relayOnly, destinations: [] })).toBe(true);
    expect(none({ groupReceivers: needsHost, destinations: [elsewhere] })).toBe(true);
    expect(none({ groupReceivers: [], destinations: [onItsHost] })).toBe(false);
  });

  it("finds the destination at an address the server remembers while the member is away or behind the relay", () => {
    const login = destination({ id: "d1", repo: "rest:http://192.168.1.21:8000/tower-login" });
    const on = [roles({ receiverLocations: ["http://192.168.1.21:8000/tower-login"], byMember: side({ receiver: request({ state: "allowed" }) }) })];
    const relayOnly = [{ memberId: "m1", name: "attic", needsAddress: true }];
    const of = (over: Partial<InstancesData>) => buildInstances(data({ roles: on, ...over })).instances[0];
    expect(of({ groupReceivers: [], destinations: [login] }).destination?.id).toBe("d1");
    expect(of({ groupReceivers: relayOnly, destinations: [login] }).noDestination).toBe(false);
    expect(of({ groupReceivers: [], destinations: [] }).noDestination).toBe(true);
  });

  it("says the destination of a Receiver switched off and allowed again is off", () => {
    const offered = [{ memberId: "m1", name: "attic", url: "http://192.168.1.21:8000", needsAddress: false }];
    const on = [roles({ byMember: side({ receiver: request({ state: "allowed" }) }) })];
    const off = destination({ id: "d1", repo: "rest:http://192.168.1.21:8000/tower-login", enabled: false });
    const of = (destinations: InstancesData["destinations"]) =>
      buildInstances(data({ roles: on, groupReceivers: offered, destinations })).instances[0];
    expect(of([off])).toMatchObject({ destination: { id: "d1" }, destinationOff: true, noDestination: false });
    const again = destination({ id: "d2", repo: "rest:http://192.168.1.21:8000/tower-login2" });
    expect(of([off, again])).toMatchObject({ destination: { id: "d2" }, destinationOff: false });
  });

  it("says a Receiver on a share copies through a location the user adds", () => {
    const on = [roles({ byMember: side({ receiver: request({ state: "allowed", store: "share" }) }) })];
    const inst = buildInstances(data({ roles: on, groupReceivers: [], destinations: [] })).instances[0];
    expect(inst.shareReceiver).toBe(true);
    expect(inst.noDestination ?? false).toBe(false);
  });

  it("keeps the ZFS items that replicate to it", () => {
    const to = (id: string, itemId: string) => ({ itemId, dataset: `tank/${itemId}`, replica: replicaOf({ target: { kind: "peer", id } }) });
    const { instances } = buildInstances(data({ replicas: [to("m1", "a"), to("m2", "b"), { itemId: "x", dataset: "tank/x", replica: replicaOf() }] }));
    expect(instances[0].replicas.map((r) => r.itemId)).toEqual(["a"]);
  });

  it("gives the app no roles", () => {
    const { instances } = buildInstances(data({ members: [member({ kind: "android" })], peers: [peer({ kind: "android" })] }));
    expect(instances[0].app).toBe(true);
    expect(instances[0].roles).toBeUndefined();
  });
});

describe("a card's figures", () => {
  const domain = (protection: string, lastSuccess: number, enabled = true) =>
    ({ domain: "containers", enabled, protection, lastSuccess }) as Parameters<typeof protectionFigures>[0][number];

  it("counts the protected sections among those that report, and takes the newest backup", () => {
    expect(protectionFigures([domain("green", 100), domain("red", 300), domain("green", 200), domain("", 900), domain("green", 950, false)])).toEqual({
      protectedCount: 2,
      total: 3,
      lastBackup: 300,
    });
  });
});

describe("when a scorecard is fetched again", () => {
  it("only for a connected, polled instance whose scorecard has gone stale", () => {
    const stale = NOW_S - SCORECARD_STALE_S - 1;
    expect(scorecardDue(peer({ lastPollAt: stale }), NOW_S)).toBe(true);
    expect(scorecardDue(peer({ lastPollAt: 0 }), NOW_S)).toBe(true);
    expect(scorecardDue(peer(), NOW_S)).toBe(false);
    expect(scorecardDue(peer({ lastPollAt: stale, direct: false }), NOW_S)).toBe(false);
    expect(scorecardDue(peer({ lastPollAt: stale, enabled: false }), NOW_S)).toBe(false);
    expect(scorecardDue(peer({ lastPollAt: 0, needsPairing: true }), NOW_S)).toBe(false);
    expect(scorecardDue(peer({ lastPollAt: 0, kind: "android" }), NOW_S)).toBe(false);
  });
});
