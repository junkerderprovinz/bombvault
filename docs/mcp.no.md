# MCP-server

BombVault har en innebygd server for Model Context Protocol (MCP), protokollet som AI-assistenter som Claude Code og Claude Desktop bruker for å nå eksterne verktøy. Gjennom den kan en assistent lese hvordan det står til med sikkerhetskopiene dine, og hvis du tillater det, starte en sikkerhetskopi eller avbryte en den selv har startet. Serveren er av til du lager en nøkkel: uten en aktiv nøkkel svarer endepunktet `/mcp` med `404` på alt.

## Hva en assistent kan og ikke kan gjøre {#tools}

| Verktøy | Hva det gjør | Type |
|---|---|---|
| `get_health` | Versjon, instansnavn, om en sikkerhetskopi kjører, og hva denne nøkkelen får gjøre | lese |
| `get_status` | Beskyttelsesstatus per domene: siste vellykkede sikkerhetskopi, forventet intervall, verifiseringer og off-site-kontroller, neste planlagte kjøringer | lese |
| `get_coverage` | Hva BombVault beskytter og hva det ikke beskytter, med grunnen for hver | lese |
| `list_items` | Hver beskyttet container, VM og mappesett, flashminnet og app-konfigurasjonen, med tidsplan, hva en sikkerhetskopi stopper, siste sikkerhetskopi og hvor lang tid den tok; databasecontainere viser også sin siste dump; ZFS-datasett er også med, med resultatet av den siste kontrollen | lese |
| `list_runs` | Kjøringshistorikk, nyeste først, kan filtreres på domene, element, status, type og tid | lese |
| `list_restore_points` | Gjenopprettingspunkter for ett element fra dets primære repository, og for en container også databasedumpene; et ZFS-datasett får ett gjenopprettingspunkt per sikkerhetskopi, med et snapshot av hvert datasett under det | lese |
| `get_activity` | Hva som kjører akkurat nå, med fase og prosent | lese |
| `get_storage_stats` | Størrelseshistorikk for et domenes primære repository og veksten per uke | lese |
| `list_anomalies` | Avvik BombVault har lagt merke til i sikkerhetskopiene, kan filtreres på tilstand, alvorlighet og domene, med en oversikt over det som er åpent | lese |
| `get_anomaly` | Ett av disse avvikene, med notatet som ble skrevet da det ble kvittert ut | lese |
| `start_backup` | Sikkerhetskopierer ett element med en gang | starte |
| `start_domain_backup` | Sikkerhetskopierer hvert beskyttet element i et domene | starte |
| `start_backup_everything` | Kjører Backup Everything-gjennomgangen | starte |
| `cancel_backup` | Avbryter en kjørende sikkerhetskopi som denne nøkkelen har startet | avbryte |

Dette blir værende i webgrensesnittet: gjenopprettinger av alle slag (også å laste ned, lagre eller importere en databasedump), sletting av sikkerhetskopier, prune, unlock, kontroller og øvelser, off-site-replikering, innstillinger, påloggingsdetaljer og MCP-nøkler, og å avbryte en sikkerhetskopi som tidsplanen, webgrensesnittet eller en annen nøkkel har startet. Det samme gjelder å kvittere ut et avvik eller merke det som forventet, noe som skjer på siden **Avvik**. Grunnen er at verktøyenes svar inneholder navn og feilmeldinger fra serveren din, og hvilken som helst av dem kan inneholde tekst skrevet for å styre assistenten. En assistent som går på slik tekst, kan i verste fall starte en sikkerhetskopi innenfor grensene nedenfor eller avbryte en den selv har startet.

