# Aloitus

Tämä sivu opastaa sinut tuoreesta Unraid-laatikosta ensimmäiseen varmuuskopioosi.

## Vaatimukset {#requirements}

| Vaatimus | Huomiot |
|---|---|
| **Unraid 6.12+** | Aiempia versioita ei ole testattu. Unraid on pääalusta, mutta BombVault toimii myös tavallisella Docker-isännällä ja TrueNAS Scalella (katso [Yleinen Docker-isäntä](#generic-docker-host) ja [TrueNAS SCALE](#truenas)). |
| **Restic-repon sijainti** | Paikallinen polku (suositus: array tai cache), SMB, NFS tai mikä tahansa rclone-taustajärjestelmä. |
| **Docker-soketti** | Malli liittää automaattisesti (`/var/run/docker.sock`). |
| **Unraidin flash** (`/boot`) | Malli liittää kokonaan automaattisesti (`/boot` kohteeseen `/host/boot`). Mahdollistaa flash-varmuuskopioinnin ja sen, että palautettu kontti ilmestyy takaisin normaalina, muokattavana Unraid-sovelluksena. |
| **KVM-virtuaalikoneet** (valinnainen) | VM-varmuuskopiointi keskustelee libvirtin kanssa SSH:n yli, ei libvirt-liitosta. Määritä se kohdassa **Asetukset, Integraatiot, Palvelimen SSH** (katso [Kokoonpano](configuration.md)). |
| **ZFS-tietojoukot** (valinnainen) | Sama SSH-yhteys kuin virtuaalikoneiden varmuuskopioinnissa, `zfs` isännällä ja Host Data liitettynä polkuun `/mnt` käyttötilalla Read/Write - Slave, joka on mallin oletus. Katso [ZFS-tietojoukot](zfs-datasets.md). |
| **Android-sovellus** (valinnainen) | Android 10 tai uudempi, pariliitettynä palvelimiin, joiden versio on 9.7.0 tai uudempi. Katso [Android-sovellus](android.md). |

## Asennus Unraidiin {#install-on-unraid}

Helpoin reitti on **Community Applications**.

1. Avaa **Apps**-välilehti Unraidissa.
2. Hae **BombVault**.
3. Napsauta **Install**, aseta vaaditut muuttujat (alla) ja ota käyttöön.

!!! tip "Mallin manuaalinen asennus"
    Jos haluat lisätä mallin käsin:

    1. Mene kohtaan **Docker, Add Container, Template repositories** ja lisää:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Hae **BombVault** malleista.
    3. Aseta vaaditut muuttujat ja napsauta **Apply**.

## Yleinen Docker-isäntä {#generic-docker-host}

Etkö käytä Unraidia? BombVault toimii myös tavallisena konttina millä tahansa Docker-isännällä. TrueNAS SCALEn omat vaiheet ovat kohdassa [TrueNAS SCALE](#truenas).

1. Hae repon valmis muokattava [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml).
2. Aseta `APP_KEY` (katso alla) ja osoita Host Data -taltio todelliseen datajuureesi. Tiedoston kommentit opastavat molemmat.
3. `docker compose up -d`, avaa sitten `https://<host-ip>:3443/`.

Mikä on erilaista kuin Unraidissa:

- **Ei Flash-osiota.** Käynnistys-USB:tä ei ole taltioitavaksi tai palautettavaksi, joten Flash-osiolla ei ole täällä mitään tehtävää. Sen sijaan, kun Host Data ulottuu isännän kansioon `/etc`, **Lisää** Varmuuskopiot-sivulla tarjoaa kansioehdotuksena **Järjestelmän asetukset**: `/etc`-kansiojoukon aloituspohjan, jonka tarkistat ja muokkaat ennen tallennusta.
- **Ei Unraidin omia ilmoituksia.** BombVaultin omat sovelluksen sisäiset ilmoituskanavat (webhook, etäkopion epäonnistumishälytykset jne.) toimivat tavalliseen tapaan. Vain Unraid-kohtainen työntö sen omaan ilmoitusjärjestelmään ohitetaan, koska sellaista järjestelmää ei ole, johon työntää.
- **VM-varmuuskopiointi on valinnainen ja tarvitsee erillisen libvirtd-isännän, joka tavoitetaan SSH:n yli.** Katso compose-tiedoston kommentoitu lohko. Tavallisessa Docker-isännässä ei ole sisäänrakennettua VM-hallintaa.
- **Ei kojelautawidgetiä.** BombVault Widget on Unraid-liitännäinen, joten sekin vaihe ohitetaan.
- **Kontin datan löytäminen.** Ilman Unraidin `appdata`-käytäntöä kontin datakansio löytyy `DATA_ROOT_SEGMENTS`-osista, Dockerin nimetyistä taltioista, Compose-projektin työhakemistosta ja `bombvault.data`-tunnisteesta (katso [Varmuuskopiolähteen tunnistus](configuration.md#backup-source-detection)). Nimetyt taltiot ja `/etc`-esiasetus yltävät vain polkuihin Host Data -liitoksen sisällä, joten osoita Host Data yhteiseen kantahakemistoon, joka kattaa myös Dockerin datajuuren.
- **`PLATFORM`.** Aseta se arvoon `generic` tai `truenas`. Asettamatta BombVault tunnistaa Unraidin sen oman merkin perusteella flash-liitoksesta ja käsittelee kaiken muun yleisenä, ja vain Unraidille tarkoitetut vaiheet ohitetaan sen sijaan, että niitä yritettäisiin ja ne epäonnistuisivat.

### TrueNAS SCALE {#truenas}

BombVault ei ole vielä TrueNASin sovelluskatalogissa, joten asenna se mukautettuna sovelluksena: **Apps, Discover Apps**, sitten **Install via YAML** valikossa **Custom App** -painikkeen vieressä. Alla oleva YAML asettaa samat asiat kuin repon `truenas-apps`-kansion katalogimerkintä. Korvaa `tank` poolillasi ja täytä `APP_KEY` kuten seuraava osa kuvaa.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # For VM backups:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

Avaa sitten `https://<truenas-ip>:30528/`. Se, mikä on listattuna kohdassa [Mikä on erilaista kuin Unraidissa](#generic-docker-host), pätee myös tässä, ja TrueNASissa lisäksi:

- **Oikeudet.** BombVault tarvitsee ylläolevat seitsemän. VM-varmuuskopiot epäonnistuvat ilman `SETPCAP`-oikeutta, ja ilman `SYS_CHROOT`-oikeutta jokainen kontti pysäytetään koko varmuuskopioinnin ajaksi sen sijaan, että se luettaisiin tilannevedoksesta.
- **Host Data** on isäntäpolku tietojoukolla, tässä `/mnt/tank`, liitettynä samaan polkuun sovelluksen sisällä. Siksi sekä `HOST_SOURCE_ROOT` että `HOST_MOUNT_ROOT` nimeävät sen. `rslave` välittää liitokset, jotka isäntä tekee myöhemmin, mitä ZFS-tietojoukkojen varmuuskopiointi tarvitsee.
- **ixVolumessa olevaa dataa ei varmuuskopioida.** Sovellus, jonka tallennustila on ixVolume, pitää datansa polun `/mnt/.ix-apps` alla, Host Datan ulkopuolella. BombVault tallentaa vain sovelluksen määrityksen, ja ajo nimeää kansiot, jotka se jätti pois. Valitse jokaiselle sovellukselle, jonka datan haluat varmuuskopioitavaksi, **Host Path** -tallennustila tietojoukolla Host Datan alla.
- **Virtuaalikoneiden ja ZFS-tietojoukkojen varmuuskopiot** tavoittavat TrueNASin SSH:n yli rootina. Kytke SSH-palvelu päälle kohdassa **System, Services** ja lisää julkinen avain kohdasta **Asetukset, Integraatiot, Palvelimen SSH** root-käyttäjälle kohdassa **Credentials, Users**. VM-varmuuskopiot tarvitsevat lisäksi `LIBVIRT_URI`-arvon kuten yllä, koska TrueNASin libvirtd kuuntelee omaa soketiaan. Ilman `LIBVIRT_URI`- ja `LIBVIRT_HOST`-arvoja BombVault käyttää sovelluksen oletusyhdyskäytävää isäntänä, koska `host.docker.internal` ei ratkea TrueNAS-sovelluksessa. [VM-varmuuskopiointiopas](vm-backup-ssh-setup.md#truenas-scale) käsittelee yksityiskohdat, zvol-levyillä olevat virtuaalikoneet mukaan lukien.
- **ZFS-replika käyttää eri avainta.** Toisella ZFS-palvelimella oleva replika kirjautuu avaimella, jonka replikaikkuna näyttää, ei Host SSH -avaimella. Lisää se avain vastaanottavalle palvelimelle.
- **Poistettu sovellus palaa kahdessa vaiheessa.** Asenna se ensin uudelleen TrueNASissa samalla nimellä, mikä luo sen verkon uudelleen, ja palauta sitten sen data BombVaultissa. Siihen asti palautustarkistus raportoi puuttuvan verkon, ja palautus kieltäytyy ennen kuin se muuttaa mitään.
- **Alkuperäisen viereen** luo tavallisen Docker-kontin. Se toimii, mutta TrueNASin Apps-näkymä ei listaa sitä.
- **Tietokantavedokset** toimivat tietojoukkojen liitoksilla. Tuonti datakansioon, joka on oma tietojoukkonsa, siirtää vanhan sisällön viereiseen kansioon, joten yläpuolella olevan tietojoukon täytyy mahtua ne.

### ZimaOS, CasaOS ja muut isännät {#other-hosts}

- **ZimaOS ja CasaOS:** lisää [yhteisön kauppa](https://github.com/junkerderprovinz/zimaos-apps#install) ja asenna BombVault sieltä. Se ajaa siellä asetuksella `PLATFORM=generic` ja Host Data polussa `/DATA`. ZimaOS pitää sovellusdatan polussa `/DATA/AppData/<app>`, jonka BombVault löytää itse, koska `DATA_ROOT_SEGMENTS` täsmää `appdata`-osaan kirjainkoosta riippumatta.
- **MOS:** asenna BombVault [MOS Hubista](https://mos-official.net/apps.html?search=BombVault). Se ajaa siellä asetuksella `PLATFORM=generic` ja Host Data polussa `/mnt/cache`.
- **Synology:** luo Container Managerissa projekti compose-tiedostosta (DSM 7.2 tai uudempi).
- **Proxmox:** aja se virtuaalikoneessa tai LXC-kontissa, jossa on Docker.

Ilman kommentteja yleinen compose-tiedosto on lyhyt. Korvaa `./data` isännän kansiolla, joka sisältää konttiesi datan, aseta `HOST_SOURCE_ROOT` samaksi poluksi ja täytä `APP_KEY`:

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    restart: unless-stopped
    hostname: bombvault
    container_name: bombvault
    volumes:
      - ./config:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/host/user
    environment:
      APP_KEY: ""
      PLATFORM: generic
      HOST_SOURCE_ROOT: /host/user
    ports:
      - "3443:3443"
```

Varmuuskopioidaksesi myös Dockerin nimetyt taltiot liitä kansio, joka sisältää sekä datasi että Dockerin datajuuren (`docker info -f '{{.DockerRootDir}}'`, yleensä `/var/lib/docker`), samaan tapaan kuin Unraid-malli liittää koko `/mnt`:n.

## Yksi pakollinen asetus {#the-one-required-setting}

Ainoa muuttuja, joka sinun täytyy asettaa, on `APP_KEY`, 32-tavuinen heksasalaisuus (64 heksamerkkiä), jolla johdetaan resticin repon salasana.

Luo sellainen millä tahansa koneella:

```bash
openssl rand -hex 32
```

Liitä tulos mallin `APP_KEY`-kenttään (Unraid) tai `docker-compose.yml`-tiedoston ympäristömuuttujaan `APP_KEY` (yleinen Docker-isäntä).

!!! danger "Älä hukkaa APP_KEYtäsi"
    `APP_KEY`n menettäminen tekee salatuista varmuuskopioistasi palauttamattomia. Säilytä se turvallisessa paikassa erillään palvelimesta. Kun BombVault on käynnissä, aseta kirjautumissalasana ja lataa **palautuspaketti** kohdassa **Asetukset, Palautus** (katso [Etäsijainti ja palautus](offsite-recovery.md#recovery-kit)). Se sisältää avaimen, jokaisen kohteen ja resticin komennot.

Malli liittää puolestasi myös Docker-soketin, flashin (`/boot`) ja **Host Data** -juuren (`/mnt`). Varmuuskopioiden *lähteet* ja *kohteet* sijaitsevat molemmat Host Datan alla. Täydellisen muuttujaviitteen ja etäkohteiden määrityksen löydät sivulta [Kokoonpano](configuration.md).

## Ensimmäinen käyttökerta {#first-run}

![Uuden asennuksen tervetuloasivu, jossa näkyy, mitä BombVault löysi palvelimelta.](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![Uuden asennuksen tervetuloasivu, jossa näkyy, mitä BombVault löysi palvelimelta.](assets/screenshots/getting-started/welcome-light.png#only-light)

1. Avaa verkkokäyttöliittymä osoitteessa `https://<your-unraid-ip>:3443` (itse allekirjoitettu varmenne valmiina). Uusi asennus avautuu sivulle **Tervetuloa BombVaultiin**, joka listaa, mitä BombVault löysi palvelimelta.
2. Valitse **Aloitetaan**. Yleiskatsaus näyttää sitten **Ensimmäiset vaiheet** -kortin kolmella vaiheella:
    1. **Valitse osiot**: kytke päälle se, mitä BombVault varmuuskopioi (Kontit, VMs, Flash, Kansiot, ZFS-tietojoukot, Itsevarmuuskopio). **Valitse** avaa osion merkintöjen listan, jossa **Keskeytä aikataulu** jättää yhden pois. Täällä päälle kytketyt osiot ajetaan joka yö, minkä muutat kohdassa **Asetukset, Aikataulut**.
    2. **Lisää kohde**: toinen paikka varmuuskopioille. Jokaisella osiolla on jo kansio tällä palvelimella, `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`, joka luodaan ensimmäisessä varmuuskopiossa. Turvallisin on toinen kohde tiloista ulkona, kuten Storage Box tai Backblaze B2.
    3. **Ensimmäinen varmuuskopio**: sen jälkeen se ajetaan itsestään joka yö. Kortti pysyy, kunnes ensimmäinen varmuuskopio on valmis.
3. Avaa **Varmuuskopiot** nähdäksesi jokaisen merkinnän yhdessä listassa. **Varmuuskopioi kaikki nyt** käynnistää ajon kaikille päällä oleville osioille, ja **Varmuuskopioi nyt** merkinnän sivulla varmuuskopioi vain sen.

![Ensimmäiset vaiheet -kortti kolmine vaiheineen, toinen auki.](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![Ensimmäiset vaiheet -kortti kolmine vaiheineen, toinen auki.](assets/screenshots/getting-started/first-steps-light.png#only-light)

Oletko käyttänyt BombVaultia ennen? **Palauta varmuuskopiosta** tervetuloasivulla avaa Palautus-sivun, jossa **Hätäohjattu toiminto** tuo takaisin kadonneen palvelimen asetukset, kontit, virtuaalikoneet ja kansiot. Katso [Etäsijainti ja palautus](offsite-recovery.md#emergency).

!!! tip "Valinnainen: valitse varmuuskopiointijärjestys"
    Jos joidenkin konttien pitäisi aina varmuuskopioitua ennen toisia (esimerkiksi tietokanta ennen sitä käyttävää sovellusta), avaa **Järjestys**-kortti kohdassa **Asetukset, Aikataulut** ja vedä ne haluamaasi järjestykseen. Ajastetut ajot ja täysvarmuuskopio seuraavat sitä. Kaikki, mitä jätät järjestämättä, varmuuskopioidaan eniten myöhässä oleva ensin.

## Esimerkki: ensimmäinen konttivarmuuskopiosi alusta loppuun {#example-first-backup}

Tässä käydään läpi tuore Unraid-laatikko, jossa on muutama kontti, muun muassa `plex`, kunnes ensimmäinen varmuuskopio on valmis ja tiedät, mistä sen löydät. Siihen menee noin kymmenen minuuttia sekä ensimmäisen varmuuskopion aika.

1. **Asenna.** Hae Unraidin **Apps**-välilehdellä **BombVault** ja paina **Install**.
2. **Aseta avain.** Aja tietokoneella `openssl rand -hex 32` ja liitä 64 merkkiä mallin `APP_KEY`-kenttään. Tallenna kopio salasanahallintaasi nyt, ennen mitään muuta. Paina **Apply**.
3. **Avaa se.** Mene osoitteeseen `https://<your-unraid-ip>:3443` ja hyväksy itse allekirjoitettu varmenne. **Tervetuloa BombVaultiin** listaa, mitä se löysi, esimerkiksi 12 konttia, 2 virtuaalikonetta ja flash-aseman.
4. **Aloitetaan.** Yleiskatsaus avautuu **Ensimmäiset vaiheet** -kortilla.
5. **Valitse osiot.** **Kontit** on päällä alusta asti. Kytke päälle muut haluamasi, kuten **Flash**. **Valitse** avaa löytämiensä konttien listan, jossa **Keskeytä aikataulu** jättää yhden pois, esimerkiksi testikontin, josta et välitä. Päälle kytketyt osiot ajetaan joka yö täysvarmuuskopiossa.
6. **Lisää kohde.** Jokaisella osiolla on jo oma kansionsa tällä palvelimella, polun `/mnt/user/bombvault` alla, joten ensimmäisellä varmuuskopiolla on paikka mihin mennä. Tämä vaihe lisää toisen paikan, ja turvallisin on tiloista ulkona. Voit tehdä sen nyt [S3-esimerkin](offsite-recovery.md#example-s3) avulla tai palata siihen, kun ensimmäinen varmuuskopio on valmis. Kortti pysyy siihen asti.
7. **Ensimmäinen varmuuskopio.** Avaa **Varmuuskopiot** ja paina **Varmuuskopioi kaikki nyt** tai odota yötä. Tilapään rengas muuttuu edistymisrenkaaksi ja laskee merkinnät, esimerkiksi **Varmuuskopiointi käynnissä: 3 / 12**. Jokainen kontti pysäytetään datan lukemisen ajaksi ja käynnistetään heti sen jälkeen uudelleen. Kontti, jonka data on ZFS:llä, on alhaalla vain tilannevedoksen muutaman sekunnin ajan. Ensimmäinen varmuuskopio lukee kaiken, joten se kestää pisimpään. Myöhemmät tallentavat vain sen, mikä muuttui.
8. **Tarkista se.** Kun rengas sanoo **Kaikki on varmuuskopioitu**, avaa **Historia** yleiskatsauksessa: jokaisella kontilla on vihreä rivi kohdassa **Ajot**. Avaa **Varmuuskopiot**, napsauta `plex` ja katso sen **Varmuuskopiot**-välilehteä: ensimmäinen tilannevedos on siellä, ja sen sisältävä paikka on **Paikallinen**. Muutaman minuutin kuluttua sen **Tarkista**-välilehti näyttää palautustarkistuksen, joka palautti otoksen varmuuskopiosta ja luki sen takaisin.
9. **Suojaa avain.** Avaa **Asetukset, Turvallisuus** ja aseta kirjautumissalasana. Avaa sitten **Asetukset, Palautus** ja paina **Lataa palautuspaketti** kohdassa **Hätätilanteiden varalle**. Säilytä tiedosto palvelimen ulkopuolella vaiheen 2 `APP_KEY`-kopion vieressä. Yleiskatsauksen muistutus **Tallenna palautuspakettisi** katoaa, kun painat **Olen tallentanut sen turvallisesti**.

Tästä lähtien kontit varmuuskopioidaan joka yö. Hyviä seuraavia askelia tässä järjestyksessä: [etäkohde](offsite-recovery.md#example-s3), jotta kuollut palvelin ei vie varmuuskopioita mukanaan, sitten [ilmoitukset](notifications.md), jotta epäonnistuminen tavoittaa sinut, ja sitten katse [Aikatauluihin](schedules.md), jos kello 03.00 ei sovi sinulle.

![Tilapää ensimmäisen varmuuskopion aikana, edistymisrengas ja sen vaiheet näkyvissä.](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![Tilapää ensimmäisen varmuuskopion aikana, edistymisrengas ja sen vaiheet näkyvissä.](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "Isäntäintegraation tarkistus"
    Yleiskatsauksen **Isäntäintegraatio**-kortti koettelee jokaisen liitoksen ja komentorivityökalun (Docker-soketti, libvirt, restic, qemu-img, rclone) ja raportoi puuttuvat osat, joten voit varmistaa kontin olevan kytketty oikein ennen kuin luotat siihen. Se odottaa kohdassa **Mukauta**, kunnes lisäät sen.

## Tutustu käyttöliittymään {#navigation}

![Asetukset ruutuina, valitun ruudun kortit vieressä. Jokainen muutos kirjoitetaan heti, kun teet sen.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Asetukset ruutuina, valitun ruudun kortit vieressä. Jokainen muutos kirjoitetaan heti, kun teet sen.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Sivupalkissa on viisi kohtaa: **Yleiskatsaus**, **Poikkeamat** (kun poikkeamien tunnistus on päällä), **Varmuuskopiot**, **Palautus** ja **Kirjaudu ulos** -kohdan alla **Asetukset**. Sivun pääasiallinen toiminto leijuu oikeassa alakulmassa, ja **Lisää** sisältää harvinaiset. Puhelimella palkissa ovat Yleiskatsaus, Varmuuskopiot, Palautus ja Asetukset, ja **Lisää** sisältää loput.

Sivut ja asetukset käyttävät ikkunan koko leveyden. Ikkuna pitää painikkeensa alareunassaan, eteenpäin vievä oikealla. Puhelimella alareuna, jossa on yli kolme painiketta, pitää ulospääsyn ja pääasiallisen toiminnon ja laittaa muut kohtaan **Lisää**.

![Puhelimella alareunan palkki Lisää avattuna.](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![Puhelimella alareunan palkki Lisää avattuna.](assets/screenshots/getting-started/phone-more-light.png#only-light)

- **Yleiskatsaus** näyttää, onko kaikki varmuuskopioitu ja mikä vaatii sinua. Katso [Yleiskatsaus](overview.md).
- **Varmuuskopiot** listaa jokaisen kontin, virtuaalikoneen, kansiojoukon, flash-aseman, jokaisen ZFS-tietojoukon ja itsevarmuuskopion merkintöinä. Listan yläpuolella olevat ruudut valitsevat lajin, ja **Suodattimet**, **Lajittelu** ja **Hae** rajaavat sitä. Jokaisella merkinnällä on oma sivunsa kolmella välilehdellä: **Varmuuskopiot**, **Asetukset** ja **Tarkista**. Takaisin-painike merkinnän kuvakkeen vasemmalla puolella johtaa listaan. Katso [Varmuuskopiot ja merkinnät](backups.md).
- **Palautus** on yksi sivu kaikelle: valitse osio, hae merkintä ja valitse varmuuskopio. Hätäohjattu toiminto ja muut lähteet alkavat myös täältä. Katso [Palautus](restore.md).
- **Asetukset** ovat neljätoista ruutua: Yleiset, Ulkoasu, Aikataulut, Kohteet, Palautus, Ilmoitukset, Tarkista, Turvallisuus, Ilmentymät, Pariliitos, Integraatiot, Sovellukset, Järjestelmä ja Tietoja. Katso [Asetukset](settings.md). Paina `/` tai Ctrl+K hakeaksesi niistä kaikista, myös niillä nimillä, jotka sivuilla oli aiemmissa versioissa. Vedä ruudut omaan järjestykseesi kohdassa **Ulkoasu**. Kun sivun vieressä oleva ruutujen sarake puristaisi sitä, kuten noin 1200 pikselin levyisessä ikkunassa, ruuduista tulee sivun yläpuolella oleva kaista, jota voi vierittää sivuttain. Puhelin näyttää saman kaistan.
- **Kohteet** ja **Ilmentymät** ovat asetusruutuja, joilla on omat sivunsa. **Näytä myös sivupalkissa** niiden ruudussa kiinnittää ne sivupalkkiin.

Missään ei ole Tallenna-painiketta eikä mikään aukea ulos: harvinaisilla asioilla on oma korttinsa tai ikkunansa, ja selitykset ovat (i)-kuplissa. Päivityksen jälkeen ikkuna aukeaa itsestään, ja siinä on kortti jokaiselle julkaisulle viimeisimmästä versiostasi lähtien, ja vihreät pisteet merkitsevät sen, mikä muuttui. Napsautus piilottaa pisteen. **Uutta** Versio-kortissa kohdassa **Asetukset, Tietoja** laskee, mitä et ole vielä nähnyt, ja avaa kyseisen ikkunan uudelleen kunkin julkaisun muistiinpanoineen ja muutoksineen ja painikkeella **Siirry sinne**.

## Rakentaminen lähdekoodista {#build-from-source}

BombVault on yksi staattinen Go-binääri, joka tarjoaa JSON-API:n ja sisäänrakennetun React-käyttöliittymän. Rakenna käyttöliittymä ensin ja aja sitten binääri:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

Käyttöliittymän rakennus tarvitaan myös komentoon `go run`. Repo seuraa vain tyhjää merkkiä polun `web/dist` alla, joten ilman komentoa `npm --prefix web run build` binääri ei sisällä mitään ja vastaa `500 SPA index not found`, mikä on odotettua. Dockeria, libvirtiä ja Unraidia ei voi testata CI:ssä, joten tarkista liitokset, restic ja VM:n SSH-yhteys oikealla isännällä yleiskatsauksen **Isäntäintegraatio**-kortilla (tai `GET /api/spike`) ennen kuin avaat pull requestin.

## Seuraavat askeleet {#next-steps}

- Lisää yksi tai useampi kohde tiloista ulos (jokainen osio voi kopioida useaan kerralla) ja tallenna palautuspakettisi kuten kohdassa **[Etäsijainti ja palautus](offsite-recovery.md)** kuvataan. [S3-esimerkki](offsite-recovery.md#example-s3) opastaa yhden läpi.
- Varmista, että epäonnistuminen tavoittaa sinut **[Ilmoitusten](notifications.md)** avulla.
- Tarkista, milloin asiat ajetaan, **[Aikatauluista](schedules.md)**.
- Tuo kaikki ryhmäsi palvelimet puhelimeesi **[Android-sovelluksella](android.md)**.
- **[Esimerkit vaihe vaiheelta](guides.md)** käyvät läpi yleiset työt, ja **[Ominaisuudet](features.md)** listaa kaiken, mitä BombVault tekee.
- Kloonaatko asetuksen tai siirrytkö uuteen laatikkoon? Siirrä koko kokoonpanosi **Vie / tuo asetukset** -kortilla. Katso [Kokoonpano](configuration.md#portable-settings-export-and-import).
- Törmäsitkö ongelmaan? Katso **[Vianmääritys](troubleshooting.md)**.
