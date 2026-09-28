# Off-site & Wiederherstellung

Lokale Backups schützen dich vor einem verlorenen Container oder einem schlechten Update. Off-site-Replikation und ein getestetes Recovery-Kit schützen dich vor der ganzen Box, Ransomware oder einem Brand. Diese Seite behandelt das Replizieren ins Off-site, das Manipulationssicher-Machen dieser Kopie, den Nachweis der Wiederherstellbarkeit und die Wiederherstellung, wenn BombVault selbst verschwunden ist.

## Off-site-Replikation

Behalte das schnelle lokale Backup und kopiere es an einen oder mehrere andere Orte. An welche Orte ein Bereich kopiert wird, wählst du auf der Karte **Domänen** unter **Einstellungen, Speicher**, mit einem Chip pro Ort (siehe [Speicherorte](storage-places.md#domains)). BombVault kopiert neue Snapshots dorthin mit `restic copy` auf Best-Effort-Basis, sodass eine gescheiterte Kopie das lokale Backup nie fehlschlagen lässt. Der Ort, an dem ein Bereich gespeichert ist, muss nicht lokal sein; siehe [Ein Bereich an einem entfernten Ort](#remote-primary-repositories).

- **Mehrere Kopie-Orte pro Bereich.** Ein Bereich lässt sich gleichzeitig an mehrere Orte kopieren, zum Beispiel an einen rest-server bei einem Freund und in einen B2-Bucket. Aufbewahrung, Speicherklasse, Append-only, Grenzen und Wachstumsbudget gehören zum Ort, jede Kopie folgt also den Regeln des Ortes, an dem sie landet.
- **Kopier-Zeitplan pro Bereich** (neben jedem anderen Zeitplan unter Einstellungen, Zeitpläne bearbeitet): lasse ihn leer, um nach jedem lokalen Backup zu kopieren, oder setze eine Taktung (zum Beispiel `weekly Sun 03:00`), um seltener zu kopieren, als du sicherst. **Jetzt kopieren** in der Zeile des Bereichs startet ihn auf Abruf.
- **Aufbewahrung pro Ort.** Jeder Ort hat seine eigenen Regeln, so kann ein Off-site-Ort Kopien länger als Archiv behalten. Ein Ort, bei dem jede Regel auf null steht, kürzt nie.
- **Bandbreitenlimits** pro Ort begrenzen die restic-Upload- und Download-Rate, sodass das Kopieren dein WAN nicht auslastet.
- Eine **Replikationsanzeige** zeigt, welcher Bereich gerade kopiert, während es läuft (auf seiner Seite und im Dashboard). Es ist eine aktive Anzeige, kein Prozentbalken, weil `restic copy` keinen maschinenlesbaren Fortschritt bereitstellt.

!!! note "Von jedem Ort wiederherstellen"
    Jeder Container, jede VM, jedes Ordner-Set, der Flash und die App-Konfiguration listen ihre Backups als eine Zeitleiste über alle Orte, an denen ein Backup liegt. Ein nach B2 kopiertes Backup erscheint einmal, markiert mit jedem Ort, der es hält. Eine Wiederherstellung nimmt den ersten erreichbaren Ort, beginnend mit dem Repository, in das das Element geschrieben wird, und du kannst pro Zeile einen anderen Ort wählen. Off-site-Orte werden erst gelesen, wenn du sie öffnest. Das Löschen an einem Ort prüft zuerst die anderen und sagt, ob es die letzte Kopie war.

## Ablage pro Element {#placement}

Jede Container-, VM- und Ordner-Set-Karte hat eine Zeile **Ablage** mit drei Segmenten:

- **Lokal** schreibt das Element in das unter **Gespeichert auf** gezeigte Repository und kopiert es nirgendwohin. Nutze es für Daten, die schon eine zweite Kopie haben, zum Beispiel eine Freigabe, die auf einem NAS liegt.
- **Lokal + Off-site** schreibt es zusätzlich dorthin und kopiert es zu den unter **Kopie nach** angehakten Zielen, ein Chip pro Off-site-Ziel des Bereichs. Ein Häkchen entfernen, und das Ziel bekommt von diesem Element nichts Neues mehr.
- **Nur Off-site** schreibt das Element direkt an den Ort unter **Senden an**, jeden Ort außer dem Speicherort des Bereichs. Wird der Bereich schon an diesen Ort kopiert, bekommt das Element ein direktes Repository neben den Kopien; sonst legt BombVault dort ein Repository für den Bereich an.

Der Ort ist ab dem ersten Backup des Elements fest, weil BombVault Backups nie zwischen Repositories verschiebt. Die Kopien können sich jederzeit ändern. Ein Ziel, das ein Element nicht mehr bekommt, behält seine vorhandenen Kopien und kürzt sie beim nächsten Off-site-Lauf des Bereichs auf seine eigene Aufbewahrung; **In B2 löschen** auf der Karte entfernt sie sofort. Existieren manche dieser Kopien nirgendwo sonst, listet die Bestätigung sie nach Datum auf und fragt nach dem Namen des Elements. Bei Append-only-Zielen lässt sich nicht löschen.

Unter der Zeile zeigt die Karte, wohin das Element geht und was tatsächlich da ist: an wie vielen Standorten es liegt, wann jedes Ziel zuletzt gesehen wurde, und ob 3-2-1 erfüllt ist. Ein Standort ist der Server mit den Originaldaten und jeder Ort an einem anderen Standort (siehe [Außer Haus](#off-the-premises-mark)). BombVault prüft Kopien und Standorte; den Teil "zwei Medien" von 3-2-1 prüft es nicht.

### Vorgaben pro Bereich

Die Karte **Domänen** unter Einstellungen, Speicher hat eine Zeile pro Bereich. **Kopiert nach** gilt sofort für jedes Element ohne eigene Wahl, sowie für die Projektordner von Compose-Stacks. Sobald ein Bereich Backups hat, gilt **Gespeichert in** für ein neues Element bei seinem ersten Backup, und eine Änderung verschiebt keine Backups. Vor dem Speichern nennt die Zeile jeden Ort, der Elemente gewinnt oder verliert, und wie viele Snapshots das bedeutet. In der Rückfrage gibt es den Schalter **Auf Einträge ohne Backups anwenden**, der zusätzlich jedes Element ohne bisheriges Backup auf die neue Vorgabe setzt. **Ausnahmen** listet die Elemente mit eigener Wahl.

Ein neu angehakter Ort unter **Kopiert nach** bekommt jedes Element, das nicht auf Lokal steht. Die Bestätigung nennt die Anzahl der Elemente und, wo bekannt, wie viel Verlauf das ist.

### Direkte Repositories

Wählst du unter Nur Off-site einen Ort, an den der Bereich schon kopiert wird, fragt BombVault einmal nach, legt dann ein direktes Repository neben den Kopien an, zum Beispiel `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, und richtet das Element darauf aus. Bei einem Kopieziel ohne Ort öffnet die Wahl einen Dialog mit einer vorgeschlagenen Adresse und einem Verbindungstest, der nichts anlegt, und **Anlegen und nutzen** erstellt das Repository. Ein direktes Repository übernimmt Schlüssel, Speicherklasse, Grenzen, Append-only-Einstellung und Aufbewahrung des Ortes und ändert sich mit ihnen. Kann ein neuer Schlüssel für den Ort es nicht öffnen, behält das direkte Repository seinen bisherigen Schlüssel, und das Speichern sagt das. Seine Snapshots tragen das Tag `bv:direct`, und jeder andere Aufbewahrungslauf lässt sie stehen, sodass ein direktes Repository, das die Verbindung zu seinem Ort verloren hat, nie nach den lokalen Regeln altert. Ein B2-Schlüssel, der auf einen Ordner beschränkt ist, muss die Adresse des Ortes abdecken und nicht nur den Ordner des Bereichs, sonst ist der Ordner daneben nicht erreichbar.

### Außer Haus {#off-the-premises-mark}

Eine Kopie zählt nur dann als eigener Standort, wenn ihr Ort an einem anderen Standort steht. Ein Cloud-Ort zählt immer, ein Ordner auf diesem Unraid nie; bei einem NAS, einem rest-server oder einem SFTP-Server beantwortest du in den Details des Ortes **Wo steht das Gerät?** mit **Hier im Haus** oder **An einem anderen Ort**. Die Antwort zählt nur bei Standorten und 3-2-1 auf den Karten und im Dashboard. Sie ändert keine Kopie.

### Nach einem Neuaufbau

Kopier-Entscheidungen leben in BombVaults eigenen Einstellungen. Nach einem Neuaufbau über Backups entdecken ohne wiederhergestelltes `/config` sind sie weg, und alles zu kopieren würde die Elemente, die du ausgelassen hattest, wieder nach B2 schicken. Die Off-site-Replikation jedes neu aufgebauten Bereichs pausiert deshalb. Das Dashboard zeigt es in Gelb, und die Zeile des Bereichs auf der Karte Domänen bietet **Vorgabe bestätigen** mit einer Vorschau, was der nächste Lauf kopiert, und den Namen in den Backups, die keinen Eintrag haben, die du dort auslassen kannst. Nur die Bestätigung beendet die Pause; eine Einstellungsdatei zu importieren bringt Regeln und Vorgaben zurück, beendet die Pause aber nicht.

## Ein Bereich an einem entfernten Ort {#remote-primary-repositories}

Ein Bereich muss nicht lokal gespeichert sein. Solange an seinem bisherigen Ort keine Backups liegen, wählst du auf der Karte Domänen unter **Gespeichert in** einen entfernten Ort, und der Bereich sichert direkt dorthin, ohne lokale Kopie und ohne Kopierschritt. Das entfernte Repository ist dann die einzige Kopie, solange der Bereich nicht zusätzlich an einen anderen Ort kopiert wird. Jeder entfernte Ort bringt dieselben Schutzmaßnahmen mit:

- **Einen Verbindungstest**, bevor irgendetwas geschrieben wird.
- **Bandbreitengrenzen** für die Sicherung selbst, dieselben Schalter `--limit-upload` und `--limit-download`, die auch eine Kopie nutzt.
- **Append-only-Schutz**, geprüft mit demselben aktiven Manipulationstest. Ist er an, kürzt BombVault das Repository nie, weil mit den Zugangsdaten auf dieser Kiste niemand die einzige Kopie der Sicherung löschen können darf.
- **Ein Wachstumsbudget**, abgeleitet aus demselben Größentrend, den die Speicher-Karte verfolgt.

Ein Bereich an einem entfernten Ort ist wie ein lokaler die Quelle seiner Kopien; siehe [Kopien zwischen Orten mit unterschiedlichen Zugangsdaten](storage-places.md#different-credentials).

!!! note "Zugangsdaten gehören zum Ort"
    Ein entfernter Ort hat seine eigenen Zugangsdaten. Ein Ort, der mit den gemeinsamen Cloud-Zugangsdaten eingerichtet wurde, benutzt sie weiter, bis sein Zugang in seinen Details geändert wird.

### SMB und WebDAV ohne Host-Mount {#smb-webdav}

Die rclone-Variante des Fensters **Ort hinzufügen** hat ein Formular für eine Windows- oder Samba-Freigabe und für einen WebDAV-Server (Nextcloud, ownCloud, SharePoint oder einen anderen). Trag einen kurzen Namen ein, dazu Host und Freigabe (SMB) oder URL und Servertyp (WebDAV), Benutzer und Passwort, und BombVault schreibt den rclone-Abschnitt für dich. rclone verschleiert das Passwort selbst, bevor es gespeichert wird; ein Ziel mit einem Namen, den es schon gibt, ersetzt diesen Abschnitt, statt einen zweiten anzulegen.

Das neue Remote erscheint dann in der Liste der Remotes im Formular, wo du es für den Ort auswählst. Die Freigabe ist das erste Pfadsegment, nicht Teil des Namens.

Das ist der bessere Weg, als die Freigabe auf Unraid einzuhängen: restic rät davon ab, ein Repository auf einer eingehängten CIFS-Freigabe zu halten, und hier wird nichts eingehängt. NFS fehlt im Formular, weil weder restic noch rclone ein NFS-Backend hat; für NFS hängst du den Export auf dem Host ein und fügst ihn als Ort mit **Andere Freigabe** hinzu.

## Unveränderliches (Append-only) Off-site

Markiere ein Off-site-Repo als Append-only, sodass Ransomware oder ein kompromittierter Host deine Backups nicht löschen oder überschreiben kann. Die Gegenseite (ein `restic/rest-server`, der im `--append-only`-Modus läuft) **erzwingt** es. BombVault **verifiziert** es nur und zeigt niemals grün allein auf eine Konfigurationsbehauptung hin.

Das Fenster **Ort hinzufügen** enthält ein fertiges Rezept für einen rest-server im Append-only-Modus, mit einem Benutzer für dieses BombVault. Bei einem rest-server-Ort mit eingeschaltetem **Append-only** führt **Auf append-only prüfen** in den Details des Ortes den Manipulationstest für jeden Bereichspfad, jede eingeschaltete Kopie und jedes Repository an diesem Ort aus und gibt eine Antwort für den ganzen Ort. So lässt sich Append-only-Off-site einrichten, ohne Konfigurationsdateien von Hand zu bearbeiten.

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

An einem Ort prüft **Auf append-only prüfen** jeden Bereichspfad, jede eingeschaltete Kopie und jedes Repository dort mit dessen eigenen Zugangsdaten und fasst die Urteile zu einer Antwort zusammen: Nimmt ein einziges Repository ein Löschen an, lautet sie für den ganzen Ort *Löschen möglich*.

## DR-Übungen

BombVault bietet zwei Stufen des Nachweises, dass deine Backups tatsächlich wiederherstellbar und nicht nur vorhanden sind.

- **Wiederherstellungs-Prüfübungen (lokal).** BombVault führt regelmäßig `restic check --read-data-subset` aus (begrenzt, nie eine plattenfüllende Vollwiederherstellung) und zeigt pro Bereich ein Abzeichen *zuletzt als wiederherstellbar geprüft*. Die Taktung liegt unter Einstellungen, Zeitpläne; das Abzeichen unter Einstellungen, Integrität.
- **DR-Übungen (Off-site).** BombVault stellt ein echtes Ziel aus dem Off-site-Repo in eine Wegwerf-Sandbox wieder her, prüft es Datei für Datei und Byte für Byte und räumt dann auf. Dies beweist, dass du aus dem Off-site wiederherstellen kannst, nicht nur, dass das Repo antwortet. Geübt wird nur mit Orten an einem anderen Standort, denn eine Kopie im selben Haus sagt nichts darüber, ob du den Verlust des Hauses überstehst. Wird ein Bereich an mehrere davon kopiert, kommt pro geplantem Lauf einer an die Reihe, und das Dashboard nennt den Ort der letzten Übung.

Die **Ransomware-Schutz-Scorecard** im Dashboard fasst dies zu einer grün / gelb / rot-Haltung pro Bereich zusammen, mit einer altersgestempelten Checkliste (Off-site konfiguriert, Append-only verifiziert, Replikation aktuell, Wiederherstellungsübung bestanden, Verschlüsselung an, Kürzungsstrategie gesetzt). Jede rote Zeile verlinkt tief zur Behebung, und die Karte wird nur bei verifizierten Fakten grün.

## Empfänger-Dashboard (die empfangende Seite)

![Die empfangende Seite, nur lesend beobachtet, mit einer Integritätsprüfung auf dieser Hardware.](assets/screenshots/receiver.png)

*Die empfangende Seite, nur lesend beobachtet, mit einer Integritätsprüfung auf dieser Hardware.*

Alles oben ist die *sendende* Seite. Auf der Box, die unveränderliche Off-site-Kopien von einem anderen BombVault **empfängt**, gibt dir das Empfänger-Dashboard eine unabhängige, schreibgeschützte Überwachung dieser Repositorys auf der empfangenden Hardware, sodass ein stiller Fehler am fernen Ende nicht unbemerkt bleibt.

Schalte den **Empfänger**-Schalter in den Einstellungen ein, um einen **Empfänger**-Tab freizulegen. Er ist standardmäßig aus; aktiviere ihn nur auf einer Box, die tatsächlich unveränderliche Off-site-Backups empfängt. Registriere dann ein empfangenes Repository (schreibgeschützt, geöffnet mit dem Schlüssel der sendenden Instanz), um zu erhalten:

- **Einen nach Quelle gruppierten Snapshot-Bestand**, sodass du genau sehen kannst, welche Container, VMs und Dateisätze eingetroffen sind.
- **Zuletzt empfangen** pro Quelle, sodass du weißt, wie frisch jede ist.
- **Ein unabhängiges `restic check`**, das auf der empfangenden Hardware läuft, sodass die Integrität dort geprüft wird, wo die Daten tatsächlich liegen, nicht nur beim Sender.
- **Einen Totmannschalter:** ein Alarm, wenn eine Quelle innerhalb eines von dir gesetzten Fensters aufhört zu senden.
- **Integritätsalarme:** ein Alarm, wenn eine Prüfung auf der empfangenden Seite fehlschlägt.

Der Empfänger ist strikt schreibgeschützt. Er schreibt niemals in das empfangene Repository, sodass er die Append-only-Garantie, auf die sich der Sender verlässt, nie brechen kann.

## Durchgerechnetes Beispiel: zwei Unraid-Kisten, Ende zu Ende

Oben stehen die Einzelteile. Hier ist ein vollständiger Aufbau mit echten Werten, weil sich Teile leichter zusammensetzen lassen, wenn man sie einmal zusammengesetzt gesehen hat.

Zwei Kisten: **TOWER** betreibt die Container und schiebt die Backups, **VAULT** nimmt sie an und erzwingt die Unveränderlichkeit. Setze deine eigenen Namen, Adressen und Freigabepfade ein.

**1. Auf VAULT den Append-only-Server aufsetzen.** In BombVault auf TOWER *Einstellungen → Speicher* öffnen, auf **Ort hinzufügen** klicken, **rest-server** wählen und auf **Rezept zeigen** klicken. Den Block **Unraid-Vorlage** kopieren, auf VAULT als `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml` speichern, dann *Docker → Add Container* und **rest-server** aus der Vorlagenliste wählen. Vor dem Start die angezeigte `htpasswd`-Zeile auf VAULT in `/mnt/user/appdata/rest-server/.htpasswd` schreiben. Das Passwort wird nur einmal angezeigt und nie gespeichert; das Rezept hat es zusammen mit dem Benutzer schon ins Formular auf TOWER eingetragen, lass dieses Fenster also offen. Die `htpasswd`-Zeile trägt dasselbe Passwort, für dich schon bcrypt-gehasht, du musst also selbst nichts hashen.

    `--append-only` im OPTIONS-Feld stehen lassen. Ohne das ist VAULT wieder nur eine gewöhnliche Freigabe.

**2. Auf TOWER den Ort hinzufügen.** VAULTs Adresse `http://VAULT:8000` neben Benutzer und Passwort eintragen, die das Rezept ausgefüllt hat, dann auf **Verbindung testen** klicken. BombVault baut daraus die Adresse:

    rest:http://VAULT:8000/tower

Das erste Pfadsegment ist der htpasswd-Benutzer, hier `tower`, und jeder Bereich bekommt darunter seinen Ordner, zum Beispiel `rest:http://VAULT:8000/tower/container`. **Wo steht das Gerät?** mit **An einem anderen Ort** beantworten, auf **Hinzufügen** klicken und den Ort unter **Kopiert nach** für die Bereiche anhaken, die dorthin sollen.

**3. Auf TOWER Append-only einschalten.** In den Details des Ortes unter **Schutz** den Schalter **Append-only** einschalten, dann auf **Auf append-only prüfen** klicken. Der Test prüft jeden Bereichspfad, jede Kopie und jedes Repository an diesem Ort und gibt eine Antwort für den ganzen Ort, die *Löschen verweigert* lauten muss. Was die Antworten bedeuten:

| Ergebnis | Was passiert ist |
| --- | --- |
| **Löschen verweigert** | VAULT hat das Löschen verweigert. Das ist der einzige bestandene Zustand. |
| **Löschen möglich** | VAULT hat ein Löschen angenommen. `--append-only` fehlt oder wurde entfernt. |
| eine Meldung statt eines Ergebnisses | Der Test konnte nicht laufen. Meist ist die Adresse nicht die, die restic selbst benutzt, oder die Zugangsdaten haben sich geändert. Es wird nichts vermerkt und kein Alarm ausgelöst. |

**4. Auf VAULT ansehen, was ankommt.** *Einstellungen → Empfänger* einschalten, den Reiter **Empfänger** öffnen und das Repository schreibgeschützt registrieren.

!!! warning "Der Ort ist ein Pfad **innerhalb** des Containers, relativ zum Host-Mount geschrieben"
    Trage `user/appdata/rest-server/tower/container` ein, **nicht** `/mnt/user/appdata/…`. BombVault läuft in einem Container, in dem das `/mnt` des Hosts an anderer Stelle eingehängt ist; ein absoluter Host-Pfad existiert dort nicht. Fügst du trotzdem einen ein, nennt BombVault dir den relativen Pfad, den du stattdessen brauchst.

    Der **sendende APP_KEY** ist der Schlüssel von TOWER, nicht der von VAULT. Du findest ihn auf TOWER unter *Einstellungen → System*.

**5. Wenn du magst, mach es gegenseitig.** Dieselben fünf Schritte in die andere Richtung: ein rest-server auf TOWER, der VAULTs Kopie annimmt. Dann erzwingt jede Kiste die Unveränderlichkeit für die andere, und keine kann die Backups der anderen löschen.

## Geführte Wiederherstellung

Ein eigener **Recovery**-Tab führt eine frische oder neu aufgebaute Installation durch den Katastrophenfall, an einem Ort:

1. **Prüft, dass BombVault deine Backups lesen kann** (der Verschlüsselungsschlüssel-Fallstrick vorab).
2. **Stellt BombVaults eigene Einstellungen wieder her**, sodass die Backup-Pfade, Off-site-Ziele und Zugangsdaten, die der Rest des Ablaufs braucht, vorausgefüllt sind. Das Einstellungs-Backup wird von dem Ort gelesen, den die Zeile „Selbst-Backup“ unter **Gespeichert in** nennt, oder von der Kopie des Selbst-Backups unter **Kopiert nach**; der Schritt zeigt diesen Ort mit seiner Adresse. Um von einem anderen Ort zu lesen, änderst du zuerst die Zeile „Selbst-Backup“ in Schritt 3. Angewendet wird die Wiederherstellung per Selbst-Neustart über den Docker-Socket, sodass die laufende Einstellungsdatenbank nie unter einem offenen Handle überschrieben wird.
3. **Hängt deine vorhandenen Backups an**, über die Zeilen der Karte Domänen: In der Zeile jedes Bereichs wählst du unter **Gespeichert in** den Ort, an dem seine Backups liegen, und unter **Kopiert nach** die Orte mit seinen Kopien. Einen Ort, den noch keine Zeile anbietet, etwa eine Freigabe, einen Server oder einen Cloud-Bucket, verbindest du mit **Ort hinzufügen**, demselben Fenster wie unter Einstellungen, Speicher. **Verbinden & prüfen** testet danach, ob sich die Backups lesen lassen.
4. **Entdeckt** die darin gespeicherten Container, VMs, Dateisätze und ZFS-Datasets.
5. **Stellt Container und VMs in einem Rutsch wieder her** (gestoppt belassen, sodass du sie bewusst startest) und listet Dateisätze und ZFS-Elemente auf, die du einzeln wiederherstellst; ZFS-Elemente kommen ausgeschaltet zurück. Dein Recovery-Kit ist einen Klick entfernt.

!!! note "Off-site-Kopien warten nach einem Neuaufbau"
    Wenn Schritt 4 Einträge ohne die alten Einstellungen neu aufbaut, pausiert die Off-site-Replikation dieser Bereiche, bis die Ablage-Vorgabe bestätigt ist. Siehe [Ablage pro Element](#placement).

!!! tip "Geplante Migration versus Katastrophe"
    Die geführte Wiederherstellung stellt BombVaults eigene Einstellungen aus einem Backup wieder her. Für einen *geplanten* Umzug auf eine neue Box kannst du deine Konfiguration stattdessen direkt mit der Karte **Einstellungen exportieren und importieren** (eine portable JSON-Datei) mitnehmen. Siehe [Konfiguration](configuration.md#portable-settings-export-and-import).

### Wiederherstellung aus einem anderen BombVault-Repo

Eine separate Karte im **Recovery**-Tab öffnet das Repo einer *anderen* BombVault-Instanz (eine unter `/mnt` eingehängte Freigabe oder eine Remote-URL) mit **dem `APP_KEY` dieser Instanz**, in einer einmaligen, schreibgeschützten Sitzung. Durchstöbere die dort gespeicherten Container, VMs und Dateisätze, wähle einen Snapshot und stelle ihn wieder her, und das wiederhergestellte Objekt wird ein normaler lokaler Container, eine VM oder ein Dateisatz. Es wird niemals etwas in das andere Repo geschrieben, und deine eigenen Backup-Einstellungen bleiben unangetastet (die Sitzung lebt im Speicher und läuft von selbst ab). Einen Container von Server A auf Server B zu verschieben bedeutet nicht mehr, deine Repo-Einstellungen umzustellen und danach zurückzudrehen. Live-Server-zu-Server-Föderation ist ausdrücklich außerhalb des Umfangs; dies ist ein bewusster Einmal-Pull.

## Wiederherstellungspaket für den Verschlüsselungsschlüssel

Dies ist das Stück, das eine Notfallwiederherstellung selbst dann möglich macht, wenn kein BombVault läuft.

Ein Klick lädt den **Master-Key**, das **abgeleitete restic-Passwort** und die **genauen Repo-Orte und -Befehle** herunter, sodass du direkt mit dem restic-CLI auf jeder Maschine wiederherstellen kannst. Eine Dashboard-Erinnerung nervt, bis du es gespeichert hast.

!!! danger "Bewahre das Recovery-Kit off-box auf"
    Das Kit enthält das Geheimnis, das deine Backups entschlüsselt. Bewahre es an einem sicheren Ort getrennt vom Server auf (ein Passwortmanager, eine gedruckte Kopie im Safe). Wenn du sowohl BombVault als auch `APP_KEY` ohne Recovery-Kit verlierst, können deine verschlüsselten Backups nicht wiederhergestellt werden.

!!! warning "Der neueste Snapshot ist nicht immer der richtige"
    Seit restic 0.17 zeigt `restic snapshots` die Größe jedes Snapshots. Nach einem Datenverlust kann der neueste Snapshot der geleerte sein, stelle also keinen Snapshot wieder her, der viel kleiner ist als die davor. Nach Ransomware kann es der verschlüsselte in der üblichen Größe sein. Wenn BombVault noch läuft, sieh zuerst auf der Seite **Anomalien** nach: Sie nennt das letzte gute Backup. Für eine Wiederherstellung braucht es keine Anomalie-Daten von BombVault, und die Aufbewahrungspause behält immer nur mehr Snapshots.

### Das Kit versiegeln

Wenn du die age-Verschlüsselung für die schlichten Exporte eingeschaltet hast (Einstellungen), wird auch das Kit damit versiegelt und als `bombvault-recovery-kit.md.age` heruntergeladen. Es ist ASCII-armored statt binär, also weiterhin reiner Text: Du kannst es wie bisher in einen Passwortmanager einfügen oder ausdrucken, nur ist der Inhalt ohne deinen Schlüssel nicht lesbar.

!!! warning "Den age-Schlüssel nicht im Kit aufbewahren"
    Um ein versiegeltes Kit zu öffnen, brauchst du deinen **privaten** age-Schlüssel. Bewahre ihn an einem Ort auf, der nicht vom Kit selbst abhängt, sonst musst du zwei Dinge wiederherstellen statt einem. Das Versiegeln lohnt sich, wenn das Kit an einem Ort liegt, den du nicht ganz im Griff hast (ein geteilter Passwortmanager, Cloud-Notizen, ein Ausdruck im Büro); ein Kit in deinem eigenen Safe schützt schon der Safe.

    Ist die Verschlüsselung an und kein brauchbarer Empfänger eingerichtet, wird der Download gleich verweigert. BombVault gibt den Master-Key nie ersatzweise im Klartext heraus.

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
