# Probleemoplossing

Een korte FAQ. Voor de volledige VM-via-SSH-tabel voor probleemoplossing aan de hostkant (permission-denied, host-key-verificatie, ontbrekende template-variabelen en meer), zie de [VM-back-up-via-SSH-gids](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) op GitHub.

## Er is iets niet correct aangesloten

Voeg de kaart **Host-integratie** toe aan het overzicht onder **Aanpassen**. De controle van hostintegratie test elke mount en CLI (Docker-socket, libvirt, restic, qemu-img, rclone) en meldt eventuele ontbrekende onderdelen, en **Nu controleren** draait haar opnieuw. Begin hier voordat je een bug veronderstelt: een ontbrekende mount of een onbereikbare host duikt meteen op.

## Ik kan de web-UI niet bereiken

BombVault serveert HTTPS out of the box op poort `3443` (zelfondertekend certificaat), dus open `https://<jouw-unraid-ip>:3443`. Accepteer de waarschuwing over het zelfondertekende certificaat, of zet BombVault achter een reverse proxy met je eigen certificaat. Als je draait met `HTTP_ONLY=true`, serveert het in plaats daarvan platte HTTP op poort `3000` (bedoeld voor gebruik achter een TLS-terminerende proxy).

## Ik ben mijn APP_KEY kwijt

