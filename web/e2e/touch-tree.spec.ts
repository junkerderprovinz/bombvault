// ---------------------------------------------------------------------------
// Touch-tree e2e — the geometry gate for the touch interaction mode.
//
// The dom twins (SelectionTree.touch.dom.test.tsx) prove SEMANTICS in jsdom;
// this spec proves GEOMETRY on the real binary, in real device emulation:
// a tap target's 44px promise is a fact about laid-out pixels, which jsdom
// cannot witness (no layout), and about the (pointer: coarse) media query,
// which jsdom stubs away. So this file runs ONLY on the two mobile projects
// (hasTouch → coarse pointer → interactionMode="touch" by construction —
// the mode follows the pointer axis, never the width), and asserts
// with locator.tap() + boundingBox():
//
//   1. every tree row is a >=44px-tall full-row target,
//   2. the chevron is a real >=44x44 button (the desktop tree renders an
//      aria-hidden 10px glyph instead — its presence here is ALSO the
//      touch-mode tell),
//   3. the chevron zone and the label zone are DISJOINT hit areas (expand
//      and check never share a tap surface),
//   4. tap toggles aria-checked in BOTH directions, and
//   5. the chevron expands WITHOUT moving the check.
//
// Harness honesty — what is mocked and why: the e2e
// webServer is the real bombvault binary over a wiped fresh DB, but the
// harness has no Docker, so a fresh DB can never contain a container and the
// folders editor could never render. The CONTAINER DOMAIN is therefore
// fulfilled at the Playwright route layer — the SPA, its fetches, the binary
// and every route shape are real; only the container payloads are staged,
// mirroring the Go JSON shapes field-for-field (api.ts). Everything else
// (settings, SSE progress, the save PATCH round-trip) rides the real server;
// the PATCH is fulfilled too so the harness DB stays untouched by a spec.
// ---------------------------------------------------------------------------
import { expect, test, type Page } from "@playwright/test";

// The two device projects from playwright.config.ts. Everything else is a
// desktop project.
const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// --- staged container domain (Go JSON shapes, api.ts) ----------------------

const PLEX_SOURCE = "/mnt/user/appdata/plex";

const CONTAINERS_BODY = {
  ok: true,
  containers: [
    {
      name: "plex",
      image: "lscr.io/linuxserver/plex:latest",
      state: "running",
      status: "Up 2 hours",
      ip: "172.18.0.5",
      installed: true, // the folders editor is advanced && installed — this flag opens it
      includeInSchedule: true,
      lastBackup: null,
      lastBackupStarted: null,
      preHook: "",
      postHook: "",
      stopContainers: [],
      excludes: [],
      lastUpdateCheck: 0,
      lastUpdateResult: "",
      stack: "",
    },
  ],
};

const MOUNTS_BODY = {
  ok: true,
  mounts: [
    {
      source: PLEX_SOURCE,
      dest: "/config",
      selected: true, // the served baseline starts CHECKED, so the taps below
      // exercise true → false and then false → true — both directions of the
      // toggle contract in one session.
      isAppdata: false,
      reachable: true,
    },
    {
      // A SECOND served include, and not fixture decoration: FoldersEditor's
      // empty-selection guard refuses (row shake + blocked warn line, no save)
      // any toggle that would leave ZERO includes for the item — with a single
      // selected mount the first tap trips exactly that contract instead of
      // exercising it. Two served includes keep every tap a legal toggle.
      source: "/mnt/user/data",
      dest: "/data",
      selected: true,
      isAppdata: false,
      reachable: true,
    },
  ],
  custom: [],
  excluded: [],
  excludeCaches: {},
  hostMountRoot: "/mnt/user",
  hostSourceRoot: "/mnt",
};

const BROWSE_BODY = {
  ok: true,
  status: "ok",
  truncated: false,
  dirs: [{ name: "library", path: "user/appdata/plex/library" }],
};

/** Route-level staging of the container domain + the boot-look cut
 *  (the display-prefs abort — mobile-shell.spec.ts's bootWithoutServerLook —
 *  so this spec keeps the harness default English labels regardless of worker
 *  order). */
async function stageContainerDomain(page: Page): Promise<void> {
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.route("**/api/containers", (route) => route.fulfill({ json: CONTAINERS_BODY }));
  await page.route("**/api/containers/plex/mounts*", (route) => route.fulfill({ json: MOUNTS_BODY }));
  await page.route("**/api/browse*", (route) => route.fulfill({ json: BROWSE_BODY }));
  // The selection save (FoldersEditor's drain PATCHes /api/containers/plex).
  // Fulfilled so the interaction is fully offline and the wiped harness DB
  // never sees a spec-driven write.
  await page.route("**/api/containers/plex", (route) => route.fulfill({ json: { ok: true } }));
}

/** Boot the /containers page in advanced mode and open the folders editor.
 *  Advanced mode is seeded as a boot-time localStorage write (the SAME key
 *  the Advanced toggle owns, "bombvault.advanced" = "1"): the detail's
 *  folders editor is advanced+installed-only, and the display-prefs abort
 *  above means the server can never adopt or overwrite this look mid-run.
 *
 *  The phone surface is ONE stacked layout: there is no list-level "Backup
 *  folders" chip — the editor renders INSIDE the card's stacked detail. So
 *  the entry is: tap the plex card, and the Back row confirms the detail
 *  stacked. The geometry under test below is the SAME touch tree; only the
 *  door moved. */
