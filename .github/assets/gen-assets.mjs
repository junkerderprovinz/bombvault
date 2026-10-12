/**
 * Renders the PNG assets with @resvg/resvg-js (installed globally):
 *   bombvault-banner[-dark].png  from the SVGs that gen-banner.mjs writes
 *   icon.png                     512x512 transparent icon from icon.svg
 *   bombvault-banner-logo.png    1600x500 banner without text
 *   social-preview.png           1280x640 card for the GitHub repository
 *   the Android launcher layers, the web app's icons and the docs favicon
 * and writes android/store/icon.svg, the app icon that android/store/render.mjs
 * puts on the store pages.
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
import { LOGO_H, LOGO_SVG, LOGO_W, OPT_CX, OPT_CY, placeLogo } from "./logo.mjs";

const require = createRequire(import.meta.url);
const { Resvg } = require(`${execSync("npm root -g").toString().trim()}/@resvg/resvg-js`);

const __dir = dirname(fileURLToPath(import.meta.url));
const ROOT = join(__dir, "../..");

const png = (svg, width, background) =>
  new Resvg(svg, { fitTo: { mode: "width", value: width }, background }).render().asPng();
const canvas = (w, h, body) =>
  `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">${body}</svg>`;

// The README picks the light or dark banner with <picture> and prefers-color-scheme.
for (const [suffix, bg] of [["", "#ffffff"], ["-dark", "#0d1117"]]) {
  const svg = readFileSync(join(__dir, `bombvault-banner${suffix}.svg`), "utf8");
  writeFileSync(join(__dir, `bombvault-banner${suffix}.png`), png(svg, 1600, bg));
  console.log(`bombvault-banner${suffix}.png written (1600x500)`);
}

// social-preview.png is the dark banner on GitHub's 1280x640 card. GitHub takes
// it from the repository settings, not from this file, so upload it there.
{
  const banner = readFileSync(join(__dir, "bombvault-banner-dark.svg"), "utf8").replace(
    /<svg\b[^>]*>/,
    '<svg x="0" y="120" width="1280" height="400" viewBox="0 0 1600 500" xmlns="http://www.w3.org/2000/svg">',
  );
  writeFileSync(join(__dir, "social-preview.png"), png(canvas(1280, 640, banner), 1280, "#0d1117"));
  console.log("social-preview.png written (1280x640)");
}

// The largest distance from the optical centre to the box's edge along either
// axis, and to any drawn pixel at all, which is the spark's tip.
const HALF = Math.max(OPT_CX, LOGO_W - OPT_CX, OPT_CY, LOGO_H - OPT_CY);
const REACH = (() => {
  const img = new Resvg(LOGO_SVG, { fitTo: { mode: "width", value: Math.round(LOGO_W) } }).render();
  const k = LOGO_W / img.width;
  const px = img.pixels;
  let max = 0;
  for (let y = 0; y < img.height; y++) {
    for (let x = 0; x < img.width; x++) {
      if (px[(y * img.width + x) * 4 + 3] < 128) continue;
      max = Math.max(max, Math.hypot((x + 0.5) * k - OPT_CX, (y + 0.5) * k - OPT_CY));
    }
  }
  return max;
})();

// icon.png has no tile, since the logo reads on dark and light backgrounds. The
// side is twice HALF plus 4 percent, so nothing clips. The docs use it as their
// favicon.
{
  const side = 2 * HALF * 1.04;
  const icon = png(canvas(side, side, placeLogo(side / 2, side / 2, 1)), 512);
  writeFileSync(join(__dir, "icon.png"), icon);
  writeFileSync(join(ROOT, "docs/assets/favicon.png"), icon);
  console.log("icon.png and the docs favicon written (512x512 transparent, optically centred)");
}

// The web app's favicon is the logo as it is, 512 wide.
writeFileSync(join(ROOT, "web/public/logo.png"), png(LOGO_SVG, 512));
console.log("web/public/logo.png written (512 wide)");

// bombvault-banner-logo.png is the support-thread banner: the logo on white.
{
  const BW = 1600, BH = 500;
  const body = `<rect width="${BW}" height="${BH}" fill="#ffffff"/>${placeLogo(BW / 2, BH / 2, 460 / LOGO_H)}`;
  writeFileSync(join(__dir, "bombvault-banner-logo.png"), png(canvas(BW, BH, body), BW, "white"));
  console.log("bombvault-banner-logo.png written (1600x500, textless, optically centred)");
}

// The app tile: white, with the right half a shade darker so it meets the
// vault's own dark half. Android draws it from ic_launcher_background.xml, the
// web app's maskable icon and the store icon draw it here.
const TILE_LIGHT = "#ffffff", TILE_SHADE = "#e2e2e2";
const tile = (side) =>
  `<rect width="${side}" height="${side}" fill="${TILE_LIGHT}"/><rect x="${side / 2}" width="${side / 2}" height="${side}" fill="${TILE_SHADE}"/>`;

// The Android launcher icon is layers of 108dp, and only a circle of 66dp in
// the middle survives every launcher's mask, so the spark's tip touches that
// circle. The monochrome layer is the same shape in one colour for themed
// icons; its alpha is doubled so the seams between the logo's parts close up.
{
  const RES = join(ROOT, "android/app/src/main/res");
  const DENSITIES = { mdpi: 1, hdpi: 1.5, xhdpi: 2, xxhdpi: 3, xxxhdpi: 4 };
  const s = 33 / REACH;
  const fg = canvas(108, 108, placeLogo(54, 54, s));
  const flat = LOGO_SVG.replace(/fill="#[0-9a-fA-F]{3,6}"/g, 'fill="#000000"');
  const mono = canvas(
    108,
    108,
    `<filter id="solid"><feComponentTransfer><feFuncA type="table" tableValues="0 1 1"/></feComponentTransfer></filter><g filter="url(#solid)">${placeLogo(54, 54, s, flat)}</g>`,
  );
  for (const [name, factor] of Object.entries(DENSITIES)) {
    const width = Math.round(108 * factor);
    writeFileSync(join(RES, `mipmap-${name}`, "ic_launcher_foreground.png"), png(fg, width));
    writeFileSync(join(RES, `mipmap-${name}`, "ic_launcher_monochrome.png"), png(mono, width));
  }
  console.log("android launcher foreground and monochrome layers written (108dp, five densities)");
}

// The web app manifest's icons: the logo alone for launchers that show it as
// is, and a maskable one on the app tile whose logo stays inside the circle of
// 80 percent that every mask keeps.
{
  const PUB = join(ROOT, "web/public");
  for (const [name, side, maskable] of [
    ["icon-192.png", 192, false],
    ["icon-512.png", 512, false],
    ["icon-maskable-512.png", 512, true],
  ]) {
    const body = maskable
      ? tile(side) + placeLogo(side / 2, side / 2, (0.38 * side) / REACH)
      : placeLogo(side / 2, side / 2, (0.46 * side) / HALF);
    writeFileSync(join(PUB, name), png(canvas(side, side, body), side));
  }
  console.log("web app manifest icons written (192, 512, maskable 512)");
}

// The store icon keeps the tile's rounded corners, which render.mjs squares off
// for Play, and the vault as large as the store pages have always shown it.
{
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 1000">
  <defs>
    <style>
      .cls-1 { fill: ${TILE_LIGHT}; }
      .cls-2 { fill: ${TILE_SHADE}; }
    </style>
  </defs>
  <g id="tile">
    <rect id="background" class="cls-1" width="1000" height="1000" rx="68" ry="68"/>
    <path class="cls-2" d="M932,0h-432v1000h432c37.56,0,68-30.44,68-68V68c0-37.56-30.44-68-68-68Z"/>
  </g>
  ${placeLogo(500, 503, 1)}
</svg>
`;
  writeFileSync(join(ROOT, "android/store/icon.svg"), svg);
  console.log("android/store/icon.svg written");
}
