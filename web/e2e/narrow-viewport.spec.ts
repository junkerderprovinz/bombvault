// ---------------------------------------------------------------------------
// Narrow-viewport backstop: the UI-considerations long-text row,
// operationalized.
//
// The one backstop state, made executable evidence: German and French chrome
// must fit 320-360px: the de compound words ("Einstellungen",
// "Wiederherstellung") and fr accented long labels ("Paramètres",
// "Récupération") are the stress cases. For every combination of {de, fr} x
// {320px, 360px}, every bottom-bar slot caption and every More-sheet row
// label stays on a single line box and the bar container shows no horizontal
// overflow: the min-w-0 + truncate CSS contract's visible outcome.
//
// Geometry, not screenshots (fast + non-flaky): each caption's height is
// measured against an explicitly normalized 20px line box, with the
// truncate contract intact (white-space: nowrap) a caption is one line box;
// a dropped contract lets a long label wrap into a second box (~40px),
// which is the exact failure mode the 1.5x bound cannot pass. Overflow is
// scrollWidth vs clientWidth on the bar itself. Both measurements tolerate
// subpixel rounding (+0.5/+1px) and nothing else.
//
// The locale is seeded the way a returning visitor's browser carries it,
// the persisted `bv-lang` localStorage key (web/src/lib/i18n.ts STORAGE_KEY;
// not exported, kept in sync by hand), via addInitScript before the first
// page script runs. The mobile chrome has no language control (the switcher
// lives in the desktop-only Sidebar controls), so there is no UI-driven
// path; seeding is the only honest way to boot the page in de/fr.
//
// The boot-time display-prefs reconciliation is cut (see bootSeededPage):
// the server is the truth for the look (#191), so a stored bv-lang on the
// harness DB would silently overwrite this page's seed, and with de/fr
// workers running in parallel, each page would see whichever locale booted
// first, not its own.
//
// SCOPE NOTE: this file sweeps the shell chrome (bar + More sheet), the
// Dashboard log block, the Settings tab strip and the landscape boundary
// pair. The other destination surfaces (the six route surfaces, the Files
// editor header, the populated guided-restore step 5) carry their narrow
// sweeps beside their own treatments, in the specs where the surfaces they
// guard actually exist.
// ---------------------------------------------------------------------------
import { expect, test, type Page } from "@playwright/test";

// The two device projects from playwright.config.ts, the backstop targets
// the mobile chrome, which only exists below the 48rem switch.
const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

// Persisted-locale localStorage key (i18n.ts STORAGE_KEY) + the localized
// "More" trigger label per locale. Waiting on the localized trigger before
// measuring proves the locale table is live (de ships inline; fr arrives via
// its async locale chunk): measuring before that would silently assert the
// English fallback instead of the stress-case strings.
const LOCALE_STORAGE_KEY = "bv-lang";
const MORE_LABEL = { de: "Mehr", fr: "Plus" } as const;
// The sheet's fresh-DB stress-case row: Settings, the one always-on More
// destination since Recovery moved into the bar (its equally long caption
// stays covered by the bar-caption sweep below).
const SETTINGS_LABEL = { de: "Einstellungen", fr: "Paramètres" } as const;

// Explicit single-line box used for the height arithmetic (see header).
const LINE_BOX = 20;

// Boot the page with this test's locale standing, immune to the server-side
// look and to the other workers: seed the persisted key before any page
// script runs (a returning visitor's localStorage), then abort the
// boot-time display-prefs reconciliation. sync()'s fetch failing is the
// app's own documented degradation path ("offline: the cache is the look",
// displayPrefs.ts / #191), so the seeded locale renders, and nothing is
// PUT back to the shared harness server, which is what makes parallel de/fr
// workers deterministic (a booted de page otherwise seeds bv-lang=de on the
// server and every later fr boot adopts it, observed live).
async function bootSeededPage(
  page: Page,
  locale: string,
  width: number,
  path = "/dashboard",
  // Landscape dimensions (the boundary tests) pass a real phone-landscape
  // height; the portrait sweep keeps the original 700.
  height = 700,
): Promise<void> {
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(
    ([key, value]) => {
      window.localStorage.setItem(key, value);
    },
    [LOCALE_STORAGE_KEY, locale] as const,
  );
  await page.setViewportSize({ width, height });
  await page.goto(path);
}

