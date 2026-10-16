import { describe, expect, it } from "vitest";
import { en, type TranslationKey } from "./i18n";
import { snapshotFallbackText, snapshotWindowText } from "./snapshotRead";

const t = (key: TranslationKey) => en[key];

describe("snapshotWindowText", () => {
  it("rounds a window up to whole seconds", () => {
    expect(snapshotWindowText(t, 3200)).toBe("Container stopped for 4 s");
  });

  it("never reads a short window as none", () => {
    expect(snapshotWindowText(t, 0)).toBe("Container stopped for 1 s");
  });
});

describe("snapshotFallbackText", () => {
  it("names the path a known reason is about", () => {
    expect(snapshotFallbackText(t, "split-share: /mnt/user/appdata/plex")).toBe(
      "Stopped for the whole backup: /mnt/user/appdata/plex lies on more than one disk or pool",
    );
  });

  it("reads a reason without detail", () => {
    expect(snapshotFallbackText(t, "ssh-missing")).toBe(
      "Stopped for the whole backup: no SSH connection to the host is set up",
    );
  });

  it("names the capability a refused chroot needs", () => {
    expect(snapshotFallbackText(t, "chroot-refused")).toBe(
      "Stopped for the whole backup: BombVault's container needs the SYS_CHROOT capability to enter the snapshot view",
    );
  });

  it("keeps the code of a reason it has no sentence for", () => {
    expect(snapshotFallbackText(t, "canmount-off: cache/appdata")).toBe(
      "Stopped for the whole backup: cache/appdata cannot be read from a snapshot (canmount-off)",
    );
  });
});
