/**
 * Render icon.svg using a real Chromium browser via Playwright.
 * This gives pixel-perfect output identical to how a browser displays it.
 */
import { readFileSync, writeFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const __dir = dirname(fileURLToPath(import.meta.url));
const svg = readFileSync(join(__dir, "icon.svg"), "utf8");

// SVG viewBox dimensions
const VB_W = 914.72, VB_H = 886.45;

async function renderAt(outFile, pxW) {
  const pxH = Math.round(pxW * VB_H / VB_W);
  const html = `<!DOCTYPE html>
<html><head><meta charset="utf-8">
<style>*{margin:0;padding:0;background:white;}
svg{width:${pxW}px;height:${pxH}px;display:block;}</style>
</head><body>${svg}</body></html>`;

  const browser = await chromium.launch();
  const page = await browser.newPage();
  await page.setViewportSize({ width: pxW + 20, height: pxH + 20 });
  await page.setContent(html, { waitUntil: "networkidle" });
  const clip = { x: 0, y: 0, width: pxW, height: pxH };
  const buf = await page.screenshot({ clip, omitBackground: false });
  writeFileSync(join(__dir, outFile), buf);
  console.log(`${outFile} written (${pxW}×${pxH})`);
  await browser.close();
}

await renderAt("icon.png", 512);
await renderAt("_banner-logo-tmp.png", Math.round(460 * VB_W / VB_H));
