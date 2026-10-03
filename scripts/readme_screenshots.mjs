// Builds the README screenshots in .github/assets/screenshots: each page of a
// running BombVault inside a drawn browser window next to a caption, on the
// dark relief background the store pictures use.
//
// Point it at a demo instance with invented data, never at a real server: the
// pictures show container names, sizes and addresses. The receiver shot needs
// a second instance in the same group with a received repository.
//
// Every layer is rendered at twice the size, which keeps the text sharp
// through the window's tilt.
//
// Deps (global): playwright-core with its Chromium installed.
// Run: node scripts/readme_screenshots.mjs <sender url> <receiver url>

import { execSync } from "node:child_process";
import { copyFileSync, existsSync, readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const { chromium } = require(`${execSync("npm root -g").toString().trim()}/playwright-core`);

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const out = join(root, ".github", "assets", "screenshots");
const docs = join(root, "docs", "assets", "screenshots");
const [sender, receiver] = process.argv.slice(2);
if (!sender || !receiver) throw new Error("usage: node scripts/readme_screenshots.mjs <sender url> <receiver url>");

// `shown` is the address the window's bar prints, matching the example
// servers of the app's store pictures rather than the demo's own.
const SHOTS = [
  {
    file: "dashboard.png",
    base: sender,
    path: "/dashboard",
    shown: "192.168.1.20:3443",
    caption: "All your backups at a glance, <em>before you need one</em>",
  },
  {
    file: "recovery.png",
    base: sender,
    path: "/recovery",
    shown: "192.168.1.20:3443",
    caption: "Server gone? <em>Bring it all back.</em>",
  },
  {
    file: "containers.png",
    base: sender,
    path: "/containers",
    shown: "192.168.1.20:3443",
    caption: "Each container gets <em>its own switch</em>",
    // BombVault's own card comes first and only says it does not back itself up.
    scrollTo: "homeassistant",
  },
  { file: "settings.png", base: sender, path: "/settings", shown: "192.168.1.20:3443", caption: "No Save button. <em>It just saves.</em>" },
  {
    file: "receiver.png",
    base: receiver,
    path: "/instances",
    shown: "10.8.0.2:3443",
    caption: "Trust the copy. <em>Check it yourself.</em>",
    open: "Details",
  },
];
const SUB = "BombVault for Unraid and Docker";

const W = 1600;
const H = 940;
const VIEW_W = 1080;
const VIEW_H = 720;
const BAR_H = 48;

async function cached(file, url) {
  const path = join(tmpdir(), `BombVault-${file}`);
  if (!existsSync(path)) {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`${file}: fetch ${res.status}`);
    writeFileSync(path, Buffer.from(await res.arrayBuffer()));
  }
  return readFileSync(path);
}

const dataUrl = (buf, type) => `data:${type};base64,${buf.toString("base64")}`;
const bree = dataUrl(
  await cached("BreeSerif-Regular.ttf", "https://github.com/google/fonts/raw/main/ofl/breeserif/BreeSerif-Regular.ttf"),
  "font/ttf",
);
const lato = dataUrl(await cached("Lato-Regular.ttf", "https://github.com/google/fonts/raw/main/ofl/lato/Lato-Regular.ttf"), "font/ttf");
const logo = dataUrl(readFileSync(join(root, "web", "public", "logo-light.svg")), "image/svg+xml");

async function page(browser, { base, path, scrollTo, open }) {
  const ctx = await browser.newContext({ viewport: { width: VIEW_W, height: VIEW_H }, deviceScaleFactor: 2, colorScheme: "dark" });
  const tab = await ctx.newPage();
  await tab.goto(base + path);
  await tab.waitForSelector("main");
  await tab.evaluate(() => document.fonts.ready);
  // The cards fill in from their own requests after the first paint.
  await tab.waitForTimeout(2500);
  if (open) await tab.getByRole("button", { name: open }).first().click();
  if (scrollTo) {
    await tab
      .getByText(scrollTo, { exact: true })
      .first()
      .evaluate((el) => {
        const card = el.closest(".rounded-card") ?? el;
        card.style.scrollMarginTop = "20px";
        card.scrollIntoView();
      });
  }
  await tab.waitForFunction(() => !/Loading/.test(document.body.innerText), null, { timeout: 60000 });
  if (open || scrollTo) await tab.waitForTimeout(800);
  const png = await tab.screenshot();
  await ctx.close();
  return png;
}

