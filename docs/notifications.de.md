# Benachrichtigungen

BombVault kann dir Bescheid geben, wenn eine Sicherung fehlschlägt, wenn sie klappt, wenn eine überfällig ist, und einmal pro Woche, wie alles lief. Alles wird unter **Einstellungen, Benachrichtigungen** eingestellt: was BombVault über einzelne Sicherungen hinaus schickt in der Karte **Benachrichtigungen**, und wohin die Nachrichten gehen in der Karte **Kanäle**. Jeder Kanal entscheidet selbst, wann er sich meldet, und auch ein Healthchecks-Check ist ein Kanal.

![Einstellungen, Benachrichtigungen mit ausgefülltem Gotify-Kanal.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Einstellungen, Benachrichtigungen mit ausgefülltem Gotify-Kanal.](assets/screenshots/notifications/gotify-light.png#only-light)

## Was eine Nachricht auslöst {#what}

Die Schalter in der Karte **Benachrichtigungen** gelten für alle Kanäle:

- **Geplante Läufe zusammenfassen**: eine Nachricht pro geplantem Lauf, etwa *42 von 45 erfolgreich*, statt einer pro Container oder VM. Eine Sicherung, die du von Hand startest, schickt weiter ihre eigene Nachricht. Ein eingefrorener Gast, eine fehlgeschlagene Aufbewahrung oder ein überschrittenes Wachstumsbudget gehen auch bei eingeschalteten Zusammenfassungen sofort hinaus.
- **Bei Container-Update benachrichtigen**: eine Nachricht für jeden Container, den **Container nach erfolgreicher Sicherung aktualisieren** auf ein neueres Image gehoben hat, damit du prüfen kannst, ob er noch läuft.
- **Überfällige Sicherungen melden**: Einmal am Tag um 09:00 prüft BombVault, ob eine eingeschaltete Sicherung überfällig ist, das heißt älter als das Doppelte ihres Zeitplans, und schickt pro Fall eine Nachricht, die den Eintrag nennt, der zurückgefallen ist. Die nächste erfolgreiche Sicherung hebt die Meldung auf.
- **Wochenbericht**: eine Zusammenfassung pro Woche mit Laufzahlen, neuen Sicherungsdaten, welche Bereiche überfällig sind, ob die Off-site-Kopien aktuell sind, und den häufigsten Fehlern. Sein Plan ist ein Knopf wie jeder andere Plan, und **Aus** schaltet ihn ab. Siehe [Zeitpläne](schedules.md#plans).

Ob ein Kanal auch von Erfolgen hört, stellst du am Kanal selbst unter **Benachrichtigen** ein. Welche Anomalien eine Nachricht schicken, wird unter **Einstellungen, Prüfung** festgelegt, siehe [Anomalien](anomalies.md). Eine zurückgestellte Sicherung schickt eine Nachricht, wenn sie zu warten beginnt, und eine, wenn die Wartezeit endet, nicht eine pro Versuch.

## Kanäle {#channels}

**Kanal hinzufügen** unten in der Karte **Kanäle** bietet **Unraid-Benachrichtigungen**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **E-Mail (SMTP)**, **Apprise**, **Webhook** und **Healthchecks** an. Leg so viele an, wie du magst, auch zwei derselben Art, etwa ein ntfy-Topic für Fehler und ein zweites für alles. Ein neuer Kanal startet ausgeschaltet, damit er schon gespeichert ist, bevor seine Felder ausgefüllt sind.

Jeder Kanal hat:

- einen eigenen Schalter, der ihn abschaltet, ohne seine Felder zu verlieren.
- **Benachrichtigen**: **Nie**, **Nur bei Fehler** oder **Bei Erfolg und Fehler**. Ein neuer Kanal startet mit **Nur bei Fehler**. Eine Sicherung, die geklappt, aber etwas ausgelassen hat, erreicht auch einen Kanal, der nur Fehler meldet.
- **Name**, den du frei lassen kannst. Er steht statt der Art da, praktisch bei zwei Kanälen derselben Art.
- **Test senden**, das eine Testnachricht über genau diesen Kanal schickt, mit den Werten, wie sie gerade im Formular stehen, gespeichert oder nicht. Die Zeile unter dem Kanal sagt, ob der Dienst sie angenommen hat, und wenn nicht, warum.
- **Kanal entfernen**, den Mülleimer daneben.

Jeder Kanal sendet für sich mit eigenem Zeitlimit, sodass ein Host, der nie antwortet, die anderen nicht aufhält. Scheitert ein Dienst so, dass ein neuer Versuch helfen kann, versucht es der Kanal bis zu dreimal. Eine Warnung zu Überfälligem, zum Budget oder zu einer Anomalie, die kein Kanal angenommen hat, wird später erneut versucht, und ein Wochenbericht ebenso.

| Kanal | Felder | Hinweise |
|---|---|---|
| **Unraid-Benachrichtigungen** | keine | Sendet an das eigene Benachrichtigungssystem von Unraid, das an Pushover, E-Mail oder Discord weiterleiten kann. Das läuft über die SSH-Verbindung unter **Einstellungen, Anbindungen, Host-SSH**, dieser Schlüssel muss also freigegeben sein. Auf TrueNAS und anderen Hosts bleibt der Kanal still und sagt, warum. |
| **ntfy** | **Topic-URL**, **Zugangstoken (optional)** | Die Adresse endet mit deinem Topic. Einen Token brauchst du nur für ein geschütztes Topic. |
| **Gotify** | **Server-URL**, **App-Token**, **Priorität bei Fehlschlag**, **Priorität bei Erfolg** | Die Prioritäten reichen von 0 bis 10 und beginnen bei 8 für Fehler und 5 für Erfolge. |
| **Discord** | **Webhook-URL** | Die Adresse eines Webhooks aus den Einstellungen des Discord-Kanals. |
| **Slack** | **Webhook-URL** | Die Adresse eines eingehenden Webhooks deiner Slack-App. |
| **Matrix** | **Homeserver-URL**, **Access-Token**, **Raum-ID** | Das Konto hinter dem Token muss im Raum sein. |
| **E-Mail (SMTP)** | **SMTP-Host**, **Port**, **Verschlüsselung**, **Benutzername**, **Passwort**, **Absenderadresse**, **Empfängeradresse** | **Verschlüsselung** ist **STARTTLS**, **TLS (implicit)** oder **None**. |
| **Apprise** | **Apprise-API-URL**, **Tags (optional)**, **Apprise-Adressen (optional)** | Richte es auf den `/notify/<key>`-Endpunkt deines Apprise-API-Containers, um seine über 100 Dienste zu erreichen, und Tags wählen eine Teilmenge der Ziele dieses Keys. Oder trag Apprise-Adressen ein, eine pro Zeile, etwa `tgram://` für Telegram oder `pover://` für Pushover, dann endet die API-URL auf `/notify`, ohne Key. |
| **Webhook** | **Webhook-URL**, **Methode**, **Header**, **Inhalt** | Eine Anfrage, die du selbst baust, siehe [Webhook](#webhook). |
| **Healthchecks** | **Ping-URL** | Siehe [Healthchecks](#healthchecks). |

Geheimnisse wie die Tokens, das SMTP-Passwort und die Werte von Webhook-Headern werden verschlüsselt gespeichert, und der Server gibt sie nie an die Seite zurück. Unter einem gespeicherten Geheimnis steht im Feld **Gespeichert. Lass das Feld leer, um es zu behalten.** Wenn du änderst, wohin ein Kanal sendet, schickt BombVault das alte Geheimnis nicht an den neuen Ort: Das Feld sagt **Die neue Adresse wird gespeichert, sobald du das hier erneut eingibst.**, und die neue Adresse wird gespeichert, sobald du es eingibst.

Einstellungen aus einer Version vor der Kanalliste werden beim ersten Start übernommen, ohne dass etwas verloren geht. Jeder Kanal, den du ausgefüllt hattest, wird ein eigener Eintrag mit deiner bisherigen Wahl bei **Benachrichtigen**. Ein Webhook im Format Discord, Slack, Gotify oder ntfy wird ein Kanal dieser Art, und ein Webhook im Format Generic JSON schickt über seinen **Inhalt** weiter `title`, `message` und `ok`. Aus dem Unraid-Schalter wird ein Unraid-Kanal, aus der Healthchecks-URL samt Ausnahmen ein Healthchecks-Kanal.

### Webhook {#webhook}

Ein **Webhook**-Kanal schickt eine HTTP-Anfrage an eine beliebige Adresse. **Methode** ist GET, POST oder PUT. **Header hinzufügen** legt eine Zeile unter **Header** an, und die Werte der Header werden wie Geheimnisse behandelt, ein `Authorization`-Header ist dort also sicher. **Inhalt** geht genau so raus, wie du ihn schreibst, und diese Platzhalter füllen sich selbst:

| Platzhalter | Wird zu |
|---|---|
| `%%title%%` | dem Titel der Nachricht |
| `%%body%%` | der Nachricht selbst |
| `%%ok%%` | `true`, oder `false` bei einem Fehler |
| `%%event%%` | dem, was passiert ist, etwa `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` oder `critical` |
| `%%importance%%` | der Stufe, die Unraid dafür nimmt: `normal`, `warning` oder `alert` |
| `%%link%%` | einem Link zur Nachricht, leer, wenn es keinen gibt |
| `%%key%%` | dem Bereich, etwa `container` oder `VM` |
| `%%time%%` | der Zeit in UTC, etwa `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

Platzhalter gehen auch in der Adresse und in Header-Werten, und jeder Wert wird passend zu seiner Stelle maskiert: als JSON, wenn der Header `Content-Type` JSON nennt, und als Parameter in der Adresse. Ein neuer Webhook startet mit diesem Header und schickt `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. Einen Namen, den BombVault nicht kennt, lässt es stehen, wie er ist, damit ein Tippfehler in der Nachricht auffällt. Lässt du **Inhalt** leer, geht kein Inhalt mit.

## Healthchecks {#healthchecks}

Ein Dienst wie [Healthchecks.io](https://healthchecks.io) schlägt Alarm, wenn sich BombVault nicht mehr meldet. Leg einen Kanal **Healthchecks** an und füge die **Ping-URL** des Checks ein, dann pingt BombVault sie über den ganzen Ablauf jeder Sicherung: beim Start, dann bei Erfolg oder mit `/fail`. Dieser Kanal hat kein **Benachrichtigen**, denn ein Check braucht seine Erfolgspings, um grün zu bleiben. Ein Bereich, der nirgendwohin sichern kann, zählt als fehlgeschlagen, sein Check wird also rot, statt einen Erfolg zu hören.

Bereiche mit derselben URL teilen sich einen Check. Ist eine Sicherung des einen fehlgeschlagen, pingen auch die guten Sicherungen der anderen `/fail` und nennen ihn, sodass der Check rot bleibt, bis dieser Eintrag wieder eine gute Sicherung hat oder du den Fehler unter **Braucht dich** quittierst.

Ein Bereich kann eine eigene Adresse pingen: **Ausnahme** im Healthchecks-Kanal fügt eine hinzu, und die Zeile sagt dann **Alle anderen Bereiche pingen diese URL.** **Test senden** schreibt einen Eintrag ins Log jedes Checks und lässt seinen Zustand, wie er ist.

## Beispiel: Gotify auf dem Handy {#example-gotify}

Du betreibst Gotify unter `https://gotify.example.com` und willst, dass Fehler auf dem Handy klingeln, während Erfolge leise ankommen.

1. Öffne in Gotify **Apps**, lege eine Anwendung namens `BombVault` an und kopiere ihr Token.
2. Öffne in BombVault **Einstellungen, Benachrichtigungen** und schalte **Geplante Läufe zusammenfassen** ein, damit eine Nacht mit 45 Sicherungen eine Nachricht schickt statt 45.
3. Drücke in **Kanäle** auf **Kanal hinzufügen** und wähle **Gotify**.
4. Gib `https://gotify.example.com` als **Server-URL** ein. BombVault hängt `/message` selbst an.
5. Füge das Token bei **App-Token** ein.
6. Stelle **Benachrichtigen** auf **Bei Erfolg und Fehler**. Lass **Priorität bei Fehlschlag** auf 8 und **Priorität bei Erfolg** auf 5. Die Android-App von Gotify behandelt 8 und höher als hoch, daher machen nur Fehler ein Geräusch.
7. Schalte den Kanal ein und drücke **Test senden**. Die Zeile unter dem Kanal sagt, ob Gotify es angenommen hat, und ein abgelehntes Token wird als solches benannt.

Der nächste geplante Lauf schickt eine Zusammenfassung mit Priorität 5, oder mit 8, wenn darin etwas fehlgeschlagen ist.

## Beispiel: ntfy ohne Konto {#example-ntfy}

ntfy braucht für ein öffentliches Topic kein Konto. Wähle einen Topic-Namen, der schwer zu erraten ist, denn jeder, der ihn kennt, kann mitlesen.

1. Drücke in **Kanäle** auf **Kanal hinzufügen** und wähle **ntfy**.
2. Gib die Adresse des Topics als **Topic-URL** ein, etwa `https://ntfy.sh/bombvault-tower-7f3k`, oder ein Topic auf deinem eigenen ntfy-Server. Lass **Zugangstoken (optional)** für ein öffentliches Topic leer.
3. Abonniere dasselbe Topic in der ntfy-App, schalte den Kanal ein und drücke **Test senden**.

BombVault sendet die Nachricht als Body und ihren Titel im Header `Title`. Ein Fehler geht mit Priorität hoch und dem Tag `warning` hinaus, damit er in der App auffällt, und eine eingefroren gebliebene VM als dringend.

## Wenn Nachrichten nicht ankommen {#troubleshooting}

Siehe [Benachrichtigungen kommen nicht an](troubleshooting.md#notifications-do-not-arrive) unter Fehlersuche.