const locales = ["de", "fr"] as const;
const widths = [320, 360];

for (const locale of locales) {
  for (const width of widths) {
    test(`narrow viewport ${locale} @ ${width}px: bar and sheet labels single-line, bar never overflows`, async ({ page }, testInfo) => {
      test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the backstop targets the mobile chrome");
      await bootSeededPage(page, locale, width);

      // The locale table is live: the trigger carries its localized label.
      const bar = page.getByTestId("bottom-nav");
      const moreTrigger = bar.getByRole("button", { name: MORE_LABEL[locale] });
      await expect(moreTrigger).toBeVisible();

      // Single-line contract, bar captions: every slot caption is one
      // normalized line box (+subpixel), never two (~2x). Two full line
      // boxes need >= 2 * LINE_BOX, so the 1.5x bound is strict about the
      // two-line failure mode while tolerating subpixel rounding.
      const captionStats = await bar
        .locator("span.truncate")
        .evaluateAll(
          (els, box) =>
            els.map((el) => {
              el.style.lineHeight = `${box}px`;
              const height = el.getBoundingClientRect().height;
              el.style.lineHeight = "";
              return { text: (el.textContent ?? "").trim(), height };
            }),
          LINE_BOX,
        );
      expect(
        captionStats.length,
        "the bar must carry caption labels to assert against",
      ).toBeGreaterThan(0);
      for (const s of captionStats) {
        expect(s.height, `caption "${s.text}" must stay on one line`).toBeGreaterThan(0);
        expect(
          s.height,
          `caption "${s.text}" wrapped to a second line (height ${s.height}px vs one ${LINE_BOX}px line box)`,
        ).toBeLessThanOrEqual(LINE_BOX * 1.5 + 0.5);
      }

      // No-overflow contract: the bar never scrolls horizontally, with
      // min-w-0 + truncate the slots shrink/ellipsis instead of pushing the
      // bar wider than the viewport. (+1px subpixel tolerance.)
      const barOverflow = await bar.evaluate((el) => ({
        scrollWidth: el.scrollWidth,
        clientWidth: el.clientWidth,
      }));
      expect(
        barOverflow.scrollWidth,
        `the bar overflows horizontally at ${width}px (${barOverflow.scrollWidth} > ${barOverflow.clientWidth})`,
      ).toBeLessThanOrEqual(barOverflow.clientWidth + 1);

      // Same single-line contract inside the More sheet: open it and measure
      // every destination row's label span (the long de/fr Settings label is
      // the fresh-DB stress case; the sheet is where >1 row lands later).
      await moreTrigger.click();
      const sheet = page.getByTestId("more-sheet");
      await expect(sheet).toBeVisible();
      await expect(sheet.getByRole("link", { name: SETTINGS_LABEL[locale] })).toBeVisible();

      const rowStats = await sheet
        .getByRole("link")
        .locator("span.truncate")
        .evaluateAll(
          (els, box) =>
            els.map((el) => {
              el.style.lineHeight = `${box}px`;
              const height = el.getBoundingClientRect().height;
              el.style.lineHeight = "";
              return { text: (el.textContent ?? "").trim(), height };
            }),
          LINE_BOX,
        );
      expect(
        rowStats.length,
        "the sheet must carry row labels to assert against",
      ).toBeGreaterThan(0);
      for (const s of rowStats) {
        expect(s.height, `sheet row "${s.text}" must stay on one line`).toBeGreaterThan(0);
        expect(
          s.height,
          `sheet row "${s.text}" wrapped to a second line (height ${s.height}px)`,
        ).toBeLessThanOrEqual(LINE_BOX * 1.5 + 0.5);
      }
    });
  }
}

