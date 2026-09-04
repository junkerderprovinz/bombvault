# Off-site & Wiederherstellung

Lokale Backups schützen dich vor einem verlorenen Container oder einem schlechten Update. Off-site-Replikation und ein getestetes Recovery-Kit schützen dich vor der ganzen Box, Ransomware oder einem Brand. Diese Seite behandelt das Replizieren ins Off-site, das Manipulationssicher-Machen dieser Kopie, den Nachweis der Wiederherstellbarkeit und die Wiederherstellung, wenn BombVault selbst verschwunden ist.

## Off-site-Replikation

Behalte das schnelle lokale Backup und füge eine oder mehrere Off-site-Repliken hinzu. Setze ein Repo pro Bereich im Tab **Einstellungen, Off-site**. BombVault repliziert neue Snapshots dorthin mit `restic copy` auf Best-Effort-Basis, sodass ein Off-site-Aussetzer das lokale Backup nie fehlschlagen lässt. Das lokale Repo bleibt primär.

- **Mehrere Off-site-Ziele pro Bereich.** Jeder Bereich (Container, VMs, Flash, Config und Dateisätze) kann gleichzeitig an mehrere Off-site-Ziele replizieren, nicht nur eines, sodass du zum Beispiel einen rest-server auf der Box eines Freundes und einen S3-Bucket parallel behalten kannst. Füge zusätzliche Ziele unter Einstellungen, Off-site hinzu, jedes mit eigenem Repository, S3-Speicherklasse, Append-only-Flag, Aufbewahrung und Wachstumsbudget. Eine bestehende einzelne Off-site-Einrichtung wird unangetastet als erstes Ziel übernommen, und jedes Ziel eines Bereichs repliziert nach dem Off-site-Zeitplan dieses Bereichs.
- **Off-site-Zeitplan pro Bereich** (neben jedem anderen Zeitplan unter Einstellungen, Zeitpläne bearbeitet): lasse ihn leer, um nach jedem lokalen Backup zu replizieren, oder setze eine Taktung (zum Beispiel `weekly Sun 03:00`), um seltener ins Off-site zu liefern, als du lokal sicherst. Ein Button **Jetzt replizieren** deckt Läufe auf Abruf ab.
- **Off-site-Aufbewahrung** liegt unter Einstellungen, Off-site, sodass du Off-site-Kopien länger als Archiv behalten kannst. Lasse die Richtlinie ganz auf null, um Off-site-Snapshots nie automatisch zu kürzen.
- **Bandbreitenlimits** (Einstellungen, Off-site) begrenzen die restic-Upload-/Download-Rate, sodass die Replikation dein WAN nicht auslastet.
- Eine **Replikationsanzeige** zeigt, welcher Bereich gerade repliziert, während es läuft (auf seiner Seite und im Dashboard). Es ist eine aktive Anzeige, kein Prozentbalken, weil `restic copy` keinen maschinenlesbaren Fortschritt bereitstellt.

!!! note "Direkt aus dem Off-site wiederherstellen"
    Jeder Backup-Browser hat einen Schalter **Lokal / Off-site**, sodass du bei verlorenem oder beschädigtem lokalem Repo direkt aus der Off-site-Replik auflisten und wiederherstellen kannst. Das Löschen erfolgt pro Quelle: Ein Backup zu entfernen betrifft nur die Kopie, die du gerade ansiehst.

## Entfernte primäre Repositories {#remote-primary-repositories}

