// @vitest-environment jsdom
// The Pause button on a container card writes the same "include in schedule"
// flag as the switch on that card, so the two, the Paused badge and the
// phone's scheduled count must all follow either control.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import type { Container } from "../lib/api";
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
  listContainers: vi.fn(async () => ({ ok: true, containers: [nginx(server.include)] })),
  setInclude: vi.fn(async (_name: string, include: boolean) => {
    server.include = include;
    return { ok: true };
  }),
}));

function nginx(includeInSchedule: boolean): Container {
  return {
    name: "nginx",
    image: "nginx:latest",
    state: "running",
    status: "Up 2 hours",
    ip: "",
    installed: true,
    includeInSchedule,
    lastBackup: null,
    lastBackupStarted: null,
    preHook: "",
    postHook: "",
    stopContainers: [],
    excludes: [],
    lastUpdateCheck: 0,
    lastUpdateResult: "",
    stack: "",
    placement: placementView(),
  };
}

const { setInclude } = await import("../lib/api");
const { Containers } = await import("./Containers");

const scheduleSwitch = () => screen.getByRole("switch", { name: "Include in schedule" });

beforeEach(() => {
  vi.clearAllMocks();
  fake.reset();
  server.include = true;
  desktop = true;
});
afterEach(cleanup);

describe("the Pause button on a container card", () => {
  it("takes the container out of the schedule and turns the switch off", async () => {
    renderWithProviders(<Containers />);
    fireEvent.click(await screen.findByRole("button", { name: "Pause schedule" }));

    await waitFor(() => expect(setInclude).toHaveBeenCalledWith("nginx", false));
    expect(await screen.findByText("Schedule paused")).toBeTruthy();
    expect(scheduleSwitch().getAttribute("aria-checked")).toBe("false");
    expect(screen.getByRole("button", { name: "Resume schedule" })).toBeTruthy();
  });

  it("puts a paused container back with Resume", async () => {
    server.include = false;
    renderWithProviders(<Containers />);
    expect(await screen.findByText("Schedule paused")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Resume schedule" }));

    await waitFor(() => expect(setInclude).toHaveBeenCalledWith("nginx", true));
    await waitFor(() => expect(screen.queryByText("Schedule paused")).toBeNull());
    expect(scheduleSwitch().getAttribute("aria-checked")).toBe("true");
    expect(screen.getByRole("button", { name: "Pause schedule" })).toBeTruthy();
  });

  it("follows the switch", async () => {
    renderWithProviders(<Containers />);
    await screen.findByRole("button", { name: "Pause schedule" });

    fireEvent.click(scheduleSwitch());

    expect(await screen.findByRole("button", { name: "Resume schedule" })).toBeTruthy();
    expect(screen.getByText("Schedule paused")).toBeTruthy();
    expect(setInclude).toHaveBeenCalledWith("nginx", false);
  });

  it("leaves Back up now available on a paused container", async () => {
    server.include = false;
    renderWithProviders(<Containers />);
    await screen.findByText("Schedule paused");

    const backup = screen.getByRole("button", { name: /^Back up now/ }) as HTMLButtonElement;
    expect(backup.disabled).toBe(false);
  });
});

describe("pausing a container on a phone", () => {
  it("marks the list card and drops it from the scheduled count", async () => {
    desktop = false;
    renderWithProviders(<Containers />);
    expect(await screen.findByText("1 Containers · 1 Scheduled")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: /^nginx/ }));
    const backup = (await screen.findByRole("button", { name: /^Back up now/ })) as HTMLButtonElement;
    fireEvent.click(screen.getByRole("button", { name: "Pause schedule" }));
    await waitFor(() => expect(setInclude).toHaveBeenCalledWith("nginx", false));
    expect(await screen.findByText("Schedule paused")).toBeTruthy();
    expect(backup.disabled).toBe(false);

    fireEvent.click(screen.getByRole("button", { name: /Back$/ }));
    expect(await screen.findByText("1 Containers")).toBeTruthy();
    const card = screen.getByRole("button", { name: /^nginx/ });
    expect(within(card).getByText("Schedule paused")).toBeTruthy();
  });
});
