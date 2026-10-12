import { useEffect, useRef, useState, type MouseEvent, type PointerEvent, type RefObject } from "react";
import { logoBurst } from "./logoBurst";
import { motionStopped } from "./motion";

/** When each of the three shake stages starts, in ms after the press. */
export const QUAKE_AT = [350, 1000, 1650] as const;
/** When a press still held goes off. */
export const BURST_AT = 2300;
/** How long the mark takes to grow back after the burst. */
export const REBORN_MS = 600;

export type LogoStage = 0 | 1 | 2 | 3;
export type LogoPhase = "idle" | "gone" | "reborn";

export interface LogoHold {
  /** The drawn mark, which the burst slices into shards. */
  markRef: RefObject<HTMLImageElement | null>;
  /** The shake stage while the press is held, 0 when it is not shaking. */
  stage: LogoStage;
  phase: LogoPhase;
  handlers: {
    onPointerDown: (e: PointerEvent) => void;
    onPointerUp: () => void;
    onPointerLeave: () => void;
    onPointerCancel: () => void;
    onClick: (e: MouseEvent) => void;
    onContextMenu: (e: MouseEvent) => void;
  };
}

/**
 * useLogoHold turns a held press on the logo into a shake that grows in three
 * stages and then a burst, while a short click still navigates.
 */
export function useLogoHold(onNavigate: () => void): LogoHold {
  const markRef = useRef<HTMLImageElement>(null);
  const [stage, setStage] = useState<LogoStage>(0);
  const [phase, setPhase] = useState<LogoPhase>("idle");
  const timers = useRef<ReturnType<typeof setTimeout>[]>([]);
  const reborn = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const stopBurst = useRef<(() => void) | null>(null);
  const busy = useRef(false);
  // Set once the shake has started: the release after that is play, not a
  // request to go to the overview.
  const played = useRef(false);

  const clear = () => {
    for (const id of timers.current) clearTimeout(id);
    timers.current = [];
  };
  const release = () => {
    clear();
    setStage(0);
  };

  useEffect(
    () => () => {
      clear();
      clearTimeout(reborn.current);
      stopBurst.current?.();
    },
    [],
  );

  const goOff = () => {
    timers.current = [];
    const img = markRef.current;
    if (!img) return;
    busy.current = true;
    setStage(0);
    setPhase("gone");
    stopBurst.current = logoBurst(img, () => {
      stopBurst.current = null;
      setPhase("reborn");
      reborn.current = setTimeout(() => {
        busy.current = false;
        setPhase("idle");
      }, REBORN_MS);
    });
  };

  return {
    markRef,
    stage,
    phase,
    handlers: {
      onPointerDown(e) {
        if (e.button !== 0) return;
        played.current = false;
        if (busy.current || motionStopped()) return;
        clear();
        timers.current = [
          ...QUAKE_AT.map((at, i) =>
            setTimeout(() => {
              played.current = true;
              setStage((i + 1) as LogoStage);
            }, at),
          ),
          setTimeout(goOff, BURST_AT),
        ];
      },
      onPointerUp: release,
      onPointerLeave: release,
      onPointerCancel: release,
      onClick(e) {
        // A keyboard click has no pointer travel behind it and always
        // navigates.
        if (e.detail !== 0 && (played.current || busy.current)) {
          played.current = false;
          return;
        }
        onNavigate();
      },
      onContextMenu: (e) => e.preventDefault(),
    },
  };
}
