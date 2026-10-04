# BombVault

**Unraid-datasi, sinetöitynä holviin. Pudota varmuuskopio. Räjäytä palautus.**

BombVault on itse isännöity, Unraid-natiivi verkkosovellus Docker-konttiesi ja KVM/libvirt-virtuaalikoneidesi **varmuuskopiointiin ja täydelliseen katastrofista toipumiseen**. Se pyörii yhtenä multi-arch Docker-konttina, tarjoaa modernin verkkokäyttöliittymän, joka noudattaa järjestelmäsi vaalean/tumman teeman valintaa, ja hoitaa koko elinkaaren: varmuuskopiointi, ajastus, tarkistus ja palautus.

Palautukset ovat automaattisia. Kontit ilmestyvät takaisin Unraidin Docker-välilehdelle täsmälleen ennallaan, ja virtuaalikoneet määritellään uudelleen VM Managerissa levyineen ja UEFI NVRAM -muisteineen. Ei manuaalista uudelleenasennusta, ei uudelleenmääritystä, ei draamaa.

Käyttövoimana [restic](https://restic.net), joten jokainen varmuuskopio on deduplikoitu, inkrementaalinen ja aina salattu.

!!! note "Pidä APP_KEY turvassa"
    BombVault johtaa restic-arkiston salasanan 32-tavuisesta salaisuudesta nimeltä `APP_KEY`. Sen menettäminen tekee salatuista varmuuskopioista palautuskelvottomia. Luo sellainen komennolla `openssl rand -hex 32` ja säilytä se turvallisessa paikassa. Katso [Asetukset](configuration.md).

## Mitä BombVault suojaa

| Toimialue | Mitä tallennetaan |
|---|---|
| **Docker-kontit** | Appdata-hakemisto sekä kontin määritys (image, ympäristömuuttujat, portit, tunnisteet, taltiot). |
| **KVM / libvirt -virtuaalikoneet** | VM:n levykuva(t), XML-määritys ja UEFI NVRAM, varmuuskopioituna SSH:n yli (ei libvirt-liitosta). |
| **Unraid flash** | Koko USB-flash (`/boot`): käyttöjärjestelmä, lisenssi, array-määritys, jaot, verkko ja laajennusten määritys. |
| **Sovelluksen asetukset** | BombVaultin oma `/config`: sen asetustietokanta, etätunnukset ja libvirt-SSH-avainpari. |
| **Tiedostot ja kansiot** | Nimetyt **tiedostojoukot**, mikä tahansa palvelimen kansio, kukin valinnaisin joukkokohtaisin poissulkukuvioin. |
| **ZFS-tietojoukot** | Tietojoukko kaikkine alapuolisine tietojoukkoineen, luettuna yhdestä ZFS-tilannevedoksesta ja tallennettuna kuten kansio. Katso [ZFS-tietojoukot](zfs-datasets.md). |

## Palautus on tähti

Kopioituaan datan takaisin restic-tilannevedoksesta BombVault toistaa tallennetun kontin määrityksen Docker-rajapintaa vasten, joten kontti ilmestyy takaisin Unraidin Docker-välilehdelle aivan kuin se olisi aina ollut siellä (sama image, samat asetukset, samat porttikartoitukset). Virtuaalikoneiden XML määritellään uudelleen SSH:n yli, ja niiden levyt sekä UEFI NVRAM liitetään takaisin, vaikka VM olisi poistettu.

Kun varmuuskopiointi pysäyttää riippuvaisia kontteja, ne palaavat oikeassa järjestyksessä: BombVault käynnistää ne uudelleen Compose-määrityksen `depends_on`-järjestyksessä ja odottaa, että kukin raportoi olevansa terve, ennen kuin käynnistää siitä riippuvaiset, joten mikään ei ryntää sellaisen tietokannan tai yhdyskäytävän edelle, joka ei ole vielä pystyssä. Katso [Ominaisuudet](features.md).

## Miten se toimii

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

BombVault käyttää Docker-sokettia pysäyttääkseen kontit ennen varmuuskopiota ja luodakseen ne uudelleen palautuksen jälkeen. Virtuaalikoneita varten se ajaa `virsh`-komennon isännällä SSH:n yli (`qemu+ssh://`) sammuttaakseen virtuaalikoneen hallitusti tai ottaakseen live-tilannevedoksen. Se ei koskaan liitä (bind mount) yhtäkään libvirt-polkua, joten se ei voi häiritä isännän VM Manageria.

BombVault on orkestrointi- ja käyttöliittymäkerros, ei tallennusmoottori. Kaikki varsinainen datan siirto kulkee resticin läpi.

## Pikaopas

Uusi täällä? Siirry kohtaan **[Aloitus](getting-started.md)** asentaaksesi BombVaultin Unraidiin Community Applicationsin kautta ja ajaaksesi ensimmäisen varmuuskopiosi. Tutustu sitten täyteen **[Ominaisuudet](features.md)**-listaan, viritä **[Asetukset](configuration.md)** ja pystytä **[Etäsijainti ja palautus](offsite-recovery.md)**.

Etäsijainti voi haarautua useaan kohteeseen per toimialue yhtä aikaa, vain luku -tilainen **vastaanottimen kojelauta** valvoo näitä kopioita niitä vastaanottavassa laatikossa, ja voit kantaa koko kokoonpanosi uuteen laatikkoon **Vie / tuo asetukset** -kortilla. Katso [Etäsijainti ja palautus](offsite-recovery.md) ja [Asetukset](configuration.md#portable-settings-export-and-import).

**[Android-sovellus](android.md)** tuo ryhmäsi kaikki palvelimet puhelimeesi, kaikkien toimintaloki yhdellä näytöllä.

## Kiitokset {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)**, tekijänä [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0), antoi BombVaultille sen lähtöajatuksen: Docker-konttien varmuuskopioinnin yhdellä napsautuksella ja automaattisen uudelleenasennuksen. BombVault on erillinen, Gon ja resticin päälle rakennettu toteutus, joka vie ajatuksen virtuaalikoneisiin, flashiin ja muuhun.
- **[restic](https://restic.net/)** on nopea, turvallinen ja deduplikoiva varmuuskopiomoottori, jota BombVault ohjaa.
- **[rclone](https://rclone.org/)** tarjoaa pilvitaustajärjestelmät.
- Useimmat painikkeiden symbolit ovat ilmaisesta Core Solid -sarjasta, jonka on tehnyt **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [lähde](https://github.com/webalys-hq/streamline-vectors)). Loput ovat sarjoista Font Awesome Free, Material Design Icons, Simple Icons ja Tabler Icons, tai ne on piirretty projektia varten.

## Lisenssi {#license}

Copyright (C) 2026 Junker der Provinz. BombVault on vapaa ohjelmisto, jota koskee **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Saat ajaa, tutkia, jakaa ja muuttaa sitä. Jos levität sitä tai ajat muutettua versiota verkkopalveluna, sinun on julkaistava lähdekoodisi saman lisenssin alla ja säilytettävä olemassa olevat tekijänoikeus- ja nimeämismerkinnät.

Nimeä ja brändiä ei ole lisensoitu. AGPL kattaa vain lähdekoodin: "BombVault", sen logo ja sen brändi pysyvät varattuina, joten haaran (forkin) on käytettävä omaa nimeään ja brändiään eikä se saa esittää olevansa BombVault.

## Linkit

- **Lähdekoodi:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid-tukiketju:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Ongelmat:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-tasoinen isännän hallinta"
    Docker-soketin kautta BombVault voi pysäyttää, poistaa ja luoda uudelleen kontteja sekä lukea ja kirjoittaa appdataa, ja VM-varmuuskopiointia varten se kirjautuu isäntään SSH:n yli ajaakseen `virsh`-komennon. Kuka tahansa, joka pääsee sen verkkokäyttöliittymään, hallitsee käytännössä isäntää root-oikeuksin. Aja BombVaultia vain luotetussa, altistamattomassa verkossa, ja ota valinnainen salasanaportti käyttöön (Asetukset, Turvallisuus) heti kun käytät etä- tai muuttumattomia varmuuskopioita. Katso koko turvallisuusmalli kohdasta [Asetukset](configuration.md).
