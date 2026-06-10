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
 * The container / CA icon is NOT rendered here — it comes from the hand-made
 * transparent icon-source.png (downscaled in gen-assets.py).
 *
 * Run: node .github/assets/gen-assets.mjs && python .github/assets/gen-assets.py
 */
import { readFileSync, writeFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { execSync } from "node:child_process";

const require = createRequire(import.meta.url);
const { Resvg } = require(`${execSync("npm root -g").toString().trim()}/@resvg/resvg-js`);

const __dir = dirname(fileURLToPath(import.meta.url));
const svg = readFileSync(join(__dir, "bombvault-banner.svg"), "utf8");

// Banner SVG is self-contained (text-as-paths); rasterize at 1600 wide on white.
const png = new Resvg(svg, { fitTo: { mode: "width", value: 1600 }, background: "white" });
writeFileSync(join(__dir, "bombvault-banner.png"), png.render().asPng());
console.log("bombvault-banner.png written (1600x500 from bombvault-banner.svg)");
