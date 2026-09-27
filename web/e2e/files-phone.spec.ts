// The Folders page at phone width, with sets staged at the route layer (a
// fresh harness database has none). On the phones: the 24px card rhythm, no
// pan, no control past the edge, and each card's actions on their own row
// below the name. On the desktop: the 40px rhythm and a header whose actions
// stay on one line. German, because its labels run longest.
import { expect, test, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

const set = (id: string, name: string, path: string, pathExists: boolean, excludes: string[] = []) => ({
  id: id.padEnd(32, "0"),
  name,
  path,
  excludes,
  enabled: true,
  lastBackup: 0,
  scheduleCadence: "",
  repo: "",
  repoEffective: "user/bombvault/files",
  effectiveSchedule: { kind: "none", spec: "", alsoSpec: "" },
  pathExists,
});

const FILE_SETS = [
  set("a1", "Fotoarchiv Familienurlaube Südtirol", "user/fotos/archiv/familienurlaube/suedtirol-2019-bis-2026", true, ["*.tmp", "Thumbs.db"]),
  set("b2", "Dokumente Steuererklärungen", "user/dokumente/steuer", false),
];

async function bootGerman(page: Page, width: number): Promise<void> {
  await page.route("**/api/files", (route) => route.fulfill({ json: { ok: true, fileSets: FILE_SETS } }));
  // Same seeding as narrow-viewport.spec.ts: the stored locale is the look,
  // and the server's display prefs must not overwrite it mid-boot.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => window.localStorage.setItem("bv-lang", "de"));
  await page.setViewportSize({ width, height: 800 });
  await page.goto("/files");
  await expect(page.getByRole("heading", { level: 1, name: "Ordner" })).toBeVisible();
  await expect(page.getByText(FILE_SETS[1].name)).toBeVisible();
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => {})),
    ),
  );
}

async function cardGap(page: Page): Promise<string> {
  return page
    .locator("#bv-main .max-w-6xl")
    .first()
    .evaluate((root) => getComputedStyle(root).rowGap);
}

for (const width of [320, 360]) {
  test(`folders @ ${width}px: nothing pans, clips or renders stray markup`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone rhythm lives below 48rem");
    await bootGerman(page, width);

    expect(await cardGap(page)).toBe("24px");

    const layout = await page.evaluate(() => {
      const main = document.querySelector("#bv-main");
      const vw = window.innerWidth;
      const clipped = main
        ? [...main.querySelectorAll("button, a, input, select, textarea, [role='switch']")]
            .filter((el) => {
              const r = el.getBoundingClientRect();
              return r.width > 0 && (r.right > vw + 1 || r.left < -1);
            })
            .map((el) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40))
        : null;
      return {
        docPan: document.documentElement.scrollWidth - vw,
        mainPan: main ? main.scrollWidth - main.clientWidth : null,
        clipped,
      };
    });
    expect(layout.mainPan, "#bv-main is missing").not.toBeNull();
    expect(layout.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
    expect(layout.mainPan!, "#bv-main scrolls horizontally").toBeLessThanOrEqual(1);
    expect(layout.clipped, "controls clip the viewport edge").toEqual([]);
    // A backtick in JSX text compiles fine and renders on the page.
    await expect(page.locator("#bv-main")).not.toContainText("`");

    // The actions take their own row under the name, so a long name keeps
    // the width it needs.
    for (const { name } of FILE_SETS) {
      const card = page.locator("div.rounded-card").filter({ hasText: name }).first();
      const nameBox = (await card.getByText(name, { exact: true }).boundingBox())!;
      const editBox = (await card.getByRole("button", { name: "Bearbeiten" }).boundingBox())!;
      expect(editBox.y, `the actions of "${name}" share the name's row`).toBeGreaterThanOrEqual(nameBox.y + nameBox.height);
    }
  });
}

test("folders on the desktop keep the 40px rhythm and a one-line header", async ({ page }, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await bootGerman(page, testInfo.project.use.viewport!.width);

  expect(await cardGap(page)).toBe("40px");
  const tops = await page
    .getByRole("heading", { level: 1 })
    .locator("xpath=../following-sibling::div[1]")
    .locator("button")
    .evaluateAll((buttons) => [...new Set(buttons.map((b) => Math.round(b.getBoundingClientRect().top)))]);
  expect(tops, "the header actions wrapped").toHaveLength(1);
});
