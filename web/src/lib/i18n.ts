// ---------------------------------------------------------------------------
// Minimal i18n — en + de; other locales stub to English.
// useT() returns a translation function. Language state is in localStorage.
// ---------------------------------------------------------------------------

import { useState, useCallback } from "react";

// ---------------------------------------------------------------------------
// Locale strings — en is the source of truth
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
  "containers.includeInSchedule": "Include in schedule",
  "containers.schedule": "Schedule",

  // Snapshots
  "snapshots.title": "Snapshots",
  "snapshots.colId": "ID",
  "snapshots.colTime": "Time",
  "snapshots.colTags": "Tags",
  "snapshots.colSize": "Size",
  "snapshots.restore": "Restore",
  "snapshots.none": "No snapshots found",

  // Restore
  "restore.confirmTitle": "Confirm restore",
  "restore.confirmBody":
    "This will stop the container, replace its appdata and recreate it from the snapshot. Continue?",
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
} as const;

type TranslationKey = keyof typeof en;
type Translations = Record<TranslationKey, string>;

// ---------------------------------------------------------------------------
// German locale
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
  "containers.discover": "Erkennen",
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

  "snapshots.title": "Snapshots",
  "snapshots.colId": "ID",
  "snapshots.colTime": "Zeitpunkt",
  "snapshots.colTags": "Tags",
  "snapshots.colSize": "Größe",
  "snapshots.restore": "Wiederherstellen",
  "snapshots.none": "Keine Snapshots gefunden",

  "restore.confirmTitle": "Wiederherstellung bestätigen",
  "restore.confirmBody":
    "Der Container wird gestoppt, seine Appdata ersetzt und aus dem Snapshot neu erstellt. Fortfahren?",
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
};

// ---------------------------------------------------------------------------
// Locale registry
// ---------------------------------------------------------------------------

type LangCode = "en" | "de";

const locales: Record<LangCode, Translations> = { en, de };

const SUPPORTED_LANGS: LangCode[] = ["en", "de"];
const LANG_NAMES: Record<LangCode, string> = { en: "English", de: "Deutsch" };
const STORAGE_KEY = "bv-lang";
const DEFAULT_LANG: LangCode = "en";

function resolveCode(raw: string | null): LangCode {
  if (raw === "en" || raw === "de") return raw;
  const browser = navigator.language.slice(0, 2);
  if (browser === "en" || browser === "de") return browser;
  return DEFAULT_LANG;
}

function getLang(): LangCode {
  return resolveCode(localStorage.getItem(STORAGE_KEY));
}

function setLang(code: LangCode): void {
  localStorage.setItem(STORAGE_KEY, code);
}

/** Called at boot in main.tsx. */
export function applyStoredLanguage(): void {
  const code = getLang();
  document.documentElement.setAttribute("lang", code);
}

// ---------------------------------------------------------------------------
// useT() hook
// ---------------------------------------------------------------------------

/** Returns [t, currentLang, setLanguage, supportedLangs, langNames]. */
export function useT(): {
  t: (key: TranslationKey) => string;
  lang: LangCode;
  setLanguage: (code: LangCode) => void;
  supportedLangs: LangCode[];
  langNames: Record<LangCode, string>;
} {
  const [lang, setLangState] = useState<LangCode>(getLang);

  const setLanguage = useCallback((code: LangCode) => {
    setLang(code);
    document.documentElement.setAttribute("lang", code);
    setLangState(code);
  }, []);

  const t = useCallback(
    (key: TranslationKey): string => {
      const locale = locales[lang] ?? locales[DEFAULT_LANG];
      return locale[key] ?? en[key] ?? key;
    },
    [lang]
  );

  return { t, lang, setLanguage, supportedLangs: SUPPORTED_LANGS, langNames: LANG_NAMES };
}
