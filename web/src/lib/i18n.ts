// ---------------------------------------------------------------------------
// i18n — React Context-based, 42 locales, flag switcher support
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
import bg from "./locales/bg";
import sk from "./locales/sk";
import sl from "./locales/sl";
import hr from "./locales/hr";
import sr from "./locales/sr";
import lt from "./locales/lt";
import lv from "./locales/lv";
import et from "./locales/et";
import is from "./locales/is";
import ca from "./locales/ca";
import gl from "./locales/gl";
import eu from "./locales/eu";
import id from "./locales/id";
import ms from "./locales/ms";
import hi from "./locales/hi";
import fa from "./locales/fa";

// ---------------------------------------------------------------------------
// Translation key set — en is the source of truth
// (exported so the locale-parity test can import the base table)
// ---------------------------------------------------------------------------

export const en = {
  // General
  "language.label": "Language",
  "theme.dark": "Dark",
  "theme.light": "Light",

  // Nav
  "nav.dashboard": "Dashboard",
  "nav.containers": "Containers",
  "nav.vms": "VMs",
  "nav.flash": "Flash",
  "nav.config": "Self-Backup",
  "nav.receiver": "Receiver",
  "nav.fleet": "Fleet",
  "nav.settings": "Settings",
  "nav.reportBug": "Report a bug",

  // Mode toggle
  "mode.simpleView": "Simple view",
  "mode.advancedView": "Advanced view",

  // Dashboard
  "dashboard.title": "Dashboard",
  "dashboard.subtitle": "Your backup status at a glance.",
  "dashboard.summaryHealth": "Overall health",
  "dashboard.summaryNextBackup": "Next backup",
  "dashboard.summaryLastResult": "Last result",
  "dashboard.lastBackups": "Last Backups",
  "dashboard.recentRuns": "Recent Runs",
  "dashboard.spikeStatus": "System Status",
  "dashboard.noRuns": "No runs yet",
  "dashboard.spikeLink": "Run host-integration check",
  "dashboard.hostIntegrationCheck": "Host Integration Check",
  "dashboard.allOk": "All systems OK",
  "dashboard.degraded": "Degraded",
  "dashboard.checking": "Checking…",
  "dashboard.noContainers": "No containers found.",

  // Dashboard customization (#46) — reorder + hide cards, persisted per-browser
  "dashboard.customize": "Customize",
  "dashboard.customizeDone": "Done",
  "dashboard.customizeHint": "Drag a card to reorder it, or hide the ones you don't need. Saved in this browser.",
  "dashboard.moveUp": "Move up",
  "dashboard.moveDown": "Move down",
  "dashboard.hideCard": "Hide",
  "dashboard.showCard": "Show",
  "dashboard.makeHalfWidth": "Make half width",
  "dashboard.makeFullWidth": "Make full width",
  "dashboard.hiddenCards": "Hidden cards",
  "dashboard.resetLayout": "Reset to default",
  "dashboard.blockSummary": "Overview",
  "dashboard.blockStats": "Statistics",
  "dashboard.blockBackups": "Backups & history",

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
  "spike.info": "INFO",
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
  "containers.scheduleHint": "Included containers back up one after another from the scheduled time. They share one repository, so they run in sequence, not all at once.",
  "containers.notInstalled": "Not installed",
  "containers.notInstalledTitle": "Not installed (backups only)",
  "containers.notInstalledHint": "These containers are no longer installed but still have backups. Restore them, or delete their backups to free space.",
  "containers.deleteBackups": "Delete all backups",
  "containers.deleteBackupsConfirm": "Delete ALL backups of this container? The snapshots are permanently removed from the repository and cannot be undone.",
  "containers.filter": "Filter:",
  "containers.filterAll": "All",
  "containers.filterInstalled": "Installed",
  "containers.sectionsLabel": "Sections",
  "containers.selectAll": "Select all",
  "containers.selectedCount": "selected",
  "containers.backupSelected": "Back up selected",
  "containers.restoreSelected": "Restore selected (latest)",
  "containers.restoreSelectedConfirm": "Restore the LATEST backup of the selected containers? Each is stopped, its appdata replaced, and the container recreated.",
  "containers.clearSelection": "Clear",
  "containers.working": "Working…",
  "containers.batchStarted": "Backup started. It runs on the server, so you can close this tab.",
  "containers.batchAlreadyRunning": "A batch backup is already running.",
  "containers.batchRunning": "Backing up selected containers…",
  "containers.selfNote": "This is BombVault. It doesn't back up its own container (that would stop itself); its settings are recovered via Discover.",

  // Post-backup update-check line (G4)
  "containers.updateCheckLabel": "Update check",
  "containers.updateCheckUpToDate": "up to date",
  "containers.updateCheckUpdated": "updated",
  "containers.updateCheckFailed": "check failed",

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
  "restore.cancel": "Cancel restore",
  "restore.cancelConfirmSafe": "Cancel the restore? The partial output folder is left as-is.",
  "restore.cancelConfirmInPlace":
    "{name} is mid-restore. Cancelling leaves this restore partial. You may need to restore it again. Cancel anyway?",
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
    "Running in the background. You can close this panel; the outcome appears in the run history.",
  "restore.completeContainer": "Restore complete: container is being recreated.",
  "restore.completeVM": "Restore complete: VM disks have been replaced.",
  "restore.recreateComplete": "Recreate complete: the container has been recreated.",

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
  "stack.restoreFinished": "Stack restore finished. See the run history for per-container results.",
  "stack.memberRestored": "restored",
  "stack.memberStarted": "started",

  // Runs
  "run.kindBackup": "Backup",
  "run.kindRestore": "Restore",
  "run.kindUpdate": "Update",
  "run.kindExport": "Export",
  "run.kindDRDrill": "DR check",
  "update.afterBackup": "Update after successful backup",
  "update.afterBackupHint": "Pull the image and recreate this container right after a successful backup, so you always have a fresh restore point first. It runs at the backup's time (backups run one after another), not a fixed clock time. For updates on a set schedule instead, version-gated, see ShipLog.",
  "run.statusRunning": "Running",
  "run.statusSuccess": "Success",
  "run.statusFailed": "Failed",
  "run.statusSkipped": "Skipped",
  "run.historyTitle": "Run History",
  "run.filterDay": "Day:",
  "run.allDays": "All days",
  "run.recentTitle": "Recent runs",
  "run.colKind": "Kind",
  "run.colStatus": "Status",
  "run.colStarted": "Started",
  "run.colFinished": "Finished",
  "run.colContainer": "Container",

  // Settings
  "settings.title": "Settings",
  // `settings.encryption` ("Encryption"), the old standalone <h3> heading
  // above this sub-section, is RETIRED (jdp, live-review, GlimStone
  // follow-up round: "Export und Verschlüsselung: Texte normal formatieren,
  // es sind keine Überschriften mehr") — the ToggleRow below now shows its
  // own dynamic on/off label directly (settings.encryptionOn/Off), which
  // already says more than the static generic word this key held, so a
  // second, now-unused heading string would just be dead weight.
  "settings.encryptionLabel": "Password",
  "settings.encryptionOn": "Enabled (password derived from APP_KEY)",
  "settings.encryptionOff": "Disabled (no password)",
  "settings.encryptionHint":
    "Encryption is fixed per repository when it is created, so this only decides how a NEW repository is made. For a repository that already exists BombVault reads the mode off the repository itself. See the Recovery page. Changing this against an existing repository just stops restic from opening it.",

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
  "settings.pathsHint": "Relative subpaths under the host mount root ({root}). Click Browse to navigate directories or type a path directly.",
  "settings.containersPath": "Containers path",
  "settings.vmsPath": "VMs path",
  "settings.flashPath": "Flash path",
  "settings.configPath": "Self-Backup path",
  "settings.restoreFolder": "Default restore folder",
  "settings.restoreFolderHint": "Where 'restore to a folder' extracts snapshots by default.",
  // Inline Local/Remote mode switch on each backup path field (issue #152): a
  // path already accepts a restic remote URL directly, so switching to Remote
  // opens the connection-test/safety-settings dialog instead of the folder browser.
  "settings.pathMode.local": "Local",
  "settings.pathMode.remote": "Remote",
  // Hover/focus explanation for the icon-only Local/Remote segments above
  // (GlimStone follow-up round, point 2) — removing the text label loses the
  // meaning, so each glyph gets a fuller InfoBubble-style tooltip instead.
  "settings.pathMode.localTip": "Local path on this host",
  "settings.pathMode.remoteTip": "Remote restic repository",
  "settings.primaryRemote.title": "Remote primary safety settings",
  "settings.primaryRemote.hint": "This backup path is a remote restic repository. It IS the primary copy, not a replica. Configure bandwidth limits, append-only protection and a growth-budget alarm for it here, the same protections an off-site copy gets.",
  "settings.primaryRemote.budgetHint": "Alarm when this repository grows past a byte budget (0 = off).",
  "settings.offsiteTitle": "Off-site copy (optional)",
  "settings.offsiteHint": "After each successful local backup, also replicate it to a second repo with restic copy. Enter a remote (rest:http://host:8000/repo, s3:…, b2:…) or a local subpath; leave blank to disable. The local backup stays primary.",
  "source.label": "Source:",
  "source.local": "Local",
  "source.offsite": "Off-site",
  // Hover/focus explanation for SourceToggle's icon-only Local/Off-site
  // segments (jdp, live-review, icon-badge standing rule) — same reasoning
  // as settings.pathMode.localTip/remoteTip above: losing the visible text
  // label needs a fuller tooltip to carry the same meaning.
  "source.localTip": "Local repository on this host",
  "source.offsiteTip": "Off-site replica",
  // Only rendered when a domain has 2+ off-site targets: the picker choosing
  // WHICH off-site copy to browse/restore from.
  "source.offsiteTarget": "Off-site target",
  "source.hint": "Restore and delete act on the selected source only: deleting a local backup never touches the off-site copy, and vice versa.",
  // jdp (live-review, offsite tab card split): "Können wir für Container, VMs,
  // Flash, Ordner jeweils eine eigene Card machen? ... Titel dann jeweils
  // OFFSITE-KOPIE CONTAINER, OFFSITE-KOPIE VMS, etc." — one templated key
  // (not four discrete ones) reused across all four new per-domain Cards,
  // {domain} filled in via .replace() at each call site with the SAME
  // nav.containers/vms/flash/files label the row already showed before the
  // split (see Settings.tsx's offsite-tab map) — matching this file's own
  // `.replace("{x}", ...)` convention (settings.pathsHint, jobs.cadenceDaily,
  // etc.) rather than translating "Containers"/"VMs"/"Flash"/"Folders" a
  // second time across 26 locales. The group heading (offsite.sectionTitle)
  // this replaces is now gone — see that call site's own comment for why.
  "offsite.copyDomainTitle": "Off-site copy {domain}",
  "offsite.schedulePlaceholder": "blank = after each backup · e.g. weekly Sun 03:00",
  "offsite.replicateNow": "Replicate now",
  "offsite.replicateStarted": "Replication started - it runs in the background; the running indicator shows progress.",
  "offsite.replicating": "Replicating…",
  // Issue #159: appended next to offsite.replicating when the live progress
  // event carries a startedAt (see OffsiteIndicator) but no live per-snapshot
  // percentage is available YET (e.g. restic is still walking the source tree
  // before it starts copying packs) — a live elapsed duration is shown in its
  // place, same as before percentage support existed.
  "offsite.replicatingWithDuration": "Replicating… ({duration})",
  // Issue #159: restic copy DOES print a real, parseable per-snapshot
  // pack-copy percentage (see restic.Copy's doc comment), and lib/progress.ts's
  // offsiteRunProgress folds it into the snapshot count to get ONE run-level
  // figure. Word order matters here and is not free styling: the first cut read
  // "Replicating snapshot 15 of 126 (55%)", where the parenthetical was the
  // CURRENT snapshot's own pack progress but every reader took it as
  // "15/126 = 55%" — two correct numbers rendered as one wrong claim. With the
  // percentage leading and "snapshot k of N" demoted to the parenthetical, the
  // two now agree (~12% either way) instead of fighting. Do not reorder them
  // back. The "overall" wording is what makes the scope explicit — keep an
  // equivalent in every locale.
  "offsite.replicatingSnapshotPercent": "Replicating… {percent}% overall (snapshot {index} of {total})",
  "offsite.replicatingSnapshotPercentWithDuration": "Replicating… {percent}% overall (snapshot {index} of {total}) · {duration}",
  // The (i) next to that readout (OffsiteIndicator). The percentage counts
  // SNAPSHOTS against a best-effort estimate, not bytes — house rule says that
  // caveat belongs behind an info bubble, not as permanent prose on the line.
  "offsite.overallPercentHint": "Overall progress for this replication run, counted in snapshots: restic only ever reports progress for one snapshot at a time, never for a whole copy. Snapshots differ in size, so treat this as an estimate.",
  "offsite.replicateFailed": "Replication failed",
  "offsite.test": "Test connection",
  // Shown instead of offsite.test once the domain has additional targets: that
  // button probes the PRIMARY target ONLY, and saying so is the whole point.
  "offsite.testPrimary": "Test primary",
  "offsite.testing": "Testing…",
  "offsite.testOk": "reachable + initialised",
  "offsite.testUninitialized": "reachable, not initialised",
  "offsite.testFailed": "not reachable",
  "offsite.repoLocalHint": "Also accepts a plain folder under the \"Host Data\" mount. Enter it relative to that mount, without the leading /mnt: a share at /mnt/remotes/nas/bombvault is entered as remotes/nas/bombvault.",
  // Off-site setup wizard (v4 ransomware protection)
  "offsite.wizard.setup": "Set up…",
  "offsite.wizard.close": "Close",
  "offsite.wizard.step1": "1 · Choose a backend",
  "offsite.wizard.backendRest": "rest-server (recommended, append-only capable)",
  "offsite.wizard.backendRclone": "rclone remote",
  "offsite.wizard.backendS3": "Amazon S3 / S3-compatible",
  "offsite.wizard.backendPath": "Local path / mounted share (no server needed)",
  "offsite.wizard.step2": "2 · Deploy the append-only server",
  "offsite.wizard.step2Hint": "Run this on your storage box to stand up a restic rest-server with --append-only. The generated password is shown only once.",
  "offsite.wizard.generate": "Generate deployment snippet",
  "offsite.wizard.regenerate": "Regenerate (new password)",
  "offsite.wizard.snippetError": "Could not generate the snippet",
  "offsite.wizard.passwordWarning": "This password is shown ONCE and is never stored by BombVault. Save it now. You need it for the credentials below and it cannot be recovered.",
  "offsite.wizard.tlsNote": "This recipe uses plain HTTP, fine on a trusted LAN or VPN. If the storage box is reachable over the internet, put rest-server behind HTTPS (a TLS reverse proxy) so the repository credential isn't sent in the clear.",
  "offsite.wizard.password": "Generated password (save this)",
  "offsite.wizard.step3": "3 · Repository URL + credentials",
  "offsite.wizard.repoUrl": "Off-site repository URL",
  "offsite.wizard.repoUrlPlaceholder": "rest:http://192.168.x.x:8000/bombvault-containers/containers",
  "offsite.wizard.credentials": "REST server credentials",
  "offsite.wizard.credLoadError": "Could not load existing credentials. Reload before editing.",
  "offsite.wizard.step4": "4 · Enable append-only protection",
  "offsite.immutable": "Immutable (append-only)",
  "offsite.immutableHint": "BombVault stops pruning/deleting off-site and lets the far side enforce retention. The far side must actually refuse deletes, verified below.",
  "offsite.rcloneWarning": "rclone serve restic --append-only has an open upstream retry bug that can drop appends. rest-server is recommended for immutable off-site.",
  "offsite.s3Unverified": "Note: S3 append-only can't be verified automatically. Set bucket versioning + deny-delete manually; the scorecard keeps this domain marked unverified.",
  "offsite.tamperTestNow": "Test append-only now",
  "offsite.tamperTesting": "Testing…",
  // Tamper verdicts carry NO ✓/✗ glyph — OffsiteWizard.tsx renders the glyph
  // as its own JSX node so RTL locales (ar/he) place it correctly.
  "offsite.tamperOk": "delete refused, append-only active",
  "offsite.tamperFail": "server ACCEPTED the delete, NOT protected",
  "offsite.tamperUnverifiable": "not verifiable for this repo type",
  "offsite.tamperError": "Tamper test inconclusive (server unreachable)",
  "offsite.retention.title": "5 · Retention strategy",
  "offsite.retention.farside": "Far-side prune (recommended)",
  "offsite.retention.window": "Maintenance window",
  "offsite.retention.grow": "Grow + budget alarm",
  "offsite.retention.farsideHint": "Run restic forget --prune on the storage box itself (BombVault stays append-only). Cron hint:",
  "offsite.retention.windowHint": "Temporarily run a second, non-append-only rest-server, prune, then shut it down. Credentials are never persisted and a mandatory tamper re-test follows. Use only when far-side prune is not possible.",
  "offsite.retention.windowRestOnly": "Only applies to REST-server destinations. This backend can't run a temporary second server against it. Use Far-side prune or Grow + budget alarm instead.",
  "offsite.retention.growHint": "Never prune off-site; instead alarm when the repo grows past a byte budget. The honest default until you pick a prune path.",
  "offsite.retention.budget": "Growth budget (GB, 0 = off)",

  // Additional off-site targets (multi-off-site) — extra per-domain copies
  "offsite.targets.title": "Additional off-site targets",
  "offsite.targets.hint": "Replicate this domain to more than one off-site location. The primary target above is edited separately; the ones you add here are extra copies.",
  "offsite.targets.scheduleNote": "All targets for this domain replicate on this domain's off-site schedule. There is no separate per-target schedule.",
  "offsite.targets.add": "Add target",
  "offsite.targets.none": "No additional targets yet.",
  "offsite.targets.edit": "Edit",
  // Per-target probe (issue #138): the primary editor's Test button never
  // covered these, so a broken extra copy could hide behind its green verdict.
  "offsite.targets.test": "Test",
  "offsite.targets.remove": "Remove",
  "offsite.targets.removing": "Removing…",
  "offsite.targets.confirmRemove": "Confirm remove",
  "offsite.targets.save": "Save target",
  "offsite.targets.cancel": "Cancel",
  "offsite.targets.name": "Name (optional)",
  "offsite.targets.credsLabel": "Credentials",
  "offsite.targets.credsDefault": "Shared (default)",
  "offsite.targets.namePlaceholder": "e.g. Backblaze B2",
  "offsite.targets.repoRequired": "Enter a repository URL.",
  "offsite.targets.loadError": "Could not load off-site targets.",
  "offsite.targets.retentionTitle": "Retention (0 = keep all)",

  "settings.domains": "Domains",
  "settings.domainsHint": "Turn each backup domain on or off. Enabling VMs or Flash reveals its tab in the sidebar.",
  "settings.containersEnabled": "Containers",
  "settings.containersEnabledHint": "Back up and restore Docker containers, BombVault's core domain, enabled by default.",
  "settings.vmsEnabled": "VMs",
  "settings.vmsEnabledHint": "Back up and restore virtual machines over SSH via libvirt.",
  "settings.flashEnabled": "Flash",
  "settings.flashEnabledHint": "Back up the Unraid USB flash boot drive (/boot) to a restic repository.",
  "settings.configEnabled": "Self-Backup",
  "settings.configEnabledHint": "Back up BombVault's own settings, targets, and credentials, so a fresh install can restore its configuration too (self-backup).",
  "settings.receiverEnabled": "Receiver dashboard",
  "settings.receiverEnabledHint": "Monitor an append-only off-site repo another BombVault pushes to (read-only)",
  "settings.fleetEnabled": "Fleet view",
  "settings.fleetEnabledHint": "Watch the protection status of peer BombVault instances (read-only)",
  "settings.schedule": "Schedule",
  "settings.scheduleOff": "off",
  "settings.language": "Language",
  "settings.theme": "Theme",
  "settings.save": "Save",
  "settings.saved": "Settings saved",
  "settings.error": "Error saving settings",

  // Retention
  "settings.retentionTitle": "Snapshot retention",
  "settings.retentionHint": "How many backups to keep per item. After each backup, restic prunes older snapshots to this policy. All zero = keep everything (off).",
  // Merged card (GlimStone follow-up round, Paths & Storage tab rework, merge
  // A) — image cleanup, Unraid's own update-status reconciliation, and
  // private registry credentials all sit under one roof: everything the
  // post-backup container update pull touches.
  "settings.imageMaintenanceTitle": "Image Cleanup & Update Status",
  "settings.imageMaintenanceHint": "Housekeeping for the post-backup container update: prune the superseded image and refresh Unraid's own cached update status.",
  "settings.pruneImageAfterUpdate": "Remove the old image after an update",
  "settings.pruneImageAfterUpdateHint": "After a container is updated to a newer image, delete the superseded old image. Off by default. Keeping it makes rolling back cheap (a BombVault snapshot restores data, not the old image). A shared base image is never deleted.",
  "settings.registriesTitle": "Container registries",
  "settings.registriesHint": "Credentials for private registries, used when \"update after backup\" checks a container for a newer image (for example a sponsor-gated ghcr.io image). Public images keep pulling anonymously. Leave a token blank to keep the saved one; removing a row deletes its credential.",
  "settings.registriesEmpty": "No registries configured. All images are pulled anonymously.",
  "settings.registryHost": "Registry host",
  "settings.registryUser": "Username",
  "settings.registryToken": "Token / password",
  "settings.registryRemove": "Remove",
  "settings.registryAdd": "Add registry",
  "settings.retentionLast": "Keep last",
  "settings.retentionDaily": "Keep daily",
  "settings.retentionWeekly": "Keep weekly",
  "settings.retentionMonthly": "Keep monthly",
  "settings.retentionLastInfo": "Keeps the N most recent snapshots, no matter when they were made.",
  "settings.retentionDailyInfo": "Keeps one snapshot for each of the last N calendar days that has a backup, one per day, not N backups.",
  "settings.retentionWeeklyInfo": "Keeps one snapshot for each of the last N calendar weeks that has a backup.",
  "settings.retentionMonthlyInfo": "Keeps one snapshot for each of the last N calendar months that has a backup.",
  "settings.retentionCombineInfo": "The four rules combine with OR: a snapshot survives if any single rule would keep it. They don't add up to a fixed count. Applied separately to each backed-up item.",
  "settings.retentionLocal": "Local repo",
  "settings.retentionOffsite": "Off-site repo",
  "settings.retentionOffsiteTitle": "Off-site retention",
  "settings.retentionOffsiteHint": "A separate policy for the off-site repo, so you can keep it longer as an archive. All zero = keep every off-site backup (no off-site pruning).",
  "settings.retentionOffsiteImmutableInfo": "An immutable off-site destination is never pruned from here, no matter these settings. See Off-site › Retention strategy for how to prune it.",

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

  // Dashboard widget (embeddable activity log)
  "settings.widget": "Dashboard widget",
  "settings.widgetHint": "Embed BombVault's activity log in Homepage, Organizr, Heimdall or any dashboard that can show an iframe: a tiny standalone page with just the log.",
  "settings.widgetHow": "Paste the URL into your dashboard's iframe widget. That is all it needs.",
  "settings.widgetAccess": "The token grants read-only access to the activity log and nothing else; Disable revokes it immediately.",
  "settings.widgetEnglish": "The widget page itself is English-only. An Unraid dashboard variant may come later.",
  "settings.widgetGenerate": "Generate token",
  "settings.widgetRegenerate": "Regenerate",
  "settings.widgetDisable": "Disable",
  "settings.widgetToken": "Widget token",
  "settings.widgetUrl": "Widget URL",
  "settings.widgetUrlOnce": "The URL (with its token) is shown only right after generating. Regenerate to get a fresh one.",
  "settings.widgetPreview": "Live preview",
  "settings.fleet": "Fleet view",
  "settings.fleetHint": "Let other BombVault instances' Fleet views poll this one's protection status, read-only. Nothing here lets a peer trigger any action.",
  "settings.instanceName": "Instance name",
  "settings.fleetHow": "Generate a token here, then paste this instance's URL and the token into the PEER's own Fleet page (Settings → Fleet on the box doing the watching).",
  "settings.fleetAccess": "The token grants read-only access to this instance's protection scorecard and nothing else; Disable revokes it immediately.",
  "settings.fleetToken": "Fleet token",
  "settings.fleetRegenerate": "Regenerate",
  "settings.fleetDisable": "Disable",
  "settings.fleetGenerate": "Generate token",
  "settings.fleetTokenOnce": "The token is shown only right after generating. Regenerate to get a fresh one.",
  "settings.fleetTokenPasteHint": "Paste this into the peer instance's Fleet page, along with this instance's URL.",
  "settings.fleetUrlHint": "This instance's own URL (for a peer to enter): {url}",
  "settings.dashTile": "Unraid dashboard widget",
  "settings.dashTileHint": "Put a BombVault status widget right on the Unraid Dashboard. A small companion plugin adds the widget; BombVault can install it for you over its existing host SSH connection.",
  "settings.dashTileChecking": "Checking plugin status…",
  "settings.dashTileNoSsh": "The host SSH connection is not set up (Settings → VM Backup over SSH), so BombVault cannot install the plugin for you. Install it manually on Unraid under Plugins → Install Plugin with this URL:",
  "settings.dashTileCa": "It may also be available in Community Applications. Search for BombVault Widget.",
  "settings.dashTileNotInstalled": "The dashboard widget plugin is not installed.",
  "settings.dashTileConfirm": "Install installs the bombvault-widget plugin on your Unraid host via the regular Unraid plugin mechanism. It shows up under Plugins like any other plugin and can be removed there (or here) anytime.",
  "settings.dashTileRepo": "View the plugin on GitHub",
  "settings.dashTileInstall": "Install plugin",
  "settings.dashTileInstalling": "Installing…",
  "settings.dashTileInstalled": "Dashboard widget plugin is installed ({version})",
  "settings.dashTileInstalledNoV": "Dashboard widget plugin is installed",
  "settings.dashTileInstallOk": "Plugin installed. Open the Unraid Dashboard to see the new BombVault widget.",
  "settings.dashTileInstalledHint": "The widget appears on the Unraid Dashboard. If it is not visible, enable it in the Dashboard's widget management.",
  "settings.dashTileRemove": "Remove plugin",
  "settings.dashTileRemoving": "Removing…",
  "settings.dashTileRemoveOk": "Plugin removed. It no longer appears on the Unraid Dashboard.",

  // Off-site (rclone)
  "rclone.title": "Off-site (rclone)",
  "rclone.hint": "Paste an rclone config to back up to the cloud (Backblaze B2, S3, Google Drive, …). It is stored encrypted. SMB/NFS need no rclone: mount the share on Unraid and set a Backup Path to it.",
  "rclone.configured": "Configured remotes",
  "rclone.pathHint": "Then set a Backup Path to \"rclone:<remote>:<bucket>/path\" to send that domain off-site.",
  "cloud.title": "Cloud credentials (S3 / restic REST)",
  "cloud.hint": "Credentials for off-site restic backends, without rclone. After saving, set a Backup Path to a remote repo, e.g. s3:s3.amazonaws.com/bucket/path, rest:http://host:8000/repo, b2:bucket:path or sftp:user@host:/repo. Secrets are stored encrypted and never shown again.",
  "cloud.secretSet": "saved (leave blank to keep)",
  "cloud.storageClass.label": "Off-site storage class",
  "cloud.storageClass.default": "(provider default)",
  "cloud.storageClass.hint": "Applies to native S3 backends only (repositories that start with s3:). Deep-archive tiers (Glacier Flexible Retrieval, Deep Archive) are intentionally not offered because they break restic restore.",
  "cloud.credSets.title": "Additional credential sets",
  "cloud.credSets.hint": "Give one or more off-site targets their own S3 or restic REST credentials, instead of sharing the ones above.",
  "cloud.credSets.add": "Add credential set",
  "cloud.credSets.name": "Name",
  "cloud.credSets.none": "No additional credential sets yet.",
  "rclone.save": "Save config",
  "notify.title": "Notifications",
  "notify.hint": "Get notified when a backup finishes, and choose which events trigger it below. Unraid notifications work here in Simple mode; more delivery channels (webhook, Matrix, Healthchecks, email) live under Advanced.",
  "notify.on": "Notify",
  "notify.onNever": "Never",
  "notify.onFailure": "Only on failure",
  "notify.onAlways": "On success and failure",
  "notify.channelsTitle": "Notification channels",
  "notify.channelsHint": "Configure the webhook, Matrix and email channels that deliver the notifications configured above.",
  "notify.webhook": "Webhook URL",
  "notify.webhookChannel": "Webhook",
  "notify.webhookFormat": "Webhook format",
  "notify.apprise": "Apprise",
  "notify.appriseUrl": "Apprise API URL",
  "notify.appriseTags": "Tags (optional)",
  "notify.appriseHint": "Point at your Apprise API container's /notify/<key> endpoint to fan out to its 100+ services (Telegram, Pushover, Signal, …). Tags route to a subset of that key's targets; leave blank for all.",
  "notify.matrix": "Matrix",
  "notify.matrixHomeserver": "Homeserver URL",
  "notify.matrixToken": "Access token",
  "notify.matrixRoom": "Room ID",
  "notify.healthchecksTitle": "Healthchecks",
  "notify.healthchecks": "Healthchecks.io ping URL",
  "notify.healthchecksLifecycle": "Healthchecks is pinged for the whole backup lifecycle (start, success and failure) whenever a URL is set, independent of the 'notify on' setting above, so the check stays green on success even with failure-only notifications.",
  "notify.hcPerDomain": "Per-domain checks (advanced)",
  "notify.hcPerDomainHint": "Leave a field blank to use the global URL above. A domain with its own URL gets its own check, with its own runtime and history.",
  "notify.scheduledSummary": "Summarise scheduled runs",
  "notify.scheduledSummaryHint": "Send ONE summary per scheduled backup run (e.g. \"42 of 45 succeeded\") instead of a separate message for every container/VM. Healthchecks is already summarised. Manual backups still notify per item.",
  "notify.notifyOnUpdate": "Notify on container update",
  "notify.notifyOnUpdateHint": "When \"update after backup\" upgrades a container to a newer image, send a message so you can verify it still works. Fires per updated container (updates are rare).",
  "notify.unraid": "Unraid notifications",
  "notify.unraidHint": "Send to Unraid's own notification system (which can forward to Pushover, email, Discord, …). It runs over the SSH connection from Settings → VM Backup over SSH, so the key must be authorised there, but libvirt/VMs are NOT required (ignore a \"libvirt not reachable\" result if you don't back up VMs). Use Send test below to check it.",
  "notify.unraidPlatformMismatch": "BombVault detected this host as \"{platform}\", not Unraid. Unraid notifications stay off even with this switched on. If this IS an Unraid host, check that the host's /boot is bind-mounted to /host/boot inside the container (see the BombVault Unraid template) and restart the container.",
  "notify.smtp": "Email (SMTP)",
  "notify.smtpHost": "SMTP host",
  "notify.smtpPort": "Port",
  "notify.smtpUser": "Username",
  "notify.smtpPass": "Password",
  "notify.smtpFrom": "From address",
  "notify.smtpTo": "To address",
  "notify.smtpTls": "Encryption",
  "notify.test": "Send test",
  "notify.tested": "Test sent",

  // Integrity (restic check)
  "integrity.title": "Integrity & maintenance",
  "integrity.hint": "Verify a repository's structure (restic check), clear stale locks left by an interrupted run, or prune to reclaim space from deleted backups.",
  "integrity.verify": "Verify",
  "integrity.checking": "Checking…",
  "integrity.ok": "Healthy",
  "integrity.failed": "Check failed",
  // Minimal per-button fail glyph (GlimStone standing rule: failures toast,
  // the button itself only shows a brief fixed indicator matching
  // integrity.ok's own visual weight — see IntegrityCard's run()).
  "integrity.failedShort": "✗ Failed",
  "integrity.unlock": "Unlock",
  "integrity.prune": "Prune",
  "integrity.verifyHint": "Run restic check to verify structure and metadata are intact.",
  "integrity.unlockHint": "Clear stale repository locks left by a crashed or interrupted run (fixes 'repository is already locked').",
  "integrity.pruneHint": "Apply your retention policy and reclaim space (reclaims space only when no policy is set; can take a while).",
  "integrity.pruneConfirm": "Prune now applies your retention policy. It removes snapshots beyond your keep rules (last/daily/weekly/monthly) and reclaims space. With no policy set it only reclaims space. Continue?",
  // #109: the off-site wizard's tamper test, surfaced in Integrity & maintenance
  // under its plainer name. The verdict strings are shared (offsite.tamperOk/…).
  "integrity.appendOnly": "Append-only check",
  "integrity.appendOnlyHint": "Prove the off-site repository still refuses deletes (append-only protection), the same check as the off-site wizard's tamper test.",
  "integrity.appendOnlyLast": "append-only protection · Last checked {time}",
  "integrity.appendOnlyNever": "append-only protection · never checked",

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
  "drill.targetVM": "Drill target (VM)",
  "drill.targetMostRecent": "Most recent backup",
  "drill.drNote": "A real restore extracts the newest off-site snapshot into a temporary sandbox, verifies it, then cleans up. It downloads real data and can take a while.",
  "drill.runDR": "Run real restore",
  "drill.runningDR": "Restoring…",
  "drill.confirmDR": "This performs a REAL restore of the newest off-site snapshot into a temporary sandbox to prove it is recoverable, then deletes it. It downloads real data and can take a while. Continue?",
  "drill.provenOffsite": "proven restorable from off-site",
  "drill.offsiteVerified": "off-site verified",

  // Drill reason + check labels (#30)
  "drill.checkOffsiteDr": "off-site DR restore",
  "drill.checkLocal": "local integrity check",
  "drill.failReasonPrefix": "reason:",
  "drill.runOffsiteDr": "Run off-site DR check",
  "drill.rerunOffsiteDr": "Re-run off-site DR check",
  "drill.runningOffsiteDr": "Running off-site DR check…",

  // Off-site DR opt-out (#37)
  "settings.offsiteDrills": "Scheduled off-site DR drill",
  "settings.offsiteDrillsHelp": "Restores the full off-site snapshot on the drill schedule to prove remote recovery. This re-downloads the whole backup each run, which costs egress on metered clouds (for example Backblaze B2). Turn off to keep only the free local integrity check and run the off-site DR check manually.",
  "drill.manualOnly": "Off-site DR: manual only",
  "drill.manualOnlyTitle": "Scheduled off-site DR drill is off. Run the off-site check manually with the button.",

  // Pre/post-backup hooks
  "hooks.title": "Backup hooks",
  "hooks.hint": "Commands run inside the container with sh -c. The pre-command runs before the backup; use it to prepare data that should be backed up, for example dumping a database into the container's appdata. If the pre-command fails, the backup is aborted. The post-command runs after the container is started again and its failure is only logged. Hooks only run commands, they do not add extra folders to the backup.",
  "hooks.pre": "Pre-backup command",
  "hooks.post": "Post-backup command",
  "folders.title": "Backup folders",
  "folders.hint": "Choose which of this container's mapped folders to back up. The appdata folder is selected by default. Tick others to include them, or add a custom path under the host mount. Unticking everything reverts to the automatic appdata default.",
  "folders.appdataDefault": "appdata (default)",
  "folders.notReachable": "not under the host mount, can't be backed up",
  "folders.customMissing": "no data folder detected (nothing to back up here)",
  "folders.customPlaceholder": "/mnt/user/some/folder",
  "folders.addCustom": "Add a folder path",
  "folders.add": "Add",
  "folders.save": "Save folders",
  "folders.saved": "Saved",
  "folders.empty": "No mapped folders found for this container.",
  "stophook.title": "Stop other containers",
  "stophook.hint": "Stop these other containers while this one is backed up (for example a database), then start them again afterwards.",
  "stophook.noCandidates": "No other installed containers found.",
  "stophook.remove": "Remove {name}",
  "export.button": "Export (plain tar)",
  "export.exportedTo": "Exported to:",
  "backup.configOnly": "Config only: no data folders (definition saved for recreate)",

  // Per-container exclude patterns (#36)
  "excludes.title": "Exclude patterns",
  "excludes.hint": "One pattern per line. A container path (e.g. /config/Library/.../Cache) is matched against the backed-up volume; a bare name like .git matches at any depth. Brace lists like {a,b} are not supported; use one line each.",
  "excludes.placeholder": "/config/Library/Application Support/Plex Media Server/Cache\n/config/Library/Application Support/Plex Media Server/Metadata\n.git",
  "excludes.save": "Save excludes",
  "excludes.saved": "Excludes saved",
  "excludes.error": "Could not save excludes",
  "excludes.resolvedTo": "resolves to:",
  "excludes.noMatch": "Passed to restic as-is (not a recognized container path).",
  "excludes.excludesNothing": "This folder's volume is not in the backup, so this line excludes nothing.",
  // Exclude preview status (#38)
  "excludes.willExclude": "will be excluded from the backup",
  "excludes.matchesAnywhere": "will be excluded wherever it appears",
  // Exclusion assistant — server-side junk/large-folder scan with one-click exclude
  "excludes.assistTitle": "Exclusion assistant",
  "excludes.assistHint": "Scans this container's backup folders for well-known cache/temp/log folders and unusually large directories, so you can exclude them and shrink the backup.",
  "excludes.assistScan": "Scan for junk & large folders",
  "excludes.assistRescan": "Rescan",
  "excludes.assistScanning": "Scanning…",
  "excludes.assistScanFailed": "Scan failed",
  "excludes.assistTruncated": "The scan hit its time limit inside {path}. Folders after it were not examined.",
  // Whole backup folders that produced no rows at all, so no per-row flag can
  // speak for them. Without these, one unreadable folder among four reads as a
  // finished scan of all four.
  "excludes.assistUnexamined": "These backup folders were not examined at all: {paths}",
  "excludes.assistUnreadable": "These backup folders could not be read, so anything inside them is missing from this list: {paths}",
  "excludes.assistPathsUnavailable": "This container's backup folders cannot be reached right now. Check that the array or share holding them is mounted.",
  "excludes.assistNothingFound": "Nothing left to exclude: no junk or oversized folders found in this container's backup.",
  // Where the sizes come from, and what that promises (#175). The snapshot
  // date is required, not optional: an as-of-last-backup size shown without it
  // would just be a different misleading number.
  "excludes.assistSourceSnapshot": "Sizes come from the backup of {when}, so they are exact.",
  // Three reasons for a folder scan, three sentences. One string used to serve
  // all of them and told a user who had just been shown a failed index read
  // that the container had never been backed up.
  "excludes.assistSourceLive": "This container has no backup yet, so the sizes come from a live scan of the folders.",
  "excludes.assistSourceLiveRequested": "The sizes come from a scan of the folders as they are right now.",
  "excludes.assistSourceLiveNotInSnapshot": "The last backup does not cover the folders selected now, so the sizes come from a live scan of the folders.",
  "excludes.assistSnapshotStale": "This backup is more than a day old, so anything created since then is missing from this list.",
  "excludes.assistScanCurrent": "Check the folders as they are now",
  "excludes.assistIndexFailed": "Could not finish reading the backup index.",
  "excludes.assistScanLive": "Scan the folders instead",
  "excludes.assistSizeAtLeast": "at least {size}",
  "excludes.assistSizeMinimumTip": "The scan stopped inside this folder, so this size is a minimum.",
  "excludes.assistReasonCache": "Known cache",
  "excludes.assistReasonLarge": "Large folder",
  "excludes.assistExclude": "Exclude",
  "excludes.assistCurrent": "Current exclusions",
  "excludes.assistNoneYet": "None yet. Pick a suggestion above or type one.",
  "excludes.assistRemove": "Remove exclusion",
  "excludes.assistRemoveLine": "Remove exclusion {line}",

  // Appearance / Accent
  // ("settings.appearance", the old umbrella Card title, was removed here and
  // from every locale in the GlimStone follow-up pass — the shared Appearance
  // Card it titled was split into one Card per sub-topic (live-review point
  // 5), and nothing else ever read this key.)
  //   "settings.colors" is a LATER live-review round's own new key (jdp:
  // "Die card von Akzentfarbe und Regenbogenmodus in eine mergen. Gehört ja
  // zusammen") — the merged accent+rainbow Card's own title. Deliberately a
  // THIRD string, not a repurposed "settings.accentColor"/"settings.rainbow":
  // those two keep their existing meaning as the merged Card's own two
  // sub-topic row labels below, so the Card's own heading needed its own
  // "colour, broadly" word that doesn't collide with either.
  "settings.colors": "Colors",
  "settings.accentColor": "Accent color",
  "settings.accentPresets": "Presets",
  // New in the GlimStone follow-up pass, live-review round 6 (presets became
  // individually editable + resettable, and the count grew from 5 to 8):
  // "accentPreset" (singular) numbers each swatch's own accessible name
  // ("Preset 1", "Preset 2", …) the same way "settings.rainbowPalette" does
  // for the rainbow palette's 8 swatches.
  //   "settings.accentReset" REPLACES the former "settings.accentPresetsReset"
  // ("Reset presets") in every locale. That key existed because the accent row
  // used to carry TWO reset controls — an icon badge for the preset swatches
  // and a separate "Reset" text button for the active accent — and the two
  // needed distinguishable labels. The row now has exactly ONE control that
  // does both jobs (see AccentCard's own header comment in Settings.tsx for
  // why the split was the defect), so the label has to name both halves;
  // "Reset presets" on a control that also throws away the user's chosen
  // accent would be an outright lie. Renamed rather than re-valued so no
  // reader can keep the old, now-wrong meaning in mind.
  "settings.accentPreset": "Preset",
  "settings.accentReset": "Reset accent color and presets",
  // Shape (GlimStone form-engine — shape engine): round/soft/square corner
  // radius, applied everywhere via one attribute (index.css's [data-shape]
  // rules, lib/shape.ts's applyShape()). Wording matches KnightLoader's own
  // copy of this exact picker, the same "one product, every language" reason
  // the rainbow keys below give.
  "settings.shape": "Corners",
  "settings.shapeHint": "Applies to cards, buttons, tabs, inputs and badges at once.",
  "settings.shape.round": "Round",
  "settings.shape.soft": "Soft",
  "settings.shape.square": "Square",
  // Motion intensity (GlimStone motion-engine) — a deliberate reversal of
  // design-language.md's own prior "OS-controlled only, no fifth user
  // switch" decision; see that doc's Motion Intensity section and
  // lib/motion.ts's own header for the full course-correction note. Same
  // "one product, every language" reasoning as the shape keys right above.
  "settings.motion": "Animations",
  "settings.motionHint": "How much every animation in the app moves: a manual dial that sits alongside your system's reduced-motion setting, never overrides it.",
  "settings.motion.off": "Off",
  "settings.motion.subtle": "Subtle",
  "settings.motion.full": "Full",
  // Rainbow (GlimStone form-engine Phase 2, Task 1) — the accent, plural:
  // an eight-colour palette handed out by list position instead of one
  // accent everywhere. Originally matched the same keys in KnightLoader (the
  // other shipped app using this exact mechanism) verbatim; the GlimStone
  // follow-up pass renamed "settings.rainbow"/"settings.rainbowReactive"/
  // "settings.rainbowRotate" per live-review feedback (short verb-phrase
  // labels, their old descriptive sentences moved into InfoBubbles instead —
  // see Settings.tsx's Rainbow Card comment) — a deliberate, accepted
  // divergence from KnightLoader's wording, not an oversight.
  "settings.rainbow": "Rainbow Mode",
  "settings.rainbowHint": "Each row in a list gets its own colour from a set of eight, instead of everything sharing one accent colour, which makes long lists easier to tell apart at a glance.",
  "settings.rainbowReactive": "Reactive Mode",
  "settings.rainbowReactiveHint": "When on, a row or item's colour only appears while you're hovering it, or while it's running or selected. Otherwise it stays neutral. When off, every coloured row and item shows its colour all the time.",
  "settings.rainbowRotate": "Colour Rotation",
  "settings.rainbowRotateHint": "Shifts which colour in the palette counts as position 0, so the same list of rows doesn't always start on the exact same colour every time you turn Rainbow Mode on or reload the page.",
  "settings.rainbowPalette": "Palette colour",
  // Row label in front of the 8 swatches (live-review round 4) — a
  // DIFFERENT string from settings.rainbowPalette above on purpose: that one
  // stays a per-swatch aria-label ("Palette colour 1", "...2", …), this one
  // is the whole row's own opening caption ("Colour palette:"). See the
  // palette-row JSX's own comment for the "caption removed then reinstated
  // with a different string" history.
  "settings.rainbowPaletteLabel": "Colour palette",
  // The palette row's own reset badge, which used to borrow the generic
  // "common.reset". Given its own string in the same round that unified the
  // accent row's two resets above: that Card now holds TWO identical-looking
  // neutral square reset badges a few rows apart, and a bare "Reset" bubble on
  // both would leave the user guessing which one throws away which work.
  // Each names its own target instead. ("Colour", matching this row's own
  // settings.rainbowPaletteLabel right above, not the "color" spelling the
  // accent keys use — the en table's existing, deliberate split, untouched
  // here rather than half-normalised in passing.)
  "settings.rainbowPaletteReset": "Reset colour palette",
  // Quiet toasts (form-engine Task 9) — severity-based quiet mode for the
  // toast system; "success" toasts are suppressed, "warn"/"fail" never are
  // (lib/toastEngine.ts's shouldShowToast). Copy reworded (jdp, live review —
  // "same as KnightLoader's quiet-notifications toggle, adapted to
  // BombVault's own always-shown category") to name BOTH always-shown
  // severities instead of only "failures": an offsite-target test that
  // hasn't run yet, or a batch action that partly failed, push a "warn"
  // toast (see toastEngine.ts's ToastSeverity) and must stay visible in
  // quiet mode exactly like a save error does — the old copy's "only for
  // failures" undersold that.
  "settings.quietToasts": "Quiet toasts",
  "settings.quietToastsHint": "Hides success notices like save and copy confirmations. Errors, and anything else that needs your attention, still show.",

  // Dashboard stat cards
  "dashboard.statContainers": "Containers",
  "dashboard.statVMs": "VMs",
  "dashboard.statActiveJobs": "Active plans",
  "dashboard.statPausedJobs": "Paused plans",
  "dashboard.statErrors": "Errors",
  "dashboard.statMissingContainers": "Missing containers",
  "dashboard.statMissingVMs": "Missing VMs",

  // Error detail panel (#126) — the modal opened from the dashboard error count
  "errorPanel.title": "Backup errors",
  "errorPanel.resolve": "Resolve",
  "errorPanel.resolveAll": "Mark all resolved",
  "errorPanel.affected": "Affected",
  "errorPanel.empty": "No unresolved backup errors.",
  "errorPanel.count": "{count} occurrences",
  "errorPanel.filterPlaceholder": "Filter errors…",

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
  "dashboard.domainConfig": "Self-Backup",

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
  "ransomware.drillFailed": "restore drill failed",
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

  // Storage forecast line (growth/week + time-to-full, riding /api/stats)
  "dashboard.forecastGrowth": "Growing {bytes}/week",
  "dashboard.forecastShrink": "Shrinking {bytes}/week",
  "dashboard.forecastFull": "Repo volume full in ~{weeks} weeks",
  "dashboard.forecastFullOneWeek": "Repo volume full in ~1 week",
  "dashboard.forecastFullOverYear": "Repo volume full in > 1 year",
  "dashboard.forecastFree": "{bytes} free",

  // Domain filters + dashboard duration (#39/#40/#41)
  "dashboard.duration": "Duration",
  "containers.searchPlaceholder": "Search containers…",
  "vms.searchPlaceholder": "Search VMs…",
  "filter.all": "All",
  "filter.scheduled": "Scheduled",
  "filter.notScheduled": "Not scheduled",
  "filter.backedUp": "Backed up",
  "filter.neverBackedUp": "Never backed up",
  "filter.schedule": "Schedule",
  "filter.backup": "Backup",
  "filter.noMatch": "No items match the current filters.",

  // Jobs page
  "jobs.containersSection": "Containers",
  "jobs.vmsSection": "VMs",
  "jobs.flashSection": "Flash",
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
  "cadence.everyNUnavailable": "Every N days is not available for this schedule. It fires at fixed times and has nothing to count the interval from. Use one of the other modes instead.",
  "cadence.time": "Time",
  "cadence.days": "Days",
  "cadence.every": "Every",
  "cadence.daysUnit": "days",
  "cadence.fmtDaily": "daily at {time}",
  "cadence.fmtWeekly": "weekly ({days}) at {time}",
  "cadence.fmtEveryN": "every {n} days at {time}",
  // Cron cadence mode (#107)
  "cadence.cron": "Cron",
  "cadence.cronExpr": "Expression",
  "cadence.cronPlaceholder": "minute hour day month weekday",
  "cadence.cronInvalid": "Invalid cron expression. Expected 5 fields: minute hour day-of-month month day-of-week (e.g. 0 */6 * * *).",
  "cadence.cronValid": "Valid cron expression",
  "cadence.cronNext": "next: {first}, then {rest}",
  "cadence.cronExamples": "Examples",
  "cadence.cronExEvery6h": "every 6 hours",
  "cadence.cronExWeekdays": "weekdays at 02:30",
  "cadence.cronExMonthly": "on the 1st of each month",
  "cadence.fmtCron": "cron: {expr}",
  // TimePicker (GlimStone form-engine — new shared component, design-language.md
  // "The time picker"): aria-labels for the popover's two scrollable listbox
  // columns. The trigger's own accessible name reuses whatever `label` the
  // caller already passes (e.g. cadence.time), so no separate key is needed
  // for that half.
  "timePicker.hour": "Hour",
  "timePicker.minute": "Minute",
  "time.justNow": "just now",
  "time.minuteAgo": "1 minute ago",
  "time.minutesAgo": "{n} minutes ago",
  "time.hourAgo": "1 hour ago",
  "time.hoursAgo": "{n} hours ago",
  "time.dayAgo": "1 day ago",
  "time.daysAgo": "{n} days ago",
  "folder.browse": "Browse…",
  "folder.browseTitle": "Browse folders",
  "folder.use": "Use this folder",
  "folder.none": "No subdirectories",
  "folder.loading": "Loading…",
  "folder.pathHint": "Path must be a relative subpath (no leading / or ..)",
  "folder.couldNotRead": "Could not read directory",
  "folder.newFolder": "New folder",
  "folder.newFolderPlaceholder": "New folder name",
  "folder.creating": "Creating…",
  "folder.createFailed": "Could not create folder",
  "folder.browseFailed": "Browse failed",
  // ("common.reset" lived here until the accent row's leftover "Reset" TEXT
  // button was deleted and the rainbow palette's badge got its own named
  // string. Nothing read it afterwards, so it was dropped from all 42 tables
  // rather than left as a key that looks generic-and-shared but is used
  // nowhere — the next person to need a reset label would have reached for it
  // and reintroduced exactly the ambiguity this round removed.)
  "containers.subtitle": "Manage container backups, schedules, and restores.",
  "containers.emptyDocker": "No containers found. Is Docker running?",
  "containers.bulkResult": "{ok} ok, {fail} failed",
  "vm.method.saveFailed": "Couldn't change the backup method. It was not switched.",
  "jobs.noContainersIncluded": "No containers included in schedule.",
  "jobs.syncSchedules": "Use the Containers schedule for VMs, Flash and Folders too",
  "jobs.syncSchedulesHint": "When enabled, VMs, Flash and Folders all follow the Containers schedule instead of their own. Turn it off to set each domain's cadence independently.",
  "jobs.flashScheduleHint": "Backs up the Unraid USB flash boot drive (/boot) at the scheduled time.",
  "jobs.vmIncludeHint": "Backs up every VM with “include in schedule” enabled (set it per VM in the VMs tab).",
  "schedule.includeAll": "Include all in schedule",
  "schedule.excludeAll": "Exclude all from schedule",
  "schedule.updateFailed": "Failed to update schedule",
  // Per-item schedule overrides (#121)
  "jobs.noVMsIncluded": "No VMs included in schedule.",
  "settings.perItemSchedules": "Per-item schedules",
  "settings.perItemSchedulesHint": "Let individual containers and VMs override the domain schedule with their own cadence. Off by default; an item left blank keeps the domain schedule.",
  "schedule.overrideTitle": "Schedule override",
  "schedule.overrideUsesDefault": "Uses the domain schedule",
  "schedule.overrideEdit": "Set override",
  "schedule.overrideHint": "Empty uses the domain schedule.",
  "schedule.overrideSaved": "Override saved",

  // Auth / Login
  "auth.loginTitle": "BombVault",
  "auth.passwordLabel": "Password",
  "auth.signIn": "Sign in",
  "auth.signingIn": "Signing in…",
  "auth.invalidPassword": "Invalid password",
  "auth.loginError": "Login failed",

  // Settings — Security card
  "auth.security": "Security",
  "auth.authOff": "Authentication is off. All LAN users have full access.",
  "auth.authOn": "Authentication is enabled.",
  "auth.setPassword": "Set password",
  "auth.changePassword": "Change password",
  "auth.confirmPassword": "Confirm password",
  "auth.passwordMismatch": "Passwords do not match",
  "auth.passwordSaved": "Password saved",
  "auth.passwordCleared": "Authentication disabled",
  "auth.passwordHint":
    "Leave both fields empty to disable authentication. BombVault has root-equivalent host control. A password is recommended if this instance is reachable by untrusted LAN users.",
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
  // Reveal eye (form-engine Task 6): aria-label for the show/hide toggle on
  // every secret/token field.
  "common.showValue": "Show value",
  "common.hideValue": "Hide value",
  // ConfirmDialog (form-engine Task 7): the generic, reusable dialog chrome
  // labels — NOT per-call-site copy. Every confirm() call site keeps its own
  // existing message string; only these three boilerplate labels are new.
  "confirmDialog.title": "Confirm",
  "common.confirm": "Confirm",
  "common.cancel": "Cancel",
  // Toast (form-engine Task 9): generic dismiss label for every toast's
  // close button — not per-message copy.
  "toast.dismiss": "Dismiss notification",

  // Failure fallbacks. Every one of these replaced a HARDCODED English literal
  // sitting in a `res.error ?? "…"` or `err instanceof Error ? … : "…"` tail —
  // 56 of them across seven files, each rendering untranslated English into
  // whatever language the user had picked, and only ever at the moment
  // something had already gone wrong.
  //   They live under `common.` when the same sentence genuinely fits every
  // caller (a delete that failed is a delete that failed) and under their own
  // page's namespace when naming the thing that failed is the whole value of
  // the message — "Failed to load VMs" against "Failed to load containers"
  // tells the user which half of the app is broken, which a shared
  // "Failed to load" would throw away.
  //   The server's own `res.error` still wins wherever it is present; these are
  // the tails for a transport failure or a response carrying no message, which
  // is exactly the case a hardcoded string served worst.
  "common.actionFailed": "Action failed",
  "common.deleteFailed": "Delete failed",
  "common.removeFailed": "Remove failed",
  "common.saveFailed": "Save failed",
  "common.discoverFailed": "Discover failed",
  "common.checkFailed": "Check failed",
  "common.networkError": "Network error",
  "common.backupFailed": "Backup failed",
  "common.restoreFailed": "Restore failed",
  "common.compareFailed": "Compare failed",
  "common.loadBackupsFailed": "Failed to load backups",
  "common.deleteBackupsFailed": "Failed to delete backups",
  "containers.loadFailed": "Failed to load containers",
  "containers.backupStartFailed": "Failed to start backup",
  "containers.updateSettingFailed": "Failed to update setting",
  "vms.loadFailed": "Failed to load VMs",
  "files.loadSetsFailed": "Failed to load folder sets",
  "flash.loadBackupsFailed": "Failed to load flash backups",
  "config.loadBackupsFailed": "Failed to load settings backups",
  "config.loadSettingsFailed": "Could not load current settings",
  "dashboard.loadRunsFailed": "Failed to load runs",

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
  "flash.restoreNote": "Restore downloads a ZIP of the snapshot. The running /boot is never touched. Drop the .zip straight into the Unraid USB creator, or unzip it onto a fresh USB to rebuild your flash.",
  "flash.none": "No flash backups yet. Run a backup above.",
  // Merged card (GlimStone follow-up round, Paths & Storage tab rework, merge
  // B) — the flash zip export, plain-export encryption, and the restic
  // repositories' own encryption all sit under one roof: how backup exports
  // and repositories are protected.
  "settings.exportsEncryptionTitle": "Export & Repository Encryption",
  "settings.exportsEncryptionHint": "Controls for encrypting plain export artifacts and the restic repositories' own encryption.",
  // Scheduled flash zip export (#28): a plain .zip written to a folder after each flash backup.
  "flash.zipExport.title": "Flash zip export",
  "flash.zipExport.hint": "After each flash backup, also write the snapshot out as a plain .zip to a folder, ready for off-server sync (Syncthing, rclone, a cloud drive).",
  "flash.zipExport.enable": "Export a zip after each flash backup",
  "flash.zipExport.enableHint": "Every time a flash backup succeeds, the snapshot is written as a .zip to the folder below.",
  "flash.zipExport.path": "Export folder",
  "flash.zipExport.pathHint": "Relative subpath under the host mount root where the .zip lands. Point it at a Syncthing/rclone folder to get the flash off the server automatically.",
  "flash.zipExport.keepHistory": "Keep exported zip files",
  "flash.zipExport.keepHistoryHint": "Off: keep a single flash-latest.zip that's overwritten each time. On: keep the newest N timestamped flash-<date>.zip files. This is separate from the restic retention: off keeps a single, always-overwritten file (never fills the destination); on keeps the newest N and deletes older ones.",
  "flash.zipExport.keepN": "Zips to keep",
  "flash.zipExport.keepNHint": "The newest N timestamped zips are kept; older ones are deleted automatically.",
  "flash.zipExport.latestNote": "A single flash-latest.zip is overwritten after every backup.",
  "flash.zipExport.plaintextWarn": "The exported .zip is not encrypted, even if your flash repository is. Only sync it somewhere you trust.",
  "flash.zipExport.pathRequired": "Choose an export folder to turn this on.",
  // GlimStone follow-up round, Paths & Storage tab rework, merge B: "(age)"
  // dropped from the visible title — design-language.md's own "explanations
  // live in a bubble" rule, same as every other Card title in this file. What
  // age IS moved to export.encrypt.ageInfo below, an InfoBubble on the
  // merged card's sub-heading, so the information wasn't lost, just relocated.
  //   `export.encrypt.title` itself ("Encrypt plain exports"/"Plain-Exporte
  // verschlüsseln") is RETIRED (jdp, live-review, GlimStone follow-up round:
  // "Export und Verschlüsselung: Texte normal formatieren, es sind keine
  // Überschriften mehr") — the standalone <h3> it used to head is gone;
  // ToggleRow's own `export.encrypt.enable` label is now this sub-section's
  // only visible caption (Settings.tsx's own comment on that ToggleRow has
  // the full writeup), so a second, now-unused heading string would just be
  // dead weight.
  "export.encrypt.hint": "The restic repositories are already encrypted. This optionally seals the plain export artifacts (container and VM tar.gz plus their xml sidecars, and the flash zip) with age, so they are safe to store or move off the box.",
  "export.encrypt.ageInfo": "age (age-encryption.org) is a small, modern file-encryption tool, a simpler alternative to GPG for sealing a file to one or more recipients.",
  "export.encrypt.enable": "Encrypt exports with age",
  "export.encrypt.enableHint": "When on, container, VM, and flash exports are sealed with age before they are written to disk, and gain a .age suffix.",
  "export.encrypt.recipients": "age recipients",
  "export.encrypt.recipientsHint": "One recipient per line. Use an age public key (age1...) or an SSH public key. The matching private key is needed to decrypt off-box. With encryption on and no valid recipient, the export fails rather than writing plaintext.",
  "export.encrypt.recipientsPlaceholder": "age1qz...\nssh-ed25519 AAAA...",
  "export.encrypt.recipientsRequired": "Add at least one age recipient, otherwise the encrypted export will fail.",

  // Config self-backup (BombVault's own settings). Minimal en/de set for Task 12;
  // the full 24-locale translation lands in Task 14.
  "config.title": "Self-Backup",
  "config.subtitle": "Back up BombVault's own settings so a rebuilt server can restore itself.",
  "config.settingsTitle": "Self-Backup settings",
  "config.settingsHint": "Protect BombVault's own configuration (its settings database, off-site credentials and SSH keys) so a fresh install can restore itself and pick up right where it left off.",
  "config.enabled": "Back up BombVault's settings",
  "config.enabledHint": "Include BombVault's own /config in the schedule below.",
  "config.path": "Backup location",
  "config.pathHint": "Relative subpath under the host mount root where the config repo is written.",
  "config.schedule": "Schedule",
  "config.scheduleHint": "Backs up BombVault's own settings, targets and credentials at the scheduled time.",
  "config.offsite": "Off-site repo (optional)",
  "config.offsiteHint": "Replicate the config backup to a second, off-site repo after each local backup.",
  "config.offsiteSchedule": "Off-site schedule",
  "config.immutable": "Off-site repo is append-only (immutable)",
  "config.immutableHint": "Skip off-site pruning and refuse off-site deletes. The far side (append-only) enforces it.",
  "config.backupTitle": "Back up settings now",
  "config.backupHint": "Captures BombVault's own /config: the settings database, off-site credentials (rclone.conf) and SSH keypair.",
  "config.backupNow": "Back up settings now",
  "config.backingUp": "Backing up…",
  "config.snapshotsTitle": "Settings backups",
  "config.snapshotsHint": "To restore these settings onto a rebuilt server, use the Recovery tab: restoring settings restarts BombVault to apply them, so it lives there with the rest of the disaster-recovery flow.",
  "config.none": "No settings backups yet. Run a backup above.",

  // Receiver dashboard (read-only monitoring of an append-only off-site repo
  // another BombVault pushes to)
  "receiver.title": "Receiver",
  "receiver.subtitle": "Monitor the off-site copies other BombVault instances push to this box, read-only.",
  "receiver.addRepo": "Add received repo",
  // Card-title Badge headline for the empty-state list card (GlimStone
  // follow-up pass, "half-overlap card notch") — distinct from receiver.title
  // (the page's own h1) since the two sit right on top of each other.
  "receiver.emptyTitle": "Received repos",
  "receiver.empty": "No received repositories yet. Add the repo another BombVault pushes its off-site copies to, and BombVault watches it read-only: what arrived, when the last backup came in, and an independent integrity check on this hardware.",
  "receiver.loadError": "Could not load received repositories.",
  "receiver.reachable": "Reachable",
  "receiver.unreachable": "Unreachable",
  "receiver.monitoringOff": "Monitoring off",
  "receiver.lastReceived": "Last received",
  "receiver.never": "Never",
  "receiver.snapshotsCount": "{n} snapshots",
  "receiver.checkOk": "Check OK",
  "receiver.checkFailed": "Check failed",
  "receiver.checkNever": "Not checked yet",
  "receiver.lastChecked": "Last checked {time}",
  "receiver.checkNow": "Check now",
  "receiver.deepCheck": "Deep check (read data)",
  "receiver.details": "Details",
  "receiver.edit": "Edit",
  "receiver.remove": "Remove",
  "receiver.removing": "Removing…",
  // Downgraded from window.confirm() to the two-click inline-confirm pattern
  // (form-engine Task 7) — removing a monitoring entry is reversible (the
  // repo is never touched, only re-added later), so it no longer gets a full
  // dialog. Matches OffsiteTargetsSection's "offsite.targets.confirmRemove".
  "receiver.confirmRemove": "Confirm remove",
  "receiver.inventoryTitle": "Inventory by source",
  "receiver.inventoryLoading": "Loading inventory…",
  "receiver.inventoryError": "Could not load the inventory.",
  "receiver.inventoryEmpty": "No snapshots received yet.",
  "receiver.colSource": "Source",
  "receiver.colSnapshots": "Snapshots",
  "receiver.colLastReceived": "Last received",
  "receiver.colSize": "Size",
  "receiver.total": "Total",
  "receiver.addTitle": "Add received repo",
  "receiver.editTitle": "Edit received repo",
  "receiver.name": "Name",
  "receiver.repoLocation": "Repository location",
  "receiver.repoLocationHint": "The restic location this repo lives at: rest:http://host:8000/repo, s3:…, rclone:remote:path, or a subpath under the host mount.",
  "receiver.appKey": "Sending APP_KEY",
  "receiver.appKeyHint": "The APP_KEY (64 hex characters) of the BombVault that pushes to this repo, so it can be opened read-only. Stored encrypted and never shown again.",
  "receiver.appKeyKeep": "saved (leave blank to keep)",
  "receiver.appKeyInvalid": "The APP_KEY must be 64 lowercase hex characters.",
  "receiver.deadManHours": "Dead-mans-switch (hours)",
  "receiver.deadManHoursHint": "Alert when no backup has been received from a source within this many hours.",
  "receiver.checkCadence": "Check cadence",
  "receiver.checkCadenceHint": "When to run the independent integrity check on this box. Leave blank for daily, 'off' to disable, or e.g. 'weekly Sun 05:00'.",
  "receiver.checkCadencePlaceholder": "blank = daily 04:00 · off · daily HH:MM · weekly Sun 05:00",
  "receiver.readDataPercent": "Deep-check sample (%)",
  "receiver.readDataPercentHint": "How much pack data restic re-reads on the scheduled check. 0 = structural check only.",
  "receiver.enabledLabel": "Monitor this repository",
  "receiver.nameRequired": "Enter a name.",
  "receiver.repoRequired": "Enter a repository location.",
  "receiver.saveError": "Could not save the received repo.",
  "fleet.title": "Fleet",
  "fleet.subtitle": "Watch the protection status of peer BombVault instances, read-only.",
  "fleet.addPeer": "Add peer",
  // Card-title Badge headline for the empty-state list card (GlimStone
  // follow-up pass, "half-overlap card notch") — distinct from fleet.title
  // (the page's own h1) since the two sit right on top of each other.
  "fleet.emptyTitle": "Fleet peers",
  "fleet.empty": "No fleet peers yet. Add another BombVault instance's URL and fleet token, and this box polls it read-only for its protection scorecard, nothing more.",
  "fleet.loadError": "Could not load fleet peers.",
  "fleet.monitoringOff": "Monitoring off",
  "fleet.pollNever": "Never polled",
  "fleet.pollOk": "Poll OK",
  "fleet.pollFailed": "Poll failed",
  "fleet.lastPolled": "Last polled {time}",
  "fleet.pollNow": "Poll now",
  "fleet.polling": "Polling…",
  "fleet.details": "Details",
  "fleet.edit": "Edit",
  "fleet.remove": "Remove",
  "fleet.removing": "Removing…",
  // Downgraded from window.confirm() to the two-click inline-confirm pattern
  // (form-engine Task 7) — same rationale as receiver.confirmRemove above.
  "fleet.confirmRemove": "Confirm remove",
  "fleet.scorecardTitle": "Protection scorecard",
  "fleet.noScorecard": "No cached scorecard yet. Poll this peer to fetch one.",
  "fleet.lastBackup": "last backup {time}",
  "fleet.protection.green": "Protected",
  "fleet.protection.amber": "Degraded",
  "fleet.protection.red": "At risk",
  "fleet.protection.none": "Unknown",
  "fleet.addTitle": "Add fleet peer",
  "fleet.editTitle": "Edit fleet peer",
  "fleet.name": "Name",
  "fleet.url": "Peer URL",
  "fleet.urlHint": "The peer's base URL, e.g. https://192.168.1.50:3443. Find it on that instance's own Settings page.",
  "fleet.token": "Peer fleet token",
  "fleet.tokenKeep": "saved (leave blank to keep)",
  "fleet.tokenHint": "Generated on the PEER's own Settings → System page, then pasted here so this instance can poll it.",
  "fleet.enabledLabel": "Poll this peer",
  "fleet.nameRequired": "Enter a name.",
  "fleet.urlRequired": "Enter a peer URL.",
  "fleet.saveError": "Could not save the fleet peer.",
  "fleet.mesh.offersTitle": "Off-site storage offers",
  "fleet.mesh.offersHint": "A peer proposed its own off-site storage. Review and accept to turn it into a normal off-site target. Nothing is applied automatically.",
  "fleet.mesh.saveError": "Could not save this.",
  "fleet.mesh.unknownPeer": "Unknown peer",
  "fleet.mesh.applyTo": "Apply to:",
  "fleet.mesh.accept": "Accept",
  "fleet.mesh.decline": "Decline",
  "fleet.mesh.status.pending": "Pending",
  "fleet.mesh.status.accepted": "Accepted",
  "fleet.mesh.status.declined": "Declined",
  "fleet.mesh.proposeButton": "Offer storage",
  "fleet.mesh.proposeTitle": "Offer off-site storage",
  "fleet.mesh.proposeHint": "Send this instance's own off-site storage connection details to {peer}, so its admin doesn't have to be told the URL and password out of band.",
  "fleet.mesh.domain": "Which domain is this storage for?",
  "fleet.mesh.baseUrl": "Where will you deploy the rest-server?",
  "fleet.mesh.baseUrlHint": "The real address the rest-server will be reachable at, e.g. http://192.168.1.50:8000. BombVault cannot guess this.",
  "fleet.mesh.baseUrlRequired": "Enter the base URL where you will deploy the rest-server.",
  "fleet.mesh.sending": "Sending…",
  "fleet.mesh.send": "Send offer",
  "fleet.mesh.sent": "Offer sent to {peer}.",
  "fleet.mesh.deployNow": "Now deploy the rest-server with this recipe:",
  "fleet.mesh.dockerRun": "docker run",
  "fleet.mesh.compose": "docker-compose.yml",

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
  "vm.ssh.publicKey": "Public key: append to Unraid /root/.ssh/authorized_keys",
  "vm.ssh.copy": "Copy",
  "vm.ssh.copied": "Copied",
  "vm.ssh.copyFailed": "Copy failed",
  "vm.ssh.test": "Test connection",
  "vm.ssh.testing": "Testing…",
  "vm.ssh.testOk": "Connected, libvirt reachable",
  "vm.ssh.testFail": "Connection failed",
  "vm.ssh.setupTitle": "Set up (one time)",
  "vm.ssh.step1": "Copy the command below and run it in the Unraid terminal to authorize this key (it survives reboots).",
  "vm.ssh.step2": "Set the container's “VM Backup: Host” variable to your Unraid LAN IP (e.g. 192.168.x.x); on simple bridge networking host.docker.internal also works.",
  "vm.ssh.step3": "Click Test connection. Once it's green, enable VMs under Domains.",
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
  "recovery.appKeyExplain": "To read existing backups this container needs the SAME APP_KEY it used before. It's in your recovery kit. Set it in the Unraid container template if it isn't already, then re-check.",
  "recovery.appKeyRemedy": "The encryption key doesn't match these backups. Set the original APP_KEY (from your recovery kit) in the container template, then re-check.",
  "recovery.readable": "Your backups are readable.",
  "recovery.notReachable": "Couldn't reach your backups yet. Attach the location below, then re-check.",
  "recovery.recheck": "Check",
  // Step 2 — restore BombVault's own settings first (optional, before attach)
  "recovery.stepConfig": "Restore BombVault's own settings",
  "recovery.configHint": "On a rebuilt server, restore BombVault's own settings first (its backup paths, off-site targets and credentials) so the steps below come pre-filled. Point it at the settings backup you set up earlier. No settings backup? Skip this and attach your backups manually below.",
  "recovery.configAppKeyReminder": "Your APP_KEY must match this backup. That's the check in Step 1 above.",
  "recovery.configSourceLabel": "Where is the settings backup?",
  "recovery.configLocalPath": "Local path",
  "recovery.configOffsiteUrl": "Off-site repo URL",
  "recovery.configRestore": "Restore",
  "recovery.configRestoring": "Restoring…",
  "recovery.configRestarting": "BombVault is restarting to apply your settings… this page reloads automatically when it's back.",
  "recovery.configManualRestart": "Your settings are staged. Restart the BombVault container in Unraid, then continue. They apply on the next boot.",
  "recovery.configReloadWhenBack": "BombVault is taking longer than expected to come back. Reload this page once it's up to load your restored settings.",
  "recovery.configReload": "Reload now",
  "recovery.configSkip": "Skip: I don't have a settings backup",
  "recovery.configSkipped": "Skipped. Attach your backups manually below.",
  // Step 3 — attach your backups
  "recovery.step2": "Attach your backups",
  "recovery.cloudCreds": "Cloud credentials (optional)",
  "recovery.cloudCredsHint": "Only needed when a backup path points at S3, a restic REST server or an rclone remote. A local path or a mounted share needs nothing here.",
  "recovery.attachHint": "Point BombVault at your existing backups: a local path under the host mount, or an off-site repo (rest / S3 / B2 / sftp / rclone) with its credentials. Then connect to confirm.",
  "recovery.credsSaveHint": "Off-site credentials save with each card's own Save button. Save them before you connect & preview.",
  "recovery.connectPreview": "Connect & preview",
  // Encryption mode — DETECTED, not asserted. The repositories themselves say
  // whether they need the APP_KEY-derived password, so the common path (attach
  // an existing repo) asks the user nothing; only the genuinely undecidable
  // cases still offer the switch. These are live status readouts, not
  // explanations, so they stay visible page text (the mechanism itself is what
  // lives in the (i) bubble).
  "recovery.encChecking": "Checking how your backups are encrypted…",
  "recovery.encEncrypted": "Detected: your backups are encrypted. BombVault will use the password derived from your APP_KEY.",
  "recovery.encPlain": "Detected: your backups have no password.",
  "recovery.encAbsent": "No backup repository exists at these locations yet, so there is nothing to detect. Your choice below decides how it gets created.",
  "recovery.encUnknown": "Can't tell yet: the repositories couldn't be opened, so their encryption mode is unknown. Fix the location above and check again, or set it yourself if you already know.",
  "recovery.encConflict": "Your repositories disagree: some are encrypted, some are not. One setting can't open both. Point the odd one out at a new, empty location, or restore from the matching set only.",
  "recovery.encUnconfigured": "No backup location configured yet. Set the paths below, then connect.",
  "recovery.encDetectHint":
    "Encryption isn't a preference: a repository is created either with a password (derived from your APP_KEY) or without one, and that never changes afterwards. BombVault opens the configured repositories to see which it is, so restoring on a fresh box needs no guesswork. A repository that can't be reached is reported as unknown, never as unencrypted.",
  "recovery.encStateEncrypted": "encrypted",
  "recovery.encStatePlain": "no password",
  "recovery.encStateAbsent": "not created yet",
  "recovery.encStateUnreachable": "couldn't be read",
  "recovery.encSourceLocal": "local",
  "recovery.encSourceOffsite": "off-site",
  // Step 3 — discover everything
  "recovery.step3": "Discover what's in your backups",
  "recovery.discover": "Discover backups",
  "recovery.foundCounts": "Found {c} containers and {v} VMs.",
  "recovery.foundNone": "Nothing found yet. Check the connection and attachment above. If you expected backups here, make sure your APP_KEY matches these backups.",
  // Step 4 — review & restore all (left stopped)
  "recovery.step4": "Review and restore",
  "recovery.restoreAll": "Restore all (left stopped)",
  "recovery.restoreAllResult": "Restored {ok}, failed {fail}. Start them from the Containers/VMs tabs when ready.",
  "recovery.vmSshNote": "VM restore needs the libvirt SSH link. Set it up under Settings → VM Backup over SSH.",
  "recovery.noneDiscovered": "Run Discover above first.",
  // Step 5 — recovery kit (safety net for next time)
  "recovery.step5": "Your recovery kit",
  "recovery.kitHint": "Download and store your recovery kit somewhere safe. It holds the encryption key and the exact restic commands to restore even without BombVault.",
  "recovery.kitDownload": "Download recovery kit",
  // Dashboard fresh-install nudge → guided Recovery tab
  "recovery.freshNudge": "Restoring from a previous server or a rebuild? Recover your existing backups.",
  "recovery.freshNudgeCta": "Go to Recovery",

  // Settings — section tabs + schedule group headings + subtitle (v5 redesign)
  "settings.tab.general": "General",
  "settings.tab.storage": "Paths & Storage",
  "settings.tab.schedules": "Schedules",
  "settings.tab.offsite": "Off-site",
  "settings.tab.notifications": "Notifications",
  "settings.tab.integrity": "Integrity",
  "settings.tab.system": "System",
  "settings.schedulesOptions": "Schedule options",
  "settings.schedulesOffsite": "Off-site replication schedules",
  "settings.schedulesSelfBackup": "Self-backup schedule",
  "settings.schedulesChecks": "Restore-check schedule",
  "settings.tamperTestSchedule": "Tamper-test schedule",
  "settings.tamperScheduleInactive": "Inactive: no off-site repo is marked append-only, so this schedule never runs. Mark an off-site repo as append-only in its off-site settings to enable it.",
  // Backup Everything (a 6th, independent pass over all five domains above)
  "settings.everythingTitle": "Backup Everything",
  "settings.everythingHint": "Runs every domain once, in order (containers, VMs, flash, folders, then the self-backup) so a single ping afterward (via the post-command below) confirms the whole server is protected. Off by default, and independent of each domain's own schedule above.",
  "settings.everythingHooksHint": "These commands run on BombVault's own server via sh -c, not inside a container. The pre-command is best-effort and never blocks the pass; the post-command always fires exactly once, after every domain has been attempted, whether or not any of them failed.",
  "settings.everythingOverlapWarning": "This runs independently of the per-domain schedules above. If both are on, containers, VMs, flash, folders and the self-backup each run twice: once here, once on their own schedule. Turn off the domains you don't want duplicated.",
  "settings.everythingRunNow": "Run Backup Everything now",
  "settings.everythingStarted": "Started. It runs on the server across every domain in turn; watch the Activity Log for the outcome.",
  "settings.everythingAlreadyRunning": "A Backup Everything pass is already running.",
  "settings.everythingBusy": "Working…",
  "settings.subtitle": "BombVault configuration. Changes take effect immediately.",
  // Filter drawer trigger (v5 redesign)
  "filter.button": "Filters",

  // Settings — weekly digest card, backup-engine cache card, revoke-all sessions
  "settings.digestTitle": "Weekly digest",
  "settings.digestHint": "One summary message per week: run counts, new backup data, off-site currency and the top failures, sent through the notification channels above.",
  "settings.digestToggle": "Weekly digest",
  // Settings — missed-schedule catch-up (schedules tab) + overdue-backup watchdog (notifications tab)
  "settings.missedSchedulesTitle": "Missed schedules",
  "settings.catchUpMissed": "Catch up missed backups after start",
  "settings.catchUpMissedHint": "If the server was off when a schedule was due, run that backup about two minutes after BombVault starts.",
  "settings.watchdogTitle": "Overdue backup watchdog",
  "settings.watchdogHint": "Checks once a day whether any enabled backup is overdue (older than twice its schedule) and sends one notification per incident through the channels configured above.",
  "settings.watchdogToggle": "Notify when backups are overdue",
  "settings.cacheTitle": "Backup engine cache",
  "settings.cacheHint": "The backup engine keeps a cache of repository data under /config so incremental and off-site runs stay fast. When it grows past this limit, the least-recently-used per-repository caches are removed after scheduled runs.",
  "settings.cacheLimitLabel": "Cache size limit (MB, 0 = unlimited)",
  "settings.logoutAll": "Sign out everywhere",

  // What's new dialog (#48) — shown once when a new version is running
  "whatsnew.title": "What's new in {version}",
  "whatsnew.loading": "Loading release notes…",
  "whatsnew.loadFailed": "Couldn't load the release notes here. Open them on GitHub.",
  "whatsnew.retry": "Try again",
  "whatsnew.viewOnGitHub": "View full release on GitHub",
  "whatsnew.close": "Close",

  // Files domain (folder-set backups, #62) — shown to the user as "Folders".
  // The files.* KEY names, the "files" domain literal, routes and types are
  // technical identifiers and deliberately keep the historical name.
  "nav.files": "Folders",
  "files.title": "Folders",
  "files.subtitle": "Back up any folders on this server, with schedules, off-site copies and restores.",
  // Card-title Badge headline for the empty-state list card (GlimStone
  // follow-up pass, "half-overlap card notch") — distinct from files.title
  // (the page's own h1) since the two sit right on top of each other.
  "files.setsTitle": "Folder sets",
  "files.empty": "No folder sets yet. Add a folder (shares, documents, photos, anything under your mounts) and BombVault protects it like everything else: schedules, off-site copies, integrity checks and restores. No separate file-backup tool needed.",
  "files.addSet": "Add folder set",
  "files.editSet": "Edit folder set",
  "files.name": "Name",
  "files.path": "Folder",
  "files.pathHint": "The folder to back up, a relative subpath under the host mount root.",
  "files.excludes": "Exclude patterns",
  "files.excludesHint": "One pattern per line, passed to restic as --exclude (e.g. *.tmp, cache/).",
  "files.excludesCount": "Excludes: {n}",
  "files.enabled": "Include in schedule",
  "files.pathMissing": "Folder not found",
  "files.noPath": "No folder set",
  "files.noPathHint": "Rebuilt from backups without a folder. Set a folder to back it up again. Restoring to a folder already works.",
  "files.deleteSet": "Remove set",
  "files.deleteSetConfirm": "Remove this folder set from the list? Its backups are not deleted and can be rediscovered later.",
  "files.deleteBackupsConfirm": "Delete ALL backups of this folder set? The snapshots are permanently removed, the repository is pruned and the set is forgotten. This cannot be undone.",
  "files.restoreOriginal": "Restore to original location",
  "files.restoreOriginalConfirm": "Restore this backup over the set's folder? Existing files will be overwritten.",
  "files.restoreToFolder": "Restore to a folder",
  "files.restoreSelectFiles": "Select files",
  "files.restoreComplete": "Restore complete: the files have been written.",
  "files.backupAll": "Back up all now",
  "files.discoverHint": "Lost the set list? Rebuild it from the backups in storage.",
  "files.cancel": "Cancel",
  // "Host system config" preset (generic/TrueNAS only — Unraid already has the
  // flash domain for this). Platform-expansion plan Task 7.
  "files.addPreset": "Add preset: Host system config",
  "files.addPresetHint": "A conservative starting point for host-level configuration outside your containers, not a claim of completeness. Review the folder before saving.",
  // Files domain integration — Settings, Dashboard, Recovery (#62 task 7)
  "settings.filesEnabled": "Folders",
  "settings.filesEnabledHint": "Back up arbitrary folders under your mounts as file sets, independent of the other domains.",
  "settings.filesPath": "Folders path",
  "jobs.filesSection": "Folders",
  "jobs.filesIncludeHint": "Backs up every folder set with “include in schedule” enabled. Toggle each set below or on the Folders tab.",
  "jobs.noFileSetsIncluded": "No folder sets yet. Add them on the Folders tab.",
  "dashboard.domainFiles": "Folders",
  "recovery.filesFound": "Found {f} folder sets.",
  "recovery.filesRestoreHint": "Rediscovered folder sets carry no original folder. Each one restores into a folder you choose.",
  // Restore from another BombVault repo — Recovery page (#61 task 11)
  "recovery.foreignTitle": "Restore from another BombVault repo",
  "recovery.foreignIntro": "Pull single containers, VMs or folder sets out of a DIFFERENT BombVault instance's backups: connect read-only, browse what's inside, restore what you pick. This reads the other repository, changes nothing over there, and leaves your own backup settings untouched.",
  "recovery.foreignStepConnect": "Connect to the other repository",
  "recovery.foreignStepBrowse": "Browse & restore",
  "recovery.foreignLocation": "Repository location",
  "recovery.foreignLocationHint": "A folder under the host mount. Mount the other server's backup share on this host and point this at it. The repository must be locally mounted (remote repo URLs aren't accepted here).",
  "recovery.foreignKey": "APP_KEY of the other instance",
  "recovery.foreignKeyHint": "The 64-character key from the OTHER instance's recovery kit. Your own key stays untouched.",
  "recovery.foreignConnect": "Connect",
  "recovery.foreignConnecting": "Connecting…",
  "recovery.foreignConnected": "Connected. The repository is readable.",
  "recovery.foreignClose": "Disconnect",
  "recovery.foreignNotConnected": "Connect to a repository above first.",
  "recovery.foreignEmpty": "The repository is readable but holds no BombVault backups.",
  "recovery.foreignLatest": "Latest backup",
  "recovery.foreignTargetFolder": "Target folder",
  "recovery.foreignWholeSet": "Whole set",
  "recovery.foreignPickSubfolder": "Pick a subfolder",
  "recovery.foreignSubfolderHint": "Restore only the ticked subfolders or files of this set into the target folder, for example one stack out of a whole-appdata backup.",
  "recovery.foreignAppdataDest": "Appdata destination",
  "recovery.foreignAppdataDestHint": "Where the container's appdata is restored. Leave blank for the default. A container backed up from a pool this server does not have (for example /mnt/zfs) is remapped here so it lands correctly.",
  "recovery.foreignOverwrite": "Overwrite if the destination already contains data",
  "recovery.foreignBindWarning": "These binds point at storage this server does not have. Appdata is remapped automatically, but fix these in the container's template after the restore:",
  "recovery.foreignRestore": "Restore here",
  "recovery.foreignExistsConfirm": "“{name}” already exists on this system, so restoring will OVERWRITE it with the foreign backup. Continue?",
  "recovery.foreignUnverifiedConfirm": "BombVault could not read this system's current containers and VMs, so it cannot tell whether “{name}” already exists here. Restoring may overwrite an existing one. Continue?",
  "recovery.foreignExpired": "The session has expired (sessions last 30 minutes). Reconnect to keep browsing.",
  "recovery.foreignReconnect": "Reconnect",
  "recovery.foreignVMDest": "VM disk destination",
  "recovery.foreignVMDestHint": "Where the VM's disks are written. The disk images go to <destination>/<vm-name>/, so pick a folder on a real mounted pool, not the RAM disk. A foreign VM is restored left stopped, so start it yourself once you have checked it.",

  // Dashboard activity log (Task 4) — a flat, docker-logs-style merged view of
  // run history + live SSE progress + the next scheduled run.
  "activityLog.title": "Activity Log",
  "activityLog.filterPlaceholder": "Filter… (e.g. plex, failed, off-site)",
  "activityLog.filterAllDomains": "All domains",
  "activityLog.filterAllTypes": "All types",
  "activityLog.typeBackup": "Backup",
  "activityLog.typeRestore": "Restore",
  "activityLog.typePrune": "Prune",
  "activityLog.typeVerify": "Verify",
  "activityLog.typeOffsite": "Off-site",
  "activityLog.typeExport": "Export",
  "activityLog.jumpToLatest": "Jump to latest",
  "activityLog.glyphRunning": "Running",
  "activityLog.glyphSuccess": "Success",
  "activityLog.glyphFailed": "Failed",
  "activityLog.glyphOffsite": "Off-site",
  "activityLog.glyphInfo": "Info",
  "activityLog.domainContainers": "Containers",
  "activityLog.domainVMs": "VMs",
  "activityLog.domainFlash": "Flash",
  "activityLog.domainConfig": "Self-Backup",
  "activityLog.domainFiles": "Folders",
  "activityLog.domainEverything": "Backup Everything",
  "activityLog.jobBackup": "backup",
  "activityLog.jobOffsite": "off-site replication",
  "activityLog.jobDrill": "restore-verification drill",
  "activityLog.jobTamper": "tamper test",
  "activityLog.jobDigest": "weekly digest",
  "activityLog.jobWatchdog": "overdue-backup check",
  "activityLog.lineBackingUpItem": "Backing up {name} … {percent}%",
  "activityLog.lineRestoringItem": "Restoring {name} … {percent}%",
  "activityLog.lineBackingUpBatch": "Backing up all {domain} … {percent}%",
  "activityLog.lineOffsiteRunning": "Off-site upload: {domain} …",
  // Issue #159: the {duration}-carrying sibling of lineOffsiteRunning, used
  // once the live progress event's startedAt is known (see activityLog.ts's
  // buildLiveLines) but no live per-snapshot percentage is available yet.
  "activityLog.lineOffsiteRunningWithDuration": "Off-site upload: {domain} … ({duration})",
  // Issue #159: the RUN-LEVEL percentage (lib/progress.ts's offsiteRunProgress),
  // with the snapshot count as the parenthetical detail behind it — {total} is
  // a best-effort candidate count, since restic never reports a whole-run total
  // across snapshots. Same word-order rule as offsite.replicatingSnapshotPercent
  // above, for the same reason; that key's comment has the full story.
  "activityLog.lineOffsiteRunningSnapshotPercent": "Off-site upload: {domain} … {percent}% overall (snapshot {index} of {total})",
  "activityLog.lineOffsiteRunningSnapshotPercentWithDuration": "Off-site upload: {domain} … {percent}% overall (snapshot {index} of {total}) · {duration}",
  "activityLog.linePruneRunning": "Pruning: {domain} …",
  "activityLog.lineVerifyRunning": "Verifying: {domain} …",
  "activityLog.lineDrillRunning": "Restore check running: {domain} …",
  "activityLog.lineDRDrillRunning": "Off-site DR check running: {domain} …",
  "activityLog.lineTamperRunning": "Tamper test running: {domain} …",
  "activityLog.lineExportRunning": "Exporting flash ZIP …",
  "activityLog.lineBackupSuccess": "{name} backed up: {bytes} in {duration}",
  "activityLog.lineBackupFailed": "{name} backup failed: {error}",
  "activityLog.lineBackupSkipped": "{name} backup skipped: {error}",
  "activityLog.lineRestoreSuccess": "{name} restored: {duration}",
  "activityLog.lineRestoreFailed": "{name} restore failed: {error}",
  "activityLog.lineUpdateSuccess": "{name} updated: {duration}",
  "activityLog.lineUpdateFailed": "{name} update failed: {error}",
  "activityLog.linePruneSuccess": "Retention prune done: {domain}",
  "activityLog.linePruneFailed": "Prune failed ({domain}): {error}",
  "activityLog.lineVerifySuccess": "Verify passed: {domain}",
  "activityLog.lineVerifyFailed": "Verify failed ({domain}): {error}",
  "activityLog.lineOffsiteSuccess": "Off-site replication done: {domain} ({duration})",
  "activityLog.lineOffsiteFailed": "Off-site replication failed ({domain}): {error}",
  "activityLog.lineDrillSuccess": "Restore check passed: {domain}",
  "activityLog.lineDrillFailed": "Restore check failed ({domain}): {error}",
  "activityLog.lineDRDrillSuccess": "Off-site DR restore verified: {domain}",
  "activityLog.lineDRDrillFailed": "Off-site DR restore FAILED ({domain}): {error}",
  "activityLog.lineTamperSuccess": "Tamper test passed: {domain} (delete refused)",
  "activityLog.lineTamperFailed": "Tamper test FAILED. {domain} is not append-only: {error}",
  "activityLog.lineTamperSkipped": "Tamper test skipped ({domain}): {error}",
  "activityLog.lineExportSuccess": "Flash ZIP export done: {bytes} ({duration})",
  "activityLog.lineExportFailed": "Flash ZIP export failed: {error}",
  "activityLog.lineOther": "{name} {kind}: {status}",
  "activityLog.lineNextWithDomain": "next: {job} ({domain}) at {time} (in {countdown})",
  "activityLog.lineNextNoDomain": "next: {job} at {time} (in {countdown})",
  "activityLog.lineEmpty": "nothing yet",
  // Heatmap → Activity Log day drilldown (clicking a heatmap cell filters the
  // log to that day; the chip shows the active day and its × clears it).
  "activityLog.dayFilterChip": "Showing {date}",
  "activityLog.clearDayFilter": "Clear day filter",

  // Export / import settings (portable config file)
  "settingsIO.title": "Export / import settings",
  "settingsIO.desc":
    "Save this instance's configuration to a file, or load a file exported earlier into this instance. Only settings and off-site destinations are moved. Your backups, snapshots and history are not affected.",
  "settingsIO.exportHeading": "Export",
  "settingsIO.includeCreds": "Include credentials (off-site and notification secrets)",
  "settingsIO.credsWarning":
    "With credentials, this file is as sensitive as your recovery kit: it holds your off-site and notification secrets in readable form. Store it somewhere safe.",
  "settingsIO.exportButton": "Export settings",
  "settingsIO.exporting": "Exporting…",
  "settingsIO.importHeading": "Import",
  "settingsIO.importHint":
    "Load a settings file exported from BombVault. You'll see a summary and a confirmation before anything changes.",
  "settingsIO.chooseFile": "Choose a settings file",
  "settingsIO.reading": "Reading file…",
  "settingsIO.previewTitle": "This file contains",
  "settingsIO.previewExportedAt": "Exported",
  "settingsIO.previewAppVersion": "From BombVault version",
  "settingsIO.previewOffsiteTargets": "Off-site targets",
  "settingsIO.previewCredentials": "Credentials",
  "settingsIO.previewCredsIncluded": "included",
  "settingsIO.previewCredsNotIncluded": "not included",
  "settingsIO.previewSettingsAreas": "Settings areas",
  "settingsIO.previewNone": "none",
  "settingsIO.confirmWarning":
    "This replaces your current BombVault settings. Your backup data and history are not affected.",
  "settingsIO.confirmButton": "Replace settings",
  "settingsIO.importing": "Importing…",
  "settingsIO.cancel": "Cancel",
  "settingsIO.importSuccess": "Settings imported.",
  "settingsIO.importFailed": "Import failed",
  "settingsIO.group.domains": "Backup sources",
  "settingsIO.group.schedules": "Schedules",
  "settingsIO.group.retention": "Retention",
  "settingsIO.group.offsite": "Off-site",
  "settingsIO.group.drills": "Restore drills",
  "settingsIO.group.digest": "Digest",
  "settingsIO.group.everything": "Backup Everything",
  "settingsIO.group.monitoring": "Monitoring",
  "settingsIO.group.language": "Language",
  "settingsIO.group.exportEncryption": "Export encryption",

  // Backup order (#119) — manual per-container backup sequence, Containers page.
  "backupOrder.title": "Backup order",
  "backupOrder.hint": "Set the sequence scheduled and batch backups run in. Containers left off the list run afterwards, most overdue first.",
  "backupOrder.moveUp": "Move up",
  "backupOrder.moveDown": "Move down",
  "backupOrder.save": "Save order",
  "backupOrder.saved": "Order saved",
  "backupOrder.saveError": "Could not save the backup order.",
  "backupOrder.empty": "No scheduled containers to order yet.",
  "vmBackupOrder.title": "VM backup order",
  "vmBackupOrder.hint": "Set the sequence the scheduled VM run backs VMs up in. VMs left off the list run afterwards, in name order.",
  "vmBackupOrder.empty": "No scheduled VMs to order yet.",
  "backupOrder.reset": "Clear order",

  // Health-gated restart (#119) — Settings, Schedules tab.
  "settings.restartHealthTitle": "Restart after backup",
  "settings.restartHealthWait": "Wait for dependencies to be healthy before starting the next",
  "settings.restartHealthWaitHint": "When 'Stop other containers during backup' stops containers, they restart in dependency order after the backup (and after any post-backup update). With this on, each container must report healthy or running before the ones that depend on it start.",
  "settings.restartHealthTimeoutLabel": "Per-container health timeout (seconds)",
  "settings.restartHealthTimeoutHint": "How long to wait for one container to become healthy before its dependents start anyway. Range 5 to 3600.",

  // Reconcile Unraid update status (#116) — Settings, Storage tab. After the
  // post-backup container update recreates a container, ask Unraid to refresh
  // its own cached update status so the Docker tab's stale banner clears.
  "settings.reconcileUnraidStatus": "Refresh Unraid's update status after updating a container",
  "settings.reconcileUnraidStatusHint": "Clear Unraid's update banner after BombVault updates a container in the post-backup update step.",
} as const;

