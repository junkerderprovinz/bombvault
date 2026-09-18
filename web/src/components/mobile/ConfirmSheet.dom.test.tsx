// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act } from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { ConfirmSheet } from "./ConfirmSheet";
import { useConfirm, type ConfirmOptions } from "../../lib/useConfirm";
import { DESKTOP_QUERY } from "../../lib/useMediaQuery";
import { en, I18nProvider } from "../../lib/i18n";

// Behavioral proof of the confirm presentation swap: ONE useConfirm promise
// API, TWO presentation halves. The sheet half (ConfirmSheet) is asserted
// directly as observable behavior — stacked action order, tone mapping, close
// paths, focus discipline — and the swap itself is asserted through the real
// useConfirm() hook under a controlled matchMedia stub (below).
//
// Class-token assertions (bg-statusFailBg, bg-statusFailSolid, ...) follow
// the same documented exception as BottomSheet.dom.test.tsx: these are
// STYLING contracts and jsdom computes no geometry, so the observable form
// of the contract IS the token. Presence assertions, never whole-class
// snapshots.

const MESSAGE = "Delete container plex and everything in it? This cannot be undone.";

// ---------------------------------------------------------------------------
// Controlled matchMedia — why it must be built this way: useMediaQuery
// caches its desktop MediaQueryList at MODULE level on first use, so a
// per-test re-stub of window.matchMedia can never reach an already-created
// list. This stub is installed before the file's first render (beforeEach),
// so the hook caches THESE objects for the file's lifetime; the Map is
// deliberately never cleared between tests for the same reason. Flipping
// desktopMatches mutates the same objects the hook still reads (matches is
// a live getter) and setDesktopWidth() fires the change listeners — the
// exact sequence a real browser resize produces, including the
// useSyncExternalStore re-render.
// ---------------------------------------------------------------------------
interface FakeMql {
  matches: boolean;
  addEventListener(type: string, listener: () => void): void;
  removeEventListener(type: string, listener: () => void): void;
  addListener(listener: () => void): void;
  removeListener(listener: () => void): void;
}

const fakeMqls = new Map<string, FakeMql>();
const mqlListeners = new Map<string, Set<() => void>>();
let desktopMatches = true;

function fakeMql(query: string): FakeMql {
  const existing = fakeMqls.get(query);
  if (existing) return existing;
  const listeners = new Set<() => void>();
  // Only the width axis answers the desktop flag; every other query (the
  // coarse-pointer axis among them) answers false, like jsdom really would.
  const widthAxis = query === DESKTOP_QUERY;
  const m: FakeMql = {
    get matches() {
      return widthAxis ? desktopMatches : false;
    },
    addEventListener: (_type, listener) => listeners.add(listener),
    removeEventListener: (_type, listener) => listeners.delete(listener),
    addListener: (listener) => listeners.add(listener),
    removeListener: (listener) => listeners.delete(listener),
  };
  fakeMqls.set(query, m);
  mqlListeners.set(query, listeners);
  return m;
}

function setDesktopWidth(matches: boolean): void {
  desktopMatches = matches;
  for (const listener of mqlListeners.get(DESKTOP_QUERY) ?? []) listener();
}

