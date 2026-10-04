# Android-App

Die Android-App holt jeden BombVault-Server deiner Gruppe aufs Handy. Sie startet mit einer Liste deiner Server, darüber das Aktivitätsprotokoll aller Server mit denselben Zeilen, die das Dashboard zeigt, und öffnet die Handy-Ansicht des Servers, den du antippst. Die App selbst sichert nichts.

## Die App bekommen {#install}

- **APK:** Jedes Release hat `bombvault-android.apk` auf seiner Release-Seite, und [dieser Link](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) lädt immer den neuesten Build. Die App braucht Android 10 oder neuer, und Android fragt einmal, ob die App, mit der du die Datei öffnest, Apps installieren darf.
- **Google Play:** Die App ist in einem geschlossenen Test, bis sie öffentlich werden kann. Google Play listet eine App von einem neuen Entwicklerkonto erst, nachdem mindestens 12 Tester sie 14 Tage lang installiert hatten. Wenn du helfen willst, tritt der [Testergruppe](https://groups.google.com/g/arrowloop-testers) bei, öffne die [Testseite](https://play.google.com/apps/testing/bombvault.halleluja.design), tippe auf **Tester werden** und installiere BombVault aus Google Play.
- **F-Droid:** Der Eintrag folgt.

Kopple die App mit Servern ab Version 9.7.0. Ein Server mit einer älteren Version kann in derselben Gruppe sein, zeigt das Handy aber als gewöhnliche Instanz, und die App liest die Aktivität dieses Servers erst nach einer Anmeldung.

## Kopplung per QR-Code {#pairing}

1. Öffne auf einem beliebigen Server deiner Gruppe **Einstellungen, Kopplung** und wähle **Phrase zeigen**. Die zwölf Wörter erscheinen mit einem QR-Code daneben.
2. Tippe in der App auf **QR-Code scannen** und richte das Handy auf den Code. Du kannst die Wörter auch einfügen oder eintippen.
3. Die App listet die Server dieser Gruppe auf, bevor sie etwas speichert. **Alle übernehmen** fügt jeden davon hinzu.

Danach tritt das Handy der Gruppe bei wie eine weitere Instanz. Es liest über die Gruppe ohne Anmeldung, was auf jedem Server läuft, zu Hause direkt und unterwegs über das Relay. Wie die Gruppe selbst funktioniert, steht unter [Instanzen koppeln](offsite-recovery.md#pairing).

!!! note "Die Oberfläche braucht trotzdem einen Weg zum Server"
    Die Serverliste und das Aktivitätsprotokoll kommen über die Gruppe. Die Oberfläche eines Servers öffnet sich direkt, das Handy muss die Adresse des Servers also erreichen, zu Hause oder über ein VPN.

## Angemeldet auf einem gekoppelten Handy {#sign-in}

Ein mit deiner Gruppe gekoppeltes Handy öffnet jeden ihrer Server bereits angemeldet. Bevor es eine Seite lädt, bittet es diesen Server über die Gruppe um eine Sitzung, und ein Server gibt eine nur an ein Mitglied heraus, das ein Handy ist. Ein Server, den du über seine Adresse hinzugefügt hast, fragt wie im Browser nach dem Passwort. Wer die zwölf Wörter hat, kann ohnehin schon jedes Backup der Gruppe öffnen, die Kopplung gewährt also nichts Neues.

## Server außerhalb einer Gruppe {#other-servers}

- **Server hinzufügen** nimmt die Adresse, mit der du BombVault im Browser öffnest, etwa `192.168.1.10:3443`. Ohne `http://` oder `https://` davor nimmt die App https.
- Server, die sich im lokalen Netz bekannt geben, stehen unter **In diesem Netz** und öffnen sich mit einem Tippen. Das tun sie, solange **Im Netzwerk finden** unter Einstellungen, Anbindungen eingeschaltet ist. Ein Server in einem anderen Netz oder hinter einem VPN taucht dort nicht auf.
- Einem selbstsignierten Zertifikat vertraut die App einmal anhand seines SHA-256-Fingerabdrucks. Zeigt der Server später ein anderes, warnt die App dich und öffnet ihn erst, wenn du dem neuen Zertifikat vertraust.

## Das Handy auf der Instanzen-Seite {#instances}

Das Handy bekommt auf der Instanzen-Seite jedes Servers der Gruppe eine eigene Karte, als Android-App gekennzeichnet und mit dem Namen, den du dem Handy gegeben hast. Es hat keine Scorecard, weil es nichts sichert, und **Entfernen** nimmt es von der Seite.

## Einstellungen {#settings}

Das Zahnrad neben dem Plus öffnet die Einstellungen der App:

- die Sprache und der Name, den das Handy auf der Instanzen-Seite zeigt (leer heißt: das Modell des Handys),
- das Aussehen, das dem ersten Server der Liste folgt, bis du ein eigenes einstellst, und die Animationen, die eine eigene Einstellung haben,
- ein Bericht zum Kopieren, wenn du ein Problem meldest; er enthält keine Adresse, keinen Namen und keine Phrase,
- die Karte Über diese App mit der Datenschutzerklärung,
- **Alle Server entfernen**, das jeden Server aus der App entfernt und die Gruppe verlässt. Auf den Servern selbst ändert sich nichts.

## Downloads und Uploads {#files}

Exporte, Recovery-Kits, Flash-ZIPs und Datenbank-Dumps landen im Download-Ordner des Handys, wie aus einem Browser. Ein Einstellungs-Import öffnet die Dateiauswahl des Handys.

## Auf einem Touchscreen {#touch}

Unter einem Finger schwebt nichts, deshalb wird ein Bedienelement abgedunkelt, solange es gehalten wird, und ein Knopf mit Markenlogo leuchtet in seiner Markenfarbe, bis der Finger loslässt. Ein langes Drücken auf einen Knopf zählt als langsames Tippen und öffnet kein Link-Menü.

## Stattdessen im Browser {#browser}

Chrome und Edge können BombVaults Web-Oberfläche als App in einem eigenen Fenster installieren, auf dem Handy wie auf dem Computer. Nichts wird zwischengespeichert, ein Update ist also sofort zu sehen.

## Datenschutz {#privacy}

Die App hat keine Konten, keine Werbung und keine Analyse, und sie führt nichts im Hintergrund aus. Ihre [Datenschutzerklärung](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) führt auf, was sie speichert und was sie wohin sendet.
