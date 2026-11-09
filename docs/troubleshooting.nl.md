# Problemen oplossen

Een korte FAQ. Voor de volledige tabel voor probleemoplossing aan de hostkant bij VM-back-up via SSH (permission denied, controle van de hostsleutel, ontbrekende templatevariabelen en meer), zie de [gids voor VM-back-up via SSH](vm-backup-ssh-setup.md).

## Er is iets niet goed aangesloten {#something-is-not-wired-up-correctly}

Voeg de kaart **Host-integratie** toe aan het overzicht onder **Aanpassen**. De controle van de hostintegratie test elke mount en elk CLI-programma (Docker-socket, libvirt, restic, qemu-img, rclone) en meldt wat er ontbreekt, en **Nu controleren** voert haar opnieuw uit. Begin hier voordat je een bug aanneemt: een ontbrekende mount of een onbereikbare host valt meteen op.

## Ik kan de webinterface niet bereiken {#i-cannot-reach-the-web-ui}

BombVault serveert standaard HTTPS op poort `3443` (zelfondertekend certificaat), dus open `https://<je-unraid-ip>:3443`. Accepteer de waarschuwing over het zelfondertekende certificaat, of zet BombVault achter een reverse proxy met je eigen certificaat. Draai je met `HTTP_ONLY=true`, dan serveert het in plaats daarvan gewoon HTTP op poort `3000` (bedoeld voor gebruik achter een proxy die TLS afhandelt).

## Ik ben mijn APP_KEY kwijt {#i-lost-my-app_key}

