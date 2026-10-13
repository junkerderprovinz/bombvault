# Kom godt i gang

Denne side fører dig fra en frisk Unraid-boks til din første sikkerhedskopi.

## Krav

| Krav | Bemærkninger |
|---|---|
| **Unraid 6.12+** | Tidligere versioner er ikke testet. Unraid er hovedmålet, men BombVault kører også på en almindelig Docker-vært og på TrueNAS Scale (se [Generisk Docker-vært](#generic-docker-host)). |
| **Restic-repo-placering** | En lokal sti (anbefalet: dit array eller cache), SMB, NFS eller en hvilken som helst rclone-backend. |
| **Docker-socket** | Monteres automatisk af skabelonen (`/var/run/docker.sock`). |
| **Unraid-flash** (`/boot`) | Monteres i sin helhed automatisk af skabelonen (`/boot` til `/host/boot`). Driver flash-sikkerhedskopiering og lader en gendannet container dukke op igen som en normal, redigerbar Unraid-app. |
| **KVM-VM'er** (tilvalg) | VM-sikkerhedskopiering taler med libvirt over SSH, ingen libvirt-montering. Sæt det op under **Indstillinger, Integrationer, Værts-SSH** (se [Konfiguration](configuration.md)). |
| **ZFS-datasæt** (tilvalg) | Den samme SSH-forbindelse som til VM-sikkerhedskopier, `zfs` på værten og Host Data tilknyttet som `/mnt` med adgangstilstanden Read/Write - Slave, skabelonens standard. Se [ZFS-datasæt](zfs-datasets.md). |
| **Android-app** (valgfri) | Android 10 eller nyere, parret med servere på version 9.7.0 eller nyere. Se [Android-app](android.md). |

## Installer på Unraid

Den nemmeste vej er **Community Applications**.

1. Åbn fanen **Apps** i Unraid.
2. Søg efter **BombVault**.
3. Klik på **Install**, angiv de nødvendige variabler (nedenfor), og anvend.

!!! tip "Manuel skabeloninstallation"
    Hvis du foretrækker at tilføje skabelonen manuelt:

    1. Gå til **Docker, Add Container, Template repositories**, og tilføj:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Søg efter **BombVault** under Templates.
    3. Angiv de nødvendige variabler, og klik på **Apply**.

## Generisk Docker-vært {#generic-docker-host}

Ikke Unraid? BombVault kører også som almindelig container på enhver Docker-vært (det er også det, der bærer containerunderstøttelsen på TrueNAS Scale, forud for en egen post i dens app-katalog).

1. Hent den redigeringsklare [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) fra arkivet.
2. Sæt `APP_KEY` (se nedenfor) og peg Host Data-diskenheden på din rigtige datarod. Kommentarerne i filen gennemgår begge dele.
3. `docker compose up -d`, åbn derefter `https://<vært-ip>:3443/`.

Hvad der er anderledes end på Unraid:

- **Intet Flash-område.** Der er ingen boot-USB at indfange eller genskabe, så Flash-området har intet at lave her. I stedet tilbyder **Tilføj** på siden Sikkerhedskopier **Systemkonfiguration** som mappeforslag: et mappesæt med `/etc` til at begynde med, som du gennemgår og retter, før du gemmer.
- **Ingen Unraid-egne meddelelser.** BombVaults egne meddelelseskanaler (webhook, advarsler om mislykket off-site og lignende) virker som sædvanlig; kun den Unraid-specifikke besked til dens eget meddelelsessystem udelades, da et sådant system ikke findes her.
- **Sikkerhedskopiering af virtuelle maskiner er valgfri og kræver en separat libvirtd-vært, der kan nås over SSH.** Se den udkommenterede blok i compose-filen. En generisk Docker-vært har ingen indbygget VM-styring.
- **Ingen dashboard-widget.** BombVault Widget er et Unraid-plugin, så det trin springes også over.
- **At finde en containers data.** Uden Unraids `appdata`-konvention findes en containers datamappe ud fra segmenterne i `DATA_ROOT_SEGMENTS`, navngivne Docker-volumener, et Compose-projekts arbejdsmappe og labelen `bombvault.data` (se [Genkendelse af sikkerhedskopiernes kilder](configuration.md#backup-source-detection)). Navngivne volumener og `/etc`-forudindstillingen når kun stier inden for Host Data-monteringen, så peg Host Data på en fælles overordnet mappe, der også dækker Dockers datarod.
- **`PLATFORM`.** Sæt den til `generic` eller `truenas`. Uden værdi genkender BombVault Unraid på dets egen markør på flash-monteringen og behandler alt andet som generisk, og de trin, der kun gælder Unraid, springes over i stedet for at blive forsøgt og fejle.

**TrueNAS Scale** følger den samme compose-vej; en katalogpost er forberedt i repositoriet, men endnu ikke indsendt. VM-sikkerhedskopiering kræver dér `LIBVIRT_URI`, fordi TrueNAS' libvirtd lytter på sin egen socket (`/run/truenas_libvirt/libvirt-sock`), som de tre `LIBVIRT_*`-variabler ikke kan udtrykke (se [Konfiguration](configuration.md)). Så langt er det afprøvet: zvol-sikkerhedskopien blev kørt mod en rigtig TrueNAS Scale-boks, på en zvol tilsluttet en kørende VM, og `zfs snapshot`, `zfs send`, restic og `zfs receive` førte den frem og tilbage byte for byte. En komplet gendannelse styret af BombVault selv er endnu ikke kørt på TrueNAS-hardware, og den zvol var tyndt allokeret, så gennemstrømningen ved mange gigabyte er ikke testet. Test en gendannelse dér, før du stoler på den.

## Den ene påkrævede indstilling

Den eneste variabel, du skal angive, er `APP_KEY`, en 32-byte hex-hemmelighed (64 hex-tegn), der bruges til at udlede restic-repositoriets adgangskode.

Generer en på en hvilken som helst maskine:

```bash
openssl rand -hex 32
```

Indsæt resultatet i skabelonens `APP_KEY`-felt (Unraid) eller i miljøvariablen `APP_KEY` i `docker-compose.yml` (generisk Docker-vært).

!!! danger "Mist ikke din APP_KEY"
    Mister du `APP_KEY`, kan dine krypterede sikkerhedskopier ikke gendannes. Opbevar den et sikkert sted adskilt fra serveren. Når BombVault kører, skal du angive en login-adgangskode og hente **Gendannelseskit** under **Indstillinger, Gendannelse** (se [Off-site og gendannelse](offsite-recovery.md#recovery-kit)). Det indeholder nøglen, hver destination og restic-kommandoerne.

Skabelonen monterer også Docker-socket'en, flashen (`/boot`) og **Host Data**-roden (`/mnt`) for dig. Både *kilder* og *destinationer* for sikkerhedskopier lever under Host Data. For den fulde variabelreference og off-site-opsætningen, se [Konfiguration](configuration.md).

## Første kørsel

![Oversigten efter en første sikkerhedskopi: hvad der kræver opmærksomhed, beskyttelsen af hvert område, destinationerne og historikken.](assets/screenshots/dashboard.png)

*Oversigten efter en første sikkerhedskopi: hvad der kræver opmærksomhed, beskyttelsen af hvert område, destinationerne og historikken.*

1. Åbn web-UI'en på `https://<your-unraid-ip>:3443` (selvsigneret certifikat fra start). En ny installation åbner på **Velkommen til BombVault**, som oplister, hvad BombVault fandt på serveren.
2. Vælg **Kom i gang**. Oversigten viser så kortet **Første trin** med tre trin:
    1. **Vælg områder**: slå til, hvad BombVault sikkerhedskopierer (Containers, VMs, Flash, Mapper, ZFS-datasæt, Auto-sikkerhedskopi). **Vælg** åbner listen over et områdes elementer, hvor **Sæt tidsplan på pause** udelader ét. De områder, du slår til her, kører hver nat, hvilket du ændrer under **Indstillinger, Tidsplan**.
    2. **Tilføj destination**: hvor sikkerhedskopierne havner. En lokal mappe er nok til at begynde med. Sikrest er en ekstra destination uden for huset, for eksempel en Storage Box eller Backblaze B2. Lokale repositories har som standard `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` og oprettes ved den første sikkerhedskopi.
    3. **Første sikkerhedskopi**: derefter kører det af sig selv hver nat.
3. Åbn **Sikkerhedskopier** for at se alle elementer på én liste. **Sikkerhedskopiér alt nu** starter en kørsel over alle områder, der er slået til, og **Sikkerhedskopier nu** på et elements side tager kun det ene.

Har du brugt BombVault før? **Gendan fra en sikkerhedskopi** på velkomstsiden åbner siden Gendannelse, hvor **Nødguide** henter indstillinger, containere, VM'er og mapper fra en tabt server tilbage. Se [Off-site og gendannelse](offsite-recovery.md#emergency).

!!! tip "Valgfrit: vælg en sikkerhedskopi-rækkefølge"
    Hvis nogle containere altid skal sikkerhedskopieres før andre (for eksempel en database før den app, der bruger den), så åbn kortet **Rækkefølge** under **Indstillinger, Tidsplan**, og træk dem ind i den ønskede rækkefølge. Planlagte kørsler og den fulde sikkerhedskopi følger den derefter; alt, du lader stå uordnet, sikkerhedskopieres mest-overskredet-først.

!!! note "Vært-integrationstjek"
    Kortet **Vært-integration** på oversigten prober hver montering og hvert CLI (Docker-socket, libvirt, restic, qemu-img, rclone) og rapporterer eventuelle manglende dele, så du kan bekræfte, at containeren er korrekt forbundet, før du forlader dig på den. Det venter under **Tilpas**, til du tilføjer det.

## Find rundt {#navigation}

![Indstillinger som felter, med kortene for det valgte felt ved siden af. Hver ændring skrives, mens du laver den.](assets/screenshots/settings.png)

*Indstillinger som felter, med kortene for det valgte felt ved siden af. Hver ændring skrives, mens du laver den.*

Sidepanelet har fem punkter: **Oversigt**, **Afvigelser** (så længe afvigelsesgenkendelse er slået til), **Sikkerhedskopier**, **Gendannelse** og under **Log ud** **Indstillinger**. En sides hovedhandling flyder nederst til højre, og **Mere** rummer de sjældne. På en telefon rummer bjælken Oversigt, Sikkerhedskopier, Gendannelse og Indstillinger, og **Mere** har resten.

- **Sikkerhedskopier** oplister hver container, VM, hvert mappesæt, flashdrevet, hvert ZFS-datasæt og auto-sikkerhedskopien som elementer. Felter over listen vælger en slags, og **Filtre**, **Sortér** og **Søg** indsnævrer den. Hvert element har en side for sig med tre faner: **Sikkerhedskopier**, **Indstillinger** og **Verificering**.
- **Gendannelse** er én side til alt: vælg et område, søg efter et element, og vælg en sikkerhedskopi. Nødguiden og de andre kilder starter også her.
- **Indstillinger** er fjorten felter: Generelt, Udseende, Tidsplan, Destinationer, Gendannelse, Notifikationer, Verificering, Sikkerhed, Instanser, Parring, Integrationer, Apps, System og Info. Tryk på `/` eller Ctrl+K for at søge i dem alle, også efter de navne, siderne havde i tidligere versioner. Træk felterne i din egen rækkefølge under **Udseende**.
- **Destinationer** og **Instanser** er indstillingsfelter med sider for sig. **Vis også i sidepanelet** på deres felt fæstner dem til sidepanelet.

Intet har en Gem-knap, og intet folder sig ud: sjældne ting har et kort eller et vindue for sig, og forklaringer står i (i)-boblerne. Efter en opdatering markerer grønne prikker, hvad der er ændret. Et klik skjuler en prik, og **Nyt** i kortet Version under **Indstillinger, Info** oplister hver ændring med **Gå derhen**.

## Byg fra kildekoden {#build-from-source}

BombVault er én statisk Go-binærfil, der leverer en JSON-API og en indlejret React-grænseflade. Byg grænsefladen først, og kør derefter binærfilen:

```bash
npm --prefix web ci
npm --prefix web run build     # skriver web/dist, som binærfilen indlejrer
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # enheds- og integrationstest, med en rigtig restic-tur frem og tilbage
golangci-lint run ./...
go run ./cmd/bombvault         # leverer https://localhost:3443 med et selvsigneret certifikat
```

Grænsefladebygget er også nødvendigt for `go run`. Repositoriet indeholder kun en tom markør under `web/dist`, så uden `npm --prefix web run build` indlejrer binærfilen intet og svarer `500 SPA index not found`, hvilket er forventet. Docker, libvirt og Unraid kan ikke testes i CI, så kontrollér monteringer, restic og VM-SSH-forbindelsen på en rigtig vært med kortet **Vært-integration** på oversigten (eller `GET /api/spike`), før du åbner en pull request.

## Næste skridt

- Gennemse de fulde **[Funktioner](features.md)**.
- Få alle servere i din gruppe på din telefon med **[Android-appen](android.md)**.
- Tilføj en eller flere destinationer uden for huset (hvert område kan kopiere til flere på én gang), og gem dit gendannelseskit, som beskrevet under **[Off-site og gendannelse](offsite-recovery.md)**.
- Kloner du en opsætning eller flytter til en ny boks? Bær hele din konfiguration over med kortet **Eksportér / importér indstillinger**. Se [Konfiguration](configuration.md#portable-settings-export-and-import).
- Løb ind i et problem? Se **[Fejlfinding](troubleshooting.md)**.
