// The net picture's geometry: which arrows two instances share, where the
// nodes stand in a wide and in a narrow box, and how the arrows of one bundle
// keep apart.
import { describe, expect, it } from "vitest";

import { server } from "../../components/zfs/replica/replica.testsupport";
import { en, type TranslationKey } from "../../lib/i18n";
import { fleetPeer, groupMember, memberRoles, roleRequest, roleSide, zfsRole } from "./instances.testsupport";
import { buildInstances } from "./instancesModel";
import { groupNodes, netFlows, netLanes, netLayout, NODE_RADIUS, type NetFlow } from "./netModel";

const t = ((key: TranslationKey) => en[key]) as Parameters<typeof groupNodes>[2];

describe("the arrows between this server and a member", () => {
  it("point outwards for what the member does and inwards for what this server does", () => {
    const flows = netFlows(
      memberRoles({
        byMember: roleSide({ receiver: roleRequest({ state: "allowed" }), zfs: zfsRole("asked") }),
        forMember: roleSide({ fetcher: roleRequest() }),
      }),
    );
    expect(flows).toEqual([
      { dir: "out", role: "receiver", waiting: false },
      { dir: "out", role: "zfs", waiting: true },
      { dir: "in", role: "fetcher", waiting: true },
    ]);
  });

  it("leave out a role that was declined, taken back or never asked for", () => {
    const flows = netFlows(
      memberRoles({
        byMember: roleSide({ receiver: roleRequest({ state: "refused" }), fetcher: roleRequest({ state: "revoked" }) }),
        forMember: roleSide({ zfs: zfsRole("off") }),
      }),
    );
    expect(flows).toEqual([]);
    expect(netFlows(undefined)).toEqual([]);
  });

  it("leave out a key deposit, which is one sealed file and no place backups go to", () => {
    const keeper = { state: "held" as const, at: "" };
    expect(netFlows(memberRoles({ byMember: roleSide({ keeper }), forMember: roleSide({ keeper }) }))).toEqual([]);
  });
});

describe("what stands around this server", () => {
  it("lists the paired instances first, then the ZFS servers, then the phones", () => {
    const { instances } = buildInstances({
      members: [groupMember({ id: "phone", name: "Pixel", kind: "android" }), groupMember(), groupMember({ id: "m2", name: "barn", direct: false, relay: true })],
      peers: [fleetPeer({ id: "old", memberId: "", name: "office", needsPairing: true }), fleetPeer({ enabled: false })],
      received: [],
      pulls: [],
      offers: [],
      roles: [memberRoles({ byMember: roleSide({ receiver: roleRequest({ state: "allowed" }) }) })],
      destinations: null,
      groupReceivers: null,
      replicas: [],
    });
    const nodes = groupNodes(instances, [server({ enabled: false })], t);
    expect(nodes.map((n) => [n.key, n.kind, n.sub, n.dim])).toEqual([
      ["m1", "peer", en["pairing.direct"], true],
      ["m2", "peer", en["pairing.viaRelay"], false],
      ["zfs:nas", "zfs", en["instances.overSsh"], true],
      ["phone", "phone", en["fleet.kindAndroid"], false],
    ]);
    expect(nodes[0].flows).toEqual([{ dir: "out", role: "receiver", waiting: false }]);
    expect(nodes[3].flows).toEqual([]);
  });

  it("says when a paired instance is not reached", () => {
    const { instances } = buildInstances({
      members: [],
      peers: [fleetPeer({ direct: false, relay: false })],
      received: [],
      pulls: [],
      offers: [],
      roles: [],
      destinations: null,
      groupReceivers: null,
      replicas: [],
    });
    expect(groupNodes(instances, [], t)[0].sub).toBe(en["instances.notConnected"]);
  });
});

