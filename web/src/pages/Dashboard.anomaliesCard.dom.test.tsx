// @vitest-environment jsdom
/**
 * The anomalies card.
 *
 * The card's job is to be believed. Every state has to be told apart from
 * every other one, and the failed fetch has to be told apart from silence:
 * "nothing wrong" and "we could not look" are the two answers that must never
 * share a wording.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

import { AnomaliesCard } from "./Dashboard";
import { AnomalyProvider, useOpenAnomalies } from "../lib/useAnomalies";
import { ANOMALY_CHANGED_EVENT } from "../lib/anomalies";
import { countText, en } from "../lib/i18n";
import type { AnomalySummary, AnomalyView } from "../lib/api";

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return {
    ...actual,
    getAnomalySummary: vi.fn(),
    getAnomalies: vi.fn(),
    getAnomalyItems: vi.fn(),
  };
});

const api = await import("../lib/api");
const getAnomalySummary = vi.mocked(api.getAnomalySummary);
const getAnomalies = vi.mocked(api.getAnomalies);

const t = ((key: string, n?: number) =>
  countText((en as Record<string, string>)[key] ?? key, "en", n)) as unknown as Parameters<
  typeof AnomaliesCard
>[0]["t"];

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

function renderCard(props: Partial<Parameters<typeof AnomaliesCard>[0]> = {}) {
  return render(
    <MemoryRouter>
      <AnomaliesCard t={t} summary={null} open={[]} loading={false} error={false} {...props} />
    </MemoryRouter>
  );
}

beforeEach(() => {
  seq = 0;
  getAnomalySummary.mockReset();
  getAnomalies.mockReset();
});

afterEach(cleanup);

describe("AnomaliesCard", () => {
  it("says it is still looking while the first answer is on its way", () => {
    renderCard({ loading: true });
    expect(screen.getByText(en["dashboard.checking"])).toBeTruthy();
  });

  // The one state the card must never get wrong: a request that did not
  // arrive is not an all-clear.
  it("never reads as an all-clear when the request failed", () => {
    renderCard({ error: true, summary: summary() });
    expect(screen.getByText(en["anomaly.loadFailed"])).toBeTruthy();
    expect(screen.queryByText(en["anomaly.allClear"])).toBeNull();
  });

  it("offers the settings when detection is switched off", () => {
    renderCard({ summary: summary({ enabled: false }) });
    expect(screen.getByText(en["anomaly.off"])).toBeTruthy();
    expect(screen.getByRole("link", { name: en["anomaly.openSettings"] }).getAttribute("href")).toBe(
      "/settings#anomalies"
    );
    expect(screen.queryByText(en["anomaly.showEarlier"])).toBeNull();
  });

  it("still points at earlier findings while detection is off", () => {
    renderCard({
      summary: summary({ enabled: false, open: { critical: 1, warning: 0, info: 0 } }),
      open: [finding()],
    });
    expect(screen.getByRole("link", { name: en["anomaly.showEarlier"] })).toBeTruthy();
  });

  it("shows the rows it already has while the first pass runs", () => {
    renderCard({
      summary: summary({ ready: false, open: { critical: 1, warning: 0, info: 0 } }),
      open: [finding()],
    });
    expect(screen.getByText(en["anomaly.checking"])).toBeTruthy();
    expect(screen.getByText(en["anomaly.severity.critical"])).toBeTruthy();
  });

  it("counts the items that have not learned enough yet", () => {
    renderCard({ summary: summary({ learningItems: 3 }) });
    expect(screen.getByText(en["anomaly.allClear"])).toBeTruthy();
    expect(screen.getByText(en["anomaly.learningCount"].replace("{n}", "3"))).toBeTruthy();
  });

  it("keeps notes off the card and links to them instead", () => {
    renderCard({
      summary: summary({ open: { critical: 0, warning: 0, info: 2 } }),
      open: [finding({ severity: "info" })],
    });
    expect(screen.getByText(en["anomaly.allClear"])).toBeTruthy();
    expect(screen.getByRole("link", { name: en["anomaly.notesCount"].replace("{n}", "2") })).toBeTruthy();
    expect(screen.queryByText(en["anomaly.severity.info"])).toBeNull();
  });

  it("names the items it could not check", () => {
    renderCard({ summary: summary({ evalErrors: 2 }) });
    const line = screen.getByText(en["anomaly.evalErrors"].replace("{n}", "2"));
    expect(line.className).toContain("text-statusWarn");
  });

  // Vault's split: the criticals first, a few warnings under them, and a count
  // of what a full list would add.
  it("shows five critical and three warning rows, then says how many are left", () => {
    const rows = [
      ...Array.from({ length: 7 }, () => finding()),
      ...Array.from({ length: 5 }, () => finding({ severity: "warning" })),
    ];
    renderCard({ summary: summary({ open: { critical: 7, warning: 5, info: 0 } }), open: rows });
    expect(screen.getAllByText(en["anomaly.severity.critical"])).toHaveLength(5);
    expect(screen.getAllByText(en["anomaly.severity.warning"])).toHaveLength(3);
    expect(screen.getByText(en["anomaly.moreCount"].replace("{n}", "4"))).toBeTruthy();
    expect(screen.getByRole("link", { name: en["anomaly.showAll"] })).toBeTruthy();
  });

  it("marks a critical whose cause has gone away", () => {
    renderCard({
      summary: summary({ open: { critical: 1, warning: 0, info: 0 }, recoveredCritical: 1 }),
      open: [finding({ recoveredAt: 1700000500 })],
    });
    expect(screen.getByText(en["anomaly.recovered"])).toBeTruthy();
  });

  // Releasing a hold deletes old backups again, so it is never one click.
  it("asks before an acknowledgement releases a retention hold", async () => {
    const onAcknowledge = vi.fn().mockResolvedValue({ ok: true });
    renderCard({
      summary: summary({ open: { critical: 1, warning: 0, info: 0 }, retentionHeld: 1 }),
      open: [finding({ name: "plex", retentionHeld: true })],
      onAcknowledge,
    });

    fireEvent.click(screen.getByRole("button", { name: en["anomaly.action.acknowledge"] }));
    expect(screen.getByRole("dialog")).toBeTruthy();
    expect(
      screen.getByText(en["anomaly.releaseConfirm"].replace("{name}", "plex"))
    ).toBeTruthy();
    expect(onAcknowledge).not.toHaveBeenCalled();

    const dialog = screen.getByRole("dialog");
    await act(async () => {
      fireEvent.click(within(dialog).getByRole("button", { name: en["anomaly.action.acknowledge"] }));
    });
    expect(onAcknowledge).toHaveBeenCalledTimes(1);
  });
});

function OpenProbe() {
  const { list } = useOpenAnomalies();
  return <output>{list.map((a) => a.id).join(",")}</output>;
}

describe("the card's data", () => {
  // The count in the summary and the rows under it are read at two different
  // moments. Tying both to the generation is what keeps them from disagreeing.
  it("refetches the rows whenever the summary reports a new generation", async () => {
    getAnomalySummary
      .mockResolvedValueOnce({ ok: true, summary: summary({ open: { critical: 1, warning: 0, info: 0 } }) })
      .mockResolvedValue({
        ok: true,
        summary: summary({ generation: 2, open: { critical: 2, warning: 0, info: 0 } }),
      });
    getAnomalies
      .mockResolvedValueOnce({ ok: true, anomalies: [finding({ id: "first" })], nextCursor: "" })
      .mockResolvedValue({
        ok: true,
        anomalies: [finding({ id: "first" }), finding({ id: "second" })],
        nextCursor: "",
      });

    render(
      <AnomalyProvider>
        <OpenProbe />
      </AnomalyProvider>
    );

    await waitFor(() => expect(screen.getByRole("status").textContent).toBe("first"));
    window.dispatchEvent(new Event(ANOMALY_CHANGED_EVENT));
    await waitFor(() => expect(screen.getByRole("status").textContent).toBe("first,second"));
  });

  it("pages until the server stops handing out a cursor", async () => {
    getAnomalySummary.mockResolvedValue({ ok: true, summary: summary() });
    getAnomalies
      .mockResolvedValueOnce({ ok: true, anomalies: [finding({ id: "one" })], nextCursor: "c1" })
      .mockResolvedValueOnce({ ok: true, anomalies: [finding({ id: "two" })], nextCursor: "" });

    render(
      <AnomalyProvider>
        <OpenProbe />
      </AnomalyProvider>
    );

    await waitFor(() => expect(screen.getByRole("status").textContent).toBe("one,two"));
    expect(getAnomalies).toHaveBeenCalledTimes(2);
    expect(getAnomalies.mock.calls[1][0]).toMatchObject({ cursor: "c1" });
  });
});
