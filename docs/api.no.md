# API og integrasjoner

BombVault har et lite HTTP-API for skript, dashbord og hjemmeautomasjon. Det leser det samme som dashbordet viser, og kan starte en sikkerhetskopi. Alt annet, som gjenoppretting, sletting av kopier og innstillinger, blir i nettgrensesnittet.

## Tokener {#tokens}

Hver forespørsel trenger et API-token, også når det ikke er satt noe innloggingspassord. Lag et under **Innstillinger, System, API-tokener**:

1. Skriv et navn som sier hvor tokenet brukes, for eksempel «Home Assistant» eller «Uptime Kuma».
2. Slå på **Tillat å starte sikkerhetskopier** hvis tokenet skal kunne starte kopier. Uten det kan det bare lese.
3. Klikk **Lag token**. Tokenet vises én gang. BombVault tar bare vare på et fingeravtrykk av det, så kopier det nå.

Send tokenet i en header, enten `Authorization: Bearer <token>` eller `X-API-Key: <token>`. Et token begynner med `bvapi_`. Det åpner bare API-et: en MCP-nøkkel virker ikke her, og et token virker ikke for MCP.

Hvert token har en flis med navnet, om det kan starte kopier, de fire siste tegnene, når og hvorfra det sist ble brukt, og dagens kall. På flisen kan du gi det nytt navn, endre hva det får gjøre, bytte det eller tilbakekalle det. **Logg** viser kopiene det startet og de siste kallene. Gjenoppretter du BombVaults konfigurasjon fra en kopi, blir alle tokener tilbakekalt, fordi kopien kan inneholde tokener du har tilbakekalt senere.

