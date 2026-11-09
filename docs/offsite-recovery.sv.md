# Off-site och återställning

!!! note "Off-site-kopior väntar efter en återuppbyggnad"
    När Nödguiden eller **Identifiera säkerhetskopior** bygger upp poster utan de gamla inställningarna väntar off-site-kopiorna av de avsnitten tills du tar över deras säkerhetskopior som den här serverns egna. Se [Efter en återuppbyggnad](#after-a-rebuild).

Lokala säkerhetskopior skyddar dig mot en förlorad container eller en dålig uppdatering. Off-site-kopior och ett testat återställningskit skyddar dig mot hela lådan, ransomware eller en brand. Den här sidan täcker lagringsmål och off-site-kopior, hur du gör en kopia manipuleringssäker, hur du bevisar att du kan återställa, hur du parkopplar flera servrar och hur du får tillbaka allt när BombVault självt är borta.

## Off-site-kopior {#off-site-copies}

Behåll den snabba lokala säkerhetskopian och lägg till ett eller flera lagringsmål utanför huset. BombVault kopierar nya ögonblicksbilder dit med `restic copy` efter bästa förmåga, så ett off-site-hack får aldrig den lokala säkerhetskopian att misslyckas. I den här formen skriver varje avsnitt först till sin egen mapp och off-site-repositoriet är en kopia. Den egna mappen behöver inte ligga på den här servern: se [Ett avsnitts egna mapp](#own-folder) för att hålla den i ett lagringsmål som S3 eller en rest-server.

- **Flera lagringsmål per avsnitt.** Varje avsnitt (containrar, VM:ar, flash, auto-säkerhetskopia, mappar och ZFS-datauppsättningar) kan kopiera till flera externa lagringsmål på en gång, så du kan till exempel hålla en rest-server hos en vän och en S3-bucket parallellt. Kortet **Mål per avsnitt** på sidan Lagringsmål slår på eller av varje rad från ett avsnitt till ett mål.
- **När kopior körs.** Ett måls kort **Kopiera** ställer in **När det kopieras**: **Efter varje säkerhetskopia**, eller enligt en egen plan som samlar ögonblicksbilderna och kopierar vid fasta tider. Under **Inställningar, Schema, Externa kopior per avsnitt** ger **Undantag** ett avsnitt en egen kopieringsplan.
- **Kortet Externa kopior.** Under **Inställningar, Lagringsmål** har varje avsnitt med en off-site-kopia en rad i **Externa kopior**, med **Testa anslutning**, **Replikera nu** för att starta en kopiering på begäran, och **Konfigurera** eller **Redigera**. Resten av avsnittets kopieringsinställningar öppnas i dess fönster därifrån.
- **Gallring** ställs in per lagringsmål under **Hur länge det bevaras**, så att du kan behålla off-site-kopior längre som ett arkiv. Kopian i ett avsnitts off-site-fält följer den som varje annan kopia, och ett avsnitt kan behålla egna värden under **Regler per avsnitt**. Alla noll trimmar aldrig off-site-ögonblicksbilder. Se [Hur länge det bevaras](#keep).
- **Bandbreddsgränser** (**Off-site-bandbredd** under **Inställningar, Lagringsmål**, eller ett måls kort **Kopiera**) begränsar restics upp- och nedladdningstakt så att kopior inte mättar ditt WAN. De anges i KiB/s, enheten restic tar, och varje fält visar samma takt i MB/s och Mbit/s medan du skriver: 2048 läses som ungefär 2,0 MB/s (16,8 Mbit/s).
- **Komprimering** är **Av**, **Automatiskt** (restics standard) eller **Maximal**, inställd på varje lagringsmål och, för ett avsnitt, under dess regler där. Säkerhetskopior, kopior och rensning skriver med den, och återställningskitet nämner den, så att vanligt restic kan fortsätta skriva på samma sätt.
- **Streaming först.** Medan en mediaserver som Plex, Jellyfin eller Emby strömmar laddar kopior upp med en lägre gräns och går tillbaka till den normala några minuter efter att strömmen slutat. Kortet **Streaming** på sidan Lagringsmål har **Sänk off-site-kopior under streaming**, **Mediaservrar** (containrar med Plex, Jellyfin eller Emby i imagenamnet tills du väljer), **Stream över (Mbit/s)**, **Uppladdningsgräns under streaming (KiB/s)** och **Tillbaka till normalt efter (min)**. BombVault läser trafiken från Docker, så en mediaserver på värdnätverket går inte att mäta. REST, S3, B2, Azure, Google Cloud, Swift och rclone över HTTP bromsar mitt i en kopiering. SFTP och lokala eller monterade mappar tar den lägre gränsen vid sitt nästa kopieringssteg.
- **Kall lagring (S3).** Ett S3-mål kan använda en billigare **Lagringsklass** som restic fortfarande kan läsa direkt, till exempel Standard-IA, One Zone-IA, Intelligent-Tiering eller Glacier Instant Retrieval. Glacier Flexible Retrieval och Deep Archive är uteslutna, eftersom restic inte kan återställa från dem. En rclone-remote ställer in sin klass i rclone-konfigurationen.
- **Samtidiga anslutningar.** På ett S3-, B2-, Azure-, Google Cloud-, Swift-, rest-server-, SFTP- eller rclone-mål anger **Samtidiga anslutningar** på kortet **Anslutning** hur många anslutningar restic öppnar på en gång, från 1 till 32. Lämnas fältet tomt behåller restic sin standard på 5. Fler kan snabba upp en avlägsen eller långsam förbindelse, färre är snällare mot en leverantör som stryper. Säkerhetskopior, kopior, kontroller, rensning och återställningar på målet använder alla värdet, och en inställningsexport tar med det. En lokal eller monterad mapp har ingen sådan inställning.
- **Tillväxtbudget.** **Tillväxtbudget (GB, 0 = av)** i ett avsnitts kopieringsfönster slår larm när repositoriet växer förbi den storleken. Den raderar aldrig något, så den passar en append-only-kopia som inget rensar härifrån.
- Medan en kopiering körs säger översiktens statushuvud **Extern kopiering pågår**. Det är en aktivitetsindikator, inte en procentstapel, eftersom `restic copy` inte exponerar något maskinläsbart förlopp.

!!! note "Återställ från vilken plats som helst"
    Varje post listar sina säkerhetskopior som en tidslinje över alla platser där en säkerhetskopia ligger, på sin flik **Säkerhetskopior**. En säkerhetskopia som kopierats till B2 visas en gång, märkt med varje plats som har den. En återställning tar den första platsen den når, med början i repositoriet som posten skrivs till, och du kan välja en annan plats per ögonblicksbild. Externa lagringsmål läses först när du klickar på **Kontrollera**, eftersom det tar en stund att lista molnlagring. Att radera på ett ställe kontrollerar först de andra och säger om det var den sista kopian. Se [Återställning](restore.md).

## Lagringsmål {#destinations}

Ett lagringsmål är en plats dit säkerhetskopior går: en lokal mapp, en delad resurs, en egen server eller en molntjänst. Det sätts upp en gång och betjänar sedan varje avsnitt. Öppna **Lagringsmål** från dess inställningsbricka, eller slå på **Visa även i sidofältet** där för att fästa det.

![Sidan Lagringsmål: listan över mål och flödet för vilket avsnitt som går vart.](assets/screenshots/storage/locations-dark.png#only-dark)
![Sidan Lagringsmål: listan över mål och flödet för vilket avsnitt som går vart.](assets/screenshots/storage/locations-light.png#only-light)

**Lägg till mål** öppnar ett fönster i fyra steg:

1. **Leverantör.** Varje tjänst listas med sin logotyp, med en sökning uppe till höger i fönstret, i grupper: **Lokal** för en disk eller delad resurs ansluten till den här servern, **NAS eller server** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV och andra), **S3-tjänst** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 och andra), din egen S3-server (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), **Molnlagring** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud och resten som rclone stöder) och **Från din grupp**, de mottagande servrarna i din grupp. Var och en säger hur väl den passar för säkerhetskopior: molndiskar saktar ner under många förfrågningar, så den första säkerhetskopian och rensningen tar längre tid där. För en tjänst som ingen bricka täcker tar brickan **Rclone** under **Egen rclone-konfiguration** emot remotes inklistrade från en rclone.conf.
2. **Logga in.** Fälten beror på tjänsten: en åtkomstnyckel för S3, en användare och ett lösenord för WebDAV och SMB, ett applösenord där tvåfaktorsinloggning blockerar det vanliga, BombVaults publika SSH-nyckel för SFTP och Storage Box, eller en token för tjänster som loggar in via en webbläsare. För dem visar fönstret ett `rclone authorize`-kommando att köra på en dator med webbläsare, och token det skriver ut går in i fältet. Där du redan har en [sparad inloggning](#sign-ins) för tjänsten väljer **Sparad inloggning** den i stället för **Nya nycklar**. **Testa anslutning** kontrollerar inloggningen innan något sparas. Fönstret listar sedan mapparna på lagringsmålet, med **Ny mapp** för att skapa en och det lediga utrymmet där tjänsten rapporterar det. En tom mapp är tryggast. BombVault lägger en mapp per avsnitt i den.
3. **Skydd.** Fönstret säger rakt ut vad tjänsten kan. En rest-server i append-only-läge nekar radering, och manipuleringstestet kontrollerar det. En S3-bucket kan behålla gamla versioner med versionshantering och objektlås, vilket BombVault ännu inte kan kontrollera. En molndisk kan inte neka radering alls: den som tar sig in på servern tar sig in i den kopian också. Slå på **Skydd mot radering (append-only)** bara där andra sidan verkligen nekar radering. BombVault rensar då aldrig där. **För nödläge** säger vad återställningskitet innehåller för det här målet.
4. **För vilka avsnitt.** Välj de avsnitt som säkerhetskopierar hit. Den första kopian tar hela historiken med sig. Poster som valt egna lagringsmål får inte det nya av sig själva, och fönstret nämner dem.

![Lägg till mål: det första steget, med varje leverantör som en bricka.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Lägg till mål: det första steget, med varje leverantör som en bricka.](assets/screenshots/storage/add-providers-light.png#only-light)

![Lägg till mål: inloggningssteget efter ett lyckat test.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Lägg till mål: inloggningssteget efter ett lyckat test.](assets/screenshots/storage/add-signin-light.png#only-light)

Vissa tjänster ber om lite mer vid inloggningen:

- **SFTP och Storage Box.** **Lägg till nyckel** lägger BombVaults publika nyckel på servern med **Kontots lösenord**, som används för just den inloggningen och inte sparas någonstans. Kommandot för att göra det för hand visas under det. Testet visar **Servernyckel** med sitt fingeravtryck. BombVault kommer ihåg den och nekar servern om den senare visar en annan.
- **S3-nycklar för en enda bucket**, till exempel en Cloudflare R2-objekttoken eller en avgränsad AWS-nyckel, kanske inte kan lista de andra buckets. Skriv bucketen i **Bucket**, så öppnar testet och mapplistan just den bucketen.
- **S3-regioner och adresser.** Regionen läses från en slutpunkt som nämner den (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi), Garage börjar med sin egen standardregion, och IDrive e2 ber om slutpunkten som dess instrumentpanel visar. En adress inklistrad med bucketen på, som R2-instrumentpanelen visar den, fyller i servern och bucketen. En bucket som skapas från mappbläddraren hamnar i regionen formuläret nämner.
- **Lagringsklass.** Väljaren erbjuder bara de klasser tjänsten tar emot, och **Fler inställningar** visar bara de alternativ som formar en ny bucket.
- **Google Drive och OneDrive.** Rensad data raderas för gott i stället för att hamna i papperskorgen, där den fortfarande skulle räknas mot kvoten. En delad Google-disk går under **Fler inställningar**.
- **SMB-servrar som inte listar sina resurser** tar namnet på en under **Utdelning**, och testet öppnar den resursen direkt.
- **FTP** ber om sin **Kryptering**: **Ingen**, **Explicit TLS (FTPES)** eller **Implicit TLS (FTPS, port 990)**.
- **OneDrive och Zoho WorkDrive.** När en inloggning öppnar flera diskar eller arbetsytor frågar fönstret vilken säkerhetskopiorna går till.
- **En rest-server med ett självsignerat certifikat.** Lägg certifikatet som en PEM-fil i mappen `certs` i BombVaults datamapp (`/config/certs` i containern, `appdata/bombvault/certs` på Unraid). Säkerhetskopior, kopior och testet av skydd mot radering litar på det utan omstart.

S3-tjänster körs genom restics egen S3-backend, vilket är det som låter en lagringsklass och objektlås gälla. En rest-server nås av restic direkt. Varje annan tjänst körs genom den rclone som BombVault levererar, och dess del av rclone-konfigurationen visas i kortet **rclone.conf** på målets sida, med hemligheterna dolda. En inställningsexport bär lagringsmålen, och med inloggningsuppgifter inkluderade bär den deras inloggningar också.

Att spara ett lagringsmål som redan innehåller säkerhetskopior av containrar, VM:ar eller mappar, till exempel ett du tog bort och lägger till igen, lägger till det i de avsnitten som en obockad kopia. Ingenting kopieras dit förrän du bockar för det, men **Återställ** erbjuder de gamla säkerhetskopiorna direkt.

Mappen på den här servern heter **Lokal** överallt: på lagringssidorna, i flödet, på en post och på Återställning. Med flera mappar på den här servern går var och en under sitt eget namn. Ett lagringsmål som ett avsnitts off-site-fält sätter upp får namn efter sin plats, den sista mappen i adressen eller, utan en, sin server, och får ett nummer när ett annat mål redan går under det namnet. Varje mål kan döpas om i fältet **Namn** i sitt kort **Anslutning**, också en bucket-mapp som flera avsnitt delar och kopian som ett avsnitts off-site-fält beskriver, och ett namn du ger det blir kvar. Där två mål med samma namn står bredvid varandra bär var och en sin server eller mapp.

### Ett lagringsmåls sida {#location-page}

Varje lagringsmål har en egen sida. Den visar bara de kort som gäller för det målet, i den här ordningen:

![Ett lagringsmåls sida: Egna mappar, Anslutning och Autentiseringsuppgifter.](assets/screenshots/storage/location-page-dark.png#only-dark)
![Ett lagringsmåls sida: Egna mappar, Anslutning och Autentiseringsuppgifter.](assets/screenshots/storage/location-page-light.png#only-light)

- **Egna mappar**: de avsnitt som skriver sina säkerhetskopior hit först. Det här målets inloggning, skydd och gränser gäller för dem. **Använd som egen mapp** flyttar egna mappar för fler avsnitt hit, och **Ändra** flyttar bort en igen. Se [Ett avsnitts egna mapp](#own-folder).
- **Säkerhetskopieras direkt hit**: varje avsnitt vars poster säkerhetskopierar rakt till den här platsen i stället för till sin egna mapp, och hur många poster som gör det. Ett sådant direkt repositorium tar alla sina inställningar från det här målet. Ett som inget säkerhetskopierar till längre säger det, och **Ta bort** tar bort det från listan. Säkerhetskopiorna på dess plats blir kvar. Se [Direkta repositorier](#direct-repositories).
- **Anslutning**: **Namn**, omkopplaren **Tillgängligt** (av betyder att BombVault inte längre skriver hit, och poster som säkerhetskopierar hit misslyckas med ett tydligt meddelande i stället för att hamna någon annanstans), mappen, **Komprimering**, **Lagringsklass**, **Samtidiga anslutningar** och **Utanför lokalen**. På målet **Lokal** listar det mappen för varje avsnitt under **Egna mappar här**. **Ändra anslutning** testar och sparar en ny adress eller inloggning. När andra platser loggar in med samma sparade inloggning säger fönstret det, och en ändring här ger det här målet en egen inloggning i stället för att skriva om den delade.
- **Autentiseringsuppgifter**: vilken sparad inloggning det använder (**Loggar in med**), vilka andra mål som använder den också (**Används också av**), och **Redigera**, som öppnar inloggningsfönstret. Se [Inloggningar](#sign-ins).
- **rclone.conf**: det här målets del av rclone-konfigurationen, för ett mål som körs genom rclone.
- **Skydd mot radering**, med **Skydd mot radering (append-only)**, **Acceptera utan skydd mot radering**, **Testa skyddet mot radering nu** och **Vem rensar**, som säger om BombVault rensar kopian och de egna mapparna här, enligt det här målets kort **Hur länge det bevaras** eller ett avsnitts egna värden under **Regler per avsnitt**, eller lämnar det åt andra sidan. En lokal mapp visar det här kortet bara när den har ett skydd som du kan slå av. Se [Skydd mot radering](#delete-protection-append-only).
- **Kopiera**: **När det kopieras**, och **Uppladdningsgräns (KiB/s)** och **Nedladdningsgräns (KiB/s)**. Ett mål som bara håller egna mappar och inga kopior visar gränserna som ett kort som heter **Bandbredd**.
- **Hur länge det bevaras**: **Kort**, **Balanserad**, **Lång** eller **Egna värden**, med en tidslinje över vilka säkerhetskopior regeln behåller. Se [Hur länge det bevaras](#keep).
- **Ligger redan under det här målet**: avsnitt vars handskrivna adress ligger i en mapp i det här målet. Att ta över ett lämnar repositoriet där det är. Inloggning, lagringsklass och skydd mot radering kommer då från målet. BombVault kontrollerar först att målets inloggning öppnar repositoriet, och den nekar att lägga ett append-only-repositorium under ett mål som inte är append-only.
- **Regler per avsnitt**: ett avsnitts eget repositorium, egen gallring eller komprimering på det här målet. **Ändra regler** öppnar dem, och **Tillbaka till målet** tar tillbaka ett avsnitt. Inloggning, bandbredd och när kopior körs kommer alltid från målet.
- Ett kärl som visar hur fullt målet är, med märket där det är fullt och hur många veckor som återstår vid tillväxten de senaste veckorna. Ett mål som inte rapporterar sin storlek säger **Storlek okänd**.

**Flytta** och **Ta bort mål** står bland sidans åtgärder, beskrivna under [Flytta och ta bort](#move-remove).

Ett lagringsmål på distans som höll säkerhetskopior och plötsligt inte håller något repositorium, till exempel en rest-server vars disk inte monterades, får inget nytt, tomt av sig självt. Dess sida säger det så fort en säkerhetskopiering eller kopiering hittar repositoriet borta. Montera disken igen. Om du tömde det gamla repositoriet med avsikt börjar **Skapa ett nytt arkiv här** om, för varje avsnitt i målet och dess repositorier på en gång. Skyddet gäller ett lagringsmål som har stått kvar på sin plats. Ett lagringsmål du lägger till, eller ett avsnitt du bockar för vid ett mål, börjar från början: finns det inget repositorium på den adressen skapar den första kopieringen ett, även där ett mål som togs bort tidigare hade ett.

Två avslag säger vad som behöver rättas, i kopieringens körning och på målets sida. **Den här bucketen finns inte på servern** kommer så fort en S3-server säger det, och BombVault skapar ingenting där, så att ett felskrivet bucketnamn syns. En server som inte svarar alls är ett annat fall: restic fortsätter att försöka i ungefär 15 minuter per anrop, ifall den kommer tillbaka. **Här finns redan ett restic-arkiv, och den här instansens nyckel öppnar det inte** betyder att säkerhetskopiorna där hör till en annan `APP_KEY`.

En mottagande server som en annan instans i din grupp kör visas bland leverantörerna som **Från din grupp**. Se [Mottagande server](#receiving-server).

BombVault kontrollerar sina lagringsmål vid varje start. Den rättar till det som inte kräver något beslut av dig, till exempel en länk till en kopia eller ett repositorium som är borta, en kopia på ett avsnitts egna mapp eller ett direkt repositorium vars kopia är borta, och skriver en rad i loggen för varje rättelse. Det som saknar en säker åtgärd loggas bara.

![Samma sida på en telefon.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![Samma sida på en telefon.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Hur länge det bevaras {#keep}

Hur länge säkerhetskopior bevaras ställs in per lagringsmål, i dess kort **Hur länge det bevaras**. Regeln täcker kopiorna på det målet och de egna mappar som ligger där, målet **Lokal** inkluderat. Ett avsnitt kan behålla egna värden på ett mål under **Regler per avsnitt**, och de gäller före målets kort för det avsnittet.

En regel behåller en säkerhetskopia per dag, vecka, månad och år, så långt tillbaka du ställer in, och rensar ut resten efter varje säkerhetskopiering eller kopiering:

| Förinställning | Senaste säkerhetskopior | Dagar | Veckor | Månader | År |
|---|---|---|---|---|---|
| **Kort** | 0 | 7 | 4 | 3 | 0 |
| **Balanserad** | 0 | 7 | 4 | 6 | 2 |
| **Lång** | 0 | 14 | 8 | 12 | 3 |

**Egna värden** ställer in de fem antalen själv. Ett 0 slår av det antalet, och en regel med alla antal på 0 behåller allt. Prickarna i kortet visar vilka av 40 exempelsäkerhetskopior en regel behåller, den nyaste längst till höger. Exemplet är detsamma för varje mål, eftersom bilden ritas innan BombVault har läst säkerhetskopiorna där. Det räknar bakåt från i dag, så hur många prickar som blir kvar ändras med datumet.

En mapp på den här värden och en fjärrmapp som skrivits in för hand har också ett eget kort, som gäller de egna mapparna i just den mappen och inga andra. Tills du sparar det följer en sådan mapp den regel BombVault redan hade för den.

**Fråga innan mindre bevaras**, på som standard, frågar innan en ändring som behåller färre säkerhetskopior sparas, och säger hur många av de 40 exempelsäkerhetskopiorna den nya regeln tar bort. Den gäller för varje lagringsmål. **Tillbaka till målet** under **Regler per avsnitt** ställer samma fråga.

Ett mål med skydd mot radering rensas aldrig från den här lådan. Dess andra sida behåller eller släpper gamla säkerhetskopior enligt sina egna regler. Medan en post har en öppen kritisk avvikelse om förlorad data behåller gallringen den postens gamla säkerhetskopior tills du kvitterar den. Se [Avvikelser](anomalies.md).

En säkerhetskopia som innehåller mindre än den borde tränger aldrig ut dem som kom före den. Det är en säkerhetskopia som inte läser några filer där tidigare säkerhetskopior hade några, som saknar en vald mapp eller som inte kunde läsa vissa filer. Dess körning slutar med en varning som nämner vad som saknas och säger att de äldre säkerhetskopiorna behölls, du får samma meddelande i dina aviseringskanaler, och ingen gallring, i postens egna mapp eller i dess kopior, rör postens gamla säkerhetskopior förrän en fullständig säkerhetskopia har kommit igenom. En första säkerhetskopia, och en vald mapp som alltid har varit tom, räknas som fullständiga.

För att se vad nästa körning verkligen skulle radera öppnar du **Förhandsgranska** under **Inställningar, Lagringsmål**. Den listar **Vad nästa körning skulle ta bort** för avsnittets egna mapp och för var och en av dess kopior, per repositorium och per post med återställningspunkterna nämnda. Den tar inget lås och ändrar ingenting.

![Hur länge det bevaras med Balanserad vald: prickarna visar vilka av de 40 exempelsäkerhetskopiorna som stannar.](assets/screenshots/storage/keep-dark.png#only-dark)
![Hur länge det bevaras med Balanserad vald: prickarna visar vilka av de 40 exempelsäkerhetskopiorna som stannar.](assets/screenshots/storage/keep-light.png#only-light)

#### Exempel: vad Balanserad behåller {#example-retention}

En container har säkerhetskopierats varje natt kl. 03:00 sedan 1 oktober 2024, så onsdagen den 4 november 2026 har den 765 säkerhetskopior. Dess lagringsmål är inställt på **Balanserad**: 7 dagar, 4 veckor, 6 månader och 2 år.

Varje antal behåller den nyaste säkerhetskopian för sin dag, vecka, månad eller år, och en säkerhetskopia som flera antal väljer behålls en gång. Veckor löper från måndag till söndag:

| Regel | Säkerhetskopior den väljer | Nya den lägger till |
|---|---|---|
| 7 dagar | 4 nov, 3 nov, 2 nov, 1 nov, 31 okt, 30 okt, 29 okt | alla 7 |
| 4 veckor | 4 nov, 1 nov, 25 okt, 18 okt | 25 okt, 18 okt |
| 6 månader | 4 nov, 31 okt, 30 sep, 31 aug, 31 jul, 30 jun | 30 sep, 31 aug, 31 jul, 30 jun |
| 2 år | 4 nov, 31 dec 2025 | 31 dec 2025 |

Det lämnar 14 säkerhetskopior av 765. Den äldsta är från 31 december 2025, så i dag kan du gå tillbaka lite drygt tio månader. Tack vare deduplicering tar de knappt mer plats än en full säkerhetskopia plus det som ändrats emellan.

**År** förtjänar en andra titt. Det räknar kalenderår med en säkerhetskopia i, med början i det innevarande, och årets nyaste säkerhetskopia är dagens. Så det första året lägger inte till något till de andra antalen, och det andra lägger till den sista säkerhetskopian från föregående år. Det är därför en regels räckvidd varierar under året. Samma säkerhetskopior under varje förinställning:

| Förinställning | Säkerhetskopior som behålls i dag | Äldsta säkerhetskopia i dag | Räckvidd under året |
|---|---|---|---|
| **Kort** | 10 | 30 september 2026 | 1 till 2 månader |
| **Balanserad** | 14 | 31 december 2025 | ungefär 4 månader tidigt på våren, ett helt år den 31 december |
| **Lång** | 30 | 31 december 2024 | 1 år den 1 januari, 2 år den 31 december |

Att gå från **Balanserad** till **Kort** frågar först, på grund av **Fråga innan mindre bevaras**. Frågan säger hur många av de 40 säkerhetskopiorna i bilden den nya regeln tar bort. Talet kommer från exemplet, så det ändras med datumet och räknar inte dina egna säkerhetskopior. Av säkerhetskopiorna i det här exemplet skulle 31 augusti, 31 juli, 30 juni och 31 december 2025 försvinna. **Spara ändå** tillämpar regeln, och nästa rensning raderar dem från det målet för gott.

För att behålla containrarna längre än resten på det här målet trycker du på **Ändra regler** under **Regler per avsnitt**, väljer **Containers** och sätter **Egna värden** i **Regler för Containers**, till exempel 14 dagar, 8 veckor, 12 månader och 2 år. De andra avsnitten fortsätter att följa målets kort, och **Tillbaka till målet** tar tillbaka Containers.

### Inloggningar {#sign-ins}

En inloggning är nyckeln eller lösenordet BombVault använder för en S3-tjänst eller en rest-server. Inloggningar är en enda lista, kortet **Inloggningar** under **Inställningar, Lagringsmål**, och varje rad säger vilka lagringsmål som använder den (**Används av**).

- **Sparad när du lägger till ett mål.** Lägg till mål sparar nycklarna du skriver som en inloggning. Nästa mål hos samma tjänst kan välja den under **Sparad inloggning** i stället för **Nya nycklar**, och en ändring av den senare gäller för varje mål som använder den.
- **Standarden.** En stjärna markerar standardinloggningen. Den används överallt där ingen inloggning är vald, till exempel en adress skriven för hand eller en återställning på en ny installation. En hämtningskälla lånar den inte. **Gör till standard** ger stjärnan till en annan.
- **Redigera och Testa.** **Redigera**, här eller i kortet **Autentiseringsuppgifter** för ett mål, öppnar ett fönster för inloggningen. **Testa** loggar in på varje mål som använder den med det du skrivit, innan något sparas. **Spara** lagrar den.
- **Ta bort.** En inloggning kan tas bort bara när inget lagringsmål, ingen hämtningskälla och inget mottaget repositorium använder den och inga gamla säkerhetskopior behöver den, och standarden går bara som den sista. Gamla säkerhetskopior behöver den när en egen mapp flyttade bort från en plats som loggar in med den och lämnade dem kvar där. BombVault säger då vilken plats det är.
- **Att ändra ett måls anslutning** skriver aldrig om en inloggning som andra platser delar. Målet får en egen inloggning i stället, och fönstret säger i förväg att dess inloggning är delad. För att ändra den delade överallt redigerar du den i kortet **Inloggningar**.

![Kortet Inloggningar: varje sparad inloggning med vad som använder den, och stjärnan på standarden.](assets/screenshots/storage/signins-dark.png#only-dark)
![Kortet Inloggningar: varje sparad inloggning med vad som använder den, och stjärnan på standarden.](assets/screenshots/storage/signins-light.png#only-light)

En mapp på den här servern behöver ingen inloggning. SMB, WebDAV, SFTP, Storage Box och molndiskarna håller sina i rclone-konfigurationen, som målets kort **rclone.conf** visar, så de syns inte i den här listan.

### SMB, WebDAV och andra rclone-remotes {#smb-webdav}

En Windows- eller Samba-resurs och en WebDAV-server (Nextcloud, ownCloud, SharePoint eller vilken annan som helst) är leverantörsbrickor i **Lägg till mål**, under **NAS eller server**. Fyll i värd och resurs (SMB) eller adress och servertyp (WebDAV), användaren och lösenordet, så skriver BombVault rclone-avsnittet åt dig. rclone gör lösenordet oläsligt innan det lagras.

För en tjänst som ingen bricka täcker tar brickan **Rclone** under **Egen rclone-konfiguration** emot remotes inklistrade från en rclone.conf. De läggs till de remotes BombVault redan har och ersätter dem aldrig, och de lagras krypterade. För att säkerhetskopiera ett avsnitt till en av dem anger du `rclone:<remote>:<folder>` som det avsnittets off-site-repositorium, till exempel `rclone:nas:backups/bombvault`. Resursen eller bucketen är det första sökvägssegmentet, inte en del av namnet.

Det här är den bättre vägen än att montera resursen på Unraid: restic avråder från att hålla ett repositorium på en monterad CIFS-resurs, och här monteras ingenting. NFS har ingen bricka eftersom varken restic eller rclone har en NFS-backend. För NFS monterar du exporten på värden och använder den som en lokal mapp.

### Flytta och ta bort {#move-remove}

- **Flytta** tar kopian av ett avsnitt till ett annat lagringsmål, med hela dess historik. Allt blir kvar på den gamla platsen. BombVault raderar ingenting där. Flytten körs i bakgrunden och kan ta lång tid: ungefär 9 minuter per 100 GB mellan lokala diskar, timmar över en långsam länk. Översikten visar den medan den körs.
- **Ta bort mål** erbjuder två sätt. **Ta bort mål, behåll säkerhetskopiorna** slutar skriva dit och lämnar säkerhetskopiorna på plats. Du kan läsa dem senare under **Återställning, Från en gammal säkerhetskopiemapp**. **Ta bort målet och radera säkerhetskopiorna där** läser först vad som är lagrat där och listar det: vilka säkerhetskopior som går, vilka av dem som inte finns någon annanstans, och vilka repositorier som blir kvar eftersom skydd mot radering nekar, poster fortfarande säkerhetskopierar rakt till dem, ett annat mål eller avsnitt behåller sina kopior där, eller de inte gick att läsa. Poster som skulle förlora sin enda säkerhetskopia listas separat och behålls om du inte slår på **Radera dessa också**. Du bekräftar genom att skriva målets namn, och raderingen körs i bakgrunden.
- Det här fungerar för alla slags lagringsmål: ett som flera avsnitt använder, ett mål som sattes upp för ett avsnitt, ett repositorium du satt upp och mappen ett avsnitt skriver sina egna säkerhetskopior till. Poster som säkerhetskopierar till ett borttaget repositorium följer sitt avsnitt igen, och det gör också en avsnittsstandard som pekade dit. Ett avsnitts mapp kan inte bara försvinna, eftersom varje avsnitt behöver en. Fönstret frågar efter mappen avsnitten flyttar till, och varje avsnitt behåller sitt mappnamn under den.
- Att radera säkerhetskopiorna på ett lagringsmål fungerar också utan inloggningslösenord. När ett lösenord är satt kräver det en inloggning på själva instansen.

### Exempel: en off-site-kopia på Backblaze B2 {#example-s3}

Containrarna och auto-säkerhetskopian ska också ligga hos Backblaze B2, så att en död server inte tar dem med sig. Samma steg passar Wasabi, Hetzner Object Storage och de flesta andra S3-tjänster.

1. **I B2** skapar du en privat bucket, till exempel `tower-backups`, i en region nära dig. Under **Application Keys** skapar du en nyckel med läs- och skrivåtkomst, begränsad till den bucketen, och noterar dess nyckel-ID och applikationsnyckel. B2 visar applikationsnyckeln en gång. Ställ in bucketens livscykel på att bara behålla den senaste versionen, annars behåller B2 rensad data som dolda versioner och fakturerar dig för dem.
2. **Leverantör.** Öppna **Lagringsmål** i BombVault, tryck på **Lägg till mål** och välj **Backblaze B2** under **S3-tjänst**.
3. **Logga in.** Ange nyckel-ID som **Åtkomstnyckel-ID** och applikationsnyckeln som **Hemlig åtkomstnyckel**. Ange bucketens slutpunkt under **Slutpunkt**, till exempel `s3.eu-central-003.backblazeb2.com`, så läser BombVault regionen från den. Skriv `tower-backups` i **Bucket**, eftersom den här nyckeln bara öppnar en bucket.
4. **Testa anslutning.** Den ska säga **Anslutningen är upprättad** och lista bucketens mappar. Tryck på **Ny mapp** och döp den till `bombvault`. BombVault lägger en mapp per avsnitt i den, till exempel `bombvault/containers`.
5. **Skydd.** Fönstret förklarar att B2 kan behålla gamla versioner med objektlås och att BombVault inte kan kontrollera detta. Låt **Skydd mot radering (append-only)** vara av om inte bucketen verkligen nekar radering, så att BombVault kan rensa enligt målets behållregler.
6. **För vilka avsnitt.** Bocka för **Containers** och **Auto-säkerhetskopia**. Auto-säkerhetskopian bär dina inställningar, så att en ny installation kan få tillbaka dem. Fönstret säger hur många poster och, där det är känt, hur mycket historik den första kopian tar med sig.
7. Tryck på **Spara mål**.

Den första kopian körs efter nästa säkerhetskopiering. För att starta den nu öppnar du **Inställningar, Lagringsmål** och trycker på **Replikera nu** på avsnittets rad i **Externa kopior**. Statushuvudet säger **Extern kopiering pågår**. Den första kopian tar med hela historiken, så 200 GB över en 100 Mbit/s-uppkoppling tar ungefär fyra och en halv timme. Senare kopior skickar bara det som är nytt.

För att hålla din linje ledig öppnar du målets sida och sätter **Uppladdningsgräns (KiB/s)** i kortet **Kopiera**, till exempel till `5120`, som läses som ungefär 5,0 MB/s (41,9 Mbit/s). Kontrollera **Hur länge det bevaras** på samma sida medan du är där.

För att se att det fungerade tittar du på **Skyddsstatus** på översikten, där **Extern** blir **kopian är aktuell**, och på en containers flik **Säkerhetskopior**, där dess ögonblicksbilder nu nämner `tower-backups` som en andra plats. Tryck sedan på **Ladda ner återställningskit** igen under **Inställningar, Återställning**, eftersom kitet listar det nya målet och dess inloggning.

**Cloudflare R2 fungerar på samma sätt,** med en objekttoken i stället för en nyckel. Klistra in S3-adressen som R2-instrumentpanelen visar, med bucketen på, så fyller BombVault i servern och bucketen.

### Exempel: en rest-server som du kör själv {#example-rest-server}

En rest-server i append-only-läge nekar radering, så en kopia på den överlever även om någon tar över din server. En andra låda, hos en vän eller i garaget, räcker. En parkopplad BombVault kan sätta upp detta åt dig, se [Mottagande server](#receiving-server). För att köra den för hand på vilken Docker-värd som helst:

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

Den sista raden frågar efter ett lösenord och skriver användaren till filen `.htpasswd` i datamappen.

1. Tryck på **Lägg till mål** i BombVault och välj **rest-server** under **NAS eller server**.
2. Ange adressen med användaren som mapp, till exempel `http://192.168.2.20:8000/bombvault`, och **Användarnamn** och **Lösenord**. Tryck på **Testa anslutning**.
3. Slå på **Skydd mot radering (append-only)** under **Skydd**. BombVault rensar då aldrig där och lämnar det åt andra sidan.
4. Bocka för avsnitten och tryck på **Spara mål**.
5. När den första kopian finns öppnar du målets sida och trycker på **Testa skyddet mot radering nu**. BombVault försöker radera ett testobjekt. Kortet ska säga **Senast: radering nekades**, och **Skydd mot radering** på översikten blir **kontrollerat**. Om det säger **Senast: radering nekades inte** saknas `--append-only`.

Eftersom inget rensar den här kopian härifrån sätter du **Tillväxtbudget (GB, 0 = av)** i avsnittets fönster, som öppnas med **Redigera** på dess rad i **Externa kopior**, så att du får veta det innan disken blir full. Med ett självsignerat certifikat, se anmärkningen om `certs` ovan.

## Ett avsnitts egna mapp {#own-folder}

Varje avsnitt har en egen mapp: repositoriet dess säkerhetskopior skrivs till först. Kopior till andra lagringsmål går ut därifrån. En ny installation håller de egna mapparna på den här servern, under `/mnt/user/bombvault`, och tillsammans bildar de lagringsmålet **Lokal**.

En egen mapp kan också ligga i ett lagringsmål du redan har, till exempel en NAS, en rest-server eller en S3-bucket. Avsnittet säkerhetskopierar då dit direkt, utan en lokal kopia emellan, och den egna mappen följer det målets inloggning, skydd mot radering och bandbreddsgränser. BombVault rensar den efter varje säkerhetskopiering enligt målets kort **Hur länge det bevaras**, och ett avsnitts egna värden under **Regler per avsnitt** går före kortet. Målet listar avsnittet i sitt kort **Egna mappar**, och **Mål per avsnitt** visar målet som avsnittets hem.

### Flytta en egen mapp {#move-own-folder}

1. Öppna lagringsmålet som ska hålla den egna mappen och välj **Använd som egen mapp**, eller **Lägg till sektioner** i dess kort **Egna mappar**. För att flytta flera avsnitt på en gång väljer du **Flytta egna mappar** bredvid **Egna mappar här** i kortet **Anslutning** för målet Lokal.
2. Välj avsnitten under **Avsnitt** och platsen under **Flytta till**. **En annan plats** tar en mapp på den här värden eller adressen till ett repositorium på distans skriven för hand, med en inloggning bland dina sparade inloggningar.
3. Läs **Vad som händer**. För varje avsnitt säger den vart nya säkerhetskopior går, om en kopia som redan finns på den nya platsen blir den egna mappen, och vilka säkerhetskopior som kopieras över först.
4. **Kopiera de säkerhetskopior som redan finns** är på som standard. Avslaget blir de gamla säkerhetskopiorna där de är, men posten listar dem inte längre. **Från en gammal säkerhetskopiemapp** på sidan Återställning läser dem fortfarande med mappens adress och, för en fjärrmapp, dess inloggning.
5. Välj **Byt**. När den nya platsens kort **Hur länge det bevaras** behåller mindre än avsnittets regel i dag frågar BombVault först **Bevara mindre?** Där tar **Behåll de nuvarande värdena för** det avsnittet **under Regler per avsnitt** med dagens värden, **Spara ändå** går vidare, och **Tillbaka** lämnar allt som det var. När säkerhetskopior måste kopieras först kopierar BombVault dem och byter sedan den egna mappen. När de redan finns där kontrollerar den dem före bytet.

Efter flytten går nya säkerhetskopior till den flyttade egna mappen, även för ett avsnitt som hade **Lokal** avslaget och säkerhetskopierade rakt till en kopia. En kopia som redan låg på den nya platsen blir den egna mappen, och kopieringen dit upphör. Där den kopian behållit några säkerhetskopior längre än avsnittets behållregler behåller avsnittet det som någon av reglerna behåller, så ingen av dem går förlorad.

När flytten inte kan gå vidare säger fönstret varför i en ruta under **Vad som händer**. När det bara är kopieringen av de gamla säkerhetskopiorna som är i vägen låter avslaget av **Kopiera de säkerhetskopior som redan finns** den egna mappen flytta utan dem.

Ett avsnitt behöver alltid en egen mapp. Att ta bort lagringsmålet som håller den frågar efter mappen avsnittet flyttar till, se [Flytta och ta bort](#move-remove). En egen mapp som nya säkerhetskopior går förbi, eftersom avsnittet säkerhetskopierar rakt till en kopia, markeras **(av)** där ett mål listar vem som använder den.

![Fönstret som flyttar en egen mapp, med Vad som händer ifyllt för ett avsnitt.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![Fönstret som flyttar en egen mapp, med Vad som händer ifyllt för ett avsnitt.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Exempel: Containers flyttar till en NAS {#example-own-folder}

Cache-poolen håller på att bli full, och du har ett lagringsmål som heter `nas`, en SMB-resurs på en andra låda. Avsnittet Containers har 61 säkerhetskopior i sin egna mapp på den här servern och kopierar till Backblaze B2.

1. Öppna **Lagringsmål**, sedan `nas`, och tryck på **Använd som egen mapp**.
2. Bocka för **Containers** under **Avsnitt**.
3. Läs **Vad som händer**. Den säger att nya säkerhetskopior av Containers går till mappen på `nas`, att de 61 säkerhetskopiorna i Lokal kopieras dit först, att kopior fortsätter gå till Backblaze B2 och att skydd, bandbreddsgränser och tillväxtlarmet kommer från `nas`.
4. Låt **Kopiera de säkerhetskopior som redan finns** vara på och tryck på **Byt**.

BombVault kopierar säkerhetskopiorna i bakgrunden och byter sedan den egna mappen. Sidan `nas` listar Containers under **Egna mappar**, en post som följer sitt avsnitt nämner `nas` som sitt hem, och nästa säkerhetskopia hamnar där, med kopian till B2 gående ut från `nas`. Från och med nu är det kortet **Hur länge det bevaras** på `nas` som rensar mappen. Den gamla mappen på den här servern blir kvar där den är, och BombVault raderar ingenting där.

### Skriva en fjärradress för hand {#remote-primary-repositories}

Kortet **Anslutning** för målet Lokal tar fortfarande en fjärradress för ett avsnitts egna mapp: **Ändra** bredvid avsnittet öppnar ett fönster med en omkopplare **Lokal / Fjärran**. **Fjärran** tar en restic-adress som `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` eller `rclone:remote:bucket/path`, och **Säkerhetsinställningar för fjärransluten primärrepo** lägger till ett anslutningstest, upp- och nedladdningsgränser, append-only-skydd med samma manipuleringstest som ett externt mål får, och ett larm för tillväxtbudget.

En sådan adress loggar in med standardinloggningen. När den är exakt den mapp som ett lagringsmål du har håller för avsnittet följer den det målet av sig själv, målets kort och dess omkopplare **Tillgängligt** inräknade, om den inte loggar in på ett annat sätt eller är append-only där målet inte är det. En regel som gallrade mappen fram till dess blir kvar som avsnittets egen under **Regler per avsnitt**. När den ligger i en annan mapp i det målet listar målets sida avsnittet under **Ligger redan under det här målet**, och att ta över det ger det målets inloggning, lagringsklass och skydd mot radering. Att flytta den egna mappen in i ett lagringsmål med **Använd som egen mapp** är det enklare sättet, eftersom fönstret kontrollerar de säkerhetskopior som redan finns innan något ändras.

## Placering per post {#placement}

Kortet **Schema och mål** på en posts flik Inställningar har en rad knappar: **Lokal** och en knapp per externt lagringsmål för avsnittet. Tända knappar får postens säkerhetskopior.

- Med **Lokal** tänd skrivs posten till repositoriet som visas under **Sparad på** och kopieras till varje annat tänt mål. Släck ett mål så får det inget nytt från den här posten. Enbart Lokal kopierar ingenstans, vilket passar data som redan har en andra kopia, till exempel en delad resurs som bor på en NAS.
- Med **Lokal** släckt skrivs posten rakt till det första tända målets direkta repositorium och kopieras därifrån till de andra tända. Första gången skapar en dialog det direkta repositoriet.
- En knapp förblir tänd, eftersom en säkerhetskopia behöver en plats att gå till. För att lämna en post utanför säkerhetskopior använder du **Säkerhetskopiera inte**.

BombVault flyttar aldrig säkerhetskopior mellan repositorier. En post som följer sitt avsnitt följer med när avsnittets standard ändras: dess nästa säkerhetskopia hamnar på den nya platsen, och de säkerhetskopior den har blir kvar där de är. Ett mål som valts på själva posten förblir fast från dess första säkerhetskopia. Kortet **Repository** på fliken Inställningar nämner det. Kopiorna kan ändras när som helst. Ett mål som inte längre får en post behåller de kopior det har och trimmar dem till sin egen gallring vid avsnittets nästa off-site-körning. Från append-only-mål går det inte att radera.

Platsen en post lämnat behåller sina säkerhetskopior. På postens flik **Säkerhetskopior** och på **Återställning** står den som **Bara äldre säkerhetskopior**, en återställning kan fortfarande läsa från den, och en rensning där lämnar de säkerhetskopiorna ifred.

**Följ standarden** tar tillbaka en post med ett eget val till sitt avsnitt. Den frågar först, säger vart nya säkerhetskopior går då, och nämner vad varje mål slutar få och vad nästa körning skulle ladda upp.

Under raden säger kortet vart posten går och vad som faktiskt finns där: hur många platser som håller den, när varje mål senast sågs och om 3-2-1 är uppfyllt. En plats är servern med originaldatan, varje externt mål och varje repositorium markerat **Utanför lokalen**. BombVault kontrollerar kopior och platser. Den kontrollerar inte delen "två medier" i 3-2-1.

### Standard per avsnitt {#placement-defaults}

Kortet **Mål per avsnitt** på sidan Lagringsmål ritar standarden för varje avsnitt som ett 3-2-1-flöde. Varje linje från ett avsnitt till ett lagringsmål är en omkopplare, **Lokal** inkluderad, för varje avsnitt, även flashenheten, ZFS-datauppsättningar och auto-säkerhetskopian. Ett klick på en linje eller en punkt slår om den direkt, och streckade linjer är av. Bara att slå av ett avsnitts **Lokal** frågar först, i samma dialog som standardkortet, eftersom avsnittet då säkerhetskopierar rakt till en kopia. **Avbryt** lämnar allt som det var. **Graf / Omkopplare** visar detsamma som rader med knappar. En omkopplare avgör vart nya säkerhetskopior går. Poster utan ett eget val, och projektmapparna för Compose-stackar, följer direkt, så deras nästa säkerhetskopia går till de nya platserna. Säkerhetskopior som redan är lagrade blir kvar där de är och kan fortfarande återställas därifrån. Flash, ZFS-datauppsättningar och auto-säkerhetskopian säkerhetskopierar till sin lokala mapp och kopierar externt därifrån. Deras linje till **Lokal** kan också slås av. Deras säkerhetskopior går då rakt till deras externa mål, och en ZFS-post med en replik och inget mål alls skickar bara sin replik.

Med **Lokal** av och bara externa mål på säkerhetskopierar ett avsnitt rakt till dem, och BombVault väljer repositoriet där själv. Containrar, VM:ar och mappuppsättningar går till det direkta repositoriet för det första mål som är på, beskrivet nedan, och kopieras därifrån till de andra. Flashenheten, ZFS-datauppsättningar och auto-säkerhetskopian skriver varje säkerhetskopia till varje mål som är på, och varje mål behåller dem enligt sin egen gallring.

Ett avsnitt med varje linje av säger **Inte säkerhetskopierat**. Dess poster får inga säkerhetskopior och räknas som oskyddade.

Poster som valt egna lagringsmål på sin sida behåller dem. Flödet säger hur många det finns i ett avsnitt, till exempel **2 poster avviker**, och leder till dem: till postens flik Inställningar när det finns en, till en lista med vart var och en säkerhetskopierar när det finns flera. **Följ standarden** där tar tillbaka en post.

Ett nytt externt mål tar emot varje post som inte är inställd på Lokal. Fönstret som lägger till det säger hur många poster och, där det är känt, hur mycket historik det är.

### Direkta repositorier {#direct-repositories}

Ett direkt repositorium är där poster säkerhetskopierar rakt till ett externt mål. BombVault lägger det bredvid målets mapp med `-direct` tillagt i namnet, eller, när målet ligger i roten av sin bucket, sin remote eller en rest-server-användare, i en mapp inuti den som är uppkallad efter avsnittet, till exempel `containers-direct`. I flödet på sidan Lagringsmål skapas det utan fråga. Att slå av Lokal för en enskild post, så att ett mål utan ett direkt repositorium blir dess hem, öppnar en dialog med det föreslagna stället, till exempel `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, och ett anslutningstest som inte skapar något. **Skapa och använd** skapar repositoriet och pekar posten på det. Ett direkt repositorium har inga egna inställningar. Det följer sitt måls omkopplare, inloggning, lagringsklass, gränser, append-only-inställning och gallring, och ändras med dem. Målets sida listar det under **Säkerhetskopieras direkt hit**, med varje avsnitt som säkerhetskopierar rakt till det och hur många poster som gör det. Ett som inget säkerhetskopierar till längre säger det där och kan tas bort, och säkerhetskopiorna på dess plats blir kvar. När en ny nyckel för målet inte kan öppna det behåller det direkta repositoriet den nyckel det har och sparandet säger det. En post på ett direkt repositorium kopieras därifrån till de andra tända målen, aldrig till målet som repositoriet tillhör. Dess ögonblicksbilder bär taggen `bv:direct`, och varje annan gallringsomgång behåller dem, så ett direkt repositorium som förlorat sin länk till sitt mål åldras aldrig enligt de lokala reglerna. B2 nås via sin S3-slutpunkt, med nyckel-ID och applikationsnyckel inskrivna som S3-nycklar. En nyckel som är begränsad till målets egen mapp kan inte nå mappen bredvid, så begränsa nyckeln till mappen ovanför i stället.

### Utanför lokalen {#off-the-premises}

Ett lagringsmål eller ett repositorium kan markeras **Utanför lokalen**. Sådana på distans börjar markerade. Slå av det för en rest-server i samma byggnad. Märket räknar bara platser och 3-2-1 på korten. Det ändrar ingen kopia.

En ZFS-replik är en kopia utanför lokalen, men inte en säkerhetskopia med historik. Den behåller bara de ögonblicksbilder du ställer in, och gamla versioner och enskilda filer kommer fortfarande från säkerhetskopiorna. Den går från en ZFS-server rakt till den andra, över SSH till en ZFS-server eller till mottagarslutpunkten hos en parkopplad instans. Parkopplingsgruppen bär bara begäran. Se [Replik](zfs-datasets.md#replica).

### Efter en återuppbyggnad {#after-a-rebuild}

Kopieringsvalen finns i BombVaults egna inställningar. Efter en återuppbyggnad via **Identifiera säkerhetskopior** eller Nödguiden utan återställda inställningar är de borta, och att kopiera allt skulle skicka de poster du lämnat utanför till B2 igen. Detsamma gäller när ett avsnitt som aldrig har kopierat off-site från den här databasen hittar egna säkerhetskopior på ett lagringsmål, eller äldre i sin egna mapp, till exempel efter att ett mål lagts till ovanpå säkerhetskopiorna från en tidigare installation med samma `APP_KEY`. Off-site-kopior av det avsnittet väntar då på ditt beslut.

**Behöver uppmärksamhet** på översikten och sidan för varje lagringsmål som avsnittet kopierar till frågar om säkerhetskopiorna är den här serverns egna. **Ta över dem som den här serverns egna** bekräftar avsnittets standard, och dess kopior körs igen från nästa omgång. **Kontrollera reglerna först** leder till avsnittets rad i **Mål per avsnitt**, där du ser vart varje post går och kan ställa in en förlorad regel igen på postens sida. Kopieringen fortsätter att vänta där tills du trycker på **Ta över dem som den här serverns egna**, som först visar vad nästa körning kopierar och namnen i säkerhetskopiorna som saknar post, som du kan lämna utanför. Den nyaste säkerhetskopian av varje post som väntar säger det i sin körning, med en länk till beslutet, och en avisering går ut när aviseringar är på. Att importera en inställningsfil för tillbaka regler och standarder men fattar inte beslutet åt dig.

![Behöver uppmärksamhet efter en återuppbyggnad: mappuppsättningarna hittade säkerhetskopior från tiden före den här databasen, och deras off-site-kopior väntar på beslutet.](assets/screenshots/offsite/copy-decision-dark.png#only-dark)
![Behöver uppmärksamhet efter en återuppbyggnad: mappuppsättningarna hittade säkerhetskopior från tiden före den här databasen, och deras off-site-kopior väntar på beslutet.](assets/screenshots/offsite/copy-decision-light.png#only-light)

## Skydd mot radering (append-only) {#delete-protection-append-only}

Slå på skydd mot radering för ett externt lagringsmål så att ransomware, eller en komprometterad värd, inte kan radera eller skriva om dina säkerhetskopior. Andra sidan (en `restic/rest-server` som körs i läget `--append-only`) **upprätthåller** det. BombVault bara **verifierar** det och visar aldrig grönt på ett konfigurationspåstående ensamt. En mapp på den här servern kan inte neka radering, eftersom den som styr BombVault styr mappen också, så lokala mål erbjuder inte omkopplaren.

Omkopplaren hör till lagringsmålet: ett avsnitt som kopierar dit tar den från målet, så slå på den från målets sida under **Lagringsmål**. Den schemalagda **Kontrollera raderingsskydd** körs bara medan ett avsnitt kopierar till ett mål med omkopplaren på. Andra sidan måste startas med `--append-only`. Med imagen `restic/rest-server`, på TrueNAS som på vilken annan Docker-värd som helst, går det i variabeln `OPTIONS`.

Vissa platser kan inte neka en radering alls, till exempel en molndisk som OneDrive. Om du håller kopior där med avsikt slår du på **Acceptera utan skydd mot radering** på målets sida. Översikten visar då det saknade skyddet mot radering för ett avsnitt som bara kopierar till accepterade mål som en accepterad risk, en grå ring i stället för en gul punkt, och avsnittets färg följer dess andra kontroller. Inget annat ändras: säkerhetskopiorna där kan fortfarande raderas, BombVault rensar dem fortfarande enligt målets behållregler, och omkopplaren försvinner när skydd mot radering är på. En kopia som flyttar till en annan plats förlorar acceptansen. Att slå på skydd mot radering rensar den också.

Det snabbaste sättet till en append-only-kopia är en parkopplad BombVault med en [mottagande server](#receiving-server): den kör rest-servern åt dig, i append-only-läge.

!!! note "En lyckad radering under `/locks/` är väntad"
    Append-only betyder inte att inget någonsin kan tas bort. restic måste ta och släppa sina egna lås, så `/locks/` förblir skrivbar och raderbar med avsikt. Ögonblicksbilder och datan bakom dem, som är vad ransomware skulle gå efter, kan inte tas bort. Om du själv sonderar andra sidan är en radering som lyckas under `/locks/` rätt beteende och inte ett hål i skyddet.

!!! warning "Append-only-repositorier rensas aldrig från den här lådan"
    Ett append-only-mål rensar aldrig gamla ögonblicksbilder härifrån. Rensning är andra sidans jobb. Sätt ett **larm för tillväxtbudget** för det så att du får veta innan repositoriets storlek skenar.

## Manipuleringstest {#tamper-test}

BombVault bevisar med jämna mellanrum append-only-garantin genom att faktiskt försöka en radering mot off-site-repositoriet, riktad mot ett objekt som inte finns. **Testa skyddet mot radering nu** på ett måls sida kör det på begäran, och **Kontrollera raderingsskydd** under **Inställningar, Verifiera, Automatiska kontroller** kör det enligt ett schema.

Att slå av **Kontrollera raderingsskydd** döljer inte ett hål. Ett mål med skydd mot radering som aldrig testats läses då "inte bevisat, dess kontroll är av", och ett mål utan skydd mot radering läses "inget" i båda fallen.

En kopia som är avslagen testas inte, och målet säger det. Anslutningstestet körs ändå på den, eftersom det bara läser, så en kopia kan kontrolleras innan den slås på.

- **Nekad** betyder skyddad.
- **Accepterad** betyder inte skyddad.
- Ett **ofullständigt** resultat (servern går inte att nå, autentiseringsfel) vänder aldrig det lagrade utslaget.

En verklig vändning från skyddad till oskyddad utlöser ett enda larm.

## DR-övningar {#dr-drills}

BombVault erbjuder två nivåer av bevis för att dina säkerhetskopior verkligen går att återställa, inte bara finns.

- **Återställningskontroller (lokalt).** BombVault kör med jämna mellanrum `restic check --read-data-subset` (begränsad, aldrig en diskfyllande full återställning). Intervallet finns under **Inställningar, Verifiera, Automatiska kontroller**, och kortet **Kontrollera säkerhetskopior** på samma bricka visar resultatet.
- **Teståterställning (off-site).** BombVault återställer ett verkligt mål från off-site-repositoriet till en förseglad sandlåda, verifierar det fil för fil och byte för byte och städar sedan upp. Det bevisar att du kan återställa från off-site, inte bara att repositoriet svarar.

Kortet **Skyddsstatus** på översikten sammanfattar detta per avsnitt i fyra kontroller: en aktuell säkerhetskopia på den här servern, en kopia off-site, skydd mot radering där och en teståterställning därifrån. Ett klick på ett avsnitt visar detaljerna, och kortet blir grönt bara på kontrollerade fakta. Ett mål du accepterat utan skydd mot radering räknas där som en accepterad risk, inte som en varning.

## Parkoppla instanser {#pairing}

Flera BombVault-instanser arbetar tillsammans som en grupp: de visar varandras skyddsstatus, tar roller åt varandra och kan förvara varandras nyckel. En instans går med i gruppen med tolv ord.

![Parkopplingssidan med sina tre stegkort.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![Parkopplingssidan med sina tre stegkort.](assets/screenshots/offsite/pairing-light.png#only-light)

På den första instansen öppnar du **Inställningar, Parkoppling** och trycker på **Skapa fras**. Tre stegkort visar hur det går till. Tolv ord visas i ett fönster med en knapp **Kopiera**. På varje annan instans öppnar du samma ställe, trycker på **Ange fras** och klistrar in eller skriver dem, eller trycker på **Klistra in** i det fönstret. Ett ord som inte finns i listan nämns med sin position medan du skriver, och det sista ordet bär en kontrollsumma, så ett felskrivet eller ombytt ord fångas innan något parkopplas. Skapa frasen på bara en instans: två instanser som båda skapar en bildar två separata grupper. Om ingen dyker upp inom en minut erbjuder sidan två vägar ut: visa orden igen för att skriva in dem där borta, eller skriv in den andra instansens ord och gå med i dess grupp i ett steg. Parkoppling fungerar utan inloggningslösenord, men sätt ett: utan det kan vem som helst som kan öppna webbgränssnittet läsa orden och, via gruppen, be om roller som lämnar ut restic-lösenordet för instanserna i den. Parkopplingskortet säger det tills ett lösenord är satt. Med ett lösenord frågar visningen av frasen igen efter det. **Lämna gruppen** tar ut en instans igen. Kortet **Instanser** på parkopplingssidan listar gruppens medlemmar. Bilden av vem som gör vad åt vem finns på sidan Instanser.

Den som kan orden kan gå med i gruppen, så behandla dem som ett lösenord.

**Hur medlemmar når varandra.** Varje instans lär sig sin egen adress i nätverket från din webbläsare i samma stund som du loggar in, listad under **Kända adresser** i kortet **Den här instansen** på parkopplingssidan tillsammans med de instansen hittade själv. Rätta den där om en omvänd proxy eller en ovanlig port står framför. I samma nätverk annonserar medlemmar den adressen med multicast och pratar direkt, och där multicast inte kan korsa ett containernätverk, till exempel Dockers standardbrygga, söker en instans i stället igenom sitt eget subnät efter de andra med ett signerat anrop som bara en gruppmedlem kan svara på, så parkopplingen blir klar på sekunder ändå utan ett relä. Om inget dyker upp tar **Hittar du den inte?** en adress för hand, för ett annat subnät eller en ovanlig port. Instanser i olika nätverk går genom ett relä, valt i en rad om fyra i kortet **Relä** på samma sida. Varje form kommer med en bild:

- **Projektrelä** (standard): `parleyport.halleluja.design`, reläet som KnightLoader också använder. Inget att sätta upp.
- **Eget relä**: containern [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) från Unraid Community Apps (finns också under **Inställningar, Appar**). Ange dess adress på varje instans som ska använda den.
- **BombVault-instans**: en av dina instanser som redan går att nå utifrån, med **Fungera som relä** påslaget. Den instansen svarar då på `/relay/connect` på sin egen adress, bakom den omvända proxy och det certifikat den redan har, och släpper bara in din grupp. Ange dess adress på varje instans som ska använda den.
- **Inget relä**: medlemmar hittar varandra automatiskt i samma nätverk, och ingen annanstans.

**Vad reläet ser.** En knapp med samma namn bredvid de fyra formerna visar detta för formen du valt. Varje anrop mellan medlemmar förseglas med AES-256-GCM under en nyckel som härleds från de tolv orden, och den nyckeln lämnar aldrig dina instanser. Reläet får veta en hash som grupperar anslutningarna, vilken instans ett meddelande är till, hur stort det är och när det passerar. Ett direkt anrop i det lokala nätverket förseglas på samma sätt och signeras också, så ingenting beror på det självsignerade certifikat en instans serverar. Anrop som lämnar ut något (rollsvar, ett restic-lösenord, mottagarinloggningar, nyckeldepositioner) signeras också med en nyckel för medlemmen som skickar dem, fäst vid första ögonkastet. Se [Säkerhetsmodell](configuration.md#security-model).

**Vad som reser över gruppen.** Skyddsstatusen på sidan Instanser, en begäran om att kontrollera ett avsnitt nu, rollförfrågningar och deras svar, lagringserbjudanden, och det en mottagare eller hämtare behöver när dess roll är tillåten: den andra instansens repositoriernas platser och dess restic-lösenord. Säkerhetskopieringsdata gör det aldrig. Den går fortfarande rakt till restic-backends. APP_KEY gör det inte heller: restic-lösenordet öppnar den instansens repositorier och inget annat, inte dess lagrade hemligheter, sessioner eller återställningskoder.

**Poster från före parkopplingen.** Instanser som lagts till med en flottatoken, och mottagna repositorier och hämtningskällor som sattes upp med den andra instansens APP_KEY, blir kvar efter uppdateringen och markeras **Parkoppla igen**. De fortsätter fungera: vid sin första start ersätter BombVault varje lagrad APP_KEY med restic-lösenordet som härleds från den. Parkoppla båda instanserna, redigera sedan posten och välj dess instans. Tills dess står sådana poster i kortet **Utan instans** på sidan Instanser.

Det enda stället som fortfarande tar en APP_KEY för hand är [Från en gammal säkerhetskopiemapp](#restore-from-another-bombvault-repo), för fallet där den andra instansen är borta och inte kan svara i en grupp.

## Instanser och roller {#roles}

Sidan **Instanser** (en inställningsbricka som kan fästas i sidofältet) visar gruppen som ett nät över vem som gör vad åt vem, ett kort för den här instansen och ett kort för varje annan medlem, Android-telefonerna och ZFS-servrarna. **Parkoppling** nere till höger leder till parkopplingssidan. Vilka delar som är på ställs in under **Inställningar, Instanser, Flera servrar**: **Instanser**, **Mottagare** och **Hämtning**.

![Sidan Instanser: gruppen som ett nät, och ett kort per instans med sina rollbrickor, en av dem Inaktiv.](assets/screenshots/offsite/instances-dark.png#only-dark)
![Sidan Instanser: gruppen som ett nät, och ett kort per instans med sina rollbrickor, en av dem Inaktiv.](assets/screenshots/offsite/instances-light.png#only-light)

Varje kort visar medlemmens skyddsstatus och dess roller som brickor. En roll är något som en instans gör åt en annan:

- **Mottagare** håller den här serverns off-site-kopior i en rest-server i append-only-läge. Den här servern skickar efter varje säkerhetskopiering och kan inte radera något där, även om någon tar över den.
- **Hämtare** hämtar den här serverns säkerhetskopior enligt sitt eget schema. Den här servern behöver inte vara vaken för det. Instansen som hämtar behöver ha **Hämtning** påslaget innan den kan tillåta begäran.
- **ZFS-server** tar emot den här serverns ZFS-repliker. Varje ZFS-post väljer den under **Replik** i sina inställningar.
- **Nyckelförvarare** håller en förseglad kopia av den här serverns återställningskit. Se [Nyckelförvarare](#key-keeper).

Att slå på en roll skickar en begäran, och ingenting startar förrän någon på den andra instansen trycker på **Tillåt**. Begäran visas på den instansens kort som ett märke, och dess **Detaljer** listar den med **Tillåt** och **Avvisa**. En tillåten roll kan återkallas där när som helst. En streckad linje i nätet är en roll som fortfarande väntar. En instans på BombVault 9.x kan inte svara på en begäran, så en roll är aktiv där så fort du slår på den, och en begäran från en 9.x-instans väntar på **Tillåt** här.

**Detaljer** på ett kort öppnar resten: den medlemmens mottagna repositorier och hämtningskällor, lagringserbjudanden, övervakning, **Ta bort** och **Glöm nyckeln**.

En medlem kan erbjuda sin egen off-site-lagring till en annan medlem via gruppen. Den andra administratören ser erbjudandet i medlemmens **Detaljer** och accepterar eller avvisar det. Att acceptera skapar en sparad inloggning och ett vanligt lagringsmål, efter samma kontroller som ett mål du skriver in. Målet kommer med skydd mot radering påslaget, eftersom receptet som erbjudandet visar är en rest-server i append-only-läge, så **Hur länge det bevaras** gäller inte där. Varje erbjudande har en egen inloggning och en egen mapp, så att erbjuda ett avsnitt igen, till samma medlem eller en annan, lämnar den första partnerns kopior i fred: lägg till den nya raden i `.htpasswd` och låt servern fortsätta köra. Bara anslutningsuppgifter reser så, aldrig säkerhetskopieringsdata. **Kontrollera nu** ber en medlem verifiera ett avsnitts repositorium. Inget på sidan kan starta en säkerhetskopiering, återställning eller radera något på en annan låda.

Instanser som är avstängda och telefoner med appen stängd behåller sitt kort som **Inaktiv**, i grått, med när de senast sågs, och står dämpade i nätet tills du **Tar bort** dem. Direkt efter att den här servern startat säger ett sådant kort **Kontrollerar** tills gruppen är tillbaka. **Ta bort** fungerar på ett sådant kort vilka delar som än är påslagna, och det avslutar också medlemmens öppna begäranden, dess lediga ZFS-mottagarplatser och dess väntande nyckelbegäranden. Kortet **Instanser** på parkopplingssidan listar de här medlemmarna som **Inaktiv** också.

## Mottagare och hämtare {#receiver}

På lådan som **tar emot** append-only-kopior från en annan BombVault ger de mottagna repositorierna dig oberoende, skrivskyddad övervakning på mottagarhårdvaran, så att ett tyst fel i andra änden inte går obemärkt förbi. Slå på **Mottagare** under **Inställningar, Instanser**, öppna sedan den skickande instansens **Detaljer** på sidan Instanser och lägg till det mottagna repositoriet (skrivskyddat, öppnat med den skickande instansens restic-lösenord, som den får över [parkopplingsgruppen](#pairing)). Du får:

- **En inventering per källa**, så att du ser exakt vilka containrar, VM:ar och mappuppsättningar som har landat. Säkerhetskopior som den skickande instansen gjorde innan den hade en [egen host](#backup-host) räknas till samma källa.
- **Senast mottagen** per källa, så att du vet hur färsk var och en är.
- **En oberoende `restic check`** som körs på mottagarhårdvaran, så att integriteten verifieras där datan faktiskt ligger, inte bara hos avsändaren.
- **En död mans knapp:** ett larm när en källa slutar skicka inom ett fönster du ställer in. Den tittar en gång om dagen, klockan 09:15. Du får också ett larm när repositoriet inte går att läsa alls, oavsett vad kontrollschemat säger, och när inget har kommit fram ett helt fönster efter att du lade till det. Ett repositorium du lägger till före avsändarens första kopia är i ordning fram till dess.
- **Integritetslarm:** ett larm när en kontroll på mottagarsidan misslyckas. En kontroll som inte kunde öppna repositoriet säger ingenting om integriteten, så den lämnar inget utlåtande och försöker igen nästa dag vid samma tid.

Ett repositorium på en annan server behöver den serverns inloggning: spara den som en [inloggning](#sign-ins) och välj den under **Autentiseringsuppgifter** i dialogen.

Mottagaren är strikt skrivskyddad. Den skriver aldrig till det mottagna repositoriet, så den kan aldrig bryta append-only-garantin som avsändaren förlitar sig på.

En hämtare fungerar åt andra hållet: med **Hämtning** påslaget läser en instans en annan medlems säkerhetskopior enligt sitt eget schema, in i ett eget repositorium för varje källa, så att de aldrig räknas som den här lådans egna säkerhetskopior. Dess källor står också i den medlemmens detaljer: repositoriets plats, vilken sorts säkerhetskopia det håller och hur ofta det ska hämtas. Restic-lösenordet kommer över gruppen, aldrig APP_KEY, och andra sidan har inget mer att sätta upp och behöver inte köra. Källan läses bara, aldrig mer, och varje sida behåller sin egen inloggning. En hämtning körs den dag och vid den tid som dess schema säger, och en ny källa väntar också på den tiden. **Hämta nu** väntar inte. En hämtning som missades för att den här lådan var avstängd körs så fort den är tillbaka. En som misslyckades försöker igen nästa dag vid samma tid, även med ett veckoschema, och en misslyckad hämtning syns i aktivitetsloggen och skickar ett larm tills en hämtning fungerar igen. En `rclone:`-källa nekas, eftersom rclone skulle nå den med den här instansens remotes.

En hämtad kopia bara växer. BombVault lägger till i den och varken gallrar eller kontrollerar den, så det som källan sedan länge har glömt blir kvar här tills du raderar mappen, och sidan Lagringsmål räknar inte med den. Den ligger i `pulled/<source id>` inuti avsnittets egna mapp, och när den mappen flyttas blir kopian kvar där den är och senare hämtningar fortsätter där i stället för att kopiera allt igen. Om du raderar den börjar nästa hämtning om i den egna mapp som avsnittet har då.

### Mottagande server {#receiving-server}

Den mottagande lådan kan också köra rest-servern som de andra kopierar till. Den sätts upp första gången du tillåter en **Mottagare**-begäran som ber om det: fönstret säger att servern behöver en mottagande server och ber om en mapp på en delad resurs, med **Ny mapp** för att skapa en, och en port (8000 om inte en annan container använder den). BombVault gör då följande:

1. nekar om en container som heter `rest-server` redan finns eller en annan container håller porten,
2. hämtar `restic/rest-server` och startar den via Docker-socketen i append-only-läge med privata repositorier och en inloggningsfil i mappen,
3. skriver dess Unraid-mall till flashenheten, så att containern förblir redigerbar i Docker-fliken, eller erbjuder mallen som en nedladdning när flashenheten är utom räckhåll,
4. kör manipuleringstestet mot den och visar om den nekar radering.

Instanserna i din grupp hittar sedan servern bland leverantörerna under **Från din grupp**, uppkallad efter den mottagande lådan, och rollen **Mottagare** erbjuder den som det säkra valet. Varje instans får en egen inloggning första gången den använder servern och skriver bara in i sin egen mapp där. Med **Mottagare** påslaget under **Inställningar, Instanser** öppnar **Mottagarserver** på den här instansens kort servern igen. Dess fönster listar inloggningarna under **Partner med inloggning**, och **Återkalla inloggning** tar bort en. Det den instansen redan kopierat blir kvar i mappen. Uppsättningen skapar också en **Inloggning utanför gruppen**, vars lösenord fönstret visar en gång.

En instans som når den mottagande lådan bara via reläet kan inte använda servern, eftersom reläet inte bär några säkerhetskopior. Lägg först till den mottagande lådans adress på parkopplingssidan. När BombVault körs på en egen IP-adress (till exempel på br0) fyller du i **Adress för partner**, eftersom servern lyssnar på värdens adress.

## Exempel: parkoppla två Unraid-lådor {#worked-example-two-unraid-boxes-end-to-end}

Det här är en komplett uppsättning av delarna ovan, eftersom de är lättare att sätta ihop när man har sett dem ihopsatta.

Två lådor: **TOWER** kör containrarna och skickar säkerhetskopior. **VAULT** tar emot dem och upprätthåller append-only. Byt ut mot dina egna namn, adresser och resursvägar.

**1. Parkoppla de två lådorna.** Öppna **Inställningar, Parkoppling** på TOWER, tryck på **Skapa fras** och skriv in de tolv orden på VAULT under **Ange fras**. Sätt först ett inloggningslösenord på båda.

**2. På TOWER, be VAULT bli mottagare.** Slå på **Mottagare** på VAULTs kort på sidan Instanser, välj avsnitten och **VAULTs mottagande server** som plats, och skicka begäran.

**3. På VAULT, tillåt den.** VAULTs sida Instanser visar begäran på TOWERs kort. Tryck på **Tillåt**. Första gången frågar VAULT efter en mapp för sin mottagande server, till exempel `user/appdata/rest-server`, startar rest-servern i append-only-läge och kör manipuleringstestet mot den.

**4. Tillbaka på TOWER.** Från och med nu kopierar TOWER till VAULT efter varje säkerhetskopiering, och det nya lagringsmålet visas på TOWERs sida Lagringsmål med sitt skydd mot radering.

Vad manipuleringstestet svarar:

| Resultat | Vad som hände |
| --- | --- |
| **skyddad** | VAULT nekade raderingen. Det här är det enda godkända läget. |
| **inte skyddad** | VAULT accepterade en radering. `--append-only` saknas eller har tagits bort. |
| **ofullständigt** | Ingetdera. Oftast är URL:en inte den restic själv använder, eller så har inloggningsuppgifterna ändrats. Inget registreras och inget larm utlöses. |

**5. Titta på vad som kommer fram, om du vill.** Slå på **Mottagare** under **Inställningar, Instanser** på VAULT, öppna TOWERs **Detaljer** och lägg till det mottagna repositoriet: klicka på en av platserna som dialogen listar. VAULT får TOWERs restic-lösenord över gruppen, och eftersom platsen ligger på VAULTs egen mottagande server läser den mappen direkt. Ingen skriver in en nyckel eller en inloggning.

**6. Gör det ömsesidigt.** Upprepa stegen i andra riktningen: VAULT ber TOWER bli dess mottagare. Varje låda upprätthåller då append-only för den andra, och ingen kan radera den andras säkerhetskopior.

En rest-server som du kör för hand fungerar också: lägg till den med **Lägg till mål** som en rest-server, med användare och lösenord från dess `.htpasswd`, och slå på skydd mot radering. Behåll `--append-only` i dess alternativ. Utan det är andra sidan en vanlig delad resurs igen.

!!! warning "En lokal sökväg är en sökväg inuti containern"
    Där BombVault ber om en mapp på den här servern anger du den relativt värdmonteringen, till exempel `user/appdata/rest-server/bombvault-containers`, inte `/mnt/user/appdata/...`. BombVault körs i en container, där värdens `/mnt` är monterad någon annanstans. En absolut värdsökväg finns inte inuti den. Om du klistrar in en säger BombVault vilken relativ sökväg du ska använda i stället.

## Nödguide {#emergency}

**Nödguiden** tar tillbaka en server som är borta eller som har satts upp på nytt. Den startar på sidan **Återställning** under **Nödläge**, och från **Återställ från en säkerhetskopia** på välkomstsidan för en ny installation. Den går i fyra steg: **Åtkomst**, **Inställningar**, **Urval** och **Återställning**.

1. **Åtkomst.** Visa BombVault nyckeln till de gamla säkerhetskopiorna, på ett av tre sätt:
    - **Ladda upp återställningskit**: filen du laddade ner under uppsättningen. Den innehåller nyckeln och varje lagringsmål, och BombVault fyller i resten från den. Ett kit förseglat med age ber om identiteten det förseglades för. Den används en gång och sparas inte.
    - **Via gruppen**: en instans i din grupp förvarar nyckeln som en deposition. Du behöver gruppens tolv ord, din utlämningskod och någon vid den instansen. Se [Nyckelförvarare](#key-keeper).
    - **Skriv in nyckeln**: den gamla serverns `APP_KEY`, och var dess säkerhetskopior finns.

    BombVault provar sedan nyckeln på varje lagringsmål och säger för vart och ett om det är läsbart, hittades inte, är låst med en annan nyckel, inte går att nå eller fortfarande saknar sin inloggning. Ingenting har sparats än, och nyckeln stannar i minnet en begränsad tid. **Ta över nyckeln** får den här servern att skriva sina säkerhetskopior med den nyckeln från och med nu. **Ångra övertagandet** går tillbaka till den egna nyckeln tills den första säkerhetskopian har körts. En server som redan har egna säkerhetskopior nekar övertagandet och pekar på [Från en gammal säkerhetskopiemapp](#restore-from-another-bombvault-repo) i stället. **Spara inloggningsuppgifter** lagrar de inloggningar kitet bär för lagringsmål på distans, och frågar innan det ersätter inloggningar som den här servern redan har.
2. **Inställningar.** **Återställ inställningar** återställer BombVaults egna inställningar från den nyaste auto-säkerhetskopian: lagringsmål, inloggningar, scheman och regler. BombVault startar om för att tillämpa dem, via Docker-socketen, så att den levande inställningsdatabasen aldrig skrivs över under ett öppet handtag, och guiden fortsätter av sig själv efteråt. Den här serverns inloggning blir kvar: du fortsätter logga in med lösenordet du använder nu. Hemligheter som den gamla servern hade lagrat och som inte går att ta över listas, så att du kan skriva in dem igen. Återställningen avslutar varje session och tar bort lösennycklarna, så sätt upp dem igen efteråt.
3. **Urval.** BombVault går igenom dina säkerhetskopior och listar de containrar, VM:ar, mappuppsättningar och ZFS-datauppsättningar den hittar. Den nyaste ögonblicksbilden är förvald överallt. Det den här servern fortfarande har börjar avslaget, eftersom en återställning skulle ersätta det. Mappuppsättningar vet inte längre var de kom ifrån efter en återuppbyggnad, så de går in i en mapp du väljer.
4. **Återställning.** Allt som är valt kommer tillbaka på en gång, lämnat stoppat om du vill starta varje container och VM själv, vilket är tryggast efter en total förlust. Ladda ner ett nytt återställningskit efteråt: det gamla hör till den förra installationen.

![Nödguiden vid sitt första steg, med de tre vägarna in.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![Nödguiden vid sitt första steg, med de tre vägarna in.](assets/screenshots/offsite/emergency-light.png#only-light)

### Exempel: en ny server från återställningskitet {#example-emergency}

Servern TOWER dog med sina diskar. Den hade 23 containrar och 2 VM:ar, och kopierade dem och sin auto-säkerhetskopia till Backblaze B2. Du har `bombvault-recovery-kit.md` från din lösenordshanterare och en ny låda med Unraid på. Dess gamla `APP_KEY` är borta, och du behöver den inte.

1. **Installera BombVault** på den nya lådan och ge den en ny `APP_KEY`, som på vilken ny installation som helst. Sätt upp VM-länken under **Inställningar, Integrationer, Värd-SSH** om du vill ha tillbaka VM:arna, eftersom en VM-återställning behöver den.
2. **Starta guiden.** Välj **Återställ från en säkerhetskopia** på **Välkommen till BombVault**. Välj **Starta nödguiden** under **Nödläge** på sidan Återställning.
3. **Åtkomst.** Välj **Ladda upp återställningskit** och släpp filen. BombVault provar sin nyckel på varje lagringsmål och säger till exempel **Återställningskitet lästes. Dess nyckel öppnar 0 av 2 lagringsmål.** Den lokala mappen säger **hittades inte**, eftersom disken dog, och Backblaze B2 säger **inloggning saknas**.
4. **Spara inloggningsuppgifter.** Kitet bär inloggningen till varje mål. Tryck på **Spara inloggningsuppgifter**, så provas målen igen och Backblaze B2 säger nu **läsbar**. Ett kit som skrevs före 10.5 innehåller bara standardinloggningen. Där ett mål fortfarande säger **inloggning saknas** anger du dess inloggning under **Ange saknade inloggningsuppgifter** och trycker på **Försök med målen igen**.
5. **Ta över nyckeln.** Den här servern skriver nu sina säkerhetskopior med den gamla nyckeln, så att de nya säkerhetskopiorna passar de gamla. Den behåller sin egen `APP_KEY` i mallen. **Ångra övertagandet** fungerar tills den första säkerhetskopian har körts.
6. **Inställningar.** Steget öppnas på platsen där nyckeln hittade auto-säkerhetskopian, här B2, med adressen redan ifylld. Välj **Återställ inställningar**, som läser den nyaste auto-säkerhetskopian därifrån. BombVault startar om för att tillämpa den, och guiden fortsätter av sig själv. Dina lagringsmål, scheman och regler är tillbaka, och du fortsätter logga in med den här serverns lösenord. Skriv in hemligheterna som guiden listar igen, till exempel aviseringsinställningar.
7. **Urval.** **Var finns säkerhetskopiorna?** öppnas på off-site-kopiorna när inget lokalt finns kvar. BombVault läser varje mål, så att ett dött mål inte döljer de andra, listar de 23 containrarna och 2 VM:arna och återställer var och en från det mål som har dess nyaste ögonblicksbild.
8. **Återställning.** Slå på **Lämna stoppad efteråt**, vilket är tryggast efter en total förlust, och starta. **Vad som händer** säger vad som kommer tillbaka. Resultatraden lyder till exempel **25 återställda, 0 misslyckade. Starta dem under Säkerhetskopior vid behov.**
9. **Avsluta.** Starta dina databaser först, sedan apparna som använder dem. Öppna **Inställningar, Återställning** och ladda ner ett nytt återställningskit, eftersom det gamla hör till den förra installationen. Kontrollera **Behöver uppmärksamhet** på översikten, och kör **Säkerhetskopiera allt nu** när allt fungerar.

Om auto-säkerhetskopian också saknas öppnas steget på **Hoppa över: jag har ingen auto-säkerhetskopia** och säger det. Lagringsmålen kommer då från kitet ensamt, du sätter upp scheman och regler igen, och off-site-kopior väntar tills du tar över deras säkerhetskopior som den här serverns egna, se [Efter en återuppbyggnad](#after-a-rebuild). **Urval** listar fortfarande vad off-site-kopiorna innehåller. Mappuppsättningar hamnar i en mapp du väljer. Containrar och VM:ar kommer tillbaka som vanliga filer i en mapp under **Bara filerna kan komma tillbaka**, eftersom deras definitioner ligger bredvid det lokala repositoriet och i auto-säkerhetskopian, och en off-site-kopia innehåller ingetdera.

!!! tip "Planerad flytt kontra katastrof"
    Nödguiden återställer BombVaults egna inställningar från en säkerhetskopia. För en *planerad* flytt till en ny låda kan du i stället ta med din konfiguration direkt med kortet **Exportera / importera inställningar** (en portabel JSON-fil). Se [Konfiguration](configuration.md#portable-settings-export-and-import).

### Från en gammal säkerhetskopiemapp {#restore-from-another-bombvault-repo}

**Från en gammal säkerhetskopiemapp** under **Andra källor** på sidan Återställning öppnar en *annan* BombVault-instans repositorium (en delad resurs monterad under `/mnt`, eller en fjärradress) med den instansens återställningskit eller dess inskrivna `APP_KEY`, i en engångssession som är skrivskyddad. Bläddra bland containrar, VM:ar, mappuppsättningar och ZFS-datauppsättningar som finns lagrade där, välj en ögonblicksbild och återställ den, så blir det återställda objektet en vanlig lokal container, VM eller mappuppsättning. Ingenting skrivs någonsin till det andra repositoriet, och dina egna säkerhetskopieringsinställningar förblir orörda (sessionen lever i minnet och går ut av sig själv). Att flytta en container från server A till server B betyder inte att du pekar om dina inställningar och sedan ändrar tillbaka dem. Det här fönstret är en engångsgrej: det öppnar en session, återställer det du väljer och glömmer den andra instansen. Om du i stället vill ha ett stående arrangemang, där den här lådan hämtar en annan instans ögonblicksbilder till ett eget repositorium enligt ett schema, är det rollen **Hämtare** på sidan Instanser.

### Det den här servern hämtade {#restore-what-was-fetched}

Varje källa som den här servern har hämtat från har en egen rad **Hämtat från** under **Andra källor** på sidan Återställning. En hämtad kopia är låst med den här serverns egen nyckel, inte källans, så ingenting efterfrågas: källans nyckel öppnar den inte. Mappuppsättningar och ZFS-datauppsättningar hamnar i en mapp du väljer. Containrar och VM:ar kommer tillbaka som vanliga filer i en mapp, eftersom en hämtning bär ögonblicksbilderna men inte definitionerna. För att återskapa dem använder du källans återställningskit och auto-säkerhetskopia i Nödguiden.

En container vars nätverk inte finns på den här servern, till exempel ett Unraid-`br0`-nätverk på en vanlig Docker-värd, visar en nätverksväljare under sin rad. BombVault skapar den på nätverket du väljer, tillsammans med dess andra nätverk. Den fasta IP-adressen och MAC-adressen hörde till det gamla nätverket och släpps, så det nya nätverket tilldelar dem.

## Nyckelförvarare {#key-keeper}

En nyckelförvarare är en instans i din grupp som håller en förseglad kopia av den här serverns återställningskit. Den kan inte öppna kopian själv. Om den här servern någonsin går förlorad ber en ny installation som går med i din grupp förvararen om kopian och öppnar den med din utlämningskod.

![Fönstret med utlämningskoden: tjugo tecken i fyra grupper, visad en gång.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![Fönstret med utlämningskoden: tjugo tecken i fyra grupper, visad en gång.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. Slå på **Nyckelförvarare** på sidan Instanser, på kortet för instansen som ska förvara den, och välj **Deponera**. Att deponera kräver ett inloggningslösenord.
2. Den här servern visar **Utlämningskod** en gång: tjugo bokstäver och siffror i fyra grupper om fem. Skriv ner den och bekräfta med **Jag har skrivit ner koden**. Den här servern lagrar inte koden, bara det den behöver för att försegla en deposition för den.
3. Förvararen håller nu depositionen, och den här servern förnyar den när kitet ändras. **Nyckeldeposition** på den här instansens kort listar **Den här serverns deposition** med varje förvarare och dess läge, och **Depositioner som den här servern förvarar** för de andra. **Skapa en ny kod** ersätter koden. Varje förvarare byter till den nya när den får den nya depositionen.

Den dagen behöver du tre saker: gruppens tolv ord, utlämningskoden och åtkomst till förvararens gränssnitt, där någon måste trycka på **Tillåt**. Välj **Via gruppen** i Nödguiden, välj depositionen, be om den och skriv utlämningskoden. Koden används på den nya servern och skickas inte till förvararen. Hos förvararen visas begäran under **Nyckeldeposition** med vem som frågar, om den instansen någonsin setts där förut och hur ofta den frågat. **Tillåt** lämnar ut den förseglade kopian, och att tillåta kräver ett inloggningslösenord. Den nya installationen har 30 minuter på sig att hämta den efter Tillåt, en begäran som ingen svarar på går ut efter ett dygn, och en instans som frågar för ofta måste vänta en timme.

Utan koden kan ingen öppna kopian, förvararen inkluderad. Om koden förloras, skapa en ny medan den här servern fortfarande körs. Om servern redan är borta fungerar återställningskitet och den inskrivna APP_KEY fortfarande. Förvararen kan radera en deposition när som helst, och ägaren måste då fråga igen.

## Återställningskit {#recovery-kit}

Det här är biten som gör katastrofåterställning möjlig även när det inte finns någon körande BombVault.

**Ladda ner återställningskit** i kortet **För nödfall** under **Inställningar, Återställning** ger dig **nyckeln**, det **härledda restic-lösenordet**, varje **lagringsmål** med repositoriet för varje avsnitt, de inloggningar en ny installation behöver för att nå dem och de **exakta restic-kommandona**, så att du kan återställa rakt med restic-CLI på vilken maskin som helst. Det kräver ett inloggningslösenord. Kitet skriver ut nyckeln också med kryptering avslagen, eftersom container- och VM-definitionerna alltid är förseglade med den. Efter ett övertagande skriver det ut den övertagna säkerhetskopienyckeln och den här serverns APP_KEY, som en ny installation behöver bara för att få tillbaka inställningarna. Ett kort **Spara ditt återställningskit** på översikten påminner dig tills du har förvarat det.

!!! danger "Förvara återställningskitet utanför servern"
    Kitet innehåller hemligheten som dekrypterar dina säkerhetskopior. Förvara det på en säker plats åtskild från servern (en lösenordshanterare, en utskrift i ett kassaskåp). Om du förlorar både BombVault och `APP_KEY` utan återställningskit och utan nyckeldeposition går dina krypterade säkerhetskopior inte att återställa.

!!! warning "Den nyaste ögonblicksbilden är inte alltid den att återställa"
    Sedan restic 0.17 visar `restic snapshots` varje ögonblicksbilds storlek. Efter dataförlust kan den nyaste ögonblicksbilden vara den tömda, så återställ inte en ögonblicksbild som är mycket mindre än de före den. Efter ransomware kan den vara den krypterade i vanlig storlek. Om BombVault fortfarande körs, kontrollera först dess sida **Avvikelser**: den nämner den sista goda säkerhetskopian, och återställningsfönstret förväljer den. En återställning behöver ingen av BombVaults avvikelsedata, och gallringspausen behåller bara fler ögonblicksbilder.

### Hosten för en servers säkerhetskopior {#backup-host}

Varje server skriver sina säkerhetskopior under en egen restic-host: `bv-` och tolv tecken som räknas fram ur säkerhetskopiornas nyckel, till exempel `bv-3f9a0c41d2e7`. Säkerhetskopior från innan en server fick en egen host har hosten `bombvault`. Kitet anger den här serverns host, och dess kommandon skickar med båda via `--host`, så att `latest` aldrig väljer en säkerhetskopia som en annan server har skrivit till samma repositorium. En server som tar över en annans säkerhetskopior fortsätter under den serverns host, eftersom hosten kommer från nyckeln.

### Försegla kitet {#sealing-the-kit}

Om du har slagit på **Kryptera exporter med age** under **Inställningar, System** förseglas kitet med det också och laddas ner som `bombvault-recovery-kit.md.age`. Det är ASCII-pansrat snarare än binärt, så det är fortfarande vanlig text: att klistra in det i en lösenordshanterare eller skriva ut det fungerar precis som förut, innehållet är oläsligt utan din nyckel.

!!! warning "Förvara inte age-nyckeln inuti kitet"
    Du behöver din **privata** age-nyckel för att öppna ett förseglat kit. Förvara den någonstans som inte beror på själva kitet, annars har du två saker att återställa i stället för en. Förseglingen är värd det när kitet förvaras någonstans du inte helt kontrollerar (en delad lösenordshanterare, molnanteckningar, en utskrift på ett kontor). Ett kit i ditt eget kassaskåp är redan skyddat av kassaskåpet.

    Med kryptering på och ingen användbar mottagare konfigurerad nekas nedladdningen helt. BombVault faller aldrig tillbaka på att lämna ut huvudnyckeln i klartext.

### Om du inte har kitet till hands {#if-you-do-not-have-the-kit-to-hand}

Lösenordet lagras inte någonstans, det **beräknas** från säkerhetskopiornas nyckel, så du kan återskapa det själv med bara nyckeln och ett skal:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

Det är HMAC-SHA256 över den fasta strängen `bombvault:restic-repo`, nyckelad med råbyten av den hexadecimala `APP_KEY`, utskriven som 64 hexadecimala tecken med gemener. Samma värde finns i kitet, listat som det härledda restic-lösenordet. Det här är för den dag kitet ligger någonstans där du inte är. På en server som tagit över en annans säkerhetskopior använder du den övertagna säkerhetskopienyckeln, som är den serverns gamla APP_KEY.

[Hosten](#backup-host) kommer från samma nyckel och strängen `bombvault:host`: `bv-` och de första tolv av de tecknen.

```sh
echo "bv-$(printf 'bombvault:host' | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r | cut -c1-12)"
```

!!! warning "För ett mottaget repositorium, använd den SKICKANDE instansens nyckel"
    Ett repositorium som kom hit genom en off-site-kopia skapades av maskinen som skickade det, med **dess** nyckel. Att härleda från den mottagande lådans nyckel ger ett lösenord som restic avvisar, vilket läses precis som ett skadat repositorium och inte är det. Det här är den vanliga orsaken till att `restic check` på ett mottaget repositorium frågar efter ett lösenord om och om igen.

Eftersom återställningsdefinitionerna ligger **inuti** varje repositorium (`<repo>/def`, `<repo>/vm-def`) är en kopierad repositoriemapp fullständigt självförsörjande, så kitet plus repositoriet är allt en återställning från noll behöver. En off-site-kopia och en hämtad kopia innehåller ögonblicksbilderna utan definitionerna: de ger tillbaka filer, och en container eller VM återskapas från auto-säkerhetskopian.

## Få tillbaka en databasdump {#database-dumps}

En databasdump är en egen återställningspunkt i containrarnas repositorium, taggad `dbdump:<container>`, som håller den enda filen `/dbdump/<container>.sql`. BombVault listar, laddar ner och importerar dem på containerns flik **Säkerhetskopior**. Nedan är samma steg med bara restic, för den dag BombVault inte finns.

```sh
restic -r <repo> snapshots --host <host> --host bombvault --tag dbdump:<container>
restic -r <repo> dump --host <host> --host bombvault --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

`<host>` är serverns [host](#backup-host) från återställningskitet.

Taggarna `dbversion:` och `dbname:` på varje dump säger vilken serverversion den kom från och vilka databaser den innehåller. En komplett fil slutar med `-- PostgreSQL database cluster dump complete` eller `-- Dump completed`.

Importera den till en container med samma eller en nyare version (PostgreSQL), eller samma huvudversion (MySQL och MariaDB), startad en gång med en tom datamapp så att den initieras. Värden behöver ingen databasklient, containern har en:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

För en enskild databas ur en full dump tar MySQL och MariaDB `--one-database <name>` på klientkommandot. En PostgreSQL-dump har ett avsnitt per databas, vart och ett börjar med en rad `\connect <name>`: kopiera det avsnittet till en egen fil och importera det med `-d <name>` efter att du skapat databasen.

!!! warning "En root-dump bär serverns användare"
    En full MySQL- eller MariaDB-dump som tagits som root innehåller systemdatabasen `mysql`, så att importera den ersätter den nya serverns konton, root-lösenordet inkluderat, med dem från dumpen. På PostgreSQL är `role ... already exists` för användaren som containern skapade väntat och ofarligt.
