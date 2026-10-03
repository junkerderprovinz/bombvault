# Off-site en herstel

!!! note "Off-site kopieën wachten na een rebuild"
    Als stap 4 items herbouwt zonder de oude instellingen, pauzeert de off-site replicatie van die domeinen tot de standaardplaatsing is bevestigd. Zie [Plaatsing per item](#placement).

Lokale back-ups beschermen je tegen een verloren container of een slechte update. Off-site replicatie en een geteste herstelkit beschermen je tegen de hele machine, ransomware of een brand. Deze pagina behandelt off-site repliceren, die kopie manipulatiebestendig maken, bewijzen dat je kunt herstellen, en herstellen wanneer BombVault zelf weg is.

## Off-site replicatie

Houd de snelle lokale back-up en voeg een of meer off-site replica's toe. Stel een repo per domein in op de pagina **Instellingen, Off-site**. BombVault repliceert nieuwe snapshots daarheen met `restic copy` op best-effort-basis, zodat een off-site hapering de lokale back-up nooit laat mislukken. In deze vorm blijft de lokale repo primair en is de off-site repo een replica, maar de primaire repo van een domein hoeft helemaal niet lokaal te zijn; zie [Externe primaire repositories](#remote-primary-repositories) hieronder om rechtstreeks naar S3, een rest-server enzovoort te back-uppen in plaats van ernaar te repliceren.

- **Meerdere off-site doelen per domein.** Elk domein (containers, VM's, flash, config, bestandssets en ZFS-datasets) kan tegelijk naar meerdere off-site bestemmingen repliceren, niet slechts één, zodat je bijvoorbeeld een rest-server op de machine van een vriend en een S3-bucket parallel kunt houden. Voeg extra doelen toe op Instellingen, Off-site, elk met zijn eigen repository, S3-opslagklasse, append-only-vlag, retentie en groeibudget. Een bestaande enkele off-site setup wordt onaangeroerd overgenomen als het eerste doel, en elk doel van een domein repliceert op de off-site planning van dat domein.
- **Off-site planning per domein** (bewerkt naast elke andere planning op Instellingen, Schema's): laat het leeg om na elke lokale back-up te repliceren, of stel een cadans in (bijvoorbeeld `weekly Sun 03:00`) om minder vaak off-site te sturen dan je lokaal back-upt. Een knop **Nu repliceren** dekt runs op aanvraag.
- **Off-site retentie** staat op Instellingen, Bewaarbeleid zodat je off-site kopieën langer als archief kunt bewaren. Laat het beleid geheel op nul om off-site snapshots nooit automatisch te trimmen.
- **Bandbreedtelimieten** (Instellingen, Off-site) begrenzen de restic-upload/downloadsnelheid zodat replicatie je WAN niet verzadigt.
- Een **replicatie-indicator** toont welk domein aan het repliceren is terwijl het draait (op zijn pagina en het Dashboard). Het is een actieve indicator, geen percentagebalk, omdat `restic copy` geen machine-leesbare voortgang blootgeeft.

!!! note "Herstel vanaf elke plek"
    Elke container, VM, bestandsset, de flash en de app-configuratie tonen hun back-ups als één tijdlijn over alle plekken waar een back-up ligt. Een back-up die naar B2 is gekopieerd, verschijnt één keer, gemarkeerd met elke plek die hem bevat. Een herstel neemt de eerste plek die het kan bereiken, te beginnen bij de repository waarnaar het item wordt geschreven, en je kunt per rij een andere plek kiezen. Off-site plekken worden alleen gelezen als je ze opent. Verwijderen op één plek controleert eerst de andere en zegt of het de laatste kopie was.

## Plaatsing per item {#placement}

Elke kaart van een container, VM en bestandsset heeft een rij **Plaatsing** met drie segmenten:

- **Lokaal** schrijft het item naar de repository die onder **Opgeslagen op** staat en kopieert het nergens naartoe. Gebruik dit voor data die al een tweede kopie heeft, bijvoorbeeld een share die op een NAS leeft.
- **Lokaal + off-site** schrijft het ook daar en kopieert het naar de doelen die onder **Kopieer naar** zijn aangevinkt, één chip per off-site doel van het domein. Vink een chip uit en dat doel krijgt niets nieuws meer van dit item.
- **Alleen off-site** schrijft het item rechtstreeks naar de plek onder **Verstuur naar**: een directe repository naast een off-site doel, of een remote repository die je hebt ingesteld onder Instellingen, Opslag, Repository's.

De locatie ligt vast vanaf de eerste back-up van het item, omdat BombVault back-ups nooit tussen repositories verplaatst. De kopieën kunnen op elk moment veranderen. Een doel dat een item niet meer krijgt, houdt de kopieën die het heeft en trimt ze naar zijn eigen retentie bij de volgende off-site run van het domein; **Verwijderen bij B2** op de kaart verwijdert ze meteen. Als sommige van die kopieën nergens anders bestaan, toont de bevestiging ze op datum en vraagt om de naam van het item. Bij append-only doelen kan niet worden verwijderd.

Onder de rij zegt de kaart waar het item naartoe gaat en wat er werkelijk is: hoeveel locaties het bevatten, wanneer elk doel voor het laatst is gezien, en of aan 3-2-1 wordt voldaan. Een locatie is de server met de oorspronkelijke data, elk off-site doel en elke repository die als **Buiten het pand** is gemarkeerd. BombVault controleert kopieën en locaties; het controleert niet het «twee media» deel van 3-2-1.

### Standaardplaatsing

Instellingen, Opslag, **Standaardplaatsing** heeft één rij per domein met dezelfde drie segmenten. De kopieën gelden meteen voor elk item zonder eigen keuze, en voor de projectmappen van Compose-stacks. De locatie geldt voor een nieuw item bij zijn eerste back-up; wijzigen verplaatst geen back-ups. Voor het opslaan noemt de rij elk doel dat items wint of verliest en hoeveel snapshots dat betekent. **Toepassen op items zonder back-ups** zet elk item dat nog geen back-up heeft terug op de standaard.

Een nieuw off-site doel ontvangt elk item dat niet op Lokaal staat. Het venster dat het toevoegt, zegt hoeveel items en, waar bekend, hoeveel geschiedenis dat is, en biedt aan om de items over te slaan die al bij andere doelen zijn uitgesloten.

### Directe repository's

Het kiezen van de directe repository van een doel onder Alleen off-site opent een venster met een voorgestelde locatie naast het doel, bijvoorbeeld `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, en een verbindingstest die niets aanmaakt. **Aanmaken en gebruiken** maakt de repository aan en wijst het item ernaartoe. Een directe repository neemt de sleutel, opslagklasse, limieten, append-only-instelling en retentie van het doel over, en verandert daarmee mee; de kaart Repository's toont hem alleen-lezen. Als een nieuwe sleutel van het doel hem niet kan openen, houdt de directe repository de sleutel die hij heeft, en het opslaan meldt dat. Zijn snapshots dragen de tag `bv:direct`, en elke andere retentieronde spaart ze, zodat een directe repository die zijn link met zijn doel kwijt is, nooit veroudert volgens de lokale regels. B2 wordt bereikt via zijn S3-eindpunt, met de sleutel-ID en de toepassingssleutel als S3-inloggegevens; een sleutel die beperkt is tot de eigen map van het doel, kan niet bij de map ernaast, beperk de sleutel dus in plaats daarvan tot de map boven het doel.

### Buiten het pand

Een benoemde repository kan op de kaart Repository's worden gemarkeerd als **Buiten het pand**. Remote repository's beginnen gemarkeerd; zet het uit voor een rest-server in hetzelfde gebouw. De markering telt alleen mee voor locaties en 3-2-1 op de kaarten. Er verandert geen kopie door.

### Na een rebuild

Kopieerkeuzes leven in BombVaults eigen instellingen. Na een rebuild via Back-ups ontdekken zonder hersteld `/config` zijn ze weg, en alles kopiëren zou de items die je had uitgesloten opnieuw naar B2 sturen. De off-site replicatie van elk herbouwd domein pauzeert daarom. Het Dashboard toont dit in oranje, en Standaardplaatsing biedt **Standaard bevestigen** met een voorbeeld van wat de volgende run kopieert en de namen in de back-ups zonder item, die je daar kunt uitsluiten. Alleen de bevestiging beëindigt de pauze; het importeren van een instellingenbestand brengt regels en standaarden terug maar beëindigt de pauze niet.

## Externe primaire repositories {#remote-primary-repositories}

Het back-uppad van een domein (Instellingen, Opslag) is niet beperkt tot een lokale map: richt het rechtstreeks op een restic-remote (`s3:...`, `rest:http://host:8000/repo`, `sftp:gebruiker@host:/repo`, `rclone:remote:bucket/pad`) en BombVault back-upt daar direct naartoe, zonder aparte lokale kopie en zonder replicatiestap. Dat is een werkelijk andere vorm dan de off-sitereplicatie hierboven: daar is het lokale repository primair en is het off-site-repository er een archief van naar beste vermogen; hier **is** het externe repository het primaire, en is het de enige kopie zolang je voor dat domein niet ook off-sitereplicatie (of een tweede remote) instelt.

Elk van de zes padvelden (Containers, VM's, Flash, Zelf-back-up, Mappen, ZFS-datasets) heeft er direct naast een schakelaar **Lokaal / Extern**:

- **Lokaal** toont de vertrouwde mappenbrowser.
- **Extern** vervangt hem door een eenvoudig URL-veld, plus een knop die hetzelfde venster voor verbindingstest en inloggegevens opent dat off-sitebestemmingen gebruiken, maar dan ingesteld voor dit primaire repository. Daar krijg je:
    - **Een verbindingstest** tegen het echte pad, voordat je erop vertrouwt.
    - **Bandbreedtelimieten** (upload en download), zodat een geplande back-up naar een extern primair repository je WAN-lijn niet dichtslibt: dezelfde restic-opties `--limit-upload` en `--limit-download` die off-sitereplicatie gebruikt, nu toegepast op de back-up zelf.
    - **Append-only-bescherming (onveranderlijkheid)**, gecontroleerd met dezelfde actieve manipulatietest (een echte DELETE-sonde naar de overkant) die off-sitebestemmingen krijgen. Staat die aan, dan weigert BombVault het repository zelf op te schonen: omdat er geen aparte lokale kopie achter zit, mogen de inloggegevens op deze machine niet in staat zijn de enige kopie van de back-up te wissen.
    - **Een alarm voor het groeibudget**, afgeleid van dezelfde trend in repositorygrootte die de Opslag-kaart al bijhoudt.

Niets hiervan is verplicht: een met de hand ingetypt extern pad zonder opgeslagen veiligheidsinstellingen back-upt precies zoals altijd (onbeperkte bandbreedte, opschoonbaar, geen budgetalarm). Het veiligheidsvenster is er voor als je dezelfde bescherming wilt die een off-sitekopie krijgt, zonder daarvoor apart een off-sitebestemming te moeten aanmaken.

!!! note "Cloud- en REST-inloggegevens worden gedeeld"
    Een extern primair repository meldt zich aan met dezelfde S3-/REST-inloggegevens die onder Instellingen, Cloudtoegang, Gedeelde cloud-inloggegevens staan. Een aparte opslag voor inloggegevens van primaire repositories bestaat niet.

### SMB en WebDAV zonder host-mount {#smb-webdav}

Instellingen, Cloudtoegang, rclone heeft een formulier voor een Windows- of Samba-share en voor een WebDAV-server (Nextcloud, ownCloud, SharePoint of een andere). Vul een korte naam in, de host en share (SMB) of de URL en het servertype (WebDAV), de gebruiker en het wachtwoord, en BombVault schrijft de rclone-sectie voor je. rclone versluiert het wachtwoord zelf voordat het wordt opgeslagen; een bestemming toevoegen met een naam die al bestaat, vervangt die sectie in plaats van een tweede toe te voegen.

Het formulier antwoordt met de kant-en-klare locatie, bijvoorbeeld `rclone:nas:backups`. Zet die in een back-uppad of een off-sitebestemming en voeg een submap toe als je die wilt (`rclone:nas:backups/bombvault`). De share is het eerste padsegment, geen deel van de naam.

Dit is een betere route dan de share op Unraid te mounten: restic raadt af een repository op een gemounte CIFS-share te houden, en hier wordt niets gemount. NFS staat niet in het formulier, omdat restic noch rclone een NFS-backend heeft; mount voor NFS de export op de host en wijs er een back-uppad naar.

## Onveranderlijk (append-only) off-site

Vlag een off-site repo als append-only zodat ransomware, of een gecompromitteerde host, je back-ups niet kan verwijderen of herschrijven. De andere kant (een `restic/rest-server` in `--append-only`-modus) **dwingt** het af. BombVault **verifieert** het alleen en toont nooit groen op basis van louter een configuratie-claim.

De wizard **begeleide off-site setup** leidt je van de backendkeuze (rest-server / rclone / S3) via een kant-en-klaar rest-server-deploysnippet, een verbindingstest, de onveranderlijk-schakelaar (die meteen de tamper-test draait) en een retentiestrategie, zodat append-only off-site bereikbaar is zonder configs met de hand te bewerken.

!!! note "Een geslaagde verwijdering onder `/locks/` is verwacht"
    Append-only betekent niet dat er niets meer verwijderd kan worden. restic moet zijn eigen locks zetten en weer vrijgeven, dus `/locks/` blijft met opzet schrijfbaar en verwijderbaar. Snapshots en de data erachter, precies waar ransomware op mikt, kunnen niet worden verwijderd. Als je de andere kant zelf test, is een geslaagde verwijdering onder `/locks/` correct gedrag en geen gat in de bescherming.

!!! warning "Onveranderlijke repo's worden nooit vanaf deze machine geprund"
    Een onveranderlijke off-site prunt bewust nooit oude snapshots. Stel er een **groeibudget-alarm** voor in zodat je gewaarschuwd wordt voordat de repo-grootte uit de hand loopt.

## Tamper-test

BombVault bewijst periodiek de append-only-garantie door daadwerkelijk een verwijdering te proberen tegen de off-site repo, gericht op een niet-bestaand object:

- **Geweigerd** betekent beschermd.
- **Geaccepteerd** betekent niet beschermd.
- Een **onduidelijk** resultaat (server onbereikbaar, auth-fout) draait het opgeslagen oordeel nooit om.

Een echte omslag van beschermd naar onbeschermd vuurt één enkele waarschuwing af.

## DR-oefeningen

BombVault biedt twee niveaus van bewijs dat je back-ups daadwerkelijk herstelbaar zijn, niet alleen aanwezig.

- **Herstelverificatie-oefeningen (lokaal).** BombVault draait periodiek `restic check --read-data-subset` (begrensd, nooit een schijfvullend volledig herstel) en toont een badge *Herstelbaarheid geverifieerd* per domein. De cadans staat op Instellingen, Schema's; de badge op Instellingen, Integriteit.
- **DR-oefeningen (off-site).** BombVault herstelt een echt doel vanuit de off-site repo in een wegwerp-sandbox, verifieert het bestand-voor-bestand en byte-voor-byte, en ruimt daarna op. Dit bewijst dat je vanaf off-site kunt herstellen, niet alleen dat de repo antwoordt.

De **ransomwarebeschermings-scorecard** op het Dashboard vat dit samen tot een groene / oranje / rode houding per domein, met een van datum voorziene checklist (off-site geconfigureerd, append-only geverifieerd, replicatie actueel, hersteloefening geslaagd, versleuteling aan, prune-strategie ingesteld). Elke rode rij linkt diep door naar de fix, en de kaart wordt alleen groen op geverifieerde feiten.

## Koppeling van instanties {#pairing}

Ontvangers, ophaalbronnen, de Instanties-pagina en Mesh-off-site praten allemaal met een andere BombVault. Dat doen ze als leden van één koppelingsgroep, en een instantie treedt tot de groep toe met twaalf woorden.

Open op de eerste instantie **Instellingen → Koppeling** en klik in de koppelkaarten op **Frase genereren**. Er verschijnen twaalf woorden in een venster met een knop **Kopiëren**. Open op elke andere instantie dezelfde plek, klik op **Frase invoeren** en plak of typ ze, of klik in dat venster op **Plakken**. Een woord dat niet op de lijst staat, wordt al tijdens het typen met zijn plaats genoemd, en het laatste woord bevat een checksum, zodat een verkeerd getypt of verwisseld woord wordt opgemerkt voordat er iets gekoppeld wordt. Genereer de frase maar op één instantie: twee instanties die allebei een zin aanmaken, vormen twee aparte groepen. Meldt zich een minuut lang niemand, dan biedt het tabblad twee uitwegen: de woorden opnieuw tonen om ze daar in te voeren, of de woorden van de andere instantie invoeren en in één stap bij zijn groep aansluiten. Koppelen werkt ook zonder inlogwachtwoord, maar stel er een in: zonder wachtwoord kan iedereen die deze webinterface kan openen de woorden lezen en via de groep het restic-wachtwoord van elke instantie erin bemachtigen. De koppelkaart vermeldt dit zolang er geen wachtwoord is ingesteld. Met een wachtwoord vraagt het opnieuw tonen van de zin erom. Met **Groep verlaten** haal je een instantie er weer uit.

Iedereen die de woorden kent kan tot de groep toetreden, behandel ze dus als een wachtwoord.

**Hoe leden elkaar bereiken.** Elke instantie leert zijn eigen adres op het netwerk van je browser zodra je je aanmeldt, te zien in de relaykaart als **Deze instantie op jouw netwerk**; corrigeer het daar als er een reverse proxy of een ongebruikelijke poort voor zit. Op hetzelfde netwerk maken leden dat adres bekend via multicast en praten ze rechtstreeks met elkaar, en waar multicast een containernetwerk zoals Dockers standaard bridge-netwerk niet kan doorkruisen, doorzoekt een instantie in plaats daarvan zijn eigen subnet naar de anderen met een ondertekende oproep die alleen een groepslid kan beantwoorden, zodat koppelen ook dan in enkele seconden klaar is, zonder relay. Wordt er niets gevonden, dan neemt **Niet gevonden?** onder de koppelkaart een adres met de hand aan, voor een ander subnet of een afwijkende poort. Instanties op verschillende netwerken lopen via een relay, gekozen op hetzelfde tabblad:

- **Project-relay** (de standaard): `parleyport.halleluja.design`, dezelfde relay die ook KnightLoader gebruikt. Niets in te stellen.
- **Eigen relay**: de container [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) uit de Unraid Community Apps, of een van je instanties die al van buitenaf bereikbaar is met **Als relay dienen** aangezet. Die instantie antwoordt dan op `/relay/connect` op zijn eigen adres, achter de reverse proxy en het certificaat die hij al heeft, en laat alleen jouw groep binnen. Vul het adres van de relay in op elke instantie die hem moet gebruiken.
- **Geen relay**: leden vinden elkaar automatisch op hetzelfde netwerk, en verder nergens.

**Wat de relay ziet.** Elke oproep tussen leden is verzegeld met AES-256-GCM onder een sleutel die is afgeleid van de twaalf woorden, en die sleutel verlaat je instanties nooit. De relay komt een hash te weten die de verbindingen groepeert, voor welke instantie een bericht bedoeld is, hoe groot het is en wanneer het langskomt. Een rechtstreekse oproep op het lokale netwerk is op dezelfde manier verzegeld en bovendien ondertekend, zodat niets afhangt van het zelfondertekende certificaat dat een instantie serveert.

**Wat er over de groep loopt.** De scorecards op de Instanties-pagina, een verzoek om nu één domein te controleren, Mesh-off-site-aanbiedingen, en wat een ontvanger of ophaalbron nodig heeft: de repository-locaties van de andere instantie en zijn restic-wachtwoord. Back-updata gaat er nooit overheen, die gaat nog steeds rechtstreeks naar de restic-backends. De APP_KEY ook niet: het restic-wachtwoord opent alleen de repository's van die instantie en verder niets, niet zijn opgeslagen geheimen, sessies of herstelcodes.

**Items van vóór de koppeling.** Instanties die met een fleet-token zijn toegevoegd, en ontvangers en ophaalbronnen die met de APP_KEY van de andere instantie zijn ingesteld, blijven na de update bestaan en zijn gemarkeerd met **Opnieuw koppelen**. Ontvangers en ophaalbronnen blijven werken: bij de eerste start vervangt BombVault elke opgeslagen APP_KEY door het daaruit afgeleide restic-wachtwoord. Koppel beide instanties, bewerk daarna de invoer en kies zijn instantie. Zo'n instantie neemt zijn oude kaart over zodra een instantie met dezelfde naam in de groep verschijnt.

De enige plek die nog met de hand een APP_KEY vraagt, is [Herstellen vanuit een andere BombVault-repo](#restore-from-another-bombvault-repo), voor het geval de andere instantie weg is en in geen enkele groep meer kan antwoorden.

## Ontvanger-dashboard (de ontvangende kant)

![De ontvangende kant, alleen-lezen bewaakt, met een integriteitscontrole op deze machine.](assets/screenshots/receiver.png)

*De ontvangende kant, alleen-lezen bewaakt, met een integriteitscontrole op deze machine.*

Alles hierboven is de *zendende* kant. Op de machine die onveranderlijke off-site kopieën van een andere BombVault **ontvangt**, geeft het Ontvanger-dashboard je onafhankelijke, alleen-lezen monitoring van die repositories op de ontvangende hardware, zodat een stille fout aan de andere kant niet onopgemerkt blijft.

Zet de schakelaar **Ontvanger** in Instellingen aan om een tabblad **Ontvanger** te onthullen. Het is standaard uit; schakel het alleen in op een machine die daadwerkelijk onveranderlijke off-site back-ups ontvangt. Registreer daarna een ontvangen repository (alleen-lezen, geopend met het restic-wachtwoord van de zendende instantie, dat het via de [koppelingsgroep](#pairing) krijgt) om te krijgen:

- **Een snapshotinventaris gegroepeerd per bron**, zodat je precies kunt zien welke containers, VM's en bestandssets zijn geland.
- **Laatst ontvangen** per bron, zodat je weet hoe vers elk is.
- **Een onafhankelijke `restic check`** die op de ontvangende hardware draait, zodat de integriteit wordt geverifieerd waar de data daadwerkelijk zit, niet alleen bij de zender.
- **Een dead-man's switch:** een waarschuwing wanneer een bron stopt met verzenden binnen een venster dat je instelt.
- **Integriteitswaarschuwingen:** een waarschuwing wanneer een controle aan de ontvangende kant mislukt.

De Ontvanger is strikt alleen-lezen. Het schrijft nooit naar de ontvangen repository, dus het kan nooit de append-only-garantie breken waar de zender op vertrouwt.

## Uitgewerkt voorbeeld: twee Unraid-machines, van begin tot eind

Hierboven staan de onderdelen. Dit is één volledige opstelling met echte waarden, want onderdelen zijn makkelijker samen te voegen als je ze één keer samengevoegd hebt gezien.

Twee machines: **TOWER** draait de containers en stuurt de back-ups, **VAULT** ontvangt ze en dwingt onveranderlijkheid af. Vul je eigen namen, adressen en sharepaden in.

**1. Zet op VAULT de append-only-server op.** Ga in BombVault op TOWER naar *Instellingen → Off-site → Instellen*, kies **rest-server** en genereer het recept. Kopieer het tabblad **Unraid-sjabloon (XML)**, sla het op VAULT op als `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, dan *Docker → Add Container* en kies **rest-server** uit de sjabloonlijst. Schrijf vóór het starten de getoonde `htpasswd`-regel op VAULT in `/mnt/user/appdata/rest-server/.htpasswd`. Het eenmalige wachtwoord wordt één keer getoond en nooit bewaard: kopieer het nu. Die regel bevat hetzelfde wachtwoord, al voor je gehasht met bcrypt: de platte tekst hoort in de REST-inloggegevens op TOWER, de gehashte regel in de `.htpasswd` op VAULT. Je hoeft zelf niets te hashen.

    Laat `--append-only` in het OPTIONS-veld staan. Daar draait alles om: zonder dat is VAULT weer een gewone share.

**2. Richt op TOWER de externe repo erop.** De repo-URL volgt het patroon dat het recept afdrukt:

    rest:http://VAULT:8000/bombvault-containers/containers

Het eerste padsegment is de htpasswd-gebruiker, het tweede de repository. Vul de gegenereerde gebruiker en het wachtwoord in als REST-inloggegevens van de bestemming en voer de **verbindingstest** uit.

**3. Zet op TOWER «Onveranderlijk» aan.** De manipulatietest loopt meteen en moet *beschermd* melden. Wat de antwoorden betekenen:

| Resultaat | Wat er gebeurde |
| --- | --- |
| **beschermd** | VAULT weigerde de verwijdering. Dit is de enige geslaagde toestand. |
| **NIET beschermd** | VAULT accepteerde een verwijdering. `--append-only` ontbreekt of is verwijderd. |
| **niet doorslaggevend** | Geen van beide. Meestal is de URL niet die welke restic zelf gebruikt, of de inloggegevens zijn gewijzigd. Er wordt niets vastgelegd en geen waarschuwing gegeven. |

**4. Kijk op VAULT wat er binnenkomt.** Koppel de twee machines ([Koppeling van instanties](#pairing)), zet *Instellingen → Algemeen → Ontvanger* aan, open het tabblad **Ontvanger** en registreer de repository alleen-lezen met TOWER als de zendende instantie.

!!! warning "De locatie is een pad **binnen** de container, geschreven ten opzichte van de host-mount"
    Vul `user/appdata/rest-server/bombvault-containers/containers` in, **niet** `/mnt/user/appdata/…`. BombVault draait in een container waar de `/mnt` van de host elders is gemount; een absoluut hostpad bestaat daar niet. Plak je er toch een, dan noemt BombVault nu het relatieve pad dat je nodig hebt.

    VAULT krijgt het restic-wachtwoord van TOWER via de groep zodra je opslaat; niemand hoeft een sleutel over te typen.

**5. Maak het wederzijds, als je wilt.** Herhaal dezelfde vijf stappen in de andere richting: een rest-server op TOWER die de kopie van VAULT ontvangt. Elke machine dwingt dan onveranderlijkheid af voor de andere, en geen van beide kan de back-ups van de ander verwijderen.

## Begeleid herstel

Een speciaal tabblad **Herstel** leidt een verse of herbouwde installatie op één plek door het noodgeval:

1. **Herstelt eerst BombVaults eigen instellingen**, zodat de back-uppaden, off-site doelen en inloggegevens die de rest van de flow nodig heeft al zijn ingevuld (toegepast via een self-restart over de Docker-socket, zodat de live instellingendatabase nooit onder een open handle wordt overschreven).
2. **Controleert of BombVault je back-ups kan lezen** (het encryptiesleutel-addertje vooraf).
3. Laat je **wijzen naar je bestaande repo** (lokaal of off-site).
4. **Ontdekt** de containers, VM's, bestandssets en ZFS-datasets die erin zijn opgeslagen.
5. **Herstelt de containers en VM's in één keer** (gestopt gelaten, zodat je ze bewust start) en toont de bestandssets en ZFS-items om een voor een te herstellen; ZFS-items komen uitgeschakeld terug. Je herstelkit is één klik weg.

!!! tip "Geplande migratie versus noodgeval"
    Begeleid herstel herstelt BombVaults eigen instellingen vanuit een back-up. Voor een *geplande* verhuizing naar een nieuwe machine kun je in plaats daarvan je configuratie rechtstreeks meenemen met de kaart **Instellingen exporteren / importeren** (een portable JSON-bestand). Zie [Configuratie](configuration.md#portable-settings-export-and-import).

### Herstellen vanuit een andere BombVault-repo {#restore-from-another-bombvault-repo}

Een aparte kaart op het tabblad **Herstel** opent de repo van een *andere* BombVault-instantie (een share gemount onder `/mnt`, of een remote URL) met **de `APP_KEY` van die instantie**, in een eenmalige, alleen-lezen sessie. Blader door de containers, VM's en bestandssets die daar zijn opgeslagen, kies een snapshot en herstel hem, en het herstelde object wordt een normale lokale container, VM of bestandsset. Er wordt nooit iets naar de andere repo geschreven, en je eigen back-upinstellingen blijven onaangeroerd (de sessie leeft in het geheugen en verloopt vanzelf). Een container van server A naar server B verplaatsen betekent niet langer je repo-instellingen omleiden en die achteraf terugdraaien. Deze kaart is eenmalig: ze opent een sessie, herstelt wat je kiest en vergeet de andere instantie. Wil je in plaats daarvan een vaste regeling, waarbij deze machine volgens een planning de snapshots van een andere instantie in haar eigen repository ophaalt, dan is dat het tabblad **Ophalen** van de pagina **Instanties**.

## Herstelkit voor de encryptiesleutel

Dit is het onderdeel dat noodherstel mogelijk maakt, zelfs wanneer er geen draaiende BombVault is.

Eén klik downloadt de **hoofdsleutel**, het **afgeleide restic-wachtwoord** en de **exacte repo-locaties en commando's**, zodat je rechtstreeks met de restic-CLI op elke machine kunt herstellen. Een Dashboard-herinnering zeurt totdat je hem hebt bewaard.

!!! danger "Bewaar de herstelkit buiten de server"
    De kit bevat het geheim dat je back-ups ontsleutelt. Bewaar hem ergens veilig en gescheiden van de server (een wachtwoordmanager, een geprinte kopie in een kluis). Als je zowel BombVault als `APP_KEY` verliest zonder herstelkit, kunnen je versleutelde back-ups niet worden hersteld.

!!! warning "De nieuwste snapshot is niet altijd de juiste om te herstellen"
    Sinds restic 0.17 toont `restic snapshots` de grootte van elke snapshot. Na dataverlies kan de nieuwste snapshot de leeggemaakte zijn, dus herstel geen snapshot die veel kleiner is dan de vorige. Na ransomware kan het de versleutelde zijn, met de gebruikelijke grootte. Draait BombVault nog, kijk dan eerst op de pagina **Anomalieën**: die noemt de laatste goede back-up. Een herstel heeft geen anomaliegegevens van BombVault nodig, en de retentiepauze bewaart alleen maar meer snapshots.

### De kit verzegelen

Als je age-versleuteling voor de platte exports hebt aangezet (Instellingen), wordt de kit daar ook mee verzegeld en gedownload als `bombvault-recovery-kit.md.age`. Hij is ASCII-armored in plaats van binair, dus het blijft platte tekst: in een wachtwoordmanager plakken of afdrukken werkt precies als voorheen, alleen is de inhoud zonder je sleutel onleesbaar.

!!! warning "Bewaar de age-sleutel niet in de kit"
    Je hebt je **privé**-age-sleutel nodig om een verzegelde kit te openen. Bewaar die ergens waar hij niet van de kit zelf afhangt, anders heb je twee dingen te herstellen in plaats van één. Verzegelen loont als de kit ergens ligt waar je niet volledig zelf over gaat (een gedeelde wachtwoordmanager, notities in de cloud, een afdruk op kantoor); een kit in je eigen kluis wordt al door de kluis beschermd.

    Met versleuteling aan en geen bruikbare ontvanger ingesteld wordt de download volledig geweigerd. BombVault geeft de hoofdsleutel nooit alsnog onversleuteld af.

### Als het pakket niet bij de hand is

Het wachtwoord staat nergens opgeslagen, het wordt **berekend** uit de `APP_KEY`. Met de sleutel en een shell kun je het dus zelf namaken:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Dat is HMAC-SHA256 over de vaste tekst `bombvault:restic-repo`, met de ruwe bytes van de hexadecimale `APP_KEY` als sleutel, weergegeven als 64 hexadecimale tekens in kleine letters. Dezelfde waarde staat in het pakket, als afgeleid restic-wachtwoord; dit is voor de dag dat het pakket ergens anders ligt dan jij.

!!! warning "Gebruik bij een ontvangen repository de sleutel van de VERZENDENDE instantie"
    Een repository dat hier via off-sitereplicatie is beland, is aangemaakt door de machine die het stuurde, met **diens** `APP_KEY`. Afleiden uit de sleutel van de ontvangende machine geeft een wachtwoord dat restic weigert, wat precies leest als een kapot repository terwijl het dat niet is. Dat is de gebruikelijke reden dat `restic check` op een ontvangen repository steeds opnieuw om het wachtwoord vraagt.

Omdat hersteldefinities **binnen** elke repo leven (`<repo>/def`, `<repo>/vm-def`), is een gekopieerde repo-map volledig zelfstandig, dus de kit plus de repo is alles wat een bare-metal-herstel nodig heeft.

## Een databasedump terughalen {#database-dumps}

Een databasedump is een eigen herstelpunt in de containerrepository, met het label `dbdump:<container>` en één bestand, `/dbdump/<container>.sql`. BombVault toont, downloadt en importeert ze onder **Back-ups**; hieronder staan dezelfde stappen met alleen restic, voor de dag dat BombVault er niet is.

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
