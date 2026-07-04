// ---------------------------------------------------------------------------
// i18n — React Context-based, 26 locales, flag switcher support
// ---------------------------------------------------------------------------

import { createContext, useContext, useState, useCallback } from "react";
import type { ReactNode } from "react";
import { createElement } from "react";

// Translated locales (en + de are defined inline below as the source of truth).
import fr from "./locales/fr";
import es from "./locales/es";
import it from "./locales/it";
import pt from "./locales/pt";
import nl from "./locales/nl";
import pl from "./locales/pl";
import ru from "./locales/ru";
import uk from "./locales/uk";
import cs from "./locales/cs";
import sv from "./locales/sv";
import da from "./locales/da";
import fi from "./locales/fi";
import no from "./locales/no";
import tr from "./locales/tr";
import el from "./locales/el";
import hu from "./locales/hu";
import ro from "./locales/ro";
import ja from "./locales/ja";
import ko from "./locales/ko";
import zh from "./locales/zh";
import ar from "./locales/ar";
import he from "./locales/he";
import th from "./locales/th";
import vi from "./locales/vi";

// ---------------------------------------------------------------------------
// Translation key set — en is the source of truth
// ---------------------------------------------------------------------------

const en = {
  // General
  "language.label": "Language",
  "theme.toggle": "Toggle theme",
  "theme.dark": "Dark",
  "theme.light": "Light",

  // Nav
  "nav.dashboard": "Dashboard",
  "nav.containers": "Containers",
  "nav.vms": "VMs",
  "nav.flash": "Flash",
  "nav.config": "Config",
  "nav.settings": "Settings",
  "nav.reportBug": "Report a bug",
  "nav.advanced": "Advanced",
  "nav.comingSoon": "Coming soon",

  // Dashboard
  "dashboard.title": "Dashboard",
  "dashboard.lastBackups": "Last Backups",
  "dashboard.recentRuns": "Recent Runs",
  "dashboard.spikeStatus": "System Status",
  "dashboard.noRuns": "No runs yet",
  "dashboard.spikeLink": "Run host-integration check",
  "dashboard.hostIntegrationCheck": "Host Integration Check",
  "dashboard.allOk": "All systems OK",
  "dashboard.degraded": "Degraded",
  "dashboard.checking": "Checking…",

  // Spike
  "spike.title": "Host Integration",
  "spike.overall": "Overall:",
  "spike.allOk": "ALL OK",
  "spike.degraded": "DEGRADED",
  "spike.colCheck": "Check",
  "spike.colStatus": "Status",
  "spike.colDetail": "Detail",
  "spike.ok": "OK",
  "spike.fail": "FAIL",
  "spike.bestEffort": "optional",
  "spike.checkNow": "Check now",
  "spike.probeFailed": "probe failed (see server logs)",

  // Containers
  "containers.title": "Containers",
  "containers.discover": "Discover backups",
  "containers.discovering": "Discovering…",
  "containers.discoverHint": "Lost /config? Rebuild the backup list from storage.",
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
  "containers.includeInSchedule": "Include in schedule",
  "containers.schedule": "Schedule",
  "containers.notInstalled": "Not installed",
  "containers.notInstalledTitle": "Not installed (backups only)",
  "containers.notInstalledHint": "These containers are no longer installed but still have backups. Restore them, or delete their backups to free space.",
  "containers.deleteBackups": "Delete all backups",
  "containers.deleteBackupsConfirm": "Delete ALL backups of this container? The snapshots are permanently removed from the repository and cannot be undone.",
  "containers.filter": "Filter:",
  "containers.filterAll": "All",
  "containers.filterInstalled": "Installed",
  "containers.selectAll": "Select all",
  "containers.selectedCount": "selected",
  "containers.backupSelected": "Back up selected",
  "containers.restoreSelected": "Restore selected (latest)",
  "containers.restoreSelectedConfirm": "Restore the LATEST backup of the selected containers? Each is stopped, its appdata replaced, and the container recreated.",
  "containers.clearSelection": "Clear",
  "containers.working": "Working…",
  "containers.batchStarted": "Backup started — it runs on the server, so you can close this tab.",
  "containers.batchAlreadyRunning": "A batch backup is already running.",
  "containers.batchRunning": "Backing up selected containers…",
  "containers.selfNote": "This is BombVault — it doesn't back up its own container (that would stop itself); its settings are recovered via Discover.",

  // Backups (restic snapshots, shown to the user as "Backups")
  "snapshots.title": "Backups",
  "snapshots.colId": "ID",
  "snapshots.colTime": "Time",
  "snapshots.colTags": "Tags",
  "snapshots.colSize": "Size",
  "snapshots.restore": "Restore",
  "snapshots.none": "No backups found",
  "snapshots.files": "Files",
  "snapshots.delete": "Delete",
  "snapshots.deleteConfirm": "Delete this backup? The snapshot is removed from the repository (run Prune in Settings to reclaim the space). This cannot be undone.",
  "snapshots.deleteAll": "Delete all backups",
  "snapshots.deleteAllConfirm": "Delete ALL backups of this VM from the selected source (local or off-site)? The snapshots are permanently removed and the repository is pruned. This cannot be undone.",
  "snapshots.deletingAll": "Deleting…",
  "snapshots.recreate": "Recreate from saved config",
  "snapshots.recreateConfirm": "Recreate this container from its saved configuration? It is created and started from the stored definition (image, env, ports, volumes).",
  "snapshots.configOnlyHint": "Config-only backup: the container's definition is saved, but it has no data folders to snapshot. If you delete the container it appears under \"Not installed\", where you can recreate it from this config.",

  // Snapshot tags + compare (diff)
  "snapshot.tags": "Tags",
  "snapshot.addTag": "Add tag",
  "snapshot.compare": "Compare",
  "snapshot.pickTwo": "Pick two snapshots to compare",
  "snapshot.added": "added",
  "snapshot.removed": "removed",
  "snapshot.changed": "changed",
  "snapshot.diffSummary": "+{addedFiles} files ({addedBytes}), ~{changedFiles} changed, -{removedFiles} files ({removedBytes})",

  // File-level restore
  "files.restore": "Restore",
  "files.restored": "Restored",
  "files.restoreConfirm": "Restore the selected files to their original locations? Existing files will be overwritten.",
  "files.filterPlaceholder": "Filter files…",
  "files.none": "No matching files",
  "files.loadFailed": "Failed to load files",
  "files.more": "Refine the filter to see more files.",
  "files.selectHint": "Tick the files and folders to restore, then choose where.",
  "files.dest.inPlace": "Restore in place (original location)",
  "files.dest.toFolder": "Restore to a folder",
  "files.restoreSelected": "Restore selected ({n})",
  "files.restoredInPlace": "Restored the selected files to their original location.",

  // Restore
  "restore.confirmTitle": "Confirm restore",
  "restore.confirmBody":
    "This will stop the container, replace its appdata and recreate it from the backup. Continue?",
  "restore.confirm": "Confirm",
  "restore.cancel": "Cancel restore",
  "restore.cancelConfirmSafe": "Cancel the restore? The partial output folder is left as-is.",
  "restore.cancelConfirmInPlace":
    "{name} is mid-restore. Cancelling leaves this restore partial — you may need to restore it again. Cancel anyway?",
  "restore.cancelling": "Cancelling…",
  "restore.cancelled": "Restore cancelled",
  "restore.preview": "Preview",
  "restore.started": "Restore started",
  "restore.toFolder": "Restore to folder…",
  "restore.toFolderHint":
    "Extracts this snapshot into a folder under your backup mount. The running container is not touched.",
  "restore.targetPath": "Target folder",
  "restore.restoredTo": "Restored to {path}",
  "restore.progress": "Restoring… {pct}%",
  "restore.open": "Restore…",
  "restore.mode.inPlace": "Restore in place",
  "restore.mode.files": "Individual files",
  "restore.mode.toFolder": "To a folder",
  "restore.inPlaceHint": "Recreate this container exactly as it was.",
  "restore.leaveStopped": "Leave stopped after restore (don't start it)",
  "restore.bgHint":
    "Running in the background — you can close this panel; the outcome appears in the run history.",

  // Stacks (compose-project restore)
  "stack.title": "Stacks",
  "stack.restore": "Restore stack…",
  "stack.members": "{n} containers",
  "stack.restoreHint":
    "Restores every container in this stack from its latest backup, left stopped, then (optionally) starts them in dependency order.",
  "stack.startInOrder": "Start in dependency order after restore",
  "stack.restoreConfirm":
    "Restore all containers in this stack? Each is recreated from its latest backup.",
  "stack.restoring": "Restoring stack…",
  "stack.restored": "Stack restored",
  "stack.restoreFinished": "Stack restore finished — see the run history for per-container results.",
  "stack.memberRestored": "restored",
  "stack.memberStarted": "started",

  // Runs
  "run.kindBackup": "Backup",
  "run.kindRestore": "Restore",
  "run.statusRunning": "Running",
  "run.statusSuccess": "Success",
  "run.statusFailed": "Failed",
  "run.historyTitle": "Run History",
  "run.filterDay": "Day:",
  "run.allDays": "All days",
  "run.colKind": "Kind",
  "run.colStatus": "Status",
  "run.colStarted": "Started",
  "run.colFinished": "Finished",
  "run.colContainer": "Container",

  // Settings
  "settings.title": "Settings",
  "settings.encryption": "Encryption",
  "settings.encryptionOn": "Enabled (password derived from APP_KEY)",
  "settings.encryptionOff": "Disabled (no password)",
  "settings.encryptionWarning":
    "Encryption is fixed per repository at init time. Changing this requires a new empty path.",

  // Encryption-key recovery kit
  "recovery.title": "Recovery kit",
  "recovery.download": "Download recovery kit",
  "recovery.why":
    "With encryption on, your APP_KEY is the master secret for every backup. Download a recovery kit (the key, the derived restic password, repo locations and manual restore steps) so you can restore even without a running BombVault container. Store it offline and securely.",
  "recovery.nagTitle": "Save your recovery kit",
  "recovery.nagBody":
    "With encryption on, losing your APP_KEY means losing your backups. Download the recovery kit and store it somewhere safe and offline.",
  "recovery.stored": "I've stored it safely",

  "settings.paths": "Backup Paths",
  "settings.containersPath": "Containers path",
  "settings.vmsPath": "VMs path",
  "settings.flashPath": "Flash path",
  "settings.restoreFolder": "Default restore folder",
  "settings.restoreFolderHint": "Where 'restore to a folder' extracts snapshots by default.",
  "settings.offsiteTitle": "Off-site copy (optional)",
  "settings.offsiteHint": "After each successful local backup, also replicate it to a second repo with restic copy. Enter a remote (rest:http://host:8000/repo, s3:…, b2:…) or a local subpath; leave blank to disable. The local backup stays primary.",
  "source.label": "Source:",
  "source.local": "Local",
  "source.offsite": "Off-site",
  "source.hint": "Restore and delete act on the selected source only — deleting a local backup never touches the off-site copy, and vice versa.",
  "offsite.schedulePlaceholder": "blank = after each backup · e.g. weekly Sun 03:00",
  "offsite.replicateNow": "Replicate now",
  "offsite.replicating": "Replicating…",
  "offsite.replicateFailed": "Replication failed",
  "offsite.test": "Test connection",
  "offsite.testing": "Testing…",
  "offsite.testOk": "reachable + initialised",
  "offsite.testUninitialized": "reachable, not initialised",
  "offsite.testFailed": "not reachable",
  // Off-site setup wizard (v4 ransomware protection)
  "offsite.wizard.setup": "Set up…",
  "offsite.wizard.close": "Close",
  "offsite.wizard.step1": "1 · Choose a backend",
  "offsite.wizard.backendRest": "rest-server (recommended — append-only capable)",
  "offsite.wizard.backendRclone": "rclone remote",
  "offsite.wizard.backendS3": "Amazon S3 / S3-compatible",
  "offsite.wizard.step2": "2 · Deploy the append-only server",
  "offsite.wizard.step2Hint": "Run this on your storage box to stand up a restic rest-server with --append-only. The generated password is shown only once.",
  "offsite.wizard.generate": "Generate deployment snippet",
  "offsite.wizard.regenerate": "Regenerate (new password)",
  "offsite.wizard.snippetError": "Could not generate the snippet",
  "offsite.wizard.passwordWarning": "This password is shown ONCE and is never stored by BombVault. Save it now — you need it for the credentials below and it cannot be recovered.",
  "offsite.wizard.tlsNote": "This recipe uses plain HTTP — fine on a trusted LAN or VPN. If the storage box is reachable over the internet, put rest-server behind HTTPS (a TLS reverse proxy) so the repository credential isn't sent in the clear.",
  "offsite.wizard.password": "Generated password (save this)",
  "offsite.wizard.step3": "3 · Repository URL + credentials",
  "offsite.wizard.repoUrl": "Off-site repository URL",
  "offsite.wizard.repoUrlPlaceholder": "rest:http://192.168.x.x:8000/bombvault-containers/containers",
  "offsite.wizard.saveRepo": "Save repository",
  "offsite.wizard.credentials": "REST server credentials",
  "offsite.wizard.saveCreds": "Save credentials",
  "offsite.wizard.credLoadError": "Could not load existing credentials — reload before editing.",
  "offsite.wizard.step4": "4 · Enable append-only protection",
  "offsite.immutable": "Immutable (append-only)",
  "offsite.immutableHint": "BombVault stops pruning/deleting off-site and lets the far side enforce retention. The far side must actually refuse deletes — verified below.",
  "offsite.rcloneWarning": "rclone serve restic --append-only has an open upstream retry bug that can drop appends. rest-server is recommended for immutable off-site.",
  "offsite.s3Unverified": "Note: S3 append-only can't be verified automatically. Set bucket versioning + deny-delete manually; the scorecard keeps this domain marked unverified.",
  "offsite.tamperTestNow": "Test append-only now",
  "offsite.tamperTesting": "Testing…",
  "offsite.tamperOk": "✓ delete refused — append-only active",
  "offsite.tamperFail": "✗ server ACCEPTED the delete — NOT protected",
  "offsite.tamperUnverifiable": "not verifiable for this repo type",
  "offsite.tamperError": "Tamper test inconclusive (server unreachable)",
  "offsite.retention.title": "5 · Retention strategy",
  "offsite.retention.farside": "Far-side prune (recommended)",
  "offsite.retention.window": "Maintenance window",
  "offsite.retention.grow": "Grow + budget alarm",
  "offsite.retention.farsideHint": "Run restic forget --prune on the storage box itself (BombVault stays append-only). Cron hint:",
  "offsite.retention.windowHint": "Temporarily run a second, non-append-only rest-server, prune, then shut it down — credentials are never persisted and a mandatory tamper re-test follows. Use only when far-side prune is not possible.",
  "offsite.retention.growHint": "Never prune off-site; instead alarm when the repo grows past a byte budget. The honest default until you pick a prune path.",
  "offsite.retention.budget": "Growth budget (GB, 0 = off)",
  "offsite.retention.saveBudget": "Save budget",
  "flash.downloading": "Downloading…",
  "settings.domains": "Domains",
  "settings.containersEnabled": "Containers",
  "settings.vmsEnabled": "VMs",
  "settings.flashEnabled": "Flash",
  "settings.configEnabled": "App configuration",
  "settings.schedule": "Schedule",
  "settings.scheduleOff": "off",
  "settings.language": "Language",
  "settings.save": "Save",
  "settings.saved": "Settings saved",
  "settings.error": "Error saving settings",

  // Retention
  "settings.retentionTitle": "Retention",
  "settings.retentionHint": "How many backups to keep per item. After each backup, restic prunes older snapshots to this policy. All zero = keep everything (off).",
  "settings.retentionLast": "Keep last",
  "settings.retentionDaily": "Keep daily",
  "settings.retentionWeekly": "Keep weekly",
  "settings.retentionMonthly": "Keep monthly",
  "settings.retentionLocal": "Local repo",
  "settings.retentionOffsite": "Off-site repo",
  "settings.retentionOffsiteHint": "A separate policy for the off-site repo, so you can keep it longer as an archive. All zero = keep every off-site backup (no off-site pruning).",

  // Off-site bandwidth
  "settings.offsiteLimits": "Off-site bandwidth",
  "settings.limitUpload": "Upload limit (KiB/s)",
  "settings.limitDownload": "Download limit (KiB/s)",
  "settings.limitHint": "0 = unlimited. Caps restic's off-site transfer rate.",

  // Monitoring (Prometheus)
  "settings.metrics": "Monitoring (Prometheus)",
  "settings.metricsEnable": "Expose /metrics",
  "settings.metricsToken": "Scrape token (optional)",
  "settings.metricsHint": "Prometheus-format metrics at /metrics for Grafana/Uptime Kuma. If a token is set, scrape with Authorization: Bearer <token>.",

  // Off-site (rclone)
  "rclone.title": "Off-site (rclone)",
  "rclone.hint": "Paste an rclone config to back up to the cloud (Backblaze B2, S3, Google Drive, …). It is stored encrypted. SMB/NFS need no rclone: mount the share on Unraid and set a Backup Path to it.",
  "rclone.configured": "Configured remotes",
  "rclone.pathHint": "Then set a Backup Path to \"rclone:<remote>:<bucket>/path\" to send that domain off-site.",
  "cloud.title": "Cloud credentials (S3 / restic REST)",
  "cloud.hint": "Credentials for off-site restic backends, without rclone. After saving, set a Backup Path to a remote repo, e.g. s3:s3.amazonaws.com/bucket/path, rest:http://host:8000/repo, b2:bucket:path or sftp:user@host:/repo. Secrets are stored encrypted and never shown again.",
  "cloud.secretSet": "saved — leave blank to keep",
  "rclone.save": "Save config",
  "notify.title": "Notifications",
  "notify.hint": "Get notified when a backup finishes. Set up any of the channels below; they all fire according to the policy.",
  "notify.on": "Notify",
  "notify.onNever": "Never",
  "notify.onFailure": "Only on failure",
  "notify.onAlways": "On success and failure",
  "notify.webhook": "Webhook URL",
  "notify.webhookFormat": "Webhook format",
  "notify.matrix": "Matrix",
  "notify.matrixHomeserver": "Homeserver URL",
  "notify.matrixToken": "Access token",
  "notify.matrixRoom": "Room ID",
  "notify.healthchecks": "Healthchecks.io ping URL",
  "notify.healthchecksLifecycle": "Healthchecks is pinged for the whole backup lifecycle — start, success and failure — whenever a URL is set, independent of the 'notify on' setting above, so the check stays green on success even with failure-only notifications.",
  "notify.hcPerDomain": "Per-domain checks (advanced)",
  "notify.hcPerDomainHint": "Leave a field blank to use the global URL above. A domain with its own URL gets its own check, with its own runtime and history.",
  "notify.unraid": "Unraid notifications",
  "notify.unraidHint": "Send to Unraid's own notification system (which can forward to Pushover, email, Discord, …). Needs the SSH connection set up (Settings → VM Backup over SSH).",
  "notify.smtp": "Email (SMTP)",
  "notify.smtpHost": "SMTP host",
  "notify.smtpPort": "Port",
  "notify.smtpUser": "Username",
  "notify.smtpPass": "Password",
  "notify.smtpFrom": "From address",
  "notify.smtpTo": "To address",
  "notify.smtpTls": "Encryption",
  "notify.save": "Save",
  "notify.test": "Send test",
  "notify.tested": "Test sent",

  // Integrity (restic check)
  "integrity.title": "Integrity & maintenance",
  "integrity.hint": "Verify a repository's structure (restic check), clear stale locks left by an interrupted run, or prune to reclaim space from deleted backups.",
  "integrity.verify": "Verify",
  "integrity.checking": "Checking…",
  "integrity.ok": "✓ Healthy",
  "integrity.failed": "Check failed",
  "integrity.unlock": "Unlock",
  "integrity.prune": "Prune",
  "integrity.verifyHint": "Run restic check to verify structure and metadata are intact.",
  "integrity.unlockHint": "Clear stale repository locks left by a crashed or interrupted run (fixes 'repository is already locked').",
  "integrity.pruneHint": "Apply your retention policy and reclaim space (reclaims space only when no policy is set; can take a while).",
  "integrity.pruneConfirm": "Prune now applies your retention policy — it removes snapshots beyond your keep rules (last/daily/weekly/monthly) and reclaims space. With no policy set it only reclaims space. Continue?",

  // Restore-verification drills ("verify restorability")
  "verify.now": "Verify restorability",
  "verify.running": "Verifying…",
  "verify.ok": "Verified restorable",
  "verify.failed": "Verification failed",
  "verify.last": "Last verified {time}",
  "verify.never": "Never verified",
  "verify.auto": "Automatic restore checks",
  "verify.subsetPct": "Data sample (%)",
  "verify.hint": "Periodically reads a random sample of backup data to prove it is intact and restorable.",
  "verify.shield": "verified",

  // DR drill controls (real off-site restore) + the off-site restorability badge
  "drill.kindLabel": "Drill type:",
  "drill.kindSubset": "Integrity check",
  "drill.kindDR": "Real restore (off-site)",
  "drill.target": "Drill target (container)",
  "drill.targetMostRecent": "Most recent backup",
  "drill.drNote": "A real restore extracts the newest off-site snapshot into a temporary sandbox, verifies it, then cleans up. It downloads real data and can take a while.",
  "drill.drVMsNote": "Real restore isn't available for VMs — their disk images are too large to sandbox-restore. Use the integrity check instead.",
  "drill.runDR": "Run real restore",
  "drill.runningDR": "Restoring…",
  "drill.confirmDR": "This performs a REAL restore of the newest off-site snapshot into a temporary sandbox to prove it is recoverable, then deletes it. It downloads real data and can take a while. Continue?",
  "drill.provenOffsite": "proven restorable from off-site",

  // Pre/post-backup hooks
  "hooks.title": "Backup hooks",
  "hooks.hint": "Commands run inside the container with sh -c. The pre-command runs before the backup; use it to prepare data that should be backed up, for example dumping a database into the container's appdata. If the pre-command fails, the backup is aborted. The post-command runs after the container is started again and its failure is only logged. Hooks only run commands, they do not add extra folders to the backup.",
  "hooks.pre": "Pre-backup command",
  "hooks.post": "Post-backup command",
  "folders.title": "Backup folders",
  "folders.hint": "Choose which of this container's mapped folders to back up. The appdata folder is selected by default. Tick others to include them, or add a custom path under the host mount. Unticking everything reverts to the automatic appdata default.",
  "folders.appdataDefault": "appdata (default)",
  "folders.notReachable": "not under the host mount, can't be backed up",
  "folders.customPlaceholder": "/mnt/user/some/folder",
  "folders.addCustom": "Add a folder path",
  "folders.add": "Add",
  "folders.save": "Save folders",
  "folders.saved": "Saved",
  "folders.empty": "No mapped folders found for this container.",
  "stophook.title": "Stop other containers",
  "stophook.hint": "Stop these other containers while this one is backed up (for example a database), then start them again afterwards. One container name per line.",
  "export.button": "Export (plain tar)",
  "export.exportedTo": "Exported to:",
  "backup.configOnly": "Config only — no data folders (definition saved for recreate)",

  // Appearance / Accent
  "settings.appearance": "Appearance",
  "settings.accentColor": "Accent color",
  "settings.accentPresets": "Presets",

  // Dashboard stat cards
  "dashboard.statContainers": "Containers",
  "dashboard.statVMs": "VMs",
  "dashboard.statActiveJobs": "Active jobs",
  "dashboard.statPausedJobs": "Paused jobs",
  "dashboard.statErrors": "Errors",
  "dashboard.statMissingContainers": "Missing containers",
  "dashboard.statMissingVMs": "Missing VMs",

  // Dashboard protection (RPO) status
  "dashboard.protectionTitle": "Protection status",
  "dashboard.rpoOk": "Up to date",
  "dashboard.rpoWarn": "Due soon",
  "dashboard.rpoOverdue": "Overdue",
  "dashboard.rpoNever": "No backup yet",
  "dashboard.rpoOff": "Not scheduled",
  "dashboard.domainContainers": "Containers",
  "dashboard.domainVMs": "VMs",
  "dashboard.domainFlash": "Flash",
  "dashboard.domainConfig": "Config",

  // Dashboard ransomware-protection card (v4)
  "ransomware.title": "Ransomware protection",
  "ransomware.protGreen": "Protected",
  "ransomware.protAmber": "Needs attention",
  "ransomware.protRed": "At risk",
  "ransomware.configured": "off-site configured",
  "ransomware.appendOnlyVerified": "append-only verified",
  "ransomware.appendOnlyStale": "append-only proof stale",
  "ransomware.appendOnlyFailed": "append-only test failed",
  "ransomware.appendOnlyNever": "append-only not proven yet",
  "ransomware.appendOnlyOff": "append-only not enabled",
  "ransomware.replicationCurrent": "replication current",
  "ransomware.replicationOverdue": "replication overdue",
  "ransomware.replicationNever": "not replicated yet",
  "ransomware.drillOffsite": "restore drill (off-site)",
  "ransomware.drillOverdue": "restore drill overdue",
  "ransomware.drillNever": "no restore drill yet",
  "ransomware.encryptionOn": "encryption on",
  "ransomware.pruneStrategy": "prune strategy set",

  // Dashboard backup-health heatmap
  "dashboard.healthTitle": "Backup health",
  "dashboard.heatLess": "Less",
  "dashboard.heatMore": "More",

  // Dashboard storage (repo size + dedup) card
  "dashboard.storageTitle": "Storage",
  "dashboard.dedup": "Dedup",
  "dashboard.snapshotsLabel": "Snapshots",
  "dashboard.noStats": "No data yet",

  // Jobs page
  "nav.jobs": "Plans",
  "jobs.title": "Plans",
  "jobs.subtitle": "Backup plans by domain",
  "jobs.configureInSettings": "Configure schedules in Settings",
  "jobs.containersSection": "Containers",
  "jobs.vmsSection": "VMs",
  "jobs.flashSection": "Flash",
  "jobs.active": "Active",
  "jobs.paused": "Paused",
  "jobs.notScheduled": "Not scheduled",
  "jobs.cadenceDaily": "Daily at {time}",
  "jobs.cadenceWeekly": "Weekly ({days}) at {time}",
  "jobs.cadenceEveryN": "Every {n} days at {time}",
  "sort.label": "Sort:",
  "sort.nameAsc": "Name (A–Z)",
  "sort.status": "Status",
  "sort.ip": "IP",
  "cadence.off": "Off",
  "cadence.daily": "Daily",
  "cadence.weekly": "Weekly",
  "cadence.everyN": "Every N days",
  "cadence.time": "Time",
  "cadence.days": "Days",
  "cadence.every": "Every",
  "cadence.daysUnit": "days",
  "cadence.fmtDaily": "daily at {time}",
  "cadence.fmtWeekly": "weekly ({days}) at {time}",
  "cadence.fmtEveryN": "every {n} days at {time}",
  "time.justNow": "just now",
  "time.minutesAgo": "{n}m ago",
  "time.hoursAgo": "{n}h ago",
  "time.daysAgo": "{n}d ago",
  "folder.browse": "Browse…",
  "folder.browseTitle": "Browse folders",
  "folder.use": "Use this folder",
  "folder.none": "No subdirectories",
  "folder.loading": "Loading…",
  "folder.pathHint": "Path must be a relative subpath (no leading / or ..)",
  "folder.couldNotRead": "Could not read directory",
  "folder.browseFailed": "Browse failed",
  "common.reset": "Reset",
  "containers.subtitle": "Manage container backups, schedules, and restores.",
  "containers.emptyDocker": "No containers found. Is Docker running?",
  "containers.bulkResult": "{ok} ok, {fail} failed",
  "vm.method.saveFailed": "Couldn't change the backup method — it was not switched.",
  "jobs.noVMs": "No VMs yet",
  "jobs.noContainersIncluded": "No containers included in schedule.",
  "jobs.flashRow": "Unraid flash config",
  "jobs.flashPlanned": "planned",
  "jobs.vmPlanned": "VM backup executor not yet implemented.",
  "jobs.syncSchedules": "Use the Containers schedule for VMs and Flash too",
  "jobs.vmIncludeHint": "Backs up every VM with “include in schedule” enabled (set it per VM in the VMs tab).",
  "jobs.flashNotImplemented": "Note: Flash backup executor is not yet implemented in Phase 1 — schedule is stored but not executed.",
  "schedule.includeAll": "Include all in schedule",
  "schedule.excludeAll": "Exclude all from schedule",

  // Auth / Login
  "auth.loginTitle": "BombVault",
  "auth.passwordLabel": "Password",
  "auth.signIn": "Sign in",
  "auth.signingIn": "Signing in…",
  "auth.invalidPassword": "Invalid password",
  "auth.loginError": "Login failed",

  // Settings — Security card
  "auth.security": "Security",
  "auth.authOff": "Authentication is off — all LAN users have full access.",
  "auth.authOn": "Authentication is enabled.",
  "auth.setPassword": "Set password",
  "auth.changePassword": "Change password",
  "auth.confirmPassword": "Confirm password",
  "auth.passwordMismatch": "Passwords do not match",
  "auth.passwordSaved": "Password saved",
  "auth.passwordCleared": "Authentication disabled",
  "auth.passwordHint":
    "Leave both fields empty to disable authentication. BombVault has root-equivalent host control — a password is recommended if this instance is reachable by untrusted LAN users.",
  "auth.logout": "Sign out",
  "auth.saving": "Saving…",
  "auth.saveError": "Failed to save",

  // Common action labels (shared across container / VM / settings buttons)
  "common.backingUp": "Backing up…",
  "common.restoring": "Restoring…",
  "common.done": "Done",
  "common.close": "Close",
  "common.loadingBackups": "Loading backups…",
  "common.saving": "Saving…",
  "common.restoreRunning": "A restore is running…",
  "common.backupRunning": "A backup is running…",
  "common.replicateRunning": "A replication is running…",

  // VMs page
  "vms.title": "Virtual Machines",
  "vms.subtitle": "Manage VM backups, schedules, and restores.",
  "vms.empty": "No VMs found. Is libvirt/KVM running?",
  "vms.backupSelected": "Back up selected",
  "vms.restoreSelected": "Restore selected (latest)",
  "vms.restoreSelectedConfirm": "Restore the LATEST backup of the selected VMs? Each VM is shut off, its disk files replaced, and the VM restored.",
  "vms.notInstalledHint": "These VMs are no longer defined on the host but still have backups. Restore them to recover, or use the Backups panel to browse their snapshots.",
  "vms.removeEntry": "Remove entry",
  "vms.removeEntryConfirm": "Remove this VM's entry from the list? Its backups, if any, are not deleted.",
  "vms.discoverHint": "VM deleted from Unraid (or lost after a reinstall)? Rebuild its backup entry from storage so you can restore it.",

  // Flash (Unraid USB) backup
  "flash.title": "Flash Backup",
  "flash.subtitle": "Back up and restore the Unraid USB flash (the whole /boot).",
  "flash.backupTitle": "Back up the flash",
  "flash.backupHint": "Captures the entire USB flash (/boot): Unraid OS, license, array config, shares, network and plugin config.",
  "flash.backupNow": "Back up flash now",
  "flash.backingUp": "Backing up…",
  "flash.download": "Download (.zip)",
  "flash.restoreNote": "Restore downloads a ZIP of the snapshot — the running /boot is never touched. Drop the .zip straight into the Unraid USB creator, or unzip it onto a fresh USB to rebuild your flash.",
  "flash.none": "No flash backups yet — run a backup above.",
  // Scheduled flash zip export (#28): a plain .zip written to a folder after each flash backup.
  "flash.zipExport.title": "Flash zip export",
  "flash.zipExport.hint": "After each flash backup, also write the snapshot out as a plain .zip to a folder — ready for off-server sync (Syncthing, rclone, a cloud drive).",
  "flash.zipExport.enable": "Export a zip after each flash backup",
  "flash.zipExport.enableHint": "Every time a flash backup succeeds, the snapshot is written as a .zip to the folder below.",
  "flash.zipExport.path": "Export folder",
  "flash.zipExport.pathHint": "Relative subpath under the host mount root where the .zip lands — point it at a Syncthing/rclone folder to get the flash off the server automatically.",
  "flash.zipExport.keepHistory": "Keep history",
  "flash.zipExport.keepHistoryHint": "Off: keep a single flash-latest.zip that's overwritten each time. On: keep the newest N timestamped flash-<date>.zip files.",
  "flash.zipExport.keepN": "Zips to keep",
  "flash.zipExport.keepNHint": "The newest N timestamped zips are kept; older ones are deleted automatically.",
  "flash.zipExport.latestNote": "A single flash-latest.zip is overwritten after every backup.",
  "flash.zipExport.plaintextWarn": "The exported .zip is not encrypted, even if your flash repository is. Only sync it somewhere you trust.",
  "flash.zipExport.pathRequired": "Choose an export folder to turn this on.",

  // Config self-backup (BombVault's own settings). Minimal en/de set for Task 12;
  // the full 24-locale translation lands in Task 14.
  "config.title": "Config Backup",
  "config.subtitle": "Back up BombVault's own settings so a rebuilt server can restore itself.",
  "config.settingsTitle": "Config backup settings",
  "config.settingsHint": "Protect BombVault's own configuration — its settings database, off-site credentials and SSH keys — so a fresh install can restore itself and pick up right where it left off.",
  "config.enabled": "Back up BombVault's settings",
  "config.enabledHint": "Include BombVault's own /config in the schedule below.",
  "config.path": "Backup location",
  "config.pathHint": "Relative subpath under the host mount root where the config repo is written.",
  "config.schedule": "Schedule",
  "config.schedulePlaceholder": "off · e.g. daily 03:30",
  "config.scheduleHint": "When to automatically back up the settings. Leave 'off' to only back up on demand.",
  "config.offsite": "Off-site repo (optional)",
  "config.offsiteHint": "Replicate the config backup to a second, off-site repo after each local backup.",
  "config.offsiteSchedule": "Off-site schedule",
  "config.immutable": "Off-site repo is append-only (immutable)",
  "config.immutableHint": "Skip off-site pruning and refuse off-site deletes — the far side (append-only) enforces it.",
  "config.backupTitle": "Back up settings now",
  "config.backupHint": "Captures BombVault's own /config: the settings database, off-site credentials (rclone.conf) and SSH keypair.",
  "config.backupNow": "Back up settings now",
  "config.backingUp": "Backing up…",
  "config.snapshotsTitle": "Settings backups",
  "config.snapshotsHint": "To restore these settings onto a rebuilt server, use the Recovery tab — restoring settings restarts BombVault to apply them, so it lives there with the rest of the disaster-recovery flow.",
  "config.none": "No settings backups yet — run a backup above.",

  // Container / VM state badge labels
  "state.created":      "Created",
  "state.running":      "Running",
  "state.paused":       "Paused",
  "state.restarting":   "Restarting",
  "state.removing":     "Removing",
  "state.exited":       "Exited",
  "state.dead":         "Dead",
  "state.shutoff":      "Shut off",
  "state.inshutdown":   "Shutting down",
  "state.crashed":      "Crashed",
  "state.pmsuspended":  "Suspended",
  "state.notInstalled": "Not installed",

  // VM backup (SSH)
  "vm.method": "Method",
  "vm.method.graceful": "Graceful (shutdown)",
  "vm.method.live": "Live snapshot",
  "vm.method.hint": "Graceful shuts the VM down during the backup; Live keeps it running (snapshot, no downtime).",
  "vm.ssh.title": "VM Backup over SSH",
  "vm.ssh.desc": "VM backup reaches libvirt over SSH (no mount). Authorize this key on Unraid, then test.",
  "vm.ssh.host": "Host",
  "vm.ssh.publicKey": "Public key — append to Unraid /root/.ssh/authorized_keys",
  "vm.ssh.copy": "Copy",
  "vm.ssh.copied": "Copied",
  "vm.ssh.test": "Test connection",
  "vm.ssh.testing": "Testing…",
  "vm.ssh.testOk": "Connected — libvirt reachable",
  "vm.ssh.testFail": "Connection failed",
  "vm.ssh.setupTitle": "Set up (one time)",
  "vm.ssh.step1": "Copy the command below and run it in the Unraid terminal to authorize this key (it survives reboots).",
  "vm.ssh.step2": "Set the container's “VM Backup: Host” variable to your Unraid LAN IP (e.g. 192.168.x.x); on simple bridge networking host.docker.internal also works.",
  "vm.ssh.step3": "Click Test connection — once it's green, enable VMs under Domains.",
  "vm.ssh.copyCmd": "Copy command",
  "vm.ssh.guide": "Full setup & networking guide",

  // Guided Recovery tab (disaster-recovery walkthrough) — note: the `recovery.*`
  // prefix above is the encryption *kit*; the page title uses `recovery.pageTitle`
  // to avoid colliding with the existing `recovery.title` ("Recovery kit").
  "nav.recovery": "Recovery",
  "recovery.pageTitle": "Disaster recovery",
  "recovery.intro": "Recover your containers and VMs from an existing backup onto this install. Point BombVault at your backups, discover what's in them, and restore.",
  // Step 1 — connection / APP_KEY readability check
  "recovery.step1": "Can BombVault read your backups?",
  "recovery.appKeyExplain": "To read existing backups this container needs the SAME APP_KEY it used before — it's in your recovery kit. Set it in the Unraid container template if it isn't already, then re-check.",
  "recovery.appKeyRemedy": "The encryption key doesn't match these backups. Set the original APP_KEY (from your recovery kit) in the container template, then re-check.",
  "recovery.readable": "Your backups are readable.",
  "recovery.notReachable": "Couldn't reach your backups yet — attach the location below, then re-check.",
  "recovery.recheck": "Re-check",
  // Step 2 — restore BombVault's own settings first (optional, before attach)
  "recovery.stepConfig": "Restore BombVault's own settings",
  "recovery.configHint": "On a rebuilt server, restore BombVault's own settings first — its backup paths, off-site targets and credentials — so the steps below come pre-filled. Point it at the settings backup you set up earlier. No settings backup? Skip this and attach your backups manually below.",
  "recovery.configAppKeyReminder": "Your APP_KEY must match this backup — that's the check in Step 1 above.",
  "recovery.configSourceLabel": "Where is the settings backup?",
  "recovery.configLocalPath": "Local path",
  "recovery.configOffsiteUrl": "Off-site repo URL",
  "recovery.configRestore": "Restore BombVault's settings",
  "recovery.configRestoring": "Restoring…",
  "recovery.configRestarting": "BombVault is restarting to apply your settings… this page reloads automatically when it's back.",
  "recovery.configManualRestart": "Your settings are staged. Restart the BombVault container in Unraid, then continue — they apply on the next boot.",
  "recovery.configReloadWhenBack": "BombVault is taking longer than expected to come back. Reload this page once it's up to load your restored settings.",
  "recovery.configReload": "Reload now",
  "recovery.configSkip": "Skip — I don't have a settings backup",
  "recovery.configSkipped": "Skipped. Attach your backups manually below.",
  // Step 3 — attach your backups
  "recovery.step2": "Attach your backups",
  "recovery.attachHint": "Point BombVault at your existing backups: a local path under the host mount, or an off-site repo (rest / S3 / B2 / sftp / rclone) with its credentials. Then connect to confirm.",
  "recovery.credsSaveHint": "Off-site credentials save with each card's own Save button — save them before you connect & preview.",
  "recovery.connectPreview": "Connect & preview",
  // Step 3 — discover everything
  "recovery.step3": "Discover what's in your backups",
  "recovery.discover": "Discover backups",
  "recovery.foundCounts": "Found {c} containers and {v} VMs.",
  "recovery.foundNone": "Nothing found yet — check the connection and attachment above. If you expected backups here, make sure your APP_KEY matches these backups.",
  // Step 4 — review & restore all (left stopped)
  "recovery.step4": "Review and restore",
  "recovery.restoreAll": "Restore all (left stopped)",
  "recovery.restoreAllResult": "Restored {ok}, failed {fail}. Start them from the Containers/VMs tabs when ready.",
  "recovery.vmSshNote": "VM restore needs the libvirt SSH link — set it up under Settings → VM Backup over SSH.",
  "recovery.noneDiscovered": "Run Discover above first.",
  // Step 5 — recovery kit (safety net for next time)
  "recovery.step5": "Your recovery kit",
  "recovery.kitHint": "Download and store your recovery kit somewhere safe — it holds the encryption key and the exact restic commands to restore even without BombVault.",
  "recovery.kitDownload": "Download recovery kit",
  // Dashboard fresh-install nudge → guided Recovery tab
  "recovery.freshNudge": "Restoring from a previous server or a rebuild? Recover your existing backups.",
  "recovery.freshNudgeCta": "Go to Recovery",
} as const;

