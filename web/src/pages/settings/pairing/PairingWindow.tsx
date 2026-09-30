// PairingWindow is the window the phrase card opens over itself: the twelve
// words to read out, or the field for another instance's words. It wears the
// hue of the card that opened it, and the footer holds its answers, one of
// them always the way out.
import { useEffect, useId, useRef, type CSSProperties, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Badge } from "../../../components/Badge";
import { InfoBubble } from "../../../components/InfoBubble";
import { hueVars } from "../../../lib/appearance";

export function PairingWindow({
  title,
  hint,
  hueIndex,
  onClose,
  footer,
  children,
}: {
  title: string;
  /** What the window is for, as the (i) in its title badge. */
  hint?: string;
  hueIndex?: number;
  onClose: () => void;
  footer: ReactNode;
  children: ReactNode;
}) {
  const titleId = useId();
  const cardRef = useRef<HTMLDivElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;

  useEffect(() => {
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") closeRef.current();
    };
    document.addEventListener("keydown", onKey);
    cardRef.current?.focus();
    return () => {
      document.removeEventListener("keydown", onKey);
      opener?.focus();
    };
  }, []);

  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={cardRef}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className={`glim-modal-card relative flex w-full max-w-3xl flex-col gap-5 rounded-card bg-carbon-surface p-5 shadow-2xl outline-none${
          hueIndex !== undefined ? " glim-hue" : ""
        }`}
        style={hueIndex !== undefined ? (hueVars(hueIndex) as CSSProperties) : undefined}
      >
        <h2 id={titleId} className="flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={hueIndex}>
            {title}
            {hint && <InfoBubble tip={hint} onAccent />}
          </Badge>
        </h2>
        {children}
        <div className="flex flex-wrap items-center justify-end gap-3">{footer}</div>
      </div>
    </div>,
    document.body,
  );
}
