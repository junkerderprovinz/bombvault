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
  opens: false,
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
      { id: "m1", device: "Tower", deviceKind: "server", form: "plugin", version: "1.4.2", presence: "connected", relay: false, open: {}, print: "m1" },
      { id: "m2", device: "Cellar", deviceKind: "server", form: "plugin", version: "1.4.2", presence: "away", relay: false, lastSeen: "at 1000", open: {}, print: "m2" },
    ]);
    expect(apps[0].instances[0]).toMatchObject({ deviceKind: "phone", relay: true });
    expect(apps[0].instances[0].open).toBeUndefined();
  });

  it("shows a phone app as an Android app and its version with a single v", () => {
    const [app] = fortressApps([member({ app: "arrowloop", name: "ArrowLoop", deployment: "phone", version: "v2.6.9-12-gabc" })], String);
    expect(app.instances[0]).toMatchObject({ form: "android", version: "2.6.9-12-gabc" });
  });

  it("opens nothing on a paired phone", () => {
    const [app] = fortressApps([member({})], String, false);
    expect(app.instances[0].open).toBeUndefined();
  });

  it("marks two members that claim one app on one server, and names them by the end of their ids", () => {
    const apps = fortressApps(
      [
        member({ id: "aaaaaaaa11111111", host: "Tower" }),
        member({ id: "bbbbbbbb22222222", device: "tower" }),
        member({ id: "cccccccc33333333", device: "Cellar" }),
      ],
      String,
    );
    expect(apps[0].instances.map((i) => [i.print, i.doubt])).toEqual([
      ["11111111", true],
      ["22222222", true],
      ["33333333", undefined],
    ]);
  });
});

describe("fortressApps logos", () => {
  it("gives each app the logo the first of its instances published", () => {
    const [app] = fortressApps(
      [member({}), member({ id: "m2", device: "Cellar", logo: "data:image/svg+xml;base64,PHN2Zy8+" }), member({ id: "m3", logo: "data:image/svg+xml;base64,b3RoZXI=" })],
      String,
    );
    expect(app.logo).toBe("data:image/svg+xml;base64,PHN2Zy8+");
  });

  it("leaves an app that published none without one", () => {
    expect(fortressApps([member({})], String)[0].logo).toBeUndefined();
  });
});

describe("fortressPermissions", () => {
  const members = [member({}), member({ id: "m2", app: "firesquire", name: "FireSquire" })];

  it("names the app acted on first, then where it happens", () => {
    const perms = fortressPermissions(
      [
        { from: "towertorch", to: "bombvault", action: "backup.request", on: true },
        { from: "bombvault", to: "firesquire", action: "backup.export", on: false },
      ],
      members,
      t,
    );
    expect(perms).toEqual([
      { id: "towertorch bombvault backup.request", from: "towertorch", to: ["bombvault"], on: true, title: "Ask BombVault for a container backup from TowerTorch" },
      { id: "bombvault firesquire backup.export", from: "bombvault", to: ["firesquire"], on: false, title: "Fetch FireSquire's own backup into BombVault" },
    ]);
  });

  it("says who opens whose pages, and who learns which containers another app updates", () => {
    const perms = fortressPermissions(
      [
        { from: "knightloader", to: "bombvault", action: "ui.open", on: true },
        { from: "bombvault", to: "shiplog", action: "updates.managed", on: true },
      ],
      [member({ app: "knightloader", name: "KnightLoader" }), member({ id: "m3", app: "shiplog", name: "ShipLog" })],
      t,
    );
    expect(perms.map((p) => p.title)).toEqual(["Open BombVault in KnightLoader", "Ask ShipLog in BombVault which containers it updates itself"]);
  });

  it("shows an action it has no words for by its name", () => {
    const [p] = fortressPermissions([{ from: "bombvault", to: "shiplog", action: "update.run", on: true }], members, t);
    expect(p).toMatchObject({ title: "update.run", detail: "BombVault at shiplog" });
  });

  it("leaves out what an app may do at itself while it runs in one place", () => {
    const own = { from: "bombvault", to: "bombvault", action: "ui.open", on: true };
    const kl = { from: "knightloader", to: "knightloader", action: "ui.open", on: true };
    const one = [member({ app: "knightloader", name: "KnightLoader" })];
    expect(fortressPermissions([own, kl], one, t)).toEqual([]);
    const two = [...one, member({ id: "m4", app: "bombvault", name: "BombVault" }), member({ id: "m5", app: "knightloader", name: "KnightLoader" })];
    expect(fortressPermissions([own, kl], two, t).map((p) => p.id)).toEqual(["bombvault bombvault ui.open", "knightloader knightloader ui.open"]);
  });

  it("reads a switch back from its id", () => {
    expect(permissionOf("towertorch bombvault backup.request", false)).toEqual({ from: "towertorch", to: "bombvault", action: "backup.request", on: false });
  });
});
