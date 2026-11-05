# Wiederherstellung

Jede Wiederherstellung beginnt auf der Seite **Wiederherstellung**: Wähle einen Bereich, suche, was du zurückhaben willst, und wähle eine Sicherung. Ein Fenster stellt jede Art von Eintrag wieder her und fragt in Schritten. Standardmäßig ändert sich am Original nichts, denn die Sicherung kommt daneben zurück.

![Die Seite Wiederherstellung mit den Bereichen als Kacheln, der Suche und der Karte Stacks.](assets/screenshots/restore/page-dark.png#only-dark)
![Die Seite Wiederherstellung mit den Bereichen als Kacheln, der Suche und der Karte Stacks.](assets/screenshots/restore/page-light.png#only-light)

## Die Seite Wiederherstellung {#page}

- **Bereiche** listet die Bereiche als Kacheln auf, jede mit der Zahl der Einträge, die Snapshots haben.
- **Was willst du zurückholen?** durchsucht die Einträge des gewählten Bereichs. Jede Zeile zeigt ihren neuesten Snapshot, und **Eintrag öffnen** führt zur Seite des Eintrags.
- **Stacks** listet die Compose-Projekte mit mindestens zwei gesicherten Containern auf, etwa `immich` mit seiner Datenbank und Redis. Die Suche findet einen Stack über seinen Namen oder über einen seiner Container.
- **Läuft gerade** zeigt eine Wiederherstellung, solange sie läuft, auch nachdem du ihr Fenster geschlossen hast.
- **Notfall** startet den Notfall-Assistenten für einen Server, der weg ist oder neu aufgesetzt wurde, siehe [Notfall-Assistent](offsite-recovery.md#emergency).
- **Andere Quellen** enthält **Aus einem alten Sicherungsordner** für die Sicherungen eines anderen BombVault und **Aus dem Appdata.Backup-Plugin übernehmen**.

![Die Seite Wiederherstellung auf einem Handy.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![Die Seite Wiederherstellung auf einem Handy.](assets/screenshots/restore/page-phone-light.png#only-light)

## Das Wiederherstellungsfenster {#window}

Ein Klick auf einen Eintrag öffnet **Wiederherstellung** mit seinem Namen. Es fragt in Schritten, und jeder Schritt lässt sich ändern, bevor du startest:

1. **Sicherung**: Der empfohlene Snapshot ist vorgewählt. Das ist der neueste oder, nach einer offenen Anomalie, der letzte davor. Die Sicherung, die vor einer früheren Wiederherstellung gemacht wurde, hält den Stand, den diese Wiederherstellung ersetzt hat, darum wird sie übergangen, außer es gibt keine andere. Ältere stehen in der Liste, und **Vergleichen** zeigt, was zwei davon unterscheidet.
2. **Quelle**: woher die Dateien kommen, wenn ein Snapshot an mehreren Orten liegt. **Lokal** ist am schnellsten.
3. **Was**: **Alles** oder **Einzelne Dateien**, wo du die Dateien und Ordner abhakst, die du willst.
4. **Wohin**: das Ziel. Die Auswahl hängt von der Art des Eintrags ab:

    | Ziel | Was es macht |
    |---|---|
    | **Neben das Original** | Die Vorgabe. Ein Container kommt als zweiter Container namens `<name>-restored` mit eigenen Ports zurück, die Disks und das NVRAM einer VM landen in einem Ordner namens `<folder>-restored`, und ein Ordner-Set kommt als `<path>-restored` zurück. Am Original ändert sich nichts. |
    | **Original ersetzen** | Überschreibt, was jetzt da ist. Mit **Vorher den heutigen Zustand sichern**, das standardmäßig an ist, sichert BombVault zuerst den heutigen Stand, damit du zurückgehen kannst. Schlägt diese Sicherung fehl, startet die Wiederherstellung nicht. Das klappt auch bei einem Container oder einer VM, die nicht mehr auf dem Server ist: Die Ordner, die die Wiederherstellung überschreibt, werden für sich gesichert. |
    | **In einen Ordner** | Nur die Dateien, zum Durchsehen. |
    | **Als ZIP herunterladen** | Der Flash, für einen neuen Stick. BombVault schreibt nie in den laufenden Flash. |
    | **In ein neues Dataset** und **In das Dataset zurückschreiben** | ZFS-Datasets, siehe [ZFS-Datasets](zfs-datasets.md#restore). |

5. **Was passiert** sagt in Worten, was die Wiederherstellung tun wird, und eine Szene zeigt das gewählte Ziel.

Bevor **Wiederherstellen** freigegeben wird, prüft das Fenster, ob das Repository antwortet, ob der Schlüssel es öffnet, ob der Snapshot da ist und ob das Ziel Platz hat. Solange eine Prüfung fehlschlägt, bleibt der Knopf gesperrt, und das (i) darin sagt, welche.

Das Fenster zeigt außerdem, was die Wiederherstellung gegenüber dem ändert, was jetzt da ist: neue, ersetzte und unveränderte Dateien, mit der Liste auf Wunsch, und die Dateien am Ziel, die nicht in der Sicherung sind und bleiben, wo sie sind. restic ermittelt das als Probelauf aus Größe und Änderungszeit, ohne die Dateien zu lesen, und ein sehr großer Baum bricht nach 30 Sekunden ab und sagt das. Bei Containern und VMs vergleicht es die Einstellungen, die die Wiederherstellung neu anlegt, mit den laufenden: Image und Tag, Ports, Variablennamen und Volumes, oder Speicher, vCPUs, Disks und Netzwerk.

Manches kann eine Wiederherstellung stoppen, bevor sie etwas ändert. Die statische IP und die veröffentlichten Ports eines Containers müssen frei sein, und sein Netzwerk muss existieren. Auf Unraid lehnt jede Wiederherstellung an Ort und Stelle einen Ordner ab, der nicht auf einem eingehängten Pool oder Share liegt, denn die Dateien landeten im Arbeitsspeicher und wären nach einem Neustart weg. Eine Wiederherstellung an Ort und Stelle nennt jeden anderen Container, dessen Bind-Mount in einen Ordner hineinreicht, in den sie schreibt, zum Beispiel „Dieser Pfad wird auch von nextcloud-db genutzt“, aber das warnt nur.

Einige Optionen erscheinen dort, wo sie passen: **Danach gestoppt lassen**, um einen Container oder eine VM anzusehen, bevor sie wieder läuft, und **Ohne GPU und Laufzeit wiederherstellen** auf einem Host, dem die GPU oder Laufzeit fehlt, mit der die Sicherung gemacht wurde.

Die Wiederherstellung läuft im Hintergrund. Du kannst das Fenster schließen, und der Fortschritt zeigt sich auch unter **Läuft gerade**. Eine lange Wiederherstellung zeigt eine Prozentzahl und lässt sich abbrechen, und eine abgebrochene Wiederherstellung wird als abgebrochen vermerkt, nicht als fehlgeschlagen. Eine Wiederherstellung, die abgebrochen wird, während der Container gerade stoppt, oder eine, die den Container nicht entfernen kann, hat noch nichts wiederhergestellt, also startet der Container wieder. Ein Container, der seine Volumes mit `--volumes-from` von einem anderen nimmt, behält sie, und die Wiederherstellung beginnt nicht, solange dieser andere Container nicht existiert.

![Das Wiederherstellungsfenster im Schritt Was, mit abgehakten einzelnen Dateien.](assets/screenshots/restore/window-what-dark.png#only-dark)
![Das Wiederherstellungsfenster im Schritt Was, mit abgehakten einzelnen Dateien.](assets/screenshots/restore/window-what-light.png#only-light)

### Wohin eine Wiederherstellung in einen Ordner geht {#folders}

**Ordner für Wiederherstellungen** unter **Einstellungen, Wiederherstellung** legt den **Standardordner für Wiederherstellungen** fest, auf Wunsch mit einem **Unterordner pro Bereich**, und eine **Ausnahme** für einen Bereich. Gewählte Dateien landen direkt im Zielordner: Eine Datei kommt als genau diese Datei zurück, ein Ordner als der Ordner mit seinem Inhalt, und bei Auswahlen aus verschiedenen Ordnern bleibt nur der Pfad unterhalb des gemeinsamen Ordners erhalten. BombVault lehnt ein Ziel ab, das nicht auf eingehängtem Speicher liegt, denn auf Unraid liegt so ein Ordner im Arbeitsspeicher und ist nach einem Neustart weg, und es stellt nie in seinen eigenen Datenordner wieder her.

## Beispiel: eine Datei zurückholen {#example-single-file}

Jemand hat gestern Nacht die `config.php` von Nextcloud bearbeitet, und Nextcloud startet nicht mehr. Die nächtliche Sicherung von 03:00 hat die gute Datei noch. Du willst diese eine Datei, nicht den ganzen Container.

1. Öffne **Wiederherstellung**, wähle die Kachel **Container** und tippe `nextcloud` in **Was willst du zurückholen?**.
2. Klicke auf `nextcloud`. Das Fenster öffnet sich mit der Sicherung von 03:00 als vorgewählter, empfohlener.
3. Lass **Quelle** auf **Lokal**.
4. Wähle unter **Was** **Einzelne Dateien**. Öffne die Ordner bis `config` und hake `config.php` ab.
5. Wähle unter **Wohin** **In einen Ordner** und einen Ordner, zum Beispiel `user/restore`, das auf dem Host `/mnt/user/restore` ist.
6. Lies **Was passiert**: Die Datei landet im Zielordner, und am Container ändert sich nichts. Drücke **Wiederherstellen**.

Das Fenster endet mit **Fertig: Die Dateien liegen in /mnt/user/restore**. Du findest die Datei als `/mnt/user/restore/config.php`, ohne die Ordner darüber. Vergleiche sie mit der kaputten, kopiere sie zurück in den Appdata-Ordner und starte Nextcloud neu.

Um die Datei stattdessen direkt zurückzulegen, wähle in Schritt 5 **Original ersetzen**. Lass **Vorher den heutigen Zustand sichern** an, damit BombVault den kaputten Stand sichert, bevor es schreibt, tippe `nextcloud` zur Bestätigung und drücke **Wiederherstellen**. Nextcloud wird angehalten, während die gewählte Datei an ihren Platz zurückgeschrieben wird, und danach wieder gestartet, und es werden nur die Dateien geschrieben, die du abgehakt hast. Eine Wiederherstellung, die mittendrin fehlschlägt, lässt Nextcloud gestoppt, denn die Datei, die es liest, ist vielleicht halb geschrieben, und der Lauf sagt das.

![Das Wiederherstellungsfenster im Schritt Was passiert, mit gewähltem Original ersetzen.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![Das Wiederherstellungsfenster im Schritt Was passiert, mit gewähltem Original ersetzen.](assets/screenshots/restore/window-replace-light.png#only-light)

## Was zurückkommt, nach Art {#kinds}

**Container** werden aus ihrer gespeicherten Definition über die Docker-API neu angelegt, mit Entrypoint, Hostname, Stopp-Einstellungen, Volumes und Mounts, Ressourcenlimits, Log-Treiber, DNS-Einstellungen und alten Links. Ein Container kommt auf dem Image zurück, auf dem er bei der Sicherung lief, nicht auf einem neueren, das später gezogen wurde, und ein fehlendes Image wird zuerst gezogen, mit den Zugangsdaten aus **Container-Registries** unter **Einstellungen, Wiederherstellung**. Auch seine GPU oder Laufzeit (`--gpus`, `--runtime=nvidia`) kommt zurück. Auf einem Host ohne diesen Treiber oder diese Laufzeit sagt die Wiederherstellung das und bietet **Ohne GPU und Laufzeit wiederherstellen** an. Ein Link auf einen Container, der fehlt oder angehalten ist, während der wiederhergestellte startet, wird weggelassen, und der Laufverlauf sagt das. Eine Wiederherstellung, die fehlschlägt, nachdem der Container entfernt wurde, legt ihn zurück, angehalten.

Eine Sicherung kann Ordner enthalten, die beim Container nicht mehr gewählt sind, zum Beispiel eine ältere von vor dem Verschwinden eines Ordners. Ein Ordner, der weg oder leer ist, kommt zurück. Einer, in dem jetzt Dateien liegen, bleibt, wie er ist, weil du ihn vielleicht absichtlich abgewählt hast, und die Prüfung vor der Wiederherstellung und der Verlauf der Läufe nennen ihn. Hol ihn mit **Einzelne Dateien** oder **In einen Ordner** zurück.

**VMs** bekommen ihr XML über SSH neu definiert und ihre Disks und das UEFI-NVRAM wieder angehängt, auch nachdem die VM gelöscht wurde. Siehe [VM-Sicherungen](vm-backup-ssh-setup.md) für VMs mit Unraid-Snapshots und Disks auf zvols.

**Ordner-Sets** kommen mit Besitzer, Modi und Zeitstempeln zurück, wie restic sie speichert. Wo das Ziel nur den Besitzer verweigert, wie `/mnt/user` es tut, endet der Lauf mit einem Hinweis, der das sagt. BombVault vergleicht dann die Dateien mit der Sicherung, und eine Datei, die nicht angekommen ist, lässt die Wiederherstellung fehlschlagen.

**Der Flash** berührt nie das laufende `/boot`. Die Wiederherstellung ist ein Download namens `flash-<id>.zip`, bereit für den Unraid USB Creator. Das ZIP enthält den `APP_KEY`, darum braucht der Download ein Login-Passwort und eine Anmeldung an der Instanz. Das Fenster listet außerdem die Plugins in jeder Flash-Sicherung mit Version und Größe auf und kann eines einzeln in den laufenden Flash zurücklegen: seine `.plg`-Datei, seinen Ordner unter `config/plugins` und die Paketdateien, die die Sicherung enthält. Unraid installiert es beim nächsten Start oder sofort unter **Plugins, Install Plugin**.

**ZFS-Datasets** haben eigene Ziele und behalten ihre ZFS-Eigenschaften, siehe [Wiederherstellen](zfs-datasets.md#restore).

**Das Selbstbackup** kommt nur über den Notfall-Assistenten zurück, der BombVault neu startet, sodass die laufende Datenbank nie unter einem offenen Handle überschrieben wird.

**Datenbank-Dumps** werden nie von allein zurückgespielt. Eine Zeile unter **Datenbank-Dumps** lässt sich herunterladen, in einen Ordner speichern oder in eine frisch gestartete Datenbank importieren. Siehe [Datenbank-Dumps](backups.md#database-dumps).

## Mehrere Einträge auf einmal wiederherstellen {#several}

Wähle auf der Seite Wiederherstellung Einträge so, wie du Zeilen in der Liste Sicherungen wählst: Strg+Klick, Umschalt+Klick, Strg+A, langer Druck oder **Auswählen**. **Wiederherstellen** öffnet dann ein Fenster für alle.

- **Welche Sicherung**: **Empfohlen** nimmt für jeden Eintrag die Sicherung, die sein eigenes Fenster empfehlen würde. **Ein Zeitpunkt für alle** nimmt für jeden Eintrag seine neueste Sicherung ab einem **Zeitpunkt**, den du wählst, oder früher, zum Beispiel am Abend, bevor etwas schiefging.
- **Wohin**: **Neben das Original** oder **Original ersetzen**, mit **Vorher den heutigen Zustand sichern**.

Jeder Eintrag wird so geprüft wie eine einzelne Wiederherstellung, und dann laufen sie nacheinander. Jede Zeile zeigt, wie ihre eigene verlief: **Wartet**, **Läuft**, **Wiederhergestellt**, **Fehlgeschlagen** oder **Ausgelassen**. **Nach diesem anhalten** lässt die laufende Wiederherstellung fertig werden und startet keine weitere. Der Flash kommt als ZIP zum Herunterladen, und ZFS-Datasets und das Selbstbackup werden einzeln wiederhergestellt.

![Mehrere Einträge wiederherstellen: eine Zeile pro Eintrag mit ihrem Fortschritt.](assets/screenshots/restore/several-dark.png#only-dark)
![Mehrere Einträge wiederherstellen: eine Zeile pro Eintrag mit ihrem Fortschritt.](assets/screenshots/restore/several-light.png#only-light)

## Einen Stack wiederherstellen {#stacks}

Ein Klick auf einen Stack in der Karte **Stacks** öffnet **Stack wiederherstellen**. Es stellt jedes Mitglied gleichzeitig aus seiner neuesten Sicherung wieder her, und mit **Nach der Wiederherstellung in Abhängigkeitsreihenfolge starten** startet es sie in ihrer `depends_on`-Reihenfolge: zuerst Datenbanken und Caches, dann die App. Fehlt der Projektordner eines Mitglieds an dem Ort, den du gewählt hast, bietet das Fenster stattdessen den Stand von Lokal an.

BombVault prüft zuerst die Mitglieder und den Projektordner, und eine Wiederherstellung, die nicht klappen könnte, ändert nichts. Sonst stoppt es die laufenden Mitglieder, die Apps vor ihren Datenbanken, schreibt dann den Projektordner und stellt dann die Mitglieder wieder her. Kommt der Projektordner nicht zurück, werden die Mitglieder trotzdem wiederhergestellt, bleiben aber gestoppt, weil der Ordner vielleicht halb geschrieben ist und dort meist die Bind-Mounts eines Mitglieds liegen. Der Lauf jedes Mitglieds sagt das.

Eine Datenbank, die nur Datenbank-Dumps und keine Dateisicherung hat und deren Container und benanntes Volume weg sind, kommt ohne ihre Daten zurück. Die Prüfung sagt das vor der Wiederherstellung. Sie bleibt gestoppt, und ebenso die Container, die von ihr abhängen, damit keine App ihre Einrichtung auf einer leeren Datenbank laufen lässt. Starte sie, importiere einen Dump und starte die anderen. Siehe [Datenbank-Dumps](backups.md#database-dumps).

## Andere Quellen {#other-sources}

- **Aus einem alten Sicherungsordner** öffnet das Repository eines anderen BombVault, das nicht mehr läuft, auf einer Platte oder an einem Off-site-Speicherort, mit seinem Recovery-Kit oder dem eingetippten `APP_KEY`. Die Sitzung ist schreibgeschützt und lebt nur im Arbeitsspeicher. Siehe [Aus einem alten Sicherungsordner](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Aus dem Appdata.Backup-Plugin übernehmen** liest den Sicherungsordner des Plugins. Jedes Container-Archiv wird zu einem Wiederherstellungspunkt seines Containers, datiert auf den Zeitpunkt, an dem das Plugin es erstellt hat. Schon früher importierte Archive werden übersprungen, und die Archive selbst werden nur gelesen. Der Container braucht zuerst eine Sicherung in BombVault, damit die Wiederherstellung seine Definition hat. Die Aufbewahrung lässt importierte Wiederherstellungspunkte in Ruhe, lösche also selbst einen, wenn du ihn nicht mehr brauchst.

## Nach einer Wiederherstellung {#after}

Eine Kopie, die neben dem Original wiederhergestellt wurde, steht unter **Sicherungen** als ausgeschlossen, zählt also nicht als offene Aufgabe. Nimm sie in den Zeitplan auf, wenn du sie behalten willst. Ein Container oder eine VM, der oder die bei der Sicherung lief, kommt laufend zurück, einer, der angehalten war, bleibt angehalten. Auf Unraid erscheint ein wiederhergestellter Container im Docker-Tab als normale, bearbeitbare App wieder, und eine VM im VM-Manager mit angehängten Disks und NVRAM.
