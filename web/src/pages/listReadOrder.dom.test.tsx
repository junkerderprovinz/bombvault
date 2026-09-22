// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, screen, waitFor } from "@testing-library/react";
import type { Container, FileSetView, PlacementView, VM } from "../lib/api";
import { placementView, renderWithProviders } from "../lib/placement.testsupport";

class FakeEventSource {
  onmessage: ((ev: MessageEvent) => void) | null = null;
  close() {
    /* no-op */
  }
}
vi.stubGlobal("EventSource", FakeEventSource);

const fake = await vi.hoisted(async () => (await import("../lib/placement.testsupport")).createPlacementApi());
const lists = vi.hoisted(() => ({
  listContainers: vi.fn(),
  listVMs: vi.fn(),
  listFileSets: vi.fn(),
  getFileSetPreset: vi.fn(async () => ({ ok: true, offered: false, name: "", path: "", excludes: [] })),
}));

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  ...fake.api,
  ...lists,
}));

const { Containers } = await import("./Containers");
const { VMs } = await import("./VMs");
const { Files } = await import("./Files");
const { placementChanged } = await import("../lib/placementEvents");
const { reposChanged } = await import("../lib/useNamedRepos");

const follows = placementView({ homeFollows: true, copiesFollow: true });
const own = placementView({ homeFollows: true, copiesFollow: false });

function container(placement: PlacementView): Container {
  return {
    name: "nginx",
    image: "nginx:latest",
    state: "running",
    status: "Up 2 hours",
    ip: "",
    installed: true,
    includeInSchedule: true,
    lastBackup: null,
    lastBackupStarted: null,
    preHook: "",
    postHook: "",
    stopContainers: [],
    excludes: [],
    lastUpdateCheck: 0,
    lastUpdateResult: "",
    stack: "",
    placement,
  };
}

function vm(placement: PlacementView): VM {
  return {
    name: "Windows",
    libvirtName: "Windows",
    state: "running",
    method: "graceful",
    includeInSchedule: true,
    lastBackup: null,
    lastBackupStarted: null,
    placement,
  };
}

function fileSet(placement: PlacementView): FileSetView {
  return {
    id: "set-1",
    name: "Photos",
    path: "photos",
    excludes: [],
    enabled: true,
    lastBackup: 0,
    pathExists: true,
    placement,
  };
}

interface Reads {
  /** Answers the read that started nth, counting the page's first as zero. */
  answer(nth: number, res: unknown): void;
  started(): number;
}

function listReads(fn: ReturnType<typeof vi.fn>): Reads {
  const waiting: ((res: unknown) => void)[] = [];
  fn.mockImplementation(() => new Promise((resolve) => waiting.push(resolve)));
  return {
    answer: (nth, res) => waiting[nth]?.(res),
    started: () => waiting.length,
  };
}

// A repo change starts a read, the placement write announces itself and starts
// another, and the first one answers last with what it saw before the write.
async function landOutOfOrder(reads: Reads, older: unknown, fresh: unknown): Promise<void> {
  await waitFor(() => expect(screen.getByText("Follows the default")).toBeTruthy());
  act(() => reposChanged());
  act(() => placementChanged());
  await waitFor(() => expect(reads.started()).toBe(3));
  await act(async () => reads.answer(2, fresh));
  await act(async () => reads.answer(1, older));
}

describe("a list answer older than a placement write", () => {
  beforeEach(() => {
    fake.reset();
    for (const fn of Object.values(lists)) fn.mockClear();
  });
  afterEach(cleanup);

  it("does not put a container card back on what it saw", async () => {
    const reads = listReads(lists.listContainers);
    renderWithProviders(<Containers />);
    reads.answer(0, { ok: true, containers: [container(follows)] });
    await landOutOfOrder(reads, { ok: true, containers: [container(follows)] }, { ok: true, containers: [container(own)] });
    expect(screen.getByText("Own copies")).toBeTruthy();
  });

  it("does not put a VM card back on what it saw", async () => {
    const reads = listReads(lists.listVMs);
    renderWithProviders(<VMs />);
    reads.answer(0, { ok: true, vms: [vm(follows)] });
    await landOutOfOrder(reads, { ok: true, vms: [vm(follows)] }, { ok: true, vms: [vm(own)] });
    expect(screen.getByText("Own copies")).toBeTruthy();
  });

  it("does not put a folder set card back on what it saw", async () => {
    const reads = listReads(lists.listFileSets);
    renderWithProviders(<Files />);
    reads.answer(0, { ok: true, fileSets: [fileSet(follows)] });
    await landOutOfOrder(reads, { ok: true, fileSets: [fileSet(follows)] }, { ok: true, fileSets: [fileSet(own)] });
    expect(screen.getByText("Own copies")).toBeTruthy();
  });
});
