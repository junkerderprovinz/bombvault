# Off-site & Wiederherstellung

!!! note "Off-site-Kopien warten nach einem Neuaufbau"
    Wenn Schritt 4 Einträge ohne die alten Einstellungen neu aufbaut, pausiert die Off-site-Replikation dieser Bereiche, bis die Ablage-Vorgabe bestätigt ist. Siehe [Ablage pro Element](#placement).

Lokale Backups schützen dich vor einem verlorenen Container oder einem schlechten Update. Off-site-Replikation und ein getestetes Recovery-Kit schützen dich vor der ganzen Box, Ransomware oder einem Brand. Diese Seite behandelt das Replizieren ins Off-site, das Manipulationssicher-Machen dieser Kopie, den Nachweis der Wiederherstellbarkeit und die Wiederherstellung, wenn BombVault selbst verschwunden ist.

## Off-site-Replikation

Behalte das schnelle lokale Backup und füge eine oder mehrere Off-site-Repliken hinzu. Setze ein Repo pro Bereich auf der Seite **Einstellungen, Off-site**. BombVault repliziert neue Snapshots dorthin mit `restic copy` auf Best-Effort-Basis, sodass ein Off-site-Aussetzer das lokale Backup nie fehlschlagen lässt. In dieser Form bleibt das lokale Repo primär und das Off-site-Repo ist eine Replik, aber das primäre Repo eines Bereichs muss gar nicht lokal sein; wie du direkt nach S3, auf einen rest-server usw. sicherst, statt dorthin zu replizieren, steht unten unter [Entfernte primäre Repositories](#remote-primary-repositories).

- **Mehrere Off-site-Ziele pro Bereich.** Jeder Bereich (Container, VMs, Flash, Config, Dateisätze und ZFS-Datasets) kann gleichzeitig an mehrere Off-site-Ziele replizieren, nicht nur eines, sodass du zum Beispiel einen rest-server auf der Box eines Freundes und einen S3-Bucket parallel behalten kannst. Füge zusätzliche Ziele unter Einstellungen, Off-site hinzu, jedes mit eigenem Repository, S3-Speicherklasse, Append-only-Flag, Aufbewahrung und Wachstumsbudget. Eine bestehende einzelne Off-site-Einrichtung wird unangetastet als erstes Ziel übernommen, und jedes Ziel eines Bereichs repliziert nach dem Off-site-Zeitplan dieses Bereichs.
- **Off-site-Zeitplan pro Bereich** (neben jedem anderen Zeitplan unter Einstellungen, Zeitpläne bearbeitet): lasse ihn leer, um nach jedem lokalen Backup zu replizieren, oder setze eine Taktung (zum Beispiel `weekly Sun 03:00`), um seltener ins Off-site zu liefern, als du lokal sicherst. Ein Button **Jetzt replizieren** deckt Läufe auf Abruf ab.
- **Off-site-Aufbewahrung** liegt unter Einstellungen, Aufbewahrung, sodass du Off-site-Kopien länger als Archiv behalten kannst. Lasse die Richtlinie ganz auf null, um Off-site-Snapshots nie automatisch zu kürzen.
- **Bandbreitenlimits** (Einstellungen, Off-site) begrenzen die restic-Upload-/Download-Rate, sodass die Replikation dein WAN nicht auslastet.
- Eine **Replikationsanzeige** zeigt, welcher Bereich gerade repliziert, während es läuft (auf seiner Seite und im Dashboard). Es ist eine aktive Anzeige, kein Prozentbalken, weil `restic copy` keinen maschinenlesbaren Fortschritt bereitstellt.

!!! note "Von jedem Ort wiederherstellen"
    Jeder Container, jede VM, jedes Ordner-Set, der Flash und die App-Konfiguration listen ihre Backups als eine Zeitleiste über alle Orte, an denen ein Backup liegt. Ein nach B2 kopiertes Backup erscheint einmal, markiert mit jedem Ort, der es hält. Eine Wiederherstellung nimmt den ersten erreichbaren Ort, beginnend mit dem Repository, in das das Element geschrieben wird, und du kannst pro Zeile einen anderen Ort wählen. Off-site-Orte werden erst gelesen, wenn du sie öffnest. Das Löschen an einem Ort prüft zuerst die anderen und sagt, ob es die letzte Kopie war.

## Ablage pro Element {#placement}

Jede Container-, VM- und Ordner-Set-Karte hat eine Zeile **Ablage** mit drei Segmenten:

- **Lokal** schreibt das Element in das unter **Gespeichert auf** gezeigte Repository und kopiert es nirgendwohin. Nutze es für Daten, die schon eine zweite Kopie haben, zum Beispiel eine Freigabe, die auf einem NAS liegt.
- **Lokal + Off-site** schreibt es zusätzlich dorthin und kopiert es zu den unter **Kopie nach** angehakten Zielen, ein Chip pro Off-site-Ziel des Bereichs. Ein Häkchen entfernen, und das Ziel bekommt von diesem Element nichts Neues mehr.
- **Nur Off-site** schreibt das Element direkt an den Ort unter **Senden an**: ein direktes Repository neben einem Off-site-Ziel, oder ein entferntes Repository, das du unter Einstellungen, Speicher, Repositories einrichtest.

Der Ort ist ab dem ersten Backup des Elements fest, weil BombVault Backups nie zwischen Repositories verschiebt. Die Kopien können sich jederzeit ändern. Ein Ziel, das ein Element nicht mehr bekommt, behält seine vorhandenen Kopien und kürzt sie beim nächsten Off-site-Lauf des Bereichs auf seine eigene Aufbewahrung; **In B2 löschen** auf der Karte entfernt sie sofort. Existieren manche dieser Kopien nirgendwo sonst, listet die Bestätigung sie nach Datum auf und fragt nach dem Namen des Elements. Bei Append-only-Zielen lässt sich nicht löschen.

Unter der Zeile zeigt die Karte, wohin das Element geht und was tatsächlich da ist: an wie vielen Standorten es liegt, wann jedes Ziel zuletzt gesehen wurde, und ob 3-2-1 erfüllt ist. Ein Standort ist der Server mit den Originaldaten, jedes Off-site-Ziel und jedes als **Außer Haus** markierte Repository. BombVault prüft Kopien und Standorte; den Teil "zwei Medien" von 3-2-1 prüft es nicht.

### Ablage-Vorgaben

Einstellungen, Speicher, **Ablage-Vorgaben** haben eine Zeile pro Bereich mit denselben drei Segmenten. Die Kopien gelten sofort für jedes Element ohne eigene Wahl, sowie für die Projektordner von Compose-Stacks. Der Ort gilt für ein neues Element bei seinem ersten Backup; ihn zu ändern verschiebt keine Backups. Vor dem Speichern nennt die Zeile jedes Ziel, das Elemente gewinnt oder verliert, und wie viele Snapshots das bedeutet. **Auf Einträge ohne Backups anwenden** setzt jedes Element ohne bisheriges Backup auf die Vorgabe zurück.

Ein neues Off-site-Ziel bekommt jedes Element, das nicht auf Lokal steht. Der Dialog, der es hinzufügt, nennt die Anzahl der Elemente und, wo bekannt, wie viel Verlauf das ist, und bietet an, die Elemente auszulassen, die schon bei anderen Zielen ausgeschlossen sind.

### Direkte Repositories

Wählst du das direkte Repository eines Ziels unter Nur Off-site, öffnet sich ein Dialog mit einem vorgeschlagenen Ort neben dem Ziel, zum Beispiel `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, und ein Verbindungstest, der nichts anlegt. **Anlegen und nutzen** erstellt das Repository und richtet das Element darauf aus. Ein direktes Repository übernimmt Schlüssel, Speicherklasse, Limits, Append-only-Einstellung und Aufbewahrung des Ziels und ändert sich mit ihnen; die Repositories-Karte zeigt es schreibgeschützt. Kann ein neuer Schlüssel für das Ziel es nicht öffnen, behält das direkte Repository seinen bisherigen Schlüssel, und das Speichern sagt das. Seine Snapshots tragen das Tag `bv:direct`, und jeder andere Aufbewahrungslauf lässt sie stehen, sodass ein direktes Repository, das die Verbindung zu seinem Ziel verloren hat, nie nach den lokalen Regeln altert. B2 wird über seinen S3-Endpunkt angesprochen, mit der Schlüssel-ID und dem Anwendungsschlüssel als S3-Zugangsdaten; ein Schlüssel, der auf den eigenen Ordner des Ziels beschränkt ist, erreicht den Ordner daneben nicht, beschränke ihn deshalb stattdessen auf den Ordner über dem Ziel.

### Außer Haus

Ein benanntes Repository lässt sich auf der Repositories-Karte als **Außer Haus** markieren. Entfernte Repositories starten markiert; schalte es für einen rest-server im selben Gebäude aus. Die Markierung zählt nur bei Standorten und 3-2-1 auf den Karten. Sie ändert keine Kopie.

### Nach einem Neuaufbau

Kopier-Entscheidungen leben in BombVaults eigenen Einstellungen. Nach einem Neuaufbau über Backups entdecken ohne wiederhergestelltes `/config` sind sie weg, und alles zu kopieren würde die Elemente, die du ausgelassen hattest, wieder nach B2 schicken. Die Off-site-Replikation jedes neu aufgebauten Bereichs pausiert deshalb. Das Dashboard zeigt es in Gelb, und Ablage-Vorgaben bieten **Vorgabe bestätigen** mit einer Vorschau, was der nächste Lauf kopiert, und den Namen in den Backups, die keinen Eintrag haben, die du dort auslassen kannst. Nur die Bestätigung beendet die Pause; eine Einstellungsdatei zu importieren bringt Regeln und Vorgaben zurück, beendet die Pause aber nicht.

## Entfernte primäre Repositories {#remote-primary-repositories}

Der Sicherungspfad einer Domäne (Einstellungen, Speicher) ist nicht auf einen lokalen Ordner beschränkt: richte ihn direkt auf ein restic-Remote (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/pfad`), und BombVault sichert unmittelbar dorthin, ohne getrennte lokale Kopie und ohne Replikationsschritt. Das ist eine wirklich andere Form als die Off-site-Replikation weiter oben: dort ist das lokale Repo primär und das Off-site-Repo ein Archiv davon nach bestem Bemühen; hier **ist** das entfernte Repo das primäre und die einzige Kopie, solange du für diese Domäne nicht zusätzlich eine Off-site-Replikation (oder ein zweites Remote) einrichtest.

Jedes der sechs Pfadfelder (Container, VMs, Flash, Selbst-Backup, Ordner, ZFS-Datasets) hat direkt daneben einen Schalter **Lokal / Remote**:

- **Lokal** zeigt den gewohnten Ordner-Browser.
- **Remote** tauscht ihn gegen ein einfaches URL-Feld, dazu eine Schaltfläche, die denselben Dialog für Verbindungstest und Zugangsdaten öffnet, den auch Off-site-Ziele verwenden, nur eben für dieses primäre Repo. Von dort bekommst du:
    - **Einen Verbindungstest** gegen den echten Pfad, bevor du dich darauf verlässt.
    - **Bandbreitengrenzen** (Hoch- und Herunterladen), damit eine geplante Sicherung auf ein entferntes primäres Repo nicht deine WAN-Leitung auslastet: dieselben restic-Schalter `--limit-upload` und `--limit-download`, die die Off-site-Replikation nutzt, angewandt auf die Sicherung selbst.
    - **Append-only-Schutz (Unveränderlichkeit)**, geprüft mit demselben aktiven Manipulationstest (eine echte DELETE-Probe gegen die Gegenseite), den auch Off-site-Ziele bekommen. Ist er an, weigert sich BombVault, das Repo selbst zu bereinigen: weil dahinter keine getrennte lokale Kopie steht, dürfen die Zugangsdaten auf dieser Kiste nicht in der Lage sein, die einzige Kopie der Sicherung zu löschen.
    - **Einen Alarm für das Wachstumsbudget**, abgeleitet aus demselben Trend der Repo-Größe, den die Speicher-Karte ohnehin verfolgt.

Nichts davon ist Pflicht: ein von Hand eingetragener entfernter Pfad ohne gespeicherte Sicherheitseinstellungen sichert genau so wie bisher (unbegrenzte Bandbreite, bereinigbar, kein Budgetalarm). Der Sicherheitsdialog ist für den Fall da, dass du dieselben Schutzmaßnahmen willst, die eine Off-site-Kopie bekommt, ohne dafür extra ein Off-site-Ziel einrichten zu müssen.

!!! note "Cloud- und REST-Zugangsdaten werden geteilt"
    Ein entferntes primäres Repo meldet sich mit denselben S3-/REST-Zugangsdaten an, die unter Einstellungen, Cloud-Zugänge, Geteilte Cloud-Zugangsdaten hinterlegt sind. Einen getrennten Speicher für Zugangsdaten primärer Repos gibt es nicht.

### SMB und WebDAV ohne Host-Mount {#smb-webdav}

Unter Einstellungen, Cloud-Zugänge, rclone gibt es ein Formular für eine Windows- oder Samba-Freigabe und für einen WebDAV-Server (Nextcloud, ownCloud, SharePoint oder einen anderen). Trag einen kurzen Namen ein, dazu Host und Freigabe (SMB) oder URL und Servertyp (WebDAV), Benutzer und Passwort, und BombVault schreibt den rclone-Abschnitt für dich. rclone verschleiert das Passwort selbst, bevor es gespeichert wird; ein Ziel mit einem Namen, den es schon gibt, ersetzt diesen Abschnitt, statt einen zweiten anzulegen.

Das Formular antwortet mit dem fertigen Ort, zum Beispiel `rclone:nas:backups`. Trag ihn in einen Backup-Pfad oder ein Off-site-Ziel ein und häng einen Unterordner an, wenn du einen willst (`rclone:nas:backups/bombvault`). Die Freigabe ist das erste Pfadsegment, nicht Teil des Namens.

Das ist der bessere Weg, als die Freigabe auf Unraid einzuhängen: restic rät davon ab, ein Repository auf einer eingehängten CIFS-Freigabe zu halten, und hier wird nichts eingehängt. NFS fehlt im Formular, weil weder restic noch rclone ein NFS-Backend hat; für NFS hängst du den Export auf dem Host ein und richtest einen Backup-Pfad darauf.

## Unveränderliches (Append-only) Off-site

Markiere ein Off-site-Repo als Append-only, sodass Ransomware oder ein kompromittierter Host deine Backups nicht löschen oder überschreiben kann. Die Gegenseite (ein `restic/rest-server`, der im `--append-only`-Modus läuft) **erzwingt** es. BombVault **verifiziert** es nur und zeigt niemals grün allein auf eine Konfigurationsbehauptung hin.

Der Assistent der **geführten Off-site-Einrichtung** führt dich von der Backend-Wahl (rest-server / rclone / S3) über ein einsatzbereites rest-server-Deploy-Snippet, einen Verbindungstest, den Unveränderlichkeits-Schalter (der den Manipulationstest sofort ausführt) bis zu einer Aufbewahrungsstrategie, sodass Append-only-Off-site ohne Handbearbeitung von Konfigurationen erreichbar ist.

!!! note "Eine erfolgreiche Löschung unter `/locks/` ist erwartet"
    Append-only heißt nicht, dass gar nichts mehr entfernt werden kann. restic muss seine eigenen Sperren setzen und wieder lösen, deshalb bleibt `/locks/` absichtlich schreib- und löschbar. Snapshots und die Daten dahinter, also genau das, worauf Ransomware zielt, lassen sich nicht entfernen. Wer die Gegenseite selbst abklopft, bekommt unter `/locks/` eine erfolgreiche Löschung: das ist richtig so und kein Loch im Schutz.

!!! warning "Unveränderliche Repos werden von dieser Box aus nie gekürzt"
    Ein unveränderliches Off-site kürzt bewusst nie alte Snapshots. Setze einen **Wachstumsbudget-Alarm** dafür, damit du alarmiert wirst, bevor die Repo-Größe außer Kontrolle gerät.

## Manipulationstest

BombVault beweist die Append-only-Garantie regelmäßig, indem es tatsächlich einen Löschversuch gegen das Off-site-Repo unternimmt, gezielt auf ein nicht existierendes Objekt:

- **Verweigert** bedeutet geschützt.
- **Akzeptiert** bedeutet nicht geschützt.
- Ein **unschlüssiges** Ergebnis (Server nicht erreichbar, Auth-Fehler) kippt das gespeicherte Urteil nie.

Ein echtes Kippen von geschützt zu ungeschützt löst einen einzelnen Alarm aus.

## DR-Übungen

BombVault bietet zwei Stufen des Nachweises, dass deine Backups tatsächlich wiederherstellbar und nicht nur vorhanden sind.

- **Wiederherstellungs-Prüfübungen (lokal).** BombVault führt regelmäßig `restic check --read-data-subset` aus (begrenzt, nie eine plattenfüllende Vollwiederherstellung) und zeigt pro Bereich ein Abzeichen *Wiederherstellbar verifiziert*. Die Taktung liegt unter Einstellungen, Zeitpläne; das Abzeichen unter Einstellungen, Integrität.
- **DR-Übungen (Off-site).** BombVault stellt ein echtes Ziel aus dem Off-site-Repo in eine Wegwerf-Sandbox wieder her, prüft es Datei für Datei und Byte für Byte und räumt dann auf. Dies beweist, dass du aus dem Off-site wiederherstellen kannst, nicht nur, dass das Repo antwortet.

Die **Ransomware-Schutz-Scorecard** im Dashboard fasst dies zu einer grün / gelb / rot-Haltung pro Bereich zusammen, mit einer altersgestempelten Checkliste (Off-site konfiguriert, Append-only verifiziert, Replikation aktuell, Wiederherstellungsübung bestanden, Verschlüsselung an, Kürzungsstrategie gesetzt). Jede rote Zeile verlinkt tief zur Behebung, und die Karte wird nur bei verifizierten Fakten grün.

## Instanzen koppeln {#pairing}

Empfänger, Holen, die Instanzen-Seite und Mesh-Off-site sprechen alle mit einem anderen BombVault. Das tun sie als Mitglieder einer Kopplungsgruppe, und in die Gruppe kommt eine Instanz mit zwölf Wörtern.

Öffne auf der ersten Instanz **Einstellungen → Kopplung** und klick in den Kopplungskarten auf **Phrase generieren**. Es erscheinen zwölf Wörter in einem Fenster mit einer **Kopieren**-Schaltfläche. Öffne auf jeder weiteren Instanz dieselbe Stelle, klick auf **Phrase eingeben** und füg die Wörter ein oder tipp sie ab, oder klick in diesem Fenster auf **Einfügen**. Ein Wort, das nicht auf der Liste steht, nennt die Seite schon beim Tippen mit seiner Stelle, und das letzte Wort enthält eine Prüfsumme: Ein vertipptes oder vertauschtes Wort fällt auf, bevor etwas gekoppelt wird. Erstell die Phrase nur auf einer Instanz, denn zwei Instanzen, die beide eine Phrase erstellen, bilden zwei getrennte Gruppen. Meldet sich eine Minute lang niemand, bietet der Reiter zwei Wege heraus: die Wörter erneut anzeigen, um sie drüben einzugeben, oder die Wörter der anderen Instanz eingeben und ihrer Gruppe in einem Schritt beitreten. Koppeln geht auch ohne Anmeldepasswort, aber leg eins fest: ohne Passwort kann jeder, der diese Weboberfläche öffnen kann, die Wörter lesen und sich über die Gruppe das restic-Passwort jeder Instanz darin holen. Die Kopplungskarte weist darauf hin, solange kein Passwort gesetzt ist. Mit Passwort verlangt das erneute Anzeigen der Phrase danach. Mit **Gruppe verlassen** nimmst du eine Instanz wieder heraus.

Wer die Wörter kennt, kommt in die Gruppe. Behandle sie also wie ein Passwort.

**Wie sich die Mitglieder erreichen.** Jede Instanz übernimmt ihre eigene Adresse im Netzwerk aus deinem Browser, sobald du dich anmeldest, zu sehen in der Relay-Karte als **Diese Instanz in deinem Netzwerk**; korrigier sie dort, wenn ein Reverse Proxy oder ein ungewöhnlicher Port davorliegt. Im selben Netzwerk geben die Mitglieder diese Adresse per Multicast bekannt und sprechen direkt miteinander, und wo Multicast ein Container-Netz wie Dockers Standard-Bridge-Netz nicht durchquert, durchsucht eine Instanz stattdessen ihr eigenes Subnetz nach den anderen, mit einem signierten Aufruf, den nur ein Gruppenmitglied beantworten kann, sodass die Kopplung auch ohne Relay in Sekunden steht. Findet sich nichts, nimmt **Findest du sie nicht?** unter der Kopplungskarte eine Adresse von Hand entgegen, für ein anderes Subnetz oder einen unüblichen Port. Instanzen in verschiedenen Netzen laufen über ein Relay, das du auf demselben Reiter wählst:

- **Projekt-Relay** (Vorgabe): `parleyport.halleluja.design`, dasselbe Relay, das auch KnightLoader nutzt. Einzurichten gibt es nichts.
- **Eigenes Relay**: der Container [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) aus den Unraid Community Apps oder eine deiner Instanzen, die schon von außen erreichbar ist, mit eingeschaltetem **Als Relay dienen**. Diese Instanz antwortet dann unter `/relay/connect` an ihrer eigenen Adresse, hinter dem Reverse Proxy und dem Zertifikat, die sie schon hat, und lässt nur deine Gruppe hinein. Trag die Adresse des Relays auf jeder Instanz ein, die es nutzen soll.
- **Kein Relay**: Die Mitglieder finden sich automatisch im selben Netzwerk, und sonst nirgends.

**Was das Relay sieht.** Jeder Aufruf zwischen Mitgliedern ist mit AES-256-GCM unter einem Schlüssel versiegelt, der aus den zwölf Wörtern entsteht und deine Instanzen nie verlässt. Das Relay erfährt einen Hash, über den es die Verbindungen zusammenführt, dazu für welche Instanz eine Nachricht ist, wie groß sie ist und wann sie durchläuft. Ein direkter Aufruf im lokalen Netz ist genauso versiegelt und zusätzlich signiert, damit hängt nichts am selbstsignierten Zertifikat einer Instanz.

**Was über die Gruppe läuft.** Die Scorecards auf der Instanzen-Seite, die Bitte, eine Domäne jetzt zu prüfen, Mesh-Off-site-Angebote und was Empfänger und Holen brauchen: die Repository-Adressen der anderen Instanz und ihr Restic-Passwort. Backup-Daten laufen nie darüber, die gehen weiter direkt zu den Restic-Backends. Auch der APP_KEY nicht: Das Restic-Passwort öffnet die Repositorys der anderen Instanz und sonst nichts, weder ihre gespeicherten Geheimnisse noch Sitzungen oder Wiederherstellungscodes.

**Einträge von vor der Kopplung.** Mit einem Fleet-Token hinzugefügte Instanzen sowie Empfänger und Holen-Quellen mit dem APP_KEY der anderen Instanz bleiben nach dem Update erhalten und tragen **Neu koppeln**. Empfänger und Quellen laufen weiter: Beim ersten Start ersetzt BombVault jeden gespeicherten APP_KEY durch das daraus abgeleitete Restic-Passwort. Kopple beide Instanzen, bearbeite dann den Eintrag und wähl seine Instanz. Eine solche Instanz übernimmt ihre alte Karte, sobald eine Instanz mit demselben Namen in der Gruppe auftaucht.

Den APP_KEY von Hand braucht nur noch die [Wiederherstellung aus einem anderen BombVault-Repo](#restore-from-another-bombvault-repo), für den Fall, dass die andere Instanz weg ist und in keiner Gruppe mehr antworten kann.

## Empfänger-Dashboard (die empfangende Seite)

![Die empfangende Seite, nur lesend beobachtet, mit einer Integritätsprüfung auf dieser Hardware.](assets/screenshots/receiver.png)

*Die empfangende Seite, nur lesend beobachtet, mit einer Integritätsprüfung auf dieser Hardware.*

Alles oben ist die *sendende* Seite. Auf der Box, die unveränderliche Off-site-Kopien von einem anderen BombVault **empfängt**, gibt dir das Empfänger-Dashboard eine unabhängige, schreibgeschützte Überwachung dieser Repositorys auf der empfangenden Hardware, sodass ein stiller Fehler am fernen Ende nicht unbemerkt bleibt.

Schalte den **Empfänger**-Schalter in den Einstellungen ein, um einen **Empfänger**-Tab freizulegen. Er ist standardmäßig aus; aktiviere ihn nur auf einer Box, die tatsächlich unveränderliche Off-site-Backups empfängt. Registriere dann ein empfangenes Repository (schreibgeschützt, geöffnet mit dem Restic-Passwort der sendenden Instanz, das über die [Kopplungsgruppe](#pairing) kommt), um zu erhalten:

- **Einen nach Quelle gruppierten Snapshot-Bestand**, sodass du genau sehen kannst, welche Container, VMs und Dateisätze eingetroffen sind.
- **Zuletzt empfangen** pro Quelle, sodass du weißt, wie frisch jede ist.
- **Ein unabhängiges `restic check`**, das auf der empfangenden Hardware läuft, sodass die Integrität dort geprüft wird, wo die Daten tatsächlich liegen, nicht nur beim Sender.
- **Einen Totmannschalter:** ein Alarm, wenn eine Quelle innerhalb eines von dir gesetzten Fensters aufhört zu senden.
- **Integritätsalarme:** ein Alarm, wenn eine Prüfung auf der empfangenden Seite fehlschlägt.

Der Empfänger ist strikt schreibgeschützt. Er schreibt niemals in das empfangene Repository, sodass er die Append-only-Garantie, auf die sich der Sender verlässt, nie brechen kann.

## Durchgerechnetes Beispiel: zwei Unraid-Kisten, Ende zu Ende

Oben stehen die Einzelteile. Hier ist ein vollständiger Aufbau mit echten Werten, weil sich Teile leichter zusammensetzen lassen, wenn man sie einmal zusammengesetzt gesehen hat.

Zwei Kisten: **TOWER** betreibt die Container und schiebt die Backups, **VAULT** nimmt sie an und erzwingt die Unveränderlichkeit. Setze deine eigenen Namen, Adressen und Freigabepfade ein.

**1. Auf VAULT den Append-only-Server aufsetzen.** In BombVault auf TOWER unter *Einstellungen → Off-site → Einrichten* **rest-server** wählen und das Rezept erzeugen. Den Reiter **Unraid-Vorlage (XML)** kopieren, auf VAULT als `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml` speichern, dann *Docker → Add Container* und **rest-server** aus der Vorlagenliste wählen. Vor dem Start die angezeigte `htpasswd`-Zeile auf VAULT in `/mnt/user/appdata/rest-server/.htpasswd` schreiben. Das Einmal-Passwort wird nur einmal angezeigt und nie gespeichert, kopiere es jetzt. Diese Zeile trägt dasselbe Passwort, für dich schon bcrypt-gehasht: der Klartext gehört in die REST-Zugangsdaten auf TOWER, die gehashte Zeile in die `.htpasswd` auf VAULT. Du musst selbst nichts hashen.

    `--append-only` im OPTIONS-Feld stehen lassen. Es ist der ganze Sinn der Sache: ohne das ist VAULT wieder eine gewöhnliche Freigabe.

**2. Auf TOWER das Off-site-Repo darauf richten.** Die Repo-URL folgt dem Muster, das das Rezept ausgibt:

    rest:http://VAULT:8000/bombvault-containers/containers

Das erste Pfadsegment ist der htpasswd-Benutzer, das zweite das Repository. Trage den erzeugten Benutzer und das Passwort als REST-Zugangsdaten des Ziels ein und führe den **Verbindungstest** aus.

**3. Auf TOWER „Unveränderlich“ einschalten.** Der Manipulationstest läuft sofort und muss *geschützt* melden. Was die Antworten bedeuten:

| Ergebnis | Was passiert ist |
| --- | --- |
| **geschützt** | VAULT hat das Löschen verweigert. Das ist der einzige bestandene Zustand. |
| **NICHT geschützt** | VAULT hat ein Löschen angenommen. `--append-only` fehlt oder wurde entfernt. |
| **unentschieden** | Weder noch. Meist ist die URL nicht die, die restic selbst benutzt, oder die Zugangsdaten haben sich geändert. Es wird nichts vermerkt und kein Alarm ausgelöst. |

**4. Auf VAULT ansehen, was ankommt.** Die beiden Kisten koppeln ([Instanzen koppeln](#pairing)), *Einstellungen → Allgemein → Empfänger* einschalten, den Reiter **Empfänger** öffnen und das Repository schreibgeschützt registrieren, mit TOWER als sendender Instanz.

!!! warning "Der Ort ist ein Pfad **innerhalb** des Containers, relativ zum Host-Mount geschrieben"
    Trage `user/appdata/rest-server/bombvault-containers/containers` ein, **nicht** `/mnt/user/appdata/…`. BombVault läuft in einem Container, in dem das `/mnt` des Hosts an anderer Stelle eingehängt ist; ein absoluter Host-Pfad existiert dort nicht. Fügst du trotzdem einen ein, nennt BombVault dir jetzt den relativen Pfad, den du stattdessen brauchst.

    VAULT bekommt das Restic-Passwort von TOWER beim Speichern über die Gruppe; niemand tippt einen Schlüssel ab.

**5. Wenn du magst, mach es gegenseitig.** Dieselben fünf Schritte in die andere Richtung: ein rest-server auf TOWER, der VAULTs Kopie annimmt. Dann erzwingt jede Kiste die Unveränderlichkeit für die andere, und keine kann die Backups der anderen löschen.

## Geführte Wiederherstellung

Ein eigener Reiter **Wiederherstellung** führt eine frische oder neu aufgebaute Installation durch den Katastrophenfall, an einem Ort:

1. **Stellt zuerst BombVaults eigene Einstellungen wieder her**, sodass die Backup-Pfade, Off-site-Ziele und Zugangsdaten, die der Rest des Ablaufs braucht, vorausgefüllt sind (angewendet per Selbst-Neustart über den Docker-Socket, sodass die laufende Einstellungsdatenbank nie unter einem offenen Handle überschrieben wird).
2. **Prüft, dass BombVault deine Backups lesen kann** (der Verschlüsselungsschlüssel-Fallstrick vorab).
3. Lässt dich **auf dein bestehendes Repo verweisen** (lokal oder Off-site).
4. **Entdeckt** die darin gespeicherten Container, VMs, Dateisätze und ZFS-Datasets.
5. **Stellt Container und VMs in einem Rutsch wieder her** (gestoppt belassen, sodass du sie bewusst startest) und listet Dateisätze und ZFS-Elemente auf, die du einzeln wiederherstellst; ZFS-Elemente kommen ausgeschaltet zurück. Dein Recovery-Kit ist einen Klick entfernt.

!!! tip "Geplante Migration versus Katastrophe"
    Die geführte Wiederherstellung stellt BombVaults eigene Einstellungen aus einem Backup wieder her. Für einen *geplanten* Umzug auf eine neue Box kannst du deine Konfiguration stattdessen direkt mit der Karte **Einstellungen exportieren / importieren** (eine portable JSON-Datei) mitnehmen. Siehe [Konfiguration](configuration.md#portable-settings-export-and-import).

### Wiederherstellung aus einem anderen BombVault-Repo {#restore-from-another-bombvault-repo}

Eine separate Karte im Reiter **Wiederherstellung** öffnet das Repo einer *anderen* BombVault-Instanz (eine unter `/mnt` eingehängte Freigabe oder eine Remote-URL) mit **dem `APP_KEY` dieser Instanz**, in einer einmaligen, schreibgeschützten Sitzung. Durchstöbere die dort gespeicherten Container, VMs und Dateisätze, wähle einen Snapshot und stelle ihn wieder her, und das wiederhergestellte Objekt wird ein normaler lokaler Container, eine VM oder ein Dateisatz. Es wird niemals etwas in das andere Repo geschrieben, und deine eigenen Backup-Einstellungen bleiben unangetastet (die Sitzung lebt im Speicher und läuft von selbst ab). Einen Container von Server A auf Server B zu verschieben bedeutet nicht mehr, deine Repo-Einstellungen umzustellen und danach zurückzudrehen. Diese Karte ist für einen einzelnen Vorgang: Sie öffnet eine Sitzung, stellt wieder her, was du auswählst, und vergisst die andere Instanz. Willst du stattdessen eine dauerhafte Einrichtung, bei der diese Box die Snapshots einer anderen Instanz nach Zeitplan in ihr eigenes Repository holt, ist das der Reiter **Holen** der Seite **Instanzen**.

Ein Container, dessen Netzwerk es auf diesem Server nicht gibt, etwa ein `br0`-Netzwerk von Unraid auf einem normalen Docker-Host, zeigt unter seiner Zeile eine Netzwerkauswahl. BombVault legt ihn im gewählten Netzwerk an, mit seiner MAC-Adresse und seinen weiteren Netzwerken. Die feste IP-Adresse gehörte zum alten Netzwerk und fällt weg, die Adresse vergibt dann das neue Netzwerk.

## Wiederherstellungspaket für den Verschlüsselungsschlüssel

Dies ist das Stück, das eine Notfallwiederherstellung selbst dann möglich macht, wenn kein BombVault läuft.

Ein Klick lädt den **Master-Key**, das **abgeleitete restic-Passwort** und die **genauen Repo-Orte und -Befehle** herunter, sodass du direkt mit dem restic-CLI auf jeder Maschine wiederherstellen kannst. Eine Dashboard-Erinnerung nervt, bis du es gespeichert hast.

!!! danger "Bewahre das Recovery-Kit off-box auf"
    Das Kit enthält das Geheimnis, das deine Backups entschlüsselt. Bewahre es an einem sicheren Ort getrennt vom Server auf (ein Passwortmanager, eine gedruckte Kopie im Safe). Wenn du sowohl BombVault als auch `APP_KEY` ohne Recovery-Kit verlierst, können deine verschlüsselten Backups nicht wiederhergestellt werden.

!!! warning "Der neueste Snapshot ist nicht immer der richtige"
    Seit restic 0.17 zeigt `restic snapshots` die Größe jedes Snapshots. Nach einem Datenverlust kann der neueste Snapshot der geleerte sein, stelle also keinen Snapshot wieder her, der viel kleiner ist als die davor. Nach Ransomware kann es der verschlüsselte in der üblichen Größe sein. Wenn BombVault noch läuft, sieh zuerst auf der Seite **Anomalien** nach: Sie nennt das letzte gute Backup. Für eine Wiederherstellung braucht es keine Anomalie-Daten von BombVault, und die Aufbewahrungspause behält immer nur mehr Snapshots.

### Das Kit versiegeln

Hast du die age-Verschlüsselung für die schlichten Exporte eingeschaltet (Einstellungen), wird das Kit ebenfalls damit versiegelt und als `bombvault-recovery-kit.md.age` heruntergeladen. Es ist ASCII-armored statt binär und bleibt damit Klartext: Einfügen in einen Passwortmanager oder Ausdrucken funktioniert genau wie vorher, nur ist der Inhalt ohne deinen Schlüssel nicht lesbar.

!!! warning "Den age-Schlüssel nicht im Kit aufbewahren"
    Zum Öffnen eines versiegelten Kits brauchst du deinen **privaten** age-Schlüssel. Bewahre ihn an einem Ort auf, der nicht vom Kit selbst abhängt, sonst hast du zwei Dinge wiederherzustellen statt einem. Das Versiegeln lohnt sich, wenn das Kit an einem Ort liegt, den du nicht ganz im Griff hast (ein geteilter Passwortmanager, Cloud-Notizen, ein Ausdruck im Büro); ein Kit in deinem eigenen Safe schützt schon der Safe.

    Mit eingeschalteter Verschlüsselung und ohne brauchbaren Empfänger wird der Download rundweg verweigert. BombVault gibt den Master-Key nie ersatzweise im Klartext heraus.

### Wenn das Kit gerade nicht zur Hand ist

Das Passwort ist nirgends gespeichert, es wird aus dem `APP_KEY` **berechnet**. Mit dem Schlüssel und einer Shell kannst du es also selbst nachbilden:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Das ist HMAC-SHA256 über die feste Zeichenkette `bombvault:restic-repo`, als Schlüssel die rohen Bytes des hexadezimalen `APP_KEY`, ausgegeben als 64 Hex-Zeichen in Kleinschreibung. Derselbe Wert steht im Kit als abgeleitetes restic-Passwort; das hier ist für den Tag, an dem das Kit woanders liegt als du.

!!! warning "Bei einem empfangenen Repository den Schlüssel der SENDENDEN Instanz nehmen"
    Ein Repository, das über die Off-site-Replikation hier gelandet ist, wurde von der sendenden Maschine mit **deren** `APP_KEY` angelegt. Leitest du aus dem Schlüssel der empfangenden Kiste ab, kommt ein Passwort heraus, das restic ablehnt. Das liest sich genau wie ein kaputtes Repository und ist keines. Das ist der übliche Grund, warum `restic check` auf einem empfangenen Repo immer wieder nach dem Passwort fragt.

Weil Recovery-Definitionen **in** jedem Repo liegen (`<repo>/def`, `<repo>/vm-def`), ist ein kopierter Repo-Ordner vollständig eigenständig, sodass das Kit plus das Repo alles ist, was eine Bare-Metal-Wiederherstellung braucht.

## Einen Datenbank-Dump zurückholen {#database-dumps}

Ein Datenbank-Dump ist ein eigener Wiederherstellungspunkt im Container-Repository, mit der Marke `dbdump:<container>` und der einen Datei `/dbdump/<container>.sql`. BombVault listet, lädt und importiert sie unter **Backups**; unten stehen dieselben Schritte mit restic allein, für den Tag, an dem BombVault nicht da ist.

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
