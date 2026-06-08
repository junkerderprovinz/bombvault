// English is the source of truth: its key set defines TranslationKey, and every
// other locale must provide exactly these keys (enforced at compile time via the
// Translation type and at runtime by test/locales.test.ts).
//
// Count-neutral phrasing is used throughout — no fragile i18next plural forms.
export const en = {
  "language.label": "Language",
  "theme.toggle": "Toggle theme",

  "dashboard.title": "BombVault — Dashboard",
  "dashboard.body": "P0 foundation is running.",
  "dashboard.spikeLink": "Run the host-integration spike",

  "spike.title": "Host Integration Spike",
  "spike.overall": "Overall:",
  "spike.allOk": "ALL OK",
  "spike.degraded": "DEGRADED",
  "spike.colCheck": "Check",
  "spike.colStatus": "Status",
  "spike.colDetail": "Detail",
  "spike.ok": "OK",
  "spike.fail": "FAIL",
  "spike.probeFailed": "probe failed (see server logs)",

  // P1 — container backup/restore
  "nav.containers": "Containers",
  "nav.destinations": "Destinations",

  "containers.title": "Containers",
  "containers.discover": "Discover",
  "containers.backupNow": "Back up now",
  "containers.lastBackup": "Last backup",
  "containers.never": "Never",
  "containers.colName": "Name",
  "containers.colImage": "Image",
  "containers.colStatus": "Status",
  "containers.colAppdata": "Appdata",
  "containers.colActions": "Actions",
  "containers.backupStarted": "Backup started",
  "containers.noDestination": "No destination configured",

  "destinations.title": "Destinations",
  "destinations.localPath": "Repository path",
  "destinations.password": "Repository password",
  "destinations.save": "Save",
  "destinations.initOnSave": "Initialize repository on save",
  "destinations.saved": "Saved",
  "destinations.testOk": "Connection OK",

  "snapshots.title": "Snapshots",
  "snapshots.colId": "ID",
  "snapshots.colTime": "Time",
  "snapshots.colTags": "Tags",
  "snapshots.colSize": "Size",
  "snapshots.restore": "Restore",
  "snapshots.none": "No snapshots found",

  "restore.confirmTitle": "Confirm restore",
  "restore.confirmBody": "This will stop the container, replace its appdata and recreate it from the snapshot. Continue?",
  "restore.confirm": "Confirm",
  "restore.cancel": "Cancel",
  "restore.preview": "Preview",
  "restore.started": "Restore started",

  "run.kindBackup": "Backup",
  "run.kindRestore": "Restore",
  "run.statusRunning": "Running",
  "run.statusSuccess": "Success",
  "run.statusFailed": "Failed",
  "run.historyTitle": "Run history",
  "run.colKind": "Kind",
  "run.colStatus": "Status",
  "run.colStarted": "Started",
  "run.colFinished": "Finished",
} as const;

export type TranslationKey = keyof typeof en;
export type Translation = Record<TranslationKey, string>;
