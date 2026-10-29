// What the settings search knows about: every card on every page, and the
// caption and (i) text of the rows on it, as translation keys. The search
// resolves them through `t`, so it searches in the reader's language.
//
// It is written by hand, because cards take their strings through helpers and
// from other files. searchIndex.test.ts checks every card title a page draws
// against it, page by page.
import type { TranslationKey } from "../../lib/i18n";
import type { SettingsPageId } from "./settingsPages";

export interface SearchRow {
  /** The caption key, which the row shows as its label. */
  key: TranslationKey;
  /** The key of its (i) text, when it has one. */
  hint?: TranslationKey;
}

export interface SearchCard {
  /** The card's title key. */
  title: TranslationKey;
  /** Replacements for `{name}` placeholders in the title, as keys. */
  vars?: Record<string, TranslationKey>;
  /** The card's own (i) text. */
  hint?: TranslationKey;
  rows: SearchRow[];
  /** Further text on the card that is searched but never shown as a result. */
  body?: TranslationKey[];
}

// The five keep rules, drawn with their (i) wherever a keep-policy is edited.
const KEEP_RULE_ROWS: SearchRow[] = [
  { key: "settings.retentionLast", hint: "settings.retentionLastInfo" },
  { key: "settings.retentionDaily", hint: "settings.retentionDailyInfo" },
  { key: "settings.retentionWeekly", hint: "settings.retentionWeeklyInfo" },
  { key: "settings.retentionMonthly", hint: "settings.retentionMonthlyInfo" },
  { key: "settings.retentionYearly", hint: "settings.retentionYearlyInfo" },
];

// The additional off-site targets editor (OffsiteTargetsSection) sits on
// every per-domain off-site Card, so its rows are shared rather than typed
// out six times.
const OFFSITE_TARGET_ROWS: SearchRow[] = [
  { key: "offsite.targets.title" },
  { key: "offsite.targets.name" },
  { key: "offsite.wizard.repoUrl" },
  { key: "offsite.targets.credsLabel" },
  { key: "cloud.storageClass.label" },
  { key: "settings.compression", hint: "settings.compressionInfo" },
  { key: "offsite.immutable", hint: "offsite.immutableHint" },
  { key: "offsite.targets.retentionTitle" },
  ...KEEP_RULE_ROWS,
  { key: "offsite.retention.budget" },
];

