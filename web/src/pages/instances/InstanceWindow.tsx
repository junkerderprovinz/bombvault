import { useEffect, useRef, type CSSProperties, type ReactNode } from "react";
import { createPortal } from "react-dom";

import { Badge } from "../../components/Badge";
import { InfoBubble } from "../../components/InfoBubble";
import { BottomSheet } from "../../components/mobile/BottomSheet";
import { useBackCloses } from "../../lib/useBackCloses";
import { hueVars } from "../../lib/appearance";
import { focusableElements } from "../../lib/useConfirm";
import { useIsDesktop } from "../../lib/useMediaQuery";

/** InstanceWindow is the window a card of the grid opens: a centred box on
 *  the desktop and a full sheet on a phone. Its body scrolls between the
 *  heading and the footer, which holds the way out. */
export function InstanceWindow({
  title,
  hint,
  facts,
  footer,
  wide = false,
  onClose,
  children,
}: {
  title: string;
  /** What the window is for, as the (i) in its heading. */
  hint?: string;
  /** A line under the title. With it the desktop box is headed by the plain
   *  title and this line, without it by the title as a notch. */
  facts?: ReactNode;
  footer: ReactNode;
  /** Room for a grid of tiles, four to a row. */
  wide?: boolean;
  onClose: () => void;
  children: ReactNode;
}) {
  const isDesktop = useIsDesktop();
  const boxRef = useRef<HTMLDivElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  useBackCloses(isDesktop, onClose);

  useEffect(() => {
    if (!isDesktop) return;
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const onKey = (e: KeyboardEvent) => {
      const box = boxRef.current;
      // A dialog opened from inside the window answers the keys first.
      const dialogs = document.querySelectorAll('[role="dialog"], [role="alertdialog"]');
      if (!box || dialogs[dialogs.length - 1] !== box) return;
      if (e.key === "Escape") {
        closeRef.current();
        return;
      }
      if (e.key !== "Tab") return;
      const focusables = focusableElements(box);
      if (focusables.length === 0) return;
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      const active = document.activeElement;
      const inside = active instanceof Node && box.contains(active);
      if (e.shiftKey && (!inside || active === first || active === box)) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && (!inside || active === last)) {
        e.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", onKey);
    boxRef.current?.focus();
    return () => {
      document.removeEventListener("keydown", onKey);
      opener?.focus();
    };
  }, [isDesktop]);

  if (!isDesktop) {
    return (
      <BottomSheet
        open
        onClose={onClose}
        title={title}
        hint={hint}
        fullHeight
        headerClose={false}
        footer={<div className="flex flex-wrap items-center justify-end gap-2 py-3">{footer}</div>}
      >
        <div className="flex flex-col gap-6 pb-4 pt-1">
          {facts && <p className="text-xs text-carbon-textMuted wrap-anywhere">{facts}</p>}
          {children}
        </div>
      </BottomSheet>
    );
  }

  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={boxRef}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className={`glim-modal-card relative flex max-h-[90vh] w-full ${wide ? "max-w-[66.25rem]" : "max-w-3xl"} flex-col rounded-card bg-carbon-surface shadow-2xl outline-none`}
      >
        {facts ? (
          <div className="flex flex-col gap-1 px-6 pt-6">
            <h2 className="flex items-center gap-1.5 text-title font-semibold text-carbon-text wrap-anywhere">
              {title}
              {hint && <InfoBubble tip={hint} />}
            </h2>
            <p className="text-sm text-carbon-textSub wrap-anywhere">{facts}</p>
          </div>
        ) : (
          <h2 className="flex items-center px-6">
            <Badge tone="heading" size="heading" wrap>
              {title}
              {hint && <InfoBubble tip={hint} onAccent />}
            </Badge>
          </h2>
        )}
        <div className="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto px-6 pb-2 pt-6">{children}</div>
        <div className="flex flex-wrap items-center justify-end gap-2 px-6 pb-6 pt-4">{footer}</div>
      </div>
    </div>,
    document.body,
  );
}

/** WindowCard is a card inside a window. It sits one step up from the
 *  window's surface, so its rows and buttons keep a ground of their own. */
export function WindowCard({
  title,
  hint,
  hueIndex,
  children,
}: {
  title: string;
  hint?: string;
  hueIndex: number;
  children: ReactNode;
}) {
  return (
    <section
      aria-label={title}
      className="relative glim-notch-card glim-hue flex flex-col gap-3 rounded-card bg-carbon-surface2 p-3 pt-6 md:p-5 md:pt-7"
      style={hueVars(hueIndex) as CSSProperties}
    >
      <h3 className="flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={hueIndex}>
          {title}
          {hint && <InfoBubble tip={hint} onAccent />}
        </Badge>
      </h3>
      {children}
    </section>
  );
}
