# Off-site & Wiederherstellung

!!! note "Off-site-Kopien warten nach einem Neuaufbau"
    Baut der Notfall-Assistent oder **Sicherungen entdecken** Einträge ohne die alten Einstellungen neu auf, pausieren die Off-site-Kopien dieser Bereiche, bis ihre Vorgabe bestätigt ist. Siehe [Nach einem Neuaufbau](#after-a-rebuild).

Lokale Sicherungen schützen dich vor einem verlorenen Container oder einem missratenen Update. Off-site-Kopien und ein getestetes Recovery-Kit schützen dich vor dem Verlust der ganzen Box, vor Ransomware oder einem Brand. Diese Seite behandelt Speicherorte und Off-site-Kopien, den Schutz einer Kopie vor Manipulation, den Nachweis, dass du wiederherstellen kannst, das Koppeln mehrerer Server und die Wiederherstellung, wenn BombVault selbst weg ist.

## Off-site-Kopien {#off-site-copies}

Behalte die schnelle lokale Sicherung und füge einen oder mehrere Speicherorte außer Haus hinzu. BombVault kopiert neue Snapshots mit `restic copy` auf Best-Effort-Basis dorthin, sodass ein Off-site-Aussetzer die lokale Sicherung nie fehlschlagen lässt. So bleibt das lokale Repo primär und das Off-site-Repo eine Kopie, das primäre Repo eines Bereichs muss aber gar nicht lokal sein; siehe [Entfernte primäre Repositories](#remote-primary-repositories) weiter unten, um direkt auf S3, einen rest-server und Ähnliches zu sichern, statt dorthin zu kopieren.

- **Mehrere Speicherorte pro Bereich.** Jeder Bereich (Container, VMs, Flash, Selbstbackup, Ordner und ZFS-Datasets) kann gleichzeitig auf mehrere Speicherorte außer Haus kopieren, sodass du zum Beispiel einen rest-server bei einem Freund und einen S3-Bucket parallel halten kannst. Die Karte **Speicherorte je Bereich** auf der Seite Speicherorte schaltet jede Linie von einem Bereich zu einem Ort ein oder aus.
- **Wann Kopien laufen.** Die Karte **Kopieren** eines Orts legt **Wann kopieren** fest: **Nach jeder Sicherung** oder nach einem eigenen Plan, der die Snapshots sammelt und zu festen Zeiten kopiert. Unter **Einstellungen, Zeitplan, Off-site-Kopien pro Bereich** gibt **Ausnahme** einem Bereich einen eigenen Kopierplan. **Jetzt replizieren** in der Karte **Off-site-Kopie** eines Bereichs unter **Einstellungen, Speicherorte** startet eine Kopie sofort.
- **Die Aufbewahrung** stellst du pro Speicherort unter **Wie lange aufheben** ein, sodass du Off-site-Kopien länger als Archiv behalten kannst. Die Kopie im Off-site-Feld eines Bereichs folgt der Karte **Off-site-Aufbewahrung** unter **Einstellungen, Speicherorte**; lass sie ganz auf null, um Off-site-Snapshots nie automatisch zu kürzen.
- **Bandbreitenlimits** (**Off-site-Bandbreite** unter **Einstellungen, Speicherorte** oder die Karte **Kopieren** eines Orts) begrenzen die restic-Upload-/Download-Rate, damit Kopien dein WAN nicht auslasten.
- Während eine Kopie läuft, sagt der Statuskopf der Übersicht **Off-site-Kopie läuft**. Das ist eine Aktivitätsanzeige, kein Prozentbalken, weil `restic copy` keinen maschinenlesbaren Fortschritt liefert.

!!! note "Von jedem Ort wiederherstellen"
    Jeder Eintrag listet seine Sicherungen im Reiter **Sicherungen** als eine Zeitleiste über alle Orte, an denen eine Sicherung liegt. Eine nach B2 kopierte Sicherung erscheint einmal, markiert mit jedem Ort, der sie hält. Eine Wiederherstellung nimmt den ersten Ort, den sie erreicht, beginnend mit dem Repository, in das der Eintrag geschrieben wird, und du kannst pro Snapshot einen anderen Ort wählen. Speicherorte außer Haus werden erst gelesen, wenn du **Prüfen** klickst, weil das Auflisten von Cloud-Speicher eine Weile dauert. Das Löschen an einem Ort prüft zuerst die anderen und sagt, ob es die letzte Kopie war.

## Speicherorte {#destinations}

Ein Speicherort ist ein Ort, an den Sicherungen gehen: ein lokaler Ordner, eine Freigabe, ein eigener Server oder ein Cloud-Dienst. Er wird einmal eingerichtet und dient dann jedem Bereich. Öffne **Speicherorte** über ihre Einstellungskachel oder schalte dort **Auch in der Seitenleiste zeigen** ein, um sie anzuheften.

**Speicherort hinzufügen** öffnet ein Fenster mit vier Schritten:

1. **Anbieter.** Jeder Dienst steht mit seinem Logo da, mit einer Suche darüber, in Gruppen: Speicherdienste mit S3-Buckets (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 und andere), eigene S3-Server (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), eigener Server und Freigaben (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, ein eingehängter Pfad), Cloud-Speicher (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud und der Rest, den rclone kann) und die Empfangsserver deiner Gruppe. Jeder sagt, wie gut er sich für Sicherungen eignet: Cloud-Laufwerke bremsen bei vielen Anfragen, die erste Sicherung und das Aufräumen dauern dort also länger.
2. **Anmelden.** Die Felder hängen vom Dienst ab: ein Zugangsschlüssel für S3, Benutzer und Passwort für WebDAV und SMB, ein App-Passwort, wo die Zwei-Faktor-Anmeldung die normale blockiert, BombVaults öffentlicher SSH-Schlüssel für SFTP und die Storage Box oder ein Token für Dienste, die sich über einen Browser anmelden. Für diese zeigt das Fenster einen `rclone authorize`-Befehl, den du auf einem Rechner mit Browser ausführst; das ausgegebene Token kommt ins Feld. **Verbindung testen** prüft die Anmeldung, bevor etwas gespeichert wird. Danach listet das Fenster die Ordner am Speicherort, mit **Neuer Ordner** zum Anlegen und dem freien Platz, wo der Dienst ihn meldet. Ein leerer Ordner ist am sichersten; BombVault legt darin einen Ordner pro Bereich an.
3. **Schutz.** Das Fenster sagt offen, was der Dienst kann. Ein rest-server im Append-only-Modus verweigert das Löschen, und der Manipulationstest prüft das. Ein S3-Bucket kann mit Versionierung und Object Lock alte Versionen halten, was BombVault noch nicht prüfen kann. Ein Cloud-Laufwerk kann das Löschen gar nicht verweigern: Wer in den Server kommt, kommt auch an diese Kopie. Schalte **Löschschutz (append-only)** nur ein, wo die Gegenseite das Löschen wirklich verweigert; BombVault räumt dort dann nie auf. **Für den Ernstfall** sagt, was das Recovery-Kit zu diesem Ort enthält.
4. **Für welche Bereiche.** Wähl die Bereiche, die hier sichern. Die erste Kopie bringt den ganzen Verlauf mit. Einträge, die ihre Speicherorte selbst gewählt haben, bekommen den neuen nicht von selbst, und das Fenster nennt sie.

S3-Dienste laufen über restics eigenes S3-Backend, und nur damit greifen Speicherklasse und Object Lock. Jeder andere Dienst läuft über das rclone, das BombVault mitbringt, und sein Remote erscheint dann in der Karte **Off-site (rclone)** unter **Einstellungen, Speicherorte**. Ein Einstellungsexport enthält die Speicherorte; mit Zugangsdaten enthält er auch ihre Anmeldung.

Jeder Speicherort hat eine eigene Seite:

- **Verbindung** mit seinem Namen, dem Schalter **Verfügbar** (aus heißt, BombVault schreibt hier nicht mehr, und Einträge, die hierher sichern, schlagen mit einer klaren Meldung fehl, statt woanders zu landen), dem Ordner, der Kompression, der Speicherklasse und **Außer Haus**. **Verbindung ändern** testet und speichert eine neue Adresse oder Anmeldung.
- **Zugangsdaten**: der geteilte Satz oder ein eigener.
- **Löschschutz**, mit **Löschschutz jetzt testen** und der Angabe, wer diese Kopie aufräumt.
- **Kopieren**: wann kopiert wird und die Upload- und Download-Grenzen.
- **Wie lange aufheben**: **Kurz**, **Ausgewogen**, **Lang** oder **Eigene Werte**, mit einer Zeitleiste, welche Sicherungen die Regel behält. **Vor dem Verkürzen nachfragen** fragt, bevor eine kürzere Regel Sicherungen löscht, deren einzige Kopie hier liegt.
- **Regeln je Bereich**: das eigene Repository, die eigene Aufbewahrung oder Kompression eines Bereichs an diesem Ort. **Regeln ändern** öffnet sie, und **Auf den Speicherort zurücksetzen** holt einen Bereich zurück.
- **Liegt schon unter diesem Speicherort**: Bereiche, deren von Hand eingetippte Adresse in einem Ordner dieses Orts liegt. Eine Übernahme lässt das Repository, wo es ist; Anmeldung, Speicherklasse und Löschschutz kommen dann vom Ort. BombVault prüft vorher, ob die Anmeldung des Orts das Repository öffnet, und hängt ein Append-only-Repository nie unter einen Ort, der nicht append-only ist.
- Ein Gefäß, das zeigt, wie voll der Ort ist, mit der Marke, wo er voll ist, und wie viele Wochen beim Wachstum der letzten Wochen bleiben. Ein Ort, der seine Größe nicht meldet, zeigt **Größe unbekannt**.

Ein Empfangsserver, den eine andere Instanz deiner Gruppe betreibt, erscheint unter den Anbietern als **Aus deiner Gruppe**; siehe [Empfangsserver](#receiving-server).

### Umziehen und entfernen {#move-remove}

- **Umziehen** bringt die Kopie eines Bereichs samt ganzem Verlauf an einen anderen Speicherort. Am alten Ort bleibt alles liegen; BombVault löscht dort nichts. Der Umzug läuft im Hintergrund und kann dauern: etwa 9 Minuten pro 100 GB zwischen lokalen Platten, Stunden über eine langsame Leitung. Die Übersicht zeigt ihn, solange er läuft.
- **Speicherort entfernen** bietet zwei Wege. **Speicherort entfernen, Sicherungen behalten** hört auf, dort zu schreiben, und lässt die Sicherungen liegen; du kannst sie später unter **Wiederherstellung, Aus einem alten Sicherungsordner** lesen. **Speicherort entfernen und Sicherungen dort löschen** liest zuerst, was dort liegt, und listet es auf: welche Sicherungen gehen, welche davon nirgends sonst liegen und welche Repositories bleiben, weil der Löschschutz es verweigert, Einträge noch direkt dorthin sichern, ein anderer Ort oder Bereich dort ebenfalls Kopien hält oder sie nicht lesbar waren. Einträge, die ihre einzige Sicherung verlören, stehen eigens da und bleiben, wenn du nicht **Auch diese löschen** anhakst. Du bestätigst, indem du den Namen des Orts eintippst, und das Löschen läuft im Hintergrund.
- Die Sicherungen an einem Speicherort zu löschen braucht ein Login-Passwort und eine Anmeldung an der Instanz. Ohne Passwort lässt sich der Ort trotzdem entfernen, nur seine Sicherungen lassen sich von hier aus nicht löschen.

## Speicherort pro Eintrag {#placement}

Die Karte **Zeitplan und Speicherort** im Reiter Einstellungen eines Eintrags hat eine Zeile mit Knöpfen: **Lokal** und einer pro Speicherort außer Haus des Bereichs. Die eingeschalteten bekommen die Sicherungen des Eintrags.

- Mit eingeschaltetem **Lokal** wird der Eintrag in das Repository geschrieben, das unter **Gespeichert auf** steht, und auf jeden anderen eingeschalteten Ort kopiert. Schaltest du einen Ort aus, bekommt er von diesem Eintrag nichts Neues mehr. Lokal allein kopiert nirgendwohin, was für Daten passt, die schon eine zweite Kopie haben, zum Beispiel eine Freigabe auf einem NAS.
- Mit ausgeschaltetem **Lokal** wird der Eintrag direkt in das Direkt-Repository des ersten eingeschalteten Orts geschrieben und von dort auf die anderen eingeschalteten kopiert. Beim ersten Mal legt ein Dialog dieses Direkt-Repository an.
- Ein Knopf bleibt eingeschaltet, weil eine Sicherung einen Ort braucht. Um einen Eintrag aus den Sicherungen herauszunehmen, nimm **Nicht sichern**.

Der Ort ist ab der ersten Sicherung des Eintrags fest, weil BombVault Sicherungen nie zwischen Repositories verschiebt; die Karte **Repository** im Reiter Einstellungen nennt ihn. Die Kopien können sich jederzeit ändern. Ein Ort, der einen Eintrag nicht mehr bekommt, behält die Kopien, die er hat, und kürzt sie beim nächsten Off-site-Lauf des Bereichs nach seiner eigenen Aufbewahrung. Von Append-only-Orten kann nicht gelöscht werden.

Unter der Zeile sagt die Karte, wohin der Eintrag geht und was tatsächlich da ist: an wie vielen Standorten er liegt, wann jeder Ort zuletzt gesehen wurde und ob 3-2-1 erfüllt ist. Ein Standort ist der Server mit den Originaldaten, jeder Ort außer Haus und jedes als **Außer Haus** markierte Repository. BombVault prüft Kopien und Standorte; den Teil "zwei Medien" von 3-2-1 prüft es nicht.

### Vorgaben je Bereich {#placement-defaults}

Die Karte **Speicherorte je Bereich** auf der Seite Speicherorte ist die Vorgabe jedes Bereichs. In der Grafik ist jede Linie eine Kopie, und ein Klick auf eine Linie oder einen Punkt schaltet sie ein oder aus; gestrichelte Linien sind aus. **Grafik / Schalter** zeigt dasselbe als Knopfreihen. Die Kopien gelten sofort für jeden Eintrag ohne eigene Wahl und für die Projektordner von Compose-Stacks. Der Ort gilt für einen neuen Eintrag ab seiner ersten Sicherung; eine Änderung verschiebt keine Sicherungen. Vor dem Speichern nennt BombVault jeden Ort, der Einträge gewinnt oder verliert, und wie viele Snapshots das bedeutet.

Einträge, die woanders sichern als ihr Bereich, stehen in der Karte **Eigene Wahl**, jeder mit **Vorgabe übernehmen**. Neue Sicherungen gehen dann an die Speicherorte des Bereichs, und die früheren bleiben, wo sie sind.

Ein neuer Ort außer Haus bekommt jeden Eintrag, der nicht auf Lokal steht. Das Fenster, das ihn hinzufügt, sagt, wie viele Einträge und, wo bekannt, wie viel Verlauf das ist.

### Direkt-Repositories

Schaltest du Lokal für einen Eintrag aus, sodass ein Ort ohne Direkt-Repository sein Zuhause wird, öffnet sich ein Dialog mit einem vorgeschlagenen Ort daneben, zum Beispiel `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, und einem Verbindungstest, der nichts anlegt. **Anlegen und nutzen** legt das Repository an und richtet den Eintrag darauf. Ein Direkt-Repository übernimmt Schlüssel, Speicherklasse, Grenzen, Append-only-Einstellung und Aufbewahrung des Orts und ändert sich mit ihnen; die Karte Repositories zeigt es nur lesend. Kann ein neuer Schlüssel des Orts es nicht öffnen, behält das Direkt-Repository seinen Schlüssel, und das Speichern sagt das. Ein Eintrag auf einem Direkt-Repository wird von dort auf die anderen eingeschalteten Orte kopiert, nie auf den Ort, zu dem das Repository gehört. Seine Snapshots tragen die Marke `bv:direct`, und jeder andere Aufbewahrungslauf lässt sie stehen, sodass ein Direkt-Repository, das die Verbindung zu seinem Ort verloren hat, nie nach den lokalen Regeln altert. B2 wird über seinen S3-Endpunkt angesprochen, mit Schlüssel-ID und Anwendungsschlüssel als S3-Zugangsdaten; ein Schlüssel, der auf den Ordner des Orts beschränkt ist, erreicht den Ordner daneben nicht, beschränke den Schlüssel also auf den Ordner darüber.

### Außer Haus

Ein Speicherort oder ein benanntes Repository kann als **Außer Haus** markiert werden. Entfernte starten markiert; schalte es für einen rest-server im selben Gebäude aus. Die Markierung zählt nur Standorte und 3-2-1 auf den Karten. Sie ändert keine Kopie.

Eine ZFS-Replik ist eine Kopie außer Haus, aber keine Sicherung mit Verlauf. Sie behält nur die Snapshots, die du einstellst, und alte Versionen und einzelne Dateien kommen weiter aus den Sicherungen. Sie geht direkt von einem ZFS-Server zum anderen, über SSH an einen ZFS-Server oder an den Empfangs-Endpunkt einer gekoppelten Instanz. Die Kopplungsgruppe trägt nur die Anfrage. Siehe [Replik](zfs-datasets.md#replica).

### Nach einem Neuaufbau {#after-a-rebuild}

Die Kopierwahl liegt in BombVaults eigenen Einstellungen. Nach einem Neuaufbau über **Sicherungen entdecken** oder den Notfall-Assistenten ohne wiederhergestellte Einstellungen ist sie weg, und alles zu kopieren würde die Einträge, die du ausgelassen hattest, wieder nach B2 schicken. Deshalb pausieren die Off-site-Kopien jedes neu aufgebauten Bereichs. Die Übersicht sagt das, und die Karte **Speicherorte je Bereich** bietet **Vorgabe bestätigen** an, mit einer Vorschau dessen, was der nächste Lauf kopiert, und den Namen in den Sicherungen, die keinen Eintrag haben, die du dort auslassen kannst. Nur die Bestätigung beendet die Pause; ein Import einer Einstellungsdatei bringt Regeln und Vorgaben zurück, beendet sie aber nicht.

## Entfernte primäre Repositories {#remote-primary-repositories}

Der eigene Pfad eines Bereichs ist nicht auf einen lokalen Ordner beschränkt: Richte ihn direkt auf ein restic-Remote (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`), und BombVault sichert direkt dorthin, ohne eigene lokale Kopie und ohne Kopierschritt. Das ist eine andere Form als die Off-site-Kopien oben: Dort ist das lokale Repo primär und das Off-site-Repo ein Best-Effort-Archiv davon; hier **ist** das entfernte Repo das primäre, und es ist die einzige Kopie, solange du den Bereich nicht zusätzlich auf einen anderen Speicherort kopierst.

Den Pfad jedes Bereichs (Container, VMs, Flash, Selbstbackup, Ordner, ZFS-Datasets) setzt du auf der Seite des Speicherorts **Lokaler Ordner**: **Ändern** neben dem Bereich in seiner Karte **Verbindung** öffnet ein Fenster mit einem Schalter **Lokal / Remote**:

- **Lokal** zeigt die gewohnte Ordnerauswahl.
- **Remote** tauscht sie gegen ein schlichtes URL-Feld und die Verbindungseinstellungen, die ein Ort außer Haus nutzt, hier für dieses primäre Repo. Von dort bekommst du:
    - **Einen Verbindungstest** gegen den echten Pfad, bevor du dich darauf verlässt.
    - **Bandbreitenlimits** (Upload/Download), damit eine geplante Sicherung an ein entferntes primäres Repo dein WAN nicht auslastet: dieselben restic-Flags `--limit-upload`/`--limit-download` wie bei Off-site-Kopien, angewendet auf die Sicherung selbst.
    - **Append-only-Schutz**, geprüft mit demselben aktiven Manipulationstest (ein echter DELETE-Versuch gegen die Gegenseite) wie bei Orten außer Haus. Ist er an, räumt BombVault das Repo selbst nie auf, denn dahinter liegt keine eigene lokale Kopie, und die Zugangsdaten auf dieser Box dürfen die einzige Kopie der Sicherung nicht löschen können.
    - **Einen Wachstumsbudget-Alarm**, abgeleitet aus demselben Größenverlauf, den die Karte Speicher ohnehin verfolgt.

Nichts davon ist Pflicht: Ein von Hand eingetippter entfernter Pfad ohne gespeicherte Schutzeinstellungen sichert unbegrenzt, mit Aufräumen und ohne Budget-Alarm. Die Schutzeinstellungen sind für den Fall da, dass du denselben Schutz wie bei einer Off-site-Kopie willst, ohne dafür eigens einen Speicherort anzulegen.

!!! note "Cloud-/REST-Zugangsdaten sind geteilt"
    Ein entferntes primäres Repo meldet sich mit den geteilten Cloud-Zugangsdaten unter **Einstellungen, Speicherorte** an; einen eigenen Zugangsdaten-Speicher für primäre Repos gibt es nicht.

### SMB und WebDAV ohne Host-Mount {#smb-webdav}

Die Karte **Off-site (rclone)** unter **Einstellungen, Speicherorte** hat ein Formular für eine Windows- oder Samba-Freigabe und für einen WebDAV-Server (Nextcloud, ownCloud, SharePoint oder einen anderen). Trag einen kurzen Namen, Host und Freigabe (SMB) oder URL und Servertyp (WebDAV), Benutzer und Passwort ein, und BombVault schreibt den rclone-Abschnitt für dich. rclone verschleiert das Passwort selbst, bevor es gespeichert wird; ein Eintrag mit einem schon vorhandenen Namen ersetzt diesen Abschnitt, statt einen zweiten anzulegen. **Speicherort hinzufügen** bietet SMB und WebDAV mit denselben Feldern als Anbieter an.

Das Formular antwortet mit dem fertigen Ort, zum Beispiel `rclone:nas:backups`. Nimm ihn als Pfad eines Bereichs oder als Speicherort und häng einen Unterordner an, wenn du einen willst (`rclone:nas:backups/bombvault`). Die Freigabe ist das erste Pfadsegment, nicht Teil des Namens.

Das ist der bessere Weg, als die Freigabe auf Unraid einzuhängen: restic rät davon ab, ein Repository auf einer eingehängten CIFS-Freigabe zu halten, und hier ist nichts eingehängt. NFS ist nicht im Formular, weil weder restic noch rclone ein NFS-Backend haben; häng für NFS den Export auf dem Host ein und nimm ihn als lokalen Ordner.

## Löschschutz (append-only)

Schalte den Löschschutz für einen Speicherort außer Haus ein, damit Ransomware oder ein kompromittierter Host deine Sicherungen nicht löschen oder überschreiben kann. Die Gegenseite (ein `restic/rest-server` im `--append-only`-Modus) **erzwingt** ihn. BombVault **prüft** ihn nur nach und zeigt nie grün allein auf eine Konfigurationsbehauptung hin. Ein Ordner auf diesem Server kann das Löschen nicht verweigern, weil, wer BombVault steuert, auch den Ordner steuert, deshalb bieten lokale Orte den Schalter nicht an.

Der schnellste Weg zu einer Append-only-Kopie ist ein gekoppeltes BombVault mit einem [Empfangsserver](#receiving-server): Es betreibt den rest-server für dich, im Append-only-Modus.

!!! note "Ein erfolgreiches Löschen unter `/locks/` ist zu erwarten"
    Append-only heißt nicht, dass nie etwas entfernt werden kann. restic muss seine eigenen Locks setzen und wieder freigeben, deshalb bleibt `/locks/` beschreibbar und löschbar. Snapshots und die Daten dahinter, auf die es Ransomware abgesehen hätte, lassen sich nicht entfernen. Prüfst du die Gegenseite selbst, ist ein erfolgreiches Löschen unter `/locks/` richtiges Verhalten und keine Lücke im Schutz.

!!! warning "Append-only-Repos werden von dieser Box aus nie aufgeräumt"
    Ein Append-only-Ort räumt alte Snapshots von hier aus nie auf; das ist Aufgabe der Gegenseite. Setz dafür einen **Wachstumsbudget-Alarm**, damit du gewarnt wirst, bevor die Repo-Größe davonläuft.

## Manipulationstest

BombVault beweist die Append-only-Garantie regelmäßig, indem es tatsächlich einen Löschversuch gegen das Off-site-Repo unternimmt, gezielt auf ein nicht existierendes Objekt. **Löschschutz jetzt testen** auf der Seite eines Orts führt ihn sofort aus, und **Löschschutz prüfen** unter **Einstellungen, Prüfung, Automatische Prüfungen** nach Plan.

- **Verweigert** heißt geschützt.
- **Angenommen** heißt nicht geschützt.
- Ein **unschlüssiges** Ergebnis (Server nicht erreichbar, Anmeldefehler) kippt das gespeicherte Urteil nie.

Ein echter Wechsel von geschützt zu ungeschützt löst einen einzigen Alarm aus.

## DR-Übungen

BombVault bietet zwei Stufen des Nachweises, dass deine Sicherungen wirklich wiederherstellbar sind und nicht nur vorhanden.

- **Wiederherstellungs-Prüfübungen (lokal).** BombVault führt regelmäßig `restic check --read-data-subset` aus (begrenzt, nie eine plattenfüllende Vollwiederherstellung). Der Rhythmus steht unter **Einstellungen, Prüfung, Automatische Prüfungen**, und die Karte **Sicherungen prüfen** auf derselben Kachel zeigt das Ergebnis.
- **Probe-Wiederherstellung (Off-site).** BombVault stellt ein echtes Ziel aus dem Off-site-Repo in eine abgeschottete Sandbox wieder her, prüft es Datei für Datei und Byte für Byte und räumt dann auf. Das beweist, dass du aus dem Off-site wiederherstellen kannst, nicht nur, dass das Repo antwortet.

Die Karte **Schutzstatus** auf der Übersicht fasst das pro Bereich in vier Prüfungen zusammen: eine aktuelle Sicherung auf diesem Server, eine Kopie außer Haus, Löschschutz dort und eine Probe-Wiederherstellung von dort. Ein Klick auf einen Bereich zeigt die Einzelheiten, und die Karte wird nur bei verifizierten Fakten grün.

## Instanzen koppeln {#pairing}

Mehrere BombVaults arbeiten als eine Gruppe zusammen: Sie zeigen einander ihren Schutzstatus, übernehmen Rollen füreinander und können den Schlüssel des anderen verwahren. Eine Instanz tritt der Gruppe mit zwölf Wörtern bei.

Öffne auf der ersten Instanz **Einstellungen, Kopplung** und drück **Phrase generieren**. Drei Schrittkarten zeigen den Ablauf. Zwölf Wörter erscheinen in einem Fenster mit einem Knopf **Kopieren**. Öffne auf jeder anderen Instanz dieselbe Stelle, drück **Phrase eingeben** und füge sie ein oder tippe sie ab, oder drück **Einfügen** in diesem Fenster. Ein Wort, das nicht auf der Liste steht, wird schon beim Tippen mit seiner Position genannt, und das letzte Wort trägt eine Prüfsumme, sodass ein vertipptes oder vertauschtes Wort auffällt, bevor gekoppelt wird. Generiere die Phrase nur auf einer Instanz: Zwei Instanzen, die beide eine erzeugen, bilden zwei getrennte Gruppen. Taucht eine Minute lang niemand auf, bietet die Seite zwei Auswege: die Wörter noch einmal zeigen, um sie drüben einzugeben, oder die Wörter der anderen Instanz eingeben und ihrer Gruppe in einem Schritt beitreten. Die Kopplung funktioniert ohne Login-Passwort, aber setz eins: Ohne kann jeder, der die Weboberfläche öffnen kann, die Wörter lesen und über die Gruppe um Rollen bitten, die das restic-Passwort der Instanzen darin herausgeben. Die Kopplungskarte sagt das, bis ein Passwort gesetzt ist. Mit Passwort fragt das erneute Zeigen der Phrase danach. **Gruppe verlassen** nimmt eine Instanz wieder heraus.

Wer die Wörter kennt, kann der Gruppe beitreten, behandle sie also wie ein Passwort.

**Wie Mitglieder einander erreichen.** Jede Instanz lernt ihre eigene Adresse im Netz in dem Moment von deinem Browser, in dem du dich anmeldest; sie steht unter **Bekannte Adressen** auf der Kopplungsseite, zusammen mit denen, die die Instanz selbst gefunden hat. Korrigiere sie dort, wenn ein Reverse Proxy oder ein ungewöhnlicher Port davorsitzt. Im selben Netz geben Mitglieder diese Adresse per Multicast bekannt und sprechen direkt miteinander, und wo Multicast ein Container-Netz nicht durchquert, etwa Dockers Standard-Bridge, durchsucht eine Instanz stattdessen ihr eigenes Subnetz mit einem signierten Aufruf, den nur ein Gruppenmitglied beantworten kann, sodass die Kopplung auch ohne Relay in Sekunden fertig ist. Taucht nichts auf, nimmt **Findest du sie nicht?** eine Adresse von Hand, für ein anderes Subnetz oder einen ungewöhnlichen Port. Instanzen in verschiedenen Netzen gehen über ein Relay, das du auf derselben Seite wählst; jede seiner vier Formen hat ein Bild:

- **Projekt-Relay** (Vorgabe): `parleyport.halleluja.design`, das Relay, das auch KnightLoader nutzt. Nichts einzurichten.
- **Eigenes Relay**: der Container [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) aus den Unraid Community Apps (auch unter **Einstellungen, Apps**) oder eine deiner Instanzen, die schon von außen erreichbar ist, mit eingeschaltetem **Als Relay dienen**. Diese Instanz antwortet dann unter `/relay/connect` auf ihrer eigenen Adresse, hinter dem Reverse Proxy und Zertifikat, die sie schon hat, und lässt nur deine Gruppe herein. Trag die Adresse des Relays auf jeder Instanz ein, die es nutzen soll.
- **Kein Relay**: Mitglieder finden einander automatisch im selben Netz und nirgends sonst.

**Was das Relay sieht.** Jeder Aufruf zwischen Mitgliedern ist mit AES-256-GCM versiegelt, unter einem Schlüssel, der aus den zwölf Wörtern abgeleitet ist und deine Instanzen nie verlässt. Das Relay erfährt einen Hash, der die Verbindungen gruppiert, für welche Instanz eine Nachricht ist, wie groß sie ist und wann sie durchgeht. Ein direkter Aufruf im lokalen Netz ist genauso versiegelt und zusätzlich signiert, sodass nichts vom selbstsignierten Zertifikat einer Instanz abhängt. Aufrufe, die etwas herausgeben (Rollen-Antworten, ein restic-Passwort, Empfänger-Logins, Hinterlegungen), sind außerdem mit einem Schlüssel des sendenden Mitglieds signiert, den eine Instanz beim ersten Kontakt festhält; siehe [Sicherheitsmodell](configuration.md#security-model).

**Was über die Gruppe läuft.** Der Schutzstatus auf der Seite Instanzen, eine Bitte, einen Bereich jetzt zu prüfen, Rollen-Anfragen und ihre Antworten, Speicher-Angebote und was ein Empfänger oder Abholer braucht, sobald seine Rolle erlaubt ist: die Repository-Orte der anderen Instanz und ihr restic-Passwort. Sicherungsdaten nie; die gehen weiterhin direkt an die restic-Backends. Auch nicht der APP_KEY: Das restic-Passwort öffnet die Repositories dieser Instanz und sonst nichts, nicht ihre gespeicherten Geheimnisse, Sitzungen oder Notfallcodes.

**Einträge aus der Zeit vor der Kopplung.** Instanzen, die mit einem Fleet-Token hinzugefügt wurden, sowie empfangene Repositories und Abholquellen, die mit dem APP_KEY der anderen Instanz eingerichtet wurden, bleiben nach dem Update erhalten und sind mit **Neu koppeln** markiert. Sie laufen weiter: Beim ersten Start ersetzt BombVault jeden gespeicherten APP_KEY durch das daraus abgeleitete restic-Passwort. Kopple beide Instanzen, bearbeite dann den Eintrag und wähl seine Instanz. Bis dahin stehen solche Einträge in der Karte **Ohne Instanz** auf der Seite Instanzen.

Die eine Stelle, die noch einen APP_KEY von Hand nimmt, ist [Aus einem alten Sicherungsordner](#restore-from-another-bombvault-repo), für den Fall, dass die andere Instanz weg ist und in keiner Gruppe mehr antworten kann.

## Instanzen und Rollen {#roles}

Die Seite **Instanzen** (eine Einstellungskachel, die sich an die Seitenleiste heften lässt) zeigt die Gruppe als Netz, wer was für wen tut, eine Karte für diese Instanz und eine Karte für jedes andere Mitglied, die Android-Handys und die ZFS-Server. **Koppeln** schwebt unten rechts. Welche Teile an sind, legst du unter **Einstellungen, Instanzen, Mehrere Server** fest: **Instanzen**, **Empfänger** und **Holen**.

Jede Karte zeigt den Schutzstatus des Mitglieds und seine Rollen als Kacheln. Eine Rolle ist etwas, das eine Instanz für eine andere tut:

- **Empfänger** verwahrt die Off-site-Kopien dieses Servers in einem rest-server im Append-only-Modus. Dieser Server sendet nach jeder Sicherung und kann dort nichts löschen, selbst wenn ihn jemand übernimmt.
- **Abholer** holt die Sicherungen dieses Servers nach eigenem Zeitplan. Dieser Server muss dafür nicht wach sein.
- **ZFS-Server** nimmt die ZFS-Repliken dieses Servers. Jeder ZFS-Eintrag wählt ihn in seinen Einstellungen unter **Replik**.
- **Schlüsselhüter** verwahrt eine versiegelte Kopie des Recovery-Kits dieses Servers. Siehe [Schlüsselhüter](#key-keeper).

Das Einschalten einer Rolle schickt eine Anfrage, und nichts beginnt, bis jemand an der anderen Instanz **Erlauben** drückt. Die Anfrage erscheint auf der Karte dieser Instanz als Abzeichen, und ihre **Details** listen sie mit **Erlauben** und **Ablehnen**; eine erlaubte Rolle lässt sich dort jederzeit entziehen. Eine gestrichelte Linie im Netz ist eine Rolle, die noch wartet. Eine Instanz mit BombVault 9.x kann eine Anfrage nicht beantworten, deshalb ist eine Rolle dort aktiv, sobald du sie einschaltest, und eine Anfrage einer 9.x-Instanz wartet hier auf **Erlauben**.

**Details** auf einer Karte öffnet den Rest: die empfangenen Repositories und Abholquellen dieses Mitglieds, Speicher-Angebote, die Überwachung, **Entfernen** und **Schlüssel vergessen**.

## Empfänger und Abholer {#receiver}

![Eine empfangene Off-site-Kopie in den Details einer Instanz, schreibgeschützt überwacht, mit einer Integritätsprüfung auf dieser Hardware.](assets/screenshots/receiver.png)

*Eine empfangene Off-site-Kopie in den Details einer Instanz, schreibgeschützt überwacht, mit einer Integritätsprüfung auf dieser Hardware.*

Auf der Box, die Append-only-Kopien von einem anderen BombVault **empfängt**, geben dir die empfangenen Repositories eine unabhängige, schreibgeschützte Überwachung auf der empfangenden Hardware, damit ein stiller Fehler am anderen Ende nicht unbemerkt bleibt. Schalte **Empfänger** unter **Einstellungen, Instanzen** ein, öffne dann die **Details** der sendenden Instanz auf der Seite Instanzen und füge das empfangene Repository hinzu (schreibgeschützt, geöffnet mit dem restic-Passwort der sendenden Instanz, das sie über die [Kopplungsgruppe](#pairing) bekommt). Du bekommst:

- **Einen Bestand nach Quelle**, sodass du genau siehst, welche Container, VMs und Ordnersätze angekommen sind.
- **Zuletzt empfangen** pro Quelle, sodass du weißt, wie frisch jede ist.
- **Eine unabhängige `restic check`** auf der empfangenden Hardware, sodass die Integrität dort geprüft wird, wo die Daten wirklich liegen, nicht nur beim Sender.
- **Einen Totmannschalter:** einen Alarm, wenn eine Quelle innerhalb eines Fensters, das du setzt, aufhört zu senden.
- **Integritätsalarme:** einen Alarm, wenn eine Prüfung auf der empfangenden Seite fehlschlägt.

Der Empfänger ist strikt schreibgeschützt. Er schreibt nie in das empfangene Repository und kann so die Append-only-Garantie, auf die sich der Sender verlässt, nie brechen.

Ein Abholer arbeitet andersherum: Mit eingeschaltetem **Holen** liest eine Instanz die Sicherungen eines anderen Mitglieds nach eigenem Zeitplan in ihr eigenes Repository. Seine Quellen stehen ebenfalls in den Details dieses Mitglieds.

### Empfangsserver {#receiving-server}

Die empfangende Box kann auch den rest-server betreiben, auf den die anderen kopieren. Er wird eingerichtet, wenn du zum ersten Mal eine **Empfänger**-Anfrage erlaubst, die ihn braucht: Das Fenster sagt, dass der Server einen Empfangsserver braucht, und fragt nach einem Ordner auf einer Freigabe, mit **Neuer Ordner** zum Anlegen, und einem Port (8000, wenn kein anderer Container ihn belegt). BombVault dann:

1. lehnt ab, wenn es schon einen Container namens `rest-server` gibt oder ein anderer Container den Port hält;
2. zieht `restic/rest-server` und startet ihn über den Docker-Socket im Append-only-Modus mit privaten Repositories und einer Login-Datei im Ordner;
3. schreibt sein Unraid-Template auf den Flash, sodass der Container im Docker-Tab bearbeitbar bleibt, oder bietet das Template zum Herunterladen an, wenn der Flash nicht erreichbar ist;
4. führt den Manipulationstest dagegen aus und zeigt, ob er Löschen verweigert.

Die Instanzen deiner Gruppe finden den Server dann unter den Anbietern bei **Aus deiner Gruppe**, benannt nach der empfangenden Box, und die Rolle **Empfänger** bietet ihn als sichere Wahl an. Jede Instanz bekommt beim ersten Benutzen ein eigenes Login und schreibt dort nur in ihren eigenen Ordner. Mit eingeschaltetem **Empfänger** unter **Einstellungen, Instanzen** öffnet **Empfangsserver** auf der Karte dieser Instanz den Server wieder. Sein Fenster listet die Logins unter **Partner mit Login**, und **Login entziehen** nimmt eines weg; was diese Instanz schon kopiert hat, bleibt im Ordner. Die Einrichtung legt außerdem ein **Login außerhalb der Gruppe** an, dessen Passwort das Fenster einmal zeigt.

Eine Instanz, die die empfangende Box nur über das Relay erreicht, kann den Server nicht nutzen, weil das Relay keine Sicherungen trägt. Trag die Adresse der empfangenden Box vorher auf der Kopplungsseite ein. Läuft BombVault auf einer eigenen IP-Adresse (zum Beispiel auf br0), füll **Adresse für Partner** aus, weil der Server auf der Adresse des Hosts lauscht.

## Beispiel: zwei Unraid-Kisten, von Anfang bis Ende

Oben stehen die Einzelteile. Hier ist ein vollständiger Aufbau, weil sich Teile leichter zusammensetzen lassen, wenn man sie einmal zusammengesetzt gesehen hat.

Zwei Kisten: **TOWER** betreibt die Container und schiebt die Sicherungen, **VAULT** nimmt sie an und erzwingt append-only. Setze deine eigenen Namen, Adressen und Freigabepfade ein.

**1. Die beiden Kisten koppeln.** Öffne auf TOWER **Einstellungen, Kopplung**, drück **Phrase generieren** und gib die zwölf Wörter auf VAULT unter **Phrase eingeben** ein. Setz vorher auf beiden ein Login-Passwort.

**2. Auf TOWER VAULT als Empfänger anfragen.** Schalte auf der Seite Instanzen auf VAULTs Karte **Empfänger** ein, wähl die Bereiche und **Empfangsserver von VAULT** als Ort und schick die Anfrage.

**3. Auf VAULT erlauben.** VAULTs Seite Instanzen zeigt die Anfrage auf TOWERs Karte. Drück **Erlauben**. Beim ersten Mal fragt VAULT nach einem Ordner für seinen Empfangsserver, etwa `user/appdata/rest-server`, startet den rest-server im Append-only-Modus und führt den Manipulationstest dagegen aus.

**4. Zurück auf TOWER.** Ab jetzt kopiert TOWER nach jeder Sicherung zu VAULT, und der neue Speicherort erscheint auf TOWERs Seite Speicherorte mit seinem Löschschutz.

Was der Manipulationstest antwortet:

| Ergebnis | Was passiert ist |
| --- | --- |
| **geschützt** | VAULT hat das Löschen verweigert. Das ist der einzige bestandene Zustand. |
| **nicht geschützt** | VAULT hat ein Löschen angenommen. `--append-only` fehlt oder wurde entfernt. |
| **unentschieden** | Weder noch. Meist ist die URL nicht die, die restic selbst benutzt, oder die Zugangsdaten haben sich geändert. Es wird nichts vermerkt und kein Alarm ausgelöst. |

**5. Wenn du magst, ansehen, was ankommt.** Schalte auf VAULT **Empfänger** unter **Einstellungen, Instanzen** ein, öffne TOWERs **Details** und füge das empfangene Repository hinzu. VAULT bekommt TOWERs restic-Passwort über die Gruppe; niemand tippt einen Schlüssel ab.

**6. Gegenseitig machen.** Dieselben Schritte in die andere Richtung: VAULT fragt TOWER als Empfänger an. Dann erzwingt jede Kiste append-only für die andere, und keine kann die Sicherungen der anderen löschen.

Ein rest-server, den du selbst betreibst, geht auch: Füg ihn mit **Speicherort hinzufügen** als rest-server hinzu, mit Benutzer und Passwort aus seiner `.htpasswd`, und schalte den Löschschutz ein. Lass `--append-only` in seinen Optionen stehen; ohne das ist die Gegenseite wieder eine gewöhnliche Freigabe.

!!! warning "Ein lokaler Pfad ist ein Pfad im Container"
    Wo BombVault nach einem Ordner auf diesem Server fragt, trag ihn relativ zum Host-Mount ein, etwa `user/appdata/rest-server/bombvault-containers`, nicht `/mnt/user/appdata/...`. BombVault läuft in einem Container, in dem das `/mnt` des Hosts an anderer Stelle eingehängt ist; ein absoluter Host-Pfad existiert dort nicht. Fügst du trotzdem einen ein, nennt BombVault dir den relativen Pfad, den du stattdessen brauchst.

## Notfall-Assistent {#emergency}

Der **Notfall-Assistent** holt einen Server zurück, der weg ist oder neu aufgesetzt wurde. Er startet auf der Seite **Wiederherstellung** unter **Notfall** und über **Aus einer Sicherung wiederherstellen** auf der Willkommensseite einer neuen Installation. Er läuft in vier Schritten: **Zugang**, **Einstellungen**, **Auswahl** und **Wiederherstellen**.

1. **Zugang.** Zeig BombVault den Schlüssel der alten Sicherungen, auf einem von drei Wegen:
    - **Recovery-Kit hochladen**: die Datei, die du bei der Einrichtung heruntergeladen hast. Sie enthält den Schlüssel und jeden Speicherort, und BombVault füllt den Rest daraus aus. Ein mit age versiegeltes Kit fragt nach der Identität, für die es versiegelt wurde; sie wird einmal benutzt und nicht gespeichert.
    - **Über die Gruppe**: Eine Instanz deiner Gruppe verwahrt den Schlüssel als Hinterlegung. Du brauchst die zwölf Wörter der Gruppe, deinen Wiederherstellungscode und jemanden an dieser Instanz. Siehe [Schlüsselhüter](#key-keeper).
    - **Schlüssel eintippen**: den `APP_KEY` des alten Servers und wo seine Sicherungen liegen.

    BombVault probiert den Schlüssel dann an jedem Speicherort und sagt für jeden, ob er lesbar ist, nicht gefunden wurde, mit einem anderen Schlüssel gesperrt ist, nicht erreichbar ist oder noch seine Anmeldung braucht. Gespeichert ist noch nichts, und der Schlüssel bleibt eine begrenzte Zeit im Speicher. **Schlüssel übernehmen** lässt diesen Server ab jetzt mit diesem Schlüssel sichern; **Übernahme rückgängig machen** geht zurück zum eigenen Schlüssel, bis die erste Sicherung gelaufen ist. Ein Server, der schon eigene Sicherungen hat, lehnt die Übernahme ab und verweist auf [Aus einem alten Sicherungsordner](#restore-from-another-bombvault-repo). **Zugangsdaten speichern** übernimmt die Zugangsdaten, die das Kit für entfernte Speicherorte mitbringt, und fragt, bevor es Zugangsdaten ersetzt, die dieser Server schon hat.
2. **Einstellungen.** **Einstellungen zurückholen** stellt BombVaults eigene Einstellungen aus dem neuesten Selbstbackup wieder her: Speicherorte, Zugangsdaten, Zeitpläne und Regeln. BombVault startet dafür neu, über den Docker-Socket, sodass die laufende Einstellungsdatenbank nie unter einem offenen Handle überschrieben wird, und der Assistent macht danach von selbst weiter. Die Anmeldung dieses Servers bleibt: Du meldest dich weiter mit dem Passwort an, das du jetzt nutzt. Geheimnisse des alten Servers, die sich nicht übernehmen lassen, werden aufgelistet, damit du sie neu eingeben kannst.
3. **Auswahl.** BombVault sieht deine Sicherungen durch und listet die Container, VMs, Ordnersätze und ZFS-Datasets, die es findet. Überall ist der neueste Snapshot vorgewählt. Was dieser Server noch hat, startet ausgeschaltet, weil eine Wiederherstellung es ersetzen würde. Ordnersätze wissen nach einem Neuaufbau nicht mehr, woher sie kamen, und gehen deshalb in einen Ordner, den du wählst.
4. **Wiederherstellen.** Alles Gewählte kommt in einem Rutsch zurück, auf Wunsch gestoppt, damit du jeden Container und jede VM selbst startest, was nach einem Totalverlust am sichersten ist. Lade danach ein neues Recovery-Kit herunter: Das alte gehört zur vorigen Installation.

!!! tip "Geplanter Umzug oder Katastrophe"
    Der Notfall-Assistent stellt BombVaults eigene Einstellungen aus einer Sicherung wieder her. Für einen *geplanten* Umzug auf eine neue Box kannst du deine Konfiguration stattdessen direkt mit der Karte **Einstellungen exportieren / importieren** (eine portable JSON-Datei) mitnehmen. Siehe [Konfiguration](configuration.md#portable-settings-export-and-import).

### Aus einem alten Sicherungsordner {#restore-from-another-bombvault-repo}

**Aus einem alten Sicherungsordner** unter **Andere Quellen** auf der Seite Wiederherstellung öffnet das Repository einer *anderen* BombVault-Instanz (eine unter `/mnt` eingehängte Freigabe oder eine entfernte Adresse) mit dem Recovery-Kit dieser Instanz oder ihrem eingetippten `APP_KEY`, in einer einmaligen, schreibgeschützten Sitzung. Durchstöbere die dort gespeicherten Container, VMs, Ordnersätze und ZFS-Datasets, wähl einen Snapshot und stell ihn wieder her, und das Wiederhergestellte wird ein normaler lokaler Container, eine VM oder ein Ordnersatz. In das andere Repository wird nie etwas geschrieben, und deine eigenen Sicherungseinstellungen bleiben unangetastet (die Sitzung lebt im Speicher und läuft von selbst ab). Einen Container von Server A auf Server B zu bringen heißt nicht, die eigenen Einstellungen umzubiegen und danach zurückzustellen. Dieses Fenster ist einmalig: Es öffnet eine Sitzung, stellt wieder her, was du auswählst, und vergisst die andere Instanz. Willst du stattdessen eine dauerhafte Regelung, bei der diese Box die Snapshots einer anderen Instanz nach Plan in ihr eigenes Repository holt, ist das die Rolle **Abholer** auf der Seite Instanzen.

Ein Container, dessen Netzwerk es auf diesem Server nicht gibt, etwa ein `br0`-Netzwerk von Unraid auf einem normalen Docker-Host, zeigt unter seiner Zeile eine Netzwerkauswahl. BombVault legt ihn im gewählten Netzwerk an, zusammen mit seinen weiteren Netzwerken. Feste IP- und MAC-Adresse gehörten zum alten Netzwerk und fallen weg, beide vergibt dann das neue Netzwerk.

## Schlüsselhüter {#key-keeper}

Ein Schlüsselhüter ist eine Instanz deiner Gruppe, die eine versiegelte Kopie des Recovery-Kits dieses Servers verwahrt. Selbst öffnen kann sie die Kopie nicht. Geht dieser Server einmal verloren, bittet eine neue Installation, die deiner Gruppe beitritt, den Hüter um die Kopie und öffnet sie mit deinem Wiederherstellungscode.

1. Schalte auf der Seite Instanzen auf der Karte der Instanz, die verwahren soll, **Schlüsselhüter** ein und wähl **Hinterlegen**. Das Hinterlegen braucht ein Login-Passwort.
2. Dieser Server zeigt den **Wiederherstellungscode** einmal: zwanzig Buchstaben und Ziffern in vier Fünfergruppen. Schreib ihn auf und bestätige mit **Ich habe den Code aufgeschrieben**. Dieser Server speichert den Code nicht, nur, was er braucht, um eine Hinterlegung für ihn zu versiegeln.
3. Der Hüter hält jetzt die Hinterlegung, und dieser Server erneuert sie, wenn sich das Kit ändert. **Hinterlegung** auf der Karte dieser Instanz listet die **Hinterlegung dieses Servers** mit jedem Hüter und seinem Zustand und die **Hinterlegungen, die dieser Server verwahrt** für die anderen. **Neuen Code erstellen** ersetzt den Code; jeder Hüter wechselt auf den neuen, sobald er die neue Hinterlegung bekommt.

An dem Tag brauchst du drei Dinge: die zwölf Wörter deiner Gruppe, den Wiederherstellungscode und Zugang zur Oberfläche des Hüters, wo jemand **Erlauben** drücken muss. Wähl im Notfall-Assistenten **Über die Gruppe**, nimm die Hinterlegung, frag sie an und tipp den Wiederherstellungscode ein. Der Code wird auf dem neuen Server benutzt und nicht an den Hüter geschickt. Auf dem Hüter steht die Anfrage unter **Hinterlegung**, mit wer fragt, ob diese Instanz dort schon einmal gesehen wurde und wie oft sie gefragt hat; **Erlauben** gibt die versiegelte Kopie heraus, und das Erlauben braucht ein Login-Passwort. Die neue Installation hat nach dem Erlauben 30 Minuten, um sie abzuholen, eine Anfrage, die niemand beantwortet, läuft nach einem Tag ab, und eine Instanz, die zu oft fragt, muss eine Stunde warten.

Ohne den Code kann niemand die Kopie öffnen, der Hüter eingeschlossen. Ist der Code verloren, erstell einen neuen, solange dieser Server noch läuft; ist der Server schon weg, gehen weiterhin das Recovery-Kit und der eingetippte APP_KEY. Der Hüter kann eine Hinterlegung jederzeit löschen, und der Besitzer muss dann erneut anfragen.

## Recovery-Kit {#recovery-kit}

Dies ist das Stück, das eine Notfallwiederherstellung selbst dann möglich macht, wenn kein BombVault läuft.

**Recovery-Kit herunterladen** in der Karte **Für den Notfall** unter **Einstellungen, Wiederherstellung** gibt dir den **Schlüssel**, das **abgeleitete restic-Passwort**, jeden **Speicherort** mit dem Repository jedes Bereichs, die Zugangsdaten, die eine neue Installation braucht, um sie zu erreichen, und die **genauen restic-Befehle**, sodass du direkt mit dem restic-CLI auf jeder Maschine wiederherstellen kannst. Es braucht ein Login-Passwort. Das Kit druckt den Schlüssel auch bei ausgeschalteter Verschlüsselung, weil die Container- und VM-Definitionen immer damit versiegelt sind. Nach einer Übernahme druckt es den übernommenen Sicherungsschlüssel und den APP_KEY dieses Servers, den eine neue Installation nur braucht, um die Einstellungen zurückzuholen. Eine Karte **Sichere dein Recovery-Kit** auf der Übersicht erinnert dich, bis du es gespeichert hast.

!!! danger "Bewahre das Recovery-Kit getrennt vom Server auf"
    Das Kit enthält das Geheimnis, das deine Sicherungen entschlüsselt. Bewahre es an einem sicheren Ort getrennt vom Server auf (ein Passwortmanager, eine gedruckte Kopie im Safe). Verlierst du BombVault und `APP_KEY` ohne Recovery-Kit und ohne Hinterlegung, können deine verschlüsselten Sicherungen nicht wiederhergestellt werden.

!!! warning "Der neueste Snapshot ist nicht immer der richtige"
    Seit restic 0.17 zeigt `restic snapshots` die Größe jedes Snapshots. Nach einem Datenverlust kann der neueste Snapshot der geleerte sein, stell also keinen Snapshot wieder her, der viel kleiner ist als die davor. Nach Ransomware kann es der verschlüsselte in der üblichen Größe sein. Wenn BombVault noch läuft, sieh zuerst auf der Seite **Anomalien** nach: Sie nennt die letzte gute Sicherung, und das Wiederherstellungsfenster wählt sie vor. Für eine Wiederherstellung braucht es keine Anomalie-Daten von BombVault, und die Aufbewahrungspause behält immer nur mehr Snapshots.

### Das Kit versiegeln

Hast du **Exporte mit age verschlüsseln** unter **Einstellungen, System** eingeschaltet, wird das Kit ebenfalls damit versiegelt und als `bombvault-recovery-kit.md.age` heruntergeladen. Es ist ASCII-armored statt binär und bleibt damit Klartext: Einfügen in einen Passwortmanager oder Ausdrucken funktioniert genau wie vorher, nur ist der Inhalt ohne deinen Schlüssel nicht lesbar.

!!! warning "Den age-Schlüssel nicht im Kit aufbewahren"
    Zum Öffnen eines versiegelten Kits brauchst du deinen **privaten** age-Schlüssel. Bewahre ihn an einem Ort auf, der nicht vom Kit selbst abhängt, sonst hast du zwei Dinge wiederherzustellen statt einem. Das Versiegeln lohnt sich, wenn das Kit an einem Ort liegt, den du nicht ganz im Griff hast (ein geteilter Passwortmanager, Cloud-Notizen, ein Ausdruck im Büro); ein Kit in deinem eigenen Safe schützt schon der Safe.

    Mit eingeschalteter Verschlüsselung und ohne brauchbaren Empfänger wird der Download rundweg verweigert. BombVault gibt den Master-Key nie ersatzweise im Klartext heraus.

### Wenn das Kit gerade nicht zur Hand ist

Das Passwort ist nirgends gespeichert, es wird aus dem Schlüssel der Sicherungen **berechnet**. Mit dem Schlüssel und einer Shell kannst du es also selbst nachbilden:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Das ist HMAC-SHA256 über die feste Zeichenkette `bombvault:restic-repo`, als Schlüssel die rohen Bytes des hexadezimalen `APP_KEY`, ausgegeben als 64 Hex-Zeichen in Kleinschreibung. Derselbe Wert steht im Kit als abgeleitetes restic-Passwort; das hier ist für den Tag, an dem das Kit woanders liegt als du. Auf einem Server, der die Sicherungen eines anderen übernommen hat, nimm den übernommenen Sicherungsschlüssel, also den alten APP_KEY jenes Servers.

!!! warning "Bei einem empfangenen Repository den Schlüssel der sendenden Instanz nehmen"
    Ein Repository, das über eine Off-site-Kopie hier gelandet ist, wurde von der sendenden Maschine mit **deren** Schlüssel angelegt. Leitest du aus dem Schlüssel der empfangenden Kiste ab, kommt ein Passwort heraus, das restic ablehnt. Das liest sich genau wie ein kaputtes Repository und ist keines. Das ist der übliche Grund, warum `restic check` auf einem empfangenen Repo immer wieder nach dem Passwort fragt.

Weil Recovery-Definitionen **in** jedem Repo liegen (`<repo>/def`, `<repo>/vm-def`), ist ein kopierter Repo-Ordner vollständig eigenständig, sodass das Kit plus das Repo alles ist, was eine Bare-Metal-Wiederherstellung braucht.

## Einen Datenbank-Dump zurückholen {#database-dumps}

Ein Datenbank-Dump ist ein eigener Wiederherstellungspunkt im Container-Repository, mit der Marke `dbdump:<container>` und der einen Datei `/dbdump/<container>.sql`. BombVault listet, lädt und importiert sie im Reiter **Sicherungen** des Containers; unten stehen dieselben Schritte mit restic allein, für den Tag, an dem BombVault nicht da ist.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Die Marken `dbversion:` und `dbname:` an jedem Dump sagen, aus welcher Serverversion er stammt und welche Datenbanken er enthält. Eine vollständige Datei endet mit `-- PostgreSQL database cluster dump complete` oder `-- Dump completed`.

Spiele ihn in einen Container derselben oder einer neueren Version (PostgreSQL) beziehungsweise derselben Hauptversion (MySQL und MariaDB) ein, der einmal mit leerem Datenordner gestartet wurde, damit er sich einrichtet. Der Host braucht keinen Datenbank-Client, der Container hat einen:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Für eine einzelne Datenbank aus einem vollen Dump nehmen MySQL und MariaDB `--one-database <name>` am Client-Befehl. Ein PostgreSQL-Dump hat je Datenbank einen Abschnitt, der mit einer Zeile `\connect <name>` beginnt: kopiere diesen Abschnitt in eine eigene Datei und spiele sie nach dem Anlegen der Datenbank mit `-d <name>` ein.

!!! warning "Ein Root-Dump bringt die Benutzer des Servers mit"
    Ein voller MySQL- oder MariaDB-Dump, als root genommen, enthält die Systemdatenbank `mysql`. Beim Einspielen ersetzt er damit die Konten des neuen Servers, das Root-Passwort eingeschlossen, durch die aus dem Dump. Bei PostgreSQL ist `role ... already exists` für den Benutzer, den der Container angelegt hat, zu erwarten und harmlos.
