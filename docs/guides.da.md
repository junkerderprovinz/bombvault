# Gennemgåede eksempler

Hvert af disse følger ét rigtigt job fra start til slut med navne og tal udfyldt. De hører til på den side, der forklarer funktionen, så detaljerne står lige ved siden af.

## Kom godt i gang {#start}

- [Din første containersikkerhedskopi, fra start til slut](getting-started.md#example-first-backup): fra installationen til gendannelseskittet.

## Lager og off-site {#storage}

- [En off-site-kopi på Backblaze B2](offsite-recovery.md#example-s3): en bucket, en nøgle til den bucket og den første kopi. Cloudflare R2 virker på samme måde.
- [En rest-server, du kører selv](offsite-recovery.md#example-rest-server): en off-site-kopi, der afviser sletninger, med `docker run`-linjen og manipulationstesten.
- [Hvad Afbalanceret beholder](offsite-recovery.md#example-retention): opbevaring over to år med natlige sikkerhedskopier, sikkerhedskopi for sikkerhedskopi.
- [Containere flytter til en NAS](offsite-recovery.md#example-own-folder): et områdes egen mappe skifter sted, og hvad der sker med de sikkerhedskopier, der allerede er lavet.

## Sikkerhedskopier og tidsplaner {#backups}

- [Tre planer, der passer sammen](schedules.md#example): en fuld sikkerhedskopi om natten, VM'er én gang om ugen og en egen plan til én post hver 6. time.
- [Plex venter, mens nogen ser noget](backups.md#example-plex): en planlagt sikkerhedskopiering, der ikke afbryder en film.
- [En VM med to Unraid-snapshots](vm-backup-ssh-setup.md#example-unraid-snapshots): en live-sikkerhedskopi, der beholder snapshottene, og en gendannelse ved siden af originalen.
- [En replika til en anden Unraid-boks](zfs-datasets.md#example-replica): en ZFS-kopi, du kan læse når som helst, og tre måder at få én fil ud af den.

## Få tingene tilbage {#restore}

- [Hent én fil tilbage](restore.md#example-single-file): en enkelt fil fra i nats containersikkerhedskopi.
- [Én fil fra en ZFS-sikkerhedskopi](zfs-datasets.md#example-zfs-file): det samme for et datasæt.
- [En ny server fra gendannelseskittet](offsite-recovery.md#example-emergency): en død server vender tilbage på en ny boks uden sin gamle `APP_KEY` i skabelonen.

## Hold dig informeret {#staying-informed}

- [Gotify på din telefon](notifications.md#example-gotify): fejl ringer, og succeser kommer stille.
- [ntfy uden konto](notifications.md#example-ntfy): beskeder til dit eget emne.

## Flere servere {#several-servers}

- [Parring af to Unraid-bokse](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): to servere, der holder hinandens kopier, og ingen af dem kan slette den andens sikkerhedskopier.
- [Et kort og en alarm i Home Assistant](api.md#home-assistant-example): BombVault på et dashboard med en besked, når en sikkerhedskopiering mislykkes.
