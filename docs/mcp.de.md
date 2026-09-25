# MCP-Server

BombVault bringt einen Server für das Model Context Protocol (MCP) mit. Über dieses Protokoll greifen KI-Assistenten wie Claude Code und Claude Desktop auf fremde Werkzeuge zu. Ein Assistent kann damit nachlesen, wie es um deine Backups steht, und, wenn du es erlaubst, ein Backup starten oder eines abbrechen, das er selbst gestartet hat. Solange du keinen Schlüssel anlegst, ist der Server aus: Ohne aktiven Schlüssel antwortet der Endpunkt `/mcp` auf alles mit `404`.

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
| `get_storage_stats` | Größenverlauf des primären Repositorys einer Domäne und sein Wachstum pro Woche | lesen |
| `list_anomalies` | Anomalien, die BombVault in den Backups bemerkt hat, filterbar nach Zustand, Schweregrad und Domäne, mit einer Übersicht über das, was offen ist | lesen |
| `get_anomaly` | Einer dieser Befunde, mit der Notiz, die beim Bestätigen hinterlassen wurde | lesen |
| `start_backup` | Sichert ein Element sofort | starten |
| `start_domain_backup` | Sichert jedes geschützte Element einer Domäne | starten |
| `start_backup_everything` | Startet ein Gesamt-Backup | starten |
| `cancel_backup` | Bricht ein laufendes Backup ab, das dieser Schlüssel gestartet hat | abbrechen |

Folgendes bleibt in der Web-Oberfläche: Wiederherstellungen jeder Art (auch das Herunterladen, Speichern und Importieren eines Datenbank-Dumps), das Löschen von Backups, Prune, Unlock, Prüfungen und Übungen, die Off-site-Replikation, Einstellungen, Zugangsdaten und MCP-Schlüssel sowie das Abbrechen eines Backups, das der Zeitplan, die Web-Oberfläche oder ein anderer Schlüssel gestartet hat. Dasselbe gilt für das Bestätigen einer Anomalie oder das Markieren als erwartet, das auf der Seite **Anomalien** geschieht. Der Grund: Die Antworten der Werkzeuge enthalten Namen und Fehlermeldungen von deinem Server, und in jedem davon kann Text stehen, der den Assistenten lenken soll. Ein Assistent, der darauf hereinfällt, kann im schlimmsten Fall ein Backup innerhalb der unten genannten Grenzen starten oder eines abbrechen, das er selbst gestartet hat.

