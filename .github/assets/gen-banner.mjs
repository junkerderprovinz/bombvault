/**
 * Generates the self-contained BombVault banner SVG:
 *   bombvault-banner.svg : white 1600x500; the logo (embedded verbatim from
 *                          icon.svg) on the left, "BombVault" in Bree Serif + a
 *                          cheeky claim to the right. The text is converted to
 *                          SVG paths (opentype.js) so the SVG needs NO font and
 *                          renders identically with resvg or a browser.
 *
 * Then run gen-assets.mjs to rasterize it to bombvault-banner.png.
 *
 * Deps: `npm i -g opentype.js`. The Bree Serif (OFL) font is fetched at runtime
 * to the OS temp dir — it is NOT committed to the repo.
 *
 * Tweak NAME / CLAIM / sizes below, then: node .github/assets/gen-banner.mjs
 */
import { readFileSync, writeFileSync, existsSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { tmpdir } from "node:os";
import { createRequire } from "node:module";
import { execSync } from "node:child_process";

const require = createRequire(import.meta.url);
const opentype = require(`${execSync("npm root -g").toString().trim()}/opentype.js`);

const __dir = dirname(fileURLToPath(import.meta.url));

// ---- content + styling -----------------------------------------------------
const NAME = "BombVault";
const CLAIM = "Drop a backup. Detonate a restore.";
const NAME_FILL = "#242626"; // logo dark charcoal
const CLAIM_FILL = "#5a5d5e"; // logo mid grey
const W = 1600, H = 500;
const LH = 410;                    // logo height
const LW = LH * (914.72 / 886.45); // keep logo aspect
const nameSize = 148, claimSize = 42, gap = 64, lineGap = 22;
// ---------------------------------------------------------------------------

const fontPath = join(tmpdir(), "BombVault-BreeSerif-Regular.ttf");
if (!existsSync(fontPath)) {
  const url =
    "https://github.com/google/fonts/raw/main/ofl/breeserif/BreeSerif-Regular.ttf";
  const res = await fetch(url);
  if (!res.ok) throw new Error(`font fetch ${res.status}`);
  writeFileSync(fontPath, Buffer.from(await res.arrayBuffer()));
}
const font = opentype.parse(readFileSync(fontPath));

// Claim is set in Lato (a humanist sans that pairs with Bree Serif) — shared
// across all Bree-Serif repos for a consistent look.
const claimFontPath = join(tmpdir(), "BombVault-Lato-Regular.ttf");
if (!existsSync(claimFontPath)) {
  const r = await fetch("https://github.com/google/fonts/raw/main/ofl/lato/Lato-Regular.ttf");
  if (!r.ok) throw new Error(`claim font fetch ${r.status}`);
  writeFileSync(claimFontPath, Buffer.from(await r.arrayBuffer()));
}
const claimFont = opentype.parse(readFileSync(claimFontPath));

const nameW = font.getAdvanceWidth(NAME, nameSize);
const claimW = claimFont.getAdvanceWidth(CLAIM, claimSize);
const groupW = LW + gap + Math.max(nameW, claimW);
const startX = (W - groupW) / 2;
const LX = startX, LY = (H - LH) / 2;
const textX = startX + LW + gap;

const sc = (s) => s / font.unitsPerEm;
const nameAsc = font.ascender * sc(nameSize);
const nameDesc = -font.descender * sc(nameSize);
const claimAsc = claimFont.ascender * (claimSize / claimFont.unitsPerEm);
const blockH = nameAsc + nameDesc + lineGap + claimAsc;
const nameBaseline = H / 2 - blockH / 2 + nameAsc;
const claimBaseline = nameBaseline + nameDesc + lineGap + claimAsc;

const namePath = font.getPath(NAME, textX, nameBaseline, nameSize).toPathData(2);
const claimPath = claimFont.getPath(CLAIM, textX, claimBaseline, claimSize).toPathData(2);

// Embed the logo verbatim: drop the XML decl, position its root <svg>.
let logo = readFileSync(join(__dir, "icon.svg"), "utf8").replace(/<\?xml[^>]*\?>\s*/, "");
logo = logo.replace(
  /<svg\b[^>]*viewBox="0 0 914\.72 886\.45"[^>]*>/,
  `<svg x="${LX.toFixed(1)}" y="${LY.toFixed(1)}" width="${LW.toFixed(1)}" height="${LH}" viewBox="0 0 914.72 886.45" xmlns="http://www.w3.org/2000/svg">`,
);

const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
  <rect width="${W}" height="${H}" fill="#ffffff"/>
  ${logo}
  <path d="${namePath}" fill="${NAME_FILL}"/>
  <path d="${claimPath}" fill="${CLAIM_FILL}"/>
</svg>
`;
writeFileSync(join(__dir, "bombvault-banner.svg"), svg);
console.log("bombvault-banner.svg written — now run gen-assets.mjs for the PNG");
