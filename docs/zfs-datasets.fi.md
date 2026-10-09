# ZFS-tietojoukot

**ZFS**-sivu varmuuskopioi ZFS-tietojoukkoja. Kohde on yksi tietojoukko yhdessä kaikkien sen alla olevien tietojoukkojen kanssa. Jokaista varmuuskopiota varten BombVault ottaa koko puusta yhden ZFS-tilannevedoksen, joten jokainen sen tietojoukko tallentuu samalta hetkeltä. Sen jälkeen se lukee kunkin tietojoukon tiedostot tästä tilannevedoksesta, tallentaa ne resticillä samalla tavalla kuin kansion ja poistaa tilannevedoksen heti perään. Varmuuskopiot on deduplikoitu, jokaista voi selata, ja yksittäisiä tiedostoja voi palauttaa.

Varmuuskopio ei koskaan syötä datavirtaa resticiin eikä koskaan palauta tietojoukkoa aiempaan tilaan. BombVault tuhoaa vain tilannevedoksia, jotka se on itse luonut. Valinnainen [replika](#replica) on ainoa paikka, jossa käytetään komentoa `zfs send`: se kopioi tietojoukot toiselle ZFS-palvelimelle eikä koske varmuuskopioon.

## Vaatimukset {#requirements}

- **SSH-yhteys tähän palvelimeen.** ZFS-tietojoukot käyttävät samaa avainta, isäntää ja käyttäjää kuin virtuaalikoneiden varmuuskopiot. Jos ne toimivat jo, tämäkin toimii. Muuten seuraa GitHubissa olevaa [ohjetta virtuaalikoneen varmuuskopiosta SSH:n kautta](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). Mallin kentät ovat **Host SSH: Address**, **Host SSH: Port** ja **Host SSH: User**.
- **`zfs`-komento kyseisessä isännässä.** Unraid 6.12 ja uudemmat sekä TrueNAS SCALE sisältävät sen.
- **Host Data liitettynä polkuun `/mnt` Access Mode -asetuksella Read/Write - Slave.** Se on mallin oletus. Tietojoukon tilannevedos näkyy tietojoukon kansiossa `.zfs/snapshot` vasta BombVaultin käynnistyttyä, joten kontin täytyy saada isännän myöhemmin tekemät liitokset.
- **Tietojoukot liitettyinä polun `/mnt` alle.** Unraidissa poolit ovat polussa `/mnt/<pool>`, joten näin on jo.

Ota toimialue käyttöön kohdassa **Asetukset, Yleiset** (ZFS-tietojoukot). ZFS-sivulla näkyy sitten kortti **Yhteys tähän palvelimeen**. Se testaa SSH-yhteyden, kertoo käyttäjän ja isännän, joihin se yhdistää, ja sanoo, mitä puuttuu, kun jotain puuttuu. Isäntäintegraation tarkistus (`/spike`) näyttää saman tuloksen.

## Kohteet ja alatietojoukot {#items-and-children}

Avaa ZFS-sivulla **Lisää tietojoukkoja**. Luettelo tulee palvelimelta. Valitse ylin tietojoukko siitä, mitä haluat varmuuskopioida, esimerkiksi `cache/appdata`, niin kohde kattaa sen ja jokaisen sen alla olevan tietojoukon.

- **Uudet alatietojoukot tulevat mukaan itsestään.** Myöhemmin kohteen alle luotu tietojoukko varmuuskopioidaan seuraavalla ajolla, ja se ajo mainitsee sen uutena. Sen ensimmäinen varmuuskopio lukee sen kerran kokonaan; sen jälkeen luetaan vain muutokset.
- **Voit jättää yksittäisiä alatietojoukkoja pois.** Kytke yksi pois kohteen asetuksista, niin se jää pois kaiken alleen kuuluvan kanssa. Pois jätetty alatietojoukko, jota ei enää ole palvelimella, merkitään sellaiseksi, ja sen voi poistaa luettelosta.
- **Alatietojoukot, joita ei voi lukea, ohitetaan, mutta ei koskaan hiljaa.** Ajo luettelee ne, kohde näyttää, montako ohitettiin, ja kojelaudan kattavuuskortti laskee jokaisen suojaamattomaksi. Ajo varmuuskopioi silti kaiken muun eikä epäonnistu ohitetun alatietojoukon takia. Syyt ovat [syykooditaulukossa](#reason-codes): tietojoukko, jota ei ole liitetty, jolla on `canmount=off`, `legacy`-liitoskohta tai ei liitoskohtaa lainkaan, salausavain, jota ei ole ladattu, tilannevedosten käyttö poistettuna tai liitoskohta, jota BombVault ei näe.
- **Ohitettu tietojoukko ei vie alatietojoukkojaan mukanaan.** Tietojoukko, jolla on `canmount=off` ja joka sisältää vain muita tietojoukkoja, ohitetaan (näytetään "vain rakenne"), ja sen liitetyt alatietojoukot varmuuskopioidaan. Salattu tietojoukko, jonka avainta ei ole ladattu, ohitetaan yhdessä niiden alatietojoukkojen kanssa, jotka jakavat sen avaimen.
- **Alatietojoukot, jotka ovat virtuaalikoneen levyjä tai järjestelmätietoja, ovat aluksi pois päältä** lisäysikkunassa, ja syy näkyy kytkimen vieressä. Kokonaisen poolin lisääminen pyytää vahvistuksen, joka luettelee, mitä poolissa on.

### Taltiot {#volumes}

Taltio (zvol) sisältää virtuaalilevyn tiedostojen sijaan, eikä ZFS-sivu koskaan varmuuskopioi sellaista.

- Taltio, jota virtuaalikone käyttää, varmuuskopioidaan kyseisen virtuaalikoneen kanssa sivulla **VMs**.
- Taltiota, jota mikään virtuaalikone ei käytä (iSCSI-extent, irrotettu levy), **BombVault ei varmuuskopioi**. Lisäysikkuna ja ZFS-sivu laskevat nämä taltiot ja kertovat sen. Myöhempi versio varmuuskopioi ne.

Kohteen puussa olevat taltiot ohitetaan ja mainitaan jokaisella ajolla.

### Dockerin tallennustila {#docker-storage}

Dockerin ZFS-tallennusajurilla jokainen kuvakerros on tietojoukko, jolla on `legacy`-liitoskohta. Lisäysikkuna kokoaa ne yhdelle riville yläjoukkoa kohden. Puu, jossa on yli 20 tällaista tietojoukkoa, ei voi olla kohde: niin kauan kuin siitä on tilannevedos, Docker ei voi poistaa kuvakerroksia. Lisää sen sijaan sen alla olevat tietojoukot, esimerkiksi `appdata`.

### Kohteet eivät koskaan mene päällekkäin {#overlap}

Tietojoukko voi kuulua vain yhteen kohteeseen. BombVault hylkää uuden kohteen, joka on olemassa olevan sisällä tai joka sisältäisi sellaisen. Jos haluat yhdistää useita alakohteita yhdeksi yläkohteeksi, poista ensin alakohteet ja valitse niiden varmuuskopioiden säilyttäminen, ja lisää sitten yläkohde. Jokainen tietojoukko säilyttää historiansa omalla nimellään, joten seuraava varmuuskopio jatkaa siitä, mihin vanhat kohteet jäivät, eikä lue kaikkea uudelleen.

## Konttien pysäyttäminen ja komennot tilannevedoksen ympärillä {#consistency}

Tilannevedos käynnissä olevasta tietokannasta on kuin äkillinen sähkökatko: tietokanta yleensä toipuu, mutta sen on toivuttava. Jokainen kohde voi tehdä asialle kaksi asiaa, ja molemmat koskevat vain tilannevedoksen hetkeä, eivät koko varmuuskopiota.

- **Pysäytä nämä kontit tilannevedoksen ajaksi.** BombVault pysäyttää luetellut kontit, ottaa tilannevedoksen ja käynnistää ne heti uudelleen. Saman riippuvuustason kontit pysähtyvät rinnakkain, riippuvaiset ensin, joten koko ikkuna kestää yleensä muutaman sekunnin; ajo näyttää, kuinka kauan. Varmuuskopio lukee sen jälkeen jäädytettyä tilannevedosta, kun sovellukset jo toimivat taas. Vain käynnissä olleet kontit pysäytetään.
- **Komento ennen tilannevedosta ja sen jälkeen.** Se ajetaan valitsemassasi kontissa, esimerkiksi tietokannan vedostamiseksi tietojoukkoon juuri ennen tilannevedosta pysäyttämättä mitään. Jos komento ennen tilannevedosta epäonnistuu, varmuuskopio epäonnistuu eikä tilannevedosta oteta. Tilannevedoksen jälkeinen epäonnistunut komento näytetään ajossa, mutta se ei kaada varmuuskopiota.

Mitä tapahtuu, kun jokin menee pieleen:

- Jos konttia ei voi pysäyttää, BombVault käynnistää jo pysäyttämänsä kontit, ja varmuuskopio epäonnistuu kontin nimen kanssa. Se ei koskaan tyydy tilannevedokseen käynnissä olevista sovelluksista.
- Pysäytys odottaa, kunnes käynnissä oleva konttivarmuuskopio on valmis (enintään 30 minuuttia manuaalisessa ajossa, varmuuskopion aikarajaan asti ajastetussa), jotta ne kaksi eivät koskaan pysäytä ja käynnistä samaa konttia yhtä aikaa.
- Ennen ensimmäisen kontin pysäyttämistä BombVault kirjaa, mitkä se pysäyttää. Jos BombVault tapetaan ikkunan aikana, se käynnistää nuo kontit seuraavan käynnistyksensä yhteydessä, lähettää ilmoituksen, ja kohde näyttää punaisen huomautuksen jokaisesta kontista, jota se ei saanut käyntiin.

Automaattiset tietokantavedokset (katso [Ominaisuudet](features.md)) ajetaan kontin oman varmuuskopion yhteydessä sivulla **Kontit**, eivät ZFS-kohteen kanssa. Tietokanta, jonka kontti varmuuskopioidaan vain sen tietojoukon kautta, ei saa vedosta, joten anna sille tässä komento.

Kontti voi olla tässä luettelossa ja sivulla **Kontit** yhtä aikaa. Sen tiedot tallennetaan silloin kahdesti, kahteen repoon, ja **Täysvarmuuskopio** pysäyttää sen kahdesti. Kohde kertoo siitä.

## Palauttaminen {#restore}

Avaa kohteessa **Varmuuskopiot**, valitse varmuuskopio ja sitten tietojoukko. Oletuksena se on kohteen ylin tietojoukko.

- **Palauta tietojoukkoon.** Varmuuskopion tiedostot kirjoitetaan tietojoukon liitoskohtaan. Samannimiset tiedostot korvataan, muut jäävät. Tietojoukkoa ei koskaan palauteta aiempaan tilaan eikä korvata. BombVault tarkistaa, että tietojoukko on liitetty, näkyvissä ja kirjoitettavissa, kerran ennen aloitusta ja uudelleen juuri ennen kirjoittamista. Kohtiin, joihin on liitetty alatietojoukko, ei kirjoiteta mitään: alatietojoukko säilyttää tiedostonsa, omistajansa ja oikeutensa, ja se palautetaan omasta varmuuskopiostaan.
- **Palauta kansioon.** Valitse kansio polun `/mnt` alta. BombVault tarkistaa, että kansio on liitetyssä poolissa tai jaossa ja että vapaata tilaa on riittävästi. Tämä toimii ilman SSH-yhteyttä ja myös tietojoukoille, joita ei enää ole.
- **Uuteen tietojoukkoon.** Anna tietojoukko, jota ei vielä ole. BombVault luo sen varmuuskopioon tallennetuilla ZFS-ominaisuuksilla ja palauttaa sen sisään, katso [Palauttaminen uutena tietojoukkona](#new-dataset).
- **Valitse tiedostot** (edistynyt): kirjoita tietojoukkoon takaisin vain valitsemasi tiedostot ja kansiot.
- **Tämän varmuuskopion kaikki tietojoukot** (edistynyt): puun jokainen tietojoukko omaan alikansioonsa valitsemassasi kansiossa. Varmuuskopiossa ohitetut tietojoukot mainitaan.
- **Toiselta palvelimelta:** **Palautus**-sivu palauttaa toisen BombVaultin reposta, aina kansioon: yhden varmuuskopion kaikki tietojoukot kukin omaan alikansioonsa tai yhden puun tietojoukon kokonaan tai valitut tiedostot.

Kohteen pysäytettävien konttien luetteloa tarjotaan myös palautettaessa tietojoukkoon. Nuo kontit pysyvät pysäytettyinä koko palautuksen ajan, ja konttivarmuuskopiot odottavat sillä välin.

### Turvatilannevedos {#safety-snapshot}

Ennen kuin BombVault kirjoittaa tietojoukkoon, se ottaa ZFS-tilannevedoksen juuri siitä tietojoukosta nimellä `bombvault-prerestore-<aika>`. Se on oletuksena päällä; sen poistaminen käytöstä vaatii toisen vahvistuksen. Jos tilannevedosta ei voi ottaa, mitään ei palauteta.

BombVault ei koskaan poista turvatilannevedosta itse. Kohde luettelee ne iän ja koon kanssa, kukin toiminnolla **Poista**, ja varoittaa, kun vanhin on yli 30 päivää vanha, koska se pitää poistettuja ja muutettuja tietoja poolissa.

Palataksesi takaisin palautuksen jälkeen kopioi yksittäisiä tiedostoja tietojoukon sisältä polusta `.zfs/snapshot/bombvault-prerestore-<aika>`. `zfs rollback <dataset>@bombvault-prerestore-<aika>` toimii vain niin kauan kuin se on tietojoukon uusin tilannevedos. `zfs rollback -r` poistaa kaikki uudemmat tilannevedokset, myös automaattiset.

### Palauttaminen uutena tietojoukkona {#new-dataset}

BombVault tallentaa jokaisen varmuuskopion mukana kunkin tietojoukon paikallisesti asetetut ZFS-ominaisuudet: compression, recordsize, quota, reservation, atime, xattr, acltype, casesensitivity ja omat käyttäjäominaisuutesi. Periytyvät ja vain luku -arvot jätetään pois, koska ne palaavat itsestään. Varmuuskopioissa ajalta ennen kuin BombVault tallensi niitä ei ole yhtään.

- **Uuteen tietojoukkoon** ajaa `zfs create` -komennon jokaisella tallennetulla ominaisuudella. casesensitivity, normalization ja utf8only voi asettaa vain näin. Kiintiöt ja varaukset asetetaan vasta tiedostojen jälkeen, jotta ne eivät voi estää niitä. Liitoskohta jätetään pois, jotta kopio ei törmää alkuperäiseen, samoin `canmount`, `readonly` ja salaus, jotta palautus voi kirjoittaa. Salatun alle luotu uusi tietojoukko saa sen salauksen. Yläpuolisen tietojoukon on oltava olemassa. Jos jokin epäonnistuu luonnin jälkeen, uusi tietojoukko jää palvelimelle, koska BombVault ei koskaan tuhoa tietojoukkoa.
- **Palauta tietojoukkoon** näyttää tallennetut ominaisuudet palautuksen vieressä. **Aseta myös nämä ominaisuudet** asettaa ne, jotka olemassa oleva tietojoukko vielä hyväksyy, ennen kuin yhtään tiedostoa kirjoitetaan. Kiintiöt ja varaukset asetetaan vasta tiedostojen jälkeen, jotta ne eivät voi estää niitä. Ilman tätä kytkintä tietojoukko säilyttää asetuksensa.

## Mitä varmuuskopio sisältää {#contents}

Varmuuskopiossa: jokaisen varmuuskopioidun tietojoukon tiedostot ja kansiot omistajineen, oikeuksineen, aikaleimoineen ja laajennettuine määritteineen siten kuin restic ne tallentaa sekä kunkin tietojoukon paikallisesti asetetut ZFS-ominaisuudet.

Ei varmuuskopiossa:

- kunkin tietojoukon ylimmän kansion oma omistaja ja oikeudet (kaikki sen alla on mukana). Palautus tietojoukkoon jättää olemassa olevan ylimmän kansion ennalleen, palautus kansioon luo sen oikeuksilla `0755`;
- olemassa olevat ZFS-tilannevedokset;
- ohitetut tai pois jätetyt alatietojoukot;
- taltiot.

Palauttaaksesi uuteen pooliin luo pool ja palauta jokainen tietojoukko uuteen tietojoukkoon. Sitä, palautuvatko NFSv4-ACL:t, joita TrueNAS käyttää SMB-tietojoukoissa, niin kuin odotat, ei ole vielä tarkistettu, joten kokeile palautusta omilla tiedoillasi ennen kuin luotat niihin.

## Salatut tietojoukot {#encryption}

Salattu tietojoukko varmuuskopioidaan vain, kun sen avain on ladattu. Muuten se ohitetaan varoituksen kera; lataa avain komennolla `zfs load-key` ja liitä tietojoukko. BombVault lukee tiedot purettuina ja tallentaa ne resticin repoon, joka on salattu. Jos olet poistanut salauksen käytöstä BombVaultissa, tuo repo ei ole salattu.

## Replika {#replica}

Replika on kohteen tietojoukkojen kopio toisella ZFS-palvelimella. BombVault pitää sen ajan tasalla komennoilla `zfs send` ja `zfs receive`. Ensimmäinen ajo lähettää kaiken, sen jälkeen siirtyvät vain muuttuneet lohkot. Toisella palvelimella kopion voi liittää heti.

Replika ei koskaan korvaa varmuuskopiota. Vanhemmat versiot, yksittäiset tiedostot ja tarkistus tulevat edelleen varmuuskopioista, ja replika säilyttää vain niin monta tilannevedosta kuin asetat. Ajan tasalla oleva replika lasketaan kopioksi toimipaikan ulkopuolella, mutta kohde, jolla on replika mutta ei varmuuskopiota, pysyy oranssina.

Ota se käyttöön kohteen asetusten kortissa **Replica**. Siellä valitset, minne replika menee, milloin se ajetaan (**After every backup** tai **Own plan**) ja kuinka monta tilannevedosta jää kohdepalvelimelle. Kortti luettelee jokaisen tietojoukon ja taltion tiloineen, ja **Replicate now** käynnistää ajon. Replika-ajolla on oma lukkonsa, joten pitkä ensimmäinen siirto ei koskaan hidasta varmuuskopioita.

### Lähetys ZFS-palvelimelle {#replica-push}

Mikä tahansa kone, jossa on ZFS ja SSH, voi vastaanottaa, esimerkiksi toinen Unraid tai TrueNAS. BombVaultin ei tarvitse toimia siellä.

1. Avaa kohdassa **Asetukset, Storage locations** **Add storage location** ja valitse **ZFS server**.
2. Anna osoite, käyttäjä ja portti. Ikkuna näyttää BombVaultin julkisen avaimen. Lisää se käyttäjän tiedostoon `~/.ssh/authorized_keys` palvelimella. Unraidissa se on kohdassa **Settings, Users, root, SSH keys**.
3. Testaa yhteys. Ikkuna luettelee sitten palvelimen poolit. Valitse yksi ja aseta juuri, jonka oletus on `<pool>/bombvault-replica`.
4. Valitse uusi palvelin kohteen kortissa **Replica**.

Rootilla muuta ei tarvita. Omalla käyttäjällä tarvitaan nämä oikeudet kohdepalvelimen poolissa, jotka ikkuna myös näyttää:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

Lähteellä samanlainen käyttäjä tarvitsee nämä kohteen ylimmällä tietojoukolla:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

Tässä suunnassa kohteen omistava BombVault pitää myös avainta, jolla voi kirjoittaa palvelimelle.

### Toisen BombVaultin nouto {#replica-pull}

Toisinpäin pariliitetty BombVault hakee replikan itse. Lähteellä ei silloin ole avainta, jolla voisi kirjoittaa kopioon tai poistaa siitä, joten kopio säilyy, vaikka joku olisi vallannut lähteen.

1. Avaa instanssissa, jonka pitää säilyttää kopio, **Instances**, sitten **Fetch**, paina **Add source** ja valitse **ZFS datasets**.
2. Valitse pariliitetty instanssi ja sen ZFS-kohteet. Valitse sitten tämän palvelimen pooli ja juuri, aikataulu ja kuinka monta tilannevedosta jää.
3. Tallennus kysyy lähteeltä. Siellä kohde näyttää pyynnön, eikä mitään tapahdu ennen kuin joku painaa **Allow**.

Sen jälkeen lähde lisää avaimen toiselle instanssille. Avaimella voi vain lähettää tämän kohteen tilannevedoksia ja luoda omia replika-tilannevedoksia. Sillä ei voi poistaa mitään eikä nähdä mitään muuta. **Revoke access** kohteen kortissa **Replica** poistaa avaimen milloin tahansa. Se, mitä toinen instanssi jo pitää hallussaan, jää sinne.

Hakeva instanssi ajaa oman aikataulunsa mukaan ja siivoaa itse. Aikataulu ja säilytys asetetaan sen puolella.

### Mihin data päätyy {#replica-target}

Jokainen tietojoukko päätyy polkuun `<root>/<server>/<pool>/<path>`. Palvelinkansio on lähdeinstanssin nimi, joka lukitaan ensimmäisessä siirrossa, joten kaksi palvelinta, joilla on sama poolin nimi, eivät koskaan joudu toistensa tielle. Esimerkiksi palvelimen `tower` tietojoukko `cache/appdata` päätyy polkuun `backup/bombvault-replica/tower/cache/appdata`.

Kopio kohdepalvelimella on vain luettavissa eikä liitettynä, joten se ei koskaan peitä mitään kyseisellä palvelimella. ZFS-ominaisuudet siirtyvät mukana, paitsi liitospiste sekä `sharenfs` ja `sharesmb`.

### Mitä mukaan tulee {#replica-contents}

Mukaan tulee kaikki, minkä kohde varmuuskopioi, sekä sen alla olevat taltiot, jotka varmuuskopio ohittaa. Alatietojoukko, jonka olet kytkenyt kohteessa pois, jää pois. Kaikki ajon tietojoukot tulevat yhdestä tilannevedoksesta, kuten varmuuskopiossa.

### Kuinka kauan tilannevedokset säilyvät {#replica-retention}

Kohdepalvelimella uusi replika säilyttää 7 päivittäistä ja 3 viikoittaista tilannevedosta. Valitse sen sijaan **Short**, **Balanced** tai **Long**, tai aseta **Custom values**. Siellä poistetaan vain tilannevedoksia, joiden nimi on `bombvault-replica-<14 numeroa>`, eikä koskaan uusinta, jonka molemmat puolet jakavat.

Lähteellä BombVault säilyttää vain viimeisimmän replika-tilannevedoksen sekä kirjanmerkin jokaiselle lähettämälleen tilalle. Kirjanmerkit eivät vie tilaa. Seuraava siirto alkaa niistä.

### Salatut tietojoukot replikassa {#replica-encryption}

Salattu tietojoukko lähetetään raakamuodossa. Se pysyy salattuna kohdepalvelimella, eikä kohdepalvelin koskaan näe avainta. Säilytä avain turvallisesti: tarvitset sitä kopion avaamiseen palautuksen jälkeen, ja replika ilman avainta on lukukelvoton.

### Replikan käyttö {#replica-use}

Avaa kohteen välilehti **Varmuuskopiot** ja napsauta replikariviä kortissa **Storage locations**. Paneeli luettelee kohdepalvelimen tilannevedokset ja näyttää komennot oikeine nimineen.

Vanhaa tilaa voi katsoa kloonaamalla tilannevedoksen kohdepalvelimella. Klooni ei vie tilaa ennen kuin jokin muuttuu, ja replika pysyy koskemattomana:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Jos lähde pettää, muuta kopio tavalliseksi, kirjoitettavaksi tietojoukoksi kohdepalvelimella:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

BombVault lopettaa sen jälkeen replikoinnin kyseiseen tietojoukkoon, kunnes aloitat uuden ensimmäisen ajon.

Jos haluat palauttaa tilan lähteelle, paina paneelissa **Bring back as a new dataset**. BombVault lähettää tilannevedoksen uuteen tietojoukkoon alkuperäisen viereen, ja sen nimi on `<dataset>-bombvault-restore-` ja aikaleima. Se ei koskaan kirjoita alkuperäisen päälle.

## Jäljelle jääneet tilannevedokset {#leftover-snapshots}

Varmuuskopion tilannevedoksen nimi on `<dataset>@bombvault-<14 numeroa>`, esimerkiksi `cache/appdata@bombvault-20260924021500` (UTC). BombVault poistaa sen heti varmuuskopion jälkeen. Jos se epäonnistuu, esimerkiksi koska tietojoukko on varattu tai BombVault pysäytettiin, BombVault poistaa sen:

- ennen kohteen seuraavaa varmuuskopiota,
- kun BombVault käynnistyy, jokaiselle kohteelle, myös toimialueen ollessa pois päältä,
- kun poistat kohteen,
- kun painat kohteessa **Poista nyt**, joka näyttää myös, montako on jäljellä.

Vain nimet, jotka ovat täsmälleen `bombvault-` ja 14 numeroa, poistetaan. Turvatilannevedoksiin, omiin tilannevedoksiisi ja automaattisiin tilannevedoksiin ei koskaan kosketa. Jos haluat poistaa sellaisen käsin:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

Replikan tilannevedoksen nimi on `<dataset>@bombvault-replica-<14 numeroa>`, eikä se ole jäännös. Se pysyy lähteellä, kunnes seuraava replika-ajo korvaa sen, ja kohdepalvelimella niin kauan kuin säilytys pitää sen. Siivooja ei koskaan koske siihen, koska se vastaa vain nimiä, joissa `bombvault-` seuraa täsmälleen 14 numeroa.

## Poikkeamat {#anomalies}

Tyhjennetty alatietojoukko tuskin muuttaa suuren puun kokonaismäärää, joten poikkeamien tunnistus seuraa kohteen jokaista tietojoukkoa erikseen: koolla, tiedostomäärällä, uusilla tiedoilla ja restic-ajalla on kullakin oma historiansa. Historia kuuluu tietojoukon nimelle, joten se säilyy, kun puu myöhemmin varmuuskopioidaan toisen kohteen kautta.

Tietojoukko, jonka edellinen ajo varmuuskopioi ja jota tämä ajo ei voinut lukea, lasketaan tyhjennetyksi, kunhan kohteen valinta ei ole muuttunut. Tämä kattaa lataamattoman avaimen, liittämättömän tietojoukon ja puusta kadonneen tietojoukon. Itse pois sulkemasi alatietojoukko muuttaa valintaa, joten sen historia alkaa silloin alusta. Niin kauan kuin kadonneita tietoja koskeva löydös on auki, säilytys pitää juuri tuon tietojoukon vanhat varmuuskopiot ja karsii muun puun tavalliseen tapaan.

**Poikkeamat**-sivulla jokaisella tietojoukolla on oma rivinsä kohteen paneelissa, joka avautuu kohteen kortin kohdasta **Valvonta** tai, jos kohteella ei ole mitään avoinna, sen riviltä kortilla **Ei mitään avoinna**. Kohteen puu tällä sivulla näyttää avoimet löydökset kunkin tietojoukon vieressä. Löydöksen linkki avaa kohteen palautuspaneelin tietojoukon viimeisimmän hyvän varmuuskopion kohdalta. Se, valmistuuko ajo, arvioidaan koko kohteelle, koska ajo onnistuu tai epäonnistuu kokonaisuutena.

Itse tarkistukset kuvataan kohdassa [Ominaisuudet](features.md). [MCP-palvelimen](mcp.md) kautta yhdistetty avustaja voi luetella ZFS-kohteen palautuspisteet, käynnistää sen varmuuskopion ja lukea löydökset, mutta löydös kuitataan **Poikkeamat**-sivulla.

## Syykoodit {#reason-codes}

Sivu, ajohistoria ja ilmoitukset nimeävät ongelman jollakin näistä koodeista. Useimmilla korjaus näkyy myös sivulla vieressä.

| Koodi | Merkitys | Mitä tehdä |
|---|---|---|
| `ssh-missing` | SSH-yhteyttä ei ole määritetty tässä kontissa. | Määritä SSH-yhteys kuten virtuaalikoneiden varmuuskopioille. |
| `host-placeholder` | Host SSH: Address on yhä esimerkkiarvo, eikä `host.docker.internal` vastannut. | Aseta Host SSH: Address tämän palvelimen LAN-IP:ksi. |
| `host-fallback` | Host SSH: Address on yhä esimerkkiarvo, ja `host.docker.internal` toimii. | Ei mitään, tai aseta LAN-IP. |
| `ssh-unreachable` | Palvelinta ei tavoiteta SSH:lla. | Tarkista osoite ja portti ja että SSH on päällä. |
| `ssh-auth` | Palvelin hylkäsi BombVaultin avaimen. | Aja yhteyskortilla näkyvä komento kerran palvelimella. |
| `zfs-not-found` | SSH-isännällä ei ole `zfs`-komentoa. | Osoita Host SSH: Address koneeseen, joka omistaa poolit. |
| `zfs-permission` | SSH-käyttäjä ei saa ajaa tätä zfs-komentoa. | Käytä root-käyttäjää, tai katso [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` nimeää eri isännän tai käyttäjän kuin SSH-kentät. | Täsmäytä ne, tai tyhjennä SSH-kentät, jolloin molemmat tulevat URI:sta. |
| `zfs-error` | zfs ilmoitti muusta virheestä. | Tiedot näyttävät sen viestin. |
| `propagation-missing` | Isännän uudet liitokset eivät tavoita konttia. | Aseta Host Datan Access Mode arvoon Read/Write - Slave ja käynnistä BombVault uudelleen. |
| `invalid-name` | Tietojoukon nimi, jota BombVault ei hyväksy. | Nimeä tietojoukko uudelleen. |
| `name-too-long` | Puun tietojoukko on liian pitkä tilannevedoksen nimeksi. | Nimeä se uudelleen, tai lisää sen alla oleva tietojoukko kohteeksi. |
| `invalid-exclude` | Poissulkemismalli tai pois jätetty alatietojoukko ei sovi kohteeseen. | Korjaa viestin mainitsema merkintä. Jos haluat jättää pois kokonaisen alatietojoukon, kytke se pois mallin kirjoittamisen sijaan. |
| `not-found` | Tietojoukkoa ei ole palvelimella. | Poista kohde tai luo tietojoukko uudelleen. Sen varmuuskopiot voi yhä palauttaa. |
| `not-filesystem` | Tämä on taltio, ei tiedostojärjestelmä. | Katso [Taltiot](#volumes). |
| `overlaps-item` | Tietojoukko on päällekkäin olemassa olevan kohteen kanssa. | Katso [Kohteet eivät koskaan mene päällekkäin](#overlap). |
| `docker-storage` | Puu sisältää Dockerin kuvatallennuksen. | Katso [Dockerin tallennustila](#docker-storage). |
| `nothing-readable` | Kohteen yhtäkään tietojoukkoa ei voi juuri nyt lukea. | Katso ohitettujen tietojoukkojen koodit. |
| `snapshot-failed` | Tilannevedosta ei voitu luoda. | Tiedot näyttävät zfs:n viestin. |
| `containers-busy` | Konttivarmuuskopio oli yhä käynnissä, kun konttien piti pysähtyä. | Aloita myöhemmin uudelleen. Ajastetut ajot odottavat itsestään. |
| `consistency-stop-failed` | Konttia ei voitu pysäyttää, joten tilannevedosta ei otettu. | Tarkista kontti, tai poista se luettelosta. |
| `pre-snapshot-failed` | Tilannevedosta edeltävä komento epäonnistui. | Ajon tiedot näyttävät sen tulosteen. |
| `container-unknown` | Luetteloitua konttia ei ole olemassa. | Poista se luettelosta. |
| `container-is-self` | BombVault ei voi pysäyttää omaa konttiaan. | Poista se luettelosta. |
| `leftover-snapshots` | Palvelimella on yhä tilannevedoksia, joita BombVault ei voinut poistaa. | Paina **Poista nyt**, katso [Jäljelle jääneet tilannevedokset](#leftover-snapshots). |
| `zvol` | Taltio puussa, ohitettu. | Katso [Taltiot](#volumes). |
| `canmount-off` | Ei koskaan liitetty (`canmount=off`), ohitettu. | Jos siinä on tietoja, liitä se tai siirrä tiedot alatietojoukkoon. |
| `legacy-mount` | Legacy-liitoskohta, ohitettu. | Anna sille liitoskohta polun `/mnt` alta. |
| `no-mountpoint` | Ei liitoskohtaa, ohitettu. | Anna sille liitoskohta polun `/mnt` alta. |
| `not-mounted` | Ei liitetty palvelimella, ohitettu. | Liitä se komennolla `zfs mount`, tai aseta `canmount=on`. |
| `key-not-loaded` | Salattu, eikä avainta ole ladattu, ohitettu. | `zfs load-key`, liitä se sitten. |
| `snapdir-disabled` | Tilannevedosten käyttö on pois päältä, ohitettu. | `zfs set snapdir=hidden <dataset>`. Kansio `.zfs` pysyy piilossa. |
| `not-visible` | BombVault ei näe tietojoukon liitoskohtaa. | Siirrä liitoskohta Host Data -polun alle, tai liitä se konttiin samaan polkuun asetuksella Read/Write - Slave. |
| `shfs-only` | Tietojoukko näkyy vain polun `/mnt/user` kautta, joka piilottaa tilannevedokset. | Liitä Host Dataksi `/mnt`, ei `/mnt/user`. |
| `snapshot-not-visible` | Tilannevedos luotiin, mutta se ei ilmestynyt BombVaultiin. | Aja **Kokeile pääsyä tilannevedoksiin**; katso alta. |
| `snapshot-loop` | Tilannevedos ei tavoittanut BombVaultia, koska Host Data ei välitä uusia liitoksia. | Aseta Host Datan Access Mode arvoon Read/Write - Slave ja käynnistä BombVault uudelleen. |
| `backup-failed` | restic epäonnistui tämän tietojoukon kohdalla. | Ajon tiedot kertovat syyn. |
| `not-reached` | Ajo päättyi ennen tätä tietojoukkoa. | Aja varmuuskopio uudelleen. |
| `gone` | Tietojoukkoa ei enää ole palvelimella. | Ei mitään. Sen varmuuskopiot voi yhä palauttaa. |
| `read-only-mount` | BombVault voi vain lukea tietojoukkoa, joten se ei voi palauttaa siihen. | Aseta liitos arvoon Read/Write - Slave, tai palauta kansioon. |
| `destination-not-mounted` | Kansio ei ole liitetyssä poolissa tai jaossa. | Valitse kansio poolista tai jaosta. |
| `not-enough-space` | Kohteessa ei ole tarpeeksi vapaata tilaa. | Vapauta tilaa tai valitse toinen kansio. |
| `safety-snapshot-failed` | Turvatilannevedosta ei voitu ottaa, joten mitään ei palautettu. | Tiedot näyttävät zfs:n viestin. |
| `safety-name-too-long` | Tietojoukon nimi on liian pitkä turvatilannevedokselle. | Kytke turvatilannevedos pois, tai palauta kansioon. |
| `dataset-exists` | Tämänniminen tietojoukko on jo olemassa. | Valitse uusi nimi tai palauta suoraan tietojoukkoon. |
| `create-failed` | Uutta tietojoukkoa ei voitu luoda. | Yksityiskohdissa näkyy zfs:n viesti. Tarkista, että yläpuolinen tietojoukko on olemassa. |
| `new-dataset-not-visible` | Uusi tietojoukko luotiin, mutta BombVault ei näe sitä, joten mitään ei palautettu. | Tietojoukko jää palvelimelle. Liitä se Host Data -polun alle ja palauta sen sisään. |
| `set-properties-failed` | Tallennettuja ominaisuuksia ei voitu asettaa, joten mitään ei palautettu. | Tiedot näyttävät zfs:n viestin. |
| `set-limits-failed` | Tiedostot palautettiin, mutta tallennettua kiintiötä tai varausta ei voitu asettaa. | Yksityiskohdissa näkyy zfs:n viesti. Aseta kiintiö tai varaus itse komennolla `zfs set`. |

### Tarkista, mitä kontti näkee {#mountinfo}

**Kokeile pääsyä tilannevedoksiin** kohteessa ottaa sen puusta oikean tilannevedoksen, etsii sitä BombVaultin sisältä jokaiselle tietojoukolle ja poistaa sen taas. Se on nopein tapa todentaa koko reitti ennen ensimmäistä ajastettua ajoa.

Jos haluat katsoa itse, aja tämä palvelimella:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Jokainen rivi on yksi liitos kontin sisällä. Tietojoukon rivi näyttää sen polun kontin sisällä (polun `/host/user` alla) ja tietojoukon nimen. Kenttä `master:N` tällä rivillä tarkoittaa, että liitos saa isännän myöhemmin tekemät liitokset, ja sitä pääsy tilannevedoksiin tarvitsee. Jos se puuttuu, aseta Host Datan Access Mode arvoon Read/Write - Slave ja käynnistä BombVault uudelleen.

## TrueNAS SCALE {#truenas}

- Kun `LIBVIRT_URI` on asetettu (kuten virtuaalikoneiden varmuuskopioita varten TrueNASissa), BombVault ottaa zfs-komentojensa SSH-isännän, käyttäjän ja portin URI:sta, jokaisen, jota ei ole asetettu erikseen. Ilman virtuaalikoneiden varmuuskopioita aseta sen sijaan `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` ja `LIBVIRT_SSH_PORT`. Lisää muuttujat kohtaan **Additional Environment Variables**.
- Muu käyttäjä kuin root tarvitsee oikeudet kohteen ylimpään tietojoukkoon, jolloin ne kattavat jokaisen sen alla olevan tietojoukon:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Palautus uuteen tietojoukkoon tarvitsee lisäksi `create`-oikeuden yläpuoliseen tietojoukkoon, ja tallennettujen ominaisuuksien asettaminen tarvitsee oikeudet niihin ominaisuuksiin.

  TrueNASissa SSH-istunnolla ilman root-oikeuksia ei ole polussaan hakemistoa `/usr/sbin`; BombVault kutsuu silloin suoraan `/usr/sbin/zfs`.
- Sovelluksen **Host Data** -polun on oltava isäntäpolku tietojoukkojen yläpuolella, esimerkiksi `/mnt/tank`, ei ixVolume. Isäntäpolun kanssa sovellus välittää isännän uudet liitokset BombVaultille (`rslave`), ja sitä pääsy tilannevedoksiin tarvitsee.
