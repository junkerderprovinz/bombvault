// ---------------------------------------------------------------------------
// Mobile shell e2e: the real-binary contract for the mobile chrome.
//
// The first test proves the chrome switch (bar on mobile / Sidebar on
// desktop, each exactly once and never both). The rest expand to the sheet
// contract: the More trigger opens the More sheet, its rows are the fresh-DB
// registry, it closes through all three BottomSheet paths (Escape, scrim,
// close button), the Tab trap holds focus inside the dialog, and the bar
// stays bottom-docked in landscape. The sign-out parity test runs on all
// four projects: the fresh harness DB has auth disabled, so the row must be
// absent from the More sheet and from the desktop Sidebar footer on the same
// environment (one authEnabled gate, both surfaces, the enabled-side
// behavior is proven by the MoreSheet dom test mocks; real-auth flows are
// out of scope here).
//
// Branching on the Playwright project name (rather than a viewport probe in
// the test) keeps every assertion honest about which contract each project
// verifies.
//
// The bar-exactness test tightens the chrome-switch smoke into the
// exactness contract: the bar renders exactly the registry slots, the
// enabled bar destinations plus the More trigger, nothing else (bar-exactness
// test below).
// ---------------------------------------------------------------------------
import { expect, test, type Page } from "@playwright/test";

// The two device projects from playwright.config.ts. Everything else is a
// desktop project.
const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// The look's truth lives on the server (displayPrefs.ts, #191): at boot the
// page adopts the harness DB's stored display prefs, bv-lang included. The
// DATA_DIR is wiped before every run, not per worker (playwright.config.ts's
// wipe-then-boot webServer command), so nothing survives from a previous
// run; but all four projects still share that one server for the whole run,
// so a locale any worker PUTs would rewrite every localized label for the
// workers that boot after it ("More" -> "Mehr"). So every test cuts the
// boot-time reconciliation fetch: sync()'s fetch failing is the app's own
// documented degradation path ("offline: the cache is the look"), which
// keeps every worker on the harness default locale regardless of what its
// siblings do, with nothing PUT back to the shared server.
async function bootWithoutServerLook(page: Page): Promise<void> {
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.goto("/dashboard");
}

test("chrome switches with the viewport: bar on mobile, Sidebar on desktop", async ({ page }, testInfo) => {
  await bootWithoutServerLook(page);
  if (MOBILE_PROJECTS.has(testInfo.project.name)) {
    // Mobile: the bottom bar is present exactly once, the desktop Sidebar is
    // not in the DOM at all, and main is the scroller.
    await expect(page.getByTestId("bottom-nav")).toBeVisible();
    await expect(page.getByTestId("bottom-nav")).toHaveCount(1);
    await expect(page.getByTestId("desktop-sidebar")).toHaveCount(0);
    await expect(page.locator("#bv-main")).toHaveCount(1);
  } else {
    // Desktop: today's shell, untouched: Sidebar visible, zero bars.
    await expect(page.getByTestId("desktop-sidebar")).toBeVisible();
    await expect(page.getByTestId("bottom-nav")).toHaveCount(0);
  }
});

test("the bar renders exactly the registry slots: enabled destinations + More, nothing else", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: bar exactness is the mobile contract");
  await bootWithoutServerLook(page);
  const bar = page.getByTestId("bottom-nav");
  // The slot row is the only child of the bar-card (the More sheet is a portal
  // to document.body, never a slot); the row's direct children are the slots.
  const slots = bar.locator("div.flex.h-14 > *");
  // Fresh-DB gating is the environment under test: every domain gate off,
  // files_enabled defaults false (internal/store/settings_test.go pins it),
  // so the registry yields Dashboard, Recovery and Containers as bar
  // destinations, plus the always-rendered More trigger = 4 slots. Settings
  // lives in the More sheet, not the bar ("Recovery is what people open on a
  // phone when something went wrong", so it rides the bar). The substance of
  // the exactness contract is the exactness, the bar renders the enabled
  // destinations + More and never a hand-added or leaked slot, per the one
  // nav registry's gate semantics.
  await expect(slots).toHaveCount(4);
  await expect(bar.getByRole("link", { name: "Dashboard" })).toHaveCount(1);
  await expect(bar.getByRole("link", { name: "Recovery" })).toHaveCount(1);
  await expect(bar.getByRole("link", { name: "Containers" })).toHaveCount(1);
  await expect(bar.getByRole("button", { name: "More" })).toHaveCount(1);
  // And nothing else: zero destination links beyond the three named above,
  // no gated destination (files among them) leaks into the bar while its
  // desktop Sidebar gate is off, and Settings is not a bar destination.
  await expect(bar.getByRole("link")).toHaveCount(3);
  await expect(bar.getByRole("link", { name: "Settings" })).toHaveCount(0);
});

