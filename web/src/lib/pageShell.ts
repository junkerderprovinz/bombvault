// Root wrapper classes for routed pages, so every page shares one card gap and
// one content width.
//
// The width is 1152px (max-w-6xl) rather than 1024px because of Dashboard, the
// densest page. Measured in German, the longest-label locale: at 1024px its
// 7-across advanced stat tier gets 136px cells and "Speicherbelegung" needs
// exactly 136px, and the "jeden 3. Tag um 5:15 Uhr" schedule label truncates
// (131px wanted, 124px given). At 1152px the cells are 154px and no page
// breaks, reflows or overflows.
//
// Login is not on this shell. Layout.tsx returns it before rendering the
// sidebar and the main column, so it never sits under <Outlet />, and its
// narrow max-w-sm card is a different layout rather than a page column.

/**
 * The root wrapper for a routed page: one column, the app-wide 40px card gap
 * and the 1152px content cap. A page that needs something else says why in a
 * comment at the call site instead of using a different literal.
 */
export const PAGE_SHELL = "flex flex-col gap-10 max-w-6xl";

/**
 * The responsive rhythm for the pages restructured for phone-width columns.
 *
 * Identical to PAGE_SHELL at >=48rem (md:gap-10 is gap-10, same 1152px cap),
 * so the desktop layout is unchanged by construction; below the breakpoint the
 * Card rhythm steps down to 24px, where 40px gaps read as wasted scroll on a
 * phone-width column.
 *
 * A stated, per-file exception (eslint.config.js) per page that adopts this
 * constant, and not a replacement of PAGE_SHELL itself:
 * retuning every routed page's mobile rhythm is nobody's decision but the
 * pages' own, and this constant exists because exactly the screens
 * restructured for phones carry it.
 */
export const PAGE_SHELL_RESPONSIVE = "flex flex-col gap-6 md:gap-10 max-w-6xl";

/**
 * The root wrapper for a page embedded as a tab panel of another page:
 * PAGE_SHELL_RESPONSIVE's gaps without its width cap, which the host page
 * already applies. The panel drops to 24px gaps below 48rem along with its
 * host.
 */
export const PAGE_SHELL_TABBED_RESPONSIVE = "flex flex-col gap-6 md:gap-10";
