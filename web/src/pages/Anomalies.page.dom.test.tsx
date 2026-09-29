// @vitest-environment jsdom
/**
 * The Anomalies page.
 *
 * A card stands for one item, so every finding has to land on the card of the
 * item it belongs to, and settling a card has to send that card's ids and no
 * others. What a card leaves out of sight (the figures, the actions, the
 * monitoring settings) has to be one press away.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

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
const getSettings = vi.mocked(api.getSettings);
const acknowledgeAnomalies = vi.mocked(api.acknowledgeAnomalies);
const markAnomaliesExpected = vi.mocked(api.markAnomaliesExpected);
const setItemAnomalyPrefs = vi.mocked(api.setItemAnomalyPrefs);

const { Anomalies } = await import("./Anomalies");

const GB = 1024 ** 3;

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
    lastRunAt: 1700000000,
    observed: 2 * GB,
    expected: 40 * GB,
    threshold: 20 * GB,
    samples: 12,
    sensitivity: "balanced",
    details: {},
    occurrences: 1,
    firstSeenAt: 1700000000,
    lastSeenAt: 1700000000,
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

async function renderPage(path = "/anomalies") {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <MemoryRouter initialEntries={[path]}>
            <AnomalyProvider>
              <Anomalies />
            </AnomalyProvider>
          </MemoryRouter>
        </ToastProvider>
      </I18nProvider>
    );
  });
}

/** Opens a SelectField by its accessible name and picks one of its options. */
async function pick(fieldLabel: string, optionLabel: string) {
  fireEvent.click(screen.getByRole("combobox", { name: fieldLabel }));
  await act(async () => {
    fireEvent.click(screen.getByRole("option", { name: optionLabel }));
  });
}

function card(name: string): HTMLElement {
  return screen.getByRole("region", { name });
}

async function openQuiet(name: string) {
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: new RegExp(`^${name}`) }));
  });
}

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
  getSettings.mockReset();
  acknowledgeAnomalies.mockReset();
  markAnomaliesExpected.mockReset();
  setItemAnomalyPrefs.mockReset();

  getAnomalySummary.mockResolvedValue({ ok: true, summary: summary() });
  getAnomalyItems.mockResolvedValue({ ok: true, items: [] });
  getSettings.mockResolvedValue({ ok: true, settings: settings() });
  serve([]);
});

afterEach(() => {
  cleanup();
  localStorage.removeItem("bv-lang");
});

