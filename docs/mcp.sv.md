# MCP-server

BombVault har en inbyggd server för Model Context Protocol (MCP), protokollet som AI-assistenter som Claude Code och Claude Desktop använder för att nå externa verktyg. Genom den kan en assistent läsa hur dina säkerhetskopior mår och, om du tillåter det, starta en säkerhetskopiering eller avbryta en som den själv startade. Den är av tills du skapar en nyckel eller slår på [inloggning via OAuth](#oauth): fram till dess svarar slutpunkten `/mcp` `404` på allt.

## Vad en assistent kan och inte kan göra {#tools}

| Verktyg | Vad det gör | Typ |
|---|---|---|
| `get_health` | Version, instansnamn, om en säkerhetskopia pågår och vad den här nyckeln får göra | läsa |
| `get_status` | Skyddsstatus per domän: senaste lyckade säkerhetskopia, förväntat intervall, verifieringar och off-site-kontroller, nästa schemalagda körningar | läsa |
| `get_coverage` | Vad BombVault skyddar och vad det inte skyddar, med skälet för varje | läsa |
| `list_items` | Varje skyddad container, VM och mappuppsättning, flashminnet och appkonfigurationen, med schema, vad en säkerhetskopia stoppar, senaste säkerhetskopian och hur lång tid den tog; databascontainrar visar även sin senaste dump; ZFS-dataset finns också med, med resultatet av sin senaste kontroll | läsa |
| `list_runs` | Körningshistorik, nyaste först, filtrerbar på domän, objekt, status, typ och tid | läsa |
| `list_restore_points` | Återställningspunkter för ett objekt från dess primära repository, och för en container även dess databasdumpar; ett ZFS-dataset får en återställningspunkt per säkerhetskopia, med en snapshot av varje dataset under det | läsa |
| `get_activity` | Vad som körs just nu, med fas och procent | läsa |
| `get_storage_stats` | Storlekshistorik för en domäns primära repository och dess tillväxt per vecka, med ledigt utrymme och veckor tills det är fullt där volymen kan mätas | läsa |
| `list_anomalies` | Avvikelser som BombVault har upptäckt i säkerhetskopiorna, kan filtreras på tillstånd, allvarlighet och domän, med en sammanfattning av det som är öppet | läsa |
| `get_anomaly` | En av dessa avvikelser, med anteckningen som lämnades när den kvitterades | läsa |
| `start_backup` | Säkerhetskopierar ett objekt direkt | starta |
| `start_domain_backup` | Säkerhetskopierar varje skyddat objekt i en domän | starta |
| `start_backup_everything` | Kör Backup Everything-genomgången | starta |
| `cancel_backup` | Avbryter en pågående säkerhetskopia som den här nyckeln har startat | avbryta |

Detta stannar i webbgränssnittet: återställningar av alla slag (även att ladda ner, spara eller importera en databasdump), att radera säkerhetskopior, prune, unlock, kontroller och övningar, off-site-replikering, inställningar, inloggningsuppgifter och MCP-nycklar, samt att avbryta en säkerhetskopia som schemat, webbgränssnittet eller en annan nyckel har startat. Detsamma gäller att kvittera en avvikelse eller markera den som förväntad, vilket görs på sidan **Avvikelser**. Skälet är att verktygens svar innehåller namn och felmeddelanden från din server, och vilket som helst av dem kan innehålla text som skrivits för att styra assistenten. En assistent som går på sådan text kan i värsta fall starta en säkerhetskopia inom gränserna nedan eller avbryta en som den själv startat.

Om ett objekts primära repository ligger någon annanstans (S3, REST, SFTP, rclone) kontaktar `list_restore_points` det, och anropet kan ta en stund. Off-site-kopior kan inte listas via MCP. Vad avvikelsekontrollerna tittar på beskrivs under [Funktioner](features.md), och hur ett ZFS-objekt har en ögonblicksbild per datauppsättning under [ZFS-datauppsättningar](zfs-datasets.md#contents).

## Vad en startad säkerhetskopia gör {#starting-backups}

En assistents säkerhetskopia är samma säkerhetskopia som webbgränssnittet startar. En körande container stoppas tills dess säkerhetskopia är klar, tillsammans med de containrar som är inställda att stoppas med den. En VM med metoden "graceful" stängs av och startas igen. Ett ZFS-dataset stoppar de containrar som är inställda för det medan dess snapshot tas. Mappuppsättningar, flashminnet och konfigurationen fortsätter att köra. Efteråt tillämpar BombVault lagringspolicyn och kopierar eventuellt till off-site-repositoryt. `list_items` talar om för assistenten vad ett objekt stoppar och hur lång tid dess senaste säkerhetskopia tog, och verktygens beskrivningar ber den säga det till dig innan den startar något.

Eftersom en säkerhetskopia stoppar saker och trycker ut gamla återställningspunkter är starter via MCP begränsade:

- 12 startade säkerhetskopior per timme och nyckel.
- 15 minuter mellan två MCP-starter av samma objekt, samma domän eller Backup Everything.
- Högst 4 MCP-starter av samma objekt på 24 timmar.
- **Lagringsskydd.** När en domän behåller ett fast antal återställningspunkter (bara "behåll de senaste N", utan daglig, veckovis eller månadsvis regel, lokalt eller på ett off-site-mål), trycker varje ny säkerhetskopia ut den äldsta. BombVault nekar då en MCP-start av ett objekt vars senaste N-1 lyckade säkerhetskopior alla startades via MCP. Därför finns alltid minst en återställningspunkt i den behållna mängden som schemat eller du har skapat. Med "behåll den senaste 1" kan en assistent inte säkerhetskopiera det objektet alls. Nästa schemalagda säkerhetskopia gör plats igen.

En start av en domän eller av Backup Everything hoppar över de objekt som en gräns håller tillbaka och nämner dem i svaret. Webbgränssnittet och schemat påverkas inte av något av detta. Timbudgeten finns i minnet, så en omstart av BombVault nollställer den.

## Slå på det {#switch-on}

1. Öppna **Inställningar, System, MCP-server** och klicka på knappen för din klient. En klient som inte finns i listan ansluter via **Annan klient**.
2. Behåll under **Nyckel** valet **Ny nyckel** och det föreslagna namnet, klientens, eller skriv ett som säger var nyckeln används, till exempel ”Claude Code på laptopen”. En nyckel per klient låter dig återkalla en utan att röra de andra. **Befintlig nyckel** ger klienten en nyckel som du skapat tidigare.
3. Slå på **Tillåt att starta säkerhetskopior** för en nyckel som ska kunna starta säkerhetskopior; utan det kan den bara läsa. Du kan ändra det senare på nyckelns ruta, och ändringen gäller från assistentens nästa förfrågan utan ny anslutning.
4. Klicka på **Skapa nyckel**. Nyckeln visas en gång. BombVault behåller bara ett fingeravtryck av den och kan inte visa den igen, så kopiera den nu. Stänger du dialogen innan klienten har använt nyckeln fortsätter kortet att visa den tills du bekräftar att du har kopierat den.

Utan inloggningslösenord är själva webbgränssnittet öppet för alla i ditt nätverk, och den som kan öppna det kan också skapa en nyckel. Kortet säger det. Öppnar du BombVault under ett namn som ser offentligt ut (till exempel `bombvault.example.com` bakom en omvänd proxy) och inget inloggningslösenord är satt, går det inte att skapa eller byta nycklar från den adressen, så att ingen webbsida på internet kan få din webbläsare att skapa en. Sätt ett inloggningslösenord, eller öppna BombVault via dess IP-adress eller ett lokalt namn som `tower` eller `tower.local`.

## Dina nycklar och deras logg {#keys}

Varje nyckel har en egen ruta på kortet. Den visar nyckelns namn, om den får starta säkerhetskopior eller bara läser, nyckelns fyra sista tecken, när den skapades eller senast byttes, när en klient senast använde den och hur många anrop den har gjort i dag. I rutan byter du namn på nyckeln, ändrar dess behörighet, byter ut den eller återkallar den. En återkallad nyckel flyttas till listan med återkallade nycklar, där du kan ta bort den för gott när ingen körning i historiken längre nämner den.

Bredvid namnet visar rutan logotypen för den klient som nyckeln skapades för. En nyckel som skapats via **Annan klient**, eller innan kortet listade klienter, visar i stället en nyckel.

**Logg** i en ruta öppnar det nyckeln har gjort. Först kommer de säkerhetskopior den startade, var och en med sin status och en länk till körningen i aktivitetsloggen på instrumentpanelen. Under dem står dess anrop, nyaste först, med verktyget och vad som blev av anropet. Ett avvisande säger varför: nyckeln får bara läsa, lagringsskyddet höll tillbaka säkerhetskopian, en annan säkerhetskopiering pågick redan, objektet säkerhetskopierades via MCP för några minuter sedan, eller nyckeln skickade för många begäranden. En avbrytning länkar till körningen den gällde.

BombVault sparar varje nyckels poster i upp till 30 dagar: de senaste 500 lyckade starterna och avbrotten och vid sidan av dem de senaste 200 övriga anropen (läsningar, avvisningar och fel), så en assistent som om och om igen frågar efter en pågående säkerhetskopiering eller försöker med ett avvisat anrop inte kan tränga ut dess start ur loggen. För varje anrop sparar det verktyget, utfallet och körningen som en avbrytning nämnde. Det sparar aldrig vad assistenten skickade, och aldrig nyckeln eller dess fingeravtryck. Diagnostikpaketet räknar bara posterna, och en export av inställningarna utelämnar dem.

## Anslut en klient {#clients}

Varje klient har en knapp på kortet, under **På den här datorn** eller **I molnet**. Knappen öppnar en dialog i tre steg: nyckeln; konfigurationen för den klienten, med adressen du öppnade kortet på, en knapp för att kopiera den, var konfigurationen ligger och, med BombVaults eget certifikat, vad klienten behöver för att lita på det; och väntan på klientens första anrop. Dialogen följer nyckelns senaste användning och blir grön när anropet kommer.

Dialogen håller nyckeln borta från varje kommandorad. Där klienten kan läsa den från en miljövariabel (`BOMBVAULT_MCP_KEY`), en dold fråga eller en egen fil nämner konfigurationen den bara. Där klienten saknar ett sådant sätt ligger nyckeln i dess konfigurationsfil eller inställningar, och dialogen säger det. Där en klients dokumentation inte säger hur den hanterar ett okänt certifikat skriver dialogen det steget som vad du gör om klienten avvisar BombVaults certifikat.

| Klient | Inställning | Var nyckeln kommer ifrån |
|---|---|---|
| AnythingLLM | konfigurationsfil | konfigurationsfilen |
| Antigravity | konfigurationsfil | miljövariabel |
| Claude Code | kommando | nyckelfil |
| Claude Desktop | konfigurationsfil | nyckelfil |
| Cline | konfigurationsfil | konfigurationsfilen |
| Codex CLI | konfigurationsfil | miljövariabel |
| Continue | konfigurationsfil | `~/.continue/.env` |
| Copilot CLI | konfigurationsfil | konfigurationsfilen |
| Cursor | konfigurationsfil | miljövariabel |
| Gemini CLI | konfigurationsfil | miljövariabel |
| GitHub Copilot (VS Code) | konfigurationsfil | dold fråga |
| Goose | konfigurationsfil | miljövariabel |
| Jan | formulär i appen | appens inställningar |
| JetBrains (AI Assistant, Junie) | konfigurationsfil | konfigurationsfilen |
| Kimi Code | konfigurationsfil | konfigurationsfilen |
| LM Studio | konfigurationsfil | konfigurationsfilen |
| Mistral Vibe | konfigurationsfil | miljövariabel |
| Msty | formulär i appen | appens inställningar |
| n8n | formulär i appen | inloggningsuppgifterna i n8n |
| Open WebUI | formulär i appen | appens inställningar |
| opencode | konfigurationsfil | miljövariabel |
| Perplexity (Mac) | formulär i appen | nyckelfil |
| Qwen Code | konfigurationsfil | miljövariabel |
| Roo Code | konfigurationsfil | miljövariabel |
| Visual Studio | konfigurationsfil | konfigurationsfilen |
| Warp | konfigurationsfil | konfigurationsfilen |
| Windsurf | konfigurationsfil | miljövariabel |
| Zed | konfigurationsfil | konfigurationsfilen |
| Grok | formulär, i molnet | leverantörens servrar |
| Le Chat | formulär, i molnet | leverantörens servrar |
| ChatGPT | inloggning via OAuth, i molnet | ett åtkomsttoken, se [nedan](#oauth) |
| Claude (claude.ai) | inloggning via OAuth, i molnet | ett åtkomsttoken, se [nedan](#oauth) |

Avsnitten nedan förklarar inställningen av Claude Code och Claude Desktop närmare och tar upp vad varje annan klient behöver.

### Claude Code {#claude-code}

Claude Code når BombVault via `mcp-remote`, som behöver Node.js på den datorn. Spara först nyckeln i en egen textfil, på en enda rad:

```text
X-API-Key: <your key>
```

Kör sedan kommandot från kortet en gång i en terminal, med sökvägen till den filen ifylld. Bakom ett certifikat som din dator litar på ser det ut så här:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

Med BombVaults eget certifikat (se [TLS och certifikat](#tls)) pekar kommandot dessutom Node.js på det nedladdade certifikatet:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Kontrollera anslutningen med `/mcp` i Claude Code. `--scope user` gör BombVault tillgängligt i alla dina projekt. Claude Code sparar bara sökvägen till nyckelfilen, så nyckeln syns varken i kommandot och din skalhistorik eller i processlistan. Lägg filen där bara du kan läsa den och utanför alla mappar som du checkar in. `@latest` får `npx` att hämta en aktuell `mcp-remote`; annars skulle en äldre, globalt installerad version användas, och den känner inte till `--header-file`.

Skriv inte `${BOMBVAULT_MCP_KEY}` i argumenten till `mcp-remote` för Claude Code. Claude Code fyller i en sådan referens från sin egen miljö innan den startar `mcp-remote`, så nyckeln hamnar på den processens kommandorad, där andra program och användare på datorn kan läsa den.

Utan Node.js, och bara bakom ett certifikat som din dator litar på, kan Claude Code ansluta på egen hand. Lägg en `.mcp.json` i projektmappen:

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

Sätt `BOMBVAULT_MCP_KEY` där Claude Code startar, till exempel under `"env"` i `~/.claude/settings.json` eller i din skalprofil, redigerad i en textredigerare i stället för inskriven vid prompten. Här är referensen säker, eftersom Claude Code inte startar någon ytterligare process som nyckeln skulle hamna i. BombVaults eget certifikat fungerar inte på det här sättet: Claude Codes egen anslutning nekar det även när `NODE_EXTRA_CA_CERTS` är satt. Checka aldrig in en `.mcp.json` där nyckeln står utskriven.

### Claude Desktop {#claude-desktop}

Claude Desktop når BombVault via `mcp-remote`, som behöver Node.js på den datorn. Spara först nyckeln i en egen textfil, som en enda rad, så som det beskrivs för [Claude Code](#claude-code). Öppna konfigurationsfilen i Claude Desktop via **Settings, Developer, Edit Config**. Den ligger i `%APPDATA%\Claude\claude_desktop_config.json` på Windows och i `~/Library/Application Support/Claude/claude_desktop_config.json` på macOS. Lägg till posten från kortet inuti `"mcpServers"`, bredvid servrar som redan finns där, och starta om Claude Desktop:

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

- `NODE_EXTRA_CA_CERTS` finns bara med för BombVaults eget certifikat. Bakom ett certifikat som din dator redan litar på tar du bort det.
- `--allow-http` läggs bara till för en vanlig `http://`-adress.
- På Windows skriver du sökvägarna med vanliga snedstreck, till exempel `C:/Users/sam/bombvault-key.txt`, eftersom ett ensamt omvänt snedstreck inte är giltig JSON. Håll nyckelfilens sökväg fri från mellanslag: Claude Desktop på Windows skickar en sökväg med mellanslag till `npx` i två delar.
- Konfigurationen nämner bara nyckelfilen, så nyckeln syns varken i den eller i processlistan. Lägg filen där bara du kan läsa den.

### Klienter i molnet {#cloud-clients}

ChatGPT, Claude på claude.ai, Grok och Le Chat anropar BombVault från sin leverantörs servrar, så BombVault måste nås från internet med ett offentligt betrott certifikat, till exempel bakom en omvänd proxy; Le Chat avvisar självsignerade. En inloggning på proxyn kan skydda webbgränssnittet, men `/mcp` måste nå BombVault utan den: de här tjänsterna kan inte logga in på en proxy, och BombVault kontrollerar själv deras nyckel eller token. Grok och Le Chat skickar en fast nyckel, och deras knappar ställer in dem som de andra. ChatGPT, och i de flesta organisationer även Claude på claude.ai, ansluter bara via inloggning med OAuth, som beskrivs härnäst.

### Inloggning via OAuth {#oauth}

För en klient som inte kan ta en nyckel är BombVault sin egen OAuth-auktoriseringsserver. Klienten registrerar sig själv, skickar dig till en BombVault-sida, och där loggar du in med ditt inloggningslösenord (och den andra faktorn, om du har ställt in en) och tillåter den. Klienten får sedan ett token som bara gäller MCP-slutpunkten i denna BombVault, och förnyar det själv.

1. Sätt ett inloggningslösenord under **Inställningar, System**. Utan lösenord erbjuder BombVault ingen inloggning alls, eftersom det inte skulle finnas någon att fråga om samtycke.
2. Gör BombVault nåbar från internet över https med ett certifikat som webbläsare litar på, oftast via en omvänd proxy. Klienten anropar `/mcp`, `/oauth/` och `/.well-known/` från sina egna servrar, så en proxy med egen inloggning måste släppa igenom de tre sökvägarna till BombVault. Samtyckessidan på `/oauth/authorize` öppnas i din egen webbläsare och får ligga bakom proxyns inloggning. Ange också proxyn i `TRUSTED_PROXY` (se [Konfiguration](configuration.md)). BombVault begränsar klientregistreringar per adress, och utan den ser varje klient ut att komma från proxyn.
3. Slå på **Inloggning via OAuth** på MCP-kortet och ange **Offentlig adress**: https-adressen utan sökväg, till exempel `https://backup.example.com`. Varje token är bundet till den här adressen, så efter en ändring måste varje klient logga in igen.
4. Klicka på knappen för ChatGPT eller Claude. Dialogen visar **Connector-URL**, alltså den offentliga adressen med `/mcp` efter, och var den hör hemma i den klienten. I ChatGPT slår du på utvecklarläge under **Inställningar, Appar och connectors, Avancerade inställningar**, väljer **Skapa**, klistrar in connector-URL:en som MCP-server-URL och väljer OAuth som autentisering. På claude.ai öppnar du **Inställningar, Connectors, Lägg till anpassad connector**, klistrar in connector-URL:en, lämnar OAuth-klient-ID och hemlighet tomma och väljer **Anslut**.
5. Klienten öppnar samtyckessidan. Den visar vem som frågar, vart ditt svar skickar dig tillbaka och reglaget **Tillåt att starta säkerhetskopior**, som börjar avslaget. Välj **Tillåt** eller **Neka**.

Varje klient som har loggat in får en ruta bredvid nycklarna, med sitt märke, sin logg, **Återkalla** och **Tillåt att starta säkerhetskopior**, och samma gränser som en nyckel. Återkallelse gäller direkt. När samma klient loggar in igen ersätter dess nya tillstånd det gamla, och ett tillstånd som ingen har använt på 30 dagar går ut. Upp till 10 klienter kan vara inloggade samtidigt, utöver de 10 nycklarna.

Samtyckessidan tar bara emot en begäran från en registrerad klient som anger exakt en av sina registrerade returadresser: https, eller en loopback-adress på valfri port för en klient på din egen dator. Bara auktoriseringskodflödet med PKCE (S256) godtas, och ditt svar är bundet till din session, så ingen annan webbplats kan skicka det åt dig. Åtkomsttoken gäller en timme. Ett uppdateringstoken ersätts vid varje användning, och dyker ett upp igen efteråt återkallar BombVault tillståndet, eftersom någon annan har en kopia. En klient som upprepar sin senaste förnyelse inom 30 sekunder, eftersom svaret aldrig kom fram, får i stället nya token. BombVault hämtar inga klientmetadata från internet, så klienter registrerar sig via dynamisk klientregistrering.

### Andra klienter {#other-clients}

Alla klienter som talar Streamable HTTP fungerar:

- URL: webbgränssnittets adress plus `/mcp`, till exempel `https://192.168.1.10:3443/mcp`.
- Nyckeln i `Authorization: Bearer <key>` eller i `X-API-Key: <key>`. Skickas båda måste de innehålla samma nyckel.
- `POST` med `Content-Type: application/json` och `Accept: application/json, text/event-stream`.
- Ett JSON-RPC-meddelande per förfrågan; batchar nekas.
- Protokollversionerna 2026-07-28, 2025-11-25, 2025-06-18 och 2025-03-26.

## TLS och certifikat {#tls}

BombVault levererar HTTPS med ett certifikat som det har utfärdat själv, och från början nämner det certifikatet bara `localhost`, `127.0.0.1` och `::1`. Claude Code och `mcp-remote` nekar det på en LAN-adress. Vägarna runt det, i den ordning som passar de flesta Unraid-installationer:

1. **Lägg till adressen i MCP-kortet.** Öppnar du kortet över HTTPS på en adress som certifikatet inte nämner, säger kortet det och erbjuder **Lägg till den här adressen i certifikatet**. BombVault utfärdar då sitt certifikat igen med den adressen (din webbläsare varnar en gång till, precis som första gången). Klicka sedan på **Ladda ner certifikat**; utdragen sätter `NODE_EXTRA_CA_CERTS` till den nedladdade filen, så att klienten litar på just det certifikatet. Det betyder också att varje klient som är konfigurerad med en tidigare nedladdad fil slutar ansluta så snart certifikatet utfärdas igen, på den här datorn och på alla andra, tills den får den nya filen.
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

- Utan en aktiv nyckel och med inloggning via OAuth avslagen svarar `/mcp` `404`.
- Inloggning via OAuth erbjuds bara så länge ett inloggningslösenord är satt. Token, koder och klienthemligheter lagras bara som fingeravtryck, och ett token gäller bara för adressen det utfärdades för.
- En klient kan registrera sig högst 10 gånger i timmen från en adress, och BombVault behåller högst 100 registrerade klienter som ingen har loggat in med, var och en i ett dygn. Fel koder och uppdateringstoken räknas mot samma spärr som fel nycklar.
- Tillstånd beter sig som nycklar när en konfigurationssäkerhetskopia återställs eller `APP_KEY` ändras: efter en återställning måste varje klient logga in igen.
- Inga adresser är undantagna. Förfrågningar från `localhost`, Unraid-värden, en omvänd proxy eller `tailscale serve` behöver en nyckel som alla andra, även när webbgränssnittet saknar inloggningslösenord.
- Nycklar sparas bara som fingeravtryck, visas en gång och kan döpas om, bytas och återkallas. Upp till 10 aktiva nycklar, var och en med sin egen brytare **Tillåt att starta säkerhetskopior**.
- Varje skapande, byte, behörighetsändring och återkallelse skickar en avisering via dina aviseringskanaler, med adressen den kom ifrån, om inte aviseringar är avstängda.
- 5 felaktiga nycklar per minut och adress, därefter `429`. 120 förfrågningar per minut och 12 startade säkerhetskopior per timme och nyckel, plus väntetiden och lagringsskyddet ovan.
- Förfrågningar från en webbläsarsida med en annan origin nekas.
- Så länge inget inloggningslösenord är satt går det inte att skapa nycklar från ett värdnamn som ser offentligt ut.
- Varje säkerhetskopia som en assistent startar, och de prune- och off-site-körningar som följer av den, markeras "via MCP" med nyckelns namn i aktivitetsloggen, i felpanelen och i aviseringen om säkerhetskopian.
- Varje verktygsanrop skrivs i containerns logg med nyckelns id och dess sista fyra tecken (aldrig namnet) och räknas i `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Att återställa en säkerhetskopia av konfigurationen återkallar alla nycklar, eftersom den återställda databasen kan innehålla nycklar som du återkallade efter att den sparades. Skapa nya nycklar efteråt.
- En nyckel slutar fungera när `APP_KEY` ändras (en ominstallation eller en återställning till en annan container). Kortet upptäcker det och markerar nyckeln, och **Byt nyckel** ger den en giltig hemlighet igen.
- Behandla en nyckel som ett lösenord. En klient som inte kan läsa nyckeln från en miljövariabel, en fråga eller en nyckelfil har den i klartext i sin konfiguration eller sina inställningar, och dess dialog säger det. Välj hellre en nyckel som bara får läsa på en dator du litar mindre på.

## Vad som lämnar maskinen {#privacy}

Det som en assistent läser går till AI-leverantören bakom den: objektnamn, scheman, körningshistorik med felmeddelanden, id:n och tider för återställningspunkter, namn på databasmotorer och storlekar på dumpar, pågående aktivitet, lagringssiffror, täckning och status. BombVault tar bort värdsökvägar, repository-platser, värdnamn, inloggningsuppgifter, hook-kommandon och nycklar innan något lämnar maskinen.

## Felsökning {#troubleshooting}

| Vad du ser | Vad det betyder |
|---|---|
| `404` | Ingen aktiv nyckel och inloggning via OAuth är avslagen, eller en fel sökväg som `/api/mcp`. Slutpunkten är `/mcp`. |
| `401` | Nyckeln saknas, är felskriven, återkallad eller bytt. Kanske slänger en proxy headern `Authorization` (prova `X-API-Key`). Om kortet markerar nyckeln som inte längre giltig har `APP_KEY` ändrats: byt nyckeln. |
| `403` | Förfrågan kom från en webbläsarsida med en annan origin. Använd en skrivbords- eller kommandoradsklient. |
| `405` vid GET | Normalt. Slutpunkten tar bara emot `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Klienten är för gammal för Streamable HTTP. Uppdatera den. |
| `400` "batch requests are not accepted" | Klienten skickar JSON-RPC-batchar. Skicka ett meddelande per förfrågan. |
| `429` | För många felaktiga nycklar från den här adressen, eller fler än 120 förfrågningar i minuten med en nyckel. Vänta en minut och kontrollera om assistenten har fastnat i en loop. |
| Fel med "certificate", "self-signed" eller "unable to verify" | Klienten litar inte på BombVaults certifikat. Se [TLS och certifikat](#tls). |
| `busy` | En annan säkerhetskopia eller en underhållsuppgift upptar domänen. Försök igen när den är klar. |
| `cooldown` | Det här objektet, den här domänen eller Backup Everything startades via MCP för mindre än 15 minuter sedan. |
| `retention_guard` | Ytterligare en MCP-säkerhetskopia skulle bara lämna återställningspunkter från MCP i ett fönster med "behåll de senaste N", eller så har objektet redan fått 4 säkerhetskopior via MCP de senaste 24 timmarna, misslyckade och avbrutna medräknade. I det första fallet gör nästa schemalagda säkerhetskopia plats, i det andra är objektet ledigt igen 24 timmar efter den äldsta av dem. Du kan alltid starta den i webbgränssnittet. |
| `rate_limited` | Nyckeln har använt sina 12 starter för den här timmen. |
| `not_permitted` vid en start | Nyckeln får bara läsa. Slå på **Tillåt att starta säkerhetskopior** i kortet; ingen ny anslutning behövs. Vid ett avbrott betyder det att den här nyckeln inte startade körningen. |
| `domain_off` | Den typen av säkerhetskopia är avstängd i inställningarna. |
| `not_found` | BombVault skyddar inte det objektet. Lägg först till det i webbgränssnittet; MCP skapar aldrig konfiguration. |
| Klienten hittar inte auktoriseringsservern | Inloggning via OAuth är avslagen, inget inloggningslösenord är satt, eller proxyn släpper inte igenom `/.well-known/` till BombVault. |
| Samtyckessidan säger att returadressen inte är registrerad | Klienten skickade en returadress som den inte har registrerat. Ta bort connectorn i klienten och lägg till den igen. |
| En inloggad klient får `401` | Dess tillstånd har återkallats, gått ut efter 30 dagar utan användning, eller så har den offentliga adressen ändrats. Klienten loggar in igen. |

Sätt inte miljövariabeln `MCPGODEBUG` på containern. Den ändrar hur MCP-biblioteket beter sig, och ett felaktigt värde stoppar BombVault vid start innan det skriver en enda loggrad.
