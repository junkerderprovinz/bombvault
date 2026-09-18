import { useEffect, useRef, useState, type ReactNode } from "react";
import { useIsDesktop } from "../lib/useMediaQuery";
import { TapPopover } from "./mobile/TapPopover";

// ---------------------------------------------------------------------------
// FilterPopover — shared, accessible filter disclosure (#2.6)
// ---------------------------------------------------------------------------
// Collapses a page's filter controls (search + installed/schedule/backup chips,
// and on the VMs page the sort chips) behind a single "Filters" button so the
// toolbar stays uncluttered. The controls themselves are unchanged and keep
// their own state + localStorage persistence — this only relocates where they
// render. Extracted from two drifted per-page copies so both pages share one
// accessible implementation.
//
// Accessible: the trigger reports aria-haspopup="dialog" + aria-expanded, the
// panel is a role="dialog" labelled by the trigger text, and the decorative
// funnel icon is aria-hidden. Closes on click-outside (mousedown) or Escape.
//
// `active` marks that at least one filter inside is set to a non-default value.
// The schedule/backup (and Containers' installed) filters persist to
// localStorage, so a restored non-"all" filter would otherwise silently shrink
// the list behind the collapsed button with no hint. The accent dot on the
// trigger surfaces that a filter is applied; each page computes `active` from
// its own current filter state.
//
// Two presentations, ONE state model:
//   - Desktop (>= 48rem, useIsDesktop — the ONE width authority): the original
//     in-flow presentation, byte-identical — the absolutely-positioned panel
//     hangs off the wrapper and the component's own outside-mousedown +
//     Escape listeners own dismissal.
//   - Below the breakpoint: the panel mounts through TapPopover instead. The
//     in-flow absolute panel is positioned relative to this wrapper with no
//     viewport awareness, so on a phone a trigger near the screen's start or
//     end edge shoves its own panel off-screen and clips it — the same class
//     of bug computeBubblePosition was written to fix for bubbles. TapPopover
//     supplies the viewport-clamped anchoring (computeBubblePosition), the
//     consuming backdrop (outside taps die on it, never click through),
//     Escape, re-tap, and the light focus contract; this component keeps
//     owning the CONTENT and every bit of filter state, which passes through
//     as children untouched.
//
// The mobile branch deliberately does NOT touch this component's own `open`
// state: it stays false on mobile forever, so the desktop dismissal effect
// below no-ops and the two dismissal systems can never fight over one
// boolean. The trigger content (funnel glyph, label, active dot) is shared
// verbatim between the branches; the trigger's >=44px touch floor comes from
// TapPopover's baked-in min-h-11/min-w-11 (the desktop glim-btn keeps its
// token height).

export function FilterPopover({
  label,
  children,
  active = false,
}: {
  label: string;
  children: ReactNode;
  active?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const isDesktop = useIsDesktop();

  useEffect(() => {
    if (!open) return;
    function onPointerDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  const trigger = (
    <>
      {/* FILLED funnel glyph (a filled silhouette reads as the active
          filter affordance at this size — the closed path was already
          fill-capable, so it flips directly: same path data,
          `fill="currentColor"`, no redraw). */}
      <svg width="12" height="12" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true">
        <path d="M1 2.5h10L7 7v3.5L5 11.5V7z" />
      </svg>
      {label}
      {/* Active-filter indicator: a persisted non-default filter silently
          shrinks the list behind the collapsed button, so hint that one is on. */}
      {active && <span className="h-1.5 w-1.5 rounded-full bg-accent" aria-hidden="true" />}
    </>
  );

  if (!isDesktop) {
    return (
      <TapPopover
        label={label}
        trigger={trigger}
        /* `glim-btn` rather than its own padding ([327], jdp: "Der Filter
           button größer machen und ins größensystem einbinden") — see the
           desktop branch below; the surface colour and its hover stay local
           because this trigger opens a popover and should not read as one of
           the page's actions. The 44px touch floor is TapPopover's. */
        /* hover:bg-carbon-hover sits BELOW the surface2 this trigger is filled
           with (the hover-tier order hoverRamp.test.ts pins): surface2's own
           hover tier is surface3. */
        triggerClassName="glim-btn bg-carbon-surface2 font-medium text-carbon-text hover:bg-carbon-surface3 transition-colors"
        /* Same surface recipe as the desktop panel below (p-4, min-w, max-w,
           column layout) — only the positioning system changed. */
        panelClassName="flex flex-col gap-4 p-4 min-w-[16rem] max-w-[min(90vw,26rem)]"
      >
        {children}
      </TapPopover>
    );
  }

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen((p) => !p)}
        aria-expanded={open}
        aria-haspopup="dialog"
        /* `glim-btn` rather than its own padding ([327], jdp: "Der Filter button
           größer machen und ins größensystem einbinden"). It was `py-1.5
           text-xs`: smaller than every real button, and sized by two literals
           no token could ever reach — so when `--btn-h` moved from 2.25rem to
           2rem in [233] this control did not move with it, and it has been
           drifting quietly ever since. The class supplies height, horizontal
           padding, gap, radius and font size from the tokens; only the surface
           colour and its hover stay local, because this trigger opens a
           popover and should not read as one of the page's actions. */
        className="glim-btn bg-carbon-surface2 font-medium text-carbon-text hover:bg-carbon-surface3 transition-colors"
      >
        {trigger}
      </button>
      {open && (
        <div
          role="dialog"
          aria-label={label}
          className="absolute start-0 top-full z-20 mt-2 w-max min-w-[16rem] max-w-[min(90vw,26rem)] rounded-card bg-carbon-surface p-4 shadow-xl flex flex-col gap-4"
        >
          {children}
        </div>
      )}
    </div>
  );
}
