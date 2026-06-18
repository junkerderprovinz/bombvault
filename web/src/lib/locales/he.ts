import type { Translations } from "../i18n";

const he: Partial<Translations> = {
  // General
  "language.label": "שפה",
  "theme.toggle": "החלף ערכת נושא",
  "theme.dark": "כהה",
  "theme.light": "בהיר",

  // Nav
  "nav.dashboard": "לוח בקרה",
  "nav.containers": "Containers",
  "nav.vms": "VMs",
  "nav.flash": "Flash",
  "nav.settings": "הגדרות",
  "nav.comingSoon": "בקרוב",

  // Dashboard
  "dashboard.title": "לוח בקרה",
  "dashboard.lastBackups": "גיבויים אחרונים",
  "dashboard.recentRuns": "הרצות אחרונות",
  "dashboard.spikeStatus": "מצב המערכת",
  "dashboard.noRuns": "אין הרצות עדיין",
  "dashboard.spikeLink": "הפעל בדיקת שילוב מארח",
  "dashboard.hostIntegrationCheck": "בדיקת שילוב מארח",
  "dashboard.allOk": "כל המערכות תקינות",
  "dashboard.degraded": "פגום",
  "dashboard.checking": "בודק…",

  // Spike
  "spike.title": "שילוב מארח",
  "spike.overall": "סיכום:",
  "spike.allOk": "הכל תקין",
  "spike.degraded": "פגום",
  "spike.colCheck": "בדיקה",
  "spike.colStatus": "סטטוס",
  "spike.colDetail": "פרטים",
  "spike.ok": "OK",
  "spike.fail": "כשל",
  "spike.bestEffort": "אופציונלי",
  "spike.checkNow": "בדוק עכשיו",
  "spike.probeFailed": "הבדיקה נכשלה (ראה לוגי שרת)",

  // Containers
  "containers.title": "Containers",
  "containers.discover": "גלה גיבויים",
  "containers.discovering": "מגלה…",
  "containers.discoverHint": "אבד /config? בנה מחדש את רשימת הגיבויים מהאחסון.",
  "containers.backupNow": "גבה עכשיו",
  "containers.lastBackup": "גיבוי אחרון",
  "containers.never": "מעולם",
  "containers.colName": "שם",
  "containers.colImage": "תמונה",
  "containers.colStatus": "סטטוס",
  "containers.colAppdata": "Appdata",
  "containers.colActions": "פעולות",
  "containers.backupStarted": "הגיבוי התחיל",
  "containers.noDestination": "לא הוגדר יעד",
  "containers.includeInSchedule": "כלול בלוח הזמנים",
  "containers.schedule": "לוח זמנים",
  "containers.notInstalled": "לא מותקן",
  "containers.notInstalledTitle": "לא מותקן (גיבויים בלבד)",
  "containers.notInstalledHint": "ה-containers האלו אינם מותקנים עוד אך עדיין קיימים להם גיבויים. שחזר אותם או מחק את הגיבויים לפינוי מקום.",
  "containers.deleteBackups": "מחק את כל הגיבויים",
  "containers.deleteBackupsConfirm": "למחוק את כל הגיבויים של ה-container הזה? ה-snapshots יוסרו לצמיתות מהמאגר ולא ניתן לבטל פעולה זו.",
  "containers.filter": "סינון:",
  "containers.filterAll": "הכל",
  "containers.filterInstalled": "מותקנים",
  "containers.selectAll": "בחר הכל",
  "containers.selectedCount": "נבחרו",
  "containers.backupSelected": "גבה את הנבחרים",
  "containers.restoreSelected": "שחזר את הנבחרים (אחרון)",
  "containers.restoreSelectedConfirm": "לשחזר את הגיבוי האחרון של ה-containers הנבחרים? כל אחד יופסק, הנתונים יוחלפו והוא יופעל מחדש.",
  "containers.clearSelection": "נקה בחירה",
  "containers.working": "מבצע…",

  // Backups (restic snapshots)
  "snapshots.title": "גיבויים",
  "snapshots.colId": "מזהה",
  "snapshots.colTime": "זמן",
  "snapshots.colTags": "תגיות",
  "snapshots.colSize": "גודל",
  "snapshots.restore": "שחזר",
  "snapshots.none": "לא נמצאו גיבויים",

  // Restore
  "restore.confirmTitle": "אשר שחזור",
  "restore.confirmBody":
    "פעולה זו תעצור את ה-container, תחליף את הנתונים שלו ותיצור אותו מחדש מהגיבוי. להמשיך?",
  "restore.confirm": "אשר",
  "restore.cancel": "ביטול",
  "restore.preview": "תצוגה מקדימה",
  "restore.started": "השחזור התחיל",

  // Runs
  "run.kindBackup": "גיבוי",
  "run.kindRestore": "שחזור",
  "run.statusRunning": "פועל",
  "run.statusSuccess": "הצלחה",
  "run.statusFailed": "כשל",
  "run.historyTitle": "היסטוריית הרצות",
  "run.colKind": "סוג",
  "run.colStatus": "סטטוס",
  "run.colStarted": "התחיל",
  "run.colFinished": "הסתיים",
  "run.colContainer": "Container",

  // Settings
  "settings.title": "הגדרות",
  "settings.encryption": "הצפנה",
  "settings.encryptionOn": "מופעל (סיסמה נגזרת מ-APP_KEY)",
  "settings.encryptionOff": "מושבת (ללא סיסמה)",
  "settings.encryptionWarning":
    "ההצפנה קבועה לכל מאגר בעת האתחול. שינוי מחייב נתיב ריק חדש.",
  "settings.paths": "נתיבי גיבוי",
  "settings.containersPath": "נתיב Containers",
  "settings.vmsPath": "נתיב VMs",
  "settings.flashPath": "נתיב Flash",
  "settings.domains": "דומיינים",
  "settings.containersEnabled": "Containers",
  "settings.vmsEnabled": "VMs",
  "settings.flashEnabled": "Flash",
  "settings.schedule": "לוח זמנים",
  "settings.scheduleOff": "כבוי",
  "settings.language": "שפה",
  "settings.save": "שמור",
  "settings.saved": "ההגדרות נשמרו",
  "settings.error": "שגיאה בשמירה",

  // Appearance / Accent
  "settings.appearance": "מראה",
  "settings.accentColor": "צבע הדגשה",
  "settings.accentPresets": "ערכות מוגדרות מראש",

  // Dashboard stat cards
  "dashboard.statContainers": "Containers",
  "dashboard.statVMs": "VMs",
  "dashboard.statActiveJobs": "משימות פעילות",
  "dashboard.statPausedJobs": "משימות מושהות",
  "dashboard.statErrors": "שגיאות",
  "dashboard.statMissingContainers": "Containers חסרים",
  "dashboard.statMissingVMs": "VMs חסרים",

  // Jobs page
  "nav.jobs": "משימות",
  "jobs.title": "משימות",
  "jobs.subtitle": "תוכניות גיבוי לפי דומיין",
  "jobs.configureInSettings": "הגדר לוחות זמנים בהגדרות",
  "jobs.containersSection": "Containers",
  "jobs.vmsSection": "VMs",
  "jobs.flashSection": "Flash",
  "jobs.active": "פעיל",
  "jobs.paused": "מושהה",
  "jobs.notScheduled": "לא מתוזמן",
  "jobs.noVMs": "אין VMs עדיין",
  "jobs.noContainersIncluded": "אין containers בלוח הזמנים.",
  "jobs.flashRow": "הגדרת Unraid flash",
  "jobs.flashPlanned": "מתוכנן",
  "jobs.vmPlanned": "מנוע גיבוי VM טרם מומש.",

  // Auth / Login
  "auth.loginTitle": "BombVault",
  "auth.passwordLabel": "סיסמה",
  "auth.signIn": "התחבר",
  "auth.signingIn": "מתחבר…",
  "auth.invalidPassword": "סיסמה שגויה",
  "auth.loginError": "הכניסה נכשלה",

  // Settings — Security card
  "auth.security": "אבטחה",
  "auth.authOff": "האימות מושבת — לכל משתמשי ה-LAN גישה מלאה.",
  "auth.authOn": "האימות מופעל.",
  "auth.setPassword": "הגדר סיסמה",
  "auth.changePassword": "שנה סיסמה",
  "auth.confirmPassword": "אשר סיסמה",
  "auth.passwordMismatch": "הסיסמאות אינן תואמות",
  "auth.passwordSaved": "הסיסמה נשמרה",
  "auth.passwordCleared": "האימות הושבת",
  "auth.passwordHint":
    "השאר את שני השדות ריקים להשבתת האימות. ל-BombVault גישת root למארח — מומלץ להגדיר סיסמה אם המופע נגיש למשתמשים לא מהימנים ב-LAN.",
  "auth.logout": "התנתק",
  "auth.saving": "שומר…",
  "auth.saveError": "השמירה נכשלה",

  // VMs page
  "vms.title": "מכונות וירטואליות",
  "vms.subtitle": "נהל גיבויים, לוחות זמנים ושחזורים של VMs.",
  "vms.empty": "לא נמצאו VMs. האם libvirt/KVM פועל?",
  "vms.backupSelected": "גבה את הנבחרים",
  "vms.restoreSelected": "שחזר את הנבחרים (אחרון)",
  "vms.restoreSelectedConfirm": "לשחזר את הגיבוי האחרון של ה-VMs הנבחרים? כל VM יכובה, קבצי הדיסק יוחלפו וה-VM ישוחזר.",
  "vms.notInstalledHint": "ה-VMs האלו אינם מוגדרים עוד במארח אך עדיין קיימים להם גיבויים. שחזר אותם או עיין ב-snapshots שלהם בלוח הגיבויים.",

  // Container / VM state badge labels
  "state.created":      "נוצר",
  "state.running":      "פועל",
  "state.paused":       "מושהה",
  "state.restarting":   "מתאתחל",
  "state.removing":     "מוסר",
  "state.exited":       "יצא",
  "state.dead":         "מת",
  "state.shutoff":      "כבוי",
  "state.inshutdown":   "מכבה",
  "state.crashed":      "קרס",
  "state.pmsuspended":  "מושעה",
  "state.notInstalled": "לא מותקן",

  // Backups — files
  "snapshots.files": "קבצים",

  // File-level restore
  "files.restore": "שחזר",
  "files.restored": "שוחזר",
  "files.restoreConfirm": "לשחזר קובץ זה למיקומו המקורי? הקובץ הנוכחי יידרס.",
  "files.filterPlaceholder": "סינון קבצים…",
  "files.none": "אין קבצים תואמים",
  "files.loadFailed": "טעינת הקבצים נכשלה",
  "files.more": "צמצם את הסינון כדי לראות עוד קבצים.",

  // Retention
  "settings.retentionTitle": "שמירה",
  "settings.retentionHint": "כמה גיבויים לשמור לכל פריט. אחרי כל גיבוי, restic גוזם snapshots ישנים לפי מדיניות זו. הכל 0 = לשמור הכל (כבוי).",
  "settings.retentionLast": "שמור אחרונים",
  "settings.retentionDaily": "שמור יומיים",
  "settings.retentionWeekly": "שמור שבועיים",
  "settings.retentionMonthly": "שמור חודשיים",

  // Off-site (rclone)
  "rclone.title": "Off-site (rclone)",
  "rclone.hint": "הדבק תצורת rclone כדי לגבות לענן (Backblaze B2, S3, Google Drive, …). נשמרת מוצפנת. SMB/NFS אינם צריכים rclone: עגן את השיתוף ב-Unraid והצבע אליו בנתיב גיבוי.",
  "rclone.configured": "יעדים מרוחקים מוגדרים",
  "rclone.pathHint": "ואז הגדר נתיב גיבוי אל „rclone:<remote>:<bucket>/path‟ כדי לשלוח דומיין זה מחוץ לאתר.",
  "rclone.save": "שמור תצורה",

  // Integrity (restic check)
  "integrity.title": "שלמות",
  "integrity.hint": "הרץ restic check כדי לאמת שהמבנה והמטא-נתונים של המאגר תקינים.",
  "integrity.verify": "אמת",
  "integrity.checking": "בודק…",
  "integrity.ok": "✓ תקין",
  "integrity.failed": "הבדיקה נכשלה",

  // Pre/post-backup hooks
  "hooks.title": "Hooks לגיבוי",
  "hooks.hint": "הפקודות רצות בתוך ה-container (sh -c). Pre רץ לפני הגיבוי (למשל לדמפ DB אל appdata כדי שייכלל) — כישלון מבטל את הגיבוי. Post רץ אחרי שה-container שב לפעול; כישלונו רק נרשם בלוג.",
  "hooks.pre": "פקודת טרום-גיבוי",
  "hooks.post": "פקודת בתר-גיבוי",

  // Flash (Unraid USB) backup
  "flash.title": "גיבוי Flash",
  "flash.subtitle": "גבה ושחזר את כונן ה-USB flash של Unraid (כל ה-‎/boot).",
  "flash.backupTitle": "גבה את ה-flash",
  "flash.backupHint": "לוכד את כל כונן ה-USB flash (‎/boot): מערכת Unraid, רישיון, תצורת מערך, שיתופים, רשת ותצורת תוספים.",
  "flash.backupNow": "גבה flash עכשיו",
  "flash.backingUp": "מגבה…",
  "flash.restoring": "מחלץ…",
  "flash.restoreNote": "השחזור מְחַלֵּץ snapshot לתיקייה (מוצגת למטה) — לעולם אינו דורס את ה-‎/boot הפעיל. העתק את הקבצים המשוחזרים ל-USB חדש כדי לבנות מחדש את ה-flash.",
  "flash.restoredTo": "חולץ אל:",
  "flash.none": "אין עדיין גיבויי flash — הרץ גיבוי למעלה.",

  // VM backup (SSH)
  "vm.method": "שיטה",
  "vm.method.graceful": "מסודרת (כיבוי)",
  "vm.method.live": "Snapshot חי",
  "vm.ssh.title": "גיבוי VM דרך SSH",
  "vm.ssh.desc": "גיבוי VM מגיע ל-libvirt דרך SSH (ללא עיגון). אשר מפתח זה ב-Unraid, ואז בדוק.",
  "vm.ssh.host": "Host",
  "vm.ssh.publicKey": "מפתח ציבורי — הוסף ל-‎/root/.ssh/authorized_keys של Unraid",
  "vm.ssh.copy": "העתק",
  "vm.ssh.copied": "הועתק",
  "vm.ssh.test": "בדוק חיבור",
  "vm.ssh.testing": "בודק…",
  "vm.ssh.testOk": "מחובר — libvirt נגיש",
  "vm.ssh.testFail": "החיבור נכשל",
  "vm.ssh.setupTitle": "הגדרה (פעם אחת)",
  "vm.ssh.step1": "העתק את הפקודה למטה והרץ אותה במסוף Unraid כדי לאשר מפתח זה (שורד אתחולים).",
  "vm.ssh.step2": "הגדר את משתנה ה-container „VM Backup: Host‟ לכתובת ה-LAN IP של Unraid (למשל 192.168.x.x); ברשת bridge פשוטה גם host.docker.internal עובד.",
  "vm.ssh.step3": "לחץ על בדוק חיבור — ברגע שהוא ירוק, הפעל VMs תחת דומיינים.",
  "vm.ssh.copyCmd": "העתק פקודה",
  "vm.ssh.guide": "מדריך הגדרה ורשת מלא",
};

export default he;
