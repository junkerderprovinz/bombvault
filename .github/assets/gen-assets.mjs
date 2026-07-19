/**
 * Render step for the BombVault banner (via @resvg/resvg-js, global install).
 *   bombvault-banner.png : rasterizes the self-contained bombvault-banner.svg
 *                          (white 1600x500; logo + "BombVault" in Bree Serif +
 *                          a claim) to PNG.
 *
 * The banner SVG's text is already baked to paths, so NO font is needed here. To
 * change the name/claim, regenerate bombvault-banner.svg from icon.svg + the
 * Bree Serif (OFL) font via opentype.js, then re-run this.
 *
 * Also renders, from the icon.svg master (Logo 2.0):
 *   icon.png                 : 512x512 TRANSPARENT container/CA/Unraid icon —
 *                              no tile, no frame; optically centred on the
 *                              designer-marked centre (see below).
 *   bombvault-banner-logo.png: 1600x500 textless support-thread banner.
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

// Theme-adaptive banner pair (house rule, ShipLog reference): light + dark,
// served by the README via <picture> prefers-color-scheme. The SVGs are
// self-contained (text-as-paths), so no font is needed here.
for (const [suffix, bg] of [["", "#ffffff"], ["-dark", "#0d1117"]]) {
  const svg = readFileSync(join(__dir, `bombvault-banner${suffix}.svg`), "utf8");
  const png = new Resvg(svg, { fitTo: { mode: "width", value: 1600 }, background: bg });
  writeFileSync(join(__dir, `bombvault-banner${suffix}.png`), png.render().asPng());
  console.log(`bombvault-banner${suffix}.png written (1600x500)`);
}

// ---------------------------------------------------------------------------
// Logo 2.0 geometry. The OPTICAL centre was marked by the designer with a
// helper dot in the delivered source: (441.6, 461.2) in the 898.34x865.1
// viewBox — NOT the geometric centre (the top-right sparks add bounding-box
// size the eye ignores). Every centred placement uses this point.
// ---------------------------------------------------------------------------
const LOGO_W = 898.34, LOGO_H = 865.1;
const OPT_CX = 441.6, OPT_CY = 461.2;
const logoRaw = readFileSync(join(__dir, "icon.svg"), "utf8").replace(/<\?xml[^>]*\?>\s*/, "");
const placeLogo = (x, y, w, h) =>
  logoRaw.replace(
    /<svg\b[^>]*viewBox="0 0 898\.34 865\.1"[^>]*>/,
    `<svg x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" viewBox="0 0 ${LOGO_W} ${LOGO_H}" xmlns="http://www.w3.org/2000/svg">`,
  );

// icon.png — the container/CA/Unraid icon: TRANSPARENT square, no tile, no
// frame (the logo reads on dark and light backgrounds by itself), optically
// centred on the designer dot. Canvas side = 2x the largest optical half-extent
// plus a small breathing margin, so nothing clips and the dot sits dead centre.
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

// bombvault-banner-logo.png — the textless support-thread banner: white
// 1600x500, logo only, optically centred on both axes.
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