describe("ConfirmSheet (the mobile presentation, direct)", () => {
  beforeEach(() => {
    desktopMatches = true;
    vi.stubGlobal("matchMedia", fakeMql);
  });
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  function renderSheet(tone?: "fail" | "warn") {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(
      <I18nProvider>
        <ConfirmSheet
          title={en["confirmDialog.title"]}
          message={MESSAGE}
          confirmLabel="Delete it"
          cancelLabel={en["common.cancel"]}
          tone={tone}
          onConfirm={onConfirm}
          onCancel={onCancel}
        />
      </I18nProvider>,
    );
    return { onConfirm, onCancel };
  }

  it("renders the fail-toned sheet with the destructive confirm stacked ABOVE the safe cancel", () => {
    renderSheet();
    const panel = screen.getByRole("dialog");
    // BottomSheet's tone surface: the sheet primitive's fail-tone panel.
    expect(panel.className).toContain("bg-statusFailBg");
    expect(panel.className).toContain("border-statusFailBorder");
    expect(screen.getByText(MESSAGE)).toBeTruthy();

    const confirmButton = screen.getByRole("button", { name: "Delete it" });
    const cancelButton = screen.getByRole("button", { name: en["common.cancel"] });
    // DOM order IS the stack order: cancel follows confirm, so the careless
    // tap swipe crosses the safe action's slot first (header comment).
    expect(
      confirmButton.compareDocumentPosition(cancelButton) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    // Tone mapping mirrors ConfirmDialog's footer: fail -> danger Button.
    expect(confirmButton.className).toContain("bg-statusFailSolid");
    expect(confirmButton.className).toContain("w-full");
    // The safe action carries NO status colour (neutral), never the danger.
    expect(cancelButton.className).not.toContain("statusFailSolid");
  });

  it("maps the warn tone to warn tokens on both the panel and the confirm control", () => {
    renderSheet("warn");
    const panel = screen.getByRole("dialog");
    expect(panel.className).toContain("bg-statusWarnBg");
    expect(panel.className).toContain("border-statusWarnBorder");
    expect(screen.getByRole("button", { name: "Delete it" }).className).toContain("bg-statusWarnSolid");
  });

  it("starts focus on the sheet's close control, never on the destructive one", () => {
    renderSheet();
    // BottomSheet's open effect focuses the header close button — the safe
    // outcome. No autoFocus anywhere in this tree can steal it.
    expect(document.activeElement).toBe(screen.getByRole("button", { name: en["common.close"] }));
    expect(document.activeElement).not.toBe(screen.getByRole("button", { name: "Delete it" }));
    expect(document.activeElement).not.toBe(screen.getByRole("button", { name: en["common.cancel"] }));
  });

  it("resolves every close path: confirm fires onConfirm; cancel, close, Escape and the scrim all cancel", () => {
    const { onConfirm, onCancel } = renderSheet();
    fireEvent.click(screen.getByRole("button", { name: "Delete it" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: en["common.cancel"] }));
    fireEvent.click(screen.getByRole("button", { name: en["common.close"] }));
    fireEvent.keyDown(document, { key: "Escape" });
    // The scrim is BottomSheet's aria-hidden sibling of the panel.
    const scrim = document.body.querySelector<HTMLElement>(":scope > [aria-hidden='true']");
    expect(scrim).not.toBeNull();
    fireEvent.click(scrim!);
    expect(onCancel).toHaveBeenCalledTimes(4);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });
});

// ---------------------------------------------------------------------------
// The swap itself, through the real hook: same confirm() call, presentation
// chosen by the width axis alone.
// ---------------------------------------------------------------------------
function ConfirmHarness({ options, results }: { options?: ConfirmOptions; results: boolean[] }) {
  const { confirm, confirmDialog } = useConfirm();
  return (
    <I18nProvider>
      <button onClick={() => void confirm(MESSAGE, options).then((r) => results.push(r))}>
        trigger
      </button>
      {confirmDialog}
    </I18nProvider>
  );
}

describe("useConfirm presentation swap", () => {
  beforeEach(() => {
    desktopMatches = true;
    vi.stubGlobal("matchMedia", fakeMql);
  });
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("at/above 48rem renders the ConfirmDialog card, unchanged", () => {
    const results: boolean[] = [];
    render(<ConfirmHarness results={results} />);
    fireEvent.click(screen.getByRole("button", { name: "trigger" })); // opens the confirmation
    const panel = screen.getByRole("dialog");
    // The desktop discriminators: described message + the neutral card
    // surface — never a tinted sheet panel.
    expect(panel.getAttribute("aria-describedby")).toBe("confirmdialog-message");
    expect(panel.className).toContain("bg-carbon-surface");
    expect(panel.className).not.toContain("statusFail");
  });

  it("below 48rem the same pending request renders the fail-toned ConfirmSheet", () => {
    const results: boolean[] = [];
    render(<ConfirmHarness results={results} />);
    fireEvent.click(screen.getByRole("button", { name: "trigger" })); // opens (desktop card first)
    act(() => setDesktopWidth(false));
    const panel = screen.getByRole("dialog");
    expect(panel.className).toContain("bg-statusFailBg");
    expect(panel.className).toContain("border-statusFailBorder");
    expect(panel.getAttribute("aria-describedby")).toBeNull();
    // Same translated labels on both faces — only the surface changed.
    expect(screen.getByRole("button", { name: en["common.confirm"] })).toBeTruthy();
    expect(screen.getByRole("button", { name: en["common.cancel"] })).toBeTruthy();
  });

  it("the sheet branch never default-focuses the destructive control", () => {
    render(<ConfirmHarness results={[]} />);
    fireEvent.click(screen.getByRole("button", { name: "trigger" })); // opens
    act(() => setDesktopWidth(false));
    // Initial focus is the header close button (the safe outcome); the
    // desktop card's autoFocus-Cancel parity holds through the same
    // mechanism. The confirm control is reachable but never given focus.
    expect(document.activeElement).toBe(screen.getByRole("button", { name: en["common.close"] }));
    expect(document.activeElement).not.toBe(
      screen.getByRole("button", { name: en["common.confirm"] }),
    );
  });

  it("confirm resolves true through the sheet", async () => {
    const results: boolean[] = [];
    render(<ConfirmHarness results={results} />);
    act(() => setDesktopWidth(false));
    const trigger = screen.getByRole("button", { name: "trigger" });
    trigger.focus();
    fireEvent.click(trigger); // opens the sheet
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["common.confirm"] }));
    });
    expect(results).toEqual([true]);
  });

  it("Escape resolves false EXACTLY once (the benign double-dispatch) and restores the trigger", async () => {
    const results: boolean[] = [];
    render(<ConfirmHarness results={results} />);
    act(() => setDesktopWidth(false));
    const trigger = screen.getByRole("button", { name: "trigger" });
    trigger.focus();
    fireEvent.click(trigger); // opens the sheet
    // One Escape keypress: useConfirm's document listener AND BottomSheet's
    // both fire. settle() nulls its resolver on the first call, so the
    // promise resolves once — results holds a single false, not two.
    await act(async () => {
      fireEvent.keyDown(document, { key: "Escape" });
    });
    expect(results).toEqual([false]);
    expect(document.activeElement).toBe(trigger);
  });

  it("a width flip WHILE pending swaps the presentation without losing the request", async () => {
    const results: boolean[] = [];
    render(<ConfirmHarness results={results} />);
    fireEvent.click(screen.getByRole("button", { name: "trigger" })); // opens as the desktop card
    expect(screen.getByRole("dialog").getAttribute("aria-describedby")).toBe("confirmdialog-message");
    act(() => setDesktopWidth(false)); // rotate/shrink mid-confirmation
    expect(screen.getByRole("dialog").className).toContain("bg-statusFailBg");
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["common.cancel"] }));
    });
    expect(results).toEqual([false]);
  });
});
