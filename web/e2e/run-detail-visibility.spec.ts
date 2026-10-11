// A backup started from an entry's page while the phone goes to the background
// and comes back. The watch that follows the run has to
//
//   1. ask /api/runs on its two-second beat while the page is visible,
//   2. stop asking while the page is hidden, over several missed beats,
//   3. ask at once on return, before any timer, and take a run that finished
//      meanwhile from that answer and not from the client's clock.
//
// The harness has no Docker, so no container exists and no run can start.
// The entry and the runs are staged at the route layer; the SPA, its
// requests and the binary are real. The page reads the entry's own runs with
// an item in the query, so a request without one is the watch's.
import { expect, test, type Page } from "@playwright/test";
import { MOBILE_PROJECTS, container, item, stageEntries } from "./entries";

const RUN_ID = "0f1e2d3c4b5a69788796a5b4c3d2e1f0";
const STARTED_AT = Math.floor(Date.now() / 1000) - 60;

const RUNNING_RUN = {
  id: RUN_ID,
  targetId: "plex",
  kind: "backup",
  status: "running",
  startedAt: STARTED_AT,
  finishedAt: null,
  snapshotId: "",
  bytes: 0,
  error: "",
  acknowledged: false,
  target: "plex",
  domain: "container",
};

const DONE_RUN = {
  ...RUNNING_RUN,
  status: "success",
  finishedAt: STARTED_AT + 45,
  snapshotId: RUN_ID,
  bytes: 5_000_000_000,
};

// The staged list of runs as the story advances: none before the start, the
// running one while the watch follows it, the finished one once the page is
// hidden.
let runsPhase: "empty" | "running" | "done" = "empty";

function runsBody() {
  if (runsPhase === "empty") return [];
  return [runsPhase === "running" ? RUNNING_RUN : DONE_RUN];
}

// A headless page never goes to the background, so the state a tab switch
// would give the app is put in its place and announced.
async function setPageVisibility(page: Page, state: "visible" | "hidden") {
  await page.evaluate((s) => {
    Object.defineProperty(document, "visibilityState", { configurable: true, get: () => s });
    document.dispatchEvent(new Event("visibilitychange"));
  }, state);
}

test("a hidden page stops asking for the run, and the return asks first and takes the finished run", async ({
  page,
}, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "the story of a phone that goes to the background");
  runsPhase = "empty";

  // A browser that has answered the stop warning before, so the press starts the backup.
  await page.addInitScript(() => localStorage.setItem("bv-container-stop-ack", "1"));

  await stageEntries(page, {
    items: [item({ key: "plex", lastBackup: 0, lastRunStatus: "" })],
    containers: [container({ name: "plex", lastBackup: null, lastBackupStarted: null })],
  });
  let asked = 0;
  let lastAskedAt = 0;
  await page.route(
    (url) => url.pathname === "/api/runs",
    async (route) => {
      if (new URL(route.request().url()).searchParams.has("itemKind")) {
        return route.fulfill({ json: { ok: true, runs: [] } });
      }
      asked += 1;
      lastAskedAt = Date.now();
      await route.fulfill({ json: { ok: true, runs: runsBody() } });
    },
  );
  await page.route("**/api/containers/plex/backup", (route) => route.fulfill({ json: { ok: true, started: true } }));

  await page.goto("/backups/container/plex");
  const backup = page.getByTestId("page-actions").getByRole("button", { name: "Back up now" });
  await expect(backup).toBeVisible();

  // The watch notes the newest run before it starts the backup.
  const started = page.waitForRequest("**/api/containers/plex/backup");
  await backup.click();
  await started;
  runsPhase = "running";

  await expect.poll(() => asked, { timeout: 15_000, message: "the watch never asked while visible" }).toBeGreaterThanOrEqual(3);

  // One request may already be on its way when the page hides. After it,
  // more than two beats pass without another.
  await setPageVisibility(page, "hidden");
  await page.waitForTimeout(2_600);
  const whileHidden = asked;
  await page.waitForTimeout(4_600);
  expect(asked, "the watch kept asking while the page was hidden").toBe(whileHidden);

  runsPhase = "done";
  const shownAt = Date.now();
  await setPageVisibility(page, "visible");
  await expect.poll(() => asked, { timeout: 5_000, message: "no request on return" }).toBeGreaterThan(whileHidden);
  expect(lastAskedAt - shownAt, "the return waited for its timer before it asked").toBeLessThan(1_500);

  // The toast carries the snapshot of the recorded run.
  await expect(page.getByText("Done · 0f1e2d3c")).toBeVisible();
});