/** The cards of each page, in the order the page draws them. */
export const SETTINGS_INDEX: Record<SettingsPageId, SearchCard[]> = {
  general: [
    {
      title: "settings.domains",
      hint: "settings.domainsHint",
      rows: [
        { key: "settings.containersEnabled", hint: "settings.containersEnabledHint" },
        { key: "settings.vmsEnabled", hint: "settings.vmsEnabledHint" },
        { key: "settings.flashEnabled", hint: "settings.flashEnabledHint" },
        { key: "settings.filesEnabled", hint: "settings.filesEnabledHint" },
        { key: "settings.zfsEnabled", hint: "settings.zfsEnabledHint" },
        { key: "settings.configEnabled", hint: "settings.configEnabledHint" },
      ],
    },
    {
      title: "nav.containers",
      hint: "tile.general.containersHint",
      rows: [{ key: "settings.dbDumps", hint: "settings.dbDumpsHint" }],
    },
  ],

  look: [
    {
      title: "tile.look.title",
      hint: "tile.look.hint",
      rows: [
        { key: "language.label" },
        { key: "settings.theme" },
        { key: "settings.shape", hint: "settings.shapeHint" },
        { key: "settings.motion", hint: "settings.motionHint" },
        { key: "settings.quietToasts", hint: "settings.quietToastsHint" },
        { key: "tile.look.order", hint: "tile.look.orderHint" },
      ],
    },
    {
      title: "settings.labels",
      hint: "settings.labelsHint",
      rows: [
        { key: "settings.labels.buttons" },
        { key: "settings.labels.sidebar", hint: "settings.axisSidebarHint" },
        { key: "settings.labels.tabs" },
        { key: "settings.labels.bottombar", hint: "settings.axisBottombarHint" },
      ],
    },
    {
      title: "settings.colors",
      rows: [
        { key: "settings.accentColor", hint: "settings.accentRainbowHint" },
        { key: "settings.rainbow", hint: "settings.rainbowHint" },
        { key: "settings.disco", hint: "settings.discoHint" },
        { key: "settings.rainbowReactive", hint: "settings.rainbowReactiveHint" },
        { key: "settings.rainbowRotate", hint: "settings.rainbowRotateHint" },
        { key: "settings.rainbowPaletteLabel" },
      ],
    },
  ],

  schedules: [
    { title: "tile.week.title", hint: "tile.week.hint", rows: [] },
    {
      title: "settings.everythingTitle",
      hint: "schedules.fullCardHint",
      rows: [
        { key: "schedules.whenRuns", hint: "schedules.whenRunsHint" },
        { key: "settings.catchUpMissed", hint: "settings.catchUpMissedHint" },
        { key: "schedules.hooks" },
        { key: "schedules.hookPre", hint: "schedules.hookPreHint" },
        { key: "schedules.hookPost", hint: "schedules.hookPostHint" },
      ],
    },
    {
      title: "schedules.plans",
      hint: "schedules.plansCardHint",
      rows: [{ key: "exceptions.add" }, { key: "schedules.offsite" }],
    },
    {
      title: "schedules.entries",
      hint: "schedules.entriesHint",
      rows: [{ key: "schedules.perItem", hint: "schedules.perItemHint" }],
    },
    {
      title: "schedules.order",
      hint: "schedules.orderHint",
      rows: [
        { key: "settings.restartHealthWait", hint: "settings.restartHealthWaitHint" },
        { key: "settings.restartHealthTimeoutLabel", hint: "settings.restartHealthTimeoutHint" },
      ],
    },
    {
      title: "settings.containerSnapshotsTitle",
      rows: [{ key: "settings.containerSnapshots", hint: "settings.containerSnapshotsHint" }],
    },
    {
      title: "schedules.updates",
      rows: [
        { key: "settings.updateEveryContainer", hint: "settings.updateEveryContainerHint" },
        { key: "settings.pruneImageAfterUpdate", hint: "settings.pruneImageAfterUpdateHint" },
        { key: "settings.reconcileUnraidStatus", hint: "settings.reconcileUnraidStatusHint" },
      ],
    },
    {
      title: "idle.title",
      hint: "idle.hint",
      rows: [
        { key: "idle.cpu", hint: "idle.cpuHint" },
        { key: "idle.net" },
        { key: "idle.quiet", hint: "idle.quietHint" },
      ],
    },
  ],

  storage: [
    {
      title: "settings.labels.sidebar",
      rows: [{ key: "tile.sidebar.show", hint: "tile.sidebar.showHint" }],
    },
    {
      title: "storage.title",
      rows: [],
      body: [
        "storage.add.title",
        "storage.add.ownRclone",
        "storage.add.ownRcloneTip",
        "rclone.paste",
        "rclone.configured",
        "storage.connection",
        "storage.folder",
        "storage.credentials",
        "storage.protected",
        "storage.copy.title",
        "storage.keep.title",
        "storage.rules.title",
        "storage.streaming.title",
        "streaming.toggle",
        "streaming.servers",
        "streaming.threshold",
        "streaming.limit",
        "streaming.hold",
      ],
    },
    {
      title: "signIns.title",
      hint: "signIns.hint",
      rows: [],
      body: ["signIns.makeDefault", "signIns.test", "cloud.credSets.name", "cloud.storageClass.label"],
    },
    {
      title: "storage.flow.title",
      hint: "storage.flow.hint",
      rows: [],
      body: ["storage.flow.graph", "storage.flow.switches"],
    },
    {
      title: "repos.title",
      hint: "repos.intro",
      rows: [
        { key: "repos.name" },
        { key: "repos.location", hint: "repos.locationHint" },
      ],
    },
    {
      title: "storage.keep.title",
      rows: [{ key: "settings.retentionWhere" }, { key: "storage.rules.open" }],
    },
    {
      title: "restore.preview",
      rows: [
        { key: "retentionPreview.title", hint: "retentionPreview.hint" },
        { key: "retentionPreview.sourceLabel" },
        { key: "retentionPreview.show" },
      ],
    },
    {
      title: "offsite.copies.title",
      hint: "settings.offsiteHint",
      rows: OFFSITE_TARGET_ROWS,
      body: ["offsite.targets.hint", "offsite.targets.scheduleNote", "offsite.test", "offsite.replicateNow", "offsite.wizard.setup"],
    },
    {
      title: "settings.offsiteLimits",
      hint: "settings.limitHint",
      rows: [{ key: "settings.limitUpload" }, { key: "settings.limitDownload" }],
    },
  ],

  restore: [
    {
      title: "tile.restore.folders",
      hint: "tile.restore.foldersHint",
      rows: [
        { key: "settings.restoreFolder", hint: "settings.restoreFolderHint" },
        { key: "tile.restore.perSection", hint: "tile.restore.perSectionHint" },
        { key: "tile.restore.exception" },
      ],
    },
    {
      title: "settings.registriesTitle",
      hint: "settings.registriesHint",
      rows: [
        { key: "settings.registryHost" },
        { key: "settings.registryUser" },
        { key: "settings.registryToken" },
      ],
    },
    {
      title: "tile.restore.emergency",
      rows: [{ key: "recovery.title", hint: "recovery.why" }],
      body: ["tile.restore.verifyLine"],
    },
  ],

  notifications: [
    {
      title: "notify.title",
      hint: "notify.hint",
      rows: [
        { key: "notify.on" },
        { key: "notify.scheduledSummary", hint: "notify.scheduledSummaryHint" },
        { key: "notify.notifyOnUpdate", hint: "notify.notifyOnUpdateHint" },
        { key: "settings.watchdogToggle", hint: "settings.watchdogHint" },
        { key: "notify.unraid", hint: "notify.unraidHint" },
        { key: "settings.digestToggle", hint: "settings.digestHint" },
      ],
    },
    {
      title: "notify.channelsTitle",
      hint: "notify.channelsHint",
      rows: [
        { key: "notify.webhookChannel" },
        { key: "notify.webhook" },
        { key: "notify.webhookFormat" },
        { key: "notify.apprise", hint: "notify.appriseHint" },
        { key: "notify.appriseUrl" },
        { key: "notify.appriseTags" },
        { key: "notify.matrix" },
        { key: "notify.matrixHomeserver" },
        { key: "notify.matrixToken" },
        { key: "notify.matrixRoom" },
        { key: "notify.smtp" },
        { key: "notify.smtpHost" },
        { key: "notify.smtpPort" },
        { key: "notify.smtpTls" },
        { key: "notify.smtpUser" },
        { key: "notify.smtpPass" },
        { key: "notify.smtpFrom" },
        { key: "notify.smtpTo" },
      ],
    },
    {
      title: "notify.healthchecksTitle",
      hint: "notify.healthchecksHint",
      rows: [{ key: "notify.healthchecks", hint: "notify.healthchecksLifecycle" }, { key: "exceptions.add" }],
    },
  ],

  integrity: [
    {
      title: "integrity.checkTitle",
      hint: "integrity.checkHint",
      rows: [],
    },
    {
      title: "verify.autoChecks",
      rows: [
        { key: "verify.trialRestore", hint: "verify.trialRestoreHint" },
        { key: "settings.offsiteDrills", hint: "settings.offsiteDrillsHelp" },
        { key: "settings.startTest", hint: "settings.startTestHelp" },
        { key: "verify.testTarget", hint: "verify.testTargetHint" },
        { key: "verify.testTargetVM" },
        { key: "verify.subsetPct", hint: "verify.subsetPctHint" },
        { key: "verify.tamper", hint: "verify.tamperHint" },
      ],
    },
    {
      title: "anomaly.settings.title",
      hint: "anomaly.settings.hint",
      rows: [
        { key: "anomaly.settings.toggle" },
        { key: "anomaly.settings.sensitivity", hint: "anomaly.settings.sensitivityHint" },
        { key: "anomaly.settings.notifyMin", hint: "anomaly.settings.notifyHint" },
        { key: "anomaly.settings.holdToggle", hint: "anomaly.settings.holdHint" },
      ],
    },
  ],

  security: [
    {
      title: "auth.security",
      hint: "auth.passwordHint",
      rows: [
        { key: "auth.currentPassword" },
        { key: "auth.setPassword" },
        { key: "auth.changePassword" },
        { key: "auth.confirmPassword", hint: "auth.passwordMinHint" },
      ],
    },
    {
      title: "auth.twoFactor",
      hint: "auth.twoFactorHint",
      rows: [
        { key: "auth.secretManual" },
        { key: "auth.confirmCode" },
        { key: "auth.disableCodePrompt" },
      ],
    },
    {
      title: "auth.passkeys",
      hint: "auth.passkeysHint",
      rows: [{ key: "auth.passkeyNameLabel" }],
    },
    {
      title: "apps.phone.title",
      rows: [{ key: "auth.phonePairing", hint: "auth.phonePairingHint" }],
    },
    {
      title: "tile.crypt.title",
      rows: [
        { key: "tile.crypt.toggle", hint: "settings.encryptionHint" },
      ],
      body: ["settings.encryptionPasswordWhere", "tile.crypt.line"],
    },
  ],

  instances: [
    {
      title: "settings.labels.sidebar",
      rows: [{ key: "tile.sidebar.show", hint: "tile.sidebar.showHint" }],
    },
    {
      title: "tile.instances.modules",
      hint: "tile.instances.modulesHint",
      rows: [
        { key: "instances.title", hint: "settings.fleetEnabledHint" },
        { key: "receiver.title", hint: "settings.receiverEnabledHint" },
        { key: "pull.title", hint: "settings.pullEnabledHint" },
      ],
    },
  ],

  pairing: [
    { title: "pairing.title", rows: [], body: ["settings.pairingNeedsDomain"] },
    { title: "pairing.step1Title", rows: [], body: ["pairing.step1Body"] },
    { title: "pairing.stepOtherTitle", rows: [], body: ["pairing.step2Body"] },
    { title: "pairing.step3Title", rows: [], body: ["pairing.step3Body"] },
    {
      title: "pairing.phraseTitle",
      hint: "pairing.phraseTip",
      rows: [
        { key: "pairing.passwordLabel" },
        { key: "pairing.wordsLabel" },
        { key: "pairing.enterLabel" },
        { key: "pairing.enterLabelOther" },
        { key: "pairing.cantFindLabel" },
      ],
    },
    {
      title: "instances.title",
      hint: "pairing.group.hint",
      rows: [{ key: "pairing.group.zfsTitle", hint: "pairing.group.zfsTip" }],
    },
    {
      title: "settings.fleet",
      rows: [{ key: "settings.instanceName", hint: "settings.fleetHint" },{ key: "pairing.knownAddresses", hint: "relay.selfAddressTip" }],
    },
    {
      title: "relay.title",
      hint: "relay.hint",
      rows: [
        { key: "relay.serve", hint: "relay.serveHint" },
        { key: "relay.addressLabel", hint: "relay.addressTip" },
      ],
    },
  ],

  integrations: [
    {
      title: "mcp.title",
      hint: "mcp.hint",
      rows: [
        { key: "mcp.endpointLabel", hint: "mcp.endpointHint" },
        { key: "mcp.oauthToggle", hint: "mcp.oauthHint" },
        { key: "mcp.oauthAddressLabel", hint: "mcp.oauthAddressHint" },
        { key: "mcp.oauthConnectorLabel", hint: "mcp.oauthConnectorHint" },
        { key: "mcp.snippetsLabel", hint: "mcp.connectHint" },
        { key: "mcp.allowStart" },
      ],
    },
    {
      title: "api.title",
      hint: "api.hint",
      rows: [
        { key: "api.addressLabel", hint: "api.addressHint" },
        { key: "api.labelLabel" },
        { key: "mcp.allowStart", hint: "api.allowStartHint" },
        { key: "api.newTokenTitle", hint: "mcp.showOnce" },
        { key: "api.description", hint: "api.descriptionHint" },
      ],
    },
    {
      title: "integrations.services",
      rows: [
        { key: "ha.title", hint: "ha.hint" },
        { key: "ha.enable" },
        { key: "ha.host" },
        { key: "notify.smtpPort" },
        { key: "notify.smtpUser" },
        { key: "notify.smtpPass" },
        { key: "ha.prefix", hint: "ha.prefixHint" },
        { key: "ha.tls", hint: "ha.tlsHint" },
        { key: "ha.buttons", hint: "ha.buttonsHint" },
        { key: "mdns.enable", hint: "mdns.hint" },
        { key: "settings.metricsEnable", hint: "settings.metricsHint" },
        { key: "settings.metricsToken" },
      ],
    },
    {
      title: "settings.widget",
      hint: "settings.widgetHint",
      rows: [
        { key: "settings.widgetToken", hint: "settings.widgetAccess" },
        { key: "settings.dashTile", hint: "settings.dashTileHint" },
        { key: "settings.widgetSetup" },
      ],
    },
    {
      title: "vm.ssh.title",
      hint: "vm.ssh.cardHint",
      rows: [
        { key: "vm.ssh.host", hint: "vm.ssh.hostHint" },
        { key: "vm.ssh.keyLabel", hint: "vm.ssh.keyHint" },
        { key: "settings.widgetSetup" },
      ],
    },
  ],

  apps: [
    {
      title: "apps.phone.title",
      hint: "apps.phone.hint",
      rows: [],
    },
    {
      title: "apps.parleyport.title",
      hint: "apps.parleyport.hint",
      rows: [{ key: "apps.parleyport.toRelay" }],
    },
    {
      title: "apps.widget.title",
      hint: "apps.widget.cardHint",
      rows: [{ key: "settings.dashTile", hint: "settings.dashTileHint" }],
    },
  ],

  system: [
    {
      title: "settingsIO.title",
      hint: "settingsIO.desc",
      rows: [
        { key: "settingsIO.exportHeading" },
        { key: "settingsIO.includeCreds" },
        { key: "export.encrypt.enable", hint: "export.encrypt.hint" },
        { key: "export.encrypt.recipients", hint: "export.encrypt.recipientsHint" },
        { key: "settingsIO.importHeading", hint: "settingsIO.importHint" },
      ],
      body: ["export.encrypt.ageInfo", "export.encrypt.enableHint", "export.encrypt.kitSealed"],
    },
    {
      title: "tile.system.performance",
      hint: "tile.system.performanceHint",
      rows: [
        { key: "settings.cacheLimitLabel", hint: "settings.cacheHint" },
        { key: "settings.coresLabel", hint: "settings.coresHint" },
      ],
    },
  ],

  info: [
    {
      title: "about.version",
      hint: "tile.info.versionHint",
      rows: [{ key: "tile.info.check" }, { key: "whatsNew.title", hint: "whatsNew.hint" }, { key: "whatsNew.markAll" }, { key: "whatsNew.showAll" }],
      body: ["tile.info.glimstoneHint", "tile.info.resticHint", "tile.info.rcloneHint"],
    },
    {
      title: "tile.info.help",
      hint: "tile.info.helpHint",
      rows: [{ key: "tile.info.manual" }, { key: "tile.info.bugReport" }],
      body: ["about.report"],
    },
    { title: "about.title", rows: [], body: ["about.body"] },
  ],
};
