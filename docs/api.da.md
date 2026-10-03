# API og integrationer

BombVault har et lille HTTP-API til scripts, dashboards og hjemmeautomatisering. Det læser det samme, som dashboardet viser, og kan starte en sikkerhedskopi. Alt andet, som gendannelser, sletning af kopier og indstillinger, bliver i brugerfladen.

## Tokens {#tokens}

Hver forespørgsel skal have et API-token, også når der ikke er sat en adgangskode. Opret et under **Indstillinger, Integrationer, API-tokens**:

1. Skriv et navn, der siger, hvor tokenet bruges, for eksempel "Home Assistant" eller "Uptime Kuma".
2. Slå **Tillad at starte sikkerhedskopier** til, hvis tokenet skal kunne starte kopier. Uden det kan det kun læse.
3. Klik på **Opret token**. Tokenet vises én gang. BombVault gemmer kun et fingeraftryk af det, så kopiér det nu.

Send tokenet i en header, enten `Authorization: Bearer <token>` eller `X-API-Key: <token>`. Et token starter med `bvapi_`. Det åbner kun API'et: en MCP-nøgle virker ikke her, og et token virker ikke til MCP.

Hvert token har en flise med navnet, om det må starte kopier, de sidste fire tegn, hvornår og hvorfra det sidst blev brugt, og dagens kald. På flisen kan du omdøbe det, ændre hvad det må, udskifte det eller tilbagekalde det. **Log** viser de kopier, det startede, og dets seneste kald. Gendanner du BombVaults konfiguration fra en kopi, tilbagekaldes alle tokens, fordi kopien kan indeholde tokens, du har tilbagekaldt siden.

