# API und Integrationen

BombVault hat eine kleine HTTP-API für Skripte, Dashboards und Hausautomation. Sie liest dasselbe, was das Dashboard zeigt, und kann ein Backup starten. Alles andere, etwa Wiederherstellungen, das Löschen von Backups und Einstellungen, bleibt in der Weboberfläche.

## Tokens {#tokens}

Jede Anfrage braucht ein API-Token, auch wenn kein Login-Passwort gesetzt ist. Du legst es unter **Einstellungen, Anbindungen, API-Tokens** an:

1. Gib einen Namen ein, der sagt, wo das Token benutzt wird, etwa „Home Assistant“ oder „Uptime Kuma“.
2. Schalte **Backups starten erlauben** ein, wenn das Token Backups starten soll. Ohne diese Erlaubnis kann es nur lesen.
3. Klick auf **Token anlegen**. Das Token wird einmal angezeigt. BombVault behält nur einen Fingerabdruck davon, kopiere es also jetzt.

Schick das Token in einem Header, entweder `Authorization: Bearer <token>` oder `X-API-Key: <token>`. Ein Token beginnt mit `bvapi_`. Es öffnet nur die API: Ein MCP-Schlüssel funktioniert hier nicht, und ein Token funktioniert nicht für MCP.

Jedes Token hat eine Kachel mit seinem Namen, ob es Backups starten darf, den letzten vier Zeichen, wann und von wo es zuletzt benutzt wurde und seinen Aufrufen heute. Auf der Kachel kannst du es umbenennen, ändern, was es darf, ersetzen oder widerrufen. **Protokoll** zeigt die Backups, die es gestartet hat, und seine letzten Aufrufe. Wenn du BombVaults Konfiguration aus einem Backup wiederherstellst, werden alle Tokens widerrufen, weil das Backup Tokens enthalten kann, die du später widerrufen hast.

