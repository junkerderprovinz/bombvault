# Herstel

Elk herstel begint op de pagina **Herstel**: kies een onderdeel, zoek wat je terug wilt en kies een back-up. Eén venster herstelt elk soort item en vraagt in stappen. Standaard verandert er niets aan het origineel, want de back-up komt ernaast terug.

![De pagina Herstel met de onderdelen als tegels, het zoekveld en de kaart Stacks.](assets/screenshots/restore/page-dark.png#only-dark)
![De pagina Herstel met de onderdelen als tegels, het zoekveld en de kaart Stacks.](assets/screenshots/restore/page-light.png#only-light)

## De pagina Herstel {#page}

- **Onderdelen** toont de onderdelen als tegels, elk met hoeveel items snapshots hebben.
- **Wat wil je terughalen?** doorzoekt de items van het gekozen onderdeel. Elke rij toont zijn nieuwste snapshot, en **Item openen** leidt naar de pagina van het item.
- **Bestand zoeken** zoekt een bestand of map op naam in elke back-up, zie [Een bestand zoeken](#find-a-file).
- **Stacks** toont de Compose-projecten met minstens twee geback-upte containers, zoals `immich` met zijn database en Redis. Het zoekveld vindt een stack op zijn naam of op een van zijn containers.
- **Bezig** toont een herstel terwijl het loopt, ook nadat je zijn venster sloot.
- **Noodgeval** start de Noodassistent voor een server die weg is of opnieuw is ingesteld, zie [Noodassistent](offsite-recovery.md#emergency).
- **Andere bronnen** bevat **Uit een oude back-upmap**, voor de back-ups van een andere BombVault, **Importeren uit de Appdata.Backup-plug-in** en **Taken uit Duplicati of Duplicacy**.
- **Andere servers** toont de andere servers van een gedeelde repository waar deze server bij hoort, zie [Andere servers](#other-servers).

![De pagina Herstel op een telefoon.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![De pagina Herstel op een telefoon.](assets/screenshots/restore/page-phone-light.png#only-light)

## Het herstelvenster {#window}

Een klik op een item opent **Herstel** met zijn naam. Het vraagt in stappen, en elke stap kan worden gewijzigd voordat je begint:

1. **Back-up**: de aanbevolen snapshot is vooraf gekozen. Dat is de nieuwste, of na een openstaande anomalie de laatste daarvóór. De back-up die vóór een eerder herstel is gemaakt, bevat de toestand die dat herstel verving, dus die wordt overgeslagen, tenzij er geen andere is. Oudere staan in de lijst, en **Vergelijken** toont wat er tussen twee verschilt.
2. **Bron**: waar de bestanden vandaan komen, als een snapshot op meerdere plekken staat. **Lokaal** is het snelst.
3. **Wat**: **Alles**, of **Losse bestanden**, waar je de bestanden en mappen aanvinkt die je wilt.
4. **Waarheen**: het doel. De keuzes hangen af van het soort item:

    | Doel | Wat het doet |
    |---|---|
    | **Naast het origineel** | De standaard. Een container komt terug als een tweede container met de naam `<name>-restored` met eigen poorten, de schijven en NVRAM van een VM komen in een map met de naam `<folder>-restored`, en een mappenset komt terug als `<path>-restored`. Er verandert niets aan het origineel. |
    | **Origineel vervangen** | Overschrijft wat er nu staat. Met **Eerst de huidige staat back-uppen**, standaard aan, maakt BombVault eerst een back-up van de toestand van vandaag, zodat je terug kunt. Mislukt die back-up, dan start het herstel niet. Dat werkt ook voor een container of VM die van de server verdwenen is: de mappen die het herstel overschrijft, krijgen een eigen back-up. |
    | **Naar een map** | Alleen de bestanden, om door te bladeren. |
    | **Downloaden als ZIP** | De flash, voor een nieuwe stick. BombVault schrijft nooit in de draaiende flash. |
    | **In een nieuwe dataset** en **Terugzetten in de dataset** | ZFS-datasets, zie [ZFS-datasets](zfs-datasets.md#restore). |

5. **Wat er gebeurt** zegt in woorden wat het herstel gaat doen, en een afbeelding toont het doel dat je koos.

Voordat **Herstellen** ontgrendelt, controleert het venster of het repository antwoordt, of de sleutel het opent, of de snapshot er is en of het doel ruimte heeft. Zolang een controle mislukt, blijft de knop vergrendeld, en de (i) erin zegt welke.

Het venster toont ook wat het herstel verandert ten opzichte van wat er nu staat: nieuwe, vervangen en ongewijzigde bestanden, met de lijst op verzoek, en de bestanden op het doel die niet in de back-up staan en blijven waar ze zijn. restic berekent dit als een droge run op grootte en wijzigingstijd, zonder de bestanden te lezen, en een zeer grote boom stopt na 30 seconden en zegt dat. Voor containers en VM's vergelijkt het de instellingen die het herstel opnieuw aanmaakt met de draaiende: image en tag, poorten, variabelenamen en volumes, of geheugen, vCPU's, schijven en netwerk.

Sommige dingen kunnen een herstel stoppen voordat het iets verandert. Het vaste IP en de gepubliceerde poorten van een container moeten vrij zijn, en zijn netwerk moet bestaan. Op Unraid weigert elk herstel op zijn plaats een map die niet op een gemounte pool of share staat, omdat de bestanden in het geheugen zouden belanden en na een herstart weg zouden zijn. Een herstel op zijn plaats noemt elke andere container waarvan een bind-mount tot in een map reikt waarin het schrijft, bijvoorbeeld "dit pad wordt ook gebruikt door nextcloud-db", maar dat waarschuwt alleen.

Een paar opties verschijnen waar ze van toepassing zijn: **Daarna gestopt laten**, om een container of VM te bekijken voordat hij weer draait, en **Herstellen zonder GPU en runtime** op een host die de GPU of runtime mist waarmee de back-up is gemaakt.

Het herstel loopt op de achtergrond. Je kunt het venster sluiten, en de voortgang staat ook onder **Bezig**. Een lang herstel toont een percentage en kan worden geannuleerd, en een geannuleerd herstel wordt als geannuleerd vastgelegd, niet als mislukt. Een herstel dat wordt geannuleerd terwijl de container stopt, of een herstel dat de container niet kan verwijderen, heeft nog niets hersteld, dus de container start weer. Een container die zijn volumes met `--volumes-from` van een andere neemt, houdt ze, en het herstel begint niet zolang die andere container niet bestaat.

![Het herstelvenster bij de stap Wat, met losse bestanden aangevinkt.](assets/screenshots/restore/window-what-dark.png#only-dark)
![Het herstelvenster bij de stap Wat, met losse bestanden aangevinkt.](assets/screenshots/restore/window-what-light.png#only-light)

### Waar een herstel naar een map terechtkomt {#folders}

**Herstelmappen** onder **Instellingen, Herstel** bepaalt de **Standaard herstelmap**, desgewenst met een **Submap per onderdeel**, en een **Uitzondering** voor één onderdeel. Gekozen bestanden komen rechtstreeks in de doelmap: één bestand komt terug als alleen dat bestand, een map als de map met zijn inhoud, en keuzes uit verschillende mappen houden alleen hun pad onder de map die ze delen. BombVault weigert een doel dat niet op gemounte opslag staat, omdat zo'n map op Unraid in het geheugen leeft en na een herstart weg is, en het herstelt nooit in zijn eigen datamap.

## Voorbeeld: één bestand terughalen {#example-single-file}

Iemand heeft gisteravond het `config.php` van Nextcloud bewerkt en Nextcloud start niet meer. De nachtelijke back-up van 03:00 heeft het goede bestand nog. Je wilt dat ene bestand, niet de hele container.

1. Open **Herstel**, kies de tegel **Containers** en typ `nextcloud` in **Wat wil je terughalen?**.
2. Klik op `nextcloud`. Het venster opent met de back-up van 03:00 vooraf gekozen als de aanbevolen.
3. Laat **Bron** op **Lokaal** staan.
4. Kies onder **Wat** **Losse bestanden**. Open de mappen tot `config` en vink `config.php` aan.
5. Kies onder **Waarheen** **Naar een map** en kies een map, bijvoorbeeld `user/restore`, wat op de host `/mnt/user/restore` is.
6. Lees **Wat er gebeurt**: het bestand komt in de doelmap te staan en er verandert niets aan de container. Druk op **Herstellen**.

Het venster eindigt met **Klaar: de bestanden staan in /mnt/user/restore**. Je vindt het bestand als `/mnt/user/restore/config.php`, zonder de mappen erboven. Vergelijk het met het kapotte bestand, kopieer het terug in de appdata-map en herstart Nextcloud.

Om het bestand in plaats daarvan meteen terug te zetten, kies je in stap 5 **Origineel vervangen**. Laat **Eerst de huidige staat back-uppen** aan, zodat BombVault de kapotte toestand back-upt voordat het schrijft, typ `nextcloud` ter bevestiging en druk op **Herstellen**. Nextcloud wordt gestopt terwijl het gekozen bestand op zijn plek wordt teruggeschreven en daarna weer gestart, en alleen de bestanden die je aanvinkte, worden geschreven. Een herstel dat halverwege mislukt, laat Nextcloud gestopt, omdat het bestand dat het leest half geschreven kan zijn, en de run meldt dat.

![Het herstelvenster bij Wat er gebeurt, met Origineel vervangen gekozen.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![Het herstelvenster bij Wat er gebeurt, met Origineel vervangen gekozen.](assets/screenshots/restore/window-replace-light.png#only-light)

## Een bestand zoeken {#find-a-file}

Weet je de naam van een bestand, maar niet welk item of welke back-up het nog heeft, typ dan de naam in **Bestand zoeken** en druk op **Zoeken**. Gewone tekst past op elke plek in de naam en let niet op hoofdletters, dus `invoice` vindt `Invoice-2024.pdf`. Met `*` of `?` moet het patroon op de hele naam passen, dus `*.pdf` vindt elke pdf. Typ alleen de naam, zonder map.

De zoekopdracht leest elke back-up van de containers, mappensets, ZFS-datasets en de flash. Elke treffer toont het item, het pad en elke back-up waarin het bestand staat, de nieuwste eerst, met de datum, de grootte en de bestemming waar het is gelezen. Een back-up waarin het bestand dezelfde grootte en wijzigingstijd heeft als in de vorige, zegt **ongewijzigd**, zodat je ziet waar het bestand voor het laatst veranderde. Een klik op een back-up opent het herstelvenster daarop, met **Losse bestanden** gekozen en het bestand aangevinkt. Een bestand van de flash komt terug als deel van de ZIP van de flash, omdat de flash alleen als geheel wordt hersteld.

Zoeken duurt even, omdat restic door elke back-up loopt. **Stoppen** beëindigt het, en de pagina verlaten ook. Onder het veld staat welke bestemmingen het las:

- Bestemmingen op deze server en in je netwerk worden altijd doorzocht.
- Een back-up die op meerdere bestemmingen staat, wordt maar één keer gelezen, bij de eerste. Een bestemming waarvan alle back-ups elders zijn gelezen, zegt **al bij een andere bestemming doorzocht**.
- Off-site-bestemmingen zoals S3, B2 of SFTP blijven buiten de zoekopdracht, tenzij je **Ook off-site-bestemmingen doorzoeken** aanzet. Dan worden daar alleen de back-ups gelezen die geen bestemming hier heeft, meestal de oudere die een langere bewaartermijn off-site houdt. De eerste zoekopdracht op zo'n bestemming kan lang duren, en sommige aanbieders rekenen de gedownloade data aan.
- Een bestemming die niet kon worden geopend, zegt **kon niet worden gelezen**, en de (i) ernaast vertelt waarom. De andere bestemmingen worden gewoon doorzocht.

Virtuele machines worden als hele schijven opgeslagen, dus de bestanden erin worden niet doorzocht. De zelf-back-up, de back-ups van een andere BombVault onder **Andere bronnen** en die van de servers onder **Andere servers** worden ook niet doorzocht. Een zoekopdracht toont de eerste 200 bestanden, dus bij een korte naam zoals `log` heb je misschien een paar letters meer nodig.

## Wat er terugkomt, per soort {#kinds}

**Containers** worden opnieuw aangemaakt uit hun opgeslagen definitie via de Docker-API, met hun entrypoint, hostnaam, stopinstellingen, volumes en mounts, resourcelimieten, logdriver, DNS-instellingen en oude links. Een container komt terug op de image waarop hij liep toen de back-up werd gemaakt, niet op een nieuwere die later is opgehaald, en een ontbrekende image wordt eerst opgehaald, met de inloggegevens uit **Container-registry's** onder **Instellingen, Herstel**. Zijn GPU of runtime (`--gpus`, `--runtime=nvidia`) komt ook terug. Op een host zonder dat stuurprogramma of die runtime zegt het herstel dat en biedt het **Herstellen zonder GPU en runtime** aan. Een link naar een container die ontbreekt, of gestopt is terwijl de herstelde start, wordt weggelaten, en de runhistorie zegt dat. Een herstel dat mislukt nadat de container was verwijderd, zet hem terug, gestopt.

Een back-up kan mappen bevatten die de container niet meer geselecteerd heeft, bijvoorbeeld een oudere van voordat een map verdween. Een map die weg of leeg is, komt terug. Een map waar nu bestanden in staan, blijft zoals hij is, omdat je hem misschien met opzet hebt uitgevinkt, en de controle vóór het herstel en de rungeschiedenis noemen hem. Haal hem terug met **Losse bestanden** of **Naar een map**.

**VM's** krijgen hun XML opnieuw gedefinieerd via SSH en hun schijven en UEFI NVRAM opnieuw gekoppeld, zelfs nadat de VM was verwijderd. Zie [VM-back-ups](vm-backup-ssh-setup.md) voor VM's met Unraid-snapshots en schijven op zvols.

**Mappensets** komen terug met hun eigenaar, modi en tijdstempels zoals restic ze opslaat. Waar het doel alleen de eigenaar weigert, zoals `/mnt/user` doet, eindigt de run met een opmerking die dat zegt. BombVault vergelijkt dan de bestanden met de back-up, en een bestand dat niet is aangekomen, laat het herstel mislukken.

**De flash** raakt de draaiende `/boot` nooit aan. Het herstel is een download genaamd `flash-<id>.zip`, klaar om in de Unraid USB Creator te zetten. De ZIP bevat de `APP_KEY`, dus het downloaden vraagt een inlogwachtwoord en een aanmelding op de instantie. Het venster toont ook de plug-ins in elke flashback-up met hun versie en grootte, en kan er één terugzetten in de draaiende flash: zijn `.plg`-bestand, zijn map onder `config/plugins` en de pakketbestanden die de back-up bevat. Unraid installeert hem bij de volgende start, of meteen onder **Plugins, Install Plugin**.

**ZFS-datasets** hebben eigen doelen en behouden hun ZFS-eigenschappen, zie [Herstellen](zfs-datasets.md#restore).

**De zelf-back-up** komt alleen terug via de Noodassistent, die BombVault herstart, zodat de draaiende database nooit onder een open handle wordt overschreven.

**Databasedumps** worden nooit uit zichzelf teruggeladen. Een rij onder **Databasedumps** kan worden gedownload, in een map worden opgeslagen of worden geïmporteerd in een vers gestarte database. Zie [Databasedumps](backups.md#database-dumps).

## Meerdere items tegelijk herstellen {#several}

Kies op de pagina Herstel items zoals je rijen kiest in de lijst Back-ups: Ctrl+klik, Shift+klik, Ctrl+A, lang drukken of **Selecteren**. **Herstellen** opent dan één venster voor alle.

- **Welke back-up**: **Aanbevolen** neemt voor elk item de back-up die zijn eigen venster zou aanbevelen. **Eén moment voor alle** neemt voor elk item zijn nieuwste back-up vanaf een **Moment** dat je kiest of eerder, bijvoorbeeld de avond voordat er iets misging.
- **Waarheen**: **Naast het origineel** of **Origineel vervangen**, met **Eerst de huidige staat back-uppen**.

Elk item wordt gecontroleerd zoals een enkel herstel, en daarna lopen ze na elkaar. Elke rij toont hoe zijn eigen herstel ging: **Wacht**, **Actief**, **Hersteld**, **Mislukt** of **Weggelaten**. **Stoppen na dit item** laat het lopende herstel afmaken en start er geen meer. De flash komt als ZIP om te downloaden, en ZFS-datasets en de zelf-back-up worden één voor één hersteld.

![Meerdere items herstellen: één rij per item met zijn voortgang.](assets/screenshots/restore/several-dark.png#only-dark)
![Meerdere items herstellen: één rij per item met zijn voortgang.](assets/screenshots/restore/several-light.png#only-light)

## Een stack herstellen {#stacks}

Een klik op een stack in de kaart **Stacks** opent **Stack herstellen**. Het herstelt elk lid tegelijk vanaf zijn laatste back-up, en met **Na herstel starten in afhankelijkheidsvolgorde** start het ze in hun volgorde `depends_on`: eerst databases en caches, dan de app. Ontbreekt de projectmap van een lid op de plek die je koos, dan biedt het venster in plaats daarvan de toestand uit Lokaal aan.

BombVault controleert eerst de leden en de projectmap, en een herstel dat niet zou kunnen werken, verandert niets. Anders stopt het de draaiende leden, de apps vóór hun databases, schrijft dan de projectmap en herstelt daarna de leden. Komt de projectmap niet terug, dan worden de leden toch hersteld, maar blijven ze gestopt, omdat de map half geschreven kan zijn en de bind-mounts van een lid er meestal in liggen. De run van elk lid meldt dat.

Een database die alleen databasedumps heeft en geen back-up van bestanden, en waarvan de container en het named volume weg zijn, komt terug zonder zijn data. De controle zegt dat vóór het herstel. Hij blijft gestopt, net als de containers die van hem afhangen, zodat geen app zijn installatie op een lege database uitvoert. Start hem, importeer een dump en start de andere. Zie [Databasedumps](backups.md#database-dumps).

## Andere bronnen {#other-sources}

- **Uit een oude back-upmap** opent het repository van een andere BombVault die niet meer draait, op een schijf of op een off-site-bestemming, met zijn herstelkit of zijn ingetypte `APP_KEY`. De sessie is alleen-lezen en leeft alleen in het geheugen. Zie [Uit een oude back-upmap](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Importeren uit de Appdata.Backup-plug-in** leest de back-upmap van de plug-in. Elk containerarchief wordt een herstelpunt van zijn container, gedateerd op het moment dat de plug-in het maakte. Eerder geïmporteerde archieven worden overgeslagen, en de archieven zelf worden alleen gelezen. De container heeft eerst één back-up in BombVault nodig, zodat het herstel zijn definitie heeft. Het bewaarbeleid laat geïmporteerde herstelpunten met rust, dus verwijder er zelf een als je hem niet meer nodig hebt.
- **Taken uit Duplicati of Duplicacy** maakt van de back-uptaken van die tools mappensets. Het leest alleen hun instellingen. De oude back-ups blijven bij de oude tool, en BombVault begint met een eigen eerste back-up. Geen van beide tools hoeft hier geïnstalleerd te zijn.
    - **Uit Duplicati**: exporteer in Duplicati elke taak zonder wachtwoord naar een bestand en kies de `.json`-bestanden. Een export met de wachtwoorden erin neemt ook de inloggegevens van de bestemming mee.
    - **Uit Duplicacy**: kies de map van Duplicacy Web waarin `duplicacy.json` staat, of een map die je met de opdrachtregel van Duplicacy back-upt en waarin `.duplicacy` staat. Sleutels die de opdrachtregel in `.duplicacy/preferences` heeft opgeslagen, gaan mee. Duplicacy Web bewaart zijn inloggegevens versleuteld, dus het venster vraagt je erom.

    Er wordt niets aangemaakt voordat je het voorbeeld hebt gezien. Elke bronmap wordt een mappenset, met de map op deze server waar hij is gevonden, de uitsluitingen die konden worden omgezet en de planning. Draaide de oude tool in een eigen container en is een map niet gevonden, dan kies je die zelf. De back-ups gaan naar een nieuwe bestemming, in een map `bombvault` binnen de oude bestemming, of dat nu een lokale map, S3, SFTP, een rclone-remote of Backblaze B2 via zijn S3-adres is. Inloggegevens uit het bestand worden bij je andere inloggegevens opgeslagen, en wat er in het bestand ontbreekt, wordt je gevraagd. Notities onder elke taak vertellen wat niet is meegekomen, zoals insluitregels, reguliere expressies, hoe lang back-ups werden bewaard en het oude versleutelingswachtwoord, dat BombVault niet nodig heeft.

## Andere servers {#other-servers}

In een [gedeelde repository](offsite-recovery.md#shared-repository) opent elke sleutel de back-ups van alle servers. Daarom toont de pagina Herstel de andere leden onder **Andere servers**, elk met zijn naam, zijn host, hoeveel items hij heeft en wanneer zijn nieuwste back-up liep. Een host die bij geen server hoort die deze kent, verschijnt als **Een server die deze niet kent**. Een klik opent **Back-ups van** en de naam van de server, alleen-lezen en zonder dat je iets hoeft in te voeren. Mappensets en ZFS-datasets gaan naar een map die je kiest. Containers en VM's komen terug als gewone bestanden in een map, omdat de andere server hun definities met zijn eigen sleutel verzegelt. De eigen back-ups van deze server in de gedeelde repository staan op het tabblad **Back-ups** van elk item.

## Na een herstel {#after}

Een kopie die naast het origineel is hersteld, staat onder **Back-ups** als uitgesloten, zodat ze niet als openstaande taak telt. Voeg haar toe aan de planning als je haar wilt houden. Een container of VM die draaide toen hij werd geback-upt, komt draaiend terug, een die gestopt was, blijft gestopt. Op Unraid verschijnt een herstelde container weer in het Docker-tabblad als een gewone, bewerkbare app, en een VM in de VM Manager met zijn schijven en NVRAM gekoppeld.
