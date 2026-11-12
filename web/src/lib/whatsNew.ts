// What each release brought, as the green dots on the pages show it. A browser
// that last saw an older release gets the dots of everything that came after.

import type { TranslationKey } from "./i18n";

/** A step on the way to a change: the path a link leads to, the name a row
 *  carries in data-new-way, or a tab of the strip inside the element that
 *  carries `within` there. */
export type WayStep = string | { within: string; tab: string };

/** A change on a page, marked on its element and on the way there. */
interface Placed {
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
  /** The element is there in the Android app only, so a browser marks neither
   *  it nor the way to it. */
  app?: boolean;
  text: TranslationKey;
}

/** A change no page shows, such as how a backup behaves. Its dot stands in the
 *  list only. */
interface Unplaced {
  id: string;
  page?: undefined;
  text: TranslationKey;
}

export interface Release {
  version: string;
  changes: (Placed | Unplaced)[];
}

export type Change = (Placed | Unplaced) & { since: string };
export type PlacedChange = Placed & { since: string };

export function placed(change: Change): change is PlacedChange {
  return change.page !== undefined;
}

const ENTRY = "/backups/:kind/:id";
const LOCATION = "/storage/:id";
const TO_STORAGE: WayStep[] = ["/settings", "/settings/storage", "/storage"];
const TO_INSTANCES: WayStep[] = ["/settings", "/settings/instances", "/instances"];
// Only a ZFS entry with a replica shows the Replica card, on its Settings tab.
const TO_REPLICA_CARD: WayStep[] = ["/backups", "zfs-replica", { within: "entry-tabs", tab: "settings" }];
// A container's own settings are on its Settings tab.
const TO_CONTAINER_SETTINGS: WayStep[] = ["/backups", "container", { within: "entry-tabs", tab: "settings" }];