export type TranslationKey = keyof typeof en;
export type Translations = Record<TranslationKey, string>;

// ---------------------------------------------------------------------------
// German locale (full)
// ---------------------------------------------------------------------------

const de: Translations = {
  "language.label": "Sprache",
  "theme.toggle": "Design umschalten",
  "theme.dark": "Dunkel",
  "theme.light": "Hell",

  "nav.dashboard": "Dashboard",
  "nav.containers": "Container",
  "nav.vms": "VMs",
  "nav.flash": "Flash",
  "nav.config": "Config",
  "nav.settings": "Einstellungen",
  "nav.reportBug": "Fehler melden",
  "nav.advanced": "Erweitert",
  "nav.comingSoon": "Demnächst",

  "dashboard.title": "Dashboard",
  "dashboard.lastBackups": "Letzte Backups",
  "dashboard.recentRuns": "Letzte Ausführungen",
  "dashboard.spikeStatus": "Systemstatus",
  "dashboard.noRuns": "Noch keine Ausführungen",
  "dashboard.spikeLink": "Host-Integration prüfen",
  "dashboard.hostIntegrationCheck": "Host-Integration-Check",
  "dashboard.allOk": "Alle Systeme OK",
  "dashboard.degraded": "Eingeschränkt",
  "dashboard.checking": "Prüfe…",

  "spike.title": "Host-Integration",
  "spike.overall": "Gesamt:",
  "spike.allOk": "ALLES OK",
  "spike.degraded": "EINGESCHRÄNKT",
  "spike.colCheck": "Prüfung",
  "spike.colStatus": "Status",
  "spike.colDetail": "Detail",
  "spike.ok": "OK",
  "spike.fail": "FEHLER",
  "spike.bestEffort": "optional",
  "spike.checkNow": "Jetzt prüfen",
  "spike.probeFailed": "Prüfung fehlgeschlagen (siehe Server-Logs)",

  "containers.title": "Container",
  "containers.discover": "Backups entdecken",
  "containers.discovering": "Suche…",
  "containers.discoverHint": "/config verloren? Backup-Liste aus dem Speicher wiederherstellen.",
  "containers.backupNow": "Jetzt sichern",
  "containers.lastBackup": "Letztes Backup",
  "containers.never": "Nie",
  "containers.colName": "Name",
  "containers.colImage": "Image",
  "containers.colStatus": "Status",
  "containers.colAppdata": "Appdata",
  "containers.colActions": "Aktionen",
  "containers.backupStarted": "Backup gestartet",
  "containers.noDestination": "Kein Ziel konfiguriert",
  "containers.includeInSchedule": "Im Zeitplan einschließen",
  "containers.schedule": "Zeitplan",
  "containers.notInstalled": "Nicht installiert",
  "containers.notInstalledTitle": "Nicht installiert (nur Backups)",
  "containers.notInstalledHint": "Diese Container sind nicht mehr installiert, haben aber noch Backups. Stelle sie wieder her oder lösche ihre Backups, um Platz freizugeben.",
  "containers.deleteBackups": "Alle Backups löschen",
  "containers.deleteBackupsConfirm": "ALLE Backups dieses Containers löschen? Die Snapshots werden dauerhaft aus dem Repository entfernt und können nicht wiederhergestellt werden.",
  "containers.filter": "Filter:",
  "containers.filterAll": "Alle",
  "containers.filterInstalled": "Installiert",
  "containers.selectAll": "Alle auswählen",
  "containers.selectedCount": "ausgewählt",
  "containers.backupSelected": "Auswahl sichern",
  "containers.restoreSelected": "Auswahl wiederherstellen (neuestes)",
  "containers.restoreSelectedConfirm": "Das NEUESTE Backup der ausgewählten Container wiederherstellen? Jeder wird gestoppt, seine Appdata ersetzt und neu erstellt.",
  "containers.clearSelection": "Leeren",
  "containers.working": "Arbeite…",
  "containers.batchStarted": "Backup gestartet — es läuft auf dem Server, du kannst diesen Tab schließen.",
  "containers.batchAlreadyRunning": "Es läuft bereits ein Sammel-Backup.",
  "containers.batchRunning": "Sichere ausgewählte Container…",
  "containers.selfNote": "Das ist BombVault — es sichert seinen eigenen Container nicht (würde sich selbst stoppen); seine Einstellungen werden über Discover wiederhergestellt.",

  "snapshots.title": "Backups",
  "snapshots.colId": "ID",
  "snapshots.colTime": "Zeitpunkt",
  "snapshots.colTags": "Tags",
  "snapshots.colSize": "Größe",
  "snapshots.restore": "Wiederherstellen",
  "snapshots.none": "Keine Backups gefunden",
  "snapshots.files": "Dateien",
  "snapshots.delete": "Löschen",
  "snapshots.deleteConfirm": "Dieses Backup löschen? Der Snapshot wird aus dem Repository entfernt (zum Freigeben des Speichers in den Einstellungen „Aufräumen“ ausführen). Kann nicht rückgängig gemacht werden.",
  "snapshots.deleteAll": "Alle Backups löschen",
  "snapshots.deleteAllConfirm": "ALLE Backups dieser VM aus der gewählten Quelle (lokal oder Off-site) löschen? Die Snapshots werden dauerhaft entfernt und das Repository wird aufgeräumt. Kann nicht rückgängig gemacht werden.",
  "snapshots.deletingAll": "Wird gelöscht…",
  "snapshots.recreate": "Aus gespeicherter Konfig neu erstellen",
  "snapshots.recreateConfirm": "Diesen Container aus seiner gespeicherten Konfiguration neu erstellen? Er wird aus der gespeicherten Definition (Image, Env, Ports, Volumes) angelegt und gestartet.",
  "snapshots.configOnlyHint": "Nur-Konfig-Backup: die Definition des Containers ist gesichert, es gibt aber keine Datenordner zum Snapshotten. Wird der Container gelöscht, erscheint er unter „Nicht installiert“ und kann von dort aus dieser Konfig neu erstellt werden.",

  // Snapshot tags + compare (diff)
  "snapshot.tags": "Tags",
  "snapshot.addTag": "Tag hinzufügen",
  "snapshot.compare": "Vergleichen",
  "snapshot.pickTwo": "Zwei Snapshots zum Vergleichen wählen",
  "snapshot.added": "hinzugefügt",
  "snapshot.removed": "entfernt",
  "snapshot.changed": "geändert",
  "snapshot.diffSummary": "+{addedFiles} Dateien ({addedBytes}), ~{changedFiles} geändert, -{removedFiles} Dateien ({removedBytes})",

  // File-level restore
  "files.restore": "Wiederherstellen",
  "files.restored": "Wiederhergestellt",
  "files.restoreConfirm": "Ausgewählte Dateien an ihren Originalort wiederherstellen? Vorhandene Dateien werden überschrieben.",
  "files.filterPlaceholder": "Dateien filtern…",
  "files.none": "Keine passenden Dateien",
  "files.loadFailed": "Dateien konnten nicht geladen werden",
  "files.more": "Filter verfeinern, um mehr Dateien zu sehen.",
  "files.selectHint": "Dateien und Ordner ankreuzen, dann Ziel wählen.",
  "files.dest.inPlace": "Am Ursprungsort wiederherstellen",
  "files.dest.toFolder": "In einen Ordner wiederherstellen",
  "files.restoreSelected": "Auswahl wiederherstellen ({n})",
  "files.restoredInPlace": "Ausgewählte Dateien an ihren Originalort wiederhergestellt.",

  "restore.confirmTitle": "Wiederherstellung bestätigen",
  "restore.confirmBody":
    "Der Container wird gestoppt, seine Appdata ersetzt und aus dem Backup neu erstellt. Fortfahren?",
  "restore.confirm": "Bestätigen",
  "restore.cancel": "Wiederherstellung abbrechen",
  "restore.cancelConfirmSafe": "Wiederherstellung abbrechen? Der bereits geschriebene Zielordner bleibt unverändert erhalten.",
  "restore.cancelConfirmInPlace":
    "{name} wird gerade wiederhergestellt. Ein Abbruch lässt diese Wiederherstellung unvollständig zurück — möglicherweise musst du sie erneut ausführen. Trotzdem abbrechen?",
  "restore.cancelling": "Wird abgebrochen…",
  "restore.cancelled": "Wiederherstellung abgebrochen",
  "restore.preview": "Vorschau",
  "restore.started": "Wiederherstellung gestartet",
  "restore.toFolder": "In Ordner wiederherstellen…",
  "restore.toFolderHint":
    "Entpackt diesen Snapshot in einen Ordner unter deinem Backup-Mount. Der laufende Container wird nicht angetastet.",
  "restore.targetPath": "Zielordner",
  "restore.restoredTo": "Wiederhergestellt nach {path}",
  "restore.progress": "Wiederherstellen… {pct} %",
  "restore.open": "Wiederherstellen…",
  "restore.mode.inPlace": "Am Originalort wiederherstellen",
  "restore.mode.files": "Einzelne Dateien",
  "restore.mode.toFolder": "In einen Ordner",
  "restore.inPlaceHint": "Diesen Container exakt wie zuvor neu erstellen.",
  "restore.leaveStopped": "Nach dem Restore gestoppt lassen (nicht starten)",
  "restore.bgHint":
    "Läuft im Hintergrund — du kannst dieses Panel schließen; das Ergebnis erscheint im Ausführungsverlauf.",

  // Stacks (Compose-Projekt-Wiederherstellung)
  "stack.title": "Stacks",
  "stack.restore": "Stack wiederherstellen…",
  "stack.members": "{n} Container",
  "stack.restoreHint":
    "Stellt jeden Container dieses Stacks aus dem letzten Backup wieder her (gestoppt) und startet sie danach optional in Abhängigkeitsreihenfolge.",
  "stack.startInOrder": "Nach dem Restore in Abhängigkeitsreihenfolge starten",
  "stack.restoreConfirm":
    "Alle Container dieses Stacks wiederherstellen? Jeder wird aus seinem letzten Backup neu erstellt.",
  "stack.restoring": "Stack wird wiederhergestellt…",
  "stack.restored": "Stack wiederhergestellt",
  "stack.restoreFinished": "Stack-Wiederherstellung abgeschlossen — Ergebnisse je Container im Verlauf.",
  "stack.memberRestored": "wiederhergestellt",
  "stack.memberStarted": "gestartet",

  "run.kindBackup": "Backup",
  "run.kindRestore": "Wiederherstellung",
  "run.statusRunning": "Läuft",
  "run.statusSuccess": "Erfolgreich",
  "run.statusFailed": "Fehlgeschlagen",
  "run.historyTitle": "Ausführungsverlauf",
  "run.filterDay": "Tag:",
  "run.allDays": "Alle Tage",
  "run.colKind": "Art",
  "run.colStatus": "Status",
  "run.colStarted": "Gestartet",
  "run.colFinished": "Abgeschlossen",
  "run.colContainer": "Container",

  "settings.title": "Einstellungen",
  "settings.encryption": "Verschlüsselung",
  "settings.encryptionOn": "Aktiviert (Passwort aus APP_KEY)",
  "settings.encryptionOff": "Deaktiviert (kein Passwort)",
  "settings.encryptionWarning":
    "Die Verschlüsselung ist beim Initialisieren des Repositorys festgelegt. Eine Änderung erfordert einen neuen leeren Pfad.",

  // Encryption-key recovery kit
  "recovery.title": "Wiederherstellungs-Kit",
  "recovery.download": "Recovery-Kit herunterladen",
  "recovery.why":
    "Mit aktivierter Verschlüsselung ist dein APP_KEY das Master-Geheimnis für jedes Backup. Lade ein Recovery-Kit herunter (den Schlüssel, das abgeleitete restic-Passwort, die Repo-Pfade und manuelle Wiederherstellungsschritte), damit du auch ohne laufenden BombVault-Container wiederherstellen kannst. Bewahre es sicher und offline auf.",
  "recovery.nagTitle": "Sichere dein Recovery-Kit",
  "recovery.nagBody":
    "Mit aktivierter Verschlüsselung bedeutet ein verlorener APP_KEY verlorene Backups. Lade das Recovery-Kit herunter und bewahre es sicher und offline auf.",
  "recovery.stored": "Sicher aufbewahrt",

  "settings.paths": "Backup-Pfade",
  "settings.containersPath": "Container-Pfad",
  "settings.vmsPath": "VMs-Pfad",
  "settings.flashPath": "Flash-Pfad",
  "settings.restoreFolder": "Standard-Restore-Ordner",
  "settings.restoreFolderHint": "Wohin 'in einen Ordner wiederherstellen' Snapshots standardmäßig entpackt.",
  "settings.offsiteTitle": "Offsite-Kopie (optional)",
  "settings.offsiteHint": "Nach jedem erfolgreichen lokalen Backup wird es zusätzlich per restic copy in ein zweites Repo repliziert. Ein Remote (rest:http://host:8000/repo, s3:…, b2:…) oder einen lokalen Unterpfad angeben; leer lassen zum Deaktivieren. Das lokale Backup bleibt primär.",
  "source.label": "Quelle:",
  "source.local": "Lokal",
  "source.offsite": "Offsite",
  "source.hint": "Restore und Löschen wirken nur auf die gewählte Quelle — ein lokales Backup zu löschen rührt die Offsite-Kopie nie an und umgekehrt.",
  "offsite.schedulePlaceholder": "leer = nach jedem Backup · z.B. weekly Sun 03:00",
  "offsite.replicateNow": "Jetzt replizieren",
  "offsite.replicating": "Repliziere…",
  "offsite.replicateFailed": "Replikation fehlgeschlagen",
  "offsite.test": "Verbindung testen",
  "offsite.testing": "Teste…",
  "offsite.testOk": "erreichbar + initialisiert",
  "offsite.testUninitialized": "erreichbar, nicht initialisiert",
  "offsite.testFailed": "nicht erreichbar",
  // Off-site-Einrichtungsassistent (v4 Ransomware-Schutz)
  "offsite.wizard.setup": "Einrichten…",
  "offsite.wizard.close": "Schließen",
  "offsite.wizard.step1": "1 · Backend wählen",
  "offsite.wizard.backendRest": "rest-server (empfohlen — append-only-fähig)",
  "offsite.wizard.backendRclone": "rclone-Remote",
  "offsite.wizard.backendS3": "Amazon S3 / S3-kompatibel",
  "offsite.wizard.step2": "2 · Append-only-Server bereitstellen",
  "offsite.wizard.step2Hint": "Auf deiner Storage-Box ausführen, um einen restic rest-server mit --append-only zu starten. Das erzeugte Passwort wird nur einmal angezeigt.",
  "offsite.wizard.generate": "Deployment-Snippet erzeugen",
  "offsite.wizard.regenerate": "Neu erzeugen (neues Passwort)",
  "offsite.wizard.snippetError": "Snippet konnte nicht erzeugt werden",
  "offsite.wizard.passwordWarning": "Dieses Passwort wird nur EINMAL angezeigt und von BombVault nicht gespeichert. Jetzt sichern — es wird für die Zugangsdaten unten benötigt und kann nicht wiederhergestellt werden.",
  "offsite.wizard.tlsNote": "Dieses Rezept nutzt einfaches HTTP — auf einem vertrauenswürdigen LAN oder VPN unproblematisch. Ist die Storage-Box über das Internet erreichbar, stelle rest-server hinter HTTPS (einen TLS-Reverse-Proxy), damit die Repository-Zugangsdaten nicht im Klartext übertragen werden.",
  "offsite.wizard.password": "Erzeugtes Passwort (sichern)",
  "offsite.wizard.step3": "3 · Repository-URL + Zugangsdaten",
  "offsite.wizard.repoUrl": "Off-site-Repository-URL",
  "offsite.wizard.repoUrlPlaceholder": "rest:http://192.168.x.x:8000/bombvault-containers/containers",
  "offsite.wizard.saveRepo": "Repository speichern",
  "offsite.wizard.credentials": "REST-Server-Zugangsdaten",
  "offsite.wizard.saveCreds": "Zugangsdaten speichern",
  "offsite.wizard.credLoadError": "Vorhandene Zugangsdaten konnten nicht geladen werden — vor dem Bearbeiten neu laden.",
  "offsite.wizard.step4": "4 · Append-only-Schutz aktivieren",
  "offsite.immutable": "Unveränderlich (append-only)",
  "offsite.immutableHint": "BombVault prunet/löscht off-site nicht mehr und überlässt die Aufbewahrung der Gegenseite. Die Gegenseite muss Löschungen wirklich verweigern — unten verifiziert.",
  "offsite.rcloneWarning": "rclone serve restic --append-only hat einen offenen Upstream-Retry-Bug, der Appends verlieren kann. Für unveränderliches Off-site wird rest-server empfohlen.",
  "offsite.s3Unverified": "Hinweis: S3-Append-only lässt sich nicht automatisch verifizieren. Bucket-Versionierung + Delete-Verbot manuell setzen; die Scorecard führt diese Domäne weiterhin als unverifiziert.",
  "offsite.tamperTestNow": "Append-only jetzt testen",
  "offsite.tamperTesting": "Teste…",
  "offsite.tamperOk": "✓ delete refused — append-only active",
  "offsite.tamperFail": "✗ server ACCEPTED the delete — NOT protected",
  "offsite.tamperUnverifiable": "not verifiable for this repo type",
  "offsite.tamperError": "Tamper-Test nicht eindeutig (Server nicht erreichbar)",
  "offsite.retention.title": "5 · Aufbewahrungsstrategie",
  "offsite.retention.farside": "Prune auf der Gegenseite (empfohlen)",
  "offsite.retention.window": "Wartungsfenster",
  "offsite.retention.grow": "Wachsen + Budget-Alarm",
  "offsite.retention.farsideHint": "restic forget --prune auf der Storage-Box selbst ausführen (BombVault bleibt append-only). Cron-Hinweis:",
  "offsite.retention.windowHint": "Kurzzeitig einen zweiten, nicht-append-only rest-server starten, prunen und wieder herunterfahren — Zugangsdaten werden nie gespeichert und ein Pflicht-Tamper-Retest folgt. Nur wenn Prune auf der Gegenseite nicht möglich ist.",
  "offsite.retention.growHint": "Off-site nie prunen; stattdessen alarmieren, wenn das Repo ein Byte-Budget überschreitet. Der ehrliche Standard, bis du einen Prune-Pfad wählst.",
  "offsite.retention.budget": "Wachstumsbudget (GB, 0 = aus)",
  "offsite.retention.saveBudget": "Budget speichern",
  "flash.downloading": "Lade herunter…",
  "settings.domains": "Domänen",
  "settings.containersEnabled": "Container",
  "settings.vmsEnabled": "VMs",
  "settings.flashEnabled": "Flash",
  "settings.configEnabled": "App-Konfiguration",
  "settings.schedule": "Zeitplan",
  "settings.scheduleOff": "aus",
  "settings.language": "Sprache",
  "settings.save": "Speichern",
  "settings.saved": "Einstellungen gespeichert",
  "settings.error": "Fehler beim Speichern",

  // Retention
  "settings.retentionTitle": "Aufbewahrung",
  "settings.retentionHint": "Wie viele Backups pro Objekt behalten werden. Nach jedem Backup räumt restic ältere Snapshots gemäß dieser Regel auf. Alles 0 = alles behalten (aus).",
  "settings.retentionLast": "Letzte behalten",
  "settings.retentionDaily": "Täglich behalten",
  "settings.retentionWeekly": "Wöchentlich behalten",
  "settings.retentionMonthly": "Monatlich behalten",
  "settings.retentionLocal": "Lokales Repo",
  "settings.retentionOffsite": "Off-site-Repo",
  "settings.retentionOffsiteHint": "Eine separate Regel für das Off-site-Repo, damit du es länger als Archiv behalten kannst. Alles 0 = jedes Off-site-Backup behalten (kein Off-site-Prune).",

  // Off-site-Bandbreite
  "settings.offsiteLimits": "Off-site-Bandbreite",
  "settings.limitUpload": "Upload-Limit (KiB/s)",
  "settings.limitDownload": "Download-Limit (KiB/s)",
  "settings.limitHint": "0 = unbegrenzt. Begrenzt resticts Off-site-Transferrate.",

  // Monitoring (Prometheus)
  "settings.metrics": "Monitoring (Prometheus)",
  "settings.metricsEnable": "/metrics bereitstellen",
  "settings.metricsToken": "Scrape-Token (optional)",
  "settings.metricsHint": "Prometheus-Metriken unter /metrics für Grafana/Uptime Kuma. Mit gesetztem Token via Authorization: Bearer <token> abrufen.",

  // Off-site (rclone)
  "rclone.title": "Off-site (rclone)",
  "rclone.hint": "rclone-Konfiguration einfügen, um in die Cloud zu sichern (Backblaze B2, S3, Google Drive, …). Wird verschlüsselt gespeichert. SMB/NFS brauchen kein rclone: Freigabe in Unraid mounten und einen Backup-Pfad daraufzeigen.",
  "rclone.configured": "Konfigurierte Remotes",
  "rclone.pathHint": "Dann einen Backup-Pfad auf \"rclone:<remote>:<bucket>/pfad\" setzen, um diese Domäne off-site zu senden.",
  "cloud.title": "Cloud-Zugangsdaten (S3 / restic REST)",
  "cloud.hint": "Zugangsdaten für off-site restic-Backends, ohne rclone. Nach dem Speichern einen Backup-Pfad auf ein Remote-Repo setzen, z.B. s3:s3.amazonaws.com/bucket/pfad, rest:http://host:8000/repo, b2:bucket:pfad oder sftp:user@host:/repo. Secrets werden verschlüsselt gespeichert und nie wieder angezeigt.",
  "cloud.secretSet": "gespeichert — leer lassen zum Beibehalten",
  "rclone.save": "Konfig speichern",
  "notify.title": "Benachrichtigungen",
  "notify.hint": "Lass dich benachrichtigen, wenn ein Backup fertig ist. Richte beliebige Kanäle unten ein; sie feuern alle gemäß der Richtlinie.",
  "notify.on": "Benachrichtigen",
  "notify.onNever": "Nie",
  "notify.onFailure": "Nur bei Fehler",
  "notify.onAlways": "Bei Erfolg und Fehler",
  "notify.webhook": "Webhook-URL",
  "notify.webhookFormat": "Webhook-Format",
  "notify.matrix": "Matrix",
  "notify.matrixHomeserver": "Homeserver-URL",
  "notify.matrixToken": "Access-Token",
  "notify.matrixRoom": "Raum-ID",
  "notify.healthchecks": "Healthchecks.io-Ping-URL",
  "notify.healthchecksLifecycle": "Healthchecks wird über den ganzen Backup-Lebenszyklus gepingt — Start, Erfolg und Fehler — sobald eine URL gesetzt ist, unabhängig von der 'Benachrichtigen bei'-Einstellung oben, damit die Prüfung auch bei nur-Fehler-Benachrichtigungen bei Erfolg grün bleibt.",
  "notify.hcPerDomain": "Prüfungen pro Domäne (erweitert)",
  "notify.hcPerDomainHint": "Lass ein Feld leer, um die globale URL oben zu verwenden. Eine Domäne mit eigener URL erhält ihre eigene Prüfung, mit eigener Laufzeit und Historie.",
  "notify.unraid": "Unraid-Benachrichtigungen",
  "notify.unraidHint": "An Unraids eigenes Benachrichtigungssystem senden (das an Pushover, E-Mail, Discord, … weiterleiten kann). Erfordert die eingerichtete SSH-Verbindung (Einstellungen → VM Backup over SSH).",
  "notify.smtp": "E-Mail (SMTP)",
  "notify.smtpHost": "SMTP-Host",
  "notify.smtpPort": "Port",
  "notify.smtpUser": "Benutzername",
  "notify.smtpPass": "Passwort",
  "notify.smtpFrom": "Absenderadresse",
  "notify.smtpTo": "Empfängeradresse",
  "notify.smtpTls": "Verschlüsselung",
  "notify.save": "Speichern",
  "notify.test": "Test senden",
  "notify.tested": "Test gesendet",

  // Integrity (restic check)
  "integrity.title": "Integrität & Wartung",
  "integrity.hint": "Struktur eines Repos verifizieren (restic check), verwaiste Locks eines abgebrochenen Laufs entfernen oder per Prune Speicher gelöschter Backups freigeben.",
  "integrity.verify": "Prüfen",
  "integrity.checking": "Prüfe…",
  "integrity.ok": "✓ Intakt",
  "integrity.failed": "Prüfung fehlgeschlagen",
  "integrity.unlock": "Entsperren",
  "integrity.prune": "Aufräumen",
  "integrity.verifyHint": "restic check ausführen, um Struktur und Metadaten zu verifizieren.",
  "integrity.unlockHint": "Verwaiste Repo-Locks eines abgestürzten/abgebrochenen Laufs entfernen (behebt „repository is already locked“).",
  "integrity.pruneHint": "Retention anwenden und Speicher freigeben (ohne Policy nur Speicher; kann dauern).",
  "integrity.pruneConfirm": "Aufräumen wendet jetzt deine Retention an — entfernt Snapshots jenseits deiner Keep-Regeln (last/daily/weekly/monthly) und gibt Speicher frei. Ohne Policy wird nur Speicher freigegeben. Fortfahren?",

  // Restore-verification drills ("verify restorability")
  "verify.now": "Wiederherstellbarkeit prüfen",
  "verify.running": "Wird geprüft…",
  "verify.ok": "Wiederherstellbar verifiziert",
  "verify.failed": "Verifizierung fehlgeschlagen",
  "verify.last": "Zuletzt geprüft {time}",
  "verify.never": "Nie geprüft",
  "verify.auto": "Automatische Restore-Prüfungen",
  "verify.subsetPct": "Datenstichprobe (%)",
  "verify.hint": "Liest regelmäßig eine zufällige Stichprobe der Backup-Daten, um zu beweisen, dass sie intakt und wiederherstellbar sind.",
  "verify.shield": "verifiziert",

  // DR-Test-Steuerung (echte Off-site-Wiederherstellung) + Off-site-Badge
  "drill.kindLabel": "Prüfart:",
  "drill.kindSubset": "Integritätsprüfung",
  "drill.kindDR": "Echte Wiederherstellung (Off-site)",
  "drill.target": "Testziel (Container)",
  "drill.targetMostRecent": "Neuestes Backup",
  "drill.drNote": "Eine echte Wiederherstellung entpackt den neuesten Off-site-Snapshot in eine temporäre Sandbox, verifiziert ihn und räumt danach auf. Dabei werden echte Daten geladen, das kann dauern.",
  "drill.drVMsNote": "Echte Wiederherstellung ist für VMs nicht verfügbar — ihre Disk-Images sind zu groß für eine Sandbox-Wiederherstellung. Nutze stattdessen die Integritätsprüfung.",
  "drill.runDR": "Echte Wiederherstellung starten",
  "drill.runningDR": "Stelle wieder her…",
  "drill.confirmDR": "Dies führt eine ECHTE Wiederherstellung des neuesten Off-site-Snapshots in eine temporäre Sandbox durch, um die Wiederherstellbarkeit zu beweisen, und löscht sie danach. Dabei werden echte Daten geladen, das kann dauern. Fortfahren?",
  "drill.provenOffsite": "nachweislich aus Off-site wiederherstellbar",

  // Pre/post-backup hooks
  "hooks.title": "Backup-Hooks",
  "hooks.hint": "Befehle laufen im Container mit sh -c. Der Pre-Befehl läuft vor dem Backup; nutze ihn, um Daten vorzubereiten, die mitgesichert werden sollen, etwa eine Datenbank in die appdata des Containers zu dumpen. Schlägt der Pre-Befehl fehl, wird das Backup abgebrochen. Der Post-Befehl läuft, nachdem der Container wieder gestartet wurde, und sein Fehler wird nur geloggt. Hooks führen nur Befehle aus, sie fügen dem Backup keine zusätzlichen Ordner hinzu.",
  "hooks.pre": "Pre-Backup-Befehl",
  "hooks.post": "Post-Backup-Befehl",
  "folders.title": "Gesicherte Ordner",
  "folders.hint": "Wähle, welche gemappten Ordner dieses Containers gesichert werden. Der appdata-Ordner ist standardmäßig ausgewählt. Hake weitere an, um sie einzuschließen, oder füge einen eigenen Pfad unterhalb des Host-Mounts hinzu. Hakst du alles ab, gilt wieder die automatische appdata-Erkennung.",
  "folders.appdataDefault": "appdata (Standard)",
  "folders.notReachable": "nicht unter dem Host-Mount, kann nicht gesichert werden",
  "folders.customPlaceholder": "/mnt/user/irgendein/ordner",
  "folders.addCustom": "Ordnerpfad hinzufügen",
  "folders.add": "Hinzufügen",
  "folders.save": "Ordner speichern",
  "folders.saved": "Gespeichert",
  "folders.empty": "Keine gemappten Ordner für diesen Container gefunden.",
  "stophook.title": "Andere Container stoppen",
  "stophook.hint": "Diese anderen Container während des Backups dieses Containers stoppen (zum Beispiel eine Datenbank) und danach wieder starten. Ein Container-Name pro Zeile.",
  "export.button": "Export (Plain-tar)",
  "export.exportedTo": "Exportiert nach:",
  "backup.configOnly": "Nur Konfiguration — keine Datenordner (Definition für Wiederherstellung gesichert)",

  // Appearance / Accent
  "settings.appearance": "Erscheinungsbild",
  "settings.accentColor": "Akzentfarbe",
  "settings.accentPresets": "Voreinstellungen",

  // Dashboard stat cards
  "dashboard.statContainers": "Container",
  "dashboard.statVMs": "VMs",
  "dashboard.statActiveJobs": "Aktive Jobs",
  "dashboard.statPausedJobs": "Pausierte Jobs",
  "dashboard.statErrors": "Fehler",
  "dashboard.statMissingContainers": "Fehlende Container",
  "dashboard.statMissingVMs": "Fehlende VMs",

  // Dashboard protection (RPO) status
  "dashboard.protectionTitle": "Schutzstatus",
  "dashboard.rpoOk": "Aktuell",
  "dashboard.rpoWarn": "Bald fällig",
  "dashboard.rpoOverdue": "Überfällig",
  "dashboard.rpoNever": "Noch kein Backup",
  "dashboard.rpoOff": "Kein Zeitplan",
  "dashboard.domainContainers": "Container",
  "dashboard.domainVMs": "VMs",
  "dashboard.domainFlash": "Flash",
  "dashboard.domainConfig": "Config",

  // Dashboard-Ransomware-Schutz-Karte (v4)
  "ransomware.title": "Ransomware-Schutz",
  "ransomware.protGreen": "Geschützt",
  "ransomware.protAmber": "Aufmerksamkeit nötig",
  "ransomware.protRed": "Gefährdet",
  "ransomware.configured": "Off-site konfiguriert",
  "ransomware.appendOnlyVerified": "Append-only verifiziert",
  "ransomware.appendOnlyStale": "Append-only-Nachweis veraltet",
  "ransomware.appendOnlyFailed": "Append-only-Test fehlgeschlagen",
  "ransomware.appendOnlyNever": "Append-only noch nicht nachgewiesen",
  "ransomware.appendOnlyOff": "Append-only nicht aktiviert",
  "ransomware.replicationCurrent": "Replikation aktuell",
  "ransomware.replicationOverdue": "Replikation überfällig",
  "ransomware.replicationNever": "Noch nicht repliziert",
  "ransomware.drillOffsite": "Restore-Test (Off-site)",
  "ransomware.drillOverdue": "Restore-Test überfällig",
  "ransomware.drillNever": "Noch kein Restore-Test",
  "ransomware.encryptionOn": "Verschlüsselung an",
  "ransomware.pruneStrategy": "Prune-Strategie gesetzt",

  // Dashboard backup-health heatmap
  "dashboard.healthTitle": "Backup-Verlauf",
  "dashboard.heatLess": "Weniger",
  "dashboard.heatMore": "Mehr",

  // Dashboard storage (repo size + dedup) card
  "dashboard.storageTitle": "Speicher",
  "dashboard.dedup": "Dedup",
  "dashboard.snapshotsLabel": "Snapshots",
  "dashboard.noStats": "Noch keine Daten",

  // Jobs page
  "nav.jobs": "Pläne",
  "jobs.title": "Pläne",
  "jobs.subtitle": "Backup-Pläne nach Domäne",
  "jobs.configureInSettings": "Zeitpläne in den Einstellungen konfigurieren",
  "jobs.containersSection": "Container",
  "jobs.vmsSection": "VMs",
  "jobs.flashSection": "Flash",
  "jobs.active": "Aktiv",
  "jobs.paused": "Pausiert",
  "jobs.notScheduled": "Kein Zeitplan",
  "jobs.cadenceDaily": "Täglich um {time}",
  "jobs.cadenceWeekly": "Wöchentlich ({days}) um {time}",
  "jobs.cadenceEveryN": "Alle {n} Tage um {time}",
  "sort.label": "Sortieren:",
  "sort.nameAsc": "Name (A–Z)",
  "sort.status": "Status",
  "sort.ip": "IP",
  "cadence.off": "Aus",
  "cadence.daily": "Täglich",
  "cadence.weekly": "Wöchentlich",
  "cadence.everyN": "Alle N Tage",
  "cadence.time": "Zeit",
  "cadence.days": "Tage",
  "cadence.every": "Alle",
  "cadence.daysUnit": "Tage",
  "cadence.fmtDaily": "täglich um {time} Uhr",
  "cadence.fmtWeekly": "wöchentlich ({days}) um {time} Uhr",
  "cadence.fmtEveryN": "jeden {n}. Tag um {time} Uhr",
  "time.justNow": "gerade eben",
  "time.minutesAgo": "vor {n} Min.",
  "time.hoursAgo": "vor {n} Std.",
  "time.daysAgo": "vor {n} T.",
  "folder.browse": "Durchsuchen…",
  "folder.browseTitle": "Ordner durchsuchen",
  "folder.use": "Diesen Ordner verwenden",
  "folder.none": "Keine Unterordner",
  "folder.loading": "Lädt…",
  "folder.pathHint": "Pfad muss ein relativer Unterpfad sein (kein führendes / oder ..)",
  "folder.couldNotRead": "Verzeichnis konnte nicht gelesen werden",
  "folder.browseFailed": "Durchsuchen fehlgeschlagen",
  "common.reset": "Zurücksetzen",
  "containers.subtitle": "Container-Backups, Zeitpläne und Wiederherstellungen verwalten.",
  "containers.emptyDocker": "Keine Container gefunden. Läuft Docker?",
  "containers.bulkResult": "{ok} ok, {fail} fehlgeschlagen",
  "vm.method.saveFailed": "Backup-Methode konnte nicht geändert werden — sie wurde nicht umgestellt.",
  "jobs.noVMs": "Noch keine VMs",
  "jobs.noContainersIncluded": "Keine Container im Zeitplan enthalten.",
  "jobs.flashRow": "Unraid Flash-Konfiguration",
  "jobs.flashPlanned": "geplant",
  "jobs.vmPlanned": "VM-Backup-Executor noch nicht implementiert.",
  "jobs.syncSchedules": "Container-Zeitplan auch für VMs und Flash verwenden",
  "jobs.vmIncludeHint": "Sichert jede VM mit aktiviertem „In Zeitplan aufnehmen“ (pro VM im VMs-Tab einstellbar).",
  "jobs.flashNotImplemented": "Hinweis: Der Flash-Backup-Executor ist in Phase 1 noch nicht implementiert — der Zeitplan wird gespeichert, aber nicht ausgeführt.",
  "schedule.includeAll": "Alle in den Zeitplan",
  "schedule.excludeAll": "Alle aus dem Zeitplan",

  // Auth / Login
  "auth.loginTitle": "BombVault",
  "auth.passwordLabel": "Passwort",
  "auth.signIn": "Anmelden",
  "auth.signingIn": "Anmeldung läuft…",
  "auth.invalidPassword": "Falsches Passwort",
  "auth.loginError": "Anmeldung fehlgeschlagen",

  // Settings — Security card
  "auth.security": "Sicherheit",
  "auth.authOff": "Authentifizierung ist deaktiviert — alle LAN-Nutzer haben vollen Zugriff.",
  "auth.authOn": "Authentifizierung ist aktiviert.",
  "auth.setPassword": "Passwort setzen",
  "auth.changePassword": "Passwort ändern",
  "auth.confirmPassword": "Passwort bestätigen",
  "auth.passwordMismatch": "Passwörter stimmen nicht überein",
  "auth.passwordSaved": "Passwort gespeichert",
  "auth.passwordCleared": "Authentifizierung deaktiviert",
  "auth.passwordHint":
    "Beide Felder leer lassen, um die Authentifizierung zu deaktivieren. BombVault hat root-ähnliche Host-Kontrolle — ein Passwort ist empfohlen, wenn diese Instanz für nicht vertrauenswürdige LAN-Nutzer erreichbar ist.",
  "auth.logout": "Abmelden",
  "auth.saving": "Speichern…",
  "auth.saveError": "Speichern fehlgeschlagen",

  // Common action labels (shared across container / VM / settings buttons)
  "common.backingUp": "Sichere…",
  "common.restoring": "Stelle wieder her…",
  "common.done": "Fertig",
  "common.close": "Schließen",
  "common.loadingBackups": "Sicherungen werden geladen…",
  "common.saving": "Speichern…",
  "common.restoreRunning": "Eine Wiederherstellung läuft…",
  "common.backupRunning": "Eine Sicherung läuft…",
  "common.replicateRunning": "Eine Replikation läuft…",

  // VMs page
  "vms.title": "Virtuelle Maschinen",
  "vms.subtitle": "VM-Backups, Zeitpläne und Wiederherstellungen verwalten.",
  "vms.empty": "Keine VMs gefunden. Läuft libvirt/KVM?",
  "vms.backupSelected": "Auswahl sichern",
  "vms.restoreSelected": "Auswahl wiederherstellen (neuestes)",
  "vms.restoreSelectedConfirm": "Das NEUESTE Backup der ausgewählten VMs wiederherstellen? Jede VM wird heruntergefahren, ihre Disk-Dateien ersetzt und die VM wiederhergestellt.",
  "vms.notInstalledHint": "Diese VMs sind nicht mehr auf dem Host definiert, haben aber noch Backups. Stelle sie wieder her oder sieh ihre Snapshots im Backups-Panel ein.",
  "vms.removeEntry": "Eintrag entfernen",
  "vms.removeEntryConfirm": "Den Eintrag dieser VM aus der Liste entfernen? Vorhandene Backups werden nicht gelöscht.",
  "vms.discoverHint": "VM aus Unraid gelöscht (oder nach einer Neuinstallation verloren)? Baue ihren Backup-Eintrag aus dem Speicher neu auf, um sie wiederherzustellen.",

  // Flash (Unraid USB) backup
  "flash.title": "Flash-Backup",
  "flash.subtitle": "Den Unraid-USB-Stick (das ganze /boot) sichern und wiederherstellen.",
  "flash.backupTitle": "Flash sichern",
  "flash.backupHint": "Sichert den kompletten USB-Stick (/boot): Unraid-OS, Lizenz, Array-Config, Shares, Netzwerk und Plugin-Config.",
  "flash.backupNow": "Flash jetzt sichern",
  "flash.backingUp": "Sichere…",
  "flash.download": "Download (.zip)",
  "flash.restoreNote": "Restore lädt ein ZIP des Snapshots herunter — der laufende /boot wird nie angefasst. Das .zip direkt in den Unraid-USB-Creator geben oder auf einen frischen USB-Stick entpacken, um deinen Flash neu aufzubauen.",
  "flash.none": "Noch keine Flash-Backups — oben eines starten.",
  // Geplanter Flash-ZIP-Export (#28): ein einfaches .zip, das nach jedem Flash-Backup in einen Ordner geschrieben wird.
  "flash.zipExport.title": "Flash-ZIP-Export",
  "flash.zipExport.hint": "Nach jedem Flash-Backup den Snapshot zusätzlich als einfaches .zip in einen Ordner schreiben — bereit für Off-Server-Sync (Syncthing, rclone, ein Cloud-Laufwerk).",
  "flash.zipExport.enable": "Nach jedem Flash-Backup ein ZIP exportieren",
  "flash.zipExport.enableHint": "Bei jedem erfolgreichen Flash-Backup wird der Snapshot als .zip in den Ordner unten geschrieben.",
  "flash.zipExport.path": "Export-Ordner",
  "flash.zipExport.pathHint": "Relativer Unterpfad unter dem Host-Mount-Root, in den das .zip geschrieben wird — auf einen Syncthing-/rclone-Ordner zeigen lassen, um den Flash automatisch vom Server zu bekommen.",
  "flash.zipExport.keepHistory": "Verlauf behalten",
  "flash.zipExport.keepHistoryHint": "Aus: eine einzige flash-latest.zip behalten, die jedes Mal überschrieben wird. An: die neuesten N flash-<Datum>.zip-Dateien mit Zeitstempel behalten.",
  "flash.zipExport.keepN": "Zu behaltende ZIPs",
  "flash.zipExport.keepNHint": "Die neuesten N ZIPs mit Zeitstempel werden behalten, ältere automatisch gelöscht.",
  "flash.zipExport.latestNote": "Eine einzige flash-latest.zip wird nach jedem Backup überschrieben.",
  "flash.zipExport.plaintextWarn": "Das exportierte .zip ist nicht verschlüsselt, auch wenn dein Flash-Repository es ist. Synce es nur an einen vertrauenswürdigen Ort.",
  "flash.zipExport.pathRequired": "Wähle einen Export-Ordner, um dies zu aktivieren.",

  // Config-Selbst-Backup (BombVaults eigene Einstellungen). Minimaler en/de-Satz
  // für Task 12; die vollständige 24-Sprachen-Übersetzung folgt in Task 14.
  "config.title": "Config-Backup",
  "config.subtitle": "Sichert BombVaults eigene Einstellungen, damit sich ein neu aufgesetzter Server selbst wiederherstellen kann.",
  "config.settingsTitle": "Config-Backup-Einstellungen",
  "config.settingsHint": "Schützt BombVaults eigene Konfiguration — die Einstellungsdatenbank, Offsite-Zugangsdaten und SSH-Schlüssel — damit eine frische Installation sich selbst wiederherstellt und genau dort weitermacht, wo sie aufgehört hat.",
  "config.enabled": "BombVaults Einstellungen sichern",
  "config.enabledHint": "BombVaults eigenes /config in den unten stehenden Zeitplan aufnehmen.",
  "config.path": "Backup-Ort",
  "config.pathHint": "Relativer Unterpfad unter dem Host-Mount-Root, in den das Config-Repo geschrieben wird.",
  "config.schedule": "Zeitplan",
  "config.schedulePlaceholder": "off · z.B. daily 03:30",
  "config.scheduleHint": "Wann die Einstellungen automatisch gesichert werden. 'off' lassen, um nur bei Bedarf zu sichern.",
  "config.offsite": "Offsite-Repo (optional)",
  "config.offsiteHint": "Das Config-Backup nach jedem lokalen Backup in ein zweites, ausgelagertes Repo replizieren.",
  "config.offsiteSchedule": "Offsite-Zeitplan",
  "config.immutable": "Offsite-Repo ist append-only (unveränderlich)",
  "config.immutableHint": "Offsite-Pruning überspringen und Offsite-Löschungen verweigern — die Gegenseite (append-only) erzwingt es.",
  "config.backupTitle": "Einstellungen jetzt sichern",
  "config.backupHint": "Erfasst BombVaults eigenes /config: die Einstellungsdatenbank, Offsite-Zugangsdaten (rclone.conf) und das SSH-Schlüsselpaar.",
  "config.backupNow": "Einstellungen jetzt sichern",
  "config.backingUp": "Sichere…",
  "config.snapshotsTitle": "Einstellungs-Backups",
  "config.snapshotsHint": "Um diese Einstellungen auf einem neu aufgesetzten Server wiederherzustellen, den Wiederherstellungs-Tab verwenden — das Wiederherstellen der Einstellungen startet BombVault neu, damit sie angewendet werden, daher liegt es dort beim übrigen Notfall-Ablauf.",
  "config.none": "Noch keine Einstellungs-Backups — oben eines starten.",

  // Container / VM state badge labels
  "state.created":      "Erstellt",
  "state.running":      "Läuft",
  "state.paused":       "Pausiert",
  "state.restarting":   "Neustart",
  "state.removing":     "Wird entfernt",
  "state.exited":       "Beendet",
  "state.dead":         "Tot",
  "state.shutoff":      "Ausgeschaltet",
  "state.inshutdown":   "Fährt herunter",
  "state.crashed":      "Abgestürzt",
  "state.pmsuspended":  "Suspendiert",
  "state.notInstalled": "Nicht installiert",

  // VM-Backup (SSH)
  "vm.method": "Methode",
  "vm.method.graceful": "Graceful (Herunterfahren)",
  "vm.method.live": "Live-Snapshot",
  "vm.method.hint": "Graceful fährt die VM während des Backups herunter; Live lässt sie laufen (Snapshot, kein Ausfall).",
  "vm.ssh.title": "VM-Backup über SSH",
  "vm.ssh.desc": "VM-Backup erreicht libvirt über SSH (ohne Mount). Diesen Schlüssel auf Unraid autorisieren, dann testen.",
  "vm.ssh.host": "Host",
  "vm.ssh.publicKey": "Public Key — an Unraids /root/.ssh/authorized_keys anhängen",
  "vm.ssh.copy": "Kopieren",
  "vm.ssh.copied": "Kopiert",
  "vm.ssh.test": "Verbindung testen",
  "vm.ssh.testing": "Teste…",
  "vm.ssh.testOk": "Verbunden — libvirt erreichbar",
  "vm.ssh.testFail": "Verbindung fehlgeschlagen",
  "vm.ssh.setupTitle": "Einrichten (einmalig)",
  "vm.ssh.step1": "Den Befehl unten kopieren und im Unraid-Terminal ausführen, um diesen Schlüssel zu autorisieren (überlebt Reboots).",
  "vm.ssh.step2": "Die Container-Variable “VM Backup: Host” auf die LAN-IP deines Unraid-Servers setzen (z. B. 192.168.x.x); bei einfachem Bridge-Netz geht auch host.docker.internal.",
  "vm.ssh.step3": "Auf “Verbindung testen” klicken — sobald grün, VMs unter Domänen aktivieren.",
  "vm.ssh.copyCmd": "Befehl kopieren",
  "vm.ssh.guide": "Vollständige Setup- & Netzwerk-Anleitung",

  // Guided Recovery tab (disaster-recovery walkthrough)
  "nav.recovery": "Wiederherstellung",
  "recovery.pageTitle": "Notfall-Wiederherstellung",
  "recovery.intro": "Stelle deine Container und VMs aus einem vorhandenen Backup auf dieser Installation wieder her. Richte BombVault auf deine Backups aus, finde heraus, was darin steckt, und stelle es wieder her.",
  // Schritt 1 — Verbindungs-/APP_KEY-Lesbarkeitsprüfung
  "recovery.step1": "Kann BombVault deine Backups lesen?",
  "recovery.appKeyExplain": "Um vorhandene Backups zu lesen, braucht dieser Container denselben APP_KEY wie zuvor — er steht in deinem Recovery-Kit. Setze ihn im Unraid-Container-Template, falls noch nicht geschehen, und prüfe erneut.",
  "recovery.appKeyRemedy": "Der Verschlüsselungsschlüssel passt nicht zu diesen Backups. Trage den ursprünglichen APP_KEY (aus deinem Recovery-Kit) im Container-Template ein und prüfe erneut.",
  "recovery.readable": "Deine Backups sind lesbar.",
  "recovery.notReachable": "Deine Backups waren noch nicht erreichbar — hänge den Speicherort unten an und prüfe erneut.",
  "recovery.recheck": "Erneut prüfen",
  // Schritt 2 — zuerst BombVaults eigene Einstellungen wiederherstellen (optional)
  "recovery.stepConfig": "BombVaults eigene Einstellungen wiederherstellen",
  "recovery.configHint": "Stelle auf einem neu aufgesetzten Server zuerst BombVaults eigene Einstellungen wieder her — Backup-Pfade, Off-site-Ziele und Zugangsdaten — damit die Schritte unten schon vorausgefüllt sind. Richte es auf das zuvor eingerichtete Einstellungs-Backup aus. Kein Einstellungs-Backup? Überspringe dies und hänge deine Backups unten manuell an.",
  "recovery.configAppKeyReminder": "Dein APP_KEY muss zu diesem Backup passen — das ist die Prüfung in Schritt 1 oben.",
  "recovery.configSourceLabel": "Wo liegt das Einstellungs-Backup?",
  "recovery.configLocalPath": "Lokaler Pfad",
  "recovery.configOffsiteUrl": "Off-site-Repo-URL",
  "recovery.configRestore": "BombVaults Einstellungen wiederherstellen",
  "recovery.configRestoring": "Stelle wieder her…",
  "recovery.configRestarting": "BombVault startet neu, um deine Einstellungen anzuwenden… diese Seite lädt automatisch neu, sobald es wieder da ist.",
  "recovery.configManualRestart": "Deine Einstellungen sind bereitgestellt. Starte den BombVault-Container in Unraid neu und fahre dann fort — sie werden beim nächsten Start angewendet.",
  "recovery.configReloadWhenBack": "BombVault braucht länger als erwartet, um zurückzukommen. Lade diese Seite neu, sobald es wieder läuft, um deine wiederhergestellten Einstellungen zu laden.",
  "recovery.configReload": "Jetzt neu laden",
  "recovery.configSkip": "Überspringen — ich habe kein Einstellungs-Backup",
  "recovery.configSkipped": "Übersprungen. Hänge deine Backups unten manuell an.",
  // Schritt 3 — Backups anhängen
  "recovery.step2": "Backups anhängen",
  "recovery.attachHint": "Richte BombVault auf deine vorhandenen Backups aus: einen lokalen Pfad unter dem Host-Mount oder ein Off-site-Repo (rest / S3 / B2 / sftp / rclone) mit den zugehörigen Zugangsdaten. Verbinde dich dann, um es zu bestätigen.",
  "recovery.credsSaveHint": "Off-site-Zugangsdaten werden über den eigenen Speichern-Button der jeweiligen Karte gespeichert — speichere sie, bevor du „Verbinden & prüfen“ klickst.",
  "recovery.connectPreview": "Verbinden & prüfen",
  // Schritt 3 — alles entdecken
  "recovery.step3": "Entdecke, was in deinen Backups steckt",
  "recovery.discover": "Backups entdecken",
  "recovery.foundCounts": "{c} Container und {v} VMs gefunden.",
  "recovery.foundNone": "Noch nichts gefunden — prüfe Verbindung und Anhang oben. Falls du hier Backups erwartest, stelle sicher, dass dein APP_KEY zu diesen Backups passt.",
  // Schritt 4 — prüfen & alle wiederherstellen (gestoppt lassen)
  "recovery.step4": "Prüfen und wiederherstellen",
  "recovery.restoreAll": "Alle wiederherstellen (gestoppt lassen)",
  "recovery.restoreAllResult": "{ok} wiederhergestellt, {fail} fehlgeschlagen. Starte sie bei Bedarf über die Tabs Container/VMs.",
  "recovery.vmSshNote": "Für die VM-Wiederherstellung wird die libvirt-SSH-Verbindung benötigt — richte sie unter Einstellungen → VM-Backup über SSH ein.",
  "recovery.noneDiscovered": "Führe zuerst oben „Entdecken“ aus.",
  // Schritt 5 — Recovery-Kit (Sicherheitsnetz fürs nächste Mal)
  "recovery.step5": "Dein Recovery-Kit",
  "recovery.kitHint": "Lade dein Recovery-Kit herunter und bewahre es sicher auf — es enthält den Verschlüsselungsschlüssel und die genauen restic-Befehle, um selbst ohne BombVault wiederherzustellen.",
  "recovery.kitDownload": "Recovery-Kit herunterladen",
  // Dashboard-Hinweis bei frischer Installation → geführter Wiederherstellungs-Tab
  "recovery.freshNudge": "Wiederherstellung von einem früheren Server oder nach einem Neuaufbau? Stelle deine vorhandenen Backups wieder her.",
  "recovery.freshNudgeCta": "Zur Wiederherstellung",
};

