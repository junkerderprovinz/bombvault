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

## Unveränderliches (Append-only) Off-site

Markiere ein Off-site-Repo als Append-only, sodass Ransomware oder ein kompromittierter Host deine Backups nicht löschen oder überschreiben kann. Die Gegenseite (ein `restic/rest-server`, der im `--append-only`-Modus läuft) **erzwingt** es. BombVault **verifiziert** es nur und zeigt niemals grün allein auf eine Konfigurationsbehauptung hin.

Der Assistent der **geführten Off-site-Einrichtung** führt dich von der Backend-Wahl (rest-server / rclone / S3) über ein einsatzbereites rest-server-Deploy-Snippet, einen Verbindungstest, den Unveränderlichkeits-Schalter (der den Manipulationstest sofort ausführt) bis zu einer Aufbewahrungsstrategie, sodass Append-only-Off-site ohne Handbearbeitung von Konfigurationen erreichbar ist.

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

Alles oben ist die *sendende* Seite. Auf der Box, die unveränderliche Off-site-Kopien von einem anderen BombVault **empfängt**, gibt dir das Empfänger-Dashboard eine unabhängige, schreibgeschützte Überwachung dieser Repositorys auf der empfangenden Hardware, sodass ein stiller Fehler am fernen Ende nicht unbemerkt bleibt.

Schalte den **Empfänger**-Schalter in den Einstellungen ein, um einen **Empfänger**-Tab freizulegen. Er ist standardmäßig aus; aktiviere ihn nur auf einer Box, die tatsächlich unveränderliche Off-site-Backups empfängt. Registriere dann ein empfangenes Repository (schreibgeschützt, geöffnet mit dem Schlüssel der sendenden Instanz), um zu erhalten:

- **Einen nach Quelle gruppierten Snapshot-Bestand**, sodass du genau sehen kannst, welche Container, VMs und Dateisätze eingetroffen sind.
- **Zuletzt empfangen** pro Quelle, sodass du weißt, wie frisch jede ist.
- **Ein unabhängiges `restic check`**, das auf der empfangenden Hardware läuft, sodass die Integrität dort geprüft wird, wo die Daten tatsächlich liegen, nicht nur beim Sender.
- **Einen Totmannschalter:** ein Alarm, wenn eine Quelle innerhalb eines von dir gesetzten Fensters aufhört zu senden.
- **Integritätsalarme:** ein Alarm, wenn eine Prüfung auf der empfangenden Seite fehlschlägt.

Der Empfänger ist strikt schreibgeschützt. Er schreibt niemals in das empfangene Repository, sodass er die Append-only-Garantie, auf die sich der Sender verlässt, nie brechen kann.

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

Weil Recovery-Definitionen **in** jedem Repo liegen (`<repo>/def`, `<repo>/vm-def`), ist ein kopierter Repo-Ordner vollständig eigenständig, sodass das Kit plus das Repo alles ist, was eine Bare-Metal-Wiederherstellung braucht.
