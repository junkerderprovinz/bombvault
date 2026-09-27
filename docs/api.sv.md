# API och integrationer

BombVault har ett litet HTTP-API för skript, instrumentpaneler och hemautomation. Det läser samma saker som instrumentpanelen visar och kan starta en säkerhetskopia. Allt annat, som återställningar, borttagning av kopior och inställningar, stannar i webbgränssnittet.

## Token {#tokens}

Varje förfrågan behöver ett API-token, även när inget inloggningslösenord är satt. Skapa ett under **Inställningar, System, API-token**:

1. Skriv ett namn som säger var tokenet används, till exempel "Home Assistant" eller "Uptime Kuma".
2. Slå på **Tillåt att starta säkerhetskopior** om tokenet ska kunna starta kopior. Utan det kan det bara läsa.
3. Klicka på **Skapa token**. Tokenet visas en gång. BombVault sparar bara ett fingeravtryck av det, så kopiera det nu.

Skicka tokenet i ett huvud, antingen `Authorization: Bearer <token>` eller `X-API-Key: <token>`. Ett token börjar med `bvapi_`. Det öppnar bara API:t: en MCP-nyckel fungerar inte här, och ett token fungerar inte för MCP.

Varje token har en ruta med namnet, om det får starta kopior, de fyra sista tecknen, när och varifrån det senast användes och dagens anrop. I rutan kan du byta namn på det, ändra vad det får göra, byta ut det eller återkalla det. **Logg** visar kopiorna det startade och de senaste anropen. Återställer du BombVaults konfiguration från en kopia återkallas alla token, eftersom kopian kan innehålla token som du har återkallat sedan dess.

Utan inloggningslösenord kan alla som kan öppna webbgränssnittet också skapa ett token. Öppnar du BombVault under ett namn som ser offentligt ut och inget lösenord är satt, går det inte att skapa token från den adressen, samma regel som för [MCP-nycklarna](mcp.md#switch-on).

## Slutpunkter {#endpoints}

| Rutt | Vad den returnerar eller gör | Token |
|---|---|---|
| `GET /api/v1/health` | Version, instansnamn, om en kopia körs och vad det här tokenet får göra | läsa |
| `GET /api/v1/status` | Skyddsstatus per område: senaste lyckade kopia, förväntat intervall, kontroller, nästa schemalagda körningar | läsa |
| `GET /api/v1/activity` | Vad som körs just nu, med fas och procent | läsa |
| `GET /api/v1/items` | Varje skyddat objekt med schema, vad en kopia stoppar och senaste kopia; `?domain=` för ett område | läsa |
| `GET /api/v1/runs` | Körhistorik, nyast först; filter `limit`, `domain`, `item`, `status`, `kind`, `since` | läsa |
| `GET /api/v1/anomalies` | Avvikelser med en sammanfattning av de öppna; filter `state`, `severity`, `domain`, `limit` | läsa |
| `GET /api/v1/anomalies/{id}` | En avvikelse | läsa |
| `GET /api/v1/storage/{domain}` | Storlekshistorik, tillväxt per vecka och ledigt utrymme för varje repository i ett område | läsa |
| `POST /api/v1/backups` | Säkerhetskopierar ett objekt (`{"domain":"containers","item":"plex"}`) eller ett helt område (`{"domain":"vms"}`) | starta |
| `POST /api/v1/backups/everything` | Kör Backup Everything | starta |
| `POST /api/v1/runs/{id}/cancel` | Avbryter en pågående kopia som det här tokenet startade | starta |

Områdena heter `containers`, `vms`, `files`, `zfs`, `flash` och `config`. Tider är Unix-sekunder. Svaren är desamma som från [MCP-verktygen](mcp.md#tools) med samma namn, så de två hänger ihop.

En kopia som startas här är samma kopia som webbgränssnittet startar: en körande container stoppas tills kopian är klar. Förfrågan svarar direkt, och `/api/v1/activity` och `/api/v1/runs` visar hur det går.

## Exempel {#examples}

```sh
# Hur går det för säkerhetskopiorna?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Säkerhetskopiera en container nu.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

Med BombVaults eget självsignerade certifikat lägger du till `--cacert bombvault-cert.pem` (filen från **Ladda ner certifikat** på MCP-kortet) eller `-k` på ett nätverk du litar på.

## Fel och gränser {#errors}

Ett fel kommer tillbaka som `{"error": {"code": "...", "message": "..."}}` med motsvarande status:

| Status | Koder | Betydelse |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Ett argument saknas eller är fel |
| 401 | `no_token`, `invalid_token` | Inget token, eller inget aktivt |
| 403 | `not_permitted` | Tokenet får bara läsa, eller det startade inte körningen |
| 404 | `not_found` | Inget sådant objekt, körning eller avvikelse |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Något annat körs, området är avstängt eller det finns inget att göra |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | En gräns håller tillbaka förfrågan; `Retry-After` säger när du kan försöka igen |

Starter följer samma gränser som [starter via MCP](mcp.md#starting-backups): 12 i timmen per token, 15 minuter mellan två starter av samma objekt, högst 4 starter av ett objekt på 24 timmar och lagringsskyddet. De tre sista räknar starter via MCP och via API:t tillsammans. Ett token får göra 120 förfrågningar i minuten. Fem misslyckade försök från en adress spärrar den i en minut.

## OpenAPI {#openapi}

BombVault levererar en beskrivning av de här rutterna på `/api/v1/openapi.json` (OpenAPI 3.1). Den kräver inget token. Läs in den i Swagger UI, Postman eller en kodgenerator.
