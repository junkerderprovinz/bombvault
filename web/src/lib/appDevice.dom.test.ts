// @vitest-environment jsdom
// What the Android app says about the phone's screen: set before the page
// runs, then posted again whenever it changes.
import { afterEach, describe, expect, it, vi } from "vitest";

type Listener = (e: { data: string }) => void;

function app(device?: unknown) {
  const listeners: Listener[] = [];
  const posted: unknown[] = [];
  Object.assign(window, {
    bombvaultApp: {
      postMessage: (m: string) => posted.push(JSON.parse(m)),
      addEventListener: (_: string, fn: Listener) => listeners.push(fn),
      removeEventListener: (_: string, fn: Listener) => listeners.splice(listeners.indexOf(fn), 1),
    },
    ...(device === undefined ? {} : { bombvaultDevice: device }),
  });
  return { posted, say: (msg: unknown) => listeners.forEach((fn) => fn({ data: JSON.stringify(msg) })), listeners };
}

async function load() {
  vi.resetModules();
  return import("./appDevice");
}

afterEach(() => {
  delete (window as { bombvaultApp?: unknown }).bombvaultApp;
  delete (window as { bombvaultDevice?: unknown }).bombvaultDevice;
});

describe("the phone's screen", () => {
  it("says nothing outside the app", async () => {
    (window as { bombvaultDevice?: unknown }).bombvaultDevice = { oled: true };
    const { appDevice } = await load();
    expect(appDevice()).toBeUndefined();
  });

  it("says nothing in an app too old to tell", async () => {
    app();
    const { appDevice } = await load();
    expect(appDevice()).toBeUndefined();
  });

  it("reads what the app set before the page ran", async () => {
    app({ oled: true, accent: { light: "#3F5F91", dark: "#A8C8FF" } });
    const { appDevice } = await load();
    expect(appDevice()).toEqual({ oled: true, accent: { light: "#3F5F91", dark: "#A8C8FF" } });
  });

  it("drops a colour that is not one", async () => {
    app({ oled: false, accent: { light: "red", dark: "#A8C8FF" } });
    const { appDevice } = await load();
    expect(appDevice()).toEqual({ oled: false });
  });

  it("asks the app to report changes and follows them", async () => {
    const phone = app({ oled: false });
    const { appDevice, watchAppDevice } = await load();
    const seen: unknown[] = [];
    const stop = watchAppDevice((d) => seen.push(d));
    expect(phone.posted).toEqual([{ op: "device" }]);

    phone.say({ op: "state", servers: [] });
    phone.say({ op: "device", oled: true });
    expect(seen).toEqual([{ oled: true }]);
    expect(appDevice()).toEqual({ oled: true });

    stop();
    expect(phone.listeners).toHaveLength(0);
  });

  it("hands an OLED switch to the app and keeps it at once", async () => {
    const phone = app({ oled: false });
    const { appDevice, tellAppOled } = await load();
    tellAppOled(true);
    expect(phone.posted).toEqual([{ op: "oled", on: true }]);
    expect(appDevice()?.oled).toBe(true);
  });
});
