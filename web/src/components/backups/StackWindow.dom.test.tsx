// @vitest-environment jsdom
// The stacks card and the window a stack is restored through: which stack and
// from where, the check before the restore, and the members coming back.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import type { BackupItem, CheckLine, Run } from "../../lib/api";
import type { StackGroup } from "../../lib/backupList";
import { countText, en } from "../../lib/i18n";
import { renderWithProviders } from "../../lib/placement.testsupport";
import { RESTORE_NO_RUNTIME } from "../../lib/runReason";
import { DESKTOP_QUERY } from "../../lib/useMediaQuery";

const calls = vi.hoisted(() => ({
  checkRestore: vi.fn(),
  restoreStack: vi.fn(),
  getStackDir: vi.fn(),
  listRuns: vi.fn(),
  listOffsiteTargets: vi.fn(),
}));

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...calls,
  getSettings: () => Promise.resolve({ ok: true, platform: "unraid" }),
}));

const { StacksCard, StackWindow } = await import("./StackWindow");

window.matchMedia = ((query: string) => ({
  matches: query === DESKTOP_QUERY,
  media: query,
  onchange: null,
  addListener: () => {},
  removeListener: () => {},
  addEventListener: () => {},
  removeEventListener: () => {},
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia;

const streams: FakeEventSource[] = [];
class FakeEventSource {
  onmessage: ((ev: MessageEvent<string>) => void) | null = null;
  constructor() {
    streams.push(this);
  }
  close(): void {}
}

function progress(frame: Record<string, unknown>) {
  act(() => {
    streams.at(-1)?.onmessage?.({ data: JSON.stringify(frame) } as MessageEvent<string>);
  });
}

function member(name: string, over: Partial<BackupItem> = {}): BackupItem {
  return {
    kind: "container",
    key: name,
    name,
    included: true,
    paused: false,
    effectiveSchedule: { kind: "everything", spec: "daily@03:00", alsoSpec: "" },
    lastBackup: 1,
    lastRunStatus: "success",
    sourceBytes: 1,
    runs: [],
    ...over,
  };
}

const STACKS: StackGroup[] = [
  { project: "immich", members: [member("immich"), member("immich_postgres", { dbDataCoverage: "live" })] },
  { project: "paperless", members: [member("paperless"), member("paperless-redis")] },
];

const PASSED: CheckLine[] = [
  { id: "repository", status: "ok" },
  { id: "space", status: "ok", need: 10, free: 100 },
];

function restoreRun(target: string, over: Partial<Run> = {}): Run {
  return { id: `new-${target}`, kind: "restore", domain: "container", target, status: "success", error: "", ...over } as Run;
}

const t = (key: keyof typeof en, n?: number) => countText(en[key], "en", n);
const onClose = vi.fn();
const onStarted = vi.fn();

async function open(initial = "immich") {
  await act(async () => {
    renderWithProviders(<StackWindow stacks={STACKS} initial={initial} onClose={onClose} onStarted={onStarted} />);
  });
}

const win = () => within(screen.getByRole("dialog", { name: "Restore stack" }));

async function press(name: string) {
  await act(async () => {
    fireEvent.click(win().getByRole("button", { name }));
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  streams.length = 0;
  localStorage.setItem("bv-lang", "en");
  globalThis.EventSource = FakeEventSource as unknown as typeof EventSource;
  calls.checkRestore.mockResolvedValue({ ok: true, ready: true, members: [{ name: "immich", ready: true, checks: PASSED }] });
  calls.restoreStack.mockResolvedValue({ ok: true, started: true });
  calls.getStackDir.mockResolvedValue({ ok: true, found: true });
  calls.listRuns.mockResolvedValue({ ok: true, runs: [] });
  calls.listOffsiteTargets.mockResolvedValue({ ok: true, targets: [] });
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("the stacks card", () => {
  it("names each stack with its members and offers to restore it", async () => {
    const onRestore = vi.fn();
    renderWithProviders(<StacksCard stacks={STACKS} onRestore={onRestore} t={t} />);
    const rows = within(screen.getByTestId("stacks")).getAllByRole("listitem");
    expect(rows.map((row) => row.textContent)).toEqual([
      "immich2 containers · immich, immich_postgresRestore stack",
      "paperless2 containers · paperless, paperless-redisRestore stack",
    ]);
    fireEvent.click(within(rows[1]).getByRole("button", { name: "Restore stack" }));
    expect(onRestore).toHaveBeenCalledWith("paperless");
  });
});

describe("the stack window", () => {
  it("opens on the stack it was asked for and lets another be chosen", async () => {
    await open("paperless");
    const chosen = () => win().getAllByRole("radio").map((r) => [r.textContent?.split(/\d/)[0], r.getAttribute("aria-checked")]);
    expect(chosen()).toEqual([
      ["immich", "false"],
      ["paperless", "true"],
    ]);
    fireEvent.click(win().getByRole("radio", { name: /^immich/ }));
    expect(chosen()[0]).toEqual(["immich", "true"]);
  });

  it("checks the restore before it asks, and names a member whose data is copied while the stack runs", async () => {
    await open();
    expect(calls.checkRestore).not.toHaveBeenCalled();
    await press("Next");
    await waitFor(() => expect(calls.checkRestore).toHaveBeenCalledTimes(1));
    expect(calls.checkRestore.mock.calls[0][0]).toEqual({ kind: "stack", name: "immich", source: "local" });
    expect(win().getByText(/^Restore all containers in this stack\?.*immich_postgres/)).toBeTruthy();
    expect(calls.restoreStack).not.toHaveBeenCalled();
  });

  it("restores the stack, started in order, and shows its members", async () => {
    await open();
    await press("Next");
    await waitFor(() => expect((win().getByRole("button", { name: "Restore stack" }) as HTMLButtonElement).disabled).toBe(false));
    await press("Restore stack");
    expect(calls.restoreStack).toHaveBeenCalledWith("immich", true, true, "local", undefined);
    expect(onStarted).toHaveBeenCalledTimes(1);
    expect(win().getAllByRole("listitem").map((row) => row.textContent)).toEqual(["immich", "immich_postgres"]);
    expect(win().getByText(en["restore.bgHint"])).toBeTruthy();
    expect(win().getByRole("button", { name: "Cancel restore" })).toBeTruthy();
  });

  it("leaves the members stopped when the order is switched off", async () => {
    await open();
    fireEvent.click(win().getByRole("switch", { name: en["stack.startInOrder"] }));
    await press("Next");
    await waitFor(() => expect((win().getByRole("button", { name: "Restore stack" }) as HTMLButtonElement).disabled).toBe(false));
    await press("Restore stack");
    expect(calls.restoreStack).toHaveBeenCalledWith("immich", false, true, "local", undefined);
  });

  it("holds the restore back while a member fails its check, and says which", async () => {
    calls.checkRestore.mockResolvedValue({
      ok: true,
      ready: false,
      members: [{ name: "immich", ready: false, checks: [{ id: "space", status: "fail", reason: "short", need: 1000, free: 10 }] }],
    });
    await open();
    await press("Next");
    await waitFor(() => expect(calls.checkRestore).toHaveBeenCalled());
    const start = win().getByRole("button", { name: /^Restore stack/ }) as HTMLButtonElement;
    await waitFor(() => expect(start.getAttribute("aria-label") ?? start.textContent).toBeTruthy());
    expect(start.disabled).toBe(true);
    await press("Back");
    expect(win().getAllByRole("radio")).toHaveLength(2);
  });

  it("asks about the project folder when the off-site source does not hold it", async () => {
    calls.listOffsiteTargets.mockResolvedValue({ ok: true, targets: [{ id: "t1", name: "Storage Box", repo: "sftp:box", enabled: true }] });
    calls.getStackDir.mockResolvedValue({ ok: true, found: false });
    await open();
    fireEvent.click(await win().findByRole("tab", { name: "Off-site" }));
    await press("Next");
    expect(calls.getStackDir).toHaveBeenCalledWith("immich", "offsite");
    expect(calls.checkRestore).not.toHaveBeenCalled();
    expect(win().getByText(/^The project folder is not in Storage Box\./)).toBeTruthy();
    expect((win().getByRole("button", { name: "Next" }) as HTMLButtonElement).disabled).toBe(true);

    await press("Use Local");
    await waitFor(() => expect((win().getByRole("button", { name: "Restore stack" }) as HTMLButtonElement).disabled).toBe(false));
    await press("Restore stack");
    expect(calls.restoreStack).toHaveBeenCalledWith("immich", true, true, "offsite", "local");
  });

  it("goes straight on when the off-site source holds the project folder", async () => {
    calls.listOffsiteTargets.mockResolvedValue({ ok: true, targets: [{ id: "t1", name: "Storage Box", repo: "sftp:box", enabled: true }] });
    await open();
    fireEvent.click(await win().findByRole("tab", { name: "Off-site" }));
    await press("Next");
    await waitFor(() => expect(calls.checkRestore).toHaveBeenCalled());
    expect(calls.checkRestore.mock.calls[0][0].source).toBe("offsite");
  });

  it("says why the restore did not start and stays on the check", async () => {
    calls.restoreStack.mockResolvedValue({ ok: false, error: "a restore is already running" });
    await open();
    await press("Next");
    await waitFor(() => expect((win().getByRole("button", { name: "Restore stack" }) as HTMLButtonElement).disabled).toBe(false));
    await press("Restore stack");
    expect(screen.getByText("a restore is already running")).toBeTruthy();
    expect(onStarted).not.toHaveBeenCalled();
    expect(win().getByRole("button", { name: "Back" })).toBeTruthy();
  });

  it("follows each member, says when the stack is back and offers a member Docker refused again", async () => {
    await open();
    await press("Next");
    await waitFor(() => expect((win().getByRole("button", { name: "Restore stack" }) as HTMLButtonElement).disabled).toBe(false));
    vi.useFakeTimers();
    await press("Restore stack");

    progress({ key: "container:immich", phase: "restore", percent: 40, active: true });
    expect(within(win().getAllByRole("listitem")[0]).getByRole("progressbar")).toBeTruthy();

    calls.listRuns.mockResolvedValue({
      ok: true,
      runs: [restoreRun("immich"), restoreRun("immich_postgres", { status: "failed", error: RESTORE_NO_RUNTIME })],
    });
    progress({ key: "container:immich", phase: "restore", percent: 100, active: false });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2500);
    });
    const rows = win().getAllByRole("listitem").map((row) => row.textContent);
    expect(rows).toEqual(["immichstarted", "immich_postgresFailed"]);
    expect(win().getByRole("button", { name: "Restore without GPU and runtime" })).toBeTruthy();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(9000);
    });
    expect(win().getByText(en["stack.restoreFinished"])).toBeTruthy();
    expect(win().queryByRole("button", { name: "Cancel restore" })).toBeNull();
    await press("Close");
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
