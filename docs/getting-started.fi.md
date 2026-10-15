# Aloitus

Tämä sivu opastaa sinut tuoreesta Unraid-laatikosta ensimmäiseen varmuuskopioosi.

## Vaatimukset

| Vaatimus | Huomiot |
|---|---|
| **Unraid 6.12+** | Aiempia versioita ei ole testattu. Unraid on pääalusta, mutta BombVault toimii myös tavallisella Docker-isännällä ja TrueNAS Scalella (katso [Yleinen Docker-isäntä](#generic-docker-host)). |
| **Restic-repon sijainti** | Paikallinen polku (suositus: array tai cache), SMB, NFS tai mikä tahansa rclone-taustajärjestelmä. |
| **Docker-soketti** | Malli liittää automaattisesti (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | Malli liittää kokonaan automaattisesti (`/boot` kohteeseen `/host/boot`). Mahdollistaa flash-varmuuskopioinnin ja sen, että palautettu kontti ilmestyy takaisin normaalina, muokattavana Unraid-sovelluksena. |
| **KVM-virtuaalikoneet** (valinnainen) | VM-varmuuskopiointi keskustelee libvirtin kanssa SSH:n yli, ei libvirt-liitosta. Määritä se kohdassa **Asetukset, Integraatiot, Palvelimen SSH** (katso [Asetukset](configuration.md)). |
| **ZFS-tietojoukot** (valinnainen) | Sama SSH-yhteys kuin virtuaalikoneiden varmuuskopioinnissa, `zfs` isännällä ja Host Data liitettynä polkuun `/mnt` käyttötilalla Read/Write - Slave, joka on mallin oletus. Katso [ZFS-tietojoukot](zfs-datasets.md). |
| **Android-sovellus** (valinnainen) | Android 10 tai uudempi, pariliitettynä palvelimiin, joiden versio on 9.7.0 tai uudempi. Katso [Android-sovellus](android.md). |

## Asennus Unraidiin

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
    2. Hae **BombVault** kohdasta Templates.
    3. Aseta vaaditut muuttujat ja napsauta **Apply**.

## Yleinen Docker-isäntä {#generic-docker-host}

Ei Unraidia? BombVault toimii myös tavallisena konttina millä tahansa Docker-isännällä (tämä kannattelee myös TrueNAS Scalen konttituen, ennen kuin sillä on oma merkintänsä sikäläisessä sovelluskatalogissa).

1. Nouda arkistosta valmiiksi muokattava [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml).
2. Aseta `APP_KEY` (katso alta) ja osoita Host Data -taltio todelliseen datajuureesi. Tiedoston kommentit käyvät läpi molemmat.
3. `docker compose up -d`, avaa sitten `https://<isännän-ip>:3443/`.

Mikä poikkeaa Unraidista:

- **Ei Flash-osiota.** Käynnistys-USB:tä ei ole talteen otettavaksi tai palautettavaksi, joten Flash-osiolla ei ole täällä tehtävää. Sen sijaan Varmuuskopiot-sivun **Lisää** tarjoaa kansioehdotuksena **Järjestelmän asetukset**: aloittavan `/etc`-kansiojoukon, jonka käyt läpi ja muokkaat ennen tallennusta.
- **Ei Unraidin omia ilmoituksia.** BombVaultin omat ilmoituskanavat (webhook, off-site-epäonnistumisen varoitukset ja niin edelleen) toimivat tavalliseen tapaan; pois jää vain Unraid-kohtainen lähetys sen omaan ilmoitusjärjestelmään, koska sellaista järjestelmää ei täällä ole.
- **Virtuaalikoneiden varmuuskopiointi on valinnaista ja vaatii erillisen, SSH:n yli tavoitettavan libvirtd-isännän.** Katso compose-tiedoston kommentoitu lohko. Yleisessä Docker-isännässä ei ole omaa virtuaalikoneiden hallintaa.
- **Ei kojelautawidgetiä.** BombVault Widget on Unraid-laajennus, joten tämäkin vaihe jää pois.
- **Kontin datan löytäminen.** Ilman Unraidin `appdata`-käytäntöä kontin datakansio löydetään `DATA_ROOT_SEGMENTS`-muuttujan segmenttien, Dockerin nimettyjen taltioiden, Compose-projektin työhakemiston ja `bombvault.data`-tunnisteen perusteella (katso [Varmuuskopion lähteiden tunnistus](configuration.md#backup-source-detection)). Nimetyt taltiot ja `/etc`-esiasetus yltävät vain Host Data -liitoksen sisällä oleviin polkuihin, joten osoita Host Data yhteiseen ylähakemistoon, joka kattaa myös Dockerin datajuuren.
- **`PLATFORM`.** Aseta arvoksi `generic` tai `truenas`. Jos sitä ei aseteta, BombVault tunnistaa Unraidin sen omasta merkistä flash-liitoksessa ja käsittelee kaiken muun yleisenä, ja vain Unraidia koskevat vaiheet ohitetaan sen sijaan, että niitä yritettäisiin ja ne epäonnistuisivat.

**TrueNAS Scale** kulkee samaa compose-reittiä; luettelomerkintä on valmisteltu repoon, mutta sitä ei ole vielä lähetetty. Virtuaalikoneiden varmuuskopiointi vaatii siellä `LIBVIRT_URI`-muuttujan, koska TrueNASin libvirtd kuuntelee omaa sokettiaan (`/run/truenas_libvirt/libvirt-sock`), jota kolme `LIBVIRT_*`-muuttujaa eivät pysty ilmaisemaan (katso [Asetukset](configuration.md)). Kuinka pitkälle tämä on todennettu: BombVaultin oma zvol-varmuuskopio ja -palautus ajettiin SSH:n yli oikealla TrueNAS Scale -laitteella, ja jokainen palautus luettiin takaisin tavu tavulta muuttumattomana, sekä paikalleen että poistetun zvolin tilalle. Koko virtuaalikoneen palautusta siellä (verkkotunnus määritelty uudelleen ja käynnistetty) ja läpäisyä monen gigatavun kokoluokassa ei ole testattu. Testaa palautus siellä ennen kuin luotat siihen.

## Ainoa vaadittu asetus

Ainoa muuttuja, joka sinun on asetettava, on `APP_KEY`, 32-tavuinen heksadesimaalisalaisuus (64 heksamerkkiä), jota käytetään restic-arkiston salasanan johtamiseen.

Luo sellainen millä tahansa koneella:

```bash
openssl rand -hex 32
```

Liitä tulos mallin `APP_KEY`-kenttään (Unraid) tai `docker-compose.yml`-tiedoston `APP_KEY`-ympäristömuuttujaan (tavallinen Docker-isäntä).

!!! danger "Älä menetä APP_KEY:tä"
    `APP_KEY`:n menettäminen tekee salatuista varmuuskopioistasi palautuskelvottomia. Säilytä se turvallisessa paikassa erillään palvelimesta. Kun BombVault on käynnissä, aseta kirjautumissalasana ja lataa **palautuspaketti** kohdassa **Asetukset, Palautus** (katso [Etäsijainti ja palautus](offsite-recovery.md#recovery-kit)). Siinä on avain, kaikki kohteet ja resticin komennot.

Malli liittää myös Docker-soketin, flashin (`/boot`) ja **Host Data** -juuren (`/mnt`) puolestasi. Varmuuskopioinnin *lähteet* ja *kohteet* asuvat molemmat Host Datan alla. Täydellinen muuttujaviite ja etäsijainnin määritys löytyvät kohdasta [Asetukset](configuration.md).

## Ensimmäinen ajo

![Yleiskatsaus ensimmäisen varmuuskopion jälkeen: mikä vaatii sinua, kunkin osion suojaus, kohteet ja historia.](assets/screenshots/dashboard.png)

*Yleiskatsaus ensimmäisen varmuuskopion jälkeen: mikä vaatii sinua, kunkin osion suojaus, kohteet ja historia.*

1. Avaa verkkokäyttöliittymä osoitteessa `https://<your-unraid-ip>:3443` (itse allekirjoitettu varmenne valmiiksi). Uusi asennus avautuu sivulle **Tervetuloa BombVaultiin**, jossa luetellaan, mitä BombVault löysi palvelimelta.
2. Valitse **Aloitetaan**. Yleiskatsauksessa näkyy sitten **Ensimmäiset vaiheet** -kortti, jossa on kolme vaihetta:
    1. **Valitse osiot**: ota käyttöön se, mitä BombVault varmuuskopioi (Kontit, VMs, Flash, Kansiot, ZFS-tietojoukot, Itsevarmuuskopio). **Valitse** avaa osion merkintöjen luettelon, jossa **Keskeytä aikataulu** jättää yhden pois. Tässä käyttöön ottamasi osiot ajetaan joka yö, mitä muutat kohdassa **Asetukset, Aikataulu**.
    2. **Lisää kohde**: minne varmuuskopiot menevät. Aloittamiseen riittää paikallinen kansio. Turvallisin on toinen kohde tilojen ulkopuolella, kuten Storage Box tai Backblaze B2. Paikalliset repositoryt ovat oletuksena `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` ja ne luodaan ensimmäisen varmuuskopion yhteydessä.
    3. **Ensimmäinen varmuuskopio**: sen jälkeen se ajetaan itsestään joka yö.
3. Avaa **Varmuuskopiot** nähdäksesi kaikki merkinnät yhdessä luettelossa. **Varmuuskopioi kaikki nyt** käynnistää ajon kaikille käyttöön otetuille osioille, ja **Varmuuskopioi nyt** merkinnän sivulla varmuuskopioi vain sen yhden.

Oletko käyttänyt BombVaultia ennen? **Palauta varmuuskopiosta** tervetulosivulla avaa Palautus-sivun, jossa **Hätäohjattu toiminto** tuo takaisin kadonneen palvelimen asetukset, kontit, virtuaalikoneet ja kansiot. Katso [Etäsijainti ja palautus](offsite-recovery.md#emergency).

!!! tip "Valinnaista: valitse varmuuskopiojärjestys"
    Jos jotkin kontit tulisi aina varmuuskopioida ennen muita (esimerkiksi tietokanta ennen sitä käyttävää sovellusta), avaa **Järjestys**-kortti kohdassa **Asetukset, Aikataulu** ja vedä ne haluamaasi järjestykseen. Ajoitetut ajot ja täysvarmuuskopio noudattavat sitä; kaikki järjestämättä jättämäsi varmuuskopioidaan erääntyneimmät ensin.

!!! note "Isäntäintegraation tarkistus"
    Yleiskatsauksen **Isäntäintegraatio**-kortti koettaa jokaista liitosta ja komentorivityökalua (Docker-soketti, libvirt, restic, qemu-img, rclone) ja raportoi puuttuvat palaset, joten voit vahvistaa, että kontti on kytketty oikein ennen kuin luotat siihen. Se odottaa kohdassa **Mukauta**, kunnes lisäät sen.

## Tien löytäminen {#navigation}

![Asetukset ruutuina ja valitun ruudun kortit vieressä. Jokainen muutos kirjoitetaan heti.](assets/screenshots/settings.png)

*Asetukset ruutuina ja valitun ruudun kortit vieressä. Jokainen muutos kirjoitetaan heti.*

Sivupalkissa on viisi kohtaa: **Yleiskatsaus**, **Poikkeamat** (kun poikkeamien tunnistus on päällä), **Varmuuskopiot**, **Palautus** ja **Kirjaudu ulos** -kohdan alla **Asetukset**. Sivun pääasia kelluu oikeassa alakulmassa, ja **Lisää** sisältää harvinaisemmat. Puhelimessa palkissa ovat Yleiskatsaus, Varmuuskopiot, Palautus ja Asetukset, ja **Lisää** sisältää loput.

Sivut ja asetukset käyttävät ikkunan koko leveyden. Ikkuna pitää painikkeensa alareunassaan, ja eteenpäin vievä on oikealla. Puhelimessa alareuna, jossa on yli kolme painiketta, säilyttää poistumisen ja pääasian ja siirtää muut kohtaan **Lisää**.

- **Varmuuskopiot** luettelee merkintöinä jokaisen kontin, virtuaalikoneen, kansiojoukon, flash-aseman, jokaisen ZFS-tietojoukon ja Itsevarmuuskopion. Luettelon yläpuolella olevat ruudut valitsevat lajin, ja **Suodattimet**, **Lajittelu** ja **Hae** rajaavat sitä. Jokaisella merkinnällä on oma sivunsa, jossa on kolme välilehteä: **Varmuuskopiot**, **Asetukset** ja **Tarkistus**. Merkinnän kuvakkeen vasemmalla puolella oleva takaisin-painike johtaa luetteloon.
- **Palautus** on yksi sivu kaikelle: valitse osio, hae merkintä ja valitse varmuuskopio. Myös Hätäohjattu toiminto ja muut lähteet alkavat täältä.
- **Asetukset** on neljätoista ruutua: Yleiset, Ulkoasu, Aikataulu, Kohteet, Palautus, Ilmoitukset, Tarkistus, Turvallisuus, Ilmentymät, Pariliitos, Integraatiot, Sovellukset, Järjestelmä ja Tieto. Paina `/` tai Ctrl+K hakeaksesi niistä kaikista, myös sivujen aiemmilla nimillä. Vedä ruudut omaan järjestykseesi kohdassa **Ulkoasu**. Jos sivun vieressä oleva ruutujen sarake puristaisi sivua, kuten noin 1200 pikselin levyisessä ikkunassa, ruudut muuttuvat sivun yläpuolella olevaksi kaistaksi, jota voi vierittää sivusuunnassa. Puhelin näyttää saman kaistan.
- **Kohteet** ja **Ilmentymät** ovat asetusruutuja, joilla on omat sivunsa. **Näytä myös sivupalkissa** niiden ruudussa kiinnittää ne sivupalkkiin.

Missään ei ole Tallenna-painiketta, eikä mikään avaudu auki: harvinaisille asioille on oma kortti tai ikkuna, ja selitykset ovat (i)-kuplissa. Päivityksen jälkeen julkaisutiedot aukeavat itsestään, ja vihreät pisteet merkitsevät sitä, mikä on muuttunut. Napsautus piilottaa pisteen. **Uutta** Versio-kortissa kohdassa **Asetukset, Tieto** laskee, mitä et ole vielä nähnyt, ja avaa julkaisutiedot sekä luettelon kaikista muutoksista, joissa **Siirry sinne** vie perille, yhden ikkunan kahtena välilehtenä.

## Kääntäminen lähdekoodista {#build-from-source}

BombVault on yksi staattinen Go-binääri, joka tarjoaa JSON-rajapinnan ja upotetun React-käyttöliittymän. Käännä ensin käyttöliittymä ja aja sitten binääri:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

Käyttöliittymän käännös tarvitaan myös `go run` -komentoa varten. Repo seuraa polussa `web/dist` vain tyhjää merkkitiedostoa, joten ilman komentoa `npm --prefix web run build` binääri ei upota mitään ja vastaa `500 SPA index not found`, mikä on odotettua. Dockeria, libvirtiä ja Unraidia ei voi testata CI:ssä, joten tarkista liitokset, restic ja virtuaalikoneiden SSH-yhteys oikealla isännällä yleiskatsauksen **Isäntäintegraatio**-kortilla (tai `GET /api/spike`) ennen kuin avaat pull requestin.

## Seuraavat vaiheet

- Selaa täyttä **[Ominaisuudet](features.md)**-listaa.
- Tuo ryhmäsi kaikki palvelimet puhelimeesi **[Android-sovelluksella](android.md)**.
- Lisää yksi tai useampi kohde tilojen ulkopuolelle (kukin osio voi kopioida usealle kerralla) ja tallenna palautuspakettisi, kuten kuvataan kohdassa **[Etäsijainti ja palautus](offsite-recovery.md)**.
- Kloonaatko kokoonpanoa tai siirrytkö uuteen laatikkoon? Kanna koko kokoonpanosi mukanasi **Vie / tuo asetukset** -kortilla. Katso [Asetukset](configuration.md#portable-settings-export-and-import).
- Törmäsitkö esteeseen? Katso **[Vianmääritys](troubleshooting.md)**.
