# Fehlersuche

Eine kurze FAQ. Die vollständige Tabelle zur Fehlersuche auf Host-Seite bei VM über SSH (permission denied, Host-Key-Prüfung, fehlende Template-Variablen und mehr) steht in der [Anleitung zur VM-Sicherung über SSH](vm-backup-ssh-setup.md).

## Etwas ist nicht richtig verkabelt {#something-is-not-wired-up-correctly}

Füge die Karte **Host-Integration** der Übersicht unter **Anpassen** hinzu. Der Host-Integrations-Check prüft jeden Mount und jedes CLI (Docker-Socket, libvirt, restic, qemu-img, rclone) und meldet fehlende Teile, und **Jetzt prüfen** führt ihn erneut aus. Fang hier an, bevor du einen Fehler annimmst: Ein fehlender Mount oder ein nicht erreichbarer Host zeigt sich sofort.

## Ich erreiche die Web-UI nicht {#i-cannot-reach-the-web-ui}

BombVault liefert ab Werk HTTPS auf Port `3443` (selbstsigniertes Zertifikat), öffne also `https://<deine-unraid-ip>:3443`. Akzeptiere die Warnung zum selbstsignierten Zertifikat, oder setze BombVault hinter einen Reverse-Proxy mit deinem eigenen Zertifikat. Läuft es mit `HTTP_ONLY=true`, liefert es stattdessen einfaches HTTP auf Port `3000` (gedacht für den Einsatz hinter einem Proxy, der TLS beendet).

## Ich habe meinen APP_KEY verloren {#i-lost-my-app_key}

