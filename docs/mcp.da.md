# MCP-server

BombVault har en indbygget server til Model Context Protocol (MCP), den protokol som AI-assistenter som Claude Code og Claude Desktop bruger til at nå eksterne værktøjer. Gennem den kan en assistent læse, hvordan det står til med dine sikkerhedskopier, og hvis du tillader det, starte en sikkerhedskopi eller annullere en, den selv har startet. Serveren er slået fra, indtil du opretter en nøgle: uden en aktiv nøgle svarer endepunktet `/mcp` med `404` på alt.

## Hvad en assistent kan og ikke kan {#tools}

| Værktøj | Hvad det gør | Art |
|---|---|---|
| `get_health` | Version, instansnavn, om en sikkerhedskopi kører, og hvad denne nøgle må | læse |
| `get_status` | Beskyttelsesstatus pr. domæne: seneste vellykkede sikkerhedskopi, forventet interval, verifikationer og off-site-kontroller, næste planlagte kørsler | læse |
| `get_coverage` | Hvad BombVault beskytter, og hvad det ikke beskytter, med begrundelsen for hvert | læse |
| `list_items` | Hver beskyttet container, VM og mappesæt, flash-drevet og app-konfigurationen, med tidsplan, hvad en sikkerhedskopi stopper, seneste sikkerhedskopi og hvor lang tid den tog; databasecontainere viser også deres seneste dump; ZFS-datasæt er også med, med resultatet af deres seneste kontrol | læse |
| `list_runs` | Kørselshistorik, nyeste først, kan filtreres efter domæne, element, status, art og tid | læse |
| `list_restore_points` | Gendannelsespunkter for ét element fra dets primære repository, og for en container også dens databasedumps; et ZFS-datasæt får ét gendannelsespunkt pr. sikkerhedskopi, med et snapshot af hvert datasæt under det | læse |
| `get_activity` | Hvad der kører lige nu, med fase og procent | læse |
| `get_storage_stats` | Størrelseshistorik for et domænes primære repository og dets vækst pr. uge | læse |
| `list_anomalies` | Usædvanlige sikkerhedskopier, som BombVault har bemærket, kan filtreres efter tilstand, alvor og domæne, med en oversigt over det, der er åbent | læse |
| `get_anomaly` | Én af disse afvigelser, med den note, der blev skrevet, da den blev kvitteret | læse |
| `start_backup` | Sikkerhedskopierer ét element med det samme | starte |
| `start_domain_backup` | Sikkerhedskopierer hvert beskyttet element i et domæne | starte |
| `start_backup_everything` | Kører Backup Everything-gennemløbet | starte |
| `cancel_backup` | Annullerer en kørende sikkerhedskopi, som denne nøgle har startet | annullere |

Dette bliver i webgrænsefladen: gendannelser af enhver art (også download, gem og import af et databasedump), sletning af sikkerhedskopier, prune, unlock, kontroller og øvelser, off-site-replikering, indstillinger, legitimationsoplysninger og MCP-nøgler samt annullering af en sikkerhedskopi, som tidsplanen, webgrænsefladen eller en anden nøgle har startet. Det samme gælder for at kvittere for en afvigelse eller markere den som forventet, hvilket sker på siden **Afvigelser**. Grunden er, at værktøjernes svar indeholder navne og fejlmeddelelser fra din server, og hver af dem kan rumme tekst, der er skrevet for at styre assistenten. En assistent, der falder for den slags tekst, kan i værste fald starte en sikkerhedskopi inden for grænserne nedenfor eller annullere en, den selv har startet.

Ligger et elements primære repository et andet sted (S3, REST, SFTP, rclone), kontakter `list_restore_points` det, og kaldet kan tage et stykke tid. Off-site-kopier kan ikke vises via MCP.

## Hvad en startet sikkerhedskopi gør {#starting-backups}

En assistents sikkerhedskopi er den samme, som webgrænsefladen starter. En kørende container stoppes, indtil dens sikkerhedskopi er færdig, sammen med de containere, der er sat til at stoppe med den. En VM med metoden "graceful" lukkes ned og startes igen. Et ZFS-datasæt stopper de containere, der er sat op til det, mens dets snapshot tages. Mappesæt, flash-drevet og konfigurationen kører videre. Bagefter anvender BombVault opbevaringspolitikken og kopierer eventuelt til off-site-repositoryet. `list_items` fortæller assistenten, hvad et element stopper, og hvor lang tid dets seneste sikkerhedskopi tog, og værktøjernes beskrivelser beder den sige det til dig, før den starter noget.

Fordi en sikkerhedskopi stopper ting og skubber gamle gendannelsespunkter ud, er starter via MCP begrænsede:

