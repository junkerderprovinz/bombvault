import { useEffect, useRef } from "react";
import { donationHandlers, loadPaypal, type GiveFrequency, type PaypalButtons, type PaypalConfig } from "./paypal";

interface ClickActions {
  resolve(): unknown;
  reject(): unknown;
}

/**
 * usePaypalButtons renders PayPal's two buttons into the returned ref, and
 * again whenever the frequency changes, since a subscription needs the other
 * SDK namespace. The amount is read at click time, so picking another amount
 * reloads nothing.
 */
export function usePaypalButtons({
  config,
  frequency,
  amount,
  description,
  onDone,
  onError,
}: {
  config: PaypalConfig;
  frequency: GiveFrequency;
  /** The amount to charge, or null while the typed amount is not valid. */
  amount: string | null;
  description: string;
  onDone: () => void;
  onError: () => void;
}) {
  const container = useRef<HTMLDivElement>(null);
  const latest = useRef({ amount, onDone, onError });
  useEffect(() => {
    latest.current = { amount, onDone, onError };
  });

  useEffect(() => {
    const box = container.current;
    if (!box) return;
    let buttons: PaypalButtons | null = null;
    let cancelled = false;

    loadPaypal(config, frequency !== "once").then(
      (paypal) => {
        if (cancelled) return;
        const instance = paypal.Buttons({
          // Blue is one of the five colours PayPal allows, and reads on both themes.
          style: { layout: "vertical", color: "blue", shape: "rect", borderRadius: 10, label: "donate", height: 40 },
          // A half-typed amount keeps the buttons from opening PayPal at all.
          onClick: (_: unknown, actions: ClickActions) =>
            latest.current.amount ? actions.resolve() : actions.reject(),
          ...donationHandlers(
            config,
            frequency,
            () => latest.current.amount ?? "0",
            description,
            () => latest.current.onDone()
          ),
          onError: () => latest.current.onError(),
        });
        buttons = instance;
        box.replaceChildren();
        instance.render(box).catch(() => latest.current.onError());
      },
      () => {
        if (!cancelled) latest.current.onError();
      }
    );

    return () => {
      cancelled = true;
      buttons?.close().catch(() => {});
    };
  }, [config, frequency, description]);

  return container;
}
