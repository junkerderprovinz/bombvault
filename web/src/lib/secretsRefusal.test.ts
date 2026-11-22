import { describe, expect, it, vi } from "vitest";

import * as api from "./api";
import { en } from "./i18n";
import { flashZipRefusal } from "./secretsRefusal";

const t = (key: keyof typeof en) => en[key];

function answer(body: unknown) {
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify(body), { headers: { "content-type": "application/json" } })));
}

describe("flashZipRefusal", () => {
  it("lets the ZIP download without a login password", async () => {
    answer({ ok: true, enabled: false, authed: false });
    await api.getAuth();
    expect(flashZipRefusal(t)).toBeUndefined();
    vi.unstubAllGlobals();
  });

  it("asks a paired phone for its own sign-in", async () => {
    answer({ ok: true, enabled: true, authed: true, paired: true });
    await api.getAuth();
    expect(flashZipRefusal(t)).toBe(en["restoreMany.flashOwnLogin"]);
    answer({ ok: true, enabled: true, authed: true });
    await api.getAuth();
    vi.unstubAllGlobals();
  });
});
