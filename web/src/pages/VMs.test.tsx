// @vitest-environment jsdom
// VMRow's actions have to send VM.libvirtName, the raw libvirt domain name.
// VM.name is only for display, and on TrueNAS it is a friendly name virsh does
// not know. Only a component test sees how a prop reaches an API call, hence
// jsdom.
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup, within } from "@testing-library/react";
import { VMRow } from "./VMs";
import type { VM } from "../lib/api";
import {
  homeOption,
  placementOptions,
  placementView,
  stubEventSource,
  timelineMark,
  timelinePlace,
  timelineRow,
} from "../lib/placement.testsupport";

// useProgress() (lib/progress.ts) opens a real EventSource on mount; jsdom
// does not implement it.
stubEventSource();

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return {
    ...actual,
    // useBackupWatch's fire() reads listRuns() before and after start(), so an
    // empty list keeps the watch off the network.
    listRuns: vi.fn(async () => ({ ok: true, runs: [] })),
    backupVMNow: vi.fn(async () => ({ ok: true, started: true })),
    forgetVM: vi.fn(async () => ({ ok: true })),
    deleteBackupsVM: vi.fn(async () => ({ ok: true })),
    setVMInclude: vi.fn(async () => ({ ok: true })),
    getTimeline: vi.fn(async () => ({ ok: true, places: [], rows: [] })),
    getPlacementOptions: vi.fn(async () => ({ ok: true, options: placementOptions({ homes: [homeOption()] }) })),
    getSettings: vi.fn(async () => ({ ok: true, platform: "unraid" })),
  };
});

// Imported after vi.mock so these bindings are the mocked functions.
import { backupVMNow, deleteBackupsVM, forgetVM, getTimeline, setVMInclude } from "../lib/api";
import { en } from "../lib/i18n";

const noop = () => {
  /* no-op */
};
// VMRow only needs t() to return a stable string per key; none of this test's
// assertions depend on real translations.
const t = ((key: string) => key) as unknown as Parameters<typeof VMRow>[0]["t"];
// For the one test that reads a sentence rather than a key.
const tEn = ((key: string) => en[key as keyof typeof en] ?? key) as unknown as Parameters<typeof VMRow>[0]["t"];

// A TrueNAS-shaped VM whose display name and libvirt name differ.
const trueNasVM: VM = {
  name: "debian",
  libvirtName: "550e8400-e29b-41d4-a716-446655440000",
  state: "running",
  method: "graceful",
  includeInSchedule: false,
  lastBackup: null,
  lastBackupStarted: null,
  placement: placementView(),
};

afterEach(() => {
  cleanup();
});

describe("VMRow action wiring", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sends VM.libvirtName to backupVMNow, never the display VM.name", async () => {
    render(<VMRow vm={trueNasVM} t={t} onRefresh={noop} onPlacement={noop} index={0} />);

    // By role and name: the viewer chooses whether a control shows its text,
    // its glyph or both, and only the accessible name stays the same.
    fireEvent.click(screen.getByRole("button", { name: "containers.backupNow" }));

    await waitFor(() => expect(backupVMNow).toHaveBeenCalled());
    expect(backupVMNow).toHaveBeenCalledWith(trueNasVM.libvirtName);
    expect(backupVMNow).not.toHaveBeenCalledWith(trueNasVM.name);
  });

  it("still shows the display name to the user, not the raw identifier", () => {
    render(<VMRow vm={trueNasVM} t={t} onRefresh={noop} onPlacement={noop} index={0} />);
    expect(screen.getByText(trueNasVM.name)).toBeTruthy();
    expect(screen.queryByText(trueNasVM.libvirtName)).toBeNull();
  });
});

