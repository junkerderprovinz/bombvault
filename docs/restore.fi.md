# Palautus

Jokainen palautus alkaa **Palautus**-sivulta: valitse osio, hae haluamasi ja valitse varmuuskopio. Yksi ikkuna palauttaa kaikenlaiset merkinnät ja kysyy vaiheittain. Oletuksena alkuperäiseen ei muutu mitään, sillä varmuuskopio palautuu sen viereen.

![Palautus-sivu, jossa osiot ovat ruutuina, haku ja Stackit-kortti.](assets/screenshots/restore/page-dark.png#only-dark)
![Palautus-sivu, jossa osiot ovat ruutuina, haku ja Stackit-kortti.](assets/screenshots/restore/page-light.png#only-light)

## Palautus-sivu {#page}

- **Osiot** listaa osiot ruutuina, kussakin se, kuinka monella merkinnällä on tilannevedoksia.
- **Mitä haluat palauttaa?** hakee valitun osion merkinnöistä. Jokaisella rivillä näkyy sen uusin tilannevedos, ja **Avaa merkintä** johtaa merkinnän sivulle.
- **Etsi tiedosto** etsii tiedostoa tai kansiota sen nimellä jokaisesta varmuuskopiosta, katso [Tiedoston etsiminen](#find-a-file).
- **Stackit** listaa Compose-projektit, joissa on vähintään kaksi varmuuskopioitua konttia, kuten `immich` tietokantoineen ja Redisineen. Haku löytää stackin nimellä tai minkä tahansa sen kontin nimellä.
- **Käynnissä nyt** näyttää palautuksen sen ajan, kun se on käynnissä, myös sen jälkeen kun suljit sen ikkunan.
- **Hätätilanne** käynnistää Hätäohjatun toiminnon palvelimelle, joka on poissa tai pystytettiin uudelleen, katso [Hätäohjattu toiminto](offsite-recovery.md#emergency).
- **Muut lähteet** sisältää kohdat **Vanhasta varmuuskopiokansiosta**, toisen BombVaultin varmuuskopioille, **Tuo Appdata.Backup-liitännäisestä** ja **Tehtävät Duplicatista tai Duplicacysta**.
- **Muut palvelimet** listaa jaetun arkiston muut palvelimet, kun tämä palvelin on siinä mukana, katso [Muut palvelimet](#other-servers).

![Palautus-sivu puhelimella.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![Palautus-sivu puhelimella.](assets/screenshots/restore/page-phone-light.png#only-light)

## Palautusikkuna {#window}

Merkinnän napsautus avaa **Palauta**-ikkunan, jossa on sen nimi. Ikkuna kysyy vaiheittain, ja jokaista vaihetta voi muuttaa ennen aloitusta:

1. **Varmuuskopio**: suositeltu tilannevedos on valittu valmiiksi. Se on uusin tai avoimen poikkeaman jälkeen viimeinen ennen sitä. Aiempaa palautusta ennen otettu varmuuskopio sisältää tilan, jonka se palautus korvasi, joten se ohitetaan, ellei muuta ole. Vanhemmat ovat listassa, ja **Vertaa** näyttää, mikä kahden välillä eroaa.
2. **Lähde**: mistä tiedostot tulevat, kun tilannevedos on useassa paikassa. **Paikallinen** on nopein.
3. **Mitä**: **Kaikki** tai **Yksittäiset tiedostot**, jolloin valitset haluamasi tiedostot ja kansiot.
4. **Minne**: kohde. Vaihtoehdot riippuvat merkinnän lajista:

    | Kohde | Mitä se tekee |
    |---|---|
    | **Alkuperäisen viereen** | Oletus. Kontti palautuu toisena konttina nimellä `<name>-restored` omilla porteillaan, virtuaalikoneen levyt ja NVRAM päätyvät kansioon nimeltä `<folder>-restored`, ja kansiojoukko palautuu polkuun `<path>-restored`. Alkuperäiseen ei muutu mitään. |
    | **Korvaa alkuperäinen** | Ylikirjoittaa sen, mitä siellä nyt on. Kun **Varmuuskopioi nykytila ensin** on päällä, mikä on oletus, BombVault varmuuskopioi ensin tämän hetken tilan, jotta voit palata. Jos varmuuskopiointi epäonnistuu, palautusta ei aloiteta. Se toimii myös kontille tai VM:lle, jota ei enää ole palvelimella: kansiot, joiden päälle palautus kirjoittaa, varmuuskopioidaan erikseen. |
    | **Kansioon** | Vain tiedostot selattavaksi. |
    | **Lataa ZIP-tiedostona** | Flash-asema uutta tikkua varten. BombVault ei koskaan kirjoita käynnissä olevalle flash-asemalle. |
    | **Uuteen tietojoukkoon** ja **Palauta tietojoukkoon** | ZFS-tietojoukot, katso [ZFS-tietojoukot](zfs-datasets.md#restore). |

5. **Mitä tapahtuu** kertoo sanoin, mitä palautus tekee, ja kuva näyttää valitsemasi kohteen.

Ennen kuin **Palauta** aukeaa, ikkuna tarkistaa, että repo vastaa, että avain avaa sen, että tilannevedos on olemassa ja että kohteessa on tilaa. Kun jokin tarkistus epäonnistuu, painike pysyy lukittuna, ja siinä oleva (i) kertoo, mikä.

Ikkuna näyttää myös, mitä palautus muuttaa verrattuna siihen, mitä nyt on: uudet, korvatut ja muuttumattomat tiedostot, lista pyynnöstä, sekä kohteen tiedostot, joita ei ole varmuuskopiossa ja jotka jäävät paikalleen. restic laskee tämän kuivaajona koon ja muokkausajan perusteella lukematta tiedostoja, ja erittäin suuri puu pysähtyy 30 sekunnin kuluttua ja kertoo sen. Konteille ja virtuaalikoneille se vertaa asetuksia, jotka palautus luo uudelleen, käynnissä oleviin: image ja tunniste, portit, muuttujien nimet ja taltiot, tai muisti, vCPU:t, levyt ja verkko.

Jotkin asiat voivat pysäyttää palautuksen ennen kuin se muuttaa mitään. Kontin staattisen IP:n ja julkaistujen porttien on oltava vapaina, ja sen verkon on oltava olemassa. Unraidissa jokainen paikalleen tehtävä palautus kieltäytyy kansiosta, joka ei ole liitetyllä poolilla tai jaolla, koska tiedostot päätyisivät muistiin ja katoaisivat uudelleenkäynnistyksessä. Paikalleen tehtävä palautus nimeää jokaisen muun kontin, jonka bind mount ylettyy kansioon, johon se kirjoittaa, esimerkiksi "tätä polkua käyttää myös nextcloud-db", mutta se vain varoittaa.

Muutama valinta ilmestyy siellä, missä se on käyttökelpoinen: **Jätä pysäytetyksi jälkeenpäin**, jotta voit katsoa konttia tai virtuaalikonetta ennen kuin se käynnistyy uudelleen, ja **Palauta ilman GPU:ta ja ajoympäristöä** isännällä, jolta puuttuu GPU tai ajoympäristö, jolla varmuuskopio tehtiin.

Palautus ajetaan taustalla. Voit sulkea ikkunan, ja edistyminen näkyy myös kohdassa **Käynnissä nyt**. Pitkä palautus näyttää prosentin ja sen voi peruuttaa, ja peruutettu palautus kirjataan peruutetuksi, ei epäonnistuneeksi. Palautus, joka peruutetaan kontin pysähtyessä, tai palautus, joka ei pysty poistamaan konttia, ei ole vielä palauttanut mitään, joten kontti käynnistyy uudelleen. Kontti, joka ottaa taltionsa toiselta kontilta `--volumes-from`-valinnalla, säilyttää ne, eikä palautus suostu alkamaan niin kauan kuin sitä toista konttia ei ole.

![Palautusikkuna Mitä-vaiheessa, yksittäiset tiedostot valittuina.](assets/screenshots/restore/window-what-dark.png#only-dark)
![Palautusikkuna Mitä-vaiheessa, yksittäiset tiedostot valittuina.](assets/screenshots/restore/window-what-light.png#only-light)

### Minne kansioon palautus menee {#folders}

**Palautuskansiot** kohdassa **Asetukset, Palautus** asettaa **Palautuksen oletuskansion**, halutessasi **Alikansio osiota kohti** -valinnalla, sekä **Poikkeuksen** yhdelle osiolle. Valitut tiedostot päätyvät suoraan kohdekansioon: yksi tiedosto palautuu pelkkänä tiedostona, kansio kansiona sisältöineen, ja eri kansioista tehdyistä valinnoista säilyy vain polku niiden yhteisen kansion alapuolella. BombVault kieltäytyy kohteesta, joka ei ole liitetyllä tallennustilalla, koska Unraidissa sellainen kansio on muistissa ja katoaa uudelleenkäynnistyksessä, eikä se koskaan palauta omaan datakansioonsa.

## Esimerkki: yksi tiedosto takaisin {#example-single-file}

Joku muokkasi viime yönä Nextcloudin `config.php`-tiedostoa, eikä Nextcloud enää käynnisty. Yöllinen varmuuskopio kello 03.00 sisältää edelleen hyvän tiedoston. Haluat sen yhden tiedoston, et koko konttia.

1. Avaa **Palautus**, valitse **Kontit**-ruutu ja kirjoita `nextcloud` kenttään **Mitä haluat palauttaa?**.
2. Napsauta `nextcloud`. Ikkuna aukeaa kello 03.00 tehty varmuuskopio suositeltuna ja valmiiksi valittuna.
3. Jätä **Lähde** arvoon **Paikallinen**.
4. Valitse kohdassa **Mitä** **Yksittäiset tiedostot**. Avaa kansiot kansioon `config` asti ja valitse `config.php`.
5. Valitse kohdassa **Minne** **Kansioon** ja valitse kansio, esimerkiksi `user/restore`, joka on isännällä `/mnt/user/restore`.
6. Lue **Mitä tapahtuu**: tiedosto päätyy kohdekansioon, eikä konttiin muutu mitään. Paina **Palauta**.

Ikkuna päättyy tekstiin **Valmis: tiedostot ovat kansiossa /mnt/user/restore**. Löydät tiedoston polusta `/mnt/user/restore/config.php`, ilman sen yläpuolella olevia kansioita. Vertaa sitä rikkinäiseen, kopioi se takaisin appdata-kansioon ja käynnistä Nextcloud uudelleen.

Jos haluat laittaa tiedoston suoraan takaisin, valitse vaiheessa 5 **Korvaa alkuperäinen**. Pidä **Varmuuskopioi nykytila ensin** päällä, jotta BombVault varmuuskopioi rikkinäisen tilan ennen kirjoittamista, kirjoita vahvistukseksi `nextcloud` ja paina **Palauta**. Nextcloud pysäytetään, kun valittu tiedosto kirjoitetaan takaisin paikalleen, ja käynnistetään jälkeenpäin uudelleen. Vain valitsemasi tiedostot kirjoitetaan. Kesken epäonnistuva palautus jättää Nextcloudin pysäytetyksi, koska sen lukema tiedosto voi olla puoliksi kirjoitettu, ja ajo kertoo sen.

![Palautusikkuna Mitä tapahtuu -vaiheessa, Korvaa alkuperäinen valittuna.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![Palautusikkuna Mitä tapahtuu -vaiheessa, Korvaa alkuperäinen valittuna.](assets/screenshots/restore/window-replace-light.png#only-light)

## Tiedoston etsiminen {#find-a-file}

Kun tiedät tiedoston nimen mutta et sitä, missä merkinnässä tai varmuuskopiossa se vielä on, kirjoita nimi kenttään **Etsi tiedosto** ja paina **Hae**. Pelkkä teksti osuu mihin kohtaan nimeä tahansa eikä välitä kirjainkoosta, joten `invoice` löytää tiedoston `Invoice-2024.pdf`. Kun käytät merkkiä `*` tai `?`, kuvion on vastattava koko nimeä, joten `*.pdf` löytää jokaisen PDF:n. Kirjoita pelkkä nimi, ilman kansiota.

Haku lukee jokaisen konttien, kansiojoukkojen, ZFS-tietojoukkojen ja flash-aseman varmuuskopion. Jokainen osuma näyttää merkinnän, polun ja jokaisen varmuuskopion, jossa tiedosto on, uusin ensin, sekä päivämäärän, koon ja paikan, josta se luettiin. Varmuuskopio, jossa tiedostolla on sama koko ja muokkausaika kuin sitä edeltävässä, näyttää **ennallaan**, joten näet, missä kohtaa tiedosto viimeksi muuttui. Varmuuskopion napsautus avaa sille palautusikkunan, jossa **Yksittäiset tiedostot** on valittu ja tiedosto rastitettu. Flash-aseman tiedosto palautuu osana flash-aseman ZIP-tiedostoa, koska flash-asema palautetaan vain kokonaisena.

Haku kestää hetken, koska restic käy läpi jokaisen varmuuskopion. **Pysäytä** lopettaa sen, ja niin tekee myös sivulta poistuminen. Kentän alla kerrotaan, mitkä paikat haku luki:

- Tämän palvelimen ja verkkosi paikoista haetaan aina.
- Varmuuskopio, joka on useassa paikassa, luetaan vain kerran, ensimmäisestä. Paikka, jonka kaikki varmuuskopiot luettiin muualta, näyttää **käyty jo läpi toisessa paikassa**.
- Etäpaikat, kuten S3, B2 tai SFTP, jätetään pois, ellet kytke päälle valintaa **Hae myös etäpaikoista**. Silloin niistä luetaan vain ne varmuuskopiot, joita mikään paikka täällä ei sisällä, yleensä vanhemmat, jotka pidempi säilytys pitää etänä. Ensimmäinen haku tällaisesta paikasta voi kestää kauan, ja jotkin palveluntarjoajat laskuttavat ladattavasta datasta.
- Paikka, jota ei voitu avata, näyttää **ei voitu lukea**, ja sen vieressä oleva (i) kertoo syyn. Muista paikoista haetaan silti.

Virtuaalikoneet tallennetaan kokonaisina levyinä, joten haku ei käy läpi niiden sisällä olevia tiedostoja. Myöskään itsevarmuuskopiota, kohdan **Muut lähteet** toisen BombVaultin varmuuskopioita ja kohdan **Muut palvelimet** palvelimien varmuuskopioita haku ei käy läpi. Haku listaa ensimmäiset 200 tiedostoa, joten lyhyt nimi kuten `log` voi tarvita lisää kirjaimia.

## Mitä palautuu, lajeittain {#kinds}

**Kontit** luodaan uudelleen tallennetusta määrityksestään Docker-rajapinnan kautta, mukana entrypoint, isäntänimi, pysäytysasetukset, taltiot ja liitokset, resurssirajat, lokiajuri, DNS-asetukset ja vanhat linkit. Kontti palautuu imageen, jota se ajoi varmuuskopiota otettaessa, ei myöhemmin haettuun uudempaan, ja puuttuva image haetaan ensin kohdan **Asetukset, Palautus** **Container-rekisterit**-kirjautumisella. Myös sen GPU tai ajoympäristö (`--gpus`, `--runtime=nvidia`) palautuu. Isännällä, jolla ei ole kyseistä ajuria tai ajoympäristöä, palautus kertoo sen ja tarjoaa vaihtoehdon **Palauta ilman GPU:ta ja ajoympäristöä**. Linkki konttiin, jota ei ole tai joka on pysäytetty palautetun käynnistyessä, jätetään pois, ja ajohistoria kertoo sen. Palautus, joka epäonnistuu sen jälkeen, kun kontti oli poistettu, laittaa sen takaisin pysäytettynä.

Varmuuskopio voi sisältää kansioita, joita kontilla ei enää ole valittuna, esimerkiksi vanhempi varmuuskopio, joka otettiin ennen kuin kansio katosi. Kansio, joka on poissa tai tyhjä, palautuu. Kansio, jossa on nyt tiedostoja, pysyy ennallaan, koska olet ehkä poistanut sen valinnan tarkoituksella, ja palautusta edeltävä tarkistus ja ajohistoria nimeävät sen. Saat sen takaisin valinnalla **Yksittäiset tiedostot** tai **Kansioon**.

**Virtuaalikoneille** määritellään XML uudelleen SSH:n yli ja niiden levyt ja UEFI NVRAM liitetään takaisin, vaikka virtuaalikone olisi poistettu. Katso [VM-varmuuskopiot](vm-backup-ssh-setup.md) virtuaalikoneista, joilla on Unraid-tilannevedoksia ja zvol-levyjä.

**Kansiojoukot** palautuvat omistajuuksineen, oikeuksineen ja aikaleimoineen sellaisina kuin restic ne tallentaa. Kun kohde kieltäytyy vain omistajuudesta, kuten `/mnt/user` tekee, ajo päättyy huomautukseen, joka kertoo sen. BombVault vertaa sitten tiedostoja varmuuskopioon, ja tiedosto, joka ei saapunut perille, saa palautuksen epäonnistumaan.

**Flash-asema** ei koskaan kosketa elävää `/boot`-kansiota. Palautus on lataus nimeltä `flash-<id>.zip`, valmis pudotettavaksi Unraid USB Creatoriin. ZIP sisältää `APP_KEY`:n, joten sen lataaminen vaatii kirjautumissalasanan ja kirjautumisen instanssissa. Ikkuna listaa myös jokaisen flash-varmuuskopion liitännäiset versioineen ja kokoineen ja voi laittaa yksittäisen takaisin käynnissä olevalle flash-asemalle: sen `.plg`-tiedoston, sen kansion polussa `config/plugins` ja varmuuskopion sisältämät pakettitiedostot. Unraid asentaa sen seuraavassa käynnistyksessä tai heti kohdassa **Plugins, Install Plugin**.

**ZFS-tietojoukoilla** on omat kohteensa, ja ne säilyttävät ZFS-ominaisuutensa, katso [Palautus](zfs-datasets.md#restore).

**Itsevarmuuskopio** palautuu vain Hätäohjatun toiminnon kautta, joka käynnistää BombVaultin uudelleen, joten elävää tietokantaa ei koskaan ylikirjoiteta avoimen kahvan alta.

**Tietokantavedoksia** ei koskaan ladata takaisin itsestään. Rivin kohdassa **Tietokantavedokset** voi ladata, tallentaa kansioon tai tuoda juuri käynnistettyyn tietokantaan. Katso [Tietokantavedokset](backups.md#database-dumps).

## Useiden merkintöjen palautus kerralla {#several}

Valitse Palautus-sivulla merkintöjä samalla tavalla kuin rivejä Varmuuskopiot-listassa: Ctrl+napsautus, Shift+napsautus, Ctrl+A, pitkä painallus tai **Valitse**. **Palauta** avaa sitten yhden ikkunan kaikille.

- **Mikä varmuuskopio**: **Suositeltu** ottaa jokaiselle merkinnälle sen varmuuskopion, jota sen oma ikkuna suosittelisi. **Yksi hetki kaikille** ottaa jokaiselle merkinnälle sen uusimman varmuuskopion valitsemastasi **Hetki**stä tai sitä aiemmin, esimerkiksi edellisen illan, ennen kuin jokin meni pieleen.
- **Minne**: **Alkuperäisen viereen** tai **Korvaa alkuperäinen**, **Varmuuskopioi nykytila ensin** -valinnalla.

Jokainen merkintä tarkistetaan kuten yksittäinen palautus, ja sitten ne ajetaan peräkkäin. Jokainen rivi näyttää, miten sen oma meni: **Odottaa**, **Käynnissä**, **Palautettu**, **Epäonnistui** tai **Jätetty pois**. **Pysäytä tämän jälkeen** antaa käynnissä olevan palautuksen valmistua eikä aloita enempää. Flash-asema tulee ladattavana ZIP-tiedostona, ja ZFS-tietojoukot ja itsevarmuuskopio palautetaan yksi kerrallaan.

![Useiden merkintöjen palautus: yksi rivi merkintää kohti edistymisineen.](assets/screenshots/restore/several-dark.png#only-dark)
![Useiden merkintöjen palautus: yksi rivi merkintää kohti edistymisineen.](assets/screenshots/restore/several-light.png#only-light)

## Stackin palautus {#stacks}

Stackin napsautus **Stackit**-kortissa avaa **Palauta stack** -ikkunan. Se palauttaa jokaisen jäsenen sen viimeisimmästä varmuuskopiosta samaan aikaan, ja valinnalla **Käynnistä riippuvuusjärjestyksessä palautuksen jälkeen** se käynnistää ne `depends_on`-järjestyksessä: tietokannat ja välimuistit ensin, sitten sovellus. Kun jäsenen projektikansio puuttuu valitsemastasi paikasta, ikkuna tarjoaa sen sijaan Paikallisen tilan.

BombVault tarkistaa ensin jäsenet ja projektikansion, eikä palautus, joka ei voisi onnistua, muuta mitään. Muuten se pysäyttää käynnissä olevat jäsenet, sovellukset ennen niiden tietokantoja, kirjoittaa sitten projektikansion ja palauttaa sen jälkeen jäsenet. Jos projektikansio ei palaudu, jäsenet palautetaan silti, mutta ne jäävät pysäytetyiksi, koska kansio voi olla puoliksi kirjoitettu ja jäsenen bind mountit ovat yleensä juuri siellä. Jokaisen jäsenen ajo kertoo sen.

Tietokanta, jolla on vain tietokantavedoksia eikä tiedostovarmuuskopiota ja jonka kontti ja nimetty taltio ovat poissa, palautuu ilman dataansa. Tarkistus kertoo sen ennen palautusta. Tietokanta jää pysäytetyksi, ja niin jäävät myös siitä riippuvat kontit, jotta mikään sovellus ei aja asennustaan tyhjää tietokantaa vasten. Käynnistä se, tuo vedos ja käynnistä muut. Katso [Tietokantavedokset](backups.md#database-dumps).

## Muut lähteet {#other-sources}

- **Vanhasta varmuuskopiokansiosta** avaa sellaisen toisen BombVaultin repon, jota ei enää ajeta, levyltä tai etäkohteesta, sen palautuspaketilla tai kirjoitetulla `APP_KEY`-arvolla. Istunto on vain luku ja elää vain muistissa. Katso [Vanhasta varmuuskopiokansiosta](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Tuo Appdata.Backup-liitännäisestä** lukee liitännäisen varmuuskopiokansion. Jokaisesta konttiarkistosta tulee kontin palautuspiste, päivätty sen mukaan, milloin liitännäinen sen teki. Aiemmin tuodut arkistot ohitetaan, ja arkistoja itseään vain luetaan. Kontille tarvitaan ensin yksi varmuuskopio BombVaultissa, jotta palautuksella on sen määritys. Säilytys jättää tuodut palautuspisteet rauhaan, joten poista sellainen itse, kun et enää tarvitse sitä.
- **Tehtävät Duplicatista tai Duplicacysta** muuttaa näiden työkalujen varmuuskopiointitehtävät kansiojoukoiksi. Se lukee vain niiden asetukset. Vanhat varmuuskopiot jäävät vanhaan työkaluun, ja BombVault aloittaa omalla ensimmäisellä varmuuskopiollaan. Kumpaakaan työkalua ei tarvitse asentaa tänne.
    - **Duplicatista**: vie Duplicatissa jokainen tehtävä tiedostoon ilman salasanaa ja valitse `.json`-tiedostot. Jos vienti sisältää salasanat, mukaan tulevat myös kohteen tunnukset.
    - **Duplicacysta**: valitse Duplicacy Webin kansio, jossa on `duplicacy.json`, tai kansio, jonka varmuuskopioit Duplicacyn komentoriviltä ja jossa on `.duplicacy`. Avaimet, jotka komentorivi on tallentanut tiedostoon `.duplicacy/preferences`, tulevat mukana. Duplicacy Web säilyttää tunnuksensa salattuina, joten ikkuna kysyy ne.

    Mitään ei luoda ennen kuin olet nähnyt esikatselun. Jokaisesta lähdekansiosta tulee kansiojoukko, johon tulevat kansio siitä paikasta tällä palvelimella, josta se löytyi, poissulut, jotka voitiin kääntää, ja aikataulu. Jos vanha työkalu toimi omassa kontissaan eikä jotain kansiota löytynyt, valitset sen itse. Varmuuskopiot menevät uuteen kohteeseen, kansioon `bombvault` vanhan kohteen sisällä, kun vanha kohde on paikallinen kansio, S3, SFTP, rclone-remote tai Backblaze B2 S3-osoitteensa kautta. Tiedostosta saadut tunnukset tallennetaan muiden tunnustesi joukkoon, ja ne, jotka tiedostosta puuttuvat, kysytään sinulta. Kunkin tehtävän alla olevat huomautukset kertovat, mikä ei tullut mukaan, esimerkiksi sisällytyssäännöt, säännölliset lausekkeet, kuinka kauan varmuuskopioita säilytettiin ja vanha salaussalasana, jota BombVault ei tarvitse.

## Muut palvelimet {#other-servers}

[Jaetussa arkistossa](offsite-recovery.md#shared-repository) jokainen avain avaa kaikkien palvelimien varmuuskopiot, joten Palautus-sivu listaa muut jäsenet kohdassa **Muut palvelimet**, kunkin nimen, isännän, merkintöjen määrän ja sen, milloin sen uusin varmuuskopio ajettiin. Isäntä, joka ei kuulu millekään tämän palvelimen tuntemalle palvelimelle, näkyy muodossa **Palvelin, jota tämä palvelin ei tunne**. Napsautus avaa näkymän, jonka otsikossa on **Palvelimen**, palvelimen nimi ja **varmuuskopiot**, vain luku ja ilman mitään syötettävää. Kansiojoukot ja ZFS-tietojoukot menevät valitsemaasi kansioon. Kontit ja VM:t palaavat tavallisina tiedostoina kansioon, koska toinen palvelin sinetöi niiden määritykset omalla avaimellaan. Tämän palvelimen omat varmuuskopiot jaetussa arkistossa ovat kunkin merkinnän **Varmuuskopiot**-välilehdellä.

## Palautuksen jälkeen {#after}

Alkuperäisen viereen palautettu kopio listataan kohdassa **Varmuuskopiot** pois suljettuna, joten se ei lasketa avoimeksi tehtäväksi. Lisää se aikatauluun, jos haluat pitää sen. Kontti tai virtuaalikone, joka oli käynnissä varmuuskopioitaessa, palautuu käynnissä olevana, pysäytetty pysyy pysäytettynä. Unraidissa palautettu kontti ilmestyy uudelleen Docker-välilehdelle tavallisena, muokattavana sovelluksena, ja virtuaalikone VM Manageriin levyineen ja NVRAMeineen liitettyinä.
