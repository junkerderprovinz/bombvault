# BombVault

**Unraid-dataene dine, forseglet i et hvelv. Slipp en sikkerhetskopi. Utløs en gjenoppretting.**

BombVault er en selvhostet, Unraid-nativ webapp for **sikkerhetskopiering og full katastrofegjenoppretting** av Docker-containerne og KVM/libvirt-VM-ene dine. Den kjører som én multi-arch Docker-container, gir deg et webgrensesnitt i et mørkt, lyst eller systemtema, og håndterer hele livssyklusen: sikkerhetskopier, planlegg, verifiser og gjenopprett.

Gjenopprettinger er automatiske. Containere dukker opp igjen i Unraids Docker-fane akkurat som før, og VM-er defineres på nytt i VM Manager med diskene og UEFI-NVRAM koblet til igjen. Ingen manuell reinstallasjon, ingen omkonfigurering, ikke noe dramatikk.

Drevet av [restic](https://restic.net), så hver sikkerhetskopi er deduplisert, inkrementell og alltid kryptert.

!!! note "Ta vare på APP_KEY-en din"
    BombVault utleder passordet til restic-repositoriet fra en 32-byte hemmelighet kalt `APP_KEY`. Mister du den sammen med gjenopprettingssettet, blir krypterte sikkerhetskopier umulige å gjenopprette. Generer en med `openssl rand -hex 32` og oppbevar den et trygt sted. Se [Konfigurasjon](configuration.md).

## Hva BombVault beskytter

Hver container, VM, mappesett, flashen, hvert ZFS-datasett og BombVaults egne innstillinger er et **element** med en egen side. Elementer av samme slag danner et **område**, som du slår på under **Innstillinger, Generelt**.

| Område | Hva som lagres |
|---|---|
| **Kontainere** | Appdata-katalogen pluss container-definisjonen (image, miljøvariabler, porter, etiketter, volumer). |
| **VM-er** | VM-diskimage(r), XML-definisjonen og UEFI-NVRAM, sikkerhetskopiert over SSH (ingen libvirt-montering). |
| **Flash** | Hele USB-flashen (`/boot`): OS, lisens, array-config, delinger, nettverks- og plugin-config. |
| **Auto-sikkerhetskopi** | BombVaults egen `/config`: innstillingsdatabasen, påloggingene til målene og libvirt-SSH-nøkkelparet. |
| **Mapper** | En hvilken som helst mappe på serveren som mappesett, hver med valgfrie ekskluderingsmønstre. |
| **ZFS-datasett** | Et datasett med alle datasett under det, lest fra ett ZFS-øyeblikksbilde og lagret som en mappe. Se [ZFS-datasett](zfs-datasets.md). |

## Gjenoppretting er stjernen

Etter å ha kopiert dataene tilbake fra restic-øyeblikksbildet, spiller BombVault den lagrede container-definisjonen på nytt mot Docker-API-et, slik at containeren dukker opp igjen i Unraids Docker-fane som om den alltid hadde vært der (samme image, samme innstillinger, samme portmappinger). VM-er får XML-en sin definert på nytt over SSH og diskene og UEFI-NVRAM koblet til igjen, selv etter at VM-en ble slettet.

Ett vindu gjenoppretter alle slag. Som standard velger det **Ved siden av originalen**, som henter sikkerhetskopien tilbake som en kopi og lar originalen stå som den er. **Erstatt originalen** tar først en sikkerhetskopi av dagens tilstand, så en gjenoppretting kan angres. En mappe eller et nytt ZFS-datasett er de andre målene.

Når en sikkerhetskopi stopper avhengige containere, kommer de tilbake i riktig rekkefølge: BombVault starter dem på nytt i deres Compose `depends_on`-rekkefølge og venter på at hver enkelt skal rapportere som sunn før den starter dem som er avhengige av den, slik at ingenting løper i forveien av en database eller en gateway som ikke er oppe ennå. Se [Funksjoner](features.md).

## Slik fungerer det

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

BombVault bruker Docker-socketen til å stoppe containere før en sikkerhetskopi og gjenskape dem etter en gjenoppretting. For VM-er kjører den `virsh` på verten over SSH (`qemu+ssh://`) for å slå av en VM på en ryddig måte eller ta et live-øyeblikksbilde. Den bind-monterer aldri en libvirt-sti, så den kan ikke komme i veien for VM Manager på verten.

BombVault er orkestrerings- og UI-laget, ikke lagringsmotoren. All faktisk dataflytting går gjennom restic.

## Kom raskt i gang

Ny her? Gå til **[Kom i gang](getting-started.md)** for å installere BombVault på Unraid via Community Applications og kjøre din første sikkerhetskopi. Utforsk deretter alle **[Funksjoner](features.md)**, finjuster **[Konfigurasjonen](configuration.md)** din, og sett opp **[Ekstern lagring og gjenoppretting](offsite-recovery.md)**.

Hvert område kan kopiere til flere mål samtidig, en paret BombVault kan oppbevare de eksterne kopiene dine på en server som nekter sletting, og **Nødveiviser** henter en tapt server tilbake fra gjenopprettingssettet, via gruppen din eller med den inntastede nøkkelen. For å ta med konfigurasjonen til en ny boks bruker du kortet **Eksporter / importer innstillinger**. Se [Ekstern lagring og gjenoppretting](offsite-recovery.md) og [Konfigurasjon](configuration.md#portable-settings-export-and-import).

**[Android-appen](android.md)** gir deg alle servere i gruppen din på telefonen, med aktivitetsloggen for alle på én skjerm.

## Takk til {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** av [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) ga BombVault utgangspunktet: sikkerhetskopiering med ett klikk og automatisk reinstallering av Docker-containere. BombVault er en egen implementasjon i Go og restic som tar idéen videre til VM-er, flashen og mer.
- **[restic](https://restic.net/)** er den raske, sikre, deduplikerende sikkerhetskopimotoren som BombVault styrer.
- **[rclone](https://rclone.org/)** står for skybackendene.
- De fleste symbolene på knapper kommer fra det gratis Core Solid-settet fra **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [kilde](https://github.com/webalys-hq/streamline-vectors)). Resten kommer fra Font Awesome Free, Material Design Icons, Simple Icons og Tabler Icons, eller er tegnet for prosjektet.

## Lisens {#license}

Copyright (C) 2026 Junker der Provinz. BombVault er fri programvare under **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Du kan kjøre, studere, dele og endre den. Hvis du distribuerer den, eller kjører en endret versjon som en nettverkstjeneste, må du publisere kildekoden din under samme lisens og beholde de eksisterende opphavsretts- og kildehenvisningene.

Navnet og merkevaren er ikke lisensiert. AGPL dekker bare kildekoden: "BombVault", logoen og merkevaren forblir forbeholdt, så en fork må bruke sitt eget navn og sin egen merkevare og kan ikke utgi seg for å være BombVault.

## Lenker

- **Kildekode:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid-supporttråd:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Feilrapporter:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-ekvivalent kontroll over hosten"
    Gjennom Docker-socketen kan BombVault stoppe, fjerne og gjenskape containere og lese/skrive appdata, og for VM-sikkerhetskopiering logger den seg inn på hosten over SSH for å kjøre `virsh`. Alle som når webgrensesnittet, har i praksis root på hosten. Kjør BombVault kun på et betrodd, ikke-eksponert nettverk, og sett et påloggingspassord under **Innstillinger, Sikkerhet**. Gjenopprettingssettet, nøkkeldeponeringer og sletting av sikkerhetskopiene på et mål krever det. Se [Konfigurasjon](configuration.md#security-model) for hele sikkerhetsmodellen.
