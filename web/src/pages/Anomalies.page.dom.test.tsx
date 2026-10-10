// @vitest-environment jsdom
/**
 * The Anomalies page.
 *
 * Every finding stands on its own in one list, under the name of the item it
 * belongs to, with the way out of it in reach. Settling has to send exactly the
 * ids the pressed button stands for, and what a finding leaves out of the list
 * (the figures, the curve, the item's monitoring) has to open in a window.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";

import { I18nProvider, en } from "../lib/i18n";
import { isolateLtr } from "../lib/ltrFragments";
import { ToastProvider } from "../lib/toast";
import { AnomalyProvider } from "../lib/useAnomalies";
import { ANOMALY_CHANGED_EVENT } from "../lib/anomalies";
import type { AnomalyFilter, AnomalyItem, AnomalySummary, AnomalyView, Settings } from "../lib/api";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getAnomalies: vi.fn(),
    getAnomalySummary: vi.fn(),
    getAnomalyItems: vi.fn(),
    getAnomalyItemSeries: vi.fn(),
    getAnomalyChanges: vi.fn(),
    getSettings: vi.fn(),
    acknowledgeAnomalies: vi.fn(),
    markAnomaliesExpected: vi.fn(),
    setItemAnomalyPrefs: vi.fn(),
    forgetAnomalyExpectation: vi.fn(),
  };
});

const api = await import("../lib/api");
const getAnomalies = vi.mocked(api.getAnomalies);
const getAnomalySummary = vi.mocked(api.getAnomalySummary);
const getAnomalyItems = vi.mocked(api.getAnomalyItems);
const getAnomalyItemSeries = vi.mocked(api.getAnomalyItemSeries);
const getAnomalyChanges = vi.mocked(api.getAnomalyChanges);
const getSettings = vi.mocked(api.getSettings);
const acknowledgeAnomalies = vi.mocked(api.acknowledgeAnomalies);
const markAnomaliesExpected = vi.mocked(api.markAnomaliesExpected);
const setItemAnomalyPrefs = vi.mocked(api.setItemAnomalyPrefs);

const { Anomalies } = await import("./Anomalies");

const GB = 1024 ** 3;
const SEEN = 1700000000;
const SEEN_TEXT = isolateLtr(new Date(SEEN * 1000).toLocaleString());

function summary(over: Partial<AnomalySummary> = {}): AnomalySummary {
  return {
    enabled: true,
    ready: true,
    generation: 1,
    open: { critical: 0, warning: 0, info: 0 },
    recoveredCritical: 0,
    learningItems: 0,
    retentionHeld: 0,
    evalErrors: 0,
    notifyMuted: false,
    backfill: { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 },
    unmeasuredVolumes: [],
    ...over,
  };
}

let seq = 0;
function finding(over: Partial<AnomalyView> = {}): AnomalyView {
  seq += 1;
  return {
    id: `an-${seq}`,
    detector: "source",
    metric: "source_bytes_shrink",
    severity: "critical",
    state: "open",
    scopeKind: "item",
    scopeId: `tg-${seq}`,
    targetId: `tg-${seq}`,
    domain: "container",
    part: "",
    targetName: "",
    name: `item-${seq}`,
    runId: `run-${seq}`,
    lastRunId: `run-${seq}`,
    lastRunAt: SEEN,
    observed: 2 * GB,
    expected: 40 * GB,
    threshold: 20 * GB,
    samples: 12,
    sensitivity: "balanced",
    details: {},
    occurrences: 1,
    firstSeenAt: SEEN,
    lastSeenAt: SEEN,
    recoveredAt: 0,
    resolvedAt: 0,
    ackedAt: 0,
    clearedAt: 0,
    ackNote: "",
    notifiedAt: 0,
    expectable: true,
    retentionHeld: false,
    stillPresent: false,
    ...over,
  };
}

function item(over: Partial<AnomalyItem> = {}): AnomalyItem {
  return {
    targetId: "tg-plex",
    domain: "container",
    name: "plex",
    scheduled: true,
    sensitivity: "",
    effective: "balanced",
    notifyMin: "",
    effectiveNotifyMin: "warning",
    learning: { samples: 10, needed: 10, newData: 10, source: 10, duration: 10, noData: false },
    typical: { sourceBytes: GB, newDataBytes: 1024 ** 2, resticMs: 30000 },
    dump: null,
    datasets: [],
    open: { critical: 0, warning: 0, info: 0 },
    retentionHeld: false,
    selectionSince: 0,
    expectations: [],
    ...over,
  };
}

function settings(over: Partial<Settings> = {}): Settings {
  return {
    anomalyEnabled: true,
    anomalySensitivity: "balanced",
    anomalyNotifyMin: "warning",
    ...over,
  } as Settings;
}

function page(anomalies: AnomalyView[], nextCursor = "") {
  return Promise.resolve({ ok: true as const, anomalies, nextCursor });
}

/** Serves the open and the closed listing the page asks for. */
function serve(open: AnomalyView[], closed: AnomalyView[] = []) {
  getAnomalies.mockImplementation((f?: AnomalyFilter) => page(f?.state === "closed" ? closed : open));
}

function Where() {
  const { pathname, search, hash } = useLocation();
  return <output data-testid="where">{pathname + search + hash}</output>;
}

async function renderPage(path = "/anomalies") {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <MemoryRouter initialEntries={[path]}>
            <AnomalyProvider>
              <Anomalies />
              <Where />
            </AnomalyProvider>
          </MemoryRouter>
        </ToastProvider>
      </I18nProvider>
    );
  });
}

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

/** Opens a SelectField by its accessible name and picks one of its options. */
async function pick(fieldLabel: string, optionLabel: string) {
  fireEvent.click(screen.getByRole("combobox", { name: fieldLabel }));
  await press(screen.getByRole("option", { name: optionLabel }));
}

/** The findings on the page, top to bottom. */
function findings(): HTMLElement[] {
  return [...document.querySelectorAll<HTMLElement>("li[id^='finding-']")];
}

/** The finding filed under an item's name. */
function findingOf(name: string): HTMLElement {
  const hit = findings().find((li) => within(li).queryByText(name, { selector: "a, span" }));
  if (!hit) throw new Error(`no finding of ${name}`);
  return hit;
}

function names(): (string | null | undefined)[] {
  return findings().map((li) => li.querySelector("a, span.font-semibold")?.textContent);
}

