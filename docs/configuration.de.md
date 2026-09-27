# Konfiguration

Diese Seite behandelt die Umgebungsvariablen des Containers, die vom Template bereitgestellten Mounts, das VM-Backup über SSH und die Off-site-Einrichtung. Wohin die Backups gehen, stellst du in der App unter **Einstellungen, Speicher** ein, nicht über Umgebungsvariablen.

## Umgebungsvariablen

| Variable | Erforderlich | Beschreibung |
|---|---|---|
| `APP_KEY` | **Ja** | 32-Byte-Hex-Geheimnis (64 Hex-Zeichen) zum Ableiten des restic-Repo-Passworts. Erzeuge es mit `openssl rand -hex 32`. Bewahre es sicher auf: geht es verloren, sind verschlüsselte Backups unwiederbringlich. |
| `LIBVIRT_HOST` | Für VMs | Über SSH erreichter Unraid-Host für das VM-Backup (Standard `host.docker.internal`; das Template füllt einen LAN-IP-Platzhalter vor). Nutze deine Unraid-LAN-IP, erforderlich in einem benutzerdefinierten `br0.x`-Netzwerk. |
| `LIBVIRT_SSH_PORT` | Nein | Host-SSH-Port für das VM-Backup (Standard `22`). |
| `LIBVIRT_SSH_USER` | Nein | SSH-Benutzer auf dem Host für das VM-Backup (Standard `root`). |
| `LIBVIRT_URI` | Nein | Vollständige libvirt-Verbindungs-URI, wird **wortwörtlich** verwendet statt sie aus den drei obigen `LIBVIRT_*`-Variablen zusammenzusetzen (die dann für den Verbindungsstring ignoriert werden). Standardmäßig nicht gesetzt. Wird auf TrueNAS Scale benötigt, dessen libvirtd auf einem nicht standardmäßigen Socket lauscht, den die zusammengesetzte Form nicht abbilden kann: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Siehe den TrueNAS-Scale-Abschnitt in [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). |
| `PORT` | Nein | HTTP-Port (Standard `3000`; nur mit `HTTP_ONLY=true` verwendet). |
| `HTTPS_PORT` | Nein | HTTPS-Port (Standard `3443`; das Template veröffentlicht ihn 1:1, sodass die WebUI unter `https://<ip>:3443` antwortet). |
| `HTTP_ONLY` | Nein | Setze `true`, um den selbstsignierten HTTPS-Listener zu deaktivieren und nur schlichtes HTTP auszuliefern (zur Verwendung hinter einem TLS-terminierenden Reverse Proxy). |
| `TRUSTED_PROXY` | Nein | Kommagetrennte Adressen oder CIDR-Bereiche des Reverse Proxys vor BombVault (zum Beispiel `192.168.20.11` oder `10.0.0.0/8`). Nur von diesen Stationen wird `X-Forwarded-For` geglaubt, und die Anmeldebremse zählt Fehlversuche dann pro echtem Client, statt alle hinter dem Proxy in einen Topf zu werfen. Nicht gesetzt (Standard) heißt: niemandem vertrauen. Ein bedingungslos geglaubter Header ließe jeden Aufrufer seinen eigenen Topf wählen. |
| `HOST_SOURCE_ROOT` | Nein | Der als **Host Data** eingehängte Host-Pfad (Standard `/mnt`). BombVault übersetzt die von Docker gemeldeten Bind-Mount-Quellen in Pfade unter diesem Mount. Nur ändern, wenn du ein anderes Host-Wurzelverzeichnis eingehängt hast. |
| `DATA_ROOT_SEGMENTS` | Nein | Kommagetrennte Pfadsegment-Namen, die eine Bind-Mount-Quelle als Backup-Daten kennzeichnen (Standard `appdata`, passend zu Unraids Konvention `/mnt/user/appdata/<container>`). Der Bind-Mount eines Containers wird für das Backup automatisch ausgewählt, wenn ein BELIEBIGES gelistetes Segment als vollständiges Pfadsegment in seiner Host-Quelle erscheint, zum Beispiel bezieht `DATA_ROOT_SEGMENTS=appdata,config` auch einen `.../config`-Bind mit ein. Siehe [Backup-Quellenerkennung](#backup-source-detection) für die weiteren, immer aktiven Wege, wie der Datenordner eines Containers gefunden wird. |
| `PLATFORM` | Nein | Erzwingt, als welche Plattform BombVault sich selbst betrachtet, statt sie automatisch zu erkennen: `unraid`, `generic` oder `truenas` (Standard nicht gesetzt: erkennt Unraid automatisch, indem nach dessen `dockerMan`-Marker unter dem Flash-Mount gesucht wird, ansonsten `generic`; ein nicht erkannter Wert fällt ebenfalls auf `generic` zurück, was protokolliert wird). Setze sie explizit auf einem generischen Docker-Host oder TrueNAS Scale, statt dich auf die reine Unraid-Auto-Erkennung zu verlassen; das macht die generische Compose-Datei bereits so. Ändert die appdata-Fallback-Konvention, die Standard-Wiederherstellungsziele bei instanzübergreifenden Wiederherstellungen und ob die nur für Unraid vorgesehenen Benachrichtigungs- und Begleit-Plugin-Schritte überhaupt versucht werden (siehe `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Nein | Der Name des BombVault-Containers selbst, sodass er sich nie selbst sichert (und damit stoppt). |
| `BACKUP_MAX_HOURS` | Nein | Maximale Echtzeit-Stunden, die ein einzelner Backup-Lauf seinen Bereichs-Lock halten darf, bevor er zwangsweise abgebrochen wird (ein Schutz, damit ein verklemmter Lauf den Bereich nicht für immer blockieren kann). Leer (der Standard) verwendet `48`. Erhöhe es für sehr große oder langsame Cloud-Backups (ein am Limit abgebrochener Lauf schlägt mit `context deadline exceeded` fehl). Setze `0`, um das Limit ganz zu deaktivieren. |
| `TZ` | Nein | Zeitzone für den Planer (zum Beispiel `Europe/Berlin`). **Nicht gesetzt bedeutet, dass alle Zeitpläne in UTC laufen**: ein Plan mit 02:30 startet dann um 02:30 UTC und nicht nach der lokalen Uhrzeit. Auf Unraid setzt du das nie selbst: Das System gibt seine eigene Zeitzone an jeden Container weiter. |

## Mounts

Hänge den Docker-Socket, den Flash (`/boot`) und das Wurzelverzeichnis **Host Data** (`/mnt`) ein, wie im CA-Template gezeigt. Backup-*Quellen* und -*Ziele* liegen beide unter Host Data, und es ist **slave** eingehängt, sodass eine Remote-Freigabe, die nach dem Containerstart eingehängt wird (zum Beispiel unter `/mnt/remotes`), ohne Neustart sichtbar wird.

Eine frische Installation speichert jeden Bereich am Ort **Unraid**, unter `/mnt/user/bombvault` mit einem Ordner pro Bereich (`container`, `vms`, `flash`, `config`, `files`), angelegt beim ersten Backup. Weitere Orte, lokal oder entfernt, fügst du unter **Einstellungen, Speicher** hinzu; siehe [Speicherorte](storage-places.md).

!!! note "Prüfung der Host-Integration"
    Öffne `/spike` in der Web-Oberfläche, nachdem der Container gestartet ist. Es prüft jeden Mount und jedes CLI (Docker-Socket, libvirt, restic, qemu-img, rclone) und meldet fehlende Teile.

## Erkennung der Sicherungsquellen {#backup-source-detection}

Für jeden Container wählt BombVault selbst aus, welche Bind-Mounts und benannten Volumes gesichert werden. Ein Pfad wird übernommen, sobald einer der folgenden Punkte zutrifft (das Ergebnis lässt sich pro Container jederzeit unter **Sicherungspfade** überschreiben):

- **Treffer auf ein Datenwurzel-Segment:** die Host-Quelle des Binds enthält eines der Segmente aus `DATA_ROOT_SEGMENTS` als vollständige Pfadkomponente (voreingestellt nur `appdata`).
- **Benannte Docker-Volumes** werden immer eingeschlossen, denn zu ihnen gibt es kein wegwerfbares Gegenstück und damit nichts zu filtern, **aber nur, wenn der echte Host-Speicherpfad des Volumes selbst über den Host-Data-Mount erreichbar ist**, genau wie jeder andere Host-Pfad, den BombVault sichert. Der Standardtreiber für lokale Volumes legt ein Volume unterhalb der Datenwurzel des Docker-Daemons ab, also `/var/lib/docker/volumes/<name>/_data`, sofern das nicht angepasst wurde (nachsehen mit `docker info -f '{{.DockerRootDir}}'`). Dieser Ort liegt NICHT im schmalen Ein-Verzeichnis-Host-Data-Mount, den die generische `docker-compose.yml` standardmäßig verwendet. Ein nicht erreichbares Volume wird stillschweigend übersprungen und nicht als Fehler gemeldet. Damit benannte Volumes auf einem generischen Host wirklich gesichert werden, richte Host Data (und `HOST_SOURCE_ROOT`) auf einen gemeinsamen übergeordneten Ordner, der auch die Docker-Datenwurzel abdeckt. Die Abwägung dazu steht im Host-Data-Kommentar der Compose-Datei (Unraid umgeht das, indem es aus demselben Grund gleich ganz `/mnt` einhängt, seine eigene allgemeingültige Konvention auf oberster Ebene).
- **Projektverzeichnis von Docker Compose:** trägt der Container das übliche Label `com.docker.compose.project.working_dir` (von `docker compose up` automatisch gesetzt), kommt dieses Verzeichnis ebenfalls dazu, unabhängig davon, ob irgendein Bind auf ein Datenwurzel-Segment gepasst hat.
- **Label-Übersteuerung `bombvault.data`:** setze am Container das Label `bombvault.data=true`, um ALLE seine Bind-Mounts einzuschließen, für ein Layout, das keine der beiden Konventionen oben erfasst (etwa ein einzelner Bind `/srv/plex/config` ohne Compose-Projekt). Jeder nicht leere Wert außer `false` gilt als wahr; ein fehlendes Label oder `bombvault.data=false` ändert nichts.

## Sicherheitsmodell

!!! warning "Root-äquivalente Kontrolle über den Host"
    Über den Docker-Socket kann BombVault Container stoppen, entfernen und neu erstellen sowie Appdata lesen/schreiben, und für das VM-Backup meldet es sich über SSH am Host an (`qemu+ssh://`, standardmäßig root), um `virsh` auszuführen. Wer die Web-Oberfläche erreichen kann, hat praktisch Root-Rechte auf dem Host.

- **Optionaler Passwortschutz** (Einstellungen, Sicherheit): setze ein Passwort, um Login zu verlangen, lösche es, um zu deaktivieren. Standardmäßig aus für die Nutzung im vertrauenswürdigen LAN. Das Passwort wird mit Argon2id über einen mit `APP_KEY` gepfefferten Wert gespeichert, ein kopiertes `/config` ist ohne den Schlüssel also wertlos und mit ihm langsam anzugreifen. Ein neues Passwort braucht mindestens 12 Zeichen; ein vorhandenes kürzeres funktioniert weiter, bis es geändert wird. Sitzungen sind signiert (HMAC abgeleitet aus `APP_KEY`), und eine Passwortänderung macht sie ungültig; Anmeldungen sind auf fünf Fehlversuche pro Minute und Client begrenzt.
- **Zwei-Faktor-Anmeldung** (Einstellungen): ein Zeitcode aus einer Authenticator-App zusätzlich zum Passwort, dazu acht einmalige Notfallcodes, die beim Einschalten einmal ausgegeben werden. Das gemeinsame Geheimnis liegt mit `APP_KEY` verschlüsselt, und das Ausschalten verlangt einen aktuellen Code.
- Weil die Sperre Opt-in ist, sind bei nicht gesetztem Passwort die gesamte UI und API (einschließlich der Off-site-Einrichtung, der Manipulationstest-Routen und des Recovery-Kits) für jeden erreichbar, der den Port erreichen kann. Aktiviere die Sperre, sobald Off-site-, unveränderliche Backups oder Verschlüsselung im Einsatz sind.
- Betreibe BombVault nur in einem vertrauenswürdigen, nicht exponierten Netzwerk. Für Fernzugriff setze es hinter einen Reverse Proxy, der Authentifizierung und TLS ergänzt. Antworten tragen grundlegende Sicherheits-Header (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Hinter einem Reverse Proxy trägt jede Anfrage die Adresse des Proxys, ohne `TRUSTED_PROXY` zählt die Anmeldebremse also alle Clients in einen Topf und die Fehlversuche eines Angreifers sperren auch dich aus. Trage den Proxy in `TRUSTED_PROXY` ein, dann zählt sie wieder pro Client.
- Mit `HTTP_ONLY=true` verliert das Session-Cookie sein `Secure`-Flag (das muss es, um über schlichtes HTTP zu funktionieren), aktiviere das Passwort also nur hinter einem TLS-terminierenden Proxy, wenn Vertraulichkeit wichtig ist.
- Die SSH-Verbindung für das VM-Backup vertraut dem Host-Key beim ersten Verbinden (TOFU) und pinnt ihn danach. Verifiziere den Host-Key außerhalb des Kanals, wenn dein Pfad vom Container zum Host nicht vertrauenswürdig ist.
- Backups werden von restic verschlüsselt, wenn die Verschlüsselung aktiviert ist (Einstellungen; standardmäßig an), mit dem aus `APP_KEY` abgeleiteten Schlüssel.

## VM-Backup über SSH

BombVault sichert KVM/libvirt-VMs, **ohne irgendeinen libvirt-Pfad einzuhängen**. Es führt `virsh` auf dem Host über SSH aus (`qemu+ssh://`), sodass es deinen Host-VM-Manager niemals beeinträchtigen kann.

Schnelleinrichtung:

1. **Einstellungen, System, VM-Backup über SSH:** kopiere den angezeigten öffentlichen Schlüssel.
2. Hänge ihn an Unraids `/root/.ssh/authorized_keys` an (auch auf dem Flash gespeichert, damit er Neustarts überdauert).
3. Klicke auf **Verbindung testen**.

Das Template fügt `--add-host=host.docker.internal:host-gateway` hinzu, damit der Container den Host erreichen kann. Setze `LIBVIRT_HOST` auf deine Unraid-LAN-IP, falls dieser Name nicht auflöst (zum Beispiel, wenn der Container in einem benutzerdefinierten `br0.x`-Netzwerk läuft). Wenn du Unraids SSH-Port geändert hast, setze `LIBVIRT_SSH_PORT` passend. **Live-Snapshots** benötigen zusätzlich den qemu-Gast-Agenten in der VM und den Datenträger auf `/mnt/cache` (nicht `/mnt/user`).

!!! important "Vollständige Anleitung zu VM-Einrichtung und Netzwerk"
    Die komplette Schritt-für-Schritt-Anleitung (SSH-Aktivierung, persistente Schlüsselautorisierung, Routing für benutzerdefinierte Netzwerke und VLANs, Methode pro VM und host-seitige Fehlerbehebung) findest du unter [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) auf GitHub.

## Off-site-Einrichtung

Off-site-Kopien gehen an Speicherorte. Leg den Ort unter **Einstellungen, Speicher** mit **Ort hinzufügen** an und hak ihn dann in der Zeile des Bereichs unter **Kopiert nach** an. [Speicherorte](storage-places.md) beschreibt jede Verbindungsart, und [Off-site & Wiederherstellung](offsite-recovery.md) behandelt Append-only, den Manipulationstest und DR-Übungen. Kurz gefasst:

- **Verbindungsarten:** ein Ordner auf diesem Unraid oder eine unter `/mnt/remotes` eingehängte NAS-Freigabe, S3-Speicher (Backblaze B2 und die anderen Cloud-Anbieter oder ein selbst betriebener Dienst wie MinIO oder Garage), rest-server, SFTP einschließlich Hetzner Storage Box, WebDAV für Nextcloud, ownCloud und OpenCloud, Azure Blob und jedes rclone-Remote. Backblaze B2 braucht nur den Schlüssel: BombVault liest den Bucket und den S3-Endpunkt daraus ab.
- **Zugangsdaten** werden verschlüsselt bei dem Ort gespeichert, zu dem sie gehören. Die Zugangsdaten-Sätze, die Abhol-Quellen benutzen, liegen im Tab **Holen** der Seite Instanzen.
- **SSH-Ziele brauchen auf der Gegenseite nichts installiert.** Ein SFTP-Ort braucht nur einen SSH-Server. Füge den öffentlichen Schlüssel, den das SFTP-Formular zeigt (auch unter **Einstellungen, System, VM-Backup über SSH** und in `/config/ssh/id_ed25519.pub`), den `~/.ssh/authorized_keys` des Zielbenutzers hinzu.
- **Off-site-Kopie:** BombVault kopiert neue Snapshots mit `restic copy` auf Best-Effort-Basis, zusätzlich zu dem Ort, an dem ein Bereich gespeichert ist. Jeder Bereich hat seinen eigenen Kopier-Zeitplan unter Einstellungen, Zeitpläne, dazu **Jetzt kopieren** in seiner Zeile.
- **Mehrere Kopie-Orte pro Bereich:** hak unter **Kopiert nach** so viele Orte an, wie du willst; jeder kopiert nach dem Zeitplan des Bereichs.
- **Aufbewahrung, Grenzen, Speicherklasse und Wachstumsbudget gehören zum Ort** und werden in seinen Details eingestellt. Die Aufbewahrung eines Ortes gilt für jedes Repository an ihm, so kann ein Off-site-Ort Kopien länger als Archiv behalten; ein Ort, bei dem jede Regel auf null steht, kürzt nie.
- **Kalt- und Archiv-Speicherklasse (S3):** wähle für einen S3-Ort eine wiederherstellungslesbare Stufe (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). rclone-Remotes setzen ihre Klasse in der rclone-Konfiguration.
- **Ein Bereich an einem entfernten Ort:** siehe [Ein Bereich an einem entfernten Ort](offsite-recovery.md#remote-primary-repositories).

## Portable Einstellungen (Export und Import) {#portable-settings-export-and-import}

Die Karte **Einstellungen exportieren und importieren** auf der Einstellungsseite schreibt deine gesamte BombVault-Konfiguration (Bereichseinstellungen, Speicherorte, Zeitpläne, Benachrichtigungen) in eine portable JSON-Datei, die du auf einer anderen Instanz importieren kannst, sodass ein Umzug auf eine neue Box oder das Klonen eines Setups nicht bedeutet, alles von Hand neu einzugeben. Der Import zeigt eine Vorschau und fragt nach Bestätigung und rührt niemals deine Backup-Daten oder -Historie an. Die Vorschau zählt die Speicherorte in der Datei und, wenn die Zugangsdaten dabei sind, die Zugangsdaten-Sätze; für eine ältere Datei ohne Speicherorte baut BombVault sie aus den importierten Einstellungen auf.

!!! warning "Der Export kann Zugangsdaten enthalten"
    Du wählst, ob die Zugangsdaten deiner Orte und Benachrichtigungen in der Datei enthalten sein sollen. Mit enthaltenen Zugangsdaten ist der Export so sensibel wie dein Recovery-Kit, also bewahre ihn an einem sicheren Ort auf. Ohne sie hält die Datei nur nicht-geheime Einstellungen.
