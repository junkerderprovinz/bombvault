// ---------------------------------------------------------------------------
// toastEngine — pure, framework-free state transitions for the toast queue
// (GlimStone form-engine Task 9, design-language.md "Toasts").
//
// Every timing rule the spec calls out is deterministic MATH over
// (list, id, now) rather than a live setTimeout, so it's directly
// unit-testable the same way this repo tests everything else — as a plain
// function call inspected with plain assertions, no jsdom/fake-timers/real
// waiting needed (see Toggle.test.ts's header comment: this repo's test
// suite is `environment: "node"` with zero DOM-rendering infrastructure).
// The STATEFUL half — real setTimeout handles, Date.now(), React state, the
// createPortal(..., document.body) render — lives in the companion hook,
// lib/toast.tsx, which is a thin real-timer wrapper around these pure
// transitions. Mirrors useConfirm.tsx/useReveal.ts's split: real timers and
// `document` access are exactly the two things a node-environment test can't
// exercise directly, so they're pushed to the one file this suite
// deliberately doesn't unit-test (covered by live Playwright instead).
//
// The one rule this file exists to get exactly right (design-language.md's
// "Toasts" section): "Hover or focus pauses the countdown — and preserves
// the remaining time, rather than restarting it." pauseToast freezes
// `remainingMs` at whatever's actually left; resumeToast restarts the clock
// from THAT value, never from the full duration again.
// ---------------------------------------------------------------------------

export type ToastSeverity = "success" | "warn" | "fail";

/** Fixed duration (design-language.md: "4 seconds is long enough to read,
 *  short enough not to pile up — a toast is a notice, not a to-do list."). */
export const TOAST_DURATION_MS = 4000;

/** Hard cap on simultaneously-visible toasts. Nothing in the spec pins an
 *  exact number, but "not a to-do list" implies a small corner stack, not
 *  an unbounded one — live-measured against this app's actual bottom-end
 *  viewport corner (see toastEngine.test.ts / Toast.test.ts for the math):
 *  4 cards is the largest count that still comfortably fits above the fold
 *  on a short (~720px) viewport without needing scroll, while still showing
 *  more than a single toast at a time when several actions land in quick
 *  succession. Without a cap, rapid repeated triggers (holding Enter on a
 *  focused button fires at keyboard auto-repeat rate) grow the stack
 *  unbounded — live-reproduced pre-fix: 30+ toasts produced a 1580px-tall
 *  stack in a 720px viewport, 18 cards permanently off-screen with no
 *  scroll affordance, and ~86% of the right-hand column unclickable
 *  (pointer-events:auto on every card intercepting clicks meant for the
 *  page behind it) — directly contradicting the toast system's own
 *  non-blocking design (see components/Toast.tsx's header comment). */
export const MAX_VISIBLE_TOASTS = 4;

export interface ToastEntry {
  id: string;
  message: string;
  severity: ToastSeverity;
  /** Authoritative ONLY while paused (expiresAt === null): what's left of
   *  the 4s, frozen at the moment hover/focus began. While running this
   *  value is stale (kept only so a freshly-pushed toast still reports a
   *  sane number) — expiresAt is the source of truth whenever it's set. */
  remainingMs: number;
  /** Epoch ms this toast auto-dismisses at, or null while paused. */
  expiresAt: number | null;
}

/** Stacks a new toast onto the END of the list — never replaces an existing
 *  one (design-language.md: "Stacked, not replaced. Multiple toasts queue in
 *  a fixed corner rather than one overwriting the next"). Bounded at
 *  `maxVisible` (which must be >= 1): once the stack would exceed the cap,
 *  the OLDEST toast(s) are dropped from the FRONT — never the newest. A user
 *  who just triggered an action expects to see feedback for THAT action, so
 *  the brand-new push always survives; it's the stalest, already-half-read
 *  toast that gives up its spot.
 *
 *  One exception to "oldest first": a PAUSED toast (expiresAt === null) is
 *  one the user is actively hovering or tabbed into, and evicting it does
 *  the exact damage the hover/focus pause exists to prevent — yanking away
 *  something being read, or destroying a keyboard user's focus position
 *  (React unmounts the focused dismiss button and `document.activeElement`
 *  snaps back to <body>). Since a paused toast never expires on its own, it
 *  is also permanently the oldest entry, so a naive drop-oldest would target
 *  it FIRST. So the cap skips paused toasts while it has running ones left
 *  to drop, and only falls back to dropping the plain oldest if every older
 *  toast is paused. */
