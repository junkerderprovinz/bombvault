import { describe, expect, it } from "vitest";
import { countText, en, type TranslationKey } from "./i18n";
import { domainName, folderStateText, placeErrorText, probeFactText, probeFailureText, providerName } from "./placeText";

const t = (key: TranslationKey, n?: number) => countText(en[key], "en", n);

describe("placeErrorText", () => {
  it("has its own sentence for every place refusal", () => {
    for (const code of [
      "place-name-taken",
      "place-home-domain",
      "place-is-repository",
      "place-folder-blank",
      "place-domain-unavailable",
      "place-address-taken",
      "place-off",
      "place-repo-shared",
      "place-no-append-only",
      "place-nothing-to-test",
      "place-keeps-less",
      "place-append-only-off",
      "place-rest-path-deep",
      "place-owned-field",
      "place-name-missing",
      "place-unasked",
      "place-unknown-provider",
    ]) {
      const text = placeErrorText(t, "en", { ok: false, code, error: "server text" }, "settings.error");
      expect(text, code).not.toBe("server text");
      expect(text, code).not.toBe("");
    }
  });

  it("names what holds a place in use", () => {
    const text = placeErrorText(
      t,
      "en",
      {
        ok: false,
        code: "place-in-use",
        holders: { homeDomains: ["containers", "vms"], defaults: ["files"], items: [{ domain: "files", key: "a" }], directInUse: [] },
      },
      "settings.error"
    );
    expect(text).toBe(
      "This place is still in use: where Containers and VMs are stored, the default for Folders, and 1 item stored there. Change that first."
    );
  });

  it("says a domain still uses its folder when a place in use names no holders", () => {
    const text = placeErrorText(
      t,
      "en",
      { ok: false, code: "place-in-use", error: "the place has no folder for a domain that uses it: vms" },
      "settings.error"
    );
    expect(text).toBe(en["places.error.folderInUse"]);
  });

  it("counts the snapshots at an address that already holds backups", () => {
    const text = placeErrorText(
      t,
      "en",
      { ok: false, code: "place-location-established", snapshots: 9, domains: ["containers"] },
      "settings.error"
    );
    expect(text).toBe(
      "Backups lie at the old address (9 snapshots of Containers), and the new one does not hold the same repository."
    );
  });

  it("leaves out the count when the address held no counted snapshots of a domain", () => {
    for (const res of [
      { snapshots: 0, domains: [] },
      { snapshots: 0, domains: ["containers"] },
      { snapshots: 5, domains: [] },
    ]) {
      const text = placeErrorText(t, "en", { ok: false, code: "place-location-established", ...res }, "settings.error");
      expect(text).toBe(en["places.error.locationEstablishedPlain"]);
    }
  });

  it("says why a probe refused the write, translated where the code is known", () => {
    const denied = placeErrorText(
      t,
      "en",
      { ok: false, code: "place-probe-failed", probe: { ok: false, code: "direct-access-denied", error: "403" } },
      "settings.error"
    );
    expect(denied).toBe(en["places.error.probeFailed"].replace("{reason}", en["placementCode.directAccessDenied"]));
    const plain = probeFailureText(t, "en", { ok: false, code: "place-probe-failed", error: "dial tcp: refused" });
    expect(plain).toBe("The connection test failed: dial tcp: refused");
  });

  it("hands codes it does not know to the placement table, then to the server", () => {
    expect(placeErrorText(t, "en", { ok: false, code: "stale", error: "x" }, "settings.error")).toBe(en["placementCode.stale"]);
    expect(placeErrorText(t, "en", { ok: false, code: "no-such-code", error: "server text" }, "settings.error")).toBe("server text");
  });
});

describe("probe findings", () => {
  it("fills a finding's parameters into its sentence", () => {
    expect(probeFactText(t, { key: "places.probe.b2Bucket", params: { bucket: "bv-eu" } })).toBe(
      "The key is limited to the bucket bv-eu."
    );
  });

  it("leaves out a finding this version does not know", () => {
    expect(probeFactText(t, { key: "places.probe.somethingNew" })).toBeNull();
    expect(probeFactText(t, { key: "constructor" })).toBeNull();
  });

  it("has words for every folder state", () => {
    for (const state of ["empty", "repository", "absent", "error"] as const) {
      expect(folderStateText(t, state)).not.toBe("");
    }
  });
});

describe("names", () => {
  it("names a provider or domain it does not know by its id, even one every object has", () => {
    for (const id of ["dropbox", "constructor", "toString"]) {
      expect(providerName(t, id)).toBe(id);
      expect(domainName(t, id)).toBe(id);
    }
    expect(providerName(t, "b2")).toBe("Backblaze B2");
    expect(domainName(t, "vms")).toBe("VMs");
  });
});
