// @vitest-environment jsdom
// The Pause button on a VM card writes the same "include in schedule" flag as
// the switch on that card, so the two, the Paused badge and the phone's
// scheduled count must all follow either control.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import type { VM } from "../lib/api";
import { DESKTOP_QUERY } from "../lib/useMediaQuery";
import { placementView, renderWithProviders } from "../lib/placement.testsupport";

class FakeEventSource {
  onmessage: ((ev: MessageEvent) => void) | null = null;
  close() {
    /* no-op */
  }
}
vi.stubGlobal("EventSource", FakeEventSource);
// Anything the page asks for that a test does not name answers with an empty ok.
vi.stubGlobal(
  "fetch",
  vi.fn(async () => new Response(JSON.stringify({ ok: true }), { status: 200 })),
);

let desktop = true;
window.matchMedia = ((query: string) => ({
  get matches() {
    return query === DESKTOP_QUERY ? desktop : false;
  },
  media: query,
  onchange: null,
  addListener: () => {},
  removeListener: () => {},
  addEventListener: () => {},
  removeEventListener: () => {},
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia;

const fake = await vi.hoisted(async () => (await import("../lib/placement.testsupport")).createPlacementApi());
const server = vi.hoisted(() => ({ include: true }));

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  ...fake.api,
  getSettings: vi.fn(async () => ({ ok: true, settings: { vmsEnabled: true } })),
  listVMs: vi.fn(async () => ({ ok: true, vms: [windows(server.include)] })),
  setVMInclude: vi.fn(async (_name: string, include: boolean) => {
    server.include = include;
    return { ok: true };
  }),
}));

function windows(includeInSchedule: boolean): VM {
  return {
    name: "Windows 11",
    libvirtName: "win11",
    state: "running",
    method: "graceful",
    includeInSchedule,
    lastBackup: null,
    lastBackupStarted: null,
    placement: placementView(),
  };
}

const { setVMInclude } = await import("../lib/api");
const { VMs } = await import("./VMs");

const scheduleSwitch = () => screen.getByRole("switch", { name: "Include in schedule" });

beforeEach(() => {
  vi.clearAllMocks();
  fake.reset();
  server.include = true;
  desktop = true;
});
afterEach(cleanup);

describe("the Pause button on a VM card", () => {
  it("takes the VM out of the schedule by its libvirt name and turns the switch off", async () => {
    renderWithProviders(<VMs />);
    fireEvent.click(await screen.findByRole("button", { name: "Pause schedule" }));

    await waitFor(() => expect(setVMInclude).toHaveBeenCalledWith("win11", false));
    expect(await screen.findByText("Schedule paused")).toBeTruthy();
    expect(scheduleSwitch().getAttribute("aria-checked")).toBe("false");
    expect(screen.getByRole("button", { name: "Resume schedule" })).toBeTruthy();
  });

  it("puts a paused VM back with Resume", async () => {
    server.include = false;
    renderWithProviders(<VMs />);
    expect(await screen.findByText("Schedule paused")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Resume schedule" }));

    await waitFor(() => expect(setVMInclude).toHaveBeenCalledWith("win11", true));
    await waitFor(() => expect(screen.queryByText("Schedule paused")).toBeNull());
    expect(scheduleSwitch().getAttribute("aria-checked")).toBe("true");
  });

  it("follows the switch", async () => {
    renderWithProviders(<VMs />);
    await screen.findByRole("button", { name: "Pause schedule" });

    fireEvent.click(scheduleSwitch());

    expect(await screen.findByRole("button", { name: "Resume schedule" })).toBeTruthy();
    expect(screen.getByText("Schedule paused")).toBeTruthy();
  });

  it("leaves Back up now available on a paused VM", async () => {
    server.include = false;
    renderWithProviders(<VMs />);
    await screen.findByText("Schedule paused");

    const backup = screen.getByRole("button", { name: /^Back up now/ }) as HTMLButtonElement;
    expect(backup.disabled).toBe(false);
  });
});

describe("pausing a VM on a phone", () => {
  it("marks the list card and drops it from the scheduled count", async () => {
    desktop = false;
    renderWithProviders(<VMs />);
    expect(await screen.findByText("1 VMs · 1 Scheduled")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: /^Windows 11/ }));
    const backup = (await screen.findByRole("button", { name: /^Back up now/ })) as HTMLButtonElement;
    fireEvent.click(screen.getByRole("button", { name: "Pause schedule" }));
    await waitFor(() => expect(setVMInclude).toHaveBeenCalledWith("win11", false));
    expect(await screen.findByText("Schedule paused")).toBeTruthy();
    expect(backup.disabled).toBe(false);

    fireEvent.click(screen.getByRole("button", { name: /Back$/ }));
    expect(await screen.findByText("1 VMs")).toBeTruthy();
    const card = screen.getByRole("button", { name: /^Windows 11/ });
    expect(within(card).getByText("Schedule paused")).toBeTruthy();
  });
});
