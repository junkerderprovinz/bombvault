// @vitest-environment jsdom
// A stack restore takes every member down, so before it asks it checks each
// member the way a single container restore is checked.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import type { Container } from "../lib/api";

class FakeEventSource {
  onmessage: ((ev: MessageEvent) => void) | null = null;
  close() {
    /* no-op */
  }
}
vi.stubGlobal("EventSource", FakeEventSource);

const ok = [
  { id: "repository", status: "ok" },
  { id: "key", status: "ok" },
  { id: "snapshot", status: "ok", detail: "aaaa1111" },
  { id: "space", status: "ok", need: 10, free: 100 },
];
const short = [...ok.slice(0, 3), { id: "space", status: "fail", reason: "short", need: 1000, free: 100 }];
const plan = (shared: { path: string; containers: string[] }[] = []) => ({
  inPlace: true,
  added: 1,
  changed: 0,
  unchanged: 0,
  extra: 0,
  files: [],
  listCapped: false,
  partial: false,
  missing: false,
  definition: [],
  shared,
});
const checkRestore = vi.fn();
const restoreStack = vi.fn(async () => ({ ok: true, started: true }));

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return {
    ...actual,
    checkRestore: (...a: unknown[]) => checkRestore(...a),
    restoreStack: (...a: unknown[]) => restoreStack(...(a as [])),
  };
});

const { StackCard } = await import("../components/containers/StackCard");
const { en } = await import("../lib/i18n");

const t = ((key: string) => en[key as keyof typeof en] ?? key) as unknown as Parameters<typeof StackCard>[0]["t"];

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

async function openAndRestore() {
  render(
    <StackCard group={{ project: "media", members: [member("plex"), member("plexdb")] }} onRestored={() => undefined} t={t} index={0} />
  );
  fireEvent.click(screen.getByRole("button", { name: en["stack.restore"] }));
  const button = screen.getAllByRole("button", { name: en["stack.restore"] }).at(-1)!;
  await act(async () => {
    fireEvent.click(button);
  });
  return waitFor(() => screen.getByRole("dialog"));
}

afterEach(() => {
  cleanup();
  checkRestore.mockReset();
  restoreStack.mockClear();
});

describe("the stack restore check", () => {
  it("checks every member and restores once the answer is yes", async () => {
    checkRestore.mockResolvedValue({
      ok: true,
      ready: true,
      checks: null,
      members: [
        { name: "plex", ready: true, checks: ok, plan: plan([{ path: "/mnt/user/appdata/plex", containers: ["tautulli"] }]) },
        { name: "plexdb", ready: true, checks: ok, plan: plan() },
      ],
    });
    const dialog = await openAndRestore();
    expect(checkRestore).toHaveBeenCalledWith({ kind: "stack", name: "media", source: "local" });
    expect(within(dialog).getByText("plexdb")).toBeTruthy();
    expect(within(dialog).getByText("/mnt/user/appdata/plex is also used by tautulli.")).toBeTruthy();
    await act(async () => {
      fireEvent.click(within(dialog).getByRole("button", { name: en["common.confirm"] }));
    });
    expect(restoreStack).toHaveBeenCalledTimes(1);
  });

  it("keeps Confirm locked and names the member whose check failed", async () => {
    checkRestore.mockResolvedValue({
      ok: true,
      ready: false,
      checks: null,
      members: [
        { name: "plex", ready: true, checks: ok, plan: plan() },
        { name: "plexdb", ready: false, checks: short, plan: plan() },
      ],
    });
    const dialog = await openAndRestore();
    const confirm = within(dialog).getByRole("button", { name: en["common.confirm"] });
    expect(confirm.hasAttribute("disabled")).toBe(true);
    const reason = en["restoreCheck.blockedMember"]
      .replace("{name}", "plexdb")
      .replace("{line}", en["restoreCheck.line.space"]);
    expect(within(dialog).getAllByLabelText(reason).length).toBeGreaterThan(0);
    expect(restoreStack).not.toHaveBeenCalled();
  });
});
