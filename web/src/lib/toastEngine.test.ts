// Every timestamp here is an injected `now`, never a real clock or fake timers,
// so the suite runs in the plain node environment.
import { describe, expect, it } from "vitest";
import {
  ACTION_TOAST_DURATION_MS,
  MAX_VISIBLE_TOASTS,
  NO_ENGAGEMENT,
  TOAST_DURATION_MS,
  addToast,
  applyEngagement,
  pauseToast,
  removeToast,
  resumeToast,
  shouldShowToast,
  type ToastEngagement,
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

  it("keeps a toast that offers an action for the longer action duration", () => {
    const action = { label: "Undo", onClick: () => {} };
    const list = addToast([], { id: "a", message: "Linked", severity: "success", action }, 1_000_000);
    expect(list[0].expiresAt).toBe(1_000_000 + ACTION_TOAST_DURATION_MS);
    expect(list[0].remainingMs).toBe(ACTION_TOAST_DURATION_MS);
    expect(ACTION_TOAST_DURATION_MS).toBeGreaterThan(TOAST_DURATION_MS);
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

  it("never grows past MAX_VISIBLE_TOASTS", () => {
    let list: ToastEntry[] = [];
    // One more push than the cap allows.
    for (let i = 0; i < MAX_VISIBLE_TOASTS + 1; i++) {
      list = addToast(list, { id: `t${i}`, message: `Msg ${i}`, severity: "success" }, i);
    }
    expect(list).toHaveLength(MAX_VISIBLE_TOASTS);
  });

  it("drops the oldest toasts to make room, never the one just pushed", () => {
    let list: ToastEntry[] = [];
    for (let i = 0; i < MAX_VISIBLE_TOASTS + 1; i++) {
      list = addToast(list, { id: `t${i}`, message: `Msg ${i}`, severity: "success" }, i);
    }
    // t0 is gone; every toast from t1 on, the newest included, survived.
    expect(list.map((t) => t.id)).toEqual(
      Array.from({ length: MAX_VISIBLE_TOASTS }, (_, i) => `t${i + 1}`)
    );
    expect(list.some((t) => t.id === "t0")).toBe(false);
    expect(list.some((t) => t.id === `t${MAX_VISIBLE_TOASTS}`)).toBe(true);
  });

  it("keeps dropping the oldest as pushes keep coming, well past a single overflow", () => {
    let list: ToastEntry[] = [];
    const total = MAX_VISIBLE_TOASTS * 3;
    for (let i = 0; i < total; i++) {
      list = addToast(list, { id: `t${i}`, message: `Msg ${i}`, severity: "success" }, i);
    }
    expect(list).toHaveLength(MAX_VISIBLE_TOASTS);
    // Only the tail end (the most recent MAX_VISIBLE_TOASTS pushes) remains.
    expect(list.map((t) => t.id)).toEqual(
      Array.from({ length: MAX_VISIBLE_TOASTS }, (_, i) => `t${total - MAX_VISIBLE_TOASTS + i}`)
    );
  });

  it("respects a custom maxVisible override without touching the exported default", () => {
    let list: ToastEntry[] = [];
    list = addToast(list, { id: "a", message: "A", severity: "success" }, 0, TOAST_DURATION_MS, 2);
    list = addToast(list, { id: "b", message: "B", severity: "success" }, 0, TOAST_DURATION_MS, 2);
    list = addToast(list, { id: "c", message: "C", severity: "success" }, 0, TOAST_DURATION_MS, 2);
    expect(list.map((t) => t.id)).toEqual(["b", "c"]);
  });

  it("drops nothing while under the cap", () => {
    let list: ToastEntry[] = [];
    for (let i = 0; i < MAX_VISIBLE_TOASTS; i++) {
      list = addToast(list, { id: `t${i}`, message: `Msg ${i}`, severity: "success" }, i);
    }
    expect(list).toHaveLength(MAX_VISIBLE_TOASTS);
    expect(list.map((t) => t.id)).toEqual(Array.from({ length: MAX_VISIBLE_TOASTS }, (_, i) => `t${i}`));
  });

  it("skips a paused toast when making room", () => {
    let list: ToastEntry[] = [];
    for (let i = 0; i < MAX_VISIBLE_TOASTS; i++) {
      list = addToast(list, { id: `t${i}`, message: `Msg ${i}`, severity: "success" }, i);
    }
    // Hovering or tabbing into the oldest toast pauses it, and plain
    // drop-oldest would evict exactly the toast the user is reading.
    list = pauseToast(list, "t0", 10);
    list = addToast(list, { id: "new", message: "New", severity: "success" }, 20);
    expect(list).toHaveLength(MAX_VISIBLE_TOASTS);
    expect(list.map((t) => t.id)).toEqual(["t0", "t2", "t3", "new"]);
    // t0 is still paused and still holding its frozen remainder.
    expect(list[0].expiresAt).toBeNull();
  });

  it("drops the oldest when every older toast is paused, and the newest still survives", () => {
    let list: ToastEntry[] = [];
    for (let i = 0; i < MAX_VISIBLE_TOASTS; i++) {
      list = addToast(list, { id: `t${i}`, message: `Msg ${i}`, severity: "success" }, i);
    }
    for (let i = 0; i < MAX_VISIBLE_TOASTS; i++) list = pauseToast(list, `t${i}`, 10);
    list = addToast(list, { id: "new", message: "New", severity: "success" }, 20);
    expect(list).toHaveLength(MAX_VISIBLE_TOASTS);
    expect(list.map((t) => t.id)).toEqual(["t1", "t2", "t3", "new"]);
  });

  it("drops several at once past the cap, still preferring running toasts over paused ones", () => {
    let list: ToastEntry[] = [];
    for (let i = 0; i < 6; i++) {
      list = addToast(list, { id: `t${i}`, message: `Msg ${i}`, severity: "success" }, i, TOAST_DURATION_MS, 6);
    }
    list = pauseToast(list, "t1", 10);
    // Back down to the real cap of 4: 3 must go, and t1 must not be one of them.
    list = addToast(list, { id: "new", message: "New", severity: "success" }, 20);
    expect(list.map((t) => t.id)).toEqual(["t1", "t4", "t5", "new"]);
  });
});

describe("applyEngagement", () => {
  it("reports engaged while either hover OR focus is active", () => {
    expect(applyEngagement(NO_ENGAGEMENT, "hover", true)).toEqual({ next: { hover: true, focus: false }, engaged: true });
    expect(applyEngagement(NO_ENGAGEMENT, "focus", true)).toEqual({ next: { hover: false, focus: true }, engaged: true });
  });

  it("stays engaged when the mouse leaves a toast the keyboard is still inside", () => {
    // hover -> Tab in -> mouse away. The countdown must not resume: focus is
    // still on the dismiss button, and resuming would dismiss the toast out
    // from under the user, snapping activeElement back to <body>.
    let e = applyEngagement(NO_ENGAGEMENT, "hover", true);
    e = applyEngagement(e.next, "focus", true);
    e = applyEngagement(e.next, "hover", false);
    expect(e).toEqual({ next: { hover: false, focus: true }, engaged: true });
  });

  it("stays engaged in the mirror order: focus, hover, then un-hover", () => {
    let e = applyEngagement(NO_ENGAGEMENT, "focus", true);
    e = applyEngagement(e.next, "hover", true);
    e = applyEngagement(e.next, "hover", false);
    expect(e.engaged).toBe(true);
  });

  it("disengages only once both have ended, whichever ends last", () => {
    let e = applyEngagement(NO_ENGAGEMENT, "hover", true);
    e = applyEngagement(e.next, "focus", true);
    e = applyEngagement(e.next, "hover", false);
    e = applyEngagement(e.next, "focus", false);
    expect(e).toEqual({ next: { hover: false, focus: false }, engaged: false });
  });

  it("disengages on a plain hover-in and hover-out with no focus involved", () => {
    const inn = applyEngagement(NO_ENGAGEMENT, "hover", true);
    expect(inn.engaged).toBe(true);
    const out = applyEngagement(inn.next, "hover", false);
    expect(out).toEqual({ next: { hover: false, focus: false }, engaged: false });
  });

  it("is idempotent for a repeated edge (e.g. moving the mouse onto the card's own dismiss button)", () => {
    const once = applyEngagement(NO_ENGAGEMENT, "hover", true);
    const twice = applyEngagement(once.next, "hover", true);
    expect(twice).toEqual(once);
  });

  it("never mutates the state it was handed", () => {
    const prev: ToastEngagement = { hover: true, focus: false };
    applyEngagement(prev, "focus", true);
    expect(prev).toEqual({ hover: true, focus: false });
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

  it("pauses only the targeted toast while its siblings keep running", () => {
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
    const twice = pauseToast(once, "a", 3000); // must not recompute from null
    expect(twice[0].remainingMs).toBe(2500);
  });

  it("never goes negative even if pause is somehow called after the deadline", () => {
    const list = addToast([], { id: "a", message: "A", severity: "success" }, 0);
    const paused = pauseToast(list, "a", 9999);
    expect(paused[0].remainingMs).toBe(0);
  });
});

describe("resumeToast", () => {
  it("restarts the countdown from the frozen remainder, not the full 4s", () => {
    // Push at t=0 (expires t=4000). Hover at t=1500 (2500ms left, frozen).
    let list = addToast([], { id: "a", message: "A", severity: "success" }, 0);
    list = pauseToast(list, "a", 1500);
    expect(list[0].remainingMs).toBe(2500);

    // Resume at t=9000, well past the original t=4000 expiry. Had the pause
    // not frozen the clock, the toast would already have expired.
    const resumed = resumeToast(list, "a", 9000);
    // 9000 + 2500 (the frozen remainder) = 11500.
    expect(resumed[0].expiresAt).toBe(11500);
    // Resetting to the full duration would give 9000 + 4000 = 13000.
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

describe("shouldShowToast", () => {
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
