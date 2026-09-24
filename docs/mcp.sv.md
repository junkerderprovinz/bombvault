# MCP-server

BombVault har en inbyggd server för Model Context Protocol (MCP), protokollet som AI-assistenter som Claude Code och Claude Desktop använder för att nå externa verktyg. Genom den kan en assistent läsa hur det står till med dina säkerhetskopior och, om du tillåter det, starta en säkerhetskopia eller avbryta en som den själv har startat. Servern är avstängd tills du skapar en nyckel: utan en aktiv nyckel svarar slutpunkten `/mcp` med `404` på allt.

## Vad en assistent kan och inte kan göra {#tools}

| Verktyg | Vad det gör | Typ |
|---|---|---|
| `get_health` | Version, instansnamn, om en säkerhetskopia pågår och vad den här nyckeln får göra | läsa |
| `get_status` | Skyddsstatus per domän: senaste lyckade säkerhetskopia, förväntat intervall, verifieringar och off-site-kontroller, nästa schemalagda körningar | läsa |
| `get_coverage` | Vad BombVault skyddar och vad det inte skyddar, med skälet för varje | läsa |
| `list_items` | Varje skyddad container, VM och mappuppsättning, flashminnet och appkonfigurationen, med schema, vad en säkerhetskopia stoppar, senaste säkerhetskopian och hur lång tid den tog; databascontainrar visar även sin senaste dump | läsa |
| `list_runs` | Körningshistorik, nyaste först, filtrerbar på domän, objekt, status, typ och tid | läsa |
| `list_restore_points` | Återställningspunkter för ett objekt från dess primära repository, och för en container även dess databasdumpar | läsa |
| `get_activity` | Vad som körs just nu, med fas och procent | läsa |
| `get_storage_stats` | Storlekshistorik för en domäns primära repository och dess tillväxt per vecka | läsa |
| `start_backup` | Säkerhetskopierar ett objekt direkt | starta |
| `start_domain_backup` | Säkerhetskopierar varje skyddat objekt i en domän | starta |
| `start_backup_everything` | Kör Backup Everything-genomgången | starta |
| `cancel_backup` | Avbryter en pågående säkerhetskopia som den här nyckeln har startat | avbryta |

Detta stannar i webbgränssnittet: återställningar av alla slag (även att ladda ner, spara eller importera en databasdump), att radera säkerhetskopior, prune, unlock, kontroller och övningar, off-site-replikering, inställningar, inloggningsuppgifter och MCP-nycklar, samt att avbryta en säkerhetskopia som schemat, webbgränssnittet eller en annan nyckel har startat. Skälet är att verktygens svar innehåller namn och felmeddelanden från din server, och vilket som helst av dem kan innehålla text som skrivits för att styra assistenten. En assistent som går på sådan text kan i värsta fall starta en säkerhetskopia inom gränserna nedan eller avbryta en som den själv startat.

Om ett objekts primära repository ligger någon annanstans (S3, REST, SFTP, rclone) kontaktar `list_restore_points` det, och anropet kan ta en stund. Off-site-kopior kan inte listas via MCP.

## Vad en startad säkerhetskopia gör {#starting-backups}

En assistents säkerhetskopia är samma säkerhetskopia som webbgränssnittet startar. En körande container stoppas tills dess säkerhetskopia är klar, tillsammans med de containrar som är inställda att stoppas med den. En VM med metoden "graceful" stängs av och startas igen. Mappuppsättningar, flashminnet och konfigurationen fortsätter att köra. Efteråt tillämpar BombVault lagringspolicyn och kopierar eventuellt till off-site-repositoryt. `list_items` talar om för assistenten vad ett objekt stoppar och hur lång tid dess senaste säkerhetskopia tog, och verktygens beskrivningar ber den säga det till dig innan den startar något.

Eftersom en säkerhetskopia stoppar saker och trycker ut gamla återställningspunkter är starter via MCP begränsade:

- 12 startade säkerhetskopior per timme och nyckel.
- 15 minuter mellan två MCP-starter av samma objekt, samma domän eller Backup Everything.
- Högst 4 MCP-starter av samma objekt på 24 timmar.
- **Lagringsskydd.** När en domän behåller ett fast antal återställningspunkter (bara "behåll de senaste N", utan daglig, veckovis eller månadsvis regel, lokalt eller på ett off-site-mål), trycker varje ny säkerhetskopia ut den äldsta. BombVault nekar då en MCP-start av ett objekt vars senaste N-1 lyckade säkerhetskopior alla startades via MCP. Därför finns alltid minst en återställningspunkt i den behållna mängden som schemat eller du har skapat. Med "behåll den senaste 1" kan en assistent inte säkerhetskopiera det objektet alls. Nästa schemalagda säkerhetskopia gör plats igen.