function tile(label: string): HTMLElement {
  const tiles = screen.getByRole("group", { name: en["anomaly.filter.severity"] });
  return within(tiles).getByRole("button", { name: new RegExp(`^${label}`) });
}

async function openDetails(of: HTMLElement): Promise<HTMLElement> {
  await press(within(of).getByRole("button", { name: en["fleet.details"] }));
  return screen.getByRole("dialog");
}

async function showClosed() {
  await press(screen.getByRole("tab", { name: en["anomaly.closedRecent"] }));
}

/** The ring of one item, by the name it starts with. */
function ring(name: string): HTMLElement {
  return screen.getByRole("button", { name: new RegExp(`^${name} · `) });
}

async function openMonitoring(name: string): Promise<HTMLElement> {
  await press(ring(name));
  return screen.getByRole("dialog");
}

const restoreLabel = en["anomaly.action.restoreLastGood"].replace("{date}", SEEN_TEXT);
const scrolled = vi.fn();

beforeEach(() => {
  seq = 0;
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  Element.prototype.scrollIntoView = scrolled;
  scrolled.mockReset();
  getAnomalies.mockReset();
  getAnomalySummary.mockReset();
  getAnomalyItems.mockReset();
  getAnomalyItemSeries.mockReset();
  getAnomalyChanges.mockReset();
  getSettings.mockReset();
  acknowledgeAnomalies.mockReset();
  markAnomaliesExpected.mockReset();
  setItemAnomalyPrefs.mockReset();

  getAnomalySummary.mockResolvedValue({ ok: true, summary: summary() });
  getAnomalyItems.mockResolvedValue({ ok: true, items: [] });
  getAnomalyItemSeries.mockResolvedValue({ ok: true, series: [] });
  getAnomalyChanges.mockResolvedValue({ ok: true, changes: { state: "running" } });
  getSettings.mockResolvedValue({ ok: true, settings: settings() });
  serve([]);
});

afterEach(() => {
  cleanup();
  localStorage.removeItem("bv-lang");
});

describe("the list", () => {
  it("gives every finding a line of its own under its item's name", async () => {
    serve([
      finding({ targetId: "tg-plex", name: "plex" }),
      finding({ targetId: "tg-plex", name: "plex", scopeKind: "dump", metric: "dump_bytes_growth", severity: "warning" }),
      finding({ targetId: "tg-sonarr", name: "sonarr", severity: "warning" }),
    ]);
    await renderPage();

    expect(names()).toEqual(["plex", "plex", "sonarr"]);
    expect(within(findings()[1]!).getByText(en["anomaly.items.dumpSeries"])).toBeTruthy();
    expect(within(findings()[0]!).queryByText(en["anomaly.items.dumpSeries"])).toBeNull();
  });

  it("leads from the name to the item's own page", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex" })]);
    await renderPage();
    expect(within(findings()[0]!).getByRole("link", { name: "plex" }).getAttribute("href")).toBe("/containers");
  });

  it("files a restore check and a disk under the kind of check and says what they cover", async () => {
    serve([
      finding({ metric: "drill_dr", detector: "integrity", scopeKind: "domain", scopeId: "files:offsite:t1", targetId: "", domain: "files", name: "", targetName: "wasabi" }),
      finding({ metric: "capacity_low", detector: "capacity", severity: "info", scopeKind: "volume", scopeId: "vol-1", targetId: "", domain: "container", name: "", observed: 0.04, details: { freeBytes: 30 * GB } }),
    ]);
    await renderPage();

    const check = findingOf(en["anomaly.detector.integrity"]);
    expect(within(check).getByText(/wasabi/)).toBeTruthy();
    expect(within(check).queryByRole("link")).toBeNull();
    expect(
      within(findingOf(en["anomaly.detector.capacity"])).getByText(
        new RegExp(en["anomaly.card.volume"].replace("{domains}", "Containers"))
      )
    ).toBeTruthy();
  });

  it("puts the worst finding first and tints only a critical one", async () => {
    serve([
      finding({ targetId: "tg-a", name: "calm", severity: "info" }),
      finding({ targetId: "tg-b", name: "loud", severity: "critical" }),
    ]);
    await renderPage();

    const [first, second] = findings();
    expect(names()).toEqual(["loud", "calm"]);
    expect(first!.className).toContain("bg-statusFailBgSoft");
    expect(second!.className).not.toContain("bg-statusFailBgSoft");
  });

  it("says what happened without repeating the name, and the whole sentence in the window", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex" })]);
    await renderPage();

    const line = findings()[0]!;
    const short = en["anomaly.short.shrink"]
      .replace("{current}", isolateLtr("2.0 GB"))
      .replace("{typical}", isolateLtr("40.0 GB"));
    expect(within(line).getByText(short)).toBeTruthy();
    expect(within(line).getAllByText(/plex/)).toHaveLength(1);

    const window = await openDetails(line);
    expect(
      within(window).getByText(
        en["anomaly.sentence.sourceShrink"]
          .replace("{name}", "plex")
          .replace("{current}", isolateLtr("2.0 GB"))
          .replace("{typical}", isolateLtr("40.0 GB"))
      )
    ).toBeTruthy();
  });

  it("names the dataset beside a dataset's finding", async () => {
    serve([
      finding({ targetId: "tg-tank", name: "tank", domain: "zfs", scopeKind: "zfsds", part: "tank/media" }),
    ]);
    await renderPage();
    const series = within(findingOf("tank")).getByText("tank/media");
    expect(series.getAttribute("dir")).toBe("ltr");
  });

  it("says how often a finding was seen and which backup was the last good one", async () => {
    serve([
      finding({
        targetId: "tg-plex",
        name: "plex",
        occurrences: 3,
        lastGood: { runId: "run-9", snapshotId: "snap-9", at: SEEN },
      }),
    ]);
    await renderPage();
    const facts = within(findings()[0]!).getByText(/Containers · /).textContent;
    expect(facts).toContain(en["anomaly.occurrences"].replace("{n}", "3"));
    expect(facts).toContain(`${en["anomaly.detail.lastGood"]}: ${SEEN_TEXT}`);
  });
});

