// @vitest-environment jsdom
// The protection card says when a scheduled domain has no off-site copy. It is
// rendered directly because the page only shows it for scheduled domains, and
// nothing is scheduled on a test box.

import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import type { DomainStatus } from "../lib/api";
import { ProtectionCard } from "./Dashboard";

function domain(over: Partial<DomainStatus> = {}): DomainStatus {
  return {
    domain: "containers",
    enabled: true,
    schedule: "daily@03:00",
    coveredBy: "",
    lastSuccess: 1_700_000_000,
    periodSeconds: 86_400,
    status: "ok", // "off" would skip the branch under test
    lastVerified: 0,
    lastVerifiedOK: false,
    verifiedDetail: "",
    drillDetail: "",
    offsiteConfigured: false,
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
    protection: "red",
    tamperState: "",
    replicationState: "",
    drillState: "",
    encryptionOn: true,
    pruneStrategySet: false,
    ...over,
  };
}

function renderCard(domains: DomainStatus[]) {
  // Real English strings, so the assertions read as what a person sees.
  const t = ((k: string) => en[k as keyof typeof en] ?? k) as never;
  return render(
    <I18nProvider>
      <ProtectionCard t={t} domains={domains} loading={false} hueIndex={0} />
    </I18nProvider>
  );
}

describe("ProtectionCard, off-site", () => {
  afterEach(cleanup);

  it("says so when a scheduled domain has no off-site copy", () => {
    renderCard([domain()]);
    expect(screen.getByText(/No off-site copy/)).toBeTruthy();
  });

  it("stays quiet once an off-site repo exists", () => {
    // The badge answers whether a second copy exists, not whether it is
    // healthy; the other columns cover that.
    renderCard([domain({ offsiteConfigured: true })]);
    expect(screen.queryByText(/No off-site copy/)).toBeNull();
  });

  it("stays quiet for a domain that is not backed up at all", () => {
    // Without a first copy, a missing second copy is the wrong advice.
    renderCard([domain({ status: "off", schedule: "" })]);
    expect(screen.queryByText(/No off-site copy/)).toBeNull();
  });

  it("marks every scheduled domain that lacks one, not just the first", () => {
    renderCard([
      domain({ domain: "containers" }),
      domain({ domain: "vms" }),
      domain({ domain: "flash", offsiteConfigured: true }),
    ]);
    expect(screen.getAllByText(/No off-site copy/)).toHaveLength(2);
  });
});

describe("ProtectionCard, the ZFS row", () => {
  afterEach(cleanup);

  it("names the domain and offers the drill every DR-capable domain gets", () => {
    renderCard([domain({ domain: "zfs", offsiteConfigured: true, offsiteDrillScheduled: true })]);
    expect(screen.getByText(en["dashboard.domainZFS"])).toBeTruthy();
    expect(screen.getByRole("button", { name: en["drill.runOffsiteDr"] })).toBeTruthy();
  });
});
