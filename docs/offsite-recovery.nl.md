# Off-site en herstel

!!! note "Off-site-kopieën wachten na een herinstallatie"
    Bouwen de Noodassistent of **Back-ups ontdekken** items opnieuw op zonder de oude instellingen, dan wachten de off-site-kopieën van die onderdelen tot je hun back-ups overneemt als back-ups van deze server. Zie [Na een herinstallatie](#after-a-rebuild).

Lokale back-ups beschermen je tegen een verloren container of een slechte update. Off-site-kopieën en een geteste herstelkit beschermen je tegen de hele machine, ransomware of brand. Deze pagina behandelt bestemmingen en off-site-kopieën, een kopie onaantastbaar maken, bewijzen dat je kunt herstellen, meerdere servers koppelen en alles terughalen als BombVault zelf weg is.

## Off-site-kopieën {#off-site-copies}

Houd de snelle lokale back-up en voeg een of meer bestemmingen buiten de locatie toe. BombVault kopieert er nieuwe snapshots naartoe met `restic copy` op best-effort-basis, zodat een off-site-probleem de lokale back-up nooit laat mislukken. In deze opzet schrijft elk onderdeel eerst naar zijn eigen map en is het off-site-repository een kopie. De eigen map hoeft niet op deze server te staan: zie [De eigen map van een onderdeel](#own-folder) om hem in een bestemming zoals S3 of een rest-server te houden.

- **Meerdere bestemmingen per onderdeel.** Elk onderdeel (containers, VM's, flash, zelf-back-up, mappen en ZFS-datasets) kan naar meerdere off-site-bestemmingen tegelijk kopiëren, zodat je bijvoorbeeld een rest-server bij een vriend en een S3-bucket naast elkaar kunt houden. De kaart **Bestemmingen per onderdeel** op de pagina Bestemmingen zet elke lijn van een onderdeel naar een bestemming aan of uit.
- **Wanneer kopieën draaien.** De kaart **Kopiëren** van een bestemming bevat **Wanneer kopiëren**: **Na elke back-up**, of volgens een eigen planning die de snapshots verzamelt en op vaste tijden kopieert. Onder **Instellingen, Planning, Off-site-kopieën per onderdeel** geeft **Uitzondering** één onderdeel een eigen kopieerplanning.
- **De kaart Offsite-kopieën.** Onder **Instellingen, Bestemmingen** heeft elk onderdeel met een off-site-kopie een rij in **Offsite-kopieën**, met **Verbinding testen**, **Nu repliceren** om een kopie op verzoek te starten, en **Instellen** of **Bewerken**. De rest van de kopieerinstellingen van het onderdeel opent van daaruit in zijn venster.
- **Bewaarbeleid** stel je per bestemming in onder **Hoe lang bewaren**, zodat je off-site-kopieën langer kunt bewaren als archief. De kopie in het off-site-veld van een onderdeel volgt het als elke andere kopie, en een onderdeel kan eigen waarden houden onder **Regels per onderdeel**. Alles op nul snoeit off-site-snapshots nooit. Zie [Hoe lang bewaren](#keep).
- **Bandbreedtelimieten** (**Off-site bandbreedte** onder **Instellingen, Bestemmingen**, of de kaart **Kopiëren** van een bestemming) begrenzen de upload- en downloadsnelheid van restic, zodat kopieën je WAN niet verzadigen. Ze worden in KiB/s ingevoerd, de eenheid die restic neemt, en elk veld toont terwijl je typt dezelfde snelheid in MB/s en Mbit/s: 2048 leest als ongeveer 2,0 MB/s (16,8 Mbit/s).
- **Compressie** is **Uit**, **Automatisch** (de standaard van restic) of **Maximaal**, ingesteld op elke bestemming en, voor één onderdeel, onder zijn regels daar. Back-ups, kopieën en opschonen schrijven ermee, en de herstelkit noemt haar, zodat gewoon restic op dezelfde manier kan blijven schrijven.
- **Streaming eerst.** Terwijl een mediaserver zoals Plex, Jellyfin of Emby streamt, uploaden kopieën met een lagere limiet en keren ze een paar minuten nadat de stream eindigt terug naar de normale. De kaart **Streaming** op de pagina Bestemmingen heeft **Off-site kopieën vertragen tijdens streaming**, **Mediaservers** (containers met Plex, Jellyfin of Emby in de imagenaam tot je kiest), **Stream boven (Mbit/s)**, **Uploadlimiet tijdens streaming (KiB/s)** en **Terug naar normaal na (min)**. BombVault leest het verkeer van Docker, dus een mediaserver op het hostnetwerk kan niet worden gemeten. REST, S3, B2, Azure, Google Cloud, Swift en rclone over HTTP vertragen midden in een kopie. SFTP en lokale of gemounte mappen nemen de lagere limiet bij hun volgende kopieerstap.
- **Koude opslag (S3).** Een S3-bestemming kan een goedkopere **Opslagklasse** gebruiken die restic nog meteen kan lezen, zoals Standard-IA, One Zone-IA, Intelligent-Tiering of Glacier Instant Retrieval. Glacier Flexible Retrieval en Deep Archive blijven buiten beschouwing, omdat restic er niet uit kan herstellen. Een rclone-remote stelt zijn klasse in de rclone-configuratie in.
- **Gelijktijdige verbindingen.** Op een S3-, B2-, Azure-, Google Cloud-, Swift-, rest-server-, SFTP- of rclone-bestemming bepaalt **Gelijktijdige verbindingen** op de kaart **Verbinding** hoeveel verbindingen restic tegelijk opent, van 1 tot 32. Blijft het veld leeg, dan houdt restic zijn standaard van 5. Meer kan een verre of trage lijn sneller maken, minder ontziet een aanbieder die afknijpt. Back-ups, kopieën, controles, opschonen en herstel op de bestemming gebruiken het allemaal, en een export van de instellingen neemt het mee. Een lokale of gekoppelde map heeft deze instelling niet.
- **Groeibudget.** **Groeibudget (GB, 0 = uit)** in het kopieervenster van een onderdeel slaat alarm als het repository boven die grootte groeit. Het verwijdert nooit iets, dus het past bij een append-only-kopie die van hieruit niet wordt opgeschoond.
- Terwijl een kopie loopt, zegt de statuskop van het overzicht **Off-site-kopie loopt**. Het is een actieve indicator, geen voortgangsbalk met percentage, omdat `restic copy` geen machineleesbare voortgang geeft.

!!! note "Herstellen vanaf elke plek"
    Elk item toont zijn back-ups als één tijdlijn over alle plekken waar een back-up staat, op zijn tabblad **Back-ups**. Een back-up die naar B2 is gekopieerd, verschijnt één keer, gemarkeerd met elke plek die hem bevat. Een herstel neemt de eerste plek die het kan bereiken, beginnend met het repository waarnaar het item wordt geschreven, en je kunt per snapshot een andere plek kiezen. Off-site-bestemmingen worden pas gelezen als je op **Controleren** klikt, omdat het uitlezen van cloudopslag even duurt. Verwijderen op één plek controleert eerst de andere en zegt of het de laatste kopie was. Zie [Herstel](restore.md).

## Bestemmingen {#destinations}

Een bestemming is een plek waar back-ups naartoe gaan: een lokale map, een share, een eigen server of een clouddienst. Ze wordt één keer ingesteld en bedient dan elk onderdeel. Open **Bestemmingen** via zijn instellingentegel, of schakel daar **Ook in de zijbalk tonen** in om hem vast te zetten.

![De pagina Bestemmingen: de lijst met bestemmingen en de stroom van welk onderdeel waarheen gaat.](assets/screenshots/storage/locations-dark.png#only-dark)
![De pagina Bestemmingen: de lijst met bestemmingen en de stroom van welk onderdeel waarheen gaat.](assets/screenshots/storage/locations-light.png#only-light)

**Bestemming toevoegen** opent een venster in vier stappen:

1. **Aanbieder.** Elke dienst staat met zijn logo in een lijst, met een zoekveld rechtsboven in het venster, in groepen: **Lokaal** voor een schijf of share die aan deze server hangt, **NAS of server** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV en andere), **S3-dienst** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 en andere), je eigen S3-server (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), **Cloudopslag** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud en de rest die rclone ondersteunt) en **Uit je groep**, de ontvangende servers van je groep. Elke zegt hoe goed hij bij back-ups past: cloudschijven vertragen bij veel verzoeken, dus de eerste back-up en het opschonen duren er langer. Voor een dienst die geen tegel dekt, neemt de tegel **Rclone** onder **Eigen rclone-configuratie** remotes die uit een rclone.conf zijn geplakt.
2. **Aanmelden.** De velden hangen af van de dienst: een toegangssleutel voor S3, een gebruiker en wachtwoord voor WebDAV en SMB, een app-wachtwoord waar tweestapsaanmelding het gewone blokkeert, de publieke SSH-sleutel van BombVault voor SFTP en de Storage Box, of een token voor diensten die via een browser aanmelden. Daarvoor toont het venster een opdracht `rclone authorize` om op een computer met een browser uit te voeren, en het token dat die afdrukt, komt in het veld. Heb je al een [opgeslagen login](#sign-ins) voor de dienst, dan kiest **Opgeslagen inloggegevens** die in plaats van **Nieuwe sleutels**. **Verbinding testen** controleert de aanmelding voordat er iets wordt opgeslagen. Het venster toont dan de mappen op de bestemming, met **Nieuwe map** om er een te maken en de vrije ruimte waar de dienst die meldt. Een lege map is het veiligst. BombVault zet er één map per onderdeel in.
3. **Bescherming.** Het venster zegt duidelijk wat de dienst kan. Een rest-server in append-only-modus weigert verwijderen, en de sabotagetest controleert dat. Een S3-bucket kan oude versies bewaren met versiebeheer en objectvergrendeling, wat BombVault nog niet kan controleren. Een cloudschijf kan verwijderen helemaal niet weigeren: wie bij de server binnenkomt, komt ook bij die kopie. Zet **Bescherming tegen verwijderen (append-only)** alleen aan waar de andere kant verwijderen echt weigert. BombVault schoont daar dan nooit op. **Voor noodgevallen** zegt wat de herstelkit voor deze bestemming bevat.
4. **Voor welke onderdelen.** Kies de onderdelen die hierheen back-uppen. De eerste kopie neemt de hele geschiedenis mee. Items die hun eigen bestemmingen kozen, krijgen de nieuwe niet vanzelf, en het venster noemt ze.

![Bestemming toevoegen: de eerste stap, met elke aanbieder als tegel.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Bestemming toevoegen: de eerste stap, met elke aanbieder als tegel.](assets/screenshots/storage/add-providers-light.png#only-light)

![Bestemming toevoegen: de stap Aanmelden na een geslaagde test.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Bestemming toevoegen: de stap Aanmelden na een geslaagde test.](assets/screenshots/storage/add-signin-light.png#only-light)

Sommige diensten vragen bij het aanmelden iets meer:

- **SFTP en de Storage Box.** **Sleutel toevoegen** zet de publieke sleutel van BombVault op de server met het **Wachtwoord van het account**, dat voor die ene aanmelding wordt gebruikt en nergens wordt bewaard. De opdracht om het met de hand te doen staat eronder. De test toont de **Serversleutel** met zijn vingerafdruk. BombVault onthoudt hem en weigert de server als die later een andere toont.
- **S3-sleutels voor één bucket**, zoals een objecttoken van Cloudflare R2 of een beperkte AWS-sleutel, mogen de andere buckets misschien niet tonen. Typ de bucket in **Bucket**, en de test en de maplijst openen die bucket.
- **S3-regio's en adressen.** De regio wordt gelezen uit een eindpunt dat haar noemt (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi), Garage begint met zijn eigen standaardregio, en IDrive e2 vraagt om het eindpunt dat zijn dashboard toont. Een adres dat met de bucket erop is geplakt, zoals het dashboard van R2 het toont, vult de server en de bucket in. Een bucket die met de mapbrowser is gemaakt, komt in de regio die het formulier noemt.
- **Opslagklasse.** De keuzelijst biedt alleen de klassen die de dienst neemt, en **Meer instellingen** toont alleen de opties die een nieuwe bucket vormgeven.
- **Google Drive en OneDrive.** Opgeschoonde data wordt definitief verwijderd in plaats van naar de prullenbak te gaan, waar ze nog steeds van de quota afgaat. Een gedeelde schijf van Google komt onder **Meer instellingen**.
- **SMB-servers die hun shares niet tonen**, nemen de naam van een share onder **Gedeelde map**, en de test opent die share rechtstreeks.
- **FTP** vraagt om zijn **Versleuteling**: **Geen**, **Expliciete TLS (FTPES)** of **Impliciete TLS (FTPS, poort 990)**.
- **OneDrive en Zoho WorkDrive.** Opent een aanmelding meerdere schijven of werkruimtes, dan vraagt het venster waarheen de back-ups gaan.
- **Een rest-server met een zelfondertekend certificaat.** Zet het certificaat als PEM-bestand in de map `certs` van de datamap van BombVault (`/config/certs` in de container, `appdata/bombvault/certs` op Unraid). Back-ups, kopieën en de test van de bescherming tegen verwijderen vertrouwen het zonder herstart.

S3-diensten lopen via de eigen S3-backend van restic, waardoor een opslagklasse en objectvergrendeling kunnen gelden. Een rest-server wordt rechtstreeks door restic bereikt. Elke andere dienst loopt via de rclone die BombVault meelevert, en zijn deel van de rclone-configuratie staat in de kaart **rclone.conf** op de pagina van de bestemming, met de geheimen verborgen. Een export van instellingen bevat de bestemmingen, en met inloggegevens erbij ook hun aanmeldingen.

Sla je een bestemming op die al back-ups van containers, VM's of mappen bevat, bijvoorbeeld een die je verwijderde en opnieuw toevoegt, dan wordt ze aan die onderdelen toegevoegd als een kopie zonder vinkje. Er wordt niets heen gekopieerd tot je haar aanvinkt, maar **Herstel** biedt de oude back-ups meteen aan.

De map op deze server heet overal **Lokaal**: op de opslagpagina's, in de stroom, op een item en bij Herstel. Met meerdere mappen op deze server gaat elk onder zijn eigen naam. Een bestemming die het off-site-veld van een onderdeel instelt, heet naar haar plek, de laatste map van haar adres of, zonder, haar server, en krijgt een nummer als een andere bestemming al zo heet. Elke bestemming kan worden hernoemd in het veld **Naam** van haar kaart **Verbinding**, ook een bucketmap die meerdere onderdelen delen en de kopie die het off-site-veld van een onderdeel beschrijft, en een naam die je geeft, blijft staan. Waar twee bestemmingen met dezelfde naam naast elkaar staan, draagt elk zijn server of map.

### De pagina van een bestemming {#location-page}

Elke bestemming heeft een eigen pagina. Ze toont alleen de kaarten die voor die bestemming gelden, in deze volgorde:

![De pagina van een bestemming: Eigen mappen, Verbinding en Inloggegevens.](assets/screenshots/storage/location-page-dark.png#only-dark)
![De pagina van een bestemming: Eigen mappen, Verbinding en Inloggegevens.](assets/screenshots/storage/location-page-light.png#only-light)

- **Eigen mappen**: de onderdelen die hun back-ups hier eerst naartoe schrijven. De aanmelding, bescherming en limieten van deze bestemming gelden voor hen. **Als eigen map gebruiken** verplaatst de eigen map van meer onderdelen hierheen, en **Wijzigen** verplaatst er een weer weg. Zie [De eigen map van een onderdeel](#own-folder).
- **Back-upt rechtstreeks hierheen**: elk onderdeel waarvan de items rechtstreeks naar deze plek back-uppen in plaats van naar hun eigen map, en hoeveel items dat doen. Zo'n direct repository neemt al zijn instellingen over van deze bestemming. Eén waar niets meer naartoe back-upt, zegt dat, en **Verwijderen** haalt het van de lijst. De back-ups op zijn plek blijven. Zie [Directe repository's](#direct-repositories).
- **Verbinding**: de **Naam**, de schakelaar **Beschikbaar** (uit betekent dat BombVault hier niet meer schrijft, en items die hierheen back-uppen mislukken met een duidelijke melding in plaats van elders terecht te komen), de map, **Compressie**, **Opslagklasse**, **Gelijktijdige verbindingen** en **Buiten de locatie**. Op de bestemming **Lokaal** toont ze de map van elk onderdeel onder **Eigen mappen hier**. **Verbinding wijzigen** test en bewaart een nieuw adres of nieuwe inloggegevens. Melden andere plekken zich met dezelfde opgeslagen inloggegevens aan, dan zegt het venster dat, en een wijziging hier geeft deze bestemming eigen inloggegevens in plaats van de gedeelde te herschrijven.
- **Inloggegevens**: welke opgeslagen inloggegevens ze gebruikt (**Logt in met**), welke andere bestemmingen ze ook gebruiken (**Ook gebruikt door**) en **Bewerken**, dat het venster met de inloggegevens opent. Zie [Inloggegevens](#sign-ins).
- **rclone.conf**: het deel van de rclone-configuratie van deze bestemming, voor een bestemming die via rclone loopt.
- **Bescherming tegen verwijderen**, met **Bescherming tegen verwijderen (append-only)**, **Accepteren zonder bescherming tegen verwijderen**, **Bescherming tegen verwijderen nu testen** en **Wie pruned**, wat zegt of BombVault de kopie en de eigen mappen hier opschoont, volgens de kaart **Hoe lang bewaren** van deze bestemming of de eigen waarden van een onderdeel onder **Regels per onderdeel**, of dat aan de andere kant overlaat. Een lokale map toont deze kaart alleen als ze een bescherming heeft die je kunt uitschakelen. Zie [Bescherming tegen verwijderen](#delete-protection-append-only).
- **Kopiëren**: **Wanneer kopiëren**, en de **Uploadlimiet (KiB/s)** en **Downloadlimiet (KiB/s)**. Een bestemming die alleen eigen mappen en geen kopieën bevat, toont de limieten als een kaart genaamd **Bandbreedte**.
- **Hoe lang bewaren**: **Kort**, **Gebalanceerd**, **Lang** of **Eigen waarden**, met een tijdlijn van welke back-ups de regel bewaart. Zie [Hoe lang bewaren](#keep).
- **Staat al onder deze bestemming**: onderdelen waarvan het met de hand getypte adres in een map van deze bestemming ligt. Er een overnemen laat het repository waar het is. Aanmelding, opslagklasse en bescherming tegen verwijderen komen dan van de bestemming. BombVault controleert eerst dat de inloggegevens van de bestemming het repository openen, en weigert een append-only-repository onder een bestemming te zetten die niet append-only is.
- **Regels per onderdeel**: het eigen repository, bewaarbeleid of de compressie van één onderdeel op deze bestemming. **Regels wijzigen** opent ze, en **Terug naar de bestemming** brengt een onderdeel terug. Aanmelding, bandbreedte en wanneer kopieën draaien komen altijd van de bestemming.
- Een vat dat toont hoe vol de bestemming is, met de markering waar ze vol is en hoeveel weken er nog zijn bij de groei van de laatste weken. Een bestemming die haar grootte niet meldt, zegt **Grootte onbekend**.

**Verplaatsen** en **Bestemming verwijderen** staan tussen de acties van de pagina, beschreven onder [Verplaatsen en verwijderen](#move-remove).

Een externe bestemming die back-ups bevatte en ineens geen repository meer heeft, zoals een rest-server waarvan de schijf niet is gemount, krijgt niet uit zichzelf een nieuw, leeg repository. Haar pagina zegt dat zodra een back-up of kopie merkt dat het repository weg is. Mount de schijf opnieuw. Heb je het oude repository met opzet leeggemaakt, dan begint **Hier een nieuw repository aanmaken** opnieuw, voor elk onderdeel van de bestemming en zijn repository's tegelijk. Deze beveiliging is er voor een bestemming die op haar plaats is gebleven. Een bestemming die je toevoegt, of een onderdeel dat je bij een bestemming aanvinkt, begint schoon: staat er op dat adres geen repository, dan maakt de eerste kopie er een aan, ook waar een eerder verwijderde bestemming er een had.

Twee weigeringen noemen wat je moet rechtzetten, in de run van de kopie en op de pagina van de bestemming. **Deze bucket bestaat niet op de server** komt zodra een S3-server dat zegt, en BombVault maakt daar niets aan, zodat een verkeerd getypte bucketnaam zichtbaar blijft. Een server die helemaal niet antwoordt, is een ander geval: restic blijft het per aanroep ongeveer 15 minuten opnieuw proberen, voor het geval hij terugkomt. **Hier staat al een restic-repository, en de sleutel van deze instantie opent het niet** betekent dat de back-ups daar bij een andere `APP_KEY` horen.

Een ontvangende server die een andere instantie van je groep draait, verschijnt onder de aanbieders als **Uit je groep**. Zie [Ontvangserver](#receiving-server).

BombVault controleert zijn bestemmingen bij elke start. Het zet recht wat geen beslissing van jou nodig heeft, zoals een koppeling aan een kopie of repository die weg is, een kopie op de eigen map van een onderdeel, of een direct repository waarvan de kopie weg is, en schrijft voor elke reparatie een regel in het logboek. Wat geen veilige oplossing heeft, wordt alleen gelogd.

![Dezelfde pagina op een telefoon.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![Dezelfde pagina op een telefoon.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Hoe lang bewaren {#keep}

Hoe lang back-ups worden bewaard, stel je per bestemming in, in haar kaart **Hoe lang bewaren**. De regel geldt voor de kopieën op die bestemming en de eigen mappen die daar liggen, de bestemming **Lokaal** inbegrepen. Een onderdeel kan op één bestemming eigen waarden houden onder **Regels per onderdeel**, en die gaan voor de kaart van de bestemming voor dat onderdeel.

Een regel bewaart één back-up per dag, week, maand en jaar, zo ver terug als je instelt, en ruimt de rest op na elke back-up of kopie:

| Voorinstelling | Laatste back-ups | Dagen | Weken | Maanden | Jaren |
|---|---|---|---|---|---|
| **Kort** | 0 | 7 | 4 | 3 | 0 |
| **Gebalanceerd** | 0 | 7 | 4 | 6 | 2 |
| **Lang** | 0 | 14 | 8 | 12 | 3 |

**Eigen waarden** stelt de vijf aantallen zelf in. Een 0 schakelt dat aantal uit, en een regel waarvan elk aantal 0 is, bewaart alles. De stippen in de kaart tonen welke van 40 voorbeeldback-ups een regel bewaart, de nieuwste uiterst rechts. Het voorbeeld is voor elke bestemming hetzelfde, omdat het plaatje wordt getekend voordat BombVault de back-ups daar heeft gelezen. Het telt terug vanaf vandaag, dus hoeveel stippen er blijven staan, hangt af van de datum.

Een map op deze host en een met de hand getypte externe map hebben ook een eigen kaart, die de eigen mappen in die map dekt en geen andere. Tot je die opslaat, schoont BombVault zo'n map op volgens de regel die het er al voor had.

**Vragen voor minder bewaren**, standaard aan, vraagt om bevestiging voordat een wijziging die minder back-ups bewaart wordt opgeslagen, en zegt hoeveel van de 40 voorbeeldback-ups de nieuwe regel verwijdert. Het geldt voor elke bestemming. **Terug naar de bestemming** onder **Regels per onderdeel** stelt dezelfde vraag.

Een bestemming met bescherming tegen verwijderen wordt vanaf deze machine nooit opgeschoond. Haar andere kant bewaart of laat oude back-ups vallen volgens eigen regels. Zolang een item een openstaande kritieke anomalie over verloren data heeft, bewaart het bewaarbeleid de oude back-ups van dat item tot je haar bevestigt. Zie [Anomalieën](anomalies.md).

Een back-up die minder bevat dan zou moeten, laat de back-ups ervoor nooit verlopen. Dat is een back-up die geen bestanden leest waar eerdere er wel hadden, een gekozen map mist of sommige bestanden niet kon lezen. Zijn run eindigt met een waarschuwing die noemt wat er ontbreekt en zegt dat de oudere back-ups bewaard zijn. Je krijgt hetzelfde bericht via je meldingskanalen, en geen opschoning, in de eigen map van het item of bij zijn kopieën, raakt de oude back-ups van dat item aan tot er een volledige back-up doorkomt. Een eerste back-up, en een gekozen map die altijd leeg was, tellen als volledig.

Om te zien wat de volgende run echt zou verwijderen, open je **Voorbeeld** onder **Instellingen, Bestemmingen**. Het toont **Wat de volgende uitvoering zou verwijderen** voor de eigen map van het onderdeel en voor elk van zijn kopieën, per repository en per item met de herstelpunten bij naam. Het neemt geen lock en verandert niets.

![Hoe lang bewaren met Gebalanceerd gekozen: de stippen tonen welke van de 40 voorbeeldback-ups blijven.](assets/screenshots/storage/keep-dark.png#only-dark)
![Hoe lang bewaren met Gebalanceerd gekozen: de stippen tonen welke van de 40 voorbeeldback-ups blijven.](assets/screenshots/storage/keep-light.png#only-light)

#### Voorbeeld: wat Gebalanceerd bewaart {#example-retention}

Een container wordt sinds 1 oktober 2024 elke nacht om 03:00 geback-upt, dus op woensdag 4 november 2026 heeft hij 765 back-ups gehad. Zijn bestemming staat op **Gebalanceerd**: 7 dagen, 4 weken, 6 maanden en 2 jaar.

Elk aantal bewaart de nieuwste back-up van zijn dag, week, maand of jaar, en een back-up die meerdere aantallen kiezen, wordt één keer bewaard. Weken lopen van maandag tot zondag:

| Regel | Back-ups die ze kiest | Nieuwe die ze toevoegt |
|---|---|---|
| 7 dagen | 4 nov, 3 nov, 2 nov, 1 nov, 31 okt, 30 okt, 29 okt | alle 7 |
| 4 weken | 4 nov, 1 nov, 25 okt, 18 okt | 25 okt, 18 okt |
| 6 maanden | 4 nov, 31 okt, 30 sep, 31 aug, 31 jul, 30 jun | 30 sep, 31 aug, 31 jul, 30 jun |
| 2 jaar | 4 nov, 31 dec 2025 | 31 dec 2025 |

Dat laat 14 back-ups van de 765 over. De oudste is van 31 december 2025, dus vandaag kun je iets meer dan tien maanden terug. Dankzij deduplicatie nemen ze weinig meer ruimte in dan één volledige back-up plus wat er tussendoor veranderde.

**Jaren** verdient een tweede blik. Het telt kalenderjaren met een back-up erin, beginnend met het huidige, en de nieuwste back-up van dit jaar is die van vandaag. Het eerste jaar voegt dus niets toe aan de andere aantallen, en het tweede voegt de laatste back-up van het vorige jaar toe. Daarom wisselt het bereik van een regel door het jaar heen. Dezelfde back-ups onder elke voorinstelling:

| Voorinstelling | Vandaag bewaarde back-ups | Oudste back-up vandaag | Bereik door het jaar heen |
|---|---|---|---|
| **Kort** | 10 | 30 september 2026 | 1 tot 2 maanden |
| **Gebalanceerd** | 14 | 31 december 2025 | ongeveer 4 maanden in het vroege voorjaar, een heel jaar op 31 december |
| **Lang** | 30 | 31 december 2024 | 1 jaar op 1 januari, 2 jaar op 31 december |

Van **Gebalanceerd** naar **Kort** gaan, vraagt eerst om bevestiging, vanwege **Vragen voor minder bewaren**. De vraag zegt hoeveel van de 40 back-ups in het plaatje de nieuwe regel verwijdert. Dat getal komt uit het voorbeeld, dus het verandert met de datum en telt je eigen back-ups niet mee. Van de back-ups in dit voorbeeld zouden 31 augustus, 31 juli, 30 juni en 31 december 2025 verdwijnen. **Toch opslaan** past de regel toe, en de volgende opschoonronde verwijdert ze definitief van die bestemming.

Om de containers op deze bestemming langer te bewaren dan de rest, druk je op **Regels wijzigen** onder **Regels per onderdeel**, kies je **Containers** en stel je **Eigen waarden** in bij **Regels voor Containers**, bijvoorbeeld 14 dagen, 8 weken, 12 maanden en 2 jaar. De andere onderdelen blijven de kaart van de bestemming volgen, en **Terug naar de bestemming** brengt Containers terug.

### Inloggegevens {#sign-ins}

Inloggegevens zijn de sleutel of het wachtwoord die BombVault voor een S3-dienst of een rest-server gebruikt. Ze vormen één lijst, de kaart **Inloggegevens** onder **Instellingen, Bestemmingen**, en elke rij zegt welke bestemmingen ze gebruiken (**Gebruikt door**).

- **Opgeslagen bij het toevoegen van een bestemming.** Bestemming toevoegen slaat de sleutels die je typt op als inloggegevens. De volgende bestemming bij dezelfde dienst kan ze kiezen onder **Opgeslagen inloggegevens** in plaats van **Nieuwe sleutels**, en een latere wijziging geldt voor elke bestemming die ze gebruikt.
- **De standaard.** Een ster markeert de standaardinloggegevens. Ze worden gebruikt waar geen inloggegevens zijn gekozen, zoals bij een met de hand getypt adres of een herstel op een verse installatie. Een ophaalbron leent ze niet. **Als standaard** geeft de ster aan andere inloggegevens.
- **Bewerken en Testen.** **Bewerken**, hier of in de kaart **Inloggegevens** van een bestemming, opent één venster voor de inloggegevens. **Testen** meldt zich met wat je typte aan bij elke bestemming die ze gebruikt, voordat er iets wordt opgeslagen. **Opslaan** bewaart ze.
- **Verwijderen.** Inloggegevens kunnen alleen worden verwijderd als geen bestemming, ophaalbron of ontvangen repository ze gebruikt en geen oude back-ups ze nodig hebben, en de standaard gaat alleen als laatste. Oude back-ups hebben ze nodig als een eigen map is verhuisd van een plek die ermee inlogt en ze daar heeft achtergelaten. BombVault zegt dan welke plek dat is.
- **De verbinding van een bestemming wijzigen** herschrijft nooit inloggegevens die andere plekken delen. De bestemming krijgt in plaats daarvan eigen inloggegevens, en het venster zegt vooraf dat haar inloggegevens gedeeld zijn. Om de gedeelde overal te wijzigen, bewerk je ze in de kaart **Inloggegevens**.

![De kaart Inloggegevens: elke opgeslagen inlog met wat hem gebruikt, en de ster bij de standaard.](assets/screenshots/storage/signins-dark.png#only-dark)
![De kaart Inloggegevens: elke opgeslagen inlog met wat hem gebruikt, en de ster bij de standaard.](assets/screenshots/storage/signins-light.png#only-light)

Een map op deze server heeft geen inloggegevens nodig. SMB, WebDAV, SFTP, de Storage Box en de cloudschijven bewaren de hunne in de rclone-configuratie, die de kaart **rclone.conf** van de bestemming toont, dus ze staan niet in deze lijst.

### SMB, WebDAV en andere rclone-remotes {#smb-webdav}

Een Windows- of Samba-share en een WebDAV-server (Nextcloud, ownCloud, SharePoint of een andere) zijn aanbiedertegels in **Bestemming toevoegen**, onder **NAS of server**. Vul de host en share (SMB) of het adres en servertype (WebDAV), de gebruiker en het wachtwoord in, en BombVault schrijft de rclone-sectie voor je. rclone verhult het wachtwoord voordat het wordt opgeslagen.

Voor een dienst die geen tegel dekt, neemt de tegel **Rclone** onder **Eigen rclone-configuratie** remotes die uit een rclone.conf zijn geplakt. Ze worden toegevoegd aan de remotes die BombVault al heeft en vervangen ze nooit, en ze worden versleuteld opgeslagen. Om een onderdeel naar een van hen te back-uppen, vul je `rclone:<remote>:<folder>` in als het off-site-repository van dat onderdeel, bijvoorbeeld `rclone:nas:backups/bombvault`. De share of bucket is het eerste padsegment, geen deel van de naam.

Dit is de betere route dan de share op Unraid mounten: restic raadt af een repository op een gemounte CIFS-share te houden, en hier wordt niets gemount. NFS heeft geen tegel, omdat noch restic noch rclone een NFS-backend heeft. Mount voor NFS de export op de host en gebruik hem als lokale map.

### Verplaatsen en verwijderen {#move-remove}

- **Verplaatsen** brengt de kopie van één onderdeel naar een andere bestemming, met zijn hele geschiedenis. Alles blijft op de oude plek staan. BombVault verwijdert daar niets. De verplaatsing loopt op de achtergrond en kan lang duren: ongeveer 9 minuten per 100 GB tussen lokale schijven, uren over een trage verbinding. Het overzicht toont haar terwijl ze loopt.
- **Bestemming verwijderen** biedt twee wegen. **Bestemming verwijderen, back-ups bewaren** stopt met daar schrijven en laat de back-ups staan. Je kunt ze later lezen onder **Herstel, Uit een oude back-upmap**. **Bestemming verwijderen en de back-ups daar wissen** leest eerst wat er is opgeslagen en toont het: welke back-ups verdwijnen, welke daarvan nergens anders staan, en welke repository's blijven omdat bescherming tegen verwijderen weigert, items er nog rechtstreeks naartoe back-uppen, een andere bestemming of een ander onderdeel er zijn kopieën bewaart, of ze niet konden worden gelezen. Items die hun enige back-up zouden verliezen, staan apart en blijven bewaard tenzij je **Wis deze ook** inschakelt. Je bevestigt door de naam van de bestemming te typen, en het verwijderen loopt op de achtergrond.
- Dit werkt voor elk soort bestemming: een die meerdere onderdelen gebruiken, een doel dat voor één onderdeel is ingesteld, een repository dat je instelde en de map waarnaar een onderdeel zijn eigen back-ups schrijft. Items die naar een verwijderd repository back-uppen, volgen weer hun onderdeel, en een standaard van een onderdeel die daarheen wees, ook. De map van een onderdeel kan niet zomaar weg, want elk onderdeel heeft er een nodig. Het venster vraagt om de map waarheen de onderdelen verhuizen, en elk onderdeel houdt zijn mapnaam eronder.
- Het verwijderen van de back-ups op een bestemming werkt ook zonder inlogwachtwoord. Is er een ingesteld, dan vraagt het een aanmelding op de instantie zelf.

### Voorbeeld: een off-site-kopie op Backblaze B2 {#example-s3}

De containers en de zelf-back-up moeten ook bij Backblaze B2 staan, zodat een kapotte server ze niet meeneemt. Dezelfde stappen passen bij Wasabi, Hetzner Object Storage en de meeste andere S3-diensten.

1. **In B2** maak je een privébucket, zoals `tower-backups`, in een regio bij jou in de buurt. Maak onder **Application Keys** een sleutel met lees- en schrijftoegang, beperkt tot die bucket, en noteer zijn sleutel-ID en applicatiesleutel. B2 toont de applicatiesleutel één keer. Stel de lifecycle van de bucket in om alleen de laatste versie te bewaren, anders bewaart B2 opgeschoonde data als verborgen versies en rekent het die aan.
2. **Aanbieder.** Open in BombVault **Bestemmingen**, druk op **Bestemming toevoegen** en kies **Backblaze B2** onder **S3-dienst**.
3. **Aanmelden.** Vul het sleutel-ID in als **Toegangssleutel-ID** en de applicatiesleutel als **Geheime toegangssleutel**. Vul het eindpunt van de bucket in onder **Eindpunt**, zoals `s3.eu-central-003.backblazeb2.com`, en BombVault leest er de regio uit. Typ `tower-backups` in **Bucket**, omdat deze sleutel maar één bucket opent.
4. **Verbinding testen.** Het hoort **Verbinding gemaakt** te zeggen en de mappen van de bucket te tonen. Druk op **Nieuwe map** en noem hem `bombvault`. BombVault zet er één map per onderdeel in, zoals `bombvault/containers`.
5. **Bescherming.** Het venster legt uit dat B2 oude versies kan bewaren met objectvergrendeling en dat BombVault dit niet kan controleren. Laat **Bescherming tegen verwijderen (append-only)** uit tenzij de bucket verwijderen echt weigert, zodat BombVault kan opschonen volgens de bewaarregels van de bestemming.
6. **Voor welke onderdelen.** Vink **Containers** en **Zelf-back-up** aan. De zelf-back-up bevat je instellingen, zodat een nieuwe installatie ze kan terughalen. Het venster zegt hoeveel items en, waar bekend, hoeveel geschiedenis de eerste kopie meebrengt.
7. Druk op **Bestemming opslaan**.

De eerste kopie loopt na de volgende back-up. Om haar nu te starten, open je **Instellingen, Bestemmingen** en druk je op **Nu repliceren** in de rij van het onderdeel in **Offsite-kopieën**. De statuskop zegt **Off-site-kopie loopt**. De eerste kopie brengt de hele geschiedenis mee, dus 200 GB over een uplink van 100 Mbit/s duurt ongeveer vier en een half uur. Latere kopieën sturen alleen wat nieuw is.

Om je lijn vrij te houden, open je de pagina van de bestemming en stel je **Uploadlimiet (KiB/s)** in de kaart **Kopiëren** in, bijvoorbeeld op `5120`, wat leest als ongeveer 5,0 MB/s (41,9 Mbit/s). Controleer **Hoe lang bewaren** op dezelfde pagina meteen mee.

Om te zien dat het werkte, kijk je bij **Beschermingsstatus** op het overzicht, waar **Off-site** verandert in **kopie actueel**, en op het tabblad **Back-ups** van een container, waar zijn snapshots nu `tower-backups` als tweede plek noemen. Druk daarna opnieuw op **Herstelkit downloaden** onder **Instellingen, Herstel**, omdat de kit de nieuwe bestemming en haar inloggegevens vermeldt.

**Cloudflare R2 werkt op dezelfde manier,** met een objecttoken in plaats van een sleutel. Plak het S3-adres dat het dashboard van R2 toont, met de bucket erop, en BombVault vult de server en de bucket in.

### Voorbeeld: een rest-server die je zelf draait {#example-rest-server}

Een rest-server in append-only-modus weigert verwijderen, dus een kopie erop overleeft, zelfs als iemand je server overneemt. Een tweede machine, bij een vriend of in de garage, is genoeg. Een gekoppelde BombVault kan dit voor je instellen, zie [Ontvangserver](#receiving-server). Om het met de hand op een willekeurige Docker-host te draaien:

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

De laatste regel vraagt om een wachtwoord en schrijft de gebruiker naar het bestand `.htpasswd` in de datamap.

1. Druk in BombVault op **Bestemming toevoegen** en kies **rest-server** onder **NAS of server**.
2. Vul het adres in met de gebruiker als map, zoals `http://192.168.2.20:8000/bombvault`, en de **Gebruikersnaam** en het **Wachtwoord**. Druk op **Verbinding testen**.
3. Zet bij **Bescherming** **Bescherming tegen verwijderen (append-only)** aan. BombVault schoont daar dan nooit op en laat dat aan de andere kant over.
4. Vink de onderdelen aan en druk op **Bestemming opslaan**.
5. Zodra de eerste kopie er is, open je de pagina van de bestemming en druk je op **Bescherming tegen verwijderen nu testen**. BombVault probeert een testobject te verwijderen. De kaart hoort **Laatst: verwijderen geweigerd** te zeggen, en **Bescherming tegen verwijderen** op het overzicht verandert in **gecontroleerd**. Zegt het **Laatst: verwijderen niet geweigerd**, dan ontbreekt `--append-only`.

Omdat niets deze kopie van hieruit opschoont, stel je **Groeibudget (GB, 0 = uit)** in het venster van het onderdeel in, geopend met **Bewerken** in zijn rij van **Offsite-kopieën**, zodat je het hoort voordat de schijf volloopt. Met een zelfondertekend certificaat, zie de opmerking over `certs` hierboven.

## De eigen map van een onderdeel {#own-folder}

Elk onderdeel heeft een eigen map: het repository waarnaar zijn back-ups eerst worden geschreven. Kopieën naar andere bestemmingen gaan van daaruit. Een nieuwe installatie houdt de eigen mappen op deze server, onder `/mnt/user/bombvault`, en samen vormen ze de bestemming **Lokaal**.

Een eigen map kan ook in een bestemming liggen die je al hebt, zoals een NAS, een rest-server of een S3-bucket. Het onderdeel maakt dan rechtstreeks daar zijn back-up, zonder lokale kopie ertussen, en de eigen map volgt de aanmelding, bescherming tegen verwijderen en bandbreedtelimieten van die bestemming. BombVault schoont hem na elke back-up op volgens de kaart **Hoe lang bewaren** van de bestemming, en de eigen waarden van een onderdeel onder **Regels per onderdeel** gaan voor op de kaart. De bestemming toont het onderdeel in haar kaart **Eigen mappen**, en **Bestemmingen per onderdeel** toont de bestemming als thuis van het onderdeel.

### Een eigen map verplaatsen {#move-own-folder}

1. Open de bestemming die de eigen map moet bevatten en kies **Als eigen map gebruiken**, of **Onderdelen toevoegen** in haar kaart **Eigen mappen**. Om meerdere onderdelen tegelijk te verplaatsen, kies je **Eigen mappen verplaatsen** naast **Eigen mappen hier** in de kaart **Verbinding** van de bestemming Lokaal.
2. Kies de onderdelen onder **Onderdelen** en de plek onder **Verplaatsen naar**. **Een andere plek** neemt een map op deze host of het adres van een extern repository dat met de hand is getypt, met inloggegevens uit je opgeslagen inloggegevens.
3. Lees **Wat er gebeurt**. Voor elk onderdeel zegt het waar nieuwe back-ups heen gaan, of een kopie die al op de nieuwe plek staat de eigen map wordt, en welke back-ups eerst worden overgekopieerd.
4. **Al gemaakte back-ups meekopiëren** staat standaard aan. Uitgeschakeld blijven de oude back-ups waar ze zijn, maar het item toont ze niet meer. **Uit een oude back-upmap** op de pagina Herstel leest ze nog steeds, met het adres van de map en, bij een externe map, de login ervan.
5. Kies **Omzetten**. Bewaart de kaart **Hoe lang bewaren** van de nieuwe plek minder dan de regel die het onderdeel vandaag heeft, dan vraagt BombVault eerst **Minder bewaren?** Daar neemt **Huidige waarden van** dat onderdeel **bewaren onder Regels per onderdeel** de waarden van vandaag mee, **Toch opslaan** gaat door, en **Terug** laat alles zoals het was. Moeten back-ups eerst worden gekopieerd, dan kopieert BombVault ze en zet daarna de eigen map om. Staan ze er al, dan controleert het ze vóór het omzetten.

Na de verplaatsing gaan nieuwe back-ups naar de verplaatste eigen map, ook voor een onderdeel dat **Lokaal** had uitgeschakeld en rechtstreeks naar een kopie back-upte. Een kopie die al op de nieuwe plek stond, wordt de eigen map, en het kopiëren daarheen eindigt. Waar die kopie sommige back-ups langer bewaarde dan de bewaarregels van het onderdeel, bewaart het onderdeel wat een van beide regels bewaart, zodat er geen verloren gaat.

Kan de verplaatsing niet doorgaan, dan zegt het venster waarom in een vak onder **Wat er gebeurt**. Staat alleen het kopiëren van de oude back-ups in de weg, dan laat het uitschakelen van **Al gemaakte back-ups meekopiëren** de eigen map zonder die verhuizen.

Een onderdeel heeft altijd een eigen map nodig. Het verwijderen van de bestemming die haar bevat, vraagt om de map waarheen het onderdeel verhuist, zie [Verplaatsen en verwijderen](#move-remove). Een eigen map waar nieuwe back-ups langs gaan, omdat het onderdeel rechtstreeks naar een kopie back-upt, is gemarkeerd met **(uit)** waar een bestemming toont wie haar gebruikt.

![Het venster dat een eigen map verplaatst, met Wat er gebeurt ingevuld voor één onderdeel.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![Het venster dat een eigen map verplaatst, met Wat er gebeurt ingevuld voor één onderdeel.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Voorbeeld: Containers verhuizen naar een NAS {#example-own-folder}

De cachepool loopt vol, en je hebt een bestemming genaamd `nas`, een SMB-share op een tweede machine. Het onderdeel Containers heeft 61 back-ups in zijn eigen map op deze server en kopieert naar Backblaze B2.

1. Open **Bestemmingen**, dan `nas`, en druk op **Als eigen map gebruiken**.
2. Vink **Containers** aan onder **Onderdelen**.
3. Lees **Wat er gebeurt**. Het zegt dat nieuwe back-ups van Containers naar de map op `nas` gaan, dat de 61 back-ups in Lokaal eerst daarheen worden gekopieerd, dat kopieën naar Backblaze B2 blijven gaan en dat bescherming, bandbreedtelimieten en het groeialarm van `nas` komen.
4. Laat **Al gemaakte back-ups meekopiëren** aan en druk op **Omzetten**.

BombVault kopieert de back-ups op de achtergrond en zet daarna de eigen map om. De pagina van `nas` toont Containers onder **Eigen mappen**, een item dat zijn onderdeel volgt noemt `nas` zijn thuis, en de volgende back-up komt daar terecht, waarbij de kopie naar B2 vanaf `nas` vertrekt. Vanaf nu is het de kaart **Hoe lang bewaren** van `nas` die de map opschoont. De oude map op deze server blijft staan waar hij is, en BombVault verwijdert daar niets.

### Een extern adres met de hand typen {#remote-primary-repositories}

De kaart **Verbinding** van de bestemming Lokaal neemt nog steeds een extern adres voor de eigen map van een onderdeel: **Wijzigen** naast het onderdeel opent een venster met een schakelaar **Lokaal / Extern**. **Extern** neemt een restic-adres zoals `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` of `rclone:remote:bucket/path`, en **Beveiligingsinstellingen voor extern primair repository** voegen een verbindingstest, upload- en downloadlimieten, append-only-bescherming met dezelfde sabotagetest als een off-site-bestemming krijgt, en een groeibudgetalarm toe.

Zo'n adres meldt zich aan met de standaardinloggegevens. Is het precies de map die een bestemming die je hebt voor het onderdeel bijhoudt, dan volgt het die bestemming vanzelf, inclusief de kaart van de bestemming en de schakelaar **Beschikbaar**, tenzij het anders inlogt of append-only is waar de bestemming dat niet is. Een regel die de map tot dan opschoonde, blijft als eigen regel van het onderdeel staan onder **Regels per onderdeel**. Ligt het in een andere map van die bestemming, dan toont de pagina van die bestemming het onderdeel onder **Staat al onder deze bestemming**, en het overnemen geeft het de inloggegevens, opslagklasse en bescherming tegen verwijderen van de bestemming. De eigen map met **Als eigen map gebruiken** naar een bestemming verplaatsen is de eenvoudiger weg, omdat het venster de al gemaakte back-ups controleert voordat er iets verandert.

## Plaatsing per item {#placement}

De kaart **Planning en bestemming** op het tabblad Instellingen van een item heeft een rij knoppen: **Lokaal** en één knop per off-site-bestemming van het onderdeel. Verlichte knoppen krijgen de back-ups van het item.

- Met **Lokaal** verlicht wordt het item geschreven naar het repository dat onder **Opgeslagen op** staat en gekopieerd naar elke andere verlichte bestemming. Maak een bestemming donker en ze krijgt niets nieuws meer van dit item. Alleen Lokaal kopieert nergens heen, wat past bij data die al een tweede kopie heeft, bijvoorbeeld een share die op een NAS staat.
- Met **Lokaal** donker wordt het item rechtstreeks geschreven naar het directe repository van de eerste verlichte bestemming en vandaar gekopieerd naar de andere verlichte. De eerste keer maakt een dialoogvenster dat directe repository aan.
- Eén knop blijft verlicht, omdat een back-up een plek nodig heeft. Om een item van back-ups uit te sluiten, gebruik je **Niet back-uppen**.

BombVault verplaatst nooit back-ups tussen repository's. Een item dat zijn onderdeel volgt, gaat mee als de standaard van het onderdeel verandert: zijn volgende back-up komt op de nieuwe plek, en de back-ups die hij heeft, blijven staan waar ze zijn. Een bestemming die op het item zelf is gekozen, ligt vast vanaf zijn eerste back-up. De kaart **Repository** op het tabblad Instellingen noemt haar. De kopieën kunnen op elk moment veranderen. Een bestemming die een item niet meer krijgt, houdt de kopieën die ze heeft en snoeit ze bij de volgende off-site-run van het onderdeel naar haar eigen bewaarbeleid. Van append-only-bestemmingen kan niets worden verwijderd.

De plek die een item verliet, houdt zijn back-ups. Op het tabblad **Back-ups** van het item en bij **Herstel** staat ze als **Alleen eerdere back-ups**, een herstel kan er nog uit lezen, en een opschoonronde daar laat die back-ups met rust.

**Standaard volgen** brengt een item met een eigen keuze terug naar zijn onderdeel. Het vraagt eerst, zegt waar nieuwe back-ups dan heen gaan en noemt wat elke bestemming niet meer krijgt en wat de volgende run zou uploaden.

Onder de rij zegt de kaart waar het item heen gaat en wat er daadwerkelijk is: hoeveel sites het bevatten, wanneer elke bestemming voor het laatst is gezien en of aan 3-2-1 wordt voldaan. Een site is de server met de originele data, elke off-site-bestemming en elk repository dat als **Buiten de locatie** is gemarkeerd. BombVault controleert kopieën en sites. Het onderdeel "twee media" van 3-2-1 controleert het niet.

### Standaarden per onderdeel {#placement-defaults}

De kaart **Bestemmingen per onderdeel** op de pagina Bestemmingen tekent de standaard van elk onderdeel als een 3-2-1-stroom. Elke lijn van een onderdeel naar een bestemming is een schakelaar, **Lokaal** inbegrepen, voor elk onderdeel, ook de flash, ZFS-datasets en de zelf-back-up. Een klik op een lijn of een stip schakelt haar meteen om, en gestippelde lijnen staan uit. Alleen het uitschakelen van **Lokaal** van een onderdeel vraagt eerst om bevestiging, in hetzelfde dialoogvenster als de standaardkaart, omdat het onderdeel dan rechtstreeks naar een kopie back-upt. **Annuleren** laat alles zoals het was. **Grafiek / Schakelaars** toont hetzelfde als rijen knoppen. Een schakelaar bepaalt waar nieuwe back-ups heen gaan. Items zonder eigen keuze, en de projectmappen van Compose-stacks, volgen meteen, dus hun volgende back-up gaat naar de nieuwe plekken. Al opgeslagen back-ups blijven waar ze zijn en kunnen nog van daaruit worden hersteld. Flash, ZFS-datasets en de zelf-back-up maken een back-up naar hun lokale map en kopiëren vanaf die back-up off-site. Hun lijn naar **Lokaal** kan ook worden uitgeschakeld. Hun back-ups gaan dan rechtstreeks naar hun off-site-bestemmingen, en een ZFS-item met een replica en helemaal geen bestemming stuurt alleen zijn replica.

Met **Lokaal** uit en alleen off-site-bestemmingen aan maakt een onderdeel rechtstreeks daarheen een back-up, en BombVault kiest daar zelf het repository. Containers, VM's en mappensets gaan naar het directe repository van de eerste bestemming die aanstaat, hieronder beschreven, en worden vandaar naar de andere gekopieerd. De flash, ZFS-datasets en de zelf-back-up schrijven elke back-up naar elke bestemming die aanstaat, en elke bestemming bewaart ze volgens haar eigen bewaarbeleid.

Een onderdeel waarvan elke lijn uitstaat, zegt **Zonder back-up**. Zijn items krijgen geen back-ups en tellen als onbeschermd.

Items die op hun pagina hun eigen bestemmingen kozen, houden die. De stroom zegt hoeveel het er in een onderdeel zijn, bijvoorbeeld **2 items wijken af**, en leidt ernaartoe: naar het tabblad Instellingen van het item als er één is, naar een lijst met waar elk heen back-upt als er meerdere zijn. **Standaard volgen** daar brengt een item terug.

Een nieuwe off-site-bestemming ontvangt elk item dat niet op Lokaal staat. Het venster dat haar toevoegt, zegt hoeveel items en, waar bekend, hoeveel geschiedenis dat is.

### Directe repository's {#direct-repositories}

Een direct repository is waar items rechtstreeks naar een off-site-bestemming back-uppen. BombVault zet het naast de map van de bestemming met `-direct` achter de naam, of, wanneer de bestemming op de root van haar bucket, haar remote of een rest-server-gebruiker staat, in een map erin die naar het onderdeel is genoemd, zoals `containers-direct`. In de stroom op de pagina Bestemmingen wordt het zonder vraag aangemaakt. Lokaal uitschakelen voor één item, zodat een bestemming zonder direct repository zijn thuis wordt, opent een dialoogvenster met de voorgestelde locatie, bijvoorbeeld `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, en een verbindingstest die niets aanmaakt. **Aanmaken en gebruiken** maakt het repository aan en wijst het item ernaar. Een direct repository heeft geen eigen instellingen. Het volgt de schakelaar, aanmelding, opslagklasse, limieten, append-only-instelling en het bewaarbeleid van zijn bestemming en verandert daarmee mee. De pagina van de bestemming toont het onder **Back-upt rechtstreeks hierheen**, met elk onderdeel dat er rechtstreeks heen back-upt en hoeveel items dat doen. Eén waar niets meer naartoe back-upt, zegt dat daar en kan worden verwijderd, en de back-ups op zijn plek blijven. Kan een nieuwe sleutel voor de bestemming het niet openen, dan houdt het directe repository de sleutel die het heeft en zegt het opslaan dat. Een item op een direct repository wordt vandaar naar de andere verlichte bestemmingen gekopieerd, nooit naar de bestemming waartoe het repository behoort. Zijn snapshots dragen de tag `bv:direct`, en elke andere bewaarronde bewaart ze, zodat een direct repository dat zijn koppeling met zijn bestemming verloor nooit veroudert volgens de lokale regels. B2 wordt bereikt via zijn S3-eindpunt, met het sleutel-ID en de applicatiesleutel ingevoerd als de S3-sleutels. Een sleutel die beperkt is tot de eigen map van de bestemming, kan de map ernaast niet bereiken, dus beperk de sleutel in plaats daarvan tot de map erboven.

### Buiten de locatie {#off-the-premises}

Een bestemming of een repository kan worden gemarkeerd als **Buiten de locatie**. Externe starten gemarkeerd. Schakel het uit voor een rest-server in hetzelfde gebouw. De markering telt alleen sites en 3-2-1 op de kaarten. Ze verandert geen kopie.

Een ZFS-replica is een kopie buiten de locatie, maar geen back-up met geschiedenis. Ze bewaart alleen de snapshots die je instelt, en oude versies en losse bestanden komen nog uit de back-ups. Ze gaat van de ene ZFS-server rechtstreeks naar de andere, via SSH naar een ZFS-server of naar het ontvangsteindpunt van een gekoppelde instantie. De koppelgroep draagt alleen het verzoek. Zie [Replica](zfs-datasets.md#replica).

### Na een herinstallatie {#after-a-rebuild}

Kopieerkeuzes leven in de eigen instellingen van BombVault. Na een herbouw via **Back-ups ontdekken** of de Noodassistent zonder herstelde instellingen zijn ze weg, en alles kopiëren zou de items die je had weggelaten opnieuw naar B2 sturen. Hetzelfde geldt als een onderdeel dat vanuit deze database nooit off-site kopieerde, eigen back-ups vindt bij een bestemming, of oudere in zijn eigen map, bijvoorbeeld nadat een bestemming is toegevoegd over de back-ups van een eerdere installatie met dezelfde `APP_KEY`. Off-site-kopieën van dat onderdeel wachten dan op je beslissing.

**Aandacht nodig** in het overzicht en de pagina van elke bestemming waar het onderdeel naartoe kopieert, vragen of de back-ups van deze server zijn. **Als back-ups van deze server overnemen** bevestigt de standaard van het onderdeel, en zijn kopieën draaien weer vanaf de volgende ronde. **Eerst de regels controleren** leidt naar de rij van het onderdeel in **Bestemmingen per onderdeel**, waar je ziet waar elk item naartoe gaat en een verloren regel opnieuw kunt instellen op de pagina van het item. Het kopiëren blijft daar wachten tot je op **Als back-ups van deze server overnemen** drukt, dat eerst toont wat de volgende run kopieert en de namen in de back-ups die geen item hebben, die je kunt weglaten. De nieuwste back-up van elk wachtend item meldt dat in zijn run, met een link naar de beslissing, en er gaat een melding uit als meldingen aanstaan. Een instellingenbestand importeren brengt regels en standaarden terug, maar neemt de beslissing niet voor je.

![Aandacht nodig na een herinstallatie: de mappensets vonden back-ups van vóór deze database, en hun off-site-kopieën wachten op de beslissing.](assets/screenshots/offsite/copy-decision-dark.png#only-dark)
![Aandacht nodig na een herinstallatie: de mappensets vonden back-ups van vóór deze database, en hun off-site-kopieën wachten op de beslissing.](assets/screenshots/offsite/copy-decision-light.png#only-light)

## Bescherming tegen verwijderen (append-only) {#delete-protection-append-only}

Schakel bescherming tegen verwijderen in voor een off-site-bestemming, zodat ransomware of een gecompromitteerde host je back-ups niet kan verwijderen of herschrijven. De andere kant (een `restic/rest-server` die in modus `--append-only` draait) **dwingt** het af. BombVault **controleert** het alleen en toont nooit groen op basis van een configuratiebewering alleen. Een map op deze server kan verwijderen niet weigeren, omdat wie BombVault beheert ook de map beheert, dus lokale bestemmingen bieden de schakelaar niet aan.

De schakelaar hoort bij de bestemming: een onderdeel dat daarheen kopieert, neemt hem van de bestemming over, dus zet hem aan op de pagina van de bestemming onder **Bestemmingen**. De geplande **Verwijderbeveiliging controleren** draait alleen zolang een onderdeel naar een bestemming kopieert waarvan de schakelaar aanstaat. De andere kant moet met `--append-only` zijn gestart. Met de image `restic/rest-server` gaat dat, op TrueNAS net als op elke andere Docker-host, in de variabele `OPTIONS`.

Sommige plekken kunnen een verwijdering helemaal niet weigeren, een cloudschijf zoals OneDrive bijvoorbeeld. Houd je daar met opzet kopieën, schakel dan **Accepteren zonder bescherming tegen verwijderen** in op de pagina van de bestemming. Het overzicht toont de ontbrekende bescherming tegen verwijderen van een onderdeel dat alleen naar geaccepteerde bestemmingen kopieert dan als een geaccepteerd risico, een grijze ring in plaats van een amberkleurige stip, en de kleur van het onderdeel volgt zijn andere controles. Verder verandert er niets: de back-ups daar kunnen nog steeds worden verwijderd, BombVault schoont ze nog steeds op volgens de bewaarregels van de bestemming, en de schakelaar verdwijnt zodra bescherming tegen verwijderen aanstaat. Een kopie die naar een andere plek verhuist, verliest de acceptatie. Bescherming tegen verwijderen inschakelen wist haar ook.

De snelste weg naar een append-only-kopie is een gekoppelde BombVault met een [ontvangende server](#receiving-server): die draait de rest-server voor je, in append-only-modus.

In een [gedeelde repository](#shared-repository) geldt dit voor de leden en niet voor de server die er eigenaar van is. Die server komt rechtstreeks bij de map en vergeet en schoont daar op.

!!! note "Een geslaagde verwijdering onder `/locks/` is te verwachten"
    Append-only betekent niet dat er nooit iets kan worden verwijderd. restic moet zijn eigen locks nemen en loslaten, dus `/locks/` blijft per ontwerp beschrijfbaar en verwijderbaar. Snapshots en de data erachter, wat ransomware zou aanvallen, kunnen niet worden verwijderd. Test je zelf de andere kant, dan is een verwijdering die onder `/locks/` slaagt juist gedrag en geen gat in de bescherming.

!!! warning "Append-only-repository's worden nooit vanaf deze machine opgeschoond"
    Een append-only-bestemming schoont oude snapshots nooit vanaf hier op. Opschonen is het werk van de andere kant. Stel er een **groeibudgetalarm** voor in, zodat je een melding krijgt voordat de grootte van het repository op hol slaat.

## Sabotagetest {#tamper-test}

BombVault bewijst de append-only-garantie periodiek door daadwerkelijk te proberen iets te verwijderen in het off-site-repository, gericht op een niet-bestaand object. **Bescherming tegen verwijderen nu testen** op de pagina van een bestemming voert het op verzoek uit, en **Verwijderbeveiliging controleren** onder **Instellingen, Verificatie, Automatische controles** voert het volgens een planning uit.

Het uitschakelen van **Verwijderbeveiliging controleren** verbergt geen gat. Een bestemming met bescherming tegen verwijderen die nooit is getest, leest dan "niet bewezen, zijn controle staat uit", en een bestemming zonder bescherming tegen verwijderen leest in beide gevallen "geen".

Een kopie die is uitgeschakeld, wordt niet getest, en de bestemming zegt dat. De verbindingstest draait er wel op, omdat die alleen leest, dus een kopie kan worden gecontroleerd voordat ze wordt ingeschakeld.

- **Geweigerd** betekent beschermd.
- **Geaccepteerd** betekent niet beschermd.
- Een **niet eenduidig** resultaat (server onbereikbaar, aanmeldfout) draait het opgeslagen oordeel nooit om.

Een echte omslag van beschermd naar onbeschermd stuurt één waarschuwing.

## DR-oefeningen {#dr-drills}

BombVault biedt twee niveaus van bewijs dat je back-ups echt te herstellen zijn, niet alleen aanwezig.

- **Herstelverificatieoefeningen (lokaal).** BombVault voert periodiek `restic check --read-data-subset` uit (begrensd, nooit een schijfvullend volledig herstel). Het ritme staat onder **Instellingen, Verificatie, Automatische controles**, en de kaart **Back-ups controleren** op dezelfde tegel toont het resultaat.
- **Testherstel (off-site).** BombVault herstelt een echt doel uit het off-site-repository in een afgeschermde sandbox, controleert het bestand voor bestand en byte voor byte, en ruimt daarna op. Dat bewijst dat je vanuit off-site kunt herstellen, niet alleen dat het repository antwoordt.

De kaart **Beschermingsstatus** op het overzicht vat dit per onderdeel samen in vier controles: een actuele back-up op deze server, een kopie off-site, bescherming tegen verwijderen daar en een testherstel vanaf daar. Een klik op een onderdeel toont de details, en de kaart wordt alleen groen op geverifieerde feiten. Een bestemming die je zonder bescherming tegen verwijderen accepteerde, telt daar als een geaccepteerd risico, niet als een waarschuwing.

## Instanties koppelen {#pairing}

Meerdere BombVaults werken samen als één groep: ze tonen elkaars beschermingsstatus, nemen rollen voor elkaar op zich en kunnen elkaars sleutel bewaren. Een instantie treedt met twaalf woorden tot de groep toe.

![De koppelpagina met zijn drie stapkaarten.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![De koppelpagina met zijn drie stapkaarten.](assets/screenshots/offsite/pairing-light.png#only-light)

Open op de eerste instantie **Instellingen, Koppeling** en druk op **Frase genereren**. Drie stapkaarten tonen hoe het gaat. Twaalf woorden verschijnen in een venster met een knop **Kopiëren**. Open op elke andere instantie dezelfde plek, druk op **Frase invoeren** en plak of typ ze, of druk op **Plakken** in dat venster. Een woord dat niet op de lijst staat, wordt tijdens het typen met zijn positie genoemd, en het laatste woord draagt een controlesom, dus een verkeerd getypt of verwisseld woord wordt opgemerkt voordat er iets koppelt. Genereer de frase op één instantie alleen: twee instanties die er allebei een genereren, vormen twee aparte groepen. Komt er een minuut niemand opdagen, dan biedt de pagina twee uitwegen: toon de woorden opnieuw om ze daar in te voeren, of voer de woorden van de andere instantie in en sluit in één stap aan bij haar groep. Koppelen werkt zonder inlogwachtwoord, maar stel er een in: zonder kan iedereen die de webinterface kan openen de woorden lezen en, via de groep, rollen aanvragen die het restic-wachtwoord van de instanties daarin uitgeven. De koppelkaart zegt dat tot er een wachtwoord is ingesteld. Met een wachtwoord vraagt het opnieuw tonen van de frase erom. **Groep verlaten** haalt een instantie er weer uit. De kaart **Instanties** op de koppelpagina toont de leden van de groep. Het beeld van wie wat voor wie doet, staat op de pagina Instanties.

Iedereen die de woorden kent, kan tot de groep toetreden, dus behandel ze als een wachtwoord.

**Hoe leden elkaar bereiken.** Elke instantie leert haar eigen adres op het netwerk van je browser op het moment dat je je aanmeldt, getoond onder **Bekende adressen** in de kaart **Deze instantie** op de koppelpagina, samen met de adressen die de instantie zelf vond. Corrigeer het daar als er een reverse proxy of een ongebruikelijke poort voor staat. Op hetzelfde netwerk maken leden dat adres via multicast bekend en praten ze rechtstreeks, en waar multicast een containernetwerk niet kan oversteken, zoals de standaardbridge van Docker, zoekt een instantie in plaats daarvan in haar eigen subnet naar de anderen met een ondertekende aanroep die alleen een groepslid kan beantwoorden, zodat het koppelen ook zonder relay in seconden klaar is. Vindt het niets, dan neemt **Vind je hem niet?** één adres met de hand, voor een ander subnet of een niet-standaardpoort. Instanties op verschillende netwerken lopen via een relay, gekozen in één rij van vier in de kaart **Relay** op dezelfde pagina. Elke vorm komt met een plaatje:

- **Projectrelay** (de standaard): `parleyport.halleluja.design`, de relay die ook KnightLoader gebruikt. Niets in te stellen.
- **Eigen relay**: de container [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) uit Unraid Community Apps (ook onder **Instellingen, Apps**). Vul zijn adres in op elke instantie die hem moet gebruiken.
- **BombVault-instantie**: een van je instanties die al van buitenaf bereikbaar is, met **Als relay dienen** ingeschakeld. Die instantie antwoordt dan op `/relay/connect` op haar eigen adres, achter de reverse proxy en het certificaat die ze al heeft, en laat alleen je groep binnen. Vul haar adres in op elke instantie die haar moet gebruiken.
- **Geen relay**: leden vinden elkaar automatisch op hetzelfde netwerk, en nergens anders.

**Wat de relay ziet.** Een knop met dezelfde naam naast de vier vormen toont dit voor de vorm die je koos. Elke aanroep tussen leden wordt verzegeld met AES-256-GCM onder een sleutel die uit de twaalf woorden is afgeleid, en die sleutel verlaat je instanties nooit. De relay leert een hash die de verbindingen groepeert, voor welke instantie een bericht is, hoe groot het is en wanneer het langskomt. Een directe aanroep op het lokale netwerk wordt op dezelfde manier verzegeld en ook ondertekend, zodat niets afhangt van het zelfondertekende certificaat dat een instantie serveert. Aanroepen die iets uitgeven (rolantwoorden, een restic-wachtwoord, logins van ontvangers, bewaarkopieën van sleutels) worden ook ondertekend met een sleutel van het lid dat ze verstuurt, bij de eerste keer vastgezet. Zie [Beveiligingsmodel](configuration.md#security-model).

**Wat er over de groep reist.** De beschermingsstatus op de pagina Instanties, een verzoek om één onderdeel nu te controleren, rolverzoeken en hun antwoorden, opslagaanbiedingen, en wat een ontvanger of ophaler nodig heeft zodra zijn rol is toegestaan: de repositorylocaties van de andere instantie en haar restic-wachtwoord. Back-updata nooit. Die gaat nog steeds rechtstreeks naar de restic-backends. De APP_KEY ook niet: het restic-wachtwoord opent de repository's van die instantie en niets anders, niet haar opgeslagen geheimen, sessies of vrijgavecodes.

**Items van vóór het koppelen.** Instanties die met een fleet-token zijn toegevoegd, en ontvangen repository's en ophaalbronnen die met de APP_KEY van de andere instantie zijn ingesteld, blijven na de update bestaan en zijn gemarkeerd als **Opnieuw koppelen**. Ze blijven werken: bij zijn eerste start vervangt BombVault elke opgeslagen APP_KEY door het restic-wachtwoord dat ervan is afgeleid. Koppel beide instanties, bewerk dan het item en kies zijn instantie. Tot dan staan zulke items in de kaart **Zonder instantie** op de pagina Instanties.

De enige plek die nog met de hand een APP_KEY neemt, is [Uit een oude back-upmap](#restore-from-another-bombvault-repo), voor het geval de andere instantie weg is en in een groep niet kan antwoorden.

## Instanties en rollen {#roles}

De pagina **Instanties** (een instellingentegel die aan de zijbalk kan worden vastgezet) toont de groep als een net van wie wat voor wie doet, een kaart voor deze instantie en een kaart voor elk ander lid, de Android-telefoons en de ZFS-servers. **Koppeling** rechtsonder leidt naar de koppelpagina. Welke delen aanstaan, stel je in onder **Instellingen, Instanties, Meerdere servers**: **Instanties**, **Ontvanger** en **Ophalen**.

![De pagina Instanties: de groep als net, en een kaart per instantie met zijn roltegels, waarvan één Inactief.](assets/screenshots/offsite/instances-dark.png#only-dark)
![De pagina Instanties: de groep als net, en een kaart per instantie met zijn roltegels, waarvan één Inactief.](assets/screenshots/offsite/instances-light.png#only-light)

Elke kaart toont de beschermingsstatus van het lid en zijn rollen als tegels. Een rol is iets wat een instantie voor een andere doet:

- **Ontvanger** bewaart de off-site-kopieën van deze server in een rest-server in append-only-modus. Deze server verstuurt na elke back-up en kan daar niets verwijderen, ook niet als iemand hem overneemt.
- **Ophaler** haalt de back-ups van deze server op volgens zijn eigen planning. Deze server hoeft daarvoor niet wakker te zijn. De instantie die ophaalt, moet **Ophalen** ingeschakeld hebben voordat ze het verzoek kan toestaan.
- **ZFS-server** neemt de ZFS-replica's van deze server aan. Elk ZFS-item kiest hem onder **Replica** in zijn instellingen.
- **Sleutelbewaarder** bewaart een verzegelde kopie van de herstelkit van deze server. Zie [Sleutelbewaarder](#key-keeper).

Een rol inschakelen stuurt een verzoek, en niets start tot iemand op de andere instantie op **Toestaan** drukt. Het verzoek verschijnt op de kaart van die instantie als badge, en zijn **Details** tonen het met **Toestaan** en **Afwijzen**. Een toegestane rol kan daar op elk moment worden ingetrokken. Een gestippelde lijn in het net is een rol die nog wacht. Een instantie op BombVault 9.x kan een verzoek niet beantwoorden, dus een rol is daar actief zodra je hem inschakelt, en een verzoek van een 9.x-instantie wacht hier op **Toestaan**.

**Details** op een kaart opent de rest: de ontvangen repository's en ophaalbronnen van dat lid, opslagaanbiedingen, bewaking, **Verwijderen** en **Sleutel vergeten**.

Onder de kaarten horen de kaart **Gedeelde repository** op een machine met een ontvangserver en de kaart **Gedeelde repository's** op een lid bij de [gedeelde repository](#shared-repository).

Een lid kan zijn eigen off-site-opslag via de groep aan een ander lid aanbieden. De andere beheerder ziet het aanbod in de **Details** van het lid en accepteert of wijst het af. Accepteren maakt opgeslagen inloggegevens en een gewone bestemming aan, na dezelfde controles als bij een bestemming die je intypt. De bestemming komt met bescherming tegen verwijderen ingeschakeld, omdat het recept dat het aanbod toont een append-only rest-server is, dus **Hoe lang bewaren** geldt daar niet. Elk aanbod heeft een eigen login en een eigen map. Bied je een onderdeel opnieuw aan, aan hetzelfde lid of een ander, dan blijven de kopieën van de eerste partner dus met rust: voeg de nieuwe regel toe aan `.htpasswd` en laat de server draaien. Alleen verbindingsgegevens reizen zo, nooit back-updata. **Nu controleren** vraagt een lid het repository van één onderdeel te controleren. Niets op de pagina kan een back-up of herstel starten of iets verwijderen op een andere machine.

Instanties die zijn uitgeschakeld en telefoons met de app gesloten, houden hun kaart als **Inactief**, in grijs, met wanneer ze voor het laatst zijn gezien, en staan gedimd in het net tot je ze **Verwijderen**. Direct nadat deze server start, zegt zo'n kaart **Controleren** tot de groep weer terug is. **Verwijderen** werkt op zo'n kaart ongeacht welke delen aanstaan, en beëindigt ook de openstaande verzoeken van het lid, zijn ongebruikte ZFS-ontvangstplekken en zijn wachtende sleutelverzoeken. De kaart **Instanties** op de koppelpagina toont deze leden ook als **Inactief**.

## Ontvanger en ophaler {#receiver}

Op de machine die append-only-kopieën van een andere BombVault **ontvangt**, geven de ontvangen repository's je onafhankelijke, alleen-lezen bewaking op de ontvangende hardware, zodat een stil falen aan de andere kant niet onopgemerkt blijft. Schakel **Ontvanger** in onder **Instellingen, Instanties**, open dan de **Details** van de verzendende instantie op de pagina Instanties en voeg het ontvangen repository toe (alleen-lezen, geopend met het restic-wachtwoord van de verzendende instantie, dat het via de [koppelgroep](#pairing) krijgt). Je krijgt:

- **Een inventaris per bron**, zodat je precies kunt zien welke containers, VM's en mappensets zijn aangekomen. Back-ups die de verzendende instantie maakte voordat ze een [eigen host](#backup-host) had, tellen mee voor dezelfde bron.
- **Laatst ontvangen** per bron, zodat je weet hoe vers elke is.
- **Een onafhankelijke `restic check`** op de ontvangende hardware, zodat de integriteit wordt geverifieerd waar de data echt staat, niet alleen bij de afzender.
- **Een dodemansknop:** een waarschuwing als een bron binnen een venster dat je instelt, niets meer verstuurt. Hij kijkt één keer per dag, om 09:15. Je krijgt ook één waarschuwing als het repository helemaal niet te lezen is, wat de controleplanning ook zegt, en als er een vol venster nadat je het hebt toegevoegd nog niets is aangekomen. Een repository dat je toevoegt vóór de eerste kopie van de afzender, is tot dan in orde.
- **Integriteitswaarschuwingen:** een waarschuwing als een controle aan de ontvangende kant mislukt. Een controle die het repository niet kon openen, zegt niets over de integriteit, dus ze laat geen oordeel achter en probeert het de volgende dag op hetzelfde tijdstip opnieuw.

Een repository op een andere server heeft de login van die server nodig: sla die op als [inloggegevens](#sign-ins) en kies ze in het venster onder **Inloggegevens**.

De ontvanger is strikt alleen-lezen. Hij schrijft nooit naar het ontvangen repository, dus hij kan de append-only-garantie waarop de afzender rekent nooit breken.

Een ophaler werkt andersom: met **Ophalen** ingeschakeld leest een instantie de back-ups van een ander lid volgens zijn eigen planning, in een eigen repository voor elke bron, zodat ze nooit als eigen back-ups van deze machine tellen. Zijn bronnen staan ook in de details van dat lid: de repositorylocatie, welk soort back-up het bevat en hoe vaak er wordt opgehaald. Het restic-wachtwoord komt via de groep, nooit de APP_KEY, en de andere kant hoeft niets verder in te stellen en hoeft niet te draaien. De bron wordt alleen gelezen, en elke kant houdt zijn eigen inloggegevens. Een ophaalactie draait op de dag en het tijdstip die de planning noemt, en ook een nieuwe bron wacht op dat tijdstip. **Nu ophalen** wacht niet. Een ophaalactie die gemist is doordat deze machine uitstond, draait zodra ze terug is. Een mislukte ophaalactie probeert het de volgende dag op hetzelfde tijdstip opnieuw, ook bij een wekelijkse planning, verschijnt in het activiteitenlogboek en stuurt één waarschuwing tot een ophaalactie weer lukt. Een bron met `rclone:` wordt geweigerd, omdat rclone hem met de remotes van deze instantie zou bereiken.

Een opgehaalde kopie groeit alleen. BombVault voegt eraan toe, maar schoont haar niet op en controleert haar niet, dus wat de bron allang vergeten is, blijft hier tot je de map verwijdert, en de pagina Bestemmingen telt haar niet mee. Ze staat in `pulled/<source id>` binnen de eigen map van het onderdeel, en als die map verhuist, blijft de kopie waar ze is en gaan latere ophaalacties daar verder, in plaats van alles opnieuw te kopiëren. Verwijder je haar, dan begint de volgende ophaalactie opnieuw in de eigen map die het onderdeel dan heeft.

### Ontvangserver {#receiving-server}

De ontvangende machine kan ook de rest-server draaien waarnaar de anderen kopiëren. Hij wordt ingesteld de eerste keer dat je een verzoek voor **Ontvanger** toestaat dat erom vraagt: het venster zegt dat de server een ontvangserver nodig heeft en vraagt om een map op een share, met **Nieuwe map** om er een te maken, en een poort (8000 tenzij een andere container hem gebruikt). BombVault dan:

1. weigert als er al een container met de naam `rest-server` bestaat of een andere container de poort bezet,
2. haalt `restic/rest-server` op en start hem via de Docker-socket in append-only-modus met privérepository's en een loginbestand in de map,
3. schrijft zijn Unraid-template naar de flash, zodat de container bewerkbaar blijft in het Docker-tabblad, of biedt de template als download aan als de flash buiten bereik is,
4. voert de sabotagetest ertegen uit en toont of hij verwijderen weigert.

De instanties in je groep vinden de server dan onder de aanbieders bij **Uit je groep**, genoemd naar de ontvangende machine, en de rol **Ontvanger** biedt hem aan als de veilige keuze. Elke instantie krijgt de eerste keer dat ze de server gebruikt een eigen login en schrijft daar alleen in haar eigen map. Met **Ontvanger** ingeschakeld onder **Instellingen, Instanties** opent **Ontvangserver** op de kaart van deze instantie de server opnieuw. Zijn venster toont de logins onder **Partners met een login**, en **Login intrekken** neemt er een weg. Wat die instantie al kopieerde, blijft in de map. De installatie maakt ook één **Login buiten de groep** aan, waarvan het venster het wachtwoord één keer toont.

Een instantie die de ontvangende machine alleen via de relay bereikt, kan de server niet gebruiken, omdat de relay geen back-ups draagt. Voeg eerst het adres van de ontvangende machine toe op de koppelpagina. Draait BombVault op een eigen IP-adres (bijvoorbeeld op br0), vul dan **Adres voor partners (optioneel)** in, omdat de server op het adres van de host luistert.

Een ontvangende machine kan ook een [gedeelde repository](#shared-repository) voor de groep bijhouden.

## Gedeelde repository {#shared-repository}

Meerdere servers van een groep kunnen samen naar één restic-repository kopiëren. Gegevens die ze gemeen hebben, zoals dezelfde mappenset of dezelfde appdata, staan daar maar één keer, dus samen hebben ze minder ruimte nodig dan met elk een eigen repository.

Eén server van de groep is de eigenaar van de repository. Alleen hij vergeet daar oude back-ups en schoont op. De anderen zijn leden: ze bereiken de repository via een tweede rest-server in append-only-modus en kunnen alleen toevoegen. Elke server schrijft onder een [eigen host](#backup-host), zodat de back-ups gescheiden blijven, ook waar twee servers een item met dezelfde naam hebben.

Wat er nodig is:

- een [koppelgroep](#pairing),
- **Back-ups versleutelen** ingeschakeld, op de eigenaar en op elk lid, omdat elke server een eigen restic-sleutel krijgt,
- een [ontvangende server](#receiving-server) op de eigenaar die de leden kunnen bereiken. Draait BombVault op een eigen IP-adres, vul dan eerst **Adres voor partners** in het venster van de ontvangserver in.

Een server is eigenaar van hoogstens één gedeelde repository.

### Instellen {#shared-set-up}

Zodra de ontvangserver draait, heeft de pagina **Instanties** van zijn machine onder de instanties een kaart **Gedeelde repository**. Druk op **Gedeelde repository instellen**, kies een **Map voor de gedeelde repository** op een share met veel ruimte, laat de **Poort** op 8001 tenzij een andere container hem gebruikt, en druk op **Instellen en starten**. BombVault dan:

1. weigert als er al een container met de naam `rest-server-shared` bestaat of een andere container de poort bezet,
2. maakt de repository aan in een map `shared` daarin, met het wachtwoord van deze server,
3. start `restic/rest-server` als `rest-server-shared`, in append-only-modus met een loginbestand in de map,
4. schrijft op Unraid zijn template naar de flash, zodat de container bewerkbaar blijft in het Docker-tabblad,
5. voert de sabotagetest ertegen uit. **Opnieuw controleren** op de kaart voert hem nog eens uit.

De kaart toont de **Leden**, elk met zijn sleutels en zijn laatste back-up, de verzoeken om mee te doen, en **Verwijderen** voor elk lid.

### Meedoen {#shared-join}

Open op de server die wil meedoen **Bestemming toevoegen**. Onder **Uit je groep** verschijnt de eigenaar een tweede keer, als **Gedeelde repository op** en zijn naam. Kies je hem, dan gaat er een verzoek naar de eigenaar. Daar staat het op de kaart **Gedeelde repository** met **Toestaan** en **Afwijzen**.

Na **Toestaan** geeft de eigenaar via de groep een eenmalige sleutel en een login op zijn tweede rest-server. Het lid voegt daarmee een eigen restic-sleutel toe, en daarna verwijdert de eigenaar de eenmalige sleutel. Een lid dat niet binnen een dag klaar is, verliest de eenmalige sleutel en de login weer en kan opnieuw vragen. Kon de eigenaar het lid niet bereiken met zijn antwoord, dan biedt de kaart **Nog eens laten weten** aan.

De eigenaar weigert een server zolang die nog niet onder een eigen host schrijft, wat betekent dat hij een oudere release draait, en ook wanneer een ander lid al onder dezelfde host schrijft. Twee servers delen een host als ze een `APP_KEY` delen.

Zodra het lid erbij is, verschijnt onder **Bestemmingen** een bestemming met de naam **Shared with** en de naam van de eigenaar, met elk onderdeel uitgeschakeld. Schakel op de pagina van de bestemming de onderdelen in die daarheen moeten kopiëren. De bestemming heeft bescherming tegen verwijderen en geen eigen bewaarregels, omdat alleen de eigenaar daar vergeet. Alleen back-ups onder de eigen host van het lid gaan erheen, dus oudere back-ups onder de host `bombvault` blijven waar ze zijn.

De eigenaar kan ook zijn eigen back-ups erin kopiëren: **Ook de back-ups van deze server erin kopiëren** op zijn kaart voegt de bestemming ook bij hem toe. Voor de eigenaar staat die kopie op zijn eigen schijven, en niets houdt hem tegen om daar te verwijderen. Het is dus geen kopie buiten de deur.

### Wie opruimt, en wanneer {#shared-upkeep}

Alleen de eigenaar vergeet en schoont op. Zijn instellingen staan in het deel **Gedeelde repository** van het venster **Ontvangserver**, dat je opent vanaf de eigen kaart van de eigenaar op de pagina Instanties:

- **Opruimvenster** bepaalt wanneer de opschoning loopt, dagelijks of wekelijks. Tot je het verandert, begint het op zondag om 03:00, en **Lengte van het venster in uren** is 6.
- **Ongebruikte ruimte die een opschoning mag laten staan, in %** is 5 tenzij je het verandert. Een hogere waarde herverpakt minder, dus de opschoning is sneller klaar.
- **Teruggelezen gegevens na een opschoning, in %** is 5 tenzij je het verandert. Na elke opschoning controleert de eigenaar de repository en leest dit deel van de gegevens terug.
- **Hoe lang bewaren** volgt per onderdeel de off-site-regels van de eigenaar, en **Een eigen regel instellen** geeft de gedeelde repository een eigen regel. De back-ups van elke server verouderen los van elkaar.
- **Nu opruimen** vergeet, schoont op en controleert meteen, nadat de leden het te horen hebben gekregen.

Buiten het venster vergeet de eigenaar één keer per dag, en de ruimte komt terug met de volgende opschoning. Voor een opschoning laat hij het de leden weten, en die kopiëren niets naar de repository tot het venster sluit. Een lid dat nog aan het kopiëren is, krijgt tot een half uur om af te ronden, anders schuift de opschoning door naar het volgende venster. Een kopie die in het venster valt, wacht en staat als uitgesteld in het activiteitenlogboek en op de kaart van het lid. De volgende ronde haalt hem in.

Waar de eigenaar twijfelt, bewaart hij meer. Een item dat een lid vasthoudt vanwege een [anomalie](anomalies.md), houdt al zijn back-ups. Een lid dat niet kan zeggen wat het vasthoudt, houdt die keer al zijn back-ups. Back-ups onder een host waaronder geen server van de groep schrijft, worden nooit vergeten. Het venster toont ook de ruimte: wat de repository inneemt, wat de servers apart nodig zouden hebben en wat dat bespaart. De getallen komen bij elke opruimbeurt.

### Wat leden zien {#shared-members}

Op een lid heeft de pagina Instanties een kaart **Gedeelde repository's**. Die toont elke gedeelde repository waar de server naartoe kopieert of waarvoor hij vroeg om mee te doen, en hoe zijn verzoek ervoor staat: **Wacht op een ja**, **Wordt lid**, **Doet mee**, **Afgewezen**, **Verlopen** of **Geen lid**. Voor een repository waarin hij meedoet, toont de kaart **Wie opschoont**, **Volgende opruimbeurt**, **Samen met** voor de andere leden, **Laatst hierheen gekopieerd** en hoeveel ruimte de repository inneemt en bespaart.

### Verlaten {#shared-leave}

**Verlaten** op de kaart van het lid haalt de server eruit: de eigenaar verwijdert zijn sleutel en login, en zijn bestemming daar wordt uitgeschakeld. **Verwijderen** op de kaart van de eigenaar haalt een lid er van de andere kant uit. In beide gevallen blijven de back-ups van de server in de repository en verouderen ze volgens de regels van de eigenaar. Bij de eigenaar staan ze onder **Vertrokken servers**, waar **Back-ups vergeten** ze allemaal vergeet zodra je de naam van de server intypt. Het volgende opruimvenster geeft de ruimte terug.

### De back-ups van andere servers {#shared-restore}

Elke server vindt zijn eigen back-ups in de gedeelde repository op het tabblad **Back-ups** van zijn items, net als elke andere kopie. De back-ups van de andere leden staan op de pagina Herstel onder [Andere servers](restore.md#other-servers).

### Na een noodgeval {#shared-emergency}

De [herstelkit](#recovery-kit) heeft een hoofdstuk **Shared repositories**. Dat noemt elke gedeelde repository met zijn eigenaar, de onderdelen die erheen kopiëren, de andere leden met hun hosts, en een opdracht `restic snapshots` met de host van deze server. De kit van de eigenaar noemt ook de poort, de map en de login van zijn rest-server.

In de Noodassistent staat de repository tussen de plaatsen als **Gedeelde repository bij** en de naam van de eigenaar. Een nieuwe installatie die de sleutel van een lid overneemt, schrijft onder dezelfde host, omdat de host uit de sleutel komt. Ze krijgt haar bestemming in de gedeelde repository terug en toont en herstelt daar alleen haar eigen back-ups.

Een nieuwe installatie die de sleutel van de eigenaar overneemt, wordt in zijn plaats eigenaar van de repository. Ze laat de back-ups verouderen en ruimt op zoals voorheen, en start de rest-server opnieuw op dezelfde map. Koppel haar opnieuw met de groep, stel onder **Instanties** haar ontvangserver in en stel de regels in het venster van de ontvangserver opnieuw in, omdat ze niet in de kit staan. Zodra de groep de leden bereikt, laat de nieuwe eigenaar het hun zelf weten. Elk lid controleert eerst: het voegt een wegwerpsleutel aan de repository toe die alleen een server bij de map van de repository kan verwijderen, en volgt de nieuwe eigenaar zodra die sleutel weg is. Antwoordt de nieuwe eigenaar op een ander adres, dan doet elk lid deze controle daar met zijn eigen sleutel en login. Het gaat na of het daar dezelfde repository vindt en zet pas dan zijn bestemmingen om naar het nieuwe adres. Tot dan kopieert het verder naar het oude adres, en zijn kaart zegt waarom. Sleutels die van zulke controles over zijn, verwijdert de eigenaar bij zijn volgende opruimbeurt.

Draait de oude eigenaar nog, dan zegt zijn kaart dat de andere server nu de eigenaar van de repository is. **Terughalen** maakt hem weer eigenaar, voor als een kit per ongeluk op een tweede server is overgenomen.

### Grenzen {#shared-limits}

- **Elke sleutel leest alles.** Alle sleutels van een restic-repository openen dezelfde hoofdsleutel, dus elk lid kan de back-ups van de anderen lezen. Container- en VM-definities blijven verzegeld met de eigen sleutel van elke server, dus de containers en VM's van een andere server komen alleen als bestanden terug. Een server die ooit toegang had, kan de hoofdsleutel gelezen hebben, dus verwijderen sluit hem niet voorgoed buiten. Alleen een nieuwe repository doet dat.
- **De eigenaar kan verwijderen.** Geen lid kan daar iets verwijderen, maar de eigenaar komt rechtstreeks bij de map en vergeet en schoont de back-ups van elk lid op. Een fout of een overname bij de eigenaar raakt ze allemaal.
- **Schade raakt elke server.** Een kapotte pack ontbreekt voor elke server waarvan de back-ups hem gebruiken, dus een gedeelde repository vervangt geen tweede kopie.
- **Alleen op een ontvangserver.** S3, SFTP en andere plaatsen kunnen een lid niet beperken tot toevoegen, dus BombVault biedt een gedeelde repository alleen aan op een ontvangserver.
- **Alleen als kopieerdoel.** De eigen map van een onderdeel kan niet in een gedeelde repository liggen, omdat elke opschoning de back-ups van alle servers zou ophouden.
- **Oudere geschiedenis blijft waar ze is.** Alleen back-ups onder de eigen host van elke server gaan erin, en er wordt niets uit andere repository's overgezet.

## Voorbeeld: twee Unraid-machines koppelen {#worked-example-two-unraid-boxes-end-to-end}

Dit is één complete opstelling van de bovenstaande onderdelen, omdat ze makkelijker te combineren zijn als je ze eenmaal gecombineerd hebt gezien.

Twee machines: **TOWER** draait de containers en pusht back-ups. **VAULT** ontvangt ze en dwingt append-only af. Vervang de namen, adressen en sharepaden door de jouwe.

**1. Koppel de twee machines.** Open op TOWER **Instellingen, Koppeling**, druk op **Frase genereren** en voer de twaalf woorden op VAULT in onder **Frase invoeren**. Stel eerst op beide een inlogwachtwoord in.

**2. Vraag op TOWER aan VAULT om ontvanger te zijn.** Schakel op de pagina Instanties **Ontvanger** in op de kaart van VAULT, kies de onderdelen en **de ontvangserver van VAULT** als plek, en verstuur het verzoek.

**3. Sta het op VAULT toe.** De pagina Instanties van VAULT toont het verzoek op de kaart van TOWER. Druk op **Toestaan**. De eerste keer vraagt VAULT om een map voor zijn ontvangserver, zoals `user/appdata/rest-server`, start de rest-server in append-only-modus en voert de sabotagetest ertegen uit.

**4. Terug op TOWER.** Vanaf dan kopieert TOWER na elke back-up naar VAULT, en de nieuwe bestemming verschijnt op de pagina Bestemmingen van TOWER met zijn bescherming tegen verwijderen.

Wat de sabotagetest antwoordt:

| Resultaat | Wat er gebeurde |
| --- | --- |
| **Beschermd** | VAULT weigerde de verwijdering. Dit is de enige geslaagde toestand. |
| **Niet beschermd** | VAULT accepteerde een verwijdering. `--append-only` ontbreekt of is verwijderd. |
| **Niet eenduidig** | Geen van beide. Meestal is de URL niet die restic zelf gebruikt, of zijn de inloggegevens veranderd. Er wordt niets vastgelegd en er gaat geen waarschuwing uit. |

**5. Kijk wat er aankomt, als je wilt.** Schakel **Ontvanger** in onder **Instellingen, Instanties** op VAULT, open de **Details** van TOWER en voeg het ontvangen repository toe: klik op een van de plekken die het venster toont. VAULT krijgt het restic-wachtwoord van TOWER via de groep, en omdat de plek op de eigen ontvangserver van VAULT staat, leest het de map rechtstreeks. Niemand typt een sleutel of een login.

**6. Maak het wederzijds.** Herhaal de stappen in de andere richting: VAULT vraagt TOWER zijn ontvanger te zijn. Elke machine dwingt dan append-only af voor de ander, en geen van beide kan de back-ups van de ander verwijderen.

Een rest-server die je zelf met de hand draait, werkt ook: voeg hem toe met **Bestemming toevoegen** als rest-server, met de gebruiker en het wachtwoord uit zijn `.htpasswd`, en schakel bescherming tegen verwijderen in. Houd `--append-only` in zijn opties. Zonder dat is de andere kant weer een gewone share.

!!! warning "Een lokaal pad is een pad binnen de container"
    Waar BombVault om een map op deze server vraagt, vul je die relatief aan de host-mount in, zoals `user/appdata/rest-server/bombvault-containers`, niet `/mnt/user/appdata/...`. BombVault draait in een container, waar de `/mnt` van de host elders is gemount. Een absoluut hostpad bestaat daarbinnen niet. Plak je er een, dan zegt BombVault welk relatief pad je in plaats daarvan moet gebruiken.

## Noodassistent {#emergency}

De **Noodassistent** brengt een server terug die weg is of opnieuw is ingesteld. Hij begint op de pagina **Herstel** onder **Noodgeval**, en bij **Herstellen vanaf een back-up** op de welkomstpagina van een nieuwe installatie. Hij loopt in vier stappen: **Toegang**, **Instellingen**, **Selectie** en **Herstel**.

1. **Toegang.** Laat BombVault de sleutel van de oude back-ups zien, op een van drie manieren:
    - **Herstelkit uploaden**: het bestand dat je bij het instellen downloadde. Het bevat de sleutel en elke bestemming, en BombVault vult de rest daaruit in. Een kit die met age is verzegeld, vraagt om de identiteit waarvoor hij is verzegeld. Ze wordt één keer gebruikt en niet opgeslagen.
    - **Via de groep**: een instantie van je groep bewaart de sleutel als bewaarkopie. Je hebt de twaalf woorden van de groep nodig, je vrijgavecode en iemand bij die instantie. Zie [Sleutelbewaarder](#key-keeper).
    - **Sleutel intypen**: de `APP_KEY` van de oude server, en waar zijn back-ups staan.

    BombVault probeert de sleutel dan op elke bestemming en zegt voor elke of ze leesbaar is, niet gevonden, met een andere sleutel vergrendeld, niet bereikbaar of nog haar inloggegevens mist. Er is nog niets opgeslagen, en de sleutel blijft beperkte tijd in het geheugen. **Sleutel overnemen** laat deze server vanaf nu zijn back-ups met die sleutel schrijven. **Overname ongedaan maken** gaat terug naar zijn eigen sleutel tot de eerste back-up heeft gedraaid. Een server die al eigen back-ups heeft, weigert de overname en wijst in plaats daarvan naar [Uit een oude back-upmap](#restore-from-another-bombvault-repo). **Inloggegevens opslaan** bewaart de inloggegevens die de kit voor externe bestemmingen bevat, en vraagt eerst of het inloggegevens mag vervangen die deze server al heeft.
2. **Instellingen.** **Instellingen terughalen** herstelt de eigen instellingen van BombVault uit de nieuwste zelf-back-up: bestemmingen, inloggegevens, planningen en regels. BombVault herstart om ze toe te passen, via de Docker-socket, zodat de live instellingendatabase nooit onder een open handle wordt overschreven, en de assistent gaat daarna vanzelf verder. De login van deze server blijft: je blijft je aanmelden met het wachtwoord dat je nu gebruikt. Geheimen die de oude server had opgeslagen en die niet kunnen worden overgenomen, worden getoond, zodat je ze opnieuw kunt invoeren. Het herstel beëindigt elke sessie en verwijdert de passkeys, dus stel ze daarna opnieuw in.
3. **Selectie.** BombVault zoekt door je back-ups en toont de containers, VM's, mappensets en ZFS-datasets die het vindt. De nieuwste snapshot is overal vooraf gekozen. Wat deze server nog heeft, begint uitgeschakeld, omdat een herstel het zou vervangen. Mappensets weten na een herinstallatie niet meer waar ze vandaan kwamen, dus ze gaan naar een map die je kiest.
4. **Herstel.** Alles wat is gekozen komt in één keer terug, gestopt gelaten als je elke container en VM zelf wilt starten, wat het veiligst is na een totaal verlies. Download daarna een nieuwe herstelkit: de oude hoort bij de vorige installatie.

Een gedeelde repository uit de kit staat tussen de plaatsen als **Gedeelde repository bij** en de naam van de server die er eigenaar van is, en de assistent toont daar alleen de eigen back-ups van deze server. Zie [Na een noodgeval](#shared-emergency).

![De Noodassistent bij zijn eerste stap, met de drie ingangen.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![De Noodassistent bij zijn eerste stap, met de drie ingangen.](assets/screenshots/offsite/emergency-light.png#only-light)

### Voorbeeld: een nieuwe server vanuit de herstelkit {#example-emergency}

De server TOWER is met zijn schijven gestorven. Hij had 23 containers en 2 VM's, en kopieerde ze en zijn zelf-back-up naar Backblaze B2. Je hebt `bombvault-recovery-kit.md` uit je wachtwoordmanager en een nieuwe machine met Unraid erop. Zijn oude `APP_KEY` is weg, en die heb je niet nodig.

1. **Installeer BombVault** op de nieuwe machine en geef hem een nieuwe `APP_KEY`, zoals bij elke nieuwe installatie. Stel de VM-verbinding in onder **Instellingen, Integraties, Host-SSH** als je de VM's terug wilt, want een VM-herstel heeft die nodig.
2. **Start de assistent.** Kies op **Welkom bij BombVault** **Herstellen vanaf een back-up**. Kies op de pagina Herstel **Noodassistent starten** onder **Noodgeval**.
3. **Toegang.** Kies **Herstelkit uploaden** en sleep het bestand erheen. BombVault probeert zijn sleutel op elke bestemming en zegt bijvoorbeeld **Herstelkit gelezen. De sleutel ervan opent 0 van 2 bestemmingen.** De lokale map zegt **niet gevonden**, omdat de schijf stierf, en Backblaze B2 zegt **inloggegevens ontbreken**.
4. **Inloggegevens opslaan.** De kit bevat de inloggegevens van elke bestemming. Druk op **Inloggegevens opslaan**, en de bestemmingen worden opnieuw geprobeerd, zodat Backblaze B2 nu **leesbaar** zegt. Een kit die vóór 10.5 is gemaakt, bevat alleen de standaardinloggegevens. Zegt een bestemming nog **inloggegevens ontbreken**, voer haar inloggegevens dan in onder **Ontbrekende inloggegevens invoeren** en druk op **Bestemmingen opnieuw proberen**.
5. **Sleutel overnemen.** Deze server schrijft zijn back-ups nu met de oude sleutel, zodat de nieuwe back-ups bij de oude passen. Hij houdt zijn eigen `APP_KEY` in de template. **Overname ongedaan maken** werkt tot de eerste back-up heeft gedraaid.
6. **Instellingen.** De stap opent op de plek waar de sleutel de zelf-back-up vond, hier B2, met het adres al ingevuld. Kies **Instellingen terughalen**, dat de nieuwste zelf-back-up van daar leest. BombVault herstart om haar toe te passen, en de assistent gaat vanzelf verder. Je bestemmingen, planningen en regels zijn terug, en je blijft je aanmelden met het wachtwoord van deze server. Voer de geheimen die de assistent toont opnieuw in, zoals de meldingsinstellingen.
7. **Selectie.** **Waar staan de back-ups?** opent op de off-site-kopieën als er lokaal niets over is. BombVault leest elke bestemming, zodat een dode de andere niet verbergt, toont de 23 containers en 2 VM's en herstelt elk van de bestemming met zijn nieuwste snapshot.
8. **Herstel.** Schakel **Daarna gestopt laten** in, wat het veiligst is na een totaal verlies, en start. **Wat er gebeurt** zegt wat er terugkomt. De resultaatregel leest bijvoorbeeld **25 hersteld, 0 mislukt. Start ze indien nodig onder Back-ups.**
9. **Afronden.** Start eerst je databases, dan de apps die ze gebruiken. Open **Instellingen, Herstel** en download een nieuwe herstelkit, omdat de oude bij de vorige installatie hoort. Controleer **Aandacht nodig** op het overzicht, en voer **Nu alles back-uppen** uit zodra alles werkt.

Ontbreekt ook de zelf-back-up, dan opent de stap op **Overslaan: ik heb geen zelf-back-up** en zegt dat. De bestemmingen komen dan alleen uit de kit, je stelt planningen en regels opnieuw in, en off-site-kopieën wachten tot je hun back-ups overneemt als back-ups van deze server, zie [Na een herinstallatie](#after-a-rebuild). **Selectie** toont nog steeds wat de off-site-kopieën bevatten. Mappensets gaan naar een map die je kiest. Containers en VM's komen terug als gewone bestanden in een map onder **Alleen de bestanden kunnen terugkomen**, omdat hun definities naast het lokale repository en in de zelf-back-up staan, en een off-site-kopie bevat geen van beide.

!!! tip "Geplande verhuizing tegenover een ramp"
    De Noodassistent herstelt de eigen instellingen van BombVault uit een back-up. Voor een *geplande* verhuizing naar een nieuwe machine kun je je configuratie in plaats daarvan rechtstreeks meenemen met de kaart **Instellingen exporteren / importeren** (een draagbaar JSON-bestand). Zie [Configuratie](configuration.md#portable-settings-export-and-import).

### Uit een oude back-upmap {#restore-from-another-bombvault-repo}

**Uit een oude back-upmap** onder **Andere bronnen** op de pagina Herstel opent het repository van een *andere* BombVault-instantie (een share onder `/mnt` gemount, of een extern adres) met de herstelkit van die instantie of haar ingetypte `APP_KEY`, in een eenmalige, alleen-lezen sessie. Blader door de containers, VM's, mappensets en ZFS-datasets die daar zijn opgeslagen, kies een snapshot en herstel die, en het herstelde object wordt een gewone lokale container, VM of mappenset. Er wordt nooit iets naar het andere repository geschreven, en je eigen back-upinstellingen blijven onaangeroerd (de sessie leeft in het geheugen en verloopt vanzelf). Een container van server A naar server B verhuizen betekent niet dat je je instellingen moet omleiden en daarna terugdraaien. Dit venster is eenmalig: het opent een sessie, herstelt wat je kiest en vergeet de andere instantie. Wil je liever een vaste regeling, waarbij deze machine volgens een planning de snapshots van een andere instantie in zijn eigen repository ophaalt, dan is dat de rol **Ophaler** op de pagina Instanties.

### Wat deze server heeft opgehaald {#restore-what-was-fetched}

Elke bron waarvan deze server heeft opgehaald, heeft een eigen regel **Opgehaald van** onder **Andere bronnen** op de pagina Herstel. Een opgehaalde kopie is vergrendeld met de eigen sleutel van deze server, niet met die van de bron, dus er wordt niets gevraagd: de sleutel van de bron opent haar niet. Mappensets en ZFS-datasets gaan naar een map die je kiest. Containers en VM's komen terug als gewone bestanden in een map, omdat een ophaalactie de snapshots meeneemt en niet de definities. Wil je ze opnieuw aanmaken, gebruik dan de herstelkit en de zelf-back-up van de bron in de Noodassistent.

Een container waarvan het netwerk op deze server niet bestaat, zoals een Unraid-netwerk `br0` op een gewone Docker-host, toont onder zijn rij een netwerkkiezer. BombVault maakt hem aan op het netwerk dat je kiest, samen met zijn andere netwerken. Het vaste IP en MAC-adres hoorden bij het oude netwerk en vallen weg, zodat het nieuwe netwerk ze toewijst.

## Sleutelbewaarder {#key-keeper}

Een sleutelbewaarder is een instantie van je groep die een verzegelde kopie van de herstelkit van deze server bewaart. Hij kan de kopie zelf niet openen. Gaat deze server ooit verloren, dan vraagt een nieuwe installatie die tot je groep toetreedt de bewaarder om de kopie en opent haar met je vrijgavecode.

![Het venster met de vrijgavecode: twintig tekens in vier groepen, één keer getoond.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![Het venster met de vrijgavecode: twintig tekens in vier groepen, één keer getoond.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. Schakel op de pagina Instanties **Sleutelbewaarder** in op de kaart van de instantie die hem moet bewaren, en kies **Bewaarkopie maken**. Een bewaarkopie maken vereist een inlogwachtwoord.
2. Deze server toont de **Vrijgavecode** één keer: twintig letters en cijfers in vier groepen van vijf. Schrijf haar op en bevestig met **Ik heb de code opgeschreven**. Deze server slaat de code niet op, alleen wat hij nodig heeft om er een bewaarkopie voor te verzegelen.
3. De bewaarder houdt nu de bewaarkopie, en deze server vernieuwt haar als de kit verandert. **Bewaarkopie van de sleutel** op de kaart van deze instantie toont **Bewaarkopie van deze server** met elke bewaarder en zijn status, en **Bewaarkopieën die deze server bewaart** voor de anderen. **Nieuwe code maken** vervangt de code. Elke bewaarder schakelt over op de nieuwe zodra hij de nieuwe bewaarkopie krijgt.

Op die dag heb je drie dingen nodig: de twaalf woorden van je groep, de vrijgavecode en toegang tot de interface van de bewaarder, waar iemand op **Toestaan** moet drukken. Kies in de Noodassistent **Via de groep**, kies de bewaarkopie, vraag haar aan en typ de vrijgavecode. De code wordt op de nieuwe server gebruikt en niet naar de bewaarder gestuurd. Bij de bewaarder staat het verzoek onder **Bewaarkopie van de sleutel** met wie het vraagt, of die instantie daar ooit eerder is gezien en hoe vaak ze vroeg. **Toestaan** geeft de verzegelde kopie uit, en toestaan vereist een inlogwachtwoord. De nieuwe installatie heeft na het toestaan 30 minuten om haar op te halen, een verzoek dat niemand beantwoordt verloopt na een dag, en een instantie die te vaak vraagt, moet een uur wachten.

Zonder de code kan niemand de kopie openen, de bewaarder inbegrepen. Is de code kwijt, maak dan een nieuwe zolang deze server nog draait. Is de server al weg, dan werken de herstelkit en de ingetypte APP_KEY nog. De bewaarder kan een bewaarkopie op elk moment verwijderen, en de eigenaar moet dan opnieuw vragen.

## Herstelkit {#recovery-kit}

Dit is het onderdeel dat noodherstel mogelijk maakt, ook als er geen draaiende BombVault is.

**Herstelkit downloaden** in de kaart **Voor noodgevallen** onder **Instellingen, Herstel** geeft je de **sleutel**, het **afgeleide restic-wachtwoord**, elke **bestemming** met het repository van elk onderdeel, de inloggegevens die een nieuwe installatie nodig heeft om ze te bereiken, en de **exacte restic-opdrachten**, zodat je rechtstreeks kunt herstellen met de restic-CLI op een willekeurige machine. Het vereist een inlogwachtwoord. De kit toont de sleutel ook met versleuteling uitgeschakeld, omdat de container- en VM-definities er altijd mee zijn verzegeld. Na een overname toont ze de overgenomen back-upsleutel en de APP_KEY van deze server, die een nieuwe installatie alleen nodig heeft om de instellingen terug te halen. Een kaart **Bewaar je herstelkit** op het overzicht herinnert je eraan tot je haar hebt opgeslagen.

!!! danger "Bewaar de herstelkit buiten de server"
    De kit bevat het geheim waarmee je back-ups worden ontsleuteld. Bewaar haar ergens veilig en gescheiden van de server (een wachtwoordmanager, een afgedrukt exemplaar in een kluis). Raak je zowel BombVault als `APP_KEY` kwijt zonder herstelkit en zonder bewaarkopie van de sleutel, dan zijn je versleutelde back-ups niet te herstellen.

!!! warning "De nieuwste snapshot is niet altijd degene om te herstellen"
    Sinds restic 0.17 toont `restic snapshots` de grootte van elke snapshot. Na dataverlies kan de nieuwste snapshot de lege zijn, dus herstel geen snapshot die veel kleiner is dan die ervoor. Na ransomware kan het de versleutelde zijn, op de gebruikelijke grootte. Draait BombVault nog, kijk dan eerst op zijn pagina **Anomalieën**: die noemt de laatste goede back-up, en het herstelvenster selecteert die vooraf. Een herstel heeft geen anomaliedata van BombVault nodig, en de bewaarpauze bewaart alleen méér snapshots.

### De host van de back-ups van een server {#backup-host}

Elke server schrijft zijn back-ups onder een eigen restic-host: `bv-` en twaalf tekens die uit de sleutel van de back-ups worden berekend, zoals `bv-3f9a0c41d2e7`. Back-ups van voordat een server er een had, dragen de host `bombvault`. De kit noemt de host van deze server, en haar opdrachten geven beide mee met `--host`, zodat `latest` nooit een back-up kiest die een andere server naar hetzelfde repository heeft geschreven. Een server die de back-ups van een andere overneemt, gaat verder onder de host van die server, omdat de host uit de sleutel komt. Voor elke [gedeelde repository](#shared-repository) waar de server naartoe kopieert of waarvan hij eigenaar is, noemt het hoofdstuk **Shared repositories** van de kit de andere servers daar met hun hosts.

### De kit verzegelen {#sealing-the-kit}

Heb je **Exports versleutelen met age** ingeschakeld onder **Instellingen, Systeem**, dan wordt de kit er ook mee verzegeld en downloadt hij als `bombvault-recovery-kit.md.age`. Hij is ASCII-armored in plaats van binair, dus hij is nog steeds platte tekst: plakken in een wachtwoordmanager of afdrukken werkt precies zoals eerder, de inhoud is onleesbaar zonder je sleutel.

!!! warning "Bewaar de age-sleutel niet in de kit"
    Je hebt je **privé**sleutel van age nodig om een verzegelde kit te openen. Bewaar hem ergens dat niet van de kit zelf afhangt, anders heb je twee dingen om te herstellen in plaats van één. Verzegelen loont als de kit ergens ligt dat je niet volledig beheert (een gedeelde wachtwoordmanager, cloudnotities, een afdruk op kantoor). Een kit in je eigen kluis is al beschermd door de kluis.

    Met versleuteling aan en geen bruikbare ontvanger ingesteld wordt de download botweg geweigerd. BombVault valt nooit terug op het ongecodeerd uitgeven van de hoofdsleutel.

### Als je de kit niet bij de hand hebt {#if-you-do-not-have-the-kit-to-hand}

Het wachtwoord wordt nergens opgeslagen, het wordt **berekend** uit de sleutel van de back-ups, dus je kunt het zelf reproduceren met niets dan de sleutel en een shell:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

Dat is HMAC-SHA256 over de vaste string `bombvault:restic-repo`, gesleuteld met de ruwe bytes van de hex-`APP_KEY`, afgedrukt als 64 hex-tekens in kleine letters. Dezelfde waarde staat in de kit, vermeld als het afgeleide restic-wachtwoord. Dit is voor de dag dat de kit ergens ligt waar jij niet bent. Gebruik op een server die de back-ups van een andere heeft overgenomen de overgenomen back-upsleutel, de oude APP_KEY van die server.

De [host](#backup-host) komt uit dezelfde sleutel en de string `bombvault:host`: `bv-` en de eerste twaalf van die tekens.

```sh
echo "bv-$(printf 'bombvault:host' | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r | cut -c1-12)"
```

!!! warning "Gebruik voor een ontvangen repository de sleutel van de VERZENDENDE instantie"
    Een repository dat hier via een off-site-kopie aankwam, is gemaakt door de machine die het verstuurde, met **haar** sleutel. Afleiden uit de sleutel van de ontvangende machine geeft een wachtwoord dat restic afwijst, wat er precies uitziet als een corrupt repository en dat niet is. Dit is de gebruikelijke reden dat `restic check` op een ontvangen repository steeds weer om een wachtwoord vraagt.

Omdat de hersteldefinities **in** elk repository staan (`<repo>/def`, `<repo>/vm-def`), is een gekopieerde repositorymap volledig op zichzelf staand, dus de kit plus het repository is alles wat een herstel op kale hardware nodig heeft. Een off-site-kopie en een opgehaalde kopie bevatten de snapshots zonder de definities: ze geven bestanden terug, en een container of VM wordt opnieuw aangemaakt uit de zelf-back-up.

## Een databasedump terughalen {#database-dumps}

Een databasedump is een eigen herstelpunt in het containerrepository, getagd `dbdump:<container>`, met het ene bestand `/dbdump/<container>.sql`. BombVault toont, downloadt en importeert ze op het tabblad **Back-ups** van de container. Hieronder staan dezelfde stappen met alleen restic, voor de dag dat BombVault er niet is.

```sh
restic -r <repo> snapshots --host <host> --host bombvault --tag dbdump:<container>
restic -r <repo> dump --host <host> --host bombvault --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

`<host>` is de [host](#backup-host) van de server uit de herstelkit.

De tags `dbversion:` en `dbname:` op elke dump zeggen van welke serverversie hij komt en welke databases hij bevat. Een compleet bestand eindigt met `-- PostgreSQL database cluster dump complete` of `-- Dump completed`.

Importeer hem in een container van dezelfde of een nieuwere versie (PostgreSQL), of dezelfde hoofdversie (MySQL en MariaDB), eenmalig gestart met een lege datamap zodat hij initialiseert. De host heeft geen databaseclient nodig, de container heeft er een:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Voor één database uit een volledige dump nemen MySQL en MariaDB `--one-database <name>` bij de clientopdracht. Een PostgreSQL-dump heeft één sectie per database, elk beginnend met een regel `\connect <name>`: kopieer die sectie naar een eigen bestand en importeer het met `-d <name>` nadat je de database hebt aangemaakt.

!!! warning "Een rootdump bevat de gebruikers van de server"
    Een volledige MySQL- of MariaDB-dump die als root is gemaakt, bevat de systeemdatabase `mysql`, dus importeren vervangt de accounts van de nieuwe server, het wachtwoord van root inbegrepen, door die uit de dump. Op PostgreSQL is `role ... already exists` voor de gebruiker die de container aanmaakte te verwachten en onschuldig.
