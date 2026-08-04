# BombVault

**Dine Unraid-data, forseglet i en boks. Slip en sikkerhedskopi. Detonér en gendannelse.**

BombVault er en selvhostet, Unraid-native webapp til **sikkerhedskopiering og fuld katastrofegendannelse** af dine Docker-containere og KVM/libvirt-VM'er. Den kører som en enkelt multi-arch Docker-container, giver dig en moderne mørk web-UI og håndterer hele livscyklussen: sikkerhedskopier, planlæg, verificer og gendan.

Gendannelser sker automatisk. Containere dukker op igen i Unraids Docker-fane præcis som før, og VM'er bliver defineret på ny i VM Manager med deres diske og UEFI NVRAM tilkoblet igen. Ingen manuel geninstallation, ingen omkonfiguration, intet drama.

Drevet af [restic](https://restic.net), så hver sikkerhedskopi er dedupliseret, inkrementel og altid krypteret.

!!! note "Hold din APP_KEY sikker"
    BombVault udleder restic-repositoriets adgangskode fra en 32-byte hemmelighed ved navn `APP_KEY`. Mister du den, kan krypterede sikkerhedskopier ikke gendannes. Generer en med `openssl rand -hex 32`, og gem den et sikkert sted. Se [Konfiguration](configuration.md).

## Hvad BombVault beskytter

| Domæne | Hvad der gemmes |
|---|---|
| **Docker-containere** | Appdata-mappen plus containerdefinitionen (image, env-variabler, porte, labels, volumener). |
| **KVM / libvirt-VM'er** | VM-diskimage(s), XML-definitionen og UEFI NVRAM, sikkerhedskopieret over SSH (ingen libvirt-montering). |
| **Unraid-flash** | Hele USB-flashen (`/boot`): OS, licens, array-konfiguration, shares, netværks- og plugin-konfiguration. |
| **App-konfiguration** | BombVaults egen `/config`: dens indstillingsdatabase, off-site-legitimationsoplysninger og libvirt-SSH-nøgleparret. |
| **Filer og mapper** | Navngivne **filsæt**, en hvilken som helst mappe på serveren, hver med valgfrie udelukkelsesmønstre pr. sæt. |

## Gendannelse er stjernen

Efter at data er kopieret tilbage fra restic-øjebliksbilledet, afspiller BombVault den gemte containerdefinition mod Docker-API'en, så containeren dukker op igen i Unraids Docker-fane, som om den altid havde været der (samme image, samme indstillinger, samme portmapninger). VM'er får deres XML defineret på ny over SSH og deres diske og UEFI NVRAM tilkoblet igen, selv efter at VM'en er blevet slettet.

Når en sikkerhedskopi stopper afhængige containere, kommer de tilbage i den rigtige rækkefølge: BombVault genstarter dem i deres Compose `depends_on`-rækkefølge og venter på, at hver enkelt rapporterer som sund, før de containere, der afhænger af den, startes, så intet skynder sig frem forbi en database eller en gateway, der endnu ikke er oppe. Se [Funktioner](features.md).

## Sådan virker det

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

BombVault er orkestrerings- og UI-laget, ikke lagringsmotoren. Al faktisk dataflytning går gennem restic.

## Hurtig start

Ny her? Gå til **[Kom godt i gang](getting-started.md)** for at installere BombVault på Unraid via Community Applications og køre din første sikkerhedskopi. Udforsk derefter de fulde **[Funktioner](features.md)**, tilpas din **[Konfiguration](configuration.md)**, og opsæt **[Off-site og gendannelse](offsite-recovery.md)**.

Off-site kan fordele til flere destinationer pr. domæne på én gang, et skrivebeskyttet **modtager-dashboard** overvåger disse kopier på den boks, der modtager dem, og du kan bære hele din konfiguration over til en ny boks med kortet **Eksportér og importér indstillinger**. Se [Off-site og gendannelse](offsite-recovery.md) og [Konfiguration](configuration.md#portable-settings-export-and-import).

## Links

- **Kildekode:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid-supporttråd:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issues:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-ækvivalent kontrol over værten"
    Gennem Docker-socket'en kan BombVault stoppe, fjerne og genoprette containere og læse/skrive appdata, og til VM-sikkerhedskopiering logger den ind på værten over SSH for at køre `virsh`. Enhver, der kan nå dens web-UI, har reelt root på værten. Kør kun BombVault på et betroet, ikke-eksponeret netværk, og aktivér den valgfrie adgangskodesikring (Indstillinger, Sikkerhed), når off-site- eller uforanderlige sikkerhedskopier er i brug. Se [Konfiguration](configuration.md) for den fulde sikkerhedsmodel.
