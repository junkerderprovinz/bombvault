// @vitest-environment jsdom
// The ransomware card's append-only row. The card is rendered directly, like in
// Dashboard.protectionCard.dom.test.tsx, because on a test box no domain is
// scheduled and the page renders nothing.

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
    offPremisesCovered: false,
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
    // "" is what protectionChecks in internal/api/service.go reports when
    // append-only is off: nothing to prove, which differs from a failed proof.
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
  // A red row renders its label as a <Link> into Settings.
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
    // The off-site row above is already red and links to the same settings.
    renderCard([domain({ offsiteConfigured: false, protection: "red" })]);
    const row = screen.getByText(en["ransomware.appendOnlyOff"]);
    expect(row.className).toContain("text-carbon-textMuted");
    expect(row.className).not.toContain("text-statusWarn");
  });

  it("leaves a proven append-only row alone", () => {
    renderCard([domain({ offsiteImmutable: true, tamperState: "ok", lastTamperAt: 1_700_000_000 })]);
    expect(screen.queryByText(en["ransomware.appendOnlyOff"])).toBeNull();
    const row = screen.getByText(en["ransomware.appendOnlyVerified"]);
    expect(row.className).not.toContain("text-statusWarn");
  });

  it("does not touch the failed and never arms", () => {
    // A claim that failed or was never proven stays red, unlike a claim nobody
    // made.
    renderCard([domain({ offsiteImmutable: true, tamperState: "failed", lastTamperAt: 1_700_000_000 })]);
    expect(screen.getByText(en["ransomware.appendOnlyFailed"])).toBeTruthy();
    cleanup();
    renderCard([domain({ offsiteImmutable: true, tamperState: "never" })]);
    expect(screen.getByText(en["ransomware.appendOnlyNever"])).toBeTruthy();
  });
});

describe("RansomwareCard, paused replication", () => {
  afterEach(cleanup);

  it("shows a paused replication as its own amber row", () => {
    renderCard([domain({ replicationState: "paused", protection: "amber" })]);
    const row = screen.getByText(en["ransomware.replicationPaused"]);
    expect(row.className).toContain("text-statusWarn");
  });
});
