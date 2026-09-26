// PayPal's JavaScript SDK for the PayPal window, and the order or subscription
// it creates. GlimStone's reference/paypal.ts, typed without `any`.
//
// A one-off donation and a recurring one need the SDK loaded with different
// parameters (`intent=capture` against `intent=subscription&vault=true`), so
// each gets its own namespace and both can live on one page. Nothing loads
// until somebody opens the window.
//
// Two things break the SDK silently. An element with `id="paypal"` becomes the
// global `window.paypal` before the SDK can claim it, and a page-level
// function named `open` or `close` replaces the `window.open` the SDK uses for
// its login popup. Neither shows more than a TypeError deep inside PayPal's
// bundle.

export type GiveFrequency = "once" | "month" | "year";

export interface PaypalConfig {
  /** The public client id of the maker's live PayPal app. */
  clientId: string;
  /** One plan per interval, each priced at 1 EUR per unit, so any whole amount
   *  is the plan times a quantity. */
  plans: { month: string; year: string };
  currency: string;
}

/** The slice of the SDK this window uses. */
export interface PaypalNamespace {
  Buttons(options: Record<string, unknown>): PaypalButtons;
}

export interface PaypalButtons {
  render(container: HTMLElement): Promise<void>;
  close(): Promise<void>;
}

interface OrderActions {
  order: { create(order: unknown): Promise<string>; capture(): Promise<unknown> };
}

interface SubscriptionActions {
  subscription: { create(subscription: unknown): Promise<string> };
}

// Only the PayPal wallet and the card button: the other funding sources PayPal
// adds by region crowd the window and duplicate what the card button covers.
const DISABLED_FUNDING = "sepa,paylater,venmo,bancontact,blik,eps,giropay,ideal,mybank,p24,sofort";

const loads = new Map<string, Promise<PaypalNamespace>>();

// No locale parameter: PayPal follows the browser's language, and a locale it
// does not know stops the SDK from loading at all.
export function loadPaypal(config: PaypalConfig, recurring: boolean): Promise<PaypalNamespace> {
  const namespace = recurring ? "paypalRecurring" : "paypalOnce";
  const params = new URLSearchParams({
    "client-id": config.clientId,
    currency: config.currency,
    intent: recurring ? "subscription" : "capture",
    components: "buttons",
    "disable-funding": DISABLED_FUNDING,
  });
  if (recurring) params.set("vault", "true");
  const src = `https://www.paypal.com/sdk/js?${params}`;

  let load = loads.get(src);
  if (!load) {
    load = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = src;
      script.dataset.namespace = namespace;
      script.onload = () => resolve((window as unknown as Record<string, PaypalNamespace>)[namespace]!);
      script.onerror = () => {
        loads.delete(src);
        script.remove();
        reject(new Error("paypal sdk"));
      };
      document.head.appendChild(script);
    });
    loads.set(src, load);
  }
  return load;
}

/** A typed amount as PayPal's decimal string, or null below 1 or with more
 *  than two decimals. Accepts a comma as the decimal mark. */
export function parseAmount(text: string): string | null {
  const v = text.replace(",", ".").trim();
  if (!/^\d+(\.\d{1,2})?$/.test(v) || Number(v) < 1) return null;
  return Number(v).toFixed(2).replace(/\.00$/, "");
}

/** The button handlers for one donation. `amount` is read when the donor
 *  clicks, as a decimal string such as "25" or "12.50"; a recurring donation
 *  rounds it to whole units. */
export function donationHandlers(
  config: PaypalConfig,
  frequency: GiveFrequency,
  amount: () => string,
  description: string,
  onDone: () => void
): Record<string, unknown> {
  if (frequency === "once") {
    return {
      createOrder: (_: unknown, actions: OrderActions) => {
        const money = { currency_code: config.currency, value: amount() };
        return actions.order.create({
          purchase_units: [
            {
              description,
              amount: { ...money, breakdown: { item_total: money } },
              // DONATION marks the payment as a gift in the donor's PayPal history.
              items: [{ name: description, quantity: "1", category: "DONATION", unit_amount: money }],
            },
          ],
        });
      },
      onApprove: (_: unknown, actions: OrderActions) => actions.order.capture().then(onDone),
    };
  }
  return {
    createSubscription: (_: unknown, actions: SubscriptionActions) =>
      actions.subscription.create({
        plan_id: config.plans[frequency],
        quantity: String(Math.max(1, Math.round(Number(amount())))),
      }),
    onApprove: () => onDone(),
  };
}
