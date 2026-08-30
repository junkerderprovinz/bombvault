// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Button (#178) — the three promises the engine makes.
//
//   1. The width does not move between modes. jdp: "Alle buttons bleiben in
//      allen drei modi auch gleich breit." That is why the stage is derived
//      from the label rather than from what is painted.
//   2. A button always has an accessible name. Glyph mode is where an engine
//      like this normally introduces 197 unlabelled controls at once, so the
//      label is hidden visually and kept in the accessible tree, never removed.
//   3. When the text IS hidden, the name comes back on hover and on focus, in
//      the app's own `.glim-bubble` — never in the native `title=` balloon the
//      design language calls the anti-pattern.
// ---------------------------------------------------------------------------
import { useRef } from "react";
import { afterEach, beforeEach, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { Button } from "./Button";
import { setLabelMode } from "../lib/controls";

function renderButton(label = "Clear", withGlyph = true) {
  return render(
    <Button label={label} glyph={withGlyph ? <svg data-testid="g" /> : undefined} onClick={() => {}} />
  );
}

function stageClass(): string {
  const el = screen.getByRole("button");
  return [...el.classList].find((c) => /^bv-btn-(xs|sm|md|lg)$/.test(c)) ?? "";
}

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});

it("keeps the same width stage in all three modes", () => {
  // A label long enough to sit ABOVE the smallest stage, deliberately: with a
  // short one ("Clear"), a broken implementation that sized from the visible
  // text would still land on "xs" in every mode and this test would pass while
  // proving nothing. It has to be able to tell the two apart.
  const long = "Off-site-DR-Prüfung starten";
  const stages: string[] = [];
  for (const mode of ["text", "textGlyph", "glyph"] as const) {
    setLabelMode("buttons", mode);
    renderButton(long);
    stages.push(stageClass());
    cleanup();
  }
  // One distinct stage across all three: the mode changed what is SHOWN, never
  // how wide the control is.
  expect(new Set(stages).size).toBe(1);
  expect(stages[0]).toBe("bv-btn-lg");
});

it("still has an accessible name in glyph mode", () => {
  setLabelMode("buttons", "glyph");
  renderButton();
  // Found BY ITS NAME, which is the whole point: the text is invisible but a
  // screen reader still announces it.
  expect(screen.getByRole("button", { name: "Clear" })).toBeTruthy();
});

// The tooltip contract, after jdp's ruling on the contradiction glyph mode
// created: a button that hides its text says what it is in the app's REAL
// `.glim-bubble`, never in the native `title=` balloon that the design
// language calls the anti-pattern and lint-rules/icon-badge-needs-tooltip.js
// fails the build over. Glyph mode would otherwise have committed that
// anti-pattern on all 171 buttons at once, from one setting.
it("turns the label into a real bubble when the text is hidden, not a native title", () => {
  setLabelMode("buttons", "glyph");
  renderButton();
  const el = screen.getByRole("button");
  expect(el.getAttribute("title")).toBeNull();
  expect(document.querySelector(".glim-bubble")).toBeNull();
  fireEvent.mouseEnter(el);
  expect(document.querySelector(".glim-bubble")?.textContent).toBe("Clear");
  fireEvent.mouseLeave(el);
  expect(document.querySelector(".glim-bubble")).toBeNull();
});

// The native balloon never appeared on keyboard focus, which is half of why
// it had to go: a control reachable by Tab whose only explanation needs a
// mouse is not explained.
it("opens the same bubble on keyboard focus, not only on hover", () => {
  setLabelMode("buttons", "glyph");
  renderButton();
  const el = screen.getByRole("button");
  fireEvent.focus(el);
  expect(document.querySelector(".glim-bubble")?.textContent).toBe("Clear");
  fireEvent.blur(el);
  expect(document.querySelector(".glim-bubble")).toBeNull();
});

it("does not repeat the label as a tooltip while the text is visible", () => {
  setLabelMode("buttons", "textGlyph");
  renderButton();
  const el = screen.getByRole("button");
  expect(el.getAttribute("title")).toBeNull();
  fireEvent.mouseEnter(el);
  expect(document.querySelector(".glim-bubble")).toBeNull();
});

// `title` is the CHANGING half — why the button is unavailable, which job is
// holding it. It used to be the one thing the native balloon still carried;
// dropping it would have been a silent regression at 57 call sites.
it("shows the extra explanation in the bubble in every mode, joined to the name once the text is hidden", () => {
  for (const [mode, expected] of [
    ["textGlyph", "Another backup is running"],
    ["glyph", "Clear — Another backup is running"],
  ] as const) {
    cleanup();
    setLabelMode("buttons", mode);
    render(
      <Button label="Clear" glyph={<svg />} title="Another backup is running" onClick={() => {}} />
    );
    fireEvent.mouseEnter(screen.getByRole("button"));
    expect(document.querySelector(".glim-bubble")?.textContent).toBe(expected);
  }
});

// The one thing the native balloon did BETTER, and the reason the replacement
// needs a wrapper at all: a disabled <button> emits no mouse events and takes
// no focus, so its own handlers can never fire — at exactly the moment the
// user wants to know why it is dead. 54 of the 57 buttons carrying a `title`
// pass `disabled` too, so this is the common case, not a corner.
it("still reaches a DISABLED button's explanation, which the element itself cannot report", () => {
  setLabelMode("buttons", "text");
  const { container } = render(
    <Button label="Restore" title="Pick a snapshot first" disabled onClick={() => {}} />
  );
  const wrapper = container.firstElementChild as HTMLElement;
  expect(wrapper.tagName).toBe("SPAN");
  fireEvent.mouseEnter(wrapper);
  expect(document.querySelector(".glim-bubble")?.textContent).toBe("Pick a snapshot first");
});

