# Praktické příklady

Každý z nich provede jedním skutečným úkolem od začátku do konce, s doplněnými jmény a čísly. Stojí na stránce, která danou funkci vysvětluje, takže podrobnosti jsou hned vedle.

## Začínáme {#start}

- [Tvoje první záloha kontejneru od začátku do konce](getting-started.md#example-first-backup): od instalace po sadu pro obnovu.

## Úložiště a kopie mimo lokalitu {#storage}

- [Kopie mimo lokalitu na Backblaze B2](offsite-recovery.md#example-s3): bucket, klíč jen pro ten bucket a první kopie. Cloudflare R2 funguje stejně.
- [Rest-server, který si provozuješ sám](offsite-recovery.md#example-rest-server): kopie mimo lokalitu, která odmítá mazání, s řádkem `docker run` a tamper testem.
- [Co drží Vyvážené](offsite-recovery.md#example-retention): uchovávání během 400 dní nočních záloh, záloha po záloze.
- [Kontejnery se stěhují na NAS](offsite-recovery.md#example-own-folder): vlastní složka sekce změní místo a co se stane se zálohami, které už existují.

## Zálohy a plány {#backups}

- [Tři plány, které do sebe zapadají](schedules.md#example): úplná záloha v noci, VM jednou týdně a vlastní plán pro jednu položku každých 6 hodin.
- [Plex počká, dokud se někdo dívá](backups.md#example-plex): plánovaná záloha, která nepřeruší film.
- [VM se dvěma snapshoty Unraidu](vm-backup-ssh-setup.md#example-unraid-snapshots): živá záloha, která snapshoty zachová, a obnova vedle originálu.
- [Replika na druhý Unraid](zfs-datasets.md#example-replica): kopie ZFS, kterou si můžeš kdykoli přečíst, a tři cesty, jak z ní dostat jeden soubor.

## Dostat věci zpět {#restore}

- [Dostat zpět jeden soubor](restore.md#example-single-file): jediný soubor z včerejší noční zálohy kontejneru.
- [Jeden soubor ze zálohy ZFS](zfs-datasets.md#example-zfs-file): totéž pro datovou sadu.
- [Nový server ze sady pro obnovu](offsite-recovery.md#example-emergency): mrtvý server se vrátí na nové krabici, bez starého `APP_KEY` v šabloně.

## Být v obraze {#staying-informed}

- [Gotify v telefonu](notifications.md#example-gotify): selhání zvoní a úspěchy dorazí potichu.
- [ntfy jako webhook](notifications.md#example-ntfy): zprávy bez účtu.

## Více serverů {#several-servers}

- [Spárování dvou krabic s Unraidem](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): dva servery, které si drží navzájem kopie, a ani jeden nemůže mazat zálohy toho druhého.
- [Karta a upozornění v Home Assistantu](api.md#home-assistant-example): BombVault na dashboardu a zpráva, když záloha selže.