describe("the cards", () => {
  it("gives every item one card holding its own findings, its dump's and its datasets'", async () => {
    serve([
      finding({ targetId: "tg-plex", name: "plex" }),
      finding({ targetId: "tg-plex", name: "plex", scopeKind: "dump", metric: "dump_bytes_growth", severity: "warning" }),
      finding({ targetId: "tg-sonarr", name: "sonarr", severity: "warning" }),
    ]);
    await renderPage();

    expect(screen.getAllByRole("region").map((r) => r.getAttribute("aria-label"))).toEqual(["plex", "sonarr"]);
    const lines = within(card("plex")).getAllByRole("button", { expanded: false });
    expect(lines.map((b) => b.textContent)).toEqual([
      expect.stringContaining(en["anomaly.short.shrink"].slice(0, 9)),
      expect.stringContaining(en["anomaly.items.dumpSeries"]),
    ]);
  });

  it("gives a restore check and a disk a card of their own", async () => {
    serve([
      finding({ metric: "drill_dr", detector: "integrity", scopeKind: "domain", scopeId: "files:offsite:t1", targetId: "", domain: "files", name: "", targetName: "wasabi" }),
      finding({ metric: "capacity_low", detector: "capacity", severity: "info", scopeKind: "volume", scopeId: "vol-1", targetId: "", domain: "container", name: "", observed: 0.04, details: { freeBytes: 30 * GB } }),
    ]);
    await renderPage();

    expect(within(card(en["anomaly.detector.integrity"])).getByText(/wasabi/)).toBeTruthy();
    expect(
      within(card(en["anomaly.detector.capacity"])).getByText(
        new RegExp(en["anomaly.card.volume"].replace("{domains}", "Containers"))
      )
    ).toBeTruthy();
  });

  it("puts the worst card first and tints only a critical one", async () => {
    serve([
      finding({ targetId: "tg-a", name: "calm", severity: "info" }),
      finding({ targetId: "tg-b", name: "loud", severity: "critical" }),
    ]);
    await renderPage();

    const [first, second] = screen.getAllByRole("region");
    expect(first.getAttribute("aria-label")).toBe("loud");
    expect(first.className).toContain("bg-statusFailBgSoft");
    expect(second.className).not.toContain("bg-statusFailBgSoft");
  });

  it("leaves the item's name out of the line and says the whole sentence once it is opened", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex" })]);
    await renderPage();

    const line = within(card("plex")).getByRole("button", { expanded: false });
    expect(line.textContent).not.toContain("plex");
    await act(async () => {
      fireEvent.click(line);
    });
    expect(line.getAttribute("aria-expanded")).toBe("true");
    expect(
      screen.getByText(
        en["anomaly.sentence.sourceShrink"]
          .replace("{name}", "plex")
          .replace("{current}", isolateLtr("2.0 GB"))
          .replace("{typical}", isolateLtr("40.0 GB"))
      )
    ).toBeTruthy();
  });

  it("names the dataset in front of a dataset's line", async () => {
    serve([
      finding({ targetId: "tg-tank", name: "tank", domain: "zfs", scopeKind: "zfsds", part: "tank/media" }),
    ]);
    await renderPage();
    const series = within(card("tank")).getByText(/^tank\/media/);
    expect(series.getAttribute("dir")).toBe("ltr");
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
    await act(async () => {
      fireEvent.click(
        within(card("plex")).getByRole("button", { name: en["anomaly.action.acknowledgeAll"].replace("{n}", "2") })
      );
    });

    expect(acknowledgeAnomalies).toHaveBeenCalledTimes(1);
    expect(acknowledgeAnomalies.mock.calls[0]![0]).toEqual(plex.map((a) => a.id));
    expect(heard).toHaveBeenCalled();
    window.removeEventListener(ANOMALY_CHANGED_EVENT, heard);
  });

  it("asks first when a finding of the card is keeping old backups", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex", severity: "warning", retentionHeld: true })]);
    acknowledgeAnomalies.mockResolvedValue({ ok: true, changed: 1, skipped: 0, released: 1 });
    await renderPage();

    fireEvent.click(within(card("plex")).getByRole("button", { name: en["anomaly.action.acknowledge"] }));
    expect(screen.getByText(en["anomaly.releaseConfirm"].replace("{name}", "plex"))).toBeTruthy();
    expect(acknowledgeAnomalies).not.toHaveBeenCalled();

    await act(async () => {
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["anomaly.action.acknowledge"] }));
    });
    expect(acknowledgeAnomalies).toHaveBeenCalledWith(["an-1"]);
  });

  it("settles one finding from its opened line, without a note", async () => {
    const rows = [
      finding({ targetId: "tg-plex", name: "plex", severity: "warning" }),
      finding({ targetId: "tg-plex", name: "plex", severity: "warning", metric: "source_bytes_growth" }),
    ];
    serve(rows);
    markAnomaliesExpected.mockResolvedValue({ ok: true, changed: 1, skipped: 0, released: 0 });
    await renderPage();

    const plex = card("plex");
    await act(async () => {
      fireEvent.click(within(plex).getAllByRole("button", { expanded: false })[1]!);
    });
    await act(async () => {
      fireEvent.click(within(plex).getByRole("button", { name: en["anomaly.action.expected"] }));
    });
    expect(markAnomaliesExpected).toHaveBeenCalledWith([rows[1]!.id]);
    expect(screen.queryByRole("textbox")).toBeNull();
  });

  it("makes restoring the card's main action for a critical finding with a good backup", async () => {
    serve([
      finding({
        targetId: "tg-plex",
        name: "plex",
        lastGood: { runId: "run-9", snapshotId: "snap-9", at: 1700000000 },
      }),
    ]);
    await renderPage();
    const label = en["anomaly.action.restoreLastGood"].replace(
      "{date}",
      isolateLtr(new Date(1700000000 * 1000).toLocaleString())
    );
    expect(within(card("plex")).getByRole("button", { name: label })).toBeTruthy();
    expect(within(card("plex")).queryByRole("button", { name: en["anomaly.action.acknowledge"] })).toBeNull();
  });

  it("links a warning's last good backup from its opened line", async () => {
    serve([
      finding({
        targetId: "tg-plex",
        name: "plex",
        severity: "warning",
        lastGood: { runId: "run-9", snapshotId: "snap-9", at: 1700000000 },
      }),
    ]);
    await renderPage();
    await act(async () => {
      fireEvent.click(within(card("plex")).getByRole("button", { expanded: false }));
    });
    const link = screen.getByRole("link", {
      name: en["anomaly.action.restoreLastGood"].replace("{date}", isolateLtr(new Date(1700000000 * 1000).toLocaleString())),
    });
    expect(link.getAttribute("href")).toBe("/containers?restore=snap-9&at=1700000000&item=plex");
  });

  it("links a dataset's last good backup at its ZFS item, naming the dataset", async () => {
    serve([
      finding({
        targetId: "tg-tank",
        severity: "warning",
        domain: "zfs",
        name: "tank/media",
        scopeKind: "zfsds",
        scopeId: "tank/media/photos",
        part: "tank/media/photos",
        lastGood: { runId: "run-9", snapshotId: "snap-9", at: 1700000000 },
      }),
    ]);
    await renderPage();
    await act(async () => {
      fireEvent.click(within(card("tank/media")).getByRole("button", { expanded: false }));
    });
    const link = screen.getByRole("link", {
      name: en["anomaly.action.restoreLastGood"].replace("{date}", isolateLtr(new Date(1700000000 * 1000).toLocaleString())),
    });
    expect(link.getAttribute("href")).toBe("/zfs?restore=snap-9&at=1700000000&item=tank%2Fmedia&dataset=tank%2Fmedia%2Fphotos");
  });
});

