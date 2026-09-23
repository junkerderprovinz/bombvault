// @vitest-environment jsdom
import { useRef } from "react";
import { afterEach, beforeEach, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { Button } from "./Button";
import { setLabelMode } from "../lib/controls";

function renderButton(label = "Clear", withGlyph = true) {
  return render(
    <Button label={label} glyph={withGlyph ? <svg data-testid="g" /> : undefined} onClick={() => {}} />
  );
}

function stageClass(): string {
  const el = screen.getByRole("button");
  return [...el.classList].find((c) => /^glim-btn-(xs|sm|md|lg)$/.test(c)) ?? "";
}

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});

it("keeps the same width stage in the two modes that show text", () => {
  // A short label lands on "xs" whether the stage comes from the label or from
  // the visible text, so only a long one tells the two apart.
  const long = "Off-site-DR-Prüfung starten";
  const stages: string[] = [];
  for (const mode of ["text", "textGlyph"] as const) {
    setLabelMode("buttons", mode);
    renderButton(long);
    stages.push(stageClass());
    cleanup();
  }
  expect(new Set(stages).size).toBe(1);
  expect(stages[0]).toBe("glim-btn-lg");
});

it("takes no stage at all in the two modes that hide text", () => {
  for (const mode of ["glyph", "reactive"] as const) {
    cleanup();
    setLabelMode("buttons", mode);
    renderButton("Off-site-DR-Prüfung starten");
    // No floor means the button is as wide as its glyph, and in reactive mode
    // it can grow out of that as the words arrive.
    expect(stageClass()).toBe("");
  }
});

it("still has an accessible name in glyph mode", () => {
  setLabelMode("buttons", "glyph");
  renderButton();
  expect(screen.getByRole("button", { name: "Clear" })).toBeTruthy();
});

// A hidden label goes into `.glim-bubble`, not a native `title=`, which
// lint-rules/icon-badge-needs-tooltip.js rejects.
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

it("opens the same bubble on keyboard focus, not only on hover", () => {
  setLabelMode("buttons", "glyph");
  renderButton();
  const el = screen.getByRole("button");
  fireEvent.keyDown(document.body, { key: "Tab" });
  act(() => el.focus());
  expect(document.querySelector(".glim-bubble")?.textContent).toBe("Clear");
  act(() => el.blur());
  expect(document.querySelector(".glim-bubble")).toBeNull();
});

// Focus after a pointer press comes from a click, or from a dialog handing it
// back after one. No keyboard user asked, so no bubble.
it("does not open on focus when the pointer was used last", () => {
  setLabelMode("buttons", "glyph");
  renderButton();
  const el = screen.getByRole("button");
  fireEvent.pointerDown(el);
  act(() => el.focus());
  expect(document.querySelector(".glim-bubble")).toBeNull();
});

// A button disabled under its open bubble is swapped for a new element inside
// the wrapper, and the old one never reports losing the focus or the pointer.
it("closes an open bubble when the button is disabled under it, and again when it comes back", () => {
  setLabelMode("buttons", "text");
  const props = { label: "Prune", title: "Apply your retention policy", onClick: () => {} };
  const { rerender } = render(<Button {...props} />);
  fireEvent.keyDown(document.body, { key: "Tab" });
  act(() => screen.getByRole("button").focus());
  expect(document.querySelector(".glim-bubble")?.textContent).toBe("Apply your retention policy");

  rerender(<Button {...props} disabled busy />);
  expect(document.querySelector(".glim-bubble"), "the bubble outlived the button it belonged to").toBeNull();

  // Opened again on the disabled one, through the wrapper as a pointer would...
  fireEvent.mouseEnter(screen.getByRole("button").parentElement as HTMLElement);
  expect(document.querySelector(".glim-bubble")).not.toBeNull();
  // ...and the job ending swaps the element back, which closes it too.
  rerender(<Button {...props} />);
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

// `title` carries the part that changes, such as why the button is unavailable.
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

// A disabled <button> fires no mouse events and takes no focus, so the bubble
// hangs off a wrapping span instead. Most buttons with a `title` are disabled.
it("reaches a disabled button's explanation through a wrapping span", () => {
  setLabelMode("buttons", "text");
  const { container } = render(
    <Button label="Restore" title="Pick a snapshot first" disabled onClick={() => {}} />
  );
  const wrapper = container.firstElementChild as HTMLElement;
  expect(wrapper.tagName).toBe("SPAN");
  fireEvent.mouseEnter(wrapper);
  expect(document.querySelector(".glim-bubble")?.textContent).toBe("Pick a snapshot first");
});

// A wrapper would break a `w-full` button such as FolderBrowser's rows, which
// are never disabled.
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

it("shows a button's text in glyph mode when it has no glyph", () => {
  setLabelMode("buttons", "glyph");
  renderButton("Clear", false);
  expect(screen.getByText("Clear").className).toContain("glim-btn-label");
});

it("gives a longer label a wider stage", () => {
  setLabelMode("buttons", "text");
  renderButton("Kijelölés törlése");
  expect(stageClass()).toBe("glim-btn-md");
});

// `danger` and `warn` use the solid status tokens: in both themes they sit at
// the opposite lightness to the background, so one ink reads on either.
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
  // bv-convention-exception: no-status-color-on-control -- a test of the tone
  // table, which has to name `warn` to pin it apart from `danger`.
  render(<Button label="Delete" tone="warn" onClick={() => {}} />);
  expect(screen.getByRole("button").className).not.toContain("bg-statusFailSolid");
});

// The chip variant is the remove control inside a 0.75rem pill. A width stage
// would burst the pill, and it still needs an accessible name.
it("a chip takes no width stage and never shows its text", () => {
  for (const mode of ["text", "textGlyph", "glyph"] as const) {
    cleanup();
    setLabelMode("buttons", mode);
    render(<Button label="Remove plex" variant="chip" onClick={() => {}} />);
    const el = screen.getByRole("button");
    expect(el.className).toContain("glim-btn-chip");
    for (const stage of ["glim-btn-xs", "glim-btn-sm", "glim-btn-md", "glim-btn-lg"]) {
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

// Button merges the caller's ref with the tooltip's own. ErrorDetailPanel
// focuses its button through an object ref when it opens, and its focus trap
// depends on that.
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
  fireEvent.click(close);
  expect(document.activeElement).toBe(close);
  expect(fromCallback).toBe(screen.getByRole("button", { name: "Cancel" }));
});
