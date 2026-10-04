# BombVault

**Je Unraid-data, veilig opgeborgen in een kluis. Laat een back-up vallen. Blaas een herstel af.**

BombVault is een self-hosted, Unraid-native webapp voor **back-up en volledig noodherstel** van je Docker-containers en KVM/libvirt-VM's. Het draait als een enkele multi-arch Docker-container, geeft je een moderne web-UI die de licht/donker-voorkeur van je systeem volgt, en beheert de hele levenscyclus: back-uppen, plannen, verifiëren en herstellen.

Herstel gaat automatisch. Containers verschijnen weer in het Unraid Docker-tabblad precies zoals voorheen, en VM's worden opnieuw gedefinieerd in de VM Manager met hun schijven en UEFI NVRAM opnieuw gekoppeld. Geen handmatige herinstallatie, geen herconfiguratie, geen gedoe.

Aangedreven door [restic](https://restic.net), zodat elke back-up gededupliceerd, incrementeel en altijd versleuteld is.

!!! note "Bewaar je APP_KEY veilig"
    BombVault leidt het wachtwoord van de restic-repository af uit een 32-byte geheim genaamd `APP_KEY`. Als je het kwijtraakt, zijn versleutelde back-ups onherstelbaar. Genereer er een met `openssl rand -hex 32` en bewaar het ergens veilig. Zie [Configuratie](configuration.md).

## Wat BombVault beschermt

| Domein | Wat wordt opgeslagen |
|---|---|
| **Docker-containers** | Appdata-map plus de containerdefinitie (image, env-variabelen, poorten, labels, volumes). |
| **KVM / libvirt-VM's** | VM-schijfimage(s), de XML-definitie en UEFI NVRAM, geback-upt via SSH (geen libvirt-mount). |
| **Unraid-flash** | De hele USB-flash (`/boot`): OS, licentie, array-configuratie, shares, netwerk- en plugin-configuratie. |
| **App-configuratie** | BombVaults eigen `/config`: de instellingendatabase, off-site inloggegevens en het libvirt SSH-sleutelpaar. |
| **Bestanden en mappen** | Benoemde **bestandssets**, elke map op de server, elk met optionele exclude-patronen per set. |
| **ZFS-datasets** | Een dataset met alle datasets eronder, gelezen uit één ZFS-snapshot en opgeslagen als een map. Zie [ZFS-datasets](zfs-datasets.md). |

## Herstel is de ster

Nadat de data is teruggekopieerd uit de restic-snapshot, speelt BombVault de opgeslagen containerdefinitie opnieuw af tegen de Docker API, zodat de container weer in het Unraid Docker-tabblad verschijnt alsof hij er altijd is geweest (dezelfde image, dezelfde instellingen, dezelfde poorttoewijzingen). VM's krijgen hun XML opnieuw gedefinieerd via SSH en hun schijven en UEFI NVRAM opnieuw gekoppeld, zelfs nadat de VM was verwijderd.

Wanneer een back-up afhankelijke containers stopt, komen ze in de juiste volgorde terug: BombVault herstart ze in hun Compose `depends_on`-volgorde en wacht tot elk healthy meldt voordat het de containers start die ervan afhangen, zodat niets vooruit racet op een database of gateway die nog niet online is. Zie [Functies](features.md).

## Hoe het werkt

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
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

Off-site kan tegelijk uitwaaieren naar meerdere doelen per domein, een alleen-lezen **ontvanger-dashboard** bewaakt die kopieën op de machine die ze ontvangt, en je kunt je hele configuratie meenemen naar een nieuwe machine met de kaart **Instellingen exporteren / importeren**. Zie [Off-site en herstel](offsite-recovery.md) en [Configuratie](configuration.md#portable-settings-export-and-import).

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
    Via de Docker-socket kan BombVault containers stoppen, verwijderen en opnieuw aanmaken en appdata lezen/schrijven, en voor VM-back-up logt het via SSH in op de host om `virsh` uit te voeren. Iedereen die de web-UI kan bereiken heeft in feite root op de host. Draai BombVault alleen op een vertrouwd, niet-blootgesteld netwerk en schakel de optionele wachtwoordbeveiliging (Instellingen, Beveiliging) in zodra off-site of onveranderlijke back-ups in gebruik zijn. Zie [Configuratie](configuration.md) voor het volledige beveiligingsmodel.
