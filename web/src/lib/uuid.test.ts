// randomId must keep working on the plain-HTTP origins BombVault documents
// (HTTP_ONLY=true / the template's "WebUI Port (HTTP)"), where the page is NOT
// a secure context and crypto.randomUUID is therefore `undefined`. Same class
// of gate as copyText's clipboard fallback — see clipboard.test.ts.
import { afterEach, describe, expect, it, vi } from "vitest";
import { randomId } from "./uuid";

const V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

describe("randomId", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("uses crypto.randomUUID when the page IS a secure context", () => {
    const randomUUID = vi.fn().mockReturnValue("11111111-2222-4333-8444-555555555555");
    vi.stubGlobal("crypto", { randomUUID, getRandomValues: vi.fn() });
    expect(randomId()).toBe("11111111-2222-4333-8444-555555555555");
    expect(randomUUID).toHaveBeenCalled();
  });

  it("falls back to getRandomValues when randomUUID is missing (insecure origin)", () => {
    // crypto.randomUUID is secure-context-only; getRandomValues is not. On
    // http://<lan-ip>:<port> only the latter exists — this is the real shape
    // of the environment, not a hypothetical one.
    const getRandomValues = vi.fn((b: Uint8Array) => {
      for (let i = 0; i < b.length; i++) b[i] = i * 7 + 3;
      return b;
    });
    vi.stubGlobal("crypto", { getRandomValues });
    const id = randomId();
    expect(getRandomValues).toHaveBeenCalled();
    expect(id).toMatch(V4);
  });

  it("still returns a v4-shaped id when no crypto object exists at all", () => {
    vi.stubGlobal("crypto", undefined);
    expect(randomId()).toMatch(V4);
  });

  it("does not repeat itself on the insecure path", () => {
    vi.stubGlobal("crypto", {
      getRandomValues: (b: Uint8Array) => {
        for (let i = 0; i < b.length; i++) b[i] = Math.floor(Math.random() * 256);
        return b;
      },
    });
    const ids = new Set(Array.from({ length: 500 }, () => randomId()));
    expect(ids.size).toBe(500);
  });
});
