# Funktionen

Ein kurzer Katalog dessen, was BombVault kann. Jede Zeile verlinkt auf die Seite, die es erklärt.

## Sicherungsumfang {#backup-scope}

- **Container**: der Appdata-Ordner und die Container-Definition, mit Ordnerauswahl pro Container. [Sicherungen und Einträge](backups.md#what-is-saved)
- **VMs**: Disk-Images, XML und UEFI-NVRAM über SSH, mit geordnetem Herunterfahren, Live-Snapshot oder nur geänderten Blöcken. VMs mit Unraid-Snapshots und Disks auf zvols sind vollständig abgedeckt. [VM-Sicherungen](vm-backup-ssh-setup.md)
- **Flash**: der gesamte Unraid-USB-Flash, mit geplantem ZIP-Export. [Sicherungen und Einträge](backups.md#settings-tab)
- **Selbstbackup**: die eigenen Einstellungen und Zugangsdaten von BombVault. [Notfall-Assistent](offsite-recovery.md#emergency)
- **Ordner**: jeder Ordner auf dem Server als Ordner-Set. [Einen Ordner hinzufügen](backups.md#add)
- **ZFS-Datasets**: ein Dataset-Baum, gelesen aus einem Snapshot, mit einzeln wiederherstellbaren Dateien und einer optionalen Replik auf einem zweiten ZFS-Server. [ZFS-Datasets](zfs-datasets.md)
- **Datenbank-Dumps**: PostgreSQL, MySQL und MariaDB, vom laufenden Server in einen eigenen Wiederherstellungspunkt gedumpt. [Datenbank-Dumps](backups.md#database-dumps)

## Sicherungen und Eintragsseiten {#backups}

- **Eine Liste** mit Kacheln, **Filter**, **Sortieren** und **Suchen**, und den letzten vierzehn Läufen jeder Zeile als Streifen. [Die Liste](backups.md#list)
- **Auswählen wie im Dateimanager**, und eine Auswahl auf dem Server sichern oder pausieren. [Mehrere Einträge auswählen](backups.md#select)
- **Eine Seite pro Eintrag** mit **Sicherungen**, **Einstellungen** und **Prüfung**. [Die Seite eines Eintrags](backups.md#entry-page)
- **Tags und Vergleich** für Snapshots, und **Exportieren** als tar, zip oder restic-Repository. [Tab Sicherungen](backups.md#backups-tab)
- **Ausschlussmuster** mit Live-Vorschau und empfohlenen Ausschlüssen für bekannte Apps. [Ausschlussmuster](backups.md#excludes)
- **Befehle vorher und nachher**, und **Andere Container stoppen** mit einem Neustart in Abhängigkeitsreihenfolge, der auf Gesundheit wartet. [Befehle und andere Container](backups.md#commands)
- **Wenn die App beschäftigt ist**: Eine geplante Sicherung wartet auf Plex, Jellyfin, Emby, deine eigene Prüfung oder ruhigen Datenverkehr und versucht es von selbst wieder. [Wenn die App beschäftigt ist](backups.md#app-busy)
- **Snapshot statt Stopp**: Ein Container auf ZFS steht nur ein paar Sekunden still. [Zeitpläne](schedules.md#snapshot-instead-of-stop)
- **Einfache Exporte**, auf Wunsch mit age versiegelt. [Einfache Exporte](backups.md#exports)

## Wiederherstellung {#restore}

- **Eine Seite und ein Fenster** für jede Art, mit vorgewählter empfohlener Sicherung. [Wiederherstellung](restore.md)
- **Neben das Original** als Vorgabe, und **Original ersetzen** sichert vorher den heutigen Zustand. [Das Wiederherstellungsfenster](restore.md#window)
- **Einzelne Dateien** in einen Ordner oder an ihren Platz zurück. [Eine Datei zurückholen](restore.md#example-single-file)
- **Mehrere Einträge auf einmal**, jeden aus seiner empfohlenen Sicherung oder von einem Zeitpunkt. [Mehrere Einträge wiederherstellen](restore.md#several)
- **Ganze Compose-Stacks**, in Abhängigkeitsreihenfolge gestartet. [Einen Stack wiederherstellen](restore.md#stacks)
- **Container und VMs werden neu angelegt** auf dem Image, auf dem sie liefen, mit GPU, Limits und Links. [Was zurückkommt](restore.md#kinds)
- **Prüfungen vor der Wiederherstellung** und eine Liste dessen, was sie ändert. [Das Wiederherstellungsfenster](restore.md#window)
- **Flash als ZIP**, und ein Plugin nach dem anderen zurück in den laufenden Flash. [Was zurückkommt](restore.md#kinds)
- **ZFS** in das Dataset mit Sicherheits-Snapshot, in einen Ordner oder in ein neues Dataset mit seinen Eigenschaften. [ZFS-Datasets](zfs-datasets.md#restore)
- **Notfall-Assistent** für einen verlorenen Server, mit dem Recovery-Kit, über die Gruppe oder mit dem eingetippten Schlüssel. [Notfall-Assistent](offsite-recovery.md#emergency)
- **Aus einem alten Sicherungsordner**, **Aus dem Appdata.Backup-Plugin übernehmen** und **Jobs aus Duplicati oder Duplicacy**, das ihre Sicherungsjobs in Ordner-Sets umwandelt. [Andere Quellen](restore.md#other-sources)

## Speicher und Zeitplanung {#storage-scheduling}

- **Speicherorte** jeder Art: lokale Ordner, SMB, WebDAV, SFTP, rest-server, S3, Cloud-Laufwerke und jedes rclone-Remote. [Speicherorte](offsite-recovery.md#destinations)
- **Zugangsdaten**: eine Liste mit einer Vorgabe. [Zugangsdaten](offsite-recovery.md#sign-ins)
- **Eigene Ordner** auf diesem Server oder in einem Speicherort. [Der eigene Ordner eines Bereichs](offsite-recovery.md#own-folder)
- **3-2-1 als Ablauf**, in dem jede Zeile ein Schalter ist, und **Zuordnung pro Eintrag**. [Zuordnung pro Eintrag](offsite-recovery.md#placement)
- **Off-site-Kopien** an mehrere Orte pro Bereich, mit Bandbreitenlimits, Kompression, Cold-Storage-Klassen und einer Bremse, während Plex streamt. [Off-site-Kopien](offsite-recovery.md#off-site-copies)
- **Direkte Repositories**, aufgelistet unter **Sichert direkt hierher**. [Direkte Repositories](offsite-recovery.md#direct-repositories)
- **Aufbewahrung als Zeitleiste**: **Kurz**, **Ausgewogen**, **Lang** oder **Eigene Werte** pro Ort, mit Regeln pro Bereich. [Wie lange aufheben](offsite-recovery.md#keep)
- **Den Verlauf eines Bereichs umziehen**, und **Speicherort entfernen** mit oder ohne seine Sicherungen. [Umziehen und entfernen](offsite-recovery.md#move-remove)
- **Zeitpläne, die sich wie Sätze lesen**, das Band **Woche**, ein Gesamt-Backup, Pläne pro Bereich und ein eigener Plan pro Eintrag. [Zeitpläne](schedules.md)
- **Verpasste Sicherungen nachholen**, eine manuelle Reihenfolge und Updates nach einer erfolgreichen Sicherung. [Zeitpläne](schedules.md)

## Überblick, Prüfung und Überwachung {#insight-verification-monitoring}

- **Die Übersicht**: Statuskopf, **Braucht dich**, **Schutzstatus**, Speicherprognose, Verlauf und optionale Karten. [Übersicht](overview.md)
- **Anomalie-Erkennung** pro Eintrag, die auch alte Sicherungen behält, wenn eine Quelle zusammenbricht. [Anomalien](anomalies.md)
- **Wiederherstellungsprüfung** nach der ersten Sicherung und **Start-Test** in einer isolierten Kopie. [Tab Prüfung](backups.md#verify-tab)
- **Geplante Prüfungen und Probe-Wiederherstellungen** aus der Off-site-Kopie. [DR-Tests](offsite-recovery.md#dr-drills)
- **Größe nach Ordner**, und warum eine Sicherung langsam war. [Was ein Lauf dir sagt](backups.md#run-notes)
- **Benachrichtigungen**: eine Liste von Kanälen für Unraid, ntfy, Gotify, Discord, Slack, Matrix, E-Mail, Apprise, jeden Webhook und Healthchecks, jeder mit eigener Regel, mit Wochenbericht und Bericht über Überfälliges. [Benachrichtigungen](notifications.md)
- **Prometheus**, ein Home-Assistant-Gerät, eine HTTP-API und mDNS. [API und Integrationen](api.md)
- **Fehlerbericht**: eine bereinigte ZIP für ein GitHub-Issue. [Fehlersuche](troubleshooting.md#still-stuck)

## Ransomware-Schutz {#ransomware-protection}

- **Löschschutz (append-only)**, von der Gegenseite durchgesetzt und durch einen Manipulationstest belegt. [Löschschutz](offsite-recovery.md#delete-protection-append-only)
- **Ohne Löschschutz akzeptieren** für einen Ort, der ein Löschen nicht verweigern kann. [Löschschutz](offsite-recovery.md#delete-protection-append-only)
- **Das endgültige Löschen von Sicherungen braucht deine eigene Anmeldung**, sobald ein Login-Passwort gesetzt ist, und ein gekoppeltes Handy kann es nicht. [Sicherheitsmodell](configuration.md#security-model)
- **Recovery-Kit**, auf Wunsch mit age versiegelt, und ein **Schlüsselhüter** in deiner Gruppe. [Recovery-Kit](offsite-recovery.md#recovery-kit), [Schlüsselhüter](offsite-recovery.md#key-keeper)

## Mehrere Server {#several-servers}

- **Kopplung mit zwölf Wörtern**, Ende-zu-Ende-verschlüsselt, direkt oder über ein Relay. [Instanzen koppeln](offsite-recovery.md#pairing)
- **Die Seite Instanzen** mit der Gruppe als Netz und den Rollen **Empfänger**, **Abholer**, **ZFS-Server** und **Schlüsselhüter**. [Instanzen und Rollen](offsite-recovery.md#roles)
- **Ein Empfangsserver**, für dich eingerichtet, im Append-only-Modus. [Empfangsserver](offsite-recovery.md#receiving-server)

## KI-Assistenten (MCP) {#mcp}

Ein eingebauter MCP-Server lässt einen Assistenten Sicherungsstatus, Abdeckung, Laufverlauf und Wiederherstellungspunkte lesen und mit einem Schlüssel, der es erlaubt, eine Sicherung starten. Wiederherstellungen, Löschungen, Aufräumen und Einstellungen bleiben in der Weboberfläche. [MCP-Server](mcp.md)

## Apps und Begleiter {#apps}

- **Android-App**: jeder Server deiner Gruppe auf dem Handy. [Android-App](android.md)
- **BombVault Widget**: eine Kachel im Unraid-Dashboard. [Apps](settings.md#apps)
- **Einbettbares Aktivitätsprotokoll** für Dashboards wie Homepage oder Heimdall. [Dashboard-Widget](api.md#dashboard-widget)

## Weiteres {#other}

- **42 Sprachen**, drei Designs, und Beschriftungen, Farben, Ecken und Animation nach deiner Wahl. [Aussehen](settings.md#look)
- **Einstellungen, die sich selbst speichern**, mit einer Suche über alle vierzehn Kacheln. [Einstellungen](settings.md)
- **Was ist neu** nach einem Update, mit grünen Punkten an dem, was sich geändert hat. [Info](settings.md#info)
- **HTTPS ab Werk** und ein Healthcheck für Auto-Heal-Werkzeuge. [Konfiguration](configuration.md#network)
