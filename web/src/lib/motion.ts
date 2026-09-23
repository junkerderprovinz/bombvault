import { save as saveDisplayPrefs } from "./displayPrefs";

// Motion intensity, set as data-motion on <html> and stored like the other look
// settings. It sits alongside prefers-reduced-motion rather than in front of
// it: index.css applies the three offered levels only inside
// `@media (prefers-reduced-motion: no-preference)`, so the OS setting wins. The
// hidden "storm" level is the exception, since nobody reaches it by accident;
// the reduce block names it per selector and restores its animations
// (lib/stormOverridesOs.test.ts guards that).

export type MotionIntensity = "off" | "subtle" | "wild" | "storm";

/** MOTION_INTENSITIES is what the picker offers. The hidden "storm" is left out,
 *  but isMotionIntensity accepts it as a stored value. */
export const MOTION_INTENSITIES: MotionIntensity[] = ["off", "subtle", "wild"];

/** Every level, including the one no picker lists. Validation reads this. */
const ALL_INTENSITIES: MotionIntensity[] = [...MOTION_INTENSITIES, "storm"];

/** How many clicks on an already chosen "wild" open the storm. */
export const STORM_CLICKS = 5;

/**
 * stormTap counts clicks on "wild" while it is already chosen and returns
 * "storm" on the fifth, otherwise undefined. Any other click resets the count:
 * clicking "off" five times is annoyance, not curiosity. The caller keeps the
 * count and the found flag in the settings screen's state rather than in
 * storage, so the option is offered only while it is chosen or for as long as
 * that screen stays open.
 */
export function stormTap(
  state: { taps: number },
  clicked: string,
  current: MotionIntensity,
): MotionIntensity | undefined {
  if (clicked !== "wild" || current !== "wild") {
    state.taps = 0;
    return undefined;
  }
  state.taps += 1;
  if (state.taps < STORM_CLICKS) return undefined;
  state.taps = 0;
  return "storm";
}

const STORAGE_KEY = "bv-motion";

/**
 * DEFAULT is "subtle". A "system" option would only repeat index.css, which
 * honours prefers-reduced-motion on its own. At "wild" the route wrapper
 * animates while the cards stagger in, which a reporter saw as badges flashing
 * green on Firefox/macOS and as the page trembling on Chromium (#228); the
 * cause is unconfirmed, so the lively level has to be chosen. "subtle" keeps
 * the 6px entrance because the axis is polish, not a fallback.
 */
const DEFAULT: MotionIntensity = "subtle";

// "full" is the pre-2.0.0 name of "wild". Without the alias a stored "full"
// would fall back to the default and move that choice down to "subtle".
const LEGACY_ALIASES: Readonly<Record<string, MotionIntensity>> = { full: "wild" };

// Checks against ALL_INTENSITIES so a stored "storm" survives the next reload.
function isMotionIntensity(v: unknown): v is MotionIntensity {
  return typeof v === "string" && (ALL_INTENSITIES as string[]).includes(v);
}

/**
 * normalise turns any string into a level: the value itself, the level a legacy
 * spelling names, or the default. getMotionIntensity and applyMotionIntensity
 * both take raw localStorage values and share it so they cannot disagree.
 */
function normalise(value: string | null | undefined): MotionIntensity {
  if (isMotionIntensity(value)) return value;
  // Validate the alias rather than testing `value in LEGACY_ALIASES`: `in` walks
  // the prototype chain, and "toString" would return a function that
  // typechecks as a MotionIntensity.
  if (value !== null && value !== undefined) {
    const alias = LEGACY_ALIASES[value];
    if (isMotionIntensity(alias)) return alias;
  }
  return DEFAULT;
}

/**
 * getMotionIntensity returns the stored level, translating a legacy spelling
 * and defaulting to "subtle" when unset or invalid. It never writes: it runs
 * on every boot, and a stale key would otherwise mean a write on every load.
 */
export function getMotionIntensity(): MotionIntensity {
  return normalise(localStorage.getItem(STORAGE_KEY));
}

/** applyMotionIntensity sets the data-motion attribute index.css's motion
 *  tokens key off. It accepts any value, such as a raw localStorage string,
 *  and falls back to DEFAULT for anything that is not a level. */
export function applyMotionIntensity(intensity: MotionIntensity | string | undefined): void {
  const m = normalise(intensity);
  document.documentElement.setAttribute("data-motion", m);
}

/** setMotionIntensity persists the choice and applies it immediately. */
export function setMotionIntensity(intensity: MotionIntensity): void {
  localStorage.setItem(STORAGE_KEY, intensity);
  saveDisplayPrefs();
  applyMotionIntensity(intensity);
}

/** applyStoredMotionIntensity runs in main.tsx before the first render, so the
 *  page never paints at the wrong level. */
export function applyStoredMotionIntensity(): void {
  applyMotionIntensity(getMotionIntensity());
}
