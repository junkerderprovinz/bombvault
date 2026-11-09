# Praxisbeispiele

Jedes dieser Beispiele führt eine echte Aufgabe von Anfang bis Ende durch, mit eingesetzten Namen und Zahlen. Sie hängen an der Seite, die die Funktion erklärt, sodass die Einzelheiten gleich daneben stehen.

## Erste Schritte {#start}

- [Deine erste Container-Sicherung von A bis Z](getting-started.md#example-first-backup): von der Installation bis zum Recovery-Kit.

## Speicher und Off-site {#storage}

- [Eine Off-site-Kopie bei Backblaze B2](offsite-recovery.md#example-s3): ein Bucket, ein Schlüssel nur für diesen Bucket und die erste Kopie. Cloudflare R2 funktioniert genauso.
- [Ein rest-server, den du selbst betreibst](offsite-recovery.md#example-rest-server): eine Off-site-Kopie, die Löschen verweigert, mit der `docker run`-Zeile und dem Manipulationstest.
- [Was Ausgewogen aufbewahrt](offsite-recovery.md#example-retention): die Aufbewahrung über zwei Jahre nächtlicher Sicherungen, Sicherung für Sicherung.
- [Container ziehen auf ein NAS um](offsite-recovery.md#example-own-folder): Der eigene Ordner eines Bereichs wechselt den Ort, und was mit den schon vorhandenen Sicherungen passiert.

## Sicherungen und Zeitpläne {#backups}

- [Drei Pläne, die zusammenpassen](schedules.md#example): ein Gesamt-Backup in der Nacht, VMs einmal die Woche und ein eigener Plan für einen Eintrag alle 6 Stunden.
- [Plex wartet, solange jemand schaut](backups.md#example-plex): eine geplante Sicherung, die keinen Film unterbricht.
- [Eine VM mit zwei Unraid-Snapshots](vm-backup-ssh-setup.md#example-unraid-snapshots): eine Live-Sicherung, die die Snapshots behält, und eine Wiederherstellung neben das Original.
- [Eine Replik auf eine zweite Unraid-Box](zfs-datasets.md#example-replica): eine ZFS-Kopie, die du jederzeit lesen kannst, und drei Wege, eine einzelne Datei daraus zu holen.

## Etwas zurückholen {#restore}

- [Eine Datei zurückholen](restore.md#example-single-file): eine einzelne Datei aus der Container-Sicherung der letzten Nacht.
- [Eine Datei aus einer ZFS-Sicherung](zfs-datasets.md#example-zfs-file): dasselbe für ein Dataset.
- [Ein neuer Server aus dem Recovery-Kit](offsite-recovery.md#example-emergency): Ein toter Server kommt auf einer neuen Maschine zurück, ohne seinen alten `APP_KEY` im Template.

## Auf dem Laufenden bleiben {#staying-informed}

- [Gotify auf dem Handy](notifications.md#example-gotify): Fehler klingeln, Erfolge kommen leise an.
- [ntfy ohne Konto](notifications.md#example-ntfy): Nachrichten an ein eigenes Topic.

## Mehrere Server {#several-servers}

- [Zwei Unraid-Boxen koppeln](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): zwei Server, die gegenseitig ihre Kopien verwahren, und keiner kann die Sicherungen des anderen löschen.
- [Eine Karte und ein Alarm in Home Assistant](api.md#home-assistant-example): BombVault auf einem Dashboard, mit einer Nachricht, wenn eine Sicherung fehlschlägt.