function frame(shot, png) {
  return `<!doctype html><html><head><meta charset="utf-8"><style>
@font-face { font-family: "Bree Serif"; src: url(${bree}); }
@font-face { font-family: Lato; src: url(${lato}); }
* { box-sizing: border-box; margin: 0; }
body { width: ${W}px; height: ${H}px; overflow: hidden; position: relative; font-family: Lato, sans-serif; background: #0c0c0b; }
.backdrop, .backdrop * { position: absolute; }
.backdrop { inset: 0; overflow: hidden; }
.wall { inset: 0; background: radial-gradient(55% 60% at 62% 35%, #26231d, #0f0e0c 72%); }
.mark { left: -9%; top: -4%; width: 46%; transform: rotate(-10deg); opacity: .32;
  filter: grayscale(1) brightness(.36) contrast(1.2) drop-shadow(-2px -2px 0 rgba(255,255,255,.16)) drop-shadow(12px 18px 26px rgba(0,0,0,.85)); }
.vignette { inset: 0; box-shadow: inset 0 0 200px rgba(0,0,0,.6); }
.copy { position: absolute; left: 80px; top: 0; bottom: 0; width: 330px; display: flex; flex-direction: column; justify-content: center; gap: 26px; }
.copy img { width: 72px; }
h1 { font: 400 46px/1.14 "Bree Serif", serif; color: #f4f4f4; text-wrap: balance; }
h1 em { font-style: normal; color: #FCC419; }
.copy p { font-size: 21px; color: #9d9481; }
.stage { position: absolute; left: 470px; top: ${(H - VIEW_H - BAR_H) / 2}px; width: ${VIEW_W}px; height: ${VIEW_H + BAR_H}px; perspective: 2400px; }
.floor { position: absolute; left: 6%; right: 6%; bottom: -34px; height: 60px; border-radius: 50%; background: rgba(0,0,0,.75); filter: blur(28px); }
.win { position: absolute; inset: 0; border-radius: 12px; background: #1c1c1c; transform: rotateY(-7deg) rotateX(3deg);
  box-shadow: 0 0 0 1px #333, 0 1px 0 1px rgba(255,255,255,.05), 0 2px 4px rgba(0,0,0,.35), 0 16px 32px rgba(0,0,0,.4), 0 48px 96px rgba(0,0,0,.5); }
.bar { height: ${BAR_H}px; display: flex; align-items: center; gap: 16px; padding: 0 16px; border-radius: 12px 12px 0 0;
  background: linear-gradient(#242424, #1c1c1c); border-bottom: 1px solid #0e0e0e; }
.dots { display: flex; gap: 8px; } .dots i { width: 12px; height: 12px; border-radius: 50%; background: #3d3d3d; }
.nav { display: flex; gap: 14px; color: #7c7c7c; } .nav svg { width: 16px; height: 16px; display: block; }
.url { flex: 1; height: 30px; border-radius: 15px; background: #2b2b2b; color: #b4b4b4; font-size: 14px; display: flex; align-items: center; gap: 8px; padding: 0 16px; }
.url svg { width: 12px; height: 12px; color: #7c7c7c; }
.more { color: #7c7c7c; font-size: 18px; }
.view img { display: block; border-radius: 0 0 12px 12px; }
</style></head><body>
<div class="backdrop"><div class="wall"></div><img class="mark" src="${logo}"><div class="vignette"></div></div>
<div class="copy"><img src="${logo}"><h1>${shot.caption}</h1><p>${SUB}</p></div>
<div class="stage">
  <div class="floor"></div>
  <div class="win">
    <div class="bar">
      <div class="dots"><i></i><i></i><i></i></div>
      <div class="nav">
        <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M10 3 5 8l5 5"/></svg>
        <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="m6 3 5 5-5 5"/></svg>
        <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M13 8a5 5 0 1 1-1.5-3.6M13 2.5V5h-2.5"/></svg>
      </div>
      <div class="url"><svg viewBox="0 0 12 12" fill="currentColor"><rect x="2" y="5" width="8" height="6" rx="1.2"/><path d="M4 5V3.6a2 2 0 0 1 4 0V5" fill="none" stroke="currentColor" stroke-width="1.3"/></svg>${shot.shown}${shot.path}</div>
      <div class="more">&#8942;</div>
    </div>
    <div class="view"><img src="${dataUrl(png, "image/png")}" width="${VIEW_W}" height="${VIEW_H}"></div>
  </div>
</div>
</body></html>`;
}

const browser = await chromium.launch();
try {
  for (const shot of SHOTS) {
    const png = await page(browser, shot);
    const canvas = await browser.newPage({ viewport: { width: W, height: H }, deviceScaleFactor: 2 });
    await canvas.setContent(frame(shot, png));
    await canvas.evaluate(() => document.fonts.ready);
    await canvas.evaluate(() => Promise.all([...document.images].map((img) => img.decode())));
    await canvas.screenshot({ path: join(out, shot.file) });
    await canvas.close();
    copyFileSync(join(out, shot.file), join(docs, shot.file));
    console.log(`wrote ${shot.file}`);
  }
} finally {
  await browser.close();
}