// ...and no wrapper anywhere else, so nothing an ENABLED button sits in
// changes shape. FolderBrowser's two `w-full` rows are never disabled, which
// is what makes the wrapper safe to add at all.
it("wraps nothing when the button is enabled, or when a disabled one has nothing to say", () => {
  setLabelMode("buttons", "text");
  for (const props of [
    { title: "Pick a snapshot first" },
    { disabled: true },
  ] as const) {
    cleanup();
    const { container } = render(<Button label="Restore" onClick={() => {}} {...props} />);
    expect((container.firstElementChild as HTMLElement).tagName).toBe("BUTTON");
  }
});

it("shows a button's text in glyph mode when it has no glyph yet", () => {
  setLabelMode("buttons", "glyph");
  renderButton("Clear", false);
  // 146 of the app's buttons have no glyph yet. Until they do, a blank square
  // is the worse failure, so those fall back to their text.
  expect(screen.getByText("Clear").className).toContain("bv-btn-label");
});

it("gives a longer label a wider stage", () => {
  setLabelMode("buttons", "text");
  renderButton("Kijelölés törlése");
  expect(stageClass()).toBe("bv-btn-md");
});

// Tone -> classes. ConfirmDialog used to own this table and asserted the class
// names itself; it passes a tone NAME now, so the mapping is pinned here
// instead of quietly losing the coverage in the move.
//
// `danger` and `warn` deliberately use the SOLID status tokens over
// `carbon-background`: both themes' solid fail/warn values sit at the opposite
// lightness to that theme's own background, so one ink stays legible in both
// without a dedicated contrast token.
it("resolves each tone to its own fill", () => {
  setLabelMode("buttons", "text");
  for (const [tone, expected] of [
    ["accent", "bg-accent"],
    ["neutral", "bg-carbon-surface3"],
    ["danger", "bg-statusFailSolid"],
    ["warn", "bg-statusWarnSolid"],
  ] as const) {
    cleanup();
    render(<Button label="Delete" tone={tone} onClick={() => {}} />);
    expect(screen.getByRole("button").className).toContain(expected);
  }
});

it("keeps the destructive and the warning fills distinct", () => {
  setLabelMode("buttons", "text");
  // bv-convention-exception: no-status-color-on-control -- this is the TEST of
  // the tone table, not a call site painting a control. `warn` has to be named
  // literally here or there is nothing pinning it apart from `danger`; the rule
  // guards real UI, and its own message asks for this line rather than an
  // eslint-disable. (Pre-existing: the branch's `eslint src` failed on it
  // before this round touched the file, since the gate list only ran tsc and
  // vitest.)
  render(<Button label="Delete" tone="warn" onClick={() => {}} />);
  expect(screen.getByRole("button").className).not.toContain("bg-statusFailSolid");
});

// The chip variant (the remove control inside a pill). Its whole reason to
// exist is that it must NOT take a width stage - it sits inside a 0.75rem
// pill, and a stage would burst it - while still carrying a real accessible
// name, which is what four of these lost when they were first converted to a
// bare "x".
it("a chip takes no width stage and never shows its text", () => {
  for (const mode of ["text", "textGlyph", "glyph"] as const) {
    cleanup();
    setLabelMode("buttons", mode);
    render(<Button label="Remove plex" variant="chip" onClick={() => {}} />);
    const el = screen.getByRole("button");
    expect(el.className).toContain("bv-btn-chip");
    for (const stage of ["bv-btn-xs", "bv-btn-sm", "bv-btn-md", "bv-btn-lg"]) {
      expect(el.className).not.toContain(stage);
    }
    // Announced and on hover, never painted next to the thing it removes.
    expect(screen.getByText("Remove plex").className).toBe("sr-only");
    expect(el.getAttribute("title")).toBeNull();
    fireEvent.mouseEnter(el);
    expect(document.querySelector(".glim-bubble")?.textContent).toBe("Remove plex");
    fireEvent.mouseLeave(el);
  }
});

it("a chip carries the pill's own ink rather than painting a surface", () => {
  setLabelMode("buttons", "textGlyph");
  render(<Button label="Remove plex" variant="chip" onClick={() => {}} />);
  const cls = screen.getByRole("button").className;
  for (const fill of ["bg-carbon-surface3", "bg-accent", "bg-statusFailSolid"]) {
    expect(cls).not.toContain(fill);
  }
});

it("a chip still gets a glyph when the call site passes none", () => {
  setLabelMode("buttons", "text");
  render(<Button label="Remove plex" variant="chip" onClick={() => {}} />);
  expect(screen.getByRole("button").querySelector("svg")).toBeTruthy();
});

// The tooltip needs the button's own rect to place its bubble against, so
// `ref` is no longer forwarded straight through — it is merged with the
// tooltip's. Both shapes have a live caller and neither may be dropped:
// ErrorDetailPanel passes an OBJECT ref and calls `.current?.focus()` when the
// panel opens, which is what makes Escape and the focus trap behave.
it("still hands the element to the caller's own ref, object or callback", () => {
  setLabelMode("buttons", "text");
  let fromCallback: HTMLButtonElement | null = null;
  function Harness() {
    const object = useRef<HTMLButtonElement>(null);
    return (
      <>
        <Button label="Close" ref={object} onClick={() => object.current?.focus()} />
        <Button
          label="Cancel"
          ref={(el) => {
            fromCallback = el;
          }}
          onClick={() => {}}
        />
      </>
    );
  }
  render(<Harness />);
  const close = screen.getByRole("button", { name: "Close" });
  // Focusing THROUGH the object ref is the actual thing a dialog does on open.
  fireEvent.click(close);
  expect(document.activeElement).toBe(close);
  expect(fromCallback).toBe(screen.getByRole("button", { name: "Cancel" }));
});
