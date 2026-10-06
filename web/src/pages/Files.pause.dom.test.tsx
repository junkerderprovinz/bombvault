// @vitest-environment jsdom
// The Pause button on a folder set card writes the same flag as the set's
// "Include in schedule" switch, so the two and the Paused badge must follow
// either control, and "Back up all now" must leave a paused set out.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import type { FileSetView } from "../lib/api";
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

const fake = await vi.hoisted(async () => (await import("../lib/placement.testsupport")).createPlacementApi());
const server = vi.hoisted(() => ({ enabled: true }));

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  ...fake.api,
  getFileSetPreset: vi.fn(async () => ({ ok: true, offered: false, name: "", path: "", excludes: [] })),
  listFileSets: vi.fn(async () => ({ ok: true, fileSets: [photos(server.enabled)] })),
  patchFileSet: vi.fn(async (_id: string, patch: { enabled?: boolean }) => {
    if (patch.enabled !== undefined) server.enabled = patch.enabled;
    return { ok: true };
  }),
  backupFilesAll: vi.fn(async () => ({ ok: true })),
}));

function photos(enabled: boolean): FileSetView {
  return {
    id: "set-1",
    name: "Photos",
    path: "photos",
    excludes: [],
    enabled,
    lastBackup: 0,
    pathExists: true,
    placement: placementView(),
  };
}

const { patchFileSet, listFileSets } = await import("../lib/api");
const { Files } = await import("./Files");

const scheduleSwitch = () => screen.getByRole("switch", { name: "Include in schedule" });

beforeEach(() => {
  vi.clearAllMocks();
  fake.reset();
  server.enabled = true;
});
afterEach(cleanup);

describe("the Pause button on a folder set card", () => {
  it("takes the set out of the schedule and turns the switch off", async () => {
    renderWithProviders(<Files />);
    fireEvent.click(await screen.findByRole("button", { name: "Pause schedule" }));

    await waitFor(() => expect(patchFileSet).toHaveBeenCalledWith("set-1", { enabled: false }));
    expect(await screen.findByText("Schedule paused")).toBeTruthy();
    expect(scheduleSwitch().getAttribute("aria-checked")).toBe("false");
    expect(screen.getByRole("button", { name: "Resume schedule" })).toBeTruthy();
    // The reload brings the server's schedule sentence for the new value.
    await waitFor(() => expect(listFileSets).toHaveBeenCalledTimes(2));
  });

  it("puts a paused set back with Resume", async () => {
    server.enabled = false;
    renderWithProviders(<Files />);
    expect(await screen.findByText("Schedule paused")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Resume schedule" }));

    await waitFor(() => expect(patchFileSet).toHaveBeenCalledWith("set-1", { enabled: true }));
    await waitFor(() => expect(screen.queryByText("Schedule paused")).toBeNull());
    expect(scheduleSwitch().getAttribute("aria-checked")).toBe("true");
  });

  it("follows the switch", async () => {
    renderWithProviders(<Files />);
    await screen.findByRole("button", { name: "Pause schedule" });

    fireEvent.click(scheduleSwitch());

    expect(await screen.findByRole("button", { name: "Resume schedule" })).toBeTruthy();
    expect(screen.getByText("Schedule paused")).toBeTruthy();
  });

  it("leaves Back up now available and Back up all without the set", async () => {
    server.enabled = false;
    renderWithProviders(<Files />);
    await screen.findByText("Schedule paused");

    const backup = screen.getByRole("button", { name: /^Back up now/ }) as HTMLButtonElement;
    expect(backup.disabled).toBe(false);
    const all = screen.getByRole("button", { name: /^Back up all/ }) as HTMLButtonElement;
    expect(all.disabled).toBe(true);
  });
});
