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

Starter følger de samme grensene som [starter via MCP](mcp.md#starting-backups): 12 i timen per token, 15 minutter mellom to starter av samme element, høyst 4 starter av ett element på 24 timer, og oppbevaringsvernet. De tre siste teller starter via MCP og via API-et sammen. Et token kan gjøre 120 forespørsler i minuttet. Fem mislykkede forsøk fra én adresse sperrer den i ett minutt.

## OpenAPI {#openapi}

BombVault leverer en beskrivelse av disse rutene på `/api/v1/openapi.json` (OpenAPI 3.1). Den krever ikke noe token. Last den inn i Swagger UI, Postman eller en kodegenerator.
