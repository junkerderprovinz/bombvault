// @vitest-environment jsdom
/**
 * The coverage card.
 *
 * The protection card beside it is per domain: it says whether the items that
 * ARE scheduled ran on time. This one answers the question that traffic light
 * cannot, and the reason the question matters is that its answer is invisible
 * everywhere else. A container nobody ever added is absent from every list and
 * every error, so nothing on the dashboard turns amber for it. It has to be
 * named, by name, or it is not found until it is needed.
 */
import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { CoverageCard } from "./Dashboard";
import { en } from "../lib/i18n";
import type { CoverageReport } from "../lib/api";

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as unknown as Parameters<
  typeof CoverageCard
>[0]["t"];

function renderCard(coverage: CoverageReport | null, loading = false) {
  return render(<CoverageCard t={t} coverage={coverage} loading={loading} />);
}

afterEach(cleanup);

describe("CoverageCard", () => {
  it("names the container nobody ever added", () => {
    renderCard({
      total: 2,
      protected: 1,
      domains: [
        {
          domain: "containers",
          enabled: true,
          total: 2,
          protected: 1,
          unprotected: [{ name: "sonarr", reason: "not-set-up", neverBackedUp: true }],
        },
        { domain: "vms", enabled: false, total: 0, protected: 0, unprotected: [] },
        { domain: "files", enabled: true, total: 0, protected: 0, unprotected: [] },
      ],
    });

    expect(screen.getByText("sonarr")).toBeTruthy();
    expect(screen.getByText(new RegExp(en["coverage.reason.notSetUp"], "i"))).toBeTruthy();
  });

  it("says everything is protected rather than showing an empty box", () => {
    renderCard({
      total: 3,
      protected: 3,
      domains: [
        { domain: "containers", enabled: true, total: 3, protected: 3, unprotected: [] },
        { domain: "vms", enabled: false, total: 0, protected: 0, unprotected: [] },
        { domain: "files", enabled: true, total: 0, protected: 0, unprotected: [] },
      ],
    });

    expect(screen.getByText(en["coverage.allProtected"])).toBeTruthy();
  });

  // The reason is the difference between "there is a hole" and "here is the
  // switch that closes it". A list of bare names would make the operator hunt.
  it("gives each item its reason", () => {
    renderCard({
      total: 3,
      protected: 0,
      domains: [
        {
          domain: "containers",
          enabled: true,
          total: 3,
          protected: 0,
          unprotected: [
            { name: "plex", reason: "not-included", neverBackedUp: false },
            { name: "radarr", reason: "override-off", neverBackedUp: true },
            { name: "lidarr", reason: "no-schedule", neverBackedUp: true },
          ],
        },
      ],
    });

    expect(screen.getByText(new RegExp(en["coverage.reason.notIncluded"], "i"))).toBeTruthy();
    expect(screen.getByText(new RegExp(en["coverage.reason.overrideOff"], "i"))).toBeTruthy();
    expect(screen.getByText(new RegExp(en["coverage.reason.noSchedule"], "i"))).toBeTruthy();
  });

  // A switched-off domain is a decision. Counting its items as unprotected
  // would make the card cry wolf on a correctly configured server, and a card
  // that cries wolf gets hidden.
  it("does not count a switched-off domain against the ratio", () => {
    renderCard({
      total: 1,
      protected: 1,
      domains: [
        { domain: "containers", enabled: true, total: 1, protected: 1, unprotected: [] },
        { domain: "vms", enabled: false, total: 0, protected: 0, unprotected: [] },
        { domain: "files", enabled: false, total: 0, protected: 0, unprotected: [] },
      ],
    });

    expect(screen.getByText(en["coverage.allProtected"])).toBeTruthy();
  });
});
