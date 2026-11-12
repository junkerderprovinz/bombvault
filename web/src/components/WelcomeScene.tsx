// The picture above the first steps: a container, a VM and a folder hop into
// the open vault, the door swings shut, its dial turns and a seal says the
// first backup is in. The markup is the last frame and the stylesheet plays
// it in a loop, so where motion is off the vault stands closed and sealed.
import type { CSSProperties, ReactNode } from "react";
import { hueVars } from "../lib/appearance";

const VAULT = { x: 120, y: 74 };

function Item({ turn, x, y, children }: { turn: number; x: number; y: number; children: ReactNode }) {
  const style = {
    ...hueVars(turn),
    "--from-x": `${x - VAULT.x}px`,
    "--from-y": `${y - VAULT.y}px`,
    "--turn": turn,
  } as CSSProperties;
  return (
    <g className="glim-hue glim-pic glim-welcome-item" style={style}>
      <g transform={`translate(${VAULT.x} ${VAULT.y})`}>{children}</g>
    </g>
  );
}

/** WelcomeScene is decorative: the steps beside it say everything it shows. */
export function WelcomeScene() {
  return (
    <div className="glim-pic glim-welcome" aria-hidden="true">
      <svg viewBox="0 0 240 140" className="glim-welcome-svg">
        <line className="glim-welcome-floor" x1="16" y1="126" x2="224" y2="126" />
        <rect className="glim-welcome-body" x="78" y="26" width="84" height="96" rx="12" />
        <rect className="glim-welcome-hole" x="88" y="40" width="64" height="68" rx="8" />

        <Item turn={0} x={30} y={104}>
          <rect className="glim-welcome-shape" x="-9" y="-7" width="18" height="14" rx="2" />
          <path className="glim-welcome-line" d="M-9 -2h18M-3 -7v14" />
        </Item>
        <Item turn={1} x={44} y={70}>
          <rect className="glim-welcome-shape" x="-9" y="-7" width="18" height="12" rx="2" />
          <path className="glim-welcome-line" d="M-4 8h8" />
        </Item>
        <Item turn={2} x={30} y={38}>
          <path className="glim-welcome-shape" d="M-9 -6h6l2 2h10v11h-18z" />
        </Item>

        <g className="glim-welcome-door">
          <rect className="glim-welcome-door-face" x="88" y="40" width="64" height="68" rx="8" />
          <g className="glim-welcome-dial">
            <circle className="glim-welcome-dial-ring" cx="120" cy="74" r="16" />
            <path className="glim-welcome-dial-spokes" d="M120 58v32M104 74h32M108.7 62.7l22.6 22.6M131.3 62.7l-22.6 22.6" />
            <circle className="glim-welcome-dial-hub" cx="120" cy="74" r="5" />
          </g>
          <rect className="glim-welcome-bolt" x="146" y="56" width="10" height="6" rx="2" />
          <rect className="glim-welcome-bolt" x="146" y="86" width="10" height="6" rx="2" />
        </g>

        <g className="glim-welcome-seal">
          <circle className="glim-scene-seal" cx="164" cy="30" r="12" />
          <path className="glim-welcome-check" d="M158.5 30.5l3.8 3.8l7.2 -7.6" />
        </g>
        <path className="glim-welcome-spark" d="M196 18l2.4 6.6l6.6 2.4l-6.6 2.4l-2.4 6.6l-2.4 -6.6l-6.6 -2.4l6.6 -2.4z" />
      </svg>
    </div>
  );
}
