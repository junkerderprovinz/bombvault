# Genomgångna exempel

Vart och ett av de här går igenom en verklig uppgift från början till slut, med namn och siffror ifyllda. De står på sidan som förklarar funktionen, så detaljerna finns precis bredvid.

## Kom igång {#start}

- [Din första containersäkerhetskopia, från början till slut](getting-started.md#example-first-backup): från installationen till återställningskitet.

## Lagring och off-site {#storage}

- [En off-site-kopia på Backblaze B2](offsite-recovery.md#example-s3): en hink, en nyckel för just den hinken och den första kopian. Cloudflare R2 fungerar likadant.
- [En rest-server som du kör själv](offsite-recovery.md#example-rest-server): en off-site-kopia som vägrar radering, med raden `docker run` och manipuleringstestet.
- [Vad Balanserad behåller](offsite-recovery.md#example-retention): gallring över två års nattliga säkerhetskopior, säkerhetskopia för säkerhetskopia.
- [Containrar flyttar till en NAS](offsite-recovery.md#example-own-folder): ett avsnitts egna mapp byter plats, och vad som händer med de säkerhetskopior som redan finns.

## Säkerhetskopior och scheman {#backups}

- [Tre planer som passar ihop](schedules.md#example): en full säkerhetskopiering på natten, VM:ar en gång i veckan och en egen plan för en post var 6:e timme.
- [Plex väntar medan någon tittar](backups.md#example-plex): en schemalagd säkerhetskopiering som inte avbryter en film.
- [En VM med två Unraid-ögonblicksbilder](vm-backup-ssh-setup.md#example-unraid-snapshots): en live-säkerhetskopiering som behåller ögonblicksbilderna, och en återställning bredvid originalet.
- [En replik till en andra Unraid-låda](zfs-datasets.md#example-replica): en ZFS-kopia som du kan läsa när som helst, och tre sätt att få ut en enskild fil ur den.

## Få tillbaka saker {#restore}

- [Få tillbaka en enda fil](restore.md#example-single-file): en enskild fil ur gårdagens containersäkerhetskopia.
- [En fil ur en ZFS-säkerhetskopia](zfs-datasets.md#example-zfs-file): detsamma för en datauppsättning.
- [En ny server från återställningskitet](offsite-recovery.md#example-emergency): en död server kommer tillbaka på en ny låda, utan sin gamla `APP_KEY` i mallen.

## Håll dig informerad {#staying-informed}

- [Gotify i telefonen](notifications.md#example-gotify): fel ringer och lyckade körningar kommer tyst.
- [ntfy utan konto](notifications.md#example-ntfy): meddelanden till ett eget ämne.

## Flera servrar {#several-servers}

- [Parkoppla två Unraid-lådor](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): två servrar som håller varandras kopior, och ingen kan radera den andras säkerhetskopior.
- [Ett kort och en avisering i Home Assistant](api.md#home-assistant-example): BombVault på en instrumentpanel, med ett meddelande när en säkerhetskopiering misslyckas.
