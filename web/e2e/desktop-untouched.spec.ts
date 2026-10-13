// What every routed page looks like at desktop width. On both desktop
// projects (768px is the 48rem boundary, 1280px a comfortable window) the
// sidebar is visible, the phone's bottom bar is not in the DOM at all, and
// the page scrolls inside bv-main.
//
// A loop over the routes and not a few spot checks, so a route that is added
// without joining it is a visible gap and a regression on one page fails by
// name. /receiver and /fleet ride through their redirects to /instances.
//
// The loop runs on the empty states of a fresh database, since it is about
// the shell and not about data. The pages with a phone layout of their own
// then show that nothing of it reaches the desktop.
import { expect, test, type Page } from "@playwright/test";
import { container, item, stageEntries } from "./entries";

// A handler can still wait on the real server when its test has passed;
// what it throws then is dropped.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

// The two desktop projects from playwright.config.ts (>= the 48rem chrome
// switch). Branching on the project name, the mobile-shell.spec.ts pattern,
// keeps every assertion honest about which contract each project verifies.
const DESKTOP_PROJECTS = new Set(["desktop-1280", "desktop-768"]);

// Every destination of the router (web/src/app/router.tsx), with one entry's
// page for all of them, plus /receiver and /fleet, which redirect to
// /instances. /containers stands for the links to a kind's own page, which
// redirect to the list. /, /jobs, /pull and the unlisted /glyphs sheet are
// not destinations.
const ROUTES = [
  "/dashboard",
  "/anomalies",
  "/backups",
  "/backups/flash/flash",
  "/storage",
  "/containers",
  "/restore",
  "/instances",
  "/receiver",
  "/fleet",
  "/settings",
];

for (const route of ROUTES) {
  test(`desktop untouched at ${route}`, async ({ page }, testInfo) => {
    test.skip(!DESKTOP_PROJECTS.has(testInfo.project.name), "desktop-only: the desktop-invariance contract");
    await page.goto(route);
    await expect(page.getByTestId("desktop-sidebar")).toBeVisible();
    await expect(page.getByTestId("bottom-nav")).toHaveCount(0);
    await expect(page.locator("#bv-main")).toHaveCount(1);
  });
}

// The geometry half of "untouched": presence is not invariance. The shell's
// desktop chrome sits at a contracted geometry on every destination: the
// sidebar rail starts at the shell's 1rem gutter and ends left of the
// `bv-main` scroller, and every route must reproduce /dashboard's boxes
// within a 1px tolerance (subpixel rounding across engines). Two layers
// hold that line: the absolute asserts pin the contract itself, so a shift
// applied uniformly everywhere (a base margin change on the shell root)
// fails exactly like a per-route drift would; the relational asserts pin
// consistency, so one destination cannot sit differently from the others.
// jsdom computes no layout, so this guard only runs where layout is real.
const GEOMETRY_TOLERANCE_PX = 1;

/** The shell's two structural boxes, the sidebar rail and the `bv-main`
 *  scroller: as x/width only: y/height legitimately vary per page content,
 *  horizontal geometry may not. */
async function shellGeometry(page: Page) {
  return page.evaluate(() => {
    const box = (el: Element | null) => {
      if (!el) throw new Error("shell element missing");
      const r = el.getBoundingClientRect();
      return { x: r.x, width: r.width };
    };
    return {
      sidebar: box(document.querySelector('[data-testid="desktop-sidebar"]')),
      main: box(document.querySelector("#bv-main")),
    };
  });
}

/** The absolute geometry contract, asserted on whichever page is loaded:
 *  the rail's left edge sits at the 1rem gutter (measured the way the CSS
 *  means it, from the document root's computed font size) and the rail ends
 *  left of the scroller. */
async function assertAbsoluteGeometry(page: Page, label: string) {
  const { sidebar, main } = await shellGeometry(page);
  const remPx = await page.evaluate(() =>
    parseFloat(getComputedStyle(document.documentElement).fontSize)
  );
  expect(
    Math.abs(sidebar.x - remPx),
    `${label}: the sidebar starts at the shell's 1rem gutter (x=${sidebar.x}, 1rem=${remPx}px)`
  ).toBeLessThanOrEqual(GEOMETRY_TOLERANCE_PX);
  expect(
    sidebar.x + sidebar.width,
    `${label}: the sidebar ends left of the bv-main scroller`
  ).toBeLessThanOrEqual(main.x + GEOMETRY_TOLERANCE_PX);
}

