# Back-ups en items

Alles wat BombVault back-upt, staat in één lijst onder **Back-ups**: containers, VM's, mappensets, de flash, ZFS-datasets en de zelf-back-up. Elk daarvan is een **item** met een eigen pagina. Deze pagina loopt langs de lijst, de itempagina met haar drie tabbladen en de instellingen die een item kan hebben.

![De lijst Back-ups met de tegel Containers gekozen: elke rij toont zijn laatste runs als een strook en één status.](assets/screenshots/backups/list-dark.png#only-dark)
![De lijst Back-ups met de tegel Containers gekozen: elke rij toont zijn laatste runs als een strook en één status.](assets/screenshots/backups/list-light.png#only-light)

## Wat elk onderdeel opslaat {#what-is-saved}

| Onderdeel | Wat wordt opgeslagen |
|---|---|
| **Containers** | De appdata-map plus de containerdefinitie: image, variabelen, poorten, labels en volumes. De kaart **Back-upmappen** op het tabblad **Instellingen** van de container vinkt precies aan welke mappen de back-up omvat, met een live telling van de paden en een lijst van wat je hebt weggelaten. Data die een container buiten Host Data bewaart, bijvoorbeeld in een TrueNAS-ixVolume of een Compose-project onder `/opt`, wordt niet geback-upt, en de run noemt de mappen die hij heeft overgeslagen. De run noemt ook een map binnen Host Data die er op dit moment niet is. Systeempaden zoals `/etc/localtime` of de Docker-socket tellen niet als data. |
| **VM's** | De schijfimages, de XML-definitie en de UEFI NVRAM, met een nette afsluiting of een live-snapshot, via SSH. Zie [VM-back-ups](vm-backup-ssh-setup.md). |
| **Flash** | Alleen Unraid. De hele USB-flash (`/boot`): OS, licentie, array-configuratie, shares, netwerk- en plugin-configuratie. |
| **Zelf-back-up** | BombVaults eigen `/config`: de instellingendatabase, de inloggegevens van de bestemmingen, het libvirt SSH-sleutelpaar, de sleutel en de vastgezette hostsleutels van de ZFS-servers en de certificaten, vastgelegd met SQLite `VACUUM INTO`, zodat de database nooit midden in een schrijfactie wordt gekopieerd. |
| **Mappen** | Elke map op de server als mappenset, zoals een share, je documenten of een fotobibliotheek, met optionele uitsluitpatronen. Mappensets hebben planningen, bewaarbeleid, off-site-kopieën, controles en herstelproeven zoals al het andere. |
| **ZFS-datasets** | Een dataset en elke dataset eronder, gelezen uit één ZFS-snapshot zodat alles van hetzelfde moment komt, en met restic opgeslagen als een map. Een ZFS-item kan ook een blokgetrouwe replica op een tweede ZFS-server bewaren. Zie [ZFS-datasets](zfs-datasets.md). |

Elke back-up loopt via restic en is dus incrementeel en gededupliceerd, zodat zelfs grote VM-schijven het repository niet opblazen. BombVault maakt nooit een back-up van zijn eigen container en stopt die dus ook nooit. Een run kan veilig worden gestopt, want restic schrijft zijn snapshot als laatste, dus een afgebroken run laat ongebruikte data achter en geen snapshot. Een lock die een herstart midden in een run heeft achtergelaten, wordt opgeruimd en de run wordt nog één keer geprobeerd, pas nadat de lock tien minuten niet is ververst.

## De lijst {#list}

Tegels boven de lijst kiezen een soort, zoals **Containers** of **ZFS**, en elke tegel telt zijn items. Een tegel selecteert, hij verbergt de rest van de pagina niet.

Elke rij toont de laatste veertien runs van het item als kleine vierkantjes en één status, zoals **Beschermd**, **Niet beschermd**, **Uitgesteld** of **Bezig met repliceren**. Al het andere staat in de infobubbel naast de naam, en zolang de aanwijzer op de rij rust, staat die tekst in de rij zelf. Een markering naast de naam van een container zegt dat hij sinds zijn laatste back-up opnieuw is aangemaakt met een andere image, andere poorten, variabelen of volumes. Ze verdwijnt met de volgende back-up.

Drie knoppen boven de lijst beperken haar:

- **Filters**: op **Planning** (**Alle**, **Gepland**, **Gepauzeerd**), op **Back-up** (**Met back-up**, **Zonder back-up**) en, voor **Containers en VM's**, **Geïnstalleerd** of **Niet geïnstalleerd**.
- **Sorteren**: op naam, status, IP, laatste back-up, grootte of planning. Elk veld behalve IP sorteert beide kanten op, zoals **Grootte, grootste eerst** en **Grootte, kleinste eerst**.
- **Zoeken**: op naam, image of IP.

Items die niet meer geïnstalleerd zijn maar nog back-ups hebben, staan apart onder **Niet geïnstalleerd (alleen back-ups)**. Herstel ze, of verwijder hun back-ups om de ruimte vrij te maken.

De hoofdactie van de pagina zweeft rechtsonder: **Nu alles back-uppen** start een run over elk ingeschakeld onderdeel. **Meer** bevat de rest:

- **Selecteren** begint een selectie, zie hieronder.
- **Back-ups ontdekken** leest wat de bestemmingen bevatten en bouwt daaruit items opnieuw op, bijvoorbeeld na een verse installatie. Een container zonder eigen data heeft geen snapshot en komt terug uit de definitie die naast het repository gespiegeld staat. Off-site-kopieën van opnieuw opgebouwde onderdelen wachten dan tot je hun back-ups overneemt als back-ups van deze server, zie [Na een herinstallatie](offsite-recovery.md#after-a-rebuild).
- **ZFS-verbinding met deze server** opent de SSH-test van het onderdeel ZFS, zie [ZFS-datasets](zfs-datasets.md#requirements).
- **Nu alles repliceren**, met de tegel ZFS gekozen, verstuurt de replica van elk ZFS-item.

![Dezelfde lijst op een telefoon.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![Dezelfde lijst op een telefoon.](assets/screenshots/backups/list-phone-light.png#only-light)

## Meerdere items kiezen {#select}

Kies rijen zoals je bestanden kiest in een bestandsbeheerder: Ctrl+klik, Shift+klik, Ctrl+A, of lang drukken op een telefoon. Een gekozen rij toont een vinkje in plaats van haar icoon, en een balk boven de lijst telt ze en biedt **Selectie back-uppen**, **Schema pauzeren** en **Schema hervatten**, **Alles selecteren** en **Selectie wissen**. Elke rij heeft ook een rechtsklikmenu met zijn eigen acties.

Een selectie wordt op de server geback-upt, het ene item na het andere, dus ze loopt door als je het tabblad sluit. Om meerdere items samen te herstellen gebruik je de [pagina Herstel](restore.md#several).

![Twee containers gekozen, met de balk boven de lijst.](assets/screenshots/backups/bulk-dark.png#only-dark)
![Twee containers gekozen, met de balk boven de lijst.](assets/screenshots/backups/bulk-light.png#only-light)

## Een map of een ZFS-dataset toevoegen {#add}

Containers, VM's en de flash verschijnen vanzelf zodra hun onderdeel aanstaat onder **Instellingen, Algemeen**. Mappen en ZFS-datasets kies je zelf, want BombVault kan niet raden welke je bedoelt. **Toevoegen** opent **Wat moet BombVault verder back-uppen?**:

- **Eén map** toont de shares die BombVault vond en neemt elke andere map onder Host Data. Geef de mappenset een naam, zoals `Documents`. De naam ligt na de eerste back-up vast, omdat de snapshots eronder worden opgeslagen.
- **Een ZFS-dataset** toont de datasets van deze server. Kies degene bovenaan wat je wilt back-uppen, zie [Items en onderliggende datasets](zfs-datasets.md#items-and-children).
- **Systeemconfiguratie** verschijnt op een host zonder Unraid waarvan Host Data tot `/etc` reikt. Het is een voorgestelde mappenset voor `/etc` die je nakijkt voordat je opslaat.

## Een item weglaten {#dont-back-up}

Een item dat je bewust weglaat, zoals een testcontainer, kun je markeren met **Niet back-uppen**. Het telt dan niet meer als onbeschermd, en **Niet meer uitsluiten** haalt het terug. **Schema pauzeren** is de zachtere weg: het item verlaat de planning en de volledige back-up, krijgt een grijze badge **Schema gepauzeerd**, en **Nu back-up maken** werkt nog steeds. Het zet dezelfde schakelaar om als **In de planning** op het tabblad Instellingen van het item.

## De pagina van een item {#entry-page}

Een klik op een rij opent de pagina van het item. De kop noemt het item en zijn toestand in één regel, zoals **Laatste back-up 3 uur geleden** of **Uitgesteld, volgende poging om 03:15**, en bevat zijn acties: **Nu back-up maken**, **Herstellen** en onder **Meer** de zeldzame, zoals **Nieuwste back-up exporteren**, **Container exporteren** of **VM exporteren**, **Koppelen aan een eerder item**, **Alle back-ups verwijderen**, **Item verwijderen** en **Back-ups overal verwijderen**. Een terugknop links van het icoon leidt naar de lijst. Terwijl een back-up loopt, staat **Back-up annuleren** naast de voortgang, en een geannuleerde run wordt als geannuleerd vastgelegd, niet als mislukt.

**Back-ups overal verwijderen** verwijdert de back-ups van het item in één keer op elke bestemming, off-site-kopieën inbegrepen. Het venster toont elke plek met het aantal back-ups dat er staat, en de knop wordt vrijgegeven zodra je de naam van het item typt. Een plek die op append-only staat, houdt zijn back-ups en meldt dat. Het item en de instellingen blijven, dus een item dat je niet meer back-upt, blijft zo, en alleen de back-ups en hun geschiedenis zijn weg.

De pagina heeft drie tabbladen: **Back-ups**, **Instellingen** en **Verificatie**.

### Tabblad Back-ups {#backups-tab}

Dit tabblad toont de snapshots van het item, nieuwste eerst, met een balk per dag. Een back-up die op meerdere plekken staat, verschijnt één keer, gemarkeerd met elke bestemming die haar bevat. Een klik op een snapshot herstelt hem, en een klik op een plek binnen een snapshot herstelt vanaf precies die plek.

- **Bestemmingen** noemt elke plek met back-ups van dit item en hoeveel. Off-site-plekken worden pas gelezen als je op **Controleren** drukt, omdat het uitlezen van cloudopslag even duurt. Een plek waar het item niet meer naartoe back-upt, staat als **Alleen eerdere back-ups** en kan nog steeds worden hersteld.
- Elke snapshot heeft een menu: **Tag toevoegen** of **Tag wijzigen**, **Vergelijken met een andere back-up**, dat de bestanden toont die erbij kwamen, verdwenen of veranderden, **Exporteren** als tar, zip of een eigen restic-repository, en verwijderen op één plek alleen.
- **Databasedumps** van een container staan in een eigen lijst, met downloaden, opslaan in een map en importeren.

Hoeveel snapshots blijven bestaan, bepaalt de bestemming, zie [Hoe lang bewaren](offsite-recovery.md#keep).

![Het tabblad Back-ups van een container: een balk per dag en de bestemmingen die elke snapshot bevatten.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![Het tabblad Back-ups van een container: een balk per dag en de bestemmingen die elke snapshot bevatten.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Tabblad Instellingen {#settings-tab}

Elk item begint met **Planning en bestemming**:

- **In de planning** neemt het item op in of haalt het uit de geplande runs.
- **Eigen plan** geeft dit ene item een eigen planning, zie [Een eigen planning voor één item](schedules.md#own-plan). Zonder eigen planning loopt het mee met de planning van zijn onderdeel of in de volledige back-up.
- Een rij knoppen, **Lokaal** en één per off-site-bestemming, zegt waarheen de back-ups gaan. **Volgt de standaard** betekent dat het item gaat waar zijn onderdeel naartoe gaat, en een klik erop opent **Bestemmingen per onderdeel**. Zie [Plaatsing per item](offsite-recovery.md#placement).
- **Repository** noemt waar de back-ups worden geschreven (**Opgeslagen op**). Die plek ligt vast vanaf de eerste back-up, de kopieën kunnen op elk moment veranderen.

De kaarten daaronder hangen af van het soort item:

| Soort | Kaarten |
|---|---|
| Container | **Back-upmappen** (welke gekoppelde mappen worden geback-upt, met **Cachemappen overslaan (CACHEDIR.TAG)**), **Uitsluitpatronen**, **Andere containers stoppen**, **Opdrachten ervoor en erna**, **Database**, **Als de app bezig is**, **Bijwerken na een geslaagde back-up**, **Snapshot in plaats van stoppen**, **Eerdere namen** |
| VM | **VM** met **Methode** (**Net (afsluiten)** of **Live-snapshot**) en **Alleen gewijzigde blokken**, zie de [VM-back-upgids](vm-backup-ssh-setup.md#per-vm-method) |
| Mappenset | **Map** en **Uitsluitpatronen** |
| Flash | **Flash-zip-export**, die elke flashback-up als gewone `.zip` wegschrijft naar een map die je kiest, in één stap, zodat een synchronisatieprogramma nooit een half bestand ziet. Een flashback-up die minder bevat dan zou moeten, laat de laatste ZIP staan, en de run van de export meldt dat. |
| ZFS-dataset | de onderliggende datasets, de containers die gestopt worden en de opdrachten rond de snapshot, **Replica** en **Toegang tot snapshots testen**, zie [ZFS-datasets](zfs-datasets.md) |

Instellingen worden vanzelf opgeslagen terwijl je ze wijzigt.

![Het tabblad Instellingen van een container, met Eigen plan in de eerste kaart.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![Het tabblad Instellingen van een container, met Eigen plan in de eerste kaart.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Tabblad Verificatie {#verify-tab}

- **Controles** toont de herstelcontrole die op de eerste back-up volgt. **Herstel controleren** voert haar opnieuw uit op de nieuwste back-up: BombVault herstelt een steekproef van maximaal 100 bestanden en 256 MiB naar een tijdelijke map en laat restic elk bestand teruglezen tegen de hashes.
- **Starttest**, bij een container, herstelt de nieuwste back-up naar een geïsoleerde kopie en start die: een naam die met `bombvault-test-` begint, een eigen intern Docker-netwerk zonder gepubliceerde poorten en zonder weg naar het LAN, 1 CPU en 2 GiB geheugen. De test slaagt als de kopie healthy meldt, antwoordt op zijn eerste blootgestelde poort, of, als geen van beide kan, blijft draaien. Het origineel wordt nooit aangeraakt, en de kopie, haar netwerk en haar data worden daarna verwijderd, ook na een herstart midden in een test. Containers op het hostnetwerk, geprivilegieerde containers, containers met apparaten en containers die een andere container nodig hebben, staan als niet testbaar. **Starttest** onder **Instellingen, Verificatie, Automatische controles** test één container per run, steeds degene die het langst geleden is getest.
- **Anomalieën** stelt de eigen gevoeligheid en het meldingsniveau van dit item in en toont zijn openstaande bevindingen.
- **Grootte per map** toont wat de ruimte in de nieuwste back-up inneemt en hoeveel daarvan de laatste back-up erbij bracht, niveau voor niveau.
- **Recente uitvoeringen** toont de laatste runs met start, einde en duur. Een klik toont de details.

![Het tabblad Verificatie van een container met Grootte per map geopend.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![Het tabblad Verificatie van een container met Grootte per map geopend.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Uitsluitpatronen {#excludes}

De kaart **Uitsluitpatronen** neemt één patroon per regel. Typ paden zoals je ze binnen de container ziet, zoals `/config/Library/Application Support/Plex Media Server/Cache`, of een kale naam zoals `.git`, die op elke diepte past. Bij een mappenset typ je een pad zoals de host het kent, zoals `/mnt/user/data/cache`. Een voorbeeld onder het veld toont waartoe elke regel leidt en waarschuwt als een regel niets raakt of alles zou uitsluiten. Een regel met `/` of `*`, of een die een hele geback-upte map noemt, wordt genegeerd, zodat een uitsluiting een back-up nooit leeg kan maken. Wil je een hele map weglaten, haal dan het vinkje weg onder **Back-upmappen**.

Voor bekende images zoals Plex, Jellyfin, Sonarr, Radarr, Immich en Nextcloud biedt de kaart de mappen aan die de app zelf weer vult, zoals caches, logs en miniaturen. Er wordt niets uitgesloten tot je op **Selectie uitsluiten** drukt.

Een nieuwe mappenset begint met de groep **Systeemrommel** ingeschakeld: `.DS_Store`, `Thumbs.db`, `desktop.ini`, `@eaDir` (Synology), `.Recycle.Bin` (de Unraid Recycle Bin plugin), `.Trash-*` en `*.tmp`. Elke regel is een kale naam en past dus op elke diepte onder de map, en je kunt elke regel uitzetten voordat je de set aanmaakt. Een bestaande mappenset houdt de uitsluitingen die hij heeft. **Systeemrommel uitsluiten** in het bewerkvenster biedt dezelfde regels aan, en die komen pas in de set als je op **Selectie uitsluiten** drukt en opslaat.

Dezelfde kaart bevat waarschuwingen die een mappenscan niet kan zien. Immich bewaart bijvoorbeeld zijn albums, gezichten en data in een PostgreSQL-database in een aparte container, dus een bestandsback-up van alleen de Immich-container herstelt de foto's zonder dat alles, en het herstel ziet er geslaagd uit. Maak ook een back-up van de databasecontainer.

## Databasedumps {#database-dumps}

PostgreSQL-, MySQL- en MariaDB-containers die BombVault herkent, worden vóór elke back-up gedumpt, vanaf de draaiende server, in een eigen herstelpunt naast de bestanden. Dat omvat de officiële images, PostGIS, TimescaleDB, pgvector, pgautoupgrade, de databaseimages van Immich, de MariaDB-images van linuxserver, yobasystems en jc21 en mysql-server van Oracle. Een container die alleen op een database lijkt, krijgt dezelfde optie, uitgeschakeld tot je haar kiest.

De dump stroomt rechtstreeks in het repository en wordt nooit naar een schijf geschreven. De inloggegevens komen uit de eigen variabelen van de container, `*_FILE`-geheimen inbegrepen, en verlaten de container nooit. De kaart **Database** op het tabblad Instellingen van de container zegt of de datamap wordt opgeslagen terwijl de container gestopt is, wordt gekopieerd terwijl hij draait of niet wordt opgeslagen, en schakelt de dump voor deze container in of uit. Het label `bombvault.dbdump=false` doet hetzelfde en gaat voor op de kaart. **Automatische databasedumps** onder **Instellingen, Algemeen** schakelt het voor alle containers.

Een mislukte dump laat de back-up niet mislukken. Hij verschijnt als een eigen mislukte run met de reden en stuurt een melding. Dumps worden nooit uit zichzelf teruggeladen: download er een, gewoon of gecomprimeerd, sla hem op in een map, of importeer hem met één klik op het tabblad **Back-ups** in een vers gestarte database. Om een dump terug te krijgen zonder BombVault, zie [Een databasedump terughalen](offsite-recovery.md#database-dumps). Als er een mislukt, zie [Een databasedump is mislukt](troubleshooting.md#a-database-dump-failed).

## Opdrachten en andere containers {#commands}

- **Opdrachten ervoor en erna** draaien in de container met `sh -c`, bijvoorbeeld om een cache naar schijf te schrijven. Een opdracht vóór de back-up die mislukt, breekt de back-up af. Een opdracht vóór de back-up die met code 75 eindigt, stelt in plaats daarvan een geplande back-up uit, zie hieronder.
- **Andere containers stoppen** noemt containers die gestopt worden zolang deze wordt geback-upt, zoals zijn database. Daarna starten ze weer in hun Compose-volgorde `depends_on`, en standaard moet elke container healthy melden voordat de volgende start. Het wachten en de time-out staan onder **Herstarten na back-up** in de kaart **Volgorde** onder **Instellingen, Planning**.
- Een container die de netwerk-, PID- of IPC-namespace deelt van een container die stopt, via `--net=container:` of `network_mode: service:` in Compose, stopt en start mee, ook als hij niet op de lijst staat, en hij start na de container waarbij hij zich aansluit. Een herstarte container krijgt een nieuw netwerk, en een container die op het oude netwerk bleef draaien, zou alleen zijn loopback hebben. De lijst zelf blijft zoals je hem hebt ingesteld. Kon zo'n container niet gestopt worden, dan meldt de run dat hij geen netwerk heeft tot hij opnieuw is gestart. Hetzelfde geldt voor de eigen container van BombVault, die zichzelf nooit stopt en niet op de lijst kan worden gezet.
- Een container die daarna niet meer draait, wordt genoemd in de run en in een melding, ook als de back-up zelf is mislukt.

## Als de app bezig is {#app-busy}

Een back-up stopt zijn container, dus een geplande back-up kan wachten terwijl iemand de app gebruikt en het uit zichzelf opnieuw proberen. De kaart **Als de app bezig is** op het tabblad Instellingen van een container stelt dat in:

- **Activiteitscontrole** vraagt Plex, Jellyfin of Emby vóór een geplande back-up of er iemand kijkt. Plex wordt gevraagd met het token uit zijn eigen `Preferences.xml`, dus er valt niets in te vullen. Jellyfin en Emby nemen een **API-sleutel**, aangemaakt in het dashboard van de server onder API keys.
- **Gepauzeerde sessies negeren na (min)** laat een film die voor de nacht is gepauzeerd, ophouden mee te tellen.
- **Back-up maken als de server niet antwoordt** bepaalt wat er gebeurt als de server niet bevraagd kan worden: aan, dan loopt de back-up en zegt het logboek dat. Uit, dan wacht hij alsof er iemand kijkt.
- **Wachten zolang de container data verstuurt** is voor apps zonder zo'n controle, zoals een gameserver of een downloader. De back-up wacht zolang de container bij een meting binnen de laatste **Rustig gedurende (min)** minuten meer dan **Bezig vanaf (KiB/s)** verstuurde.
- **Opnieuw proberen elke (min)**, **Alleen binnen een tijdvenster proberen** met **Proberen tussen**, en **Hoogstens wachten (uur)** bepalen hoe de pogingen verlopen.
- **Aan het einde van het wachten** geeft ofwel op, waarna de back-up onder **Aandacht nodig** staat met **Opnieuw proberen**, ofwel maakt **Toch back-up maken**.

Een wachtende back-up wordt vastgelegd als **Uitgesteld**, in amber, en de rest van de run gaat door. Het overzicht, de lijst en het item tonen hem met de volgende poging, en de melding van achterstallige back-ups en de beschermingsstatus wachten erop tot zijn deadline. Je krijgt één melding als hij wordt uitgesteld en één als hij eindigt, niet één per poging. Een poging wacht zolang een andere back-up loopt, en een wacht gaat door na een herstart. Een back-up die je zelf start, loopt altijd meteen, of het nu met **Nu back-up maken**, **Opnieuw proberen**, een selectie, de API of MCP is, en een die doorgaat, beëindigt een openstaande wacht.

Het tabblad Instellingen van de container heeft ook een oudere, eenvoudiger manier: **Wachten tot de app rustig is** met **Hoogstens wachten (uur)**. De back-up start zodra de app rustig is, wat de kaart **Rust voor de back-up** onder **Instellingen, Planning** bepaalt. De leden van een Compose-stack die in dezelfde run aan de beurt zijn, wachten samen, en de wachtende back-up houdt geen lock vast, zodat de andere containers doorgaan.

![De kaart Als de app bezig is van een Plex-container met de activiteitscontrole ingesteld.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![De kaart Als de app bezig is van een Plex-container met de activiteitscontrole ingesteld.](assets/screenshots/backups/app-busy-light.png#only-light)

### Voorbeeld: Plex wacht zolang iemand kijkt {#example-plex}

De volledige back-up loopt elke nacht om 03:00. Niemand hoort een film te zien stoppen omdat Plex voor zijn back-up uitging, maar de back-up moet wel voor de ochtend gebeuren.

1. Open **Back-ups**, dan het item `plex`, dan zijn tabblad **Instellingen**.
2. Zet in **Als de app bezig is** de **Activiteitscontrole** op Plex. Er valt niets te typen.
3. Zet **Gepauzeerde sessies negeren na (min)** op `30`, zodat een film die om middernacht is gepauzeerd, om half een niet meer meetelt.
4. Laat **Back-up maken als de server niet antwoordt** aan.
5. Zet **Opnieuw proberen elke (min)** op `15`.
6. Schakel **Alleen binnen een tijdvenster proberen** in en zet **Proberen tussen** op `22:00` en `06:00`.
7. Zet **Hoogstens wachten (uur)** op `3` en **Aan het einde van het wachten** op **Toch back-up maken**.

Wat er gebeurt op een nacht dat om 03:00 nog iemand kijkt: BombVault vraagt Plex, vindt een sessie die afspeelt en legt de back-up vast als **Uitgesteld** met de volgende poging om 03:15. De andere containers van de run worden gewoon geback-upt. Als de film om 04:10 afloopt, vindt de poging van 04:15 Plex rustig en maakt de back-up, en het wachten eindigt met één melding. Zou de film om 06:00 nog lopen, dan bereikt het wachten zijn einde en wordt Plex toch geback-upt, zodat zijn back-ups nooit meer dan drie uur achterlopen.

Voor je eigen controle laat je het **Pre-back-upcommando** met code 75 eindigen zolang de app bezig is, bijvoorbeeld `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Meer symptomen en oplossingen staan bij [Problemen oplossen](troubleshooting.md#deferred-backups).

## Gewone exports {#exports}

restic blijft de motor, maar een export geeft je een kopie die noch BombVault noch restic nodig heeft.

- **Container exporteren** en **VM exporteren** onder **Meer** op de pagina van een item schrijven een kopie weg zoals het nu is, naast het repository: de back-upmappen van de container plus zijn Unraid-template, of de schijfimages van de VM plus zijn definitie, te herstellen met `virsh define`.
- **Exporteren** op een snapshot schrijft een opgeslagen back-up weg naar de exportmap op de server, als tar, zip of een eigen restic-repository, en kan één snapshot of alle snapshots nemen. Een flashback-up en een zelf-back-up bevatten sleutels, dus het exporteren ervan vraagt een inlogwachtwoord en een aanmelding op de instantie.

Exports zijn standaard gewone bestanden. **Exports versleutelen met age** onder **Instellingen, Systeem** verzegelt ze, de flash-ZIP inbegrepen, voor de ontvangers die je opgeeft, elk een publieke age-sleutel of een publieke SSH-sleutel. Je ontsleutelt ze later buiten de server met de bijbehorende privésleutel. Staat versleuteling aan en is er geen geldige ontvanger, dan mislukt een export met een duidelijke fout in plaats van ooit gewone bestanden te schrijven.

## Wat een run je vertelt {#run-notes}

- **Timing.** Elke run toont start, einde en duur, en elk item heeft zijn eigen **Recente uitvoeringen**.
- **Waarom een back-up traag was.** Terwijl een back-up loopt, houdt BombVault bij hoe druk de CPU, de schijven en het netwerk zijn. Duurt een back-up veel langer dan gewoonlijk en zat één ding duidelijk aan zijn limiet, dan zegt de run dat, bijvoorbeeld "The target disk disk1 was 98% busy". Anders zegt hij niets.
- **Voortgang van controles en opschoonrondes.** Het activiteitenlogboek en de kaart **Back-ups controleren** tonen hoe ver restic heeft geteld, zoals *12 van 47 packs*, met de resterende tijd van die stap zodra er genoeg is om die te schatten.
- **Bestanden die niet gelezen konden worden.** Een back-up die doorging maar iets oversloeg, kleurt zijn regel oranje en noemt wat is weggelaten.