// ---------------------------------------------------------------------------
// Helpers shared by the geometry sweeps below, the measurement approach in
// full, so any sweep that later joins this file inherits the exact same
// contracts (and tolerances, and nothing looser).
//
// Three contracts, all geometry not screenshots:
//   1. the page never scrolls horizontally, checked on documentElement and
//      on #bv-main, because the app scrolls inside #bv-main and the
//      page-pan bug class is exactly that scroller growing a horizontal axis;
//   2. every rendered control sits inside the viewport, unless it lives in a
//      genuinely scrollable inline container (a chip strip: reachable by
//      scrolling it: the sanctioned case). The exemption requires
//      overflow-x: auto/scroll with the container actually overflowing:
//      #bv-main scrolls vertically, so a horizontal overflow there is the
//      bug, never a sanctioned scroller.
//
// Data honesty: the Dashboard sweep stages its read models at the route
// layer (Go JSON shapes field-for-field): the log block needs runs to render
// rows, impossible on a fresh DB.
// ---------------------------------------------------------------------------

/** Let layout settle before any geometry read, in three steps: web-font swap
 *  changes text metrics for a frame; the entrance/tab-slide animations
 *  translate their hosts (glim-tab-slide alone is an 18px X slide) so a
 *  mid-animation read invents phantom pans (observed live: /settings
 *  measured +2..+18px wide mid-slide, /flash +3 mid-fade); then two rAFs
 *  flush the post-animation layout. Infinite animations (busy spinners)
 *  are excluded and the wait is capped so a perpetually-moving page can
 *  never wedge the sweep. */
async function settle(page: Page): Promise<void> {
  await page.evaluate(() => {
    const finite = document
      .getAnimations()
      .filter((a) => a.effect?.getTiming().iterations !== Infinity);
    const drained = Promise.all(finite.map((a) => a.finished.catch(() => {})));
    const capped = new Promise((resolve) => setTimeout(resolve, 750));
    return Promise.race([drained, capped])
      .then(() => document.fonts.ready)
      .then(
        () =>
          new Promise<void>((resolve) =>
            requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
          ),
      );
  });
}

/** Contract 1: neither documentElement nor #bv-main grows a horizontal axis
 *  (+1px subpixel tolerance, nothing else). A failing #bv-main names the
 *  widest offenders so the fix knows where to look. */
async function assertNoHorizontalPan(page: Page, label: string): Promise<void> {
  await settle(page);
  const m = await page.evaluate(() => {
    const main = document.querySelector("#bv-main");
    const wide: string[] = [];
    if (main && main.scrollWidth > main.clientWidth + 1) {
      const limit = main.getBoundingClientRect().right;
      for (const el of Array.from(main.querySelectorAll("*"))) {
        const cs = window.getComputedStyle(el);
        if (cs.display === "none" || cs.visibility === "hidden") continue;
        const rect = el.getBoundingClientRect();
        if (rect.width === 0 || rect.right <= limit + 1) continue;
        const cls = typeof el.className === "string" ? el.className : "";
        wide.push(
          `${el.tagName.toLowerCase()}.${cls.split(" ").slice(0, 3).join(".")} "${(el.textContent ?? "").trim().slice(0, 30)}" right=${Math.round(rect.right)}`,
        );
        if (wide.length >= 5) break;
      }
    }
    return {
      doc: { sw: document.documentElement.scrollWidth, cw: window.innerWidth },
      main: main ? { sw: main.scrollWidth, cw: main.clientWidth } : null,
      wide,
    };
  });
  expect(m.doc.sw, `${label}: documentElement scrolls horizontally`).toBeLessThanOrEqual(m.doc.cw + 1);
  expect(
    m.main,
    `${label}: #bv-main is missing: the sweep must run against the real app shell`,
  ).not.toBeNull();
  expect(
    m.main!.sw,
    `${label}: #bv-main scrolls horizontally (the page-pan bug class); widest: ${m.wide.join(" | ")}`,
  ).toBeLessThanOrEqual(m.main!.cw + 1);
}

