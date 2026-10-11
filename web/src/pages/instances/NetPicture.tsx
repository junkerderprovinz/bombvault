import { useState, type CSSProperties, type FocusEvent, type KeyboardEvent, type PointerEvent, type ReactNode } from "react";

import { InfoBubble } from "../../components/InfoBubble";
import { IconZFS } from "../../components/navGlyphs";
import { hueVars } from "../../lib/appearance";
import { useT } from "../../lib/i18n";
import { LogoImage, PHONE_PATH } from "./InstanceCard";
import { ROLES } from "./RoleTile";
import { FLOW_ROLES, ROLE_KINDS } from "./instancesModel";
import { NODE_RADIUS, netLanes, netLayout, type NetLabel, type NetLane, type NetNode, type Point } from "./netModel";

const at = ([x, y]: Point) => `${x.toFixed(1)} ${y.toFixed(1)}`;

/** useNetHover keeps which instance the pointer or the focus is on, read
 *  from the nearest element that names one. The picture and the entries
 *  beside it share it, so pointing at either lights up both. */
export function useNetHover() {
  const [hot, setHot] = useState<string | null>(null);
  const over = (e: PointerEvent | FocusEvent) =>
    setHot((e.target as Element).closest("[data-net]")?.getAttribute("data-net") ?? null);
  const out = () => setHot(null);
  return { hot, handlers: { onPointerOver: over, onPointerLeave: out, onFocus: over, onBlur: out } };
}

/** Brings the entry beside the picture into view that stands for the same
 *  instance as a node. */
function showEntry(key: string) {
  const entry = Array.from(document.querySelectorAll("[data-net]")).find(
    (el) => el.getAttribute("data-net") === key && !el.closest("svg"),
  );
  entry?.scrollIntoView({ block: "nearest" });
}

function Label({ label, name, sub, tall }: { label: NetLabel; name: string; sub: string; tall: boolean }) {
  const height = tall ? 62 : 36;
  const top = label.align === "mid" ? label.y - 18 : label.y - height / 2;
  return (
    <foreignObject x={Math.round(label.x)} y={Math.round(top)} width={Math.round(label.width)} height={height}>
      <div className={`glim-net-label glim-net-label-${label.align}`}>
        <b>{name}</b>
        {sub && <span>{sub}</span>}
      </div>
    </foreignObject>
  );
}

function Lane({ lane, title }: { lane: NetLane; title: string }) {
  const travel = {
    ...hueVars(ROLE_KINDS.indexOf(lane.role)),
    "--lane-dx": `${(lane.neck[0] - lane.from[0]).toFixed(1)}px`,
    "--lane-dy": `${(lane.neck[1] - lane.from[1]).toFixed(1)}px`,
    "--lane-turn": lane.turn,
  } as CSSProperties;
  const [bx, by] = lane.badge;
  return (
    <g
      className={`glim-net-lane glim-hue glim-pic${lane.waiting ? " glim-net-wait" : ""}`}
      data-lane={`${lane.dir}:${lane.role}:${lane.waiting ? "waiting" : "on"}`}
      style={travel}
    >
      <title>{title}</title>
      <path className="glim-net-reach" d={`M${at(lane.from)} L${at(lane.tip)}`} />
      <path className="glim-net-line" d={`M${at(lane.from)} L${at(lane.neck)}`} />
      <path className="glim-net-head" d={`M${at(lane.tip)} L${at(lane.barbs[0])} L${at(lane.barbs[1])}Z`} />
      {!lane.waiting && <circle className="glim-net-dot" cx={lane.from[0].toFixed(1)} cy={lane.from[1].toFixed(1)} r="3.4" />}
      <circle className="glim-net-badge" cx={bx.toFixed(1)} cy={by.toFixed(1)} r="7.5" />
      <g className="glim-net-badge-glyph" transform={`translate(${(bx - 4.5).toFixed(1)} ${(by - 4.5).toFixed(1)}) scale(.5625)`}>
        {ROLES[lane.role].glyph}
      </g>
    </g>
  );
}

function Shape({ node, x, y }: { node: Pick<NetNode, "kind">; x: number; y: number }) {
  if (node.kind === "peer") return <LogoImage x={x - 25} y={y - 25} size={50} />;
  if (node.kind === "phone") {
    return (
      <>
        <rect className="glim-net-tile" x={x - 14} y={y - 21} width="28" height="42" rx="8" />
        <path className="glim-net-tile-ink" transform={`translate(${x - 10} ${y - 10}) scale(1.25)`} fillRule="evenodd" d={PHONE_PATH} />
      </>
    );
  }
  return (
    <>
      <rect className="glim-net-tile glim-net-tile-zfs" x={x - 20} y={y - 20} width="40" height="40" rx="8" />
      <g className="glim-net-tile-ink glim-net-tile-zfs-ink" transform={`translate(${x - 11} ${y - 11}) scale(1.375)`}>
        <IconZFS />
      </g>
    </>
  );
}

