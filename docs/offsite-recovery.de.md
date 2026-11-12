# Off-site und Wiederherstellung

!!! note "Off-site-Kopien warten nach einem Neuaufbau"
    Wenn der Notfall-Assistent oder **Sicherungen entdecken** Einträge ohne die alten Einstellungen neu aufbaut, warten die Off-site-Kopien dieser Bereiche, bis du ihre Sicherungen als eigene dieses Servers übernimmst. Siehe [Nach einem Neuaufbau](#after-a-rebuild).

Lokale Sicherungen schützen dich vor einem verlorenen Container oder einem schlechten Update. Off-site-Kopien und ein getestetes Recovery-Kit schützen dich vor dem Verlust der ganzen Box, vor Ransomware oder einem Brand. Diese Seite behandelt Speicherorte und Off-site-Kopien, wie man eine Kopie manipulationssicher macht, wie du beweist, dass du wiederherstellen kannst, das Koppeln mehrerer Server und wie du alles zurückholst, wenn BombVault selbst weg ist.

## Off-site-Kopien {#off-site-copies}

Behalte die schnelle lokale Sicherung und füge einen oder mehrere Speicherorte außer Haus hinzu. BombVault kopiert neue Snapshots mit `restic copy` nach bestem Bemühen dorthin, ein Off-site-Schluckauf lässt die lokale Sicherung also nie scheitern. In dieser Form schreibt jeder Bereich zuerst in seinen eigenen Ordner, und das Off-site-Repo ist eine Kopie. Der eigene Ordner muss nicht auf diesem Server liegen: Wie du ihn in einem Speicherort wie S3 oder einem rest-server hältst, steht unter [Der eigene Ordner eines Bereichs](#own-folder).

- **Mehrere Speicherorte pro Bereich.** Jeder Bereich (Container, VMs, Flash, Selbstbackup, Ordner und ZFS-Datasets) kann auf mehrere Off-site-Speicherorte zugleich kopieren, sodass du zum Beispiel einen rest-server bei einem Freund und einen S3-Bucket parallel halten kannst. Die Karte **Speicherorte je Bereich** auf der Seite Speicherorte schaltet jede Linie von einem Bereich zu einem Ort an oder aus.
- **Wann Kopien laufen.** Die Karte **Kopieren** eines Speicherorts legt **Wann kopieren** fest: **Nach jeder Sicherung** oder nach einem eigenen Plan, der die Snapshots sammelt und zu festen Zeiten kopiert. Unter **Einstellungen, Zeitplan, Off-site-Kopien pro Bereich** gibt **Ausnahme** einem Bereich einen eigenen Kopierplan.
- **Die Karte Off-site-Kopien.** Unter **Einstellungen, Speicherorte** hat jeder Bereich mit einer Off-site-Kopie eine Zeile in **Off-site-Kopien**, mit **Verbindung testen**, **Jetzt replizieren**, um eine Kopie auf Abruf zu starten, und **Einrichten** oder **Bearbeiten**. Die übrigen Kopiereinstellungen des Bereichs öffnen sich von dort in seinem Fenster.
- **Aufbewahrung** wird pro Speicherort unter **Wie lange aufheben** festgelegt, sodass du Off-site-Kopien als Archiv länger behalten kannst. Die Kopie im Off-site-Feld eines Bereichs folgt ihr wie jede andere Kopie, und ein Bereich kann unter **Regeln je Bereich** eigene Werte behalten. Lauter Nullen kürzen Off-site-Snapshots nie. Siehe [Wie lange aufheben](#keep).
- **Bandbreitenlimits** (**Off-site-Bandbreite** unter **Einstellungen, Speicherorte** oder die Karte **Kopieren** eines Speicherorts) begrenzen die Upload-/Download-Rate von restic, damit Kopien dein WAN nicht auslasten. Sie werden in KiB/s eingegeben, der Einheit, die restic nimmt, und jedes Feld zeigt dieselbe Rate beim Tippen in MB/s und Mbit/s: 2048 liest sich als etwa 2,0 MB/s (16,8 Mbit/s).
- **Kompression** ist **Aus**, **Automatisch** (die Vorgabe von restic) oder **Maximum**, an jedem Speicherort eingestellt und für einen Bereich unter dessen Regeln dort. Sicherungen, Kopien und Aufräumen schreiben damit, und das Recovery-Kit nennt sie, damit einfaches restic genauso weiterschreiben kann.
- **Streaming zuerst.** Während ein Mediaserver wie Plex, Jellyfin oder Emby streamt, laden Kopien mit einem niedrigeren Limit hoch und kehren einige Minuten nach dem Ende des Streams zum normalen zurück. Die Karte **Streaming** auf der Seite Speicherorte hat **Off-site-Kopien beim Streaming bremsen**, **Mediaserver** (Container mit Plex, Jellyfin oder Emby im Image-Namen, bis du wählst), **Stream ab (Mbit/s)**, **Upload-Grenze beim Streaming (KiB/s)** und **Zurück zum Normalen nach (Min.)**. BombVault liest den Datenverkehr aus Docker, ein Mediaserver im Host-Netzwerk lässt sich also nicht messen. REST, S3, B2, Azure, Google Cloud, Swift und rclone über HTTP werden mitten in einer Kopie langsamer. SFTP sowie lokale oder gemountete Ordner übernehmen das niedrigere Limit bei ihrem nächsten Kopierschritt.
- **Cold Storage (S3).** Ein S3-Speicherort kann eine günstigere **Speicherklasse** nutzen, die restic noch sofort lesen kann, etwa Standard-IA, One Zone-IA, Intelligent-Tiering oder Glacier Instant Retrieval. Glacier Flexible Retrieval und Deep Archive bleiben außen vor, weil restic daraus nicht wiederherstellen kann. Ein rclone-Remote legt seine Klasse in der rclone-Konfiguration fest.
- **Gleichzeitige Verbindungen.** An einem S3-, B2-, Azure-, Google-Cloud-, Swift-, rest-server-, SFTP- oder rclone-Speicherort legt **Gleichzeitige Verbindungen** auf seiner Karte **Verbindung** fest, wie viele Verbindungen restic auf einmal öffnet, von 1 bis 32. Bleibt das Feld leer, behält restic seine Vorgabe von 5. Mehr können eine lange oder langsame Leitung schneller machen, weniger schonen einen Anbieter, der drosselt. Sicherungen, Kopien, Prüfungen, Aufräumen und Wiederherstellungen an diesem Ort nutzen den Wert, und ein Einstellungsexport nimmt ihn mit. Ein lokaler oder gemounteter Ordner hat diese Einstellung nicht.
- **Wachstumsbudget.** **Wachstumsbudget (GB, 0 = aus)** im Kopierfenster eines Bereichs schlägt Alarm, wenn das Repository über diese Größe hinauswächst. Es löscht nie etwas und passt daher zu einer Append-only-Kopie, die von hier aus nichts aufräumt.
- Während eine Kopie läuft, sagt der Statuskopf der Übersicht **Off-site-Kopie läuft**. Das ist eine aktive Anzeige, kein Prozentbalken, weil `restic copy` keinen maschinenlesbaren Fortschritt liefert.

!!! note "Von jedem Ort wiederherstellen"
    Jeder Eintrag listet seine Sicherungen als eine Zeitleiste über alle Orte auf, an denen eine Sicherung liegt, auf seinem Tab **Sicherungen**. Eine Sicherung, die nach B2 kopiert wurde, erscheint einmal, markiert mit jedem Ort, der sie hält. Eine Wiederherstellung nimmt den ersten Ort, den sie erreichen kann, beginnend mit dem Repository, in das der Eintrag schreibt, und du kannst pro Snapshot einen anderen Ort wählen. Off-site-Speicherorte werden nur gelesen, wenn du auf **Prüfen** klickst, weil das Auflisten von Cloud-Speicher eine Weile dauert. Das Löschen an einem Ort prüft zuerst die anderen und sagt, ob es die letzte Kopie war. Siehe [Wiederherstellung](restore.md).

## Speicherorte {#destinations}

Ein Speicherort ist ein Ort, an den Sicherungen gehen: ein lokaler Ordner, ein Share, ein eigener Server oder ein Cloud-Dienst. Er wird einmal eingerichtet und bedient dann jeden Bereich. Öffne **Speicherorte** über seine Einstellungskachel oder schalte dort **Auch in der Seitenleiste zeigen** ein, um ihn anzuheften.

![Die Seite Speicherorte: die Liste der Orte und der Ablauf, welcher Bereich wohin geht.](assets/screenshots/storage/locations-dark.png#only-dark)
![Die Seite Speicherorte: die Liste der Orte und der Ablauf, welcher Bereich wohin geht.](assets/screenshots/storage/locations-light.png#only-light)

**Speicherort hinzufügen** öffnet ein Fenster in vier Schritten:

1. **Anbieter.** Jeder Dienst wird mit seinem Logo aufgelistet, mit einer Suche oben rechts im Fenster, in Gruppen: **Lokal** für eine Platte oder einen Share, die an diesen Server angeschlossen sind, **NAS oder Server** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV und andere), **S3-Dienst** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 und andere), dein eigener S3-Server (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), **Cloud-Speicher** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud und der Rest, den rclone unterstützt) und **Aus deiner Gruppe**, die Empfangsserver deiner Gruppe. Jeder sagt, wie gut er sich für Sicherungen eignet: Cloud-Laufwerke werden bei vielen Anfragen langsamer, die erste Sicherung und das Aufräumen dauern dort also länger. Für einen Dienst, den keine Kachel abdeckt, nimmt die Kachel **rclone** unter **Eigene rclone-Konfiguration** Remotes, die aus einer rclone.conf eingefügt werden.
2. **Anmelden.** Die Felder hängen vom Dienst ab: ein Zugangsschlüssel für S3, ein Benutzer und Passwort für WebDAV und SMB, ein App-Passwort, wo die Zwei-Faktor-Anmeldung das normale blockiert, der öffentliche SSH-Schlüssel von BombVault für SFTP und die Storage Box, oder ein Token für Dienste, die sich über einen Browser anmelden. Dafür zeigt das Fenster einen Befehl `rclone authorize`, den du auf einem Rechner mit Browser ausführst, und das Token, das er ausgibt, kommt ins Feld. Hast du für den Dienst schon eine [gespeicherte Anmeldung](#sign-ins), wählt **Gespeicherte Zugangsdaten** sie statt **Neue Schlüssel**. **Verbindung testen** prüft die Anmeldung, bevor etwas gespeichert wird. Das Fenster listet dann die Ordner auf dem Speicherort auf, mit **Neuer Ordner**, um einen anzulegen, und dem freien Platz, wo der Dienst ihn meldet. Ein leerer Ordner ist am sichersten. BombVault legt darin einen Ordner pro Bereich an.
3. **Schutz.** Das Fenster sagt deutlich, was der Dienst kann. Ein rest-server im Append-only-Modus verweigert das Löschen, und der Manipulationstest prüft das. Ein S3-Bucket kann alte Versionen mit Versionierung und Object Lock behalten, was BombVault noch nicht prüfen kann. Ein Cloud-Laufwerk kann das Löschen gar nicht verweigern: Wer in den Server kommt, kommt auch an diese Kopie. Schalte **Löschschutz (append-only)** nur dort ein, wo die Gegenseite das Löschen wirklich verweigert. BombVault räumt dort dann nie auf. **Für den Notfall** sagt, was das Recovery-Kit für diesen Ort enthält.
4. **Für welche Bereiche.** Wähle die Bereiche, die hierher sichern. Die erste Kopie bringt den ganzen Verlauf mit. Einträge, die eigene Speicherorte gewählt haben, bekommen den neuen nicht von selbst, und das Fenster nennt sie.

![Speicherort hinzufügen: der erste Schritt, mit jedem Anbieter als Kachel.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Speicherort hinzufügen: der erste Schritt, mit jedem Anbieter als Kachel.](assets/screenshots/storage/add-providers-light.png#only-light)

![Speicherort hinzufügen: der Schritt Anmelden nach einem erfolgreichen Test.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Speicherort hinzufügen: der Schritt Anmelden nach einem erfolgreichen Test.](assets/screenshots/storage/add-signin-light.png#only-light)

Manche Dienste verlangen bei der Anmeldung etwas mehr:

- **SFTP und die Storage Box.** **Schlüssel hinterlegen** legt den öffentlichen Schlüssel von BombVault mit dem **Passwort des Kontos** auf dem Server ab, das für diese eine Anmeldung genutzt und nirgends gespeichert wird. Der Befehl, es von Hand zu tun, steht darunter. Der Test zeigt den **Serverschlüssel** mit seinem Fingerabdruck. BombVault merkt ihn sich und lehnt den Server ab, wenn er später einen anderen zeigt.
- **S3-Schlüssel für einen einzelnen Bucket**, etwa ein Cloudflare-R2-Objekt-Token oder ein eingeschränkter AWS-Schlüssel, listen die anderen Buckets unter Umständen nicht auf. Tippe den Bucket in **Bucket** ein, und der Test und die Ordnerliste öffnen diesen Bucket.
- **S3-Regionen und -Adressen.** Die Region wird aus einem Endpunkt gelesen, der sie nennt (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi), Garage beginnt mit seiner eigenen Standardregion, und IDrive e2 verlangt den Endpunkt, den sein Dashboard zeigt. Eine Adresse, die mit dem Bucket darauf eingefügt wird, so wie das R2-Dashboard sie zeigt, füllt Server und Bucket aus. Ein Bucket, der aus dem Ordner-Browser angelegt wird, landet in der Region, die das Formular nennt.
- **Speicherklasse.** Die Auswahl bietet nur die Klassen an, die der Dienst nimmt, und **Weitere Einstellungen** zeigt nur die Optionen, die einen neuen Bucket prägen.
- **Google Drive und OneDrive.** Aufgeräumte Daten werden endgültig gelöscht, statt in den Papierkorb zu gehen, wo sie weiter auf die Quota zählen würden. Ein geteiltes Google-Laufwerk kommt unter **Weitere Einstellungen**.
- **SMB-Server, die ihre Freigaben nicht auflisten,** nehmen unter **Freigabe** den Namen einer, und der Test öffnet diese Freigabe direkt.
- **FTP** fragt nach seiner **Verschlüsselung**: **Keine**, **Explizites TLS (FTPES)** oder **Implizites TLS (FTPS, Port 990)**.
- **OneDrive und Zoho WorkDrive.** Öffnet eine Anmeldung mehrere Laufwerke oder Arbeitsbereiche, fragt das Fenster, in welches die Sicherungen gehen.
- **Ein rest-server mit selbstsigniertem Zertifikat.** Lege das Zertifikat als PEM-Datei in den Ordner `certs` im Datenordner von BombVault (`/config/certs` im Container, `appdata/bombvault/certs` auf Unraid). Sicherungen, Kopien und der Löschschutztest vertrauen ihm ohne Neustart.

S3-Dienste laufen über das eigene S3-Backend von restic, wodurch eine Speicherklasse und Object Lock greifen können. Ein rest-server wird von restic direkt erreicht. Jeder andere Dienst läuft über das rclone, das BombVault mitliefert, und sein Abschnitt der rclone-Konfiguration zeigt sich in der Karte **rclone.conf** auf der Seite des Ortes, mit verborgenen Geheimnissen. Ein Einstellungsexport trägt die Speicherorte, und mit eingeschlossenen Zugangsdaten auch deren Anmeldungen.

Wird ein Speicherort gespeichert, der schon Sicherungen von Containern, VMs oder Ordnern hält, zum Beispiel einer, den du entfernt hast und neu hinzufügst, wird er diesen Bereichen als nicht angehakte Kopie hinzugefügt. Es wird nichts dorthin kopiert, bis du ihn abhakst, aber **Wiederherstellung** bietet die alten Sicherungen sofort an.

Der Ordner auf diesem Server heißt überall **Lokal**: auf den Speicherseiten, im Ablauf, an einem Eintrag und bei der Wiederherstellung. Bei mehreren Ordnern auf diesem Server geht jeder unter seinem eigenen Namen. Ein Speicherort, den das Off-site-Feld eines Bereichs einrichtet, wird nach seinem Ort benannt, dem letzten Ordner seiner Adresse oder, ohne einen, seinem Server, und bekommt eine Nummer, wenn ein anderer Ort schon so heißt. Jeder Ort lässt sich im Feld **Name** seiner Karte **Verbindung** umbenennen, auch ein Bucket-Ordner, den mehrere Bereiche teilen, und die Kopie, die das Off-site-Feld eines Bereichs beschreibt, und ein Name, den du vergibst, bleibt. Wo zwei Orte mit demselben Namen nebeneinander stehen, trägt jeder seinen Server oder Ordner.

### Die Seite eines Speicherorts {#location-page}

Jeder Speicherort hat eine eigene Seite. Sie zeigt nur die Karten, die für diesen Ort gelten, in dieser Reihenfolge:

![Die Seite eines Speicherorts: Eigene Ordner, Verbindung und Zugangsdaten.](assets/screenshots/storage/location-page-dark.png#only-dark)
![Die Seite eines Speicherorts: Eigene Ordner, Verbindung und Zugangsdaten.](assets/screenshots/storage/location-page-light.png#only-light)

- **Eigene Ordner**: die Bereiche, die ihre Sicherungen zuerst hierher schreiben. Anmeldung, Schutz und Limits dieses Ortes gelten für sie. **Als eigenen Ordner nutzen** verschiebt den eigenen Ordner weiterer Bereiche hierher, und **Ändern** verschiebt einen wieder weg. Siehe [Der eigene Ordner eines Bereichs](#own-folder).
- **Sichert direkt hierher**: jeder Bereich, dessen Einträge direkt an diesen Ort sichern statt in ihren eigenen Ordner, und wie viele Einträge es tun. Ein solches direktes Repository übernimmt alle seine Einstellungen von diesem Ort. Eines, auf das nichts mehr sichert, sagt das, und **Entfernen** nimmt es aus der Liste. Die Sicherungen an seinem Ort bleiben. Siehe [Direkte Repositories](#direct-repositories).
- **Verbindung**: der **Name**, der Schalter **Verfügbar** (aus heißt, BombVault schreibt hier nicht mehr, und Einträge, die hierher sichern, scheitern mit einer klaren Meldung, statt woanders zu landen), der Ordner, **Kompression**, **Speicherklasse**, **Gleichzeitige Verbindungen** und **Außer Haus**. Auf dem Ort **Lokal** listet sie unter **Eigene Ordner hier** den Ordner jedes Bereichs auf. **Verbindung ändern** testet und speichert eine neue Adresse oder Anmeldung. Melden sich andere Orte mit denselben gespeicherten Zugangsdaten an, sagt das Fenster es, und eine Änderung hier gibt diesem Ort eigene Zugangsdaten, statt die gemeinsamen umzuschreiben.
- **Zugangsdaten**: welche gespeicherten Zugangsdaten er nutzt (**Meldet sich an mit**), welche anderen Orte sie auch nutzen (**Auch verwendet von**) und **Bearbeiten**, was das Fenster der Zugangsdaten öffnet. Siehe [Zugangsdaten](#sign-ins).
- **rclone.conf**: der Abschnitt dieses Ortes in der rclone-Konfiguration, für einen Ort, der über rclone läuft.
- **Löschschutz**, mit **Löschschutz (append-only)**, **Ohne Löschschutz akzeptieren**, **Löschschutz jetzt testen** und **Wer aufräumt**, was sagt, ob BombVault die Kopie und die eigenen Ordner hier aufräumt, nach der Karte **Wie lange aufheben** dieses Ortes oder den eigenen Werten eines Bereichs unter **Regeln je Bereich**, oder das der Gegenseite überlässt. Ein lokaler Ordner zeigt diese Karte nur, wenn er einen Schutz hat, den du ausschalten kannst. Siehe [Löschschutz](#delete-protection-append-only).
- **Kopieren**: **Wann kopieren** sowie **Upload-Limit (KiB/s)** und **Download-Limit (KiB/s)**. Ein Ort, der nur eigene Ordner und keine Kopien hält, zeigt die Limits als Karte namens **Bandbreite**.
- **Wie lange aufheben**: **Kurz**, **Ausgewogen**, **Lang** oder **Eigene Werte**, mit einer Zeitleiste, welche Sicherungen die Regel behält. Siehe [Wie lange aufheben](#keep).
- **Schon unter diesem Speicherort**: Bereiche, deren von Hand eingetippte Adresse in einem Ordner dieses Ortes liegt. Sie zu übernehmen lässt das Repository, wo es ist. Anmeldung, Speicherklasse und Löschschutz kommen dann vom Ort. BombVault prüft zuerst, dass die Anmeldung des Ortes das Repository öffnet, und lehnt es ab, ein Append-only-Repository unter einen Ort zu stellen, der nicht append-only ist.
- **Regeln je Bereich**: das eigene Repository, die Aufbewahrung oder Kompression eines Bereichs an diesem Ort. **Regeln ändern** öffnet sie, und **Auf den Speicherort zurücksetzen** holt einen Bereich zurück. Anmeldung, Bandbreite und wann Kopien laufen kommen immer vom Ort.
- Ein Gefäß, das zeigt, wie voll der Ort ist, mit der Markierung, wo er voll ist, und wie viele Wochen bei dem Wachstum der letzten Wochen übrig sind. Ein Ort, der seine Größe nicht meldet, sagt **Größe unbekannt**.

**Umziehen** und **Speicherort entfernen** stehen unter den Aktionen der Seite, beschrieben unter [Umziehen und entfernen](#move-remove).

Ein entfernter Speicherort, der Sicherungen hielt und plötzlich kein Repository mehr hat, etwa ein rest-server, dessen Platte nicht gemountet wurde, bekommt nicht von selbst ein neues, leeres. Seine Seite sagt das, sobald eine Sicherung oder Kopie das Repository verschwunden findet. Mounte die Platte wieder. Hast du das alte Repository absichtlich geleert, beginnt **Hier ein neues Repository anlegen** von vorn, für jeden Bereich des Ortes und seine Repositories auf einmal. Dieser Schutz gilt für einen Speicherort, der an seinem Platz geblieben ist. Ein Speicherort, den du hinzufügst, oder ein Bereich, den du an einem Ort anhakst, fängt sauber an: Liegt an dieser Adresse kein Repository, legt die erste Kopie eines an, auch wo ein früher entfernter Speicherort eines hatte.

Zwei Ablehnungen nennen, was zu richten ist, im Lauf der Kopie und auf der Seite des Speicherorts. **Diesen Bucket gibt es auf dem Server nicht** kommt, sobald ein S3-Server das meldet, und BombVault legt dort nichts an, ein falsch getippter Bucketname bleibt also sichtbar. Ein Server, der gar nicht antwortet, ist ein anderer Fall: restic versucht es pro Aufruf etwa 15 Minuten lang weiter, falls er zurückkommt. **Hier liegt schon ein restic-Repository, und der Schlüssel dieser Instanz öffnet es nicht** heißt, dass die Sicherungen dort zu einem anderen `APP_KEY` gehören.

Ein Empfangsserver, den eine andere Instanz deiner Gruppe betreibt, erscheint unter den Anbietern als **Aus deiner Gruppe**. Siehe [Empfangsserver](#receiving-server).

BombVault prüft seine Speicherorte bei jedem Start. Es richtet, was keine Entscheidung von dir braucht, etwa einen Link auf eine Kopie oder ein Repository, das weg ist, eine Kopie auf dem eigenen Ordner eines Bereichs oder ein direktes Repository, dessen Kopie weg ist, und schreibt für jede Korrektur eine Zeile ins Protokoll. Was keine sichere Korrektur hat, wird nur protokolliert.

![Dieselbe Seite auf einem Handy.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![Dieselbe Seite auf einem Handy.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Wie lange aufheben {#keep}

Wie lange Sicherungen aufgehoben werden, wird pro Speicherort in seiner Karte **Wie lange aufheben** festgelegt. Die Regel gilt für die Kopien an diesem Ort und die eigenen Ordner, die dort liegen, den Ort **Lokal** eingeschlossen. Ein Bereich kann an einem Ort unter **Regeln je Bereich** eigene Werte behalten, und die gehen der Karte des Ortes für diesen Bereich vor.

Eine Regel behält eine Sicherung pro Tag, Woche, Monat und Jahr, so weit zurück, wie du einstellst, und räumt den Rest nach jeder Sicherung oder Kopie weg:

| Vorgabe | Letzte Sicherungen | Tage | Wochen | Monate | Jahre |
|---|---|---|---|---|---|
| **Kurz** | 0 | 7 | 4 | 3 | 0 |
| **Ausgewogen** | 0 | 7 | 4 | 6 | 2 |
| **Lang** | 0 | 14 | 8 | 12 | 3 |

**Eigene Werte** legt die fünf Zahlen selbst fest. Eine 0 schaltet diese Zahl ab, und eine Regel mit jeder Zahl auf 0 behält alles. Die Punkte in der Karte zeigen, welche von 40 Beispielsicherungen eine Regel behält, die neueste ganz rechts. Das Beispiel ist für jeden Ort dasselbe, da das Bild gezeichnet wird, bevor BombVault die Sicherungen dort gelesen hat. Es zählt von heute zurück, also ändert sich mit dem Datum, wie viele Punkte bleiben.

Ein Ordner auf diesem Host und ein von Hand eingetippter entfernter Ordner haben ebenfalls eine eigene Karte, die die eigenen Ordner in diesem Ordner abdeckt und keine anderen. Bis du sie speicherst, räumt BombVault so einen Ordner nach der Regel auf, die es schon für ihn hatte.

**Vor dem Verkürzen nachfragen**, standardmäßig an, fragt nach, bevor eine Änderung gespeichert wird, die weniger Sicherungen behält, und sagt, wie viele der 40 Beispielsicherungen die neue Regel entfernt. Es gilt für jeden Speicherort. **Auf den Speicherort zurücksetzen** unter **Regeln je Bereich** stellt dieselbe Frage.

Ein Ort mit Löschschutz wird von dieser Box aus nie aufgeräumt. Seine Gegenseite behält oder verwirft alte Sicherungen nach ihren eigenen Regeln. Solange ein Eintrag eine offene kritische Anomalie über verlorene Daten hat, behält die Aufbewahrung seine alten Sicherungen, bis du sie quittierst. Siehe [Anomalien](anomalies.md).

Eine Sicherung, die weniger enthält, als sie sollte, lässt die Sicherungen vor ihr nie verfallen. Das ist eine Sicherung, die keine Dateien liest, wo frühere welche hatten, einen gewählten Ordner verfehlt oder manche Dateien nicht lesen konnte. Ihr Lauf endet mit einer Warnung, die nennt, was fehlt, und sagt, dass die älteren Sicherungen behalten wurden. Dieselbe Nachricht kommt auf deinen Benachrichtigungskanälen, und kein Aufräumen, weder im eigenen Ordner des Eintrags noch bei seinen Kopien, rührt die alten Sicherungen dieses Eintrags an, bis eine vollständige Sicherung durchkommt. Eine erste Sicherung und ein gewählter Ordner, der immer leer war, zählen als vollständig.

Um zu sehen, was der nächste Lauf wirklich löschen würde, öffne **Vorschau** unter **Einstellungen, Speicherorte**. Sie listet **Was der nächste Lauf entfernen würde** für den eigenen Ordner des Bereichs und für jede seiner Kopien auf, pro Repository und pro Eintrag, mit den benannten Wiederherstellungspunkten. Sie nimmt keine Sperre und ändert nichts.

![Wie lange aufheben mit gewähltem Ausgewogen: Die Punkte zeigen, welche der 40 Beispielsicherungen bleiben.](assets/screenshots/storage/keep-dark.png#only-dark)
![Wie lange aufheben mit gewähltem Ausgewogen: Die Punkte zeigen, welche der 40 Beispielsicherungen bleiben.](assets/screenshots/storage/keep-light.png#only-light)

#### Beispiel: was Ausgewogen aufbewahrt {#example-retention}

Ein Container wird seit dem 1. Oktober 2024 jede Nacht um 03:00 gesichert, sodass er am Mittwoch, dem 4. November 2026, 765 Sicherungen hat. Sein Speicherort steht auf **Ausgewogen**: 7 Tage, 4 Wochen, 6 Monate und 2 Jahre.

Jede Zahl behält die neueste Sicherung ihres Tages, ihrer Woche, ihres Monats oder Jahres, und eine Sicherung, die mehrere Zahlen wählen, wird einmal behalten. Wochen laufen von Montag bis Sonntag:

| Regel | Sicherungen, die sie wählt | Neue, die sie hinzufügt |
|---|---|---|
| 7 Tage | 4. Nov, 3. Nov, 2. Nov, 1. Nov, 31. Okt, 30. Okt, 29. Okt | alle 7 |
| 4 Wochen | 4. Nov, 1. Nov, 25. Okt, 18. Okt | 25. Okt, 18. Okt |
| 6 Monate | 4. Nov, 31. Okt, 30. Sep, 31. Aug, 31. Jul, 30. Jun | 30. Sep, 31. Aug, 31. Jul, 30. Jun |
| 2 Jahre | 4. Nov, 31. Dez 2025 | 31. Dez 2025 |

Das lässt 14 Sicherungen von 765 übrig. Die älteste ist vom 31. Dezember 2025, du kannst also heute gut zehn Monate zurückgehen. Dank Deduplizierung brauchen sie kaum mehr Platz als eine volle Sicherung plus das, was sich dazwischen geändert hat.

**Jahre** verdient einen zweiten Blick. Es zählt Kalenderjahre, in denen es eine Sicherung gibt, beginnend mit dem laufenden, und die neueste Sicherung dieses Jahres ist die von heute. Das erste Jahr fügt also nichts zu den anderen Zahlen hinzu, und das zweite bringt die letzte Sicherung des Vorjahres dazu. Deshalb ändert sich die Reichweite einer Regel im Lauf des Jahres. Dieselben Sicherungen unter jeder Vorgabe:

| Vorgabe | Heute behaltene Sicherungen | Älteste Sicherung heute | Reichweite im Lauf des Jahres |
|---|---|---|---|
| **Kurz** | 10 | 30. September 2026 | 1 bis 2 Monate |
| **Ausgewogen** | 14 | 31. Dezember 2025 | etwa 4 Monate im Vorfrühling, ein volles Jahr am 31. Dezember |
| **Lang** | 30 | 31. Dezember 2024 | 1 Jahr am 1. Januar, 2 Jahre am 31. Dezember |

Der Wechsel von **Ausgewogen** zu **Kurz** fragt zuerst nach, wegen **Vor dem Verkürzen nachfragen**. Die Frage sagt, wie viele der 40 Sicherungen im Bild die neue Regel entfernt. Diese Zahl stammt aus dem Beispiel im Bild, sie ändert sich also mit dem Datum und zählt nicht deine eigenen Sicherungen. Von den Sicherungen dieses Containers würden der 31. August, 31. Juli, 30. Juni und 31. Dezember 2025 wegfallen. **Trotzdem speichern** wendet die Regel an, und das nächste Aufräumen löscht sie an diesem Ort endgültig.

Willst du die Container an diesem Ort länger aufbewahren als den Rest, drücke unter **Regeln je Bereich** **Regeln ändern**, wähle **Container** und setze in **Regeln für Container** **Eigene Werte**, zum Beispiel 14 Tage, 8 Wochen, 12 Monate und 2 Jahre. Die anderen Bereiche folgen weiter der Karte des Ortes, und **Auf den Speicherort zurücksetzen** holt Container zurück.

### Zugangsdaten {#sign-ins}

Zugangsdaten sind der Schlüssel oder das Passwort, die BombVault für einen S3-Dienst oder einen rest-server nutzt. Zugangsdaten sind eine Liste, die Karte **Zugangsdaten** unter **Einstellungen, Speicherorte**, und jede Zeile sagt, welche Speicherorte sie nutzen (**Verwendet von**).

- **Gespeichert, wenn du einen Ort hinzufügst.** Speicherort hinzufügen speichert die Schlüssel, die du eintippst, als Zugangsdaten. Der nächste Ort beim selben Dienst kann sie unter **Gespeicherte Zugangsdaten** statt **Neue Schlüssel** wählen, und eine spätere Änderung daran gilt für jeden Ort, der sie nutzt.
- **Die Vorgabe.** Ein Stern markiert die Standard-Zugangsdaten. Sie werden überall genutzt, wo keine gewählt ist, etwa bei einer von Hand eingetippten Adresse oder einer Wiederherstellung auf einer frischen Installation. Eine Pull-Quelle leiht sie sich nicht. **Als Standard** gibt den Stern an andere weiter.
- **Bearbeiten und Testen.** **Bearbeiten**, hier oder in der Karte **Zugangsdaten** eines Ortes, öffnet ein Fenster für die Zugangsdaten. **Testen** meldet sich an jedem Ort an, der sie nutzt, mit dem, was du eingetippt hast, bevor etwas gespeichert wird. **Speichern** speichert sie.
- **Entfernen.** Zugangsdaten lassen sich nur entfernen, wenn kein Speicherort, keine Pull-Quelle und kein empfangenes Repository sie nutzt und keine alten Sicherungen sie brauchen, und die Standardzugangsdaten gehen erst als letzte. Alte Sicherungen brauchen sie, wenn ein eigener Ordner von einem Ort weggezogen ist, der sich mit ihnen anmeldet, und sie dort gelassen hat. BombVault sagt dann, welcher Ort das ist.
- **Das Ändern der Verbindung eines Ortes** schreibt nie Zugangsdaten um, die andere Orte teilen. Der Ort bekommt stattdessen eigene, und das Fenster sagt vorher, dass seine Zugangsdaten geteilt sind. Um die geteilten überall zu ändern, bearbeite sie in der Karte **Zugangsdaten**.

![Die Karte Zugangsdaten: jede gespeicherte Anmeldung mit dem, was sie nutzt, und dem Stern an der Vorgabe.](assets/screenshots/storage/signins-dark.png#only-dark)
![Die Karte Zugangsdaten: jede gespeicherte Anmeldung mit dem, was sie nutzt, und dem Stern an der Vorgabe.](assets/screenshots/storage/signins-light.png#only-light)

Ein Ordner auf diesem Server braucht keine Zugangsdaten. SMB, WebDAV, SFTP, die Storage Box und die Cloud-Laufwerke halten ihre in der rclone-Konfiguration, die die Karte **rclone.conf** des Ortes zeigt, sie erscheinen also nicht in dieser Liste.

### SMB, WebDAV und andere rclone-Remotes {#smb-webdav}

Ein Windows- oder Samba-Share und ein WebDAV-Server (Nextcloud, ownCloud, SharePoint oder jeder andere) sind Anbieter-Kacheln in **Speicherort hinzufügen**, unter **NAS oder Server**. Trage Host und Share (SMB) oder Adresse und Servertyp (WebDAV), den Benutzer und das Passwort ein, und BombVault schreibt den rclone-Abschnitt für dich. rclone verschleiert das Passwort, bevor es gespeichert wird.

Für einen Dienst, den keine Kachel abdeckt, nimmt die Kachel **rclone** unter **Eigene rclone-Konfiguration** Remotes, die aus einer rclone.conf eingefügt werden. Sie werden zu den Remotes hinzugefügt, die BombVault schon hat, und ersetzen sie nie, und sie werden verschlüsselt gespeichert. Um einen Bereich auf eines davon zu sichern, trage `rclone:<remote>:<folder>` als Off-site-Repository dieses Bereichs ein, zum Beispiel `rclone:nas:backups/bombvault`. Der Share oder Bucket ist das erste Pfadsegment, nicht Teil des Namens.

Das ist der bessere Weg, als den Share auf Unraid zu mounten: restic rät davon ab, ein Repository auf einem gemounteten CIFS-Share zu halten, und hier wird nichts gemountet. NFS hat keine Kachel, weil weder restic noch rclone ein NFS-Backend haben. Mounte bei NFS den Export auf dem Host und nutze ihn als lokalen Ordner.

### Umziehen und entfernen {#move-remove}

- **Umziehen** nimmt die Kopie eines Bereichs mit seinem ganzen Verlauf an einen anderen Speicherort mit. Alles bleibt am alten Ort. BombVault löscht dort nichts. Der Umzug läuft im Hintergrund und kann lange dauern: etwa 9 Minuten pro 100 GB zwischen lokalen Platten, Stunden über eine langsame Leitung. Die Übersicht zeigt ihn, solange er läuft.
- **Speicherort entfernen** bietet zwei Wege. **Speicherort entfernen, Sicherungen behalten** hört auf, dort zu schreiben, und lässt die Sicherungen liegen. Du kannst sie später unter **Wiederherstellung, Aus einem alten Sicherungsordner** lesen. **Speicherort entfernen und Sicherungen dort löschen** liest zuerst, was dort gespeichert ist, und listet es auf: welche Sicherungen wegfallen, welche davon nirgends sonst liegen und welche Repositories bleiben, weil der Löschschutz es verweigert, Einträge noch direkt darauf sichern, ein anderer Ort oder Bereich dort seine Kopien hält oder sie sich nicht lesen ließen. Einträge, die ihre einzige Sicherung verlören, werden getrennt aufgelistet und bleiben erhalten, solange du nicht **Auch diese löschen** einschaltest. Du bestätigst, indem du den Namen des Ortes eintippst, und das Löschen läuft im Hintergrund.
- Das funktioniert für jede Art von Speicherort: einen, den mehrere Bereiche nutzen, ein Ziel, das für einen Bereich eingerichtet wurde, ein Repository, das du eingerichtet hast, und den Ordner, in den ein Bereich seine eigenen Sicherungen schreibt. Einträge, die auf ein entferntes Repository sichern, folgen wieder ihrem Bereich, ebenso eine Bereichsvorgabe, die dorthin zeigte. Der Ordner eines Bereichs kann nicht einfach gehen, weil jeder Bereich einen braucht. Das Fenster fragt nach dem Ordner, in den die Bereiche umziehen, und jeder Bereich behält darunter seinen Ordnernamen.
- Das Löschen der Sicherungen an einem Speicherort geht auch ohne Login-Passwort. Ist eines gesetzt, braucht es eine Anmeldung an der Instanz selbst.

### Beispiel: eine Off-site-Kopie bei Backblaze B2 {#example-s3}

Die Container und das Selbstbackup sollen auch bei Backblaze B2 liegen, damit ein toter Server sie nicht mitnimmt. Dieselben Schritte passen zu Wasabi, Hetzner Object Storage und den meisten anderen S3-Diensten.

1. **In B2** lege einen privaten Bucket an, etwa `tower-backups`, in einer Region in deiner Nähe. Lege unter **Application Keys** einen Schlüssel mit Lese- und Schreibzugriff an, beschränkt auf diesen Bucket, und notiere seine Key-ID und seinen Application Key. B2 zeigt den Application Key einmal. Stelle den Lifecycle des Buckets so ein, dass nur die letzte Version bleibt, sonst behält B2 aufgeräumte Daten als versteckte Versionen und berechnet sie dir.
2. **Anbieter.** Öffne in BombVault **Speicherorte**, drücke **Speicherort hinzufügen** und wähle **Backblaze B2** unter **S3-Dienst**.
3. **Anmelden.** Trage die Key-ID als **Access key ID** und den Application Key als **Secret access key** ein. Trage den Endpunkt des Buckets unter **Endpunkt** ein, etwa `s3.eu-central-003.backblazeb2.com`, und BombVault liest die Region daraus. Tippe `tower-backups` in **Bucket** ein, weil dieser Schlüssel nur einen Bucket öffnet.
4. **Verbindung testen.** Es sollte **Verbindung steht** sagen und die Ordner des Buckets auflisten. Drücke **Neuer Ordner** und nenne ihn `bombvault`. BombVault legt darin einen Ordner pro Bereich an, etwa `bombvault/containers`.
5. **Schutz.** Das Fenster erklärt, dass B2 alte Versionen mit Object Lock behalten kann und BombVault das nicht prüfen kann. Lass **Löschschutz (append-only)** aus, es sei denn, der Bucket verweigert Löschen wirklich, damit BombVault nach den Aufbewahrungsregeln des Ortes aufräumen kann.
6. **Für welche Bereiche.** Hake **Container** und **Selbstbackup** ab. Das Selbstbackup trägt deine Einstellungen, damit eine Neuinstallation sie zurückholen kann. Das Fenster sagt, wie viele Einträge und, wo bekannt, wie viel Verlauf die erste Kopie mitbringt.
7. Drücke **Speicherort speichern**.

Die erste Kopie läuft nach der nächsten Sicherung. Um sie jetzt zu starten, öffne **Einstellungen, Speicherorte** und drücke **Jetzt replizieren** in der Zeile des Bereichs unter **Off-site-Kopien**. Der Statuskopf sagt **Off-site-Kopie läuft**. Die erste Kopie bringt den ganzen Verlauf mit, 200 GB über einen 100-Mbit/s-Uplink dauern also etwa viereinhalb Stunden. Spätere Kopien senden nur das Neue.

Um deine Leitung freizuhalten, öffne die Seite des Ortes und setze **Upload-Limit (KiB/s)** in der Karte **Kopieren**, zum Beispiel auf `5120`, was sich als etwa 5,0 MB/s (41,9 Mbit/s) liest. Wirf dabei einen Blick auf **Wie lange aufheben** auf derselben Seite.

Um zu sehen, dass es geklappt hat, sieh dir den **Schutzstatus** in der Übersicht an, wo **Off-site** auf **Kopie aktuell** wechselt, und den Tab **Sicherungen** eines Containers, wo seine Snapshots jetzt `tower-backups` als zweiten Ort nennen. Drücke dann unter **Einstellungen, Wiederherstellung** noch einmal **Recovery-Kit herunterladen**, weil das Kit den neuen Ort und seine Anmeldung aufführt.

**Cloudflare R2 funktioniert genauso,** mit einem Objekt-Token statt eines Schlüssels. Füge die S3-Adresse ein, die das R2-Dashboard zeigt, mit dem Bucket darauf, und BombVault füllt Server und Bucket aus.

### Beispiel: ein rest-server, den du selbst betreibst {#example-rest-server}

Ein rest-server im Append-only-Modus verweigert das Löschen, eine Kopie darauf übersteht also auch, wenn jemand deinen Server übernimmt. Eine zweite Box bei einem Freund oder in der Garage genügt. Ein gekoppeltes BombVault kann das für dich einrichten, siehe [Empfangsserver](#receiving-server). Um ihn auf einem beliebigen Docker-Host von Hand zu betreiben:

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

Die letzte Zeile fragt nach einem Passwort und schreibt den Benutzer in die Datei `.htpasswd` im Datenordner.

1. Drücke in BombVault **Speicherort hinzufügen** und wähle **rest-server** unter **NAS oder Server**.
2. Trage die Adresse mit dem Benutzer als Ordner ein, etwa `http://192.168.2.20:8000/bombvault`, und den **Benutzername** und das **Passwort**. Drücke **Verbindung testen**.
3. Schalte unter **Schutz** **Löschschutz (append-only)** ein. BombVault räumt dort dann nie auf und überlässt das der Gegenseite.
4. Hake die Bereiche ab und drücke **Speicherort speichern**.
5. Ist die erste Kopie da, öffne die Seite des Ortes und drücke **Löschschutz jetzt testen**. BombVault versucht, ein Testobjekt zu löschen. Die Karte sollte **Zuletzt: Löschen verweigert** sagen, und **Löschschutz** in der Übersicht wechselt auf **geprüft**. Sagt sie **Zuletzt: Löschen nicht verweigert**, fehlt `--append-only`.

Da nichts diese Kopie von hier aus aufräumt, setze **Wachstumsbudget (GB, 0 = aus)** im Fenster des Bereichs, geöffnet mit **Bearbeiten** in seiner Zeile unter **Off-site-Kopien**, damit du es erfährst, bevor die Platte vollläuft. Bei einem selbstsignierten Zertifikat siehe den Hinweis zu `certs` oben.

## Der eigene Ordner eines Bereichs {#own-folder}

Jeder Bereich hat einen eigenen Ordner: das Repository, in das seine Sicherungen zuerst geschrieben werden. Kopien an andere Speicherorte gehen von dort aus. Eine Neuinstallation hält die eigenen Ordner auf diesem Server, unterhalb von `/mnt/user/bombvault`, und zusammen bilden sie den Speicherort **Lokal**.

Ein eigener Ordner kann auch in einem Speicherort liegen, den du schon hast, etwa einem NAS, einem rest-server oder einem S3-Bucket. Der Bereich sichert dann direkt dorthin, ohne lokale Kopie dazwischen, und der eigene Ordner folgt der Anmeldung, dem Löschschutz und den Bandbreitenlimits dieses Ortes. BombVault räumt ihn nach jeder Sicherung nach der Karte **Wie lange aufheben** des Ortes auf, und die eigenen Werte eines Bereichs unter **Regeln je Bereich** gehen der Karte vor. Der Ort listet den Bereich in seiner Karte **Eigene Ordner** auf, und **Speicherorte je Bereich** zeigt den Ort als Zuhause des Bereichs.

### Einen eigenen Ordner umziehen {#move-own-folder}

1. Öffne den Speicherort, der den eigenen Ordner halten soll, und wähle **Als eigenen Ordner nutzen** oder **Bereiche hinzufügen** in seiner Karte **Eigene Ordner**. Um mehrere Bereiche auf einmal umzuziehen, wähle **Eigene Ordner verschieben** neben **Eigene Ordner hier** in der Karte **Verbindung** des Ortes Lokal.
2. Wähle die Bereiche unter **Bereiche** und den Ort unter **Verschieben nach**. **Ein anderer Ort** nimmt einen Ordner auf diesem Host oder die von Hand eingetippte Adresse eines entfernten Repositorys, mit Zugangsdaten aus deinen gespeicherten.
3. Lies **Was passiert**. Es sagt für jeden Bereich, wohin neue Sicherungen gehen, ob eine Kopie, die schon am neuen Ort liegt, zum eigenen Ordner wird, und welche Sicherungen zuerst herüberkopiert werden.
4. **Bisherige Sicherungen mitnehmen** ist standardmäßig an. Ausgeschaltet bleiben die alten Sicherungen, wo sie sind, aber der Eintrag listet sie nicht mehr. **Aus einem alten Sicherungsordner** auf der Seite Wiederherstellung liest sie weiterhin, mit der Adresse des Ordners und bei einem entfernten Ordner mit seinem Login.
5. Wähle **Umstellen**. Hebt die Karte **Wie lange aufheben** am neuen Ort weniger auf als die heutige Regel des Bereichs, fragt BombVault zuerst **Weniger aufheben?** Dort nimmt **Bisherige Werte für** diesen Bereich **unter Regeln je Bereich behalten** die heutigen Werte mit, **Trotzdem speichern** macht weiter, und **Zurück** lässt alles, wie es war. Müssen Sicherungen zuerst kopiert werden, kopiert BombVault sie und stellt dann den eigenen Ordner um. Sind sie schon da, prüft es sie vor dem Umstellen.

Nach dem Umzug gehen neue Sicherungen in den umgezogenen eigenen Ordner, auch bei einem Bereich, bei dem **Lokal** ausgeschaltet war und der direkt auf eine Kopie sicherte. Eine Kopie, die schon am neuen Ort lag, wird zum eigenen Ordner, und das Kopieren dorthin endet. Hat diese Kopie manche Sicherungen länger behalten als die Aufbewahrungsregeln des Bereichs, behält der Bereich, was eine der beiden Regeln behält, sodass keine davon verloren geht.

Kann der Umzug nicht weitergehen, sagt das Fenster warum, in einem Kasten unter **Was passiert**. Ist nur das Kopieren der alten Sicherungen im Weg, lässt das Ausschalten von **Bisherige Sicherungen mitnehmen** den eigenen Ordner ohne sie umziehen.

Ein Bereich braucht immer einen eigenen Ordner. Den Speicherort zu entfernen, der ihn hält, fragt nach dem Ordner, in den der Bereich umzieht, siehe [Umziehen und entfernen](#move-remove). Ein eigener Ordner, an dem neue Sicherungen vorbeigehen, weil der Bereich direkt auf eine Kopie sichert, ist dort mit **(aus)** markiert, wo ein Ort auflistet, wer ihn nutzt.

![Das Fenster, das einen eigenen Ordner umzieht, mit ausgefülltem Was passiert für einen Bereich.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![Das Fenster, das einen eigenen Ordner umzieht, mit ausgefülltem Was passiert für einen Bereich.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Beispiel: Container ziehen auf ein NAS um {#example-own-folder}

Der Cache-Pool läuft voll, und du hast einen Speicherort namens `nas`, einen SMB-Share auf einer zweiten Box. Der Bereich Container hat 61 Sicherungen in seinem eigenen Ordner auf diesem Server und kopiert zu Backblaze B2.

1. Öffne **Speicherorte**, dann `nas`, und drücke **Als eigenen Ordner nutzen**.
2. Hake **Container** unter **Bereiche** ab.
3. Lies **Was passiert**. Es sagt, dass neue Sicherungen von Container in den Ordner auf `nas` gehen, dass die 61 Sicherungen in Lokal zuerst dorthin kopiert werden, dass Kopien weiter zu Backblaze B2 gehen und dass Schutz, Bandbreitenlimits und der Wachstumsalarm von `nas` kommen.
4. Lass **Bisherige Sicherungen mitnehmen** an und drücke **Umstellen**.

BombVault kopiert die Sicherungen im Hintergrund und stellt dann den eigenen Ordner um. Die Seite `nas` listet Container unter **Eigene Ordner** auf, ein Eintrag, der seinem Bereich folgt, nennt `nas` als sein Zuhause, und die nächste Sicherung landet dort, wobei die Kopie nach B2 von `nas` aus geht. Ab jetzt räumt die Karte **Wie lange aufheben** von `nas` den Ordner auf. Der alte Ordner auf diesem Server bleibt, wo er ist, und BombVault löscht dort nichts.

### Eine entfernte Adresse von Hand eintippen {#remote-primary-repositories}

Die Karte **Verbindung** des Ortes Lokal nimmt für den eigenen Ordner eines Bereichs weiterhin eine entfernte Adresse: **Ändern** neben dem Bereich öffnet ein Fenster mit einem Schalter **Lokal / Entfernt**. **Entfernt** nimmt eine restic-Adresse wie `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` oder `rclone:remote:bucket/path`, und **Sicherheitseinstellungen für Remote-Primärrepo** fügen einen Verbindungstest, Upload- und Download-Limits, Append-only-Schutz mit demselben Manipulationstest wie bei einem Off-site-Ort und einen Wachstumsbudget-Alarm hinzu.

Eine solche Adresse meldet sich mit den Standard-Zugangsdaten an. Ist sie genau der Ordner, den ein Speicherort, den du hast, für den Bereich führt, folgt sie diesem Ort von selbst, samt seiner Karte und seinem Schalter **Verfügbar**, es sei denn, sie meldet sich anders an oder ist append-only, wo der Ort es nicht ist. Eine Regel, die den Ordner bis dahin aufgeräumt hat, bleibt als eigene des Bereichs unter **Regeln je Bereich** erhalten. Liegt sie in einem anderen Ordner dieses Ortes, listet die Seite dieses Ortes den Bereich unter **Schon unter diesem Speicherort** auf, und ihn zu übernehmen gibt ihm die Anmeldung, Speicherklasse und den Löschschutz des Ortes. Den eigenen Ordner mit **Als eigenen Ordner nutzen** in einen Speicherort zu verschieben ist der einfachere Weg, weil das Fenster die schon gemachten Sicherungen prüft, bevor sich etwas ändert.

## Zuordnung pro Eintrag {#placement}

Die Karte **Zeitplan und Speicherort** auf dem Tab Einstellungen eines Eintrags hat eine Reihe von Knöpfen: **Lokal** und einen Knopf pro Off-site-Speicherort des Bereichs. Hell leuchtende Knöpfe bekommen die Sicherungen des Eintrags.

- Bei leuchtendem **Lokal** wird der Eintrag in das Repository geschrieben, das unter **Gespeichert auf** steht, und an jeden anderen leuchtenden Ort kopiert. Dunkle einen Ort, und er bekommt von diesem Eintrag nichts Neues. Nur Lokal kopiert nirgendwohin, was zu Daten passt, die schon eine zweite Kopie haben, zum Beispiel ein Share, der auf einem NAS liegt.
- Bei dunklem **Lokal** wird der Eintrag direkt in das direkte Repository des ersten leuchtenden Ortes geschrieben und von dort an die anderen leuchtenden kopiert. Beim ersten Mal legt ein Dialog dieses direkte Repository an.
- Ein Knopf bleibt hell, weil eine Sicherung einen Ort braucht, an den sie geht. Um einen Eintrag aus den Sicherungen auszulassen, nutze **Nicht sichern**.

BombVault verschiebt nie Sicherungen zwischen Repositories. Ein Eintrag, der seinem Bereich folgt, geht mit, wenn sich die Vorgabe des Bereichs ändert: Seine nächste Sicherung landet am neuen Ort, und die Sicherungen, die er hat, bleiben, wo sie sind. Ein am Eintrag selbst gewählter Ort steht ab seiner ersten Sicherung fest. Die Karte **Repository** auf dem Tab Einstellungen nennt ihn. Die Kopien können sich jederzeit ändern. Ein Ort, der einen Eintrag nicht mehr bekommt, behält die Kopien, die er hat, und kürzt sie beim nächsten Off-site-Lauf des Bereichs auf seine eigene Aufbewahrung. Aus Append-only-Orten lässt sich nicht löschen.

Der Ort, den ein Eintrag verlassen hat, behält seine Sicherungen. Auf dem Tab **Sicherungen** des Eintrags und bei der **Wiederherstellung** steht er als **Nur frühere Sicherungen**, eine Wiederherstellung kann weiter davon lesen, und ein Aufräumen dort lässt diese Sicherungen in Ruhe.

**Vorgabe übernehmen** bringt einen Eintrag mit eigener Wahl zu seinem Bereich zurück. Es fragt zuerst nach, sagt, wohin neue Sicherungen dann gehen, und nennt, was jeder Ort nicht mehr bekommt und was der nächste Lauf hochladen würde.

Unter der Reihe sagt die Karte, wohin der Eintrag geht und was tatsächlich da ist: wie viele Standorte ihn halten, wann jeder Ort zuletzt gesehen wurde und ob 3-2-1 erfüllt ist. Ein Standort ist der Server mit den Originaldaten, jeder Off-site-Ort und jedes Repository, das mit **Außer Haus** markiert ist. BombVault prüft Kopien und Standorte. Den Teil „zwei Medien“ von 3-2-1 prüft es nicht.

### Vorgaben pro Bereich {#placement-defaults}

Die Karte **Speicherorte je Bereich** auf der Seite Speicherorte zeichnet die Vorgabe jedes Bereichs als 3-2-1-Ablauf. Jede Linie von einem Bereich zu einem Speicherort ist ein Schalter, **Lokal** eingeschlossen, für jeden Bereich, auch den Flash, ZFS-Datasets und das Selbstbackup. Ein Klick auf eine Linie oder einen Punkt schaltet sie sofort um, und gestrichelte Linien sind aus. Nur das Ausschalten von **Lokal** eines Bereichs fragt vorher nach, im selben Dialog wie die Vorgabekarte, weil der Bereich dann direkt auf eine Kopie sichert. **Abbrechen** lässt alles, wie es war. **Grafik / Schalter** zeigt dasselbe als Reihen von Knöpfen. Ein Schalter entscheidet, wohin neue Sicherungen gehen. Einträge ohne eigene Wahl und die Projektordner von Compose-Stacks folgen sofort, ihre nächste Sicherung geht also an die neuen Orte. Schon gespeicherte Sicherungen bleiben, wo sie sind, und lassen sich von dort weiter wiederherstellen. Flash, ZFS-Datasets und das Selbstbackup sichern in ihren lokalen Ordner und kopieren von dieser Sicherung aus nach Off-site. Auch ihre Linie zu **Lokal** lässt sich ausschalten. Ihre Sicherungen gehen dann direkt an ihre Off-site-Orte, und ein ZFS-Eintrag mit einer Replik und gar keinem Ort sendet nur seine Replik.

Bei ausgeschaltetem **Lokal** und nur eingeschalteten Off-site-Orten sichert ein Bereich direkt dorthin, und BombVault wählt das Repository dort selbst. Container, VMs und Ordner-Sets gehen in das direkte Repository des ersten Ortes, der an ist, unten beschrieben, und werden von dort an die anderen kopiert. Der Flash, ZFS-Datasets und das Selbstbackup schreiben jede Sicherung an jeden Ort, der an ist, und jeder Ort behält sie nach seiner eigenen Aufbewahrung.

Ein Bereich, bei dem jede Linie aus ist, sagt **Nicht gesichert**. Seine Einträge bekommen keine Sicherungen und zählen als nicht geschützt.

Einträge, die auf ihrer Seite eigene Speicherorte gewählt haben, behalten sie. Der Ablauf sagt, wie viele es in einem Bereich sind, zum Beispiel **2 Einträge weichen ab**, und führt zu ihnen: zum Tab Einstellungen des Eintrags, wenn es einer ist, und zu einer Liste, wohin jeder sichert, wenn es mehrere sind. **Vorgabe übernehmen** dort bringt einen Eintrag zurück.

Ein neuer Off-site-Ort bekommt jeden Eintrag, der nicht auf Lokal gesetzt ist. Das Fenster, das ihn hinzufügt, sagt, wie viele Einträge und, wo bekannt, wie viel Verlauf das ist.

### Direkte Repositories {#direct-repositories}

Ein direktes Repository ist dort, wo Einträge direkt an einen Off-site-Ort sichern. BombVault legt es neben den Ordner des Ortes, mit `-direct` am Namen, oder, wenn der Ort an der Wurzel seines Buckets, seines Remotes oder eines rest-server-Benutzers sitzt, in einen Ordner darin, der nach dem Bereich benannt ist, etwa `containers-direct`. Im Ablauf auf der Seite Speicherorte wird es ohne Rückfrage angelegt. Lokal für einen einzelnen Eintrag auszuschalten, sodass ein Ort ohne direktes Repository sein Zuhause wird, öffnet einen Dialog mit dem vorgeschlagenen Ort, zum Beispiel `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, und einem Verbindungstest, der nichts anlegt. **Anlegen und nutzen** legt das Repository an und richtet den Eintrag darauf. Ein direktes Repository hat keine eigenen Einstellungen. Es folgt dem Schalter, der Anmeldung, der Speicherklasse, den Limits, der Append-only-Einstellung und der Aufbewahrung seines Ortes und ändert sich mit ihnen. Die Seite des Ortes listet es unter **Sichert direkt hierher** auf, mit jedem Bereich, der direkt darauf sichert, und wie viele Einträge es tun. Eines, auf das nichts mehr sichert, sagt das dort und lässt sich entfernen, und die Sicherungen an seinem Ort bleiben. Kann ein neuer Schlüssel für den Ort es nicht öffnen, behält das direkte Repository den Schlüssel, den es hat, und das Speichern sagt das. Ein Eintrag auf einem direkten Repository wird von dort an die anderen leuchtenden Orte kopiert, nie an den Ort, zu dem das Repository gehört. Seine Snapshots tragen den Tag `bv:direct`, und jeder andere Aufbewahrungsdurchgang behält sie, sodass ein direktes Repository, das den Link zu seinem Ort verloren hat, nie nach den lokalen Regeln altert. B2 wird über seinen S3-Endpunkt erreicht, mit der Key-ID und dem Application Key als S3-Schlüssel. Ein Schlüssel, der auf den eigenen Ordner des Ortes beschränkt ist, erreicht den Ordner daneben nicht, beschränke den Schlüssel also stattdessen auf den Ordner darüber.

### Außer Haus {#off-the-premises}

Ein Speicherort oder ein Repository kann mit **Außer Haus** markiert werden. Entfernte beginnen markiert. Schalte es für einen rest-server im selben Gebäude aus. Die Markierung zählt nur Standorte und 3-2-1 auf den Karten. Sie ändert keine Kopie.

Eine ZFS-Replik ist eine Kopie außer Haus, aber keine Sicherung mit Verlauf. Sie behält nur die Snapshots, die du einstellst, und alte Versionen und einzelne Dateien kommen weiter aus den Sicherungen. Sie geht von einem ZFS-Server direkt zum anderen, über SSH zu einem ZFS-Server oder zum Empfangs-Endpunkt einer gekoppelten Instanz. Die Koppelgruppe trägt nur die Anfrage. Siehe [Replik](zfs-datasets.md#replica).

### Nach einem Neuaufbau {#after-a-rebuild}

Kopierentscheidungen leben in den eigenen Einstellungen von BombVault. Nach einem Neuaufbau über **Sicherungen entdecken** oder den Notfall-Assistenten ohne wiederhergestellte Einstellungen sind sie weg, und alles zu kopieren würde die Einträge, die du ausgelassen hattest, wieder nach B2 senden. Dasselbe gilt, wenn ein Bereich, der mit dieser Datenbank nie off-site kopiert hat, an einem Speicherort eigene Sicherungen findet oder ältere in seinem eigenen Ordner, zum Beispiel nachdem ein Speicherort über den Sicherungen einer früheren Installation mit demselben `APP_KEY` hinzugefügt wurde. Die Off-site-Kopien dieses Bereichs warten dann auf deine Entscheidung.

**Braucht dich** in der Übersicht und die Seite jedes Speicherorts, zu dem der Bereich kopiert, fragen, ob die Sicherungen die eigenen dieses Servers sind. **Als eigene dieses Servers übernehmen** bestätigt die Vorgabe des Bereichs, und seine Kopien laufen ab dem nächsten Durchgang wieder. **Erst die Regeln prüfen** führt zur Zeile des Bereichs in **Speicherorte je Bereich**, wo du siehst, wohin jeder Eintrag geht, und eine verlorene Regel auf der Seite des Eintrags neu setzen kannst. Das Kopieren wartet dort weiter, bis du **Als eigene dieses Servers übernehmen** drückst, was zuerst zeigt, was der nächste Lauf kopiert, und die Namen in den Sicherungen, die keinen Eintrag haben und die du auslassen kannst. Die neueste Sicherung jedes wartenden Eintrags sagt das in ihrem Lauf, mit einem Link zur Entscheidung, und eine Benachrichtigung geht hinaus, wenn Benachrichtigungen eingeschaltet sind. Der Import einer Einstellungsdatei bringt Regeln und Vorgaben zurück, nimmt dir die Entscheidung aber nicht ab.

![Braucht dich nach einem Neuaufbau: Die Ordner-Sets haben Sicherungen von vor dieser Datenbank gefunden, und ihre Off-site-Kopien warten auf die Entscheidung.](assets/screenshots/offsite/copy-decision-dark.png#only-dark)
![Braucht dich nach einem Neuaufbau: Die Ordner-Sets haben Sicherungen von vor dieser Datenbank gefunden, und ihre Off-site-Kopien warten auf die Entscheidung.](assets/screenshots/offsite/copy-decision-light.png#only-light)

## Löschschutz (append-only) {#delete-protection-append-only}

Schalte den Löschschutz für einen Off-site-Speicherort ein, damit Ransomware oder ein kompromittierter Host deine Sicherungen nicht löschen oder umschreiben kann. Die Gegenseite (ein `restic/rest-server`, der im Modus `--append-only` läuft) **erzwingt** ihn. BombVault **prüft** ihn nur und zeigt nie Grün allein wegen einer Konfigurationsbehauptung. Ein Ordner auf diesem Server kann das Löschen nicht verweigern, weil, wer BombVault kontrolliert, auch den Ordner kontrolliert, lokale Orte bieten den Schalter also nicht an.

Der Schalter gehört zum Speicherort: Ein Bereich, der dorthin kopiert, übernimmt ihn vom Ort, schalte ihn also auf der Seite des Ortes unter **Speicherorte** ein. Das geplante **Löschschutz prüfen** läuft nur, solange ein Bereich an einen Ort mit eingeschaltetem Schalter kopiert. Die Gegenseite muss mit `--append-only` gestartet werden. Beim Image `restic/rest-server` gehört das, auf TrueNAS wie auf jedem anderen Docker-Host, in die Variable `OPTIONS`.

Manche Orte können ein Löschen gar nicht verweigern, ein Cloud-Laufwerk wie OneDrive zum Beispiel. Wenn du dort absichtlich Kopien hältst, schalte **Ohne Löschschutz akzeptieren** auf der Seite des Ortes ein. Die Übersicht zeigt dann den fehlenden Löschschutz eines Bereichs, der nur an akzeptierte Orte kopiert, als akzeptiertes Risiko, ein grauer Ring statt eines gelben Punkts, und die Farbe des Bereichs folgt seinen anderen Prüfungen. Sonst ändert sich nichts: Die Sicherungen dort lassen sich weiter löschen, BombVault räumt sie weiter nach den Aufbewahrungsregeln des Ortes auf, und der Schalter verschwindet, sobald der Löschschutz an ist. Eine Kopie, die an einen anderen Ort zieht, verliert die Akzeptanz. Auch das Einschalten des Löschschutzes löscht sie.

Der schnellste Weg zu einer Append-only-Kopie ist ein gekoppeltes BombVault mit einem [Empfangsserver](#receiving-server): Es betreibt den rest-server für dich, im Append-only-Modus.

In einem [gemeinsamen Repository](#shared-repository) gilt das für die Mitglieder, aber nicht für den Server, dem es gehört. Dieser Server erreicht den Ordner direkt, vergisst dort Sicherungen und räumt auf.

!!! note "Ein erfolgreiches Löschen unter `/locks/` ist zu erwarten"
    Append-only heißt nicht, dass nie etwas entfernt werden kann. restic muss seine eigenen Sperren nehmen und freigeben, daher bleibt `/locks/` absichtlich beschreibbar und löschbar. Snapshots und die Daten dahinter, worauf Ransomware es abgesehen hätte, lassen sich nicht entfernen. Wenn du die Gegenseite selbst prüfst, ist ein Löschen, das unter `/locks/` gelingt, korrektes Verhalten und keine Lücke im Schutz.

!!! warning "Append-only-Repos werden von dieser Box aus nie aufgeräumt"
    Ein Append-only-Ort räumt alte Snapshots von hier aus nie auf. Das Aufräumen ist Sache der Gegenseite. Setze dafür einen **Wachstumsbudget-Alarm**, damit du gewarnt wirst, bevor die Größe des Repos davonläuft.

## Manipulationstest {#tamper-test}

BombVault belegt die Append-only-Garantie regelmäßig, indem es tatsächlich versucht, auf dem Off-site-Repo etwas zu löschen, gezielt auf ein nicht vorhandenes Objekt. **Löschschutz jetzt testen** auf der Seite eines Ortes führt ihn auf Abruf aus, und **Löschschutz prüfen** unter **Einstellungen, Prüfung, Automatische Prüfungen** führt ihn nach Zeitplan aus.

Das Ausschalten von **Löschschutz prüfen** verbirgt keine Lücke. Ein Ort mit Löschschutz, der nie getestet wurde, liest sich dann „nicht bewiesen, seine Prüfung ist aus“, und ein Ort ohne Löschschutz liest sich in beiden Fällen „keiner“.

Eine Kopie, die ausgeschaltet ist, wird nicht getestet, und der Ort sagt das. Der Verbindungstest läuft darauf trotzdem, weil er nur liest, eine Kopie lässt sich also prüfen, bevor sie eingeschaltet wird.

- **Verweigert** heißt geschützt.
- **Akzeptiert** heißt nicht geschützt.
- Ein **nicht eindeutiges** Ergebnis (Server nicht erreichbar, Authentifizierungsfehler) kippt nie das gespeicherte Urteil.

Ein echter Wechsel von geschützt zu ungeschützt löst genau einen Alarm aus.

## DR-Tests {#dr-drills}

BombVault bietet zwei Stufen des Beweises, dass deine Sicherungen tatsächlich wiederherstellbar sind, nicht nur vorhanden.

- **Wiederherstellungsprüfungen (lokal).** BombVault führt regelmäßig `restic check --read-data-subset` aus (begrenzt, nie eine plattenfüllende vollständige Wiederherstellung). Der Takt steht unter **Einstellungen, Prüfung, Automatische Prüfungen**, und die Karte **Sicherungen prüfen** auf derselben Kachel zeigt das Ergebnis.
- **Probe-Wiederherstellung (Off-site).** BombVault stellt ein echtes Ziel aus dem Off-site-Repo in einer abgeschotteten Sandbox wieder her, prüft es Datei für Datei und Byte für Byte und räumt dann auf. Das belegt, dass du von Off-site wiederherstellen kannst, nicht nur, dass das Repo antwortet.

Die Karte **Schutzstatus** in der Übersicht fasst das pro Bereich in vier Prüfungen zusammen: eine aktuelle Sicherung auf diesem Server, eine Kopie off-site, Löschschutz dort und eine Probe-Wiederherstellung von dort. Ein Klick auf einen Bereich zeigt die Einzelheiten, und die Karte wird nur auf Grundlage geprüfter Tatsachen grün. Ein Ort, den du ohne Löschschutz akzeptiert hast, zählt dort als akzeptiertes Risiko, nicht als Warnung.

## Instanzen koppeln {#pairing}

Mehrere BombVaults arbeiten als eine Gruppe zusammen: Sie zeigen gegenseitig ihren Schutzstatus, übernehmen Rollen füreinander und können gegenseitig ihren Schlüssel verwahren. Eine Instanz tritt der Gruppe mit zwölf Wörtern bei.

![Die Kopplungsseite mit ihren drei Schrittkarten.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![Die Kopplungsseite mit ihren drei Schrittkarten.](assets/screenshots/offsite/pairing-light.png#only-light)

Öffne auf der ersten Instanz **Einstellungen, Kopplung** und drücke **Phrase generieren**. Drei Schrittkarten zeigen, wie es geht. Zwölf Wörter erscheinen in einem Fenster mit einem Knopf **Kopieren**. Öffne auf jeder weiteren Instanz dieselbe Stelle, drücke **Phrase eingeben** und füge sie ein oder tippe sie ein, oder drücke **Einfügen** in diesem Fenster. Ein Wort, das nicht auf der Liste steht, wird beim Tippen mit seiner Position genannt, und das letzte Wort trägt eine Prüfsumme, sodass ein falsch getipptes oder vertauschtes Wort erkannt wird, bevor etwas koppelt. Generiere die Phrase nur auf einer Instanz: Zwei Instanzen, die beide eine generieren, bilden zwei getrennte Gruppen. Taucht eine Minute lang niemand auf, sagt dir die Karte, dass du die Wörter drüben eingeben sollst, und **Phrase zeigen** holt sie dafür wieder hervor. Sie bietet auch an, stattdessen die Wörter der anderen Instanz einzugeben. Die Kopplung funktioniert ohne Login-Passwort, aber setze eines: Ohne es kann jeder, der die Weboberfläche öffnen kann, die Wörter lesen und über die Gruppe Rollen anfragen, die das restic-Passwort der Instanzen darin herausgeben. Die Kopplungskarte sagt das, bis ein Passwort gesetzt ist. Mit Passwort verlangt das erneute Zeigen der Phrase es. **Verlassen** nimmt eine Instanz zugleich aus der Gruppe und aus der Festung. Unter der Karte der Phrase stehen die Mitglieder als die Karten der Seite Instanzen, und solange noch niemand gekommen ist, sagt eine Zeile, dass die Instanz auf eine mit dieser Phrase wartet. Das Bild, wer für wen was tut, steht auf der Seite Instanzen.

Die Wörter allein lassen niemanden herein. Eine neue Instanz oder ein neues Handy steht in der Karte **Wartet auf Freigabe**, bis du auf einer Instanz, die schon in der Gruppe ist, **Freigeben** drückst, und deine Entscheidung erreicht jede erreichbare Instanz. Vergleiche den Schlüssel daneben mit dem, den die neue Instanz auf ihrer eigenen Kopplungsseite zeigt. **Ablehnen** weist sie endgültig ab. Koppeln sich zwei neue Instanzen, führt jede die andere als wartend, also gib sie auf beiden frei. Mitglieder, die schon vorher gekoppelt waren, bleiben freigegeben. Behandle die Wörter trotzdem wie ein Passwort: Wer sie hat, kann um Aufnahme bitten.

**Wie Mitglieder einander erreichen.** Jede Instanz erfährt ihre eigene Adresse im Netzwerk von deinem Browser in dem Moment, in dem du dich anmeldest. Im selben Netzwerk geben Mitglieder diese Adresse per Multicast bekannt und sprechen direkt miteinander, und wo Multicast ein Container-Netzwerk nicht überqueren kann, wie die Standard-Bridge von Docker, durchsucht eine Instanz stattdessen ihr eigenes Subnetz nach den anderen mit einem signierten Aufruf, den nur ein Gruppenmitglied beantworten kann, sodass die Kopplung auch ohne Relay in Sekunden fertig wird. Die Kopplung läuft immer über die zwölf Wörter, und ein Feld für eine Adresse gibt es nicht. Instanzen in verschiedenen Netzwerken gehen über ein Relay, in einer Reihe von vier in der Karte **Relay** auf derselben Seite gewählt. Jede Form kommt mit einem Bild:

- **Projekt-Relay** (die Vorgabe): `parleyport.halleluja.design`, das Relay, das auch KnightLoader nutzt. Nichts einzurichten.
- **Eigenes Relay**: der Container [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) aus den Unraid Community Apps (auch unter **Einstellungen, Apps**). Trage seine Adresse auf jeder Instanz ein, die es nutzen soll.
- **BombVault-Instanz**: eine deiner Instanzen, die schon von außen erreichbar ist, mit eingeschaltetem **Als Relay dienen**. Diese Instanz antwortet dann unter `/relay/connect` auf ihrer eigenen Adresse, hinter dem Reverse-Proxy und Zertifikat, die sie schon hat, und lässt nur deine Gruppe hinein. Trage ihre Adresse auf jeder Instanz ein, die es nutzen soll.
- **Kein Relay**: Mitglieder finden sich im selben Netzwerk automatisch und sonst nirgends.

**Was das Relay sieht.** Ein Knopf gleichen Namens neben den vier Formen zeigt das für die gewählte Form. Jeder Aufruf zwischen Mitgliedern wird mit AES-256-GCM unter einem aus den zwölf Wörtern abgeleiteten Schlüssel versiegelt, und dieser Schlüssel verlässt deine Instanzen nie. Das Relay erfährt einen Hash, der die Verbindungen gruppiert, für welche Instanz eine Nachricht bestimmt ist, wie groß sie ist und wann sie durchgeht. Ein direkter Aufruf im lokalen Netzwerk wird genauso versiegelt und zusätzlich signiert, sodass nichts vom selbstsignierten Zertifikat abhängt, das eine Instanz liefert. Aufrufe, die etwas herausgeben (Rollenantworten, ein restic-Passwort, Empfänger-Logins, Schlüsselhinterlegungen), werden außerdem mit einem Schlüssel des Mitglieds signiert, das sie sendet, beim ersten Sehen festgehalten. Siehe [Sicherheitsmodell](configuration.md#security-model).

**Was über die Gruppe reist.** Der Schutzstatus auf der Seite Instanzen, eine Anfrage, einen Bereich jetzt zu prüfen, Rollenanfragen und ihre Antworten, Speicherangebote und was ein Empfänger oder Abholer braucht, sobald seine Rolle erlaubt ist: die Repository-Orte der anderen Instanz und ihr restic-Passwort. Sicherungsdaten reisen nie. Sie gehen weiter direkt an die restic-Backends. Auch der APP_KEY nicht: Das restic-Passwort öffnet die Repositories dieser Instanz und nichts sonst, nicht ihre gespeicherten Geheimnisse, Sitzungen oder Wiederherstellungscodes.

**Einträge von vor der Kopplung.** Instanzen, die mit einem Fleet-Token hinzugefügt wurden, sowie empfangene Repositories und Pull-Quellen, die mit dem APP_KEY der anderen Instanz eingerichtet wurden, bleiben nach dem Update bestehen und sind mit **Neu koppeln** markiert. Sie funktionieren weiter: Beim ersten Start ersetzt BombVault jeden gespeicherten APP_KEY durch das daraus abgeleitete restic-Passwort. Koppele beide Instanzen, bearbeite dann den Eintrag und wähle seine Instanz. Bis dahin stehen solche Einträge in der Karte **Ohne Instanz** auf der Seite Instanzen.

Die eine Stelle, die einen APP_KEY noch von Hand nimmt, ist [Aus einem alten Sicherungsordner](#restore-from-another-bombvault-repo), für den Fall, dass die andere Instanz weg ist und in einer Gruppe nicht antworten kann.

## Die Festung {#fortress}

Eine Festung bringt BombVault mit den anderen Apps der Familie zusammen, etwa ShipLog, TowerTorch, FireSquire und TollRoll, unter einer Phrase aus zwölf Wörtern. Es ist die Phrase auf der Kopplungsseite: Die Wörter, die deine Instanzen und Handys koppeln, holen auch die anderen Apps herein. Gib die Wörter von BombVault in der anderen App ein, oder gib die Wörter, die eine andere App zeigt, mit **Phrase eingeben** auf einem BombVault ein, das noch in keiner Gruppe ist. Ändert sich die Phrase, kommen die Instanzen und Handys, mit denen BombVault gekoppelt ist, ohne neue Kopplung mit, und eine, die gerade weg war, folgt, sobald sie wieder da ist. Die alte Phrase gilt weiter, bis jedes Gerät gefolgt ist, höchstens 30 Tage lang. Ein Gerät, das danach noch fehlt, steht auf der Kopplungsseite und in der App, damit du dort die Phrase eingeben oder es entfernen kannst.

- **Instanzname**: der eine Name, unter dem diese Instanz bei ihren anderen Instanzen, auf ihren Handys und bei den anderen Apps erscheint. Bleibt er leer, ist es der Name des Unraid-Servers. ShipLog und TowerTorch finden BombVault darunter auf ihrem eigenen Server.
- **Die anderen Apps**: Ist **Andere Apps der Festung zeigen** eingeschaltet, stehen sie unter den Mitgliedern in denselben Karten, jede mit ihrem Logo, wenn die App eines veröffentlicht.
- **Erlaubnisse**: Alles, was eine App bei einer anderen tun darf, hat einen eigenen Schalter, etwa sehen, wann die letzte Sicherung geklappt hat, eine Container-Sicherung anfordern oder die Sicherungen durchsehen. Am Anfang sind alle eingeschaltet.
- **Phrase zeigen** zeigt die Wörter neben einem QR-Code, sobald du dein Passwort eingibst. **Ändern** gibt der Festung eine neue Phrase, die jede erreichbare App und Instanz bekommt. Die alte Phrase funktioniert noch 30 Tage, und die Kopplungsseite nennt jedes Gerät, das die Änderung verpasst hat. **Verlassen** nimmt BombVault aus der Festung und aus seiner Gruppe.

**Was die anderen Apps tun.** ShipLog fragt nach einer Sicherung eines Containers, bevor es ihn aktualisiert, und BombVault überlässt ShipLog das Update der Container, die es betreut, deshalb bleibt dort der Schalter **Nach erfolgreicher Sicherung aktualisieren** aus. TowerTorch fragt vor einem Wartungslauf nach einer Sicherung, und FireSquire prüft, wann der Flash zuletzt gesichert wurde. Eine Anfrage wartet, solange eine andere Sicherung läuft. TollRoll zeigt BombVault in seiner Kachel im Unraid-Dashboard. Die anderen Apps lesen den Zustand von BombVault unter `GET /api/fortress/v1/status` und `GET /api/fortress/v1/log`, und das können auch ein API-Token oder ein angemeldeter Browser.

**Die eigene Sicherung einer App.** Bevor BombVault den Container einer App sichert, die sich selbst sichern kann, holt es die eigene Sicherung dieser App und legt sie im Repository des Containers ab, aber nur, wenn dieses Repository verschlüsselt ist. Sie erscheint in der Sicherungsliste des Containers mit dem Namen der App und einem Download, altert mit den Sicherungen des Containers, geht mit ihnen in die Off-site-Kopien und wird mit ihnen gelöscht. Ist **Sicherungen an Apps zurückgeben** eingeschaltet, gibt **An … zurückgeben** sie der App wieder, und das ersetzt, was die App gerade hat. Dieser Schalter bleibt aus, bis du ihn einschaltest.

## Instanzen und Rollen {#roles}

Die Seite **Instanzen** (eine Einstellungskachel, die an die Seitenleiste geheftet werden kann) zeigt die Gruppe als Netz, wer für wen was tut, eine Karte für diese Instanz und eine Karte für jedes andere Mitglied, die Android-Handys und die ZFS-Server. **Kopplung** unten rechts führt zur Kopplungsseite. Welche Teile an sind, wird unter **Einstellungen, Instanzen, Mehrere Server** festgelegt: **Instanzen**, **Empfänger** und **Holen**.

![Die Seite Instanzen: die Gruppe als Netz und eine Karte pro Instanz mit ihren Rollenkacheln, eine davon Inaktiv.](assets/screenshots/offsite/instances-dark.png#only-dark)
![Die Seite Instanzen: die Gruppe als Netz und eine Karte pro Instanz mit ihren Rollenkacheln, eine davon Inaktiv.](assets/screenshots/offsite/instances-light.png#only-light)

Jede Karte zeigt den Schutzstatus des Mitglieds und seine Rollen als Kacheln. Eine Rolle ist etwas, das eine Instanz für eine andere tut:

- **Empfänger** hält die Off-site-Kopien dieses Servers in einem rest-server im Append-only-Modus. Dieser Server sendet nach jeder Sicherung und kann dort nichts löschen, auch wenn ihn jemand übernimmt.
- **Abholer** holt die Sicherungen dieses Servers nach seinem eigenen Zeitplan. Dieser Server muss dafür nicht wach sein. Die abholende Instanz braucht eingeschaltetes **Holen**, bevor sie die Anfrage erlauben kann.
- **ZFS-Server** nimmt die ZFS-Repliken dieses Servers auf. Jeder ZFS-Eintrag wählt ihn in seinen Einstellungen unter **Replik**.
- **Schlüsselhüter** verwahrt eine versiegelte Kopie des Recovery-Kits dieses Servers. Siehe [Schlüsselhüter](#key-keeper).

Das Einschalten einer Rolle sendet eine Anfrage, und nichts startet, bis jemand auf der anderen Instanz **Erlauben** drückt. Die Anfrage erscheint als Abzeichen auf der Karte dieser Instanz, und ihre **Details** listen sie mit **Erlauben** und **Ablehnen** auf. Eine erlaubte Rolle lässt sich dort jederzeit entziehen. Eine gestrichelte Linie im Netz ist eine Rolle, die noch wartet. Eine Instanz mit BombVault 9.x kann eine Anfrage nicht beantworten, eine Rolle ist dort also aktiv, sobald du sie einschaltest, und eine Anfrage einer 9.x-Instanz wartet hier auf **Erlauben**.

**Details** auf einer Karte öffnet den Rest: die empfangenen Repositories und Pull-Quellen dieses Mitglieds, Speicherangebote, Überwachung, **Entfernen** und **Schlüssel vergessen**.

Unter den Karten gehören die Karte **Gemeinsames Repository** auf einer Box mit Empfangsserver und die Karte **Gemeinsame Repositories** auf einem Mitglied zum [gemeinsamen Repository](#shared-repository).

Ein Mitglied kann einem anderen Mitglied über die Gruppe seinen eigenen Off-site-Speicher anbieten. Der andere Admin sieht das Angebot in den **Details** des Mitglieds und nimmt es an oder lehnt es ab. Annehmen legt gespeicherte Zugangsdaten und einen gewöhnlichen Speicherort an, nach denselben Prüfungen wie bei einem Ort, den du eintippst. Der Speicherort kommt mit eingeschaltetem Löschschutz, denn das Rezept, das das Angebot zeigt, ist ein append-only rest-server, darum gilt **Wie lange aufheben** dort nicht. Jedes Angebot hat ein eigenes Login und einen eigenen Ordner. Bietest du einen Bereich erneut an, demselben oder einem anderen Mitglied, bleiben die Kopien des ersten Partners also unberührt: Füge die neue Zeile zu `.htpasswd` hinzu und lass den Server laufen. Auf diesem Weg reisen nur Verbindungsdaten, nie Sicherungsdaten. **Jetzt prüfen** bittet ein Mitglied, das Repository eines Bereichs zu prüfen. Nichts auf der Seite kann auf einer anderen Box eine Sicherung, Wiederherstellung oder ein Löschen starten.

Instanzen, die heruntergefahren sind, und Handys mit geschlossener App behalten ihre Karte als **Inaktiv**, in Grau, mit dem Zeitpunkt, zu dem sie zuletzt gesehen wurden, und stehen gedimmt im Netz, bis du sie **Entfernen**. Direkt nach dem Start dieses Servers sagt so eine Karte **Wird geprüft**, bis die Gruppe wieder da ist. **Entfernen** funktioniert auf so einer Karte, egal welche Teile eingeschaltet sind, und beendet auch die offenen Anfragen des Mitglieds, seine ungenutzten ZFS-Empfangsslots und seine wartenden Schlüsselanfragen. Die Karte **Instanzen** auf der Kopplungsseite listet diese Mitglieder ebenfalls als **Inaktiv** auf.

## Empfänger und Abholer {#receiver}

Auf der Box, die Append-only-Kopien von einem anderen BombVault **empfängt**, geben die empfangenen Repositories dir unabhängige, schreibgeschützte Überwachung auf der empfangenden Hardware, damit ein stiller Fehler am anderen Ende nicht unbemerkt bleibt. Schalte **Empfänger** unter **Einstellungen, Instanzen** ein, öffne dann die **Details** der sendenden Instanz auf der Seite Instanzen und füge das empfangene Repository hinzu (schreibgeschützt, geöffnet mit dem restic-Passwort der sendenden Instanz, das sie über die [Koppelgruppe](#pairing) bekommt). Du bekommst:

- **Ein Inventar nach Quelle**, damit du genau siehst, welche Container, VMs und Ordner-Sets gelandet sind. Sicherungen, die die sendende Instanz vor ihrem [eigenen Host](#backup-host) gemacht hat, zählen zur selben Quelle.
- **Zuletzt empfangen** pro Quelle, damit du weißt, wie frisch jede ist.
- **Ein unabhängiges `restic check`** auf der empfangenden Hardware, damit die Integrität dort geprüft wird, wo die Daten tatsächlich liegen, nicht nur beim Sender.
- **Einen Totmannschalter:** einen Alarm, wenn eine Quelle innerhalb eines von dir gesetzten Fensters nicht mehr sendet. Er schaut einmal am Tag nach, um 09:15. Einen Alarm bekommst du auch, wenn das Repository gar nicht gelesen werden kann, egal was der Prüfplan sagt, und wenn ein ganzes Fenster nach dem Hinzufügen noch nichts angekommen ist. Ein Repository, das du vor der ersten Kopie des Senders hinzufügst, ist bis dahin in Ordnung.
- **Integritätsalarme:** einen Alarm, wenn eine Prüfung auf der empfangenden Seite fehlschlägt. Eine Prüfung, die das Repository nicht öffnen konnte, sagt nichts über die Integrität, sie hinterlässt also kein Urteil und versucht es am nächsten Tag zur selben Zeit erneut.

Ein Repository auf einem anderen Server braucht das Login dieses Servers: Speichere es als [Zugangsdaten](#sign-ins) und wähle es im Dialog unter **Zugangsdaten**.

Der Empfänger ist strikt schreibgeschützt. Er schreibt nie in das empfangene Repository und kann daher nie die Append-only-Garantie brechen, auf die sich der Sender verlässt.

Ein Abholer arbeitet andersherum: Bei eingeschaltetem **Holen** liest eine Instanz die Sicherungen eines anderen Mitglieds nach ihrem eigenen Zeitplan, in ein eigenes Repository pro Quelle, sodass sie nie als eigene Sicherungen dieser Box zählen. Seine Quellen stehen ebenfalls in den Details dieses Mitglieds: der Repository-Ort, welche Art Sicherung es hält und wie oft geholt wird. Das restic-Passwort kommt über die Gruppe, nie der APP_KEY, und die Gegenseite hat nichts weiter einzurichten und muss nicht laufen. Die Quelle wird nur gelesen, und jede Seite behält ihre eigene Anmeldung. Eine Abholung läuft an dem Tag und zu der Uhrzeit, die ihr Zeitplan nennt, und auch eine neue Quelle wartet auf diese Zeit. **Jetzt holen** wartet nicht. Eine Abholung, die ausgefallen ist, weil diese Box aus war, läuft, sobald sie wieder da ist. Eine fehlgeschlagene versucht es am nächsten Tag zur selben Zeit erneut, auch bei einem wöchentlichen Zeitplan, und eine fehlgeschlagene Abholung erscheint im Aktivitätsprotokoll und schickt einen Alarm, bis eine Abholung wieder klappt. Eine `rclone:`-Quelle wird abgelehnt, weil rclone sie mit den Remotes dieser Instanz erreichen würde.

Eine geholte Kopie wächst nur. BombVault fügt nur hinzu, räumt sie nicht auf und prüft sie nicht. Was die Quelle längst vergessen hat, bleibt hier also liegen, bis du den Ordner löschst, und die Seite Speicherorte zählt sie nicht mit. Sie liegt in `pulled/<source id>` im eigenen Ordner des Bereichs, und wenn dieser Ordner umzieht, bleibt die Kopie, wo sie ist, und spätere Abholungen machen dort weiter, statt alles neu zu kopieren. Löschst du sie, fängt die nächste Abholung in dem eigenen Ordner von vorn an, den der Bereich dann hat.

### Empfangsserver {#receiving-server}

Die empfangende Box kann auch den rest-server betreiben, auf den die anderen kopieren. Er wird eingerichtet, wenn du zum ersten Mal eine **Empfänger**-Anfrage erlaubst, die danach fragt: Das Fenster sagt, dass der Server einen Empfangsserver braucht, und fragt nach einem Ordner auf einem Share, mit **Neuer Ordner**, um einen anzulegen, und einem Port (8000, sofern ihn kein anderer Container nutzt). BombVault dann:

1. lehnt ab, wenn schon ein Container namens `rest-server` existiert oder ein anderer Container den Port hält,
2. zieht `restic/rest-server` und startet ihn über den Docker-Socket im Append-only-Modus mit privaten Repositories und einer Login-Datei im Ordner,
3. schreibt sein Unraid-Template auf den Flash, damit der Container im Docker-Tab bearbeitbar bleibt, oder bietet das Template als Download an, wenn der Flash außer Reichweite ist,
4. führt den Manipulationstest dagegen aus und zeigt, ob er das Löschen verweigert.

Die Instanzen deiner Gruppe finden den Server dann unter den Anbietern bei **Aus deiner Gruppe**, benannt nach der empfangenden Box, und die Rolle **Empfänger** bietet ihn als sichere Wahl an. Jede Instanz bekommt beim ersten Nutzen des Servers einen eigenen Login und schreibt dort nur in ihren eigenen Ordner. Bei eingeschaltetem **Empfänger** unter **Einstellungen, Instanzen** öffnet **Empfangsserver** auf der Karte dieser Instanz den Server wieder. Sein Fenster listet die Logins unter **Partner mit Login** auf, und **Login entziehen** nimmt einen weg. Was diese Instanz schon kopiert hat, bleibt im Ordner. Die Einrichtung legt außerdem einen **Login außerhalb der Gruppe** an, dessen Passwort das Fenster einmal zeigt.

Eine Instanz, die die empfangende Box nur über das Relay erreicht, kann den Server nicht nutzen, weil das Relay keine Sicherungen trägt. Füge zuerst die Adresse der empfangenden Box auf der Kopplungsseite hinzu. Läuft BombVault auf einer eigenen IP-Adresse (zum Beispiel auf br0), fülle **Adresse für Partner** aus, weil der Server auf der Adresse des Hosts lauscht.

Eine empfangende Box kann außerdem ein [gemeinsames Repository](#shared-repository) für die Gruppe bereitstellen.

## Gemeinsames Repository {#shared-repository}

Mehrere Server einer Gruppe können zusammen in ein restic-Repository kopieren. Daten, die sie gemeinsam haben, etwa dasselbe Ordner-Set oder dieselben Appdata, liegen dort nur einmal. Zusammen brauchen sie also weniger Platz als mit je einem eigenen Repository.

Ein Server der Gruppe ist der Besitzer des Repositorys. Nur er vergisst dort alte Sicherungen und räumt auf. Die anderen sind Mitglieder: Sie erreichen das Repository über einen zweiten rest-server im Append-only-Modus und können nur hinzufügen. Jeder Server schreibt unter einem [eigenen Host](#backup-host), so bleiben die Sicherungen getrennt, auch wenn zwei Server einen Eintrag mit demselben Namen haben.

Was du dafür brauchst:

- eine [Koppelgruppe](#pairing),
- **Sicherungen verschlüsseln**, eingeschaltet beim Besitzer und bei jedem Mitglied, weil jeder Server einen eigenen restic-Schlüssel bekommt,
- einen [Empfangsserver](#receiving-server) beim Besitzer, den die Mitglieder erreichen. Läuft BombVault auf einer eigenen IP-Adresse, trag zuerst im Fenster des Empfangsservers die **Adresse für Partner** ein.

Ein Server ist Besitzer von höchstens einem gemeinsamen Repository.

### Einrichten {#shared-set-up}

Sobald der Empfangsserver läuft, hat die Seite **Instanzen** seiner Box unter den Instanzen eine Karte **Gemeinsames Repository**. Drück **Gemeinsames Repository einrichten**, wähl einen **Ordner für das gemeinsame Repository** auf einem Share mit viel Platz, lass den **Port** auf 8001, sofern ihn kein anderer Container nutzt, und drück **Einrichten und starten**. BombVault dann:

1. lehnt ab, wenn schon ein Container namens `rest-server-shared` existiert oder ein anderer Container den Port hält,
2. legt das Repository in einem Ordner `shared` darin an, mit dem Passwort dieses Servers,
3. startet `restic/rest-server` als `rest-server-shared`, im Append-only-Modus mit einer Login-Datei im Ordner,
4. schreibt auf Unraid sein Template auf den Flash, damit der Container im Docker-Tab bearbeitbar bleibt,
5. führt den Manipulationstest dagegen aus. **Erneut prüfen** auf der Karte führt ihn noch einmal aus.

Die Karte listet die **Mitglieder** auf, jedes mit seinen Schlüsseln und seiner letzten Sicherung, dazu die Beitrittsanfragen und **Entfernen** für jedes Mitglied.

### Beitreten {#shared-join}

Öffne auf dem Server, der beitreten will, **Speicherort hinzufügen**. Unter **Aus deiner Gruppe** erscheint der Besitzer ein zweites Mal, als **Gemeinsames Repository auf** und sein Name. Wählst du ihn, geht eine Anfrage an den Besitzer. Dort steht sie auf der Karte **Gemeinsames Repository** mit **Erlauben** und **Ablehnen**.

Nach **Erlauben** gibt der Besitzer über die Gruppe einen Einmalschlüssel und einen Login für seinen zweiten rest-server heraus. Das Mitglied legt damit einen eigenen restic-Schlüssel an, danach entfernt der Besitzer den Einmalschlüssel. Ist ein Mitglied nicht innerhalb eines Tages fertig, verliert es Einmalschlüssel und Login wieder und kann erneut fragen. Konnte der Besitzer das Mitglied mit seiner Antwort nicht erreichen, bietet die Karte **Noch einmal sagen** an.

Der Besitzer weist einen Server ab, solange dieser noch nicht unter einem eigenen Host schreibt, also ein älteres Release laufen hat, und wenn schon ein anderes Mitglied unter demselben Host schreibt. Zwei Server teilen sich einen Host, wenn sie sich einen `APP_KEY` teilen.

Ist das Mitglied dabei, erscheint unter **Speicherorte** ein Speicherort namens **Shared with** mit dem Namen des Besitzers, alle Bereiche ausgeschaltet. Schalte auf der Seite des Ortes die Bereiche ein, die dorthin kopieren sollen. Der Ort hat Löschschutz und keine eigenen Aufbewahrungsregeln, weil dort nur der Besitzer vergisst. Dorthin gehen nur Sicherungen unter dem eigenen Host des Mitglieds, ältere Sicherungen unter dem Host `bombvault` bleiben also, wo sie sind.

Der Besitzer kann auch seine eigenen Sicherungen hineinkopieren: **Eigene Sicherungen auch hineinkopieren** auf seiner Karte legt den Ort auch bei ihm an. Für den Besitzer liegt diese Kopie auf den eigenen Platten, und nichts hindert ihn, dort zu löschen. Sie ist also keine Kopie außer Haus.

### Wer aufräumt, und wann {#shared-upkeep}

Nur der Besitzer vergisst und räumt auf. Seine Einstellungen stehen im Teil **Gemeinsames Repository** des Fensters **Empfangsserver**, das du über die eigene Karte des Besitzers auf der Seite Instanzen öffnest:

- **Aufräumfenster** legt fest, wann der Prune läuft, täglich oder wöchentlich. Solange du es nicht änderst, beginnt es sonntags um 03:00, und die **Fensterlänge in Stunden** ist 6.
- **Ungenutzter Platz, den ein Prune lassen darf, in %** ist 5, solange du nichts änderst. Ein höherer Wert packt weniger um, der Prune ist also schneller fertig.
- **Nach dem Prune zurückgelesene Daten, in %** ist 5, solange du nichts änderst. Nach jedem Prune prüft der Besitzer das Repository und liest diesen Anteil der Daten zurück.
- **Wie lange aufheben** folgt für jeden Bereich den Off-site-Regeln des Besitzers, und **Eigene Regel festlegen** gibt dem gemeinsamen Repository eine eigene. Die Sicherungen jedes Servers altern für sich.
- **Jetzt aufräumen** vergisst, räumt auf und prüft sofort, nachdem die Mitglieder Bescheid bekommen haben.

Außerhalb des Fensters vergisst der Besitzer einmal am Tag, und der Platz kommt mit dem nächsten Prune zurück. Vor einem Prune sagt er den Mitgliedern Bescheid, und sie kopieren nichts in das Repository, bis sich das Fenster schließt. Ein Mitglied, das noch kopiert, hat bis zu einer halben Stunde, um fertig zu werden, sonst rückt der Prune ins nächste Fenster. Eine Kopie, die ins Fenster fällt, wartet und erscheint im Aktivitätsprotokoll und auf der Karte des Mitglieds als zurückgestellt. Der nächste Durchlauf holt sie nach.

Wo der Besitzer unsicher ist, behält er mehr. Ein Eintrag, den ein Mitglied wegen einer [Anomalie](anomalies.md) festhält, behält alle seine Sicherungen. Ein Mitglied, das nicht sagen kann, was es festhält, behält dieses Mal alle seine Sicherungen. Sicherungen unter einem Host, unter dem kein Server der Gruppe schreibt, werden nie vergessen. Das Fenster zeigt auch den Platz: was das Repository belegt, was die Server getrennt bräuchten und was das spart. Die Zahlen kommen mit jedem Aufräumen.

### Was Mitglieder sehen {#shared-members}

Auf einem Mitglied hat die Seite Instanzen eine Karte **Gemeinsame Repositories**. Sie zeigt jedes gemeinsame Repository, in das der Server kopiert oder dem er beitreten wollte, und wie es um seine Anfrage steht: **Wartet auf ein Ja**, **Tritt bei**, **Dabei**, **Abgelehnt**, **Abgelaufen** oder **Kein Mitglied**. Für ein Repository, bei dem er dabei ist, zeigt die Karte **Wer aufräumt**, **Nächstes Aufräumen**, **Gemeinsam mit** für die anderen Mitglieder, **Letzte Kopie hierher** und wie viel Platz das Repository belegt und spart.

### Austreten {#shared-leave}

**Austreten** auf der Karte des Mitglieds nimmt den Server heraus: Der Besitzer entfernt seinen Schlüssel und Login, und sein Speicherort dort wird ausgeschaltet. **Entfernen** auf der Karte des Besitzers nimmt ein Mitglied von der anderen Seite heraus. In beiden Fällen bleiben die Sicherungen des Servers im Repository und altern nach den Regeln des Besitzers. Beim Besitzer stehen sie unter **Ausgetretene Server**. Dort vergisst **Sicherungen vergessen** alle, sobald du den Namen des Servers eintippst. Das nächste Aufräumfenster gibt den Platz frei.

### Die Sicherungen anderer Server {#shared-restore}

Jeder Server findet seine eigenen Sicherungen im gemeinsamen Repository auf dem Tab **Sicherungen** seiner Einträge, wie jede andere Kopie. Die Sicherungen der anderen Mitglieder liegen auf der Seite Wiederherstellung unter [Andere Server](restore.md#other-servers).

### Nach einem Notfall {#shared-emergency}

Das [Recovery-Kit](#recovery-kit) hat ein Kapitel **Shared repositories**. Es nennt jedes gemeinsame Repository mit seinem Besitzer, die Bereiche, die dorthin kopieren, die anderen Mitglieder mit ihren Hosts und einen Befehl `restic snapshots` mit dem Host dieses Servers. Das Kit des Besitzers nennt außerdem Port, Ordner und Login seines rest-servers.

Im Notfall-Assistenten erscheint das Repository unter den Orten als **Gemeinsames Repository bei** und der Name des Besitzers. Eine Neuinstallation, die den Schlüssel eines Mitglieds übernimmt, schreibt unter demselben Host, weil der Host aus dem Schlüssel kommt. Sie bekommt ihren Speicherort im gemeinsamen Repository zurück und listet dort nur ihre eigenen Sicherungen auf und stellt nur diese wieder her.

Eine Neuinstallation, die den Schlüssel des Besitzers übernimmt, wird an seiner Stelle Besitzer des Repositorys. Sie lässt die Sicherungen altern und räumt auf wie bisher und startet den rest-server wieder auf demselben Ordner. Kopple sie wieder mit der Gruppe, richte unter **Instanzen** ihren Empfangsserver ein und leg die Regeln im Fenster des Empfangsservers neu fest, weil sie nicht im Kit stehen. Sobald die Gruppe die Mitglieder erreicht, sagt ihnen der neue Besitzer selbst Bescheid. Jedes Mitglied prüft das zuerst: Es legt im Repository einen Wegwerfschlüssel an, den nur ein Server mit Zugriff auf den Ordner des Repositorys entfernen kann, und folgt dem neuen Besitzer, sobald dieser Schlüssel weg ist. Antwortet der neue Besitzer unter einer anderen Adresse, macht jedes Mitglied diese Prüfung dort mit dem eigenen Schlüssel und Login, vergewissert sich, dass es dasselbe Repository findet, und stellt erst dann seine Speicherorte auf die neue Adresse um. Bis dahin kopiert es weiter an die alte Adresse, und seine Karte sagt, warum. Schlüssel, die von solchen Prüfungen übrig sind, entfernt der Besitzer beim nächsten Aufräumen.

Läuft der alte Besitzer noch, sagt seine Karte, dass jetzt der andere Server das Repository hält. **Zurückholen** macht ihn wieder zum Besitzer, für den Fall, dass ein Kit versehentlich auf einem zweiten Server übernommen wurde.

### Grenzen {#shared-limits}

- **Jeder Schlüssel liest alles.** Alle Schlüssel eines restic-Repositorys öffnen denselben Hauptschlüssel, jedes Mitglied kann also die Sicherungen der anderen lesen. Container- und VM-Definitionen bleiben mit dem eigenen Schlüssel jedes Servers versiegelt, die Container und VMs eines anderen Servers kommen also nur als Dateien zurück. Ein Server, der einmal Zugang hatte, könnte den Hauptschlüssel gelesen haben. Entfernen sperrt ihn also nicht für immer aus, das schafft nur ein neues Repository.
- **Der Besitzer kann löschen.** Kein Mitglied kann dort etwas löschen, aber der Besitzer erreicht den Ordner direkt, vergisst die Sicherungen jedes Mitglieds und räumt sie auf. Ein Fehler oder eine Übernahme beim Besitzer trifft sie alle.
- **Ein Schaden trifft jeden Server.** Ein kaputtes Pack fehlt jedem Server, dessen Sicherungen es nutzen. Ein gemeinsames Repository ersetzt also keine zweite Kopie.
- **Nur auf einem Empfangsserver.** S3, SFTP und andere Orte können ein Mitglied nicht aufs Hinzufügen beschränken, darum bietet BombVault ein gemeinsames Repository nur auf einem Empfangsserver an.
- **Nur als Kopierziel.** Der eigene Ordner eines Bereichs kann nicht in einem gemeinsamen Repository liegen, weil jeder Prune die Sicherungen aller Server aufhalten würde.
- **Ältere Sicherungen bleiben, wo sie sind.** Nur Sicherungen unter dem eigenen Host jedes Servers kommen hinein, und aus anderen Repositories wird nichts herübergeholt.

## Beispiel: zwei Unraid-Boxen koppeln {#worked-example-two-unraid-boxes-end-to-end}

Das ist eine vollständige Einrichtung der obigen Teile, denn sie lassen sich leichter zusammensetzen, wenn man sie einmal zusammengesetzt gesehen hat.

Zwei Boxen: **TOWER** betreibt die Container und schiebt Sicherungen. **VAULT** empfängt sie und erzwingt Append-only. Setze deine eigenen Namen, Adressen und Share-Pfade ein.

**1. Die beiden Boxen koppeln.** Öffne auf TOWER **Einstellungen, Kopplung**, drücke **Phrase generieren** und gib die zwölf Wörter auf VAULT unter **Phrase eingeben** ein. Setze vorher auf beiden ein Login-Passwort.

**2. Auf TOWER VAULT bitten, Empfänger zu sein.** Schalte auf der Seite Instanzen **Empfänger** auf der Karte von VAULT ein, wähle die Bereiche und **den Empfangsserver von VAULT** als Ort und sende die Anfrage.

**3. Auf VAULT erlauben.** Die Seite Instanzen von VAULT zeigt die Anfrage auf der Karte von TOWER. Drücke **Erlauben**. Beim ersten Mal fragt VAULT nach einem Ordner für seinen Empfangsserver, etwa `user/appdata/rest-server`, startet den rest-server im Append-only-Modus und führt den Manipulationstest dagegen aus.

**4. Zurück auf TOWER.** Von da an kopiert TOWER nach jeder Sicherung zu VAULT, und der neue Speicherort erscheint auf der Seite Speicherorte von TOWER mit seinem Löschschutz.

Was der Manipulationstest antwortet:

| Ergebnis | Was passiert ist |
| --- | --- |
| **geschützt** | VAULT hat das Löschen verweigert. Das ist der einzige bestandene Zustand. |
| **nicht geschützt** | VAULT hat ein Löschen angenommen. `--append-only` fehlt oder wurde entfernt. |
| **ohne Ergebnis** | Keins von beiden. Meist ist die URL nicht die, die restic selbst nutzt, oder die Zugangsdaten haben sich geändert. Es wird nichts vermerkt und kein Alarm ausgelöst. |

**5. Beobachte, was ankommt, wenn du willst.** Schalte auf VAULT **Empfänger** unter **Einstellungen, Instanzen** ein, öffne die **Details** von TOWER und füge das empfangene Repository hinzu: Klicke einen der Orte an, die der Dialog auflistet. VAULT bekommt das restic-Passwort von TOWER über die Gruppe, und weil der Ort auf dem eigenen Empfangsserver von VAULT liegt, liest es den Ordner direkt. Niemand tippt einen Schlüssel oder ein Login.

**6. Mach es gegenseitig.** Wiederhole die Schritte in der anderen Richtung: VAULT bittet TOWER, sein Empfänger zu sein. Jede Box erzwingt dann Append-only für die andere, und keine kann die Sicherungen der anderen löschen.

Ein rest-server, den du von Hand betreibst, funktioniert auch: Füge ihn mit **Speicherort hinzufügen** als rest-server hinzu, mit Benutzer und Passwort aus seiner `.htpasswd`, und schalte den Löschschutz ein. Behalte `--append-only` in seinen Optionen. Ohne es ist die Gegenseite wieder ein gewöhnlicher Share.

!!! warning "Ein lokaler Pfad ist ein Pfad im Container"
    Wo BombVault nach einem Ordner auf diesem Server fragt, gib ihn relativ zum Host-Mount ein, etwa `user/appdata/rest-server/bombvault-containers`, nicht `/mnt/user/appdata/...`. BombVault läuft in einem Container, in dem das `/mnt` des Hosts woanders gemountet ist. Ein absoluter Host-Pfad existiert darin nicht. Fügst du einen ein, sagt dir BombVault, welchen relativen Pfad du stattdessen nehmen sollst.

## Notfall-Assistent {#emergency}

Der **Notfall-Assistent** holt einen Server zurück, der weg ist oder neu aufgesetzt wurde. Er startet auf der Seite **Wiederherstellung** unter **Notfall** und über **Aus einer Sicherung wiederherstellen** auf der Willkommensseite einer Neuinstallation. Er läuft in vier Schritten: **Zugang**, **Einstellungen**, **Auswahl** und **Wiederherstellung**.

1. **Zugang.** Zeige BombVault den Schlüssel der alten Sicherungen, auf einem von drei Wegen:
    - **Recovery-Kit hochladen**: die Datei, die du bei der Einrichtung heruntergeladen hast. Sie enthält den Schlüssel und jeden Speicherort, und BombVault füllt den Rest daraus aus. Ein mit age versiegeltes Kit fragt nach der Identität, für die es versiegelt wurde. Sie wird einmal genutzt und nicht gespeichert.
    - **Über die Gruppe**: Eine Instanz deiner Gruppe hält den Schlüssel als Hinterlegung. Du brauchst die zwölf Wörter der Gruppe, deinen Wiederherstellungscode und jemanden bei dieser Instanz. Siehe [Schlüsselhüter](#key-keeper).
    - **Schlüssel eintippen**: der `APP_KEY` des alten Servers und wo seine Sicherungen liegen.

    BombVault probiert den Schlüssel dann an jedem Speicherort aus und sagt für jeden, ob er lesbar ist, nicht gefunden wurde, mit einem anderen Schlüssel gesperrt ist, nicht erreichbar ist oder ihm noch die Anmeldung fehlt. Noch ist nichts gespeichert, und der Schlüssel bleibt für begrenzte Zeit im Arbeitsspeicher. **Schlüssel übernehmen** lässt diesen Server ab jetzt seine Sicherungen mit diesem Schlüssel schreiben. **Übernahme rückgängig machen** geht zu seinem eigenen Schlüssel zurück, bis die erste Sicherung gelaufen ist. Ein Server, der schon eigene Sicherungen hat, lehnt die Übernahme ab und verweist stattdessen auf [Aus einem alten Sicherungsordner](#restore-from-another-bombvault-repo). **Zugangsdaten speichern** speichert die Zugangsdaten, die das Kit für entfernte Speicherorte trägt, und fragt nach, bevor es Zugangsdaten ersetzt, die dieser Server schon hat.
2. **Einstellungen.** **Einstellungen zurückholen** stellt die eigenen Einstellungen von BombVault aus dem neuesten Selbstbackup wieder her: Speicherorte, Zugangsdaten, Zeitpläne und Regeln. BombVault startet sich dafür über den Docker-Socket neu, damit die laufende Einstellungsdatenbank nie unter einem offenen Handle überschrieben wird, und der Assistent macht danach von selbst weiter. Der Login dieses Servers bleibt: Du meldest dich weiter mit dem Passwort an, das du jetzt nutzt. Geheimnisse, die der alte Server gespeichert hatte und die sich nicht übernehmen lassen, werden aufgelistet, damit du sie neu eingeben kannst. Die Wiederherstellung beendet jede Sitzung und entfernt die Passkeys, richte sie danach also neu ein.
3. **Auswahl.** BombVault durchsucht deine Sicherungen und listet die Container, VMs, Ordner-Sets und ZFS-Datasets auf, die es findet. Der neueste Snapshot ist überall vorgewählt. Was dieser Server noch hat, beginnt ausgeschaltet, weil eine Wiederherstellung es ersetzen würde. Ordner-Sets wissen nach einem Neuaufbau nicht mehr, woher sie kamen, daher gehen sie in einen Ordner, den du wählst.
4. **Wiederherstellung.** Alles Gewählte kommt auf einmal zurück, auf Wunsch angehalten gelassen, wenn du jeden Container und jede VM selbst starten willst, was nach einem Totalverlust am sichersten ist. Lade danach ein neues Recovery-Kit herunter: Das alte gehört zur vorigen Installation.

Ein gemeinsames Repository aus dem Kit erscheint unter den Orten als **Gemeinsames Repository bei** und der Name des Servers, dem es gehört. Der Assistent listet dort nur die eigenen Sicherungen dieses Servers auf. Siehe [Nach einem Notfall](#shared-emergency).

![Der Notfall-Assistent im ersten Schritt, mit den drei Wegen hinein.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![Der Notfall-Assistent im ersten Schritt, mit den drei Wegen hinein.](assets/screenshots/offsite/emergency-light.png#only-light)

### Beispiel: ein neuer Server aus dem Recovery-Kit {#example-emergency}

Der Server TOWER ist mit seinen Platten gestorben. Er hielt 23 Container und 2 VMs und kopierte sie und sein Selbstbackup nach Backblaze B2. Du hast `bombvault-recovery-kit.md` aus deinem Passwortmanager und eine neue Box mit Unraid darauf. Der alte `APP_KEY` ist weg, und du brauchst ihn nicht.

1. **Installiere BombVault** auf der neuen Box und gib ihm einen neuen `APP_KEY`, wie bei jeder Neuinstallation. Richte die VM-Verbindung unter **Einstellungen, Anbindungen, Host-SSH** ein, wenn du die VMs zurückhaben willst, weil eine VM-Wiederherstellung sie braucht.
2. **Starte den Assistenten.** Wähle auf **Willkommen bei BombVault** **Aus einer Sicherung wiederherstellen**. Wähle auf der Seite Wiederherstellung unter **Notfall** **Notfall-Assistent starten**.
3. **Zugang.** Wähle **Recovery-Kit hochladen** und lege die Datei ab. BombVault probiert seinen Schlüssel an jedem Speicherort aus und sagt zum Beispiel **Recovery-Kit gelesen. Sein Schlüssel öffnet 0 von 2 Speicherorten.** Der lokale Ordner sagt **nicht gefunden**, da die Platte gestorben ist, und Backblaze B2 sagt **Zugang fehlt**.
4. **Zugangsdaten speichern.** Das Kit trägt die Anmeldung jedes Ortes. Drücke **Zugangsdaten speichern**, und die Orte werden erneut probiert, Backblaze B2 sagt jetzt also **lesbar**. Ein Kit, das vor 10.5 geschrieben wurde, enthält nur die Standard-Zugangsdaten. Wo ein Ort noch **Zugang fehlt** sagt, trag seine Zugangsdaten unter **Fehlende Zugangsdaten eintragen** ein und drücke **Speicherorte noch einmal versuchen**.
5. **Schlüssel übernehmen.** Dieser Server schreibt seine Sicherungen jetzt mit dem alten Schlüssel, damit die neuen Sicherungen zu den alten passen. Er behält seinen eigenen `APP_KEY` im Template. **Übernahme rückgängig machen** funktioniert, bis die erste Sicherung gelaufen ist.
6. **Einstellungen.** Der Schritt öffnet sich an dem Ort, an dem der Schlüssel das Selbstbackup gefunden hat, hier B2, mit seiner Adresse schon eingetragen. Wähle **Einstellungen zurückholen**, was das neueste Selbstbackup von dort liest. BombVault startet sich zum Anwenden neu, und der Assistent macht von selbst weiter. Deine Speicherorte, Zeitpläne und Regeln sind zurück, und du meldest dich weiter mit dem Passwort dieses Servers an. Gib die Geheimnisse, die der Assistent auflistet, neu ein, etwa Benachrichtigungseinstellungen.
7. **Auswahl.** **Wo liegen die Sicherungen?** öffnet sich auf den Off-site-Kopien, wenn lokal nichts übrig ist. BombVault liest jeden Ort, damit ein toter die anderen nicht verdeckt, listet die 23 Container und 2 VMs auf und stellt jeden von dem Ort wieder her, der seinen neuesten Snapshot hat.
8. **Wiederherstellung.** Schalte **Danach gestoppt lassen** ein, was nach einem Totalverlust am sichersten ist, und starte. **Was passiert** sagt, was zurückkommt. Die Ergebniszeile liest sich zum Beispiel **25 wiederhergestellt, 0 fehlgeschlagen. Starte sie bei Bedarf unter Sicherungen.**
9. **Abschluss.** Starte zuerst deine Datenbanken, dann die Apps, die sie nutzen. Öffne **Einstellungen, Wiederherstellung** und lade ein neues Recovery-Kit herunter, weil das alte zur vorigen Installation gehört. Prüfe **Braucht dich** in der Übersicht, und führe **Jetzt alles sichern** aus, sobald alles funktioniert.

Fehlt auch das Selbstbackup, öffnet sich der Schritt auf **Überspringen: ich habe kein Selbstbackup** und sagt das. Die Speicherorte kommen dann allein aus dem Kit, du richtest Zeitpläne und Regeln neu ein, und die Off-site-Kopien warten, bis du ihre Sicherungen als eigene dieses Servers übernimmst, siehe [Nach einem Neuaufbau](#after-a-rebuild). **Auswahl** listet trotzdem auf, was die Off-site-Kopien enthalten. Ordner-Sets gehen in einen Ordner deiner Wahl. Container und VMs kommen als einfache Dateien in einem Ordner unter **Nur die Dateien können zurückkommen** zurück, weil ihre Definitionen neben dem lokalen Repository und im Selbstbackup liegen, und eine Off-site-Kopie enthält keins von beiden.

!!! tip "Geplanter Umzug oder Katastrophe"
    Der Notfall-Assistent stellt die eigenen Einstellungen von BombVault aus einer Sicherung wieder her. Für einen *geplanten* Umzug auf eine neue Box kannst du stattdessen deine Konfiguration direkt mit der Karte **Einstellungen exportieren / importieren** (eine portable JSON-Datei) mitnehmen. Siehe [Konfiguration](configuration.md#portable-settings-export-and-import).

### Aus einem alten Sicherungsordner {#restore-from-another-bombvault-repo}

**Aus einem alten Sicherungsordner** unter **Andere Quellen** auf der Seite Wiederherstellung öffnet das Repository einer *anderen* BombVault-Instanz (ein Share, der unter `/mnt` gemountet ist, oder eine entfernte Adresse) mit dem Recovery-Kit dieser Instanz oder ihrem eingetippten `APP_KEY`, in einer einmaligen, schreibgeschützten Sitzung. Durchsuche die dort gespeicherten Container, VMs, Ordner-Sets und ZFS-Datasets, wähle einen Snapshot und stelle ihn wieder her, und das wiederhergestellte Objekt wird zu einem normalen lokalen Container, einer VM oder einem Ordner-Set. In das andere Repository wird nie etwas geschrieben, und deine eigenen Sicherungseinstellungen bleiben unberührt (die Sitzung lebt im Arbeitsspeicher und läuft von selbst ab). Einen Container von Server A auf Server B zu holen heißt nicht, deine Einstellungen umzuleiten und danach zurückzusetzen. Dieses Fenster ist einmalig: Es öffnet eine Sitzung, stellt wieder her, was du wählst, und vergisst die andere Instanz. Willst du stattdessen eine dauerhafte Anordnung, bei der diese Box die Snapshots einer anderen Instanz nach Zeitplan in ihr eigenes Repository holt, ist das die Rolle **Abholer** auf der Seite Instanzen.

### Was dieser Server geholt hat {#restore-what-was-fetched}

Jede Quelle, von der dieser Server geholt hat, hat unter **Andere Quellen** auf der Seite Wiederherstellung eine eigene Zeile **Geholt von**. Eine geholte Kopie ist mit dem eigenen Schlüssel dieses Servers verschlossen, nicht mit dem der Quelle, darum wird nichts abgefragt: Der Schlüssel der Quelle öffnet sie nicht. Ordner-Sets und ZFS-Datasets gehen in einen Ordner deiner Wahl. Container und VMs kommen als einfache Dateien in einem Ordner zurück, weil eine Abholung die Snapshots mitbringt und nicht die Definitionen. Um sie neu anzulegen, nimm das Recovery-Kit und das Selbstbackup der Quelle im Notfall-Assistenten.

Ein Container, dessen Netzwerk auf diesem Server nicht existiert, etwa ein Unraid-Netz `br0` auf einem schlichten Docker-Host, zeigt unter seiner Zeile eine Netzwerkauswahl. BombVault legt ihn im gewählten Netzwerk an, zusammen mit seinen anderen Netzwerken. Die feste IP und MAC-Adresse gehörten zum alten Netzwerk und entfallen, das neue Netzwerk vergibt sie also.

## Schlüsselhüter {#key-keeper}

Ein Schlüsselhüter ist eine Instanz deiner Gruppe, die eine versiegelte Kopie des Recovery-Kits dieses Servers hält. Sie kann die Kopie selbst nicht öffnen. Geht dieser Server einmal verloren, fragt eine Neuinstallation, die deiner Gruppe beitritt, den Hüter nach der Kopie und öffnet sie mit deinem Wiederherstellungscode.

![Das Fenster mit dem Wiederherstellungscode: zwanzig Zeichen in vier Gruppen, einmal angezeigt.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![Das Fenster mit dem Wiederherstellungscode: zwanzig Zeichen in vier Gruppen, einmal angezeigt.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. Schalte auf der Seite Instanzen **Schlüsselhüter** auf der Karte der Instanz ein, die ihn halten soll, und wähle **Hinterlegen**. Eine Hinterlegung braucht ein Login-Passwort.
2. Dieser Server zeigt den **Wiederherstellungscode** einmal: zwanzig Buchstaben und Ziffern in vier Fünfergruppen. Schreibe ihn auf und bestätige mit **Ich habe den Code aufgeschrieben**. Dieser Server speichert den Code nicht, nur, was er braucht, um eine Hinterlegung dafür zu versiegeln.
3. Der Hüter hält jetzt die Hinterlegung, und dieser Server erneuert sie, wenn sich das Kit ändert. **Hinterlegung** auf der Karte dieser Instanz listet **Hinterlegung dieses Servers** mit jedem Hüter und seinem Zustand auf, und **Hinterlegungen, die dieser Server verwahrt** für die anderen. **Neuen Code erstellen** ersetzt den Code. Jeder Hüter wechselt zum neuen, wenn er die neue Hinterlegung bekommt.

An diesem Tag brauchst du drei Dinge: die zwölf Wörter deiner Gruppe, den Wiederherstellungscode und Zugang zur Oberfläche des Hüters, wo jemand **Erlauben** drücken muss. Wähle im Notfall-Assistenten **Über die Gruppe**, wähle die Hinterlegung, frage sie an und tippe den Wiederherstellungscode ein. Der Code wird auf dem neuen Server genutzt und nicht an den Hüter gesendet. Beim Hüter erscheint die Anfrage unter **Hinterlegung** mit dem, wer fragt, ob diese Instanz dort schon einmal gesehen wurde und wie oft sie gefragt hat. **Erlauben** gibt die versiegelte Kopie heraus, und das Erlauben braucht ein Login-Passwort. Die Neuinstallation hat nach dem Erlauben 30 Minuten Zeit, sie abzuholen, eine Anfrage, die niemand beantwortet, läuft nach einem Tag ab, und eine Instanz, die zu oft fragt, muss eine Stunde warten.

Ohne den Code kann niemand die Kopie öffnen, auch der Hüter nicht. Ist der Code verloren, erstelle einen neuen, solange dieser Server noch läuft. Ist der Server schon weg, funktionieren das Recovery-Kit und der eingetippte APP_KEY weiter. Der Hüter kann eine Hinterlegung jederzeit löschen, und der Besitzer muss dann erneut fragen.

## Recovery-Kit {#recovery-kit}

Das ist das Stück, das Notfallwiederherstellung auch dann möglich macht, wenn kein BombVault läuft.

**Recovery-Kit herunterladen** in der Karte **Für den Notfall** unter **Einstellungen, Wiederherstellung** gibt dir den **Schlüssel**, das **abgeleitete restic-Passwort**, jeden **Speicherort** mit dem Repository jedes Bereichs, die Zugangsdaten, die eine Neuinstallation braucht, um sie zu erreichen, und die **genauen restic-Befehle**, damit du direkt mit dem restic-CLI auf jeder Maschine wiederherstellen kannst. Es braucht ein Login-Passwort. Das Kit druckt den Schlüssel auch bei ausgeschalteter Verschlüsselung, weil die Container- und VM-Definitionen immer damit versiegelt sind. Nach einer Übernahme druckt es den übernommenen Sicherungsschlüssel und den APP_KEY dieses Servers, den eine Neuinstallation nur braucht, um die Einstellungen zurückzuholen. Eine Karte **Sichere dein Recovery-Kit** in der Übersicht erinnert dich, bis du es aufbewahrt hast.

!!! danger "Bewahre das Recovery-Kit außerhalb des Servers auf"
    Das Kit enthält das Geheimnis, das deine Sicherungen entschlüsselt. Bewahre es an einem sicheren Ort getrennt vom Server auf (ein Passwortmanager, ein Ausdruck in einem Safe). Verlierst du BombVault und `APP_KEY` ohne Recovery-Kit und ohne Schlüsselhinterlegung, lassen sich deine verschlüsselten Sicherungen nicht wiederherstellen.

!!! warning "Der neueste Snapshot ist nicht immer der, den du wiederherstellen willst"
    Seit restic 0.17 zeigt `restic snapshots` die Größe jedes Snapshots. Nach einem Datenverlust kann der neueste Snapshot der geleerte sein, stelle also keinen Snapshot wieder her, der weit kleiner ist als die davor. Nach Ransomware kann es der verschlüsselte in der üblichen Größe sein. Läuft BombVault noch, sieh dir zuerst seine Seite **Anomalien** an: Sie nennt die letzte gute Sicherung, und das Wiederherstellungsfenster wählt sie vor. Eine Wiederherstellung braucht keine Anomaliedaten von BombVault, und die Aufbewahrungspause behält immer nur mehr Snapshots.

### Der Host der Sicherungen eines Servers {#backup-host}

Jeder Server schreibt seine Sicherungen unter einem eigenen restic-Host: `bv-` und zwölf Zeichen, die aus dem Schlüssel der Sicherungen berechnet werden, etwa `bv-3f9a0c41d2e7`. Sicherungen aus der Zeit, bevor ein Server einen eigenen hatte, tragen den Host `bombvault`. Das Kit nennt den Host dieses Servers, und seine Befehle geben beide mit `--host` an, damit `latest` nie eine Sicherung erwischt, die ein anderer Server in dasselbe Repository geschrieben hat. Ein Server, der die Sicherungen eines anderen übernimmt, macht unter dem Host jenes Servers weiter, weil der Host aus dem Schlüssel kommt. Für jedes [gemeinsame Repository](#shared-repository), in das der Server kopiert oder dessen Besitzer er ist, nennt das Kapitel **Shared repositories** im Kit die anderen Server dort mit ihren Hosts.

### Das Kit versiegeln {#sealing-the-kit}

Hast du **Exporte mit age verschlüsseln** unter **Einstellungen, System** eingeschaltet, wird auch das Kit damit versiegelt und als `bombvault-recovery-kit.md.age` heruntergeladen. Es ist ASCII-gepanzert statt binär und damit weiterhin Klartext: Es in einen Passwortmanager einzufügen oder auszudrucken funktioniert genau wie zuvor, der Inhalt ist ohne deinen Schlüssel unlesbar.

!!! warning "Speichere den age-Schlüssel nicht im Kit"
    Du brauchst deinen **privaten** age-Schlüssel, um ein versiegeltes Kit zu öffnen. Bewahre ihn dort auf, wo er nicht vom Kit selbst abhängt, sonst hast du zwei Dinge wiederherzustellen statt einem. Das Versiegeln lohnt sich, wenn das Kit an einem Ort liegt, den du nicht ganz kontrollierst (ein geteilter Passwortmanager, Cloud-Notizen, ein Ausdruck in einem Büro). Ein Kit in deinem eigenen Safe ist durch den Safe schon geschützt.

    Ist die Verschlüsselung an und kein brauchbarer Empfänger eingerichtet, wird der Download rundheraus abgelehnt. BombVault fällt nie darauf zurück, den Hauptschlüssel im Klartext herauszugeben.

### Wenn du das Kit nicht zur Hand hast {#if-you-do-not-have-the-kit-to-hand}

Das Passwort wird nirgends gespeichert, es wird aus dem Schlüssel der Sicherungen **berechnet**, du kannst es also mit nichts als dem Schlüssel und einer Shell selbst reproduzieren:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

Das ist HMAC-SHA256 über die feste Zeichenfolge `bombvault:restic-repo`, mit den Rohbytes des Hex-`APP_KEY` als Schlüssel, ausgegeben als 64 Hex-Zeichen in Kleinbuchstaben. Derselbe Wert steht im Kit, aufgeführt als abgeleitetes restic-Passwort. Das ist für den Tag, an dem das Kit dort ist, wo du nicht bist. Nimm auf einem Server, der die Sicherungen eines anderen übernommen hat, den übernommenen Sicherungsschlüssel, das ist der alte APP_KEY jenes Servers.

Der [Host](#backup-host) kommt aus demselben Schlüssel und der Zeichenfolge `bombvault:host`: `bv-` und die ersten zwölf dieser Zeichen.

```sh
echo "bv-$(printf 'bombvault:host' | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r | cut -c1-12)"
```

!!! warning "Bei einem empfangenen Repository nimm den Schlüssel der SENDENDEN Instanz"
    Ein Repository, das über eine Off-site-Kopie hier angekommen ist, wurde von der Maschine angelegt, die es gesendet hat, mit **ihrem** Schlüssel. Eine Ableitung aus dem Schlüssel der empfangenden Box ergibt ein Passwort, das restic ablehnt, was sich genau wie ein beschädigtes Repository liest und keines ist. Das ist der übliche Grund, warum `restic check` auf einem empfangenen Repo immer wieder nach einem Passwort fragt.

Weil die Wiederherstellungsdefinitionen **in** jedem Repo liegen (`<repo>/def`, `<repo>/vm-def`), ist ein kopierter Repo-Ordner vollständig in sich geschlossen, das Kit plus das Repo ist also alles, was eine Bare-Metal-Wiederherstellung braucht. Eine Off-site-Kopie und eine geholte Kopie enthalten die Snapshots ohne die Definitionen: Sie geben Dateien zurück, und ein Container oder eine VM wird aus dem Selbstbackup neu angelegt.

## Einen Datenbank-Dump zurückholen {#database-dumps}

Ein Datenbank-Dump ist ein eigener Wiederherstellungspunkt im Container-Repository, mit dem Tag `dbdump:<container>`, der die einzelne Datei `/dbdump/<container>.sql` enthält. BombVault listet sie auf dem Tab **Sicherungen** des Containers auf, lädt sie herunter und importiert sie. Unten stehen dieselben Schritte nur mit restic, für den Tag, an dem BombVault nicht da ist.

```sh
restic -r <repo> snapshots --host <host> --host bombvault --tag dbdump:<container>
restic -r <repo> dump --host <host> --host bombvault --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

`<host>` ist der [Host](#backup-host) des Servers aus dem Recovery-Kit.

Die Tags `dbversion:` und `dbname:` an jedem Dump sagen, aus welcher Serverversion er stammt und welche Datenbanken er enthält. Eine vollständige Datei endet mit `-- PostgreSQL database cluster dump complete` oder `-- Dump completed`.

Importiere ihn in einen Container derselben oder einer neueren Version (PostgreSQL) oder derselben Hauptversion (MySQL und MariaDB), der einmal mit leerem Datenordner gestartet wurde, damit er sich initialisiert. Der Host braucht keinen Datenbank-Client, der Container hat einen:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Für eine einzelne Datenbank aus einem vollständigen Dump nehmen MySQL und MariaDB `--one-database <name>` am Client-Befehl. Ein PostgreSQL-Dump hat pro Datenbank einen Abschnitt, jeder beginnt mit einer Zeile `\connect <name>`: Kopiere diesen Abschnitt in eine eigene Datei und importiere ihn nach dem Anlegen der Datenbank mit `-d <name>`.

!!! warning "Ein Root-Dump trägt die Benutzer des Servers"
    Ein vollständiger MySQL- oder MariaDB-Dump als root enthält die Systemdatenbank `mysql`, sein Import ersetzt also die Konten des neuen Servers, das Passwort von root eingeschlossen, durch die aus dem Dump. Bei PostgreSQL ist `role ... already exists` für den Benutzer, den der Container angelegt hat, zu erwarten und harmlos.
