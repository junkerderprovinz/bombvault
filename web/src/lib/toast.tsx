import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { ToastViewport } from "../components/Toast";
import {
  TOAST_DURATION_MS,
  addToast,
  pauseToast,
  removeToast,
  resumeToast,
  shouldShowToast,
  type ToastEntry,
  type ToastSeverity,
} from "./toastEngine";
import { useT } from "./i18n";

// ---------------------------------------------------------------------------
// useToast / ToastProvider — the stateful half of the GlimStone toast system
// (form-engine Task 9, design-language.md "Toasts"), a Context + Provider
// mounted once at the app root — the same shape as lib/advanced.tsx's
// AdvancedProvider (this repo's existing precedent for a global,
// localStorage-persisted client preference), extended here to also own a
// live queue instead of a single boolean.
//
// Mirrors useConfirm.tsx/useReveal.ts's split: the pure component
// (components/Toast.tsx) never touches `document` or a hook, so it stays
// directly callable in a node-environment test; every real-timer/DOM
// concern lives HERE instead:
//   - setTimeout handles per toast (scheduled on push/resume, cleared on
//     pause/dismiss) — toastEngine.ts's pure pause/resume MATH is what this
//     file's timers are built on top of; see that file's header comment for
//     why the pause/resume precision itself is tested there, not here.
//   - createPortal(..., document.body) — same fix as useConfirm.tsx/
//     InfoBubble.tsx: a `position: fixed` viewport nested under any
//     .bv-page-enter/.bv-modal-card ancestor (both use `transform`, which
//     creates a new containing block) would be clipped to that ancestor's
//     box instead of covering the real viewport.
//   - quiet-mode persistence (localStorage), read once at mount exactly
//     like AdvancedProvider's own `advanced` flag.
//   - hover/focus ENGAGEMENT tracking, kept as two independent booleans per
//     toast id (the `engagement` ref map below) rather than collapsing both
//     into a single pause/resume call. "Hover OR focus pauses" only holds
//     together if resume asks "are BOTH now disengaged?" instead of "did
//     THIS ONE event just end?" — otherwise Tab-ing into a hovered toast
//     then moving the mouse away (focus is still very much inside it) fires
//     onMouseLeave, which would resume and auto-dismiss the toast out from
//     under a keyboard user's focus, snapping `document.activeElement` back
//     to <body>. The mirror order (focus first, then hover-then-unhover)
//     has the same root cause. See handleMouseEnter/Leave/Focus/Blur below.
// This hook itself is NOT unit tested for the same reason useConfirm.tsx
// isn't (see that file's header comment): real timers + `document` are
// exactly what a node-environment test can't exercise directly. Covered by
// live Playwright verification instead — precisely timed hover-pause-then-
// resume measurements (including the combined hover+focus case above),
// multi-toast stacking (bounded at toastEngine.MAX_VISIBLE_TOASTS — see that
// file), keyboard dismissal, and the non-blocking click-through requirement.
//
// Adoption note: this file, plus Fleet.tsx's CopyBlock and Settings.tsx's
// VMSSHCard/handleSetPassword and Config.tsx's ConfigSettingsCard, are 4
// self-contained proof-of-adoption sites. The ~35 remaining inline-status
// sites across this codebase (most of them the `SaveBar` component's ~30
// call sites in Settings.tsx, which all share one generic `save()` helper)
// are DELIBERATELY left on their original inline-flash behaviour — see the
// SaveBar comment at the top of Settings.tsx for why converting that shared
// helper is out of scope here and left as explicit, deliberate follow-up
// work rather than something folded into this task silently.
// ---------------------------------------------------------------------------

const QUIET_STORAGE_KEY = "bombvault.quietToasts";

function readStoredQuiet(): boolean {
  try {
    return localStorage.getItem(QUIET_STORAGE_KEY) === "1";
  } catch {
    return false;
  }
}

interface ToastContextValue {
  /** Queue a toast. Severity defaults to "success" (the routine,
   *  quiet-mode-suppressible case). A push that quiet mode filters out is a
   *  silent no-op — it never queues invisibly for later; see
   *  toastEngine.shouldShowToast for the exact rule. */
  push: (message: string, severity?: ToastSeverity) => void;
  /** Current quiet-toasts preference (Settings › General › Appearance). */
  quiet: boolean;
  setQuiet: (next: boolean) => void;
}

const noop = () => {};

// Safe default so useToast() never throws outside a Provider (mirrors
// i18n.ts's I18nContext default for the same reason: tests / early renders).
const ToastContext = createContext<ToastContextValue>({
  push: noop,
  quiet: false,
  setQuiet: noop,
});

/** Mount once at the app root, inside <I18nProvider> (the dismiss button's
 *  aria-label needs a live translation) — see app/router.tsx. */
