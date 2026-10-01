# MCP-server

BombVault har en indbygget server til Model Context Protocol (MCP), den protokol som AI-assistenter som Claude Code og Claude Desktop bruger til at nå eksterne værktøjer. Gennem den kan en assistent læse, hvordan dine sikkerhedskopier har det, og, hvis du tillader det, starte en sikkerhedskopi eller annullere en, den selv har startet. Den er slået fra, indtil du opretter en nøgle eller slår [login via OAuth](#oauth) til: indtil da svarer endepunktet `/mcp` `404` på alt.

## Hvad en assistent kan og ikke kan {#tools}

| Værktøj | Hvad det gør | Art |
|---|---|---|
| `get_health` | Version, instansnavn, om en sikkerhedskopi kører, og hvad denne nøgle må | læse |
| `get_status` | Beskyttelsesstatus pr. domæne: seneste vellykkede sikkerhedskopi, forventet interval, verifikationer og off-site-kontroller, næste planlagte kørsler, backups, der venter på, at appen er i ro, for containere den nyeste starttest | læse |
| `get_coverage` | Hvad BombVault beskytter, og hvad det ikke beskytter, med begrundelsen for hvert | læse |
| `list_items` | Hver beskyttet container, VM og mappesæt, flash-drevet og app-konfigurationen, med tidsplan, hvad en sikkerhedskopi stopper, seneste sikkerhedskopi og hvor lang tid den tog; databasecontainere viser også deres seneste dump; ZFS-datasæt er også med, med resultatet af deres seneste kontrol; hvert element har sit seneste gendannelsestjek, og en container sin seneste starttest eller grunden til, at den ikke kan testes; en container, der er oprettet igen med andre indstillinger siden sin seneste sikkerhedskopi, viser ændringerne | læse |
| `list_runs` | Kørselshistorik, nyeste først, kan filtreres efter domæne, element, status, art og tid; en langsom sikkerhedskopi, som én ting holdt tilbage, nævner den | læse |
| `list_restore_points` | Gendannelsespunkter for ét element fra dets primære repository, og for en container også dens databasedumps; et ZFS-datasæt får ét gendannelsespunkt pr. sikkerhedskopi, med et snapshot af hvert datasæt under det | læse |
| `get_activity` | Hvad der kører lige nu, med fase og procent | læse |
| `get_storage_stats` | Størrelseshistorik for et domænes primære repository og dets vækst pr. uge samt brugt, fri og samlet plads på disken eller fjernlageret for hvert af dets repositories | læse |
| `get_size_breakdown` | Hvilke mapper og filer der fylder i den nyeste sikkerhedskopi af en container, en VM eller et mappesæt, og hvor meget af det den seneste sikkerhedskopi tilføjede | læse |
| `list_anomalies` | Afvigelser, som BombVault har bemærket i sikkerhedskopierne, kan filtreres efter tilstand, alvor og domæne, med en oversigt over det, der er åbent | læse |
| `get_anomaly` | Én af disse afvigelser, med den note, der blev skrevet, da den blev kvitteret | læse |
| `start_backup` | Sikkerhedskopierer ét element med det samme | starte |
| `start_domain_backup` | Sikkerhedskopierer hvert beskyttet element i et domæne | starte |
| `start_backup_everything` | Kører Backup Everything-gennemløbet | starte |
| `cancel_backup` | Annullerer en kørende sikkerhedskopi, som denne nøgle har startet | annullere |

Dette bliver i webgrænsefladen: gendannelser af enhver art (også download, gem og import af et databasedump), sletning af sikkerhedskopier, prune, unlock, kontroller og øvelser, off-site-replikering, indstillinger, legitimationsoplysninger og MCP-nøgler samt annullering af en sikkerhedskopi, som tidsplanen, webgrænsefladen eller en anden nøgle har startet. Det samme gælder for at kvittere for en afvigelse eller markere den som forventet, hvilket sker på siden **Afvigelser**. Grunden er, at værktøjernes svar indeholder navne og fejlmeddelelser fra din server, og hver af dem kan rumme tekst, der er skrevet for at styre assistenten. En assistent, der falder for den slags tekst, kan i værste fald starte en sikkerhedskopi inden for grænserne nedenfor eller annullere en, den selv har startet.

Ligger et elements primære repository et andet sted (S3, REST, SFTP, rclone), kontakter `list_restore_points` det, og kaldet kan tage et stykke tid. Off-site-kopier kan ikke vises via MCP. Hvad afvigelsestjekkene ser på, står under [Funktioner](features.md), og hvordan et ZFS-element har ét snapshot pr. datasæt, under [ZFS-datasæt](zfs-datasets.md#contents).

## Hvad en startet sikkerhedskopi gør {#starting-backups}

En assistents sikkerhedskopi er den samme, som webgrænsefladen starter. En kørende container stoppes, indtil dens sikkerhedskopi er færdig, sammen med de containere, der er sat til at stoppe med den. En VM med metoden "graceful" lukkes ned og startes igen. Et ZFS-datasæt stopper de containere, der er sat op til det, mens dets snapshot tages. Mappesæt, flash-drevet og konfigurationen kører videre. Bagefter anvender BombVault opbevaringspolitikken og kopierer eventuelt til off-site-repositoryet. `list_items` fortæller assistenten, hvad et element stopper, og hvor lang tid dets seneste sikkerhedskopi tog, og værktøjernes beskrivelser beder den sige det til dig, før den starter noget.

Fordi en sikkerhedskopi stopper ting og skubber gamle gendannelsespunkter ud, er starter via MCP begrænsede:

- 12 startede sikkerhedskopier pr. time pr. nøgle.
- 15 minutter mellem to MCP-starter af samme element, samme domæne eller Backup Everything.
- Højst 4 MCP-starter af samme element på 24 timer.
- **Opbevaringsværn.** Når et domæne beholder et fast antal gendannelsespunkter (kun "behold de sidste N", uden daglig, ugentlig eller månedlig regel, lokalt eller på en off-site-destination), skubber hver ny sikkerhedskopi den ældste ud. BombVault afviser så en MCP-start af et element, hvis nyeste N-1 vellykkede sikkerhedskopier alle blev startet via MCP. Der bliver derfor altid mindst ét gendannelsespunkt i det beholdte sæt, som tidsplanen eller du har lavet. Med "behold den sidste 1" kan en assistent slet ikke sikkerhedskopiere det element. Den næste planlagte sikkerhedskopi giver plads igen. En årsregel alene tæller som "behold den sidste 1", fordi den kun beholder ét gendannelsespunkt for det aktuelle år.

En start af et domæne eller af Backup Everything udelader de elementer, som en grænse holder tilbage, og nævner dem i svaret. Webgrænsefladen og tidsplanen er ikke berørt af noget af dette. Timebudgettet ligger i hukommelsen, så en genstart af BombVault nulstiller det.

## Slå det til {#switch-on}

1. Åbn **Indstillinger, Integrationer, MCP-server**, og klik på knappen for din klient. En klient, der ikke står på listen, forbinder via **Anden klient**.
2. Behold under **Nøgle** valget **Ny nøgle** og det foreslåede navn, klientens, eller skriv et, der siger, hvor nøglen bruges, for eksempel "Claude Code på laptoppen". Én nøgle pr. klient lader dig tilbagekalde én uden at røre de andre. **Eksisterende nøgle** giver klienten en nøgle, du har lavet før.
3. Slå **Tillad at starte sikkerhedskopier** til for en nøgle, der skal kunne starte sikkerhedskopier; uden det kan den kun læse. Du kan ændre det senere på nøglens felt, og ændringen gælder fra assistentens næste forespørgsel uden ny forbindelse.
4. Klik på **Opret nøgle**. Nøglen vises én gang. BombVault gemmer kun et fingeraftryk af den og kan ikke vise den igen, så kopiér den nu. Lukker du dialogen, før klienten har brugt nøglen, bliver kortet ved med at vise den, til du bekræfter, at du har kopieret den.

Uden en login-adgangskode er selve webgrænsefladen åben for alle på dit netværk, og alle, der kan åbne den, kan også oprette en nøgle. Kortet siger det. Åbner du BombVault under et navn, der ser offentligt ud (for eksempel `bombvault.example.com` bag en reverse proxy), og er der ingen login-adgangskode, kan der ikke oprettes eller udskiftes nøgler fra den adresse, så ingen webside på internettet kan få din browser til at oprette en. Sæt en login-adgangskode, eller åbn BombVault via dens IP-adresse eller et lokalt navn som `tower` eller `tower.local`.

## Dine nøgler og deres log {#keys}

Hver nøgle har sit eget felt på kortet. Det viser nøglens navn, om den må starte sikkerhedskopier eller kun læse, de sidste fire tegn af nøglen, hvornår den blev oprettet eller sidst udskiftet, hvornår en klient sidst brugte den, og hvor mange kald den har lavet i dag. På feltet omdøber du nøglen, ændrer dens tilladelse, udskifter den eller tilbagekalder den. En tilbagekaldt nøgle flytter til listen over tilbagekaldte nøgler, hvor du kan slette den for altid, når ingen kørsel i historikken nævner den længere.

Ved siden af navnet viser feltet logoet for den klient, nøglen blev lavet til. En nøgle lavet via **Anden klient**, eller før kortet viste klienter, viser en nøgle i stedet.

**Log** på et felt viser, hvad nøglen har gjort. Først kommer de sikkerhedskopier, den startede, hver med sin status og et link til kørslen i aktivitetsloggen på dashboardet. Under dem står dens kald, nyeste først, med værktøjet og hvad der blev af kaldet. En afvisning siger hvorfor: nøglen må kun læse, opbevaringsværnet holdt sikkerhedskopien tilbage, en anden sikkerhedskopi kørte allerede, elementet blev sikkerhedskopieret via MCP for få minutter siden, eller nøglen sendte for mange forespørgsler. En annullering linker til den kørsel, den handlede om.

BombVault gemmer hver nøgles poster i op til 30 dage: de nyeste 500 vellykkede starter og annulleringer og ved siden af dem de nyeste 200 øvrige kald (læsninger, afvisninger og fejl), så en assistent, der gentagne gange spørger til en kørende sikkerhedskopi eller prøver et afvist kald igen og igen, ikke kan skubbe dens start ud af loggen. For hvert kald gemmer det værktøjet, resultatet og den kørsel, en annullering nævnte. Det gemmer aldrig, hvad assistenten sendte, og aldrig nøglen eller dens fingeraftryk. Diagnosepakken tæller kun posterne, og en eksport af indstillingerne udelader dem.

## Tilslut en klient {#clients}

Hver klient har en knap på kortet, under **På denne computer** eller **I skyen**. Knappen åbner en dialog i tre trin: nøglen; konfigurationen til den klient, med den adresse, du åbnede kortet på, en knap til at kopiere den, hvor konfigurationen ligger og, med BombVaults eget certifikat, hvad klienten skal bruge for at stole på det; og ventetiden på klientens første kald. Dialogen følger nøglens seneste brug og bliver grøn, når det kald kommer.

Dialogen holder nøglen væk fra alle kommandolinjer. Hvor klienten kan læse den fra en miljøvariabel (`BOMBVAULT_MCP_KEY`), en skjult forespørgsel eller en fil af sin egen, nævner konfigurationen den kun. Hvor klienten ikke kan det, står nøglen i dens konfigurationsfil eller indstillinger, og dialogen siger det. Hvor en klients dokumentation ikke siger, hvordan den behandler et ukendt certifikat, skriver dialogen det trin som det, du gør, hvis klienten afviser BombVaults certifikat.

| Klient | Opsætning | Hvor nøglen kommer fra |
|---|---|---|
| AnythingLLM | konfigurationsfil | konfigurationsfilen |
| Antigravity | konfigurationsfil | miljøvariabel |
| Claude Code | kommando | nøglefil |
| Claude Desktop | konfigurationsfil | nøglefil |
| Cline | konfigurationsfil | konfigurationsfilen |
| Codex CLI | konfigurationsfil | miljøvariabel |
| Continue | konfigurationsfil | `~/.continue/.env` |
| Copilot CLI | konfigurationsfil | konfigurationsfilen |
| Cursor | konfigurationsfil | miljøvariabel |
| Gemini CLI | konfigurationsfil | miljøvariabel |
| GitHub Copilot (VS Code) | konfigurationsfil | skjult forespørgsel |
| Goose | konfigurationsfil | miljøvariabel |
| Jan | formular i appen | appens indstillinger |
| JetBrains (AI Assistant, Junie) | konfigurationsfil | konfigurationsfilen |
| Kimi Code | konfigurationsfil | konfigurationsfilen |
| LM Studio | konfigurationsfil | konfigurationsfilen |
| Mistral Vibe | konfigurationsfil | miljøvariabel |
| Msty | formular i appen | appens indstillinger |
| n8n | formular i appen | n8n's loginoplysninger |
| Open WebUI | formular i appen | appens indstillinger |
| opencode | konfigurationsfil | miljøvariabel |
| Perplexity (Mac) | formular i appen | nøglefil |
| Qwen Code | konfigurationsfil | miljøvariabel |
| Roo Code | konfigurationsfil | miljøvariabel |
| Visual Studio | konfigurationsfil | konfigurationsfilen |
| Warp | konfigurationsfil | konfigurationsfilen |
| Windsurf | konfigurationsfil | miljøvariabel |
| Zed | konfigurationsfil | konfigurationsfilen |
| Grok | formular, i skyen | udbyderens servere |
| Le Chat | formular, i skyen | udbyderens servere |
| ChatGPT | login via OAuth, i skyen | et adgangstoken, se [nedenfor](#oauth) |
| Claude (claude.ai) | login via OAuth, i skyen | et adgangstoken, se [nedenfor](#oauth) |

Afsnittene nedenfor forklarer opsætningen af Claude Code og Claude Desktop nærmere og nævner, hvad enhver anden klient har brug for.

### Claude Code {#claude-code}

Claude Code når BombVault via `mcp-remote`, som kræver Node.js på den computer. Gem først nøglen i en tekstfil for sig, som en enkelt linje:

```text
X-API-Key: <your key>
```

Kør derefter kommandoen fra kortet én gang i en terminal, med stien til den fil udfyldt. Bag et certifikat, din computer stoler på, ser den sådan ud:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

Med BombVaults eget certifikat (se [TLS og certifikater](#tls)) peger kommandoen også Node.js på det hentede certifikat:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Tjek forbindelsen med `/mcp` inde i Claude Code. `--scope user` gør BombVault tilgængelig i alle dine projekter. Claude Code gemmer kun stien til nøglefilen, så nøglen hverken dukker op i kommandoen og din shell-historik eller i proceslisten. Læg filen et sted, hvor kun du kan læse den, og uden for enhver mappe, du committer. `@latest` får `npx` til at hente en aktuel `mcp-remote`; ellers ville en ældre, globalt installeret version blive brugt, og den kender ikke `--header-file`.

Skriv ikke `${BOMBVAULT_MCP_KEY}` ind i argumenterne til `mcp-remote` for Claude Code. Claude Code udfylder sådan en reference fra sit eget miljø, før den starter `mcp-remote`, så nøglen havner på processens kommandolinje, hvor andre programmer og brugere på computeren kan læse den.

Uden Node.js, og kun bag et certifikat, din computer stoler på, kan Claude Code selv oprette forbindelsen. Læg en `.mcp.json` i projektmappen:

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

Sæt `BOMBVAULT_MCP_KEY`, hvor Claude Code starter, for eksempel under `"env"` i `~/.claude/settings.json` eller i din shell-profil, redigeret i en teksteditor i stedet for skrevet ved prompten. Her er referencen sikker, fordi Claude Code ikke starter nogen ekstra proces, som nøglen kunne havne i. BombVaults eget certifikat virker ikke på denne måde: Claude Codes egen forbindelse afviser det, også når `NODE_EXTRA_CA_CERTS` er sat. Commit aldrig en `.mcp.json`, hvor nøglen står skrevet ud.

### Claude Desktop {#claude-desktop}

Claude Desktop når BombVault via `mcp-remote`, som kræver Node.js på den computer. Gem først nøglen i en tekstfil for sig, som en enkelt linje, som beskrevet under [Claude Code](#claude-code). Åbn konfigurationsfilen i Claude Desktop via **Settings, Developer, Edit Config**. Den ligger i `%APPDATA%\Claude\claude_desktop_config.json` på Windows og i `~/Library/Application Support/Claude/claude_desktop_config.json` på macOS. Tilføj posten fra kortet inde i `"mcpServers"`, ved siden af de servere, der allerede står der, og genstart Claude Desktop:

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

- `NODE_EXTRA_CA_CERTS` er der kun for BombVaults eget certifikat. Bag et certifikat, som din computer allerede stoler på, udelader du det.
- `--allow-http` tilføjes kun for en almindelig `http://`-adresse.
- På Windows skriver du stierne med almindelige skråstreger, fx `C:/Users/sam/bombvault-key.txt`, fordi en enkelt omvendt skråstreg ikke er gyldig JSON. Hold stien til nøglefilen fri for mellemrum: Claude Desktop på Windows sender en sti med mellemrum til `npx` i to stykker.
- Konfigurationen nævner kun nøglefilen, så nøglen hverken vises i den eller i proceslisten. Læg filen et sted, hvor kun du kan læse den.

### Klienter i skyen {#cloud-clients}

ChatGPT, Claude på claude.ai, Grok og Le Chat kalder BombVault fra deres leverandørs servere, så BombVault skal kunne nås fra internettet med et offentligt betroet certifikat, for eksempel bag en reverse proxy; Le Chat afviser selvsignerede. Et login på proxyen kan beskytte webgrænsefladen, men `/mcp` skal nå BombVault uden det: disse tjenester kan ikke logge ind på en proxy, og BombVault tjekker selv deres nøgle eller token. Grok og Le Chat sender en fast nøgle, og deres knapper sætter dem op som de andre. ChatGPT, og i de fleste organisationer også Claude på claude.ai, forbinder kun via login med OAuth, som er beskrevet herunder.

### Login via OAuth {#oauth}

For en klient, der ikke kan tage en nøgle, er BombVault sin egen OAuth-autorisationsserver. Klienten registrerer sig selv, sender dig til en BombVault-side, og der logger du ind med din adgangskode (og den anden faktor, hvis du har sat en op) og giver den lov. Klienten får så et token, der kun gælder for MCP-endepunktet i denne BombVault, og fornyer det selv.

1. Sæt en adgangskode under **Indstillinger, Sikkerhed**. Uden en tilbyder BombVault slet ikke login, for der ville ikke være nogen at spørge om samtykke.
2. Gør BombVault tilgængelig fra internettet over https med et certifikat, som browsere stoler på, som regel via en reverse proxy. Klienten kalder `/mcp`, `/oauth/` og `/.well-known/` fra sine egne servere, så en proxy med sit eget login skal lade de tre stier gå igennem til BombVault. Samtykkesiden på `/oauth/authorize` åbner i din egen browser og må blive bag proxyens login. Angiv også proxyen i `TRUSTED_PROXY` (se [Konfiguration](configuration.md)). BombVault begrænser klientregistreringer pr. adresse, og uden den ser alle klienter ud til at komme fra proxyen.
3. Slå **Login via OAuth** til på MCP-kortet, og indtast den **Offentlige adresse**: https-adressen uden sti, for eksempel `https://backup.example.com`. Hvert token er bundet til denne adresse, så efter en ændring skal hver klient logge ind igen.
4. Klik på knappen for ChatGPT eller Claude. Dialogen viser **Connector-URL**, altså den offentlige adresse med `/mcp` bagefter, og hvor den hører hjemme i den klient. I ChatGPT slår du udviklertilstand til under **Indstillinger, Apps og connectors, Avancerede indstillinger**, vælger **Opret**, indsætter connector-URL'en som MCP-server-URL og vælger OAuth som godkendelse. På claude.ai åbner du **Indstillinger, Connectors, Tilføj brugerdefineret connector**, indsætter connector-URL'en, lader OAuth-klient-id og hemmelighed stå tomme og vælger **Forbind**.
5. Klienten åbner samtykkesiden. Den viser, hvem der spørger, hvor dit svar sender dig tilbage til, og kontakten **Tillad at starte sikkerhedskopier**, som starter slået fra. Vælg **Tillad** eller **Afvis**.

Hver klient, der har logget ind, får en flise ved siden af nøglerne, med sit mærke, sin log, **Tilbagekald** og **Tillad at starte sikkerhedskopier**, og de samme grænser som en nøgle. Tilbagekaldelse virker med det samme. Når den samme klient logger ind igen, erstatter dens nye tilladelse den gamle, og en tilladelse, som ingen har brugt i 30 dage, udløber. Op til 10 klienter kan være logget ind på én gang, ud over de 10 nøgler.

Samtykkesiden tager kun imod en anmodning fra en registreret klient, der angiver præcis en af sine registrerede returadresser: https, eller en loopback-adresse på en vilkårlig port for en klient på din egen computer. Kun authorization code-flowet med PKCE (S256) accepteres, og dit svar er bundet til din session, så ingen anden hjemmeside kan sende det for dig. Adgangstokens gælder en time. Et refresh-token erstattes ved hver brug, og dukker et op igen bagefter, tilbagekalder BombVault tilladelsen, fordi en anden har en kopi. En klient, der gentager sin seneste fornyelse inden for 30 sekunder, fordi svaret aldrig nåede frem, får i stedet nye tokens. BombVault henter ikke klientmetadata fra internettet, så klienter registrerer sig via dynamisk klientregistrering.

### Andre klienter {#other-clients}

Enhver klient, der taler Streamable HTTP, virker:

- URL: webgrænsefladens adresse plus `/mcp`, for eksempel `https://192.168.1.10:3443/mcp`.
- Nøglen i `Authorization: Bearer <key>` eller i `X-API-Key: <key>`. Sendes begge, skal de indeholde samme nøgle.
- `POST` med `Content-Type: application/json` og `Accept: application/json, text/event-stream`.
- Én JSON-RPC-besked pr. forespørgsel; batches afvises.
- Protokolversionerne 2026-07-28, 2025-11-25, 2025-06-18 og 2025-03-26.

## TLS og certifikater {#tls}

BombVault leverer HTTPS med et certifikat, det selv har udstedt, og i starten nævner det certifikat kun `localhost`, `127.0.0.1` og `::1`. Claude Code og `mcp-remote` afviser det på en LAN-adresse. Vejene uden om, i den rækkefølge der passer til de fleste Unraid-installationer:

1. **Tilføj adressen i MCP-kortet.** Åbner du kortet over HTTPS på en adresse, certifikatet ikke nævner, siger kortet det og tilbyder **Tilføj denne adresse til certifikatet**. BombVault udsteder så sit certifikat igen med den adresse (din browser advarer én gang til, ligesom første gang). Klik derefter på **Hent certifikat**; uddragene sætter `NODE_EXTRA_CA_CERTS` til den hentede fil, så klienten stoler på netop det certifikat. Det betyder også, at enhver klient, der er sat op med en tidligere hentet fil, holder op med at forbinde, så snart certifikatet udstedes igen, på denne computer og på alle andre, indtil den får den nye fil.
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

- Uden en aktiv nøgle og med login via OAuth slået fra svarer `/mcp` `404`.
- Login via OAuth tilbydes kun, så længe der er sat en adgangskode. Tokens, koder og klienthemmeligheder gemmes kun som fingeraftryk, og et token gælder kun for den adresse, det blev udstedt til.
- En klient kan højst registrere sig 10 gange i timen fra én adresse, og BombVault gemmer højst 100 registrerede klienter, som ingen har logget ind med, hver i et døgn. Forkerte koder og refresh-tokens tæller med i samme spærring som forkerte nøgler.
- Tilladelser opfører sig som nøgler, når en konfigurationsbackup gendannes, eller `APP_KEY` ændres: efter en gendannelse skal hver klient logge ind igen.
- Ingen adresser er undtaget. Forespørgsler fra `localhost`, Unraid-værten, en reverse proxy eller `tailscale serve` kræver en nøgle som alle andre, også når webgrænsefladen ikke har en login-adgangskode.
- Nøgler gemmes kun som fingeraftryk, vises én gang og kan omdøbes, udskiftes og tilbagekaldes. Op til 10 aktive nøgler, hver med sin egen kontakt **Tillad at starte sikkerhedskopier**.
- Hver oprettelse, udskiftning, ændring af rettigheder og tilbagekaldelse sender en notifikation via dine notifikationskanaler med den adresse, den kom fra, medmindre notifikationer er slået fra.
- 5 forkerte nøgler pr. minut pr. adresse, derefter `429`. 120 forespørgsler pr. minut og 12 startede sikkerhedskopier pr. time pr. nøgle, plus ventetiden og opbevaringsværnet ovenfor.
- Forespørgsler fra en browserside med en anden origin afvises.
- Så længe der ikke er en login-adgangskode, kan der ikke oprettes nøgler fra et værtsnavn, der ser offentligt ud.
- Hver sikkerhedskopi, en assistent starter, og de prune- og off-site-kørsler, den fører til, er markeret "via MCP" med nøglens navn i aktivitetsloggen, i fejlpanelet og i notifikationen om sikkerhedskopien.
- Hvert værktøjskald skrives i containerens log med nøglens id og dens sidste fire tegn (aldrig navnet) og tælles i `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Gendannelse af en konfigurationssikkerhedskopi tilbagekalder alle nøgler, fordi den gendannede database kan indeholde nøgler, du tilbagekaldte, efter den blev gemt. Opret nye nøgler bagefter.
- En nøgle holder op med at virke, når `APP_KEY` ændres (en geninstallation eller en gendannelse i en anden container). Kortet opdager det og markerer nøglen, og **Udskift nøgle** giver den en gyldig hemmelighed igen.
- Behandl en nøgle som en adgangskode. En klient, der ikke kan læse nøglen fra en miljøvariabel, en forespørgsel eller en nøglefil, gemmer den i klartekst i sin konfiguration eller sine indstillinger, og dens dialog siger det. Brug hellere en nøgle, der kun må læse, på en computer, du stoler mindre på.

## Hvad der forlader maskinen {#privacy}

Det, en assistent læser, går til AI-udbyderen bag den: navne på elementer, tidsplaner, kørselshistorik med fejlmeddelelser, id'er og tidspunkter for gendannelsespunkter, navne på databasemotorer og størrelser på dumps, igangværende aktivitet, lagertal, dækning og status. BombVault fjerner værtsstier, repository-placeringer, værtsnavne, legitimationsoplysninger, hook-kommandoer og nøgler, før noget forlader maskinen.

## Fejlfinding {#troubleshooting}

| Hvad du ser | Hvad det betyder |
|---|---|
| `404` | Ingen aktiv nøgle, og login via OAuth er slået fra, eller en forkert sti som `/api/mcp`. Endepunktet er `/mcp`. |
| `401` | Nøglen mangler, er stavet forkert, tilbagekaldt eller udskiftet. Måske smider en proxy headeren `Authorization` væk (prøv `X-API-Key`). Markerer kortet nøglen som ikke længere gyldig, er `APP_KEY` ændret: udskift nøglen. |
| `403` | Forespørgslen kom fra en browserside med en anden origin. Brug en skrivebords- eller kommandolinjeklient. |
| `405` ved GET | Normalt. Endepunktet tager kun imod `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Klienten er for gammel til Streamable HTTP. Opdater den. |
| `400` "batch requests are not accepted" | Klienten sender JSON-RPC-batches. Send én besked pr. forespørgsel. |
| `429` | For mange forkerte nøgler fra denne adresse, eller mere end 120 forespørgsler i minuttet med én nøgle. Vent et minut, og tjek om assistenten sidder fast i en løkke. |
| Fejl med "certificate", "self-signed" eller "unable to verify" | Klienten stoler ikke på BombVaults certifikat. Se [TLS og certifikater](#tls). |
| `busy` | En anden sikkerhedskopi eller en vedligeholdelsesopgave optager domænet. Prøv igen, når den er færdig. |
| `cooldown` | Dette element, dette domæne eller Backup Everything blev startet via MCP for mindre end 15 minutter siden. |
| `retention_guard` | Endnu en MCP-sikkerhedskopi ville kun efterlade gendannelsespunkter fra MCP i et vindue med "behold de sidste N", eller elementet har allerede fået 4 sikkerhedskopier via MCP inden for de sidste 24 timer, mislykkede og annullerede medregnet. I det første tilfælde giver den næste planlagte sikkerhedskopi plads, i det andet er elementet fri igen 24 timer efter den ældste af dem. Du kan altid starte den i webgrænsefladen. |
| `rate_limited` | Nøglen har brugt sine 12 starter for denne time. |
| `not_permitted` ved en start | Nøglen må kun læse. Slå **Tillad at starte sikkerhedskopier** til i kortet; ny forbindelse er ikke nødvendig. Ved en annullering betyder det, at denne nøgle ikke startede kørslen. |
| `domain_off` | Den type sikkerhedskopi er slået fra i indstillingerne. |
| `not_found` | BombVault beskytter ikke det element. Tilføj det først i webgrænsefladen; MCP opretter aldrig konfiguration. |
| Klienten kan ikke finde autorisationsserveren | Login via OAuth er slået fra, der er ingen adgangskode, eller proxyen lader ikke `/.well-known/` gå igennem til BombVault. |
| Samtykkesiden siger, at returadressen ikke er registreret | Klienten sendte en returadresse, den ikke har registreret. Fjern connectoren i klienten, og tilføj den igen. |
| En klient, der er logget ind, får `401` | Dens tilladelse er tilbagekaldt, udløbet efter 30 dage uden brug, eller den offentlige adresse er ændret. Klienten logger ind igen. |

Sæt ikke miljøvariablen `MCPGODEBUG` på containeren. Den ændrer MCP-bibliotekets opførsel, og en ugyldig værdi stopper BombVault ved start, før den skriver en eneste loglinje.