/** Contract 2: no control clips a viewport edge unless a real inline
 *  x-scroller (overflow-x auto/scroll and actually overflowing) contains it
 * : those are reachable by scrolling, the sanctioned chip-row case. There
 *  is no overflow-y condition: CSS resolves `overflow-y: visible` to `auto`
 *  whenever overflow-x is non-visible, so every sanctioned scroller computes
 *  overflow-y: auto too (observed live in this sweep's first run). The
 *  page-level pan is contract 1's job, so exempting scroller content here
 *  masks nothing. */
async function assertNothingClipped(page: Page, label: string): Promise<void> {
  await settle(page);
  const offenders = await page.evaluate(() => {
    const vw = window.innerWidth;
    const found: string[] = [];
    const controls = document.querySelectorAll("button, a, input, select, textarea, [role='switch']");
    for (const el of Array.from(controls)) {
      const cs = window.getComputedStyle(el);
      if (cs.display === "none" || cs.visibility === "hidden") continue;
      const rect = el.getBoundingClientRect();
      if (rect.width === 0 && rect.height === 0) continue;
      let inInlineScroller = false;
      for (let anc = el.parentElement; anc; anc = anc.parentElement) {
        const axs = window.getComputedStyle(anc).overflowX;
        if ((axs === "auto" || axs === "scroll") && anc.scrollWidth > anc.clientWidth + 1) {
          inInlineScroller = true;
          break;
        }
      }
      if (inInlineScroller) continue;
      if (rect.right > vw + 1 || rect.left < -1) {
        const name = (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40);
        found.push(
          `${el.tagName.toLowerCase()} "${name}" left=${Math.round(rect.left)} right=${Math.round(rect.right)} viewport=${vw}`,
        );
      }
    }
    return found;
  });
  expect(offenders, `${label}: controls clip the viewport edge`).toEqual([]);
}

// --- staged read models (Go JSON shapes, api.ts, field-for-field) ------------

/** The Dashboard read models for the log sweep: a couple of finished runs so
 *  the merged log renders rows, empty schedule/next so the idle line stays
 *  out. */
async function stageDashboardForSweep(page: Page): Promise<void> {
  const now = Math.floor(Date.now() / 1000);
  const run = (i: number) => ({
    id: String(i).padStart(2, "0").padEnd(32, "a"),
    targetId: String(i).padStart(2, "0").padEnd(32, "b"),
    kind: "backup",
    status: "success",
    startedAt: now - (i + 1) * 3600,
    finishedAt: now - (i + 1) * 3600 + 60,
    snapshotId: "f".repeat(32),
    bytes: 1_000_000,
    error: "",
    acknowledged: false,
    target: `svc-${String(i).padStart(2, "0")}`,
    domain: "container",
  });
  await page.route("**/api/status", (route) =>
    route.fulfill({
      json: {
        ok: true,
        domains: [
          {
            domain: "containers",
            enabled: true,
            schedule: "every day",
            coveredBy: "",
            lastSuccess: now - 3600,
            periodSeconds: 86400,
            status: "ok",
            lastVerified: 0,
            lastVerifiedOK: false,
            verifiedDetail: "",
            drillDetail: "",
            offsiteConfigured: false,
            offsiteImmutable: false,
            lastTamperAt: 0,
            lastTamperOK: false,
            lastReplicationAt: 0,
            lastReplicationOK: false,
            lastDrDrillAt: 0,
            lastDrDrillOK: false,
            lastOffsiteSubsetAt: 0,
            lastOffsiteSubsetOK: false,
            offsiteDrillScheduled: false,
            protection: "green",
            tamperState: "",
            replicationState: "",
            drillState: "",
            encryptionOn: true,
            pruneStrategySet: true,
          },
        ],
      },
    }),
  );
  await page.route("**/api/schedule/next", (route) => route.fulfill({ json: { ok: true, runs: [] } }));
  await page.route("**/api/runs", (route) => route.fulfill({ json: { ok: true, runs: [run(0), run(1)] } }));
  await page.route("**/api/stats*", (route) =>
    route.fulfill({
      json: {
        ok: true,
        latest: { domain: "containers", source: "local", at: now, rawSize: 1_050_000_000_000, restoreSize: 260_000_000_000, snapshots: 3 },
      },
    }),
  );
}

