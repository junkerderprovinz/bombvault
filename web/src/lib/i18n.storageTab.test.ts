import { describe, expect, it } from "vitest";
import { de, en } from "./i18n";

// Texts that send the reader into Settings name the Storage tab, never a tab or
// card that the storage places replaced.
const POINTERS = ["config.pathMoved", "config.offsiteMoved", "update.afterBackupOrphans", "settings.tamperScheduleInactive"] as const;
const GONE = [/Paths (&|and) storage/i, /Settings, Off-site/, /off-site settings/i, /Pfade (&|und) Speicher/i, /Einstellungen, Off-site/, /Off-site-Einstellungen/];

describe("texts that point into Settings", () => {
  it("call the tab Storage", () => {
    expect(en["settings.tab.storage"]).toBe("Storage");
    expect(de["settings.tab.storage"]).toBe("Speicher");
  });

  it("send the reader to the Storage tab in English and German", () => {
    for (const key of POINTERS) {
      expect(en[key], key).toMatch(/Settings[,\s→]+Storage/);
      expect(de[key], key).toMatch(/Einstellungen[,\s→]+Speicher/);
      for (const old of GONE) {
        expect(en[key], key).not.toMatch(old);
        expect(de[key], key).not.toMatch(old);
      }
    }
  });
});
