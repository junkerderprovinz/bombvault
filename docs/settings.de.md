# Einstellungen

Die **Einstellungen** bestehen aus vierzehn Kacheln. Jede Kachel ist eine Seite voller Karten, und jede Änderung wird beim Machen geschrieben, mit einem kurzen Aufblitzen am Bedienelement oder einem Wackeln, wenn der Server sie ablehnt. Ein paar Fenster behalten einen **Speichern**-Knopf, weil ein halb fertiges Speichern unsicher wäre, etwa bei Zugangsdaten oder dem Login-Passwort. Erklärungen stehen in den (i)-Blasen neben jedem Bedienelement.

Drücke irgendwo in den Einstellungen `/` oder Strg+K, um alle zu durchsuchen, auch nach den Namen, die Seiten in früheren Versionen hatten. Die Reihenfolge der Kacheln lässt sich unter **Aussehen** ändern. Wo eine Kachelspalte neben der Seite sie zusammendrücken würde, etwa in einem Fenster von etwa 1200 Pixeln Breite oder auf einem Handy, werden die Kacheln zu einem Streifen über der Seite, der seitlich scrollt.

![Die Einstellungen mit geöffneter Kachel Allgemein.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Die Einstellungen mit geöffneter Kachel Allgemein.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Diese Seite listet jede Kachel und ihre Karten auf und verlinkt auf die Seite, die jede davon ausführlich erklärt.

## Allgemein {#general}

- **Bereiche** schaltet jeden Bereich ein oder aus: **Container**, **VMs**, **Flash**, **Ordner**, **ZFS-Datasets** und **Selbstbackup**. Ein Bereich, der aus ist, wird nach keinem Zeitplan gesichert, auch nicht nach dem eigenen Plan eines Eintrags, und seine Einträge zählen nicht. **Jetzt sichern** funktioniert trotzdem. **Flash** erscheint nur auf Unraid.
- **Container** enthält, was für alle Container gilt, etwa **Automatische Datenbank-Dumps**. Alles, was einen einzelnen Container betrifft, steht auf seiner Seite. Siehe [Datenbank-Dumps](backups.md#database-dumps).

## Aussehen {#look}

- **Darstellung**: **Sprache** (42 Sprachen), **Design** (**Dunkel**, **Hell** oder **System**), **Ecken**, **Animation** (**Aus**, **Dezent**, **Wild** oder **Sturm**), **Leise Benachrichtigungen**, was Erfolgsmeldungen wie Speicherbestätigungen ausblendet, und **Reihenfolge der Einstellungsseiten**, wo du eine Kachel hältst und an ihren Platz ziehst oder Alt und die Pfeiltasten nutzt.
- **Beschriftungen**: wie viel von einem Bedienelement angezeigt wird, getrennt für **Knöpfe**, **Seitenleiste**, **Tabs** und **Untere Leiste**: **Text**, **Text und Symbol**, **Symbol** oder **Reaktiv**, was nur das Symbol zeigt und die Worte einblendet, wenn du darauf zeigst. Die **Seitenleiste** betrifft nur das Desktop-Layout und die **Untere Leiste** nur das Handy-Layout.
- **Farben**: eine **Akzentfarbe** oder der **Regenbogen-Modus**, in dem jede Zeile einer Liste aus einer Palette von acht ihre eigene Farbe bekommt. Der Regenbogen-Modus bringt den **Disco-Modus**, in dem die Farben langsam zur nächsten gleiten, den **Reaktiven Modus**, in dem die Farbe einer Zeile nur sichtbar ist, solange du darauf zeigst, sie läuft oder gewählt ist, die **Farbenrotation** und die **Farbpalette** selbst.

BombVault hält das Aussehen auf dem Server. Ein Browser, der zum ersten Mal kommt, übernimmt es. Wenn dein System weniger Bewegung wünscht, gilt das immer vor **Animation**.

## Zeitplan {#schedule}

**Woche**, **Gesamt-Backup** mit **Wann gesichert wird**, **Verpasste Sicherungen nach dem Start nachholen** und **Befehle vorher und nachher**, **Pläne pro Bereich** mit **Off-site-Kopien pro Bereich**, **Einträge im Zeitplan**, **Reihenfolge** mit **Neustart nach der Sicherung**, **Snapshot statt Stopp**, **Updates nach der Sicherung** und **Ruhe vor der Sicherung**. Siehe [Zeitpläne](schedules.md).

## Speicherorte {#storage-locations}

- **Auch in der Seitenleiste zeigen** heftet die Seite Speicherorte an die Seitenleiste.
- Die Liste deiner Speicherorte mit **Speicherort hinzufügen**. Ein Klick öffnet die Seite eines Speicherorts, siehe [Die Seite eines Speicherorts](offsite-recovery.md#location-page).
- **Zugangsdaten**: die gespeicherten Zugangsdaten mit dem Standardstern, siehe [Zugangsdaten](offsite-recovery.md#sign-ins).
- **Speicherorte je Bereich**: der 3-2-1-Ablauf, in dem jede Zeile ein Schalter ist, siehe [Vorgaben pro Bereich](offsite-recovery.md#placement-defaults).
- **Repositories**: Repositories, die für einzelne Einträge angelegt wurden, siehe [Zuordnung pro Eintrag](offsite-recovery.md#placement).
- **Wie lange aufheben** verweist auf die Speicherorte, wo die Aufbewahrung eingestellt wird, und **Vorschau** zeigt **Was der nächste Lauf entfernen würde**, pro Repository und pro Eintrag, bevor es passiert. Sie ändert und sperrt nichts und antwortet daher auch, während eine Sicherung läuft.
- **Off-site-Kopien**: eine Zeile pro Bereich mit **Verbindung testen**, **Jetzt replizieren** und **Einrichten** oder **Bearbeiten**, siehe [Off-site-Kopien](offsite-recovery.md#off-site-copies).
- **Off-site-Bandbreite**: **Upload-Limit (KiB/s)** und **Download-Limit (KiB/s)** für jede Kopie. 0 heißt unbegrenzt.

Die Karte **Streaming**, die das Upload-Limit senkt, während Plex, Jellyfin oder Emby streamen, liegt auf der Seite Speicherorte selbst.

## Wiederherstellung {#restore}

- Ein Link auf die Seite **Wiederherstellung**.
- **Ordner für Wiederherstellungen**: der **Standardordner für Wiederherstellungen** für eine Wiederherstellung in einen Ordner, mit **Unterordner pro Bereich** und einer **Ausnahme** pro Bereich. Siehe [Wohin eine Wiederherstellung in einen Ordner geht](restore.md#folders).
- **Container-Registries**: Zugangsdaten für private Registries, etwa ein durch Sponsoring geschütztes `ghcr.io`-Image. Eine Wiederherstellung nutzt sie, um ein fehlendes Image zu ziehen, und ein Update nach der Sicherung, um nach einem neueren zu suchen. Öffentliche Images werden ohne sie gezogen. **Registry hinzufügen** bietet öffentliche Registries, selbst betriebene, solche bei einem Cloud-Anbieter und **Andere Registry** für jede Adresse.
- **Für den Notfall**: **Recovery-Kit herunterladen**, was ein Login-Passwort braucht. Siehe [Recovery-Kit](offsite-recovery.md#recovery-kit).

## Benachrichtigungen {#notifications}

**Benachrichtigungen** mit **Benachrichtigen**, den Zusammenfassungen, **Überfällige Sicherungen melden**, **Unraid-Benachrichtigungen** und dem **Wochenbericht**, **Kanäle** mit Webhook, Apprise, Matrix, Gotify und E-Mail (SMTP) sowie **Healthchecks**. Siehe [Benachrichtigungen](notifications.md).

## Prüfung {#verify}

- **Sicherungen prüfen**: das Ergebnis des letzten `restic check` pro Bereich, mit **Entsperren** und **Aufräumen** unter **Mehr** in jeder Zeile. **Entsperren** hebt eine veraltete Sperre auf, und **Aufräumen** wendet die Aufbewahrungsregeln auf Abruf an.
- **Automatische Prüfungen**: die geplante Wiederherstellungsprüfung mit ihrer **Datenstichprobe (%)** und den Testzielen, **Start-Test**, der auch einen Container in eine isolierte Kopie wiederherstellt und startet, der **Geplante Off-site-DR-Test** und **Löschschutz prüfen**, der geplante Manipulationstest. Siehe [DR-Tests](offsite-recovery.md#dr-drills) und [Manipulationstest](offsite-recovery.md#tamper-test).
- **Anomalien**: Erkennung, Empfindlichkeit und welche Funde eine Nachricht schicken. Siehe [Anomalien](anomalies.md).

## Sicherheit {#security}

- **Passwort**: das Login-Passwort, mindestens 12 Zeichen. Es zu ändern oder zu löschen verlangt das aktuelle und, bei eingeschalteter Zwei-Faktor-Authentifizierung, den Code.
- **Zwei-Faktor-Authentifizierung**: ein Code aus einer Authenticator-App zusätzlich zum Passwort, mit acht Wiederherstellungscodes, die einmal angezeigt werden.
- **Passkeys**: Anmelden mit einem Schlüssel auf Handy, Laptop oder Sicherheitsstick. Sie brauchen einen echten Domainnamen und ein Zertifikat, dem der Browser vertraut.
- **Android-App**: **Handys können sich mit den zwölf Wörtern der Gruppe koppeln**, womit sich ein gekoppeltes Handy ohne das Passwort anmelden kann.
- **Verschlüsselung**: **Sicherungen verschlüsseln**, standardmäßig an.

Mehreres braucht ein Login-Passwort, etwa das Recovery-Kit, Schlüsselhinterlegungen und das endgültige Löschen von Sicherungen. Siehe [Sicherheitsmodell](configuration.md#security-model).

## Instanzen {#instances}

- **Auch in der Seitenleiste zeigen** heftet die Seite Instanzen an die Seitenleiste.
- **Mehrere Server** schaltet die Teile einer Gruppe ein: **Instanzen**, die Seite mit einer Karte für jedes Mitglied, **Empfänger** und **Holen**. Siehe [Instanzen und Rollen](offsite-recovery.md#roles).
- Ein Link auf die Seite Instanzen, mit den Teilen, die an sind.

## Kopplung {#pairing}

**Kopplung** mit **Phrase generieren** oder **Phrase eingeben**, den Schrittkarten, dem Namen dieser Instanz und den **Bekannten Adressen**, die Karte **Relay** mit ihren vier Formen und **Was das Relay sieht**, die Mitglieder der Gruppe, und **ZFS-Server ohne BombVault** für ein Replikziel über SSH. Siehe [Instanzen koppeln](offsite-recovery.md#pairing) und [Auf einen ZFS-Server schieben](zfs-datasets.md#replica-push).

## Anbindungen {#integrations}

- **MCP-Server**: Schlüssel für KI-Assistenten, **Client wählen** und das Protokoll jedes Schlüssels. Siehe [MCP-Server](mcp.md).
- **API-Tokens**: benannte Tokens für Skripte und Dashboards. Siehe [Tokens](api.md#tokens).
- **Dienste**: **Home Assistant** über MQTT, **Im Netzwerk finden** (mDNS als `bombvault.local`) und **/metrics bereitstellen** für Prometheus, mit einem **Scrape-Token (optional)**. Siehe [Home Assistant](api.md#home-assistant) und [BombVault im Netzwerk finden](api.md#mdns).
- **Dashboard-Widget**: ein schreibgeschütztes Token und eine Adresse für jedes Dashboard, das einen iframe zeigt, etwa Homepage oder Heimdall, nur mit dem Live-Aktivitätsprotokoll. Siehe [Dashboard-Widget](api.md#dashboard-widget).
- **Host-SSH**: der öffentliche Schlüssel von BombVault und **Verbindung testen**, für VM- und ZFS-Dataset-Sicherungen und Unraid-Benachrichtigungen. Siehe die [Anleitung zur VM-Sicherung](vm-backup-ssh-setup.md).

## Apps {#apps}

- **Android-App**: die APK für die Version, die dein Server betreibt, mit einem QR-Code, und der F-Droid-Eintrag. Siehe [Android-App](android.md#install).
- **ParleyPort**: der Relay-Container mit seinem Unraid-Template und dem Docker-Befehl, der ihn startet.
- **BombVault Widget**: eine Kachel im Unraid-Dashboard mit dem Aktivitätsprotokoll von BombVault und dem nächsten Lauf. Mit einer Host-SSH-Verbindung installiert und entfernt die Karte das Plugin selbst. Ohne sie gibt sie dir die `.plg`-Adresse zum Installieren unter **Plugins, Install Plugin**.

## System {#system}

- **Einstellungen exportieren / importieren**: deine ganze Konfiguration als portable JSON-Datei, mit oder ohne die Zugangsdaten. Der Import zeigt zuerst **Diese Datei enthält** und lässt Links auf Speicherorte weg, die es nicht mehr gibt. **Exporte mit age verschlüsseln** versiegelt die einfachen Exporte, das Flash-ZIP und das Recovery-Kit für die **age-Empfänger**, die du aufführst. Siehe [Portable Einstellungen](configuration.md#portable-settings-export-and-import).
- **Leistung**: **CPU-Threads (0 = alle)**, **Cache-Größenlimit (MB, 0 = unbegrenzt)** für den restic-Cache unter `/config` und **Speicherlimit pro Sicherung (MiB, 0 = keins)**, ein weiches Limit, das eine große Sicherung verlangsamt, statt sie wegen Speichermangels abzuschießen.

## Info {#info}

- **Version**: BombVault, GlimStone, restic und rclone, mit **Nach Updates suchen** und **Neu**, das die Änderungen zählt, die du noch nicht gesehen hast, und ein Fenster öffnet, das sie auflistet.
- **Hilfe**: **Handbuch** öffnet diese Dokumentation, und **Fehlerbericht** lädt eine bereinigte ZIP für ein GitHub-Issue herunter. Es braucht ein Login-Passwort.
- **Über BombVault**: das Projekt, seine Lizenz und wie du den Autor erreichst.