Uten innloggingspassord kan alle som kan åpne nettgrensesnittet, også lage et token. Åpner du BombVault under et navn som ser offentlig ut, og det ikke er satt noe passord, kan det ikke lages tokener fra den adressen, samme regel som for [MCP-nøklene](mcp.md#switch-on).

## Endepunkter {#endpoints}

| Rute | Hva den returnerer eller gjør | Token |
|---|---|---|
| `GET /api/v1/health` | Versjon, instansnavn, om en kopi kjører, og hva dette tokenet får gjøre | lese |
| `GET /api/v1/status` | Beskyttelsesstatus per område: siste vellykkede kopi, forventet intervall, kontroller, neste planlagte kjøringer | lese |
| `GET /api/v1/activity` | Hva som kjører akkurat nå, med fase og prosent | lese |
| `GET /api/v1/items` | Hvert beskyttet element med tidsplan, hva en kopi stopper og siste kopi; `?domain=` for ett område | lese |
| `GET /api/v1/runs` | Kjørehistorikk, nyeste først; filtre `limit`, `domain`, `item`, `status`, `kind`, `since` | lese |
| `GET /api/v1/anomalies` | Anomalier med en oversikt over de åpne; filtre `state`, `severity`, `domain`, `limit` | lese |
| `GET /api/v1/anomalies/{id}` | Én anomali | lese |
| `GET /api/v1/storage/{domain}` | Størrelseshistorikk, vekst per uke og ledig plass for hvert repository i et område | lese |
| `POST /api/v1/backups` | Tar kopi av ett element (`{"domain":"containers","item":"plex"}`) eller et helt område (`{"domain":"vms"}`) | starte |
| `POST /api/v1/backups/everything` | Kjører Backup Everything | starte |
| `POST /api/v1/runs/{id}/cancel` | Avbryter en kjørende kopi som dette tokenet startet | starte |

Områdene heter `containers`, `vms`, `files`, `zfs`, `flash` og `config`. Tider er Unix-sekunder. Svarene er de samme som fra [MCP-verktøyene](mcp.md#tools) med samme navn, så de to følger hverandre.

En kopi som startes her, er den samme som nettgrensesnittet starter: en container som kjører, stoppes til kopien er ferdig. Forespørselen svarer med en gang, og `/api/v1/activity` og `/api/v1/runs` viser hvordan det går.

## Eksempler {#examples}

```sh
# Hvordan står det til med sikkerhetskopiene?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Ta kopi av én container nå.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

Med BombVaults eget selvsignerte sertifikat legger du til `--cacert bombvault-cert.pem` (filen fra **Last ned sertifikat** på MCP-kortet) eller `-k` på et nettverk du stoler på.

## Feil og grenser {#errors}

En feil kommer tilbake som `{"error": {"code": "...", "message": "..."}}` med tilhørende status:

| Status | Koder | Betydning |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Et argument mangler eller er feil |
| 401 | `no_token`, `invalid_token` | Intet token, eller ikke et aktivt |
| 403 | `not_permitted` | Tokenet kan bare lese, eller det startet ikke kjøringen |
| 404 | `not_found` | Ikke noe slikt element, kjøring eller anomali |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Noe annet kjører, området er slått av, eller det er ingenting å gjøre |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | En grense holder forespørselen tilbake; `Retry-After` sier når du kan prøve igjen |

Starter følger de samme grensene som [starter via MCP](mcp.md#starting-backups): 12 i timen per token, 15 minutter mellom to starter av samme element, høyst 4 starter av ett element på 24 timer, og oppbevaringsvernet. De tre siste teller starter via MCP, via API-et og fra Home Assistant sammen. Et token kan gjøre 120 forespørsler i minuttet. Fem mislykkede forsøk fra én adresse sperrer den i ett minutt.

## OpenAPI {#openapi}

BombVault leverer en beskrivelse av disse rutene på `/api/v1/openapi.json` (OpenAPI 3.1). Den krever ikke noe token. Last den inn i Swagger UI, Postman eller en kodegenerator.

## Home Assistant {#home-assistant}

BombVault kan dukke opp i Home Assistant som en enhet, via MQTT-discovery. Home Assistant trenger MQTT-integrasjonen sin og en megler, for eksempel Mosquitto-tillegget. Ingen egen komponent trengs.

1. Åpne **Innstillinger, System, Home Assistant** i BombVault.
2. Skriv inn adressen og porten til megleren, og brukernavn og passord hvis den ber om det. Slå på **Bruk TLS** hvis megleren bruker TLS, som regel på port 8883; sertifikatet må være gyldig for adressen du skrev inn.
3. Slå på **Koble til Home Assistant** og klikk **Lagre**. Kortet viser når tilkoblingen er oppe.

Enheten heter BombVault, eller BombVault med instansnavnet i parentes, og har disse entitetene:

| Entitet | Hva den viser |
|---|---|
| Status | `ok`, `warning`, `failed` eller `off`, det verste av områdene som er slått på |
| Running job | Hva som kjører nå, eller `idle` |
| Open anomalies | Hvor mange anomalier som er åpne |
| Next scheduled backup | Når neste planlagte kopi starter |
| *Område* last backup | Når den siste vellykkede kopien av området kjørte |
| *Område* last result | Hvordan den siste kopien endte |
| *Område* repository free space | Ledig plass der det primære repositoryet ligger, hvis BombVault kan lese den |
| Back up *område* | En knapp som tar kopi av hele området |

Navnene på entitetene er på engelsk, fordi Home Assistant tar dem slik BombVault sender dem. Hvert område som er slått på, får egne entiteter, og et område du slår av, mister dem. Knappene følger de samme grensene som [starter via API-et](#errors). Alle som kan publisere på megleren, kan trykke på dem, så gi megleren et passord eller slå av **Knapper starter sikkerhetskopier**.

BombVault leser statusen sin hvert 15. sekund og publiserer den når noe har endret seg, som JSON under `<prefiks>/<node>/state`. Prefikset er `bombvault` så lenge du ikke endrer det, og noden er en kort id BombVault velger én gang. Discovery-meldingene går til standardprefikset `homeassistant` i Home Assistant. Begge beholdes (retained). En last will merker enheten som utilgjengelig hvis BombVault stopper uten å si fra. Slår du forbindelsen av, fjerner BombVault enheten og entitetene fra Home Assistant.

## Finn BombVault på nettverket {#mdns}

BombVault kunngjør nettgrensesnittet sitt på det lokale nettverket via mDNS, protokollen bak Bonjour og Avahi. En nettleser når den da som `https://bombvault.local:3443`, eller `http://bombvault.local:3000` med `HTTP_ONLY`, og tjenestelesere viser den som nettjeneste med undertypen `_bombvault`. TXT-postene inneholder versjonen og stien. Bryteren ligger under **Innstillinger, System, Finn på nettverket** og er slått på fra start. Bruker en annen enhet allerede navnet, tar BombVault `bombvault-2.local` og så videre, og kortet viser adressen den fikk. Når BombVault stopper eller du slår kunngjøringen av, sier den fra på nettverket, og nettlesere fjerner oppføringen med en gang.

Om kunngjøringen når nettverket ditt, avhenger av hvordan containeren er koblet til:

- **bridge**, standarden i Unraid-malen: kunngjøringen blir i Dockers nettverk, og ingen på LAN-et ser den. Åpne BombVault på vertens adresse som før.
- **br0** eller et annet macvlan- eller ipvlan-nettverk: containeren har sin egen adresse på LAN-et, og kunngjøringen når det.
- **host**: kunngjøringen går ut over vertens grensesnitt, ved siden av Unraids egen.

Bare IPv4-adresser kunngjøres.