export function addToast(
  list: ToastEntry[],
  toast: { id: string; message: string; severity: ToastSeverity },
  now: number,
  durationMs: number = TOAST_DURATION_MS,
  maxVisible: number = MAX_VISIBLE_TOASTS
): ToastEntry[] {
  const newest: ToastEntry = {
    id: toast.id,
    message: toast.message,
    severity: toast.severity,
    remainingMs: durationMs,
    expiresAt: now + durationMs,
  };
  let toDrop = list.length + 1 - maxVisible;
  if (toDrop <= 0) return [...list, newest];
  // Pass 1, oldest first: give up the running (un-engaged) toasts.
  const kept: ToastEntry[] = [];
  for (const t of list) {
    if (toDrop > 0 && t.expiresAt != null) {
      toDrop--;
      continue;
    }
    kept.push(t);
  }
  // Pass 2: still over the cap, so every remaining older toast is paused —
  // drop the plain oldest of those rather than the user's newest feedback.
  if (toDrop > 0) kept.splice(0, toDrop);
  return [...kept, newest];
}

/** Removes exactly the targeted toast — every other toast in the stack is
 *  untouched ("each is independently dismissible"). A no-op on an unknown id. */
export function removeToast(list: ToastEntry[], id: string): ToastEntry[] {
  return list.filter((t) => t.id !== id);
}

/** Hover/focus enters the toast: freeze the countdown at whatever's actually
 *  left right now, and drop expiresAt so nothing else can race it forward.
 *  A no-op on an unknown id or a toast that's already paused (so a second
 *  pause call — e.g. moving the mouse from the card onto its own dismiss
 *  button — can never re-freeze an already-frozen value against a stale,
 *  now-null expiresAt). */
export function pauseToast(list: ToastEntry[], id: string, now: number): ToastEntry[] {
  return list.map((t) => {
    if (t.id !== id || t.expiresAt == null) return t;
    return { ...t, remainingMs: Math.max(0, t.expiresAt - now), expiresAt: null };
  });
}

/** Hover/focus leaves the toast: restart the clock from the FROZEN
 *  remainder, not the full 4s again — the exact "preserves the remaining
 *  time, rather than restarting it" requirement. A no-op on an unknown id or
 *  a toast that's already running (not paused). */
export function resumeToast(list: ToastEntry[], id: string, now: number): ToastEntry[] {
  return list.map((t) => {
    if (t.id !== id || t.expiresAt != null) return t;
    return { ...t, expiresAt: now + t.remainingMs };
  });
}

/** The two independent ways a user can be engaged with a toast. Kept as two
 *  named flags rather than one boolean because they end and begin
 *  independently: the mouse can leave while the keyboard is still inside. */
export interface ToastEngagement {
  hover: boolean;
  focus: boolean;
}

export type ToastEngagementKind = keyof ToastEngagement;

/** Shared "neither hovered nor focused" starting point. Readonly because it is
 *  one object handed to every caller — applyEngagement always returns a fresh
 *  object rather than editing what it was given. */
export const NO_ENGAGEMENT: Readonly<ToastEngagement> = { hover: false, focus: false };

/** Folds one hover/focus edge into a toast's engagement state and answers the
 *  only question the caller actually has: may the countdown run again now?
 *
 *  This is the rule the first implementation of the toast system got wrong:
 *  onMouseLeave and onBlur each resumed the countdown on their own, so Tab-ing
 *  into a hovered toast and then moving the mouse away — with focus still very
 *  much inside the toast — resumed it and auto-dismissed it out from under the
 *  keyboard user, snapping `document.activeElement` back to <body>. "Hover OR
 *  focus pauses" only holds together if resuming asks "are BOTH disengaged?"
 *  rather than "did THIS one just end?".
 *
 *  It lives here, in the pure engine, rather than inline in lib/toast.tsx's
 *  DOM handlers, for the same reason every other timing rule does: this file
 *  is unit-testable as plain function calls, and that file (real timers +
 *  `document`) deliberately isn't. A false `engaged` doubles as "nothing left
 *  to remember about this toast", so the caller can drop it from its per-id
 *  bookkeeping instead of accumulating an entry per toast id forever. */
export function applyEngagement(
  prev: Readonly<ToastEngagement>,
  kind: ToastEngagementKind,
  active: boolean
): { next: ToastEngagement; engaged: boolean } {
  const next = { ...prev, [kind]: active };
  return { next, engaged: next.hover || next.focus };
}

/** Severity-based quiet mode (design-language.md: "A quiet mode filters by
 *  severity, not by muting everything. Failures and anything that blocks
 *  progress ... always surface; routine completions can be suppressed").
 *  "success" is the one routine/suppressible category here — warn/fail are
 *  never muted, quiet mode or not. */
export function shouldShowToast(severity: ToastSeverity, quiet: boolean): boolean {
  return !quiet || severity !== "success";
}
