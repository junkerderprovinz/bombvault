# API en integraties

BombVault heeft een kleine HTTP-API voor scripts, dashboards en domotica. Die leest hetzelfde als wat het dashboard toont en kan een back-up starten. Al het andere, zoals herstellen, back-ups verwijderen en instellingen, blijft in de webinterface.

## Tokens {#tokens}

Elke aanvraag heeft een API-token nodig, ook als er geen inlogwachtwoord is. Maak er een aan onder **Instellingen, Systeem, API-tokens**:

1. Typ een naam die zegt waar het token gebruikt wordt, bijvoorbeeld "Home Assistant" of "Uptime Kuma".
2. Zet **Back-ups laten starten** aan als het token back-ups moet kunnen starten. Anders kan het alleen lezen.
3. Klik op **Token aanmaken**. Het token wordt één keer getoond. BombVault bewaart er alleen een vingerafdruk van, dus kopieer het nu.

Stuur het token in een header, `Authorization: Bearer <token>` of `X-API-Key: <token>`. Een token begint met `bvapi_`. Het opent alleen de API: een MCP-sleutel werkt hier niet, en een token werkt niet voor MCP.

Elk token heeft een tegel met zijn naam, of het back-ups mag starten, de laatste vier tekens, wanneer en vanwaar het het laatst gebruikt is en zijn aanroepen van vandaag. Op de tegel kun je het hernoemen, wijzigen wat het mag, vervangen of intrekken. **Logboek** toont de back-ups die het startte en zijn laatste aanroepen. Als je de configuratie van BombVault uit een back-up herstelt, worden alle tokens ingetrokken, want de back-up kan tokens bevatten die je later hebt ingetrokken.

