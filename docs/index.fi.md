# BombVault

**Unraid-datasi, sinetöitynä holviin. Pudota varmuuskopio. Räjäytä palautus.**

![Yleiskatsaus: tilapalkki, mikä kaipaa sinua, kunkin osion suojaus, kohteet ja historia.](assets/screenshots/overview/home-dark.png#only-dark)
![Yleiskatsaus: tilapalkki, mikä kaipaa sinua, kunkin osion suojaus, kohteet ja historia.](assets/screenshots/overview/home-light.png#only-light)

BombVault on itse isännöity verkkosovellus, jolla **varmuuskopioit ja palautat täydellisesti** Unraidissa ja muissa Docker-isännissä, kuten TrueNAS SCALEssa, ZimaOS:ssa tai tavallisessa Linux-koneessa. Se varmuuskopioi Docker-kontit, KVM/libvirt-virtuaalikoneet, kansiot, ZFS-tietojoukot ja Unraidin flash-aseman. Se pyörii yhtenä multi-arch Docker-konttina, tarjoaa verkkokäyttöliittymän tummalla, vaalealla tai järjestelmän teemalla ja hoitaa koko elinkaaren: varmuuskopiointi, ajastus, tarkistus ja palautus.

Palautukset ovat automaattisia. Unraidissa kontit ilmestyvät takaisin Docker-välilehdelle täsmälleen ennallaan, ja virtuaalikoneet määritellään uudelleen VM Managerissa levyineen ja UEFI NVRAM -muisteineen. Ei manuaalista uudelleenasennusta, ei uudelleenmääritystä, ei draamaa. TrueNASissa alkuperäisen viereen palautettu kontti pyörii tavallisena Docker-konttina, jota Apps-näkymä ei listaa.

Käyttövoimana [restic](https://restic.net), joten jokainen varmuuskopio on deduplikoitu, inkrementaalinen ja oletuksena salattu.

!!! note "Pidä APP_KEY turvassa"
    BombVault johtaa restic-arkiston salasanan 32-tavuisesta salaisuudesta nimeltä `APP_KEY`. Sen menettäminen yhdessä palautuspaketin kanssa tekee salatuista varmuuskopioista palautuskelvottomia. Luo sellainen komennolla `openssl rand -hex 32` ja säilytä se turvallisessa paikassa. Katso [Asetukset](configuration.md).

## Mitä BombVault suojaa {#what-bombvault-protects}

Jokainen kontti, virtuaalikone, kansiojoukko, flash, ZFS-tietojoukko ja BombVaultin omat asetukset on **merkintä**, jolla on oma sivunsa. Samanlaiset merkinnät muodostavat **osion**, jonka otat käyttöön kohdassa **Asetukset, Yleiset**.

| Osio | Mitä tallennetaan |
|---|---|
| **Kontit** | Appdata-hakemisto sekä kontin määritys (image, ympäristömuuttujat, portit, tunnisteet, taltiot). |
| **VMs** | VM:n levykuva(t), XML-määritys ja UEFI NVRAM, varmuuskopioituna SSH:n yli (ei libvirt-liitosta). |
| **Flash** | Koko USB-flash (`/boot`): käyttöjärjestelmä, lisenssi, array-määritys, jaot, verkko ja laajennusten määritys. |
| **Itsevarmuuskopio** | BombVaultin oma `/config`: sen asetustietokanta, kohteidensa tunnukset, libvirt-SSH-avainpari, ZFS-palvelinten avain ja kiinnitetyt isäntäavaimet sekä varmenteet. |
| **Kansiot** | Mikä tahansa palvelimen kansio kansiojoukkona, kukin valinnaisin poissulkukuvioin. |
| **ZFS-tietojoukot** | Tietojoukko kaikkine alapuolisine tietojoukkoineen, luettuna yhdestä ZFS-tilannevedoksesta ja tallennettuna kuten kansio. Katso [ZFS-tietojoukot](zfs-datasets.md). |

## Palautus on tähti {#restore-is-the-star}

Kopioituaan datan takaisin restic-varmuuskopiosta BombVault toistaa tallennetun kontin määrityksen Docker-rajapintaa vasten, joten kontti ilmestyy takaisin Unraidin Docker-välilehdelle aivan kuin se olisi aina ollut siellä (sama image, samat asetukset, samat porttikartoitukset). Virtuaalikoneiden XML määritellään uudelleen SSH:n yli, ja niiden levyt sekä UEFI NVRAM liitetään takaisin, vaikka VM olisi poistettu.

Yksi ikkuna palauttaa kaikenlaiset merkinnät. Oletuksena valittuna on **Alkuperäisen viereen**, joka tuo varmuuskopion takaisin kopiona ja jättää alkuperäisen ennalleen. **Korvaa alkuperäinen** varmuuskopioi ensin nykytilan, joten palautuksen voi perua. Kansio tai uusi ZFS-tietojoukko ovat muut kohteet.

Kun varmuuskopiointi pysäyttää riippuvaisia kontteja, ne palaavat oikeassa järjestyksessä: BombVault käynnistää ne uudelleen Compose-määrityksen `depends_on`-järjestyksessä ja odottaa, että kukin raportoi olevansa terve, ennen kuin käynnistää siitä riippuvaiset, joten mikään ei ryntää sellaisen tietokannan tai yhdyskäytävän edelle, joka ei ole vielä pystyssä. Katso [Palautus](restore.md) ja [Komennot ja muut kontit](backups.md#commands).

## Miten se toimii {#how-it-works}

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + sign-ins (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault käyttää Docker-sokettia pysäyttääkseen kontit ennen varmuuskopiota ja luodakseen ne uudelleen palautuksen jälkeen. Virtuaalikoneita varten se ajaa `virsh`-komennon isännällä SSH:n yli (`qemu+ssh://`) sammuttaakseen virtuaalikoneen hallitusti tai ottaakseen live-tilannevedoksen. Se ei koskaan liitä (bind mount) yhtäkään libvirt-polkua, joten se ei voi häiritä isännän VM Manageria.

BombVault on orkestrointi- ja käyttöliittymäkerros, ei tallennusmoottori. Kaikki varsinainen datan siirto kulkee resticin läpi.

## Pikaopas {#quick-start}

Uusi täällä? Siirry kohtaan **[Aloitus](getting-started.md)** asentaaksesi BombVaultin ja ajaaksesi ensimmäisen varmuuskopiosi, ja pystytä sitten **[Etäsijainti ja palautus](offsite-recovery.md)**. **[Esimerkit vaihe vaiheelta](guides.md)** käyvät yleiset tehtävät läpi alusta loppuun, **[Asetukset](settings.md)**-sivu listaa jokaisen ruudun ja **[Ominaisuudet](features.md)** on lyhyt luettelo kaikesta, mitä BombVault osaa.

Kukin osio voi kopioida usealle kohteelle yhtä aikaa, pariliitoksella liitetty BombVault voi säilyttää etäkopiosi palvelimessa, joka kieltäytyy poistoista, ja **Hätäohjattu toiminto** palauttaa kadonneen palvelimen palautuspaketista, ryhmäsi kautta tai kirjoitetulla avaimella. Kokoonpanon siirrät uuteen laatikkoon **Vie / tuo asetukset** -kortilla. Katso [Etäsijainti ja palautus](offsite-recovery.md) ja [Asetukset](configuration.md#portable-settings-export-and-import).

**[Android-sovellus](android.md)** tuo ryhmäsi kaikki palvelimet puhelimeesi, kaikkien toimintaloki yhdellä näytöllä.

Dokumentaatio on saatavilla 26 kielellä ja käyttöliittymä 42 kielellä. Sivut, joilla ei vielä ole käännöstä, näkyvät englanniksi.

## Käytetyt sanat {#glossary}

Muutamalla sanalla on BombVaultissa ja näillä sivuilla kiinteä merkitys.

| Sana | Merkitys |
|---|---|
| **Merkintä** | Yksi asia, jonka BombVault varmuuskopioi: kontti, virtuaalikone, kansiojoukko, flash-asema, ZFS-tietojoukko alapuolisineen tai sen omat asetukset. Jokaisella on oma sivunsa. |
| **Osio** | Kaikki yhdenlaiset merkinnät, esimerkiksi Kontit. Otat osiot käyttöön kohdassa **Asetukset, Yleiset**. |
| **Kohde** | Paikka, johon varmuuskopiot menevät: kansio tällä palvelimella, jako, oma palvelin tai pilvipalvelu. Katso [Kohteet](offsite-recovery.md#destinations). |
| **Paikallinen** | Tämän palvelimen kohde, joka sisältää osioiden kansiot. |
| **Oma kansio** | Arkisto, johon osio kirjoittaa varmuuskopionsa ensin. Se on oletuksena tällä palvelimella ja voi sijaita kohteessa. Katso [Osion oma kansio](offsite-recovery.md#own-folder). |
| **Kopio** | Omasta kansiosta toiseen kohteeseen `restic copy` -komennolla kopioitu varmuuskopio. |
| **Suora arkisto** | Etäsijainnin arkisto, johon merkinnät varmuuskopioivat suoraan ilman paikallista kopiota välissä. Katso [Suorat arkistot](offsite-recovery.md#direct-repositories). |
| **Kirjautuminen** | S3-palvelun tai rest-serverin tallennettu avain tai salasana. Yksi niistä on oletus. Katso [Kirjautumiset](offsite-recovery.md#sign-ins). |
| **Sijoittelu** | Mihin kohteisiin merkinnän varmuuskopiot menevät. Se asetetaan merkinnälle tai osion oletuksena. Katso [Sijoittelu merkinnän mukaan](offsite-recovery.md#placement). |
| **Replika** | ZFS-merkinnän lohkotarkka kopio toisella ZFS-palvelimella, tehty `zfs send` -komennolla. Se on varmuuskopion rinnalla eikä korvaa sitä. Katso [Replika](zfs-datasets.md#replica). |
| **Rooli** | Jotain, mitä ryhmäsi yksi instanssi tekee toiselle: Vastaanottaja, Noutaja, ZFS-palvelin tai Avaimenhaltija. Katso [Instanssit ja roolit](offsite-recovery.md#roles). |

## Kiitokset {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)**, tekijänä [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0), antoi BombVaultille sen lähtöajatuksen: Docker-konttien varmuuskopioinnin yhdellä napsautuksella ja automaattisen uudelleenasennuksen. BombVault on erillinen, Gon ja resticin päälle rakennettu toteutus, joka vie ajatuksen virtuaalikoneisiin, flashiin ja muuhun.
- **[restic](https://restic.net/)** on nopea, turvallinen ja deduplikoiva varmuuskopiomoottori, jota BombVault ohjaa.
- **[rclone](https://rclone.org/)** tarjoaa pilvitaustajärjestelmät.
- Useimmat painikkeiden symbolit ovat ilmaisesta Core Solid -sarjasta, jonka on tehnyt **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [lähde](https://github.com/webalys-hq/streamline-vectors)). Loput ovat sarjoista Font Awesome Free, Material Design Icons, Simple Icons ja Tabler Icons, tai ne on piirretty projektia varten.

## Lisenssi {#license}

Copyright (C) 2026 Junker der Provinz. BombVault on vapaa ohjelmisto, jota koskee **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Saat ajaa, tutkia, jakaa ja muuttaa sitä. Jos levität sitä tai ajat muutettua versiota verkkopalveluna, sinun on julkaistava lähdekoodisi saman lisenssin alla ja säilytettävä olemassa olevat tekijänoikeus- ja nimeämismerkinnät.

Nimeä ja brändiä ei ole lisensoitu. AGPL kattaa vain lähdekoodin: "BombVault", sen logo ja sen brändi pysyvät varattuina, joten haaran (forkin) on käytettävä omaa nimeään ja brändiään eikä se saa esittää olevansa BombVault.

## Linkit {#links}

- **Lähdekoodi:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid-tukiketju:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Ongelmat:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-tasoinen isännän hallinta"
    Docker-soketin kautta BombVault voi pysäyttää, poistaa ja luoda uudelleen kontteja sekä lukea ja kirjoittaa appdataa, ja VM-varmuuskopiointia varten se kirjautuu isäntään SSH:n yli ajaakseen `virsh`-komennon. Kuka tahansa, joka pääsee sen verkkokäyttöliittymään, hallitsee käytännössä isäntää root-oikeuksin. Aja BombVaultia vain luotetussa, altistamattomassa verkossa, ja aseta kirjautumissalasana kohdassa **Asetukset, Turvallisuus**. Palautuspaketti, avaintalletukset, flashin ZIP-tiedosto, flash- tai itsevarmuuskopion vienti ja kohteen varmuuskopioiden poistaminen vaativat sen. Katso koko turvallisuusmalli kohdasta [Asetukset](configuration.md#security-model).
