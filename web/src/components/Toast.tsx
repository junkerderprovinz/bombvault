import type { ToastAction, ToastSeverity } from "../lib/toastEngine";
import { Button } from "./Button";

// The presentational half of the toast system. These components use no hooks,
// so tests can call them as plain functions; the queue, timers and portal live
// in lib/toast.tsx. The viewport ignores pointer events and only the cards take
// them, so a toast never blocks a click on the page around it.

export interface ToastCardProps {
  id: string;
  message: string;
  severity: ToastSeverity;
  /** Taking the action also dismisses the toast, since what it offered is done. */
  action?: ToastAction;
  /** Accessible name for the dismiss button, the same for every toast. */
  dismissLabel: string;
  onDismiss: (id: string) => void;
  onMouseEnter: (id: string) => void;
  onMouseLeave: (id: string) => void;
  onFocus: (id: string) => void;
  onBlur: (id: string) => void;
}

// Severity shows only in the glyph colour. A coloured rail on the card would
// break under the square corner setting.
const SEVERITY_ICON_CLASS: Record<ToastSeverity, string> = {
  success: "text-statusOkSolid",
  warn: "text-statusWarnSolid",
  fail: "text-statusFailSolid",
};

// Failures and warnings interrupt the screen reader (role="alert" is
// assertive); a success waits its turn (role="status" is polite). Each toast is
// inserted fresh, so the roles announce it without a separate live region.
function roleFor(severity: ToastSeverity): "alert" | "status" {
  return severity === "success" ? "status" : "alert";
}

// Solid badges with the mark drawn in the card's surface colour, so it reads as
// a cutout.
function ToastGlyph({ severity }: { severity: ToastSeverity }) {
  const className = `mt-0.5 shrink-0 ${SEVERITY_ICON_CLASS[severity]}`;
  const cutout = "var(--carbon-surface, transparent)";
  if (severity === "fail") {
    return (
      <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true" className={className}>
        <circle cx="8" cy="8" r="6.4" />
        <rect x="7.3" y="4.6" width="1.4" height="4" rx="0.7" fill={cutout} />
        <circle cx="8" cy="10.9" r="0.85" fill={cutout} />
      </svg>
    );
  }
  if (severity === "warn") {
    return (
      <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true" className={className}>
        <path d="M8 2.2 14.5 13.6H1.5L8 2.2Z" />
        <rect x="7.3" y="6.6" width="1.4" height="3" rx="0.7" fill={cutout} />
        <circle cx="8" cy="11.9" r="0.85" fill={cutout} />
      </svg>
    );
  }
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true" className={className}>
      <circle cx="8" cy="8" r="6.4" />
      <rect x="4.5" y="8.4" width="3.5" height="1.5" rx="0.75" fill={cutout} transform="rotate(45 6.25 9.15)" />
      <rect x="5.815" y="7.25" width="6.27" height="1.5" rx="0.75" fill={cutout} transform="rotate(-50.2 8.95 8)" />
    </svg>
  );
}

export function ToastCard({
  id,
  message,
  severity,
  action,
  dismissLabel,
  onDismiss,
  onMouseEnter,
  onMouseLeave,
  onFocus,
  onBlur,
}: ToastCardProps) {
  return (
    <div
      role={roleFor(severity)}
      // React's onFocus/onBlur bubble, so focusing the dismiss button pauses
      // the countdown just as hovering does.
      onMouseEnter={() => onMouseEnter(id)}
      onMouseLeave={() => onMouseLeave(id)}
      onFocus={() => onFocus(id)}
      onBlur={() => onBlur(id)}
      onKeyDown={(e) => {
        if (e.key === "Escape") {
          e.stopPropagation();
          onDismiss(id);
        }
      }}
      className="glim-toast pointer-events-auto flex w-80 max-w-[calc(100vw-2rem)] items-start gap-2.5 rounded-card bg-carbon-surface px-3.5 py-3 text-carbon-text"
    >
      <ToastGlyph severity={severity} />
      <p className="min-w-0 flex-1 text-sm leading-snug wrap-break-word">{message}</p>
      {action && (
        <Button
          label={action.label}
          labelKey={null}
          tone="neutral"
          onClick={() => {
            action.onClick();
            onDismiss(id);
          }}
          className="shrink-0"
        />
      )}
      <button
        type="button"
        onClick={() => onDismiss(id)}
        aria-label={dismissLabel}
        className="-m-1 shrink-0 rounded-control p-1 text-carbon-textMuted opacity-80 hover:bg-carbon-hover hover:text-carbon-text hover:opacity-100 focus-visible:opacity-100 focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--focus-ring)"
      >
        <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
          <rect x="6.8" y="2.5" width="2.4" height="11" rx="0.6" transform="rotate(45 8 8)" />
          <rect x="6.8" y="2.5" width="2.4" height="11" rx="0.6" transform="rotate(-45 8 8)" />
        </svg>
      </button>
    </div>
  );
}

export interface ToastViewportEntry {
  id: string;
  message: string;
  severity: ToastSeverity;
  action?: ToastAction;
}

export interface ToastViewportProps {
  toasts: ToastViewportEntry[];
  dismissLabel: string;
  onDismiss: (id: string) => void;
  // Hover and focus stay separate events because the caller resumes the
  // countdown only once both have left: moving the mouse off a toast that
  // still has keyboard focus must not resume it.
  onMouseEnter: (id: string) => void;
  onMouseLeave: (id: string) => void;
  onFocus: (id: string) => void;
  onBlur: (id: string) => void;
}

// ToastViewport stacks toasts in the bottom-end corner, newest at the bottom.
// The engine caps how many are visible; the height limit and scrolling only
// matter on a very short viewport. The inset is padding rather than an offset
// because overflow-y: auto also clips horizontally, and a flush box would cut
// off each card's shadow and the start of its slide-in.
export function ToastViewport({ toasts, dismissLabel, onDismiss, onMouseEnter, onMouseLeave, onFocus, onBlur }: ToastViewportProps) {
  return (
    <div className="pointer-events-none fixed bottom-0 end-0 z-[70] flex max-h-screen flex-col gap-2 overflow-y-auto p-4">
      {toasts.map((t) => (
        <ToastCard
          key={t.id}
          id={t.id}
          message={t.message}
          severity={t.severity}
          action={t.action}
          dismissLabel={dismissLabel}
          onDismiss={onDismiss}
          onMouseEnter={onMouseEnter}
          onMouseLeave={onMouseLeave}
          onFocus={onFocus}
          onBlur={onBlur}
        />
      ))}
    </div>
  );
}
