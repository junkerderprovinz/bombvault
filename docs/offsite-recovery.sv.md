# Off-site och återställning

!!! note "Off-site-kopior väntar efter en ombyggnad"
    När Nödguiden eller **Identifiera säkerhetskopior** bygger upp poster utan de gamla inställningarna pausas off-site-kopior av de avsnitten tills deras standard är bekräftad. Se [Efter en ombyggnad](#after-a-rebuild).

Lokala säkerhetskopior skyddar mot en förlorad container eller en dålig uppdatering. Off-site-kopior och ett testat återställningskit skyddar mot hela boxen, ransomware eller en brand. Den här sidan täcker mål och off-site-kopior, hur du gör en kopia manipuleringssäker, hur du bevisar att du kan återställa, hur du parkopplar flera servrar och hur du får tillbaka allt när BombVault självt är borta.

## Off-site-kopior {#off-site-copies}

Behåll den snabba lokala säkerhetskopian och lägg till ett eller flera mål utanför huset. BombVault kopierar nya säkerhetskopior dit med `restic copy` på best-effort-basis, så ett off-site-hickande får aldrig den lokala säkerhetskopian att misslyckas. I den här formen är det lokala arkivet primärt och off-site-arkivet en kopia, men ett avsnitts primära arkiv behöver inte alls vara lokalt; se [Fjärranslutna primära arkiv](#remote-primary-repositories) nedan för att säkerhetskopiera rakt till S3, en rest-server och liknande i stället för att kopiera dit.

- **Flera mål per avsnitt.** Varje avsnitt (containrar, VM:ar, flash, auto-säkerhetskopia, mappar och ZFS-datauppsättningar) kan kopiera till flera off-site-mål samtidigt, så att du till exempel kan ha en rest-server hos en vän och en S3-bucket parallellt. Kortet **Mål per avsnitt** på sidan Mål slår på eller av varje linje från ett avsnitt till ett mål.
- **När kopiorna körs.** Ett måls kort **Kopiera** anger **När det kopieras**: **Efter varje säkerhetskopia**, eller enligt en egen plan som samlar säkerhetskopiorna och kopierar vid fasta tider. Under **Inställningar, Schema, Externa kopior per avsnitt** ger **Undantag** ett avsnitt en egen kopieringsplan. **Replikera nu** i ett avsnitts off-site-kopiakort under **Inställningar, Mål** startar en kopiering på begäran.
- **Retention** ställs in per mål under **Hur länge det bevaras**, så att du kan behålla off-site-kopior längre som arkiv. Kopian i ett avsnitts off-site-fält följer kortet **Extern lagring** under **Inställningar, Mål**; lämna det helt noll för att aldrig autotrimma off-site-säkerhetskopior.
- **Bandbreddsgränser** (**Off-site-bandbredd** under **Inställningar, Mål**, eller ett måls kort **Kopiera**) begränsar restics uppladdnings-/nedladdningshastighet så att kopior inte mättar ditt WAN.
- Medan en kopiering körs står det **Off-site-kopiering pågår** i översiktens statushuvud. Det är en aktivitetsindikator, inte en procentstapel, eftersom `restic copy` inte ger någon maskinläsbar förloppsinformation.

!!! note "Återställ från vilken plats som helst"
    Varje post listar sina säkerhetskopior som en enda tidslinje över alla platser där en säkerhetskopia ligger, på fliken **Säkerhetskopior**. En säkerhetskopia som kopierats till B2 visas en gång, markerad med varje plats som håller den. En återställning tar den första plats den når, med början i arkivet posten skrivs till, och du kan välja en annan plats per säkerhetskopia. Off-site-mål läses först när du klickar **Kontrollera**, eftersom det tar en stund att lista molnlagring. Att radera på en plats kontrollerar först de andra och säger om det var den sista kopian.

## Mål {#destinations}

Ett mål är en plats dit säkerhetskopior går: en lokal mapp, en resurs, en egen server eller en molntjänst. Det sätts upp en gång och betjänar sedan varje avsnitt. Öppna **Mål** från dess inställningsbricka, eller slå på **Visa även i sidofältet** där för att fästa det.

**Lägg till mål** öppnar ett fönster i fyra steg:

1. **Leverantör.** Varje tjänst listas med sin logotyp, med en sökning uppe till höger i fönstret, i grupper: lagringstjänster med S3-buckets (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 och andra), din egen S3-server (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), din egen server och dina resurser (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, en monterad sökväg), molnlagring (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud och resten som rclone stöder) och din grupps mottagarservrar. Varje tjänst anger hur väl den passar för säkerhetskopior: molndiskar saktar ner vid många förfrågningar, så den första säkerhetskopian och rensningen tar längre tid där.
2. **Inloggning.** Fälten beror på tjänsten: en åtkomstnyckel för S3, ett användarnamn och lösenord för WebDAV och SMB, ett applösenord där tvåfaktorsinloggning blockerar det vanliga, BombVaults publika SSH-nyckel för SFTP och Storage Box, eller en token för tjänster som loggar in via en webbläsare. För dem visar fönstret ett `rclone authorize`-kommando att köra på en dator med webbläsare; token som det skriver ut går in i fältet. **Testa** kontrollerar inloggningen innan något sparas. Fönstret listar sedan mapparna på målet, med **Ny mapp** för att skapa en och det lediga utrymmet där tjänsten rapporterar det. En tom mapp är säkrast; BombVault lägger en mapp per avsnitt i den.
3. **Skydd.** Fönstret säger rakt ut vad tjänsten kan. En rest-server i append-only-läge vägrar radering, och manipulationstestet kontrollerar det. En S3-bucket kan behålla gamla versioner med versionshantering och object lock, vilket BombVault ännu inte kan kontrollera. En molndisk kan inte vägra radering alls: den som tar sig in på servern tar sig in i den kopian också. Slå på **Skydd mot radering (append-only)** bara där den bortre sidan verkligen vägrar radering; BombVault rensar då aldrig där. **För nödfall** anger vad återställningskitet innehåller för det här målet.
4. **För vilka avsnitt.** Välj avsnitten som säkerhetskopieras hit. Den första kopieringen tar med hela historiken. Poster som valt egna mål får inte det nya av sig själva, och fönstret namnger dem.

S3-tjänster körs via restics eget S3-backend, vilket är det som låter en lagringsklass och object lock gälla. Varje annan tjänst körs via rclone som BombVault levereras med, och dess fjärr dyker sedan upp i kortet **Off-site (rclone)** under **Inställningar, Mål**. En inställningsexport bär målen; med autentiseringsuppgifter inkluderade bär den också deras inloggning.

Mappen på den här servern heter **Lokal** överallt: på sidorna för mål, i flödet, på en post och under Återställning. Finns det flera mappar på den här servern har var och en sitt eget namn. Ett mål som ett avsnitts off-site-fält skapar får namn efter sin plats, alltså adressens sista mapp eller, om den saknar mapp, dess server, och får ett nummer när ett annat mål redan heter så. Ett namn du ger det på dess sida består. Där två mål med samma namn står bredvid varandra bär var och en dessutom sin server eller mapp.

Varje mål har en egen sida:

- **Anslutning** med namn, reglaget **Tillgängligt** (av betyder att BombVault inte längre skriver hit, och poster som säkerhetskopieras hit misslyckas med ett tydligt meddelande i stället för att hamna någon annanstans), mappen, komprimering, lagringsklass och **Utanför lokalerna**. **Ändra anslutning** testar och sparar en ny adress eller inloggning.
- **Autentiseringsuppgifter**: den delade uppsättningen eller en egen.
- **Raderingsskydd**, med **Testa skyddet mot radering nu** och vem som rensar den här kopian.
- **Kopiera**: när det kopieras, samt upp- och nedladdningsgränserna.
- **Hur länge det bevaras**: **Kort**, **Balanserad**, **Lång** eller **Egna värden**, med en tidslinje över vilka säkerhetskopior regeln behåller. **Fråga innan mindre bevaras** frågar innan en kortare regel raderar säkerhetskopior vars enda kopia finns här.
- **Regler per avsnitt**: ett avsnitts eget arkiv, egen retention eller komprimering vid det här målet. **Ändra regler** öppnar dem, och **Tillbaka till målet** tar tillbaka ett avsnitt.
- **Ligger redan under det här målet**: avsnitt vars handskrivna adress ligger i en mapp hos det här målet. Att ta över ett lämnar arkivet där det är; inloggning, lagringsklass och raderingsskydd kommer då från målet. BombVault kontrollerar först att målets inloggning öppnar arkivet, och vägrar lägga ett append-only-arkiv under ett mål som inte är append-only.
- Ett kärl som visar hur fullt målet är, med markeringen där det är fullt och hur många veckor som återstår vid de senaste veckornas tillväxt. Ett mål som inte rapporterar sin storlek säger **Storlek okänd**.

En mottagarserver som en annan instans i din grupp kör visas bland leverantörerna som **Från din grupp**; se [Mottagarserver](#receiving-server).

### Flytta och ta bort {#move-remove}

- **Flytta** tar ett avsnitts kopia, med hela dess historik, till ett annat mål. Allt blir kvar på den gamla platsen; BombVault raderar ingenting där. Flytten körs i bakgrunden och kan ta lång tid: ungefär 9 minuter per 100 GB mellan lokala diskar, timmar över en långsam länk. Översikten visar den medan den körs.
- **Ta bort mål** erbjuder två vägar. **Ta bort mål, behåll säkerhetskopiorna** slutar skriva dit och lämnar säkerhetskopiorna kvar; du kan läsa dem senare under **Återställning, Från en gammal säkerhetskopiemapp**. **Ta bort målet och radera säkerhetskopiorna där** läser först vad som lagras där och listar det: vilka säkerhetskopior som försvinner, vilka av dem som inte finns någon annanstans, och vilka arkiv som blir kvar eftersom raderingsskyddet vägrar, poster fortfarande säkerhetskopieras rakt till dem, ett annat mål eller avsnitt har sina kopior där, eller de inte gick att läsa. Poster som skulle förlora sin enda säkerhetskopia listas separat och behålls om du inte slår på **Radera dessa också**. Du bekräftar genom att skriva målets namn, och raderingen körs i bakgrunden.
- Att radera säkerhetskopiorna på ett mål kräver ett inloggningslösenord och en inloggning på instansen. Utan lösenord kan målet ändå tas bort, bara dess säkerhetskopior kan inte raderas härifrån.

## Placering per post {#placement}

Kortet **Schema och mål** på en posts flik Inställningar har en rad knappar: **Lokal** och en knapp per off-site-mål i avsnittet. Tända knappar får postens säkerhetskopior.

- Med **Lokal** tänd skrivs posten till arkivet som visas under **Sparad på** och kopieras till varje annat tänt mål. Släck ett mål så får det inget nytt från den här posten. Enbart Lokal kopierar ingenstans, vilket passar data som redan har en andra kopia, till exempel en resurs som ligger på en NAS.
- Med **Lokal** släckt skrivs posten rakt till det första tända målets direkta arkiv och kopieras därifrån till de andra tända. Första gången skapar en dialog det direkta arkivet.
- En knapp förblir tänd, eftersom en säkerhetskopia behöver en plats att gå till. För att lämna en post utanför säkerhetskopiorna, använd **Säkerhetskopiera inte**.

BombVault flyttar aldrig säkerhetskopior mellan arkiv. En post som följer sitt avsnitt följer med när avsnittets standard ändras: dess nästa säkerhetskopia hamnar på den nya platsen, och de säkerhetskopior den redan har blir kvar där de är. En plats som valts på själva posten är fastlagd från dess första säkerhetskopia; kortet **Arkiv** på fliken Inställningar namnger den. Kopiorna kan ändras när som helst. Ett mål som inte längre får en post behåller kopiorna det har och trimmar dem enligt sin egen retention vid avsnittets nästa off-site-körning. Från append-only-mål går det inte att radera.

Platsen som en post har lämnat behåller sina säkerhetskopior. På postens flik **Säkerhetskopior** och under Återställning står den som **Bara äldre säkerhetskopior**, en återställning kan fortfarande läsa därifrån, och en rensning där låter de säkerhetskopiorna vara.

**Följ standarden** för tillbaka en post med eget val till sitt avsnitt. Den frågar först, säger vart nya säkerhetskopior går då och namnger vad varje mål slutar få och vad nästa körning skulle ladda upp.

Under raden anger kortet vart posten går och vad som faktiskt finns där: hur många platser som håller den, när varje mål senast sågs och om 3-2-1 är uppfyllt. En plats är servern med originaldatan, varje off-site-mål och varje arkiv markerat **Utanför lokalerna**. BombVault kontrollerar kopior och platser; det kontrollerar inte delen "två medier" i 3-2-1.

### Standard per avsnitt {#placement-defaults}

Kortet **Mål per avsnitt** på sidan Mål ritar varje avsnitts standard som ett 3-2-1-flöde. Varje linje från ett avsnitt till ett mål är ett reglage, **Lokal** inräknat, för varje avsnitt, även flashenheten, ZFS-datauppsättningarna och auto-säkerhetskopian. Ett klick på en linje eller en punkt slår om den direkt, utan fråga; streckade linjer är av. **Graf / Reglage** visar samma sak som rader med knappar. Ett reglage avgör vart nya säkerhetskopior går. Poster utan eget val och projektmapparna i Compose-stackar följer direkt, så deras nästa säkerhetskopia går till de nya platserna. Säkerhetskopior som redan är lagrade blir kvar där de är och kan fortfarande återställas därifrån. Flash, ZFS-datauppsättningar och auto-säkerhetskopian säkerhetskopieras till sin lokala mapp och kopieras därifrån off-site. Deras linje till **Lokal** kan också slås av; deras säkerhetskopior går då direkt till deras off-site-mål, och en ZFS-post med en replika och inget mål alls skickar bara sin replika.

Med **Lokal** av och bara off-site-mål på säkerhetskopierar ett avsnitt rakt till dem, och BombVault väljer arkivet där själv. Containers, VMs och mappuppsättningar går till det direkta arkivet hos det första mål som är på, beskrivet nedan, och kopieras därifrån till de andra. Flashenheten, ZFS-datauppsättningarna och auto-säkerhetskopian skriver varje säkerhetskopia till varje mål som är på, och varje mål behåller dem enligt sin egen retention.

Ett avsnitt där alla linjer är av säger **Ingen säkerhetskopia**. Dess poster får inga säkerhetskopior och räknas som oskyddade.

Poster som har valt egna mål på sin sida behåller dem. Flödet säger hur många det är i ett avsnitt, till exempel **2 poster avviker**, och leder till dem: till postens flik Inställningar när det är en, till en lista med vart var och en säkerhetskopierar när det är flera. **Följ standarden** där för tillbaka en post.

Ett nytt off-site-mål får varje post som inte är satt till Lokal. Fönstret som lägger till det säger hur många poster och, där det är känt, hur mycket historik det är.

### Direkta arkiv

Ett direkt arkiv är platsen där poster säkerhetskopieras rakt till ett off-site-mål. BombVault lägger det bredvid målets mapp med `-direct` tillagt i namnet, eller, när målet ligger i roten av sin bucket, sitt fjärrmål eller en rest-server-användare, i en mapp i det som är uppkallad efter avsnittet, till exempel `containers-direct`. I flödet på sidan Mål skapas det utan fråga. Att stänga av Lokal för en enskild post, så att ett mål utan direkt arkiv blir dess hem, öppnar en dialog med den föreslagna platsen, till exempel `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, och ett anslutningstest som inte skapar något. **Skapa och använd** skapar arkivet och pekar posten mot det. Ett direkt arkiv tar målets nyckel, lagringsklass, gränser, append-only-inställning och retention och ändras med dem; kortet Arkiv visar det skrivskyddat. När en ny nyckel för målet inte kan öppna det behåller det direkta arkivet den nyckel det har, och sparandet säger det. En post på ett direkt arkiv kopieras därifrån till de andra tända målen, aldrig till målet arkivet tillhör. Dess säkerhetskopior bär taggen `bv:direct`, och varje annan retentionsomgång behåller dem, så ett direkt arkiv som förlorat kopplingen till sitt mål åldras aldrig enligt de lokala reglerna. B2 nås via sin S3-slutpunkt, med nyckel-ID och programnyckel inskrivna som S3-uppgifter; en nyckel begränsad till målets egen mapp kan inte nå mappen bredvid, så begränsa nyckeln till mappen ovanför i stället.

### Utanför lokalerna

Ett mål eller ett namngivet arkiv kan markeras **Utanför lokalerna**. Fjärranslutna börjar markerade; stäng av det för en rest-server i samma byggnad. Markeringen räknar bara platser och 3-2-1 på korten. Den ändrar ingen kopia.

En ZFS-replik är en kopia utanför lokalerna, men inte en säkerhetskopia med historik. Den behåller bara de ögonblicksbilder du anger, och gamla versioner och enskilda filer kommer fortfarande från säkerhetskopiorna. Den går från en ZFS-server rakt till den andra, över SSH till en ZFS-server eller till mottagningspunkten hos en parkopplad instans. Parkopplingsgruppen bär bara begäran. Se [Replik](zfs-datasets.md#replica).

### Efter en ombyggnad {#after-a-rebuild}

Kopieringsvalen finns i BombVaults egna inställningar. Efter en ombyggnad via **Identifiera säkerhetskopior** eller Nödguiden utan återställda inställningar är de borta, och att kopiera allt skulle skicka poster du lämnat utanför till B2 igen. Off-site-kopior av varje ombyggt avsnitt pausas därför. Översikten säger det, och kortet **Mål per avsnitt** erbjuder **Bekräfta standard** med en förhandsvisning av vad nästa körning kopierar och namnen i säkerhetskopiorna utan post, som du kan lämna utanför där. Bara bekräftelsen avslutar pausen; att importera en inställningsfil ger tillbaka regler och standarder men avslutar den inte.

## Fjärranslutna primära arkiv {#remote-primary-repositories}

Ett avsnitts egen sökväg är inte begränsad till en lokal mapp: peka den direkt mot en restic-fjärr (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`) så säkerhetskopierar BombVault direkt dit, utan separat lokal kopia och utan kopieringssteg. Det är en annan form än off-site-kopiorna ovan: där är det lokala arkivet primärt och off-site-arkivet ett best-effort-arkiv av det; här **är** fjärrarkivet det primära och den enda kopian, om du inte också kopierar avsnittet till ett annat mål.

Varje avsnitts sökväg (Containers, VMs, Flash, Auto-säkerhetskopia, Mappar, ZFS-datauppsättningar) anges på sidan för målet **Lokal**: **Ändra** bredvid avsnittet i dess kort **Anslutning** öppnar ett fönster med en omkopplare **Lokal / Fjärran**:

- **Lokal** visar den välbekanta mappbläddraren.
- **Fjärran** byter ut den mot ett vanligt URL-fält och anslutningsinställningarna ett off-site-mål använder, konfigurerade för det här primära i stället. Därifrån får du:
    - **Ett anslutningstest** mot den levande sökvägen, innan du förlitar dig på den.
    - **Bandbreddsgränser** (upp-/nedladdning) så att en schemalagd säkerhetskopia till ett fjärranslutet primärt arkiv inte mättar ditt WAN: samma restic-flaggor `--limit-upload`/`--limit-download` som off-site-kopior använder, tillämpade på själva säkerhetskopieringen.
    - **Append-only-skydd**, verifierat med samma aktiva manipulationstest (en riktig DELETE-sond mot den bortre sidan) som off-site-mål får. Med det påslaget vägrar BombVault att själv rensa arkivet, eftersom det inte finns någon separat lokal kopia bakom det och uppgifterna på den här boxen inte får kunna radera den enda kopian av säkerhetskopian.
    - **Ett larm för tillväxtbudget**, samplat från samma arkivstorlekstrend som kortet Lagring redan följer.

Inget av detta är obligatoriskt: en handskriven fjärrsökväg utan sparade säkerhetsinställningar säkerhetskopierar obegränsat, rensningsbart och utan budgetlarm. Säkerhetsinställningarna finns för när du vill ha skydden en off-site-kopia får, utan ett separat mål bara för att få dem.

!!! note "Moln-/REST-uppgifter är delade"
    Ett fjärranslutet primärt arkiv autentiserar med de delade molnautentiseringsuppgifterna under **Inställningar, Mål**; det finns ingen separat uppgiftslagring för primära arkiv.

### SMB och WebDAV utan värdmontering {#smb-webdav}

Kortet **Off-site (rclone)** under **Inställningar, Mål** har ett formulär för en Windows- eller Samba-resurs och för en WebDAV-server (Nextcloud, ownCloud, SharePoint eller någon annan). Fyll i ett kort namn, värd och resurs (SMB) eller URL och servertyp (WebDAV), användare och lösenord, så skriver BombVault rclone-avsnittet åt dig. rclone fördunklar lösenordet själv innan det lagras; att lägga till ett med ett namn som redan finns ersätter det avsnittet i stället för att lägga till ett andra. **Lägg till mål** erbjuder SMB och WebDAV som leverantörer med samma fält.

Formuläret svarar med det färdiga målet, till exempel `rclone:nas:backups`. Använd det som ett avsnitts sökväg eller ett mål och lägg till en undermapp om du vill ha en (`rclone:nas:backups/bombvault`). Resursen är det första sökvägssegmentet, inte en del av namnet.

Det här är den bättre vägen än att montera resursen på Unraid: restic avråder från att hålla ett arkiv på en monterad CIFS-resurs, och här monteras ingenting. NFS finns inte i formuläret eftersom varken restic eller rclone har ett NFS-backend; för NFS, montera exporten på värden och använd den som en lokal mapp.

## Raderingsskydd (append-only)

Slå på raderingsskydd för ett off-site-mål så att ransomware, eller en komprometterad värd, inte kan radera eller skriva över dina säkerhetskopior. Den bortre sidan (en `restic/rest-server` i läget `--append-only`) **upprätthåller** det. BombVault **verifierar** det bara och visar aldrig grönt på ett konfigurationspåstående ensamt. En mapp på den här servern kan inte vägra radering, eftersom den som styr BombVault styr också mappen, så lokala mål erbjuder inte reglaget.

Snabbaste vägen till en append-only-kopia är en parkopplad BombVault med en [mottagarserver](#receiving-server): den kör rest-servern åt dig, i append-only-läge.

!!! note "En lyckad radering under `/locks/` är väntad"
    Append-only betyder inte att inget någonsin kan tas bort. restic måste ta och släppa sina egna lås, så `/locks/` förblir skrivbar och raderbar med flit. Säkerhetskopiorna och datan bakom dem, som är det ransomware skulle gå efter, kan inte tas bort. Om du själv provar den bortre sidan är en radering som lyckas under `/locks/` korrekt beteende och inget hål i skyddet.

!!! warning "Append-only-arkiv rensas aldrig från den här boxen"
    Ett append-only-mål rensar aldrig gamla säkerhetskopior härifrån; rensning är den bortre sidans jobb. Ange ett **larm för tillväxtbudget** för det så att du varnas innan arkivstorleken skenar.

## Manipulationstest

BombVault bevisar med jämna mellanrum append-only-garantin genom att faktiskt försöka radera i off-site-arkivet, riktat mot ett obefintligt objekt. **Testa skyddet mot radering nu** på ett måls sida kör det på begäran, och **Kontrollera raderingsskydd** under **Inställningar, Verifiering, Automatiska kontroller** kör det enligt ett schema.

- **Nekat** betyder skyddat.
- **Accepterat** betyder oskyddat.
- Ett **ofullständigt** resultat (servern onåbar, autentiseringsfel) vänder aldrig det lagrade utlåtandet.

En verklig övergång från skyddat till oskyddat utlöser ett enda larm.

## DR-övningar

BombVault erbjuder två nivåer av bevis för att dina säkerhetskopior verkligen går att återställa, inte bara finns.

- **Återställningsverifieringsövningar (lokala).** BombVault kör med jämna mellanrum `restic check --read-data-subset` (begränsat, aldrig en diskfyllande full återställning). Kadensen finns under **Inställningar, Verifiering, Automatiska kontroller**, och kortet **Kontrollera säkerhetskopior** på samma bricka visar resultatet.
- **Provåterställning (off-site).** BombVault återställer ett riktigt mål från off-site-arkivet i en avskärmad sandlåda, verifierar det fil för fil och byte för byte och städar sedan upp. Det bevisar att du kan återhämta dig från off-site, inte bara att arkivet svarar.

Kortet **Skyddsstatus** på översikten sammanfattar detta per avsnitt i fyra kontroller: en aktuell säkerhetskopia på den här servern, en kopia off-site, raderingsskydd där och en provåterställning därifrån. Ett klick på ett avsnitt visar detaljerna, och kortet blir bara grönt på verifierade fakta.

## Parkoppla instanser {#pairing}

Flera BombVault arbetar tillsammans som en grupp: de visar varandras skyddsstatus, tar på sig roller åt varandra och kan förvara varandras nyckel. En instans går med i gruppen med tolv ord.

Öppna **Inställningar, Parkoppling** på den första instansen och tryck på **Skapa fras**. Tre stegkort visar hur det går till. Tolv ord visas i ett fönster med en knapp **Kopiera**. På varje annan instans öppnar du samma ställe, trycker på **Ange fras** och klistrar in eller skriver dem, eller trycker på **Klistra in** i det fönstret. Ett ord som inte finns i listan namnges med sin position medan du skriver, och det sista ordet bär en kontrollsumma, så ett felskrivet eller bytt ord upptäcks innan något parkopplas. Skapa frasen på bara en instans: två instanser som båda skapar en bildar två skilda grupper. Om ingen dyker upp inom en minut erbjuder sidan två utvägar: visa orden igen för att ange dem där borta, eller ange den andra instansens ord och gå med i dess grupp i ett steg. Parkoppling fungerar utan inloggningslösenord, men ange ett: utan det kan vem som helst som kan öppna webbgränssnittet läsa orden och, via gruppen, be om roller som lämnar ut restic-lösenordet för instanserna i den. Parkopplingskortet säger det tills ett lösenord är angivet. Med ett lösenord frågar visning av frasen igen efter det. **Lämna gruppen** tar ut en instans igen. Kortet **Instanser** på parkopplingssidan listar gruppens medlemmar; bilden över vem som gör vad för vem finns på sidan Instanser.

Den som kan orden kan gå med i gruppen, så behandla dem som ett lösenord.

**Hur medlemmar når varandra.** Varje instans får veta sin egen adress i nätverket av din webbläsare i samma stund du loggar in, listad under **Kända adresser** i kortet **Den här instansen** på parkopplingssidan tillsammans med dem instansen hittade själv; rätta den där om en omvänd proxy eller en ovanlig port sitter framför. I samma nätverk annonserar medlemmar den adressen med multicast och talar direkt, och där multicast inte kan korsa ett containernätverk, till exempel Dockers standardbrygga, söker en instans i stället igenom sitt eget subnät efter de andra med ett signerat anrop som bara en gruppmedlem kan svara på, så parkopplingen blir klar på sekunder ändå utan relä. Om inget dyker upp tar **Hittar du den inte?** emot en adress för hand, för ett annat subnät eller en ovanlig port. Instanser i olika nätverk går via ett relä, valt i en rad med fyra i kortet **Relä** på samma sida; varje form har var sin bild:

- **Projektets relä** (standard): `parleyport.halleluja.design`, samma relä som KnightLoader använder. Inget att ställa in.
- **Eget relä**: containern [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) från Unraid Community Apps (också under **Inställningar, Appar**). Ange dess adress på varje instans som ska använda det.
- **BombVault-instans**: en av dina instanser som redan nås utifrån, med **Fungera som relä** påslaget. Den instansen svarar då på `/relay/connect` på sin egen adress, bakom den omvända proxy och det certifikat den redan har, och släpper bara in din grupp. Ange dess adress på varje instans som ska använda det.
- **Inget relä**: medlemmar hittar varandra automatiskt i samma nätverk, och ingen annanstans.

**Vad reläet ser.** En knapp med samma namn bredvid de fyra formerna visar det för formen du valt. Varje anrop mellan medlemmar förseglas med AES-256-GCM under en nyckel härledd ur de tolv orden, och den nyckeln lämnar aldrig dina instanser. Reläet får veta en hash som grupperar anslutningarna, vilken instans ett meddelande är till, hur stort det är och när det passerar. Ett direkt anrop i det lokala nätverket förseglas på samma sätt och signeras dessutom, så ingenting beror på det självsignerade certifikat en instans serverar. Anrop som lämnar ut något (rollsvar, ett restic-lösenord, mottagarinloggningar, nyckeldepositioner) signeras också med en nyckel för medlemmen som skickar dem, fästad vid första anblicken; se [Säkerhetsmodell](configuration.md#security-model).

**Vad som färdas över gruppen.** Skyddsstatusen på sidan Instanser, en begäran att kontrollera ett avsnitt nu, rollförfrågningar och svaren på dem, erbjudanden om mål och det en mottagare eller hämtare behöver när dess roll väl tillåtits: den andra instansens arkivplatser och dess restic-lösenord. Säkerhetskopieringsdata gör det aldrig; den går fortfarande rakt till restic-backenderna. Inte heller APP_KEY: restic-lösenordet öppnar den instansens arkiv och inget annat, inte dess lagrade hemligheter, sessioner eller återställningskoder.

**Poster från före parkopplingen.** Instanser som lagts till med en fleet-token, och mottagna arkiv och hämtningskällor som satts upp med den andra instansens APP_KEY, finns kvar efter uppdateringen och är markerade **Parkoppla igen**. De fortsätter fungera: vid första starten ersätter BombVault varje lagrad APP_KEY med restic-lösenordet härlett ur den. Parkoppla båda instanserna, redigera sedan posten och välj dess instans. Tills dess står sådana poster i kortet **Utan instans** på sidan Instanser.

Det enda ställe som fortfarande tar en APP_KEY för hand är [Från en gammal säkerhetskopiemapp](#restore-from-another-bombvault-repo), för fallet att den andra instansen är borta och inte kan svara i en grupp.

## Instanser och roller {#roles}

Sidan **Instanser** (en inställningsbricka som kan fästas i sidofältet) visar gruppen som ett nät över vem som gör vad för vem, ett kort för den här instansen och ett kort för varje annan medlem, Android-telefonerna och ZFS-servrarna. **Parkoppling** nere till höger leder till parkopplingssidan. Vilka delar som är på ställs in under **Inställningar, Instanser, Flera servrar**: **Instanser**, **Mottagare** och **Hämtning**.

Varje kort visar medlemmens skyddsstatus och dess roller som brickor. En roll är något en instans gör för en annan:

- **Mottagare** förvarar den här serverns off-site-kopior i en rest-server i append-only-läge. Den här servern skickar efter varje säkerhetskopia och kan inte radera något där, även om någon tar över den.
- **Hämtare** hämtar den här serverns säkerhetskopior enligt ett eget schema. Den här servern behöver inte vara vaken för det.
- **ZFS-server** tar emot den här serverns ZFS-repliker. Varje ZFS-post väljer den under **Replik** i sina inställningar.
- **Nyckelförvarare** förvarar en förseglad kopia av den här serverns återställningskit. Se [Nyckelförvarare](#key-keeper).

Att slå på en roll skickar en begäran, och ingenting startar förrän någon på den andra instansen trycker på **Tillåt**. Begäran visas på den instansens kort som ett märke, och dess **Detaljer** listar den med **Tillåt** och **Avvisa**; en tillåten roll kan återkallas där när som helst. En streckad linje i nätet är en roll som fortfarande väntar. En instans på BombVault 9.x kan inte svara på en begäran, så en roll är aktiv där så fort du slår på den, och en begäran från en 9.x-instans väntar här på **Tillåt**.

**Detaljer** på ett kort öppnar resten: den medlemmens mottagna arkiv och hämtningskällor, erbjudanden om mål, övervakning, **Ta bort** och **Glöm nyckeln**.

## Mottagare och hämtare {#receiver}

![En mottagen off-site-kopia i en instanss detaljer, bevakad skrivskyddat, med en integritetskontroll körd på den här hårdvaran.](assets/screenshots/receiver.png)

*En mottagen off-site-kopia i en instanss detaljer, bevakad skrivskyddat, med en integritetskontroll körd på den här hårdvaran.*

På boxen som **tar emot** append-only-kopior från en annan BombVault ger de mottagna arkiven dig oberoende, skrivskyddad övervakning på den mottagande hårdvaran, så att ett tyst fel i andra änden inte går obemärkt förbi. Slå på **Mottagare** under **Inställningar, Instanser**, öppna sedan den sändande instansens **Detaljer** på sidan Instanser och lägg till det mottagna arkivet (skrivskyddat, öppnat med den sändande instansens restic-lösenord, som den får över [parkopplingsgruppen](#pairing)). Du får:

- **En inventering per källa**, så att du ser exakt vilka containrar, VM:ar och mappuppsättningar som har anlänt.
- **Senast mottaget** per källa, så att du vet hur färsk varje är.
- **En oberoende `restic check`** körd på den mottagande hårdvaran, så att integriteten verifieras där datan faktiskt ligger, inte bara hos avsändaren.
- **En dödmansknapp:** ett larm när en källa slutar skicka inom ett fönster du anger.
- **Integritetslarm:** ett larm när en kontroll på den mottagande sidan misslyckas.

Mottagaren är strikt skrivskyddad. Den skriver aldrig till det mottagna arkivet, så den kan aldrig bryta append-only-garantin som avsändaren förlitar sig på.

En hämtare arbetar åt andra hållet: med **Hämtning** påslaget läser en instans en annan medlems säkerhetskopior in i sitt eget arkiv enligt ett eget schema. Dess källor står också i den medlemmens detaljer.

### Mottagarserver {#receiving-server}

Den mottagande boxen kan också köra rest-servern som de andra kopierar till. Den sätts upp första gången du tillåter en **Mottagare**-förfrågan som ber om den: fönstret säger att servern behöver en mottagarserver och ber om en mapp på en resurs, med **Ny mapp** för att skapa en, och en port (8000 om inte en annan container använder den). BombVault då:

1. vägrar om en container som heter `rest-server` redan finns eller en annan container håller porten;
2. hämtar `restic/rest-server` och startar den via Docker-socketen i append-only-läge med privata arkiv och en inloggningsfil i mappen;
3. skriver dess Unraid-mall till flash-enheten, så att containern förblir redigerbar på fliken Docker, eller erbjuder mallen som nedladdning när flash-enheten inte går att nå;
4. kör manipulationstestet mot den och visar om den vägrar radering.

Instanserna i din grupp hittar sedan servern bland leverantörerna under **Från din grupp**, uppkallad efter den mottagande boxen, och rollen **Mottagare** erbjuder den som det säkra valet. Varje instans får en egen inloggning första gången den använder servern och skriver bara i sin egen mapp där. Med **Mottagare** påslaget under **Inställningar, Instanser** öppnar **Mottagarserver** på den här instansens kort servern igen. Dess fönster listar inloggningarna under **Partner med inloggning**, och **Återkalla inloggning** tar bort en; det den instansen redan kopierat blir kvar i mappen. Uppsättningen skapar också en **Inloggning utanför gruppen**, vars lösenord fönstret visar en gång.

En instans som når den mottagande boxen bara via reläet kan inte använda servern, eftersom reläet inte bär några säkerhetskopior. Lägg först till den mottagande boxens adress på parkopplingssidan. När BombVault körs på en egen IP-adress (på br0, till exempel), fyll i **Adress för partner**, eftersom servern lyssnar på värdens adress.

## Komplett exempel: två Unraid-boxar, från början till slut

Allt ovan beskriver delarna. Det här är en komplett uppsättning, eftersom delarna är lättare att sätta ihop när man sett dem hopsatta en gång.

Två boxar: **TOWER** kör containrarna och skickar säkerhetskopior; **VAULT** tar emot dem och upprätthåller append-only. Byt ut mot dina egna namn, adresser och resurssökvägar.

**1. Parkoppla de två boxarna.** Öppna **Inställningar, Parkoppling** på TOWER, tryck på **Skapa fras** och skriv in de tolv orden på VAULT under **Ange fras**. Ange först ett inloggningslösenord på båda.

**2. På TOWER, be VAULT vara mottagare.** På sidan Instanser slår du på **Mottagare** på VAULT:s kort, väljer avsnitten och **VAULT:s mottagarserver** som plats och skickar begäran.

**3. På VAULT, tillåt.** VAULT:s sida Instanser visar begäran på TOWER:s kort. Tryck på **Tillåt**. Första gången ber VAULT om en mapp för sin mottagarserver, till exempel `user/appdata/rest-server`, startar rest-servern i append-only-läge och kör manipulationstestet mot den.

**4. Tillbaka på TOWER.** Från då kopierar TOWER till VAULT efter varje säkerhetskopia, och det nya målet visas på TOWER:s sida Mål med sitt raderingsskydd.

Vad manipulationstestet svarar:

| Resultat | Vad som hände |
| --- | --- |
| **skyddat** | VAULT vägrade raderingen. Det här är det enda godkända läget. |
| **oskyddat** | VAULT accepterade en radering. `--append-only` saknas eller togs bort. |
| **ofullständigt** | Ingetdera. Oftast är URL:en inte den restic själv använder, eller så ändrades uppgifterna. Ingenting registreras och inget larm utlöses. |

**5. Bevaka det som kommer, om du vill.** Slå på **Mottagare** under **Inställningar, Instanser** på VAULT, öppna TOWER:s **Detaljer** och lägg till det mottagna arkivet. VAULT får TOWER:s restic-lösenord över gruppen; ingen skriver en nyckel.

**6. Gör det ömsesidigt.** Upprepa stegen i andra riktningen: VAULT ber TOWER vara dess mottagare. Varje box upprätthåller då append-only åt den andra, och ingen kan radera den andras säkerhetskopior.

En rest-server du kör för hand fungerar också: lägg till den med **Lägg till mål** som rest-server, med användare och lösenord från dess `.htpasswd`, och slå på raderingsskydd. Behåll `--append-only` i dess alternativ; utan det är den bortre sidan en vanlig resurs igen.

!!! warning "En lokal sökväg är en sökväg inuti containern"
    Där BombVault ber om en mapp på den här servern, ange den relativt värdmonteringen, till exempel `user/appdata/rest-server/bombvault-containers`, inte `/mnt/user/appdata/...`. BombVault körs i en container, där värdens `/mnt` är monterad någon annanstans; en absolut värdsökväg finns inte inuti den. Om du klistrar in en säger BombVault vilken relativ sökväg du ska använda i stället.

## Nödguide {#emergency}

**Nödguiden** tar tillbaka en server som är borta eller sattes upp på nytt. Den startar på sidan **Återställning** under **Nödläge**, och från **Återställ från en säkerhetskopia** på välkomstsidan för en ny installation. Den går i fyra steg: **Åtkomst**, **Inställningar**, **Urval** och **Återställning**.

1. **Åtkomst.** Visa BombVault nyckeln till de gamla säkerhetskopiorna, på ett av tre sätt:
    - **Ladda upp återställningskit**: filen du laddade ner under uppsättningen. Den innehåller nyckeln och varje mål, och BombVault fyller i resten från den. Ett kit förseglat med age frågar efter identiteten det förseglades för; den används en gång och lagras inte.
    - **Via gruppen**: en instans i din grupp förvarar nyckeln som en deposition. Du behöver gruppens tolv ord, din utlämningskod och någon vid den instansen. Se [Nyckelförvarare](#key-keeper).
    - **Skriv in nyckeln**: den gamla serverns `APP_KEY`, och var dess säkerhetskopior finns.

    BombVault provar sedan nyckeln på varje mål och säger för vart och ett om det är läsbart, inte hittat, låst med en annan nyckel, inte nåbart eller fortfarande saknar inloggning. Ingenting har sparats ännu, och nyckeln finns kvar i minnet under begränsad tid. **Ta över nyckeln** får den här servern att skriva sina säkerhetskopior med den nyckeln från och med nu; **Ångra övertagandet** går tillbaka till den egna nyckeln tills den första säkerhetskopian har körts. En server som redan har egna säkerhetskopior vägrar övertagandet och hänvisar till [Från en gammal säkerhetskopiemapp](#restore-from-another-bombvault-repo). **Spara inloggningsuppgifter** lagrar inloggningarna kitet bär för fjärranslutna mål, och frågar innan det ersätter inloggningar den här servern redan har.
2. **Inställningar.** **Återställ inställningar** återställer BombVaults egna inställningar från den nyaste auto-säkerhetskopian: mål, autentiseringsuppgifter, scheman och regler. BombVault startar om för att tillämpa dem, via Docker-socketen, så den aktiva inställningsdatabasen skrivs aldrig över under ett öppet handtag, och guiden fortsätter av sig själv efteråt. Den här serverns inloggning blir kvar: du fortsätter logga in med lösenordet du använder nu. Hemligheter den gamla servern lagrat som inte kan tas över listas, så att du kan ange dem igen.
3. **Urval.** BombVault går igenom dina säkerhetskopior och listar containrarna, VM:arna, mappuppsättningarna och ZFS-datauppsättningarna den hittar. Den nyaste säkerhetskopian är förvald överallt. Det den här servern fortfarande har börjar avstängt, eftersom en återställning skulle ersätta det. Mappuppsättningar vet inte längre var de kom ifrån efter en ombyggnad, så de går till en mapp du väljer.
4. **Återställning.** Allt valt kommer tillbaka på en gång, lämnat stoppat om du vill starta varje container och VM själv, vilket är säkrast efter en total förlust. Ladda efteråt ner ett nytt återställningskit: det gamla hör till den tidigare installationen.

!!! tip "Planerad flytt kontra katastrof"
    Nödguiden återställer BombVaults egna inställningar från en säkerhetskopia. För en *planerad* flytt till en ny box kan du i stället föra över din konfiguration direkt med kortet **Exportera / importera inställningar** (en portabel JSON-fil). Se [Konfiguration](configuration.md#portable-settings-export-and-import).

### Från en gammal säkerhetskopiemapp {#restore-from-another-bombvault-repo}

**Från en gammal säkerhetskopiemapp** under **Andra källor** på sidan Återställning öppnar en *annan* BombVault-instans arkiv (en resurs monterad under `/mnt`, eller en fjärradress) med den instansens återställningskit eller dess inskrivna `APP_KEY`, i en engångssession, skrivskyddad. Bläddra bland containrarna, VM:arna, mappuppsättningarna och ZFS-datauppsättningarna som lagras där, välj en säkerhetskopia och återställ den, så blir det återställda objektet en vanlig lokal container, VM eller mappuppsättning. Ingenting skrivs någonsin till det andra arkivet, och dina egna säkerhetskopieringsinställningar förblir orörda (sessionen lever i minnet och löper ut av sig själv). Att flytta en container från server A till server B innebär inte att du pekar om dina inställningar och återställer dem efteråt. Det här fönstret är en engångsgrej: det öppnar en session, återställer det du väljer och glömmer den andra instansen. Vill du i stället ha ett stående arrangemang, där den här boxen enligt ett schema hämtar en annan instans säkerhetskopior in i sitt eget arkiv, är det rollen **Hämtare** på sidan Instanser.

En container vars nätverk inte finns på den här servern, till exempel ett Unraid-nätverk `br0` på en vanlig Docker-värd, visar en nätverksväljare under sin rad. BombVault skapar det i nätverket du väljer, tillsammans med dess övriga nätverk. Den fasta IP-adressen och MAC-adressen tillhörde det gamla nätverket och tas bort, så det nya nätverket tilldelar dem.

## Nyckelförvarare {#key-keeper}

En nyckelförvarare är en instans i din grupp som håller en förseglad kopia av den här serverns återställningskit. Den kan inte öppna kopian själv. Om den här servern någonsin förloras ber en ny installation som går med i din grupp förvararen om kopian och öppnar den med din utlämningskod.

1. På sidan Instanser slår du på **Nyckelförvarare** på kortet för instansen som ska förvara den och väljer **Deponera**. Att deponera kräver ett inloggningslösenord.
2. Den här servern visar **Utlämningskoden** en gång: tjugo bokstäver och siffror i fyra grupper om fem. Skriv ner den och bekräfta med **Jag har skrivit ner koden**. Den här servern lagrar inte koden, bara det den behöver för att försegla en deposition för den.
3. Förvararen håller nu depositionen, och den här servern förnyar den när kitet ändras. **Nyckeldeposition** på den här instansens kort listar **Den här serverns deposition** med varje förvarare och dess tillstånd, och **Depositioner som den här servern förvarar** för de andra. **Skapa en ny kod** ersätter koden; varje förvarare byter till den nya när den får den nya depositionen.

Den dagen behöver du tre saker: gruppens tolv ord, utlämningskoden och åtkomst till förvararens gränssnitt, där någon måste trycka på **Tillåt**. Välj **Via gruppen** i Nödguiden, välj depositionen, be om den och skriv utlämningskoden. Koden används på den nya servern och skickas inte till förvararen. Hos förvararen visas begäran under **Nyckeldeposition** med vem som ber, om den instansen någonsin setts där förut och hur ofta den bett; **Tillåt** lämnar ut den förseglade kopian, och att tillåta kräver ett inloggningslösenord. Den nya installationen har 30 minuter på sig att hämta den efter Tillåt, en begäran som ingen besvarar löper ut efter ett dygn, och en instans som ber för ofta måste vänta en timme.

Utan koden kan ingen öppna kopian, förvararen inräknad. Om koden förloras, skapa en ny medan den här servern fortfarande körs; om servern redan är borta fungerar återställningskitet och den inskrivna APP_KEY fortfarande. Förvararen kan radera en deposition när som helst, och ägaren måste då be igen.

## Återställningskit {#recovery-kit}

Det här är delen som gör katastrofåterställning möjlig även när ingen BombVault körs.

**Ladda ner återställningskit** i kortet **För nödfall** under **Inställningar, Återställning** ger dig **nyckeln**, det **härledda restic-lösenordet**, varje **mål** med arkivet för varje avsnitt, inloggningarna en ny installation behöver för att nå dem och de **exakta restic-kommandona**, så att du kan återställa direkt med restic-CLI på vilken dator som helst. Det kräver ett inloggningslösenord. Kitet skriver ut nyckeln också med kryptering avstängd, eftersom container- och VM-definitionerna alltid är förseglade med den. Efter ett övertagande skriver det ut den övertagna säkerhetskopienyckeln och den här serverns APP_KEY, som en ny installation bara behöver för att få tillbaka inställningarna. Ett kort **Spara ditt återställningskit** på översikten påminner dig tills du har sparat det.

!!! danger "Förvara återställningskitet utanför servern"
    Kitet innehåller hemligheten som dekrypterar dina säkerhetskopior. Förvara det på en säker plats åtskild från servern (en lösenordshanterare, en utskrift i ett kassaskåp). Om du förlorar både BombVault och `APP_KEY` utan återställningskit och utan nyckeldeposition går dina krypterade säkerhetskopior inte att återställa.

!!! warning "Den nyaste säkerhetskopian är inte alltid den som ska återställas"
    Sedan restic 0.17 visar `restic snapshots` varje ögonblicksbilds storlek. Efter dataförlust kan den nyaste säkerhetskopian vara den tömda, så återställ inte en säkerhetskopia som är mycket mindre än de före den. Efter ransomware kan det vara den krypterade i vanlig storlek. Om BombVault fortfarande körs, kontrollera först sidan **Avvikelser**: den namnger den sista goda säkerhetskopian, och återställningsfönstret förväljer den. En återställning behöver ingen av BombVaults avvikelsedata, och retentionspausen behåller bara fler säkerhetskopior.

### Att försegla kitet

Om du har slagit på **Kryptera exporter med age** under **Inställningar, System** är kitet förseglat med den också och laddas ner som `bombvault-recovery-kit.md.age`. Det är ASCII-armerat i stället för binärt, så det är fortfarande vanlig text: att klistra in det i en lösenordshanterare eller skriva ut det fungerar precis som förut, innehållet är bara oläsligt utan din nyckel.

!!! warning "Förvara inte age-nyckeln inuti kitet"
    Du behöver din **privata** age-nyckel för att öppna ett förseglat kit. Förvara den någonstans som inte beror på kitet självt, annars har du två saker att återställa i stället för en. Att försegla är värt det när kitet förvaras någonstans du inte helt kontrollerar (en delad lösenordshanterare, molnanteckningar, en utskrift på ett kontor); ett kit i ditt eget kassaskåp är redan skyddat av kassaskåpet.

    Med kryptering på och ingen användbar mottagare konfigurerad vägras nedladdningen helt. BombVault faller aldrig tillbaka på att lämna ut huvudnyckeln i klartext.

### Om du inte har kitet till hands

Lösenordet lagras ingenstans, det **beräknas** ur säkerhetskopiornas nyckel, så du kan återskapa det själv med inget annat än nyckeln och ett skal:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

Det är HMAC-SHA256 över den fasta strängen `bombvault:restic-repo`, med nyckeln som råbytes av hex-`APP_KEY`, utskrivet som 64 hextecken med gemener. Samma värde finns i kitet, listat som det härledda restic-lösenordet; det här är för dagen då kitet finns någonstans där du inte är. På en server som tagit över en annans säkerhetskopior, använd den övertagna säkerhetskopienyckeln, som är den serverns gamla APP_KEY.

!!! warning "För ett mottaget arkiv, använd nyckeln för den SÄNDANDE instansen"
    Ett arkiv som kom hit genom en off-site-kopia skapades av maskinen som skickade det, med **dess** nyckel. Att härleda ur den mottagande boxens nyckel ger ett lösenord som restic avvisar, vilket ser exakt ut som ett korrupt arkiv men inte är det. Det är den vanliga orsaken till att `restic check` på ett mottaget arkiv frågar efter lösenord om och om igen.

Eftersom återställningsdefinitionerna ligger **inuti** varje arkiv (`<repo>/def`, `<repo>/vm-def`) är en kopierad arkivmapp helt fristående, så kitet plus arkivet är allt en återställning på blank hårdvara behöver.

## Att få tillbaka en databasdump {#database-dumps}

En databasdump är en egen återställningspunkt i containerarkivet, taggad `dbdump:<container>`, med den enda filen `/dbdump/<container>.sql`. BombVault listar, laddar ner och importerar dem på containerns flik **Säkerhetskopior**; nedan finns samma steg med bara restic, för dagen då BombVault inte finns.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Taggarna `dbversion:` och `dbname:` på varje dump anger vilken serverversion den kommer från och vilka databaser den innehåller. En komplett fil slutar med `-- PostgreSQL database cluster dump complete` eller `-- Dump completed`.

Importera den i en container med samma eller en nyare version (PostgreSQL), eller samma huvudversion (MySQL och MariaDB), startad en gång med en tom datamapp så att den initieras. Värden behöver ingen databasklient, containern har en:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

För en enskild databas ur en full dump tar MySQL och MariaDB `--one-database <namn>` på klientkommandot. En PostgreSQL-dump har ett avsnitt per databas, vart och ett inleds med en rad `\connect <namn>`: kopiera det avsnittet till en egen fil och importera det med `-d <namn>` efter att du skapat databasen.

!!! warning "En root-dump bär serverns användare"
    En full MySQL- eller MariaDB-dump tagen som root innehåller systemdatabasen `mysql`, så att importera den ersätter den nya serverns konton, root-lösenordet inräknat, med dem från dumpen. På PostgreSQL är `role ... already exists` för användaren containern skapade väntat och ofarligt.