Liegt das primäre Repository eines Elements woanders (S3, REST, SFTP, rclone), fragt `list_restore_points` dort nach, und der Aufruf kann eine Weile dauern. Off-site-Kopien lassen sich über MCP nicht auflisten. Worauf die Anomalie-Prüfungen achten, steht unter [Funktionen](features.md), und wie ein ZFS-Element für jedes Dataset einen eigenen Snapshot anlegt, unter [ZFS-Datasets](zfs-datasets.md#contents).

## Was ein gestartetes Backup tut {#starting-backups}

Das Backup eines Assistenten ist dasselbe Backup, das die Web-Oberfläche startet. Ein laufender Container wird bis zum Ende seines Backups gestoppt, zusammen mit den Containern, die mit ihm stoppen sollen. Eine VM mit der Methode "graceful" wird heruntergefahren und wieder gestartet. Ein ZFS-Dataset stoppt die dafür eingestellten Container, solange sein Snapshot entsteht. Ordner-Sets, der Flash-Stick und die Konfiguration laufen weiter. Danach wendet BombVault die Aufbewahrungsregel an und kopiert eventuell ins Off-site-Repository. `list_items` sagt dem Assistenten, was ein Element stoppt und wie lange sein letztes Backup gedauert hat, und die Beschreibungen der Werkzeuge bitten ihn, dir das vor dem Start zu sagen.

Weil ein Backup Dinge anhält und alte Wiederherstellungspunkte verdrängt, sind Starts über MCP begrenzt:

- 12 gestartete Backups pro Stunde und Schlüssel.
- 15 Minuten zwischen zwei MCP-Starts desselben Elements, derselben Domäne oder des Gesamt-Backups.
- Höchstens 4 MCP-Starts desselben Elements in 24 Stunden.
- **Aufbewahrungsschutz.** Behält eine Domäne eine feste Anzahl Wiederherstellungspunkte (nur "die letzten N behalten", ohne tägliche, wöchentliche oder monatliche Regel, lokal oder auf einem Off-site-Ziel), schiebt jedes neue Backup den ältesten hinaus. BombVault lehnt dann einen MCP-Start eines Elements ab, dessen neueste N-1 erfolgreiche Backups alle über MCP gestartet wurden. So bleibt immer mindestens ein Wiederherstellungspunkt im behaltenen Satz, den der Zeitplan oder du angelegt hast. Bei "die letzten 1 behalten" kann ein Assistent dieses Element gar nicht sichern. Das nächste geplante Backup schafft wieder Platz.

Ein Start einer Domäne oder des Gesamt-Backups lässt die Elemente aus, die eine Grenze zurückhält, und nennt sie in der Antwort. Die Web-Oberfläche und der Zeitplan sind von alldem nicht betroffen. Das Stundenkontingent liegt im Speicher, ein Neustart von BombVault setzt es also zurück.

## Einschalten {#switch-on}

1. Öffne **Einstellungen, System, MCP-Server** und klick auf **Neuer Schlüssel**.
2. Gib dem Schlüssel einen Namen, der sagt, wo er benutzt wird, zum Beispiel "Claude Code auf dem Laptop". Mit einem Schlüssel pro Client kannst du einen widerrufen, ohne die anderen anzufassen.
3. Lass **Backups starten erlauben** an, oder schalte es für einen Schlüssel aus, der nur lesen soll. Du kannst das später in der Zeile des Schlüssels ändern, und die Änderung gilt ab der nächsten Anfrage des Assistenten, ohne neue Verbindung.
4. Klick auf **Schlüssel anlegen**. Der Schlüssel wird einmal angezeigt. BombVault behält nur einen Fingerabdruck davon und kann ihn nicht noch einmal zeigen, also kopiere ihn gleich oder nimm einen der Ausschnitte darunter, die dann den echten Schlüssel enthalten.

Ohne Login-Passwort ist schon die Web-Oberfläche für alle in deinem Netz offen, und wer sie öffnen kann, kann auch einen Schlüssel anlegen. Die Karte sagt das. Öffnest du BombVault unter einem öffentlich aussehenden Namen (zum Beispiel `bombvault.example.com` über einen Reverse Proxy) und ist kein Login-Passwort gesetzt, lassen sich von dieser Adresse keine Schlüssel anlegen oder ersetzen. So kann keine Webseite im Internet deinen Browser dazu bringen, einen anzulegen. Setz ein Login-Passwort, oder öffne BombVault über seine IP-Adresse oder einen lokalen Namen wie `tower` oder `tower.local`.

## Client verbinden {#clients}

Die Karte zeigt fertige Ausschnitte für die Adresse, unter der du sie geöffnet hast: Client wählen und Ausschnitt kopieren. Hier steht, was die Ausschnitte tun, und die Formen, die die Karte nicht zeigt.

### Claude Code {#claude-code}

Führ den Befehl aus der Karte einmal im Terminal aus. Mit einem Zertifikat, dem dein Rechner vertraut, sieht er so aus:

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

Prüf die Verbindung mit `/mcp` in Claude Code. `--scope user` legt den Schlüssel in deiner Benutzerkonfiguration ab und nicht in einer Projektdatei.

Der Befehl enthält den Schlüssel, und deine Shell hebt ihn womöglich in ihrer History auf. Das vermeidest du mit einer `.mcp.json` im Projektordner und dem Schlüssel in einer Umgebungsvariablen. Claude Code setzt `${BOMBVAULT_MCP_KEY}` beim Lesen der Datei ein:

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

Setz `BOMBVAULT_MCP_KEY` dort, wo Claude Code startet, zum Beispiel in deinem Shell-Profil, und zwar im Texteditor statt an der Eingabeaufforderung. Committe nie eine `.mcp.json`, in der der Schlüssel ausgeschrieben steht.

Mit BombVaults eigenem Zertifikat (siehe [TLS und Zertifikate](#tls)) startet der Befehl aus der Karte stattdessen `mcp-remote` und zeigt Node.js das heruntergeladene Zertifikat:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

Die einfachen Anführungszeichen halten deine Shell davon ab, die Variable aufzulösen; das erledigt `mcp-remote` selbst. Dieselbe Form funktioniert in `.mcp.json`: Nimm den Eintrag für Claude Desktop weiter unten und lass `BOMBVAULT_MCP_KEY` aus seinem `env` weg, dann kommt der Schlüssel aus deiner Umgebung.

### Claude Desktop {#claude-desktop}

Claude Desktop erreicht BombVault über `mcp-remote`, das Node.js auf dem Rechner braucht. Öffne die Konfigurationsdatei in Claude Desktop über **Einstellungen, Entwickler, Konfiguration bearbeiten** (Settings, Developer, Edit Config). Sie liegt unter Windows in `%APPDATA%\Claude\claude_desktop_config.json` und unter macOS in `~/Library/Application Support/Claude/claude_desktop_config.json`. Trag den Eintrag aus der Karte unter `"mcpServers"` ein, neben die Server, die schon dort stehen, und starte Claude Desktop neu:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://192.168.1.10:3443/mcp", "--header", "X-API-Key:${BOMBVAULT_MCP_KEY}"],
      "env": {
        "BOMBVAULT_MCP_KEY": "<your key>",
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` steht nur für BombVaults eigenes Zertifikat da. Hinter einem Zertifikat, dem dein Rechner schon vertraut, lässt du es weg.
- `--allow-http` kommt nur bei einer schlichten `http://`-Adresse dazu.
- Der Header steht als `X-API-Key:${BOMBVAULT_MCP_KEY}` da, ohne Leerzeichen nach dem Doppelpunkt und mit dem Schlüssel in `env`. `mcp-remote` trennt einen `--header`-Wert auf manchen Systemen am ersten Leerzeichen, und ein Schlüssel hinter einem Leerzeichen ginge verloren.

### Eigene Connectors in Claudes Einstellungen {#custom-connectors}

Connectors, die du in Claudes eigenen Einstellungen hinzufügst (auf claude.ai und in der Connector-Liste von Claude Desktop), werden noch nicht unterstützt. Diese Connectors werden aus der Cloud von Anthropic angesprochen, brauchen also eine öffentliche HTTPS-Adresse, und sie melden sich über OAuth an. Einen festen Schlüssel können sie nicht mitschicken, und BombVault bietet nur feste Schlüssel, keine OAuth-Anmeldung. BombVault dafür ins Internet zu stellen, würde also nichts bringen. Nimm Claude Code, oder Claude Desktop über `mcp-remote` wie oben.

### Andere Clients {#other-clients}

Jeder Client, der Streamable HTTP spricht, funktioniert:

- URL: die Adresse der Web-Oberfläche plus `/mcp`, zum Beispiel `https://192.168.1.10:3443/mcp`.
- Der Schlüssel in `Authorization: Bearer <key>` oder in `X-API-Key: <key>`. Kommen beide, müssen sie denselben Schlüssel tragen.
- `POST` mit `Content-Type: application/json` und `Accept: application/json, text/event-stream`.
- Eine JSON-RPC-Nachricht pro Anfrage; Batches werden abgelehnt.
- Protokollversionen 2026-07-28, 2025-11-25, 2025-06-18 und 2025-03-26.

## TLS und Zertifikate {#tls}

BombVault liefert HTTPS mit einem selbst ausgestellten Zertifikat aus, und das nennt anfangs nur `localhost`, `127.0.0.1` und `::1`. Claude Code und `mcp-remote` lehnen es auf einer LAN-Adresse ab. Die Wege darum herum, in der Reihenfolge, die zu den meisten Unraid-Installationen passt:

1. **Die Adresse in der MCP-Karte eintragen.** Öffnest du die Karte über HTTPS unter einer Adresse, die das Zertifikat nicht nennt, sagt sie das und bietet **Diese Adresse ins Zertifikat eintragen** an. BombVault stellt sein Zertifikat dann mit dieser Adresse neu aus (dein Browser warnt noch einmal, wie beim ersten Mal). Danach klickst du auf **Zertifikat herunterladen**; die Ausschnitte setzen `NODE_EXTRA_CA_CERTS` auf die heruntergeladene Datei, sodass der Client genau diesem Zertifikat vertraut.
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

- Ohne aktiven Schlüssel antwortet `/mcp` mit `404`.
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
- Behandle einen Schlüssel wie ein Passwort. Claude Code und Claude Desktop legen ihn im Klartext in ihrer Konfiguration ab. Nimm auf einem Rechner, dem du weniger traust, lieber einen Schlüssel, der nur lesen darf.

## Was die Box verlässt {#privacy}

Was ein Assistent liest, geht an den KI-Anbieter dahinter: Namen von Elementen, Zeitpläne, der Laufverlauf mit Fehlermeldungen, IDs und Zeiten von Wiederherstellungspunkten, die Namen der Datenbank-Engines und die Größen der Dumps, die laufende Aktivität, Speicherzahlen, Abdeckung und Status. BombVault entfernt Host-Pfade, Repository-Orte, Hostnamen, Zugangsdaten, Hook-Befehle und Schlüssel, bevor etwas hinausgeht.

## Fehlersuche {#troubleshooting}

| Was du siehst | Was es bedeutet |
|---|---|
| `404` | Kein aktiver Schlüssel, oder ein falscher Pfad wie `/api/mcp`. Der Endpunkt ist `/mcp`. |
| `401` | Der Schlüssel fehlt, ist vertippt, widerrufen oder ersetzt. Vielleicht verwirft ein Proxy den `Authorization`-Header (versuch `X-API-Key`). Markiert die Karte den Schlüssel als ungültig, hat sich `APP_KEY` geändert: Ersetze den Schlüssel. |
| `403` | Die Anfrage kam von einer Browserseite mit anderem Origin. Nimm einen Desktop- oder Kommandozeilen-Client. |
| `405` bei GET | Normal. Der Endpunkt nimmt nur `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Der Client ist zu alt für Streamable HTTP. Aktualisiere ihn. |
| `400` "batch requests are not accepted" | Der Client schickt JSON-RPC-Batches. Schick eine Nachricht pro Anfrage. |
| `429` | Zu viele falsche Schlüssel von dieser Adresse, oder mehr als 120 Anfragen pro Minute mit einem Schlüssel. Warte eine Minute und prüf, ob der Assistent in einer Schleife hängt. |
| Fehler mit "certificate", "self-signed" oder "unable to verify" | Der Client vertraut BombVaults Zertifikat nicht. Siehe [TLS und Zertifikate](#tls). |
| `busy` | Ein anderes Backup oder eine Wartungsaufgabe belegt diese Domäne. Versuch es wieder, wenn sie fertig ist. |
| `cooldown` | Dieses Element, diese Domäne oder das Gesamt-Backup wurde vor weniger als 15 Minuten über MCP gestartet. |
| `retention_guard` | Ein weiteres MCP-Backup ließe in einem "die letzten N behalten"-Fenster nur noch Wiederherstellungspunkte aus MCP übrig. Das nächste geplante Backup schafft Platz, oder du startest es in der Web-Oberfläche. |
| `rate_limited` | Der Schlüssel hat seine 12 Starts für diese Stunde verbraucht. |
| `not_permitted` bei einem Start | Der Schlüssel darf nur lesen. Schalte **Backups starten erlauben** in der Karte ein; eine neue Verbindung ist nicht nötig. Bei einem Abbruch heißt es, dass dieser Schlüssel den Lauf nicht gestartet hat. |
| `domain_off` | Diese Backup-Art ist in den Einstellungen ausgeschaltet. |
| `not_found` | BombVault schützt dieses Element nicht. Nimm es zuerst in der Web-Oberfläche auf; MCP legt nie Konfiguration an. |

Setz die Umgebungsvariable `MCPGODEBUG` am Container nicht. Sie ändert das Verhalten der MCP-Bibliothek, und ein fehlerhafter Wert hält BombVault beim Start an, bevor es auch nur eine Log-Zeile schreibt.
