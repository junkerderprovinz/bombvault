# Konfiguration

Diese Seite behandelt die Umgebungsvariablen des Containers, die vom Template bereitgestellten Mounts, das VM-Backup über SSH und die Off-site-Einrichtung. Backup-**Repository-Pfade** werden in der App konfiguriert (Einstellungen, Backup-Pfade), nicht über Umgebungsvariablen.

## Umgebungsvariablen

| Variable | Erforderlich | Beschreibung |
|---|---|---|
| `APP_KEY` | **Ja** | 32-Byte-Hex-Geheimnis (64 Hex-Zeichen) zum Ableiten des restic-Repo-Passworts. Erzeuge es mit `openssl rand -hex 32`. Bewahre es sicher auf: geht es verloren, sind verschlüsselte Backups unwiederbringlich. |
| `LIBVIRT_HOST` | Für VMs | Über SSH erreichter Unraid-Host für das VM-Backup (Standard `host.docker.internal`; das Template füllt einen LAN-IP-Platzhalter vor). Nutze deine Unraid-LAN-IP, erforderlich in einem benutzerdefinierten `br0.x`-Netzwerk. |
| `LIBVIRT_SSH_PORT` | Nein | Host-SSH-Port für das VM-Backup (Standard `22`). |
| `LIBVIRT_SSH_USER` | Nein | SSH-Benutzer auf dem Host für das VM-Backup (Standard `root`). |
| `PORT` | Nein | HTTP-Port (Standard `3000`; nur mit `HTTP_ONLY=true` verwendet). |
| `HTTPS_PORT` | Nein | HTTPS-Port (Standard `3443`; das Template veröffentlicht ihn 1:1, sodass die WebUI unter `https://<ip>:3443` antwortet). |
| `HTTP_ONLY` | Nein | Setze `true`, um den selbstsignierten HTTPS-Listener zu deaktivieren und nur schlichtes HTTP auszuliefern (zur Verwendung hinter einem TLS-terminierenden Reverse Proxy). |
| `HOST_SOURCE_ROOT` | Nein | Der als **Host Data** eingehängte Host-Pfad (Standard `/mnt`). BombVault übersetzt die von Docker gemeldeten Bind-Mount-Quellen in Pfade unter diesem Mount. Nur ändern, wenn du ein anderes Host-Wurzelverzeichnis eingehängt hast. |
| `BOMBVAULT_SELF_CONTAINER` | Nein | Der Name des BombVault-Containers selbst, sodass er sich nie selbst sichert (und damit stoppt) (Standard `BombVault`; bei Bridge-Netzwerk automatisch über den Hostnamen erkannt). |
| `BACKUP_MAX_HOURS` | Nein | Maximale Echtzeit-Stunden, die ein einzelner Backup-Lauf seinen Bereichs-Lock halten darf, bevor er zwangsweise abgebrochen wird (ein Schutz, damit ein verklemmter Lauf den Bereich nicht für immer blockieren kann). Leer (der Standard) verwendet `48`. Erhöhe es für sehr große oder langsame Cloud-Backups (ein am Limit abgebrochener Lauf schlägt mit `context deadline exceeded` fehl). Setze `0`, um das Limit ganz zu deaktivieren. |
| `TZ` | Nein | Zeitzone für den Planer (zum Beispiel `Europe/Berlin`). |

## Mounts

Hänge den Docker-Socket, den Flash (`/boot`) und das Wurzelverzeichnis **Host Data** (`/mnt`) ein, wie im CA-Template gezeigt. Backup-*Quellen* und -*Ziele* liegen beide unter Host Data, und es ist **rslave** eingehängt, sodass eine Remote-Freigabe, die nach dem Containerstart eingehängt wird (zum Beispiel unter `/mnt/remotes`), ohne Neustart sichtbar wird.

Backup-Repository-Pfade sind standardmäßig `/mnt/user/bombvault/{container,vms,flash,config,files}`, angelegt beim ersten Backup. Ändere den Ort jederzeit unter **Einstellungen, Backup-Pfade**.

!!! note "Prüfung der Host-Integration"
    Öffne `/spike` in der Web-Oberfläche, nachdem der Container gestartet ist. Es prüft jeden Mount und jedes CLI (Docker-Socket, libvirt, restic, qemu-img, rclone) und meldet fehlende Teile.

## Sicherheitsmodell

!!! warning "Root-äquivalente Kontrolle über den Host"
    Über den Docker-Socket kann BombVault Container stoppen, entfernen und neu erstellen sowie Appdata lesen/schreiben, und für das VM-Backup meldet es sich über SSH am Host an (`qemu+ssh://`, standardmäßig root), um `virsh` auszuführen. Wer die Web-Oberfläche erreichen kann, hat praktisch Root-Rechte auf dem Host.