// The localized log-block title the sweep locates the card by
// (activityLog.title; the de table inline in i18n.ts, fr in locales/fr.ts).
const LOG_TITLE = { de: "Aktivitätsprotokoll", fr: "Journal d'activité" } as const;

for (const locale of locales) {
  for (const width of widths) {
    const tag = `${locale} @ ${width}px`;

    test(`narrow sweep ${tag}: the Dashboard log block never pans and clips nothing`, async ({ page }, testInfo) => {
      test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the sweep targets the mobile surfaces");
      await stageDashboardForSweep(page);
      await bootSeededPage(page, locale, width);

      const card = page
        .locator("div.glim-notch-card")
        .filter({ hasText: LOG_TITLE[locale] })
        .filter({ visible: true });
      await expect(card).toBeVisible();

      await assertNoHorizontalPan(page, `${tag} /dashboard log`);
      await assertNothingClipped(page, `${tag} /dashboard log`);
    });
  }
}

// ---------------------------------------------------------------------------
// The landscape boundary: one proof, two viewports, the seeded-locale boot
// reused with real landscape heights. The mobile-chrome query is width-only
// ("min-width: 48rem"): 740px stays mobile, 844x390,
// a modern phone in landscape, 844 CSS px wide, crosses 48rem and gets the
// desktop chrome. Both run on the mobile projects only: the projects just
// supply the base device context, and setViewportSize overrides it either
// way. The needles are the chrome itself (bottom nav vs desktop Sidebar),
// the surfaces every width owns, independent of any page's data.
// ---------------------------------------------------------------------------

test("landscape 740x360: below 48rem the mobile chrome owns the shell", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the boundary pair rides the mobile projects");
  await bootSeededPage(page, "en", 740, "/dashboard", 360);

  // Mobile chrome at a phone-landscape width: the bottom bar is mounted and
  // the desktop Sidebar is nowhere in the DOM.
  await expect(page.getByTestId("bottom-nav")).toBeVisible();
  await expect(page.getByTestId("desktop-sidebar")).toHaveCount(0);
  await expect(page.locator("#bv-main")).toHaveCount(1);
});

test("landscape 844x390: at >=48rem the desktop chrome owns the shell", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the boundary pair rides the mobile projects");
  await bootSeededPage(page, "en", 844, "/dashboard", 390);

  // 844 CSS px >= 768 (48rem): the desktop shell renders and no mobile
  // chrome exists: the width-only switch did its job at a landscape height.
  await expect(page.getByTestId("desktop-sidebar")).toBeVisible();
  await expect(page.getByTestId("bottom-nav")).toHaveCount(0);
  await expect(page.getByRole("heading", { level: 1, name: "Dashboard" })).toBeVisible();
});

// ---------------------------------------------------------------------------
// The Settings tab strip at phone widths.
// The strip pins every segment to the sidebar row-box width (--nav-row-w,
// 200px) on desktop, and before the phone arm of --settings-tab-seg-w that
// same pin on a 390px phone wrapped its seven flex-none segments into seven
// stacked rows, roughly 350px of chrome before any Settings content. The clamp
// fits one row from 360px up and bounds the wrap at two on the narrowest
// supported width, where its 2.5rem floor takes over. Geometry, not
// screenshots, per this file's contracts: how many rows the seven tabs
// occupy, and no segment shrunk below that floor (tap-target scale; the
// row height is --nav-row-h at every width). English on purpose: the clamp
// is a fixed per-segment width with truncating labels, so it behaves the
// same in every locale, and en keeps the assertion on that mechanism rather
// than on a locale's typography.
// ---------------------------------------------------------------------------

