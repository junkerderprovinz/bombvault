import { describe, expect, it } from "vitest";
import { en } from "../../lib/i18n";
import { buildItems, fold, search } from "./SettingsSearch";
import { SETTINGS_PAGES } from "./settingsPages";

const item = (name: string, prose = "", tier = 2) => ({ foldedName: fold(name), foldedProse: fold(prose), tier, name });

describe("fold", () => {
  it("drops case and accents, so a plain keyboard finds umlauts", () => {
    expect(fold("Zeitpläne")).toBe("zeitplane");
    expect(fold("Größe")).toBe("grosse");
  });
});

describe("search", () => {
  const items = [
    item("Backup CPU"),
    item("Snapshot retention", "How long backups are kept"),
    item("Retention", "", 0),
    item("Off-site retention"),
  ];

  it("finds nothing for an empty query", () => {
    expect(search(items, "  ")).toEqual([]);
  });

  it("wants every word of the query", () => {
    expect(search(items, "snapshot keep").map((h) => h.item.name)).toEqual([]);
    expect(search(items, "snapshot kept").map((h) => h.item.name)).toEqual(["Snapshot retention"]);
  });

  it("ranks a name that starts with the word above one that only contains it", () => {
    expect(search(items, "reten")[0].item.name).toBe("Retention");
  });

  it("ranks every name match above a match in an explanation", () => {
    const prose = search([item("Snapshot retention", "backups are kept"), item("Kept copies")], "kept");
    expect(prose.map((h) => [h.item.name, h.inHint])).toEqual([
      ["Kept copies", false],
      ["Snapshot retention", true],
    ]);
  });
});