Der Sicherungspfad einer Domäne (Einstellungen, Pfade & Speicher) ist nicht auf einen lokalen Ordner beschränkt: richte ihn direkt auf ein restic-Remote (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`, `rclone:remote:bucket/pfad`), und BombVault sichert unmittelbar dorthin, ohne getrennte lokale Kopie und ohne Replikationsschritt. Das ist eine wirklich andere Form als die Off-site-Replikation weiter oben: dort ist das lokale Repo primär und das Off-site-Repo ein Archiv davon nach bestem Bemühen; hier **ist** das entfernte Repo das primäre und die einzige Kopie, solange du für diese Domäne nicht zusätzlich eine Off-site-Replikation (oder ein zweites Remote) einrichtest.

Jedes der fünf Pfadfelder (Container, VMs, Flash, Konfiguration, Dateien) hat direkt daneben einen Schalter **Lokal / Entfernt**:

- **Lokal** zeigt den gewohnten Ordner-Browser.
- **Entfernt** tauscht ihn gegen ein einfaches URL-Feld, dazu eine Schaltfläche, die denselben Dialog für Verbindungstest und Zugangsdaten öffnet, den auch Off-site-Ziele verwenden, nur eben für dieses primäre Repo. Von dort bekommst du:
    - **Einen Verbindungstest** gegen den echten Pfad, bevor du dich darauf verlässt.
    - **Bandbreitengrenzen** (Hoch- und Herunterladen), damit eine geplante Sicherung auf ein entferntes primäres Repo nicht deine WAN-Leitung auslastet: dieselben restic-Schalter `--limit-upload` und `--limit-download`, die die Off-site-Replikation nutzt, angewandt auf die Sicherung selbst.
    - **Append-only-Schutz (Unveränderlichkeit)**, geprüft mit demselben aktiven Manipulationstest (eine echte DELETE-Probe gegen die Gegenseite), den auch Off-site-Ziele bekommen. Ist er an, weigert sich BombVault, das Repo selbst zu bereinigen: weil dahinter keine getrennte lokale Kopie steht, dürfen die Zugangsdaten auf dieser Kiste nicht in der Lage sein, die einzige Kopie der Sicherung zu löschen.
    - **Einen Alarm für das Wachstumsbudget**, abgeleitet aus demselben Trend der Repo-Größe, den die Speicher-Karte ohnehin verfolgt.

Nichts davon ist Pflicht: ein von Hand eingetragener entfernter Pfad ohne gespeicherte Sicherheitseinstellungen sichert genau so wie bisher (unbegrenzte Bandbreite, bereinigbar, kein Budgetalarm). Der Sicherheitsdialog ist für den Fall da, dass du dieselben Schutzmaßnahmen willst, die eine Off-site-Kopie bekommt, ohne dafür extra ein Off-site-Ziel einrichten zu müssen.

!!! note "Cloud- und REST-Zugangsdaten werden geteilt"
    Ein entferntes primäres Repo meldet sich mit denselben S3-/REST-Zugangsdaten an, die unter Einstellungen, Off-site, Cloud-Zugangsdaten hinterlegt sind. Einen getrennten Speicher für Zugangsdaten primärer Repos gibt es nicht.

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

- **Wiederherstellungs-Prüfübungen (lokal).** BombVault führt regelmäßig `restic check --read-data-subset` aus (begrenzt, nie eine plattenfüllende Vollwiederherstellung) und zeigt pro Bereich ein Abzeichen *zuletzt als wiederherstellbar geprüft*. Die Taktung liegt unter Einstellungen, Zeitpläne; das Abzeichen unter Einstellungen, Integrität.
- **DR-Übungen (Off-site).** BombVault stellt ein echtes Ziel aus dem Off-site-Repo in eine Wegwerf-Sandbox wieder her, prüft es Datei für Datei und Byte für Byte und räumt dann auf. Dies beweist, dass du aus dem Off-site wiederherstellen kannst, nicht nur, dass das Repo antwortet.

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

**1. Auf VAULT den Append-only-Server aufsetzen.** In BombVault auf TOWER unter *Einstellungen → Off-site → geführte Einrichtung* **rest-server** wählen und das Rezept erzeugen. Den Reiter **Unraid-Vorlage (XML)** kopieren, auf VAULT als `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml` speichern, dann *Docker → Add Container* und **rest-server** aus der Vorlagenliste wählen. Vor dem Start die angezeigte `htpasswd`-Zeile auf VAULT in `/mnt/user/appdata/rest-server/.htpasswd` schreiben. Das Einmal-Passwort wird nur einmal angezeigt und nie gespeichert, kopiere es jetzt.

    `--append-only` im OPTIONS-Feld stehen lassen. Es ist der ganze Sinn der Sache: ohne das ist VAULT wieder eine gewöhnliche Freigabe.

**2. Auf TOWER das Off-site-Repo darauf richten.** Die Repo-URL folgt dem Muster, das das Rezept ausgibt:

    rest:http://VAULT:8000/bombvault-containers/containers

Das erste Pfadsegment ist der htpasswd-Benutzer, das zweite das Repository. Trage den erzeugten Benutzer und das Passwort als REST-Zugangsdaten des Ziels ein und führe den **Verbindungstest** aus.

**3. Auf TOWER „“ einschalten.** Der Manipulationstest läuft sofort und muss *geschützt* melden. Was die Antworten bedeuten:

| Ergebnis | Was passiert ist |
| --- | --- |
| **geschützt** | VAULT hat das Löschen verweigert. Das ist der einzige bestandene Zustand. |
| **NICHT geschützt** | VAULT hat ein Löschen angenommen. `--append-only` fehlt oder wurde entfernt. |
| **unentschieden** | Weder noch. Meist ist die URL nicht die, die restic selbst benutzt, oder die Zugangsdaten haben sich geändert. Es wird nichts vermerkt und kein Alarm ausgelöst. |

**4. Auf VAULT ansehen, was ankommt.** *Einstellungen → Empfänger* einschalten, den Reiter **Empfänger** öffnen und das Repository schreibgeschützt registrieren.

!!! warning "Der Ort ist ein Pfad **innerhalb** des Containers, relativ zum Host-Mount geschrieben"
    Trage `user/appdata/rest-server/bombvault-containers/containers` ein, **nicht** `/mnt/user/appdata/…`. BombVault läuft in einem Container, in dem das `/mnt` des Hosts an anderer Stelle eingehängt ist; ein absoluter Host-Pfad existiert dort nicht. Fügst du trotzdem einen ein, nennt BombVault dir jetzt den relativen Pfad, den du stattdessen brauchst.

    Der **sendende APP_KEY** ist der Schlüssel von TOWER, nicht der von VAULT. Du findest ihn auf TOWER unter *Einstellungen → System*.

**5. Wenn du magst, mach es gegenseitig.** Dieselben fünf Schritte in die andere Richtung: ein rest-server auf TOWER, der VAULTs Kopie annimmt. Dann erzwingt jede Kiste die Unveränderlichkeit für die andere, und keine kann die Backups der anderen löschen.

## Geführte Wiederherstellung

Ein eigener **Recovery**-Tab führt eine frische oder neu aufgebaute Installation durch den Katastrophenfall, an einem Ort:

1. **Stellt zuerst BombVaults eigene Einstellungen wieder her**, sodass die Backup-Pfade, Off-site-Ziele und Zugangsdaten, die der Rest des Ablaufs braucht, vorausgefüllt sind (angewendet per Selbst-Neustart über den Docker-Socket, sodass die laufende Einstellungsdatenbank nie unter einem offenen Handle überschrieben wird).
2. **Prüft, dass BombVault deine Backups lesen kann** (der Verschlüsselungsschlüssel-Fallstrick vorab).
3. Lässt dich **auf dein bestehendes Repo verweisen** (lokal oder Off-site).
4. **Entdeckt** die darin gespeicherten Container, VMs und Dateisätze.
5. **Stellt sie alle wieder her** (gestoppt belassen, sodass du sie bewusst startest), mit deinem Recovery-Kit einen Klick entfernt.

!!! tip "Geplante Migration versus Katastrophe"
    Die geführte Wiederherstellung stellt BombVaults eigene Einstellungen aus einem Backup wieder her. Für einen *geplanten* Umzug auf eine neue Box kannst du deine Konfiguration stattdessen direkt mit der Karte **Einstellungen exportieren und importieren** (eine portable JSON-Datei) mitnehmen. Siehe [Konfiguration](configuration.md#portable-settings-export-and-import).

### Wiederherstellung aus einem anderen BombVault-Repo

Eine separate Karte im **Recovery**-Tab öffnet das Repo einer *anderen* BombVault-Instanz (eine unter `/mnt` eingehängte Freigabe oder eine Remote-URL) mit **dem `APP_KEY` dieser Instanz**, in einer einmaligen, schreibgeschützten Sitzung. Durchstöbere die dort gespeicherten Container, VMs und Dateisätze, wähle einen Snapshot und stelle ihn wieder her, und das wiederhergestellte Objekt wird ein normaler lokaler Container, eine VM oder ein Dateisatz. Es wird niemals etwas in das andere Repo geschrieben, und deine eigenen Backup-Einstellungen bleiben unangetastet (die Sitzung lebt im Speicher und läuft von selbst ab). Einen Container von Server A auf Server B zu verschieben bedeutet nicht mehr, deine Repo-Einstellungen umzustellen und danach zurückzudrehen. Live-Server-zu-Server-Föderation ist ausdrücklich außerhalb des Umfangs; dies ist ein bewusster Einmal-Pull.

## Wiederherstellungspaket für den Verschlüsselungsschlüssel

Dies ist das Stück, das eine Notfallwiederherstellung selbst dann möglich macht, wenn kein BombVault läuft.

Ein Klick lädt den **Master-Key**, das **abgeleitete restic-Passwort** und die **genauen Repo-Orte und -Befehle** herunter, sodass du direkt mit dem restic-CLI auf jeder Maschine wiederherstellen kannst. Eine Dashboard-Erinnerung nervt, bis du es gespeichert hast.

!!! danger "Bewahre das Recovery-Kit off-box auf"
    Das Kit enthält das Geheimnis, das deine Backups entschlüsselt. Bewahre es an einem sicheren Ort getrennt vom Server auf (ein Passwortmanager, eine gedruckte Kopie im Safe). Wenn du sowohl BombVault als auch `APP_KEY` ohne Recovery-Kit verlierst, können deine verschlüsselten Backups nicht wiederhergestellt werden.

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