describe("settling findings", () => {
  it("acknowledges all of one item's findings in one call and leaves the other items alone", async () => {
    const plex = [
      finding({ targetId: "tg-plex", name: "plex", severity: "warning" }),
      finding({ targetId: "tg-plex", name: "plex", severity: "warning", metric: "duration_slower" }),
    ];
    serve([...plex, finding({ targetId: "tg-sonarr", name: "sonarr", severity: "warning" })]);
    acknowledgeAnomalies.mockResolvedValue({ ok: true, changed: 2, skipped: 0, released: 0 });
    const heard = vi.fn();
    window.addEventListener(ANOMALY_CHANGED_EVENT, heard);

    await renderPage();
    const all = en["anomaly.action.acknowledgeAll"].replace("{n}", "2");
    expect(screen.getAllByRole("button", { name: all })).toHaveLength(1);
    await press(within(findings()[0]!).getByRole("button", { name: all }));

    expect(acknowledgeAnomalies).toHaveBeenCalledTimes(1);
    expect(acknowledgeAnomalies.mock.calls[0]![0]).toEqual(plex.map((a) => a.id));
    expect(heard).toHaveBeenCalled();
    expect(screen.getByText(en["anomaly.toast.acknowledgedAll"].replace("{n}", "2"))).toBeTruthy();
    window.removeEventListener(ANOMALY_CHANGED_EVENT, heard);
  });

  it("acknowledges only the findings the chosen tile shows", async () => {
    const note = finding({ targetId: "tg-plex", name: "plex", severity: "info" });
    serve([
      finding({ targetId: "tg-plex", name: "plex", severity: "warning" }),
      finding({ targetId: "tg-plex", name: "plex", severity: "warning", metric: "duration_slower" }),
      note,
    ]);
    acknowledgeAnomalies.mockResolvedValue({ ok: true, changed: 2, skipped: 0, released: 0 });
    await renderPage();
    expect(screen.getByRole("button", { name: en["anomaly.action.acknowledgeAll"].replace("{n}", "3") })).toBeTruthy();

    await press(tile(en["anomaly.tile.warning"]));
    await press(screen.getByRole("button", { name: en["anomaly.action.acknowledgeAll"].replace("{n}", "2") }));
    expect(acknowledgeAnomalies.mock.calls[0]![0]).not.toContain(note.id);
  });

  it("asks first when a finding is keeping old backups", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex", severity: "warning", retentionHeld: true })]);
    acknowledgeAnomalies.mockResolvedValue({ ok: true, changed: 1, skipped: 0, released: 1 });
    await renderPage();

    expect(within(findings()[0]!).getByText(en["anomaly.retentionKept"])).toBeTruthy();
    fireEvent.click(within(findings()[0]!).getByRole("button", { name: en["anomaly.action.acknowledge"] }));
    expect(screen.getByText(en["anomaly.releaseConfirm"].replace("{name}", "plex"))).toBeTruthy();
    expect(acknowledgeAnomalies).not.toHaveBeenCalled();

    await press(within(screen.getByRole("dialog")).getByRole("button", { name: en["anomaly.action.acknowledge"] }));
    expect(acknowledgeAnomalies).toHaveBeenCalledWith(["an-1"]);
  });

  it("marks one finding as expected from its line, without a note", async () => {
    const rows = [
      finding({ targetId: "tg-plex", name: "plex", severity: "warning" }),
      finding({ targetId: "tg-plex", name: "plex", severity: "warning", metric: "source_bytes_growth" }),
    ];
    serve(rows);
    markAnomaliesExpected.mockResolvedValue({ ok: true, changed: 1, skipped: 0, released: 0 });
    await renderPage();

    await press(within(findings()[1]!).getByRole("button", { name: en["anomaly.action.expected"] }));
    expect(markAnomaliesExpected).toHaveBeenCalledWith([rows[1]!.id]);
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.getByText(en["anomaly.markedExpected"])).toBeTruthy();
  });

  it("offers no way to mark a finding as expected that cannot be", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex", expectable: false })]);
    await renderPage();
    expect(screen.queryByRole("button", { name: en["anomaly.action.expected"] })).toBeNull();
    expect(screen.getByRole("button", { name: en["anomaly.action.acknowledge"] })).toBeTruthy();
  });

  it("says why when the server refuses", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex" })]);
    acknowledgeAnomalies.mockResolvedValue({ ok: false, code: "not-found", changed: 0, skipped: 0, released: 0 });
    await renderPage();
    await press(screen.getByRole("button", { name: en["anomaly.action.acknowledge"] }));
    expect(screen.getByText(en["anomaly.error.notFound"])).toBeTruthy();
  });

  it("settles a finding from its window and closes the window", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex", severity: "warning" })]);
    acknowledgeAnomalies.mockResolvedValue({ ok: true, changed: 1, skipped: 0, released: 0 });
    await renderPage();

    const window = await openDetails(findings()[0]!);
    await press(within(window).getByRole("button", { name: en["anomaly.action.acknowledge"] }));
    expect(acknowledgeAnomalies).toHaveBeenCalledWith(["an-1"]);
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("the way out of a finding", () => {
  const lastGood = { runId: "run-9", snapshotId: "snap-9", at: SEEN };

  it("makes restoring the main action of a critical finding with a good backup", async () => {
    serve([
      finding({ targetId: "tg-plex", name: "plex", lastGood }),
      finding({ targetId: "tg-sonarr", name: "sonarr", lastGood }),
    ]);
    await renderPage();

    const [first, second] = findings().map((li) => within(li).getByRole("button", { name: restoreLabel }));
    expect(first!.className).toContain("bg-accent");
    expect(second!.className).not.toContain("bg-accent");
    await press(first!);
    expect(screen.getByTestId("where").textContent).toBe("/containers?restore=snap-9&at=1700000000&item=plex");
  });

  it("offers a warning's last good backup in its window", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex", severity: "warning", lastGood })]);
    await renderPage();
    expect(within(findings()[0]!).queryByRole("button", { name: restoreLabel })).toBeNull();

    const window = await openDetails(findings()[0]!);
    await press(within(window).getByRole("button", { name: restoreLabel }));
    expect(screen.getByTestId("where").textContent).toBe("/containers?restore=snap-9&at=1700000000&item=plex");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("restores a dataset's last good backup at its ZFS item, naming the dataset", async () => {
    serve([
      finding({
        targetId: "tg-tank",
        severity: "warning",
        domain: "zfs",
        name: "tank/media",
        scopeKind: "zfsds",
        scopeId: "tank/media/photos",
        part: "tank/media/photos",
        lastGood,
      }),
    ]);
    await renderPage();
    const window = await openDetails(findings()[0]!);
    await press(within(window).getByRole("button", { name: restoreLabel }));
    expect(screen.getByTestId("where").textContent).toBe(
      "/zfs?restore=snap-9&at=1700000000&item=tank%2Fmedia&dataset=tank%2Fmedia%2Fphotos"
    );
  });

  it("offers the comparison where the server can make one, and starts it in the window", async () => {
    serve([
      finding({ targetId: "tg-plex", name: "plex", severity: "warning" }),
      finding({ targetId: "tg-sonarr", name: "sonarr", severity: "warning", metric: "duration_slower" }),
    ]);
    await renderPage();

    expect(within(findingOf("sonarr")).queryByRole("button", { name: en["anomaly.changes.title"] })).toBeNull();
    await press(within(findingOf("plex")).getByRole("button", { name: en["anomaly.changes.title"] }));
    expect(getAnomalyChanges).toHaveBeenCalledWith("an-1", false);
    expect(within(screen.getByRole("dialog")).getByRole("status").textContent).toBe(en["breakdown.running"]);
  });

  it("waits with the comparison until it is asked for", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex", severity: "warning" })]);
    await renderPage();
    const window = await openDetails(findings()[0]!);
    expect(getAnomalyChanges).not.toHaveBeenCalled();
    await press(within(window).getByRole("button", { name: en["anomaly.changes.title"] }));
    expect(getAnomalyChanges).toHaveBeenCalledTimes(1);
    expect(within(window).queryByRole("button", { name: en["anomaly.changes.title"] })).toBeNull();
  });
});

describe("a finding's window", () => {
  function row(window: HTMLElement, term: string): string | null | undefined {
    return within(window).getByText(term, { selector: "dt" }).nextElementSibling?.textContent;
  }

  it("opens over the page and closes again", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex" })]);
    await renderPage();
    expect(screen.queryByRole("dialog")).toBeNull();

    const window = await openDetails(findings()[0]!);
    expect(window.getAttribute("aria-modal")).toBe("true");
    expect(document.getElementById(window.getAttribute("aria-labelledby")!)?.textContent).toBe("plex");
    await press(within(window).getByRole("button", { name: en["common.close"] }));
    expect(screen.queryByRole("dialog")).toBeNull();

    await openDetails(findings()[0]!);
    await act(async () => {
      fireEvent.keyDown(document, { key: "Escape" });
    });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("shows the measured and the usual level", async () => {
    serve([finding({})]);
    await renderPage();
    const window = await openDetails(findings()[0]!);
    expect(row(window, en["anomaly.detail.observed"])).toBe(isolateLtr("2.0 GB"));
    expect(row(window, en["anomaly.detail.expected"])).toBe(isolateLtr("40.0 GB"));
  });

  // The detector measures a rate per second; the window has to say per hour.
  it("reads the largest usual amount per hour out of a per-second rate", async () => {
    serve([finding({ metric: "new_data", detector: "new_data", details: { refRate: 1024 } })]);
    await renderPage();
    const window = await openDetails(findings()[0]!);
    expect(within(window).getByText(en["anomaly.detail.more"])).toBeTruthy();
    expect(row(window, en["anomaly.detail.refRate"])).toBe(isolateLtr("3.5 MB"));
  });

  // The two projections are independent: one is what BombVault itself writes,
  // the other is everything filling the same disk.
  it("says where a capacity projection comes from", async () => {
    serve([
      finding({
        metric: "capacity_eta",
        detector: "capacity",
        scopeKind: "volume",
        targetId: "",
        observed: 6,
        details: { etaGrowthDays: 12, etaFreeDays: 6, slopePerDay: 2 * GB },
      }),
    ]);
    await renderPage();
    const window = await openDetails(findings()[0]!);
    expect(row(window, en["anomaly.detail.etaGrowth"])).toBe("12 days");
    expect(row(window, en["anomaly.detail.etaFree"])).toBe("6 days");
    expect(row(window, en["anomaly.detail.slope"])).toBe(isolateLtr("2.0 GB"));
  });

  it("asks for the item's measurements and draws them where a finding has any", async () => {
    serve([
      finding({ targetId: "tg-plex", name: "plex" }),
      finding({ metric: "capacity_low", detector: "capacity", severity: "info", scopeKind: "volume", targetId: "", name: "" }),
    ]);
    getAnomalyItemSeries.mockResolvedValue({
      ok: true,
      series: [
        {
          scopeKind: "item",
          part: "",
          failed: [],
          quantities: [
            {
              quantity: "sourceBytes",
              points: [
                { runId: "run-0", at: SEEN - 86400, value: 40 * GB },
                { runId: "run-1", at: SEEN, value: 2 * GB },
              ],
              learning: false,
              samples: 10,
              needed: 10,
              band: { low: { metric: "source_bytes_shrink", expected: 40 * GB, threshold: 20 * GB, samples: 10 }, high: null },
            },
          ],
        },
      ],
    });
    await renderPage();

    const window = await openDetails(findingOf("plex"));
    expect(getAnomalyItemSeries).toHaveBeenCalledWith("tg-plex");
    expect(within(window).getByRole("img", { name: new RegExp(`^${en["anomaly.curve.sourceBytes"]}`) })).toBeTruthy();
    expect(window.querySelector("[data-part='marked']")).toBeTruthy();
    await press(within(window).getByRole("button", { name: en["common.close"] }));

    getAnomalyItemSeries.mockClear();
    await openDetails(findingOf(en["anomaly.detector.capacity"]));
    expect(getAnomalyItemSeries).not.toHaveBeenCalled();
  });

  it("says that the cause of a critical finding has gone away", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex", recoveredAt: SEEN })]);
    await renderPage();
    expect(within(findings()[0]!).getByText(en["anomaly.recovered"])).toBeTruthy();
    const window = await openDetails(findings()[0]!);
    expect(within(window).getByText(en["anomaly.recoveredHint"].replace("{date}", SEEN_TEXT))).toBeTruthy();
  });
});

describe("the severity tiles", () => {
  const mixed = () => [
    finding({ targetId: "tg-a", name: "alpha", severity: "critical" }),
    finding({ targetId: "tg-b", name: "beta", severity: "warning" }),
    finding({ targetId: "tg-c", name: "gamma", severity: "warning" }),
    finding({ targetId: "tg-d", name: "delta", severity: "info" }),
  ];

  it("count the open findings, with one tile for all of them", async () => {
    serve(mixed());
    await renderPage();

    const open = (n: number) => en["anomaly.tile.open"].replace("{n}", String(n));
    expect(tile(en["filter.all"]).textContent).toContain(open(4));
    expect(tile(en["anomaly.severity.critical"]).textContent).toContain(open(1));
    expect(tile(en["anomaly.tile.warning"]).textContent).toContain(open(2));
    expect(tile(en["anomaly.tile.info"]).textContent).toContain(open(1));
    expect(tile(en["filter.all"]).getAttribute("aria-pressed")).toBe("true");
    expect(tile(en["anomaly.tile.warning"]).getAttribute("aria-pressed")).toBe("false");
  });

  it("show what the chosen tile stands for and keep every tile in place", async () => {
    serve(mixed());
    await renderPage();

    await press(tile(en["anomaly.tile.warning"]));
    expect(names()).toEqual(["beta", "gamma"]);
    expect(tile(en["anomaly.tile.warning"]).getAttribute("aria-pressed")).toBe("true");
    expect(tile(en["filter.all"]).getAttribute("aria-pressed")).toBe("false");
    expect(within(screen.getByRole("group", { name: en["anomaly.filter.severity"] })).getAllByRole("button")).toHaveLength(4);
    expect(tile(en["anomaly.severity.critical"]).textContent).toContain(en["anomaly.tile.open"].replace("{n}", "1"));

    await press(tile(en["filter.all"]));
    expect(names()).toEqual(["alpha", "beta", "gamma", "delta"]);
  });

  it("show two severities together and fall back to all once every one is chosen", async () => {
    serve(mixed());
    await renderPage();

    await press(tile(en["anomaly.tile.info"]));
    await press(tile(en["anomaly.severity.critical"]));
    expect(names()).toEqual(["alpha", "delta"]);

    await press(tile(en["anomaly.tile.warning"]));
    expect(names()).toHaveLength(4);
    expect(tile(en["filter.all"]).getAttribute("aria-pressed")).toBe("true");

    await press(tile(en["anomaly.tile.info"]));
    await press(tile(en["anomaly.tile.info"]));
    expect(tile(en["filter.all"]).getAttribute("aria-pressed")).toBe("true");
  });

  it("say so when nothing is open at the chosen severity", async () => {
    serve([finding({ severity: "info" })]);
    await renderPage();
    await press(tile(en["anomaly.severity.critical"]));
    expect(screen.getByText(en["anomaly.emptySeverity"])).toBeTruthy();
    expect(findings()).toHaveLength(0);
  });

  it("count the closed findings once the closed ones are shown", async () => {
    serve(mixed(), [finding({ name: "old", severity: "warning", state: "acknowledged", ackedAt: SEEN })]);
    await renderPage();
    await showClosed();
    expect(tile(en["filter.all"]).textContent).toContain(en["anomaly.tile.closed"].replace("{n}", "1"));
    expect(tile(en["anomaly.severity.critical"]).textContent).toContain(en["anomaly.tile.closed"].replace("{n}", "0"));
  });
});

describe("open and closed", () => {
  it("starts on the open findings and asks for the closed ones only when they are chosen", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex" })], [finding({ name: "sonarr", state: "resolved", resolvedAt: SEEN })]);
    await renderPage();

    expect(screen.getByRole("tab", { name: en["anomaly.state.open"] }).getAttribute("aria-selected")).toBe("true");
    expect(getAnomalies.mock.calls.every(([f]) => f?.state === "open")).toBe(true);

    await showClosed();
    expect(names()).toEqual(["sonarr"]);
    await press(screen.getByRole("tab", { name: en["anomaly.state.open"] }));
    expect(names()).toEqual(["plex"]);
  });

  it("reads the last 30 days and shows how each finding was closed, with its note", async () => {
    serve([], [
      finding({ name: "plex", state: "acknowledged", severity: "warning", ackedAt: SEEN, ackNote: "cleaned up on purpose" }),
      finding({ name: "sonarr", state: "resolved", severity: "warning", resolvedAt: SEEN }),
      finding({ name: "radarr", state: "expected", severity: "info", ackedAt: SEEN, stillPresent: true }),
    ]);
    await renderPage();
    await showClosed();

    const since = getAnomalies.mock.calls.find(([f]) => f?.state === "closed")![0]!.since!;
    expect(Math.abs(since - (Date.now() / 1000 - 30 * 86400))).toBeLessThan(60);

    const plex = findingOf("plex");
    expect(within(plex).getByText(en["anomaly.state.acknowledged"])).toBeTruthy();
    expect(within(plex).getByText(new RegExp(`${en["anomaly.noteLabel"]}: cleaned up on purpose`))).toBeTruthy();
    expect(within(plex).getByText(new RegExp(en["anomaly.closedAt"].replace("{date}", "")))).toBeTruthy();
    expect(within(findingOf("sonarr")).getByText(en["anomaly.state.resolved"])).toBeTruthy();
    expect(within(findingOf("radarr")).getByText(en["anomaly.stillPresent"])).toBeTruthy();
  });

  it("offers a closed finding's details and nothing that would settle it again", async () => {
    serve([], [finding({ name: "plex", state: "acknowledged", ackedAt: SEEN, ackNote: "seen" })]);
    await renderPage();
    await showClosed();

    expect(screen.queryByRole("button", { name: en["anomaly.action.acknowledge"] })).toBeNull();
    const window = await openDetails(findings()[0]!);
    expect(within(window).getByText(`${en["anomaly.noteLabel"]}: seen`)).toBeTruthy();
    expect(within(window).queryByRole("button", { name: en["anomaly.action.acknowledge"] })).toBeNull();
  });

  it("says when nothing was closed", async () => {
    await renderPage();
    await showClosed();
    expect(screen.getByText(en["anomaly.emptyClosed"])).toBeTruthy();
  });

  it("follows the cursor to the end of the closed findings", async () => {
    const first = finding({ name: "plex", state: "resolved", resolvedAt: SEEN });
    const second = finding({ name: "sonarr", state: "resolved", resolvedAt: SEEN });
    getAnomalies.mockImplementation((f?: AnomalyFilter) =>
      f?.state === "closed" ? (f.cursor ? page([second]) : page([first], "cur-2")) : page([])
    );
    await renderPage();
    await showClosed();
    expect(names()).toEqual(["plex", "sonarr"]);
  });

  it("offers another try when the closed findings were refused", async () => {
    getAnomalies.mockImplementation((f?: AnomalyFilter) =>
      f?.state === "closed" ? Promise.reject(new Error("offline")) : page([])
    );
    await renderPage();
    await showClosed();
    expect(screen.getByText(en["anomaly.loadFailed"])).toBeTruthy();

    serve([], [finding({ name: "plex", state: "resolved", resolvedAt: SEEN })]);
    await press(screen.getByRole("button", { name: en["anomaly.retry"] }));
    expect(names()).toEqual(["plex"]);
  });
});

