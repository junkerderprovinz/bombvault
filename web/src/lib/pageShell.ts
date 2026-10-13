// Root wrapper classes for routed pages, so every page shares one card gap and
// one content width.
//
// A page is as wide as the window leaves it beside the sidebar, as in
// KnightLoader, so the page actions stand in the window's bottom corner and a
// settings card uses the room a wide window gives it.
//
// Login is not on this shell. Layout.tsx returns it before rendering the
// sidebar and the main column, so it never sits under <Outlet />, and its
// narrow max-w-sm card is a different layout rather than a page column.

/**
 * The root wrapper for a routed page: one column across the main area and the
 * app-wide 40px card gap. A page that needs something else says why in a
 * comment at the call site instead of using a different literal.
 */
export const PAGE_SHELL = "flex flex-col gap-10";

/**
 * The responsive rhythm for the pages restructured for phone-width columns.
 *
 * Identical to PAGE_SHELL at 48rem and above (md:gap-10 is gap-10). Below
 * that the Card rhythm steps down to 24px, where 40px gaps read as wasted
 * scroll on a phone-width column.
 *
 * A stated, per-file exception (eslint.config.js) per page that adopts this
 * constant, and not a replacement of PAGE_SHELL itself:
 * retuning every routed page's mobile rhythm is nobody's decision but the
 * pages' own, and this constant exists because exactly the screens
 * restructured for phones carry it.
 */
export const PAGE_SHELL_RESPONSIVE = "flex flex-col gap-6 md:gap-10";

/**
 * The root wrapper for a page embedded as a tab panel of another page:
 * PAGE_SHELL_RESPONSIVE's gaps. The panel drops to 24px gaps below 48rem along
 * with its host.
 */
export const PAGE_SHELL_TABBED_RESPONSIVE = "flex flex-col gap-6 md:gap-10";