// Structural assertions rather than snapshots: the risk is the VM and
// container rows drifting apart, and a snapshot goes green on any change that
// is merely re-approved.
describe("VMRow matches the container card's structure", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the backups disclosure as a pressable chip, not a bespoke button", () => {
    render(<VMRow vm={trueNasVM} t={t} onRefresh={noop} onPlacement={noop} index={0} />);

    // Selector's segments carry aria-pressed, which a plain <button> lacks.
    const chip = screen.getByRole("button", { name: "snapshots.title" });
    expect(chip.getAttribute("aria-pressed")).toBe("false");
  });

  it("carries the placement bar, the same row the container card has", async () => {
    render(<VMRow vm={trueNasVM} t={t} onRefresh={noop} onPlacement={noop} index={0} />);
    expect(await screen.findByRole("toolbar", { name: en["placement.title"] })).toBeTruthy();
  });

  it("keeps the backups pane collapsed until the chip is pressed, and the row owns that state", () => {
    render(<VMRow vm={trueNasVM} t={t} onRefresh={noop} onPlacement={noop} index={0} />);

    // Closed: VMRestorePanel returns null, so nothing of its content exists.
    expect(screen.queryByText("source.label")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "snapshots.title" }));
    expect(
      screen.getByRole("button", { name: "snapshots.title" }).getAttribute("aria-pressed")
    ).toBe("true");
  });

  it("shows last-backup as one combined summary line, like the container row", () => {
    render(<VMRow vm={trueNasVM} t={t} onRefresh={noop} onPlacement={noop} index={0} />);

    // lastBackup is null on the fixture, so the line ends in the "never" key.
    // Two stacked <p>s would leave no single node with both halves.
    expect(screen.getByText("containers.lastBackup: containers.never")).toBeTruthy();
  });

  it("offers the backup method as two icon-only badges, with the stored one active", () => {
    render(<VMRow vm={trueNasVM} t={t} onRefresh={noop} onPlacement={noop} index={0} />);

    // A native <select> would expose a combobox here instead of the segments.
    const method = screen.getByRole("tablist", { name: "vm.method" });
    expect(within(method).queryByRole("combobox")).toBeNull();

    // Single-select segments are tabs with aria-selected, and an icon-only
    // segment's accessible name is its label.
    const graceful = screen.getByRole("tab", { name: "vm.method.graceful" });
    const live = screen.getByRole("tab", { name: "vm.method.live" });
    // Both show at once, so the alternative never has to be inferred.
    expect(graceful.getAttribute("aria-selected")).toBe("true");
    expect(live.getAttribute("aria-selected")).toBe("false");
  });

  it("opens the backups timeline under the libvirt name", async () => {
    render(<VMRow vm={trueNasVM} t={t} onRefresh={noop} onPlacement={noop} index={0} />);
    fireEvent.click(screen.getByRole("button", { name: "snapshots.title" }));
    await waitFor(() => expect(getTimeline).toHaveBeenCalledWith("vms", trueNasVM.libvirtName));
  });

  it("deletes the local backups its question names", async () => {
    vi.mocked(getTimeline).mockResolvedValueOnce({
      ok: true,
      places: [timelinePlace()],
      rows: [timelineRow("a1a1a1a1", "2026-09-18T03:00:00Z", timelineMark("local", "a1a1a1a1"))],
    });
    render(<VMRow vm={trueNasVM} t={tEn} onRefresh={noop} onPlacement={noop} index={0} />);
    fireEvent.click(screen.getByRole("button", { name: en["snapshots.title"] }));
    fireEvent.click(await screen.findByRole("button", { name: en["snapshots.deleteAll"] }));

    expect(await screen.findByText(/ALL local backups/)).toBeTruthy();
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["snapshots.deleteAll"] }));
    await waitFor(() => expect(deleteBackupsVM).toHaveBeenCalledWith(trueNasVM.libvirtName, "local"));
  });
});

// A VM that is no longer defined gets the same controls as a missing
// container: the schedule switch, and one removal button that reads "Delete
// all backups" while backups exist and "Remove entry" once there are none.
// Removing only the entry would hide existing backups from the page.
describe("VMRow when the VM is no longer defined", () => {
  // Display name and libvirt name differ on purpose, same as trueNasVM above.
  const orphan: VM = { ...trueNasVM, state: "not-installed", includeInSchedule: true };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("offers the schedule switch", () => {
    render(<VMRow vm={orphan} t={t} onRefresh={noop} onPlacement={noop} index={0} />);
    const sw = screen.getByRole("switch", { name: en["containers.includeInSchedule"] });
    expect(sw.getAttribute("aria-checked")).toBe("true");
  });

  it("saves the switch as a VM setting, under the libvirt name", async () => {
    render(<VMRow vm={orphan} t={t} onRefresh={noop} onPlacement={noop} index={0} />);
    const sw = screen.getByRole("switch", { name: en["containers.includeInSchedule"] });

    fireEvent.click(sw);

    await waitFor(() => expect(sw.getAttribute("aria-checked")).toBe("false"));
    expect(setVMInclude).toHaveBeenCalledWith(orphan.libvirtName, false);
  });

  it("offers Remove entry, not Delete all backups, when it has no backups", async () => {
    render(<VMRow vm={orphan} t={t} onRefresh={noop} onPlacement={noop} index={0} />);
    expect(screen.queryByRole("button", { name: "containers.deleteBackups" })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "vms.removeEntry" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: en["vms.removeEntry"] }));

    await waitFor(() => expect(forgetVM).toHaveBeenCalledWith(orphan.libvirtName));
    expect(deleteBackupsVM).not.toHaveBeenCalled();
  });

  it("offers Delete all backups, not Remove entry, when it has backups", async () => {
    render(<VMRow vm={{ ...orphan, lastBackup: 1_757_000_000 }} t={t} onRefresh={noop} onPlacement={noop} index={0} />);
    expect(screen.queryByRole("button", { name: "vms.removeEntry" })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "containers.deleteBackups" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: en["containers.deleteBackups"] }));

    await waitFor(() => expect(deleteBackupsVM).toHaveBeenCalledWith(orphan.libvirtName, "local"));
    expect(forgetVM).not.toHaveBeenCalled();
  });
});