describe("a link to one item", () => {
  it("brings the item's only finding into view and opens it", async () => {
    serve([
      finding({ targetId: "tg-a", name: "alpha" }),
      finding({ targetId: "tg-b", name: "beta", severity: "warning" }),
    ]);
    await renderPage("/anomalies?scope=item:tg-b");

    expect(scrolled).toHaveBeenCalledTimes(1);
    expect(scrolled.mock.contexts[0]).toBe(findingOf("beta"));
    const window = screen.getByRole("dialog");
    expect(document.getElementById(window.getAttribute("aria-labelledby")!)?.textContent).toBe("beta");
  });

  it("brings the first of several findings into view and opens none", async () => {
    serve([
      finding({ targetId: "tg-a", name: "alpha" }),
      finding({ targetId: "tg-b", name: "beta", severity: "info" }),
      finding({ targetId: "tg-b", name: "beta", severity: "warning" }),
    ]);
    await renderPage("/anomalies?scope=item:tg-b");

    expect(scrolled).toHaveBeenCalledTimes(1);
    expect(scrolled.mock.contexts[0]).toBe(findings()[1]);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("opens the monitoring of an item without findings", async () => {
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item(), item({ targetId: "tg-2", name: "sonarr" })] });
    await renderPage("/anomalies?scope=item:tg-2");
    const window = screen.getByRole("dialog");
    expect(within(window).getByText("sonarr")).toBeTruthy();
    expect(within(window).getByRole("combobox", { name: en["anomaly.items.sensitivity"] })).toBeTruthy();
  });
});

