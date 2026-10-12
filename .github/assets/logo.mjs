// The logo master, icon.svg, for gen-banner.mjs and gen-assets.mjs. Its size
// comes from its own viewBox, so a new master only needs a new optical centre.
import { readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

export const LOGO_SVG = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "icon.svg"), "utf8").replace(
  /<\?xml[^>]*\?>\s*/,
  "",
);
const VIEWBOX = LOGO_SVG.match(/<svg\b[^>]*viewBox="([^"]+)"/)[1];
export const [LOGO_W, LOGO_H] = VIEWBOX.trim().split(/[\s,]+/).slice(2).map(Number);

// Where the vault's light and dark halves meet, at the height of the vault's
// middle. The sparks at the top right widen the box without adding weight, so
// every placement centres on this point, and on the two-tone app tile the
// halves line up with the tile's.
export const OPT_CX = 417.8;
export const OPT_CY = 430.25;

/** The logo as a nested <svg>, scaled by s with its optical centre at cx, cy. */
export function placeLogo(cx, cy, s, svg = LOGO_SVG) {
  const attrs = [
    `x="${(cx - OPT_CX * s).toFixed(2)}"`,
    `y="${(cy - OPT_CY * s).toFixed(2)}"`,
    `width="${(LOGO_W * s).toFixed(2)}"`,
    `height="${(LOGO_H * s).toFixed(2)}"`,
    `viewBox="${VIEWBOX}"`,
  ].join(" ");
  return svg.replace(/<svg\b[^>]*>/, `<svg ${attrs} xmlns="http://www.w3.org/2000/svg">`);
}
