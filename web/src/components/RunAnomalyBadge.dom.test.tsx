// @vitest-environment jsdom
// The mark on a run in the history and the activity log, which has to say
// why without a trip to the Anomalies page.
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import { RunAnomalyBadge } from "./RunAnomalyBadge";
import { countText, en } from "../lib/i18n";
import type { AnomalyView } from "../lib/api";

const t = ((key: string, n?: number) =>
  countText((en as Record<string, string>)[key] ?? key, "en", n)) as unknown as Parameters<
  typeof RunAnomalyBadge
>[0]["t"];

function finding(over: Partial<AnomalyView> = {}): AnomalyView {
  return {
    id: "an-1",
    detector: "duration",
    metric: "duration_slower",
    severity: "warning",
    state: "open",
    scopeKind: "item",
    scopeId: "tg-1",
    targetId: "tg-1",
    domain: "container",
    part: "",
    targetName: "",
    name: "plex",
    runId: "run-1",
    lastRunId: "run-1",
    lastRunAt: 1700000000,
    observed: 600_000,
    expected: 60_000,
    threshold: 300_000,
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

afterEach(cleanup);

describe("RunAnomalyBadge", () => {
  it("marks nothing on a run no finding points at", () => {
    const { container } = render(<RunAnomalyBadge t={t} />);
    expect(container.textContent).toBe("");
  });

  it("takes the tone of the worst finding and explains itself in a bubble", () => {
    render(
      <RunAnomalyBadge
        t={t}
        findings={[finding(), finding({ id: "an-2", metric: "source_bytes_shrink", severity: "critical" })]}
      />
    );
    const badge = screen.getByText(en["anomaly.runBadge"]);
    expect(badge.className).toContain("statusFail");
    expect(badge.getAttribute("title")).toBeNull();
    expect(screen.getByLabelText(/Backing up plex took \u206610m/)).toBeTruthy();
  });
});