`APP_KEY` leidt het wachtwoord van de restic-repository af. Zonder deze, zonder de herstelkit en zonder een bewaarkopie van de sleutel in je groep kunnen versleutelde back-ups niet worden hersteld. Daarom toont het overzicht **Bewaar je herstelkit** totdat je hem hebt gedownload. Zie [Off-site en herstel](offsite-recovery.md#recovery-kit). Genereer een sleutel met `openssl rand -hex 32` en bewaar hem buiten de server voordat je op enige back-up vertrouwt.

## Ik ben het inlogwachtwoord vergeten

Kies **Wachtwoord vergeten?** op de loginpagina. BombVault schrijft een code van 12 tekens naar het log van zijn container (op Unraid: tabblad Docker, BombVault-pictogram, **Logs**). Typ die binnen 10 minuten samen met een nieuw wachtwoord. De reset zet ook tweefactorauthenticatie uit, verwijdert elke passkey, trekt elk API-token, elke MCP-sleutel en elke OAuth-aanmelding in, beëindigt alle andere sessies en schakelt telefoonkoppeling uit. Voeg je passkeys opnieuw toe onder **Instellingen, Beveiliging**, schakel de telefoonkoppeling daar weer in en maak nieuwe tokens en sleutels aan waar je ze nog nodig hebt. Zie [Beveiligingsmodel](configuration.md#login).

## Back-ups zijn gestopt en melden dat de sleutel niet meer te openen is

Deze server heeft in de Noodassistent de back-upsleutel van een andere server overgenomen, en daarna is `APP_KEY` gewijzigd. De overgenomen sleutel is verzegeld met de oude APP_KEY, dus BombVault kan hem niet openen en maakt liever geen back-ups dan dat het met de verkeerde sleutel schrijft. **Aandacht nodig** in het overzicht noemt het. Zet APP_KEY terug naar wat hij was, of upload de herstelkit opnieuw op de pagina Herstel onder **Noodgeval**.

## Een telefoon belandt op de loginpagina

Een gekoppelde telefoon krijgt zijn sessie via de groep alleen zolang **Telefoons kunnen koppelen met de twaalf woorden van de groep** aanstaat onder **Instellingen, Beveiliging**. Een wachtwoordreset zet het uit. Een app die ouder is dan de server krijgt ook geen sessie totdat hij is bijgewerkt. Een server die je via zijn adres hebt toegevoegd vraagt altijd om het wachtwoord. Een telefoon die van de pagina Instanties is verwijderd, komt uitgeschakeld terug wanneer zijn app weer opent; zet **Deze instantie bevragen** aan onder **Bewaking** in zijn details om hem binnen te laten.

## VM-back-up wil geen verbinding maken

VM-back-up praat met libvirt via SSH, nooit een mount.

- Bevestig dat SSH is ingeschakeld op de host en dat BombVaults publieke sleutel geautoriseerd is in `/root/.ssh/authorized_keys` (Instellingen, Integraties, Host-SSH toont de sleutel en een knop **Verbinding testen**).
- Stel op een custom `br0.x`-netwerk `LIBVIRT_HOST` in op je Unraid LAN-IP (de container kan de host daar niet via `host.docker.internal` bereiken). Schakel **Settings, Docker, Host access to custom networks** in.
- Als je de SSH-poort van Unraid hebt gewijzigd, stel `LIBVIRT_SSH_PORT` overeenkomstig in.
- Stel op TrueNAS `LIBVIRT_URI` in met de eigen libvirt-socket van TrueNAS, geef de app de capability `SETPCAP` en voeg de sleutel toe aan root onder **Credentials, Users** (zie [TrueNAS SCALE](getting-started.md#truenas)).
- Volledige stap-voor-stap-diagnose (bereikbaarheidstest, VLAN-routing, `Permission denied (publickey)`, `Host key verification failed`) staat in de [VM-back-up-via-SSH-gids](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Een live VM-snapshot is niet gedraaid

Live snapshots hebben de qemu guest agent geïnstalleerd in de VM nodig en de schijf op `/mnt/cache` (of `/mnt/diskX`), niet `/mnt/user`. Op een uitgeschakelde VM valt live automatisch terug op net afsluiten. Een nette back-up sluit de VM af, maakt een back-up van de schijven en herstart hem daarna, dus hij is altijd consistent. Hij wacht tot tien minuten tot de gast is afgesloten en zet hem nooit geforceerd uit, zodat een Windows-gast die updates installeert niet halverwege wordt onderbroken. Een gast die daarna nog draait, blijft met rust, en die back-up mislukt met de reden.

## Een back-up mislukte met "repository is already locked"

Dit is meestal een verweesde restic-lock die achterblijft wanneer de container werd bijgewerkt of herstart midden in een operatie. BombVault detecteert een aantoonbaar verweesde lock, wist hem geforceerd en probeert automatisch één keer opnieuw. Als het aanhoudt, open de kaart **Back-ups controleren** onder **Instellingen, Verificatie** en kies **Ontgrendelen** onder **Meer** in de rij van het betrokken onderdeel om een verouderde lock met de hand te wissen. Een echt probleem komt nog steeds boven in plaats van verborgen te worden. Na een herstart wacht BombVault tot zo'n lock tien minuten niet is vernieuwd. Een restic die nog draait, bijvoorbeeld in een tweede BombVault op dezelfde repository, vernieuwt zijn lock elke vijf minuten.

## Mijn off-site kopie is niet gemaakt na een back-up

Off-site replicatie is opzettelijk best-effort, dus een off-site hapering laat de lokale back-up nooit mislukken. Controleer wanneer de bestemming kopieert: de kaart **Kopiëren** zegt **Na elke back-up** of noemt een eigen plan, en **Instellingen, Planning, Off-site-kopieën per onderdeel** kan een onderdeel een eigen plan geven. Gebruik **Nu repliceren** in de kaart **Off-site kopie** van het onderdeel onder **Instellingen, Bestemmingen** voor een run op aanvraag, en let op de statuskop van het overzicht, die **Off-site-kopie loopt** zegt zolang het loopt. Een mislukte kopie staat onder **Aandacht nodig**.

## Het certificaat van een rest-server wordt niet vertrouwd

restic weigert een certificaat dat het niet kent, en de foutmelding zegt waar het moet komen. Kopieer het zelfondertekende certificaat van de rest-server, of dat van de certificaatautoriteit die het heeft ondertekend, als PEM-bestand naar de map `certs` in de datamap van BombVault (`/config/certs` in de container, `appdata/bombvault/certs` in Unraid) en test opnieuw. Een herstart is niet nodig.

## Een back-up meldt dat gegevens buiten Host Data niet zijn geback-upt {#data-outside-host-data}

De container bewaart gegevens in een map die BombVault niet via Host Data kan lezen. De back-up slaat de definitie op en elke map die hij kan bereiken, kleurt zijn regel in het activiteitenlogboek oranje en noemt de mappen die hij heeft overgeslagen. Twee gevallen komen vaak voor: een ixVolume op TrueNAS onder `/mnt/.ix-apps` en een anoniem Docker-volume onder `/var/lib/docker/volumes`, dat Docker zelf aanmaakt voor een map die het image declareert en die niemand heeft gekoppeld, zoals de gegevens van een monitoringagent of de cache van een mediaserver.

- **Je wilt die gegevens geback-upt hebben.** Koppel de map aan een hostpad onder Host Data, bijvoorbeeld een map in je appdata-share, dan neemt de volgende back-up hem mee.
- **Je hebt ze niet nodig.** Voeg het pad van de map binnen de container toe, zoals `/var/lib/beszel-agent` of `/cache`, als regel in de kaart **Uitsluitpatronen** op het tabblad **Instellingen** van de container. Vanaf de volgende back-up laat de waarschuwing die map weg, en het voorbeeld onder het veld zegt waarom. Een regel voor een map erboven werkt ook, een kale naam niet.

## Een grote back-up werd afgebroken wegens geheugengebrek

Stel **Geheugenlimiet per back-up (MiB, 0 = geen)** in de kaart **Prestaties** onder **Instellingen, Systeem** ruim onder wat de container mag gebruiken. Dicht bij de limiet geeft restic vaker geheugen vrij. Dat kost wat snelheid, maar de back-up wordt dan trager in plaats van te worden afgebroken.

## Een herstel brak af voordat het begon

Voordat er iets wordt gestopt of verwijderd, draait herstel een pre-flight conflictcontrole: het verifieert dat het statische IP en de gepubliceerde hostpoorten van de container vrij zijn. Als een andere container er al een vasthoudt, breekt het af met een duidelijke, bruikbare melding in plaats van een half afgemaakt herstel achter te laten. Maak de conflicterende poort of IP vrij en probeer opnieuw.

Als het netwerk van de container niet bestaat, bijvoorbeeld omdat zijn TrueNAS-app is verwijderd, noemt de controle vóór het herstel het netwerk en weigert het herstel voordat het iets wijzigt. Installeer de app opnieuw in TrueNAS onder dezelfde naam, wat het netwerk weer aanmaakt, en herstel daarna zijn data.

## Een platte export mislukte in plaats van een bestand te schrijven

Als **Exports versleutelen met age** aan staat onder **Instellingen, Systeem** maar er geen geldige ontvanger is ingesteld, mislukt een export met een duidelijke fout in plaats van platte tekst te schrijven. Voeg een geldige ontvanger toe (een age-publieke sleutel of een SSH-publieke sleutel), of zet versleuteling uit als je bedoelt dat de export platte tekst is. Zie [Functies](features.md).

## Een databasedump is mislukt

Een mislukte dump laat de back-up eromheen nooit mislukken; hij wordt als een eigen mislukte run vastgelegd, en de reden noemt wat je moet verhelpen.

- **Aanmelden geweigerd.** De dump meldt zich aan met de wachtwoordvariabelen van de container zelf (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` of hun `_FILE`-versies). Controleer ze op de databasecontainer. Een `_FILE`-variabele die naar een secret wijst dat de gebruiker van de container niet mag lezen, mislukt op dezelfde manier.
- **Ontbrekende rechten.** Met een willekeurig root-wachtwoord kan de dump alleen als de app-gebruiker inloggen, zodat hij alleen die ene database bevat, en MySQL 8.4 en nieuwer kan hem helemaal weigeren. Geef de container een echt root-wachtwoord, of zet zijn dump uit.
- **De systeemtabellen moeten worden bijgewerkt.** MariaDB weigert te dumpen wanneer zijn systeemtabellen uit een oudere versie komen (fout 1558). Voeg de variabele `MARIADB_AUTO_UPGRADE=1` toe en herstart de container, of draai `mariadb-upgrade` er één keer in.
- **Geen dumpgereedschap.** Een slanke of zelfgebouwde image zonder `pg_dump`, `mysqldump` of `mariadb-dump` kan niet gedumpt worden. Gebruik de officiële image, of zet de dump uit.
- **Een tijdslimiet.** Een dump krijgt `DB_DUMP_MAX_HOURS` (standaard 6), de back-up eromheen krijgt `BACKUP_MAX_HOURS`, en een dump die niet meer vordert wordt na `BACKUP_STALL_HOURS` afgekapt. Achter dat laatste zit meestal een lock die de applicatie vasthoudt. Verhoog de limiet die aansloeg, of dump terwijl de applicatie rustig is.
- **De container is gepauzeerd of herstart.** De dump praat met de draaiende server. Als de container steeds opnieuw start, zegt zijn eigen log waarom.
- **Een beschadigde dump kon niet worden verwijderd.** Een dump die BombVault niet kon afmaken, wordt weer verwijderd. Lukt dat verwijderen niet, dan blijft de dump als beschadigd in de lijst staan en kun je hem daar verwijderen.

## Een import is mislukt

Een import stopt de container, zet zijn datamap opzij en laat de image er een lege voor in de plaats maken. Mislukt een stap vóór de import zelf, dan wordt de oude map vanzelf teruggezet. Mislukt de import, dan houdt de container de verse map en blijft de oude ernaast staan als `<datamap>.bombvault-before-import-<tijdstempel>`; de foutmelding van de run noemt het exacte pad.

Een datamap die een eigen dataset is, zoals op TrueNAS, kan niet worden hernoemd. De import laat de map dan op zijn plaats en verplaatst de inhoud naar de bewaarde map ernaast, dus de dataset erboven heeft daar ruimte voor nodig. De import controleert dat voordat hij de container stopt.

Met de hand terugzetten: stop de container, hernoem de huidige datamap uit de weg, hernoem de bewaarde map terug naar de oorspronkelijke naam en start de container. Bij een dataset verplaats je de inhoud in plaats van de mappen te hernoemen. Op Unraid doet de bestandsbeheerder op het tabblad Shares dit.

## Een back-up van een ZFS-dataset mislukte of sloeg een dataset over {#zfs-datasets}

Elk probleem heeft een redencode tussen vierkante haken, en de pagina [ZFS-datasets](zfs-datasets.md#reason-codes) noemt ze allemaal met de oplossing. De drie meest voorkomende:

- **`snapshot-loop`**: de snapshot bereikte BombVault niet, omdat Host Data nieuwe koppelingen niet doorgeeft. Bewerk de container, zet de Access Mode van Host Data op Read/Write - Slave en herstart BombVault.
- **`key-not-loaded`**: een versleutelde dataset waarvan de sleutel niet geladen is, wordt overgeslagen. Laad de sleutel met `zfs load-key` en koppel de dataset aan; de volgende back-up neemt hem mee.
- **`ssh-auth`**: de server weigerde de sleutel van BombVault. **ZFS-verbinding met deze server**, te openen met **Verbinding bekijken** op de pagina Back-ups, toont de opdracht die hem toestaat; voer die één keer uit op de server.

## Een item blijft op "Leert N/10" staan

De meeste anomaliecontroles beginnen na 10 geslaagde back-ups van een item, en de telling begint opnieuw na **Als verwacht markeren** en nadat de selectie van het item is gewijzigd. Een item zonder planning leert niet, en een container zonder appdata heeft niets om van te leren, wat zijn badge ook zegt. **Leervoortgang** op de pagina Anomalieën toont per item een ring van tien delen.

## Retentie verwijdert geen oude back-ups meer van één item

Een open kritieke anomalie houdt ze vast: de bron van het item is bijna leeg, sterk gekrompen, of een back-up heeft het grootste deel van de data opnieuw opgeslagen. Open de anomalie via de badge bij het item. Als er data ontbreekt of versleuteld is, herstel dan eerst vanaf de gelinkte laatste goede back-up. Bevestig daarna de anomalie, of markeer haar als verwacht als de wijziging van jou kwam, en de volgende run schoont weer gewoon op. De retentievoorvertoning markeert zo'n item als bewaard. Bij een ZFS-item houdt alleen de dataset die de anomalie noemt zijn oude back-ups; de andere datasets van de boom worden gewoon opgeschoond.

## Handmatig opschonen meldt dat sommige items zijn bewaard

Dezelfde oorzaak: opschonen laat de oude back-ups van een item met zo'n anomalie met rust en noemt het item in zijn melding. Al het andere wordt gewoon opgeschoond.

## De geschiedenisimport meldt dat een repository niet gelezen kon worden

Na de update leest BombVault één keer de groottes van eerdere back-ups uit elke repository. Een repository die op dat moment niet bereikbaar was, zoals een uitgevallen off-site-bestemming of een share die niet gekoppeld was, staat op de kaart **Anomalieën** onder **Instellingen, Verificatie** en wordt één keer per dag opnieuw geprobeerd. Zijn items leren intussen van nieuwe back-ups.

## De waarschuwing over schijfruimte klopt niet met het Unraid-dashboard

Op de Unraid-gebruikersshare (`/mnt/user`) is de vrije ruimte die van de hele array, niet van één schijf. Externe repositories worden alleen gemeten via rclone-remotes die hun vrije ruimte melden; S3-, B2-, REST- en SFTP-repositories hebben geen waarde en staan als niet gemeten op de kaart **Anomalieën**.

## Een AI-assistent krijgt geen verbinding

De pagina [MCP-server](mcp.md#troubleshooting) zet op een rij wat elke statuscode en elke weigering van het MCP-eindpunt betekent en wat je eraan kunt doen.

## De container blijft herstarten of ziet er unhealthy uit

BombVault meldt healthy/unhealthy vanuit zijn eigen `/api/health`. Een auto-heal-tool (zoals Autoheal) kan hem automatisch herstarten als de engine ooit vastloopt. Controleer het containerlog en de kaart **Host-integratie** voor de onderliggende oorzaak.

## Nog steeds vast?

- Lees de volledige pagina's [Configuratie](configuration.md) en [Off-site en herstel](offsite-recovery.md).
- Vraag het in de [Unraid-supportthread](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Open een [GitHub-issue](https://github.com/junkerderprovinz/bombvault/issues).
