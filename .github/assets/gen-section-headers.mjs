/**
 * Draws every `##` heading of README.md as a chapter band (number, name, a rule
 * running out to the right) in a light and a dark SVG, and writes the heading
 * back as a <picture> in a paragraph.
 *
 * GitHub draws a rule under every <h2>, so the band is no heading. The link
 * around it carries the id the table of contents points to. Running it again
 * reads the names back from the alt text, so a renamed section only needs its
 * new name in the README, as a plain `## 3. New name` line.
 *
 * The titles of the download groups get a smaller band with the group's icon in
 * place of the number. scripts/gen_download_buttons.py writes their markup and
 * lists them in download-buttons/groups.json, so run that one first.
 *
 * The repo's font and the ink of each theme come from
 * .github/assets/section-headers.json: { "font": "<URL of a TTF or WOFF2>", "light": "#…", "dark": "#…" }
 *
 * Deps (global): opentype.js, and uv for a WOFF2 font. Run: node .github/assets/gen-section-headers.mjs
 */
import { readFileSync, writeFileSync, existsSync, mkdirSync, readdirSync, rmSync } from "node:fs";
import { join, dirname, basename } from "node:path";
import { fileURLToPath } from "node:url";
import { tmpdir } from "node:os";
import { createRequire } from "node:module";
import { execSync } from "node:child_process";

const require = createRequire(import.meta.url);
const opentype = require(`${execSync("npm root -g").toString().trim()}/opentype.js`);

const HERE = dirname(fileURLToPath(import.meta.url));
const README = join(HERE, "..", "..", "README.md");
const OUT = join(HERE, "sections");
const REL = ".github/assets/sections";
const config = JSON.parse(readFileSync(join(HERE, "section-headers.json"), "utf8"));

const fontPath = join(tmpdir(), "section-headers-" + basename(decodeURIComponent(new URL(config.font).pathname)));
if (!existsSync(fontPath)) {
  const res = await fetch(config.font);
  if (!res.ok) throw new Error(`font fetch ${res.status}`);
  writeFileSync(fontPath, Buffer.from(await res.arrayBuffer()));
}
// opentype.js reads no WOFF2; fontTools unpacks it, brotli being what the glyph table is packed with.
const ttfPath = fontPath.replace(/\.woff2$/i, ".ttf");
if (ttfPath !== fontPath && !existsSync(ttfPath)) {
  execSync(`uv run -q --no-project --with fonttools --with brotli python -c "from fontTools.ttLib import TTFont; f = TTFont(r'${fontPath}'); f.flavor = None; f.save(r'${ttfPath}')"`);
}
const buf = readFileSync(ttfPath);
const font = opentype.parse(buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength));

// At width="100%" in GitHub's 830px column the band stands about 62px high and the
// name is about 36px, well above the 24px of a plain heading.
const W = 1280, H = 96, NUM = 66, NAME = 56;

const r = (n) => +n.toFixed(2);
// opentype's toPathData writes NaN for some coordinates close to zero.
const pathData = (p) =>
  p.commands.map((c) =>
    c.type === "Z" ? "Z"
    : c.type === "Q" ? `Q${r(c.x1)} ${r(c.y1)} ${r(c.x)} ${r(c.y)}`
    : c.type === "C" ? `C${r(c.x1)} ${r(c.y1)} ${r(c.x2)} ${r(c.y2)} ${r(c.x)} ${r(c.y)}`
    : `${c.type}${r(c.x)} ${r(c.y)}`).join("");

// Glyph by glyph with the font's kerning. opentype.js's own layout runs the
// substitution tables first and stops on some it does not support, DejaVu's
// among them; a heading needs none of them.
function shape(s, size) {
  const scale = size / font.unitsPerEm;
  const glyphs = [];
  let x = 0, prev = null;
  for (const ch of s) {
    const g = font.charToGlyph(ch);
    if (prev) x += font.getKerningValue(prev, g) * scale;
    glyphs.push({ g, x });
    x += g.advanceWidth * scale;
    prev = g;
  }
  return { glyphs, width: x };
}
const width = (s, size) => shape(s, size).width;
const text = (s, x, y, size, fill) =>
  `<path d="${shape(s, size).glyphs.map(({ g, x: gx }) => pathData(g.getPath(x + gx, y, size))).join("")}" fill="${fill}"/>`;

