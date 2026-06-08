/**
 * Generate icon.png and bombvault-banner.png from icon.svg.
 * Step 1 (this script): render SVG at two sizes with @resvg/resvg-js.
 * Step 2: Python (gen-assets.py) composites the banner logo onto a white canvas.
 *
 * Run via gen-assets.sh (or gen-assets.cmd).
 */
import { readFileSync, writeFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { execSync } from "node:child_process";

const require = createRequire(import.meta.url);
const globalNodeModules = execSync("npm root -g").toString().trim();
const { Resvg } = require(`${globalNodeModules}/@resvg/resvg-js`);

const __dir = dirname(fileURLToPath(import.meta.url));
const svg = readFileSync(join(__dir, "icon.svg"), "utf8");

// icon.png — 512×512, white background (CA icon)
const iconResvg = new Resvg(svg, {
  fitTo: { mode: "width", value: 512 },
  background: "white",
});
writeFileSync(join(__dir, "icon.png"), iconResvg.render().asPng());
console.log("icon.png written (512×512)");

// banner-logo.png — logo at 460px height for Pillow to composite
// SVG aspect: 914.72 / 886.45 ≈ 1.032 (slightly wider than tall)
const LOGO_H = 460;
const LOGO_W = Math.round(LOGO_H * (914.72 / 886.45));
const logoResvg = new Resvg(svg, {
  fitTo: { mode: "height", value: LOGO_H },
  background: "white",
});
writeFileSync(join(__dir, "_banner-logo-tmp.png"), logoResvg.render().asPng());
console.log(`_banner-logo-tmp.png written (~${LOGO_W}×${LOGO_H})`);
