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
// Theme-adaptive banner pair (house rule, ShipLog reference): GitHub serves the
// dark variant via <picture> prefers-color-scheme. Logo 2.0 reads on both
// backgrounds by itself, so both themes embed the SAME logo.
const THEMES = [
  { suffix: "",      bg: "#ffffff", name: "#1f2328", claim: "#5a5d5e" },
  { suffix: "-dark", bg: "#0d1117", name: "#e6edf3", claim: "#9aa4ad" },
];
const W = 1600, H = 500;
const LH = 386;                    // logo height (house standard)
// Logo 2.0 geometry (viewBox 898.34 x 865.1). The logo's OPTICAL centre —
// marked by the designer with a helper dot in the source file — is NOT the
// geometric centre: the sparks at the top right add visual weight the eye
// ignores. All placements centre on this point, not the bounding box.
const LOGO_W = 898.34, LOGO_H = 865.1;
const OPT_CX = 441.6, OPT_CY = 461.2; // designer-marked optical centre
const LW = LH * (LOGO_W / LOGO_H); // keep logo aspect
const nameSize = 132, claimSize = 44, gap = 70, lineGap = 8;   // house standard
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
const startX = 165; // left-anchored (house standard)
// Vertically centre on the OPTICAL centre, not the bounding box.
const LX = startX, LY = H / 2 - OPT_CY * (LH / LOGO_H);
const textX = startX + LW + gap;

const sc = (s) => s / font.unitsPerEm;
const nameAsc = font.ascender * sc(nameSize);
const nameDesc = -font.descender * sc(nameSize);
const claimAsc = claimFont.ascender * (claimSize / claimFont.unitsPerEm);
const blockH = nameAsc + nameDesc + lineGap + claimAsc;
const nameBaseline = H / 2 - blockH / 2 + nameAsc;
const claimBaseline = nameBaseline + nameDesc + lineGap + claimAsc;

// Render text as ONE <path> PER GLYPH, not a single merged path: resvg's tessellator
// can silently abort a merged multi-subpath path partway through for certain
// glyph/coordinate combinations, and per-glyph paths sidestep that entirely.
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
