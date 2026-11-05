# ZFS-Datasets

ZFS-Datasets sind Einträge in der Liste **Sicherungen**, wie Container und VMs. Ein Eintrag ist ein Dataset zusammen mit jedem Dataset darunter. Für jede Sicherung macht BombVault einen ZFS-Snapshot des ganzen Baums, sodass jedes Dataset darin im selben Augenblick erfasst wird. Es liest dann die Dateien jedes Datasets aus diesem Snapshot und speichert sie mit restic, so wie es einen Ordner speichert, und entfernt den Snapshot gleich danach. Die Sicherungen sind dedupliziert, du kannst jede davon durchsehen, und einzelne Dateien lassen sich wiederherstellen.

Die Sicherung schickt nie einen Stream in restic und rollt nie ein Dataset zurück. BombVault zerstört nur Snapshots, die es selbst angelegt hat. Die optionale [Replik](#replica) ist die einzige Stelle, die `zfs send` nutzt: Sie kopiert die Datasets auf einen zweiten ZFS-Server und berührt die Sicherung nicht.

## Voraussetzungen {#requirements}

- **Die SSH-Verbindung zu diesem Server.** ZFS-Datasets nutzen denselben Schlüssel, Host und Benutzer wie VM-Sicherungen. Funktionieren VM-Sicherungen schon, funktioniert auch das. Sonst folge der [Anleitung zur VM-Sicherung über SSH](vm-backup-ssh-setup.md). Die Template-Felder heißen **Host SSH: Address**, **Host SSH: Port** und **Host SSH: User**.
- **Der Befehl `zfs` auf diesem Host.** Unraid 6.12 und neuer sowie TrueNAS SCALE haben ihn.
- **Host Data als `/mnt` gemappt mit Access Mode Read/Write - Slave.** Das ist die Vorgabe des Templates. Der Snapshot eines Datasets erscheint im Ordner `.zfs/snapshot` des Datasets erst, nachdem BombVault gestartet ist, der Container muss also Mounts erhalten, die der Host später macht.
- **Die Datasets unterhalb von `/mnt` gemountet.** Auf Unraid liegen Pools unter `/mnt/<pool>`, das ist also schon der Fall.

Schalte den Bereich unter **Einstellungen, Allgemein** ein (ZFS-Datasets). **ZFS-Verbindung zu diesem Server** unter **Mehr** auf der Seite Sicherungen öffnet dann **Verbindung zu diesem Server**. Es testet die SSH-Verbindung, nennt Benutzer und Host, mit denen es sich verbindet, und sagt, was fehlt, wenn etwas fehlt. Ist die Verbindung unterbrochen, sagt die Liste Sicherungen das mit **Verbindung ansehen**, und die Karte **Host-Integration** in der Übersicht zeigt dasselbe Ergebnis.

## Einträge und untergeordnete Datasets {#items-and-children}

Wähle auf der Seite Sicherungen **Hinzufügen**, dann **Ein ZFS-Dataset**. Die Liste kommt vom Server. Wähle das Dataset ganz oben von dem, was gesichert werden soll, zum Beispiel `cache/appdata`, und der Eintrag deckt es und jedes Dataset darunter ab.

- **Neue untergeordnete Datasets kommen von selbst dazu.** Ein Dataset, das später unterhalb des Eintrags angelegt wird, wird mit dem nächsten Lauf gesichert, und dieser Lauf nennt es als neu. Seine erste Sicherung liest es einmal vollständig. Danach werden nur Änderungen gelesen.
- **Du kannst einzelne untergeordnete auslassen.** Schalte ein untergeordnetes Dataset auf dem Tab Einstellungen des Eintrags aus, und es wird zusammen mit allem darunter ausgelassen. Ein ausgelassenes untergeordnetes Dataset, das auf dem Server nicht mehr existiert, wird als solches markiert und lässt sich aus der Liste entfernen.
- **Untergeordnete Datasets, die sich nicht lesen lassen, werden übersprungen, nie stillschweigend.** Der Lauf listet sie auf, der Eintrag zeigt, wie viele übersprungen wurden, und die Karte **Nicht gesichert** der Übersicht zählt jedes als nicht geschützt. Der Lauf sichert alles andere trotzdem und scheitert nicht wegen eines übersprungenen untergeordneten Datasets. Die Gründe stehen in der [Tabelle der Grundcodes](#reason-codes): ein Dataset, das nicht gemountet ist, `canmount=off` hat, einen `legacy`- oder gar keinen Mountpoint, einen Verschlüsselungsschlüssel, der nicht geladen ist, ausgeschalteten Snapshot-Zugriff oder einen Mountpoint, den BombVault nicht sieht.
- **Ein übersprungenes Dataset nimmt seine untergeordneten nicht mit.** Ein Dataset mit `canmount=off`, das nur andere Datasets enthält, wird übersprungen (als „nur Struktur“ angezeigt), und seine gemounteten untergeordneten werden gesichert. Ein verschlüsseltes Dataset, dessen Schlüssel nicht geladen ist, wird zusammen mit den untergeordneten übersprungen, die seinen Schlüssel teilen.
- **Untergeordnete Datasets, die VM-Disks oder Systemdaten sind, beginnen ausgeschaltet** im Hinzufügen-Dialog, mit dem Grund neben dem Schalter. Das Hinzufügen eines ganzen Pools verlangt eine Bestätigung, die auflistet, was er enthält.

### Volumes {#volumes}

Ein Volume (zvol) hält eine virtuelle Disk statt Dateien, und ein ZFS-Eintrag sichert nie eines.

- Ein Volume, das eine VM nutzt, wird mit dieser VM gesichert.
- Ein Volume, das keine VM nutzt (ein iSCSI-Extent, eine Disk, die du abgehängt hast), wird **von BombVault nicht gesichert**. Das Hinzufügen-Fenster und die Seite Sicherungen zählen diese Volumes und sagen das. Eine spätere Version wird sie sichern.

Volumes im Baum eines Eintrags werden übersprungen und bei jedem Lauf genannt.

### Der Speicher von Docker {#docker-storage}

Beim ZFS-Speichertreiber von Docker ist jede Image-Schicht ein Dataset mit einem `legacy`-Mountpoint. Der Hinzufügen-Dialog faltet diese pro übergeordnetem Dataset zu einer Zeile zusammen. Ein Baum, der mehr als 20 solcher Datasets enthält, kann kein Eintrag werden: Solange ein Snapshot davon existiert, kann Docker keine Image-Schichten entfernen. Füge stattdessen die Datasets darunter hinzu, zum Beispiel `appdata`.

### Einträge überlappen nie {#overlap}

Ein Dataset kann nur zu einem Eintrag gehören. BombVault lehnt einen neuen Eintrag ab, der innerhalb eines bestehenden liegt oder einen enthalten würde. Um mehrere untergeordnete Einträge zu einem übergeordneten zusammenzuführen, lösche zuerst die untergeordneten Einträge und wähle, ihre Sicherungen zu behalten, und füge dann den übergeordneten hinzu. Jedes Dataset behält seinen Verlauf unter seinem eigenen Namen, sodass die nächste Sicherung dort weitermacht, wo die alten Einträge aufgehört haben, und nicht alles noch einmal liest.

## Container anhalten und Befehle rund um den Snapshot {#consistency}

Ein Snapshot einer laufenden Datenbank ist wie ein plötzlicher Stromausfall: Die Datenbank erholt sich meist, aber sie muss es tun. Jeder Eintrag kann dagegen zweierlei tun, und beides deckt nur den Augenblick des Snapshots ab, nicht die ganze Sicherung.

- **Diese Container für den Snapshot anhalten.** BombVault hält die aufgeführten Container an, macht den Snapshot und startet sie sofort wieder. Container einer Abhängigkeitsebene halten parallel an, abhängige zuerst, sodass das ganze Fenster meist ein paar Sekunden dauert. Der Lauf zeigt, wie lange es war. Die Sicherung liest dann den eingefrorenen Snapshot, während die Apps schon wieder laufen. Es werden nur Container angehalten, die liefen.
- **Ein Befehl vor und nach dem Snapshot.** Er läuft in einem Container deiner Wahl, zum Beispiel um kurz vor dem Snapshot eine Datenbank in das Dataset zu dumpen, ohne etwas anzuhalten. Schlägt der Befehl vor dem Snapshot fehl, scheitert die Sicherung, und es wird kein Snapshot gemacht. Ein fehlschlagender Befehl nach dem Snapshot wird am Lauf angezeigt, lässt die Sicherung aber nicht scheitern.

Was passiert, wenn etwas schiefgeht:

- Lässt sich ein Container nicht anhalten, startet BombVault die schon angehaltenen wieder, und die Sicherung scheitert und nennt den Container. Es fällt nie auf einen Snapshot laufender Apps zurück.
- Das Anhalten wartet, bis eine laufende Container-Sicherung fertig ist (bis zu 30 Minuten bei einem manuellen Lauf, bis zum Zeitlimit der Sicherung bei einem geplanten), sodass beide nie denselben Container gleichzeitig anhalten und starten.
- Bevor der erste Container anhält, schreibt BombVault auf, welche es anhält. Wird BombVault im Fenster beendet, startet es diese Container beim nächsten Start wieder, sendet eine Benachrichtigung, und der Eintrag zeigt einen roten Hinweis für jeden Container, den es nicht starten konnte.

Die automatischen [Datenbank-Dumps](backups.md#database-dumps) laufen mit der Sicherung eines Containers, nicht mit einem ZFS-Eintrag. Eine Datenbank, deren Container nur über sein Dataset gesichert wird, bekommt keinen Dump, gib ihr hier also einen Befehl.

Ein Container kann auf dieser Liste stehen und zugleich als Container gesichert werden. Seine Daten werden dann zweimal gespeichert, in zwei Repositories, und das Gesamt-Backup hält ihn zweimal an. Der Eintrag sagt das.

## Wiederherstellen {#restore}

Öffne den Eintrag und wähle **Wiederherstellen**, oder wähle ihn auf der Seite **Wiederherstellung**. Das Wiederherstellungsfenster fragt nach der Sicherung, dann nach dem **Dataset**. Standardmäßig ist das das oberste Dataset des Eintrags. Unter **Was** stellst du alles oder nur die gewählten Dateien und Ordner wieder her. **Wohin** bietet:

- **In das Dataset zurückschreiben.** Dateien aus der Sicherung werden in den Mountpoint des Datasets geschrieben. Dateien mit demselben Namen werden überschrieben, andere bleiben. Das Dataset wird nie zurückgerollt oder ersetzt. BombVault prüft einmal vor dem Start und noch einmal direkt vor dem Schreiben, dass das Dataset gemountet, sichtbar und beschreibbar ist. Wo ein untergeordnetes Dataset darin gemountet ist, wird nichts geschrieben: Das untergeordnete behält seine Dateien, Besitzer und Rechte und wird aus seiner eigenen Sicherung wiederhergestellt.
- **In einen Ordner.** Wähle einen Ordner unterhalb von `/mnt`. BombVault prüft, dass der Ordner auf einem gemounteten Pool oder Share liegt und genug freier Platz da ist. Das funktioniert ohne die SSH-Verbindung und für Datasets, die es nicht mehr gibt.
- **In ein neues Dataset.** Nenne ein Dataset, das noch nicht existiert. Das Fenster schlägt `<dataset>-restored` vor. BombVault legt es mit den in der Sicherung gespeicherten ZFS-Eigenschaften an und stellt darin wieder her, siehe [Als neues Dataset wiederherstellen](#new-dataset).

Wählst du **Alle Datasets dieser Sicherung** als Dataset, schreibt das jedes Dataset des Baums in einen eigenen Unterordner des gewählten Ordners. Datasets, die in dieser Sicherung übersprungen wurden, werden genannt. Um aus dem Repository eines anderen BombVault wiederherzustellen, nutze **Aus einem alten Sicherungsordner** auf der Seite Wiederherstellung. Sie stellt immer in einen Ordner wieder her: alle Datasets einer Sicherung, jedes in einen eigenen Unterordner, oder ein Dataset des Baums, ganz oder mit gewählten Dateien.

Die Liste der anzuhaltenden Container des Eintrags wird auch für eine Wiederherstellung in das Dataset angeboten. Diese Container bleiben für die ganze Wiederherstellung angehalten, und Container-Sicherungen warten währenddessen.

### Der Sicherheits-Snapshot {#safety-snapshot}

Bevor es in ein Dataset schreibt, macht BombVault einen ZFS-Snapshot nur dieses Datasets, benannt `bombvault-prerestore-<time>`. **Vorher einen ZFS-Snapshot vom jetzigen Stand behalten** ist standardmäßig an. Es auszuschalten braucht eine zweite Bestätigung. Lässt sich der Snapshot nicht machen, wird nichts wiederhergestellt.

BombVault löscht einen Sicherheits-Snapshot nie von selbst. Der Eintrag listet sie mit Alter und Größe auf, jeden mit einer Aktion **Löschen**, und warnt, wenn der älteste mehr als 30 Tage alt ist, weil er gelöschte und geänderte Daten auf dem Pool hält.

Um nach einer Wiederherstellung zurückzugehen, kopiere einzelne Dateien aus `.zfs/snapshot/bombvault-prerestore-<time>` im Dataset. `zfs rollback <dataset>@bombvault-prerestore-<time>` funktioniert nur, solange er der neueste Snapshot dieses Datasets ist. `zfs rollback -r` löscht jeden neueren Snapshot, auch die automatischen.

### Als neues Dataset wiederherstellen {#new-dataset}

BombVault speichert die lokal gesetzten ZFS-Eigenschaften jedes Datasets mit jeder Sicherung: Kompression, Record-Größe, Quota, Reservierung, atime, xattr, acltype, Groß- und Kleinschreibung und deine eigenen User-Properties. Geerbte und schreibgeschützte Werte werden weggelassen, weil sie von selbst zurückkommen. Sicherungen von vor der Zeit, in der BombVault sie speicherte, haben keine.

- **In ein neues Dataset** führt `zfs create` mit jeder gespeicherten Eigenschaft aus. Groß-/Kleinschreibung, Normalisierung und utf8only lassen sich nur so setzen. Quotas und Reservierungen werden nach den Dateien gesetzt, sodass sie diese nicht ablehnen können. Der Mountpoint wird weggelassen, damit die Kopie nicht mit dem Original kollidiert, ebenso `canmount`, `readonly` und die Verschlüsselungseinstellungen, damit die Wiederherstellung schreiben kann. Ein neues Dataset unter einem verschlüsselten übernimmt dessen Verschlüsselung. Das Dataset über dem neuen muss existieren. Schlägt nach dem Anlegen des neuen Datasets etwas fehl, bleibt es auf dem Server, weil BombVault nie ein Dataset zerstört.
- **In das Dataset zurückschreiben** listet die gespeicherten Eigenschaften neben der Wiederherstellung auf. **Diese Eigenschaften auch setzen** setzt die, die ein bestehendes Dataset noch annehmen kann, bevor eine Datei geschrieben wird. Quotas und Reservierungen werden nach den Dateien gesetzt, sodass sie diese nicht ablehnen können. Ohne diesen Schalter behält das Dataset seine Einstellungen.

## Was in der Sicherung ist {#contents}

In der Sicherung: die Dateien und Ordner jedes gesicherten Datasets, mit Besitzer, Modi, Zeitstempeln und erweiterten Attributen, wie restic sie speichert, und die lokal gesetzten ZFS-Eigenschaften jedes Datasets.

Nicht in der Sicherung:

- Besitzer und Modus des obersten Ordners jedes Datasets selbst (alles darunter ist enthalten). Eine Wiederherstellung in das Dataset lässt den bestehenden obersten Ordner, wie er ist, eine Wiederherstellung in einen Ordner legt ihn mit Modus `0755` an,
- bestehende ZFS-Snapshots,
- untergeordnete Datasets, die übersprungen oder ausgelassen wurden,
- Volumes.

Um auf einen neuen Pool wiederherzustellen, lege den Pool an und stelle jedes Dataset in ein neues Dataset wieder her. Ob NFSv4-ACLs, wie TrueNAS sie auf SMB-Datasets nutzt, so zurückkommen, wie du es erwartest, ist noch nicht geprüft, kontrolliere eine Wiederherstellung also an deinen eigenen Daten, bevor du dich darauf verlässt.

## Verschlüsselte Datasets {#encryption}

Ein verschlüsseltes Dataset wird nur gesichert, solange sein Schlüssel geladen ist. Sonst wird es mit einer Warnung übersprungen. Lade den Schlüssel mit `zfs load-key` und mounte das Dataset. BombVault liest die Daten entschlüsselt und speichert sie im Repository von restic, das verschlüsselt ist. Hast du die Verschlüsselung in BombVault ausgeschaltet, ist dieses Repository es nicht.

## Replik {#replica}

Eine Replik ist eine Kopie der Datasets eines Eintrags auf einem zweiten ZFS-Server. BombVault hält sie mit `zfs send` und `zfs receive` aktuell. Der erste Lauf sendet alles, danach reisen nur die geänderten Blöcke. Auf dem anderen Server kannst du die Kopie sofort mounten.

Eine Replik ersetzt nie die Sicherung. Ältere Versionen, einzelne Dateien und die Prüfung kommen weiter aus den Sicherungen, und die Replik behält nur so viele Snapshots, wie du einstellst. Eine aktuelle Replik zählt als Kopie außer Haus, aber nicht als Sicherung.

Die Replik wartet nicht auf die Sicherung. **Nach jeder Sicherung** heißt nach dem Sicherungslauf des Eintrags, ob dieser Lauf geklappt hat, fehlgeschlagen ist oder nichts zu tun fand. Hat der ZFS-Bereich keinen eingeschalteten Speicherort und der Eintrag kein eigenes Repository, sendet ein Sicherungslauf des Eintrags nur die Replik, und die Karte Replik sagt das in der Blase neben **Wann**. Die Liste Sicherungen, der Kopf des Eintrags und der Laufverlauf zeigen die Replik: **Wird repliziert**, solange sie läuft, und **Repliziert** mit der Zeit, sobald sie fertig ist.

**Nur Replik, keine Sicherung** in der Karte Replik lässt den ganzen ZFS-Bereich Repliken senden und keine Sicherungen machen. Der eigene Ordner und die Off-site-Speicherorte behalten ihre Schalter, ein Ausschalten sichert also wieder dorthin, und die Karte jedes ZFS-Eintrags zeigt den Schalter des Bereichs, wie er gerade steht. Ein solcher Eintrag wartet statt auf eine Sicherung auf seine erste Replik, **Jetzt sichern** wird als **Jetzt replizieren** angeboten, und die Container des Eintrags halten an und seine Befehle laufen rund um den Snapshot wie bei einer Sicherung. Seine Repliken zählen für einen Zeitplan alle paar Tage, die Meldung über Überfälliges und die Abdeckung.

Schalte sie in der Karte **Replik** auf dem Tab Einstellungen des Eintrags ein. Die Karte listet jedes Dataset und Volume mit seinem Zustand auf, und **Jetzt replizieren** startet einen Lauf. Ein Replik-Lauf hat eine eigene Sperre, eine lange erste Übertragung hält die Sicherungen also nie auf.

| Feld | Was es festlegt |
|---|---|
| **Repliziert nach** | Das Ziel: ein ZFS-Server (markiert **über SSH**) oder eine **gekoppelte Instanz**. **ZFS-Server hinzufügen** richtet einen neuen ein. |
| **Wann** | **Nach jeder Sicherung**, also nach dem Sicherungslauf des Eintrags, oder **Eigener Plan**. |
| **Auf dem Ziel behalten** | Wie viele Snapshots auf dem ZFS-Server bleiben: **Kurz**, **Ausgewogen**, **Lang** oder **Eigene Werte**. Eine gekoppelte Instanz entscheidet das selbst. |
| **Auf diesem Server behalten** | Wie viele der neuesten Replik-Snapshots auch auf diesem Server bleiben. 0 oder 1 behält nur den letzten. |
| **Nur Replik, keine Sicherung** | Der ganze ZFS-Bereich sendet Repliken und macht keine Sicherungen. |

![Die Karte Replik eines ZFS-Eintrags.](assets/screenshots/zfs/replica-dark.png#only-dark)
![Die Karte Replik eines ZFS-Eintrags.](assets/screenshots/zfs/replica-light.png#only-light)

**Alle jetzt replizieren**, neben den Sicherungsaktionen, wenn auf der Seite Sicherungen die Kachel ZFS gewählt ist, sendet die Replik jedes ZFS-Eintrags, einen nach dem anderen. Der Server führt den Stapel aus, er läuft also weiter, wenn du die Seite verlässt, und ein Eintrag, der nicht replizieren kann, wird genannt und übersprungen.

### Auf einen ZFS-Server schieben {#replica-push}

Jede Maschine mit ZFS und SSH kann empfangen, zum Beispiel ein zweites Unraid oder ein TrueNAS. BombVault muss dort nicht laufen.

1. Wähle unter **Einstellungen, Kopplung** **Per SSH verbinden** in **ZFS-Server ohne BombVault**, oder **ZFS-Server hinzufügen** in der Karte **Replik** des Eintrags.
2. Gib Adresse, Benutzer und Port ein. Der Dialog zeigt den öffentlichen Schlüssel von BombVault. Füge ihn der `~/.ssh/authorized_keys` des Benutzers auf dem Server hinzu. Auf Unraid ist das unter **Users, root, SSH keys**. Je nach **Display Settings** sitzt Users im Kopfmenü oder unter Settings. Bearbeite auf TrueNAS den Benutzer unter **Credentials, Users**. Das ist der Replik-Schlüssel. Er unterscheidet sich vom Schlüssel der VM- und Host-SSH-Verbindung, füge ihn also auch dann hinzu, wenn jener schon freigegeben ist.
3. Teste die Verbindung. Der Dialog listet dann die Pools des Servers auf. Wähle einen und setze die Wurzel, die standardmäßig `<pool>/bombvault-replica` ist.
4. Wähle den neuen Server in der Karte **Replik** des Eintrags. Der Server hat dann auf der Seite Instanzen eine eigene Karte, und **Öffnen** führt zu seiner Seite.

Mit root ist nichts weiter nötig. Ein eigener Benutzer braucht diese Rechte auf dem Pool des Ziels, die der Dialog auch zeigt:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop,readonly,canmount <pool>
```

Auf der Quelle braucht derselbe Benutzertyp diese auf dem obersten Dataset des Eintrags:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy,mount <dataset>
```

In dieser Richtung hält das BombVault, das den Eintrag hält, auch einen Schlüssel, der auf dem Server schreiben kann.

BombVault hält den Host-Schlüssel fest, den der Server bei der ersten Verbindung zeigt, und lehnt später einen anderen ab. Wurde der Server neu installiert, drücke auf der Seite des Servers **Host-Schlüssel vergessen**, und die nächste Verbindung hält den neuen Schlüssel fest.

Kann der Host dieses Servers den ZFS-Server selbst erreichen, leitet er `zfs send` direkt in sein eigenes ssh zum Server, mit dem Replik-Schlüssel und dem festgehaltenen Host-Schlüssel, sodass der Stream nicht durch BombVault läuft. Sonst läuft der Stream durch BombVault.

### An eine gekoppelte Instanz senden {#replica-receive}

Ein gekoppeltes BombVault kann die Replik selbst empfangen. Niemand bekommt SSH-Zugang zum anderen Host, und kein Schlüssel kommt in eine `authorized_keys`-Datei.

1. Wähle in der Karte **Replik** des Eintrags die gekoppelte Instanz als Ziel, oder schalte die Rolle **ZFS-Server** auf der Karte dieser Instanz auf der Seite Instanzen ein und wähle dort die ZFS-Einträge. Die Karte zeigt **Wartet auf Freigabe**, bis sie antwortet.
2. Öffne auf der empfangenden Instanz **Instanzen**. Die Anfrage erscheint als Abzeichen auf der Karte der sendenden Instanz, und ihre **Details** listen sie auf. Wähle auf diesem Server Pool und Wurzel und wie viele Snapshots bleiben, dann drücke **Erlauben** oder **Ablehnen**.
3. Nach **Erlauben** sendet die Quelle nach ihrem eigenen Plan, wie bei jedem anderen Ziel.

Die empfangende Instanz akzeptiert nur, was die Freigabe abdeckt: die Datasets des Eintrags, in ihre eigene Wurzel. Sie führt `zfs receive` selbst aus, und die Quelle hat keine Möglichkeit, dort etwas zu löschen oder zurückzurollen. Die Kopie übersteht also eine Quelle, die jemand übernommen hat. Die empfangende Instanz behält ihre eigene Aufbewahrung. Die Quelle schlägt nur eine Regel vor, wenn sie anfragt. So eine Quelle kann trotzdem neue Snapshots senden, und mit der Zeit verdrängen diese ältere nach der Regel wie alle anderen. Ein Snapshotname, der nach der Uhr der empfangenden Instanz in der Zukunft liegt, zählt für die Regel erst, wenn diese Zeit gekommen ist.

**Freigabe entziehen** auf der empfangenden Instanz beendet die Freigabe jederzeit und sagt es der Quelle, die dann zeigt, dass die Freigabe zurückgezogen wurde, und anhält. Was die empfangende Instanz schon hält, bleibt dort. Eine abgelehnte Anfrage bleibt abgelehnt. Weitere Datasets oder eine Anfrage nach einem Entzug fragen erneut.

Die empfangende Instanz muss über HTTPS antworten. Die Quelle sendet nichts an eine einfache http-Adresse, da Token und Daten unverschlüsselt durchs Netz gingen. Jede Übertragung lässt außerdem ein Zehntel des empfangenden Pools frei und hält an, bevor sie daran nagen würde, sodass eine Quelle den Pool des empfangenden Servers nicht füllen kann.

### Wohin die Daten gehen {#replica-target}

Jedes Dataset landet unter `<root>/<server>/<pool>/<path>`. Der Server-Ordner ist der Name der Quellinstanz, bei der ersten Übertragung festgelegt, sodass zwei Server mit demselben Pool-Namen sich nie in die Quere kommen. Zum Beispiel landet `cache/appdata` eines Servers namens `tower` unter `backup/bombvault-replica/tower/cache/appdata`.

Nachdem BombVault neu aufgesetzt wurde, lehnt das Ziel den Ordner ab, den dieses BombVault zuvor gefüllt hat. Die Karte Replik bietet dann **Ordner übernehmen** an, und die nächste Replik macht von den Kopien darin weiter. Übernimm ihn nur, wenn kein anderes BombVault noch in diesen Ordner repliziert.

Die Kopie auf dem Ziel ist schreibgeschützt. Standardmäßig ist sie nicht gemountet und überdeckt daher nie etwas auf diesem Server. Ein per SSH verbundener ZFS-Server hat auf seiner Seite einen Schalter **Auf dem Server nur lesend einhängen**: Ist er an, mountet BombVault die Kopien dort nach jedem Lauf schreibgeschützt, sodass du ihre Dateien auf dem Ziel lesen und kopieren kannst. Verschlüsselte Kopien bleiben gesperrt, bis ihr Schlüssel geladen ist, und das Ausschalten hängt nichts aus. Eine gekoppelte Instanz, die die Replik empfängt, hält sie nicht gemountet. ZFS-Eigenschaften reisen mit, außer dem Mountpoint sowie `sharenfs` und `sharesmb`. Auch die Reservierungen `reservation` und `refreservation` bleiben zurück, sodass die Kopie nur den Platz braucht, den ihre Daten brauchen.

### Was hineingeht {#replica-contents}

Alles, was der Eintrag sichert, geht hinein, und auch die Volumes darunter, die die Sicherung überspringt. Ein untergeordnetes Dataset, das du im Eintrag ausgeschaltet hast, bleibt draußen. Alle Datasets eines Laufs stammen aus einem Snapshot, wie bei der Sicherung.

Eine Kopie, die eine andere BombVault auf diesen Server repliziert, wird nicht weitergesendet, weil das die Historie ausdünnen würde, die ihre Quelle noch braucht. Enthält der Baum eines Eintrags so eine Kopie, scheitert der Lauf für dieses Dataset und sagt warum, bis du es im Eintrag ausschaltest. Ein Eintrag, der selbst so eine Kopie ist, kann gar nicht replizieren. Das Löschen eines Eintrags oder das Ausschalten seiner Replik lässt diese Kopien und ihre Snapshots in Ruhe. Sobald du eine Kopie übernimmst, wie unter [Die Replik nutzen](#replica-use) beschrieben, zählt sie als normales Dataset.

### Wie lange Snapshots bleiben {#replica-retention}

Auf dem Ziel behält eine neue Replik 7 tägliche und 3 wöchentliche Snapshots. Wähle stattdessen **Kurz**, **Ausgewogen** oder **Lang**, oder setze **Eigene Werte**. Eigene Werte, die alle null sind, behalten jeden Snapshot, und die Karte sagt das. Dort werden nur Snapshots mit dem Namen `bombvault-replica-<14 digits>` je entfernt, und nie der neueste, den beide Seiten teilen.

Auf der Quelle behält BombVault den letzten Replik-Snapshot, dazu ein Lesezeichen für jeden Zustand, den es gesendet hat. Lesezeichen kosten keinen Platz. Die nächste Übertragung startet von ihnen.

Um auch auf der Quelle mehr zu behalten, setze **Auf diesem Server behalten** in der Karte Replik, zum Beispiel auf 7. So viele der neuesten Replik-Snapshots bleiben dann nach jeder Replik auf diesem Server, neben dem, was das Ziel behält. 0 oder 1 behält nur den letzten. Der letzte trägt einen Hold, sodass die Basis der nächsten Übertragung nicht versehentlich zerstört wird. Die älteren haben keinen. Jeder Snapshot hält die Blöcke fest, die sich danach geändert haben, jeder, den du behältst, belegt also Platz auf dem Pool. Das Ausschalten der Replik, der Wechsel auf ein anderes Ziel oder das Löschen des Eintrags entfernt sie auch von diesem Server. Snapshots eines anderen Eintrags, der darunter liegt, bleiben. Ist der Host nicht erreichbar, während du den Eintrag löschst, bleiben die Snapshots ebenfalls, und die Seite sagt das. Entferne sie von Hand: `zfs release -r bombvault-replica <dataset>@<snapshot>`, dann `zfs destroy`.

Die Dateien eines behaltenen Snapshots lassen sich sofort lesen oder kopieren, aus dem Ordner `.zfs/snapshot` im Dataset oder aus einem Klon, ohne das Dataset oder die Replik anzufassen:

```
ls /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261006014100
zfs clone cache/appdata@bombvault-replica-20261006014100 cache/appdata-20261006
```

Ein Klon braucht keinen Platz, bis sich etwas darin ändert. Zerstöre ihn mit `zfs destroy cache/appdata-20261006`, wenn du fertig bist, denn ein Snapshot mit einem Klon lässt sich nicht entfernen.

Um auf den letzten Replik-Snapshot zurückzugehen, genügt hier `zfs rollback cache/appdata@<latest>`, und auf dem Ziel nichts. Das Zurückrollen des ganzen Datasets auf einen älteren zerstört jeden Snapshot danach, auf diesem Server und auf dem Ziel. Der letzte ist gehalten, hebe also zuerst seinen Hold auf und rolle das Dataset dann hier zurück:

```
zfs release -r bombvault-replica cache/appdata@bombvault-replica-20261008014200
zfs rollback -r cache/appdata@bombvault-replica-20261006014100
```

Die Kopie auf dem Ziel muss auch zurück, aber nicht immer auf denselben Snapshot, da ihre eigenen Aufbewahrungsregeln ihn vielleicht fallen gelassen haben. Führe die Replik einmal aus. Sie hält für dieses Dataset an, weil das Ziel neuere Snapshots hat, und die Karte Replik und der Laufverlauf nennen den Snapshot, auf den das Ziel zurückzurollen ist: den neuesten, den beide Seiten noch teilen. Rolle die Kopie darauf zurück, auf dem ZFS-Server oder auf dem Host der gekoppelten Instanz, die die Replik empfängt, zum Beispiel:

```
zfs rollback -r backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261005014000
```

`zfs rollback` wirkt auf ein Dataset, wiederhole die Rollbacks also für jedes untergeordnete Dataset, das du auch zurückhaben willst. Danach macht **Jetzt replizieren** vom gemeinsamen Snapshot aus weiter.

### Verschlüsselte Datasets in einer Replik {#replica-encryption}

Ein verschlüsseltes Dataset wird roh gesendet. Es bleibt auf dem Ziel verschlüsselt, und das Ziel sieht den Schlüssel nie. Bewahre den Schlüssel sicher auf: Du brauchst ihn, um die Kopie nach einer Wiederherstellung zu öffnen, und eine Replik ohne ihn ist unlesbar.

### Die Replik nutzen {#replica-use}

Öffne den Tab **Sicherungen** des Eintrags und klicke auf die Zeile **ZFS-Replik** unter den Orten, die seine Snapshots halten. Das Blatt listet die Snapshots auf dem Ziel auf und zeigt die Befehle mit deinen echten Namen.

Um einen alten Zustand anzusehen, klone einen Snapshot auf dem Ziel. Ein Klon braucht keinen Platz, bis sich etwas ändert, und die Replik bleibt unberührt:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Fällt die Quelle aus, mach die Kopie auf dem Ziel zu einem normalen, beschreibbaren Dataset:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs list -rH -o name -t filesystem backup/bombvault-replica/tower/cache/appdata | xargs zfs set canmount=on && zfs mount -a
```

BombVault hört danach auf, in dieses Dataset und seine untergeordneten Datasets zu replizieren. Der nächste Lauf scheitert mit einem Grund, und was du geschrieben hast, bleibt. Dasselbe gilt für alles andere, was seit ihrem neuesten Snapshot auf die Kopie geschrieben wurde: BombVault rollt eine Kopie nie zurück. Um wieder dorthin zu replizieren, hol dir, was du brauchst, von der Kopie herunter und zerstöre sie auf dem Ziel. Der nächste Lauf sendet dann alles neu.

Um einen Zustand zur Quelle zurückzuholen, drücke im Blatt **Als neues Dataset zurückholen**. BombVault sendet den Snapshot in ein neues Dataset neben dem Original, benannt `<dataset>-bombvault-restore-` plus Zeitstempel. Es schreibt nie über das Original. Ein untergeordnetes Dataset, das auf der Quelle verloren ist, kommt ebenfalls zurück, solange die Kopie diesen Snapshot noch hat. Eines, das ihn nicht hat, bleibt draußen, und der Lauf sagt das.

### Beispiel: eine Replik auf eine zweite Unraid-Box {#example-replica}

Der Server `tower` sichert das Dataset `cache/appdata` als ZFS-Eintrag. Eine zweite Unraid-Box unter `192.168.1.20`, ohne BombVault, hat einen Pool namens `backup` und soll eine Kopie halten, die du jederzeit lesen kannst.

1. Öffne **Sicherungen**, dann `cache/appdata`, dann seinen Tab **Einstellungen**. Wähle in der Karte **Replik** unter **Repliziert nach** **ZFS-Server hinzufügen**.
2. Gib `192.168.1.20`, den Benutzer `root` und Port `22` ein. Der Dialog zeigt den Replik-Schlüssel. Füge ihn auf der zweiten Box unter **Users, root, SSH keys** ein und drücke **Apply**. Dieser Schlüssel unterscheidet sich vom Host-SSH-Schlüssel, füge ihn also auch dann hinzu, wenn jener schon da ist.
3. Teste die Verbindung. Wähle den Pool `backup` und behalte die vorgeschlagene Wurzel, `backup/bombvault-replica`.
4. Zurück auf der Karte ist der neue Server unter **Repliziert nach** gewählt. Lass **Wann** auf **Nach jeder Sicherung**, lass **Auf dem Ziel behalten** bei seinen 7 täglichen und 3 wöchentlichen Snapshots und setze **Auf diesem Server behalten** auf `7`.
5. Drücke **Jetzt replizieren**. Der erste Lauf sendet alles, 300 GB über eine 1-Gbit/s-Leitung dauern also mindestens 40 Minuten. Die Karte und die Liste Sicherungen sagen **Wird repliziert**, dann **Repliziert** mit der Zeit. Spätere Läufe senden nur die Blöcke, die sich geändert haben.

Die Datasets liegen jetzt unter `backup/bombvault-replica/tower/cache/appdata` auf der zweiten Box, schreibgeschützt. In der Übersicht zählt **Off-site** des ZFS-Bereichs die Replik als Kopie außer Haus.

Um eine Datei von letztem Dienstag zu holen, hast du drei Wege:

- **Auf diesem Server**, weil **Auf diesem Server behalten** sieben Snapshots hält:

    ```
    cp /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261103014100/plex/Preferences.xml /mnt/user/restore/
    ```

- **Auf der zweiten Box**, mit einem Klon eines Snapshots auf dem Ziel. Der Tab **Sicherungen** des Eintrags zeigt die Befehle mit deinen echten Namen, wenn du auf die Zeile **ZFS-Replik** klickst. Entferne den Klon mit `zfs destroy`, wenn du fertig bist.
- **Mit einem Mount**, indem du **Auf dem Server nur lesend einhängen** auf der Seite des ZFS-Servers einschaltest. Nach dem nächsten Lauf stehen die Kopien auf der zweiten Box gemountet und schreibgeschützt.

Versionen, die älter sind, als die Replik behält, kommen weiter aus den Sicherungen, wie das nächste Beispiel zeigt.

### Beispiel: eine Datei aus einer ZFS-Sicherung {#example-zfs-file}

1. Öffne **Wiederherstellung**, wähle die Kachel **ZFS** und klicke auf `cache/appdata`.
2. Wähle die Sicherung und unter **Dataset** das Dataset, das die Datei enthält.
3. Wähle unter **Was** **Einzelne Dateien** und hake die Datei ab.
4. Wähle unter **Wohin** **In einen Ordner** und einen Ordner unterhalb von `/mnt`, etwa `user/restore`. Das braucht keine SSH-Verbindung und funktioniert auch für ein Dataset, das es nicht mehr gibt.
5. Drücke **Wiederherstellen**. Die Datei landet direkt in diesem Ordner, ohne den Pfad darüber.

## Übrig gebliebene Snapshots {#leftover-snapshots}

Der Snapshot einer Sicherung heißt `<dataset>@bombvault-<14 digits>`, zum Beispiel `cache/appdata@bombvault-20260924021500` (UTC). BombVault entfernt ihn gleich nach der Sicherung. Schlägt das fehl, zum Beispiel weil das Dataset beschäftigt ist oder BombVault gestoppt wurde, entfernt BombVault ihn:

- vor der nächsten Sicherung dieses Eintrags,
- beim Start von BombVault, für jeden Eintrag, auch bei ausgeschalteter Domain,
- wenn du den Eintrag löschst,
- wenn du am Eintrag **Jetzt entfernen** drückst, was zeigt, wie viele übrig sind.

Es werden nur Namen entfernt, die genau `bombvault-` plus 14 Ziffern entsprechen. Sicherheits-Snapshots, deine eigenen Snapshots und automatische Snapshots werden nie angefasst. Um einen von Hand zu entfernen:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

Der Snapshot einer Replik heißt `<dataset>@bombvault-replica-<14 digits>` und ist kein Überbleibsel. Er bleibt auf der Quelle, bis ihn ein neuerer Replik-Lauf ersetzt oder solange **Auf diesem Server behalten** ihn hält, und auf dem Ziel, solange die Aufbewahrung ihn hält. Der Aufräumer fasst ihn nie an, weil er nur `bombvault-` gefolgt von genau 14 Ziffern trifft.

## Anomalien {#anomalies}

Ein untergeordnetes Dataset, das geleert wurde, ändert die Summe eines großen Baums kaum, daher beobachtet die Anomalie-Erkennung jedes Dataset eines Eintrags für sich: Größe, Dateizahl, neue Daten und restic-Zeit haben jeweils ihren eigenen Verlauf. Dieser Verlauf gehört zum Namen des Datasets und bleibt daher, wenn der Baum später von einem anderen Eintrag gesichert wird.

Ein Dataset, das der vorige Lauf gesichert hat und das dieser Lauf nicht lesen konnte, zählt als geleert, solange sich die Auswahl des Eintrags nicht geändert hat. Das deckt einen nicht geladenen Schlüssel, ein nicht gemountetes Dataset und eines, das aus dem Baum verschwunden ist. Ein untergeordnetes Dataset, das du selbst ausschließt, ändert die Auswahl, sein Verlauf beginnt also stattdessen neu. Solange ein Fund über verlorene Daten offen ist, behält die Aufbewahrung die alten Sicherungen dieses einen Datasets und räumt den Rest des Baums wie gewohnt auf.

Jedes Dataset hat in der Überwachung des Eintrags eine eigene Zeile, die sich mit **Überwachung** auf der Seite **Anomalien** öffnet und auf dem Tab **Prüfung** des Eintrags steht. Die offenen Funde zeigen sich neben jedem Dataset. Der Link in einem Fund öffnet das Wiederherstellungsfenster auf der letzten guten Sicherung des Datasets. Ob ein Lauf durchläuft, wird für den ganzen Eintrag beurteilt, weil ein Lauf als Ganzes gelingt oder scheitert.

Die Prüfungen selbst beschreibt [Anomalien](anomalies.md). Ein Assistent, der über den [MCP-Server](mcp.md) verbunden ist, kann die Wiederherstellungspunkte eines ZFS-Eintrags auflisten, seine Sicherung starten und die Funde lesen, aber das Quittieren geschieht auf der Seite **Anomalien**.

## Grundcodes {#reason-codes}

Die Seite des Eintrags, der Laufverlauf und die Benachrichtigungen nennen ein Problem mit einem dieser Codes. Bei den meisten steht die Lösung auch auf der Seite daneben.

| Code | Bedeutung | Was zu tun ist |
|---|---|---|
| `ssh-missing` | Die SSH-Verbindung ist in diesem Container nicht eingerichtet. | Richte die SSH-Verbindung wie für VM-Sicherungen ein. |
| `host-placeholder` | Host SSH: Address ist noch der Platzhalter, und `host.docker.internal` hat auch nicht geantwortet. | Setze Host SSH: Address auf die LAN-IP dieses Servers. |
| `host-fallback` | Host SSH: Address ist noch der Platzhalter, und `host.docker.internal` funktioniert. | Nichts, oder setze die LAN-IP. |
| `ssh-unreachable` | Der Server ist über SSH nicht erreichbar. | Prüfe Adresse und Port und dass SSH eingeschaltet ist. |
| `ssh-auth` | Der Server hat den Schlüssel von BombVault abgelehnt. | Führe den Befehl, der unter **Verbindung zu diesem Server** steht, einmal auf dem Server aus. |
| `zfs-not-found` | Der SSH-Host hat keinen Befehl `zfs`. | Richte Host SSH: Address auf die Maschine, der die Pools gehören. |
| `zfs-permission` | Der SSH-Benutzer darf diesen zfs-Befehl nicht ausführen. | Nimm root oder siehe [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` nennt einen anderen Host oder Benutzer als die SSH-Felder. | Bring sie in Einklang oder leere die SSH-Felder, sodass beides aus der URI kommt. |
| `zfs-error` | zfs meldete einen anderen Fehler. | Die Details zeigen seine Meldung. |
| `propagation-missing` | Neue Mounts auf dem Host erreichen den Container nicht. | Setze den Access Mode von Host Data auf Read/Write - Slave und starte BombVault neu. |
| `invalid-name` | Ein Dataset-Name, den BombVault nicht akzeptiert. | Benenne das Dataset um. |
| `name-too-long` | Ein Dataset im Baum ist für einen Snapshot-Namen zu lang. | Benenne es um oder nimm ein Dataset darunter als Eintrag. |
| `invalid-exclude` | Ein Ausschlussmuster oder ein ausgelassenes untergeordnetes Dataset passt nicht zum Eintrag. | Korrigiere den Eintrag, den die Meldung nennt. Um ein ganzes untergeordnetes Dataset auszulassen, schalte es aus, statt ein Muster zu schreiben. |
| `not-found` | Das Dataset existiert auf dem Server nicht. | Entferne den Eintrag oder lege das Dataset neu an. Seine Sicherungen bleiben wiederherstellbar. |
| `not-filesystem` | Das ist ein Volume, kein Dateisystem. | Siehe [Volumes](#volumes). |
| `overlaps-item` | Das Dataset überlappt einen bestehenden Eintrag. | Siehe [Einträge überlappen nie](#overlap). |
| `docker-storage` | Der Baum enthält den Image-Speicher von Docker. | Siehe [Der Speicher von Docker](#docker-storage). |
| `nothing-readable` | Kein Dataset im Eintrag lässt sich gerade lesen. | Sieh dir die Codes der übersprungenen Datasets an. |
| `snapshot-failed` | Der Snapshot ließ sich nicht anlegen. | Die Details zeigen die Meldung von zfs. |
| `containers-busy` | Eine Container-Sicherung lief noch, als die Container anhalten mussten. | Starte später noch einmal. Geplante Läufe warten von selbst. |
| `consistency-stop-failed` | Ein Container ließ sich nicht anhalten, daher wurde kein Snapshot gemacht. | Prüfe den Container oder nimm ihn von der Liste. |
| `pre-snapshot-failed` | Der Befehl vor dem Snapshot ist fehlgeschlagen. | Die Lauf-Details zeigen seine Ausgabe. |
| `container-unknown` | Ein aufgeführter Container existiert nicht. | Entferne ihn aus der Liste. |
| `container-is-self` | BombVault kann seinen eigenen Container nicht anhalten. | Entferne ihn aus der Liste. |
| `leftover-snapshots` | Snapshots, die BombVault nicht entfernen konnte, sind noch auf dem Server. | Drücke **Jetzt entfernen**, siehe [Übrig gebliebene Snapshots](#leftover-snapshots). |
| `zvol` | Ein Volume im Baum, übersprungen. | Siehe [Volumes](#volumes). |
| `canmount-off` | Nie gemountet (`canmount=off`), übersprungen. | Enthält es Daten, mounte es oder verschiebe die Daten in ein untergeordnetes Dataset. |
| `legacy-mount` | Legacy-Mountpoint, übersprungen. | Gib ihm einen Mountpoint unterhalb von `/mnt`. |
| `no-mountpoint` | Kein Mountpoint, übersprungen. | Gib ihm einen Mountpoint unterhalb von `/mnt`. |
| `not-mounted` | Auf dem Server nicht gemountet, übersprungen. | Mounte es mit `zfs mount` oder setze `canmount=on`. |
| `key-not-loaded` | Verschlüsselt, und der Schlüssel ist nicht geladen, übersprungen. | `zfs load-key`, dann mounten. |
| `snapdir-disabled` | Der Snapshot-Zugriff ist ausgeschaltet, übersprungen. | `zfs set snapdir=hidden <dataset>`. Der Ordner `.zfs` bleibt versteckt. |
| `not-visible` | BombVault kann den Mountpoint des Datasets nicht sehen. | Verschiebe den Mountpoint unterhalb des Host-Data-Pfads oder mappe ihn mit Read/Write - Slave unter demselben Pfad in den Container. |
| `shfs-only` | Das Dataset ist nur über `/mnt/user` sichtbar, was Snapshots verbirgt. | Mappe `/mnt`, nicht `/mnt/user`, als Host Data. |
| `snapshot-not-visible` | Der Snapshot wurde angelegt, erschien aber nicht in BombVault. | Führe **Snapshot-Zugriff testen** aus. Siehe unten. |
| `snapshot-loop` | Der Snapshot erreichte BombVault nicht, weil Host Data neue Mounts nicht durchreicht. | Setze den Access Mode von Host Data auf Read/Write - Slave und starte BombVault neu. |
| `backup-failed` | restic ist für dieses Dataset fehlgeschlagen. | Die Lauf-Details zeigen warum. |
| `not-reached` | Der Lauf endete vor diesem Dataset. | Führe die Sicherung noch einmal aus. |
| `gone` | Das Dataset ist nicht mehr auf dem Server. | Nichts. Seine Sicherungen bleiben wiederherstellbar. |
| `read-only-mount` | BombVault kann das Dataset nur lesen und daher nicht hinein wiederherstellen. | Setze das Mapping auf Read/Write - Slave oder stelle in einen Ordner wieder her. |
| `destination-not-mounted` | Der Ordner liegt nicht auf einem gemounteten Pool oder Share. | Wähle einen Ordner auf einem Pool oder Share. |
| `not-enough-space` | Nicht genug freier Platz am Ziel. | Schaffe Platz oder wähle einen anderen Ordner. |
| `safety-snapshot-failed` | Der Sicherheits-Snapshot ließ sich nicht machen, daher wurde nichts wiederhergestellt. | Die Details zeigen die Meldung von zfs. |
| `safety-name-too-long` | Der Dataset-Name ist für einen Sicherheits-Snapshot zu lang. | Schalte den Sicherheits-Snapshot aus oder stelle in einen Ordner wieder her. |
| `dataset-exists` | Ein Dataset mit diesem Namen existiert schon. | Wähle einen neuen Namen oder stelle in das Dataset selbst wieder her. |
| `create-failed` | Das neue Dataset ließ sich nicht anlegen. | Die Details zeigen die Meldung von zfs. Prüfe, dass das Dataset darüber existiert. |
| `new-dataset-not-visible` | Das neue Dataset wurde angelegt, BombVault kann es aber nicht sehen, daher wurde nichts wiederhergestellt. | Das Dataset bleibt auf dem Server. Mounte es unterhalb des Host-Data-Pfads und stelle hinein wieder her. |
| `set-properties-failed` | Die gespeicherten Eigenschaften ließen sich nicht setzen, daher wurde nichts wiederhergestellt. | Die Details zeigen die Meldung von zfs. |
| `set-limits-failed` | Die Dateien wurden wiederhergestellt, aber die gespeicherte Quota oder Reservierung ließ sich nicht setzen. | Die Details zeigen die Meldung von zfs. Setze die Quota oder Reservierung selbst mit `zfs set`. |

### Prüfen, was der Container sieht {#mountinfo}

**Snapshot-Zugriff testen** auf dem Tab Einstellungen des Eintrags macht einen echten Snapshot seines Baums, sucht in BombVault für jedes Dataset danach und entfernt ihn wieder. Es ist der schnellste Weg, den ganzen Pfad vor dem ersten geplanten Lauf zu beweisen.

Um selbst nachzusehen, führe das auf dem Server aus:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Jede Zeile ist ein Mount im Container. Die Zeile eines Datasets zeigt seinen Pfad im Container (unterhalb von `/host/user`) und den Dataset-Namen. Ein Feld `master:N` in dieser Zeile heißt, der Mount erhält die Mounts, die der Host später macht, was der Snapshot-Zugriff braucht. Fehlt es, setze den Access Mode von Host Data auf Read/Write - Slave und starte BombVault neu.

## TrueNAS SCALE {#truenas}

- Ist `LIBVIRT_URI` gesetzt (wie für VM-Sicherungen auf TrueNAS), nimmt BombVault SSH-Host, Benutzer und Port für seine zfs-Befehle aus der URI, jeweils jene, die nicht einzeln gesetzt sind. Ohne VM-Sicherungen muss nichts gesetzt werden: `host.docker.internal` löst sich in einer TrueNAS-App nicht auf, daher erreicht BombVault den Host über das Standard-Gateway der App, als root auf Port 22. Setze `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` und `LIBVIRT_SSH_PORT` im `environment` der App, wenn du eine andere Adresse oder einen anderen Benutzer willst.
- Füge den Host-SSH-Schlüssel aus **Einstellungen, Anbindungen, Host-SSH** dem Benutzer root unter **Credentials, Users** hinzu und schalte den SSH-Dienst unter **System, Services** ein. Eine [Replik](#replica-push) auf dieses oder ein anderes TrueNAS braucht zusätzlich den Replik-Schlüssel, der ein anderer ist.
- Ein anderer Benutzer als root braucht Rechte auf dem obersten Dataset des Eintrags, die dann jedes Dataset darunter abdecken:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Das Wiederherstellen in ein neues Dataset braucht außerdem `create` auf dem Dataset darüber, und das Setzen gespeicherter Eigenschaften braucht Rechte für diese Eigenschaften.

  Eine SSH-Sitzung eines Nicht-root-Benutzers auf TrueNAS hat `/usr/sbin` nicht im Pfad. BombVault ruft dann `/usr/sbin/zfs` direkt auf.
- Das **Host Data** der App muss ein Host-Pfad über den Datasets sein, zum Beispiel `/mnt/tank`, kein ixVolume. Mit einem Host-Pfad reicht die App die neuen Mounts des Hosts an BombVault weiter (`rslave`), was der Snapshot-Zugriff braucht. Die ganze App-Einrichtung steht unter [TrueNAS SCALE](getting-started.md#truenas).
