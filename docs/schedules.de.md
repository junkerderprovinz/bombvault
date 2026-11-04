# Zeitpläne

**Einstellungen, Zeitplan** enthält jeden Plan: das Gesamt-Backup, das alles abdeckt, einen eigenen Plan für einen Bereich, die Off-site-Kopien, welche Einträge mitmachen und in welcher Reihenfolge. Ein einzelner Eintrag kann auf seiner Seite einen eigenen Plan bekommen. Diese Seite erklärt, wie sie zusammenspielen, und geht eine typische Einrichtung durch.

## Pläne, die sich wie Sätze lesen {#plans}

Jeder Plan ist ein Knopf, der sich wie ein Satz liest, etwa *täglich um 03:00* oder *alle 2 Tage um 01:30*. Ein Klick öffnet ihn mit fünf Modi:

| Modus | Was du einstellst |
|---|---|
| **Aus** | Auf diesem Plan läuft nichts. |
| **Täglich** | **Zeit** |
| **Wöchentlich** | **Tage** und **Zeit** |
| **Alle N Tage** | **Alle** so viele **Tage**, und die **Zeit**. Es zählt ab dem letzten geplanten Lauf, eine von Hand gestartete Sicherung eines Eintrags schiebt den Plan also nicht zurück. |
| **Cron** | Ein **Ausdruck** aus fünf Feldern, etwa `0 */6 * * *` für alle 6 Stunden. **Beispiele** bietet einige an, und das Fenster zeigt die nächsten Läufe. |

