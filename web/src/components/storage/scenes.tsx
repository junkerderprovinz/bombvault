// The two pictures on a storage location's page that explain what a setting
// does: a copy that only carries the new packs across, and a delete that the
// lock refuses. Each is drawn as its last frame and set in motion by the
// stylesheet, so it stands still wherever motion is off.
import type { CSSProperties, ReactNode } from "react";
import type { StorageLocation } from "../../lib/api";
import { Badge } from "../Badge";
import { IconTrash } from "../navGlyphs";
import { useT } from "../../lib/i18n";
import { LocationMark } from "./LocationMark";

type Marked = Pick<StorageLocation, "mark" | "kind" | "name">;

function Head({ x, width, place }: { x: number; width: number; place: Marked }) {
  return (
    <foreignObject x={x} y="0" width={width} height="26">
      <div className="flex h-full min-w-0 items-center justify-center gap-2 text-dense font-semibold text-carbon-text">
        <LocationMark location={place} size="scene" />
        <span className="truncate">{place.name}</span>
      </div>
    </foreignObject>
  );
}

function Pack({ kind, size = 16 }: { kind: "old" | "new" | "slot"; size?: number }) {
  return (
    <rect
      className={`glim-scene-pack glim-scene-pack-${kind}`}
      x={-size / 2}
      y={-size / 2}
      width={size}
      height={size}
      rx={size / 4}
    />
  );
}

function At({ x, y, children }: { x: number; y: number; children: ReactNode }) {
  return <g transform={`translate(${x} ${y})`}>{children}</g>;
}

const PACKS = Array.from({ length: 15 }, (_, k) => k);
// The packs the last backup added, one in each row.
const FRESH = [3, 7, 11];
const SOURCE_X = 10;
const TARGET_X = 226;

function packAt(box: number, k: number): [number, number] {
  return [box + 18 + (k % 5) * 22, 48 + Math.floor(k / 5) * 22];
}

/**
 * CopyScene shows why a copy is quick after the first one: the packs already
 * at the storage location stay where they are and only the new ones travel.
 */
export function CopyScene({ target }: { target: Marked }) {
  const { t } = useT();
  const source: Marked = { kind: "local", name: t("source.local") };
  return (
    <div className="glim-pic glim-scene flex flex-col items-center gap-2">
      <svg viewBox="0 0 360 118" role="img" aria-label={t("storage.copy.sceneAlt")} className="max-w-[440px]">
        <rect className="glim-scene-box" x={SOURCE_X} y="30" width="124" height="80" rx="12" />
        <rect className="glim-scene-box" x={TARGET_X} y="30" width="124" height="80" rx="12" />
        <Head x={SOURCE_X} width={124} place={source} />
        <Head x={TARGET_X} width={124} place={target} />
        <path className="glim-scene-flow" d="M144 70H212" />
        <path className="glim-scene-arrow" d="M207 64l7 6-7 6" />
        {PACKS.map((k) => {
          const [x, y] = packAt(SOURCE_X, k);
          return (
            <At key={k} x={x} y={y}>
              <Pack kind={FRESH.includes(k) ? "new" : "old"} />
            </At>
          );
        })}
        {PACKS.map((k) => {
          const [x, y] = packAt(TARGET_X, k);
          const turn = FRESH.indexOf(k);
          if (turn < 0) {
            return (
              <At key={k} x={x} y={y}>
                <Pack kind="old" />
              </At>
            );
          }
          const flight = {
            "--parcel-from": `${SOURCE_X - TARGET_X}px`,
            "--parcel-turn": turn,
            "--parcel-hop": 1 + turn * 0.35,
          } as CSSProperties;
          return (
            <At key={k} x={x} y={y}>
              <Pack kind="slot" />
              <g style={flight}>
                <g className="glim-parcel-fly">
                  <g className="glim-parcel-hop">
                    <Pack kind="new" />
                  </g>
                </g>
                <circle className="glim-scene-ring glim-parcel-ring" r="8" />
              </g>
            </At>
          );
        })}
      </svg>
      <div className="flex flex-col items-center gap-1.5 text-center">
        <b className="text-sm font-semibold text-carbon-text">{t("storage.copy.sceneLine")}</b>
        <span className="flex flex-wrap justify-center gap-x-4 gap-y-1 text-xs text-carbon-textMuted">
          <span className="inline-flex items-center gap-1.5">
            <i className="h-2.5 w-2.5 rounded-[calc(var(--radius-control)/4)] bg-carbon-textMuted opacity-50" />
            {t("storage.copy.keyThere")}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <i className="h-2.5 w-2.5 rounded-[calc(var(--radius-control)/4)] bg-(--pic)" />
            {t("storage.copy.keyNew")}
          </span>
        </span>
      </div>
    </div>
  );
}

