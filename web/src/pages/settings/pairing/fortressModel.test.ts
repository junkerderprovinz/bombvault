import { describe, expect, it } from "vitest";
import type { FortressMember } from "../../../lib/api";
import { en } from "../../../lib/i18n";
import { fortressApps, fortressPermissions, permissionOf } from "./fortressModel";

const t = (key: keyof typeof en) => en[key];

const member = (over: Partial<FortressMember>): FortressMember => ({
  id: "m1",
  app: "towertorch",
  name: "TowerTorch",
  version: "1.4.2",
  device: "Tower",
  deployment: "plugin",
  kind: "instance",
  online: true,
  relay: false,
  ...over,
});

describe("fortressApps", () => {
  it("puts each other app on one card with a row per place it runs, and leaves BombVault out", () => {
    const apps = fortressApps(
      [
        member({}),
        member({ id: "m2", device: "Cellar", online: false, lastSeen: 1000 }),
        member({ id: "m3", app: "bombvault", name: "BombVault", deployment: "container" }),
        member({ id: "m4", app: "shiplog", name: "ShipLog", deployment: "container", kind: "mobile", relay: true }),
      ],
      (unix) => `at ${unix}`,
    );
    expect(apps.map((a) => a.id)).toEqual(["shiplog", "towertorch"]);
    expect(apps[1].instances).toEqual([
      { id: "m1", device: "Tower", deviceKind: "server", form: "plugin", version: "1.4.2", presence: "connected", relay: false, lastSeen: undefined },
      { id: "m2", device: "Cellar", deviceKind: "server", form: "plugin", version: "1.4.2", presence: "away", relay: false, lastSeen: "at 1000" },
    ]);
    expect(apps[0].instances[0]).toMatchObject({ deviceKind: "phone", relay: true });
  });
});

describe("fortressPermissions", () => {
  const members = [member({}), member({ id: "m2", app: "firesquire", name: "FireSquire" })];

  it("words what an app may do at BombVault and what BombVault fetches, naming both apps", () => {
    const perms = fortressPermissions(
      [
        { from: "towertorch", to: "bombvault", action: "backup.request", on: true },
        { from: "bombvault", to: "firesquire", action: "backup.export", on: false },
      ],
      members,
      t,
    );
    expect(perms).toEqual([
      { id: "towertorch bombvault backup.request", from: "towertorch", to: ["bombvault"], on: true, title: en["fortress.action.backupRequest"], detail: "TowerTorch at BombVault" },
      { id: "bombvault firesquire backup.export", from: "bombvault", to: ["firesquire"], on: false, title: en["fortress.action.backupExport"], detail: "BombVault at FireSquire" },
    ]);
  });

  it("shows an action it has no words for by its name", () => {
    const [p] = fortressPermissions([{ from: "bombvault", to: "shiplog", action: "update.run", on: true }], members, t);
    expect(p).toMatchObject({ title: "update.run", detail: "BombVault at shiplog" });
  });

  it("reads a switch back from its id", () => {
    expect(permissionOf("towertorch bombvault backup.request", false)).toEqual({ from: "towertorch", to: "bombvault", action: "backup.request", on: false });
  });
});
