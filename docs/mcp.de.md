# MCP-Server

BombVault hat einen eingebauten Server für das Model Context Protocol (MCP), das Protokoll, mit dem KI-Assistenten wie Claude Code und Claude Desktop externe Werkzeuge erreichen. Darüber kann ein Assistent lesen, wie es um deine Sicherungen steht, und, wenn du es erlaubst, eine Sicherung starten oder eine abbrechen, die er selbst gestartet hat. Er ist aus, bis du einen Schlüssel anlegst oder die [Anmeldung über OAuth](#oauth) einschaltest: Bis dahin antwortet der Endpunkt `/mcp` auf alles mit `404`.

## Was ein Assistent kann und was nicht {#tools}

| Tool | Was es tut | Art |
|---|---|---|
| `get_health` | Version, Instanzname, ob eine Sicherung läuft, und was dieser Schlüssel darf | lesen |
| `get_status` | Schutzstatus pro Domain: letzte erfolgreiche Sicherung, erwartetes Intervall, Prüfungen und Off-site-Prüfungen, nächste geplante Läufe, Sicherungen, die auf eine ruhende App warten, und bei Containern der neueste Start-Test | lesen |
| `get_coverage` | Was BombVault schützt und was nicht, mit dem Grund für jedes | lesen |
| `list_items` | Jeder geschützte Container, jede VM, jedes Ordner-Set, der Flash und die App-Konfiguration, mit Zeitplan, was eine Sicherung davon anhält, der letzten Sicherung und wie lange sie dauerte. Datenbank-Container tragen auch ihren letzten Dump. ZFS-Datasets werden ebenfalls aufgelistet, mit dem Ergebnis ihrer letzten Prüfung. Jedes Element trägt seine letzte Wiederherstellungsprüfung, und ein Container seinen letzten Start-Test oder warum er sich nicht testen lässt. Ein Container, der seit seiner letzten Sicherung mit anderen Einstellungen neu angelegt wurde, listet auf, was sich geändert hat | lesen |
| `list_runs` | Laufverlauf, neueste zuerst, filterbar nach Domain, Element, Status, Art und Zeit. Eine langsame Sicherung, die eine einzige Sache aufgehalten hat, nennt sie | lesen |
| `list_restore_points` | Wiederherstellungspunkte eines Elements aus seinem primären Repository, und bei einem Container seine Datenbank-Dumps. Ein ZFS-Dataset bekommt pro Sicherung einen Wiederherstellungspunkt, mit einem Snapshot jedes Datasets darunter | lesen |
| `get_activity` | Was gerade läuft, mit Phase und Prozentzahl | lesen |
| `get_storage_stats` | Größenverlauf des primären Repositorys einer Domain und sein Wachstum pro Woche sowie der belegte, freie und gesamte Platz auf der Platte oder dem Remote jedes ihrer Repositories | lesen |
| `get_size_breakdown` | Welche Ordner und Dateien in der neuesten Sicherung eines Containers, einer VM oder eines Ordner-Sets den Platz belegen und wie viel davon jeweils die letzte Sicherung hinzugefügt hat | lesen |
| `list_anomalies` | Anomalien, die BombVault in den Sicherungen bemerkt hat, filterbar nach Status, Schweregrad und Domain, mit einer Zusammenfassung dessen, was offen ist | lesen |
| `get_anomaly` | Einer dieser Funde, mit der Notiz, die beim Quittieren hinterlassen wurde | lesen |
| `start_backup` | Sichert ein Element jetzt | starten |
| `start_domain_backup` | Sichert jedes geschützte Element einer Domain | starten |
| `start_backup_everything` | Führt den Durchgang Alles sichern aus | starten |
| `cancel_backup` | Bricht eine laufende Sicherung ab, die dieser Schlüssel gestartet hat | abbrechen |

Diese bleiben in der Weboberfläche: Wiederherstellungen jeder Art (auch das Herunterladen, Speichern oder Importieren eines Datenbank-Dumps), das Löschen von Sicherungen, Aufräumen, Entsperren, Prüfungen und Tests, Off-site-Replikation, Einstellungen, Zugangsdaten und MCP-Schlüssel sowie das Abbrechen einer Sicherung, die der Zeitplan, die Weboberfläche oder ein anderer Schlüssel gestartet hat. Dasselbe gilt für das Quittieren einer Anomalie oder das Markieren als erwartet, was auf der Seite **Anomalien** geschieht. Der Grund ist, dass die Ausgabe der Tools Namen und Fehlermeldungen von deinem Server enthält, und jede davon könnte Text tragen, der geschrieben wurde, um den Assistenten zu lenken. Ein Assistent, der auf solchen Text hereinfällt, kann schlimmstenfalls innerhalb der Limits unten eine Sicherung starten oder eine abbrechen, die er selbst gestartet hat.

Ist das primäre Repository eines Elements entfernt (S3, REST, SFTP, rclone), kontaktiert `list_restore_points` es, der Aufruf kann also eine Weile dauern. Off-site-Kopien lassen sich über MCP nicht auflisten. Was die Anomalieprüfungen ansehen, beschreibt [Anomalien](anomalies.md), und wie ein ZFS-Element einen Snapshot pro Dataset hält, steht unter [ZFS-Datasets](zfs-datasets.md#contents).

## Was eine gestartete Sicherung tut {#starting-backups}

Die Sicherung eines Assistenten ist dieselbe Sicherung, die die Weboberfläche startet. Ein laufender Container wird angehalten, bis seine Sicherung fertig ist, zusammen mit den Containern, die mit ihm anhalten sollen. Eine VM mit der Methode Graceful wird heruntergefahren und wieder gestartet. Ein ZFS-Dataset hält die dafür eingestellten Container an, während sein Snapshot gemacht wird. Ordner-Sets, der Flash und die Konfiguration laufen weiter. Danach wendet BombVault die Aufbewahrungsrichtlinie an und kopiert unter Umständen in das Off-site-Repository. `list_items` sagt dem Assistenten, was ein Element anhält und wie lange seine letzte Sicherung dauerte, und die Tool-Beschreibungen bitten ihn, es dir zu sagen, bevor er etwas startet.

Weil eine Sicherung Dinge anhält und alte Wiederherstellungspunkte herausrotiert, sind Starts über MCP begrenzt:

- 12 gestartete Sicherungen pro Stunde und Schlüssel.
- 15 Minuten zwischen zwei MCP-Starts desselben Elements, derselben Domain oder von Alles sichern.
- Höchstens 4 MCP-Starts desselben Elements in 24 Stunden.
- **Aufbewahrungsschutz.** Behält eine Domain eine feste Zahl von Wiederherstellungspunkten (nur „letzte N behalten“, keine tägliche, wöchentliche oder monatliche Regel, lokal oder an einem Off-site-Ziel), schiebt jede neue Sicherung die älteste hinaus. BombVault lehnt dann einen MCP-Start eines Elements ab, dessen neueste N-1 erfolgreiche Sicherungen alle über MCP gestartet wurden. So bleibt immer mindestens ein Wiederherstellungspunkt, den der Zeitplan oder du gemacht hast, in der behaltenen Menge. Bei „letzte 1 behalten“ kann ein Assistent dieses Element gar nicht sichern. Die nächste geplante Sicherung schafft wieder Platz. Eine reine Jahresregel zählt als „letzte 1 behalten“, da sie einen einzigen Wiederherstellungspunkt für das laufende Jahr hält.

Ein Start einer Domain oder von Alles sichern lässt die Elemente aus, die ein Limit zurückhält, und nennt sie in seiner Antwort. Die Weboberfläche und der Zeitplan unterliegen all dem nicht. Das Stundenbudget lebt im Arbeitsspeicher, ein Neustart von BombVault setzt es also zurück.

Starts über die [API](api.md#errors) und von [Home Assistant](api.md#home-assistant) zählen pro Element auf dieselben Limits wie Starts über MCP und auf den Aufbewahrungsschutz.

## Einschalten {#switch-on}

1. Öffne **Einstellungen, Anbindungen, MCP-Server**, klicke auf **Client wählen** und wähle deinen im Fenster. Die Suche oben rechts findet ihn nach Namen. Ein Client, der nicht aufgelistet ist, verbindet sich über **Anderer Client**.
2. Lass unter **Schlüssel** **Neuer Schlüssel** und den vorgeschlagenen Namen stehen, der der des Clients ist, oder tippe einen ein, der sagt, wo der Schlüssel genutzt wird, zum Beispiel „Claude Code auf dem Laptop“. Ein Schlüssel pro Client lässt dich einen entziehen, ohne die anderen anzufassen. **Vorhandener Schlüssel** gibt dem Client einen Schlüssel, den du früher angelegt hast.
3. Schalte **Sicherungen starten erlauben** für einen Schlüssel ein, der Sicherungen starten können soll. Ohne das kann der Schlüssel nur lesen. Du kannst es später in den **Details** des Schlüssels ändern, und die Änderung gilt für die nächste Anfrage des Assistenten ohne neue Verbindung.
4. Klicke auf **Schlüssel anlegen**. Der Schlüssel wird einmal angezeigt. BombVault behält nur einen Fingerabdruck davon und kann ihn nicht erneut zeigen, kopiere ihn also jetzt. Schließt du das Fenster, bevor der Client den Schlüssel genutzt hat, zeigt die Karte ihn weiter, bis du bestätigst, dass du ihn kopiert hast.

Ohne Login-Passwort steht die Weboberfläche selbst jedem in deinem Netzwerk offen, und jeder, der sie öffnen kann, kann auch einen Schlüssel anlegen. Die Karte sagt das. Öffnest du BombVault unter einem öffentlich aussehenden Namen (zum Beispiel `bombvault.example.com` über einen Reverse-Proxy) und ist kein Login-Passwort gesetzt, lassen sich Schlüssel von dieser Adresse aus weder anlegen noch ersetzen, damit keine Webseite im Internet deinen Browser dazu bringen kann, einen anzulegen. Setze ein Login-Passwort oder öffne BombVault über seine IP-Adresse oder einen lokalen Namen wie `tower` oder `tower.local`.

## Deine Schlüssel und ihr Protokoll {#keys}

Jeder Schlüssel hat unter **Schlüssel und angemeldete Clients** eine eigene Zeile. Sie zeigt den Namen des Schlüssels, ob er Sicherungen starten darf oder nur lesen, die letzten vier Zeichen des Schlüssels, wann er angelegt oder zuletzt ersetzt wurde und wann ein Client ihn zuletzt genutzt hat. **Details** öffnet ein Fenster, in dem du den Schlüssel umbenennst, seine Berechtigung änderst, ihn ersetzt oder entziehst und das zeigt, wie viele Aufrufe er heute gemacht hat. Ein entzogener Schlüssel wandert in die Liste der entzogenen Schlüssel, wo du ihn endgültig löschen kannst, sobald ihn kein Lauf im Verlauf mehr nennt.

Neben ihrem Namen zeigt die Zeile das Zeichen des Clients, für den der Schlüssel gemacht wurde. Ein Schlüssel, der über **Anderer Client** oder vor der Client-Auswahl der Karte angelegt wurde, zeigt stattdessen einen Schlüssel.

Das **Protokoll** in den Details eines Schlüssels zeigt, was dieser Schlüssel getan hat, und ein entzogener Schlüssel behält einen eigenen Knopf **Protokoll**. Die Sicherungen, die er gestartet hat, stehen zuerst, jede mit ihrem Zustand und einem Link auf diesen Lauf im Aktivitätsprotokoll des Dashboards. Darunter stehen seine Aufrufe, neueste zuerst, mit dem Tool und dem, was aus dem Aufruf wurde. Eine Ablehnung sagt warum: Der Schlüssel darf nur lesen, der Aufbewahrungsschutz hat die Sicherung zurückgehalten, eine andere Sicherung lief schon, dasselbe Element, dieselbe Domain oder derselbe Durchgang wurde wenige Minuten zuvor außerhalb der Weboberfläche gestartet, oder der Schlüssel hat zu viele Anfragen geschickt. Ein Abbruch verlinkt auf den Lauf, um den es ging.

BombVault hält die Einträge jedes Schlüssels bis zu 30 Tage: die neuesten 500 erfolgreichen Starts und Abbrüche und daneben die neuesten 200 anderen Aufrufe (Lesezugriffe, Ablehnungen und Fehler). Ein Assistent, der eine laufende Sicherung ständig abfragt oder einen abgelehnten Aufruf ständig wiederholt, kann den Start dieser Sicherung nicht aus dem Protokoll drängen. Pro Aufruf speichert es das Tool, das Ergebnis und den Lauf, den ein Abbruch nannte. Es speichert nie, was der Assistent gesendet hat, und nie den Schlüssel oder seinen Fingerabdruck. Das Diagnosepaket zählt die Einträge nur, und ein Einstellungsexport lässt sie weg.

## Einen Client verbinden {#clients}

**Client wählen** öffnet ein Fenster mit jedem Client als Kachel, unter **Auf diesem Rechner** oder **In der Cloud**, und einer Suche oben rechts. Eine Kachel öffnet die Einrichtung des Clients im selben Fenster, in drei Schritten: der Schlüssel, die Konfiguration für diesen Client mit der Adresse, unter der du die Karte geöffnet hast, einem Knopf zum Kopieren, wo die Konfiguration liegt und, bei BombVaults eigenem Zertifikat, was der Client braucht, um ihm zu vertrauen. Und das Warten auf den ersten Aufruf des Clients. Das Fenster beobachtet die letzte Nutzung des Schlüssels und wird grün, wenn dieser Aufruf eintrifft.

Die Einrichtung hält den Schlüssel aus jeder Befehlszeile heraus. Wo der Client ihn aus einer Umgebungsvariable (`BOMBVAULT_MCP_KEY`), einer maskierten Eingabe oder einer eigenen Datei lesen kann, nennt die Konfiguration ihn nur. Wo der Client keinen solchen Weg hat, steht der Schlüssel in seiner Konfigurationsdatei oder seinen Einstellungen, und die Einrichtung sagt das. Wo die Dokumentation eines Clients nicht sagt, wie er ein Zertifikat behandelt, das er nicht kennt, schreibt die Einrichtung diesen Schritt als das, was zu tun ist, falls der Client das Zertifikat von BombVault ablehnt.

| Client | Einrichtung | Woher der Schlüssel kommt |
|---|---|---|
| AnythingLLM | Konfigurationsdatei | die Konfigurationsdatei |
| Antigravity | Konfigurationsdatei | Umgebungsvariable |
| Claude Code | Befehl | Schlüsseldatei |
| Claude Desktop | Konfigurationsdatei | Schlüsseldatei |
| Cline | Konfigurationsdatei | die Konfigurationsdatei |
| Codex CLI | Konfigurationsdatei | Umgebungsvariable |
| Continue | Konfigurationsdatei | `~/.continue/.env` |
| Copilot CLI | Konfigurationsdatei | die Konfigurationsdatei |
| Cursor | Konfigurationsdatei | Umgebungsvariable |
| Gemini CLI | Konfigurationsdatei | Umgebungsvariable |
| GitHub Copilot (VS Code) | Konfigurationsdatei | maskierte Eingabe |
| Goose | Konfigurationsdatei | Umgebungsvariable |
| Jan | Formular in der App | die Einstellungen der App |
| JetBrains (AI Assistant, Junie) | Konfigurationsdatei | die Konfigurationsdatei |
| Kimi Code | Konfigurationsdatei | die Konfigurationsdatei |
| LM Studio | Konfigurationsdatei | die Konfigurationsdatei |
| Mistral Vibe | Konfigurationsdatei | Umgebungsvariable |
| Msty | Formular in der App | die Einstellungen der App |
| n8n | Formular in der App | der Zugangsdatenspeicher von n8n |
| Open WebUI | Formular in der App | die Einstellungen der App |
| opencode | Konfigurationsdatei | Umgebungsvariable |
| Perplexity (Mac) | Formular in der App | Schlüsseldatei |
| Qwen Code | Konfigurationsdatei | Umgebungsvariable |
| Roo Code | Konfigurationsdatei | Umgebungsvariable |
| Visual Studio | Konfigurationsdatei | die Konfigurationsdatei |
| Warp | Konfigurationsdatei | die Konfigurationsdatei |
| Windsurf | Konfigurationsdatei | Umgebungsvariable |
| Zed | Konfigurationsdatei | die Konfigurationsdatei |
| Grok | Formular, in der Cloud | die Server des Anbieters |
| Le Chat | Formular, in der Cloud | die Server des Anbieters |
| ChatGPT | Anmeldung über OAuth, in der Cloud | ein Access-Token, siehe [unten](#oauth) |
| Claude (claude.ai) | Anmeldung über OAuth, in der Cloud | ein Access-Token, siehe [unten](#oauth) |

Die folgenden Abschnitte erklären die Einrichtungen von Claude Code und Claude Desktop genauer und listen auf, was jeder andere Client braucht.

### Claude Code {#claude-code}

Claude Code erreicht BombVault über `mcp-remote`, das Node.js auf diesem Computer braucht. Speichere den Schlüssel zuerst in einer eigenen Textdatei, als einzelne Zeile:

```text
X-API-Key: <your key>
```

Führe dann den Befehl aus der Karte einmal in einem Terminal aus, mit dem eingesetzten Pfad dieser Datei. Hinter einem Zertifikat, dem dein Computer vertraut, sieht er so aus:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

Mit dem eigenen Zertifikat von BombVault (siehe [TLS und Zertifikate](#tls)) verweist der Befehl außerdem Node.js auf das heruntergeladene Zertifikat:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Prüfe die Verbindung mit `/mcp` in Claude Code. `--scope user` macht BombVault in allen deinen Projekten verfügbar. Claude Code behält nur den Pfad der Schlüsseldatei, der Schlüssel erscheint also weder im Befehl und deinem Shell-Verlauf noch in der Prozessliste. Bewahre die Datei dort auf, wo nur du sie lesen kannst, und außerhalb jedes Ordners, den du eincheckst. `@latest` lässt `npx` ein aktuelles `mcp-remote` holen. Ein älteres, global installiertes würde sonst genutzt, und es kennt `--header-file` nicht.

Schreibe nicht `${BOMBVAULT_MCP_KEY}` in die `mcp-remote`-Argumente für Claude Code. Claude Code setzt so einen Verweis aus seiner eigenen Umgebung ein, bevor es `mcp-remote` startet, sodass der Schlüssel in der Befehlszeile dieses Prozesses landet, wo andere Programme und Benutzer des Computers ihn lesen können.

Ohne Node.js, und nur hinter einem Zertifikat, dem dein Computer vertraut, kann sich Claude Code selbst verbinden. Lege eine `.mcp.json` in den Projektordner:

```json
{
  "mcpServers": {
    "bombvault": {
      "type": "http",
      "url": "https://bombvault.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${BOMBVAULT_MCP_KEY}"
      }
    }
  }
}
```

Setze `BOMBVAULT_MCP_KEY` dort, wo Claude Code startet, zum Beispiel unter `"env"` in `~/.claude/settings.json` oder in deinem Shell-Profil, in einem Texteditor bearbeitet statt am Prompt getippt. Hier ist der Verweis sicher, weil Claude Code keinen zweiten Prozess startet, der ihn tragen würde. Das eigene Zertifikat von BombVault funktioniert so nicht: Die eigene Verbindung von Claude Code lehnt es auch mit gesetztem `NODE_EXTRA_CA_CERTS` ab. Checke nie eine `.mcp.json` mit eingetragenem Schlüssel ein.

### Claude Desktop {#claude-desktop}

Claude Desktop erreicht BombVault über `mcp-remote`, das Node.js auf diesem Computer braucht. Speichere den Schlüssel zuerst in einer eigenen Textdatei, als einzelne Zeile, wie bei [Claude Code](#claude-code) beschrieben. Öffne die Konfigurationsdatei über **Settings, Developer, Edit Config** in Claude Desktop. Sie liegt unter Windows in `%APPDATA%\Claude\claude_desktop_config.json` und unter macOS in `~/Library/Application Support/Claude/claude_desktop_config.json`. Füge den Eintrag aus der Karte innerhalb von `"mcpServers"` hinzu, neben den Servern, die schon dort sind, und starte Claude Desktop neu:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote@latest", "https://192.168.1.10:3443/mcp", "--header-file", "<path of the file with your key>"],
      "env": {
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` steht nur für das eigene Zertifikat von BombVault da. Lass es hinter einem Zertifikat weg, dem dein Computer schon vertraut.
- `--allow-http` wird nur für eine einfache `http://`-Adresse hinzugefügt.
- Schreibe die Pfade unter Windows mit Schrägstrichen, zum Beispiel `C:/Users/sam/bombvault-key.txt`, weil ein einzelner Backslash kein gültiges JSON ist. Halte den Pfad der Schlüsseldatei frei von Leerzeichen: Claude Desktop unter Windows reicht einen Pfad mit Leerzeichen in zwei Stücken an `npx` weiter.
- Die Konfiguration nennt nur die Schlüsseldatei, der Schlüssel erscheint also weder darin noch in der Prozessliste. Bewahre die Datei dort auf, wo nur du sie lesen kannst.

### Clients in der Cloud {#cloud-clients}

ChatGPT, Claude auf claude.ai, Grok und Le Chat rufen BombVault von den Servern ihrer Anbieter aus auf, daher muss BombVault aus dem Internet erreichbar sein, mit einem öffentlich vertrauten Zertifikat, zum Beispiel hinter einem Reverse-Proxy. Le Chat lehnt selbstsignierte ab. Ein Login am Proxy kann die Weboberfläche schützen, aber `/mcp` muss ohne ihn zu BombVault durchgehen: Diese Dienste können sich an einem Proxy nicht anmelden, und BombVault prüft ihren Schlüssel oder ihr Token selbst. Grok und Le Chat senden einen festen Schlüssel, und ihre Knöpfe richten sie wie die anderen ein. ChatGPT und Claude auf claude.ai verbinden sich in den meisten Organisationen nur über die Anmeldung mit OAuth, die als Nächstes beschrieben wird.

### Anmeldung über OAuth {#oauth}

Für einen Client, der keinen Schlüssel nehmen kann, ist BombVault sein eigener OAuth-Autorisierungsserver. Der Client registriert sich selbst, schickt dich auf eine BombVault-Seite, und dort meldest du dich mit deinem Login-Passwort (und dem zweiten Faktor, falls eingerichtet) an und erlaubst es. Der Client bekommt dann ein Token, das nur für den MCP-Endpunkt dieses BombVault funktioniert, und erneuert es von selbst.

1. Setze unter **Einstellungen, Sicherheit** ein Login-Passwort. Ohne eines bietet BombVault gar keine Anmeldung an, weil niemand da wäre, der um Zustimmung gefragt wird.
2. Mache BombVault aus dem Internet über https mit einem Zertifikat erreichbar, dem Browser vertrauen, meist über einen Reverse-Proxy. Der Client ruft `/mcp`, `/oauth/` und `/.well-known/` von seinen eigenen Servern aus auf, ein Proxy mit eigenem Login muss diese drei Pfade also zu BombVault durchlassen. Die Zustimmungsseite unter `/oauth/authorize` öffnet sich in deinem eigenen Browser und darf hinter dem Proxy-Login bleiben. Nenne den Proxy auch in `TRUSTED_PROXY` (siehe [Konfiguration](configuration.md)). BombVault begrenzt Client-Registrierungen pro Adresse, und ohne das scheint jeder Client vom Proxy zu kommen.
3. Schalte auf der MCP-Karte **Anmeldung über OAuth** ein und gib die **Öffentliche Adresse** ein: die https-Adresse ohne Pfad, zum Beispiel `https://backup.example.com`. Jedes Token ist an diese Adresse gebunden, nach einer Änderung muss sich also jeder Client neu anmelden.
4. Wähle unter **Client wählen** ChatGPT oder Claude. Das Fenster zeigt die **Connector-URL**, die öffentliche Adresse plus `/mcp`, und wo sie in diesem Client hingehört. Schalte in ChatGPT den Entwicklermodus unter **Settings, Apps & Connectors, Advanced settings** ein, wähle **Create**, füge die Connector-URL als MCP-Server-URL ein und wähle OAuth als Authentifizierung. Öffne auf claude.ai **Settings, Connectors, Add custom connector**, füge die Connector-URL ein, lass die OAuth-Client-ID und das Secret leer und wähle **Connect**.
5. Der Client öffnet die Zustimmungsseite. Sie zeigt, wer anfragt, wohin deine Antwort dich zurückschickt, und den Schalter **Sicherungen starten erlauben**, der zunächst aus ist. Wähle **Erlauben** oder **Ablehnen**.

Jeder Client, der sich angemeldet hat, bekommt in derselben Liste eine eigene Zeile mit seinem Zeichen. Seine **Details** enthalten sein Protokoll, **Entziehen** und **Sicherungen starten erlauben**, und es gelten dieselben Limits wie bei einem Schlüssel. Das Entziehen wirkt sofort. Meldet sich derselbe Client erneut an, ersetzt seine neue Freigabe die alte, und eine Freigabe, die 30 Tage lang niemand genutzt hat, läuft ab. Bis zu 10 Clients können gleichzeitig angemeldet sein, zusätzlich zu den 10 Schlüsseln.

Die Zustimmungsseite akzeptiert eine Anfrage nur von einem registrierten Client, der genau eine seiner registrierten Rückkehradressen nennt: https oder eine Loopback-Adresse auf einem beliebigen Port für einen Client auf deinem eigenen Computer. Es wird nur der Authorization-Code-Flow mit PKCE (S256) akzeptiert, und deine Antwort ist an deine Sitzung gebunden, sodass keine andere Website sie für dich senden kann. Access-Tokens gelten eine Stunde. Ein Refresh-Token wird bei jeder Nutzung ersetzt, und eines, das danach wieder auftaucht, entzieht die Freigabe, weil jemand anderes eine Kopie hält. Ein Client, der seine letzte Erneuerung innerhalb von 30 Sekunden wiederholt, weil die Antwort ihn nie erreichte, bekommt stattdessen neue Tokens. BombVault holt keine Client-Metadaten-Dokumente aus dem Internet, Clients registrieren sich also über dynamische Client-Registrierung.

### Andere Clients {#other-clients}

Jeder Client, der Streamable HTTP spricht, funktioniert:

- URL: die Adresse der Weboberfläche plus `/mcp`, zum Beispiel `https://192.168.1.10:3443/mcp`.
- Der Schlüssel in `Authorization: Bearer <key>` oder in `X-API-Key: <key>`. Werden beide gesendet, müssen sie denselben Schlüssel tragen.
- `POST` mit `Content-Type: application/json` und `Accept: application/json, text/event-stream`.
- Eine JSON-RPC-Nachricht pro Anfrage. Batches werden abgelehnt.
- Protokollversionen 2026-07-28, 2025-11-25, 2025-06-18 und 2025-03-26.

## TLS und Zertifikate {#tls}

BombVault liefert HTTPS mit einem Zertifikat, das es selbst erstellt hat, und anfangs nennt dieses Zertifikat nur `localhost`, `127.0.0.1` und `::1`. Claude Code und `mcp-remote` lehnen es an einer LAN-Adresse ab. Die Wege darum herum, in der Reihenfolge, die zu den meisten Unraid-Installationen passt:

1. **Die Adresse in der MCP-Karte hinzufügen.** Wird die Karte über HTTPS unter einer Adresse geöffnet, die das Zertifikat nicht nennt, sagt sie das und bietet **Diese Adresse ins Zertifikat eintragen** an. BombVault stellt sein Zertifikat mit der hinzugefügten Adresse neu aus (dein Browser warnt noch einmal, wie beim ersten Mal). Klicke dann auf **Zertifikat herunterladen**. Die Schnipsel setzen `NODE_EXTRA_CA_CERTS` auf die heruntergeladene Datei, der Client vertraut also genau diesem Zertifikat. Das heißt auch, dass jeder Client, der mit einem früheren Download eingerichtet wurde, sich nicht mehr verbindet, sobald das Zertifikat neu ausgestellt ist, auf diesem Computer und auf jedem anderen, bis er die neue Datei bekommt.
2. **Ein Reverse-Proxy mit vertrauenswürdigem Zertifikat** (Nginx Proxy Manager, SWAG, Caddy, Traefik). Der Client sieht dann das Zertifikat des Proxys und braucht nichts Zusätzliches, und die Karte warnt nicht vor dem eigenen von BombVault.
3. **Tailscale.** `tailscale serve` vor dem Container oder die Unraid-Tailscale-Integration gibt dir einen `ts.net`-Namen mit vertrauenswürdigem Zertifikat.
4. **`HTTP_ONLY=true`**, nur hinter einem Proxy, der TLS beendet, oder in einem Netzwerk, dem du voll vertraust. Es stellt die ganze Weboberfläche auf einfaches HTTP um, braucht eine Änderung der Container-Einstellungen und sendet den Schlüssel unverschlüsselt.

Setze nie `NODE_TLS_REJECT_UNAUTHORIZED=0`. Es schaltet die Zertifikatsprüfung für alles aus, womit dieser Node.js-Prozess spricht.

Ein Reverse-Proxy muss den Header `Authorization` (oder `X-API-Key`) durchreichen, was Proxys tun, solange man ihnen nichts anderes sagt, und darf `/mcp` nicht puffern oder umschreiben. Ein Location-Block für Nginx oder Nginx Proxy Manager, der auch das Zertifikat von BombVault prüft:

```nginx
location /mcp {
    proxy_pass https://192.168.1.10:3443;
    proxy_ssl_verify on;
    proxy_ssl_trusted_certificate /data/bombvault-cert.pem;
    proxy_ssl_name localhost;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_set_header Host $host;
}
```

Hinter einem Proxy trägt jede Anfrage die Adresse des Proxys. Fünf falsche Schlüssel von einem falsch eingerichteten Client sperren dann jeden MCP-Client hinter diesem Proxy für eine Minute aus. Nenne den Proxy in `TRUSTED_PROXY` (siehe [Konfiguration](configuration.md)), um pro Client zu zählen.

## Sicherheitsmodell {#security}

- Ohne aktiven Schlüssel und bei ausgeschalteter Anmeldung über OAuth antwortet `/mcp` mit `404`.
- Die Anmeldung über OAuth wird nur angeboten, solange ein Login-Passwort gesetzt ist. Tokens, Codes und Client-Secrets werden nur als Fingerabdrücke gespeichert, und ein Token funktioniert nur für die Adresse, für die es ausgestellt wurde.
- Ein Client kann sich höchstens 10 Mal pro Stunde von einer Adresse registrieren, und BombVault behält höchstens 100 registrierte Clients, mit denen sich niemand angemeldet hat, je einen Tag lang. Falsche Codes und Refresh-Tokens zählen zur selben Sperre wie falsche Schlüssel.
- Freigaben verhalten sich wie Schlüssel, wenn eine Konfigurationssicherung wiederhergestellt wird oder sich `APP_KEY` ändert: Nach einer Wiederherstellung muss sich jeder Client neu anmelden.
- Keine ausgenommenen Adressen. Anfragen von `localhost`, dem Unraid-Host, einem Reverse-Proxy oder `tailscale serve` brauchen einen Schlüssel wie alle anderen, auch wenn die Weboberfläche kein Login-Passwort hat.
- Schlüssel werden nur als Fingerabdrücke gespeichert, einmal angezeigt und lassen sich umbenennen, ersetzen und entziehen. Bis zu 10 aktive Schlüssel, jeder mit seinem eigenen Schalter **Sicherungen starten erlauben**.
- Jedes Anlegen, Ersetzen, Ändern der Berechtigung und Entziehen sendet eine Benachrichtigung über deine Benachrichtigungskanäle, mit der Adresse, von der es kam, sofern Benachrichtigungen nicht ausgeschaltet sind.
- 5 falsche Schlüssel pro Minute und Adresse, dann `429`. 120 Anfragen pro Minute und 12 gestartete Sicherungen pro Stunde und Schlüssel, dazu die Wartezeit und der Aufbewahrungsschutz von oben.
- Anfragen von einer Browserseite auf einem anderen Origin werden abgelehnt.
- Schlüssel lassen sich nicht von einem öffentlich aussehenden Hostnamen aus anlegen, solange kein Login-Passwort gesetzt ist.
- Jede Sicherung, die ein Assistent startet, und die Aufräum- und Off-site-Läufe, die sie auslöst, sind im Aktivitätsprotokoll, im Fehlerfeld und in der Sicherungsbenachrichtigung mit „via MCP“ und dem Namen des Schlüssels markiert.
- Jeder Tool-Aufruf wird mit der ID und den letzten vier Zeichen des Schlüssels (nie seinem Namen) in das Container-Protokoll geschrieben und in `/metrics` gezählt (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Das Wiederherstellen einer Konfigurationssicherung entzieht jeden Schlüssel, weil die wiederhergestellte Datenbank Schlüssel enthalten kann, die du nach dem Speichern entzogen hast. Lege danach neue Schlüssel an.
- Ein Schlüssel funktioniert nicht mehr, wenn sich `APP_KEY` ändert (eine Neuinstallation oder eine Wiederherstellung auf einen anderen Container). Die Karte erkennt das und markiert den Schlüssel, und **Schlüssel ersetzen** gibt ihm wieder ein funktionierendes Geheimnis.
- Behandle einen Schlüssel wie ein Passwort. Ein Client, der den Schlüssel nicht aus einer Umgebungsvariable, einer Eingabe oder einer Schlüsseldatei lesen kann, hält ihn im Klartext in seiner Konfiguration oder seinen Einstellungen, und seine Einrichtung sagt das. Bevorzuge auf einem Computer, dem du weniger vertraust, einen schreibgeschützten Schlüssel.

## Was die Box verlässt {#privacy}

Alles, was ein Assistent liest, geht an den KI-Anbieter dahinter: Elementnamen, Zeitpläne, Laufverlauf mit Fehlermeldungen, IDs und Zeiten von Wiederherstellungspunkten, Namen von Datenbank-Engines und Dump-Größen, aktuelle Aktivität, Speicherzahlen, Abdeckung und Status. BombVault entfernt Host-Pfade, Repository-Orte, Hostnamen, Zugangsdaten, Hook-Befehle und Schlüssel, bevor etwas hinausgeht.

## Fehlersuche {#troubleshooting}

| Was du siehst | Was es bedeutet |
|---|---|
| `404` | Kein aktiver Schlüssel und die Anmeldung über OAuth ist aus, oder ein falscher Pfad wie `/api/mcp`. Der Endpunkt ist `/mcp`. |
| `401` | Der Schlüssel fehlt, ist falsch getippt, entzogen oder ersetzt. Ein Proxy lässt vielleicht den Header `Authorization` fallen (versuche `X-API-Key`). Markiert die Karte den Schlüssel als nicht mehr gültig, hat sich `APP_KEY` geändert: Ersetze den Schlüssel. |
| `403` | Die Anfrage kam von einer Browserseite auf einem anderen Origin. Nutze einen Desktop- oder Kommandozeilen-Client. |
| `405` bei GET | Normal. Der Endpunkt nimmt nur `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Der Client ist zu alt für Streamable HTTP. Aktualisiere ihn. |
| `400` "batch requests are not accepted" | Der Client sendet JSON-RPC-Batches. Sende eine Nachricht pro Anfrage. |
| `429` | Zu viele falsche Schlüssel von dieser Adresse, oder mehr als 120 Anfragen pro Minute mit einem Schlüssel. Warte eine Minute und prüfe, ob der Assistent in einer Schleife hängt. |
| Fehler mit "certificate", "self-signed" oder "unable to verify" | Der Client vertraut dem Zertifikat von BombVault nicht. Siehe [TLS und Zertifikate](#tls). |
| `busy` | Eine andere Sicherung oder ein Wartungsjob hält diese Domain. Versuche es erneut, wenn er fertig ist. |
| `cooldown` | Dieses Element, diese Domain oder Alles sichern wurde vor weniger als 15 Minuten außerhalb der Weboberfläche gestartet. |
| `retention_guard` | Eine weitere MCP-Sicherung würde in einem Fenster „letzte N behalten“ nur über MCP gemachte Wiederherstellungspunkte übrig lassen, oder das Element hat in den letzten 24 Stunden schon 4 Sicherungen über MCP bekommen, fehlgeschlagene und abgebrochene eingeschlossen. Im ersten Fall schafft die nächste geplante Sicherung Platz, im zweiten ist das Element 24 Stunden nach der ältesten dieser Sicherungen wieder frei. In beiden Fällen kannst du es in der Weboberfläche starten. |
| `rate_limited` | Der Schlüssel hat seine 12 Starts für diese Stunde verbraucht. |
| `not_permitted` bei einem Start | Der Schlüssel ist schreibgeschützt. Schalte in der Karte **Sicherungen starten erlauben** ein. Keine neue Verbindung nötig. Bei einem Abbruch heißt es, dass der Lauf nicht von diesem Schlüssel gestartet wurde. |
| `domain_off` | Dieser Bereich ist unter **Einstellungen, Allgemein** ausgeschaltet. |
| `not_found` | Das Element wird von BombVault nicht geschützt. Füge es zuerst in der Weboberfläche hinzu. MCP legt nie Konfiguration an. |
| Der Client findet den Autorisierungsserver nicht | Die Anmeldung über OAuth ist aus, es ist kein Login-Passwort gesetzt, oder der Proxy lässt `/.well-known/` nicht zu BombVault durch. |
| Die Zustimmungsseite sagt, die Rückkehradresse sei nicht registriert | Der Client hat eine Rückkehradresse gesendet, die er nicht registriert hat. Entferne den Connector im Client und füge ihn neu hinzu. |
| Ein angemeldeter Client bekommt `401` | Seine Freigabe wurde entzogen, ist nach 30 Tagen ohne Nutzung abgelaufen, oder die öffentliche Adresse hat sich geändert. Der Client meldet sich neu an. |

Setze die Umgebungsvariable `MCPGODEBUG` nicht am Container. Sie ändert das Verhalten der MCP-Bibliothek, und ein fehlerhafter Wert stoppt BombVault beim Start, bevor es eine einzige Protokollzeile schreibt.
