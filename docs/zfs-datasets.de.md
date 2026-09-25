# ZFS-Datasets

Die Seite **ZFS** sichert ZFS-Datasets. Ein Element ist ein Dataset zusammen mit jedem Dataset darunter. Für jedes Backup legt BombVault einen einzigen ZFS-Snapshot des ganzen Baums an, sodass jedes Dataset darin vom selben Augenblick stammt. Danach liest es die Dateien jedes Datasets aus diesem Snapshot, speichert sie mit restic so wie einen Ordner und entfernt den Snapshot gleich wieder. Die Backups sind dedupliziert, jedes lässt sich durchsuchen, und einzelne Dateien lassen sich wiederherstellen.

BombVault nutzt für Datasets nie `zfs send`, rollt nie ein Dataset zurück und löscht nie eines.

## Voraussetzungen {#requirements}

- **Die SSH-Verbindung zu diesem Server.** ZFS-Datasets nutzen denselben Schlüssel, Host und Benutzer wie VM-Backups. Laufen VM-Backups schon, klappt auch das hier. Sonst folge der [Anleitung zum VM-Backup über SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) auf GitHub. Die Template-Felder heißen **Host SSH: Address**, **Host SSH: Port** und **Host SSH: User**.
- **Der Befehl `zfs` auf diesem Host.** Unraid ab 6.12 und TrueNAS SCALE haben ihn.
- **Host Data als `/mnt` gemappt, mit Access Mode Read/Write - Slave.** Das ist die Vorgabe des Templates. Der Snapshot eines Datasets taucht im Ordner `.zfs/snapshot` des Datasets erst auf, nachdem BombVault gestartet ist, deshalb muss der Container Mounts mitbekommen, die der Host später anlegt.
- **Die Datasets unter `/mnt` eingehängt.** Unter Unraid liegen Pools unter `/mnt/<pool>`, das ist also schon der Fall.

Schalte die Domäne unter **Einstellungen, Allgemein** ein (ZFS-Datasets). Die ZFS-Seite zeigt dann die Karte **Verbindung zu diesem Server**. Sie testet die SSH-Verbindung, nennt Benutzer und Host, mit denen sie sich verbindet, und sagt, was fehlt, wenn etwas fehlt. Die Host-Integrationsprüfung (`/spike`) zeigt dasselbe Ergebnis.

## Elemente und Unter-Datasets {#items-and-children}

Öffne auf der ZFS-Seite **Datasets hinzufügen**. Die Liste kommt vom Server. Wähle das Dataset ganz oben in dem, was gesichert werden soll, zum Beispiel `cache/appdata`, und das Element umfasst dieses Dataset und jedes darunter.

