import { useLayoutEffect, useRef, type ReactNode } from "react";
import { useIsDesktop } from "../lib/useMediaQuery";
import { Button } from "./Button";

/** What the corner reports to index.css, so toasts rise above it. */
const CLEARANCE = "--page-actions-clearance";

/**
 * PageActions holds what a page is there for, in the same corner on every
 * page: the bottom end of the content column, floating over the rows that
 * scroll under it.
 *
 * It is the last child of the page's root element and sticks to the bottom of
 * the scroller, so it is never fixed and needs no width or offset of its own.
 * At the end of the page it takes its place in the flow below the last row and
 * covers nothing. index.css lets the page's root fill the scroller, which
 * keeps the corner at the bottom of a page shorter than the window.
 */
export function PageActions({ children }: { children: ReactNode }) {
  const bar = useRef<HTMLDivElement>(null);

  // How far the corner reaches up from the bottom of the window depends on
  // what stands below the scroller and on how many rows the actions wrap
  // into, so it is measured.
  useLayoutEffect(() => {
    const el = bar.current;
    if (!el) return;
    const root = document.documentElement;
    const report = () => {
      const scroller = el.closest("main");
      const below = scroller ? window.innerHeight - scroller.getBoundingClientRect().bottom : 0;
      root.style.setProperty(CLEARANCE, `${Math.max(0, below) + el.offsetHeight}px`);
    };
    const observer = new ResizeObserver(report);
    observer.observe(el);
    window.addEventListener("resize", report);
    report();
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", report);
      root.style.removeProperty(CLEARANCE);
    };
  }, []);

  return (
    <div
      ref={bar}
      data-testid="page-actions"
      className="glim-page-actions pointer-events-none sticky bottom-0 z-10 mt-auto flex justify-end pb-4 max-md:pb-3"
    >
      <div className="pointer-events-auto flex flex-wrap items-center justify-end gap-2.5">{children}</div>
    </div>
  );
}

/**
 * PageAction is one button in that corner. The one a page exists for is
 * `primary` and the only filled accent in the view. Below the desktop width
 * every action shows its glyph alone.
 */
export function PageAction({
  label,
  glyph,
  onClick,
  primary = false,
  disabled = false,
  busy = false,
  title,
}: {
  label: string;
  /** Required, because a phone shows nothing else. */
  glyph: ReactNode;
  onClick: () => void;
  primary?: boolean;
  disabled?: boolean;
  busy?: boolean;
  /** What changes about the action: that it runs, or why it cannot. */
  title?: string;
}) {
  const isDesktop = useIsDesktop();
  return (
    <Button
      label={label}
      labelKey={null}
      glyph={glyph}
      tone={primary ? "accent" : "subtle"}
      variant={isDesktop ? "default" : "icon"}
      glyphOnly={!isDesktop}
      onClick={onClick}
      disabled={disabled}
      busy={busy}
      title={title}
      className="glim-btn-key"
    />
  );
}
