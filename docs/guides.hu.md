# Részletes példák

Mindegyik példa egy valódi feladatot visz végig az elejétől a végéig, kitöltött nevekkel és számokkal. Annak az oldalnak a részei, amely a funkciót magyarázza, így a részletek mellette vannak.

## Kezdő lépések {#start}

- [Az első konténermentésed az elejétől a végéig](getting-started.md#example-first-backup): a telepítéstől a helyreállítási csomagig.

## Tárolás és telephelyen kívüli mentés {#storage}

- [Telephelyen kívüli másolat a Backblaze B2-n](offsite-recovery.md#example-s3): egy vödör, egy kulcs ahhoz a vödörhöz, és az első másolat. A Cloudflare R2 ugyanígy működik.
- [Egy rest-server, amelyet magad üzemeltetsz](offsite-recovery.md#example-rest-server): egy telephelyen kívüli másolat, amely megtagadja a törlést, a `docker run` sorral és a kijátszási próbával.
- [Mit tart meg a Kiegyensúlyozott](offsite-recovery.md#example-retention): a megőrzés két évnyi éjszakai mentésen, mentésről mentésre.
- [A konténerek egy NAS-ra költöznek](offsite-recovery.md#example-own-folder): egy szakasz saját mappája helyet vált, és mi lesz a már elkészült mentésekkel.

## Mentések és ütemezések {#backups}

- [Három terv, amely összeillik](schedules.md#example): éjszaka egy teljes mentés, hetente egyszer a VM-ek, és egy saját terv egy elemhez 6 óránként.
- [A Plex vár, amíg valaki néz](backups.md#example-plex): egy ütemezett mentés, amely nem szakít félbe egy filmet.
- [Egy VM két Unraid-pillanatképpel](vm-backup-ssh-setup.md#example-unraid-snapshots): egy élő mentés, amely megtartja a pillanatképeket, és egy visszaállítás az eredeti mellé.
- [Replika egy második Unraid-gépre](zfs-datasets.md#example-replica): egy ZFS-másolat, amelyet bármikor elolvashatsz, és három út, hogy egyetlen fájlt kiszedj belőle.

## A mentések visszahozása {#restore}

- [Egyetlen fájl visszahozása](restore.md#example-single-file): egyetlen fájl a tegnap éjszakai konténermentésből.
- [Egy fájl egy ZFS-mentésből](zfs-datasets.md#example-zfs-file): ugyanez egy adatkészletnél.
- [Új szerver a helyreállítási csomagból](offsite-recovery.md#example-emergency): egy elpusztult szerver új gépen tér vissza, a régi `APP_KEY` nélkül a sablonban.

## Tájékozottság {#staying-informed}

- [Gotify a telefonodon](notifications.md#example-gotify): a hibák csörögnek, a sikerek csendben érkeznek.
- [Az ntfy fiók nélkül](notifications.md#example-ntfy): üzenetek egy saját témába.

## Több szerver {#several-servers}

- [Két Unraid-gép párosítása](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): két szerver, amely egymás másolatait tartja, és egyik sem törölheti a másik mentéseit.
- [Egy kártya és egy riasztás a Home Assistantben](api.md#home-assistant-example): a BombVault egy irányítópulton, és üzenet, ha egy mentés hibázik.
