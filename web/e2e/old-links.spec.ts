// Links to the pages each kind had of its own. Bookmarks, release notes and
// support answers carry /containers, /vms, /files, /flash, /zfs and /config,
// and a finding's restore link used to name its entry in ?item=. Each has to
// land on the list of its kind or on the entry it names, through the real
// router and the server's fallback for client routes.
import { expect, test } from "@playwright/test";
import { EVERY_KIND, MOBILE_PROJECTS, SET_ID, ZFS_ID, stageEntries } from "./entries";

// A handler can still wait on the real server when its test has passed;
// what it throws then is dropped.
test.afterEach(async ({ page }) => {
  await page.unrouteAll({ behavior: "ignoreErrors" });
});

const immich = EVERY_KIND.runs!["container:immich"];

for (const [link, kind, tile] of [
  ["/containers", "container", /^Containers/],
  ["/vms", "vm", /^VMs/],
  ["/files", "files", /^Folders/],
  ["/zfs", "zfs", /^ZFS/],
] as const) {
  test(`${link} opens the Backups list on its kind`, async ({ page }) => {
    await stageEntries(page, EVERY_KIND);
    await page.goto(link);
    await expect(page).toHaveURL(new RegExp(`/backups\\?kind=${kind}$`));
    await expect(page.getByRole("heading", { level: 1, name: "Backups" })).toBeAttached();
    await expect(page.getByRole("group", { name: "What the list shows" }).getByRole("button", { name: tile })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    await expect(page.getByTestId("backup-row")).toHaveCount(1);
  });
}

for (const [link, target, name] of [
  ["/flash", "/backups/flash/flash", "Unraid flash"],
  ["/config", "/backups/config/config", "BombVault settings"],
] as const) {
  test(`${link} opens the one entry of its kind`, async ({ page }) => {
    await stageEntries(page, EVERY_KIND);
    await page.goto(link);
    await expect(page).toHaveURL(new RegExp(`${target}$`));
    await expect(page.getByRole("heading", { level: 1, name })).toBeAttached();
  });
}

test("a restore link to a container opens its page on the backup it names", async ({ page }) => {
  await stageEntries(page, EVERY_KIND);
  const wanted = immich[1];
  await page.goto(`/containers?restore=${wanted.snapshotId}&at=${wanted.finishedAt}&item=immich`);
  await expect(page).toHaveURL(
    new RegExp(`/backups/container/immich\\?restore=${wanted.snapshotId}&at=${wanted.finishedAt}$`),
  );
  await expect(page.getByRole("dialog", { name: "Restore immich" })).toBeVisible();
});

test("a link to a VM opens its page under the libvirt name", async ({ page }) => {
  await stageEntries(page, EVERY_KIND);
  await page.goto("/vms?item=win11");
  await expect(page).toHaveURL(/\/backups\/vm\/win11$/);
  await expect(page.getByRole("heading", { level: 1, name: "Windows 11" })).toBeAttached();
});

test("a link that names a folder set finds the page of its id", async ({ page }) => {
  await stageEntries(page, EVERY_KIND);
  await page.goto("/files?item=Fotos");
  await expect(page).toHaveURL(new RegExp(`/backups/files/${SET_ID}$`));
  await expect(page.getByRole("heading", { level: 1, name: "Fotos" })).toBeAttached();
});

test("a link that names a ZFS item by its dataset finds the page of its id", async ({ page }) => {
  await stageEntries(page, EVERY_KIND);
  await page.goto(`/zfs?item=${encodeURIComponent("tank/appdata")}`);
  await expect(page).toHaveURL(new RegExp(`/backups/zfs/${ZFS_ID}$`));
  await expect(page.getByRole("heading", { level: 1, name: "tank/appdata" })).toBeAttached();
});

test("a link to a folder set that is gone opens the list of folders", async ({ page }) => {
  await stageEntries(page, EVERY_KIND);
  await page.goto("/files?restore=abc123&at=1700000000&item=Filme");
  await expect(page).toHaveURL(/\/backups\?kind=files$/);
  await expect(page.getByTestId("backup-row")).toHaveCount(1);
});

test("a link to a container that is gone says so and leads back to the list", async ({ page }) => {
  await stageEntries(page, EVERY_KIND);
  await page.goto("/containers?item=vanished");
  await expect(page).toHaveURL(/\/backups\/container\/vanished$/);
  await page.locator("#bv-main").getByRole("link", { name: "Backups" }).click();
  await expect(page).toHaveURL(/\/backups$/);
});

test("the old links work on a phone as well", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "the phone layout");
  await stageEntries(page, EVERY_KIND);
  await page.goto("/vms");
  await expect(page).toHaveURL(/\/backups\?kind=vm$/);
  await expect(page.getByTestId("backup-row")).toHaveCount(1);
  await expect(page.getByTestId("bottom-nav")).toBeVisible();
});
