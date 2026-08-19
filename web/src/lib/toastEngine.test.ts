// ---------------------------------------------------------------------------
// toastEngine — pure state-transition tests (GlimStone form-engine Task 9).
//
// Every timestamp here is a plain injected `now` number, never a real clock
// or a fake-timer library — see toastEngine.ts's header comment for why this
// keeps the suite on the same `environment: "node"`, no-jsdom footing as the
// rest of this repo's tests (Toggle.test.ts etc.).
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import {
  TOAST_DURATION_MS,
  addToast,
  pauseToast,
  removeToast,
  resumeToast,
  shouldShowToast,
  type ToastEntry,
} from "./toastEngine";

describe("TOAST_DURATION_MS", () => {
  it("is a fixed 4 seconds, not an open-ended 'until dismissed' value", () => {
    expect(TOAST_DURATION_MS).toBe(4000);
  });
});

describe("addToast", () => {
  it("schedules expiresAt exactly durationMs after `now` (fixed duration)", () => {
    const list = addToast([], { id: "a", message: "Saved", severity: "success" }, 1_000_000);
    expect(list[0].expiresAt).toBe(1_000_000 + TOAST_DURATION_MS);
    expect(list[0].remainingMs).toBe(TOAST_DURATION_MS);
  });

  it("stacks onto the list rather than replacing an existing toast", () => {
    let list = addToast([], { id: "a", message: "First", severity: "success" }, 0);
    list = addToast(list, { id: "b", message: "Second", severity: "fail" }, 0);
    expect(list).toHaveLength(2);
    expect(list.map((t) => t.id)).toEqual(["a", "b"]);
    // The first toast's own fields are untouched by the second push.
    expect(list[0].message).toBe("First");
    expect(list[0].severity).toBe("success");
  });

  it("stacks three or more toasts, each keeping its own id/message/severity", () => {
    let list: ToastEntry[] = [];
    list = addToast(list, { id: "a", message: "One", severity: "success" }, 0);
    list = addToast(list, { id: "b", message: "Two", severity: "warn" }, 100);
    list = addToast(list, { id: "c", message: "Three", severity: "fail" }, 200);
    expect(list).toHaveLength(3);
    expect(list.map((t) => t.message)).toEqual(["One", "Two", "Three"]);
  });
});

describe("removeToast", () => {
  it("removes only the targeted toast, leaving the rest of the stack untouched", () => {
    let list = addToast([], { id: "a", message: "A", severity: "success" }, 0);
    list = addToast(list, { id: "b", message: "B", severity: "fail" }, 0);
    list = addToast(list, { id: "c", message: "C", severity: "warn" }, 0);
    const next = removeToast(list, "b");
    expect(next.map((t) => t.id)).toEqual(["a", "c"]);
  });

  it("is a no-op for an unknown id", () => {
    const list = addToast([], { id: "a", message: "A", severity: "success" }, 0);
    expect(removeToast(list, "does-not-exist")).toEqual(list);
  });
});

describe("pauseToast", () => {
  it("freezes remainingMs at exactly what's left, not the full duration", () => {
    // Pushed at t=0 (4000ms duration); hovered at t=1500 -> 2500ms should be left.
    const list = addToast([], { id: "a", message: "A", severity: "success" }, 0);
    const paused = pauseToast(list, "a", 1500);
    expect(paused[0].remainingMs).toBe(2500);
    expect(paused[0].expiresAt).toBeNull();
  });

  it("only pauses the targeted toast — sibling toasts in the stack keep running", () => {
    let list = addToast([], { id: "a", message: "A", severity: "success" }, 0);
    list = addToast(list, { id: "b", message: "B", severity: "success" }, 0);
    const paused = pauseToast(list, "a", 1000);
    const a = paused.find((t) => t.id === "a")!;
    const b = paused.find((t) => t.id === "b")!;
    expect(a.expiresAt).toBeNull();
    expect(b.expiresAt).toBe(TOAST_DURATION_MS); // untouched
  });

  it("is a no-op on an unknown id", () => {
    const list = addToast([], { id: "a", message: "A", severity: "success" }, 0);
    expect(pauseToast(list, "nope", 1000)).toEqual(list);
  });

  it("is a no-op on a toast that's already paused (never re-freezes against a stale null expiresAt)", () => {
    const list = addToast([], { id: "a", message: "A", severity: "success" }, 0);
    const once = pauseToast(list, "a", 1500); // remainingMs -> 2500
    const twice = pauseToast(once, "a", 3000); // should NOT recompute from null
    expect(twice[0].remainingMs).toBe(2500);
  });

  it("never goes negative even if pause is somehow called after the deadline", () => {
    const list = addToast([], { id: "a", message: "A", severity: "success" }, 0);
    const paused = pauseToast(list, "a", 9999);
    expect(paused[0].remainingMs).toBe(0);
  });
});