// ---------------------------------------------------------------------------
// Locale registry — 26 languages. Only en + de are fully translated;
// all others stub to English (full translations are a separate backlog item).
// ---------------------------------------------------------------------------

export interface Language {
  /** BCP-47 language code used as the locale key. */
  code: string;
  /** Endonym — the language's own name, shown in the picker. */
  label: string;
  /** ISO 3166-1 alpha-2 region code used by flag-icons (fi fi-XX). */
  flag: string;
  /** true for right-to-left languages (Arabic, Hebrew, …). */
  rtl?: boolean;
}

export const LANGUAGES: Language[] = [
  { code: "en", label: "English",      flag: "gb" },
  { code: "de", label: "Deutsch",      flag: "de" },
  { code: "fr", label: "Français",     flag: "fr" },
  { code: "es", label: "Español",      flag: "es" },
  { code: "it", label: "Italiano",     flag: "it" },
  { code: "pt", label: "Português",    flag: "pt" },
  { code: "nl", label: "Nederlands",   flag: "nl" },
  { code: "pl", label: "Polski",       flag: "pl" },
  { code: "ru", label: "Русский",      flag: "ru" },
  { code: "uk", label: "Українська",   flag: "ua" },
  { code: "cs", label: "Čeština",      flag: "cz" },
  { code: "sv", label: "Svenska",      flag: "se" },
  { code: "da", label: "Dansk",        flag: "dk" },
  { code: "fi", label: "Suomi",        flag: "fi" },
  { code: "no", label: "Norsk",        flag: "no" },
  { code: "tr", label: "Türkçe",       flag: "tr" },
  { code: "el", label: "Ελληνικά",     flag: "gr" },
  { code: "hu", label: "Magyar",       flag: "hu" },
  { code: "ro", label: "Română",       flag: "ro" },
  { code: "ja", label: "日本語",        flag: "jp" },
  { code: "ko", label: "한국어",        flag: "kr" },
  { code: "zh", label: "中文",          flag: "cn" },
  { code: "ar", label: "العربية",       flag: "sa", rtl: true },
  { code: "he", label: "עברית",         flag: "il", rtl: true },
  { code: "th", label: "ไทย",           flag: "th" },
  { code: "vi", label: "Tiếng Việt",   flag: "vn" },
];

