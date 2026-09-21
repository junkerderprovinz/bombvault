// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// useVisibilityGate; jsdom behavior proofs for the visibility pause gate (the
// SSE/poll reconciliation fix).
//
// document.visibilityState is shadowed with an own-property getter (jsdom
// never flips it by itself) and each transition is driven the way a real
// browser drives it: a visibilitychange event on the document. The hook is
// the real useSyncExternalStore wiring; no re-render is faked, so a test
// that passes here means the consumer render path flips on the real event.
//
// The windowless branch is pinned as a source assert, not a runtime render:
// window.document is an [Unforgeable] property in jsdom (non-configurable),
// so no runtime test can make `typeof document` read "undefined" in a DOM
// environment. Reading the module source is the repo's established doctrine
// for exactly this (useMediaQuery.test.ts: "Node environment, no DOM: this
// reads source text, it does not render").
// ---------------------------------------------------------------------------
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";
import { isPageVisible, useVisibilityGate } from "./useVisibilityGate";

// vitest's jsdom environment rewrites import.meta.url to the jsdom origin (a
// non-file: URL), so the node-env resolution pattern (useMediaQuery.test.ts's
// fileURLToPath(new URL(".", import.meta.url))) cannot be copied here. Vitest
// always runs from web/ (package.json scripts, ci, and every documented
// invocation), so resolving from the process cwd is stable.
const source = readFileSync(join(process.cwd(), "src", "lib", "useVisibilityGate.ts"), "utf8");

// Shadow document.visibilityState with an own getter and announce the change
// the way the browser does. act() so React flushes the store subscription
// before the test reads the hook's value.
function setPageVisibility(state: "visible" | "hidden"): void {
  Object.defineProperty(document, "visibilityState", {
    configurable: true,
    get: () => state,
  });
  act(() => {
    document.dispatchEvent(new Event("visibilitychange"));
  });
}

afterEach(() => {
  // Deleting the own property restores the prototype getter (jsdom's default
  // "visible") for the next test.
  delete (document as { visibilityState?: unknown }).visibilityState;
  cleanup();
});

describe("useVisibilityGate", () => {
  it("reads visible as the default in a live document", () => {
    const { result } = renderHook(() => useVisibilityGate());
    expect(result.current).toBe(true);
    // The plain read non-React consumers use agrees with the hook.
    expect(isPageVisible()).toBe(true);
  });

  it("flips false on hidden and true again on visible, driven by the real event", () => {
    const { result } = renderHook(() => useVisibilityGate());
    expect(result.current).toBe(true);

    setPageVisibility("hidden");
    expect(result.current).toBe(false);
    expect(isPageVisible()).toBe(false);

    setPageVisibility("visible");
    expect(result.current).toBe(true);
    expect(isPageVisible()).toBe(true);
  });

  it("respects an initial hidden state (a page already backgrounded at mount)", () => {
    // Shadow before the first render; no event, no act: the hook's very first
    // snapshot must already read hidden, or a consumer mounted on a background
    // tab would subscribe its SSE machinery before the first flip.
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => "hidden",
    });
    const { result } = renderHook(() => useVisibilityGate());
    expect(result.current).toBe(false);
  });

  it("keeps multiple hook instances on one answer (one shared axis)", () => {
    const a = renderHook(() => useVisibilityGate());
    const b = renderHook(() => useVisibilityGate());
    expect(a.result.current).toBe(true);
    expect(b.result.current).toBe(true);
    setPageVisibility("hidden");
    expect(a.result.current).toBe(false);
    expect(b.result.current).toBe(false);
  });

  it("pins the windowless default to visible (source assert; window.document is Unforgeable in jsdom)", () => {
    // The plain read: the guard must come first (typeof check) and default to
    // true before any document access.
    expect(source).toMatch(/if \(typeof document === "undefined"\) return true;/);
    // The hook's server snapshot must agree; useSyncExternalStore's third
    // slot is the windowless answer React would use off-client.
    expect(source).toMatch(/useSyncExternalStore\(\s*subscribeVisibility,\s*getVisibilitySnapshot,\s*\(\) => true,?\s*\)/);
  });
});
