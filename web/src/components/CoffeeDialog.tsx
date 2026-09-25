import { useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import { Badge } from "./Badge";
import { Button } from "./Button";
import { useT } from "../lib/i18n";
import { COFFEE_WIDGET } from "../lib/donate";

// The Buy Me a Coffee window: BMAC's own widget inside a house window, so a
// donor pays without leaving the app. The widget keeps BMAC's look and every
// payment BMAC offers there, card and wallet included.
//
// AboutCard mounts it only while it is open, so nothing from BMAC loads before
// somebody asks for it.

export function CoffeeDialog({ onClose }: { onClose: () => void }) {
  const { t } = useT();
  const cardRef = useRef<HTMLDivElement>(null);
  const title = "Buy Me a Coffee";

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    cardRef.current?.focus();
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      {/* As tall as the screen allows less a margin: BMAC's payment step is
          about 1200px, and every pixel here is scrolling a donor is spared. */}
      <div
        ref={cardRef}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-labelledby="coffee-title"
        className="glim-modal-card relative flex h-[calc(100vh-7rem)] w-full max-w-lg flex-col rounded-card bg-carbon-surface shadow-2xl"
      >
        <div className="flex items-start justify-between gap-4 px-5 py-4">
          <h2 id="coffee-title" className="flex items-center">
            <Badge tone="heading" size="heading" wrap>{title}</Badge>
          </h2>
        </div>

        <div className="flex min-h-0 flex-1 flex-col gap-4 px-5 pb-5">
          <p className="text-sm text-carbon-textSub">{t("about.coffeeIntro")}</p>
          <div className="flex min-h-0 flex-1 rounded-card bg-carbon-surface2 p-2">
            {/* White behind the frame so the first paint is not a dark hole on
                the dark theme; BMAC's page is light either way. */}
            <iframe
              src={COFFEE_WIDGET}
              title={title}
              allow="payment"
              className="min-h-0 w-full flex-1 rounded-control border-0 bg-white"
            />
          </div>
        </div>

        <div className="flex justify-end gap-2 px-5 pb-5">
          <Button label={t("common.close")} labelKey="common.close" tone="neutral" onClick={onClose} />
        </div>
      </div>
    </div>,
    document.body
  );
}
