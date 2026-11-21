// The pictures in the choices of where a node's backups go: a backup leaves
// the node and lands where the choice sends it. Each is drawn as the frame it
// rests on and set in motion by the stylesheet while its choice is taken, so
// it stands still wherever motion is off. They draw with the theme's own
// tokens, like the storage location pictures whose parcel they borrow.
import type { CSSProperties, ReactNode } from "react";
import { IconKey } from "../../components/glyphs";
import { IconDatabase } from "../../components/navGlyphs";
import type { NodePlaceKind } from "../../lib/api";
import { LogoImage } from "./InstanceCard";

const NODE_X = 16;
const TARGET_X = 92;
const MID_Y = 20;

function At({ x, y, className, children }: { x: number; y: number; className?: string; children: ReactNode }) {
  return (
    <g transform={`translate(${x} ${y})`} className={className}>
      {children}
    </g>
  );
}

function Pack({ kept = false }: { kept?: boolean }) {
  return (
    <rect className={`glim-scene-pack glim-scene-pack-new${kept ? " glim-place-kept" : ""}`} x="-4.5" y="-4.5" width="9" height="9" rx="2.25" />
  );
}

/** A node is a server without a face of its own: two rack units. */
function NodeTile() {
  return (
    <At x={NODE_X} y={MID_Y}>
      <rect className="glim-scene-seal glim-place-tile" x="-13" y="-13" width="26" height="26" rx="7" />
      <rect className="glim-scene-cut" x="-7.5" y="-7" width="15" height="5.6" rx="1.6" />
      <rect className="glim-scene-cut" x="-7.5" y="1.4" width="15" height="5.6" rx="1.6" />
      <circle className="glim-scene-seal" cx="4.2" cy="-4.2" r="1" />
      <circle className="glim-scene-seal" cx="4.2" cy="4.2" r="1" />
    </At>
  );
}

function LocationTile() {
  return (
    <At x={TARGET_X} y={MID_Y}>
      <rect className="glim-place-ground glim-place-tile" x="-13" y="-13" width="26" height="26" rx="7" />
      <g className="glim-place-ink" transform="translate(-7 -7) scale(.875)">
        <IconDatabase />
      </g>
    </At>
  );
}

/** A round mark on a corner of what it belongs to. */
function Corner({ x, children }: { x: number; children: ReactNode }) {
  return (
    <At x={x + 11} y={MID_Y + 10}>
      <circle className="glim-place-badge" r="6.5" />
      {children}
    </At>
  );
}

function Lock() {
  return (
    <g transform="translate(0 -.6) scale(.75)">
      <rect className="glim-scene-cut" x="-4" y="-1.2" width="8" height="6.4" rx="1.4" />
      <path className="glim-scene-cut-line" d="M-2.4 -1.2v-1.9a2.4 2.4 0 0 1 4.8 0v1.9" />
    </g>
  );
}

function Key() {
  return (
    <g className="glim-place-badge-ink" transform="translate(-4 -4) scale(.5)">
      <IconKey />
    </g>
  );
}

const PICTURE = "glim-pic glim-scene glim-place block w-20 sm:w-28";

const FLIGHT = { "--parcel-from": `${NODE_X - TARGET_X}px`, "--parcel-hop": 0.35 } as CSSProperties;

/**
 * PlacementPicture shows where a node's backups go: into this instance's
 * receiving server, which keeps what arrives under lock; to a storage
 * location the node signs in to with a key of its own; or to this instance as
 * a copy it fetches, while the node keeps the backup.
 */
export function PlacementPicture({ kind, on }: { kind: NodePlaceKind; on: boolean }) {
  return (
    <span data-placement-picture={kind} className={on ? `${PICTURE} glim-place-on` : PICTURE}>
      <svg viewBox="0 0 112 40" aria-hidden="true">
        <path className="glim-scene-flow" d={`M34 ${MID_Y}H70`} />
        <path className="glim-scene-arrow" d={`M67 ${MID_Y - 4}l4 4-4 4`} />
        {/* The parcel travels under both ends, so it leaves from inside the
            node and goes into where it lands. */}
        <At x={TARGET_X} y={MID_Y}>
          <g style={FLIGHT}>
            <g className="glim-parcel-fly">
              <g className="glim-parcel-hop">
                <Pack />
              </g>
            </g>
          </g>
        </At>
        <At x={52} y={MID_Y} className="glim-place-parked">
          <Pack />
        </At>
        <NodeTile />
        {kind === "location" ? <LocationTile /> : <LogoImage x={TARGET_X - 15} y={MID_Y - 15} size={30} />}
        <At x={TARGET_X} y={MID_Y}>
          <circle className="glim-scene-ring glim-parcel-ring" r="8" />
        </At>
        {kind === "receiver" && (
          <Corner x={TARGET_X}>
            <Lock />
          </Corner>
        )}
        {kind === "location" && (
          <Corner x={NODE_X}>
            <Key />
          </Corner>
        )}
        {kind === "node" && (
          <At x={NODE_X + 11} y={MID_Y + 10}>
            <Pack kept />
          </At>
        )}
      </svg>
    </span>
  );
}
