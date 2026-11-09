# Felsökning

En kort FAQ. För den fullständiga felsökningstabellen på värdsidan för VM över SSH (permission-denied, värdnyckelverifiering, saknade mallvariabler och mer), se [guiden för VM-säkerhetskopiering över SSH](vm-backup-ssh-setup.md).

## Något är inte inkopplat rätt {#something-is-not-wired-up-correctly}

Lägg till kortet **Värdintegration** på översikten under **Anpassa**. Värdintegrationskontrollen sonderar varje montering och CLI (Docker-socket, libvirt, restic, qemu-img, rclone) och rapporterar eventuella saknade delar, och **Kontrollera nu** kör den igen. Börja här innan du antar att det är en bugg: en saknad montering eller en värd som inte går att nå syns direkt.

## Jag når inte webbgränssnittet {#i-cannot-reach-the-web-ui}

BombVault serverar HTTPS direkt ur lådan på port `3443` (självsignerat certifikat), så öppna `https://<din-unraid-ip>:3443`. Godta varningen om det självsignerade certifikatet, eller lägg BombVault bakom en omvänd proxy med ditt eget certifikat. Om du kör med `HTTP_ONLY=true` serverar den i stället vanlig HTTP på port `3000` (avsett för användning bakom en proxy som avslutar TLS).

## Jag har tappat bort min APP_KEY {#i-lost-my-app_key}

