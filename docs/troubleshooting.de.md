# Fehlerbehebung

Eine kurze FAQ. Die vollständige host-seitige Fehlerbehebungstabelle für VM-über-SSH (permission-denied, Host-Key-Verifizierung, fehlende Template-Variablen und mehr) findest du in der [Anleitung zum VM-Backup über SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) auf GitHub.

## Etwas ist nicht richtig verdrahtet

Füge der Übersicht unter **Anpassen** die Karte **Host-Integration** hinzu. Die Prüfung der Host-Integration prüft jeden Mount und jedes CLI (Docker-Socket, libvirt, restic, qemu-img, rclone) und meldet fehlende Teile, und **Jetzt prüfen** führt sie erneut aus. Beginne hier, bevor du einen Bug vermutest: Ein fehlender Mount oder ein unerreichbarer Host fällt sofort auf.

## Ich erreiche die Weboberfläche nicht

BombVault liefert HTTPS von Haus aus auf Port `3443` (selbstsigniertes Zertifikat), öffne also `https://<your-unraid-ip>:3443`. Akzeptiere die Warnung zum selbstsignierten Zertifikat, oder setze BombVault hinter einen Reverse Proxy mit deinem eigenen Zertifikat. Wenn du mit `HTTP_ONLY=true` läufst, liefert es stattdessen schlichtes HTTP auf Port `3000` (gedacht für die Verwendung hinter einem TLS-terminierenden Proxy).

## Ich habe meinen APP_KEY verloren

