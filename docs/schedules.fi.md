# Aikataulut

Kohta **Asetukset, Aikataulut** sisältää kaikki aikataulut: täysvarmuuskopion, joka kattaa kaiken, osion oman aikataulun, etäkopiot sekä sen, mitkä merkinnät ovat mukana ja missä järjestyksessä. Yksittäiselle merkinnälle voi asettaa oman aikataulun sen omalla sivulla. Tällä sivulla kerrotaan, miten ne sopivat yhteen, ja käydään läpi tyypillinen asetus.

## Aikataulut luettuna lauseina {#plans}

Jokainen aikataulu on painike, joka kuuluu lauseena, esimerkiksi *päivittäin kello 03.00* tai *joka 2. päivä kello 01.30*. Napsautus avaa sen viidellä tilalla:

| Tila | Mitä asetat |
|---|---|
| **Pois** | Tällä aikataululla ei aja mitään. |
| **Päivittäin** | **Aika** |
| **Viikoittain** | **Päivät** ja **Aika** |
| **Joka N. päivä** | **Joka** tietty määrä **päivää** ja **Aika**. Laskenta alkaa viimeisestä ajastetusta ajosta, joten yhden merkinnän varmuuskopiointi käsin ei siirrä aikataulua. |
| **Cron** | Viisikenttäinen **Lauseke**, esimerkiksi `0 */6 * * *` joka 6. tunti. **Esimerkkejä** tarjoaa muutaman, ja ikkuna näyttää seuraavat ajot. |

