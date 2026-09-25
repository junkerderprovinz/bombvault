# MCP-server

BombVault heeft een ingebouwde server voor het Model Context Protocol (MCP), het protocol waarmee AI-assistenten zoals Claude Code en Claude Desktop externe hulpmiddelen bereiken. Daarmee kan een assistent lezen hoe het met je back-ups staat en, als je dat toestaat, een back-up starten of er een annuleren die hij zelf heeft gestart. De server staat uit tot je een sleutel aanmaakt: zonder actieve sleutel antwoordt het eindpunt `/mcp` op alles met `404`.

## Wat een assistent wel en niet kan {#tools}

| Hulpmiddel | Wat het doet | Soort |
|---|---|---|
| `get_health` | Versie, naam van de instantie, of er een back-up loopt en wat deze sleutel mag | lezen |
| `get_status` | Beschermingsstatus per domein: laatste geslaagde back-up, verwacht interval, verificaties en off-site-controles, volgende geplande runs | lezen |
| `get_coverage` | Wat BombVault beschermt en wat niet, telkens met de reden | lezen |
| `list_items` | Elke beschermde container, VM en mappenset, de flashstick en de app-configuratie, met planning, wat een back-up ervan stopt, de laatste back-up en hoe lang die duurde; databasecontainers vermelden ook hun laatste dump; ZFS-datasets staan er ook in, met de uitkomst van hun laatste controle | lezen |
| `list_runs` | Runhistorie, nieuwste eerst, te filteren op domein, item, status, soort en tijd | lezen |
| `list_restore_points` | Herstelpunten van één item uit zijn primaire repository, en bij een container ook zijn databasedumps; een ZFS-dataset krijgt één herstelpunt per back-up, met een snapshot van elke dataset eronder | lezen |
| `get_activity` | Wat er nu loopt, met fase en percentage | lezen |
| `get_storage_stats` | Groottegeschiedenis van de primaire repository van een domein en de groei per week | lezen |
| `list_anomalies` | Anomalieën die BombVault in de back-ups heeft opgemerkt, te filteren op status, ernst en domein, met een overzicht van wat nog openstaat | lezen |
| `get_anomaly` | Eén van die meldingen, met de notitie die bij het bevestigen is achtergelaten | lezen |
| `start_backup` | Maakt nu een back-up van één item | starten |
| `start_domain_backup` | Maakt een back-up van elk beschermd item van een domein | starten |
| `start_backup_everything` | Start de Backup Everything-ronde | starten |
| `cancel_backup` | Annuleert een lopende back-up die deze sleutel heeft gestart | annuleren |

Dit blijft in de webinterface: herstel van elke soort (ook het downloaden, opslaan of importeren van een databasedump), back-ups verwijderen, prune, unlock, controles en oefeningen, off-site-replicatie, instellingen, inloggegevens en MCP-sleutels, en het annuleren van een back-up die de planning, de webinterface of een andere sleutel heeft gestart. Hetzelfde geldt voor het bevestigen van een anomalie of het markeren ervan als verwacht, wat op de pagina **Anomalieën** gebeurt. De reden: de antwoorden van de hulpmiddelen bevatten namen en foutmeldingen van je server, en in elk daarvan kan tekst staan die bedoeld is om de assistent te sturen. Een assistent die daarin trapt, kan in het ergste geval een back-up starten binnen de grenzen hieronder, of er een annuleren die hij zelf heeft gestart.