En start av en domän eller av Backup Everything hoppar över de objekt som en gräns håller tillbaka och nämner dem i svaret. Webbgränssnittet och schemat påverkas inte av något av detta. Timbudgeten finns i minnet, så en omstart av BombVault nollställer den.

## Slå på det {#switch-on}

1. Öppna **Inställningar, System, MCP-server** och klicka på **Ny nyckel**.
2. Ge nyckeln ett namn som säger var den används, till exempel "Claude Code på laptopen". Med en nyckel per klient kan du återkalla en utan att röra de andra.
3. Låt **Tillåt att starta säkerhetskopior** vara på, eller stäng av det för en nyckel som bara ska läsa. Du kan ändra det senare på nyckelns rad, och ändringen gäller från assistentens nästa förfrågan utan ny anslutning.
4. Klicka på **Skapa nyckel**. Nyckeln visas en gång. BombVault sparar bara ett fingeravtryck av den och kan inte visa den igen, så kopiera den nu eller ta ett av utdragen nedanför, som då innehåller den riktiga nyckeln.

Utan inloggningslösenord är själva webbgränssnittet öppet för alla i ditt nätverk, och den som kan öppna det kan också skapa en nyckel. Kortet säger det. Öppnar du BombVault under ett namn som ser offentligt ut (till exempel `bombvault.example.com` bakom en omvänd proxy) och inget inloggningslösenord är satt, går det inte att skapa eller byta nycklar från den adressen, så att ingen webbsida på internet kan få din webbläsare att skapa en. Sätt ett inloggningslösenord, eller öppna BombVault via dess IP-adress eller ett lokalt namn som `tower` eller `tower.local`.

## Anslut en klient {#clients}

Kortet visar färdiga utdrag för adressen du öppnade det på: välj din klient och kopiera utdraget. Resten av avsnittet förklarar vad utdragen gör och ger formerna som kortet inte visar.

### Claude Code {#claude-code}

Kör kommandot från kortet en gång i en terminal. Med ett certifikat som din dator litar på ser det ut så här:

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

Kontrollera anslutningen med `/mcp` i Claude Code. `--scope user` sparar nyckeln i din användarkonfiguration i stället för i en projektfil.

Kommandot innehåller nyckeln, och ditt skal kan spara det i sin historik. Det undviker du med en `.mcp.json` i projektmappen och nyckeln i en miljövariabel. Claude Code fyller i `${BOMBVAULT_MCP_KEY}` när den läser filen:

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

Sätt `BOMBVAULT_MCP_KEY` där Claude Code startar, till exempel i din skalprofil, redigerad i en textredigerare i stället för inskriven vid prompten. Checka aldrig in en `.mcp.json` där nyckeln står utskriven.

