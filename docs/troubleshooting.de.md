# Fehlerbehebung

Eine kurze FAQ. Die vollständige host-seitige Fehlerbehebungstabelle für VM-über-SSH (permission-denied, Host-Key-Verifizierung, fehlende Template-Variablen und mehr) findest du in der [Anleitung zum VM-Backup über SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) auf GitHub.

## Etwas ist nicht korrekt verdrahtet

Öffne `/spike` in der Web-Oberfläche. Die Prüfung der Host-Integration prüft jeden Mount und jedes CLI (Docker-Socket, libvirt, restic, qemu-img, rclone) und meldet fehlende Teile. Beginne hier, bevor du einen Bug vermutest: ein fehlender Mount oder ein nicht erreichbarer Host zeigt sich sofort.

## Ich erreiche die Web-Oberfläche nicht

BombVault liefert HTTPS von Haus aus auf Port `3443` (selbstsigniertes Zertifikat), öffne also `https://<your-unraid-ip>:3443`. Akzeptiere die Warnung zum selbstsignierten Zertifikat, oder setze BombVault hinter einen Reverse Proxy mit deinem eigenen Zertifikat. Wenn du mit `HTTP_ONLY=true` läufst, liefert es stattdessen schlichtes HTTP auf Port `3000` (gedacht für die Verwendung hinter einem TLS-terminierenden Proxy).

## Ich habe meinen APP_KEY verloren

`APP_KEY` leitet das Passwort des restic-Repositorys ab. Ohne ihn (und ohne das Wiederherstellungspaket für den Verschlüsselungsschlüssel) können verschlüsselte Backups nicht wiederhergestellt werden. Deshalb nervt dich das Dashboard, das Recovery-Kit herunterzuladen. Siehe [Off-site & Wiederherstellung](offsite-recovery.md). Erzeuge einen Schlüssel mit `openssl rand -hex 32` und bewahre ihn off-box auf, bevor du dich auf ein Backup verlässt.

## Das VM-Backup verbindet sich nicht

Das VM-Backup spricht über SSH mit libvirt, nie über einen Mount.

