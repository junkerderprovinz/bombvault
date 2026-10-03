// @vitest-environment jsdom
// A bulk or stack restore can hit a container whose GPU or runtime this host
// lacks. Both offer the restore again without them, for those containers only.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { Container, Run } from "../lib/api";

class FakeEventSource {
  onmessage: ((ev: MessageEvent) => void) | null = null;
  close() {
    /* no-op */
  }
}
vi.stubGlobal("EventSource", FakeEventSource);
vi.stubGlobal(
  "fetch",
  vi.fn(async () => new Response(JSON.stringify({ ok: true }), { status: 200 })),
);

const REFUSED =
  'restore failed: the container used a GPU or runtime this host does not have: Error response from daemon: could not select device driver "" with capabilities: [[gpu]]';

let runs: Run[] = [];
let seq = 0;
function record(name: string, status: string, error = "") {
  seq++;
  runs = [
    {
      id: `r${seq}`,
      targetId: name,
      kind: "restore",
      status,
      startedAt: 1_700_000_000 + seq,
      finishedAt: 1_700_000_000 + seq,
      snapshotId: "",
      bytes: 0,
      error,
      target: name,
      domain: "container",
    },
    ...runs,
  ];
}

// plex fails for its GPU until it is restored without it; anything else restores.
const restore = vi.fn(async (name: string, ..._rest: unknown[]) => {
  const withoutRuntime = _rest[4] === true;
  record(name, name === "plex" && !withoutRuntime ? "failed" : "success", name === "plex" && !withoutRuntime ? REFUSED : "");
  return { ok: true, started: true };
});
const restoreStack = vi.fn(async () => {
  record("plexdb", "success");
  record("plex", "failed", REFUSED);
  return { ok: true, started: true };
});

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return {
    ...actual,
    listContainers: vi.fn(async () => ({ ok: true, containers: [member("plex"), member("plexdb")] })),
    listRuns: vi.fn(async () => ({ ok: true, runs })),
    restore: (name: string, ...rest: unknown[]) => restore(name, ...rest),
    restoreStack: () => restoreStack(),
    checkRestore: vi.fn(async () => ({ ok: true, ready: true, checks: [], plan: null })),
  };
});

function member(name: string): Container {
  return {
    name,
    image: `img/${name}:1`,
    state: "running",
    status: "",
    ip: "",
    installed: true,
    includeInSchedule: true,
    lastBackup: 1,
    lastBackupStarted: null,
    preHook: "",
    postHook: "",
    stopContainers: [],
    excludes: [],
    lastUpdateCheck: 0,
    lastUpdateResult: "",
    stack: "media",
  };
}

const { Containers, StackCard } = await import("./Containers");
const { en } = await import("../lib/i18n");
const { ToastProvider } = await import("../lib/toast");
const { AdvancedProvider } = await import("../lib/advanced");

const t = ((key: string) => en[key as keyof typeof en] ?? key) as unknown as Parameters<typeof StackCard>[0]["t"];

beforeEach(() => {
  runs = [];
  restore.mockClear();
  localStorage.setItem("bombvault.advanced", "1");
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});

async function retryWithoutRuntime() {
  const retry = await screen.findByRole("button", { name: en["restore.withoutRuntime"] }, { timeout: 12000 });
  expect(document.body.textContent).toContain(en["restore.noRuntimeList"].replace("{names}", "plex"));
  await act(async () => {
    fireEvent.click(retry);
  });
  await waitFor(() => expect(restore.mock.calls.some((c) => c[0] === "plex" && c[5] === true)).toBe(true));
  expect(restore.mock.calls.filter((c) => c[5] === true).map((c) => c[0])).toEqual(["plex"]);
}

describe("restoring several containers", () => {
  it("offers the containers refused for their GPU the restore without it", async () => {
    render(
      <ToastProvider>
        <AdvancedProvider>
          <Containers />
        </AdvancedProvider>
      </ToastProvider>,
    );
    fireEvent.click(await screen.findByRole("checkbox", { name: en["containers.selectAll"] }));
    fireEvent.click(screen.getByRole("button", { name: en["containers.restoreSelected"] }));
    await act(async () => {
      fireEvent.click(await screen.findByRole("button", { name: en["common.confirm"] }));
    });
    await retryWithoutRuntime();
  }, 30000);
});

describe("restoring a stack", () => {
  it("offers the members refused for their GPU the restore without it", async () => {
    render(
      <StackCard group={{ project: "media", members: [member("plex"), member("plexdb")] }} onRestored={() => undefined} t={t} index={0} />,
    );
    fireEvent.click(screen.getByRole("button", { name: en["stack.restore"] }));
    const button = screen.getAllByRole("button", { name: en["stack.restore"] }).at(-1)!;
    await act(async () => {
      fireEvent.click(button);
    });
    await act(async () => {
      fireEvent.click(await screen.findByRole("button", { name: en["common.confirm"] }));
    });
    await retryWithoutRuntime();
  }, 30000);
});
