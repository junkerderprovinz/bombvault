// The pictures of the first steps, one per step card, playing one scene in
// turn: the three kinds of thing to keep are switched on, the vault drops into
// place, and they hop into it before the door shuts and a seal pops. Each is
// drawn as the frame it rests on and set in motion by the stylesheet, so where
// motion is off the areas stand on, the vault stands open in the second card
// and closed and sealed in the third.
import type { CSSProperties, ReactNode } from "react";
import { hueVars } from "../lib/appearance";

const VAULT = { x: 120, y: 74 };

/** The three kinds of thing BombVault keeps, drawn around their middle. */
const KINDS: ReactNode[] = [
  <>
    <rect className="glim-welcome-shape" x="-9" y="-7" width="18" height="14" rx="2" />
    <path className="glim-welcome-line" d="M-9 -2h18M-3 -7v14" />
  </>,
  <>
    <rect className="glim-welcome-shape" x="-9" y="-7" width="18" height="12" rx="2" />
    <path className="glim-welcome-line" d="M-4 8h8" />
  </>,
  <path className="glim-welcome-shape" d="M-9 -6h6l2 2h10v11h-18z" />,
];

function Frame({ children }: { children: ReactNode }) {
  return (
    <svg viewBox="0 0 240 140" className="glim-welcome-svg" aria-hidden="true">
      <line className="glim-welcome-floor" x1="16" y1="126" x2="224" y2="126" />
      {children}
    </svg>
  );
}

function Door({ className }: { className: string }) {
  return (
    <g className={className}>
      <rect className="glim-welcome-door-face" x="88" y="40" width="64" height="68" rx="8" />
      <g className="glim-welcome-dial">
        <circle className="glim-welcome-dial-ring" cx="120" cy="74" r="16" />
        <path className="glim-welcome-dial-spokes" d="M120 58v32M104 74h32M108.7 62.7l22.6 22.6M131.3 62.7l-22.6 22.6" />
        <circle className="glim-welcome-dial-hub" cx="120" cy="74" r="5" />
      </g>
      <rect className="glim-welcome-bolt" x="146" y="56" width="10" height="6" rx="2" />
      <rect className="glim-welcome-bolt" x="146" y="86" width="10" height="6" rx="2" />
    </g>
  );
}

function Body() {
  return (
    <>
      <rect className="glim-welcome-body" x="78" y="26" width="84" height="96" rx="12" />
      <rect className="glim-welcome-hole" x="88" y="40" width="64" height="68" rx="8" />
    </>
  );
}

/** AreasPicture is the first step: a tile per kind, each switched on in turn. */
export function AreasPicture() {
  return (
    <Frame>
      {KINDS.map((kind, i) => {
        const x = 50 + i * 70;
        return (
          <g key={i} className="glim-hue glim-pic" style={{ ...hueVars(i), "--turn": i } as CSSProperties}>
            <rect className="glim-welcome-tile" x={x - 24} y="34" width="48" height="44" rx="10" />
            <g transform={`translate(${x} 56) scale(1.3)`}>{kind}</g>
            <rect className="glim-welcome-track" x={x - 13} y="92" width="26" height="14" rx="7" />
            <circle className="glim-welcome-knob glim-welcome-switch" cx={x + 6} cy="99" r="4.5" />
          </g>
        );
      })}
    </Frame>
  );
}

/** PlacePicture is the second step: the vault drops onto the floor, open. */
export function PlacePicture() {
  return (
    <Frame>
      <g className="glim-welcome-drop">
        <Body />
        <Door className="glim-welcome-door-open" />
      </g>
    </Frame>
  );
}

function Item({ turn, x, y }: { turn: number; x: number; y: number }) {
  const style = {
    ...hueVars(turn),
    "--from-x": `${x - VAULT.x}px`,
    "--from-y": `${y - VAULT.y}px`,
    "--turn": turn,
  } as CSSProperties;
  return (
    <g className="glim-hue glim-pic glim-welcome-item" style={style}>
      <g transform={`translate(${VAULT.x} ${VAULT.y})`}>{KINDS[turn]}</g>
    </g>
  );
}

/** BackupPicture is the third step: the kinds hop in, the door shuts, the
 *  dial turns home and the seal pops with the spark. */
export function BackupPicture() {
  return (
    <Frame>
      <Body />
      <Item turn={0} x={30} y={104} />
      <Item turn={1} x={44} y={70} />
      <Item turn={2} x={30} y={38} />
      <Door className="glim-welcome-door" />
      <g className="glim-welcome-seal">
        <circle className="glim-scene-seal" cx="164" cy="30" r="12" />
        <path className="glim-welcome-check" d="M158.5 30.5l3.8 3.8l7.2 -7.6" />
      </g>
      <path className="glim-welcome-spark" d="M196 18l2.4 6.6l6.6 2.4l-6.6 2.4l-2.4 6.6l-2.4 -6.6l-6.6 -2.4l6.6 -2.4z" />
    </Frame>
  );
}