`APP_KEY` leidt het wachtwoord van het restic-repository af. Zonder hem, zonder de herstelkit en zonder bewaarkopie van de sleutel in je groep zijn versleutelde back-ups niet te herstellen. Daarom toont het overzicht **Bewaar je herstelkit** tot je haar hebt gedownload. Zie [Off-site en herstel](offsite-recovery.md#recovery-kit). Genereer een sleutel met `openssl rand -hex 32` en bewaar hem buiten de server voordat je op een back-up vertrouwt.

## Ik ben het inlogwachtwoord vergeten {#i-forgot-the-login-password}

Kies **Wachtwoord vergeten?** op de inlogpagina. BombVault schrijft een code van 12 tekens naar het containerlogboek (op Unraid: tabblad Docker, BombVault-icoon, **Logs**). Typ haar binnen 10 minuten met een nieuw wachtwoord. De reset zet ook tweefactorauthenticatie uit, verwijdert elke passkey, trekt elk API-token, elke MCP-sleutel en elke OAuth-aanmelding in, beëindigt elke andere sessie en zet het koppelen van telefoons uit. Voeg je passkeys opnieuw toe onder **Instellingen, Beveiliging**, schakel het koppelen van telefoons daar weer in en maak nieuwe tokens en sleutels aan waar je ze nog nodig hebt. Zie [Beveiligingsmodel](configuration.md#login).

## Back-ups staan stil en zeggen dat de sleutel niet meer te openen is {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

Deze server nam in de Noodassistent de back-upsleutel van een andere server over, en `APP_KEY` is daarna gewijzigd. De overgenomen sleutel is verzegeld met de oude APP_KEY, dus BombVault kan hem niet openen en maakt geen back-ups in plaats van met de verkeerde sleutel te schrijven. **Aandacht nodig** op het overzicht noemt het. Zet APP_KEY terug op wat hij was, of upload de herstelkit opnieuw op de pagina Herstel onder **Noodgeval**.

## Een telefoon belandt op de inlogpagina {#a-phone-lands-on-the-login-page}

Een gekoppelde telefoon krijgt zijn sessie alleen via de groep zolang **Telefoons kunnen koppelen met de twaalf woorden van de groep** aanstaat onder **Instellingen, Beveiliging**. Een wachtwoordreset zet het uit. Een app die ouder is dan de server krijgt ook geen sessie tot hij is bijgewerkt. Een server die via zijn adres is toegevoegd, vraagt altijd om het wachtwoord. Een telefoon die van de pagina Instanties is verwijderd, komt uitgeschakeld terug als zijn app weer opent. Schakel **Deze instantie bevragen** onder **Bewaking** in zijn details in om hem binnen te laten.

## VM-back-up kan niet verbinden {#vm-backup-will-not-connect}

VM-back-up praat met libvirt via SSH, nooit via een mount.

- Controleer of SSH op de host is ingeschakeld en de publieke sleutel van BombVault is geautoriseerd in `/root/.ssh/authorized_keys` (Instellingen, Integraties, Host-SSH toont de sleutel en een knop **Verbinding testen**).
- Stel op een eigen `br0.x`-netwerk `LIBVIRT_HOST` in op het LAN-IP van je Unraid (de container kan de host daar niet via `host.docker.internal` bereiken). Schakel **Settings, Docker, Host access to custom networks** in.
- Heb je de SSH-poort van Unraid gewijzigd, zet dan `LIBVIRT_SSH_PORT` erop af.
- Stel op TrueNAS `LIBVIRT_URI` in met de eigen libvirt-socket van TrueNAS, geef de app de capability `SETPCAP` en voeg de sleutel toe aan root onder **Credentials, Users** (zie [TrueNAS SCALE](getting-started.md#truenas)).
- De volledige stapsgewijze diagnose (bereikbaarheidstest, VLAN-routering, `Permission denied (publickey)`, `Host key verification failed`) staat in de [gids voor VM-back-up via SSH](vm-backup-ssh-setup.md).

## Een live VM-snapshot liep niet {#a-live-vm-snapshot-did-not-run}

Live-snapshots hebben de schijf op `/mnt/cache` (of `/mnt/diskX`) nodig, niet `/mnt/user`. Met de qemu guest agent in de VM is de snapshot gequiesceerd, en zonder agent is hij crashconsistent. Bij een uitgeschakelde VM valt live automatisch terug op netjes afsluiten. Een back-up met netjes afsluiten sluit de VM af, maakt een back-up van de schijven en start hem daarna weer, dus hij is altijd consistent. Hij wacht tot tien minuten tot de gast is afgesloten en forceert hem nooit uit, zodat een Windows-gast die updates installeert niet halverwege wordt afgekapt. Een gast die daarna nog draait, wordt met rust gelaten, en die back-up mislukt met de reden.

## Een back-up mislukte met "repository is already locked" {#a-backup-failed-with-repository-is-already-locked}

Dit is meestal een verweesde restic-lock die achterbleef toen de container midden in een bewerking werd bijgewerkt of herstart. BombVault herkent een aantoonbaar verweesde lock, wist haar en probeert het automatisch nog één keer. Na een herstart wacht BombVault tot zo'n lock tien minuten niet is ververst. Een restic die nog draait, ververst zijn lock elke vijf minuten, en BombVault verwijdert nooit een lock die nog wordt ververst, omdat die van een tweede BombVault kan zijn die naar hetzelfde repository schrijft. Een lock die 30 minuten niet is ververst, wordt verwijderd, wie haar ook heeft gezet. Blijft de fout, open dan de kaart **Back-ups controleren** onder **Instellingen, Verificatie** en kies **Ontgrendelen** onder **Meer** in de rij van het betrokken onderdeel om een verouderde lock met de hand te wissen. Ook **Ontgrendelen** laat een lock staan die nog wordt ververst en vertelt je wanneer dat voor het laatst gebeurde. Een echt probleem komt nog steeds aan de oppervlakte in plaats van te worden verborgen.

## Mijn off-site-kopie gebeurde niet na een back-up {#my-off-site-copy-did-not-happen-after-a-backup}

Off-site-replicatie is per ontwerp best-effort, dus een off-site-probleem laat de lokale back-up nooit mislukken. Kijk wanneer de bestemming kopieert: haar kaart **Kopiëren** zegt **Na elke back-up** of noemt een eigen planning, en **Instellingen, Planning, Off-site-kopieën per onderdeel** kan een onderdeel een eigen planning geven. Gebruik **Nu repliceren** in de rij van het onderdeel in de kaart **Offsite-kopieën** onder **Instellingen, Bestemmingen** voor een run op verzoek, en kijk naar de statuskop van het overzicht, die **Off-site-kopie loopt** zegt zolang ze loopt. Een mislukte kopie staat onder **Aandacht nodig**.

## Het certificaat van een rest-server wordt niet vertrouwd {#a-rest-servers-certificate-is-not-trusted}

restic weigert een certificaat dat het niet kent, en de fout zegt waar je het moet neerzetten. Kopieer het zelfondertekende certificaat van de rest-server, of dat van de autoriteit die het ondertekende, als PEM-bestand in de map `certs` van de datamap van BombVault (`/config/certs` in de container, `appdata/bombvault/certs` op Unraid) en test opnieuw. Een herstart is niet nodig.

## Een back-up zegt dat data buiten Host Data niet is geback-upt {#data-outside-host-data}

De container bewaart data in een map die BombVault niet via Host Data kan lezen. De back-up slaat de definitie en elke map die hij kan bereiken op, kleurt zijn regel in het activiteitenlogboek oranje en noemt de mappen die hij wegliet. Twee gevallen komen vaak voor: een ixVolume op TrueNAS onder `/mnt/.ix-apps`, en een benoemd Docker-volume onder `/var/lib/docker/volumes`, zoals het databasevolume van een compose-stack. De kaart **Back-upmappen** op het tabblad **Instellingen** van de container toont elke zo'n map met zijn pad in de container en zijn hostpad, rood gemarkeerd.

Een anoniem Docker-volume wordt niet gemeld. Docker maakt er zelf een voor een map die de image declareert en niemand koppelde, geeft het een naam van 64 hex-cijfers en bewaart het onder `/var/lib/docker/volumes`. Het bevat meestal een cache, dus de back-up blijft groen en de kaart toont het alleen, met een opmerking dat het anoniem is.

- **Je wilt die data back-uppen.** Koppel de map aan een hostpad onder Host Data, bijvoorbeeld een map in je appdata-share, en de volgende back-up neemt haar mee.
- **Je hebt het niet nodig.** Voeg een van de twee paden toe als regel in de kaart **Uitsluitpatronen** op hetzelfde tabblad: het pad in de container, zoals `/cache`, of het hostpad dat de waarschuwing noemt, zoals `/var/lib/docker/volumes/jellyfin_cache/_data`. Vanaf de volgende back-up laat de waarschuwing die map weg, en het voorbeeld onder het veld zegt waarom. Een regel voor een map erboven werkt ook, een kale naam of `/` niet.

## Een back-up zegt Uitgesteld {#deferred-backups}

De app van de container was bezig, dus zijn back-up wacht en wordt vanzelf opnieuw geprobeerd. De regel in het activiteitenlogboek zegt waarom en wanneer de volgende poging is. De kaart **Als de app bezig is** op het tabblad **Instellingen** van de container zegt hetzelfde en wanneer het wachten eindigt. Je hoeft niets te doen. Om meteen een back-up te maken, gebruik je **Nu back-up maken**: een back-up die je zelf start, wacht nooit, ook niet voor exitcode 75.

- **Je eigen controle.** Laat het **Pre-back-upcommando** met code 75 eindigen zolang de app bezig is, bijvoorbeeld `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Elke andere code die niet 0 is, laat de back-up zoals voorheen mislukken, dus een kapot script verschijnt nog steeds onder **Aandacht nodig**.
- **Plex, Jellyfin of Emby wordt nooit bevraagd.** BombVault vraagt eerst de server op het eigen adres van de container en voert, als dat niet antwoordt, curl of wget uit in de container. Een image met geen van beide kan niet via zichzelf worden bevraagd. Het logboek noemt dat dan. Met **Back-up maken als de server niet antwoordt** aan loopt zo'n back-up toch, met uit wacht hij.
- **Jellyfin of Emby weigert.** Controleer de **API-sleutel**. Die maak je in het dashboard van de server onder API keys.
- **Plex weigert.** BombVault leest het token uit `Preferences.xml` in de map die op `/config` is gemount. Een server waarop nooit is aangemeld, heeft er geen en antwoordt alleen waar hij toegang zonder toestaat.
- **Een gepauzeerde film houdt de back-up de hele nacht wachtend.** Stel **Gepauzeerde sessies negeren na (min)** in, bijvoorbeeld op 30.
- **De verkeerscontrole laat de back-up nooit door.** Verhoog **Bezig vanaf (KiB/s)** boven wat de container verstuurt terwijl niemand hem gebruikt. Direct nadat BombVault start, wacht de controle tot hij de container **Rustig gedurende (min)** minuten heeft bekeken.
- **De back-up draait nooit 's nachts.** Met een venster ingesteld komen pogingen alleen binnen het venster. Een wachttijd die zijn deadline buiten het venster bereikt, eindigt nog steeds op de deadline.

## Een grote back-up werd gedood omdat het geheugen opraakte {#a-large-backup-was-killed-for-running-out-of-memory}

Stel **Geheugenlimiet per back-up (MiB, 0 = geen)** in de kaart **Prestaties** onder **Instellingen, Systeem** in, ruim onder wat de container mag gebruiken. Dicht bij de limiet maakt restic vaker geheugen vrij, wat wat snelheid kost, zodat de back-up vertraagt in plaats van te worden gedood.

## Een herstel brak af voordat het begon {#a-restore-aborted-before-it-started}

Voordat iets wordt gestopt of verwijderd, voert een herstel een conflictcontrole vooraf uit: het controleert of het vaste IP en de gepubliceerde hostpoorten van de container vrij zijn. Houdt een andere container er al een vast, dan breekt het af met een duidelijke bruikbare melding in plaats van een half afgemaakt herstel achter te laten. Maak de conflicterende poort of het IP vrij en probeer opnieuw.

Bestaat het netwerk van de container niet, bijvoorbeeld omdat zijn TrueNAS-app is verwijderd, dan noemt de herstelcontrole het netwerk en weigert het herstel voordat het iets verandert. Installeer de app in TrueNAS opnieuw onder dezelfde naam, wat het netwerk opnieuw aanmaakt, en herstel dan zijn data.

## Een gewone export mislukte in plaats van een bestand te schrijven {#a-plain-export-failed-instead-of-writing-a-file}

Staat **Exports versleutelen met age** aan onder **Instellingen, Systeem** maar is er geen geldige ontvanger ingesteld, dan mislukt een export met een duidelijke fout in plaats van platte tekst te schrijven. Voeg een geldige ontvanger toe (een publieke age-sleutel of een publieke SSH-sleutel), of schakel versleuteling uit als je wilt dat de export platte tekst is. Zie [Gewone exports](backups.md#exports).

## Een databasedump is mislukt {#a-database-dump-failed}

Een mislukte dump laat de back-up eromheen nooit mislukken. Hij wordt vastgelegd als een eigen mislukte run, en de reden noemt wat je moet oplossen.

- **Login geweigerd.** De dump meldt zich aan met de eigen wachtwoordvariabelen van de container (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` of hun `_FILE`-versies). Controleer ze op de databasecontainer. Een `_FILE`-variabele die wijst naar een geheim dat de eigen gebruiker van de container niet kan lezen, mislukt op dezelfde manier.
- **Ontbrekende rechten.** Met een willekeurig rootwachtwoord kan de dump zich alleen als de appgebruiker aanmelden, dus hij bevat die ene database, en MySQL 8.4 en nieuwer kunnen hem helemaal weigeren. Geef de container een echt rootwachtwoord, of schakel de dump ervoor uit.
- **De systeemtabellen hebben een upgrade nodig.** MariaDB weigert te dumpen als zijn systeemtabellen van een oudere versie zijn (fout 1558). Voeg de variabele `MARIADB_AUTO_UPGRADE=1` toe en herstart de container, of voer één keer `mariadb-upgrade` erin uit.
- **Geen dumptool.** Een slanke of zelfgebouwde image zonder `pg_dump`, `mysqldump` of `mariadb-dump` kan niet worden gedumpt. Gebruik de officiële image, of schakel de dump uit.
- **Een tijdslimiet.** Een dump krijgt `DB_DUMP_MAX_HOURS` (standaard 6), de back-up eromheen krijgt `BACKUP_MAX_HOURS`, en een dump die geen voortgang meer maakt, wordt na `BACKUP_STALL_HOURS` afgekapt. De gebruikelijke oorzaak van die laatste is een lock die de applicatie vasthoudt. Verhoog de limiet die afging, of dump terwijl de applicatie rustig is.
- **De container is gepauzeerd of herstart.** De dump praat met de draaiende server. Herstart de container steeds, dan zegt zijn eigen logboek waarom.
- **Een beschadigde dump kon niet worden verwijderd.** Een dump die BombVault niet kon afmaken, wordt weer verwijderd. Mislukt die verwijdering, dan blijft de dump in de lijst staan, als beschadigd gemarkeerd, en kun je hem daar verwijderen.

## Een import is mislukt {#an-import-failed}

Een import stopt de container, verplaatst zijn datamap opzij en laat de image er een lege voor in de plaats maken. Mislukt een stap vóór de import zelf, dan wordt de oude map automatisch teruggezet. Mislukt de import, dan houdt de container de verse map en blijft de oude ernaast staan als `<data folder>.bombvault-before-import-<timestamp>`. De foutmelding van de run noemt het exacte pad.

Een datamap die een eigen dataset is, zoals op TrueNAS, kan niet worden hernoemd. De import laat de map dan staan en verplaatst de inhoud naar de bewaarde map ernaast, dus de dataset erboven heeft ruimte nodig voor die inhoud. De import controleert dat voordat hij de container stopt.

Om het met de hand terug te zetten: stop de container, hernoem de huidige datamap zodat hij uit de weg staat, hernoem de bewaarde map terug naar de oorspronkelijke naam en start de container. Verplaats voor een dataset de inhoud in plaats van de mappen te hernoemen. Op Unraid doet de bestandsbeheerder dit vanaf het tabblad Shares.

## Een back-up van een ZFS-dataset mislukte of sloeg een dataset over {#zfs-datasets}

Elk probleem draagt een redencode tussen haakjes, en de pagina [ZFS-datasets](zfs-datasets.md#reason-codes) toont ze allemaal met de oplossing. De drie meest voorkomende:

- **`snapshot-loop`**: de snapshot bereikte BombVault niet omdat Host Data nieuwe mounts niet doorgeeft. Bewerk de container, zet de Access Mode van Host Data op Read/Write - Slave en herstart BombVault.
- **`key-not-loaded`**: een versleutelde dataset waarvan de sleutel niet is geladen, wordt overgeslagen. Laad de sleutel met `zfs load-key` en mount de dataset. De volgende back-up neemt haar mee.
- **`ssh-auth`**: de server weigerde de sleutel van BombVault. **ZFS-verbinding met deze server**, geopend met **Verbinding bekijken** op de pagina Back-ups, toont de opdracht die hem autoriseert. Voer die één keer uit op de server.

## Een item blijft op "Leert N/10" {#an-entry-stays-at-learning-n10}

De meeste anomaliecontroles beginnen na 10 geslaagde back-ups van een item, en de telling begint opnieuw na **Als verwacht markeren** en nadat de selectie van het item veranderde. Een item dat niet in de planning staat, leert niet, en een container zonder appdata heeft niets om van te leren, wat zijn badge zegt. **Leervoortgang** op de pagina Anomalieën toont een ring van tien delen per item.

## Het bewaarbeleid verwijdert geen oude back-ups van één item meer {#retention-stopped-deleting-old-backups-of-one-entry}

Een openstaande kritieke anomalie houdt ze vast: de bron van het item is bijna leeg, sterk gekrompen, of één back-up sloeg het grootste deel van zijn data opnieuw op. Open de anomalie via de badge op het item. Ontbreekt er data of is ze versleuteld, herstel dan eerst vanaf de gelinkte laatste goede back-up. Bevestig daarna de anomalie, of markeer haar als verwacht als de verandering van jou kwam, en de volgende run schoont op zoals gewoonlijk. Het voorbeeld van het bewaarbeleid markeert zo'n item als bewaard. Bij een ZFS-item houdt alleen de dataset die in de anomalie wordt genoemd zijn oude back-ups, en de andere datasets van de boom worden gewoon opgeschoond.

## Handmatig opschonen zegt dat sommige items zijn bewaard {#manual-prune-says-some-entries-were-kept}

Dezelfde oorzaak: opschonen laat de oude back-ups van een item met zo'n anomalie met rust en noemt het item in zijn bericht. Al het andere wordt gewoon opgeschoond.

## Een back-up mislukt met "on no mounted pool or share" {#no-mounted-pool}

Op Unraid leeft `/mnt` in het geheugen, dus een map daar waarop geen pool is gemount, is bij de volgende herstart weg en verborgen zodra de pool terug is. BombVault weigert daar een back-up of een flash-ZIP te schrijven, ook als er in die map al een repository van een eerdere schrijfactie staat. Mount de pool en start de back-up opnieuw.

## Geschiedenisimport zegt dat een repository niet kon worden gelezen {#history-import-says-a-repository-could-not-be-read}

Na de upgrade leest BombVault de groottes van eerdere back-ups eenmalig uit elk repository. Een repository dat het op dat moment niet kon bereiken, zoals een off-site-doel dat uit stond of een share die niet gemount was, wordt geteld op de kaart **Anomalieën** onder **Instellingen, Verificatie** en eenmaal per dag opnieuw geprobeerd. De items leren intussen van nieuwe back-ups.

## De waarschuwing over schijfruimte komt niet overeen met het Unraid-dashboard {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

Op de Unraid-gebruikersshare (`/mnt/user`) is de vrije ruimte die van de hele array, niet van één schijf. Externe bestemmingen die BombVault via rclone bereikt, zoals SFTP, de Hetzner Storage Box, SMB, WebDAV of een cloudschijf, worden gemeten als de andere kant zijn vrije ruimte meldt, en een met de hand getypt `sftp:`-adres ook. S3-, B2- en REST-repository's hebben geen cijfer: hun bestemming zegt **Grootte onbekend**, en de kaart **Anomalieën** toont ze als niet gemeten.

## Een AI-assistent kan niet verbinden {#an-ai-assistant-cannot-connect}

De [MCP-serverpagina](mcp.md#troubleshooting) toont wat elke statuscode en elke weigering van het MCP-eindpunt betekent en wat je eraan kunt doen.

## De container blijft herstarten of ziet er niet gezond uit {#the-container-keeps-restarting-or-looks-unhealthy}

BombVault meldt healthy/unhealthy via zijn eigen `/api/health`. Een auto-heal-tool (zoals Autoheal) kan hem automatisch herstarten als de engine ooit vastloopt. Bekijk het containerlogboek en de kaart **Host-integratie** voor de onderliggende oorzaak.

## Een bestemming weigert op te slaan {#location-refused}

BombVault controleert een bestemming voordat het haar opslaat, en de weigering zegt in één zin waarom. De gebruikelijke:

- **"This location lies inside another repository or target, or contains one."** Een repository binnen een repository zou beide breken. De melding noemt de andere plek. Kies een andere map, of verwijder die eerst.
- **Het is een repository dat je instelde.** De bestemming is een van de rijen onder **Repository's**, of ligt erin of eromheen. Kies een andere plek, of verwijder dat repository eerst.
- **Dezelfde plek als de eigen map van een onderdeel.** Een kopie op de eigen map van een onderdeel zou bij elke run zijn back-ups op zichzelf kopiëren, dus ze wordt geweigerd. Kies een andere plek, of zie [De eigen map van een onderdeel](offsite-recovery.md#own-folder).
- **"That is not an off-site target of this domain"** kwam van een link naar een bestemming die was verwijderd. Een actuele versie wist deze bij de start en bij een import. Zie je het nog, werk BombVault dan bij.
- **Een volledig hostpad** zoals `/mnt/user/backups`. BombVault draait in een container, dus het wil het pad onder Host Data, `user/backups`, en de melding noemt het pad dat je moet invullen.

## Een externe bestemming zegt dat haar repository weg is {#repository-gone}

Een bestemming die back-ups bevatte en ineens geen repository meer heeft, krijgt niet uit zichzelf een nieuw, leeg repository, omdat dat het echte zou verbergen zodra een schijf terug is. Een rest-server waarvan de schijf niet is gemount, is de gebruikelijke oorzaak, dus mount de schijf opnieuw en voer de back-up of kopie nog eens uit. Alleen als je het oude repository met opzet hebt leeggemaakt of verwijderd, druk je op de pagina van de bestemming op **Hier een nieuw repository aanmaken**. Het begint opnieuw voor elk onderdeel van die bestemming tegelijk.

## Een cloudaanmelding is verlopen {#cloud-sign-in-expired}

De test of de volgende kopie zegt dan dat de aanmelding is verlopen. **Verbinding wijzigen** op de pagina van de bestemming meldt je opnieuw aan.

- **Box, Jottacloud en andere met refresh-tokens voor eenmalig gebruik:** BombVault bewaart het token dat rclone ververste, ook over herstarts heen en in de herstelkit. Na een herstel vanaf een oudere instellingenback-up kan het token erin zijn verlopen, dus meld je nog één keer aan. Het repository blijft zoals het is.
- **iCloud Drive:** Apple beëindigt de aanmelding na 30 dagen. Het heeft een nieuw vertrouwenstoken en nieuwe cookies uit `rclone config` nodig.
- **OneDrive:** een aanmelding met meerdere schijven vraagt welke je wilt gebruiken, dus een bestemming zonder keuze blijft onbruikbaar tot je kiest. Voor OneDrive Business en Google Drive zie de opmerking over opgeschoonde data onder [Bestemmingen](offsite-recovery.md#destinations).

## "De off-site-kopie van de laatste back-up loopt nog" {#copy-still-running}

De back-up ervoor is klaar, en zijn kopie is nog onderweg, wat een nieuwe back-up van dat onderdeel blokkeert. Wacht tot de statuskop niet meer **Off-site-kopie loopt** zegt. De knoppen die een back-up starten, zeggen hetzelfde terwijl ze wachten. De eerste kopie naar een nieuwe bestemming kan uren duren, zie het [S3-voorbeeld](offsite-recovery.md#example-s3).

## Een ZFS-replica stopt omdat het doel nieuwere snapshots heeft {#replica-target-newer}

Het doel heeft snapshots die nieuwer zijn dan de nieuwste die het met deze server deelt, bijvoorbeeld omdat de dataset hier is teruggedraaid of de kopie op het doel is gewijzigd. De kaart Replica en de runhistorie noemen de snapshot bij **Doel terugzetten naar**. Voer `zfs rollback -r` naar die snapshot uit op het doel, voor elke dataset die de run noemt, en druk op **Nu repliceren**. Zie [Hoe lang snapshots blijven](zfs-datasets.md#replica-retention).

## Een VM met Unraid-snapshots mislukt en noemt een bestand {#vm-chain}

BombVault volgt de backingketen van elke schijf en maakt een back-up van elk bestand erin. Een keten die niet tot het einde gevolgd kan worden, laat de back-up mislukken in plaats van een deel van de schijf weg te laten, en de melding noemt het bestand. Controleer of het bestand onder `/mnt` bestaat en of de snapshot nog in de VM Manager staat. Een bestand buiten Host Data kan helemaal niet worden gelezen. Zie [VM's met Unraid-snapshots](vm-backup-ssh-setup.md#unraid-snapshots).

## Off-site-kopieën wachten na een herinstallatie op je beslissing {#copies-paused}

Na **Back-ups ontdekken** of de Noodassistent zonder herstelde instellingen, of als een onderdeel eigen back-ups van vóór deze database vindt, wachten de off-site-kopieën van dat onderdeel, omdat BombVault niet meer weet welke items je wegliet. **Aandacht nodig** en de pagina van de bestemming vragen ernaar. Druk op **Als back-ups van deze server overnemen** om verder te gaan, of op **Eerst de regels controleren** om eerst naar de regels van het onderdeel te kijken. Het importeren van een instellingenbestand neemt de beslissing niet voor je. Zie [Na een herinstallatie](offsite-recovery.md#after-a-rebuild).

## Meldingen komen niet aan {#notifications-do-not-arrive}

Begin met **Test versturen** onder **Instellingen, Meldingen**. De regel onder de kaart zegt wat er gebeurde.

- **Er wordt helemaal niets verstuurd.** Controleer **Melden**. Met **Nooit** gaat er geen bericht uit, alleen de pings van een Healthchecks-URL, en met **Alleen bij mislukking** blijft een goede run stil. Met **Geplande runs samenvatten** aan stuurt een geplande run aan het einde één samenvatting.
- **Een nieuw adres.** Nadat je de server van Matrix, Gotify of e-mail wijzigt, vraagt het veld opnieuw om het token of wachtwoord, en het nieuwe adres wordt pas opgeslagen zodra je het intypt. BombVault stuurt het oude geheim niet naar een nieuwe plek.
- **Gotify** zegt dat de server het app-token weigerde: kopieer het opnieuw uit de applicatie in Gotify. De **Server-URL** is het basisadres, zonder `/message`.
- **Webhook:** controleer of het **Webhook-formaat** bij de dienst past. Een Discord-adres heeft **Discord** nodig, en ntfy heeft **ntfy** nodig.
- **Unraid-meldingen** lopen over de Host-SSH-verbinding, dus test die eerst onder **Instellingen, Integraties**. Op een ander platform blijven ze uit, en de kaart zegt dat.
- **Een e-mailmelding komt nooit.** Controleer **Versleuteling** tegen de poort, meestal **STARTTLS** op 587 of **TLS (implicit)** op 465.

Healthchecks-pings zijn hier niet van afhankelijk, zie [Healthchecks](notifications.md#healthchecks).

## De Android-app kan een server niet openen {#android-cannot-open}

Het startscherm zegt op de kaart van de server waarom:

- **Niet verbonden:** noch het adres van de server noch de relay antwoordt. Op **Geen relay** bereikt de app een server alleen op zijn eigen adres.
- **Certificaat controleren:** de server toont een ander certificaat dan het certificaat dat je vertrouwde. De app opent hem pas nadat je het nieuwe vertrouwt, dus zorg dat de wijziging van jou is.
- **Aanmelden nodig:** een server die via zijn adres is toegevoegd, vraagt om het wachtwoord. Zie ook [Een telefoon belandt op de inlogpagina](#a-phone-lands-on-the-login-page).
- **De sleutel is veranderd:** het startscherm toont de vingerafdruk van de nieuwe sleutel, en de pagina opent pas nadat je hem vertrouwt. Zie [Onderweg](android.md#away).

## Een andere gebruiker kan het lokale repository niet lezen {#repository-permissions}

Het lokale repository blijft leesbaar voor andere gebruikers, zoals een syncgebruiker via SMB. Vóór 10.4.1 konden bestanden die een opschoonronde, tag, ophaalactie, onderdeelverplaatsing, verwijdering of stackback-up schreef tot de volgende nacht onleesbaar blijven. Werk BombVault bij, en ze zijn meteen leesbaar. Het repository van de zelf-back-up blijft met opzet alleen voor root leesbaar, omdat het de instellingen van BombVault bevat.

## Zit je nog vast? {#still-stuck}

- Lees de volledige pagina's [Configuratie](configuration.md) en [Off-site en herstel](offsite-recovery.md).
- **Bugrapport** in de kaart **Help** onder **Instellingen, Info** downloadt een ZIP voor een GitHub-issue: de hostcontrole, je configuratie met elk geheim verwijderd, de recente runs, wat er hierna gepland staat en het recente logboek sinds de container voor het laatst startte. Het bevat ook de laatste dump van elke database, de ZFS-items met hun mounts, de openstaande anomalieën en hoeveel MCP-sleutels er zijn. Wachtwoorden, tokens en de rclone-configuratie worden eruit gehaald, en de bundel zegt dat in zijn eigen manifest. Het vereist een inlogwachtwoord. Kijk voor een crash die de container herstartte in `docker logs`.
- Vraag het in de [Unraid-supportthread](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Open een [GitHub-issue](https://github.com/junkerderprovinz/bombvault/issues).
