# Sicherungen und Einträge

Alles, was BombVault sichert, steht in einer Liste unter **Sicherungen**: Container, VMs, Ordner-Sets, der Flash, ZFS-Datasets und das Selbstbackup. Jedes davon ist ein **Eintrag** mit einer eigenen Seite. Diese Seite führt durch die Liste, die Eintragsseite und ihre drei Tabs sowie die Einstellungen, die ein Eintrag haben kann.

![Die Liste Sicherungen mit gewählter Kachel Container: Jede Zeile zeigt ihre letzten Läufe als Streifen und einen Status.](assets/screenshots/backups/list-dark.png#only-dark)
![Die Liste Sicherungen mit gewählter Kachel Container: Jede Zeile zeigt ihre letzten Läufe als Streifen und einen Status.](assets/screenshots/backups/list-light.png#only-light)

## Was jeder Bereich sichert {#what-is-saved}

| Bereich | Was gesichert wird |
|---|---|
| **Container** | Der Appdata-Ordner plus die Container-Definition: Image, Variablen, Ports, Labels und Volumes. Die Karte **Gesicherte Ordner** auf dem Tab **Einstellungen** des Containers hakt genau ab, welche Ordner die Sicherung abdeckt, mit einer laufenden Zählung der Pfade und einer Liste dessen, was du ausgelassen hast. Daten, die ein Container außerhalb von Host Data ablegt, etwa in einem TrueNAS-ixVolume oder einem Compose-Projekt unter `/opt`, werden nicht gesichert, und der Lauf nennt die Ordner, die er ausgelassen hat. Der Lauf nennt auch einen Ordner innerhalb von Host Data, den es gerade nicht gibt. Systempfade wie `/etc/localtime` oder der Docker-Socket zählen nicht als Daten. |
| **VMs** | Die Disk-Images, die XML-Definition und das UEFI-NVRAM, mit einem Graceful-Shutdown oder einem Live-Snapshot, über SSH. Siehe [VM-Sicherungen](vm-backup-ssh-setup.md). |
| **Flash** | Nur Unraid. Der ganze USB-Flash (`/boot`): Betriebssystem, Lizenz, Array-Konfiguration, Shares, Netzwerk und Plugin-Konfiguration. |
| **Selbstbackup** | Das eigene `/config` von BombVault: seine Einstellungsdatenbank, die Zugangsdaten der Speicherorte, das libvirt-SSH-Schlüsselpaar, der Schlüssel und die festgehaltenen Host-Schlüssel der ZFS-Server und die Zertifikate, als Snapshot mit SQLite `VACUUM INTO`, damit die Datenbank nie mitten in einem Schreibvorgang erfasst wird. |
| **Ordner** | Jeder Ordner auf dem Server als Ordner-Set, etwa ein Share, deine Dokumente oder eine Fotobibliothek, mit optionalen Ausschlussmustern. Ordner-Sets haben Zeitpläne, Aufbewahrung, Off-site-Kopien, Prüfungen und Wiederherstellungsproben wie alles andere auch. |
| **ZFS-Datasets** | Ein Dataset und jedes Dataset darunter, gelesen aus einem ZFS-Snapshot, sodass alle vom selben Augenblick stammen, und mit restic wie ein Ordner gespeichert. Ein ZFS-Eintrag kann außerdem eine blockgenaue Replik auf einem zweiten ZFS-Server halten. Siehe [ZFS-Datasets](zfs-datasets.md). |

Jede Sicherung läuft über restic, sie ist also inkrementell und dedupliziert, und selbst große VM-Disks blähen das Repository nicht auf. BombVault sichert seinen eigenen Container nie und hält ihn deshalb auch nie an. Ein Lauf lässt sich gefahrlos stoppen, denn restic schreibt seinen Snapshot zuletzt, ein abgebrochener Lauf hinterlässt also nicht referenzierte Daten und keinen Snapshot. Eine Sperre, die ein Neustart mitten in einem Lauf zurückgelassen hat, wird aufgehoben und der Lauf noch einmal versucht, und zwar erst, nachdem die Sperre zehn Minuten lang nicht aufgefrischt wurde.

## Die Liste {#list}

Kacheln über der Liste wählen eine Art, etwa **Container** oder **ZFS**, und jede Kachel zählt ihre Einträge. Eine Kachel wählt aus, sie blendet den Rest der Seite nicht aus.

Jede Zeile zeigt die letzten vierzehn Läufe des Eintrags als kleine Quadrate und einen Status, etwa **Geschützt**, **Nicht geschützt**, **Zurückgestellt** oder **Wird repliziert**. Alles Weitere dazu steht in der Info-Blase neben seinem Namen, und solange der Zeiger auf der Zeile ruht, steht dieser Text in der Zeile selbst. Eine Markierung neben dem Namen eines Containers sagt, dass er seit seiner letzten Sicherung mit einem anderen Image, anderen Ports, Variablen oder Volumes neu angelegt wurde. Sie verschwindet mit der nächsten Sicherung.

Drei Knöpfe über der Liste grenzen sie ein:

- **Filter**: nach **Zeitplan** (**Alle**, **Geplant**, **Pausiert**), nach **Sicherung** (**Gesichert**, **Nie gesichert**) und, bei **Container und VMs**, **Installiert** oder **Nicht installiert**.
- **Sortieren**: nach Name, Status, IP, letzter Sicherung, Größe oder Zeitplan. Jedes Feld außer IP sortiert in beide Richtungen, etwa **Größe, größte zuerst** und **Größe, kleinste zuerst**.
- **Suchen**: nach Name, Image oder IP.

Einträge, die nicht mehr installiert sind, aber noch Sicherungen haben, stehen getrennt unter **Nicht installiert (nur Sicherungen)**. Stelle sie wieder her oder lösche ihre Sicherungen, um den Platz freizugeben.

Die Hauptaktion der Seite schwebt unten rechts: **Jetzt alles sichern** startet einen Lauf über alle eingeschalteten Bereiche. **Mehr** enthält den Rest:

- **Auswählen** startet eine Auswahl, siehe unten.
- **Sicherungen entdecken** liest, was die Speicherorte enthalten, und baut daraus Einträge neu auf, zum Beispiel nach einer frischen Installation. Ein Container ohne eigene Daten hat keinen Snapshot und kommt aus der Definition zurück, die neben dem Repository gespiegelt liegt. Off-site-Kopien wiederhergestellter Bereiche warten dann, bis du ihre Sicherungen als eigene dieses Servers übernimmst, siehe [Nach einem Neuaufbau](offsite-recovery.md#after-a-rebuild).
- **ZFS-Verbindung zu diesem Server** öffnet den SSH-Test des ZFS-Bereichs, siehe [ZFS-Datasets](zfs-datasets.md#requirements).
- **Alle jetzt replizieren** sendet bei gewählter ZFS-Kachel die Replik jedes ZFS-Eintrags.

![Dieselbe Liste auf einem Handy.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![Dieselbe Liste auf einem Handy.](assets/screenshots/backups/list-phone-light.png#only-light)

## Mehrere Einträge auswählen {#select}

Wähle Zeilen so aus, wie du Dateien in einem Dateimanager auswählst: Strg+Klick, Umschalt+Klick, Strg+A oder auf dem Handy ein langer Druck. Eine gewählte Zeile zeigt statt ihres Symbols einen Haken, und eine Leiste über der Liste zählt sie und bietet **Auswahl sichern**, **Zeitplan pausieren** und **Zeitplan fortsetzen**, **Alle auswählen** und **Auswahl aufheben**. Jede Zeile hat außerdem ein Rechtsklick-Menü mit eigenen Aktionen.

Eine Auswahl wird auf dem Server gesichert, ein Eintrag nach dem anderen, sie läuft also weiter, wenn du den Tab schließt. Um mehrere Einträge zusammen wiederherzustellen, nutze die [Seite Wiederherstellung](restore.md#several).

![Zwei gewählte Container mit der Leiste über der Liste.](assets/screenshots/backups/bulk-dark.png#only-dark)
![Zwei gewählte Container mit der Leiste über der Liste.](assets/screenshots/backups/bulk-light.png#only-light)

## Einen Ordner oder ein ZFS-Dataset hinzufügen {#add}

Container, VMs und der Flash erscheinen von selbst, sobald ihr Bereich unter **Einstellungen, Allgemein** eingeschaltet ist. Ordner und ZFS-Datasets wählst du selbst aus, denn BombVault kann nicht erraten, welche du meinst. **Hinzufügen** öffnet **Was soll BombVault zusätzlich sichern?**:

- **Einen Ordner** listet die Shares auf, die BombVault gefunden hat, und nimmt jeden anderen Ordner unterhalb von Host Data. Gib dem Ordner-Set einen Namen, etwa `Documents`. Der Name steht nach der ersten Sicherung fest, weil die Snapshots darunter gespeichert werden.
- **Ein ZFS-Dataset** listet die Datasets dieses Servers auf. Wähle das oberste von dem, was gesichert werden soll, siehe [Einträge und untergeordnete Datasets](zfs-datasets.md#items-and-children).
- **Systemkonfiguration** erscheint auf einem Host ohne Unraid, dessen Host Data bis `/etc` reicht. Es ist ein vorbereitetes `/etc`-Ordner-Set, das du vor dem Speichern prüfst.

## Einen Eintrag auslassen {#dont-back-up}

Einen Eintrag, den du absichtlich auslässt, etwa einen Testcontainer, kannst du mit **Nicht sichern** markieren. Er zählt dann nicht mehr als nicht geschützt, und **Nicht mehr ausnehmen** holt ihn zurück. **Zeitplan pausieren** ist der sanftere Weg: Der Eintrag verlässt den Zeitplan und das Gesamt-Backup, trägt ein graues Abzeichen **Zeitplan pausiert**, und **Jetzt sichern** funktioniert weiter. Es setzt denselben Schalter wie **Im Zeitplan** auf dem Tab Einstellungen des Eintrags.

## Die Seite eines Eintrags {#entry-page}

Ein Klick auf eine Zeile öffnet die Seite des Eintrags. Der Kopf nennt den Eintrag und seinen Zustand in einer Zeile, etwa **Zuletzt gesichert vor 3 Stunden** oder **Zurückgestellt, nächster Versuch um 03:15**, und enthält seine Aktionen: **Jetzt sichern**, **Wiederherstellen** und unter **Mehr** die seltenen, etwa **Neueste Sicherung exportieren**, **Container exportieren** oder **VM exportieren**, **Mit früherem Eintrag verknüpfen**, **Alle Sicherungen löschen** und **Eintrag entfernen**. Ein Zurück-Knopf links vom Symbol führt zur Liste. Während eine Sicherung läuft, steht **Sicherung abbrechen** neben ihrem Fortschritt, und ein abgebrochener Lauf wird als abgebrochen vermerkt, nicht als fehlgeschlagen.

Die Seite hat drei Tabs: **Sicherungen**, **Einstellungen** und **Prüfung**.

### Tab Sicherungen {#backups-tab}

Dieser Tab listet die Snapshots des Eintrags auf, neueste zuerst, mit einem Balken pro Tag. Eine Sicherung, die an mehreren Orten liegt, erscheint einmal, markiert mit jedem Speicherort, der sie hält. Ein Klick auf einen Snapshot stellt ihn wieder her, und ein Klick auf eine Stelle in einem Snapshot stellt sie genau von dort wieder her.

- **Speicherorte** nennt jeden Ort mit Sicherungen dieses Eintrags und wie viele. Off-site-Orte werden nur gelesen, wenn du **Prüfen** drückst, weil das Auflisten von Cloud-Speicher eine Weile dauert. Ein Ort, auf den der Eintrag nicht mehr sichert, steht als **Nur frühere Sicherungen** da und lässt sich weiter für Wiederherstellungen nutzen.
- Jeder Snapshot hat ein Menü: **Tag hinzufügen** oder **Tag ändern**, **Mit anderer Sicherung vergleichen**, das die Dateien auflistet, die dazugekommen, verschwunden oder geändert sind, **Exportieren** als tar, zip oder als eigenes restic-Repository, und Löschen an nur einem Ort.
- **Datenbank-Dumps** eines Containers stehen in einer eigenen Liste, mit Download, Speichern in einen Ordner und Import.

Wie viele Snapshots bleiben, legt der Speicherort fest, siehe [Wie lange aufheben](offsite-recovery.md#keep).

![Der Tab Sicherungen eines Containers: ein Balken pro Tag und die Speicherorte, die jeden Snapshot halten.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![Der Tab Sicherungen eines Containers: ein Balken pro Tag und die Speicherorte, die jeden Snapshot halten.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Tab Einstellungen {#settings-tab}

Jeder Eintrag beginnt mit **Zeitplan und Speicherort**:

- **Im Zeitplan** nimmt den Eintrag in die geplanten Läufe auf oder heraus.
- **Eigener Plan** gibt diesem einen Eintrag einen eigenen Plan, siehe [Ein eigener Plan für einen Eintrag](schedules.md#own-plan). Ohne einen läuft er nach dem Plan seines Bereichs oder im Gesamt-Backup.
- Eine Reihe von Knöpfen, **Lokal** und je einer pro Off-site-Speicherort, sagt, wohin die Sicherungen gehen. **Folgt der Vorgabe** heißt, der Eintrag geht dorthin, wohin sein Bereich geht, und ein Klick darauf öffnet **Speicherorte je Bereich**. Siehe [Zuordnung pro Eintrag](offsite-recovery.md#placement).
- **Repository** nennt, wohin die Sicherungen geschrieben werden (**Gespeichert auf**). Dieser Ort steht ab der ersten Sicherung fest, die Kopien können sich jederzeit ändern.

Die Karten darunter hängen von der Art des Eintrags ab:

| Art | Karten |
|---|---|
| Container | **Gesicherte Ordner** (welche gemappten Ordner gesichert werden, mit **Cache-Ordner überspringen (CACHEDIR.TAG)**), **Ausschlussmuster**, **Andere Container stoppen**, **Befehle vorher und nachher**, **Datenbank**, **Wenn die App beschäftigt ist**, **Nach erfolgreicher Sicherung aktualisieren**, **Snapshot statt Stopp**, **Frühere Namen** |
| VM | **VM** mit **Methode** (**Graceful (Herunterfahren)** oder **Live-Snapshot**) und **Nur geänderte Blöcke**, siehe die [Anleitung zur VM-Sicherung](vm-backup-ssh-setup.md#per-vm-method) |
| Ordner-Set | **Ordner** und **Ausschlussmuster** |
| Flash | **Flash-ZIP-Export**, das jede Flash-Sicherung als einfache `.zip` in einen Ordner deiner Wahl schreibt, in einem Schritt, sodass ein Sync-Werkzeug nie eine halbe Datei sieht. Eine Flash-Sicherung, die weniger enthält, als sie sollte, lässt das letzte ZIP an seinem Platz, und der Lauf des Exports sagt das. |
| ZFS-Dataset | die untergeordneten Datasets, die zu stoppenden Container und die Befehle rund um den Snapshot, **Replik** und **Snapshot-Zugriff testen**, siehe [ZFS-Datasets](zfs-datasets.md) |

Einstellungen speichern sich selbst, während du sie änderst.

![Der Tab Einstellungen eines Containers, mit Eigener Plan in der ersten Karte.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![Der Tab Einstellungen eines Containers, mit Eigener Plan in der ersten Karte.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Tab Prüfung {#verify-tab}

- **Prüfungen** zeigt die Wiederherstellungsprüfung, die auf die erste Sicherung folgt. **Wiederherstellung prüfen** führt sie noch einmal auf der neuesten Sicherung aus: BombVault stellt eine Stichprobe von bis zu 100 Dateien und 256 MiB in einen temporären Ordner wieder her und lässt restic jede Datei gegen ihre Hashes zurücklesen.
- **Start-Test** stellt bei einem Container die neueste Sicherung in einer isolierten Kopie wieder her und startet sie: ein Name, der mit `bombvault-test-` beginnt, ein eigenes internes Docker-Netzwerk ohne veröffentlichte Ports und ohne Weg ins LAN, 1 CPU und 2 GiB Speicher. Der Test gilt als bestanden, wenn die Kopie gesund meldet, auf ihrem ersten freigegebenen Port antwortet oder, wenn beides nicht zutrifft, weiterläuft. Das Original wird nie angefasst, und die Kopie, ihr Netzwerk und ihre Daten werden danach entfernt, auch nach einem Neustart mitten im Test. Container im Host-Netzwerk, privilegierte, solche mit Geräten und solche, die einen anderen Container brauchen, werden als nicht testbar angezeigt. **Start-Test** unter **Einstellungen, Prüfung, Automatische Prüfungen** testet pro Lauf einen Container, den am längsten nicht getesteten zuerst.
- **Anomalien** legt die eigene Empfindlichkeit und Benachrichtigungsstufe dieses Eintrags fest und zeigt seine offenen Funde.
- **Größe nach Ordner** zeigt, was in der neuesten Sicherung den Platz belegt und wie viel davon die letzte Sicherung gebracht hat, eine Ebene nach der anderen.
- **Letzte Läufe** listet die letzten Läufe mit Start, Ende und Dauer. Ein Klick zeigt die Details.

![Der Tab Prüfung eines Containers mit geöffneter Größe nach Ordner.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![Der Tab Prüfung eines Containers mit geöffneter Größe nach Ordner.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Ausschlussmuster {#excludes}

Die Karte **Ausschlussmuster** nimmt ein Muster pro Zeile. Gib Pfade so ein, wie du sie im Container siehst, etwa `/config/Library/Application Support/Plex Media Server/Cache`, oder einen bloßen Namen wie `.git`, der in jeder Tiefe passt. Bei einem Ordner-Set gibst du einen Pfad so ein, wie der Host ihn kennt, etwa `/mnt/user/data/cache`. Eine Vorschau unter dem Feld zeigt, wozu jede Zeile aufgelöst wird, und warnt, wenn eine Zeile nichts trifft oder alles ausschließen würde. Eine Zeile mit `/` oder `*` oder eine, die einen ganzen gesicherten Ordner nennt, wird ignoriert, ein Ausschluss kann also nie eine Sicherung leeren. Um einen ganzen Ordner auszulassen, nimm stattdessen den Haken unter **Gesicherte Ordner** heraus.

Für bekannte Images wie Plex, Jellyfin, Sonarr, Radarr, Immich und Nextcloud bietet die Karte die Ordner an, die die App von selbst wieder füllt, etwa Caches, Logs und Vorschaubilder. Ausgeschlossen wird nichts, bis du **Auswahl ausschließen** drückst.

Dieselbe Karte trägt Warnungen, die ein Ordnerscan nicht sehen kann. Immich zum Beispiel hält seine Alben, Gesichter und Datumsangaben in einer PostgreSQL-Datenbank in einem separaten Container, daher stellt eine Dateisicherung allein des Immich-Containers die Bilder ohne all das wieder her, und die Wiederherstellung sieht aus, als hätte sie geklappt. Sichere den Datenbank-Container auch.

## Datenbank-Dumps {#database-dumps}

PostgreSQL-, MySQL- und MariaDB-Container, die BombVault erkennt, werden vor jeder Sicherung vom laufenden Server in einen eigenen Wiederherstellungspunkt neben den Dateien gedumpt. Das umfasst die offiziellen Images, PostGIS, TimescaleDB, pgvector, pgautoupgrade, die Datenbank-Images von Immich, die MariaDB-Images von linuxserver, yobasystems und jc21 sowie mysql-server von Oracle. Ein Container, der nur wie eine Datenbank aussieht, bekommt dieselbe Option, ausgeschaltet, bis du sie wählst.

Der Dump läuft direkt ins Repository und wird nie auf eine Platte geschrieben. Die Zugangsdaten kommen aus den eigenen Variablen des Containers, `*_FILE`-Secrets eingeschlossen, und verlassen ihn nie. Die Karte **Datenbank** auf dem Tab Einstellungen des Containers sagt, ob der Datenordner im gestoppten Zustand gesichert, im laufenden Zustand kopiert oder nicht gesichert wird, und schaltet den Dump für diesen Container ein oder aus. Das Label `bombvault.dbdump=false` tut dasselbe und hat Vorrang vor der Karte. **Automatische Datenbank-Dumps** unter **Einstellungen, Allgemein** schaltet es für alle um.

Ein fehlgeschlagener Dump lässt die Sicherung nicht fehlschlagen. Er erscheint als eigener fehlgeschlagener Lauf mit dem Grund und schickt eine Benachrichtigung. Dumps werden nie von allein zurückgespielt: Lade einen herunter, unverändert oder komprimiert, speichere ihn in einen Ordner oder importiere ihn mit einem Klick auf dem Tab **Sicherungen** in eine frisch gestartete Datenbank. Wie du einen Dump ohne BombVault zurückholst, steht unter [Einen Datenbank-Dump zurückholen](offsite-recovery.md#database-dumps). Wenn einer fehlschlägt, siehe [Ein Datenbank-Dump ist fehlgeschlagen](troubleshooting.md#a-database-dump-failed).

## Befehle und andere Container {#commands}

- **Befehle vorher und nachher** laufen mit `sh -c` im Container, zum Beispiel um einen Cache auf die Platte zu schreiben. Ein fehlschlagender Befehl vor der Sicherung bricht sie ab. Ein Befehl vor der Sicherung, der mit Code 75 endet, stellt stattdessen eine geplante Sicherung zurück, siehe unten.
- **Andere Container stoppen** nennt Container, die angehalten werden, während dieser gesichert wird, etwa seine Datenbank. Danach starten sie in der Reihenfolge ihres Compose-`depends_on` wieder, und standardmäßig muss jeder gesund melden, bevor der nächste startet. Die Wartezeit und ihr Zeitlimit stehen unter **Neustart nach der Sicherung** in der Karte **Reihenfolge** unter **Einstellungen, Zeitplan**.
- Ein Container, der den Netzwerk-, PID- oder IPC-Namespace eines angehaltenen Containers teilt, über `--net=container:` oder `network_mode: service:` in Compose, stoppt und startet mit ihm, auch wenn er nicht auf der Liste steht, und er startet nach dem Container, dem er sich anschließt. Ein neu gestarteter Container bekommt ein neues Netzwerk, und einer, der im alten Netzwerk weiterliefe, hätte nur noch sein Loopback. Die Liste selbst bleibt, wie du sie eingestellt hast. Ließ sich so ein Container nicht stoppen, sagt der Lauf, dass er bis zu seinem Neustart kein Netzwerk hat. Dasselbe gilt für den Container von BombVault selbst, der sich nie selbst stoppt und nicht auf die Liste gesetzt werden kann.
- Ein Container, der danach nicht wieder läuft, wird im Lauf und in einer Benachrichtigung genannt, auch wenn die Sicherung selbst fehlgeschlagen ist.

## Wenn die App beschäftigt ist {#app-busy}

Eine Sicherung hält ihren Container an, daher kann eine geplante warten, solange jemand die App nutzt, und es von selbst später wieder versuchen. Die Karte **Wenn die App beschäftigt ist** auf dem Tab Einstellungen eines Containers richtet das ein:

- **Aktivitätsprüfung** fragt vor einer geplanten Sicherung bei Plex, Jellyfin oder Emby nach, ob jemand zuschaut. Plex wird mit dem Token aus seiner eigenen `Preferences.xml` gefragt, du musst also nichts eingeben. Jellyfin und Emby brauchen einen **API-Schlüssel**, den du im Dashboard des Servers unter API-Schlüssel erstellst.
- **Pausierte Sitzungen ignorieren nach (Min.)** lässt einen Film, der über Nacht pausiert ist, nicht mehr mitzählen.
- **Sichern, wenn der Server nicht antwortet** legt fest, was passiert, wenn der Server nicht gefragt werden kann: An, dann läuft die Sicherung und das Protokoll sagt es. Aus, dann wartet sie, als würde jemand zuschauen.
- **Warten, solange der Container Daten sendet** ist für Apps ohne so eine Prüfung gedacht, etwa einen Spieleserver oder einen Downloader. Die Sicherung wartet, solange der Container bei irgendeiner Messung in den letzten **Ruhig für (Min.)** Minuten mehr als **Beschäftigt ab (KiB/s)** gesendet hat.
- **Erneut versuchen alle (Min.)**, **Nur in einem Zeitfenster versuchen** mit **Versuchen zwischen** und **Höchstens warten (Stunden)** legen fest, wie die Versuche ablaufen.
- **Am Ende der Wartezeit** gibt entweder auf, wodurch die Sicherung unter **Braucht dich** mit **Erneut versuchen** erscheint, oder **Trotzdem sichern**.

Eine wartende Sicherung wird als **Zurückgestellt** vermerkt, in Gelb, und der Rest des Laufs geht weiter. Die Übersicht, die Liste und der Eintrag zeigen sie mit dem nächsten Versuch, und die Meldung über Überfälliges und der Schutzstatus warten bis zu ihrer Frist darauf. Du bekommst eine Benachrichtigung, wenn sie zurückgestellt wird, und eine, wenn sie endet, nicht eine pro Versuch. Ein Versuch wartet, solange eine andere Sicherung läuft, und eine Wartezeit läuft nach einem Neustart weiter. Eine Sicherung, die du selbst startest, läuft immer sofort, ob mit **Jetzt sichern**, **Erneut versuchen**, einer Auswahl, der API oder MCP, und eine, die durchläuft, beendet eine anstehende Wartezeit.

Der Tab Einstellungen des Containers hat außerdem einen älteren, einfacheren Weg: **Warten, bis die App ruht** mit **Höchstens warten (Stunden)**. Die Sicherung startet, sobald die App ruht, was die Karte **Ruhe vor der Sicherung** unter **Einstellungen, Zeitplan** festlegt. Die Mitglieder eines Compose-Stacks, die im selben Lauf fällig sind, warten gemeinsam, und die wartende Sicherung hält keine Sperre, die anderen Container kommen also voran.

![Die Karte Wenn die App beschäftigt ist eines Plex-Containers mit eingerichteter Aktivitätsprüfung.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![Die Karte Wenn die App beschäftigt ist eines Plex-Containers mit eingerichteter Aktivitätsprüfung.](assets/screenshots/backups/app-busy-light.png#only-light)

### Beispiel: Plex wartet, solange jemand schaut {#example-plex}

Das Gesamt-Backup läuft jede Nacht um 03:00. Niemand soll einen Film abbrechen sehen, weil Plex für seine Sicherung heruntergefahren wurde, aber die Sicherung soll trotzdem vor dem Morgen stattfinden.

1. Öffne **Sicherungen**, dann den Eintrag `plex`, dann seinen Tab **Einstellungen**.
2. Stelle in **Wenn die App beschäftigt ist** die **Aktivitätsprüfung** auf Plex. Du musst nichts eintippen.
3. Setze **Pausierte Sitzungen ignorieren nach (Min.)** auf `30`, damit ein um Mitternacht pausierter Film um halb eins nicht mehr zählt.
4. Lass **Sichern, wenn der Server nicht antwortet** an.
5. Setze **Erneut versuchen alle (Min.)** auf `15`.
6. Schalte **Nur in einem Zeitfenster versuchen** ein und setze **Versuchen zwischen** auf `22:00` und `06:00`.
7. Setze **Höchstens warten (Stunden)** auf `3` und **Am Ende der Wartezeit** auf **Trotzdem sichern**.

Was in einer Nacht passiert, in der um 03:00 noch jemand schaut: BombVault fragt Plex, findet eine Sitzung, die läuft, und vermerkt die Sicherung als **Zurückgestellt** mit dem nächsten Versuch um 03:15. Die anderen Container des Laufs werden wie gewohnt gesichert. Wenn der Film um 04:10 endet, findet der Versuch um 04:15 Plex untätig und sichert es, und die Wartezeit endet mit einer Benachrichtigung. Würde der Film um 06:00 immer noch laufen, erreichte die Wartezeit ihr Ende und Plex würde trotzdem gesichert, seine Sicherungen liegen also nie mehr als drei Stunden zurück.

Für eine eigene Prüfung lass den **Befehl vor der Sicherung** mit Code 75 enden, solange die App beschäftigt ist, zum Beispiel `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Weitere Symptome und Lösungen stehen unter [Fehlersuche](troubleshooting.md#deferred-backups).

## Einfache Exporte {#exports}

restic bleibt die Engine, aber ein Export gibt dir eine Kopie, die weder BombVault noch restic braucht.

- **Container exportieren** und **VM exportieren** unter **Mehr** auf der Seite eines Eintrags schreiben eine Kopie des jetzigen Stands neben das Repository: die Sicherungsordner des Containers plus sein Unraid-Template, oder die Disk-Images der VM plus ihre Definition, wiederherstellbar mit `virsh define`.
- **Exportieren** an einem Snapshot schreibt eine gespeicherte Sicherung in den Export-Ordner auf dem Server, als tar, zip oder als eigenes restic-Repository, und kann einen Snapshot oder alle nehmen. Eine Flash-Sicherung und ein Selbstbackup enthalten Schlüssel, darum braucht ihr Export ein Login-Passwort und eine Anmeldung an der Instanz.

Exporte sind standardmäßig einfache Dateien. **Exporte mit age verschlüsseln** unter **Einstellungen, System** versiegelt sie, das Flash-ZIP eingeschlossen, für die Empfänger, die du aufführst, jeweils ein öffentlicher age-Schlüssel oder ein öffentlicher SSH-Schlüssel. Du entschlüsselst sie später außerhalb der Box mit dem passenden privaten Schlüssel. Ist die Verschlüsselung an und es gibt keinen gültigen Empfänger, schlägt ein Export mit einer klaren Fehlermeldung fehl, statt je einfache Dateien zu schreiben.

## Was ein Lauf dir sagt {#run-notes}

- **Zeiten.** Jeder Lauf nennt Start, Ende und Dauer, und jeder Eintrag hat seine eigenen **Letzten Läufe**.
- **Warum eine Sicherung langsam war.** Während eine Sicherung läuft, beobachtet BombVault, wie ausgelastet CPU, Platten und Netzwerk sind. Dauert eine Sicherung deutlich länger als üblich und war eine Sache klar am Limit, sagt der Lauf es, zum Beispiel „Die Zielplatte disk1 war zu 98 % ausgelastet“. Sonst sagt er nichts.
- **Fortschritt von Prüfungen und Bereinigungen.** Das Aktivitätsprotokoll und die Karte **Sicherungen prüfen** zeigen, wie weit restic gezählt hat, etwa *12 von 47 Packs*, mit der verbleibenden Zeit in diesem Schritt, sobald genug da ist, um sie zu schätzen.
- **Dateien, die nicht gelesen werden konnten.** Eine Sicherung, die durchgelaufen ist, aber etwas ausgelassen hat, färbt ihre Zeile orange und nennt, was fehlt.
