# MCP-Server

BombVault bringt einen Server für das Model Context Protocol (MCP) mit. Über dieses Protokoll greifen KI-Assistenten wie Claude Code und Claude Desktop auf fremde Werkzeuge zu. Ein Assistent kann damit nachlesen, wie es um deine Backups steht, und, wenn du es erlaubst, ein Backup starten oder eines abbrechen, das er selbst gestartet hat. Solange du keinen Schlüssel anlegst und die [Anmeldung über OAuth](#oauth) nicht einschaltest, ist der Server aus: Bis dahin antwortet der Endpunkt `/mcp` auf alles mit `404`.

## Was ein Assistent kann und was nicht {#tools}

| Werkzeug | Was es tut | Art |
|---|---|---|
| `get_health` | Version, Instanzname, ob gerade ein Backup läuft und was dieser Schlüssel darf | lesen |
| `get_status` | Schutzstatus je Domäne: letztes erfolgreiches Backup, erwartetes Intervall, Prüfungen und Off-site-Kontrollen, nächste geplante Läufe | lesen |
| `get_coverage` | Was BombVault schützt und was nicht, jeweils mit Grund | lesen |
| `list_items` | Jeder geschützte Container, jede VM, jedes Ordner-Set, der Flash-Stick und die App-Konfiguration, mit Zeitplan, was ein Backup davon stoppt, dem letzten Backup und seiner Dauer; Datenbank-Container tragen zusätzlich ihren letzten Dump; ZFS-Datasets stehen ebenfalls darin, mit dem Ergebnis ihrer letzten Prüfung | lesen |
| `list_runs` | Laufverlauf, neueste zuerst, filterbar nach Domäne, Element, Status, Art und Zeit | lesen |
| `list_restore_points` | Wiederherstellungspunkte eines Elements aus seinem primären Repository, bei einem Container auch seine Datenbank-Dumps; ein ZFS-Dataset bekommt einen Wiederherstellungspunkt je Backup, mit einem Snapshot jedes Datasets darunter | lesen |
| `get_activity` | Was gerade läuft, mit Phase und Prozentangabe | lesen |
| `get_storage_stats` | Größenverlauf des primären Repositorys einer Domäne und sein Wachstum pro Woche, dazu belegter, freier und gesamter Platz auf dem Datenträger oder Remote jedes ihrer Repositorys | lesen |
| `list_anomalies` | Anomalien, die BombVault in den Backups bemerkt hat, filterbar nach Zustand, Schweregrad und Domäne, mit einer Übersicht über das, was offen ist | lesen |
| `get_anomaly` | Einer dieser Funde, mit der Notiz, die beim Quittieren hinterlassen wurde | lesen |
| `start_backup` | Sichert ein Element sofort | starten |
| `start_domain_backup` | Sichert jedes geschützte Element einer Domäne | starten |
| `start_backup_everything` | Startet ein Gesamt-Backup | starten |
| `cancel_backup` | Bricht ein laufendes Backup ab, das dieser Schlüssel gestartet hat | abbrechen |

Folgendes bleibt in der Web-Oberfläche: Wiederherstellungen jeder Art (auch das Herunterladen, Speichern und Importieren eines Datenbank-Dumps), das Löschen von Backups, Prune, Unlock, Prüfungen und Übungen, die Off-site-Replikation, Einstellungen, Zugangsdaten und MCP-Schlüssel sowie das Abbrechen eines Backups, das der Zeitplan, die Web-Oberfläche oder ein anderer Schlüssel gestartet hat. Dasselbe gilt für das Quittieren einer Anomalie oder das Markieren als erwartet, das auf der Seite **Anomalien** geschieht. Der Grund: Die Antworten der Werkzeuge enthalten Namen und Fehlermeldungen von deinem Server, und in jedem davon kann Text stehen, der den Assistenten lenken soll. Ein Assistent, der darauf hereinfällt, kann im schlimmsten Fall ein Backup innerhalb der unten genannten Grenzen starten oder eines abbrechen, das er selbst gestartet hat.

Liegt das primäre Repository eines Elements woanders (S3, REST, SFTP, rclone), fragt `list_restore_points` dort nach, und der Aufruf kann eine Weile dauern. Off-site-Kopien lassen sich über MCP nicht auflisten. Worauf die Anomalie-Prüfungen achten, steht unter [Funktionen](features.md), und wie ein ZFS-Element für jedes Dataset einen eigenen Snapshot anlegt, unter [ZFS-Datasets](zfs-datasets.md#contents).

## Was ein gestartetes Backup tut {#starting-backups}

Das Backup eines Assistenten ist dasselbe Backup, das die Web-Oberfläche startet. Ein laufender Container wird bis zum Ende seines Backups gestoppt, zusammen mit den Containern, die mit ihm stoppen sollen. Eine VM mit der Methode "graceful" wird heruntergefahren und wieder gestartet. Ein ZFS-Dataset stoppt die dafür eingestellten Container, solange sein Snapshot entsteht. Ordner-Sets, der Flash-Stick und die Konfiguration laufen weiter. Danach wendet BombVault die Aufbewahrungsregel an und kopiert eventuell ins Off-site-Repository. `list_items` sagt dem Assistenten, was ein Element stoppt und wie lange sein letztes Backup gedauert hat, und die Beschreibungen der Werkzeuge bitten ihn, dir das vor dem Start zu sagen.

Weil ein Backup Dinge anhält und alte Wiederherstellungspunkte verdrängt, sind Starts über MCP begrenzt:

- 12 gestartete Backups pro Stunde und Schlüssel.
- 15 Minuten zwischen zwei MCP-Starts desselben Elements, derselben Domäne oder des Gesamt-Backups.
- Höchstens 4 MCP-Starts desselben Elements in 24 Stunden.
- **Aufbewahrungsschutz.** Behält eine Domäne eine feste Anzahl Wiederherstellungspunkte (nur "die letzten N behalten", ohne tägliche, wöchentliche oder monatliche Regel, lokal oder auf einem Off-site-Ziel), schiebt jedes neue Backup den ältesten hinaus. BombVault lehnt dann einen MCP-Start eines Elements ab, dessen neueste N-1 erfolgreiche Backups alle über MCP gestartet wurden. So bleibt immer mindestens ein Wiederherstellungspunkt im behaltenen Satz, den der Zeitplan oder du angelegt hast. Bei "die letzten 1 behalten" kann ein Assistent dieses Element gar nicht sichern. Das nächste geplante Backup schafft wieder Platz.

Ein Start einer Domäne oder des Gesamt-Backups lässt die Elemente aus, die eine Grenze zurückhält, und nennt sie in der Antwort. Die Web-Oberfläche und der Zeitplan sind von alldem nicht betroffen. Das Stundenkontingent liegt im Speicher, ein Neustart von BombVault setzt es also zurück.

Starts über die [API](api.md#errors) und aus [Home Assistant](api.md#home-assistant) zählen bei den Grenzen je Element und beim Aufbewahrungsschutz mit den MCP-Starts zusammen.

## Einschalten {#switch-on}

1. Öffne **Einstellungen, System, MCP-Server** und klick auf den Knopf deines Clients. Ein Client, der nicht in der Liste steht, verbindet sich über **Anderer Client**.
2. Lass unter **Schlüssel** die Auswahl **Neuer Schlüssel** und den vorgeschlagenen Namen, also den des Clients, oder gib einen ein, der sagt, wo der Schlüssel benutzt wird, zum Beispiel „Claude Code auf dem Laptop“. Ein Schlüssel pro Client erlaubt es, einen zu widerrufen, ohne die anderen anzufassen. **Vorhandener Schlüssel** gibt dem Client einen Schlüssel, den du früher angelegt hast.
3. Schalte **Backups starten erlauben** für einen Schlüssel ein, der Backups starten können soll; ohne das kann er nur lesen. Du kannst es später auf der Kachel des Schlüssels ändern, und die Änderung gilt ab der nächsten Anfrage des Assistenten, ohne neue Verbindung.
4. Klick auf **Schlüssel anlegen**. Der Schlüssel wird einmal angezeigt. BombVault behält nur einen Fingerabdruck davon und kann ihn nicht noch einmal zeigen, also kopier ihn jetzt. Schließt du den Dialog, bevor der Client den Schlüssel benutzt hat, zeigt die Karte ihn weiter an, bis du bestätigst, dass du ihn kopiert hast.

Ohne Login-Passwort ist schon die Web-Oberfläche für alle in deinem Netz offen, und wer sie öffnen kann, kann auch einen Schlüssel anlegen. Die Karte sagt das. Öffnest du BombVault unter einem öffentlich aussehenden Namen (zum Beispiel `bombvault.example.com` über einen Reverse Proxy) und ist kein Login-Passwort gesetzt, lassen sich von dieser Adresse keine Schlüssel anlegen oder ersetzen. So kann keine Webseite im Internet deinen Browser dazu bringen, einen anzulegen. Setz ein Login-Passwort, oder öffne BombVault über seine IP-Adresse oder einen lokalen Namen wie `tower` oder `tower.local`.

## Deine Schlüssel und ihr Protokoll {#keys}

Jeder Schlüssel hat auf der Karte eine eigene Kachel. Sie zeigt den Namen, ob der Schlüssel Backups starten darf oder nur liest, die letzten vier Zeichen des Schlüssels, wann er angelegt oder zuletzt ersetzt wurde, wann ein Client ihn zuletzt benutzt hat und wie viele Aufrufe er heute gemacht hat. Auf der Kachel benennst du den Schlüssel um, änderst seine Berechtigung, ersetzt ihn oder widerrufst ihn. Ein widerrufener Schlüssel wandert in die Liste der widerrufenen Schlüssel. Dort kannst du ihn endgültig löschen, sobald kein Lauf im Verlauf ihn mehr nennt.

Neben dem Namen zeigt die Kachel das Zeichen des Clients, für den der Schlüssel angelegt wurde. Ein Schlüssel, der über **Anderer Client** entstanden ist oder bevor die Karte Clients aufgelistet hat, zeigt stattdessen einen Schlüssel.

**Protokoll** auf einer Kachel zeigt, was dieser Schlüssel getan hat. Oben stehen die Backups, die er gestartet hat, jeweils mit ihrem Stand und einem Link auf diesen Lauf im Aktivitätsprotokoll des Dashboards. Darunter stehen seine Aufrufe, die neuesten zuerst, mit dem Werkzeug und dem Ergebnis. Eine Ablehnung nennt den Grund: Der Schlüssel darf nur lesen, der Aufbewahrungsschutz hat das Backup zurückgehalten, es lief schon ein anderes Backup, das Element wurde vor wenigen Minuten über MCP gesichert, oder der Schlüssel hat zu viele Anfragen geschickt. Ein Abbruch verlinkt den Lauf, um den es ging.

BombVault hebt die Einträge jedes Schlüssels bis zu 30 Tage auf: die neuesten 500 erfolgreichen Starts und Abbrüche und daneben die neuesten 200 übrigen Aufrufe (Lesezugriffe, Ablehnungen und Fehler). Ein Assistent, der ein laufendes Backup immer wieder abfragt oder einen abgelehnten Aufruf immer wieder versucht, kann so dessen Start nicht aus dem Protokoll drängen. Zu jedem Aufruf speichert es das Werkzeug, das Ergebnis und bei einem Abbruch den Lauf. Was der Assistent geschickt hat, speichert es nie, den Schlüssel und seinen Fingerabdruck auch nicht. Das Diagnosepaket zählt die Einträge nur, und ein Einstellungsexport lässt sie weg.

## Client verbinden {#clients}

Jeder Client hat auf der Karte einen Knopf, unter **Auf diesem Rechner** oder **In der Cloud**. Der Knopf öffnet einen Dialog in drei Schritten: der Schlüssel; die Konfiguration für diesen Client, mit der Adresse, unter der du die Karte geöffnet hast, einem Knopf zum Kopieren, dem Ort, an dem die Konfiguration liegt, und bei BombVaults eigenem Zertifikat dem, was der Client braucht, um ihm zu vertrauen; und das Warten auf den ersten Aufruf des Clients. Der Dialog beobachtet die letzte Nutzung des Schlüssels und wird grün, sobald dieser Aufruf ankommt.

Der Dialog hält den Schlüssel von jeder Befehlszeile fern. Wo der Client ihn aus einer Umgebungsvariable (`BOMBVAULT_MCP_KEY`), einer verdeckten Abfrage oder einer eigenen Datei lesen kann, nennt die Konfiguration ihn nur. Wo der Client das nicht kann, steht der Schlüssel in seiner Konfigurationsdatei oder seinen Einstellungen, und der Dialog sagt das. Wo die Dokumentation eines Clients nicht sagt, wie er mit einem unbekannten Zertifikat umgeht, schreibt der Dialog diesen Schritt als das, was zu tun ist, wenn der Client BombVaults Zertifikat ablehnt.

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
| GitHub Copilot (VS Code) | Konfigurationsdatei | verdeckte Abfrage |
| Goose | Konfigurationsdatei | Umgebungsvariable |
| Jan | Formular in der App | die Einstellungen der App |
| JetBrains (AI Assistant, Junie) | Konfigurationsdatei | die Konfigurationsdatei |
| Kimi Code | Konfigurationsdatei | die Konfigurationsdatei |
| LM Studio | Konfigurationsdatei | die Konfigurationsdatei |
| Mistral Vibe | Konfigurationsdatei | Umgebungsvariable |
| Msty | Formular in der App | die Einstellungen der App |
| n8n | Formular in der App | die Zugangsdaten von n8n |
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
| ChatGPT | Anmeldung über OAuth, in der Cloud | ein Zugriffstoken, siehe [unten](#oauth) |
| Claude (claude.ai) | Anmeldung über OAuth, in der Cloud | ein Zugriffstoken, siehe [unten](#oauth) |

Die Abschnitte unten erklären die Einrichtung von Claude Code und Claude Desktop genauer und nennen, was jeder andere Client braucht.

### Claude Code {#claude-code}

Claude Code erreicht BombVault über `mcp-remote`, das Node.js auf dem Rechner braucht. Speicher den Schlüssel zuerst in einer eigenen Textdatei, als einzelne Zeile:

```text
X-API-Key: <your key>
```

Führ dann den Befehl aus der Karte einmal im Terminal aus, mit dem Pfad dieser Datei eingesetzt. Hinter einem Zertifikat, dem dein Rechner vertraut, sieht er so aus:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

Mit BombVaults eigenem Zertifikat (siehe [TLS und Zertifikate](#tls)) zeigt der Befehl Node.js außerdem das heruntergeladene Zertifikat:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Prüf die Verbindung mit `/mcp` in Claude Code. `--scope user` macht BombVault in all deinen Projekten verfügbar. Claude Code merkt sich nur den Pfad der Schlüsseldatei, deshalb taucht der Schlüssel weder im Befehl und in deiner Shell-History noch in der Prozessliste auf. Leg die Datei dort ab, wo nur du sie lesen kannst, und außerhalb jedes Ordners, den du committest. `@latest` sorgt dafür, dass `npx` ein aktuelles `mcp-remote` holt; sonst würde ein älteres, global installiertes genommen, und das kennt `--header-file` nicht.

Schreib `${BOMBVAULT_MCP_KEY}` für Claude Code nicht in die Argumente von `mcp-remote`. Claude Code setzt so einen Verweis aus seiner eigenen Umgebung ein, bevor es `mcp-remote` startet. Der Schlüssel landet dann in der Befehlszeile dieses Prozesses, wo andere Programme und Benutzer des Rechners ihn lesen können.

Ohne Node.js, und nur hinter einem Zertifikat, dem dein Rechner vertraut, kann sich Claude Code selbst verbinden. Leg dazu eine `.mcp.json` in den Projektordner:

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

Setz `BOMBVAULT_MCP_KEY` dort, wo Claude Code startet, zum Beispiel unter `"env"` in `~/.claude/settings.json` oder in deinem Shell-Profil, und zwar im Texteditor statt an der Eingabeaufforderung. Hier ist der Verweis sicher, weil Claude Code keinen zweiten Prozess startet, der ihn mitträgt. Mit BombVaults eigenem Zertifikat geht dieser Weg nicht: Die Verbindung, die Claude Code selbst aufbaut, lehnt es ab, auch wenn `NODE_EXTRA_CA_CERTS` gesetzt ist. Committe nie eine `.mcp.json`, in der der Schlüssel ausgeschrieben steht.

### Claude Desktop {#claude-desktop}

Claude Desktop erreicht BombVault über `mcp-remote`, das Node.js auf dem Rechner braucht. Speicher den Schlüssel zuerst in einer eigenen Textdatei als einzelne Zeile, wie bei [Claude Code](#claude-code) beschrieben. Öffne die Konfigurationsdatei in Claude Desktop über **Einstellungen, Entwickler, Konfiguration bearbeiten** (Settings, Developer, Edit Config). Sie liegt unter Windows in `%APPDATA%\Claude\claude_desktop_config.json` und unter macOS in `~/Library/Application Support/Claude/claude_desktop_config.json`. Trag den Eintrag aus der Karte unter `"mcpServers"` ein, neben die Server, die schon dort stehen, und starte Claude Desktop neu:

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

- `NODE_EXTRA_CA_CERTS` steht nur für BombVaults eigenes Zertifikat da. Hinter einem Zertifikat, dem dein Rechner schon vertraut, lässt du es weg.
- `--allow-http` kommt nur bei einer schlichten `http://`-Adresse dazu.
- Unter Windows schreibst du die Pfade mit normalen Schrägstrichen, etwa `C:/Users/sam/bombvault-key.txt`, denn ein einzelner Backslash ist kein gültiges JSON. Halte den Pfad der Schlüsseldatei frei von Leerzeichen: Claude Desktop gibt unter Windows einen Pfad mit Leerzeichen in zwei Teilen an `npx` weiter.
- Die Konfiguration nennt nur die Schlüsseldatei, deshalb taucht der Schlüssel weder in ihr noch in der Prozessliste auf. Leg die Datei dort ab, wo nur du sie lesen kannst.

### Clients in der Cloud {#cloud-clients}

ChatGPT, Claude auf claude.ai, Grok und Le Chat rufen BombVault von den Servern ihrer Anbieter aus auf. BombVault muss dafür aus dem Internet erreichbar sein, mit einem öffentlich vertrauenswürdigen Zertifikat, zum Beispiel hinter einem Reverse Proxy; Le Chat lehnt selbst ausgestellte ab. Ein Login am Proxy darf die Weboberfläche schützen, aber `/mcp` muss ohne ihn zu BombVault durchgehen: Diese Dienste können sich an keinem Proxy anmelden, und BombVault prüft ihren Schlüssel oder ihr Token selbst. Grok und Le Chat schicken einen festen Schlüssel, und ihre Knöpfe richten sie ein wie die anderen. ChatGPT und in den meisten Organisationen auch Claude auf claude.ai verbinden sich nur über eine Anmeldung mit OAuth, die als Nächstes beschrieben ist.

### Anmeldung über OAuth {#oauth}

Für einen Client, der keinen Schlüssel annimmt, ist BombVault sein eigener OAuth-Anmeldeserver. Der Client registriert sich selbst, schickt dich auf eine Seite von BombVault, und dort meldest du dich mit deinem Login-Passwort (und dem zweiten Faktor, falls eingerichtet) an und erlaubst ihn. Der Client bekommt dann ein Token, das nur für den MCP-Endpunkt dieses BombVault gilt, und erneuert es selbst.

1. Setz unter **Einstellungen, System** ein Login-Passwort. Ohne Passwort bietet BombVault gar keine Anmeldung an, weil es niemanden gäbe, der zustimmen könnte.
2. Mach BombVault über https aus dem Internet erreichbar, mit einem Zertifikat, dem Browser vertrauen, meist über einen Reverse Proxy. Der Client ruft `/mcp`, `/oauth/` und `/.well-known/` von seinen eigenen Servern aus auf, ein Proxy mit eigenem Login muss diese drei Pfade also zu BombVault durchlassen. Die Zustimmungsseite unter `/oauth/authorize` öffnet sich in deinem eigenen Browser und darf hinter dem Proxy-Login bleiben. Trag den Proxy außerdem in `TRUSTED_PROXY` ein (siehe [Konfiguration](configuration.md)). BombVault begrenzt die Registrierungen von Clients pro Adresse, und ohne diesen Eintrag scheint jeder Client vom Proxy zu kommen.
3. Schalte auf der MCP-Karte **Anmeldung über OAuth** ein und trag die **Öffentliche Adresse** ein: die https-Adresse ohne Pfad, zum Beispiel `https://backup.example.com`. Jedes Token ist an diese Adresse gebunden, nach einer Änderung muss sich also jeder Client neu anmelden.
4. Klick auf den Knopf von ChatGPT oder Claude. Der Dialog zeigt die **Connector-URL**, also die öffentliche Adresse mit `/mcp` dahinter, und wo sie in diesem Client hingehört. In ChatGPT schaltest du unter **Einstellungen, Apps & Connectors, Erweiterte Einstellungen** den Entwicklermodus ein, wählst **Erstellen**, fügst die Connector-URL als MCP-Server-URL ein und wählst OAuth als Authentifizierung. Auf claude.ai öffnest du **Einstellungen, Connectors, Benutzerdefinierten Connector hinzufügen**, fügst die Connector-URL ein, lässt OAuth-Client-ID und Secret leer und wählst **Verbinden**.
5. Der Client öffnet die Zustimmungsseite. Sie zeigt, wer fragt, wohin dich deine Antwort zurückschickt, und den Schalter **Backups starten erlauben**, der aus ist. Wähle **Erlauben** oder **Ablehnen**.

Jeder angemeldete Client bekommt eine Kachel neben den Schlüsseln, mit seinem Zeichen, seinem Log, **Widerrufen** und **Backups starten erlauben**, und dieselben Grenzen wie ein Schlüssel. Widerrufen wirkt sofort. Meldet sich derselbe Client neu an, ersetzt seine neue Freigabe die alte, und eine Freigabe, die 30 Tage niemand benutzt hat, läuft ab. Bis zu 10 Clients können gleichzeitig angemeldet sein, zusätzlich zu den 10 Schlüsseln.

Die Zustimmungsseite nimmt eine Anfrage nur von einem registrierten Client an, der genau eine seiner registrierten Rücksprungadressen nennt: https, oder eine Loopback-Adresse mit beliebigem Port für einen Client auf deinem eigenen Rechner. Angenommen wird nur der Authorization-Code-Ablauf mit PKCE (S256), und deine Antwort ist an deine Sitzung gebunden, keine andere Website kann sie also für dich abschicken. Zugriffstokens gelten eine Stunde. Ein Refresh-Token wird bei jeder Benutzung ersetzt, und taucht eines danach noch einmal auf, widerruft BombVault die Freigabe, weil jemand anderes eine Kopie hat. Wiederholt ein Client seine letzte Erneuerung innerhalb von 30 Sekunden, weil ihn die Antwort nicht erreicht hat, bekommt er stattdessen neue Tokens. BombVault lädt keine Client-Metadaten aus dem Internet, Clients registrieren sich daher über die dynamische Client-Registrierung.

### Andere Clients {#other-clients}

Jeder Client, der Streamable HTTP spricht, funktioniert:

- URL: die Adresse der Web-Oberfläche plus `/mcp`, zum Beispiel `https://192.168.1.10:3443/mcp`.
- Der Schlüssel in `Authorization: Bearer <key>` oder in `X-API-Key: <key>`. Kommen beide, müssen sie denselben Schlüssel tragen.
- `POST` mit `Content-Type: application/json` und `Accept: application/json, text/event-stream`.
- Eine JSON-RPC-Nachricht pro Anfrage; Batches werden abgelehnt.
- Protokollversionen 2026-07-28, 2025-11-25, 2025-06-18 und 2025-03-26.

## TLS und Zertifikate {#tls}

BombVault liefert HTTPS mit einem selbst ausgestellten Zertifikat aus, und das nennt anfangs nur `localhost`, `127.0.0.1` und `::1`. Claude Code und `mcp-remote` lehnen es auf einer LAN-Adresse ab. Die Wege darum herum, in der Reihenfolge, die zu den meisten Unraid-Installationen passt:

1. **Die Adresse in der MCP-Karte eintragen.** Öffnest du die Karte über HTTPS unter einer Adresse, die das Zertifikat nicht nennt, sagt sie das und bietet **Diese Adresse ins Zertifikat eintragen** an. BombVault stellt sein Zertifikat dann mit dieser Adresse neu aus (dein Browser warnt noch einmal, wie beim ersten Mal). Danach klickst du auf **Zertifikat herunterladen**; die Ausschnitte setzen `NODE_EXTRA_CA_CERTS` auf die heruntergeladene Datei, sodass der Client genau diesem Zertifikat vertraut. Das heißt auch: Jeder Client, der mit einer früher heruntergeladenen Datei eingerichtet ist, verbindet sich ab dem Neuausstellen nicht mehr, auf diesem Rechner wie auf jedem anderen, bis er die neue Datei bekommt.
2. **Ein Reverse Proxy mit vertrauenswürdigem Zertifikat** (Nginx Proxy Manager, SWAG, Caddy, Traefik). Der Client sieht dann das Zertifikat des Proxys und braucht nichts weiter, und die Karte warnt nicht vor BombVaults eigenem.
3. **Tailscale.** `tailscale serve` vor dem Container oder die Tailscale-Integration von Unraid gibt dir einen `ts.net`-Namen mit vertrauenswürdigem Zertifikat.
4. **`HTTP_ONLY=true`**, nur hinter einem Proxy, der TLS beendet, oder in einem Netz, dem du voll vertraust. Es schaltet die ganze Web-Oberfläche auf schlichtes HTTP, verlangt eine Änderung an den Container-Einstellungen und schickt den Schlüssel unverschlüsselt.

Setz nie `NODE_TLS_REJECT_UNAUTHORIZED=0`. Das schaltet die Zertifikatsprüfung für alles ab, womit dieser Node.js-Prozess redet.

Ein Reverse Proxy muss den `Authorization`-Header (oder `X-API-Key`) durchreichen, was Proxys tun, solange man es ihnen nicht anders sagt, und darf `/mcp` weder puffern noch umschreiben. Ein Location-Block für Nginx oder Nginx Proxy Manager, der auch BombVaults Zertifikat prüft:

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

Hinter einem Proxy trägt jede Anfrage die Adresse des Proxys. Fünf falsche Schlüssel von einem falsch eingerichteten Client sperren dann jeden MCP-Client hinter diesem Proxy für eine Minute aus. Trag den Proxy in `TRUSTED_PROXY` ein (siehe [Konfiguration](configuration.md)), dann wird pro Client gezählt.

## Sicherheitsmodell {#security}

- Ohne aktiven Schlüssel und bei ausgeschalteter Anmeldung über OAuth antwortet `/mcp` mit `404`.
- Die Anmeldung über OAuth wird nur angeboten, solange ein Login-Passwort gesetzt ist. Tokens, Codes und Client-Secrets werden nur als Fingerabdruck gespeichert, und ein Token gilt nur für die Adresse, für die es ausgestellt wurde.
- Ein Client kann sich von einer Adresse aus höchstens 10-mal pro Stunde registrieren, und BombVault behält höchstens 100 registrierte Clients, mit denen sich niemand angemeldet hat, jeweils einen Tag lang. Falsche Codes und Refresh-Tokens zählen zur selben Sperre wie falsche Schlüssel.
- Freigaben verhalten sich bei der Wiederherstellung eines Konfigurations-Backups und bei einem geänderten `APP_KEY` wie Schlüssel: Nach einer Wiederherstellung muss sich jeder Client neu anmelden.
- Keine Ausnahmen für bestimmte Adressen. Anfragen von `localhost`, vom Unraid-Host, von einem Reverse Proxy oder von `tailscale serve` brauchen einen Schlüssel wie jede andere, auch wenn die Web-Oberfläche kein Login-Passwort hat.
- Schlüssel werden nur als Fingerabdruck gespeichert, einmal angezeigt und lassen sich umbenennen, ersetzen und widerrufen. Bis zu 10 aktive Schlüssel, jeder mit eigenem Schalter **Backups starten erlauben**.
- Jedes Anlegen, Ersetzen, jede Rechteänderung und jeder Widerruf schickt eine Benachrichtigung über deine Kanäle, mit der Adresse, von der es kam, außer die Benachrichtigungen sind ausgeschaltet.
- 5 falsche Schlüssel pro Minute und Adresse, danach `429`. 120 Anfragen pro Minute und 12 gestartete Backups pro Stunde und Schlüssel, dazu die Wartezeit und der Aufbewahrungsschutz von oben.
- Anfragen einer Browserseite von einem anderen Origin werden abgelehnt.
- Solange kein Login-Passwort gesetzt ist, lassen sich unter einem öffentlich aussehenden Hostnamen keine Schlüssel anlegen.
- Jedes Backup, das ein Assistent startet, und die Prune- und Off-site-Läufe, die daraus folgen, sind mit "über MCP" und dem Namen des Schlüssels markiert: im Aktivitätsprotokoll, im Fehlerpanel und in der Backup-Benachrichtigung.
- Jeder Werkzeugaufruf landet im Container-Log mit der ID und den letzten vier Zeichen des Schlüssels (nie mit seinem Namen) und wird in `/metrics` gezählt (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Das Wiederherstellen eines Konfigurations-Backups widerruft jeden Schlüssel, weil die wiederhergestellte Datenbank Schlüssel enthalten kann, die du nach ihrer Sicherung widerrufen hast. Leg danach neue an.
- Ein Schlüssel hört auf zu funktionieren, wenn sich `APP_KEY` ändert (eine Neuinstallation oder eine Wiederherstellung in einen anderen Container). Die Karte erkennt das und markiert den Schlüssel, und **Schlüssel ersetzen** gibt ihm wieder ein gültiges Geheimnis.
- Behandle einen Schlüssel wie ein Passwort. Ein Client, der den Schlüssel weder aus einer Umgebungsvariable noch aus einer Abfrage oder einer Schlüsseldatei lesen kann, hält ihn im Klartext in seiner Konfiguration oder seinen Einstellungen, und sein Dialog sagt das. Nimm auf einem Rechner, dem du weniger vertraust, lieber einen Schlüssel, der nur lesen darf.

## Was die Box verlässt {#privacy}

Was ein Assistent liest, geht an den KI-Anbieter dahinter: Namen von Elementen, Zeitpläne, der Laufverlauf mit Fehlermeldungen, IDs und Zeiten von Wiederherstellungspunkten, die Namen der Datenbank-Engines und die Größen der Dumps, die laufende Aktivität, Speicherzahlen, Abdeckung und Status. BombVault entfernt Host-Pfade, Repository-Orte, Hostnamen, Zugangsdaten, Hook-Befehle und Schlüssel, bevor etwas hinausgeht.

## Fehlersuche {#troubleshooting}

| Was du siehst | Was es bedeutet |
|---|---|
| `404` | Kein aktiver Schlüssel und die Anmeldung über OAuth ist aus, oder ein falscher Pfad wie `/api/mcp`. Der Endpunkt ist `/mcp`. |
| `401` | Der Schlüssel fehlt, ist vertippt, widerrufen oder ersetzt. Vielleicht verwirft ein Proxy den `Authorization`-Header (versuch `X-API-Key`). Markiert die Karte den Schlüssel als ungültig, hat sich `APP_KEY` geändert: Ersetze den Schlüssel. |
| `403` | Die Anfrage kam von einer Browserseite mit anderem Origin. Nimm einen Desktop- oder Kommandozeilen-Client. |
| `405` bei GET | Normal. Der Endpunkt nimmt nur `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Der Client ist zu alt für Streamable HTTP. Aktualisiere ihn. |
| `400` "batch requests are not accepted" | Der Client schickt JSON-RPC-Batches. Schick eine Nachricht pro Anfrage. |
| `429` | Zu viele falsche Schlüssel von dieser Adresse, oder mehr als 120 Anfragen pro Minute mit einem Schlüssel. Warte eine Minute und prüf, ob der Assistent in einer Schleife hängt. |
| Fehler mit "certificate", "self-signed" oder "unable to verify" | Der Client vertraut BombVaults Zertifikat nicht. Siehe [TLS und Zertifikate](#tls). |
| `busy` | Ein anderes Backup oder eine Wartungsaufgabe belegt diese Domäne. Versuch es wieder, wenn sie fertig ist. |
| `cooldown` | Dieses Element, diese Domäne oder das Gesamt-Backup wurde vor weniger als 15 Minuten über MCP gestartet. |
| `retention_guard` | Ein weiteres MCP-Backup ließe in einem "die letzten N behalten"-Fenster nur noch Wiederherstellungspunkte aus MCP übrig, oder das Element hat in den letzten 24 Stunden schon 4 Backups über MCP bekommen, fehlgeschlagene und abgebrochene mitgezählt. Im ersten Fall schafft das nächste geplante Backup Platz, im zweiten ist das Element 24 Stunden nach dem ältesten dieser Backups wieder frei. In der Web-Oberfläche kannst du es jederzeit starten. |
| `rate_limited` | Der Schlüssel hat seine 12 Starts für diese Stunde verbraucht. |
| `not_permitted` bei einem Start | Der Schlüssel darf nur lesen. Schalte **Backups starten erlauben** in der Karte ein; eine neue Verbindung ist nicht nötig. Bei einem Abbruch heißt es, dass dieser Schlüssel den Lauf nicht gestartet hat. |
| `domain_off` | Diese Backup-Art ist in den Einstellungen ausgeschaltet. |
| `not_found` | BombVault schützt dieses Element nicht. Nimm es zuerst in der Web-Oberfläche auf; MCP legt nie Konfiguration an. |
| Der Client findet den Anmeldeserver nicht | Die Anmeldung über OAuth ist aus, es ist kein Login-Passwort gesetzt, oder der Proxy lässt `/.well-known/` nicht zu BombVault durch. |
| Die Zustimmungsseite meldet eine nicht registrierte Rücksprungadresse | Der Client hat eine Rücksprungadresse geschickt, die er nicht registriert hat. Entferne den Connector im Client und füge ihn neu hinzu. |
| Ein angemeldeter Client bekommt `401` | Seine Freigabe wurde widerrufen, ist nach 30 Tagen ohne Nutzung abgelaufen, oder die öffentliche Adresse hat sich geändert. Der Client meldet sich neu an. |

Setz die Umgebungsvariable `MCPGODEBUG` am Container nicht. Sie ändert das Verhalten der MCP-Bibliothek, und ein fehlerhafter Wert hält BombVault beim Start an, bevor es auch nur eine Log-Zeile schreibt.
