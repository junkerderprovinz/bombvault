// The activity log's "jump to the newest line" button, measured in a real
// browser: it has to sit centred under the lines and leave the log's
// scrollbar free, on every width the app ships.
import { expect, test, type Page } from "@playwright/test";

const HOUR = 3600;
const NOW = Math.floor(Date.now() / 1000);

// Enough finished runs that the log scrolls at every project width.
const RUNS = Array.from({ length: 60 }, (_, i) => ({
  id: `run-${i}`,
  targetId: `c-${i}`,
  kind: "backup",
  status: "success",
  startedAt: NOW - (i + 1) * HOUR,
  finishedAt: NOW - (i + 1) * HOUR + 90,
  snapshotId: `s${i}`.padEnd(8, "0"),
  bytes: 1024 * 1024 * (i + 1),
  error: "",
  acknowledged: false,
  target: `container-${i}`,
  domain: "container",
}));

async function openLog(page: Page) {
  await page.route("**/api/runs", (route) => route.fulfill({ json: { ok: true, runs: RUNS } }));
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "de"));
  await page.goto("/dashboard");
  const card = page.locator("#bv-main h2", { hasText: "Aktivitätsprotokoll" }).locator("xpath=..");
  const log = card.locator(".overflow-y-auto");
  await expect(log.getByText("container-59").first()).toBeAttached();
  return log;
}

test("the jump to the newest line sits centred under the log and clear of its scrollbar", async ({ page }) => {
  const log = await openLog(page);
  await log.evaluate((el) => {
    el.scrollTop = 0;
    el.dispatchEvent(new Event("scroll"));
  });
  const jump = page.getByRole("button", { name: "Zum Aktuellen springen" });
  await expect(jump).toBeVisible();

  const box = await log.evaluate((el) => {
    const r = el.getBoundingClientRect();
    // clientWidth stops where a classic scrollbar starts.
    return { left: r.left, width: r.width, contentRight: r.left + el.clientLeft + el.clientWidth };
  });
  const btn = await jump.boundingBox();
  if (!btn) throw new Error("the jump button has no box");

  expect(Math.abs(btn.x + btn.width / 2 - (box.left + box.width / 2))).toBeLessThanOrEqual(2);
  expect(btn.x + btn.width).toBeLessThan(box.contentRight);

  await jump.click();
  await expect(jump).toBeHidden();
});
