# VM-Sicherungen

BombVault sichert KVM/libvirt-VMs, ohne einen libvirt-Pfad zu mounten. Es führt `virsh` über SSH (`qemu+ssh://`) auf dem Host aus, liest und schreibt die VM-Disks über den Host-Data-Mount (`/mnt` auf dem Host ist `/host/user` im Container) und kopiert das UEFI-NVRAM über SSH. Weil nichts, was zu libvirt gehört, per Bind-Mount eingebunden wird, kann BombVault dem Unraid VM Manager nie in die Quere kommen.

Diese Anleitung behandelt die Einrichtung, die Sicherungsmethoden, VMs mit Unraid-Snapshots und TrueNAS SCALE. Container-Sicherungen brauchen nichts davon.

ZFS-Dataset-Sicherungen nutzen dieselbe SSH-Verbindung, mit demselben Schlüssel, Host und Benutzer. Sobald die VM-Sicherungen sich verbinden, verbinden sich auch ZFS-Dataset-Einträge. Siehe [ZFS-Datasets](zfs-datasets.md).

## Was du einstellst {#what-you-configure}

| Einstellung | Wo | Vorgabe | Bedeutung |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | Template-Variable | `host.docker.internal` | Der Unraid-Host, über SSH erreicht |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | Template-Variable | `22` | Der SSH-Port von Unraid |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | Template-Variable | `root` | Der SSH-Benutzer auf dem Host |
| Öffentlicher Schlüssel | **Einstellungen, Anbindungen, Host-SSH** | beim ersten Start erzeugt | Gib ihn auf dem Host frei |

BombVault erzeugt sein SSH-Schlüsselpaar beim ersten Start unter `/config/ssh/id_ed25519`, abgelegt in Appdata. Der Schlüssel des Hosts wird bei der ersten Verbindung in `/config/ssh/known_hosts` festgehalten.

Das Template füllt **Host SSH: Address** mit dem Platzhalter `192.168.x.x`. BombVault behandelt das als nicht gesetzt und versucht `host.docker.internal`, was im Standardnetz `bridge` funktioniert. In einem eigenen Netz ersetze ihn durch die LAN-IP des Hosts (Schritt 3 B).

## Schritt 1: SSH auf dem Unraid-Host aktivieren {#step-1}

1. Öffne in Unraid **Settings, Management Access** und stelle **Use SSH** auf **Yes**.
2. Hast du den SSH-Port geändert, merke ihn dir. **Host SSH: Port** muss passen.

## Schritt 2: Den öffentlichen Schlüssel von BombVault freigeben {#step-2}

Das `/root` von Unraid liegt im Arbeitsspeicher und wird bei jedem Start gelöscht, der Schlüssel muss also zusätzlich auf dem Flash liegen. Führe das im Unraid-Terminal aus:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 und neuer kopieren `/boot/config/ssh/root.pubkeys` bei jedem Start in die `authorized_keys` von root, das übersteht also Neustarts. Es gilt für jeden `root`-Login, egal welchen Hostnamen oder Port BombVault nutzt.

## Schritt 3: Netzwerk {#step-3}

BombVault muss vom Container aus eine TCP-Verbindung zum SSH-Port des Hosts öffnen. Wähle den Fall, der zu deiner Einrichtung passt.

### A. BombVault im Standardnetz bridge {#bridge}

- Lass **Host SSH: Address** auf `host.docker.internal` oder dem Platzhalter des Templates. Das Template fügt `--add-host=host.docker.internal:host-gateway` hinzu, was auf das Gateway von docker0 zeigt, und das ist der Host.
- Setze **Host SSH: Port** auf deinen SSH-Port.

### B. BombVault in einem eigenen Netz {#custom-network}

Ein Container in `br0.x`, macvlan oder ipvlan erreicht den Host nicht über `host.docker.internal`, weil 172.17.0.1 docker0 ist und von `br0.x` aus nicht erreichbar. Stattdessen:

1. Stelle in Unraid **Settings, Docker, Host access to custom networks** auf **Enabled**.
2. Setze **Host SSH: Address** auf die LAN-IP des Unraid-Hosts, die Adresse, unter der du die Web-UI öffnest.
3. Liegen das Netz des Containers und der Host in verschiedenen VLANs, erlaube die Route auf deinem Router oder in der Firewall: vom Container-VLAN zur Host-IP auf dem SSH-Port, über TCP.
4. Lässt sich die LAN-IP des Hosts immer noch nicht erreichen, nimm die Shim-IP des Hosts im Subnetz des Containers. Du findest sie auf dem Host mit `ip -4 addr show | grep -B2 <container-subnet>`.

### Prüfen, ob der Host antwortet {#verify-reachability}

