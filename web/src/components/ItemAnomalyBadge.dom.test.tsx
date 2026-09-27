// @vitest-environment jsdom
// The badge beside an item's name: its open findings, or how far it has
// learned. The learning caption goes with the detection switch; the counts stay.
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

import { ItemAnomalyBadge } from "./ItemAnomalyBadge";
import { countText, en } from "../lib/i18n";
import type { AnomalyItem } from "../lib/api";

const t = ((key: string, n?: number) =>
  countText((en as Record<string, string>)[key] ?? key, "en", n)) as unknown as Parameters<
  typeof ItemAnomalyBadge
>[0]["t"];

function item(over: Partial<AnomalyItem> = {}): AnomalyItem {
  return {
    targetId: "tg-1",
    domain: "container",
    name: "plex",
    scheduled: true,
    sensitivity: "",
    effective: "balanced",
    notifyMin: "",
    effectiveNotifyMin: "warning",
    learning: { samples: 10, needed: 10, newData: 10, source: 10, duration: 10, noData: false },
    typical: { sourceBytes: null, newDataBytes: null, resticMs: null },
    dump: null,
    datasets: [],
    open: { critical: 0, warning: 0, info: 0 },
    retentionHeld: false,
    selectionSince: 0,
    expectations: [],
    ...over,
  };
}

function renderBadge(props: Partial<Parameters<typeof ItemAnomalyBadge>[0]> = {}) {
  return render(
    <MemoryRouter>
      <ItemAnomalyBadge t={t} enabled {...props} />
    </MemoryRouter>
  );
}

afterEach(cleanup);

describe("ItemAnomalyBadge", () => {
  it("links the open count to the item's own findings", () => {
    renderBadge({ item: item({ open: { critical: 1, warning: 2, info: 0 } }) });
    const link = screen.getByRole("link", { name: /Open anomalies for plex: 3/ });
    expect(link.getAttribute("href")).toBe("/anomalies?scope=item:tg-1#findings");
    expect(link.textContent).toContain("3");
  });

  it("counts the findings of the item's database dump with its own", () => {
    renderBadge({
      item: item({
        open: { critical: 0, warning: 1, info: 0 },
        dump: {
          part: "",
          learning: { samples: 10, needed: 10 },
          typical: { sourceBytes: null, resticMs: null },
          open: { critical: 1, warning: 0, info: 0 },
          retentionHeld: true,
        },
      }),
    });
    const link = screen.getByRole("link", { name: /Open anomalies for plex: 2/ });
    expect(link.querySelector("[class*='statusFail']")).toBeTruthy();
  });

  it("shows how far an item has learned", () => {
    renderBadge({
      item: item({
        learning: { samples: 7, needed: 10, newData: 7, source: 7, duration: 7, noData: false },
      }),
    });
    expect(screen.getByText("Learning 7/10")).toBeTruthy();
  });

  it("says when an item has nothing to learn from", () => {
    renderBadge({
      item: item({
        learning: { samples: 0, needed: 10, newData: 0, source: 0, duration: 0, noData: true },
      }),
    });
    expect(screen.getByText(en["anomaly.noData"])).toBeTruthy();
  });

  it("keeps the counts but drops the learning caption while detection is off", () => {
    renderBadge({
      enabled: false,
      item: item({
        open: { critical: 0, warning: 1, info: 0 },
        learning: { samples: 7, needed: 10, newData: 7, source: 7, duration: 7, noData: false },
      }),
    });
    expect(screen.getByRole("link", { name: /Open anomalies for plex: 1/ })).toBeTruthy();
    expect(screen.queryByText("Learning 7/10")).toBeNull();
  });

  it("renders nothing for an item the engine does not know", () => {
    const { container } = renderBadge();
    expect(container.textContent).toBe("");
  });

  it("renders nothing once an item has learned", () => {
    const { container } = renderBadge({ item: item() });
    expect(container.textContent).toBe("");
  });
});