- **Optionaler Passwortschutz** (Einstellungen, Sicherheit): setze ein Passwort, um Login zu verlangen, lösche es, um zu deaktivieren. Standardmäßig aus für die Nutzung im vertrauenswürdigen LAN. Sitzungen sind signiert (HMAC abgeleitet aus `APP_KEY`), und eine Passwortänderung macht sie ungültig; Logins sind ratenbegrenzt.
- Weil die Sperre Opt-in ist, sind bei nicht gesetztem Passwort die gesamte UI und API (einschließlich der Off-site-Einrichtung, der Manipulationstest-Routen und des Recovery-Kits) für jeden erreichbar, der den Port erreichen kann. Aktiviere die Sperre, sobald Off-site-, unveränderliche Backups oder Verschlüsselung im Einsatz sind.
- Betreibe BombVault nur in einem vertrauenswürdigen, nicht exponierten Netzwerk. Für Fernzugriff setze es hinter einen Reverse Proxy, der Authentifizierung und TLS ergänzt. Antworten tragen grundlegende Sicherheits-Header (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
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

Richte eine Off-site-Replik im Tab **Einstellungen, Off-site** ein. Siehe [Off-site & Wiederherstellung](offsite-recovery.md) für den vollständigen Ablauf (unveränderlich/append-only, Manipulationstest und DR-Übungen). Kurz gefasst:

- **Backends:** SMB/CIFS und NFS (Freigabe einhängen und einen Backup-Pfad darauf richten), native restic-Backends ohne rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`) oder jedes rclone-Remote (`rclone:<remote>:<bucket>/path`).
- **Cloud-Zugangsdaten** werden verschlüsselt gespeichert unter Einstellungen, Off-site, Cloud-Zugangsdaten.
- **SSH-Ziele brauchen auf der Gegenseite nichts installiert.** `sftp:` benötigt nur einen SSH-Server. Füge den öffentlichen Schlüssel aus **Einstellungen, System, VM-Backup über SSH** (auch unter `/config/ssh/id_ed25519.pub`) den `~/.ssh/authorized_keys` des Zielbenutzers hinzu.
- **Off-site-Kopie:** BombVault repliziert neue Snapshots mit `restic copy` auf Best-Effort-Basis. Das lokale Repo bleibt primär. Jeder Bereich hat seinen eigenen Off-site-Zeitplan, plus einen Button **Jetzt replizieren**.
- **Mehrere Off-site-Ziele pro Bereich:** jeder Bereich kann gleichzeitig an mehrere Off-site-Ziele replizieren. Füge zusätzliche Ziele unter Einstellungen, Off-site hinzu, jedes mit eigenem Repository, S3-Speicherklasse, Append-only-Flag, Aufbewahrung und Wachstumsbudget; sie alle replizieren nach dem Off-site-Zeitplan dieses Bereichs. Eine bestehende einzelne Off-site-Einrichtung wird als erstes Ziel übernommen.
- **Aufbewahrung pro Quelle:** die lokale Richtlinie liegt unter Einstellungen, Pfade & Speicher; die Off-site-Richtlinie unter Einstellungen, Off-site (lasse sie ganz auf null, um Off-site-Snapshots nie automatisch zu kürzen).
- **Bandbreitenlimits:** begrenze die restic-Upload-/Download-Rate unter Einstellungen, Off-site.
- **Kalt- und Archiv-Speicherklasse (S3):** wähle für ein natives S3-Off-site-Repo eine wiederherstellungslesbare Stufe (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). rclone-Remotes setzen ihre Klasse in der rclone-Konfiguration.

## Portable Einstellungen (Export und Import) {#portable-settings-export-and-import}

Die Karte **Einstellungen exportieren und importieren** auf der Einstellungsseite schreibt deine gesamte BombVault-Konfiguration (Bereichseinstellungen, Off-site-Ziele, Zeitpläne, Aufbewahrung, Benachrichtigungen) in eine portable JSON-Datei, die du auf einer anderen Instanz importieren kannst, sodass ein Umzug auf eine neue Box oder das Klonen eines Setups nicht bedeutet, alles von Hand neu einzugeben. Der Import zeigt eine Vorschau und fragt nach Bestätigung und rührt niemals deine Backup-Daten oder -Historie an.

!!! warning "Der Export kann Zugangsdaten enthalten"
    Du wählst, ob die Off-site- und Benachrichtigungs-Zugangsdaten in der Datei enthalten sein sollen. Mit enthaltenen Zugangsdaten ist der Export so sensibel wie dein Recovery-Kit, also bewahre ihn an einem sicheren Ort auf. Ohne sie hält die Datei nur nicht-geheime Einstellungen.