export function ToastProvider({ children }: { children: ReactNode }) {
  const { t } = useT();
  const [toasts, setToasts] = useState<ToastEntry[]>([]);
  const [quiet, setQuietState] = useState<boolean>(readStoredQuiet);
  // One live setTimeout handle per toast id — never more than one at a time
  // per toast (pause always clears before resume schedules a new one), so a
  // Map keyed by id is enough to find-and-clear the right timer on dismiss/
  // pause without touching any other toast's timer.
  const timers = useRef(new Map<string, ReturnType<typeof setTimeout>>());
  const nextId = useRef(0);
  // Hover and focus engagement, tracked as two INDEPENDENT booleans per
  // toast id — see this file's header comment for the bug this exists to
  // fix. A ref, not React state: this bookkeeping never needs to trigger a
  // re-render on its own — only the actual pause/resume (which does live in
  // `toasts` state, below) does.
  const engagement = useRef(new Map<string, { hover: boolean; focus: boolean }>());

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
    (message: string, severity: ToastSeverity = "success") => {
      if (!shouldShowToast(severity, quiet)) return;
      const id = `toast-${++nextId.current}`;
      setToasts((list) => {
        const next = addToast(list, { id, message, severity }, Date.now());
        // addToast caps the stack at MAX_VISIBLE_TOASTS, dropping the OLDEST
        // entries once the cap is exceeded (toastEngine.ts — the fix for the
        // unbounded-stack bug: rapid repeated triggers, e.g. holding Enter
        // on a focused button at keyboard auto-repeat rate, used to grow the
        // stack without limit). A dropped toast still has a live setTimeout
        // scheduled from its own earlier push (and may be mid-hover/focus),
        // so clean both up now — otherwise a stray timer fires a pointless
        // dismiss(id) later against an id that's already gone, and the
        // engagement map would grow unbounded across a long session.
        for (const dropped of list) {
          if (!next.some((t) => t.id === dropped.id)) {
            clearTimer(dropped.id);
            engagement.current.delete(dropped.id);
          }
        }
        return next;
      });
      scheduleTimer(id, TOAST_DURATION_MS);
    },
    [quiet, scheduleTimer, clearTimer]
  );

  // Hover/focus enters a toast: stop its live timer (so it can never fire
  // mid-hover/mid-focus) and freeze remainingMs at the pure engine's own
  // math. A no-op via pauseToast's own already-paused guard when the OTHER
  // engagement flag already paused it (e.g. focus arrives while the toast is
  // already hover-paused) — see toastEngine.pauseToast.
  const pause = useCallback(
    (id: string) => {
      clearTimer(id);
      setToasts((list) => pauseToast(list, id, Date.now()));
    },
    [clearTimer]
  );

  // Restart the timer from whatever pauseToast froze — NOT the full
  // TOAST_DURATION_MS again (that would be the exact "resets to full
  // duration" bug the spec calls out by name). Only ever invoked once BOTH
  // engagement flags read false — see handleMouseLeave/handleBlur below,
  // never called directly from DOM event wiring.
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

  // The four DOM-facing handlers actually wired to ToastViewport (below).
  // pause() fires on either engagement edge unconditionally — harmless,
  // since pauseToast is itself a no-op once already paused. resume() is the
  // one that needs the combined check: it only fires once BOTH `hover` and
  // `focus` read false for this id, which is the actual fix for "hover away
  // while still Tab-focused inside the toast (or the mirror order) must NOT
  // resume the countdown out from under the user."
  const engagementFor = useCallback((id: string) => {
    let e = engagement.current.get(id);
    if (!e) {
      e = { hover: false, focus: false };
      engagement.current.set(id, e);
    }
    return e;
  }, []);

  const handleMouseEnter = useCallback(
    (id: string) => {
      engagementFor(id).hover = true;
      pause(id);
    },
    [engagementFor, pause]
  );

  const handleMouseLeave = useCallback(
    (id: string) => {
      const e = engagementFor(id);
      e.hover = false;
      if (!e.focus) resume(id);
    },
    [engagementFor, resume]
  );

  const handleFocus = useCallback(
    (id: string) => {
      engagementFor(id).focus = true;
      pause(id);
    },
    [engagementFor, pause]
  );

  const handleBlur = useCallback(
    (id: string) => {
      const e = engagementFor(id);
      e.focus = false;
      if (!e.hover) resume(id);
    },
    [engagementFor, resume]
  );

  const setQuiet = useCallback((next: boolean) => {
    setQuietState(next);
    try {
      localStorage.setItem(QUIET_STORAGE_KEY, next ? "1" : "0");
    } catch {
      /* ignore — quiet mode just won't survive a reload */
    }
  }, []);

  // Every live timer is cleared on unmount so a stray callback can never
  // setState after ToastProvider goes away (ToastProvider mounts once at
  // the app root and in practice never unmounts, but this plan has already
  // hit a real leaked-timer/leaked-state bug once — see RevealInput.tsx's
  // sibling fix commit — so this is the same discipline applied here).
  useEffect(() => {
    const timerMap = timers.current;
    return () => {
      timerMap.forEach((handle) => clearTimeout(handle));
      timerMap.clear();
    };
  }, []);

  // Portal-rendered to <body> — see this file's header comment for why
  // (the same transform-containing-block bug useConfirm.tsx/InfoBubble.tsx
  // already fixed for their own fixed-position overlays).
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

  // Memoized so every consumer of useToast() (every page that pushes a
  // toast, including large ones like Settings) doesn't re-render just
  // because ToastProvider itself re-rendered — push/setQuiet are already
  // stable useCallback references, so only an actual `quiet` change should
  // ever produce a new context value here.
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
