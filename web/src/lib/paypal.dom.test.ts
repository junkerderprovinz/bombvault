// @vitest-environment jsdom
// The SDK is a script tag, and the window waits on the promise it returns. A
// script that loads but never sets its namespace, as behind a content blocker,
// has to reject so the window can say so, and the next opening loads it again.
import { afterEach, expect, it } from "vitest";
import { loadPaypal, type PaypalConfig } from "./paypal";

const config: PaypalConfig = { clientId: "test", currency: "EUR", plans: { month: "m", year: "y" } };

afterEach(() => {
  document.head.innerHTML = "";
  delete (window as unknown as Record<string, unknown>)["paypalOnce"];
});

function sdkScript(): HTMLScriptElement {
  const script = document.head.querySelector<HTMLScriptElement>("script[data-namespace]");
  if (!script) throw new Error("no SDK script");
  return script;
}

it("rejects when the SDK loads without setting its namespace", async () => {
  const load = loadPaypal(config, false);
  sdkScript().dispatchEvent(new Event("load"));
  await expect(load).rejects.toThrow("paypal sdk");
  expect(document.head.querySelector("script[data-namespace]")).toBeNull();
});

it("resolves to the namespace the SDK set", async () => {
  const load = loadPaypal(config, false);
  const paypal = { Buttons: () => ({ render: async () => {}, close: async () => {} }) };
  (window as unknown as Record<string, unknown>)["paypalOnce"] = paypal;
  sdkScript().dispatchEvent(new Event("load"));
  await expect(load).resolves.toBe(paypal);
});
