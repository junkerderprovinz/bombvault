# BombVault

**הנתונים שלך ב-Unraid, חתומים בכספת. שחרר גיבוי. פוצץ שחזור.**

BombVault היא אפליקציית ווב מתארחת-עצמית, ילידת Unraid, ל**גיבוי והתאוששות מאסון מלאה** של ה-Docker containers ומכונות ה-KVM/libvirt שלך. היא רצה כ-Docker container יחיד רב-ארכיטקטורה, מעניקה לך ממשק ווב מודרני שמתאים את עצמו להעדפת המצב הבהיר/הכהה של המערכת שלך, ומטפלת בכל מחזור החיים: גיבוי, תזמון, אימות ושחזור.

שחזורים הם אוטומטיים. Containers מופיעים מחדש בלשונית Docker של Unraid בדיוק כמו קודם, ומכונות וירטואליות מוגדרות מחדש ב-VM Manager עם הדיסקים ו-UEFI NVRAM שלהן מחוברים מחדש. ללא התקנה ידנית מחדש, ללא הגדרה מחדש, ללא דרמה.

מונע על ידי [restic](https://restic.net), כך שכל גיבוי מבוצע דדופליקציה, אינקרמנטלי ותמיד מוצפן.

!!! note "שמור על ה-APP_KEY שלך"
    BombVault גוזרת את סיסמת מאגר ה-restic מסוד באורך 32 בתים בשם `APP_KEY`. אובדנו הופך את הגיבויים המוצפנים לבלתי ניתנים לשחזור. צור אחד באמצעות `openssl rand -hex 32` ואחסן אותו במקום בטוח. ראה [הגדרות](configuration.md).

## מה BombVault מגן עליו

| דומיין | מה נשמר |
|---|---|
| **Docker containers** | תיקיית appdata בתוספת הגדרת ה-container (image, משתני env, פורטים, תוויות, volumes). |
| **KVM / libvirt VMs** | קבצי דיסק ה-VM, הגדרת ה-XML ו-UEFI NVRAM, מגובים דרך SSH (ללא עיגון libvirt). |
| **Unraid flash** | כל כונן ה-USB flash (`/boot`): מערכת ההפעלה, הרישיון, תצורת המערך, השיתופים, הרשת ותצורת התוספים. |
| **הגדרות האפליקציה** | ה-`/config` של BombVault עצמה: מסד נתוני ההגדרות שלה, פרטי ההתחברות מחוץ לאתר וזוג מפתחות ה-SSH של libvirt. |
| **קבצים ותיקיות** | **קבוצות קבצים** בעלות שם, כל תיקייה בשרת, כל אחת עם דפוסי החרגה אופציונליים לכל קבוצה. |
| **מערכי נתונים של ZFS** | מערך נתונים עם כל מה שמתחתיו, נקרא מתצלום ZFS אחד ונשמר כמו תיקייה. ראה [מערכי נתונים של ZFS](zfs-datasets.md). |

## השחזור הוא הכוכב

לאחר העתקת הנתונים בחזרה מתמונת המצב של restic, BombVault משחזרת את הגדרת ה-container השמורה מול ה-Docker API, כך שה-container מופיע מחדש בלשונית Docker של Unraid כאילו תמיד היה שם (אותו image, אותן הגדרות, אותם מיפויי פורטים). מכונות וירטואליות מקבלות את ה-XML שלהן מוגדר מחדש דרך SSH ואת הדיסקים ו-UEFI NVRAM שלהן מחוברים מחדש, גם לאחר שה-VM נמחקה.

כאשר גיבוי עוצר containers תלויים, הם חוזרים בסדר הנכון: BombVault מפעילה אותם מחדש בסדר ה-`depends_on` של Compose וממתינה שכל אחד ידווח על מצב תקין לפני הפעלת אלה התלויים בו, כך ששום דבר לא מקדים מסד נתונים או שער שעדיין לא עלה. ראה [תכונות](features.md).

## איך זה עובד

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

‏BombVault משתמשת ב-Docker socket כדי לעצור containers לפני גיבוי וכדי ליצור אותם מחדש אחרי שחזור. עבור מכונות וירטואליות היא מריצה `virsh` במארח דרך SSH (`qemu+ssh://`) כדי לכבות מכונה באופן מסודר או לצלם תמונת מצב חיה. היא לעולם אינה מעגנת נתיב libvirt, ולכן אינה יכולה להפריע ל-VM Manager במארח.

BombVault היא שכבת התזמור והממשק, לא מנוע האחסון. כל תנועת הנתונים בפועל עוברת דרך restic.

## התחלה מהירה

חדש כאן? עבור אל **[תחילת העבודה](getting-started.md)** כדי להתקין את BombVault ב-Unraid דרך Community Applications ולהריץ את הגיבוי הראשון שלך. לאחר מכן חקור את כל **[התכונות](features.md)**, כוונן את **[ההגדרות](configuration.md)** שלך, והגדר **[מחוץ לאתר והתאוששות](offsite-recovery.md)**.

מחוץ לאתר יכול להתפצל למספר יעדים לכל דומיין בבת אחת, **לוח בקרה של מקבל** לקריאה בלבד מנטר את העותקים האלה בתיבה שמקבלת אותם, ואתה יכול לשאת את כל התצורה שלך לתיבה חדשה עם כרטיס **ייצוא / ייבוא הגדרות**. ראה [מחוץ לאתר והתאוששות](offsite-recovery.md) ו-[הגדרות](configuration.md#portable-settings-export-and-import).

**[אפליקציית Android](android.md)** מביאה לטלפון שלך את כל השרתים של הקבוצה שלך, עם יומן הפעילות של כולם במסך אחד.

## קרדיטים {#credits}

- **‏[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** מאת [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) נתן ל-BombVault את רעיון המוצא שלה: גיבוי בלחיצה אחת והתקנה מחדש אוטומטית של Docker containers. ‏BombVault היא מימוש נפרד ב-Go וב-restic, שמרחיב את הרעיון למכונות וירטואליות, ל-flash ועוד.
- **‏[restic](https://restic.net/)** הוא מנוע הגיבוי המהיר, המאובטח ומבצע הדדופליקציה ש-BombVault מפעילה.
- **‏[rclone](https://rclone.org/)** מספק את ה-backends של הענן.
- רוב הסמלים שעל הכפתורים לקוחים מהסט החינמי Core Solid של **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [מקור](https://github.com/webalys-hq/streamline-vectors)). השאר לקוחים מ-Font Awesome Free,‏ Material Design Icons,‏ Simple Icons ו-Tabler Icons, או שצוירו עבור הפרויקט.

## רישיון {#license}

‏Copyright (C) 2026 Junker der Provinz. ‏BombVault היא תוכנה חופשית תחת **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). מותר לך להריץ אותה, ללמוד אותה, לשתף אותה ולשנות אותה. אם אתה מפיץ אותה, או מריץ גרסה ששונתה כשירות רשת, עליך לפרסם את קוד המקור שלך תחת אותו רישיון ולשמור על הודעות זכויות היוצרים והייחוס הקיימות.

השם והמיתוג אינם כלולים ברישיון. ה-AGPL חל על קוד המקור בלבד: הזכויות על השם "BombVault", על הלוגו שלה ועל המיתוג שלה שמורות, ולכן fork חייב להשתמש בשם ובמיתוג משלו ואסור לו להציג את עצמו כ-BombVault.

## קישורים

- **קוד מקור:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **שרשור התמיכה של Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **בעיות:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "שליטה שוות-ערך ל-root על המארח"
    דרך ה-Docker socket, BombVault יכולה לעצור, להסיר וליצור מחדש containers ולקרוא/לכתוב appdata, ולגיבוי VM היא מתחברת למארח דרך SSH כדי להריץ `virsh`. כל מי שיכול להגיע לממשק הווב שלה שולט למעשה כ-root על המארח. הרץ את BombVault רק ברשת מהימנה ולא חשופה, והפעל את שער הסיסמה האופציונלי (הגדרות, אבטחה) ברגע שגיבויים מחוץ לאתר או בלתי-ניתנים-לשינוי נמצאים בשימוש. ראה [הגדרות](configuration.md) למודל האבטחה המלא.
