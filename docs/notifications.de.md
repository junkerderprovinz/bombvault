# Benachrichtigungen

BombVault kann dir Bescheid geben, wenn eine Sicherung fehlschlägt, wenn sie klappt, wenn eine überfällig ist, und einmal pro Woche, wie alles lief. Alles wird unter **Einstellungen, Benachrichtigungen** eingestellt: was eine Nachricht auslöst in der Karte **Benachrichtigungen**, wohin die Nachrichten gehen in der Karte **Kanäle**, und die Check-in-Pings in der Karte **Healthchecks**.

![Einstellungen, Benachrichtigungen mit ausgefülltem Gotify-Kanal.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Einstellungen, Benachrichtigungen mit ausgefülltem Gotify-Kanal.](assets/screenshots/notifications/gotify-light.png#only-light)

## Was eine Nachricht auslöst {#what}

Die Karte **Benachrichtigungen** legt die Regeln für alle Kanäle auf einmal fest:

- **Benachrichtigen**: **Nie**, **Nur bei Fehler** oder **Bei Erfolg und Fehler**, für alle Kanäle weiter unten. Eine frische Installation startet mit **Nur bei Fehler**, ein Kanal, den du ausfüllst, sagt dir also Bescheid, wenn eine Sicherung schiefgeht.
- **Geplante Läufe zusammenfassen**: eine Nachricht pro geplantem Lauf, etwa *42 von 45 erfolgreich*, statt einer pro Container oder VM. Eine Sicherung, die du von Hand startest, schickt weiter ihre eigene Nachricht. Ein eingefrorener Gast, eine fehlgeschlagene Aufbewahrung oder ein überschrittenes Wachstumsbudget gehen auch bei eingeschalteten Zusammenfassungen sofort hinaus.
- **Bei Container-Update benachrichtigen**: eine Nachricht für jeden Container, den **Container nach erfolgreicher Sicherung aktualisieren** auf ein neueres Image gehoben hat, damit du prüfen kannst, ob er noch läuft.
- **Überfällige Sicherungen melden**: Einmal am Tag um 09:00 prüft BombVault, ob eine eingeschaltete Sicherung überfällig ist, das heißt älter als das Doppelte ihres Zeitplans, und schickt pro Fall eine Nachricht, die den Eintrag nennt, der zurückgefallen ist. Die nächste erfolgreiche Sicherung hebt die Meldung auf.
- **Unraid-Benachrichtigungen**: sendet an das eigene Benachrichtigungssystem von Unraid, das an Pushover, E-Mail oder Discord weiterleiten kann. Das läuft über die SSH-Verbindung unter **Einstellungen, Anbindungen, Host-SSH**, dieser Schlüssel muss also freigegeben sein. Auf TrueNAS und anderen Hosts bleibt es aus.
- **Wochenbericht**: eine Zusammenfassung pro Woche mit Laufzahlen, neuen Sicherungsdaten, welche Bereiche überfällig sind, ob die Off-site-Kopien aktuell sind, und den häufigsten Fehlern. Sein Plan ist ein Knopf wie jeder andere Plan, und **Aus** schaltet ihn ab. Siehe [Zeitpläne](schedules.md#plans).

Welche Anomalien eine Nachricht schicken, wird unter **Einstellungen, Prüfung** festgelegt, siehe [Anomalien](anomalies.md). Eine zurückgestellte Sicherung schickt eine Nachricht, wenn sie zu warten beginnt, und eine, wenn die Wartezeit endet, nicht eine pro Versuch.

**Test senden** am Ende der Karte schickt eine Testnachricht über jeden eingeschalteten Kanal, mit den Werten, wie sie gerade im Formular stehen, und nennt jeden, der fehlgeschlagen ist. Ein Healthchecks-Check bekommt einen Eintrag in seinem Log, der seinen Zustand lässt, wie er ist.

## Kanäle {#channels}

Jeder Kanal hat in der Karte **Kanäle** einen eigenen Schalter, und seine Felder erscheinen, sobald er an ist. Jeder Kanal sendet für sich mit eigenem Zeitlimit, sodass ein Host, der nie antwortet, die anderen nicht aufhält. Eine Warnung zu Überfälligem, zum Budget oder zu einer Anomalie, die kein Kanal angenommen hat, wird erneut versucht, und ein Wochenbericht ebenso. Die Nachricht zu einer einzelnen Sicherung oder eine Zusammenfassung wird einmal gesendet.

| Kanal | Felder | Hinweise |
|---|---|---|
| **Webhook** | **Webhook-URL**, **Webhook-Format** | Formate: **Generic JSON**, **Discord**, **Slack**, **Gotify** und **ntfy**. Generic JSON sendet `title`, `message` und `ok`. |
| **Apprise** | **Apprise-API-URL**, **Tags (optional)** | Richte es auf den `/notify/<key>`-Endpunkt deines Apprise-API-Containers, um seine über 100 Dienste zu erreichen. Tags wählen eine Teilmenge der Ziele dieses Schlüssels. |
| **Matrix** | **Homeserver-URL**, **Access-Token**, **Raum-ID** | Das Token bleibt auf dem Server und wird nie wieder angezeigt. |
| **Gotify** | **Server-URL**, **App-Token**, **Priorität bei Fehlschlag**, **Priorität bei Erfolg** | Die Prioritäten reichen von 0 bis 10 und beginnen bei 8 für Fehler und 5 für Erfolge. |
| **E-Mail (SMTP)** | **SMTP-Host**, **Port**, **Verschlüsselung**, **Benutzername**, **Passwort**, **Absenderadresse**, **Empfängeradresse** | **Verschlüsselung** ist **STARTTLS**, **TLS (implicit)** oder **None**. |

Geheimnisse wie das Matrix-Token, das Gotify-App-Token und das SMTP-Passwort werden verschlüsselt gespeichert. Unter einem gespeicherten Geheimnis steht im Feld **Gespeichert. Lass das Feld leer, um es zu behalten.** Wenn du die Adresse des Servers änderst, schickt BombVault das alte Geheimnis nicht an den neuen Ort: Das Feld fragt wieder danach, und die neue Adresse wird gespeichert, sobald du es eingibst.

## Healthchecks {#healthchecks}

Ein Dienst wie [Healthchecks.io](https://healthchecks.io) schlägt Alarm, wenn sich BombVault nicht mehr meldet. Füge die **Ping-URL** des Checks ein, und BombVault pingt sie über den ganzen Ablauf jeder Sicherung: beim Start, dann bei Erfolg oder mit `/fail`. Das passiert immer, wenn eine URL gesetzt ist, egal was bei **Benachrichtigen** steht, damit der Check bei Erfolg auch mit reinen Fehlermeldungen grün bleibt. Ein Bereich, der nirgendwohin sichern kann, zählt als fehlgeschlagen, sein Check wird also rot, statt einen Erfolg zu hören.

Bereiche mit derselben URL teilen sich einen Check. Ist eine Sicherung des einen fehlgeschlagen, pingen auch die guten Sicherungen der anderen `/fail` und nennen ihn, sodass der Check rot bleibt, bis dieser Eintrag wieder eine gute Sicherung hat oder du den Fehler unter **Braucht dich** quittierst.

Ein Bereich kann eine eigene Adresse pingen: **Ausnahme** in der Karte fügt eine hinzu, und die Zeile sagt **Alle anderen Bereiche pingen diese URL.**

## Beispiel: Gotify auf dem Handy {#example-gotify}

Du betreibst Gotify unter `https://gotify.example.com` und willst, dass Fehler auf dem Handy klingeln, während Erfolge leise ankommen.

1. Öffne in Gotify **Apps**, lege eine Anwendung namens `BombVault` an und kopiere ihr Token.
2. Öffne in BombVault **Einstellungen, Benachrichtigungen**. Stelle in **Benachrichtigungen** bei **Benachrichtigen** **Bei Erfolg und Fehler** ein und schalte **Geplante Läufe zusammenfassen** ein, damit eine Nacht mit 45 Sicherungen eine Nachricht schickt statt 45.
3. Schalte in **Kanäle** **Gotify** ein.
4. Gib `https://gotify.example.com` als **Server-URL** ein. BombVault hängt `/message` selbst an.
5. Füge das Token bei **App-Token** ein.
6. Lass **Priorität bei Fehlschlag** auf 8 und **Priorität bei Erfolg** auf 5. Die Android-App von Gotify behandelt 8 und höher als hoch, daher machen nur Fehler ein Geräusch.
7. Drücke **Test senden**. Die Zeile unter der Karte sagt, ob Gotify es angenommen hat, und ein abgelehntes Token wird als solches benannt.

Der nächste geplante Lauf schickt eine Zusammenfassung mit Priorität 5, oder mit 8, wenn darin etwas fehlgeschlagen ist.

## Beispiel: ntfy als Webhook {#example-ntfy}

ntfy braucht für ein öffentliches Topic kein Konto. Wähle einen Topic-Namen, der schwer zu erraten ist, denn jeder, der ihn kennt, kann mitlesen.

1. Schalte in **Kanäle** **Webhook** ein.
2. Gib die Adresse des Topics als **Webhook-URL** ein, etwa `https://ntfy.sh/bombvault-tower-7f3k`, oder die Adresse deines eigenen ntfy-Servers.
3. Stelle **Webhook-Format** auf **ntfy**.
4. Abonniere dasselbe Topic in der ntfy-App und drücke **Test senden**.

BombVault sendet die Nachricht als Body und ihren Titel in `X-Title`. Ein Fehler geht mit Priorität hoch und dem Tag `warning` hinaus, damit er in der App auffällt.

## Wenn Nachrichten nicht ankommen {#troubleshooting}

Siehe [Benachrichtigungen kommen nicht an](troubleshooting.md#notifications-do-not-arrive) unter Fehlersuche.
