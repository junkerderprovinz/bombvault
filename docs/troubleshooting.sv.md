# Felsökning

En kort FAQ. För den fullständiga felsökningstabellen för VM-över-SSH på värdsidan (permission-denied, host-key-verifiering, saknade mallvariabler och mer), se [guiden för VM-säkerhetskopiering över SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) på GitHub.

## Något är inte korrekt inkopplat

Lägg till kortet **Värdintegration** på översikten under **Anpassa**. Värdintegrationskontrollen sonderar varje montering och CLI (Docker-socket, libvirt, restic, qemu-img, rclone) och rapporterar eventuella saknade delar, och **Kontrollera nu** kör den igen. Börja här innan du antar ett fel: en saknad montering eller en onåbar värd syns direkt.

## Jag kan inte nå webbgränssnittet

BombVault serverar HTTPS direkt ur lådan på port `3443` (självsignerat certifikat), så öppna `https://<din-unraid-ip>:3443`. Godkänn varningen om det självsignerade certifikatet, eller placera BombVault bakom en reverse proxy med ditt eget certifikat. Om du kör med `HTTP_ONLY=true` serverar den vanlig HTTP på port `3000` istället (avsedd för användning bakom en TLS-terminerande proxy).

## Jag förlorade min APP_KEY

`APP_KEY` härleder restic-arkivets lösenord. Utan den, utan återställningskitet och utan en nyckeldeposition i din grupp går krypterade säkerhetskopior inte att återställa. Därför visar översikten **Spara ditt återställningskit** tills du har laddat ner det. Se [Off-site och återställning](offsite-recovery.md#recovery-kit). Skapa en nyckel med `openssl rand -hex 32` och förvara den utanför servern innan du förlitar dig på någon säkerhetskopia.

## Jag har glömt inloggningslösenordet

Välj **Glömt lösenordet?** på inloggningssidan. BombVault skriver en kod på 12 tecken i sin containerlogg (i Unraid: fliken Docker, BombVault-ikonen, **Logs**). Skriv in den med ett nytt lösenord inom 10 minuter. Återställningen stänger också av tvåfaktorsautentisering, tar bort alla lösennycklar, återkallar alla API-token, MCP-nycklar och OAuth-inloggningar, avslutar alla andra sessioner och stänger av parkopplingen av telefoner. Lägg till dina lösennycklar igen under **Inställningar, Säkerhet**, slå på parkopplingen av telefoner där igen och skapa nya token och nycklar där du fortfarande behöver dem. Se [Säkerhetsmodell](configuration.md#login).

## Säkerhetskopiorna har stannat och säger att nyckeln inte längre går att öppna

Den här servern tog över en annan servers säkerhetskopienyckel i Nödguiden, och `APP_KEY` ändrades efteråt. Den övertagna nyckeln är förseglad med den gamla APP_KEY, så BombVault kan inte öppna den och säkerhetskopierar ingenting i stället för att skriva med fel nyckel. **Behöver uppmärksamhet** på översikten namnger den. Sätt tillbaka APP_KEY till det den var, eller ladda upp återställningskitet igen på sidan Återställning under **Nödläge**.

## En telefon hamnar på inloggningssidan

En parkopplad telefon får sin session via gruppen bara så länge **Telefoner kan parkopplas med gruppens tolv ord** är på under **Inställningar, Säkerhet**. En återställning av lösenordet stänger av det. En app som är äldre än servern får heller ingen session förrän den uppdaterats. En server som lagts till med sin adress frågar alltid efter lösenordet. En telefon som har tagits bort från sidan Instanser kommer tillbaka avstängd när dess app öppnas igen; slå på **Fråga av den här instansen** under **Övervakning** i dess detaljer för att släppa in den.

## VM-säkerhetskopiering ansluter inte

VM-säkerhetskopiering pratar med libvirt över SSH, aldrig en montering.

- Bekräfta att SSH är aktiverat på värden och att BombVaults publika nyckel är auktoriserad i `/root/.ssh/authorized_keys` (Inställningar, Integrationer, Värd-SSH visar nyckeln och en **Testa anslutning**-knapp).
- På ett anpassat `br0.x`-nätverk, sätt `LIBVIRT_HOST` till din Unraid-LAN-IP (containern kan inte nå värden via `host.docker.internal` där). Aktivera **Settings, Docker, Host access to custom networks**.
- Om du ändrade Unraids SSH-port, sätt `LIBVIRT_SSH_PORT` att matcha.
- På TrueNAS, sätt `LIBVIRT_URI` med TrueNAS egen libvirt-socket, ge appen capabilityn `SETPCAP` och lägg till nyckeln hos root under **Credentials, Users** (se [TrueNAS SCALE](getting-started.md#truenas)).
- Fullständig steg-för-steg-diagnos (nåbarhetstest, VLAN-routning, `Permission denied (publickey)`, `Host key verification failed`) finns i [guiden för VM-säkerhetskopiering över SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## En live-VM-ögonblicksbild kördes inte

Live-ögonblicksbilder behöver qemu-gästagenten installerad i VM:en och disken på `/mnt/cache` (eller `/mnt/diskX`), inte `/mnt/user`. På en avstängd VM faller live automatiskt tillbaka till mjuk. En mjuk säkerhetskopiering stänger av VM:en, säkerhetskopierar diskarna och startar sedan om den, så den är alltid konsekvent. Den väntar upp till tio minuter på att gästen stängs av och tvingar aldrig av den, så att en Windows-gäst som installerar uppdateringar inte avbryts halvvägs. En gäst som fortfarande körs efter det lämnas ifred, och den säkerhetskopieringen misslyckas med orsaken.

## En säkerhetskopiering misslyckades med "repository is already locked"

Det här är oftast ett föräldralöst restic-lås som blev kvar när containern uppdaterades eller startades om mitt i en operation. BombVault upptäcker ett bevisat föräldralöst lås, rensar det med tvång och försöker en gång till, automatiskt. Om det består, öppna kortet **Kontrollera säkerhetskopior** under **Inställningar, Verifiering** och välj **Lås upp** under **Mer** på det berörda avsnittets rad för att rensa ett gammalt lås för hand. Ett verkligt problem dyker ändå upp i stället för att döljas. Efter en omstart väntar BombVault tills ett sådant lås gått tio minuter utan förnyelse. Ett restic som fortfarande körs, till exempel i en andra BombVault på samma arkiv, förnyar sitt lås var femte minut.

## Min off-site-kopia hände inte efter en säkerhetskopiering

Off-site-replikering är best-effort av design, så ett off-site-hickande får aldrig den lokala säkerhetskopian att misslyckas. Kontrollera när målet kopierar: dess kort **Kopiera** säger **Efter varje säkerhetskopia** eller namnger en egen plan, och **Inställningar, Schema, Externa kopior per avsnitt** kan ge ett avsnitt en egen plan. Använd **Replikera nu** i avsnittets off-site-kopiakort under **Inställningar, Mål** för en körning på begäran, och titta på översiktens statushuvud, som säger **Extern kopiering pågår** medan den körs. En misslyckad kopiering syns under **Behöver uppmärksamhet**.

## En rest-servers certifikat betros inte

restic vägrar ett certifikat det inte känner till, och felet säger var det ska ligga. Kopiera rest-serverns självsignerade certifikat, eller certifikatet från utfärdaren som signerade det, som PEM-fil till mappen `certs` i BombVaults datamapp (`/config/certs` i containern, `appdata/bombvault/certs` i Unraid) och testa igen. Ingen omstart behövs.

## En säkerhetskopia säger att data utanför Host Data inte säkerhetskopierades {#data-outside-host-data}

Containern har data i en mapp som BombVault inte kan läsa via Host Data. Säkerhetskopian sparar definitionen och varje mapp den når, färgar sin rad i aktivitetsloggen orange och namnger mapparna den hoppade över. Två fall är vanliga: en ixVolume på TrueNAS under `/mnt/.ix-apps` och en namngiven Docker-volym under `/var/lib/docker/volumes`, till exempel databasvolymen i en compose-stack. Kortet **Säkerhetskopierade mappar** på containerns flik **Inställningar** visar varje sådan mapp med dess sökväg inuti containern och dess värdsökväg, markerad med rött.

En anonym Docker-volym rapporteras inte. Docker skapar den själv för en mapp som imagen anger och som ingen har mappat, ger den ett namn på 64 hexadecimala siffror och lägger den under `/var/lib/docker/volumes`. Oftast innehåller den en cache, så säkerhetskopian förblir grön och kortet visar den bara, med en notis om att den är anonym.

- **Du vill säkerhetskopiera de data.** Mappa mappen till en värdsökväg under Host Data, till exempel en mapp i din appdata-delning, så tar nästa säkerhetskopia med den.
- **Du behöver dem inte.** Lägg till en av dess två sökvägar som en rad i kortet **Uteslutningsmönster** på samma flik: sökvägen inuti containern, till exempel `/cache`, eller värdsökvägen som varningen nämner, till exempel `/var/lib/docker/volumes/jellyfin_cache/_data`. Från nästa säkerhetskopia utelämnar varningen den mappen, och förhandsgranskningen under fältet säger varför. En rad för en mapp ovanför fungerar också, ett rent namn eller `/` gör det inte.

## En stor säkerhetskopiering avbröts för att minnet tog slut

Ställ in **Minnesgräns per säkerhetskopiering (MiB, 0 = ingen)** i kortet **Prestanda** under **Inställningar, System** en bra bit under det som containern får använda. Nära gränsen frigör restic minne oftare, vilket kostar lite fart, så säkerhetskopieringen blir långsammare i stället för att avbrytas.

## En återställning avbröts innan den startade

Innan något stoppas eller tas bort kör återställningen en konfliktkontroll före körning: den verifierar att containerns statiska IP och publicerade värdportar är lediga. Om en annan container redan håller en av dem avbryter den med ett tydligt, åtgärdbart meddelande istället för att lämna en halvfärdig återställning. Frigör den konfliktande porten eller IP:n och försök igen.

Om containerns nätverk inte finns, till exempel för att dess TrueNAS-app togs bort, nämner kontrollen före återställningen nätverket, och återställningen vägrar innan den ändrar något. Installera om appen i TrueNAS under samma namn, vilket skapar nätverket igen, och återställ sedan dess data.

## En vanlig export misslyckades istället för att skriva en fil

Om **Kryptera exporter med age** är på under **Inställningar, System** men ingen giltig mottagare är angiven misslyckas en export med ett tydligt fel i stället för att skriva klartext. Lägg till en giltig mottagare (en publik age-nyckel eller en publik SSH-nyckel), eller stäng av krypteringen om du avser att exporten ska vara klartext. Se [Funktioner](features.md).

## En databasdump misslyckades

En misslyckad dump fäller aldrig säkerhetskopian omkring den; den bokförs som en egen misslyckad körning, och orsaken säger vad som ska åtgärdas.

- **Inloggning nekad.** Dumpen loggar in med containerns egna lösenordsvariabler (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` eller deras `_FILE`-varianter). Kontrollera dem på databascontainern. En `_FILE`-variabel som pekar på en hemlighet containerns egen användare inte får läsa misslyckas på samma sätt.
- **Saknade rättigheter.** Med ett slumpmässigt root-lösenord kan dumpen bara logga in som appanvändaren och innehåller därmed bara den ena databasen, och MySQL 8.4 och senare kan neka den helt. Ge containern ett riktigt root-lösenord, eller stäng av dumpen för den.
- **Systemtabellerna behöver uppgraderas.** MariaDB vägrar dumpas när dess systemtabeller kommer från en äldre version (fel 1558). Lägg till variabeln `MARIADB_AUTO_UPGRADE=1` och starta om containern, eller kör `mariadb-upgrade` inuti den en gång.
- **Inget dumpverktyg.** En slimmad eller egenbyggd avbild utan `pg_dump`, `mysqldump` eller `mariadb-dump` går inte att dumpa. Använd den officiella avbilden, eller stäng av dumpen.
- **En tidsgräns.** En dump får `DB_DUMP_MAX_HOURS` (6 som standard), säkerhetskopian omkring får `BACKUP_MAX_HOURS`, och en dump som slutar göra framsteg kapas efter `BACKUP_STALL_HOURS`. Det sista beror oftast på ett lås som applikationen håller. Höj den gräns som slog till, eller dumpa medan applikationen är lugn.
- **Containern är pausad eller startar om.** Dumpen pratar med servern medan den kör. Om containern startar om gång på gång säger dess egen logg varför.
- **En skadad dump gick inte att ta bort.** En dump som BombVault inte kunde slutföra raderas igen. När den raderingen misslyckas står dumpen kvar i listan märkt som skadad, och du kan ta bort den där.

## En import misslyckades

En import stoppar containern, flyttar undan dess datamapp och låter avbilden skapa en tom i stället. Misslyckas ett steg före själva importen läggs den gamla mappen tillbaka av sig själv. Misslyckas importen behåller containern den färska mappen, och den gamla ligger kvar bredvid som `<datamapp>.bombvault-before-import-<tidsstämpel>`; körningens felmeddelande namnger den exakta sökvägen.

En datamapp som är en egen datauppsättning, som på TrueNAS, går inte att byta namn på. Importen låter då mappen ligga kvar och flyttar dess innehåll till den bevarade mappen bredvid, så datauppsättningen ovanför behöver plats för det. Importen kontrollerar det innan den stoppar containern.

Så lägger du tillbaka den för hand: stoppa containern, byt namn på den nuvarande datamappen så att den är ur vägen, byt tillbaka den bevarade mappen till det ursprungliga namnet och starta containern. För en datauppsättning flyttar du innehållet i stället för att byta namn på mapparna. På Unraid gör filhanteraren under fliken Shares detta.

## En säkerhetskopia av en ZFS-datauppsättning misslyckades eller hoppade över en uppsättning {#zfs-datasets}

Varje problem har en orsakskod inom hakparenteser, och sidan [ZFS-datauppsättningar](zfs-datasets.md#reason-codes) listar alla med åtgärden. De tre vanligaste:

- **`snapshot-loop`**: ögonblicksbilden nådde inte BombVault eftersom Host Data inte skickar vidare nya monteringar. Redigera containern, sätt Access Mode för Host Data till Read/Write - Slave och starta om BombVault.
- **`key-not-loaded`**: en krypterad uppsättning vars nyckel inte är laddad hoppas över. Ladda nyckeln med `zfs load-key` och montera uppsättningen; nästa säkerhetskopia tar med den.
- **`ssh-auth`**: servern avvisade BombVaults nyckel. **ZFS-anslutning till den här servern**, öppnad med **Visa anslutningen** på sidan Säkerhetskopior, visar kommandot som auktoriserar den; kör det en gång på servern.

## En post fastnar på "Lär sig N/10"

De flesta avvikelsekontroller startar efter 10 lyckade säkerhetskopior av en post, och räkningen börjar om efter **Markera som väntad** och efter att postens urval ändrats. En post som inte är schemalagd lär sig inte, och en container utan appdata har inget att lära sig av, vilket dess märke säger. **Inlärningsförlopp** på sidan Avvikelser visar en ring av tio delar per post.

## Retention slutade radera gamla säkerhetskopior av en post

En öppen kritisk avvikelse håller kvar dem: postens källa är nästan tom, har krympt kraftigt eller en säkerhetskopia lagrade det mesta av dess data igen. Öppna avvikelsen från märket på posten. Om data saknas eller krypterats, återställ först från den länkade sista goda säkerhetskopian. Kvittera sedan avvikelsen, eller markera den som väntad om ändringen var din, så gallrar nästa körning som vanligt. Retentionsförhandsvisningen markerar en sådan post som behållen. För en ZFS-post behåller bara datauppsättningen som nämns i avvikelsen sina gamla säkerhetskopior, och trädets övriga datauppsättningar gallras som vanligt.

## Manuell rensning säger att vissa poster behölls

Samma orsak: rensningen lämnar gamla säkerhetskopior av en post med en sådan avvikelse ifred och namnger posten i sitt meddelande. Allt annat rensas som vanligt.

## Historikimporten säger att ett repository inte kunde läsas

Efter uppgraderingen läser BombVault storlekarna på tidigare säkerhetskopior från varje arkiv en gång. Ett arkiv det inte kunde nå då, till exempel ett off-site-mål som låg nere eller en resurs som inte var monterad, räknas på kortet **Avvikelser** under **Inställningar, Verifiering** och försöks igen en gång om dagen. Dess poster lär sig under tiden av nya säkerhetskopior.

## Varningen om diskutrymme stämmer inte med Unraids instrumentpanel

På Unraids användarshare (`/mnt/user`) är det lediga utrymmet hela arrayens, inte en enskild disks. Fjärrepositorier mäts bara via rclone-fjärrar som rapporterar sitt lediga utrymme; S3-, B2-, REST- och SFTP-repositorier saknar uppgift och listas som ej uppmätta i kortet **Avvikelser**.

## En AI-assistent kan inte ansluta

Sidan [MCP-server](mcp.md#troubleshooting) visar vad varje statuskod och varje nekande från MCP-slutpunkten betyder och vad du kan göra åt det.

## Containern startar om hela tiden eller ser osund ut

BombVault rapporterar frisk/ofrisk från sitt eget `/api/health`. Ett autoläkningsverktyg (som Autoheal) kan starta om den automatiskt om motorn någonsin hänger sig. Kontrollera containerloggen och kortet **Värdintegration** för den underliggande orsaken.

## Fortfarande fast?

- Läs de fullständiga sidorna [Konfiguration](configuration.md) och [Off-site och återställning](offsite-recovery.md).
- Fråga på [Unraid-supporttråden](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Öppna ett [GitHub-ärende](https://github.com/junkerderprovinz/bombvault/issues).
