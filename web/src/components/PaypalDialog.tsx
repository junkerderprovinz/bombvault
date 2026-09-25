import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Badge } from "./Badge";
import { Button } from "./Button";
import { Selector } from "./Selector";
import { useT } from "../lib/i18n";
import { PAYPAL, PAYPAL_AMOUNTS, PAYPAL_DEFAULT_AMOUNT, PAYPAL_DESCRIPTION } from "../lib/donate";
import { parseAmount, type GiveFrequency } from "../lib/paypal";
import { usePaypalButtons } from "../lib/usePaypalButtons";

// The PayPal window: how often and how much in the house's own selectors, then
// PayPal's two buttons. The PayPal button opens PayPal's login popup; the card
// button opens a card form inside this window, for somebody without a PayPal
// account. PayPal draws its buttons itself and allows no more than a colour, a
// radius and a height, so they stand apart from the house controls above.
//
// AboutCard mounts it only while it is open, so the SDK is not fetched before
// somebody asks for it.

const FREQUENCIES: GiveFrequency[] = ["once", "month", "year"];

export function PaypalDialog({ onClose }: { onClose: () => void }) {
  const { t } = useT();
  const cardRef = useRef<HTMLDivElement>(null);
  const [frequency, setFrequency] = useState<GiveFrequency>("once");
  const [preset, setPreset] = useState(PAYPAL_DEFAULT_AMOUNT);
  const [typed, setTyped] = useState("");
  const [status, setStatus] = useState<"idle" | "done" | "failed">("idle");

  // A valid typed amount wins over the presets, so the window never shows two
  // amounts at once. A half-typed one charges nothing until it parses.
  const typedAmount = parseAmount(typed);
  const amount = typed === "" ? preset : typedAmount;

  const buttonsRef = usePaypalButtons({
    config: PAYPAL,
    frequency,
    amount,
    description: PAYPAL_DESCRIPTION,
    onDone: () => setStatus("done"),
    onError: () => setStatus("failed"),
  });

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    cardRef.current?.focus();
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  const labels: Record<GiveFrequency, string> = {
    once: t("about.paypalOnce"),
    month: t("about.paypalMonthly"),
    year: t("about.paypalYearly"),
  };

  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={cardRef}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-labelledby="paypal-window-title"
        className="glim-modal-card relative flex max-h-[85vh] w-full max-w-lg flex-col rounded-card bg-carbon-surface shadow-2xl"
      >
        <div className="flex items-start justify-between gap-4 px-5 py-4">
          <h2 id="paypal-window-title" className="flex items-center">
            <Badge tone="heading" size="heading" wrap>PayPal</Badge>
          </h2>
        </div>

        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-5 pb-5">
          <p className="text-sm text-carbon-text">{t("about.donateAppeal")}</p>
          <p className="text-sm text-carbon-textSub">{t("about.paypalIntro")}</p>

          <div className="flex flex-col gap-1.5">
            <span className="text-xs font-semibold uppercase tracking-widest text-carbon-textSub">
              {t("about.paypalFrequency")}
            </span>
            <Selector
              items={FREQUENCIES.map((f) => ({ id: f, label: labels[f] }))}
              label={t("about.paypalFrequency")}
              select="one"
              active={frequency}
              onChange={(id) => {
                setFrequency(id as GiveFrequency);
                setStatus("idle");
              }}
              variant="well"
              buttonHeight
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <span className="text-xs font-semibold uppercase tracking-widest text-carbon-textSub">
              {t("about.paypalAmount")}
            </span>
            <div className="flex flex-wrap items-center gap-2">
              <Selector
                items={PAYPAL_AMOUNTS.map((a) => ({ id: a, label: `${a} €` }))}
                label={t("about.paypalAmount")}
                // After the three frequencies, so the two strips do not repeat
                // each other's colours straight down.
                hueOffset={FREQUENCIES.length}
                select="one"
                active={typedAmount ? null : preset}
                onChange={(id) => {
                  setPreset(id);
                  setTyped("");
                }}
                variant="well"
                buttonHeight
              />
              <input
                type="text"
                inputMode="decimal"
                value={typed}
                placeholder={t("about.paypalOtherAmount")}
                aria-label={t("about.paypalOtherAmount")}
                aria-invalid={typed !== "" && !typedAmount}
                onChange={(e) => setTyped(e.target.value)}
                className={`h-9 w-36 rounded-control border bg-carbon-surface2 px-3 text-sm text-carbon-text glim-field-focus ${
                  typedAmount ? "border-accent" : "border-transparent"
                }`}
              />
            </div>
          </div>

          {/* PayPal's buttons are cross-origin frames drawn light. On the dark
              theme the browser would paint them an opaque white strip, so
              their box declares the light scheme. */}
          <div ref={buttonsRef} className="mt-3 min-h-28 [color-scheme:light]">
            <p className="py-2 text-center text-xs text-carbon-textMuted">{t("about.paypalLoading")}</p>
          </div>

          {status === "done" && <p className="text-sm text-statusOk">{t("about.paypalThanks")}</p>}
          {status === "failed" && <p className="text-sm text-statusFail">{t("about.paypalFailed")}</p>}
        </div>

        <div className="flex justify-end gap-2 px-5 pb-5">
          <Button label={t("common.close")} labelKey="common.close" tone="neutral" onClick={onClose} />
        </div>
      </div>
    </div>,
    document.body
  );
}
