// @vitest-environment jsdom
// The ransomware scorecard's append-only row.
//
// Reported in support as "immutable offsite should be baked into the product".
// The app already knew whether the off-site copy was deletable, and said so in
// the quietest way it had: a grey dash reading "append-only not enabled", on a
// card titled "ransomware protection", next to a domain graded "Protected".
//
// Renders the card directly rather than the page, for the same reason
// Dashboard.protectionCard.dom.test.tsx does: an unscheduled domain carries no
// protection posture, so on a test box `shown` is empty and the card renders
// nothing at all.

import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import type { DomainStatus } from "../lib/api";
import { RansomwareCard } from "./Dashboard";

function domain(over: Partial<DomainStatus> = {}): DomainStatus {
  return {
    domain: "containers",
    enabled: true,
    schedule: "daily@03:00",
    coveredBy: "",
    lastSuccess: 1_700_000_000,
    periodSeconds: 86_400,
    status: "ok",
    lastVerified: 0,
    lastVerifiedOK: false,
    verifiedDetail: "",
    drillDetail: "",
    offsiteConfigured: true,
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
    // Non-empty, or the card filters the domain out before any row renders.
    protection: "green",
    // "" is exactly !offsiteImmutable (internal/api/service.go,
    // protectionChecks): no append-only claim to prove, which is a different
    // thing from a claim that failed.
    tamperState: "",
    replicationState: "",
    drillState: "",
    encryptionOn: true,
    pruneStrategySet: true,
    ...over,
  };
}

function renderCard(domains: DomainStatus[]) {
  const t = ((k: string) => en[k as keyof typeof en] ?? k) as never;
  // A "bad" row renders its label as a <Link> into Settings, so the card needs
  // a router around it the moment any row is red.
  return render(
    <MemoryRouter>
      <I18nProvider>
        <RansomwareCard t={t} domains={domains} loading={false} hueIndex={0} />
      </I18nProvider>
    </MemoryRouter>
  );
}

describe("RansomwareCard, append-only not enabled", () => {
  afterEach(cleanup);

  it("reads as a gap, not as a grey dash, once an off-site copy exists", () => {
    renderCard([domain()]);
    const row = screen.getByText(en["ransomware.appendOnlyOff"]);
    expect(row.className).toContain("text-statusWarn");
    expect(row.className).not.toContain("text-carbon-textMuted");
  });

  it("stays grey when there is no off-site copy to make immutable", () => {
    // The row above it is already red and links to the same settings page.
    // "append-only not enabled" underneath would be advice about the wrong step.
    renderCard([domain({ offsiteConfigured: false, protection: "red" })]);
    const row = screen.getByText(en["ransomware.appendOnlyOff"]);
    expect(row.className).toContain("text-carbon-textMuted");
    expect(row.className).not.toContain("text-statusWarn");
  });

  it("leaves a proven append-only row alone", () => {
    // Only the "" arm is amber. A verified claim still reads verified, so a
    // correctly configured box cannot come out of this looking worse.
    renderCard([domain({ offsiteImmutable: true, tamperState: "ok", lastTamperAt: 1_700_000_000 })]);
    expect(screen.queryByText(en["ransomware.appendOnlyOff"])).toBeNull();
    const row = screen.getByText(en["ransomware.appendOnlyVerified"]);
    expect(row.className).not.toContain("text-statusWarn");
  });

  it("does not touch the failed and never arms", () => {
    // Both were already red and stay red: a claim that failed, or was never
    // proven, is a different statement from a claim nobody made.
    renderCard([domain({ offsiteImmutable: true, tamperState: "failed", lastTamperAt: 1_700_000_000 })]);
    expect(screen.getByText(en["ransomware.appendOnlyFailed"])).toBeTruthy();
    cleanup();
    renderCard([domain({ offsiteImmutable: true, tamperState: "never" })]);
    expect(screen.getByText(en["ransomware.appendOnlyNever"])).toBeTruthy();
  });
});