`APP_KEY` härleder restic-repositoriets lösenord. Utan den, utan återställningskitet och utan en nyckeldeposition i din grupp går krypterade säkerhetskopior inte att återställa. Därför visar översikten **Spara ditt återställningskit** tills du har laddat ner det. Se [Off-site och återställning](offsite-recovery.md#recovery-kit). Generera en nyckel med `openssl rand -hex 32` och förvara den utanför servern innan du litar på någon säkerhetskopia.

## Jag har glömt inloggningslösenordet {#i-forgot-the-login-password}

Välj **Glömt lösenordet?** på inloggningssidan. BombVault skriver en kod på 12 tecken till sin containerlogg (på Unraid: fliken Docker, BombVault-ikonen, **Loggar**). Skriv in den med ett nytt lösenord inom 10 minuter. Återställningen stänger också av tvåfaktorsautentisering, tar bort varje lösennyckel, återkallar varje API-token, MCP-nyckel och OAuth-inloggning, avslutar varje annan session och stänger av telefonparkoppling. Lägg till dina lösennycklar igen under **Inställningar, Säkerhet**, slå på telefonparkoppling igen där och skapa nya token och nycklar där du fortfarande behöver dem. Se [Säkerhetsmodell](configuration.md#login).

## Säkerhetskopieringen står still och säger att nyckeln inte går att öppna {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

Den här servern tog över en annan servers säkerhetskopienyckel i Nödguiden, och `APP_KEY` ändrades efteråt. Den övertagna nyckeln är förseglad med den gamla APP_KEY, så BombVault kan inte öppna den och säkerhetskopierar ingenting i stället för att skriva med fel nyckel. **Behöver uppmärksamhet** på översikten nämner det. Sätt tillbaka APP_KEY till vad den var, eller ladda upp återställningskitet igen på sidan Återställning under **Nödläge**.

## En telefon hamnar på inloggningssidan {#a-phone-lands-on-the-login-page}

En parkopplad telefon får sin session via gruppen bara medan **Telefoner kan parkopplas med gruppens tolv ord** är på under **Inställningar, Säkerhet**. En lösenordsåterställning stänger av den. En app som är äldre än servern får heller ingen session förrän den uppdateras. En server som lagts till med sin adress frågar alltid efter lösenordet. En telefon som tagits bort från sidan Instanser kommer tillbaka avstängd när dess app öppnas igen. Slå på **Fråga av den här instansen** under **Övervakning** i dess detaljer för att släppa in den.

## VM-säkerhetskopiering ansluter inte {#vm-backup-will-not-connect}

VM-säkerhetskopiering pratar med libvirt över SSH, aldrig en montering.

- Kontrollera att SSH är på på värden och att BombVaults publika nyckel är auktoriserad i `/root/.ssh/authorized_keys` (Inställningar, Integrationer, Värd-SSH visar nyckeln och en knapp **Testa anslutning**).
- På ett anpassat `br0.x`-nätverk sätter du `LIBVIRT_HOST` till din Unraid-LAN-IP (containern når inte värden via `host.docker.internal` där). Slå på **Settings, Docker, Host access to custom networks**.
- Om du ändrat Unraids SSH-port, sätt `LIBVIRT_SSH_PORT` så att den stämmer.
- På TrueNAS sätter du `LIBVIRT_URI` med TrueNAS egen libvirt-socket, ger appen förmågan `SETPCAP` och lägger till nyckeln hos root under **Credentials, Users** (se [TrueNAS SCALE](getting-started.md#truenas)).
- Fullständig steg-för-steg-diagnos (nåbarhetstest, VLAN-routing, `Permission denied (publickey)`, `Host key verification failed`) finns i [guiden för VM-säkerhetskopiering över SSH](vm-backup-ssh-setup.md).

## En live-ögonblicksbild av en VM kördes inte {#a-live-vm-snapshot-did-not-run}

Live-ögonblicksbilder kräver att disken ligger på `/mnt/cache` (eller `/mnt/diskX`), inte `/mnt/user`. Med qemu guest agent i VM:en blir ögonblicksbilden vilande, och utan den är ögonblicksbilden kraschkonsistent. På en avstängd VM faller live automatiskt tillbaka på kontrollerad avstängning. En kontrollerad säkerhetskopiering stänger av VM:en, säkerhetskopierar diskarna och startar den sedan igen, så den är alltid konsekvent. Den väntar upp till tio minuter på att gästen ska stängas av och tvingar den aldrig, så en Windows-gäst som installerar uppdateringar avbryts inte halvvägs. En gäst som fortfarande kör efter det lämnas ifred, och den säkerhetskopian misslyckas med orsaken.

## En säkerhetskopiering misslyckades med "repository is already locked" {#a-backup-failed-with-repository-is-already-locked}

Det här är oftast ett föräldralöst restic-lås som lämnats kvar när containern uppdaterades eller startades om mitt i en åtgärd. BombVault upptäcker ett bevisligen föräldralöst lås, rensar det och försöker en gång till, automatiskt. Efter en omstart väntar BombVault tills ett sådant lås har gått tio minuter utan att förnyas. Ett restic som fortfarande körs förnyar sitt lås var femte minut, och BombVault tar aldrig bort ett lås som fortfarande förnyas, eftersom det kan tillhöra en andra BombVault som skriver till samma repositorium. Ett lås som har gått 30 minuter utan att förnyas tas bort, oavsett vem som tog det. Om felet består öppnar du kortet **Kontrollera säkerhetskopior** under **Inställningar, Verifiera** och väljer **Lås upp** under **Mer** på det berörda avsnittets rad för att rensa ett gammalt lås för hand. **Lås upp** låter också ett lås som fortfarande förnyas vara kvar och visar när det senast förnyades. Ett verkligt problem kommer fortfarande fram i stället för att döljas.

## Min off-site-kopia gjordes inte efter en säkerhetskopiering {#my-off-site-copy-did-not-happen-after-a-backup}

Off-site-replikering görs efter bästa förmåga med avsikt, så ett off-site-hack får aldrig den lokala säkerhetskopian att misslyckas. Kontrollera när lagringsmålet kopierar: dess kort **Kopiera** säger **Efter varje säkerhetskopia** eller nämner en egen plan, och **Inställningar, Schema, Externa kopior per avsnitt** kan ge ett avsnitt en egen plan. Använd **Replikera nu** på avsnittets rad i kortet **Externa kopior** under **Inställningar, Lagringsmål** för en körning på begäran, och titta på översiktens statushuvud, som säger **Extern kopiering pågår** medan den körs. En misslyckad kopia visas under **Behöver uppmärksamhet**.

## En rest-servers certifikat är inte betrott {#a-rest-servers-certificate-is-not-trusted}

restic vägrar ett certifikat det inte känner till, och felet säger var det ska ligga. Kopiera rest-serverns självsignerade certifikat, eller det från myndigheten som signerade det, som en PEM-fil till mappen `certs` i BombVaults datamapp (`/config/certs` i containern, `appdata/bombvault/certs` på Unraid) och testa igen. Ingen omstart behövs.

## En säkerhetskopiering säger att data utanför Host Data inte säkerhetskopierades {#data-outside-host-data}

Containern håller data i en mapp som BombVault inte kan läsa via Host Data. Säkerhetskopian sparar definitionen och varje mapp den når, gör sin rad i aktivitetsloggen orange och nämner de mappar den lämnade utanför. Två fall är vanliga: en ixVolume på TrueNAS under `/mnt/.ix-apps`, och en namngiven Docker-volym under `/var/lib/docker/volumes`, till exempel databasvolymen i en compose-stack. Kortet **Säkerhetskopierade mappar** på containerns flik **Inställningar** listar varje sådan mapp med sin sökväg inuti containern och sin värdsökväg, markerad i rött.

En anonym Docker-volym rapporteras inte. Docker skapar en av sig själv för en mapp som imagen deklarerar och som ingen mappat, ger den ett namn med 64 hex-siffror och håller den under `/var/lib/docker/volumes`. Den innehåller oftast en cache, så säkerhetskopian förblir grön och kortet listar den bara, med en anmärkning om att den är anonym.

- **Du vill ha den datan säkerhetskopierad.** Mappa mappen till en värdsökväg under Host Data, till exempel en mapp i din appdata-resurs, så tar nästa säkerhetskopia med den.
- **Du behöver den inte.** Lägg till en av dess två sökvägar som en rad i kortet **Uteslutningsmönster** på samma flik: sökvägen inuti containern, till exempel `/cache`, eller värdsökvägen som varningen nämner, till exempel `/var/lib/docker/volumes/jellyfin_cache/_data`. Från nästa säkerhetskopia lämnar varningen den mappen utanför, och förhandsvisningen under fältet säger varför. En rad för en mapp ovanför fungerar också, ett enkelt namn eller `/` gör det inte.

## En säkerhetskopiering säger Uppskjuten {#deferred-backups}

Containerns app var upptagen, så dess säkerhetskopiering väntar och provas igen av sig själv. Raden i aktivitetsloggen säger varför och när nästa försök är. Kortet **När appen är upptagen** på containerns flik **Inställningar** säger detsamma och när väntan tar slut. Inget behöver göras. För att säkerhetskopiera direkt använder du **Säkerhetskopiera nu**: en säkerhetskopiering du startar själv väntar aldrig, inte heller för en slutkod 75.

- **Din egen kontroll.** Låt **Kommando före säkerhetskopiering** avsluta med kod 75 medan appen är upptagen, till exempel `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Vilken annan kod som helst än 0 får säkerhetskopian att misslyckas som förut, så ett trasigt skript syns fortfarande under **Behöver uppmärksamhet**.
- **Plex, Jellyfin eller Emby frågas aldrig.** BombVault frågar först servern på containerns egen adress och kör, när den inte svarar, curl eller wget inuti containern. En image med ingetdera kan inte frågas via sig själv. Loggen nämner då det. Med **Säkerhetskopiera när servern inte svarar** på körs en sådan säkerhetskopiering ändå, med den av väntar den.
- **Jellyfin eller Emby nekar.** Kontrollera **API-nyckel**. Den görs i serverns instrumentpanel under API keys.
- **Plex nekar.** BombVault läser token från `Preferences.xml` i mappen som är monterad på `/config`. En server som aldrig loggats in på har ingen och svarar bara där den tillåter åtkomst utan.
- **En pausad film håller säkerhetskopian väntande hela natten.** Sätt **Ignorera pausade sessioner efter (min)**, till exempel till 30.
- **Trafikkontrollen släpper aldrig igenom säkerhetskopian.** Höj **Upptagen från (KiB/s)** över vad containern skickar medan ingen använder den. Direkt efter att BombVault startat väntar kontrollen tills den har bevakat containern i **Tyst i (min)** minuter.
- **Säkerhetskopian körs aldrig på natten.** Med ett fönster satt sker försöken bara inom det. En väntan som når sin tidsgräns utanför fönstret slutar ändå vid tidsgränsen.

## En stor säkerhetskopiering dödades för att minnet tog slut {#a-large-backup-was-killed-for-running-out-of-memory}

Sätt **Minnesgräns per säkerhetskopiering (MiB, 0 = ingen)** i kortet **Prestanda** under **Inställningar, System**, långt under vad containern får använda. Nära gränsen frigör restic minne oftare, vilket kostar lite fart, så säkerhetskopian går långsammare i stället för att dödas.

## En återställning avbröts innan den startade {#a-restore-aborted-before-it-started}

Innan något stoppas eller tas bort kör återställningen en konfliktkontroll i förväg: den kontrollerar att containerns statiska IP och publicerade värdportar är lediga. Om en annan container redan håller en av dem avbryts den med ett tydligt, användbart meddelande i stället för att lämna en halvfärdig återställning. Frigör den portar eller IP som krockar och försök sedan igen.

Om containerns nätverk inte finns, till exempel för att dess TrueNAS-app raderats, nämner återställningskontrollen nätverket och återställningen vägrar innan den ändrar något. Installera om appen i TrueNAS under samma namn, vilket skapar nätverket igen, och återställ sedan dess data.

## En vanlig export misslyckades i stället för att skriva en fil {#a-plain-export-failed-instead-of-writing-a-file}

Om **Kryptera exporter med age** är på under **Inställningar, System** men ingen giltig mottagare är satt misslyckas en export med ett tydligt fel i stället för att skriva klartext. Lägg till en giltig mottagare (en publik age-nyckel eller en publik SSH-nyckel), eller stäng av krypteringen om du vill att exporten ska vara klartext. Se [Vanliga exporter](backups.md#exports).

## En databasdump misslyckades {#a-database-dump-failed}

En misslyckad dump får aldrig säkerhetskopian runt den att misslyckas. Den loggas som en egen misslyckad körning, och orsaken nämner vad som ska åtgärdas.

- **Inloggningen nekades.** Dumpen loggar in med containerns egna lösenordsvariabler (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` eller deras `_FILE`-versioner). Kontrollera dem på databascontainern. En `_FILE`-variabel som pekar på en hemlighet som containerns egen användare inte kan läsa misslyckas på samma sätt.
- **Behörigheter saknas.** Med ett slumpmässigt root-lösenord kan dumpen bara logga in som appanvändaren, så den håller just den databasen, och MySQL 8.4 och nyare kan neka den helt. Ge containern ett riktigt root-lösenord, eller stäng av dumpen för den.
- **Systemtabellerna behöver en uppgradering.** MariaDB vägrar dumpa när dess systemtabeller kommer från en äldre version (fel 1558). Lägg till variabeln `MARIADB_AUTO_UPGRADE=1` och starta om containern, eller kör `mariadb-upgrade` inuti den en gång.
- **Inget dumpverktyg.** En slimmad eller egenbyggd image utan `pg_dump`, `mysqldump` eller `mariadb-dump` kan inte dumpas. Använd den officiella imagen, eller stäng av dumpen.
- **En tidsgräns.** En dump får `DB_DUMP_MAX_HOURS` (6 som standard), säkerhetskopian runt den får `BACKUP_MAX_HOURS`, och en dump som slutar göra framsteg avbryts efter `BACKUP_STALL_HOURS`. Den vanliga orsaken till den sista är ett lås som applikationen håller. Höj gränsen som slog till, eller dumpa medan applikationen är lugn.
- **Containern är pausad eller startar om.** Dumpen pratar med den körande servern. Om containern hela tiden startar om säger dess egen logg varför.
- **En skadad dump kunde inte tas bort.** En dump som BombVault inte kunde göra klar raderas igen. När den raderingen misslyckas stannar dumpen kvar i listan markerad som skadad, och du kan radera den där.

## En import misslyckades {#an-import-failed}

En import stoppar containern, flyttar undan dess datamapp och låter imagen skapa en tom i dess ställe. Om ett steg före själva importen misslyckas läggs den gamla mappen tillbaka automatiskt. Om importen misslyckas behåller containern den nya mappen och den gamla blir kvar bredvid som `<data folder>.bombvault-before-import-<timestamp>`. Körningens felmeddelande nämner den exakta sökvägen.

En datamapp som är en egen datauppsättning, som på TrueNAS, går inte att byta namn på. Importen lämnar då mappen på plats och flyttar dess innehåll till den bevarade mappen bredvid, så datauppsättningen ovanför behöver plats för det. Importen kontrollerar det innan den stoppar containern.

För att lägga tillbaka den för hand: stoppa containern, byt namn på den nuvarande datamappen så att den kommer ur vägen, byt tillbaka den bevarade mappen till det ursprungliga namnet och starta containern. För en datauppsättning flyttar du innehållet i stället för att byta namn på mapparna. På Unraid gör filhanteraren det från fliken Shares.

## En ZFS-säkerhetskopiering misslyckades eller hoppade över en datauppsättning {#zfs-datasets}

Varje problem bär en orsakskod inom hakparenteser, och sidan [ZFS-datauppsättningar](zfs-datasets.md#reason-codes) listar alla med åtgärd. De tre vanligaste:

- **`snapshot-loop`**: ögonblicksbilden nådde inte BombVault eftersom Host Data inte släpper igenom nya monteringar. Redigera containern, sätt Access Mode för Host Data till Read/Write - Slave och starta om BombVault.
- **`key-not-loaded`**: en krypterad datauppsättning vars nyckel inte är laddad hoppas över. Ladda nyckeln med `zfs load-key` och montera datauppsättningen. Nästa säkerhetskopia tar med den.
- **`ssh-auth`**: servern nekade BombVaults nyckel. **ZFS-anslutning till den här servern**, som öppnas med **Visa anslutningen** på sidan Säkerhetskopior, visar kommandot som auktoriserar den. Kör det en gång på servern.

## En post står kvar på "Learning N/10" {#an-entry-stays-at-learning-n10}

De flesta avvikelsekontroller börjar efter 10 lyckade säkerhetskopior av en post, och räkningen börjar om efter **Markera som väntad** och efter att postens urval ändrats. En post som inte är schemalagd lär sig inte, och en container utan appdata har inget att lära sig av, vilket dess märke säger. **Inlärningsförlopp** på sidan Avvikelser visar en ring med tio delar per post.

## Gallringen slutade radera gamla säkerhetskopior av en post {#retention-stopped-deleting-old-backups-of-one-entry}

En öppen kritisk avvikelse håller kvar dem: postens källa är nästan tom, har krympt kraftigt eller en säkerhetskopia lagrade det mesta av dess data på nytt. Öppna avvikelsen från märket på posten. Om data saknas eller har krypterats, återställ först från den länkade sista goda säkerhetskopian. Kvittera sedan avvikelsen, eller markera den som väntad om ändringen var din, så rensar nästa körning som vanligt. Gallringsförhandsvisningen markerar en sådan post som behållen. För en ZFS-post behåller bara datauppsättningen som nämns i avvikelsen sina gamla säkerhetskopior, och trädets andra datauppsättningar rensas som vanligt.

## Manuell rensning säger att vissa poster behölls {#manual-prune-says-some-entries-were-kept}

Samma orsak: rensningen lämnar gamla säkerhetskopior av en post med en sådan avvikelse ifred och nämner posten i sitt meddelande. Allt annat rensas som vanligt.

## En säkerhetskopiering misslyckas med "on no mounted pool or share" {#no-mounted-pool}

På Unraid ligger `/mnt` i minnet, så en mapp där utan en monterad pool är borta vid nästa omstart och dold när poolen är tillbaka. BombVault vägrar att skriva en säkerhetskopia eller en flash-ZIP dit, även när ett repositorium från en tidigare skrivning redan ligger i den mappen. Montera poolen och kör säkerhetskopieringen igen.

## Historikimporten säger att ett repositorium inte kunde läsas {#history-import-says-a-repository-could-not-be-read}

Efter uppgraderingen läser BombVault storlekarna på tidigare säkerhetskopior från varje repositorium en gång. Ett repositorium som inte gick att nå då, till exempel ett off-site-mål som var nere eller en delad resurs som inte var monterad, räknas på kortet **Avvikelser** under **Inställningar, Verifiera** och provas igen en gång om dagen. Dess poster lär sig av nya säkerhetskopior under tiden.

## Varningen om diskutrymme stämmer inte med Unraids instrumentpanel {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

På Unraids användarresurs (`/mnt/user`) är det lediga utrymmet hela arrayens, inte en disks. Lagringsmål på distans som BombVault når via rclone, till exempel SFTP, Hetzner Storage Box, SMB, WebDAV eller en molndisk, mäts när andra sidan rapporterar sitt lediga utrymme, och det gör även en `sftp:`-adress som skrivits in för hand. S3-, B2- och REST-repositorier har inget värde: deras lagringsmål säger **Storlek okänd**, och kortet **Avvikelser** listar dem som inte mätta.

## En AI-assistent kan inte ansluta {#an-ai-assistant-cannot-connect}

[MCP-serversidan](mcp.md#troubleshooting) listar vad varje statuskod och varje nekande från MCP-slutpunkten betyder och vad du kan göra åt det.

## Containern startar hela tiden om eller ser ohälsosam ut {#the-container-keeps-restarting-or-looks-unhealthy}

BombVault rapporterar frisk/ohälsosam från sitt eget `/api/health`. Ett verktyg som reparerar sig själv (till exempel Autoheal) kan starta om den automatiskt om motorn någonsin hänger sig. Kontrollera containerloggen och kortet **Värdintegration** för den underliggande orsaken.

## Ett lagringsmål vägrar sparas {#location-refused}

BombVault kontrollerar ett mål innan det sparar det, och nekandet säger varför i en mening. De vanliga:

- **"This location lies inside another repository or target, or contains one."** Ett repositorium inuti ett repositorium skulle förstöra båda. Meddelandet nämner den andra platsen. Välj en annan mapp, eller ta först bort den.
- **Det är ett repositorium du satt upp.** Målet är en av raderna under **Arkiv**, eller ligger inuti eller runt en. Välj en annan plats, eller ta först bort det repositoriet.
- **Samma plats som ett avsnitts egna mapp.** En kopia till ett avsnitts egna mapp skulle kopiera dess säkerhetskopior på sig själva vid varje körning, så den nekas. Välj en annan plats, eller se [Ett avsnitts egna mapp](offsite-recovery.md#own-folder).
- **"That is not an off-site target of this domain"** kom från en länk till ett mål som tagits bort. En aktuell version rensar de vid start och i en import. Om du fortfarande ser det, uppdatera BombVault.
- **En fullständig värdsökväg** som `/mnt/user/backups`. BombVault körs i en container, så den vill ha sökvägen under Host Data, `user/backups`, och meddelandet nämner den du ska skriva.

## Ett lagringsmål på distans säger att dess repositorium är borta {#repository-gone}

Ett mål som höll säkerhetskopior och plötsligt inte håller något repositorium får inget nytt, tomt av sig självt, eftersom det skulle dölja det verkliga när en disk är tillbaka. En rest-server vars disk inte monterades är den vanliga orsaken, så montera disken igen och kör säkerhetskopian eller kopian en gång till. Bara om du tömde eller raderade det gamla repositoriet med avsikt trycker du på **Skapa ett nytt arkiv här** på målets sida. Det börjar om för varje avsnitt i det målet på en gång.

## En molninloggning har gått ut {#cloud-sign-in-expired}

Testet eller nästa kopia säger då att inloggningen gick ut. **Ändra anslutning** på målets sida loggar in på nytt.

- **Box, Jottacloud och andra med engångstoken för förnyelse:** BombVault behåller token som rclone förnyade, även över omstarter och i återställningskitet. Efter en återställning från en äldre inställningssäkerhetskopia kan token i den ha gått ut, så logga in en gång till. Repositoriet förblir som det är.
- **iCloud Drive:** Apple avslutar inloggningen efter 30 dagar. Den behöver en ny förtroendetoken och nya kakor från `rclone config`.
- **OneDrive:** en inloggning med flera diskar frågar vilken som ska användas, så ett mål som lämnats utan val förblir oanvändbart tills du väljer. För OneDrive Business och Google Drive, se anmärkningen om rensad data under [Lagringsmål](offsite-recovery.md#destinations).

## "Off-site-kopian av den senaste säkerhetskopian pågår fortfarande" {#copy-still-running}

Säkerhetskopian före blev klar och dess kopia är fortfarande på väg, vilket blockerar en ny säkerhetskopiering av det avsnittet. Vänta tills statushuvudet inte längre säger **Extern kopiering pågår**. Knapparna som startar en säkerhetskopiering säger detsamma medan de väntar. Den första kopian till ett nytt mål kan ta timmar, se [S3-exemplet](offsite-recovery.md#example-s3).

## En ZFS-replik stannar för att målet har nyare ögonblicksbilder {#replica-target-newer}

Målet har ögonblicksbilder som är nyare än den nyaste det delar med den här servern, till exempel för att datauppsättningen här rullades tillbaka eller kopian på målet ändrades. Kortet Replik och körhistoriken nämner ögonblicksbilden under **Rulla tillbaka målet till**. Kör `zfs rollback -r` till den ögonblicksbilden på målet, för varje datauppsättning som körningen nämner, och tryck på **Replikera nu**. Se [Hur länge ögonblicksbilder stannar](zfs-datasets.md#replica-retention).

## En VM med Unraid-ögonblicksbilder misslyckas och nämner en fil {#vm-chain}

BombVault följer varje disks stödkedja och säkerhetskopierar varje fil i den. En kedja som inte går att följa till slutet får säkerhetskopian att misslyckas i stället för att utelämna en del av disken, och meddelandet nämner filen. Kontrollera att filen finns under `/mnt` och att ögonblicksbilden fortfarande listas i VM Manager. En fil utanför Host Data går inte att läsa alls. Se [VM:ar med Unraid-ögonblicksbilder](vm-backup-ssh-setup.md#unraid-snapshots).

## Off-site-kopior väntar på ditt beslut efter en återuppbyggnad {#copies-paused}

Efter **Identifiera säkerhetskopior** eller Nödguiden utan återställda inställningar, eller när ett avsnitt hittar egna säkerhetskopior från tiden före den här databasen, väntar off-site-kopior av det avsnittet, eftersom BombVault inte längre vet vilka poster du lämnat utanför. **Behöver uppmärksamhet** och lagringsmålets sida frågar om det. Tryck på **Ta över dem som den här serverns egna** för att fortsätta, eller på **Kontrollera reglerna först** för att titta på avsnittets regler innan du gör det. Att importera en inställningsfil fattar inte beslutet åt dig. Se [Efter en återuppbyggnad](offsite-recovery.md#after-a-rebuild).

## Aviseringar kommer inte fram {#notifications-do-not-arrive}

Börja med **Skicka test** på kanalen under **Inställningar, Aviseringar**. Raden under kanalen säger vad som hände.

- **Ingenting skickas alls.** Kontrollera att kanalen är påslagen och titta på dess **Avisera**. Med **Aldrig** skickar den ingenting, och med **Endast vid fel** förblir en lyckad körning tyst. Med **Sammanfatta schemalagda körningar** på skickar en schemalagd körning en sammanfattning vid sitt slut.
- **En ny adress.** Efter att du ändrat vart en kanal skickar, till exempel servern för Matrix, Gotify eller ntfy eller värden för en e-postkanal, frågar fältet efter token eller lösenord igen, och den nya adressen sparas först när du skrivit det. BombVault skickar inte den gamla hemligheten till ett nytt ställe.
- **Gotify** säger att servern nekade app-token: kopiera den igen från applikationen i Gotify. **Serverns URL** är basadressen, utan `/message`.
- **Discord, Slack och ntfy** är egna kanaler. En Discord-adress i en **Webhook**-kanal får JSON som Discord inte förstår, så lägg till en **Discord**-kanal för den.
- **Webhook:** en platshållare som BombVault inte känner till står kvar som den är skriven, så kontrollera stavningen, till exempel `%%title%%`. För ett JSON-innehåll låter du rubriken `Content-Type` stå kvar på `application/json`, så att värdena escapas som JSON.
- **Unraid-aviseringar** går över Värd-SSH-anslutningen, så testa den först under **Inställningar, Integrationer**. På en annan plattform förblir de av, och kanalen säger det.
- **Ett aviseringsmejl kommer aldrig.** Kontrollera **Kryptering** mot porten, oftast **STARTTLS** på 587 eller **TLS (implicit)** på 465.

Healthchecks-pingar beror inte på **Avisera**, se [Healthchecks](notifications.md#healthchecks).

## Android-appen kan inte öppna en server {#android-cannot-open}

Startskärmen säger varför på serverns kort:

- **Inte ansluten:** varken serverns adress eller reläet svarar. På **Inget relä** når appen en server bara på dess egen adress.
- **Kontrollera certifikat:** servern visar ett annat certifikat än det du litade på. Appen öppnar den först när du litar på det nya, så se till att ändringen är din.
- **Inloggning behövs:** en server som lagts till med sin adress frågar efter lösenordet. Se även [En telefon hamnar på inloggningssidan](#a-phone-lands-on-the-login-page).
- **Nyckeln ändrades:** startskärmen visar den nya nyckelns fingeravtryck, och sidan öppnas först när du litar på den. Se [Borta från hemmet](android.md#away).

## En annan användare kan inte läsa det lokala repositoriet {#repository-permissions}

Det lokala repositoriet förblir läsbart för andra användare, till exempel en synkanvändare över SMB. Före 10.4.1 kunde filer som en rensning, tagg, hämtning, avsnittsflytt, radering eller stackbackup skrev förbli oläsbara till nästa natt. Uppdatera BombVault, så är de läsbara direkt. Repositoriet för auto-säkerhetskopian förblir läsbart bara för root, med avsikt, eftersom det innehåller BombVaults inställningar.

## Kommer du inte vidare? {#still-stuck}

- Läs de fullständiga sidorna [Konfiguration](configuration.md) och [Off-site och återställning](offsite-recovery.md).
- **Felrapport** på kortet **Hjälp** under **Inställningar, Info** laddar ner en ZIP-fil för ett GitHub-ärende: värdkontrollen, din konfiguration med varje hemlighet borttagen, de senaste körningarna, vad som är schemalagt härnäst och den senaste loggen sedan containern senast startade. Den innehåller också den senaste dumpen för varje databas, ZFS-posterna med sina monteringar, de öppna avvikelserna och hur många MCP-nycklar som finns. Lösenord, token och rclone-konfigurationen tas bort, och paketet säger det i sitt eget manifest. Den kräver ett inloggningslösenord. För en krasch som startade om containern, titta i `docker logs`.
- Fråga i [Unraids supporttråd](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Öppna ett [GitHub-ärende](https://github.com/junkerderprovinz/bombvault/issues).
