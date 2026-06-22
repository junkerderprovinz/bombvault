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
  "nav.settings": "Settings",
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
  "snapshots.recreate": "Recreate from saved config",
  "snapshots.recreateConfirm": "Recreate this container from its saved configuration? It is created and started from the stored definition (image, env, ports, volumes).",
  "snapshots.configOnlyHint": "Config-only backup: the container's definition is saved, but it has no data folders to snapshot. If you delete the container it appears under \"Not installed\", where you can recreate it from this config.",

  // File-level restore
  "files.restore": "Restore",
  "files.restored": "Restored",
  "files.restoreConfirm": "Restore this file to its original location? It overwrites the current file.",
  "files.filterPlaceholder": "Filter files…",
  "files.none": "No matching files",
  "files.loadFailed": "Failed to load files",
  "files.more": "Refine the filter to see more files.",

  // Restore
  "restore.confirmTitle": "Confirm restore",
  "restore.confirmBody":
    "This will stop the container, replace its appdata and recreate it from the backup. Continue?",
  "restore.confirm": "Confirm",
  "restore.cancel": "Cancel",
  "restore.preview": "Preview",
  "restore.started": "Restore started",

  // Runs
  "run.kindBackup": "Backup",
  "run.kindRestore": "Restore",
  "run.statusRunning": "Running",
  "run.statusSuccess": "Success",
  "run.statusFailed": "Failed",
  "run.historyTitle": "Run History",
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
  "settings.paths": "Backup Paths",
  "settings.containersPath": "Containers path",
  "settings.vmsPath": "VMs path",
  "settings.flashPath": "Flash path",
  "settings.offsiteTitle": "Off-site copy (optional)",
  "settings.offsiteHint": "After each successful local backup, also replicate it to a second repo with restic copy. Enter a remote (rest:http://host:8000/repo, s3:…, b2:…) or a local subpath; leave blank to disable. The local backup stays primary.",
  "settings.domains": "Domains",
  "settings.containersEnabled": "Containers",
  "settings.vmsEnabled": "VMs",
  "settings.flashEnabled": "Flash",
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
  "notify.unraid": "Unraid notifications",
  "notify.unraidHint": "Send to Unraid's own notification system (which can forward to Pushover, email, Discord, …). Needs the SSH connection set up (Settings → VM Backup over SSH).",
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
  "integrity.pruneHint": "Reclaim disk space from deleted/forgotten backups (can take a while).",
  "integrity.pruneConfirm": "Prune reclaims space from deleted backups and can take several minutes. Continue?",

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
  "jobs.noVMs": "No VMs yet",
  "jobs.noContainersIncluded": "No containers included in schedule.",
  "jobs.flashRow": "Unraid flash config",
  "jobs.flashPlanned": "planned",
  "jobs.vmPlanned": "VM backup executor not yet implemented.",

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
  "common.loadingBackups": "Loading backups…",
  "common.saving": "Saving…",

  // VMs page
  "vms.title": "Virtual Machines",
  "vms.subtitle": "Manage VM backups, schedules, and restores.",
  "vms.empty": "No VMs found. Is libvirt/KVM running?",
  "vms.backupSelected": "Back up selected",
  "vms.restoreSelected": "Restore selected (latest)",
  "vms.restoreSelectedConfirm": "Restore the LATEST backup of the selected VMs? Each VM is shut off, its disk files replaced, and the VM restored.",
  "vms.notInstalledHint": "These VMs are no longer defined on the host but still have backups. Restore them to recover, or use the Backups panel to browse their snapshots.",
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
  "nav.settings": "Einstellungen",
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
  "snapshots.recreate": "Aus gespeicherter Konfig neu erstellen",
  "snapshots.recreateConfirm": "Diesen Container aus seiner gespeicherten Konfiguration neu erstellen? Er wird aus der gespeicherten Definition (Image, Env, Ports, Volumes) angelegt und gestartet.",
  "snapshots.configOnlyHint": "Nur-Konfig-Backup: die Definition des Containers ist gesichert, es gibt aber keine Datenordner zum Snapshotten. Wird der Container gelöscht, erscheint er unter „Nicht installiert“ und kann von dort aus dieser Konfig neu erstellt werden.",

  // File-level restore
  "files.restore": "Wiederherstellen",
  "files.restored": "Wiederhergestellt",
  "files.restoreConfirm": "Diese Datei an ihren Originalort wiederherstellen? Die aktuelle Datei wird überschrieben.",
  "files.filterPlaceholder": "Dateien filtern…",
  "files.none": "Keine passenden Dateien",
  "files.loadFailed": "Dateien konnten nicht geladen werden",
  "files.more": "Filter verfeinern, um mehr Dateien zu sehen.",

  "restore.confirmTitle": "Wiederherstellung bestätigen",
  "restore.confirmBody":
    "Der Container wird gestoppt, seine Appdata ersetzt und aus dem Backup neu erstellt. Fortfahren?",
  "restore.confirm": "Bestätigen",
  "restore.cancel": "Abbrechen",
  "restore.preview": "Vorschau",
  "restore.started": "Wiederherstellung gestartet",

  "run.kindBackup": "Backup",
  "run.kindRestore": "Wiederherstellung",
  "run.statusRunning": "Läuft",
  "run.statusSuccess": "Erfolgreich",
  "run.statusFailed": "Fehlgeschlagen",
  "run.historyTitle": "Ausführungsverlauf",
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
  "settings.paths": "Backup-Pfade",
  "settings.containersPath": "Container-Pfad",
  "settings.vmsPath": "VMs-Pfad",
  "settings.flashPath": "Flash-Pfad",
  "settings.offsiteTitle": "Offsite-Kopie (optional)",
  "settings.offsiteHint": "Nach jedem erfolgreichen lokalen Backup wird es zusätzlich per restic copy in ein zweites Repo repliziert. Ein Remote (rest:http://host:8000/repo, s3:…, b2:…) oder einen lokalen Unterpfad angeben; leer lassen zum Deaktivieren. Das lokale Backup bleibt primär.",
  "settings.domains": "Domänen",
  "settings.containersEnabled": "Container",
  "settings.vmsEnabled": "VMs",
  "settings.flashEnabled": "Flash",
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
  "notify.unraid": "Unraid-Benachrichtigungen",
  "notify.unraidHint": "An Unraids eigenes Benachrichtigungssystem senden (das an Pushover, E-Mail, Discord, … weiterleiten kann). Erfordert die eingerichtete SSH-Verbindung (Einstellungen → VM Backup over SSH).",
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
  "integrity.pruneHint": "Speicherplatz gelöschter/vergessener Backups freigeben (kann dauern).",
  "integrity.pruneConfirm": "Aufräumen gibt Speicher gelöschter Backups frei und kann einige Minuten dauern. Fortfahren?",

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
  "jobs.noVMs": "Noch keine VMs",
  "jobs.noContainersIncluded": "Keine Container im Zeitplan enthalten.",
  "jobs.flashRow": "Unraid Flash-Konfiguration",
  "jobs.flashPlanned": "geplant",
  "jobs.vmPlanned": "VM-Backup-Executor noch nicht implementiert.",

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
  "common.loadingBackups": "Sicherungen werden geladen…",
  "common.saving": "Speichern…",

  // VMs page
  "vms.title": "Virtuelle Maschinen",
  "vms.subtitle": "VM-Backups, Zeitpläne und Wiederherstellungen verwalten.",
  "vms.empty": "Keine VMs gefunden. Läuft libvirt/KVM?",
  "vms.backupSelected": "Auswahl sichern",
  "vms.restoreSelected": "Auswahl wiederherstellen (neuestes)",
  "vms.restoreSelectedConfirm": "Das NEUESTE Backup der ausgewählten VMs wiederherstellen? Jede VM wird heruntergefahren, ihre Disk-Dateien ersetzt und die VM wiederhergestellt.",
  "vms.notInstalledHint": "Diese VMs sind nicht mehr auf dem Host definiert, haben aber noch Backups. Stelle sie wieder her oder sieh ihre Snapshots im Backups-Panel ein.",
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
