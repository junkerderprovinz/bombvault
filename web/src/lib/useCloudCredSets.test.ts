import { describe, expect, it } from "vitest";
import { de, en, type TranslationKey } from "./i18n";
import { credSetLabel } from "./useCloudCredSets";

const set = { id: "s1", name: "NAS direct", s3KeyId: "", s3Region: "", restUser: "bv", s3StorageClass: "", s3SecretSet: false, restPasswordSet: true };

describe("credSetLabel", () => {
  it("names a kept set after its direct repository in the reader's language", () => {
    const kept = { ...set, keptFor: "r1" };
    expect(credSetLabel((k: TranslationKey) => en[k], kept)).toBe("NAS direct (kept credentials)");
    expect(credSetLabel((k: TranslationKey) => de[k], kept)).toBe("NAS direct (behaltene Zugangsdaten)");
  });

  it("leaves a set someone named under that name", () => {
    expect(credSetLabel((k: TranslationKey) => en[k], set)).toBe("NAS direct");
  });
});