describe("learning progress", () => {
  function lit(name: string): Element[] {
    return [...ring(name).querySelectorAll("circle[data-lit='true']")];
  }

  it("gives every watched item a ring of ten parts, one lit for each backup learned from", async () => {
    getAnomalyItems.mockResolvedValue({
      ok: true,
      items: [
        item(),
        item({ targetId: "tg-2", name: "sonarr", learning: { samples: 4, needed: 10, newData: 4, source: 4, duration: 3, noData: false } }),
        item({ targetId: "tg-3", name: "radarr", learning: { samples: 0, needed: 10, newData: 0, source: 0, duration: 0, noData: false } }),
      ],
    });
    serve([finding({ targetId: "tg-3", name: "radarr" })]);
    await renderPage();

    for (const name of ["plex", "sonarr", "radarr"]) expect(ring(name).querySelectorAll("circle")).toHaveLength(10);
    expect(lit("plex")).toHaveLength(10);
    expect(lit("sonarr")).toHaveLength(4);
    expect(lit("radarr")).toHaveLength(0);
  });

  it("turns a full ring green and leaves one still filling in the accent", async () => {
    getAnomalyItems.mockResolvedValue({
      ok: true,
      items: [
        item(),
        item({ targetId: "tg-2", name: "sonarr", learning: { samples: 4, needed: 10, newData: 4, source: 4, duration: 3, noData: false } }),
      ],
    });
    await renderPage();
    expect(lit("plex").every((c) => c.classList.contains("stroke-statusOkSolid"))).toBe(true);
    expect(lit("sonarr").every((c) => c.classList.contains("stroke-accent"))).toBe(true);
  });

  it("says how far each item has come, and that one is not scheduled", async () => {
    getAnomalyItems.mockResolvedValue({
      ok: true,
      items: [
        item(),
        item({
          targetId: "tg-2",
          name: "sonarr",
          scheduled: false,
          learning: { samples: 4, needed: 10, newData: 4, source: 4, duration: 3, noData: false },
        }),
      ],
    });
    await renderPage();

    expect(ring("plex").getAttribute("aria-label")).toBe(`plex · ${en["anomaly.learningDone"]}`);
    const learning = en["anomaly.learningProgress"].replace("{n}", "4").replace("{needed}", "10");
    expect(ring("sonarr").getAttribute("aria-label")).toBe(`sonarr · ${learning} · ${en["anomaly.items.notScheduled"]}`);
  });

  it("grows in only the parts a backup added while the page was open", async () => {
    const learning = (samples: number) => ({ samples, needed: 10, newData: samples, source: samples, duration: samples, noData: false });
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item({ learning: learning(4) })] });
    await renderPage();
    expect(ring("plex").querySelectorAll(".glim-ring-grow")).toHaveLength(0);

    getAnomalySummary.mockResolvedValue({ ok: true, summary: summary({ generation: 2 }) });
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item({ learning: learning(6) })] });
    await act(async () => {
      window.dispatchEvent(new Event(ANOMALY_CHANGED_EVENT));
    });
    await waitFor(() => expect(lit("plex")).toHaveLength(6));
    expect(ring("plex").querySelectorAll(".glim-ring-grow")).toHaveLength(2);
  });

  it("stands aside while a severity or the closed findings are chosen", async () => {
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
    serve([finding({ targetId: "tg-x", name: "sonarr", severity: "warning" })]);
    await renderPage();
    expect(ring("plex")).toBeTruthy();

    await press(tile(en["anomaly.tile.warning"]));
    expect(screen.queryByRole("button", { name: /^plex · / })).toBeNull();
    await press(tile(en["filter.all"]));
    expect(ring("plex")).toBeTruthy();

    await showClosed();
    expect(screen.queryByRole("button", { name: /^plex · / })).toBeNull();
  });

  it("offers another try instead of an empty card when the items were refused", async () => {
    getAnomalyItems.mockRejectedValueOnce(new Error("offline"));
    await renderPage();
    expect(screen.getByText(en["anomaly.loadFailed"])).toBeTruthy();

    getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
    await press(screen.getByRole("button", { name: en["anomaly.retry"] }));
    expect(ring("plex")).toBeTruthy();
  });
});

