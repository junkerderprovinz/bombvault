import { useEffect, useRef, useState, type ReactNode } from "react";

// FilterPopover puts a page's filter controls behind one "Filters" button. The
// controls keep their own state and persistence; this only changes where they
// render. It closes on an outside mousedown or Escape.
//
// `active` puts a dot on the trigger when a filter is set to something other
// than its default. Filters persist to localStorage, so a restored filter
// could otherwise shrink the list with nothing on screen to say why.

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
        /* Only the surface and its hover are local, so a popover trigger does
           not read as one of the page's actions. */
        className="glim-btn bg-carbon-surface2 font-medium text-carbon-text hover:bg-carbon-surface3 transition-colors"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true">
          <path d="M1 2.5h10L7 7v3.5L5 11.5V7z" />
        </svg>
        {label}
        {active && <span className="h-1.5 w-1.5 rounded-full bg-accent" aria-hidden="true" />}
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