`APP_KEY` leitet das Passwort des restic-Repositorys ab. Ohne ihn, ohne das Recovery-Kit und ohne eine Hinterlegung in deiner Gruppe können verschlüsselte Sicherungen nicht wiederhergestellt werden. Deshalb zeigt die Übersicht **Sichere dein Recovery-Kit**, bis du es heruntergeladen hast. Siehe [Off-site & Wiederherstellung](offsite-recovery.md#recovery-kit). Erzeuge einen Schlüssel mit `openssl rand -hex 32` und bewahre ihn getrennt vom Server auf, bevor du dich auf irgendeine Sicherung verlässt.

## Ich habe das Login-Passwort vergessen

Wähl auf der Login-Seite **Passwort vergessen?**. BombVault schreibt einen Code aus 12 Zeichen in sein Container-Log (unter Unraid: Docker-Tab, BombVault-Symbol, **Logs**). Tipp ihn innerhalb von 10 Minuten mit einem neuen Passwort ein. Das Zurücksetzen schaltet außerdem die Zwei-Faktor-Authentifizierung ab, entfernt jeden Passkey, widerruft jedes API-Token, jeden MCP-Schlüssel und jede OAuth-Anmeldung, beendet jede andere Sitzung und schaltet die Kopplung von Handys aus. Füg deine Passkeys unter **Einstellungen, Sicherheit** wieder hinzu, schalte die Kopplung von Handys dort wieder ein und erstell neue Tokens und Schlüssel, wo du sie noch brauchst. Siehe [Sicherheitsmodell](configuration.md#login).

## Sicherungen stehen still, weil sich der Schlüssel nicht mehr öffnen lässt

Dieser Server hat im Notfall-Assistenten den Sicherungsschlüssel eines anderen Servers übernommen, und danach wurde `APP_KEY` geändert. Der übernommene Schlüssel ist mit dem alten APP_KEY versiegelt, BombVault kann ihn also nicht öffnen und sichert lieber nichts, als mit dem falschen Schlüssel zu schreiben. **Braucht dich** auf der Übersicht nennt das. Setz den APP_KEY auf den alten Wert zurück oder lade das Recovery-Kit auf der Seite Wiederherstellung unter **Notfall** erneut hoch.

## Ein Handy landet auf der Login-Seite

Ein gekoppeltes Handy bekommt seine Sitzung über die Gruppe nur, solange **Handys können sich mit den zwölf Wörtern der Gruppe koppeln** unter **Einstellungen, Sicherheit** an ist. Ein Zurücksetzen des Passworts schaltet das aus. Auch eine App, die älter ist als der Server, bekommt keine Sitzung, bis sie aktualisiert ist. Ein Server, den du über seine Adresse hinzugefügt hast, fragt immer nach dem Passwort. Ein Handy, das von der Seite Instanzen entfernt wurde, kommt ausgeschaltet zurück, wenn seine App wieder öffnet; schalte in seinen Details unter **Überwachung** **Diese Instanz abfragen** ein, um es hereinzulassen.

## Das VM-Backup verbindet sich nicht

Das VM-Backup spricht über SSH mit libvirt, nie über einen Mount.

- Bestätige, dass SSH auf dem Host aktiviert ist und BombVaults öffentlicher Schlüssel in `/root/.ssh/authorized_keys` autorisiert ist (Einstellungen, Anbindungen, Host-SSH zeigt den Schlüssel und einen Knopf **Verbindung testen**).
- Setze in einem benutzerdefinierten `br0.x`-Netzwerk `LIBVIRT_HOST` auf deine Unraid-LAN-IP (der Container kann den Host dort nicht über `host.docker.internal` erreichen). Aktiviere **Settings, Docker, Host access to custom networks**.
- Wenn du Unraids SSH-Port geändert hast, setze `LIBVIRT_SSH_PORT` passend.
- Setze auf TrueNAS `LIBVIRT_URI` mit dem eigenen libvirt-Socket von TrueNAS, gib der App die Capability `SETPCAP` und füge den Schlüssel unter **Credentials, Users** bei root hinzu (siehe [TrueNAS SCALE](getting-started.md#truenas)).
- Die vollständige Schritt-für-Schritt-Diagnose (Erreichbarkeitstest, VLAN-Routing, `Permission denied (publickey)`, `Host key verification failed`) steht in der [Anleitung zum VM-Backup über SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Ein Live-VM-Snapshot lief nicht

Live-Snapshots benötigen den in der VM installierten qemu-Gast-Agenten und den Datenträger auf `/mnt/cache` (oder `/mnt/diskX`), nicht `/mnt/user`. Bei einer ausgeschalteten VM fällt Live automatisch auf geordnet zurück. Eine geordnete Sicherung fährt die VM herunter, sichert die Datenträger und startet sie dann neu, sodass sie immer konsistent ist. Sie wartet bis zu zehn Minuten, bis der Gast heruntergefahren ist, und schaltet ihn nie hart aus, damit ein Windows-Gast, der gerade Updates installiert, nicht mittendrin abbricht. Ein Gast, der danach noch läuft, bleibt in Ruhe, und diese Sicherung schlägt mit dem Grund fehl.

## Eine Sicherung schlug mit "repository is already locked" fehl

Das ist meist ein verwaister restic-Lock, der zurückblieb, als der Container mitten im Betrieb aktualisiert oder neu gestartet wurde. BombVault erkennt einen nachweislich verwaisten Lock, löst ihn zwangsweise und wiederholt einmal, automatisch. Bleibt er bestehen, öffne die Karte **Sicherungen prüfen** unter **Einstellungen, Prüfung** und wähl in der Zeile des betroffenen Bereichs unter **Mehr** **Entsperren**, um einen veralteten Lock von Hand zu lösen. Ein echtes Problem kommt trotzdem ans Licht, statt versteckt zu werden. Nach einem Neustart wartet BombVault, bis so ein Lock zehn Minuten lang nicht erneuert wurde. Ein restic, das noch läuft, zum Beispiel in einem zweiten BombVault auf demselben Repository, erneuert seinen Lock alle fünf Minuten.

## Meine Off-site-Kopie lief nach einer Sicherung nicht

Off-site-Kopien sind absichtlich Best-Effort, sodass ein Off-site-Aussetzer die lokale Sicherung nie fehlschlagen lässt. Prüfe, wann der Speicherort kopiert: Seine Karte **Kopieren** sagt **Nach jeder Sicherung** oder nennt einen eigenen Plan, und **Einstellungen, Zeitplan, Off-site-Kopien pro Bereich** kann einem Bereich einen eigenen Plan geben. **Jetzt replizieren** in der Karte **Off-site-Kopie** des Bereichs unter **Einstellungen, Speicherorte** startet eine Kopie sofort, und der Statuskopf der Übersicht sagt **Off-site-Kopie läuft**, solange sie läuft. Eine fehlgeschlagene Kopie steht unter **Braucht dich**.

## Dem Zertifikat eines rest-servers wird nicht vertraut

restic lehnt ein Zertifikat ab, das es nicht kennt, und die Fehlermeldung sagt, wohin es gehört. Kopier das selbstsignierte Zertifikat des rest-servers, oder das der Stelle, die es signiert hat, als PEM-Datei in den Ordner `certs` in BombVaults Datenordner (`/config/certs` im Container, `appdata/bombvault/certs` unter Unraid) und teste noch einmal. Ein Neustart ist nicht nötig.

## Eine große Sicherung wurde wegen fehlenden Speichers beendet

Setz **Speicherlimit pro Sicherung (MiB, 0 = keins)** auf der Karte **Leistung** unter **Einstellungen, System**, deutlich unter den Speicher, den der Container nutzen darf. Nahe am Limit gibt restic öfter Speicher frei, was etwas Tempo kostet, sodass die Sicherung langsamer wird, statt beendet zu werden.

## Eine Wiederherstellung brach ab, bevor sie startete

Bevor irgendetwas gestoppt oder entfernt wird, führt die Wiederherstellung eine Vorab-Konfliktprüfung durch: Sie prüft, ob die statische IP des Containers und die veröffentlichten Host-Ports frei sind. Belegt ein anderer Container einen davon bereits, bricht sie mit einer klaren Meldung ab, statt eine halbfertige Wiederherstellung zu hinterlassen. Gib den belegten Port oder die IP frei und versuche es erneut.

Gibt es das Netzwerk des Containers nicht, etwa weil seine TrueNAS-App gelöscht wurde, nennt die Prüfung vor der Wiederherstellung das Netzwerk, und die Wiederherstellung bricht ab, bevor sie etwas ändert. Installiere die App in TrueNAS unter demselben Namen neu, das legt das Netzwerk wieder an, und stelle dann ihre Daten wieder her.

## Ein schlichter Export schlug fehl, statt eine Datei zu schreiben

Ist **Exporte mit age verschlüsseln** unter **Einstellungen, System** an, aber kein gültiger Empfänger gesetzt, schlägt ein Export mit einem klaren Fehler fehl, statt Klartext zu schreiben. Füge einen gültigen Empfänger hinzu (einen age-Public-Key oder einen SSH-Public-Key), oder schalte die Verschlüsselung aus, wenn der Export Klartext sein soll. Siehe [Funktionen](features.md).

## Ein Datenbank-Dump schlug fehl

Ein fehlgeschlagener Dump lässt nie die Sicherung drumherum scheitern; er wird als eigener fehlgeschlagener Lauf festgehalten, und der Grund nennt, was zu tun ist.

- **Anmeldung abgelehnt.** Der Dump meldet sich mit den Passwort-Variablen des Containers selbst an (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` oder deren `_FILE`-Varianten). Prüfe sie am Datenbank-Container. Eine `_FILE`-Variable, die auf ein Secret zeigt, das der Benutzer des Containers nicht lesen darf, scheitert genauso.
- **Fehlende Rechte.** Mit einem zufälligen Root-Passwort kann sich der Dump nur als App-Benutzer anmelden und erfasst dann nur deren eine Datenbank; MySQL 8.4 und neuer lehnt ihn unter Umständen ganz ab. Gib dem Container ein echtes Root-Passwort, oder schalte den Dump für ihn ab.
- **Die Systemtabellen brauchen ein Upgrade.** MariaDB verweigert den Dump, wenn seine Systemtabellen aus einer älteren Version stammen (Fehler 1558). Füge die Variable `MARIADB_AUTO_UPGRADE=1` hinzu und starte den Container neu, oder führe `mariadb-upgrade` einmal darin aus.
- **Kein Dump-Werkzeug.** Ein schlankes oder selbst gebautes Image ohne `pg_dump`, `mysqldump` oder `mariadb-dump` lässt sich nicht dumpen. Nimm das offizielle Image, oder schalte den Dump ab.
- **Ein Zeitlimit.** Ein Dump bekommt `DB_DUMP_MAX_HOURS` (voreingestellt 6), die Sicherung drumherum `BACKUP_MAX_HOURS`, und ein Dump, der nicht mehr vorankommt, wird nach `BACKUP_STALL_HOURS` abgebrochen. Hinter dem letzten Fall steckt meist eine Sperre, die die Anwendung hält. Erhöhe das Limit, das gegriffen hat, oder dumpe, während die Anwendung ruhig ist.
- **Der Container ist pausiert oder startet neu.** Der Dump spricht mit dem laufenden Server. Startet der Container immer wieder neu, sagt sein eigenes Log, warum.
- **Ein beschädigter Dump ließ sich nicht entfernen.** Einen Dump, den BombVault nicht zu Ende bringen konnte, löscht es wieder. Scheitert dieses Löschen, bleibt der Dump als beschädigt markiert in der Liste stehen und du kannst ihn dort löschen.

## Ein Import schlug fehl

Ein Import stoppt den Container, schiebt seinen Datenordner zur Seite und lässt das Image einen leeren an dessen Stelle anlegen. Scheitert ein Schritt vor dem eigentlichen Import, legt BombVault den alten Ordner von selbst zurück. Scheitert der Import, behält der Container den frischen Ordner, und der alte bleibt als `<Datenordner>.bombvault-before-import-<Zeitstempel>` daneben liegen; die Fehlermeldung des Laufs nennt den genauen Pfad.

Ein Datenordner, der ein eigenes Dataset ist, wie auf TrueNAS, lässt sich nicht umbenennen. Der Import lässt den Ordner dann an seinem Platz und verschiebt seinen Inhalt in den aufbewahrten Ordner daneben, das Dataset darüber braucht also Platz dafür. Das prüft der Import, bevor er den Container stoppt.

Von Hand zurücklegen: Container stoppen, den aktuellen Datenordner aus dem Weg umbenennen, den aufbewahrten Ordner auf den ursprünglichen Namen zurück umbenennen und den Container starten. Bei einem Dataset verschiebst du den Inhalt, statt die Ordner umzubenennen. Auf Unraid erledigt das der Dateimanager im Tab Shares.

## Eine ZFS-Dataset-Sicherung schlug fehl oder hat ein Dataset übersprungen {#zfs-datasets}

Jedes Problem trägt einen Grundcode in eckigen Klammern, und die Seite [ZFS-Datasets](zfs-datasets.md#reason-codes) listet alle mit der Abhilfe. Die drei häufigsten:

- **`snapshot-loop`**: Der Snapshot kam nicht bei BombVault an, weil Host Data neue Einhängungen nicht weiterreicht. Bearbeite den Container, setze den Access Mode von Host Data auf Read/Write - Slave und starte BombVault neu.
- **`key-not-loaded`**: Ein verschlüsseltes Dataset, dessen Schlüssel nicht geladen ist, wird übersprungen. Lade den Schlüssel mit `zfs load-key` und hänge das Dataset ein; die nächste Sicherung nimmt es mit.
- **`ssh-auth`**: Der Server hat BombVaults Schlüssel abgelehnt. **ZFS-Verbindung zu diesem Server**, geöffnet mit **Verbindung ansehen** auf der Seite Sicherungen, zeigt den Befehl, der ihn freischaltet; führe ihn einmal auf dem Server aus.

## Ein Eintrag bleibt bei "Lernt N/10" stehen

Die meisten Anomalie-Prüfungen beginnen nach 10 erfolgreichen Sicherungen eines Eintrags, und die Zählung beginnt neu nach **Als erwartet markieren** und nachdem sich die Auswahl des Eintrags geändert hat. Ein Eintrag ohne Zeitplan lernt nicht, und ein Container ohne Appdata hat nichts, woraus er lernen könnte, was sein Abzeichen sagt. **Lernfortschritt** auf der Seite Anomalien zeigt pro Eintrag einen Ring aus zehn Teilen.

## Die Aufbewahrung löscht bei einem Eintrag keine alten Sicherungen mehr

Eine offene kritische Anomalie hält sie fest: Die Quelle des Eintrags ist fast leer, stark geschrumpft, oder eine Sicherung hat die meisten Daten neu gespeichert. Öffne die Anomalie über das Abzeichen am Eintrag. Wenn Daten fehlen oder verschlüsselt wurden, stell zuerst aus der verlinkten letzten guten Sicherung wieder her. Quittiere die Anomalie danach oder markiere sie als erwartet, wenn die Änderung von dir kam, und der nächste Lauf räumt wie gewohnt auf. Die Vorschau der Aufbewahrung markiert so einen Eintrag als behalten. Bei einem ZFS-Eintrag behält nur das in der Anomalie genannte Dataset seine alten Sicherungen, die anderen Datasets des Baums werden wie gewohnt aufgeräumt.

## Das manuelle Aufräumen meldet, dass einige Einträge behalten wurden

Dieselbe Ursache: Das Aufräumen lässt die alten Sicherungen eines Eintrags mit so einer Anomalie in Ruhe und nennt den Eintrag in seiner Meldung. Alles andere wird wie gewohnt aufgeräumt.

## Der Verlaufsimport meldet ein Repository, das nicht gelesen werden konnte

Nach dem Update liest BombVault einmal die Größen früherer Sicherungen aus jedem Repository. Ein Repository, das zu diesem Zeitpunkt nicht erreichbar war, etwa ein ausgefallener Speicherort oder eine nicht eingehängte Freigabe, steht in der Karte **Anomalien** unter **Einstellungen, Prüfung** und wird einmal am Tag erneut versucht. Seine Einträge lernen in der Zwischenzeit aus neuen Sicherungen.

## Die Speicherplatzwarnung passt nicht zum Unraid-Dashboard

Auf dem Unraid-User-Share (`/mnt/user`) ist der freie Platz der des ganzen Arrays, nicht der einer einzelnen Platte. Entfernte Repositories werden nur über rclone-Remotes gemessen, die ihren freien Platz melden; S3-, B2-, REST- und SFTP-Repositories haben keine Angabe und stehen in der Karte **Anomalien** als nicht gemessen.

## Ein KI-Assistent verbindet sich nicht

Die Seite [MCP-Server](mcp.md#troubleshooting) listet auf, was jeder Statuscode und jede Ablehnung des MCP-Endpunkts bedeutet und was du dagegen tun kannst.

## Der Container startet ständig neu oder wirkt ungesund

BombVault meldet gesund/ungesund aus seinem eigenen `/api/health`. Ein Auto-Heal-Werkzeug (wie Autoheal) kann ihn automatisch neu starten, falls sich die Engine je verklemmt. Prüfe das Container-Log und die Karte **Host-Integration** auf die zugrunde liegende Ursache.

## Immer noch festgefahren?

- Lies die vollständigen Seiten [Konfiguration](configuration.md) und [Off-site & Wiederherstellung](offsite-recovery.md).
- Frage im [Unraid-Support-Thread](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Öffne ein [GitHub-Issue](https://github.com/junkerderprovinz/bombvault/issues).