describe("a finding's figures", () => {
  function figure(label: string, value: string) {
    return en["anomaly.figure"].replace("{label}", label).replace("{value}", value);
  }

  async function openOnly() {
    await act(async () => {
      fireEvent.click(screen.getAllByRole("region")[0]!.querySelector("button[aria-expanded]")!);
    });
  }

  // The detector measures a rate per second; the bubble has to say per hour.
  it("reads the largest usual amount per hour out of a per-second rate", async () => {
    serve([finding({ metric: "new_data", detector: "new_data", details: { refRate: 1024 } })]);
    await renderPage();
    await openOnly();
    const bubble = screen.getByLabelText(new RegExp(en["anomaly.detail.refRate"]));
    expect(bubble.getAttribute("aria-label")).toContain(figure(en["anomaly.detail.refRate"], isolateLtr("3.5 MB")));
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
    await openOnly();
    const tip = screen.getByLabelText(new RegExp(en["anomaly.detail.etaGrowth"])).getAttribute("aria-label");
    expect(tip).toContain(figure(en["anomaly.detail.etaGrowth"], "12 days"));
    expect(tip).toContain(figure(en["anomaly.detail.etaFree"], "6 days"));
    expect(tip).toContain(figure(en["anomaly.detail.slope"], isolateLtr("2.0 GB")));
  });

  it("shows the measured and the usual level beside the line", async () => {
    serve([finding({})]);
    await renderPage();
    await openOnly();
    expect(screen.getByText(en["anomaly.detail.observed"]).nextElementSibling?.textContent).toBe(isolateLtr("2.0 GB"));
    expect(screen.getByText(en["anomaly.detail.expected"]).nextElementSibling?.textContent).toBe(isolateLtr("40.0 GB"));
  });
});

describe("the severity tiles", () => {
  it("count the open findings and hide a severity until pressed again", async () => {
    serve([
      finding({ targetId: "tg-a", name: "alpha", severity: "critical" }),
      finding({ targetId: "tg-b", name: "beta", severity: "warning" }),
      finding({ targetId: "tg-b", name: "beta", severity: "warning" }),
    ]);
    await renderPage();

    const tiles = screen.getByRole("group", { name: en["anomaly.filter.severity"] });
    const warnings = within(tiles).getByRole("button", { name: new RegExp(en["anomaly.tile.warning"]) });
    expect(warnings.textContent).toContain("2");
    expect(warnings.getAttribute("aria-pressed")).toBe("true");

    fireEvent.click(warnings);
    expect(warnings.getAttribute("aria-pressed")).toBe("false");
    expect(screen.queryByRole("region", { name: "beta" })).toBeNull();
    expect(card("alpha")).toBeTruthy();

    fireEvent.click(warnings);
    expect(card("beta")).toBeTruthy();
  });

  it("say so when every open finding is hidden", async () => {
    serve([finding({ severity: "info" })]);
    await renderPage();
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["anomaly.tile.info"]) }));
    expect(screen.getByText(en["anomaly.emptyHidden"])).toBeTruthy();
  });
});

