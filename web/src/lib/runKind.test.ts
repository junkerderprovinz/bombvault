import { describe, expect, it } from "vitest";
import { en } from "./i18n";
import { runKindLabel } from "./runKind";

const t = ((key: keyof typeof en) => en[key]) as Parameters<typeof runKindLabel>[0];

describe("runKindLabel", () => {
  it("names an Appdata.Backup import instead of printing its raw kind", () => {
    expect(runKindLabel(t, "import")).toBe(en["run.kindImport"]);
  });

  it("names every entry the key deposit writes", () => {
    const kinds = ["keydeposit", "keyrenew", "keywithdraw", "keycode", "keykeep", "keygone", "keydrop", "keyask", "keyallow", "keyrefuse", "keyrelease"];
    const labels = kinds.map((kind) => runKindLabel(t, kind));
    expect(labels).toEqual([
      en["run.kindKeyDeposit"],
      en["run.kindKeyRenew"],
      en["run.kindKeyWithdraw"],
      en["run.kindKeyCode"],
      en["run.kindKeyKeep"],
      en["run.kindKeyGone"],
      en["run.kindKeyDrop"],
      en["run.kindKeyAsk"],
      en["run.kindKeyAllow"],
      en["run.kindKeyRefuse"],
      en["run.kindKeyRelease"],
    ]);
    expect(new Set(labels).size).toBe(kinds.length);
  });

  it.each([
    ["keyread", "run.kindKeyRead"],
    ["keyadopt", "run.kindKeyAdopt"],
    ["keysignins", "run.kindKeySignIns"],
    ["keysettings", "run.kindKeySettings"],
    ["keyundo", "run.kindKeyUndo"],
  ] as const)("names what happened to the backup key for %s", (kind, key) => {
    expect(runKindLabel(t, kind)).toBe(en[key]);
  });

  it("keeps an unknown kind as it came", () => {
    expect(runKindLabel(t, "someday")).toBe("someday");
  });
});