- 12 startede sikkerhedskopier pr. time pr. nøgle.
- 15 minutter mellem to MCP-starter af samme element, samme domæne eller Backup Everything.
- Højst 4 MCP-starter af samme element på 24 timer.
- **Opbevaringsværn.** Når et domæne beholder et fast antal gendannelsespunkter (kun "behold de sidste N", uden daglig, ugentlig eller månedlig regel, lokalt eller på en off-site-destination), skubber hver ny sikkerhedskopi den ældste ud. BombVault afviser så en MCP-start af et element, hvis nyeste N-1 vellykkede sikkerhedskopier alle blev startet via MCP. Der bliver derfor altid mindst ét gendannelsespunkt i det beholdte sæt, som tidsplanen eller du har lavet. Med "behold den sidste 1" kan en assistent slet ikke sikkerhedskopiere det element. Den næste planlagte sikkerhedskopi giver plads igen.

En start af et domæne eller af Backup Everything udelader de elementer, som en grænse holder tilbage, og nævner dem i svaret. Webgrænsefladen og tidsplanen er ikke berørt af noget af dette. Timebudgettet ligger i hukommelsen, så en genstart af BombVault nulstiller det.

## Slå det til {#switch-on}

1. Åbn **Indstillinger, System, MCP-server**, og klik på **Ny nøgle**.
2. Giv nøglen et navn, der siger, hvor den bruges, for eksempel "Claude Code på den bærbare". Med én nøgle pr. klient kan du tilbagekalde én uden at røre de andre.
3. Lad **Tillad at starte sikkerhedskopier** være slået til, eller slå det fra for en nøgle, der kun skal læse. Du kan ændre det senere i nøglens række, og ændringen gælder fra assistentens næste forespørgsel uden ny forbindelse.
4. Klik på **Opret nøgle**. Nøglen vises én gang. BombVault gemmer kun et fingeraftryk af den og kan ikke vise den igen, så kopiér den nu, eller tag et af uddragene nedenunder, som så indeholder den rigtige nøgle.

Uden en login-adgangskode er selve webgrænsefladen åben for alle på dit netværk, og alle, der kan åbne den, kan også oprette en nøgle. Kortet siger det. Åbner du BombVault under et navn, der ser offentligt ud (for eksempel `bombvault.example.com` bag en reverse proxy), og er der ingen login-adgangskode, kan der ikke oprettes eller udskiftes nøgler fra den adresse, så ingen webside på internettet kan få din browser til at oprette en. Sæt en login-adgangskode, eller åbn BombVault via dens IP-adresse eller et lokalt navn som `tower` eller `tower.local`.

## Tilslut en klient {#clients}

Kortet viser færdige uddrag til den adresse, du åbnede det på: vælg din klient, og kopiér uddraget. Resten af afsnittet forklarer, hvad uddragene gør, og giver de former, kortet ikke viser.

### Claude Code {#claude-code}

Kør kommandoen fra kortet én gang i en terminal. Med et certifikat, din computer stoler på, ser den sådan ud:

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

Tjek forbindelsen med `/mcp` inde i Claude Code. `--scope user` gemmer nøglen i din brugerkonfiguration i stedet for i en projektfil.

Kommandoen indeholder nøglen, og din shell gemmer den måske i sin historik. Det undgår du med en `.mcp.json` i projektmappen og nøglen i en miljøvariabel. Claude Code indsætter `${BOMBVAULT_MCP_KEY}`, når den læser filen:

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

Sæt `BOMBVAULT_MCP_KEY`, hvor Claude Code starter, for eksempel i din shell-profil, redigeret i en teksteditor i stedet for skrevet ved prompten. Commit aldrig en `.mcp.json`, hvor nøglen står skrevet ud.

