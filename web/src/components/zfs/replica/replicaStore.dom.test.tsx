// @vitest-environment jsdom
// One answer per item, shared by every part of the page that shows the
// replica. It has to end up on what the server holds after the last change,
// however slow a read is and however a run or a bring back ends.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import type { ProgressMap } from "../../../lib/progress";
import type { ZFSReplica } from "../../../lib/api";
import { replica } from "./replica.testsupport";

let answers: Array<(value: ZFSReplica) => void> = [];
let progress: ProgressMap = {};

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    getZFSReplica: () => new Promise<ZFSReplica>((resolve) => answers.push(resolve)),
  };
});

vi.mock("../../../lib/progress", () => ({ useProgress: () => progress }));

const { useReplica } = await import("./replicaStore");

let reloadIt: () => void = () => undefined;

function Probe() {
  const { replica: value, reload } = useReplica("zfs1");
  reloadIt = reload;
  return <span data-testid="state">{value ? `${value.state}:${value.keep.preset}` : "none"}</span>;
}

async function answer(value: ZFSReplica) {
  const next = answers.shift();
  if (!next) throw new Error("no read is waiting");
  await act(async () => next(value));
}

function frame(active: boolean) {
  return { phase: "replicate" as const, percent: active ? 40 : 100, active: true, finished: active ? undefined : true, lastSeen: Date.now() };
}

beforeEach(() => {
  answers = [];
  progress = {};
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("replica store", () => {
  it("reads again after a change that came in while a read was on its way", async () => {
    render(<Probe />);
    await act(async () => undefined);
    await answer(replica());
    expect(screen.getByTestId("state").textContent).toBe("ok:own");

    await act(async () => reloadIt());
    expect(answers).toHaveLength(1);
    await act(async () => reloadIt());
    await answer(replica({ keep: { preset: "balanced", own: [0, 7, 3, 0, 0] } }));
    // The answer that left before the second change is not shown over it.
    expect(screen.getByTestId("state").textContent).toBe("ok:own");
    await act(async () => undefined);
    expect(answers).toHaveLength(1);
    await answer(replica({ keep: { preset: "long", own: [0, 7, 3, 0, 0] } }));
    expect(screen.getByTestId("state").textContent).toBe("ok:long");
  });

  it("reads again when a bring back ends", async () => {
    const { rerender } = render(<Probe />);
    await act(async () => undefined);
    await answer(replica({ state: "running" }));
    progress = { "zfs-replica-restore:zfs1": frame(true) };
    rerender(<Probe />);
    progress = {};
    rerender(<Probe />);
    await act(async () => undefined);
    await answer(replica({ state: "ok" }));
    expect(screen.getByTestId("state").textContent).toBe("ok:own");
  });

  it("keeps asking while the server still reports a run that has stopped reporting", async () => {
    vi.useFakeTimers();
    render(<Probe />);
    await act(async () => undefined);
    await answer(replica({ state: "running" }));
    expect(answers).toHaveLength(0);
    await act(async () => vi.advanceTimersByTime(2500));
    expect(answers).toHaveLength(1);
    await answer(replica({ state: "failed" }));
    await act(async () => vi.advanceTimersByTime(5000));
    expect(answers).toHaveLength(0);
    expect(screen.getByTestId("state").textContent).toBe("failed:own");
  });
});
