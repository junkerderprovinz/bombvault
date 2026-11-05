# BombVault

**Dine Unraid-data, forseglet i en boks. Slip en sikkerhedskopi. Detonér en gendannelse.**

![Oversigten: statusdelen, hvad der kræver opmærksomhed, beskyttelsen af hvert område, destinationerne og historikken.](assets/screenshots/overview/home-dark.png#only-dark)
![Oversigten: statusdelen, hvad der kræver opmærksomhed, beskyttelsen af hvert område, destinationerne og historikken.](assets/screenshots/overview/home-light.png#only-light)

BombVault er en selvhostet webapp til **sikkerhedskopiering og fuld katastrofegendannelse** på Unraid og andre Docker-værter, for eksempel TrueNAS SCALE, ZimaOS eller en almindelig Linux-boks. Den sikkerhedskopierer Docker-containere, KVM/libvirt-VM'er, mapper, ZFS-datasæt og Unraids flashdrev. Den kører som en enkelt multi-arch Docker-container, giver dig en brugerflade i et mørkt, lyst eller systemtema, og håndterer hele livscyklussen: sikkerhedskopier, planlæg, verificer og gendan.

Gendannelser sker automatisk. På Unraid dukker containere op igen i Docker-fanen præcis som før, og VM'er bliver defineret på ny i VM Manager med deres diske og UEFI NVRAM tilkoblet igen. Ingen manuel geninstallation, ingen omkonfiguration, intet drama. På TrueNAS kører en container, der er gendannet ved siden af originalen, som en almindelig Docker-container, som skærmen Apps ikke viser.

Drevet af [restic](https://restic.net), så hver sikkerhedskopi er dedupliseret, inkrementel og krypteret som standard.

!!! note "Hold din APP_KEY sikker"
    BombVault udleder restic-repositoriets adgangskode fra en 32-byte hemmelighed ved navn `APP_KEY`. Mister du den sammen med gendannelseskittet, kan krypterede sikkerhedskopier ikke gendannes. Generer en med `openssl rand -hex 32`, og gem den et sikkert sted. Se [Konfiguration](configuration.md).

## Hvad BombVault beskytter {#what-bombvault-protects}

Hver container, VM, mappesæt, flashdrevet, hvert ZFS-datasæt og BombVaults egne indstillinger er et **element** med en side for sig. Elementer af samme slags danner et **område**, som du slår til under **Indstillinger, Generelt**.

| Område | Hvad der gemmes |
|---|---|
| **Containers** | Appdata-mappen plus containerdefinitionen (image, env-variabler, porte, labels, volumener). |
| **VMs** | VM-diskimage(s), XML-definitionen og UEFI NVRAM, sikkerhedskopieret over SSH (ingen libvirt-montering). |
| **Flash** | Hele USB-flashen (`/boot`): OS, licens, array-konfiguration, shares, netværks- og plugin-konfiguration. |
| **Auto-sikkerhedskopi** | BombVaults egen `/config`: dens indstillingsdatabase, logindene til dens destinationer, libvirt-SSH-nøgleparret, nøglen og de gemte værtsnøgler til ZFS-serverne og certifikaterne. |
| **Mapper** | En hvilken som helst mappe på serveren som et mappesæt, hvert med valgfrie udelukkelsesmønstre. |
| **ZFS-datasæt** | Et datasæt med alle datasæt under det, læst fra ét ZFS-snapshot og gemt som en mappe. Se [ZFS-datasæt](zfs-datasets.md). |

## Gendannelse er stjernen {#restore-is-the-star}

Efter at data er kopieret tilbage fra restic-sikkerhedskopien, afspiller BombVault den gemte containerdefinition mod Docker-API'en, så containeren dukker op igen i Unraids Docker-fane, som om den altid havde været der (samme image, samme indstillinger, samme portmapninger). VM'er får deres XML defineret på ny over SSH og deres diske og UEFI NVRAM tilkoblet igen, selv efter at VM'en er blevet slettet.

Ét vindue gendanner alle slags. Som standard vælger det **Ved siden af originalen**, som bringer sikkerhedskopien tilbage som en kopi og lader originalen være. **Erstat originalen** tager først en sikkerhedskopi af den aktuelle tilstand, så en gendannelse kan fortrydes. En mappe eller et nyt ZFS-datasæt er de andre mål.

Når en sikkerhedskopi stopper afhængige containere, kommer de tilbage i den rigtige rækkefølge: BombVault genstarter dem i deres Compose `depends_on`-rækkefølge og venter på, at hver enkelt rapporterer som sund, før de containere, der afhænger af den, startes, så intet skynder sig frem forbi en database eller en gateway, der endnu ikke er oppe. Se [Gendannelse](restore.md) og [Kommandoer og andre containere](backups.md#commands).

## Sådan virker det {#how-it-works}

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

BombVault bruger Docker-socket'en til at stoppe containere før en sikkerhedskopi og genskabe dem efter en gendannelse. Til VM'er kører den `virsh` på værten over SSH (`qemu+ssh://`) for at lukke en VM pænt ned eller tage et live-snapshot. Den bind-monterer aldrig en libvirt-sti, så den kan ikke komme i vejen for VM Manager på værten.

BombVault er orkestrerings- og UI-laget, ikke lagringsmotoren. Al faktisk dataflytning går gennem restic.

## Hurtig start {#quick-start}

Ny her? Gå til **[Kom godt i gang](getting-started.md)** for at installere BombVault og køre din første sikkerhedskopi, og opsæt derefter **[Off-site og gendannelse](offsite-recovery.md)**. De **[gennemgåede eksempler](guides.md)** følger de almindelige jobs fra start til slut, siden **[Indstillinger](settings.md)** viser hver flise, og **[Funktioner](features.md)** er det korte katalog over alt, BombVault kan.

Hvert område kan kopiere til flere destinationer på én gang, en parret BombVault kan opbevare dine off-site-kopier på en server, der afviser sletninger, og **Nødguide** henter en tabt server tilbage fra dens gendannelseskit, gennem din gruppe eller med den indtastede nøgle. Brug kortet **Eksportér / importér indstillinger**, hvis du vil flytte din konfiguration til en ny boks. Se [Off-site og gendannelse](offsite-recovery.md) og [Konfiguration](configuration.md#portable-settings-export-and-import).

**[Android-appen](android.md)** giver dig alle servere i din gruppe på din telefon, med aktivitetsloggen for dem alle på én skærm.

Dokumentationen findes på 26 sprog og brugerfladen på 42. Sider, der endnu ikke er oversat, vises på engelsk.

## Ord brugt her {#glossary}

Nogle få ord har en fast betydning i BombVault og på disse sider.

| Ord | Betydning |
|---|---|
| **Element** | Én ting, BombVault sikkerhedskopierer: en container, en VM, et mappesæt, flashdrevet, et ZFS-datasæt med dem under det eller dets egne indstillinger. Hvert har en side for sig. |
| **Område** | Alle elementer af én slags, for eksempel Containers. Du slår områder til under **Indstillinger, Generelt**. |
| **Destination** | Et sted, sikkerhedskopier havner: en mappe på denne server, en share, en server, du selv ejer, eller en cloudtjeneste. Se [Destinationer](offsite-recovery.md#destinations). |
| **Lokal** | Destinationen på denne server, der rummer områdernes mapper. |
| **Egen mappe** | Det repository, et område først skriver sine sikkerhedskopier til. Det ligger som standard på denne server og kan ligge på en destination. Se [Et områdes egen mappe](offsite-recovery.md#own-folder). |
| **Kopi** | En sikkerhedskopi, der er kopieret fra egen mappe til en anden destination med `restic copy`. |
| **Direkte repository** | Et repository på en off-site-destination, som elementer sikkerhedskopierer direkte til uden en lokal kopi imellem. Se [Direkte repositories](offsite-recovery.md#direct-repositories). |
| **Adgangsoplysninger** | Den gemte nøgle eller adgangskode til en S3-tjeneste eller en rest-server. Én af dem er standard. Se [Adgangsoplysninger](offsite-recovery.md#sign-ins). |
| **Placering** | Hvilke destinationer der får et elements sikkerhedskopier, sat på elementet eller som dets områdes standard. Se [Placering pr. element](offsite-recovery.md#placement). |
| **Replika** | En blokpræcis kopi af et ZFS-element på en anden ZFS-server, lavet med `zfs send`. Den står ved siden af sikkerhedskopien og erstatter den ikke. Se [Replika](zfs-datasets.md#replica). |
| **Rolle** | Noget, en instans i din gruppe gør for en anden: Modtager, Henter, ZFS-server eller Nøgleholder. Se [Instanser og roller](offsite-recovery.md#roles). |

## Tak til {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** af [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) gav BombVault dets udgangspunkt: sikkerhedskopiering med ét klik og automatisk geninstallation af Docker-containere. BombVault er en selvstændig implementering i Go og restic, der fører idéen videre til VM'er, flashen og mere.
- **[restic](https://restic.net/)** er den hurtige, sikre, deduplikerende sikkerhedskopimotor, som BombVault styrer.
- **[rclone](https://rclone.org/)** leverer cloud-backends.
- De fleste symboler på knapper kommer fra det gratis Core Solid-sæt fra **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [kilde](https://github.com/webalys-hq/streamline-vectors)). Resten kommer fra Font Awesome Free, Material Design Icons, Simple Icons og Tabler Icons eller er tegnet til projektet.

## Licens {#license}

Copyright (C) 2026 Junker der Provinz. BombVault er fri software under **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Du må køre, studere, dele og ændre det. Hvis du distribuerer det eller kører en ændret version som en netværkstjeneste, skal du offentliggøre din kildekode under samme licens og bevare de eksisterende ophavsrets- og kildeangivelser.

Navnet og brandingen er ikke licenseret. AGPL dækker kun kildekoden: "BombVault", dets logo og dets branding forbliver forbeholdt, så en fork skal bruge sit eget navn og sin egen branding og må ikke udgive sig for at være BombVault.

## Links {#links}

- **Kildekode:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid-supporttråd:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issues:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-ækvivalent kontrol over værten"
    Gennem Docker-socket'en kan BombVault stoppe, fjerne og genoprette containere og læse/skrive appdata, og til VM-sikkerhedskopiering logger den ind på værten over SSH for at køre `virsh`. Enhver, der kan nå dens web-UI, har reelt root på værten. Kør kun BombVault på et betroet, ikke-eksponeret netværk, og angiv en login-adgangskode under **Indstillinger, Sikkerhed**. Gendannelseskittet, nøgledeponeringer, flash-ZIP'en, eksport af en flash-sikkerhedskopi eller auto-sikkerhedskopi og sletning af sikkerhedskopier i en destination kræver en. Se [Konfiguration](configuration.md#security-model) for den fulde sikkerhedsmodel.
