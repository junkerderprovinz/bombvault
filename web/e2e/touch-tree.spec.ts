// The folder tree of a container under a finger. Its tests in jsdom prove
// what a tap means. This proves the geometry, which needs laid-out pixels and
// a real (pointer: coarse) answer, so it runs on the two phone projects:
//
//   1. every row of the tree is a target at least 44px tall,
//   2. the chevron is a button of at least 44 by 44px,
//   3. the chevron and the label do not share a tap surface,
//   4. a tap checks and unchecks, and
//   5. the chevron opens a folder without moving the check.
//
// The harness has no Docker, so the container, its folders and the listing
// of a folder are staged at the route layer, and so is the answer to the
// save, which keeps the harness database as it was.
import { expect, test, type Page } from "@playwright/test";
import { MOBILE_PROJECTS, container, item, stageEntries } from "./entries";

const PLEX_SOURCE = "/mnt/user/appdata/plex";

const MOUNTS_BODY = {
  ok: true,
  mounts: [
    // Checked as served, so the taps go from checked to unchecked and back.
    { source: PLEX_SOURCE, dest: "/config", selected: true, isAppdata: false, reachable: true },
    // The editor refuses a toggle that would leave nothing selected, so a
    // second folder keeps every tap on the first a legal one.
    { source: "/mnt/user/data", dest: "/data", selected: true, isAppdata: false, reachable: true },
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

/** Opens the Settings tab of the container's page, where its folders stand open. */
async function openFoldersEditor(page: Page): Promise<void> {
  await stageEntries(page, {
    items: [item({ key: "plex", lastBackup: 0, lastRunStatus: "" })],
    containers: [container({ name: "plex", lastBackup: null, lastBackupStarted: null })],
  });
  await page.route("**/api/containers/plex/mounts*", (route) => route.fulfill({ json: MOUNTS_BODY }));
  await page.route("**/api/browse*", (route) => route.fulfill({ json: BROWSE_BODY }));
  await page.route("**/api/containers/plex", (route) => route.fulfill({ json: { ok: true } }));
  await page.goto("/backups/container/plex?tab=settings");
  await expect(backupTree(page)).toBeVisible();
}

function backupTree(page: Page) {
  return page.getByRole("tree", { name: "Backup folder selection" });
}

test("touch rows are full-size targets and the chevron zone is disjoint from the label", async ({ page }, testInfo) => {
  test.skip(
    !MOBILE_PROJECTS.has(testInfo.project.name),
    "touch-only: tap() needs hasTouch and the touch tree needs (pointer: coarse)",
  );
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

  // (2) The chevron is a real >=44x44 button. Its existence is already the
  // touch-mode tell: the desktop tree renders an aria-hidden 10px glyph.
  // Name suffix-matched: the row's host path is prefixed to the action word,
  // so the name is "{path} Expand".
  const chevron = row.getByRole("button", { name: /Expand$/ });
  await expect(chevron).toBeVisible();
  const chevronBox = await chevron.boundingBox();
  expect(chevronBox, "the chevron must be laid out to measure it").not.toBeNull();
  expect(chevronBox!.width).toBeGreaterThanOrEqual(44);
  expect(chevronBox!.height).toBeGreaterThanOrEqual(44);

  // (3) Dedicated zones: the chevron's hit area and the label's hit area are
  // horizontally disjoint; no shared tap surface between expand and check.
  // Disjoint x-intervals imply disjoint rectangles; 1px of slack
  // absorbs subpixel rounding at fractional device scales.
  const labelBox = await row.locator("span.font-mono").first().boundingBox();
  expect(labelBox, "the row label must be laid out to measure it").not.toBeNull();
  expect(
    chevronBox!.x,
    "the chevron zone must start at/after the label's right edge: expand and toggle never share a hit area",
  ).toBeGreaterThanOrEqual(labelBox!.x + labelBox!.width - 1);
});

test("tap toggles the check both ways; the chevron expands without ever toggling", async ({ page }, testInfo) => {
  test.skip(
    !MOBILE_PROJECTS.has(testInfo.project.name),
    "touch-only: tap() needs hasTouch and the touch tree needs (pointer: coarse)",
  );
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

  // (4a) Tap toggles off: the full row is the toggle target. The optimistic
  // flip is synchronous; the "Saved" toast is the drain's own completion
  // signal (pushed only after the PATCH resolved ok), so waiting for it here
  // guarantees the row has left its busy gate before the next tap: a tap
  // landing mid-save would hit the same guarded handler Space uses and do
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