- Bestätige, dass SSH auf dem Host aktiviert ist und BombVaults öffentlicher Schlüssel in `/root/.ssh/authorized_keys` autorisiert ist (Einstellungen, System, VM-Backup über SSH zeigt den Schlüssel und einen Button **Verbindung testen**).
- Setze in einem benutzerdefinierten `br0.x`-Netzwerk `LIBVIRT_HOST` auf deine Unraid-LAN-IP (der Container kann den Host dort nicht über `host.docker.internal` erreichen). Aktiviere **Settings, Docker, Host access to custom networks**.
- Wenn du Unraids SSH-Port geändert hast, setze `LIBVIRT_SSH_PORT` passend.
- Die vollständige Schritt-für-Schritt-Diagnose (Erreichbarkeitstest, VLAN-Routing, `Permission denied (publickey)`, `Host key verification failed`) steht in der [Anleitung zum VM-Backup über SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Ein Live-VM-Snapshot lief nicht

Live-Snapshots benötigen den in der VM installierten qemu-Gast-Agenten und den Datenträger auf `/mnt/cache` (oder `/mnt/diskX`), nicht `/mnt/user`. Bei einer ausgeschalteten VM fällt Live automatisch auf geordnet zurück. Ein geordnetes Backup fährt die VM herunter, sichert die Datenträger und startet sie dann neu, sodass es immer konsistent ist.

## Ein Backup schlug mit "repository is already locked" fehl

Das ist meist ein verwaister restic-Lock, der zurückblieb, als der Container mitten im Betrieb aktualisiert oder neu gestartet wurde. BombVault erkennt einen nachweislich verwaisten Lock, löst ihn zwangsweise und wiederholt einmal, automatisch. Falls er bestehen bleibt, nutze **Einstellungen, Integrität & Wartung, Entsperren** für den betroffenen Bereich, um einen veralteten Lock von Hand zu lösen. Ein echtes Problem tritt weiterhin zutage, statt verborgen zu werden.

## Meine Off-site-Kopie erfolgte nicht nach einem Backup

Off-site-Replikation ist per Design Best-Effort, sodass ein Off-site-Aussetzer das lokale Backup nie fehlschlagen lässt. Prüfe den Off-site-Zeitplan für diesen Bereich (Einstellungen, Zeitpläne): ein leerer Zeitplan repliziert nach jedem lokalen Backup, während eine Taktung seltener liefert. Nutze **Jetzt replizieren** im Off-site-Tab für einen Lauf auf Abruf, und beobachte die Replikationsanzeige im Dashboard.

## Eine Wiederherstellung brach ab, bevor sie startete

Bevor irgendetwas gestoppt oder entfernt wird, führt die Wiederherstellung eine Vorab-Konfliktprüfung durch: sie verifiziert, dass die statische IP des Containers und die veröffentlichten Host-Ports frei sind. Wenn ein anderer Container einen davon bereits belegt, bricht sie mit einer klaren, handlungsleitenden Meldung ab, statt eine halbfertige Wiederherstellung zu hinterlassen. Gib den konfliktierenden Port oder die IP frei und versuche es erneut.

## Ein Schlichtexport schlug fehl, statt eine Datei zu schreiben

Wenn die age-Verschlüsselung an ist (Einstellungen), aber kein gültiger Empfänger gesetzt ist, schlägt ein Export mit einem klaren Fehler fehl, statt Klartext zu schreiben. Füge einen gültigen Empfänger hinzu (einen age-Public-Key oder einen SSH-Public-Key), oder schalte die Verschlüsselung aus, wenn der Export Klartext sein soll. Siehe [Funktionen](features.md).

## Ein Datenbank-Dump schlug fehl

Ein fehlgeschlagener Dump lässt nie das Backup drumherum scheitern; er wird als eigener fehlgeschlagener Lauf festgehalten, und der Grund nennt, was zu tun ist.

- **Anmeldung abgelehnt.** Der Dump meldet sich mit den Passwort-Variablen des Containers selbst an (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` oder deren `_FILE`-Varianten). Prüfe sie am Datenbank-Container. Eine `_FILE`-Variable, die auf ein Secret zeigt, das der Benutzer des Containers nicht lesen darf, scheitert genauso.
- **Fehlende Rechte.** Mit einem zufälligen Root-Passwort kann sich der Dump nur als App-Benutzer anmelden und erfasst dann nur deren eine Datenbank; MySQL 8.4 und neuer lehnt ihn unter Umständen ganz ab. Gib dem Container ein echtes Root-Passwort, oder schalte den Dump für ihn ab.
- **Die Systemtabellen brauchen ein Upgrade.** MariaDB verweigert den Dump, wenn seine Systemtabellen aus einer älteren Version stammen (Fehler 1558). Füge die Variable `MARIADB_AUTO_UPGRADE=1` hinzu und starte den Container neu, oder führe `mariadb-upgrade` einmal darin aus.
- **Kein Dump-Werkzeug.** Ein schlankes oder selbst gebautes Image ohne `pg_dump`, `mysqldump` oder `mariadb-dump` lässt sich nicht dumpen. Nimm das offizielle Image, oder schalte den Dump ab.
- **Ein Zeitlimit.** Ein Dump bekommt `DB_DUMP_MAX_HOURS` (voreingestellt 6), das Backup drumherum `BACKUP_MAX_HOURS`, und ein Dump, der nicht mehr vorankommt, wird nach `BACKUP_STALL_HOURS` abgebrochen. Hinter dem letzten Fall steckt meist eine Sperre, die die Anwendung hält. Erhöhe das Limit, das gegriffen hat, oder dumpe, während die Anwendung ruhig ist.
- **Der Container ist pausiert oder startet neu.** Der Dump spricht mit dem laufenden Server. Startet der Container immer wieder neu, sagt sein eigenes Log, warum.
- **Ein beschädigter Dump ließ sich nicht entfernen.** Einen Dump, den BombVault nicht zu Ende bringen konnte, löscht es wieder. Scheitert dieses Löschen, bleibt der Dump als beschädigt markiert in der Liste stehen und du kannst ihn dort löschen.

## Ein Import schlug fehl

Ein Import stoppt den Container, schiebt seinen Datenordner zur Seite und lässt das Image einen leeren an dessen Stelle anlegen. Scheitert ein Schritt vor dem eigentlichen Import, legt BombVault den alten Ordner von selbst zurück. Scheitert der Import, behält der Container den frischen Ordner, und der alte bleibt als `<Datenordner>.bombvault-before-import-<Zeitstempel>` daneben liegen; die Fehlermeldung des Laufs nennt den genauen Pfad.

Von Hand zurücklegen: Container stoppen, den aktuellen Datenordner aus dem Weg umbenennen, den aufbewahrten Ordner auf den ursprünglichen Namen zurück umbenennen und den Container starten. Auf Unraid erledigt das der Dateimanager im Tab Shares.

## Ein ZFS-Dataset-Backup schlug fehl oder hat ein Dataset übersprungen {#zfs-datasets}

Jedes Problem trägt einen Grundcode in eckigen Klammern, und die Seite [ZFS-Datasets](zfs-datasets.md#reason-codes) listet alle mit der Abhilfe. Die drei häufigsten:

- **`snapshot-loop`**: Der Snapshot kam nicht bei BombVault an, weil Host Data neue Einhängungen nicht weiterreicht. Bearbeite den Container, setze den Access Mode von Host Data auf Read/Write - Slave und starte BombVault neu.
- **`key-not-loaded`**: Ein verschlüsseltes Dataset, dessen Schlüssel nicht geladen ist, wird übersprungen. Lade den Schlüssel mit `zfs load-key` und hänge das Dataset ein; das nächste Backup nimmt es mit.
- **`ssh-auth`**: Der Server hat BombVaults Schlüssel abgelehnt. Die Verbindungskarte auf der ZFS-Seite zeigt den Befehl, der ihn freischaltet; führe ihn einmal auf dem Server aus.

## Ein Element bleibt bei "Lernt N/10" stehen

Die meisten Anomalie-Prüfungen beginnen nach 10 erfolgreichen Backups eines Elements, und die Zählung beginnt neu nach **Als erwartet markieren** und nachdem sich die Auswahl des Elements geändert hat. Ein Element ohne Zeitplan lernt nicht, und ein Container ohne Appdata hat nichts, woraus er lernen könnte; das sagt auch sein Abzeichen.

## Die Aufbewahrung löscht bei einem Element keine alten Backups mehr

Eine offene kritische Anomalie hält sie fest: Die Quelle des Elements ist fast leer, stark geschrumpft, oder ein Backup hat die meisten Daten neu gespeichert. Öffne die Anomalie über das Abzeichen am Element. Wenn Daten fehlen oder verschlüsselt wurden, stelle zuerst aus dem verlinkten letzten guten Backup wieder her. Quittiere die Anomalie danach, oder markiere sie als erwartet, wenn die Änderung von dir kam, und der nächste Lauf räumt wieder wie gewohnt auf. Die Aufbewahrungsvorschau kennzeichnet so ein Element als behalten. Bei einem ZFS-Element behält nur das Dataset, das die Anomalie nennt, seine alten Backups; die übrigen Datasets des Baums werden wie gewohnt aufgeräumt.

## Das manuelle Aufräumen meldet, dass einige Elemente behalten wurden

Dieselbe Ursache: Das Aufräumen lässt die alten Backups eines Elements mit so einer Anomalie in Ruhe und nennt das Element in seiner Meldung. Alles andere wird wie gewohnt aufgeräumt.

## Der Verlaufsimport meldet ein Repository, das nicht gelesen werden konnte

Nach dem Update liest BombVault einmal die Größen früherer Backups aus jedem Repository. Ein Repository, das zu diesem Zeitpunkt nicht erreichbar war, etwa ein ausgefallenes Off-site-Ziel oder eine nicht eingehängte Freigabe, steht in der Karte **Anomalien** unter **Einstellungen, Integrität** und wird einmal am Tag erneut versucht. Seine Elemente lernen in der Zwischenzeit aus neuen Backups.

## Die Speicherplatzwarnung passt nicht zum Unraid-Dashboard

Auf dem Unraid-User-Share (`/mnt/user`) ist der freie Platz der des ganzen Arrays, nicht der einer einzelnen Platte. Entfernte Repositories werden nur über rclone-Remotes gemessen, die ihren freien Platz melden; S3-, B2-, REST- und SFTP-Repositories haben keine Angabe und stehen in der Karte **Anomalien** als nicht gemessen.

## Ein KI-Assistent verbindet sich nicht

Die Seite [MCP-Server](mcp.md#troubleshooting) listet auf, was jeder Statuscode und jede Ablehnung des MCP-Endpunkts bedeutet und was du dagegen tun kannst.

## Der Container startet ständig neu oder wirkt ungesund

BombVault meldet gesund/ungesund aus seinem eigenen `/api/health`. Ein Auto-Heal-Werkzeug (wie Autoheal) kann ihn automatisch neu starten, falls sich die Engine je verklemmt. Prüfe das Container-Log und den `/spike`-Bericht auf die zugrunde liegende Ursache.

## Immer noch festgefahren?

- Lies die vollständigen Seiten [Konfiguration](configuration.md) und [Off-site & Wiederherstellung](offsite-recovery.md).
- Frage im [Unraid-Support-Thread](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Öffne ein [GitHub-Issue](https://github.com/junkerderprovinz/bombvault/issues).
