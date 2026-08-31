// @vitest-environment jsdom
// [376] — the protection card says when there is no second copy.
//
// Before this, both off-site columns were driven by a RECORDED off-site run, so
// a domain with no off-site repo at all rendered neither and the row went quiet
// at exactly the point where it had the most to say. Measured on jdp's box:
// protectionLevel returned "red" for all five domains ("enabled but no off-site
// copy, unprotected by design") while the page never printed the words
// off-site, append-only or 3-2-1 once.
//
// This renders the card directly rather than the page, because the page cannot
// show it on the test box: nothing is scheduled there, an unscheduled domain
// takes the "off" branch, and the branch under test is never reached. That is
// correct behaviour and a useless place to verify from.

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
    status: "ok", // NOT "off" — an unscheduled domain never reaches the branch
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
  // English straight from the table, so the assertions read as the words a
  // person sees rather than as key names.
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
    // The badge answers "is there a second copy anywhere", not "is it healthy".
    // Once one exists the existing columns take over, and repeating the point
    // would be the noise this card already had too much of.
    renderCard([domain({ offsiteConfigured: true })]);
    expect(screen.queryByText(/No off-site copy/)).toBeNull();
  });

  it("stays quiet for a domain that is not backed up at all", () => {
    // status "off" means no schedule. Telling somebody their second copy is
    // missing, for data nobody is copying a first time, is advice about the
    // wrong problem.
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
