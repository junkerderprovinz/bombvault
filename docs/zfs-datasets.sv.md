# ZFS-datauppsättningar

ZFS-datauppsättningar är poster i listan **Säkerhetskopior**, som containrar och VM:ar. En post är en datauppsättning tillsammans med varje datauppsättning under den. För varje säkerhetskopia tar BombVault en enda ZFS-ögonblicksbild av hela trädet, så att varje datauppsättning i det fångas i samma ögonblick. Den läser sedan varje datauppsättnings filer från den ögonblicksbilden och lagrar dem med restic, på samma sätt som en mapp, och tar bort ögonblicksbilden direkt efteråt. Säkerhetskopiorna är deduplicerade, du kan bläddra i varje, och enskilda filer går att återställa.

Säkerhetskopian skickar aldrig en ström in i restic och rullar aldrig tillbaka en datauppsättning. BombVault förstör bara ögonblicksbilder som det själv har skapat. Den valfria [repliken](#replica) är det enda som använder `zfs send`: den kopierar datauppsättningarna till en andra ZFS-server och rör inte säkerhetskopian.

## Krav {#requirements}

- **SSH-anslutningen till den här servern.** ZFS-datauppsättningar använder samma nyckel, värd och användare som VM-säkerhetskopior. Fungerar VM-säkerhetskopior redan, fungerar det här också. Annars följer du [guiden för VM-säkerhetskopiering över SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) på GitHub. Mallfälten heter **Host SSH: Address**, **Host SSH: Port** och **Host SSH: User**.
- **Kommandot `zfs` på den värden.** Unraid 6.12 och senare samt TrueNAS SCALE har det.
- **Host Data mappad som `/mnt` med Access Mode Read/Write - Slave.** Det är mallens standard. Ögonblicksbilden av en datauppsättning dyker upp i datauppsättningens mapp `.zfs/snapshot` först efter att BombVault har startat, så containern måste ta emot monteringar som värden gör senare.
- **Datauppsättningarna monterade under `/mnt`.** På Unraid ligger pooler under `/mnt/<pool>`, så det är redan fallet.

Slå på avsnittet under **Inställningar, Allmänt** (ZFS-datauppsättningar). **ZFS-anslutning till den här servern** under **Mer** på sidan Säkerhetskopior öppnar sedan **Anslutning till den här servern**. Den testar SSH-länken, namnger användaren och värden den ansluter till och säger vad som saknas när något gör det. När länken är nere säger listan Säkerhetskopior det med **Visa anslutningen**, och kortet **Värdintegration** på översikten visar samma resultat.

## Poster och underordnade datauppsättningar {#items-and-children}

Välj **Lägg till** på sidan Säkerhetskopior och sedan **En ZFS-datauppsättning**. Listan kommer från servern. Välj datauppsättningen överst i det du vill säkerhetskopiera, till exempel `cache/appdata`, så omfattar posten den och varje datauppsättning under den.

- **Nya underordnade datauppsättningar ansluter sig själva.** En datauppsättning som skapas under posten senare säkerhetskopieras vid nästa körning, och den körningen namnger den som ny. Dess första säkerhetskopia läser den helt en gång; därefter läses bara ändringar.
- **Du kan lämna enskilda underordnade utanför.** Slå av en underordnad på postens flik Inställningar så lämnas den utanför tillsammans med allt under den. En utelämnad underordnad som inte längre finns på servern markeras som sådan och kan tas bort från listan.
- **Underordnade som inte går att läsa hoppas över, aldrig tyst.** Körningen listar dem, posten visar hur många som hoppades över, och översiktens kort **Inte säkerhetskopierat** räknar var och en som oskyddad. Körningen säkerhetskopierar allt annat och misslyckas inte på grund av en överhoppad underordnad. Orsakerna finns i [tabellen över orsakskoder](#reason-codes): en datauppsättning som inte är monterad, har `canmount=off`, en `legacy`-monteringspunkt eller ingen, en krypteringsnyckel som inte är inläst, avstängd ögonblicksbildsåtkomst eller en monteringspunkt BombVault inte ser.
- **En överhoppad datauppsättning tar inte med sig sina underliggande.** En datauppsättning med `canmount=off` som bara innehåller andra datauppsättningar hoppas över (visas som "bara struktur"), och dess monterade underliggande datauppsättningar säkerhetskopieras. En krypterad datauppsättning vars nyckel inte är laddad hoppas över tillsammans med de underliggande som delar dess nyckel.
- **Underliggande datauppsättningar som är VM-diskar eller systemdata börjar avstängda** i dialogen för att lägga till, med orsaken bredvid omkopplaren. Lägger du till en hel pool får du bekräfta det i en dialog som listar vad den innehåller.

### Volymer {#volumes}

En volym (zvol) rymmer en virtuell disk i stället för filer, och en ZFS-post säkerhetskopierar aldrig en.

- En volym som en VM använder säkerhetskopieras med den VM:en.
- En volym som ingen VM använder (en iSCSI-extent, en disk du kopplat bort) **säkerhetskopieras inte av BombVault**. Fönstret för att lägga till och sidan Säkerhetskopior räknar de volymerna och säger det. En senare version kommer att säkerhetskopiera dem.

Volymer inuti en posts träd hoppas över och namnges vid varje körning.

### Dockers lagring {#docker-storage}

Med Dockers ZFS-lagringsdrivrutin är varje image-lager en datauppsättning med en `legacy`-monteringspunkt. Dialogen för att lägga till viker ihop dessa till en rad per förälder. Ett träd som rymmer fler än 20 sådana datauppsättningar kan inte bli en post: medan en ögonblicksbild av det finns kan Docker inte ta bort image-lager. Lägg i stället till datauppsättningarna under det, till exempel `appdata`.

### Poster överlappar aldrig {#overlap}

En datauppsättning kan tillhöra bara en post. BombVault avvisar en ny post som ligger inuti en befintlig eller som skulle innehålla en. För att slå ihop flera underordnade poster till en överordnad post, radera först de underordnade posterna och välj att behålla deras säkerhetskopior, och lägg sedan till den överordnade. Varje datauppsättning behåller sin historik under sitt eget namn, så nästa säkerhetskopia fortsätter där de gamla posterna slutade och läser inte allt igen.

## Stoppa containrar och köra kommandon runt ögonblicksbilden {#consistency}

En ögonblicksbild av en körande databas är som ett plötsligt strömavbrott: databasen återhämtar sig oftast, men den måste göra det. Varje post kan göra två saker åt det, och båda täcker bara ögonblicksbildens ögonblick, inte hela säkerhetskopian.

- **Stoppa de här containrarna för ögonblicksbilden.** BombVault stoppar containrarna i listan, tar ögonblicksbilden och startar dem igen direkt. Containrar på samma beroendenivå stoppas parallellt, de beroende först, så hela fönstret varar oftast några sekunder; körningen visar hur länge. Säkerhetskopian läser sedan den frysta ögonblicksbilden medan apparna redan kör igen. Bara containrar som körde stoppas.
- **Ett kommando före och efter ögonblicksbilden.** Det körs i en container du väljer, till exempel för att dumpa en databas till datauppsättningen precis före ögonblicksbilden utan att stoppa något. Misslyckas kommandot före ögonblicksbilden misslyckas säkerhetskopian och ingen ögonblicksbild tas. Ett kommando efter ögonblicksbilden som misslyckas visas på körningen men får inte säkerhetskopian att misslyckas.

Vad som händer när något går fel:

- Kan en container inte stoppas startar BombVault de som redan har stoppats, och säkerhetskopian misslyckas med containerns namn. Det faller aldrig tillbaka på en ögonblicksbild av appar som körs.
- Stoppet väntar tills en pågående containersäkerhetskopia är klar (upp till 30 minuter vid en manuell körning, upp till säkerhetskopians tidsgräns vid en schemalagd), så att de två aldrig stoppar och startar samma container samtidigt.
- Innan den första containern stoppas skriver BombVault ner vilka den stoppar. Om BombVault dödas inom fönstret startar den de containrarna igen nästa gång den startar, skickar en avisering, och posten visar en röd not för varje container den inte kunde starta.

De automatiska databasdumparna (se [Funktioner](features.md)) körs med en containers egen säkerhetskopia, inte med en ZFS-post. En databas vars container säkerhetskopieras bara via sin datauppsättning får ingen dump, så ge den ett kommando här.

En container kan stå i den här listan och samtidigt säkerhetskopieras som container. Dess data lagras då två gånger, i två arkiv, och den totala säkerhetskopian stoppar den två gånger. Posten säger det.

## Återställning {#restore}

Öppna posten och välj **Återställning**, eller välj den på sidan **Återställning**. Återställningsfönstret frågar efter säkerhetskopian och sedan **Datauppsättning**. Som standard är det postens översta datauppsättning. Under **Vad** återställer du allt eller bara filerna och mapparna du väljer. **Vart** erbjuder:

- **Återställ in i datauppsättningen.** Filer från säkerhetskopian skrivs till datauppsättningens monteringspunkt. Filer med samma namn skrivs över, andra ligger kvar. Datauppsättningen rullas aldrig tillbaka eller ersätts. BombVault kontrollerar att datauppsättningen är monterad, synlig och skrivbar, en gång innan det börjar och igen precis innan det skriver. Där en underliggande datauppsättning är monterad inuti skrivs ingenting: den behåller sina filer, sin ägare och sina behörigheter och återställs från sin egen säkerhetskopia.
- **Till en mapp.** Välj en mapp under `/mnt`. BombVault kontrollerar att mappen ligger på en monterad pool eller resurs och att det finns tillräckligt med ledigt utrymme. Det fungerar utan SSH-länken och för datauppsättningar som inte längre finns.
- **Till en ny datauppsättning.** Ange en datauppsättning som ännu inte finns; fönstret föreslår `<datauppsättning>-restored`. BombVault skapar den med ZFS-egenskaperna som lagrats i säkerhetskopian och återställer in i den, se [Återställning som en ny datauppsättning](#new-dataset).

Att välja **Alla datauppsättningar i den här säkerhetskopian** som datauppsättning skriver varje datauppsättning i trädet till en egen undermapp i mappen du väljer. Datauppsättningar som hoppades över i den säkerhetskopian namnges. För att återställa från en annan BombVaults arkiv, använd **Från en gammal säkerhetskopiemapp** på sidan Återställning; den återställer alltid till en mapp: alla datauppsättningar i en säkerhetskopia, var och en till sin undermapp, eller en datauppsättning i trädet, hel eller valda filer.

Postens lista över containrar att stoppa erbjuds även för en återställning in i datauppsättningen. De containrarna förblir stoppade under hela återställningen, och containersäkerhetskopior väntar under tiden.

### Säkerhetsögonblicksbilden {#safety-snapshot}

Innan den skriver in i en datauppsättning tar BombVault en ZFS-ögonblicksbild av just den datauppsättningen, med namnet `bombvault-prerestore-<tid>`. **Spara först en ZFS-ögonblicksbild av läget nu** är på som standard; att stänga av det kräver en andra bekräftelse. Om ögonblicksbilden inte kan tas återställs ingenting.

BombVault raderar aldrig en säkerhetsögonblicksbild på egen hand. Posten listar dem med ålder och storlek, var och en med åtgärden **Ta bort**, och varnar när den äldsta är mer än 30 dagar gammal, eftersom den håller kvar raderad och ändrad data på poolen.

För att gå tillbaka efter en återställning kopierar du enskilda filer från `.zfs/snapshot/bombvault-prerestore-<tid>` inuti datauppsättningen. `zfs rollback <dataset>@bombvault-prerestore-<tid>` fungerar bara så länge den är datauppsättningens senaste ögonblicksbild. `zfs rollback -r` tar bort alla nyare ögonblicksbilder, även automatiska.

### Återställa som ny datauppsättning {#new-dataset}

BombVault sparar med varje säkerhetskopia de lokalt satta ZFS-egenskaperna för varje datauppsättning: compression, recordsize, quota, reservation, atime, xattr, acltype, casesensitivity och dina egna användaregenskaper. Ärvda och skrivskyddade värden utelämnas, eftersom de kommer tillbaka av sig själva. Säkerhetskopior från innan BombVault sparade dem har inga.

- **Till en ny datauppsättning** kör `zfs create` med varje sparad egenskap. casesensitivity, normalization och utf8only kan bara sättas så. Kvoter och reservationer sätts först efter filerna, så att de inte kan neka dem. Monteringspunkten utelämnas så att kopian inte krockar med originalet, liksom `canmount`, `readonly` och krypteringen, så att återställningen kan skriva. En ny datauppsättning under en krypterad tar över dess kryptering. Datauppsättningen ovanför måste finnas. Om något misslyckas efter att den skapats ligger den nya datauppsättningen kvar på servern, eftersom BombVault aldrig förstör en datauppsättning.
- **Återställ in i datauppsättningen** visar de sparade egenskaperna bredvid återställningen. **Sätt även dessa egenskaper** sätter de som en befintlig datauppsättning fortfarande tar emot, innan någon fil skrivs. Kvoter och reservationer sätts först efter filerna, så att de inte kan neka dem. Utan den brytaren behåller datauppsättningen sina inställningar.

## Vad säkerhetskopian innehåller {#contents}

I säkerhetskopian: filerna och mapparna i varje säkerhetskopierad datauppsättning, med ägare, behörigheter, tidsstämplar och utökade attribut så som restic lagrar dem, samt de lokalt satta ZFS-egenskaperna för varje datauppsättning.

Inte i säkerhetskopian:

- ägare och behörigheter för själva den översta mappen i varje datauppsättning (allt under den ingår). En återställning in i datauppsättningen lämnar den befintliga översta mappen som den är, en återställning till en mapp skapar den med behörigheterna `0755`;
- befintliga ZFS-ögonblicksbilder;
- underliggande datauppsättningar som hoppades över eller utelämnades;
- volymer.

För att återställa till en ny pool skapar du poolen och återställer varje datauppsättning till en ny datauppsättning. Om NFSv4-ACL:er, så som TrueNAS använder dem på SMB-datauppsättningar, kommer tillbaka som du förväntar dig är ännu inte kontrollerat, så testa en återställning med dina egna data innan du litar på dem.

## Krypterade datauppsättningar {#encryption}

En krypterad datauppsättning säkerhetskopieras bara medan dess nyckel är laddad. Annars hoppas den över med en varning; ladda nyckeln med `zfs load-key` och montera datauppsättningen. BombVault läser data dekrypterat och lagrar det i restics repository, som är krypterat. Har du stängt av kryptering i BombVault är det repositoryt inte krypterat.

## Replik {#replica}

En replik är en kopia av en posts datauppsättningar på en andra ZFS-server. BombVault håller den aktuell med `zfs send` och `zfs receive`. Första körningen skickar allt, därefter färdas bara de ändrade blocken. På den andra servern kan du montera kopian direkt.

En replik ersätter aldrig säkerhetskopian. Äldre versioner, enskilda filer och kontrollen kommer fortfarande från säkerhetskopiorna, och repliken behåller bara så många ögonblicksbilder som du anger. En aktuell replik räknas som en kopia utanför lokalerna, men en post med en replik och utan säkerhetskopia förblir orange.

Slå på det i kortet **Replik** på postens flik Inställningar. Där väljer du vart repliken går, när den körs (**Efter varje säkerhetskopia** eller **Egen plan**) och hur många ögonblicksbilder som blir kvar på målet. Kortet listar varje datauppsättning och volym med dess tillstånd, och **Replikera nu** startar en körning. En replikkörning har ett eget lås, så en lång första överföring håller aldrig upp säkerhetskopiorna.

### Skicka till en ZFS-server {#replica-push}

Vilken maskin som helst med ZFS och SSH kan ta emot, till exempel en andra Unraid eller en TrueNAS. BombVault behöver inte köra där.

1. Under **Inställningar, Parkoppling**, välj **Anslut via SSH** i **ZFS-server utan BombVault**, eller **Lägg till ZFS-server** i postens kort **Replik**.
2. Ange adress, användare och port. Dialogen visar BombVaults publika nyckel. Lägg till den i användarens `~/.ssh/authorized_keys` på servern. På Unraid finns det under **Users, root, SSH keys**. Beroende på **Display Settings** ligger Users i huvudmenyn eller under Settings.
3. Testa anslutningen. Dialogen listar sedan serverns pooler. Välj en och ange roten, som som standard är `<pool>/bombvault-replica`.
4. Välj den nya servern i postens kort **Replik**. Servern har då ett eget kort på sidan Instanser, och **Öppna** leder till dess sida.

Med root behövs inget mer. En egen användare behöver de här behörigheterna på målets pool, som dialogen också visar:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

På källan behöver samma slags användare dessa på postens översta datauppsättning:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

I den här riktningen håller BombVault som äger posten också en nyckel som kan skriva på servern.

BombVault sparar värdnyckeln som servern visar vid första anslutningen och avvisar senare alla andra. Om servern har installerats om, tryck på **Glöm värdnyckel** på serverns sida, så sparar nästa anslutning den nya nyckeln.

### Skicka till en parkopplad instans {#replica-receive}

Ett parkopplat BombVault kan ta emot repliken själv. Ingen får SSH-åtkomst till den andra värden, och ingen nyckel hamnar i en `authorized_keys`-fil.

1. I postens kort **Replik**, välj den parkopplade instansen som mål, eller slå på rollen **ZFS-server** på den instansens kort på sidan Instanser och välj ZFS-posterna där. Kortet visar **Väntar på godkännande** tills den svarar.
2. Öppna **Instanser** på den mottagande instansen. Begäran visas på den sändande instansens kort som ett märke, och dess **Detaljer** listar den. Välj poolen och roten på den servern och hur många ögonblicksbilder som blir kvar, och tryck sedan på **Tillåt** eller **Avvisa**.
3. Efter **Tillåt** skickar källan enligt sitt eget schema, som till vilket annat mål som helst.

Den mottagande instansen accepterar bara det godkännandet täcker: postens datauppsättningar, in i sin egen rot. Den kör `zfs receive` själv, och källan har inget sätt att radera eller rulla tillbaka något där. Så kopian överlever en källa som någon tagit över. Den mottagande instansen behåller sin egen retention. Källan föreslår bara en regel när den ber.

**Revoke access** på den mottagande instansen avslutar godkännandet när som helst och meddelar källan, som då visar att godkännandet har återkallats och slutar. Det som den mottagande instansen redan har blir kvar där. En avvisad begäran förblir avvisad. Fler datauppsättningar, eller en begäran efter ett återkallande, frågar igen.

Den mottagande instansen måste svara över HTTPS. Till en vanlig http-adress skickar källan ingenting, eftersom token och data annars skulle gå okrypterade över nätet. Varje överföring lämnar dessutom en tiondel av den mottagande poolen fri och stannar innan den skulle ta av den, så en enda källa kan inte fylla den mottagande serverns pool.

### Var datan hamnar {#replica-target}

Varje datauppsättning hamnar i `<root>/<server>/<pool>/<path>`. Servermappen är källinstansens namn, som fastställs vid den första överföringen, så två servrar med samma poolnamn kommer aldrig i vägen för varandra. Till exempel hamnar `cache/appdata` på en server som heter `tower` i `backup/bombvault-replica/tower/cache/appdata`.

Kopian på målet är skrivskyddad och inte monterad, så den täcker aldrig något på den servern. ZFS-egenskaper följer med, utom monteringspunkten, `sharenfs` och `sharesmb`. Reservationerna `reservation` och `refreservation` stannar också kvar, så kopian tar bara den plats som dess data behöver.

### Vad som ingår {#replica-contents}

Allt posten säkerhetskopierar går med, och volymerna under den också, som säkerhetskopian hoppar över. En underordnad datauppsättning du slagit av i posten stannar utanför. Alla datauppsättningar i en körning kommer från en ögonblicksbild, som i säkerhetskopian.

### Hur länge ögonblicksbilderna blir kvar {#replica-retention}

På målet behåller en ny replik 7 dagliga och 3 veckovisa ögonblicksbilder. Välj i stället **Short**, **Balanced** eller **Long**, eller ange **Custom values**. Bara ögonblicksbilder med namnet `bombvault-replica-<14 digits>` tas någonsin bort där, och aldrig den nyaste som båda sidor delar.

På källan behåller BombVault bara den senaste replikögonblicksbilden, plus ett bokmärke för varje tillstånd som har skickats. Bokmärken tar ingen plats. Nästa överföring utgår från dem.

### Krypterade datauppsättningar i en replik {#replica-encryption}

En krypterad datauppsättning skickas rå. Den förblir krypterad på målet, och målet ser aldrig nyckeln. Förvara nyckeln säkert: du behöver den för att öppna kopian efter en återställning, och en replik utan den går inte att läsa.

### Använda repliken {#replica-use}

Öppna postens flik **Säkerhetskopior** och klicka på raden **ZFS-replik** bland platserna som håller dess ögonblicksbilder. Bladet listar ögonblicksbilderna på målet och visar kommandona med dina riktiga namn.

För att titta på ett gammalt tillstånd klonar du en ögonblicksbild på målet. En klon tar ingen plats förrän något ändras, och repliken förblir orörd:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Om källan går sönder gör du kopian på målet till en vanlig, skrivbar datauppsättning:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

Efteråt slutar BombVault att replikera till den datauppsättningen, tills du startar en ny första körning.

För att få tillbaka ett tillstånd till källan trycker du på **Bring back as a new dataset** i bladet. BombVault skickar ögonblicksbilden till en ny datauppsättning bredvid originalet, med namnet `<dataset>-bombvault-restore-` plus en tidsstämpel. Det skriver aldrig över originalet.

## Kvarlämnade ögonblicksbilder {#leftover-snapshots}

Ögonblicksbilden av en säkerhetskopia heter `<dataset>@bombvault-<14 siffror>`, till exempel `cache/appdata@bombvault-20260924021500` (UTC). BombVault tar bort den direkt efter säkerhetskopian. Om det misslyckas, till exempel för att datauppsättningen är upptagen eller BombVault stoppades, tar BombVault bort den:

- före nästa säkerhetskopia av den posten,
- när BombVault startar, för varje post, även med avsnittet avstängt,
- när du raderar posten,
- när du trycker på **Ta bort nu** på posten, som visar hur många som är kvar.

Bara namn som exakt är `bombvault-` plus 14 siffror tas bort. Säkerhetsögonblicksbilder, dina egna ögonblicksbilder och automatiska ögonblicksbilder rörs aldrig. För att ta bort en för hand:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

Ögonblicksbilden av en replik heter `<dataset>@bombvault-replica-<14 digits>` och är ingen kvarlämning. Den blir kvar på källan tills nästa replikkörning ersätter den, och på målet så länge lagringstiden behåller den. Städaren rör den aldrig, eftersom den bara matchar `bombvault-` följt av exakt 14 siffror.

## Avvikelser {#anomalies}

En tömd underordnad ändrar knappt totalen i ett stort träd, så avvikelsedetekteringen bevakar varje datauppsättning i en post för sig: storlek, filantal, ny data och restic-tid har var sin historik. Den historiken tillhör datauppsättningens namn, så den blir kvar när trädet senare säkerhetskopieras av en annan post.

En datauppsättning som föregående körning säkerhetskopierade och den här körningen inte kunde läsa räknas som tömd, så länge postens urval inte ändrats. Det omfattar en nyckel som inte är inläst, en datauppsättning som inte är monterad och en som försvunnit ur trädet. En underordnad du själv undantar ändrar urvalet, så dess historik börjar i stället om. Medan ett fynd om förlorad data är öppet behåller retention just den datauppsättningens gamla säkerhetskopior och gallrar resten av trädet som vanligt.

Varje datauppsättning har en egen rad i postens övervakning, som öppnas med **Övervakning** på sidan **Avvikelser** och står på postens flik **Verifiering**. De öppna fynden visas bredvid varje datauppsättning. Länken i ett fynd öppnar återställningsfönstret på datauppsättningens sista goda säkerhetskopia. Om en körning blir klar bedöms för hela posten, eftersom en körning lyckas eller misslyckas som helhet.

Själva kontrollerna beskrivs under [Funktioner](features.md). En assistent ansluten via [MCP-servern](mcp.md) kan lista en ZFS-posts återställningspunkter, starta dess säkerhetskopia och läsa fynden, men att kvittera ett fynd görs på sidan **Avvikelser**.

## Orsakskoder {#reason-codes}

Postens sida, körhistoriken och aviseringarna namnger ett problem med en av dessa koder. De flesta har också åtgärden bredvid sig på sidan.

| Kod | Betydelse | Vad du gör |
|---|---|---|
| `ssh-missing` | SSH-anslutningen är inte konfigurerad i den här containern. | Konfigurera SSH-anslutningen som för VM-säkerhetskopior. |
| `host-placeholder` | Host SSH: Address är fortfarande exempelvärdet, och `host.docker.internal` svarade inte heller. | Sätt Host SSH: Address till den här serverns LAN-IP. |
| `host-fallback` | Host SSH: Address är fortfarande exempelvärdet, och `host.docker.internal` fungerar. | Inget, eller sätt LAN-IP:n. |
| `ssh-unreachable` | Servern går inte att nå över SSH. | Kontrollera adress och port, och att SSH är påslaget. |
| `ssh-auth` | Servern avvisade BombVaults nyckel. | Kör kommandot som visas under **Anslutning till den här servern** en gång på servern. |
| `zfs-not-found` | SSH-värden har inget `zfs`-kommando. | Peka Host SSH: Address mot maskinen som äger poolerna. |
| `zfs-permission` | SSH-användaren får inte köra det här zfs-kommandot. | Använd root, eller se [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` anger en annan värd eller användare än SSH-fälten. | Få dem att stämma, eller töm SSH-fälten så att båda kommer från URI:n. |
| `zfs-error` | zfs rapporterade ett annat fel. | Detaljerna visar dess meddelande. |
| `propagation-missing` | Nya monteringar på värden når inte containern. | Sätt Access Mode för Host Data till Read/Write - Slave och starta om BombVault. |
| `invalid-name` | Ett namn på en datauppsättning som BombVault inte godtar. | Byt namn på datauppsättningen. |
| `name-too-long` | En datauppsättning i trädet är för lång för ett ögonblicksbildsnamn. | Döp om den, eller lägg till en datauppsättning under den som post. |
| `invalid-exclude` | Ett exkluderingsmönster eller en utelämnad underordnad passar inte posten. | Rätta posten som meddelandet namnger. För att lämna ut en hel underordnad datauppsättning, slå av den i stället för att skriva ett mönster. |
| `not-found` | Datauppsättningen finns inte på servern. | Ta bort posten, eller återskapa datauppsättningen. Dess säkerhetskopior går fortfarande att återställa. |
| `not-filesystem` | Det här är en volym, inte ett filsystem. | Se [Volymer](#volumes). |
| `overlaps-item` | Datauppsättningen överlappar en befintlig post. | Se [Poster överlappar aldrig](#overlap). |
| `docker-storage` | Trädet innehåller Dockers avbildningslagring. | Se [Dockers lagring](#docker-storage). |
| `nothing-readable` | Ingen datauppsättning i posten går att läsa just nu. | Titta på koderna för de överhoppade datauppsättningarna. |
| `snapshot-failed` | Ögonblicksbilden kunde inte skapas. | Detaljerna visar meddelandet från zfs. |
| `containers-busy` | En containersäkerhetskopia körde fortfarande när containrarna skulle stoppas. | Starta igen senare. Schemalagda körningar väntar av sig själva. |
| `consistency-stop-failed` | En container kunde inte stoppas, så ingen ögonblicksbild togs. | Kontrollera containern, eller ta bort den från listan. |
| `pre-snapshot-failed` | Kommandot före ögonblicksbilden misslyckades. | Körningsdetaljerna visar dess utdata. |
| `container-unknown` | En container i listan finns inte. | Ta bort den från listan. |
| `container-is-self` | BombVault kan inte stoppa sin egen container. | Ta bort den från listan. |
| `leftover-snapshots` | Ögonblicksbilder som BombVault inte kunde ta bort ligger kvar på servern. | Tryck på **Ta bort nu**, se [Kvarlämnade ögonblicksbilder](#leftover-snapshots). |
| `zvol` | En volym i trädet, överhoppad. | Se [Volymer](#volumes). |
| `canmount-off` | Aldrig monterad (`canmount=off`), överhoppad. | Innehåller den data, montera den eller flytta datan till en underliggande datauppsättning. |
| `legacy-mount` | Legacy-monteringspunkt, överhoppad. | Ge den en monteringspunkt under `/mnt`. |
| `no-mountpoint` | Ingen monteringspunkt, överhoppad. | Ge den en monteringspunkt under `/mnt`. |
| `not-mounted` | Inte monterad på servern, överhoppad. | Montera den med `zfs mount`, eller sätt `canmount=on`. |
| `key-not-loaded` | Krypterad och nyckeln är inte laddad, överhoppad. | `zfs load-key`, montera den sedan. |
| `snapdir-disabled` | Åtkomst till ögonblicksbilder är avstängd, överhoppad. | `zfs set snapdir=hidden <dataset>`. Mappen `.zfs` förblir dold. |
| `not-visible` | BombVault ser inte datauppsättningens monteringspunkt. | Flytta monteringspunkten under Host Data-sökvägen, eller mappa in den i containern på samma sökväg med Read/Write - Slave. |
| `shfs-only` | Datauppsättningen syns bara via `/mnt/user`, som döljer ögonblicksbilder. | Mappa `/mnt`, inte `/mnt/user`, som Host Data. |
| `snapshot-not-visible` | Ögonblicksbilden skapades men dök inte upp i BombVault. | Kör **Testa åtkomsten till ögonblicksbilder**; se nedan. |
| `snapshot-loop` | Ögonblicksbilden nådde inte BombVault eftersom Host Data inte släpper igenom nya monteringar. | Sätt Access Mode för Host Data till Read/Write - Slave och starta om BombVault. |
| `backup-failed` | restic misslyckades för den här datauppsättningen. | Körningsdetaljerna visar varför. |
| `not-reached` | Körningen slutade före den här datauppsättningen. | Kör säkerhetskopian igen. |
| `gone` | Datauppsättningen finns inte längre på servern. | Inget. Dess säkerhetskopior går fortfarande att återställa. |
| `read-only-mount` | BombVault kan bara läsa datauppsättningen och kan därför inte återställa in i den. | Sätt mappningen till Read/Write - Slave, eller återställ till en mapp. |
| `destination-not-mounted` | Mappen ligger inte på en monterad pool eller resurs. | Välj en mapp på en pool eller resurs. |
| `not-enough-space` | Inte tillräckligt med ledigt utrymme på målet. | Frigör utrymme eller välj en annan mapp. |
| `safety-snapshot-failed` | Säkerhetsögonblicksbilden kunde inte tas, så ingenting återställdes. | Detaljerna visar meddelandet från zfs. |
| `safety-name-too-long` | Namnet på datauppsättningen är för långt för en säkerhetsögonblicksbild. | Stäng av säkerhetsögonblicksbilden, eller återställ till en mapp. |
| `dataset-exists` | Det finns redan en datauppsättning med det här namnet. | Välj ett nytt namn, eller återställ till själva datauppsättningen. |
| `create-failed` | Den nya datauppsättningen kunde inte skapas. | Detaljerna visar meddelandet från zfs. Kontrollera att datauppsättningen ovanför finns. |
| `new-dataset-not-visible` | Den nya datauppsättningen skapades, men BombVault ser den inte, så ingenting återställdes. | Datauppsättningen ligger kvar på servern. Montera den under Host Data-sökvägen och återställ till den. |
| `set-properties-failed` | De sparade egenskaperna kunde inte sättas, så ingenting återställdes. | Detaljerna visar meddelandet från zfs. |
| `set-limits-failed` | Filerna återställdes, men den sparade kvoten eller reservationen kunde inte sättas. | Detaljerna visar meddelandet från zfs. Sätt kvoten eller reservationen själv med `zfs set`. |

### Kontrollera vad containern ser {#mountinfo}

**Testa åtkomsten till ögonblicksbilder** på postens flik Inställningar tar en riktig ögonblicksbild av dess träd, letar efter den inuti BombVault för varje datauppsättning och tar bort den igen. Det är det snabbaste sättet att bevisa hela vägen före den första schemalagda körningen.

För att titta själv kör du det här på servern:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Varje rad är en montering i containern. Raden för en datauppsättning visar dess sökväg i containern (under `/host/user`) och namnet på datauppsättningen. Ett fält `master:N` på den raden betyder att monteringen tar emot de monteringar som värden gör senare, och det är vad åtkomst till ögonblicksbilder behöver. Saknas det sätter du Access Mode för Host Data till Read/Write - Slave och startar om BombVault.

## TrueNAS SCALE {#truenas}

- När `LIBVIRT_URI` är satt (som för VM-säkerhetskopior på TrueNAS) tar BombVault SSH-värd, användare och port för sina zfs-kommandon från URI:n, var och en som inte är satt för sig. Utan VM-säkerhetskopior sätter du i stället `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` och `LIBVIRT_SSH_PORT`. Lägg till variablerna under **Additional Environment Variables**.
- En annan användare än root behöver behörighet på postens översta datauppsättning, som sedan täcker varje datauppsättning under den:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Att återställa till en ny datauppsättning kräver dessutom `create` på datauppsättningen ovanför, och att sätta sparade egenskaper kräver behörighet för de egenskaperna.

  En SSH-session utan root på TrueNAS har inte `/usr/sbin` i sin sökväg; BombVault anropar då `/usr/sbin/zfs` direkt.
- Appens **Host Data** måste vara en värdsökväg ovanför datauppsättningarna, till exempel `/mnt/tank`, inte en ixVolume. Med en värdsökväg skickar appen värdens nya monteringar vidare till BombVault (`rslave`), och det behöver åtkomst till ögonblicksbilder.