const SLOT_Y = 58;
// The pack the pointer goes for.
const HIT = 2;

function slotX(k: number): number {
  return 86 + k * 32;
}

/**
 * LockScene plays a delete against this location's own setting: with delete
 * protection it bounces off and the backup stays, without it the backup is
 * gone.
 */
export function LockScene({ place, locked }: { place: Marked; locked: boolean }) {
  const { t } = useT();
  const hit: [number, number] = [slotX(HIT) + 4, SLOT_Y + 4];
  // Where the pointer ends up: backed off from a locked pack, still on the
  // slot of a deleted one.
  const rest: [number, number] = locked ? [180, 74] : hit;
  const hand = {
    "--hand-from-x": `${236 - rest[0]}px`,
    "--hand-from-y": `${104 - rest[1]}px`,
    "--hand-hit-x": `${hit[0] - rest[0]}px`,
    "--hand-hit-y": `${hit[1] - rest[1]}px`,
  } as CSSProperties;
  return (
    <div className="glim-pic glim-scene flex flex-col items-center gap-2">
      <svg
        viewBox="0 0 300 104"
        role="img"
        aria-label={t(locked ? "storage.lock.sceneRefused" : "storage.lock.sceneDeleted")}
        className="max-w-[360px]"
      >
        <rect
          className={`glim-scene-box${locked ? " glim-scene-box-locked" : ""}`}
          x="62"
          y="30"
          width="176"
          height="56"
          rx="12"
        />
        <Head x={62} width={176} place={place} />
        {[0, 1, 3, 4].map((k) => (
          <At key={k} x={slotX(k)} y={SLOT_Y}>
            <Pack kind="old" size={22} />
          </At>
        ))}
        <At x={slotX(HIT)} y={SLOT_Y}>
          {locked ? (
            <g className="glim-lock-shake">
              <Pack kind="old" size={22} />
              <g className="glim-lock-badge">
                <g className="glim-lock-badge-pop">
                  <g transform="scale(.8)">
                    <circle className="glim-scene-seal" r="9" />
                    <rect className="glim-scene-cut" x="-4" y="-1.2" width="8" height="6.4" rx="1.4" />
                    <path className="glim-scene-cut-line" d="M-2.4 -1.2v-1.9a2.4 2.4 0 0 1 4.8 0v1.9" />
                  </g>
                </g>
              </g>
              <circle className="glim-scene-ring glim-lock-ring" r="9" />
            </g>
          ) : (
            <>
              <Pack kind="slot" size={22} />
              <g className="glim-lock-gone">
                <Pack kind="old" size={22} />
              </g>
              <circle className="glim-scene-ring glim-scene-ring-gone glim-lock-ring" r="9" />
            </>
          )}
        </At>
        <At x={rest[0]} y={rest[1]}>
          <g className="glim-lock-hand" style={hand}>
            <g className="glim-lock-press">
              <path className="glim-scene-pointer" d="M0 0v16l4.2-4 3 7 2.8-1.2-3-6.8H13z" />
              <g transform="translate(15 19)">
                <circle className="glim-scene-bin" r="8.5" />
                <g transform="translate(-5 -5) scale(.625)" className="text-carbon-text">
                  <IconTrash />
                </g>
              </g>
            </g>
          </g>
        </At>
      </svg>
      <div className="flex flex-wrap items-center justify-center gap-2 text-center text-sm text-carbon-textSub">
        <Badge tone={locked ? "ok" : "warn"}>{t(locked ? "storage.lock.refused" : "storage.lock.deleted")}</Badge>
        <span>{t(locked ? "storage.lock.stays" : "storage.lock.gone")}</span>
      </div>
    </div>
  );
}