Alle Zeiten gelten in der Zeitzone des Servers. Auf Unraid ist das die des Systems. Anderswo setze `TZ`, sonst läuft jeder Plan in UTC, siehe [Umgebungsvariablen](configuration.md#environment-variables).

## Woche {#week}

Die Karte **Woche** zeichnet jeden geplanten Lauf von Montag bis Sonntag, eine Zeile pro Tag: Sicherungen, Off-site-Kopien und Prüfungen. Zeig auf einen Strich, um zu lesen, was dort startet. Wo drei Pläne innerhalb von zwei Stunden starten, ist der Abschnitt markiert, damit du sie entzerren kannst.

![Einstellungen, Zeitplan: das Band Woche oben, dann das Gesamt-Backup und die Pläne pro Bereich.](assets/screenshots/schedule/week-dark.png#only-dark)
![Einstellungen, Zeitplan: das Band Woche oben, dann das Gesamt-Backup und die Pläne pro Bereich.](assets/screenshots/schedule/week-light.png#only-light)

## Gesamt-Backup {#full-backup}

Das **Gesamt-Backup** ist der Standardplan für alles. **Wann gesichert wird** legt ihn fest: ein Durchgang, der nacheinander jeden Bereich sichert, Container, VMs, Flash, Ordner, ZFS-Datasets und zuletzt das Selbstbackup. Die Zeile darunter sagt, welche Bereiche er umfasst. **Aus** schaltet ihn ab, und dann läuft nur, was einen eigenen Plan hat.

- **Verpasste Sicherungen nach dem Start nachholen**: Wenn der Server zu einem geplanten Zeitpunkt aus war, führt BombVault diese Sicherung etwa zwei Minuten nach dem Start aus.
- **Befehle vorher und nachher**: **Befehl vor dem Gesamt-Backup** läuft mit `sh -c` auf dem Server, bevor der Durchgang beginnt, zum Beispiel ein Skript, das einen Dienst anhält. Schlägt er fehl, läuft das Gesamt-Backup trotzdem. **Befehl nach dem Gesamt-Backup** läuft einmal, nachdem jeder Bereich versucht wurde, auch wenn einer fehlgeschlagen ist, was zu einem Ping passt, der bestätigt, dass der ganze Server geschützt ist. Befehle für einen einzelnen Container stehen auf seiner Seite.

**Jetzt alles sichern** auf der Seite Sicherungen startet denselben Durchgang von Hand, über alle eingeschalteten Bereiche.

## Pläne pro Bereich {#plans-per-section}

Ein Bereich kann das Gesamt-Backup verlassen und zu einer eigenen Zeit laufen. In **Pläne pro Bereich** fügt **Ausnahme** einen Bereich mit eigenem Plan hinzu, und **Ausnahme entfernen** schickt ihn zurück. Die Zeile in der Karte sagt, was die anderen tun, etwa **Alle anderen Bereiche laufen im Gesamt-Backup mit.**

Ein Bereich mit eigenem Plan wird im Gesamt-Backup nicht noch einmal gesichert, es wird also nichts doppelt gesichert. Dasselbe gilt für einen einzelnen Eintrag mit eigenem Plan.

**Off-site-Kopien pro Bereich** funktioniert genauso für Kopien. Standardmäßig kopiert jeder Bereich direkt nach jeder Sicherung an seine Off-site-Orte. **Ausnahme** gibt einem Bereich einen eigenen Kopierplan, zum Beispiel damit die VMs nur in den Sonntagnächten zu Backblaze B2 kopieren. Auch die Karte **Kopieren** eines Speicherorts kann **Wann kopieren** festlegen, siehe [Off-site-Kopien](offsite-recovery.md#off-site-copies).

## Eingeplante Einträge {#scheduled-entries}

**Einträge im Zeitplan** sagt pro Bereich, wie viele Einträge mitmachen, etwa **38 von 41 im Zeitplan**. **Einträge** öffnet die Liste mit einem Schalter **Im Zeitplan** pro Eintrag. Ein Eintrag, der ausgeschaltet ist, wird nur gesichert, wenn du ihn von Hand startest. Er zählt als nicht geschützt, solange du ihn nicht mit **Nicht sichern** markierst.

## Reihenfolge {#order}

**Reihenfolge** legt fest, in welcher Reihenfolge geplante Läufe und das Gesamt-Backup Container und VMs sichern. Zieh einen Eintrag an seinen Platz oder nutze die Pfeile, und jede Verschiebung wird sofort gespeichert. Einträge, die du ohne Reihenfolge lässt, werden nach dem am längsten Überfälligen zuerst gesichert. Eine einzelne Sicherung von Hand bleibt unberührt.

**Neustart nach der Sicherung** in derselben Karte betrifft die Container, die **Andere Container stoppen** während einer Sicherung heruntergefahren hat. Sie starten in ihrer Abhängigkeitsreihenfolge wieder, und mit **Vor dem Start des nächsten warten, bis Abhängigkeiten gesund sind** muss jeder gesund oder laufend melden, bevor der nächste startet. **Gesundheits-Timeout pro Container (Sekunden)** begrenzt die Wartezeit, standardmäßig 120, in einem Bereich von 5 bis 3600.

## Snapshot statt Stopp {#snapshot-instead-of-stop}

**Container auf ZFS nur für einen Snapshot stoppen** ist standardmäßig an. Ein Container, dessen Daten auf ZFS liegen, wird nur angehalten, während BombVault einen Snapshot dieser Daten macht, meist ein paar Sekunden, und die Sicherung wird dann aus dem Snapshot gelesen. Es braucht die SSH-Verbindung zum Host, die auch ZFS-Datasets nutzen. Ein Container, dessen Daten teilweise woanders liegen, wird wie bisher für die ganze Sicherung angehalten, und sein Lauf sagt, warum, während ein Container ohne Daten auf ZFS ohne Hinweis angehalten wird. Es funktioniert auf einem Unraid-User-Share, der auf einem einzigen ZFS-Pool liegt. Auf TrueNAS braucht die App außerdem die Capability `SYS_CHROOT`, siehe [TrueNAS SCALE](getting-started.md#truenas). Ein einzelner Container lässt sich auf seiner eigenen Seite anders einstellen.

Die Sicherung landet unter denselben Pfaden und Tags wie eine, die bei angehaltenem Container gemacht wurde, sodass Wiederherstellungen und Prüfungen beide gleich behandeln. Der Befehl vor der Sicherung und der Datenbank-Dump laufen weiterhin zuerst, die anderen zu stoppenden Container gehen mit herunter und kommen mit zurück, und der Befehl nach der Sicherung läuft, sobald der Container wieder oben ist. Ein Update nach der Sicherung wartet weiterhin, bis die Sicherung fertig ist. Seine Läufe zeigen **Snapshot** und wie viele Sekunden der Container unten war. Sollte BombVault mitten in einer Container-Sicherung stoppen, startet es die Container, die es heruntergehalten hat, beim nächsten Start wieder.

## Updates nach der Sicherung {#updates}

- **Container nach erfolgreicher Sicherung aktualisieren** zieht ein neueres Image erst, nachdem die Sicherung davor fertig ist, sodass zuerst immer ein frischer Wiederherstellungspunkt existiert. Es ist standardmäßig aus, und ein einzelner Container lässt sich auf seiner Seite unter **Nach erfolgreicher Sicherung aktualisieren** anders einstellen.
- **Altes Image nach Update entfernen** löscht das Image, das der Container verlassen hat. Ein Basis-Image, das andere Container teilen, wird nie gelöscht.
- **Unraids Update-Status nach dem Aktualisieren eines Containers auffrischen** räumt Unraids Update-Hinweis für diesen Container weg.

Eine Nachricht pro aktualisiertem Container legst du unter **Einstellungen, Benachrichtigungen** mit **Bei Container-Update benachrichtigen** fest.

## Ruhe vor der Sicherung {#idle}

Die Karte **Ruhe vor der Sicherung** legt fest, wann eine App als untätig gilt, bei einem Container, dessen geplante Sicherung darauf wartet: **CPU unter (% eines Kerns)**, **Verkehr unter (Mbit/s)** und **Mindestens (Min.)**. Ein Mediaserver gilt als untätig, wenn er nicht streamt, und bei einem Container im Host-Netzwerk zählt nur die CPU. Das Warten selbst wird an jedem Container eingestellt, siehe [Wenn die App beschäftigt ist](backups.md#app-busy).

## Ein eigener Plan für einen Eintrag {#own-plan}

Jeder Eintrag hat **Eigener Plan** in der Karte **Zeitplan und Speicherort** seines Tabs Einstellungen. Mit einem Plan dort läuft der Eintrag zu seinen eigenen Zeiten und verlässt das Gesamt-Backup und den Plan seines Bereichs. Ohne einen folgt er ihnen.

## Beispiel: drei Pläne, die zusammenpassen {#example}

Eine typische Unraid-Box: Die meisten Container lassen sich nachts sichern, die VMs sind groß und ändern sich langsam, und Immich bekommt den ganzen Tag neue Fotos.

1. **Gesamt-Backup um 03:00.** Öffne **Einstellungen, Zeitplan**. Stelle im **Gesamt-Backup** **Wann gesichert wird** auf **Täglich** um `03:00`. Schalte **Verpasste Sicherungen nach dem Start nachholen** ein, damit eine Nacht mit ausgeschaltetem Server ausgeglichen wird.
2. **VMs einmal die Woche.** Drücke in **Pläne pro Bereich** **Ausnahme**, wähle **VMs** und stelle ihren Plan auf **Wöchentlich**, **Tage** Sonntag, **Zeit** `04:00`. Die Karte sagt jetzt, dass alle anderen Bereiche im Gesamt-Backup laufen.
3. **VMs kopieren auch sonntags nach außer Haus.** Drücke in **Off-site-Kopien pro Bereich** **Ausnahme**, wähle **VMs** und stelle **Wöchentlich**, Sonntag um `06:00` ein, nachdem ihre Sicherung Zeit hatte, fertig zu werden.
4. **Immich alle 6 Stunden.** Öffne **Sicherungen**, dann `immich`, dann seinen Tab **Einstellungen**. Stelle **Eigener Plan** auf **Cron** mit `0 */6 * * *`.

Was jetzt läuft:

| Wann | Was |
|---|---|
| Jeden Tag um 03:00 | Container außer `immich`, Flash, Ordner, ZFS-Datasets und das Selbstbackup, ein Bereich nach dem anderen |
| Sonntag um 04:00 | VMs |
| Sonntag um 06:00 | Die Off-site-Kopie der VMs |
| 00:00, 06:00, 12:00 und 18:00 | `immich` für sich |

Die Karte **Woche** zeigt die Zeile von 03:00 an jedem Tag, die Striche von 04:00 und 06:00 am Sonntag und vier Striche am Tag für `immich`. `immich` wird um 03:00 nicht ein zweites Mal gesichert, weil ein Eintrag mit eigenem Plan das Gesamt-Backup verlässt. Kopien der anderen Bereiche laufen weiterhin direkt nach jeder Sicherung.

Wenn du den eigenen Plan von `immich` später entfernst, läuft es mit seinem Bereich wieder im Lauf von 03:00 mit.
