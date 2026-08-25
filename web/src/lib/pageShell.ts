// ---------------------------------------------------------------------------
// PAGE_SHELL — the ONE root-wrapper class every routed page uses.
//
// jdp, live review: "Im Tab Selbst-Backup und Flash sind die Cards schmaler.
// Können wir die nicht überall gleich breit machen?"
//
// He was right, and it was worse than the two pages he happened to open. A
// sweep of every page's REAL root element (not a grep of the first match in
// each file — that picks up inner elements) found FIVE different rendered card
// widths and TWO different card gaps, measured live at a 1920x1080 viewport
// (<main> content box 1633px, so every cap below was actually in effect):
//
//     page         max-width class   rendered   gap
//     Recovery     (none)            1633px     40px
//     Settings     (none, see below) 1424px     40px
//     Dashboard    max-w-6xl         1152px     40px
//     Containers   max-w-5xl         1024px     24px
//     VMs          max-w-5xl         1024px     24px
//     Files        max-w-5xl         1024px     24px
//     Fleet        max-w-5xl         1024px     40px
//     Receiver     max-w-5xl         1024px     40px
//     Flash        max-w-3xl          768px     24px
//     Config       max-w-3xl          768px     40px
//
// Why 40px (gap-10) and not 24px
// ------------------------------
// 40px was already the majority (6 of 10 pages) AND it is the value earlier
// rounds deliberately converted Config, Receiver, Fleet and Recovery to, each
// recording its own measurement history in a wrapper comment ("systemweit
// gleich machen"). Those rounds only ever touched the page jdp had named that
// day, which is why Containers, VMs, Files and Flash were still on the old
// 24px. This constant is that conversion finished.
//
// Why 1152px (max-w-6xl) and not 1024px
// -------------------------------------
// Raw page count favours 1024px (5 pages) — but page count is the wrong
// tie-breaker here, because those five are plain flex-col Card stacks whose
// content does not depend on being exactly 1024px wide; they simply fill
// whatever wrapper they are given. Dashboard is the one page with genuinely
// dense multi-column content (a `md:grid-cols-2` block grid with user-
// selectable half-width blocks, 7-column container-query run rows, and an
// Advanced-mode 7-across stat tier), and it measures at the EDGE at 1024px:
//
//   * 7-across advanced stat tier, measured live in de (longest-label locale)
//     by cloning the tier to 7 cells at both widths:
//         at 1024px → 136px cells; longest label "Speicherbelegung" needs
//                     exactly 136px. Zero slack — one character from clipping.
//         at 1152px → 154px cells; same label needs 154px of a 154px cell at
//                     its natural size, i.e. 18px of real headroom over 1024.
//   * the `@[44rem]` (704px) container-query run rows still engage at both
//     widths (container 984px at 1024, 1112px at 1152), so that threshold is
//     NOT what decides this.
//   * shrinking Dashboard to 1024px additionally starts truncating the
//     "jeden 3. Tag um 5:15 Uhr" schedule label (needs 131px, gets 124px).
//
// So unifying DOWN would put the app's densest row one glyph from clipping in
// its longest-label locale, to buy nothing on the five pages that would keep
// their current width. Unifying UP costs nothing measurable anywhere: no page
// has content that breaks, reflows or overflows at 1152px (verified live).
//
// The two surfaces currently WIDER than 1152px are not deliberate width
// decisions at all and are not evidence for a wider value:
//   * Recovery had NO max-width — its 1633px was simply whatever the window
//     handed it, a missing constraint rather than a choice. It takes the
//     shared cap like everyone else.
//   * Settings' 1424px is a DERIVED measurement of its own tab strip, not a
//     chosen page width. It is the one real exception — see PAGE_SHELL_TABBED.
//
// Not in scope: Login
// -------------------
// pages/Login.tsx is deliberately NOT on this shell and should not be "fixed"
// onto it by a later sweep. It is not a routed content page at all — Layout.tsx
// returns it BEFORE rendering the sidebar/`main` shell when auth is blocked, so
// it never sits under `<Outlet />`. Its `w-full max-w-sm` is a centred
// full-screen auth card, a different layout primitive with its own reason to be
// narrow, not a page column that drifted.
// ---------------------------------------------------------------------------

/**
 * The root wrapper for a routed page: one column, the app-wide 40px Card
 * rhythm, and the app-wide 1152px content cap.
 *
 * Every page under `<Outlet />` uses this verbatim. If a page ever needs to
 * differ, that has to be a stated, reasoned exception in a comment at the call
 * site (Settings is the only one today) — never a quietly different literal,
 * which is exactly how the five-widths/two-gaps spread above accumulated.
 */
export const PAGE_SHELL = "flex flex-col gap-10 max-w-6xl";

/**
 * Settings only — the ONE stated exception to PAGE_SHELL's width.
 *
 * Same 40px rhythm, but no `max-w-6xl`, plus the `flex-1` its sticky
 * AboutFooter needs to fill `main`'s column (see the footer's own comment).
 *
 * Why the width cap has to be absent here, measured live rather than assumed:
 * Settings' 7-tab Selector strip is `size="lg"` + `equalWidth`, so its width is
 * 7x its widest segment — 1424px in de. Its Card panels are then capped to that
 * measured strip width (`tabStripWidth`, a ResizeObserver reading), which is a
 * standing jdp instruction ("Settings cards should match the tab row's width").
 * Putting `max-w-6xl` on this root caps the STRIP too, and at 1152px the strip
 * no longer fits on one line — verified live: strip height 32px -> 68px, the 7
 * segments falling onto 2 rows. That two-row strip is a bug an earlier round
 * already fixed once (a lone "System" tab stranded on row 2); re-introducing it
 * to win 272px of width uniformity is the wrong trade.
 *
 * Note this is a genuine conflict between two of jdp's own asks — "cards match
 * the tab row" (Settings-specific, older) and "überall gleich breit" (app-wide,
 * this round) — which cannot both hold while the strip needs 1424px. Resolving
 * it properly means making the STRIP narrower (dropping `equalWidth`, whose
 * natural hugged width is ~814px in de, or stepping `size` down from "lg"),
 * which is a change to a deliberate prior decision and is jdp's call, not a
 * silent squeeze here.
 */
export const PAGE_SHELL_TABBED = "flex flex-col gap-10 flex-1";
