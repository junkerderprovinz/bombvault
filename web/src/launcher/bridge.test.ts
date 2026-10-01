import { describe, expect, it } from "vitest";
import { bridgeOver, displayAddress, normalizeAddress, type Port } from "./bridge";

describe("normalizeAddress", () => {
  it("gives a bare host https, as BombVault serves by default", () => {
    expect(normalizeAddress("192.168.1.10:3443")).toBe("https://192.168.1.10:3443/");
  });

  it("keeps a scheme someone typed", () => {
    expect(normalizeAddress("http://tower.lan:3080")).toBe("http://tower.lan:3080/");
  });

  it("keeps a path for a server behind a reverse proxy", () => {
    expect(normalizeAddress(" https://home.example/bombvault ")).toBe("https://home.example/bombvault");
  });

  it("refuses what the app cannot open", () => {
    expect(normalizeAddress("")).toBeNull();
    expect(normalizeAddress("ftp://tower.lan")).toBeNull();
    expect(normalizeAddress("https://")).toBeNull();
  });
});

describe("displayAddress", () => {
  it("drops https and the trailing slash but keeps http, which matters", () => {
    expect(displayAddress("https://192.168.1.10:3443/")).toBe("192.168.1.10:3443");
    expect(displayAddress("http://tower.lan:3080/")).toBe("http://tower.lan:3080");
  });
});

describe("bridgeOver", () => {
  it("hands each fetch the answer with its own ticket", async () => {
    const sent: string[] = [];
    const listeners: ((e: { data: string }) => void)[] = [];
    const port: Port = {
      postMessage: (d) => sent.push(d),
      addEventListener: (_, fn) => listeners.push(fn),
      removeEventListener: () => undefined,
    };
    const bridge = bridgeOver(port);
    const first = bridge.fetch("a", "/api/runs");
    const second = bridge.fetch("b", "/api/runs");
    const [t1, t2] = sent.map((d) => (JSON.parse(d) as { ticket: number }).ticket);

    // Answers come back in whatever order the servers reply.
    for (const fn of listeners) fn({ data: JSON.stringify({ op: "fetched", ticket: t2, status: 401, body: "" }) });
    for (const fn of listeners) fn({ data: JSON.stringify({ op: "fetched", ticket: t1, status: 200, body: "{}" }) });

    expect(await first).toEqual({ status: 200, body: "{}" });
    expect(await second).toEqual({ status: 401, body: "" });
  });
});
