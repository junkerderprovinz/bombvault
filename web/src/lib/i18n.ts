// ---------------------------------------------------------------------------
// i18n — React Context-based, 26 locales, flag switcher support
// ---------------------------------------------------------------------------

import { createContext, useContext, useState, useCallback } from "react";
import type { ReactNode } from "react";
import { createElement } from "react";

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
  "nav.jobs": "Jobs",
  "jobs.title": "Jobs",
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

  // VMs page
  "vms.title": "Virtual Machines",
  "vms.subtitle": "Manage VM backups, schedules, and restores.",
  "vms.empty": "No VMs found. Is libvirt/KVM running?",
  "vms.backupSelected": "Back up selected",
  "vms.restoreSelected": "Restore selected (latest)",
  "vms.restoreSelectedConfirm": "Restore the LATEST backup of the selected VMs? Each VM is shut off, its disk files replaced, and the VM restored.",
  "vms.notInstalledHint": "These VMs are no longer defined on the host but still have backups. Restore them to recover, or use the Backups panel to browse their snapshots.",

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
} as const;

export type TranslationKey = keyof typeof en;
type Translations = Record<TranslationKey, string>;

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
  "nav.jobs": "Jobs",
  "jobs.title": "Jobs",
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

  // VMs page
  "vms.title": "Virtuelle Maschinen",
  "vms.subtitle": "VM-Backups, Zeitpläne und Wiederherstellungen verwalten.",
  "vms.empty": "Keine VMs gefunden. Läuft libvirt/KVM?",
  "vms.backupSelected": "Auswahl sichern",
  "vms.restoreSelected": "Auswahl wiederherstellen (neuestes)",
  "vms.restoreSelectedConfirm": "Das NEUESTE Backup der ausgewählten VMs wiederherstellen? Jede VM wird heruntergefahren, ihre Disk-Dateien ersetzt und die VM wiederhergestellt.",
  "vms.notInstalledHint": "Diese VMs sind nicht mehr auf dem Host definiert, haben aber noch Backups. Stelle sie wieder her oder sieh ihre Snapshots im Backups-Panel ein.",

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

// Fully translated locales — all others fall back to English at runtime.
const locales: Record<string, Translations> = { en, de };

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
