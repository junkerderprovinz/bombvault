// Every tab in a desktop window between 780 and 1400px wide, with the sidebar
// beside it: no control reaches past the window, the page does not pan, and
// no button cuts its label short. German, because its labels run longest; the
// domains are on and a folder set is staged, so the header rows carry every
// action they can.
import { expect, test, type Page } from "@playwright/test";

const WIDTHS = [780, 900, 1024, 1400];

const PAGES = ["/dashboard", "/anomalies", "/containers", "/vms", "/flash", "/config", "/files", "/zfs", "/instances", "/recovery", "/settings"];

const FILE_SET = {
  id: "a1".padEnd(32, "0"),
  name: "Dokumente vom Hauptserver im Keller",
  path: "user/dokumente/hauptserver/keller",
  excludes: [],
  enabled: true,
  lastBackup: 0,
  scheduleCadence: "",
  repo: "",
  repoEffective: "user/bombvault/files",
  effectiveSchedule: { kind: "none", spec: "", alsoSpec: "", reason: "schedule-off" },
  pathExists: true,
};

async function stage(page: Page): Promise<void> {
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "de"));
  await page.route("**/api/settings", async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    const response = await route.fetch();
    const body = await response.json();
    const on = {
      containersEnabled: true, vmsEnabled: true, flashEnabled: true, configEnabled: true, filesEnabled: true,
      zfsEnabled: true, receiverEnabled: true, fleetEnabled: true, pullEnabled: true, anomalyEnabled: true,
    };
    await route.fulfill({ response, json: { ...body, settings: { ...body.settings, ...on } } });
  });
  await page.route("**/api/files", (route) => route.fulfill({ json: { ok: true, fileSets: [FILE_SET] } }));
}

async function settle(page: Page): Promise<void> {
  await page.waitForLoadState("networkidle");
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => {})),
    ),
  );
}

for (const path of PAGES) {
  test(`${path}: nothing runs past a desktop window from 780 to 1400px`, async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== "desktop-1280", "sets its own window widths");
    await stage(page);
    await page.goto(path);
    for (const width of WIDTHS) {
      await page.setViewportSize({ width, height: 900 });
      await settle(page);
      const layout = await page.evaluate(() => {
        const main = document.querySelector("#bv-main")!;
        const vw = document.documentElement.clientWidth;
        const name = (el: Element) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40);
        const controls = [...main.querySelectorAll("button, a, input, select, textarea, [role='switch']")].filter(
          (el) => el.getBoundingClientRect().width > 0,
        );
        return {
          pan: main.scrollWidth - main.clientWidth,
          past: controls.filter((el) => el.getBoundingClientRect().right > vw + 1).map(name),
          cut: [...main.querySelectorAll(".glim-btn-label")]
            .filter((el) => el.getBoundingClientRect().width > 0 && el.scrollWidth > el.clientWidth + 1)
            .map((el) => el.textContent),
        };
      });
      expect(layout.past, `${width}px: controls past the window`).toEqual([]);
      expect(layout.cut, `${width}px: button labels cut short`).toEqual([]);
      expect(layout.pan, `${width}px: the page pans`).toBeLessThanOrEqual(0);
    }
  });
}
