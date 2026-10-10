// @vitest-environment jsdom
// The curve of what an item's backups measured. It shows the usual range as a
// band once detection knows one, the bare points while it is still learning,
// and the run a finding was raised on.
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider, en } from "../../lib/i18n";
import { isolateLtr } from "../../lib/ltrFragments";
import type { AnomalyQuantity, AnomalySeries, AnomalyView } from "../../lib/api";

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return { ...actual, getAnomalyItemSeries: vi.fn() };
});

const api = await import("../../lib/api");
const getAnomalyItemSeries = vi.mocked(api.getAnomalyItemSeries);
const { SeriesCurve } = await import("./SeriesCurve");

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as never;
const MB = 1024 ** 2;
const DAY = 86400;

function quantity(over: Partial<AnomalyQuantity> = {}, values = [20, 22, 19, 21, 1200]): AnomalyQuantity {
  return {
    quantity: "newDataBytes",
    points: values.map((value, i) => ({ runId: `run-${i}`, at: 1700000000 + i * DAY, value: value * MB })),
    learning: false,
    samples: 10,
    needed: 10,
    band: { low: null, high: { metric: "new_data", expected: 22 * MB, threshold: 40 * MB, samples: 10 } },
    ...over,
  };
}

function finding(over: Partial<AnomalyView> = {}): AnomalyView {
  return {
    id: "an-1",
    metric: "new_data",
    severity: "warning",
    state: "open",
    scopeKind: "item",
    part: "",
    targetId: "tg-1",
    domain: "container",
    name: "nextcloud",
    runId: "run-4",
    observed: 1200 * MB,
    expected: 22 * MB,
    recoveredAt: 0,
    details: { refBytes: 40 * MB },
    ...over,
  } as AnomalyView;
}

function serve(...series: AnomalySeries[]) {
  getAnomalyItemSeries.mockResolvedValue({ ok: true, series });
}

async function show(a?: AnomalyView) {
  await act(async () => {
    render(
      <I18nProvider>
        <SeriesCurve targetId="tg-1" finding={a} t={t} />
      </I18nProvider>
    );
  });
}

function parts(name: string): Element[] {
  return [...document.querySelectorAll(`[data-part="${name}"]`)];
}

function usualRange(range: string): string {
  return en["anomaly.figure"].replace("{label}", en["anomaly.curve.range"]).replace("{value}", range);
}

beforeEach(() => {
  localStorage.setItem("bv-lang", "en");
  getAnomalyItemSeries.mockReset();
});

afterEach(() => {
  cleanup();
  localStorage.removeItem("bv-lang");
});

describe("with a usual range", () => {
  beforeEach(() => serve({ scopeKind: "item", part: "", quantities: [quantity()], failed: [] }));

  it("draws the range as a band and names its edge", async () => {
    await show(finding());
    expect(getAnomalyItemSeries).toHaveBeenCalledWith("tg-1");
    expect(parts("band")).toHaveLength(1);
    expect(screen.getByText(usualRange(en["anomaly.curve.upTo"].replace("{value}", isolateLtr("40.0 MB"))))).toBeTruthy();
    expect(screen.getByText(en["anomaly.curve.newData"])).toBeTruthy();
  });

  it("marks the finding's run and prints how far it stood from the usual", async () => {
    await show(finding());
    expect(parts("marked")).toHaveLength(1);
    expect(parts("jump")).toHaveLength(1);
    expect(screen.getByText(isolateLtr(en["anomaly.curve.times"].replace("{n}", "30")))).toBeTruthy();
  });

  it("draws the item's size and marks nothing without a finding", async () => {
    serve({ scopeKind: "item", part: "", quantities: [quantity(), quantity({ quantity: "sourceBytes" })], failed: [] });
    await show();
    expect(screen.getByText(en["anomaly.curve.sourceBytes"])).toBeTruthy();
    expect(parts("line")).toHaveLength(1);
    expect(parts("marked")).toHaveLength(0);
    expect(parts("jump")).toHaveLength(0);
  });

  it("calls the band the usual range and does not promise a finding outside it", async () => {
    await show(finding());
    const hint = screen.getByLabelText(en["anomaly.curve.hint"]);
    expect(hint).toBeTruthy();
    expect(en["anomaly.curve.hint"]).not.toMatch(/reports an anomaly/);
  });

  it("says so when the cause of an open finding has gone away", async () => {
    await show(finding({ severity: "critical", recoveredAt: 1700400000 }));
    expect(screen.getByText(en["anomaly.curve.settled"])).toBeTruthy();
    expect(parts("jump")).toHaveLength(0);
  });
});

describe("while an item is learning", () => {
  beforeEach(() =>
    serve({
      scopeKind: "item",
      part: "",
      quantities: [quantity({ quantity: "sourceBytes", learning: true, samples: 4, band: null }, [20, 22, 19, 21])],
      failed: [],
    })
  );

  it("shows the points without a band and the backups still to come", async () => {
    await show();
    expect(parts("band")).toHaveLength(0);
    expect(parts("line")[0].getAttribute("d")).toMatch(/^M[\d. ]+(L[\d. ]+){3}$/);
    expect(parts("to-learn")).toHaveLength(6);
  });

  it("says the usual range is not known yet", async () => {
    await show();
    expect(screen.getByText(usualRange(en["anomaly.curve.learning"].replace("{needed}", "10")))).toBeTruthy();
  });
});

describe("the series behind a finding", () => {
  it("draws a dump's own series and names it", async () => {
    serve(
      { scopeKind: "item", part: "", quantities: [quantity({ quantity: "resticMs" })], failed: [] },
      { scopeKind: "dump", part: "", quantities: [quantity({ quantity: "resticMs" })], failed: [] }
    );
    await show(finding({ metric: "dump_duration_slower", scopeKind: "dump" }));
    expect(screen.getByText(en["anomaly.curve.dumpDuration"])).toBeTruthy();
  });

  it("names the dataset a ZFS finding is about", async () => {
    serve(
      { scopeKind: "item", part: "", quantities: [], failed: [] },
      { scopeKind: "zfsds", part: "tank/media", quantities: [quantity({ quantity: "sourceBytes" })], failed: [] }
    );
    await show(finding({ metric: "source_bytes_growth", scopeKind: "zfsds", part: "tank/media" }));
    expect(screen.getByText("tank/media").getAttribute("dir")).toBe("ltr");
  });

  it("crosses out the runs that failed", async () => {
    serve({
      scopeKind: "item",
      part: "",
      quantities: [quantity({ quantity: "resticMs" })],
      failed: [{ runId: "bad", at: 1700000000 + 1.5 * DAY }],
    });
    await show(finding({ metric: "flaky" }));
    expect(parts("failed")).toHaveLength(1);
  });

  it("draws nothing for a series detection has not measured", async () => {
    serve({ scopeKind: "item", part: "", quantities: [], failed: [] });
    await show();
    expect(document.querySelector("svg")).toBeNull();
  });
});

describe("loading", () => {
  it("offers another try when the measurements were refused", async () => {
    getAnomalyItemSeries.mockRejectedValueOnce(new Error("offline"));
    await show();
    expect(screen.getByText(en["anomaly.curve.loadFailed"])).toBeTruthy();

    serve({ scopeKind: "item", part: "", quantities: [quantity({ quantity: "sourceBytes" })], failed: [] });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["anomaly.retry"] }));
    });
    expect(parts("line")).toHaveLength(1);
  });
});
