// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";

async function under(href: string | null) {
  document.head.querySelector("base")?.remove();
  if (href !== null) {
    const base = document.createElement("base");
    base.href = href;
    document.head.append(base);
  }
  vi.resetModules();
  return import("./base");
}

afterEach(() => {
  document.head.querySelector("base")?.remove();
});

describe("base", () => {
  it("leaves the paths at the root alone", async () => {
    const b = await under("/");
    expect(b.basePath()).toBe("/");
    expect(b.withinBase("/api/health")).toBe("/api/health");
    expect(b.routerBase()).toBe("/");
  });

  it("puts the paths under the prefix the server named", async () => {
    const b = await under("/fortress/open/0123abcd/");
    expect(b.basePath()).toBe("/fortress/open/0123abcd/");
    expect(b.withinBase("/api/progress")).toBe("/fortress/open/0123abcd/api/progress");
    expect(b.routerBase()).toBe("/fortress/open/0123abcd");
  });

  it("leaves another address as it is", async () => {
    const b = await under("/bombvault/");
    expect(b.withinBase("https://example.com/x")).toBe("https://example.com/x");
    expect(b.withinBase("//example.com/x")).toBe("//example.com/x");
    expect(b.withinBase("api/health")).toBe("api/health");
  });
});