Ligger det primære repositoryet til et element et annet sted (S3, REST, SFTP, rclone), kontakter `list_restore_points` det, og kallet kan ta litt tid. Off-site-kopier kan ikke listes via MCP. Hva avvikskontrollene ser på, står under [Funksjoner](features.md), og hvordan et ZFS-element har ett øyeblikksbilde per datasett, under [ZFS-datasett](zfs-datasets.md#contents).

## Hva en startet sikkerhetskopi gjør {#starting-backups}

En assistents sikkerhetskopi er den samme som webgrensesnittet starter. En kjørende container stoppes til sikkerhetskopien er ferdig, sammen med containerne som er satt til å stoppe med den. En VM med metoden "graceful" slås av og startes igjen. Et ZFS-datasett stopper containerne som er satt opp for det, mens snapshotet tas. Mappesett, flashminnet og konfigurasjonen fortsetter å kjøre. Etterpå bruker BombVault oppbevaringspolicyen og kopierer eventuelt til off-site-repositoryet. `list_items` forteller assistenten hva et element stopper og hvor lang tid den siste sikkerhetskopien tok, og verktøybeskrivelsene ber den si det til deg før den starter noe.

Fordi en sikkerhetskopi stopper ting og skyver ut gamle gjenopprettingspunkter, er starter via MCP begrenset:

- 12 startede sikkerhetskopier per time per nøkkel.
- 15 minutter mellom to MCP-starter av samme element, samme domene eller Backup Everything.
- Høyst 4 MCP-starter av samme element på 24 timer.
- **Oppbevaringsvern.** Når et domene beholder et fast antall gjenopprettingspunkter (bare "behold de siste N", uten daglig, ukentlig eller månedlig regel, lokalt eller på et off-site-mål), skyver hver ny sikkerhetskopi ut den eldste. BombVault avviser da en MCP-start av et element der de nyeste N-1 vellykkede sikkerhetskopiene alle ble startet via MCP. Det blir derfor alltid minst ett gjenopprettingspunkt igjen i det beholdte settet som tidsplanen eller du har laget. Med "behold den siste 1" kan en assistent ikke sikkerhetskopiere det elementet i det hele tatt. Neste planlagte sikkerhetskopi gir plass igjen.

En start av et domene eller av Backup Everything utelater elementene som en grense holder tilbake, og nevner dem i svaret. Webgrensesnittet og tidsplanen berøres ikke av noe av dette. Timebudsjettet ligger i minnet, så en omstart av BombVault nullstiller det.

## Slå det på {#switch-on}

1. Åpne **Innstillinger, System, MCP-server** og klikk på **Ny nøkkel**.
2. Gi nøkkelen et navn som sier hvor den brukes, for eksempel "Claude Code på laptopen". Med én nøkkel per klient kan du tilbakekalle én uten å røre de andre.
3. La **Tillat å starte sikkerhetskopier** stå på, eller slå det av for en nøkkel som bare skal lese. Du kan endre det senere i nøkkelens rad, og endringen gjelder fra assistentens neste forespørsel uten ny tilkobling.
4. Klikk på **Lag nøkkel**. Nøkkelen vises én gang. BombVault lagrer bare et fingeravtrykk av den og kan ikke vise den igjen, så kopier den nå eller ta et av utdragene under, som da inneholder den ekte nøkkelen.

Uten påloggingspassord er selve webgrensesnittet åpent for alle på nettverket ditt, og den som kan åpne det, kan også lage en nøkkel. Kortet sier det. Åpner du BombVault under et navn som ser offentlig ut (for eksempel `bombvault.example.com` bak en omvendt proxy), og det ikke er satt noe påloggingspassord, kan det ikke lages eller byttes nøkler fra den adressen, slik at ingen nettside på internett kan få nettleseren din til å lage en. Sett et påloggingspassord, eller åpne BombVault via IP-adressen eller et lokalt navn som `tower` eller `tower.local`.

## Koble til en klient {#clients}

Kortet viser ferdige utdrag for adressen du åpnet det på: velg klienten din og kopier utdraget. Resten av avsnittet forklarer hva utdragene gjør, og gir formene kortet ikke viser.

### Claude Code {#claude-code}

Kjør kommandoen fra kortet én gang i en terminal. Med et sertifikat datamaskinen din stoler på, ser den slik ut:

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

Sjekk tilkoblingen med `/mcp` inne i Claude Code. `--scope user` lagrer nøkkelen i brukerkonfigurasjonen din i stedet for i en prosjektfil.

Kommandoen inneholder nøkkelen, og skallet ditt kan lagre den i historikken. Det unngår du med en `.mcp.json` i prosjektmappen og nøkkelen i en miljøvariabel. Claude Code setter inn `${BOMBVAULT_MCP_KEY}` når den leser filen:

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

Sett `BOMBVAULT_MCP_KEY` der Claude Code starter, for eksempel i skallprofilen din, redigert i en tekstredigerer i stedet for skrevet inn ved ledeteksten. Commit aldri en `.mcp.json` der nøkkelen står skrevet ut.

Med BombVaults eget sertifikat (se [TLS og sertifikater](#tls)) kjører kommandoen fra kortet i stedet `mcp-remote` og peker Node.js på det nedlastede sertifikatet:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

De enkle anførselstegnene hindrer skallet i å utvide variabelen; det gjør `mcp-remote` selv. Samme form fungerer i `.mcp.json`: bruk oppføringen for Claude Desktop nedenfor og ta `BOMBVAULT_MCP_KEY` ut av dens `env`, så kommer nøkkelen fra miljøet ditt.

### Claude Desktop {#claude-desktop}

Claude Desktop når BombVault via `mcp-remote`, som trenger Node.js på den datamaskinen. Åpne konfigurasjonsfilen i Claude Desktop via **Settings, Developer, Edit Config**. Den ligger i `%APPDATA%\Claude\claude_desktop_config.json` på Windows og i `~/Library/Application Support/Claude/claude_desktop_config.json` på macOS. Legg til oppføringen fra kortet inne i `"mcpServers"`, ved siden av serverne som allerede står der, og start Claude Desktop på nytt:

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

- `NODE_EXTRA_CA_CERTS` er bare med for BombVaults eget sertifikat. Bak et sertifikat datamaskinen din allerede stoler på, tar du det bort.
- `--allow-http` legges bare til for en vanlig `http://`-adresse.
- Headeren skrives `X-API-Key:${BOMBVAULT_MCP_KEY}`, uten mellomrom etter kolonet og med nøkkelen i `env`. På noen systemer deler `mcp-remote` en `--header`-verdi ved første mellomrom, og en nøkkel skrevet etter et mellomrom ville gå tapt.

### Egne connectors i Claudes innstillinger {#custom-connectors}

Connectors som du legger til i Claudes egne innstillinger (på claude.ai og i connectorlisten i Claude Desktop), støttes ikke ennå. Slike connectors kontaktes fra Anthropics sky, så de trenger en offentlig HTTPS-adresse, og de logger inn via OAuth. De kan ikke sende med en fast nøkkel, og BombVault tilbyr bare faste nøkler, ingen OAuth-pålogging. Å legge BombVault ut på internett for deres skyld ville ikke hjelpe. Bruk Claude Code, eller Claude Desktop via `mcp-remote` som over.

### Andre klienter {#other-clients}

Enhver klient som snakker Streamable HTTP, fungerer:

- URL: adressen til webgrensesnittet pluss `/mcp`, for eksempel `https://192.168.1.10:3443/mcp`.
- Nøkkelen i `Authorization: Bearer <key>` eller i `X-API-Key: <key>`. Sendes begge, må de inneholde samme nøkkel.
- `POST` med `Content-Type: application/json` og `Accept: application/json, text/event-stream`.
- Én JSON-RPC-melding per forespørsel; batcher avvises.
- Protokollversjonene 2026-07-28, 2025-11-25, 2025-06-18 og 2025-03-26.

## TLS og sertifikater {#tls}

BombVault leverer HTTPS med et sertifikat det har utstedt selv, og i starten nevner det sertifikatet bare `localhost`, `127.0.0.1` og `::1`. Claude Code og `mcp-remote` avviser det på en LAN-adresse. Veiene rundt det, i rekkefølgen som passer de fleste Unraid-installasjoner:

1. **Legg til adressen i MCP-kortet.** Åpner du kortet over HTTPS på en adresse sertifikatet ikke nevner, sier kortet det og tilbyr **Legg denne adressen til sertifikatet**. BombVault utsteder da sertifikatet på nytt med den adressen (nettleseren din advarer én gang til, akkurat som første gang). Klikk deretter på **Last ned sertifikat**; utdragene setter `NODE_EXTRA_CA_CERTS` til den nedlastede filen, slik at klienten stoler på akkurat det sertifikatet.
2. **En omvendt proxy med et klarert sertifikat** (Nginx Proxy Manager, SWAG, Caddy, Traefik). Klienten ser da proxyens sertifikat og trenger ikke noe mer, og kortet advarer ikke om BombVaults eget.
3. **Tailscale.** `tailscale serve` foran containeren, eller Unraids Tailscale-integrasjon, gir deg et `ts.net`-navn med et klarert sertifikat.
4. **`HTTP_ONLY=true`**, bare bak en proxy som avslutter TLS eller på et nettverk du stoler helt på. Det setter hele webgrensesnittet over på vanlig HTTP, krever en endring i containerinnstillingene og sender nøkkelen ukryptert.

Sett aldri `NODE_TLS_REJECT_UNAUTHORIZED=0`. Det slår av sertifikatkontrollen for alt den Node.js-prosessen snakker med.

En omvendt proxy må sende headeren `Authorization` (eller `X-API-Key`) videre, noe proxyer gjør med mindre man ber dem om noe annet, og må verken bufre eller skrive om `/mcp`. En location-blokk for Nginx eller Nginx Proxy Manager som også kontrollerer BombVaults sertifikat:

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

Bak en proxy bærer hver forespørsel proxyens adresse. Fem feil nøkler fra én feilkonfigurert klient stenger da alle MCP-klienter bak den proxyen ute i ett minutt. Oppgi proxyen i `TRUSTED_PROXY` (se [Konfigurasjon](configuration.md)) for å telle per klient.

## Sikkerhetsmodell {#security}

- Uten en aktiv nøkkel svarer `/mcp` med `404`.
- Ingen adresser er unntatt. Forespørsler fra `localhost`, Unraid-verten, en omvendt proxy eller `tailscale serve` trenger en nøkkel som alle andre, også når webgrensesnittet ikke har påloggingspassord.
- Nøkler lagres bare som fingeravtrykk, vises én gang og kan gis nytt navn, byttes og tilbakekalles. Opptil 10 aktive nøkler, hver med sin egen bryter **Tillat å starte sikkerhetskopier**.
- Hver oppretting, utskifting, endring av rettigheter og tilbakekalling sender et varsel via varslingskanalene dine, med adressen det kom fra, med mindre varsler er slått av.
- 5 feil nøkler per minutt per adresse, deretter `429`. 120 forespørsler per minutt og 12 startede sikkerhetskopier per time per nøkkel, i tillegg til ventetiden og oppbevaringsvernet over.
- Forespørsler fra en nettleserside med en annen origin avvises.
- Så lenge det ikke er satt noe påloggingspassord, kan det ikke lages nøkler fra et vertsnavn som ser offentlig ut.
- Hver sikkerhetskopi en assistent starter, og prune- og off-site-kjøringene den fører til, er merket "via MCP" med nøkkelens navn i aktivitetsloggen, i feilpanelet og i varselet om sikkerhetskopien.
- Hvert verktøykall skrives til containerloggen med nøkkelens id og dens siste fire tegn (aldri navnet) og telles i `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Å gjenopprette en sikkerhetskopi av konfigurasjonen tilbakekaller alle nøkler, fordi den gjenopprettede databasen kan inneholde nøkler du tilbakekalte etter at den ble lagret. Lag nye nøkler etterpå.
- En nøkkel slutter å virke når `APP_KEY` endres (en reinstallasjon eller en gjenoppretting til en annen container). Kortet oppdager det og merker nøkkelen, og **Bytt nøkkel** gir den en gyldig hemmelighet igjen.
- Behandle en nøkkel som et passord. Claude Code og Claude Desktop lagrer den i klartekst i konfigurasjonen sin. På en datamaskin du stoler mindre på, er en nøkkel som bare kan lese å foretrekke.

## Hva som forlater maskinen {#privacy}

Det en assistent leser, går til AI-leverandøren bak den: navn på elementer, tidsplaner, kjøringshistorikk med feilmeldinger, id-er og tidspunkter for gjenopprettingspunkter, navn på databasemotorer og størrelser på dumper, pågående aktivitet, lagringstall, dekning og status. BombVault fjerner vertsstier, repository-plasseringer, vertsnavn, påloggingsdetaljer, hook-kommandoer og nøkler før noe forlater maskinen.

## Feilsøking {#troubleshooting}

| Hva du ser | Hva det betyr |
|---|---|
| `404` | Ingen aktiv nøkkel, eller en feil sti som `/api/mcp`. Endepunktet er `/mcp`. |
| `401` | Nøkkelen mangler, er feilskrevet, tilbakekalt eller byttet. Kanskje kaster en proxy headeren `Authorization` (prøv `X-API-Key`). Merker kortet nøkkelen som ikke lenger gyldig, er `APP_KEY` endret: bytt nøkkelen. |
| `403` | Forespørselen kom fra en nettleserside med en annen origin. Bruk en skrivebords- eller kommandolinjeklient. |
| `405` ved GET | Normalt. Endepunktet tar bare imot `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Klienten er for gammel for Streamable HTTP. Oppdater den. |
| `400` "batch requests are not accepted" | Klienten sender JSON-RPC-batcher. Send én melding per forespørsel. |
| `429` | For mange feil nøkler fra denne adressen, eller flere enn 120 forespørsler i minuttet med én nøkkel. Vent ett minutt og sjekk om assistenten har gått seg fast i en løkke. |
| Feil med "certificate", "self-signed" eller "unable to verify" | Klienten stoler ikke på BombVaults sertifikat. Se [TLS og sertifikater](#tls). |
| `busy` | En annen sikkerhetskopi eller en vedlikeholdsoppgave opptar domenet. Prøv igjen når den er ferdig. |
| `cooldown` | Dette elementet, dette domenet eller Backup Everything ble startet via MCP for mindre enn 15 minutter siden. |
| `retention_guard` | Én MCP-sikkerhetskopi til ville bare etterlate gjenopprettingspunkter fra MCP i et vindu med "behold de siste N". Neste planlagte sikkerhetskopi gir plass, eller start den i webgrensesnittet. |
| `rate_limited` | Nøkkelen har brukt opp sine 12 starter for denne timen. |
| `not_permitted` ved en start | Nøkkelen kan bare lese. Slå på **Tillat å starte sikkerhetskopier** i kortet; ny tilkobling trengs ikke. Ved en avbrytelse betyr det at denne nøkkelen ikke startet kjøringen. |
| `domain_off` | Den typen sikkerhetskopi er slått av i innstillingene. |
| `not_found` | BombVault beskytter ikke det elementet. Legg det først til i webgrensesnittet; MCP lager aldri konfigurasjon. |

Ikke sett miljøvariabelen `MCPGODEBUG` på containeren. Den endrer hvordan MCP-biblioteket oppfører seg, og en ugyldig verdi stopper BombVault ved oppstart før den skriver en eneste logglinje.