describe("monitoring", () => {
  it("opens from a ring, with the item's own sensitivity and notifications", async () => {
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
    await renderPage();

    const window = await openMonitoring("plex");
    expect(document.getElementById(window.getAttribute("aria-labelledby")!)?.textContent).toBe(
      en["anomaly.card.monitoring"]
    );
    expect(within(window).getByRole("combobox", { name: en["anomaly.items.sensitivity"] })).toBeTruthy();
    expect(within(window).getByRole("combobox", { name: en["anomaly.items.notifyMin"] })).toBeTruthy();
    expect(within(window).getByText(en["anomaly.learn.title"])).toBeTruthy();
    expect(getAnomalyItemSeries).toHaveBeenCalledWith("tg-plex");
    expect(within(window).queryByRole("button", { name: en["common.back"] })).toBeNull();
  });

  it("opens from a finding's window and leads back to it", async () => {
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
    serve([finding({ targetId: "tg-plex", name: "plex" })]);
    await renderPage();

    const details = await openDetails(findings()[0]!);
    await press(within(details).getByRole("button", { name: en["anomaly.card.monitoring"] }));
    const window = screen.getByRole("dialog");
    expect(within(window).getByRole("combobox", { name: en["anomaly.items.sensitivity"] })).toBeTruthy();

    await press(within(window).getByRole("button", { name: en["common.back"] }));
    expect(within(screen.getByRole("dialog")).getByText(en["anomaly.detail.observed"])).toBeTruthy();
  });

  it("is not offered for a finding no item stands behind", async () => {
    serve([finding({ metric: "capacity_low", detector: "capacity", severity: "info", scopeKind: "volume", targetId: "", name: "" })]);
    await renderPage();
    const window = await openDetails(findings()[0]!);
    expect(within(window).queryByRole("button", { name: en["anomaly.card.monitoring"] })).toBeNull();
    expect(within(window).queryByRole("button", { name: en["anomaly.openItem"] })).toBeNull();
  });

  it("leads on to the item's own page", async () => {
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item({ domain: "vm", name: "win11" })] });
    await renderPage();
    const window = await openMonitoring("win11");
    await press(within(window).getByRole("button", { name: en["anomaly.openItem"] }));
    expect(screen.getByTestId("where").textContent).toBe("/vms");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("saves a sensitivity straight away and puts it back when the server refuses", async () => {
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
    setItemAnomalyPrefs.mockResolvedValue({ ok: false, error: "no", code: "bad-request" });
    await renderPage();
    await openMonitoring("plex");

    await pick(en["anomaly.items.sensitivity"], en["anomaly.sensitivity.strict"]);
    expect(setItemAnomalyPrefs).toHaveBeenCalledWith("tg-plex", { sensitivity: "strict" });
    await waitFor(() => expect(screen.getByText(en["anomaly.error.badRequest"])).toBeTruthy());
    expect(
      screen.getByRole("combobox", { name: en["anomaly.items.sensitivity"] }).textContent
    ).toContain(en["anomaly.sensitivity.follow"].replace("{preset}", en["anomaly.sensitivity.balanced"]));
  });

  it("saves the item's notification minimum on its own", async () => {
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
    setItemAnomalyPrefs.mockResolvedValue({ ok: true });
    await renderPage();
    await openMonitoring("plex");

    await pick(en["anomaly.items.notifyMin"], en["anomaly.settings.notify.critical"]);
    expect(setItemAnomalyPrefs).toHaveBeenCalledWith("tg-plex", { notifyMin: "critical" });
  });

  it("says how far an item still learning has come", async () => {
    getAnomalyItems.mockResolvedValue({
      ok: true,
      items: [item({ learning: { samples: 4, needed: 10, newData: 4, source: 4, duration: 3, noData: false } })],
    });
    await renderPage();
    const window = await openMonitoring("plex");
    expect(
      within(window).getByText(en["anomaly.learningProgress"].replace("{n}", "4").replace("{needed}", "10"))
    ).toBeTruthy();
    expect(within(window).getByLabelText(en["anomaly.learningHint"].replace("{needed}", "10"))).toBeTruthy();
  });

  it("gives a dump and a dataset series the size line that has no new-data figure", async () => {
    getAnomalyItems.mockResolvedValue({
      ok: true,
      items: [
        item({
          dump: {
            part: "",
            learning: { samples: 10, needed: 10 },
            typical: { sourceBytes: 2 * 1024 ** 2, resticMs: 4000 },
            open: { critical: 0, warning: 0, info: 0 },
            retentionHeld: false,
          },
          datasets: [
            {
              part: "tank/media",
              learning: { samples: 10, needed: 10 },
              typical: { sourceBytes: 3 * GB, resticMs: 9000 },
              open: { critical: 0, warning: 0, info: 0 },
              retentionHeld: false,
            },
          ],
        }),
      ],
    });
    await renderPage();
    await openMonitoring("plex");

    expect(screen.getByText(en["anomaly.items.dumpSeries"])).toBeTruthy();
    expect(screen.getByText("tank/media").getAttribute("dir")).toBe("ltr");
    const sized = screen.getAllByText((text) => text.startsWith("Usually:") && text.includes("restic time"));
    for (const line of sized) expect(line.textContent).not.toContain("{");
    expect(
      screen.getByText(
        en["anomaly.items.typicalSize"].replace("{size}", isolateLtr("2.0 MB")).replace("{duration}", isolateLtr("4s"))
      )
    ).toBeTruthy();
  });

  it("gives a restic time under a second in milliseconds", async () => {
    getAnomalyItems.mockResolvedValue({
      ok: true,
      items: [
        item({
          dump: {
            part: "",
            learning: { samples: 10, needed: 10 },
            typical: { sourceBytes: 2 * 1024 ** 2, resticMs: 359 },
            open: { critical: 0, warning: 0, info: 0 },
            retentionHeld: false,
          },
        }),
      ],
    });
    await renderPage();
    await openMonitoring("plex");
    expect(
      screen.getByText(
        en["anomaly.items.typicalSize"].replace("{size}", isolateLtr("2.0 MB")).replace("{duration}", isolateLtr("359ms"))
      )
    ).toBeTruthy();
  });

  it("names the dataset or the dump an expectation belongs to", async () => {
    getAnomalyItems.mockResolvedValue({
      ok: true,
      items: [
        item({
          domain: "zfs",
          name: "tank",
          expectations: [
            { scopeKind: "zfsds", part: "tank/media", family: "source_bytes_down", sinceAt: SEEN, ceiling: 0, updatedAt: SEEN },
          ],
        }),
        item({
          targetId: "tg-2",
          name: "postgres",
          expectations: [
            { scopeKind: "dump", part: "", family: "dump_bytes_down", sinceAt: SEEN, ceiling: 0, updatedAt: SEEN },
          ],
        }),
      ],
    });
    await renderPage();

    const tank = await openMonitoring("tank");
    expect(within(tank).getByText(en["anomaly.markedExpected"])).toBeTruthy();
    expect(within(tank).getByText("tank/media").parentElement?.textContent).toContain(en["anomaly.family.sourceBytesDown"]);
    await press(within(tank).getByRole("button", { name: en["common.close"] }));

    const postgres = await openMonitoring("postgres");
    const dump = within(postgres).getByText(en["anomaly.items.dumpSeries"]);
    expect(dump.parentElement?.textContent).toContain(en["anomaly.family.dumpBytesDown"]);
  });

  it("offers to forget what was marked as expected", async () => {
    getAnomalyItems.mockResolvedValue({
      ok: true,
      items: [
        item({
          expectations: [
            { scopeKind: "item", part: "", family: "new_data", sinceAt: 0, ceiling: GB, updatedAt: SEEN },
          ],
        }),
      ],
    });
    await renderPage();
    await openMonitoring("plex");
    expect(
      screen.getByText(
        en["anomaly.expectation.ceiling"].replace("{family}", en["anomaly.family.newData"]).replace("{bytes}", "1.0 GB")
      )
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: en["anomaly.expectation.forget"] })).toBeTruthy();
  });
});

