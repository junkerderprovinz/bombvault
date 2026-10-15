// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, screen } from "@testing-library/react";
import type { Run } from "../../lib/api";
import { en } from "../../lib/i18n";
import { renderWithProviders } from "../../lib/placement.testsupport";
import { EntryRuns } from "./EntryRuns";

function run(over: Partial<Run>): Run {
  return {
    id: "r1",
    target: "nextcloud",
    domain: "container",
    kind: "backup",
    status: "success",
    startedAt: 1_700_000_000,
    finishedAt: 1_700_000_010,
    snapshotId: "",
    bytes: 0,
    error: "",
    ...over,
  } as Run;
}

describe("EntryRuns", () => {
  afterEach(cleanup);

  it("says which data a successful run could not reach", () => {
    const error = "data folders outside Host Data were not backed up: /mnt/.ix-apps/app_mounts/nc/data";
    renderWithProviders(<EntryRuns runs={[run({ error })]} />);
    expect(screen.getByText(en["runReason.dataOutsideHost"], { exact: false })).toBeTruthy();
    expect(screen.getByText("/mnt/.ix-apps/app_mounts/nc/data")).toBeTruthy();
  });

  it("stays quiet about a plain success", () => {
    renderWithProviders(<EntryRuns runs={[run({})]} />);
    expect(screen.queryByText(en["runReason.dataOutsideHost"], { exact: false })).toBeNull();
  });
});