describe("the arrangement", () => {
  it("puts this server in the middle of a wide box, with the others on both sides", () => {
    const layout = netLayout("wide", 4);
    expect(layout.width).toBe(720);
    expect(layout.hub.x).toBe(360);
    expect(layout.points.map((p) => Math.sign(p.x - layout.hub.x))).toEqual([1, -1, 1, -1]);
    expect(layout.points.map((p) => p.label.align)).toEqual(["start", "end", "start", "end"]);
    // The two on one side stand above and below this server.
    expect(layout.points[0].y).toBeLessThan(layout.hub.y);
    expect(layout.points[2].y).toBeGreaterThan(layout.hub.y);
  });

  it("moves this server to the edge of a narrow box and stacks the others on one side", () => {
    const layout = netLayout("narrow", 3);
    expect(layout.width).toBe(340);
    expect(layout.hub.x).toBe(42);
    expect(layout.points.every((p) => p.x > layout.hub.x && p.label.align === "start")).toBe(true);
    const ys = layout.points.map((p) => p.y);
    expect(ys).toEqual([...ys].sort((a, b) => a - b));
    expect(layout.height).toBeGreaterThanOrEqual(ys[2] + NODE_RADIUS.peer);
  });

  it("moves this server towards the start while a wide box has nodes on one side only", () => {
    expect(netLayout("wide", 1).hub.x).toBe(220);
    expect(netLayout("wide", 2).hub.x).toBe(360);
  });

  it("keeps every node and its name inside the box, however many there are", () => {
    for (const kind of ["wide", "narrow"] as const) {
      for (const n of [1, 2, 5, 9]) {
        const layout = netLayout(kind, n);
        for (const p of layout.points) {
          expect(p.x - NODE_RADIUS.peer).toBeGreaterThan(0);
          expect(p.x + NODE_RADIUS.peer).toBeLessThan(layout.width);
          expect(p.y - NODE_RADIUS.peer).toBeGreaterThanOrEqual(0);
          expect(p.y + NODE_RADIUS.peer).toBeLessThanOrEqual(layout.height);
          expect(p.label.x).toBeGreaterThanOrEqual(0);
          expect(p.label.x + p.label.width).toBeLessThanOrEqual(layout.width);
          expect(p.label.width).toBeGreaterThan(60);
        }
      }
    }
  });
});

describe("the arrows of one link", () => {
  const hub = { x: 100, y: 100, r: 34 };
  const node = { x: 300, y: 100, r: 27 };
  const flow = (dir: NetFlow["dir"], role: NetFlow["role"], waiting = false): NetFlow => ({ dir, role, waiting });

  it("draw the plain link between the rims of both shapes", () => {
    const { link, lanes } = netLanes(hub, node, [], 0);
    expect(lanes).toEqual([]);
    expect(link[0][0]).toBeCloseTo(hub.x + hub.r + 5);
    expect(link[1][0]).toBeCloseTo(node.x - node.r - 5);
    expect(link[0][1]).toBeCloseTo(100);
  });

  it("start at this server for a role the node has, and at the node for one this server has", () => {
    const { lanes } = netLanes(hub, node, [flow("out", "receiver"), flow("in", "fetcher")], 0);
    const [out, back] = lanes;
    expect(out.from[0]).toBeLessThan(out.tip[0]);
    expect(back.from[0]).toBeGreaterThan(back.tip[0]);
    // Each keeps to its own side of the link.
    expect(Math.sign(out.from[1] - 100)).toBe(-Math.sign(back.from[1] - 100));
  });

  it("keep the arrows of one direction apart and their glyphs off each other", () => {
    const { lanes } = netLanes(hub, node, [flow("out", "receiver"), flow("out", "fetcher"), flow("out", "zfs")], 0);
    const offsets = lanes.map((l) => l.tip[1]);
    expect(new Set(offsets.map((y) => y.toFixed(1))).size).toBe(3);
    for (let i = 1; i < lanes.length; i++) {
      const gap = Math.hypot(lanes[i].badge[0] - lanes[i - 1].badge[0], lanes[i].badge[1] - lanes[i - 1].badge[1]);
      expect(gap).toBeGreaterThan(15);
    }
  });

  it("end the line where the arrow head begins", () => {
    const [lane] = netLanes(hub, node, [flow("out", "receiver")], 0).lanes;
    expect(Math.hypot(lane.tip[0] - lane.neck[0], lane.tip[1] - lane.neck[1])).toBeCloseTo(7);
    const [a, b] = lane.barbs;
    expect(Math.hypot(a[0] - b[0], a[1] - b[1])).toBeCloseTo(7.6);
  });

  it("spreads the travelling dots of neighbouring arrows over the loop", () => {
    const flows = [flow("out", "receiver"), flow("out", "fetcher"), flow("in", "zfs")];
    const turns = [0, 1, 2].flatMap((spoke) => netLanes(hub, node, flows, spoke).lanes.map((l) => l.turn));
    expect(turns.every((turn) => turn >= 0 && turn < 7)).toBe(true);
    expect(new Set(turns).size).toBeGreaterThan(4);
  });
});