describe("the closed findings", () => {
  it("counts the last 30 days and shows a stored note once a finding is opened", async () => {
    serve([], [
      finding({ name: "plex", state: "acknowledged", ackedAt: 1700000000, ackNote: "cleaned up on purpose" }),
      finding({ name: "sonarr", state: "resolved", resolvedAt: 1700000000 }),
    ]);
    await renderPage();

    const since = getAnomalies.mock.calls.find(([f]) => f?.state === "closed")![0]!.since!;
    expect(Math.abs(since - (Date.now() / 1000 - 30 * 86400))).toBeLessThan(60);

    const row = screen.getByRole("button", { name: new RegExp(en["anomaly.closedRecent"]) });
    expect(row.textContent).toContain("2");
    fireEvent.click(row);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /^plex:/ }));
    });
    expect(screen.getByText(`${en["anomaly.noteLabel"]}: cleaned up on purpose`)).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["anomaly.action.acknowledge"] })).toBeNull();
  });

  it("says when nothing was closed", async () => {
    await renderPage();
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["anomaly.closedRecent"]) }));
    expect(screen.getByText(en["anomaly.emptyClosed"])).toBeTruthy();
  });
});

describe("a link to one item", () => {
  it("opens and scrolls to the card of that item", async () => {
    serve([
      finding({ targetId: "tg-a", name: "alpha" }),
      finding({ targetId: "tg-b", name: "beta", severity: "warning" }),
    ]);
    await renderPage("/anomalies?scope=item:tg-b");

    expect(within(card("beta")).getByRole("button", { expanded: true })).toBeTruthy();
    expect(within(card("alpha")).queryByRole("button", { expanded: true })).toBeNull();
    expect(scrolled).toHaveBeenCalledTimes(1);
    expect(scrolled.mock.contexts[0]).toBe(card("beta"));
  });

  it("opens the monitoring of an item without findings", async () => {
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item(), item({ targetId: "tg-2", name: "sonarr" })] });
    await renderPage("/anomalies?scope=item:tg-2");
    expect(screen.getByRole("button", { name: /^sonarr/ }).getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByRole("button", { name: /^plex/ }).getAttribute("aria-expanded")).toBe("false");
  });
});

describe("monitoring", () => {
  it("opens an item's settings from its card", async () => {
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
    serve([finding({ targetId: "tg-plex", name: "plex" })]);
    await renderPage();

    const toggle = within(card("plex")).getByRole("button", { name: en["anomaly.card.monitoring"] });
    await act(async () => {
      fireEvent.click(toggle);
    });
    expect(within(card("plex")).getByRole("combobox", { name: en["anomaly.items.sensitivity"] })).toBeTruthy();
    expect(screen.queryByText(`${en["anomaly.quiet"]} · 1`)).toBeNull();
  });

  it("saves a sensitivity straight away and puts it back when the server refuses", async () => {
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
    setItemAnomalyPrefs.mockResolvedValue({ ok: false, error: "no", code: "bad-request" });
    await renderPage();
    await openQuiet("plex");

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
    await openQuiet("plex");

    await pick(en["anomaly.items.notifyMin"], en["anomaly.settings.notify.critical"]);
    expect(setItemAnomalyPrefs).toHaveBeenCalledWith("tg-plex", { notifyMin: "critical" });
  });

  it("lists the items without findings in one card and marks one that is not scheduled", async () => {
    getAnomalyItems.mockResolvedValue({
      ok: true,
      items: [
        item(),
        item({ targetId: "tg-2", name: "sonarr", scheduled: false }),
        item({ targetId: "tg-3", name: "radarr" }),
      ],
    });
    serve([finding({ targetId: "tg-3", name: "radarr" })]);
    await renderPage();

    expect(screen.getByText(`${en["anomaly.quiet"]} · 2`)).toBeTruthy();
    expect(screen.getByRole("button", { name: /^sonarr/ }).textContent).toContain(en["anomaly.items.notScheduled"]);
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
    await openQuiet("plex");

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
    await openQuiet("plex");
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
            { scopeKind: "zfsds", part: "tank/media", family: "source_bytes_down", sinceAt: 1700000000, ceiling: 0, updatedAt: 1700000000 },
          ],
        }),
        item({
          targetId: "tg-2",
          name: "postgres",
          expectations: [
            { scopeKind: "dump", part: "", family: "dump_bytes_down", sinceAt: 1700000000, ceiling: 0, updatedAt: 1700000000 },
          ],
        }),
      ],
    });
    await renderPage();
    await openQuiet("tank");
    await openQuiet("postgres");
    const dataset = screen.getByText("tank/media");
    expect(dataset.parentElement?.textContent).toContain(en["anomaly.family.sourceBytesDown"]);
    const dump = screen.getByText(en["anomaly.items.dumpSeries"]);
    expect(dump.parentElement?.textContent).toContain(en["anomaly.family.dumpBytesDown"]);
  });

  it("offers to forget what was marked as expected", async () => {
    getAnomalyItems.mockResolvedValue({
      ok: true,
      items: [
        item({
          expectations: [
            { scopeKind: "item", part: "", family: "new_data", sinceAt: 0, ceiling: GB, updatedAt: 1700000000 },
          ],
        }),
      ],
    });
    await renderPage();
    await openQuiet("plex");
    expect(
      screen.getByText(
        en["anomaly.expectation.ceiling"].replace("{family}", en["anomaly.family.newData"]).replace("{bytes}", "1.0 GB")
      )
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: en["anomaly.expectation.forget"] })).toBeTruthy();
  });

  it("offers another try instead of an empty list when the items were refused", async () => {
    getAnomalyItems.mockRejectedValueOnce(new Error("offline"));
    await renderPage();
    expect(screen.getByText(en["anomaly.loadFailed"])).toBeTruthy();

    getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["anomaly.retry"] }));
    });
    expect(screen.getByRole("button", { name: /^plex/ })).toBeTruthy();
  });
});

