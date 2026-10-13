# Erste Schritte

Diese Seite führt dich von einer frischen Unraid-Box bis zu deiner ersten Sicherung.

## Voraussetzungen

| Voraussetzung | Hinweise |
|---|---|
| **Unraid 6.12+** | Ältere Versionen sind nicht getestet. Unraid ist das Hauptziel, BombVault läuft aber auch auf einem schlichten Docker-Host und auf TrueNAS Scale (siehe [Generischer Docker-Host](#generic-docker-host)). |
| **Speicherort des restic-Repos** | Ein lokaler Pfad (empfohlen: dein Array oder Cache), SMB, NFS oder ein beliebiges rclone-Backend. |
| **Docker-Socket** | Wird vom Template automatisch eingehängt (`/var/run/docker.sock`). |
| **Unraid-Flash** (`/boot`) | Wird vom Template automatisch komplett eingehängt (`/boot` nach `/host/boot`). Ermöglicht das Flash-Backup und lässt einen wiederhergestellten Container als normale, bearbeitbare Unraid-App wiedererscheinen. |
| **KVM-VMs** (optional) | Das VM-Backup spricht über SSH mit libvirt, kein libvirt-Mount. Einzurichten unter **Einstellungen, Anbindungen, Host-SSH** (siehe [Konfiguration](configuration.md)). |
| **ZFS-Datasets** (optional) | Dieselbe SSH-Verbindung wie beim VM-Backup, `zfs` auf dem Host und Host Data als `/mnt` eingebunden mit dem Zugriffsmodus Read/Write - Slave, der Vorgabe des Templates. Siehe [ZFS-Datasets](zfs-datasets.md). |
| **Android-App** (optional) | Android 10 oder neuer, gekoppelt mit Servern ab Version 9.7.0. Siehe [Android-App](android.md). |

## Auf Unraid installieren

Der einfachste Weg sind die **Community Applications**.

1. Öffne den **Apps**-Tab in Unraid.
2. Suche nach **BombVault**.
3. Klicke auf **Install**, setze die erforderlichen Variablen (unten) und übernimm.

!!! tip "Manuelle Template-Installation"
    Falls du das Template lieber von Hand hinzufügst:

    1. Gehe zu **Docker, Add Container, Template repositories** und füge hinzu:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Suche in den Templates nach **BombVault**.
    3. Setze die erforderlichen Variablen und klicke auf **Apply**.

## Generischer Docker-Host {#generic-docker-host}

Kein Unraid? BombVault läuft auch als schlichter Container auf jedem Docker-Host (darauf beruht auch die Container-Unterstützung auf TrueNAS Scale, noch vor einem eigenen Eintrag im dortigen App-Katalog).

1. Hol dir die fertig anpassbare [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) aus dem Repo.
2. Setze `APP_KEY` (siehe unten) und richte das Host-Data-Volume auf deine echte Datenwurzel. Die Kommentare in der Datei führen durch beides.
3. `docker compose up -d`, dann `https://<host-ip>:3443/` öffnen.

Was gegenüber Unraid anders ist:

- **Kein Bereich Flash.** Es gibt keinen Boot-Stick zu erfassen oder wiederherzustellen, der Bereich Flash hat hier also nichts zu tun. Stattdessen schlägt **Hinzufügen** auf der Seite Sicherungen die **Systemkonfiguration** als Ordner vor: ein `/etc`-Ordnersatz als Anfang, den du vor dem Speichern prüfst und anpasst.
- **Keine Unraid-eigenen Benachrichtigungen.** BombVaults eigene Benachrichtigungskanäle (Webhook, Alarme bei fehlgeschlagenem Off-site und so weiter) arbeiten wie gewohnt; ausgelassen wird nur die Unraid-spezifische Meldung an dessen eigenes Benachrichtigungssystem, weil es ein solches hier nicht gibt.
- **VM-Sicherung ist optional und braucht einen separaten, per SSH erreichbaren libvirtd-Host.** Siehe den auskommentierten Block in der Compose-Datei. Ein generischer Docker-Host bringt selbst keine VM-Verwaltung mit.
- **Kein Dashboard-Widget.** Das BombVault Widget ist ein Unraid-Plugin, dieser Schritt entfällt also ebenfalls.
- **Den Datenordner eines Containers finden.** Ohne Unraids `appdata`-Konvention wird der Datenordner eines Containers über die Segmente in `DATA_ROOT_SEGMENTS`, benannte Docker-Volumes, das Arbeitsverzeichnis eines Compose-Projekts und das Label `bombvault.data` gefunden (siehe [Erkennung der Sicherungsquellen](configuration.md#backup-source-detection)). Benannte Volumes und die `/etc`-Voreinstellung erreichen nur Pfade innerhalb des Host-Data-Mounts, richte Host Data also auf einen gemeinsamen übergeordneten Ordner, der auch die Datenwurzel von Docker abdeckt.
- **`PLATFORM`.** Setze es auf `generic` oder `truenas`. Ohne Wert erkennt BombVault Unraid an dessen eigenem Marker auf dem Flash-Mount und behandelt alles andere als generisch, und die nur für Unraid gedachten Schritte werden übersprungen, statt versucht zu werden und fehlzuschlagen.

**TrueNAS Scale** nimmt denselben Compose-Weg; ein Katalogeintrag ist im Repository vorbereitet, aber noch nicht eingereicht. Das VM-Backup braucht dort `LIBVIRT_URI`, weil das libvirtd von TrueNAS an einem eigenen Socket lauscht (`/run/truenas_libvirt/libvirt-sock`), den die drei `LIBVIRT_*`-Variablen nicht ausdrücken können (siehe [Konfiguration](configuration.md)). Wie weit das erprobt ist: Das Zvol-Backup lief gegen eine echte TrueNAS-Scale-Box, auf einem Zvol, das an einer laufenden VM hing, und `zfs snapshot`, `zfs send`, restic und `zfs receive` haben es Byte für Byte hin und zurück gebracht. Eine vollständige, von BombVault selbst gesteuerte Wiederherstellung wurde auf TrueNAS-Hardware noch nicht ausgeführt, und dieses Zvol war dünn provisioniert, der Durchsatz bei vielen Gigabyte ist also ungetestet. Teste dort eine Wiederherstellung, bevor du dich darauf verlässt.

## Die eine erforderliche Einstellung

Die einzige Variable, die du setzen musst, ist `APP_KEY`, ein 32-Byte-Hex-Geheimnis (64 Hex-Zeichen), das zum Ableiten des restic-Repository-Passworts dient.

Erzeuge eines auf einer beliebigen Maschine:

```bash
openssl rand -hex 32
```

Füge das Ergebnis in das Feld `APP_KEY` des Templates ein (Unraid) oder in die Umgebungsvariable `APP_KEY` in `docker-compose.yml` (generischer Docker-Host).

!!! danger "Verliere deinen APP_KEY nicht"
    Geht `APP_KEY` verloren, sind deine verschlüsselten Sicherungen unwiederbringlich. Bewahre ihn an einem sicheren Ort getrennt vom Server auf. Sobald BombVault läuft, setz ein Login-Passwort und lade unter **Einstellungen, Wiederherstellung** das **Recovery-Kit** herunter (siehe [Off-site & Wiederherstellung](offsite-recovery.md#recovery-kit)). Es enthält den Schlüssel, jeden Speicherort und die restic-Befehle.

Das Template hängt außerdem den Docker-Socket, den Flash (`/boot`) und das Wurzelverzeichnis **Host Data** (`/mnt`) für dich ein. *Quellen* und *Ziele* der Sicherungen liegen beide unter Host Data. Die vollständige Variablenreferenz und die Off-site-Einrichtung findest du unter [Konfiguration](configuration.md).

## Erster Start

![Die Übersicht nach der ersten Sicherung: was dich braucht, der Schutz jedes Bereichs, die Speicherorte und der Verlauf.](assets/screenshots/dashboard.png)

*Die Übersicht nach der ersten Sicherung: was dich braucht, der Schutz jedes Bereichs, die Speicherorte und der Verlauf.*

1. Öffne die Weboberfläche unter `https://<your-unraid-ip>:3443` (selbstsigniertes Zertifikat von Haus aus). Eine neue Installation öffnet mit **Willkommen bei BombVault**, wo steht, was BombVault auf dem Server gefunden hat.
2. Wähl **Los geht’s**. Die Übersicht zeigt dann die Karte **Erste Schritte** mit drei Schritten:
    1. **Bereiche wählen**: Schalte ein, was BombVault sichert (Container, VMs, Flash, Ordner, ZFS-Datasets, Selbstbackup). **Auswählen** öffnet die Liste der Einträge eines Bereichs, wo **Zeitplan pausieren** einen davon auslässt. Die Bereiche, die du hier einschaltest, laufen jede Nacht; das änderst du unter **Einstellungen, Zeitplan**.
    2. **Speicherort hinzufügen**: wohin die Sicherungen gehen. Ein lokaler Ordner reicht für den Anfang. Am sichersten ist ein zweiter Speicherort außer Haus, etwa eine Storage Box oder Backblaze B2. Lokale Repositories liegen standardmäßig unter `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` und werden bei der ersten Sicherung angelegt.
    3. **Erste Sicherung**: Danach läuft es jede Nacht von selbst.
3. Öffne **Sicherungen**, dort stehen alle Einträge in einer Liste. **Jetzt alles sichern** startet einen Lauf über alle eingeschalteten Bereiche, und **Jetzt sichern** auf der Seite eines Eintrags sichert nur diesen.

Hast du BombVault schon einmal benutzt? **Aus einer Sicherung wiederherstellen** auf der Willkommensseite öffnet die Seite Wiederherstellung, wo der **Notfall-Assistent** Einstellungen, Container, VMs und Ordner eines verlorenen Servers zurückholt. Siehe [Off-site & Wiederherstellung](offsite-recovery.md#emergency).

!!! tip "Optional: eine Reihenfolge festlegen"
    Wenn manche Container immer vor anderen gesichert werden sollen (zum Beispiel eine Datenbank vor der App, die sie nutzt), öffne die Karte **Reihenfolge** unter **Einstellungen, Zeitplan** und zieh sie in die gewünschte Reihenfolge. Geplante Läufe und das Gesamt-Backup folgen ihr dann; alles, was du unsortiert lässt, wird zuerst nach höchster Überfälligkeit gesichert.

!!! note "Host-Integration prüfen"
    Die Karte **Host-Integration** auf der Übersicht prüft jeden Mount und jedes CLI (Docker-Socket, libvirt, restic, qemu-img, rclone) und meldet fehlende Teile, sodass du bestätigen kannst, dass der Container richtig verdrahtet ist, bevor du dich darauf verlässt. Sie wartet unter **Anpassen**, bis du sie hinzufügst.

## Dich zurechtfinden {#navigation}

![Die Einstellungen als Kacheln, daneben die Karten der gewählten Kachel. Jede Änderung wird sofort geschrieben.](assets/screenshots/settings.png)

*Die Einstellungen als Kacheln, daneben die Karten der gewählten Kachel. Jede Änderung wird sofort geschrieben.*

Die Seitenleiste hat fünf Einträge: **Übersicht**, **Anomalien** (solange die Anomalie-Erkennung an ist), **Sicherungen**, **Wiederherstellung** und, unter **Abmelden**, **Einstellungen**. Die Hauptaktion einer Seite schwebt unten rechts, und **Mehr** enthält die seltenen. Am Handy enthält die Leiste Übersicht, Sicherungen, Wiederherstellung und Einstellungen, und **Mehr** hat den Rest.

Seiten und Einstellungen nutzen die ganze Breite des Fensters. Ein Fenster hat seine Knöpfe im Fuß, der Knopf, der weiterführt, steht rechts. Hat ein Fuß am Handy mehr als drei Knöpfe, bleiben der Weg hinaus und die Hauptaktion stehen, die übrigen liegen unter **Mehr**.

- **Sicherungen** listet jeden Container, jede VM, jeden Ordnersatz, den Flash, jedes ZFS-Dataset und das Selbstbackup als Einträge. Kacheln über der Liste wählen eine Art, und **Filter**, **Sortieren** und **Suchen** grenzen sie ein. Jeder Eintrag hat eine eigene Seite mit drei Reitern: **Sicherungen**, **Einstellungen** und **Prüfung**. Ein Zurück-Knopf links vom Symbol des Eintrags führt zur Liste.
- **Wiederherstellung** ist eine Seite für alles: Bereich wählen, Eintrag suchen und eine Sicherung aussuchen. Der Notfall-Assistent und die anderen Quellen starten hier ebenfalls.
- **Einstellungen** sind vierzehn Kacheln: Allgemein, Aussehen, Zeitplan, Speicherorte, Wiederherstellung, Benachrichtigungen, Prüfung, Sicherheit, Instanzen, Kopplung, Anbindungen, Apps, System und Info. Mit `/` oder Strg+K durchsuchst du alle, auch nach den Namen, die Seiten in früheren Versionen hatten. Unter **Aussehen** ziehst du die Kacheln in deine eigene Reihenfolge.
- **Speicherorte** und **Instanzen** sind Einstellungskacheln mit eigenen Seiten. **Auch in der Seitenleiste zeigen** auf ihrer Kachel heftet sie an die Seitenleiste.

Nichts hat einen Speichern-Knopf und nichts klappt auf: Seltenes hat eine eigene Karte oder ein Fenster, und Erklärungen stehen in den (i)-Blasen. Nach einem Update markieren grüne Punkte, was sich geändert hat. Ein Klick blendet einen Punkt aus, und **Neu** in der Karte Version unter **Einstellungen, Info** listet jede Änderung mit **Hingehen**.

## Aus dem Quellcode bauen {#build-from-source}

BombVault ist ein einzelnes statisches Go-Binary, das eine JSON-API und eine eingebettete React-Oberfläche ausliefert. Baue zuerst die Oberfläche, dann starte das Binary:

```bash
npm --prefix web ci
npm --prefix web run build     # schreibt web/dist, das das Binary einbettet
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # Unit- und Integrationstests, mit einem echten restic-Durchlauf hin und zurück
golangci-lint run ./...
go run ./cmd/bombvault         # antwortet unter https://localhost:3443 mit einem selbstsignierten Zertifikat
```

Der Build der Oberfläche wird auch für `go run` gebraucht. Das Repository enthält unter `web/dist` nur einen leeren Marker, ohne `npm --prefix web run build` bettet das Binary also nichts ein und antwortet mit `500 SPA index not found`, und das ist so zu erwarten. Docker, libvirt und Unraid lassen sich in der CI nicht testen, prüfe Mounts, restic und die SSH-Verbindung für das VM-Backup also auf einem echten Host mit der Karte **Host-Integration** der Übersicht (oder `GET /api/spike`), bevor du einen Pull Request öffnest.

## Nächste Schritte

- Durchstöbere die vollständigen **[Funktionen](features.md)**.
- Hol dir jeden Server deiner Gruppe mit der **[Android-App](android.md)** aufs Handy.
- Füge einen oder mehrere Speicherorte außer Haus hinzu (jeder Bereich kann gleichzeitig auf mehrere kopieren) und sichere dein Recovery-Kit, wie unter **[Off-site & Wiederherstellung](offsite-recovery.md)** beschrieben.
- Klonst du ein Setup oder wechselst auf eine neue Box? Nimm deine gesamte Konfiguration mit der Karte **Einstellungen exportieren / importieren** mit. Siehe [Konfiguration](configuration.md#portable-settings-export-and-import).
- Auf ein Problem gestoßen? Siehe **[Fehlerbehebung](troubleshooting.md)**.