Staat de primaire repository van een item elders (S3, REST, SFTP, rclone), dan neemt `list_restore_points` daar contact mee op en kan de aanroep even duren. Off-site-kopieën zijn via MCP niet op te vragen. Waar de anomaliecontroles naar kijken, staat onder [Functies](features.md), en hoe een ZFS-item één snapshot per dataset bijhoudt, onder [ZFS-datasets](zfs-datasets.md#contents).

## Wat een gestarte back-up doet {#starting-backups}

De back-up van een assistent is dezelfde back-up die de webinterface start. Een draaiende container wordt gestopt tot zijn back-up klaar is, samen met de containers die met hem mee moeten stoppen. Een VM met de methode "graceful" wordt afgesloten en weer gestart. Een ZFS-dataset stopt de containers die ervoor zijn ingesteld zolang de snapshot wordt gemaakt. Mappensets, de flashstick en de configuratie blijven draaien. Daarna past BombVault het bewaarbeleid toe en kopieert het eventueel naar de off-site-repository. `list_items` vertelt de assistent wat een item stopt en hoe lang de laatste back-up duurde, en de beschrijvingen van de hulpmiddelen vragen hem dat aan jou te melden voor hij iets start.

Omdat een back-up dingen stilzet en oude herstelpunten eruit duwt, zijn starts via MCP begrensd:

- 12 gestarte back-ups per uur per sleutel.
- 15 minuten tussen twee MCP-starts van hetzelfde item, hetzelfde domein of Backup Everything.
- Hoogstens 4 MCP-starts van hetzelfde item in 24 uur.
- **Bewaarbeveiliging.** Houdt een domein een vast aantal herstelpunten (alleen "laatste N bewaren", zonder dagelijkse, wekelijkse of maandelijkse regel, lokaal of op een off-site-bestemming), dan duwt elke nieuwe back-up de oudste eruit. BombVault weigert dan een MCP-start van een item waarvan de nieuwste N-1 geslaagde back-ups allemaal via MCP zijn gestart. Er blijft dus altijd minstens één herstelpunt in de bewaarde set dat de planning of jij hebt gemaakt. Met "laatste 1 bewaren" kan een assistent van dat item helemaal geen back-up maken. De volgende geplande back-up maakt weer ruimte.

Een start van een domein of van Backup Everything laat de items weg die een grens tegenhoudt en noemt ze in het antwoord. De webinterface en de planning hebben met geen van deze grenzen te maken. Het uurbudget staat in het geheugen, dus een herstart van BombVault zet het op nul.

## Inschakelen {#switch-on}

1. Open **Instellingen, Systeem, MCP-server** en klik op **Nieuwe sleutel**.
2. Geef de sleutel een naam die zegt waar hij wordt gebruikt, bijvoorbeeld "Claude Code op de laptop". Met één sleutel per client kun je er een intrekken zonder de andere aan te raken.
3. Laat **Back-ups laten starten** aan, of zet het uit voor een sleutel die alleen mag lezen. Je kunt dit later op de tegel van de sleutel wijzigen, en de wijziging geldt vanaf het volgende verzoek van de assistent, zonder nieuwe verbinding.
4. Klik op **Sleutel aanmaken**. De sleutel wordt één keer getoond. BombVault bewaart er alleen een vingerafdruk van en kan hem niet opnieuw tonen, dus kopieer hem meteen of neem een van de fragmenten eronder, die dan de echte sleutel bevatten.

Zonder inlogwachtwoord staat de webinterface zelf open voor iedereen in je netwerk, en wie hem kan openen, kan ook een sleutel aanmaken. De kaart zegt dat. Open je BombVault onder een naam die er openbaar uitziet (bijvoorbeeld `bombvault.example.com` achter een reverse proxy) en is er geen inlogwachtwoord ingesteld, dan kunnen vanaf dat adres geen sleutels worden aangemaakt of vervangen, zodat geen webpagina op internet je browser er een kan laten aanmaken. Stel een inlogwachtwoord in, of open BombVault via zijn IP-adres of een lokale naam zoals `tower` of `tower.local`.

## Je sleutels en hun logboek {#keys}

Elke sleutel heeft een eigen tegel op de kaart. Die toont de naam van de sleutel, of hij back-ups mag starten of alleen leest, de laatste vier tekens van de sleutel, wanneer hij is aangemaakt of voor het laatst vervangen, wanneer een client hem voor het laatst gebruikte en hoeveel aanroepen hij vandaag deed. Op de tegel geef je de sleutel een andere naam, wijzig je zijn recht, vervang je hem of trek je hem in. Een ingetrokken sleutel verhuist naar de lijst met ingetrokken sleutels, waar je hem voorgoed kunt verwijderen zodra geen run in de geschiedenis hem nog noemt.

**Logboek** op een tegel opent wat die sleutel deed. Eerst komen de back-ups die hij startte, elk met zijn stand en een link naar die run in het activiteitenlogboek op het dashboard. Daaronder staan zijn aanroepen, nieuwste eerst, met de tool en wat er van de aanroep werd. Een weigering zegt waarom: de sleutel mag alleen lezen, de bewaarbeveiliging hield de back-up tegen, er liep al een andere back-up, het item is een paar minuten geleden via MCP geback-upt, of de sleutel stuurde te veel verzoeken. Een annulering linkt naar de run waar het om ging.

BombVault bewaart de regels van elke sleutel hooguit 30 dagen: de nieuwste 500 starts, annuleringen, weigeringen en fouten en daarnaast de nieuwste 200 geslaagde leesacties, zodat een assistent die een lopende back-up steeds opnieuw opvraagt de start ervan niet uit het logboek kan drukken. Per aanroep bewaart het de tool, de uitkomst en de run die een annulering noemde. Wat de assistent stuurde bewaart het nooit, en de sleutel of zijn vingerafdruk evenmin. Het diagnosepakket telt de regels alleen, en een export van de instellingen laat ze weg.

## Een client koppelen {#clients}

De kaart toont kant-en-klare fragmenten voor het adres waarop je hem hebt geopend: kies je client en kopieer het fragment. Hieronder staat wat de fragmenten doen, plus de vormen die de kaart niet toont.

### Claude Code {#claude-code}

Voer het commando van de kaart één keer uit in een terminal. Met een certificaat dat je computer vertrouwt, ziet het er zo uit:

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

Controleer de verbinding met `/mcp` in Claude Code. `--scope user` bewaart de sleutel in je gebruikersconfiguratie en niet in een projectbestand.

Het commando bevat de sleutel, en je shell bewaart het misschien in zijn geschiedenis. Om dat te vermijden zet je een `.mcp.json` in de projectmap en bewaar je de sleutel in een omgevingsvariabele. Claude Code vult `${BOMBVAULT_MCP_KEY}` in wanneer het het bestand leest:

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

Stel `BOMBVAULT_MCP_KEY` in waar Claude Code start, bijvoorbeeld in je shellprofiel, bewerkt in een teksteditor in plaats van getypt aan de prompt. Commit nooit een `.mcp.json` waarin de sleutel uitgeschreven staat.

Met BombVaults eigen certificaat (zie [TLS en certificaten](#tls)) start het commando van de kaart in plaats daarvan `mcp-remote` en wijst het Node.js op het gedownloade certificaat:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

De enkele aanhalingstekens houden je shell ervan af de variabele in te vullen; dat doet `mcp-remote` zelf. Dezelfde vorm werkt in `.mcp.json`: neem de vermelding voor Claude Desktop hieronder en laat `BOMBVAULT_MCP_KEY` weg uit de `env`, dan komt de sleutel uit je omgeving.

### Claude Desktop {#claude-desktop}

Claude Desktop bereikt BombVault via `mcp-remote`, dat Node.js op die computer nodig heeft. Open het configuratiebestand in Claude Desktop via **Settings, Developer, Edit Config**. Het staat op Windows in `%APPDATA%\Claude\claude_desktop_config.json` en op macOS in `~/Library/Application Support/Claude/claude_desktop_config.json`. Zet de vermelding van de kaart binnen `"mcpServers"`, naast de servers die er al staan, en start Claude Desktop opnieuw:

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

- `NODE_EXTRA_CA_CERTS` staat er alleen voor BombVaults eigen certificaat. Achter een certificaat dat je computer al vertrouwt, laat je het weg.
- `--allow-http` komt er alleen bij voor een gewoon `http://`-adres.
- De header staat er als `X-API-Key:${BOMBVAULT_MCP_KEY}`, zonder spatie na de dubbele punt en met de sleutel in `env`. Op sommige systemen knipt `mcp-remote` een `--header`-waarde bij de eerste spatie af, en een sleutel na een spatie zou verloren gaan.

### Eigen connectors in de instellingen van Claude {#custom-connectors}

Connectors die je in de instellingen van Claude zelf toevoegt (op claude.ai en in de connectorlijst van Claude Desktop) worden nog niet ondersteund. Die connectors worden vanuit de cloud van Anthropic aangesproken, hebben dus een openbaar HTTPS-adres nodig, en melden zich aan via OAuth. Ze kunnen geen vaste sleutel meesturen, en BombVault biedt alleen vaste sleutels, geen OAuth-aanmelding. BombVault daarvoor op internet zetten zou dus niet helpen. Gebruik Claude Code, of Claude Desktop via `mcp-remote` zoals hierboven.

### Andere clients {#other-clients}

Elke client die Streamable HTTP spreekt, werkt:

- URL: het adres van de webinterface plus `/mcp`, bijvoorbeeld `https://192.168.1.10:3443/mcp`.
- De sleutel in `Authorization: Bearer <key>` of in `X-API-Key: <key>`. Komen ze allebei mee, dan moeten ze dezelfde sleutel bevatten.
- `POST` met `Content-Type: application/json` en `Accept: application/json, text/event-stream`.
- Eén JSON-RPC-bericht per verzoek; batches worden geweigerd.
- Protocolversies 2026-07-28, 2025-11-25, 2025-06-18 en 2025-03-26.

## TLS en certificaten {#tls}

BombVault levert HTTPS met een certificaat dat het zelf heeft uitgegeven, en dat certificaat noemt in het begin alleen `localhost`, `127.0.0.1` en `::1`. Claude Code en `mcp-remote` weigeren het op een LAN-adres. De uitwegen, in de volgorde die bij de meeste Unraid-installaties past:

1. **Het adres toevoegen in de MCP-kaart.** Open je de kaart via HTTPS op een adres dat het certificaat niet noemt, dan zegt de kaart dat en biedt ze **Dit adres aan het certificaat toevoegen** aan. BombVault geeft zijn certificaat dan opnieuw uit met dat adres erbij (je browser waarschuwt nog één keer, net als de eerste keer). Klik daarna op **Certificaat downloaden**; de fragmenten zetten `NODE_EXTRA_CA_CERTS` op het gedownloade bestand, zodat de client precies dat certificaat vertrouwt.
2. **Een reverse proxy met een vertrouwd certificaat** (Nginx Proxy Manager, SWAG, Caddy, Traefik). De client ziet dan het certificaat van de proxy en heeft niets extra's nodig, en de kaart waarschuwt niet over dat van BombVault.
3. **Tailscale.** `tailscale serve` voor de container, of de Tailscale-integratie van Unraid, geeft je een `ts.net`-naam met een vertrouwd certificaat.
4. **`HTTP_ONLY=true`**, alleen achter een proxy die TLS afhandelt of in een netwerk dat je volledig vertrouwt. Het zet de hele webinterface op gewoon HTTP, vraagt een wijziging in de containerinstellingen en stuurt de sleutel onversleuteld.

Zet nooit `NODE_TLS_REJECT_UNAUTHORIZED=0`. Dat schakelt de certificaatcontrole uit voor alles waarmee dat Node.js-proces praat.

Een reverse proxy moet de header `Authorization` (of `X-API-Key`) doorgeven, wat proxy's doen tenzij je ze anders opdraagt, en mag `/mcp` niet bufferen of herschrijven. Een location-blok voor Nginx of Nginx Proxy Manager dat ook het certificaat van BombVault controleert:

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

Achter een proxy draagt elk verzoek het adres van de proxy. Vijf verkeerde sleutels van één verkeerd ingestelde client sluiten dan elke MCP-client achter die proxy een minuut lang buiten. Zet de proxy in `TRUSTED_PROXY` (zie [Configuratie](configuration.md)) om per client te tellen.

## Beveiligingsmodel {#security}

- Zonder actieve sleutel antwoordt `/mcp` met `404`.
- Geen uitzonderingen voor adressen. Verzoeken van `localhost`, van de Unraid-host, van een reverse proxy of van `tailscale serve` hebben een sleutel nodig zoals elk ander, ook als de webinterface geen inlogwachtwoord heeft.
- Sleutels worden alleen als vingerafdruk opgeslagen, één keer getoond, en kunnen worden hernoemd, vervangen en ingetrokken. Tot 10 actieve sleutels, elk met een eigen schakelaar **Back-ups laten starten**.
- Elk aanmaken, vervangen, elke rechtenwijziging en elke intrekking stuurt een melding via je meldingskanalen, met het adres waar het vandaan kwam, tenzij meldingen uit staan.
- 5 verkeerde sleutels per minuut per adres, daarna `429`. 120 verzoeken per minuut en 12 gestarte back-ups per uur per sleutel, plus de wachttijd en de bewaarbeveiliging van hierboven.
- Verzoeken van een browserpagina met een andere origin worden geweigerd.
- Zolang er geen inlogwachtwoord is, kunnen er geen sleutels worden aangemaakt vanaf een hostnaam die er openbaar uitziet.
- Elke back-up die een assistent start, en de prune- en off-site-runs die eruit volgen, is gemarkeerd met "via MCP" en de naam van de sleutel: in het activiteitenlogboek, in het foutpaneel en in de back-upmelding.
- Elke aanroep van een hulpmiddel komt in het containerlog met de id van de sleutel en de laatste vier tekens (nooit de naam) en wordt geteld in `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Het herstellen van een configuratieback-up trekt elke sleutel in, omdat de herstelde database sleutels kan bevatten die je na het opslaan ervan hebt ingetrokken. Maak daarna nieuwe aan.
- Een sleutel werkt niet meer als `APP_KEY` verandert (een herinstallatie of een herstel in een andere container). De kaart merkt dat op en markeert de sleutel, en **Sleutel vervangen** geeft hem weer een geldig geheim.
- Behandel een sleutel als een wachtwoord. Claude Code en Claude Desktop bewaren hem als platte tekst in hun configuratie. Neem op een computer die je minder vertrouwt liever een sleutel die alleen mag lezen.

## Wat de machine verlaat {#privacy}

Wat een assistent leest, gaat naar de AI-aanbieder erachter: namen van items, planningen, runhistorie met foutmeldingen, id's en tijden van herstelpunten, namen van database-engines en groottes van dumps, lopende activiteit, opslagcijfers, dekking en status. BombVault haalt hostpaden, repositorylocaties, hostnamen, inloggegevens, hookcommando's en sleutels eruit voordat er iets vertrekt.

## Problemen oplossen {#troubleshooting}

| Wat je ziet | Wat het betekent |
|---|---|
| `404` | Geen actieve sleutel, of een verkeerd pad zoals `/api/mcp`. Het eindpunt is `/mcp`. |
| `401` | De sleutel ontbreekt, is verkeerd getypt, ingetrokken of vervangen. Misschien laat een proxy de header `Authorization` vallen (probeer `X-API-Key`). Markeert de kaart de sleutel als niet meer geldig, dan is `APP_KEY` veranderd: vervang de sleutel. |
| `403` | Het verzoek kwam van een browserpagina met een andere origin. Gebruik een desktop- of opdrachtregelclient. |
| `405` bij GET | Normaal. Het eindpunt neemt alleen `POST` aan. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | De client is te oud voor Streamable HTTP. Werk hem bij. |
| `400` "batch requests are not accepted" | De client stuurt JSON-RPC-batches. Stuur één bericht per verzoek. |
| `429` | Te veel verkeerde sleutels vanaf dit adres, of meer dan 120 verzoeken per minuut met één sleutel. Wacht een minuut en kijk of de assistent in een lus zit. |
| Fouten met "certificate", "self-signed" of "unable to verify" | De client vertrouwt het certificaat van BombVault niet. Zie [TLS en certificaten](#tls). |
| `busy` | Een andere back-up of een onderhoudstaak bezet dat domein. Probeer het opnieuw als die klaar is. |
| `cooldown` | Dit item, dit domein of Backup Everything is minder dan 15 minuten geleden via MCP gestart. |
| `retention_guard` | Nog een MCP-back-up zou in een venster "laatste N bewaren" alleen herstelpunten uit MCP overlaten. De volgende geplande back-up maakt ruimte, of start hem in de webinterface. |
| `rate_limited` | De sleutel heeft zijn 12 starts voor dit uur opgebruikt. |
| `not_permitted` bij een start | De sleutel mag alleen lezen. Zet **Back-ups laten starten** aan in de kaart; een nieuwe verbinding is niet nodig. Bij een annulering betekent het dat deze sleutel de run niet heeft gestart. |
| `domain_off` | Dat soort back-up staat uit in de instellingen. |
| `not_found` | BombVault beschermt dat item niet. Voeg het eerst toe in de webinterface; MCP maakt nooit configuratie aan. |

Zet de omgevingsvariabele `MCPGODEBUG` niet op de container. Die verandert het gedrag van de MCP-bibliotheek, en een onjuiste waarde houdt BombVault bij het starten tegen voordat het ook maar één logregel schrijft.
