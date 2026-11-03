// @vitest-environment jsdom
// An instance's scorecard says what the overview of that instance says about
// delete protection: an accepted gap is protected with the risk accepted, and
// a gap that turns a section amber is named under it.
import { afterEach, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";

import type { DomainStatus } from "../../lib/api";
import { I18nProvider, en } from "../../lib/i18n";
import { domain } from "../overview/overview.testsupport";
import { PeerScorecard } from "./PeerScorecard";

afterEach(cleanup);

function show(domains: DomainStatus[]) {
  render(
    <I18nProvider>
      <PeerScorecard domains={domains} />
    </I18nProvider>
  );
}

const row = (name: string) => within(document.querySelector(`li[data-domain="${name}"]`) as HTMLElement);
const open = { offsiteImmutable: false, tamperState: "" };

it("calls a section protected with the risk accepted when its only gap was accepted", () => {
  show([domain({ ...open, deleteRiskAccepted: true, protection: "green" }), domain({ domain: "zfs" })]);

  expect(row("containers").getByText(en["ransomware.protAccepted"])).toBeTruthy();
  expect(row("zfs").getByText(en["fleet.protection.green"])).toBeTruthy();
});

it("names the delete protection gap under an amber section", () => {
  show([
    domain({ ...open, protection: "amber" }),
    domain({ domain: "files", tamperState: "never", tamperOff: true, protection: "amber" }),
  ]);

  expect(row("containers").getByText(en["overview.check.lockNone"])).toBeTruthy();
  expect(row("files").getByText(en["overview.check.lockCheckOff"])).toBeTruthy();
  expect(screen.getAllByText(en["fleet.protection.amber"])).toHaveLength(2);
});

it("names no delete protection gap where something else makes a section amber", () => {
  show([domain({ replicationState: "overdue", protection: "amber" })]);

  expect(row("containers").queryByText(en["overview.check.lockNone"])).toBeNull();
  expect(row("containers").queryByText(en["overview.check.lockCheckOff"])).toBeNull();
});
