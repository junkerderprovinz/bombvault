# Android-App

Die Android-App bringt jeden BombVault-Server deiner Gruppe aufs Handy. Sie startet mit einer Liste deiner Server, darüber das Aktivitätsprotokoll von allen, dieselben Zeilen wie in der Übersicht, und öffnet die Handy-Ansicht des Servers, auf den du tippst. Die App sichert selbst nichts.

![Der Startbildschirm der App: oben das Aktivitätsprotokoll aller Server, darunter die Liste der Server.](assets/screenshots/android/start-dark.png#only-dark)
![Der Startbildschirm der App: oben das Aktivitätsprotokoll aller Server, darunter die Liste der Server.](assets/screenshots/android/start-light.png#only-light)

## Die App besorgen {#install}

- **Einstellungen, Apps:** Die Karte der Android-App bietet die APK für die Version, die dein Server betreibt, mit einem QR-Code zum Scannen vom Handy, und verlinkt den F-Droid-Eintrag.
- **APK:** Jede Version hat auf ihrer Release-Seite `bombvault-android.apk`, und [dieser Link](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) lädt immer den neuesten Build herunter. Sie braucht Android 10 oder neuer, und Android fragt einmal, ob die App, mit der du die Datei öffnest, Apps installieren darf.
- **Google Play:** [BombVault auf Google Play](https://play.google.com/store/apps/details?id=bombvault.halleluja.design). Die App ist mit demselben Schlüssel signiert wie die APK, also lässt sich die eine über die andere installieren.
- **F-Droid:** [BombVault auf F-Droid](https://f-droid.org/packages/bombvault.halleluja.design/). F-Droid baut die App aus dem Quellcode und signiert sie mit seinem eigenen Schlüssel, sodass eine neue Version ein paar Tage später dort ankommt. Um zwischen der F-Droid-App und der APK zu wechseln, deinstalliere zuerst die jeweils andere.

Koppele die App mit Servern ab Version 9.7.0. Ein Server mit einer älteren Version kann in derselben Gruppe sein, zeigt das Handy aber als gewöhnliche Instanz, und die App liest die Aktivität dieses Servers erst nach einer Anmeldung.

## Koppeln per QR-Code {#pairing}

1. Öffne auf einem beliebigen Server deiner Gruppe **Einstellungen, Kopplung** und wähle **Phrase zeigen**. Die zwölf Wörter erscheinen mit einem QR-Code daneben.
2. Tippe in der App auf **QR-Code scannen** und richte das Handy auf den Code. Du kannst die Wörter auch einfügen oder eintippen.
3. Die App listet die Server dieser Gruppe auf, bevor sie etwas speichert. **Alle übernehmen** fügt jeden davon hinzu.

Das Handy tritt der Gruppe dann wie eine weitere Instanz bei. Es liest über die Gruppe ohne Anmeldung, was auf jedem Server läuft, zu Hause direkt und unterwegs über das Relay. Wie die Gruppe selbst funktioniert, beschreibt [Instanzen koppeln](offsite-recovery.md#pairing).

Die App tritt der Gruppe über das Relay bei, auch zu Hause. Wenn deine Instanzen **Kein Relay** nutzen, wähle unter **Einstellungen, Kopplung** eines aus, bevor du den Code scannst.

## Unterwegs {#away}

Die App versucht es zuerst mit einem Server unter seiner eigenen Adresse. Antwortet die nicht, etwa im Mobilfunknetz, öffnet sie stattdessen die Oberfläche des Servers über das Relay der Gruppe. Ein Server der Gruppe, dessen Adresse die App noch nicht kennt, wird genauso geöffnet. Antworten weder die Adresse noch das Relay, sagt der Startbildschirm das auf der Karte des Servers.

Über das Relay erreicht die Seite, was ein gekoppeltes Handy zu Hause erreicht, siehe unten. Das Anmelden mit dem Passwort und das Speichern von Dateien funktionieren nur unter der eigenen Adresse des Servers. Fortschrittsbalken aktualisieren sich alle zwei Sekunden statt fortlaufend. Die Dateien der Oberfläche bleiben nach dem ersten Besuch auf dem Handy, spätere Besuche laden also schneller. Ein Server älter als 10.0.0 öffnet sich nur unter seiner eigenen Adresse.

Die App nimmt eine Antwort nur an, wenn der Server sie mit dem Schlüssel signiert hat, den die App beim ersten Kennenlernen dieses Servers festgehalten hat. Antwortet der Server später mit einem anderen Schlüssel, zeigt der Startbildschirm den Fingerabdruck des neuen Schlüssels, und die Seite öffnet sich erst, nachdem du ihm vertraust.

## Server wechseln {#switch}

Bei einem geöffneten Server zeigt der Kopf seiner Übersicht den Namen des Servers, mit einem Zurück-Pfeil davor. Der Pfeil führt zur Übersicht der App, dem Startbildschirm, der jeden Server auflistet, den die App kennt, mit seinem Zustand: **Verbunden**, **Nicht verbunden**, **Anmeldung nötig** oder **Zertifikat prüfen**. Ein Server der Gruppe, der nicht antwortet, ist **Inaktiv**, in Grau, mit dem Zeitpunkt, zu dem die Gruppe ihn zuletzt gesehen hat, und einer, von dem die App noch nichts gehört hat, sagt **Wird geprüft**. Bei jeder Verbindung liest die App die Liste der Gruppe von einer ihrer Instanzen, sodass auch eine Instanz aufgelistet wird, der sie auf dem Relay nie begegnet ist, etwa eine, die beim Beitritt des Handys ausgeschaltet war. Tippe dort auf einen anderen, um ihn zu öffnen, zu Hause direkt und sonst über das Relay.

## Auf einem gekoppelten Handy angemeldet {#sign-in}

Ein mit deiner Gruppe gekoppeltes Handy öffnet jeden ihrer Server schon angemeldet. Bevor es eine Seite lädt, fragt es diesen Server über die Gruppe nach einer Sitzung, und ein Server gibt eine nur an ein Mitglied heraus, das ein Handy ist.

Ein gekoppeltes Handy bekommt nicht alles, was ein Passwort bekommt. Seine Sitzung reicht an eine feste Liste dessen, was die App zeigt: die Übersicht, die Läufe und das Aktivitätsprotokoll, die Sicherungen zum Ansehen, Starten und Wiederherstellen, die Anomalien zum Lesen und die Seite Instanzen zum Ansehen, ohne die empfangenen Repositories und Pull-Quellen ihrer Mitglieder. Speicherorte und Befehle kommen ohne ihre Adressen und Geheimnisse zurück. Eine Wiederherstellung, die das Original ersetzt, sichert vorher den heutigen Zustand. Eine Wiederherstellung in einen Ordner landet in einem neuen Ordner innerhalb des Ordners für Wiederherstellungen, der unter **Einstellungen, Wiederherstellung** eingestellt ist, und das Fenster bietet keinen anderen an. Schlüssel, das Recovery-Kit, Schlüsselhinterlegungen, Einstellungen und das Löschen von Sicherungen bleiben einer Anmeldung am Server selbst vorbehalten, und alles andere antwortet, dass es diese Anmeldung braucht. Über das Relay gilt dieselbe Liste, auch auf einem Server ohne Passwort.

Der Schalter dafür ist **Handys können sich mit den zwölf Wörtern der Gruppe koppeln** in der Karte **Android-App** unter **Einstellungen, Sicherheit**. Ein Passwort-Reset schaltet ihn aus und meldet jedes Handy ab, bis du ihn dort wieder einschaltest.

## Zertifikate {#certificates}

Ein selbstsigniertes Zertifikat wird einmal über seinen SHA-256-Fingerabdruck als vertrauenswürdig eingestuft. Zeigt der Server später ein anderes, warnt dich die App und öffnet ihn erst, nachdem du dem neuen Zertifikat vertraust.

## Das Handy auf der Seite Instanzen {#instances}

Das Handy bekommt auf der Seite Instanzen jedes Servers der Gruppe eine eigene Karte, als Android-App gekennzeichnet und nach dem Namen benannt, den du dem Handy gegeben hast. Sie hat keine Bewertungskarte, weil es nichts sichert. **Entfernen** nimmt es von der Seite und beendet sofort seine Sitzung, genauso wie das Ausschalten von **Diese Instanz abfragen** unter **Überwachung** in seinen Details. Das Handy kennt die Wörter weiter, taucht also wieder auf, wenn seine App sich öffnet, aber ausgeschaltet, und es bekommt keine Sitzung, bis jemand es einschaltet. Bei einem verlorenen Handy entferne seine Karte auf jedem Server, oder schalte **Handys können sich mit den zwölf Wörtern der Gruppe koppeln** unter **Einstellungen, Sicherheit** aus, oder lass jede Instanz **Gruppe verlassen** und koppele sie mit neuen Wörtern neu.

Ein Handy, das vor 10.0.0 gekoppelt wurde, beginnt bei seinem ersten Besuch nach dem Update, seine Aufrufe zu signieren, und jeder Server behält den ersten Schlüssel, den er für das Handy bekommt. Kommt jemand anderes, der die Wörter der Gruppe kennt, zuerst dorthin, während das Handy offline ist, weist dieser Server das Handy von da an ab, und die App kann ihn über das Relay nicht mehr öffnen. Zur Behebung melde dich an diesem Server in einem Browser an, wähle auf der Karte des Handys auf der Seite Instanzen **Schlüssel vergessen** und dann **Vergessen bestätigen**, und öffne die App wieder. Der Server nimmt bei seinem nächsten Aufruf den eigenen Schlüssel des Handys. Wird das Handy wieder abgewiesen, hat jemand anderes die Wörter: Lass jede Instanz **Gruppe verlassen** und koppele sie mit neuen Wörtern neu, wie bei einem verlorenen Handy.

## Einstellungen {#settings}

Das Zahnrad neben dem Plus öffnet die Einstellungen der App:

- die Sprache und den Namen, den das Handy auf der Seite Instanzen zeigt (leer heißt: das Modell des Handys),
- das Aussehen, das dem ersten Server der Liste folgt, bis du ein eigenes festlegst, und Animationen, die eine eigene Einstellung haben,
- einen Bericht zum Kopieren, wenn du ein Problem meldest. Er enthält keine Adresse, keinen Namen und keine Phrase,
- die Karte Über mit der Datenschutzerklärung,
- **Alle Server entfernen**, was jeden Server aus der App entfernt und die Gruppe verlässt. Jeder Server, den das Handy noch erreicht, nimmt es von seiner Seite Instanzen.

## Downloads und Uploads {#files}

Exporte, Recovery-Kits, Flash-ZIPs und Datenbank-Dumps landen im Ordner Downloads des Handys, wie bei einem Browser. Ein Import von Einstellungen öffnet die Dateiauswahl des Handys. Über das Relay wird keine Datei gespeichert. Öffne dafür den Server unter seiner eigenen Adresse.

## Auf einem Touchscreen {#touch}

Unter einem Finger schwebt nichts, daher wird ein Bedienelement abgedunkelt, solange man es hält, und ein Knopf mit einem Markenlogo leuchtet in seiner Markenfarbe, bis der Finger sich hebt. Ein langer Druck auf einen Knopf gilt als langsamer Tipp und öffnet kein Link-Menü. Ein Tipp auf einen ausgegrauten Knopf zeigt, warum er ausgegraut ist, bis du irgendwo anders tippst.

## Stattdessen im Browser {#browser}

Chrome und Edge können die Weboberfläche von BombVault als App in einem eigenen Fenster installieren, auf dem Handy wie auf dem Computer. Es wird nichts zwischengespeichert, ein Update zeigt sich also sofort.

## Datenschutz {#privacy}

Die App hat keine Konten, keine Werbung und keine Analyse, und sie führt nichts im Hintergrund aus. Ihre [Datenschutzerklärung](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) listet auf, was sie speichert und was sie wohin sendet.
