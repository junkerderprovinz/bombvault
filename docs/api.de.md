# API und Integrationen

BombVault hat eine kleine HTTP-API für Skripte, Dashboards und Hausautomation. Sie liest dieselben Dinge, die die Übersicht zeigt, und kann eine Sicherung starten. Alles andere, etwa Wiederherstellungen, das Löschen von Sicherungen und Einstellungen, bleibt in der Weboberfläche.

## Tokens {#tokens}

Jede Anfrage braucht ein API-Token, auch wenn kein Login-Passwort gesetzt ist. Lege eines unter **Einstellungen, Anbindungen, API-Tokens** an:

1. Tippe einen Namen ein, der sagt, wo das Token genutzt wird, zum Beispiel „Home Assistant“ oder „Uptime Kuma“.
2. Schalte **Sicherungen starten erlauben** ein, wenn das Token Sicherungen starten können soll. Ohne das kann das Token nur lesen.
3. Klicke auf **Token anlegen**. Das Token wird einmal angezeigt. BombVault behält nur einen Fingerabdruck davon, kopiere es also jetzt.

Sende das Token in einem Header, entweder `Authorization: Bearer <token>` oder `X-API-Key: <token>`. Ein Token beginnt mit `bvapi_`. Es öffnet nur die API: Ein MCP-Schlüssel funktioniert hier nicht, und ein Token funktioniert nicht für MCP.

Jedes Token hat eine Kachel mit seinem Namen, ob es Sicherungen starten darf, den letzten vier Zeichen, wann und von wo es zuletzt benutzt wurde, und seinen Aufrufen heute. Auf der Kachel kannst du es umbenennen, ändern, was es darf, es ersetzen oder entziehen. Das **Protokoll** zeigt die Sicherungen, die es gestartet hat, und seine letzten Aufrufe. Wird die Konfiguration von BombVault aus einer Sicherung wiederhergestellt, werden alle Tokens entzogen, weil die Sicherung Tokens enthalten kann, die du später entzogen hast.

