// Disco walks the rainbow palette. Every animation frame it moves each root
// --rb-* property a little further along the loop in discoLoop.ts, and every
// hued element follows, because hueVars() points it at the root. One clock
// drives the whole walk. A timer stepping the colours and a transition gliding
// them would each keep their own time, and every step would land a little
// early or late, as a jolt in the glide.
//
// The switch is stored like the other look settings (localStorage plus
// saveDisplayPrefs), the walk is not, and it never touches the rainbow state.
// The walk has no prefers-reduced-motion gate: this is a hidden mode, and
// finding it is a statement of intent. The storm level in index.css follows
// the same reasoning. The glide is a colour fade and does follow the motion
// engine, so with reduced motion, or at the "off" level, disco steps.
import { RAINBOW, applyRainbow, rainbowState } from "./appearance";
import { contrastOn } from "./accent";
import { buildLoop, colourAt, type Loop } from "./discoLoop";
import { save as saveDisplayPrefs } from "./displayPrefs";

const STORAGE_KEY = "bv-disco";

/** The walk covers one palette colour's worth of loop every 2.4 seconds, so a
 *  full turn of eight colours takes 19.2 seconds. Stepping, it moves one
 *  colour on at the same interval. */
export const DISCO_TICK_MS = 2400;

/** Turn-ons needed to unlock, like STORM_CLICKS in motion.ts. */
export const DISCO_UNLOCK_CLICKS = 5;

/** Longest pause between two turn-ons of one run, so somebody comparing
 *  rainbow on and off over a minute does not unlock disco by accident. */
export const DISCO_UNLOCK_WINDOW_MS = 3000;

/** getDisco returns the stored switch, false when unset or unreadable. */
export function getDisco(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) === "true";
  } catch {
    return false;
  }
}

let frame: number | null = null;
let lastFrame: number | undefined;
let loop: Loop = buildLoop(RAINBOW);
// The palette entry --rb-0 starts on, the same as at rest.
let start = 0;
// How much of a full turn the walk has covered, from 0 up to 1.
let travelled = 0;
// The colours last written. applyStoredDisco clears it, because applyRainbow
// may have written the resting palette over them since.
let painted: string[] = [];
let reducedMotion: MediaQueryList | undefined;

function paint(): void {
  const root = document.documentElement;
  reducedMotion ??= window.matchMedia("(prefers-reduced-motion: reduce)");
  const steps = root.getAttribute("data-motion") === "off" || reducedMotion.matches;
  const n = loop.palette.length;
  for (let i = 0; i < n; i++) {
    const colour = steps
      ? loop.palette[(i + start + Math.floor(travelled * n)) % n]
      : colourAt(loop, loop.at[(i + start) % n] + travelled * loop.at[n]);
    if (painted[i] === colour) continue;
    painted[i] = colour;
    root.style.setProperty(`--rb-${i}`, colour);
    root.style.setProperty(`--rb-ink-${i}`, contrastOn(colour));
  }
}

function walk(now: number): void {
  const turnMs = DISCO_TICK_MS * loop.palette.length;
  travelled = (travelled + (now - (lastFrame ?? now)) / turnMs) % 1;
  lastFrame = now;
  paint();
  frame = requestAnimationFrame(walk);
}

/** stopDisco stops the walk and is safe to call when nothing runs. The colours
 *  stay where the last frame left them; the caller decides whether to restore
 *  the palette. */
export function stopDisco(): void {
  if (frame !== null) {
    cancelAnimationFrame(frame);
    frame = null;
  }
}

/**
 * applyStoredDisco starts or stops the walk to match the switch and sets
 * `data-disco` on the root element. It runs at boot and whenever the switch or
 * rainbow changes. A walk that is already running picks up the new palette
 * and carries on from where it is, so re-applying never starts a second one or
 * sends the colours back to the start.
 */
export function applyStoredDisco(on: boolean = getDisco()): void {
  const root = document.documentElement;
  if (on) root.setAttribute("data-disco", "on");
  else root.removeAttribute("data-disco");

  // With rainbow off nothing on screen is hued. The switch stays on, and the
  // walk resumes when main.tsx re-applies both after rainbow comes back.
  const live = rainbowState();
  if (!on || !live.on) {
    if (frame !== null) {
      stopDisco();
      applyRainbow(live, { animate: false });
    }
    return;
  }

  loop = buildLoop(live.palette);
  start = live.rotate ? live.seed : 0;
  painted = [];
  if (frame === null) {
    travelled = 0;
    lastFrame = undefined;
    frame = requestAnimationFrame(walk);
  }
}

/** setDisco persists the switch and starts or stops the walk right away. */
export function setDisco(on: boolean): void {
  try {
    localStorage.setItem(STORAGE_KEY, on ? "true" : "false");
    saveDisplayPrefs();
  } catch {
    // Storage disabled, as in a private window. The choice will not survive a
    // reload, but `on` is passed on so the switch and the walk agree until then.
  }
  applyStoredDisco(on);
}

/**
 * discoTap counts Rainbow Mode turn-ons that come within
 * DISCO_UNLOCK_WINDOW_MS of each other and returns true on the fifth.
 * Counting turn-ons rather than clicks means the gesture ends with rainbow on,
 * the only state in which disco has colours to walk.
 *
 * The caller holds the count, as with stormTap in motion.ts, so the unlock
 * itself is never persisted.
 */
export function discoTap(
  state: { taps: number; last: number },
  turnedOn: boolean,
  clock: { now: number },
): boolean {
  if (!turnedOn) return false;
  const gap = clock.now - state.last;
  state.last = clock.now;
  state.taps = state.taps > 0 && gap <= DISCO_UNLOCK_WINDOW_MS ? state.taps + 1 : 1;
  if (state.taps < DISCO_UNLOCK_CLICKS) return false;
  state.taps = 0;
  return true;
}
