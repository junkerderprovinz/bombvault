// ---------------------------------------------------------------------------
// StickyActionBar; the one shared sticky-in-flow action bar (consumed by the
// tree Save bar, the Dashboard's trigger zone, and the step flows that follow).
//
// Why sticky-in-flow and never position:fixed (the locked shell discipline):
// fixed chrome fights the Layout visualViewport keyboard mechanism (a fixed
// bar does not move when the keyboard resizes the visual viewport) and needs
// manual width/safe-area syncing that in-flow sticky gets for free.
//
// Why the last direct child of the page column (the research pitfall): a
// sticky element is confined to its parent box; nested inside a Card, the bar
// "sticks" only within that card's own height and scrolls away with it. As a
// direct child of the PAGE_SHELL column (whose nearest scrolling ancestor is
// main#bv-main, Layout.tsx) the bar rides the page scroll and pins to the
// viewport bottom while any of the column is on screen. For the same reason
// no overflow/contain wrapper may sit between this bar and main#bv-main;
// keep new page wrappers clean (.glim-page-enter, the per-route wrapper
// Layout renders, is verified clean).
//
// Chrome mirrors the BottomNav precedent (BottomNav.tsx): sidebar surface,
// 12px padding above and below. The bar carries no safe-area inset, and
// never will: it is sticky inside main#bv-main, and BottomNav, main's flex
// sibling below it, owns the home-indicator inset on its own host, so the
// bar never reaches the screen edge. Reserving the inset here only made the
// bar 22px taller than every other phone chrome row on devices that have
// one. No separator line, because surfaces in this app are told apart by
// shade. No negative margins and no fixed positioning either; the bar spans
// the page column's own width inside the scroller gutter (the phone's 16px
// `main` padding).
//
// The scroller keeps --sticky-action-h of scroll padding for this bar
// (Layout.tsx), so a control the keyboard scrolls to does not come to rest
// underneath it.
// ---------------------------------------------------------------------------

import type { ReactNode } from "react";

export function StickyActionBar({ children, className = "" }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={`sticky bottom-0 z-10 bg-carbon-sidebar pt-3 pb-3 ${className}`}
    >
      {/* Rows stack: count/busy row, then the primary action, then the
          plain-language second row (the save-bar anatomy). */}
      <div className="flex flex-col gap-2">{children}</div>
    </div>
  );
}