describe("with detection switched off", () => {
  beforeEach(() => getSettings.mockResolvedValue({ ok: true, settings: settings({ anomalyEnabled: false }) }));

  it("says so, keeps the earlier findings listed and leads to the switch", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex" })]);
    await renderPage();

    expect(screen.getByText(en["anomaly.offPage"])).toBeTruthy();
    expect(names()).toEqual(["plex"]);
    await press(screen.getByRole("button", { name: en["anomaly.openSettings"] }));
    expect(screen.getByTestId("where").textContent).toBe("/settings/integrity#anomalies");
  });

  it("shows an item's monitoring without the settings that would change nothing", async () => {
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
    await renderPage();
    const window = await openMonitoring("plex");
    expect(within(window).getByText(en["anomaly.learn.title"])).toBeTruthy();
    expect(within(window).queryByRole("combobox")).toBeNull();
  });
});

describe("before and without findings", () => {
  it("keeps quiet about an empty page while the first pass is still out", async () => {
    getAnomalySummary.mockReturnValue(new Promise<never>(() => undefined));
    await renderPage();
    expect(screen.getByText(en["dashboard.checking"])).toBeTruthy();
    expect(screen.queryByText(en["anomaly.emptyOpen"])).toBeNull();
    expect(screen.queryByRole("group", { name: en["anomaly.filter.severity"] })).toBeNull();
  });

  it("offers another try when the open findings were refused", async () => {
    getAnomalies.mockRejectedValueOnce(new Error("offline"));
    await renderPage();
    expect(screen.getByText(en["anomaly.loadFailed"])).toBeTruthy();
    expect(screen.queryByText(en["anomaly.emptyOpen"])).toBeNull();

    serve([finding({ targetId: "tg-plex", name: "plex" })]);
    await press(screen.getByRole("button", { name: en["anomaly.retry"] }));
    expect(names()).toEqual(["plex"]);
  });

  it("says there is nothing open", async () => {
    await renderPage();
    expect(screen.getByText(en["anomaly.emptyOpen"])).toBeTruthy();
    expect(tile(en["filter.all"]).textContent).toContain(en["anomaly.tile.open"].replace("{n}", "0"));
  });

  it("follows the cursor to the end, so no open finding is left out", async () => {
    const first = finding({ targetId: "tg-plex", name: "plex" });
    const second = finding({ targetId: "tg-sonarr", name: "sonarr" });
    getAnomalies.mockImplementation((f?: AnomalyFilter) =>
      f?.state === "open" ? (f.cursor ? page([second]) : page([first], "cur-2")) : page([])
    );
    await renderPage();
    expect(names()).toEqual(["plex", "sonarr"]);
  });

  it("says where sensitivity is set", async () => {
    await renderPage();
    const link = screen.getByRole("link", { name: `${en["nav.settings"]} › ${en["settings.tab.integrity"]}` });
    expect(link.getAttribute("href")).toBe("/settings/integrity#anomalies");
  });
});

