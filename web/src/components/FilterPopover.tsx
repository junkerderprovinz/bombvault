import { useEffect, useRef, useState, type ReactNode } from "react";

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
// the list behind the collapsed button with no hint. The accent dot + accent
// border on the trigger surface that a filter is applied; each page computes
// `active` from its own current filter state.

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

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen((p) => !p)}
        aria-expanded={open}
        aria-haspopup="dialog"
        className={`inline-flex items-center gap-1.5 rounded-lg border bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors ${
          active ? "border-accent" : "border-carbon-border"
        }`}
      >
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true">
          <path d="M1 2.5h10L7 7v3.5L5 11.5V7z" stroke="currentColor" strokeWidth="1" strokeLinejoin="round" />
        </svg>
        {label}
        {/* Active-filter indicator: a persisted non-default filter silently
            shrinks the list behind the collapsed button, so hint that one is on. */}
        {active && <span className="h-1.5 w-1.5 rounded-full bg-accent" aria-hidden="true" />}
      </button>
      {open && (
        <div
          role="dialog"
          aria-label={label}
          className="absolute left-0 top-full z-20 mt-2 w-max min-w-[16rem] max-w-[min(90vw,26rem)] rounded-card border border-carbon-border bg-carbon-surface p-4 shadow-lg flex flex-col gap-4"
        >
          {children}
        </div>
      )}
    </div>
  );
}
