// One record per instance out of the lists the modules keep, the roles that
// can be told from them, and the rows that belong to no card.
import { describe, expect, it } from "vitest";

import { replica as replicaOf, receiveRequest } from "../../components/zfs/replica/replica.testsupport";
import { destination } from "../../lib/placement.testsupport";
import {
  fleetPeer as peer,
  groupMember as member,
  meshOffer as offer,
  NOW_S,
  pullSource as pull,
  receivedRepo as received,
} from "./instances.testsupport";
import {
  buildInstances,
  protectionFigures,
  scorecardDue,
  SCORECARD_STALE_S,
  servedNames,
  type InstancesData,
} from "./instancesModel";

function data(over: Partial<InstancesData> = {}): InstancesData {
  return {
    members: [member()],
    peers: [peer()],
    received: [],
    pulls: [],
    offers: [],
    requests: [],
    logins: [],
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
    expect(instances[0]).toMatchObject({ key: "old", needsPairing: true, address: "https://10.0.0.9:3443", roles: {} });
  });

  it("makes a card for a member known only from its login or its request", () => {
    const { instances } = buildInstances(
      data({
        members: [],
        peers: [],
        logins: [{ memberId: "m7", name: "barn", user: "barn-login", createdAt: 1 }],
        requests: [receiveRequest({ peer: "m8", peerName: "shed" })],
      }),
    );
    expect(instances.map((i) => [i.key, i.name, i.connected])).toEqual([
      ["m7", "barn", false],
      ["m8", "shed", false],
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

  it("counts the requests that wait: asked replicas and open storage offers", () => {
    const { instances } = buildInstances(
      data({
        requests: [
          receiveRequest({ id: "a", peer: "m1" }),
          receiveRequest({ id: "b", peer: "m1", state: "allowed" }),
          receiveRequest({ id: "c", peer: "m1", state: "refused" }),
        ],
        offers: [offer(), offer({ id: "o2", status: "accepted" })],
      }),
    );
    expect(instances[0].asks).toBe(2);
    expect(instances[0].requests.map((r) => r.id)).toEqual(["a", "b"]);
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
  const offered = [{ memberId: "m1", name: "attic", url: "http://192.168.1.21:8000", needsAddress: false }];

  it("is a receiver when a destination lies on its receiving server", () => {
    const login = destination({ id: "d1", name: "attic", repo: "rest:http://192.168.1.21:8000/tower-login/" });
    const { instances } = buildInstances(data({ groupReceivers: offered, destinations: [login] }));
    expect(instances[0].roles.receiver).toBe("on");
    expect(instances[0].destination?.id).toBe("d1");
  });

  it("is no receiver while no destination points at its receiving server", () => {
    const elsewhere = destination({ repo: "rest:http://192.168.1.210:8000/tower-login" });
    const { instances } = buildInstances(data({ groupReceivers: offered, destinations: [elsewhere] }));
    expect(instances[0].roles.receiver).toBe("off");
  });

  it("is no receiver when it is connected and offers no receiving server", () => {
    expect(buildInstances(data()).instances[0].roles.receiver).toBe("off");
  });

  it("leaves the receiver role open for an instance it cannot ask", () => {
    const away = data({ members: [], peers: [peer({ direct: false, relay: false })] });
    expect(buildInstances(away).instances[0].roles).not.toHaveProperty("receiver");
    const relayOnly = data({ groupReceivers: [{ memberId: "m1", name: "attic", needsAddress: true }] });
    expect(buildInstances(relayOnly).instances[0].roles).not.toHaveProperty("receiver");
    const unread = data({ destinations: null });
    expect(buildInstances(unread).instances[0].roles).not.toHaveProperty("receiver");
  });

  it("never claims to know whether an instance fetches from this server", () => {
    const { instances } = buildInstances(data({ pulls: [pull()] }));
    expect(instances[0].roles).not.toHaveProperty("fetcher");
  });

  it("follows the ZFS items that replicate to it: waiting while one still asks, on once allowed", () => {
    const to = (peerState: "asked" | "allowed" | "refused", itemId: string) => ({
      itemId,
      dataset: `tank/${itemId}`,
      replica: replicaOf({ target: { kind: "peer", id: "m1" }, peerState }),
    });
    const role = (replicas: InstancesData["replicas"]) => buildInstances(data({ replicas })).instances[0].roles.zfs;
    expect(role([])).toBe("off");
    expect(role([to("refused", "a")])).toBe("off");
    expect(role([to("allowed", "a")])).toBe("on");
    expect(role([to("allowed", "a"), to("asked", "b")])).toBe("waiting");
    expect(role([{ itemId: "x", dataset: "tank/x", replica: replicaOf() }])).toBe("off");
  });

  it("gives the app no roles", () => {
    const { instances } = buildInstances(data({ members: [member({ kind: "android" })], peers: [peer({ kind: "android" })] }));
    expect(instances[0].app).toBe(true);
    expect(instances[0].roles).toEqual({});
  });
});

describe("what this server does for others", () => {
  it("names the instances per role", () => {
    const all = data({
      members: [member(), member({ id: "m2", name: "barn" }), member({ id: "m3", name: "shed" })],
      peers: [],
      received: [received()],
      logins: [{ memberId: "m2", name: "barn", user: "barn-login", createdAt: 1 }],
      pulls: [pull({ memberId: "m3" })],
      requests: [receiveRequest({ peer: "m2", state: "allowed" }), receiveRequest({ id: "rq2", peer: "m3" })],
    });
    const { instances } = buildInstances(all);
    expect(servedNames(instances, all.logins)).toEqual({ receiver: ["attic", "barn"], fetcher: ["shed"], zfs: ["barn"] });
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
