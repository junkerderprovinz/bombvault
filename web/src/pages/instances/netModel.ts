// The geometry of the net picture: this server in the middle, everything
// paired or connected around it, and one arrow per role between two of them.
import type { MemberRoles, RoleSide, ZFSReplicaServer } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { ROLE_KINDS, roleState, type Instance, type RoleKind } from "./instancesModel";

export type NetNodeKind = "peer" | "zfs" | "phone";

export interface NetFlow {
  /** "out" carries this server's data to the node, "in" the node's data here. */
  dir: "out" | "in";
  role: RoleKind;
  /** Asked for and not answered yet. */
  waiting: boolean;
}

export interface NetNode {
  /** The key of the card that stands for the same thing in the grid. */
  key: string;
  kind: NetNodeKind;
  name: string;
  /** The line under the name: how it is reached, or what it is. */
  sub: string;
  dim: boolean;
  flows: NetFlow[];
}

/** netFlows lists the arrows between this server and one member. Data goes
 *  to the member for every role it has, and comes here for every role this
 *  server was asked or allowed to take for it. */
export function netFlows(roles: MemberRoles | undefined): NetFlow[] {
  if (!roles) return [];
  const along = (side: RoleSide, dir: NetFlow["dir"]): NetFlow[] =>
    ROLE_KINDS.flatMap((role) => {
      const state = roleState(side, role);
      return state === "off" ? [] : [{ dir, role, waiting: state === "waiting" }];
    });
  return [...along(roles.byMember, "out"), ...along(roles.forMember, "in")];
}

/** groupNodes lists what stands around this server in the picture: the
 *  paired instances with their arrows, then the ZFS servers reached over SSH,
 *  then the phones. A row from before pairing belongs to no group and is left
 *  out. */
export function groupNodes(instances: Instance[], servers: ZFSReplicaServer[], t: ReturnType<typeof useT>["t"]): NetNode[] {
  const paired = instances.filter((i) => !i.needsPairing);
  const reached = (i: Instance) => (i.connected ? t(i.direct ? "pairing.direct" : "pairing.viaRelay") : t("instances.notConnected"));
  return [
    ...paired
      .filter((i) => !i.app)
      .map((i): NetNode => ({ key: i.key, kind: "peer", name: i.name, sub: reached(i), dim: i.peer?.enabled === false, flows: netFlows(i.roles) })),
    ...servers.map((s): NetNode => ({ key: `zfs:${s.id}`, kind: "zfs", name: s.name, sub: t("instances.overSsh"), dim: !s.enabled, flows: [] })),
    ...paired
      .filter((i) => i.app)
      .map((i): NetNode => ({ key: i.key, kind: "phone", name: i.name, sub: t("fleet.kindAndroid"), dim: false, flows: [] })),
  ];
}

export type Point = [x: number, y: number];

export interface NetLabel {
  x: number;
  /** The middle of the label's box. */
  y: number;
  width: number;
  align: "start" | "end" | "mid";
}

export interface NetLayout {
  width: number;
  height: number;
  hub: { x: number; y: number; r: number; label: NetLabel };
  points: { x: number; y: number; label: NetLabel }[];
}

/** The radius of a node's shape, which an arrow stops short of. */
export const NODE_RADIUS: Record<NetNodeKind, number> = { peer: 27, zfs: 25, phone: 24 };

/** How far two arrows of one bundle lie apart. */
const LANE_GAP = 10;

/** netLayout places n nodes around this server. A wide box stacks them on
 *  both sides with their names on the outside. A narrow one has room for one
 *  side only, so this server moves to the edge. */
export function netLayout(kind: "wide" | "narrow", n: number): NetLayout {
  const wide = kind === "wide";
  const width = wide ? 720 : 340;
  const reach = wide ? 212 : 160;
  const pitch = wide ? 84 : 74;
  const right = wide ? Math.ceil(n / 2) : n;
  const left = n - right;
  const height = Math.max(150, right * pitch + 10);
  const hub = { x: !wide ? 42 : left ? 360 : 220, y: right > 1 ? height / 2 : 63, r: 34 };
  const points = Array.from({ length: n }, (_, j) => {
    const side = wide && j % 2 ? -1 : 1;
    const offset = (wide ? j >> 1 : j) - ((side > 0 ? right : left) - 1) / 2;
    const x = hub.x + side * (reach - 14 * Math.abs(offset));
    const y = hub.y + offset * pitch;
    const label: NetLabel = side > 0 ? { x: x + 36, y, width: width - x - 40, align: "start" } : { x: 4, y, width: x - 40, align: "end" };
    return { x, y, label };
  });
  return { width, height, hub: { ...hub, label: { x: hub.x - 40, y: hub.y + 58, width: 80, align: "mid" } }, points };
}

export interface NetLane extends NetFlow {
  from: Point;
  /** Where the line ends and the arrow head begins. */
  neck: Point;
  tip: Point;
  /** The two back corners of the arrow head. */
  barbs: [Point, Point];
  /** Where the role's glyph sits on the line. */
  badge: Point;
  /** Spreads the travelling dots of neighbouring arrows over the loop, 0 to 6. */
  turn: number;
}

/** netLanes draws the link between this server and one node, and the arrows
 *  beside it. Each direction keeps to the right of the way its data travels,
 *  so two arrows never share a line, and a bundle narrows towards this
 *  server, where every bundle meets. */
export function netLanes(
  hub: { x: number; y: number; r: number },
  node: { x: number; y: number; r: number },
  flows: NetFlow[],
  spoke: number,
): { link: [Point, Point]; lanes: NetLane[] } {
  const length = Math.hypot(node.x - hub.x, node.y - hub.y);
  const ux = (node.x - hub.x) / length;
  const uy = (node.y - hub.y) / length;
  // A line beside the link still ends near the rim of both shapes.
  const ends = (beside: number): [Point, Point] => {
    const at = (c: { x: number; y: number; r: number }, d: number, way: number): Point => {
      const along = Math.sqrt(Math.max(144, (c.r + 5) ** 2 - d * d)) * way;
      return [c.x + ux * along - uy * d, c.y + uy * along + ux * d];
    };
    return [at(hub, beside * 0.7, 1), at(node, beside, -1)];
  };
  const lanes = (["out", "in"] as const).flatMap((dir) => {
    const mine = flows.filter((f) => f.dir === dir);
    return mine.map((flow, s): NetLane => {
      const [near, far] = ends((s + 1) * LANE_GAP * (dir === "out" ? 1 : -1));
      const [from, tip] = dir === "out" ? [near, far] : [far, near];
      const run = Math.hypot(tip[0] - from[0], tip[1] - from[1]);
      const vx = (tip[0] - from[0]) / run;
      const vy = (tip[1] - from[1]) / run;
      const neck: Point = [tip[0] - vx * 7, tip[1] - vy * 7];
      // The glyphs sit towards the outer node, where the bundles of
      // neighbouring nodes are furthest apart.
      const share = 0.6 + (s - (mine.length - 1) / 2) * 0.2;
      return {
        ...flow,
        from,
        neck,
        tip,
        barbs: [
          [neck[0] - vy * 3.8, neck[1] + vx * 3.8],
          [neck[0] + vy * 3.8, neck[1] - vx * 3.8],
        ],
        badge: [near[0] + (far[0] - near[0]) * share, near[1] + (far[1] - near[1]) * share],
        turn: (spoke * 5 + s * 3 + (dir === "in" ? 4 : 0)) % 7,
      };
    });
  });
  return { link: ends(0), lanes };
}
