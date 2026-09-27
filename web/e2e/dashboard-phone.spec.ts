// The safety cards on the phone Home: ransomware protection, protection status
// and what nothing backs up, between the anomalies card and the activity log.
// A fresh harness database has nothing scheduled and no off-site copy, so the
// domain status and the coverage report are staged at the route layer with
// long German labels, a failed off-site drill and items whose names run on.
// On the phones: the order, no pan, nothing past the edge of the screen or of
// its card, and targets a finger can hit. On the desktop: the grid in its
// default order, with each card once and no phone block beside it.
import { expect, test, type Page } from "@playwright/test";

const MOBILE_PROJECTS = new Set(["mobile-iphone", "mobile-android"]);

const NOW = Math.floor(Date.now() / 1000);
const HOUR = 3600;
const DAY = 86400;

const RANSOMWARE = "Ransomware-Schutz";
const PROTECTION = "Schutzstatus";
const COVERAGE = "Nicht gesichert";

const DRILL_DETAIL =
  "restic restore in die Sandbox abgebrochen: Paketdatei data/00/beispiel-paket-nummer-eins nicht lesbar";
const LONG_ITEM = "Nextcloud_AIO_Datenbank_Hauptserver_Produktivsystem_Zweitinstanz";

function domain(over: Record<string, unknown>) {
  return {
    domain: "containers",
    enabled: true,
    schedule: "daily 03:00",
    coveredBy: "",
    lastSuccess: NOW - 2 * HOUR,
    periodSeconds: DAY,
    status: "ok",
    lastVerified: NOW - DAY,
    lastVerifiedOK: true,
    verifiedDetail: "",
    drillDetail: "",
    offsiteConfigured: true,
    offsiteImmutable: true,
    lastTamperAt: NOW - 3 * DAY,
    lastTamperOK: true,
    lastReplicationAt: NOW - 3 * HOUR,
    lastReplicationOK: true,
    lastDrDrillAt: NOW - 5 * DAY,
    lastDrDrillOK: true,
    lastOffsiteSubsetAt: NOW - 2 * DAY,
    lastOffsiteSubsetOK: true,
    offsiteDrillScheduled: true,
    protection: "green",
    tamperState: "ok",
    replicationState: "ok",
    drillState: "ok",
    encryptionOn: true,
    pruneStrategySet: true,
    ...over,
  };
}

const DOMAINS = [
  domain({
    status: "overdue",
    lastSuccess: NOW - 3 * DAY,
    offsiteImmutable: false,
    tamperState: "",
    lastTamperAt: 0,
    replicationState: "overdue",
    lastReplicationAt: NOW - 4 * DAY,
    drillState: "failed",
    lastDrDrillOK: false,
    drillDetail: DRILL_DETAIL,
    protection: "amber",
  }),
  domain({
    domain: "vms",
    schedule: "weekly mon,wed,fri 02:30",
    lastDrDrillAt: 0,
    lastDrDrillOK: false,
    offsiteDrillScheduled: false,
    drillState: "",
  }),
  domain({
    domain: "flash",
    status: "never",
    schedule: "everyN 3 04:00",
    lastSuccess: 0,
    lastVerified: 0,
    lastVerifiedOK: false,
    offsiteConfigured: false,
    offsiteImmutable: false,
    lastTamperAt: 0,
    lastReplicationAt: 0,
    lastDrDrillAt: 0,
    lastOffsiteSubsetAt: 0,
    offsiteDrillScheduled: false,
    protection: "red",
    tamperState: "",
    replicationState: "",
    drillState: "",
    pruneStrategySet: false,
  }),
  domain({
    domain: "files",
    schedule: "",
    coveredBy: "15 2 * * 1-5",
    status: "warn",
    tamperState: "stale",
    offsiteDrillScheduled: false,
    lastDrDrillAt: 0,
    drillState: "never",
    protection: "amber",
  }),
  domain({ domain: "zfs", schedule: "daily 01:45" }),
];