export const SUPPORTED = LANGUAGES.map((l) => l.code);

/** Locales offered in the language switcher UI. All locales are selectable;
 *  any without a full translation fall back to English at runtime (see `t`). */
export const OFFERED_LANGUAGES: Language[] = LANGUAGES;

const DEFAULT_CODE = "en";
const STORAGE_KEY = "bv-lang";

/** Whether a language code is right-to-left. */
export const isRtl = (code: string): boolean =>
  LANGUAGES.find((l) => l.code === code)?.rtl ?? false;

// Translated locales. en + de live inline above; the other 24 are imported from
// ./locales/<code>.ts (each typed as Translations, so a missing/renamed key
// fails the build). Any locale still absent from this map falls back to English.
// en + de are the complete source of truth; the other 24 are Partial and fall
// back to en at runtime for any missing key (see the t() lookup).
const locales: Record<string, Partial<Translations>> = {
  en,
  de,
  fr,
  es,
  it,
  pt,
  nl,
  pl,
  ru,
  uk,
  cs,
  sv,
  da,
  fi,
  no,
  tr,
  el,
  hu,
  ro,
  ja,
  ko,
  zh,
  ar,
  he,
  th,
  vi,
};

/** Resolve a raw locale code to one offered in the switcher (else the default). */
function resolveCode(raw: string | null): string {
  const offered = OFFERED_LANGUAGES.map((l) => l.code);
  if (raw && offered.includes(raw)) return raw;
  const browser = navigator.language.slice(0, 2);
  if (offered.includes(browser)) return browser;
  return DEFAULT_CODE;
}

