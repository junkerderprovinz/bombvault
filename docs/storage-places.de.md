# Speicherorte

Ein Speicherort ist eine Stelle, an der BombVault Backups ablegt: ein Ordner auf diesem Unraid, eine Freigabe auf einem NAS, ein Bucket bei einem Cloud-Anbieter, ein rest-server, ein SFTP-Konto oder eine Nextcloud. Jeden Ort verbindest du einmal, unter **Einstellungen, Speicher**, und seine Zugangsdaten, seine Aufbewahrung, sein Schutz und sein Standort gehören zu ihm. Die fünf Bereiche (Container, VMs, der Flash, BombVaults eigene Konfiguration und Ordner-Sets) wählen dann unter diesen Orten: wo jeder Bereich gespeichert wird und wohin er kopiert wird.

## Einen Ort hinzufügen {#add-a-place}

**Ort hinzufügen** öffnet ein Fenster mit einer Kachel pro Anbieter, in drei Gruppen: Cloud-Speicher, selbst betriebene Dienste sowie NAS und dieser Server.

1. Wähl eine Kachel und füll ihr Formular aus. Der Knopf mit dem Auge zeigt ein eingetipptes Geheimnis an.
2. **Verbindung testen** prüft den Ort und legt nichts an. Für den Ordner jedes Bereichs meldet der Test, was er vorgefunden hat: leer oder noch nicht vorhanden, schon ein restic-Repository darin, oder den Fehler, an dem er gescheitert ist.
3. Gib dem Ort einen Namen; der Name des Anbieters ist schon eingetragen. Bei einem Gerät, das du selbst betreibst, beantwortest du **Wo steht das Gerät?**. Cloud-Anbieter stehen immer an einem anderen Standort, und ein Ordner auf diesem Unraid steht immer hier.
4. **Hinzufügen** speichert den Ort.

