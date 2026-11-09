# דוגמאות מעשיות

כל אחת מהדוגמאות האלה עוברת על משימה אמיתית אחת מההתחלה עד הסוף, עם שמות ומספרים מלאים. הן יושבות בעמוד שמסביר את התכונה, כך שהפרטים נמצאים לידן.

## תחילת העבודה {#start}

- [גיבוי הקונטיינר הראשון שלך, מקצה לקצה](getting-started.md#example-first-backup): מההתקנה ועד ערכת השחזור.

## אחסון ומחוץ לאתר {#storage}

- [עותק מחוץ לאתר ב-Backblaze B2](offsite-recovery.md#example-s3): דלי, מפתח לדלי הזה והעותק הראשון. Cloudflare R2 עובד באותה דרך.
- [rest-server שמריצים בעצמך](offsite-recovery.md#example-rest-server): עותק מחוץ לאתר שמסרב למחיקות, עם שורת `docker run` ובדיקת החבלה.
- [מה שומרת התוכנית המאוזנת](offsite-recovery.md#example-retention): שמירה על פני שנתיים של גיבויים לילִיים, גיבוי אחר גיבוי.
- [קונטיינרים עוברים ל-NAS](offsite-recovery.md#example-own-folder): התיקייה העצמית של תחום מחליפה מקום, ומה קורה לגיבויים שכבר נעשו.

## גיבויים ולוחות זמנים {#backups}

- [שלוש תוכניות שמתאימות זו לזו](schedules.md#example): גיבוי מלא בלילה, מכונות וירטואליות פעם בשבוע, ותוכנית משלה לפריט אחד כל 6 שעות.
- [Plex ממתין בזמן שמישהו צופה](backups.md#example-plex): גיבוי מתוזמן שלא קוטע סרט.
- [מכונה וירטואלית עם שני תצלומי Unraid](vm-backup-ssh-setup.md#example-unraid-snapshots): גיבוי חי ששומר את התצלומים, ושחזור לצד המקור.
- [רפליקה לתיבת Unraid שנייה](zfs-datasets.md#example-replica): עותק ZFS שאפשר לקרוא בכל עת, ושלוש דרכים להוציא ממנו קובץ אחד.

## להחזיר דברים {#restore}

- [להחזיר קובץ אחד](restore.md#example-single-file): קובץ בודד מגיבוי הקונטיינר של אמש.
- [קובץ אחד מגיבוי ZFS](zfs-datasets.md#example-zfs-file): אותו דבר למערך נתונים.
- [שרת חדש מתוך ערכת השחזור](offsite-recovery.md#example-emergency): שרת שמת חוזר בתיבה חדשה, בלי ה-`APP_KEY` הישן שלו בתבנית.

## להישאר מעודכנים {#staying-informed}

- [Gotify בטלפון](notifications.md#example-gotify): כשלונות מצלצלים והצלחות מגיעות בשקט.
- [ntfy בלי חשבון](notifications.md#example-ntfy): הודעות לנושא משלך.

## כמה שרתים {#several-servers}

- [צימוד של שתי תיבות Unraid](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): שני שרתים ששומרים זה את העותקים של זה, ואף אחד מהם לא יכול למחוק את הגיבויים של השני.
- [כרטיס והתראה ב-Home Assistant](api.md#home-assistant-example): BombVault בלוח מחוונים, עם הודעה כשגיבוי נכשל.