function storedCode(): string {
  return resolveCode(localStorage.getItem(STORAGE_KEY));
}

/** Called at boot in main.tsx before first React render (flash prevention). */
export function applyStoredLanguage(): void {
  const code = storedCode();
  document.documentElement.setAttribute("lang", code);
  if (isRtl(code)) document.documentElement.setAttribute("dir", "rtl");
}

// ---------------------------------------------------------------------------
// React Context
// ---------------------------------------------------------------------------

export interface I18nContextValue {
  lang: string;
  setLanguage: (code: string) => void;
  t: (key: TranslationKey) => string;
  languages: Language[];
}

// Provide a safe default so `useT()` never throws outside a Provider during tests.
const I18nContext = createContext<I18nContextValue>({
  lang: DEFAULT_CODE,
  setLanguage: () => undefined,
  t: (key) => en[key] ?? key,
  languages: OFFERED_LANGUAGES,
});

/** Mount once at the app root (Layout or main). Children share one language state. */
export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<string>(storedCode);

  const setLanguage = useCallback((code: string) => {
    const offered = OFFERED_LANGUAGES.map((l) => l.code);
    if (!offered.includes(code)) return;
    localStorage.setItem(STORAGE_KEY, code);
    document.documentElement.setAttribute("lang", code);
    document.documentElement.setAttribute("dir", isRtl(code) ? "rtl" : "ltr");
    setLangState(code);
  }, []);

  const t = useCallback(
    (key: TranslationKey): string => {
      const locale = locales[lang] ?? locales[DEFAULT_CODE];
      return locale[key] ?? en[key] ?? key;
    },
    [lang]
  );

  return createElement(
    I18nContext.Provider,
    { value: { lang, setLanguage, t, languages: OFFERED_LANGUAGES } },
    children
  );
}