Med BombVaults eget certifikat (se [TLS og certifikater](#tls)) kører kommandoen fra kortet i stedet `mcp-remote` og peger Node.js på det hentede certifikat:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

De enkelte anførselstegn forhindrer din shell i at udfolde variablen; det gør `mcp-remote` selv. Samme form virker i `.mcp.json`: brug posten til Claude Desktop nedenfor, og udelad `BOMBVAULT_MCP_KEY` fra dens `env`, så kommer nøglen fra dit miljø.

### Claude Desktop {#claude-desktop}

Claude Desktop når BombVault via `mcp-remote`, som kræver Node.js på den computer. Åbn konfigurationsfilen i Claude Desktop via **Settings, Developer, Edit Config**. Den ligger i `%APPDATA%\Claude\claude_desktop_config.json` på Windows og i `~/Library/Application Support/Claude/claude_desktop_config.json` på macOS. Tilføj posten fra kortet inde i `"mcpServers"`, ved siden af de servere, der allerede står der, og genstart Claude Desktop:

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

- `NODE_EXTRA_CA_CERTS` er der kun for BombVaults eget certifikat. Bag et certifikat, som din computer allerede stoler på, udelader du det.
- `--allow-http` tilføjes kun for en almindelig `http://`-adresse.
- Headeren skrives `X-API-Key:${BOMBVAULT_MCP_KEY}`, uden mellemrum efter kolonet og med nøglen i `env`. På nogle systemer deler `mcp-remote` en `--header`-værdi ved det første mellemrum, og en nøgle skrevet efter et mellemrum ville gå tabt.

### Egne connectors i Claudes indstillinger {#custom-connectors}

Connectors, som du tilføjer i Claudes egne indstillinger (på claude.ai og i connectorlisten i Claude Desktop), understøttes endnu ikke. De connectors kontaktes fra Anthropics sky, så de kræver en offentlig HTTPS-adresse, og de logger ind via OAuth. De kan ikke sende en fast nøgle med, og BombVault tilbyder kun faste nøgler, ingen OAuth-login. At lægge BombVault ud på internettet for deres skyld ville ikke hjælpe. Brug Claude Code, eller Claude Desktop via `mcp-remote` som ovenfor.

### Andre klienter {#other-clients}

Enhver klient, der taler Streamable HTTP, virker:

- URL: webgrænsefladens adresse plus `/mcp`, for eksempel `https://192.168.1.10:3443/mcp`.
- Nøglen i `Authorization: Bearer <key>` eller i `X-API-Key: <key>`. Sendes begge, skal de indeholde samme nøgle.
- `POST` med `Content-Type: application/json` og `Accept: application/json, text/event-stream`.
- Én JSON-RPC-besked pr. forespørgsel; batches afvises.
- Protokolversionerne 2026-07-28, 2025-11-25, 2025-06-18 og 2025-03-26.

## TLS og certifikater {#tls}

BombVault leverer HTTPS med et certifikat, det selv har udstedt, og i starten nævner det certifikat kun `localhost`, `127.0.0.1` og `::1`. Claude Code og `mcp-remote` afviser det på en LAN-adresse. Vejene uden om, i den rækkefølge der passer til de fleste Unraid-installationer:

1. **Tilføj adressen i MCP-kortet.** Åbner du kortet over HTTPS på en adresse, certifikatet ikke nævner, siger kortet det og tilbyder **Tilføj denne adresse til certifikatet**. BombVault udsteder så sit certifikat igen med den adresse (din browser advarer én gang til, ligesom første gang). Klik derefter på **Hent certifikat**; uddragene sætter `NODE_EXTRA_CA_CERTS` til den hentede fil, så klienten stoler på netop det certifikat.
2. **En reverse proxy med et betroet certifikat** (Nginx Proxy Manager, SWAG, Caddy, Traefik). Klienten ser så proxyens certifikat og behøver intet ekstra, og kortet advarer ikke om BombVaults eget.
3. **Tailscale.** `tailscale serve` foran containeren, eller Unraids Tailscale-integration, giver dig et `ts.net`-navn med et betroet certifikat.
4. **`HTTP_ONLY=true`**, kun bag en proxy, der afslutter TLS, eller på et netværk, du stoler fuldt på. Det skifter hele webgrænsefladen til almindelig HTTP, kræver en ændring i containerindstillingerne og sender nøglen ukrypteret.

Sæt aldrig `NODE_TLS_REJECT_UNAUTHORIZED=0`. Det slår certifikatkontrollen fra for alt, som den Node.js-proces taler med.

En reverse proxy skal sende headeren `Authorization` (eller `X-API-Key`) videre, hvad proxyer gør, medmindre man beder dem om andet, og må hverken buffere eller omskrive `/mcp`. En location-blok til Nginx eller Nginx Proxy Manager, der også kontrollerer BombVaults certifikat:

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

Bag en proxy bærer hver forespørgsel proxyens adresse. Fem forkerte nøgler fra én forkert opsat klient låser så alle MCP-klienter bag den proxy ude i et minut. Angiv proxyen i `TRUSTED_PROXY` (se [Konfiguration](configuration.md)) for at tælle pr. klient.

## Sikkerhedsmodel {#security}

- Uden en aktiv nøgle svarer `/mcp` med `404`.
- Ingen adresser er undtaget. Forespørgsler fra `localhost`, Unraid-værten, en reverse proxy eller `tailscale serve` kræver en nøgle som alle andre, også når webgrænsefladen ikke har en login-adgangskode.
- Nøgler gemmes kun som fingeraftryk, vises én gang og kan omdøbes, udskiftes og tilbagekaldes. Op til 10 aktive nøgler, hver med sin egen kontakt **Må starte sikkerhedskopier**.
- Hver oprettelse, udskiftning, ændring af rettigheder og tilbagekaldelse sender en notifikation via dine notifikationskanaler med den adresse, den kom fra, medmindre notifikationer er slået fra.
- 5 forkerte nøgler pr. minut pr. adresse, derefter `429`. 120 forespørgsler pr. minut og 12 startede sikkerhedskopier pr. time pr. nøgle, plus ventetiden og opbevaringsværnet ovenfor.
- Forespørgsler fra en browserside med en anden origin afvises.
- Så længe der ikke er en login-adgangskode, kan der ikke oprettes nøgler fra et værtsnavn, der ser offentligt ud.
- Hver sikkerhedskopi, en assistent starter, og de prune- og off-site-kørsler, den fører til, er markeret "via MCP" med nøglens navn i aktivitetsloggen, i fejlpanelet og i notifikationen om sikkerhedskopien.
- Hvert værktøjskald skrives i containerens log med nøglens id og dens sidste fire tegn (aldrig navnet) og tælles i `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Gendannelse af en konfigurationssikkerhedskopi tilbagekalder alle nøgler, fordi den gendannede database kan indeholde nøgler, du tilbagekaldte, efter den blev gemt. Opret nye nøgler bagefter.
- En nøgle holder op med at virke, når `APP_KEY` ændres (en geninstallation eller en gendannelse i en anden container). Kortet opdager det og markerer nøglen, og **Udskift nøgle** giver den en gyldig hemmelighed igen.
- Behandl en nøgle som en adgangskode. Claude Code og Claude Desktop gemmer den i klartekst i deres konfiguration. På en computer, du stoler mindre på, er en nøgle, der kun må læse, at foretrække.

## Hvad der forlader maskinen {#privacy}

Det, en assistent læser, går til AI-udbyderen bag den: navne på elementer, tidsplaner, kørselshistorik med fejlmeddelelser, id'er og tidspunkter for gendannelsespunkter, navne på databasemotorer og størrelser på dumps, igangværende aktivitet, lagertal, dækning og status. BombVault fjerner værtsstier, repository-placeringer, værtsnavne, legitimationsoplysninger, hook-kommandoer og nøgler, før noget forlader maskinen.

## Fejlfinding {#troubleshooting}

| Hvad du ser | Hvad det betyder |
|---|---|
| `404` | Ingen aktiv nøgle, eller en forkert sti som `/api/mcp`. Endepunktet er `/mcp`. |
| `401` | Nøglen mangler, er stavet forkert, tilbagekaldt eller udskiftet. Måske smider en proxy headeren `Authorization` væk (prøv `X-API-Key`). Markerer kortet nøglen som ikke længere gyldig, er `APP_KEY` ændret: udskift nøglen. |
| `403` | Forespørgslen kom fra en browserside med en anden origin. Brug en skrivebords- eller kommandolinjeklient. |
| `405` ved GET | Normalt. Endepunktet tager kun imod `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Klienten er for gammel til Streamable HTTP. Opdater den. |
| `400` "batch requests are not accepted" | Klienten sender JSON-RPC-batches. Send én besked pr. forespørgsel. |
| `429` | For mange forkerte nøgler fra denne adresse, eller mere end 120 forespørgsler i minuttet med én nøgle. Vent et minut, og tjek om assistenten sidder fast i en løkke. |
| Fejl med "certificate", "self-signed" eller "unable to verify" | Klienten stoler ikke på BombVaults certifikat. Se [TLS og certifikater](#tls). |
| `busy` | En anden sikkerhedskopi eller en vedligeholdelsesopgave optager domænet. Prøv igen, når den er færdig. |
| `cooldown` | Dette element, dette domæne eller Backup Everything blev startet via MCP for mindre end 15 minutter siden. |
| `retention_guard` | Endnu en MCP-sikkerhedskopi ville kun efterlade gendannelsespunkter fra MCP i et vindue med "behold de sidste N". Den næste planlagte sikkerhedskopi giver plads, eller start den i webgrænsefladen. |
| `rate_limited` | Nøglen har brugt sine 12 starter for denne time. |
| `not_permitted` ved en start | Nøglen må kun læse. Slå **Må starte sikkerhedskopier** til i kortet; ny forbindelse er ikke nødvendig. Ved en annullering betyder det, at denne nøgle ikke startede kørslen. |
| `domain_off` | Den type sikkerhedskopi er slået fra i indstillingerne. |
| `not_found` | BombVault beskytter ikke det element. Tilføj det først i webgrænsefladen; MCP opretter aldrig konfiguration. |

Sæt ikke miljøvariablen `MCPGODEBUG` på containeren. Den ændrer MCP-bibliotekets opførsel, og en ugyldig værdi stopper BombVault ved start, før den skriver en eneste loglinje.