// The table of contents has no number and takes a section sign in its place.
function band(number, name, ink) {
  const label = number ? number.padStart(2, "0") : "§";
  const numW = width(label, NUM);
  const nameX = numW + 34;
  const ruleX = nameX + width(name, NAME) + 24;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
  ${text(label, 0, 72, NUM, ink)}
  <rect x="${r(numW + 14)}" y="22" width="3" height="56" rx="1.5" fill="${ink}"/>
  ${text(name, nameX, 70, NAME, ink)}
  <rect x="${r(ruleX)}" y="51" width="${r(W - ruleX)}" height="2" fill="${ink}"/>
</svg>
`;
}

// A group title is a step below a section: wider canvas, so smaller on screen,
// and the icon drawn in strokes on a 24 unit grid.
const GW = 1620, GH = 90, ICON = 54, GNAME = 52;

function groupBand(icon, word, ink) {
  const nameX = ICON + 34;
  const ruleX = nameX + width(word, GNAME) + 24;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${GW}" height="${GH}" viewBox="0 0 ${GW} ${GH}">
  <g transform="translate(0 ${(GH - ICON) / 2 + 1}) scale(${ICON / 24})" fill="none" stroke="${ink}" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">${icon}</g>
  <rect x="${ICON + 14}" y="20" width="3" height="52" rx="1.5" fill="${ink}"/>
  ${text(word, nameX, 65, GNAME, ink)}
  <rect x="${r(ruleX)}" y="47" width="${r(GW - ruleX)}" height="2" fill="${ink}"/>
</svg>
`;
}

// GitHub's anchor for a heading: lower case, punctuation dropped, spaces to dashes.
const slug = (s) => s.toLowerCase().replace(/[^\p{L}\p{N} _-]/gu, "").replace(/ /g, "-");

const GENERATED = /^<p><a id="[^"]*".*? alt="([^"]+)".*<\/p>$/;
mkdirSync(OUT, { recursive: true });
// A renamed or removed section leaves no file behind.
for (const f of readdirSync(OUT)) rmSync(join(OUT, f));

const groups = join(HERE, "download-buttons", "groups.json");
for (const { id, word, icon } of existsSync(groups) ? JSON.parse(readFileSync(groups, "utf8")) : []) {
  writeFileSync(join(OUT, `${id}.svg`), groupBand(icon, word, config.light));
  writeFileSync(join(OUT, `${id}-dark.svg`), groupBand(icon, word, config.dark));
  console.log(id);
}

let fenced = false;
// A checkout with core.autocrlf has CRLF, and the README keeps whichever it has.
const source = readFileSync(README, "utf8");
const nl = source.includes("\r\n") ? "\r\n" : "\n";
const lines = source.split(/\r?\n/).map((line) => {
  if (/^\s*(```|~~~)/.test(line)) fenced = !fenced;
  if (fenced) return line;
  const heading = line.match(/^## (.+)$/)?.[1] ?? line.match(GENERATED)?.[1];
  if (!heading) return line;
  const [, number = "", name] = heading.match(/^(?:(\d+)\.\s+)?(.+)$/);
  const id = slug(heading);
  writeFileSync(join(OUT, `${id}.svg`), band(number, name, config.light));
  writeFileSync(join(OUT, `${id}-dark.svg`), band(number, name, config.dark));
  console.log(`${id}`);
  return `<p><a id="${id}" href="#${id}"><picture><source media="(prefers-color-scheme: dark)" srcset="${REL}/${id}-dark.svg"><img src="${REL}/${id}.svg" alt="${heading}" width="100%"></picture></a></p>`;
});
writeFileSync(README, lines.join(nl));
