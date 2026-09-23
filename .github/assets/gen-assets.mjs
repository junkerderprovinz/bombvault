/**
 * Renders the PNG assets with @resvg/resvg-js (installed globally):
 *   bombvault-banner[-dark].png  from the SVGs that gen-banner.mjs writes
 *   icon.png                     512x512 transparent icon from icon.svg
 *   bombvault-banner-logo.png    1600x500 banner without text
 *
 * The banner text is already converted to paths, so no font is needed here.
 *
 * Run: node .github/assets/gen-banner.mjs && node .github/assets/gen-assets.mjs
 */
import { readFileSync, writeFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { execSync } from "node:child_process";

const require = createRequire(import.meta.url);
const { Resvg } = require(`${execSync("npm root -g").toString().trim()}/@resvg/resvg-js`);

const __dir = dirname(fileURLToPath(import.meta.url));

// The README picks the light or dark banner with <picture> and prefers-color-scheme.
for (const [suffix, bg] of [["", "#ffffff"], ["-dark", "#0d1117"]]) {
  const svg = readFileSync(join(__dir, `bombvault-banner${suffix}.svg`), "utf8");
  const png = new Resvg(svg, { fitTo: { mode: "width", value: 1600 }, background: bg });
  writeFileSync(join(__dir, `bombvault-banner${suffix}.png`), png.render().asPng());
  console.log(`bombvault-banner${suffix}.png written (1600x500)`);
}

// The designer marked the logo's optical centre in the source file. The sparks
// at the top right widen the bounding box without adding visual weight, so
// every placement centres on this point rather than on the box.
const LOGO_W = 898.34, LOGO_H = 865.1;
const OPT_CX = 441.6, OPT_CY = 461.2;
const logoRaw = readFileSync(join(__dir, "icon.svg"), "utf8").replace(/<\?xml[^>]*\?>\s*/, "");
const placeLogo = (x, y, w, h) =>
  logoRaw.replace(
    /<svg\b[^>]*viewBox="0 0 898\.34 865\.1"[^>]*>/,
    `<svg x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" viewBox="0 0 ${LOGO_W} ${LOGO_H}" xmlns="http://www.w3.org/2000/svg">`,
  );

// icon.png has no tile, since the logo reads on dark and light backgrounds. The
// side is twice the largest distance from the optical centre plus 4 percent, so
// nothing clips.
{
  const half = Math.max(OPT_CX, LOGO_W - OPT_CX, OPT_CY, LOGO_H - OPT_CY) * 1.04;
  const side = 2 * half;
  const iconSvg = `<svg xmlns="http://www.w3.org/2000/svg" width="${side.toFixed(2)}" height="${side.toFixed(2)}" viewBox="0 0 ${side.toFixed(2)} ${side.toFixed(2)}">
  ${placeLogo(half - OPT_CX, half - OPT_CY, LOGO_W, LOGO_H)}
</svg>`;
  const iconPng = new Resvg(iconSvg, { fitTo: { mode: "width", value: 512 } });
  writeFileSync(join(__dir, "icon.png"), iconPng.render().asPng());
  console.log("icon.png written (512x512 transparent, optically centred)");
}

// bombvault-banner-logo.png is the support-thread banner: the logo on white.
{
  const BW = 1600, BH = 500, LH = 460;
  const s = LH / LOGO_H, LW = LOGO_W * s;
  const logoBanner = `<svg xmlns="http://www.w3.org/2000/svg" width="${BW}" height="${BH}" viewBox="0 0 ${BW} ${BH}">
  <rect width="${BW}" height="${BH}" fill="#ffffff"/>
  ${placeLogo(BW / 2 - OPT_CX * s, BH / 2 - OPT_CY * s, LW, LH)}
</svg>`;
  const lb = new Resvg(logoBanner, { fitTo: { mode: "width", value: 1600 }, background: "white" });
  writeFileSync(join(__dir, "bombvault-banner-logo.png"), lb.render().asPng());
  console.log("bombvault-banner-logo.png written (1600x500, textless, optically centred)");
}
