# Esimerkit vaihe vaiheelta

Jokainen näistä käy läpi yhden oikean tehtävän alusta loppuun, nimet ja luvut täytettyinä. Ne nojaavat ominaisuutta selittävään sivuun, joten yksityiskohdat löytyvät aivan vierestä.

## Aloitus {#start}

- [Ensimmäinen konttivarmuuskopiosi alusta loppuun](getting-started.md#example-first-backup): asennuksesta palautuspakettiin.

## Tallennus ja etäsijainti {#storage}

- [Etäkopio Backblaze B2:een](offsite-recovery.md#example-s3): bucket, avain juuri sitä bucketia varten ja ensimmäinen kopio. Cloudflare R2 toimii samalla tavalla.
- [Itse ajamasi rest-server](offsite-recovery.md#example-rest-server): etäkopio, joka kieltäytyy poistoista, mukana `docker run` -rivi ja väärennystesti.
- [Mitä Balanced säilyttää](offsite-recovery.md#example-retention): säilytys 400 päivän yövarmuuskopioiden yli, varmuuskopio kerrallaan.
- [Kontit siirtyvät NAS-laitteelle](offsite-recovery.md#example-own-folder): osion oma kansio vaihtaa paikkaa, ja mitä jo tehdyille varmuuskopioille tapahtuu.

## Varmuuskopiot ja ajastukset {#backups}

- [Kolme yhteensopivaa suunnitelmaa](schedules.md#example): täysi varmuuskopio yöllä, virtuaalikoneet kerran viikossa ja oma suunnitelma yhdelle merkinnälle 6 tunnin välein.
- [Plex odottaa, kun joku katsoo](backups.md#example-plex): ajastettu varmuuskopio, joka ei keskeytä elokuvaa.
- [Virtuaalikone ja kaksi Unraid-tilannevedosta](vm-backup-ssh-setup.md#example-unraid-snapshots): live-varmuuskopio, joka säilyttää tilannevedokset, ja palautus alkuperäisen viereen.
- [Replika toiselle Unraid-koneelle](zfs-datasets.md#example-replica): ZFS-kopio, jonka voit lukea milloin tahansa, ja kolme tapaa hakea siitä yksi tiedosto.

## Tietojen palauttaminen {#restore}

- [Yhden tiedoston palautus](restore.md#example-single-file): yksittäinen tiedosto viime yön konttivarmuuskopiosta.
- [Yksi tiedosto ZFS-varmuuskopiosta](zfs-datasets.md#example-zfs-file): sama tietojoukolle.
- [Uusi palvelin palautuspaketista](offsite-recovery.md#example-emergency): kuollut palvelin nousee uudelle koneelle ilman vanhaa `APP_KEY`-avainta mallipohjassa.

## Pysy ajan tasalla {#staying-informed}

- [Gotify puhelimessasi](notifications.md#example-gotify): virheet soivat ja onnistumiset saapuvat hiljaa.
- [ntfy webhookina](notifications.md#example-ntfy): ilmoitukset ilman tiliä.

## Useita palvelimia {#several-servers}

- [Kahden Unraid-koneen pariliitos](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): kaksi palvelinta, jotka säilyttävät toistensa kopioita, eikä kumpikaan voi poistaa toisen varmuuskopioita.
- [Kortti ja hälytys Home Assistantissa](api.md#home-assistant-example): BombVault kojelaudalla ja viesti, kun varmuuskopio epäonnistuu.
