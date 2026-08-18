# Konfiguration

Denne side dækker containerens miljøvariabler, de monteringer skabelonen leverer, VM-sikkerhedskopiering over SSH og off-site-opsætningen. **Repository-stier** for sikkerhedskopier konfigureres inde i appen (Indstillinger, Sikkerhedskopistier), ikke via miljøvariabler.

## Miljøvariabler

| Variabel | Påkrævet | Beskrivelse |
|---|---|---|
| `APP_KEY` | **Ja** | 32-byte hex-hemmelighed (64 hex-tegn), der bruges til at udlede restic-repoets adgangskode. Generer med `openssl rand -hex 32`. Hold den sikker: mister du den, kan krypterede sikkerhedskopier ikke gendannes. |
| `LIBVIRT_HOST` | Til VM'er | Unraid-vært nået over SSH til VM-sikkerhedskopiering (default `host.docker.internal`; skabelonen forudfylder en LAN-IP-pladsholder). Brug din Unraid LAN-IP, påkrævet på et brugerdefineret `br0.x`-netværk. |
| `LIBVIRT_SSH_PORT` | Nej | Værts-SSH-port til VM-sikkerhedskopiering (default `22`). |
| `LIBVIRT_SSH_USER` | Nej | SSH-bruger på værten til VM-sikkerhedskopiering (default `root`). |
| `LIBVIRT_URI` | Nej | Fuld libvirt-forbindelses-URI, brugt **ordret** i stedet for at bygge en ud fra de tre `LIBVIRT_*`-variabler ovenfor (som så ignoreres for forbindelsesstrengen). Default usat. Nødvendig på TrueNAS Scale, hvis libvirtd lytter på en ikke-standard socket, som den byggede streng-form ikke kan udtrykke: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Se TrueNAS Scale-afsnittet i [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). |
| `PORT` | Nej | HTTP-port (default `3000`; kun brugt med `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Nej | HTTPS-port (default `3443`; skabelonen publicerer den 1:1, så WebUI'en svarer på `https://<ip>:3443`). |
| `HTTP_ONLY` | Nej | Sæt `true` for at deaktivere den selvsignerede HTTPS-lytter og kun servere almindelig HTTP (til brug bag en TLS-terminerende reverse proxy). |
| `HOST_SOURCE_ROOT` | Nej | Værtsstien monteret som **Host Data** (default `/mnt`). BombVault oversætter de bind-mount-kilder, Docker rapporterer, til stier under denne montering. Skift kun, hvis du monterede en anden værtsrod. |
| `DATA_ROOT_SEGMENTS` | Nej | Kommaseparerede sti-segmentnavne, der markerer en bind-mount-kilde som sikkerhedskopidata (default `appdata`, svarende til Unraids `/mnt/user/appdata/<container>`-konvention). En containers bind-mount auto-vælges til sikkerhedskopiering, når ETHVERT angivet segment optræder som et helt sti-segment i dens værtskilde, for eksempel vælger `DATA_ROOT_SEGMENTS=appdata,config` også en `.../config`-bind. Se [Registrering af sikkerhedskopikilder](#backup-source-detection) for de andre, altid aktive måder, en containers datamappe findes på. |
| `PLATFORM` | Nej | Tvinger hvilken platform BombVault opfatter sig selv som kørende på, i stedet for at auto-detektere det: `unraid`, `generic` eller `truenas` (default usat: auto-detekterer Unraid ved at probe efter dens `dockerMan`-markør under flash-monteringen, ellers `generic`; en ukendt værdi falder også tilbage til `generic`, logget). Sæt den eksplicit på en generisk Docker-vært eller TrueNAS Scale i stedet for at stole på auto-proben, der kun virker på Unraid: den generiske compose-fil gør netop det. Ændrer appdata-fallback-konventionen, standardværdierne for gendannelsesdestination på tværs af instanser, og om notifikations-/ledsager-plugin-trinnene, der kun findes på Unraid, overhovedet forsøges (se `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Nej | Navnet på selve BombVault-containeren, så den aldrig sikkerhedskopierer (og dermed stopper) sig selv (default `BombVault`; auto-detekteret via værtsnavnet på bridge-netværk). |
| `BACKUP_MAX_HOURS` | Nej | Maksimalt antal vægur-timer, en enkelt sikkerhedskopikørsel må holde sin domænelås, før den tvangsannulleres (en beskyttelse, så en fastlåst kørsel ikke kan blokere domænet for evigt). Tom (default) bruger `48`. Hæv den til meget store eller langsomme cloud-sikkerhedskopier (en kørsel annulleret ved grænsen fejler med `context deadline exceeded`). Sæt `0` for at deaktivere grænsen helt. |
| `TZ` | Nej | Tidszone for planlæggeren (for eksempel `Europe/Berlin`). |

## Monteringer

Montér Docker-socket'en, flashen (`/boot`) og **Host Data**-roden (`/mnt`) som vist i CA-skabelonen. Både *kilder* og *destinationer* for sikkerhedskopier lever under Host Data, og den er monteret **rslave**, så en remote share, der monteres, efter containeren er startet (for eksempel under `/mnt/remotes`), bliver synlig uden en genstart.

Repository-stier for sikkerhedskopier defaulter til `/mnt/user/bombvault/{container,vms,flash,config,files}`, oprettet ved den første sikkerhedskopi. Skift placeringen når som helst i **Indstillinger, Sikkerhedskopistier**.

!!! note "Vært-integrationstjek"
    Åbn `/spike` i web-UI'en, når containeren er startet. Den prober hver montering og hvert CLI (Docker-socket, libvirt, restic, qemu-img, rclone) og rapporterer eventuelle manglende dele.

## Sikkerhedsmodel

!!! warning "Root-ækvivalent kontrol over værten"
    Gennem Docker-socket'en kan BombVault stoppe, fjerne og genoprette containere og læse/skrive appdata, og til VM-sikkerhedskopiering logger den ind på værten over SSH (`qemu+ssh://`, root som standard) for at køre `virsh`. Enhver, der kan nå dens web-UI, har reelt root på værten.

- **Valgfri adgangskodebeskyttelse** (Indstillinger, Sikkerhed): sæt en adgangskode for at kræve login, ryd den for at deaktivere. Som standard fra til betroet-LAN-brug. Sessioner er signeret (HMAC afledt af `APP_KEY`), og at ændre adgangskoden ugyldiggør dem; logins er rate-begrænsede.
- Fordi sikringen er et tilvalg, er hele UI'en og API'en (inklusive off-site-opsætningen, manipulationstest-ruterne og gendannelseskittet), når den er usat, tilgængelige for enhver, der kan nå porten. Aktivér sikringen, når off-site, uforanderlige sikkerhedskopier eller kryptering er i brug.
- Kør kun BombVault på et betroet, ikke-eksponeret netværk. For fjernadgang, sæt den bag en reverse proxy, der tilføjer autentificering og TLS. Svar bærer grundlæggende sikkerhedsheaders (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Med `HTTP_ONLY=true` mister session-cookien sit `Secure`-flag (det er nødvendigt for at virke over almindelig HTTP), så aktivér kun adgangskoden bag en TLS-terminerende proxy, hvis fortrolighed betyder noget.
- VM-sikkerhedskopiets SSH-forbindelse stoler på værtsnøglen ved første forbindelse (TOFU) og pinner den derefter. Verificér værtens nøgle out-of-band, hvis din container-til-vært-sti ikke er betroet.
- Sikkerhedskopier er krypteret af restic, når kryptering er aktiveret (Indstillinger; som standard til), med nøglen afledt af `APP_KEY`.

## VM-sikkerhedskopiering over SSH

BombVault sikkerhedskopierer KVM/libvirt-VM'er **uden at montere nogen libvirt-sti**. Den kører `virsh` på værten over SSH (`qemu+ssh://`), så den aldrig kan påvirke din værts-VM Manager.

Hurtig opsætning:

1. **Indstillinger, System, VM Backup over SSH:** kopiér den viste offentlige nøgle.
2. Tilføj den til Unraids `/root/.ssh/authorized_keys` (også persisteret til flashen, så den overlever genstarter).
3. Klik på **Test connection**.

Skabelonen tilføjer `--add-host=host.docker.internal:host-gateway`, så containeren kan nå værten. Sæt `LIBVIRT_HOST` til din Unraid LAN-IP, hvis det navn ikke resolverer (for eksempel når containeren kører på et brugerdefineret `br0.x`-netværk). Hvis du ændrede Unraids SSH-port, så sæt `LIBVIRT_SSH_PORT` til at matche. **Live-øjebliksbilleder** kræver derudover qemu guest agent i VM'en og disken på `/mnt/cache` (ikke `/mnt/user`).

!!! important "Fuld VM-opsætning og netværksguide"
    Den komplette trin-for-trin-guide (SSH-aktivering, persistent nøgleautorisering, brugerdefineret-netværks- og VLAN-routing, metode pr. VM og fejlfinding på værtssiden) findes på [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) på GitHub.

## Off-site-opsætning

Opsæt en off-site-replika på fanen **Indstillinger, Off-site**. Se [Off-site og gendannelse](offsite-recovery.md) for det fulde arbejdsforløb (uforanderlig/append-only, manipulationstest og DR-øvelser). Kort sagt:

- **Backends:** SMB/CIFS og NFS (montér share'en, og peg en Backup Path mod den), native restic-backends uden rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`) eller en hvilken som helst rclone-remote (`rclone:<remote>:<bucket>/path`).
- **Cloud-legitimationsoplysninger** gemmes krypteret under Indstillinger, Off-site, Cloud credentials.
- **SSH-destinationer kræver intet installeret på den anden side.** `sftp:` kræver kun en SSH-server. Tilføj den offentlige nøgle fra **Indstillinger, System, VM Backup over SSH** (også på `/config/ssh/id_ed25519.pub`) til destinationsbrugerens `~/.ssh/authorized_keys`.
- **Off-site-kopi:** BombVault replikerer nye øjebliksbilleder med `restic copy` på et best-effort-grundlag. Det lokale repo forbliver primært. Hvert domæne har sin egen off-site-tidsplan plus en **Replikér nu**-knap.
- **Flere off-site-destinationer pr. domæne:** hvert domæne kan replikere til flere off-site-destinationer på én gang. Tilføj ekstra destinationer på Indstillinger, Off-site, hver med sit eget repository, sin S3-lagringsklasse, sit append-only-flag, sin opbevaring og sit vækstbudget; de replikerer alle på det domænes off-site-tidsplan. En eksisterende enkelt off-site-opsætning overføres som den første destination.
- **Opbevaring pr. kilde:** den lokale politik lever på Indstillinger, Stier og lagring; off-site-politikken på Indstillinger, Off-site (lad den stå helt-nul for aldrig at auto-trimme off-site-øjebliksbilleder).
- **Båndbreddegrænser:** begræns restic-upload/download-hastigheden under Indstillinger, Off-site.
- **Kold- og arkivlagringsklasse (S3):** for et native S3 off-site-repo, vælg et gendannelses-læsbart niveau (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). rclone-remotes sætter deres klasse i rclone-konfigurationen.

## Bærbare indstillinger (eksportér og importér) {#portable-settings-export-and-import}

Kortet **Eksportér og importér indstillinger** på Indstillinger-siden skriver hele din BombVault-konfiguration (domæneindstillinger, off-site-destinationer, tidsplaner, opbevaring, notifikationer) til en bærbar JSON-fil, du kan importere på en anden instans, så et flyt til en ny boks eller kloning af en opsætning ikke betyder at genindtaste alt manuelt. Import viser en forhåndsvisning og beder om bekræftelse, og den rører aldrig dine sikkerhedskopidata eller -historik.

!!! warning "Eksporten kan indeholde legitimationsoplysninger"
    Du vælger, om off-site- og notifikations-legitimationsoplysninger skal medtages i filen. Med legitimationsoplysninger medtaget er eksporten lige så følsom som dit gendannelseskit, så opbevar den et sikkert sted. Uden dem indeholder filen kun ikke-hemmelige indstillinger.
