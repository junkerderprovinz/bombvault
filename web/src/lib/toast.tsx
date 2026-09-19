import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { ToastViewport } from "../components/Toast";
import {
  NO_ENGAGEMENT,
  addToast,
  applyEngagement,
  pauseToast,
  removeToast,
  resumeToast,
  shouldShowToast,
  toastDuration,
  type ToastAction,
  type ToastEngagement,
  type ToastEngagementKind,
  type ToastEntry,
  type ToastSeverity,
} from "./toastEngine";
import { useT } from "./i18n";

// ToastProvider owns the live toast queue: the timers, the portal and the
// quiet-mode preference. The pure rules (stacking, pause and resume arithmetic,
// which hover or focus edge pauses or resumes) live in toastEngine.ts and are
// unit tested there; this file keeps only what needs React, real timers and
// the document, and is not unit tested.
//
// The viewport is portalled to <body> because a `position: fixed` element under
// a transformed ancestor (.glim-page-enter, .glim-modal-card) is clipped to
// that ancestor instead of covering the viewport.

const QUIET_STORAGE_KEY = "bombvault.quietToasts";

function readStoredQuiet(): boolean {
  try {
    return localStorage.getItem(QUIET_STORAGE_KEY) === "1";
  } catch {
    return false;
  }
}

interface ToastContextValue {
  /** Queues a toast. Severity defaults to "success", the routine case quiet
   *  mode may suppress. A suppressed push is dropped, not queued for later
   *  (see toastEngine.shouldShowToast). */
  push: (message: string, severity?: ToastSeverity, action?: ToastAction) => void;
  /** Current quiet-toasts preference (Settings › General › Appearance). */
  quiet: boolean;
  setQuiet: (next: boolean) => void;
}

const noop = () => {};

// A no-op default so useToast() works outside the provider, in tests and
// early renders.
const ToastContext = createContext<ToastContextValue>({
  push: noop,
  quiet: false,
  setQuiet: noop,
});

/** Mount once at the app root, inside <I18nProvider>, since the dismiss
 *  button's aria-label is translated. */
export function ToastProvider({ children }: { children: ReactNode }) {
  const { t } = useT();
  const [toasts, setToasts] = useState<ToastEntry[]>([]);
  const [quiet, setQuietState] = useState<boolean>(readStoredQuiet);
  // One live timer per toast id; pause always clears it before resume
  // schedules a new one.
  const timers = useRef(new Map<string, ReturnType<typeof setTimeout>>());
  const nextId = useRef(0);
  // Hover and focus engagement per toast id, in a ref because it never needs
  // a render of its own; the pause itself lives in `toasts`. A toast without
  // an entry is disengaged.
  const engagement = useRef(new Map<string, ToastEngagement>());

  const clearTimer = useCallback((id: string) => {
    const handle = timers.current.get(id);
    if (handle !== undefined) {
      clearTimeout(handle);
      timers.current.delete(id);
    }
  }, []);

  const dismiss = useCallback((id: string) => {
    clearTimer(id);
    engagement.current.delete(id);
    setToasts((list) => removeToast(list, id));
  }, [clearTimer]);

  const scheduleTimer = useCallback(
    (id: string, ms: number) => {
      clearTimer(id);
      timers.current.set(
        id,
        setTimeout(() => dismiss(id), ms)
      );
    },
    [clearTimer, dismiss]
  );

  const push = useCallback(
    (message: string, severity: ToastSeverity = "success", action?: ToastAction) => {
      if (!shouldShowToast(severity, quiet)) return;
      const id = `toast-${++nextId.current}`;
      setToasts((list) => {
        const next = addToast(list, { id, message, severity, action }, Date.now());
        // addToast caps the stack at MAX_VISIBLE_TOASTS by dropping the oldest
        // entries. A dropped toast still has the timer from its own push, so
        // clear it here rather than let it fire a stale dismiss, and drop its
        // engagement entry in case the cap had to evict a paused toast.
        for (const dropped of list) {
          if (!next.some((t) => t.id === dropped.id)) {
            clearTimer(dropped.id);
            engagement.current.delete(dropped.id);
          }
        }
        return next;
      });
      scheduleTimer(id, toastDuration({ action }));
    },
    [quiet, scheduleTimer, clearTimer]
  );

  // Stops the timer and freezes the remaining time. pauseToast ignores a toast
  // that is already paused, e.g. when focus arrives while it is hovered.
  const pause = useCallback(
    (id: string) => {
      clearTimer(id);
      setToasts((list) => pauseToast(list, id, Date.now()));
    },
    [clearTimer]
  );

  // Restarts the timer from the time pauseToast froze, not from the full
  // duration. Called only through setEngagement, once neither hover nor focus
  // is engaged.
  const resume = useCallback(
    (id: string) => {
      setToasts((list) => {
        const next = resumeToast(list, id, Date.now());
        const entry = next.find((toast) => toast.id === id);
        if (entry?.expiresAt != null) {
          scheduleTimer(id, Math.max(0, entry.expiresAt - Date.now()));
        }
        return next;
      });
    },
    [scheduleTimer]
  );

  // Handles the four hover and focus events. applyEngagement decides whether
  // an edge pauses or resumes; this keeps the per-id map and drives the timers.
  const setEngagement = useCallback(
    (id: string, kind: ToastEngagementKind, active: boolean) => {
      const { next, engaged } = applyEngagement(engagement.current.get(id) ?? NO_ENGAGEMENT, kind, active);
      // Only engaged toasts keep an entry, so the map does not grow with every
      // toast shown.
      if (engaged) engagement.current.set(id, next);
      else engagement.current.delete(id);
      if (active) pause(id);
      else if (!engaged) resume(id);
    },
    [pause, resume]
  );

  const handleMouseEnter = useCallback((id: string) => setEngagement(id, "hover", true), [setEngagement]);
  const handleMouseLeave = useCallback((id: string) => setEngagement(id, "hover", false), [setEngagement]);
  const handleFocus = useCallback((id: string) => setEngagement(id, "focus", true), [setEngagement]);
  const handleBlur = useCallback((id: string) => setEngagement(id, "focus", false), [setEngagement]);

  const setQuiet = useCallback((next: boolean) => {
    setQuietState(next);
    try {
      localStorage.setItem(QUIET_STORAGE_KEY, next ? "1" : "0");
    } catch {
      /* quiet mode just won't survive a reload */
    }
  }, []);

  // Clear every timer on unmount so no callback sets state afterwards.
  useEffect(() => {
    const timerMap = timers.current;
    return () => {
      timerMap.forEach((handle) => clearTimeout(handle));
      timerMap.clear();
    };
  }, []);

  const viewport = createPortal(
    <ToastViewport
      toasts={toasts}
      dismissLabel={t("toast.dismiss")}
      onDismiss={dismiss}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
      onFocus={handleFocus}
      onBlur={handleBlur}
    />,
    document.body
  );

  // Memoized so consumers such as Settings re-render only when `quiet`
  // changes, not whenever the queue does.
  const value = useMemo(() => ({ push, quiet, setQuiet }), [push, quiet, setQuiet]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      {viewport}
    </ToastContext.Provider>
  );
}

export function useToast() {
  return useContext(ToastContext);
}
