# MCP-server

BombVault har en innebygd server for Model Context Protocol (MCP), protokollen som AI-assistenter som Claude Code og Claude Desktop bruker for å nå eksterne verktøy. Gjennom den kan en assistent lese hvordan det står til med sikkerhetskopiene dine og, hvis du tillater det, starte en sikkerhetskopi eller avbryte en den selv startet. Den er av til du lager en nøkkel eller slår på [pålogging via OAuth](#oauth): frem til da svarer endepunktet `/mcp` `404` på alt.

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
| `get_storage_stats` | Størrelseshistorikk for et domenes primære repository og veksten per uke, pluss brukt, ledig og total plass på disken eller fjernlageret til hvert av repositoriene | lese |
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

Starter via [API-et](api.md#errors) teller med i de samme grensene per element som starter via MCP, og i oppbevaringsvernet.

## Slå det på {#switch-on}

1. Åpne **Innstillinger, System, MCP-server**, og klikk på knappen for klienten din. En klient som ikke står i listen, kobler til via **Annen klient**.
2. Behold under **Nøkkel** valget **Ny nøkkel** og navnet som foreslås, klientens, eller skriv et som sier hvor nøkkelen brukes, for eksempel «Claude Code på laptopen». Én nøkkel per klient lar deg tilbakekalle én uten å røre de andre. **Eksisterende nøkkel** gir klienten en nøkkel du har laget før.
3. Slå på **Tillat å starte sikkerhetskopier** for en nøkkel som skal kunne starte sikkerhetskopier; uten det kan den bare lese. Du kan endre det senere på nøkkelens flis, og endringen gjelder fra assistentens neste forespørsel uten ny tilkobling.
4. Klikk på **Lag nøkkel**. Nøkkelen vises én gang. BombVault beholder bare et fingeravtrykk av den og kan ikke vise den igjen, så kopier den nå. Lukker du dialogen før klienten har brukt nøkkelen, fortsetter kortet å vise den til du bekrefter at du har kopiert den.

Uten påloggingspassord er selve webgrensesnittet åpent for alle på nettverket ditt, og den som kan åpne det, kan også lage en nøkkel. Kortet sier det. Åpner du BombVault under et navn som ser offentlig ut (for eksempel `bombvault.example.com` bak en omvendt proxy), og det ikke er satt noe påloggingspassord, kan det ikke lages eller byttes nøkler fra den adressen, slik at ingen nettside på internett kan få nettleseren din til å lage en. Sett et påloggingspassord, eller åpne BombVault via IP-adressen eller et lokalt navn som `tower` eller `tower.local`.

## Nøklene dine og loggen deres {#keys}

Hver nøkkel har sin egen flis på kortet. Den viser navnet på nøkkelen, om den kan starte sikkerhetskopier eller bare lese, de fire siste tegnene i nøkkelen, når den ble opprettet eller sist byttet, når en klient sist brukte den og hvor mange kall den har gjort i dag. På flisen gir du nøkkelen nytt navn, endrer tillatelsen, bytter den eller tilbakekaller den. En tilbakekalt nøkkel flytter til listen over tilbakekalte nøkler, der du kan slette den for godt når ingen kjøring i historikken nevner den lenger.

Ved siden av navnet viser flisen logoen til klienten nøkkelen ble laget for. En nøkkel laget via **Annen klient**, eller før kortet listet opp klienter, viser en nøkkel i stedet.

**Logg** på en flis åpner det nøkkelen har gjort. Først kommer sikkerhetskopiene den startet, hver med status og en lenke til kjøringen i aktivitetsloggen på dashbordet. Under dem står kallene, nyeste først, med verktøyet og hva som ble av kallet. En avvisning sier hvorfor: nøkkelen kan bare lese, oppbevaringsvernet holdt sikkerhetskopien tilbake, en annen sikkerhetskopi kjørte allerede, elementet ble sikkerhetskopiert via MCP for noen minutter siden, eller nøkkelen sendte for mange forespørsler. En avbrytelse lenker til kjøringen det gjaldt.

BombVault tar vare på oppføringene til hver nøkkel i opptil 30 dager: de nyeste 500 vellykkede startene og avbrytelsene og ved siden av dem de nyeste 200 andre kallene (lesinger, avvisninger og feil), så en assistent som spør om en pågående sikkerhetskopi eller prøver et avvist kall igjen og igjen, ikke kan skyve starten ut av loggen. For hvert kall lagrer det verktøyet, utfallet og kjøringen en avbrytelse nevnte. Det lagrer aldri det assistenten sendte, og aldri nøkkelen eller fingeravtrykket. Diagnosepakken teller bare oppføringene, og en eksport av innstillingene utelater dem.

## Koble til en klient {#clients}

Hver klient har en knapp på kortet, under **På denne datamaskinen** eller **I skyen**. Knappen åpner en dialog i tre trinn: nøkkelen; konfigurasjonen for den klienten, med adressen du åpnet kortet på, en knapp for å kopiere den, hvor konfigurasjonen ligger og, med BombVaults eget sertifikat, hva klienten trenger for å stole på det; og ventingen på klientens første kall. Dialogen følger med på nøkkelens siste bruk og blir grønn når det kallet kommer.

Dialogen holder nøkkelen unna alle kommandolinjer. Der klienten kan lese den fra en miljøvariabel (`BOMBVAULT_MCP_KEY`), en skjult forespørsel eller en egen fil, nevner konfigurasjonen den bare. Der klienten ikke har en slik måte, står nøkkelen i konfigurasjonsfilen eller innstillingene, og dialogen sier det. Der dokumentasjonen til en klient ikke sier hvordan den behandler et ukjent sertifikat, skriver dialogen det steget som hva du gjør hvis klienten avviser BombVaults sertifikat.

| Klient | Oppsett | Hvor nøkkelen kommer fra |
|---|---|---|
| AnythingLLM | konfigurasjonsfil | konfigurasjonsfilen |
| Antigravity | konfigurasjonsfil | miljøvariabel |
| Claude Code | kommando | nøkkelfil |
| Claude Desktop | konfigurasjonsfil | nøkkelfil |
| Cline | konfigurasjonsfil | konfigurasjonsfilen |
| Codex CLI | konfigurasjonsfil | miljøvariabel |
| Continue | konfigurasjonsfil | `~/.continue/.env` |
| Copilot CLI | konfigurasjonsfil | konfigurasjonsfilen |
| Cursor | konfigurasjonsfil | miljøvariabel |
| Gemini CLI | konfigurasjonsfil | miljøvariabel |
| GitHub Copilot (VS Code) | konfigurasjonsfil | skjult forespørsel |
| Goose | konfigurasjonsfil | miljøvariabel |
| Jan | skjema i appen | appens innstillinger |
| JetBrains (AI Assistant, Junie) | konfigurasjonsfil | konfigurasjonsfilen |
| Kimi Code | konfigurasjonsfil | konfigurasjonsfilen |
| LM Studio | konfigurasjonsfil | konfigurasjonsfilen |
| Mistral Vibe | konfigurasjonsfil | miljøvariabel |
| Msty | skjema i appen | appens innstillinger |
| n8n | skjema i appen | påloggingsdataene i n8n |
| Open WebUI | skjema i appen | appens innstillinger |
| opencode | konfigurasjonsfil | miljøvariabel |
| Perplexity (Mac) | skjema i appen | nøkkelfil |
| Qwen Code | konfigurasjonsfil | miljøvariabel |
| Roo Code | konfigurasjonsfil | miljøvariabel |
| Visual Studio | konfigurasjonsfil | konfigurasjonsfilen |
| Warp | konfigurasjonsfil | konfigurasjonsfilen |
| Windsurf | konfigurasjonsfil | miljøvariabel |
| Zed | konfigurasjonsfil | konfigurasjonsfilen |
| Grok | skjema, i skyen | leverandørens servere |
| Le Chat | skjema, i skyen | leverandørens servere |
| ChatGPT | pålogging via OAuth, i skyen | et tilgangstoken, se [nedenfor](#oauth) |
| Claude (claude.ai) | pålogging via OAuth, i skyen | et tilgangstoken, se [nedenfor](#oauth) |

Avsnittene nedenfor forklarer oppsettet av Claude Code og Claude Desktop nærmere og sier hva enhver annen klient trenger.

### Claude Code {#claude-code}

Claude Code når BombVault via `mcp-remote`, som trenger Node.js på den datamaskinen. Lagre først nøkkelen i en egen tekstfil, på én linje:

```text
X-API-Key: <your key>
```

Kjør deretter kommandoen fra kortet én gang i en terminal, med stien til den filen fylt inn. Bak et sertifikat datamaskinen din stoler på, ser den slik ut:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

Med BombVaults eget sertifikat (se [TLS og sertifikater](#tls)) peker kommandoen i tillegg Node.js på det nedlastede sertifikatet:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Sjekk tilkoblingen med `/mcp` inne i Claude Code. `--scope user` gjør BombVault tilgjengelig i alle prosjektene dine. Claude Code lagrer bare stien til nøkkelfilen, så nøkkelen dukker verken opp i kommandoen og skallhistorikken din eller i prosesslisten. Legg filen der bare du kan lese den, og utenfor alle mapper du committer. `@latest` får `npx` til å hente en oppdatert `mcp-remote`; ellers ville en eldre, globalt installert versjon blitt brukt, og den kjenner ikke `--header-file`.

Ikke skriv `${BOMBVAULT_MCP_KEY}` inn i argumentene til `mcp-remote` for Claude Code. Claude Code fyller inn en slik referanse fra sitt eget miljø før den starter `mcp-remote`, så nøkkelen havner på kommandolinjen til den prosessen, der andre programmer og brukere på datamaskinen kan lese den.

Uten Node.js, og bare bak et sertifikat datamaskinen din stoler på, kan Claude Code koble til på egen hånd. Legg en `.mcp.json` i prosjektmappen:

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

Sett `BOMBVAULT_MCP_KEY` der Claude Code starter, for eksempel under `"env"` i `~/.claude/settings.json` eller i skallprofilen din, redigert i en tekstredigerer i stedet for skrevet inn ved ledeteksten. Her er referansen trygg, fordi Claude Code ikke starter noen ekstra prosess som nøkkelen kunne havne i. BombVaults eget sertifikat fungerer ikke på denne måten: Claude Codes egen tilkobling avviser det selv når `NODE_EXTRA_CA_CERTS` er satt. Commit aldri en `.mcp.json` der nøkkelen står skrevet ut.

### Claude Desktop {#claude-desktop}

Claude Desktop når BombVault via `mcp-remote`, som trenger Node.js på den datamaskinen. Lagre først nøkkelen i en egen tekstfil, som én linje, slik det er beskrevet for [Claude Code](#claude-code). Åpne konfigurasjonsfilen i Claude Desktop via **Settings, Developer, Edit Config**. Den ligger i `%APPDATA%\Claude\claude_desktop_config.json` på Windows og i `~/Library/Application Support/Claude/claude_desktop_config.json` på macOS. Legg til oppføringen fra kortet inne i `"mcpServers"`, ved siden av serverne som allerede står der, og start Claude Desktop på nytt:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote@latest", "https://192.168.1.10:3443/mcp", "--header-file", "<path of the file with your key>"],
      "env": {
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` er bare med for BombVaults eget sertifikat. Bak et sertifikat datamaskinen din allerede stoler på, tar du det bort.
- `--allow-http` legges bare til for en vanlig `http://`-adresse.
- På Windows skriver du stiene med vanlige skråstreker, for eksempel `C:/Users/sam/bombvault-key.txt`, fordi en enkelt omvendt skråstrek ikke er gyldig JSON. Hold stien til nøkkelfilen fri for mellomrom: Claude Desktop på Windows sender en sti med mellomrom til `npx` i to deler.
- Konfigurasjonen nevner bare nøkkelfilen, så nøkkelen vises verken i den eller i prosesslisten. Legg filen der bare du kan lese den.

### Klienter i skyen {#cloud-clients}

ChatGPT, Claude på claude.ai, Grok og Le Chat kaller BombVault fra leverandørens servere, så BombVault må kunne nås fra internett med et offentlig klarert sertifikat, for eksempel bak en omvendt proxy; Le Chat avviser selvsignerte. En pålogging på proxyen kan beskytte nettgrensesnittet, men `/mcp` må komme frem til BombVault uten den: disse tjenestene kan ikke logge på en proxy, og BombVault sjekker selv nøkkelen eller tokenet deres. Grok og Le Chat sender en fast nøkkel, og knappene deres setter dem opp som de andre. ChatGPT, og i de fleste organisasjoner også Claude på claude.ai, kobler seg bare til via pålogging med OAuth, som er beskrevet nedenfor.

### Pålogging via OAuth {#oauth}

For en klient som ikke kan ta en nøkkel, er BombVault sin egen OAuth-autorisasjonsserver. Klienten registrerer seg selv, sender deg til en BombVault-side, og der logger du på med innloggingspassordet ditt (og den andre faktoren, hvis du har satt en opp) og gir den tillatelse. Klienten får så et token som bare gjelder MCP-endepunktet i denne BombVault, og fornyer det selv.

1. Sett et innloggingspassord under **Innstillinger, System**. Uten passord tilbyr BombVault ingen pålogging, fordi det ikke finnes noen å spørre om samtykke.
2. Gjør BombVault tilgjengelig fra internett over https med et sertifikat som nettlesere stoler på, vanligvis via en omvendt proxy. Klienten kaller `/mcp`, `/oauth/` og `/.well-known/` fra sine egne servere, så en proxy med egen pålogging må slippe disse tre stiene gjennom til BombVault. Samtykkesiden på `/oauth/authorize` åpnes i din egen nettleser og kan bli bak proxyens pålogging. Oppgi også proxyen i `TRUSTED_PROXY` (se [Konfigurasjon](configuration.md)). BombVault begrenser klientregistreringer per adresse, og uten den ser alle klienter ut til å komme fra proxyen.
3. Slå på **Pålogging via OAuth** på MCP-kortet, og skriv inn **Offentlig adresse**: https-adressen uten sti, for eksempel `https://backup.example.com`. Hvert token er bundet til denne adressen, så etter en endring må hver klient logge på igjen.
4. Klikk på knappen for ChatGPT eller Claude. Dialogen viser **Connector-URL**, altså den offentlige adressen med `/mcp` bak, og hvor den skal inn i den klienten. I ChatGPT slår du på utviklermodus under **Innstillinger, Apper og connectors, Avanserte innstillinger**, velger **Opprett**, limer inn connector-URL-en som MCP-server-URL og velger OAuth som autentisering. På claude.ai åpner du **Innstillinger, Connectors, Legg til egendefinert connector**, limer inn connector-URL-en, lar OAuth-klient-ID og hemmelighet stå tomme og velger **Koble til**.
5. Klienten åpner samtykkesiden. Den viser hvem som spør, hvor svaret ditt sender deg tilbake til, og bryteren **Tillat å starte sikkerhetskopier**, som starter av. Velg **Tillat** eller **Avslå**.

Hver klient som har logget på, får en flis ved siden av nøklene, med merket sitt, loggen sin, **Tilbakekall** og **Tillat å starte sikkerhetskopier**, og de samme grensene som en nøkkel. Tilbakekalling virker med en gang. Når samme klient logger på igjen, erstatter den nye tillatelsen den gamle, og en tillatelse som ingen har brukt på 30 dager, utløper. Opptil 10 klienter kan være pålogget samtidig, i tillegg til de 10 nøklene.

Samtykkesiden tar bare imot en forespørsel fra en registrert klient som oppgir nøyaktig én av sine registrerte returadresser: https, eller en loopback-adresse på en hvilken som helst port for en klient på din egen datamaskin. Bare authorization code-flyten med PKCE (S256) godtas, og svaret ditt er bundet til økten din, så ingen annen nettside kan sende det for deg. Tilgangstokener gjelder en time. Et oppdateringstoken erstattes ved hver bruk, og dukker ett opp igjen etterpå, tilbakekaller BombVault tillatelsen, fordi noen andre har en kopi. En klient som gjentar sin siste fornyelse innen 30 sekunder, fordi svaret aldri kom frem, får i stedet nye tokener. BombVault henter ikke klientmetadata fra internett, så klienter registrerer seg via dynamisk klientregistrering.

### Andre klienter {#other-clients}

Enhver klient som snakker Streamable HTTP, fungerer:

- URL: adressen til webgrensesnittet pluss `/mcp`, for eksempel `https://192.168.1.10:3443/mcp`.
- Nøkkelen i `Authorization: Bearer <key>` eller i `X-API-Key: <key>`. Sendes begge, må de inneholde samme nøkkel.
- `POST` med `Content-Type: application/json` og `Accept: application/json, text/event-stream`.
- Én JSON-RPC-melding per forespørsel; batcher avvises.
- Protokollversjonene 2026-07-28, 2025-11-25, 2025-06-18 og 2025-03-26.

## TLS og sertifikater {#tls}

BombVault leverer HTTPS med et sertifikat det har utstedt selv, og i starten nevner det sertifikatet bare `localhost`, `127.0.0.1` og `::1`. Claude Code og `mcp-remote` avviser det på en LAN-adresse. Veiene rundt det, i rekkefølgen som passer de fleste Unraid-installasjoner:

1. **Legg til adressen i MCP-kortet.** Åpner du kortet over HTTPS på en adresse sertifikatet ikke nevner, sier kortet det og tilbyr **Legg denne adressen til sertifikatet**. BombVault utsteder da sertifikatet på nytt med den adressen (nettleseren din advarer én gang til, akkurat som første gang). Klikk deretter på **Last ned sertifikat**; utdragene setter `NODE_EXTRA_CA_CERTS` til den nedlastede filen, slik at klienten stoler på akkurat det sertifikatet. Det betyr også at enhver klient som er satt opp med en tidligere nedlastet fil, slutter å koble til så snart sertifikatet utstedes på nytt, på denne datamaskinen og på alle andre, til den får den nye filen.
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

- Uten en aktiv nøkkel og med pålogging via OAuth av svarer `/mcp` `404`.
- Pålogging via OAuth tilbys bare så lenge et innloggingspassord er satt. Tokener, koder og klienthemmeligheter lagres bare som fingeravtrykk, og et token gjelder bare for adressen det ble utstedt for.
- En klient kan registrere seg høyst 10 ganger i timen fra én adresse, og BombVault beholder høyst 100 registrerte klienter som ingen har logget på med, hver i et døgn. Feil koder og oppdateringstokener teller mot samme sperre som feil nøkler.
- Tillatelser oppfører seg som nøkler når en konfigurasjonskopi gjenopprettes eller `APP_KEY` endres: etter en gjenoppretting må hver klient logge på igjen.
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
- Behandle en nøkkel som et passord. En klient som ikke kan lese nøkkelen fra en miljøvariabel, en forespørsel eller en nøkkelfil, har den i klartekst i konfigurasjonen eller innstillingene sine, og dialogen dens sier det. Bruk heller en nøkkel som bare kan lese på en datamaskin du stoler mindre på.

## Hva som forlater maskinen {#privacy}

Det en assistent leser, går til AI-leverandøren bak den: navn på elementer, tidsplaner, kjøringshistorikk med feilmeldinger, id-er og tidspunkter for gjenopprettingspunkter, navn på databasemotorer og størrelser på dumper, pågående aktivitet, lagringstall, dekning og status. BombVault fjerner vertsstier, repository-plasseringer, vertsnavn, påloggingsdetaljer, hook-kommandoer og nøkler før noe forlater maskinen.

## Feilsøking {#troubleshooting}

| Hva du ser | Hva det betyr |
|---|---|
| `404` | Ingen aktiv nøkkel og pålogging via OAuth er av, eller en feil sti som `/api/mcp`. Endepunktet er `/mcp`. |
| `401` | Nøkkelen mangler, er feilskrevet, tilbakekalt eller byttet. Kanskje kaster en proxy headeren `Authorization` (prøv `X-API-Key`). Merker kortet nøkkelen som ikke lenger gyldig, er `APP_KEY` endret: bytt nøkkelen. |
| `403` | Forespørselen kom fra en nettleserside med en annen origin. Bruk en skrivebords- eller kommandolinjeklient. |
| `405` ved GET | Normalt. Endepunktet tar bare imot `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Klienten er for gammel for Streamable HTTP. Oppdater den. |
| `400` "batch requests are not accepted" | Klienten sender JSON-RPC-batcher. Send én melding per forespørsel. |
| `429` | For mange feil nøkler fra denne adressen, eller flere enn 120 forespørsler i minuttet med én nøkkel. Vent ett minutt og sjekk om assistenten har gått seg fast i en løkke. |
| Feil med "certificate", "self-signed" eller "unable to verify" | Klienten stoler ikke på BombVaults sertifikat. Se [TLS og sertifikater](#tls). |
| `busy` | En annen sikkerhetskopi eller en vedlikeholdsoppgave opptar domenet. Prøv igjen når den er ferdig. |
| `cooldown` | Dette elementet, dette domenet eller Backup Everything ble startet via MCP for mindre enn 15 minutter siden. |
| `retention_guard` | Én MCP-sikkerhetskopi til ville bare etterlate gjenopprettingspunkter fra MCP i et vindu med "behold de siste N", eller elementet har allerede fått 4 sikkerhetskopier via MCP de siste 24 timene, mislykkede og avbrutte medregnet. I det første tilfellet gir neste planlagte sikkerhetskopi plass, i det andre er elementet ledig igjen 24 timer etter den eldste av dem. Du kan alltid starte den i webgrensesnittet. |
| `rate_limited` | Nøkkelen har brukt opp sine 12 starter for denne timen. |
| `not_permitted` ved en start | Nøkkelen kan bare lese. Slå på **Tillat å starte sikkerhetskopier** i kortet; ny tilkobling trengs ikke. Ved en avbrytelse betyr det at denne nøkkelen ikke startet kjøringen. |
| `domain_off` | Den typen sikkerhetskopi er slått av i innstillingene. |
| `not_found` | BombVault beskytter ikke det elementet. Legg det først til i webgrensesnittet; MCP lager aldri konfigurasjon. |
| Klienten finner ikke autorisasjonsserveren | Pålogging via OAuth er av, det er ikke satt noe innloggingspassord, eller proxyen slipper ikke `/.well-known/` gjennom til BombVault. |
| Samtykkesiden sier at returadressen ikke er registrert | Klienten sendte en returadresse den ikke har registrert. Fjern connectoren i klienten og legg den til på nytt. |
| En pålogget klient får `401` | Tillatelsen er tilbakekalt, har utløpt etter 30 dager uten bruk, eller den offentlige adressen er endret. Klienten logger på igjen. |

Ikke sett miljøvariabelen `MCPGODEBUG` på containeren. Den endrer hvordan MCP-biblioteket oppfører seg, og en ugyldig verdi stopper BombVault ved oppstart før den skriver en eneste logglinje.
