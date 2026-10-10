// @vitest-environment jsdom
// The card of what needs the person: one row per thing, each with the action
// that settles it.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import { en } from "../lib/i18n";
import type { Need } from "../lib/needs";
import { NeedsCard } from "./overview/NeedsCard";
import { finding, item, resetServer, run, server } from "./overview/overview.testsupport";
import { inApp } from "./overview/overviewApp.testsupport";

vi.mock("../lib/api", async (importOriginal) => {
  const { stagedApi } = await import("./overview/overview.testsupport");
  return stagedApi(await importOriginal<typeof import("../lib/api")>());
});

const onErrors = vi.fn();
const onItemsChanged = vi.fn();
const onKitStored = vi.fn();

beforeEach(() => {
  resetServer();
  vi.clearAllMocks();
});

afterEach(cleanup);

function renderCard(needs: Need[]) {
  render(inApp(<NeedsCard needs={needs} onErrors={onErrors} onItemsChanged={onItemsChanged} onKitStored={onKitStored} />));
}

const called = (name: string) => server.calls.filter((c) => c.name === name).map((c) => c.args);

describe("NeedsCard", () => {
  it("says there is nothing to do when nothing waits", () => {
    renderCard([]);
    expect(screen.getByText(en["overview.needs.none"])).toBeTruthy();
    expect(screen.queryByRole("listitem")).toBeNull();
  });

  it("backs a failed entry up again", async () => {
    const plex = item();
    renderCard([{ is: "failedRun", run: run({ id: "r1", status: "failed", error: "restic exited 1" }), item: plex }]);

    const row = within(screen.getByTestId("need-failed-run"));
    expect(row.getByRole("link", { name: "plex" })).toBeTruthy();
    expect(row.getByText(/restic exited 1/)).toBeTruthy();

    fireEvent.click(row.getByRole("button", { name: en["breakdown.retry"] }));
    await act(async () => {});

    expect(called("backupNow")).toEqual([["plex"]]);
  });

  it("opens the failures from a failed run", () => {
    renderCard([{ is: "failedRun", run: run({ id: "r1", status: "failed" }), item: item() }]);

    fireEvent.click(screen.getByRole("button", { name: en["overview.needs.viewErrors"] }));

    expect(onErrors).toHaveBeenCalledOnce();
  });

  it("does not offer to repeat a failed restore", () => {
    renderCard([{ is: "failedRun", run: run({ id: "r1", kind: "restore", status: "failed" }), item: item() }]);

    const row = within(screen.getByTestId("need-failed-run"));
    expect(row.getByText(en["run.kindRestore"])).toBeTruthy();
    expect(row.queryByRole("button", { name: en["breakdown.retry"] })).toBeNull();
    expect(row.getByRole("button", { name: en["overview.needs.viewErrors"] })).toBeTruthy();
  });

  it("does not offer to repeat a run whose entry is gone from the list", () => {
    renderCard([{ is: "failedRun", run: run({ id: "r1", status: "failed" }) }]);

    expect(screen.queryByRole("button", { name: en["breakdown.retry"] })).toBeNull();
  });

  it("takes an entry that was left out into the schedule", async () => {
    const gitea = item({
      key: "gitea",
      name: "gitea",
      included: false,
      lastBackup: 0,
      effectiveSchedule: { kind: "none", spec: "", alsoSpec: "", reason: "excluded" },
    });
    renderCard([{ is: "unprotected", item: gitea }]);

    const row = within(screen.getByTestId("need-unprotected"));
    expect(row.getByText(en["backups.status.notProtected"])).toBeTruthy();

    fireEvent.click(row.getByRole("button", { name: en["overview.needs.include"] }));
    await act(async () => {});

    expect(called("setInclude")).toEqual([["gitea", true]]);
    expect(onItemsChanged).toHaveBeenCalledOnce();
  });

  it("leads to the entry where switching it on would change nothing", () => {
    const flash = item({
      kind: "flash",
      key: "flash",
      name: "flash",
      lastBackup: 0,
      effectiveSchedule: { kind: "none", spec: "", alsoSpec: "", reason: "schedule-off" },
    });
    renderCard([{ is: "unprotected", item: flash }]);

    const row = within(screen.getByTestId("need-unprotected"));
    expect(row.getByRole("link", { name: en["backups.name.flash"] })).toBeTruthy();
    expect(row.queryByRole("button", { name: en["overview.needs.include"] })).toBeNull();
    expect(row.getByRole("button", { name: en["zfs.replica.view"] })).toBeTruthy();
  });

  it("names an anomaly's entry and says what it found", () => {
    renderCard([{ is: "anomaly", finding: finding({ id: "a1", name: "immich", targetId: "tg-immich", retentionHeld: true }) }]);

    const row = within(screen.getByTestId("need-anomaly"));
    const link = row.getByRole("link", { name: en["overview.needs.anomalyAt"].replace("{name}", "immich") });
    expect(link.getAttribute("href")).toBe("/anomalies?scope=item:tg-immich");
    expect(row.getByText(en["anomaly.retentionKept"])).toBeTruthy();
    expect(row.getByRole("button", { name: en["overview.needs.check"] })).toBeTruthy();
  });

  it("downloads the recovery kit and takes the word that it is stored", async () => {
    renderCard([{ is: "recoveryKit" }]);
    const row = within(screen.getByTestId("need-recovery-kit"));

    fireEvent.click(row.getByRole("button", { name: en["dbdump.download"] }));
    await act(async () => {});
    expect(called("downloadRecoveryKit")).toHaveLength(1);
    expect(onKitStored).not.toHaveBeenCalled();

    fireEvent.click(row.getByRole("button", { name: en["recovery.stored"] }));
    await act(async () => {});
    expect(called("ackRecoveryKit")).toHaveLength(1);
    expect(onKitStored).toHaveBeenCalledOnce();
  });
});
