# Off-site en herstel

!!! note "Off-site kopieën wachten na een rebuild"
    Wanneer de Noodassistent of **Back-ups ontdekken** items opnieuw opbouwt zonder de oude instellingen, pauzeren de off-site kopieën van die onderdelen totdat hun standaard is bevestigd. Zie [Na een rebuild](#after-a-rebuild).

Lokale back-ups beschermen je tegen een verloren container of een slechte update. Off-site kopieën en een geteste herstelkit beschermen je tegen de hele machine, ransomware of een brand. Deze pagina behandelt bestemmingen en off-site kopieën, een kopie manipulatiebestendig maken, bewijzen dat je kunt herstellen, meerdere servers koppelen en alles terughalen wanneer BombVault zelf weg is.

## Off-site kopieën {#off-site-copies}

Houd de snelle lokale back-up en voeg een of meer bestemmingen buiten de deur toe. BombVault kopieert nieuwe snapshots daarheen met `restic copy` op best-effort-basis, dus een off-site hapering laat de lokale back-up nooit mislukken. In deze opzet blijft de lokale repo primair en is de off-site repo een kopie, maar de primaire repo van een onderdeel hoeft helemaal niet lokaal te zijn; zie [Externe primaire repositories](#remote-primary-repositories) hieronder voor rechtstreeks back-uppen naar S3, een rest-server en dergelijke in plaats van ernaartoe kopiëren.

- **Meerdere bestemmingen per onderdeel.** Elk onderdeel (containers, VM's, flash, zelf-back-up, mappen en ZFS-datasets) kan tegelijk naar meerdere off-site bestemmingen kopiëren, zodat je bijvoorbeeld een rest-server op de machine van een vriend en een S3-bucket naast elkaar kunt houden. De kaart **Bestemmingen per onderdeel** op de pagina Bestemmingen schakelt elke regel van een onderdeel naar een bestemming in of uit.
- **Wanneer kopieën draaien.** De kaart **Kopiëren** van een bestemming stelt **Wanneer kopiëren** in: **Na elke back-up**, of volgens een eigen plan dat de snapshots verzamelt en op vaste tijden kopieert. Onder **Instellingen, Planning, Off-site-kopieën per domein** geeft **Uitzondering** één onderdeel een eigen kopieerplan. **Nu repliceren** in de kaart **Off-site kopie** van een onderdeel onder **Instellingen, Bestemmingen** start een kopie op aanvraag.
- **Bewaarbeleid** stel je per bestemming in onder **Hoe lang bewaren**, zodat je off-site kopieën als archief langer kunt bewaren. De kopie in het off-site-veld van een onderdeel volgt de kaart **Off-site bewaarbeleid** onder **Instellingen, Bestemmingen**; laat die geheel op nul om off-site snapshots nooit automatisch te trimmen.
- **Bandbreedtelimieten** (**Off-site bandbreedte** onder **Instellingen, Bestemmingen**, of de kaart **Kopiëren** van een bestemming) begrenzen de restic-upload/downloadsnelheid, zodat kopieën je WAN niet verzadigen.
- Terwijl een kopie loopt, zegt de statuskop van het overzicht **Off-site-kopie loopt**. Het is een actieve indicator, geen voortgangsbalk, omdat `restic copy` geen machineleesbare voortgang geeft.

!!! note "Herstel vanaf elke plek"
    Elk item toont zijn back-ups als één tijdlijn over alle plekken waar een back-up ligt, op het tabblad **Back-ups**. Een back-up die naar B2 is gekopieerd, verschijnt één keer, gemarkeerd met elke plek die hem bevat. Een herstel neemt de eerste plek die bereikbaar is, te beginnen met de repository waar het item naartoe wordt geschreven, en je kunt per snapshot een andere plek kiezen. Off-site bestemmingen worden pas gelezen als je op **Controleren** klikt, omdat het opvragen van cloudopslag even duurt. Verwijderen op één plek controleert eerst de andere en zegt of het de laatste kopie was.

## Bestemmingen {#destinations}

Een bestemming is een plek waar back-ups heen gaan: een lokale map, een share, een eigen server of een clouddienst. Je stelt haar één keer in en ze bedient daarna elk onderdeel. Open **Bestemmingen** via zijn instellingentegel, of schakel daar **Ook in de zijbalk tonen** in om hem vast te zetten.

**Bestemming toevoegen** opent een venster in vier stappen:

1. **Aanbieder.** Elke dienst staat met zijn logo in de lijst, met een zoekveld erboven, in groepen: opslagdiensten met S3-buckets (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 en andere), je eigen S3-server (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), je eigen server en shares (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, een gemount pad), cloudopslag (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud en de rest die rclone ondersteunt) en de ontvangservers van je groep. Elke dienst zegt hoe goed ze bij back-ups past: clouddrives vertragen bij veel verzoeken, dus de eerste back-up en het opschonen duren daar langer.
2. **Aanmelden.** De velden hangen van de dienst af: een toegangssleutel voor S3, een gebruiker en wachtwoord voor WebDAV en SMB, een app-wachtwoord waar tweestapsaanmelding het gewone blokkeert, de publieke SSH-sleutel van BombVault voor SFTP en de Storage Box, of een token voor diensten die via een browser aanmelden. Voor die laatste toont het venster een `rclone authorize`-opdracht om op een computer met een browser uit te voeren; het token dat die afdrukt, gaat in het veld. **Testen** controleert de aanmelding voordat er iets wordt opgeslagen. Het venster toont daarna de mappen op de bestemming, met **Nieuwe map** om er een te maken en de vrije ruimte waar de dienst die meldt. Een lege map is het veiligst; BombVault zet er één map per onderdeel in.
3. **Bescherming.** Het venster zegt zonder omwegen wat de dienst kan. Een rest-server in append-only-modus weigert verwijderen, en de tamper-test controleert dat. Een S3-bucket kan oude versies bewaren met versiebeheer en objectvergrendeling, wat BombVault nog niet kan controleren. Een clouddrive kan verwijderen helemaal niet weigeren: wie in de server komt, komt ook bij die kopie. Zet **Bescherming tegen verwijderen (append-only)** alleen aan waar de andere kant verwijderen echt weigert; BombVault schoont daar dan nooit op. **Voor noodgevallen** zegt wat de herstelkit voor deze bestemming bevat.
4. **Voor welke onderdelen.** Kies de onderdelen die hier heen back-uppen. De eerste kopie neemt de hele geschiedenis mee. Items die hun eigen bestemmingen hebben gekozen, krijgen de nieuwe niet vanzelf, en het venster noemt ze.

S3-diensten lopen via restic's eigen S3-backend, en daardoor kunnen een opslagklasse en objectvergrendeling gelden. Elke andere dienst loopt via de rclone die met BombVault wordt meegeleverd, en haar remote verschijnt dan in de kaart **Off-site (rclone)** onder **Instellingen, Bestemmingen**. Een export van instellingen bevat de bestemmingen; met inloggegevens erbij bevat hij ook hun aanmelding.

Elke bestemming heeft een eigen pagina:

- **Verbinding** met zijn naam, de schakelaar **Beschikbaar** (uit betekent dat BombVault hier niet meer schrijft, en items die hierheen back-uppen mislukken met een duidelijke melding in plaats van ergens anders te belanden), de map, compressie, opslagklasse en **Buiten de locatie**. **Verbinding wijzigen** test en bewaart een nieuw adres of een nieuwe aanmelding.
- **Inloggegevens**: de gedeelde set of een eigen set.
- **Bescherming tegen verwijderen**, met **Bescherming tegen verwijderen nu testen** en wie deze kopie opschoont.
- **Kopiëren**: wanneer er wordt gekopieerd, en de upload- en downloadlimieten.
- **Hoe lang bewaren**: **Kort**, **Gebalanceerd**, **Lang** of **Eigen waarden**, met een tijdlijn van welke back-ups de regel behoudt. **Vragen voor minder bewaren** vraagt bevestiging voordat een kortere regel back-ups wist waarvan de enige kopie hier ligt.
- **Regels per onderdeel**: de eigen repository, het eigen bewaarbeleid of de eigen compressie van één onderdeel bij deze bestemming. **Regels wijzigen** opent ze, en **Terug naar de bestemming** zet een onderdeel terug.
- **Staat al onder deze bestemming**: onderdelen waarvan het met de hand getypte adres in een map van deze bestemming ligt. Er een overnemen laat de repository waar hij is; aanmelding, opslagklasse en bescherming tegen verwijderen komen dan van de bestemming. BombVault controleert eerst of de aanmelding van de bestemming de repository opent, en weigert een append-only repository onder een bestemming te zetten die niet append-only is.
- Een vat dat laat zien hoe vol de bestemming is, met de markering waar ze vol is en hoeveel weken er nog zijn bij de groei van de laatste weken. Een bestemming die haar grootte niet meldt, zegt **Grootte onbekend**.

Een ontvangserver die een andere instantie van je groep draait, verschijnt onder de aanbieders als **Uit je groep**; zie [Ontvangserver](#receiving-server).

### Verplaatsen en verwijderen {#move-remove}

- **Verplaatsen** brengt de kopie van één onderdeel met zijn hele geschiedenis naar een andere bestemming. Alles blijft op de oude plek staan; BombVault wist daar niets. De verplaatsing draait op de achtergrond en kan lang duren: ongeveer 9 minuten per 100 GB tussen lokale schijven, uren over een trage verbinding. Het overzicht toont haar zolang ze loopt.
- **Bestemming verwijderen** biedt twee wegen. **Bestemming verwijderen, back-ups bewaren** stopt met schrijven daar en laat de back-ups staan; je kunt ze later lezen onder **Herstel, Uit een oude back-upmap**. **Bestemming verwijderen en de back-ups daar wissen** leest eerst wat daar is opgeslagen en toont het: welke back-ups verdwijnen, welke daarvan nergens anders staan, en welke repository's blijven omdat bescherming tegen verwijderen het weigert, items er nog rechtstreeks naartoe back-uppen, een andere bestemming of een ander onderdeel er zijn kopieën bewaart of ze niet gelezen konden worden. Items die hun enige back-up zouden verliezen, staan apart en blijven bewaard tenzij je **Wis deze ook** aanvinkt. Je bevestigt door de naam van de bestemming te typen, en het wissen draait op de achtergrond.
- Het wissen van de back-ups op een bestemming vraagt een inlogwachtwoord en een aanmelding op de instantie. Zonder wachtwoord kan de bestemming nog wel worden verwijderd, alleen kunnen haar back-ups niet vanaf hier worden gewist.

## Plaatsing per item {#placement}

De kaart **Planning en bestemming** op het tabblad Instellingen van een item heeft een rij knoppen: **Lokaal** en één knop per off-site bestemming van het onderdeel. Verlichte knoppen krijgen de back-ups van het item.

- Met **Lokaal** verlicht wordt het item geschreven naar de repository die onder **Opgeslagen op** staat en gekopieerd naar elke andere verlichte bestemming. Maak een bestemming donker en ze krijgt niets nieuws meer van dit item. Alleen Lokaal kopieert nergens heen, wat past bij data die al een tweede kopie heeft, bijvoorbeeld een share die op een NAS leeft.
- Met **Lokaal** donker wordt het item rechtstreeks geschreven naar de directe repository van de eerste verlichte bestemming en vanaf daar gekopieerd naar de andere verlichte. De eerste keer maakt een dialoog die directe repository aan.
- Eén knop blijft verlicht, omdat een back-up een plek nodig heeft. Om een item van back-ups uit te sluiten, gebruik je **Niet back-uppen**.

De locatie ligt vast vanaf de eerste back-up van het item, omdat BombVault nooit back-ups tussen repository's verplaatst; de kaart **Repository** op het tabblad Instellingen noemt haar. De kopieën kunnen op elk moment veranderen. Een bestemming die een item niet meer krijgt, houdt de kopieën die ze heeft en trimt ze bij de eerstvolgende off-site run van het onderdeel naar haar eigen bewaarbeleid. Van append-only bestemmingen kan niet worden gewist.

Onder de rij zegt de kaart waar het item heen gaat en wat er werkelijk is: hoeveel locaties het bevatten, wanneer elke bestemming het laatst is gezien en of aan 3-2-1 is voldaan. Een locatie is de server met de originele data, elke off-site bestemming en elke repository die als **Buiten het pand** is gemarkeerd. BombVault controleert kopieën en locaties; het controleert het onderdeel "twee media" van 3-2-1 niet.

### Standaard per onderdeel {#placement-defaults}

De kaart **Bestemmingen per onderdeel** op de pagina Bestemmingen is de standaard van elk onderdeel. In de grafiek is elke lijn een kopie, en een klik op een lijn of een stip schakelt haar in of uit; gestreepte lijnen staan uit. **Grafiek / Schakelaars** toont hetzelfde als rijen knoppen. De kopieën gelden meteen voor elk item zonder eigen keuze en voor de projectmappen van Compose-stacks. De locatie geldt voor een nieuw item bij zijn eerste back-up; haar wijzigen verplaatst geen back-ups. Vóór het opslaan noemt BombVault elke bestemming die items wint of verliest en hoeveel snapshots dat betekent.

Items die ergens anders heen back-uppen dan hun onderdeel, staan in de kaart **Eigen keuze**, elk met **Standaard volgen**. Nieuwe back-ups gaan dan naar de bestemmingen van het onderdeel, en de eerdere blijven waar ze zijn.

Een nieuwe off-site bestemming krijgt elk item dat niet op Lokaal staat. Het venster dat haar toevoegt, zegt hoeveel items en, waar bekend, hoeveel geschiedenis dat is.

### Directe repository's

Lokaal uitzetten voor een item, zodat een bestemming zonder directe repository zijn thuis wordt, opent een dialoog met een voorgestelde locatie naast die bestemming, bijvoorbeeld `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, en een verbindingstest die niets aanmaakt. **Aanmaken en gebruiken** maakt de repository aan en wijst het item ernaar. Een directe repository neemt de sleutel, opslagklasse, limieten, append-only-instelling en het bewaarbeleid van de bestemming over en verandert mee; de kaart Repository's toont hem alleen-lezen. Kan een nieuwe sleutel van de bestemming hem niet openen, dan houdt de directe repository de sleutel die hij heeft en zegt het opslaan dat. Een item op een directe repository wordt vanaf daar gekopieerd naar de andere verlichte bestemmingen, nooit naar de bestemming waartoe de repository behoort. Zijn snapshots dragen de tag `bv:direct`, en elke andere bewaarronde laat ze staan, zodat een directe repository die zijn koppeling met zijn bestemming kwijt is, nooit veroudert volgens de lokale regels. B2 bereik je via zijn S3-eindpunt, met de sleutel-ID en de toepassingssleutel als S3-inloggegevens; een sleutel die tot de eigen map van de bestemming is beperkt, bereikt de map ernaast niet, dus beperk de sleutel in plaats daarvan tot de map erboven.

### Buiten het pand

Een bestemming of een benoemde repository kan worden gemarkeerd als **Buiten de locatie**. Externe staan standaard gemarkeerd; zet het uit voor een rest-server in hetzelfde gebouw. De markering telt alleen locaties en 3-2-1 op de kaarten. Ze verandert geen kopie.

Een ZFS-replica is een kopie buiten het pand, maar geen back-up met geschiedenis. Ze bewaart alleen de snapshots die je instelt, en oude versies en losse bestanden komen nog steeds uit de back-ups. Ze gaat van de ene ZFS-server rechtstreeks naar de andere, via SSH naar een ZFS-server of naar het ontvangstpunt van een gekoppelde instantie. De koppelingsgroep draagt alleen het verzoek. Zie [Replica](zfs-datasets.md#replica).

### Na een rebuild {#after-a-rebuild}

Kopieerkeuzes leven in de eigen instellingen van BombVault. Na een rebuild via **Back-ups ontdekken** of de Noodassistent zonder teruggezette instellingen zijn ze weg, en alles kopiëren zou de items die je had uitgesloten opnieuw naar B2 sturen. Off-site kopieën van elk herbouwd onderdeel pauzeren daarom. Het overzicht zegt het, en de kaart **Bestemmingen per onderdeel** biedt **Standaard bevestigen** met een voorbeeld van wat de volgende run kopieert en de namen in de back-ups die geen item hebben, die je daar kunt uitsluiten. Alleen de bevestiging beëindigt de pauze; een instellingenbestand importeren brengt regels en standaarden terug maar beëindigt haar niet.

## Externe primaire repositories {#remote-primary-repositories}

Het eigen pad van een onderdeel is niet beperkt tot een lokale map: wijs het rechtstreeks naar een restic-remote (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`) en BombVault back-upt er rechtstreeks naartoe, zonder aparte lokale kopie en zonder kopieerstap. Dit is een andere opzet dan de off-site kopieën hierboven: daar is de lokale repo primair en de off-site repo een best-effort-archief ervan; hier **is** de remote repo de primaire, en ze is de enige kopie tenzij je dat onderdeel ook naar een andere bestemming kopieert.

Het pad van elk onderdeel (Containers, VM's, Flash, Zelf-back-up, Mappen, ZFS-datasets) stel je in op de pagina van de bestemming **Lokale map**: **Wijzigen** naast het onderdeel in haar kaart **Verbinding** opent een venster met een schakelaar **Lokaal / Extern**:

- **Lokaal** toont de vertrouwde mapkiezer.
- **Extern** vervangt hem door een gewoon URL-veld en de verbindingsinstellingen die een off-site bestemming gebruikt, hier voor deze primaire ingesteld. Daarvandaan krijg je:
    - **Een verbindingstest** tegen het live pad, voordat je erop vertrouwt.
    - **Bandbreedtelimieten** (upload/download) zodat een geplande back-up naar een externe primaire repo je WAN niet verzadigt: dezelfde restic-vlaggen `--limit-upload`/`--limit-download` die off-site kopieën gebruiken, toegepast op de back-up zelf.
    - **Append-only-bescherming**, geverifieerd met dezelfde actieve tamper-test (een echte DELETE-sonde tegen de andere kant) die off-site bestemmingen krijgen. Staat die aan, dan weigert BombVault de repo zelf op te schonen, omdat er geen aparte lokale kopie achter zit en de inloggegevens op deze machine de enige kopie van de back-up niet mogen kunnen wissen.
    - **Een groeibudgetalarm**, bemonsterd uit dezelfde trend van de repogrootte die de kaart Opslag al bijhoudt.

Niets hiervan is verplicht: een met de hand getypt extern pad zonder opgeslagen veiligheidsinstellingen back-upt onbeperkt, kan worden opgeschoond en heeft geen budgetalarm. De veiligheidsinstellingen zijn er voor wanneer je de bescherming van een off-site kopie wilt, zonder een aparte bestemming alleen daarvoor.

!!! note "Cloud-/REST-inloggegevens worden gedeeld"
    Een externe primaire repo meldt zich aan met de gedeelde cloud-inloggegevens onder **Instellingen, Bestemmingen**; er is geen aparte inloggegevensopslag voor primaire repo's.

### SMB en WebDAV zonder host-mount {#smb-webdav}

De kaart **Off-site (rclone)** onder **Instellingen, Bestemmingen** heeft een formulier voor een Windows- of Samba-share en voor een WebDAV-server (Nextcloud, ownCloud, SharePoint of een andere). Vul een korte naam, de host en share (SMB) of de URL en het servertype (WebDAV), de gebruiker en het wachtwoord in, en BombVault schrijft de rclone-sectie voor je. rclone verhult het wachtwoord zelf voordat het wordt opgeslagen; een bestemming toevoegen met een naam die al bestaat, vervangt die sectie in plaats van een tweede toe te voegen. **Bestemming toevoegen** biedt SMB en WebDAV als aanbieders met dezelfde velden.

Het formulier antwoordt met de afgewerkte locatie, bijvoorbeeld `rclone:nas:backups`. Gebruik haar als pad van een onderdeel of als bestemming en voeg een submap toe als je er een wilt (`rclone:nas:backups/bombvault`). De share is het eerste padsegment, geen deel van de naam.

Dit is de betere route dan de share op Unraid te mounten: restic raadt af een repository op een gemounte CIFS-share te bewaren, en hier wordt niets gemount. NFS zit niet in het formulier omdat noch restic noch rclone een NFS-backend heeft; mount voor NFS de export op de host en gebruik hem als lokale map.

## Bescherming tegen verwijderen (append-only)

Zet bescherming tegen verwijderen aan voor een off-site bestemming, zodat ransomware of een gecompromitteerde host je back-ups niet kan wissen of herschrijven. De andere kant (een `restic/rest-server` die in `--append-only`-modus draait) **dwingt** het af. BombVault **verifieert** het alleen en toont nooit groen op basis van een configuratieclaim alleen. Een map op deze server kan verwijderen niet weigeren, omdat wie BombVault beheert ook de map beheert, dus lokale bestemmingen bieden de schakelaar niet aan.

De snelste weg naar een append-only kopie is een gekoppelde BombVault met een [ontvangserver](#receiving-server): die draait de rest-server voor je, in append-only-modus.

!!! note "Een geslaagde verwijdering onder `/locks/` is verwacht"
    Append-only betekent niet dat er nooit iets kan worden verwijderd. restic moet zijn eigen locks nemen en vrijgeven, dus `/locks/` blijft per ontwerp beschrijfbaar en verwijderbaar. Snapshots en de data erachter, waar ransomware op uit is, kunnen niet worden verwijderd. Als je de andere kant zelf test, is een verwijdering die onder `/locks/` slaagt correct gedrag en geen gat in de bescherming.

!!! warning "Append-only repo's worden nooit vanaf deze machine geprund"
    Een append-only bestemming schoont oude snapshots nooit vanaf hier op; opschonen is het werk van de andere kant. Stel er een **groeibudgetalarm** voor in, zodat je een melding krijgt voordat de repogrootte op hol slaat.

## Tamper-test

BombVault bewijst de append-only-garantie periodiek door daadwerkelijk een verwijdering te proberen tegen de off-site repo, gericht op een niet-bestaand object. **Bescherming tegen verwijderen nu testen** op de pagina van een bestemming draait de test op aanvraag, en **Verwijderbeveiliging controleren** onder **Instellingen, Verificatie, Automatische controles** draait haar volgens een schema.

- **Geweigerd** betekent beschermd.
- **Geaccepteerd** betekent niet beschermd.
- Een **niet-eenduidig** resultaat (server onbereikbaar, authenticatiefout) draait het opgeslagen oordeel nooit om.

Een echte omslag van beschermd naar onbeschermd geeft één melding.

## DR-oefeningen

BombVault biedt twee niveaus van bewijs dat je back-ups echt herstelbaar zijn, niet alleen aanwezig.

- **Herstelverificatie-oefeningen (lokaal).** BombVault draait periodiek `restic check --read-data-subset` (begrensd, nooit een schijfvullend volledig herstel). Het ritme staat onder **Instellingen, Verificatie, Automatische controles**, en de kaart **Back-ups controleren** op dezelfde tegel toont het resultaat.
- **Testherstel (off-site).** BombVault herstelt een echt doel uit de off-site repo naar een afgeschermde sandbox, verifieert het bestand voor bestand en byte voor byte, en ruimt dan op. Dit bewijst dat je vanuit off-site kunt herstellen, niet alleen dat de repo antwoordt.

De kaart **Beschermingsstatus** in het overzicht vat dit per onderdeel samen in vier controles: een actuele back-up op deze server, een kopie off-site, bescherming tegen verwijderen daar en een testherstel vanaf daar. Een klik op een onderdeel toont de details, en de kaart wordt alleen groen op geverifieerde feiten.

## Koppeling van instanties {#pairing}

Meerdere BombVaults werken samen als één groep: ze tonen elkaars beschermingsstatus, nemen rollen voor elkaar op zich en kunnen elkaars sleutel bewaren. Een instantie sluit zich met twaalf woorden bij de groep aan.

Open op de eerste instantie **Instellingen, Koppeling** en druk op **Frase genereren**. Drie stapkaarten laten zien hoe het gaat. Twaalf woorden verschijnen in een venster met een knop **Kopiëren**. Open op elke andere instantie dezelfde plek, druk op **Frase invoeren** en plak of typ ze, of druk in dat venster op **Plakken**. Een woord dat niet op de lijst staat, wordt tijdens het typen met zijn positie genoemd, en het laatste woord bevat een controlesom, dus een verkeerd getypt of verwisseld woord wordt gevangen voordat er iets koppelt. Genereer de frase op één instantie: twee instanties die er allebei een genereren, vormen twee aparte groepen. Als er een minuut lang niemand verschijnt, biedt de pagina twee uitwegen: de woorden opnieuw tonen om ze daar in te voeren, of de woorden van de andere instantie invoeren en in één stap bij haar groep komen. Koppelen werkt zonder inlogwachtwoord, maar stel er een in: zonder kan iedereen die de webinterface kan openen de woorden lezen en via de groep rollen vragen die het restic-wachtwoord van de instanties daarin uitgeven. De koppelingskaart zegt dat totdat er een wachtwoord is ingesteld. Met een wachtwoord vraagt het opnieuw tonen van de frase erom. **Groep verlaten** haalt een instantie er weer uit.

Wie de woorden kent, kan zich bij de groep aansluiten, dus behandel ze als een wachtwoord.

**Hoe leden elkaar bereiken.** Elke instantie leert haar eigen adres op het netwerk van je browser zodra je je aanmeldt; het staat onder **Bekende adressen** op de koppelingspagina samen met de adressen die de instantie zelf vond; corrigeer het daar als een reverse proxy of een ongebruikelijke poort ervoor zit. Op hetzelfde netwerk kondigen leden dat adres per multicast aan en praten ze rechtstreeks, en waar multicast een containernetwerk niet kan oversteken, zoals Dockers standaard bridge, zoekt een instantie in plaats daarvan haar eigen subnet af naar de anderen met een ondertekende aanroep die alleen een groepslid kan beantwoorden, zodat koppelen ook zonder relay in seconden klaar is. Als er niets opduikt, neemt **Vind je hem niet?** één adres met de hand, voor een ander subnet of een niet-standaardpoort. Instanties op verschillende netwerken gaan via een relay, gekozen op dezelfde pagina; elk van de vier vormen komt met een plaatje:

- **Projectrelay** (de standaard): `parleyport.halleluja.design`, de relay die ook KnightLoader gebruikt. Niets in te stellen.
- **Eigen relay**: de container [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) uit de Unraid Community Apps (ook onder **Instellingen, Apps**), of een van je instanties die al van buiten bereikbaar is met **Als relay dienen** ingeschakeld. Die instantie antwoordt dan op `/relay/connect` op haar eigen adres, achter de reverse proxy en het certificaat die ze al heeft, en laat alleen je groep binnen. Voer het adres van de relay in op elke instantie die haar moet gebruiken.
- **Geen relay**: leden vinden elkaar automatisch op hetzelfde netwerk, en nergens anders.

**Wat de relay ziet.** Elke aanroep tussen leden is verzegeld met AES-256-GCM onder een sleutel die uit de twaalf woorden is afgeleid, en die sleutel verlaat je instanties nooit. De relay leert een hash die de verbindingen groepeert, voor welke instantie een bericht is, hoe groot het is en wanneer het passeert. Een directe aanroep op het lokale netwerk is op dezelfde manier verzegeld en bovendien ondertekend, dus niets hangt af van het zelfondertekende certificaat dat een instantie aanbiedt. Aanroepen die iets uitgeven (rolantwoorden, een restic-wachtwoord, logins van ontvangers, bewaarkopieën van de sleutel) zijn bovendien ondertekend met een sleutel van het lid dat ze verstuurt, bij eerste ontmoeting vastgepind; zie [Beveiligingsmodel](configuration.md#security-model).

**Wat er over de groep reist.** De beschermingsstatus op de pagina Instanties, een verzoek om één onderdeel nu te controleren, rolverzoeken en hun antwoorden, opslagaanbiedingen, en wat een ontvanger of ophaler nodig heeft zodra zijn rol is toegestaan: de repositorylocaties van de andere instantie en haar restic-wachtwoord. Back-updata reist nooit mee; die gaat nog steeds rechtstreeks naar de restic-backends. De APP_KEY ook niet: het restic-wachtwoord opent de repository's van die instantie en niets anders, niet haar opgeslagen geheimen, sessies of herstelcodes.

**Items van vóór het koppelen.** Instanties die met een fleet-token zijn toegevoegd, en ontvangen repository's en pull-bronnen die met de APP_KEY van de andere instantie zijn ingesteld, blijven na de update bestaan en zijn gemarkeerd met **Opnieuw koppelen**. Ze blijven werken: bij de eerste start vervangt BombVault elke opgeslagen APP_KEY door het restic-wachtwoord dat eruit is afgeleid. Koppel beide instanties, bewerk dan het item en kies zijn instantie. Tot dan staan zulke items in de kaart **Zonder instantie** op de pagina Instanties.

De enige plek die nog een APP_KEY met de hand vraagt, is [Uit een oude back-upmap](#restore-from-another-bombvault-repo), voor het geval dat de andere instantie weg is en in een groep niet kan antwoorden.

## Instanties en rollen {#roles}

De pagina **Instanties** (een instellingentegel die aan de zijbalk kan worden vastgezet) toont de groep als een net van wie wat voor wie doet, een kaart voor deze instantie en een kaart voor elk ander lid, de Android-telefoons en de ZFS-servers. **Instantie koppelen** zweeft rechtsonder. Welke onderdelen aanstaan, stel je in onder **Instellingen, Instanties, Meerdere servers**: **Instanties**, **Ontvanger** en **Ophalen**.

Elke kaart toont de beschermingsstatus van het lid en zijn rollen als tegels. Een rol is iets wat de ene instantie voor de andere doet:

- **Ontvanger** bewaart de off-site kopieën van deze server in een rest-server in append-only-modus. Deze server verstuurt na elke back-up en kan daar niets verwijderen, ook niet als iemand hem overneemt.
- **Ophaler** haalt de back-ups van deze server op volgens zijn eigen schema. Deze server hoeft daarvoor niet wakker te zijn.
- **ZFS-server** neemt de ZFS-replica's van deze server aan. Elk ZFS-item kiest hem onder **Replica** in zijn instellingen.
- **Sleutelbewaarder** bewaart een verzegelde kopie van de herstelkit van deze server. Zie [Sleutelbewaarder](#key-keeper).

Een rol inschakelen verstuurt een verzoek, en er begint niets totdat iemand op de andere instantie op **Toestaan** drukt. Het verzoek verschijnt op de kaart van die instantie als badge, en haar **Details** tonen het met **Toestaan** en **Afwijzen**; een toegestane rol kan daar op elk moment worden ingetrokken. Een gestreepte lijn in het net is een rol die nog wacht. Een instantie op BombVault 9.x kan een verzoek niet beantwoorden, dus een rol is daar actief zodra je hem inschakelt, en een verzoek van een 9.x-instantie wacht hier op **Toestaan**.

**Details** op een kaart opent de rest: de ontvangen repository's en pull-bronnen van dat lid, opslagaanbiedingen, bewaking, **Verwijderen** en **Sleutel vergeten**.

## Ontvanger en ophaler {#receiver}

![Een ontvangen off-site kopie in de details van een instantie, alleen-lezen bewaakt, met een integriteitscontrole op deze hardware.](assets/screenshots/receiver.png)

*Een ontvangen off-site kopie in de details van een instantie, alleen-lezen bewaakt, met een integriteitscontrole op deze hardware.*

Op de machine die append-only kopieën van een andere BombVault **ontvangt**, geven de ontvangen repository's je onafhankelijke, alleen-lezen bewaking op de ontvangende hardware, zodat een stil falen aan de andere kant niet onopgemerkt blijft. Schakel **Ontvanger** in onder **Instellingen, Instanties**, open dan de **Details** van de verzendende instantie op de pagina Instanties en voeg de ontvangen repository toe (alleen-lezen, geopend met het restic-wachtwoord van de verzendende instantie, dat ze via de [koppelingsgroep](#pairing) krijgt). Je krijgt:

- **Een inventaris per bron**, zodat je precies kunt zien welke containers, VM's en mappensets zijn aangekomen.
- **Laatst ontvangen** per bron, zodat je weet hoe vers elk is.
- **Een onafhankelijke `restic check`** die op de ontvangende hardware draait, zodat integriteit wordt geverifieerd waar de data werkelijk staat, niet alleen bij de afzender.
- **Een dead-man's switch:** een waarschuwing wanneer een bron stopt met versturen binnen een venster dat je instelt.
- **Integriteitswaarschuwingen:** een waarschuwing wanneer een controle aan de ontvangende kant mislukt.

De ontvanger is strikt alleen-lezen. Hij schrijft nooit naar de ontvangen repository, dus hij kan de append-only-garantie waarop de afzender rekent nooit breken.

Een ophaler werkt andersom: met **Ophalen** ingeschakeld leest een instantie de back-ups van een ander lid volgens haar eigen schema in haar eigen repository. De bronnen ervan staan ook in de details van dat lid.

### Ontvangserver {#receiving-server}

De ontvangende machine kan ook de rest-server draaien waarnaar de anderen kopiëren. Hij wordt ingesteld de eerste keer dat je een **Ontvanger**-verzoek toestaat dat erom vraagt: het venster zegt dat de server een ontvangserver nodig heeft en vraagt om een map op een share, met **Nieuwe map** om er een te maken, en een poort (8000 tenzij een andere container hem gebruikt). BombVault:

1. weigert als er al een container met de naam `rest-server` bestaat of een andere container de poort bezet;
2. haalt `restic/rest-server` op en start die via de Docker-socket in append-only-modus met privé-repository's en een loginbestand in de map;
3. schrijft de Unraid-template naar de flash, zodat de container bewerkbaar blijft in het tabblad Docker, of biedt de template als download aan wanneer de flash buiten bereik is;
4. voert de tamper-test ertegen uit en toont of hij verwijderen weigert.

De instanties van je groep vinden de server daarna onder de aanbieders bij **Uit je groep**, genoemd naar de ontvangende machine, en de rol **Ontvanger** biedt hem aan als de veilige keuze. Elke instantie krijgt de eerste keer dat ze de server gebruikt een eigen login en schrijft daar alleen in haar eigen map. Met **Ontvanger** ingeschakeld onder **Instellingen, Instanties** opent **Ontvangserver** op de kaart van deze instantie de server opnieuw. Zijn venster toont de logins onder **Partners met een login**, en **Login intrekken** haalt er een weg; wat die instantie al heeft gekopieerd, blijft in de map. De instelling maakt ook één **Login buiten de groep**, waarvan het venster het wachtwoord één keer toont.

Een instantie die de ontvangende machine alleen via de relay bereikt, kan de server niet gebruiken, omdat de relay geen back-ups draagt. Voeg eerst het adres van de ontvangende machine toe op de koppelingspagina. Als BombVault op een eigen IP-adres draait (bijvoorbeeld op br0), vul dan **Adres voor partners** in, omdat de server op het adres van de host luistert.

## Uitgewerkt voorbeeld: twee Unraid-machines, van begin tot eind

Alles hierboven beschrijft de onderdelen. Dit is één volledige opzet, omdat de onderdelen makkelijker te combineren zijn als je ze één keer gecombineerd hebt gezien.

Twee machines: **TOWER** draait de containers en pusht back-ups; **VAULT** ontvangt ze en dwingt append-only af. Vervang door je eigen namen, adressen en sharepaden.

**1. Koppel de twee machines.** Open op TOWER **Instellingen, Koppeling**, druk op **Frase genereren** en voer de twaalf woorden op VAULT in onder **Frase invoeren**. Stel eerst op beide een inlogwachtwoord in.

**2. Vraag op TOWER aan VAULT om ontvanger te zijn.** Schakel op de pagina Instanties **Ontvanger** in op de kaart van VAULT, kies de onderdelen en **de ontvangserver van VAULT** als plek, en verstuur het verzoek.

**3. Sta het op VAULT toe.** De pagina Instanties van VAULT toont het verzoek op de kaart van TOWER. Druk op **Toestaan**. De eerste keer vraagt VAULT om een map voor zijn ontvangserver, zoals `user/appdata/rest-server`, start de rest-server in append-only-modus en voert de tamper-test ertegen uit.

**4. Terug op TOWER.** Vanaf dan kopieert TOWER na elke back-up naar VAULT, en de nieuwe bestemming verschijnt op de pagina Bestemmingen van TOWER met haar bescherming tegen verwijderen.

Wat de tamper-test antwoordt:

| Resultaat | Wat er gebeurde |
| --- | --- |
| **beschermd** | VAULT weigerde de verwijdering. Dit is de enige geslaagde toestand. |
| **niet beschermd** | VAULT accepteerde een verwijdering. `--append-only` ontbreekt of is verwijderd. |
| **niet-eenduidig** | Geen van beide. Meestal is de URL niet die welke restic zelf gebruikt, of zijn de inloggegevens veranderd. Er wordt niets vastgelegd en er gaat geen melding af. |

**5. Bekijk wat er aankomt, als je wilt.** Schakel op VAULT **Ontvanger** in onder **Instellingen, Instanties**, open de **Details** van TOWER en voeg de ontvangen repository toe. VAULT krijgt het restic-wachtwoord van TOWER via de groep; niemand typt een sleutel.

**6. Maak het wederzijds.** Herhaal de stappen in de andere richting: VAULT vraagt TOWER om zijn ontvanger te zijn. Elke machine dwingt dan append-only af voor de andere, en geen van beide kan de back-ups van de ander wissen.

Een rest-server die je met de hand draait, werkt ook: voeg hem toe met **Bestemming toevoegen** als rest-server, met de gebruiker en het wachtwoord uit zijn `.htpasswd`, en schakel bescherming tegen verwijderen in. Houd `--append-only` in zijn opties; zonder is de andere kant weer een gewone share.

!!! warning "Een lokaal pad is een pad binnen de container"
    Waar BombVault om een map op deze server vraagt, voer je haar relatief aan de host-mount in, zoals `user/appdata/rest-server/bombvault-containers`, niet `/mnt/user/appdata/...`. BombVault draait in een container, waar de `/mnt` van de host elders is gemount; een absoluut hostpad bestaat daarin niet. Plak je er een, dan zegt BombVault welk relatief pad je in plaats daarvan gebruikt.

## Noodassistent {#emergency}

De **Noodassistent** haalt een server terug die weg is of opnieuw is ingesteld. Hij begint op de pagina **Herstel** onder **Noodgeval**, en bij **Herstellen vanaf een back-up** op de welkomstpagina van een nieuwe installatie. Hij verloopt in vier stappen: **Toegang**, **Instellingen**, **Selectie** en **Herstel**.

1. **Toegang.** Laat BombVault de sleutel van de oude back-ups zien, op een van drie manieren:
    - **Herstelkit uploaden**: het bestand dat je tijdens de installatie hebt gedownload. Het bevat de sleutel en elke bestemming, en BombVault vult de rest daaruit in. Een kit die met age is verzegeld, vraagt om de identiteit waarvoor hij is verzegeld; die wordt één keer gebruikt en niet opgeslagen.
    - **Via de groep**: een instantie van je groep bewaart de sleutel als bewaarkopie. Je hebt de twaalf woorden van de groep nodig, je vrijgavecode en iemand bij die instantie. Zie [Sleutelbewaarder](#key-keeper).
    - **Sleutel intypen**: de `APP_KEY` van de oude server, en waar zijn back-ups staan.

    BombVault probeert de sleutel dan op elke bestemming en zegt voor elke of ze leesbaar is, niet gevonden, met een andere sleutel vergrendeld, niet bereikbaar of nog zonder aanmelding. Er is nog niets opgeslagen, en de sleutel blijft beperkte tijd in het geheugen. **Sleutel overnemen** laat deze server voortaan zijn back-ups met die sleutel schrijven; **Overname ongedaan maken** gaat terug naar zijn eigen sleutel totdat de eerste back-up heeft gedraaid. Een server die al eigen back-ups heeft, weigert de overname en verwijst in plaats daarvan naar [Uit een oude back-upmap](#restore-from-another-bombvault-repo). **Inloggegevens opslaan** bewaart de aanmeldingen die de kit voor externe bestemmingen bevat, en vraagt bevestiging voordat het aanmeldingen vervangt die deze server al heeft.
2. **Instellingen.** **Instellingen terughalen** herstelt de eigen instellingen van BombVault uit de nieuwste zelf-back-up: bestemmingen, inloggegevens, schema's en regels. BombVault herstart om ze toe te passen, via de Docker-socket, zodat de live instellingendatabase nooit onder een open handle wordt overschreven, en de assistent gaat daarna vanzelf verder. De login van deze server blijft: je blijft aanmelden met het wachtwoord dat je nu gebruikt. Geheimen die de oude server had opgeslagen en die niet kunnen worden overgenomen, worden vermeld, zodat je ze opnieuw kunt invoeren.
3. **Selectie.** BombVault doorzoekt je back-ups en toont de containers, VM's, mappensets en ZFS-datasets die het vindt. De nieuwste snapshot is overal voorgeselecteerd. Wat deze server nog heeft, staat uit, omdat een herstel het zou vervangen. Mappensets weten na een rebuild niet meer waar ze vandaan kwamen, dus ze gaan in een map die je kiest.
4. **Herstel.** Alles wat is geselecteerd komt in één keer terug, gestopt gelaten als je elke container en VM zelf wilt starten, wat na een totaalverlies het veiligst is. Download daarna een nieuwe herstelkit: de oude hoort bij de vorige installatie.

!!! tip "Geplande migratie versus noodgeval"
    De Noodassistent herstelt de eigen instellingen van BombVault uit een back-up. Voor een *geplande* verhuizing naar een nieuwe machine kun je in plaats daarvan je configuratie rechtstreeks meenemen met de kaart **Instellingen exporteren / importeren** (een portable JSON-bestand). Zie [Configuratie](configuration.md#portable-settings-export-and-import).

### Uit een oude back-upmap {#restore-from-another-bombvault-repo}

**Uit een oude back-upmap** onder **Andere bronnen** op de pagina Herstel opent de repository van een *andere* BombVault-instantie (een share die onder `/mnt` is gemount, of een extern adres) met de herstelkit van die instantie of haar ingetypte `APP_KEY`, in een eenmalige, alleen-lezen sessie. Blader door de containers, VM's, mappensets en ZFS-datasets die daar staan, kies een snapshot en herstel hem, en het herstelde object wordt een gewone lokale container, VM of mappenset. Er wordt nooit iets naar de andere repository geschreven, en je eigen back-upinstellingen blijven onaangeroerd (de sessie leeft in het geheugen en verloopt vanzelf). Een container van server A naar server B verplaatsen betekent niet dat je je instellingen moet omleggen en daarna terugdraaien. Dit venster is eenmalig: het opent een sessie, herstelt wat je kiest en vergeet de andere instantie. Wil je in plaats daarvan een vaste regeling, waarbij deze machine de snapshots van een andere instantie volgens een schema in haar eigen repository ophaalt, dan is dat de rol **Ophaler** op de pagina Instanties.

Een container waarvan het netwerk op deze server niet bestaat, zoals een Unraid-netwerk `br0` op een gewone Docker-host, toont onder zijn rij een netwerkkiezer. BombVault maakt hem aan op het netwerk dat je kiest, samen met zijn andere netwerken. Het vaste IP en MAC-adres hoorden bij het oude netwerk en vervallen, dus het nieuwe netwerk wijst ze toe.

## Sleutelbewaarder {#key-keeper}

Een sleutelbewaarder is een instantie van je groep die een verzegelde kopie van de herstelkit van deze server bewaart. Hij kan de kopie zelf niet openen. Gaat deze server ooit verloren, dan vraagt een nieuwe installatie die bij je groep aansluit de bewaarder om de kopie en opent haar met je vrijgavecode.

1. Schakel op de pagina Instanties **Sleutelbewaarder** in op de kaart van de instantie die hem moet bewaren, en kies **Bewaarkopie maken**. Een bewaarkopie maken vraagt een inlogwachtwoord.
2. Deze server toont de **Vrijgavecode** één keer: twintig letters en cijfers in vier groepen van vijf. Schrijf haar op en bevestig met **Ik heb de code opgeschreven**. Deze server slaat de code niet op, alleen wat hij nodig heeft om er een bewaarkopie voor te verzegelen.
3. De bewaarder houdt nu de bewaarkopie, en deze server vernieuwt haar wanneer de kit verandert. **Bewaarkopie van de sleutel** op de kaart van deze instantie toont **Bewaarkopie van deze server** met elke bewaarder en zijn status, en **Bewaarkopieën die deze server bewaart** voor de anderen. **Nieuwe code maken** vervangt de code; elke bewaarder schakelt over op de nieuwe zodra hij de nieuwe bewaarkopie krijgt.

Op die dag heb je drie dingen nodig: de twaalf woorden van je groep, de vrijgavecode en toegang tot de interface van de bewaarder, waar iemand op **Toestaan** moet drukken. Kies in de Noodassistent **Via de groep**, kies de bewaarkopie, vraag haar aan en typ de vrijgavecode. De code wordt op de nieuwe server gebruikt en niet naar de bewaarder gestuurd. Bij de bewaarder verschijnt het verzoek onder **Bewaarkopie van de sleutel** met wie vraagt, of die instantie daar ooit eerder is gezien en hoe vaak ze vroeg; **Toestaan** geeft de verzegelde kopie uit, en toestaan vraagt een inlogwachtwoord. De nieuwe installatie heeft 30 minuten na het Toestaan om haar op te halen, een verzoek dat niemand beantwoordt verloopt na een dag, en een instantie die te vaak vraagt, moet een uur wachten.

Zonder de code kan niemand de kopie openen, de bewaarder ook niet. Is de code kwijt, maak dan een nieuwe zolang deze server nog draait; is de server al weg, dan werken de herstelkit en de ingetypte APP_KEY nog. De bewaarder kan een bewaarkopie op elk moment wissen, en de eigenaar moet dan opnieuw vragen.

## Herstelkit {#recovery-kit}

Dit is het onderdeel dat noodherstel mogelijk maakt, zelfs wanneer er geen draaiende BombVault is.

**Herstelkit downloaden** in de kaart **Voor noodgevallen** onder **Instellingen, Herstel** geeft je de **sleutel**, het **afgeleide restic-wachtwoord**, elke **bestemming** met de repository van elk onderdeel, de aanmeldingen die een nieuwe installatie nodig heeft om ze te bereiken, en de **exacte restic-commando's**, zodat je rechtstreeks met de restic-CLI op elke machine kunt herstellen. Hij vraagt een inlogwachtwoord. De kit drukt de sleutel ook af wanneer versleuteling uit staat, omdat de container- en VM-definities er altijd mee zijn verzegeld. Na een overname drukt hij de overgenomen back-upsleutel af en de APP_KEY van deze server, die een nieuwe installatie alleen nodig heeft om de instellingen terug te halen. Een kaart **Bewaar je herstelkit** in het overzicht herinnert je eraan totdat je hem hebt bewaard.

!!! danger "Bewaar de herstelkit buiten de server"
    De kit bevat het geheim dat je back-ups ontsleutelt. Bewaar hem ergens veilig en gescheiden van de server (een wachtwoordmanager, een geprinte kopie in een kluis). Als je zowel BombVault als `APP_KEY` verliest zonder herstelkit en zonder bewaarkopie van de sleutel, kunnen je versleutelde back-ups niet worden hersteld.

!!! warning "De nieuwste snapshot is niet altijd de juiste om te herstellen"
    Sinds restic 0.17 toont `restic snapshots` de grootte van elke snapshot. Na dataverlies kan de nieuwste snapshot de leeggemaakte zijn, dus herstel geen snapshot die veel kleiner is dan de vorige. Na ransomware kan het de versleutelde zijn, met de gebruikelijke grootte. Draait BombVault nog, kijk dan eerst op de pagina **Anomalieën**: die noemt de laatste goede back-up, en het herstelvenster selecteert die vooraf. Een herstel heeft geen enkele anomaliedata van BombVault nodig, en de bewaarpauze houdt altijd alleen méér snapshots.

### De kit verzegelen

Als je **Exports versleutelen met age** hebt aangezet onder **Instellingen, Systeem**, wordt de kit daar ook mee verzegeld en gedownload als `bombvault-recovery-kit.md.age`. Hij is ASCII-armored in plaats van binair, dus het blijft platte tekst: in een wachtwoordmanager plakken of afdrukken werkt precies als voorheen, alleen is de inhoud zonder je sleutel onleesbaar.

!!! warning "Bewaar de age-sleutel niet in de kit"
    Je hebt je **privé**-age-sleutel nodig om een verzegelde kit te openen. Bewaar die ergens waar hij niet van de kit zelf afhangt, anders heb je twee dingen te herstellen in plaats van één. Verzegelen loont als de kit ergens ligt waar je niet volledig zelf over gaat (een gedeelde wachtwoordmanager, notities in de cloud, een afdruk op kantoor); een kit in je eigen kluis wordt al door de kluis beschermd.

    Met versleuteling aan en geen bruikbare ontvanger ingesteld wordt de download volledig geweigerd. BombVault geeft de hoofdsleutel nooit alsnog onversleuteld af.

### Als het pakket niet bij de hand is

Het wachtwoord staat nergens opgeslagen, het wordt **berekend** uit de sleutel van de back-ups. Met de sleutel en een shell kun je het dus zelf namaken:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Dat is HMAC-SHA256 over de vaste tekst `bombvault:restic-repo`, met de ruwe bytes van de hexadecimale `APP_KEY` als sleutel, weergegeven als 64 hexadecimale tekens in kleine letters. Dezelfde waarde staat in het pakket, als afgeleid restic-wachtwoord; dit is voor de dag dat het pakket ergens anders ligt dan jij. Gebruik op een server die de back-ups van een andere heeft overgenomen de overgenomen back-upsleutel, de oude APP_KEY van die server.

!!! warning "Gebruik bij een ontvangen repository de sleutel van de VERZENDENDE instantie"
    Een repository dat hier via een off-site kopie is beland, is aangemaakt door de machine die het stuurde, met **diens** sleutel. Afleiden uit de sleutel van de ontvangende machine geeft een wachtwoord dat restic weigert, wat precies leest als een kapot repository terwijl het dat niet is. Dat is de gebruikelijke reden dat `restic check` op een ontvangen repository steeds opnieuw om het wachtwoord vraagt.

Omdat hersteldefinities **binnen** elke repo leven (`<repo>/def`, `<repo>/vm-def`), is een gekopieerde repo-map volledig zelfstandig, dus de kit plus de repo is alles wat een bare-metal-herstel nodig heeft.

## Een databasedump terughalen {#database-dumps}

Een databasedump is een eigen herstelpunt in de containerrepository, met het label `dbdump:<container>` en één bestand, `/dbdump/<container>.sql`. BombVault toont, downloadt en importeert ze op het tabblad **Back-ups** van de container; hieronder staan dezelfde stappen met alleen restic, voor de dag dat BombVault er niet is.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

De labels `dbversion:` en `dbname:` op elke dump zeggen uit welke serverversie hij komt en welke databases hij bevat. Een volledig bestand eindigt met `-- PostgreSQL database cluster dump complete` of `-- Dump completed`.

Importeer hem in een container van dezelfde of een nieuwere versie (PostgreSQL), of dezelfde hoofdversie (MySQL en MariaDB), die één keer met een lege datamap is gestart zodat hij zich initialiseert. De host heeft geen databaseclient nodig, de container heeft er een:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Voor één database uit een volledige dump nemen MySQL en MariaDB `--one-database <name>` op het clientcommando. Een PostgreSQL-dump heeft per database een sectie die begint met een regel `\connect <name>`: kopieer die sectie naar een eigen bestand en importeer dat met `-d <name>` nadat je de database hebt aangemaakt.

!!! warning "Een dump als root neemt de gebruikers van de server mee"
    Een volledige MySQL- of MariaDB-dump die als root is genomen, bevat de systeemdatabase `mysql`; importeren vervangt daarmee de accounts van de nieuwe server, inclusief het root-wachtwoord, door die uit de dump. Op PostgreSQL is `role ... already exists` voor de gebruiker die de container aanmaakte te verwachten en onschuldig.
