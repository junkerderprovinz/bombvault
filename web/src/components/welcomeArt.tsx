// The pictures of the first steps, one per step card, playing one scene in
// turn: each kind of thing to keep is switched on and flies off at once,
// across the cards, into the vault of the second card, whose door opens for
// them and shuts behind them, and the vault of the last card locks and says
// so. Each is drawn as the frame it rests on and set in motion by the
// stylesheet, so where motion is off the areas stand on and both vaults stand
// closed, the last one with its badge.
import type { CSSProperties, ReactNode } from "react";
import { hueVars } from "../lib/appearance";
import { IconCheck } from "./navGlyphs";

/** Where a kind's tile sits in the first picture and the vault's opening in
 *  the second, in the pictures' own 240 by 140 units. */
export const TILE_AT = [0, 1, 2].map((i) => [50 + i * 70, 56] as const);
export const VAULT_AT = [120, 74] as const;

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

function Door({ className, dial }: { className?: string; dial?: string }) {
  return (
    <g className={className}>
      <rect className="glim-welcome-door-face" x="88" y="40" width="64" height="68" rx="8" />
      <circle className="glim-welcome-dial-ring" cx="120" cy="74" r="16" />
      <g className={dial}>
        <path className="glim-welcome-dial-spokes" d="M120 58v32M104 74h32M108.7 62.7l22.6 22.6M131.3 62.7l-22.6 22.6" />
      </g>
      <circle className="glim-welcome-dial-hub" cx="120" cy="74" r="5" />
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
        const [x, y] = TILE_AT[i]!;
        return (
          <g key={i} className="glim-hue glim-pic" style={{ ...hueVars(i), "--turn": i } as CSSProperties}>
            <rect className="glim-welcome-tile" x={x - 24} y={y - 22} width="48" height="44" rx="10" />
            <g transform={`translate(${x} ${y}) scale(1.3)`}>{kind}</g>
            <rect className="glim-welcome-track" x={x - 13} y="92" width="26" height="14" rx="7" />
            <circle className="glim-welcome-knob glim-welcome-switch" cx={x + 6} cy="99" r="4.5" />
          </g>
        );
      })}
    </Frame>
  );
}

/** PlacePicture is the second step: the vault stands ready and opens its
 *  door when the parcels arrive. */
export function PlacePicture() {
  return (
    <Frame>
      <Body />
      <Door className="glim-welcome-opening" />
    </Frame>
  );
}

/** Where the confetti of the last card flies, around the seal. */
const CONFETTI = Array.from({ length: 16 }, (_, k) => {
  const dx = ((k * 37) % 150) - 75;
  return {
    "--peak-x": `${dx / 2}px`,
    "--peak-y": `${-18 - (k % 4) * 5}px`,
    "--fall-x": `${dx}px`,
    "--fall-y": `${24 + ((k * 53) % 60)}px`,
    "--spin": `${k % 2 ? 360 : -360}deg`,
  } as CSSProperties;
});

/** BackupPicture is the third step: the dial turns home to lock what flew in,
 *  and the seal pops with a badge that says it is backed up. */
export function BackupPicture({ done }: { done: string }) {
  return (
    <Frame>
      <Body />
      <Door dial="glim-welcome-dial" />
      <circle className="glim-scene-ring glim-welcome-ring" cx="120" cy="18" r="16" />
      <g className="glim-welcome-seal">
        <g transform="translate(120 18)">
          <rect className="glim-step-badge-ground" x="-46" y="-13" width="92" height="26" rx="13" />
          <g transform="translate(-35 -6) scale(.75)" className="glim-step-badge-ink">
            <IconCheck />
          </g>
          <text className="glim-step-badge-text" x="7" y="4.5" textAnchor="middle">
            {done}
          </text>
        </g>
      </g>
      <g transform="translate(120 18)">
        {CONFETTI.map((flight, k) => (
          <g key={k} className="glim-welcome-confetti" style={flight}>
            <rect className={`glim-step-confetti-${k % 4}`} x="-2" y="-3.5" width="4" height="7" rx="1" />
          </g>
        ))}
      </g>
    </Frame>
  );
}

/** One thing on its way from its tile in the first card into the open vault
 *  of the second. It lives above the cards, so it crosses their edges. */
export function Parcel({ turn, style }: { turn: number; style: CSSProperties }) {
  return (
    <div className="glim-hue glim-pic glim-welcome-parcel" style={{ ...hueVars(turn), "--turn": turn, ...style } as CSSProperties} aria-hidden="true">
      <svg viewBox="-12 -12 24 24">
        {KINDS[turn]}
      </svg>
    </div>
  );
}
