// @vitest-environment jsdom
// PayPal's SDK is a script from paypal.com, so these tests catch the script
// element the window creates and answer its load with a fake SDK that records
// the options each set of buttons was given. The handlers in those options are
// what PayPal calls when a donor clicks, so calling them here checks the order
// or subscription a donor would really be sent to approve.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

type Handler = (data: unknown, actions: unknown) => unknown;

interface FakeButtons {
  options: Record<string, unknown>;
  closed: boolean;
}

let scripts: HTMLScriptElement[] = [];
let buttons: FakeButtons[] = [];

beforeEach(() => {
  scripts = [];
  buttons = [];
  // A fresh module graph, so the SDK cache in lib/paypal.ts starts empty.
  vi.resetModules();
  const create = document.createElement.bind(document);
  vi.spyOn(document, "createElement").mockImplementation((tag: string, options?: ElementCreationOptions) => {
    const el = create(tag, options);
    if (tag === "script") scripts.push(el as HTMLScriptElement);
    return el;
  });
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  for (const s of document.head.querySelectorAll("script")) s.remove();
});

async function open() {
  const { I18nProvider, en } = await import("../lib/i18n");
  const { PaypalDialog } = await import("./PaypalDialog");
  render(
    <I18nProvider>
      <PaypalDialog onClose={() => {}} />
    </I18nProvider>
  );
  return en;
}

function sdkScripts() {
  return scripts.filter((s) => s.src.startsWith("https://www.paypal.com/sdk/js?"));
}

/** Answers the newest SDK script with a fake namespace, as its load would. */
async function loadSdk() {
  const script = sdkScripts().at(-1)!;
  const namespace = script.dataset.namespace!;
  (window as unknown as Record<string, unknown>)[namespace] = {
    Buttons(options: Record<string, unknown>) {
      const record: FakeButtons = { options, closed: false };
      buttons.push(record);
      return {
        render(box: HTMLElement) {
          const drawn = document.createElement("div");
          drawn.dataset.testid = "paypal-buttons";
          box.appendChild(drawn);
          return Promise.resolve();
        },
        close() {
          record.closed = true;
          return Promise.resolve();
        },
      };
    },
  };
  await act(async () => {
    script.onload?.(new Event("load"));
  });
  return script;
}

const handler = (name: string) => buttons.at(-1)!.options[name] as Handler;

/** What createOrder hands PayPal. */
function order() {
  const create = vi.fn((o: unknown) => Promise.resolve(o));
  handler("createOrder")(null, { order: { create, capture: () => Promise.resolve() } });
  return create.mock.calls[0]![0] as {
    purchase_units: {
      amount: { currency_code: string; value: string };
      items: { quantity: string; category: string; unit_amount: { value: string } }[];
    }[];
  };
}

function segments(name: string) {
  return within(screen.getByRole("tablist", { name })).getAllByRole("tab");
}

it("loads one SDK, for a one-off gift, in the browser's language", async () => {
  await open();
  const [script] = sdkScripts();
  expect(sdkScripts()).toHaveLength(1);
  const params = new URL(script!.src).searchParams;
  expect(params.get("client-id")).toBe(
    "BAAbFqgNYfuCIBT_gwVE64oqj-E-jmxFiLaoR1yMIF9KK-CW16x5Pt2bSjBloqbTF4TvjFYw3ZTLnRP8_U"
  );
  expect(params.get("currency")).toBe("EUR");
  expect(params.get("intent")).toBe("capture");
  expect(params.get("vault")).toBeNull();
  expect(params.get("disable-funding")).toContain("paylater");
  expect(params.get("locale")).toBeNull();
});

it("renders PayPal's buttons in the house style, in a light-scheme box", async () => {
  await open();
  await loadSdk();
  expect(buttons).toHaveLength(1);
  expect(buttons[0]!.options.style).toEqual({
    layout: "vertical",
    color: "blue",
    shape: "rect",
    borderRadius: 10,
    label: "donate",
    height: 40,
  });
  const box = document.querySelector('[data-testid="paypal-buttons"]')!.parentElement!;
  expect(box.className).toContain("[color-scheme:light]");
});