test("desktop shell geometry is identical across every routed destination", async ({ page }, testInfo) => {
  test.skip(
    !DESKTOP_PROJECTS.has(testInfo.project.name),
    "desktop-only: the geometry half of the desktop-invariance contract"
  );
  await page.goto("/dashboard");
  // The SPA mounts after the document loads, and evaluate() does not
  // auto-wait: anchor both elements first, exactly like the presence
  // checks above, or the measure races the mount and finds nothing.
  await expect(page.getByTestId("desktop-sidebar")).toBeVisible();
  await expect(page.locator("#bv-main")).toBeVisible();
  await assertAbsoluteGeometry(page, "/dashboard");
  const baseline = await shellGeometry(page);
  for (const route of ROUTES) {
    await page.goto(route);
    await expect(page.getByTestId("desktop-sidebar")).toBeVisible();
    await expect(page.locator("#bv-main")).toBeVisible();
    await assertAbsoluteGeometry(page, route);
    const boxes = await shellGeometry(page);
    for (const key of ["sidebar", "main"] as const) {
      expect(
        Math.abs(boxes[key].x - baseline[key].x),
        `${route}: the ${key} rail's x drifted from the /dashboard baseline`
      ).toBeLessThanOrEqual(GEOMETRY_TOLERANCE_PX);
      expect(
        Math.abs(boxes[key].width - baseline[key].width),
        `${route}: the ${key} rail's width drifted from the /dashboard baseline`
      ).toBeLessThanOrEqual(GEOMETRY_TOLERANCE_PX);
    }
  }
});

// The >=48rem leakage pass for /dashboard: the one page with a phone face
// asserts that nothing its phone half adds exists in the desktop DOM.
// Role/text-based wherever possible so a restyle of the mobile chrome
// cannot silently outdate the guard; the StickyActionBar is found by its
// test id, which a restyle leaves alone.
const DASHBOARD_ROUTES = ["/dashboard"];

for (const route of DASHBOARD_ROUTES) {
  test(`desktop untouched of the Dashboard phone chrome at ${route}`, async ({ page }, testInfo) => {
    test.skip(!DESKTOP_PROJECTS.has(testInfo.project.name), "desktop-only: the >=48rem leakage contract");
    await page.goto(route);

    // The sticky-in-flow action bar (the thumb-zone trigger's host).
    await expect(page.getByTestId("sticky-action-bar")).toHaveCount(0);
    // The filled full-width accent control, the Backup Everything trigger's
    // signature (Button tone="accent" over the caller's w-full stage); no
    // desktop control carries it.
    await expect(page.locator("button.w-full.bg-accent")).toHaveCount(0);
    // The page's main action stands in the page action corner instead.
    await expect(page.getByTestId("page-actions").getByRole("button", { name: "Back up everything now" })).toBeVisible();
  });
}

// The Backups list shows a phone twenty rows at a time. The desktop shows
// every row at once and has nothing to load.
test("desktop untouched of the phone's window on the Backups list", async ({ page }, testInfo) => {
  test.skip(!DESKTOP_PROJECTS.has(testInfo.project.name), "desktop-only: the >=48rem leakage contract");
  const names = Array.from({ length: 45 }, (_, i) => `svc-${String(i).padStart(2, "0")}`);
  await stageEntries(page, {
    items: names.map((key) => item({ key })),
    containers: names.map((name) => container({ name })),
  });
  await page.goto("/backups");

  await expect(page.getByTestId("backup-row")).toHaveCount(45);
  await expect(page.getByText("Load more")).toHaveCount(0);
  await expect(page.getByTestId("sticky-action-bar")).toHaveCount(0);
  await expect(page.locator("button.w-full.bg-accent")).toHaveCount(0);
});

// Positive half for the Dashboard page: the desktop customizable grid is not
// merely free of mobile chrome, it is present. The customize pencil is
// desktop-only chrome in service of that grid (it mounts only at md+), so it
// being visible proves the desktop page rendered rather than the phone's
// fixed Home block order (where the pencil has nothing to edit and is gated
// away entirely).
test("desktop dashboard keeps the customizable grid chrome", async ({ page }, testInfo) => {
  test.skip(!DESKTOP_PROJECTS.has(testInfo.project.name), "desktop-only: the desktop-invariance contract");
  await page.goto("/dashboard");
  await expect(page.getByRole("heading", { level: 1, name: "Overview" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Customize" })).toBeVisible();
});