Führe das im Unraid-Terminal aus, mit deiner IP und deinem Port:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` heißt, der Weg funktioniert. `UNREACHABLE` heißt, das Netzwerk aus Schritt 3 muss zuerst repariert werden.

## Schritt 4: Variablen setzen und testen {#step-4}

1. Öffne in Unraid **Docker**, klicke auf BombVault, wähle **Edit**, setze **Host SSH: Address** (und **Host SSH: Port**, falls er nicht 22 ist) und drücke **Apply**. Fehlen die Variablen, importiere das Template noch einmal, denn Unraid behält die gespeicherte Konfiguration eines bestehenden Containers.
2. Öffne in BombVault **Einstellungen, Anbindungen, Host-SSH** und drücke **Verbindung testen**. Es sollte grün werden.

Derselbe Aufruf aus dem Terminal listet deine VMs auf:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Schritt 5: Die Methode pro VM {#per-vm-method}

Jede VM hat eine **Methode** in der Karte **VM** auf ihrem Tab Einstellungen (öffne die VM unter **Sicherungen**):

- **Graceful (Herunterfahren)** ist die Vorgabe. Sie fährt die VM herunter, sichert die Disks und startet sie wieder, die Sicherung ist also immer konsistent. Sie wartet bis zu zehn Minuten, bis der Gast heruntergefahren ist, und erzwingt das Ausschalten nie. Ein Gast, der danach noch läuft, bleibt unberührt, und diese Sicherung scheitert mit dem Grund. Schaltet sich der Gast innerhalb der nächsten Stunde aus, startet BombVault ihn wieder. Schlägt dieser Start fehl, sagt das der fehlgeschlagene Lauf.
- **Live-Snapshot** sichert eine laufende VM über einen externen Snapshot und `blockcommit`. Sie braucht die Disk auf `/mnt/cache` oder `/mnt/diskX`, nicht `/mnt/user`. Mit dem qemu-Gast-Agenten in der VM ist der Snapshot stillgelegt, ohne ihn ist er absturzkonsistent. Bei einer ausgeschalteten VM fällt Live von selbst auf Graceful zurück. Lässt sich der Snapshot nicht machen, scheitert diese Sicherung, und die VM läuft weiter, denn eine VM, die du live sichern lässt, wird dafür nie heruntergefahren.

## Was eine Sicherung auslässt {#left-out}

Eine Sicherung kann gelingen und trotzdem nicht die ganze VM enthalten. Der Lauf trägt dann einen Hinweis in Warnfarbe, der nennt, was fehlt:

- **Disks, die BombVault nicht liest.** Eine schreibgeschützte Disk, eine `lun` und eine `network`- oder `volume`-Disk werden ausgelassen, und der Hinweis nennt jede mit ihrem Zielgerät und ihrer Art. Die Definition in der Sicherung listet sie trotzdem auf. Ein CD-ROM- und ein Diskettenlaufwerk werden nicht genannt. Ein einfaches Blockgerät, das kein ZFS-zvol ist, lässt die Sicherung stattdessen scheitern.
- **Der Zustand eines emulierten TPM.** Er hat keinen Pfad im Domain-XML. Der Unraid VM Manager richtet so ein TPM für eine VM mit dem BIOS **OVMF TPM** ein, der Hinweis erscheint also auch auf Unraid. Ein Gast, der ein Geheimnis in seinem TPM aufbewahrt, wie es BitLocker kann, fragt nach einer Wiederherstellung, die diesen Zustand verliert, womöglich nach seinem Wiederherstellungsschlüssel. Halte diesen Schlüssel also bereit.

## Dünn belegte Disks {#thin-disks}

Eine Wiederherstellung hält eine dünn belegte Disk dünn, ob sie die Disk ersetzt oder eine neue schreibt: restic schreibt Folgen von Nullen als Löcher in eine neue Datei, und BombVault gibt die Nullblöcke einer ersetzten Datei mit `fallocate --dig-holes` frei, sobald sie geschrieben ist. Während restic eine Datei ersetzt, belegt diese Datei ihre volle Größe, wenn der Pool den Platz hat. Ein Dateisystem ohne Hole Punching behält die Disk in voller Größe, und das Protokoll sagt das. Die Platzprüfung rechnet eine neue Disk weiterhin mit voller Größe.

## Eine Wiederherstellung, die fehlschlägt {#restore-fails}

Eine Wiederherstellung an Ort und Stelle schaltet die VM aus, schreibt die Disks und ersetzt die Definition und das NVRAM erst, wenn die Disks geschrieben sind. Eine Wiederherstellung, die fehlschlägt, lässt die VM mit ihrem NVRAM definiert. Haben sich ihre Disks nicht geändert und lief sie, läuft sie wieder. Können sie sich geändert haben, bleibt sie aus, denn ein Start von einer halb geschriebenen Disk würde darüber schreiben, und der Lauf sagt das. Stellst du dieselbe Sicherung noch einmal wieder her, wird die Arbeit fertig.

Snapshots, die libvirt verzeichnet hat, was nicht die Art ist, wie der Unraid VM Manager sie anlegt, stoppen eine Wiederherstellung an Ort und Stelle, bevor die VM angefasst wird, weil libvirt eine VM mit solchen Snapshots nicht entfernen kann. Lösch sie zuerst.

## Nur geänderte Blöcke {#changed-blocks}

Eine VM mit qcow2-Disks lässt sich sichern, indem nur gelesen wird, was sich geändert hat. Schalte in derselben Karte **Nur geänderte Blöcke** ein, und BombVault nutzt statt der obigen Methode die Backup-API von libvirt:

- Es startet einen Pull-Mode-Backup-Job (`virsh backup-begin`), der im selben Augenblick einen Checkpoint namens `bombvault-<time>` anlegt, und liest die Disks über die SSH-Verbindung durch einen weitergeleiteten Unix-Socket. Antwortet der Gast-Agent, werden die Dateisysteme des Gasts nur eingefroren, während der Job startet. Ein Auftauen, das fehlschlägt, wird etwa fünfzehn Sekunden lang erneut versucht, und taut der Gast dann immer noch nicht auf, sendet BombVault eine Benachrichtigung.
- Der erste Lauf liest jeden belegten Block. Spätere Läufe lesen nur die Blöcke, die die Bitmap des Checkpoints als seit der vorigen Sicherung geschrieben markiert.
- Jede Disk wird als Ordner mit Segmenten fester Größe gespeichert, die das enthalten, was der Gast sieht. Unveränderte Segmente werden aus dem vorigen Snapshot übernommen, ohne gelesen zu werden, sodass jeder Snapshot vollständig ist und für sich wiederherstellt.
- Eine Wiederherstellung baut die Disk aus ihren Segmenten neu auf und wandelt sie mit `qemu-img` zurück in qcow2. Die Disk hat denselben Inhalt wie zur Sicherungszeit, aber die Datei ist eine neue qcow2-Datei, und eine Backing-Chain kommt abgeflacht zurück.
- Die Prüfung vor so einer Wiederherstellung listet jede Disk auf, die sie neu aufbaut, und rechnet sie mit der vollen Größe des Gasts, bei qcow2 doppelt, da das Rohabbild und seine umgewandelte Kopie nebeneinander liegen, bis die alte Datei ersetzt ist. Das ist das Meiste, was eine Disk beanspruchen kann, ein Fehlbetrag nach dieser Zählung lässt die Platzzeile also grau statt zu blockieren. Die Wiederherstellungsprüfung nach der ersten Sicherung und **Wiederherstellung prüfen** stellen außerdem sicher, dass der Snapshot jedes Segment jeder Disk in der Größe enthält, die es haben soll.
- Zwischen den Läufen hat die VM in libvirt keinen Checkpoint verzeichnet. BombVault hält die Definition des neuesten `bombvault-`-Checkpoints unter `/config/vm-checkpoints` und entfernt nur den Eintrag von libvirt dazu (`virsh checkpoint-delete --metadata`). Seine Bitmap bleibt in der qcow2-Datei und zählt weiter Schreibvorgänge mit, und der nächste Lauf legt den Checkpoint wieder an (`virsh checkpoint-create --redefine`), bevor er startet. libvirt lehnt es ab, eine VM mit Checkpoints zu entdefinieren, und so funktionieren **Remove VM** von Unraid und eine Wiederherstellung wie gewohnt.
- Ein Backup-Job, den BombVault laufen ließ, weil es mitten in einer Sicherung gestoppt hat, wird beim nächsten Start von BombVault beendet, zusammen mit den Checkpoints, die dieser Lauf verzeichnet hinterlassen hat.
- Auch das Ausschalten des Schalters, das Herausnehmen der VM aus dem Zeitplan oder das Löschen ihrer Sicherungen beendet so einen Job und löscht, solange die VM läuft, auch die Bitmap. Bei einer ausgeschalteten VM kann libvirt sie nicht löschen, und sie bleibt ungenutzt im Image.
- Checkpoints anderer Werkzeuge bleiben unberührt. Eine VM, die welche hat, lässt sich erst wiederherstellen, wenn sie gelöscht sind, und die Wiederherstellung stoppt, bevor sie die VM anfasst.

Der Schalter weicht von selbst aus. Ein fehlender oder kaputter Checkpoint, eine Disk, deren Größe sich geändert hat, oder ein neuester Snapshot, der keiner mit geänderten Blöcken ist, heißt, dass die ganze Disk noch einmal gelesen wird. Eine ausgeschaltete VM, eine Raw- oder zvol-Disk, keine Host-SSH-Verbindung, ein libvirt ohne Backup-API oder zu wenig freier Platz unter `/config` heißt, dass die übliche Sicherung mit der obigen Methode läuft. Die Karte sagt, welche davon der letzte Lauf nutzte und warum.

Voraussetzungen: libvirt mit der Backup-API (getestet mit libvirt 12.2 und QEMU 10.2 auf Unraid 7.3), sshd mit `AllowStreamLocalForwarding yes` (die Vorgabe, auch auf Unraid, das TCP-Forwarding abschaltet) und qcow2-Disks. libvirt lehnt Checkpoints auf Raw-Disks ab, und Unraid legt standardmäßig Raw-vdisks an. Die VM läuft während der Sicherung weiter, wie bei Live, und die Segmente, die auf restic warten, belegen bis zu 4 GiB unter `/config`.

## VMs mit Unraid-Snapshots {#unraid-snapshots}

Ein Snapshot, den der Unraid VM Manager macht, macht aus einer qcow2-Disk eine Chain: das ursprüngliche Image unten und darauf pro Snapshot eine Datei, die jeweils nur enthält, was sich seit der darunter geändert hat. Die VM schreibt in die neueste Datei. Eine Disk mit zwei Snapshots sind drei Dateien.

BombVault sichert so eine VM vollständig:

- **Jede Datei der Chain.** Es folgt der Backing-Chain jeder Disk bis zum Ende und sichert jede Datei darin, zusammen mit der Snapshot-Liste von Unraid (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, über SSH gelesen). Eine Chain, der man nicht bis zum Ende folgen kann, etwa weil eine Datei in der Mitte fehlt, lässt die Sicherung scheitern und nennt die Datei.
- **Eine Live-Sicherung behält die Snapshots.** **Live-Snapshot** legt ein temporäres Overlay obenauf, während es die Disk liest, und führt es danach zurück, nur in das Image direkt darunter. Die Chain ist nach der Sicherung dieselbe wie davor. Eine VM ohne Snapshots verhält sich wie immer.
- **Eine Wiederherstellung bringt die Chain zurück.** Eine Wiederherstellung an Ort und Stelle schreibt jede Datei dorthin zurück, wo sie war, und legt die Snapshot-Liste von Unraid zurück, sodass der VM Manager die Snapshots wieder auflistet. Eine Wiederherstellung neben das Original schreibt die Dateien in den Ordner der Kopie und lässt die wiederhergestellten Disks auf die wiederhergestellten Dateien zeigen, sodass die Kopie nie aus den Images des Originals liest. Die Snapshot-Liste von Unraid kommt nur bei einer Wiederherstellung an Ort und Stelle zurück, weil sie die Dateien dort nennt, wo sie gesichert wurden. Ohne sie bootet die Kopie genauso, und der VM Manager listet für sie nur keine Snapshots auf.
- **Nur geänderte Blöcke** liest, was der Gast sieht, seine Wiederherstellung kommt also als eine abgeflachte qcow2-Datei ohne die Snapshots zurück.
- **Eine Wiederherstellung bringt die Disk der Sicherung zurück, die du wählst.** Snapshots, die seit dieser Sicherung angelegt oder zusammengeführt wurden, ändern daran nichts. Eine Sicherung von vor einem Snapshot definiert die VM auf der Datei, die sie enthält, und eine Sicherung von vor einem Zusammenführen bringt ihr Overlay zurück und läuft darauf. Neuere Dateien bleiben, wo sie sind, ungenutzt und unverändert. Eine Sicherung, die ein Overlay enthält, aber nicht die Datei darunter, wird abgelehnt, bevor die VM angefasst wird. Die Definition der VM listet nach einer Wiederherstellung keine Chain auf, libvirt liest die Chain also aus den wiederhergestellten Dateien.

### Beispiel: eine VM mit zwei Snapshots {#example-unraid-snapshots}

Die VM `homeassistant` hat eine Disk, `vdisk1.qcow2`, und zwei Snapshots, die du im VM Manager vor zwei Updates gemacht hast. Ihre Disk ist jetzt eine Chain aus drei Dateien. Sie läuft die ganze Zeit, nutzt also **Live-Snapshot**.

1. Öffne **Sicherungen**, dann `homeassistant`, dann seinen Tab **Einstellungen**. Prüfe in der Karte **VM**, dass die **Methode** **Live-Snapshot** ist und **Nur geänderte Blöcke** aus.
2. Drücke **Jetzt sichern** im Kopf der Seite.
3. Wenn der Lauf fertig ist, ist seine Zeile unter **Letzte Läufe** auf dem Tab **Prüfung** grün. Die VM lief die ganze Zeit weiter, und die Sicherung enthält alle drei Dateien der Disk und die Snapshot-Liste von Unraid.
4. Im VM Manager sind die beiden Snapshots noch da, und die Disk liest weiter durch dieselben drei Dateien.

Einen Monat später geht ein Update schief, und du willst den Stand von gestern neben dem kaputten haben, um zu vergleichen:

1. Öffne **Wiederherstellung**, wähle **VMs** und klicke auf `homeassistant`.
2. Behalte die empfohlene Sicherung von letzter Nacht und wähle unter **Wohin** **Neben das Original**.
3. Drücke **Wiederherstellen**. Die drei Dateien landen in einem Ordner, der nach der VM mit `-restored` benannt ist, und die wiederhergestellte Disk zeigt auf die wiederhergestellten Dateien. An der laufenden VM ändert sich nichts.

Um stattdessen die ganze VM zurückzurollen, wähle **Original ersetzen** mit eingeschaltetem **Vorher den heutigen Zustand sichern**. BombVault sichert den kaputten Stand, fährt die VM herunter, schreibt die drei Dateien und die Snapshot-Liste von Unraid zurück und startet sie wieder.

## TrueNAS SCALE {#truenas-scale}

Die Schritte oben sind für Unraid geschrieben, aber dieselbe `qemu+ssh://`-Verbindung funktioniert gegen jeden libvirtd, den der Container erreichen kann, auch TrueNAS SCALE, das seit 25.04.2 wieder VMs auf libvirt hat. Auf TrueNAS braucht die Verbindungszeichenfolge eine Überschreibung, und der Schlüssel kommt in den eigenen Benutzereinstellungen von TrueNAS an root. Drei weitere Dinge ändern, was eine Sicherung enthält: Disks auf zvols, vTPM-Zustand und die Namen, die TrueNAS seinen VMs gibt.

