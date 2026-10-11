// What is new in the running release, as the green dots on the pages show it.
// The next release replaces the list and the release it names.

import type { TranslationKey } from "./i18n";

/** The release the list describes. A browser that last saw an older one gets
 *  the dots. */
export const WHATS_NEW_RELEASE = "10.0.0";

/** A step on the way to a change: the path a link leads to, the name a row
 *  carries in data-new-way, or a tab of the strip inside the element that
 *  carries `within` there. */
export type WayStep = string | { within: string; tab: string };

export interface Change {
  /** The value of data-new on the element the change is about. */
  id: string;
  /** The route of the page that element is on. */
  page: string;
  /** Where the list leads, for a page whose address names one entry. */
  go?: string;
  /** The page's name in the list, outermost first. */
  where: TranslationKey[];
  /** What leads to the page, outermost first. */
  way: WayStep[];
  /** The element is there at desktop width only, so a phone marks neither it
   *  nor the way to it. */
  wide?: boolean;
  text: TranslationKey;
}

const ENTRY = "/backups/:kind/:id";
const LOCATION = "/storage/:id";
const TO_STORAGE: WayStep[] = ["/settings", "/settings/storage", "/storage"];

export const CHANGES: Change[] = [
  { id: "overview-head", page: "/dashboard", where: ["nav.overview"], way: ["/dashboard"], text: "whatsNew.overviewHead" },
  { id: "overview-customize", page: "/dashboard", where: ["nav.overview"], way: ["/dashboard"], wide: true, text: "whatsNew.overviewCustomize" },
  { id: "overview-decline", page: "/dashboard", where: ["nav.overview"], way: ["/dashboard"], text: "whatsNew.overviewDecline" },

  { id: "backups-tiles", page: "/backups", where: ["backups.title"], way: ["/backups"], text: "whatsNew.backupsTiles" },
  { id: "backups-bar", page: "/backups", where: ["backups.title"], way: ["/backups"], text: "whatsNew.backupsBar" },
  { id: "backups-row", page: "/backups", where: ["backups.title"], way: ["/backups"], text: "whatsNew.backupsRow" },
  { id: "backups-actions", page: "/backups", where: ["backups.title"], way: ["/backups"], text: "whatsNew.backupsActions" },

  { id: "entry-page", page: ENTRY, go: "/backups", where: ["backups.title", "whatsNew.page.entry"], way: ["/backups", "entry"], text: "whatsNew.entryPage" },
  { id: "entry-menu", page: ENTRY, go: "/backups", where: ["backups.title", "whatsNew.page.entry"], way: ["/backups", "entry"], text: "whatsNew.entryActions" },
  {
    id: "entry-defaults",
    page: ENTRY,
    go: "/backups",
    where: ["backups.title", "whatsNew.page.entry"],
    way: ["/backups", "entry", { within: "entry-tabs", tab: "settings" }],
    text: "whatsNew.entryDefaults",
  },

  { id: "restore-tiles", page: "/restore", where: ["nav.restore"], way: ["/restore"], text: "whatsNew.restoreTiles" },
  { id: "restore-where", page: "/restore", where: ["nav.restore"], way: ["/restore", "restore-window"], text: "whatsNew.restoreWhere" },
  { id: "restore-emergency", page: "/restore", where: ["nav.restore"], way: ["/restore"], text: "whatsNew.restoreEmergency" },

  { id: "anomalies-tiles", page: "/anomalies", where: ["nav.anomalies"], way: ["/anomalies"], text: "whatsNew.anomaliesTiles" },
  { id: "anomalies-rings", page: "/anomalies", where: ["nav.anomalies"], way: ["/anomalies"], text: "whatsNew.anomaliesRings" },

  { id: "storage-list", page: "/storage", where: ["storage.title"], way: TO_STORAGE, text: "whatsNew.storageList" },
  { id: "storage-flow", page: "/storage", where: ["storage.title"], way: TO_STORAGE, text: "whatsNew.storageFlow" },
  {
    id: "location-keep",
    page: LOCATION,
    go: "/storage",
    where: ["storage.title", "whatsNew.page.location"],
    way: [...TO_STORAGE, "location"],
    text: "whatsNew.locationKeep",
  },
  {
    id: "location-move",
    page: LOCATION,
    go: "/storage",
    where: ["storage.title", "whatsNew.page.location"],
    way: [...TO_STORAGE, "location"],
    text: "whatsNew.locationMove",
  },

  {
    id: "instances-pair",
    page: "/instances",
    where: ["instances.title"],
    way: ["/settings", "/settings/instances", "/instances"],
    text: "whatsNew.instancesPair",
  },

  { id: "settings-tiles", page: "/settings/:page?", go: "/settings", where: ["nav.settings"], way: ["/settings"], text: "whatsNew.settingsTiles" },
  {
    id: "settings-pin",
    page: "/settings/storage",
    where: ["nav.settings", "storage.title"],
    way: ["/settings", "/settings/storage"],
    text: "whatsNew.settingsPin",
  },
  {
    id: "settings-week",
    page: "/settings/schedules",
    where: ["nav.settings", "settings.schedule"],
    way: ["/settings", "/settings/schedules"],
    text: "whatsNew.settingsWeek",
  },
  {
    id: "pairing-steps",
    page: "/settings/pairing",
    where: ["nav.settings", "pairing.title"],
    way: ["/settings", "/settings/pairing"],
    text: "whatsNew.pairingSteps",
  },
  {
    id: "settings-version",
    page: "/settings/info",
    where: ["nav.settings", "activityLog.glyphInfo"],
    way: ["/settings", "/settings/info"],
    text: "whatsNew.settingsVersion",
  },
];