Ohne Login-Passwort kann jeder, der die Weboberfläche öffnen kann, auch ein Token anlegen. Öffnest du BombVault unter einem öffentlich wirkenden Namen und ist kein Passwort gesetzt, lassen sich von dieser Adresse keine Tokens anlegen. Das ist dieselbe Regel wie bei den [MCP-Schlüsseln](mcp.md#switch-on).

## Endpunkte {#endpoints}

| Route | Was sie liefert oder tut | Token |
|---|---|---|
| `GET /api/v1/health` | Version, Instanzname, ob gerade ein Backup läuft und was dieses Token darf | lesen |
| `GET /api/v1/status` | Schutzstatus je Domäne: letztes erfolgreiches Backup, erwartetes Intervall, Prüfungen, nächste geplante Läufe | lesen |
| `GET /api/v1/activity` | Was gerade läuft, mit Phase und Prozentangabe | lesen |
| `GET /api/v1/items` | Jedes geschützte Element mit Zeitplan, was ein Backup davon stoppt und seinem letzten Backup; `?domain=` für eine Domäne | lesen |
| `GET /api/v1/runs` | Laufverlauf, neueste zuerst; Filter `limit`, `domain`, `item`, `status`, `kind`, `since` | lesen |
| `GET /api/v1/anomalies` | Anomalien mit einer Übersicht über das, was offen ist; Filter `state`, `severity`, `domain`, `limit` | lesen |
| `GET /api/v1/anomalies/{id}` | Eine Anomalie | lesen |
| `GET /api/v1/storage/{domain}` | Größenverlauf, Wachstum pro Woche und freier Platz jedes Repositorys einer Domäne | lesen |
| `POST /api/v1/backups` | Sichert ein Element (`{"domain":"containers","item":"plex"}`) oder eine ganze Domäne (`{"domain":"vms"}`) | starten |
| `POST /api/v1/backups/everything` | Startet das Gesamt-Backup | starten |
| `POST /api/v1/runs/{id}/cancel` | Bricht ein laufendes Backup ab, das dieses Token gestartet hat | starten |

Die Domänen heißen `containers`, `vms`, `files`, `zfs`, `flash` und `config`. Zeiten sind Unix-Sekunden. Die Antworten sind dieselben wie die der gleichnamigen [MCP-Werkzeuge](mcp.md#tools), so bleiben beide gleich.

Ein gestartetes Backup ist dasselbe Backup, das die Weboberfläche startet: Ein laufender Container wird bis zum Ende seines Backups gestoppt. Die Anfrage kommt sofort zurück, und `/api/v1/activity` und `/api/v1/runs` zeigen, wie es läuft.

## Beispiele {#examples}

```sh
# Wie stehen die Backups?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Einen Container jetzt sichern.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

Mit BombVaults eigenem selbstsigniertem Zertifikat hängst du `--cacert bombvault-cert.pem` an (die Datei bekommst du über **Zertifikat herunterladen** auf der MCP-Karte) oder in einem Netz, dem du traust, `-k`.

## Fehler und Grenzen {#errors}

Ein Fehler kommt als `{"error": {"code": "...", "message": "..."}}` mit passendem Status zurück:

| Status | Codes | Bedeutung |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Ein Argument fehlt oder ist falsch |
| 401 | `no_token`, `invalid_token` | Kein Token oder keins, das aktiv ist |
| 403 | `not_permitted`, `forbidden_origin` | Das Token darf nur lesen, oder es hat den Lauf nicht gestartet, oder die Anfrage kam von einer Seite mit anderem Ursprung |
| 404 | `not_found` | Kein solches Element, kein solcher Lauf, keine solche Anomalie |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Es läuft schon etwas, die Domäne ist aus, oder es gibt nichts zu tun |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Eine Grenze hält die Anfrage zurück; `Retry-After` sagt, wann es wieder geht |

Starts folgen denselben Grenzen wie [Starts über MCP](mcp.md#starting-backups): 12 pro Stunde und Token, 15 Minuten zwischen zwei Starts desselben Elements, höchstens 4 Starts eines Elements in 24 Stunden und der Aufbewahrungsschutz. Die letzten drei zählen Starts über MCP, die API und Home Assistant gemeinsam. Ein Token darf 120 Anfragen pro Minute stellen. Nach fünf Fehlversuchen von einer Adresse ist sie für eine Minute gesperrt.

## OpenAPI {#openapi}

BombVault liefert unter `/api/v1/openapi.json` eine Beschreibung dieser Routen (OpenAPI 3.1). Dafür braucht es kein Token. Lade sie in Swagger UI, Postman oder einen Codegenerator.

## Home Assistant {#home-assistant}

BombVault kann in Home Assistant als Gerät erscheinen, über MQTT-Discovery. Home Assistant braucht dafür seine MQTT-Integration und einen Broker, etwa das Mosquitto-Add-on. Eine eigene Komponente gibt es nicht.

1. Öffne in BombVault **Einstellungen, Anbindungen, Home Assistant**.
2. Trag Adresse und Port des Brokers ein, dazu Benutzername und Passwort, falls er danach fragt. Schalte **TLS verwenden** ein, wenn der Broker TLS spricht, meist auf Port 8883; sein Zertifikat muss für die eingetragene Adresse gültig sein. Änderst du Adresse, Port oder Benutzernamen, gib das Passwort neu ein: BombVault gibt das gespeicherte nicht an einen anderen Broker oder Benutzer weiter.
3. Schalte **Mit Home Assistant verbinden** ein und klick auf **Speichern**. Die Karte zeigt, sobald die Verbindung steht.

Das Gerät heißt BombVault, oder BombVault mit dem Instanznamen in Klammern, und hat diese Entitäten:

| Entität | Was sie zeigt |
|---|---|
| Status | `ok`, `warning`, `failed` oder `off`, das Schlechteste der eingeschalteten Domänen |
| Running job | Was gerade läuft, oder `idle` |
| Open anomalies | Wie viele Anomalien offen sind |
| Next scheduled backup | Wann das nächste geplante Backup startet |
| *Domäne* last backup | Wann das letzte erfolgreiche Backup der Domäne lief |
| *Domäne* last result | Wie ihr letztes Backup ausging |
| *Domäne* repository free space | Freier Platz dort, wo ihr primäres Repository liegt, wenn BombVault ihn lesen kann |
| Back up *domäne* | Ein Knopf, der die ganze Domäne sichert |

Die Namen der Entitäten sind englisch, weil Home Assistant sie so übernimmt, wie BombVault sie schickt. Jede eingeschaltete Domäne bekommt eigene Entitäten, eine ausgeschaltete verliert sie. Die Knöpfe erscheinen, sobald du **Knöpfe starten Backups** einschaltest; bei einer neuen Installation ist das aus. Sie folgen denselben Grenzen wie [Starts über die API](#errors). Dazu nimmt BombVault je Domäne nur einen Druck zur Zeit an und höchstens sechs pro Minute, und einen Druck, den der Broker als Retained-Nachricht aufbewahrt hat, ignoriert es. Jeder, der auf dem Broker veröffentlichen darf, kann sie drücken. Gib dem Broker deshalb ein Passwort.

BombVault liest seinen Zustand alle 15 Sekunden und veröffentlicht ihn, wenn sich etwas geändert hat, als JSON unter `<präfix>/<knoten>/state`. Das Präfix ist `bombvault`, solange du es nicht änderst, und der Knoten ist eine kurze ID, die BombVault einmal wählt. Die Discovery-Nachrichten gehen an das Standardpräfix `homeassistant` von Home Assistant. Beides wird aufbewahrt (retained). Ein Last Will meldet das Gerät als nicht verfügbar, wenn BombVault ohne Abmeldung stoppt. Schaltest du die Verbindung aus, entfernt BombVault das Gerät und seine Entitäten aus Home Assistant.

## BombVault im Netzwerk finden {#mdns}

BombVault kündigt seine Weboberfläche per mDNS im lokalen Netz an, dem Protokoll hinter Bonjour und Avahi. Ein Browser erreicht es dann als `https://bombvault.local:3443`, mit `HTTP_ONLY` als `http://bombvault.local:3000`, und Dienst-Browser führen es als Webdienst mit dem Untertyp `_bombvault`. Die TXT-Einträge tragen Version und Pfad. Der Schalter liegt unter **Einstellungen, Anbindungen, Im Netzwerk finden** und ist ab Werk an. Nutzt ein anderes Gerät den Namen schon, nimmt BombVault `bombvault-2.local` und so weiter, und die Karte zeigt die Adresse, die es bekommen hat. Stoppt BombVault oder schaltest du die Ankündigung aus, meldet es sich im Netz ab, und Browser entfernen den Eintrag sofort.

Ob die Ankündigung dein Netz erreicht, hängt davon ab, wie der Container angebunden ist:

- **bridge**, der Standard in der Unraid-Vorlage: Die Ankündigung bleibt im Docker-Netz, im LAN sieht sie niemand. Öffne BombVault wie bisher über die Adresse des Hosts.
- **br0** oder ein anderes macvlan- oder ipvlan-Netz: Der Container hat eine eigene Adresse im LAN, und die Ankündigung erreicht es.
- **host**: Die Ankündigung geht über die Schnittstellen des Hosts hinaus, neben der von Unraid selbst. Die Bridges von Docker und libvirt lässt BombVault aus.

Angekündigt werden nur IPv4-Adressen.
