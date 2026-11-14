---
hero:
  claim: Ein Backup einwerfen.
  accent: Eine Wiederherstellung zünden.
  text: Sicherung und vollständige Wiederherstellung für Unraid und andere Docker-Hosts. Container, VMs, Ordner, ZFS-Datasets und der Flash-Stick kommen so zurück, wie sie waren.
  buttons:
    - { text: Erste Schritte, link: getting-started/, primary: true }
    - { text: Praxisbeispiele, link: guides/ }
    - { text: Quellcode, link: "https://github.com/junkerderprovinz/bombvault" }
---

# BombVault

![Die Übersicht: der Statuskopf, was dich braucht, der Schutz jedes Bereichs, die Speicherorte und der Verlauf.](assets/screenshots/overview/home-dark.png#only-dark)
![Die Übersicht: der Statuskopf, was dich braucht, der Schutz jedes Bereichs, die Speicherorte und der Verlauf.](assets/screenshots/overview/home-light.png#only-light)

BombVault ist eine selbst gehostete Web-App für **Sicherung und vollständige Notfallwiederherstellung** auf Unraid und anderen Docker-Hosts, etwa TrueNAS SCALE, ZimaOS oder einer schlichten Linux-Kiste. Es sichert Docker-Container, KVM/libvirt-VMs, Ordner, ZFS-Datasets und den Unraid-Flash. Es läuft als einzelner Multi-Arch-Docker-Container, hat eine Weboberfläche im dunklen, hellen oder System-Design und deckt den gesamten Ablauf ab: sichern, planen, prüfen und wiederherstellen.

Wiederherstellungen laufen automatisch. Auf Unraid erscheinen Container genau wie zuvor im Docker-Tab wieder, und VMs werden im VM Manager neu definiert, samt wieder angehängter Disks und UEFI-NVRAM. Keine manuelle Neuinstallation, keine Neukonfiguration, kein Drama. Auf TrueNAS läuft ein Container, der neben dem Original wiederhergestellt wurde, als einfacher Docker-Container, den der Bildschirm Apps nicht auflistet.

Angetrieben von [restic](https://restic.net), sodass jede Sicherung dedupliziert, inkrementell und standardmäßig verschlüsselt ist.

!!! note "Bewahre deinen APP_KEY sicher auf"
    BombVault leitet das Passwort des restic-Repositorys aus einem 32-Byte-Geheimnis namens `APP_KEY` ab. Geht es zusammen mit dem Recovery-Kit verloren, sind verschlüsselte Sicherungen unwiederbringlich. Erzeuge eines mit `openssl rand -hex 32` und bewahre es an einem sicheren Ort auf. Siehe [Konfiguration](configuration.md).

## Was BombVault schützt {#what-bombvault-protects}

Jeder Container, jede VM, jedes Ordner-Set, der Flash, jedes ZFS-Dataset und die eigenen Einstellungen von BombVault sind ein **Eintrag** mit einer eigenen Seite. Einträge einer Art bilden einen **Bereich**, den du unter **Einstellungen, Allgemein** einschaltest.

| Bereich | Was gesichert wird |
|---|---|
| **Container** | Appdata-Verzeichnis plus die Container-Definition (Image, Umgebungsvariablen, Ports, Labels, Volumes). |
| **VMs** | Disk-Image(s) der VM, die XML-Definition und das UEFI-NVRAM, gesichert über SSH (kein libvirt-Mount). |
| **Flash** | Der gesamte USB-Flash (`/boot`): Betriebssystem, Lizenz, Array-Konfiguration, Shares, Netzwerk- und Plugin-Konfiguration. |
| **Selbstbackup** | Das eigene `/config` von BombVault: seine Einstellungsdatenbank, die Zugangsdaten seiner Speicherorte, das libvirt-SSH-Schlüsselpaar, der Schlüssel und die festgehaltenen Host-Schlüssel der ZFS-Server und die Zertifikate. |
| **Ordner** | Jeder Ordner auf dem Server als Ordner-Set, jeweils mit optionalen Ausschlussmustern. |
| **ZFS-Datasets** | Ein Dataset mit allen Datasets darunter, gelesen aus einem ZFS-Snapshot und gespeichert wie ein Ordner. Siehe [ZFS-Datasets](zfs-datasets.md). |

## Die Wiederherstellung ist der Star {#restore-is-the-star}

Nachdem die Daten aus dem restic-Snapshot zurückkopiert sind, spielt BombVault die gespeicherte Container-Definition gegen die Docker-API ein, sodass der Container im Unraid-Docker-Tab wieder erscheint, als wäre er nie weg gewesen (gleiches Image, gleiche Einstellungen, gleiche Port-Zuordnungen). Bei VMs wird das XML über SSH neu definiert und werden ihre Disks und das UEFI-NVRAM wieder angehängt, selbst nachdem die VM gelöscht wurde.

Ein Fenster stellt jede Art wieder her. Vorgewählt ist **Neben das Original**, das die Sicherung als Kopie zurückbringt und das Original lässt, wie es ist. **Original ersetzen** sichert vorher den heutigen Zustand, damit sich eine Wiederherstellung rückgängig machen lässt. Ein Ordner oder ein neues ZFS-Dataset sind die anderen Ziele.

Wenn eine Sicherung abhängige Container anhält, kommen sie in der richtigen Reihenfolge zurück: BombVault startet sie in ihrer Compose-`depends_on`-Reihenfolge neu und wartet, bis jeder gesund meldet, bevor die davon abhängigen starten, sodass nichts einer Datenbank oder einem Gateway vorauseilt, die noch nicht oben sind. Siehe [Wiederherstellung](restore.md) und [Befehle und andere Container](backups.md#commands).

## Wie es funktioniert {#how-it-works}

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + sign-ins (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault nutzt den Docker-Socket, um Container vor einer Sicherung anzuhalten und nach einer Wiederherstellung neu anzulegen. Bei VMs führt es `virsh` auf dem Host über SSH (`qemu+ssh://`) aus, um eine Domain geordnet herunterzufahren oder einen Live-Snapshot zu machen. Es bindet nie einen libvirt-Pfad ein und kann dem VM Manager auf dem Host daher nicht in die Quere kommen.

BombVault ist die Steuerungs- und Oberflächenschicht, nicht die Speicher-Engine. Die gesamte Datenbewegung läuft über restic.

## Schnellstart {#quick-start}

Neu hier? Geh zu **[Erste Schritte](getting-started.md)**, um BombVault zu installieren und die erste Sicherung zu machen, und richte dann **[Off-site und Wiederherstellung](offsite-recovery.md)** ein. Die **[Praxisbeispiele](guides.md)** führen die üblichen Aufgaben von Anfang bis Ende durch, die Seite **[Einstellungen](settings.md)** listet jede Kachel auf, und **[Funktionen](features.md)** ist der kurze Katalog von allem, was BombVault kann.

Jeder Bereich kann auf mehrere Speicherorte zugleich kopieren, ein gekoppeltes BombVault kann deine Off-site-Kopien auf einem Server verwahren, der Löschen verweigert, und der **Notfall-Assistent** holt einen verlorenen Server aus seinem Recovery-Kit zurück, über deine Gruppe oder mit seinem eingetippten Schlüssel. Um deine Konfiguration auf eine neue Box mitzunehmen, nutze die Karte **Einstellungen exportieren / importieren**. Siehe [Off-site und Wiederherstellung](offsite-recovery.md) und [Konfiguration](configuration.md#portable-settings-export-and-import).

Die **[Android-App](android.md)** bringt jeden Server deiner Gruppe aufs Handy, mit dem Aktivitätsprotokoll von allen auf einem Bildschirm.

Die Dokumentation gibt es in 26 Sprachen und die Oberfläche in 42. Seiten, die noch keine Übersetzung haben, erscheinen auf Englisch.

## Begriffe {#glossary}

Ein paar Wörter haben in BombVault und auf diesen Seiten eine feste Bedeutung.

| Begriff | Bedeutung |
|---|---|
| **Eintrag** | Eine Sache, die BombVault sichert: ein Container, eine VM, ein Ordner-Set, der Flash, ein ZFS-Dataset mit denen darunter oder seine eigenen Einstellungen. Jeder hat eine eigene Seite. |
| **Bereich** | Alle Einträge einer Art, etwa Container. Bereiche schaltest du unter **Einstellungen, Allgemein** ein. |
| **Speicherort** | Ein Ort, an den Sicherungen gehen: ein Ordner auf diesem Server, ein Share, ein eigener Server oder ein Cloud-Dienst. Siehe [Speicherorte](offsite-recovery.md#destinations). |
| **Lokal** | Der Speicherort auf diesem Server, der die Ordner der Bereiche hält. |
| **Eigener Ordner** | Das Repository, in das ein Bereich seine Sicherungen zuerst schreibt. Standardmäßig liegt es auf diesem Server und kann in einem Speicherort liegen. Siehe [Der eigene Ordner eines Bereichs](offsite-recovery.md#own-folder). |
| **Kopie** | Eine Sicherung, die mit `restic copy` aus dem eigenen Ordner an einen anderen Speicherort kopiert wurde. |
| **Direktes Repository** | Ein Repository an einem Off-site-Ort, auf das Einträge direkt sichern, ohne lokale Kopie dazwischen. Siehe [Direkte Repositories](offsite-recovery.md#direct-repositories). |
| **Zugangsdaten** | Der gespeicherte Schlüssel oder das Passwort für einen S3-Dienst oder einen rest-server. Eines davon ist die Vorgabe. Siehe [Zugangsdaten](offsite-recovery.md#sign-ins). |
| **Zuordnung** | Welche Speicherorte die Sicherungen eines Eintrags bekommen, am Eintrag oder als Vorgabe seines Bereichs eingestellt. Siehe [Zuordnung pro Eintrag](offsite-recovery.md#placement). |
| **Replik** | Eine blockgenaue Kopie eines ZFS-Eintrags auf einem zweiten ZFS-Server, gemacht mit `zfs send`. Sie steht neben der Sicherung und ersetzt sie nicht. Siehe [Replik](zfs-datasets.md#replica). |
| **Rolle** | Etwas, das eine Instanz deiner Gruppe für eine andere tut: Empfänger, Abholer, ZFS-Server oder Schlüsselhüter. Siehe [Instanzen und Rollen](offsite-recovery.md#roles). |

## Danksagung {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** von [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) hat BombVault die Ausgangsidee gegeben: Sicherung mit einem Klick und automatische Neuinstallation von Docker-Containern. BombVault ist eine eigene Umsetzung in Go und mit restic, die die Idee auf VMs, den Flash und mehr überträgt.
- **[restic](https://restic.net/)** ist die schnelle, sichere, deduplizierende Backup-Engine, die BombVault antreibt.
- **[rclone](https://rclone.org/)** liefert die Cloud-Backends.
- Die meisten Symbole auf Knöpfen stammen aus dem freien Set Core Solid von **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [Quelle](https://github.com/webalys-hq/streamline-vectors)). Die anderen kommen von Font Awesome Free, Material Design Icons, Simple Icons und Tabler Icons oder wurden für das Projekt gezeichnet.

## Lizenz {#license}

Copyright (C) 2026 Junker der Provinz. BombVault ist freie Software unter der **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Du darfst es ausführen, untersuchen, weitergeben und ändern. Wenn du es verbreitest oder eine geänderte Version als Netzwerkdienst betreibst, musst du deinen Quellcode unter derselben Lizenz veröffentlichen und die bestehenden Urheber- und Herkunftshinweise beibehalten.

Name und Markenauftritt sind nicht lizenziert. Die AGPL deckt nur den Quellcode ab: „BombVault“, sein Logo und sein Markenauftritt bleiben vorbehalten, ein Fork muss also einen eigenen Namen und Markenauftritt verwenden und darf sich nicht als BombVault ausgeben.

## Links {#links}

- **Quellcode:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid-Supportthread:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issues:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-gleiche Kontrolle über den Host"
    Über den Docker-Socket kann BombVault Container anhalten, entfernen und neu anlegen und Appdata lesen und schreiben, und für die VM-Sicherung meldet es sich per SSH am Host an, um `virsh` auszuführen. Wer seine Weboberfläche erreicht, hat damit praktisch Root auf dem Host. Betreibe BombVault nur in einem vertrauenswürdigen, nicht offenen Netzwerk und setze ein Login-Passwort unter **Einstellungen, Sicherheit**. Das Recovery-Kit, Schlüsselhinterlegungen, das Flash-ZIP und das Exportieren einer Flash-Sicherung oder eines Selbstbackups brauchen eines. Das vollständige Sicherheitsmodell steht unter [Konfiguration](configuration.md#security-model).
