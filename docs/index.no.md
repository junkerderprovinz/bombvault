# BombVault

**Unraid-dataene dine, forseglet i et hvelv. Slipp en sikkerhetskopi. Utløs en gjenoppretting.**

![Oversikten: statushodet, det som trenger deg, beskyttelsen av hvert område, målene og historikken.](assets/screenshots/overview/home-dark.png#only-dark)
![Oversikten: statushodet, det som trenger deg, beskyttelsen av hvert område, målene og historikken.](assets/screenshots/overview/home-light.png#only-light)

BombVault er en selvhostet webapp for **sikkerhetskopiering og full katastrofegjenoppretting** på Unraid og andre Docker-verter, for eksempel TrueNAS SCALE, ZimaOS eller en vanlig Linux-boks. Den sikkerhetskopierer Docker-containere, KVM/libvirt-VM-er, mapper, ZFS-datasett og Unraid-flashen. Den kjører som én multi-arch Docker-container, gir deg et webgrensesnitt i et mørkt, lyst eller systemtema, og håndterer hele livssyklusen: sikkerhetskopier, planlegg, verifiser og gjenopprett.

Gjenopprettinger er automatiske. På Unraid dukker containere opp igjen i Docker-fanen akkurat som før, og VM-er defineres på nytt i VM Manager med diskene og UEFI-NVRAM koblet til igjen. Ingen manuell reinstallasjon, ingen omkonfigurering, ikke noe dramatikk. På TrueNAS kjører en container som er gjenopprettet ved siden av originalen, som en vanlig Docker-container som Apps-skjermen ikke lister.

Drevet av [restic](https://restic.net), så hver sikkerhetskopi er deduplisert, inkrementell og kryptert som standard.

!!! note "Ta vare på APP_KEY-en din"
    BombVault utleder passordet til restic-repositoriet fra en 32-byte hemmelighet kalt `APP_KEY`. Mister du den sammen med gjenopprettingssettet, blir krypterte sikkerhetskopier umulige å gjenopprette. Generer en med `openssl rand -hex 32` og oppbevar den et trygt sted. Se [Konfigurasjon](configuration.md).

## Hva BombVault beskytter {#what-bombvault-protects}

Hver container, VM, mappesett, flashen, hvert ZFS-datasett og BombVaults egne innstillinger er et **element** med en egen side. Elementer av samme slag danner et **område**, som du slår på under **Innstillinger, Generelt**.

| Område | Hva som lagres |
|---|---|
| **Kontainere** | Appdata-katalogen pluss container-definisjonen (image, miljøvariabler, porter, etiketter, volumer). |
| **VM-er** | VM-diskimage(r), XML-definisjonen og UEFI-NVRAM, sikkerhetskopiert over SSH (ingen libvirt-montering). |
| **Flash** | Hele USB-flashen (`/boot`): OS, lisens, array-config, delinger, nettverks- og plugin-config. |
| **Auto-sikkerhetskopi** | BombVaults egen `/config`: innstillingsdatabasen, påloggingene til målene og libvirt-SSH-nøkkelparet. |
| **Mapper** | En hvilken som helst mappe på serveren som mappesett, hver med valgfrie ekskluderingsmønstre. |
| **ZFS-datasett** | Et datasett med alle datasett under det, lest fra ett ZFS-øyeblikksbilde og lagret som en mappe. Se [ZFS-datasett](zfs-datasets.md). |

## Gjenoppretting er stjernen {#restore-is-the-star}

Etter å ha kopiert dataene tilbake fra restic-sikkerhetskopien, spiller BombVault den lagrede container-definisjonen på nytt mot Docker-API-et, slik at containeren dukker opp igjen i Unraids Docker-fane som om den alltid hadde vært der (samme image, samme innstillinger, samme portmappinger). VM-er får XML-en sin definert på nytt over SSH og diskene og UEFI-NVRAM koblet til igjen, selv etter at VM-en ble slettet.

Ett vindu gjenoppretter alle slag. Som standard velger det **Ved siden av originalen**, som henter sikkerhetskopien tilbake som en kopi og lar originalen stå som den er. **Erstatt originalen** tar først en sikkerhetskopi av dagens tilstand, så en gjenoppretting kan angres. En mappe eller et nytt ZFS-datasett er de andre målene.

Når en sikkerhetskopi stopper avhengige containere, kommer de tilbake i riktig rekkefølge: BombVault starter dem på nytt i deres Compose `depends_on`-rekkefølge og venter på at hver enkelt skal rapportere som sunn før den starter dem som er avhengige av den, slik at ingenting løper i forveien av en database eller en gateway som ikke er oppe ennå. Se [Gjenoppretting](restore.md) og [Kommandoer og andre containere](backups.md#commands).

## Slik fungerer det {#how-it-works}

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

BombVault bruker Docker-socketen til å stoppe containere før en sikkerhetskopi og gjenskape dem etter en gjenoppretting. For VM-er kjører den `virsh` på verten over SSH (`qemu+ssh://`) for å slå av en VM på en ryddig måte eller ta et live-øyeblikksbilde. Den bind-monterer aldri en libvirt-sti, så den kan ikke komme i veien for VM Manager på verten.

BombVault er orkestrerings- og UI-laget, ikke lagringsmotoren. All faktisk dataflytting går gjennom restic.

## Kom raskt i gang {#quick-start}

Ny her? Gå til **[Kom i gang](getting-started.md)** for å installere BombVault og kjøre din første sikkerhetskopi, og sett så opp **[Ekstern lagring og gjenoppretting](offsite-recovery.md)**. De **[gjennomgåtte eksemplene](guides.md)** tar de vanlige oppgavene fra start til slutt, siden **[Innstillinger](settings.md)** lister hver flis, og **[Funksjoner](features.md)** er den korte katalogen over alt BombVault gjør.

Hvert område kan kopiere til flere mål samtidig, en paret BombVault kan oppbevare de eksterne kopiene dine på en server som nekter sletting, og **Nødveiviser** henter en tapt server tilbake fra gjenopprettingssettet, via gruppen din eller med den inntastede nøkkelen. For å ta med konfigurasjonen til en ny boks bruker du kortet **Eksporter / importer innstillinger**. Se [Ekstern lagring og gjenoppretting](offsite-recovery.md) og [Konfigurasjon](configuration.md#portable-settings-export-and-import).

**[Android-appen](android.md)** gir deg alle servere i gruppen din på telefonen, med aktivitetsloggen for alle på én skjerm.

Dokumentasjonen finnes på 26 språk og grensesnittet på 42. Sider som ikke er oversatt ennå, vises på engelsk.

## Ord som brukes her {#glossary}

Noen ord har en fast betydning i BombVault og på disse sidene.

| Ord | Betydning |
|---|---|
| **Element** | Én ting BombVault sikkerhetskopierer: en container, en VM, et mappesett, flashen, et ZFS-datasett med de under det, eller BombVaults egne innstillinger. Hvert har en egen side. |
| **Område** | Alle elementer av samme slag, for eksempel Containere. Du slår på områder under **Innstillinger, Generelt**. |
| **Mål** | Et sted sikkerhetskopier går: en mappe på denne serveren, en deling, en egen server eller en skytjeneste. Se [Mål](offsite-recovery.md#destinations). |
| **Lokal** | Målet på denne serveren som holder mappene til områdene. |
| **Egen mappe** | Repositoriet et område skriver sikkerhetskopiene sine til først. Som standard ligger det på denne serveren, men det kan ligge på et mål. Se [Et områdes egen mappe](offsite-recovery.md#own-folder). |
| **Kopi** | En sikkerhetskopi som er kopiert fra den egne mappen til et annet mål med `restic copy`. |
| **Direkte repository** | Et repository på et eksternt sted som elementer sikkerhetskopierer rett til, uten en lokal kopi imellom. Se [Direkte repoer](offsite-recovery.md#direct-repositories). |
| **Pålogging** | Den lagrede nøkkelen eller passordet til en S3-tjeneste eller en rest-server. Én av dem er standard. Se [Pålogginger](offsite-recovery.md#sign-ins). |
| **Plassering** | Hvilke mål som får sikkerhetskopiene til et element, satt på elementet eller som standard for området. Se [Plassering per element](offsite-recovery.md#placement). |
| **Replika** | En blokk-eksakt kopi av et ZFS-element på en andre ZFS-server, laget med `zfs send`. Den står ved siden av sikkerhetskopien og erstatter den ikke. Se [Replika](zfs-datasets.md#replica). |
| **Rolle** | Noe én instans i gruppen din gjør for en annen: Mottaker, Henter, ZFS-server eller Nøkkelvokter. Se [Instanser og roller](offsite-recovery.md#roles). |

## Takk til {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** av [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) ga BombVault utgangspunktet: sikkerhetskopiering med ett klikk og automatisk reinstallering av Docker-containere. BombVault er en egen implementasjon i Go og restic som tar idéen videre til VM-er, flashen og mer.
- **[restic](https://restic.net/)** er den raske, sikre, deduplikerende sikkerhetskopimotoren som BombVault styrer.
- **[rclone](https://rclone.org/)** står for skybackendene.
- De fleste symbolene på knapper kommer fra det gratis Core Solid-settet fra **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [kilde](https://github.com/webalys-hq/streamline-vectors)). Resten kommer fra Font Awesome Free, Material Design Icons, Simple Icons og Tabler Icons, eller er tegnet for prosjektet.

## Lisens {#license}

Copyright (C) 2026 Junker der Provinz. BombVault er fri programvare under **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Du kan kjøre, studere, dele og endre den. Hvis du distribuerer den, eller kjører en endret versjon som en nettverkstjeneste, må du publisere kildekoden din under samme lisens og beholde de eksisterende opphavsretts- og kildehenvisningene.

Navnet og merkevaren er ikke lisensiert. AGPL dekker bare kildekoden: "BombVault", logoen og merkevaren forblir forbeholdt, så en fork må bruke sitt eget navn og sin egen merkevare og kan ikke utgi seg for å være BombVault.

## Lenker {#links}

- **Kildekode:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid-supporttråd:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Feilrapporter:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-ekvivalent kontroll over hosten"
    Gjennom Docker-socketen kan BombVault stoppe, fjerne og gjenskape containere og lese/skrive appdata, og for VM-sikkerhetskopiering logger den seg inn på hosten over SSH for å kjøre `virsh`. Alle som når webgrensesnittet, har i praksis root på hosten. Kjør BombVault kun på et betrodd, ikke-eksponert nettverk, og sett et påloggingspassord under **Innstillinger, Sikkerhet**. Gjenopprettingssettet, nøkkeldeponeringer og sletting av sikkerhetskopiene på et mål krever det. Se [Konfigurasjon](configuration.md#security-model) for hele sikkerhetsmodellen.
