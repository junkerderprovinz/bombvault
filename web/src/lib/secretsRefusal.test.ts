import { describe, expect, it, vi } from "vitest";

import * as api from "./api";
import { en } from "./i18n";
import { flashZipRefusal } from "./secretsRefusal";

const t = (key: keyof typeof en) => en[key];

function answer(body: unknown) {
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify(body), { headers: { "content-type": "application/json" } })));
}

describe("flashZipRefusal", () => {
  it("names the password in the words of the recovery kit until one is set", async () => {
    answer({ ok: true, enabled: false, authed: false });
    await api.getAuth();
    expect(flashZipRefusal(t)).toBe(en["secrets.passwordNotSet"]);

    answer({ ok: true, enabled: true });
    await api.setAuthPassword("a long enough password");
    expect(flashZipRefusal(t)).toBeUndefined();
    vi.unstubAllGlobals();
  });
});