Kaikki ajat ovat palvelimen aikavyöhykkeellä. Unraidissa se on järjestelmän oma. Muualla aseta `TZ`, tai jokainen aikataulu ajetaan UTC:ssä, katso [Ympäristömuuttujat](configuration.md#environment-variables).

## Viikko {#week}

**Viikko**-kortti piirtää jokaisen suunnitellun ajon maanantaista sunnuntaihin, yksi rivi päivää kohti: varmuuskopiot, etäkopiot ja tarkistukset. Osoita viivaa nähdäksesi, mikä siinä alkaa. Kun kolme aikataulua alkaa kahden tunnin sisällä, kohta merkitään, jotta voit hajauttaa ne.

![Asetukset, Aikataulut: Viikko-palkki ylhäällä, sen alla täysvarmuuskopio ja osioiden aikataulut.](assets/screenshots/schedule/week-dark.png#only-dark)
![Asetukset, Aikataulut: Viikko-palkki ylhäällä, sen alla täysvarmuuskopio ja osioiden aikataulut.](assets/screenshots/schedule/week-light.png#only-light)

## Täysvarmuuskopio {#full-backup}

**Täysvarmuuskopio** on kaiken oletusaikataulu. Sen asettaa **Milloin varmuuskopiot ajetaan**: yksi läpiajo, joka varmuuskopioi jokaisen osion vuorollaan, kontit, virtuaalikoneet, flash, kansiot, ZFS-tietojoukot ja viimeisenä itsevarmuuskopion. Sen alla oleva rivi kertoo, mitkä osiot siihen kuuluvat. **Pois** kytkee sen pois, ja silloin ajetaan vain se, jolla on oma aikataulu.

- **Aja väliin jääneet varmuuskopiot käynnistyksen jälkeen**: kun palvelin oli sammutettuna suunniteltuna aikana, BombVault ajaa kyseisen varmuuskopion noin kaksi minuuttia käynnistyksen jälkeen. Merkinnän oma aikataulu ja etäkopioiden aikataulu odottavat seuraavaa suunniteltua aikaansa, ja niin odottaa myös aikataulu, joka ei ole koskaan onnistunut.
- **Komennot ennen ja jälkeen**: **Komento ennen täysvarmuuskopiota** ajetaan palvelimella komennolla `sh -c` ennen läpiajon alkua, esimerkiksi skripti, joka pysäyttää palvelun. Jos se epäonnistuu, täysvarmuuskopio ajetaan silti. **Komento täysvarmuuskopion jälkeen** ajetaan kerran sen jälkeen, kun jokaista osiota on yritetty, vaikka jokin niistä epäonnistuisi, mikä sopii pingille, joka vahvistaa koko palvelimen olevan suojattu. Yksittäisen kontin komennot ovat sen omalla sivulla.

**Varmuuskopioi kaikki nyt** Varmuuskopiot-sivulla käynnistää saman läpiajon käsin kaikille päällä oleville osioille.

## Aikataulut osioittain {#plans-per-section}

Osio voi jättää täysvarmuuskopion ja ajaa omaan aikaansa. Kohdassa **Aikataulut osioittain** **Poikkeus** lisää osion, jolla on oma aikataulu, ja **Poista poikkeus** palauttaa sen. Kortin rivi kertoo, mitä muut tekevät, esimerkiksi **Kaikki muut osiot ajetaan täysvarmuuskopiossa.**

Osiota, jolla on oma aikataulu, ei varmuuskopioida uudelleen täysvarmuuskopiossa, joten mitään ei varmuuskopioida kahdesti. Sama pätee yksittäiseen merkintään, jolla on oma aikataulu.

**Etäkopiot osioittain** toimii samoin kopioille. Oletuksena jokainen osio kopioidaan etäkohteisiinsa heti jokaisen varmuuskopion jälkeen. **Poikkeus** antaa yhdelle osiolle oman kopiointiaikataulun, esimerkiksi jotta virtuaalikoneet kopioidaan Backblaze B2:een vain sunnuntai-iltaisin. Myös kohteen oman **Kopioi**-kortin kohta **Milloin kopioidaan** voi asettaa tämän, katso [Etäkopiot](offsite-recovery.md#off-site-copies).

## Aikataulutetut merkinnät {#scheduled-entries}

**Aikataulutetut merkinnät** kertoo osiota kohti, kuinka monta merkintää on mukana, esimerkiksi **38 / 41 aikataulutettu**. **Merkinnät** avaa listan, jossa jokaisella merkinnällä on **Aikataulussa**-kytkin. Pois kytketty merkintä varmuuskopioidaan vain, kun käynnistät sen käsin. Se lasketaan suojaamattomaksi, ellet merkitse sitä kohtaan **Älä varmuuskopioi**.

## Järjestys {#order}

**Järjestys** asettaa järjestyksen, jossa ajastetut ajot ja täysvarmuuskopio varmuuskopioivat kontit ja virtuaalikoneet. Vedä merkintä paikalleen tai käytä nuolia, ja jokainen siirto tallentuu heti. Merkinnät, joita et järjestä, varmuuskopioidaan eniten myöhässä oleva ensin. Yksittäinen käsin tehty varmuuskopio ei vaikuta tähän.

**Uudelleenkäynnistys varmuuskopioinnin jälkeen** samalla kortilla koskee kontteja, jotka **Pysäytä muut säiliöt** pysäytti varmuuskopioinnin ajaksi. Ne käynnistyvät uudelleen riippuvuusjärjestyksessään, ja kun **Odota, että riippuvuudet ovat kunnossa, ennen seuraavan käynnistämistä** on päällä, jokaisen on raportoitava olevansa kunnossa tai käynnissä, ennen kuin seuraava käynnistyy. **Säiliökohtainen kunnon aikakatkaisu (sekuntia)** rajoittaa odotuksen, oletuksena 120, välillä 5-3600.

## Tilannevedos pysäytyksen sijaan {#snapshot-instead-of-stop}

**Pysäytä ZFS:llä olevat kontit vain tilannevedoksen ajaksi** on oletuksena päällä. Kontti, jonka data on ZFS:llä, pysäytetään vain sen ajaksi, kun BombVault ottaa kyseisestä datasta tilannevedoksen, yleensä muutaman sekunnin, ja varmuuskopio luetaan sen jälkeen tilannevedoksesta. Se vaatii SSH-yhteyden isäntään, jota ZFS-tietojoukot käyttävät. Kontti, jonka datasta osa on muualla, pysäytetään koko varmuuskopioinnin ajaksi kuten ennenkin, ja sen ajo kertoo syyn, kun taas kontti, jonka mikään data ei ole ZFS:llä, pysäytetään ilman huomautusta. Se toimii Unraidin käyttäjäjaolla, joka on yhdellä ZFS-poolilla. TrueNASissa sovellus tarvitsee lisäksi `SYS_CHROOT`-oikeuden, katso [TrueNAS SCALE](getting-started.md#truenas). Yksittäiselle kontille voi asettaa toisin sen omalla sivulla.

Varmuuskopio päätyy samoihin polkuihin ja tunnisteisiin kuin kontin ollessa pysäytettynä otettu, joten palautukset ja tarkistukset käsittelevät molempia samoin. Komento ennen varmuuskopiota ja tietokantavedos ajetaan edelleen ensin, muut pysäytettävät kontit menevät alas ja palaavat sen mukana, ja komento varmuuskopion jälkeen ajetaan heti, kun kontti on taas käynnissä. Päivitys varmuuskopion jälkeen odottaa edelleen, kunnes varmuuskopio on valmis. Sen ajoissa näkyy **Tilannevedos** ja kuinka monta sekuntia kontti oli alhaalla. Jos BombVault pysähtyy kesken minkä tahansa kontin varmuuskopioinnin, se käynnistää pitämänsä kontit uudelleen seuraavalla käynnistyskerralla.

## Päivitykset varmuuskopion jälkeen {#updates}

- **Päivitä kontit onnistuneen varmuuskopioinnin jälkeen** hakee uudemman imagen vasta, kun sitä edeltävä varmuuskopio on valmis, joten tuore palautuspiste on aina olemassa ensin. Se on oletuksena pois päältä, ja yksittäiselle kontille voi asettaa toisin sen sivulla kohdassa **Päivitä onnistuneen varmuuskopioinnin jälkeen**. Uuteen imageen siirtyy vain se, minkä olet itse asettanut: muuttujat, tunnisteet, komento, käyttäjä ja terveystarkistus, jotka ovat yhä samat kuin vanhassa imagessa, seuraavat uutta. Jos konttia ei voi luoda tai käynnistää uudella imagella, kontti palaa imageen, jota se ajoi, päivitys näkyy epäonnistuneena ja seuraava varmuuskopio yrittää uudelleen. Kontti, joka jää tämän vuoksi alas, nimetään varmuuskopion ajossa ja ilmoituksessa.
- **Poista vanha image päivityksen jälkeen** poistaa imagen, jonka kontti jätti. Perusimagea, jota muut kontit käyttävät yhdessä, ei koskaan poisteta.
- **Päivitä Unraidin päivitystila säiliön päivittämisen jälkeen** tyhjentää Unraidin päivitysbannerin kyseiseltä kontilta.

Viesti päivitettyä konttia kohti asetetaan kohdassa **Asetukset, Ilmoitukset** valinnalla **Ilmoita containerin päivityksestä**.

## Joutilaisuus ennen varmuuskopiota {#idle}

**Joutilaisuus ennen varmuuskopiota** -kortti asettaa, milloin sovellus lasketaan joutilaaksi kontille, jonka ajastettu varmuuskopio odottaa sitä: **Suoritin alle (% ytimestä)**, **Liikenne alle (Mbit/s)** ja **Vähintään (min)**. Mediapalvelin lasketaan joutilaaksi, kun se ei striimaa, ja isäntäverkossa olevalle kontille vain suoritin ratkaisee. Itse odotus asetetaan jokaiselle kontille, katso [Kun sovellus on kiireinen](backups.md#app-busy).

## Oma aikataulu yhdelle merkinnälle {#own-plan}

Jokaisella merkinnällä on **Oma aikataulu** sen Asetukset-välilehden **Aikataulu ja kohde** -kortissa. Kun siellä on aikataulu, merkintä ajetaan omina aikoinaan ja jättää täysvarmuuskopion sekä osionsa aikataulun. Ilman sitä se seuraa niitä.

## Esimerkki: kolme yhteensopivaa aikataulua {#example}

Tyypillinen Unraid-laatikko: useimmat kontit voi varmuuskopioida yöllä, virtuaalikoneet ovat suuria ja muuttuvat hitaasti, ja Immich saa uusia kuvia koko päivän.

1. **Täysvarmuuskopio kello 03.00.** Avaa **Asetukset, Aikataulut**. Aseta kohdassa **Täysvarmuuskopio** **Milloin varmuuskopiot ajetaan** arvoon **Päivittäin** kello `03:00`. Kytke päälle **Aja väliin jääneet varmuuskopiot käynnistyksen jälkeen**, jotta yö, jonka palvelin oli sammutettuna, korvataan.
2. **Virtuaalikoneet kerran viikossa.** Paina kohdassa **Aikataulut osioittain** **Poikkeus**, valitse **VMs** ja aseta sen aikataulu arvoon **Viikoittain**, **Päivät** sunnuntai, **Aika** `04:00`. Kortti kertoo nyt, että kaikki muut osiot ajetaan täysvarmuuskopiossa.
3. **Virtuaalikoneet kopioidaan etänä myös sunnuntaina.** Paina kohdassa **Etäkopiot osioittain** **Poikkeus**, valitse **VMs** ja aseta **Viikoittain**, sunnuntai kello `06:00`, kun niiden varmuuskopiolla on ollut aikaa valmistua.
4. **Immich 6 tunnin välein.** Avaa **Varmuuskopiot**, sitten `immich` ja sen **Asetukset**-välilehti. Aseta **Oma aikataulu** arvoon **Cron** lausekkeella `0 */6 * * *`.

Mitä nyt ajetaan:

| Milloin | Mitä |
|---|---|
| Joka päivä kello 03.00 | Kontit paitsi `immich`, flash, kansiot, ZFS-tietojoukot ja itsevarmuuskopio, yksi osio kerrallaan |
| Sunnuntaina kello 04.00 | VMs |
| Sunnuntaina kello 06.00 | Virtuaalikoneiden etäkopio |
| 00.00, 06.00, 12.00 ja 18.00 | `immich` yksinään |

**Viikko**-kortti näyttää 03.00-rivin joka päivälle, 04.00- ja 06.00-viivat sunnuntaille ja neljä viivaa päivässä `immich`ille. `immich` ei varmuuskopioida toista kertaa kello 03.00, koska merkintä, jolla on oma aikataulu, jättää täysvarmuuskopion. Muiden osioiden kopiot ajetaan edelleen heti jokaisen varmuuskopion jälkeen.

Kun myöhemmin poistat `immich`in oman aikataulun, se liittyy taas osionsa mukana kello 03.00 -ajoon.