describe("before and without findings", () => {
  it("keeps quiet about an empty page while the first pass is still out", async () => {
    getAnomalySummary.mockReturnValue(new Promise<never>(() => undefined));
    await renderPage();
    expect(screen.getByText(en["dashboard.checking"])).toBeTruthy();
    expect(screen.queryByText(en["anomaly.emptyOpen"])).toBeNull();
  });

  it("offers another try when the open findings were refused", async () => {
    getAnomalies.mockRejectedValueOnce(new Error("offline"));
    await renderPage();
    expect(screen.getByText(en["anomaly.loadFailed"])).toBeTruthy();
    expect(screen.queryByText(en["anomaly.emptyOpen"])).toBeNull();

    serve([finding({ targetId: "tg-plex", name: "plex" })]);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["anomaly.retry"] }));
    });
    expect(card("plex")).toBeTruthy();
  });

  it("says there is nothing open", async () => {
    await renderPage();
    expect(screen.getByText(en["anomaly.emptyOpen"])).toBeTruthy();
  });

  it("follows the cursor to the end, so no open finding is left off a card", async () => {
    const first = finding({ targetId: "tg-plex", name: "plex" });
    const second = finding({ targetId: "tg-sonarr", name: "sonarr" });
    getAnomalies.mockImplementation((f?: AnomalyFilter) =>
      f?.state === "open" ? (f.cursor ? page([second]) : page([first], "cur-2")) : page([])
    );
    await renderPage();
    expect(card("plex")).toBeTruthy();
    expect(card("sonarr")).toBeTruthy();
  });
});

it("draws no native select and no checkbox", async () => {
  getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
  serve([finding({ targetId: "tg-x", name: "sonarr" })]);
  await renderPage();
  await openQuiet("plex");
  expect(screen.getByRole("combobox", { name: en["anomaly.items.sensitivity"] })).toBeTruthy();
  expect(document.querySelectorAll("select, input[type=checkbox]")).toHaveLength(0);
});

// jsdom lays nothing out, so these pin the classes the phone layout rests on.
describe("at phone width", () => {
  it("gives a finding's line a finger-sized target", async () => {
    serve([finding({ targetId: "tg-plex", name: "plex" })]);
    await renderPage();
    expect(within(card("plex")).getByRole("button", { expanded: false }).className).toContain("min-h-11");
  });

  it("breaks an item name without spaces instead of running out of the card", async () => {
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item({ name: "nextcloud_aio_nextcloud_database" })] });
    await renderPage();
    const name = screen.getByText("nextcloud_aio_nextcloud_database", { selector: "span" });
    expect(name.className).toContain("wrap-anywhere");
  });
});