### 1. Der libvirt-Socket {#truenas-socket}

Der libvirtd von TrueNAS lauscht nicht dort, wo die aus den **Host-SSH**-Feldern gebaute Verbindungszeichenfolge ihn erwartet. Setze stattdessen `LIBVIRT_URI`, mit dem zusätzlichen Parameter `?socket=`:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault nutzt `LIBVIRT_URI` genau wie geschrieben, und `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` und `LIBVIRT_SSH_PORT` sind nicht nötig. Für jede davon, die nicht gesetzt ist, übernimmt BombVault Host, Benutzer und Port aus der URI auch für seine eigenen SSH-Befehle, sodass die NVRAM-Kopie und ZFS-Dataset-Sicherungen dieselbe Maschine erreichen wie `virsh`. Eine Variable, die du selbst setzt, gewinnt immer, und das Boot-Protokoll nennt das Ziel, das es aus der URI übernommen hat. Auf TrueNAS braucht ein anderer Benutzer als root für ZFS-Dataset-Sicherungen außerdem `zfs allow <user> snapshot,destroy,mount <dataset>`, siehe [ZFS-Datasets](zfs-datasets.md#truenas).

Trage die LAN-Adresse von TrueNAS in die URI ein. Sind weder `LIBVIRT_URI` noch `LIBVIRT_HOST` gesetzt, nimmt BombVault das Standard-Gateway der App als Host, weil sich `host.docker.internal` in einer TrueNAS-App nicht auflöst, und das Boot-Protokoll nennt die Adresse, die es gewählt hat. Das reicht für ZFS-Dataset-Sicherungen. VM-Sicherungen brauchen die URI weiterhin für den Socket.

Die App braucht für VM-Sicherungen die Capability `SETPCAP`, neben `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` und `SETUID`. Ohne sie hält libvirt mit „cannot apply process capabilities“ an. Das YAML unter [TrueNAS SCALE](getting-started.md#truenas) setzt alle, zusammen mit `SYS_CHROOT`, das Snapshot statt Stopp braucht.

Auf TrueNAS SCALE 25.10.0 wurde der Socket-Pfad auf echter Hardware geprüft: Das Standard-`qemu:///system` scheitert mit `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory`, und derselbe Aufruf mit `?socket=/run/truenas_libvirt/libvirt-sock` listet die Domains auf.

### 2. Der Schlüssel von BombVault auf TrueNAS {#truenas-key}

Schritt 2 oben schreibt auf den Flash von Unraid und gilt hier nicht. Auf TrueNAS:

1. Schalte den SSH-Dienst unter **System, Services** ein.
2. Bearbeite unter **Credentials, Users** den Benutzer **root** und füge den öffentlichen Schlüssel von BombVault aus **Einstellungen, Anbindungen, Host-SSH** als seinen öffentlichen SSH-Schlüssel ein. TrueNAS behält ihn über Neustarts und Updates.

Frische Installationen von TrueNAS 24.10 und neuer geben root kein Passwort, und der Schlüssel-Login braucht keines. Willst du root lieber nicht nutzen, gib den Schlüssel für ein Admin-Konto mit sudo frei und setze seinen Namen in `LIBVIRT_URI` an die Stelle von `root`. ZFS-Dataset-Sicherungen brauchen dann `zfs allow`, wie oben beschrieben.

Eine ZFS-Replik auf einen zweiten Server meldet sich mit einem eigenen Schlüssel an, den der Replik-Dialog zeigt. Der Host-SSH-Schlüssel gibt keine Replik frei, und der Replik-Schlüssel gibt keine VM-Sicherungen frei.

!!! warning "TrueNAS sieht mit virsh gemachte Änderungen nicht"
    Die Middleware von TrueNAS, die Schicht hinter seinen VM-Bildschirmen, soll der einzige Weg sein, seine VMs zu ändern. Ein `virsh`-Befehl über diese SSH-Verbindung, was die VM-Sicherung und -Wiederherstellung von BombVault ausführen, umgeht sie, und TrueNAS gleicht später unter Umständen Zustand ab oder überschreibt ihn, den es nicht erzeugt hat. Das stammt aus dem dokumentierten Verhalten von TrueNAS und wurde auf einer echten Box nicht beobachtet. Es ist dieselbe Art von Risiko wie beim VM Manager von Unraid, der außerhalb gemachte Änderungen abgleicht, nur auf TrueNAS schärfer.

### 3. Disks auf zvols {#truenas-zvols}

TrueNAS-VM-Disks sind oft zvols, ZFS-Blockgeräte unter `/dev/zvol/<pool>/<dataset>`, keine qcow2-Dateien. restic kann ein Blockgerät nicht über seinen Pfad sichern, wie es eine Datei sichert, daher erkennt BombVault eine zvol-Disk im Domain-XML (`<source dev="...">` statt `<source file="...">`) und behandelt sie über dieselbe SSH-Verbindung. Der BombVault-Container braucht keine ZFS-Werkzeuge.

Eine Sicherung eines zvols:

1. `zfs snapshot <dataset>@bombvault-vm-<time>` legt einen Zeitpunkt fest, auch während die VM läuft.
2. `zfs clone -o volmode=dev -o readonly=on` macht diesen Snapshot als Gerät lesbar. `volmode=dev` hält den Host davon ab, die Partitionen des Gasts zu scannen.
3. `dd` liest das Gerät, und das Rohabbild geht als eine Datei in restic, `/vm-disks/<dataset>.img`, mit dem Tag `vm:<name>:zvol:<dev>`. restic zerlegt es nach Inhalt in Chunks, sodass Blöcke, die sich seit der letzten Sicherung nicht geändert haben, nicht noch einmal gespeichert werden.
4. Der Klon und der Snapshot werden danach zerstört, ob die Sicherung geklappt hat oder nicht. Wenn BombVault mitten in einer Sicherung gestoppt hat, entfernt die nächste Sicherung dieses zvols zuerst den Snapshot `bombvault-vm-<time>` und seinen Klon `<dataset>-bombvault-vm-<time>`. Jeden anderen Snapshot lässt sie in Ruhe, und einen, von dem etwas anderes geklont wurde, behält sie. Das Protokoll nennt, was sie entfernt oder behalten hat.

BombVault nutzt dafür kein `zfs send`. Jeder Datensatz eines Send-Streams trägt die GUID des Snapshots und eine laufende Prüfsumme, sodass restic nichts von der letzten Sicherung wiederverwenden könnte und jedes Mal die ganze Disk noch einmal speichern würde. Gemessen auf TrueNAS SCALE 25.10 mit einem zvol von 512 MiB, das 200 MiB Daten hält: Die erste Sicherung fügte 211 MB hinzu, eine zweite ohne Änderung 52 KB, und eine nach der Änderung von 1 MiB 3,6 MB. Dasselbe zvol über `zfs send` fügte bei jedem Lauf 214 MB hinzu.

Jeder Lauf schreibt außerdem einen kleinen Snapshot mit dem Tag `vm:<name>` mit dem Domain-XML, sobald alle Disks drin sind. Eine VM, deren Disks alle zvols sind, hat keinen Datei-Snapshot, und das ist der, der den Lauf unter der VM aufführt und wiederherstellbar macht.

Eine Wiederherstellung schreibt jedes zvol an Ort und Stelle zurück, wie eine Datei-Disk in ihre Datei zurückgeschrieben wird, sodass die wiederhergestellte VM die wiederhergestellten Daten bootet:

1. Bevor etwas angefasst wird, prüft BombVault, dass der Lauf ein Abbild jeder zvol-Disk enthält. Ein Lauf, dem eines fehlt, wird abgelehnt, und die VM bleibt, wie sie ist.
2. Ein vorhandenes zvol bekommt zuerst einen Snapshot, `<dataset>@bombvault-before-restore-<time>`, mit der Zeit in UTC wie bei jedem Snapshot-Namen, den BombVault macht, er kann also Stunden von der lokalen Uhr abweichen. Das Protokoll nennt ihn, und ein `zfs rollback` darauf macht die Wiederherstellung rückgängig, solange er der neueste Snapshot des zvols ist. Gibt es einen neueren, etwa den der nächsten Wiederherstellung, tut es `zfs rollback -r` und zerstört die neueren Snapshots. Er bleibt, bis du ihn mit `zfs destroy` entfernst, nachdem die wiederhergestellte VM funktioniert.
3. Ein zvol, das nicht mehr existiert, wird in der Größe angelegt, die es bei der Sicherung hatte, und ein vorhandenes mit anderer Größe wird auf diese Größe zurückgesetzt.
4. `dd` schreibt das Abbild auf `/dev/zvol/<dataset>`. Bei eingeschalteter Kompression, der Vorgabe von TrueNAS, kommen die leeren Teile des Abbilds als Löcher zurück und belegen keinen Platz.

**Eine Wiederherstellung auf eine andere Instanz braucht einen Ziel-Pool.** Sie verschiebt Datei-Disks in einen Zielordner, aber ein Ordner nennt keinen ZFS-Pool, daher lehnt BombVault die Wiederherstellung ohne einen vor dem Start ab. Gib den Pool als Parameter `zvolPool` von `POST /api/foreign/restore` mit: Jedes zvol behält seinen Pfad unterhalb des Pools (`tank/vms/win10/disk0` auf `flashpool` wird zu `flashpool/vms/win10/disk0`), und das Domain-XML zeigt darauf. Das Fenster **Aus einem alten Sicherungsordner** auf der Seite Wiederherstellung hat dafür kein Feld, es ist also nur über einen direkten API-Aufruf erreichbar. Eine Wiederherstellung auf derselben Instanz schreibt jedes zvol auf sich selbst zurück.

**Der Export als Dateien lässt zvols aus.** Eine VM, deren Disks alle zvols sind, wird mit diesem Grund abgelehnt, stelle sie also aus einer Sicherung wieder her. Bei einer VM, die auch Datei-Disks hat, enthält der Export die Datei-Disks und einen Hinweis, der die ausgelassenen zvols nennt.

Das wurde auf TrueNAS SCALE 25.10 mit ZFS 2.3.4 ausprobiert: drei Sicherungen eines Test-zvols, eine Wiederherstellung an Ort und Stelle, nachdem das zvol überschrieben und vergrößert worden war, und eine Wiederherstellung auf ein zvol, das gelöscht worden war. Jede Wiederherstellung las mit derselben SHA-256 zurück wie die Quelle, das zvol hatte wieder seine alte Größe, und es blieb kein Snapshot oder Klon zurück. Eine ganze VM, deren einzige Disk ein zvol ist, wurde laufend gesichert, mit Graceful und mit Live, und ausgeschaltet, und auch eine Wiederherstellung der ganzen VM (Entdefinieren, die zvols schreiben, definieren, booten) lief dort: Das zvol las mit der Prüfsumme der Quelle zurück, und die VM startete.

Datei-Disks, wie Unraid sie nutzt, sind von alledem nicht betroffen.

### 4. vTPM-Zustand {#truenas-vtpm}

Eine VM kann ein virtuelles TPM haben, das Secure Boot und Windows-11-Gäste brauchen. Sein Zustand liegt außerhalb der Disk und des NVRAM, eine Sicherung nur dieser beiden würde einen Gast also mit einem neuen, leeren TPM zurückbringen.

- **Ein Passthrough-TPM**, ein echter Chip, der an den Gast durchgereicht wird, nennt seinen Pfad im Domain-XML. BombVault speichert seinen Zustand neben dem NVRAM über SSH und schreibt ihn vor `virsh define` zurück, nach bestem Bemühen wie das NVRAM: Ein fehlgeschlagenes Lesen oder Schreiben wird protokolliert und lässt die Sicherung oder Wiederherstellung nicht scheitern.
- **Ein emuliertes vTPM**, wie TrueNAS es für Windows-11- und Secure-Boot-Gäste einrichtet, veröffentlicht keinen Zustandspfad im Domain-XML, sein Zustand wird also nicht gesichert. BombVault rät keinen Pfad, und die Sicherung endet mit einem Hinweis, dass der Zustand des TPM der VM nicht darin ist. Halte den Wiederherstellungsschlüssel eines solchen Gasts, etwa seinen BitLocker-Schlüssel, bereit, bevor du ihn wiederherstellst.
- **Du kannst einer TrueNAS-VM nicht selbst ein TPM anhängen.** Die VM-Geräte-API von TrueNAS SCALE 25.10 akzeptiert nur `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` und `USB` und lehnt `TPM` als unbekannten Typ ab.

Die TPM-Behandlung ist nur per Unit-Test geprüft und lief nicht gegen einen echten TrueNAS-Gast mit vTPM. Eine VM ohne TPM wird genau wie bisher gesichert und wiederhergestellt.

Das NVRAM wird wie gewohnt erfasst. TrueNAS hält es pro VM unter `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`, und das Domain-XML nennt diesen Pfad.

### 5. VM-Namen auf TrueNAS {#truenas-names}

Auf Unraid ist der libvirt-Name einer VM der Name, den du gewählt hast. TrueNAS erfindet den libvirt-Namen und hat das schon zweimal geändert:

| TrueNAS-Version | libvirt-Name | Wo dein Name steht |
|---|---|---|
| 25.10 „Goldeye“ | `{id}_{name}`, etwa `1_debian` | im libvirt-Namen, hinter dem `{id}_` |
| 26 | die UUID der VM, etwa `550e8400-e29b-41d4-a716-446655440000` | nur im `<title>` des Domain-XML |

Auf einem Host, den BombVault als TrueNAS erkennt oder den `PLATFORM=truenas` benennt, zeigt die Liste Sicherungen deinen Namen: `debian` statt `1_debian` auf 25.10 und den `<title>` statt der UUID auf 26, oder die UUID, wenn es keinen Titel gibt. Das ist nur, was die Liste zeigt. Jeder `virsh`-Aufruf und die Einstellungen pro VM nutzen weiter den libvirt-Namen, sodass Sicherung und Wiederherstellung immer auf die richtige VM wirken. Auf TrueNAS 26 heißt das, dass eine VM, die TrueNAS mit neuer UUID neu anlegt, als neuer Eintrag ohne ihre Einstellungen erscheint, auch wenn ihr Name gleich aussieht. Auf Unraid ändert sich nichts, auch nicht bei einer VM, die du `10_Windows` genannt hast.

Das Schema von 25.10 wurde auf einer echten TrueNAS-SCALE-25.10-Box beobachtet. Das Schema von TrueNAS 26 stammt aus der Dokumentation von TrueNAS und ist nur per Unit-Test geprüft.

## Fehlersuche {#troubleshooting}

| Symptom | Ursache und Lösung |
|---|---|
| **Verbindung testen** hängt und scheitert dann | Der Host ist nicht erreichbar. Prüfe Schritt 3 (Netzwerk, VLAN, Firewall) sowie **Host SSH: Address** und **Port**. |
| `Permission denied (publickey)`, obwohl der Schlüssel in `authorized_keys` steht | Das `StrictModes` von sshd lehnt die Schlüsseldatei ab, weil ein übergeordneter Ordner den falschen Besitzer oder Modus hat. Das ist häufig, wenn `/root/.ssh` ein Link auf den FAT-Flash (`/boot/config/ssh/...`) ist, der für alle beschreibbar ist. Das Host-Protokoll zeigt `Authentication refused: bad ownership or modes for directory ...`. Füge `StrictModes no` in der sshd-Konfiguration des Hosts im globalen Abschnitt vor jedem `Match`-Block hinzu und führe dann `/etc/rc.d/rc.sshd restart` aus. Unraid schreibt `/etc/ssh/sshd_config` bei jedem Start neu, halte es also wie unten beschrieben fest. |
| `Permission denied (publickey)` und der Schlüssel fehlt wirklich | Wiederhole Schritt 2 und prüfe, dass SSH an ist. |
| `Host key verification failed` | Führe `docker exec BombVault rm -f /config/ssh/known_hosts` aus und teste erneut. |
| Die Prüfung in Schritt 3 sagt `UNREACHABLE` | Der Container erreicht den SSH-Port des Hosts nicht. Siehe Schritt 3: Host-Zugriff auf eigene Netze, VLAN-Routing oder die Shim- oder LAN-IP. |
| Die Variablen fehlen unter **Edit** | Importiere das Template noch einmal. Ein bestehender Container behält seine alte gespeicherte Konfiguration. |
| Eine VM mit Unraid-Snapshots scheitert und nennt eine Datei | Der Backing-Chain der Disk lässt sich nicht bis zum Ende folgen. Prüfe im VM Manager, dass jeder Snapshot dieser Disk noch da ist und die genannte Datei unterhalb von `/mnt` existiert. |

Weitere VM-Symptome stehen unter [Fehlersuche](troubleshooting.md#vm-backup-will-not-connect).

## Über Neustarts hinweg behalten {#persistence}

Unraid schreibt `/etc/ssh/sshd_config` bei jedem Start neu, daher müssen Änderungen auf der Host-Seite beim Start erneut gemacht werden:

- Der **SSH-Port** liegt in `/boot/config/ident.cfg`, über die Unraid-Web-UI gesetzt.
- **`StrictModes no`**, falls du es oben gebraucht hast, bleibt nicht erhalten. Wende es beim Start aus `/boot/config/go` an:

  ```sh
  # /boot/config/go: keep SSH key auth working with flash-based authorized_keys
  ( for i in $(seq 1 30); do [ -f /etc/ssh/sshd_config ] && break; sleep 2; done
    grep -q '^StrictModes no' /etc/ssh/sshd_config || \
      sed -i '0,/^Match /s/^Match /StrictModes no\n&/' /etc/ssh/sshd_config
    # SIGHUP reloads sshd's config in place: it keeps the listener + open sessions
    # and avoids the "killing listener process / Restarting SSH server daemon"
    # warning that `rc.sshd restart` prints at boot. Only reload if sshd is already
    # up; if it starts after this runs, it reads the edited config on its own.
    pidof sshd >/dev/null && killall -HUP sshd ) &
  ```

- Der eigene SSH-Schlüssel von BombVault und `~/.ssh/config` liegen in `/config` (Appdata) und werden bei jedem Start neu erzeugt, sie überstehen also Neustarts des Containers und des Hosts.

## Sicherheit {#security}

Der freigegebene Schlüssel gewährt Root auf dem Host, dieselbe Vertrauensstufe wie der Docker-Socket, den BombVault ohnehin nutzt. Betreibe BombVault in einem vertrauenswürdigen Netzwerk. Der Schlüssel ist ed25519, mit Modus `0600` in Appdata gespeichert, und der Schlüssel des Hosts wird festgehalten. Jeder Host-Befehl wird als getrennte Argumente übergeben, nie über eine Remote-Shell, und die SSH-Verbindung nutzt `BatchMode` und ein Verbindungs-Zeitlimit.