`APP_KEY` leitet das Passwort des restic-Repositorys ab. Ohne ihn, ohne das Recovery-Kit und ohne eine Schlüsselhinterlegung in deiner Gruppe lassen sich verschlüsselte Sicherungen nicht wiederherstellen. Deshalb zeigt die Übersicht **Sichere dein Recovery-Kit**, bis du es heruntergeladen hast. Siehe [Off-site und Wiederherstellung](offsite-recovery.md#recovery-kit). Erzeuge einen Schlüssel mit `openssl rand -hex 32` und bewahre ihn außerhalb des Servers auf, bevor du dich auf irgendeine Sicherung verlässt.

## Ich habe das Login-Passwort vergessen {#i-forgot-the-login-password}

Wähle auf der Login-Seite **Passwort vergessen?**. BombVault schreibt einen Code mit 12 Zeichen in sein Container-Protokoll (auf Unraid: Docker-Tab, BombVault-Symbol, **Logs**). Tippe ihn innerhalb von 10 Minuten mit einem neuen Passwort ein. Das Zurücksetzen schaltet außerdem die Zwei-Faktor-Authentifizierung aus, entfernt jeden Passkey, entzieht jedes API-Token, jeden MCP-Schlüssel und jede OAuth-Anmeldung, beendet jede andere Sitzung und schaltet die Handy-Kopplung aus. Füge deine Passkeys unter **Einstellungen, Sicherheit** wieder hinzu, schalte dort die Handy-Kopplung wieder ein und lege neue Tokens und Schlüssel an, wo du sie noch brauchst. Siehe [Sicherheitsmodell](configuration.md#login).

## Sicherungen stehen still und sagen, der Schlüssel lasse sich nicht mehr öffnen {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

Dieser Server hat im Notfall-Assistenten den Sicherungsschlüssel eines anderen Servers übernommen, und `APP_KEY` wurde danach geändert. Der übernommene Schlüssel ist mit dem alten APP_KEY versiegelt, BombVault kann ihn also nicht öffnen und sichert nichts, statt mit dem falschen Schlüssel zu schreiben. **Braucht dich** in der Übersicht nennt es. Setze APP_KEY auf den alten Wert zurück oder lade das Recovery-Kit auf der Seite Wiederherstellung unter **Notfall** erneut hoch.

## Ein Handy landet auf der Login-Seite {#a-phone-lands-on-the-login-page}

Ein gekoppeltes Handy bekommt seine Sitzung über die Gruppe nur, solange **Handys können sich mit den zwölf Wörtern der Gruppe koppeln** unter **Einstellungen, Sicherheit** an ist. Ein Passwort-Reset schaltet es aus. Eine App, die älter ist als der Server, bekommt ebenfalls keine Sitzung, bis sie aktualisiert ist. Ein über seine Adresse hinzugefügter Server fragt immer nach dem Passwort. Ein Handy, das von der Seite Instanzen entfernt wurde, kommt ausgeschaltet zurück, wenn seine App wieder öffnet. Schalte **Diese Instanz abfragen** unter **Überwachung** in seinen Details ein, um es hereinzulassen.

## Die VM-Sicherung verbindet sich nicht {#vm-backup-will-not-connect}

Die VM-Sicherung spricht über SSH mit libvirt, nie über einen Mount.

- Prüfe, dass SSH auf dem Host aktiviert ist und der öffentliche Schlüssel von BombVault in `/root/.ssh/authorized_keys` freigegeben ist (Einstellungen, Anbindungen, Host-SSH zeigt den Schlüssel und einen Knopf **Verbindung testen**).
- Setze in einem eigenen Netz `br0.x` `LIBVIRT_HOST` auf die LAN-IP deines Unraid (der Container erreicht den Host dort nicht über `host.docker.internal`). Aktiviere **Settings, Docker, Host access to custom networks**.
- Hast du den SSH-Port von Unraid geändert, setze `LIBVIRT_SSH_PORT` passend.
- Setze auf TrueNAS `LIBVIRT_URI` mit dem eigenen libvirt-Socket von TrueNAS, gib der App die Capability `SETPCAP` und füge den Schlüssel dem Benutzer root unter **Credentials, Users** hinzu (siehe [TrueNAS SCALE](getting-started.md#truenas)).
- Die vollständige Schritt-für-Schritt-Diagnose (Erreichbarkeitstest, VLAN-Routing, `Permission denied (publickey)`, `Host key verification failed`) steht in der [Anleitung zur VM-Sicherung über SSH](vm-backup-ssh-setup.md).

## Ein Live-VM-Snapshot lief nicht {#a-live-vm-snapshot-did-not-run}

Live-Snapshots brauchen die Disk auf `/mnt/cache` (oder `/mnt/diskX`), nicht auf `/mnt/user`. Mit dem qemu-Gast-Agenten in der VM ist der Snapshot stillgelegt, ohne ihn ist er absturzkonsistent. Bei einer ausgeschalteten VM fällt Live automatisch auf Graceful zurück. Eine Graceful-Sicherung fährt die VM herunter, sichert die Disks und startet sie dann neu, ist also immer konsistent. Sie wartet bis zu zehn Minuten, bis der Gast heruntergefahren ist, und erzwingt das Ausschalten nie, damit ein Windows-Gast, der Updates installiert, nicht auf halbem Weg abgeschnitten wird. Ein Gast, der danach noch läuft, bleibt unberührt, und diese Sicherung scheitert mit dem Grund.

## Eine Sicherung scheiterte mit „repository is already locked“ {#a-backup-failed-with-repository-is-already-locked}

Das ist meist eine verwaiste restic-Sperre, die zurückblieb, als der Container mitten in einem Vorgang aktualisiert oder neu gestartet wurde. BombVault erkennt eine nachweislich verwaiste Sperre, hebt sie auf und versucht es automatisch einmal erneut. Nach einem Neustart wartet es, bis eine solche Sperre zehn Minuten lang nicht aufgefrischt wurde. Ein restic, das noch läuft, frischt seine Sperre alle fünf Minuten auf, und BombVault entfernt nie eine Sperre, die noch aufgefrischt wird, weil sie einem zweiten BombVault gehören kann, das in dasselbe Repository schreibt. Eine Sperre, die 30 Minuten lang nicht aufgefrischt wurde, wird entfernt, egal wer sie gesetzt hat. Bleibt der Fehler bestehen, öffne die Karte **Sicherungen prüfen** unter **Einstellungen, Prüfung** und wähle **Entsperren** unter **Mehr** in der Zeile des betroffenen Bereichs, um eine veraltete Sperre von Hand aufzuheben. Auch **Entsperren** lässt eine Sperre stehen, die noch aufgefrischt wird, und sagt dir, wann das zuletzt geschah. Ein echtes Problem taucht trotzdem auf, statt versteckt zu werden.

## Meine Off-site-Kopie ist nach einer Sicherung nicht passiert {#my-off-site-copy-did-not-happen-after-a-backup}

Die Off-site-Replikation ist absichtlich nach bestem Bemühen angelegt, ein Off-site-Schluckauf lässt die lokale Sicherung also nie scheitern. Prüfe, wann der Speicherort kopiert: Seine Karte **Kopieren** sagt **Nach jeder Sicherung** oder nennt einen eigenen Plan, und **Einstellungen, Zeitplan, Off-site-Kopien pro Bereich** kann einem Bereich einen eigenen Plan geben. Nutze **Jetzt replizieren** in der Zeile des Bereichs in der Karte **Off-site-Kopien** unter **Einstellungen, Speicherorte** für einen Lauf auf Abruf, und beobachte den Statuskopf der Übersicht, der **Off-site-Kopie läuft** sagt, solange sie läuft. Eine fehlgeschlagene Kopie erscheint unter **Braucht dich**.

## Dem Zertifikat eines rest-servers wird nicht vertraut {#a-rest-servers-certificate-is-not-trusted}

restic lehnt ein Zertifikat ab, das es nicht kennt, und der Fehler sagt, wohin es gehört. Kopiere das selbstsignierte Zertifikat des rest-servers oder das der Stelle, die es signiert hat, als PEM-Datei in den Ordner `certs` im Datenordner von BombVault (`/config/certs` im Container, `appdata/bombvault/certs` auf Unraid) und teste erneut. Ein Neustart ist nicht nötig.

## Eine Sicherung sagt, Daten außerhalb von Host Data seien nicht gesichert worden {#data-outside-host-data}

Der Container hält Daten in einem Ordner, den BombVault über Host Data nicht lesen kann. Die Sicherung speichert die Definition und jeden Ordner, den sie erreicht, färbt ihre Zeile im Aktivitätsprotokoll orange und nennt die Ordner, die sie ausgelassen hat. Zwei Fälle sind häufig: ein ixVolume auf TrueNAS unter `/mnt/.ix-apps` und ein benanntes Docker-Volume unter `/var/lib/docker/volumes`, etwa das Datenbank-Volume eines Compose-Stacks. Die Karte **Gesicherte Ordner** auf dem Tab **Einstellungen** des Containers listet jeden solchen Ordner mit seinem Pfad im Container und seinem Host-Pfad auf, rot markiert.

Ein anonymes Docker-Volume wird nicht gemeldet. Docker legt eines von selbst für einen Ordner an, den das Image deklariert und den niemand zugeordnet hat, benennt es mit 64 Hex-Ziffern und hält es unter `/var/lib/docker/volumes`. Es enthält meist einen Cache, daher bleibt die Sicherung grün, und die Karte listet es nur mit dem Hinweis auf, dass es anonym ist.

- **Du willst, dass diese Daten gesichert werden.** Ordne den Ordner einem Host-Pfad unterhalb von Host Data zu, zum Beispiel einem Ordner in deinem Appdata-Share, und die nächste Sicherung schließt ihn ein.
- **Du brauchst sie nicht.** Füge einen seiner beiden Pfade als Zeile in der Karte **Ausschlussmuster** auf demselben Tab hinzu: den Pfad im Container, etwa `/cache`, oder den Host-Pfad, den die Warnung nennt, etwa `/var/lib/docker/volumes/jellyfin_cache/_data`. Ab der nächsten Sicherung lässt die Warnung diesen Ordner weg, und die Vorschau unter dem Feld sagt warum. Eine Zeile für einen Ordner darüber funktioniert auch, ein bloßer Name oder `/` nicht.

## Eine Sicherung sagt Zurückgestellt {#deferred-backups}

Die App des Containers war beschäftigt, ihre Sicherung wartet also und wird von selbst wieder versucht. Die Zeile im Aktivitätsprotokoll sagt warum und wann der nächste Versuch ist. Die Karte **Wenn die App beschäftigt ist** auf dem Tab **Einstellungen** des Containers sagt dasselbe und wann die Wartezeit endet. Du musst nichts tun. Um sofort zu sichern, nutze **Jetzt sichern**: Eine Sicherung, die du selbst startest, wartet nie, auch nicht bei einem Exit-Code von 75.

- **Deine eigene Prüfung.** Lass den **Befehl vor der Sicherung** mit Code 75 enden, solange die App beschäftigt ist, zum Beispiel `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Jeder andere Code außer 0 lässt die Sicherung wie bisher scheitern, ein kaputtes Skript taucht also weiter unter **Braucht dich** auf.
- **Plex, Jellyfin oder Emby wird nie gefragt.** BombVault fragt zuerst den Server unter der eigenen Adresse des Containers und führt, wenn der nicht antwortet, curl oder wget im Container aus. Ein Image mit keinem von beiden lässt sich nicht über sich selbst fragen. Das Protokoll nennt das dann. Bei eingeschaltetem **Sichern, wenn der Server nicht antwortet** läuft so eine Sicherung trotzdem, bei ausgeschaltetem wartet sie.
- **Jellyfin oder Emby lehnt ab.** Prüfe den **API-Schlüssel**. Er wird im Dashboard des Servers unter API-Schlüssel erstellt.
- **Plex lehnt ab.** BombVault liest das Token aus `Preferences.xml` in dem Ordner, der unter `/config` gemountet ist. Ein Server, an dem nie jemand angemeldet war, hat keines und antwortet nur dort, wo er Zugriff ohne eines erlaubt.
- **Ein pausierter Film hält die Sicherung die ganze Nacht warten.** Setze **Pausierte Sitzungen ignorieren nach (Min.)**, zum Beispiel auf 30.
- **Die Verkehrsprüfung lässt die Sicherung nie durch.** Erhöhe **Beschäftigt ab (KiB/s)** über das, was der Container sendet, wenn ihn niemand nutzt. Direkt nach dem Start von BombVault wartet die Prüfung, bis sie den Container **Ruhig für (Min.)** Minuten lang beobachtet hat.
- **Die Sicherung läuft nie nachts.** Ist ein Fenster gesetzt, finden Versuche nur darin statt. Eine Wartezeit, die ihre Frist außerhalb des Fensters erreicht, endet trotzdem an der Frist.

## Eine große Sicherung wurde wegen Speichermangels beendet {#a-large-backup-was-killed-for-running-out-of-memory}

Setze **Speicherlimit pro Sicherung (MiB, 0 = keins)** in der Karte **Leistung** unter **Einstellungen, System** deutlich unter das, was der Container nutzen darf. Nahe am Limit gibt restic Speicher öfter frei, was etwas Tempo kostet, die Sicherung wird also langsamer, statt beendet zu werden.

## Eine Wiederherstellung brach ab, bevor sie begann {#a-restore-aborted-before-it-started}

Bevor etwas angehalten oder entfernt wird, führt die Wiederherstellung eine Konfliktprüfung vorab aus: Sie prüft, ob die statische IP und die veröffentlichten Host-Ports des Containers frei sind. Hält einen schon ein anderer Container, bricht sie mit einer klaren, umsetzbaren Meldung ab, statt eine halb fertige Wiederherstellung zu hinterlassen. Gib den kollidierenden Port oder die IP frei und versuche es erneut.

Existiert das Netzwerk des Containers nicht, zum Beispiel weil seine TrueNAS-App gelöscht wurde, nennt die Wiederherstellungsprüfung das Netzwerk, und die Wiederherstellung lehnt ab, bevor sie etwas ändert. Installiere die App in TrueNAS unter demselben Namen neu, was das Netzwerk wieder anlegt, und stelle dann ihre Daten wieder her.

## Ein einfacher Export scheiterte, statt eine Datei zu schreiben {#a-plain-export-failed-instead-of-writing-a-file}

Ist **Exporte mit age verschlüsseln** unter **Einstellungen, System** an, aber kein gültiger Empfänger gesetzt, scheitert ein Export mit einer klaren Fehlermeldung, statt Klartext zu schreiben. Füge einen gültigen Empfänger hinzu (einen öffentlichen age-Schlüssel oder einen öffentlichen SSH-Schlüssel) oder schalte die Verschlüsselung aus, wenn der Export Klartext sein soll. Siehe [Einfache Exporte](backups.md#exports).

## Ein Datenbank-Dump ist fehlgeschlagen {#a-database-dump-failed}

Ein fehlgeschlagener Dump lässt die Sicherung um ihn herum nie scheitern. Er wird als eigener fehlgeschlagener Lauf vermerkt, und der Grund nennt, was zu beheben ist.

- **Login abgelehnt.** Der Dump meldet sich mit den eigenen Passwortvariablen des Containers an (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` oder ihren `_FILE`-Versionen). Prüfe sie am Datenbank-Container. Eine `_FILE`-Variable, die auf ein Secret zeigt, das der eigene Benutzer des Containers nicht lesen kann, scheitert genauso.
- **Fehlende Rechte.** Bei einem zufälligen Root-Passwort kann der Dump sich nur als App-Benutzer anmelden und hält daher nur diese eine Datenbank, und MySQL 8.4 und neuer lehnt es unter Umständen ganz ab. Gib dem Container ein echtes Root-Passwort oder schalte den Dump dafür aus.
- **Die Systemtabellen brauchen ein Upgrade.** MariaDB lehnt den Dump ab, wenn seine Systemtabellen aus einer älteren Version stammen (Fehler 1558). Füge die Variable `MARIADB_AUTO_UPGRADE=1` hinzu und starte den Container neu, oder führe einmal `mariadb-upgrade` darin aus.
- **Kein Dump-Werkzeug.** Ein abgespecktes oder selbst gebautes Image ohne `pg_dump`, `mysqldump` oder `mariadb-dump` lässt sich nicht dumpen. Nimm das offizielle Image oder schalte den Dump aus.
- **Ein Zeitlimit.** Ein Dump bekommt `DB_DUMP_MAX_HOURS` (standardmäßig 6), die Sicherung um ihn herum bekommt `BACKUP_MAX_HOURS`, und ein Dump, der keinen Fortschritt mehr macht, wird nach `BACKUP_STALL_HOURS` abgeschnitten. Die übliche Ursache beim Letzten ist eine Sperre, die die Anwendung hält. Erhöhe das Limit, das gegriffen hat, oder dumpe, während die Anwendung ruhig ist.
- **Der Container ist pausiert oder startet neu.** Der Dump spricht mit dem laufenden Server. Startet der Container ständig neu, sagt sein eigenes Protokoll warum.
- **Ein beschädigter Dump ließ sich nicht entfernen.** Ein Dump, den BombVault nicht beenden konnte, wird wieder gelöscht. Schlägt dieses Löschen fehl, bleibt der Dump als beschädigt markiert in der Liste, und du kannst ihn dort löschen.

## Ein Import ist fehlgeschlagen {#an-import-failed}

Ein Import hält den Container an, schiebt seinen Datenordner beiseite und lässt das Image einen leeren an seiner Stelle anlegen. Schlägt ein Schritt vor dem Import selbst fehl, wird der alte Ordner automatisch zurückgelegt. Schlägt der Import fehl, behält der Container den frischen Ordner, und der alte bleibt daneben als `<data folder>.bombvault-before-import-<timestamp>` stehen. Die Fehlermeldung des Laufs nennt den genauen Pfad.

Ein Datenordner, der ein eigenes Dataset ist, wie auf TrueNAS, lässt sich nicht umbenennen. Der Import lässt den Ordner dann stehen und verschiebt seinen Inhalt in den behaltenen Ordner daneben, das darüberliegende Dataset braucht also Platz dafür. Der Import prüft das, bevor er den Container anhält.

Um es von Hand zurückzulegen: Halte den Container an, benenne den jetzigen Datenordner um, damit er aus dem Weg ist, benenne den behaltenen Ordner zurück in den ursprünglichen Namen und starte den Container. Bei einem Dataset verschiebe stattdessen den Inhalt, statt die Ordner umzubenennen. Auf Unraid erledigt das der Dateimanager im Tab Shares.

## Eine ZFS-Dataset-Sicherung scheiterte oder übersprang ein Dataset {#zfs-datasets}

Jedes Problem trägt einen Grundcode in Klammern, und die Seite [ZFS-Datasets](zfs-datasets.md#reason-codes) listet alle mit der Lösung auf. Die drei häufigsten:

- **`snapshot-loop`**: Der Snapshot erreichte BombVault nicht, weil Host Data neue Mounts nicht durchreicht. Bearbeite den Container, setze den Access Mode von Host Data auf Read/Write - Slave und starte BombVault neu.
- **`key-not-loaded`**: Ein verschlüsseltes Dataset, dessen Schlüssel nicht geladen ist, wird übersprungen. Lade den Schlüssel mit `zfs load-key` und mounte das Dataset. Die nächste Sicherung schließt es ein.
- **`ssh-auth`**: Der Server hat den Schlüssel von BombVault abgelehnt. **ZFS-Verbindung zu diesem Server**, geöffnet mit **Verbindung ansehen** auf der Seite Sicherungen, zeigt den Befehl, der ihn freigibt. Führe ihn einmal auf dem Server aus.

## Ein Eintrag bleibt bei „Learning N/10“ {#an-entry-stays-at-learning-n10}

Die meisten Anomalieprüfungen starten nach 10 erfolgreichen Sicherungen eines Eintrags, und die Zählung beginnt nach **Als erwartet markieren** und nach einer Änderung der Auswahl des Eintrags von vorn. Ein Eintrag, der nicht im Zeitplan ist, lernt nicht, und ein Container ohne Appdata hat nichts, woraus er lernen könnte, was sein Abzeichen sagt. **Lernfortschritt** auf der Seite Anomalien zeigt pro Eintrag einen Ring aus zehn Teilen.

## Die Aufbewahrung löscht keine alten Sicherungen eines Eintrags mehr {#retention-stopped-deleting-old-backups-of-one-entry}

Eine offene kritische Anomalie hält sie fest: Die Quelle des Eintrags ist fast leer, stark geschrumpft, oder eine Sicherung hat den Großteil ihrer Daten noch einmal abgelegt. Öffne die Anomalie über das Abzeichen am Eintrag. Fehlen Daten oder wurden sie verschlüsselt, stelle zuerst aus der verlinkten letzten guten Sicherung wieder her. Quittiere dann die Anomalie oder markiere sie als erwartet, wenn die Änderung von dir kam, und der nächste Lauf räumt wie gewohnt auf. Die Vorschau der Aufbewahrung markiert so einen Eintrag als behalten. Bei einem ZFS-Eintrag behält nur das in der Anomalie genannte Dataset seine alten Sicherungen, die anderen Datasets des Baums werden wie gewohnt aufgeräumt.

## Das manuelle Aufräumen sagt, manche Einträge seien behalten worden {#manual-prune-says-some-entries-were-kept}

Dieselbe Ursache: Das Aufräumen lässt die alten Sicherungen eines Eintrags mit so einer Anomalie in Ruhe und nennt den Eintrag in seiner Meldung. Alles andere wird wie gewohnt aufgeräumt.

## Eine Sicherung scheitert mit „on no mounted pool or share“ {#no-mounted-pool}

Auf Unraid liegt `/mnt` im Arbeitsspeicher, ein Ordner dort, auf dem kein Pool eingehängt ist, ist also beim nächsten Neustart weg und verdeckt, sobald der Pool wieder da ist. BombVault weigert sich, dort eine Sicherung oder ein Flash-ZIP zu schreiben, auch wenn in diesem Ordner schon ein Repository von einem früheren Schreiben liegt. Hänge den Pool ein und starte die Sicherung erneut.

## Der Verlaufsimport sagt, ein Repository ließ sich nicht lesen {#history-import-says-a-repository-could-not-be-read}

Nach dem Upgrade liest BombVault die Größen früherer Sicherungen einmal aus jedem Repository. Ein Repository, das es zu diesem Zeitpunkt nicht erreichen konnte, etwa ein Off-site-Ziel, das ausgefallen war, oder ein Share, der nicht gemountet war, wird in der Karte **Anomalien** unter **Einstellungen, Prüfung** gezählt und einmal am Tag erneut versucht. Seine Einträge lernen in der Zwischenzeit aus neuen Sicherungen.

## Die Warnung zum Plattenplatz passt nicht zum Unraid-Dashboard {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

Auf dem Unraid-User-Share (`/mnt/user`) ist der freie Platz der des ganzen Arrays, nicht einer Platte. Entfernte Speicherorte, die BombVault über rclone erreicht, etwa SFTP, die Hetzner Storage Box, SMB, WebDAV oder ein Cloud-Laufwerk, werden gemessen, wenn die Gegenseite ihren freien Platz meldet, und ebenso eine von Hand eingetippte `sftp:`-Adresse. S3-, B2- und REST-Repositories haben keinen Wert: Ihr Speicherort sagt **Größe unbekannt**, und die Karte **Anomalien** listet sie als nicht gemessen.

## Ein KI-Assistent kann sich nicht verbinden {#an-ai-assistant-cannot-connect}

Die Seite [MCP-Server](mcp.md#troubleshooting) listet auf, was jeder Statuscode und jede Ablehnung des MCP-Endpunkts bedeutet und was man dagegen tut.

## Der Container startet ständig neu oder wirkt ungesund {#the-container-keeps-restarting-or-looks-unhealthy}

BombVault meldet gesund/ungesund über sein eigenes `/api/health`. Ein Auto-Heal-Werkzeug (etwa Autoheal) kann es automatisch neu starten, falls die Engine je hängt. Sieh dir das Container-Protokoll und die Karte **Host-Integration** an, um die Ursache zu finden.

## Ein Speicherort lässt sich nicht speichern {#location-refused}

BombVault prüft einen Ort, bevor es ihn speichert, und die Ablehnung sagt in einem Satz warum. Die üblichen:

- **„This location lies inside another repository or target, or contains one.“** Ein Repository in einem Repository würde beide kaputt machen. Die Meldung nennt den anderen Ort. Wähle einen anderen Ordner oder entferne jenen zuerst.
- **Es ist ein Repository, das du eingerichtet hast.** Der Ort ist eine der Zeilen unter **Repositories** oder liegt in oder um eine. Wähle einen anderen Ort oder entferne dieses Repository zuerst.
- **Derselbe Ort wie der eigene Ordner eines Bereichs.** Eine Kopie auf den eigenen Ordner eines Bereichs würde bei jedem Lauf seine Sicherungen auf sich selbst kopieren und wird daher abgelehnt. Wähle einen anderen Ort oder siehe [Der eigene Ordner eines Bereichs](offsite-recovery.md#own-folder).
- **„That is not an off-site target of this domain“** kam von einem Link auf einen Speicherort, der entfernt wurde. Eine aktuelle Version räumt diese beim Start und bei einem Import auf. Siehst du es trotzdem, aktualisiere BombVault.
- **Ein vollständiger Host-Pfad** wie `/mnt/user/backups`. BombVault läuft in einem Container, es will also den Pfad unterhalb von Host Data, `user/backups`, und die Meldung nennt den, der einzugeben ist.

## Ein entfernter Speicherort sagt, sein Repository sei weg {#repository-gone}

Ein Ort, der Sicherungen hielt und plötzlich kein Repository mehr hat, bekommt nicht von selbst ein neues, leeres, denn dieses würde das echte verdecken, sobald eine Platte wieder da ist. Ein rest-server, dessen Platte nicht gemountet wurde, ist die übliche Ursache, mounte die Platte also wieder und führe die Sicherung oder Kopie noch einmal aus. Nur wenn du das alte Repository absichtlich geleert oder gelöscht hast, drücke auf der Seite des Ortes **Hier ein neues Repository anlegen**. Es beginnt für jeden Bereich dieses Ortes auf einmal von vorn.

## Eine Cloud-Anmeldung ist abgelaufen {#cloud-sign-in-expired}

Der Test oder die nächste Kopie sagt dann, dass die Anmeldung abgelaufen ist. **Verbindung ändern** auf der Seite des Ortes meldet neu an.

- **Box, Jottacloud und andere mit Einmal-Refresh-Tokens:** BombVault behält das Token, das rclone erneuert hat, auch über Neustarts hinweg und im Recovery-Kit. Nach einer Wiederherstellung aus einer älteren Einstellungssicherung kann das darin liegende Token abgelaufen sein, melde dich also noch einmal an. Das Repository bleibt, wie es ist.
- **iCloud Drive:** Apple beendet die Anmeldung nach 30 Tagen. Es braucht ein neues Trust-Token und Cookies aus `rclone config`.
- **OneDrive:** Eine Anmeldung mit mehreren Laufwerken fragt, welches genutzt werden soll, ein Ort ohne Wahl bleibt also unbrauchbar, bis du eines wählst. Zu OneDrive Business und Google Drive siehe den Hinweis zu bereinigten Daten unter [Speicherorte](offsite-recovery.md#destinations).

## „The off-site copy of the last backup is still running“ {#copy-still-running}

Die Sicherung davor ist fertig, und ihre Kopie ist noch unterwegs, was eine neue Sicherung dieses Bereichs blockiert. Warte, bis der Statuskopf nicht mehr **Off-site-Kopie läuft** sagt. Die Knöpfe, die eine Sicherung starten, sagen dasselbe, solange sie warten. Die erste Kopie an einen neuen Ort kann Stunden dauern, siehe das [S3-Beispiel](offsite-recovery.md#example-s3).

## Eine ZFS-Replik hält an, weil das Ziel neuere Snapshots hat {#replica-target-newer}

Das Ziel hält Snapshots, die neuer sind als der neueste, den es mit diesem Server teilt, zum Beispiel weil das Dataset hier zurückgerollt oder die Kopie auf dem Ziel verändert wurde. Die Karte Replik und der Laufverlauf nennen den Snapshot unter **Ziel zurückrollen auf**. Führe `zfs rollback -r` auf diesen Snapshot auf dem Ziel aus, für jedes Dataset, das der Lauf nennt, und drücke **Jetzt replizieren**. Siehe [Wie lange Snapshots bleiben](zfs-datasets.md#replica-retention).

## Eine VM mit Unraid-Snapshots scheitert und nennt eine Datei {#vm-chain}

BombVault folgt der Backing-Chain jeder Disk und sichert jede Datei darin. Eine Chain, der man nicht bis zum Ende folgen kann, lässt die Sicherung scheitern, statt einen Teil der Disk auszulassen, und die Meldung nennt die Datei. Prüfe, dass die Datei unterhalb von `/mnt` existiert und der Snapshot im VM Manager noch aufgelistet ist. Eine Datei außerhalb von Host Data lässt sich gar nicht lesen. Siehe [VMs mit Unraid-Snapshots](vm-backup-ssh-setup.md#unraid-snapshots).

## Off-site-Kopien warten nach einem Neuaufbau auf deine Entscheidung {#copies-paused}

Nach **Sicherungen entdecken** oder dem Notfall-Assistenten ohne wiederhergestellte Einstellungen, oder wenn ein Bereich eigene Sicherungen aus der Zeit vor dieser Datenbank findet, warten die Off-site-Kopien dieses Bereichs, weil BombVault nicht mehr weiß, welche Einträge du ausgelassen hast. **Braucht dich** und die Seite des Speicherorts fragen danach. Drücke **Als eigene dieses Servers übernehmen**, um weiterzumachen, oder **Erst die Regeln prüfen**, um dir vorher die Regeln des Bereichs anzusehen. Der Import einer Einstellungsdatei nimmt dir die Entscheidung nicht ab. Siehe [Nach einem Neuaufbau](offsite-recovery.md#after-a-rebuild).

## Benachrichtigungen kommen nicht an {#notifications-do-not-arrive}

Fang mit **Test senden** unter **Einstellungen, Benachrichtigungen** an. Die Zeile unter der Karte sagt, was passiert ist.

- **Es wird gar nichts gesendet.** Prüfe **Benachrichtigen**. Bei **Nie** geht keine Nachricht hinaus, nur die Pings einer Healthchecks-URL, und bei **Nur bei Fehler** bleibt ein guter Lauf still. Bei eingeschaltetem **Geplante Läufe zusammenfassen** sendet ein geplanter Lauf an seinem Ende eine Zusammenfassung.
- **Eine neue Adresse.** Nachdem du den Server von Matrix, Gotify oder E-Mail geändert hast, fragt das Feld wieder nach dem Token oder Passwort, und die neue Adresse wird erst gespeichert, wenn du es eintippst. BombVault sendet das alte Geheimnis nicht an einen neuen Ort.
- **Gotify** sagt, der Server habe das App-Token abgelehnt: Kopiere es noch einmal aus der Anwendung in Gotify. Die **Server-URL** ist die Basisadresse, ohne `/message`.
- **Webhook:** Prüfe, ob das **Webhook-Format** zum Dienst passt. Eine Discord-Adresse braucht **Discord**, und ntfy braucht **ntfy**.
- **Unraid-Benachrichtigungen** laufen über die Host-SSH-Verbindung, teste die also zuerst unter **Einstellungen, Anbindungen**. Auf einer anderen Plattform bleiben sie aus, und die Karte sagt das.
- **Eine Benachrichtigungsmail kommt nie an.** Prüfe **Verschlüsselung** gegen den Port, meist **STARTTLS** auf 587 oder **TLS (implicit)** auf 465.

Die Healthchecks-Pings hängen von alldem nicht ab, siehe [Healthchecks](notifications.md#healthchecks).

## Die Android-App kann einen Server nicht öffnen {#android-cannot-open}

Der Startbildschirm sagt auf der Karte des Servers warum:

- **Nicht verbunden:** Weder die Adresse des Servers noch das Relay antwortet. Bei **Kein Relay** erreicht die App einen Server nur unter seiner eigenen Adresse.
- **Zertifikat prüfen:** Der Server zeigt ein anderes Zertifikat als das, dem du vertraut hast. Die App öffnet ihn erst, nachdem du dem neuen vertraust, stelle also sicher, dass die Änderung von dir kam.
- **Anmeldung nötig:** Ein über seine Adresse hinzugefügter Server fragt nach dem Passwort. Siehe auch [Ein Handy landet auf der Login-Seite](#a-phone-lands-on-the-login-page).
- **Der Schlüssel hat sich geändert:** Der Startbildschirm zeigt den Fingerabdruck des neuen Schlüssels, und die Seite öffnet sich erst, nachdem du ihm vertraust. Siehe [Unterwegs](android.md#away).

## Ein anderer Benutzer kann das lokale Repository nicht lesen {#repository-permissions}

Das lokale Repository bleibt für andere Benutzer lesbar, etwa einen Sync-Benutzer über SMB. Vor 10.4.1 konnten Dateien, die ein Aufräumen, Tag, Pull, Bereichsumzug, Löschen oder Stack-Sicherung geschrieben hatte, bis zur nächsten Nacht unlesbar bleiben. Aktualisiere BombVault, dann sind sie sofort lesbar. Das Selbstbackup-Repository bleibt absichtlich nur für root lesbar, weil es die Einstellungen von BombVault enthält.

## Immer noch nicht weiter? {#still-stuck}

- Lies die vollständigen Seiten [Konfiguration](configuration.md) und [Off-site und Wiederherstellung](offsite-recovery.md).
- **Fehlerbericht** in der Karte **Hilfe** unter **Einstellungen, Info** lädt eine ZIP für ein GitHub-Issue herunter: den Host-Check, deine Konfiguration ohne jedes Geheimnis, die letzten Läufe, was als Nächstes geplant ist, und das jüngste Protokoll seit dem letzten Start des Containers. Sie enthält auch den letzten Dump jeder Datenbank, die ZFS-Einträge mit ihren Mounts, die offenen Anomalien und wie viele MCP-Schlüssel es gibt. Passwörter, Tokens und die rclone-Konfiguration werden entfernt, und das Paket sagt das in seinem eigenen Manifest. Sie braucht ein Login-Passwort. Bei einem Absturz, der den Container neu gestartet hat, sieh dir `docker logs` an.
- Frag im [Unraid-Supportthread](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Eröffne ein [GitHub-Issue](https://github.com/junkerderprovinz/bombvault/issues).
