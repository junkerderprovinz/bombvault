import { describe, expect, it } from "vitest";
import type { PlacementDomain } from "./api";
import { en, type TranslationKey } from "./i18n";
import { placementErrorText, pushSaveWarnings, saveWarningText } from "./placementCodes";

const t = (key: TranslationKey) => en[key];

describe("placementErrorText", () => {
  it("has a sentence for every refusal code of the placement routes", () => {
    const codes = [
      "placement-unreadable", "invalid-placement", "copies-not-allowed", "unknown-target", "stack-rule",
      "copy-rule-taken", "domain-busy", "has-backups", "stale", "repo-invalid", "default-repo-missing",
      "nested-location", "foreign-domain", "mirrored-field", "companion-taken",
    ];
    for (const code of codes) {
      const text = placementErrorText(t, "en", { ok: false, error: "server text", code }, "settings.error");
      expect(text, code).not.toBe("server text");
      expect(text, code).not.toBe("");
    }
  });

  it("names the defaults that hold a repository", () => {
    const refusal = (defaultDomains: PlacementDomain[]) => ({ ok: false, error: "x", code: "repo-in-use", defaultDomains });
    expect(placementErrorText(t, "en", refusal(["containers", "vms"]), "settings.error")).toBe(
      "The default for Containers and VMs points at this repository. Change the default first."
    );
    expect(placementErrorText(t, "en", refusal([]), "settings.error")).toBe(en["repos.deleteBlocked"]);
  });

  it("counts the items on a target's direct repository before its defaults", () => {
    const refusal = (items: number, defaultDomains: PlacementDomain[]) => ({
      ok: false, error: "x", code: "target-in-use", use: { directRepoId: "d", items, defaultDomains },
    });
    expect(placementErrorText(t, "en", refusal(3, ["files"]), "settings.error")).toBe(
      "Items still back up to the direct repository of this target: 3. Point them somewhere else first."
    );
    expect(placementErrorText(t, "en", refusal(0, ["files"]), "settings.error")).toBe(
      "The default for Folders points at the direct repository of this target. Change the default first."
    );
  });

  it("names the target a direct repository goes with", () => {
    const refusal = { ok: false, error: "x", code: "direct-repo", target: { id: "t-b2", name: "B2" } };
    expect(placementErrorText(t, "en", refusal, "settings.error")).toBe(
      "This repository goes with B2. Remove that target instead."
    );
  });

  it("falls back to the server's text, then to the given key", () => {
    expect(placementErrorText(t, "en", { ok: false, error: "server text", code: "no-such-code" }, "settings.error")).toBe("server text");
    expect(placementErrorText(t, "en", { ok: false }, "settings.error")).toBe(en["settings.error"]);
  });
});

describe("save warnings", () => {
  it("fill in the target and the count and go out as warnings", () => {
    const w = { code: "direct-retention-lowered" as const, targetId: "t", targetName: "B2", items: 3 };
    expect(saveWarningText(t, w)).toBe("B2 direct now keeps less. Items whose only copy is there: 3.");
    const pushed: [string, string | undefined][] = [];
    pushSaveWarnings((m, s) => pushed.push([m, s]), t, [w]);
    pushSaveWarnings((m, s) => pushed.push([m, s]), t, undefined);
    expect(pushed).toEqual([["B2 direct now keeps less. Items whose only copy is there: 3.", "warn"]]);
  });
});