const COVERAGE_REPORT = {
  total: 23,
  protected: 17,
  domains: [
    {
      domain: "containers",
      enabled: true,
      total: 14,
      protected: 10,
      unprotected: [
        { name: LONG_ITEM, reason: "not-set-up", neverBackedUp: true },
        { name: "paperless-ngx-dokumentenverwaltung", reason: "override-off", neverBackedUp: false },
        { name: "immich_postgres", reason: "db-dump-only-copy-off", neverBackedUp: false },
        { name: "vaultwarden", reason: "db-dump-failing", neverBackedUp: false },
      ],
    },
    {
      domain: "vms",
      enabled: true,
      total: 3,
      protected: 2,
      unprotected: [{ name: "Windows-Arbeitsplatzrechner Buchhaltung", reason: "not-included", neverBackedUp: true }],
    },
    { domain: "files", enabled: false, total: 0, protected: 0, unprotected: [] },
    {
      domain: "zfs",
      enabled: true,
      total: 6,
      protected: 5,
      unprotected: [
        {
          name: "tank/fotos/archiv/familienurlaube-suedtirol-2019-bis-2026",
          reason: "zfs-member-skipped",
          code: "not-mounted",
          neverBackedUp: false,
        },
      ],
    },
  ],
};

async function stage(page: Page, width?: number): Promise<void> {
  await page.route("**/api/status", (route) => route.fulfill({ json: { ok: true, domains: DOMAINS } }));
  await page.route("**/api/coverage", (route) => route.fulfill({ json: { ok: true, coverage: COVERAGE_REPORT } }));
  // Same seeding as the other phone specs: the stored locale and view are the
  // look, and the server's display prefs must not overwrite them mid-boot.
  // The ransomware card belongs to the advanced view, on the desktop as here.
  await page.route("**/api/display-prefs*", (route) => route.abort());
  await page.addInitScript(() => {
    window.localStorage.setItem("bv-lang", "de");
    window.localStorage.setItem("bombvault.advanced", "1");
  });
  if (width) await page.setViewportSize({ width, height: 800 });
}

async function settle(page: Page): Promise<void> {
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => {})),
    ),
  );
}

async function openHome(page: Page): Promise<void> {
  await page.goto("/dashboard");
  await expect(page.getByText(LONG_ITEM)).toBeVisible();
  await expect(page.getByText(DRILL_DETAIL).first()).toBeVisible();
  await settle(page);
}

async function headings(page: Page): Promise<string[]> {
  return page
    .locator("#bv-main h2")
    .evaluateAll((hs) => hs.map((h) => (h.textContent ?? "").trim()).filter(Boolean));
}

// The card a heading belongs to: the notch's own positioned box.
function card(page: Page, title: string) {
  return page.locator("#bv-main h2", { hasText: title }).locator("xpath=..");
}

for (const width of [320, 360]) {
  test(`the phone Home @ ${width}px carries the safety cards in reading order and nothing pans`, async ({
    page,
  }, testInfo) => {
    test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the Home blocks replace the grid below 48rem");
    await stage(page, width);
    await openHome(page);

    const order = await headings(page);
    for (const title of [RANSOMWARE, PROTECTION, COVERAGE]) {
      expect(order.filter((h) => h === title), `${title} appears once`).toHaveLength(1);
    }
    const at = (title: string) => order.indexOf(title);
    expect(at("Speicher")).toBeGreaterThanOrEqual(0);
    expect(at("Anomalien")).toBeGreaterThan(at("Speicher"));
    expect(at(RANSOMWARE)).toBe(at("Anomalien") + 1);
    expect(at(PROTECTION)).toBe(at(RANSOMWARE) + 1);
    expect(at(COVERAGE)).toBe(at(PROTECTION) + 1);
    expect(at("Aktivitätsprotokoll")).toBe(at(COVERAGE) + 1);

    const layout = await page.evaluate(
      (titles) => {
        const main = document.querySelector("#bv-main")!;
        const vw = window.innerWidth;
        const name = (el: Element) => (el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 40);
        const outside = (el: Element, box: DOMRect) => {
          const r = el.getBoundingClientRect();
          return r.width > 0 && (r.right > box.right + 1 || r.left < box.left - 1);
        };
        const viewport = new DOMRect(0, 0, vw, window.innerHeight);
        const cards = [...main.querySelectorAll("h2")]
          .filter((h) => titles.includes((h.textContent ?? "").trim()))
          .map((h) => h.parentElement!);
        // A row tile inside a card is the box a control has to stay in: a
        // button hanging over its tile's edge still sits inside the card.
        const pastCard = cards.flatMap((c) => {
          const box = c.getBoundingClientRect();
          return [...c.querySelectorAll("button, a, [tabindex], span, p, li")]
            .filter((el) => {
              const tile = el.parentElement?.closest(".rounded-control");
              return outside(el, box) || (!!tile && c.contains(tile) && outside(el, tile.getBoundingClientRect()));
            })
            .map((el) => `${(c.querySelector("h2")!.textContent ?? "").trim()}: ${name(el)}`);
        });
        return {
          docPan: document.documentElement.scrollWidth - vw,
          mainPan: main.scrollWidth - main.clientWidth,
          pastViewport: [...main.querySelectorAll("button, a, input, select, [tabindex], span, p")]
            .filter((el) => outside(el, viewport))
            .map(name),
          pastCard,
        };
      },
      [RANSOMWARE, PROTECTION, COVERAGE],
    );
    expect.soft(layout.docPan, "the document scrolls horizontally").toBeLessThanOrEqual(1);
    expect.soft(layout.mainPan, "#bv-main scrolls horizontally").toBeLessThanOrEqual(1);
    expect.soft(layout.pastViewport, "boxes run past the viewport edge").toEqual([]);
    expect.soft(layout.pastCard, "boxes run past their card").toEqual([]);
    // A backtick in JSX text compiles fine and renders on the page.
    await expect.soft(page.locator("#bv-main")).not.toContainText("`");
  });
}

