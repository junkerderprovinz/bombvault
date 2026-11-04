# ZFS-datauppsättningar

ZFS-datauppsättningar är poster i listan **Säkerhetskopior**, som containrar och VM:ar. En post är en datauppsättning tillsammans med varje datauppsättning under den. För varje säkerhetskopia tar BombVault en enda ZFS-ögonblicksbild av hela trädet, så att varje datauppsättning i det fångas i samma ögonblick. Den läser sedan varje datauppsättnings filer från ögonblicksbilden och lagrar dem med restic, på samma sätt som den lagrar en mapp, och tar bort ögonblicksbilden direkt efteråt. Säkerhetskopiorna är deduplicerade, du kan bläddra i var och en av dem, och enskilda filer går att återställa.

Säkerhetskopian skickar aldrig en ström in i restic och rullar aldrig tillbaka en datauppsättning. BombVault förstör bara ögonblicksbilder som den skapat själv. Den valfria [repliken](#replica) är det enda stället som använder `zfs send`: den kopierar datauppsättningarna till en andra ZFS-server och rör inte säkerhetskopian.

## Krav {#requirements}

- **SSH-anslutningen till den här servern.** ZFS-datauppsättningar använder samma nyckel, värd och användare som VM-säkerhetskopior. Om VM-säkerhetskopior redan fungerar gör det här det också. Följ annars [guiden för VM-säkerhetskopiering över SSH](vm-backup-ssh-setup.md). Mallfälten heter **Host SSH: Address**, **Host SSH: Port** och **Host SSH: User**.
- **Kommandot `zfs` på den värden.** Unraid 6.12 och nyare samt TrueNAS SCALE har det.
- **Host Data mappad som `/mnt` med Access Mode Read/Write - Slave.** Det är mallens standard. En datauppsättnings ögonblicksbild dyker upp i datauppsättningens mapp `.zfs/snapshot` först efter att BombVault har startat, så containern måste ta emot monteringar som värden gör senare.
- **Datauppsättningarna monterade under `/mnt`.** På Unraid ligger pooler på `/mnt/<pool>`, så det är redan fallet.

Slå på avsnittet under **Inställningar, Allmänt** (ZFS-datauppsättningar). **ZFS-anslutning till den här servern** under **Mer** på sidan Säkerhetskopior öppnar sedan **Anslutning till den här servern**. Den testar SSH-anslutningen, nämner användaren och värden den ansluter till och säger vad som saknas när något gör det. När anslutningen är nere säger listan Säkerhetskopior det med **Visa anslutningen**, och kortet **Värdintegration** på översikten visar samma resultat.

## Poster och underordnade datauppsättningar {#items-and-children}

Välj **Lägg till** på sidan Säkerhetskopior och sedan **En ZFS-datauppsättning**. Listan kommer från servern. Välj datauppsättningen överst i det du vill säkerhetskopiera, till exempel `cache/appdata`, så täcker posten den och varje datauppsättning under den.

- **Nya underordnade datauppsättningar går med av sig själva.** En datauppsättning som skapas under posten senare säkerhetskopieras vid nästa körning, och den körningen nämner den som ny. Dess första säkerhetskopia läser den helt en gång. Därefter läses bara ändringar.
- **Du kan lämna enskilda barn utanför.** Slå av ett barn på postens flik Inställningar så lämnas det utanför tillsammans med allt under det. Ett barn som lämnats utanför och som inte längre finns på servern markeras som det och kan tas bort från listan.
- **Barn som inte går att läsa hoppas över, aldrig tyst.** Körningen listar dem, posten visar hur många som hoppades över, och översiktens kort **Inte säkerhetskopierat** räknar var och en som oskyddad. Körningen säkerhetskopierar fortfarande allt annat och misslyckas inte på grund av ett överhoppat barn. Orsakerna finns i [tabellen över orsakskoder](#reason-codes): en datauppsättning som inte är monterad, har `canmount=off`, en `legacy`-monteringspunkt eller ingen, en krypteringsnyckel som inte är laddad, ögonblicksbildsåtkomst avstängd eller en monteringspunkt som BombVault inte kan se.
- **En överhoppad datauppsättning tar inte sina barn med sig.** En `canmount=off`-datauppsättning som bara håller andra datauppsättningar hoppas över (visas som "structure only") och dess monterade barn säkerhetskopieras. En krypterad datauppsättning vars nyckel inte är laddad hoppas över tillsammans med de barn som delar dess nyckel.
- **Barn som är VM-diskar eller systemdata börjar avstängda** i tilläggsdialogen, med orsaken bredvid omkopplaren. Att lägga till en hel pool ber om en bekräftelse som listar vad den innehåller.

### Volymer {#volumes}

En volym (zvol) håller en virtuell disk i stället för filer, och en ZFS-post säkerhetskopierar aldrig en.

- En volym som en VM använder säkerhetskopieras med den VM:en.
- En volym som ingen VM använder (en iSCSI-extent, en disk du kopplat loss) **säkerhetskopieras inte av BombVault**. Tilläggsfönstret och sidan Säkerhetskopior räknar de här volymerna och säger det. En senare version kommer att säkerhetskopiera dem.

Volymer inuti en posts träd hoppas över och nämns vid varje körning.

### Dockers lagring {#docker-storage}

Med Dockers ZFS-lagringsdrivrutin är varje image-lager en datauppsättning med en `legacy`-monteringspunkt. Tilläggsdialogen viker ihop dessa till en rad per förälder. Ett träd som innehåller fler än 20 sådana datauppsättningar kan inte bli en post: medan en ögonblicksbild av det finns kan Docker inte ta bort image-lager. Lägg till datauppsättningarna under det i stället, till exempel `appdata`.

### Poster överlappar aldrig {#overlap}

En datauppsättning kan tillhöra bara en post. BombVault nekar en ny post som ligger inuti en befintlig, eller som skulle innehålla en. För att slå ihop flera barnposter till en föräldrapost raderar du först barnposterna och väljer att behålla deras säkerhetskopior, och lägger sedan till föräldern. Varje datauppsättning behåller sin historik under sitt eget namn, så nästa säkerhetskopia fortsätter där de gamla posterna slutade och läser inte allt igen.

## Stoppa containrar och kommandon kring ögonblicksbilden {#consistency}

En ögonblicksbild av en körande databas är som ett plötsligt strömavbrott: databasen återhämtar sig oftast, men den måste. Varje post kan göra två saker åt det, och båda täcker bara själva ögonblicket för ögonblicksbilden, inte hela säkerhetskopian.

- **Stoppa de här containrarna för ögonblicksbilden.** BombVault stoppar de listade containrarna, tar ögonblicksbilden och startar dem igen direkt. Containrar på en beroendenivå stoppas parallellt, de beroende först, så hela fönstret är oftast några sekunder. Körningen visar hur länge det var. Säkerhetskopian läser sedan den frusna ögonblicksbilden medan apparna redan körs igen. Bara containrar som körde stoppas.
- **Ett kommando före och efter ögonblicksbilden.** Det körs inuti en container du väljer, till exempel för att dumpa en databas in i datauppsättningen precis före ögonblicksbilden, utan att stoppa något. Om kommandot före ögonblicksbilden misslyckas misslyckas säkerhetskopian och ingen ögonblicksbild tas. Ett kommando efter ögonblicksbilden som misslyckas visas på körningen men får inte säkerhetskopian att misslyckas.

Vad som händer när något går fel:

- Om en container inte går att stoppa startar BombVault de som den redan stoppat och säkerhetskopian misslyckas och nämner containern. Den faller aldrig tillbaka på en ögonblicksbild av körande appar.
- Stoppet väntar på att en körande containersäkerhetskopia blir klar (upp till 30 minuter för en manuell körning, upp till säkerhetskopieringens tidsgräns för en schemalagd), så att de två aldrig stoppar och startar samma container samtidigt.
- Innan den första containern stoppas skriver BombVault ner vilka den stoppar. Om BombVault avbryts inom fönstret startar den de containrarna igen nästa gång den startar, skickar en avisering, och posten visar en röd anmärkning för varje container den inte kunde starta.

De automatiska [databasdumparna](backups.md#database-dumps) körs med en containers egen säkerhetskopia, inte med en ZFS-post. En databas vars container säkerhetskopieras bara via sin datauppsättning får ingen dump, så ge den ett kommando här.

En container kan stå på den här listan och samtidigt säkerhetskopieras som en container. Dess data lagras då två gånger, i två repositorier, och den totala säkerhetskopian stoppar den två gånger. Posten säger det.

## Återställning {#restore}

Öppna posten och välj **Återställ**, eller välj den på sidan **Återställning**. Återställningsfönstret frågar efter säkerhetskopian och sedan **Datauppsättning**. Som standard är det postens översta datauppsättning. Under **Vad** återställer du allt eller bara de filer och mappar du väljer. **Vart** erbjuder:

- **Återställ in i datauppsättningen.** Filer från säkerhetskopian skrivs in i datauppsättningens monteringspunkt. Filer med samma namn skrivs över, andra filer blir kvar. Datauppsättningen rullas aldrig tillbaka eller ersätts. BombVault kontrollerar att datauppsättningen är monterad, synlig och skrivbar, en gång innan den börjar och igen precis innan den skriver. Där en underordnad datauppsättning är monterad inuti den skrivs ingenting: barnet behåller sina filer, sin ägare och sina behörigheter och återställs från sin egen säkerhetskopia.
- **Till en mapp.** Välj en mapp under `/mnt`. BombVault kontrollerar att mappen ligger på en monterad pool eller delad resurs och att det finns tillräckligt med ledigt utrymme. Det fungerar utan SSH-anslutningen och för datauppsättningar som inte längre finns.
- **Till en ny datauppsättning.** Namnge en datauppsättning som inte finns än. Fönstret föreslår `<dataset>-restored`. BombVault skapar den med de ZFS-egenskaper som är sparade i säkerhetskopian och återställer in i den, se [Återställa som en ny datauppsättning](#new-dataset).

Att välja **Alla datauppsättningar i den här säkerhetskopian** som datauppsättning skriver varje datauppsättning i trädet till sin egen undermapp i mappen du väljer. Datauppsättningar som hoppades över i den säkerhetskopian nämns. För att återställa från en annan BombVaults repositorium använder du **Från en gammal säkerhetskopiemapp** på sidan Återställning. Den återställer alltid till en mapp: alla datauppsättningar i en säkerhetskopia, var och en till sin egen undermapp, eller en datauppsättning i trädet, hel eller valda filer.

Postens lista över containrar att stoppa erbjuds också för en återställning in i datauppsättningen. De containrarna förblir stoppade under hela återställningen, och containersäkerhetskopior väntar under tiden.

### Säkerhetsögonblicksbilden {#safety-snapshot}

Innan BombVault skriver in i en datauppsättning tar den en ZFS-ögonblicksbild av just den datauppsättningen, med namnet `bombvault-prerestore-<time>`. **Spara först en ZFS-ögonblicksbild av läget nu** är på som standard. Att slå av det kräver en andra bekräftelse. Om ögonblicksbilden inte går att ta återställs ingenting.

BombVault raderar aldrig en säkerhetsögonblicksbild av sig själv. Posten listar dem med ålder och storlek, var och en med en åtgärd **Ta bort**, och varnar när den äldsta är mer än 30 dagar gammal, eftersom den håller kvar raderad och ändrad data på poolen.

För att gå tillbaka efter en återställning kopierar du enskilda filer från `.zfs/snapshot/bombvault-prerestore-<time>` inuti datauppsättningen. `zfs rollback <dataset>@bombvault-prerestore-<time>` fungerar bara medan den är datauppsättningens nyaste ögonblicksbild. `zfs rollback -r` raderar varje nyare ögonblicksbild, automatiska inkluderade.

### Återställa som en ny datauppsättning {#new-dataset}

BombVault lagrar de lokalt satta ZFS-egenskaperna för varje datauppsättning med varje säkerhetskopia: komprimering, postlängd, kvot, reservation, atime, xattr, acltype, skiftlägeskänslighet och dina egna användaregenskaper. Ärvda och skrivskyddade värden utelämnas, eftersom de kommer tillbaka av sig själva. Säkerhetskopior från före BombVault lagrade dem har inga.

- **Till en ny datauppsättning** kör `zfs create` med varje lagrad egenskap. Skiftlägeskänslighet, normalisering och utf8only går bara att sätta på det här sättet. Kvoter och reservationer sätts efter filerna, så de kan inte neka dem. Monteringspunkten utelämnas, så kopian inte krockar med originalet, och likaså `canmount`, `readonly` och krypteringsinställningarna, så att återställningen kan skriva. En ny datauppsättning under en krypterad tar över dess kryptering. Datauppsättningen ovanför den nya måste finnas. Om något misslyckas efter att den nya datauppsättningen skapats blir den kvar på servern, eftersom BombVault aldrig förstör en datauppsättning.
- **Återställ in i datauppsättningen** listar de lagrade egenskaperna bredvid återställningen. **Sätt även dessa egenskaper** sätter de som en befintlig datauppsättning fortfarande kan ta emot, innan någon fil skrivs. Kvoter och reservationer sätts efter filerna, så de kan inte neka dem. Utan den omkopplaren behåller datauppsättningen sina inställningar.

## Vad som finns i säkerhetskopian {#contents}

I säkerhetskopian: filerna och mapparna i varje säkerhetskopierad datauppsättning, med sitt ägarskap, sina behörigheter, tidsstämplar och utökade attribut som restic lagrar dem, och de lokalt satta ZFS-egenskaperna för varje datauppsättning.

Inte i säkerhetskopian:

- ägaren och läget för varje datauppsättnings översta mapp i sig (allt under den ingår). En återställning in i datauppsättningen lämnar den befintliga översta mappen som den är, en återställning till en mapp skapar den med läge `0755`,
- befintliga ZFS-ögonblicksbilder,
- barn som hoppades över eller lämnades utanför,
- volymer.

För att återställa till en ny pool skapar du poolen och återställer varje datauppsättning till en ny datauppsättning. Om NFSv4-ACL:er, som TrueNAS använder på SMB-datauppsättningar, kommer tillbaka som du förväntar dig har inte kontrollerats än, så kontrollera en återställning på dina egna data innan du förlitar dig på dem.

## Krypterade datauppsättningar {#encryption}

En krypterad datauppsättning säkerhetskopieras bara medan dess nyckel är laddad. Annars hoppas den över med en varning. Ladda nyckeln med `zfs load-key` och montera datauppsättningen. BombVault läser datan dekrypterad och lagrar den i restics repositorium, som är krypterat. Om du slagit av kryptering i BombVault är det repositoriet inte det.

## Replik {#replica}

En replik är en kopia av en posts datauppsättningar på en andra ZFS-server. BombVault håller den aktuell med `zfs send` och `zfs receive`. Den första körningen skickar allt, därefter reser bara de ändrade blocken. På den andra servern kan du montera kopian direkt.

En replik ersätter aldrig säkerhetskopian. Äldre versioner, enskilda filer och kontrollen kommer fortfarande från säkerhetskopiorna, och repliken behåller bara så många ögonblicksbilder som du ställer in. En aktuell replik räknas som en kopia utanför lokalen, men inte som en säkerhetskopia.

Repliken väntar inte på säkerhetskopian. **Efter varje säkerhetskopia** betyder efter postens säkerhetskopiering, oavsett om den körningen lyckades, misslyckades eller inte hade något att göra. När ZFS-avsnittet inte har något lagringsmål påslaget och posten inte har något eget repositorium skickar en säkerhetskopiering av posten bara repliken, och kortet Replik säger det i bubblan bredvid **När**. Listan Säkerhetskopior, postens huvud och körhistoriken visar repliken: **Replikerar** medan den körs, och **Replikerad** med tiden när den är klar.

**Bara replik, ingen säkerhetskopia** på kortet Replik får hela ZFS-avsnittet att skicka repliker och inte ta några säkerhetskopior. Den egna mappen och de externa lagringsmålen behåller sina omkopplare, så att slå av det säkerhetskopierar till dem igen, och varje ZFS-posts kort visar avsnittets omkopplare som den står. En sådan post väntar på sin första replik i stället för en säkerhetskopia, **Säkerhetskopiera nu** erbjuds som **Replikera nu**, och postens containrar stoppas och dess kommandon körs kring ögonblicksbilden som för en säkerhetskopia. Dess repliker räknas för ett schema med några dagar emellan, aviseringen om försening och täckningen.

Slå på den i kortet **Replik** på postens flik Inställningar. Kortet listar varje datauppsättning och volym med sitt läge, och **Replikera nu** startar en körning. En replikkörning har ett eget lås, så en lång första överföring håller aldrig upp säkerhetskopiorna.

| Fält | Vad det ställer in |
|---|---|
| **Replikeras till** | Målet: en ZFS-server (markerad **via SSH**) eller en **parkopplad instans**. **Lägg till ZFS-server** sätter upp en ny. |
| **När** | **Efter varje säkerhetskopia**, vilket betyder efter postens säkerhetskopiering, eller **Egen plan**. |
| **Behåll på målet** | Hur många ögonblicksbilder som stannar på ZFS-servern: **Kort**, **Balanserad**, **Lång** eller **Egna värden**. En parkopplad instans bestämmer det själv. |
| **Behåll på den här servern** | Hur många av de nyaste replikögonblicksbilderna som också stannar på den här servern. 0 eller 1 behåller bara den senaste. |
| **Bara replik, ingen säkerhetskopia** | Hela ZFS-avsnittet skickar repliker och tar inga säkerhetskopior. |

![Kortet Replik för en ZFS-post.](assets/screenshots/zfs/replica-dark.png#only-dark)
![Kortet Replik för en ZFS-post.](assets/screenshots/zfs/replica-light.png#only-light)

**Replikera allt nu**, bredvid säkerhetskopieringsåtgärderna när brickan ZFS är vald på sidan Säkerhetskopior, skickar repliken för varje ZFS-post, en efter en. Servern kör gruppen, så den fortsätter när du lämnar sidan, och en post som inte kan replikera nämns och hoppas över.

### Skicka till en ZFS-server {#replica-push}

Vilken maskin som helst med ZFS och SSH kan ta emot, till exempel en andra Unraid eller en TrueNAS. BombVault behöver inte köra där.

1. Välj **Anslut via SSH** i **ZFS-server utan BombVault** under **Inställningar, Parkoppling**, eller **Lägg till ZFS-server** i postens kort **Replik**.
2. Ange adress, användare och port. Dialogen visar BombVaults publika nyckel. Lägg till den i användarens `~/.ssh/authorized_keys` på servern. På Unraid är det under **Users, root, SSH keys**. Beroende på **Display Settings** ligger Users i rubrikmenyn eller under Settings. På TrueNAS redigerar du användaren under **Credentials, Users**. Detta är replikanyckeln. Den skiljer sig från nyckeln för VM- och värd-SSH-länken, så lägg till den även om den andra redan är auktoriserad.
3. Testa anslutningen. Dialogen listar sedan serverns pooler. Välj en och sätt roten, som som standard är `<pool>/bombvault-replica`.
4. Välj den nya servern i postens kort **Replik**. Servern får då ett eget kort på sidan Instanser, och **Öppna** leder till dess sida.

Med root behövs inget mer. En egen användare behöver dessa behörigheter på målets pool, vilket dialogen också visar:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

På källan behöver samma sorts användare dessa på postens översta datauppsättning:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

I den här riktningen håller BombVault som äger posten också en nyckel som kan skriva på servern.

BombVault fäster värdnyckeln som servern visar vid första anslutningen och nekar en annan senare. Om servern installerades om, tryck på **Glöm värdnyckel** på serverns sida, så fäster nästa anslutning den nya nyckeln.

När den här serverns värd kan nå ZFS-servern själv pipar den `zfs send` rakt in i sin egen ssh till servern, med replikanyckeln och den fästa värdnyckeln, så strömmen inte passerar BombVault. Annars går strömmen genom BombVault.

### Skicka till en parkopplad instans {#replica-receive}

En parkopplad BombVault kan ta emot repliken själv. Ingen får SSH-åtkomst till den andra värden, och ingen nyckel hamnar i en `authorized_keys`-fil.

1. Välj den parkopplade instansen som mål i postens kort **Replik**, eller slå på rollen **ZFS-server** på den instansens kort på sidan Instanser och välj ZFS-posterna där. Kortet visar **Väntar på godkännande** tills den svarar.
2. Öppna **Instanser** på den mottagande instansen. Begäran visas på den skickande instansens kort som ett märke, och dess **Detaljer** listar den. Välj pool och rot på den servern och hur många ögonblicksbilder som stannar, och tryck sedan på **Tillåt** eller **Avvisa**.
3. Efter **Tillåt** skickar källan enligt sin egen plan, som för vilket annat mål som helst.

Den mottagande instansen accepterar bara det godkännandet täcker: postens datauppsättningar, in i sin egen rot. Den kör `zfs receive` själv, och källan har inget sätt att radera eller rulla tillbaka något där. Så kopian överlever en källa som någon har tagit över. Den mottagande instansen behåller sin egen gallring. Källan föreslår bara en regel när den frågar.

**Återkalla åtkomst** på den mottagande instansen avslutar godkännandet när som helst och talar om det för källan, som då visar att godkännandet drogs tillbaka och stannar. Det som den mottagande instansen redan håller blir kvar där. En avvisad begäran förblir avvisad. Fler datauppsättningar, eller en begäran efter en återkallning, frågar igen.

Den mottagande instansen måste svara över HTTPS. Källan skickar ingenting till en vanlig http-adress, eftersom token och data skulle korsa nätverket okrypterade. Varje överföring lämnar också en tiondel av den mottagande poolen ledig och stannar innan den skulle äta in i den, så en källa kan inte fylla den mottagande serverns pool.

### Var datan hamnar {#replica-target}

Varje datauppsättning hamnar på `<root>/<server>/<pool>/<path>`. Servermappen är namnet på källinstansen, fastställt vid första överföringen, så att två servrar med samma poolnamn aldrig kommer i vägen för varandra. Till exempel hamnar `cache/appdata` från en server som heter `tower` på `backup/bombvault-replica/tower/cache/appdata`.

Efter att BombVault satts upp på nytt nekar målet mappen som den här BombVault fyllde förut. Kortet Replik erbjuder då **Ta över mappen**, och nästa replik fortsätter från kopiorna i den. Ta över den bara när ingen annan BombVault fortfarande replikerar in i den mappen.

Kopian på målet är skrivskyddad. Som standard är den inte monterad, så den täcker aldrig något på den servern. En ZFS-server ansluten över SSH har en omkopplare **Montera skrivskyddat på servern** på sin sida: med den på monterar BombVault kopiorna där skrivskyddat efter varje körning, så att du kan läsa och kopiera deras filer på målet. Krypterade kopior förblir låsta tills deras nyckel är laddad, och att slå av det avmonterar ingenting. En parkopplad instans som tar emot repliken håller den avmonterad. ZFS-egenskaper följer med, utom monteringspunkten och `sharenfs` och `sharesmb`. Reservationerna `reservation` och `refreservation` stannar också kvar, så kopian bara tar det utrymme dess data behöver.

### Vad som går in {#replica-contents}

Allt som posten säkerhetskopierar går in, och volymerna under den också, som säkerhetskopian hoppar över. En underordnad datauppsättning du slagit av i posten stannar utanför. Alla datauppsättningar i en körning kommer från en ögonblicksbild, som i säkerhetskopian.

### Hur länge ögonblicksbilder stannar {#replica-retention}

På målet behåller en ny replik 7 dagliga och 3 veckovisa ögonblicksbilder. Välj **Kort**, **Balanserad** eller **Lång** i stället, eller sätt **Egna värden**. Bara ögonblicksbilder som heter `bombvault-replica-<14 digits>` tas någonsin bort där, och aldrig den nyaste som båda sidor delar.

På källan behåller BombVault den senaste replikögonblicksbilden, plus ett bokmärke för varje tillstånd den skickat. Bokmärken kostar inget utrymme. Nästa överföring startar från dem.

För att behålla fler på källan också, sätt **Behåll på den här servern** på kortet Replik, till exempel till 7. Så många av de nyaste replikögonblicksbilderna stannar då på den här servern efter varje replik, vid sidan av vad målet behåller. 0 eller 1 behåller bara den senaste. Den senaste bär en hold, så att basen för nästa överföring inte förstörs av misstag. De äldre har ingen. Varje ögonblicksbild håller kvar de block som ändrades efter den, så varje en du behåller tar plats på poolen. Att slå av repliken eller byta till ett annat mål, eller radera posten, tar bort dem från den här servern också.

Filerna i en behållen ögonblicksbild kan läsas eller kopieras direkt, från mappen `.zfs/snapshot` inuti datauppsättningen eller från en klon, utan att röra datauppsättningen eller repliken:

```
ls /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261006014100
zfs clone cache/appdata@bombvault-replica-20261006014100 cache/appdata-20261006
```

En klon tar inget utrymme förrän något i den ändras. Förstör den med `zfs destroy cache/appdata-20261006` när du är klar, eftersom en ögonblicksbild som har en klon inte kan tas bort.

Att gå tillbaka till den senaste replikögonblicksbilden behöver bara `zfs rollback cache/appdata@<latest>` här, och ingenting på målet. Att rulla hela datauppsättningen tillbaka till en äldre förstör varje ögonblicksbild efter den, på den här servern och på målet. Den senaste är hållen, så lyft först dess hold och rulla sedan tillbaka datauppsättningen här:

```
zfs release -r bombvault-replica cache/appdata@bombvault-replica-20261008014200
zfs rollback -r cache/appdata@bombvault-replica-20261006014100
```

Kopian på målet måste också gå tillbaka, men inte alltid till samma ögonblicksbild, eftersom dess egna behållregler kan ha släppt den. Kör repliken en gång. Den stannar för den datauppsättningen eftersom målet håller nyare ögonblicksbilder, och kortet Replik och körhistoriken nämner ögonblicksbilden som målet ska rullas tillbaka till: den nyaste som båda sidor fortfarande delar. Rulla tillbaka kopian till den på ZFS-servern, eller på värden för den parkopplade instans som tar emot repliken, till exempel:

```
zfs rollback -r backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261005014000
```

`zfs rollback` verkar på en datauppsättning, så upprepa återställningarna för varje underordnad datauppsättning du vill ha tillbaka också. Därefter fortsätter **Replikera nu** från den delade ögonblicksbilden.

### Krypterade datauppsättningar i en replik {#replica-encryption}

En krypterad datauppsättning skickas rå. Den förblir krypterad på målet, och målet ser aldrig nyckeln. Förvara nyckeln säkert: du behöver den för att öppna kopian efter en återställning, och en replik utan den går inte att läsa.

### Använda repliken {#replica-use}

Öppna postens flik **Säkerhetskopior** och klicka på raden **ZFS-replik** bland platserna som håller dess ögonblicksbilder. Bladet listar ögonblicksbilderna på målet och visar kommandona med dina verkliga namn.

För att titta på ett gammalt tillstånd klonar du en ögonblicksbild på målet. En klon tar inget utrymme förrän något ändras, och repliken förblir orörd:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Om källan havererar gör du kopian till en vanlig, skrivbar datauppsättning på målet:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

BombVault slutar sedan replikera till den datauppsättningen, tills du startar en ny första körning.

För att få tillbaka ett tillstånd till källan trycker du på **Hämta tillbaka som ny datauppsättning** i bladet. BombVault skickar ögonblicksbilden in i en ny datauppsättning bredvid originalet, med namnet `<dataset>-bombvault-restore-` plus en tidsstämpel. Den skriver aldrig över originalet.

### Exempel: en replik till en andra Unraid-låda {#example-replica}

Servern `tower` säkerhetskopierar datauppsättningen `cache/appdata` som en ZFS-post. En andra Unraid-låda på `192.168.1.20`, utan BombVault, har en pool som heter `backup` och ska hålla en kopia som du kan läsa när som helst.

1. Öppna **Säkerhetskopior**, sedan `cache/appdata` och dess flik **Inställningar**. Välj **Lägg till ZFS-server** under **Replikeras till** i kortet **Replik**.
2. Ange `192.168.1.20`, användaren `root` och port `22`. Dialogen visar replikanyckeln. Klistra in den på den andra lådan under **Users, root, SSH keys** och tryck på **Apply**. Den här nyckeln skiljer sig från Värd-SSH-nyckeln, så lägg till den även om den andra redan finns där.
3. Testa anslutningen. Välj poolen `backup` och behåll roten den föreslår, `backup/bombvault-replica`.
4. Tillbaka på kortet är den nya servern vald under **Replikeras till**. Låt **När** stå på **Efter varje säkerhetskopia**, låt **Behåll på målet** stå på sina 7 dagliga och 3 veckovisa ögonblicksbilder, och sätt **Behåll på den här servern** till `7`.
5. Tryck på **Replikera nu**. Den första körningen skickar allt, så 300 GB över en 1 Gbit/s-linje tar minst 40 minuter. Kortet och listan Säkerhetskopior säger **Replikerar**, sedan **Replikerad** med tiden. Senare körningar skickar bara de block som ändrats.

Datauppsättningarna ligger nu på `backup/bombvault-replica/tower/cache/appdata` på den andra lådan, skrivskyddade. På översikten räknar **Extern** för ZFS-avsnittet repliken som en kopia utanför lokalen.

För att få en fil från i tisdags har du tre sätt:

- **På den här servern**, eftersom **Behåll på den här servern** håller sju ögonblicksbilder:

    ```
    cp /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261103014100/plex/Preferences.xml /mnt/user/restore/
    ```

- **På den andra lådan**, med en klon av en ögonblicksbild på målet. Postens flik **Säkerhetskopior** visar kommandona med dina verkliga namn när du klickar på raden **ZFS-replik**. Ta bort klonen med `zfs destroy` när du är klar.
- **Med en montering**, genom att slå på **Montera skrivskyddat på servern** på ZFS-serverns sida. Efter nästa körning står kopiorna monterade och skrivskyddade på den andra lådan.

Versioner som är äldre än vad repliken behåller kommer fortfarande från säkerhetskopiorna, som nästa exempel visar.

### Exempel: en fil från en ZFS-säkerhetskopia {#example-zfs-file}

1. Öppna **Återställning**, välj brickan **ZFS** och klicka på `cache/appdata`.
2. Välj säkerhetskopian, och under **Datauppsättning** den datauppsättning som innehåller filen.
3. Välj **Enskilda filer** under **Vad** och bocka för filen.
4. Välj **Till en mapp** under **Vart** och välj en mapp under `/mnt`, till exempel `user/restore`. Det behöver ingen SSH-anslutning och fungerar också för en datauppsättning som inte längre finns.
5. Tryck på **Återställ**. Filen hamnar direkt i den mappen, utan sökvägen ovanför.

## Kvarglömda ögonblicksbilder {#leftover-snapshots}

En säkerhetskopias ögonblicksbild heter `<dataset>@bombvault-<14 digits>`, till exempel `cache/appdata@bombvault-20260924021500` (UTC). BombVault tar bort den direkt efter säkerhetskopian. Om det misslyckas, till exempel för att datauppsättningen är upptagen eller BombVault stoppades, tar BombVault bort den:

- före nästa säkerhetskopia av den posten,
- när BombVault startar, för varje post, även med avsnittet avstängt,
- när du raderar posten,
- när du trycker på **Ta bort nu** på posten, som visar hur många som är kvar.

Bara namn som exakt matchar `bombvault-` plus 14 siffror tas bort. Säkerhetsögonblicksbilder, dina egna ögonblicksbilder och automatiska ögonblicksbilder rörs aldrig. För att ta bort en för hand:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

En replikögonblicksbild heter `<dataset>@bombvault-replica-<14 digits>` och är inte kvarglömd. Den stannar på källan tills en nyare replikkörning ersätter den, eller så länge **Behåll på den här servern** behåller den, och på målet så länge gallringen behåller den. Rensaren rör den aldrig, eftersom den bara matchar `bombvault-` följt av exakt 14 siffror.

## Avvikelser {#anomalies}

Ett barn som tömts ändrar knappt summan för ett stort träd, så avvikelseupptäckten bevakar varje datauppsättning i en post för sig: dess storlek, antal filer, nya data och restic-tid har var sin historik. Den historiken hör till datauppsättningens namn, så den finns kvar när trädet senare säkerhetskopieras av en annan post.

En datauppsättning som den föregående körningen säkerhetskopierade och den här körningen inte kunde läsa räknas som tömd, så länge postens urval inte ändrades. Det täcker en nyckel som inte är laddad, en datauppsättning som inte är monterad och en som är borta ur trädet. Ett barn du själv utesluter ändrar urvalet, så dess historik börjar i stället om. Medan ett fynd om förlorad data är öppet behåller gallringen den enskilda datauppsättningens gamla säkerhetskopior och rensar resten av trädet som vanligt.

Varje datauppsättning har en egen rad i postens övervakning, som öppnas med **Övervakning** på sidan **Avvikelser** och står på postens flik **Verifiering**. De öppna fynden visas bredvid varje datauppsättning. Länken i ett fynd öppnar återställningsfönstret på datauppsättningens sista goda säkerhetskopia. Om en körning blir klar bedöms för hela posten, eftersom en körning lyckas eller misslyckas som helhet.

Själva kontrollerna beskrivs under [Avvikelser](anomalies.md). En assistent ansluten via [MCP-servern](mcp.md) kan lista en ZFS-posts återställningspunkter, starta dess säkerhetskopiering och läsa fynden, men att kvittera ett sker på sidan **Avvikelser**.

## Orsakskoder {#reason-codes}

Postens sida, körhistoriken och aviseringarna nämner ett problem med någon av de här koderna. De flesta har åtgärden bredvid sig på sidan också.

| Kod | Betydelse | Vad du ska göra |
|---|---|---|
| `ssh-missing` | SSH-anslutningen är inte uppsatt i den här containern. | Sätt upp SSH-anslutningen som för VM-säkerhetskopior. |
| `host-placeholder` | Host SSH: Address är fortfarande platshållaren, och `host.docker.internal` svarade inte heller. | Sätt Host SSH: Address till den här serverns LAN-IP. |
| `host-fallback` | Host SSH: Address är fortfarande platshållaren, och `host.docker.internal` fungerar. | Ingenting, eller sätt LAN-IP:n. |
| `ssh-unreachable` | Servern går inte att nå över SSH. | Kontrollera adress och port, och att SSH är påslaget. |
| `ssh-auth` | Servern nekade BombVaults nyckel. | Kör kommandot som visas under **Anslutning till den här servern** en gång på servern. |
| `zfs-not-found` | SSH-värden har inget `zfs`-kommando. | Peka Host SSH: Address på maskinen som äger poolerna. |
| `zfs-permission` | SSH-användaren får inte köra det här zfs-kommandot. | Använd root, eller se [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` nämner en annan värd eller användare än SSH-fälten. | Få dem att stämma, eller töm SSH-fälten så att båda kommer från URI:n. |
| `zfs-error` | zfs rapporterade ett annat fel. | Detaljerna visar dess meddelande. |
| `propagation-missing` | Nya monteringar på värden når inte containern. | Sätt Access Mode för Host Data till Read/Write - Slave och starta om BombVault. |
| `invalid-name` | Ett datauppsättningsnamn som BombVault inte accepterar. | Byt namn på datauppsättningen. |
| `name-too-long` | En datauppsättning i trädet är för lång för ett ögonblicksbildsnamn. | Byt namn på den, eller lägg till en datauppsättning under den som post. |
| `invalid-exclude` | Ett uteslutningsmönster eller ett barn som lämnats utanför passar inte posten. | Åtgärda posten som meddelandet nämner. För att lämna ett helt barn utanför slår du av det i stället för att skriva ett mönster. |
| `not-found` | Datauppsättningen finns inte på servern. | Ta bort posten, eller skapa datauppsättningen igen. Dess säkerhetskopior förblir möjliga att återställa. |
| `not-filesystem` | Det här är en volym, inte ett filsystem. | Se [Volymer](#volumes). |
| `overlaps-item` | Datauppsättningen överlappar en befintlig post. | Se [Poster överlappar aldrig](#overlap). |
| `docker-storage` | Trädet innehåller Dockers image-lagring. | Se [Dockers lagring](#docker-storage). |
| `nothing-readable` | Ingen datauppsättning i posten går att läsa just nu. | Titta på koderna för de överhoppade datauppsättningarna. |
| `snapshot-failed` | Ögonblicksbilden gick inte att skapa. | Detaljerna visar zfs meddelande. |
| `containers-busy` | En containersäkerhetskopia körde fortfarande när containrarna skulle stoppas. | Starta igen senare. Schemalagda körningar väntar av sig själva. |
| `consistency-stop-failed` | En container gick inte att stoppa, så ingen ögonblicksbild togs. | Kontrollera containern, eller ta bort den från listan. |
| `pre-snapshot-failed` | Kommandot före ögonblicksbilden misslyckades. | Körningens detaljer visar dess utdata. |
| `container-unknown` | En listad container finns inte. | Ta bort den från listan. |
| `container-is-self` | BombVault kan inte stoppa sin egen container. | Ta bort den från listan. |
| `leftover-snapshots` | Ögonblicksbilder som BombVault inte kunde ta bort finns kvar på servern. | Tryck på **Ta bort nu**, se [Kvarglömda ögonblicksbilder](#leftover-snapshots). |
| `zvol` | En volym i trädet, överhoppad. | Se [Volymer](#volumes). |
| `canmount-off` | Aldrig monterad (`canmount=off`), överhoppad. | Om den håller data, montera den eller flytta datan till en underordnad datauppsättning. |
| `legacy-mount` | Legacy-monteringspunkt, överhoppad. | Ge den en monteringspunkt under `/mnt`. |
| `no-mountpoint` | Ingen monteringspunkt, överhoppad. | Ge den en monteringspunkt under `/mnt`. |
| `not-mounted` | Inte monterad på servern, överhoppad. | `zfs mount` den, eller sätt `canmount=on`. |
| `key-not-loaded` | Krypterad och nyckeln är inte laddad, överhoppad. | `zfs load-key`, montera den sedan. |
| `snapdir-disabled` | Ögonblicksbildsåtkomst är avstängd, överhoppad. | `zfs set snapdir=hidden <dataset>`. Mappen `.zfs` förblir dold. |
| `not-visible` | BombVault kan inte se datauppsättningens monteringspunkt. | Flytta monteringspunkten under Host Data-sökvägen, eller mappa in den i containern på samma sökväg med Read/Write - Slave. |
| `shfs-only` | Datauppsättningen syns bara via `/mnt/user`, som döljer ögonblicksbilder. | Mappa `/mnt`, inte `/mnt/user`, som Host Data. |
| `snapshot-not-visible` | Ögonblicksbilden skapades men dök inte upp inuti BombVault. | Kör **Testa åtkomsten till ögonblicksbilder**. Se nedan. |
| `snapshot-loop` | Ögonblicksbilden nådde inte BombVault eftersom Host Data inte släpper igenom nya monteringar. | Sätt Access Mode för Host Data till Read/Write - Slave och starta om BombVault. |
| `backup-failed` | restic misslyckades för den här datauppsättningen. | Körningens detaljer visar varför. |
| `not-reached` | Körningen tog slut före den här datauppsättningen. | Kör säkerhetskopian igen. |
| `gone` | Datauppsättningen finns inte längre på servern. | Ingenting. Dess säkerhetskopior förblir möjliga att återställa. |
| `read-only-mount` | BombVault kan bara läsa datauppsättningen, så den kan inte återställa in i den. | Sätt mappningen till Read/Write - Slave, eller återställ till en mapp. |
| `destination-not-mounted` | Mappen ligger inte på en monterad pool eller delad resurs. | Välj en mapp på en pool eller delad resurs. |
| `not-enough-space` | Inte tillräckligt med ledigt utrymme på målet. | Frigör utrymme eller välj en annan mapp. |
| `safety-snapshot-failed` | Säkerhetsögonblicksbilden gick inte att ta, så ingenting återställdes. | Detaljerna visar zfs meddelande. |
| `safety-name-too-long` | Datauppsättningens namn är för långt för en säkerhetsögonblicksbild. | Slå av säkerhetsögonblicksbilden, eller återställ till en mapp. |
| `dataset-exists` | En datauppsättning med det här namnet finns redan. | Välj ett nytt namn, eller återställ in i själva datauppsättningen. |
| `create-failed` | Den nya datauppsättningen gick inte att skapa. | Detaljerna visar zfs meddelande. Kontrollera att datauppsättningen ovanför den finns. |
| `new-dataset-not-visible` | Den nya datauppsättningen skapades, men BombVault kan inte se den, så ingenting återställdes. | Datauppsättningen blir kvar på servern. Montera den under Host Data-sökvägen och återställ in i den. |
| `set-properties-failed` | De lagrade egenskaperna gick inte att sätta, så ingenting återställdes. | Detaljerna visar zfs meddelande. |
| `set-limits-failed` | Filerna återställdes, men den lagrade kvoten eller reservationen gick inte att sätta. | Detaljerna visar zfs meddelande. Sätt kvoten eller reservationen själv med `zfs set`. |

### Kontrollera vad containern ser {#mountinfo}

**Testa åtkomsten till ögonblicksbilder** på postens flik Inställningar tar en riktig ögonblicksbild av dess träd, letar efter den inuti BombVault för varje datauppsättning och tar bort den igen. Det är det snabbaste sättet att bevisa hela vägen före den första schemalagda körningen.

För att titta själv kör du det här på servern:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Varje rad är en montering inuti containern. En datauppsättnings rad visar dess sökväg inuti containern (under `/host/user`) och datauppsättningens namn. Ett fält `master:N` på den raden betyder att monteringen tar emot de monteringar värden gör senare, vilket är vad ögonblicksbildsåtkomst behöver. När det saknas, sätt Access Mode för Host Data till Read/Write - Slave och starta om BombVault.

## TrueNAS SCALE {#truenas}

- När `LIBVIRT_URI` är satt (som för VM-säkerhetskopior på TrueNAS) tar BombVault SSH-värd, användare och port för sina zfs-kommandon från URI:n, var och en som inte är satt för sig. Utan VM-säkerhetskopior behöver inget sättas: `host.docker.internal` går inte att slå upp i en TrueNAS-app, så BombVault når värden via appens standardgateway, som root på port 22. Sätt `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` och `LIBVIRT_SSH_PORT` när du vill ha en annan adress eller användare, i appens `environment`.
- Lägg till Värd-SSH-nyckeln från **Inställningar, Integrationer, Värd-SSH** hos root under **Credentials, Users**, och slå på SSH-tjänsten under **System, Services**. En [replik](#replica-push) till den här eller en annan TrueNAS behöver också replikanyckeln, som är en annan.
- En annan användare än root behöver behörighet på postens översta datauppsättning, som då täcker varje datauppsättning under den:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Att återställa till en ny datauppsättning behöver också `create` på datauppsättningen ovanför den, och att sätta lagrade egenskaper behöver behörighet för de egenskaperna.

  En SSH-session som inte är root på TrueNAS har inte `/usr/sbin` i sin sökväg. BombVault anropar då `/usr/sbin/zfs` direkt.
- Appens **Host Data** måste vara en värdsökväg ovanför datauppsättningarna, till exempel `/mnt/tank`, inte en ixVolume. Med en värdsökväg för appen vidare värdens nya monteringar till BombVault (`rslave`), vilket ögonblicksbildsåtkomst behöver. Se [TrueNAS SCALE](getting-started.md#truenas) för hela appuppsättningen.