async function openFoldersEditor(page: Page): Promise<void> {
  await page.addInitScript(() => localStorage.setItem("bombvault.advanced", "1"));
  await page.goto("/containers");
  await page.getByRole("button", { name: /^plex/ }).tap();
  await expect(page.getByRole("button", { name: "plex, Back" })).toBeVisible();
}

/** The ONE tree under test, addressed by ROLE (survives page restructures;
 *  there is exactly one tree on this page). */
function backupTree(page: Page) {
  return page.getByRole("tree", { name: "Backup folder selection" });
}

test("touch rows are full-size targets and the chevron zone is disjoint from the label", async ({ page }, testInfo) => {
  test.skip(
    !MOBILE_PROJECTS.has(testInfo.project.name),
    "touch-only: tap() needs hasTouch and the touch tree needs (pointer: coarse)",
  );
  await stageContainerDomain(page);
  await openFoldersEditor(page);

  const tree = backupTree(page);
  await expect(tree).toBeVisible();
  // aria-level="1" scoping, not name-only: once Expand renders the
  // lazy-browse child, the child treeitem's accessible name contains the
  // same path text, so a name-only locator re-resolves to TWO elements and
  // every later expect(row) is a strict-mode violation. The row under test
  // is always the top-level item.
  const row = tree.locator('[aria-level="1"]').filter({ hasText: /appdata\/plex/ });
  await expect(row).toBeVisible();

  // (1) Full-row target: the row's laid-out box is at least 44px tall
  // (min-h-[2.75rem] made real). Visible-first guards the null box.
  const rowBox = await row.boundingBox();
  expect(rowBox, "the tree row must be laid out to measure it").not.toBeNull();
  expect(rowBox!.height).toBeGreaterThanOrEqual(44);

  // (2) The chevron is a real >=44x44 button. Its EXISTENCE is already the
  // touch-mode tell — the desktop tree renders an aria-hidden 10px glyph.
  // Name suffix-matched: the row's host path is prefixed to the action word,
  // so the name is "{path} Expand".
  const chevron = row.getByRole("button", { name: /Expand$/ });
  await expect(chevron).toBeVisible();
  const chevronBox = await chevron.boundingBox();
  expect(chevronBox, "the chevron must be laid out to measure it").not.toBeNull();
  expect(chevronBox!.width).toBeGreaterThanOrEqual(44);
  expect(chevronBox!.height).toBeGreaterThanOrEqual(44);

  // (3) Dedicated zones: the chevron's hit area and the label's hit area are
  // horizontally disjoint — no shared tap surface between expand and check.
  // Disjoint x-intervals imply disjoint rectangles; 1px of slack
  // absorbs subpixel rounding at fractional device scales.
  const labelBox = await row.locator("span.font-mono").first().boundingBox();
  expect(labelBox, "the row label must be laid out to measure it").not.toBeNull();
  expect(
    chevronBox!.x,
    "the chevron zone must start at/after the label's right edge — expand and toggle never share a hit area",
  ).toBeGreaterThanOrEqual(labelBox!.x + labelBox!.width - 1);
});

test("tap toggles the check both ways; the chevron expands without ever toggling", async ({ page }, testInfo) => {
  test.skip(
    !MOBILE_PROJECTS.has(testInfo.project.name),
    "touch-only: tap() needs hasTouch and the touch tree needs (pointer: coarse)",
  );
  await stageContainerDomain(page);
  await openFoldersEditor(page);

  const tree = backupTree(page);
  await expect(tree).toBeVisible();
  // aria-level="1" scoping, not name-only: once Expand renders the
  // lazy-browse child, the child treeitem's accessible name contains the
  // same path text, so a name-only locator re-resolves to TWO elements and
  // every later expect(row) is a strict-mode violation. The row under test
  // is always the top-level item.
  const row = tree.locator('[aria-level="1"]').filter({ hasText: /appdata\/plex/ });
  await expect(row).toBeVisible();

  // The served baseline is checked (mounts fixture: selected: true).
  await expect(row).toHaveAttribute("aria-checked", "true");

  // (4a) Tap toggles OFF — the full row is the toggle target. The optimistic
  // flip is synchronous; the "Saved" toast is the drain's OWN completion
  // signal (pushed only after the PATCH resolved ok), so waiting for it here
  // guarantees the row has left its busy gate before the next tap — a tap
  // landing mid-save would hit the SAME guarded handler Space uses and do
  // nothing (correct product behavior, flaky test otherwise).
  await row.tap();
  await expect(row).toHaveAttribute("aria-checked", "false");
  await expect(page.getByText("Saved")).toBeVisible();

  // (4b) Tap toggles back ON: two taps round-trip the check, never a
  // zero-change double-fire.
  await row.tap();
  await expect(row).toHaveAttribute("aria-checked", "true");

  // (5) The chevron EXPANDS without touching the check: the dedicated
  // expand zone runs the lazy browse, the child appears…
  await row.getByRole("button", { name: /Expand$/ }).tap();
  await expect(row).toHaveAttribute("aria-expanded", "true");
  await expect(tree.getByRole("treeitem", { name: "library" })).toBeVisible();
  // …the label followed the state…
  await expect(row.getByRole("button", { name: /Collapse$/ })).toBeVisible();
  // …and the check never moved during expansion.
  await expect(row).toHaveAttribute("aria-checked", "true");
});