test("the safety cards on a phone: their controls are big enough to tap", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the larger targets follow the coarse pointer");
  await stage(page, 360);
  await openHome(page);

  const targets = [
    ...(await card(page, RANSOMWARE).getByRole("link").all()),
    ...(await card(page, PROTECTION).getByRole("button").all()),
  ];
  expect(targets.length, "the staged status leaves no control to tap").toBeGreaterThan(3);
  for (const target of targets) {
    await target.scrollIntoViewIfNeeded();
    const box = (await target.boundingBox())!;
    expect(box.height, `${await target.textContent()} is shorter than a fingertip`).toBeGreaterThanOrEqual(43.5);
  }
});

test("the Backup Everything bar stands on the page colour, not on a card-coloured band", async ({ page }, testInfo) => {
  test.skip(!MOBILE_PROJECTS.has(testInfo.project.name), "mobile-only: the bar exists below the breakpoint");
  await stage(page, 360);
  await openHome(page);

  const colours = await page
    .getByRole("button", { name: "Gesamt-Backup" })
    .evaluate((button) => {
      const band = button.closest(".sticky") as HTMLElement;
      const shell = document.getElementById("bv-main")!.closest(".bg-carbon-background") as HTMLElement;
      const card = document.querySelector("#bv-main h2")!.parentElement as HTMLElement;
      return {
        band: getComputedStyle(band).backgroundColor,
        page: getComputedStyle(shell).backgroundColor,
        card: getComputedStyle(card).backgroundColor,
      };
    });
  expect(colours.band).toBe(colours.page);
  expect(colours.band).not.toBe(colours.card);
});

// The desktop grid in its default layout with the advanced view on. The
// recovery-kit banner above the grid comes and goes with the kit, so the list
// starts at the grid.
const DESKTOP_GRID = [
  "Wiederherstellungspunkt",
  "Nächstes Backup",
  "Letztes Ergebnis",
  "Aktivitätsprotokoll",
  PROTECTION,
  COVERAGE,
  "Anomalien",
  RANSOMWARE,
  "Letzte Backups",
  "Ausführungsverlauf",
  "Backup-Verlauf",
  "Speicher",
  "Host-Integration",
];

test("the dashboard on the desktop keeps its grid, each safety card once and no phone block", async ({
  page,
}, testInfo) => {
  test.skip(MOBILE_PROJECTS.has(testInfo.project.name), "desktop-only");
  await stage(page);
  await openHome(page);

  const all = await headings(page);
  const order = all.slice(all.indexOf(DESKTOP_GRID[0]));
  expect(order).toEqual(DESKTOP_GRID);
  for (const title of [RANSOMWARE, PROTECTION, COVERAGE]) {
    expect(order.filter((h) => h === title), `${title} appears once`).toHaveLength(1);
  }
  // The phone blocks are sections under a MobileSectionLabel; the desktop
  // cards are not.
  await expect(page.locator("#bv-main section.glim-notch-card")).toHaveCount(0);
});