Zonder inlogwachtwoord kan iedereen die de webinterface kan openen ook een token aanmaken. Open je BombVault onder een naam die publiek lijkt en is er geen wachtwoord, dan kunnen er vanaf dat adres geen tokens worden aangemaakt, net als bij de [MCP-sleutels](mcp.md#switch-on).

## Eindpunten {#endpoints}

| Route | Wat die teruggeeft of doet | Token |
|---|---|---|
| `GET /api/v1/health` | Versie, instantienaam, of er een back-up loopt en wat dit token mag | lezen |
| `GET /api/v1/status` | Beschermingsstatus per domein: laatste geslaagde back-up, verwacht interval, controles, volgende geplande runs | lezen |
| `GET /api/v1/activity` | Wat er nu loopt, met fase en percentage | lezen |
| `GET /api/v1/items` | Elk beschermd item met planning, wat een back-up stillegt en de laatste back-up; `?domain=` voor één domein | lezen |
| `GET /api/v1/runs` | Uitvoeringsgeschiedenis, nieuwste eerst; filters `limit`, `domain`, `item`, `status`, `kind`, `since` | lezen |
| `GET /api/v1/anomalies` | Anomalieën met een overzicht van wat open staat; filters `state`, `severity`, `domain`, `limit` | lezen |
| `GET /api/v1/anomalies/{id}` | Eén anomalie | lezen |
| `GET /api/v1/storage/{domain}` | Groottegeschiedenis, groei per week en vrije ruimte van elke repository van een domein | lezen |
| `POST /api/v1/backups` | Maakt een back-up van één item (`{"domain":"containers","item":"plex"}`) of een heel domein (`{"domain":"vms"}`) | starten |
| `POST /api/v1/backups/everything` | Start Backup Everything | starten |
| `POST /api/v1/runs/{id}/cancel` | Breekt een lopende back-up af die dit token startte | starten |

De domeinen zijn `containers`, `vms`, `files`, `zfs`, `flash` en `config`. Tijden zijn Unix-seconden. De antwoorden zijn die van de [MCP-tools](mcp.md#tools) met dezelfde naam, zodat beide gelijk blijven.

Een back-up die hier start is dezelfde als die van de webinterface: een draaiende container wordt stilgelegd tot zijn back-up klaar is. De aanvraag komt meteen terug, en `/api/v1/activity` en `/api/v1/runs` laten zien hoe het gaat.

## Voorbeelden {#examples}

```sh
# Hoe staan de back-ups ervoor?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Nu een back-up van één container maken.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

Met het eigen zelfondertekende certificaat van BombVault voeg je `--cacert bombvault-cert.pem` toe (het bestand dat **Certificaat downloaden** op de MCP-kaart geeft) of `-k` op een netwerk dat je vertrouwt.

## Fouten en grenzen {#errors}

Een fout komt terug als `{"error": {"code": "...", "message": "..."}}` met de bijbehorende status:

| Status | Codes | Betekenis |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Een argument ontbreekt of klopt niet |
| 401 | `no_token`, `invalid_token` | Geen token, of geen actief token |
| 403 | `not_permitted`, `forbidden_origin` | Het token mag alleen lezen, of het heeft die run niet gestart, of het verzoek kwam van een pagina met een andere oorsprong |
| 404 | `not_found` | Zo'n item, run of anomalie bestaat niet |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Er loopt al iets, het domein staat uit, of er is niets te doen |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Een grens houdt de aanvraag tegen; `Retry-After` zegt wanneer het weer kan |

Starts volgen dezelfde grenzen als [starts via MCP](mcp.md#starting-backups): 12 per uur per token, 15 minuten tussen twee starts van hetzelfde item, hoogstens 4 starts van een item in 24 uur, en de bewaarbescherming. De laatste drie tellen starts via MCP, via de API en vanuit Home Assistant samen. Een token mag 120 aanvragen per minuut doen. Na vijf mislukte pogingen vanaf een adres is dat een minuut geblokkeerd.

## OpenAPI {#openapi}

BombVault levert een beschrijving van deze routes op `/api/v1/openapi.json` (OpenAPI 3.1). Daar is geen token voor nodig. Laad hem in Swagger UI, Postman of een codegenerator.

## Home Assistant {#home-assistant}

BombVault kan in Home Assistant als apparaat verschijnen, via MQTT-discovery. Home Assistant heeft daarvoor zijn MQTT-integratie en een broker nodig, bijvoorbeeld de Mosquitto-add-on. Een eigen component is niet nodig.

1. Open in BombVault **Instellingen, Systeem, Home Assistant**.
2. Vul het adres en de poort van de broker in, en gebruikersnaam en wachtwoord als hij daarom vraagt. Zet **TLS gebruiken** aan als de broker TLS spreekt, meestal op poort 8883; zijn certificaat moet geldig zijn voor het adres dat je invulde. Verander je het adres, de poort of de gebruikersnaam, voer dan het wachtwoord opnieuw in: BombVault geeft het opgeslagen wachtwoord niet door aan een andere broker of gebruiker.
3. Zet **Verbinden met Home Assistant** aan en klik op **Opslaan**. De kaart laat zien wanneer de verbinding staat.

Het apparaat heet BombVault, of BombVault met de instantienaam tussen haakjes, en heeft deze entiteiten:

| Entiteit | Wat die toont |
|---|---|
| Status | `ok`, `warning`, `failed` of `off`, het slechtste van de ingeschakelde domeinen |
| Running job | Wat er nu loopt, of `idle` |
| Open anomalies | Hoeveel anomalieën open staan |
| Next scheduled backup | Wanneer de volgende geplande back-up start |
| *Domein* last backup | Wanneer de laatste geslaagde back-up van het domein liep |
| *Domein* last result | Hoe zijn laatste back-up afliep |
| *Domein* repository free space | De vrije ruimte waar zijn primaire repository staat, als BombVault die kan lezen |
| Back up *domein* | Een knop die het hele domein back-upt |

De namen van de entiteiten zijn Engels, omdat Home Assistant ze overneemt zoals BombVault ze stuurt. Elk ingeschakeld domein krijgt eigen entiteiten, en een domein dat je uitzet verliest ze. De knoppen verschijnen zodra je **Knoppen starten back-ups** inschakelt; op een nieuwe installatie staat dat uit. Ze volgen dezelfde grenzen als [starts via de API](#errors). Daarnaast neemt BombVault per domein één druk tegelijk aan en hoogstens zes per minuut, en negeert het een druk die de broker als retained bericht heeft bewaard. Iedereen die op de broker mag publiceren, kan erop drukken, dus geef de broker een wachtwoord.

BombVault leest zijn status elke 15 seconden en publiceert die als er iets veranderd is, als JSON onder `<voorvoegsel>/<node>/state`. Het voorvoegsel is `bombvault` zolang je het niet wijzigt, en de node is een korte id die BombVault eenmalig kiest. De discovery-berichten gaan naar het standaardvoorvoegsel `homeassistant` van Home Assistant. Beide worden bewaard (retained). Een last will markeert het apparaat als niet beschikbaar als BombVault stopt zonder zich af te melden. Zet je de koppeling uit, dan verwijdert BombVault het apparaat en zijn entiteiten uit Home Assistant.

## BombVault vinden op het netwerk {#mdns}

BombVault maakt zijn webinterface via mDNS bekend op het lokale netwerk, het protocol achter Bonjour en Avahi. Een browser bereikt het dan als `https://bombvault.local:3443`, of `http://bombvault.local:3000` met `HTTP_ONLY`, en servicebrowsers tonen het als webdienst met het subtype `_bombvault`. De TXT-records bevatten de versie en het pad. De schakelaar staat onder **Instellingen, Systeem, Vinden op het netwerk** en staat standaard aan. Gebruikt een ander apparaat de naam al, dan neemt BombVault `bombvault-2.local` enzovoort, en de kaart toont het adres dat het kreeg. Stopt BombVault of zet je de aankondiging uit, dan meldt het zich af op het netwerk en halen browsers de vermelding meteen weg.

Of de aankondiging je netwerk bereikt, hangt af van hoe de container verbonden is:

- **bridge**, de standaard in de Unraid-template: de aankondiging blijft binnen het Docker-netwerk en niemand op het LAN ziet haar. Open BombVault zoals voorheen via het adres van de host.
- **br0** of een ander macvlan- of ipvlan-netwerk: de container heeft een eigen adres op het LAN en de aankondiging bereikt het.
- **host**: de aankondiging gaat via de interfaces van de host naar buiten, naast die van Unraid zelf. De bridges van Docker en libvirt blijven erbuiten.

Alleen IPv4-adressen worden bekendgemaakt.