it("draws no native select and no checkbox", async () => {
  getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
  serve([finding({ targetId: "tg-x", name: "sonarr" })]);
  await renderPage();
  await openMonitoring("plex");
  expect(screen.getByRole("combobox", { name: en["anomaly.items.sensitivity"] })).toBeTruthy();
  expect(document.querySelectorAll("select, input[type=checkbox]")).toHaveLength(0);
});

it("folds nothing out in place", async () => {
  getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
  serve([finding({ targetId: "tg-plex", name: "plex" })], [finding({ name: "old", state: "resolved", resolvedAt: SEEN })]);
  await renderPage();
  expect(findings()).toHaveLength(1);
  expect(ring("plex")).toBeTruthy();
  expect(document.querySelectorAll("[aria-expanded]")).toHaveLength(0);
  await showClosed();
  expect(document.querySelectorAll("[aria-expanded]")).toHaveLength(0);
});

// jsdom lays nothing out, so these pin the classes the phone layout rests on.
describe("at phone width", () => {
  it("gives a learning ring a finger-sized target", async () => {
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
    await renderPage();
    expect(ring("plex").className).toContain("min-h-11");
  });

  it("breaks a name without spaces in a finding and cuts it short under a ring", async () => {
    const name = "nextcloud_aio_nextcloud_database";
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item({ name })] });
    serve([finding({ targetId: "tg-plex", name })]);
    await renderPage();
    expect(within(findings()[0]!).getByRole("link", { name }).className).toContain("wrap-anywhere");
    expect(within(ring(name)).getByText(name).className).toContain("truncate");
  });

  it("sets the tiles two to a row and lets a finding's buttons take a row each side", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex" })]);
    await renderPage();
    expect(screen.getByRole("group", { name: en["anomaly.filter.severity"] }).className).toContain("grid-cols-2");
    const details = within(findings()[0]!).getByRole("button", { name: en["fleet.details"] });
    expect(details.closest("span.flex")?.className).toContain("max-sm:w-full");
  });
});
