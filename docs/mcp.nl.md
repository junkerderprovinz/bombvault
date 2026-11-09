# MCP-server

BombVault heeft een ingebouwde server voor het Model Context Protocol (MCP), het protocol waarmee AI-assistenten zoals Claude Code en Claude Desktop externe hulpmiddelen bereiken. Via die server kan een assistent lezen hoe het met je back-ups gaat en, als je het toestaat, een back-up starten of een back-up annuleren die hij zelf startte. Hij staat uit tot je een sleutel aanmaakt of [aanmelden via OAuth](#oauth) aanzet. Tot dan antwoordt het eindpunt `/mcp` op alles met `404`.

## Wat een assistent wel en niet kan {#tools}

| Tool | Wat het doet | Soort |
|---|---|---|
| `get_health` | Versie, naam van de instantie, of er een back-up loopt en wat deze sleutel mag | lezen |
| `get_status` | Beschermingsstatus per onderdeel: laatste geslaagde back-up, verwacht interval, verificatie en off-site-controles, volgende geplande runs, back-ups die op een rustige app wachten en, voor containers, de nieuwste starttest | lezen |
| `get_coverage` | Wat BombVault beschermt en wat niet, telkens met de reden | lezen |
| `list_items` | Elke beschermde container, VM, mappenset, de flash en de appconfiguratie, met de planning, wat een back-up ervan stopt, de laatste back-up en hoe lang die duurde. Databasecontainers vermelden ook hun laatste dump. ZFS-datasets staan er ook in, met de uitkomst van hun laatste controle. Elk item draagt zijn laatste herstelcontrole, en een container zijn laatste starttest of de reden waarom hij niet te testen is. Een container die sinds zijn laatste back-up met andere instellingen opnieuw is aangemaakt, noemt wat er veranderd is | lezen |
| `list_runs` | Runhistorie, nieuwste eerst, te filteren op onderdeel, item, status, soort en tijd. Een trage back-up die door één ding werd afgeremd, noemt dat | lezen |
| `list_restore_points` | Herstelpunten van één item uit zijn primaire repository, en bij een container ook zijn databasedumps. Een ZFS-dataset krijgt één herstelpunt per back-up, met een snapshot van elke dataset eronder | lezen |
| `get_activity` | Wat er nu loopt, met fase en percentage | lezen |
| `get_storage_stats` | Groottegeschiedenis van het primaire repository van één onderdeel en zijn groei per week, en de gebruikte, vrije en totale ruimte op de schijf of remote van elk van zijn repository's | lezen |
| `get_size_breakdown` | Welke mappen en bestanden de ruimte innemen in de nieuwste back-up van één container, VM of mappenset, en hoeveel van elk de laatste back-up erbij bracht | lezen |
| `search_files` | Zoekt bestanden en mappen op naam in de back-ups van alle items, van één onderdeel of van één item (VM's worden niet doorzocht) en toont bij elke vondst elke back-up die hem bevat | lezen |
| `list_anomalies` | Anomalieën die BombVault in de back-ups opmerkte, te filteren op status, ernst en onderdeel, met een samenvatting van wat openstaat | lezen |
| `get_anomaly` | Eén van die bevindingen, met de notitie die is achtergelaten toen ze werd bevestigd | lezen |
| `start_backup` | Maakt nu een back-up van één item | starten |
| `start_domain_backup` | Maakt een back-up van elk beschermd item van één onderdeel | starten |
| `start_backup_everything` | Start de ronde Nu alles back-uppen | starten |
| `cancel_backup` | Annuleert een lopende back-up die deze sleutel startte | annuleren |

Dit blijft in de webinterface: herstellen van elk soort (ook het downloaden, opslaan of importeren van een databasedump), back-ups verwijderen, opschonen, ontgrendelen, controles en oefeningen, off-site-replicatie, instellingen, inloggegevens en MCP-sleutels, en het annuleren van een back-up die de planning, de webinterface of een andere sleutel startte. Hetzelfde geldt voor het bevestigen van een anomalie of het markeren ervan als verwacht, wat op de pagina **Anomalieën** gebeurt. De reden is dat de uitvoer van een tool namen en foutmeldingen van je server bevat, en elk daarvan kan tekst dragen die is geschreven om de assistent te sturen. Een assistent die voor zulke tekst valt, kan hoogstens een back-up starten binnen de onderstaande limieten, of er een annuleren die hij zelf startte.

Is het primaire repository van een item extern (S3, REST, SFTP, rclone), dan neemt `list_restore_points` er contact mee op, dus de aanroep kan even duren. Off-site-kopieën kunnen niet via MCP worden getoond. `search_files` leest externe repository's, off-site-kopieën inbegrepen, alleen als de assistent erom vraagt, omdat dat langer duurt en sommige aanbieders de gedownloade data in rekening brengen. Een zoekopdracht kan minuten duren, dus de eerste aanroep start hem op de achtergrond en een latere aanroep haalt het resultaat op. Wat de anomaliecontroles bekijken, staat onder [Anomalieën](anomalies.md), en hoe een ZFS-item één snapshot per dataset bewaart onder [ZFS-datasets](zfs-datasets.md#contents).

## Wat een gestarte back-up doet {#starting-backups}

De back-up van een assistent is dezelfde back-up die de webinterface start. Een draaiende container wordt gestopt tot zijn back-up klaar is, samen met de containers die met hem moeten stoppen. Een VM met de methode netjes afsluiten wordt afgesloten en weer gestart. Een ZFS-dataset stopt de containers die ervoor zijn ingesteld terwijl zijn snapshot wordt gemaakt. Mappensets, de flash en de configuratie blijven draaien. Daarna past BombVault het bewaarbeleid toe en kopieert het eventueel naar het off-site-repository. `list_items` vertelt de assistent wat een item stopt en hoe lang zijn laatste back-up duurde, en de toolbeschrijvingen vragen hem het je te zeggen voordat hij iets start.

Omdat een back-up dingen stopt en oude herstelpunten laat vallen, zijn starts via MCP beperkt:

- 12 gestarte back-ups per uur per sleutel.
- 15 minuten tussen twee MCP-starts van hetzelfde item, onderdeel of Nu alles back-uppen.
- Hoogstens 4 MCP-starts van hetzelfde item in 24 uur.
- **Bewaarbescherming.** Houdt een onderdeel een vast aantal herstelpunten (alleen "laatste N bewaren", geen dagelijkse, wekelijkse of maandelijkse regel, lokaal of op een off-site-bestemming), dan duwt elke nieuwe back-up de oudste eruit. BombVault weigert dan een MCP-start van een item waarvan de nieuwste N-1 geslaagde back-ups allemaal via MCP zijn gestart. Zo blijft altijd minstens één herstelpunt dat de planning of jij maakte in de bewaarde set. Bij "laatste 1 bewaren" kan een assistent dat item helemaal niet back-uppen. De volgende geplande back-up maakt weer ruimte. Een jaarlijkse regel op zich telt als "laatste 1 bewaren", omdat hij één herstelpunt voor het lopende jaar bewaart.

Een start van een onderdeel of Nu alles back-uppen laat de items weg die een limiet tegenhoudt en noemt ze in zijn antwoord. De webinterface en de planning worden hierdoor niet beperkt. Het uurbudget leeft in het geheugen, dus een herstart van BombVault zet het terug.

Starts via de [API](api.md#errors) en vanuit [Home Assistant](api.md#home-assistant) tellen per item mee voor dezelfde limieten als starts via MCP, en voor de bewaarbescherming.

## Inschakelen {#switch-on}

1. Open **Instellingen, Integraties, MCP-server**, klik op **Kies een client** en kies de jouwe in het venster. Het zoekveld rechtsboven vindt hem op naam. Een client die er niet in staat, verbindt via **Andere client**.
2. Houd onder **Sleutel** **Nieuwe sleutel** aan met de naam die wordt voorgesteld, de naam van de client, of typ er een die zegt waar de sleutel wordt gebruikt, bijvoorbeeld "Claude Code op de laptop". Met één sleutel per client kun je er één intrekken zonder de andere aan te raken. **Bestaande sleutel** geeft de client een sleutel die je eerder maakte.
3. Zet **Back-ups laten starten** aan voor een sleutel die back-ups moet kunnen starten. Zonder die schakelaar kan de sleutel alleen lezen. Je kunt dat later wijzigen in de **Details** van de sleutel, en de wijziging geldt voor het volgende verzoek van de assistent zonder opnieuw te verbinden.
4. Klik op **Sleutel aanmaken**. De sleutel wordt één keer getoond. BombVault bewaart er alleen een vingerafdruk van en kan hem niet opnieuw tonen, dus kopieer hem nu. Sluit je het venster voordat de client de sleutel heeft gebruikt, dan blijft de kaart hem tonen tot je bevestigt dat je hem hebt gekopieerd.

Zonder inlogwachtwoord staat de webinterface zelf open voor iedereen op je netwerk, en wie haar kan openen, kan ook een sleutel aanmaken. De kaart zegt dat. Open je BombVault onder een naam die publiek lijkt (bijvoorbeeld `bombvault.example.com` via een reverse proxy) en is er geen inlogwachtwoord ingesteld, dan kunnen vanaf dat adres geen sleutels worden aangemaakt of vervangen, zodat geen enkele webpagina op internet je browser er een kan laten aanmaken. Stel een inlogwachtwoord in, of open BombVault via het IP-adres of een lokale naam zoals `tower` of `tower.local`.

## Je sleutels en hun logboek {#keys}

Elke sleutel heeft een eigen rij onder **Sleutels en aangemelde clients**. Ze toont de naam van de sleutel, of hij back-ups mag starten of alleen mag lezen, de laatste vier tekens van de sleutel, wanneer hij is aangemaakt of voor het laatst vervangen en wanneer een client hem voor het laatst gebruikte. **Details** opent een venster waarin je de sleutel hernoemt, zijn rechten wijzigt, hem vervangt of intrekt, en dat laat zien hoeveel aanroepen hij vandaag deed. Een ingetrokken sleutel verhuist naar de lijst met ingetrokken sleutels, waar je hem definitief kunt verwijderen zodra geen run in de geschiedenis hem meer noemt.

Naast zijn naam toont de rij het merk van de client waarvoor de sleutel is gemaakt. Een sleutel die via **Andere client** is gemaakt, of voordat de kaart clients toonde, toont in plaats daarvan een sleutel.

Het **Logboek** in de details van een sleutel toont wat die sleutel deed, en een ingetrokken sleutel houdt een eigen knop **Logboek**. De back-ups die hij startte, staan bovenaan, elk met zijn status en een link naar die run in het activiteitenlogboek op het dashboard. Daaronder staan zijn aanroepen, nieuwste eerst, met de tool en wat er van de aanroep werd. Een weigering zegt waarom: de sleutel mag alleen lezen, de bewaarbescherming hield de back-up tegen, er liep al een andere back-up, hetzelfde item, onderdeel of dezelfde ronde was een paar minuten eerder buiten de webinterface gestart, of de sleutel stuurde te veel verzoeken. Een annulering linkt naar de run waar ze over ging.

BombVault bewaart de vermeldingen van elke sleutel tot 30 dagen: de nieuwste 500 geslaagde starts en annuleringen, en daarnaast de nieuwste 200 andere aanroepen (lezen, weigeringen en fouten). Een assistent die een lopende back-up blijft bevragen, of een geweigerde aanroep blijft herhalen, kan de start van die back-up niet uit het logboek duwen. Per aanroep slaat het de tool op, de uitkomst en de run die een annulering noemde. Het slaat nooit op wat de assistent stuurde, en nooit de sleutel of zijn vingerafdruk. De diagnosebundel telt de vermeldingen alleen, en een export van instellingen laat ze weg.

## Een client verbinden {#clients}

**Kies een client** opent een venster met elke client als tegel, onder **Op deze computer** of **In de cloud**, en een zoekveld rechtsboven. Een tegel opent de installatie van de client in hetzelfde venster, in drie stappen: de sleutel, de configuratie voor die client, met het adres waarop je de kaart opende, een knop om haar te kopiëren, waar de configuratie staat en, met het eigen certificaat van BombVault, wat de client nodig heeft om het te vertrouwen. En het wachten op de eerste aanroep van de client. Het venster houdt het laatste gebruik van de sleutel in de gaten en wordt groen zodra die aanroep binnenkomt.

De installatie houdt de sleutel van elke opdrachtregel af. Waar de client hem uit een omgevingsvariabele (`BOMBVAULT_MCP_KEY`), een afgeschermde prompt of een eigen bestand kan lezen, noemt de configuratie hem alleen. Waar de client zo'n weg niet heeft, staat de sleutel in zijn configuratiebestand of instellingen, en de installatie zegt dat. Waar de documentatie van een client niet zegt hoe hij met een onbekend certificaat omgaat, schrijft de installatie die stap op als wat je moet doen als de client het certificaat van BombVault weigert.

| Client | Installatie | Waar de sleutel vandaan komt |
|---|---|---|
| AnythingLLM | configuratiebestand | het configuratiebestand |
| Antigravity | configuratiebestand | omgevingsvariabele |
| Claude Code | opdracht | sleutelbestand |
| Claude Desktop | configuratiebestand | sleutelbestand |
| Cline | configuratiebestand | het configuratiebestand |
| Codex CLI | configuratiebestand | omgevingsvariabele |
| Continue | configuratiebestand | `~/.continue/.env` |
| Copilot CLI | configuratiebestand | het configuratiebestand |
| Cursor | configuratiebestand | omgevingsvariabele |
| Gemini CLI | configuratiebestand | omgevingsvariabele |
| GitHub Copilot (VS Code) | configuratiebestand | afgeschermde prompt |
| Goose | configuratiebestand | omgevingsvariabele |
| Jan | formulier in de app | de instellingen van de app |
| JetBrains (AI Assistant, Junie) | configuratiebestand | het configuratiebestand |
| Kimi Code | configuratiebestand | het configuratiebestand |
| LM Studio | configuratiebestand | het configuratiebestand |
| Mistral Vibe | configuratiebestand | omgevingsvariabele |
| Msty | formulier in de app | de instellingen van de app |
| n8n | formulier in de app | de credentialopslag van n8n |
| Open WebUI | formulier in de app | de instellingen van de app |
| opencode | configuratiebestand | omgevingsvariabele |
| Perplexity (Mac) | formulier in de app | sleutelbestand |
| Qwen Code | configuratiebestand | omgevingsvariabele |
| Roo Code | configuratiebestand | omgevingsvariabele |
| Visual Studio | configuratiebestand | het configuratiebestand |
| Warp | configuratiebestand | het configuratiebestand |
| Windsurf | configuratiebestand | omgevingsvariabele |
| Zed | configuratiebestand | het configuratiebestand |
| Grok | formulier, in de cloud | de servers van de leverancier |
| Le Chat | formulier, in de cloud | de servers van de leverancier |
| ChatGPT | aanmelden via OAuth, in de cloud | een toegangstoken, zie [hieronder](#oauth) |
| Claude (claude.ai) | aanmelden via OAuth, in de cloud | een toegangstoken, zie [hieronder](#oauth) |

De secties hieronder leggen de installaties van Claude Code en Claude Desktop uitgebreider uit en noemen wat elke andere client nodig heeft.

### Claude Code {#claude-code}

Claude Code bereikt BombVault via `mcp-remote`, dat Node.js op die computer nodig heeft. Sla eerst de sleutel op in een eigen tekstbestand, als één regel:

```text
X-API-Key: <your key>
```

Voer daarna de opdracht van de kaart één keer uit in een terminal, met het pad van dat bestand ingevuld. Achter een certificaat dat je computer vertrouwt, ziet ze er zo uit:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

Met het eigen certificaat van BombVault (zie [TLS en certificaten](#tls)) wijst de opdracht Node.js ook naar het gedownloade certificaat:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Controleer de verbinding met `/mcp` in Claude Code. `--scope user` maakt BombVault beschikbaar in al je projecten. Claude Code bewaart alleen het pad van het sleutelbestand, dus de sleutel verschijnt niet in de opdracht en je shellgeschiedenis en ook niet in de proceslijst. Houd het bestand op een plek waar alleen jij het kunt lezen, en buiten elke map die je commit. `@latest` laat `npx` een actuele `mcp-remote` ophalen. Een oudere die globaal is geïnstalleerd, zou anders worden gebruikt, en die kent `--header-file` niet.

Schrijf `${BOMBVAULT_MCP_KEY}` niet in de argumenten van `mcp-remote` voor Claude Code. Claude Code vult zo'n verwijzing in uit zijn eigen omgeving voordat het `mcp-remote` start, dus de sleutel belandt op de opdrachtregel van dat proces, waar andere programma's en gebruikers van de computer hem kunnen lezen.

Zonder Node.js, en alleen achter een certificaat dat je computer vertrouwt, kan Claude Code zelf verbinden. Zet een `.mcp.json` in de projectmap:

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

Stel `BOMBVAULT_MCP_KEY` in waar Claude Code start, bijvoorbeeld onder `"env"` in `~/.claude/settings.json` of in je shellprofiel, bewerkt in een teksteditor in plaats van op de prompt getypt. Hier is de verwijzing veilig, omdat Claude Code geen tweede proces start dat haar zou dragen. Het eigen certificaat van BombVault werkt niet op deze manier: de eigen verbinding van Claude Code weigert het, ook met `NODE_EXTRA_CA_CERTS` ingesteld. Commit nooit een `.mcp.json` waarin de sleutel is uitgeschreven.

### Claude Desktop {#claude-desktop}

Claude Desktop bereikt BombVault via `mcp-remote`, dat Node.js op die computer nodig heeft. Sla eerst de sleutel op in een eigen tekstbestand, als één regel, zoals beschreven bij [Claude Code](#claude-code). Open het configuratiebestand via **Settings, Developer, Edit Config** in Claude Desktop. Het staat op `%APPDATA%\Claude\claude_desktop_config.json` op Windows en op `~/Library/Application Support/Claude/claude_desktop_config.json` op macOS. Voeg de vermelding van de kaart toe binnen `"mcpServers"`, naast servers die er al staan, en herstart Claude Desktop:

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

- `NODE_EXTRA_CA_CERTS` staat er alleen voor het eigen certificaat van BombVault. Laat het weg achter een certificaat dat je computer al vertrouwt.
- `--allow-http` wordt alleen toegevoegd voor een gewoon `http://`-adres.
- Schrijf op Windows de paden met schuine strepen vooruit, bijvoorbeeld `C:/Users/sam/bombvault-key.txt`, omdat één backslash geen geldige JSON is. Houd het pad van het sleutelbestand vrij van spaties: Claude Desktop op Windows geeft een pad met een spatie in twee stukken aan `npx` door.
- De configuratie noemt alleen het sleutelbestand, dus de sleutel verschijnt er niet in en ook niet in de proceslijst. Houd het bestand op een plek waar alleen jij het kunt lezen.

### Clients in de cloud {#cloud-clients}

ChatGPT, Claude op claude.ai, Grok en Le Chat roepen BombVault aan vanaf de servers van hun leveranciers, dus BombVault moet vanaf internet bereikbaar zijn met een publiek vertrouwd certificaat, bijvoorbeeld achter een reverse proxy. Le Chat weigert zelfondertekende certificaten. Een login op de proxy kan de webinterface beschermen, maar `/mcp` moet er zonder doorheen naar BombVault: deze diensten kunnen niet bij een proxy inloggen, en BombVault controleert hun sleutel of token zelf. Grok en Le Chat sturen een vaste sleutel, en hun knoppen stellen ze in zoals de andere. ChatGPT, en Claude op claude.ai in de meeste organisaties, verbinden alleen via aanmelden met OAuth, hierna beschreven.

### Aanmelden via OAuth {#oauth}

Voor een client die geen sleutel kan nemen, is BombVault zijn eigen OAuth-autorisatieserver. De client registreert zichzelf, stuurt je naar een pagina van BombVault, en daar meld je je aan met je inlogwachtwoord (en de tweede factor, als je die hebt ingesteld) en sta je hem toe. De client krijgt dan een token dat alleen werkt voor het MCP-eindpunt van deze BombVault, en vernieuwt het zelf.

1. Stel een inlogwachtwoord in onder **Instellingen, Beveiliging**. Zonder biedt BombVault helemaal geen aanmelding aan, omdat er niemand zou zijn om om toestemming te vragen.
2. Maak BombVault vanaf internet bereikbaar via https met een certificaat dat browsers vertrouwen, meestal via een reverse proxy. De client roept `/mcp`, `/oauth/` en `/.well-known/` aan vanaf zijn eigen servers, dus een proxy met een eigen login moet die drie paden naar BombVault doorlaten. De toestemmingspagina op `/oauth/authorize` opent in je eigen browser en mag achter de proxylogin blijven. Noem de proxy ook in `TRUSTED_PROXY` (zie [Configuratie](configuration.md)). BombVault beperkt clientregistraties per adres, en zonder dat lijkt elke client van de proxy te komen.
3. Schakel op de MCP-kaart **Aanmelden via OAuth** in en vul het **Openbaar adres** in: het https-adres zonder pad, bijvoorbeeld `https://backup.example.com`. Elk token is aan dit adres gebonden, dus na een wijziging moet elke client zich opnieuw aanmelden.
4. Kies ChatGPT of Claude onder **Kies een client**. Het venster toont de **Connector-URL**, het openbare adres plus `/mcp`, en waar die in die client komt. Schakel in ChatGPT Developer mode in onder **Settings, Apps & Connectors, Advanced settings**, kies **Create**, plak de connector-URL als URL van de MCP-server en kies OAuth als authenticatie. Open op claude.ai **Settings, Connectors, Add custom connector**, plak de connector-URL, laat OAuth client ID en secret leeg en kies **Connect**.
5. De client opent de toestemmingspagina. Die toont wie het vraagt, waarheen je antwoord je terugstuurt en de schakelaar **Back-ups laten starten**, die uit begint te staan. Kies **Toestaan** of **Weigeren**.

Elke client die zich aanmeldde, krijgt een eigen rij in dezelfde lijst, met zijn merk. Zijn **Details** bevatten zijn logboek, **Intrekken** en **Back-ups laten starten**, en hij heeft dezelfde limieten als een sleutel. Intrekken werkt meteen. Meldt dezelfde client zich opnieuw aan, dan vervangt zijn nieuwe toekenning de oude, en een toekenning die 30 dagen door niemand is gebruikt, verloopt. Er kunnen tot 10 clients tegelijk zijn aangemeld, bovenop de 10 sleutels.

De toestemmingspagina accepteert een verzoek alleen van een geregistreerde client die precies een van zijn geregistreerde terugkeeradressen noemt: https, of een loopback-adres op een willekeurige poort voor een client op je eigen computer. Alleen de authorization code flow met PKCE (S256) wordt geaccepteerd, en je antwoord is aan je sessie gebonden, zodat geen andere website het voor je kan sturen. Toegangstokens gelden een uur. Een refresh-token wordt bij elk gebruik vervangen, en een token dat daarna opnieuw opduikt, trekt de toekenning in, omdat iemand anders een kopie heeft. Een client die zijn laatste vernieuwing binnen 30 seconden herhaalt, omdat het antwoord hem nooit bereikte, krijgt nieuwe tokens. BombVault haalt geen metadatadocumenten van clients van internet op, dus clients registreren zich via dynamic client registration.

### Andere clients {#other-clients}

Elke client die Streamable HTTP spreekt, werkt:

- URL: het adres van de webinterface plus `/mcp`, bijvoorbeeld `https://192.168.1.10:3443/mcp`.
- De sleutel in `Authorization: Bearer <key>` of in `X-API-Key: <key>`. Worden beide gestuurd, dan moeten ze dezelfde sleutel dragen.
- `POST` met `Content-Type: application/json` en `Accept: application/json, text/event-stream`.
- Eén JSON-RPC-bericht per verzoek. Batches worden geweigerd.
- Protocolversies 2026-07-28, 2025-11-25, 2025-06-18 en 2025-03-26.

## TLS en certificaten {#tls}

BombVault serveert HTTPS met een certificaat dat het zelf maakte, en dat certificaat noemt eerst alleen `localhost`, `127.0.0.1` en `::1`. Claude Code en `mcp-remote` weigeren het op een LAN-adres. De manieren eromheen, in de volgorde die bij de meeste Unraid-installaties past:

1. **Voeg het adres toe in de MCP-kaart.** Wordt de kaart via HTTPS geopend op een adres dat het certificaat niet noemt, dan zegt ze dat en biedt ze **Dit adres aan het certificaat toevoegen** aan. BombVault geeft zijn certificaat opnieuw uit met dat adres erbij (je browser waarschuwt nog een keer, zoals de eerste keer). Klik daarna op **Certificaat downloaden**. De voorbeelden zetten `NODE_EXTRA_CA_CERTS` op het gedownloade bestand, zodat de client precies dit certificaat vertrouwt. Dat betekent ook dat elke client die met een eerdere download is ingesteld, niet meer verbindt zodra het certificaat opnieuw wordt uitgegeven, op deze computer en op elke andere, tot hij het nieuwe bestand krijgt.
2. **Een reverse proxy met een vertrouwd certificaat** (Nginx Proxy Manager, SWAG, Caddy, Traefik). De client ziet dan het certificaat van de proxy en heeft niets extra's nodig, en de kaart waarschuwt niet voor het eigen certificaat van BombVault.
3. **Tailscale.** `tailscale serve` voor de container, of de Unraid Tailscale-integratie, geeft je een `ts.net`-naam met een vertrouwd certificaat.
4. **`HTTP_ONLY=true`**, alleen achter een proxy die TLS beëindigt of op een netwerk dat je volledig vertrouwt. Het schakelt de hele webinterface om naar gewoon HTTP, vraagt een wijziging in de containerinstellingen en stuurt de sleutel onversleuteld.

Zet nooit `NODE_TLS_REJECT_UNAUTHORIZED=0`. Het schakelt de certificaatcontrole uit voor alles waarmee dat Node.js-proces praat.

Een reverse proxy moet de header `Authorization` (of `X-API-Key`) doorgeven, wat proxy's doen tenzij je anders zegt, en mag `/mcp` niet bufferen of herschrijven. Een locatieblok voor Nginx of Nginx Proxy Manager dat ook het certificaat van BombVault controleert:

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

Achter een proxy draagt elk verzoek het adres van de proxy. Vijf foute sleutels van één verkeerd ingestelde client sluiten dan elke MCP-client achter die proxy een minuut buiten. Noem de proxy in `TRUSTED_PROXY` (zie [Configuratie](configuration.md)) om per client te tellen.

## Beveiligingsmodel {#security}

- Zonder actieve sleutel en met aanmelden via OAuth uit antwoordt `/mcp` met `404`.
- Aanmelden via OAuth wordt alleen aangeboden zolang er een inlogwachtwoord is ingesteld. Tokens, codes en clientgeheimen worden alleen als vingerafdruk opgeslagen, en een token werkt alleen voor het adres waarvoor het is uitgegeven.
- Een client kan zich hoogstens 10 keer per uur vanaf één adres registreren, en BombVault bewaart hoogstens 100 geregistreerde clients waar niemand mee aanmeldde, elk een dag. Foute codes en refresh-tokens tellen mee voor dezelfde blokkade als foute sleutels.
- Toekenningen gedragen zich als sleutels wanneer een configuratieback-up wordt hersteld of `APP_KEY` verandert: na een herstel moet elke client zich opnieuw aanmelden.
- Geen vrijgestelde adressen. Verzoeken van `localhost`, de Unraid-host, een reverse proxy of `tailscale serve` hebben een sleutel nodig zoals alle andere, ook als de webinterface geen inlogwachtwoord heeft.
- Sleutels worden alleen als vingerafdruk opgeslagen, één keer getoond, en kunnen worden hernoemd, vervangen en ingetrokken. Tot 10 actieve sleutels, elk met een eigen schakelaar **Back-ups laten starten**.
- Elke aanmaak, vervanging, rechtenwijziging en intrekking stuurt een melding via je meldingskanalen, met het adres waarvandaan het kwam, tenzij meldingen zijn uitgeschakeld.
- 5 foute sleutels per minuut per adres, daarna `429`. 120 verzoeken per minuut en 12 gestarte back-ups per uur per sleutel, plus de afkoeltijd en de bewaarbescherming hierboven.
- Verzoeken van een browserpagina met een andere oorsprong worden geweigerd.
- Sleutels kunnen niet worden aangemaakt vanaf een hostnaam die publiek lijkt zolang er geen inlogwachtwoord is ingesteld.
- Elke back-up die een assistent start, en de opschoon- en off-site-runs die hij veroorzaakt, is gemarkeerd met "via MCP" en de naam van de sleutel in het activiteitenlogboek, in het foutpaneel en in de back-upmelding.
- Elke toolaanroep wordt in het containerlogboek geschreven met de id en de laatste vier tekens van de sleutel (nooit zijn naam) en geteld in `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Het herstellen van een configuratieback-up trekt elke sleutel in, omdat de herstelde database sleutels kan bevatten die je na het opslaan hebt ingetrokken. Maak daarna nieuwe sleutels aan.
- Een sleutel werkt niet meer als `APP_KEY` verandert (een herinstallatie, of een herstel op een andere container). De kaart merkt dat op en markeert de sleutel, en **Sleutel vervangen** geeft hem weer een werkend geheim.
- Behandel een sleutel als een wachtwoord. Een client die de sleutel niet uit een omgevingsvariabele, een prompt of een sleutelbestand kan lezen, bewaart hem leesbaar in zijn configuratie of instellingen, en zijn installatie zegt dat. Geef de voorkeur aan een sleutel die alleen leest op een computer die je minder vertrouwt.

## Wat de server verlaat {#privacy}

Wat een assistent ook leest, gaat naar de AI-aanbieder erachter: itemnamen, planningen, runhistorie met foutmeldingen, id's en tijden van herstelpunten, namen van database-engines en dumpgroottes, de huidige activiteit, opslagcijfers, dekking en status. Een bestandszoekopdracht stuurt ook de gevonden bestanden mee: hun namen, groottes en paden binnen de back-up. BombVault verwijdert hostpaden, repositorylocaties, hostnamen, inloggegevens, hookopdrachten en sleutels voordat er iets vertrekt.

## Problemen oplossen {#troubleshooting}

| Wat je ziet | Wat het betekent |
|---|---|
| `404` | Geen actieve sleutel en aanmelden via OAuth staat uit, of een verkeerd pad zoals `/api/mcp`. Het eindpunt is `/mcp`. |
| `401` | De sleutel ontbreekt, is verkeerd getypt, ingetrokken of vervangen. Een proxy laat misschien de header `Authorization` weg (probeer `X-API-Key`). Markeert de kaart de sleutel als niet meer geldig, dan is `APP_KEY` veranderd: vervang de sleutel. |
| `403` | Het verzoek kwam van een browserpagina met een andere oorsprong. Gebruik een desktop- of opdrachtregelclient. |
| `405` bij GET | Normaal. Het eindpunt neemt alleen `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | De client is te oud voor Streamable HTTP. Werk hem bij. |
| `400` "batch requests are not accepted" | De client stuurt JSON-RPC-batches. Stuur één bericht per verzoek. |
| `429` | Te veel foute sleutels vanaf dit adres, of meer dan 120 verzoeken per minuut met één sleutel. Wacht een minuut en controleer of de assistent niet in een lus vastzit. |
| Fouten over "certificate", "self-signed" of "unable to verify" | De client vertrouwt het certificaat van BombVault niet. Zie [TLS en certificaten](#tls). |
| `busy` | Een andere back-up of onderhoudstaak houdt dat onderdeel vast. Probeer het opnieuw als die klaar is. |
| `cooldown` | Dit item, onderdeel of Nu alles back-uppen is minder dan 15 minuten geleden buiten de webinterface gestart. |
| `retention_guard` | Nog een MCP-back-up zou in een venster "laatste N bewaren" alleen herstelpunten van MCP overlaten, of het item kreeg in de laatste 24 uur al 4 back-ups via MCP, mislukte en geannuleerde meegeteld. In het eerste geval maakt de volgende geplande back-up ruimte, in het tweede is het item weer vrij 24 uur na de oudste van die back-ups. In beide gevallen kun je het in de webinterface starten. |
| `rate_limited` | De sleutel heeft zijn 12 starts voor dit uur gebruikt. |
| `not_permitted` bij een start | De sleutel mag alleen lezen. Zet **Back-ups laten starten** aan in de kaart. Opnieuw verbinden is niet nodig. Bij een annulering betekent het dat de run niet door deze sleutel is gestart. |
| `domain_off` | Dat onderdeel staat uit onder **Instellingen, Algemeen**. |
| `not_found` | Het item wordt niet door BombVault beschermd. Voeg het eerst toe in de webinterface. MCP maakt nooit configuratie aan. |
| De client kan de autorisatieserver niet vinden | Aanmelden via OAuth staat uit, er is geen inlogwachtwoord ingesteld, of de proxy laat `/.well-known/` niet door naar BombVault. |
| De toestemmingspagina zegt dat het terugkeeradres niet is geregistreerd | De client stuurde een terugkeeradres dat hij niet had geregistreerd. Verwijder de connector in de client en voeg hem opnieuw toe. |
| Een aangemelde client krijgt `401` | Zijn toekenning is ingetrokken, na 30 dagen zonder gebruik verlopen, of het openbare adres is veranderd. De client meldt zich opnieuw aan. |

Zet de omgevingsvariabele `MCPGODEBUG` niet op de container. Ze verandert hoe de MCP-bibliotheek zich gedraagt, en een misvormde waarde stopt BombVault bij het starten voordat het een enkele logregel schrijft.
