import { describe, expect, it } from "vitest";
import { en, locales, type TranslationKey } from "../lib/i18n";
import { blockModeLine } from "./VMBlockToggle";

const t = (k: TranslationKey) => en[k];

describe("the changed-block status line", () => {
  it("says nothing before the first run", () => {
    expect(blockModeLine(t, "", "")).toBeNull();
    expect(blockModeLine(t, undefined, undefined)).toBeNull();
  });

  it("names the reason for a whole-disk read", () => {
    expect(blockModeLine(t, "full", "no_checkpoint")).toBe("Last run: whole disk read (checkpoint missing)");
    expect(blockModeLine(t, "classic", "disk_format")).toBe("Last run: usual backup (not all disks are qcow2)");
  });

  it("leaves out a reason it does not know", () => {
    expect(blockModeLine(t, "full", "cosmic_ray")).toBe("Last run: whole disk read");
  });

  it("has a German text for every reason the server sends", () => {
    const reasons = ["first", "newer_backup", "no_checkpoint", "disks_changed", "checkpoint_broken", "chain_broken", "vm_off", "disk_format", "zvol", "no_ssh", "unsupported", "no_space", "job_failed"];
    for (const r of reasons) {
      const key = `vm.blocks.reason.${r}` as TranslationKey;
      expect(en[key], key).toBeTruthy();
      expect(locales.de[key], key).toBeTruthy();
    }
  });
});