// What a classic scrollbar takes on Windows and Linux. The clamp reads
// 100vw, which counts that bar while the row's content box does not get it,
// so a row that only just fits an overlay-scrollbar viewport wraps in a
// window that has a real one. No browser here renders one (device contexts
// use overlays, headless hides them), so it is subtracted instead.
const CLASSIC_SCROLLBAR = 15;

// Rows rather than a height bound: two stacked rows are 90px at 390px, inside
// any bound loose enough to let one row through.
async function assertStrip(page: Page, width: number, rows: number): Promise<void> {
  const strip = page.getByRole("tablist", { name: "Settings" });
  await expect(strip).toBeVisible();
  await settle(page);

  const geometry = await strip.evaluate((el) => {
    // The page column scrolls, not the document, so the scrollbar and the
    // padding that bound this row both belong to that column.
    let column = el.parentElement;
    while (column && getComputedStyle(column).overflowY === "visible") column = column.parentElement;
    column ??= document.documentElement;
    const style = getComputedStyle(column);
    return {
      tabs: [...el.querySelectorAll('[role="tab"]')].map((tab) => {
        const box = tab.getBoundingClientRect();
        return { top: Math.round(box.top), left: box.left, right: box.right, width: box.width };
      }),
      available:
        column.clientWidth - parseFloat(style.paddingInlineStart) - parseFloat(style.paddingInlineEnd),
      scrollbar: column.offsetWidth - column.clientWidth,
    };
  });
  expect(geometry.tabs, "the Settings strip owns exactly the seven page tabs").toHaveLength(7);

  const actual = new Set(geometry.tabs.map((t) => t.top)).size;
  expect(actual, `the seven tabs sit on ${actual} rows at ${width}px, not ${rows}`).toBe(rows);

  // Every tab survives the clamp tappable: no segment under the 2.5rem
  // floor (subpixel tolerance, nothing looser).
  for (const { width: w } of geometry.tabs) {
    expect(w, `a Settings tab shrunk to ${w}px, under the clamp floor`).toBeGreaterThanOrEqual(39.5);
  }

  if (rows > 1) return;

  // The one row has to survive a browser window too, not just a phone: what
  // the tabs and their gaps span must still fit once a classic scrollbar has
  // taken its width off the row.
  const span = Math.max(...geometry.tabs.map((t) => t.right)) - Math.min(...geometry.tabs.map((t) => t.left));
  const windowed = geometry.available - Math.max(0, CLASSIC_SCROLLBAR - geometry.scrollbar);
  expect(
    span,
    `the row spans ${span.toFixed(1)}px and a window with a scrollbar leaves ${windowed.toFixed(1)}px at ${width}px`,
  ).toBeLessThanOrEqual(windowed + 0.5);
}

for (const { width, rows } of [
  { width: 390, rows: 1 },
  { width: 360, rows: 1 },
  { width: 320, rows: 2 },
]) {
  test(`settings tab strip @ ${width}px: the seven tabs sit on ${rows} row(s)`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the clamp lives below 48rem");
    await bootSeededPage(page, "en", width, "/settings");
    await assertStrip(page, width, rows);
  });
}

// The same strip in a desktop browser narrowed to a phone width. The shell
// switches on width alone, so this is a state anyone reaches by dragging a
// window edge, and it is where a real scrollbar shows up.
test("settings tab strip @ 390px in a desktop window: one row with a scrollbar too", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-768", "one desktop project carries this; the pair would run it twice");
  await bootSeededPage(page, "en", 390, "/settings");
  await assertStrip(page, 390, 1);
});

