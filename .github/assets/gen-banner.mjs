/**
 * Writes bombvault-banner.svg and bombvault-banner-dark.svg: the logo from
 * icon.svg on the left, the name in Bree Serif and the claim in Lato on the
 * right. opentype.js converts the text to paths, so the SVGs need no font and
 * render the same in resvg and in a browser. gen-assets.mjs turns them into
 * PNGs.
 *
 * Needs `npm i -g opentype.js`. The fonts are downloaded to the OS temp dir.
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

const NAME = "BombVault";
const CLAIM = "Drop a backup. Detonate a restore.";
// The logo reads on both backgrounds, so only the text colours change.
const THEMES = [
  { suffix: "",      bg: "#ffffff", name: "#1f2328", claim: "#5a5d5e" },
  { suffix: "-dark", bg: "#0d1117", name: "#e6edf3", claim: "#9aa4ad" },
];
const W = 1600, H = 500;
const LH = 386;                    // logo height
// The designer marked the logo's optical centre in the source file. The sparks
// at the top right widen the bounding box without adding visual weight, so the
// logo is centred on this point rather than on the box.
const LOGO_W = 898.34, LOGO_H = 865.1;
const OPT_CX = 441.6, OPT_CY = 461.2;
const LW = LH * (LOGO_W / LOGO_H);
const nameSize = 132, claimSize = 44, gap = 70, lineGap = 8;

const fontPath = join(tmpdir(), "BombVault-BreeSerif-Regular.ttf");
if (!existsSync(fontPath)) {
  const url =
    "https://github.com/google/fonts/raw/main/ofl/breeserif/BreeSerif-Regular.ttf";
  const res = await fetch(url);
  if (!res.ok) throw new Error(`font fetch ${res.status}`);
  writeFileSync(fontPath, Buffer.from(await res.arrayBuffer()));
}
const font = opentype.parse(readFileSync(fontPath));

// Lato for the claim, as on the other Bree Serif banners.
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
const startX = 165;
const LX = startX, LY = H / 2 - OPT_CY * (LH / LOGO_H);
const textX = startX + LW + gap;

const sc = (s) => s / font.unitsPerEm;
const nameAsc = font.ascender * sc(nameSize);
const nameDesc = -font.descender * sc(nameSize);
const claimAsc = claimFont.ascender * (claimSize / claimFont.unitsPerEm);
const blockH = nameAsc + nameDesc + lineGap + claimAsc;
const nameBaseline = H / 2 - blockH / 2 + nameAsc;
const claimBaseline = nameBaseline + nameDesc + lineGap + claimAsc;

// One <path> per glyph: resvg's tessellator can silently stop partway through a
// single path with many subpaths.
const glyphD = (f, text, x, baseline, size) =>
  f.getPaths(text, x, baseline, size).map((p) => p.toPathData(2)).filter(Boolean);
const nameD = glyphD(font, NAME, textX, nameBaseline, nameSize);
const claimD = glyphD(claimFont, CLAIM, textX, claimBaseline, claimSize);
const paths = (ds, fill) => ds.map((d) => `<path d="${d}" fill="${fill}"/>`).join("");

// Embed the logo verbatim: drop the XML decl, position its root <svg>.
let logo = readFileSync(join(__dir, "icon.svg"), "utf8").replace(/<\?xml[^>]*\?>\s*/, "");
logo = logo.replace(
  /<svg\b[^>]*viewBox="0 0 898\.34 865\.1"[^>]*>/,
  `<svg x="${LX.toFixed(1)}" y="${LY.toFixed(1)}" width="${LW.toFixed(1)}" height="${LH}" viewBox="0 0 ${LOGO_W} ${LOGO_H}" xmlns="http://www.w3.org/2000/svg">`,
);

for (const t of THEMES) {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
  <rect width="${W}" height="${H}" fill="${t.bg}"/>
  ${logo}
  ${paths(nameD, t.name)}
  ${paths(claimD, t.claim)}
</svg>
`;
  writeFileSync(join(__dir, `bombvault-banner${t.suffix}.svg`), svg);
  console.log(`bombvault-banner${t.suffix}.svg written`);
}
console.log("now run gen-assets.mjs for the PNGs");