function NetSvg({
  kind,
  selfName,
  nodes,
  hot,
}: {
  kind: "wide" | "narrow";
  selfName: string;
  nodes: NetNode[];
  hot: string | null;
}) {
  const { t } = useT();
  const layout = netLayout(kind, nodes.length);
  const tall = kind === "narrow";

  const button = (key: string, name: string, x: number, y: number, r: number, shape: ReactNode) => (
    <g
      className="glim-net-node"
      role="button"
      tabIndex={0}
      aria-label={name}
      onClick={() => showEntry(key)}
      onKeyDown={(e: KeyboardEvent) => {
        if (e.key !== "Enter" && e.key !== " ") return;
        e.preventDefault();
        showEntry(key);
      }}
    >
      <circle className="glim-net-ring" cx={x.toFixed(1)} cy={y.toFixed(1)} r={r + 6} />
      {shape}
    </g>
  );

  function laneTitle(lane: NetLane, peer: string): string {
    const line = t(lane.dir === "out" ? "instances.net.laneOut" : "instances.net.laneIn")
      .replace("{role}", t(ROLES[lane.role].label))
      .replace("{peer}", () => peer);
    if (!lane.waiting) return line;
    return `${line} · ${t(lane.dir === "out" ? "instances.net.waiting" : "instances.request")}`;
  }

  return (
    <svg
      className={`glim-net-svg glim-net-${kind}${hot && hot !== "self" ? " glim-net-has" : ""}`}
      viewBox={`0 0 ${layout.width} ${Math.round(layout.height)}`}
      role="group"
      aria-label={t("instances.net.alt")}
    >
      {nodes.map((node, j) => {
        const p = layout.points[j];
        const r = NODE_RADIUS[node.kind];
        const { link, lanes } = netLanes(layout.hub, { x: p.x, y: p.y, r }, node.flows, j);
        return (
          <g
            key={node.key}
            data-net={node.key}
            className={`glim-net-spoke${node.dim ? " glim-net-dim" : ""}${hot === node.key ? " glim-net-hot" : ""}`}
            style={{ "--spoke-i": j } as CSSProperties}
          >
            <path className="glim-net-link" pathLength={1} d={`M${at(link[0])} L${at(link[1])}`} />
            {lanes.map((lane) => (
              <Lane key={`${lane.dir}:${lane.role}:${lane.waiting}`} lane={lane} title={laneTitle(lane, node.name)} />
            ))}
            {button(node.key, node.name, p.x, p.y, r, <Shape node={node} x={p.x} y={p.y} />)}
            <Label label={p.label} name={node.name} sub={node.sub} tall={tall} />
          </g>
        );
      })}
      <g data-net="self" className={`glim-net-spoke glim-net-hub${hot === "self" ? " glim-net-hot" : ""}`}>
        {button("self", selfName, layout.hub.x, layout.hub.y, layout.hub.r, <LogoImage x={layout.hub.x - 32} y={layout.hub.y - 32} size={64} />)}
        <Label label={layout.hub.label} name={selfName} sub={t("instances.thisInstance")} tall={tall} />
      </g>
    </svg>
  );
}

/** NetPicture draws the group as a network: this server with everything
 *  paired or connected around it, and an arrow for each role in the
 *  direction its data travels. It holds one picture for a wide box and one
 *  for a narrow one and lets the stylesheet show the one that fits. */
export function NetPicture({
  selfName,
  nodes,
  hot,
}: {
  selfName: string;
  nodes: NetNode[];
  /** The key of the instance the pointer is on, in the picture or beside it. */
  hot: string | null;
}) {
  const { t } = useT();
  if (nodes.length === 0) return null;
  const shared = { selfName, nodes, hot };
  return (
    <div className="glim-pic glim-net-box" data-testid="net-picture">
      <div className="glim-net">
        <div className="glim-net-pics">
          <NetSvg kind="wide" {...shared} />
          <NetSvg kind="narrow" {...shared} />
        </div>
        {nodes.some((n) => n.kind === "peer") && (
          <div className="glim-net-key">
            {FLOW_ROLES.map((k, i) => (
              <span key={k} className="glim-hue" style={hueVars(i) as CSSProperties}>
                <span className="glim-net-key-glyph">{ROLES[k].glyph}</span>
                {t(ROLES[k].label)}
              </span>
            ))}
            <span>
              <i className="glim-net-key-wait" />
              {t("instances.net.waiting")}
              <InfoBubble tip={t("instances.net.tip")} />
            </span>
          </div>
        )}
      </div>
    </div>
  );
}
