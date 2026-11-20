# Knoten

Ein Knoten ist ein BombVault ohne Weboberfläche, etwa auf einem VPS oder auf einem kleinen Server bei einem Freund. Er sichert die Container und Ordner auf seinem eigenen Server. Das BombVault, auf dem du ihn hinzufügst, auf dieser Seite **Heiminstanz** genannt, sagt ihm, wann er sichert und wohin die Sicherungen gehen. Du verwaltest ihn in der Weboberfläche der Heiminstanz, gleich neben deinen eigenen Einträgen.

![Die Karte des Knotens vps-falkenstein auf der Seite Instanzen: Verbunden, seine Version, seine Sicherungen auf dem Empfangsserver, wann jeder Bereich sichert, und Ziel ändern, Ersetzen und Freigeben.](assets/screenshots/nodes/card-dark.png#only-dark)
![Die Karte des Knotens vps-falkenstein auf der Seite Instanzen: Verbunden, seine Version, seine Sicherungen auf dem Empfangsserver, wann jeder Bereich sichert, und Ziel ändern, Ersetzen und Freigeben.](assets/screenshots/nodes/card-light.png#only-light)

## So funktioniert es {#how-it-works}

- **Die Heiminstanz hält den Schlüssel des Knotens.** Wenn du einen Knoten hinzufügst, erzeugt die Heiminstanz seinen `APP_KEY`, behält eine versiegelte Kopie und gibt ihn dir ein einziges Mal, in der Compose-Datei des Knotens. Mit dieser Kopie erreicht die Heiminstanz den Knoten und öffnet seine Sicherungen.
- **Der Knoten braucht keinen offenen Port.** Heiminstanz und Knoten sprechen über das Relay miteinander, über eine eigene private Verbindung, getrennt von deiner Koppelgruppe. Die Heiminstanz beginnt jeden Aufruf und signiert ihn, und der Knoten schickt ihr nichts außer seinem Hallo. Das einzige Ziel, das einen offenen Port braucht, ist [Auf dem Knoten aufbewahren](#keep-on-node).
- **Sicherungen laufen nie über das Relay.** Der Knoten schickt sie direkt an den Ort, den du gewählt hast: den Empfangsserver der Heiminstanz, einen ihrer Speicherorte oder einen rest-server auf dem Knoten selbst.
- **Der Knoten plant nichts selbst.** Er hat keine eigenen Zeitpläne, schickt keine Benachrichtigungen und tritt keiner Gruppe und keiner Festung bei. Die Heiminstanz startet seine Sicherungen, verfolgt, wie sie laufen, und schickt die Alarme.
- **Ein Knoten sichert Container und Ordner-Sets.** VMs, der Flash, ZFS-Datasets und das Selbstbackup bleiben vollen Installationen vorbehalten.

Die Verbindung nutzt das eigene Relay der Heiminstanz, wenn ihre Gruppe eines hat, sonst das Relay des Projekts. Das gilt auch, wenn die Heiminstanz ohne Relay koppelt.

## Bevor du anfängst {#before-you-start}

- **Ein Login-Passwort auf der Heiminstanz.** Einen Knoten hinzufügen, seine Compose-Datei erneut zeigen und seine Einträge öffnen geht nur mit einer Anmeldung per Passwort, siehe [Login](configuration.md#login).
- **Docker mit Compose auf dem Server des Knotens.**
- **Ein Ort für die Sicherungen**, einer der drei unter [Wohin die Sicherungen gehen](#placement). Der Empfangsserver ist die sicherste Wahl, weil der Knoten dort nichts löschen kann.

## Einen Knoten hinzufügen {#add}

1. Öffne auf der Heiminstanz **Instanzen** und wähle unten rechts **Knoten hinzufügen**.
2. **Name.** Gib ein, wie der Knoten hier heißen soll, zum Beispiel wie der Server, und wähle **Weiter**.

    ![Das Fenster Knoten hinzufügen im ersten Schritt, mit dem Namen vps-helsinki.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![Das Fenster Knoten hinzufügen im ersten Schritt, mit dem Namen vps-helsinki.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Sicherungen.** Wähle, wohin die Sicherungen gehen, und fülle aus, was dieser Ort braucht. **Sichert täglich um** ist die Zeit, zu der beide Bereiche starten. Die Heiminstanz schlägt eine halbe Stunde nach ihren eigenen Container-Sicherungen vor, damit nicht beide zugleich auf dieselben Platten zugreifen. Auf der Karte kannst du später jedem Bereich einen eigenen Plan geben. Wähle **Weiter**.

    ![Der zweite Schritt: der Empfangsserver gewählt, Adresse für Knoten mit einer https-Adresse ausgefüllt und Sichert täglich um 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![Der zweite Schritt: der Empfangsserver gewählt, Adresse für Knoten mit einer https-Adresse ausgefüllt und Sichert täglich um 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Starten.** Das Fenster zeigt die Compose-Datei des Knotens, mit `MODE`, `APP_KEY`, `NODE_HOME` und `NODE_NAME` schon ausgefüllt. Kopiere sie und bewahre den `APP_KEY` sicher auf. Auf dem Server des Knotens:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # paste the file from the window
    docker compose up -d
    ```

    Richte vor dem Start das Volume Host Data auf die Daten, die gesichert werden sollen, und setze `HOST_SOURCE_ROOT` auf denselben Pfad, wie im Beispiel unten.

5. Das Fenster wartet auf den Knoten. Sobald er antwortet, sagt die Heiminstanz ihm, wohin seine Sicherungen gehen, und prüft, ob er sie dort erreicht. Das Fenster meldet dann **vps-helsinki ist online und nimmt Aufträge dieser Instanz an.**

    ![Der letzte Schritt: die Compose-Datei, darunter der Knoten online und seine Sicherungen auf dem Empfangsserver.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![Der letzte Schritt: die Compose-Datei, darunter der Knoten online und seine Sicherungen auf dem Empfangsserver.](assets/screenshots/nodes/add-start-light.png#only-light)

Du kannst das Fenster schließen, bevor der Knoten antwortet. Seine Karte sagt dann **Wartet auf den Knoten**, und die Heiminstanz teilt ihm sein Ziel mit, sobald er auftaucht.

### Beispiel: ein VPS, der auf den Empfangsserver sichert {#example-vps}

Die Heiminstanz ist eine Unraid-Box namens `tower`, auf der der [Empfangsserver](offsite-recovery.md#receiving-server) läuft. Ein Reverse Proxy macht ihn als `https://backup.tower.example.org` erreichbar. Auf dem VPS `vps-helsinki` laufen Nextcloud und Caddy, ihre Daten liegen unter `/srv/appdata`.

1. Auf `tower`: **Instanzen**, **Knoten hinzufügen**, Name `vps-helsinki`, **Weiter**.
2. Wähle **Empfangsserver dieser Instanz** und gib unter **Adresse für Knoten** `https://backup.tower.example.org` ein. Die Container der Heiminstanz sichern um 02:30, also schlägt das Fenster 03:00 vor. Lass es dabei und wähle **Weiter**.
3. Speichere auf dem VPS die Datei aus dem Fenster, mit den Volumes und `HOST_SOURCE_ROOT` passend zu `/srv/appdata`:

    ```yaml
    services:
      bombvault-node:
        image: junkerderprovinz/bombvault:latest
        restart: unless-stopped
        hostname: bombvault-node
        container_name: bombvault-node
        volumes:
          - ./config:/config
          - /var/run/docker.sock:/var/run/docker.sock
          - /srv/appdata:/host/user
        environment:
          MODE: node
          APP_KEY: "<filled in by home>"
          NODE_HOME: "<filled in by home>"
          NODE_NAME: "vps-helsinki"
          PLATFORM: generic
          HOST_SOURCE_ROOT: /srv/appdata
    ```

4. `docker compose up -d`. Ein paar Sekunden später meldet das Fenster auf `tower`, dass der Knoten online ist, und **Die Sicherungen gehen an https://backup.tower.example.org.**
5. Um 03:00 startet die Heiminstanz die Sicherungen des Knotens. Am nächsten Morgen zeigt die Karte **Knoten** in der Übersicht die letzte Sicherung von `vps-helsinki`, und **Sicherungen** listet seine Container in einem eigenen Abschnitt.

Wie BombVault die Daten eines Containers auf einem schlichten Docker-Host findet, steht unter [Erkennung der Sicherungsquellen](configuration.md#backup-source-detection).

## Wohin die Sicherungen gehen {#placement}

Jeder Knoten hat ein Ziel für beide Bereiche. **Ziel ändern** auf seiner Karte ändert es, sobald der Knoten online ist. Am neuen Ort entstehen neue Repositories, und die Sicherungen am alten Ort bleiben, wo sie sind.

### Empfangsserver dieser Instanz {#to-receiver}

Der Knoten schreibt im Append-only-Modus auf den Empfangsserver der Heiminstanz: Er kann Sicherungen hinzufügen, aber keine löschen. Er bekommt dort ein eigenes Login, benannt nach dem Knoten, und ein Repository pro Bereich in seinem eigenen Ordner. Die Heiminstanz fügt jedem dieser Repositories ihren eigenen Schlüssel hinzu und räumt sie selbst auf, weil der Knoten das nicht kann.

Der Knoten erreicht den Server unter der **Adresse für Knoten**, die du im zweiten Schritt oder im Fenster **Empfangsserver** setzt. Ein Knoten sendet meist aus dem Internet, die Adresse muss also eine von diesen sein:

- `https://`, etwa hinter einem Reverse Proxy, Pangolin oder einem Tunnel. Zeigt der Server ein eigenes Zertifikat statt eines von einer öffentlichen Stelle, füge es im PEM-Format unter **Zertifikat** ein.
- `http://` mit einer privaten IP-Adresse, etwa `10.0.0.5` in einer WireGuard-Verbindung oder `100.101.102.103` bei Tailscale.

Einfaches `http://` an eine öffentliche Adresse lehnt BombVault ab, weil der Knoten seine Sicherungen sonst unverschlüsselt durchs Internet schicken würde. Änderst du die Adresse, sagt die Heiminstanz es jedem Knoten am Empfangsserver, und ein Knoten, der gerade nicht da ist, erfährt es, sobald er zurück ist.

### Ein Speicherort {#to-location}

Der Knoten schreibt auf einen Speicherort der Heiminstanz, den er selbst erreicht: einen S3-Dienst oder einen REST-Server, über `https://` oder unter einer privaten Adresse. Seine Repositories liegen dort in `bombvault-nodes/<node id>/containers` und `bombvault-nodes/<node id>/files`.

Der Knoten meldet sich mit eigenen [gespeicherten Zugangsdaten](offsite-recovery.md#sign-ins) an. Sie müssen andere sein als die, mit denen die Heiminstanz auf den Speicherort schreibt. Sagt das Fenster, dass es keine gibt, lege zuerst welche an. Anders als auf dem Empfangsserver kann der Knoten mit diesen Zugangsdaten meist löschen, was am Speicherort liegt, und das Fenster weist darauf hin.

### Auf dem Knoten aufbewahren {#keep-on-node}

Der Knoten bewahrt seine Sicherungen selbst auf, hinter einem eigenen rest-server im Append-only-Modus, und die Heiminstanz holt sich von jedem Bereich eine Kopie in ihren eigenen Speicher. Das passt zu einem Knoten, dem die Heiminstanz keinen Platz anbieten kann. Es ist das einzige Ziel, das einen offenen Port braucht.

![Der zweite Schritt mit Auf dem Knoten aufbewahren gewählt, der Adresse https://203.0.113.7:8443 und dem Hinweis zum offenen Port.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![Der zweite Schritt mit Auf dem Knoten aufbewahren gewählt, der Adresse https://203.0.113.7:8443 und dem Hinweis zum offenen Port.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. Gib unter **Adresse des Knotens** ein, wo die Heiminstanz den Knoten erreicht: seine öffentliche Adresse und einen freien Port, etwa `https://203.0.113.7:8443`, ohne Ordner dahinter. Nimm `http://` nur mit einer privaten Adresse, wie oben.
2. Der Knoten startet auf diesem Port den Container `bombvault-node-rest-server`, mit seinen Repositories im Ordner `bombvault-node` unter Host Data. Bei `https://` erstellt er ein Zertifikat für diese Adresse, und die Heiminstanz vertraut danach nur diesem Zertifikat.
3. Gib den Port in der Firewall des Knotens frei, damit die Heiminstanz ihn erreicht. Belegt schon ein anderer Container auf dem Knoten den Port, lehnt BombVault ihn ab und nennt den Container.

Die Heiminstanz holt einen Bereich, sobald der Knoten ihn gesichert hat, und zur Sicherheit zusätzlich jede Stunde um Viertel nach, auch wenn **Holen** ausgeschaltet ist. Was die Heiminstanz geholt hat, bleibt bei ihr wie jede [geholte Kopie](offsite-recovery.md#receiver), auch wenn der Knoten nicht mehr da ist. Der Knoten räumt seine eigenen Repositories auf, wenn die Heiminstanz ihn darum bittet, weil sein rest-server kein Löschen annimmt. Seine Ordner-Sets lassen den Ordner `bombvault-node` aus, damit er nie seine eigenen Sicherungen sichert.

## Wann Knoten sichern {#schedule}

Die Heiminstanz bestimmt den Zeitplan jedes Knotens. Die Karte zeigt einen Plan pro Bereich, etwa **Container** *Täglich um 2:00* und **Ordner** *Wöchentlich (So) um 4:00*. Ein Klick öffnet denselben Editor wie unter [Zeitpläne](schedules.md#plans), und eine Änderung ist sofort gespeichert.

![Der Plan für die Container eines Knotens, von seiner Karte aus geöffnet, mit Aus, Täglich, Wöchentlich, Alle N Tage und Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![Der Plan für die Container eines Knotens, von seiner Karte aus geöffnet, mit Aus, Täglich, Wöchentlich, Alle N Tage und Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **Ein Knoten, der weg war,** holt die verpassten Sicherungen nach, sobald er zurück ist. Die Heiminstanz schaut alle fünf Minuten nach.
- **Ein Knoten, der gerade beschäftigt ist,** wird fünf Minuten später noch einmal gefragt.
- **Während die Heiminstanz die alten Sicherungen eines Knotens aufräumt**, startet der Knoten nichts, bis sie fertig ist. Auf dem Empfangsserver und auf dem Knoten selbst öffnet sich dieses Fenster jeden Sonntag um 03:00 und dauert bis zu sechs Stunden. An einem Speicherort richtet es sich nach dem Plan der Off-site-Kopie, den die Heiminstanz für diesen Ort hat. Die Heiminstanz behält die Sicherungen nach ihrer eigenen Regel für den Bereich unter **Wie lange aufheben**, oder nach der Regel des Speicherorts, wo er eine hat.
- **In der Liste der Läufe des Knotens** stehen die Sicherungen und Wiederherstellungen, die die Heiminstanz gestartet hat, als von ihr gestartet.

## Deine Knoten im Blick {#watch}

Die Karte **Knoten** in der Übersicht listet jeden Knoten mit seinem Zustand, seiner letzten Sicherung, seinen Fehlern und seinen offenen Anomalien. Sie erscheint, sobald du einen Knoten hast.

![Die Karte Knoten in der Übersicht mit drei Knoten: einer verbunden, einer inaktiv mit zwei Fehlern und einer, auf den noch gewartet wird.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![Die Karte Knoten in der Übersicht mit drei Knoten: einer verbunden, einer inaktiv mit zwei Fehlern und einer, auf den noch gewartet wird.](assets/screenshots/nodes/overview-card-light.png#only-light)

Die Heiminstanz schickt über ihre [Benachrichtigungskanäle](notifications.md) einen Alarm pro Vorfall und den nächsten erst, wenn dieser Vorfall vorbei ist:

- **Eine Sicherung ist fehlgeschlagen**, etwa *BombVault: vps-helsinki: backup failed*, mit dem Eintrag und dem Grund. Klappt eine spätere Sicherung dieses Eintrags, ist der Vorfall vorbei.
- **Der Knoten ist nicht erreichbar**, etwa *BombVault: vps-helsinki is out of reach*. Die Heiminstanz wartet so lange, wie der kürzeste Totmannschalter der Repositories des Knotens vorgibt: 26 Stunden, wenn du nichts änderst, oder 6 Stunden, solange der Knoten noch kein Ziel hat.

Die Empfänger-Überwachung der Heiminstanz kümmert sich außerdem um jedes Repository eines Knotens, mit einem Totmannschalter und einem täglichen `restic check` um 04:00, siehe [Empfänger und Abholer](offsite-recovery.md#receiver).

## Die Einträge eines Knotens {#entries}

Unter **Sicherungen** haben die Listen Container und Ordner unter deinen eigenen Einträgen einen Abschnitt für jeden Knoten. Der eigene Container des Knotens fehlt darin. **Jetzt sichern** im Menü eines Eintrags startet seine Sicherung auf dem Knoten, und **Öffnen** öffnet seine Seite.

![Der Abschnitt des Knotens vps-falkenstein in der Liste Container: nextcloud, nextcloud-db, caddy und uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![Der Abschnitt des Knotens vps-falkenstein in der Liste Container: nextcloud, nextcloud-db, caddy und uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

Die Seite eines Eintrags auf einem Knoten sagt **Auf** und den Namen des Knotens und listet seine Sicherungen. **Wiederherstellen** bei einer davon stellt den Eintrag auf dem Knoten wieder her und ersetzt, was jetzt dort ist. Ein Container wird dafür angehalten, wie auf der Heiminstanz.

![Die Seite von nextcloud auf vps-falkenstein mit seinen Sicherungen und einem Knopf Wiederherstellen neben jeder.](assets/screenshots/nodes/entry-dark.png#only-dark)
![Die Seite von nextcloud auf vps-falkenstein mit seinen Sicherungen und einem Knopf Wiederherstellen neben jeder.](assets/screenshots/nodes/entry-light.png#only-light)

Über die Heiminstanz erreichst du eine feste Liste von Routen des Knotens: seine Container, Ordner-Sets, Läufe und Anomalien und das Sichern und Wiederherstellen davon. Einstellungen, Schlüssel und Speicher des Knotens bleiben außer Reichweite, und nichts auf dieser Liste löscht etwas. Siehe [Knoten in der API](api.md#nodes).

## Ersetzen und freigeben {#replace-release}

**Ersetzen** zeigt die Compose-Datei des Knotens noch einmal, nach einer Anmeldung mit dem Passwort. Damit ziehst du den Knoten auf einen anderen Server um oder baust ihn neu auf: Starte die Datei dort und fahre den alten Server herunter. Derselbe `APP_KEY` und dasselbe `NODE_HOME` machen ihn wieder zu demselben Knoten, mit seinen Sicherungen und seinem Verlauf. Zwei Server mit demselben `APP_KEY` zählen als ein Knoten, lass also nie beide laufen.

**Freigeben** lässt einen Knoten gehen. Er nimmt keine Aufträge der Heiminstanz mehr an und sichert nichts mehr. Seine Sicherungen bleiben, wo sie sind, ebenso jede Kopie, die die Heiminstanz geholt hat. Sein Login auf dem Empfangsserver fällt weg, und die Heiminstanz überwacht seine Repositories nicht mehr. Ist ein Knoten beim Freigeben nicht da, behält er seine Karte als **Wird freigegeben**, bis er davon erfährt. Ein freigegebener Knoten nimmt mit demselben `APP_KEY` keine Heiminstanz mehr an. Willst du den Server wieder nutzen, füge ihn als neuen Knoten hinzu und starte ihn mit der neuen Compose-Datei und einem leeren Ordner `config`.

## Sicherheit {#security}

Wird die Heiminstanz übernommen, steuert der Angreifer auch ihre Knoten, und ein Knoten hat Docker auf seinem Server in der Hand, also Root. Schütze die Heiminstanz mit einem Passwort und halte sie vom Internet fern. Wird ein Knoten übernommen, bekommt der Angreifer keine Koppelgruppe, keine Festung, keinen anderen Knoten und kein anderes Repository und kann auf dem Empfangsserver keine bisherigen Sicherungen löschen. Die Einzelheiten stehen unter [Knoten im Sicherheitsmodell](configuration.md#nodes).

Das [Recovery-Kit](offsite-recovery.md#recovery-kit) hat ein Kapitel **Nodes** mit dem `APP_KEY` jedes Knotens, seinem restic-Passwort und seinen Repositories. Damit holst du einen Knoten zurück oder stellst seine Sicherungen mit schlichtem restic wieder her.
