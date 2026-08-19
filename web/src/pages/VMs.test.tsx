// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Pins the fix for the severe VM-identifier regression: VMRow's action
// buttons (Backup Now here) must send VM.libvirtName — the raw libvirt
// domain name — to the backend, NEVER VM.name (display-only; on TrueNAS it is
// the resolved friendly name, not something virsh has ever heard of). Before
// this fix VMView/VM had only `name`, so every action call site had no choice
// but to send the display value, and virsh rejected it as "domain not found"
// on every TrueNAS VM whose friendly name differs from its raw name.
//
// This is the first component-level (jsdom) test in this repo — everything
// else under src/**/*.test.ts is pure-logic, node-environment (see
// vitest.config.ts). A cross-stack identifier/display-name bug like this one
// lives entirely in how a component wires a prop into an API call, which a
// pure-logic test cannot observe; hence the jsdom opt-in here.
// ---------------------------------------------------------------------------
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { VMRow } from "./VMs";
import type { VM } from "../lib/api";

// useProgress() (lib/progress.ts) opens a real EventSource on mount; jsdom
// does not implement it. A minimal stub is all the hook touches
// (.onmessage, .close()).
class FakeEventSource {
  onmessage: ((ev: MessageEvent) => void) | null = null;
  close() {
    /* no-op */
  }
}

vi.stubGlobal("EventSource", FakeEventSource);

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return {
    ...actual,
    // useBackupWatch's fire() snapshots listRuns() before AND polls it after
    // start() — stub it to an empty, always-ok list so the watch never blocks
    // on a real network call.
    listRuns: vi.fn(async () => ({ ok: true, runs: [] })),
    backupVMNow: vi.fn(async () => ({ ok: true, started: true })),
  };
});

// Imported AFTER vi.mock so this binding is the mocked function.
import { backupVMNow } from "../lib/api";

const noop = () => {
  /* no-op */
};
// VMRow only needs t() to return a stable string per key; none of this test's
// assertions depend on real translations.
const t = ((key: string) => key) as unknown as Parameters<typeof VMRow>[0]["t"];

// A TrueNAS-shaped VM: the display name and the raw libvirt identifier
// deliberately differ, exactly the case that exposed the bug.
const trueNasVM: VM = {
  name: "debian",
  libvirtName: "550e8400-e29b-41d4-a716-446655440000",
  state: "running",
  method: "graceful",
  includeInSchedule: false,
  lastBackup: null,
  lastBackupStarted: null,
};

afterEach(() => {
  cleanup();
});

describe("VMRow action wiring", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sends VM.libvirtName to backupVMNow, never the display VM.name", async () => {
    render(<VMRow vm={trueNasVM} t={t} onRefresh={noop} index={0} />);

    fireEvent.click(screen.getByText("containers.backupNow"));

    await waitFor(() => expect(backupVMNow).toHaveBeenCalled());
    expect(backupVMNow).toHaveBeenCalledWith(trueNasVM.libvirtName);
    expect(backupVMNow).not.toHaveBeenCalledWith(trueNasVM.name);
  });

  it("still shows the display name to the user, not the raw identifier", () => {
    render(<VMRow vm={trueNasVM} t={t} onRefresh={noop} index={0} />);
    expect(screen.getByText(trueNasVM.name)).toBeTruthy();
    expect(screen.queryByText(trueNasVM.libvirtName)).toBeNull();
  });
});
