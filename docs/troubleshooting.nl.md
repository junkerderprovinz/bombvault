# Probleemoplossing

Een korte FAQ. Voor de volledige VM-via-SSH-tabel voor probleemoplossing aan de hostkant (permission-denied, host-key-verificatie, ontbrekende template-variabelen en meer), zie de [VM-back-up-via-SSH-gids](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) op GitHub.

## Er is iets niet correct aangesloten

Open `/spike` in de web-UI. De controle van hostintegratie test elke mount en CLI (Docker-socket, libvirt, restic, qemu-img, rclone) en meldt eventuele ontbrekende onderdelen. Begin hier voordat je een bug veronderstelt: een ontbrekende mount of een onbereikbare host duikt meteen op.

## Ik kan de web-UI niet bereiken

BombVault serveert HTTPS out of the box op poort `3443` (zelfondertekend certificaat), dus open `https://<jouw-unraid-ip>:3443`. Accepteer de waarschuwing over het zelfondertekende certificaat, of zet BombVault achter een reverse proxy met je eigen certificaat. Als je draait met `HTTP_ONLY=true`, serveert het in plaats daarvan platte HTTP op poort `3000` (bedoeld voor gebruik achter een TLS-terminerende proxy).

## Ik ben mijn APP_KEY kwijt

`APP_KEY` leidt het wachtwoord van de restic-repository af. Zonder deze (en zonder de herstelkit voor de encryptiesleutel) kunnen versleutelde back-ups niet worden hersteld. Daarom zeurt het Dashboard je om de herstelkit te downloaden. Zie [Off-site en herstel](offsite-recovery.md). Genereer een sleutel met `openssl rand -hex 32` en bewaar hem buiten de server voordat je op enige back-up vertrouwt.

## VM-back-up wil geen verbinding maken

VM-back-up praat met libvirt via SSH, nooit een mount.

