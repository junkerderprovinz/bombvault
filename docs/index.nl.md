# BombVault

**Je Unraid-data, veilig opgeborgen in een kluis. Laat een back-up vallen. Blaas een herstel af.**

![Het overzicht: de statuskop, wat je aandacht nodig heeft, de bescherming van elk onderdeel, de bestemmingen en de geschiedenis.](assets/screenshots/overview/home-dark.png#only-dark)
![Het overzicht: de statuskop, wat je aandacht nodig heeft, de bescherming van elk onderdeel, de bestemmingen en de geschiedenis.](assets/screenshots/overview/home-light.png#only-light)

BombVault is een self-hosted webapp voor **back-up en volledig noodherstel** op Unraid en andere Docker-hosts, zoals TrueNAS SCALE, ZimaOS of een gewone Linux-machine. Het maakt back-ups van Docker-containers, KVM/libvirt-VM's, mappen, ZFS-datasets en de Unraid-flash. Het draait als een enkele multi-arch Docker-container, geeft je een webinterface in een donker, licht of systeemthema, en beheert de hele levenscyclus: back-uppen, plannen, verifiëren en herstellen.

Herstel gaat automatisch. Op Unraid verschijnen containers weer in het Docker-tabblad precies zoals voorheen, en VM's worden opnieuw gedefinieerd in de VM Manager met hun schijven en UEFI NVRAM opnieuw gekoppeld. Geen handmatige herinstallatie, geen herconfiguratie, geen gedoe. Op TrueNAS draait een container die naast het origineel is hersteld als een gewone Docker-container die het scherm Apps niet toont.

Aangedreven door [restic](https://restic.net), zodat elke back-up standaard gededupliceerd, incrementeel en versleuteld is.

!!! note "Bewaar je APP_KEY veilig"
    BombVault leidt het wachtwoord van het restic-repository af uit een geheim van 32 bytes genaamd `APP_KEY`. Raak je het kwijt, samen met de herstelkit, dan zijn versleutelde back-ups onherstelbaar. Genereer er een met `openssl rand -hex 32` en bewaar het ergens veilig. Zie [Configuratie](configuration.md).

## Wat BombVault beschermt {#what-bombvault-protects}

Elke container, VM, mappenset, de flash, elke ZFS-dataset en de eigen instellingen van BombVault zijn een **item** met een eigen pagina. Items van dezelfde soort vormen samen een **onderdeel**, dat je inschakelt onder **Instellingen, Algemeen**.

| Onderdeel | Wat wordt opgeslagen |
|---|---|
| **Containers** | Appdata-map plus de containerdefinitie (image, env-variabelen, poorten, labels, volumes). |
| **VM's** | VM-schijfimage(s), de XML-definitie en UEFI NVRAM, geback-upt via SSH (geen libvirt-mount). |
| **Flash** | De hele USB-flash (`/boot`): OS, licentie, array-configuratie, shares, netwerk- en plugin-configuratie. |
| **Zelf-back-up** | BombVaults eigen `/config`: de instellingendatabase, de inloggegevens van de bestemmingen, het libvirt SSH-sleutelpaar, de sleutel en de vastgezette hostsleutels van de ZFS-servers en de certificaten. |
| **Mappen** | Elke map op de server als mappenset, elk met optionele uitsluitpatronen. |
| **ZFS-datasets** | Een dataset met alle datasets eronder, gelezen uit één ZFS-snapshot en opgeslagen als een map. Zie [ZFS-datasets](zfs-datasets.md). |

## Herstel is de ster {#restore-is-the-star}

Nadat de data is teruggekopieerd uit de restic-snapshot, speelt BombVault de opgeslagen containerdefinitie opnieuw af tegen de Docker-API, zodat de container weer in het Unraid Docker-tabblad verschijnt alsof hij er altijd is geweest (dezelfde image, dezelfde instellingen, dezelfde poorttoewijzingen). VM's krijgen hun XML opnieuw gedefinieerd via SSH en hun schijven en UEFI NVRAM opnieuw gekoppeld, zelfs nadat de VM was verwijderd.

Eén venster herstelt elke soort. Standaard kiest het **Naast het origineel**: de back-up komt als kopie terug en het origineel blijft zoals het is. **Origineel vervangen** maakt eerst een back-up van de huidige staat, zodat een herstel ongedaan te maken is. Een map of een nieuwe ZFS-dataset zijn de andere doelen.

Wanneer een back-up afhankelijke containers stopt, komen ze in de juiste volgorde terug: BombVault herstart ze in hun Compose-volgorde `depends_on` en wacht tot elk healthy meldt voordat het de containers start die ervan afhangen, zodat niets vooruit racet op een database of gateway die nog niet online is. Zie [Herstel](restore.md) en [Opdrachten en andere containers](backups.md#commands).

## Hoe het werkt {#how-it-works}

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + sign-ins (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault gebruikt de Docker-socket om containers te stoppen vóór een back-up en om ze na een herstel opnieuw aan te maken. Voor VM's draait het `virsh` op de host via SSH (`qemu+ssh://`) om een domein netjes af te sluiten of een live-snapshot te maken. Het mount nooit een libvirt-pad, dus het kan de VM Manager op de host niet in de weg zitten.

BombVault is de orkestratie- en UI-laag, niet de opslagmotor. Alle echte dataverplaatsing loopt via restic.

## Snel starten {#quick-start}

Nieuw hier? Ga naar **[Aan de slag](getting-started.md)** om BombVault te installeren en je eerste back-up te maken, en stel daarna **[Off-site en herstel](offsite-recovery.md)** in. De **[uitgewerkte voorbeelden](guides.md)** lopen de gebruikelijke klussen van begin tot eind langs, de pagina **[Instellingen](settings.md)** toont elke tegel, en **[Functies](features.md)** is de korte catalogus van alles wat BombVault doet.

Elk onderdeel kan naar meerdere bestemmingen tegelijk kopiëren, een gekoppelde BombVault kan je off-site-kopieën bewaren op een server die verwijderen weigert, en de **Noodassistent** brengt een verloren server terug vanuit zijn herstelkit, via je groep of met de ingetypte sleutel. Om je configuratie naar een nieuwe machine mee te nemen, gebruik je de kaart **Instellingen exporteren / importeren**. Zie [Off-site en herstel](offsite-recovery.md) en [Configuratie](configuration.md#portable-settings-export-and-import).

De **[Android-app](android.md)** zet elke server van je groep op je telefoon, met het activiteitenlogboek van allemaal op één scherm.

De documentatie is er in 26 talen en de interface in 42. Pagina's die nog geen vertaling hebben, verschijnen in het Engels.

## Woorden die hier worden gebruikt {#glossary}

Een paar woorden hebben in BombVault en op deze pagina's een vaste betekenis.

| Woord | Betekenis |
|---|---|
| **Item** | Eén ding dat BombVault back-upt: een container, een VM, een mappenset, de flash, een ZFS-dataset met de datasets eronder, of de eigen instellingen. Elk heeft een eigen pagina. |
| **Onderdeel** | Alle items van één soort, zoals Containers. Je schakelt onderdelen in onder **Instellingen, Algemeen**. |
| **Bestemming** | Een plek waar back-ups naartoe gaan: een map op deze server, een share, een eigen server of een clouddienst. Zie [Bestemmingen](offsite-recovery.md#destinations). |
| **Lokaal** | De bestemming op deze server die de mappen van de onderdelen bevat. |
| **Eigen map** | Het repository waarnaar een onderdeel zijn back-ups eerst schrijft. Standaard staat het op deze server en het kan in een bestemming liggen. Zie [De eigen map van een onderdeel](offsite-recovery.md#own-folder). |
| **Kopie** | Een back-up die met `restic copy` uit de eigen map naar een andere bestemming is gekopieerd. |
| **Direct repository** | Een repository op een off-site-plek waar items rechtstreeks naartoe back-uppen, zonder lokale kopie ertussen. Zie [Directe repository's](offsite-recovery.md#direct-repositories). |
| **Inloggegevens** | De opgeslagen sleutel of het wachtwoord voor een S3-dienst of een rest-server. Eén ervan is de standaard. Zie [Inloggegevens](offsite-recovery.md#sign-ins). |
| **Plaatsing** | Welke bestemmingen de back-ups van een item krijgen, ingesteld op het item of als standaard van zijn onderdeel. Zie [Plaatsing per item](offsite-recovery.md#placement). |
| **Replica** | Een blokgetrouwe kopie van een ZFS-item op een tweede ZFS-server, gemaakt met `zfs send`. Ze staat naast de back-up en vervangt hem niet. Zie [Replica](zfs-datasets.md#replica). |
| **Rol** | Iets wat een instantie van je groep voor een andere doet: Ontvanger, Ophaler, ZFS-server of Sleutelbewaarder. Zie [Instanties en rollen](offsite-recovery.md#roles). |

## Dankwoord {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** van [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) gaf BombVault zijn beginidee: back-up met één klik en automatische herinstallatie van Docker-containers. BombVault is een aparte implementatie op Go en restic die het idee uitbreidt naar VM's, de flash en meer.
- **[restic](https://restic.net/)** is de snelle, veilige, deduplicerende back-upmotor die BombVault aanstuurt.
- **[rclone](https://rclone.org/)** levert de cloudbackends.
- De meeste symbolen op knoppen komen uit de gratis set Core Solid van **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [bron](https://github.com/webalys-hq/streamline-vectors)). De andere komen uit Font Awesome Free, Material Design Icons, Simple Icons en Tabler Icons, of zijn speciaal voor het project getekend.

## Licentie {#license}

Copyright (C) 2026 Junker der Provinz. BombVault is vrije software onder de **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Je mag het draaien, bestuderen, delen en wijzigen. Verspreid je het, of draai je een gewijzigde versie als netwerkdienst, dan moet je je broncode onder dezelfde licentie publiceren en de bestaande copyright- en naamsvermeldingen behouden.

De naam en de huisstijl zijn niet gelicentieerd. De AGPL dekt alleen de broncode: "BombVault", het logo en de huisstijl blijven voorbehouden, dus een fork moet een eigen naam en huisstijl gebruiken en mag zich niet voordoen als BombVault.

## Links {#links}

- **Broncode:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid-supportthread:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issues:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Controle op het niveau van root over de host"
    Via de Docker-socket kan BombVault containers stoppen, verwijderen en opnieuw aanmaken en appdata lezen en schrijven, en voor VM-back-up logt het via SSH in op de host om `virsh` uit te voeren. Wie bij de webinterface kan, heeft in feite root op de host. Draai BombVault alleen op een vertrouwd netwerk dat niet blootstaat, en stel een inlogwachtwoord in onder **Instellingen, Beveiliging**. De herstelkit, bewaarkopieën van de sleutel, de flash-ZIP, het exporteren van een flashback-up of zelf-back-up en het verwijderen van de back-ups bij een bestemming hebben er een nodig. Zie [Configuratie](configuration.md#security-model) voor het volledige beveiligingsmodel.
