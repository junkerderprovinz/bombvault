# Android-App

Die Android-App holt jeden BombVault-Server deiner Gruppe aufs Handy. Sie startet mit einer Liste deiner Server, darüber das Aktivitätsprotokoll aller Server mit denselben Zeilen, die die Übersicht zeigt, und öffnet die Handy-Ansicht des Servers, den du antippst. Die App selbst sichert nichts.

## Die App bekommen {#install}

- **Einstellungen, Apps:** Die Karte der Android-App bietet die APK für das Release an, das auf deinem Server läuft, mit einem QR-Code zum Scannen am Handy.
- **APK:** Jedes Release hat `bombvault-android.apk` auf seiner Release-Seite, und [dieser Link](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) lädt immer den neuesten Build. Die App braucht Android 10 oder neuer, und Android fragt einmal, ob die App, mit der du die Datei öffnest, Apps installieren darf.
- **Google Play:** Die App ist in einem geschlossenen Test, bis sie öffentlich werden kann. Google Play listet eine App von einem neuen Entwicklerkonto erst, nachdem mindestens 12 Tester sie 14 Tage lang installiert hatten. Wenn du helfen willst, tritt der [Testergruppe](https://groups.google.com/g/arrowloop-testers) bei, öffne die [Testseite](https://play.google.com/apps/testing/bombvault.halleluja.design), tippe auf **Tester werden** und installiere BombVault aus Google Play.
- **F-Droid:** [BombVault auf F-Droid](https://f-droid.org/packages/bombvault.halleluja.design/). F-Droid baut die App selbst aus dem Quellcode und signiert sie mit einem eigenen Schlüssel, deshalb kommt ein neues Release dort ein paar Tage später an. Willst du zwischen der F-Droid-App und der APK wechseln, deinstalliere vorher die andere.

Kopple die App mit Servern ab Version 9.7.0. Ein Server mit einer älteren Version kann in derselben Gruppe sein, zeigt das Handy aber als gewöhnliche Instanz, und die App liest die Aktivität dieses Servers erst nach einer Anmeldung.

## Kopplung per QR-Code {#pairing}

1. Öffne auf einem beliebigen Server deiner Gruppe **Einstellungen, Kopplung** und wähle **Phrase zeigen**. Die zwölf Wörter erscheinen mit einem QR-Code daneben.
2. Tippe in der App auf **QR-Code scannen** und richte das Handy auf den Code. Du kannst die Wörter auch einfügen oder eintippen.
3. Die App listet die Server dieser Gruppe auf, bevor sie etwas speichert. **Alle übernehmen** fügt jeden davon hinzu.

Danach tritt das Handy der Gruppe bei wie eine weitere Instanz. Es liest über die Gruppe ohne Anmeldung, was auf jedem Server läuft, zu Hause direkt und unterwegs über das Relay. Wie die Gruppe selbst funktioniert, steht unter [Instanzen koppeln](offsite-recovery.md#pairing).

Die App tritt der Gruppe über das Relay bei, auch zu Hause. Steht deine Instanz auf **Kein Relay**, wähl unter **Einstellungen, Kopplung** eines aus, bevor du den Code scannst.

## Unterwegs {#away}

Die App versucht einen Server zuerst unter seiner eigenen Adresse. Antwortet die nicht, etwa im Mobilnetz, öffnet sie die Oberfläche des Servers stattdessen über das Relay der Gruppe. Ein Server der Gruppe, dessen Adresse die App noch nicht kennt, öffnet sich genauso. Antworten weder die Adresse noch das Relay, steht das auf dem Startbildschirm an der Karte des Servers.

Über das Relay erreicht die Seite das, was ein gekoppeltes Handy zu Hause erreicht, wie unten beschrieben. Anmelden mit dem Passwort und Dateien speichern gehen nur unter der eigenen Adresse des Servers. Fortschrittsbalken laufen alle zwei Sekunden weiter statt fortlaufend. Die Dateien der Oberfläche bleiben nach dem ersten Besuch auf dem Handy, spätere Besuche laden deshalb schneller. Ein Server vor 10.0.0 öffnet sich nur unter seiner eigenen Adresse.

Die App nimmt eine Antwort nur an, wenn der Server sie mit dem Schlüssel signiert hat, den die App beim ersten Kontakt mit diesem Server festgehalten hat. Antwortet der Server später mit einem anderen Schlüssel, zeigt der Startbildschirm den Fingerabdruck des neuen Schlüssels, und die Seite öffnet sich erst, wenn du ihm vertraust.

## Server wechseln {#switch}

Ist ein Server offen, steht oben in seiner Übersicht sein Name, davor ein Pfeil zurück. Der Pfeil führt zur Übersicht der App, dem Startbildschirm, der jeden Server, den die App kennt, mit seinem Zustand listet: **Verbunden**, **Nicht verbunden**, **Anmeldung nötig** oder **Zertifikat prüfen**. Ein Server der Gruppe, der nicht antwortet, ist **Inaktiv**, in Grau und mit der Angabe, wann die Gruppe ihn zuletzt gesehen hat, und einer, von dem die App noch nichts gehört hat, zeigt **Wird geprüft**. Bei jeder Verbindung liest die App die Liste der Gruppe bei einer ihrer Instanzen, deshalb steht auch eine Instanz in der Liste, die sie über das Relay nie getroffen hat, etwa eine, die beim Koppeln des Handys ausgeschaltet war. Tippe dort auf einen anderen, um ihn zu öffnen, zu Hause direkt und sonst über das Relay.

## Angemeldet auf einem gekoppelten Handy {#sign-in}

Ein mit deiner Gruppe gekoppeltes Handy öffnet jeden ihrer Server bereits angemeldet. Bevor es eine Seite lädt, bittet es diesen Server über die Gruppe um eine Sitzung, und ein Server gibt eine nur an ein Mitglied heraus, das ein Handy ist. Ein Server, den du über seine Adresse hinzugefügt hast, fragt wie im Browser nach dem Passwort.

Ein gekoppeltes Handy bekommt nicht alles, was ein Passwort öffnet. Seine Sitzung erreicht eine feste Liste dessen, was die App zeigt: die Übersicht, die Läufe und das Aktivitätsprotokoll, die Sicherungen zum Ansehen, Starten und Wiederherstellen, die Anomalien zum Lesen und die Seite Instanzen zum Ansehen. Speicherorte und Befehle kommen ohne ihre Adressen und Geheimnisse an. Eine Wiederherstellung, die das Original ersetzt, sichert vorher den heutigen Zustand. Schlüssel, das Recovery-Kit, Hinterlegungen, Einstellungen und das Löschen von Sicherungen bleiben einer Anmeldung am Server selbst vorbehalten, und alles andere antwortet, dass es diese Anmeldung braucht. Über das Relay gilt dieselbe Liste, auch auf einem Server ohne Passwort.

Der Schalter dafür ist **Handys können sich mit den zwölf Wörtern der Gruppe koppeln** auf der Karte **Android-App** unter **Einstellungen, Sicherheit**. Ein Zurücksetzen des Passworts schaltet ihn ab und meldet jedes Handy ab, bis du ihn dort wieder einschaltest.

## Server außerhalb einer Gruppe {#other-servers}

- **Server hinzufügen** nimmt die Adresse, mit der du BombVault im Browser öffnest, etwa `192.168.1.10:3443`. Ohne `http://` oder `https://` davor nimmt die App https.
- Server, die sich im lokalen Netz bekannt geben, stehen unter **In diesem Netz** und öffnen sich mit einem Tippen. Das tun sie, solange **Im Netzwerk finden** auf der Karte **Dienste** unter **Einstellungen, Anbindungen** eingeschaltet ist. Ein Server in einem anderen Netz oder hinter einem VPN taucht dort nicht auf.
- Einem selbstsignierten Zertifikat vertraut die App einmal anhand seines SHA-256-Fingerabdrucks. Zeigt der Server später ein anderes, warnt die App dich und öffnet ihn erst, wenn du dem neuen Zertifikat vertraust.

## Das Handy auf der Instanzen-Seite {#instances}

Das Handy bekommt auf der Instanzen-Seite jedes Servers der Gruppe eine eigene Karte, als Android-App gekennzeichnet und mit dem Namen, den du dem Handy gegeben hast. Es hat keine Scorecard, weil es nichts sichert. **Entfernen** nimmt es von der Seite und beendet sofort seine Sitzung, genauso wie das Ausschalten von **Diese Instanz abfragen** unter **Überwachung** in seinen Details. Das Handy kennt die Wörter weiter, taucht also wieder auf, wenn seine App öffnet, aber ausgeschaltet, und bekommt keine Sitzung, bis es jemand einschaltet. Für ein verlorenes Handy entfernst du seine Karte auf jedem Server, schaltest **Handys können sich mit den zwölf Wörtern der Gruppe koppeln** unter **Einstellungen, Sicherheit** aus, oder du nimmst jede Instanz mit **Gruppe verlassen** aus der Gruppe und koppelst sie mit neuen Wörtern neu.

Ein Handy, das vor 10.0.0 gekoppelt wurde, signiert seine Aufrufe ab dem ersten Besuch nach dem Update, und jeder Server behält den ersten Schlüssel, den er für das Handy bekommt. Kommt jemand anderes mit den Wörtern der Gruppe dem Handy zuvor, während es offline ist, weist dieser Server das Handy von da an ab, und die App kann ihn über das Relay nicht mehr öffnen. Melde dich dann im Browser an diesem Server an, wähl auf der Instanzen-Seite an der Karte des Handys **Schlüssel vergessen** und danach **Vergessen bestätigen**, und öffne die App neu. Beim nächsten Aufruf übernimmt der Server den eigenen Schlüssel des Handys. Wird das Handy wieder abgewiesen, hat noch jemand die Wörter. Nimm dann wie bei einem verlorenen Handy jede Instanz mit **Gruppe verlassen** aus der Gruppe und kopple sie mit neuen Wörtern neu.

## Einstellungen {#settings}

Das Zahnrad neben dem Plus öffnet die Einstellungen der App:

- die Sprache und der Name, den das Handy auf der Instanzen-Seite zeigt (leer heißt: das Modell des Handys),
- das Aussehen, das dem ersten Server der Liste folgt, bis du ein eigenes einstellst, und die Animationen, die eine eigene Einstellung haben,
- ein Bericht zum Kopieren, wenn du ein Problem meldest; er enthält keine Adresse, keinen Namen und keine Phrase,
- die Karte Über diese App mit der Datenschutzerklärung,
- **Alle Server entfernen**, das jeden Server aus der App entfernt und die Gruppe verlässt. Jeder Server, den das Handy noch erreicht, nimmt es von seiner Instanzen-Seite.

## Downloads und Uploads {#files}

Exporte, Recovery-Kits, Flash-ZIPs und Datenbank-Dumps landen im Download-Ordner des Handys, wie aus einem Browser. Ein Einstellungs-Import öffnet die Dateiauswahl des Handys. Über das Relay speichert die App keine Datei; öffne den Server dafür unter seiner eigenen Adresse.

## Auf einem Touchscreen {#touch}

Unter einem Finger schwebt nichts, deshalb wird ein Bedienelement abgedunkelt, solange es gehalten wird, und ein Knopf mit Markenlogo leuchtet in seiner Markenfarbe, bis der Finger loslässt. Ein langes Drücken auf einen Knopf zählt als langsames Tippen und öffnet kein Link-Menü. Ein Tippen auf einen ausgegrauten Knopf zeigt, warum er ausgegraut ist, bis du woanders hintippst.

## Stattdessen im Browser {#browser}

Chrome und Edge können BombVaults Web-Oberfläche als App in einem eigenen Fenster installieren, auf dem Handy wie auf dem Computer. Nichts wird zwischengespeichert, ein Update ist also sofort zu sehen.

## Datenschutz {#privacy}

Die App hat keine Konten, keine Werbung und keine Analyse, und sie führt nichts im Hintergrund aus. Ihre [Datenschutzerklärung](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) führt auf, was sie speichert und was sie wohin sendet.