// The appearance card's pinned wells, which spread over the row they get once
// they wrap (Selector's rowFill). A groove is a raised surface, so track with
// nothing on it reads as part of the control; the check is that every row the
// groove draws is covered, in a locale whose labels run longer than English so
// a row cannot pass by being roomy.
for (const width of [390, 360, 320]) {
  test(`appearance wells @ ${width}px: every groove row is filled`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the wells only wrap below 48rem");
    await bootSeededPage(page, "de", width, "/settings");

    const wells = page.locator('[role="tablist"].bg-carbon-surface3, [role="group"].bg-carbon-surface3');
    await expect(wells.first()).toBeVisible();
    await settle(page);

    const bare = await wells.evaluateAll((strips) =>
      strips.flatMap((strip) => {
        const segments = [...strip.children].map((child) => child.getBoundingClientRect());
        if (segments.length === 0) return [];
        const inner = strip.getBoundingClientRect().width - 2 * 3.2;
        const rows = [...new Set(segments.map((box) => Math.round(box.top)))];
        return rows
          .map((top) => {
            const row = segments.filter((box) => Math.round(box.top) === top);
            const used = row.reduce((sum, box) => sum + box.width, 0) + (row.length - 1) * 3.2;
            return { label: strip.getAttribute("aria-label"), top, spare: inner - used };
          })
          .filter((row) => row.spare > 1);
      }),
    );

    expect(bare, `grooves with bare track: ${JSON.stringify(bare)}`).toEqual([]);

    // The segments stay tappable while they spread, and a row of them is a
    // row of equals rather than one stretched leftover.
    const ragged = await wells.evaluateAll((strips) =>
      strips
        .map((strip) => {
          const widths = [...strip.children].map((child) =>
            Math.round(child.getBoundingClientRect().width),
          );
          const heights = [...strip.children].map((child) =>
            Math.round(child.getBoundingClientRect().height),
          );
          return { label: strip.getAttribute("aria-label"), widths: [...new Set(widths)], heights };
        })
        .filter((strip) => strip.widths.length > 1 || strip.heights.some((h) => h < 40)),
    );
    expect(ragged, `uneven or untappable segments: ${JSON.stringify(ragged)}`).toEqual([]);
  });
}

// A card's heading is a notch on the card's top edge, on the phone as much as
// on the desktop. A heading that sits above its card with a gap reads as a
// label floating between two cards, and the page then carries two heading
// forms at once. German on purpose: its labels are the longer ones, so a
// notch that wraps to two lines still has to clear the card's first line.
for (const width of [390, 320]) {
  test(`dashboard @ ${width}px: every card heading sits on its card's top edge`, async ({ page }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the phone blocks render below 48rem");
    await bootSeededPage(page, "de", width, "/dashboard");
    await expect(page.locator("h2 > span").filter({ visible: true }).first()).toBeVisible();
    await settle(page);

    const headings = await page.locator("h2").evaluateAll((els) =>
      els
        .map((h) => {
          const badge = h.querySelector("span");
          if (!badge || !String(badge.className).includes("min-h-[22px]")) return null;
          const box = badge.getBoundingClientRect();
          if (box.width === 0) return null;
          let card: Element | null = h.nextElementSibling;
          if (!card || !String(card.className).includes("rounded-card")) card = h.closest(".rounded-card");
          if (!card) return { label: badge.textContent?.trim(), problem: "no card" };
          const cardBox = card.getBoundingClientRect();
          const first = card.firstElementChild?.getBoundingClientRect();
          return {
            label: badge.textContent?.trim(),
            position: getComputedStyle(badge).position,
            offEdge: Math.round(box.top + box.height / 2 - cardBox.top),
            overlapsFirstLine: first ? Math.round(box.bottom - first.top) : null,
          };
        })
        .filter((x) => x !== null),
    );

    expect(headings.length, "the phone dashboard renders its card headings").toBeGreaterThan(2);
    const wrong = headings.filter(
      (h) =>
        "problem" in h ||
        h.position !== "absolute" ||
        Math.abs(h.offEdge) > 1 ||
        (h.overlapsFirstLine !== null && h.overlapsFirstLine > 0),
    );
    expect(wrong, `headings off their card's edge: ${JSON.stringify(wrong)}`).toEqual([]);
  });
}