Ein neuer Ort wird noch von keinem Bereich benutzt. Wähl ihn auf der [Karte Domänen](#domains) unter **Gespeichert in** oder **Kopiert nach**, oder auf der Karte eines Elements für dieses eine Element.

## Ordner {#folders}

Ein Ort hat einen Ordner pro Bereich: `container`, `vms`, `flash`, `config` und `files`, dieselben Namen wie bei den voreingestellten Backup-Orten. Die Ordner stehen in den Details des Ortes und lassen sich dort umbenennen (siehe [Eine Adresse ändern](#addresses)). Ein Bereich ohne Ordner an einem Ort kann diesen Ort nicht wählen.

Nutzt ein Bereich einen Ort in beiden Rollen, bekommt die zweite Rolle eine Endung, und die erste behält ihren Ordner. Ein Ort, der schon die Kopien eines Bereichs empfängt, legt die direkt dorthin gesendeten Elemente in `<folder>-direct` ab; ein Ort, an dem ein Bereich schon gespeichert ist, empfängt dessen Kopien in `<folder>-copies`.

Manche Orte sind selbst ein restic-Repository: eine Adresse, an der beim Hinzufügen schon ein Repository lag, ein benanntes Repository aus einer bestehenden Einrichtung oder ein Kopieziel im obersten Verzeichnis eines Buckets. So ein Ort hat keine Ordner, alle Bereiche teilen sich sein eines Repository, und er übernimmt keine zweite Rolle. Wenn du beim selben Anbieter mehr ablegen willst, verbinde einen weiteren Bucket oder Ordner als eigenen Ort.

## Details eines Ortes {#details}

Jeder Ort ist eine Zeile mit seinem Anbieter, seiner Verwendung und seinem letzten Test oder seiner letzten Kopie. **Testen** prüft jede Adresse am Ort, und **Details** öffnet seine Einstellungen. Jede Änderung in den Details wird sofort gespeichert.

- **Allgemein**: der Name, der Schalter, der den Ort ein- und ausschaltet, die Adresse und bei einem Gerät, das du selbst betreibst, **Wo steht das Gerät?** (siehe [Außer Haus](#off-the-premises)).
- **Aufbewahrung**: letzte, täglich, wöchentlich und monatlich, für jedes Repository am Ort. Ein neuer Ort beginnt mit den voreingestellten Regeln; ein Ort, bei dem jede Regel auf null steht, kürzt nie.
- **Schutz**: der Schalter **Append-only**. Durchsetzen muss es die Gegenseite; ist der Schalter an, kürzt und löscht BombVault dort nichts. Bei einem rest-server mit eingeschaltetem Append-only führt **Auf append-only prüfen** den Manipulationstest für jeden Bereichspfad, jede eingeschaltete Kopie und jedes Repository an diesem Ort aus und zeigt eine Antwort für den ganzen Ort: *Löschen verweigert* oder *Löschen möglich* (siehe [Off-site & Wiederherstellung](offsite-recovery.md)). Diesen Abschnitt haben nur entfernte Orte, weil nichts auf dieser Box verhindern kann, dass ein lokales Repository gelöscht wird.
- **Zugang**: die Zugangsdaten und bei S3 die Speicherklasse. Ein Ort, der die gemeinsamen Zugangsdaten benutzt, bekommt bei der ersten Änderung einen eigenen Satz. Ein direktes Repository am Ort, das die neuen Zugangsdaten nicht öffnen können, behält die alten, und die Antwort sagt das. Ordner-, SFTP- und rclone-Orte haben diesen Abschnitt nicht.
- **Grenzen**: die Upload- und Download-Rate und das Wachstumsbudget.
- **Ordner**: ein Schalter pro Bereich, mit dem Namen seines Ordners. Ein Bereich, der hier ausgeschaltet ist, kann den Ort nicht wählen.

Wer die Aufbewahrung senkt, wird vorher gefragt und erfährt, wie viele Elemente das betrifft; wer Append-only ausschaltet, wird vorher gefragt und erfährt, wie viele Repositories an dem Ort den Schutz verlieren. Wird ein Ort ausgeschaltet, sind auch alle Repositories an ihm aus; ein Ort, an dem ein Bereich gespeichert ist, lässt sich nicht ausschalten.

## Die Karte Domänen {#domains}

Die Karte hat eine Zeile pro Bereich, mit seinem Zeitplan, seinem Speicherort, seinen Kopien und seinen Ausnahmen.

- **Gespeichert in**: Solange am bisherigen Ort des Bereichs keine Backups liegen, wird der gewählte Ort sein Speicherort, und der Backup-Pfad zieht dorthin um. Liegen dort schon Backups, wird die Wahl bei Containern, VMs und Ordner-Sets zur Vorgabe für neue Elemente, die sie bei ihrem ersten Backup übernehmen; Elemente mit Backups bleiben, wo sie sind, weil BombVault nie ein Backup verschiebt. Beim Flash und bei BombVaults eigener Konfiguration zieht der Speicherort um, und die schon geschriebenen Backups bleiben am alten Ort.
- **Kopiert nach**: ein Chip pro Ort, der die Kopien des Bereichs aufnehmen kann. Ein angehakter Chip macht den Ort zu einem Kopieziel des Bereichs; beim ersten Mal sagt BombVault vorher, wie viele Elemente und Snapshots der erste Lauf überträgt und welche Datenmenge das ist. Nimmst du den Haken weg, kommen keine neuen Kopien mehr dazu: Die Kopien, die schon dort liegen, bleiben und altern nach der Aufbewahrung des Ortes, und Elemente mit eigener Wahl kopieren weiter dorthin. Wer den letzten Haken wegnimmt, stoppt alle Kopien, auch zu später hinzugefügten Orten, bis wieder einer angehakt ist. Ein ausgeschalteter Ort erscheint als blasser Chip und lässt sich nicht wählen.
- **Ausnahmen**: die Elemente mit eigener Wahl, als Liste mit Links zu ihren Karten.
- **Jetzt kopieren** startet die Kopien des Bereichs sofort.

Ein Bereich, der nach einem Neuaufbau über Backups entdecken pausiert, zeigt die Pause in seiner Zeile, mit **Vorgabe bestätigen** (siehe [Ablage pro Element](offsite-recovery.md#placement)).

## Eine Adresse ändern {#addresses}

In den Details des Ortes kannst du den Ordner eines Bereichs ändern und bei einem lokalen Ort auch die Adresse, zum Beispiel nachdem du ein Repository von Hand auf eine andere Platte umgezogen hast. BombVault testet jede Adresse, die die Änderung betrifft. Es nimmt die Änderung an, wenn jede neue Adresse leer ist und an der alten nichts gespeichert war, oder wenn an jeder neuen Adresse dasselbe restic-Repository liegt wie an der alten. Alles andere lehnt es ab und nennt die Zahl der Backups, die noch an der alten Adresse liegen. Ein entfernter Ort behält seine Adresse; wenn du woandershin sichern willst, verbinde die neue Stelle als eigenen Ort.

BombVault baut die Liste der Orte aus seiner eigenen Datenbank und liest dafür nie ein entferntes Repository aus; der Test läuft nur, wenn du etwas änderst.

## Einen Ort entfernen {#remove}

Ein Ort lässt sich nur entfernen, solange ihn nichts benutzt: Kein Bereich ist dort gespeichert, keine Vorgabe zeigt auf ihn, kein Element ist dort gespeichert, und kein direktes Repository an ihm hält Elemente. Sonst nennt die Ablehnung, was ihn hält. Mit dem Ort verschwinden seine Kopieziele und auch seine eigenen Zugangsdaten, sofern keine Abhol-Quelle und kein anderer Ort sie benutzt. Im Speicher selbst wird nichts gelöscht, und die Bestätigung sagt, wie viele Kopien dort zurückbleiben.

## Ohne Ort {#without-a-place}

Eine Adresse, die nicht in die Form Ort plus Ordner passt, arbeitet weiter und steht mit ihrer Adresse unter **Ohne Ort**. Native `b2:`-, `gs:`- und `swift:`-Adressen gehören dazu. **Einem Ort zuordnen** hängt so eine Zeile an einen Ort, nach demselben Test wie beim [Ändern einer Adresse](#addresses). Ein Kopieziel ohne Ort steht außerdem in der Zeile seines Bereichs neben den Chips und kopiert weiter. Eine entfernte Zeile dort hat einen eigenen Schalter **Append-only**. Wer ihn ausschaltet, wird vorher gefragt und erfährt, wie viele Einträge an dieser Adresse Backups haben. Ein Direkt-Repository folgt dem Schalter seines Ziels.

## Außer Haus {#off-the-premises}

**Wo steht das Gerät?** hat zwei Antworten: **Hier im Haus** und **An einem anderen Ort**. Für die 3-2-1-Zeile auf den Karten und für die Off-site-Prüfungen im Dashboard zählt eine Kopie nur dann als eigener Standort, wenn ihr Ort an einem anderen Standort steht. Eine zweite Platte oder ein NAS im selben Haus ist eine zweite Kopie, aber kein zweiter Standort. Die Antwort ändert keine Kopie. Cloud-Anbieter stehen immer an einem anderen Standort und ein Ordner auf diesem Unraid immer hier, deshalb fragt das Formular bei ihnen nicht nach; bei jedem anderen Ort änderst du die Antwort in seinen Details. Ein Ort an einem anderen Standort trägt in seiner Zeile die Markierung **Anderer Standort**.

## Verbindungsarten

### Ordner auf diesem Unraid oder einem NAS {#kind-local}

Die Adresse ist ein Pfad unter `/mnt`, geschrieben ohne `/mnt`, zum Beispiel `user/bombvault`, und der Ordner jedes Bereichs liegt darunter: `user/bombvault/container`.

- **Ordner auf diesem Unraid** wählt aus den Freigaben, Platten und Pools.
- **Synology**, **QNAP**, **TrueNAS**, **Anderer Unraid** und **Andere Freigabe** wählen aus `/mnt/remotes`. Häng die Freigabe zuerst in Unraid ein, zum Beispiel mit dem Plugin Unassigned Devices. Host Data muss als Read/Write - Slave eingehängt sein, sonst bleibt eine Freigabe, die erst nach dem Start von BombVault eingehängt wird, bis zu einem Neustart unsichtbar (siehe [Konfiguration](configuration.md)).

Die Ordnerauswahl legt mit **Neuer Ordner** dort, wo sie gerade steht, einen Ordner an. Der Test prüft, dass der Ordner leer ist oder fehlt und dass BombVault dort schreiben darf.

### S3 {#kind-s3}

Die Adresse ist `s3:https://<endpoint>/<bucket>/<path>`, zum Beispiel `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** braucht nur die Schlüssel-ID und den Anwendungsschlüssel. BombVault fragt B2, auf welchen Bucket, welchen S3-Endpunkt und welchen Ordner der Schlüssel beschränkt ist, und baut daraus die Adresse. Darf der Schlüssel jeden Bucket erreichen, bietet er seine Buckets zur Auswahl an.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** und **Vultr** fragen nach dem Schlüssel und, wo der Anbieter sie braucht, nach Region, Konto-ID oder Endpunkt. BombVault trägt den Endpunkt ein und listet die Buckets, wenn der Schlüssel sie auflisten darf; sonst tippst du den Namen des Buckets ein.
- **Google Cloud Storage** läuft über seine S3-Schnittstelle mit einem HMAC-Schlüssel, den du in den Cloud-Storage-Einstellungen unter Interoperabilität anlegst. Eine Datei für ein Dienstkonto funktioniert hier nicht.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** und **Anderer S3-Dienst** brauchen die Adresse des Dienstes und einen Schlüssel.

Die Speicherklasse stellst du in den Details des Ortes ein, beschränkt auf Stufen, die eine Wiederherstellung ohne vorheriges Auftauen lesen kann.

### rest-server {#kind-rest}

Die Adresse ist `rest:<url>/<user>`, zum Beispiel `rest:https://nas.lan:8000/tower`. Das Formular fragt nach der Adresse des Servers, einem Benutzer und einem Passwort. Mit `--private-repos` darf ein Benutzer nur Pfade erreichen, die mit seinem eigenen Namen beginnen, deshalb stellt BombVault den Benutzer voran, sofern du keinen anderen Pfad eintippst. Lehnt der Server einen Pfad ab, der nicht dem Benutzer gehört, sagt die Fehlermeldung das.

Das rest-server-Formular enthält ein fertiges Rezept für einen rest-server im Append-only-Modus mit einem Benutzer für dieses BombVault. **Rezept zeigen** erzeugt ein Passwort, das nur einmal angezeigt wird, und liefert eine `docker run`-Zeile, eine Compose-Datei und eine Unraid-Vorlage, jeweils mit der `htpasswd`-Zeile für den Server; Benutzer und Passwort landen direkt im Formular.

**Anderes BombVault** listet über seinen eigenen Feldern die offenen Angebote, die andere Instanzen über die Flotte geschickt haben. Wer eines annimmt, fügt einen Ort hinzu, der nur Kopien des angebotenen Bereichs aufbewahrt, weil ein Angebot einen Benutzer für genau diesen einen Bereich mitbringt. Das Annehmen auf der Seite Flotte fügt denselben Ort hinzu.

### SFTP {#kind-sftp}

Die Adresse ist `sftp://<user>@<host>:<port>/<path>`, zum Beispiel `sftp://bv@backup.lan:22/bombvault`. Das Formular fragt nach Host, Port und Benutzer und zeigt den öffentlichen Schlüssel von BombVault. Trag diesen Schlüssel auf dem Server in die `~/.ssh/authorized_keys` des Benutzers ein; sonst muss dort nichts installiert sein. BombVault nimmt den Host-Key des Servers beim ersten Kontakt an und prüft ihn von da an.

**Hetzner Storage Box** trägt `<user>.your-storagebox.de` und Port 23 ein. Installier den Schlüssel auf der Box mit Hetzners eigenem Befehl, der einmal nach dem Passwort der Box fragt:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

Das Formular fragt nach der Adresse des Servers, dem Benutzer und einem App-Passwort. Leg das App-Passwort in den Sicherheitseinstellungen des Kontos an, und gib die Benutzer-ID ein, keine E-Mail-Adresse. BombVault baut den WebDAV-Pfad, den das Produkt benutzt, und übergibt die Verbindung über die Umgebungsvariablen von rclone an restic, mit dem Passwort in der von rclone verschleierten Form. Die Adresse lautet `rclone:bvp<id>:<path>`, wobei `bvp<id>` ein Remote ist, das nur in dieser Umgebung existiert; in die rclone-Konfiguration wird nichts geschrieben.

### Azure Blob {#kind-azure}

Die Adresse ist `azure:<container>:/<path>`. Das Formular fragt nach dem Speicherkonto und seinem Zugriffsschlüssel; nach **Verbindung testen** listet es die Container des Kontos zur Auswahl, oder du tippst den Namen eines Containers ein. BombVault übergibt Konto und Schlüssel an restic als `AZURE_ACCOUNT_NAME` und `AZURE_ACCOUNT_KEY`.

### rclone {#kind-rclone}

Die Adresse ist `rclone:<remote>:<path>`. Das Formular listet die Remotes aus der rclone-Konfiguration von BombVault zur Auswahl. Um diese Konfiguration zu ersetzen, füg eine ganze `rclone.conf` unter **rclone-Konfiguration** ein und klick auf **Konfiguration speichern**. Sie ist dann sofort gespeichert und gilt für jeden rclone-Ort, egal ob das Fenster danach einen Ort hinzufügt oder nicht.

## Kopien zwischen Orten mit unterschiedlichen Zugangsdaten {#different-credentials}

Ein Bereich, der an einem entfernten Ort gespeichert ist, ist die Quelle seiner Kopien. `restic copy` läuft mit einer einzigen Umgebung, und BombVault ergänzt die Zugangsdaten des Ziels um die der Quelle, solange die beiden nicht dieselbe Variable auf unterschiedliche Werte setzen. Ein Nextcloud-Ort und ein B2-Ort benutzen unterschiedliche Variablen, ein in Nextcloud gespeicherter Bereich lässt sich also nach B2 kopieren. Zwei S3-Konten oder zwei rest-server-Benutzer bräuchten dieselben Variablen mit unterschiedlichen Werten; restic kann nicht beide aufnehmen, und der Chip auf der Karte Domänen sagt, dass die Zugangsdaten nicht zusammenpassen.
