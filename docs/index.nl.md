# BombVault

**Je Unraid-data, veilig opgeborgen in een kluis. Laat een back-up vallen. Blaas een herstel af.**

BombVault is een self-hosted, Unraid-native webapp voor **back-up en volledig noodherstel** van je Docker-containers en KVM/libvirt-VM's. Het draait als een enkele multi-arch Docker-container, geeft je een webinterface in een donker, licht of systeemthema, en beheert de hele levenscyclus: back-uppen, plannen, verifiëren en herstellen.

Herstel gaat automatisch. Containers verschijnen weer in het Unraid Docker-tabblad precies zoals voorheen, en VM's worden opnieuw gedefinieerd in de VM Manager met hun schijven en UEFI NVRAM opnieuw gekoppeld. Geen handmatige herinstallatie, geen herconfiguratie, geen gedoe.

Aangedreven door [restic](https://restic.net), zodat elke back-up gededupliceerd, incrementeel en altijd versleuteld is.

!!! note "Bewaar je APP_KEY veilig"
    BombVault leidt het wachtwoord van de restic-repository af uit een 32-byte geheim genaamd `APP_KEY`. Raak je het kwijt, samen met de herstelkit, dan zijn versleutelde back-ups onherstelbaar. Genereer er een met `openssl rand -hex 32` en bewaar het ergens veilig. Zie [Configuratie](configuration.md).

## Wat BombVault beschermt

Elke container, VM, mappenset, de flash, elke ZFS-dataset en de eigen instellingen van BombVault zijn een **item** met een eigen pagina. Items van dezelfde soort vormen samen een **onderdeel**, dat je inschakelt onder **Instellingen, Algemeen**.

| Onderdeel | Wat wordt opgeslagen |
|---|---|
| **Containers** | Appdata-map plus de containerdefinitie (image, env-variabelen, poorten, labels, volumes). |
| **VM's** | VM-schijfimage(s), de XML-definitie en UEFI NVRAM, geback-upt via SSH (geen libvirt-mount). |
| **Flash** | De hele USB-flash (`/boot`): OS, licentie, array-configuratie, shares, netwerk- en plugin-configuratie. |
| **Zelf-back-up** | BombVaults eigen `/config`: de instellingendatabase, de inloggegevens van de bestemmingen en het libvirt SSH-sleutelpaar. |
| **Mappen** | Elke map op de server als mappenset, elk met optionele exclude-patronen. |
| **ZFS-datasets** | Een dataset met alle datasets eronder, gelezen uit één ZFS-snapshot en opgeslagen als een map. Zie [ZFS-datasets](zfs-datasets.md). |

## Herstel is de ster

Nadat de data is teruggekopieerd uit de restic-back-up, speelt BombVault de opgeslagen containerdefinitie opnieuw af tegen de Docker API, zodat de container weer in het Unraid Docker-tabblad verschijnt alsof hij er altijd is geweest (dezelfde image, dezelfde instellingen, dezelfde poorttoewijzingen). VM's krijgen hun XML opnieuw gedefinieerd via SSH en hun schijven en UEFI NVRAM opnieuw gekoppeld, zelfs nadat de VM was verwijderd.

Eén venster herstelt elke soort. Standaard kiest het **Naast het origineel**: de back-up komt als kopie terug en het origineel blijft zoals het is. **Origineel vervangen** maakt eerst een back-up van de huidige staat, zodat een herstel ongedaan te maken is. Een map of een nieuwe ZFS-dataset zijn de andere doelen.

Wanneer een back-up afhankelijke containers stopt, komen ze in de juiste volgorde terug: BombVault herstart ze in hun Compose `depends_on`-volgorde en wacht tot elk healthy meldt voordat het de containers start die ervan afhangen, zodat niets vooruit racet op een database of gateway die nog niet online is. Zie [Functies](features.md).

## Hoe het werkt

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault gebruikt de Docker-socket om containers vóór een back-up te stoppen en na een herstel opnieuw aan te maken. Voor VM's voert het `virsh` op de host uit via SSH (`qemu+ssh://`) om een VM netjes af te sluiten of een live snapshot te maken. Het bind-mount nooit een libvirt-pad, dus het kan de VM Manager op de host niet in de weg zitten.

BombVault is de orkestratie- en UI-laag, niet de opslagengine. Alle daadwerkelijke dataverplaatsing gaat via restic.

## Snelstart

Nieuw hier? Ga naar **[Aan de slag](getting-started.md)** om BombVault op Unraid te installeren via Community Applications en je eerste back-up te maken. Verken daarna de volledige **[Functies](features.md)**, stel je **[Configuratie](configuration.md)** af en zet **[Off-site en herstel](offsite-recovery.md)** op.

Elk onderdeel kan tegelijk naar meerdere bestemmingen kopiëren, een gekoppelde BombVault kan je off-site kopieën bewaren op een server die verwijderen weigert, en de **Noodassistent** haalt een verloren server terug met de herstelkit, via je groep of met de ingetypte sleutel. Om je configuratie mee te nemen naar een nieuwe machine gebruik je de kaart **Instellingen exporteren / importeren**. Zie [Off-site en herstel](offsite-recovery.md) en [Configuratie](configuration.md#portable-settings-export-and-import).

De **[Android-app](android.md)** zet elke server van je groep op je telefoon, met het activiteitenlogboek van allemaal op één scherm.

## Met dank aan {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** van [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) gaf BombVault het uitgangsidee: Docker-containers met één klik back-uppen en automatisch opnieuw installeren. BombVault is een aparte implementatie in Go en restic die het idee doortrekt naar VM's, de flash en meer.
- **[restic](https://restic.net/)** is de snelle, veilige, dedupliceerende back-upengine die BombVault aanstuurt.
- **[rclone](https://rclone.org/)** levert de cloudbackends.
- De meeste symbolen op knoppen komen uit de gratis Core Solid-set van **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [bron](https://github.com/webalys-hq/streamline-vectors)). De overige komen uit Font Awesome Free, Material Design Icons, Simple Icons en Tabler Icons, of zijn voor het project getekend.

## Licentie {#license}

Copyright (C) 2026 Junker der Provinz. BombVault is vrije software onder de **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Je mag het uitvoeren, bestuderen, delen en wijzigen. Als je het verspreidt, of een gewijzigde versie als netwerkdienst aanbiedt, moet je je broncode onder dezelfde licentie publiceren en de bestaande copyright- en naamsvermeldingen behouden.

De naam en de huisstijl vallen niet onder de licentie. De AGPL dekt alleen de broncode: "BombVault", het logo en de huisstijl blijven voorbehouden, dus een fork moet een eigen naam en huisstijl gebruiken en mag zich niet als BombVault voordoen.

## Links

- **Broncode:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid-supportthread:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issues:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-gelijkwaardige controle over de host"
    Via de Docker-socket kan BombVault containers stoppen, verwijderen en opnieuw aanmaken en appdata lezen/schrijven, en voor VM-back-up logt het via SSH in op de host om `virsh` uit te voeren. Iedereen die de web-UI kan bereiken heeft in feite root op de host. Draai BombVault alleen op een vertrouwd, niet-blootgesteld netwerk en stel een inlogwachtwoord in onder **Instellingen, Beveiliging**. De herstelkit, bewaarkopieën van de sleutel en het verwijderen van de back-ups op een bestemming vragen erom. Zie [Configuratie](configuration.md#security-model) voor het volledige beveiligingsmodel.
