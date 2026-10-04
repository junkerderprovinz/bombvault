# BombVault

**Unraid-dataene dine, forseglet i et hvelv. Slipp en sikkerhetskopi. Utløs en gjenoppretting.**

BombVault er en selvhostet, Unraid-nativ webapp for **sikkerhetskopiering og full katastrofegjenoppretting** av Docker-containerne og KVM/libvirt-VM-ene dine. Den kjører som én multi-arch Docker-container, gir deg et moderne webgrensesnitt som følger systemets lys/mørk-preferanse, og håndterer hele livssyklusen: sikkerhetskopier, planlegg, verifiser og gjenopprett.

Gjenopprettinger er automatiske. Containere dukker opp igjen i Unraids Docker-fane akkurat som før, og VM-er defineres på nytt i VM Manager med diskene og UEFI-NVRAM koblet til igjen. Ingen manuell reinstallasjon, ingen omkonfigurering, ikke noe dramatikk.

Drevet av [restic](https://restic.net), så hver sikkerhetskopi er deduplisert, inkrementell og alltid kryptert.

!!! note "Ta vare på APP_KEY-en din"
    BombVault utleder passordet til restic-repositoriet fra en 32-byte hemmelighet kalt `APP_KEY`. Mister du den, blir krypterte sikkerhetskopier umulige å gjenopprette. Generer en med `openssl rand -hex 32` og oppbevar den et trygt sted. Se [Konfigurasjon](configuration.md).

## Hva BombVault beskytter

| Domene | Hva som lagres |
|---|---|
| **Docker-containere** | Appdata-katalogen pluss container-definisjonen (image, miljøvariabler, porter, etiketter, volumer). |
| **KVM / libvirt-VM-er** | VM-diskimage(r), XML-definisjonen og UEFI-NVRAM, sikkerhetskopiert over SSH (ingen libvirt-montering). |
| **Unraid-flash** | Hele USB-flashen (`/boot`): OS, lisens, array-config, delinger, nettverks- og plugin-config. |
| **App-konfigurasjon** | BombVaults egen `/config`: innstillingsdatabasen, ekstern legitimasjon og libvirt-SSH-nøkkelparet. |
| **Filer og mapper** | Navngitte **filsett**, en hvilken som helst mappe på serveren, hver med valgfrie ekskluderingsmønstre per sett. |
| **ZFS-datasett** | Et datasett med alle datasett under det, lest fra ett ZFS-øyeblikksbilde og lagret som en mappe. Se [ZFS-datasett](zfs-datasets.md). |

## Gjenoppretting er stjernen

Etter å ha kopiert dataene tilbake fra restic-øyeblikksbildet, spiller BombVault den lagrede container-definisjonen på nytt mot Docker-API-et, slik at containeren dukker opp igjen i Unraids Docker-fane som om den alltid hadde vært der (samme image, samme innstillinger, samme portmappinger). VM-er får XML-en sin definert på nytt over SSH og diskene og UEFI-NVRAM koblet til igjen, selv etter at VM-en ble slettet.

Når en sikkerhetskopi stopper avhengige containere, kommer de tilbake i riktig rekkefølge: BombVault starter dem på nytt i deres Compose `depends_on`-rekkefølge og venter på at hver enkelt skal rapportere som sunn før den starter dem som er avhengige av den, slik at ingenting løper i forveien av en database eller en gateway som ikke er oppe ennå. Se [Funksjoner](features.md).

## Slik fungerer det

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

BombVault bruker Docker-socketen til å stoppe containere før en sikkerhetskopi og gjenskape dem etter en gjenoppretting. For VM-er kjører den `virsh` på verten over SSH (`qemu+ssh://`) for å slå av en VM på en ryddig måte eller ta et live-øyeblikksbilde. Den bind-monterer aldri en libvirt-sti, så den kan ikke komme i veien for VM Manager på verten.

BombVault er orkestrerings- og UI-laget, ikke lagringsmotoren. All faktisk dataflytting går gjennom restic.

## Kom raskt i gang

Ny her? Gå til **[Kom i gang](getting-started.md)** for å installere BombVault på Unraid via Community Applications og kjøre din første sikkerhetskopi. Utforsk deretter alle **[Funksjoner](features.md)**, finjuster **[Konfigurasjonen](configuration.md)** din, og sett opp **[Ekstern lagring og gjenoppretting](offsite-recovery.md)**.

Ekstern lagring kan fordeles til flere mål per domene samtidig, et skrivebeskyttet **mottaker-dashboard** overvåker disse kopiene på boksen som mottar dem, og du kan ta med hele konfigurasjonen din til en ny boks med kortet **Eksporter / importer innstillinger**. Se [Ekstern lagring og gjenoppretting](offsite-recovery.md) og [Konfigurasjon](configuration.md#portable-settings-export-and-import).

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
    Gjennom Docker-socketen kan BombVault stoppe, fjerne og gjenskape containere og lese/skrive appdata, og for VM-sikkerhetskopiering logger den seg inn på hosten over SSH for å kjøre `virsh`. Alle som når webgrensesnittet, har i praksis root på hosten. Kjør BombVault kun på et betrodd, ikke-eksponert nettverk, og aktiver den valgfrie passordbeskyttelsen (Innstillinger, Sikkerhet) så snart ekstern lagring eller uforanderlige sikkerhetskopier er i bruk. Se [Konfigurasjon](configuration.md) for hele sikkerhetsmodellen.