Uden adgangskode kan alle, der kan åbne brugerfladen, også oprette et token. Åbner du BombVault under et navn, der ser offentligt ud, og er der ingen adgangskode, kan der ikke oprettes tokens fra den adresse, ligesom med [MCP-nøglerne](mcp.md#switch-on).

## Endepunkter {#endpoints}

| Rute | Hvad den returnerer eller gør | Token |
|---|---|---|
| `GET /api/v1/health` | Version, instansnavn, om en kopi kører, og hvad dette token må | læse |
| `GET /api/v1/status` | Beskyttelsesstatus pr. område: seneste vellykkede kopi, forventet interval, kontroller, næste planlagte kørsler | læse |
| `GET /api/v1/activity` | Hvad der kører lige nu, med fase og procent | læse |
| `GET /api/v1/items` | Hvert beskyttet element med tidsplan, hvad en kopi stopper, og seneste kopi; `?domain=` for ét område | læse |
| `GET /api/v1/runs` | Kørselshistorik, nyeste først; filtre `limit`, `domain`, `item`, `status`, `kind`, `since` | læse |
| `GET /api/v1/anomalies` | Anomalier med et overblik over de åbne; filtre `state`, `severity`, `domain`, `limit` | læse |
| `GET /api/v1/anomalies/{id}` | Én anomali | læse |
| `GET /api/v1/storage/{domain}` | Størrelseshistorik, vækst pr. uge og ledig plads for hvert repository i et område | læse |
| `POST /api/v1/backups` | Sikkerhedskopierer ét element (`{"domain":"containers","item":"plex"}`) eller et helt område (`{"domain":"vms"}`) | starte |
| `POST /api/v1/backups/everything` | Kører Fuld sikkerhedskopi | starte |
| `POST /api/v1/runs/{id}/cancel` | Afbryder en kørende kopi, som dette token startede | starte |

Områderne hedder `containers`, `vms`, `files`, `zfs`, `flash` og `config`. Tider er Unix-sekunder. Svarene er de samme som fra [MCP-værktøjerne](mcp.md#tools) med samme navn, så de to følges ad.

En kopi, der startes her, er den samme, som brugerfladen starter: en kørende container stoppes, indtil dens kopi er færdig. Forespørgslen svarer med det samme, og `/api/v1/activity` og `/api/v1/runs` viser, hvordan det går.

## Eksempler {#examples}

```sh
# Hvordan går det med sikkerhedskopierne?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Sikkerhedskopiér én container nu.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

Med BombVaults eget selvsignerede certifikat tilføjer du `--cacert bombvault-cert.pem` (filen fra **Hent certifikat** på MCP-kortet) eller `-k` på et netværk, du stoler på.

## Fejl og grænser {#errors}

En fejl kommer tilbage som `{"error": {"code": "...", "message": "..."}}` med den tilsvarende status:

| Status | Koder | Betydning |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Et argument mangler eller er forkert |
| 401 | `no_token`, `invalid_token` | Intet token, eller ikke et aktivt |
| 403 | `not_permitted`, `forbidden_origin` | Tokenet må kun læse, eller det startede ikke kørslen, eller anmodningen kom fra en side med en anden oprindelse |
| 404 | `not_found` | Intet sådant element, kørsel eller anomali |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Noget andet kører, området er slået fra, eller der er intet at gøre |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | En grænse holder forespørgslen tilbage; `Retry-After` siger, hvornår du kan prøve igen |

Starter følger de samme grænser som [starter via MCP](mcp.md#starting-backups): 12 i timen pr. token, 15 minutter mellem to starter af samme element, højst 4 starter af et element på 24 timer og opbevaringsbeskyttelsen. De sidste tre tæller starter via MCP, via API'et og fra Home Assistant sammen. Et token må lave 120 forespørgsler i minuttet. Fem mislykkede forsøg fra én adresse spærrer den i et minut.

## OpenAPI {#openapi}

BombVault leverer en beskrivelse af disse ruter på `/api/v1/openapi.json` (OpenAPI 3.1). Den kræver intet token. Indlæs den i Swagger UI, Postman eller en kodegenerator.

## Home Assistant {#home-assistant}

BombVault kan vises i Home Assistant som en enhed via MQTT-discovery. Home Assistant skal bruge sin MQTT-integration og en broker, for eksempel Mosquitto-tilføjelsen. Der skal ikke installeres nogen egen komponent.

1. Åbn **Indstillinger, Integrationer, Home Assistant** i BombVault.
2. Skriv brokerens adresse og port, og brugernavn og adgangskode, hvis den beder om dem. Slå **Brug TLS** til, hvis brokeren taler TLS, som regel på port 8883; dens certifikat skal være gyldigt for den adresse, du skrev. Ændrer du adressen, porten eller brugernavnet, skal du indtaste adgangskoden igen: BombVault giver ikke den gemte videre til en anden broker eller bruger.
3. Slå **Forbind til Home Assistant** til, og klik på **Gem**. Kortet viser, når forbindelsen er oppe.

Enheden hedder BombVault, eller BombVault med instansnavnet i parentes, og har disse entiteter:

| Entitet | Hvad den viser |
|---|---|
| Status | `ok`, `warning`, `failed` eller `off`, det værste af de områder, der er slået til |
| Running job | Hvad der kører nu, eller `idle` |
| Open anomalies | Hvor mange anomalier der er åbne |
| Next scheduled backup | Hvornår den næste planlagte kopi starter |
| *Område* last backup | Hvornår områdets seneste vellykkede kopi kørte |
| *Område* last result | Hvordan dets seneste kopi endte |
| *Område* repository free space | Ledig plads der, hvor dets primære repository ligger, hvis BombVault kan læse den |
| Back up *område* | En knap, der sikkerhedskopierer hele området |

Entiteternes navne er på engelsk, fordi Home Assistant overtager dem, som BombVault sender dem. Hvert område, der er slået til, får sine egne entiteter, og et område, du slår fra, mister dem. Knapperne dukker op, når du slår **Knapper starter sikkerhedskopier** til; en ny installation har det slået fra. De følger de samme grænser som [starter via API'et](#errors). Derudover tager BombVault kun imod ét tryk ad gangen pr. område og højst seks i minuttet, og ser bort fra et tryk, som brokeren har gemt som retained-besked. Alle, der kan udgive på brokeren, kan trykke på dem, så giv brokeren en adgangskode.

BombVault læser sin status hvert 15. sekund og udgiver den, når noget har ændret sig, som JSON under `<præfiks>/<node>/state`. Præfikset er `bombvault`, så længe du ikke ændrer det, og noden er et kort id, som BombVault vælger én gang. Discovery-beskederne går til Home Assistants standardpræfiks `homeassistant`. Begge gemmes (retained). En last will markerer enheden som utilgængelig, hvis BombVault stopper uden at melde fra. Slår du forbindelsen fra, fjerner BombVault enheden og dens entiteter fra Home Assistant.

## Find BombVault på netværket {#mdns}

BombVault annoncerer sin brugerflade på det lokale netværk via mDNS, protokollen bag Bonjour og Avahi. En browser når den så som `https://bombvault.local:3443`, eller `http://bombvault.local:3000` med `HTTP_ONLY`, og tjenestebrowsere viser den som webtjeneste med undertypen `_bombvault`. TXT-posterne indeholder versionen og stien. Kontakten ligger under **Indstillinger, Integrationer, Find på netværket** og er slået til fra start. Bruger en anden enhed allerede navnet, tager BombVault `bombvault-2.local` og så videre, og kortet viser den adresse, den fik. Når BombVault stopper, eller du slår annonceringen fra, melder den fra på netværket, så browsere fjerner posten med det samme.

Om annonceringen når dit netværk, afhænger af, hvordan containeren er forbundet:

- **bridge**, standarden i Unraid-skabelonen: annonceringen bliver i Dockers netværk, og ingen på LAN'et ser den. Åbn BombVault på værtens adresse som før.
- **br0** eller et andet macvlan- eller ipvlan-netværk: containeren har sin egen adresse på LAN'et, og annonceringen når det.
- **host**: annonceringen går ud over værtens interfaces ved siden af Unraids egen. Docker- og libvirt-broerne springes over.

Kun IPv4-adresser annonceres.
