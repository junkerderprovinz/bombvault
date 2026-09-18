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
//       panel are siblings — see BottomSheet.tsx's header deviation note)
//   (d) the close button is found by its accessible name (common.close) and
//       closes — the ConfirmDialog strict-mode discipline
//   (e) Tab/Shift+Tab wrap over the FULL FOCUSABLE_SELECTOR candidate set
//       (header close button + body controls) in both directions
//   (f) focus is captured from the trigger at open (and moved inside, the
//       ConfirmDialog autoFocus parity) and restored to the trigger on close
//
// The second describe asserts the additive capabilities the same way, with
// one documented exception to the no-class-snapshot rule: the
// fullHeight / footer / tone / inset-clamped-padding / 44px-close contracts
// are STYLING contracts, and jsdom computes no geometry, so the observable
// form of "the panel is h-dvh" IS the class token. These tests assert the
// PRESENCE of the load-bearing tokens (has-class, never a whole className
// string) — the same targeted-token discipline, not a snapshot.
//
// Deliberately self-sufficient: BottomSheet touches no matchMedia /
// visualViewport API, so this suite passes whether or not the vitest
// setupFiles matchMedia stub is installed — nothing here relies on it.

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
    // A click on a control inside the panel bubbles up through the panel —
    // and never reaches the scrim's handler, because the panel is NOT a child
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
    // ConfirmDialog parity: focus starts INSIDE the aria-modal surface (its
    // Cancel button; the sheet's safe equivalent is the header close button).
    expect(document.activeElement).toBe(screen.getByRole("button", { name: en["common.close"] }));
  });

  it("wraps Tab and Shift+Tab focus inside the sheet in both directions", () => {
    render(<SheetHarness onClose={vi.fn()} initialOpen />);
    const close = screen.getByRole("button", { name: en["common.close"] });
    const two = screen.getByRole("button", { name: "body two" });
    // Tab from the LAST focusable wraps to the FIRST (the header close button).
    two.focus();
    fireEvent.keyDown(document, { key: "Tab" });
    expect(document.activeElement).toBe(close);
    // Shift+Tab from the FIRST focusable wraps to the LAST.
    close.focus();
    fireEvent.keyDown(document, { key: "Tab", shiftKey: true });
    expect(document.activeElement).toBe(two);
    // Note: forward movement from a MID-list control is native browser Tab
    // navigation (the trap handler only acts at the edges, per the verbatim
    // useConfirm lift), which jsdom does not implement — so only the two
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
});

// ---------------------------------------------------------------------------
// Extension contracts. PropsHarness forwards the additive props onto an
// already-open sheet — the capabilities are consumed with the sheet open,
// exactly as RunDetailSheet (fullHeight + footer) and ConfirmSheet (tone)
// mount them.
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
 *  sibling — the panel's structural order, readable without testids. */
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
    // Each physical side clamps against its OWN inset (max(1rem, inset)):
    // content clears a landscape display cutout on either rotation.
    for (const el of [header, body]) {
      expect(el.className).toContain("pl-[max(1rem,var(--safe-area-left))]");
      expect(el.className).toContain("pr-[max(1rem,var(--safe-area-right))]");
    }
    // The body keeps its bottom safe-area padding (unchanged contract).
    expect(body.className).toContain("pb-[var(--safe-area-bottom)]");
  });

  it("gives the close button the 44px touch hit box", () => {
    render(<PropsHarness sheetProps={{}} />);
    const close = screen.getByRole("button", { name: en["common.close"] });
    // h-11 / w-11 = 44px — the touch floor. Asserted as tokens (jsdom has no
    // geometry); the e2e harness asserts the rendered pixels on device views.
    expect(close.className).toContain("h-11");
    expect(close.className).toContain("w-11");
  });

  it("keeps the carbon-surface panel by default and tints it only for a tone", () => {
    const { unmount } = render(<PropsHarness sheetProps={{}} />);
    expect(screen.getByRole("dialog").className).toContain("bg-carbon-surface");
    expect(screen.getByRole("dialog").className).not.toContain("statusFail");
    unmount();

    // The fail-tone confirm sheet surface.
    const fail = render(<PropsHarness sheetProps={{ tone: "fail" }} />);
    let panel = screen.getByRole("dialog");
    expect(panel.className).toContain("bg-statusFailBg");
    expect(panel.className).toContain("border-statusFailBorder");
    fail.unmount();

    // The warn tone mirrors ConfirmDialog's non-destructive branch.
    const warn = render(<PropsHarness sheetProps={{ tone: "warn" }} />);
    panel = screen.getByRole("dialog");
    expect(panel.className).toContain("bg-statusWarnBg");
    expect(panel.className).toContain("border-statusWarnBorder");
    warn.unmount();
  });

  it("renders an optional footer after the scroll body, chrome-styled and safe-area padded", () => {
    // Default: no footer element at all — the panel is header + body only.
    const { unmount } = render(<PropsHarness sheetProps={{}} />);
    const { panel } = headerAndBody();
    expect(panel.childElementCount).toBe(2);
    unmount();

    render(<PropsHarness sheetProps={{ footer: <p>footer actions</p> }} />);
    const { body } = headerAndBody();
    const footer = screen.getByText("footer actions").parentElement as HTMLElement;
    // Chrome language: sidebar surface + top hairline (bottom bar / sticky
    // bar tokens), safe-area bottom padding (the footer is the LAST surface
    // on screen), inset-clamped sides like the rest of the panel.
    expect(footer.className).toContain("bg-carbon-sidebar");
    expect(footer.className).toContain("border-t");
    expect(footer.className).toContain("pb-[var(--safe-area-bottom)]");
    expect(footer.className).toContain("pl-[max(1rem,var(--safe-area-left))]");
    // And it comes AFTER the scroll body — the "never scrolls away" ordering.
    expect(footer.previousElementSibling).toBe(body);
  });
});
