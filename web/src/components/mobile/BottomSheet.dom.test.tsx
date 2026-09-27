// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { BottomSheet, type BottomSheetProps } from "./BottomSheet";
import { en, I18nProvider } from "../../lib/i18n";

// Behavioral proof for the BottomSheet primitive: every mechanism lifted
// from useConfirm.tsx is asserted here as observable behavior (queries +
// fired events), never as class-string snapshots:
//   (a) portal into document.body, nothing rendered when closed
//   (b) document-level Escape closes
//   (c) scrim click (target === currentTarget) closes; a click originating
//       inside the panel does not (possible at all only because scrim and
//       panel are siblings; see BottomSheet.tsx's header deviation note)
//   (d) the close button is found by its accessible name (common.close) and
//       closes; the ConfirmDialog strict-mode discipline
//   (e) Tab/Shift+Tab wrap over the full FOCUSABLE_SELECTOR candidate set
//       (header close button + body controls) in both directions
//   (f) focus is captured from the trigger at open (and moved inside, the
//       ConfirmDialog autoFocus parity) and restored to the trigger on close
//
// The second describe asserts the additive capabilities the same way, with
// one exception to the no-class-snapshot rule: fullHeight, the footer, the
// inset-clamped padding and the key-height close are styling contracts, and
// jsdom computes no geometry, so the observable form of "the panel is h-dvh"
// is the class token. These tests assert the presence of a load-bearing
// token, never a whole className string.
//
// The suite is self-sufficient: BottomSheet touches no matchMedia or
// visualViewport API, so it passes whether or not the vitest setupFiles
// matchMedia stub is installed.

function SheetHarness({
  onClose,
  initialOpen = false,
}: {
  onClose: () => void;
  initialOpen?: boolean;
}) {
  const [open, setOpen] = useState(initialOpen);
  return (
    <I18nProvider>
      <button onClick={() => setOpen(true)}>trigger</button>
      <BottomSheet
        open={open}
        onClose={() => {
          onClose();
          setOpen(false);
        }}
        // The production sheet title, taken straight from the en table: the
        // same single nav.more key the More trigger uses.
        title={en["nav.more"]}
      >
        <button>body one</button>
        <button>body two</button>
      </BottomSheet>
    </I18nProvider>
  );
}

