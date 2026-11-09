# Återställning

Varje återställning börjar på sidan **Återställning**: välj ett avsnitt, sök efter det du vill ha tillbaka och välj en säkerhetskopia. Ett enda fönster återställer alla slags poster och frågar i steg. Som standard ändras ingenting på originalet, eftersom säkerhetskopian kommer tillbaka bredvid det.

![Sidan Återställning med avsnitten som brickor, sökningen och kortet Stackar.](assets/screenshots/restore/page-dark.png#only-dark)
![Sidan Återställning med avsnitten som brickor, sökningen och kortet Stackar.](assets/screenshots/restore/page-light.png#only-light)

## Sidan Återställning {#page}

- **Avsnitt** listar avsnitten som brickor, var och en med hur många poster som har ögonblicksbilder.
- **Vad vill du få tillbaka?** söker bland posterna i det valda avsnittet. Varje rad visar sin nyaste ögonblicksbild, och **Öppna posten** leder till postens sida.
- **Hitta en fil** letar efter en fil eller mapp med dess namn i varje säkerhetskopia, se [Hitta en fil](#find-a-file).
- **Stackar** listar Compose-projekten med minst två säkerhetskopierade containrar, till exempel `immich` med sin databas och Redis. Sökningen hittar en stack på dess namn eller på någon av dess containrar.
- **Körs nu** visar en återställning medan den pågår, även efter att du stängt dess fönster.
- **Nödläge** startar Nödguiden för en server som är borta eller som har satts upp på nytt, se [Nödguide](offsite-recovery.md#emergency).
- **Andra källor** innehåller **Från en gammal säkerhetskopiemapp**, för säkerhetskopiorna från en annan BombVault, **Importera från pluginet Appdata.Backup** och **Jobb från Duplicati eller Duplicacy**.

![Sidan Återställning på en telefon.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![Sidan Återställning på en telefon.](assets/screenshots/restore/page-phone-light.png#only-light)

## Återställningsfönstret {#window}

Ett klick på en post öppnar **Återställning** med dess namn. Det frågar i steg, och varje steg kan ändras innan du startar:

1. **Säkerhetskopia**: den rekommenderade ögonblicksbilden är förvald. Det är den nyaste, eller efter en öppen avvikelse den sista före den. Säkerhetskopian som togs före en tidigare återställning innehåller det läge som den återställningen ersatte, så den hoppas över om det inte finns någon annan. Äldre finns i listan, och **Jämför** visar vad som skiljer mellan två av dem.
2. **Källa**: var filerna kommer ifrån, när en ögonblicksbild ligger på flera ställen. **Lokal** är snabbast.
3. **Vad**: **Alla**, eller **Enskilda filer**, där du bockar för de filer och mappar du vill ha.
4. **Vart**: målet. Valen beror på typen av post:

    | Mål | Vad det gör |
    |---|---|
    | **Bredvid originalet** | Standard. En container kommer tillbaka som en andra container med namnet `<name>-restored` och egna portar, en VM:s diskar och NVRAM hamnar i en mapp som heter `<folder>-restored`, och en mappuppsättning kommer tillbaka som `<path>-restored`. Ingenting ändras på originalet. |
    | **Ersätt originalet** | Skriver över det som finns nu. Med **Säkerhetskopiera nuvarande läge först**, som är på som standard, säkerhetskopierar BombVault först dagens läge så att du kan gå tillbaka. Om den säkerhetskopian misslyckas startar inte återställningen. Det fungerar också för en container eller VM som inte längre finns på servern: mapparna som återställningen skriver över säkerhetskopieras var för sig. |
    | **Till en mapp** | Bara filerna, för att bläddra bland dem. |
    | **Ladda ned som ZIP** | Flashenheten, för en ny sticka. BombVault skriver aldrig in i den körande flashenheten. |
    | **Till en ny datauppsättning** och **Återställ in i datauppsättningen** | ZFS-datauppsättningar, se [ZFS-datauppsättningar](zfs-datasets.md#restore). |

5. **Vad som händer** säger med ord vad återställningen kommer att göra, och en scen visar målet du valt.

Innan **Återställ** låses upp kontrollerar fönstret att repositoriet svarar, att nyckeln öppnar det, att ögonblicksbilden finns och att målet har plats. Så länge en kontroll misslyckas förblir knappen låst, och (i) i den säger vilken.

Fönstret visar också vad återställningen ändrar jämfört med det som finns nu: nya, ersatta och oförändrade filer, med listan på begäran, och de filer på målet som inte finns i säkerhetskopian och som stannar där de är. restic räknar ut det som en torrkörning från storlek och ändringstid, utan att läsa filerna, och ett mycket stort träd avbryts efter 30 sekunder och säger det. För containrar och VM:ar jämför det de inställningar som återställningen återskapar med de körande: image och tagg, portar, variabelnamn och volymer, eller minne, vCPU:er, diskar och nätverk.

Några saker kan stoppa en återställning innan den ändrar något. En containers statiska IP och publicerade portar måste vara lediga, och dess nätverk måste finnas. På Unraid nekar varje återställning på plats en mapp som inte ligger på en monterad pool eller share, eftersom filerna skulle hamna i minnet och vara borta efter en omstart. En återställning på plats nämner varje annan container vars bind-montering når in i en mapp som den skriver till, till exempel "den här sökvägen används också av nextcloud-db", men det är bara en varning.

Några alternativ visas där de gäller: **Lämna stoppad efteråt**, för att titta på en container eller VM innan den körs igen, och **Återställ utan GPU och runtime** på en värd som saknar den GPU eller runtime som säkerhetskopian gjordes med.

Återställningen körs i bakgrunden. Du kan stänga fönstret, och förloppet visas också under **Körs nu**. En lång återställning visar en procentsats och kan avbrytas, och en avbruten återställning loggas som avbruten, inte som misslyckad. En återställning som avbryts medan containern stoppas, eller en som inte kan ta bort containern, har inte återställt något än, så containern startar igen. En container som tar sina volymer från en annan med `--volumes-from` behåller dem, och återställningen vägrar att börja så länge den andra containern inte finns.

![Återställningsfönstret vid steget Vad, med enskilda filer ibockade.](assets/screenshots/restore/window-what-dark.png#only-dark)
![Återställningsfönstret vid steget Vad, med enskilda filer ibockade.](assets/screenshots/restore/window-what-light.png#only-light)

### Vart en återställning till en mapp går {#folders}

**Mappar för återställning** under **Inställningar, Återställning** sätter **Standardmapp för återställning**, med en **Undermapp per avsnitt** om du vill, och ett **Undantag** för ett avsnitt. Valda filer hamnar rakt i målmappen: en fil kommer tillbaka som bara den filen, en mapp som mappen med sitt innehåll, och val från olika mappar behåller bara sin sökväg under den mapp de delar. BombVault vägrar ett mål som inte ligger på monterad lagring, eftersom en sådan mapp på Unraid bor i minnet och är borta efter en omstart, och återställer aldrig in i sin egen datamapp.

## Exempel: få tillbaka en enda fil {#example-single-file}

Någon redigerade Nextclouds `config.php` i natt och Nextcloud startar inte längre. Nattens säkerhetskopia från 03:00 har fortfarande den goda filen. Du vill ha just den filen, inte hela containern.

1. Öppna **Återställning**, välj brickan **Containers** och skriv `nextcloud` i **Vad vill du få tillbaka?**.
2. Klicka på `nextcloud`. Fönstret öppnas med 03:00-säkerhetskopian förvald som den rekommenderade.
3. Låt **Källa** stå på **Lokal**.
4. Välj **Enskilda filer** under **Vad**. Öppna mapparna ner till `config` och bocka för `config.php`.
5. Välj **Till en mapp** under **Vart** och välj en mapp, till exempel `user/restore`, som på värden är `/mnt/user/restore`.
6. Läs **Vad som händer**: filen hamnar i målmappen, och ingenting ändras på containern. Tryck på **Återställ**.

Fönstret slutar med **Klart: filerna ligger i /mnt/user/restore**. Du hittar filen som `/mnt/user/restore/config.php`, utan mapparna ovanför. Jämför den med den trasiga, kopiera tillbaka den till appdata-mappen och starta om Nextcloud.

Om du hellre vill lägga tillbaka filen direkt väljer du **Ersätt originalet** i steg 5. Låt **Säkerhetskopiera nuvarande läge först** vara på, så att BombVault säkerhetskopierar det trasiga läget innan det skriver, skriv `nextcloud` för att bekräfta och tryck på **Återställ**. Nextcloud stoppas medan den valda filen skrivs tillbaka till sin plats och startas igen efteråt, och bara de filer du bockat för skrivs. En återställning som misslyckas halvvägs lämnar Nextcloud stoppad, eftersom filen den läser kan vara halvskriven, och körningen säger det.

![Återställningsfönstret vid Vad som händer, med Ersätt originalet valt.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![Återställningsfönstret vid Vad som händer, med Ersätt originalet valt.](assets/screenshots/restore/window-replace-light.png#only-light)

## Hitta en fil {#find-a-file}

När du vet namnet på en fil men inte vilken post eller säkerhetskopia som fortfarande har den, skriver du namnet i **Hitta en fil** och trycker på **Sök**. Vanlig text matchar var som helst i namnet och bryr sig inte om stora och små bokstäver, så `invoice` hittar `Invoice-2024.pdf`. Med `*` eller `?` måste mönstret matcha hela namnet, så `*.pdf` hittar varje PDF. Skriv bara namnet, utan mapp.

Sökningen läser varje säkerhetskopia av containrarna, mappuppsättningarna, ZFS-datauppsättningarna och flashenheten. Varje träff visar posten, sökvägen och varje säkerhetskopia som har filen, nyaste först, med datum, storlek och platsen den lästes på. En säkerhetskopia där filen har samma storlek och ändringstid som i den före säger **oförändrad**, så du ser var filen senast ändrades. Ett klick på en säkerhetskopia öppnar återställningsfönstret för den, med **Enskilda filer** valt och filen förbockad. En fil från flashenheten kommer tillbaka som en del av flashenhetens ZIP, eftersom flashenheten bara återställs som helhet.

Sökningen tar en stund, eftersom restic går igenom varje säkerhetskopia. **Stoppa** avslutar den, och det gör det också om du lämnar sidan. Under fältet står vilka platser den läste:

- Platser på den här servern och i ditt nätverk genomsöks alltid.
- En säkerhetskopia som ligger på flera platser läses bara en gång, på den första. En plats vars säkerhetskopior alla lästes någon annanstans säger **redan genomsökt på en annan plats**.
- Externa platser som S3, B2 eller SFTP hoppas över, om du inte slår på **Sök även på externa platser**. Då läses där bara de säkerhetskopior som ingen plats här har, oftast de äldre som sparas längre externt. Den första sökningen på en sådan plats kan ta lång tid, och vissa leverantörer tar betalt för den data som laddas ner.
- En plats som inte gick att öppna säger **kunde inte läsas**, och (i) bredvid förklarar varför. De andra platserna genomsöks ändå.

Virtuella maskiner sparas som hela diskar, så filerna i dem genomsöks inte. Auto-säkerhetskopian och säkerhetskopiorna från en annan BombVault under **Andra källor** genomsöks inte heller. En sökning listar de första 200 filerna, så ett kort namn som `log` kan behöva fler bokstäver.

## Vad som kommer tillbaka, per typ {#kinds}

**Containrar** återskapas från sin sparade definition via Docker-API:et, med sin entrypoint, värdnamn, stoppinställningar, volymer och monteringar, resursgränser, loggdrivrutin, DNS-inställningar och äldre länkar. En container kommer tillbaka på den image den körde när säkerhetskopian togs, inte på en nyare som hämtats senare, och en image som saknas hämtas först, med inloggningen från **Container-register** under **Inställningar, Återställning**. Dess GPU eller runtime (`--gpus`, `--runtime=nvidia`) kommer också tillbaka. På en värd utan den drivrutinen eller runtime säger återställningen det och erbjuder **Återställ utan GPU och runtime**. En länk till en container som saknas, eller som är stoppad när den återställda startar, utelämnas, och körhistoriken säger det. En återställning som misslyckas efter att containern tagits bort lägger tillbaka den, stoppad.

En säkerhetskopia kan innehålla mappar som containern inte längre har valda, till exempel en äldre som togs innan en mapp försvann. En mapp som är borta eller tom kommer tillbaka. En som innehåller filer nu förblir som den är, eftersom du kan ha bockat ur den med avsikt, och kontrollen före återställningen och körhistoriken nämner den. Få tillbaka den med **Enskilda filer** eller **Till en mapp**.

**VM:ar** får sin XML definierad på nytt över SSH och sina diskar och UEFI NVRAM återkopplade, även efter att VM:en har raderats. Se [VM-säkerhetskopior](vm-backup-ssh-setup.md) för VM:ar med Unraid-ögonblicksbilder och diskar på zvols.

**Mappuppsättningar** kommer tillbaka med sitt ägarskap, sina behörigheter och sina tidsstämplar som restic lagrar dem. Där målet bara vägrar ägarskapet, som `/mnt/user` gör, slutar körningen med en anteckning som säger det. BombVault jämför sedan filerna med säkerhetskopian, och en fil som inte kom fram får återställningen att misslyckas.

**Flashenheten** rör aldrig den levande `/boot`. Återställningen är en nedladdning som heter `flash-<id>.zip`, redo att släppas in i Unraid USB Creator. ZIP-filen innehåller `APP_KEY`, så att ladda ner den kräver ett inloggningslösenord och en inloggning på instansen. Fönstret listar också pluginen i varje flash-säkerhetskopia med version och storlek, och kan lägga tillbaka ett enskilt i den körande flashen: dess `.plg`-fil, dess mapp under `config/plugins` och de paketfiler som säkerhetskopian innehåller. Unraid installerar det vid nästa uppstart, eller direkt under **Plugins, Install Plugin**.

**ZFS-datauppsättningar** har egna mål och behåller sina ZFS-egenskaper, se [Återställning](zfs-datasets.md#restore).

**Auto-säkerhetskopian** kommer tillbaka bara via Nödguiden, som startar om BombVault, så att den levande databasen aldrig skrivs över under ett öppet handtag.

**Databasdumpar** laddas aldrig tillbaka på egen hand. En rad under **Databasdumpar** kan laddas ner, sparas i en mapp eller importeras till en nystartad databas. Se [Databasdumpar](backups.md#database-dumps).

## Återställa flera poster på en gång {#several}

På sidan Återställning väljer du poster på samma sätt som rader i listan Säkerhetskopior: Ctrl+klick, Skift+klick, Ctrl+A, ett långt tryck eller **Markera**. **Återställ** öppnar sedan ett fönster för alla.

- **Vilken säkerhetskopia**: **Rekommenderad** tar för varje post den säkerhetskopia dess eget fönster skulle rekommendera. **Ett tillfälle för alla** tar för varje post dess nyaste säkerhetskopia från ett **Tillfälle** du väljer eller tidigare, till exempel kvällen innan något gick fel.
- **Vart**: **Bredvid originalet** eller **Ersätt originalet**, med **Säkerhetskopiera nuvarande läge först**.

Varje post kontrolleras som en enskild återställning, och sedan körs de en efter en. Varje rad visar hur dess egen gick: **Väntar**, **Körs**, **Återställd**, **Misslyckades** eller **Utelämnad**. **Stoppa efter den här** låter den körande återställningen bli klar och startar inga fler. Flashenheten kommer som en ZIP att ladda ner, och ZFS-datauppsättningar och auto-säkerhetskopian återställs en i taget.

![Återställning av flera poster: en rad per post med sitt förlopp.](assets/screenshots/restore/several-dark.png#only-dark)
![Återställning av flera poster: en rad per post med sitt förlopp.](assets/screenshots/restore/several-light.png#only-light)

## Återställa en stack {#stacks}

Ett klick på en stack i kortet **Stackar** öppnar **Återställ stack**. Den återställer varje medlem från sin senaste säkerhetskopia samtidigt, och med **Starta i beroendeordning efter återställning** startar den dem i deras `depends_on`-ordning: databaser och cachar först, sedan appen. När en medlems projektmapp saknas på den plats du valt erbjuder fönstret läget från Lokal i stället.

BombVault kontrollerar medlemmarna och projektmappen först, och en återställning som inte kunde fungera ändrar ingenting. Annars stoppar den de körande medlemmarna, apparna före deras databaser, skriver sedan projektmappen och återställer sedan medlemmarna. Om projektmappen inte kommer tillbaka återställs medlemmarna ändå men förblir stoppade, eftersom mappen kan vara halvskriven och det är där en medlems bind-monteringar oftast ligger. Varje medlems körning säger det.

En databas som bara har databasdumpar och ingen filsäkerhetskopia, och vars container och namngivna volym är borta, kommer tillbaka utan sina data. Kontrollen säger det före återställningen. Den förblir stoppad, och det gör även containrarna som beror på den, så att ingen app kör sin installation mot en tom databas. Starta den, importera en dump och starta de andra. Se [Databasdumpar](backups.md#database-dumps).

## Andra källor {#other-sources}

- **Från en gammal säkerhetskopiemapp** öppnar repositoriet från en annan BombVault som inte längre körs, på en disk eller på ett externt lagringsmål, med dess återställningskit eller dess inskrivna `APP_KEY`. Sessionen är skrivskyddad och lever bara i minnet. Se [Från en gammal säkerhetskopiemapp](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Importera från pluginet Appdata.Backup** läser pluginets säkerhetskopiemapp. Varje containerarkiv blir en återställningspunkt för sin container, daterad när pluginet gjorde den. Arkiv som importerats tidigare hoppas över, och själva arkiven läses bara. Containern behöver först en säkerhetskopia i BombVault, så att återställningen har dess definition. Gallringen låter importerade återställningspunkter vara, så radera en själv när du inte behöver den längre.
- **Jobb från Duplicati eller Duplicacy** gör om säkerhetskopieringsjobben i de verktygen till mappuppsättningar. Importen läser bara deras inställningar. De gamla säkerhetskopiorna stannar hos det gamla verktyget, och BombVault börjar med en egen första säkerhetskopia. Inget av verktygen behöver vara installerat här.
    - **Från Duplicati**: exportera varje jobb i Duplicati till en fil utan lösenord och välj `.json`-filerna. En export som innehåller lösenorden tar med inloggningen till lagringsmålet.
    - **Från Duplicacy**: välj mappen för Duplicacy Web där `duplicacy.json` ligger, eller en mapp som du säkerhetskopierar med Duplicacys kommandorad och där `.duplicacy` ligger. Nycklar som kommandoraden har sparat i `.duplicacy/preferences` följer med. Duplicacy Web sparar sina inloggningar krypterade, så fönstret frågar efter dem.

    Inget skapas förrän du har sett förhandsvisningen. Varje källmapp blir en mappuppsättning, med mappen på den här servern där den hittades, de undantag som gick att översätta och schemat. Om det gamla verktyget körde i en egen container och en mapp inte hittades, väljer du den själv. Säkerhetskopiorna hamnar på ett nytt lagringsmål i en mapp `bombvault` inuti det gamla lagringsmålet, när det är en lokal mapp, S3, SFTP, en rclone-remote eller Backblaze B2 via dess S3-adress. En inloggning från filen sparas med dina andra inloggningar, och en som saknas i filen blir du tillfrågad om. Anteckningar under varje jobb visar vad som inte följde med, till exempel inkluderingsregler, reguljära uttryck, hur länge säkerhetskopiorna sparades och det gamla krypteringslösenordet, som BombVault inte behöver.

## Efter en återställning {#after}

En kopia som återställts bredvid originalet listas under **Säkerhetskopior** som undantagen, så den räknas inte som en öppen uppgift. Lägg till den i schemat om du vill behålla den. En container eller VM som körde när den säkerhetskopierades kommer tillbaka körande, en som var stoppad förblir stoppad. På Unraid dyker en återställd container upp igen i Docker-fliken som en vanlig, redigerbar app, och en VM i VM Manager med sina diskar och NVRAM kopplade.
