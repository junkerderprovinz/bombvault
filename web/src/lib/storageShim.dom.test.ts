// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { guardStorage } from "./storageShim";

const own = {
  localStorage: Object.getOwnPropertyDescriptor(window, "localStorage"),
  sessionStorage: Object.getOwnPropertyDescriptor(window, "sessionStorage"),
};

function refuse(target: object, name: string) {
  Object.defineProperty(target, name, {
    configurable: true,
    get() {
      throw new DOMException("The operation is insecure.", "SecurityError");
    },
  });
}

afterEach(() => {
  for (const [name, descriptor] of Object.entries(own)) {
    if (descriptor) Object.defineProperty(window, name, descriptor);
    else delete (window as unknown as Record<string, unknown>)[name];
  }
  delete (document as { cookie?: string }).cookie;
});

describe("guardStorage", () => {
  it("leaves storage that works alone", () => {
    const before = window.localStorage;
    window.localStorage.setItem("bv-theme", "dark");
    guardStorage();
    expect(window.localStorage).toBe(before);
    expect(window.localStorage.getItem("bv-theme")).toBe("dark");
  });

  it("keeps what a sandboxed page writes for as long as it is open", () => {
    refuse(window, "localStorage");
    refuse(window, "sessionStorage");
    guardStorage();
    window.localStorage.setItem("bv-theme", "light");
    window.sessionStorage.setItem("bv-tab", "2");
    expect(window.localStorage.getItem("bv-theme")).toBe("light");
    expect(window.localStorage.length).toBe(1);
    expect(window.localStorage.key(0)).toBe("bv-theme");
    expect(window.sessionStorage.getItem("bv-tab")).toBe("2");
    window.localStorage.removeItem("bv-theme");
    expect(window.localStorage.getItem("bv-theme")).toBeNull();
  });

  it("gives a sandboxed page cookies of its own", () => {
    refuse(document, "cookie");
    guardStorage();
    document.cookie = "a=1; path=/";
    document.cookie = "b=2";
    expect(document.cookie).toBe("a=1; b=2");
    document.cookie = "a=; max-age=0";
    expect(document.cookie).toBe("b=2");
  });
});
