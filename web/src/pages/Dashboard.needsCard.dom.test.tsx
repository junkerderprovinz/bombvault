// @vitest-environment jsdom
// The card of what needs the person: one row per thing, each with the action
// that settles it.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import { useLocation } from "react-router-dom";

import * as api from "../lib/api";
import { ATTENTION_CHANGED_EVENT } from "../lib/attention";
import { en } from "../lib/i18n";
import type { Need } from "../lib/needs";
import { formatRecent } from "../lib/reltime";
import { NeedsCard } from "./overview/NeedsCard";
import { DAY, NOW, failedCopy, finding, item, location, resetServer, run, server } from "./overview/overview.testsupport";
import { inApp } from "./overview/overviewApp.testsupport";

vi.mock("../lib/api", async (importOriginal) => {
  const { stagedApi } = await import("./overview/overview.testsupport");
  return stagedApi(await importOriginal<typeof import("../lib/api")>());
});

const leftOut = {
  included: false,
  lastBackup: 0,
  lastRunStatus: "",
  effectiveSchedule: { kind: "none", spec: "", alsoSpec: "", reason: "excluded" },
} as const;

const setBackupDeclined = vi.mocked(api.setBackupDeclined);
const onErrors = vi.fn();
const onItemsChanged = vi.fn();

beforeEach(() => {
  resetServer();
  vi.clearAllMocks();
});

afterEach(cleanup);

function Where() {
  return <output>{useLocation().pathname}</output>;
}

function renderCard(needs: Need[]) {
  render(
    inApp(
      <>
        <NeedsCard needs={needs} onErrors={onErrors} onItemsChanged={onItemsChanged} />
        <Where />
      </>
    )
  );
}

/** The Storage Box with failed copies of the containers and the flash. */
function failedBox(): Need & { is: "failedCopy" } {
  const sections = [failedCopy(), failedCopy({ domain: "flash", targetId: "t-flash" })];
  return {
    is: "failedCopy",
    location: location({ id: "destination:box", object: "destination", kind: "offsite", name: "Storage Box", sections }),
    sections,
    since: NOW - DAY,
  };
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

  it("leaves an entry out of the backups for good and offers the way back", async () => {
    const gitea = item({ key: "gitea", name: "gitea", ...leftOut });
    renderCard([{ is: "unprotected", item: gitea }]);
    const row = within(screen.getByTestId("need-unprotected"));
    expect(row.getByRole("button", { name: en["overview.needs.include"] })).toBeTruthy();

    fireEvent.click(row.getByRole("button", { name: en["backups.decline"] }));
    await act(async () => {});

    expect(called("setBackupDeclined")).toEqual([["container", "gitea", true]]);
    expect(onItemsChanged).toHaveBeenCalledOnce();
    expect(screen.getByText(en["backups.declined"].replace("{name}", "gitea"))).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: en["takeover.undo"] }));
    await act(async () => {});

    expect(called("setBackupDeclined")[1]).toEqual(["container", "gitea", false]);
    expect(onItemsChanged).toHaveBeenCalledTimes(2);
  });

  it("says so when the server will not leave an entry out, and reads the entries again", async () => {
    setBackupDeclined.mockResolvedValueOnce({ ok: false, error: "a schedule backs this entry up", code: "scheduled" });
    renderCard([{ is: "unprotected", item: item({ key: "gitea", name: "gitea", ...leftOut }) }]);

    fireEvent.click(screen.getByRole("button", { name: en["backups.decline"] }));
    await act(async () => {});

    expect(screen.getByText("a schedule backs this entry up")).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["takeover.undo"] })).toBeNull();
    expect(onItemsChanged).toHaveBeenCalledOnce();
  });

  it("offers to leave out an entry that has no switch of its own as well", () => {
    const flash = item({ kind: "flash", key: "flash", name: "flash", lastBackup: 0, effectiveSchedule: { kind: "none", spec: "", alsoSpec: "", reason: "schedule-off" } });
    renderCard([{ is: "unprotected", item: flash }]);

    const row = within(screen.getByTestId("need-unprotected"));
    expect(row.getByRole("button", { name: en["zfs.replica.view"] })).toBeTruthy();
    expect(row.getByRole("button", { name: en["backups.decline"] })).toBeTruthy();
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

  it("says where an off-site copy failed, for which sections and since when", () => {
    renderCard([failedBox()]);

    const row = within(screen.getByTestId("need-failed-copy"));
    const link = row.getByRole("link", { name: en["overview.needs.copyFailed"] });
    expect(link.getAttribute("href")).toBe("/storage/destination%3Abox");
    const since = en["overview.needs.copySince"].replace("{when}", formatRecent(NOW - DAY, "en"));
    expect(row.getByText(`Storage Box · ${en["nav.containers"]} and ${en["nav.flash"]} · ${since}`)).toBeTruthy();
  });

  it("tries a failed copy again for each of its sections", async () => {
    renderCard([failedBox()]);

    fireEvent.click(within(screen.getByTestId("need-failed-copy")).getByRole("button", { name: en["breakdown.retry"] }));
    await act(async () => {});

    expect(called("replicateOffsite")).toEqual([["containers"], ["flash"]]);
    expect(screen.getByText(en["offsite.replicateStarted"])).toBeTruthy();
  });

  it("opens the location of a failed copy", () => {
    renderCard([failedBox()]);

    fireEvent.click(within(screen.getByTestId("need-failed-copy")).getByRole("button", { name: en["zfs.replica.view"] }));

    expect(screen.getByRole("status").textContent).toBe("/storage/destination%3Abox");
  });

  it("marks a failed copy as seen and says that the list has changed", async () => {
    const changed = vi.fn();
    window.addEventListener(ATTENTION_CHANGED_EVENT, changed);
    renderCard([failedBox()]);

    fireEvent.click(within(screen.getByTestId("need-failed-copy")).getByRole("button", { name: en["errorPanel.resolve"] }));
    await act(async () => {});

    expect(called("ackFailedCopies")).toEqual([[["t-containers", "t-flash"]]]);
    expect(changed).toHaveBeenCalledOnce();
    window.removeEventListener(ATTENTION_CHANGED_EVENT, changed);
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
    const settingsChanged = vi.fn();
    window.addEventListener("bv:settings-changed", settingsChanged);
    renderCard([{ is: "recoveryKit" }]);
    const row = within(screen.getByTestId("need-recovery-kit"));

    fireEvent.click(row.getByRole("button", { name: en["dbdump.download"] }));
    await act(async () => {});
    expect(called("downloadRecoveryKit")).toHaveLength(1);
    expect(settingsChanged).not.toHaveBeenCalled();

    fireEvent.click(row.getByRole("button", { name: en["recovery.stored"] }));
    await act(async () => {});
    expect(called("ackRecoveryKit")).toHaveLength(1);
    expect(settingsChanged).toHaveBeenCalledOnce();
    window.removeEventListener("bv:settings-changed", settingsChanged);
  });
});