describe("BottomSheet", () => {
  afterEach(cleanup);

  it("renders the dialog into document.body via the portal, not the test container", () => {
    const { container, unmount } = render(<SheetHarness onClose={vi.fn()} initialOpen />);
    const panel = screen.getByRole("dialog");
    // The panel lives under <body> directly, outside RTL's container: any
    // ancestor with a CSS transform would otherwise trap the fixed backdrop
    // (useConfirm.tsx's portal rationale).
    expect(container.querySelector('[role="dialog"]')).toBeNull();
    expect(panel.parentElement).toBe(document.body);
    unmount();
  });

  it("renders nothing when closed", () => {
    render(<SheetHarness onClose={vi.fn()} />);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryByRole("button", { name: en["common.close"] })).toBeNull();
  });

  it("closes on a document-level Escape keydown", () => {
    const onClose = vi.fn();
    render(<SheetHarness onClose={onClose} initialOpen />);
    // Fired on `document` itself: the listener must work no matter where
    // focus currently sits (the documented-broken inline onKeyDown does not).
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("closes on a scrim click but not on a click originating inside the panel", () => {
    const onClose = vi.fn();
    render(<SheetHarness onClose={onClose} initialOpen />);
    // The scrim is the aria-hidden sibling of the panel under <body>.
    const scrim = document.body.querySelector<HTMLElement>(":scope > [aria-hidden='true']");
    expect(scrim).not.toBeNull();
    fireEvent.click(scrim!);
    expect(onClose).toHaveBeenCalledTimes(1);
    // A click on a control inside the panel bubbles up through the panel;
    // and never reaches the scrim's handler, because the panel is not a child
    // of the scrim. This is the target === currentTarget guard doing its job.
    const onClose2 = vi.fn();
    cleanup();
    render(<SheetHarness onClose={onClose2} initialOpen />);
    fireEvent.click(screen.getByRole("button", { name: "body one" }));
    expect(onClose2).not.toHaveBeenCalled();
    // ...and the sheet is still open after the interior click.
    expect(screen.getByRole("dialog")).not.toBeNull();
  });

  it("closes via the header close button found by its accessible name", () => {
    const onClose = vi.fn();
    render(<SheetHarness onClose={onClose} initialOpen />);
    const close = screen.getByRole("button", { name: en["common.close"] });
    fireEvent.click(close);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("moves focus inside the sheet on open", () => {
    render(<SheetHarness onClose={vi.fn()} initialOpen />);
    // ConfirmDialog parity: focus starts inside the aria-modal surface (its
    // Cancel button; the sheet's safe equivalent is the header close button).
    expect(document.activeElement).toBe(screen.getByRole("button", { name: en["common.close"] }));
  });

  it("wraps Tab and Shift+Tab focus inside the sheet in both directions", () => {
    render(<SheetHarness onClose={vi.fn()} initialOpen />);
    const close = screen.getByRole("button", { name: en["common.close"] });
    const two = screen.getByRole("button", { name: "body two" });
    // Tab from the last focusable wraps to the first (the header close button).
    two.focus();
    fireEvent.keyDown(document, { key: "Tab" });
    expect(document.activeElement).toBe(close);
    // Shift+Tab from the first focusable wraps to the last.
    close.focus();
    fireEvent.keyDown(document, { key: "Tab", shiftKey: true });
    expect(document.activeElement).toBe(two);
    // Note: forward movement from a mid-list control is native browser Tab
    // navigation (the trap handler only acts at the edges, per the verbatim
    // useConfirm lift), which jsdom does not implement; so only the two
    // wrap directions are assertable here, which is exactly the trap's
    // contract.
  });

  it("captures focus from the trigger at open and restores it on close", () => {
    const onClose = vi.fn();
    render(<SheetHarness onClose={onClose} />);
    const trigger = screen.getByRole("button", { name: "trigger" });
    trigger.focus();
    fireEvent.click(trigger); // opens the sheet
    expect(document.activeElement).toBe(screen.getByRole("button", { name: en["common.close"] }));
    fireEvent.keyDown(document, { key: "Escape" }); // closes via Escape
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(document.activeElement).toBe(trigger); // restored
  });

  it("leaves focus alone when another surface has already taken it", () => {
    render(
      <I18nProvider>
        <button>trigger</button>
        <button>the surface that took over</button>
        <BottomSheet open={false} onClose={vi.fn()} title={en["nav.more"]}>
          <button>body one</button>
        </BottomSheet>
      </I18nProvider>,
    );
    const trigger = screen.getByRole("button", { name: "trigger" });
    trigger.focus();
    const { rerender } = render(
      <I18nProvider>
        <button>trigger</button>
        <BottomSheet open onClose={vi.fn()} title={en["nav.more"]}>
          <button>body one</button>
        </BottomSheet>
      </I18nProvider>,
    );
    // A replacement modal focuses its own control during the commit, which is
    // what a width flip mid-confirmation does. Unmounting the sheet must not
    // drag focus back to the trigger behind it.
    const successor = screen.getByRole("button", { name: "the surface that took over" });
    successor.focus();
    rerender(
      <I18nProvider>
        <button>trigger</button>
      </I18nProvider>,
    );
    expect(document.activeElement).toBe(successor);
  });

  it("answers Tab in the newest sheet only, so two open sheets do not fight", () => {
    render(
      <I18nProvider>
        <BottomSheet open onClose={vi.fn()} title={en["nav.more"]}>
          <button>under one</button>
          <button>under two</button>
        </BottomSheet>
        <BottomSheet open onClose={vi.fn()} title={en["run.detailTitle"] ?? "Run"}>
          <button>over one</button>
          <button>over two</button>
        </BottomSheet>
      </I18nProvider>,
    );
    const overOne = screen.getByRole("button", { name: "over one" });
    overOne.focus();
    // Focus sits inside the top sheet and not on its last control, so the trap
    // has nothing to do and the browser's own Tab has to go through. A second
    // live listener would cancel it and pull focus into the sheet underneath,
    // which is what pins a keyboard user to one control.
    const wentThrough = fireEvent.keyDown(document, { key: "Tab" });
    expect(wentThrough).toBe(true);
    expect(document.activeElement).toBe(overOne);
  });
});

// ---------------------------------------------------------------------------
// Extension contracts. PropsHarness forwards the additive props onto an
// already-open sheet; the capabilities are consumed with the sheet open,
// exactly as RunDetailSheet (fullHeight and footer) and ConfirmSheet mount
// them.
// ---------------------------------------------------------------------------
function PropsHarness({ sheetProps }: { sheetProps: Partial<BottomSheetProps> }) {
  return (
    <I18nProvider>
      <BottomSheet open onClose={vi.fn()} title={en["nav.more"]} {...sheetProps}>
        <p>body content</p>
      </BottomSheet>
    </I18nProvider>
  );
}

/** The header div is the close button's parent; the scroll body is its next
 *  sibling; the panel's structural order, readable without testids. */
function headerAndBody(): { header: HTMLElement; body: HTMLElement; panel: HTMLElement } {
  const panel = screen.getByRole("dialog");
  const header = screen.getByRole("button", { name: en["common.close"] }).parentElement as HTMLElement;
  const body = header.nextElementSibling as HTMLElement;
  return { panel, header, body };
}

describe("BottomSheet extensions", () => {
  afterEach(cleanup);

  it("keeps the capped panel by default and switches to h-dvh only with fullHeight", () => {
    // Default (MoreSheet's mount): the 85dvh cap, never the full-height class.
    const { unmount } = render(<PropsHarness sheetProps={{}} />);
    let panel = screen.getByRole("dialog");
    expect(panel.className).toContain("max-h-[85dvh]");
    expect(panel.className).not.toContain("h-dvh");
    unmount();

    // The run-detail variant: h-dvh replaces the cap.
    render(<PropsHarness sheetProps={{ fullHeight: true }} />);
    panel = screen.getByRole("dialog");
    expect(panel.className).toContain("h-dvh");
    expect(panel.className).not.toContain("max-h-[85dvh]");
  });

  it("clamps header and body side padding to the safe-area insets", () => {
    render(<PropsHarness sheetProps={{}} />);
    const { header, body } = headerAndBody();
    // Each physical side clamps against its own inset (max(1rem, inset)):
    // content clears a landscape display cutout on either rotation.
    for (const el of [header, body]) {
      expect(el.className).toContain("pl-[max(1rem,var(--safe-area-left))]");
      expect(el.className).toContain("pr-[max(1rem,var(--safe-area-right))]");
    }
    // The body keeps its bottom safe-area padding while no footer follows
    // (this harness renders none): the body is the sheet's last surface, so
    // the home-indicator inset is its to carry.
    expect(body.className).toContain("pb-[var(--safe-area-bottom)]");
  });

  it("puts the close button on the key-control height stage (glim-btn-key)", () => {
    render(<PropsHarness sheetProps={{}} />);
    const close = screen.getByRole("button", { name: en["common.close"] });
    // .glim-btn sets its height outside any utility layer, so a h-11/w-11
    // utility loses the cascade and the rendered box stays 32px tall. The
    // key stage is the height that wins, and glim-btn-icon squares the box
    // at that height (the ConfirmSheet buttons' stage). Asserted as the
    // class token; jsdom computes no geometry.
    expect(close.className).toContain("glim-btn-key");
    expect(close.className).not.toContain("h-11");
    expect(close.className).not.toContain("w-11");
  });

  it("renders the close control as an engine Button", () => {
    // The header close is the shared Button component, not a hand-rolled
    // <button>: glim-btn is the one class every engine button carries, so
    // its presence pins the swap; the tone table, the tooltip mechanism and
    // the --motion-press-scale press all arrive with it.
    render(<PropsHarness sheetProps={{}} />);
    const close = screen.getByRole("button", { name: en["common.close"] });
    expect(close.className).toContain("glim-btn");
  });

  it("keeps the one carbon-surface panel, never a status tint or a hairline", () => {
    render(<PropsHarness sheetProps={{}} />);
    const panel = screen.getByRole("dialog");
    expect(panel.className).toContain("bg-carbon-surface");
    expect(panel.className).not.toContain("statusFail");
    expect(panel.className).not.toContain("statusWarn");
    expect(panel.className).not.toMatch(/\bborder\b/);
  });

  it("renders an optional footer after the scroll body, chrome-styled and safe-area padded", () => {
    // Default: no footer element at all; the panel is header + body only.
    const { unmount } = render(<PropsHarness sheetProps={{}} />);
    const { panel } = headerAndBody();
    expect(panel.childElementCount).toBe(2);
    unmount();

    render(<PropsHarness sheetProps={{ footer: <p>footer actions</p> }} />);
    const { body } = headerAndBody();
    const footer = screen.getByText("footer actions").parentElement as HTMLElement;
    // Chrome language: sidebar surface over the body's surface, separation
    // by tint alone (the sheets carry no lines anywhere); safe-area bottom
    // padding (the footer is the last surface on screen), inset-clamped
    // sides like the rest of the panel.
    expect(footer.className).toContain("bg-carbon-sidebar");
    expect(footer.className).not.toMatch(/border-\S+/);
    expect(footer.className).toContain("pb-[var(--safe-area-bottom)]");
    expect(footer.className).toContain("pl-[max(1rem,var(--safe-area-left))]");
    // And it comes after the scroll body; the "never scrolls away" ordering.
    expect(footer.previousElementSibling).toBe(body);
    // With a footer present the body drops its own inset: the footer is the
    // last surface and pads itself, so a
    // body inset beside one paid the home-indicator gap twice, 34px of dead
    // surface between the message and the action row on an iPhone.
    expect(body.className).not.toContain("pb-[var(--safe-area-bottom)]");
  });
});