- **Neue Unter-Datasets kommen von selbst dazu.** Ein Dataset, das später unter dem Element angelegt wird, wird beim nächsten Lauf gesichert, und dieser Lauf nennt es als neu. Sein erstes Backup liest es einmal vollständig, danach werden nur Änderungen gelesen.
- **Einzelne Unter-Datasets lassen sich auslassen.** Schalte eines in den Einstellungen des Elements ab, dann bleibt es samt allem darunter draußen. Ein ausgelassenes Unter-Dataset, das es auf dem Server nicht mehr gibt, wird so markiert und lässt sich aus der Liste entfernen.
- **Unter-Datasets, die nicht lesbar sind, werden übersprungen, aber nie stillschweigend.** Der Lauf führt sie auf, das Element zeigt, wie viele übersprungen wurden, und die Abdeckungskarte im Dashboard zählt jedes als nicht geschützt. Der Lauf sichert trotzdem alles andere und schlägt wegen eines übersprungenen Unter-Datasets nicht fehl. Die Gründe stehen in der [Tabelle der Grundcodes](#reason-codes): ein Dataset, das nicht eingehängt ist, `canmount=off` hat, einen `legacy`- oder gar keinen Mountpoint, einen nicht geladenen Verschlüsselungsschlüssel, abgeschalteten Snapshot-Zugriff oder einen Mountpoint, den BombVault nicht sieht.
- **Ein übersprungenes Dataset nimmt seine Unter-Datasets nicht mit.** Ein Dataset mit `canmount=off`, das nur andere Datasets enthält, wird übersprungen (angezeigt als "nur Struktur"), und seine eingehängten Unter-Datasets werden gesichert. Ein verschlüsseltes Dataset ohne geladenen Schlüssel wird zusammen mit den Unter-Datasets übersprungen, die seinen Schlüssel teilen.
- **Unter-Datasets, die VM-Platten oder Systemdaten sind, starten ausgeschaltet** im Dialog zum Hinzufügen, mit dem Grund neben dem Schalter. Wer einen ganzen Pool hinzufügt, bestätigt das vorher in einer Abfrage, die auflistet, was er enthält.

### Volumes {#volumes}

Ein Volume (zvol) enthält eine virtuelle Festplatte statt Dateien, und die ZFS-Seite sichert nie eines.

- Ein Volume, das eine VM nutzt, wird mit dieser VM auf der Seite **VMs** gesichert.
- Ein Volume, das keine VM nutzt (ein iSCSI-Extent, eine abgehängte Platte), wird **von BombVault nicht gesichert**. Der Dialog zum Hinzufügen und die ZFS-Seite zählen diese Volumes und sagen das. Eine spätere Version wird sie sichern.

Volumes im Baum eines Elements werden bei jedem Lauf übersprungen und genannt.

### Der Speicher von Docker {#docker-storage}

Mit dem ZFS-Speichertreiber von Docker ist jede Image-Schicht ein Dataset mit `legacy`-Mountpoint. Der Dialog zum Hinzufügen fasst sie zu einer Zeile pro Eltern-Dataset zusammen. Ein Baum mit mehr als 20 solcher Datasets kann kein Element werden: Solange ein Snapshot davon existiert, kann Docker keine Image-Schichten entfernen. Füge stattdessen die Datasets darunter hinzu, zum Beispiel `appdata`.

### Elemente überschneiden sich nie {#overlap}

Ein Dataset kann nur zu einem Element gehören. BombVault lehnt ein neues Element ab, das in einem bestehenden liegt oder eines enthalten würde. Um mehrere Unter-Elemente zu einem Eltern-Element zusammenzufassen, lösche zuerst die Unter-Elemente, behalte dabei ihre Backups und füge dann das Eltern-Element hinzu. Jedes Dataset behält seinen Verlauf unter seinem eigenen Namen, deshalb macht das nächste Backup dort weiter, wo die alten Elemente aufgehört haben, und liest nicht alles neu.

## Container und Befehle rund um den Snapshot {#consistency}

Ein Snapshot einer laufenden Datenbank ist wie ein plötzlicher Stromausfall: Die Datenbank erholt sich meistens, aber sie muss es tun. Jedes Element kann dagegen zwei Dinge tun, und beide gelten nur für den Augenblick des Snapshots, nicht für das ganze Backup.

- **Diese Container für den Snapshot stoppen.** BombVault stoppt die aufgeführten Container, legt den Snapshot an und startet sie sofort wieder. Container derselben Abhängigkeitsstufe stoppen parallel, abhängige zuerst, deshalb dauert das ganze Fenster meist ein paar Sekunden; der Lauf zeigt, wie lange es war. Das Backup liest danach den eingefrorenen Snapshot, während die Apps schon wieder laufen. Gestoppt werden nur Container, die liefen.
- **Ein Befehl vor und nach dem Snapshot.** Er läuft in einem Container deiner Wahl, zum Beispiel um kurz vor dem Snapshot eine Datenbank in das Dataset zu dumpen, ohne etwas zu stoppen. Scheitert der Befehl vor dem Snapshot, schlägt das Backup fehl und es entsteht kein Snapshot. Ein scheiternder Befehl nach dem Snapshot wird am Lauf angezeigt, lässt das Backup aber nicht fehlschlagen.

Was passiert, wenn etwas schiefgeht:

- Lässt sich ein Container nicht stoppen, startet BombVault die schon gestoppten wieder, und das Backup schlägt fehl und nennt den Container. Es weicht nie auf einen Snapshot laufender Apps aus.
- Das Stoppen wartet, bis ein laufendes Container-Backup fertig ist (bis zu 30 Minuten bei einem manuellen Lauf, bis zum Zeitlimit des Backups bei einem geplanten), damit die beiden nie gleichzeitig denselben Container stoppen und starten.
- Bevor der erste Container stoppt, schreibt BombVault auf, welche es stoppt. Wird BombVault innerhalb des Fensters beendet, startet es diese Container beim nächsten Start wieder, schickt eine Benachrichtigung, und das Element zeigt einen roten Hinweis für jeden Container, den es nicht starten konnte.

Die automatischen Datenbank-Dumps (siehe [Funktionen](features.md)) laufen mit dem eigenen Backup eines Containers auf der Seite **Container**, nicht mit einem ZFS-Element. Eine Datenbank, deren Container nur über sein Dataset gesichert wird, bekommt keinen Dump, also gib ihr hier einen Befehl.

Ein Container kann gleichzeitig auf dieser Liste und auf der Seite **Container** stehen. Seine Daten liegen dann doppelt, in zwei Repositories, und das **Gesamt-Backup** stoppt ihn zweimal. Das Element weist darauf hin.

## Wiederherstellen {#restore}

Öffne **Backups** am Element, wähle das Backup und dann das Dataset. Vorgegeben ist das oberste Dataset des Elements.

- **In das Dataset zurückschreiben.** Dateien aus dem Backup werden in den Mountpoint des Datasets geschrieben. Gleichnamige Dateien werden überschrieben, andere bleiben liegen. Das Dataset selbst wird nie zurückgerollt oder ersetzt. BombVault prüft, ob das Dataset eingehängt, sichtbar und beschreibbar ist, einmal vor dem Start und noch einmal direkt vor dem Schreiben. Wo ein Unter-Dataset darin eingehängt ist, wird nichts geschrieben: Das Unter-Dataset behält seine Dateien, seinen Besitzer und seine Rechte und wird aus seinem eigenen Backup wiederhergestellt.
- **In einen Ordner wiederherstellen.** Wähle einen Ordner unter `/mnt`. BombVault prüft, ob der Ordner auf einem eingehängten Pool oder einer Freigabe liegt und ob genug Platz frei ist. Das geht ohne SSH-Verbindung und auch für Datasets, die es nicht mehr gibt.
- **Dateien auswählen** (Erweitert): nur die Dateien und Ordner, die du auswählst, zurück in das Dataset schreiben.
- **Alle Datasets dieses Backups** (Erweitert): jedes Dataset des Baums in einen eigenen Unterordner des gewählten Ordners. Datasets, die in diesem Backup übersprungen wurden, werden genannt.
- **Von einem anderen Server:** Die Seite **Wiederherstellung** stellt aus dem Repository eines anderen BombVault wieder her, immer in einen Ordner: alle Datasets eines Backups, jedes in einen eigenen Unterordner, oder ein Dataset des Baums, ganz oder ausgewählte Dateien.

Die Liste der zu stoppenden Container des Elements wird auch beim Zurückschreiben in das Dataset angeboten. Diese Container bleiben während der ganzen Wiederherstellung gestoppt, und Container-Backups warten so lange.

### Der Sicherheits-Snapshot {#safety-snapshot}

Bevor es in ein Dataset schreibt, legt BombVault einen ZFS-Snapshot genau dieses Datasets an, mit dem Namen `bombvault-prerestore-<Zeit>`. Er ist standardmäßig an; ihn abzuschalten braucht eine zweite Bestätigung. Lässt sich der Snapshot nicht anlegen, wird nichts wiederhergestellt.

BombVault löscht einen Sicherheits-Snapshot nie von selbst. Das Element listet sie mit Alter und Größe auf, jeden mit einer Aktion **Löschen**, und warnt, wenn der älteste mehr als 30 Tage alt ist, weil er gelöschte und geänderte Daten im Pool festhält.

Um nach einer Wiederherstellung zurückzugehen, kopiere einzelne Dateien aus `.zfs/snapshot/bombvault-prerestore-<Zeit>` im Dataset. `zfs rollback <dataset>@bombvault-prerestore-<Zeit>` geht nur, solange er der neueste Snapshot dieses Datasets ist. `zfs rollback -r` löscht jeden neueren Snapshot, automatische eingeschlossen.

### Als neues Dataset wiederherstellen {#new-dataset}

BombVault legt keine Datasets an. Lege es auf dem Server mit den gewünschten Eigenschaften an und stelle dann in einen Ordner wieder her, der sein Mountpoint ist:

```
zfs create -o compression=lz4 cache/appdata-restored
```

und stelle in BombVault in den Ordner `cache/appdata-restored` unter `/mnt` wieder her.

## Was im Backup steckt {#contents}

Im Backup: die Dateien und Ordner jedes gesicherten Datasets, mit Besitz, Rechten, Zeitstempeln und erweiterten Attributen, so wie restic sie speichert.

Nicht im Backup:

- die ZFS-Eigenschaften der Datasets (compression, recordsize, quota, mountpoint und der Rest);
- Besitzer und Rechte des obersten Ordners jedes Datasets selbst (alles darunter ist enthalten). Eine Wiederherstellung in das Dataset lässt den vorhandenen obersten Ordner, wie er ist, eine Wiederherstellung in einen Ordner legt ihn mit den Rechten `0755` an;
- vorhandene ZFS-Snapshots;
- Unter-Datasets, die übersprungen oder ausgelassen wurden;
- Volumes.

Um auf einen neuen Pool wiederherzustellen, lege zuerst die Datasets mit den gewünschten Eigenschaften an. Ob NFSv4-ACLs, wie TrueNAS sie auf SMB-Datasets nutzt, so zurückkommen, wie du es erwartest, ist noch nicht geprüft. Teste eine Wiederherstellung mit deinen eigenen Daten, bevor du dich darauf verlässt.

## Verschlüsselte Datasets {#encryption}

Ein verschlüsseltes Dataset wird nur gesichert, solange sein Schlüssel geladen ist. Sonst wird es mit einer Warnung übersprungen; lade den Schlüssel mit `zfs load-key` und hänge das Dataset ein. BombVault liest die Daten entschlüsselt und speichert sie im Repository von restic, das verschlüsselt ist. Hast du die Verschlüsselung in BombVault abgeschaltet, ist dieses Repository es nicht.

## Übrig gebliebene Snapshots {#leftover-snapshots}

Der Snapshot eines Backups heißt `<dataset>@bombvault-<14 Ziffern>`, zum Beispiel `cache/appdata@bombvault-20260924021500` (UTC). BombVault entfernt ihn direkt nach dem Backup. Klappt das nicht, etwa weil das Dataset beschäftigt ist oder BombVault gestoppt wurde, entfernt BombVault ihn:

- vor dem nächsten Backup dieses Elements,
- beim Start von BombVault, für jedes Element, auch bei abgeschalteter Domäne,
- wenn du das Element löschst,
- wenn du am Element **Jetzt entfernen** drückst; dort steht auch, wie viele übrig sind.

Entfernt werden nur Namen, die genau `bombvault-` plus 14 Ziffern lauten. Sicherheits-Snapshots, deine eigenen Snapshots und automatische Snapshots werden nie angefasst. Um einen von Hand zu entfernen:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

## Anomalien {#anomalies}

Ein Unter-Dataset, das geleert wurde, verändert die Summe eines großen Baums kaum, deshalb beobachtet die Anomalie-Erkennung jedes Dataset eines Elements für sich: Größe, Dateianzahl, neue Daten und restic-Dauer haben jeweils ihren eigenen Verlauf. Dieser Verlauf gehört zum Namen des Datasets und bleibt deshalb, wenn der Baum später von einem anderen Element gesichert wird.

Ein Dataset, das der vorige Lauf gesichert hat und das dieser Lauf nicht lesen konnte, zählt als geleert, solange sich die Auswahl des Elements nicht geändert hat. Das deckt einen nicht geladenen Schlüssel ab, ein nicht eingehängtes Dataset und eines, das aus dem Baum verschwunden ist. Ein Unter-Dataset, das du selbst ausschließt, ändert die Auswahl, deshalb beginnt sein Verlauf stattdessen neu. Solange ein Fund zu verlorenen Daten offen ist, behält die Aufbewahrung die alten Backups genau dieses Datasets und räumt den Rest des Baums wie gewohnt auf.

Im Reiter **Elemente** der Seite **Anomalien** hat jedes Dataset eine eigene Zeile unter seinem Element, und der Baum des Elements auf dieser Seite zeigt die offenen Funde neben jedem Dataset. Der Link in einem Fund öffnet die Wiederherstellung des Elements beim letzten guten Backup des Datasets. Ob ein Lauf fertig wird, wird für das ganze Element beurteilt, weil ein Lauf als Ganzes gelingt oder scheitert.

Die Prüfungen selbst sind unter [Funktionen](features.md) beschrieben. Ein Assistent, der über den [MCP-Server](mcp.md) verbunden ist, kann die Wiederherstellungspunkte eines ZFS-Elements auflisten, sein Backup starten und die Funde lesen, quittiert wird ein Fund aber auf der Seite **Anomalien**.

## Grundcodes {#reason-codes}

Die Seite, der Laufverlauf und die Benachrichtigungen nennen ein Problem mit einem dieser Codes. Bei den meisten steht die Abhilfe auch auf der Seite daneben.

| Code | Bedeutung | Was zu tun ist |
|---|---|---|
| `ssh-missing` | Die SSH-Verbindung ist in diesem Container nicht eingerichtet. | Richte die SSH-Verbindung ein wie für VM-Backups. |
| `host-placeholder` | Host SSH: Address ist noch der Platzhalter, und `host.docker.internal` hat auch nicht geantwortet. | Setze Host SSH: Address auf die LAN-IP dieses Servers. |
| `host-fallback` | Host SSH: Address ist noch der Platzhalter, und `host.docker.internal` funktioniert. | Nichts, oder setze die LAN-IP. |
| `ssh-unreachable` | Der Server ist über SSH nicht erreichbar. | Prüfe Adresse und Port und ob SSH eingeschaltet ist. |
| `ssh-auth` | Der Server hat den Schlüssel von BombVault abgelehnt. | Führe den Befehl von der Verbindungskarte einmal auf dem Server aus. |
| `zfs-not-found` | Der SSH-Host hat keinen Befehl `zfs`. | Richte Host SSH: Address auf den Rechner, dem die Pools gehören. |
| `zfs-permission` | Der SSH-Benutzer darf diesen zfs-Befehl nicht ausführen. | Nimm root, oder siehe [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` nennt einen anderen Host oder Benutzer als die SSH-Felder. | Bring sie in Einklang, oder leere die SSH-Felder, damit beides aus der URI kommt. |
| `zfs-error` | zfs hat einen anderen Fehler gemeldet. | Die Details zeigen seine Meldung. |
| `propagation-missing` | Neue Mounts auf dem Host erreichen den Container nicht. | Setze den Access Mode von Host Data auf Read/Write - Slave und starte BombVault neu. |
| `invalid-name` | Ein Dataset-Name, den BombVault nicht annimmt. | Benenne das Dataset um. |
| `name-too-long` | Ein Dataset im Baum ist zu lang für einen Snapshot-Namen. | Benenne es um, oder füge ein Dataset darunter als Element hinzu. |
| `invalid-exclude` | Ein Ausschlussmuster oder ein ausgelassenes Unter-Dataset passt nicht zum Element. | Korrigiere den Eintrag, den die Meldung nennt. Um ein ganzes Unter-Dataset auszulassen, schalte es ab, statt ein Muster zu schreiben. |
| `not-found` | Das Dataset gibt es auf dem Server nicht. | Entferne das Element oder lege das Dataset neu an. Seine Backups bleiben wiederherstellbar. |
| `not-filesystem` | Das ist ein Volume, kein Dateisystem. | Siehe [Volumes](#volumes). |
| `overlaps-item` | Das Dataset überschneidet sich mit einem bestehenden Element. | Siehe [Elemente überschneiden sich nie](#overlap). |
| `docker-storage` | Der Baum enthält den Image-Speicher von Docker. | Siehe [Der Speicher von Docker](#docker-storage). |
| `nothing-readable` | Kein Dataset im Element ist gerade lesbar. | Sieh dir die Codes der übersprungenen Datasets an. |
| `snapshot-failed` | Der Snapshot ließ sich nicht anlegen. | Die Details zeigen die Meldung von zfs. |
| `containers-busy` | Als die Container stoppen mussten, lief noch ein Container-Backup. | Starte später noch einmal. Geplante Läufe warten von selbst. |
| `consistency-stop-failed` | Ein Container ließ sich nicht stoppen, deshalb gibt es keinen Snapshot. | Prüfe den Container oder nimm ihn von der Liste. |
| `pre-snapshot-failed` | Der Befehl vor dem Snapshot ist fehlgeschlagen. | Die Details des Laufs zeigen seine Ausgabe. |
| `container-unknown` | Ein aufgeführter Container existiert nicht. | Nimm ihn von der Liste. |
| `container-is-self` | BombVault kann seinen eigenen Container nicht stoppen. | Nimm ihn von der Liste. |
| `leftover-snapshots` | Snapshots, die BombVault nicht entfernen konnte, liegen noch auf dem Server. | Drück **Jetzt entfernen**, siehe [Übrig gebliebene Snapshots](#leftover-snapshots). |
| `zvol` | Ein Volume im Baum, übersprungen. | Siehe [Volumes](#volumes). |
| `canmount-off` | Nie eingehängt (`canmount=off`), übersprungen. | Liegen darin Daten, hänge es ein oder verschiebe die Daten in ein Unter-Dataset. |
| `legacy-mount` | Legacy-Mountpoint, übersprungen. | Gib ihm einen Mountpoint unter `/mnt`. |
| `no-mountpoint` | Kein Mountpoint, übersprungen. | Gib ihm einen Mountpoint unter `/mnt`. |
| `not-mounted` | Auf dem Server nicht eingehängt, übersprungen. | Häng es mit `zfs mount` ein, oder setze `canmount=on`. |
| `key-not-loaded` | Verschlüsselt und der Schlüssel ist nicht geladen, übersprungen. | `zfs load-key`, dann einhängen. |
| `snapdir-disabled` | Der Snapshot-Zugriff ist abgeschaltet, übersprungen. | `zfs set snapdir=hidden <dataset>`. Der Ordner `.zfs` bleibt verborgen. |
| `not-visible` | BombVault sieht den Mountpoint des Datasets nicht. | Verschiebe den Mountpoint unter den Host-Data-Pfad, oder mappe ihn unter demselben Pfad mit Read/Write - Slave in den Container. |
| `shfs-only` | Das Dataset ist nur über `/mnt/user` sichtbar, das Snapshots verbirgt. | Mappe `/mnt` als Host Data, nicht `/mnt/user`. |
| `snapshot-not-visible` | Der Snapshot wurde angelegt, ist in BombVault aber nicht aufgetaucht. | Führe **Snapshot-Zugriff testen** aus; siehe unten. |
| `snapshot-loop` | Der Snapshot hat BombVault nicht erreicht, weil Host Data neue Mounts nicht durchreicht. | Setze den Access Mode von Host Data auf Read/Write - Slave und starte BombVault neu. |
| `backup-failed` | restic ist für dieses Dataset fehlgeschlagen. | Die Details des Laufs zeigen den Grund. |
| `not-reached` | Der Lauf endete vor diesem Dataset. | Starte das Backup noch einmal. |
| `gone` | Das Dataset ist nicht mehr auf dem Server. | Nichts. Seine Backups bleiben wiederherstellbar. |
| `read-only-mount` | BombVault kann das Dataset nur lesen und deshalb nicht hinein wiederherstellen. | Setze das Mapping auf Read/Write - Slave, oder stelle in einen Ordner wieder her. |
| `destination-not-mounted` | Der Ordner liegt auf keinem eingehängten Pool und keiner Freigabe. | Wähle einen Ordner auf einem Pool oder einer Freigabe. |
| `not-enough-space` | Am Ziel ist nicht genug Platz frei. | Schaffe Platz oder wähle einen anderen Ordner. |
| `safety-snapshot-failed` | Der Sicherheits-Snapshot ließ sich nicht anlegen, deshalb wurde nichts wiederhergestellt. | Die Details zeigen die Meldung von zfs. |
| `safety-name-too-long` | Der Dataset-Name ist zu lang für einen Sicherheits-Snapshot. | Schalte den Sicherheits-Snapshot ab, oder stelle in einen Ordner wieder her. |

### Prüfen, was der Container sieht {#mountinfo}

**Snapshot-Zugriff testen** an einem Element legt einen echten Snapshot seines Baums an, sucht ihn in BombVault für jedes Dataset und entfernt ihn wieder. So lässt sich der ganze Weg am schnellsten vor dem ersten geplanten Lauf belegen.

Um selbst nachzusehen, führe das auf dem Server aus:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Jede Zeile ist ein Mount im Container. Die Zeile eines Datasets zeigt seinen Pfad im Container (unter `/host/user`) und den Dataset-Namen. Ein Feld `master:N` in dieser Zeile heißt, dass der Mount die Mounts mitbekommt, die der Host später anlegt, und genau das braucht der Snapshot-Zugriff. Fehlt es, setze den Access Mode von Host Data auf Read/Write - Slave und starte BombVault neu.

## TrueNAS SCALE {#truenas}

- Ist `LIBVIRT_URI` gesetzt (wie für VM-Backups unter TrueNAS), nimmt BombVault SSH-Host, Benutzer und Port für seine zfs-Befehle aus der URI, jeden Wert, der nicht eigens gesetzt ist. Ohne VM-Backups setze stattdessen `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` und `LIBVIRT_SSH_PORT`. Trag die Variablen unter **Additional Environment Variables** ein.
- Ein anderer Benutzer als root braucht Rechte auf dem obersten Dataset des Elements, die dann jedes Dataset darunter abdecken:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Eine SSH-Sitzung ohne root hat unter TrueNAS `/usr/sbin` nicht im Pfad; BombVault ruft dann direkt `/usr/sbin/zfs` auf.
- **Host Data** der App muss ein Host-Pfad oberhalb der Datasets sein, zum Beispiel `/mnt/tank`, kein ixVolume. Mit einem Host-Pfad reicht die App neue Mounts des Hosts an BombVault weiter (`rslave`), und das braucht der Snapshot-Zugriff.
