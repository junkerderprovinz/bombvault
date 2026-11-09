# Säkerhetskopior och poster

Allt som BombVault säkerhetskopierar står i en enda lista under **Säkerhetskopior**: containrar, VM:ar, mappuppsättningar, flashenheten, ZFS-datauppsättningar och auto-säkerhetskopian. Var och en av dem är en **post** med en egen sida. Den här sidan går igenom listan, postens sida med sina tre flikar och de inställningar en post kan ha.

![Listan Säkerhetskopior med brickan Containers vald: varje rad visar sina senaste körningar som en remsa och en enda status.](assets/screenshots/backups/list-dark.png#only-dark)
![Listan Säkerhetskopior med brickan Containers vald: varje rad visar sina senaste körningar som en remsa och en enda status.](assets/screenshots/backups/list-light.png#only-light)

## Vad varje avsnitt sparar {#what-is-saved}

| Avsnitt | Vad som sparas |
|---|---|
| **Containers** | Appdata-mappen plus containerdefinitionen: image, variabler, portar, etiketter och volymer. Kortet **Säkerhetskopierade mappar** på containerns flik **Inställningar** bockar exakt vilka mappar säkerhetskopian täcker, med en levande räkning av sökvägarna och en lista över det du lämnat utanför. Data som en container håller utanför Host Data, till exempel i en TrueNAS ixVolume eller ett Compose-projekt under `/opt`, säkerhetskopieras inte, och körningen nämner de mappar den lämnade utanför. Körningen nämner också en mapp i Host Data som inte finns just nu. Systemsökvägar som `/etc/localtime` eller Docker-socketen räknas inte som data. |
| **VMs** | Diskavbilderna, XML-definitionen och UEFI NVRAM, med en kontrollerad avstängning eller en live-ögonblicksbild, över SSH. Se [VM-säkerhetskopior](vm-backup-ssh-setup.md). |
| **Flash** | Bara Unraid. Hela USB-flashen (`/boot`): OS, licens, array-config, resurser, nätverks- och plugin-config. |
| **Auto-säkerhetskopia** | BombVaults egen `/config`: dess inställningsdatabas, inloggningarna till lagringsmålen, libvirt-SSH-nyckelparet, ZFS-servrarnas nyckel och fästa värdnycklar samt certifikaten, tagna med SQLites `VACUUM INTO` så att databasen aldrig fångas mitt i en skrivning. |
| **Mappar** | Valfri mapp på servern som en mappuppsättning, till exempel en delad resurs, dina dokument eller ett fotobibliotek, med valfria exkluderingsmönster. Mappuppsättningar har scheman, gallring, externa kopior, kontroller och återställningsövningar som allt annat. |
| **ZFS-datauppsättningar** | En datauppsättning med alla uppsättningar under den, läst från en ZFS-ögonblicksbild så att alla kommer från samma ögonblick, och sparad med restic som en mapp. En ZFS-post kan också hålla en blockexakt replik på en andra ZFS-server. Se [ZFS-datauppsättningar](zfs-datasets.md). |

Varje säkerhetskopia går genom restic, så den är inkrementell och deduplicerad, och även stora VM-diskar blåser inte upp repositoriet. BombVault säkerhetskopierar aldrig sin egen container, och stoppar den därför aldrig. En körning kan stoppas säkert, eftersom restic skriver sin ögonblicksbild sist, så en avbruten körning lämnar orefererad data men ingen ögonblicksbild. Ett lås som lämnats kvar av en omstart mitt i en körning rensas och körningen provas en gång till, och först när låset gått tio minuter utan att förnyas.

## Listan {#list}

Brickor ovanför listan väljer ett slag, till exempel **Containers** eller **ZFS**, och varje bricka räknar sina poster. En bricka väljer, den döljer inte resten av sidan.

Varje rad visar postens senaste fjorton körningar som små rutor och en enda status, till exempel **Skyddad**, **Inte skyddad**, **Uppskjuten** eller **Replikerar**. Allt annat om den står i infobubblan bredvid namnet, och medan pekaren vilar på raden står den texten i själva raden. Ett märke bredvid en containers namn säger att den återskapats med en annan image, andra portar, variabler eller volymer sedan sin senaste säkerhetskopia. Det försvinner vid nästa säkerhetskopia.

Tre knappar ovanför listan avgränsar den:

- **Filter**: efter **Schema** (**Alla**, **Schemalagd**, **Pausad**), efter **Säkerhetskopia** (**Säkerhetskopierad**, **Aldrig säkerhetskopierad**) och, för **Containers och VMs**, **Installerade** eller **Inte installerad**.
- **Sortera**: efter namn, status, IP, senaste säkerhetskopia, storlek eller schema. Varje fält utom IP sorterar åt båda hållen, till exempel **Storlek, största först** och **Storlek, minsta först**.
- **Sök**: efter namn, image eller IP.

Poster som inte längre är installerade men som fortfarande har säkerhetskopior står för sig under **Inte installerad (enbart säkerhetskopior)**. Återställ dem, eller radera deras säkerhetskopior för att frigöra utrymmet.

Sidans huvudåtgärd svävar nere till höger: **Säkerhetskopiera allt nu** startar en körning över varje påslaget avsnitt. **Mer** innehåller resten:

- **Markera** startar ett urval, se nedan.
- **Identifiera säkerhetskopior** läser vad lagringsmålen innehåller och bygger upp poster av det, till exempel efter en ny installation. En container utan egna data har ingen ögonblicksbild, och den kommer tillbaka från definitionen som speglas bredvid repositoriet. Externa kopior av återuppbyggda avsnitt väntar sedan tills du tar över deras säkerhetskopior som den här serverns egna, se [Efter en återuppbyggnad](offsite-recovery.md#after-a-rebuild).
- **ZFS-anslutning till den här servern** öppnar SSH-testet för ZFS-avsnittet, se [ZFS-datauppsättningar](zfs-datasets.md#requirements).
- **Replikera allt nu**, med brickan ZFS vald, skickar repliken för varje ZFS-post.

![Samma lista på en telefon.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![Samma lista på en telefon.](assets/screenshots/backups/list-phone-light.png#only-light)

## Välja flera poster {#select}

Välj rader som du väljer filer i en filhanterare: Ctrl+klick, Skift+klick, Ctrl+A, eller ett långt tryck på en telefon. En vald rad visar en bock i stället för sin ikon, och ett fält ovanför listan räknar dem och erbjuder **Säkerhetskopiera markerade**, **Pausa schemat** och **Återuppta schemat**, **Markera alla** och **Rensa markering**. Varje rad har också en högerklicksmeny med sina egna åtgärder.

Ett urval säkerhetskopieras på servern, en post i taget, så det fortsätter när du stänger fliken. För att återställa flera poster tillsammans använder du [sidan Återställning](restore.md#several).

![Två containrar valda, med fältet ovanför listan.](assets/screenshots/backups/bulk-dark.png#only-dark)
![Två containrar valda, med fältet ovanför listan.](assets/screenshots/backups/bulk-light.png#only-light)

## Lägga till en mapp eller en ZFS-datauppsättning {#add}

Containrar, VM:ar och flashenheten dyker upp av sig själva när deras avsnitt slås på under **Inställningar, Allmänt**. Mappar och ZFS-datauppsättningar väljer du själv, eftersom BombVault inte kan gissa vilka du menar. **Lägg till** öppnar **Vad mer ska BombVault säkerhetskopiera?**:

- **En mapp** listar de delade resurser BombVault hittat och tar vilken annan mapp som helst under Host Data. Ge mappuppsättningen ett namn, till exempel `Documents`. Namnet är fast efter den första säkerhetskopian, eftersom ögonblicksbilderna lagras under det.
- **En ZFS-datauppsättning** listar datauppsättningarna på den här servern. Välj den överst i det du vill säkerhetskopiera, se [Poster och underordnade datauppsättningar](zfs-datasets.md#items-and-children).
- **Systemkonfiguration** visas på en värd utan Unraid vars Host Data når `/etc`. Det är en förslagen `/etc`-mappuppsättning som du granskar innan du sparar.

## Lämna en post utanför {#dont-back-up}

En post som du lämnar utanför med avsikt, till exempel en testcontainer, kan markeras **Säkerhetskopiera inte**. Den räknas då inte längre som oskyddad, och **Undanta inte längre** tar tillbaka den. **Pausa schemat** är det mjukare sättet: posten lämnar schemat och den totala säkerhetskopian, får ett grått märke **Schemat pausat**, och **Säkerhetskopiera nu** fungerar fortfarande. Det sätter samma omkopplare som **I schemat** på postens flik Inställningar.

## Postens sida {#entry-page}

Ett klick på en rad öppnar postens sida. Huvudet nämner posten och dess läge på en rad, till exempel **Senast säkerhetskopierad för 3 timmar sedan** eller **Uppskjuten, nästa försök 03:15**, och innehåller dess åtgärder: **Säkerhetskopiera nu**, **Återställ**, och under **Mer** de sällsynta, till exempel **Exportera senaste säkerhetskopian**, **Exportera container** eller **Exportera VM**, **Länka till en tidigare post**, **Ta bort alla säkerhetskopior**, **Ta bort post** och **Ta bort säkerhetskopior överallt**. En bakåtknapp till vänster om ikonen leder till listan. Medan en säkerhetskopiering körs står **Avbryt säkerhetskopiering** bredvid dess förlopp, och en avbruten körning loggas som avbruten, inte som misslyckad.

**Ta bort säkerhetskopior överallt** tar bort postens säkerhetskopior på alla lagringsmål på en gång, externa kopior inräknade. Fönstret visar varje plats med hur många säkerhetskopior den har, och knappen låses upp när du skriver postens namn. En plats som är inställd på append-only behåller sina säkerhetskopior och säger det. Posten och dess inställningar finns kvar, så en post du inte längre säkerhetskopierar förblir så, och bara dess säkerhetskopior och deras historik försvinner.

Sidan har tre flikar: **Säkerhetskopior**, **Inställningar** och **Verifiering**.

### Fliken Säkerhetskopior {#backups-tab}

Den här fliken listar postens ögonblicksbilder, de nyaste först, med ett fält per dag. En säkerhetskopia som ligger på flera ställen visas en gång, märkt med varje lagringsmål som har den. Ett klick på en ögonblicksbild återställer den, och ett klick på ett ställe inuti en ögonblicksbild återställer den från exakt där.

- **Mål** nämner varje plats med säkerhetskopior av den här posten och hur många. Externa platser läses först när du trycker på **Kontrollera**, eftersom det tar en stund att lista molnlagring. En plats som posten inte längre säkerhetskopierar till står som **Bara äldre säkerhetskopior** och kan fortfarande återställas från.
- Varje ögonblicksbild har en meny: **Lägg till tagg** eller **Ändra tagg**, **Jämför med en annan säkerhetskopia**, som listar de filer som tillkom, försvann eller ändrades, **Exportera** som tar, zip eller ett eget restic-repositorium, och radera på bara ett ställe.
- **Databasdumpar** för en container står i en egen lista, med nedladdning, sparande i en mapp och import.

Hur många ögonblicksbilder som stannar bestäms av lagringsmålet, se [Hur länge det bevaras](offsite-recovery.md#keep).

![Fliken Säkerhetskopior för en container: ett fält per dag och de lagringsmål som har varje ögonblicksbild.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![Fliken Säkerhetskopior för en container: ett fält per dag och de lagringsmål som har varje ögonblicksbild.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Fliken Inställningar {#settings-tab}

Varje post börjar med **Schema och mål**:

- **I schemat** tar posten in i eller ur de schemalagda körningarna.
- **Egen plan** ger just den här posten en egen plan, se [En egen plan för en post](schedules.md#own-plan). Utan en körs den enligt sitt avsnitts plan eller i den totala säkerhetskopian.
- En rad knappar, **Lokal** och en per externt lagringsmål, säger vart säkerhetskopiorna går. **Följer standarden** betyder att posten går dit dess avsnitt går, och ett klick på den öppnar **Mål per avsnitt**. Se [Placering per post](offsite-recovery.md#placement).
- **Repository** nämner var säkerhetskopiorna skrivs (**Sparad på**). Den platsen är fast från den första säkerhetskopian, kopiorna kan ändras när som helst.

Korten nedanför beror på typen av post:

| Typ | Kort |
|---|---|
| Container | **Säkerhetskopierade mappar** (vilka mappade mappar som säkerhetskopieras, med **Hoppa över cachemappar (CACHEDIR.TAG)**), **Uteslutningsmönster**, **Stoppa andra containrar**, **Kommandon före och efter**, **Databas**, **När appen är upptagen**, **Uppdatera efter en lyckad säkerhetskopiering**, **Ögonblicksbild i stället för stopp**, **Tidigare namn** |
| VM | **VM** med **Metod** (**Mjuk (avstängning)** eller **Live-ögonblicksbild**) och **Endast ändrade block**, se [guiden för VM-säkerhetskopiering](vm-backup-ssh-setup.md#per-vm-method) |
| Mappuppsättning | **Mapp** och **Uteslutningsmönster** |
| Flashenhet | **Flash-ZIP-export**, som skriver ut varje flash-säkerhetskopia som en vanlig `.zip` till en mapp du väljer, i ett enda steg så att ett synkverktyg aldrig ser en halv fil. En flash-säkerhetskopia som innehåller mindre än den borde lämnar den senaste ZIP-filen på plats, och exportens körning säger det. |
| ZFS-datauppsättning | de underordnade datauppsättningarna, containrarna som ska stoppas och kommandona kring ögonblicksbilden, **Replik** och **Testa åtkomsten till ögonblicksbilder**, se [ZFS-datauppsättningar](zfs-datasets.md) |

Inställningarna sparas av sig själva medan du ändrar dem.

![Fliken Inställningar för en container, med Egen plan i det första kortet.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![Fliken Inställningar för en container, med Egen plan i det första kortet.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Fliken Verifiering {#verify-tab}

- **Kontroller** visar återställningskontrollen som följer på den första säkerhetskopian. **Kontrollera återställning** kör den igen på den nyaste säkerhetskopian: BombVault återställer ett urval på upp till 100 filer och 256 MiB till en tillfällig mapp och låter restic läsa tillbaka varje fil mot dess hashar.
- **Starttest**, på en container, återställer den nyaste säkerhetskopian till en isolerad kopia och startar den: ett namn som börjar med `bombvault-test-`, ett eget internt Docker-nätverk utan publicerade portar och utan väg till LAN, 1 CPU och 2 GiB minne. Den klarar testet när kopian rapporterar frisk, svarar på sin första exponerade port, eller, med ingetdera, fortsätter att köra. Originalet berörs aldrig, och kopian, dess nätverk och dess data tas bort efteråt, även efter en omstart mitt i ett test. Containrar på värdnätverket, privilegierade, sådana med enheter och sådana som behöver en annan container visas som inte testbara. **Starttest** under **Inställningar, Verifiera, Automatiska kontroller** testar en container per körning, den som testades längst sedan först.
- **Avvikelser** sätter postens egen känslighet och aviseringsnivå och visar dess öppna fynd.
- **Storlek per mapp** visar vad som tar plats i den nyaste säkerhetskopian och hur mycket av det den senaste säkerhetskopian tog in, en nivå i taget.
- **Senaste körningar** listar de senaste körningarna med start, slut och varaktighet. Ett klick visar detaljerna.

![Fliken Verifiering för en container med Storlek per mapp öppen.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![Fliken Verifiering för en container med Storlek per mapp öppen.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Uteslutningsmönster {#excludes}

Kortet **Uteslutningsmönster** tar ett mönster per rad. Skriv sökvägar som du ser dem inne i containern, till exempel `/config/Library/Application Support/Plex Media Server/Cache`, eller ett enkelt namn som `.git`, som matchar på vilket djup som helst. För en mappuppsättning skriver du sökvägen som värden känner den, till exempel `/mnt/user/data/cache`. En förhandsvisning under fältet visar vad varje rad blir, och varnar när en rad inte matchar något eller skulle utesluta allt. En rad med `/` eller `*`, eller en som nämner en hel säkerhetskopierad mapp, ignoreras, så en uteslutning kan aldrig tömma en säkerhetskopia. För att lämna en hel mapp utanför bockar du ur den under **Säkerhetskopierade mappar** i stället.

För välkända images som Plex, Jellyfin, Sonarr, Radarr, Immich och Nextcloud erbjuder kortet de mappar som appen fyller på igen av sig själv, till exempel cachar, loggar och miniatyrer. Ingenting utesluts förrän du trycker på **Undanta markerade**.

En ny mappuppsättning börjar med gruppen **Systemskräp** påslagen: `.DS_Store`, `Thumbs.db`, `desktop.ini`, `@eaDir` (Synology), `.Recycle.Bin` (Unraids Recycle Bin-plugin), `.Trash-*` och `*.tmp`. Vart och ett är ett enkelt namn, så det matchar på vilket djup som helst under mappen, och du kan stänga av vilket som helst av dem innan du skapar uppsättningen. En befintlig mappuppsättning behåller de undantag den har. **Undanta systemskräp** i dess redigeringsfönster erbjuder samma rader, och de hamnar i uppsättningen först när du trycker på **Undanta markerade** och sparar.

Samma kort bär varningar som en mappskanning inte kan se. Immich håller till exempel sina album, ansikten och datum i en PostgreSQL-databas i en separat container, så en filsäkerhetskopia av enbart Immich-containern återställer bilderna utan något av det, och återställningen ser ut att ha fungerat. Säkerhetskopiera databascontainern också.

## Databasdumpar {#database-dumps}

PostgreSQL-, MySQL- och MariaDB-containrar som BombVault känner igen dumpas före varje säkerhetskopia, från den körande servern, till en egen återställningspunkt bredvid filerna. Det täcker de officiella images, PostGIS, TimescaleDB, pgvector, pgautoupgrade, Immichs databas-images, MariaDB-images från linuxserver, yobasystems och jc21 samt Oracles mysql-server. En container som bara ser ut som en databas får samma alternativ, avstängt tills du väljer det.

Dumpen strömmar rakt in i repositoriet och skrivs aldrig till en disk. Inloggningsuppgifterna kommer från containerns egna variabler, `*_FILE`-hemligheter inkluderade, och lämnar den aldrig. Kortet **Databas** på containerns flik Inställningar säger om datamappen sparas medan den är stoppad, kopieras medan den körs eller inte sparas, och slår på eller av dumpen för den här containern. Etiketten `bombvault.dbdump=false` gör detsamma och gäller före kortet. **Automatiska databasdumpar** under **Inställningar, Allmänt** slår på eller av den för alla.

En misslyckad dump får inte säkerhetskopian att misslyckas. Den visas som en egen misslyckad körning med orsaken och skickar en avisering. Dumpar laddas aldrig tillbaka på egen hand: ladda ner en, ren eller komprimerad, spara den i en mapp, eller importera den till en nystartad databas med ett klick på fliken **Säkerhetskopior**. För att få tillbaka en dump utan BombVault, se [Få tillbaka en databasdump](offsite-recovery.md#database-dumps). När en misslyckas, se [En databasdump misslyckades](troubleshooting.md#a-database-dump-failed).

## Kommandon och andra containrar {#commands}

- **Kommandon före och efter** körs inuti containern med `sh -c`, till exempel för att tömma en cache till disk. Ett kommando före säkerhetskopian som misslyckas avbryter den. Ett kommando före säkerhetskopian som avslutas med kod 75 skjuter i stället upp en schemalagd säkerhetskopiering, se nedan.
- **Stoppa andra containrar** nämner containrar som ska stoppas medan den här säkerhetskopieras, till exempel dess databas. Efteråt startar de igen i sin Compose-`depends_on`-ordning, och som standard måste var och en rapportera frisk innan nästa startar. Väntan och dess tidsgräns finns under **Starta om efter säkerhetskopiering** i kortet **Ordning** under **Inställningar, Schema**.
- En container som delar nätverks-, PID- eller IPC-namnrymd med en som stoppas, genom `--net=container:` eller Composes `network_mode: service:`, stoppas och startas med den även när den inte står på listan, och den startar efter containern den ansluter till. En omstartad container får ett nytt nätverk, och en som lämnades igång på det gamla nätverket skulle bara ha sin loopback. Själva listan förblir som du ställde in den. Om en sådan container inte kunde stoppas säger körningen att den saknar nätverk tills den startas om. Detsamma gäller BombVaults egen container, som aldrig stoppar sig själv och inte kan läggas på listan.
- En container som inte körs igen efteråt nämns i körningen och i en avisering, även när själva säkerhetskopieringen misslyckades.

## När appen är upptagen {#app-busy}

En säkerhetskopiering stoppar sin container, så en schemalagd kan vänta medan någon använder appen och försöka igen på egen hand. Kortet **När appen är upptagen** på en containers flik Inställningar ställer in det:

- **Aktivitetskontroll** frågar Plex, Jellyfin eller Emby före en schemalagd säkerhetskopiering om någon tittar. Plex frågas med token från sin egen `Preferences.xml`, så det finns inget att skriva in. Jellyfin och Emby tar en **API-nyckel**, som görs i serverns instrumentpanel under API keys.
- **Ignorera pausade sessioner efter (min)** låter en film som pausats för natten sluta räknas.
- **Säkerhetskopiera när servern inte svarar** avgör vad som händer när servern inte går att fråga: på, säkerhetskopian körs och loggen säger det. Av, den väntar som om någon tittade.
- **Vänta medan containern skickar data** är till för appar utan en sådan kontroll, till exempel en spelserver eller en nedladdare. Säkerhetskopian väntar medan containern skickade mer än **Upptagen från (KiB/s)** vid någon avläsning de senaste **Tyst i (min)** minuterna.
- **Försök igen med intervall (min)**, **Försök bara inom ett tidsfönster** med **Försök mellan**, och **Vänta högst (timmar)** bestämmer hur försöken går.
- **När väntan är slut** ger antingen upp, vilket visar säkerhetskopian under **Behöver uppmärksamhet** med **Försök igen**, eller **Säkerhetskopiera ändå**.

En väntande säkerhetskopiering loggas som **Uppskjuten**, i bärnsten, och resten av körningen fortsätter. Översikten, listan och posten visar den med nästa försök, och aviseringen om försening och skyddsstatusen väntar på den till sin tidsgräns. Du får en avisering när den skjuts upp och en när den tar slut, inte en per försök. Ett försök väntar medan en annan säkerhetskopiering körs, och en väntan fortsätter efter en omstart. En säkerhetskopiering du startar själv körs alltid direkt, vare sig med **Säkerhetskopiera nu**, **Försök igen**, ett urval, API:et eller MCP, och en som går igenom avslutar en väntan som var pågående.

Containerns flik Inställningar har också ett äldre, enklare sätt: **Vänta tills appen är i vila** med **Vänta högst (timmar)**. Säkerhetskopian startar så fort appen är i vila, vilket kortet **Vila före säkerhetskopia** under **Inställningar, Schema** definierar. Medlemmarna i en Compose-stack som ska köras i samma körning väntar tillsammans, och den väntande säkerhetskopian håller inget lås, så de andra containrarna går före.

![Kortet När appen är upptagen för en Plex-container med aktivitetskontrollen inställd.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![Kortet När appen är upptagen för en Plex-container med aktivitetskontrollen inställd.](assets/screenshots/backups/app-busy-light.png#only-light)

### Exempel: Plex väntar medan någon tittar {#example-plex}

Den totala säkerhetskopian körs varje natt klockan 03:00. Ingen ska behöva se en film stanna för att Plex gick ner för sin säkerhetskopia, men säkerhetskopian ska ändå ske före morgonen.

1. Öppna **Säkerhetskopior**, sedan posten `plex` och dess flik **Inställningar**.
2. Ställ **Aktivitetskontroll** på Plex under **När appen är upptagen**. Det finns inget att skriva.
3. Sätt **Ignorera pausade sessioner efter (min)** till `30`, så att en film som pausats vid midnatt slutar räknas halv ett.
4. Låt **Säkerhetskopiera när servern inte svarar** vara på.
5. Sätt **Försök igen med intervall (min)** till `15`.
6. Slå på **Försök bara inom ett tidsfönster** och sätt **Försök mellan** till `22:00` och `06:00`.
7. Sätt **Vänta högst (timmar)** till `3` och **När väntan är slut** till **Säkerhetskopiera ändå**.

Vad som händer en natt när någon fortfarande tittar klockan 03:00: BombVault frågar Plex, hittar en session som spelar och loggar säkerhetskopian som **Uppskjuten** med nästa försök 03:15. Körningens andra containrar säkerhetskopieras som vanligt. När filmen slutar 04:10 hittar försöket 04:15 Plex i vila och säkerhetskopierar det, och väntan slutar med en avisering. Om filmen fortfarande skulle gå klockan 06:00 skulle väntan nå sitt slut och Plex säkerhetskopieras ändå, så dess säkerhetskopior hamnar aldrig mer än tre timmar efter.

För din egen kontroll låter du **Kommando före säkerhetskopiering** avsluta med kod 75 medan appen är upptagen, till exempel `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Fler symptom och lösningar finns i [Felsökning](troubleshooting.md#deferred-backups).

## Vanliga exporter {#exports}

restic förblir motorn, men en export ger dig en kopia som varken behöver BombVault eller restic.

- **Exportera container** och **Exportera VM** under **Mer** på en posts sida skriver en kopia som den är just nu, bredvid repositoriet: containerns säkerhetskopierade mappar plus dess Unraid-mall, eller VM:ens diskavbilder plus dess definition, möjlig att återställa med `virsh define`.
- **Exportera** på en ögonblicksbild skriver en lagrad säkerhetskopia till exportmappen på servern, som tar, zip eller ett eget restic-repositorium, och kan ta en ögonblicksbild eller alla. En flash-säkerhetskopia och en auto-säkerhetskopia innehåller nycklar, så export av en sådan kräver ett inloggningslösenord och en inloggning på instansen.

Exporter är vanliga filer som standard. **Kryptera exporter med age** under **Inställningar, System** förseglar dem, flash-ZIP-filen inkluderad, för de mottagare du listar, var och en en publik age-nyckel eller en publik SSH-nyckel. Du dekrypterar dem senare utanför lådan med den matchande privata nyckeln. Med kryptering på och utan giltig mottagare misslyckas en export med ett tydligt fel i stället för att någonsin skriva vanliga filer.

## Vad en körning berättar {#run-notes}

- **Tidtagning.** Varje körning visar start, slut och varaktighet, och varje post har sina egna **Senaste körningar**.
- **Varför en säkerhetskopiering var långsam.** Medan en säkerhetskopiering körs håller BombVault koll på hur belastade CPU:n, diskarna och nätverket är. När en säkerhetskopiering tar mycket längre än vanligt och en sak tydligt var vid sin gräns säger körningen det, till exempel "Måldisken disk1 var 98 % upptagen". Annars säger den ingenting.
- **Förlopp för kontroller och rensningar.** Aktivitetsloggen och kortet **Kontrollera säkerhetskopior** visar hur långt restic har räknat, till exempel *12 av 47 paket*, med tiden kvar i det steget när det finns nog för att uppskatta den.
- **Filer den inte kunde läsa.** En säkerhetskopiering som gick igenom men lämnade något utanför gör sin rad orange och nämner vad som lämnades utanför.