Ohne Login-Passwort kann jeder, der die Weboberfläche öffnen kann, auch ein Token anlegen. Öffnest du BombVault unter einem öffentlich aussehenden Namen und ist kein Passwort gesetzt, lassen sich von dieser Adresse aus keine Tokens anlegen, dieselbe Regel wie bei [MCP-Schlüsseln](mcp.md#switch-on).

## Endpunkte {#endpoints}

| Route | Was sie zurückgibt oder tut | Token |
|---|---|---|
| `GET /api/v1/health` | Version, Instanzname, ob eine Sicherung läuft, und was dieses Token darf | lesen |
| `GET /api/v1/status` | Schutzstatus pro Domain: letzte erfolgreiche Sicherung, erwartetes Intervall, Prüfungen, nächste geplante Läufe | lesen |
| `GET /api/v1/activity` | Was gerade läuft, mit Phase und Prozentzahl | lesen |
| `GET /api/v1/items` | Jedes geschützte Element mit seinem Zeitplan, was eine Sicherung anhält, und seiner letzten Sicherung. `?domain=` für eine Domain | lesen |
| `GET /api/v1/runs` | Laufverlauf, neueste zuerst. Filter `limit`, `domain`, `item`, `status`, `kind`, `since` | lesen |
| `GET /api/v1/anomalies` | Anomalien mit einer Zusammenfassung dessen, was offen ist. Filter `state`, `severity`, `domain`, `limit` | lesen |
| `GET /api/v1/anomalies/{id}` | Eine Anomalie | lesen |
| `GET /api/v1/storage/{domain}` | Größenverlauf, Wachstum pro Woche und freier Platz jedes Repositorys einer Domain | lesen |
| `POST /api/v1/backups` | Sichert ein Element (`{"domain":"containers","item":"plex"}`) oder eine ganze Domain (`{"domain":"vms"}`) | starten |
| `POST /api/v1/backups/everything` | Führt den Durchgang Alles sichern aus | starten |
| `POST /api/v1/runs/{id}/cancel` | Bricht eine laufende Sicherung ab, die dieses Token gestartet hat | starten |

Die Domains sind `containers`, `vms`, `files`, `zfs`, `flash` und `config`. Zeiten sind Unix-Sekunden. Die Antworten sind dieselben wie die der [MCP-Tools](mcp.md#tools) mit denselben Namen, sodass beide im Gleichschritt bleiben.

Eine gestartete Sicherung ist dieselbe Sicherung, die die Weboberfläche startet: Ein laufender Container wird angehalten, bis seine Sicherung fertig ist. Die Anfrage kehrt sofort zurück, und `/api/v1/activity` und `/api/v1/runs` zeigen, wie es läuft.

## Beispiele {#examples}

```sh
# How are the backups doing?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Back up one container now.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

Füge beim eigenen selbstsignierten Zertifikat von BombVault `--cacert bombvault-cert.pem` hinzu (die Datei, die dir **Zertifikat herunterladen** auf der MCP-Karte gibt) oder in einem Netzwerk, dem du vertraust, `-k`.

## Fehler und Limits {#errors}

Ein Fehler kommt als `{"error": {"code": "...", "message": "..."}}` mit einem passenden Status zurück:

| Status | Codes | Bedeutung |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Ein Argument fehlt oder ist falsch |
| 401 | `no_token`, `invalid_token` | Kein Token, oder keines, das aktiv ist |
| 403 | `not_permitted`, `forbidden_origin` | Das Token darf nur lesen, der Lauf wurde nicht von ihm gestartet, oder die Anfrage kam von einer Seite auf einem anderen Origin |
| 404 | `not_found` | Kein solches Element, kein solcher Lauf, keine solche Anomalie |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Etwas anderes läuft, die Domain ist aus, oder es gibt nichts zu tun |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Ein Limit hält die Anfrage zurück. `Retry-After` sagt, wann du es wieder versuchen kannst |

Starts folgen denselben Limits wie [Starts über MCP](mcp.md#starting-backups): 12 pro Stunde und Token, 15 Minuten zwischen zwei Starts desselben Elements, höchstens 4 Starts eines Elements in 24 Stunden und der Aufbewahrungsschutz. Die letzten drei zählen Starts über MCP, die API und Home Assistant zusammen. Ein Token kann 120 Anfragen pro Minute stellen. Fünf fehlgeschlagene Versuche von einer Adresse sperren sie für eine Minute.

## OpenAPI {#openapi}

BombVault liefert eine Beschreibung dieser Routen unter `/api/v1/openapi.json` (OpenAPI 3.1). Sie braucht kein Token. Lade sie in Swagger UI, Postman oder einen Code-Generator.

## Home Assistant {#home-assistant}

BombVault kann in Home Assistant über MQTT Discovery als Gerät erscheinen. Home Assistant braucht dafür seine MQTT-Integration und einen Broker, zum Beispiel das Mosquitto-Add-on. Eine eigene Komponente ist nicht nötig.

1. Öffne in BombVault **Einstellungen, Anbindungen** und schalte in der Karte **Dienste** **Home Assistant** ein. Die Felder für den Broker erscheinen darunter.
2. Gib die Adresse und den Port des Brokers ein, dazu Benutzername und Passwort, wenn er danach fragt. Schalte **TLS verwenden** ein, wenn der Broker TLS annimmt, meist auf Port 8883. Sein Zertifikat muss für die eingegebene Adresse gültig sein. Wenn du die Adresse, den Port oder den Benutzernamen änderst, gib das Passwort neu ein: BombVault gibt das gespeicherte nicht an einen anderen Broker oder Benutzer weiter.
3. Klicke auf **Speichern**. Die Zeile zeigt, wann die Verbindung steht.

Das Gerät heißt BombVault oder BombVault mit dem Instanznamen in Klammern und hat diese Entitäten:

| Entität | Was sie zeigt |
|---|---|
| Status | `ok`, `warning`, `failed` oder `off`, der schlechteste der eingeschalteten Domains |
| Running job | Was gerade läuft, oder `idle` |
| Open anomalies | Wie viele Anomalien offen sind |
| Next scheduled backup | Wann die nächste geplante Sicherung startet |
| *Domain* last backup | Wann die letzte erfolgreiche Sicherung der Domain lief |
| *Domain* last result | Wie ihre letzte Sicherung endete |
| *Domain* repository free space | Freier Platz dort, wo ihr primäres Repository liegt, wenn BombVault ihn lesen kann |
| Back up *domain* | Ein Knopf, der die ganze Domain sichert |

Jede eingeschaltete Domain bekommt eigene Entitäten, und eine Domain, die du ausschaltest, verliert sie. Die Knöpfe erscheinen, sobald du **Knöpfe starten Sicherungen** einschaltest. Eine Neuinstallation hat es aus. Sie folgen denselben Limits wie [Starts über die API](#errors). Dazu bearbeitet BombVault pro Domain jeweils einen Druck auf einmal und nicht mehr als sechs pro Minute und ignoriert einen Druck, den der Broker als zurückbehaltene Nachricht aufbewahrt hat. Wer an den Broker veröffentlichen kann, kann sie drücken, gib dem Broker also ein Passwort.

BombVault liest seinen Zustand alle 15 Sekunden und veröffentlicht ihn, wenn sich etwas geändert hat, als JSON unter `<prefix>/<node>/state`. Das Präfix ist `bombvault`, solange du es nicht änderst, und der Knoten ist eine kurze ID, die BombVault einmal wählt. Die Discovery-Nachrichten gehen an das Standardpräfix `homeassistant` von Home Assistant. Beide sind retained. Ein Last Will markiert das Gerät als nicht verfügbar, wenn BombVault ohne Abmeldung stoppt. Das Ausschalten der Verbindung entfernt das Gerät und seine Entitäten aus Home Assistant.

### Beispiel: eine Karte und ein Alarm {#home-assistant-example}

Home Assistant bildet die Entitäts-IDs aus den Namen, das Gerät von BombVault bekommt also IDs wie `sensor.bombvault_status`. Steht ein Instanzname in Klammern, tragen die IDs auch den, prüfe sie also zuerst unter **Developer Tools, States**. Eine Karte fürs Dashboard:

```yaml
type: entities
title: Backups
entities:
  - sensor.bombvault_status
  - sensor.bombvault_running_job
  - sensor.bombvault_open_anomalies
  - sensor.bombvault_next_scheduled_backup
```

Eine Automatisierung, die dein Handy benachrichtigt, wenn der Status auf `failed` springt:

```yaml
alias: BombVault backup failed
trigger:
  - platform: state
    entity_id: sensor.bombvault_status
    to: failed
action:
  - service: notify.mobile_app_my_phone
    data:
      message: A BombVault backup failed. Open Needs you on the overview.
```

## Prometheus {#metrics}

**/metrics bereitstellen** in der Karte **Dienste** unter **Einstellungen, Anbindungen** liefert Sicherungsstatus, Größen und Zeitstempel für Prometheus, Grafana oder Uptime Kuma, unter `/metrics` auf dem Port der Weboberfläche. Es ist standardmäßig aus. **Scrape-Token (optional)** lässt den Scraper ein Bearer-Token senden. Die Labels tragen keine Geheimnisse und keine Pfade.

## Dashboard-Widget {#dashboard-widget}

Die Karte **Dashboard-Widget** unter **Einstellungen, Anbindungen** erzeugt ein schreibgeschütztes Token und gibt dir eine Adresse für jedes Dashboard, das einen iframe zeigt, etwa Homepage, Organizr oder Heimdall. Die Adresse öffnet eine kleine Seite unter `/widget` mit nur dem Live-Aktivitätsprotokoll. Das Token gewährt dieses Protokoll und sonst nichts, und **Deaktivieren** entzieht es sofort. Die eingebettete Seite gibt es nur auf Englisch.

## BombVault im Netzwerk finden {#mdns}

BombVault gibt seine Weboberfläche im lokalen Netz über mDNS bekannt, das Protokoll hinter Bonjour und Avahi. Ein Browser erreicht es dann als `https://bombvault.local:3443`, oder `http://bombvault.local:3000` mit `HTTP_ONLY`, und Dienst-Browser listen es als Webdienst mit dem Subtyp `_bombvault`. Seine TXT-Einträge tragen die Version und den Pfad. Der Schalter ist **Im Netzwerk finden** in der Karte **Dienste** unter **Einstellungen, Anbindungen** und standardmäßig an. Nutzt schon ein anderes Gerät den Namen, nimmt BombVault `bombvault-2.local` und so weiter, und die Karte zeigt die Adresse, die es bekam. Stoppt BombVault oder schaltest du die Bekanntgabe aus, sagt es dem Netz Bescheid, sodass Browser den Eintrag sofort verwerfen.

Ob die Bekanntgabe dein Netzwerk erreicht, hängt davon ab, wie der Container angebunden ist:

- **bridge**, die Vorgabe im Unraid-Template: Die Bekanntgabe bleibt im Netzwerk von Docker, und niemand im LAN sieht sie. Öffne BombVault wie bisher über die Adresse des Hosts.
- **br0** oder ein anderes macvlan- oder ipvlan-Netz: Der Container hat eine eigene Adresse im LAN, und die Bekanntgabe erreicht es.
- **host**: Die Bekanntgabe geht über die Schnittstellen des Hosts hinaus, neben der von Unraid selbst. Die Bridges von Docker und libvirt lässt sie aus.

Es werden nur IPv4-Adressen bekanntgegeben.
