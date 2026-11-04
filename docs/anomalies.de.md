# Anomalien

Ransomware und eine kaputte App sehen in einer Sicherung ähnlich aus: Plötzlich ist der Großteil der Daten neu, oder er ist weg. BombVault vergleicht jede Sicherung mit dem eigenen Verlauf des Eintrags und meldet einen Fund, wenn etwas nicht stimmt. Solange ein Fund über verlorene Daten offen ist, behält es außerdem die alten Sicherungen des Eintrags, damit die letzte gute nicht herausaltert. Der Eintrag **Anomalien** in der Seitenleiste erscheint, solange die Erkennung an ist.

![Die Seite Anomalien mit einem offenen kritischen Fund.](assets/screenshots/anomalies/page-dark.png#only-dark)
![Die Seite Anomalien mit einem offenen kritischen Fund.](assets/screenshots/anomalies/page-light.png#only-light)

## Die Seite {#page}

Kacheln für **Kritisch**, **Warnungen** und **Hinweise** wählen einen Schweregrad. **Offen** und **Geschlossen** über der Liste schalten zwischen dem um, was offen ist und was in den letzten 30 Tagen geschlossen wurde. Ein kritischer Fund steht auf derselben Fläche wie die anderen, sein Schweregrad zeigt sich im Abzeichen und am farbigen Rand. Ein Fund bietet zuerst die Lösung an, und seine Details zeigen den Wert als Kurve über den üblichen Bereich.

Ist nichts offen, zeigt **Lernfortschritt** pro Eintrag einen Ring aus zehn Teilen, je einen für jede Sicherung, aus der BombVault gelernt hat. **Überwachung** öffnet die Liste der Einträge mit ihren eigenen Einstellungen.

## Was geprüft wird {#checks}

Jede Sicherung eines Containers, einer VM, einem Ordner-Set, eines Datenbank-Dumps, des Flash und des Selbstbackups wird mit dem eigenen Verlauf des Eintrags verglichen. Die Prüfungen sehen sich an:

- die neuen Daten, die ein Lauf abgelegt hat, im Vergleich zu den größten üblichen Mengen der letzten Sicherungen und zur üblichen Rate pro Stunde,
- eine Sicherung, die den Großteil der Daten noch einmal abgelegt hat, dazu zählen auch umbenannte und neu geschriebene Dateien,
- die Quellgröße und die Dateizahl, die restic für jeden Eintrag und jeden Datenbank-Dump meldet,
- die eigene Sicherungsdauer von restic,
- Fehlerserien und sporadische Fehler,
- Wiederherstellungsprüfungen, die nicht mehr bestehen,
- den freien Platz lokaler, SFTP- und rclone-Repositories, hochgerechnet aus dem Wachstum des Repositorys.

Ein Eintrag lernt aus seinen ersten 10 Sicherungen. Eine fast leere Quelle, das Neuschreiben des Großteils der Daten und Fehler werden von Anfang an geprüft.

Ein Fund zur Größe oder Dateizahl einer Quelle wird auf die erste Sicherung datiert, in der sie sich zeigte. **Mit der Sicherung davor vergleichen** listet die Ordner auf, in denen Dateien verschwunden, hinzugekommen oder geändert sind, mit einem Hinweis, wenn fast alles in einem Suchindex, einem Cache oder in Vorschaubildern liegt, die die App von selbst neu aufbaut.

ZFS-Einträge werden Dataset für Dataset überwacht, siehe [ZFS-Datasets](zfs-datasets.md#anomalies). Ein Assistent kann die offenen Funde über den [MCP-Server](mcp.md#tools) lesen.

Die Prüfungen haben Grenzen. S3-, B2- und REST-Repositories haben keinen Wert für den freien Platz, auf dem Unraid-User-Share ist der freie Platz der des ganzen Arrays, und Sicherungen von vor restic 0.17 tragen keinen Größenverlauf.

## Einen Fund schließen {#closing}

- **Warnungen** schließen sich von selbst, sobald die Ursache weg ist.
- **Kritische** Funde über verlorene Daten und eine volle Platte bleiben, bis du handelst.
- **Quittieren** markiert einen Fund als gesehen und schließt ihn. Er wird erst wieder gemeldet, wenn seine Ursache einmal verschwunden war.
- **Als erwartet markieren** sagt, dass die Änderung von dir kam. Der neue Stand gilt nach 10 Sicherungen als normal, die Prüfung auf eine fast leere Quelle wird aber nie abgeschaltet.
- Eine geänderte Auswahl, etwa ein Ordner, den du an- oder abgehakt hast, startet den Verlauf des Eintrags von selbst neu.

Eine Benachrichtigung geht einmal pro Vorfall hinaus, und Fehler und Wiederherstellungsprüfungen, die schon benachrichtigen, werden nicht doppelt gemeldet.

## Alte Sicherungen bleiben solange erhalten {#retention-hold}

Solange eine Quelle fast leer ist, stark geschrumpft ist oder der Großteil ihrer Daten neu abgelegt wurde, lassen Aufbewahrung und Aufräumen die alten Sicherungen dieses Eintrags in Ruhe, und der Fund verlinkt auf die letzte gute Sicherung, die das Wiederherstellungsfenster dann vorwählt. Bevor du so einen Fund schließt, sagt dir BombVault, dass die nächste Bereinigung alte Sicherungen wieder nach den Aufbewahrungsregeln löscht. Fehlen also Daten oder wurden sie verschlüsselt, stelle zuerst wieder her und schließe den Fund danach.

## Einstellungen {#settings}

Die Karte **Anomalien** unter **Einstellungen, Prüfung** enthält die globalen Einstellungen. Jedes Bedienelement speichert sofort bei der Änderung, und die drei unter dem Schalter sind ausgeblendet, solange die Erkennung aus ist.

| Einstellung | Vorgabe | Was sie macht |
|---|---|---|
| **Anomalien erkennen** | An | Vergleicht jede Sicherung mit dem eigenen Verlauf des Eintrags. Ausgeschaltet wird nichts Neues geprüft, und der Eintrag **Anomalien** verschwindet aus der Seitenleiste. Die Karte verlinkt weiter auf frühere Funde. |
| **Empfindlichkeit** | Ausgewogen | Streng meldet kleinere Änderungen, Nachsichtig nur große. |
| **Benachrichtigen bei** | Nur kritische Funde | Der niedrigste Schweregrad, der über die unter [Benachrichtigungen](notifications.md) eingerichteten Kanäle eine Nachricht schickt. Wiederholte Fehler bei Sicherungen und Dumps sowie fehlgeschlagene geplante Wiederherstellungsprüfungen schicken schon ihre eigene Nachricht und gehen nicht doppelt hinaus. |
| **Alte Sicherungen behalten, wenn eine Quelle stark schrumpft oder neu geschrieben wird** | An | Solange ein Eintrag einen offenen Fund für eine fast leere Quelle, ein starkes Schrumpfen oder den Großteil neu abgelegter Daten hat, lassen Aufbewahrung und Aufräumen die alten Sicherungen dieses Eintrags in Ruhe. Quittiere den Fund oder markiere ihn als erwartet, um sie freizugeben. |

Jeder Eintrag kann eine eigene Empfindlichkeit und ein eigenes Benachrichtigungs-Minimum haben. Stelle sie auf dem Tab **Prüfung** des Eintrags ein oder auf der Seite **Anomalien** unter **Überwachung**. Bei einem ZFS-Eintrag gelten sie für jedes Dataset seines Baums.