export type TranslationKey = keyof typeof en;
export type Translations = Record<TranslationKey, string>;

// ---------------------------------------------------------------------------
// German locale (full)
// (exported so the locale-parity test can import it alongside `en`)
// ---------------------------------------------------------------------------

export const de: Translations = {
  "language.label": "Sprache",
  "theme.dark": "Dunkel",
  "theme.light": "Hell",

  "nav.dashboard": "Dashboard",
  "nav.containers": "Container",
  "nav.vms": "VMs",
  "nav.flash": "Flash",
  "nav.config": "Selbst-Backup",
  "nav.receiver": "Empfänger",
  "nav.fleet": "Flotte",
  "nav.settings": "Einstellungen",
  "nav.reportBug": "Fehler melden",

  "mode.simpleView": "Einfache Ansicht",
  "mode.advancedView": "Erweiterte Ansicht",

  "dashboard.title": "Dashboard",
  "dashboard.subtitle": "Dein Backup-Status auf einen Blick.",
  "dashboard.summaryHealth": "Gesamtzustand",
  "dashboard.summaryNextBackup": "Nächstes Backup",
  "dashboard.summaryLastResult": "Letztes Ergebnis",
  "dashboard.lastBackups": "Letzte Backups",
  "dashboard.recentRuns": "Letzte Ausführungen",
  "dashboard.spikeStatus": "Systemstatus",
  "dashboard.noRuns": "Noch keine Ausführungen",
  "dashboard.spikeLink": "Host-Integration prüfen",
  "dashboard.hostIntegrationCheck": "Host-Integration-Check",
  "dashboard.allOk": "Alle Systeme OK",
  "dashboard.degraded": "Eingeschränkt",
  "dashboard.checking": "Prüfe…",
  "dashboard.noContainers": "Keine Container gefunden.",

  "dashboard.customize": "Anpassen",
  "dashboard.customizeDone": "Fertig",
  "dashboard.customizeHint": "Karte ziehen zum Umsortieren, oder ausblenden, was du nicht brauchst. In diesem Browser gespeichert.",
  "dashboard.moveUp": "Nach oben",
  "dashboard.moveDown": "Nach unten",
  "dashboard.hideCard": "Ausblenden",
  "dashboard.showCard": "Einblenden",
  "dashboard.makeHalfWidth": "Halbe Breite",
  "dashboard.makeFullWidth": "Volle Breite",
  "dashboard.hiddenCards": "Ausgeblendete Karten",
  "dashboard.resetLayout": "Auf Standard zurücksetzen",
  "dashboard.blockSummary": "Überblick",
  "dashboard.blockStats": "Statistik",
  "dashboard.blockBackups": "Backups & Verlauf",

  "spike.title": "Host-Integration",
  "spike.overall": "Gesamt:",
  "spike.allOk": "ALLES OK",
  "spike.degraded": "EINGESCHRÄNKT",
  "spike.colCheck": "Prüfung",
  "spike.colStatus": "Status",
  "spike.colDetail": "Detail",
  "spike.ok": "OK",
  "spike.fail": "FEHLER",
  "spike.info": "Info",
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
  "containers.scheduleHint": "Aufgenommene Container werden ab der geplanten Zeit nacheinander gesichert. Sie teilen sich ein Repository, laufen also der Reihe nach, nicht alle gleichzeitig.",
  "containers.notInstalled": "Nicht installiert",
  "containers.notInstalledTitle": "Nicht installiert (nur Backups)",
  "containers.notInstalledHint": "Diese Container sind nicht mehr installiert, haben aber noch Backups. Stelle sie wieder her oder lösche ihre Backups, um Platz freizugeben.",
  "containers.deleteBackups": "Alle Backups löschen",
  "containers.deleteBackupsConfirm": "ALLE Backups dieses Containers löschen? Die Snapshots werden dauerhaft aus dem Repository entfernt und können nicht wiederhergestellt werden.",
  "containers.filter": "Filter:",
  "containers.filterAll": "Alle",
  "containers.filterInstalled": "Installiert",
  "containers.sectionsLabel": "Bereiche",
  "containers.selectAll": "Alle auswählen",
  "containers.selectedCount": "ausgewählt",
  "containers.backupSelected": "Auswahl sichern",
  "containers.restoreSelected": "Auswahl wiederherstellen (neuestes)",
  "containers.restoreSelectedConfirm": "Das NEUESTE Backup der ausgewählten Container wiederherstellen? Jeder wird gestoppt, seine Appdata ersetzt und neu erstellt.",
  "containers.clearSelection": "Leeren",
  "containers.working": "Arbeite…",
  "containers.batchStarted": "Backup gestartet. Es läuft auf dem Server, du kannst diesen Tab schließen.",
  "containers.batchAlreadyRunning": "Es läuft bereits ein Sammel-Backup.",
  "containers.batchRunning": "Sichere ausgewählte Container…",
  "containers.selfNote": "Das ist BombVault. Es sichert seinen eigenen Container nicht (würde sich selbst stoppen); seine Einstellungen werden über Discover wiederhergestellt.",

  // Post-Backup-Update-Prüfzeile (G4)
  "containers.updateCheckLabel": "Update-Prüfung",
  "containers.updateCheckUpToDate": "aktuell",
  "containers.updateCheckUpdated": "aktualisiert",
  "containers.updateCheckFailed": "Prüfung fehlgeschlagen",

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
  "restore.cancel": "Wiederherstellung abbrechen",
  "restore.cancelConfirmSafe": "Wiederherstellung abbrechen? Der bereits geschriebene Zielordner bleibt unverändert erhalten.",
  "restore.cancelConfirmInPlace":
    "{name} wird gerade wiederhergestellt. Ein Abbruch lässt diese Wiederherstellung unvollständig zurück. Möglicherweise musst du sie erneut ausführen. Trotzdem abbrechen?",
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
    "Läuft im Hintergrund. Du kannst dieses Panel schließen; das Ergebnis erscheint im Ausführungsverlauf.",
  "restore.completeContainer": "Wiederherstellung abgeschlossen: Container wird neu erstellt.",
  "restore.completeVM": "Wiederherstellung abgeschlossen: VM-Datenträger wurden ersetzt.",
  "restore.recreateComplete": "Neuerstellung abgeschlossen: der Container wurde neu erstellt.",

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
  "stack.restoreFinished": "Stack-Wiederherstellung abgeschlossen: Ergebnisse je Container im Verlauf.",
  "stack.memberRestored": "wiederhergestellt",
  "stack.memberStarted": "gestartet",

  "run.kindBackup": "Backup",
  "run.kindRestore": "Wiederherstellung",
  "run.kindUpdate": "Update",
  "run.kindExport": "Export",
  "run.kindDRDrill": "DR-Prüfung",
  "update.afterBackup": "Nach erfolgreichem Backup updaten",
  "update.afterBackupHint": "Zieht das Image und baut diesen Container direkt nach einem erfolgreichen Backup neu, du hast also immer zuerst einen frischen Wiederherstellungspunkt. Läuft zur Backup-Zeit (Backups laufen nacheinander), nicht zu einer festen Uhrzeit. Für Updates nach festem Zeitplan (nach Version gestaffelt) gibt es ShipLog.",
  "run.statusRunning": "Läuft",
  "run.statusSuccess": "Erfolgreich",
  "run.statusFailed": "Fehlgeschlagen",
  "run.statusSkipped": "Übersprungen",
  "run.historyTitle": "Ausführungsverlauf",
  "run.filterDay": "Tag:",
  "run.allDays": "Alle Tage",
  "run.recentTitle": "Letzte Läufe",
  "run.colKind": "Art",
  "run.colStatus": "Status",
  "run.colStarted": "Gestartet",
  "run.colFinished": "Abgeschlossen",
  "run.colContainer": "Container",

  "settings.title": "Einstellungen",
  "settings.encryptionLabel": "Passwort",
  "settings.encryptionOn": "Aktiviert (Passwort aus APP_KEY)",
  "settings.encryptionOff": "Deaktiviert (kein Passwort)",
  "settings.encryptionHint":
    "Die Verschlüsselung wird beim Anlegen eines Repositorys festgelegt, diese Einstellung entscheidet also nur über NEUE Repositorys. Bei einem bereits vorhandenen Repository liest BombVault den Modus direkt am Repository ab. Siehe Seite „Wiederherstellung“. Änderst du das gegen ein bestehendes Repository, kann restic es schlicht nicht mehr öffnen.",

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
  "settings.pathsHint": "Relative Unterpfade unter dem Host-Mount-Root ({root}). Auf Durchsuchen klicken, um Verzeichnisse zu navigieren, oder direkt einen Pfad eingeben.",
  "settings.containersPath": "Container-Pfad",
  "settings.vmsPath": "VMs-Pfad",
  "settings.flashPath": "Flash-Pfad",
  "settings.configPath": "Self-Backup-Pfad",
  "settings.restoreFolder": "Standard-Restore-Ordner",
  "settings.restoreFolderHint": "Wohin 'in einen Ordner wiederherstellen' Snapshots standardmäßig entpackt.",
  "settings.pathMode.local": "Lokal",
  "settings.pathMode.remote": "Remote",
  "settings.pathMode.localTip": "Lokaler Pfad auf diesem Host",
  "settings.pathMode.remoteTip": "Remote-restic-Repository",
  "settings.primaryRemote.title": "Sicherheitseinstellungen für Remote-Primärrepo",
  "settings.primaryRemote.hint": "Dieser Backup-Pfad ist ein Remote-restic-Repository. Er IST die primäre Kopie, keine Replik. Bandbreitenlimits, Append-only-Schutz und einen Wachstumsbudget-Alarm dafür hier konfigurieren, denselben Schutz, den eine Offsite-Kopie erhält.",
  "settings.primaryRemote.budgetHint": "Alarm auslösen, wenn dieses Repository ein Byte-Budget überschreitet (0 = aus).",
  "settings.offsiteTitle": "Offsite-Kopie (optional)",
  "settings.offsiteHint": "Nach jedem erfolgreichen lokalen Backup wird es zusätzlich per restic copy in ein zweites Repo repliziert. Ein Remote (rest:http://host:8000/repo, s3:…, b2:…) oder einen lokalen Unterpfad angeben; leer lassen zum Deaktivieren. Das lokale Backup bleibt primär.",
  "source.label": "Quelle:",
  "source.local": "Lokal",
  "source.offsite": "Offsite",
  "source.localTip": "Lokales Repository auf diesem Host",
  "source.offsiteTip": "Offsite-Kopie",
  "source.offsiteTarget": "Offsite-Ziel",
  "source.hint": "Restore und Löschen wirken nur auf die gewählte Quelle: ein lokales Backup zu löschen rührt die Offsite-Kopie nie an und umgekehrt.",
  "offsite.copyDomainTitle": "Offsite-Kopie {domain}",
  "offsite.schedulePlaceholder": "leer = nach jedem Backup · z.B. weekly Sun 03:00",
  "offsite.replicateNow": "Jetzt replizieren",
  "offsite.replicateStarted": "Replikation gestartet - sie läuft im Hintergrund; der Laufindikator zeigt den Fortschritt.",
  "offsite.replicating": "Repliziere…",
  "offsite.replicatingWithDuration": "Repliziere… ({duration})",
  "offsite.replicatingSnapshotPercent": "Repliziere… {percent} % gesamt (Snapshot {index} von {total})",
  "offsite.replicatingSnapshotPercentWithDuration": "Repliziere… {percent} % gesamt (Snapshot {index} von {total}) · {duration}",
  "offsite.overallPercentHint": "Gesamtfortschritt dieses Replikationslaufs, gezählt in Snapshots: restic meldet Fortschritt immer nur für einen einzelnen Snapshot, nie für einen ganzen Kopiervorgang. Snapshots sind unterschiedlich groß, das hier ist also ein Schätzwert.",
  "offsite.replicateFailed": "Replikation fehlgeschlagen",
  "offsite.test": "Verbindung testen",
  "offsite.testPrimary": "Primäres Ziel testen",
  "offsite.testing": "Teste…",
  "offsite.testOk": "erreichbar + initialisiert",
  "offsite.testUninitialized": "erreichbar, nicht initialisiert",
  "offsite.testFailed": "nicht erreichbar",
  "offsite.repoLocalHint": "Nimmt auch einen normalen Ordner unter dem \"Host Data\"-Mount. Relativ zu diesem Mount eintragen, ohne führendes /mnt: eine Freigabe unter /mnt/remotes/nas/bombvault wird als remotes/nas/bombvault eingetragen.",
  // Off-site-Einrichtungsassistent (v4 Ransomware-Schutz)
  "offsite.wizard.setup": "Einrichten…",
  "offsite.wizard.close": "Schließen",
  "offsite.wizard.step1": "1 · Backend wählen",
  "offsite.wizard.backendRest": "rest-server (empfohlen, append-only-fähig)",
  "offsite.wizard.backendRclone": "rclone-Remote",
  "offsite.wizard.backendS3": "Amazon S3 / S3-kompatibel",
  "offsite.wizard.backendPath": "Lokaler Pfad / eingebundene Freigabe (kein Server nötig)",
  "offsite.wizard.step2": "2 · Append-only-Server bereitstellen",
  "offsite.wizard.step2Hint": "Auf deiner Storage-Box ausführen, um einen restic rest-server mit --append-only zu starten. Das erzeugte Passwort wird nur einmal angezeigt.",
  "offsite.wizard.generate": "Deployment-Snippet erzeugen",
  "offsite.wizard.regenerate": "Neu erzeugen (neues Passwort)",
  "offsite.wizard.snippetError": "Snippet konnte nicht erzeugt werden",
  "offsite.wizard.passwordWarning": "Dieses Passwort wird nur EINMAL angezeigt und von BombVault nicht gespeichert. Jetzt sichern. Es wird für die Zugangsdaten unten benötigt und kann nicht wiederhergestellt werden.",
  "offsite.wizard.tlsNote": "Dieses Rezept nutzt einfaches HTTP, auf einem vertrauenswürdigen LAN oder VPN unproblematisch. Ist die Storage-Box über das Internet erreichbar, stelle rest-server hinter HTTPS (einen TLS-Reverse-Proxy), damit die Repository-Zugangsdaten nicht im Klartext übertragen werden.",
  "offsite.wizard.password": "Erzeugtes Passwort (sichern)",
  "offsite.wizard.step3": "3 · Repository-URL + Zugangsdaten",
  "offsite.wizard.repoUrl": "Off-site-Repository-URL",
  "offsite.wizard.repoUrlPlaceholder": "rest:http://192.168.x.x:8000/bombvault-containers/containers",
  "offsite.wizard.credentials": "REST-Server-Zugangsdaten",
  "offsite.wizard.credLoadError": "Vorhandene Zugangsdaten konnten nicht geladen werden. Vor dem Bearbeiten neu laden.",
  "offsite.wizard.step4": "4 · Append-only-Schutz aktivieren",
  "offsite.immutable": "Unveränderlich (append-only)",
  "offsite.immutableHint": "BombVault prunet/löscht off-site nicht mehr und überlässt die Aufbewahrung der Gegenseite. Die Gegenseite muss Löschungen wirklich verweigern, unten verifiziert.",
  "offsite.rcloneWarning": "rclone serve restic --append-only hat einen offenen Upstream-Retry-Bug, der Appends verlieren kann. Für unveränderliches Off-site wird rest-server empfohlen.",
  "offsite.s3Unverified": "Hinweis: S3-Append-only lässt sich nicht automatisch verifizieren. Bucket-Versionierung + Delete-Verbot manuell setzen; die Scorecard führt diese Domäne weiterhin als unverifiziert.",
  "offsite.tamperTestNow": "Append-only jetzt testen",
  "offsite.tamperTesting": "Teste…",
  "offsite.tamperOk": "Löschen verweigert, Append-only aktiv",
  "offsite.tamperFail": "Server hat das Löschen AKZEPTIERT, NICHT geschützt",
  "offsite.tamperUnverifiable": "für diesen Repository-Typ nicht überprüfbar",
  "offsite.tamperError": "Tamper-Test nicht eindeutig (Server nicht erreichbar)",
  "offsite.retention.title": "5 · Aufbewahrungsstrategie",
  "offsite.retention.farside": "Prune auf der Gegenseite (empfohlen)",
  "offsite.retention.window": "Wartungsfenster",
  "offsite.retention.grow": "Wachsen + Budget-Alarm",
  "offsite.retention.farsideHint": "restic forget --prune auf der Storage-Box selbst ausführen (BombVault bleibt append-only). Cron-Hinweis:",
  "offsite.retention.windowHint": "Kurzzeitig einen zweiten, nicht-append-only rest-server starten, prunen und wieder herunterfahren. Zugangsdaten werden nie gespeichert und ein Pflicht-Tamper-Retest folgt. Nur wenn Prune auf der Gegenseite nicht möglich ist.",
  "offsite.retention.windowRestOnly": "Gilt nur für REST-Server-Ziele. Für dieses Backend lässt sich kein temporärer zweiter Server betreiben. Nutze stattdessen Prune auf der Gegenseite oder Wachsen + Budget-Alarm.",
  "offsite.retention.growHint": "Off-site nie prunen; stattdessen alarmieren, wenn das Repo ein Byte-Budget überschreitet. Der ehrliche Standard, bis du einen Prune-Pfad wählst.",
  "offsite.retention.budget": "Wachstumsbudget (GB, 0 = aus)",

  // Additional off-site targets (multi-off-site) — extra per-domain copies
  "offsite.targets.title": "Weitere Off-site-Ziele",
  "offsite.targets.hint": "Diese Domäne an mehr als einen Off-site-Ort replizieren. Das primäre Ziel oben wird separat bearbeitet; die hier hinzugefügten sind zusätzliche Kopien.",
  "offsite.targets.scheduleNote": "Alle Ziele dieser Domäne replizieren nach dem Off-site-Zeitplan dieser Domäne. Es gibt keinen separaten Zeitplan pro Ziel.",
  "offsite.targets.add": "Ziel hinzufügen",
  "offsite.targets.none": "Noch keine weiteren Ziele.",
  "offsite.targets.edit": "Bearbeiten",
  "offsite.targets.test": "Testen",
  "offsite.targets.remove": "Entfernen",
  "offsite.targets.removing": "Entferne…",
  "offsite.targets.confirmRemove": "Entfernen bestätigen",
  "offsite.targets.save": "Ziel speichern",
  "offsite.targets.cancel": "Abbrechen",
  "offsite.targets.name": "Name (optional)",
  "offsite.targets.credsLabel": "Zugangsdaten",
  "offsite.targets.credsDefault": "Geteilt (Standard)",
  "offsite.targets.namePlaceholder": "z. B. Backblaze B2",
  "offsite.targets.repoRequired": "Repository-URL eingeben.",
  "offsite.targets.loadError": "Off-site-Ziele konnten nicht geladen werden.",
  "offsite.targets.retentionTitle": "Aufbewahrung (0 = alle behalten)",

  "settings.domains": "Domänen",
  "settings.domainsHint": "Jede Backup-Domäne einzeln ein- oder ausschalten. Aktivieren von VMs oder Flash blendet die zugehörige Registerkarte in der Seitenleiste ein.",
  "settings.containersEnabled": "Container",
  "settings.containersEnabledHint": "Container sichern und wiederherstellen, BombVaults Kerndomäne, standardmäßig aktiv.",
  "settings.vmsEnabled": "VMs",
  "settings.vmsEnabledHint": "VMs über SSH per libvirt sichern und wiederherstellen.",
  "settings.flashEnabled": "Flash",
  "settings.flashEnabledHint": "Den Unraid-USB-Flash-Bootstick (/boot) in ein restic-Repository sichern.",
  "settings.configEnabled": "Selbst-Backup",
  "settings.configEnabledHint": "BombVaults eigene Einstellungen, Ziele und Zugangsdaten sichern, damit eine frische Installation auch ihre Konfiguration wiederherstellen kann (Selbst-Backup).",
  "settings.receiverEnabled": "Empfänger-Dashboard",
  "settings.receiverEnabledHint": "Ein Append-only-Off-site-Repo überwachen, in das ein anderes BombVault schiebt (nur lesend)",
  "settings.fleetEnabled": "Fleet-Ansicht",
  "settings.fleetEnabledHint": "Den Schutzstatus verbundener BombVault-Instanzen einsehen (nur lesend)",
  "settings.schedule": "Zeitplan",
  "settings.scheduleOff": "aus",
  "settings.language": "Sprache",
  "settings.theme": "Design",
  "settings.save": "Speichern",
  "settings.saved": "Einstellungen gespeichert",
  "settings.error": "Fehler beim Speichern",

  // Retention
  "settings.retentionTitle": "Snapshot-Aufbewahrung",
  "settings.retentionHint": "Wie viele Backups pro Objekt behalten werden. Nach jedem Backup räumt restic ältere Snapshots gemäß dieser Regel auf. Alles 0 = alles behalten (aus).",
  "settings.imageMaintenanceTitle": "Image-Bereinigung & Update-Status",
  "settings.imageMaintenanceHint": "Wartung rund um das Container-Update nach dem Backup: das abgelöste Image aufräumen und Unraids eigenen Update-Status zurücksetzen.",
  "settings.pruneImageAfterUpdate": "Altes Image nach Update entfernen",
  "settings.pruneImageAfterUpdateHint": "Nachdem ein Container auf ein neueres Image aktualisiert wurde, das abgelöste alte Image löschen. Standardmäßig aus. Es zu behalten macht ein Zurückrollen billig (ein BombVault-Snapshot stellt Daten wieder her, nicht das alte Image). Ein geteiltes Basis-Image wird nie gelöscht.",
  "settings.registriesTitle": "Container-Registries",
  "settings.registriesHint": "Zugangsdaten für private Registries, genutzt wenn Update nach Backup einen Container auf ein neueres Image prüft (z. B. ein Sponsor-exklusives ghcr.io-Image). Öffentliche Images werden weiter anonym gezogen. Token leer lassen, um das gespeicherte zu behalten; das Entfernen einer Zeile löscht deren Zugangsdaten.",
  "settings.registriesEmpty": "Keine Registries konfiguriert. Alle Images werden anonym gezogen.",
  "settings.registryHost": "Registry-Host",
  "settings.registryUser": "Benutzername",
  "settings.registryToken": "Token / Passwort",
  "settings.registryRemove": "Entfernen",
  "settings.registryAdd": "Registry hinzufügen",
  "settings.retentionLast": "Letzte behalten",
  "settings.retentionDaily": "Täglich behalten",
  "settings.retentionWeekly": "Wöchentlich behalten",
  "settings.retentionMonthly": "Monatlich behalten",
  "settings.retentionLastInfo": "Behält die N neuesten Snapshots, unabhängig davon, wann sie erstellt wurden.",
  "settings.retentionDailyInfo": "Behält einen Snapshot für jeden der letzten N Kalendertage mit einem Backup, einen pro Tag, nicht N Backups.",
  "settings.retentionWeeklyInfo": "Behält einen Snapshot für jede der letzten N Kalenderwochen mit einem Backup.",
  "settings.retentionMonthlyInfo": "Behält einen Snapshot für jeden der letzten N Kalendermonate mit einem Backup.",
  "settings.retentionCombineInfo": "Die vier Regeln kombinieren sich per ODER: ein Snapshot bleibt erhalten, wenn ihn irgendeine Regel behalten würde. Sie addieren sich nicht zu einer festen Anzahl. Gilt separat für jedes gesicherte Objekt.",
  "settings.retentionLocal": "Lokales Repo",
  "settings.retentionOffsite": "Off-site-Repo",
  "settings.retentionOffsiteTitle": "Off-site-Aufbewahrung",
  "settings.retentionOffsiteHint": "Eine separate Regel für das Off-site-Repo, damit du es länger als Archiv behalten kannst. Alles 0 = jedes Off-site-Backup behalten (kein Off-site-Prune).",
  "settings.retentionOffsiteImmutableInfo": "Ein unveränderliches Off-site-Ziel wird hier nie geprunt, egal was hier eingestellt ist. Siehe Off-site › Aufbewahrungsstrategie, wie man es dennoch prunt.",

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

  // Dashboard widget (embeddable activity log)
  "settings.widget": "Dashboard-Widget",
  "settings.widgetHint": "BombVaults Aktivitätslog in Homepage, Organizr, Heimdall oder jedem Dashboard mit iframe-Widget einbetten: eine winzige eigenständige Seite nur mit dem Log.",
  "settings.widgetHow": "Die URL einfach ins iframe-Widget des Dashboards einfügen. Mehr braucht es nicht.",
  "settings.widgetAccess": "Das Token erlaubt nur Lesezugriff auf das Aktivitätslog, sonst nichts; Deaktivieren widerruft es sofort.",
  "settings.widgetEnglish": "Die Widget-Seite selbst ist nur auf Englisch. Eine Unraid-Dashboard-Variante kommt eventuell später.",
  "settings.widgetGenerate": "Token erzeugen",
  "settings.widgetRegenerate": "Neu erzeugen",
  "settings.widgetDisable": "Deaktivieren",
  "settings.widgetToken": "Widget-Token",
  "settings.widgetUrl": "Widget-URL",
  "settings.widgetUrlOnce": "Die URL (mit Token) wird nur direkt nach dem Erzeugen angezeigt. Für eine neue einfach neu erzeugen.",
  "settings.widgetPreview": "Live-Vorschau",
  "settings.fleet": "Fleet-Ansicht",
  "settings.fleetHint": "Anderen BombVault-Instanzen erlauben, den Schutzstatus dieser Instanz rein lesend abzufragen. Nichts hier erlaubt einer verbundenen Instanz, irgendeine Aktion auszulösen.",
  "settings.instanceName": "Instanzname",
  "settings.fleetHow": "Hier einen Token erzeugen, dann die URL dieser Instanz plus den Token auf der EIGENEN Fleet-Seite der anderen Instanz einfügen (Settings → Fleet auf der beobachtenden Box).",
  "settings.fleetAccess": "Der Token gewährt nur lesenden Zugriff auf die Schutz-Scorecard dieser Instanz und nichts weiter; Deaktivieren widerruft ihn sofort.",
  "settings.fleetToken": "Fleet-Token",
  "settings.fleetRegenerate": "Erneuern",
  "settings.fleetDisable": "Deaktivieren",
  "settings.fleetGenerate": "Token erzeugen",
  "settings.fleetTokenOnce": "Der Token wird nur direkt nach dem Erzeugen angezeigt. Erneuern, um einen neuen zu bekommen.",
  "settings.fleetTokenPasteHint": "Das hier zusammen mit der URL dieser Instanz in die Fleet-Seite der anderen Instanz einfügen.",
  "settings.fleetUrlHint": "Die eigene URL dieser Instanz (zum Eintragen bei einer anderen Instanz): {url}",
  "settings.dashTile": "Unraid-Dashboard-Widget",
  "settings.dashTileHint": "Bringt ein BombVault-Status-Widget direkt aufs Unraid-Dashboard. Ein kleines Begleit-Plugin fügt das Widget hinzu; BombVault kann es über die bestehende Host-SSH-Verbindung für dich installieren.",
  "settings.dashTileChecking": "Plugin-Status wird geprüft…",
  "settings.dashTileNoSsh": "Die Host-SSH-Verbindung ist nicht eingerichtet (Einstellungen → VM-Backup über SSH), daher kann BombVault das Plugin nicht für dich installieren. Installiere es manuell in Unraid unter Plugins → Install Plugin mit dieser URL:",
  "settings.dashTileCa": "Eventuell ist es auch in den Community Applications verfügbar. Nach BombVault Widget suchen.",
  "settings.dashTileNotInstalled": "Das Dashboard-Widget-Plugin ist nicht installiert.",
  "settings.dashTileConfirm": "Installieren installiert das bombvault-widget-Plugin über den regulären Unraid-Plugin-Mechanismus auf deinem Unraid-Host. Es erscheint wie jedes andere Plugin unter Plugins und kann dort (oder hier) jederzeit entfernt werden.",
  "settings.dashTileRepo": "Plugin auf GitHub ansehen",
  "settings.dashTileInstall": "Plugin installieren",
  "settings.dashTileInstalling": "Wird installiert…",
  "settings.dashTileInstalled": "Dashboard-Widget-Plugin ist installiert ({version})",
  "settings.dashTileInstalledNoV": "Dashboard-Widget-Plugin ist installiert",
  "settings.dashTileInstallOk": "Plugin installiert. Öffne das Unraid-Dashboard, um das neue BombVault-Widget zu sehen.",
  "settings.dashTileInstalledHint": "Das Widget erscheint auf dem Unraid-Dashboard. Falls es nicht sichtbar ist, in der Widget-Verwaltung des Dashboards aktivieren.",
  "settings.dashTileRemove": "Plugin entfernen",
  "settings.dashTileRemoving": "Wird entfernt…",
  "settings.dashTileRemoveOk": "Plugin entfernt. Es erscheint nicht mehr auf dem Unraid-Dashboard.",

  // Off-site (rclone)
  "rclone.title": "Off-site (rclone)",
  "rclone.hint": "rclone-Konfiguration einfügen, um in die Cloud zu sichern (Backblaze B2, S3, Google Drive, …). Wird verschlüsselt gespeichert. SMB/NFS brauchen kein rclone: Freigabe in Unraid mounten und einen Backup-Pfad daraufzeigen.",
  "rclone.configured": "Konfigurierte Remotes",
  "rclone.pathHint": "Dann einen Backup-Pfad auf \"rclone:<remote>:<bucket>/pfad\" setzen, um diese Domäne off-site zu senden.",
  "cloud.title": "Cloud-Zugangsdaten (S3 / restic REST)",
  "cloud.hint": "Zugangsdaten für off-site restic-Backends, ohne rclone. Nach dem Speichern einen Backup-Pfad auf ein Remote-Repo setzen, z.B. s3:s3.amazonaws.com/bucket/pfad, rest:http://host:8000/repo, b2:bucket:pfad oder sftp:user@host:/repo. Secrets werden verschlüsselt gespeichert und nie wieder angezeigt.",
  "cloud.secretSet": "gespeichert (leer lassen zum Beibehalten)",
  "cloud.storageClass.label": "Off-site-Speicherklasse",
  "cloud.storageClass.default": "(Standard des Anbieters)",
  "cloud.storageClass.hint": "Gilt nur für native S3-Backends (Repositories, die mit s3: beginnen). Deep-Archive-Stufen (Glacier Flexible Retrieval, Deep Archive) werden absichtlich nicht angeboten, weil sie die restic-Wiederherstellung brechen.",
  "cloud.credSets.title": "Zusätzliche Zugangsdaten-Sätze",
  "cloud.credSets.hint": "Gib einem oder mehreren Off-site-Zielen eigene S3- oder restic-REST-Zugangsdaten, statt sich die oben geteilten zu teilen.",
  "cloud.credSets.add": "Zugangsdaten-Satz hinzufügen",
  "cloud.credSets.name": "Name",
  "cloud.credSets.none": "Noch keine zusätzlichen Zugangsdaten-Sätze.",
  "rclone.save": "Konfig speichern",
  "notify.title": "Benachrichtigungen",
  "notify.hint": "Lass dich benachrichtigen, wenn ein Backup fertig ist, und lege unten fest, bei welchen Ereignissen. Unraid-Benachrichtigungen funktionieren bereits im einfachen Modus; weitere Versandkanäle (Webhook, Matrix, Healthchecks, E-Mail) findest du unter Erweitert.",
  "notify.on": "Benachrichtigen",
  "notify.onNever": "Nie",
  "notify.onFailure": "Nur bei Fehler",
  "notify.onAlways": "Bei Erfolg und Fehler",
  "notify.channelsTitle": "Benachrichtigungskanäle",
  "notify.channelsHint": "Richte die Webhook-, Matrix- und E-Mail-Kanäle ein, über die die oben konfigurierten Benachrichtigungen verschickt werden.",
  "notify.webhook": "Webhook-URL",
  "notify.webhookChannel": "Webhook",
  "notify.webhookFormat": "Webhook-Format",
  "notify.apprise": "Apprise",
  "notify.appriseUrl": "Apprise-API-URL",
  "notify.appriseTags": "Tags (optional)",
  "notify.appriseHint": "Zeige auf den /notify/<key>-Endpunkt deines Apprise-API-Containers, um an dessen über 100 Dienste (Telegram, Pushover, Signal, …) zu verteilen. Tags erreichen nur einen Teil der Ziele dieses Keys; leer lassen für alle.",
  "notify.matrix": "Matrix",
  "notify.matrixHomeserver": "Homeserver-URL",
  "notify.matrixToken": "Access-Token",
  "notify.matrixRoom": "Raum-ID",
  "notify.healthchecksTitle": "Healthchecks",
  "notify.healthchecks": "Healthchecks.io-Ping-URL",
  "notify.healthchecksLifecycle": "Healthchecks wird über den ganzen Backup-Lebenszyklus gepingt (Start, Erfolg und Fehler), sobald eine URL gesetzt ist, unabhängig von der 'Benachrichtigen bei'-Einstellung oben, damit die Prüfung auch bei nur-Fehler-Benachrichtigungen bei Erfolg grün bleibt.",
  "notify.hcPerDomain": "Prüfungen pro Domäne (erweitert)",
  "notify.hcPerDomainHint": "Lass ein Feld leer, um die globale URL oben zu verwenden. Eine Domäne mit eigener URL erhält ihre eigene Prüfung, mit eigener Laufzeit und Historie.",
  "notify.scheduledSummary": "Geplante Läufe zusammenfassen",
  "notify.scheduledSummaryHint": "EINE Zusammenfassung pro geplantem Backup-Lauf senden (z. B. 42 von 45 erfolgreich) statt einer eigenen Nachricht für jeden Container/jede VM. Healthchecks wird bereits zusammengefasst. Manuelle Backups benachrichtigen weiterhin pro Objekt.",
  "notify.notifyOnUpdate": "Bei Container-Update benachrichtigen",
  "notify.notifyOnUpdateHint": "Wenn Update nach Backup einen Container auf ein neueres Image hebt, eine Nachricht senden, damit du prüfen kannst, ob er noch läuft. Feuert pro aktualisiertem Container (Updates sind selten).",
  "notify.unraid": "Unraid-Benachrichtigungen",
  "notify.unraidHint": "An Unraids eigenes Benachrichtigungssystem senden (das an Pushover, E-Mail, Discord, … weiterleiten kann). Läuft über die SSH-Verbindung aus Einstellungen → VM Backup over SSH, der Schlüssel muss dort also autorisiert sein; libvirt/VMs sind aber NICHT nötig (ein \"libvirt not reachable\"-Ergebnis ignorieren, wenn du keine VMs sicherst). Zum Prüfen unten \"Test senden\".",
  "notify.unraidPlatformMismatch": "BombVault hat diesen Host als \"{platform}\" erkannt, nicht als Unraid. Unraid-Benachrichtigungen bleiben deaktiviert, obwohl diese Option aktiviert ist. Falls dies wirklich ein Unraid-Host ist, prüfe, ob das /boot des Hosts nach /host/boot im Container gemountet ist (siehe die BombVault-Unraid-Vorlage), und starte den Container neu.",
  "notify.smtp": "E-Mail (SMTP)",
  "notify.smtpHost": "SMTP-Host",
  "notify.smtpPort": "Port",
  "notify.smtpUser": "Benutzername",
  "notify.smtpPass": "Passwort",
  "notify.smtpFrom": "Absenderadresse",
  "notify.smtpTo": "Empfängeradresse",
  "notify.smtpTls": "Verschlüsselung",
  "notify.test": "Test senden",
  "notify.tested": "Test gesendet",

  // Integrity (restic check)
  "integrity.title": "Integrität & Wartung",
  "integrity.hint": "Struktur eines Repos verifizieren (restic check), verwaiste Locks eines abgebrochenen Laufs entfernen oder per Prune Speicher gelöschter Backups freigeben.",
  "integrity.verify": "Prüfen",
  "integrity.checking": "Prüfe…",
  "integrity.ok": "Intakt",
  "integrity.failed": "Prüfung fehlgeschlagen",
  "integrity.failedShort": "✗ Fehlgeschlagen",
  "integrity.unlock": "Entsperren",
  "integrity.prune": "Aufräumen",
  "integrity.verifyHint": "restic check ausführen, um Struktur und Metadaten zu verifizieren.",
  "integrity.unlockHint": "Verwaiste Repo-Locks eines abgestürzten/abgebrochenen Laufs entfernen (behebt „repository is already locked“).",
  "integrity.pruneHint": "Retention anwenden und Speicher freigeben (ohne Policy nur Speicher; kann dauern).",
  "integrity.pruneConfirm": "Aufräumen wendet jetzt deine Retention an: entfernt Snapshots jenseits deiner Keep-Regeln (last/daily/weekly/monthly) und gibt Speicher frei. Ohne Policy wird nur Speicher freigegeben. Fortfahren?",
  "integrity.appendOnly": "Append-only-Prüfung",
  "integrity.appendOnlyHint": "Beweist, dass das Off-site-Repository Löschungen weiterhin verweigert (Append-only-Schutz), dieselbe Prüfung wie der Tamper-Test im Off-site-Assistenten.",
  "integrity.appendOnlyLast": "Append-only-Schutz · Zuletzt geprüft {time}",
  "integrity.appendOnlyNever": "Append-only-Schutz · nie geprüft",

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
  "drill.targetVM": "Testziel (VM)",
  "drill.targetMostRecent": "Neuestes Backup",
  "drill.drNote": "Eine echte Wiederherstellung entpackt den neuesten Off-site-Snapshot in eine temporäre Sandbox, verifiziert ihn und räumt danach auf. Dabei werden echte Daten geladen, das kann dauern.",
  "drill.runDR": "Echte Wiederherstellung starten",
  "drill.runningDR": "Stelle wieder her…",
  "drill.confirmDR": "Dies führt eine ECHTE Wiederherstellung des neuesten Off-site-Snapshots in eine temporäre Sandbox durch, um die Wiederherstellbarkeit zu beweisen, und löscht sie danach. Dabei werden echte Daten geladen, das kann dauern. Fortfahren?",
  "drill.provenOffsite": "nachweislich aus Off-site wiederherstellbar",
  "drill.offsiteVerified": "Off-site verifiziert",

  // Drill reason + check labels (#30)
  "drill.checkOffsiteDr": "Off-site-DR-Wiederherstellung",
  "drill.checkLocal": "Lokale Integritätsprüfung",
  "drill.failReasonPrefix": "Grund:",
  "drill.runOffsiteDr": "Off-site-DR-Prüfung starten",
  "drill.rerunOffsiteDr": "Off-site-DR-Prüfung erneut ausführen",
  "drill.runningOffsiteDr": "Off-site-DR-Prüfung läuft…",

  // Off-site DR opt-out (#37)
  "settings.offsiteDrills": "Geplanter Off-site-DR-Test",
  "settings.offsiteDrillsHelp": "Stellt den vollständigen Off-site-Snapshot nach dem Test-Zeitplan wieder her, um die Wiederherstellung aus der Ferne zu beweisen. Dabei wird bei jedem Lauf das gesamte Backup erneut heruntergeladen, was bei kostenpflichtigen Clouds (zum Beispiel Backblaze B2) Egress-Kosten verursacht. Schalte ihn aus, um nur die kostenlose lokale Integritätsprüfung zu behalten und die Off-site-DR-Prüfung manuell auszuführen.",
  "drill.manualOnly": "Off-site-DR: nur manuell",
  "drill.manualOnlyTitle": "Der geplante Off-site-DR-Test ist aus. Führe die Off-site-Prüfung manuell über den Button aus.",

  // Pre/post-backup hooks
  "hooks.title": "Backup-Hooks",
  "hooks.hint": "Befehle laufen im Container mit sh -c. Der Pre-Befehl läuft vor dem Backup; nutze ihn, um Daten vorzubereiten, die mitgesichert werden sollen, etwa eine Datenbank in die appdata des Containers zu dumpen. Schlägt der Pre-Befehl fehl, wird das Backup abgebrochen. Der Post-Befehl läuft, nachdem der Container wieder gestartet wurde, und sein Fehler wird nur geloggt. Hooks führen nur Befehle aus, sie fügen dem Backup keine zusätzlichen Ordner hinzu.",
  "hooks.pre": "Pre-Backup-Befehl",
  "hooks.post": "Post-Backup-Befehl",
  "folders.title": "Gesicherte Ordner",
  "folders.hint": "Wähle, welche gemappten Ordner dieses Containers gesichert werden. Der appdata-Ordner ist standardmäßig ausgewählt. Hake weitere an, um sie einzuschließen, oder füge einen eigenen Pfad unterhalb des Host-Mounts hinzu. Hakst du alles ab, gilt wieder die automatische appdata-Erkennung.",
  "folders.appdataDefault": "appdata (Standard)",
  "folders.notReachable": "nicht unter dem Host-Mount, kann nicht gesichert werden",
  "folders.customMissing": "kein Datenordner gefunden (hier gibt es nichts zu sichern)",
  "folders.customPlaceholder": "/mnt/user/irgendein/ordner",
  "folders.addCustom": "Ordnerpfad hinzufügen",
  "folders.add": "Hinzufügen",
  "folders.save": "Ordner speichern",
  "folders.saved": "Gespeichert",
  "folders.empty": "Keine gemappten Ordner für diesen Container gefunden.",
  "stophook.title": "Andere Container stoppen",
  "stophook.hint": "Diese anderen Container während des Backups dieses Containers stoppen (zum Beispiel eine Datenbank) und danach wieder starten.",
  "stophook.noCandidates": "Keine anderen installierten Container gefunden.",
  "stophook.remove": "{name} entfernen",
  "export.button": "Export (Plain-tar)",
  "export.exportedTo": "Exportiert nach:",
  "backup.configOnly": "Nur Konfiguration: keine Datenordner (Definition für Wiederherstellung gesichert)",

  // Per-container exclude patterns (#36)
  "excludes.title": "Ausschlussmuster",
  "excludes.hint": "Ein Muster pro Zeile. Ein Container-Pfad (z. B. /config/Library/.../Cache) wird gegen das gesicherte Volume abgeglichen; ein reiner Name wie .git passt in jeder Tiefe. Klammerlisten wie {a,b} werden nicht unterstützt; nutze je eine Zeile.",
  "excludes.placeholder": "/config/Library/Application Support/Plex Media Server/Cache\n/config/Library/Application Support/Plex Media Server/Metadata\n.git",
  "excludes.save": "Ausschlüsse speichern",
  "excludes.saved": "Ausschlüsse gespeichert",
  "excludes.error": "Ausschlüsse konnten nicht gespeichert werden",
  "excludes.resolvedTo": "wird aufgelöst zu:",
  "excludes.noMatch": "Wird unverändert an restic übergeben (kein erkannter Container-Pfad).",
  "excludes.excludesNothing": "Das Volume dieses Ordners ist nicht im Backup, diese Zeile schließt also nichts aus.",
  // Exclude preview status (#38)
  "excludes.willExclude": "wird vom Backup ausgeschlossen",
  "excludes.matchesAnywhere": "wird überall ausgeschlossen, wo es vorkommt",
  // Ausschluss-Assistent — serverseitiger Junk/Groß-Ordner-Scan mit Ein-Klick-Ausschluss
  "excludes.assistTitle": "Ausschluss-Assistent",
  "excludes.assistHint": "Durchsucht die gesicherten Ordner dieses Containers nach bekannten Cache-/Temp-/Log-Ordnern und ungewöhnlich großen Verzeichnissen, damit du sie ausschließen und das Backup verkleinern kannst.",
  "excludes.assistScan": "Nach Junk- und großen Ordnern suchen",
  "excludes.assistRescan": "Erneut scannen",
  "excludes.assistScanning": "Scanne…",
  "excludes.assistScanFailed": "Scan fehlgeschlagen",
  "excludes.assistTruncated": "Der Scan hat sein Zeitlimit in {path} erreicht. Ordner danach wurden nicht mehr geprüft.",
  "excludes.assistUnexamined": "Diese gesicherten Ordner wurden gar nicht geprüft: {paths}",
  "excludes.assistUnreadable": "Diese gesicherten Ordner ließen sich nicht lesen, alles darin fehlt deshalb in dieser Liste: {paths}",
  "excludes.assistPathsUnavailable": "Die gesicherten Ordner dieses Containers sind gerade nicht erreichbar. Prüfe, ob das Array oder die Freigabe mit diesen Ordnern eingebunden ist.",
  "excludes.assistNothingFound": "Nichts mehr auszuschließen: kein Junk und keine übergroßen Ordner im Backup dieses Containers gefunden.",
  "excludes.assistSourceSnapshot": "Die Größen stammen aus dem Backup vom {when} und sind damit exakt.",
  "excludes.assistSourceLive": "Dieser Container hat noch kein Backup, deshalb stammen die Größen aus einem Live-Scan der Ordner.",
  "excludes.assistSourceLiveRequested": "Die Größen stammen aus einem Scan der Ordner, so wie sie gerade jetzt sind.",
  "excludes.assistSourceLiveNotInSnapshot": "Das letzte Backup enthält die jetzt ausgewählten Ordner nicht, deshalb stammen die Größen aus einem Live-Scan der Ordner.",
  "excludes.assistSnapshotStale": "Dieses Backup ist älter als einen Tag, alles danach Entstandene fehlt also in dieser Liste.",
  "excludes.assistScanCurrent": "Ordner im jetzigen Zustand prüfen",
  "excludes.assistIndexFailed": "Der Backup-Index konnte nicht zu Ende gelesen werden.",
  "excludes.assistScanLive": "Stattdessen die Ordner scannen",
  "excludes.assistSizeAtLeast": "mindestens {size}",
  "excludes.assistSizeMinimumTip": "Der Scan ist in diesem Ordner stehengeblieben, diese Größe ist also ein Mindestwert.",
  "excludes.assistReasonCache": "Bekannter Cache",
  "excludes.assistReasonLarge": "Großer Ordner",
  "excludes.assistExclude": "Ausschließen",
  "excludes.assistCurrent": "Aktuelle Ausschlüsse",
  "excludes.assistNoneYet": "Noch keine. Wähle oben einen Vorschlag oder tippe einen ein.",
  "excludes.assistRemove": "Ausschluss entfernen",
  "excludes.assistRemoveLine": "Ausschluss {line} entfernen",

  // Appearance / Accent
  "settings.colors": "Farben",
  "settings.accentColor": "Akzentfarbe",
  "settings.accentPresets": "Voreinstellungen",
  "settings.accentPreset": "Voreinstellung",
  "settings.accentReset": "Akzentfarbe und Voreinstellungen zurücksetzen",
  "settings.shape": "Ecken",
  "settings.shapeHint": "Gilt für Karten, Knöpfe, Reiter, Eingabefelder und Abzeichen zugleich.",
  "settings.shape.round": "Rund",
  "settings.shape.soft": "Leicht",
  "settings.shape.square": "Eckig",
  "settings.motion": "Animationen",
  "settings.motionHint": "Wie stark sich alle Animationen der App bewegen: ein manueller Regler neben der Systemeinstellung für reduzierte Bewegung, der sie nie überschreibt.",
  "settings.motion.off": "Aus",
  "settings.motion.subtle": "Dezent",
  "settings.motion.full": "Voll",
  "settings.rainbow": "Regenbogen-Modus",
  "settings.rainbowHint": "Jede Zeile in einer Liste bekommt eine eigene Farbe aus einer festen Auswahl von acht, statt dass alles dieselbe Akzentfarbe hat. Das macht lange Listen auf einen Blick leichter unterscheidbar.",
  "settings.rainbowReactive": "Reaktiver Modus",
  "settings.rainbowReactiveHint": "Wenn aktiv, bleiben farbige Zeilen und Elemente neutral, bis du sie mit der Maus berührst oder sie gerade laufen oder ausgewählt sind. Die Farbe erscheint also nur bei Bedarf. Wenn deaktiviert, zeigen alle farbigen Zeilen und Elemente ihre Farbe durchgehend.",
  "settings.rainbowRotate": "Farbenrotation",
  "settings.rainbowRotateHint": "Verschiebt, welche Farbe der Palette als Position 0 gilt, damit dieselbe Liste nicht bei jedem Aktivieren des Regenbogen-Modus oder jedem Neuladen der Seite mit genau derselben Farbe beginnt.",
  "settings.rainbowPalette": "Palettenfarbe",
  "settings.rainbowPaletteLabel": "Farbpalette",
  "settings.rainbowPaletteReset": "Farbpalette zurücksetzen",
  "settings.quietToasts": "Leise Benachrichtigungen",
  "settings.quietToastsHint": "Blendet Erfolgsmeldungen wie Speicher- und Kopierbestätigungen aus. Fehler und alles andere, das deine Aufmerksamkeit braucht, werden weiterhin angezeigt.",

  // Dashboard stat cards
  "dashboard.statContainers": "Container",
  "dashboard.statVMs": "VMs",
  "dashboard.statActiveJobs": "Aktive Pläne",
  "dashboard.statPausedJobs": "Pausierte Pläne",
  "dashboard.statErrors": "Fehler",
  "dashboard.statMissingContainers": "Fehlende Container",
  "dashboard.statMissingVMs": "Fehlende VMs",

  // Error detail panel (#126) — the modal opened from the dashboard error count
  "errorPanel.title": "Sicherungsfehler",
  "errorPanel.resolve": "Erledigt",
  "errorPanel.resolveAll": "Alle als erledigt markieren",
  "errorPanel.affected": "Betroffen",
  "errorPanel.empty": "Keine offenen Sicherungsfehler.",
  "errorPanel.count": "{count} Vorkommen",
  "errorPanel.filterPlaceholder": "Fehler filtern…",

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
  "dashboard.domainConfig": "Selbst-Backup",

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
  "ransomware.drillFailed": "Restore-Test fehlgeschlagen",
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

  // Storage forecast line (growth/week + time-to-full, riding /api/stats)
  "dashboard.forecastGrowth": "Wächst {bytes}/Woche",
  "dashboard.forecastShrink": "Schrumpft {bytes}/Woche",
  "dashboard.forecastFull": "Repo-Volume voll in ~{weeks} Wochen",
  "dashboard.forecastFullOneWeek": "Repo-Volume voll in ~1 Woche",
  "dashboard.forecastFullOverYear": "Repo-Volume voll in > 1 Jahr",
  "dashboard.forecastFree": "{bytes} frei",

  // Domain filters + dashboard duration (#39/#40/#41)
  "dashboard.duration": "Dauer",
  "containers.searchPlaceholder": "Container suchen…",
  "vms.searchPlaceholder": "VMs suchen…",
  "filter.all": "Alle",
  "filter.scheduled": "Geplant",
  "filter.notScheduled": "Nicht geplant",
  "filter.backedUp": "Gesichert",
  "filter.neverBackedUp": "Nie gesichert",
  "filter.schedule": "Zeitplan",
  "filter.backup": "Backup",
  "filter.noMatch": "Keine Einträge entsprechen den aktuellen Filtern.",

  // Jobs page
  "jobs.containersSection": "Container",
  "jobs.vmsSection": "VMs",
  "jobs.flashSection": "Flash",
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
  "cadence.everyNUnavailable": "Alle N Tage ist für diesen Zeitplan nicht verfügbar. Er läuft zu festen Zeiten und hat nichts, woran er den Abstand abzählen könnte. Nimm stattdessen einen der anderen Modi.",
  "cadence.time": "Zeit",
  "cadence.days": "Tage",
  "cadence.every": "Alle",
  "cadence.daysUnit": "Tage",
  "cadence.fmtDaily": "täglich um {time} Uhr",
  "cadence.fmtWeekly": "wöchentlich ({days}) um {time} Uhr",
  "cadence.fmtEveryN": "jeden {n}. Tag um {time} Uhr",
  // Cron cadence mode (#107)
  "cadence.cron": "Cron",
  "cadence.cronExpr": "Ausdruck",
  "cadence.cronPlaceholder": "Minute Stunde Tag Monat Wochentag",
  "cadence.cronInvalid": "Ungültiger Cron-Ausdruck. Erwartet 5 Felder: Minute Stunde Monatstag Monat Wochentag (z. B. 0 */6 * * *).",
  "cadence.cronValid": "Gültiger Cron-Ausdruck",
  "cadence.cronNext": "als Nächstes: {first}, danach {rest}",
  "cadence.cronExamples": "Beispiele",
  "cadence.cronExEvery6h": "alle 6 Stunden",
  "cadence.cronExWeekdays": "werktags um 02:30",
  "cadence.cronExMonthly": "am 1. jedes Monats",
  "cadence.fmtCron": "Cron: {expr}",
  "timePicker.hour": "Stunde",
  "timePicker.minute": "Minute",
  "time.justNow": "gerade eben",
  "time.minuteAgo": "vor 1 Minute",
  "time.minutesAgo": "vor {n} Minuten",
  "time.hourAgo": "vor 1 Stunde",
  "time.hoursAgo": "vor {n} Stunden",
  "time.dayAgo": "vor 1 Tag",
  "time.daysAgo": "vor {n} Tagen",
  "folder.browse": "Durchsuchen…",
  "folder.browseTitle": "Ordner durchsuchen",
  "folder.use": "Diesen Ordner verwenden",
  "folder.none": "Keine Unterordner",
  "folder.loading": "Lädt…",
  "folder.newFolder": "Neuer Ordner",
  "folder.newFolderPlaceholder": "Name des neuen Ordners",
  "folder.creating": "Lege an…",
  "folder.createFailed": "Ordner konnte nicht angelegt werden",
  "folder.pathHint": "Pfad muss ein relativer Unterpfad sein (kein führendes / oder ..)",
  "folder.couldNotRead": "Verzeichnis konnte nicht gelesen werden",
  "folder.browseFailed": "Durchsuchen fehlgeschlagen",
  "containers.subtitle": "Container-Backups, Zeitpläne und Wiederherstellungen verwalten.",
  "containers.emptyDocker": "Keine Container gefunden. Läuft Docker?",
  "containers.bulkResult": "{ok} ok, {fail} fehlgeschlagen",
  "vm.method.saveFailed": "Backup-Methode konnte nicht geändert werden. Sie wurde nicht umgestellt.",
  "jobs.noContainersIncluded": "Keine Container im Zeitplan enthalten.",
  "jobs.syncSchedules": "Container-Zeitplan auch für VMs, Flash und Ordner verwenden",
  "jobs.syncSchedulesHint": "Wenn aktiviert, folgen VMs, Flash und Ordner dem Container-Zeitplan statt ihrem eigenen. Schalte es aus, um für jede Domäne einen eigenen Rhythmus festzulegen.",
  "jobs.flashScheduleHint": "Sichert den Unraid-USB-Flash-Bootstick (/boot) zur geplanten Zeit.",
  "jobs.vmIncludeHint": "Sichert jede VM mit aktiviertem „In Zeitplan aufnehmen“ (pro VM im VMs-Tab einstellbar).",
  "schedule.includeAll": "Alle in den Zeitplan",
  "schedule.excludeAll": "Alle aus dem Zeitplan",
  "schedule.updateFailed": "Zeitplan konnte nicht aktualisiert werden",
  // Pro-Element-Zeitpläne (#121)
  "jobs.noVMsIncluded": "Keine VMs im Zeitplan enthalten.",
  "settings.perItemSchedules": "Zeitpläne pro Element",
  "settings.perItemSchedulesHint": "Lässt einzelne Container und VMs den Domänen-Zeitplan mit einem eigenen Rhythmus überschreiben. Standardmäßig aus; ein leeres Element behält den Domänen-Zeitplan.",
  "schedule.overrideTitle": "Zeitplan-Überschreibung",
  "schedule.overrideUsesDefault": "Nutzt den Domänen-Zeitplan",
  "schedule.overrideEdit": "Überschreibung festlegen",
  "schedule.overrideHint": "Leer nutzt den Domänen-Zeitplan.",
  "schedule.overrideSaved": "Überschreibung gespeichert",

  // Auth / Login
  "auth.loginTitle": "BombVault",
  "auth.passwordLabel": "Passwort",
  "auth.signIn": "Anmelden",
  "auth.signingIn": "Anmeldung läuft…",
  "auth.invalidPassword": "Falsches Passwort",
  "auth.loginError": "Anmeldung fehlgeschlagen",

  // Settings — Security card
  "auth.security": "Sicherheit",
  "auth.authOff": "Authentifizierung ist deaktiviert. Alle LAN-Nutzer haben vollen Zugriff.",
  "auth.authOn": "Authentifizierung ist aktiviert.",
  "auth.setPassword": "Passwort setzen",
  "auth.changePassword": "Passwort ändern",
  "auth.confirmPassword": "Passwort bestätigen",
  "auth.passwordMismatch": "Passwörter stimmen nicht überein",
  "auth.passwordSaved": "Passwort gespeichert",
  "auth.passwordCleared": "Authentifizierung deaktiviert",
  "auth.passwordHint":
    "Beide Felder leer lassen, um die Authentifizierung zu deaktivieren. BombVault hat root-ähnliche Host-Kontrolle. Ein Passwort ist empfohlen, wenn diese Instanz für nicht vertrauenswürdige LAN-Nutzer erreichbar ist.",
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
  "common.showValue": "Wert anzeigen",
  "common.hideValue": "Wert verbergen",
  "confirmDialog.title": "Bestätigen",
  "toast.dismiss": "Benachrichtigung schließen",
  "common.confirm": "Bestätigen",
  "common.cancel": "Abbrechen",

  // Fehler-Rückfalltexte — siehe den englischen Block für die Herkunft.
  "common.actionFailed": "Aktion fehlgeschlagen",
  "common.deleteFailed": "Löschen fehlgeschlagen",
  "common.removeFailed": "Entfernen fehlgeschlagen",
  "common.saveFailed": "Speichern fehlgeschlagen",
  "common.discoverFailed": "Suche fehlgeschlagen",
  "common.checkFailed": "Prüfung fehlgeschlagen",
  "common.networkError": "Netzwerkfehler",
  "common.backupFailed": "Sicherung fehlgeschlagen",
  "common.restoreFailed": "Wiederherstellung fehlgeschlagen",
  "common.compareFailed": "Vergleich fehlgeschlagen",
  "common.loadBackupsFailed": "Sicherungen konnten nicht geladen werden",
  "common.deleteBackupsFailed": "Sicherungen konnten nicht gelöscht werden",
  "containers.loadFailed": "Container konnten nicht geladen werden",
  "containers.backupStartFailed": "Sicherung konnte nicht gestartet werden",
  "containers.updateSettingFailed": "Einstellung konnte nicht geändert werden",
  "vms.loadFailed": "VMs konnten nicht geladen werden",
  "files.loadSetsFailed": "Ordnersets konnten nicht geladen werden",
  "flash.loadBackupsFailed": "Flash-Sicherungen konnten nicht geladen werden",
  "config.loadBackupsFailed": "Einstellungs-Sicherungen konnten nicht geladen werden",
  "config.loadSettingsFailed": "Aktuelle Einstellungen konnten nicht geladen werden",
  "dashboard.loadRunsFailed": "Läufe konnten nicht geladen werden",

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
  "flash.restoreNote": "Restore lädt ein ZIP des Snapshots herunter. Der laufende /boot wird nie angefasst. Das .zip direkt in den Unraid-USB-Creator geben oder auf einen frischen USB-Stick entpacken, um deinen Flash neu aufzubauen.",
  "flash.none": "Noch keine Flash-Backups. Oben eines starten.",
  "settings.exportsEncryptionTitle": "Export- und Repository-Verschlüsselung",
  "settings.exportsEncryptionHint": "Einstellungen für die Verschlüsselung der Plain-Export-Artefakte und die Verschlüsselung der restic-Repositories selbst.",
  // Geplanter Flash-ZIP-Export (#28): ein einfaches .zip, das nach jedem Flash-Backup in einen Ordner geschrieben wird.
  "flash.zipExport.title": "Flash-ZIP-Export",
  "flash.zipExport.hint": "Nach jedem Flash-Backup den Snapshot zusätzlich als einfaches .zip in einen Ordner schreiben, bereit für Off-Server-Sync (Syncthing, rclone, ein Cloud-Laufwerk).",
  "flash.zipExport.enable": "Nach jedem Flash-Backup ein ZIP exportieren",
  "flash.zipExport.enableHint": "Bei jedem erfolgreichen Flash-Backup wird der Snapshot als .zip in den Ordner unten geschrieben.",
  "flash.zipExport.path": "Export-Ordner",
  "flash.zipExport.pathHint": "Relativer Unterpfad unter dem Host-Mount-Root, in den das .zip geschrieben wird. Auf einen Syncthing-/rclone-Ordner zeigen lassen, um den Flash automatisch vom Server zu bekommen.",
  "flash.zipExport.keepHistory": "Exportierte ZIP-Dateien behalten",
  "flash.zipExport.keepHistoryHint": "Aus: eine einzige flash-latest.zip behalten, die jedes Mal überschrieben wird. An: die neuesten N flash-<Datum>.zip-Dateien mit Zeitstempel behalten. Das ist unabhängig von der restic-Aufbewahrung: aus behält eine einzige, immer überschriebene Datei (füllt das Ziel nie); an behält die neuesten N und löscht ältere.",
  "flash.zipExport.keepN": "Zu behaltende ZIPs",
  "flash.zipExport.keepNHint": "Die neuesten N ZIPs mit Zeitstempel werden behalten, ältere automatisch gelöscht.",
  "flash.zipExport.latestNote": "Eine einzige flash-latest.zip wird nach jedem Backup überschrieben.",
  "flash.zipExport.plaintextWarn": "Das exportierte .zip ist nicht verschlüsselt, auch wenn dein Flash-Repository es ist. Synce es nur an einen vertrauenswürdigen Ort.",
  "flash.zipExport.pathRequired": "Wähle einen Export-Ordner, um dies zu aktivieren.",
  "export.encrypt.hint": "Die restic-Repositories sind bereits verschlüsselt. Dies verschlüsselt optional die Plain-Export-Artefakte (Container- und VM-tar.gz samt xml-Beidateien sowie das Flash-Zip) mit age, damit sie sicher außerhalb des Servers gespeichert oder bewegt werden können.",
  "export.encrypt.ageInfo": "age (age-encryption.org) ist ein kleines, modernes Verschlüsselungswerkzeug, eine einfachere Alternative zu GPG, um eine Datei für einen oder mehrere Empfänger zu versiegeln.",
  "export.encrypt.enable": "Exporte mit age verschlüsseln",
  "export.encrypt.enableHint": "Wenn aktiv, werden Container-, VM- und Flash-Exporte vor dem Schreiben mit age versiegelt und erhalten die Endung .age.",
  "export.encrypt.recipients": "age-Empfänger",
  "export.encrypt.recipientsHint": "Ein Empfänger pro Zeile. Nutze einen age-Public-Key (age1...) oder einen SSH-Public-Key. Zum Entschlüsseln außerhalb des Servers wird der passende private Schlüssel benötigt. Ist die Verschlüsselung aktiv und kein gültiger Empfänger vorhanden, schlägt der Export fehl, statt Klartext zu schreiben.",
  "export.encrypt.recipientsPlaceholder": "age1qz...\nssh-ed25519 AAAA...",
  "export.encrypt.recipientsRequired": "Füge mindestens einen age-Empfänger hinzu, sonst schlägt der verschlüsselte Export fehl.",

  // Config-Selbst-Backup (BombVaults eigene Einstellungen). Minimaler en/de-Satz
  // für Task 12; die vollständige 24-Sprachen-Übersetzung folgt in Task 14.
  "config.title": "Selbst-Backup",
  "config.subtitle": "Sichert BombVaults eigene Einstellungen, damit sich ein neu aufgesetzter Server selbst wiederherstellen kann.",
  "config.settingsTitle": "Selbst-Backup-Einstellungen",
  "config.settingsHint": "Schützt BombVaults eigene Konfiguration (die Einstellungsdatenbank, Offsite-Zugangsdaten und SSH-Schlüssel), damit eine frische Installation sich selbst wiederherstellt und genau dort weitermacht, wo sie aufgehört hat.",
  "config.enabled": "BombVaults Einstellungen sichern",
  "config.enabledHint": "BombVaults eigenes /config in den unten stehenden Zeitplan aufnehmen.",
  "config.path": "Backup-Ort",
  "config.pathHint": "Relativer Unterpfad unter dem Host-Mount-Root, in den das Config-Repo geschrieben wird.",
  "config.schedule": "Zeitplan",
  "config.scheduleHint": "Sichert BombVaults eigene Einstellungen, Ziele und Zugangsdaten zur geplanten Zeit.",
  "config.offsite": "Offsite-Repo (optional)",
  "config.offsiteHint": "Das Config-Backup nach jedem lokalen Backup in ein zweites, ausgelagertes Repo replizieren.",
  "config.offsiteSchedule": "Offsite-Zeitplan",
  "config.immutable": "Offsite-Repo ist append-only (unveränderlich)",
  "config.immutableHint": "Offsite-Pruning überspringen und Offsite-Löschungen verweigern. Die Gegenseite (append-only) erzwingt es.",
  "config.backupTitle": "Einstellungen jetzt sichern",
  "config.backupHint": "Erfasst BombVaults eigenes /config: die Einstellungsdatenbank, Offsite-Zugangsdaten (rclone.conf) und das SSH-Schlüsselpaar.",
  "config.backupNow": "Einstellungen jetzt sichern",
  "config.backingUp": "Sichere…",
  "config.snapshotsTitle": "Einstellungs-Backups",
  "config.snapshotsHint": "Um diese Einstellungen auf einem neu aufgesetzten Server wiederherzustellen, den Wiederherstellungs-Tab verwenden: das Wiederherstellen der Einstellungen startet BombVault neu, damit sie angewendet werden, daher liegt es dort beim übrigen Notfall-Ablauf.",
  "config.none": "Noch keine Einstellungs-Backups. Oben eines starten.",

  // Empfänger-Dashboard (nur lesende Überwachung eines Append-only-Off-site-Repos,
  // in das ein anderes BombVault schiebt)
  "receiver.title": "Empfänger",
  "receiver.subtitle": "Überwache die Off-site-Kopien, die andere BombVault-Instanzen auf diese Box schieben, rein lesend.",
  "receiver.addRepo": "Empfangenes Repo hinzufügen",
  "receiver.emptyTitle": "Empfangene Repos",
  "receiver.empty": "Noch keine empfangenen Repositories. Füge das Repo hinzu, in das ein anderes BombVault seine Off-site-Kopien schiebt, und BombVault überwacht es nur lesend: was angekommen ist, wann das letzte Backup eintraf und eine unabhängige Integritätsprüfung auf dieser Hardware.",
  "receiver.loadError": "Empfangene Repositories konnten nicht geladen werden.",
  "receiver.reachable": "Erreichbar",
  "receiver.unreachable": "Nicht erreichbar",
  "receiver.monitoringOff": "Überwachung aus",
  "receiver.lastReceived": "Zuletzt empfangen",
  "receiver.never": "Nie",
  "receiver.snapshotsCount": "{n} Backups",
  "receiver.checkOk": "Prüfung OK",
  "receiver.checkFailed": "Prüfung fehlgeschlagen",
  "receiver.checkNever": "Noch nicht geprüft",
  "receiver.lastChecked": "Zuletzt geprüft {time}",
  "receiver.checkNow": "Jetzt prüfen",
  "receiver.deepCheck": "Tiefenprüfung (Daten lesen)",
  "receiver.details": "Details",
  "receiver.edit": "Bearbeiten",
  "receiver.remove": "Entfernen",
  "receiver.removing": "Wird entfernt…",
  "receiver.confirmRemove": "Entfernen bestätigen",
  "receiver.inventoryTitle": "Bestand nach Quelle",
  "receiver.inventoryLoading": "Bestand wird geladen…",
  "receiver.inventoryError": "Der Bestand konnte nicht geladen werden.",
  "receiver.inventoryEmpty": "Noch keine Backups empfangen.",
  "receiver.colSource": "Quelle",
  "receiver.colSnapshots": "Backups",
  "receiver.colLastReceived": "Zuletzt empfangen",
  "receiver.colSize": "Größe",
  "receiver.total": "Gesamt",
  "receiver.addTitle": "Empfangenes Repo hinzufügen",
  "receiver.editTitle": "Empfangenes Repo bearbeiten",
  "receiver.name": "Name",
  "receiver.repoLocation": "Repository-Speicherort",
  "receiver.repoLocationHint": "Der restic-Speicherort dieses Repos: rest:http://host:8000/repo, s3:…, rclone:remote:pfad oder ein Unterpfad unter dem Host-Mount.",
  "receiver.appKey": "Sendender APP_KEY",
  "receiver.appKeyHint": "Der APP_KEY (64 Hex-Zeichen) des BombVault, das in dieses Repo schiebt, damit es nur lesend geöffnet werden kann. Verschlüsselt gespeichert und nie wieder angezeigt.",
  "receiver.appKeyKeep": "gespeichert (leer lassen zum Beibehalten)",
  "receiver.appKeyInvalid": "Der APP_KEY muss aus 64 hexadezimalen Kleinbuchstaben bestehen.",
  "receiver.deadManHours": "Totmannschalter (Stunden)",
  "receiver.deadManHoursHint": "Warnen, wenn von einer Quelle innerhalb dieser Stundenzahl kein Backup empfangen wurde.",
  "receiver.checkCadence": "Prüf-Takt",
  "receiver.checkCadenceHint": "Wann die unabhängige Integritätsprüfung auf dieser Box läuft. Leer lassen für täglich, 'off' zum Deaktivieren, oder z. B. 'weekly Sun 05:00'.",
  "receiver.checkCadencePlaceholder": "leer = daily 04:00 · off · daily HH:MM · weekly Sun 05:00",
  "receiver.readDataPercent": "Tiefenprüfung-Stichprobe (%)",
  "receiver.readDataPercentHint": "Wie viel Pack-Daten restic bei der geplanten Prüfung erneut liest. 0 = nur Strukturprüfung.",
  "receiver.enabledLabel": "Dieses Repository überwachen",
  "receiver.nameRequired": "Namen eingeben.",
  "receiver.repoRequired": "Repository-Speicherort eingeben.",
  "receiver.saveError": "Das empfangene Repo konnte nicht gespeichert werden.",
  "fleet.title": "Flotte",
  "fleet.subtitle": "Den Schutzstatus verbundener BombVault-Instanzen einsehen, rein lesend.",
  "fleet.addPeer": "Instanz hinzufügen",
  "fleet.emptyTitle": "Verbundene Instanzen",
  "fleet.empty": "Noch keine verbundenen Instanzen. Füge die URL und den Fleet-Token einer anderen BombVault-Instanz hinzu, und diese Box fragt nur lesend ihre Schutz-Scorecard ab, nicht mehr.",
  "fleet.loadError": "Verbundene Instanzen konnten nicht geladen werden.",
  "fleet.monitoringOff": "Überwachung aus",
  "fleet.pollNever": "Noch nie abgefragt",
  "fleet.pollOk": "Abfrage OK",
  "fleet.pollFailed": "Abfrage fehlgeschlagen",
  "fleet.lastPolled": "Zuletzt abgefragt {time}",
  "fleet.pollNow": "Jetzt abfragen",
  "fleet.polling": "Frage ab…",
  "fleet.details": "Details",
  "fleet.edit": "Bearbeiten",
  "fleet.remove": "Entfernen",
  "fleet.removing": "Wird entfernt…",
  "fleet.confirmRemove": "Entfernen bestätigen",
  "fleet.scorecardTitle": "Schutz-Scorecard",
  "fleet.noScorecard": "Noch keine gecachte Scorecard. Diese Instanz abfragen, um eine zu holen.",
  "fleet.lastBackup": "letztes Backup {time}",
  "fleet.protection.green": "Geschützt",
  "fleet.protection.amber": "Eingeschränkt",
  "fleet.protection.red": "Gefährdet",
  "fleet.protection.none": "Unbekannt",
  "fleet.addTitle": "Instanz hinzufügen",
  "fleet.editTitle": "Instanz bearbeiten",
  "fleet.name": "Name",
  "fleet.url": "Instanz-URL",
  "fleet.urlHint": "Die Basis-URL der Instanz, z. B. https://192.168.1.50:3443, zu finden auf deren eigener Settings-Seite.",
  "fleet.token": "Fleet-Token der Instanz",
  "fleet.tokenKeep": "gespeichert (leer lassen zum Beibehalten)",
  "fleet.tokenHint": "Auf der EIGENEN Settings → System-Seite der anderen Instanz erzeugt, dann hier eingefügt, damit diese Instanz sie abfragen kann.",
  "fleet.enabledLabel": "Diese Instanz abfragen",
  "fleet.nameRequired": "Namen eingeben.",
  "fleet.urlRequired": "Instanz-URL eingeben.",
  "fleet.saveError": "Die verbundene Instanz konnte nicht gespeichert werden.",
  "fleet.mesh.offersTitle": "Off-site-Speicher-Angebote",
  "fleet.mesh.offersHint": "Eine Instanz hat ihren eigenen Off-site-Speicher angeboten. Prüfen und annehmen, um daraus ein normales Off-site-Ziel zu machen. Nichts wird automatisch übernommen.",
  "fleet.mesh.saveError": "Konnte nicht gespeichert werden.",
  "fleet.mesh.unknownPeer": "Unbekannte Instanz",
  "fleet.mesh.applyTo": "Anwenden auf:",
  "fleet.mesh.accept": "Annehmen",
  "fleet.mesh.decline": "Ablehnen",
  "fleet.mesh.status.pending": "Ausstehend",
  "fleet.mesh.status.accepted": "Angenommen",
  "fleet.mesh.status.declined": "Abgelehnt",
  "fleet.mesh.proposeButton": "Speicher anbieten",
  "fleet.mesh.proposeTitle": "Off-site-Speicher anbieten",
  "fleet.mesh.proposeHint": "Sende die Verbindungsdaten des eigenen Off-site-Speichers an {peer}, damit dessen Verwalter nicht außerhalb von BombVault über URL und Passwort informiert werden muss.",
  "fleet.mesh.domain": "Für welche Domäne ist dieser Speicher gedacht?",
  "fleet.mesh.baseUrl": "Wo wird der rest-server bereitgestellt?",
  "fleet.mesh.baseUrlHint": "Die echte Adresse, unter der der rest-server erreichbar sein wird, z. B. http://192.168.1.50:8000. BombVault kann das nicht erraten.",
  "fleet.mesh.baseUrlRequired": "Basis-URL eingeben, unter der der rest-server bereitgestellt wird.",
  "fleet.mesh.sending": "Wird gesendet…",
  "fleet.mesh.send": "Angebot senden",
  "fleet.mesh.sent": "Angebot an {peer} gesendet.",
  "fleet.mesh.deployNow": "Jetzt den rest-server mit diesem Rezept bereitstellen:",
  "fleet.mesh.dockerRun": "docker run",
  "fleet.mesh.compose": "docker-compose.yml",

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
  "vm.ssh.publicKey": "Public Key: an Unraids /root/.ssh/authorized_keys anhängen",
  "vm.ssh.copy": "Kopieren",
  "vm.ssh.copied": "Kopiert",
  "vm.ssh.copyFailed": "Kopieren fehlgeschlagen",
  "vm.ssh.test": "Verbindung testen",
  "vm.ssh.testing": "Teste…",
  "vm.ssh.testOk": "Verbunden, libvirt erreichbar",
  "vm.ssh.testFail": "Verbindung fehlgeschlagen",
  "vm.ssh.setupTitle": "Einrichten (einmalig)",
  "vm.ssh.step1": "Den Befehl unten kopieren und im Unraid-Terminal ausführen, um diesen Schlüssel zu autorisieren (überlebt Reboots).",
  "vm.ssh.step2": "Die Container-Variable “VM Backup: Host” auf die LAN-IP deines Unraid-Servers setzen (z. B. 192.168.x.x); bei einfachem Bridge-Netz geht auch host.docker.internal.",
  "vm.ssh.step3": "Auf “Verbindung testen” klicken. Sobald grün, VMs unter Domänen aktivieren.",
  "vm.ssh.copyCmd": "Befehl kopieren",
  "vm.ssh.guide": "Vollständige Setup- & Netzwerk-Anleitung",

  // Guided Recovery tab (disaster-recovery walkthrough)
  "nav.recovery": "Wiederherstellung",
  "recovery.pageTitle": "Notfall-Wiederherstellung",
  "recovery.intro": "Stelle deine Container und VMs aus einem vorhandenen Backup auf dieser Installation wieder her. Richte BombVault auf deine Backups aus, finde heraus, was darin steckt, und stelle es wieder her.",
  // Schritt 1 — Verbindungs-/APP_KEY-Lesbarkeitsprüfung
  "recovery.step1": "Kann BombVault deine Backups lesen?",
  "recovery.appKeyExplain": "Um vorhandene Backups zu lesen, braucht dieser Container denselben APP_KEY wie zuvor. Er steht in deinem Recovery-Kit. Setze ihn im Unraid-Container-Template, falls noch nicht geschehen, und prüfe erneut.",
  "recovery.appKeyRemedy": "Der Verschlüsselungsschlüssel passt nicht zu diesen Backups. Trage den ursprünglichen APP_KEY (aus deinem Recovery-Kit) im Container-Template ein und prüfe erneut.",
  "recovery.readable": "Deine Backups sind lesbar.",
  "recovery.notReachable": "Deine Backups waren noch nicht erreichbar. Hänge den Speicherort unten an und prüfe erneut.",
  "recovery.recheck": "Prüfen",
  // Schritt 2 — zuerst BombVaults eigene Einstellungen wiederherstellen (optional)
  "recovery.stepConfig": "BombVaults eigene Einstellungen wiederherstellen",
  "recovery.configHint": "Stelle auf einem neu aufgesetzten Server zuerst BombVaults eigene Einstellungen wieder her (Backup-Pfade, Off-site-Ziele und Zugangsdaten), damit die Schritte unten schon vorausgefüllt sind. Richte es auf das zuvor eingerichtete Einstellungs-Backup aus. Kein Einstellungs-Backup? Überspringe dies und hänge deine Backups unten manuell an.",
  "recovery.configAppKeyReminder": "Dein APP_KEY muss zu diesem Backup passen. Das ist die Prüfung in Schritt 1 oben.",
  "recovery.configSourceLabel": "Wo liegt das Einstellungs-Backup?",
  "recovery.configLocalPath": "Lokaler Pfad",
  "recovery.configOffsiteUrl": "Off-site-Repo-URL",
  "recovery.configRestore": "Wiederherstellen",
  "recovery.configRestoring": "Stelle wieder her…",
  "recovery.configRestarting": "BombVault startet neu, um deine Einstellungen anzuwenden… diese Seite lädt automatisch neu, sobald es wieder da ist.",
  "recovery.configManualRestart": "Deine Einstellungen sind bereitgestellt. Starte den BombVault-Container in Unraid neu und fahre dann fort. Sie werden beim nächsten Start angewendet.",
  "recovery.configReloadWhenBack": "BombVault braucht länger als erwartet, um zurückzukommen. Lade diese Seite neu, sobald es wieder läuft, um deine wiederhergestellten Einstellungen zu laden.",
  "recovery.configReload": "Jetzt neu laden",
  "recovery.configSkip": "Überspringen: ich habe kein Einstellungs-Backup",
  "recovery.configSkipped": "Übersprungen. Hänge deine Backups unten manuell an.",
  // Schritt 3 — Backups anhängen
  "recovery.step2": "Backups anhängen",
  "recovery.cloudCreds": "Cloud-Zugangsdaten (optional)",
  "recovery.cloudCredsHint": "Nur nötig, wenn ein Backup-Pfad auf S3, einen restic-REST-Server oder ein rclone-Remote zeigt. Für einen lokalen Pfad oder eine eingebundene Freigabe brauchst du hier nichts.",
  "recovery.attachHint": "Richte BombVault auf deine vorhandenen Backups aus: einen lokalen Pfad unter dem Host-Mount oder ein Off-site-Repo (rest / S3 / B2 / sftp / rclone) mit den zugehörigen Zugangsdaten. Verbinde dich dann, um es zu bestätigen.",
  "recovery.credsSaveHint": "Off-site-Zugangsdaten werden über den eigenen Speichern-Button der jeweiligen Karte gespeichert. Speichere sie, bevor du „Verbinden & prüfen“ klickst.",
  "recovery.connectPreview": "Verbinden & prüfen",
  // Verschlüsselungsmodus — ERKANNT, nicht behauptet. Die Repositories sagen
  // selbst, ob sie das aus dem APP_KEY abgeleitete Passwort brauchen. Der
  // Normalfall (vorhandenes Repo anhängen) fragt dich also nichts mehr; nur die
  // wirklich unentscheidbaren Fälle zeigen den Schalter noch.
  "recovery.encChecking": "Prüfe, wie deine Backups verschlüsselt sind …",
  "recovery.encEncrypted": "Erkannt: Deine Backups sind verschlüsselt. BombVault nutzt das aus deinem APP_KEY abgeleitete Passwort.",
  "recovery.encPlain": "Erkannt: Deine Backups haben kein Passwort.",
  "recovery.encAbsent": "An diesen Orten liegt noch kein Backup-Repository, es gibt also nichts zu erkennen. Deine Wahl unten entscheidet, wie es angelegt wird.",
  "recovery.encUnknown": "Noch nicht feststellbar: Die Repositories liessen sich nicht öffnen, ihr Verschlüsselungsmodus ist damit unbekannt. Korrigiere den Ort oben und prüfe erneut, oder stelle ihn selbst ein, wenn du ihn kennst.",
  "recovery.encConflict": "Deine Repositories widersprechen sich: Einige sind verschlüsselt, andere nicht. Eine Einstellung kann nicht beide öffnen. Richte den Ausreisser auf einen neuen, leeren Ort aus oder stelle nur aus dem passenden Satz wieder her.",
  "recovery.encUnconfigured": "Noch kein Backup-Ort eingetragen. Trage unten die Pfade ein und verbinde dich dann.",
  "recovery.encDetectHint":
    "Verschlüsselung ist keine Vorliebe: Ein Repository wird entweder mit Passwort (aus deinem APP_KEY abgeleitet) oder ohne angelegt, und das ändert sich danach nie mehr. BombVault öffnet die eingetragenen Repositories, um zu sehen, was davon zutrifft. Auf einer frischen Kiste musst du also nicht raten. Ein Repository, das nicht erreichbar ist, gilt als unbekannt, niemals als unverschlüsselt.",
  "recovery.encStateEncrypted": "verschlüsselt",
  "recovery.encStatePlain": "kein Passwort",
  "recovery.encStateAbsent": "noch nicht angelegt",
  "recovery.encStateUnreachable": "nicht lesbar",
  "recovery.encSourceLocal": "lokal",
  "recovery.encSourceOffsite": "off-site",
  // Schritt 3 — alles entdecken
  "recovery.step3": "Entdecke, was in deinen Backups steckt",
  "recovery.discover": "Backups entdecken",
  "recovery.foundCounts": "{c} Container und {v} VMs gefunden.",
  "recovery.foundNone": "Noch nichts gefunden. Prüfe Verbindung und Anhang oben. Falls du hier Backups erwartest, stelle sicher, dass dein APP_KEY zu diesen Backups passt.",
  // Schritt 4 — prüfen & alle wiederherstellen (gestoppt lassen)
  "recovery.step4": "Prüfen und wiederherstellen",
  "recovery.restoreAll": "Alle wiederherstellen (gestoppt lassen)",
  "recovery.restoreAllResult": "{ok} wiederhergestellt, {fail} fehlgeschlagen. Starte sie bei Bedarf über die Tabs Container/VMs.",
  "recovery.vmSshNote": "Für die VM-Wiederherstellung wird die libvirt-SSH-Verbindung benötigt. Richte sie unter Einstellungen → VM-Backup über SSH ein.",
  "recovery.noneDiscovered": "Führe zuerst oben „Entdecken“ aus.",
  // Schritt 5 — Recovery-Kit (Sicherheitsnetz fürs nächste Mal)
  "recovery.step5": "Dein Recovery-Kit",
  "recovery.kitHint": "Lade dein Recovery-Kit herunter und bewahre es sicher auf. Es enthält den Verschlüsselungsschlüssel und die genauen restic-Befehle, um selbst ohne BombVault wiederherzustellen.",
  "recovery.kitDownload": "Recovery-Kit herunterladen",
  // Dashboard-Hinweis bei frischer Installation → geführter Wiederherstellungs-Tab
  "recovery.freshNudge": "Wiederherstellung von einem früheren Server oder nach einem Neuaufbau? Stelle deine vorhandenen Backups wieder her.",
  "recovery.freshNudgeCta": "Zur Wiederherstellung",

  // Einstellungen — Bereichs-Tabs + Zeitplan-Überschriften + Untertitel (v5-Redesign)
  "settings.tab.general": "Allgemein",
  "settings.tab.storage": "Pfade & Speicher",
  "settings.tab.schedules": "Zeitpläne",
  "settings.tab.offsite": "Off-site",
  "settings.tab.notifications": "Benachrichtigungen",
  "settings.tab.integrity": "Integrität",
  "settings.tab.system": "System",
  "settings.schedulesOptions": "Zeitplan-Optionen",
  "settings.schedulesOffsite": "Off-site-Replikations-Zeitpläne",
  "settings.schedulesSelfBackup": "Selbst-Backup-Zeitplan",
  "settings.schedulesChecks": "Wiederherstellungs-Prüfplan",
  "settings.tamperTestSchedule": "Tamper-Test-Zeitplan",
  "settings.tamperScheduleInactive": "Inaktiv: kein Off-site-Repo ist als append-only markiert, daher läuft dieser Zeitplan nie. Markiere ein Off-site-Repo in dessen Off-site-Einstellungen als append-only, um ihn zu aktivieren.",
  // Gesamt-Backup (ein 6., unabhängiger Durchlauf über alle fünf Bereiche oben)
  "settings.everythingTitle": "Gesamt-Backup",
  "settings.everythingHint": "Sichert einmal nacheinander jeden Bereich (Container, VMs, Flash, Ordner und zuletzt das Selbst-Backup), sodass ein einzelner Ping danach (über den Post-Befehl unten) bestätigt, dass der ganze Server geschützt ist. Standardmäßig aus und unabhängig vom eigenen Zeitplan jedes Bereichs oben.",
  "settings.everythingHooksHint": "Diese Befehle laufen auf dem eigenen Server von BombVault über sh -c, nicht in einem Container. Der Pre-Befehl ist best-effort und blockiert den Durchlauf nie; der Post-Befehl feuert immer genau einmal, nachdem jeder Bereich versucht wurde, egal ob einer davon fehlgeschlagen ist.",
  "settings.everythingOverlapWarning": "Dieser Zeitplan läuft unabhängig von den Zeitplänen der einzelnen Bereiche oben. Sind beide aktiv, werden Container, VMs, Flash, Ordner und Selbst-Backup jeweils doppelt gesichert: einmal hier, einmal im eigenen Zeitplan. Deaktiviere die Bereiche, die nicht doppelt laufen sollen.",
  "settings.everythingRunNow": "Gesamt-Backup jetzt starten",
  "settings.everythingStarted": "Gestartet. Es läuft auf dem Server nacheinander über jeden Bereich; das Aktivitätsprotokoll zeigt das Ergebnis.",
  "settings.everythingAlreadyRunning": "Es läuft bereits ein Gesamt-Backup.",
  "settings.everythingBusy": "Arbeite…",
  "settings.subtitle": "BombVault-Konfiguration. Änderungen wirken sofort.",
  // Filter-Auslöser (v5-Redesign)
  "filter.button": "Filter",

  // Einstellungen — Wochenbericht-Karte, Backup-Engine-Cache, Alle-Sitzungen-Abmeldung
  "settings.digestTitle": "Wochenbericht",
  "settings.digestHint": "Eine Zusammenfassung pro Woche: Anzahl der Läufe, neue Backup-Daten, Off-site-Aktualität und die wichtigsten Fehler, gesendet über die oben konfigurierten Benachrichtigungskanäle.",
  "settings.digestToggle": "Wochenbericht",
  // Einstellungen — Nachholen verpasster Zeitpläne (Zeitpläne-Tab) + Wächter für überfällige Backups (Benachrichtigungen-Tab)
  "settings.missedSchedulesTitle": "Verpasste Zeitpläne",
  "settings.catchUpMissed": "Verpasste Backups nach dem Start nachholen",
  "settings.catchUpMissedHint": "War der Server aus, als ein Zeitplan fällig war, wird dieses Backup etwa zwei Minuten nach dem Start von BombVault nachgeholt.",
  "settings.watchdogTitle": "Wächter für überfällige Backups",
  "settings.watchdogHint": "Prüft einmal täglich, ob ein aktiviertes Backup überfällig ist (älter als das Doppelte seines Zeitplans), und sendet pro Vorfall eine Benachrichtigung über die oben konfigurierten Kanäle.",
  "settings.watchdogToggle": "Bei überfälligen Backups benachrichtigen",
  "settings.cacheTitle": "Backup-Engine-Cache",
  "settings.cacheHint": "Die Backup-Engine hält unter /config einen Cache mit Repository-Daten, damit inkrementelle und Off-site-Läufe schnell bleiben. Wächst er über dieses Limit, werden nach geplanten Läufen die am längsten ungenutzten Repository-Caches entfernt.",
  "settings.cacheLimitLabel": "Cache-Größenlimit (MB, 0 = unbegrenzt)",
  "settings.logoutAll": "Überall abmelden",

  // Neu-Dialog (#48) — einmalig bei einer neuen laufenden Version
  "whatsnew.title": "Neu in {version}",
  "whatsnew.loading": "Lade Release-Notes…",
  "whatsnew.loadFailed": "Release-Notes konnten hier nicht geladen werden. Auf GitHub öffnen.",
  "whatsnew.retry": "Erneut versuchen",
  "whatsnew.viewOnGitHub": "Vollständige Release-Notes auf GitHub",
  "whatsnew.close": "Schließen",

  // Ordner-Domäne (Ordner-Set-Backups, #62) — der Ordner-Tab (Keys behalten
  // die historischen files.*-Namen; nur die angezeigten Werte heißen "Ordner")
  "nav.files": "Ordner",
  "files.title": "Ordner",
  "files.subtitle": "Beliebige Ordner dieses Servers sichern, mit Zeitplänen, Off-site-Kopien und Wiederherstellungen.",
  "files.setsTitle": "Ordner-Sets",
  "files.empty": "Noch keine Ordner-Sets. Füge einen Ordner hinzu (Shares, Dokumente, Fotos, alles unter deinen Mounts) und BombVault schützt ihn wie alles andere: Zeitpläne, Off-site-Kopien, Integritätsprüfungen und Wiederherstellungen. Kein separates Datei-Backup-Tool nötig.",
  "files.addSet": "Ordner-Set hinzufügen",
  "files.editSet": "Ordner-Set bearbeiten",
  "files.name": "Name",
  "files.path": "Ordner",
  "files.pathHint": "Der zu sichernde Ordner, ein relativer Unterpfad unter dem Host-Mount-Root.",
  "files.excludes": "Ausschlussmuster",
  "files.excludesHint": "Ein Muster pro Zeile, wird als --exclude an restic übergeben (z. B. *.tmp, cache/).",
  "files.excludesCount": "Ausschlüsse: {n}",
  "files.enabled": "Im Zeitplan einschließen",
  "files.pathMissing": "Ordner nicht gefunden",
  "files.noPath": "Kein Ordner gesetzt",
  "files.noPathHint": "Aus Backups ohne Ordner wiederaufgebaut. Setze einen Ordner, um wieder zu sichern. Die Wiederherstellung in einen Ordner funktioniert schon jetzt.",
  "files.deleteSet": "Set entfernen",
  "files.deleteSetConfirm": "Dieses Ordner-Set aus der Liste entfernen? Seine Backups werden nicht gelöscht und können später wiederentdeckt werden.",
  "files.deleteBackupsConfirm": "ALLE Backups dieses Ordner-Sets löschen? Die Snapshots werden dauerhaft entfernt, das Repository wird aufgeräumt und das Set wird vergessen. Kann nicht rückgängig gemacht werden.",
  "files.restoreOriginal": "An den Originalort wiederherstellen",
  "files.restoreOriginalConfirm": "Dieses Backup über den Ordner des Sets wiederherstellen? Vorhandene Dateien werden überschrieben.",
  "files.restoreToFolder": "In einen Ordner wiederherstellen",
  "files.restoreSelectFiles": "Dateien auswählen",
  "files.restoreComplete": "Wiederherstellung abgeschlossen: die Dateien wurden geschrieben.",
  "files.backupAll": "Alle jetzt sichern",
  "files.discoverHint": "Set-Liste verloren? Baue sie aus den Backups im Speicher neu auf.",
  "files.cancel": "Abbrechen",
  "files.addPreset": "Preset hinzufügen: Systemkonfiguration",
  "files.addPresetHint": "Ein vorsichtiger Ausgangspunkt für die Konfiguration auf Host-Ebene außerhalb deiner Container, kein Anspruch auf Vollständigkeit. Ordner vor dem Speichern prüfen.",
  // Files domain integration — Settings, Dashboard, Recovery (#62 task 7)
  "settings.filesEnabled": "Ordner",
  "settings.filesEnabledHint": "Beliebige Ordner unter deinen Mounts als Datei-Sets sichern, unabhängig von den anderen Domänen.",
  "settings.filesPath": "Ordner-Pfad",
  "jobs.filesSection": "Ordner",
  "jobs.filesIncludeHint": "Sichert jedes Ordner-Set mit aktiviertem „Im Zeitplan einschließen“, pro Set unten oder im Ordner-Tab umschaltbar.",
  "jobs.noFileSetsIncluded": "Noch keine Ordner-Sets. Füge sie im Ordner-Tab hinzu.",
  "dashboard.domainFiles": "Ordner",
  "recovery.filesFound": "{f} Ordner-Sets gefunden.",
  "recovery.filesRestoreHint": "Wiederentdeckte Ordner-Sets kennen ihren Ursprungsordner nicht. Jedes wird in einen Ordner deiner Wahl wiederhergestellt.",
  // Restore from another BombVault repo — Recovery page (#61 task 11)
  "recovery.foreignTitle": "Aus einem anderen BombVault-Repo wiederherstellen",
  "recovery.foreignIntro": "Hole einzelne Container, VMs oder Ordner-Sets aus den Backups einer ANDEREN BombVault-Instanz: schreibgeschützt verbinden, Inhalt durchstöbern, Auswahl wiederherstellen. Das andere Repository wird dabei nur gelesen: dort ändert sich nichts, und deine eigenen Backup-Einstellungen bleiben unangetastet.",
  "recovery.foreignStepConnect": "Mit dem anderen Repository verbinden",
  "recovery.foreignStepBrowse": "Durchstöbern & wiederherstellen",
  "recovery.foreignLocation": "Repository-Speicherort",
  "recovery.foreignLocationHint": "Ein Ordner unter dem Host-Mount. Hänge die Backup-Freigabe des anderen Servers auf diesem Host ein und richte dies darauf aus. Das Repository muss lokal eingebunden sein (Remote-Repo-URLs werden hier nicht akzeptiert).",
  "recovery.foreignKey": "APP_KEY der anderen Instanz",
  "recovery.foreignKeyHint": "Der 64-stellige Schlüssel aus dem Recovery-Kit der ANDEREN Instanz. Dein eigener Schlüssel bleibt unberührt.",
  "recovery.foreignConnect": "Verbinden",
  "recovery.foreignConnecting": "Verbinde…",
  "recovery.foreignConnected": "Verbunden. Das Repository ist lesbar.",
  "recovery.foreignClose": "Trennen",
  "recovery.foreignNotConnected": "Verbinde dich zuerst oben mit einem Repository.",
  "recovery.foreignEmpty": "Das Repository ist lesbar, enthält aber keine BombVault-Backups.",
  "recovery.foreignLatest": "Neuestes Backup",
  "recovery.foreignTargetFolder": "Zielordner",
  "recovery.foreignWholeSet": "Gesamter Satz",
  "recovery.foreignPickSubfolder": "Unterordner wählen",
  "recovery.foreignSubfolderHint": "Nur die angehakten Unterordner oder Dateien dieses Satzes in den Zielordner wiederherstellen, zum Beispiel einen einzelnen Stack aus einem gesamten Appdata-Backup.",
  "recovery.foreignAppdataDest": "Appdata-Ziel",
  "recovery.foreignAppdataDestHint": "Wohin die Appdata des Containers wiederhergestellt wird. Leer lassen für den Standard. Ein Container, der von einem Pool gesichert wurde, den dieser Server nicht hat (zum Beispiel /mnt/zfs), wird hierher umgemappt und landet so korrekt.",
  "recovery.foreignOverwrite": "Überschreiben, wenn am Ziel bereits Daten liegen",
  "recovery.foreignBindWarning": "Diese Binds zeigen auf Speicher, den dieser Server nicht hat. Appdata wird automatisch umgemappt, aber passe diese nach dem Restore im Container-Template an:",
  "recovery.foreignRestore": "Hierher wiederherstellen",
  "recovery.foreignExistsConfirm": "„{name}“ existiert bereits auf diesem System, daher ÜBERSCHREIBT die Wiederherstellung es mit dem fremden Backup. Fortfahren?",
  "recovery.foreignUnverifiedConfirm": "BombVault konnte die aktuellen Container und VMs dieses Systems nicht auslesen und kann daher nicht feststellen, ob „{name}“ hier bereits existiert. Die Wiederherstellung überschreibt womöglich ein vorhandenes. Fortfahren?",
  "recovery.foreignExpired": "Die Sitzung ist abgelaufen (Sitzungen gelten 30 Minuten). Verbinde dich erneut, um weiterzustöbern.",
  "recovery.foreignReconnect": "Erneut verbinden",
  "recovery.foreignVMDest": "Ziel der VM-Datenträger",
  "recovery.foreignVMDestHint": "Wohin die Datenträger der VM geschrieben werden. Die Abbilder landen in <destination>/<vm-name>/. Wähle daher einen Ordner auf einem echten eingebundenen Pool, nicht auf der RAM-Disk. Eine fremde VM wird gestoppt wiederhergestellt, starte sie also selbst, sobald du sie geprüft hast.",

  // Dashboard activity log (Task 4)
  "activityLog.title": "Aktivitätsprotokoll",
  "activityLog.filterPlaceholder": "Filtern… (z. B. plex, fehlgeschlagen, off-site)",
  "activityLog.filterAllDomains": "Alle Bereiche",
  "activityLog.filterAllTypes": "Alle Arten",
  "activityLog.typeBackup": "Backup",
  "activityLog.typeRestore": "Wiederherstellung",
  "activityLog.typePrune": "Bereinigung",
  "activityLog.typeVerify": "Prüfung",
  "activityLog.typeOffsite": "Off-Site",
  "activityLog.typeExport": "Export",
  "activityLog.jumpToLatest": "Zum Aktuellen springen",
  "activityLog.glyphRunning": "Läuft",
  "activityLog.glyphSuccess": "Erfolgreich",
  "activityLog.glyphFailed": "Fehlgeschlagen",
  "activityLog.glyphOffsite": "Off-Site",
  "activityLog.glyphInfo": "Info",
  "activityLog.domainContainers": "Container",
  "activityLog.domainVMs": "VMs",
  "activityLog.domainFlash": "Flash",
  "activityLog.domainConfig": "Selbst-Backup",
  "activityLog.domainFiles": "Ordner",
  "activityLog.domainEverything": "Gesamt-Backup",
  "activityLog.jobBackup": "Backup",
  "activityLog.jobOffsite": "Off-Site-Replikation",
  "activityLog.jobDrill": "Wiederherstellungs-Prüfung",
  "activityLog.jobTamper": "Manipulationstest",
  "activityLog.jobDigest": "Wochenbericht",
  "activityLog.jobWatchdog": "Backup-Überfälligkeitsprüfung",
  "activityLog.lineBackingUpItem": "Sichere {name} … {percent}%",
  "activityLog.lineRestoringItem": "Stelle {name} wieder her … {percent}%",
  "activityLog.lineBackingUpBatch": "Sichere alle {domain} … {percent}%",
  "activityLog.lineOffsiteRunning": "Off-Site-Upload: {domain} …",
  "activityLog.lineOffsiteRunningWithDuration": "Off-Site-Upload: {domain} … ({duration})",
  "activityLog.lineOffsiteRunningSnapshotPercent": "Off-Site-Upload: {domain} … {percent} % gesamt (Snapshot {index} von {total})",
  "activityLog.lineOffsiteRunningSnapshotPercentWithDuration": "Off-Site-Upload: {domain} … {percent} % gesamt (Snapshot {index} von {total}) · {duration}",
  "activityLog.linePruneRunning": "Räume auf: {domain} …",
  "activityLog.lineVerifyRunning": "Prüfe: {domain} …",
  "activityLog.lineDrillRunning": "Wiederherstellungs-Prüfung läuft: {domain} …",
  "activityLog.lineDRDrillRunning": "Off-Site-DR-Prüfung läuft: {domain} …",
  "activityLog.lineTamperRunning": "Manipulationstest läuft: {domain} …",
  "activityLog.lineExportRunning": "Exportiere Flash-ZIP …",
  "activityLog.lineBackupSuccess": "{name} gesichert: {bytes} in {duration}",
  "activityLog.lineBackupFailed": "{name}-Backup fehlgeschlagen: {error}",
  "activityLog.lineBackupSkipped": "{name}-Backup übersprungen: {error}",
  "activityLog.lineRestoreSuccess": "{name} wiederhergestellt: {duration}",
  "activityLog.lineRestoreFailed": "{name}-Wiederherstellung fehlgeschlagen: {error}",
  "activityLog.lineUpdateSuccess": "{name} aktualisiert: {duration}",
  "activityLog.lineUpdateFailed": "{name}-Update fehlgeschlagen: {error}",
  "activityLog.linePruneSuccess": "Aufbewahrungs-Bereinigung fertig: {domain}",
  "activityLog.linePruneFailed": "Bereinigung fehlgeschlagen ({domain}): {error}",
  "activityLog.lineVerifySuccess": "Prüfung bestanden: {domain}",
  "activityLog.lineVerifyFailed": "Prüfung fehlgeschlagen ({domain}): {error}",
  "activityLog.lineOffsiteSuccess": "Off-Site-Replikation abgeschlossen: {domain} ({duration})",
  "activityLog.lineOffsiteFailed": "Off-Site-Replikation fehlgeschlagen ({domain}): {error}",
  "activityLog.lineDrillSuccess": "Wiederherstellungs-Prüfung bestanden: {domain}",
  "activityLog.lineDrillFailed": "Wiederherstellungs-Prüfung fehlgeschlagen ({domain}): {error}",
  "activityLog.lineDRDrillSuccess": "Off-Site-DR-Wiederherstellung geprüft: {domain}",
  "activityLog.lineDRDrillFailed": "Off-Site-DR-Wiederherstellung FEHLGESCHLAGEN ({domain}): {error}",
  "activityLog.lineTamperSuccess": "Manipulationstest bestanden: {domain} (Löschen verweigert)",
  "activityLog.lineTamperFailed": "Manipulationstest FEHLGESCHLAGEN. {domain} ist nicht append-only: {error}",
  "activityLog.lineTamperSkipped": "Manipulationstest übersprungen ({domain}): {error}",
  "activityLog.lineExportSuccess": "Flash-ZIP-Export abgeschlossen: {bytes} ({duration})",
  "activityLog.lineExportFailed": "Flash-ZIP-Export fehlgeschlagen: {error}",
  "activityLog.lineOther": "{name} {kind}: {status}",
  "activityLog.lineNextWithDomain": "als Nächstes: {job} ({domain}) um {time} (in {countdown})",
  "activityLog.lineNextNoDomain": "als Nächstes: {job} um {time} (in {countdown})",
  "activityLog.lineEmpty": "noch nichts",
  "activityLog.dayFilterChip": "Zeige {date}",
  "activityLog.clearDayFilter": "Tagesfilter entfernen",

  // Einstellungen exportieren / importieren (portable Konfigurationsdatei)
  "settingsIO.title": "Einstellungen exportieren / importieren",
  "settingsIO.desc":
    "Speichere die Konfiguration dieser Instanz in einer Datei oder lade eine zuvor exportierte Datei in diese Instanz. Es werden nur Einstellungen und Off-site-Ziele übertragen. Deine Backups, Snapshots und der Verlauf bleiben unberührt.",
  "settingsIO.exportHeading": "Export",
  "settingsIO.includeCreds": "Zugangsdaten einschließen (Off-site- und Benachrichtigungs-Geheimnisse)",
  "settingsIO.credsWarning":
    "Mit Zugangsdaten ist diese Datei so sensibel wie dein Recovery-Kit: Sie enthält deine Off-site- und Benachrichtigungs-Geheimnisse im Klartext. Bewahre sie sicher auf.",
  "settingsIO.exportButton": "Einstellungen exportieren",
  "settingsIO.exporting": "Exportiere…",
  "settingsIO.importHeading": "Import",
  "settingsIO.importHint":
    "Lade eine aus BombVault exportierte Einstellungsdatei. Du siehst eine Zusammenfassung und eine Bestätigung, bevor etwas geändert wird.",
  "settingsIO.chooseFile": "Einstellungsdatei wählen",
  "settingsIO.reading": "Lese Datei…",
  "settingsIO.previewTitle": "Diese Datei enthält",
  "settingsIO.previewExportedAt": "Exportiert",
  "settingsIO.previewAppVersion": "Aus BombVault-Version",
  "settingsIO.previewOffsiteTargets": "Off-site-Ziele",
  "settingsIO.previewCredentials": "Zugangsdaten",
  "settingsIO.previewCredsIncluded": "enthalten",
  "settingsIO.previewCredsNotIncluded": "nicht enthalten",
  "settingsIO.previewSettingsAreas": "Einstellungsbereiche",
  "settingsIO.previewNone": "keine",
  "settingsIO.confirmWarning":
    "Dies ersetzt deine aktuellen BombVault-Einstellungen. Deine Backup-Daten und der Verlauf bleiben unberührt.",
  "settingsIO.confirmButton": "Einstellungen ersetzen",
  "settingsIO.importing": "Importiere…",
  "settingsIO.cancel": "Abbrechen",
  "settingsIO.importSuccess": "Einstellungen importiert.",
  "settingsIO.importFailed": "Import fehlgeschlagen",
  "settingsIO.group.domains": "Backup-Quellen",
  "settingsIO.group.schedules": "Zeitpläne",
  "settingsIO.group.retention": "Aufbewahrung",
  "settingsIO.group.offsite": "Off-site",
  "settingsIO.group.drills": "Wiederherstellungs-Übungen",
  "settingsIO.group.digest": "Zusammenfassung",
  "settingsIO.group.everything": "Gesamt-Backup",
  "settingsIO.group.monitoring": "Überwachung",
  "settingsIO.group.language": "Sprache",
  "settingsIO.group.exportEncryption": "Export-Verschlüsselung",

  // Backup order (#119)
  "backupOrder.title": "Backup-Reihenfolge",
  "backupOrder.hint": "Lege die Reihenfolge fest, in der geplante und Sammel-Backups laufen. Nicht gelistete Container laufen danach, am längsten überfällige zuerst.",
  "backupOrder.moveUp": "Nach oben",
  "backupOrder.moveDown": "Nach unten",
  "backupOrder.save": "Reihenfolge speichern",
  "backupOrder.saved": "Reihenfolge gespeichert",
  "backupOrder.saveError": "Backup-Reihenfolge konnte nicht gespeichert werden.",
  "backupOrder.empty": "Noch keine geplanten Container zum Sortieren.",
  "vmBackupOrder.title": "VM-Backup-Reihenfolge",
  "vmBackupOrder.hint": "Lege die Reihenfolge fest, in der der geplante VM-Lauf die VMs sichert. Nicht gelistete VMs laufen danach, in Namensreihenfolge.",
  "vmBackupOrder.empty": "Noch keine geplanten VMs zum Sortieren.",
  "backupOrder.reset": "Reihenfolge löschen",

  // Health-gated restart (#119)
  "settings.restartHealthTitle": "Neustart nach Backup",
  "settings.restartHealthWait": "Vor dem Start des nächsten warten, bis Abhängigkeiten gesund sind",
  "settings.restartHealthWaitHint": "Wenn 'Andere Container während des Backups stoppen' Container stoppt, werden sie nach dem Backup (und nach einem etwaigen Update) in Abhängigkeitsreihenfolge neu gestartet. Ist dies aktiv, muss jeder Container als gesund oder laufend melden, bevor die von ihm abhängigen starten.",
  "settings.restartHealthTimeoutLabel": "Gesundheits-Timeout pro Container (Sekunden)",
  "settings.restartHealthTimeoutHint": "Wie lange auf einen Container gewartet wird, bis er gesund ist, bevor seine Abhängigen trotzdem starten. Bereich 5 bis 3600.",

  // Reconcile Unraid update status (#116)
  "settings.reconcileUnraidStatus": "Unraids Update-Status nach dem Aktualisieren eines Containers auffrischen",
  "settings.reconcileUnraidStatusHint": "Unraids Update-Banner zurücksetzen, nachdem BombVault im Update-Schritt nach dem Backup einen Container aktualisiert hat.",
};

