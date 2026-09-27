// ---------------------------------------------------------------------------
// Run-detail visibility e2e — the background/return gate.
//
// What is proven HERE, on the real binary in real device emulation: the
// backupWatch half of the gate. The "Back up now" flow on the REAL Containers
// page (the real BackupButton -> useBackupWatch) runs its poll chain against
// the real browser's Page Visibility state, and this spec pins the contract:
//
//   1. while visible, the chain polls /api/runs on its 2s cadence,
//   2. while the page is HIDDEN, the chain stops scheduling — zero requests
//      over multiple missed intervals,
//   3. on RETURN, the watch's FIRST act is the listRuns refetch (the
//      baseline-id reconcile — "refetch FIRST, then resume"), and
//      a run that finished while hidden resolves from that refetched server
//      record (the success toast with the recorded snapshot id), never from
//      a client clock.
//
// Harness honesty — what is mocked and why: the e2e
// webServer is the real bombvault binary over a wiped fresh DB, but the
// harness has no Docker, so a fresh DB can never contain a container and no
// real run can ever fire. The CONTAINER DOMAIN and the RUNS LIST are
// therefore fulfilled at the Playwright route layer — the SPA, its fetches,
// the binary, the SSE channel and every route shape are real; only the
// staged payloads are fake, mirroring the Go JSON shapes field-for-field
// (api.ts). /api/runs is requested ONLY by the watch chain on this page (the
// dashboard's ActivityLog is not mounted here), so the request count IS the
// poll chain's heartbeat.
//
// SCOPE NOTE: this branch hosts RunDetailSheet component-locally in the
// Containers page's stacked detail (no route — router.tsx is frozen), so the
// live-section half of the gate is reachable from this page; it is pinned at
// the component level by RunDetailSheet.dom.test.tsx (real progress.ts
// singleton across a real EventSource boundary, real visibilitychange). The
// sheet's own background/return e2e travels with the PR that ports the
// sheet's own flows; the machinery proven here (this hook +
// lib/useVisibilityGate.ts) is the same machinery that sheet consumer uses.
// ---------------------------------------------------------------------------
import { expect, test, type Page } from "@playwright/test";

// The two device projects from playwright.config.ts. Everything else is a
// desktop project.
const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// --- staged payloads (Go JSON shapes, api.ts) ------------------------------

const CONTAINERS_BODY = {
  ok: true,
  containers: [
    {
      name: "plex",
      image: "lscr.io/linuxserver/plex:latest",
      state: "running",
      status: "Up 2 hours",
      ip: "172.18.0.5",
      installed: true,
      includeInSchedule: true,
      lastBackup: null,
      lastBackupStarted: null,
      preHook: "",
      postHook: "",
      stopContainers: [],
      excludes: [],
    },
  ],
};

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

// Flips the staged runs list between its three states as the story advances:
// [] is the pre-fire baseline, the running run is the in-flight watch, the
// done run is "finished while the page was hidden".
let runsPhase: "empty" | "running" | "done" = "empty";

function runsBody() {
  if (runsPhase === "empty") return [];
  return [runsPhase === "running" ? RUNNING_RUN : DONE_RUN];
}

// Shadow document.visibilityState and announce it — the same drive a real
// tab switch gives the app (headless pages never actually background).
async function setPageVisibility(page: Page, state: "visible" | "hidden") {
  await page.evaluate((s) => {
    Object.defineProperty(document, "visibilityState", { configurable: true, get: () => s });
    document.dispatchEvent(new Event("visibilitychange"));
  }, state);
}

// ---------------------------------------------------------------------------

test("hidden page stops the run poll; return refetches first and reconciles the finished run", async ({
  page,
}, testInfo) => {
  test.skip(
    !MOBILE_PROJECTS.has(testInfo.project.name),
    "mobile-only: the gate targets the phone background/return story"
  );

  // #197's one-time stop-warning ack — a real second-time user's browser
  // state, so the flow under test is fire() and not the confirm sheet.
  await page.addInitScript(() => {
    try {
      localStorage.setItem("bv-container-stop-ack", "1");
    } catch {
      /* private-mode browsers re-ask; not this test's subject */
    }
  });

  // The watch chain's heartbeat: /api/runs is requested ONLY by backupWatch
  // on this page, so the Node-side counter IS the chain's schedule.
  let runsRequests = 0;
  let lastRunsRequestAt = 0;
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.route("**/api/runs", async (route) => {
    runsRequests += 1;
    lastRunsRequestAt = Date.now();
    await route.fulfill({ json: { ok: true, runs: runsBody() } });
  });
  await page.route("**/api/containers", (route) => route.fulfill({ json: CONTAINERS_BODY }));
  await page.route("**/api/containers/plex/backup", (route) =>
    route.fulfill({ json: { ok: true, started: true } })
  );

  await page.goto("/containers");
  // The phone surface is ONE stacked layout: the row's BackupButton lives
  // inside the card's stacked detail (tap the card, the Back row confirms
  // the stack), so open the detail before firing. The watch chain under test
  // is indifferent to where its trigger mounts; nothing below changes.
  await page.getByRole("button", { name: /^plex/ }).tap();
  const backup = page.getByRole("button", { name: "Back up now" });
  await expect(backup).toBeVisible();

  // fire(): baseline GET (the pre-fire id snapshot) lands before the POST.
  const fired = page.waitForRequest("**/api/containers/plex/backup");
  await backup.click();
  await fired;
  runsPhase = "running";

  // 1. Visible: the chain is alive — baseline + at least two poll hops at
  //    the 2s cadence (first hop fires ~600ms after the POST).
  await expect
    .poll(() => runsRequests, { timeout: 15_000, message: "poll chain never ran while visible" })
    .toBeGreaterThanOrEqual(3);

  // 2. Hidden: the chain stops scheduling. One hop may already have been
  //    scheduled when the page hid — let it land, then demand silence across
  //    more than two missed intervals.
  await setPageVisibility(page, "hidden");
  await page.waitForTimeout(2_600);
  const hiddenCount = runsRequests;
  await page.waitForTimeout(4_600);
  expect(
    runsRequests,
    "the poll chain kept firing while the page was hidden (visibility gate missing)"
  ).toBe(hiddenCount);

  // 3. Return: the watch refetches FIRST — the immediate listRuns call is
  //    the reconcile (baseline-id match), not a resumed timer waiting out
  //    its interval. The staged run finished while hidden; that refetch must
  //    resolve the watch from the server record.
  runsPhase = "done";
  const shownAt = Date.now();
  await setPageVisibility(page, "visible");
  await expect
    .poll(() => runsRequests, { timeout: 5_000, message: "no immediate refetch on return" })
    .toBeGreaterThan(hiddenCount);
  expect(
    lastRunsRequestAt - shownAt,
    "the return refetch was not immediate (resumed the timer instead of reconciling first)"
  ).toBeLessThan(1_500);

  // The reconciled outcome, through the real watch -> BackupButton surface:
  // the success toast carries the recorded snapshot id's mono slice — read
  // from the refetched run, never extrapolated.
  await expect(page.getByText("Done · 0f1e2d3c")).toBeVisible();
});
