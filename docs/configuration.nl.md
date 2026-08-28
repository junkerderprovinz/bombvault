# Configuratie

Deze pagina behandelt de omgevingsvariabelen van de container, de mounts die de template levert, VM-back-up via SSH en de off-site setup. Back-up**repository-paden** worden binnen de app geconfigureerd (Instellingen, Back-uppaden), niet via omgevingsvariabelen.

## Omgevingsvariabelen

| Variabele | Vereist | Beschrijving |
|---|---|---|
| `APP_KEY` | **Ja** | 32-byte hex-geheim (64 hex-tekens) gebruikt om het restic-repo-wachtwoord af te leiden. Genereer met `openssl rand -hex 32`. Bewaar dit veilig: kwijtraken maakt versleutelde back-ups onherstelbaar. |
| `LIBVIRT_HOST` | Voor VM's | Unraid-host bereikt via SSH voor VM-back-up (standaard `host.docker.internal`; de template vult vooraf een LAN-IP-placeholder in). Gebruik je Unraid LAN-IP, vereist op een custom `br0.x`-netwerk. |
| `LIBVIRT_SSH_PORT` | Nee | SSH-poort van de host voor VM-back-up (standaard `22`). |
| `LIBVIRT_SSH_USER` | Nee | SSH-gebruiker op de host voor VM-back-up (standaard `root`). |
| `LIBVIRT_URI` | Nee | Volledige libvirt-verbindings-URI, **letterlijk** gebruikt in plaats van er één op te bouwen uit de drie `LIBVIRT_*`-variabelen hierboven (die dan voor de verbindingsstring worden genegeerd). Standaard niet ingesteld. Nodig op TrueNAS Scale, waar libvirtd luistert op een niet-standaard socket die de opgebouwde vorm niet kan uitdrukken: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Zie de TrueNAS Scale-sectie van [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). |
| `PORT` | Nee | HTTP-poort (standaard `3000`; alleen gebruikt met `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Nee | HTTPS-poort (standaard `3443`; de template publiceert hem 1:1, dus de WebUI antwoordt op `https://<ip>:3443`). |
| `HTTP_ONLY` | Nee | Zet `true` om de zelfondertekende HTTPS-listener uit te schakelen en alleen platte HTTP te serveren (voor gebruik achter een TLS-terminerende reverse proxy). |
| `HOST_SOURCE_ROOT` | Nee | Het hostpad dat als **Host Data** wordt gemount (standaard `/mnt`). BombVault vertaalt de bind-mount-bronnen die Docker meldt naar paden onder deze mount. Wijzig alleen als je een andere host-root hebt gemount. |
| `DATA_ROOT_SEGMENTS` | Nee | Kommagescheiden padsegmentnamen die een bind-mount-bron als back-updata markeren (standaard `appdata`, overeenkomstig Unraids conventie `/mnt/user/appdata/<container>`). De bind-mount van een container wordt automatisch geselecteerd voor back-up zodra ELK opgegeven segment als volledig padsegment in de hostbron voorkomt; `DATA_ROOT_SEGMENTS=appdata,config` pikt bijvoorbeeld ook een `.../config`-bind op. Zie [Detectie van back-upbronnen](#backup-source-detection) voor de andere, altijd-actieve manieren waarop de datamap van een container wordt gevonden. |
| `PLATFORM` | Nee | Forceert welk platform BombVault denkt te draaien, in plaats van automatisch te detecteren: `unraid`, `generic` of `truenas` (standaard niet ingesteld: detecteert Unraid automatisch door te zoeken naar diens `dockerMan`-marker onder de flash-mount, anders `generic`; een onherkende waarde valt ook terug op `generic`, gelogd). Stel het expliciet in op een generieke Docker-host of TrueNAS Scale, in plaats van te vertrouwen op de automatische Unraid-detectie; het generieke compose-bestand doet dit. Verandert de appdata-fallbackconventie, de standaardbestemmingen voor herstel tussen instanties, en of de Unraid-only meldings-/companion-plugin-stappen wel worden geprobeerd (zie `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Nee | De naam van de BombVault-container zelf, zodat het nooit een back-up maakt van (en dus stopt) zichzelf (standaard `BombVault`; automatisch gedetecteerd via de hostnaam op bridge-netwerken). |
| `BACKUP_MAX_HOURS` | Nee | Maximaal aantal klok-uren dat een enkele back-uprun zijn domeinlock mag vasthouden voordat hij geforceerd wordt geannuleerd (een beveiliging zodat een vastgelopen run het domein niet eeuwig kan blokkeren). Leeg (de standaard) gebruikt `48`. Verhoog het voor zeer grote of trage cloud-back-ups (een run die bij de limiet wordt geannuleerd mislukt met `context deadline exceeded`). Zet `0` om de limiet helemaal uit te schakelen. |
| `TZ` | Nee | Tijdzone voor de planner (bijvoorbeeld `Europe/Berlin`). **Niet ingesteld betekent dat alle planningen in UTC draaien**: een planning op 02:30 start dan om 02:30 UTC en niet op de lokale klok. Op Unraid stel je dit nooit zelf in: het systeem geeft zijn eigen tijdzone door aan elke container. |

## Mounts

Mount de Docker-socket, de flash (`/boot`) en de root **Host Data** (`/mnt`) zoals getoond in de CA-template. Back-up*bronnen* en *bestemmingen* leven allebei onder Host Data, en het wordt **rslave** gemount zodat een remote share die na de start van de container mount (bijvoorbeeld onder `/mnt/remotes`) zichtbaar wordt zonder herstart.

Back-uprepository-paden gaan standaard naar `/mnt/user/bombvault/{container,vms,flash,config,files}`, aangemaakt bij de eerste back-up. Wijzig de locatie op elk moment in **Instellingen, Back-uppaden**.

!!! note "Controle van hostintegratie"
    Open `/spike` in de web-UI nadat de container is gestart. Het test elke mount en CLI (Docker-socket, libvirt, restic, qemu-img, rclone) en meldt eventuele ontbrekende onderdelen.

## Beveiligingsmodel

!!! warning "Root-gelijkwaardige controle over de host"
    Via de Docker-socket kan BombVault containers stoppen, verwijderen en opnieuw aanmaken en appdata lezen/schrijven, en voor VM-back-up logt het via SSH in op de host (`qemu+ssh://`, standaard root) om `virsh` uit te voeren. Iedereen die de web-UI kan bereiken heeft in feite root op de host.

- **Optionele wachtwoordbeveiliging** (Instellingen, Beveiliging): stel een wachtwoord in om login te vereisen, wis het om uit te schakelen. Standaard uit voor gebruik op een vertrouwd LAN. Sessies zijn ondertekend (HMAC afgeleid van `APP_KEY`) en het wachtwoord wijzigen maakt ze ongeldig; logins zijn rate-limited.
- Omdat de poort opt-in is, zijn wanneer die niet is ingesteld de hele UI en API (inclusief de off-site setup, tamper-test-routes en de herstelkit) bereikbaar voor iedereen die de poort kan bereiken. Schakel de beveiliging in zodra off-site, onveranderlijke back-ups of versleuteling in gebruik zijn.
- Draai BombVault alleen op een vertrouwd, niet-blootgesteld netwerk. Zet het voor externe toegang achter een reverse proxy die authenticatie en TLS toevoegt. Antwoorden dragen basis-beveiligingsheaders (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Met `HTTP_ONLY=true` verliest de sessiecookie zijn `Secure`-vlag (dat moet, om over platte HTTP te werken), dus schakel het wachtwoord alleen achter een TLS-terminerende proxy in als vertrouwelijkheid ertoe doet.
- De VM-back-up-SSH-verbinding vertrouwt de host key bij het eerste contact (TOFU) en pint hem daarna vast. Verifieer de host key van de host out-of-band als je container-naar-host-pad niet vertrouwd is.
- Back-ups worden door restic versleuteld wanneer versleuteling is ingeschakeld (Instellingen; standaard aan), met de sleutel afgeleid van `APP_KEY`.

## VM-back-up via SSH

BombVault maakt back-ups van KVM/libvirt-VM's **zonder enig libvirt-pad te mounten**. Het draait `virsh` op de host via SSH (`qemu+ssh://`), zodat het nooit je host-VM Manager kan beïnvloeden.

Snelle setup:

1. **Instellingen, Systeem, VM-back-up via SSH:** kopieer de getoonde publieke sleutel.
2. Voeg hem toe aan Unraids `/root/.ssh/authorized_keys` (ook op de flash bewaard zodat hij herstarts overleeft).
3. Klik op **Verbinding testen**.

De template voegt `--add-host=host.docker.internal:host-gateway` toe zodat de container de host kan bereiken. Stel `LIBVIRT_HOST` in op je Unraid LAN-IP als die naam niet resolvt (bijvoorbeeld wanneer de container op een custom `br0.x`-netwerk draait). Als je de SSH-poort van Unraid hebt gewijzigd, stel `LIBVIRT_SSH_PORT` overeenkomstig in. **Live snapshots** hebben daarnaast de qemu guest agent in de VM nodig en de schijf op `/mnt/cache` (niet `/mnt/user`).

!!! important "Volledige VM-setup en netwerkgids"
    De complete stap-voor-stap-gids (SSH inschakelen, persistente sleutelautorisatie, custom-netwerk- en VLAN-routing, methode per VM en probleemoplossing aan de hostkant) staat op [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) op GitHub.

## Off-site setup

Stel een off-site replica in op het tabblad **Instellingen, Off-site**. Zie [Off-site en herstel](offsite-recovery.md) voor de volledige workflow (onveranderlijk/append-only, tamper-testen en DR-oefeningen). Kort samengevat:

- **Backends:** SMB/CIFS en NFS (mount de share en wijs er een Backup Path naar), native restic-backends zonder rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`), of elke rclone-remote (`rclone:<remote>:<bucket>/path`).
- **Cloud-inloggegevens** worden versleuteld opgeslagen onder Instellingen, Off-site, Cloud-inloggegevens.
- **SSH-doelen hebben niets geïnstalleerd nodig aan de andere kant.** `sftp:` heeft alleen een SSH-server nodig. Voeg de publieke sleutel uit **Instellingen, Systeem, VM-back-up via SSH** (ook op `/config/ssh/id_ed25519.pub`) toe aan de `~/.ssh/authorized_keys` van de doelgebruiker.
- **Off-site kopie:** BombVault repliceert nieuwe snapshots met `restic copy` op best-effort-basis. De lokale repo blijft primair. Elk domein heeft zijn eigen off-site planning, plus een knop **Nu repliceren**.
- **Meerdere off-site doelen per domein:** elk domein kan tegelijk naar meerdere off-site bestemmingen repliceren. Voeg extra doelen toe op Instellingen, Off-site, elk met zijn eigen repository, S3-opslagklasse, append-only-vlag, retentie en groeibudget; ze repliceren allemaal op de off-site planning van dat domein. Een bestaande enkele off-site setup wordt overgenomen als het eerste doel.
- **Retentie per bron:** het lokale beleid staat op Instellingen, Paden en Opslag; het off-site beleid op Instellingen, Off-site (laat het geheel op nul om off-site snapshots nooit automatisch te trimmen).
- **Bandbreedtelimieten:** begrens de restic-upload/downloadsnelheid onder Instellingen, Off-site.
- **Koude en archiefopslagklasse (S3):** kies voor een native S3 off-site repo een herstel-leesbare tier (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). rclone-remotes stellen hun klasse in de rclone-config in.

## Portable instellingen (exporteren en importeren) {#portable-settings-export-and-import}

De kaart **Instellingen exporteren en importeren** op de Instellingen-pagina schrijft je hele BombVault-configuratie (domeininstellingen, off-site doelen, planningen, retentie, meldingen) naar een portable JSON-bestand dat je op een andere instantie kunt importeren, zodat verhuizen naar een nieuwe machine of een setup klonen niet betekent dat je alles met de hand opnieuw invoert. Import toont een voorbeeld en vraagt om bevestiging, en raakt nooit je back-updata of historie aan.

!!! warning "De export kan inloggegevens bevatten"
    Je kiest of je de off-site en meldingsinloggegevens in het bestand meeneemt. Met inloggegevens erbij is de export net zo gevoelig als je herstelkit, dus bewaar hem ergens veilig. Zonder die bevat het bestand alleen niet-geheime instellingen.