// ---------------------------------------------------------------------------
// Locale registry — 42 languages, all fully translated (en is the source of
// truth; every other table is checked against it by i18n.parity.test.ts).
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
  { code: "bg", label: "Български",    flag: "bg" },
  { code: "sk", label: "Slovenčina",   flag: "sk" },
  { code: "sl", label: "Slovenščina",  flag: "si" },
  { code: "hr", label: "Hrvatski",     flag: "hr" },
  { code: "sr", label: "Српски",       flag: "rs" },
  { code: "lt", label: "Lietuvių",     flag: "lt" },
  { code: "lv", label: "Latviešu",     flag: "lv" },
  { code: "et", label: "Eesti",        flag: "ee" },
  { code: "is", label: "Íslenska",     flag: "is" },
  // The three languages of Spain get their own regional flags rather than three
  // identical Spanish ones, which would make the menu unreadable at a glance.
  { code: "ca", label: "Català",       flag: "es-ct" },
  { code: "gl", label: "Galego",       flag: "es-ga" },
  { code: "eu", label: "Euskara",      flag: "es-pv" },
  { code: "id", label: "Bahasa Indonesia", flag: "id" },
  { code: "ms", label: "Bahasa Melayu",flag: "my" },
  { code: "hi", label: "हिन्दी",         flag: "in" },
  { code: "fa", label: "فارسی",         flag: "ir", rtl: true },
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

// Translated locales. en + de live inline above; the other 40 are imported from
// ./locales/<code>.ts (each typed as Translations, so a missing/renamed key
// fails the build). Any locale still absent from this map falls back to English.
// en + de are the complete source of truth; the other 40 are Partial and fall
// back to en at runtime for any missing key (see the t() lookup).
// (exported so the locale-parity test can iterate the full registry)
export const locales: Record<string, Partial<Translations>> = {
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
  bg,
  sk,
  sl,
  hr,
  sr,
  lt,
  lv,
  et,
  is,
  ca,
  gl,
  eu,
  id,
  ms,
  hi,
  fa,
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
