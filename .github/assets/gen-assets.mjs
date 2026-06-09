/**
 * Render step for the BombVault assets (via @resvg/resvg-js, global install).
 *   - icon_white_hi.png : icon at 1024px on WHITE — source for the flood-fill
 *                         transparency step in gen-assets.py.
 *   - _banner-logo-tmp.png : logo at 460px height on WHITE for the banner.
 *
 * The logo has NO white fills of its own — its white outlines are negative space
 * (the background showing through). So we render on white, then gen-assets.py
 * makes ONLY the outer box transparent, keeping the interior white outlines.
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

// Hi-res icon on white (flood-fill source for the transparent icon).
const icon = new Resvg(svg, { fitTo: { mode: "width", value: 1024 }, background: "white" });
writeFileSync(join(__dir, "icon_white_hi.png"), icon.render().asPng());
console.log("icon_white_hi.png written (1024 wide, white)");

// Banner logo at 460px height on white (banner stays white per the style guide).
const logo = new Resvg(svg, { fitTo: { mode: "height", value: 460 }, background: "white" });
writeFileSync(join(__dir, "_banner-logo-tmp.png"), logo.render().asPng());
console.log("_banner-logo-tmp.png written (460 high, white)");
