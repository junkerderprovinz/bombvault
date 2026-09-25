// @vitest-environment jsdom
/**
 * The Anomalies page.
 *
 * Two things have to hold here. A filter has to reach the server as the query
 * the server understands, because a listing that quietly drops a narrowing
 * reads as an all-clear. And a bulk action has to be one call with every id in
 * it, asked for first when it would let old backups be deleted again.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import { AnomalyProvider } from "../lib/useAnomalies";
import { ANOMALY_CHANGED_EVENT } from "../lib/anomalies";
import type { AnomalyItem, AnomalySummary, AnomalyView, Settings } from "../lib/api";

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

const DAY = 86400;

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
    observed: 1024,
    expected: 1024 ** 3,
    threshold: 1024 ** 2,
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
    typical: { sourceBytes: 1024 ** 3, newDataBytes: 1024 ** 2, resticMs: 30000 },
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

beforeEach(() => {
  seq = 0;
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  window.location.hash = "";
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
  getAnomalies.mockImplementation(() => page([]));
});

afterEach(() => {
  cleanup();
  localStorage.removeItem("bv-lang");
});

describe("the findings tab's filters", () => {
  it("asks the server for the period the reader picked", async () => {
    await renderPage();
    await pick(en["anomaly.filter.state"], en["anomaly.filter.stateClosed"]);
    await pick(en["anomaly.filter.period"], en["anomaly.filter.period90"]);

    const now = Math.floor(Date.now() / 1000);
    const last = getAnomalies.mock.calls.at(-1)![0]!;
    expect(last.state).toBe("closed");
    expect(last.since).toBeGreaterThan(now - 90 * DAY - 60);
    expect(last.since).toBeLessThanOrEqual(now - 90 * DAY + 60);

    await pick(en["anomaly.filter.period"], en["anomaly.filter.periodAll"]);
    expect(getAnomalies.mock.calls.at(-1)![0]!.since).toBe(0);
  });

  it("leaves the period out while only open findings are listed", async () => {
    await renderPage();
    await pick(en["anomaly.filter.severity"], en["anomaly.severity.warning"]);

    const last = getAnomalies.mock.calls.at(-1)![0]!;
    expect(last.state).toBe("open");
    expect(last.severity).toBe("warning");
    expect(last.since).toBeUndefined();
    expect(screen.queryByRole("combobox", { name: en["anomaly.filter.period"] })).toBeNull();
  });

  it("narrows to the findings about ZFS datasets", async () => {
    await renderPage();
    await pick(en["common.domain"], en["dashboard.domainZFS"]);
    expect(getAnomalies.mock.calls.at(-1)![0]!.domain).toBe("zfs");
  });

  it("lists the domains in the sidebar's order", async () => {
    await renderPage();
    fireEvent.click(screen.getByRole("combobox", { name: en["common.domain"] }));
    const options = screen.getAllByRole("option").map((o) => o.textContent);
    const sidebar = ["nav.containers", "nav.vms", "nav.flash", "nav.files", "nav.zfs", "nav.config"] as const;
    expect(options.filter((o) => sidebar.some((k) => en[k] === o))).toEqual(sidebar.map((k) => en[k]));
  });

  it("narrows to one item from the query string and lets that go again", async () => {
    getAnomalies.mockImplementation(() => page([finding({ targetId: "tg-plex", name: "plex" })]));
    await renderPage("/anomalies?scope=item:tg-plex");
    expect(getAnomalies.mock.calls[0]![0]!.scope).toBe("item:tg-plex");
    expect(screen.getByText(en["anomaly.filter.itemChip"].replace("{name}", "plex"))).toBeTruthy();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["anomaly.filter.removeChip"] }));
    });
    expect(getAnomalies.mock.calls.at(-1)![0]!.scope).toBeUndefined();
  });
});

describe("the findings tab's empty states", () => {
  it.each([
    [en["anomaly.filter.stateOpen"], "anomaly.emptyOpen"],
    [en["anomaly.filter.stateClosed"], "anomaly.emptyClosed"],
    [en["anomaly.filter.any"], "anomaly.emptyAny"],
  ])("says what %s found nothing means", async (stateLabel, key) => {
    await renderPage();
    await pick(en["anomaly.filter.state"], stateLabel);
    expect(screen.getByText(en[key as keyof typeof en])).toBeTruthy();
  });

  it("says a narrowed listing is empty because of the narrowing", async () => {
    await renderPage();
    await pick(en["anomaly.filter.detector"], en["anomaly.detector.capacity"]);
    expect(screen.getByText(en["anomaly.emptyFiltered"])).toBeTruthy();
    expect(screen.queryByText(en["anomaly.emptyOpen"])).toBeNull();
  });

  it("offers another try instead of an empty state when the listing failed", async () => {
    getAnomalies.mockImplementationOnce(() => Promise.reject(new Error("offline")));
    await renderPage();
    expect(screen.getByText(en["anomaly.loadFailed"])).toBeTruthy();
    expect(screen.queryByText(en["anomaly.emptyOpen"])).toBeNull();

    getAnomalies.mockImplementation(() => page([finding({ name: "plex" })]));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["anomaly.retry"] }));
    });
    expect(screen.queryByText(en["anomaly.loadFailed"])).toBeNull();
  });
});

describe("settling several findings at once", () => {
  it("sends every selected id in one call and tells the rest of the app", async () => {
    const rows = [finding({ name: "plex" }), finding({ name: "sonarr" })];
    getAnomalies.mockImplementation(() => page(rows));
    acknowledgeAnomalies.mockResolvedValue({ ok: true, changed: 2, skipped: 0, released: 0 });
    const heard = vi.fn();
    window.addEventListener(ANOMALY_CHANGED_EVENT, heard);

    await renderPage();
    fireEvent.click(screen.getByRole("checkbox", { name: en["common.selectItem"].replace("{name}", "plex") }));
    fireEvent.click(screen.getByRole("checkbox", { name: en["common.selectItem"].replace("{name}", "sonarr") }));
    expect(screen.getByText(en["anomaly.bulk.selected"].replace("{n}", "2"))).toBeTruthy();

    const bar = screen.getByRole("group", { name: en["anomaly.bulk.selected"].replace("{n}", "2") });
    await act(async () => {
      fireEvent.click(within(bar).getByRole("button", { name: en["anomaly.action.acknowledge"] }));
    });

    expect(acknowledgeAnomalies).toHaveBeenCalledTimes(1);
    expect(acknowledgeAnomalies.mock.calls[0]![0]).toEqual([rows[0]!.id, rows[1]!.id]);
    expect(heard).toHaveBeenCalled();
    window.removeEventListener(ANOMALY_CHANGED_EVENT, heard);
  });

  it("asks first when a selected finding is keeping old backups", async () => {
    const rows = [finding({ name: "plex", retentionHeld: true })];
    getAnomalies.mockImplementation(() => page(rows));
    acknowledgeAnomalies.mockResolvedValue({ ok: true, changed: 1, skipped: 0, released: 1 });

    await renderPage();
    fireEvent.click(screen.getByRole("checkbox", { name: en["common.selectItem"].replace("{name}", "plex") }));
    const bar = screen.getByRole("group", { name: en["anomaly.bulk.selected"].replace("{n}", "1") });
    fireEvent.click(within(bar).getByRole("button", { name: en["anomaly.action.acknowledge"] }));

    expect(
      screen.getByText(en["anomaly.releaseConfirmMany"].replace("{names}", "plex"))
    ).toBeTruthy();
    expect(acknowledgeAnomalies).not.toHaveBeenCalled();

    const dialog = screen.getByRole("dialog");
    await act(async () => {
      fireEvent.click(within(dialog).getByRole("button", { name: en["anomaly.action.acknowledge"] }));
    });
    expect(acknowledgeAnomalies).toHaveBeenCalledTimes(1);
  });

  it("keeps the expected button shut while nothing selected can be expected", async () => {
    getAnomalies.mockImplementation(() => page([finding({ name: "plex", expectable: false })]));
    await renderPage();
    fireEvent.click(screen.getByRole("checkbox", { name: en["common.selectItem"].replace("{name}", "plex") }));

    const bar = screen.getByRole("group", { name: en["anomaly.bulk.selected"].replace("{n}", "1") });
    const expected = within(bar).getByRole("button", { name: en["anomaly.action.expected"] });
    expect((expected as HTMLButtonElement).disabled).toBe(true);
    expect(markAnomaliesExpected).not.toHaveBeenCalled();
  });

  it("says that a selection covers the loaded entries only while a page is left", async () => {
    getAnomalies.mockImplementation(() => page([finding({ name: "plex" })], "cur-2"));
    await renderPage();
    expect(screen.getByText(en["anomaly.bulk.loadedOnly"])).toBeTruthy();
    expect(screen.getByRole("button", { name: en["anomaly.loadMore"] })).toBeTruthy();
  });
});

describe("a finding's own lines", () => {
  it("links the last good backup at the item's restore panel", async () => {
    getAnomalies.mockImplementation(() =>
      page([
        finding({
          name: "plex",
          lastGood: { runId: "run-9", snapshotId: "snap-9", at: 1700000000 },
        }),
      ])
    );
    await renderPage();
    const link = screen.getByRole("link", {
      name: en["anomaly.action.restoreLastGood"].replace("{date}", new Date(1700000000 * 1000).toLocaleString()),
    });
    expect(link.getAttribute("href")).toBe("/containers?restore=snap-9&at=1700000000&item=plex");
  });

  it("links a dataset's last good backup at its ZFS item, naming the dataset", async () => {
    getAnomalies.mockImplementation(() =>
      page([
        finding({
          domain: "zfs",
          name: "tank/media",
          scopeKind: "zfsds",
          scopeId: "tank/media/photos",
          part: "tank/media/photos",
          lastGood: { runId: "run-9", snapshotId: "snap-9", at: 1700000000 },
        }),
      ])
    );
    await renderPage();
    const link = screen.getByRole("link", {
      name: en["anomaly.action.restoreLastGood"].replace("{date}", new Date(1700000000 * 1000).toLocaleString()),
    });
    expect(link.getAttribute("href")).toBe("/zfs?restore=snap-9&at=1700000000&item=tank%2Fmedia&dataset=tank%2Fmedia%2Fphotos");
  });

  // The detector measures a rate per second; the row has to say per hour.
  it("reads the largest usual amount per hour out of a per-second rate", async () => {
    getAnomalies.mockImplementation(() =>
      page([finding({ metric: "new_data", detector: "new_data", details: { refRate: 1024 } })])
    );
    await renderPage();
    fireEvent.click(screen.getByRole("button", { name: en["anomaly.action.details"] }));
    const term = screen.getByText(en["anomaly.detail.refRate"]);
    expect(term.nextElementSibling?.textContent).toBe("3.5 MB");
  });

  // The two projections are independent: one is what BombVault itself writes,
  // the other is everything filling the same disk.
  it("says where a capacity projection comes from", async () => {
    getAnomalies.mockImplementation(() =>
      page([
        finding({
          metric: "capacity_eta",
          detector: "capacity",
          scopeKind: "volume",
          observed: 6,
          details: { etaGrowthDays: 12, etaFreeDays: 6, slopePerDay: 2 * 1024 ** 3 },
        }),
      ])
    );
    await renderPage();
    fireEvent.click(screen.getByRole("button", { name: en["anomaly.action.details"] }));
    const value = (label: string) => screen.getByText(label).nextElementSibling?.textContent;
    expect(value(en["anomaly.detail.etaGrowth"])).toBe("12 days");
    expect(value(en["anomaly.detail.etaFree"])).toBe("6 days");
    expect(value(en["anomaly.detail.slope"])).toBe("2.0 GB");
  });
});

describe("the items tab", () => {
  async function openItems() {
    await act(async () => {
      fireEvent.click(screen.getByRole("tab", { name: en["anomaly.tab.items"] }));
    });
  }

  it("saves a sensitivity straight away and puts it back when the server refuses", async () => {
    getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
    setItemAnomalyPrefs.mockResolvedValue({ ok: false, error: "no", code: "bad-request" });
    await renderPage();
    await openItems();

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
    await openItems();

    await pick(en["anomaly.items.notifyMin"], en["anomaly.settings.notify.critical"]);
    expect(setItemAnomalyPrefs).toHaveBeenCalledWith("tg-plex", { notifyMin: "critical" });
  });

  it("says an item is not scheduled instead of how much it has learned", async () => {
    getAnomalyItems.mockResolvedValue({
      ok: true,
      items: [item({ scheduled: false, learning: { samples: 0, needed: 10, newData: 0, source: 0, duration: 0, noData: false } })],
    });
    await renderPage();
    await openItems();
    expect(screen.getByText(en["anomaly.items.notScheduled"])).toBeTruthy();
    expect(screen.queryByText(en["anomaly.learning"].replace("{n}", "0").replace("{needed}", "10"))).toBeNull();
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
              typical: { sourceBytes: 3 * 1024 ** 3, resticMs: 9000 },
              open: { critical: 0, warning: 0, info: 0 },
              retentionHeld: false,
            },
          ],
        }),
      ],
    });
    await renderPage();
    await openItems();

    expect(screen.getByText(en["anomaly.items.dumpSeries"])).toBeTruthy();
    expect(screen.getByText("tank/media")).toBeTruthy();
    const sized = screen.getAllByText((text) => text.startsWith("Usually:") && text.includes("restic time"));
    for (const line of sized) expect(line.textContent).not.toContain("{");
    expect(
      screen.getByText(
        en["anomaly.items.typicalSize"].replace("{size}", "2.0 MB").replace("{duration}", "4s")
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
    await openItems();
    expect(
      screen.getByText(
        en["anomaly.items.typicalSize"].replace("{size}", "2.0 MB").replace("{duration}", "359ms")
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
    await openItems();
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
            { scopeKind: "item", part: "", family: "new_data", sinceAt: 0, ceiling: 1024 ** 3, updatedAt: 1700000000 },
          ],
        }),
      ],
    });
    await renderPage();
    await openItems();
    expect(
      screen.getByText(
        en["anomaly.expectation.ceiling"]
          .replace("{family}", en["anomaly.family.newData"])
          .replace("{bytes}", "1.0 GB")
      )
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: en["anomaly.expectation.forget"] })).toBeTruthy();
  });

  it("says when there is nothing to list", async () => {
    await renderPage();
    await openItems();
    expect(screen.getByText(en["anomaly.items.empty"])).toBeTruthy();
  });

  it("offers another try instead of an empty list when the items were refused", async () => {
    getAnomalyItems.mockRejectedValueOnce(new Error("offline"));
    await renderPage();
    await openItems();
    expect(screen.getByText(en["anomaly.loadFailed"])).toBeTruthy();
    expect(screen.queryByText(en["anomaly.items.empty"])).toBeNull();

    getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["anomaly.retry"] }));
    });
    expect(screen.getByText("plex")).toBeTruthy();
  });

  it("keeps quiet about an empty list while the first pass is still out", async () => {
    getAnomalySummary.mockReturnValue(new Promise<never>(() => undefined));
    await renderPage("/anomalies#items");
    expect(screen.getByText(en["dashboard.checking"])).toBeTruthy();
    expect(screen.queryByText(en["anomaly.items.empty"])).toBeNull();
  });

  it("says the list is unreadable when the pass behind it was refused", async () => {
    getAnomalySummary.mockResolvedValue({ ok: false, error: "no session", summary: summary() });
    await renderPage("/anomalies#items");
    expect(screen.getByText(en["anomaly.loadFailed"])).toBeTruthy();
    expect(screen.queryByText(en["anomaly.items.empty"])).toBeNull();
    expect(getAnomalyItems).not.toHaveBeenCalled();
  });

  it("groups a four-digit open count in the badge's label", async () => {
    getAnomalyItems.mockResolvedValue({
      ok: true,
      items: [item({ open: { critical: 0, warning: 1200, info: 0 } })],
    });
    await renderPage();
    await openItems();
    const label = en["anomaly.itemBadgeAria"]
      .replace("{name}", "plex")
      .replace("{n}", (1200).toLocaleString());
    expect(screen.getByRole("link", { name: label })).toBeTruthy();
  });
});

it("draws no native select on either tab", async () => {
  getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
  getAnomalies.mockImplementation(() => page([finding({ name: "plex" })]));
  const { container } = render(
    <I18nProvider>
      <ToastProvider>
        <MemoryRouter initialEntries={["/anomalies"]}>
          <AnomalyProvider>
            <Anomalies />
          </AnomalyProvider>
        </MemoryRouter>
      </ToastProvider>
    </I18nProvider>
  );
  await act(async () => undefined);
  expect(screen.getByRole("combobox", { name: en["anomaly.filter.state"] })).toBeTruthy();
  expect(container.querySelectorAll("select")).toHaveLength(0);

  await act(async () => {
    fireEvent.click(screen.getByRole("tab", { name: en["anomaly.tab.items"] }));
  });
  expect(screen.getByRole("combobox", { name: en["anomaly.items.sensitivity"] })).toBeTruthy();
  expect(container.querySelectorAll("select")).toHaveLength(0);
});