- Bevestig dat SSH is ingeschakeld op de host en dat BombVaults publieke sleutel geautoriseerd is in `/root/.ssh/authorized_keys` (Instellingen, Systeem, VM-back-up via SSH toont de sleutel en een knop **Verbinding testen**).
- Stel op een custom `br0.x`-netwerk `LIBVIRT_HOST` in op je Unraid LAN-IP (de container kan de host daar niet via `host.docker.internal` bereiken). Schakel **Instellingen, Docker, Host access to custom networks** in.
- Als je de SSH-poort van Unraid hebt gewijzigd, stel `LIBVIRT_SSH_PORT` overeenkomstig in.
- Volledige stap-voor-stap-diagnose (bereikbaarheidstest, VLAN-routing, `Permission denied (publickey)`, `Host key verification failed`) staat in de [VM-back-up-via-SSH-gids](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Een live VM-snapshot is niet gedraaid

Live snapshots hebben de qemu guest agent geïnstalleerd in de VM nodig en de schijf op `/mnt/cache` (of `/mnt/diskX`), niet `/mnt/user`. Op een uitgeschakelde VM valt live automatisch terug op net afsluiten. Een nette back-up sluit de VM af, maakt een back-up van de schijven en herstart hem daarna, dus hij is altijd consistent.

## Een back-up mislukte met "repository is already locked"

Dit is meestal een verweesde restic-lock die achterblijft wanneer de container werd bijgewerkt of herstart midden in een operatie. BombVault detecteert een aantoonbaar verweesde lock, wist hem geforceerd en probeert automatisch één keer opnieuw. Als het aanhoudt, gebruik **Instellingen, Integriteit en onderhoud, Ontgrendelen** voor het betrokken domein om een verouderde lock met de hand te wissen. Een echt probleem komt nog steeds boven in plaats van verborgen te worden.

## Mijn off-site kopie is niet gemaakt na een back-up

Off-site replicatie is opzettelijk best-effort, dus een off-site hapering laat de lokale back-up nooit mislukken. Controleer de off-site planning voor dat domein (Instellingen, Planningen): een lege planning repliceert na elke lokale back-up, terwijl een cadans minder vaak stuurt. Gebruik **Nu repliceren** op het tabblad Off-site voor een run op aanvraag, en let op de replicatie-indicator op het Dashboard.

## Een herstel brak af voordat het begon

Voordat er iets wordt gestopt of verwijderd, draait herstel een pre-flight conflictcontrole: het verifieert dat het statische IP en de gepubliceerde hostpoorten van de container vrij zijn. Als een andere container er al een vasthoudt, breekt het af met een duidelijke, bruikbare melding in plaats van een half afgemaakt herstel achter te laten. Maak de conflicterende poort of IP vrij en probeer opnieuw.

## Een platte export mislukte in plaats van een bestand te schrijven

Als age-versleuteling aan is (Instellingen) maar er geen geldige ontvanger is ingesteld, mislukt een export met een duidelijke fout in plaats van platte tekst te schrijven. Voeg een geldige ontvanger toe (een age-publieke sleutel of een SSH-publieke sleutel), of zet versleuteling uit als je bedoelt dat de export platte tekst is. Zie [Functies](features.md).

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

Met de hand terugzetten: stop de container, hernoem de huidige datamap uit de weg, hernoem de bewaarde map terug naar de oorspronkelijke naam en start de container. Op Unraid doet de bestandsbeheerder op het tabblad Shares dit.

## Een back-up van een ZFS-dataset mislukte of sloeg een dataset over {#zfs-datasets}

Elk probleem heeft een redencode tussen vierkante haken, en de pagina [ZFS-datasets](zfs-datasets.md#reason-codes) noemt ze allemaal met de oplossing. De drie meest voorkomende:

- **`snapshot-loop`**: de snapshot bereikte BombVault niet, omdat Host Data nieuwe koppelingen niet doorgeeft. Bewerk de container, zet de Access Mode van Host Data op Read/Write - Slave en herstart BombVault.
- **`key-not-loaded`**: een versleutelde dataset waarvan de sleutel niet geladen is, wordt overgeslagen. Laad de sleutel met `zfs load-key` en koppel de dataset aan; de volgende back-up neemt hem mee.
- **`ssh-auth`**: de server weigerde de sleutel van BombVault. De verbindingskaart op de ZFS-pagina toont de opdracht die hem toestaat; voer die één keer uit op de server.

## Een item blijft op "Leert N/10" staan

De meeste anomaliecontroles beginnen na 10 geslaagde back-ups van een item, en de telling begint opnieuw na **Als verwacht markeren** en nadat de selectie van het item is gewijzigd. Een item zonder planning leert niet, en een container zonder appdata heeft niets om van te leren, wat zijn badge ook zegt.

## Retentie verwijdert geen oude back-ups meer van één item

Een open kritieke anomalie houdt ze vast: de bron van het item is bijna leeg, sterk gekrompen, of een back-up heeft het grootste deel van de data opnieuw opgeslagen. Open de anomalie via de badge bij het item. Als er data ontbreekt of versleuteld is, herstel dan eerst vanaf de gelinkte laatste goede back-up. Bevestig daarna de anomalie, of markeer haar als verwacht als de wijziging van jou kwam, en de volgende run schoont weer gewoon op. De retentievoorvertoning markeert zo'n item als bewaard. Bij een ZFS-item houdt alleen de dataset die de anomalie noemt zijn oude back-ups; de andere datasets van de boom worden gewoon opgeschoond.

## Handmatig opschonen meldt dat sommige items zijn bewaard

Dezelfde oorzaak: opschonen laat de oude back-ups van een item met zo'n anomalie met rust en noemt het item in zijn melding. Al het andere wordt gewoon opgeschoond.

## De geschiedenisimport meldt dat een repository niet gelezen kon worden

Na de update leest BombVault één keer de groottes van eerdere back-ups uit elke repository. Een repository die op dat moment niet bereikbaar was, zoals een uitgevallen off-site-doel of een share die niet gekoppeld was, staat op de kaart **Anomalieën** onder **Instellingen, Integriteit** en wordt één keer per dag opnieuw geprobeerd. Zijn items leren intussen van nieuwe back-ups.

## De waarschuwing over schijfruimte klopt niet met het Unraid-dashboard

Op de Unraid-gebruikersshare (`/mnt/user`) is de vrije ruimte die van de hele array, niet van één schijf. Externe repositories worden alleen gemeten via rclone-remotes die hun vrije ruimte melden; S3-, B2-, REST- en SFTP-repositories hebben geen waarde en staan als niet gemeten op de kaart **Anomalieën**.

## Een AI-assistent krijgt geen verbinding

De pagina [MCP-server](mcp.md#troubleshooting) zet op een rij wat elke statuscode en elke weigering van het MCP-eindpunt betekent en wat je eraan kunt doen.

## De container blijft herstarten of ziet er unhealthy uit

BombVault meldt healthy/unhealthy vanuit zijn eigen `/api/health`. Een auto-heal-tool (zoals Autoheal) kan hem automatisch herstarten als de engine ooit vastloopt. Controleer het containerlog en het `/spike`-rapport voor de onderliggende oorzaak.

## Nog steeds vast?

- Lees de volledige pagina's [Configuratie](configuration.md) en [Off-site en herstel](offsite-recovery.md).
- Vraag het in de [Unraid-supportthread](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Open een [GitHub-issue](https://github.com/junkerderprovinz/bombvault/issues).
