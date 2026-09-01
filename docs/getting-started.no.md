# Kom i gang

Denne siden tar deg fra en ny Unraid-boks til din første sikkerhetskopi.

## Krav

| Krav | Merknader |
|---|---|
| **Unraid 6.12+** | Tidligere versjoner er ikke testet. |
| **Plassering for restic-repo** | En lokal sti (anbefalt: array-et eller cachen din), SMB, NFS eller en hvilken som helst rclone-backend. |
| **Docker-socket** | Monteres automatisk av malen (`/var/run/docker.sock`). |
| **Unraid-flash** (`/boot`) | Monteres i sin helhet automatisk av malen (`/boot` til `/host/boot`). Driver flash-sikkerhetskopiering og lar en gjenopprettet container dukke opp igjen som en normal, redigerbar Unraid-app. |
| **KVM-VM-er** (valgfritt) | VM-sikkerhetskopiering snakker med libvirt over SSH, ingen libvirt-montering. Sett det opp i Innstillinger (se [Konfigurasjon](configuration.md)). |

## Installer på Unraid

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

## Generisk Docker-vert

Ikke Unraid? BombVault kjører også som en vanlig container på hvilken som helst Docker-vert (det er også det som bærer containerstøtten på TrueNAS Scale, i påvente av en egen oppføring i app-katalogen der).

1. Hent den redigeringsklare [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) fra arkivet.
2. Sett `APP_KEY` (se nedenfor) og pek Host Data-volumet mot din virkelige datarot: kommentarene i filen går gjennom begge deler.
3. `docker compose up -d`, åpne deretter `https://<vert-ip>:3443/`.

Hva som er annerledes enn på Unraid:

- **Ingen flash-/USB-domene.** Det finnes ingen oppstarts-USB å fange inn eller gjenopprette, så Flash-domenet i innstillingene har ingenting å gjøre her. I stedet tilbyr Fil-domenet ettklikksforslaget **Legg til forhåndsinnstilling: vertssystemets konfigurasjon** (et sett `/etc`-filer å begynne med, som du går gjennom og redigerer før du lagrer) som praktisk generisk motstykke.
- **Ingen Unraid-egne varsler.** BombVaults egne varselkanaler (webhook, varsler om mislykket off-site og lignende) virker som vanlig; bare den Unraid-spesifikke meldingen til dens eget varselsystem utelates, siden et slikt system ikke finnes her.
- **Sikkerhetskopiering av virtuelle maskiner er valgfri og trenger en egen libvirtd-vert som er nåbar over SSH.** Se den utkommenterte blokken i compose-filen. En generisk Docker-vert har ingen innebygd VM-håndtering.

## Den ene påkrevde innstillingen

Den eneste variabelen du må sette, er `APP_KEY`, en 32-byte hex-hemmelighet (64 hex-tegn) som brukes til å utlede passordet til restic-repositoriet.

Generer en på en hvilken som helst maskin:

```bash
openssl rand -hex 32
```

Lim resultatet inn i `APP_KEY`-feltet i malen.

!!! danger "Ikke mist APP_KEY-en din"
    Å miste `APP_KEY` gjør de krypterte sikkerhetskopiene dine umulige å gjenopprette. Oppbevar den et trygt sted og adskilt fra serveren. Når BombVault kjører, bruk dens ett-klikks **gjenopprettingssett for krypteringsnøkkel** (se [Ekstern lagring og gjenoppretting](offsite-recovery.md)) for å lagre hele gjenopprettingspakken.

Malen monterer også Docker-socketen, flashen (`/boot`) og **Host Data**-roten (`/mnt`) for deg. Sikkerhetskopi-*kilder* og -*destinasjoner* ligger begge under Host Data. For den fullstendige variabelreferansen og oppsettet for ekstern lagring, se [Konfigurasjon](configuration.md).

## Første kjøring

1. Åpne webgrensesnittet på `https://<your-unraid-ip>:3443` (selvsignert sertifikat rett ut av boksen).
2. I **Innstillinger**, aktiver sikkerhetskopidomenene du vil ha (Containere, VM-er, Flash, Config, Filer) og velg en aksentfarge.
3. På **Containere**-fanen, velg en container og klikk **Sikkerhetskopier** for å lage ditt første gjenopprettingspunkt. Repository-stier har som standard `/mnt/user/bombvault/{container,vms,flash,config,files}` og opprettes ved den første sikkerhetskopieringen.
4. Sett opp planlegging fra **Innstillinger, Tidsplaner**. Det finnes en ett-klikks *inkluder alle i tidsplan* for containere og VM-er.

!!! tip "Valgfritt: velg en sikkerhetskopieringsrekkefølge"
    Hvis noen containere alltid skal sikkerhetskopieres før andre (for eksempel en database før appen som bruker den), åpne **backup-order**-panelet på Containere-siden og dra dem inn i rekkefølgen du ønsker. Planlagte og flervalgs-kjøringer følger den deretter; alt du lar stå urangert, sikkerhetskopieres mest-forfalt-først, som før.

!!! note "Sjekk av host-integrasjon"
    Åpne `/spike` i webgrensesnittet etter at containeren har startet. Den sonderer hver montering og hvert CLI (Docker-socket, libvirt, restic, qemu-img, rclone) og rapporterer manglende deler, slik at du kan bekrefte at containeren er riktig koblet opp før du stoler på den.

## Enkel vs. Avansert

Som standard viser grensesnittet bare det essensielle (sikkerhetskopier, gjenopprett, planlegg). Bruk **Enkel / Avansert**-bryteren i sidefeltet for å avdekke ekspertkontrollene: oppbevaring, ekstern kopi, pre/post-hooks, gjenoppretting på filnivå, varsler, Prometheus-metrikker og integritets-/vedlikeholdsverktøyene. Det er en innstilling per nettleser og av som standard, så nykommere får et rent grensesnitt og erfarne brukere får alt.

## Neste steg

- Bla gjennom alle **[Funksjoner](features.md)**.
- Legg til én eller flere **[Ekstern lagring og gjenoppretting](offsite-recovery.md)**-replikaer (hvert domene kan sende til flere destinasjoner samtidig) og lagre gjenopprettingssettet ditt.
- Kloner du et oppsett eller flytter til en ny boks? Ta med hele konfigurasjonen din via kortet **Eksporter og importer innstillinger**. Se [Konfigurasjon](configuration.md#portable-settings-export-and-import).
- Støtt på et problem? Se **[Feilsøking](troubleshooting.md)**.
