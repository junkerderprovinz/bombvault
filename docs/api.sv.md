# API och integrationer

BombVault har ett litet HTTP-API för skript, instrumentpaneler och hemautomation. Det läser samma saker som instrumentpanelen visar och kan starta en säkerhetskopia. Allt annat, som återställningar, borttagning av kopior och inställningar, stannar i webbgränssnittet.

## Token {#tokens}

Varje förfrågan behöver ett API-token, även när inget inloggningslösenord är satt. Skapa ett under **Inställningar, Integrationer, API-token**:

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
| 403 | `not_permitted`, `forbidden_origin` | Tokenet får bara läsa, eller det startade inte körningen, eller så kom begäran från en sida med ett annat ursprung |
| 404 | `not_found` | Inget sådant objekt, körning eller avvikelse |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Något annat körs, området är avstängt eller det finns inget att göra |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | En gräns håller tillbaka förfrågan; `Retry-After` säger när du kan försöka igen |

Starter följer samma gränser som [starter via MCP](mcp.md#starting-backups): 12 i timmen per token, 15 minuter mellan två starter av samma objekt, högst 4 starter av ett objekt på 24 timmar och lagringsskyddet. De tre sista räknar starter via MCP, via API:t och från Home Assistant tillsammans. Ett token får göra 120 förfrågningar i minuten. Fem misslyckade försök från en adress spärrar den i en minut.

## OpenAPI {#openapi}

BombVault levererar en beskrivning av de här rutterna på `/api/v1/openapi.json` (OpenAPI 3.1). Den kräver inget token. Läs in den i Swagger UI, Postman eller en kodgenerator.

## Home Assistant {#home-assistant}

BombVault kan dyka upp i Home Assistant som en enhet via MQTT-discovery. Home Assistant behöver sin MQTT-integration och en broker, till exempel tillägget Mosquitto. Ingen egen komponent behövs.

1. Öppna **Inställningar, Integrationer, Home Assistant** i BombVault.
2. Ange brokerns adress och port, och användarnamn och lösenord om den kräver det. Slå på **Använd TLS** om brokern använder TLS, oftast på port 8883; dess certifikat måste gälla för adressen du angav. Om du ändrar adressen, porten eller användarnamnet måste du ange lösenordet igen: BombVault skickar inte det sparade vidare till en annan broker eller användare.
3. Slå på **Anslut till Home Assistant** och klicka på **Spara**. Kortet visar när anslutningen är uppe.

Enheten heter BombVault, eller BombVault med instansnamnet inom parentes, och har de här entiteterna:

| Entitet | Vad den visar |
|---|---|
| Status | `ok`, `warning`, `failed` eller `off`, det sämsta av de påslagna områdena |
| Running job | Vad som körs nu, eller `idle` |
| Open anomalies | Hur många avvikelser som är öppna |
| Next scheduled backup | När nästa schemalagda kopia startar |
| *Område* last backup | När områdets senaste lyckade kopia kördes |
| *Område* last result | Hur dess senaste kopia slutade |
| *Område* repository free space | Ledigt utrymme där dess primära repository ligger, om BombVault kan läsa det |
| Back up *område* | En knapp som säkerhetskopierar hela området |

Entiteternas namn är på engelska, eftersom Home Assistant tar dem som BombVault skickar dem. Varje påslaget område får egna entiteter, och ett område du stänger av förlorar dem. Knapparna dyker upp när du slår på **Knappar startar säkerhetskopior**, som är avslaget i en ny installation. De följer samma gränser som [starter via API:t](#errors). Dessutom tar BombVault emot ett tryck i taget per område och högst sex per minut, och bortser från ett tryck som brokern har sparat som retained-meddelande. Alla som får publicera på brokern kan trycka på dem, så ge brokern ett lösenord.

BombVault läser sin status var 15:e sekund och publicerar den när något har ändrats, som JSON under `<prefix>/<nod>/state`. Prefixet är `bombvault` så länge du inte ändrar det, och noden är ett kort id som BombVault väljer en gång. Discovery-meddelandena går till Home Assistants standardprefix `homeassistant`. Båda behålls (retained). En last will markerar enheten som otillgänglig om BombVault stannar utan att säga till. Stänger du av kopplingen tar BombVault bort enheten och dess entiteter från Home Assistant.

## Hitta BombVault i nätverket {#mdns}

BombVault annonserar sitt webbgränssnitt i det lokala nätverket via mDNS, protokollet bakom Bonjour och Avahi. En webbläsare når det då som `https://bombvault.local:3443`, eller `http://bombvault.local:3000` med `HTTP_ONLY`, och tjänstebläddrare listar det som webbtjänst med undertypen `_bombvault`. TXT-posterna innehåller versionen och sökvägen. Reglaget finns under **Inställningar, Integrationer, Hitta i nätverket** och är på från början. Om en annan enhet redan använder namnet tar BombVault `bombvault-2.local` och så vidare, och kortet visar adressen det fick. När BombVault stannar eller du stänger av annonseringen säger det till nätverket, så att webbläsare tar bort posten direkt.

Om annonseringen når ditt nätverk beror på hur containern är ansluten:

- **bridge**, standard i Unraid-mallen: annonseringen stannar i Dockers nätverk och ingen på LAN:et ser den. Öppna BombVault på värdens adress som tidigare.
- **br0** eller ett annat macvlan- eller ipvlan-nätverk: containern har en egen adress på LAN:et och annonseringen når dit.
- **host**: annonseringen går ut över värdens gränssnitt, bredvid Unraids egen. Docker- och libvirt-bryggorna hoppas över.

Bara IPv4-adresser annonseras.
