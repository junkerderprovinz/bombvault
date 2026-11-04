# Erste Schritte

Diese Seite führt dich von einer frischen Unraid-Box bis zu deiner ersten Sicherung.

## Voraussetzungen {#requirements}

| Voraussetzung | Hinweise |
|---|---|
| **Unraid 6.12+** | Ältere Versionen sind nicht getestet. Unraid ist das Hauptziel, BombVault läuft aber auch auf einem schlichten Docker-Host und auf TrueNAS Scale (siehe [Generischer Docker-Host](#generic-docker-host) und [TrueNAS SCALE](#truenas)). |
| **Ort des restic-Repos** | Ein lokaler Pfad (empfohlen: dein Array oder Cache), SMB, NFS oder ein beliebiges rclone-Backend. |
| **Docker-Socket** | Wird vom Template automatisch gemountet (`/var/run/docker.sock`). |
| **Unraid-Flash** (`/boot`) | Wird vom Template automatisch komplett gemountet (`/boot` nach `/host/boot`). Ermöglicht die Flash-Sicherung und lässt einen wiederhergestellten Container als normale, bearbeitbare Unraid-App wiedererscheinen. |
| **KVM-VMs** (optional) | Die VM-Sicherung spricht über SSH mit libvirt, kein libvirt-Mount. Einzurichten unter **Einstellungen, Anbindungen, Host-SSH** (siehe [Konfiguration](configuration.md)). |
| **ZFS-Datasets** (optional) | Dieselbe SSH-Verbindung wie bei der VM-Sicherung, `zfs` auf dem Host und Host Data als `/mnt` eingebunden mit dem Zugriffsmodus Read/Write - Slave, der Vorgabe des Templates. Siehe [ZFS-Datasets](zfs-datasets.md). |
| **Android-App** (optional) | Android 10 oder neuer, gekoppelt mit Servern ab Version 9.7.0. Siehe [Android-App](android.md). |

## Auf Unraid installieren {#install-on-unraid}

Der einfachste Weg sind die **Community Applications**.

1. Öffne den Tab **Apps** in Unraid.
2. Suche nach **BombVault**.
3. Klicke auf **Install**, setze die erforderlichen Variablen (unten) und übernimm.

!!! tip "Template von Hand installieren"
    Wenn du das Template lieber von Hand hinzufügst:

    1. Gehe zu **Docker, Add Container, Template repositories** und füge hinzu:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Suche in den Templates nach **BombVault**.
    3. Setze die erforderlichen Variablen und klicke auf **Apply**.

## Generischer Docker-Host {#generic-docker-host}

Nicht auf Unraid? BombVault läuft auch als schlichter Container auf jedem Docker-Host. TrueNAS SCALE hat unter [TrueNAS SCALE](#truenas) eigene Schritte.

1. Hol dir die fertig zu bearbeitende [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) aus dem Repo.
2. Setze `APP_KEY` (siehe unten) und richte das Host-Data-Volume auf deine echte Datenwurzel. Die Kommentare der Datei führen durch beides.
3. `docker compose up -d`, dann öffne `https://<host-ip>:3443/`.

Was anders ist als auf Unraid:

- **Kein Bereich Flash.** Es gibt keinen Boot-USB-Stick zum Erfassen oder Wiederherstellen, der Bereich Flash hat hier also nichts zu tun. Reicht Host Data stattdessen bis zum `/etc` des Hosts, bietet **Hinzufügen** auf der Seite Sicherungen die **Systemkonfiguration** als Ordnervorschlag an: ein vorbereitetes `/etc`-Ordner-Set, das du vor dem Speichern prüfst und bearbeitest.
- **Keine Unraid-eigenen Benachrichtigungen.** Die eigenen Benachrichtigungskanäle von BombVault in der App (Webhook, Warnungen bei Off-site-Fehlern usw.) funktionieren wie gewohnt. Nur der Unraid-spezifische Push an dessen eigenes Benachrichtigungssystem entfällt, da es kein solches System gibt.
- **Die VM-Sicherung ist optional und braucht einen separaten libvirtd-Host, der über SSH erreichbar ist.** Siehe den auskommentierten Block in der Compose-Datei. In einen generischen Docker-Host selbst ist kein VM-Manager eingebaut.
- **Kein Dashboard-Widget.** Das BombVault Widget ist ein Unraid-Plugin, dieser Schritt entfällt also ebenfalls.
- **Die Daten eines Containers finden.** Ohne die `appdata`-Konvention von Unraid wird der Datenordner eines Containers anhand der Segmente in `DATA_ROOT_SEGMENTS`, benannter Docker-Volumes, des Arbeitsverzeichnisses eines Compose-Projekts und des Labels `bombvault.data` gefunden (siehe [Erkennung der Sicherungsquellen](configuration.md#backup-source-detection)). Benannte Volumes und die `/etc`-Vorlage erreichen nur Pfade innerhalb des Host-Data-Mounts, richte Host Data also auf einen gemeinsamen Vorfahren, der auch die Datenwurzel von Docker abdeckt.
- **`PLATFORM`.** Setze es auf `generic` oder `truenas`. Ungesetzt erkennt BombVault Unraid an seiner eigenen Markierung am Flash-Mount und behandelt alles andere als generisch, und die Unraid-only-Schritte werden übersprungen, statt versucht zu werden und zu scheitern.

### TrueNAS SCALE {#truenas}

BombVault ist noch nicht im TrueNAS-App-Katalog, installiere es also als benutzerdefinierte App: **Apps, Discover Apps**, dann **Install via YAML** im Menü neben **Custom App**. Das YAML unten setzt dasselbe wie der Katalogeintrag im Ordner `truenas-apps` des Repositorys. Ersetze `tank` durch deinen Pool und trage `APP_KEY` ein, wie der nächste Abschnitt es beschreibt.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # For VM backups:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

Öffne dann `https://<truenas-ip>:30528/`. Was unter [Was anders ist als auf Unraid](#generic-docker-host) steht, gilt auch hier, und auf TrueNAS außerdem:

- **Capabilities.** BombVault braucht die sieben oben. Die VM-Sicherung scheitert ohne `SETPCAP`, und ohne `SYS_CHROOT` wird jeder Container für seine ganze Sicherung angehalten, statt aus einem Snapshot gelesen zu werden.
- **Host Data** ist ein Host-Pfad auf einem Dataset, hier `/mnt/tank`, im selben Pfad in der App gemountet. Deshalb nennen `HOST_SOURCE_ROOT` und `HOST_MOUNT_ROOT` ihn beide. `rslave` reicht die Mounts weiter, die der Host später macht, was ZFS-Dataset-Sicherungen brauchen.
- **Daten in einem ixVolume werden nicht gesichert.** Eine App, deren Speicher ein ixVolume ist, hält ihre Daten unter `/mnt/.ix-apps`, außerhalb von Host Data. BombVault sichert nur die Definition der App, und der Lauf nennt die Ordner, die er ausgelassen hat. Wähle für jede App, deren Daten gesichert werden sollen, den Speicher **Host Path** auf einem Dataset unterhalb von Host Data.
- **VM- und ZFS-Dataset-Sicherungen** erreichen TrueNAS über SSH als root. Schalte den SSH-Dienst unter **System, Services** ein und füge den öffentlichen Schlüssel aus **Einstellungen, Anbindungen, Host-SSH** dem Benutzer root unter **Credentials, Users** hinzu. VM-Sicherungen brauchen außerdem `LIBVIRT_URI` wie oben, weil der libvirtd von TrueNAS auf einem eigenen Socket lauscht. Sind weder `LIBVIRT_URI` noch `LIBVIRT_HOST` gesetzt, nutzt BombVault das Standard-Gateway der App als Host, da sich `host.docker.internal` in einer TrueNAS-App nicht auflöst. Die [Anleitung zur VM-Sicherung](vm-backup-ssh-setup.md#truenas-scale) behandelt die Einzelheiten, auch VMs auf zvols.
- **Eine ZFS-Replik nutzt einen anderen Schlüssel.** Eine Replik auf einem zweiten ZFS-Server meldet sich mit dem Schlüssel an, den der Replik-Dialog zeigt, nicht mit dem Host-SSH-Schlüssel. Füge diesen auf dem empfangenden Server hinzu.
- **Eine gelöschte App kommt in zwei Schritten zurück.** Installiere sie in TrueNAS zuerst unter demselben Namen neu, was ihr Netzwerk wieder anlegt, und stelle dann ihre Daten in BombVault wieder her. Bis dahin meldet die Wiederherstellungsprüfung das fehlende Netzwerk, und die Wiederherstellung lehnt ab, bevor sie etwas ändert.
- **Neben das Original** legt einen schlichten Docker-Container an. Er läuft, aber der Bildschirm Apps von TrueNAS listet ihn nicht auf.
- **Datenbank-Dumps** funktionieren auf Dataset-Mounts. Ein Import in einen Datenordner, der ein eigenes Dataset ist, verschiebt den alten Inhalt in einen Ordner daneben, das darüberliegende Dataset braucht also Platz dafür.

### ZimaOS, CasaOS und andere Hosts {#other-hosts}

- **ZimaOS und CasaOS:** Füge den [Community-Store](https://github.com/junkerderprovinz/zimaos-apps#install) hinzu und installiere BombVault daraus. Es läuft dort als `PLATFORM=generic` mit Host Data auf `/DATA`. ZimaOS hält App-Daten in `/DATA/AppData/<app>`, was BombVault von selbst findet, da `DATA_ROOT_SEGMENTS` `appdata` in jeder Schreibweise trifft.
- **MOS:** Installiere BombVault aus dem [MOS Hub](https://mos-official.net/apps.html?search=BombVault). Es läuft dort als `PLATFORM=generic` mit Host Data auf `/mnt/cache`.
- **Synology:** Lege im Container Manager ein Projekt aus der Compose-Datei an (DSM 7.2 oder neuer).
- **Proxmox:** Betreibe es in einer VM oder einem LXC-Container, der Docker hat.

Ohne Kommentare ist die generische Compose-Datei kurz. Ersetze `./data` durch den Host-Ordner, der die Daten deiner Container enthält, setze `HOST_SOURCE_ROOT` auf denselben Pfad und trage `APP_KEY` ein:

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    restart: unless-stopped
    hostname: bombvault
    container_name: bombvault
    volumes:
      - ./config:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/host/user
    environment:
      APP_KEY: ""
      PLATFORM: generic
      HOST_SOURCE_ROOT: /host/user
    ports:
      - "3443:3443"
```

Um auch benannte Docker-Volumes zu sichern, mounte einen Ordner, der sowohl deine Daten als auch die Datenwurzel von Docker enthält (`docker info -f '{{.DockerRootDir}}'`, meist `/var/lib/docker`), so wie das Unraid-Template ganz `/mnt` mountet.

## Die eine erforderliche Einstellung {#the-one-required-setting}

Die einzige Variable, die du setzen musst, ist `APP_KEY`, ein Hex-Geheimnis mit 32 Byte (64 Hex-Zeichen), aus dem das Passwort des restic-Repositorys abgeleitet wird.

Erzeuge eines auf einem beliebigen Rechner:

```bash
openssl rand -hex 32
```

Füge das Ergebnis in das Feld `APP_KEY` des Templates (Unraid) oder die Umgebungsvariable `APP_KEY` in `docker-compose.yml` (generischer Docker-Host) ein.

!!! danger "Verliere deinen APP_KEY nicht"
    Geht `APP_KEY` verloren, sind deine verschlüsselten Sicherungen unwiederbringlich. Bewahre ihn an einem sicheren Ort getrennt vom Server auf. Sobald BombVault läuft, setze ein Login-Passwort und lade das **Recovery-Kit** unter **Einstellungen, Wiederherstellung** herunter (siehe [Off-site und Wiederherstellung](offsite-recovery.md#recovery-kit)). Es enthält den Schlüssel, jeden Speicherort und die restic-Befehle.

Das Template mountet außerdem den Docker-Socket, den Flash (`/boot`) und die **Host Data**-Wurzel (`/mnt`) für dich. Sicherungs-*Quellen* und -*Ziele* liegen beide unter Host Data. Die vollständige Variablenreferenz und die Off-site-Einrichtung stehen unter [Konfiguration](configuration.md).

## Erster Start {#first-run}

![Die Willkommensseite einer neuen Installation, mit dem, was BombVault auf dem Server gefunden hat.](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![Die Willkommensseite einer neuen Installation, mit dem, was BombVault auf dem Server gefunden hat.](assets/screenshots/getting-started/welcome-light.png#only-light)

1. Öffne die Web-UI unter `https://<deine-unraid-ip>:3443` (selbstsigniertes Zertifikat ab Werk). Eine neue Installation öffnet sich auf **Willkommen bei BombVault**, was auflistet, was BombVault auf dem Server gefunden hat.
2. Wähle **Los geht’s**. Die Übersicht zeigt dann die Karte **Erste Schritte** mit drei Schritten:
    1. **Bereiche wählen**: Schalte ein, was BombVault sichert (Container, VMs, Flash, Ordner, ZFS-Datasets, Selbstbackup). **Auswählen** öffnet die Liste der Einträge eines Bereichs, wo **Zeitplan pausieren** einen auslässt. Die Bereiche, die du hier einschaltest, laufen jede Nacht, was du unter **Einstellungen, Zeitplan** änderst.
    2. **Speicherort hinzufügen**: ein zweiter Ort für die Sicherungen. Jeder Bereich hat schon einen Ordner auf diesem Server, `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`, bei der ersten Sicherung angelegt. Am sichersten ist ein zweiter Speicherort außer Haus, etwa eine Storage Box oder Backblaze B2.
    3. **Erste Sicherung**: Danach läuft sie jede Nacht von selbst. Die Karte bleibt, bis die erste Sicherung durchgelaufen ist.
3. Öffne **Sicherungen**, um jeden Eintrag in einer Liste zu sehen. **Jetzt alles sichern** startet einen Lauf über alle eingeschalteten Bereiche, und **Jetzt sichern** auf der Seite eines Eintrags sichert nur diesen.

![Die Karte Erste Schritte mit ihren drei Schritten, der zweite geöffnet.](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![Die Karte Erste Schritte mit ihren drei Schritten, der zweite geöffnet.](assets/screenshots/getting-started/first-steps-light.png#only-light)

BombVault schon früher benutzt? **Aus einer Sicherung wiederherstellen** auf der Willkommensseite öffnet die Seite Wiederherstellung, wo der **Notfall-Assistent** die Einstellungen, Container, VMs und Ordner eines verlorenen Servers zurückholt. Siehe [Off-site und Wiederherstellung](offsite-recovery.md#emergency).

!!! tip "Optional: eine Sicherungsreihenfolge festlegen"
    Sollen manche Container immer vor anderen gesichert werden (zum Beispiel eine Datenbank vor der App, die sie nutzt), öffne die Karte **Reihenfolge** unter **Einstellungen, Zeitplan** und zieh sie in die gewünschte Folge. Geplante Läufe und das Gesamt-Backup richten sich dann danach. Was du ohne Reihenfolge lässt, wird nach dem am längsten Überfälligen zuerst gesichert.

## Beispiel: deine erste Container-Sicherung von A bis Z {#example-first-backup}

Das führt durch eine frische Unraid-Box mit ein paar Containern, darunter `plex`, bis die erste Sicherung fertig ist und du weißt, wo du sie findest. Es dauert etwa zehn Minuten plus die Zeit der ersten Sicherung.

1. **Installieren.** Suche im Unraid-Tab **Apps** nach **BombVault** und drücke **Install**.
2. **Den Schlüssel setzen.** Führe auf einem Rechner `openssl rand -hex 32` aus und füge die 64 Zeichen in das Feld `APP_KEY` des Templates ein. Leg gleich jetzt eine Kopie in deinem Passwortmanager ab, vor allem anderen. Drücke **Apply**.
3. **Öffnen.** Gehe zu `https://<deine-unraid-ip>:3443` und akzeptiere das selbstsignierte Zertifikat. **Willkommen bei BombVault** listet auf, was es gefunden hat, etwa 12 Container, 2 VMs und den Flash.
4. **Los geht’s.** Die Übersicht öffnet sich mit der Karte **Erste Schritte**.
5. **Bereiche wählen.** **Container** ist von Anfang an an. Schalte ein, was du sonst willst, etwa **Flash**. **Auswählen** öffnet die Liste der gefundenen Container, wo **Zeitplan pausieren** einen auslässt, zum Beispiel einen Testcontainer, der dir egal ist. Eingeschaltete Bereiche laufen jede Nacht im Gesamt-Backup.
6. **Speicherort hinzufügen.** Jeder Bereich hat schon einen eigenen Ordner auf diesem Server, unterhalb von `/mnt/user/bombvault`, die erste Sicherung hat also ein Ziel. Dieser Schritt fügt einen zweiten Ort hinzu, und der sicherste liegt außer Haus. Das kannst du jetzt mit dem [S3-Beispiel](offsite-recovery.md#example-s3) tun oder zurückkommen, wenn die erste Sicherung durch ist. Die Karte bleibt bis dahin.
7. **Erste Sicherung.** Öffne **Sicherungen** und drücke **Jetzt alles sichern**, oder warte auf die Nacht. Der Ring im Statuskopf wird zum Fortschrittsring und zählt die Einträge, etwa **Sichert gerade: 3 von 12**. Jeder Container wird angehalten, während seine Daten gelesen werden, und gleich danach wieder gestartet. Ein Container, dessen Daten auf ZFS liegen, steht nur die wenigen Sekunden eines Snapshots still. Die erste Sicherung liest alles und dauert daher am längsten. Spätere speichern nur, was sich geändert hat.
8. **Prüfen.** Sagt der Ring **Alles gesichert**, öffne **Verlauf** in der Übersicht: Jeder Container hat unter **Läufe** eine grüne Zeile. Öffne **Sicherungen**, klicke `plex` an und sieh dir seinen Tab **Sicherungen** an: Der erste Snapshot ist da, mit **Lokal** als dem Ort, der ihn hält. Ein paar Minuten später zeigt sein Tab **Prüfung** die Wiederherstellungsprüfung, die eine Stichprobe der Sicherung wiederhergestellt und zurückgelesen hat.
9. **Den Schlüssel schützen.** Öffne **Einstellungen, Sicherheit** und setze ein Login-Passwort. Öffne dann **Einstellungen, Wiederherstellung** und drücke in **Für den Notfall** **Recovery-Kit herunterladen**. Bewahre die Datei außerhalb des Servers auf, neben der Kopie von `APP_KEY` aus Schritt 2. Die Erinnerung **Sichere dein Recovery-Kit** in der Übersicht verschwindet, sobald du **Sicher aufbewahrt** drückst.

Ab jetzt werden die Container jede Nacht gesichert. Gute nächste Schritte, in dieser Reihenfolge: ein [Off-site-Speicherort](offsite-recovery.md#example-s3), damit ein toter Server die Sicherungen nicht mitnimmt, dann [Benachrichtigungen](notifications.md), damit dich ein Fehler erreicht, dann ein Blick in die [Zeitpläne](schedules.md), falls dir 03:00 nicht passt.

![Der Statuskopf, während die erste Sicherung läuft, mit dem Fortschrittsring und seinen Stationen.](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![Der Statuskopf, während die erste Sicherung läuft, mit dem Fortschrittsring und seinen Stationen.](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "Host-Integrations-Check"
    Die Karte **Host-Integration** in der Übersicht prüft jeden Mount und jedes CLI (Docker-Socket, libvirt, restic, qemu-img, rclone) und meldet fehlende Teile, damit du bestätigen kannst, dass der Container richtig verdrahtet ist, bevor du dich darauf verlässt. Sie wartet unter **Anpassen**, bis du sie hinzufügst.

## Sich zurechtfinden {#navigation}

![Die Einstellungen als Kacheln, daneben die Karten der gewählten Kachel. Jede Änderung wird beim Machen geschrieben.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Die Einstellungen als Kacheln, daneben die Karten der gewählten Kachel. Jede Änderung wird beim Machen geschrieben.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Die Seitenleiste hat fünf Einträge: **Übersicht**, **Anomalien** (solange die Anomalie-Erkennung an ist), **Sicherungen**, **Wiederherstellung** und, unter **Abmelden**, **Einstellungen**. Die Hauptaktion einer Seite schwebt unten rechts, und **Mehr** enthält die seltenen. Auf einem Handy enthält die Leiste Übersicht, Sicherungen, Wiederherstellung und Einstellungen, und **Mehr** hat den Rest.

Seiten und Einstellungen nutzen die ganze Breite des Fensters. Ein Fenster hält seine Knöpfe in seinem Fuß, der nach vorn führende rechts. Auf einem Handy behält ein Fuß mit mehr als drei Knöpfen den Ausweg und die Hauptaktion und legt die anderen unter **Mehr**.

![Auf einem Handy: die Leiste unten, Mehr geöffnet.](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![Auf einem Handy: die Leiste unten, Mehr geöffnet.](assets/screenshots/getting-started/phone-more-light.png#only-light)

- Die **Übersicht** zeigt, ob alles gesichert ist und was dich braucht. Siehe [Übersicht](overview.md).
- **Sicherungen** listet jeden Container, jede VM, jedes Ordner-Set, den Flash, jedes ZFS-Dataset und das Selbstbackup als Einträge auf. Kacheln über der Liste wählen eine Art, und **Filter**, **Sortieren** und **Suchen** grenzen sie ein. Jeder Eintrag hat eine eigene Seite mit drei Tabs: **Sicherungen**, **Einstellungen** und **Prüfung**. Ein Zurück-Knopf links vom Symbol des Eintrags führt zur Liste. Siehe [Sicherungen und Einträge](backups.md).
- **Wiederherstellung** ist eine Seite für alles: Wähle einen Bereich, suche einen Eintrag und wähle eine Sicherung. Auch der Notfall-Assistent und die anderen Quellen starten hier. Siehe [Wiederherstellung](restore.md).
- Die **Einstellungen** sind vierzehn Kacheln: Allgemein, Aussehen, Zeitplan, Speicherorte, Wiederherstellung, Benachrichtigungen, Prüfung, Sicherheit, Instanzen, Kopplung, Anbindungen, Apps, System und Info. Siehe [Einstellungen](settings.md). Drücke `/` oder Strg+K, um alle zu durchsuchen, auch nach den Namen, die Seiten in früheren Versionen hatten. Zieh die Kacheln unter **Aussehen** in deine eigene Reihenfolge. Wo eine Kachelspalte neben der Seite sie zusammendrücken würde, etwa in einem Fenster von etwa 1200 Pixeln Breite, werden die Kacheln zu einem Streifen über der Seite, der seitlich scrollt. Ein Handy zeigt denselben Streifen.
- **Speicherorte** und **Instanzen** sind Einstellungskacheln mit eigenen Seiten. **Auch in der Seitenleiste zeigen** auf ihrer Kachel heftet sie an die Seitenleiste.

Nichts hat einen Speichern-Knopf, und nichts klappt auf: Seltenes hat eine eigene Karte oder ein Fenster, und Erklärungen stehen in den (i)-Blasen. Nach einem Update öffnet sich von selbst ein Fenster mit einer Karte für jede Version seit deiner letzten, und grüne Punkte markieren, was sich geändert hat. Ein Klick blendet einen Punkt aus. **Neu** in der Karte Version unter **Einstellungen, Info** zählt, was du noch nicht gesehen hast, und öffnet dieses Fenster wieder, mit den Hinweisen jeder Version und ihren Änderungen mit **Hingehen**.

## Aus dem Quellcode bauen {#build-from-source}

BombVault ist eine einzelne statische Go-Binärdatei, die eine JSON-API und eine eingebettete React-Oberfläche liefert. Baue zuerst die Oberfläche und starte dann die Binärdatei:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

Der Bau der Oberfläche ist auch für `go run` nötig. Das Repository verfolgt unter `web/dist` nur eine leere Markierung, ohne `npm --prefix web run build` bettet die Binärdatei also nichts ein und antwortet `500 SPA index not found`, was zu erwarten ist. Docker, libvirt und Unraid lassen sich in der CI nicht testen, prüfe daher Mounts, restic und die VM-SSH-Verbindung auf einem echten Host mit der Karte **Host-Integration** der Übersicht (oder `GET /api/spike`), bevor du einen Pull Request öffnest.

## Nächste Schritte {#next-steps}

- Füge einen oder mehrere Speicherorte außer Haus hinzu (jeder Bereich kann auf mehrere zugleich kopieren) und sichere dein Recovery-Kit, wie unter **[Off-site und Wiederherstellung](offsite-recovery.md)** beschrieben. Das [S3-Beispiel](offsite-recovery.md#example-s3) führt durch einen.
- Sorge mit **[Benachrichtigungen](notifications.md)** dafür, dass dich ein Fehler erreicht.
- Prüfe unter **[Zeitpläne](schedules.md)**, wann was läuft.
- Bring mit der **[Android-App](android.md)** jeden Server deiner Gruppe aufs Handy.
- Die **[Praxisbeispiele](guides.md)** führen durch die üblichen Aufgaben, und **[Funktionen](features.md)** listet alles auf, was BombVault kann.
- Eine Einrichtung klonen oder auf eine neue Box umziehen? Nimm deine ganze Konfiguration mit der Karte **Einstellungen exportieren / importieren** mit. Siehe [Konfiguration](configuration.md#portable-settings-export-and-import).
- Hakt etwas? Siehe **[Fehlersuche](troubleshooting.md)**.
