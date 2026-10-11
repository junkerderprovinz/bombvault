# ZFS-tietojoukot

ZFS-tietojoukot ovat merkintöjä **Varmuuskopiot**-luettelossa, kuten kontit ja VMs. Merkintä on yksi tietojoukko kaikkine sen alapuolisine tietojoukkoineen. Jokaista varmuuskopiota varten BombVault ottaa yhden ZFS-tilannevedoksen koko puusta, joten jokainen sen tietojoukko tallennetaan samalta hetkeltä. Se lukee sitten kunkin tietojoukon tiedostot tuosta tilannevedoksesta ja tallentaa ne resticillä samalla tavalla kuin kansion, ja poistaa tilannevedoksen heti perään. Varmuuskopiot ovat deduplikoituja, voit selata jokaista, ja yksittäisiä tiedostoja voi palauttaa.

Varmuuskopio ei koskaan syötä datavirtaa resticiin eikä koskaan palauta tietojoukkoa aiempaan tilaan. BombVault tuhoaa vain tilannevedoksia, jotka se on itse luonut. Valinnainen [replika](#replica) on ainoa paikka, jossa käytetään komentoa `zfs send`: se kopioi tietojoukot toiselle ZFS-palvelimelle eikä koske varmuuskopioon.

## Vaatimukset {#requirements}

- **SSH-yhteys tähän palvelimeen.** ZFS-tietojoukot käyttävät samaa avainta, isäntää ja käyttäjää kuin virtuaalikoneiden varmuuskopiot. Jos ne toimivat jo, tämäkin toimii. Muuten seuraa GitHubissa olevaa [ohjetta virtuaalikoneen varmuuskopiosta SSH:n kautta](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). Mallin kentät ovat **Host SSH: Address**, **Host SSH: Port** ja **Host SSH: User**.
- **`zfs`-komento kyseisessä isännässä.** Unraid 6.12 ja uudemmat sekä TrueNAS SCALE sisältävät sen.
- **Host Data liitettynä polkuun `/mnt` Access Mode -asetuksella Read/Write - Slave.** Se on mallin oletus. Tietojoukon tilannevedos näkyy tietojoukon kansiossa `.zfs/snapshot` vasta BombVaultin käynnistyttyä, joten kontin täytyy saada isännän myöhemmin tekemät liitokset.
- **Tietojoukot liitettyinä polun `/mnt` alle.** Unraidissa poolit ovat polussa `/mnt/<pool>`, joten näin on jo.

Ota osio käyttöön kohdassa **Asetukset, Yleiset** (ZFS-tietojoukot). **ZFS-yhteys tähän palvelimeen** Varmuuskopiot-sivun kohdassa **Lisää** avautuu sen jälkeen. Se testaa SSH-yhteyden, nimeää käyttäjän ja isännän, johon se yhdistää, ja kertoo, mikä puuttuu, kun jokin puuttuu. Kun yhteys on poikki, Varmuuskopiot-luettelo kertoo sen toiminnolla **Näytä yhteys**, ja yleiskatsauksen **Isäntäintegraatio**-kortti näyttää saman tuloksen.

## Merkinnät ja alatietojoukot {#items-and-children}

Valitse Varmuuskopiot-sivulla **Lisää** ja sitten **ZFS-tietojoukko**. Luettelo tulee palvelimelta. Valitse tietojoukko varmuuskopioitavan yläpäästä, esimerkiksi `cache/appdata`, niin merkintä kattaa sen ja jokaisen sen alapuolisen tietojoukon.

- **Uudet alatietojoukot liittyvät itsestään.** Myöhemmin merkinnän alle luotu tietojoukko varmuuskopioidaan seuraavassa ajossa, ja ajo nimeää sen uudeksi. Sen ensimmäinen varmuuskopio lukee sen kokonaan kerran; sen jälkeen luetaan vain muutokset.
- **Yksittäisiä lapsia voi jättää pois.** Kytke lapsi pois merkinnän Asetukset-välilehdellä, niin se jätetään pois kaiken sen alapuolisen kanssa. Pois jätetty lapsi, jota ei enää ole palvelimella, merkitään sellaiseksi ja sen voi poistaa luettelosta.
- **Lapset, joita ei voi lukea, ohitetaan, ei koskaan äänettömästi.** Ajo listaa ne, merkintä näyttää, kuinka monta ohitettiin, ja yleiskatsauksen **Ei varmuuskopioitu** -kortti laskee jokaisen suojaamattomaksi. Ajo varmuuskopioi silti kaiken muun eikä epäonnistu ohitetun lapsen vuoksi. Syyt ovat [syykoodien taulukossa](#reason-codes): tietojoukko, jota ei ole liitetty, jolla on `canmount=off`, `legacy`-liitoskohta tai ei liitoskohtaa lainkaan, salausavain on lataamatta, tilannevedospääsy on kytketty pois tai liitoskohta, jota BombVault ei näe.
- **Ohitettu tietojoukko ei vie alatietojoukkojaan mukanaan.** Tietojoukko, jolla on `canmount=off` ja joka sisältää vain muita tietojoukkoja, ohitetaan (näytetään "vain rakenne"), ja sen liitetyt alatietojoukot varmuuskopioidaan. Salattu tietojoukko, jonka avainta ei ole ladattu, ohitetaan yhdessä niiden alatietojoukkojen kanssa, jotka jakavat sen avaimen.
- **Alatietojoukot, jotka ovat virtuaalikoneen levyjä tai järjestelmätietoja, ovat aluksi pois päältä** lisäysikkunassa, ja syy näkyy kytkimen vieressä. Kokonaisen poolin lisääminen pyytää vahvistuksen, joka luettelee, mitä poolissa on.

### Taltiot {#volumes}

Taltio (zvol) sisältää virtuaalilevyn tiedostojen sijaan, eikä ZFS-merkintä koskaan varmuuskopioi sellaista.

- VM:n käyttämä taltio varmuuskopioidaan kyseisen VM:n mukana.
- Taltiota, jota mikään VM ei käytä (iSCSI-extent, irrottamasi levy), **BombVault ei varmuuskopioi**. Lisäysikkuna ja Varmuuskopiot-sivu laskevat nämä taltiot ja kertovat sen. Myöhempi versio varmuuskopioi ne.

Merkinnän puun sisällä olevat taltiot ohitetaan ja nimetään jokaisessa ajossa.

### Dockerin tallennustila {#docker-storage}

Dockerin ZFS-tallennusajurilla jokainen image-kerros on tietojoukko, jolla on `legacy`-liitoskohta. Lisäysikkuna taittaa nämä yhteen riviin vanhempaa kohti. Puu, jossa on yli 20 tällaista tietojoukkoa, ei voi tulla merkinnäksi niin kauan kuin siitä on tilannevedos: Docker ei voi poistaa image-kerroksia. Lisää sen sijaan sen alapuoliset tietojoukot, esimerkiksi `appdata`.

### Merkinnät eivät koskaan limity {#overlap}

Tietojoukko voi kuulua vain yhteen merkintään. BombVault hylkää uuden merkinnän, joka on olemassa olevan sisällä tai joka sisältäisi sellaisen. Yhdistääksesi useita alamerkintöjä yhdeksi yläpuoliseksi merkinnäksi poista ensin alamerkinnät ja valitse säilyttää niiden varmuuskopiot, lisää sitten yläpuolinen. Jokainen tietojoukko säilyttää historiansa omalla nimellään, joten seuraava varmuuskopio jatkaa siitä, mihin vanhat merkinnät jäivät, eikä lue kaikkea uudelleen.

## Konttien pysäyttäminen ja komennot tilannevedoksen ympärillä {#consistency}

Käynnissä olevan tietokannan tilannevedos on kuin äkillinen sähkökatko: tietokanta yleensä toipuu, mutta sen täytyy toipua. Jokainen merkintä voi tehdä asialle kaksi asiaa, ja molemmat kattavat vain tilannevedoksen hetken, eivät koko varmuuskopiota.

- **Pysäytä nämä kontit tilannevedoksen ajaksi.** BombVault pysäyttää luetellut kontit, ottaa tilannevedoksen ja käynnistää ne heti uudelleen. Saman riippuvuustason kontit pysähtyvät rinnakkain, riippuvaiset ensin, joten koko ikkuna kestää yleensä muutaman sekunnin; ajo näyttää, kuinka kauan. Varmuuskopio lukee sen jälkeen jäädytettyä tilannevedosta, kun sovellukset jo toimivat taas. Vain käynnissä olleet kontit pysäytetään.
- **Komento ennen tilannevedosta ja sen jälkeen.** Se ajetaan valitsemassasi kontissa, esimerkiksi tietokannan vedostamiseksi tietojoukkoon juuri ennen tilannevedosta pysäyttämättä mitään. Jos komento ennen tilannevedosta epäonnistuu, varmuuskopio epäonnistuu eikä tilannevedosta oteta. Tilannevedoksen jälkeinen epäonnistunut komento näytetään ajossa, mutta se ei kaada varmuuskopiota.

Mitä tapahtuu, kun jokin menee pieleen:

- Jos konttia ei voi pysäyttää, BombVault käynnistää jo pysäyttämänsä kontit, ja varmuuskopio epäonnistuu kontin nimen kanssa. Se ei koskaan tyydy tilannevedokseen käynnissä olevista sovelluksista.
- Pysäytys odottaa, kunnes käynnissä oleva konttivarmuuskopio on valmis (enintään 30 minuuttia manuaalisessa ajossa, varmuuskopion aikarajaan asti ajastetussa), jotta ne kaksi eivät koskaan pysäytä ja käynnistä samaa konttia yhtä aikaa.
- Ennen kuin ensimmäinen kontti pysäytetään, BombVault kirjoittaa muistiin, mitkä se pysäyttää. Jos BombVault tapetaan ikkunan sisällä, se käynnistää nuo kontit uudelleen seuraavalla käynnistyskerralla, lähettää ilmoituksen, ja merkintä näyttää punaisen huomautuksen jokaisesta kontista, jota se ei pystynyt käynnistämään.

Automaattiset tietokantavedokset (katso [Ominaisuudet](features.md)) ajetaan kontin oman varmuuskopion mukana, ei ZFS-merkinnän. Tietokanta, jonka kontti varmuuskopioidaan vain tietojoukkonsa kautta, ei saa vedosta, joten anna sille komento täällä.

Kontti voi olla tällä listalla ja varmuuskopioitu konttina samaan aikaan. Sen data tallennetaan silloin kahdesti, kahteen repositoryyn, ja täysvarmuuskopio pysäyttää sen kahdesti. Merkintä kertoo sen.

## Palauttaminen {#restore}

Avaa merkintä ja valitse **Palautus**, tai valitse se **Palautus**-sivulla. Palautusikkuna kysyy varmuuskopiota ja sitten **Tietojoukkoa**. Oletuksena se on merkinnän ylin tietojoukko. Kohdassa **Mitä** palautat kaiken tai vain valitsemasi tiedostot ja kansiot. **Minne** tarjoaa:

- **Palauta tietojoukkoon.** Varmuuskopion tiedostot kirjoitetaan tietojoukon liitoskohtaan. Samannimiset tiedostot korvataan, muut jäävät. Tietojoukkoa ei koskaan palauteta aiempaan tilaan eikä korvata. BombVault tarkistaa, että tietojoukko on liitetty, näkyvissä ja kirjoitettavissa, kerran ennen aloitusta ja uudelleen juuri ennen kirjoittamista. Kohtiin, joihin on liitetty alatietojoukko, ei kirjoiteta mitään: alatietojoukko säilyttää tiedostonsa, omistajansa ja oikeutensa, ja se palautetaan omasta varmuuskopiostaan.
- **Kansioon.** Valitse kansio kohteen `/mnt` alta. BombVault tarkistaa, että kansio on liitetyllä poolilla tai jaolla ja että vapaata tilaa on tarpeeksi. Tämä toimii ilman SSH-yhteyttä ja tietojoukoille, joita ei enää ole.
- **Uuteen tietojoukkoon.** Anna nimi tietojoukolle, jota ei vielä ole; ikkuna ehdottaa `<tietojoukko>-restored`. BombVault luo sen varmuuskopioon tallennetuilla ZFS-ominaisuuksilla ja palauttaa siihen, katso [Palautus uutena tietojoukkona](#new-dataset).

Kun tietojoukoksi valitaan **Tämän varmuuskopion kaikki tietojoukot**, jokainen puun tietojoukko kirjoitetaan omaan alikansioonsa valitsemasi kansion sisään. Tietojoukot, jotka ohitettiin kyseisessä varmuuskopiossa, nimetään. Jos haluat palauttaa toisen BombVaultin repositorystä, käytä Palautus-sivun kohtaa **Vanhasta varmuuskopiokansiosta**; se palauttaa aina kansioon: yhden varmuuskopion kaikki tietojoukot kukin omaan alikansioonsa, tai yhden puun tietojoukon kokonaisena tai valituin tiedostoin.

Merkinnän pysäytettävien konttien luettelo tarjotaan myös palautukselle tietojoukkoon. Nuo kontit pysyvät pysäytettyinä koko palautuksen ajan, ja konttivarmuuskopiot odottavat sillä välin.

### Turvatilannevedos {#safety-snapshot}

Ennen kuin BombVault kirjoittaa tietojoukkoon, se ottaa ZFS-tilannevedoksen vain siitä yhdestä tietojoukosta nimellä `bombvault-prerestore-<aika>`. **Ota ensin ZFS-tilannevedos nykytilasta** on oletuksena päällä; sen kytkeminen pois vaatii toisen vahvistuksen. Jos tilannevedosta ei voi ottaa, mitään ei palauteta.

BombVault ei koskaan poista turvatilannevedosta itse. Merkintä listaa ne ikineen ja kokoineen, kullakin **Poista**-toiminto, ja varoittaa, kun vanhin on yli 30 päivän ikäinen, koska se pitää poistetun ja muuttuneen datan poolilla.

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

Replika on merkinnän tietojoukkojen kopio toisella ZFS-palvelimella. BombVault pitää sen ajan tasalla komennoilla `zfs send` ja `zfs receive`. Ensimmäinen ajo lähettää kaiken, sen jälkeen kulkevat vain muuttuneet lohkot. Toisella palvelimella voit liittää kopion heti.

Replika ei koskaan korvaa varmuuskopiota. Vanhemmat versiot, yksittäiset tiedostot ja tarkistus tulevat yhä varmuuskopioista, ja replika säilyttää vain niin monta tilannevedosta kuin asetat. Ajantasainen replika lasketaan kopioksi tilojen ulkopuolella, mutta merkintä, jolla on replika mutta ei varmuuskopiota, pysyy oranssina.

Kytke se päälle merkinnän Asetukset-välilehden kortissa **Replika**. Siellä valitset, minne replika menee, milloin se ajetaan (**Jokaisen varmuuskopion jälkeen** tai **Oma aikataulu**) ja kuinka monta tilannevedosta kohteelle jää. Kortti listaa jokaisen tietojoukon ja taltion tiloineen, ja **Replikoi nyt** käynnistää ajon. Replikaajolla on oma lukkonsa, joten pitkä ensimmäinen siirto ei koskaan pidätä varmuuskopioita.

### Lähetys ZFS-palvelimelle {#replica-push}

Mikä tahansa kone, jossa on ZFS ja SSH, voi vastaanottaa, esimerkiksi toinen Unraid tai TrueNAS. BombVaultin ei tarvitse toimia siellä.

1. Valitse kohdassa **Asetukset, Pariliitos** **Yhdistä SSH:n kautta** kohdassa **ZFS-palvelin ilman BombVaultia**, tai **Lisää ZFS-palvelin** merkinnän **Replika**-kortissa.
2. Anna osoite, käyttäjä ja portti. Ikkuna näyttää BombVaultin julkisen avaimen. Lisää se käyttäjän tiedostoon `~/.ssh/authorized_keys` palvelimella. Unraidissa se on kohdassa **Settings, Users, root, SSH keys**.
3. Testaa yhteys. Ikkuna luettelee sitten palvelimen poolit. Valitse yksi ja aseta juuri, jonka oletus on `<pool>/bombvault-replica`.
4. Valitse uusi palvelin merkinnän **Replika**-kortissa. Palvelimella on sen jälkeen oma korttinsa Ilmentymät-sivulla, ja **Avaa** vie sen sivulle.

Rootilla muuta ei tarvita. Omalla käyttäjällä tarvitaan nämä oikeudet kohdepalvelimen poolissa, jotka ikkuna myös näyttää:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

Lähteellä samanlainen käyttäjä tarvitsee nämä merkinnän ylimmälle tietojoukolle:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

Tässä suunnassa BombVault, jolla merkintä on, pitää myös avainta, joka voi kirjoittaa palvelimelle.

BombVault tallentaa isäntäavaimen, jonka palvelin näyttää ensimmäisellä yhteydellä, ja hylkää myöhemmin muut. Jos palvelin on asennettu uudelleen, paina palvelimen sivulla **Forget host key**, niin seuraava yhteys tallentaa uuden avaimen.

### Lähetys pariliitetylle instanssille {#replica-receive}

Pariliitetty BombVault voi vastaanottaa replikan itse. Kukaan ei saa SSH-pääsyä toiselle isännälle, eikä mikään avain mene `authorized_keys`-tiedostoon.

1. Valitse merkinnän **Replika**-kortissa pariliitetty instanssi kohteeksi, tai kytke **ZFS-palvelin**-rooli päälle kyseisen instanssin kortilla Ilmentymät-sivulla ja valitse ZFS-merkinnät siellä. Kortti näyttää **Odottaa hyväksyntää**, kunnes se vastaa.
2. Avaa vastaanottavassa instanssissa **Ilmentymät**. Pyyntö näkyy lähettävän instanssin kortilla merkkinä, ja sen **Tiedot** listaavat sen. Valitse kyseisen palvelimen pooli ja juuri sekä kuinka monta tilannevedosta jää, paina sitten **Salli** tai **Hylkää**.
3. **Allow**-painalluksen jälkeen lähde lähettää oman aikataulunsa mukaan, kuten mihin tahansa muuhun kohteeseen.

Vastaanottava instanssi hyväksyy vain sen, mitä hyväksyntä kattaa: merkinnän tietojoukot, omaan juureensa. Se ajaa `zfs receive` -komennon itse, eikä lähteellä ole mitään keinoa poistaa tai palauttaa mitään siellä. Kopio säilyy siis, vaikka joku olisi ottanut lähteen haltuunsa. Vastaanottava instanssi pitää oman säilytyksensä. Lähde vain ehdottaa sääntöä pyytäessään.

**Revoke access** vastaanottavassa instanssissa päättää hyväksynnän milloin tahansa ja kertoo siitä lähteelle, joka näyttää sitten hyväksynnän peruutetuksi ja lopettaa. Se, mitä vastaanottava instanssi jo pitää hallussaan, jää sinne. Hylätty pyyntö pysyy hylättynä. Lisää tietojoukkoja tai pyyntö peruutuksen jälkeen kysyvät uudelleen.

Vastaanottavan instanssin on vastattava HTTPS:n kautta. Pelkkään http-osoitteeseen lähde ei lähetä mitään, koska tunniste ja tiedot kulkisivat verkossa salaamattomina. Jokainen siirto jättää lisäksi kymmenesosan vastaanottavasta poolista vapaaksi ja pysähtyy ennen kuin alkaisi käyttää sitä, joten yksi lähde ei voi täyttää vastaanottavan palvelimen poolia.

### Mihin data päätyy {#replica-target}

Jokainen tietojoukko päätyy polkuun `<root>/<server>/<pool>/<path>`. Palvelinkansio on lähdeinstanssin nimi, joka lukitaan ensimmäisessä siirrossa, joten kaksi palvelinta, joilla on sama poolin nimi, eivät koskaan joudu toistensa tielle. Esimerkiksi palvelimen `tower` tietojoukko `cache/appdata` päätyy polkuun `backup/bombvault-replica/tower/cache/appdata`.

Kopio kohdepalvelimella on vain luettavissa eikä liitettynä, joten se ei koskaan peitä mitään kyseisellä palvelimella. ZFS-ominaisuudet siirtyvät mukana, paitsi liitospiste sekä `sharenfs` ja `sharesmb`. Myös varaukset `reservation` ja `refreservation` jäävät pois, joten kopio vie vain sen tilan, jonka sen data tarvitsee.

### Mitä mukaan tulee {#replica-contents}

Mukaan tulee kaikki, minkä merkintä varmuuskopioi, ja sen alapuoliset taltiot myös, jotka varmuuskopio ohittaa. Alatietojoukko, jonka kytkit pois merkinnässä, jää pois. Kaikki ajon tietojoukot tulevat yhdestä tilannevedoksesta, kuten varmuuskopiossa.

### Kuinka kauan tilannevedokset säilyvät {#replica-retention}

Kohdepalvelimella uusi replika säilyttää 7 päivittäistä ja 3 viikoittaista tilannevedosta. Valitse sen sijaan **Short**, **Balanced** tai **Long**, tai aseta **Custom values**. Siellä poistetaan vain tilannevedoksia, joiden nimi on `bombvault-replica-<14 numeroa>`, eikä koskaan uusinta, jonka molemmat puolet jakavat.

Lähteellä BombVault säilyttää vain viimeisimmän replika-tilannevedoksen sekä kirjanmerkin jokaiselle lähettämälleen tilalle. Kirjanmerkit eivät vie tilaa. Seuraava siirto alkaa niistä.

### Salatut tietojoukot replikassa {#replica-encryption}

Salattu tietojoukko lähetetään raakamuodossa. Se pysyy salattuna kohdepalvelimella, eikä kohdepalvelin koskaan näe avainta. Säilytä avain turvallisesti: tarvitset sitä kopion avaamiseen palautuksen jälkeen, ja replika ilman avainta on lukukelvoton.

### Replikan käyttö {#replica-use}

Avaa merkinnän **Varmuuskopiot**-välilehti ja napsauta **ZFS-replika**-riviä tilannevedoksia pitävien paikkojen joukossa. Taulukko listaa kohteen tilannevedokset ja näyttää komennot todellisilla nimillesi.

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

- ennen kyseisen merkinnän seuraavaa varmuuskopiota,
- kun BombVault käynnistyy, jokaiselle merkinnälle, myös osion ollessa kytketty pois,
- kun poistat merkinnän,
- kun painat **Poista nyt** merkinnällä, joka näyttää, kuinka monta on jäljellä.

Vain nimet, jotka ovat täsmälleen `bombvault-` ja 14 numeroa, poistetaan. Turvatilannevedoksiin, omiin tilannevedoksiisi ja automaattisiin tilannevedoksiin ei koskaan kosketa. Jos haluat poistaa sellaisen käsin:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

Replikan tilannevedoksen nimi on `<dataset>@bombvault-replica-<14 numeroa>`, eikä se ole jäännös. Se pysyy lähteellä, kunnes seuraava replika-ajo korvaa sen, ja kohdepalvelimella niin kauan kuin säilytys pitää sen. Siivooja ei koskaan koske siihen, koska se vastaa vain nimiä, joissa `bombvault-` seuraa täsmälleen 14 numeroa.

## Poikkeamat {#anomalies}

Tyhjennetty lapsi muuttaa suuren puun kokonaismäärää tuskin lainkaan, joten poikkeamatunnistus valvoo merkinnän jokaista tietojoukkoa erikseen: sen koolla, tiedostomäärällä, uudella datalla ja resticin ajalla on kullakin oma historiansa. Historia kuuluu tietojoukon nimelle, joten se säilyy, kun puu myöhemmin varmuuskopioidaan toisella merkinnällä.

Tietojoukko, jonka edellinen ajo varmuuskopioi ja tämä ajo ei voinut lukea, lasketaan tyhjennetyksi, kunhan merkinnän valinta ei muuttunut. Tämä kattaa lataamattoman avaimen, liittämättömän tietojoukon ja puusta kadonneen. Lapsi, jonka suljet itse pois, muuttaa valintaa, joten sen historia alkaa alusta. Kun havainto menetetystä datasta on avoinna, säilytys pitää kyseisen yhden tietojoukon vanhat varmuuskopiot ja karsii loput puusta tavalliseen tapaan.

Jokaisella tietojoukolla on oma rivinsä merkinnän valvonnassa, joka avautuu kohdalla **Valvonta** **Poikkeamat**-sivulla ja on merkinnän **Eheys**-välilehdellä. Avoimet havainnot näkyvät kunkin tietojoukon vieressä. Havainnon linkki avaa merkinnän palautusikkunan tietojoukon viimeisimmällä hyvällä varmuuskopiolla. Se, päättyykö ajo, arvioidaan koko merkinnälle, koska ajo onnistuu tai epäonnistuu kokonaisena.

Tarkistukset itse kuvataan kohdassa [Ominaisuudet](features.md). [MCP-palvelimen](mcp.md) kautta yhdistetty avustaja voi listata ZFS-merkinnän palautuspisteet, käynnistää sen varmuuskopion ja lukea havainnot, mutta yhden kuittaaminen tehdään **Poikkeamat**-sivulla.

## Syykoodit {#reason-codes}

Merkinnän sivu, ajohistoria ja ilmoitukset nimeävät ongelman jollakin näistä koodeista. Useimmilla on korjaus myös vieressään sivulla.

| Koodi | Merkitys | Mitä tehdä |
|---|---|---|
| `ssh-missing` | SSH-yhteyttä ei ole määritetty tässä kontissa. | Määritä SSH-yhteys kuten virtuaalikoneiden varmuuskopioille. |
| `host-placeholder` | Host SSH: Address on yhä esimerkkiarvo, eikä `host.docker.internal` vastannut. | Aseta Host SSH: Address tämän palvelimen LAN-IP:ksi. |
| `host-fallback` | Host SSH: Address on yhä esimerkkiarvo, ja `host.docker.internal` toimii. | Ei mitään, tai aseta LAN-IP. |
| `ssh-unreachable` | Palvelinta ei tavoiteta SSH:lla. | Tarkista osoite ja portti ja että SSH on päällä. |
| `ssh-auth` | Palvelin hylkäsi BombVaultin avaimen. | Aja kohdassa **Yhteys tähän palvelimeen** näkyvä komento kerran palvelimella. |
| `zfs-not-found` | SSH-isännällä ei ole `zfs`-komentoa. | Osoita Host SSH: Address koneeseen, joka omistaa poolit. |
| `zfs-permission` | SSH-käyttäjä ei saa ajaa tätä zfs-komentoa. | Käytä root-käyttäjää, tai katso [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` nimeää eri isännän tai käyttäjän kuin SSH-kentät. | Täsmäytä ne, tai tyhjennä SSH-kentät, jolloin molemmat tulevat URI:sta. |
| `zfs-error` | zfs ilmoitti muusta virheestä. | Tiedot näyttävät sen viestin. |
| `propagation-missing` | Isännän uudet liitokset eivät tavoita konttia. | Aseta Host Datan Access Mode arvoon Read/Write - Slave ja käynnistä BombVault uudelleen. |
| `invalid-name` | Tietojoukon nimi, jota BombVault ei hyväksy. | Nimeä tietojoukko uudelleen. |
| `name-too-long` | Puun tietojoukko on liian pitkä tilannevedoksen nimeksi. | Nimeä se uudelleen tai lisää sen alapuolinen tietojoukko merkinnäksi. |
| `invalid-exclude` | Poissulkumalli tai pois jätetty lapsi ei sovi merkintään. | Korjaa viestin nimeämä merkintä. Jättääksesi pois koko lapsitietojoukon kytke se pois mallin kirjoittamisen sijaan. |
| `not-found` | Tietojoukkoa ei ole palvelimella. | Poista merkintä tai luo tietojoukko uudelleen. Sen varmuuskopiot pysyvät palautettavina. |
| `not-filesystem` | Tämä on taltio, ei tiedostojärjestelmä. | Katso [Taltiot](#volumes). |
| `overlaps-item` | Tietojoukko limittyy olemassa olevan merkinnän kanssa. | Katso [Merkinnät eivät koskaan limity](#overlap). |
| `docker-storage` | Puu sisältää Dockerin kuvatallennuksen. | Katso [Dockerin tallennustila](#docker-storage). |
| `nothing-readable` | Mitään merkinnän tietojoukkoa ei voi juuri nyt lukea. | Katso ohitettujen tietojoukkojen koodit. |
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

**Kokeile pääsyä tilannevedoksiin** merkinnän Asetukset-välilehdellä ottaa oikean tilannevedoksen sen puusta, etsii sen BombVaultin sisältä jokaiselle tietojoukolle ja poistaa sen taas. Se on nopein tapa todistaa koko polku ennen ensimmäistä ajastettua ajoa.

Jos haluat katsoa itse, aja tämä palvelimella:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Jokainen rivi on yksi liitos kontin sisällä. Tietojoukon rivi näyttää sen polun kontin sisällä (polun `/host/user` alla) ja tietojoukon nimen. Kenttä `master:N` tällä rivillä tarkoittaa, että liitos saa isännän myöhemmin tekemät liitokset, ja sitä pääsy tilannevedoksiin tarvitsee. Jos se puuttuu, aseta Host Datan Access Mode arvoon Read/Write - Slave ja käynnistä BombVault uudelleen.

## TrueNAS SCALE {#truenas}

- Kun `LIBVIRT_URI` on asetettu (kuten virtuaalikoneiden varmuuskopioita varten TrueNASissa), BombVault ottaa zfs-komentojensa SSH-isännän, käyttäjän ja portin URI:sta, jokaisen, jota ei ole asetettu erikseen. Ilman virtuaalikoneiden varmuuskopioita aseta sen sijaan `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` ja `LIBVIRT_SSH_PORT`. Lisää muuttujat kohtaan **Additional Environment Variables**.
- Muu käyttäjä kuin root tarvitsee oikeuden merkinnän ylimmälle tietojoukolle, joka kattaa silloin jokaisen sen alapuolisen tietojoukon:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Palautus uuteen tietojoukkoon tarvitsee lisäksi `create`-oikeuden yläpuoliseen tietojoukkoon, ja tallennettujen ominaisuuksien asettaminen tarvitsee oikeudet niihin ominaisuuksiin.

  TrueNASissa SSH-istunnolla ilman root-oikeuksia ei ole polussaan hakemistoa `/usr/sbin`; BombVault kutsuu silloin suoraan `/usr/sbin/zfs`.
- Sovelluksen **Host Data** -polun on oltava isäntäpolku tietojoukkojen yläpuolella, esimerkiksi `/mnt/tank`, ei ixVolume. Isäntäpolun kanssa sovellus välittää isännän uudet liitokset BombVaultille (`rslave`), ja sitä pääsy tilannevedoksiin tarvitsee.
