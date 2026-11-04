# Kom i gang

Denne siden tar deg fra en ny Unraid-boks til din første sikkerhetskopi.

## Krav {#requirements}

| Krav | Merknader |
|---|---|
| **Unraid 6.12+** | Tidligere versjoner er ikke testet. Unraid er hovedmålet, men BombVault kjører også på en vanlig Docker-vert og på TrueNAS Scale (se [Generisk Docker-vert](#generic-docker-host) og [TrueNAS SCALE](#truenas)). |
| **Plassering for restic-repo** | En lokal sti (anbefalt: array-et eller cachen din), SMB, NFS eller en hvilken som helst rclone-backend. |
| **Docker-socket** | Monteres automatisk av malen (`/var/run/docker.sock`). |
| **Unraid-flash** (`/boot`) | Monteres i sin helhet automatisk av malen (`/boot` til `/host/boot`). Driver flash-sikkerhetskopiering og lar en gjenopprettet container dukke opp igjen som en normal, redigerbar Unraid-app. |
| **KVM-VM-er** (valgfritt) | VM-sikkerhetskopiering snakker med libvirt over SSH, ingen libvirt-montering. Sett det opp under **Innstillinger, Integrasjoner, Verts-SSH** (se [Konfigurasjon](configuration.md)). |
| **ZFS-datasett** (valgfritt) | Den samme SSH-forbindelsen som for VM-sikkerhetskopier, `zfs` på verten og Host Data tilordnet som `/mnt` med tilgangsmodusen Read/Write - Slave, malens standard. Se [ZFS-datasett](zfs-datasets.md). |
| **Android-app** (valgfritt) | Android 10 eller nyere, paret med servere på versjon 9.7.0 eller nyere. Se [Android-app](android.md). |

## Installer på Unraid {#install-on-unraid}

Den enkleste veien er **Community Applications**.

1. Åpne **Apps**-fanen i Unraid.
2. Søk etter **BombVault**.
3. Klikk **Install**, sett de påkrevde variablene (nedenfor), og bruk.

!!! tip "Manuell malinstallasjon"
    Hvis du foretrekker å legge til malen manuelt:

    1. Gå til **Docker, Add Container, Template repositories** og legg til:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Søk etter **BombVault** i Templates.
    3. Sett de påkrevde variablene og klikk **Apply**.

## Generisk Docker-vert {#generic-docker-host}

Ikke Unraid? BombVault kjører også som en vanlig container på hvilken som helst Docker-vert. TrueNAS SCALE har egne trinn under [TrueNAS SCALE](#truenas).

1. Hent den redigeringsklare [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) fra arkivet.
2. Sett `APP_KEY` (se nedenfor) og pek Host Data-volumet mot din virkelige datarot. Kommentarene i filen går gjennom begge deler.
3. `docker compose up -d`, åpne deretter `https://<vert-ip>:3443/`.

Hva som er annerledes enn på Unraid:

- **Ikke noe Flash-område.** Det finnes ingen oppstarts-USB å fange inn eller gjenopprette, så Flash-området har ingenting å gjøre her. I stedet tilbyr **Legg til** på siden Sikkerhetskopier, når Host Data når vertens `/etc`, **Systemkonfigurasjon** som mappeforslag: et mappesett med `/etc` å begynne med, som du går gjennom og redigerer før du lagrer.
- **Ingen Unraid-egne varsler.** BombVaults egne varselkanaler (webhook, varsler om mislykket off-site og lignende) virker som vanlig. Bare den Unraid-spesifikke meldingen til dens eget varselsystem utelates, siden et slikt system ikke finnes her.
- **Sikkerhetskopiering av virtuelle maskiner er valgfri og trenger en egen libvirtd-vert som er nåbar over SSH.** Se den utkommenterte blokken i compose-filen. En generisk Docker-vert har ingen innebygd VM-håndtering.
- **Ingen dashboard-widget.** BombVault Widget er en Unraid-plugin, så det trinnet hoppes også over.
- **Finne dataene til en container.** Uten Unraids `appdata`-konvensjon finnes datamappen til en container ut fra segmentene i `DATA_ROOT_SEGMENTS`, navngitte Docker-volumer, arbeidskatalogen til et Compose-prosjekt og etiketten `bombvault.data` (se [Gjenkjenning av sikkerhetskopiens kilder](configuration.md#backup-source-detection)). Navngitte volumer og `/etc`-forhåndsinnstillingen når bare stier innenfor Host Data-monteringen, så pek Host Data mot en felles overordnet mappe som også dekker datarotkatalogen til Docker.
- **`PLATFORM`.** Sett den til `generic` eller `truenas`. Uten verdi kjenner BombVault igjen Unraid på dens egen markør på flash-monteringen og behandler alt annet som generisk, og trinnene som bare gjelder Unraid, hoppes over i stedet for å bli forsøkt og feile.

### TrueNAS SCALE {#truenas}

BombVault er ikke i app-katalogen til TrueNAS ennå, så installer den som en egendefinert app: **Apps, Discover Apps**, deretter **Install via YAML** i menyen ved siden av **Custom App**. YAML-en nedenfor setter det samme som katalogoppføringen i mappen `truenas-apps` i arkivet. Bytt ut `tank` med poolen din, og fyll inn `APP_KEY` slik neste avsnitt beskriver.

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
      # For VM-sikkerhetskopier:
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

Åpne deretter `https://<truenas-ip>:30528/`. Det som står under [Hva som er annerledes enn på Unraid](#generic-docker-host), gjelder også her, og på TrueNAS:

- **Capabilities.** BombVault trenger de sju ovenfor. VM-sikkerhetskopier feiler uten `SETPCAP`, og uten `SYS_CHROOT` stoppes hver container under hele sikkerhetskopien sin i stedet for å bli lest fra et øyeblikksbilde.
- **Host Data** er en vertssti på et datasett, her `/mnt/tank`, montert på samme sti inne i appen. Derfor angir `HOST_SOURCE_ROOT` og `HOST_MOUNT_ROOT` begge den stien. `rslave` sender videre monteringene verten lager senere, og dem trenger sikkerhetskopier av ZFS-datasett.
- **Data i et ixVolume blir ikke sikkerhetskopiert.** En app som har et ixVolume som lagring, lagrer dataene sine under `/mnt/.ix-apps`, utenfor Host Data. BombVault lagrer da bare definisjonen av appen, og kjøringen nevner mappene den utelot. Velg **Host Path**-lagring på et datasett under Host Data for hver app der du vil ha dataene sikkerhetskopiert.
- **Sikkerhetskopier av VM-er og ZFS-datasett** når TrueNAS over SSH som root. Slå på SSH-tjenesten under **System, Services**, og legg den offentlige nøkkelen fra **Innstillinger, Integrasjoner, Verts-SSH** til brukeren root under **Credentials, Users**. VM-sikkerhetskopier krever i tillegg `LIBVIRT_URI` som ovenfor, fordi libvirtd i TrueNAS lytter på sin egen socket. Er verken `LIBVIRT_URI` eller `LIBVIRT_HOST` satt, bruker BombVault appens standard-gateway som vert, siden `host.docker.internal` ikke kan slås opp i en TrueNAS-app. [Veiledningen for VM-sikkerhetskopiering](vm-backup-ssh-setup.md#truenas-scale) går gjennom detaljene, også VM-er på zvols.
- **En ZFS-replika bruker en annen nøkkel.** En replika på en annen ZFS-server logger inn med nøkkelen som replikadialogen viser, ikke med Verts-SSH-nøkkelen. Legg den til på serveren som tar imot.
- **En slettet app kommer tilbake i to trinn.** Installer den på nytt i TrueNAS under samme navn først, noe som oppretter nettverket dens igjen, og gjenopprett deretter dataene i BombVault. Fram til da melder sjekken før gjenopprettingen det manglende nettverket, og gjenopprettingen avviser før den endrer noe.
- **Ved siden av originalen** oppretter en vanlig Docker-container. Den kjører, men skjermen Apps i TrueNAS viser den ikke.
- **Databasedumper** fungerer på datasett-monteringer. En import til en datamappe som er sitt eget datasett, flytter det gamle innholdet til en mappe ved siden av, så datasettet over trenger plass til det.

### ZimaOS, CasaOS og andre verter {#other-hosts}

- **ZimaOS og CasaOS:** legg til [community-butikken](https://github.com/junkerderprovinz/zimaos-apps#install) og installer BombVault fra den. Den kjører der som `PLATFORM=generic` med Host Data på `/DATA`. ZimaOS lagrer appdata i `/DATA/AppData/<app>`, som BombVault finner av seg selv, siden `DATA_ROOT_SEGMENTS` treffer `appdata` uansett store og små bokstaver.
- **MOS:** installer BombVault fra [MOS Hub](https://mos-official.net/apps.html?search=BombVault). Den kjører der som `PLATFORM=generic` med Host Data på `/mnt/cache`.
- **Synology:** lag et prosjekt fra compose-filen i Container Manager (DSM 7.2 eller nyere).
- **Proxmox:** kjør den i en VM eller en LXC-container som har Docker.

Uten kommentarer er den generiske compose-filen kort. Bytt ut `./data` med vertsmappen som har dataene til containerne dine, sett `HOST_SOURCE_ROOT` til den samme stien og fyll inn `APP_KEY`:

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

For å sikkerhetskopiere navngitte Docker-volumer også, monterer du en mappe som har både dataene dine og Dockers datarot (`docker info -f '{{.DockerRootDir}}'`, vanligvis `/var/lib/docker`), slik Unraid-malen monterer hele `/mnt`.

## Den ene påkrevde innstillingen {#the-one-required-setting}

Den eneste variabelen du må sette, er `APP_KEY`, en 32-byte hex-hemmelighet (64 hex-tegn) som brukes til å utlede passordet til restic-repositoriet.

Generer en på en hvilken som helst maskin:

```bash
openssl rand -hex 32
```

Lim resultatet inn i `APP_KEY`-feltet i malen (Unraid), eller i miljøvariabelen `APP_KEY` i `docker-compose.yml` (generisk Docker-vert).

!!! danger "Ikke mist APP_KEY-en din"
    Å miste `APP_KEY` gjør de krypterte sikkerhetskopiene dine umulige å gjenopprette. Oppbevar den et trygt sted og adskilt fra serveren. Når BombVault kjører, sett et påloggingspassord og last ned **gjenopprettingssettet** under **Innstillinger, Gjenoppretting** (se [Ekstern lagring og gjenoppretting](offsite-recovery.md#recovery-kit)). Det inneholder nøkkelen, hvert mål og restic-kommandoene.

Malen monterer også Docker-socketen, flashen (`/boot`) og **Host Data**-roten (`/mnt`) for deg. Sikkerhetskopi-*kilder* og -*mål* ligger begge under Host Data. For den fullstendige variabelreferansen og oppsettet for ekstern lagring, se [Konfigurasjon](configuration.md).

## Første kjøring {#first-run}

![Velkomstsiden på en ny installasjon, med det BombVault fant på serveren.](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![Velkomstsiden på en ny installasjon, med det BombVault fant på serveren.](assets/screenshots/getting-started/welcome-light.png#only-light)

1. Åpne webgrensesnittet på `https://<your-unraid-ip>:3443` (selvsignert sertifikat rett ut av boksen). En ny installasjon åpner på **Velkommen til BombVault**, som viser hva BombVault fant på serveren.
2. Velg **Kom i gang**. Oversikten viser da kortet **Første steg** med tre steg:
    1. **Velg områder**: slå på det BombVault skal sikkerhetskopiere (Kontainere, VM-er, Flash, Mapper, ZFS-datasett, Auto-sikkerhetskopi). **Velg** åpner listen over elementene i et område, der **Sett tidsplanen på pause** lar ett stå utenfor. Områdene du slår på her, kjører hver natt, og det endrer du under **Innstillinger, Tidsplan**.
    2. **Legg til mål**: et andre sted for sikkerhetskopiene. Hvert område har allerede en mappe på denne serveren, `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`, som opprettes ved den første sikkerhetskopien. Tryggest er et andre mål utenfor huset, for eksempel en Storage Box eller Backblaze B2.
    3. **Første sikkerhetskopi**: etter det kjører den av seg selv hver natt. Kortet blir stående til den første sikkerhetskopien er ferdig.
3. Åpne **Sikkerhetskopier** for å se hvert element i én liste. **Sikkerhetskopier alt nå** starter en kjøring over alle områdene som er slått på, og **Sikkerhetskopier nå** på siden til et element sikkerhetskopierer bare det ene.

![Kortet Første steg med de tre stegene, det andre åpent.](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![Kortet Første steg med de tre stegene, det andre åpent.](assets/screenshots/getting-started/first-steps-light.png#only-light)

Har du brukt BombVault før? **Gjenopprett fra en sikkerhetskopi** på velkomstsiden åpner siden Gjenoppretting, der **Nødveiviser** henter tilbake innstillingene, containerne, VM-ene og mappene til en tapt server. Se [Ekstern lagring og gjenoppretting](offsite-recovery.md#emergency).

!!! tip "Valgfritt: velg en sikkerhetskopieringsrekkefølge"
    Hvis noen containere alltid skal sikkerhetskopieres før andre (for eksempel en database før appen som bruker den), åpne kortet **Rekkefølge** under **Innstillinger, Tidsplan** og dra dem inn i rekkefølgen du ønsker. Planlagte kjøringer og full sikkerhetskopi følger den deretter. Alt du lar stå urangert, sikkerhetskopieres mest forsinket først.

## Eksempel: din første containersikkerhetskopi, fra start til slutt {#example-first-backup}

Dette går gjennom en fersk Unraid-boks med noen få containere, blant dem `plex`, til den første sikkerhetskopien er ferdig og du vet hvor du finner den. Det tar omtrent ti minutter pluss tiden den første sikkerhetskopien bruker.

1. **Installer.** Søk etter **BombVault** i fanen **Apps** i Unraid og trykk på **Install**.
2. **Sett nøkkelen.** Kjør `openssl rand -hex 32` på en datamaskin og lim de 64 tegnene inn i feltet `APP_KEY` i malen. Ta en kopi i passordbehandleren din med en gang, før noe annet. Trykk på **Apply**.
3. **Åpne den.** Gå til `https://<your-unraid-ip>:3443` og godta det selvsignerte sertifikatet. **Velkommen til BombVault** lister det den fant, for eksempel 12 containere, 2 VM-er og flash-disken.
4. **Kom i gang.** Oversikten åpnes med kortet **Første steg**.
5. **Velg områder.** **Kontainere** er på fra starten. Slå på det du ellers vil ha, for eksempel **Flash**. **Velg** åpner listen over containerne den fant, der **Sett tidsplanen på pause** lar en stå utenfor, for eksempel en testcontainer du ikke bryr deg om. Områder som er slått på, kjører hver natt i den fulle sikkerhetskopien.
6. **Legg til mål.** Hvert område har allerede sin egen mappe på denne serveren, under `/mnt/user/bombvault`, så den første sikkerhetskopien har et sted å gå. Dette trinnet legger til et andre sted, og det tryggeste ligger utenfor huset. Du kan gjøre det nå med [S3-eksempelet](offsite-recovery.md#example-s3), eller komme tilbake til det når den første sikkerhetskopien er ferdig. Kortet blir stående til da.
7. **Første sikkerhetskopi.** Åpne **Sikkerhetskopier** og trykk på **Sikkerhetskopier alt nå**, eller vent til natten. Ringen i statushodet blir til en fremdriftsring og teller elementene, for eksempel **Sikkerhetskopierer nå: 3 av 12**. Hver container stoppes mens dataene leses og startes igjen rett etterpå. En container med data på ZFS er nede bare de få sekundene et øyeblikksbilde tar. Den første sikkerhetskopien leser alt, så den tar lengst tid. Senere lagrer bare det som er endret.
8. **Sjekk den.** Når ringen sier **Alt er sikkerhetskopiert**, åpner du **Historikk** på oversikten: hver container har en grønn linje under **Kjøringer**. Åpne **Sikkerhetskopier**, klikk på `plex` og se på fanen **Sikkerhetskopier**: det første øyeblikksbildet er der, med **Lokal** som stedet som har det. Noen minutter senere viser fanen **Verifisering** gjenopprettingskontrollen, som gjenopprettet et utvalg av sikkerhetskopien og leste det tilbake.
9. **Beskytt nøkkelen.** Åpne **Innstillinger, Sikkerhet** og sett et innloggingspassord. Åpne deretter **Innstillinger, Gjenoppretting** og trykk på **Last ned gjenopprettingssett** i **For nødstilfeller**. Lagre filen utenfor serveren, ved siden av `APP_KEY`-kopien fra trinn 2. Påminnelsen **Lagre gjenopprettingssettet ditt** på oversikten forsvinner når du trykker på **Jeg har lagret det trygt**.

Fra nå av sikkerhetskopieres containerne hver natt. Gode neste steg, i denne rekkefølgen: et [eksternt mål](offsite-recovery.md#example-s3), så en død server ikke tar sikkerhetskopiene med seg, deretter [varsler](notifications.md), så en feil når fram til deg, og så et blikk på [Tidsplaner](schedules.md) hvis kl. 03:00 ikke passer deg.

![Statushodet mens den første sikkerhetskopien kjører, med fremdriftsringen og stasjonene.](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![Statushodet mens den første sikkerhetskopien kjører, med fremdriftsringen og stasjonene.](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "Host-integrasjonssjekk"
    Kortet **Host-integrasjon** på oversikten sonderer hver montering og hvert CLI (Docker-socket, libvirt, restic, qemu-img, rclone) og rapporterer manglende deler, slik at du kan bekrefte at containeren er riktig koblet opp før du stoler på den. Det venter under **Tilpass** til du legger det til.

## Finn frem {#navigation}

![Innstillinger som fliser, med kortene til den valgte flisen ved siden av. Hver endring skrives mens du gjør den.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Innstillinger som fliser, med kortene til den valgte flisen ved siden av. Hver endring skrives mens du gjør den.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Sidefeltet har fem oppføringer: **Oversikt**, **Avvik** (så lenge avviksdeteksjon er på), **Sikkerhetskopier**, **Gjenoppretting** og, under **Logg ut**, **Innstillinger**. Hovedhandlingen på en side flyter nederst til høyre, og **Mer** rommer de sjeldne. På en telefon har linjen Oversikt, Sikkerhetskopier, Gjenoppretting og Innstillinger, og **Mer** har resten.

Sider og innstillinger bruker hele bredden av vinduet. Et vindu har knappene sine i bunnen, med den som går videre til høyre. På en telefon beholder en bunn med mer enn tre knapper veien ut og hovedhandlingen og legger resten under **Mer**.

![På en telefon: linjen nederst med Mer åpnet.](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![På en telefon: linjen nederst med Mer åpnet.](assets/screenshots/getting-started/phone-more-light.png#only-light)

- **Oversikt** viser om alt er sikkerhetskopiert og hva som krever oppmerksomhet. Se [Oversikt](overview.md).
- **Sikkerhetskopier** lister hver container, VM, mappesett, flashen, hvert ZFS-datasett og auto-sikkerhetskopien som elementer. Fliser over listen velger et slag, og **Filtre**, **Sorter** og **Søk** snevrer den inn. Hvert element har en egen side med tre faner: **Sikkerhetskopier**, **Innstillinger** og **Verifisering**. En tilbakeknapp til venstre for ikonet til elementet fører til listen. Se [Sikkerhetskopier og elementer](backups.md).
- **Gjenoppretting** er én side for alt: velg et område, søk etter et element og velg en sikkerhetskopi. Nødveiviseren og de andre kildene starter også her. Se [Gjenoppretting](restore.md).
- **Innstillinger** er fjorten fliser: Generelt, Utseende, Tidsplan, Mål, Gjenoppretting, Varsler, Verifisering, Sikkerhet, Instanser, Paring, Integrasjoner, Apper, System og Info. Se [Innstillinger](settings.md). Trykk `/` eller Ctrl+K for å søke i alle, også etter navnene sidene hadde i tidligere versjoner. Dra flisene i din egen rekkefølge under **Utseende**. Der en kolonne med fliser ved siden av siden ville presse den sammen, som i et vindu på rundt 1200 piksler bredde, blir flisene en stripe over siden som kan rulles sidelengs. En telefon viser den samme stripen.
- **Mål** og **Instanser** er innstillingsfliser med egne sider. **Vis også i sidefeltet** på flisen deres fester dem til sidefeltet.

Ingenting har en Lagre-knapp og ingenting folder seg ut: sjeldne ting har et eget kort eller et vindu, og forklaringer står i (i)-boblene. Etter en oppdatering åpner et vindu seg av seg selv med et kort for hver utgivelse siden din forrige versjon, og grønne prikker markerer det som er endret. Et klikk skjuler en prikk. **Nytt** i kortet Versjon under **Innstillinger, Info** teller det du ikke har sett ennå og åpner det vinduet igjen, med utgivelsesnotatene for hver utgivelse og endringene i den med **Gå dit**.

## Bygge fra kildekode {#build-from-source}

BombVault er én statisk Go-binærfil som leverer et JSON-API og et innebygd React-grensesnitt. Bygg grensesnittet først, og kjør deretter binærfilen:

```bash
npm --prefix web ci
npm --prefix web run build     # skriver web/dist, som binærfilen bygger inn
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # enhets- og integrasjonstester, med en ekte restic-runde fram og tilbake
golangci-lint run ./...
go run ./cmd/bombvault         # leverer https://localhost:3443 med et selvsignert sertifikat
```

Grensesnittbygget trengs også for `go run`. Arkivet inneholder bare en tom markør under `web/dist`, så uten `npm --prefix web run build` bygger binærfilen ikke inn noe og svarer `500 SPA index not found`, og det er forventet. Docker, libvirt og Unraid kan ikke testes i CI, så sjekk monteringer, restic og SSH-forbindelsen for VM-er på en ekte vert med kortet **Host-integrasjon** på oversikten (eller `GET /api/spike`) før du åpner en pull request.

## Neste steg {#next-steps}

- Legg til ett eller flere mål utenfor huset (hvert område kan kopiere til flere samtidig) og lagre gjenopprettingssettet ditt, som beskrevet under **[Ekstern lagring og gjenoppretting](offsite-recovery.md)**. [S3-eksempelet](offsite-recovery.md#example-s3) går gjennom ett.
- Sørg for at en feil når fram til deg med **[Varsler](notifications.md)**.
- Sjekk når ting kjører under **[Tidsplaner](schedules.md)**.
- Ha alle servere i gruppen din på telefonen med **[Android-appen](android.md)**.
- **[Gjennomgåtte eksempler](guides.md)** går gjennom de vanlige oppgavene, og **[Funksjoner](features.md)** lister alt BombVault gjør.
- Kloner du et oppsett eller flytter til en ny boks? Ta med hele konfigurasjonen din via kortet **Eksporter / importer innstillinger**. Se [Konfigurasjon](configuration.md#portable-settings-export-and-import).
- Støtt på et problem? Se **[Feilsøking](troubleshooting.md)**.
