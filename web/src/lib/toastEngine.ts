// Pure state transitions for the toast queue. Timing is computed from
// (list, id, now) instead of live timers so it can be tested as plain function
// calls; lib/toast.tsx owns the real timers, React state and the portal.
//
// Hover or focus pauses the countdown and keeps the remaining time: resuming
// continues from the frozen remainder, not from the full duration.

export type ToastSeverity = "success" | "warn" | "fail";

export const TOAST_DURATION_MS = 4000;

/** MAX_VISIBLE_TOASTS is how many cards fit above the fold on a 720px
 *  viewport. Without a cap, a key held down on a button stacks toasts off
 *  screen, and the cards block clicks on the page behind them. */
export const MAX_VISIBLE_TOASTS = 4;

/** The one control a toast may carry beside its message, such as Undo. */
export interface ToastAction {
  label: string;
  onClick: () => void;
}

/** How long a toast with an action stays. After the dialog that led to it
 *  closes, focus is back on the page and the toast is a Shift+Tab away, which
 *  takes longer than reading a line. */
export const ACTION_TOAST_DURATION_MS = 20000;

export function toastDuration(toast: { action?: ToastAction }): number {
  return toast.action ? ACTION_TOAST_DURATION_MS : TOAST_DURATION_MS;
}

export interface ToastEntry {
  id: string;
  message: string;
  severity: ToastSeverity;
  action?: ToastAction;
  /** Time left, frozen when the toast was paused. Only meaningful while
   *  expiresAt is null; while running, expiresAt is the source of truth. */
  remainingMs: number;
  /** Epoch ms this toast auto-dismisses at, or null while paused. */
  expiresAt: number | null;
}

/** addToast appends a toast and, past maxVisible (at least 1), drops the
 *  oldest ones so the feedback for the latest action always shows.
 *
 *  Paused toasts are skipped while running ones are left to drop: the user is
 *  reading or focused on them, and a paused toast never expires, so a plain
 *  drop-oldest would always pick it first and pull focus back to <body>. */
export function addToast(
  list: ToastEntry[],
  toast: { id: string; message: string; severity: ToastSeverity; action?: ToastAction },
  now: number,
  durationMs: number = toastDuration(toast),
  maxVisible: number = MAX_VISIBLE_TOASTS
): ToastEntry[] {
  const newest: ToastEntry = {
    id: toast.id,
    message: toast.message,
    severity: toast.severity,
    action: toast.action,
    remainingMs: durationMs,
    expiresAt: now + durationMs,
  };
  let toDrop = list.length + 1 - maxVisible;
  if (toDrop <= 0) return [...list, newest];
  const kept: ToastEntry[] = [];
  for (const t of list) {
    if (toDrop > 0 && t.expiresAt != null) {
      toDrop--;
      continue;
    }
    kept.push(t);
  }
  // Every remaining older toast is paused; drop the oldest of those.
  if (toDrop > 0) kept.splice(0, toDrop);
  return [...kept, newest];
}

export function removeToast(list: ToastEntry[], id: string): ToastEntry[] {
  return list.filter((t) => t.id !== id);
}

/** pauseToast freezes the countdown at what is left. A toast that is already
 *  paused is left alone, so moving from the card onto its dismiss button does
 *  not re-freeze against a null expiresAt. */
export function pauseToast(list: ToastEntry[], id: string, now: number): ToastEntry[] {
  return list.map((t) => {
    if (t.id !== id || t.expiresAt == null) return t;
    return { ...t, remainingMs: Math.max(0, t.expiresAt - now), expiresAt: null };
  });
}

/** resumeToast restarts the clock from the frozen remainder. */
export function resumeToast(list: ToastEntry[], id: string, now: number): ToastEntry[] {
  return list.map((t) => {
    if (t.id !== id || t.expiresAt != null) return t;
    return { ...t, expiresAt: now + t.remainingMs };
  });
}

/** Hover and focus are tracked separately because they end independently:
 *  the mouse can leave while the keyboard is still inside. */
export interface ToastEngagement {
  hover: boolean;
  focus: boolean;
}

export type ToastEngagementKind = keyof ToastEngagement;

export const NO_ENGAGEMENT: Readonly<ToastEngagement> = { hover: false, focus: false };

/** applyEngagement folds one hover or focus edge into the state and reports
 *  whether the toast is still engaged. The countdown may resume only when both
 *  have ended, otherwise a toast could be dismissed while it still has
 *  keyboard focus. When engaged is false the caller can forget the toast. */
export function applyEngagement(
  prev: Readonly<ToastEngagement>,
  kind: ToastEngagementKind,
  active: boolean
): { next: ToastEngagement; engaged: boolean } {
  const next = { ...prev, [kind]: active };
  return { next, engaged: next.hover || next.focus };
}

/** Quiet mode hides routine successes; warnings and failures always show. */
export function shouldShowToast(severity: ToastSeverity, quiet: boolean): boolean {
  return !quiet || severity !== "success";
}