it("creates a one-off order for 25 EUR, marked as a donation", async () => {
  await open();
  await loadSdk();
  const unit = order().purchase_units[0]!;
  expect(unit.amount).toMatchObject({ currency_code: "EUR", value: "25" });
  expect(unit.items).toEqual([
    expect.objectContaining({ quantity: "1", category: "DONATION", unit_amount: expect.objectContaining({ value: "25" }) }),
  ]);
});

it("lets a typed amount take the selection from the presets, and gives it back when cleared", async () => {
  const en = await open();
  await loadSdk();
  const field = screen.getByRole("textbox", { name: en["about.paypalOtherAmount"] });
  const selected = () => segments(en["about.paypalAmount"]).map((s) => s.getAttribute("aria-selected"));

  expect(selected()).toEqual(["false", "true", "false"]);

  fireEvent.change(field, { target: { value: "12,50" } });
  expect(selected()).toEqual(["false", "false", "false"]);
  expect(field.className).toContain("border-accent");
  expect(order().purchase_units[0]!.amount.value).toBe("12.50");

  fireEvent.change(field, { target: { value: "" } });
  expect(selected()).toEqual(["false", "true", "false"]);
  expect(field.className).not.toContain("border-accent");
});

it("keeps PayPal shut while the typed amount does not parse", async () => {
  const en = await open();
  await loadSdk();
  fireEvent.change(screen.getByRole("textbox", { name: en["about.paypalOtherAmount"] }), {
    target: { value: "0,5" },
  });
  const resolve = vi.fn();
  const reject = vi.fn();
  handler("onClick")(null, { resolve, reject });
  expect(reject).toHaveBeenCalled();
  expect(resolve).not.toHaveBeenCalled();
});

it("switches Monthly to a subscription on the month plan, with a whole quantity", async () => {
  const en = await open();
  await loadSdk();
  const once = buttons[0]!;

  fireEvent.click(screen.getByRole("tab", { name: en["about.paypalMonthly"] }));
  const script = await loadSdk();
  const params = new URL(script.src).searchParams;
  expect(params.get("intent")).toBe("subscription");
  expect(params.get("vault")).toBe("true");
  expect(once.closed).toBe(true);

  fireEvent.change(screen.getByRole("textbox", { name: en["about.paypalOtherAmount"] }), {
    target: { value: "12,50" },
  });
  const create = vi.fn(() => Promise.resolve("I-1"));
  handler("createSubscription")(null, { subscription: { create } });
  expect(create).toHaveBeenCalledWith({ plan_id: "P-2ND5083133959702RNK2375A", quantity: "13" });
});

it("uses the year plan for Yearly", async () => {
  const en = await open();
  await loadSdk();
  fireEvent.click(screen.getByRole("tab", { name: en["about.paypalYearly"] }));
  await loadSdk();
  const create = vi.fn(() => Promise.resolve("I-1"));
  handler("createSubscription")(null, { subscription: { create } });
  expect(create).toHaveBeenCalledWith({ plan_id: "P-2FN843952N550243RNK2375A", quantity: "25" });
});

it("thanks the donor once PayPal approves", async () => {
  const en = await open();
  await loadSdk();
  await act(async () => {
    await handler("onApprove")(null, { order: { capture: () => Promise.resolve() } });
  });
  expect(screen.getByText(en["about.paypalThanks"])).toBeTruthy();
});

it("says so when the SDK does not load", async () => {
  const en = await open();
  await act(async () => {
    sdkScripts()[0]!.onerror?.(new Event("error"));
  });
  expect(screen.getByText(en["about.paypalFailed"])).toBeTruthy();
});

it("gives no element the id paypal, which would shadow the SDK's global", async () => {
  await open();
  await loadSdk();
  expect(document.getElementById("paypal")).toBeNull();
});