/** Every release that brought something new, newest first. */
export const RELEASES: Release[] = [
  {
    version: "10.6.0",
    changes: [
      // The fortress shows on the Pairing page once it has loaded. The apps'
      // own backups, giving one back and the asks of the other apps show only
      // where an app takes part, so they stand in the list.
      {
        id: "fortress-join",
        page: "/settings/pairing",
        where: ["nav.settings", "pairing.title"],
        way: ["/settings", "/settings/pairing"],
        text: "whatsNew.fortress",
      },
      {
        id: "pairing-card",
        page: "/settings/pairing",
        where: ["nav.settings", "pairing.title"],
        way: ["/settings", "/settings/pairing"],
        text: "whatsNew.pairingCard",
      },
      // Remove shows only on the card of another app, so its dot stands in
      // the list.
      { id: "fortress-remove", text: "whatsNew.fortressRemove" },
      { id: "app-backups", text: "whatsNew.appBackups" },
      { id: "app-give-back", text: "whatsNew.giveBack" },
      { id: "fortress-asks", text: "whatsNew.fortressAsks" },
      { id: "shared-repo", page: "/instances", where: ["instances.title"], way: TO_INSTANCES, text: "whatsNew.sharedRepo" },
      { id: "shared-repo-member", page: "/instances", where: ["instances.title"], way: TO_INSTANCES, text: "whatsNew.sharedRepoMember" },
      // The section sits in the receiving server's window, which the button
      // on this server's own card opens.
      {
        id: "shared-upkeep",
        page: "/instances",
        where: ["instances.title"],
        way: [...TO_INSTANCES, "receiver-server"],
        text: "whatsNew.sharedUpkeep",
      },
      { id: "restore-other-servers", page: "/restore", where: ["nav.restore"], way: ["/restore"], text: "whatsNew.restoreOtherServers" },
      { id: "shared-member-space", page: "/instances", where: ["instances.title"], way: TO_INSTANCES, text: "whatsNew.sharedMemberSpace" },
      { id: "shared-repo-own", page: "/instances", where: ["instances.title"], way: TO_INSTANCES, text: "whatsNew.sharedOwnCopies" },
      // A copy waits, a join runs out of time and a server leaves only now
      // and then, so what shows them has no element that is always there.
      { id: "shared-copy-deferred", text: "whatsNew.sharedCopyDeferred" },
      { id: "shared-join-expiry", text: "whatsNew.sharedJoinExpiry" },
      { id: "shared-repo-former", text: "whatsNew.sharedFormer" },
      { id: "restore-file-search", page: "/restore", where: ["nav.restore"], way: ["/restore"], text: "whatsNew.fileSearch" },
      { id: "restore-tool-import", page: "/restore", where: ["nav.restore"], way: ["/restore"], text: "whatsNew.toolImport" },
      // A new folder set meets the junk lines first, in the form that Add
      // opens, and an existing set only behind a button on its Settings tab.
      { id: "files-junk", page: "/backups", where: ["backups.title"], way: ["/backups", "add-entry", "add-folder"], text: "whatsNew.systemJunk" },
      {
        id: "location-connections",
        page: LOCATION,
        go: "/storage",
        where: ["storage.title", "whatsNew.page.location"],
        way: [...TO_STORAGE, "network"],
        text: "whatsNew.connections",
      },
      { id: "delete-everywhere", text: "whatsNew.deleteEverywhere" },
      {
        id: "look-oled",
        page: "/settings/look",
        where: ["nav.settings", "settings.tab.look"],
        way: ["/settings", "/settings/look"],
        text: "whatsNew.oled",
      },
      // The wand shows only where the device has a colour, which most
      // browsers do not, so its dot stands in the list.
      { id: "accent-system", text: "whatsNew.systemAccent" },
      // Apply now shows only while the rule would remove backups, and the card
      // already carries the dot of location-keep, so this one stands on the
      // picture of the rule.
      {
        id: "keep-now",
        page: LOCATION,
        go: "/storage",
        where: ["storage.title", "whatsNew.page.location"],
        way: [...TO_STORAGE, "location"],
        text: "whatsNew.keepNow",
      },
      { id: "delete-no-password", text: "whatsNew.deleteNoPassword" },
    ],
  },
  {
    version: "10.5.0",
    changes: [
      {
        id: "folder-keep",
        page: LOCATION,
        go: "/storage",
        where: ["storage.title", "whatsNew.page.location"],
        way: [...TO_STORAGE, "folder"],
        text: "whatsNew.folderKeep",
      },
      { id: "restore-fetched", page: "/restore", where: ["nav.restore"], way: ["/restore"], text: "whatsNew.restoreFetched" },
      // The question shows only while backups wait for it, and the kit is a
      // download, so neither has an element to carry a dot.
      { id: "copy-decision", text: "whatsNew.copyDecision" },
      { id: "kit-sign-ins", text: "whatsNew.kitSignIns" },
      { id: "restore-stacks", page: "/restore", where: ["nav.restore"], way: ["/restore"], text: "whatsNew.restoreStacks" },
      {
        id: "entry-own-plan",
        page: ENTRY,
        go: "/backups",
        where: ["backups.title", "whatsNew.page.entry"],
        way: TO_CONTAINER_SETTINGS,
        text: "whatsNew.entryOwnPlan",
      },
      {
        id: "location-risk-accepted",
        page: LOCATION,
        go: "/storage",
        where: ["storage.title", "whatsNew.page.location"],
        way: [...TO_STORAGE, "location"],
        text: "whatsNew.deleteRisk",
      },
    ],
  },
  {
    version: "10.4.0",
    changes: [
      { id: "overview-move-mode", page: "/dashboard", where: ["nav.overview"], way: ["/dashboard"], wide: true, text: "whatsNew.overviewMoveMode" },
      {
        id: "look-label-glyphs",
        page: "/settings/look",
        where: ["nav.settings", "settings.tab.look"],
        way: ["/settings", "/settings/look"],
        text: "whatsNew.labelGlyphs",
      },
      {
        id: "apps-fdroid",
        page: "/settings/apps",
        where: ["nav.settings", "settings.tab.apps"],
        way: ["/settings", "/settings/apps"],
        text: "whatsNew.fdroid",
      },
      {
        id: "location-rename",
        page: LOCATION,
        go: "/storage",
        where: ["storage.title", "whatsNew.page.location"],
        way: [...TO_STORAGE, "location"],
        text: "whatsNew.locationRename",
      },
      {
        id: "notify-gotify",
        page: "/settings/notifications",
        where: ["nav.settings", "settings.tab.notifications"],
        way: ["/settings", "/settings/notifications"],
        text: "whatsNew.gotify",
      },
      {
        id: "deferral-retry",
        page: ENTRY,
        go: "/backups",
        where: ["backups.title", "whatsNew.page.entry"],
        way: TO_CONTAINER_SETTINGS,
        text: "whatsNew.deferral",
      },
      {
        id: "deferral-preset",
        page: ENTRY,
        go: "/backups",
        where: ["backups.title", "whatsNew.page.entry"],
        way: TO_CONTAINER_SETTINGS,
        text: "whatsNew.activityPresets",
      },
      {
        id: "deferral-net",
        page: ENTRY,
        go: "/backups",
        where: ["backups.title", "whatsNew.page.entry"],
        way: TO_CONTAINER_SETTINGS,
        text: "whatsNew.netGate",
      },
      {
        id: "zfs-replica-source-keep",
        page: ENTRY,
        go: "/backups",
        where: ["backups.title", "whatsNew.page.entry"],
        way: TO_REPLICA_CARD,
        text: "whatsNew.replicaSourceKeep",
      },
    ],
  },
  {
    version: "10.3.0",
    changes: [
      {
        id: "storage-add-bucket",
        page: "/storage",
        where: ["storage.title"],
        way: [...TO_STORAGE, "add-location"],
        text: "whatsNew.bucketField",
      },
      {
        id: "wizard-setup-choice",
        page: "/storage",
        where: ["storage.title"],
        way: [...TO_STORAGE, "add-location"],
        text: "whatsNew.wizardSetupChoice",
      },
      { id: "dest-host-key", page: "/storage", where: ["storage.title"], way: [...TO_STORAGE, "add-location"], text: "whatsNew.serverHostKey" },
      { id: "dest-switches", page: "/storage", where: ["storage.title"], way: [...TO_STORAGE, "add-location"], text: "whatsNew.serverSwitches" },
      { id: "dest-smb-share", page: "/storage", where: ["storage.title"], way: [...TO_STORAGE, "add-location"], text: "whatsNew.smbShare" },
      {
        id: "storagebox-key-install",
        page: "/storage",
        where: ["storage.title"],
        way: [...TO_STORAGE, "add-location"],
        text: "whatsNew.keyInstall",
      },
      // Both appear only on a place that refused its folder or lost its
      // repository, so a dot on them would hardly ever show.
      { id: "replica-take-over", text: "whatsNew.replicaTakeOver" },
      { id: "location-start-over", text: "whatsNew.startOver" },
      {
        id: "backup-memory-limit",
        page: "/settings/system",
        where: ["nav.settings", "settings.tab.system"],
        way: ["/settings", "/settings/system"],
        text: "whatsNew.memoryLimit",
      },
      { id: "instances-card-logo", page: "/instances", where: ["instances.title"], way: TO_INSTANCES, text: "whatsNew.instancesCardLogo" },
      { id: "instances-net-logos", page: "/instances", where: ["instances.title"], way: TO_INSTANCES, text: "whatsNew.instancesNetLogos" },
      { id: "instances-card-size", page: "/instances", where: ["instances.title"], way: TO_INSTANCES, text: "whatsNew.instancesCardSize" },
      { id: "instances-full-names", page: "/instances", where: ["instances.title"], way: TO_INSTANCES, text: "whatsNew.instancesFullNames" },
      { id: "instances-net-labels", page: "/instances", where: ["instances.title"], way: TO_INSTANCES, text: "whatsNew.instancesNetLabels" },
      { id: "instances-inactive", page: "/instances", where: ["instances.title"], way: TO_INSTANCES, text: "whatsNew.instancesInactive" },
      { id: "instances-checking", page: "/instances", where: ["instances.title"], way: TO_INSTANCES, text: "whatsNew.instancesChecking" },
      {
        id: "pairing-inactive",
        page: "/settings/pairing",
        where: ["nav.settings", "pairing.title"],
        way: ["/settings", "/settings/pairing"],
        text: "whatsNew.pairingInactive",
      },
    ],
  },
  {
    version: "10.2.0",
    changes: [
      {
        id: "settings-sign-ins",
        page: "/settings/storage",
        where: ["nav.settings", "storage.title"],
        way: ["/settings", "/settings/storage"],
        text: "whatsNew.signIns",
      },
      {
        id: "location-sign-in",
        page: LOCATION,
        go: "/storage",
        where: ["storage.title", "whatsNew.page.location"],
        way: [...TO_STORAGE, "location"],
        text: "whatsNew.locationSignIn",
      },
      {
        id: "own-folder-move",
        page: LOCATION,
        go: "/storage",
        where: ["storage.title", "whatsNew.page.location"],
        way: [...TO_STORAGE, "location"],
        text: "whatsNew.ownFolderMove",
      },
      {
        id: "own-folders-card",
        page: LOCATION,
        go: "/storage",
        where: ["storage.title", "whatsNew.page.location"],
        way: [...TO_STORAGE, "location"],
        text: "whatsNew.ownFoldersCard",
      },
      {
        id: "location-single-action",
        page: LOCATION,
        go: "/storage",
        where: ["storage.title", "whatsNew.page.location"],
        way: [...TO_STORAGE, "location"],
        text: "whatsNew.singleAction",
      },
      { id: "wizard-sign-in", text: "whatsNew.wizardSignIn" },
      {
        id: "sign-in-make-default",
        page: "/settings/storage",
        where: ["nav.settings", "storage.title"],
        way: ["/settings", "/settings/storage"],
        text: "whatsNew.signInMakeDefault",
      },
      {
        id: "sign-in-edit",
        page: "/settings/storage",
        where: ["nav.settings", "storage.title"],
        way: ["/settings", "/settings/storage"],
        text: "whatsNew.signInEdit",
      },
      { id: "got-it-check", text: "whatsNew.gotItCheck" },
      {
        id: "settings-storage-list",
        page: "/settings/storage",
        where: ["nav.settings", "storage.title"],
        way: ["/settings", "/settings/storage"],
        text: "whatsNew.settingsStorageList",
      },
      {
        id: "settings-storage-flow",
        page: "/settings/storage",
        where: ["nav.settings", "storage.title"],
        way: ["/settings", "/settings/storage"],
        text: "whatsNew.settingsStorageFlow",
      },
      {
        id: "card-foot",
        page: "/settings/storage",
        where: ["nav.settings", "storage.title"],
        way: ["/settings", "/settings/storage"],
        text: "whatsNew.cardFoot",
      },
      {
        id: "settings-offsite-copies",
        page: "/settings/storage",
        where: ["nav.settings", "storage.title"],
        way: ["/settings", "/settings/storage"],
        text: "whatsNew.offsiteCopies",
      },
      {
        id: "compression-row",
        page: "/settings/storage",
        where: ["nav.settings", "storage.title"],
        way: ["/settings", "/settings/storage"],
        text: "whatsNew.compressionRow",
      },
      {
        id: "storage-add-own-rclone",
        page: "/storage",
        where: ["storage.title"],
        way: [...TO_STORAGE, "add-location"],
        text: "whatsNew.addOwnRclone",
      },
      {
        id: "provider-logos",
        page: "/storage",
        where: ["storage.title"],
        way: [...TO_STORAGE, "add-location"],
        text: "whatsNew.providerLogos",
      },
      { id: "local-protection-card", text: "whatsNew.localProtection" },
      {
        id: "lists-as-rows",
        page: ENTRY,
        go: "/backups",
        where: ["backups.title", "whatsNew.page.entry"],
        way: ["/backups", "entry"],
        text: "whatsNew.listsAsRows",
      },
      {
        id: "card-buttons",
        page: "/settings/security",
        where: ["nav.settings", "settings.tab.security"],
        way: ["/settings", "/settings/security"],
        text: "whatsNew.cardButtons",
      },
      {
        id: "crypt-picture",
        page: "/settings/security",
        where: ["nav.settings", "settings.tab.security"],
        way: ["/settings", "/settings/security"],
        text: "whatsNew.cryptPicture",
      },
      {
        id: "zfs-replica-only-switch",
        page: ENTRY,
        go: "/backups",
        where: ["backups.title", "whatsNew.page.entry"],
        way: TO_REPLICA_CARD,
        text: "whatsNew.zfsReplicaOnlySwitch",
      },
      { id: "whats-new-cards", text: "whatsNew.releaseCards" },
      { id: "widget-domains", text: "whatsNew.widgetDomains" },
      {
        id: "location-remove",
        page: LOCATION,
        go: "/storage",
        where: ["storage.title", "whatsNew.page.location"],
        way: [...TO_STORAGE, "location"],
        text: "whatsNew.locationRemove",
      },
      {
        id: "location-rules",
        page: LOCATION,
        go: "/storage",
        where: ["storage.title", "whatsNew.page.location"],
        way: [...TO_STORAGE, "location"],
        text: "whatsNew.locationRules",
      },
      {
        id: "settings-retention",
        page: "/settings/storage",
        where: ["nav.settings", "storage.title"],
        way: ["/settings", "/settings/storage"],
        text: "whatsNew.settingsRetention",
      },
    ],
  },
  {
    version: "10.1.0",
    changes: [
      {
        id: "schedule-snapshot",
        page: "/settings/schedules",
        where: ["nav.settings", "settings.schedule"],
        way: ["/settings", "/settings/schedules"],
        text: "whatsNew.scheduleSnapshot",
      },
      { id: "containers-restart", text: "whatsNew.containersRestart" },
      { id: "zvol-vms", text: "whatsNew.zvolVms" },
      { id: "zfs-replicate-now", page: ENTRY, go: "/backups", where: ["backups.title", "whatsNew.page.entry"], way: ["/backups", "zfs-replica"], text: "whatsNew.zfsReplicateNow" },
      { id: "zfs-replicate-all", page: "/backups", where: ["backups.title"], way: ["/backups"], wide: true, text: "whatsNew.zfsReplicateAll" },
      { id: "zfs-replica-only", text: "whatsNew.zfsReplicaOnly" },
      {
        id: "zfs-server-mount",
        page: "/instances/zfs/:id",
        go: "/instances",
        where: ["instances.title", "zfs.replica.servers.title"],
        way: ["/settings", "/settings/instances", "/instances", "zfs-server"],
        text: "whatsNew.zfsServerMount",
      },
    ],
  },
  {
    version: "10.0.0",
    changes: [
      { id: "overview-head", page: "/dashboard", where: ["nav.overview"], way: ["/dashboard"], text: "whatsNew.overviewHead" },
      { id: "overview-customize", page: "/dashboard", where: ["nav.overview"], way: ["/dashboard"], wide: true, text: "whatsNew.overviewCustomize" },
      { id: "overview-decline", page: "/dashboard", where: ["nav.overview"], way: ["/dashboard"], text: "whatsNew.overviewDecline" },
      { id: "overview-instances", page: "/dashboard", where: ["nav.overview"], way: ["/dashboard"], app: true, text: "whatsNew.overviewInstances" },

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
      { id: "restore-many", page: "/restore", where: ["nav.restore"], way: ["/restore"], text: "whatsNew.restoreMany" },
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
      {
        id: "instances-phones",
        page: "/instances",
        where: ["instances.title"],
        way: ["/settings", "/settings/instances", "/instances"],
        text: "whatsNew.instancesPhones",
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
      {
        id: "settings-manual",
        page: "/settings/info",
        where: ["nav.settings", "activityLog.glyphInfo"],
        way: ["/settings", "/settings/info"],
        text: "whatsNew.settingsManual",
      },
      {
        id: "look-glyphs",
        page: "/settings/look",
        where: ["nav.settings", "settings.tab.look"],
        way: ["/settings", "/settings/look"],
        text: "whatsNew.lookGlyphs",
      },
    ],
  },
];

export const CHANGES: Change[] = RELEASES.flatMap((release) => release.changes.map((change) => ({ ...change, since: release.version })));