test("the More sheet opens with the fresh-DB registry and closes via all three paths", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the More trigger exists below the breakpoint");
  await bootWithoutServerLook(page);

  const bar = page.getByTestId("bottom-nav");
  const moreTrigger = bar.getByRole("button", { name: "More" });
  const sheet = page.getByTestId("more-sheet");

  // Open: the trigger is the bar's More slot; the sheet is the shared
  // BottomSheet carrying the More sheet's content root.
  await moreTrigger.click();
  await expect(sheet).toBeVisible();

  // Fresh-DB rows: every gate off, so the sheet holds exactly one destination
  // link: Settings (Recovery rides the bar, the gated tabs are off), plus
  // the Simple/Advanced view toggle that gives the sheet permanent content,
  // and none of the bar destinations leak into it (the duplicate-destination
  // drift the sheet's contract forbids).
  await expect(sheet.getByRole("link", { name: "Settings" })).toBeVisible();
  expect(await sheet.getByRole("link").count()).toBe(1);
  await expect(sheet.getByRole("link", { name: "Recovery" })).toHaveCount(0);
  await expect(sheet.getByRole("link", { name: "Dashboard" })).toHaveCount(0);
  await expect(sheet.getByRole("button", { name: /view/i })).toHaveCount(1);

  // Close path 1 of 3: Escape (document-level, works wherever focus sits).
  await page.keyboard.press("Escape");
  await expect(sheet).toHaveCount(0);

  // Close path 2 of 3: a click on the scrim itself. The panel is
  // bottom-anchored, so the viewport's top-left corner is always scrim.
  await moreTrigger.click();
  await expect(sheet).toBeVisible();
  await page.locator(".glim-modal-backdrop").click({ position: { x: 10, y: 10 } });
  await expect(sheet).toHaveCount(0);

  // Close path 3 of 3: the header close button (distinct accessible name,
  // ConfirmDialog strict-mode discipline). Scoped to the dialog: pages may
  // carry their own "Close" controls (the dashboard's dismiss chip does).
  await moreTrigger.click();
  await expect(sheet).toBeVisible();
  await page.getByRole("dialog").getByRole("button", { name: "Close" }).click();
  await expect(sheet).toHaveCount(0);

  // Focus trap engagement: the primitive auto-focuses its close button on
  // open; Tab-walking the panel must never move focus outside the dialog,
  // no matter how many times it wraps.
  await moreTrigger.click();
  await expect(sheet).toBeVisible();
  for (let i = 0; i < 12; i++) await page.keyboard.press("Tab");
  const focusInsideDialog = await page.evaluate(() => {
    const active = document.activeElement;
    return active instanceof Element && active.closest('[role="dialog"]') !== null;
  });
  expect(focusInsideDialog).toBe(true);
});

test("sign-out parity: the fresh DB shows the row on neither chrome surface", async ({ page }, testInfo) => {
  await bootWithoutServerLook(page);
  if (MOBILE_PROJECTS.has(testInfo.project.name)) {
    // Mobile: open the sheet: with auth disabled the sign-out row must not
    // exist inside it (the emptiness-adjacent half of the same gate).
    await page.getByTestId("bottom-nav").getByRole("button", { name: "More" }).click();
    await expect(page.getByTestId("more-sheet")).toBeVisible();
  }
  // Both surfaces: zero "Sign out" controls anywhere in the page, the
  // desktop footer's gate and the sheet's gate resolve identically on this
  // environment (authEnabled=false).
  await expect(page.getByRole("button", { name: /sign out/i })).toHaveCount(0);
});

