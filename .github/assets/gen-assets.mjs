/**
 * Render step for the BombVault banner logo (via @resvg/resvg-js, global install).
 *   - _banner-logo-tmp.png : logo at 460px height on WHITE for the banner.
 *
 * The container / CA icon is NOT rendered here — it comes from the hand-made
 * transparent `icon-source.png` (downscaled in gen-assets.py). Flood-filling the
 * SVG render to transparency left a white halo on Unraid's dark Docker tab, and
 * the user wants the container logo cleanly transparent.
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
const svg = readFileSync(join(__dir, "icon.svg"), "utf8");

// Banner logo at 460px height on white (banner stays white per the style guide).
const logo = new Resvg(svg, { fitTo: { mode: "height", value: 460 }, background: "white" });
writeFileSync(join(__dir, "_banner-logo-tmp.png"), logo.render().asPng());
console.log("_banner-logo-tmp.png written (460 high, white)");
