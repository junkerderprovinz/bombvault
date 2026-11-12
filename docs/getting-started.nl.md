# Aan de slag

Deze pagina leidt je van een verse Unraid-machine naar je eerste back-up.

## Vereisten {#requirements}

| Vereiste | Opmerkingen |
|---|---|
| **Unraid 6.12+** | Eerdere versies zijn niet getest. Unraid is het hoofddoel, maar BombVault draait ook op een gewone Docker-host en op TrueNAS Scale (zie [Gewone Docker-host](#generic-docker-host) en [TrueNAS SCALE](#truenas)). |
| **Locatie van het restic-repository** | Een lokaal pad (aanbevolen: je array of cache), SMB, NFS of een willekeurige rclone-backend. |
| **Docker-socket** | Automatisch gemount door de template (`/var/run/docker.sock`). |
| **Unraid-flash** (`/boot`) | In zijn geheel automatisch gemount door de template (`/boot` naar `/host/boot`). Maakt de flash-back-up mogelijk en laat een herstelde container weer verschijnen als een gewone, bewerkbare Unraid-app. |
| **KVM-VM's** (optioneel) | VM-back-up praat met libvirt via SSH, zonder libvirt-mount. Stel het in onder **Instellingen, Integraties, Host-SSH** (zie [Configuratie](configuration.md)). |
| **ZFS-datasets** (optioneel) | Dezelfde SSH-verbinding als bij VM-back-ups, `zfs` op de host, en Host Data gekoppeld als `/mnt` met toegangsmodus Read/Write - Slave, de standaard van de template. Zie [ZFS-datasets](zfs-datasets.md). |
| **Android-app** (optioneel) | Android 10 of nieuwer, gekoppeld met servers op versie 9.7.0 of nieuwer. Zie [Android-app](android.md). |

## Installeren op Unraid {#install-on-unraid}

De makkelijkste weg is **Community Applications**.

1. Open het tabblad **Apps** in Unraid.
2. Zoek naar **BombVault**.
3. Klik op **Install**, stel de vereiste variabelen in (hieronder) en pas toe.

!!! tip "Template handmatig installeren"
    Voeg je de template liever zelf toe:

    1. Ga naar **Docker, Add Container, Template repositories** en voeg toe:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Zoek bij Templates naar **BombVault**.
    3. Stel de vereiste variabelen in en klik op **Apply**.

## Gewone Docker-host {#generic-docker-host}

Geen Unraid? BombVault draait ook als gewone container op elke Docker-host. TrueNAS SCALE heeft eigen stappen onder [TrueNAS SCALE](#truenas).

1. Haal het kant-en-klare, bewerkbare bestand [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) uit de repo.
2. Stel `APP_KEY` in (zie hieronder) en wijs het Host Data-volume naar je echte dataroot. De opmerkingen in het bestand lopen beide langs.
3. `docker compose up -d`, en open dan `https://<host-ip>:3443/`.

Wat anders is dan op Unraid:

- **Geen onderdeel Flash.** Er is geen boot-USB om te bewaren of te herstellen, dus het onderdeel Flash heeft hier niets te doen. Reikt Host Data tot `/etc` van de host, dan biedt **Toevoegen** op de pagina Back-ups in plaats daarvan **Systeemconfiguratie** aan als mapsuggestie: een voorgestelde mappenset voor `/etc` die je nakijkt en bewerkt voordat je opslaat.
- **Geen eigen meldingen van Unraid.** De meldingskanalen in de app van BombVault zelf (webhook, waarschuwingen bij mislukte off-site-kopieën enzovoort) werken gewoon. Alleen de Unraid-specifieke push naar het eigen meldingssysteem wordt overgeslagen, omdat er zo'n systeem niet is.
- **VM-back-up is optioneel en heeft een aparte libvirtd-host nodig die via SSH bereikbaar is.** Zie het uitgecommentarieerde blok in het compose-bestand. Een gewone Docker-host heeft zelf geen VM-beheer.
- **Geen dashboardtegel.** TollRoll is een Unraid-plug-in, dus ook die stap wordt overgeslagen.
- **De data van een container vinden.** Zonder de `appdata`-conventie van Unraid wordt de datamap van een container gevonden via de segmenten in `DATA_ROOT_SEGMENTS`, Docker-benoemde volumes, de werkmap van een Compose-project en het label `bombvault.data` (zie [Detectie van back-upbronnen](configuration.md#backup-source-detection)). Benoemde volumes en de preset voor `/etc` bereiken alleen paden binnen de Host Data-mount, dus wijs Host Data naar een gemeenschappelijke voorouder die ook de dataroot van Docker omvat.
- **`PLATFORM`.** Zet het op `generic` of `truenas`. Laat je het leeg, dan detecteert BombVault Unraid aan zijn eigen markering op de flash-mount en behandelt alles anders als generiek, en worden de stappen die alleen voor Unraid zijn overgeslagen in plaats van geprobeerd en mislukt.

### TrueNAS SCALE {#truenas}

BombVault staat nog niet in de TrueNAS-appcatalogus, dus installeer het als eigen app: **Apps, Discover Apps**, daarna **Install via YAML** in het menu naast **Custom App**. De YAML hieronder stelt hetzelfde in als het catalogusitem in de map `truenas-apps` van de repository. Vervang `tank` door je pool en vul `APP_KEY` in zoals de volgende sectie beschrijft.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # For VM backups:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

Open daarna `https://<truenas-ip>:30528/`. Wat onder [Wat anders is dan op Unraid](#generic-docker-host) staat, geldt ook hier, en op TrueNAS geldt bovendien:

- **Capabilities.** BombVault heeft de zeven hierboven nodig. Zonder `SETPCAP` mislukken VM-back-ups, en zonder `SYS_CHROOT` wordt elke container voor zijn hele back-up gestopt in plaats van uit een snapshot gelezen.
- **Host Data** is een hostpad op een dataset, hier `/mnt/tank`, op hetzelfde pad in de app gemount. Daarom noemen `HOST_SOURCE_ROOT` en `HOST_MOUNT_ROOT` het allebei. `rslave` geeft de mounts door die de host later maakt, wat back-ups van ZFS-datasets nodig hebben.
- **Data in een ixVolume wordt niet geback-upt.** Een app waarvan de opslag een ixVolume is, bewaart zijn data onder `/mnt/.ix-apps`, buiten Host Data. BombVault bewaart alleen de definitie van de app, en de run noemt de mappen die hij heeft overgeslagen. Kies voor elke app waarvan je de data wilt back-uppen opslag van het type **Host Path** op een dataset onder Host Data.
- **Back-ups van VM's en ZFS-datasets** bereiken TrueNAS via SSH als root. Zet de SSH-service aan onder **System, Services**, en voeg de publieke sleutel uit **Instellingen, Integraties, Host-SSH** toe aan de gebruiker root onder **Credentials, Users**. VM-back-ups hebben ook `LIBVIRT_URI` nodig zoals hierboven, omdat libvirtd van TrueNAS op een eigen socket luistert. Zijn noch `LIBVIRT_URI` noch `LIBVIRT_HOST` ingesteld, dan gebruikt BombVault de standaardgateway van de app als host, omdat `host.docker.internal` in een TrueNAS-app niet oplost. De [VM-back-upgids](vm-backup-ssh-setup.md#truenas-scale) behandelt de details, VM's op zvols inbegrepen.
- **Een ZFS-replica gebruikt een andere sleutel.** Een replica op een tweede ZFS-server logt in met de sleutel die het replicavenster toont, niet met de Host-SSH-sleutel. Voeg die toe op de ontvangende server.
- **Een verwijderde app komt in twee stappen terug.** Installeer hem eerst in TrueNAS opnieuw onder dezelfde naam, wat zijn netwerk opnieuw aanmaakt, en herstel dan zijn data in BombVault. Tot dan meldt de herstelcontrole het ontbrekende netwerk en weigert het herstel voordat het iets verandert.
- **Naast het origineel** maakt een gewone Docker-container aan. Die draait, maar het scherm Apps van TrueNAS toont hem niet.
- **Databasedumps** werken op datasetmounts. Een import in een datamap die een eigen dataset is, verplaatst de oude inhoud naar een map ernaast, dus de dataset erboven heeft ruimte nodig voor die inhoud.

### ZimaOS, CasaOS en andere hosts {#other-hosts}

- **ZimaOS en CasaOS:** voeg de [communitystore](https://github.com/junkerderprovinz/zimaos-apps#install) toe en installeer BombVault daaruit. Het draait er als `PLATFORM=generic` met Host Data op `/DATA`. ZimaOS bewaart appdata in `/DATA/AppData/<app>`, wat BombVault zelf vindt, omdat `DATA_ROOT_SEGMENTS` `appdata` in elke schrijfwijze herkent.
- **MOS:** installeer BombVault uit de [MOS Hub](https://mos-official.net/apps.html?search=BombVault). Het draait er als `PLATFORM=generic` met Host Data op `/mnt/cache`.
- **Synology:** maak in Container Manager een project aan uit het compose-bestand (DSM 7.2 of nieuwer).
- **Proxmox:** draai het in een VM of een LXC-container met Docker.

Zonder opmerkingen is het generieke compose-bestand kort. Vervang `./data` door de hostmap met de data van je containers, zet `HOST_SOURCE_ROOT` op datzelfde pad en vul `APP_KEY` in:

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    restart: unless-stopped
    hostname: bombvault
    container_name: bombvault
    volumes:
      - ./config:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/host/user
    environment:
      APP_KEY: ""
      PLATFORM: generic
      HOST_SOURCE_ROOT: /host/user
    ports:
      - "3443:3443"
```

Om ook Docker-benoemde volumes te back-uppen, mount je een map die zowel je data als de dataroot van Docker bevat (`docker info -f '{{.DockerRootDir}}'`, meestal `/var/lib/docker`), zoals de Unraid-template heel `/mnt` mount.

## De enige vereiste instelling {#the-one-required-setting}

De enige variabele die je moet instellen is `APP_KEY`, een geheim van 32 bytes in hex (64 hex-tekens) waarmee het wachtwoord van het restic-repository wordt afgeleid.

Genereer er een op een willekeurige machine:

```bash
openssl rand -hex 32
```

Plak het resultaat in het veld `APP_KEY` van de template (Unraid), of in de omgevingsvariabele `APP_KEY` in `docker-compose.yml` (gewone Docker-host).

!!! danger "Raak je APP_KEY niet kwijt"
    Raak je `APP_KEY` kwijt, dan zijn je versleutelde back-ups onherstelbaar. Bewaar hem ergens veilig en gescheiden van de server. Stel zodra BombVault draait een inlogwachtwoord in en download de **herstelkit** onder **Instellingen, Herstel** (zie [Off-site en herstel](offsite-recovery.md#recovery-kit)). Die bevat de sleutel, elke bestemming en de restic-opdrachten.

De template mount ook de Docker-socket, de flash (`/boot`) en de **Host Data**-root (`/mnt`) voor je. Back-up*bronnen* en *-bestemmingen* staan allebei onder Host Data. Voor de volledige variabelenreferentie en de off-site-instelling, zie [Configuratie](configuration.md).

## Eerste keer starten {#first-run}

![De welkomstpagina van een nieuwe installatie, met wat BombVault op de server heeft gevonden.](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![De welkomstpagina van een nieuwe installatie, met wat BombVault op de server heeft gevonden.](assets/screenshots/getting-started/welcome-light.png#only-light)

1. Open de webinterface op `https://<je-unraid-ip>:3443` (standaard een zelfondertekend certificaat). Een nieuwe installatie opent op **Welkom bij BombVault**, dat toont wat BombVault op de server heeft gevonden.
2. Kies **Aan de slag**. Het overzicht toont dan de kaart **Eerste stappen** met drie stappen:
    1. **Kies onderdelen**: schakel in wat BombVault back-upt (Containers, VM's, Flash, Mappen, ZFS-datasets, Zelf-back-up). **Kiezen** opent de lijst met items van een onderdeel, waar **Schema pauzeren** er een weglaat. De onderdelen die je hier inschakelt, draaien elke nacht, wat je wijzigt onder **Instellingen, Planning**.
    2. **Bestemming toevoegen**: een tweede plek voor de back-ups. Elk onderdeel heeft al een map op deze server, `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`, aangemaakt bij de eerste back-up. Het veiligst is een tweede bestemming buiten de locatie, zoals een Storage Box of Backblaze B2.
    3. **Eerste back-up**: daarna loopt het elke nacht vanzelf. De kaart blijft staan tot de eerste back-up klaar is.
3. Open **Back-ups** om elk item in één lijst te zien. **Nu alles back-uppen** start een run over alle ingeschakelde onderdelen, en **Nu back-up maken** op de pagina van een item maakt alleen van dat item een back-up.

![De kaart Eerste stappen met zijn drie stappen, de tweede geopend.](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![De kaart Eerste stappen met zijn drie stappen, de tweede geopend.](assets/screenshots/getting-started/first-steps-light.png#only-light)

BombVault al eerder gebruikt? **Herstellen vanaf een back-up** op de welkomstpagina opent de pagina Herstel, waar de **Noodassistent** de instellingen, containers, VM's en mappen van een verloren server terugbrengt. Zie [Off-site en herstel](offsite-recovery.md#emergency).

!!! tip "Optioneel: kies een back-upvolgorde"
    Moeten sommige containers altijd vóór andere worden geback-upt (bijvoorbeeld een database vóór de app die hem gebruikt), open dan de kaart **Volgorde** onder **Instellingen, Planning** en sleep ze in de gewenste reeks. Geplande runs en de volledige back-up volgen die dan. Alles wat je ongeordend laat, wordt geback-upt met de langst achterstallige eerst.

## Voorbeeld: je eerste containerback-up, van begin tot eind {#example-first-backup}

Dit loopt langs een verse Unraid-machine met een paar containers, onder meer `plex`, tot de eerste back-up klaar is en je weet waar je hem vindt. Het duurt ongeveer tien minuten plus de tijd van de eerste back-up.

1. **Installeren.** Zoek in het Unraid-tabblad **Apps** naar **BombVault** en druk op **Install**.
2. **De sleutel instellen.** Voer op een computer `openssl rand -hex 32` uit en plak de 64 tekens in het veld `APP_KEY` van de template. Bewaar nu meteen een kopie in je wachtwoordmanager, vóór al het andere. Druk op **Apply**.
3. **Openen.** Ga naar `https://<je-unraid-ip>:3443` en accepteer het zelfondertekende certificaat. **Welkom bij BombVault** toont wat het vond, zoals 12 containers, 2 VM's en de flash.
4. **Aan de slag.** Het overzicht opent met de kaart **Eerste stappen**.
5. **Kies onderdelen.** **Containers** staat vanaf het begin aan. Schakel in wat je verder wilt, zoals **Flash**. **Kiezen** opent de lijst met containers die het vond, waar **Schema pauzeren** er een weglaat, bijvoorbeeld een testcontainer waar je niets om geeft. Ingeschakelde onderdelen draaien elke nacht in de volledige back-up.
6. **Bestemming toevoegen.** Elk onderdeel heeft al een eigen map op deze server, onder `/mnt/user/bombvault`, dus de eerste back-up heeft een plek om naartoe te gaan. Deze stap voegt een tweede plek toe, en de veiligste ligt buiten de locatie. Je kunt dat nu doen met het [S3-voorbeeld](offsite-recovery.md#example-s3), of erop terugkomen als de eerste back-up klaar is. De kaart blijft tot dan staan.
7. **Eerste back-up.** Open **Back-ups** en druk op **Nu alles back-uppen**, of wacht op de nacht. De ring in de statuskop wordt een voortgangsring en telt de items, zoals **Back-up wordt gemaakt: 3 van 12**. Elke container wordt gestopt terwijl zijn data wordt gelezen en meteen daarna weer gestart. Een container waarvan de data op ZFS ligt, ligt maar de paar seconden van een snapshot stil. De eerste back-up leest alles, dus die duurt het langst. Latere slaan alleen op wat veranderde.
8. **Controleren.** Als de ring **Alles is geback-upt** zegt, open dan **Geschiedenis** op het overzicht: elke container heeft een groene lijn bij **Runs**. Open **Back-ups**, klik op `plex` en kijk op zijn tabblad **Back-ups**: de eerste snapshot staat er, met **Lokaal** als de plek die hem bevat. Een paar minuten later toont zijn tabblad **Verificatie** de herstelcontrole, die een steekproef van de back-up herstelde en teruglas.
9. **De sleutel beschermen.** Open **Instellingen, Beveiliging** en stel een inlogwachtwoord in. Open daarna **Instellingen, Herstel** en druk in **Voor noodgevallen** op **Herstelkit downloaden**. Bewaar het bestand buiten de server, naast de kopie van `APP_KEY` uit stap 2. De herinnering **Bewaar je herstelkit** op het overzicht verdwijnt zodra je op **Ik heb hem veilig bewaard** drukt.

Vanaf nu worden de containers elke nacht geback-upt. Goede volgende stappen, in deze volgorde: een [off-site-bestemming](offsite-recovery.md#example-s3), zodat een kapotte server de back-ups niet meeneemt, dan [meldingen](notifications.md), zodat een mislukking je bereikt, en dan een blik op [Planningen](schedules.md) als 03:00 je niet uitkomt.

![De statuskop terwijl de eerste back-up loopt, met de voortgangsring en zijn stations.](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![De statuskop terwijl de eerste back-up loopt, met de voortgangsring en zijn stations.](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "Controle van de hostintegratie"
    De kaart **Host-integratie** op het overzicht test elke mount en elk CLI-programma (Docker-socket, libvirt, restic, qemu-img, rclone) en meldt wat er ontbreekt, zodat je kunt controleren of de container goed is aangesloten voordat je erop vertrouwt. Hij wacht onder **Aanpassen** tot je hem toevoegt.

## Je weg vinden {#navigation}

![Instellingen als tegels, met de kaarten van de gekozen tegel ernaast. Elke wijziging wordt opgeslagen terwijl je haar maakt.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Instellingen als tegels, met de kaarten van de gekozen tegel ernaast. Elke wijziging wordt opgeslagen terwijl je haar maakt.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

De zijbalk heeft vijf onderdelen: **Overzicht**, **Anomalieën** (zolang anomaliedetectie aanstaat), **Back-ups**, **Herstel** en, onder **Afmelden**, **Instellingen**. De hoofdactie van een pagina zweeft rechtsonder, en **Meer** bevat de zeldzame. Op een telefoon bevat de balk Overzicht, Back-ups, Herstel en Instellingen, en **Meer** de rest.

Pagina's en instellingen gebruiken de hele breedte van het venster. Een venster houdt zijn knoppen onderaan, met de knop die vooruit gaat rechts. Op een telefoon houdt een voet met meer dan drie knoppen de uitweg en de hoofdactie en zet de andere onder **Meer**.

![Op een telefoon: de balk onderaan met Meer geopend.](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![Op een telefoon: de balk onderaan met Meer geopend.](assets/screenshots/getting-started/phone-more-light.png#only-light)

- **Overzicht** toont of alles is geback-upt en wat je aandacht nodig heeft. Zie [Overzicht](overview.md).
- **Back-ups** toont elke container, VM, mappenset, de flash, elke ZFS-dataset en de zelf-back-up als items. Tegels boven de lijst kiezen een soort, en **Filters**, **Sorteren** en **Zoeken** versmallen haar. Elk item heeft een eigen pagina met drie tabbladen: **Back-ups**, **Instellingen** en **Verificatie**. Een terugknop links van het icoon van het item leidt naar de lijst. Zie [Back-ups en items](backups.md).
- **Herstel** is één pagina voor alles: kies een onderdeel, zoek een item en kies een back-up. De Noodassistent en de andere bronnen beginnen hier ook. Zie [Herstel](restore.md).
- **Instellingen** zijn veertien tegels: Algemeen, Uiterlijk, Planning, Bestemmingen, Herstel, Meldingen, Verificatie, Beveiliging, Instanties, Koppeling, Integraties, Apps, Systeem en Info. Zie [Instellingen](settings.md). Druk op `/` of Ctrl+K om ze allemaal te doorzoeken, ook op de namen die pagina's in eerdere versies hadden. Sleep de tegels in je eigen volgorde onder **Uiterlijk**. Waar een kolom met tegels naast de pagina haar zou samenknijpen, zoals in een venster van ongeveer 1200 pixels breed, worden de tegels een strook boven de pagina die zijwaarts scrolt. Een telefoon toont dezelfde strook.
- **Bestemmingen** en **Instanties** zijn instellingentegels met eigen pagina's. **Ook in de zijbalk tonen** op hun tegel zet ze vast in de zijbalk.

Niets heeft een knop Opslaan en niets klapt open: zeldzame dingen hebben een eigen kaart of een venster, en uitleg staat in de (i)-bubbels. Na een update opent vanzelf een venster met een kaart voor elke release sinds je laatste versie, en groene stippen markeren wat is veranderd. Een klik verbergt een stip. **Nieuw** in de kaart Versie onder **Instellingen, Info** telt wat je nog niet hebt gezien en opent dat venster opnieuw, met de notities van elke release en zijn wijzigingen met **Ga erheen**.

## Bouwen uit de broncode {#build-from-source}

BombVault is één statische Go-binary die een JSON-API en een ingebouwde React-interface serveert. Bouw eerst de interface en draai dan de binary:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

De interfacebuild is ook voor `go run` nodig. De repository volgt onder `web/dist` alleen een lege markering, dus zonder `npm --prefix web run build` bevat de binary niets en antwoordt hij `500 SPA index not found`, wat te verwachten is. Docker, libvirt en Unraid kunnen niet in CI worden getest, dus controleer mounts, restic en de VM-SSH-verbinding op een echte host met de kaart **Host-integratie** van het overzicht (of `GET /api/spike`) voordat je een pull request opent.

## Volgende stappen {#next-steps}

- Voeg een of meer bestemmingen buiten de locatie toe (elk onderdeel kan naar meerdere tegelijk kopiëren) en bewaar je herstelkit, zoals beschreven onder **[Off-site en herstel](offsite-recovery.md)**. Het [S3-voorbeeld](offsite-recovery.md#example-s3) loopt er een door.
- Zorg dat een mislukking je bereikt met **[Meldingen](notifications.md)**.
- Kijk wanneer dingen draaien onder **[Planningen](schedules.md)**.
- Zet elke server van je groep op je telefoon met de **[Android-app](android.md)**.
- De **[uitgewerkte voorbeelden](guides.md)** lopen de gebruikelijke klussen langs, en **[Functies](features.md)** toont alles wat BombVault doet.
- Een opstelling klonen of naar een nieuwe machine verhuizen? Neem je hele configuratie mee met de kaart **Instellingen exporteren / importeren**. Zie [Configuratie](configuration.md#portable-settings-export-and-import).
- Loop je ergens op vast? Zie **[Problemen oplossen](troubleshooting.md)**.
