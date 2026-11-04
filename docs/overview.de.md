# Übersicht

Die Übersicht ist die erste Seite der Seitenleiste und die, die du jeden Morgen ansiehst. Sie beantwortet auf einen Blick zwei Fragen: Ist alles gesichert, und braucht etwas dich? Sie beginnt mit einem Statuskopf und vier Karten, und weitere Karten warten unter **Anpassen**.

![Die Übersicht nach ein paar Tagen: der Statuskopf, Braucht dich mit einer Warnung, der Schutz jedes Bereichs, die Speicherorte und der Verlauf.](assets/screenshots/overview/home-dark.png#only-dark)
![Die Übersicht nach ein paar Tagen: der Statuskopf, Braucht dich mit einer Warnung, der Schutz jedes Bereichs, die Speicherorte und der Verlauf.](assets/screenshots/overview/home-light.png#only-light)

## Der Statuskopf {#status-head}

Der Kopf sitzt oben auf der Seite und bleibt immer dort. Sein Ring zeigt, wie viel geschützt ist, etwa **41 von 43 geschützt**, und die Zeile daneben sagt, wie es steht:

| Der Kopf sagt | Was es bedeutet |
|---|---|
| **Alles gesichert** | Jeder Eintrag jedes eingeschalteten Bereichs hat eine aktuelle Sicherung. |
| **Gesichert, mit Hinweisen** | Alles ist gesichert, und manche Läufe haben einen Hinweis hinterlassen, etwa Dateien, die sie nicht lesen konnten. |
| **Noch nicht alles geschützt** | Einige Einträge haben keine aktuelle Sicherung, zum Beispiel weil sie auf ihre erste warten. |
| **3 Sachen brauchen dich** | Etwas ist fehlgeschlagen oder wartet auf eine Entscheidung. Die Karte **Braucht dich** listet es auf. |
| **Sichert gerade** | Eine Sicherung läuft. Der Ring wird zum Fortschrittsring mit den Stationen **Anhalten**, **Lesen** und **Starten**, und ein Gesamt-Backup zählt seine Einträge mit, etwa **Sichert gerade: 12 von 43**. **Lauf ansehen** öffnet den Lauf. |
| **Off-site-Kopie läuft** | Eine Kopie auf einen Speicherort läuft. Sie zeigt keine Prozentzahl, weil `restic copy` keine meldet. |

Unter der Zeile stehen der Zeitpunkt des letzten Laufs und der Start des nächsten geplanten.

## Braucht dich {#needs-you}

Diese Karte listet auf, was du dir ansehen solltest, neueste zuerst, und sagt **Nichts zu tun.**, wenn die Liste leer ist. Jede Zeile hat ihren eigenen Ausweg:

- **Eine fehlgeschlagene Sicherung** nennt den Eintrag und den Grund. **Fehler ansehen** zeigt die eigene Ausgabe von restic, und **Erneut versuchen** startet die Sicherung noch einmal.
- **Off-site-Kopie fehlgeschlagen** nennt den Bereich und seit wann. **Jetzt replizieren** versucht es erneut.
- **Anomalie bei plex** führt mit **Prüfen** zum Fund. Solange ein kritischer Fund offen ist, behält die Aufbewahrung die alten Sicherungen dieses Eintrags.
- **Neue** Einträge, die BombVault gefunden hat und die noch nichts sichert, etwa ein Container, den du gerade installiert hast, bieten **Ab jetzt sichern** oder **Nicht sichern** an.
- **Eine Sicherung, die aufgegeben hat**, weil ihre App die ganze Wartezeit über beschäftigt blieb, siehe [Wenn die App beschäftigt ist](backups.md#app-busy).
- **Eine fehlgeschlagene ZFS-Replik**, in einfachen Worten, etwa dass der Server keinen zfs-Befehl hat, mit der Ausgabe des Servers unter **Ansehen**.
- **Sicherungen stehen still: Der Schlüssel lässt sich nicht mehr öffnen** erscheint, wenn dieser Server den Schlüssel eines anderen Servers übernommen hat und sich `APP_KEY` danach geändert hat. Siehe [Fehlersuche](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- **Sichere dein Recovery-Kit** bleibt, bis du das Kit heruntergeladen und **Sicher aufbewahrt** gedrückt hast.

## Schutzstatus {#protection-status}

Jeder eingeschaltete Bereich hat eine Zeile mit vier Prüfungen:

| Prüfung | Sie wird grün, wenn | Andere Worte, die du sehen kannst |
|---|---|---|
| **Lokal** | die letzte Sicherung aktuell ist | noch keine Sicherung, kein Zeitplan, letzter Lauf fehlgeschlagen, Sicherung überfällig, nicht jeder Eintrag ist geschützt, Sicherung bald fällig |
| **Off-site** | die letzte Kopie außer Haus aktuell ist | keine Off-site-Kopie, Kopie läuft, Kopie überfällig, Kopie pausiert, noch keine Kopie, Replik fehlgeschlagen |
| **Löschschutz** | ein Manipulationstest bewiesen hat, dass die Gegenseite Löschen verweigert | keiner, keiner und Risiko akzeptiert, noch nicht bewiesen, Test fehlgeschlagen, nicht bewiesen, weil die Prüfung aus ist |
| **Probe-Wiederherstellung** | eine Wiederherstellung aus der Off-site-Kopie bestanden hat | nie gelaufen, läuft, fehlgeschlagen, überfällig, nur manuell, für diesen Bereich nicht möglich |

Ein Bereich ist nur auf Grundlage geprüfter Tatsachen grün. Eine Kopie auf diesem Server zählt nicht als Off-site, und ein Speicherort, den du mit **Ohne Löschschutz akzeptieren** markiert hast, zeigt einen grauen Ring statt eines gelben Punkts, sodass die Farbe des Bereichs seinen anderen Prüfungen folgt.

Ein Klick auf einen Bereich öffnet sein Fenster mit jeder Prüfung im Klartext, den Einträgen, die er zählt, und **Probe-Wiederherstellung starten** oder **Probe-Wiederherstellung erneut starten**.

![Das Schutzfenster des Bereichs Container mit seinen vier Prüfungen im Klartext.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![Das Schutzfenster des Bereichs Container mit seinen vier Prüfungen im Klartext.](assets/screenshots/overview/protection-window-light.png#only-light)

## Speicherorte {#storage-locations}

Jeder Speicherort steht hier als Gefäß mit seinem Füllstand, seiner Wachstumsgeschwindigkeit, etwa **Wächst 4,2 GB/Woche**, und dem Zeitpunkt, zu dem er bei dieser Rate voll wäre, etwa **voll in ca. 31 Wochen**. S3, B2 und rest-server melden nicht, wie viel Platz noch frei ist, daher zeigen sie **Freier Platz unbekannt** und nur ihr Wachstum. Ein Klick führt zur Seite des Speicherorts. Siehe [Speicherorte](offsite-recovery.md#destinations).

## Verlauf {#history}

Der **Verlauf** hat zwei Tabs. **Läufe** listet jeden Lauf mit Ergebnis, Dauer und Anlass, und ein Klick öffnet ihn. **Protokoll** zeigt jede Zeile mit ihrer Zeit, wie BombVault sie schreibt, und lässt sich auf einen Tag eingrenzen. Eine Zeile wird orange, wenn eine Sicherung durchgelaufen ist, aber etwas ausgelassen hat, und die Zeile nennt, was fehlt.

## Weitere Karten unter Anpassen {#customize}

Mit **Anpassen** oben auf der Seite ordnest du die Karten an. Zieh eine Karte an ihrer Leiste, um sie zu verschieben, ändere ihre Größe am Punkt in ihrer unteren Ecke, oder blende sie aus. Ausgeblendete Karten warten unter **Ausgeblendete Karten**, und **Einblenden** holt eine zurück. Solange du sie anordnest, wackeln die Karten ein wenig, und **Fertig** beendet das. Das Layout wird in diesem Browser gespeichert.

Diese Karten sind anfangs ausgeblendet:

- **Anomalien**: die offenen Funde, siehe [Anomalien](anomalies.md).
- **Nicht gesichert**: alles auf diesem Server, was keine geplante Sicherung abdeckt, jeweils mit Grund, etwa **nie hinzugefügt**, **nicht im Zeitplan enthalten** oder **alle Speicherorte seines Bereichs sind ausgeschaltet**. Sie liest die Container direkt aus Docker und findet daher auch einen, den BombVault noch nie gesehen hat.
- **Kalender**: die letzten 90 Tage, ein Quadrat pro Tag. Ein Klick auf einen Tag zeigt seine Zeilen im Protokoll unter **Verlauf**.
- **Speicher**: wie viel jeder Bereich belegt und wie gut seine Daten sich deduplizieren.
- **Übersicht**: die älteste Sicherung über alle Bereiche, wann die nächste Sicherung läuft und wie die letzte verlief.
- **Statistik**: wie viele Container und VMs es gibt, wie viele Fehler, und wie viele Pläne aktiv oder pausiert sind.
- **Host-Integration**: was BombVault auf dem Host erreicht und braucht, etwa den Docker-Socket, libvirt, restic und rclone. Fang hier an, wenn etwas nicht richtig verkabelt scheint.

![Anpassen-Modus: Die Karten lassen sich ziehen, in der Größe ändern und ausblenden, und die ausgeblendeten warten unten.](assets/screenshots/overview/customize-dark.png#only-dark)
![Anpassen-Modus: Die Karten lassen sich ziehen, in der Größe ändern und ausblenden, und die ausgeblendeten warten unten.](assets/screenshots/overview/customize-light.png#only-light)

## Wie BombVault zählt {#counting}

Jede Karte und der Kopf zählen auf dieselbe Weise: die Einträge der eingeschalteten Bereiche, die noch installiert sind und die du nicht mit **Nicht sichern** markiert hast. Ein Eintrag zählt nur als geschützt, solange ihn ein Zeitplan erreicht und er eine Sicherung hat. Eine alte Sicherung allein zählt nicht, daher erscheint ein Container, den du aus dem Zeitplan genommen hast, als nicht geschützt, bis du ihn zurücknimmst oder mit **Nicht sichern** markierst.

## Erste Schritte {#first-steps}

Bei einer Neuinstallation beginnt die Übersicht mit der Karte **Erste Schritte**: **Bereiche wählen**, **Speicherort hinzufügen** und **Erste Sicherung**. Sie bleibt, bis die erste Sicherung durchgelaufen ist. [Erste Schritte](getting-started.md#first-run) führt dich hindurch.
