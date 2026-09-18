// ---------------------------------------------------------------------------
// Desktop-untouched — the per-page desktop invariance contract.
//
// The headline guarantee, made executable: for EVERY one of the ten routed
// destinations, on BOTH desktop projects (desktop-768 = the 48rem breakpoint
// boundary, desktop-1280 = comfortable desktop), today's shell is exactly
// what renders — the desktop Sidebar is visible, the mobile bottom bar has
// ZERO matches in the DOM (not CSS-hidden: the Layout chrome switch never
// renders it), and the `bv-main` scroller is present.
//
// Parameterized as a 10-route loop rather than spot checks: a future route
// added to the frozen router without joining this loop is a visible gap in
// the report, and any desktop-layout regression on any single destination
// fails the gate by name.
//
// The assertions deliberately run on the fresh-DB empty states the harness
// boots (playwright.config.ts): this is a chrome/layout contract, never a
// data contract. Gated destinations (/vms, /flash, ...) render their pages'
// empty states — the route tree is not gated, only the nav entries are, so
// every destination is directly reachable by goto (the Go SPA fallback,
// internal/api/spa.go, serves index.html for client routes).
//
// The second, per-page half: /dashboard additionally asserts the ABSENCE of
// every phone-only surface the Dashboard PR put below the breakpoint — the
// StickyActionBar chrome and the filled full-width New-backup trigger. Same
// discipline as the 10-route loop: fresh-DB empty states, chrome not data.
// The /dashboard route (not the / redirect) keeps the loop's canonical form.
//
// SCOPE NOTE: on the fork branch this file also carries per-page leakage
// batteries for the Containers, Files, Settings-family and Recovery mobile
// surfaces (their Fab/ListToolbar/Load-more/chip-strip/wizard needles and
// the inverse mobile-direction halves). Those pages' mobile treatments are
// not part of this PR, so their asserts travel with those pages' PRs —
// asserting against surfaces that do not exist in this tree would guard
// nothing on the desktop side and fail outright on the mobile side.
// ---------------------------------------------------------------------------
import { expect, test } from "@playwright/test";

// The two desktop projects from playwright.config.ts (>= the 48rem chrome
// switch). Branching on the project name — the mobile-shell.spec.ts pattern —
// keeps every assertion honest about WHICH contract each project verifies.
const DESKTOP_PROJECTS = new Set(["desktop-1280", "desktop-768"]);

// The ten canonical route paths, verbatim from the frozen route table
// (web/src/app/router.tsx). The redirect routes (/, /jobs) and the unlisted
// /glyphs contact sheet are not destinations and are deliberately absent.
const ROUTES = [
  "/dashboard",
  "/recovery",
  "/containers",
  "/vms",
  "/flash",
  "/files",
  "/config",
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

// ---------------------------------------------------------------------------
// The >=48rem leakage pass for /dashboard: the one page this PR gives a
// phone face asserts that NOTHING its phone half adds exists in the desktop
// DOM. Role/text-based wherever possible so a restyle of the mobile chrome
// cannot silently outdate the guard; the one class-signature check is the
// StickyActionBar's exact chrome combination, which no desktop element
// carries (verified app-wide when this pass landed — the only
// `sticky bottom-0 z-10 bg-carbon-sidebar` element in src/ is the bar).
// ---------------------------------------------------------------------------
const DASHBOARD_ROUTES = ["/dashboard"];

for (const route of DASHBOARD_ROUTES) {
  test(`desktop untouched of the Dashboard phone chrome at ${route}`, async ({ page }, testInfo) => {
    test.skip(!DESKTOP_PROJECTS.has(testInfo.project.name), "desktop-only: the >=48rem leakage contract");
    await page.goto(route);

    // The sticky-in-flow action bar (the thumb-zone trigger's host): its
    // exact class signature exists nowhere on desktop.
    await expect(page.locator("div.sticky.bottom-0.z-10.bg-carbon-sidebar")).toHaveCount(0);
    // The filled full-width accent control — the New-backup trigger's
    // signature (Button tone="accent" over the caller's w-full stage); no
    // desktop control carries it.
    await expect(page.locator("button.w-full.bg-accent")).toHaveCount(0);
    // The Home thumb-zone trigger.
    await expect(page.getByRole("button", { name: "New backup" })).toHaveCount(0);
  });
}

// Positive half for the Dashboard page: the desktop customizable grid is not
// merely free of mobile chrome — it is present. The customize pencil is
// desktop-only chrome IN SERVICE of that grid (it mounts only at md+), so it
// being visible proves the desktop page rendered rather than the phone's
// fixed Home block order (where the pencil has nothing to edit and is gated
// away entirely).
test("desktop dashboard keeps the customizable grid chrome", async ({ page }, testInfo) => {
  test.skip(!DESKTOP_PROJECTS.has(testInfo.project.name), "desktop-only: the desktop-invariance contract");
  await page.goto("/dashboard");
  await expect(page.getByRole("heading", { level: 1, name: "Dashboard" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Customize" })).toBeVisible();
});
