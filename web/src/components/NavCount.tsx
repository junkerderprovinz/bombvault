import type { NavCount as Count } from "../lib/navCounts";

const SIZES = {
  row: "h-5 min-w-5 px-1.5 text-dense",
  corner: "h-4 min-w-4 px-1 text-caption",
};

/**
 * NavCount is the figure on a navigation entry. Its colours are the entry's
 * own and come from index.css (`.glim-nav-count`), which also turns them round
 * on the current entry. `quiet` is the grey form the phone's bar uses.
 */
export function NavCount({
  count,
  quiet = false,
  size,
  className,
}: {
  count: Count;
  quiet?: boolean;
  size: keyof typeof SIZES;
  className: string;
}) {
  return (
    <span
      aria-label={count.label}
      className={`glim-nav-count${quiet ? " glim-nav-count--quiet" : ""} inline-flex shrink-0 box-border items-center justify-center rounded-pill font-semibold leading-none ${SIZES[size]} ${className}`}
    >
      {count.value}
    </span>
  );
}
