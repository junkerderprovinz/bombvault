# BombVault

**Deine Unraid-Daten, sicher im Tresor verwahrt. Ein Backup einwerfen. Eine Wiederherstellung zünden.**

BombVault ist eine selbstgehostete, Unraid-native Web-App für **Backup und vollständige Notfallwiederherstellung** deiner Docker-Container und KVM/libvirt-VMs. Sie läuft als einzelner Multi-Arch-Docker-Container, bietet eine moderne Weboberfläche, die sich an die Hell/Dunkel-Einstellung deines Systems anpasst, und deckt den gesamten Lebenszyklus ab: sichern, planen, prüfen und wiederherstellen.

Wiederherstellungen laufen automatisch. Container erscheinen exakt wie zuvor wieder im Unraid-Docker-Tab, und VMs werden im VM Manager neu definiert, samt wieder angehängter Datenträger und UEFI-NVRAM. Keine manuelle Neuinstallation, keine Neukonfiguration, kein Drama.

Angetrieben von [restic](https://restic.net), sodass jedes Backup dedupliziert, inkrementell und stets verschlüsselt ist.

!!! note "Bewahre deinen APP_KEY sicher auf"
    BombVault leitet das Passwort des restic-Repositorys aus einem 32-Byte-Geheimnis namens `APP_KEY` ab. Geht es verloren, sind verschlüsselte Backups unwiederbringlich. Erzeuge eines mit `openssl rand -hex 32` und bewahre es an einem sicheren Ort auf. Siehe [Konfiguration](configuration.md).

## Was BombVault schützt

| Bereich | Was gesichert wird |
|---|---|
| **Docker-Container** | Appdata-Verzeichnis plus die Container-Definition (Image, Umgebungsvariablen, Ports, Labels, Volumes). |
| **KVM / libvirt-VMs** | VM-Datenträger-Image(s), die XML-Definition und UEFI-NVRAM, gesichert über SSH (kein libvirt-Mount). |
| **Unraid-Flash** | Der gesamte USB-Flash (`/boot`): OS, Lizenz, Array-Konfiguration, Freigaben, Netzwerk- und Plugin-Konfiguration. |
| **App-Konfiguration** | BombVaults eigenes `/config`: seine Einstellungsdatenbank, Off-site-Zugangsdaten und das libvirt-SSH-Schlüsselpaar. |
| **Dateien & Ordner** | Benannte **Dateisätze**, jeder beliebige Ordner auf dem Server, jeweils mit optionalen Ausschlussmustern pro Satz. |

## Die Wiederherstellung ist der Star

Nach dem Zurückkopieren der Daten aus dem restic-Snapshot spielt BombVault die gespeicherte Container-Definition gegen die Docker-API ein, sodass der Container wieder im Unraid-Docker-Tab erscheint, als wäre er nie weg gewesen (gleiches Image, gleiche Einstellungen, gleiche Port-Zuordnungen). Bei VMs wird die XML über SSH neu definiert und ihre Datenträger sowie UEFI-NVRAM wieder angehängt, selbst nachdem die VM gelöscht wurde.

Wenn ein Backup abhängige Container stoppt, kommen sie in der richtigen Reihenfolge zurück: BombVault startet sie in ihrer Compose-`depends_on`-Reihenfolge neu und wartet, bis jeder als gesund gemeldet wird, bevor die davon abhängigen gestartet werden, sodass nichts einer Datenbank oder einem Gateway vorauseilt, das noch nicht bereit ist. Siehe [Funktionen](features.md).

## Wie es funktioniert

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault ist die Orchestrierungs- und UI-Schicht, nicht die Speicher-Engine. Sämtliche eigentliche Datenbewegung läuft über restic.

## Schnellstart

Neu hier? Ab zu **[Erste Schritte](getting-started.md)**, um BombVault auf Unraid über Community Applications zu installieren und dein erstes Backup auszuführen. Erkunde dann die vollständigen **[Funktionen](features.md)**, stimme deine **[Konfiguration](configuration.md)** ab und richte **[Off-site & Wiederherstellung](offsite-recovery.md)** ein.

Off-site kann pro Bereich gleichzeitig auf mehrere Ziele verteilen, ein schreibgeschütztes **Empfänger-Dashboard** überwacht diese Kopien auf der Box, die sie empfängt, und du kannst deine gesamte Konfiguration mit der Karte **Einstellungen exportieren und importieren** auf eine neue Box mitnehmen. Siehe [Off-site & Wiederherstellung](offsite-recovery.md) und [Konfiguration](configuration.md#portable-settings-export-and-import).

## Links

- **Quellcode:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid-Support-Thread:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issues:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-äquivalente Kontrolle über den Host"
    Über den Docker-Socket kann BombVault Container stoppen, entfernen und neu erstellen sowie Appdata lesen/schreiben, und für das VM-Backup meldet es sich über SSH am Host an, um `virsh` auszuführen. Wer die Web-Oberfläche erreichen kann, hat praktisch Root-Rechte auf dem Host. Betreibe BombVault nur in einem vertrauenswürdigen, nicht exponierten Netzwerk und aktiviere die optionale Passwortsperre (Einstellungen, Sicherheit), sobald Off-site- oder unveränderliche Backups im Einsatz sind. Das vollständige Sicherheitsmodell findest du unter [Konfiguration](configuration.md).