Med BombVaults eget certifikat (se [TLS och certifikat](#tls)) kör kommandot från kortet i stället `mcp-remote` och pekar Node.js på det nedladdade certifikatet:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

De enkla citattecknen hindrar ditt skal från att expandera variabeln; det gör `mcp-remote` själv. Samma form fungerar i `.mcp.json`: använd posten för Claude Desktop nedan och ta bort `BOMBVAULT_MCP_KEY` ur dess `env`, så kommer nyckeln från din miljö.

### Claude Desktop {#claude-desktop}

Claude Desktop når BombVault via `mcp-remote`, som behöver Node.js på den datorn. Öppna konfigurationsfilen i Claude Desktop via **Settings, Developer, Edit Config**. Den ligger i `%APPDATA%\Claude\claude_desktop_config.json` på Windows och i `~/Library/Application Support/Claude/claude_desktop_config.json` på macOS. Lägg till posten från kortet inuti `"mcpServers"`, bredvid servrar som redan finns där, och starta om Claude Desktop:

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

- `NODE_EXTRA_CA_CERTS` finns bara med för BombVaults eget certifikat. Bakom ett certifikat som din dator redan litar på tar du bort det.
- `--allow-http` läggs bara till för en vanlig `http://`-adress.
- Headern skrivs `X-API-Key:${BOMBVAULT_MCP_KEY}`, utan mellanslag efter kolonet och med nyckeln i `env`. På vissa system delar `mcp-remote` ett `--header`-värde vid första mellanslaget, och en nyckel skriven efter ett mellanslag skulle gå förlorad.

### Egna connectors i Claudes inställningar {#custom-connectors}

Connectors som du lägger till i Claudes egna inställningar (på claude.ai och i connectorlistan i Claude Desktop) stöds inte ännu. Sådana connectors anropas från Anthropics moln, så de behöver en offentlig HTTPS-adress, och de loggar in via OAuth. De kan inte skicka med en fast nyckel, och BombVault erbjuder bara fasta nycklar, ingen OAuth-inloggning. Att lägga ut BombVault på internet för deras skull skulle inte hjälpa. Använd Claude Code, eller Claude Desktop via `mcp-remote` som ovan.

### Andra klienter {#other-clients}

Alla klienter som talar Streamable HTTP fungerar:

- URL: webbgränssnittets adress plus `/mcp`, till exempel `https://192.168.1.10:3443/mcp`.
- Nyckeln i `Authorization: Bearer <key>` eller i `X-API-Key: <key>`. Skickas båda måste de innehålla samma nyckel.
- `POST` med `Content-Type: application/json` och `Accept: application/json, text/event-stream`.
- Ett JSON-RPC-meddelande per förfrågan; batchar nekas.
- Protokollversionerna 2026-07-28, 2025-11-25, 2025-06-18 och 2025-03-26.

## TLS och certifikat {#tls}

BombVault levererar HTTPS med ett certifikat som det har utfärdat själv, och från början nämner det certifikatet bara `localhost`, `127.0.0.1` och `::1`. Claude Code och `mcp-remote` nekar det på en LAN-adress. Vägarna runt det, i den ordning som passar de flesta Unraid-installationer:

1. **Lägg till adressen i MCP-kortet.** Öppnar du kortet över HTTPS på en adress som certifikatet inte nämner, säger kortet det och erbjuder **Lägg till den här adressen i certifikatet**. BombVault utfärdar då sitt certifikat igen med den adressen (din webbläsare varnar en gång till, precis som första gången). Klicka sedan på **Ladda ner certifikat**; utdragen sätter `NODE_EXTRA_CA_CERTS` till den nedladdade filen, så att klienten litar på just det certifikatet.
2. **En omvänd proxy med ett betrott certifikat** (Nginx Proxy Manager, SWAG, Caddy, Traefik). Klienten ser då proxyns certifikat och behöver inget mer, och kortet varnar inte för BombVaults eget.
3. **Tailscale.** `tailscale serve` framför containern, eller Unraids Tailscale-integration, ger dig ett `ts.net`-namn med ett betrott certifikat.
4. **`HTTP_ONLY=true`**, bara bakom en proxy som avslutar TLS eller i ett nätverk som du litar helt på. Det byter hela webbgränssnittet till vanlig HTTP, kräver en ändring i containerinställningarna och skickar nyckeln okrypterad.

Sätt aldrig `NODE_TLS_REJECT_UNAUTHORIZED=0`. Det stänger av certifikatkontrollen för allt som den Node.js-processen pratar med.

En omvänd proxy måste skicka vidare headern `Authorization` (eller `X-API-Key`), vilket proxyer gör om man inte säger åt dem något annat, och får varken buffra eller skriva om `/mcp`. Ett location-block för Nginx eller Nginx Proxy Manager som också kontrollerar BombVaults certifikat:

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

Bakom en proxy bär varje förfrågan proxyns adress. Fem felaktiga nycklar från en enda felkonfigurerad klient låser då ute alla MCP-klienter bakom den proxyn i en minut. Ange proxyn i `TRUSTED_PROXY` (se [Konfiguration](configuration.md)) för att räkna per klient.

## Säkerhetsmodell {#security}

- Utan en aktiv nyckel svarar `/mcp` med `404`.
- Inga adresser är undantagna. Förfrågningar från `localhost`, Unraid-värden, en omvänd proxy eller `tailscale serve` behöver en nyckel som alla andra, även när webbgränssnittet saknar inloggningslösenord.
- Nycklar sparas bara som fingeravtryck, visas en gång och kan döpas om, bytas och återkallas. Upp till 10 aktiva nycklar, var och en med sin egen brytare **Får starta säkerhetskopior**.
- Varje skapande, byte, behörighetsändring och återkallelse skickar en avisering via dina aviseringskanaler, med adressen den kom ifrån, om inte aviseringar är avstängda.
- 5 felaktiga nycklar per minut och adress, därefter `429`. 120 förfrågningar per minut och 12 startade säkerhetskopior per timme och nyckel, plus väntetiden och lagringsskyddet ovan.
- Förfrågningar från en webbläsarsida med en annan origin nekas.
- Så länge inget inloggningslösenord är satt går det inte att skapa nycklar från ett värdnamn som ser offentligt ut.
- Varje säkerhetskopia som en assistent startar, och de prune- och off-site-körningar som följer av den, markeras "via MCP" med nyckelns namn i aktivitetsloggen, i felpanelen och i aviseringen om säkerhetskopian.
- Varje verktygsanrop skrivs i containerns logg med nyckelns id och dess sista fyra tecken (aldrig namnet) och räknas i `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Att återställa en säkerhetskopia av konfigurationen återkallar alla nycklar, eftersom den återställda databasen kan innehålla nycklar som du återkallade efter att den sparades. Skapa nya nycklar efteråt.
- En nyckel slutar fungera när `APP_KEY` ändras (en ominstallation eller en återställning till en annan container). Kortet upptäcker det och markerar nyckeln, och **Byt nyckel** ger den en giltig hemlighet igen.
- Behandla en nyckel som ett lösenord. Claude Code och Claude Desktop sparar den i klartext i sin konfiguration. På en dator som du litar mindre på är en nyckel som bara får läsa att föredra.

## Vad som lämnar maskinen {#privacy}

Det som en assistent läser går till AI-leverantören bakom den: objektnamn, scheman, körningshistorik med felmeddelanden, id:n och tider för återställningspunkter, namn på databasmotorer och storlekar på dumpar, pågående aktivitet, lagringssiffror, täckning och status. BombVault tar bort värdsökvägar, repository-platser, värdnamn, inloggningsuppgifter, hook-kommandon och nycklar innan något lämnar maskinen.

## Felsökning {#troubleshooting}

| Vad du ser | Vad det betyder |
|---|---|
| `404` | Ingen aktiv nyckel, eller en felaktig sökväg som `/api/mcp`. Slutpunkten är `/mcp`. |
| `401` | Nyckeln saknas, är felskriven, återkallad eller bytt. Kanske slänger en proxy headern `Authorization` (prova `X-API-Key`). Om kortet markerar nyckeln som inte längre giltig har `APP_KEY` ändrats: byt nyckeln. |
| `403` | Förfrågan kom från en webbläsarsida med en annan origin. Använd en skrivbords- eller kommandoradsklient. |
| `405` vid GET | Normalt. Slutpunkten tar bara emot `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Klienten är för gammal för Streamable HTTP. Uppdatera den. |
| `400` "batch requests are not accepted" | Klienten skickar JSON-RPC-batchar. Skicka ett meddelande per förfrågan. |
| `429` | För många felaktiga nycklar från den här adressen, eller fler än 120 förfrågningar i minuten med en nyckel. Vänta en minut och kontrollera om assistenten har fastnat i en loop. |
| Fel med "certificate", "self-signed" eller "unable to verify" | Klienten litar inte på BombVaults certifikat. Se [TLS och certifikat](#tls). |
| `busy` | En annan säkerhetskopia eller en underhållsuppgift upptar domänen. Försök igen när den är klar. |
| `cooldown` | Det här objektet, den här domänen eller Backup Everything startades via MCP för mindre än 15 minuter sedan. |
| `retention_guard` | Ytterligare en MCP-säkerhetskopia skulle bara lämna återställningspunkter från MCP i ett fönster med "behåll de senaste N". Nästa schemalagda säkerhetskopia gör plats, eller starta den i webbgränssnittet. |
| `rate_limited` | Nyckeln har använt sina 12 starter för den här timmen. |
| `not_permitted` vid en start | Nyckeln får bara läsa. Slå på **Får starta säkerhetskopior** i kortet; ingen ny anslutning behövs. Vid ett avbrott betyder det att den här nyckeln inte startade körningen. |
| `domain_off` | Den typen av säkerhetskopia är avstängd i inställningarna. |
| `not_found` | BombVault skyddar inte det objektet. Lägg först till det i webbgränssnittet; MCP skapar aldrig konfiguration. |

Sätt inte miljövariabeln `MCPGODEBUG` på containern. Den ändrar hur MCP-biblioteket beter sig, och ett felaktigt värde stoppar BombVault vid start innan det skriver en enda loggrad.
