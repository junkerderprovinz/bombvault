# BombVault

**Deine Unraid-Daten, sicher im Tresor verwahrt. Ein Backup einwerfen. Eine Wiederherstellung zünden.**

BombVault ist eine selbstgehostete, Unraid-native Web-App für **Sicherung und vollständige Notfallwiederherstellung** deiner Docker-Container und KVM/libvirt-VMs. Sie läuft als einzelner Multi-Arch-Docker-Container, hat eine Weboberfläche im dunklen, hellen oder System-Design und deckt den gesamten Ablauf ab: sichern, planen, prüfen und wiederherstellen.

Wiederherstellungen laufen automatisch. Container erscheinen exakt wie zuvor wieder im Unraid-Docker-Tab, und VMs werden im VM Manager neu definiert, samt wieder angehängter Datenträger und UEFI-NVRAM. Keine manuelle Neuinstallation, keine Neukonfiguration, kein Drama.

Angetrieben von [restic](https://restic.net), sodass jede Sicherung dedupliziert, inkrementell und stets verschlüsselt ist.

!!! note "Bewahre deinen APP_KEY sicher auf"
    BombVault leitet das Passwort des restic-Repositorys aus einem 32-Byte-Geheimnis namens `APP_KEY` ab. Geht es zusammen mit dem Recovery-Kit verloren, sind verschlüsselte Sicherungen unwiederbringlich. Erzeuge eines mit `openssl rand -hex 32` und bewahre es an einem sicheren Ort auf. Siehe [Konfiguration](configuration.md).

## Was BombVault schützt

Jeder Container, jede VM, jeder Ordnersatz, der Flash, jedes ZFS-Dataset und BombVaults eigene Einstellungen sind ein **Eintrag** mit einer eigenen Seite. Einträge einer Art bilden einen **Bereich**, den du unter **Einstellungen, Allgemein** einschaltest.

| Bereich | Was gesichert wird |
|---|---|
| **Container** | Appdata-Verzeichnis plus die Container-Definition (Image, Umgebungsvariablen, Ports, Labels, Volumes). |
| **VMs** | VM-Datenträger-Image(s), die XML-Definition und UEFI-NVRAM, gesichert über SSH (kein libvirt-Mount). |
| **Flash** | Der gesamte USB-Flash (`/boot`): OS, Lizenz, Array-Konfiguration, Freigaben, Netzwerk- und Plugin-Konfiguration. |
| **Selbstbackup** | BombVaults eigenes `/config`: seine Einstellungsdatenbank, die Zugangsdaten seiner Speicherorte und das libvirt-SSH-Schlüsselpaar. |
| **Ordner** | Jeder beliebige Ordner auf dem Server als Ordnersatz, jeweils mit optionalen Ausschlussmustern. |
| **ZFS-Datasets** | Ein Dataset mit allen Datasets darunter, gelesen aus einem ZFS-Snapshot und gespeichert wie ein Ordner. Siehe [ZFS-Datasets](zfs-datasets.md). |

## Die Wiederherstellung ist der Star

Nach dem Zurückkopieren der Daten aus dem restic-Snapshot spielt BombVault die gespeicherte Container-Definition gegen die Docker-API ein, sodass der Container wieder im Unraid-Docker-Tab erscheint, als wäre er nie weg gewesen (gleiches Image, gleiche Einstellungen, gleiche Port-Zuordnungen). Bei VMs wird die XML über SSH neu definiert und ihre Datenträger sowie UEFI-NVRAM wieder angehängt, selbst nachdem die VM gelöscht wurde.

Ein Fenster stellt jede Art wieder her. Vorgewählt ist **Neben das Original**: Die Sicherung kommt als Kopie zurück, und das Original bleibt, wie es ist. **Original ersetzen** sichert vorher den heutigen Zustand, damit sich eine Wiederherstellung rückgängig machen lässt. Ein Ordner oder ein neues ZFS-Dataset sind die anderen Ziele.

Wenn eine Sicherung abhängige Container stoppt, kommen sie in der richtigen Reihenfolge zurück: BombVault startet sie in ihrer Compose-`depends_on`-Reihenfolge neu und wartet, bis jeder als gesund gemeldet wird, bevor die davon abhängigen gestartet werden, sodass nichts einer Datenbank oder einem Gateway vorauseilt, das noch nicht bereit ist. Siehe [Funktionen](features.md).

## Wie es funktioniert

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault nutzt den Docker-Socket, um Container vor einer Sicherung zu stoppen und nach einer Wiederherstellung neu zu erstellen. Für VMs führt es `virsh` über SSH (`qemu+ssh://`) auf dem Host aus, um eine VM geordnet herunterzufahren oder einen Live-Snapshot zu erstellen. Es hängt nie einen libvirt-Pfad per Bind-Mount ein und kann dem VM Manager auf dem Host deshalb nicht in die Quere kommen.

BombVault ist die Orchestrierungs- und UI-Schicht, nicht die Speicher-Engine. Sämtliche eigentliche Datenbewegung läuft über restic.

## Schnellstart

Neu hier? Ab zu **[Erste Schritte](getting-started.md)**, um BombVault auf Unraid über Community Applications zu installieren und deine erste Sicherung auszuführen. Erkunde dann die vollständigen **[Funktionen](features.md)**, stimme deine **[Konfiguration](configuration.md)** ab und richte **[Off-site & Wiederherstellung](offsite-recovery.md)** ein.

Jeder Bereich kann gleichzeitig auf mehrere Speicherorte kopieren, ein gekoppeltes BombVault kann deine Off-site-Kopien in einem Server verwahren, der kein Löschen zulässt, und der **Notfall-Assistent** holt einen verlorenen Server zurück, mit seinem Recovery-Kit, über deine Gruppe oder mit dem eingetippten Schlüssel. Um deine Konfiguration auf eine neue Box mitzunehmen, nimm die Karte **Einstellungen exportieren / importieren**. Siehe [Off-site & Wiederherstellung](offsite-recovery.md) und [Konfiguration](configuration.md#portable-settings-export-and-import).

Die **[Android-App](android.md)** holt jeden Server deiner Gruppe aufs Handy, mit dem Aktivitätsprotokoll aller Server auf einem Bildschirm.

## Danksagung {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** von [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) hat BombVault die Ausgangsidee gegeben: Docker-Container mit einem Klick sichern und automatisch neu installieren. BombVault ist eine eigenständige Umsetzung mit Go und restic, die die Idee auf VMs, den Flash und mehr überträgt.
- **[restic](https://restic.net/)** ist die schnelle, sichere, deduplizierende Backup-Engine, die BombVault steuert.
- **[rclone](https://rclone.org/)** liefert die Cloud-Backends.
- Die meisten Symbole auf Knöpfen stammen aus dem freien Core-Solid-Set von **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [Quelle](https://github.com/webalys-hq/streamline-vectors)). Die übrigen stammen aus Font Awesome Free, Material Design Icons, Simple Icons und Tabler Icons oder wurden für das Projekt gezeichnet.

## Lizenz {#license}

Copyright (C) 2026 Junker der Provinz. BombVault ist freie Software unter der **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Du darfst sie ausführen, untersuchen, weitergeben und verändern. Wenn du sie verbreitest oder eine veränderte Version als Netzwerkdienst betreibst, musst du deinen Quellcode unter derselben Lizenz veröffentlichen und die bestehenden Urheberrechts- und Namensnennungshinweise beibehalten.

Der Name und das Branding sind nicht lizenziert. Die AGPL gilt nur für den Quellcode: "BombVault", sein Logo und sein Branding bleiben vorbehalten, ein Fork muss also einen eigenen Namen und ein eigenes Branding verwenden und darf sich nicht als BombVault ausgeben.

## Links

- **Quellcode:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid-Support-Thread:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issues:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-äquivalente Kontrolle über den Host"
    Über den Docker-Socket kann BombVault Container stoppen, entfernen und neu erstellen sowie Appdata lesen/schreiben, und für das VM-Backup meldet es sich über SSH am Host an, um `virsh` auszuführen. Wer die Weboberfläche erreichen kann, hat praktisch Root-Rechte auf dem Host. Betreibe BombVault nur in einem vertrauenswürdigen, nicht exponierten Netzwerk und setz ein Login-Passwort unter **Einstellungen, Sicherheit**. Das Recovery-Kit, Hinterlegungen und das Löschen der Sicherungen an einem Speicherort brauchen eins. Das vollständige Sicherheitsmodell findest du unter [Konfiguration](configuration.md#security-model).
