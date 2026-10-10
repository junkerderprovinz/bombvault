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

// The six sources of the retention cards, each the caption of its switch.
const SOURCE_ROWS = (hint: TranslationKey): SearchRow[] =>
  (["nav.containers", "nav.vms", "nav.flash", "nav.files", "nav.zfs", "nav.config"] as const).map((key) => ({ key, hint }));

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
      title: "settings.schedulesOptions",
      rows: [
        { key: "settings.perItemSchedules", hint: "settings.perItemSchedulesHint" },
        { key: "jobs.syncSchedules", hint: "jobs.syncSchedulesHint" },
      ],
    },
    {
      title: "settings.everythingTitle",
      hint: "settings.everythingHint",
      rows: [
        { key: "hooks.title", hint: "settings.everythingHooksHint" },
        { key: "hooks.pre" },
        { key: "hooks.post" },
      ],
    },
    { title: "jobs.containersSection", hint: "containers.scheduleHint", rows: [] },
    { title: "jobs.vmsSection", hint: "jobs.vmIncludeHint", rows: [] },
    { title: "jobs.flashSection", hint: "jobs.flashScheduleHint", rows: [] },
    { title: "jobs.filesSection", hint: "jobs.filesIncludeHint", rows: [{ key: "files.enabled" }] },
    { title: "jobs.zfsSection", hint: "jobs.zfsIncludeHint", rows: [{ key: "files.enabled" }] },
    {
      title: "settings.schedulesOffsite",
      rows: [
        { key: "nav.containers" },
        { key: "nav.vms" },
        { key: "nav.flash" },
        { key: "nav.config" },
        { key: "nav.files" },
        { key: "nav.zfs" },
      ],
    },
    {
      title: "settings.schedulesSelfBackup",
      hint: "config.scheduleHint",
      rows: [{ key: "settings.schedulesSelfBackup" }],
    },
    {
      title: "settings.missedSchedulesTitle",
      rows: [{ key: "settings.catchUpMissed", hint: "settings.catchUpMissedHint" }],
    },
    { title: "backupOrder.title", hint: "backupOrder.hint", rows: [] },
    { title: "vmBackupOrder.title", hint: "vmBackupOrder.hint", rows: [] },
    {
      title: "settings.restartHealthTitle",
      rows: [
        { key: "settings.restartHealthWait", hint: "settings.restartHealthWaitHint" },
        { key: "settings.restartHealthTimeoutLabel", hint: "settings.restartHealthTimeoutHint" },
      ],
    },
    {
      title: "settings.imageMaintenanceTitle",
      hint: "settings.imageMaintenanceHint",
      rows: [
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
      title: "repos.title",
      hint: "repos.intro",
      rows: [
        { key: "repos.immutable", hint: "repos.immutableHint" },
        { key: "settings.compression", hint: "settings.compressionInfo" },
        { key: "repos.offPremises", hint: "repos.offPremisesHint" },
        { key: "repos.enabled" },
        { key: "repos.name" },
        { key: "repos.location", hint: "repos.locationHint" },
      ],
    },
    {
      title: "placementDefaults.title",
      hint: "placementDefaults.hint",
      rows: [
        { key: "placement.title" },
        { key: "placement.segLocal" },
        { key: "placement.storedOn" },
        { key: "placementDefaults.apply" },
        { key: "placementDefaults.confirm" },
      ],
      body: ["placementDefaults.paused", "placementDefaults.pausedHint"],
    },
    {
      title: "settings.paths",
      hint: "settings.pathsHint",
      rows: [
        { key: "settings.containersPath" },
        { key: "settings.vmsPath" },
        { key: "settings.flashPath" },
        { key: "settings.configPath" },
        { key: "settings.filesPath" },
        { key: "settings.zfsPath" },
        { key: "settings.compression", hint: "settings.compressionInfo" },
      ],
    },
    {
      title: "settings.retentionLocalTitle",
      hint: "settings.retentionHint",
      rows: [
        { key: "retentionPreview.sharedPolicy" },
        ...KEEP_RULE_ROWS,
        { key: "settings.ownRetentionTitle", hint: "settings.ownRetentionHint" },
        { key: "settings.ownRetention", hint: "settings.ownRetentionToggleHint" },
        ...SOURCE_ROWS("settings.ownRetentionToggleHint"),
      ],
      body: ["settings.retentionCombineInfo"],
    },
    {
      title: "settings.ownRetentionTitle",
      hint: "settings.ownRetentionHint",
      rows: [
        { key: "settings.ownRetention", hint: "settings.ownRetentionToggleHint" },
        { key: "settings.retentionLast", hint: "settings.retentionLastInfo" },
        { key: "settings.retentionDaily", hint: "settings.retentionDailyInfo" },
        { key: "settings.retentionWeekly", hint: "settings.retentionWeeklyInfo" },
        { key: "settings.retentionMonthly", hint: "settings.retentionMonthlyInfo" },
        { key: "settings.retentionYearly", hint: "settings.retentionYearlyInfo" },
      ],
    },
    {
      title: "settings.retentionOffsiteTitle",
      hint: "settings.retentionOffsiteHint",
      rows: [
        { key: "retentionPreview.sharedPolicy" },
        ...KEEP_RULE_ROWS,
        { key: "settings.ownRetentionTitle", hint: "settings.ownOffsiteRetentionHint" },
        { key: "settings.ownOffsiteRetention", hint: "settings.ownOffsiteRetentionToggleHint" },
        ...SOURCE_ROWS("settings.ownOffsiteRetentionToggleHint"),
        { key: "settings.retentionExtraTargets" },
      ],
      body: ["settings.retentionCombineInfo", "settings.retentionImmutableNotPruned"],
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
      title: "dest.title",
      hint: "dest.hint",
      rows: [{ key: "dest.add" }, { key: "dest.name" }, { key: "offsite.immutable" }],
    },
    {
      title: "offsite.copyDomainTitle",
      vars: { domain: "nav.containers" },
      rows: OFFSITE_TARGET_ROWS,
      body: ["offsite.targets.hint", "offsite.targets.scheduleNote", "settings.offsiteHint"],
    },
    {
      title: "offsite.copyDomainTitle",
      vars: { domain: "nav.vms" },
      rows: OFFSITE_TARGET_ROWS,
      body: ["offsite.targets.hint", "offsite.targets.scheduleNote"],
    },
    {
      title: "offsite.copyDomainTitle",
      vars: { domain: "nav.flash" },
      rows: OFFSITE_TARGET_ROWS,
      body: ["offsite.targets.hint", "offsite.targets.scheduleNote"],
    },
    {
      title: "offsite.copyDomainTitle",
      vars: { domain: "nav.files" },
      rows: OFFSITE_TARGET_ROWS,
      body: ["offsite.targets.hint", "offsite.targets.scheduleNote"],
    },
    {
      title: "offsite.copyDomainTitle",
      vars: { domain: "nav.zfs" },
      rows: OFFSITE_TARGET_ROWS,
      body: ["offsite.targets.hint", "offsite.targets.scheduleNote"],
    },
    {
      title: "offsite.copyDomainTitle",
      vars: { domain: "nav.config" },
      rows: OFFSITE_TARGET_ROWS,
      body: ["offsite.targets.hint", "offsite.targets.scheduleNote"],
    },
    {
      title: "settings.offsiteLimits",
      hint: "settings.limitHint",
      rows: [{ key: "settings.limitUpload" }, { key: "settings.limitDownload" }],
    },
    {
      title: "streaming.title",
      hint: "streaming.hint",
      rows: [
        { key: "streaming.toggle", hint: "streaming.toggleHint" },
        { key: "streaming.servers", hint: "streaming.serversHint" },
        { key: "streaming.threshold", hint: "streaming.thresholdHint" },
        { key: "streaming.limit", hint: "streaming.limitHint" },
        { key: "streaming.hold", hint: "streaming.holdHint" },
      ],
    },
    {
      title: "rclone.title",
      hint: "rclone.hint",
      rows: [
        { key: "rcloneRemote.heading", hint: "rcloneRemote.hint" },
        { key: "rcloneRemote.type" },
        { key: "rcloneRemote.name" },
        { key: "rcloneRemote.host" },
        { key: "rcloneRemote.share" },
        { key: "rcloneRemote.url" },
        { key: "rcloneRemote.vendor" },
        { key: "rcloneRemote.user" },
        { key: "rcloneRemote.password" },
      ],
      body: ["rclone.pathHint"],
    },
    {
      title: "cloud.title",
      rows: [{ key: "cloud.storageClass.label", hint: "cloud.storageClass.hint" }],
      body: ["cloud.hint"],
    },
    {
      title: "cloud.credSets.title",
      hint: "cloud.credSets.hint",
      rows: [{ key: "cloud.credSets.name" }, { key: "cloud.storageClass.label" }],
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
        { key: "notify.unraid", hint: "notify.unraidHint" },
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
      rows: [
        { key: "notify.healthchecks", hint: "notify.healthchecksLifecycle" },
        { key: "notify.hcPerDomain", hint: "notify.hcPerDomainHint" },
        { key: "nav.containers" },
        { key: "nav.vms" },
        { key: "nav.flash" },
        { key: "nav.config" },
        { key: "nav.files" },
        { key: "nav.zfs" },
      ],
    },
    {
      title: "settings.digestTitle",
      hint: "settings.digestHint",
      rows: [{ key: "settings.digestToggle" }],
    },
    {
      title: "settings.watchdogTitle",
      hint: "settings.watchdogHint",
      rows: [{ key: "settings.watchdogToggle" }],
    },
  ],

  integrity: [
    {
      title: "integrity.title",
      hint: "integrity.hint",
      rows: [
        { key: "source.label" },
        { key: "drill.kindLabel" },
        { key: "drill.target", hint: "drill.drNote" },
        { key: "drill.targetVM" },
      ],
    },
    {
      title: "verify.auto",
      hint: "verify.hint",
      rows: [
        { key: "verify.auto" },
        { key: "settings.offsiteDrills", hint: "settings.offsiteDrillsHelp" },
        { key: "settings.startTest", hint: "settings.startTestHelp" },
        { key: "verify.subsetPct" },
      ],
    },
    { title: "settings.schedulesChecks", rows: [] },
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
      title: "tile.crypt.title",
      rows: [
        { key: "settings.encryptionOn", hint: "settings.encryptionHint" },
        { key: "settings.encryptionOff", hint: "settings.encryptionHint" },
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
      hint: "settings.fleetHint",
      rows: [{ key: "settings.instanceName" }, { key: "relay.selfAddressLabel", hint: "relay.selfAddressTip" }],
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
      ],
    },
    {
      title: "ha.title",
      hint: "ha.hint",
      rows: [
        { key: "ha.enable" },
        { key: "ha.host" },
        { key: "notify.smtpPort" },
        { key: "notify.smtpUser" },
        { key: "notify.smtpPass" },
        { key: "ha.prefix", hint: "ha.prefixHint" },
        { key: "ha.tls", hint: "ha.tlsHint" },
        { key: "ha.buttons", hint: "ha.buttonsHint" },
      ],
    },
    {
      title: "mdns.title",
      hint: "mdns.hint",
      rows: [{ key: "mdns.enable" }],
    },
    {
      title: "settings.metrics",
      rows: [
        { key: "settings.metricsEnable", hint: "settings.metricsHint" },
        { key: "settings.metricsToken" },
      ],
    },
    {
      title: "settings.widget",
      hint: "settings.widgetHint",
      rows: [{ key: "settings.widgetToken" }],
    },
    {
      title: "vm.ssh.title",
      hint: "vm.ssh.desc",
      rows: [
        { key: "vm.ssh.host" },
        { key: "vm.ssh.publicKey" },
        { key: "vm.ssh.setupTitle" },
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
      hint: "apps.widget.hint",
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
      rows: [{ key: "tile.info.check" }],
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
