// The contrast arithmetic of WCAG 2, and the inks the colour engine computes
// with it. Every ink handed out here reaches 4.5:1, the minimum for body text,
// whatever colour somebody picked or the device supplied.

/** The ratio body text needs, and the floor of every ink computed here. */
export const MIN_TEXT_CONTRAST = 4.5;

// Carbon's own ink: on a yellow accent a brown-tinted black reads as a smudge.
const DARK_INK = "#161616";
const WHITE = "#FFFFFF";
const BLACK = "#000000";

// Where accent text has the least room on each ramp: surface2 on light, which
// the 45% of --ink-mix was measured on, and the card on dark. OLED's surfaces
// are darker than dark's, so the dark ink holds there as well.
const LIGHT_GROUND = "#e8e8e8";
const DARK_GROUND = "#262626";

// --ink-mix of the light theme in index.css.
const LIGHT_INK_MIX = 0.45;

type Rgb = [number, number, number];

function validHex(hex: string): boolean {
  return /^#[0-9a-fA-F]{6}$/.test(hex);
}

function rgb(hex: string): Rgb {
  const n = parseInt(hex.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

function hexOf(c: Rgb): string {
  return "#" + c.map((v) => Math.round(v).toString(16).padStart(2, "0")).join("").toUpperCase();
}

/** `amount` of `to` in `from`, the way color-mix() in srgb mixes. */
function mix(from: Rgb, to: Rgb, amount: number): Rgb {
  return [0, 1, 2].map((i) => from[i] + (to[i] - from[i]) * amount) as Rgb;
}

/**
 * Relative luminance. The channels are linearised first, because the raw
 * values overstate blue and understate green.
 */
function luminance(r: number, g: number, b: number): number {
  const lin = (v: number) => {
    const s = v / 255;
    return s <= 0.04045 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
  };
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

function ratio(a: Rgb, b: Rgb): number {
  const x = luminance(...a);
  const y = luminance(...b);
  return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
}

/** The WCAG contrast ratio of two `#rrggbb` colours, from 1 to 21. */
export function contrastRatio(a: string, b: string): number {
  return ratio(rgb(a), rgb(b));
}

/**
 * `from` moved toward `to` just far enough to reach the minimum against
 * `ground`. The ratio only grows on the way, as long as `to` lies on the far
 * side of `from` from the ground, so halving finds the least amount.
 */
function pushed(from: Rgb, to: Rgb, ground: Rgb): string {
  const reaches = (t: number) => ratio(mix(from, to, t).map(Math.round) as Rgb, ground) >= MIN_TEXT_CONTRAST;
  let lo = 0;
  let hi = 1;
  for (let i = 0; i < 16; i++) {
    const mid = (lo + hi) / 2;
    if (reaches(mid)) hi = mid;
    else lo = mid;
  }
  return hexOf(mix(from, to, hi));
}

/**
 * The ink for text or a glyph on a fill of the given colour: white or Carbon's
 * near-black, whichever reads better. On the mid-tones where neither reaches
 * 4.5:1 the near-black goes on toward black until it does, which it always
 * can. An unparseable colour gets the near-black, the default accent's ink.
 */
export function contrastOn(hex: string): string {
  if (!validHex(hex)) return DARK_INK;
  const fill = rgb(hex);
  const onWhite = ratio(fill, rgb(WHITE));
  const onDark = ratio(fill, rgb(DARK_INK));
  if (onWhite > onDark && onWhite >= MIN_TEXT_CONTRAST) return WHITE;
  if (onDark >= MIN_TEXT_CONTRAST) return DARK_INK;
  return pushed(rgb(DARK_INK), rgb(BLACK), fill);
}

/**
 * The accent as text on each theme, where index.css's own mix falls short of
 * 4.5:1. Undefined for a theme means the mix holds and nothing needs setting,
 * which is the case for every preset.
 *
 * The light ink starts from the stylesheet's mix toward black and goes on
 * toward black. The dark ink starts from the accent itself and goes toward
 * white.
 */
export function accentInks(hex: string): { light: string | undefined; dark: string | undefined } {
  if (!validHex(hex)) return { light: undefined, dark: undefined };
  const accent = rgb(hex);
  const light = mix(accent, rgb(BLACK), 1 - LIGHT_INK_MIX).map(Math.round) as Rgb;
  return {
    light: ratio(light, rgb(LIGHT_GROUND)) >= MIN_TEXT_CONTRAST ? undefined : pushed(light, rgb(BLACK), rgb(LIGHT_GROUND)),
    dark: ratio(accent, rgb(DARK_GROUND)) >= MIN_TEXT_CONTRAST ? undefined : pushed(accent, rgb(WHITE), rgb(DARK_GROUND)),
  };
}