describe("resumeToast — the exact 'preserves remaining time, not reset' requirement", () => {
  it("restarts the countdown from the FROZEN remainder, not the full 4s", () => {
    // Push at t=0 (expires t=4000). Hover at t=1500 (2500ms left, frozen).
    let list = addToast([], { id: "a", message: "A", severity: "success" }, 0);
    list = pauseToast(list, "a", 1500);
    expect(list[0].remainingMs).toBe(2500);

    // A long, deliberately-past-the-original-deadline pause: real time keeps
    // moving (to t=9000, well past the original t=4000 expiry) while the
    // mouse just sits on the toast. If pause didn't actually freeze the
    // clock, this toast would already be "expired" by now.
    const resumed = resumeToast(list, "a", 9000);
    // Correct: 9000 + 2500 (the frozen remainder) = 11500.
    expect(resumed[0].expiresAt).toBe(11500);
    // Explicitly wrong behaviour this guards against: resetting to the full
    // duration would give 9000 + 4000 = 13000 instead.
    expect(resumed[0].expiresAt).not.toBe(9000 + TOAST_DURATION_MS);
  });

  it("is a no-op on a toast that's already running (not paused)", () => {
    const list = addToast([], { id: "a", message: "A", severity: "success" }, 0);
    const resumed = resumeToast(list, "a", 2000);
    expect(resumed[0].expiresAt).toBe(TOAST_DURATION_MS); // unchanged
  });

  it("is a no-op on an unknown id", () => {
    const list = addToast([], { id: "a", message: "A", severity: "success" }, 0);
    expect(resumeToast(list, "nope", 1000)).toEqual(list);
  });

  it("resuming one paused toast never affects a different paused toast in the same stack", () => {
    let list = addToast([], { id: "a", message: "A", severity: "success" }, 0);
    list = addToast(list, { id: "b", message: "B", severity: "success" }, 0);
    list = pauseToast(list, "a", 1000);
    list = pauseToast(list, "b", 2000);
    const resumed = resumeToast(list, "a", 5000);
    const a = resumed.find((t) => t.id === "a")!;
    const b = resumed.find((t) => t.id === "b")!;
    expect(a.expiresAt).toBe(5000 + 3000); // a's own frozen remainder (4000-1000)
    expect(b.expiresAt).toBeNull(); // b is still paused, untouched
  });
});

describe("shouldShowToast — severity-based quiet mode", () => {
  it("shows every severity when quiet mode is off", () => {
    expect(shouldShowToast("success", false)).toBe(true);
    expect(shouldShowToast("warn", false)).toBe(true);
    expect(shouldShowToast("fail", false)).toBe(true);
  });

  it("suppresses only routine 'success' completions when quiet mode is on", () => {
    expect(shouldShowToast("success", true)).toBe(false);
  });

  it("never suppresses failures or blocking (warn) toasts, even in quiet mode", () => {
    expect(shouldShowToast("fail", true)).toBe(true);
    expect(shouldShowToast("warn", true)).toBe(true);
  });
});