/**
 * useT() — reads from the shared I18nContext.
 * Must be called inside <I18nProvider>. Any setLanguage call re-renders the whole tree.
 */
export function useT(): I18nContextValue {
  return useContext(I18nContext);
}

// ---------------------------------------------------------------------------
// stateLabel — maps a raw Docker / libvirt state string to a translated label.
// Normalises the raw value (lowercase, spaces→"", dashes→"") then looks up
// the matching state.* key.  Falls back to the raw string for unknown states.
// ---------------------------------------------------------------------------

const STATE_KEY_MAP: Record<string, TranslationKey> = {
  created:      "state.created",
  running:      "state.running",
  paused:       "state.paused",
  restarting:   "state.restarting",
  removing:     "state.removing",
  exited:       "state.exited",
  dead:         "state.dead",
  // Docker "stopped" → reuse exited colour/label
  stopped:      "state.exited",
  // libvirt
  shutoff:      "state.shutoff",
  "shut off":   "state.shutoff",
  inshutdown:   "state.inshutdown",
  "in shutdown":"state.inshutdown",
  crashed:      "state.crashed",
  pmsuspended:  "state.pmsuspended",
  // not-installed sentinel
  "not-installed": "state.notInstalled",
  notinstalled:    "state.notInstalled",
};

/**
 * Returns the translated display label for a container or VM state.
 * The `t` function must come from `useT()`.
 * Falls back to the original raw string when no mapping is found.
 */
export function stateLabel(t: (key: TranslationKey) => string, rawState: string): string {
  const norm = rawState.toLowerCase().trim();
  const key = STATE_KEY_MAP[norm];
  return key ? t(key) : rawState;
}