test("the bar stays bottom-docked in landscape", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: landscape contract of the bottom bar");
  // A rotated phone below the 48rem chrome breakpoint: orientation flipped,
  // still compact width, so the bottom bar is the mounted chrome and must
  // sit at the bottom. (At or above 48rem the one switch mounts the desktop
  // rail by the width-only query, a landscape phone at 844px CSS width is
  // "desktop" to it; the semantics are width-only and the
  // landscape boundary pair in narrow-viewport.spec.ts pins both sides.)
  await page.setViewportSize({ width: 740, height: 360 });
  await bootWithoutServerLook(page);
  const box = await page.getByTestId("bottom-nav").boundingBox();
  expect(box).not.toBeNull();
  // Normal-flow sibling (never fixed): the browser reserves the bar's row at
  // the end of the flex column, so its bottom edge is the viewport's bottom
  // edge in this orientation too, no side rail, no floating bar. Allow a
  // 2px tolerance for subpixel rounding.
  expect(box!.y + box!.height).toBeGreaterThanOrEqual(358);
  expect(box!.y + box!.height).toBeLessThanOrEqual(360);
});

test("tapping the already-active bar slot scrolls the scroller back to the top", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: tap-on-active contract of the bottom bar");
  await bootWithoutServerLook(page);
  // Make the scroller deterministically scrollable: the dashboard's content
  // height varies, the mechanism under test does not. grow the page, never
  // shrink the scroller: #bv-main is a flex-1 item of the flex-col shell
  // (Layout.tsx), so an inline height on it is ignored, flex-basis 0% owns
  // the main axis and the used size comes from the flex algorithm (the old
  // style.height = "200px" here was a silent no-op; the test passed only on
  // the dashboard's natural content overflow). A tall first child, the
  // per-route wrapper around the Outlet, guarantees scrollHeight >
  // clientHeight under any flex sizing, independent of what the dashboard
  // renders. Then push the scroller down, tap the active slot (we are on
  // /dashboard), and the Layout-owned scroll-to-top must fire instead of a
  // re-navigation.
  await page.locator("#bv-main").evaluate((el) => {
    const first = el.firstElementChild as HTMLElement | null;
    if (first) first.style.minHeight = `${el.clientHeight + 500}px`;
    el.scrollTop = 300;
  });
  // Honest precondition assert: if a future shell change makes the grow
  // above a no-op again (as the old height shrink was), fail here with a
  // reason instead of as a confusing poll timeout below.
  expect(
    await page.locator("#bv-main").evaluate((el) => el.scrollHeight > el.clientHeight),
  ).toBe(true);
  await expect
    .poll(() => page.locator("#bv-main").evaluate((el) => el.scrollTop))
    .toBeGreaterThan(0);
  // The no-navigation half of the contract, made executable: react-router
  // has no same-location dedup, so an unsuppressed NavLink click also pushes
  // a duplicate /dashboard entry: the scroll-to-top poll below would pass
  // either way, and the first Android back-gesture would appear dead while
  // the second leaves the app. History depth after the tap must equal the
  // depth before it (asserted relative, so engine differences in the
  // absolute starting depth cannot flake it).
  const historyDepthBefore = await page.evaluate(() => history.length);
  await page.getByTestId("bottom-nav").getByRole("link", { name: "Dashboard" }).click();
  await expect
    .poll(() => page.locator("#bv-main").evaluate((el) => el.scrollTop))
    .toBe(0);
  expect(
    await page.evaluate(() => history.length),
    "tapping the active slot must not push a history entry, the bar's contract suppresses the navigation instead of re-navigating to the same path",
  ).toBe(historyDepthBefore);
});