describe("the settings index in English", () => {
  const items = buildItems((k) => en[k], SETTINGS_PAGES);
  const where = (query: string) =>
    search(items, query)
      .filter((h) => !h.inHint)
      .map((h) => `${h.item.page}: ${h.item.cardName ?? ""}`);
  const copyCards = (hits: string[]) => hits.filter((h) => h.startsWith("storage: Off-site copy"));

  it("leads the yearly rule to both retention cards and to each off-site destination", () => {
    const hits = where("keep yearly");
    expect(hits).toContain(`storage: ${en["settings.retentionLocalTitle"]}`);
    expect(hits).toContain(`storage: ${en["settings.retentionOffsiteTitle"]}`);
    expect(copyCards(hits)).toHaveLength(6);
    expect(hits.every((h) => h.startsWith("storage: "))).toBe(true);
  });

  it("leads compression to the named repositories and each off-site destination", () => {
    const hits = where("compression");
    expect(hits).toContain(`storage: ${en["repos.title"]}`);
    expect(copyCards(hits)).toHaveLength(6);
  });

  it("leads each source's own rules to their retention card, by the phrase and by the source's name", () => {
    const local = `storage: ${en["settings.retentionLocalTitle"]}`;
    const offsite = `storage: ${en["settings.retentionOffsiteTitle"]}`;
    expect(where("own keep rules")).toContain(local);
    expect(where("own off-site keep rules")).toContain(offsite);
    expect(where("own off-site keep rules")).not.toContain(local);
    expect(where(en["nav.vms"])).toEqual(expect.arrayContaining([local, offsite]));
    expect(where("additional off-site targets keep")).toEqual([offsite]);
  });

  it("finds the preview on its own card", () => {
    expect(where(en["retentionPreview.show"])).toEqual([`storage: ${en["restore.preview"]}`]);
  });

  it("finds a row by the words behind its (i)", () => {
    const hit = search(items, "calendar years").find((h) => h.item.page === "storage");
    expect(hit?.inHint).toBe(true);
    expect(hit?.item.name).toBe(en["settings.retentionYearly"]);
  });

  it("leads the idle limits to the Schedule page", () => {
    const top = search(items, "idle before backup")[0];
    expect(top.item.page).toBe("schedules");
    expect(top.item.name).toBe(en["idle.title"]);
    expect(where("traffic below")).toEqual([`schedules: ${en["idle.title"]}`]);
    expect(where("for at least")).toContain(`schedules: ${en["idle.title"]}`);
  });

  it("leads Streaming first and its limits to the Storage locations page", () => {
    const top = search(items, "streaming first")[0];
    expect(top.item.page).toBe("storage");
    expect(top.item.name).toBe(en["streaming.title"]);
    expect(where("upload limit while streaming")).toEqual([`storage: ${en["streaming.title"]}`]);
    expect(where("media servers")).toContain(`storage: ${en["streaming.title"]}`);
    expect(where("back to normal")).toContain(`storage: ${en["streaming.title"]}`);
  });

  it("finds the idle and streaming rows by the words behind their (i)", () => {
    const cpu = search(items, "docker stats").find((h) => h.item.page === "schedules");
    expect(cpu?.inHint).toBe(true);
    expect(cpu?.item.name).toBe(en["idle.cpu"]);
    const servers = search(items, "image name").filter((h) => h.item.cardName === en["streaming.title"]);
    expect(servers.map((h) => h.item.name)).toContain(en["streaming.servers"]);
    expect(servers.every((h) => h.inHint)).toBe(true);
  });

  it("leads the API tokens, Home Assistant and the mDNS switch to the Integrations page", () => {
    expect(search(items, "api tokens")[0].item.page).toBe("integrations");
    expect(where("token name")).toEqual([`integrations: ${en["api.title"]}`]);
    expect(where("mqtt broker")).toContain(`integrations: ${en["ha.title"]}`);
    expect(where("buttons start backups")).toEqual([`integrations: ${en["ha.title"]}`]);
    expect(where("topic prefix")).toEqual([`integrations: ${en["ha.title"]}`]);
    expect(where("announce on the local network")).toEqual([`integrations: ${en["mdns.title"]}`]);
  });

  it("finds the API, Home Assistant and mDNS rows by the words behind their (i)", () => {
    const header = search(items, "x-api-key").find((h) => h.item.cardName === en["api.title"]);
    expect(header?.inHint).toBe(true);
    expect(header?.item.name).toBe(en["api.addressLabel"]);
    const port = search(items, "8883").find((h) => h.item.page === "integrations");
    expect(port?.item.name).toBe(en["ha.tls"]);
    const local = search(items, "bombvault.local").filter((h) => h.item.page === "integrations");
    expect(local.map((h) => h.item.name)).toContain(en["mdns.title"]);
  });

  it("leads what moved to the tile that holds it", () => {
    expect(where(en["settings.restoreFolder"])).toEqual([`restore: ${en["tile.restore.folders"]}`]);
    expect(where(en["recovery.title"])).toContain(`restore: ${en["tile.restore.emergency"]}`);
    expect(where(en["settings.registryHost"])).toEqual([`restore: ${en["settings.registriesTitle"]}`]);
    expect(where(en["settings.cacheLimitLabel"])).toEqual([`system: ${en["tile.system.performance"]}`]);
    expect(where(en["export.encrypt.enable"])).toEqual([`system: ${en["settingsIO.title"]}`]);
    expect(where(en["settings.restartHealthWait"])).toEqual([`schedules: ${en["settings.restartHealthTitle"]}`]);
    expect(where(en["settings.pruneImageAfterUpdate"])).toEqual([`schedules: ${en["settings.imageMaintenanceTitle"]}`]);
    expect(where(en["pull.title"])).toContain(`instances: ${en["tile.instances.modules"]}`);
    expect(where(en["tile.info.bugReport"])).toEqual([`info: ${en["tile.info.help"]}`]);
    expect(search(items, en["tile.crypt.title"])[0].item.page).toBe("security");
  });

  it("finds a page by the name its tile carried before, and says where that went", () => {
    const former = (query: string) => search(items, query).find((h) => h.item.former);
    expect(former("off-site")?.item).toMatchObject({ page: "storage", to: "storage#offsite", name: en["settings.tab.offsite"] });
    expect(former("retention")?.item).toMatchObject({ page: "storage", to: "storage#retention" });
    expect(former("cloud access")?.item).toMatchObject({ page: "storage", to: "storage#cloud" });
    expect(former("containers")?.item).toMatchObject({ page: "schedules", to: "schedules#order" });
    expect(former("integrity")?.item.page).toBe("integrity");
    expect(former("schedules")?.item.page).toBe("schedules");
    expect(search(items, "off-site")[0].item.former).toBe(true);
  });
});
