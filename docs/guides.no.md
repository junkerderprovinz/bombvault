# Gjennomgåtte eksempler

Hvert av disse tar én ekte oppgave fra start til slutt, med navn og tall fylt inn. De står på siden som forklarer funksjonen, så detaljene er rett ved siden av.

## Kom i gang {#start}

- [Din første containersikkerhetskopi, fra start til slutt](getting-started.md#example-first-backup): fra installasjonen til gjenopprettingssettet.

## Lagring og off-site {#storage}

- [En off-site-kopi på Backblaze B2](offsite-recovery.md#example-s3): en bøtte, en nøkkel for den bøtta og den første kopien. Cloudflare R2 fungerer på samme måte.
- [En rest-server du kjører selv](offsite-recovery.md#example-rest-server): en off-site-kopi som nekter sletting, med `docker run`-linjen og tamper-testen.
- [Hva Balansert beholder](offsite-recovery.md#example-retention): oppbevaring over to år med nattlige sikkerhetskopier, sikkerhetskopi for sikkerhetskopi.
- [Containere flytter til en NAS](offsite-recovery.md#example-own-folder): den egne mappen til et område bytter sted, og hva som skjer med sikkerhetskopiene som allerede er laget.

## Sikkerhetskopier og tidsplaner {#backups}

- [Tre planer som passer sammen](schedules.md#example): en full sikkerhetskopi om natten, VM-er én gang i uka og en egen plan for ett element hver 6. time.
- [Plex venter mens noen ser på](backups.md#example-plex): en planlagt sikkerhetskopi som ikke avbryter en film.
- [En VM med to Unraid-øyeblikksbilder](vm-backup-ssh-setup.md#example-unraid-snapshots): en live-sikkerhetskopi som beholder øyeblikksbildene, og en gjenoppretting ved siden av originalen.
- [En replika til en andre Unraid-boks](zfs-datasets.md#example-replica): en ZFS-kopi du kan lese når som helst, og tre måter å få én fil ut av den.

## Få ting tilbake {#restore}

- [Få én fil tilbake](restore.md#example-single-file): én enkelt fil fra gårsdagens containersikkerhetskopi.
- [Én fil fra en ZFS-sikkerhetskopi](zfs-datasets.md#example-zfs-file): det samme for et datasett.
- [En ny server fra gjenopprettingssettet](offsite-recovery.md#example-emergency): en død server kommer tilbake på en ny boks, uten den gamle `APP_KEY` i malen.

## Hold deg informert {#staying-informed}

- [Gotify på telefonen](notifications.md#example-gotify): feil ringer, og vellykkede kjøringer kommer stille.
- [ntfy uten konto](notifications.md#example-ntfy): meldinger til ditt eget emne.

## Flere servere {#several-servers}

- [Pare to Unraid-bokser](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): to servere som oppbevarer hverandres kopier, og ingen av dem kan slette den andres sikkerhetskopier.
- [Et kort og en varsling i Home Assistant](api.md#home-assistant-example): BombVault på et dashbord, med en melding når en sikkerhetskopi feiler.
