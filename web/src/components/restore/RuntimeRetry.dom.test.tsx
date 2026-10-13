// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import type { Run } from "../../lib/api";
import { en } from "../../lib/i18n";

vi.mock("../../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../../lib/api")>("../../lib/api");
  return { ...actual, listRuns: vi.fn(), restore: vi.fn() };
});

import { listRuns, restore } from "../../lib/api";
import { RuntimeRetry } from "./RuntimeRetry";

const t = ((k: string) => en[k as keyof typeof en] ?? k) as never;

function run(status: string): Run {
  return {
    id: "new",
    targetId: "t",
    kind: "restore",
    status,
    startedAt: 0,
    finishedAt: 1,
    snapshotId: "",
    bytes: 0,
    error: "",
    acknowledged: false,
    target: "plex",
    domain: "container",
  };
}

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("RuntimeRetry", () => {
  it("frees its button when the restore is cancelled elsewhere", async () => {
    vi.useFakeTimers();
    vi.mocked(listRuns).mockResolvedValueOnce({ ok: true, runs: [] });
    vi.mocked(listRuns).mockResolvedValue({ ok: true, runs: [run("cancelled")] });
    vi.mocked(restore).mockResolvedValue({ ok: true, started: true } as never);
    const done = vi.fn();
    render(<RuntimeRetry names={["plex"]} onDone={done} t={t} />);
    fireEvent.click(screen.getByRole("button", { name: en["restore.withoutRuntime"] }));
    await act(() => vi.advanceTimersByTimeAsync(5000));
    expect(done).toHaveBeenCalledOnce();
    expect(document.body.textContent).toContain(en["containers.bulkResult"].replace("{ok}", "0").replace("{fail}", "1"));
  });
});
